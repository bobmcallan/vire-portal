package auth

import (
	"net/http"
	"net/url"
	"time"
)

const defaultScope = "openid portfolio:read tools:invoke"

// HandleAuthorize handles GET /authorize — starts the MCP OAuth flow.
func (s *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	codeChallenge := q.Get("code_challenge")
	codeChallengeMethod := q.Get("code_challenge_method")
	state := q.Get("state")
	scope := q.Get("scope")

	// Validate redirect_uri is a valid URL before doing anything else
	if redirectURI == "" {
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}
	parsedRedirect, err := url.Parse(redirectURI)
	if err != nil || parsedRedirect.Host == "" {
		http.Error(w, "invalid redirect_uri", http.StatusBadRequest)
		return
	}

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

	// Look up or auto-register client
	client, ok := s.clients.Get(clientID)
	if !ok {
		// Auto-register for lenient mode (Claude Desktop)
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

	// Validate redirect_uri matches registered one.
	// Per OAuth spec, if redirect_uri is invalid, return an error page (don't redirect to unvalidated URI).
	if !containsString(client.RedirectURIs, redirectURI) {
		http.Error(w, "redirect_uri does not match registered URI", http.StatusBadRequest)
		return
	}

	if scope == "" {
		scope = defaultScope
	}

	sessionID, err := generateRandomHex(16)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sess := &AuthSession{
		SessionID:     sessionID,
		ClientID:      clientID,
		RedirectURI:   redirectURI,
		State:         state,
		CodeChallenge: codeChallenge,
		CodeMethod:    codeChallengeMethod,
		Scope:         scope,
		CreatedAt:     time.Now(),
	}
	s.sessions.Put(sess)

	// Set mcp_session_id cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "mcp_session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	// Redirect to landing page with mcp_session param
	http.Redirect(w, r, "/?mcp_session="+sessionID, http.StatusFound)
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
