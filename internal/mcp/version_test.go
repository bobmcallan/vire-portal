package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestVersionToolHandler_Combined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/version" {
			json.NewEncoder(w).Encode(map[string]string{
				"version":    "0.3.23",
				"build":      "20260216",
				"git_commit": "abc1234",
			})
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	proxy := NewMCPProxy(srv.URL, testLogger(), testConfig())
	handler := VersionToolHandler(proxy)

	result, err := handler(t.Context(), mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}

	var combined map[string]versionInfo
	text := result.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(text), &combined); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := combined["vire_portal"]; !ok {
		t.Error("missing vire_portal in response")
	}

	if srvInfo, ok := combined["vire_server"]; !ok {
		t.Error("missing vire_server in response")
	} else {
		if srvInfo.Version != "0.3.23" {
			t.Errorf("expected server version 0.3.23, got %s", srvInfo.Version)
		}
		if srvInfo.Commit != "abc1234" {
			t.Errorf("expected server commit abc1234, got %s", srvInfo.Commit)
		}
	}
}

func TestVersionToolHandler_ServerUnreachable(t *testing.T) {
	proxy := NewMCPProxy("http://127.0.0.1:1", testLogger(), testConfig())
	handler := VersionToolHandler(proxy)

	result, err := handler(t.Context(), mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatal("should not be an error result when server is unreachable")
	}

	var combined map[string]versionInfo
	text := result.Content[0].(mcpgo.TextContent).Text
	if err := json.Unmarshal([]byte(text), &combined); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := combined["vire_portal"]; !ok {
		t.Error("missing vire_portal in response")
	}
	if _, ok := combined["vire_server"]; ok {
		t.Error("vire_server should be omitted when server is unreachable")
	}
}
