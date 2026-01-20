package http

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	pgadapter "github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres"
	"github.com/lorrc/service-desk-backend/internal/auth"
	"github.com/lorrc/service-desk-backend/internal/core/services"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := pgcontainer.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		pgcontainer.WithDatabase("test-db"),
		pgcontainer.WithUsername("user"),
		pgcontainer.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("could not get connection string: %v", err)
	}

	migrationsPath, err := filepath.Abs("../../../../migrations")
	if err != nil {
		log.Fatalf("could not find migrations directory: %v", err)
	}

	migrationURL := "file://" + migrationsPath
	mig, err := migrate.New(migrationURL, connStr)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("could not run migrations: %v", err)
	}

	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("could not create connection pool: %v", err)
	}

	code := m.Run()

	testPool.Close()
	if err := pgContainer.Terminate(ctx); err != nil {
		log.Fatalf("could not terminate postgres container: %v", err)
	}

	os.Exit(code)
}

func TestMePermissions(t *testing.T) {
	ctx := context.Background()
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	userRepo := pgadapter.NewUserRepository(testPool)
	defaultOrgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	authService := services.NewAuthService(userRepo, authRepo, defaultOrgID)

	userEmail := uuid.NewString() + "@example.com"
	user, err := authService.Register(ctx, "Test User", userEmail, "Password1", "admin", uuid.Nil)
	require.NoError(t, err)

	router, tokenManager := newMeRouter()
	token, err := tokenManager.GenerateToken(user.ID, user.OrganizationID)
	require.NoError(t, err)

	req := httptest.NewRequest(stdhttp.MethodGet, "/me/permissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusOK, recorder.Code)

	var response PermissionsResponse
	err = json.NewDecoder(recorder.Body).Decode(&response)
	require.NoError(t, err)
	require.NotEmpty(t, response.Permissions)
	assert.Contains(t, response.Permissions, "tickets:create")
	assert.Contains(t, response.Permissions, "tickets:assign")

	sorted := append([]string(nil), response.Permissions...)
	sort.Strings(sorted)
	assert.Equal(t, sorted, response.Permissions)
}

func TestMePermissions_Unauthorized(t *testing.T) {
	router, _ := newMeRouter()

	req := httptest.NewRequest(stdhttp.MethodGet, "/me/permissions", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	require.Equal(t, stdhttp.StatusUnauthorized, recorder.Code)
}

func newMeRouter() (*chi.Mux, *auth.TokenManager) {
	authRepo := pgadapter.NewAuthorizationRepository(testPool)
	authzService := services.NewAuthorizationService(authRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	errorHandler := NewErrorHandler(logger)
	meHandler := NewMeHandler(authzService, errorHandler, logger)
	tokenManager := auth.NewTokenManager("test-secret", time.Hour)

	router := chi.NewRouter()
	router.Use(mw.JWTMiddleware(tokenManager))
	router.Route("/me", meHandler.RegisterRoutes)

	return router, tokenManager
}
