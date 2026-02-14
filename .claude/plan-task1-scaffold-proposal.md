# Go Scaffold Proposal for vire-portal

## Investigation Summary

After thoroughly reading the Quaero codebase, I've identified the following patterns to replicate:

### Quaero Architecture Patterns

1. **Entry Point** (`cmd/quaero/main.go`):
   - Flag parsing for config paths, port, host, version
   - Config auto-discovery (look for `vire.toml` in current dir, then `deployments/local/`)
   - Load config via `common.LoadFromFiles()` with layered priority
   - Apply CLI flag overrides
   - Init logger
   - Create `app.App` struct (holds all dependencies)
   - Create `server.Server` (wraps `app.App`)
   - Start server in goroutine, wait for SIGINT/SIGTERM
   - Graceful shutdown with timeout

2. **App Struct** (`internal/app/app.go`):
   - Central dependency container holding Config, Logger, StorageManager, handlers
   - `New()` function initializes everything in dependency order: DB -> services -> handlers
   - `Close()` tears down in reverse order
   - StorageManager is an interface (allows swapping Badger for another DB)

3. **Server** (`internal/server/`):
   - `Server` struct wraps `app.App`, `*http.ServeMux`, `*http.Server`
   - Routes registered via `setupRoutes()` using `mux.HandleFunc()`
   - Standard `net/http` mux (no external router)
   - Middleware chain: correlationID -> logging -> CORS -> recovery
   - `route_helpers.go` provides `RouteResourceCollection`, `RouteResourceItem`, etc.

4. **Config** (`internal/common/config.go`):
   - TOML-based using `github.com/pelletier/go-toml/v2`
   - `NewDefaultConfig()` returns struct with all defaults
   - `LoadFromFiles()` merges defaults -> file1 -> file2 -> env
   - `applyEnvOverrides()` reads `QUAERO_*` environment variables
   - `ApplyFlagOverrides()` applies CLI flags (highest priority)

5. **Storage** (`internal/storage/`):
   - `storage/factory.go` - `NewStorageManager()` creates the right implementation
   - `storage/badger/connection.go` - `BadgerDB` struct wraps `badgerhold.Store`
   - `storage/badger/manager.go` - `Manager` implements `interfaces.StorageManager`
   - Interface pattern: all storage accessed via interfaces, Badger is the implementation

6. **Handlers** (`internal/handlers/`):
   - Each handler is a struct with dependencies injected via constructor
   - `helpers.go` provides `WriteJSON`, `RequireMethod`, `WriteError`, etc.
   - `page_handler.go` serves Go templates and static files

7. **Pages** (`pages/`):
   - `pages/*.html` - full page templates
   - `pages/partials/*.html` - reusable template fragments
   - `pages/static/` - CSS, JS, images
   - Templates use `{{template "partial.html" .}}` composition
   - Data passed as `map[string]interface{}` with `Page` and other keys

8. **Version** (`internal/common/version.go`):
   - Build-time injection via `-ldflags`
   - `.version` file fallback
   - `GetVersion()`, `GetBuild()`, `GetGitCommit()`, `GetFullVersion()`

---

## Proposed vire-portal Go Scaffold

### Directory Structure

```
vire-portal/
  cmd/
    server/
      main.go                    # Entry point (matches Quaero cmd/quaero/main.go)
  internal/
    app/
      app.go                     # App struct, New(), Close()
    common/
      config.go                  # Config struct, TOML loading, env overrides
      version.go                 # Version info with ldflags + .version file
    handlers/
      api.go                     # Health, version, 404 handlers
      helpers.go                 # WriteJSON, RequireMethod, etc.
      page_handler.go            # Template rendering + static file serving
    server/
      server.go                  # HTTP server struct, Start(), Shutdown()
      routes.go                  # Route registration
      middleware.go              # Logging, recovery, CORS middleware
      route_helpers.go           # RouteByMethod, RouteResourceCollection, etc.
    storage/
      factory.go                 # NewStorageManager() factory
      badger/
        connection.go            # BadgerDB connection wrapper
        manager.go               # Storage manager implementation
        kv_storage.go            # Key-value storage (for future use)
    interfaces/
      storage.go                 # Storage interface definitions
  pages/
    index.html                   # Landing page template
    partials/
      head.html                  # <head> with IBM Plex Mono, meta tags
      footer.html                # Footer partial
    static/
      css/
        vire.css                 # 80s B&W aesthetic CSS
      favicon.ico                # Favicon
  vire.toml                      # Default config file
  Dockerfile.go                  # Go multi-stage Docker build
  docker-compose.go.yml          # Docker compose for Go server
```

### Dependencies (go.mod)

```
module github.com/ternarybob/vire-portal

go 1.25

require (
    github.com/pelletier/go-toml/v2 v2.2.4    // TOML config parsing
    github.com/timshannon/badgerhold/v4 v4.0.3 // BadgerDB via badgerhold
    github.com/google/uuid v1.6.0              // Correlation IDs
)
```

Minimal dependencies matching Quaero's core infrastructure. We use `log/slog` from stdlib instead of arbor (vire-portal doesn't need arbor's channel-based log routing). Standard library `log/slog` provides structured JSON logging.

### Config Struct

```go
type Config struct {
    Server  ServerConfig  `toml:"server"`
    Storage StorageConfig `toml:"storage"`
    Logging LoggingConfig `toml:"logging"`
}

type ServerConfig struct {
    Port int    `toml:"port"` // default: 8080
    Host string `toml:"host"` // default: "localhost"
}

type StorageConfig struct {
    Badger BadgerConfig `toml:"badger"`
}

type BadgerConfig struct {
    Path string `toml:"path"` // default: "./data/vire"
}

type LoggingConfig struct {
    Level  string `toml:"level"`  // default: "info"
    Format string `toml:"format"` // default: "text"
}
```

Environment overrides: `VIRE_SERVER_PORT`, `VIRE_SERVER_HOST`, `VIRE_BADGER_PATH`, `VIRE_LOG_LEVEL`, `VIRE_LOG_FORMAT`

### Storage Interface

```go
// StorageManager provides access to domain-specific storage interfaces.
// Implementations can be swapped (BadgerDB now, centralised DB later).
type StorageManager interface {
    KeyValueStorage() KeyValueStorage
    DB() interface{}
    Close() error
}

// KeyValueStorage provides basic key-value operations.
// This is the only storage interface needed for the initial scaffold.
type KeyValueStorage interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
    GetAll(ctx context.Context) (map[string]string, error)
}
```

### App Struct

```go
type App struct {
    Config         *common.Config
    Logger         *slog.Logger
    StorageManager interfaces.StorageManager

    // HTTP handlers
    APIHandler  *handlers.APIHandler
    PageHandler *handlers.PageHandler
}
```

### Landing Page Approach

The landing page template (`pages/index.html`) renders with Go `html/template`:

- `head.html` partial loads IBM Plex Mono from Google Fonts
- `vire.css` implements the 80s B&W aesthetic:
  - `* { border-radius: 0 !important; box-shadow: none !important; }`
  - Colors: `#000`, `#fff`, `#888` only
  - Font: `IBM Plex Mono` (from Google Fonts CDN)
  - Sharp-edged borders, terminal feel
- Landing page content matches `src/pages/landing.tsx`:
  - VIRE title (h1, tracking-widest)
  - Tagline
  - Sign-in buttons (black/white with inverted hover)
  - Three feature cards with numbered labels [01], [02], [03]
- Alpine.js loaded from CDN for future interactivity (no build step)

### Routes

```
GET  /                -> PageHandler.ServePage("index.html", "home")
GET  /static/*        -> PageHandler.StaticFileHandler
GET  /api/health      -> APIHandler.HealthHandler
GET  /api/version     -> APIHandler.VersionHandler
```

### Middleware Chain

Following Quaero's pattern:
1. Correlation ID (generate UUID, set X-Correlation-ID header)
2. Logging (structured request/response logging via slog)
3. CORS (Access-Control-Allow-Origin: *)
4. Recovery (panic -> 500)

### Dockerfile.go

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X internal/common.Version=$(cat .version)" -o /app/vire-portal ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/vire-portal .
COPY --from=builder /app/pages ./pages
COPY --from=builder /app/vire.toml .
EXPOSE 8080
CMD ["./vire-portal"]
```

### Coexistence with SPA

- Go scaffold lives in `cmd/`, `internal/`, `pages/`, `vire.toml`
- Existing SPA code stays in `src/`, `index.html`, `vite.config.ts`, `package.json`
- Original `Dockerfile` stays for SPA builds
- `Dockerfile.go` and `docker-compose.go.yml` are separate files
- Both can run side-by-side during migration
- `.gitignore` updated to include `data/` (BadgerDB directory)

### What This Scaffold Does NOT Include

- Authentication/OAuth (Phase 2)
- Session management (Phase 2)
- User profile/settings pages (Phase 2)
- Stripe/billing integration (Phase 3)
- MCP proxy provisioning (Phase 3)
- WebSocket (Phase 2+)
- The scaffold focuses purely on: server, config, storage, landing page, health/version
