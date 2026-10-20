package mocks

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

// MockUserRepository is a mock implementation of ports.UserRepository
type MockUserRepository struct {
	mock.Mock
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{}
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) (*domain.User, error) {
	args := m.Called(ctx, user)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockUserRepository) LockOrganization(ctx context.Context, orgID uuid.UUID) error {
	return m.Called(ctx, orgID).Error(0)
}

func (m *MockUserRepository) ValidateSession(ctx context.Context, userID, orgID uuid.UUID, version int64) (bool, error) {
	args := m.Called(ctx, userID, orgID, version)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) GetUserInfo(ctx context.Context, orgID uuid.UUID, userIDs []uuid.UUID) (map[uuid.UUID]domain.UserInfo, error) {
	args := m.Called(ctx, orgID, userIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID]domain.UserInfo), args.Error(1)
}

func (m *MockUserRepository) ListAssignableUsers(ctx context.Context, orgID uuid.UUID) ([]*domain.User, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.User), args.Error(1)
}

func (m *MockUserRepository) SetActive(ctx context.Context, userID uuid.UUID, isActive bool) error {
	args := m.Called(ctx, userID, isActive)
	return args.Error(0)
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	args := m.Called(ctx, userID, hashedPassword)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateLastActive(ctx context.Context, userID uuid.UUID, at time.Time) error {
	args := m.Called(ctx, userID, at)
	return args.Error(0)
}

// MockTicketRepository is a mock implementation of ports.TicketRepository
type MockTicketRepository struct {
	mock.Mock
}

func NewMockTicketRepository() *MockTicketRepository {
	return &MockTicketRepository{}
}

func (m *MockTicketRepository) Create(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	args := m.Called(ctx, ticket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) GetByID(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error) {
	args := m.Called(ctx, id, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) Update(ctx context.Context, ticket *domain.Ticket, orgID uuid.UUID) (*domain.Ticket, error) {
	args := m.Called(ctx, ticket, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) ListPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) GetForUpdate(ctx context.Context, id int64, orgID uuid.UUID) (*domain.Ticket, error) {
	args := m.Called(ctx, id, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) GetCounts(ctx context.Context, params ports.TicketCountsRepoParams) (*domain.TicketCounts, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TicketCounts), args.Error(1)
}

// MockAuthorizationRepository is a mock implementation of ports.AuthorizationRepository
type MockAuthorizationRepository struct {
	mock.Mock
}

func NewMockAuthorizationRepository() *MockAuthorizationRepository {
	return &MockAuthorizationRepository{}
}

func (m *MockAuthorizationRepository) GetUserPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAuthorizationRepository) AssignRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	args := m.Called(ctx, userID, roleName)
	return args.Error(0)
}

func (m *MockAuthorizationRepository) SetUserRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	args := m.Called(ctx, userID, roleName)
	return args.Error(0)
}

func (m *MockAuthorizationRepository) EnsureRBACDefaults(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCommentRepository is a mock implementation of ports.CommentRepository
type MockCommentRepository struct {
	mock.Mock
}

func NewMockCommentRepository() *MockCommentRepository {
	return &MockCommentRepository{}
}

func (m *MockCommentRepository) Create(ctx context.Context, comment *domain.Comment) (*domain.Comment, error) {
	args := m.Called(ctx, comment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Comment), args.Error(1)
}

func (m *MockCommentRepository) ListByTicketID(ctx context.Context, ticketID int64) ([]*domain.Comment, error) {
	args := m.Called(ctx, ticketID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Comment), args.Error(1)
}

// MockAuthorizationService is a mock implementation of ports.AuthorizationService
type MockAuthorizationService struct {
	mock.Mock
}

func NewMockAuthorizationService() *MockAuthorizationService {
	return &MockAuthorizationService{}
}

func (m *MockAuthorizationService) Can(ctx context.Context, userID uuid.UUID, permission string) (bool, error) {
	args := m.Called(ctx, userID, permission)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizationService) GetPermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// MockTicketService is a mock implementation of ports.TicketService
type MockTicketService struct {
	mock.Mock
}

func NewMockTicketService() *MockTicketService {
	return &MockTicketService{}
}

func (m *MockTicketService) CreateTicket(ctx context.Context, params ports.CreateTicketParams) (*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketService) GetTicket(ctx context.Context, ticketID int64, viewerID, orgID uuid.UUID) (*domain.Ticket, error) {
	args := m.Called(ctx, ticketID, viewerID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketService) UpdateStatus(ctx context.Context, params ports.UpdateStatusParams) (*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketService) AssignTicket(ctx context.Context, params ports.AssignTicketParams) (*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketService) ListTickets(ctx context.Context, params ports.ListTicketsParams) ([]*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Ticket), args.Error(1)
}

func (m *MockTicketService) Shutdown() {
	m.Called()
}

// MockNotifier is a mock implementation of ports.Notifier
type MockNotifier struct {
	mock.Mock
}

func NewMockNotifier() *MockNotifier {
	return &MockNotifier{}
}

func (m *MockNotifier) Notify(ctx context.Context, params ports.NotificationParams) {
	m.Called(ctx, params)
}

// MockTicketEventRepository is a mock implementation of ports.TicketEventRepository
type MockTicketEventRepository struct {
	mock.Mock
}

func NewMockTicketEventRepository() *MockTicketEventRepository {
	return &MockTicketEventRepository{}
}

func (m *MockTicketEventRepository) Create(ctx context.Context, event *domain.Event) (*domain.Event, error) {
	args := m.Called(ctx, event)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Event), args.Error(1)
}

func (m *MockTicketEventRepository) ListByTicketID(ctx context.Context, ticketID int64, afterID int64, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, ticketID, afterID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

// MockTransactionManager is a mock implementation of ports.TransactionManager
type MockTransactionManager struct {
	mock.Mock
}

func NewMockTransactionManager() *MockTransactionManager {
	return &MockTransactionManager{}
}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockTicketService) GetTicketCounts(ctx context.Context, params ports.TicketCountsParams) (*domain.TicketCounts, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TicketCounts), args.Error(1)
}

func (m *MockUserRepository) ListUsersPage(ctx context.Context, params ports.ListUsersParams) ([]*domain.UserSummary, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.UserSummary), args.Error(1)
}
func (m *MockUserRepository) GetRoles(ctx context.Context, userID, orgID uuid.UUID) ([]string, error) {
	args := m.Called(ctx, userID, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockUserRepository) ChangeOwnPassword(ctx context.Context, userID, orgID uuid.UUID, version int64, oldHash, newHash string) error {
	return m.Called(ctx, userID, orgID, version, oldHash, newHash).Error(0)
}
func (m *MockUserRepository) RevokeSessions(ctx context.Context, userID, orgID uuid.UUID, version int64) error {
	return m.Called(ctx, userID, orgID, version).Error(0)
}
func (m *MockTicketEventRepository) ListHistory(ctx context.Context, ticketID, beforeID int64, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, ticketID, beforeID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}
