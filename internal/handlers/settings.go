package handlers

import (
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bobmcallan/vire-portal/internal/models"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// SettingsHandler serves the settings page and handles settings updates.
type SettingsHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	userLookupFn func(string) (*models.User, error)
	userSaveFn   func(*models.User) error
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(logger *common.Logger, devMode bool, userLookupFn func(string) (*models.User, error), userSaveFn func(*models.User) error) *SettingsHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &SettingsHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		userLookupFn: userLookupFn,
		userSaveFn:   userSaveFn,
	}
}

// HandleSettings serves GET /settings.
func (h *SettingsHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("vire_session")
	loggedIn := err == nil && cookie.Value != ""

	csrfToken := ""
	if csrfCookie, err := r.Cookie("_csrf"); err == nil {
		csrfToken = csrfCookie.Value
	}

	data := map[string]interface{}{
		"Page":             "settings",
		"PageTitle":        "SETTINGS",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeySet":     false,
		"NavexaKeyPreview": "",
		"Saved":            r.URL.Query().Get("saved") == "1",
		"CSRFToken":        csrfToken,
	}

	if loggedIn {
		sub := ExtractJWTSub(cookie.Value)
		if sub != "" && h.userLookupFn != nil {
			user, err := h.userLookupFn(sub)
			if err == nil && user != nil {
				if user.NavexaKey != "" {
					data["NavexaKeySet"] = true
					if len(user.NavexaKey) >= 4 {
						data["NavexaKeyPreview"] = user.NavexaKey[len(user.NavexaKey)-4:]
					} else {
						data["NavexaKeyPreview"] = user.NavexaKey
					}
				}
			}
		}
	}

	if err := h.templates.ExecuteTemplate(w, "settings.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "settings.html").Str("error", err.Error()).Msg("failed to render settings")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleSaveSettings handles POST /settings.
func (h *SettingsHandler) HandleSaveSettings(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("vire_session")
	if err != nil || cookie.Value == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sub := ExtractJWTSub(cookie.Value)
	if sub == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.userLookupFn == nil || h.userSaveFn == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user, err := h.userLookupFn(sub)
	if err != nil || user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	navexaKey := strings.TrimSpace(r.FormValue("navexa_key"))
	user.NavexaKey = navexaKey

	if err := h.userSaveFn(user); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("error", err.Error()).Msg("failed to save user settings")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings?saved=1", http.StatusFound)
}

// ExtractJWTSub base64url-decodes the JWT payload (middle segment)
// and returns the "sub" claim. Returns empty string on any failure.
func ExtractJWTSub(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) < 2 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	return claims.Sub
}
