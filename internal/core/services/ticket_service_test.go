package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/lorrc/service-desk-backend/internal/core/mocks"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/lorrc/service-desk-backend/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTicketService_CreateTicket(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		// Setup expectations
		mockAuthz.On("Can", ctx, userID, "tickets:create").Return(true, nil)
		mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Ticket")).
			Return(&domain.Ticket{
				ID:          1,
				Title:       "Test Ticket",
				Description: "Test Description",
				Priority:    domain.PriorityMedium,
				Status:      domain.StatusOpen,
				RequesterID: userID,
			}, nil)

		params := ports.CreateTicketParams{
			Title:       "Test Ticket",
			Description: "Test Description",
			Priority:    domain.PriorityMedium,
			RequesterID: userID,
		}

		ticket, err := svc.CreateTicket(ctx, params)

		require.NoError(t, err)
		assert.NotNil(t, ticket)
		assert.Equal(t, int64(1), ticket.ID)
		assert.Equal(t, "Test Ticket", ticket.Title)
		assert.Equal(t, domain.StatusOpen, ticket.Status)

		mockAuthz.AssertExpectations(t)
		mockRepo.AssertExpectations(t)
	})

	t.Run("forbidden when no permission", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		mockAuthz.On("Can", ctx, userID, "tickets:create").Return(false, nil)

		params := ports.CreateTicketParams{
			Title:       "Test Ticket",
			Description: "Test Description",
			Priority:    domain.PriorityMedium,
			RequesterID: userID,
		}

		ticket, err := svc.CreateTicket(ctx, params)

		assert.Nil(t, ticket)
		assert.ErrorIs(t, err, apperrors.ErrForbidden)
		mockRepo.AssertNotCalled(t, "Create")
	})

	t.Run("validation error for empty title", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		mockAuthz.On("Can", ctx, userID, "tickets:create").Return(true, nil)

		params := ports.CreateTicketParams{
			Title:       "", // Empty title
			Description: "Test Description",
			Priority:    domain.PriorityMedium,
			RequesterID: userID,
		}

		ticket, err := svc.CreateTicket(ctx, params)

		assert.Nil(t, ticket)
		assert.Error(t, err)
		mockRepo.AssertNotCalled(t, "Create")
	})
}

func TestTicketService_GetTicket(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	ticketID := int64(1)

	t.Run("owner can access own ticket", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		expectedTicket := &domain.Ticket{
			ID:          ticketID,
			Title:       "Test Ticket",
			RequesterID: userID, // User is the owner
			Status:      domain.StatusOpen,
		}

		mockAuthz.On("Can", ctx, userID, "tickets:read").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(expectedTicket, nil)

		ticket, err := svc.GetTicket(ctx, ticketID, userID)

		require.NoError(t, err)
		assert.Equal(t, expectedTicket, ticket)
	})

	t.Run("non-owner without admin permission is forbidden", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		otherUserID := uuid.New()
		expectedTicket := &domain.Ticket{
			ID:          ticketID,
			Title:       "Test Ticket",
			RequesterID: otherUserID, // Different owner
			Status:      domain.StatusOpen,
		}

		mockAuthz.On("Can", ctx, userID, "tickets:read").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(expectedTicket, nil)
		mockAuthz.On("Can", ctx, userID, "tickets:read:all").Return(false, nil)

		ticket, err := svc.GetTicket(ctx, ticketID, userID)

		assert.Nil(t, ticket)
		assert.ErrorIs(t, err, apperrors.ErrForbidden)
	})

	t.Run("admin can access any ticket", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		otherUserID := uuid.New()
		expectedTicket := &domain.Ticket{
			ID:          ticketID,
			Title:       "Test Ticket",
			RequesterID: otherUserID, // Different owner
			Status:      domain.StatusOpen,
		}

		mockAuthz.On("Can", ctx, userID, "tickets:read").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(expectedTicket, nil)
		mockAuthz.On("Can", ctx, userID, "tickets:read:all").Return(true, nil)

		ticket, err := svc.GetTicket(ctx, ticketID, userID)

		require.NoError(t, err)
		assert.Equal(t, expectedTicket, ticket)
	})

	t.Run("ticket not found", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		mockAuthz.On("Can", ctx, userID, "tickets:read").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(nil, apperrors.ErrTicketNotFound)

		ticket, err := svc.GetTicket(ctx, ticketID, userID)

		assert.Nil(t, ticket)
		assert.ErrorIs(t, err, apperrors.ErrTicketNotFound)
	})
}

func TestTicketService_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	ticketID := int64(1)

	t.Run("success", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		existingTicket := &domain.Ticket{
			ID:          ticketID,
			Title:       "Test Ticket",
			RequesterID: uuid.New(), // Different from actor
			Status:      domain.StatusOpen,
		}

		mockAuthz.On("Can", ctx, userID, "tickets:update:status").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(existingTicket, nil)
		mockRepo.On("Update", ctx, mock.AnythingOfType("*domain.Ticket")).
			Return(&domain.Ticket{
				ID:     ticketID,
				Title:  "Test Ticket",
				Status: domain.StatusInProgress,
			}, nil)
		mockNotifier.On("Notify", mock.Anything, mock.Anything).Return()
		mockBroadcaster.On("Broadcast", mock.Anything).Return(nil)

		params := ports.UpdateStatusParams{
			TicketID: ticketID,
			Status:   domain.StatusInProgress,
			ActorID:  userID,
		}

		ticket, err := svc.UpdateStatus(ctx, params)

		require.NoError(t, err)
		assert.Equal(t, domain.StatusInProgress, ticket.Status)
	})

	t.Run("invalid status transition", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		closedTicket := &domain.Ticket{
			ID:          ticketID,
			Title:       "Test Ticket",
			RequesterID: uuid.New(),
			Status:      domain.StatusClosed, // Already closed
		}

		mockAuthz.On("Can", ctx, userID, "tickets:update:status").Return(true, nil)
		mockRepo.On("GetByID", ctx, ticketID).Return(closedTicket, nil)

		params := ports.UpdateStatusParams{
			TicketID: ticketID,
			Status:   domain.StatusOpen, // Cannot reopen closed ticket
			ActorID:  userID,
		}

		ticket, err := svc.UpdateStatus(ctx, params)

		assert.Nil(t, ticket)
		assert.ErrorIs(t, err, apperrors.ErrInvalidStatusTransition)
	})
}

func TestTicketService_ListTickets(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("admin sees all tickets", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		expectedTickets := []*domain.Ticket{
			{ID: 1, Title: "Ticket 1"},
			{ID: 2, Title: "Ticket 2"},
		}

		mockAuthz.On("Can", ctx, userID, "tickets:list:all").Return(true, nil)
		mockRepo.On("ListPaginated", ctx, mock.Anything).Return(expectedTickets, nil)

		params := ports.ListTicketsParams{
			ViewerID: userID,
			Limit:    10,
			Offset:   0,
		}

		tickets, err := svc.ListTickets(ctx, params)

		require.NoError(t, err)
		assert.Len(t, tickets, 2)
	})

	t.Run("customer sees only own tickets", func(t *testing.T) {
		mockRepo := mocks.NewMockTicketRepository()
		mockAuthz := mocks.NewMockAuthorizationService()
		mockNotifier := mocks.NewMockNotifier()
		mockBroadcaster := mocks.NewMockEventBroadcaster()

		svc := services.NewTicketService(mockRepo, mockAuthz, mockNotifier, mockBroadcaster)

		expectedTickets := []*domain.Ticket{
			{ID: 1, Title: "My Ticket", RequesterID: userID},
		}

		mockAuthz.On("Can", ctx, userID, "tickets:list:all").Return(false, nil)
		mockRepo.On("ListByRequesterPaginated", ctx, mock.Anything).Return(expectedTickets, nil)

		params := ports.ListTicketsParams{
			ViewerID: userID,
			Limit:    10,
			Offset:   0,
		}

		tickets, err := svc.ListTickets(ctx, params)

		require.NoError(t, err)
		assert.Len(t, tickets, 1)
		mockRepo.AssertNotCalled(t, "ListPaginated")
	})
}
