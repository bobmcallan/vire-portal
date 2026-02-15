package handlers

import (
	"encoding/json"
	"net/http"
)

// RequireMethod validates that the HTTP request uses the specified method.
// Returns true if the method matches, false otherwise (and writes error response).
func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method || (method == http.MethodGet && r.Method == http.MethodHead) {
		return true
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

// WriteJSON writes a JSON response with the specified status code and data.
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(data)
}

// WriteError writes a standard error JSON response.
func WriteError(w http.ResponseWriter, statusCode int, message string) error {
	return WriteJSON(w, statusCode, map[string]string{
		"status": "error",
		"error":  message,
	})
}
