package handlers

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/url"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// AdminPromptsHandler serves the admin prompts list and edit pages.
type AdminPromptsHandler struct {
	logger             *common.Logger
	templates          *template.Template
	devMode            bool
	jwtSecret          []byte
	userLookupFn       func(string) (*client.UserProfile, error)
	adminListPromptsFn func(string) ([]client.AdminPrompt, error)
	adminGetPromptFn   func(string, string) (*client.AdminPrompt, error)
	adminSetPromptFn   func(string, string, string) error
	apiURL             string
}

// NewAdminPromptsHandler creates a new admin prompts handler.
func NewAdminPromptsHandler(
	logger *common.Logger,
	devMode bool,
	jwtSecret []byte,
	userLookupFn func(string) (*client.UserProfile, error),
	listFn func(string) ([]client.AdminPrompt, error),
	getFn func(string, string) (*client.AdminPrompt, error),
	setFn func(string, string, string) error,
) *AdminPromptsHandler {
	pagesDir := FindPagesDir()

	templates := ParseTemplates(pagesDir)

	return &AdminPromptsHandler{
		logger:             logger,
		templates:          templates,
		devMode:            devMode,
		jwtSecret:          jwtSecret,
		userLookupFn:       userLookupFn,
		adminListPromptsFn: listFn,
		adminGetPromptFn:   getFn,
		adminSetPromptFn:   setFn,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *AdminPromptsHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP dispatches to list, edit, or save based on method and path.
func (h *AdminPromptsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	var userRole string
	if claims != nil && claims.Sub != "" && h.userLookupFn != nil {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			userRole = user.Role
		}
	}
	if userRole != "admin" {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}

	name := r.PathValue("name")
	if name != "" {
		var err error
		name, err = url.PathUnescape(name)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	userID := claims.Sub

	switch {
	case r.Method == http.MethodPost && name != "":
		h.handleSave(w, r, userID, name)
	case name != "":
		h.serveEdit(w, r, loggedIn, userRole, userID, name)
	default:
		h.serveList(w, r, loggedIn, userRole, userID)
	}
}

// serveList renders the prompts list page.
func (h *AdminPromptsHandler) serveList(w http.ResponseWriter, r *http.Request, loggedIn bool, userRole, userID string) {
	var prompts []client.AdminPrompt
	var fetchErr string
	if h.adminListPromptsFn != nil && userID != "" {
		var err error
		prompts, err = h.adminListPromptsFn(userID)
		if err != nil {
			if h.logger != nil {
				h.logger.Error().Str("error", err.Error()).Msg("failed to fetch admin prompt list")
			}
			fetchErr = "Failed to load prompt list. Ensure vire-server is running."
		}
	} else {
		fetchErr = "Admin API not configured. Set VIRE_SERVICE_KEY to enable."
	}

	data := map[string]interface{}{
		"Page":          "prompts",
		"DevMode":       h.devMode,
		"LoggedIn":      loggedIn,
		"UserRole":      userRole,
		"Prompts":       prompts,
		"PromptCount":   len(prompts),
		"FetchError":    fetchErr,
		"PortalVersion": config.GetVersion(),
		"ServerVersion": GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "prompts.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "prompts.html").Str("error", err.Error()).Msg("failed to render prompts page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// serveEdit renders the prompt edit page.
func (h *AdminPromptsHandler) serveEdit(w http.ResponseWriter, r *http.Request, loggedIn bool, userRole, userID, name string) {
	var prompt *client.AdminPrompt
	var fetchErr string
	if h.adminGetPromptFn != nil && userID != "" {
		var err error
		prompt, err = h.adminGetPromptFn(userID, name)
		if err != nil {
			if h.logger != nil {
				h.logger.Error().Str("error", err.Error()).Str("name", name).Msg("failed to fetch prompt")
			}
			fetchErr = "Failed to load prompt. Ensure vire-server is running."
		}
	} else {
		fetchErr = "Admin API not configured. Set VIRE_SERVICE_KEY to enable."
	}

	var promptJSON template.JS
	if prompt != nil {
		b, _ := json.Marshal(prompt)
		promptJSON = SafeJS(b)
	}

	data := map[string]interface{}{
		"Page":          "prompts",
		"DevMode":       h.devMode,
		"LoggedIn":      loggedIn,
		"UserRole":      userRole,
		"Prompt":        prompt,
		"PromptJSON":    promptJSON,
		"FetchError":    fetchErr,
		"PortalVersion": config.GetVersion(),
		"ServerVersion": GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "prompt-edit.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "prompt-edit.html").Str("error", err.Error()).Msg("failed to render prompt edit page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleSave saves updated prompt content.
func (h *AdminPromptsHandler) handleSave(w http.ResponseWriter, r *http.Request, userID, name string) {
	w.Header().Set("Content-Type", "application/json")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read request body"})
		return
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON body"})
		return
	}

	if h.adminSetPromptFn == nil || userID == "" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Admin API not configured"})
		return
	}

	if err := h.adminSetPromptFn(userID, name, payload.Content); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Str("name", name).Msg("failed to save prompt")
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save prompt"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
