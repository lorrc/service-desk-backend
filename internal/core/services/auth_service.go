package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	// ... imports
	"github.com/lorrc/service-desk-backend/internal/core/domain"
	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

type AuthService struct {
	userRepo ports.UserRepository
}

// Ensure interface satisfaction
var _ ports.AuthService = (*AuthService)(nil)

func NewAuthService(userRepo ports.UserRepository) ports.AuthService {
	return &AuthService{userRepo: userRepo}
}

func (s *AuthService) Register(ctx context.Context, fullName, email, password string, orgID uuid.UUID) (*domain.User, error) {
	// 1. Check if user exists (simplified check)
	existing, _ := s.userRepo.GetByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("user already exists")
	}

	// 2. Hash password
	hashedPassword, err := domain.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// 3. Create domain object
	user := &domain.User{
		Email:          email,
		FullName:       fullName,
		PasswordHash:   hashedPassword,
		OrganizationID: orgID,
	}

	// 4. Persist
	return s.userRepo.Create(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	// Check error or nil user to prevent user enumeration attacks
	if err != nil || user == nil {
		return nil, errors.New("invalid credentials")
	}

	if !user.CheckPassword(password) {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}
