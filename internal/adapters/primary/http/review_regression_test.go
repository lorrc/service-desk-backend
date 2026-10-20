package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/email"
	pgadapter "github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These regressions exercise real handlers, services, transactions and PostgreSQL.
func newReviewRouter(t *testing.T) stdhttp.Handler {
	t.Helper()
	users := pgadapter.NewUserRepository(testPool)
	rbac := pgadapter.NewAuthorizationRepository(testPool)
	permissions := services.NewAuthorizationService(rbac)
	events := pgadapter.NewTicketEventRepository(testPool)
	tx := pgadapter.NewTransactionManager(testPool)
	notifier := email.NewMockSMTPNotifier(users)
	tickets := services.NewTicketService(pgadapter.NewTicketRepository(testPool), permissions, notifier, events, tx, users)
	t.Cleanup(tickets.Shutdown)
	comments := services.NewCommentService(pgadapter.NewCommentRepository(testPool), tickets, permissions, notifier, events, tx)
	lookup := services.NewUserLookupService(users)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	errors := NewErrorHandler(logger)
	commentHandler := NewCommentHandler(comments, lookup, errors, logger)
	ticketHandler := NewTicketHandler(tickets, services.NewEventService(events, tickets), lookup, commentHandler, errors, logger)
	adminHandler := NewAdminHandler(services.NewAdminService(users, rbac, permissions, pgadapter.NewAnalyticsRepository(testPool)), reviewOnboarding(), errors, logger)
	router := chi.NewRouter()
	router.Use(mw.JWTMiddleware(auth.NewTokenManager("test-secret", time.Hour), users))
	router.Route("/tickets", ticketHandler.RegisterRoutes)
	router.Route("/admin", adminHandler.RegisterRoutes)
	meHandler := NewMeHandler(permissions, services.NewAccountService(users, permissions, pgadapter.NewOrganizationRepository(testPool)), errors, logger)
	router.Route("/me", meHandler.RegisterRoutes)
	return router
}

func reviewAuth(orgID uuid.UUID) ports.AuthService {
	return services.NewAuthService(pgadapter.NewUserRepository(testPool), pgadapter.NewAuthorizationRepository(testPool), pgadapter.NewTransactionManager(testPool), orgID)
}

func reviewOnboarding() ports.OnboardingService {
	users := pgadapter.NewUserRepository(testPool)
	roles := pgadapter.NewAuthorizationRepository(testPool)
	return services.NewOnboardingService(users, roles, services.NewAuthorizationService(roles), pgadapter.NewOrganizationRepository(testPool), pgadapter.NewInvitationRepository(testPool), pgadapter.NewTransactionManager(testPool))
}

func reviewRequest(router stdhttp.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func reviewToken(t *testing.T, user *domain.User) string {
	t.Helper()
	token, err := auth.NewTokenManager("test-secret", time.Hour).GenerateToken(user.ID, user.OrganizationID, user.SessionVersion)
	require.NoError(t, err)
	return token
}

func TestReviewOrganizationIsolation(t *testing.T) {
	ctx := context.Background()
	orgA, orgB := createTestOrganization(t, ctx), createTestOrganization(t, ctx)
	agent := registerUser(t, ctx, reviewAuth(orgA), "Agent", uuid.NewString()+"@example.com", "agent", orgA)
	other := registerUser(t, ctx, reviewAuth(orgB), "Other", uuid.NewString()+"@example.com", "agent", orgB)
	repo := pgadapter.NewTicketRepository(testPool)
	visible := createTicket(t, ctx, repo, agent.ID, "Visible")
	secret := createTicket(t, ctx, repo, other.ID, "Other organization")
	router := newReviewRouter(t)
	token := reviewToken(t, agent)
	list := reviewRequest(router, "GET", "/tickets", token, "")
	require.Equal(t, 200, list.Code, list.Body.String())
	var page PaginatedResponse[TicketDTO]
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	assert.Equal(t, visible.ID, page.Data[0].ID)
	path := fmt.Sprintf("/tickets/%d", secret.ID)
	for _, tc := range []struct {
		method, suffix, body string
		status               int
	}{
		{"GET", "", "", 404},
		{"PATCH", "/status", `{"status":"CLOSED"}`, 404},
		{"PATCH", "/assignee", fmt.Sprintf(`{"assigneeId":%q}`, agent.ID.String()), 404},
		{"GET", "/comments", "", 403},
		{"POST", "/comments", `{"body":"Not allowed"}`, 404},
		{"GET", "/events", "", 404},
	} {
		t.Run(tc.method+tc.suffix, func(t *testing.T) {
			response := reviewRequest(router, tc.method, path+tc.suffix, token, tc.body)
			assert.Equal(t, tc.status, response.Code, response.Body.String())
		})
	}
	response := reviewRequest(router, "PATCH", fmt.Sprintf("/tickets/%d/assignee", visible.ID), token, fmt.Sprintf(`{"assigneeId":%q}`, other.ID.String()))
	assert.Equal(t, 403, response.Code, response.Body.String())
	unchanged, err := repo.GetByID(ctx, secret.ID, orgB)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusOpen, unchanged.Status)
	assert.Nil(t, unchanged.AssigneeID)
}

func TestReviewSessionsAreRevoked(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)
	_, adminToken := createAdminAndToken(t, ctx, orgID)
	svc := reviewAuth(orgID)
	user := registerUser(t, ctx, svc, "Session User", uuid.NewString()+"@example.com", "customer", orgID)
	router := newReviewRouter(t)
	token := reviewToken(t, user)
	assert.Equal(t, 200, reviewRequest(router, "GET", "/tickets", token, "").Code)
	path := "/admin/users/" + user.ID.String()
	require.Equal(t, 204, reviewRequest(router, "PATCH", path+"/status", adminToken, `{"isActive":false}`).Code)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/tickets", token, "").Code)
	require.Equal(t, 204, reviewRequest(router, "PATCH", path+"/status", adminToken, `{"isActive":true}`).Code)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/tickets", token, "").Code)
	user, err := svc.Login(ctx, user.Email, "Password1")
	require.NoError(t, err)
	token = reviewToken(t, user)
	assert.Equal(t, 200, reviewRequest(router, "GET", "/tickets", token, "").Code)
	reset := reviewRequest(router, "POST", path+"/reset-password", adminToken, "")
	require.Equal(t, 200, reset.Code, reset.Body.String())
	assert.Equal(t, 401, reviewRequest(router, "GET", "/tickets", token, "").Code)
	var password ResetPasswordResponse
	require.NoError(t, json.Unmarshal(reset.Body.Bytes(), &password))
	user, err = svc.Login(ctx, user.Email, password.TemporaryPassword)
	require.NoError(t, err)
	assert.Equal(t, 200, reviewRequest(router, "GET", "/tickets", reviewToken(t, user), "").Code)
	wrongOrgToken, err := auth.NewTokenManager("test-secret", time.Hour).GenerateToken(user.ID, uuid.New(), user.SessionVersion)
	require.NoError(t, err)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/tickets", wrongOrgToken, "").Code)
}

func TestReviewRegistrationRollbackAndBootstrap(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)
	svc := reviewAuth(orgID)
	address := uuid.NewString() + "@example.com"
	_, err := svc.Register(ctx, "Retry User", address, "Password1", "missing-role", orgID)
	require.ErrorIs(t, err, apperrors.ErrRoleNotFound)
	_, err = pgadapter.NewUserRepository(testPool).GetByEmail(ctx, address)
	require.ErrorIs(t, err, apperrors.ErrUserNotFound)
	user, err := svc.Register(ctx, "Retry User", address, "Password1", "", orgID)
	require.NoError(t, err)
	permissions, err := pgadapter.NewAuthorizationRepository(testPool).GetUserPermissions(ctx, user.ID)
	require.NoError(t, err)
	assert.NotContains(t, permissions, "admin:access")
	adminEmail := uuid.NewString() + "@example.com"
	errors := make(chan error, 2)
	for range 2 {
		go func() { errors <- svc.BootstrapAdmin(ctx, "Bootstrap Admin", adminEmail, "Password1") }()
	}
	for range 2 {
		require.NoError(t, <-errors)
	}
	admin, err := pgadapter.NewUserRepository(testPool).GetByEmail(ctx, adminEmail)
	require.NoError(t, err)
	permissions, err = pgadapter.NewAuthorizationRepository(testPool).GetUserPermissions(ctx, admin.ID)
	require.NoError(t, err)
	assert.Contains(t, permissions, "admin:access")
	require.NoError(t, svc.BootstrapAdmin(ctx, "Bootstrap Admin", adminEmail, "ChangedPassword1"))
	_, err = svc.Login(ctx, adminEmail, "Password1")
	require.NoError(t, err, "bootstrap must not replace an existing password")
}

func TestReviewConcurrentTicketChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	orgID := createTestOrganization(t, ctx)
	agent := registerUser(t, ctx, reviewAuth(orgID), "Agent", uuid.NewString()+"@example.com", "agent", orgID)
	repo := pgadapter.NewTicketRepository(testPool)
	ticket := createTicket(t, ctx, repo, agent.ID, "Concurrent changes")
	router := newReviewRouter(t)
	token := reviewToken(t, agent)
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, "SELECT id FROM tickets WHERE id = $1 FOR UPDATE", ticket.ID)
	require.NoError(t, err)
	statusResult, assignResult := make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)
	path := fmt.Sprintf("/tickets/%d", ticket.ID)
	go func() { statusResult <- reviewRequest(router, "PATCH", path+"/status", token, `{"status":"CLOSED"}`) }()
	go func() {
		assignResult <- reviewRequest(router, "PATCH", path+"/assignee", token, fmt.Sprintf(`{"assigneeId":%q}`, agent.ID.String()))
	}()
	// Wait until both operations contend on the held row, making stale-read regressions deterministic.
	require.Eventually(t, func() bool {
		var waiting int
		err := testPool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%tickets%'").Scan(&waiting)
		return err == nil && waiting >= 2
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	status, assigned := <-statusResult, <-assignResult
	require.Equal(t, 200, status.Code, status.Body.String())
	require.Contains(t, []int{200, 400}, assigned.Code, assigned.Body.String())
	current, err := repo.GetByID(ctx, ticket.ID, orgID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusClosed, current.Status)
	assert.NotNil(t, current.ClosedAt)
	if assigned.Code == 200 {
		require.NotNil(t, current.AssigneeID)
		assert.Equal(t, agent.ID, *current.AssigneeID)
	}
	events, err := pgadapter.NewTicketEventRepository(testPool).ListByTicketID(ctx, ticket.ID, 0, 10)
	require.NoError(t, err)
	expected := 1
	if assigned.Code == 200 {
		expected++
	}
	assert.Len(t, events, expected)
}

func TestReviewTicketCountsAndPagination(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)
	svc := reviewAuth(orgID)
	user := registerUser(t, ctx, svc, "Customer", uuid.NewString()+"@example.com", "customer", orgID)
	other := registerUser(t, ctx, svc, "Other", uuid.NewString()+"@example.com", "customer", orgID)
	_, err := testPool.Exec(ctx, "INSERT INTO tickets (title, status, priority, requester_id) SELECT 'Older open', 'OPEN', 'HIGH', $1 FROM generate_series(1, 125)", user.ID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, "INSERT INTO tickets (title, status, priority, requester_id) SELECT 'Other customer', 'OPEN', 'HIGH', $1 FROM generate_series(1, 7)", other.ID)
	require.NoError(t, err)
	router := newReviewRouter(t)
	token := reviewToken(t, user)
	response := reviewRequest(router, "GET", "/tickets/counts", token, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var counts domain.TicketCounts
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &counts))
	assert.Equal(t, int64(125), counts.Open)
	assert.Equal(t, int64(125), counts.HighPriority)
	for _, tc := range []struct {
		offset, count int
		more          bool
	}{{0, 25, true}, {100, 25, false}, {125, 0, false}} {
		response := reviewRequest(router, "GET", fmt.Sprintf("/tickets?limit=25&offset=%d", tc.offset), token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		var page PaginatedResponse[TicketDTO]
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		assert.Len(t, page.Data, tc.count)
		assert.Equal(t, tc.more, page.Pagination.HasMore)
		for _, ticket := range page.Data {
			assert.Equal(t, user.ID.String(), ticket.RequesterID)
		}
	}
}
