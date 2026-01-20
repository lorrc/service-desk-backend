package domain

import "github.com/google/uuid"

// UserInfo is a lightweight projection for displaying user details.
type UserInfo struct {
	ID       uuid.UUID
	FullName string
	Email    string
}
