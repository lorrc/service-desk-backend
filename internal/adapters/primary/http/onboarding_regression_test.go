package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	stdhttp "net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgadapter "github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func onboardingRouter(t *testing.T) stdhttp.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewAuthHandler(reviewAuth(uuid.MustParse("00000000-0000-0000-0000-000000000001")), reviewOnboarding(), auth.NewTokenManager("test-secret", time.Hour), NewErrorHandler(logger), logger)
	router := chi.NewRouter()
	router.Route("/auth", handler.RegisterRoutes)
	router.Mount("/", newReviewRouter(t))
	return router
}

func founder(t *testing.T, router stdhttp.Handler, name string) AuthResponse {
	t.Helper()
	body := fmt.Sprintf(`{"fullName":"Founder","email":%q,"password":"Password1","organizationName":%q}`, uuid.NewString()+"@example.com", name)
	response := reviewRequest(router, "POST", "/auth/register", "", body)
	require.Equal(t, 201, response.Code, response.Body.String())
	var registered AuthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &registered))
	return registered
}

type issuedInvitation struct {
	Invitation InvitationDTO `json:"invitation"`
	Token      string        `json:"token"`
}

func issueInvitation(t *testing.T, router stdhttp.Handler, adminToken, email string) issuedInvitation {
	t.Helper()
	response := reviewRequest(router, "POST", "/admin/invitations", adminToken, fmt.Sprintf(`{"email":%q,"role":"admin"}`, email))
	require.Equal(t, 201, response.Code, response.Body.String())
	assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	var issued issuedInvitation
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &issued))
	require.Len(t, issued.Token, 43)
	assert.Equal(t, "pending", issued.Invitation.Status)
	return issued
}

func joinBody(email, token string) string {
	return fmt.Sprintf(`{"fullName":"Invited User","email":%q,"password":"Password1","invitationToken":%q,"role":"admin","organizationId":%q}`, email, token, uuid.NewString())
}

func TestOnboardingFounderAndInvitationFlow(t *testing.T) {
	ctx := context.Background()
	router := onboardingRouter(t)
	email := "Founder-" + uuid.NewString() + "@Example.COM"
	existingOrg := createTestOrganization(t, ctx)
	name := "Workspace " + uuid.NewString()
	body := fmt.Sprintf(`{"fullName":"Founder","email":%q,"password":"Password1","organizationName":%q,"role":"customer","organizationId":%q}`, email, name, existingOrg)
	response := reviewRequest(router, "POST", "/auth/register", "", body)
	require.Equal(t, 201, response.Code, response.Body.String())
	var owner AuthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &owner))
	assert.NotEqual(t, existingOrg.String(), owner.User.OrganizationID)
	assert.Equal(t, name, owner.User.OrganizationName)
	assert.Equal(t, strings.ToLower(email), owner.User.Email)
	profile := reviewRequest(router, "GET", "/me", owner.Token, "")
	require.Equal(t, 200, profile.Code, profile.Body.String())
	var identity ProfileResponse
	require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &identity))
	assert.Equal(t, []string{"admin"}, identity.Roles)
	assert.Contains(t, identity.Permissions, "admin:access")
	assert.Equal(t, name, identity.OrganizationName)
	login := reviewRequest(router, "POST", "/auth/login", "", fmt.Sprintf(`{"email":%q,"password":"Password1"}`, email))
	require.Equal(t, 200, login.Code, login.Body.String())
	assert.Contains(t, login.Body.String(), name)

	for _, extra := range []string{"", `,"organizationName":" "`, `,"organizationName":"Both","invitationToken":"token"`} {
		request := fmt.Sprintf(`{"fullName":"Invalid","email":%q,"password":"Password1"%s}`, uuid.NewString()+"@example.com", extra)
		assert.Equal(t, 422, reviewRequest(router, "POST", "/auth/register", "", request).Code)
	}

	recipient := "Member-" + uuid.NewString() + "@Example.COM"
	issued := issueInvitation(t, router, owner.Token, "  "+recipient+"  ")
	assert.Equal(t, strings.ToLower(recipient), issued.Invitation.Email)
	var hash []byte
	var lifetime float64
	require.NoError(t, testPool.QueryRow(ctx, `SELECT token_hash, extract(epoch from expires_at-created_at) FROM organization_invitations WHERE id=$1`, issued.Invitation.ID).Scan(&hash, &lifetime))
	require.Len(t, hash, 32)
	assert.NotEqual(t, []byte(issued.Token), hash)
	assert.Equal(t, float64(7*24*60*60), lifetime)
	preview := reviewRequest(router, "POST", "/auth/invitations/preview", "", fmt.Sprintf(`{"token":%q}`, issued.Token))
	require.Equal(t, 200, preview.Code, preview.Body.String())
	assert.Equal(t, "no-store", preview.Header().Get("Cache-Control"))
	assert.Contains(t, preview.Body.String(), name)
	assert.Contains(t, preview.Body.String(), strings.ToLower(recipient))
	assert.NotContains(t, preview.Body.String(), issued.Token)

	wrong := reviewRequest(router, "POST", "/auth/register", "", joinBody(uuid.NewString()+"@example.com", issued.Token))
	require.Equal(t, 422, wrong.Code, wrong.Body.String())
	assert.Contains(t, wrong.Body.String(), "invitationToken")
	response = reviewRequest(router, "POST", "/auth/register", "", joinBody("  "+recipient+"  ", issued.Token))
	require.Equal(t, 201, response.Code, response.Body.String())
	var member AuthResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &member))
	assert.Equal(t, owner.User.OrganizationID, member.User.OrganizationID)
	assert.Equal(t, name, member.User.OrganizationName)
	profile = reviewRequest(router, "GET", "/me", member.Token, "")
	require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &identity))
	assert.Equal(t, []string{"customer"}, identity.Roles)
	assert.NotContains(t, identity.Permissions, "admin:access")
	assert.Equal(t, 403, reviewRequest(router, "POST", "/admin/invitations", member.Token, `{"email":"unauthorized@example.com"}`).Code)
	assert.Equal(t, 403, reviewRequest(router, "GET", "/admin/invitations", member.Token, "").Code)
	assert.Equal(t, 422, reviewRequest(router, "POST", "/auth/register", "", joinBody(recipient, issued.Token)).Code)
	assert.Equal(t, 422, reviewRequest(router, "POST", "/auth/invitations/preview", "", fmt.Sprintf(`{"token":%q}`, issued.Token)).Code)
	assert.Equal(t, 204, reviewRequest(router, "DELETE", "/admin/invitations/"+issued.Invitation.ID, owner.Token, "").Code)
	var accepted, revoked bool
	require.NoError(t, testPool.QueryRow(ctx, "SELECT accepted_at IS NOT NULL, revoked_at IS NOT NULL FROM organization_invitations WHERE id=$1", issued.Invitation.ID).Scan(&accepted, &revoked))
	assert.True(t, accepted)
	assert.False(t, revoked)
	history := reviewRequest(router, "GET", "/admin/invitations", owner.Token, "")
	assert.Contains(t, history.Body.String(), `"status":"accepted"`)
	assert.NotContains(t, history.Body.String(), issued.Token)
	assert.NotContains(t, history.Body.String(), "tokenHash")

	// The canonical lookup also refuses a pre-existing legacy mixed-case account.
	legacy := registerUser(t, ctx, reviewAuth(existingOrg), "Legacy", "Legacy-"+uuid.NewString()+"@Example.COM", "customer", existingOrg)
	duplicateName := "Duplicate " + uuid.NewString()
	duplicate := fmt.Sprintf(`{"fullName":"Duplicate","email":%q,"password":"Password1","organizationName":%q}`, strings.ToLower(legacy.Email), duplicateName)
	assert.Equal(t, 409, reviewRequest(router, "POST", "/auth/register", "", duplicate).Code)
	var count int
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM organizations WHERE name=$1", duplicateName).Scan(&count))
	assert.Zero(t, count, "failed duplicate signup must not leave an organization")
	_, err := reviewAuth(existingOrg).Login(ctx, legacy.Email, "Password1")
	require.NoError(t, err, "legacy exact-case login remains available")
}

func TestOnboardingInvitationIsolationExpiryAndPagination(t *testing.T) {
	ctx := context.Background()
	router := onboardingRouter(t)
	owner := founder(t, router, "Invitations "+uuid.NewString())
	other := founder(t, router, "Other "+uuid.NewString())
	first := issueInvitation(t, router, owner.Token, uuid.NewString()+"@example.com")
	second := issueInvitation(t, router, owner.Token, uuid.NewString()+"@example.com")
	expired := issueInvitation(t, router, owner.Token, uuid.NewString()+"@example.com")
	assert.Equal(t, 404, reviewRequest(router, "DELETE", "/admin/invitations/"+first.Invitation.ID, other.Token, "").Code)
	for _, method := range []string{"GET", "POST", "DELETE"} {
		path := "/admin/invitations"
		if method == "DELETE" {
			path += "/" + first.Invitation.ID
		}
		assert.Equal(t, 401, reviewRequest(router, method, path, "", `{"email":"missing@example.com"}`).Code)
	}
	otherList := reviewRequest(router, "GET", "/admin/invitations", other.Token, "")
	require.Equal(t, 200, otherList.Code)
	assert.NotContains(t, otherList.Body.String(), first.Invitation.Email)
	assert.Contains(t, otherList.Body.String(), `"data":[]`)
	actorID := uuid.MustParse(owner.User.ID)
	otherOrg := uuid.MustParse(other.User.OrganizationID)
	_, err := reviewOnboarding().ListInvitations(ctx, actorID, otherOrg, 25, 0)
	require.ErrorIs(t, err, apperrors.ErrForbidden, "service itself must validate actor organization")

	for range 2 {
		assert.Equal(t, 204, reviewRequest(router, "DELETE", "/admin/invitations/"+first.Invitation.ID, owner.Token, "").Code)
	}
	assert.Equal(t, 422, reviewRequest(router, "POST", "/auth/register", "", joinBody(first.Invitation.Email, first.Token)).Code)
	_, err = testPool.Exec(ctx, `UPDATE organization_invitations SET created_at=now()-interval '8 days', expires_at=now()-interval '1 day' WHERE id=$1`, expired.Invitation.ID)
	require.NoError(t, err)
	assert.Equal(t, 422, reviewRequest(router, "POST", "/auth/register", "", joinBody(expired.Invitation.Email, expired.Token)).Code)
	for _, token := range []string{first.Token, expired.Token, "malformed", strings.Repeat("A", 43)} {
		result := reviewRequest(router, "POST", "/auth/invitations/preview", "", fmt.Sprintf(`{"token":%q}`, token))
		require.Equal(t, 422, result.Code, result.Body.String())
		assert.Contains(t, result.Body.String(), "This invitation is invalid or no longer available")
		assert.NotContains(t, result.Body.String(), token)
	}
	var page struct {
		Data       []InvitationDTO    `json:"data"`
		Count      int                `json:"count"`
		Pagination PaginationMetadata `json:"pagination"`
	}
	pageResponse := reviewRequest(router, "GET", "/admin/invitations?limit=1", owner.Token, "")
	require.NoError(t, json.Unmarshal(pageResponse.Body.Bytes(), &page))
	require.Len(t, page.Data, 1)
	assert.Equal(t, 1, page.Count)
	assert.True(t, page.Pagination.HasMore)
	firstPageID := page.Data[0].ID
	pageResponse = reviewRequest(router, "GET", "/admin/invitations?limit=1&offset=1", owner.Token, "")
	require.NoError(t, json.Unmarshal(pageResponse.Body.Bytes(), &page))
	assert.NotEqual(t, firstPageID, page.Data[0].ID)
	all := reviewRequest(router, "GET", "/admin/invitations?limit=100", owner.Token, "")
	require.NoError(t, json.Unmarshal(all.Body.Bytes(), &page))
	require.Len(t, page.Data, 3)
	assert.False(t, page.Pagination.HasMore)
	assert.Contains(t, all.Body.String(), `"status":"revoked"`)
	assert.Contains(t, all.Body.String(), `"status":"expired"`)
	assert.Contains(t, all.Body.String(), second.Invitation.ID)
	assert.Equal(t, 422, reviewRequest(router, "GET", "/admin/invitations?offset=2147483648", owner.Token, "").Code)
}

func TestOnboardingConcurrentRegistration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	router := onboardingRouter(t)
	owner := founder(t, router, "Concurrent "+uuid.NewString())
	issued := issueInvitation(t, router, owner.Token, uuid.NewString()+"@example.com")
	results := make(chan int, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			results <- reviewRequest(router, "POST", "/auth/register", "", joinBody(issued.Invitation.Email, issued.Token)).Code
		}()
	}
	close(start)
	statuses := []int{<-results, <-results}
	assert.ElementsMatch(t, []int{201, 422}, statuses)
	var count int
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM users WHERE email=$1", issued.Invitation.Email).Scan(&count))
	assert.Equal(t, 1, count)
	require.NoError(t, testPool.QueryRow(ctx, `SELECT count(*) FROM user_roles ur JOIN users u ON u.id=ur.user_id JOIN roles r ON r.id=ur.role_id WHERE u.email=$1 AND r.name='customer'`, issued.Invitation.Email).Scan(&count))
	assert.Equal(t, 1, count)

	address, name := uuid.NewString()+"@example.com", "First signup "+uuid.NewString()
	start = make(chan struct{})
	for range 2 {
		go func() {
			<-start
			result := reviewRequest(router, "POST", "/auth/register", "", fmt.Sprintf(`{"fullName":"Founder","email":%q,"password":"Password1","organizationName":%q}`, address, name))
			results <- result.Code
		}()
	}
	close(start)
	assert.ElementsMatch(t, []int{201, 409}, []int{<-results, <-results})
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM organizations WHERE name=$1", name).Scan(&count))
	assert.Equal(t, 1, count, "losing founder transaction must roll back its organization")
}

func TestOnboardingRegistrationRollback(t *testing.T) {
	ctx := context.Background()
	router := onboardingRouter(t)
	owner := founder(t, router, "Rollback "+uuid.NewString())
	address := uuid.NewString() + "@example.com"
	issued := issueInvitation(t, router, owner.Token, address)
	function := pgx.Identifier{"reject_role_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	trigger := pgx.Identifier{"reject_role_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	_, err := testPool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF EXISTS (SELECT 1 FROM users WHERE id=NEW.user_id AND email='%s') THEN
				RAISE EXCEPTION 'forced onboarding role failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER %s BEFORE INSERT ON user_roles FOR EACH ROW EXECUTE FUNCTION %s();`, function, address, trigger, function))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := testPool.Exec(context.Background(), fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON user_roles; DROP FUNCTION IF EXISTS %s();", trigger, function))
		require.NoError(t, err)
	})
	failed := reviewRequest(router, "POST", "/auth/register", "", joinBody(address, issued.Token))
	require.Equal(t, 500, failed.Code, failed.Body.String())
	var accepted bool
	require.NoError(t, testPool.QueryRow(ctx, "SELECT accepted_at IS NOT NULL FROM organization_invitations WHERE id=$1", issued.Invitation.ID).Scan(&accepted))
	assert.False(t, accepted)
	_, err = pgadapter.NewUserRepository(testPool).GetByEmail(ctx, address)
	require.ErrorIs(t, err, apperrors.ErrUserNotFound)
	name := "Failed founder " + uuid.NewString()
	failed = reviewRequest(router, "POST", "/auth/register", "", fmt.Sprintf(`{"fullName":"Founder","email":%q,"password":"Password1","organizationName":%q}`, address, name))
	require.Equal(t, 500, failed.Code, failed.Body.String())
	var count int
	require.NoError(t, testPool.QueryRow(ctx, "SELECT count(*) FROM organizations WHERE name=$1", name).Scan(&count))
	assert.Zero(t, count)
	_, err = testPool.Exec(ctx, fmt.Sprintf("DROP TRIGGER %s ON user_roles; DROP FUNCTION %s();", trigger, function))
	require.NoError(t, err)
	response := reviewRequest(router, "POST", "/auth/register", "", joinBody(address, issued.Token))
	require.Equal(t, 201, response.Code, response.Body.String())

	// An account created elsewhere after an invitation was issued must not be transferred.
	address = uuid.NewString() + "@example.com"
	issued = issueInvitation(t, router, owner.Token, address)
	otherOrg := createTestOrganization(t, ctx)
	registerUser(t, ctx, reviewAuth(otherOrg), "Existing elsewhere", address, "customer", otherOrg)
	response = reviewRequest(router, "POST", "/auth/register", "", joinBody(address, issued.Token))
	require.Equal(t, 409, response.Code, response.Body.String())
	require.NoError(t, testPool.QueryRow(ctx, "SELECT accepted_at IS NOT NULL FROM organization_invitations WHERE id=$1", issued.Invitation.ID).Scan(&accepted))
	assert.False(t, accepted)
}

func TestOnboardingRevocationWinsWaitingAcceptance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	router := onboardingRouter(t)
	owner := founder(t, router, "Revocation "+uuid.NewString())
	issued := issueInvitation(t, router, owner.Token, uuid.NewString()+"@example.com")
	tx, err := testPool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "SELECT id FROM organization_invitations WHERE id=$1 FOR UPDATE", issued.Invitation.ID)
	require.NoError(t, err)
	complete := make(chan error, 1)
	go func() {
		_, err := reviewOnboarding().Register(ctx, ports.OnboardingParams{FullName: "Waiting", Email: issued.Invitation.Email, Password: "Password1", InvitationToken: issued.Token})
		complete <- err
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := testPool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%organization_invitations%' AND query LIKE '%FOR UPDATE%'`).Scan(&waiting)
		return err == nil && waiting > 0
	}, 5*time.Second, 10*time.Millisecond)
	_, err = tx.Exec(ctx, "UPDATE organization_invitations SET revoked_at=clock_timestamp() WHERE id=$1", issued.Invitation.ID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	var invalid *apperrors.ValidationErrors
	require.ErrorAs(t, <-complete, &invalid)
	assert.Contains(t, invalid.Errors, "invitationToken")
	_, err = pgadapter.NewUserRepository(testPool).GetByEmail(ctx, issued.Invitation.Email)
	require.ErrorIs(t, err, apperrors.ErrUserNotFound)
}
