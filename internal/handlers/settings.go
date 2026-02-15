package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bobmcallan/vire-portal/internal/client"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// SettingsHandler serves the settings page and handles settings updates.
type SettingsHandler struct {
	logger       *common.Logger
	templates    *template.Template
	devMode      bool
	jwtSecret    []byte
	userLookupFn func(string) (*client.UserProfile, error)
	userSaveFn   func(string, map[string]string) error
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error), userSaveFn func(string, map[string]string) error) *SettingsHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &SettingsHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
		userSaveFn:   userSaveFn,
	}
}

// HandleSettings serves GET /settings.
func (h *SettingsHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	csrfToken := ""
	if csrfCookie, err := r.Cookie("_csrf"); err == nil {
		csrfToken = csrfCookie.Value
	}

	data := map[string]interface{}{
		"Page":             "settings",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeySet":     false,
		"NavexaKeyPreview": "",
		"Saved":            r.URL.Query().Get("saved") == "1",
		"CSRFToken":        csrfToken,
	}

	if loggedIn && claims != nil && claims.Sub != "" && h.userLookupFn != nil {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			data["NavexaKeySet"] = user.NavexaKeySet
			data["NavexaKeyPreview"] = user.NavexaKeyPreview
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
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
	if !loggedIn || claims == nil || claims.Sub == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.userSaveFn == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	navexaKey := strings.TrimSpace(r.FormValue("navexa_key"))

	if err := h.userSaveFn(claims.Sub, map[string]string{"navexa_key": navexaKey}); err != nil {
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
// Deprecated: Use IsLoggedIn and JWTClaims.Sub instead.
func ExtractJWTSub(token string) string {
	claims, err := ValidateJWT(token, []byte{})
	if err != nil {
		return ""
	}
	return claims.Sub
}
