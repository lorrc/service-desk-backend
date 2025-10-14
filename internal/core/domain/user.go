package domain

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	FullName       string
	Email          string
	PasswordHash   string
	CreatedAt      time.Time
}

// Business logic: Check password validity
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// Utility function: Hash a new password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
