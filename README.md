# Service Desk Backend

Go HTTP API for organization-scoped support tickets, comments, event feeds, and user administration. Requires Go 1.25 and PostgreSQL 16. Docker is required for database regression tests.

## Local setup

1. Copy `.env.example` to `.env` and set `DATABASE_URL` and `JWT_SECRET`.
2. Apply the SQL migrations with the golang-migrate CLI:

   ```sh
   migrate -path migrations -database "$DATABASE_URL" up
   ```

   The CLI reads the exported environment variable. The API loads `.env` itself, but does not run migrations on startup.
3. Configure `ADMIN_EMAIL`, `ADMIN_PASSWORD`, `ADMIN_FIRST_NAME`, and `ADMIN_LAST_NAME` to provision an administrator. Public registration always creates customers. Bootstrap is atomic and safe across replicas; an existing account keeps its current password and active status.
4. Run `go run ./cmd/api`. The API listens on `:8080` by default, under `/api/v1`. Readiness is available at `/health/ready`.

The migrations create a default organization with ID `00000000-0000-0000-0000-000000000001`. A custom `DEFAULT_ORG_ID` must refer to an existing organization.

## Session and proxy rollout

Apply migration `000012_user_session_version` before starting this version of the API, then deploy the backend before the frontend. The frontend now uses `GET /api/v1/tickets/counts` for dashboard totals.

Existing JWTs do not contain a session version and will be rejected. Users must sign in again. Every authenticated request checks the user's organization, active status, and session version in PostgreSQL. Password resets and active-status changes advance the version, so reactivating an account does not restore its old tokens. Deploy all API replicas together; older replicas do not enforce revocation.

By default, rate limiting uses the socket peer address and ignores forwarding headers. If a reverse proxy terminates connections, set `TRUSTED_PROXY_CIDRS` to a comma-separated list of that proxy's IP ranges. The proxy must append or overwrite forwarded addresses correctly. Forwarded chains are walked from the nearest hop to the first untrusted address. Do not trust every network merely to enable per-client limits.

## Validation

Fast tests for the changed authentication, ticket, and client-address behavior:

```sh
go test -race ./internal/core/services ./internal/auth ./internal/adapters/primary/http/middleware -run 'TestAuthService_|TestTicketService_|TestTokenManager|TestClientIP|TestRateLimit' -count=1
```

HTTP and repository regressions use disposable PostgreSQL containers and apply the real migrations:

```sh
go test -race ./internal/adapters/primary/http ./internal/adapters/secondary/postgres -run 'TestReview|TestAdmin|TestMePermissions|TestAssignee|TestTicketRepository' -count=1 -timeout=3m
go build ./...
```

The regressions cover organization isolation, revoked sessions, registration rollback, concurrent administrator bootstrap, competing ticket changes, dashboard totals, and pagination boundaries.

After editing SQL queries or migrations, regenerate the checked-in database code with sqlc 1.30.0 using `sqlc generate`. See [ARCHITECTURE.md](ARCHITECTURE.md) for dependency and transaction boundaries.
