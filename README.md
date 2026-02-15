# Vire Portal

Web application and MCP server for the Vire investment platform. Hosts a landing page, serves the MCP tool endpoint at `/mcp`, and proxies all tool calls to vire-server.

The portal is a Go server that renders HTML templates with Alpine.js for interactivity and provides an MCP (Model Context Protocol) endpoint for Claude and other MCP clients. It proxies tool calls to vire-server, injecting X-Vire-* headers for user context. Served from a Docker container alongside vire-server.

> **Repository layout:** This repo contains the portal server and MCP server code. The Docker images (`ghcr.io/bobmcallan/vire-portal:latest` and `ghcr.io/bobmcallan/vire-mcp:latest`) run alongside `ghcr.io/bobmcallan/vire-server:latest` in a three-service Docker Compose stack. Portal infrastructure (Cloud Run deployment) is managed by [vire-infra](https://github.com/bobmcallan/vire-infra) Terraform (`infra/modules/portal/`).

## Tech Stack

- **Go 1.25+** with standard `net/http` (no framework)
- **Go `html/template`** for server-side rendering
- **Alpine.js** (CDN) for client-side interactivity
- **Stateless** -- all user data managed by vire-server via REST API
- **TOML** configuration with priority: defaults < file < env (VIRE_ prefix) < CLI flags
- **Port 8080** -- default port; Docker local dev overrides to 4241 via `docker/vire-portal.toml`
- **80s B&W aesthetic** -- IBM Plex Mono, no border-radius, no box-shadow, monochrome only
- **No Firebase Auth SDK** -- OAuth is handled via direct HTTP redirects and gateway API calls

## Routes

| Route | Handler | Auth | Description |
|-------|---------|------|-------------|
| `GET /` | PageHandler | No | Landing page (server-rendered HTML template) |
| `GET /dashboard` | DashboardHandler | No | Dashboard (MCP config, tools, config status) |
| `GET /static/*` | PageHandler | No | Static files (CSS, JS) |
| `POST /mcp` | MCPHandler | No | MCP endpoint (Streamable HTTP transport, dynamic tools) |
| `GET /api/health` | HealthHandler | No | Health check (`{"status":"ok"}`) |
| `GET /api/version` | VersionHandler | No | Version info (JSON) |
| `POST /api/auth/dev` | AuthHandler | No | Dev-only login (creates session, redirects to `/dashboard`; 404 in prod) |
| `POST /api/auth/logout` | AuthHandler | No | Clears session cookie, redirects to `/` |
| `GET /settings` | SettingsHandler | No | Settings page (Navexa API key management) |
| `POST /settings` | SettingsHandler | No | Save settings (requires session cookie) |

## Prerequisites

- Go 1.25+

## Development

```bash
# Build the server binary
go build ./cmd/vire-portal/

# Run the server (auto-discovers docker/vire-portal.toml)
go run ./cmd/vire-portal/

# Run with custom port
go run ./cmd/vire-portal/ -p 9090

# Run with custom config
go run ./cmd/vire-portal/ -c custom.toml

# Run all tests
go test ./...

# Run tests verbose
go test -v ./...

# Vet for issues
go vet ./...
```

The server runs on `http://localhost:8080` by default (Docker local dev overrides to 4241 via `docker/vire-portal.toml`).

## Dev Mode

Set `environment = "dev"` in the TOML config or `VIRE_ENV=dev` as an environment variable to enable dev mode. This adds:

- A "DEV LOGIN" button on the landing page that bypasses OAuth
- `POST /api/auth/dev` endpoint that creates a minimal unsigned JWT session for `bobmcallan@gmail.com` and sets an httpOnly `vire_session` cookie
- All other functionality remains identical to prod

Dev mode is disabled by default (`environment = "prod"`). The `POST /api/auth/dev` route returns 404 when not in dev mode.

```bash
# Run in dev mode
VIRE_ENV=dev go run ./cmd/vire-portal/

# Or set in config file
# environment = "dev"
```

## Logging

Logging uses [arbor](https://github.com/ternarybob/arbor) with a fluent API. By default, logs are written to both console and file (`logs/vire-portal.log`). Configure via the `[logging]` section in the TOML config:

```toml
[logging]
level = "info"
format = "text"
outputs = ["console", "file"]
file_path = "logs/vire-portal.log"
max_size_mb = 10
max_backups = 5
```

## Configuration

Configuration priority (highest wins): CLI flags > environment variables > TOML file > defaults.

| Setting | TOML Key | Environment Variable | CLI Flag | Default |
|---------|----------|---------------------|----------|---------|
| Server port | `server.port` | `VIRE_SERVER_PORT` | `-port`, `-p` | `8080` |
| Server host | `server.host` | `VIRE_SERVER_HOST` | `-host` | `localhost` |
| API URL | `api.url` | `VIRE_API_URL` | -- | `http://localhost:8080` |
| Portfolios | `user.portfolios` | `VIRE_DEFAULT_PORTFOLIO` | -- | `[]` |
| Display currency | `user.display_currency` | `VIRE_DISPLAY_CURRENCY` | -- | `""` |
| Environment | `environment` | `VIRE_ENV` | -- | `prod` |
| Log level | `logging.level` | `VIRE_LOG_LEVEL` | -- | `info` |
| Log format | `logging.format` | `VIRE_LOG_FORMAT` | -- | `text` |
| Log outputs | `logging.outputs` | -- | -- | `["console", "file"]` |
| Log file path | `logging.file_path` | -- | -- | `logs/vire-portal.log` |
| Log max size (MB) | `logging.max_size_mb` | -- | -- | `10` |
| Log max backups | `logging.max_backups` | -- | -- | `5` |

The config file is auto-discovered from `vire-portal.toml` or `docker/vire-portal.toml`. Specify explicitly with `-c path/to/config.toml`.

The `[api]` and `[user]` sections configure the MCP proxy. `api.url` points to the vire-server instance. User context is injected as X-Vire-* headers on every proxied request. All user data is managed by vire-server.

## MCP Endpoint

The portal hosts an MCP (Model Context Protocol) server at `POST /mcp` using [mcp-go](https://github.com/mark3labs/mcp-go) with Streamable HTTP transport. Claude and other MCP clients connect to this endpoint to access investment tools.

### Architecture

```
Claude / MCP Client
  |
  | POST /mcp (Streamable HTTP)
  v
vire-portal (:8080)
  |  internal/mcp/ package
  |  - Dynamic tool catalog from GET /api/mcp/tools
  |  - Generic proxy handler (path/query/body params)
  |  - X-Vire-* header injection from config
  v
vire-server (:8080)
```

At startup, the portal fetches the tool catalog from vire-server's `GET /api/mcp/tools` endpoint with retry (3 attempts, 2s backoff). Each catalog entry defines the tool name, description, HTTP method, URL path template, and parameters. The portal validates each entry (non-empty name/method/path, method whitelist, `/api/` path prefix, no path traversal) and skips duplicates. Valid tools are dynamically registered as MCP tools and routed to the appropriate REST endpoints. If vire-server is unreachable after all retries, the portal starts with 0 tools (non-fatal).

All tool calls are proxied to vire-server. The portal does not parse or format responses -- it returns raw JSON from vire-server, letting the MCP client (Claude) format the output.

### Connecting Claude

Add this to your Claude Code MCP settings or `~/.claude.json`:

```json
{
  "mcpServers": {
    "vire": {
      "type": "http",
      "url": "http://localhost:4241/mcp"
    }
  }
}
```

### Tools

Tools are registered dynamically from vire-server's `GET /api/mcp/tools` catalog at startup. The catalog defines each tool's name, description, HTTP method, URL path template, and parameters (path, query, body). The portal builds MCP tool definitions and generic handlers from the catalog entries. See the [vire-server README](https://github.com/bobmcallan/vire) for the full tool catalog.

### X-Vire-* Headers

The proxy injects these headers on every request to vire-server:

| Header | Source | Description |
|--------|--------|-------------|
| `X-Vire-Portfolios` | Config (`user.portfolios`) | Comma-separated portfolio names |
| `X-Vire-Display-Currency` | Config (`user.display_currency`) | Currency for display values |
| `X-Vire-User-ID` | Session cookie (per-request) | Username from JWT sub claim |

Static headers are set from config on every request. Per-request headers are set when a `vire_session` cookie is present -- the handler decodes the JWT sub claim and injects the user ID. vire-server resolves the user's navexa key internally from the user ID.

## Authentication Flow

The portal uses direct OAuth with Google and GitHub. The gateway handles token exchange and issues its own JWTs. No Firebase Auth SDK is involved.

### Sign-in Flow

The sign-in buttons are **anchor tags** (`<a>`) pointing directly at the gateway's login endpoint. This is necessary because the gateway returns a 302 redirect to the OAuth provider, and you cannot follow cross-origin 302 redirects from `fetch()`.

```
1. User clicks "Sign in with Google" on /
   -> <a href="${API_URL}/api/auth/login/google">Sign in with Google</a>
   -> Browser navigates to the gateway URL (full page navigation, not fetch)
2. Gateway generates a random `state` token, stores it server-side (or in a
   signed cookie), and 302-redirects to the provider's OAuth consent screen
   with params: client_id, redirect_uri, scope, state
   -> redirect_uri = https://${DOMAIN}/auth/callback
   -> The gateway derives redirect_uri from its own config (the portal's DOMAIN).
     The same URI must be registered in Google Cloud Console / GitHub OAuth app.
3. User authorises on the provider's consent screen
4. Provider redirects to https://${DOMAIN}/auth/callback?code=xxx&state=yyy
5. The /auth/callback page extracts `code` and `state` from query params
6. Frontend sends both to gateway: POST /api/auth/callback
   { "provider": "google", "code": "xxx", "state": "yyy" }
7. Gateway validates the state token (CSRF protection), exchanges the code
   for tokens with the provider, creates/updates user profile in Firestore
8. Gateway returns:
   - Session JWT (short-lived, 1h) in response body
   - Refresh token (7d) as httpOnly Secure SameSite=Lax cookie
     (SameSite=Lax is required here -- the callback is a cross-site redirect
      from the OAuth provider, and SameSite=Strict would block the cookie)
9. Frontend stores JWT in memory (not localStorage)
10. Frontend redirects to /dashboard
11. All subsequent API calls include JWT in Authorization: Bearer header
```

**OAuth redirect_uri configuration:**

| Environment | Portal Domain | redirect_uri | Where to Register |
|-------------|--------------|--------------|-------------------|
| dev | `dev.vire.app` | `https://dev.vire.app/auth/callback` | Google Cloud Console, GitHub OAuth App |
| prod | `vire.app` | `https://vire.app/auth/callback` | Google Cloud Console, GitHub OAuth App |

The `redirect_uri` is constructed by the gateway from its own domain configuration, not passed by the portal. Both the Google and GitHub OAuth apps must have the redirect URI registered in their settings, or the provider will reject the request.

### Token Refresh Flow

```
1. Frontend detects JWT expiry (decode exp claim) or receives 401 response
2. POST /api/auth/refresh (refresh token sent automatically via httpOnly cookie)
3. Gateway validates refresh token, issues new JWT
4. Frontend stores new JWT in memory
5. No user interaction required
```

### OAuth Provider Configuration

| Provider | OAuth Endpoint | Scopes | User Info Fields |
|----------|---------------|--------|-----------------|
| Google | `accounts.google.com/o/oauth2/v2/auth` | `openid`, `email`, `profile` | email, name, picture |
| GitHub | `github.com/login/oauth/authorize` | `read:user`, `user:email` | email, login, name, avatar_url |

## API Contract with Gateway

> **Note:** The canonical API design is in the [architecture document](https://github.com/bobmcallan/vire-infra/blob/main/docs/architecture-per-user-deployment.md) (Stage 1). This section derives from it and specifies the portal-facing contract -- request/response shapes the portal must handle. If the gateway API evolves, the architecture doc is authoritative.

The portal communicates exclusively with the vire-gateway (control plane) REST API.

All protected routes require `Authorization: Bearer <jwt>` header. All request/response bodies are JSON.

### Error Response Format

All endpoints return errors in a consistent shape:

```json
{
  "error": {
    "code": "invalid_key",
    "message": "EODHD API returned 401 -- check that the key is correct and has an active subscription"
  }
}
```

| HTTP Status | Meaning | Portal Action |
|-------------|---------|---------------|
| 400 | Bad request (missing required fields) | Show validation error |
| 401 | JWT expired or invalid | Attempt token refresh; if refresh fails, redirect to `/` |
| 403 | Forbidden (account suspended) | Show account status message |
| 404 | Resource not found | Show "not found" state |
| 409 | Conflict (e.g., already provisioned) | Handle idempotently (show existing resource) |
| 422 | Validation failed (e.g., invalid API key) | Show field-level error from response |
| 429 | Rate limited | Show retry message |
| 500 | Server error | Show generic error with retry option |

### Auth Routes (Unauthenticated)

#### `GET /api/auth/login/:provider`

Redirects the browser to the OAuth provider's consent screen. The portal links to this URL directly via anchor tags (`<a href="...">`). The gateway generates a `state` token for CSRF protection, constructs the `redirect_uri` from its domain config, and returns a 302 redirect.

| Parameter | Type | Description |
|-----------|------|-------------|
| `:provider` | path | `google` or `github` |

**Response:** 302 redirect to provider OAuth URL with `client_id`, `redirect_uri`, `scope`, `state` params.

#### `POST /api/auth/callback`

Exchanges an OAuth authorization code for a session. The `state` parameter is generated by the gateway during `GET /api/auth/login/:provider` and passed through the OAuth flow.

**Request body:**
```json
{
  "provider": "google",
  "code": "4/0AY0e-g...",
  "state": "value-from-callback-query-params"
}
```

**Response (200):**
```json
{
  "token": "eyJhbG...",
  "user": {
    "user_id": "uuid",
    "email": "alice@example.com",
    "display_name": "Alice",
    "avatar_url": "https://...",
    "auth_provider": "google",
    "created_at": "2026-02-09T10:00:00Z",
    "status": "active",
    "keys_configured": false,
    "plan": "free"
  }
}
```

Also sets `refresh_token` as httpOnly, Secure, SameSite=Lax cookie.

#### `POST /api/auth/refresh`

Refreshes an expired JWT using the httpOnly refresh token cookie.

**Request body:** None (cookie sent automatically).

**Response (200):**
```json
{
  "token": "eyJhbG..."
}
```

**Response (401):** Refresh token expired or invalid. User must re-authenticate.

#### `POST /api/auth/logout`

Clears the refresh token cookie and invalidates the session.

**Response (200):**
```json
{
  "status": "ok"
}
```

### Profile Routes (JWT Required)

#### `GET /api/profile`

Returns the authenticated user's profile.

**Response (200):**
```json
{
  "user_id": "uuid",
  "email": "alice@example.com",
  "display_name": "Alice",
  "avatar_url": "https://...",
  "auth_provider": "google",
  "created_at": "2026-02-09T10:00:00Z",
  "status": "active",
  "keys_configured": true,
  "default_portfolio": "SMSF",
  "portfolios": ["SMSF", "Personal"],
  "exchange": "AU",
  "plan": "pro",
  "proxy_url": "https://vire-mcp-a1b2c3-xyz.run.app",
  "proxy_status": "running",
  "provisioned_at": "2026-02-09T10:01:00Z"
}
```

#### `PUT /api/profile`

Updates user preferences.

**Request body (partial update -- include only fields to change):**
```json
{
  "default_portfolio": "Personal",
  "portfolios": ["SMSF", "Personal"],
  "exchange": "AU"
}
```

**Editable fields:** `default_portfolio`, `portfolios`, `exchange`. Other profile fields (`log_level`, `display_name`, `avatar_url`) exist in the Firestore data model but are not exposed for portal editing in Stage 1. Identity fields (`email`, `auth_provider`, `user_id`) and infrastructure fields (`proxy_url`, `provisioned_at`) are always read-only.

**Response (200):** Updated profile object (same shape as GET /api/profile).

#### `DELETE /api/profile`

Deletes the user account. Triggers de-provisioning of MCP proxy, secret cleanup, and data deletion (30-day grace period).

**Response (200):**
```json
{
  "status": "deleted",
  "grace_period_ends": "2026-03-11T10:00:00Z"
}
```

### API Key Routes (JWT Required)

#### `PUT /api/profile/keys`

Sets or updates one or more API keys (BYOK). The gateway validates each key against the provider's API before storing. Keys are stored in Secret Manager, never in Firestore.

**Request body:**
```json
{
  "eodhd_key": "abc123...",
  "navexa_key": "def456...",
  "gemini_key": "ghi789..."
}
```

All fields are optional -- include only the keys to set or update.

**Response (200):**
```json
{
  "eodhd_key": {
    "status": "valid",
    "last4": "c123",
    "validated_at": "2026-02-09T10:05:00Z"
  },
  "navexa_key": {
    "status": "valid",
    "last4": "f456",
    "validated_at": "2026-02-09T10:05:00Z",
    "portfolios_found": 2
  },
  "gemini_key": {
    "status": "valid",
    "last4": "i789",
    "validated_at": "2026-02-09T10:05:00Z"
  }
}
```

**Response (422) -- validation failed:**
```json
{
  "eodhd_key": {
    "status": "invalid",
    "error": "API returned 401 -- check that the key is correct and has an active subscription"
  }
}
```

**Key validation methods:**
| Provider | Validation | Endpoint |
|----------|-----------|----------|
| EODHD | `GET /api/exchanges-list/?api_token={key}` | eodhistoricaldata.com |
| Navexa | `GET /v1/portfolios` with `X-API-Key` header | navexa.io |
| Gemini | `models.list` via SDK | generativelanguage.googleapis.com |

#### `DELETE /api/profile/keys/:id`

Removes a specific API key.

| Parameter | Type | Description |
|-----------|------|-------------|
| `:id` | path | `eodhd_key`, `navexa_key`, or `gemini_key` |

**Response (200):**
```json
{
  "status": "removed",
  "key_name": "navexa_key"
}
```

### Provisioning Routes (JWT Required)

#### `POST /api/profile/provision`

Provisions a dedicated MCP proxy (Cloud Run service) for the user. Requires at least `eodhd_key` to be configured.

**Response (200):**
```json
{
  "status": "provisioned",
  "proxy_url": "https://vire-mcp-a1b2c3-xyz123abc-ts.a.run.app",
  "provisioned_at": "2026-02-09T10:01:00Z"
}
```

**Response (400):** EODHD key not configured.

**Response (409):** Already provisioned (returns existing proxy_url).

#### `GET /api/profile/mcp`

Returns MCP connection configuration blocks ready for copy-paste.

**Response (200):**
```json
{
  "proxy_url": "https://vire-mcp-a1b2c3-xyz123abc-ts.a.run.app",
  "claude_code_config": {
    "mcpServers": {
      "vire": {
        "type": "http",
        "url": "https://vire-mcp-a1b2c3-xyz123abc-ts.a.run.app/mcp"
      }
    }
  },
  "claude_desktop_config": {
    "mcpServers": {
      "vire": {
        "url": "https://vire-mcp-a1b2c3-xyz123abc-ts.a.run.app/mcp"
      }
    }
  }
}
```

#### `GET /api/profile/status`

Returns the health and activity status of the user's MCP proxy.

**Response (200):**
```json
{
  "proxy_status": "running",
  "last_activity": "2026-02-09T12:30:00Z",
  "proxy_url": "https://vire-mcp-a1b2c3-xyz123abc-ts.a.run.app"
}
```

Possible `proxy_status` values: `running`, `stopped`, `not_provisioned`, `throttled`.

### Usage Routes (JWT Required)

#### `GET /api/usage`

Returns usage statistics for the current billing period.

**Response (200):**
```json
{
  "period": "2026-02",
  "total_requests": 3421,
  "quota_limit": 10000,
  "quota_remaining": 6579,
  "status": "active",
  "daily_counts": [
    { "date": "2026-02-01", "count": 120 },
    { "date": "2026-02-02", "count": 185 }
  ],
  "top_endpoints": [
    { "endpoint": "portfolio_compliance", "count": 842 },
    { "endpoint": "get_summary", "count": 521 },
    { "endpoint": "compute_indicators", "count": 498 }
  ]
}
```

### Billing Routes (JWT Required)

#### `POST /api/billing/checkout`

Creates a Stripe Checkout session for upgrading to Pro.

**Response (200):**
```json
{
  "checkout_url": "https://checkout.stripe.com/c/pay/..."
}
```

Frontend redirects the browser to `checkout_url` via `window.location.href = checkout_url`. Stripe handles payment. On completion, Stripe redirects back to `https://${DOMAIN}/billing?session_id={CHECKOUT_SESSION_ID}` -- the gateway configures this return URL when creating the session. The gateway receives a Stripe webhook to update the user's plan.

#### `POST /api/billing/portal`

Creates a Stripe Billing Portal session for managing subscription, viewing invoices, and cancelling.

**Response (200):**
```json
{
  "portal_url": "https://billing.stripe.com/p/session/..."
}
```

Frontend redirects the browser to `portal_url`.

## BYOK -- Bring Your Own Keys

Users must provide their own API keys. Vire does not proxy or resell API access.

### Key Configuration UX

The Settings page displays three key fields:

| Service | Required | What It Provides | Where to Get a Key |
|---------|----------|-----------------|-------------------|
| **EODHD** | Yes | Stock prices, fundamentals, screening | [eodhistoricaldata.com](https://eodhistoricaldata.com) -> Pricing -> API key |
| **Navexa** | No | Portfolio sync from your broker | [navexa.io](https://navexa.io) -> Settings -> API |
| **Google Gemini** | No | AI-powered filings analysis, news intelligence | [aistudio.google.com/apikey](https://aistudio.google.com/apikey) |

### Display States

Each key field shows one of (80s B&W design -- no colours, text-only indicators):
- **Not configured** -- dashed border, "NOT SET" label, empty input with "ADD" button
- **Configured** -- solid border, `[OK] ::::abcd` showing last 4 characters, "UPDATE" and "DELETE" buttons
- **Invalid** -- solid border, `[ERR]` label with error message from validation

### Security

- Keys are encrypted with Cloud KMS and stored in GCP Secret Manager
- Keys are never stored in Firestore (profile holds only a reference)
- Keys are never logged or shared
- After entry, only the last 4 characters are visible
- Each key is injected directly into the user's dedicated MCP proxy as a Cloud Run secret reference

## Dockerfile

Located at `docker/Dockerfile`. Multi-stage build matching the vire ecosystem pattern. Stage 1 builds the Go binary; stage 2 runs it on Alpine.

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /build
ARG VERSION=dev
ARG BUILD=unknown
ARG GIT_COMMIT=unknown
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w \
    -X 'github.com/bobmcallan/vire-portal/internal/config.Version=${VERSION}' \
    -X 'github.com/bobmcallan/vire-portal/internal/config.Build=${BUILD}' \
    -X 'github.com/bobmcallan/vire-portal/internal/config.GitCommit=${GIT_COMMIT}'" \
    -o vire-portal ./cmd/vire-portal

# Runtime stage
FROM alpine:3.21
LABEL org.opencontainers.image.source="https://github.com/bobmcallan/vire-portal"
WORKDIR /app
RUN apk --no-cache add ca-certificates wget
COPY --from=builder /build/vire-portal .
COPY --from=builder /build/pages ./pages
COPY --from=builder /build/docker/vire-portal.toml .
COPY .version .
RUN mkdir -p /app/logs
EXPOSE 8080
HEALTHCHECK NONE
ENTRYPOINT ["./vire-portal"]
```

## GitHub Actions Workflow

`.github/workflows/release.yml` builds and pushes Docker images for both vire-portal and vire-mcp to GHCR on push to main or version tags (matrix strategy):

- Extracts version from `.version` file
- Passes VERSION, BUILD, GIT_COMMIT as Docker build args
- Tags: `latest` (on main), semantic version, short SHA
- Uses GitHub Actions cache for Docker layers

## .version File

The `.version` file at the project root is the single source of truth:

```
version: 0.1.2
build: 02-14-20-27-29
```

- `version:` is the semantic version
- `build:` is the timestamp of the last build, updated automatically by build/deploy scripts
- Both `deploy.sh` and `build.sh` inject VERSION, BUILD, and GIT_COMMIT as Docker build args
- The CI workflow (`release.yml`) uses the same version extraction pattern

## Data Layer

The portal is stateless -- it does not store any user data locally. All user profiles and settings are managed by vire-server via its REST API:

- `GET /api/users/{id}` -- fetch user profile (username, email, role, navexa key status)
- `PUT /api/users/{id}` -- update user fields (e.g., navexa key)

The API client is in `internal/client/vire_client.go`.

## Project Structure

```
vire-portal/
├── .github/
│   └── workflows/
│       └── release.yml              # Docker build + GHCR push (matrix: portal + mcp)
├── cmd/
│   ├── vire-portal/
│   │   └── main.go                  # Portal entry point (flag parsing, config, graceful shutdown)
│   └── vire-mcp/
│       ├── main.go                  # MCP server entry point (stdio + HTTP transport)
│       ├── proxy.go                 # HTTP proxy to vire-server REST API
│       ├── handlers.go              # MCP tool handler implementations
│       ├── formatters.go            # Response formatters (markdown, JSON)
│       └── tools.go                 # MCP tool definitions (25+ tools)
├── internal/
│   ├── app/
│   │   └── app.go                   # Dependency container (Config, Logger, Handlers)
│   ├── config/
│   │   ├── config.go                # TOML loading with defaults -> file -> env -> CLI priority
│   │   ├── config_test.go
│   │   ├── defaults.go              # Default configuration values
│   │   ├── version.go               # Version info (ldflags + .version file)
│   │   └── version_test.go
│   ├── handlers/
│   │   ├── auth.go                  # POST /api/auth/dev (dev-only login), POST /api/auth/logout
│   │   ├── dashboard.go             # GET /dashboard (MCP config, tools, config status)
│   │   ├── handlers_test.go
│   │   ├── health.go                # GET /api/health
│   │   ├── helpers.go               # WriteJSON, RequireMethod, WriteError
│   │   ├── landing.go               # PageHandler (template rendering + static file serving)
│   │   ├── settings.go              # GET/POST /settings (Navexa API key management)
│   │   └── version.go               # GET /api/version
│   ├── client/
│   │   ├── vire_client.go           # HTTP client for vire-server user API (GetUser, UpdateUser)
│   │   └── vire_client_test.go
│   ├── mcp/
│   │   ├── catalog.go               # Dynamic tool catalog types, FetchCatalog, BuildMCPTool, GenericToolHandler
│   │   ├── context.go               # UserContext (per-request user identity for proxy headers)
│   │   ├── handler.go               # MCP HTTP handler (mcp-go StreamableHTTPServer, catalog fetch at startup)
│   │   ├── handlers.go              # errorResult helper, resolvePortfolio
│   │   ├── mcp_test.go              # Tests: catalog, validation, tools, handlers, proxy, integration
│   │   ├── proxy.go                 # HTTP proxy to vire-server with X-Vire-* headers
│   │   └── tools.go                 # RegisterToolsFromCatalog (dynamic registration)
│   ├── server/
│   │   ├── middleware.go             # Correlation ID, logging, CORS, recovery
│   │   ├── middleware_test.go
│   │   ├── route_helpers.go          # RouteByMethod, RouteResourceCollection
│   │   ├── route_helpers_test.go
│   │   ├── routes.go                 # Route registration
│   │   ├── routes_test.go
│   │   └── server.go                 # HTTP server (net/http, timeouts, graceful shutdown)
│   └── vire/                         # Shared packages (migrated from vire repo)
│       ├── common/                   # Version, logging, config, formatting helpers
│       ├── interfaces/               # Service and storage interface contracts
│       └── models/                   # Data structures (portfolio, market, strategy, etc.)
├── pages/
│   ├── dashboard.html                # Dashboard page (MCP config, tools, config status)
│   ├── landing.html                  # Landing page (Go html/template)
│   ├── settings.html                 # Settings page (Navexa API key management)
│   ├── partials/
│   │   ├── head.html                 # HTML head (IBM Plex Mono, Alpine.js CDN)
│   │   ├── nav.html                  # Navigation bar
│   │   └── footer.html               # Footer
│   └── static/
│       ├── css/
│       │   └── portal.css            # 80s B&W aesthetic (no border-radius, no box-shadow)
│       └── common.js                 # Client logging (debugLog, debugError) + Alpine.js init
├── docker/
│   ├── Dockerfile                    # Portal multi-stage build (golang:1.25 -> alpine)
│   ├── Dockerfile.mcp               # MCP multi-stage build (golang:1.25 -> alpine)
│   ├── docker-compose.yml            # 3-service stack: portal + mcp + vire-server
│   ├── docker-compose.dev.yml        # Dev overlay (VIRE_ENV=dev, used by deploy.sh local)
│   ├── docker-compose.ghcr.yml       # GHCR pull + watchtower auto-update
│   ├── vire-portal.toml              # Portal configuration
│   ├── vire-mcp.toml                 # MCP configuration (local)
│   ├── vire-mcp.toml.docker          # MCP configuration (Docker/CI)
│   └── README.md                     # Docker usage documentation
├── docs/
│   ├── requirements.md               # API contracts and architecture
│   └── architecture-comparison.md
├── scripts/
│   ├── deploy.sh                     # Deploy orchestration (local/ghcr/down/prune)
│   ├── build.sh                      # Docker image builder (--portal, --mcp, or both)
│   └── test-scripts.sh               # Validation suite for scripts and configs
├── .dockerignore
├── .version                          # Version metadata (source of truth)
├── go.mod
├── go.sum
├── .gitignore
├── LICENSE
└── README.md
```

## Docker (local)

The project includes deployment scripts matching the [vire](https://github.com/bobmcallan/vire) project patterns. See `docker/README.md` for full details.

### Three-Service Stack (portal + mcp + vire-server)

The recommended deployment runs all three services together via docker-compose:

```bash
# Build portal + mcp and start all services (uses dev compose overlay)
./scripts/deploy.sh local
```

This starts:
- **vire-portal** on port 4241 -- landing page + MCP endpoint (dev mode)
- **vire-mcp** on port 4243 -- standalone MCP server (proxies to vire-server)
- **vire-server** on port 4242 -- backend API (pulled from GHCR)

Local deploys automatically use `docker-compose.dev.yml` as a compose overlay, which sets `VIRE_ENV=dev` to enable dev mode (dev login, etc.). The base `docker-compose.yml` stays unchanged for prod-like builds.

The portal connects to vire-server via `VIRE_API_URL=http://vire-server:8080` (Docker internal network). Claude connects to `http://localhost:4241/mcp` (portal) or `http://localhost:4243/mcp` (standalone mcp).

### Portal Only

```bash
# Build and run portal standalone (MCP proxy will fail without vire-server)
./scripts/deploy.sh local

# Build with forced rebuild (no cache)
./scripts/deploy.sh local --force

# Deploy from GHCR with watchtower auto-update
./scripts/deploy.sh ghcr

# Stop all containers
./scripts/deploy.sh down

# Prune stopped containers and dangling images
./scripts/deploy.sh prune
```

Alternatively, build a standalone image without docker-compose:

```bash
# Build Docker image with version injection
./scripts/build.sh

# Build with verbose output
./scripts/build.sh --verbose

# Clean existing images and rebuild
./scripts/build.sh --clean
```

Or use docker directly:

```bash
# Build the Docker image
docker build -f docker/Dockerfile -t vire-portal:latest .

# Run on host port 4241
docker run -p 4241:8080 \
  -e VIRE_SERVER_HOST=0.0.0.0 \
  -e VIRE_API_URL=http://host.docker.internal:8080 \
  vire-portal:latest
```

## Cloud Run Deployment

The portal runs on Cloud Run, deployed via vire-infra Terraform. The Terraform module is at `infra/modules/portal/main.tf` in the vire-infra repo.

### Configuration from Terraform

```hcl
# Simplified from vire-infra/infra/modules/portal/main.tf
resource "google_cloud_run_v2_service" "portal" {
  name     = "vire-portal"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }
    containers {
      image = "ghcr.io/bobmcallan/vire-portal:latest"
      ports { container_port = 8080 }
      env { name = "VIRE_SERVER_HOST"; value = "0.0.0.0" }
      env { name = "VIRE_SERVER_PORT"; value = "8080" }
      env { name = "VIRE_LOG_LEVEL";   value = "info" }
      resources {
        limits = { cpu = "1", memory = "256Mi" }
      }
    }
  }
}
```

Key properties:
- **Image:** `ghcr.io/bobmcallan/vire-portal:latest` (published by this repo's GitHub Actions)
- **Port:** 8080
- **Ingress:** All (public website)
- **Auth:** Unauthenticated (public access via IAM allUsers)
- **Scaling:** 0-3 instances (scales to zero when idle)
- **Resources:** 1 CPU, 256Mi memory

## Releasing

Push a version tag to trigger the GitHub Actions workflow:

```bash
git tag v0.1.2
git push origin v0.1.2
```

This builds and pushes `ghcr.io/bobmcallan/vire-portal` with the version tag and `:latest` to GHCR.

Pushing to `main` also triggers a build with the `:latest` tag. You can trigger a build manually from the Actions tab using "Run workflow".

The vire-infra Terraform references `ghcr.io/bobmcallan/vire-portal:latest`. After a new image is pushed, re-apply Terraform or update the Cloud Run service to pull the new image.

## Architecture Context

The portal and MCP server run alongside vire-server in a three-service Docker Compose stack:

```
     ┌───────────────┐    ┌──────────────────┐
     │     User      │    │  Claude / MCP     │
     │   (browser)   │    │     Client        │
     └───────┬───────┘    └──┬────────────┬───┘
             │ GET /         │ POST /mcp  │ POST /mcp
             │               │            │
    ┌────────┴───────────────┴──┐    ┌────┴────────────────────────┐
    │  vire-portal (:4241)      │    │  vire-mcp (:4243)           │
    │  - Landing page           │    │  - Standalone MCP server    │
    │  - MCP endpoint (dynamic) │    │  - 25+ tools (stdio + HTTP) │
    │  - X-Vire-* headers       │    │  - Direct vire-server proxy │
    └────────────┬──────────────┘    └──────────────┬──────────────┘
                 │ REST API proxy                   │
                 │                                  │
    ┌────────────┴──────────────────────────────────┴──┐
    │  vire-server (:8080)                               │
    │  - Portfolio analysis, market data               │
    │  - Report generation, stock screening            │
    │  - Strategy and plan management                  │
    └──────────────────────────────────────────────────┘
```

Both the portal and vire-mcp proxy tool calls to vire-server. The portal uses dynamic tool registration from the vire-server catalog and injects X-Vire-* headers for user context. vire-mcp is a standalone MCP server with built-in tool definitions. For future multi-user cloud deployment, the portal will also integrate with vire-gateway for OAuth and user management.

## License

Private repository.
