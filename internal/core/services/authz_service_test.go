package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
	"github.com/stretchr/testify/require"
)

type fakeAuthRepo struct {
	permissions      []string
	permissionsAfter []string
	assignErr        error
	assignCalls      int
}

func (f *fakeAuthRepo) GetUserPermissions(_ context.Context, _ uuid.UUID) ([]string, error) {
	if f.assignCalls > 0 {
		return f.permissionsAfter, nil
	}
	return f.permissions, nil
}

func (f *fakeAuthRepo) AssignRole(_ context.Context, _ uuid.UUID, _ string) error {
	f.assignCalls++
	return f.assignErr
}

func (f *fakeAuthRepo) SetUserRole(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

func (f *fakeAuthRepo) EnsureRBACDefaults(_ context.Context) error {
	return nil
}

func TestAuthorizationService_GetPermissions_AssignsDefaultRoleWhenMissing(t *testing.T) {
	repo := &fakeAuthRepo{
		permissions:      []string{},
		permissionsAfter: []string{"tickets:create", "tickets:read"},
	}
	svc := NewAuthorizationService(repo)

	permissions, err := svc.GetPermissions(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Equal(t, []string{"tickets:create", "tickets:read"}, permissions)
	require.Equal(t, 1, repo.assignCalls)
}

func TestAuthorizationService_GetPermissions_IgnoresAlreadyAssigned(t *testing.T) {
	repo := &fakeAuthRepo{
		permissions:      []string{},
		permissionsAfter: []string{},
		assignErr:        apperrors.ErrRoleAlreadyAssigned,
	}
	svc := NewAuthorizationService(repo)

	permissions, err := svc.GetPermissions(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Empty(t, permissions)
	require.Equal(t, 1, repo.assignCalls)
}

func TestAuthorizationService_GetPermissions_ReturnsErrorOnAssignFailure(t *testing.T) {
	repo := &fakeAuthRepo{
		permissions:      []string{},
		permissionsAfter: []string{},
		assignErr:        errors.New("boom"),
	}
	svc := NewAuthorizationService(repo)

	_, err := svc.GetPermissions(context.Background(), uuid.New())
	require.Error(t, err)
	require.Equal(t, 1, repo.assignCalls)
}

func TestAuthorizationService_Can_UsesEnsuredPermissions(t *testing.T) {
	repo := &fakeAuthRepo{
		permissions:      []string{},
		permissionsAfter: []string{"tickets:create"},
	}
	svc := NewAuthorizationService(repo)

	allowed, err := svc.Can(context.Background(), uuid.New(), "tickets:create")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, 1, repo.assignCalls)
}
