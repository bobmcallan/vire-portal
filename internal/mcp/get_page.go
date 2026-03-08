package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// allowedPages maps page names to their URL paths.
var allowedPages = map[string]string{
	"dashboard": "/dashboard",
	"strategy":  "/strategy",
	"cash":      "/cash",
	"glossary":  "/glossary",
	"changelog": "/changelog",
	"help":      "/help",
	"mcp-info":  "/mcp-info",
	"profile":   "/profile",
	"docs":      "/docs",
}

// maxPageResponseSize limits loopback response body to 5MB.
const maxPageResponseSize = 5 << 20

// loopbackTimeout is the HTTP client timeout for loopback requests.
const loopbackTimeout = 15 * time.Second

// loopbackJWTTTL is the TTL for short-lived loopback JWTs.
const loopbackJWTTTL = 30 * time.Second

// GetPageTool returns the mcp.Tool definition for portal_get_page.
func GetPageTool() mcp.Tool {
	return mcp.NewTool("portal_get_page",
		mcp.WithDescription("Fetch a rendered portal page as HTML. Returns the full HTML of the requested page as seen by the authenticated user. Use this to review page layout, content accuracy, and data rendering without requiring screenshots."),
		mcp.WithString("page",
			mcp.Description("Page to fetch. One of: dashboard, strategy, cash, glossary, changelog, help, mcp-info, profile, docs"),
			mcp.Required(),
		),
	)
}

// GetPageToolHandler returns a handler that fetches a rendered portal page via loopback HTTP.
func GetPageToolHandler(portalBaseURL string, jwtSecret []byte) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := r.GetString("page", "")
		path, ok := allowedPages[page]
		if !ok {
			return errorResult(fmt.Sprintf("Error: %q is not a valid page. Choose one of: dashboard, strategy, cash, glossary, changelog, help, mcp-info, profile, docs", page)), nil
		}

		uc, ok := GetUserContext(ctx)
		if !ok {
			return errorResult("Error: authentication required"), nil
		}

		token, err := mintLoopbackJWT(uc.UserID, jwtSecret)
		if err != nil {
			return errorResult(fmt.Sprintf("Error: failed to mint loopback token: %v", err)), nil
		}

		req, err := http.NewRequestWithContext(ctx, "GET", portalBaseURL+path, nil)
		if err != nil {
			return errorResult(fmt.Sprintf("Error: failed to build request: %v", err)), nil
		}
		req.AddCookie(&http.Cookie{Name: "vire_session", Value: token})

		client := &http.Client{Timeout: loopbackTimeout}
		resp, err := client.Do(req)
		if err != nil {
			return errorResult(fmt.Sprintf("Error: portal request failed: %v", err)), nil
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxPageResponseSize))
		if err != nil {
			return errorResult(fmt.Sprintf("Error: failed to read response: %v", err)), nil
		}

		if resp.StatusCode != 200 {
			return errorResult(fmt.Sprintf("Error: portal returned status %d: %s", resp.StatusCode, string(body))), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(body))},
		}, nil
	}
}

// mintLoopbackJWT creates a short-lived JWT for loopback portal requests.
func mintLoopbackJWT(userID string, secret []byte) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

	now := time.Now().Unix()
	payload := struct {
		Sub string `json:"sub"`
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}{
		Sub: userID,
		Iss: "vire-portal-loopback",
		Iat: now,
		Exp: now + int64(loopbackJWTTTL.Seconds()),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	sigInput := header + "." + encodedPayload
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sigInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return sigInput + "." + signature, nil
}
