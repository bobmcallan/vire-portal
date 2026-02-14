package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

// errorResult creates an MCP error result.
func errorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(message),
		},
		IsError: true,
	}
}

// resolvePortfolio resolves the portfolio name from the request or the server default.
func resolvePortfolio(ctx context.Context, p *MCPProxy, request mcp.CallToolRequest) string {
	name := request.GetString("portfolio_name", "")
	if name != "" {
		return name
	}

	// Check if config has a default portfolio
	headers := p.UserHeaders()
	if portfolios := headers.Get("X-Vire-Portfolios"); portfolios != "" {
		// Use the first configured portfolio as default
		for i := 0; i < len(portfolios); i++ {
			if portfolios[i] == ',' {
				return portfolios[:i]
			}
		}
		return portfolios
	}

	// Ask the server for the default
	body, err := p.get(ctx, "/api/portfolios/default")
	if err != nil {
		return ""
	}
	var resp struct {
		Default string `json:"default"`
	}
	if json.Unmarshal(body, &resp) == nil && resp.Default != "" {
		return resp.Default
	}
	return ""
}
