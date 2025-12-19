package mocks

import (
	"context"

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

func (m *MockTicketRepository) GetByID(ctx context.Context, id int64) (*domain.Ticket, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Ticket), args.Error(1)
}

func (m *MockTicketRepository) Update(ctx context.Context, ticket *domain.Ticket) (*domain.Ticket, error) {
	args := m.Called(ctx, ticket)
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

func (m *MockTicketRepository) ListByRequesterPaginated(ctx context.Context, params ports.ListTicketsRepoParams) ([]*domain.Ticket, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Ticket), args.Error(1)
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

func (m *MockTicketService) GetTicket(ctx context.Context, ticketID int64, viewerID uuid.UUID) (*domain.Ticket, error) {
	args := m.Called(ctx, ticketID, viewerID)
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

// MockEventBroadcaster is a mock implementation of ports.EventBroadcaster
type MockEventBroadcaster struct {
	mock.Mock
}

func NewMockEventBroadcaster() *MockEventBroadcaster {
	return &MockEventBroadcaster{}
}

func (m *MockEventBroadcaster) Broadcast(event domain.Event) error {
	args := m.Called(event)
	return args.Error(0)
}
