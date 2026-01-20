package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

// Helper to create a user for ticket tests
func createTestUser(t *testing.T, ctx context.Context, userRepo ports.UserRepository) *domain.User {
	user := &domain.User{
		ID:             uuid.New(),
		FullName:       "Ticket Requester",
		Email:          uuid.NewString() + "@example.com", // Ensure unique email
		HashedPassword: "testpassword",
		OrganizationID: defaultOrgID,
	}
	createdUser, err := userRepo.Create(ctx, user)
	require.NoError(t, err)
	return createdUser
}

func TestTicketRepository_CreateGet(t *testing.T) {
	ctx := context.Background()
	ticketRepo, userRepo := newTestRepos(t)

	// 1. Create a prerequisite user
	testUser := createTestUser(t, ctx, userRepo)

	// 2. Create a new ticket
	newTicket, err := domain.NewTicket(domain.TicketParams{
		Title:       "Test Ticket",
		Description: "This is a description",
		Priority:    domain.PriorityMedium,
		RequesterID: testUser.ID,
	})
	require.NoError(t, err)

	createdTicket, err := ticketRepo.Create(ctx, newTicket)
	require.NoError(t, err, "Failed to create ticket")
	assert.NotZero(t, createdTicket.ID)

	// 3. Get the ticket by ID
	foundTicket, err := ticketRepo.GetByID(ctx, createdTicket.ID)
	require.NoError(t, err, "Failed to get ticket by ID")

	// 4. Assert values are correct
	assert.Equal(t, "Test Ticket", foundTicket.Title)
	assert.Equal(t, "This is a description", foundTicket.Description)
	assert.Equal(t, domain.PriorityMedium, foundTicket.Priority)
	assert.Equal(t, testUser.ID, foundTicket.RequesterID)
	assert.Equal(t, domain.StatusOpen, foundTicket.Status)
}

func TestTicketRepository_PaginatedList(t *testing.T) {
	ctx := context.Background()
	ticketRepo, userRepo := newTestRepos(t)

	user1 := createTestUser(t, ctx, userRepo)
	user2 := createTestUser(t, ctx, userRepo)

	// Create tickets
	_, _ = ticketRepo.Create(ctx, &domain.Ticket{Title: "T1", Priority: domain.PriorityHigh, RequesterID: user1.ID, Status: domain.StatusOpen})
	_, _ = ticketRepo.Create(ctx, &domain.Ticket{Title: "T2", Priority: domain.PriorityLow, RequesterID: user1.ID, Status: domain.StatusOpen})
	_, _ = ticketRepo.Create(ctx, &domain.Ticket{Title: "T3", Priority: domain.PriorityMedium, RequesterID: user1.ID, Status: domain.StatusClosed})
	_, _ = ticketRepo.Create(ctx, &domain.Ticket{Title: "T4", Priority: domain.PriorityHigh, RequesterID: user2.ID, Status: domain.StatusOpen})

	// Test case 1: List all for user 1
	params1 := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: user1.ID, Valid: true},
		Limit:       10,
		Offset:      0,
	}
	tickets1, err := ticketRepo.ListByRequesterPaginated(ctx, params1)
	require.NoError(t, err)
	assert.Len(t, tickets1, 3)

	// Test case 2: List all for user 2
	params2 := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: user2.ID, Valid: true},
		Limit:       10,
		Offset:      0,
	}
	tickets2, err := ticketRepo.ListByRequesterPaginated(ctx, params2)
	require.NoError(t, err)
	assert.Len(t, tickets2, 1)
	assert.Equal(t, "T4", tickets2[0].Title)

	// Test case 3: List with pagination (Limit 1, Offset 1) for user 1
	params3 := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: user1.ID, Valid: true},
		Limit:       1,
		Offset:      1,
	}
	tickets3, err := ticketRepo.ListByRequesterPaginated(ctx, params3)
	require.NoError(t, err)
	assert.Len(t, tickets3, 1)
	assert.Equal(t, "T2", tickets3[0].Title) // Ordered by created_at DESC

	// Test case 4: List with filter (Priority: high) for user 1
	params4 := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: user1.ID, Valid: true},
		Limit:       10,
		Offset:      0,
		Priority:    utils.ToString(string(domain.PriorityHigh)),
	}
	tickets4, err := ticketRepo.ListByRequesterPaginated(ctx, params4)
	require.NoError(t, err)
	assert.Len(t, tickets4, 1)
	assert.Equal(t, "T1", tickets4[0].Title)

	// Test case 5: List with filter (Status: closed) for user 1
	params5 := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: user1.ID, Valid: true},
		Limit:       10,
		Offset:      0,
		Status:      utils.ToString(string(domain.StatusClosed)),
	}
	tickets5, err := ticketRepo.ListByRequesterPaginated(ctx, params5)
	require.NoError(t, err)
	assert.Len(t, tickets5, 1)
	assert.Equal(t, "T3", tickets5[0].Title)
}

func TestTicketRepository_ListFilters(t *testing.T) {
	ctx := context.Background()
	ticketRepo, userRepo := newTestRepos(t)

	requester := createTestUser(t, ctx, userRepo)
	assignee := createTestUser(t, ctx, userRepo)

	ticket1, err := ticketRepo.Create(ctx, &domain.Ticket{Title: "T1", Priority: domain.PriorityHigh, RequesterID: requester.ID, Status: domain.StatusOpen})
	require.NoError(t, err)
	ticket2, err := ticketRepo.Create(ctx, &domain.Ticket{Title: "T2", Priority: domain.PriorityLow, RequesterID: requester.ID, Status: domain.StatusOpen})
	require.NoError(t, err)
	ticket3, err := ticketRepo.Create(ctx, &domain.Ticket{Title: "T3", Priority: domain.PriorityMedium, RequesterID: requester.ID, Status: domain.StatusOpen})
	require.NoError(t, err)

	require.NoError(t, ticket1.Assign(assignee.ID))
	_, err = ticketRepo.Update(ctx, ticket1)
	require.NoError(t, err)

	createdAt1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	createdAt2 := time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC)
	createdAt3 := time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC)

	_, err = testPool.Exec(ctx, "UPDATE tickets SET created_at = $1 WHERE id = $2", createdAt1, ticket1.ID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, "UPDATE tickets SET created_at = $1 WHERE id = $2", createdAt2, ticket2.ID)
	require.NoError(t, err)
	_, err = testPool.Exec(ctx, "UPDATE tickets SET created_at = $1 WHERE id = $2", createdAt3, ticket3.ID)
	require.NoError(t, err)

	assigneeParams := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: requester.ID, Valid: true},
		Limit:       10,
		Offset:      0,
		AssigneeID:  pgtype.UUID{Bytes: assignee.ID, Valid: true},
	}
	assigneeTickets, err := ticketRepo.ListByRequesterPaginated(ctx, assigneeParams)
	require.NoError(t, err)
	require.Len(t, assigneeTickets, 1)
	assert.Equal(t, "T1", assigneeTickets[0].Title)

	unassignedParams := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: requester.ID, Valid: true},
		Limit:       10,
		Offset:      0,
		Unassigned:  pgtype.Bool{Bool: true, Valid: true},
	}
	unassignedTickets, err := ticketRepo.ListByRequesterPaginated(ctx, unassignedParams)
	require.NoError(t, err)
	require.Len(t, unassignedTickets, 2)
	assert.ElementsMatch(t, []string{"T2", "T3"}, []string{unassignedTickets[0].Title, unassignedTickets[1].Title})

	dateParams := ports.ListTicketsRepoParams{
		RequesterID: pgtype.UUID{Bytes: requester.ID, Valid: true},
		Limit:       10,
		Offset:      0,
		CreatedFrom: pgtype.Timestamptz{Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true},
		CreatedTo:   pgtype.Timestamptz{Time: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC), Valid: true},
	}
	dateTickets, err := ticketRepo.ListByRequesterPaginated(ctx, dateParams)
	require.NoError(t, err)
	require.Len(t, dateTickets, 1)
	assert.Equal(t, "T2", dateTickets[0].Title)
}
