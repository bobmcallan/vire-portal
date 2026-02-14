package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// maxCatalogSize is the maximum allowed size for a catalog response (1MB).
const maxCatalogSize = 1 << 20

// allowedMethods is the whitelist of HTTP methods for catalog tools.
var allowedMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// CatalogTool represents one tool entry from GET /api/mcp/tools.
type CatalogTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Params      []CatalogParam `json:"params"`
}

// CatalogParam describes one parameter for a catalog tool.
type CatalogParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // string, number, boolean, array, object
	Description string `json:"description"`
	Required    bool   `json:"required"`
	In          string `json:"in"`           // path, query, body
	DefaultFrom string `json:"default_from"` // e.g. "user_config.default_portfolio"
}

// FetchCatalog fetches the tool catalog from vire-server.
// Returns nil, nil if the server is unreachable (non-fatal at startup).
func (p *MCPProxy) FetchCatalog(ctx context.Context) ([]CatalogTool, error) {
	body, err := p.get(ctx, "/api/mcp/tools")
	if err != nil {
		return nil, err
	}
	if len(body) > maxCatalogSize {
		return nil, fmt.Errorf("catalog response too large: %d bytes (max %d)", len(body), maxCatalogSize)
	}
	var tools []CatalogTool
	if err := json.Unmarshal(body, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse tool catalog: %w", err)
	}
	return tools, nil
}

// ValidateCatalogTool validates a single catalog tool entry.
func ValidateCatalogTool(ct CatalogTool) error {
	if ct.Name == "" {
		return fmt.Errorf("tool has empty name")
	}
	if ct.Method == "" {
		return fmt.Errorf("tool %q has empty method", ct.Name)
	}
	if !allowedMethods[strings.ToUpper(ct.Method)] {
		return fmt.Errorf("tool %q has unsupported method %q", ct.Name, ct.Method)
	}
	if ct.Path == "" {
		return fmt.Errorf("tool %q has empty path", ct.Name)
	}
	if !strings.HasPrefix(ct.Path, "/api/") {
		return fmt.Errorf("tool %q has invalid path %q (must start with /api/)", ct.Name, ct.Path)
	}
	if strings.Contains(ct.Path, "..") {
		return fmt.Errorf("tool %q has invalid path %q (contains ..)", ct.Name, ct.Path)
	}
	return nil
}

// ValidateCatalog filters and validates catalog entries, logging warnings for invalid or duplicate tools.
func ValidateCatalog(catalog []CatalogTool, logger *common.Logger) []CatalogTool {
	seen := make(map[string]bool, len(catalog))
	valid := make([]CatalogTool, 0, len(catalog))
	for _, ct := range catalog {
		if err := ValidateCatalogTool(ct); err != nil {
			logger.Warn().Str("error", err.Error()).Msg("skipping invalid catalog tool")
			continue
		}
		if seen[ct.Name] {
			logger.Warn().Str("name", ct.Name).Msg("skipping duplicate catalog tool")
			continue
		}
		seen[ct.Name] = true
		valid = append(valid, ct)
	}
	return valid
}

// BuildMCPTool converts a CatalogTool into an mcp.Tool with the appropriate schema.
func BuildMCPTool(ct CatalogTool) mcp.Tool {
	opts := []mcp.ToolOption{mcp.WithDescription(ct.Description)}
	for _, p := range ct.Params {
		if p.In == "path" || p.In == "query" || p.In == "body" {
			opt := buildParamOption(p)
			opts = append(opts, opt)
		}
	}
	return mcp.NewTool(ct.Name, opts...)
}

// buildParamOption maps a CatalogParam to the appropriate mcp-go tool option.
func buildParamOption(p CatalogParam) mcp.ToolOption {
	var opts []mcp.PropertyOption
	if p.Description != "" {
		opts = append(opts, mcp.Description(p.Description))
	}
	if p.Required {
		opts = append(opts, mcp.Required())
	}

	switch p.Type {
	case "number":
		return mcp.WithNumber(p.Name, opts...)
	case "boolean":
		return mcp.WithBoolean(p.Name, opts...)
	case "array":
		opts = append([]mcp.PropertyOption{mcp.WithStringItems()}, opts...)
		return mcp.WithArray(p.Name, opts...)
	default:
		// string, object, or unknown — all passed as string
		return mcp.WithString(p.Name, opts...)
	}
}

// GenericToolHandler creates a handler that routes an MCP tool call to
// the appropriate vire-server REST endpoint based on a CatalogTool definition.
func GenericToolHandler(p *MCPProxy, ct CatalogTool) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Resolve path, query, and body params
		path := ct.Path
		bodyParams := map[string]interface{}{}
		queryParams := url.Values{}

		for _, param := range ct.Params {
			val := resolveParamValue(ctx, p, r, param)
			switch param.In {
			case "path":
				strVal := fmt.Sprint(val)
				if val == nil || strVal == "" {
					if param.Required {
						return errorResult(fmt.Sprintf("Error: %s parameter is required", param.Name)), nil
					}
					continue
				}
				path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(strVal))
			case "query":
				if val != nil {
					strVal := fmt.Sprint(val)
					if strVal != "" {
						queryParams.Set(param.Name, strVal)
					}
				}
			case "body":
				if val != nil {
					bodyParams[param.Name] = val
				}
			}
		}

		if len(queryParams) > 0 {
			path += "?" + queryParams.Encode()
		}

		// Execute HTTP request based on method
		var respBody []byte
		var err error
		switch strings.ToUpper(ct.Method) {
		case "GET":
			respBody, err = p.get(ctx, path)
		case "POST":
			respBody, err = p.post(ctx, path, bodyOrNil(bodyParams))
		case "PUT":
			respBody, err = p.put(ctx, path, bodyOrNil(bodyParams))
		case "PATCH":
			respBody, err = p.patch(ctx, path, bodyOrNil(bodyParams))
		case "DELETE":
			respBody, err = p.del(ctx, path)
		default:
			return errorResult(fmt.Sprintf("Error: unsupported method %s", ct.Method)), nil
		}

		if err != nil {
			return errorResult(fmt.Sprintf("Error: %v", err)), nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(string(respBody))}}, nil
	}
}

// resolveParamValue extracts a parameter value from the MCP request,
// falling back to defaults from config when default_from is set.
func resolveParamValue(ctx context.Context, p *MCPProxy, r mcp.CallToolRequest, param CatalogParam) interface{} {
	// Try to get value from request arguments
	switch param.Type {
	case "number", "boolean", "array":
		if args := r.GetArguments(); args != nil {
			if v, ok := args[param.Name]; ok {
				return v
			}
		}
	default:
		// string or object
		val := r.GetString(param.Name, "")
		if val != "" {
			return val
		}
	}

	// If no value and default_from is set, resolve from config
	if param.DefaultFrom != "" {
		return resolveDefaultValue(ctx, p, param.DefaultFrom)
	}

	return nil
}

// resolveDefaultValue resolves a default value from the portal config.
func resolveDefaultValue(ctx context.Context, p *MCPProxy, defaultFrom string) interface{} {
	switch defaultFrom {
	case "user_config.default_portfolio":
		return resolveDefaultPortfolio(ctx, p)
	default:
		return nil
	}
}

// resolveDefaultPortfolio resolves the default portfolio using a 3-tier strategy:
// 1. First portfolio from X-Vire-Portfolios header (config)
// 2. API fallback: GET /api/portfolios/default from vire-server
// Returns empty string if no default can be resolved.
func resolveDefaultPortfolio(ctx context.Context, p *MCPProxy) string {
	// Tier 1: Check config headers
	headers := p.UserHeaders()
	portfolios := headers.Get("X-Vire-Portfolios")
	if portfolios != "" {
		for i := 0; i < len(portfolios); i++ {
			if portfolios[i] == ',' {
				return portfolios[:i]
			}
		}
		return portfolios
	}

	// Tier 2: API fallback
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

// bodyOrNil returns nil if the body map is empty, otherwise returns the map.
// This prevents sending an empty JSON object for methods that don't need a body.
func bodyOrNil(body map[string]interface{}) interface{} {
	if len(body) == 0 {
		return nil
	}
	return body
}
