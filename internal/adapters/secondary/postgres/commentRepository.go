package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/adapters/secondary/postgres/db"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// CommentRepository handles database operations for comments.
type CommentRepository struct {
	q db.Querier
}

// Ensure implementation matches the interface.
var _ ports.CommentRepository = (*CommentRepository)(nil)

func NewCommentRepository(pool *pgxpool.Pool) ports.CommentRepository {
	return &CommentRepository{
		q: db.New(pool),
	}
}

// mapDBCommentToDomain converts a db.Comment to a domain.Comment
// This now correctly handles the mix of primitive and struct types from sqlc.
func mapDBCommentToDomain(dbComment db.Comment) *domain.Comment {
	return &domain.Comment{
		ID:        dbComment.ID.Bytes,
		TicketID:  dbComment.TicketID, // This is a primitive int64
		AuthorID:  dbComment.AuthorID.Bytes,
		Body:      dbComment.Body, // <-- THIS IS THE FIX (was dbComment.Body.String)
		CreatedAt: dbComment.CreatedAt.Time,
	}
}

// Create persists a new comment to the database.
func (r *CommentRepository) Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	params := db.CreateCommentParams{
		TicketID: comment.TicketID,
		AuthorID: pgtype.UUID{Bytes: comment.AuthorID, Valid: true},
		Body:     comment.Body, // <-- THIS IS THE FIX (was pgtype.Text{...})
	}

	dbComment, err := r.q.CreateComment(ctx, params)
	if err != nil {
		return nil, err
	}
	return mapDBCommentToDomain(dbComment), nil
}

// ListByTicketID retrieves all comments for a specific ticket, ordered by creation.
func (r *CommentRepository) ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error) {
	// ticketID is a primitive int64
	dbComments, err := r.q.ListCommentsByTicketID(ctx, ticketID)
	if err != nil {
		return nil, err
	}

	comments := make([]*domain.Comment, len(dbComments))
	for i, dbComment := range dbComments {
		comments[i] = mapDBCommentToDomain(dbComment)
	}
	return comments, nil
}
