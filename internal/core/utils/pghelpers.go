package utils

import (
	// Ensure this import path matches the pgx version you are using (e.g., v5)
	"github.com/jackc/pgx/v5/pgtype"
)

// ToString converts a domain's primitive string to a pgtype.Text.
// An empty string is considered invalid (NULL).
func ToString(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  s != "",
	}
}

// FromString converts a pgtype.Text to a domain's primitive string.
// A NULL value is converted to an empty string ("").
func FromString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// ToNullString converts a handler's *string (pointer) to a pgtype.Text.
// A nil pointer is considered invalid (NULL).
func ToNullString(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: *s,
		Valid:  true,
	}
}
