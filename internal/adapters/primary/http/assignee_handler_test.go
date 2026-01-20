package http

import (
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
	"github.com/lorrc/service-desk-backend/internal/core/services"
)

type assigneeListResponse struct {
	Data  []AssigneeDTO `json:"data"`
	Count int           `json:"count"`
}

func TestAssigneeList(t *testing.T) {
	ctx := context.Background()
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	userRepo := pgadapter.NewUserRepository(testPool)
	defaultOrgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	authService := services.NewAuthService(userRepo, authRepo, defaultOrgID)

	adminEmail := uuid.NewString() + "@example.com"
	adminUser, err := authService.Register(ctx, "Admin User", adminEmail, "Password1", "admin", uuid.Nil)
	require.NoError(t, err)

	agentEmail := uuid.NewString() + "@example.com"
	agentUser, err := authService.Register(ctx, "Agent User", agentEmail, "Password1", "agent", uuid.Nil)
	require.NoError(t, err)

	customerEmail := uuid.NewString() + "@example.com"
	customerUser, err := authService.Register(ctx, "Customer User", customerEmail, "Password1", "customer", uuid.Nil)
	require.NoError(t, err)

	router, tokenManager := newAssigneeRouter()
	token, err := tokenManager.GenerateToken(adminUser.ID, adminUser.OrganizationID)
	require.NoError(t, err)

	req := httptest.NewRequest(stdhttp.MethodGet, "/assignees", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var response assigneeListResponse
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, len(response.Data), response.Count)

	assertAssigneePresent(t, response.Data, adminUser.ID)
	assertAssigneePresent(t, response.Data, agentUser.ID)
	assertAssigneeMissing(t, response.Data, customerUser.ID)
}

func TestAssigneeList_Forbidden(t *testing.T) {
	ctx := context.Background()
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	userRepo := pgadapter.NewUserRepository(testPool)
	defaultOrgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	authService := services.NewAuthService(userRepo, authRepo, defaultOrgID)

	customerEmail := uuid.NewString() + "@example.com"
	customerUser, err := authService.Register(ctx, "Customer User", customerEmail, "Password1", "customer", uuid.Nil)
	require.NoError(t, err)

	router, tokenManager := newAssigneeRouter()
	token, err := tokenManager.GenerateToken(customerUser.ID, customerUser.OrganizationID)
	require.NoError(t, err)

	req := httptest.NewRequest(stdhttp.MethodGet, "/assignees", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusForbidden, recorder.Code)
}

func newAssigneeRouter() (*chi.Mux, *auth.TokenManager) {
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authzService := services.NewAuthorizationService(authRepo)
	assigneeService := services.NewAssigneeService(pgadapter.NewUserRepository(testPool), authzService)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := NewErrorHandler(logger)
	handler := NewAssigneeHandler(assigneeService, errorHandler, logger)
	tokenManager := auth.NewTokenManager("test-secret", time.Hour)

	router := chi.NewRouter()
	router.Use(mw.JWTMiddleware(tokenManager))
	router.Route("/assignees", handler.RegisterRoutes)

	return router, tokenManager
}

func assertAssigneePresent(t *testing.T, assignees []AssigneeDTO, userID uuid.UUID) {
	t.Helper()
	for _, assignee := range assignees {
		if assignee.ID == userID.String() {
			return
		}
	}
	t.Fatalf("expected assignee %s to be present", userID.String())
}

func assertAssigneeMissing(t *testing.T, assignees []AssigneeDTO, userID uuid.UUID) {
	t.Helper()
	for _, assignee := range assignees {
		if assignee.ID == userID.String() {
			t.Fatalf("expected assignee %s to be absent", userID.String())
		}
	}
}
