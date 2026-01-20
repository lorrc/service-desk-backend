package domain

import (
	"strconv"
	"time"
)

// CommentSnapshot matches the API response shape for comments.
type CommentSnapshot struct {
	ID        string `json:"id"`
	TicketID  int64  `json:"ticketId"`
	AuthorID  string `json:"authorId"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
}

// TicketSnapshot matches the API response shape for tickets.
type TicketSnapshot struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	RequesterID string  `json:"requesterId"`
	AssigneeID  *string `json:"assigneeId"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   *string `json:"updatedAt"`
	ClosedAt    *string `json:"closedAt"`
}

// NewCommentSnapshot builds a comment snapshot from a domain comment.
func NewCommentSnapshot(comment *Comment) CommentSnapshot {
	return CommentSnapshot{
		ID:        strconv.FormatInt(comment.ID, 10),
		TicketID:  comment.TicketID,
		AuthorID:  comment.AuthorID.String(),
		Body:      comment.Body,
		CreatedAt: comment.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// NewTicketSnapshot builds a ticket snapshot from a domain ticket.
func NewTicketSnapshot(ticket *Ticket) TicketSnapshot {
	var assigneeID *string
	if ticket.AssigneeID != nil {
		value := ticket.AssigneeID.String()
		assigneeID = &value
	}

	var updatedAt *string
	if ticket.UpdatedAt != nil {
		value := ticket.UpdatedAt.UTC().Format(time.RFC3339)
		updatedAt = &value
	}

	var closedAt *string
	if ticket.ClosedAt != nil {
		value := ticket.ClosedAt.UTC().Format(time.RFC3339)
		closedAt = &value
	}

	return TicketSnapshot{
		ID:          ticket.ID,
		Title:       ticket.Title,
		Description: ticket.Description,
		Status:      string(ticket.Status),
		Priority:    string(ticket.Priority),
		RequesterID: ticket.RequesterID.String(),
		AssigneeID:  assigneeID,
		CreatedAt:   ticket.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   updatedAt,
		ClosedAt:    closedAt,
	}
}
