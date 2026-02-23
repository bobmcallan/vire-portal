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

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	portalBuildOnce  sync.Once
	portalBuildError error
	serverBuildOnce  sync.Once
	serverBuildError error
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

// buildServerImage builds the vire-server:test Docker image from the sibling vire repo.
func buildServerImage() error {
	serverBuildOnce.Do(func() {
		ctx := context.Background()

		// Locate vire-server repo: VIRE_SERVER_ROOT env var or ../vire relative to portal root
		serverRoot := os.Getenv("VIRE_SERVER_ROOT")
		if serverRoot == "" {
			serverRoot = filepath.Join(FindProjectRoot(), "..", "vire")
		}

		req := testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				FromDockerfile: testcontainers.FromDockerfile{
					Context:    serverRoot,
					Dockerfile: "tests/docker/Dockerfile.server",
					Repo:       "vire-server",
					Tag:        "test",
					KeepImage:  true,
				},
			},
		}

		_, serverBuildError = testcontainers.GenericContainer(ctx, req)
		if serverBuildError != nil {
			if strings.Contains(serverBuildError.Error(), "vire-server:test") {
				serverBuildError = nil
			}
		}
	})
	return serverBuildError
}

// startTestEnvironment creates the full 3-container environment:
// SurrealDB → vire-server → vire-portal, all on a shared Docker network.
func startTestEnvironment() (*PortalContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)

	// 1. Create Docker network
	testNet, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create docker network: %w", err)
	}

	// 2. Start SurrealDB
	surrealContainer, err := testcontainers.Run(ctx, "surrealdb/surrealdb:v3.0.0",
		testcontainers.WithExposedPorts("8000/tcp"),
		testcontainers.WithCmd("start", "--user", "root", "--pass", "root"),
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

	// 4. Start vire-server
	serverContainer, err := testcontainers.Run(ctx, "vire-server:test",
		testcontainers.WithExposedPorts("8080/tcp"),
		network.WithNetwork([]string{"vire-server"}, testNet),
		testcontainers.WithEnv(map[string]string{
			"VIRE_STORAGE_ADDRESS": fmt.Sprintf("ws://%s:8000/rpc", surrealIP),
			"VIRE_ENV":             "dev",
			"VIRE_SERVER_HOST":     "0.0.0.0",
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

	// 6. Start vire-portal
	portalCtr, err := testcontainers.Run(ctx, "vire-portal:test",
		testcontainers.WithExposedPorts("8080/tcp"),
		network.WithNetwork([]string{"vire-portal"}, testNet),
		testcontainers.WithEnv(map[string]string{
			"VIRE_API_URL":         fmt.Sprintf("http://%s:8080", serverIP),
			"VIRE_AUTH_JWT_SECRET": "change-me-in-production",
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
		if err := buildServerImage(); err != nil {
			portalStartErr = fmt.Errorf("build server image: %w", err)
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
		if err := buildServerImage(); err != nil {
			portalStartErr = fmt.Errorf("build server image: %w", err)
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
