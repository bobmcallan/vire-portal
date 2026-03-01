package handlers

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/bobmcallan/vire-portal/internal/client"
	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// AdminUsersHandler serves the admin users page.
type AdminUsersHandler struct {
	logger           *common.Logger
	templates        *template.Template
	devMode          bool
	jwtSecret        []byte
	userLookupFn     func(string) (*client.UserProfile, error)
	adminListUsersFn func(string) ([]client.AdminUser, error)
	serviceUserID    string
	apiURL           string
}

// NewAdminUsersHandler creates a new admin users handler.
func NewAdminUsersHandler(
	logger *common.Logger,
	devMode bool,
	jwtSecret []byte,
	userLookupFn func(string) (*client.UserProfile, error),
	adminListUsersFn func(string) ([]client.AdminUser, error),
	serviceUserID string,
) *AdminUsersHandler {
	pagesDir := FindPagesDir()

	templates := template.Must(template.ParseGlob(filepath.Join(pagesDir, "*.html")))
	template.Must(templates.ParseGlob(filepath.Join(pagesDir, "partials", "*.html")))

	return &AdminUsersHandler{
		logger:           logger,
		templates:        templates,
		devMode:          devMode,
		jwtSecret:        jwtSecret,
		userLookupFn:     userLookupFn,
		adminListUsersFn: adminListUsersFn,
		serviceUserID:    serviceUserID,
	}
}

// SetAPIURL sets the API URL for server version fetching.
func (h *AdminUsersHandler) SetAPIURL(apiURL string) {
	h.apiURL = apiURL
}

// ServeHTTP renders the admin users page.
func (h *AdminUsersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	loggedIn, claims := IsLoggedIn(r, h.jwtSecret)
	if !loggedIn {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Gate: require admin role
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

	// Fetch user list via admin API
	var users []client.AdminUser
	var fetchErr string
	if h.adminListUsersFn != nil && h.serviceUserID != "" {
		var err error
		users, err = h.adminListUsersFn(h.serviceUserID)
		if err != nil {
			if h.logger != nil {
				h.logger.Error().Str("error", err.Error()).Msg("failed to fetch admin user list")
			}
			fetchErr = "Failed to load user list. Ensure vire-server is running."
		}
	} else {
		fetchErr = "Admin API not configured. Set VIRE_SERVICE_KEY to enable."
	}

	data := map[string]interface{}{
		"Page":          "users",
		"DevMode":       h.devMode,
		"LoggedIn":      loggedIn,
		"UserRole":      userRole,
		"Users":         users,
		"UserCount":     len(users),
		"FetchError":    fetchErr,
		"PortalVersion": config.GetVersion(),
		"ServerVersion": GetServerVersion(h.apiURL),
	}

	if err := h.templates.ExecuteTemplate(w, "users.html", data); err != nil {
		if h.logger != nil {
			h.logger.Error().Str("template", "users.html").Str("error", err.Error()).Msg("failed to render users page")
		}
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
