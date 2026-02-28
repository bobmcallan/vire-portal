package common

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testContainerNames lists all container names created by the test environment.
// Only these containers are cleaned up — dev/production containers are never touched.
var testContainerNames = []string{"vire-db-tc", "vire-server-tc", "vire-portal-tc"}

// removeStaleTestContainers removes any leftover test containers from previous runs.
// This prevents "container name already in use" errors without manual intervention.
// Only removes containers matching testContainerNames — never touches other containers.
func removeStaleTestContainers(ctx context.Context) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return
	}
	defer cli.Close()

	for _, name := range testContainerNames {
		f := filters.NewArgs(filters.Arg("name", "^/"+name+"$"))
		containers, err := cli.ContainerList(ctx, container.ListOptions{
			All:     true,
			Filters: f,
		})
		if err != nil || len(containers) == 0 {
			continue
		}
		for _, c := range containers {
			_ = cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true})
		}
	}
}

var (
	portalBuildOnce  sync.Once
	portalBuildError error
	portalContainer  *PortalContainer
	portalOnce       sync.Once
	portalStartErr   error
)

// PortalContainer wraps a testcontainers environment: SurrealDB + vire-server + portal.
type PortalContainer struct {
	portal    testcontainers.Container
	server    testcontainers.Container
	surrealDB testcontainers.Container
	network   *testcontainers.DockerNetwork
	ctx       context.Context
	cancel    context.CancelFunc
	url       string
}

// URL returns the base URL of the running portal container.
func (p *PortalContainer) URL() string {
	return p.url
}

// CollectLogs saves container stdout/stderr to dir/.
func (p *PortalContainer) CollectLogs(dir string) {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	os.MkdirAll(dir, 0755)

	collectContainerLog := func(c testcontainers.Container, name string) {
		if c == nil {
			return
		}
		reader, err := c.Logs(ctx)
		if err != nil {
			return
		}
		defer reader.Close()

		logs, err := io.ReadAll(reader)
		if err != nil {
			return
		}
		os.WriteFile(filepath.Join(dir, name+".log"), logs, 0644)
	}

	collectContainerLog(p.portal, "portal")
	collectContainerLog(p.server, "vire-server")
}

// Cleanup tears down all containers and the network.
// Uses a fresh context for teardown in case the main context expired.
func (p *PortalContainer) Cleanup() {
	if p == nil {
		return
	}

	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()

	if p.portal != nil {
		p.portal.Terminate(cleanupCtx)
	}
	if p.server != nil {
		p.server.Terminate(cleanupCtx)
	}
	if p.surrealDB != nil {
		p.surrealDB.Terminate(cleanupCtx)
	}
	if p.network != nil {
		p.network.Remove(cleanupCtx)
	}
	if p.cancel != nil {
		p.cancel()
	}
}

// buildPortalImage builds the vire-portal:test Docker image once per test run.
func buildPortalImage() error {
	portalBuildOnce.Do(func() {
		ctx := context.Background()
		projectRoot := FindProjectRoot()

		req := testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    projectRoot,
					Dockerfile: "tests/docker/Dockerfile.server",
					Repo:       "vire-portal",
					Tag:        "test",
					KeepImage:  true,
				},
			},
		}

		_, portalBuildError = testcontainers.GenericContainer(ctx, req)
		if portalBuildError != nil {
			// Image may have built successfully even if container creation failed
			if strings.Contains(portalBuildError.Error(), "vire-portal:test") {
				portalBuildError = nil
			}
		}
	})
	return portalBuildError
}

// startTestEnvironment creates the full 3-container environment:
// SurrealDB → vire-server → vire-portal, all on a shared Docker network.
func startTestEnvironment() (*PortalContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)

	// 0. Remove stale test containers from previous runs (only -tc suffixed names)
	removeStaleTestContainers(ctx)

	// 1. Create Docker network
	testNet, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create docker network: %w", err)
	}

	// 2. Start SurrealDB (name suffixed with -tc to avoid conflicts with dev stack)
	surrealContainer, err := testcontainers.Run(ctx, "surrealdb/surrealdb:v3.0.0",
		testcontainers.WithExposedPorts("8000/tcp"),
		testcontainers.WithCmd("start", "--user", "root", "--pass", "root"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name: "vire-db-tc",
			},
		}),
		network.WithNetwork([]string{"surrealdb"}, testNet),
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
		return nil, fmt.Errorf("start surrealdb: %w", err)
	}

	// 3. Get SurrealDB container IP (bypass Docker DNS for CGO_ENABLED=0)
	surrealIP, err := surrealContainer.ContainerIP(ctx)
	if err != nil {
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("get surrealdb IP: %w", err)
	}

	// 4. Start vire-server (pulled from GHCR, name suffixed to avoid conflicts with dev stack)
	serverContainer, err := testcontainers.Run(ctx, "ghcr.io/bobmcallan/vire-server:latest",
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name: "vire-server-tc",
			},
		}),
		network.WithNetwork([]string{"vire-server"}, testNet),
		testcontainers.WithEnv(map[string]string{
			"VIRE_STORAGE_ADDRESS":           fmt.Sprintf("ws://%s:8000/rpc", surrealIP),
			"VIRE_STORAGE_NAMESPACE":         "vire",
			"VIRE_STORAGE_DATABASE":          "vire_test",
			"VIRE_STORAGE_USERNAME":          "root",
			"VIRE_STORAGE_PASSWORD":          "root",
			"VIRE_ENV":                       "dev",
			"VIRE_SERVER_HOST":               "0.0.0.0",
			"EODHD_API_KEY":                  "test-dummy-key",
			"GEMINI_API_KEY":                 "test-dummy-key",
			"VIRE_AUTH_JWT_SECRET":           "test-jwt-secret-for-ci",
			"VIRE_AUTH_GOOGLE_CLIENT_ID":     "test-google-client-id",
			"VIRE_AUTH_GOOGLE_CLIENT_SECRET": "test-google-client-secret",
			"VIRE_AUTH_GITHUB_CLIENT_ID":     "test-github-client-id",
			"VIRE_AUTH_GITHUB_CLIENT_SECRET": "test-github-client-secret",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/api/health").WithPort("8080/tcp").WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("start vire-server: %w", err)
	}

	// 5. Get vire-server container IP
	serverIP, err := serverContainer.ContainerIP(ctx)
	if err != nil {
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("get vire-server IP: %w", err)
	}

	// 6. Start vire-portal (name suffixed to avoid conflicts with dev stack)
	portalCtr, err := testcontainers.Run(ctx, "vire-portal:test",
		testcontainers.WithExposedPorts("8080/tcp"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Name: "vire-portal-tc",
			},
		}),
		network.WithNetwork([]string{"vire-portal"}, testNet),
		testcontainers.WithEnv(map[string]string{
			"VIRE_API_URL":         fmt.Sprintf("http://%s:8080", serverIP),
			"VIRE_AUTH_JWT_SECRET": "test-jwt-secret-for-ci",
			"VIRE_ENV":             "dev",
			"VIRE_SERVER_HOST":     "0.0.0.0",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/api/health").WithPort("8080/tcp").WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("start vire-portal: %w", err)
	}

	// 7. Get mapped portal URL for browser tests
	mappedPort, err := portalCtr.MappedPort(ctx, "8080/tcp")
	if err != nil {
		portalCtr.Terminate(ctx)
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("get portal mapped port: %w", err)
	}

	host, err := portalCtr.Host(ctx)
	if err != nil {
		portalCtr.Terminate(ctx)
		serverContainer.Terminate(ctx)
		surrealContainer.Terminate(ctx)
		testNet.Remove(ctx)
		cancel()
		return nil, fmt.Errorf("get portal host: %w", err)
	}

	url := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return &PortalContainer{
		portal:    portalCtr,
		server:    serverContainer,
		surrealDB: surrealContainer,
		network:   testNet,
		ctx:       ctx,
		cancel:    cancel,
		url:       url,
	}, nil
}

// StartPortal starts the full test environment (one per test process).
// Returns nil when VIRE_TEST_URL is set (manual mode -- tests use the existing server).
func StartPortal(t *testing.T) *PortalContainer {
	t.Helper()
	if os.Getenv("VIRE_TEST_URL") != "" {
		return nil
	}

	portalOnce.Do(func() {
		if err := buildPortalImage(); err != nil {
			portalStartErr = fmt.Errorf("build portal image: %w", err)
			return
		}
		var err error
		portalContainer, err = startTestEnvironment()
		if err != nil {
			portalStartErr = err
		}
	})

	if portalStartErr != nil {
		t.Fatalf("Failed to start test environment: %v", portalStartErr)
	}
	return portalContainer
}

// StartPortalForTestMain starts the full test environment for use in TestMain (no *testing.T).
// Returns (nil, nil) when VIRE_TEST_URL is set (manual mode).
func StartPortalForTestMain() (*PortalContainer, error) {
	if os.Getenv("VIRE_TEST_URL") != "" {
		return nil, nil
	}

	portalOnce.Do(func() {
		if err := buildPortalImage(); err != nil {
			portalStartErr = fmt.Errorf("build portal image: %w", err)
			return
		}
		var err error
		portalContainer, err = startTestEnvironment()
		if err != nil {
			portalStartErr = err
		}
	})

	if portalStartErr != nil {
		return nil, portalStartErr
	}
	return portalContainer, nil
}
