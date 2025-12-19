package postgres

import (
	"context"

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

// NewCommentRepository creates a new comment repository.
func NewCommentRepository(pool *pgxpool.Pool) ports.CommentRepository {
	return &CommentRepository{
		q: db.New(pool),
	}
}

// mapDBCommentToDomain converts a db.Comment to a domain.Comment.
func mapDBCommentToDomain(dbComment db.Comment) *domain.Comment {
	return &domain.Comment{
		ID:        dbComment.ID.Bytes,
		TicketID:  dbComment.TicketID,
		AuthorID:  dbComment.AuthorID.Bytes,
		Body:      dbComment.Body,
		CreatedAt: dbComment.CreatedAt.Time,
	}
}

// Create persists a new comment to the database.
func (r *CommentRepository) Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	params := db.CreateCommentParams{
		TicketID: comment.TicketID,
		AuthorID: pgtype.UUID{Bytes: comment.AuthorID, Valid: true},
		Body:     comment.Body,
	}

	dbComment, err := r.q.CreateComment(ctx, params)
	if err != nil {
		return nil, err
	}
	return mapDBCommentToDomain(dbComment), nil
}

// ListByTicketID retrieves all comments for a specific ticket, ordered by creation.
func (r *CommentRepository) ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error) {
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
