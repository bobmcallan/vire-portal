# Requirements: Go Architecture Scaffold

**Date:** 2026-02-14
**Requested:** Scaffold the Go-based portal architecture as described in docs/architecture-comparison.md. Foundation only — landing page as proof-of-concept.

## Scope

### In Scope
- Go module initialization and project structure (cmd/, internal/, pages/)
- HTTP server with standard net/http routing and middleware stack
- TOML configuration system (server, storage, logging, auth)
- Data layer abstraction (interface) with BadgerDB implementation
- Models for user, session, config
- Landing page: handler + Go HTML template + static assets
- Health check and version endpoints
- Middleware: logging, recovery, CORS, request ID
- Static file serving (CSS, JS from pages/static/)
- Dockerfile for Go multi-stage build
- Basic Go tests (handler tests, config tests, storage interface tests)
- Updated docker-compose files for Go binary

### Out of Scope (future phases)
- OAuth authentication flow (Google/GitHub)
- Dashboard, settings, connect, billing pages
- Stripe integration
- API proxy to vire-gateway
- WebSocket real-time events
- Full test suite for all pages
- Cloud Run Terraform updates
- Removing existing TypeScript/Preact code

## Approach
- Follow Quaero patterns from /home/bobmc/development/quaero
- Standard net/http (no external router framework)
- BadgerDB via badgerhold for embedded storage
- Data layer as Go interface — BadgerDB implements it, future DB can swap in
- Go html/template for server-side rendering
- Alpine.js (CDN) for client-side interactivity
- IBM Plex Mono font, 80s B&W aesthetic carried forward
- Keep existing SPA code alongside (both can coexist temporarily)

## Files Expected to Change/Create
- `cmd/portal/main.go` (new)
- `internal/server/server.go` (new)
- `internal/server/middleware.go` (new)
- `internal/server/routes.go` (new)
- `internal/config/config.go` (new)
- `internal/config/defaults.go` (new)
- `internal/storage/storage.go` (new — interface)
- `internal/storage/badger/manager.go` (new)
- `internal/handlers/landing.go` (new)
- `internal/handlers/health.go` (new)
- `internal/handlers/version.go` (new)
- `internal/handlers/static.go` (new)
- `internal/models/user.go` (new)
- `internal/models/session.go` (new)
- `pages/landing.html` (new)
- `pages/partials/head.html` (new)
- `pages/partials/nav.html` (new)
- `pages/partials/footer.html` (new)
- `pages/static/portal.css` (new)
- `pages/static/common.js` (new)
- `go.mod` (new)
- `portal.toml` (new — example config)
- `Dockerfile.go` (new — separate from existing Dockerfile)
- `docker/docker-compose.go.yml` (new)
