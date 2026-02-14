package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// RegisterToolsFromCatalog registers MCP tools dynamically from catalog entries.
func RegisterToolsFromCatalog(s *server.MCPServer, p *MCPProxy, catalog []CatalogTool) int {
	for _, ct := range catalog {
		tool := BuildMCPTool(ct)
		handler := GenericToolHandler(p, ct)
		s.AddTool(tool, handler)
	}
	return len(catalog)
}
