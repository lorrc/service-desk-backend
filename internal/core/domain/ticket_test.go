package domain_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTicketPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority domain.TicketPriority
		want     bool
	}{
		{"LOW is valid", domain.PriorityLow, true},
		{"MEDIUM is valid", domain.PriorityMedium, true},
		{"HIGH is valid", domain.PriorityHigh, true},
		{"empty is invalid", domain.TicketPriority(""), false},
		{"URGENT is invalid", domain.TicketPriority("URGENT"), false},
		{"lowercase is invalid", domain.TicketPriority("low"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.priority.IsValid())
		})
	}
}

func TestTicketStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status domain.TicketStatus
		want   bool
	}{
		{"OPEN is valid", domain.StatusOpen, true},
		{"IN_PROGRESS is valid", domain.StatusInProgress, true},
		{"CLOSED is valid", domain.StatusClosed, true},
		{"empty is invalid", domain.TicketStatus(""), false},
		{"PENDING is invalid", domain.TicketStatus("PENDING"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.IsValid())
		})
	}
}

func TestNewTicket(t *testing.T) {
	validRequesterID := uuid.New()

	tests := []struct {
		name        string
		params      domain.TicketParams
		expectError bool
		errorField  string // Field that should have error
	}{
		{
			name: "valid ticket",
			params: domain.TicketParams{
				Title:       "Test Ticket",
				Description: "Test description",
				Priority:    domain.PriorityMedium,
				RequesterID: validRequesterID,
			},
			expectError: false,
		},
		{
			name: "missing title",
			params: domain.TicketParams{
				Title:       "",
				Description: "Test description",
				Priority:    domain.PriorityMedium,
				RequesterID: validRequesterID,
			},
			expectError: true,
			errorField:  "title",
		},
		{
			name: "title too long",
			params: domain.TicketParams{
				Title:       strings.Repeat("a", 256),
				Description: "Test description",
				Priority:    domain.PriorityMedium,
				RequesterID: validRequesterID,
			},
			expectError: true,
			errorField:  "title",
		},
		{
			name: "description too long",
			params: domain.TicketParams{
				Title:       "Test Ticket",
				Description: strings.Repeat("a", 10001),
				Priority:    domain.PriorityMedium,
				RequesterID: validRequesterID,
			},
			expectError: true,
			errorField:  "description",
		},
		{
			name: "invalid priority",
			params: domain.TicketParams{
				Title:       "Test Ticket",
				Description: "Test description",
				Priority:    domain.TicketPriority("INVALID"),
				RequesterID: validRequesterID,
			},
			expectError: true,
			errorField:  "priority",
		},
		{
			name: "missing requester ID",
			params: domain.TicketParams{
				Title:       "Test Ticket",
				Description: "Test description",
				Priority:    domain.PriorityMedium,
				RequesterID: uuid.Nil,
			},
			expectError: true,
			errorField:  "requesterId",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket, err := domain.NewTicket(tt.params)

			if tt.expectError {
				require.Error(t, err)

				// Check that it's a ValidationErrors type
				var validationErr *apperrors.ValidationErrors
				if assert.ErrorAs(t, err, &validationErr) {
					assert.Contains(t, validationErr.Errors, tt.errorField)
				}
				assert.Nil(t, ticket)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ticket)
				assert.Equal(t, tt.params.Title, ticket.Title)
				assert.Equal(t, tt.params.Description, ticket.Description)
				assert.Equal(t, tt.params.Priority, ticket.Priority)
				assert.Equal(t, tt.params.RequesterID, ticket.RequesterID)
				assert.Equal(t, domain.StatusOpen, ticket.Status) // Default status
			}
		})
	}
}

func TestTicket_UpdateStatus(t *testing.T) {
	requesterID := uuid.New()

	tests := []struct {
		name          string
		initialStatus domain.TicketStatus
		newStatus     domain.TicketStatus
		expectError   bool
	}{
		// From OPEN
		{"OPEN to IN_PROGRESS", domain.StatusOpen, domain.StatusInProgress, false},
		{"OPEN to CLOSED", domain.StatusOpen, domain.StatusClosed, false},
		{"OPEN to OPEN (no change)", domain.StatusOpen, domain.StatusOpen, true},

		// From IN_PROGRESS
		{"IN_PROGRESS to OPEN", domain.StatusInProgress, domain.StatusOpen, false},
		{"IN_PROGRESS to CLOSED", domain.StatusInProgress, domain.StatusClosed, false},
		{"IN_PROGRESS to IN_PROGRESS", domain.StatusInProgress, domain.StatusInProgress, true},

		// From CLOSED (no transitions allowed)
		{"CLOSED to OPEN", domain.StatusClosed, domain.StatusOpen, true},
		{"CLOSED to IN_PROGRESS", domain.StatusClosed, domain.StatusInProgress, true},

		// Invalid status
		{"OPEN to INVALID", domain.StatusOpen, domain.TicketStatus("INVALID"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &domain.Ticket{
				ID:          1,
				Title:       "Test",
				Status:      tt.initialStatus,
				Priority:    domain.PriorityMedium,
				RequesterID: requesterID,
			}

			err := ticket.UpdateStatus(tt.newStatus)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, tt.initialStatus, ticket.Status) // Status unchanged
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.newStatus, ticket.Status)
				assert.NotNil(t, ticket.UpdatedAt)
			}
		})
	}
}

func TestTicket_Assign(t *testing.T) {
	requesterID := uuid.New()
	assigneeID := uuid.New()

	tests := []struct {
		name        string
		status      domain.TicketStatus
		assigneeID  uuid.UUID
		expectError bool
	}{
		{"assign to OPEN ticket", domain.StatusOpen, assigneeID, false},
		{"assign to IN_PROGRESS ticket", domain.StatusInProgress, assigneeID, false},
		{"assign to CLOSED ticket", domain.StatusClosed, assigneeID, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticket := &domain.Ticket{
				ID:          1,
				Title:       "Test",
				Status:      tt.status,
				Priority:    domain.PriorityMedium,
				RequesterID: requesterID,
			}

			err := ticket.Assign(tt.assigneeID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, ticket.AssigneeID)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ticket.AssigneeID)
				assert.Equal(t, tt.assigneeID, *ticket.AssigneeID)
				assert.NotNil(t, ticket.UpdatedAt)
			}
		})
	}
}

func TestTicket_CanTransitionTo(t *testing.T) {
	requesterID := uuid.New()

	ticket := &domain.Ticket{
		ID:          1,
		Status:      domain.StatusOpen,
		RequesterID: requesterID,
	}

	assert.True(t, ticket.CanTransitionTo(domain.StatusInProgress))
	assert.True(t, ticket.CanTransitionTo(domain.StatusClosed))
	assert.False(t, ticket.CanTransitionTo(domain.StatusOpen)) // Same status
}

func TestTicket_IsOwnedBy(t *testing.T) {
	ownerID := uuid.New()
	otherID := uuid.New()

	ticket := &domain.Ticket{
		ID:          1,
		RequesterID: ownerID,
	}

	assert.True(t, ticket.IsOwnedBy(ownerID))
	assert.False(t, ticket.IsOwnedBy(otherID))
}

func TestTicket_IsAssignedTo(t *testing.T) {
	assigneeID := uuid.New()
	otherID := uuid.New()

	// Unassigned ticket
	unassignedTicket := &domain.Ticket{ID: 1}
	assert.False(t, unassignedTicket.IsAssignedTo(assigneeID))

	// Assigned ticket
	assignedTicket := &domain.Ticket{
		ID:         1,
		AssigneeID: &assigneeID,
	}
	assert.True(t, assignedTicket.IsAssignedTo(assigneeID))
	assert.False(t, assignedTicket.IsAssignedTo(otherID))
}
