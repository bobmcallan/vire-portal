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
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	portalBuildOnce  sync.Once
	portalBuildError error
	portalContainer  *PortalContainer
	portalOnce       sync.Once
	portalStartErr   error
)

// PortalContainer wraps a testcontainers container running the portal server.
type PortalContainer struct {
	container testcontainers.Container
	ctx       context.Context
	cancel    context.CancelFunc
	url       string
}

// URL returns the base URL of the running portal container.
func (p *PortalContainer) URL() string {
	return p.url
}

// CollectLogs saves container stdout/stderr to dir/container.log.
func (p *PortalContainer) CollectLogs(dir string) {
	if p == nil || p.container == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	reader, err := p.container.Logs(ctx)
	if err != nil {
		return
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return
	}

	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "container.log"), logs, 0644)
}

// Cleanup terminates the container and cancels the context.
func (p *PortalContainer) Cleanup() {
	if p == nil || p.container == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	p.container.Terminate(ctx)
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

// startPortalContainer creates and starts a portal container, returning the mapped URL.
func startPortalContainer() (*PortalContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	env := map[string]string{
		"VIRE_ENV":         "dev",
		"VIRE_SERVER_HOST": "0.0.0.0",
	}
	if apiURL := os.Getenv("VIRE_API_URL"); apiURL != "" {
		env["VIRE_API_URL"] = apiURL
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "vire-portal:test",
			Name:         "vire-portal-test",
			ExposedPorts: []string{"8080/tcp"},
			Env:          env,
			WaitingFor:   wait.ForHTTP("/api/health").WithPort("8080/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start portal container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "8080/tcp")
	if err != nil {
		container.Terminate(ctx)
		cancel()
		return nil, fmt.Errorf("get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		cancel()
		return nil, fmt.Errorf("get host: %w", err)
	}

	url := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())

	return &PortalContainer{
		container: container,
		ctx:       ctx,
		cancel:    cancel,
		url:       url,
	}, nil
}

// StartPortal starts a shared portal container (one per test process).
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
		portalContainer, err = startPortalContainer()
		if err != nil {
			portalStartErr = err
		}
	})

	if portalStartErr != nil {
		t.Fatalf("Failed to start portal: %v", portalStartErr)
	}
	return portalContainer
}

// StartPortalForTestMain starts the portal container for use in TestMain (no *testing.T).
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
		portalContainer, err = startPortalContainer()
		if err != nil {
			portalStartErr = err
		}
	})

	if portalStartErr != nil {
		return nil, portalStartErr
	}
	return portalContainer, nil
}
