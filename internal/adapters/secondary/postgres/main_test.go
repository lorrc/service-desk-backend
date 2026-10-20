package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testPool is a global connection pool used by all tests in this package.
var testPool *pgxpool.Pool

// TestMain sets up and tears down the test database container.
func TestMain(m *testing.M) {
	code, err := runRepositoryTests(m)
	if err != nil {
		log.Printf("repository test setup or cleanup failed: %v", err)
		code = 1
	}
	os.Exit(code)
}

func runRepositoryTests(m *testing.M) (code int, err error) {
	ctx := context.Background()

	// 1. Start a PostgreSQL container
	log.Println("Setting up PostgreSQL container...")
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("test-db"),
		postgres.WithUsername("user"),
		postgres.WithPassword("password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2). // Wait for it to be ready
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return 1, fmt.Errorf("start postgres container: %w", err)
	}

	// 2. Set up a deferred function to terminate the container
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if cleanupErr := pgContainer.Terminate(cleanupCtx); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("terminate postgres container: %w", cleanupErr))
		}
	}()

	// 3. Get the dynamic connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 1, fmt.Errorf("get connection string: %w", err)
	}

	// 4. Run database migrations
	// We need to find the migrations directory, which is 4 levels up.
	// (postgres -> secondary -> adapters -> internal -> project root)
	migrationsPath, err := filepath.Abs("../../../../migrations")
	if err != nil {
		return 1, fmt.Errorf("find migrations directory: %w", err)
	}

	migrationURL := "file://" + migrationsPath
	log.Printf("Running migrations from: %s\n", migrationURL)

	mig, err := migrate.New(migrationURL, connStr)
	if err != nil {
		return 1, fmt.Errorf("create migrate instance: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := mig.Close()
		err = errors.Join(err, sourceErr, databaseErr)
	}()
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return 1, fmt.Errorf("run migrations: %w", err)
	}
	log.Println("Migrations applied successfully.")

	// 5. Create the global connection pool
	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		return 1, fmt.Errorf("create connection pool: %w", err)
	}
	defer testPool.Close()

	// 6. Run the tests
	return m.Run(), nil
}
