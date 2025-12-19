package errors

import (
	"errors"
	"fmt"
)

// Domain errors - these represent business rule violations
var (
	// Authentication & Authorization
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrForbidden          = errors.New("action forbidden")
	ErrUnauthorized       = errors.New("unauthorized")

	// User validation
	ErrUserNotFound     = errors.New("user not found")
	ErrEmailRequired    = errors.New("email is required")
	ErrEmailInvalid     = errors.New("email format is invalid")
	ErrPasswordTooWeak  = errors.New("password does not meet security requirements")
	ErrPasswordRequired = errors.New("password is required")
	ErrFullNameRequired = errors.New("full name is required")
	ErrFullNameTooLong  = errors.New("full name exceeds maximum length")

	// Ticket validation
	ErrTicketNotFound          = errors.New("ticket not found")
	ErrTitleRequired           = errors.New("title is required")
	ErrTitleTooLong            = errors.New("title exceeds maximum length of 255 characters")
	ErrDescriptionTooLong      = errors.New("description exceeds maximum length")
	ErrInvalidPriority         = errors.New("invalid ticket priority")
	ErrInvalidStatus           = errors.New("invalid ticket status")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrRequesterRequired       = errors.New("requester ID is required")
	ErrCannotAssignClosed      = errors.New("cannot assign a closed ticket")

	// Comment validation
	ErrCommentBodyRequired = errors.New("comment body is required")
	ErrCommentBodyTooLong  = errors.New("comment body exceeds maximum length")
	ErrTicketIDRequired    = errors.New("ticket ID is required")
	ErrAuthorIDRequired    = errors.New("author ID is required")

	// Generic
	ErrNotFound    = errors.New("resource not found")
	ErrInternal    = errors.New("internal server error")
	ErrBadRequest  = errors.New("bad request")
	ErrConflict    = errors.New("resource conflict")
	ErrRateLimited = errors.New("rate limit exceeded")
)

// AppError wraps errors with additional context for HTTP responses
type AppError struct {
	Err        error  // The underlying error
	Message    string // User-friendly message
	Code       string // Machine-readable error code
	StatusCode int    // HTTP status code
	Details    map[string]interface{}
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Error constructors for common cases
func NewBadRequestError(err error, message string) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		Code:       "BAD_REQUEST",
		StatusCode: 400,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Err:        ErrUnauthorized,
		Message:    message,
		Code:       "UNAUTHORIZED",
		StatusCode: 401,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Err:        ErrForbidden,
		Message:    message,
		Code:       "FORBIDDEN",
		StatusCode: 403,
	}
}

func NewNotFoundError(err error, message string) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		Code:       "NOT_FOUND",
		StatusCode: 404,
	}
}

func NewConflictError(err error, message string) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		Code:       "CONFLICT",
		StatusCode: 409,
	}
}

func NewValidationError(err error, message string, details map[string]interface{}) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		Code:       "VALIDATION_ERROR",
		StatusCode: 422,
		Details:    details,
	}
}

func NewRateLimitError() *AppError {
	return &AppError{
		Err:        ErrRateLimited,
		Message:    "Too many requests. Please try again later.",
		Code:       "RATE_LIMITED",
		StatusCode: 429,
	}
}

func NewInternalError(err error) *AppError {
	return &AppError{
		Err:        err,
		Message:    "An unexpected error occurred",
		Code:       "INTERNAL_ERROR",
		StatusCode: 500,
	}
}

// ValidationErrors holds multiple field validation errors
type ValidationErrors struct {
	Errors map[string][]string `json:"errors"`
}

func NewValidationErrors() *ValidationErrors {
	return &ValidationErrors{
		Errors: make(map[string][]string),
	}
}

func (v *ValidationErrors) Add(field, message string) {
	v.Errors[field] = append(v.Errors[field], message)
}

func (v *ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

func (v *ValidationErrors) Error() string {
	return fmt.Sprintf("validation failed: %d field(s) have errors", len(v.Errors))
}
