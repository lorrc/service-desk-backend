package email

import (
	"context"
	"log/slog"

	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// MockSMTPNotifier is a secondary adapter that mocks sending emails.
// It implements the ports.Notifier interface.
type MockSMTPNotifier struct {
	userRepo ports.UserRepository
	logger   *slog.Logger
}

// NewMockSMTPNotifier creates a new mock notifier.
// It requires a UserRepository to fetch recipient details.
func NewMockSMTPNotifier(userRepo ports.UserRepository) ports.Notifier {
	return &MockSMTPNotifier{
		userRepo: userRepo,
		logger:   slog.Default().With("component", "email_notifier"),
	}
}

// NewMockSMTPNotifierWithLogger creates a new mock notifier with a custom logger.
func NewMockSMTPNotifierWithLogger(userRepo ports.UserRepository, logger *slog.Logger) ports.Notifier {
	return &MockSMTPNotifier{
		userRepo: userRepo,
		logger:   logger.With("component", "email_notifier"),
	}
}

// Notify logs the notification to the console instead of sending an email.
// It runs in a separate goroutine and should handle its own errors.
func (n *MockSMTPNotifier) Notify(ctx context.Context, params ports.NotificationParams) {
	// Use a new background context in case the original request context is cancelled.
	notifyCtx := context.Background()

	// 1. Get the recipient's details
	user, err := n.userRepo.GetByID(notifyCtx, params.RecipientUserID)
	if err != nil {
		n.logger.Error("failed to get user for notification",
			"user_id", params.RecipientUserID,
			"error", err,
		)
		return
	}

	// 2. Log the mock email
	n.logger.Info("mock email sent",
		"to_name", user.FullName,
		"to_email", user.Email,
		"subject", params.Subject,
		"ticket_id", params.TicketID,
	)
}
