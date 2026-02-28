package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// ProfileHandler serves the profile page and handles profile updates.
type ProfileHandler struct {
	logger         *common.Logger
	templates      *template.Template
	devMode        bool
	jwtSecret      []byte
	userLookupFn   func(string) (*client.UserProfile, error)
	userSaveFn     func(string, map[string]string) error
	devMCPEndpoint func(userID string) string
	apiURL         string
}

// NewProfileHandler creates a new profile handler.
func NewProfileHandler(logger *common.Logger, devMode bool, jwtSecret []byte, userLookupFn func(string) (*client.UserProfile, error), userSaveFn func(string, map[string]string) error) *ProfileHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &ProfileHandler{
		logger:       logger,
		templates:    templates,
		devMode:      devMode,
		jwtSecret:    jwtSecret,
		userLookupFn: userLookupFn,
		userSaveFn:   userSaveFn,
	}
}

// SetDevMCPEndpointFn sets the function to generate dev-mode MCP endpoints.
func (h *ProfileHandler) SetDevMCPEndpointFn(fn func(userID string) string) {
	h.devMCPEndpoint = fn
}

// SetAPIURL sets the API URL for server version fetching.
func (h *ProfileHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// HandleProfile serves GET /profile.
func (h *ProfileHandler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)

	// Redirect unauthenticated users to landing page
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	csrfToken := ""
	if csrfCookie, err := r.Cookie("_csrf"); err == nil {
		csrfToken = csrfCookie.Value
	}

	// Determine user info from claims
	userEmail := ""
	userName := ""
	authMethod := ""
	isOAuth := false
	if claims != nil {
		userEmail = claims.Email
		userName = claims.Name
		authMethod = claims.Provider
		isOAuth = claims.Provider == "google" || claims.Provider == "github"
	}

	// Fall back to user profile for name if claims don't have it
	if userName == "" && claims != nil && claims.Sub != "" && h.userLookupFn != nil {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil && user.Username != "" {
			userName = user.Username
		}
	}

	data := map[string]interface{}{
		"Page":             "profile",
		"DevMode":          h.devMode,
		"LoggedIn":         loggedIn,
		"NavexaKeySet":     false,
		"NavexaKeyPreview": "",
		"Saved":            r.URL.Query().Get("saved") == "1",
		"CSRFToken":        csrfToken,
		"PortalVersion":    config.GetVersion(),
		"ServerVersion":    GetServerVersion(h.apiURL),
		"UserEmail":        userEmail,
		"UserName":         userName,
		"AuthMethod":       authMethod,
		"IsOAuth":          isOAuth,
	}

	if claims != nil && claims.Sub != "" && h.userLookupFn != nil {
		user, err := h.userLookupFn(claims.Sub)
		if err == nil && user != nil {
			data["NavexaKeySet"] = user.NavexaKeySet
			data["NavexaKeyPreview"] = user.NavexaKeyPreview
		}
	}

	if h.devMode && claims != nil {
		if cookie, err := r.Cookie("vire_session"); err == nil {
			data["JWTToken"] = cookie.Value
		}
		data["JWTSub"] = claims.Sub
		data["JWTEmail"] = claims.Email
		data["JWTName"] = claims.Name
		data["JWTProvider"] = claims.Provider
		data["JWTIssuer"] = claims.Iss
		data["JWTIssuedAt"] = time.Unix(claims.Iat, 0).UTC().Format(time.RFC3339)
		data["JWTExpires"] = time.Unix(claims.Exp, 0).UTC().Format(time.RFC3339)
		data["JWTExpired"] = claims.Exp < time.Now().Unix()

		// Generate dev MCP endpoint if function is available
		if h.devMCPEndpoint != nil && claims.Sub != "" {
			data["DevMCPEndpoint"] = h.devMCPEndpoint(claims.Sub)
		}
	}

	if err := h.templates.ExecuteTemplate(w, "profile.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "profile.html").Str("error", err.Error()).Msg("failed to render profile")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleSaveProfile handles POST /profile.
func (h *ProfileHandler) HandleSaveProfile(w http.ResponseWriter, r *http.Request) {
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
			h.logger.Error().Str("error", err.Error()).Msg("failed to save user profile")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/profile?saved=1", http.StatusFound)
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
