package http

import (
	"encoding/json"
	"net/http"
)

// PaginatedResponse wraps paginated data with metadata
type PaginatedResponse[T any] struct {
	Data       []T                `json:"data"`
	Pagination PaginationMetadata `json:"pagination"`
}

// PaginationMetadata contains pagination information
type PaginationMetadata struct {
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	TotalCount int64 `json:"totalCount,omitempty"`
	HasMore    bool  `json:"hasMore"`
}

// SuccessResponse wraps a successful response
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ListResponse wraps a list of items (non-paginated)
type ListResponse[T any] struct {
	Data  []T `json:"data"`
	Count int `json:"count"`
}

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Log the error but don't try to write again
		// The header has already been sent
	}
}

// WriteJSONWithHeaders writes a JSON response with custom headers
func WriteJSONWithHeaders(w http.ResponseWriter, status int, v any, headers map[string]string) {
	for key, value := range headers {
		w.Header().Set(key, value)
	}
	WriteJSON(w, status, v)
}

// WriteSuccess writes a success response
func WriteSuccess(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, SuccessResponse{Data: data})
}

// WriteCreated writes a created response
func WriteCreated(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, data)
}

// WriteNoContent writes a no content response
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WritePaginated writes a paginated response
func WritePaginated[T any](w http.ResponseWriter, data []T, limit, offset int, totalCount int64) {
	hasMore := int64(offset+len(data)) < totalCount

	response := PaginatedResponse[T]{
		Data: data,
		Pagination: PaginationMetadata{
			Limit:      limit,
			Offset:     offset,
			TotalCount: totalCount,
			HasMore:    hasMore,
		},
	}

	WriteJSON(w, http.StatusOK, response)
}

// WritePaginatedSimple writes a paginated response without total count
// Useful when counting total records is expensive
func WritePaginatedSimple[T any](w http.ResponseWriter, data []T, limit, offset int) {
	// FIX: logic to determine hasMore based on fetching limit + 1
	// If we received more items than the requested limit, we know there are more pages.
	hasMore := len(data) > limit

	responseItems := data
	if hasMore {
		// Slice the data to return only the requested amount
		responseItems = data[:limit]
	}

	response := PaginatedResponse[T]{
		Data: responseItems,
		Pagination: PaginationMetadata{
			Limit:   limit,
			Offset:  offset,
			HasMore: hasMore,
		},
	}

	WriteJSON(w, http.StatusOK, response)
}

// WriteList writes a simple list response
func WriteList[T any](w http.ResponseWriter, data []T) {
	response := ListResponse[T]{
		Data:  data,
		Count: len(data),
	}

	WriteJSON(w, http.StatusOK, response)
}
