package http

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is a generic JSON response for errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteJSON is a helper to standardize JSON responses.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(v)
	if err != nil {
		return
	}
}
