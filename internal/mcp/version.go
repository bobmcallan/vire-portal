package mcp

import (
	"context"
	"encoding/json"

	common "github.com/bobmcallan/vire-portal/internal/vire/common"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// versionInfo holds version fields for one component.
type versionInfo struct {
	Version string `json:"version"`
	Build   string `json:"build"`
	Commit  string `json:"commit"`
}

// VersionTool returns the mcp.Tool definition for the combined get_version tool.
func VersionTool() mcp.Tool {
	return mcp.NewTool("get_version",
		mcp.WithDescription("Get Vire MCP server version and status. Use this to verify connectivity."),
	)
}

// VersionToolHandler returns a handler that combines vire-portal and vire-server version info.
func VersionToolHandler(proxy *MCPProxy) server.ToolHandlerFunc {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result := map[string]versionInfo{}

		result["vire_portal"] = versionInfo{
			Version: common.GetVersion(),
			Build:   common.GetBuild(),
			Commit:  common.GetGitCommit(),
		}

		// Fetch vire-server version (graceful degradation if unreachable).
		body, err := proxy.get(ctx, "/api/version")
		if err == nil {
			var serverResp map[string]string
			if json.Unmarshal(body, &serverResp) == nil {
				result["vire_server"] = versionInfo{
					Version: serverResp["version"],
					Build:   serverResp["build"],
					Commit:  serverResp["git_commit"],
				}
			}
		}

		out, err := json.Marshal(result)
		if err != nil {
			return errorResult("failed to marshal version info"), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(out))},
		}, nil
	}
}
