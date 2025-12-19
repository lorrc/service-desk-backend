package domain

import (
	"time"

	"github.com/google/uuid"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
)

// Constants for validation
const (
	MaxTitleLength       = 255
	MaxDescriptionLength = 10000
)

// TicketStatus represents the possible states of a ticket.
type TicketStatus string

const (
	StatusOpen       TicketStatus = "OPEN"
	StatusInProgress TicketStatus = "IN_PROGRESS"
	StatusClosed     TicketStatus = "CLOSED"
)

// IsValid checks if the status is a valid ticket status
func (s TicketStatus) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusClosed:
		return true
	}
	return false
}

// String returns the string representation of the status
func (s TicketStatus) String() string {
	return string(s)
}

// TicketPriority represents the urgency of a ticket.
type TicketPriority string

const (
	PriorityLow    TicketPriority = "LOW"
	PriorityMedium TicketPriority = "MEDIUM"
	PriorityHigh   TicketPriority = "HIGH"
)

// IsValid checks if the priority is a valid ticket priority
func (p TicketPriority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

// String returns the string representation of the priority
func (p TicketPriority) String() string {
	return string(p)
}

// ParseTicketStatus converts a string to TicketStatus with validation
func ParseTicketStatus(s string) (TicketStatus, error) {
	status := TicketStatus(s)
	if !status.IsValid() {
		return "", apperrors.ErrInvalidStatus
	}
	return status, nil
}

// ParseTicketPriority converts a string to TicketPriority with validation
func ParseTicketPriority(s string) (TicketPriority, error) {
	priority := TicketPriority(s)
	if !priority.IsValid() {
		return "", apperrors.ErrInvalidPriority
	}
	return priority, nil
}

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

// TicketParams holds parameters for creating a new ticket
type TicketParams struct {
	Title       string
	Description string
	Priority    TicketPriority
	RequesterID uuid.UUID
}

// Validate validates the ticket creation parameters
func (p *TicketParams) Validate() error {
	errs := apperrors.NewValidationErrors()

	if p.Title == "" {
		errs.Add("title", "Title is required")
	} else if len(p.Title) > MaxTitleLength {
		errs.Add("title", "Title must be 255 characters or less")
	}

	if len(p.Description) > MaxDescriptionLength {
		errs.Add("description", "Description must be 10,000 characters or less")
	}

	if !p.Priority.IsValid() {
		errs.Add("priority", "Priority must be LOW, MEDIUM, or HIGH")
	}

	if p.RequesterID == uuid.Nil {
		errs.Add("requesterId", "Requester ID is required")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// NewTicket is a factory function to create a valid new ticket.
func NewTicket(params TicketParams) (*Ticket, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	return &Ticket{
		Title:       params.Title,
		Description: params.Description,
		Status:      StatusOpen, // Default status
		Priority:    params.Priority,
		RequesterID: params.RequesterID,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// validTransitions defines the valid state transitions for tickets
var validTransitions = map[TicketStatus][]TicketStatus{
	StatusOpen:       {StatusInProgress, StatusClosed},
	StatusInProgress: {StatusOpen, StatusClosed},
	StatusClosed:     {}, // Cannot transition from closed
}

// CanTransitionTo checks if the ticket can transition to the new status
func (t *Ticket) CanTransitionTo(newStatus TicketStatus) bool {
	allowed, ok := validTransitions[t.Status]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}

// UpdateStatus changes the ticket's status, enforcing business rules.
func (t *Ticket) UpdateStatus(newStatus TicketStatus) error {
	if !newStatus.IsValid() {
		return apperrors.ErrInvalidStatus
	}

	if !t.CanTransitionTo(newStatus) {
		return apperrors.ErrInvalidStatusTransition
	}

	t.Status = newStatus
	now := time.Now().UTC()
	t.UpdatedAt = &now
	return nil
}

// Assign sets or changes the assignee of the ticket.
func (t *Ticket) Assign(assigneeID uuid.UUID) error {
	if assigneeID == uuid.Nil {
		return apperrors.NewValidationErrors()
	}

	// Business rule: You cannot assign a closed ticket.
	if t.Status == StatusClosed {
		return apperrors.ErrCannotAssignClosed
	}

	t.AssigneeID = &assigneeID
	now := time.Now().UTC()
	t.UpdatedAt = &now
	return nil
}

// Unassign removes the assignee from the ticket
func (t *Ticket) Unassign() error {
	if t.Status == StatusClosed {
		return apperrors.ErrCannotAssignClosed
	}

	t.AssigneeID = nil
	now := time.Now().UTC()
	t.UpdatedAt = &now
	return nil
}

// IsOwnedBy checks if the ticket belongs to the given user
func (t *Ticket) IsOwnedBy(userID uuid.UUID) bool {
	return t.RequesterID == userID
}

// IsAssignedTo checks if the ticket is assigned to the given user
func (t *Ticket) IsAssignedTo(userID uuid.UUID) bool {
	return t.AssigneeID != nil && *t.AssigneeID == userID
}

// IsClosed checks if the ticket is in closed status
func (t *Ticket) IsClosed() bool {
	return t.Status == StatusClosed
}
