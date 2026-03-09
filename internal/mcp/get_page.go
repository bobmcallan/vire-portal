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
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// staticPages maps page names to their URL paths.
var staticPages = map[string]string{
	"dashboard":     "/dashboard",
	"strategy":      "/strategy",
	"cash":          "/cash",
	"glossary":      "/glossary",
	"changelog":     "/changelog",
	"help":          "/help",
	"mcp-info":      "/mcp-info",
	"profile":       "/profile",
	"docs":          "/docs",
	"admin/users":   "/admin/users",
	"admin/prompts": "/admin/prompts",
	"admin/gemini":  "/admin/gemini",
}

// tickerPattern validates ticker symbols (e.g. "EHL", "CBOE", "CME.US", "DFND.AU").
var tickerPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,9}(\.[A-Za-z]{1,4})?$`)

// maxPageResponseSize limits loopback response body to 5MB.
const maxPageResponseSize = 5 << 20

// loopbackTimeout is the HTTP client timeout for loopback requests.
const loopbackTimeout = 15 * time.Second

// loopbackJWTTTL is the TTL for short-lived loopback JWTs.
const loopbackJWTTTL = 30 * time.Second

// resolvePage converts a page parameter to a URL path.
// Returns the path and true if valid, or empty string and false if invalid.
func resolvePage(page string) (string, bool) {
	// Check static pages first
	if path, ok := staticPages[page]; ok {
		return path, true
	}

	// Check dynamic patterns: stock/{ticker}
	if strings.HasPrefix(page, "stock/") {
		ticker := strings.TrimPrefix(page, "stock/")
		if ticker != "" && tickerPattern.MatchString(ticker) {
			return "/stock/" + ticker, true
		}
	}

	return "", false
}

// pageList returns the list of valid page names for error messages.
func pageList() string {
	pages := make([]string, 0, len(staticPages)+1)
	for k := range staticPages {
		pages = append(pages, k)
	}
	pages = append(pages, "stock/{ticker}")
	return strings.Join(pages, ", ")
}

// GetPageTool returns the mcp.Tool definition for portal_get_page.
func GetPageTool() mcp.Tool {
	return mcp.NewTool("portal_get_page",
		mcp.WithDescription("Fetch a rendered portal page as semantic text content. Returns the main content area of the page with HTML stripped — suitable for LLM review of layout, content accuracy, and data rendering. Supports static pages and dynamic routes like stock/{ticker}."),
		mcp.WithString("page",
			mcp.Description("Page to fetch. Static pages: dashboard, strategy, cash, glossary, changelog, help, mcp-info, profile, docs, admin/users, admin/prompts, admin/gemini. Dynamic: stock/{ticker} (e.g. stock/EHL, stock/CBOE, stock/CME.US)"),
			mcp.Required(),
		),
	)
}

// GetPageToolHandler returns a handler that fetches a rendered portal page via loopback HTTP.
func GetPageToolHandler(portalBaseURL string, jwtSecret []byte) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := r.GetString("page", "")
		path, ok := resolvePage(page)
		if !ok {
			return errorResult(fmt.Sprintf("Error: %q is not a valid page. Choose one of: %s", page, pageList())), nil
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

		text := extractMainContent(string(body))

		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(text)},
		}, nil
	}
}

// extractMainContent extracts the text content from the <main> element,
// stripping scripts, styles, SVGs, and HTML tags. Returns clean text
// suitable for LLM consumption.
func extractMainContent(html string) string {
	// Extract <main> content
	mainStart := strings.Index(html, "<main")
	if mainStart == -1 {
		return stripHTML(html)
	}
	// Find the end of the opening <main> tag
	tagEnd := strings.Index(html[mainStart:], ">")
	if tagEnd == -1 {
		return stripHTML(html)
	}
	content := html[mainStart+tagEnd+1:]

	mainEnd := strings.Index(content, "</main>")
	if mainEnd != -1 {
		content = content[:mainEnd]
	}

	// Remove script blocks
	content = removeBlocks(content, "<script", "</script>")
	// Remove style blocks
	content = removeBlocks(content, "<style", "</style>")
	// Remove SVG blocks
	content = removeBlocks(content, "<svg", "</svg>")
	// Remove HTML comments
	content = removeBlocks(content, "<!--", "-->")

	// Strip remaining HTML tags
	content = stripHTML(content)

	// Decode common HTML entities
	content = strings.ReplaceAll(content, "&amp;", "&")
	content = strings.ReplaceAll(content, "&lt;", "<")
	content = strings.ReplaceAll(content, "&gt;", ">")
	content = strings.ReplaceAll(content, "&quot;", `"`)
	content = strings.ReplaceAll(content, "&#39;", "'")
	content = strings.ReplaceAll(content, "&nbsp;", " ")

	// Collapse whitespace: multiple spaces/tabs to single space
	content = collapseWhitespace(content)

	return strings.TrimSpace(content)
}

// removeBlocks removes all occurrences of content between startTag and endTag (inclusive).
func removeBlocks(s, startTag, endTag string) string {
	for {
		start := strings.Index(strings.ToLower(s), strings.ToLower(startTag))
		if start == -1 {
			return s
		}
		end := strings.Index(strings.ToLower(s[start:]), strings.ToLower(endTag))
		if end == -1 {
			return s[:start]
		}
		s = s[:start] + s[start+end+len(endTag):]
	}
}

// stripHTML removes all HTML tags from the string.
func stripHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			b.WriteRune(' ') // replace tag with space to preserve word boundaries
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// collapseWhitespace normalizes whitespace: collapses runs of spaces/tabs
// into single spaces and limits consecutive newlines to 2.
func collapseWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s) / 2)
	lines := strings.Split(s, "\n")
	blankCount := 0
	for _, line := range lines {
		trimmed := collapseSpaces(strings.TrimSpace(line))
		if trimmed == "" {
			blankCount++
			if blankCount <= 1 {
				b.WriteRune('\n')
			}
			continue
		}
		blankCount = 0
		b.WriteString(trimmed)
		b.WriteRune('\n')
	}
	return b.String()
}

// collapseSpaces reduces runs of spaces/tabs to a single space.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
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
