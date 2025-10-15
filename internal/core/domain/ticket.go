package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// TicketStatus represents the lifecycle state of a ticket.
// Using a distinct type allows us to define methods on it and ensures type safety.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "OPEN"
	StatusInProgress TicketStatus = "IN_PROGRESS"
	StatusClosed     TicketStatus = "CLOSED"
	StatusBacklog    TicketStatus = "BACKLOG"
)

// TicketPriority represents the urgency of a ticket.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "LOW"
	PriorityMedium TicketPriority = "MEDIUM"
	PriorityHigh   TicketPriority = "HIGH"
	PriorityUrgent TicketPriority = "URGENT"
)

// Ticket is the core domain entity representing a service desk request.
type Ticket struct {
	ID          int64
	Title       string
	Description string
	Status      TicketStatus
	Priority    TicketPriority

	// Foreign Keys referencing the User entity
	RequesterID uuid.UUID
	// AssigneeID is a pointer to allow for a nil (unassigned) state.
	AssigneeID *uuid.UUID

	// Timestamps
	CreatedAt time.Time
	// UpdatedAt is a pointer to allow for a nil state (never updated).
	UpdatedAt *time.Time
}

// NewTicket is a factory function (Constructor) to ensure a Ticket is always
// created in a valid initial state.
func NewTicket(title, description string, priority TicketPriority, requesterID uuid.UUID) (*Ticket, error) {
	if title == "" {
		return nil, errors.New("ticket title cannot be empty")
	}
	if requesterID == uuid.Nil {
		return nil, errors.New("requester ID is required")
	}

	// Validate priority (optional, can also be handled at the service layer)
	if !priority.IsValid() {
		return nil, errors.New("invalid ticket priority specified")
	}

	return &Ticket{
		Title:       title,
		Description: description,
		Status:      StatusOpen, // All new tickets start as OPEN
		Priority:    priority,
		RequesterID: requesterID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// IsValid checks if the priority string is one of the allowed values.
func (p TicketPriority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	}
	return false
}

// Assign handles the business logic for assigning a ticket to an agent.
func (t *Ticket) Assign(agentID uuid.UUID) error {
	// Business Rule: Cannot modify a closed ticket.
	if t.Status == StatusClosed {
		return errors.New("cannot assign a closed ticket")
	}

	// Check if the assignment is actually changing (idempotency)
	if t.AssigneeID != nil && *t.AssigneeID == agentID {
		return nil
	}

	t.AssigneeID = &agentID

	// Business Rule: Assignment often moves the status to In Progress.
	if t.Status == StatusOpen || t.Status == StatusBacklog {
		t.Status = StatusInProgress
	}

	now := time.Now().UTC()
	t.UpdatedAt = &now
	return nil
}

// UpdateStatus handles the logic for changing a ticket's status, enforcing state transitions.
func (t *Ticket) UpdateStatus(newStatus TicketStatus) error {
	// Business Rule: A closed ticket cannot be modified or reopened.
	if t.Status == StatusClosed {
		if newStatus == StatusClosed {
			return nil // Already closed, idempotent
		}
		return errors.New("cannot change the status of a closed ticket")
	}

	// Example Business Rule: Cannot move from Backlog directly to Closed.
	if t.Status == StatusBacklog && newStatus == StatusClosed {
		return errors.New("ticket must be moved to in progress before closing")
	}

	if t.Status != newStatus {
		t.Status = newStatus
		now := time.Now().UTC()
		t.UpdatedAt = &now
	}

	return nil
}
