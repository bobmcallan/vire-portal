package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/bobmcallan/vire-portal/internal/vire/common"
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

// safeScriptRe matches </script (case-insensitive) which could break out of a <script> block.
var safeScriptRe = regexp.MustCompile(`(?i)</script`)

// SafeJS sanitizes raw JSON bytes for safe embedding in HTML <script> blocks
// via template.JS. template.JS trusts its content and does not escape it, so
// raw bytes containing "</script>" would break out of the script block (XSS).
// This escapes the "/" in any "</script" sequence to "<\/script".
func SafeJS(raw []byte) template.JS {
	return template.JS(safeScriptRe.ReplaceAll(raw, []byte(`<\/script`)))
}

// ParseTemplates parses page and partial templates with a shared FuncMap.
// Provides {{cacheBust}} for cache-busting static asset URLs.
func ParseTemplates(pagesDir string) *template.Template {
	funcMap := template.FuncMap{
		"cacheBust": func() string { return common.Build },
	}
	t := template.Must(template.New("").Funcs(funcMap).ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(t.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))
	return t
}

// WriteError writes a standard error JSON response.
func WriteError(w http.ResponseWriter, statusCode int, message string) error {
	return WriteJSON(w, statusCode, map[string]string{
		"status": "error",
		"error":  message,
	})
}
