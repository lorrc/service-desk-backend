package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	mw "github.com/lorrc/service-desk-backend/internal/adapters/primary/http/middleware"
	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
)

// GetRequestID retrieves the request ID from context
func GetRequestID(ctx context.Context) string {
	return mw.GetRequestID(ctx)
}

// ErrorResponse is the standard JSON error response format
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ValidationErrorResponse includes field-level validation errors
type ValidationErrorResponse struct {
	Error  string              `json:"error"`
	Code   string              `json:"code"`
	Fields map[string][]string `json:"fields,omitempty"`
}

// ErrorHandler provides centralized error handling with logging
type ErrorHandler struct {
	logger *slog.Logger
}

// NewErrorHandler creates a new error handler with the given logger
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

// Handle processes an error and writes the appropriate HTTP response
func (h *ErrorHandler) Handle(w http.ResponseWriter, r *http.Request, err error) {
	requestID := GetRequestID(r.Context())

	// Check for AppError first (our custom error type)
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		h.logError(r, appErr.StatusCode, appErr.Err, requestID)
		h.writeErrorResponse(w, appErr.StatusCode, ErrorResponse{
			Error:   appErr.Message,
			Code:    appErr.Code,
			Details: appErr.Details,
		})
		return
	}

	// Check for ValidationErrors
	var validationErrs *apperrors.ValidationErrors
	if errors.As(err, &validationErrs) {
		h.logError(r, http.StatusUnprocessableEntity, err, requestID)
		h.writeValidationErrorResponse(w, validationErrs)
		return
	}

	// Map known domain errors to HTTP responses
	statusCode, response := h.mapDomainError(err)
	h.logError(r, statusCode, err, requestID)
	h.writeErrorResponse(w, statusCode, response)
}

// mapDomainError converts domain errors to HTTP status codes and responses
func (h *ErrorHandler) mapDomainError(err error) (int, ErrorResponse) {
	switch {
	// Authentication & Authorization
	case errors.Is(err, apperrors.ErrInvalidCredentials):
		return http.StatusUnauthorized, ErrorResponse{
			Error: "Invalid credentials",
			Code:  "INVALID_CREDENTIALS",
		}
	case errors.Is(err, apperrors.ErrUnauthorized):
		return http.StatusUnauthorized, ErrorResponse{
			Error: "Authentication required",
			Code:  "UNAUTHORIZED",
		}
	case errors.Is(err, apperrors.ErrForbidden):
		return http.StatusForbidden, ErrorResponse{
			Error: "You do not have permission to perform this action",
			Code:  "FORBIDDEN",
		}

	// Not Found errors
	case errors.Is(err, apperrors.ErrUserNotFound):
		return http.StatusNotFound, ErrorResponse{
			Error: "User not found",
			Code:  "USER_NOT_FOUND",
		}
	case errors.Is(err, apperrors.ErrTicketNotFound):
		return http.StatusNotFound, ErrorResponse{
			Error: "Ticket not found",
			Code:  "TICKET_NOT_FOUND",
		}

	// Conflict errors
	case errors.Is(err, apperrors.ErrUserExists):
		return http.StatusConflict, ErrorResponse{
			Error: "A user with this email already exists",
			Code:  "USER_EXISTS",
		}

	// Validation errors
	case errors.Is(err, apperrors.ErrTitleRequired),
		errors.Is(err, apperrors.ErrTitleTooLong),
		errors.Is(err, apperrors.ErrDescriptionTooLong),
		errors.Is(err, apperrors.ErrInvalidPriority),
		errors.Is(err, apperrors.ErrInvalidStatus),
		errors.Is(err, apperrors.ErrCommentBodyRequired),
		errors.Is(err, apperrors.ErrCommentBodyTooLong),
		errors.Is(err, apperrors.ErrEmailRequired),
		errors.Is(err, apperrors.ErrEmailInvalid),
		errors.Is(err, apperrors.ErrPasswordTooWeak),
		errors.Is(err, apperrors.ErrPasswordRequired),
		errors.Is(err, apperrors.ErrFullNameRequired):
		return http.StatusBadRequest, ErrorResponse{
			Error: err.Error(),
			Code:  "VALIDATION_ERROR",
		}

	// Business rule violations
	case errors.Is(err, apperrors.ErrInvalidStatusTransition):
		return http.StatusBadRequest, ErrorResponse{
			Error: "Invalid status transition",
			Code:  "INVALID_STATUS_TRANSITION",
		}
	case errors.Is(err, apperrors.ErrCannotAssignClosed):
		return http.StatusBadRequest, ErrorResponse{
			Error: "Cannot assign a closed ticket",
			Code:  "CANNOT_ASSIGN_CLOSED",
		}

	// Rate limiting
	case errors.Is(err, apperrors.ErrRateLimited):
		return http.StatusTooManyRequests, ErrorResponse{
			Error: "Too many requests. Please try again later.",
			Code:  "RATE_LIMITED",
		}

	// Default to internal server error
	default:
		return http.StatusInternalServerError, ErrorResponse{
			Error: "An unexpected error occurred",
			Code:  "INTERNAL_ERROR",
		}
	}
}

// logError logs the error with appropriate context
func (h *ErrorHandler) logError(r *http.Request, statusCode int, err error, requestID string) {
	logAttrs := []any{
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
		"status_code", statusCode,
		"error", err.Error(),
	}

	// Log at different levels based on status code
	switch {
	case statusCode >= 500:
		h.logger.Error("server error", logAttrs...)
	case statusCode >= 400:
		h.logger.Warn("client error", logAttrs...)
	default:
		h.logger.Info("request error", logAttrs...)
	}
}

// writeErrorResponse writes a JSON error response
func (h *ErrorHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, response ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// writeValidationErrorResponse writes a validation error response
func (h *ErrorHandler) writeValidationErrorResponse(w http.ResponseWriter, errs *apperrors.ValidationErrors) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(ValidationErrorResponse{
		Error:  "Validation failed",
		Code:   "VALIDATION_ERROR",
		Fields: errs.Errors,
	})
}

// HandleError Helper function to handle errors inline in handlers
// Usage: if HandleError(w, r, err, h.errorHandler) { return }
func HandleError(w http.ResponseWriter, r *http.Request, err error, handler *ErrorHandler) bool {
	if err != nil {
		handler.Handle(w, r, err)
		return true
	}
	return false
}
