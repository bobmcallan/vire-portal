# Architecture Comparison: SPA vs Go-Based Portal

**Date:** 2026-02-14
**Status:** Phase 1 scaffold implemented (landing page, health, version, config, storage)
**Recommendation:** Migrate to Go-based architecture

## Overview

This document compares two architectural approaches for the Vire portal:

| | **Current: Preact SPA** | **Proposed: Go-Based (Quaero Pattern)** |
|---|---|---|
| Frontend | Preact + Vite + Tailwind CSS | Go templates + Alpine.js + Spectre CSS |
| Backend | Separate gateway (vire-gateway) | Integrated Go server |
| Data | Stateless frontend, all state in gateway | BadgerDB embedded store |
| Config | nginx envsubst at runtime | TOML files with env override |
| Serving | nginx static file server | Go HTTP server (net/http) |
| Build | npm ci + tsc + vite build | go build (single binary) |
| Deployment | Docker (node builder + nginx runtime) | Docker (go builder + alpine runtime) |

## Architecture Diagrams

### Current: SPA Architecture

```
Browser (Preact SPA)
    |
    |-- GET /config.json ---------> nginx (static files)
    |-- GET /assets/* ------------> nginx (static files)
    |-- POST /api/auth/login -----> vire-gateway (separate service)
    |-- GET /api/profile ---------> vire-gateway
    |-- GET /api/usage -----------> vire-gateway
    |-- PUT /api/profile/keys ----> vire-gateway
    |-- POST /api/billing/* ------> vire-gateway --> Stripe
    |
    JWT in memory, refresh via httpOnly cookie
    All rendering happens client-side
    No persistent state in portal
```

### Proposed: Go-Based Architecture

```
Browser
    |
    |-- GET / --------------------> Go server --> HTML template --> rendered page
    |-- GET /static/* ------------> Go server --> static files (CSS/JS)
    |-- GET /profile ------------> Go server --> profile template (user info + Navexa key)
    |-- POST /api/auth/login -----> Go server --> OAuth provider
    |-- GET /api/profile ---------> Go server --> BadgerDB
    |-- PUT /api/profile/keys ----> Go server --> BadgerDB
    |-- WS /ws -------------------> Go server --> real-time events
    |
    Session managed server-side
    Server-rendered HTML with Alpine.js for interactivity
    Data persisted in embedded BadgerDB
```

## Detailed Comparison

### 1. Frontend Rendering

**SPA (Current)**

- Client-side rendering with Preact (3KB gzipped)
- Vite dev server with HMR for development
- TypeScript with strict mode
- Tailwind CSS v4 utility classes
- Manual client-side routing via `window.history.pushState`
- Initial load: fetch config.json, refresh token, then render
- All page transitions are client-side (no server round-trip)

**Go-Based (Proposed)**

- Server-side rendering with Go `html/template`
- Alpine.js for reactive client-side behavior (no build step)
- Spectre CSS framework (or substitute with Tailwind via CDN)
- Pages rendered with data already populated (no loading spinners)
- HTMX or Alpine.js for partial page updates without full reload
- WebSocket for real-time UI updates (job status, events)

**Analysis:**

| Factor | SPA | Go-Based |
|--------|-----|----------|
| Initial page load | Slower (download JS bundle, fetch config, fetch data) | Faster (server renders complete HTML) |
| Subsequent navigation | Faster (no server round-trip) | Slower (full page or partial fetch) |
| SEO | Poor (content rendered client-side) | Good (server-rendered HTML) |
| JavaScript dependency | Required for any content | Graceful degradation possible |
| Build complexity | npm + tsc + vite pipeline | No frontend build step |
| Dev iteration | HMR via Vite (sub-second) | Rebuild binary or use `-Web` flag for template changes |
| Bundle size | ~50KB JS + ~15KB CSS | ~0KB JS framework (Alpine from CDN) |

### 2. State and Data Management

**SPA (Current)**

- All state lives in the browser (memory)
- JWT stored in a module variable (lost on refresh without cookie refresh)
- No persistent client-side storage
- Every page load requires API calls to populate data
- State management: simple pub/sub (32 lines of code)
- No offline capability

**Go-Based (Proposed)**

- Centralised data store (BadgerDB) on the server
- Server maintains authoritative state
- Templates render with pre-fetched data (no client-side loading)
- Session state managed server-side (cookies for session ID)
- Configuration stored in TOML files and KV store
- Can operate offline for cached data

**Analysis:**

The SPA treats the portal as a thin view layer — all data lives in the gateway, and the portal re-fetches everything on each page load. This creates:

- Redundant API calls on every navigation
- Loading spinners on every page transition for authenticated users
- No ability to cache or pre-compute data server-side
- Configuration split between nginx envsubst (runtime) and Vite env vars (build-time)

The Go-based approach centralises data in BadgerDB, meaning:

- Pages render with data already present (no loading states)
- Server can cache, aggregate, and pre-compute before rendering
- Configuration lives in one place (TOML + env overrides)
- Server controls what data reaches the client

### 3. Configuration

**SPA (Current)**

Three configuration layers with different mechanisms:

| Layer | Mechanism | When |
|-------|-----------|------|
| Build-time | `VITE_*` env vars baked into bundle | `npm run build` |
| Docker build | `VERSION`, `BUILD`, `GIT_COMMIT` build args | `docker build` |
| Runtime | nginx envsubst on `API_URL`, `DOMAIN` | Container start |

The SPA fetches `/config.json` (served by nginx) to get runtime values. This works but is indirect — nginx generates a JSON response from environment variables via template substitution.

**Go-Based (Proposed)**

Single configuration system with clear priority:

| Priority | Source | Example |
|----------|--------|---------|
| 1 (lowest) | Defaults | Hardcoded in Go |
| 2 | TOML files | `quaero.toml` |
| 3 | Environment | `QUAERO_*` prefix |
| 4 (highest) | CLI flags | `--port 8080` |

All configuration is available server-side before any request is served. Templates can reference config values directly. No need for a `/config.json` endpoint or nginx envsubst.

**Analysis:**

The SPA configuration is fragmented across build-time, Docker build, and runtime layers. Debugging which value is active requires understanding all three. The Go approach has a single config struct loaded at startup with clear precedence rules.

### 4. Authentication

**SPA (Current)**

- OAuth via gateway redirect flow
- JWT stored in browser memory (lost on tab close)
- Refresh token in httpOnly cookie (survives tab close)
- Automatic 401 retry with token refresh in API client
- Manual base64url JWT decoding for expiry check
- No server-side session — portal is stateless

**Go-Based (Proposed)**

- OAuth handled directly by Go server
- Session stored server-side (BadgerDB or in-memory)
- Cookie-based session ID (httpOnly, Secure, SameSite)
- Server validates session on every request (middleware)
- Token refresh handled server-side (transparent to client)
- Can revoke sessions server-side immediately

**Analysis:**

| Factor | SPA | Go-Based |
|--------|-----|----------|
| Token storage | Browser memory (XSS risk if leaked) | Server-side (not exposed to JS) |
| Session revocation | Must wait for JWT expiry or refresh failure | Immediate server-side deletion |
| CSRF protection | Not needed (Bearer token in header) | Required (cookie-based sessions) |
| Token refresh | Client-side retry logic with dedup | Server-side, transparent |
| Auth state sync | Manual across tabs | Automatic (server-side session) |

### 5. Build and Deployment

**SPA (Current)**

```
Source (TypeScript/Preact)
  --> tsc (type check)
  --> vite build (bundle, minify, hash)
  --> dist/ (static files)
  --> Docker Stage 1: node:20-alpine (npm ci + build)
  --> Docker Stage 2: nginx:1.27-alpine (serve static)
  --> GHCR push
  --> Cloud Run deployment
```

Requires: Node.js 20, npm, TypeScript compiler, Vite bundler, nginx

**Go-Based (Proposed)**

```
Source (Go + HTML templates + static assets)
  --> go build (single binary with embedded templates)
  --> Docker Stage 1: golang:1.25-alpine (compile)
  --> Docker Stage 2: alpine:latest (run binary)
  --> GHCR push
  --> Cloud Run deployment
```

Requires: Go compiler only. No npm, no Node.js, no bundler.

**Analysis:**

| Factor | SPA | Go-Based |
|--------|-----|----------|
| Build dependencies | Node.js + npm + 100s of npm packages | Go compiler only |
| Build output | dist/ directory of static files | Single binary |
| Runtime dependencies | nginx (separate process) | None (Go serves HTTP) |
| Docker image layers | node (build) + nginx (runtime) | golang (build) + alpine (runtime) |
| Image size | ~40MB (nginx-alpine + assets) | ~20-30MB (alpine + binary) |
| Supply chain surface | npm ecosystem (node_modules) | Go modules (vendorable) |
| Startup time | nginx: ~100ms | Go binary: ~50ms |
| Memory usage | nginx: ~10MB + no compute | Go: ~20-50MB with BadgerDB |

### 6. Testing

**SPA (Current)**

- Vitest with jsdom environment
- 118 tests across 13 files
- Component tests with @testing-library/preact
- API client tests with mocked fetch
- Auth flow tests with fake JWTs
- No integration tests (would need gateway running)

**Go-Based (Proposed)**

- Go standard testing (`go test`)
- testify for assertions
- testcontainers for integration tests
- Handler tests with `httptest.NewServer`
- Full request/response cycle testable without browser
- Template rendering testable server-side

**Analysis:**

The SPA tests mock everything — fetch, sessionStorage, DOM. They verify the Preact components render correctly given mocked data, but never test the actual data flow. The Go approach can test the full request-response cycle (HTTP request in, HTML response out) without a browser, and can use testcontainers for integration tests against real databases.

### 7. Developer Experience

**SPA (Current)**

- `npm run dev` — Vite dev server with HMR (~100ms reload)
- TypeScript provides strong editor support and type safety
- Tailwind CSS intellisense in VS Code
- Vitest watch mode for fast test feedback
- Separate gateway must be running for API calls
- Frontend and backend deployed as separate services

**Go-Based (Proposed)**

- Single `go run` command starts everything
- Template changes: copy pages and reload browser (no rebuild)
- Go LSP provides editor support
- `go test ./...` for all tests
- Self-contained: no external services needed
- Single deployment unit

**Analysis:**

| Factor | SPA | Go-Based |
|--------|-----|----------|
| Dev startup | `npm run dev` + gateway must be running | `go run cmd/server/main.go` |
| Hot reload | Vite HMR (sub-second) | Template: file copy; Go: rebuild (~2s) |
| Type safety | TypeScript strict mode | Go static typing |
| API contract | Must match gateway types manually | Types shared in same codebase |
| Debug | Browser DevTools + source maps | Go debugger (delve) + server logs |

### 8. Operational Concerns

**SPA (Current)**

- Two services to deploy and monitor (portal + gateway)
- CORS configuration required between portal and gateway
- nginx configuration for SPA routing and security headers
- Config split across nginx envsubst and Vite env vars
- CDN-cacheable static assets (immutable hashes)
- Zero compute cost when idle (static files only)

**Go-Based (Proposed)**

- Single service to deploy and monitor
- No CORS needed (same origin)
- Security headers set in Go middleware
- Single configuration system
- Server process required (not just static files)
- Compute cost proportional to traffic

## Pros and Cons Summary

### Current SPA Architecture

**Pros:**
- Fast client-side navigation after initial load
- Zero compute cost (static files served by nginx)
- Well-understood frontend ecosystem (React/Preact patterns)
- Strong TypeScript type safety in frontend code
- CDN-friendly (immutable hashed assets)
- Scales independently from backend

**Cons:**
- Two separate services to build, deploy, and monitor
- Fragmented configuration (build-time, Docker, runtime)
- Loading spinners on every authenticated page (data fetched client-side)
- Requires separate gateway for all API operations
- No server-side control over rendered content
- Complex auth flow (client-side JWT management, refresh dedup)
- npm supply chain exposure (hundreds of transitive dependencies)
- Cannot pre-compute or aggregate data before rendering
- API contract must be kept in sync manually between two codebases

### Go-Based Architecture

**Pros:**
- Single deployable binary — one service to build, deploy, monitor
- Centralised data store (BadgerDB) — server controls all state
- Server-side rendering — pages arrive fully populated, no loading spinners
- Single configuration system (TOML + env + flags) with clear precedence
- Full control over HTML output — security headers, CSP, caching in code
- No npm/Node.js dependency — smaller supply chain surface
- Shared types between frontend templates and backend handlers
- WebSocket support for real-time UI updates
- Can operate partially offline (cached data in BadgerDB)
- Templates and static assets can update without binary rebuild
- Authentication handled server-side (sessions, not client-side JWT)
- No CORS complexity (same origin)

**Cons:**
- Go template syntax is less ergonomic than JSX
- No hot module replacement (template changes need file copy + browser refresh)
- Server process required (not zero-cost static hosting)
- Alpine.js is less capable than Preact for complex interactive UIs
- Heavier memory footprint (Go runtime + BadgerDB vs nginx)
- Server-rendered pages have full round-trip latency on navigation
- Less frontend ecosystem tooling (no Vite, no component library ecosystem)
- Go templates lack TypeScript-level type safety in the view layer

## Migration Path

If migrating to the Go-based architecture, the approach would be:

### Phase 1: Server Foundation
- Create Go HTTP server with standard `net/http` routing
- Implement middleware stack (logging, recovery, CORS, session)
- Set up TOML configuration system
- Integrate BadgerDB for persistent storage
- Port OAuth flow to server-side session management

### Phase 2: Page Migration
- Convert each Preact page to a Go HTML template
- Replace Tailwind utility classes with Spectre CSS (or keep Tailwind via CDN)
- Add Alpine.js for interactive elements (key inputs, menus, modals)
- Implement `/api/*` handlers matching current gateway endpoints
- Add WebSocket for real-time status updates

### Phase 3: Feature Parity
- Migrate all 6 pages: landing, callback, dashboard, settings, connect, billing
- Port API client logic to server-side gateway calls
- Implement Stripe integration server-side
- Add health check and version endpoints

### Phase 4: Operational
- Update Dockerfile (Go multi-stage build)
- Update CI/CD workflow for Go builds
- Update deploy scripts for single binary
- Migrate Cloud Run configuration
- Update documentation

## Recommendation

The Go-based architecture is the stronger choice for the Vire portal because:

1. **Single deployable unit** eliminates the operational overhead of coordinating two services (portal + gateway). Configuration, deployment, and debugging all happen in one place.

2. **Centralised data store** means the server controls what data reaches the client. Pages render fully populated. No loading spinners, no client-side data fetching races, no stale state.

3. **Full control over the frontend** — the server decides exactly what HTML, headers, and configuration reach the browser. Security policies (CSP, HSTS, caching) are enforced in Go code, not nginx configuration templates.

4. **Simpler authentication** — server-side sessions eliminate client-side JWT management, refresh token deduplication, and the 401 retry logic. Session revocation is immediate.

5. **Reduced supply chain risk** — no npm ecosystem dependency. The Go module system is vendorable and has a smaller attack surface than node_modules.

6. **Proven pattern** — the Quaero project demonstrates this architecture works at scale with 50+ API routes, WebSocket real-time updates, embedded database, and multi-provider LLM integration — all in a single Go binary.

The tradeoffs (slower page transitions, less ergonomic templates, required server process) are acceptable for a portal that prioritises control, simplicity, and operational reliability over client-side interactivity.
