package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// CommentRepository handles database operations for comments.
type CommentRepository struct {
	pool *pgxpool.Pool
}

// Ensure implementation matches the interface.
var _ ports.CommentRepository = (*CommentRepository)(nil)

// NewCommentRepository creates a new comment repository.
func NewCommentRepository(pool *pgxpool.Pool) ports.CommentRepository {
	return &CommentRepository{pool: pool}
}

// Create persists a new comment to the database.
func (r *CommentRepository) Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	query := `
		INSERT INTO comments (ticket_id, author_id, body, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, ticket_id, author_id, body, created_at
	`

	return r.scanComment(r.pool.QueryRow(ctx, query,
		comment.TicketID,
		pgtype.UUID{Bytes: comment.AuthorID, Valid: true},
		comment.Body,
		comment.CreatedAt,
	))
}

// ListByTicketID retrieves all comments for a specific ticket, ordered by creation.
func (r *CommentRepository) ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error) {
	query := `
		SELECT id, ticket_id, author_id, body, created_at
		FROM comments
		WHERE ticket_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanComments(rows)
}

// scanComment scans a single comment row.
func (r *CommentRepository) scanComment(row pgx.Row) (*domain.Comment, error) {
	var comment domain.Comment
	var commentID, authorID pgtype.UUID

	err := row.Scan(
		&commentID,
		&comment.TicketID,
		&authorID,
		&comment.Body,
		&comment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	comment.ID = commentID.Bytes
	comment.AuthorID = authorID.Bytes
	return &comment, nil
}

// scanComments scans multiple comment rows.
func (r *CommentRepository) scanComments(rows pgx.Rows) ([]*domain.Comment, error) {
	var comments []*domain.Comment

	for rows.Next() {
		var comment domain.Comment
		var commentID, authorID pgtype.UUID

		err := rows.Scan(
			&commentID,
			&comment.TicketID,
			&authorID,
			&comment.Body,
			&comment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		comment.ID = commentID.Bytes
		comment.AuthorID = authorID.Bytes
		comments = append(comments, &comment)
	}

	return comments, rows.Err()
}
