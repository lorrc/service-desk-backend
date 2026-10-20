package domain

import (
	"time"

	"github.com/google/uuid"
)

// Organization identifies the workspace that owns an account.
type Organization struct {
	ID   uuid.UUID
	Name string
}

// Invitation grants one email address permission to join an organization as a customer.
type Invitation struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	OrganizationName string
	Email            string
	CreatedBy        uuid.UUID
	CreatedAt        time.Time
	ExpiresAt        time.Time
	AcceptedAt       *time.Time
	RevokedAt        *time.Time
}

// Status describes whether an invitation can still be redeemed.
func (i *Invitation) Status(now time.Time) string {
	switch {
	case i.AcceptedAt != nil:
		return "accepted"
	case i.RevokedAt != nil:
		return "revoked"
	case !now.Before(i.ExpiresAt):
		return "expired"
	default:
		return "pending"
	}
}
