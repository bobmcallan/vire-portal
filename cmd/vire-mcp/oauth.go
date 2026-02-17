package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client/transport"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// callbackResult carries the OAuth callback parameters.
type callbackResult struct {
	code  string
	state string
	err   error
}

// doOAuthFlow runs the OAuth 2.1 authorization code flow with PKCE.
// It discovers server metadata, registers the client via DCR if needed,
// opens the user's browser for authorization, waits for the callback,
// and exchanges the code for tokens.
func doOAuthFlow(handler *transport.OAuthHandler, callbackPort int, logger *common.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Discover authorization server metadata.
	if _, err := handler.GetServerMetadata(ctx); err != nil {
		return fmt.Errorf("metadata discovery: %w", err)
	}

	// Register client via DCR if no client_id is cached.
	if handler.GetClientID() == "" {
		if err := handler.RegisterClient(ctx, "vire-mcp"); err != nil {
			return fmt.Errorf("client registration: %w", err)
		}
	}

	// Generate PKCE code verifier and challenge.
	codeVerifier, err := transport.GenerateCodeVerifier()
	if err != nil {
		return fmt.Errorf("PKCE verifier: %w", err)
	}
	codeChallenge := transport.GenerateCodeChallenge(codeVerifier)

	// Generate state for CSRF protection.
	state, err := transport.GenerateState()
	if err != nil {
		return fmt.Errorf("state generation: %w", err)
	}
	handler.SetExpectedState(state)

	// Build authorization URL (used to extract base URL and params).
	authURL, err := handler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("authorization URL: %w", err)
	}

	// Start local callback server.
	codeCh := make(chan callbackResult, 1)
	srv := startCallbackServer(callbackPort, codeCh)
	defer srv.Close()

	// Create auth session via POST so the server has all OAuth params.
	// The browser then opens GET /authorize?client_id=xxx which finds the
	// pending session — this avoids '&' mangling in browser URLs on Windows/WSL.
	browserURL := authURL
	if err := prepareAuthSession(ctx, authURL, logger); err != nil {
		logger.Warn().Str("error", err.Error()).Msg("POST /authorize failed, opening full URL")
	} else {
		// Open just the authorize URL with client_id (no '&' to mangle).
		parsed, _ := url.Parse(authURL)
		browserURL = fmt.Sprintf("%s://%s/authorize?client_id=%s",
			parsed.Scheme, parsed.Host, url.QueryEscape(parsed.Query().Get("client_id")))
	}

	logger.Info().Str("url", browserURL).Msg("opening browser for authorization")
	if err := openBrowser(browserURL); err != nil {
		logger.Warn().Str("error", err.Error()).Msg("failed to open browser automatically")
		fmt.Fprintf(os.Stderr, "\nOpen this URL in your browser:\n%s\n\n", browserURL)
	}

	// Wait for callback or timeout.
	select {
	case result := <-codeCh:
		if result.err != nil {
			return fmt.Errorf("callback: %w", result.err)
		}
		if err := handler.ProcessAuthorizationResponse(ctx, result.code, result.state, codeVerifier); err != nil {
			return fmt.Errorf("token exchange: %w", err)
		}
	case <-ctx.Done():
		return fmt.Errorf("authorization timed out")
	}

	logger.Info().Msg("OAuth authorization complete")
	return nil
}

// prepareAuthSession POSTs the OAuth parameters to the portal's /authorize
// endpoint, creating a server-side session. The browser can then open
// GET /authorize?client_id=xxx (a simple URL with no '&') and the portal
// will find the pending session by client_id.
func prepareAuthSession(ctx context.Context, authURL string, logger *common.Logger) error {
	parsed, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("parse authorize URL: %w", err)
	}

	baseURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	// POST the query params as form data.
	resp, err := http.PostForm(baseURL+"/authorize", parsed.Query())
	if err != nil {
		return fmt.Errorf("POST /authorize: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST /authorize returned %d", resp.StatusCode)
	}

	return nil
}

// startCallbackServer starts an HTTP server on the given port to receive
// the OAuth authorization callback. It sends the result on ch and returns
// a "you can close this tab" page to the browser.
func startCallbackServer(port int, ch chan<- callbackResult) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errCode := q.Get("error"); errCode != "" {
			desc := q.Get("error_description")
			ch <- callbackResult{err: fmt.Errorf("%s: %s", errCode, desc)}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html><body><h1>Authorization Failed</h1><p>%s</p>`+
				`<p>You can close this tab.</p></body></html>`, desc)
			return
		}

		ch <- callbackResult{code: q.Get("code"), state: q.Get("state")}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><h1>Authorization Successful</h1>`+
			`<p>You can close this tab and return to the terminal.</p></body></html>`)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}
	go srv.ListenAndServe() //nolint:errcheck // server runs until Close()
	return srv
}

// openBrowser opens the given URL in the user's default browser.
// Inside WSL it shells out to cmd.exe so the Windows browser opens.
// Inside Docker containers it returns an error (no browser available).
func openBrowser(url string) error {
	if isDocker() {
		return fmt.Errorf("running inside Docker container, no browser available")
	}

	switch runtime.GOOS {
	case "linux":
		if isWSL() {
			return exec.Command("cmd.exe", "/c", "start", url).Start()
		}
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// isDocker reports whether the process is running inside a Docker container.
func isDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// isWSL reports whether the process is running inside Windows Subsystem for Linux.
func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}
