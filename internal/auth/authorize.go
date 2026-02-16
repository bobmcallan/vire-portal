package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"
)

const defaultScope = "openid portfolio:read tools:invoke"

// HandleAuthorize handles GET /authorize — starts the MCP OAuth flow.
// The full authorize URL contains multiple query parameters (&-separated) which
// can be mangled by Windows/WSL process invocation. To support programmatic
// clients (like vire-mcp), POST /authorize accepts the same params as form data
// and returns JSON with a session URL the client can open in a browser.
func (s *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAuthorizeGET(w, r)
	case http.MethodPost:
		s.handleAuthorizePOST(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleAuthorizeGET handles the standard browser-based authorize flow.
// When all params are present, it creates a new session. When only client_id
// is present (URL truncated by Windows/WSL), it looks for a pending session
// that was already created via POST /authorize.
func (s *OAuthServer) handleAuthorizeGET(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")

	// If redirect_uri is missing, check for a pending session created via POST.
	// This handles Windows/WSL where '&' in URLs gets stripped, leaving only
	// GET /authorize?client_id=xxx.
	if redirectURI == "" {
		if clientID != "" {
			if sess := s.sessions.GetByClientID(clientID); sess != nil {
				s.setSessionCookieAndRedirect(w, r, sess.SessionID)
				return
			}
		}
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}

	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Host == "" {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

	responseType := q.Get("response_type")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	state := q.Get("state")
	scope := q.Get("scope")

	// Validate required params
	if clientID == "" || responseType == "" || codeChallenge == "" || codeChallengeMethod == "" || state == "" {
		redirectWithError(w, r, redirectURI, "invalid_request", "missing required parameters", state)
		return
	}

	if responseType != "code" {
		redirectWithError(w, r, redirectURI, "unsupported_response_type", "only code is supported", state)
		return
	}

	if codeChallengeMethod != "S256" {
		redirectWithError(w, r, redirectURI, "invalid_request", "only S256 code_challenge_method is supported", state)
		return
	}

	sessionID, err := s.createAuthSession(clientID, redirectURI, responseType, codeChallenge, codeChallengeMethod, state, scope)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.setSessionCookieAndRedirect(w, r, sessionID)
}

// handleAuthorizePOST accepts OAuth params as form data and returns JSON with
// a session URL. This lets programmatic clients (vire-mcp) send the full
// parameter set via HTTP, then open only a simple URL in the browser.
func (s *OAuthServer) handleAuthorizePOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	responseType := r.FormValue("response_type")
	codeChallenge := r.FormValue("code_challenge")
	codeChallengeMethod := r.FormValue("code_challenge_method")
	state := r.FormValue("state")
	scope := r.FormValue("scope")

	// Validate redirect_uri
	if redirectURI == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing redirect_uri"})
		return
	}
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Host == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid redirect_uri"})
		return
	}

	if clientID == "" || responseType == "" || codeChallenge == "" || codeChallengeMethod == "" || state == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required parameters"})
		return
	}
	if responseType != "code" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported response_type"})
		return
	}
	if codeChallengeMethod != "S256" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "only S256 code_challenge_method supported"})
		return
	}

	sessionID, err := s.createAuthSession(clientID, redirectURI, responseType, codeChallenge, codeChallengeMethod, state, scope)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"session_id": sessionID})
}

// HandleAuthorizeResume handles GET /authorize/resume?s={sessionID}.
// It sets the mcp_session_id cookie and redirects to the landing page.
// This endpoint exists so programmatic clients can open a simple URL in the
// browser after creating a session via POST /authorize.
func (s *OAuthServer) HandleAuthorizeResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("s")
	if sessionID == "" {
		http.Error(w, "missing session parameter", http.StatusBadRequest)
		return
	}

	// Verify the session exists.
	if _, ok := s.sessions.Get(sessionID); !ok {
		http.Error(w, "invalid or expired session", http.StatusBadRequest)
		return
	}

	s.setSessionCookieAndRedirect(w, r, sessionID)
}

// createAuthSession validates the client and creates an auth session.
func (s *OAuthServer) createAuthSession(clientID, redirectURI, responseType, codeChallenge, codeChallengeMethod, state, scope string) (string, error) {
	// Look up or auto-register client
	client, ok := s.clients.Get(clientID)
	if !ok {
		client = &OAuthClient{
			ClientID:                clientID,
			ClientName:              "auto-registered",
			RedirectURIs:            []string{redirectURI},
			GrantTypes:              []string{"authorization_code", "refresh_token"},
			ResponseTypes:           []string{"code"},
			TokenEndpointAuthMethod: "none",
			CreatedAt:               time.Now(),
		}
		s.clients.Put(client)
	}

	if !containsString(client.RedirectURIs, redirectURI) {
		return "", errors.New("redirect_uri does not match registered URI")
	}

	if scope == "" {
		scope = defaultScope
	}

	sessionID, err := generateRandomHex(16)
	if err != nil {
		return "", err
	}

	s.sessions.Put(&AuthSession{
		SessionID:     sessionID,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		State:         state,
		CodeChallenge: codeChallenge,
		CodeMethod:    codeChallengeMethod,
		Scope:         scope,
		CreatedAt:     time.Now(),
	})

	return sessionID, nil
}

// setSessionCookieAndRedirect sets the mcp_session_id cookie and redirects
// to the landing page.
func (s *OAuthServer) setSessionCookieAndRedirect(w http.ResponseWriter, r *http.Request, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mcp_session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, "/?mcp_session="+sessionID, http.StatusFound)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// redirectWithError redirects to the redirect_uri with error parameters.
func redirectWithError(w http.ResponseWriter, r *http.Request, redirectURI, errorCode, description, state string) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}
	q := u.Query()
	q.Set("error", errorCode)
	q.Set("error_description", description)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
