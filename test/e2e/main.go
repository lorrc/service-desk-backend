//go:build e2e

// Command e2e runs the real API against a disposable, seeded PostgreSQL database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/testcontainers/testcontainers-go"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	startup, stopStartup := context.WithTimeout(ctx, 2*time.Minute)
	defer stopStartup()
	container, err := pgcontainer.Run(startup, "postgres:16-alpine",
		pgcontainer.WithDatabase("service_desk_e2e"),
		pgcontainer.WithUsername("e2e"),
		pgcontainer.WithPassword("disposable-test-password"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		return fmt.Errorf("start test database: %w", err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 20*time.Second)
		defer done()
		if err := container.Terminate(cleanup); err != nil {
			log.Printf("terminate test database: %v", err)
		}
	}()
	connection, err := container.ConnectionString(startup, "sslmode=disable")
	if err != nil {
		return err
	}
	migrations, err := filepath.Abs("migrations")
	if err != nil {
		return err
	}
	migrator, err := migrate.New("file://"+migrations, connection)
	if err != nil {
		return err
	}
	defer migrator.Close()
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate test database: %w", err)
	}
	pool, err := pgxpool.New(startup, connection)
	if err != nil {
		return err
	}
	if err := seed(startup, pool); err != nil {
		pool.Close()
		return fmt.Errorf("seed test database: %w", err)
	}
	pool.Close()

	dir, err := os.MkdirTemp("", "service-desk-e2e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "api")
	build := exec.CommandContext(startup, "go", "build", "-mod=readonly", "-o", binary, "./cmd/api")
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build API: %w", err)
	}
	api := exec.CommandContext(ctx, binary)
	api.Stdout, api.Stderr = os.Stdout, os.Stderr
	api.Env = append(os.Environ(),
		"DATABASE_URL="+connection,
		"SERVER_PORT=127.0.0.1:18080",
		"JWT_SECRET=disposable-e2e-signing-key-not-for-deployment",
		"JWT_ACCESS_TOKEN_TTL=1h",
		"APP_ENV=test",
		"ADMIN_EMAIL=", "ADMIN_PASSWORD=",
		"DEFAULT_ORG_ID=00000000-0000-0000-0000-000000000001",
		"RATE_LIMIT_ENABLED=false", "TRUSTED_PROXY_CIDRS=", "LOG_LEVEL=warn",
	)
	api.Cancel = func() error { return api.Process.Signal(syscall.SIGTERM) }
	api.WaitDelay = 15 * time.Second
	err = api.Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := domain.HashPassword("E2ePassword1!")
	if err != nil {
		return err
	}
	org := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	var customer uuid.UUID
	for _, role := range []string{"admin", "agent", "customer"} {
		var id uuid.UUID
		err := pool.QueryRow(ctx, `INSERT INTO users (organization_id,full_name,email,hashed_password) VALUES ($1,$2,$3,$4) RETURNING id`, org, "E2E "+role, role+"@example.test", hash).Scan(&id)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id,role_id) SELECT $1,id FROM roles WHERE name=$2`, id, role); err != nil {
			return err
		}
		if role == "customer" {
			customer = id
		}
	}
	// More than 100 newer closed tickets must not hide this active request.
	if _, err := pool.Exec(ctx, `INSERT INTO tickets (title,status,priority,requester_id,created_at) VALUES ('Older active support request','OPEN','HIGH',$1,NOW()-INTERVAL '7 days')`, customer); err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `INSERT INTO tickets (title,status,priority,requester_id,closed_at) SELECT 'Archived request ' || n,'CLOSED','LOW',$1,NOW() FROM generate_series(1,105) n`, customer)
	return err
}
