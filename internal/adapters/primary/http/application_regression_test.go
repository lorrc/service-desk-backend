package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	pgadapter "github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplicationRoleChangeRevokesOnlyOnChange(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	_, adminToken := createAdminAndToken(t, ctx, org)
	user := registerUser(t, ctx, reviewAuth(org), "Role target", uuid.NewString()+"@example.com", "customer", org)
	token := reviewToken(t, user)
	router := newReviewRouter(t)
	endpoint := "/admin/users/" + user.ID.String() + "/role"
	require.Equal(t, 204, reviewRequest(router, "PATCH", endpoint, adminToken, `{"role":"customer"}`).Code)
	require.Equal(t, 200, reviewRequest(router, "GET", "/tickets", token, "").Code)
	repo := pgadapter.NewUserRepository(testPool)
	current, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.SessionVersion, current.SessionVersion)
	require.ErrorIs(t, pgadapter.NewAuthorizationRepository(testPool).SetUserRole(ctx, user.ID, "missing-role"), apperrors.ErrRoleNotFound)
	current, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.SessionVersion, current.SessionVersion)
	// Concurrent identical role changes must revoke once, with no transient empty role set.
	results := make(chan int, 2)
	for range 2 {
		go func() { results <- reviewRequest(router, "PATCH", endpoint, adminToken, `{"role":"agent"}`).Code }()
	}
	for range 2 {
		require.Equal(t, 204, <-results)
	}
	current, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.SessionVersion+1, current.SessionVersion)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/tickets", token, "").Code)
	roles, err := repo.GetRoles(ctx, user.ID, org)
	require.NoError(t, err)
	assert.Equal(t, []string{"agent"}, roles)
	loggedIn, err := reviewAuth(org).Login(ctx, user.Email, "Password1")
	require.NoError(t, err)
	assert.Equal(t, 200, reviewRequest(router, "GET", "/tickets", reviewToken(t, loggedIn), "").Code)
	otherOrg := createTestOrganization(t, ctx)
	other := registerUser(t, ctx, reviewAuth(otherOrg), "Other", uuid.NewString()+"@example.com", "customer", otherOrg)
	assert.Equal(t, 403, reviewRequest(router, "PATCH", "/admin/users/"+other.ID.String()+"/role", adminToken, `{"role":"agent"}`).Code)
}

func TestApplicationProfilePasswordAndSignOut(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	svc := reviewAuth(org)
	user := registerUser(t, ctx, svc, "Account Owner", uuid.NewString()+"@example.com", "customer", org)
	user, err := svc.Login(ctx, user.Email, "Password1")
	require.NoError(t, err)
	router := newReviewRouter(t)
	token := reviewToken(t, user)
	otherToken, err := auth.NewTokenManager("test-secret", 2*time.Hour).GenerateToken(user.ID, org, user.SessionVersion)
	require.NoError(t, err)
	profileResponse := reviewRequest(router, "GET", "/me", token, "")
	require.Equal(t, 200, profileResponse.Code, profileResponse.Body.String())
	var profile ProfileResponse
	require.NoError(t, json.Unmarshal(profileResponse.Body.Bytes(), &profile))
	assert.Equal(t, user.ID.String(), profile.ID)
	assert.Equal(t, "Account Owner", profile.FullName)
	assert.Equal(t, org.String(), profile.OrganizationID)
	assert.Equal(t, []string{"customer"}, profile.Roles)
	assert.Contains(t, profile.Permissions, "tickets:read")
	assert.NotContains(t, profile.Permissions, "admin:access")
	assert.NotNil(t, profile.LastSignInAt)
	assert.NotContains(t, profileResponse.Body.String(), "hashedPassword")
	assert.NotContains(t, profileResponse.Body.String(), "sessionVersion")
	for _, body := range []string{`{"currentPassword":"wrong","newPassword":"ChangedPassword2"}`, `{"currentPassword":"Password1","newPassword":"weak"}`, `{"currentPassword":"Password1","newPassword":"Password1"}`, fmt.Sprintf(`{"currentPassword":"Password1","newPassword":%q}`, "Aa1"+strings.Repeat("x", 70))} {
		response := reviewRequest(router, "POST", "/me/password", token, body)
		require.Equal(t, 422, response.Code, response.Body.String())
		require.Equal(t, 200, reviewRequest(router, "GET", "/me", token, "").Code)
	}
	response := reviewRequest(router, "POST", "/me/password", token, `{"currentPassword":"Password1","newPassword":"ChangedPassword2"}`)
	require.Equal(t, 204, response.Code, response.Body.String())
	assert.Equal(t, 401, reviewRequest(router, "GET", "/me", token, "").Code)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/me", otherToken, "").Code)
	_, err = svc.Login(ctx, user.Email, "Password1")
	require.ErrorIs(t, err, apperrors.ErrInvalidCredentials)
	user, err = svc.Login(ctx, user.Email, "ChangedPassword2")
	require.NoError(t, err)
	token = reviewToken(t, user)
	require.Equal(t, 204, reviewRequest(router, "POST", "/me/sign-out-everywhere", token, "").Code)
	assert.Equal(t, 401, reviewRequest(router, "GET", "/me", token, "").Code)
	user, err = svc.Login(ctx, user.Email, "ChangedPassword2")
	require.NoError(t, err)
	assert.Equal(t, 200, reviewRequest(router, "GET", "/me", reviewToken(t, user), "").Code)
}

func TestApplicationPasswordChangeCannotOverwriteNewerReset(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	user := registerUser(t, ctx, reviewAuth(org), "Reset target", uuid.NewString()+"@example.com", "customer", org)
	repo := pgadapter.NewUserRepository(testPool)
	resetHash, err := domain.HashPassword("AdminReset2")
	require.NoError(t, err)
	require.NoError(t, repo.UpdatePassword(ctx, user.ID, resetHash))
	proposedHash, err := domain.HashPassword("UserChange2")
	require.NoError(t, err)
	require.ErrorIs(t, repo.ChangeOwnPassword(ctx, user.ID, org, user.SessionVersion, user.HashedPassword, proposedHash), apperrors.ErrUnauthorized)
	require.ErrorIs(t, repo.RevokeSessions(ctx, user.ID, org, user.SessionVersion), apperrors.ErrUnauthorized)
	current, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, current.CheckPassword("AdminReset2"))
	assert.Equal(t, user.SessionVersion+1, current.SessionVersion)
}

func TestApplicationTicketSearchCountsAndActivePage(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	agent := registerUser(t, ctx, reviewAuth(org), "Triage agent", uuid.NewString()+"@example.com", "agent", org)
	customer := registerUser(t, ctx, reviewAuth(org), "Customer", uuid.NewString()+"@example.com", "customer", org)
	otherOrg := createTestOrganization(t, ctx)
	other := registerUser(t, ctx, reviewAuth(otherOrg), "Other agent", uuid.NewString()+"@example.com", "agent", otherOrg)
	_, err := testPool.Exec(ctx, `INSERT INTO tickets(title,description,status,priority,requester_id,assignee_id,created_at)
 SELECT 'Network % issue '||n, 'Router failed', 'OPEN', 'HIGH', $1, CASE WHEN n<=2 THEN $1::uuid ELSE NULL END, NOW()-interval '1 hour' FROM generate_series(1,7) n`, agent.ID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `INSERT INTO tickets(title,status,priority,requester_id,assignee_id) SELECT 'Network % closed '||n,'CLOSED','HIGH',$1,CASE WHEN n=1 THEN $1::uuid ELSE NULL END FROM generate_series(1,10) n`, agent.ID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `INSERT INTO tickets(title,status,priority,requester_id) VALUES ('Network % secret','OPEN','HIGH',$1),('Different subject','OPEN','LOW',$2)`, other.ID, customer.ID)
	require.NoError(t, err)
	router := newReviewRouter(t)
	token := reviewToken(t, agent)
	query := "q=" + url.QueryEscape("NETWORK %")
	response := reviewRequest(router, "GET", "/tickets?"+query+"&active=true&limit=5", token, "")
	require.Equal(t, 200, response.Code, response.Body.String())
	var page PaginatedResponse[TicketDTO]
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.Len(t, page.Data, 5)
	assert.True(t, page.Pagination.HasMore)
	for _, ticket := range page.Data {
		assert.Equal(t, "OPEN", ticket.Status)
		assert.Equal(t, agent.ID.String(), ticket.RequesterID)
	}
	response = reviewRequest(router, "GET", "/tickets?"+query+"&active=true&limit=5&offset=5", token, "")
	require.Equal(t, 200, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.Len(t, page.Data, 2)
	assert.False(t, page.Pagination.HasMore)
	countsResponse := reviewRequest(router, "GET", "/tickets/counts?"+query, token, "")
	require.Equal(t, 200, countsResponse.Code, countsResponse.Body.String())
	var counts domain.TicketCounts
	require.NoError(t, json.Unmarshal(countsResponse.Body.Bytes(), &counts))
	assert.Equal(t, int64(17), counts.All)
	assert.Equal(t, int64(14), counts.Unassigned)
	assert.Equal(t, int64(3), counts.AssignedToMe)
	assert.Equal(t, int64(7), counts.HighPriority)
	assert.Equal(t, int64(7), counts.Open)
	for _, tc := range []struct {
		filter string
		count  int64
	}{{"", counts.All}, {"unassigned=true", counts.Unassigned}, {"assigneeId=" + agent.ID.String(), counts.AssignedToMe}, {"active=true&priority=HIGH", counts.HighPriority}, {"status=OPEN", counts.Open}} {
		response = reviewRequest(router, "GET", "/tickets?limit=100&"+query+"&"+tc.filter, token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		assert.Len(t, page.Data, int(tc.count))
	}
	// Search is literal, rather than allowing SQL wildcard characters to match all rows.
	response = reviewRequest(router, "GET", "/tickets?q="+url.QueryEscape("_")+"&limit=100", token, "")
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	assert.Empty(t, page.Data)
	response = reviewRequest(router, "GET", "/tickets/counts?"+query, reviewToken(t, customer), "")
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &counts))
	assert.Zero(t, counts.All)
	future := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	response = reviewRequest(router, "GET", "/tickets/counts?"+query+"&createdFrom="+future, token, "")
	require.Equal(t, 200, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &counts))
	assert.Zero(t, counts.All)
	for _, path := range []string{"/tickets?active=invalid", "/tickets/counts?createdFrom=invalid", "/tickets?status=INVALID"} {
		assert.Equal(t, 422, reviewRequest(router, "GET", path, token, "").Code)
	}
}

func TestApplicationAdminDirectoryPagination(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	_, token := createAdminAndToken(t, ctx, org)
	otherOrg := createTestOrganization(t, ctx)
	registerUser(t, ctx, reviewAuth(otherOrg), "Directory Person Secret", uuid.NewString()+"@example.com", "customer", otherOrg)

	_, err := testPool.Exec(ctx, `WITH inserted AS (INSERT INTO users(organization_id,full_name,email,hashed_password,is_active)
 SELECT $1, 'Directory Person '||lpad(n::text,2,'0'), $2::text||n||'@example.com', 'unused-test-hash', n<=25 FROM generate_series(1,26) n RETURNING id)
 INSERT INTO user_roles(user_id,role_id) SELECT inserted.id,roles.id FROM inserted,roles WHERE roles.name='customer'`, org, uuid.NewString())
	require.NoError(t, err)
	router := newReviewRouter(t)
	var page PaginatedResponse[UserSummaryDTO]
	for _, tc := range []struct {
		offset, count int
		more          bool
	}{{0, 10, true}, {20, 5, false}, {25, 0, false}} {
		response := reviewRequest(router, "GET", fmt.Sprintf("/admin/users?q=directory&role=customer&isActive=true&limit=10&offset=%d", tc.offset), token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		assert.Len(t, page.Data, tc.count)
		assert.Equal(t, tc.more, page.Pagination.HasMore)
		for _, user := range page.Data {
			assert.True(t, user.IsActive)
			assert.Equal(t, []string{"customer"}, user.Roles)
		}
	}
	response := reviewRequest(router, "GET", "/admin/users?q=directory&isActive=false&limit=10", token, "")
	require.Equal(t, 200, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	assert.False(t, page.Data[0].IsActive)
	for _, path := range []string{"/admin/users?role=invalid", "/admin/users?isActive=invalid", "/admin/users?offset=2147483648"} {
		assert.Equal(t, 422, reviewRequest(router, "GET", path, token, "").Code)
	}
}

func TestApplicationTicketHistoryPagingAndIdentityScope(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	agent := registerUser(t, ctx, reviewAuth(org), "History Agent", uuid.NewString()+"@example.com", "agent", org)
	customer := registerUser(t, ctx, reviewAuth(org), "Unrelated Customer", uuid.NewString()+"@example.com", "customer", org)
	otherOrg := createTestOrganization(t, ctx)
	other := registerUser(t, ctx, reviewAuth(otherOrg), "Private Name", uuid.NewString()+"@example.com", "agent", otherOrg)
	router := newReviewRouter(t)
	token := reviewToken(t, agent)
	created := reviewRequest(router, "POST", "/tickets", token, `{"title":"History ticket","description":"Original","priority":"HIGH"}`)
	require.Equal(t, 201, created.Code, created.Body.String())
	var ticket TicketDTO
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &ticket))
	path := fmt.Sprintf("/tickets/%d", ticket.ID)
	require.Equal(t, 200, reviewRequest(router, "PATCH", path+"/assignee", token, fmt.Sprintf(`{"assigneeId":%q}`, agent.ID.String())).Code)
	require.Equal(t, 200, reviewRequest(router, "PATCH", path+"/status", token, `{"status":"IN_PROGRESS"}`).Code)
	require.Equal(t, 201, reviewRequest(router, "POST", path+"/comments", token, `{"body":"A real comment"}`).Code)
	require.Equal(t, 200, reviewRequest(router, "PATCH", path+"/status", token, `{"status":"CLOSED"}`).Code)
	seen := map[int64]bool{}
	before := int64(0)
	var all []TicketEventDTO
	for {
		endpoint := path + "/events?direction=backward&limit=2"
		if before > 0 {
			endpoint += fmt.Sprintf("&before=%d", before)
		}
		response := reviewRequest(router, "GET", endpoint, token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		var page TicketEventsResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		require.NotEmpty(t, page.Data)
		for i, event := range page.Data {
			assert.False(t, seen[event.ID])
			seen[event.ID] = true
			require.NotNil(t, event.Actor)
			assert.Equal(t, "History Agent", event.Actor.FullName)
			if i > 0 {
				assert.Greater(t, page.Data[i-1].ID, event.ID)
			}
			if before > 0 {
				assert.Less(t, event.ID, before)
			}
		}
		all = append(all, page.Data...)
		require.NotNil(t, page.NextCursor)
		assert.Equal(t, page.Data[len(page.Data)-1].ID, *page.NextCursor)
		if !page.HasMore {
			break
		}
		before = *page.NextCursor
	}
	require.Len(t, all, 5)
	assert.Equal(t, domain.EventStatusUpdated, all[0].Type)
	var latest domain.TicketSnapshot
	require.NoError(t, json.Unmarshal(all[0].Payload, &latest))
	assert.Equal(t, "CLOSED", latest.Status)
	var original domain.TicketSnapshot
	require.NoError(t, json.Unmarshal(all[4].Payload, &original))
	assert.Equal(t, "OPEN", original.Status)
	assert.Nil(t, original.AssigneeID)
	require.NotNil(t, all[3].Assignee)
	assert.Equal(t, "History Agent", all[3].Assignee.FullName)
	response := reviewRequest(router, "GET", fmt.Sprintf("%s/events?after=%d&limit=2", path, all[4].ID), token, "")
	require.Equal(t, 200, response.Code)
	var forward TicketEventsResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &forward))
	require.Len(t, forward.Data, 2)
	assert.True(t, forward.HasMore)
	assert.Less(t, forward.Data[0].ID, forward.Data[1].ID)
	assert.Equal(t, 403, reviewRequest(router, "GET", path+"/events?direction=backward", reviewToken(t, customer), "").Code)
	assert.Equal(t, 404, reviewRequest(router, "GET", path+"/events?direction=backward", reviewToken(t, other), "").Code)
	assert.Equal(t, 422, reviewRequest(router, "GET", path+"/events?direction=backward&after=0", token, "").Code)
	// Even inconsistent legacy foreign identities must never enrich another organization's event.
	_, err := testPool.Exec(ctx, `INSERT INTO ticket_events(ticket_id,type,payload,actor_id) VALUES($1,'TICKET_ASSIGNED',jsonb_build_object('assigneeId',$2::text),$2::uuid)`, ticket.ID, other.ID)
	require.NoError(t, err)
	response = reviewRequest(router, "GET", path+"/events?direction=backward&limit=1", token, "")
	require.Equal(t, 200, response.Code)
	forward = TicketEventsResponse{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &forward))
	require.Len(t, forward.Data, 1)
	assert.Nil(t, forward.Data[0].Actor)
	assert.Nil(t, forward.Data[0].Assignee)
	assert.NotContains(t, response.Body.String(), "Private Name")
}

func TestApplicationAuthHTTPContract(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewAuthHandler(reviewAuth(org), reviewOnboarding(), auth.NewTokenManager("test-secret", time.Hour), NewErrorHandler(logger), logger)
	router := chi.NewRouter()
	router.Route("/auth", handler.RegisterRoutes)
	address := uuid.NewString() + "@example.com"
	body := fmt.Sprintf(`{"fullName":"Public Signup","email":%q,"password":"Password1","organizationName":"New Workspace","role":"customer","organizationId":%q}`, address, org.String())
	response := reviewRequest(router, "POST", "/auth/register", "", body)
	require.Equal(t, 201, response.Code, response.Body.String())
	var registered AuthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &registered))
	require.NotNil(t, registered.User)
	assert.NotEqual(t, org.String(), registered.User.OrganizationID)
	assert.Equal(t, "New Workspace", registered.User.OrganizationName)
	assert.Equal(t, "Public Signup", registered.User.FullName)
	assert.NotEmpty(t, registered.Token)
	assert.NotContains(t, response.Body.String(), "hashedPassword")
	assert.NotContains(t, response.Body.String(), "Password1")
	profile := reviewRequest(newReviewRouter(t), "GET", "/me", registered.Token, "")
	require.Equal(t, 200, profile.Code, profile.Body.String())
	var current ProfileResponse
	require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &current))
	assert.Equal(t, []string{"admin"}, current.Roles)
	require.Equal(t, 409, reviewRequest(router, "POST", "/auth/register", "", body).Code)
	for _, tc := range []struct {
		path, body string
		status     int
	}{{"/auth/login", "{", 400}, {"/auth/login", `{"email":"invalid","password":"Password1"}`, 422}, {"/auth/login", fmt.Sprintf(`{"email":%q,"password":"wrong"}`, address), 401}, {"/auth/register", `{"fullName":"Weak","email":"weak@example.com","password":"weak"}`, 422}} {
		result := reviewRequest(router, "POST", tc.path, "", tc.body)
		assert.Equal(t, tc.status, result.Code, result.Body.String())
		assert.Contains(t, result.Header().Get("Content-Type"), "application/json")
	}
	loginBody := fmt.Sprintf(`{"email":%q,"password":"Password1"}`, address)
	require.Equal(t, 200, reviewRequest(router, "POST", "/auth/login", "", loginBody).Code)
	id, err := uuid.Parse(registered.User.ID)
	require.NoError(t, err)
	require.NoError(t, pgadapter.NewUserRepository(testPool).SetActive(ctx, id, false))
	assert.Equal(t, 403, reviewRequest(router, "POST", "/auth/login", "", loginBody).Code)
}

func TestApplicationCommentRollsBackWhenEventFails(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	user := registerUser(t, ctx, reviewAuth(org), "Commenter", uuid.NewString()+"@example.com", "customer", org)
	ticket := createTicket(t, ctx, pgadapter.NewTicketRepository(testPool), user.ID, "Atomic comment")
	// A ticket-specific real PostgreSQL failure proves comment/event atomicity.
	name := "reject_comment_event_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err := testPool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.ticket_id = %d THEN RAISE EXCEPTION 'event write rejected for regression'; END IF; RETURN NEW; END $$; CREATE TRIGGER %s BEFORE INSERT ON ticket_events FOR EACH ROW EXECUTE FUNCTION %s()`, name, ticket.ID, name, name))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, cleanupErr := testPool.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON ticket_events; DROP FUNCTION IF EXISTS %s()", name, name))
		assert.NoError(t, cleanupErr)
	})
	response := reviewRequest(newReviewRouter(t), "POST", fmt.Sprintf("/tickets/%d/comments", ticket.ID), reviewToken(t, user), `{"body":"Must not survive a failed event"}`)
	require.Equal(t, 500, response.Code, response.Body.String())
	assert.NotContains(t, response.Body.String(), "event write rejected")
	var comments, events int
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM comments WHERE ticket_id=$1", ticket.ID).Scan(&comments))
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM ticket_events WHERE ticket_id=$1", ticket.ID).Scan(&events))
	assert.Zero(t, comments)
	assert.Zero(t, events)
}

func TestApplicationTicketHistoryBeyondOneBatch(t *testing.T) {
	ctx := context.Background()
	org := createTestOrganization(t, ctx)
	user := registerUser(t, ctx, reviewAuth(org), "History batch reader", uuid.NewString()+"@example.com", "agent", org)
	ticket := createTicket(t, ctx, pgadapter.NewTicketRepository(testPool), user.ID, "Large history")
	payload, err := json.Marshal(domain.NewTicketSnapshot(ticket))
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, `INSERT INTO ticket_events(ticket_id,type,payload,actor_id) SELECT $1,'STATUS_UPDATED',$2::jsonb,$3 FROM generate_series(1,205)`, ticket.ID, payload, user.ID)
	require.NoError(t, err)
	router := newReviewRouter(t)
	token := reviewToken(t, user)
	historyIDs := map[int64]bool{}
	before := int64(0)
	for _, expected := range []struct {
		size int
		more bool
	}{{200, true}, {5, false}} {
		path := fmt.Sprintf("/tickets/%d/events?direction=backward&limit=200", ticket.ID)
		if before > 0 {
			path += fmt.Sprintf("&before=%d", before)
		}
		response := reviewRequest(router, "GET", path, token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		var page TicketEventsResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		require.Len(t, page.Data, expected.size)
		assert.Equal(t, expected.more, page.HasMore)
		for _, event := range page.Data {
			assert.False(t, historyIDs[event.ID])
			historyIDs[event.ID] = true
			if before > 0 {
				assert.Less(t, event.ID, before)
			}
		}
		require.NotNil(t, page.NextCursor)
		before = *page.NextCursor
	}
	require.Len(t, historyIDs, 205)
	after := int64(0)
	forwardCount := 0
	for _, expected := range []struct {
		size int
		more bool
	}{{200, true}, {5, false}, {0, false}} {
		response := reviewRequest(router, "GET", fmt.Sprintf("/tickets/%d/events?after=%d&limit=200", ticket.ID, after), token, "")
		require.Equal(t, 200, response.Code, response.Body.String())
		var page TicketEventsResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &page))
		require.Len(t, page.Data, expected.size)
		assert.Equal(t, expected.more, page.HasMore)
		for _, event := range page.Data {
			assert.True(t, historyIDs[event.ID])
			delete(historyIDs, event.ID)
			assert.Greater(t, event.ID, after)
			forwardCount++
		}
		if page.NextCursor != nil {
			after = *page.NextCursor
		}
	}
	assert.Equal(t, 205, forwardCount)
	assert.Empty(t, historyIDs)
}
