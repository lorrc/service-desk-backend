package postgres

import (
	"context"
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
				WithStartupTimeout(5*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("could not start postgres container: %v", err)
	}

	// 2. Set up a deferred function to terminate the container
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("could not terminate postgres container: %v", err)
		}
	}()

	// 3. Get the dynamic connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("could not get connection string: %v", err)
	}

	// 4. Run database migrations
	// We need to find the migrations directory, which is 4 levels up.
	// (postgres -> secondary -> adapters -> internal -> project root)
	migrationsPath, err := filepath.Abs("../../../../migrations")
	if err != nil {
		log.Fatalf("could not find migrations directory: %v", err)
	}

	migrationURL := "file://" + migrationsPath
	log.Printf("Running migrations from: %s\n", migrationURL)

	mig, err := migrate.New(migrationURL, connStr)
	if err != nil {
		log.Fatalf("could not create migrate instance: %v", err)
	}
	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("could not run migrations: %v", err)
	}
	log.Println("Migrations applied successfully.")

	// 5. Create the global connection pool
	testPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("could not create connection pool: %v", err)
	}

	// 6. Run the tests
	code := m.Run()

	// 7. Exit
	os.Exit(code)
}
