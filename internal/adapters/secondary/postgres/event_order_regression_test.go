package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/stretchr/testify/require"
)

func TestTicketEventRepositoryRequiresTransaction(t *testing.T) {
	_, err := NewTicketEventRepository(testPool).Create(t.Context(), &domain.Event{})
	require.ErrorContains(t, err, "requires a transaction")
}

func TestTicketEventRepositorySerializesCommittedCursors(t *testing.T) {
	for _, commitFirst := range []bool{true, false} {
		name := "commit releases the next writer"
		if !commitFirst {
			name = "rollback releases the next writer"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			tickets, users := newTestRepos(t)
			user := createTestUser(t, ctx, users)
			ticket, err := tickets.Create(ctx, &domain.Ticket{
				Title: "Ordered events", RequesterID: user.ID, Status: domain.StatusOpen, Priority: domain.PriorityMedium,
			})
			require.NoError(t, err)
			otherTicket, err := tickets.Create(ctx, &domain.Ticket{
				Title: "Independent events", RequesterID: user.ID, Status: domain.StatusOpen, Priority: domain.PriorityMedium,
			})
			require.NoError(t, err)
			events := NewTicketEventRepository(testPool)
			firstTx, err := testPool.Begin(ctx)
			require.NoError(t, err)
			defer firstTx.Rollback(context.Background())
			firstCtx := ContextWithTx(ctx, firstTx)
			// Real comment inserts take compatible FK key-share locks before their
			// events are written. Event ordering must not upgrade those row locks.
			_, err = firstTx.Exec(ctx, "INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'first')", ticket.ID, user.ID)
			require.NoError(t, err)
			first, err := events.Create(firstCtx, &domain.Event{
				TicketID: ticket.ID, ActorID: user.ID, Type: domain.EventCommentAdded, Payload: json.RawMessage(`{"body":"first"}`),
			})
			require.NoError(t, err)

			secondTx, err := testPool.Begin(ctx)
			require.NoError(t, err)
			defer secondTx.Rollback(context.Background())
			secondPID := secondTx.Conn().PgConn().PID()
			_, err = secondTx.Exec(ctx, "INSERT INTO comments (ticket_id, author_id, body) VALUES ($1, $2, 'second')", ticket.ID, user.ID)
			require.NoError(t, err)
			type writeResult struct {
				event *domain.Event
				err   error
			}
			result := make(chan writeResult, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				created, createErr := events.Create(ContextWithTx(ctx, secondTx), &domain.Event{
					TicketID: ticket.ID, ActorID: user.ID, Type: domain.EventCommentAdded, Payload: json.RawMessage(`{"body":"second"}`),
				})
				if createErr == nil {
					createErr = secondTx.Commit(ctx)
				}
				result <- writeResult{event: created, err: createErr}
			}()
			defer func() {
				cancel()
				<-finished
			}()
			require.Eventually(t, func() bool {
				var waiting bool
				err := testPool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity
					WHERE pid = $1 AND wait_event_type = 'Lock' AND wait_event = 'advisory')`, secondPID).Scan(&waiting)
				return err == nil && waiting
			}, 2*time.Second, 5*time.Millisecond)
			visible, err := events.ListByTicketID(ctx, ticket.ID, 0, 10)
			require.NoError(t, err)
			require.Empty(t, visible, "a later event must not become visible ahead of the uncommitted first event")

			// Unrelated tickets must be able to commit while this ticket is held.
			require.NoError(t, NewTransactionManager(testPool).WithTransaction(ctx, func(otherCtx context.Context) error {
				_, err := events.Create(otherCtx, &domain.Event{
					TicketID: otherTicket.ID, ActorID: user.ID, Type: domain.EventTicketCreated, Payload: json.RawMessage(`{}`),
				})
				return err
			}))
			if commitFirst {
				require.NoError(t, firstTx.Commit(ctx))
			} else {
				require.NoError(t, firstTx.Rollback(ctx))
			}
			second := <-result
			require.NoError(t, second.err)
			require.Greater(t, second.event.ID, first.ID)
			visible, err = events.ListByTicketID(ctx, ticket.ID, 0, 1)
			require.NoError(t, err)
			require.Len(t, visible, 1)
			if commitFirst {
				require.Equal(t, first.ID, visible[0].ID)
				next, err := events.ListByTicketID(ctx, ticket.ID, visible[0].ID, 1)
				require.NoError(t, err)
				require.Len(t, next, 1)
				require.Equal(t, second.event.ID, next[0].ID)
			} else {
				require.Equal(t, second.event.ID, visible[0].ID)
			}
		})
	}
}
