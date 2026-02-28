package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// validServiceKey is a 32+ char key used in tests where the server has VIRE_SERVICE_KEY configured.
const validServiceKey = "test-service-key-minimum-32-chars-ok"

// ServerEnv provides a SurrealDB + vire-server environment for API integration tests.
// Unlike the UI test environment, no portal container is started — the portal's Go code
// is exercised directly in the test process.
type ServerEnv struct {
	t           *testing.T
	server      testcontainers.Container
	surrealDB   testcontainers.Container
	testNetwork *testcontainers.DockerNetwork
	ctx         context.Context
	cancel      context.CancelFunc
	serverURL   string
	resultsDir  string
}

// ServerEnvOptions configures the test environment.
type ServerEnvOptions struct {
	ExtraEnv map[string]string
}

var (
	envBuildOnce sync.Once
	envBuildErr  error
)

// NewServerEnv creates a 2-container environment with default options.
func NewServerEnv(t *testing.T) *ServerEnv {
	return NewServerEnvWithOptions(t, ServerEnvOptions{})
}

// NewServerEnvWithOptions creates a 2-container environment (SurrealDB + vire-server).
func NewServerEnvWithOptions(t *testing.T, opts ServerEnvOptions) *ServerEnv {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	// Create results directory
	datetime := time.Now().Format("20060102-150405")
	resultsDir := filepath.Join(findProjectRoot(), "tests", "logs", datetime+"-"+t.Name())
	os.MkdirAll(resultsDir, 0755)

	// Create Docker network
	testNet, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		cancel()
		t.Fatalf("failed to create docker network: %v", err)
	}

	// Start SurrealDB
	surrealContainer, err := testcontainers.Run(ctx, "surrealdb/surrealdb:v3.0.0",
		testcontainers.WithExposedPorts("8000/tcp"),
		testcontainers.WithCmd("start", "--user", "root", "--pass", "root"),
		network.WithNetwork([]string{"surrealdb-api-tc"}, testNet),
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForListeningPort("8000/tcp"),
				wait.ForLog("Started web server"),
			).WithDeadline(60*time.Second),
		),
	)
	if err != nil {
		testNet.Remove(ctx)
		cancel()
		t.Fatalf("failed to start SurrealDB: %v", err)
	}

	surrealIP, err := surrealContainer.ContainerIP(ctx)
	if err != nil {
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		t.Fatalf("failed to get SurrealDB IP: %v", err)
	}

	// Build vire-server env vars
	serverEnv := map[string]string{
		"VIRE_STORAGE_ADDRESS":   fmt.Sprintf("ws://%s:8000/rpc", surrealIP),
		"VIRE_STORAGE_NAMESPACE": "vire",
		"VIRE_STORAGE_DATABASE":  "vire_test",
		"VIRE_STORAGE_USERNAME":  "root",
		"VIRE_STORAGE_PASSWORD":  "root",
		"VIRE_ENV":               "dev",
		"VIRE_SERVER_HOST":       "0.0.0.0",
		"VIRE_AUTH_JWT_SECRET":   "test-jwt-secret-for-ci",
		"EODHD_API_KEY":          "test-dummy-key",
		"GEMINI_API_KEY":         "test-dummy-key",
	}
	for k, v := range opts.ExtraEnv {
		serverEnv[k] = v
	}

	// Start vire-server from GHCR
	serverContainer, err := testcontainers.Run(ctx, "ghcr.io/bobmcallan/vire-server:latest",
		testcontainers.WithExposedPorts("8080/tcp"),
		network.WithNetwork([]string{"vire-server-api-tc"}, testNet),
		testcontainers.WithEnv(serverEnv),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/api/health").WithPort("8080/tcp").WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		t.Fatalf("failed to start vire-server: %v", err)
	}

	mappedPort, err := serverContainer.MappedPort(ctx, "8080/tcp")
	if err != nil {
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		t.Fatalf("failed to get mapped port: %v", err)
	}

	host, err := serverContainer.Host(ctx)
	if err != nil {
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		t.Fatalf("failed to get host: %v", err)
	}

	serverURL := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	t.Logf("Server environment ready: %s", serverURL)

	return &ServerEnv{
		t:           t,
		server:      serverContainer,
		surrealDB:   surrealContainer,
		testNetwork: testNet,
		ctx:         ctx,
		cancel:      cancel,
		serverURL:   serverURL,
		resultsDir:  resultsDir,
	}
}

// ServerURL returns the base URL of the running vire-server.
func (e *ServerEnv) ServerURL() string {
	return e.serverURL
}

// Cleanup tears down all containers and the network.
func (e *ServerEnv) Cleanup() {
	if e == nil {
		return
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()

	// Collect logs before teardown
	e.collectLogs(cleanupCtx)

	if e.server != nil {
		e.server.Terminate(cleanupCtx)
	}
	if e.surrealDB != nil {
		e.surrealDB.Terminate(cleanupCtx)
	}
	if e.testNetwork != nil {
		e.testNetwork.Remove(cleanupCtx)
	}
	if e.cancel != nil {
		e.cancel()
	}
}

// HTTPPost sends a POST request with JSON body to the vire-server.
func (e *ServerEnv) HTTPPost(path string, body interface{}) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return http.Post(e.serverURL+path, "application/json", strings.NewReader(string(bodyBytes)))
}

// HTTPRequest sends a request with custom headers to the vire-server.
func (e *ServerEnv) HTTPRequest(method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}
	req, err := http.NewRequestWithContext(e.ctx, method, e.serverURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

// SaveResult saves test output to the results directory.
func (e *ServerEnv) SaveResult(name string, data []byte) {
	os.WriteFile(filepath.Join(e.resultsDir, name), data, 0644)
}

func (e *ServerEnv) collectLogs(ctx context.Context) {
	if e.server == nil {
		return
	}
	reader, err := e.server.Logs(ctx)
	if err != nil {
		return
	}
	defer reader.Close()
	logs, _ := io.ReadAll(reader)
	os.WriteFile(filepath.Join(e.resultsDir, "vire-server.log"), logs, 0644)
}

// createUser creates a user on vire-server via POST /api/users.
func createUser(t *testing.T, env *ServerEnv, username, email, password string) {
	t.Helper()
	resp, err := env.HTTPPost("/api/users", map[string]interface{}{
		"username": username,
		"email":    email,
		"password": password,
	})
	if err != nil {
		t.Fatalf("failed to create user %s: %v", username, err)
	}
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create user %s returned %d, expected 201", username, resp.StatusCode)
	}
}

// readBody reads and returns the response body.
func readBody(t *testing.T, body io.ReadCloser) []byte {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	return data
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
