package domain

import (
	"net/mail"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
)

// Password validation constants
const (
	MinPasswordLength = 8
	MaxPasswordLength = 128
	MaxFullNameLength = 255
	MaxEmailLength    = 255
)

// PasswordRequirements defines what a valid password needs
type PasswordRequirements struct {
	MinLength        int
	RequireUppercase bool
	RequireLowercase bool
	RequireNumber    bool
	RequireSpecial   bool
}

// DefaultPasswordRequirements returns the default password requirements
func DefaultPasswordRequirements() PasswordRequirements {
	return PasswordRequirements{
		MinLength:        MinPasswordLength,
		RequireUppercase: true,
		RequireLowercase: true,
		RequireNumber:    true,
		RequireSpecial:   false, // Optional for now
	}
}

type User struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	FullName       string
	Email          string
	HashedPassword string
	CreatedAt      time.Time
	IsActive       bool
	LastActiveAt   *time.Time
}

type UserSummary struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	FullName       string
	Email          string
	Roles          []string
	IsActive       bool
	CreatedAt      time.Time
	LastActiveAt   *time.Time
}

// UserRegistrationParams holds parameters for user registration
type UserRegistrationParams struct {
	FullName string
	Email    string
	Password string
}

// Validate validates user registration parameters
func (p *UserRegistrationParams) Validate() error {
	errs := apperrors.NewValidationErrors()

	// Validate full name
	if p.FullName == "" {
		errs.Add("fullName", "Full name is required")
	} else if len(p.FullName) > MaxFullNameLength {
		errs.Add("fullName", "Full name must be 255 characters or less")
	}

	// Validate email
	if p.Email == "" {
		errs.Add("email", "Email is required")
	} else if len(p.Email) > MaxEmailLength {
		errs.Add("email", "Email must be 255 characters or less")
	} else if !isValidEmail(p.Email) {
		errs.Add("email", "Invalid email format")
	}

	// Validate password
	if passwordErrs := ValidatePassword(p.Password); len(passwordErrs) > 0 {
		for _, err := range passwordErrs {
			errs.Add("password", err)
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidatePassword checks if a password meets security requirements
// Returns a slice of error messages (empty if valid)
func ValidatePassword(password string) []string {
	var errors []string
	requirements := DefaultPasswordRequirements()

	if len(password) < requirements.MinLength {
		errors = append(errors, "Password must be at least 8 characters long")
	}

	if len(password) > MaxPasswordLength {
		errors = append(errors, "Password must be 128 characters or less")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if requirements.RequireUppercase && !hasUpper {
		errors = append(errors, "Password must contain at least one uppercase letter")
	}
	if requirements.RequireLowercase && !hasLower {
		errors = append(errors, "Password must contain at least one lowercase letter")
	}
	if requirements.RequireNumber && !hasNumber {
		errors = append(errors, "Password must contain at least one number")
	}
	if requirements.RequireSpecial && !hasSpecial {
		errors = append(errors, "Password must contain at least one special character")
	}

	return errors
}

// IsPasswordValid checks if a password is valid (returns bool for simple checks)
func IsPasswordValid(password string) bool {
	return len(ValidatePassword(password)) == 0
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// CheckPassword verifies if the provided password matches the stored hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(password))
	return err == nil
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	// Validate password first
	if errs := ValidatePassword(password); len(errs) > 0 {
		return "", apperrors.ErrPasswordTooWeak
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// NewUser creates a new user with validated parameters
func NewUser(params UserRegistrationParams, orgID uuid.UUID) (*User, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	hashedPassword, err := HashPassword(params.Password)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		FullName:       params.FullName,
		Email:          params.Email,
		HashedPassword: hashedPassword,
		CreatedAt:      time.Now().UTC(),
		IsActive:       true,
	}, nil
}
