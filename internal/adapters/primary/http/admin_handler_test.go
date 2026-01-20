package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	pgadapter "github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/services"
)

func TestAdminUsersList(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	admin, token := createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	agent := registerUser(t, ctx, authService, "Agent User", "agent-"+uuid.NewString()+"@example.com", "agent", orgID)
	customer := registerUser(t, ctx, authService, "Customer User", "customer-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, _ := newAdminRouter()
	req := httptest.NewRequest(stdhttp.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var response struct {
		Data  []UserSummaryDTO `json:"data"`
		Count int              `json:"count"`
	}
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	assertUserInList(t, response.Data, admin.ID, "admin")
	assertUserInList(t, response.Data, agent.ID, "agent")
	assertUserInList(t, response.Data, customer.ID, "customer")
}

func TestAdminUsersList_Forbidden(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	_, _ = createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	customer := registerUser(t, ctx, authService, "Customer User", "customer-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, tokenManager := newAdminRouter()
	token, err := tokenManager.GenerateToken(customer.ID, orgID)
	require.NoError(t, err)

	req := httptest.NewRequest(stdhttp.MethodGet, "/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
}

func TestAdminUpdateUserRole(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	admin, token := createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	target := registerUser(t, ctx, authService, "Target User", "target-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, _ := newAdminRouter()
	payload := []byte(`{"role":"agent"}`)
	req := httptest.NewRequest(stdhttp.MethodPatch, "/admin/users/"+target.ID.String()+"/role", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusNoContent, recorder.Code)

	listReq := httptest.NewRequest(stdhttp.MethodGet, "/admin/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()

	router.ServeHTTP(listRecorder, listReq)
	require.Equal(t, stdhttp.StatusOK, listRecorder.Code)

	var response struct {
		Data []UserSummaryDTO `json:"data"`
	}
	require.NoError(t, json.NewDecoder(listRecorder.Body).Decode(&response))

	assertUserInList(t, response.Data, admin.ID, "admin")
	assertUserInList(t, response.Data, target.ID, "agent")
}

func TestAdminUpdateUserStatus(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	_, token := createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	target := registerUser(t, ctx, authService, "Inactive User", "inactive-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, _ := newAdminRouter()
	payload := []byte(`{"isActive":false}`)
	req := httptest.NewRequest(stdhttp.MethodPatch, "/admin/users/"+target.ID.String()+"/status", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusNoContent, recorder.Code)

	listReq := httptest.NewRequest(stdhttp.MethodGet, "/admin/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()

	router.ServeHTTP(listRecorder, listReq)
	require.Equal(t, stdhttp.StatusOK, listRecorder.Code)

	var response struct {
		Data []UserSummaryDTO `json:"data"`
	}
	require.NoError(t, json.NewDecoder(listRecorder.Body).Decode(&response))

	user := findUser(response.Data, target.ID)
	require.NotNil(t, user)
	assert.False(t, user.IsActive)
}

func TestAdminResetPassword(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	_, token := createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	target := registerUser(t, ctx, authService, "Reset User", "reset-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, _ := newAdminRouter()
	req := httptest.NewRequest(stdhttp.MethodPost, "/admin/users/"+target.ID.String()+"/reset-password", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var response ResetPasswordResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	require.NotEmpty(t, response.TemporaryPassword)

	_, err := authService.Login(ctx, target.Email, "Password1")
	assert.ErrorIs(t, err, apperrors.ErrInvalidCredentials)

	user, err := authService.Login(ctx, target.Email, response.TemporaryPassword)
	require.NoError(t, err)
	assert.Equal(t, target.ID, user.ID)
}

func TestAdminResetPassword_Forbidden(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	_, _ = createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	customer := registerUser(t, ctx, authService, "Customer User", "customer-"+uuid.NewString()+"@example.com", "customer", orgID)
	target := registerUser(t, ctx, authService, "Target User", "target-"+uuid.NewString()+"@example.com", "customer", orgID)

	router, tokenManager := newAdminRouter()
	token, err := tokenManager.GenerateToken(customer.ID, orgID)
	require.NoError(t, err)

	req := httptest.NewRequest(stdhttp.MethodPost, "/admin/users/"+target.ID.String()+"/reset-password", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
}

func TestAdminAnalyticsOverview(t *testing.T) {
	ctx := context.Background()
	orgID := createTestOrganization(t, ctx)

	_, token := createAdminAndToken(t, ctx, orgID)

	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	agent := registerUser(t, ctx, authService, "Agent User", "agent-"+uuid.NewString()+"@example.com", "agent", orgID)
	customer := registerUser(t, ctx, authService, "Customer User", "customer-"+uuid.NewString()+"@example.com", "customer", orgID)

	ticketRepo := pgadapter.NewTicketRepository(testPool)

	openTicket := createTicket(t, ctx, ticketRepo, customer.ID, "Open Ticket")
	assert.Equal(t, domain.StatusOpen, openTicket.Status)

	closedTicket := createTicket(t, ctx, ticketRepo, customer.ID, "Closed Ticket")
	require.NoError(t, closedTicket.Assign(agent.ID))
	require.NoError(t, closedTicket.UpdateStatus(domain.StatusClosed))
	_, err := ticketRepo.Update(ctx, closedTicket)
	require.NoError(t, err)

	router, _ := newAdminRouter()
	req := httptest.NewRequest(stdhttp.MethodGet, "/admin/analytics/overview?days=7", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var response AnalyticsOverviewResponse
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))

	assertStatusCount(t, response.StatusCounts, domain.StatusOpen.String(), 1)
	assertStatusCount(t, response.StatusCounts, domain.StatusClosed.String(), 1)
	assertStatusCount(t, response.StatusCounts, domain.StatusInProgress.String(), 0)

	assertWorkloadUnassigned(t, response.Workload, 1)
	assert.GreaterOrEqual(t, response.MTTRHours, 0.0)

	createdTotal, resolvedTotal := sumVolume(response.Volume)
	assert.Equal(t, int64(2), createdTotal)
	assert.Equal(t, int64(1), resolvedTotal)
}

func newAdminRouter() (*chi.Mux, *auth.TokenManager) {
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	userRepo := pgadapter.NewUserRepository(testPool)
	analyticsRepo := pgadapter.NewAnalyticsRepository(testPool)
	authzService := services.NewAuthorizationService(authRepo)
	adminService := services.NewAdminService(userRepo, authRepo, authzService, analyticsRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := NewErrorHandler(logger)
	adminHandler := NewAdminHandler(adminService, errorHandler, logger)
	tokenManager := auth.NewTokenManager("test-secret", time.Hour)

	router := chi.NewRouter()
	router.Use(mw.JWTMiddleware(tokenManager))
	router.Route("/admin", adminHandler.RegisterRoutes)

	return router, tokenManager
}

func createTestOrganization(t *testing.T, ctx context.Context) uuid.UUID {
	orgID := uuid.New()
	_, err := testPool.Exec(ctx, "INSERT INTO organizations (id, name) VALUES ($1, $2)", orgID, "Test Org "+orgID.String())
	require.NoError(t, err)
	return orgID
}

func createAdminAndToken(t *testing.T, ctx context.Context, orgID uuid.UUID) (*domain.User, string) {
	userRepo := pgadapter.NewUserRepository(testPool)
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authService := services.NewAuthService(userRepo, authRepo, orgID)

	admin := registerUser(t, ctx, authService, "Admin User", "admin-"+uuid.NewString()+"@example.com", "admin", orgID)

	tokenManager := auth.NewTokenManager("test-secret", time.Hour)
	token, err := tokenManager.GenerateToken(admin.ID, orgID)
	require.NoError(t, err)

	return admin, token
}

func registerUser(t *testing.T, ctx context.Context, authService ports.AuthService, fullName, email, role string, orgID uuid.UUID) *domain.User {
	user, err := authService.Register(ctx, fullName, email, "Password1", role, orgID)
	require.NoError(t, err)
	return user
}

func createTicket(t *testing.T, ctx context.Context, repo ports.TicketRepository, requesterID uuid.UUID, title string) *domain.Ticket {
	params := domain.TicketParams{
		Title:       title,
		Description: "Analytics test",
		Priority:    domain.PriorityMedium,
		RequesterID: requesterID,
	}

	ticket, err := domain.NewTicket(params)
	require.NoError(t, err)

	created, err := repo.Create(ctx, ticket)
	require.NoError(t, err)
	return created
}

func assertUserInList(t *testing.T, users []UserSummaryDTO, userID uuid.UUID, role string) {
	user := findUser(users, userID)
	require.NotNil(t, user)
	assert.Contains(t, user.Roles, role)
	assert.True(t, user.IsActive)
}

func findUser(users []UserSummaryDTO, userID uuid.UUID) *UserSummaryDTO {
	for _, user := range users {
		if user.ID == userID.String() {
			return &user
		}
	}
	return nil
}

func assertStatusCount(t *testing.T, counts []StatusCountDTO, status string, expected int64) {
	for _, count := range counts {
		if count.Status == status {
			assert.Equal(t, expected, count.Count)
			return
		}
	}
	assert.Fail(t, "status missing", "status %s not found", status)
}

func assertWorkloadUnassigned(t *testing.T, items []WorkloadItemDTO, expected int64) {
	for _, item := range items {
		if item.AssigneeID == nil {
			assert.Equal(t, expected, item.Count)
			return
		}
	}
	assert.Fail(t, "unassigned missing")
}

func sumVolume(points []VolumePointDTO) (int64, int64) {
	var createdTotal int64
	var resolvedTotal int64
	for _, point := range points {
		createdTotal += point.CreatedCount
		resolvedTotal += point.ResolvedCount
	}
	return createdTotal, resolvedTotal
}
