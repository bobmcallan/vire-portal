# Summary: Go Architecture Scaffold

**Date:** 2026-02-14
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `cmd/portal/main.go` | Entry point — flag parsing, config auto-discovery, graceful shutdown |
| `internal/app/app.go` | App dependency container (Config, Logger, StorageManager, Handlers) |
| `internal/config/config.go` | TOML config loading with defaults → file → env priority |
| `internal/config/defaults.go` | Default configuration values |
| `internal/config/version.go` | Version info via ldflags + .version file |
| `internal/config/config_test.go` | Config tests (defaults, TOML, env overrides, edge cases) |
| `internal/interfaces/storage.go` | StorageManager + KeyValueStorage interfaces |
| `internal/storage/factory.go` | Storage factory |
| `internal/storage/badger/connection.go` | BadgerDB connection via badgerhold |
| `internal/storage/badger/manager.go` | StorageManager implementation |
| `internal/storage/badger/kv_storage.go` | KeyValueStorage implementation |
| `internal/storage/badger/kv_storage_test.go` | BadgerDB storage tests |
| `internal/handlers/landing.go` | Landing page handler (template rendering) |
| `internal/handlers/health.go` | GET /api/health handler |
| `internal/handlers/version.go` | GET /api/version handler |
| `internal/handlers/helpers.go` | WriteJSON, RequireMethod, WriteError |
| `internal/handlers/handlers_test.go` | Handler tests |
| `internal/server/server.go` | HTTP server with timeouts and graceful shutdown |
| `internal/server/routes.go` | Route registration |
| `internal/server/middleware.go` | Correlation ID, logging, CORS, recovery |
| `internal/server/route_helpers.go` | RouteByMethod, RouteResourceCollection, RouteResourceItem |
| `internal/server/server_test.go` | Server and route tests |
| `internal/server/middleware_test.go` | Middleware tests |
| `internal/models/user.go` | User model |
| `internal/models/session.go` | Session model |
| `internal/models/models_test.go` | Model tests |
| `pages/landing.html` | Landing page — 80s B&W, IBM Plex Mono, Alpine.js |
| `pages/partials/head.html` | Common HTML head |
| `pages/partials/nav.html` | Navigation bar |
| `pages/partials/footer.html` | Footer with version |
| `pages/static/css/portal.css` | Monochrome CSS (no border-radius, no box-shadow) |
| `pages/static/common.js` | Alpine.js component skeleton |
| `go.mod` | Go module with badgerhold, go-toml, uuid dependencies |
| `portal.toml` | Example TOML configuration |
| `Dockerfile.portal` | Multi-stage Go build (golang:1.25-alpine → alpine) |
| `docker/docker-compose.go.yml` | Docker Compose for Go portal (port 8081) |
| `README.md` | Added Go development section with routes, config, Docker, data layer docs |
| `docs/architecture-comparison.md` | Updated status to reflect scaffold implementation |

## Tests
- Go tests: 52 passing across 6 test files (config, handlers, storage, middleware, server, models)
- Existing SPA tests: 118/118 still passing
- SPA build: clean
- Go vet: clean
- Go build: compiles to 19MB binary

## Documentation Updated
- README.md — Go server section (build, run, test, routes, config, Docker, data layer)
- docs/architecture-comparison.md — status updated

## Devils-Advocate Findings
- **Path traversal in static handler** — fixed with http.FileServer(http.Dir())
- **CORS wildcard** — TODO'd for auth phase (dev-only currently)
- **Security headers** — added (X-Frame-Options, X-Content-Type-Options, Referrer-Policy)
- **MaxBytesReader** — added for non-GET requests
- **BadgerDB permissions** — 0700 for data directory
- **Startup race** — fixed (no time.Sleep, proper error propagation)
- **Template render to buffer** — renders to bytes.Buffer before writing response
- **Alpine.js pinned** — specific version, not wildcard
- **BadgerDB + Cloud Run scaling** — overruled (user chose singleton, interface for future DB swap)
- **net/http ServeMux** — overruled (Go 1.22+ sufficient for ~5 routes)

## Notes
- Go scaffold coexists with existing SPA code (separate Dockerfiles, different ports)
- Dockerfile.portal (Go) uses port 8081; Dockerfile (SPA) uses port 8080
- Data layer uses interface pattern (StorageManager + KeyValueStorage) for future DB migration
- Frontend: Go html/template SSR + Alpine.js from CDN (no build step)
- Follow-up needed: OAuth auth flow, remaining 5 pages, CI workflow for Go, CSRF middleware
