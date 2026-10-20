package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
)

// UserRepository defines the port for user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	LockOrganization(ctx context.Context, orgID uuid.UUID) error
	GetUserInfo(ctx context.Context, orgID uuid.UUID, userIDs []uuid.UUID) (map[uuid.UUID]domain.UserInfo, error)
	ValidateSession(ctx context.Context, userID, orgID uuid.UUID, version int64) (bool, error)
	ListAssignableUsers(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error)
	ListUsersPage(ctx context.Context, params ListUsersParams) ([]*domain.UserSummary, error)
	GetRoles(ctx context.Context, userID, orgID uuid.UUID) ([]string, error)
	ChangeOwnPassword(ctx context.Context, userID, orgID uuid.UUID, version int64, oldHash, newHash string) error
	RevokeSessions(ctx context.Context, userID, orgID uuid.UUID, version int64) error
	SetActive(ctx context.Context, userID uuid.UUID, isActive bool) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error
	UpdateLastActive(ctx context.Context, userID uuid.UUID, at time.Time) error
}

// TicketRepository defines the port for ticket persistence.
type TicketRepository interface {
	Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error)
	GetByID(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error)
	GetForUpdate(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error)
	Update(ctx context.Context, ticket *domain.Ticket, orgID uuid.UUID) (*domain.Ticket, error)
	ListPaginated(ctx context.Context, params ListTicketsRepoParams) ([]*domain.Ticket, error)
	GetCounts(ctx context.Context, params TicketCountsRepoParams) (*domain.TicketCounts, error)
}

// AuthorizationRepository defines the port for RBAC data access.
type AuthorizationRepository interface {
	GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error)
	AssignRole(ctx context.Context, userID uuid.UUID, roleName string) error
	SetUserRole(ctx context.Context, userID uuid.UUID, roleName string) error
	EnsureRBACDefaults(ctx context.Context) error
}

// AnalyticsRepository defines the port for analytics data access.
type AnalyticsRepository interface {
	GetOverview(ctx context.Context, orgID uuid.UUID, days int) (*domain.AnalyticsOverview, error)
}

// CommentRepository defines the port for comment persistence.
type CommentRepository interface {
	Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error)
	ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error)
}

// TicketEventRepository defines the port for ticket event persistence.
type TicketEventRepository interface {
	Create(ctx context.Context, event *domain.Event) (*domain.Event, error)
	ListByTicketID(ctx context.Context, ticketID int64, afterID int64, limit int) ([]*domain.Event, error)
	ListHistory(ctx context.Context, ticketID, beforeID int64, limit int) ([]*domain.Event, error)
}

// ListTicketsRepoParams defines parameters for paginated ticket queries.
type ListTicketsRepoParams struct {
	Query          string
	Active         bool
	OrganizationID uuid.UUID
	Limit          int
	Offset         int
	Status         *string
	Priority       *string
	RequesterID    *uuid.UUID
	AssigneeID     *uuid.UUID
	Unassigned     bool
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
}

// TicketCountsRepoParams contains the baseline filters shared by triage tabs.
type TicketCountsRepoParams struct {
	OrganizationID uuid.UUID
	ViewerID       uuid.UUID
	RequesterID    *uuid.UUID
	Query          string
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
}

// ListUsersParams scopes and filters an administrator's user directory.
type ListUsersParams struct {
	OrganizationID uuid.UUID
	ActorID        uuid.UUID
	Query          string
	Role           string
	IsActive       *bool
	Limit          int
	Offset         int
}
