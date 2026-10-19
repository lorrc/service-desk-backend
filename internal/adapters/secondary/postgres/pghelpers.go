package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// toText converts a domain's primitive string to a pgtype.Text.
// An empty string is considered invalid (NULL).
func toText(s string) pgtype.Text {
	return pgtype.Text{
		String: s,
		Valid:  s != "",
	}
}

// fromText converts a pgtype.Text to a domain's primitive string.
// A NULL value is converted to an empty string ("").
func fromText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// nullableText converts a handler's *string (pointer) to a pgtype.Text.
// A nil pointer is considered invalid (NULL).
func nullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{
		String: *s,
		Valid:  true,
	}
}
