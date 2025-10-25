package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Pre-defined errors for domain-specific validation.
var (
	ErrTitleRequired           = errors.New("title is required")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
)

// TicketStatus represents the possible states of a ticket.
type TicketStatus string

const (
	StatusOpen       = "OPEN"
	StatusInProgress = "IN_PROGRESS"
	StatusClosed     = "CLOSED"
)

// TicketPriority represents the urgency of a ticket.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "LOW"
	PriorityMedium TicketPriority = "MEDIUM"
	PriorityHigh   TicketPriority = "HIGH"
)

// Ticket is the core domain entity.
type Ticket struct {
	ID          int64
	Title       string
	Description string
	Status      TicketStatus
	Priority    TicketPriority
	RequesterID uuid.UUID
	AssigneeID  *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

// NewTicket is a factory function to create a valid new ticket.
func NewTicket(title, description string, priority TicketPriority, requesterID uuid.UUID) (*Ticket, error) {
	if title == "" {
		return nil, ErrTitleRequired
	}

	return &Ticket{
		Title:       title,
		Description: description,
		Status:      StatusOpen, // Default status
		Priority:    priority,
		RequesterID: requesterID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// UpdateStatus changes the ticket's status, enforcing business rules.
func (t *Ticket) UpdateStatus(newStatus TicketStatus) error {
	// Defines the valid state transitions.
	validTransitions := map[TicketStatus][]TicketStatus{
		StatusOpen:       {StatusInProgress, StatusClosed},
		StatusInProgress: {StatusOpen, StatusClosed},
		StatusClosed:     {}, // Cannot transition from closed
	}

	allowed, ok := validTransitions[t.Status]
	if !ok {
		return ErrInvalidStatusTransition // Should not happen with valid states
	}

	for _, s := range allowed {
		if s == newStatus {
			t.Status = newStatus
			now := time.Now().UTC()
			t.UpdatedAt = &now
			return nil
		}
	}

	// If the loop completes without finding a match, the transition is invalid.
	return ErrInvalidStatusTransition
}

// Assign sets or changes the assignee of the ticket.
func (t *Ticket) Assign(assigneeID uuid.UUID) error {
	// Business rule: You cannot assign a closed ticket.
	if t.Status == StatusClosed {
		return errors.New("cannot assign a closed ticket")
	}
	t.AssigneeID = &assigneeID
	now := time.Now().UTC()
	t.UpdatedAt = &now
	return nil
}
