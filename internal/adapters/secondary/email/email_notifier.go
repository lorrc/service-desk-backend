package email

import (
	"context"
	"log"

	"github.com/lorrc/service-desk-backend/internal/core/ports"
)

// MockSMTPNotifier is a secondary adapter that mocks sending emails.
// It implements the ports.Notifier interface.
type MockSMTPNotifier struct {
	userRepo ports.UserRepository
}

// NewMockSMTPNotifier creates a new mock notifier.
// It requires a UserRepository to fetch recipient details.
func NewMockSMTPNotifier(userRepo ports.UserRepository) ports.Notifier {
	return &MockSMTPNotifier{
		userRepo: userRepo,
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
		log.Printf("ERROR: [Notifier] Failed to get user %s for notification: %v", params.RecipientUserID, err)
		return
	}

	// 2. Log the mock email
	log.Println("--- MOCK EMAIL ---")
	log.Printf("TO: %s <%s>", user.FullName, user.Email)
	log.Printf("SUBJECT: %s", params.Subject)
	log.Printf("BODY: %s", params.Message)
	log.Printf("Ticket ID: %d", params.TicketID)
	log.Println("--- END MOCK EMAIL ---")
}
