# Architecture

Requests flow through HTTP adapters, application services, repository ports, and PostgreSQL adapters. `cmd/api` constructs these dependencies. Domain models own validation and ticket transitions; core ports use Go values, without pgx or sqlc types.

## Authorization and organization boundaries

JWT middleware verifies the signature, expiry, and identity claims, then validates the session against PostgreSQL. Handlers obtain the actor and organization from those validated claims. Services check permissions and ticket ownership; repository reads and updates also constrain tickets to the requester's organization. Comments and events authorize through the ticket service before accessing their data.

Tickets currently inherit their organization from their requester. There is no user-transfer operation. Any future organization-transfer feature must define ticket ownership explicitly before changing a user's organization. User display information is fetched in one batch, scoped to the viewer's organization.

Ticket lists and dashboard counts share the same requester scope: customers see their own tickets, while users with `tickets:list:all` see their organization's tickets. Triage counts share search and date constraints but aggregate each tab independently of the currently selected tab. Ticket and admin lists fetch one extra row to determine `hasMore`; the API does not claim a total from page length.

## Transaction boundaries

Application services start transactions through `TransactionManager`. Repositories use the transaction attached to the context, falling back to the pool only outside a transaction.

- Registration inserts the user and role together. Failure rolls back both.
- Administrator bootstrap locks the organization row before checking or creating the account, serializing competing replicas.
- Ticket changes acquire `SELECT ... FOR UPDATE` before reading the state used for validation. The ticket update and event insertion commit together. A concurrent writer validates against the most recently committed state.
- Replacing a user's role locks that user and commits the removal, replacement, and session-version increment together. An unchanged role set is a no-op.
- Password changes compare the verified password hash during the atomic update and increment the session version in the same statement, preventing a concurrent reset from being overwritten. Sign-out-everywhere advances that version without tracking individual devices.
- Every event insertion requires a transaction and takes a per-ticket advisory transaction lock before allocating its sequence ID. The lock is held through commit or rollback, preventing a polling cursor from skipping a lower ID that commits late. Negative advisory keys separate event locks from migration locks. Keep this acquisition order consistent in future writers.

## History and polling

The event feed supports forward cursors for live updates and backward cursors for bounded history pages. Display identities are enriched in batches within the caller's organization. The frontend merges comment events with legacy comments by comment ID, so a comment appears once. Historical reads and the live polling watermark are separate. Polling pauses when hidden or offline, backs off on transient failures, and stops on authorization or missing-ticket responses.

The browser stores filters and pagination in the URL. Server data remains in session-scoped TanStack Query keys; signing out cancels and clears that cache. Permission bootstrap must complete before permission-gated pages render, with retryable failure instead of a false unauthorized result. Local comment drafts stay independent from server refetches.

Notifications happen after ticket changes commit. The current notifier logs messages; durable delivery would require a real transport and an outbox written in the same transaction as the event. Status notifications have a timeout and are drained at shutdown.

## Extension guidelines

Keep HTTP parsing in handlers, business decisions in services or domain models, and SQL conversions in the PostgreSQL adapter. Preserve organization constraints and transaction context when adding new repository methods. Test database constraints and concurrency against PostgreSQL, and test pure validation and address parsing without external services.
