package validation

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	apperrors "github.com/lorrc/service-desk-backend/internal/core/errors"
)

// Common validation regex patterns
var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	uuidRegex  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// Validator validates request data
type Validator struct {
	errors *apperrors.ValidationErrors
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		errors: apperrors.NewValidationErrors(),
	}
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return v.errors.HasErrors()
}

// Errors returns the validation errors
func (v *Validator) Errors() *apperrors.ValidationErrors {
	return v.errors
}

// Required validates that a string is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors.Add(field, "This field is required")
	}
	return v
}

// RequiredIf validates that a string is not empty if condition is true
func (v *Validator) RequiredIf(field, value string, condition bool, message string) *Validator {
	if condition && strings.TrimSpace(value) == "" {
		v.errors.Add(field, message)
	}
	return v
}

// MinLength validates minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.errors.Add(field, "Must be at least "+strconv.Itoa(min)+" characters")
	}
	return v
}

// MaxLength validates maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.errors.Add(field, "Must be at most "+strconv.Itoa(max)+" characters")
	}
	return v
}

// Length validates exact string length
func (v *Validator) Length(field, value string, length int) *Validator {
	if len(value) != length {
		v.errors.Add(field, "Must be exactly "+strconv.Itoa(length)+" characters")
	}
	return v
}

// Email validates email format
func (v *Validator) Email(field, value string) *Validator {
	if value != "" && !emailRegex.MatchString(value) {
		v.errors.Add(field, "Must be a valid email address")
	}
	return v
}

// UUID validates UUID format
func (v *Validator) UUID(field, value string) *Validator {
	if value != "" && !uuidRegex.MatchString(value) {
		v.errors.Add(field, "Must be a valid UUID")
	}
	return v
}

// Min validates minimum integer value
func (v *Validator) Min(field string, value, min int) *Validator {
	if value < min {
		v.errors.Add(field, "Must be at least "+strconv.Itoa(min))
	}
	return v
}

// Max validates maximum integer value
func (v *Validator) Max(field string, value, max int) *Validator {
	if value > max {
		v.errors.Add(field, "Must be at most "+strconv.Itoa(max))
	}
	return v
}

// Range validates integer is within range
func (v *Validator) Range(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.errors.Add(field, "Must be between "+strconv.Itoa(min)+" and "+strconv.Itoa(max))
	}
	return v
}

// OneOf validates value is one of the allowed values
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	if value == "" {
		return v // Empty is handled by Required
	}

	for _, a := range allowed {
		if value == a {
			return v
		}
	}

	v.errors.Add(field, "Must be one of: "+strings.Join(allowed, ", "))
	return v
}

// Matches validates value matches a regex pattern
func (v *Validator) Matches(field, value string, pattern *regexp.Regexp, message string) *Validator {
	if value != "" && !pattern.MatchString(value) {
		v.errors.Add(field, message)
	}
	return v
}

// Custom adds a custom validation
func (v *Validator) Custom(field string, valid bool, message string) *Validator {
	if !valid {
		v.errors.Add(field, message)
	}
	return v
}

// NotNil validates that a pointer is not nil
func (v *Validator) NotNil(field string, value interface{}) *Validator {
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		v.errors.Add(field, "This field is required")
	}
	return v
}

// DecodeAndValidate decodes JSON request body and runs basic validation
func DecodeAndValidate[T any](r *http.Request) (*T, error) {
	var req T

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, apperrors.NewBadRequestError(err, "Invalid request body")
	}

	return &req, nil
}

// PaginationParams holds pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// DefaultPagination returns default pagination values
func DefaultPagination() PaginationParams {
	return PaginationParams{
		Limit:  25,
		Offset: 0,
	}
}

// ParsePagination extracts and validates pagination from query parameters
func ParsePagination(r *http.Request, maxLimit int) PaginationParams {
	params := DefaultPagination()

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	// Enforce maximum limit
	if params.Limit > maxLimit {
		params.Limit = maxLimit
	}

	return params
}

// ParseIntQueryParam safely parses an integer query parameter
func ParseIntQueryParam(r *http.Request, key string, defaultValue int) int {
	valueStr := r.URL.Query().Get(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value < 0 {
		return defaultValue
	}

	return value
}

// ParseStringQueryParam safely parses a string query parameter
func ParseStringQueryParam(r *http.Request, key string) *string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return nil
	}
	return &value
}

// ParseBoolQueryParam safely parses a boolean query parameter
func ParseBoolQueryParam(r *http.Request, key string, defaultValue bool) bool {
	valueStr := r.URL.Query().Get(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}
