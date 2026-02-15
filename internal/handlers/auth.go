package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	logger  *common.Logger
	devMode bool
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(logger *common.Logger, devMode bool) *AuthHandler {
	return &AuthHandler{
		logger:  logger,
		devMode: devMode,
	}
}

// HandleDevLogin handles the dev-mode login shortcut.
// In dev mode, it sets a session cookie with a dev JWT and redirects to /dashboard.
// In prod mode, it returns 404.
func (h *AuthHandler) HandleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	token := buildDevJWT()

	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// HandleLogout clears the session cookie and redirects to the landing page.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "vire_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// buildDevJWT creates a minimal unsigned JWT for dev mode.
func buildDevJWT() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))

	claims := map[string]interface{}{
		"sub":   "dev_user",
		"email": "bobmcallan@gmail.com",
		"iss":   "vire-dev",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	claimsJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(claimsJSON)

	return fmt.Sprintf("%s.%s.", header, payload)
}
