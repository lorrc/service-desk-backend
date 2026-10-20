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

JWTs issued before session-version support will be rejected, so affected users must sign in again. Every authenticated request checks the user's organization, active status, and session version in PostgreSQL. Password changes/resets, sign-out-everywhere, active-status changes, and actual role changes advance the version. Reapplying the same role preserves sessions. Deploy all API replicas together; replicas predating session-version validation do not enforce revocation.

By default, rate limiting uses the socket peer address and ignores forwarding headers. If a reverse proxy terminates connections, set `TRUSTED_PROXY_CIDRS` to a comma-separated list of that proxy's IP ranges. The proxy must append or overwrite forwarded addresses correctly. Forwarded chains are walked from the nearest hop to the first untrusted address. Do not trust every network merely to enable per-client limits.

## Validation

Fast tests for the changed authentication, ticket, and client-address behavior:

```sh
go test -race ./internal/core/services ./internal/auth ./internal/adapters/primary/http/middleware -run 'TestAuthService_|TestTicketService_|TestTokenManager|TestClientIP|TestRateLimit' -count=1
```

HTTP and repository regressions use disposable PostgreSQL containers and apply the real migrations:

```sh
go test -race ./migrations -count=1 -timeout=3m
go test -race ./internal/adapters/primary/http ./internal/adapters/secondary/postgres -run 'TestApplication|TestReview|TestAdmin|TestMePermissions|TestAssignee|TestTicketRepository|TestUserRepository|TestTicketEvent' -count=1 -timeout=4m
go build ./...
```

The regressions cover organization isolation, revoked sessions, registration rollback, concurrent administrator bootstrap, competing ticket changes, search/count parity, profile security, event pagination and ordering, and migration upgrade/rollback behavior. GitHub Actions runs the affected packages with race detection.

For browser tests, keep the frontend and backend as sibling directories and run `npm run test:e2e` from the frontend after installing its Chromium browser. The build-tagged `test/e2e` command starts disposable PostgreSQL, migrates and seeds it, then runs the real API on `127.0.0.1:18080`. It overrides the database and JWT environment for that child process; it does not migrate the developer's database.

## Application API

- `GET /tickets` accepts `q` (title, description, or ticket ID), `active=true` (non-closed), existing filters, and `limit`/`offset`. `GET /tickets/counts` returns all/unassigned/assigned-to-me/status/high-priority counts within the authorized scope and shared search/date filters. High priority counts active HIGH tickets.
- `GET /admin/users` accepts `q`, `role`, `isActive`, `limit`, and `offset`. Its response contains `data`, the returned page `count`, and `pagination.hasMore`, not an invented total.
- `GET /tickets/:id/events` keeps forward `after` pagination for polling. `direction=backward&before=<id>` retrieves older history newest first; omit `before` for the newest page. Both directions expose `hasMore` and `nextCursor`. Actor identities are resolved within the requesting organization.
- `GET /me` returns the authenticated profile, organization, roles, permissions, and sign-in metadata. `POST /me/password` accepts `currentPassword` and `newPassword`; successful password change revokes every current session. `POST /me/sign-out-everywhere` also revokes the calling token. The next request requires sign-in.

## Migration rollback limits

Migrations 2 and 3 now have reverse scripts. Migration 4 preserves indexes owned by migration 2. Migration 6 preserves grants because its backfill cannot distinguish existing grants from grants it added; migration 5 owns removal of its new permission.

Migration 11 is a no-op on the canonical BIGINT comment schema. A legacy UUID-to-BIGINT conversion discards the original UUIDs and is intentionally irreversible. Its down migration rejects that case without changing IDs. Restoring the original UUIDs requires a pre-conversion backup. A rejected migration may leave golang-migrate's version dirty; inspect the schema and migration history before manual recovery, rather than forcing the version blindly.

Edits to an already-applied migration do not run again on an existing database. Back up and verify the deployed schema before choosing a rollback; the regression suite uses disposable databases only.

After editing SQL queries or migrations, regenerate the checked-in database code with sqlc 1.30.0 using `sqlc generate`. See [ARCHITECTURE.md](ARCHITECTURE.md) for dependency and transaction boundaries.
