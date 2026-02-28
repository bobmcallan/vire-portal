# Vire Portal

Web application and MCP server for the Vire investment platform. Hosts a landing page, serves the MCP tool endpoint at `/mcp`, and proxies all tool calls to vire-server.

The portal is a Go server that renders HTML templates with Alpine.js for interactivity and provides an MCP (Model Context Protocol) endpoint for Claude and other MCP clients. It proxies tool calls to vire-server, injecting X-Vire-* headers for user context. Served from a Docker container alongside vire-server.

> **Repository layout:** This repo contains the portal server and MCP bridge code. The Docker images (`ghcr.io/bobmcallan/vire-portal:latest` and `ghcr.io/bobmcallan/vire-mcp:latest`) are published to GHCR. The portal runs alongside `ghcr.io/bobmcallan/vire-server:latest` in a Docker Compose stack. `vire-mcp` is a stdio-to-HTTP bridge launched by Claude Desktop — it connects to vire-portal's MCP endpoint via OAuth 2.1 and re-exposes tools over stdio. Portal infrastructure (Cloud Run deployment) is managed by [vire-infra](https://github.com/bobmcallan/vire-infra) Terraform (`infra/modules/portal/`).

## Tech Stack

- **Go 1.25+** with standard `net/http` (no framework)
- **Go `html/template`** for server-side rendering
- **Alpine.js** (CDN) for client-side interactivity
- **Chart.js v4** (CDN) for portfolio growth chart
- **Stateless** -- all user data managed by vire-server via REST API
- **TOML** configuration with priority: defaults < file < env (VIRE_ prefix) < CLI flags
- **Port 8080** -- default port; Docker local dev overrides to 8881 via `docker/vire-portal.toml`
- **80s B&W aesthetic** -- IBM Plex Mono, no border-radius, no box-shadow, monochrome only
- **No Firebase Auth SDK** -- OAuth is handled via direct HTTP redirects and gateway API calls

## Routes

| Route | Handler | Auth | Description |
|-------|---------|------|-------------|
| `GET /` | PageHandler | No | Landing page (server-rendered HTML template) |
| `GET /dashboard` | DashboardHandler | No | Dashboard (portfolio management, holdings, capital performance, indicators, growth chart) |
| `GET /strategy` | StrategyHandler | No | Strategy page (portfolio strategy and plan editors) |
| `GET /capital` | CapitalHandler | No | Capital page (cash transactions ledger, paged table) |
| `GET /mcp-info` | MCPPageHandler | No | MCP info page (connection config, tools catalog) |
| `GET /docs` | PageHandler | No | Docs page (Navexa setup instructions) |
| `GET /static/*` | PageHandler | No | Static files (CSS, JS) |
| `POST /mcp` | MCPHandler | No | MCP endpoint (Streamable HTTP transport, dynamic tools) |
| `GET /.well-known/oauth-authorization-server` | OAuthServer | No | OAuth 2.1 authorization server metadata |
| `GET /.well-known/oauth-protected-resource` | OAuthServer | No | OAuth 2.1 protected resource metadata |
| `POST /register` | OAuthServer | No | Dynamic Client Registration (RFC 7591) |
| `GET /authorize` | OAuthServer | No | OAuth authorization endpoint (PKCE S256) |
| `POST /token` | OAuthServer | No | Token exchange (authorization_code + refresh_token) |
| `GET /api/health` | HealthHandler | No | Health check (`{"status":"ok"}`) |
| `GET /api/server-health` | ServerHealthHandler | No | Proxied vire-server health check |
| `GET /api/version` | VersionHandler | No | Version info (JSON) |
| `POST /api/auth/login` | AuthHandler | No | Email/password login (forwards to vire-server) |
| `POST /api/auth/logout` | AuthHandler | No | Clears session cookie, redirects to `/` |
| `GET /api/auth/login/google` | AuthHandler | No | Proxies Google OAuth redirect from vire-server |
| `GET /api/auth/login/github` | AuthHandler | No | Proxies GitHub OAuth redirect from vire-server |
| `GET /auth/callback` | AuthHandler | No | OAuth callback (receives `?token=`, sets session cookie) |
| `GET /profile` | ProfileHandler | No | Profile page (user info + Navexa API key management) |
| `POST /profile` | ProfileHandler | No | Save profile (requires session cookie) |

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

# Run unit tests
go test ./internal/... -timeout 120s

# Run all tests (unit + UI -- UI tests start Docker containers)
go test ./...

# Vet for issues
go vet ./...

# Verify auth endpoints on a running server
./scripts/verify-auth.sh
```

### Testing

The test environment is fully self-contained -- no sibling repositories or local builds of vire-server are needed. UI tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a 3-container stack (SurrealDB, vire-server from GHCR, vire-portal) automatically.

**Docker test stack** (`tests/docker/docker-compose.yml`):
- **SurrealDB** -- in-memory database for vire-server
- **vire-server** -- pulled from `ghcr.io/bobmcallan/vire-server:latest` (no local build)
- **vire-portal** -- built from the local repo

Run the full stack manually with compose:

```bash
docker compose -f tests/docker/docker-compose.yml up -d
# Portal available at http://localhost:8881
docker compose -f tests/docker/docker-compose.yml down
```

**UI test runner** (`scripts/ui-test.sh`):

```bash
# Run all UI test suites
./scripts/ui-test.sh all

# Run individual suites
./scripts/ui-test.sh smoke
./scripts/ui-test.sh dashboard
./scripts/ui-test.sh nav
./scripts/ui-test.sh auth
```

Results (logs and screenshots) are written to `tests/logs/{timestamp}/`.

**CI** (`.github/workflows/test.yml`): Runs automatically on PR and push to main. Unit tests run first (`go vet` + `go test ./internal/...`), then UI tests with headless Chrome.

### Browser Tests

UI tests use chromedp (headless Chrome) and are configured via `tests/ui/test_config.toml`:

```toml
[results]
dir = "tests/logs"      # Results directory (timestamped subdirs)

[server]
url = "http://localhost:8881"  # Server under test

[browser]
headless = true            # Set false to see browser
timeout_seconds = 30
```

Test categories:
- **Smoke tests** (`TestSmoke*`): Landing page, login buttons, branding, dashboard loads
- **Dashboard tests** (`TestDashboard*`): Sections, panels, portfolio UI, capital performance, indicators, growth chart, refresh button, design rules
- **Strategy tests** (`TestStrategy*`): Strategy/plan editors, nav active state, Alpine init
- **Capital tests** (`TestCapital*`): Cash transactions table, pagination, summary row, transaction colors
- **Nav tests** (`TestNav*`): Hamburger menu, dropdown, mobile nav
- **Auth tests** (`TestAuth*`): OAuth redirect flows

Results include logs and screenshots in `tests/logs/{timestamp}/`.

The server runs on `http://localhost:8080` by default (Docker local dev overrides to 8881 via `docker/vire-portal.toml`).

## Dev Mode

Set `environment = "dev"` in the TOML config or `VIRE_ENV=dev` as an environment variable to enable dev mode. This adds:

- JWT signature verification is relaxed when `auth.jwt_secret` is empty (legacy fallback extracts `sub` claim without signature check)
- `POST /api/shutdown` endpoint for graceful shutdown via HTTP
- All other functionality remains identical to prod

Dev mode is disabled by default (`environment = "prod"`).

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
| JWT secret | `auth.jwt_secret` | `VIRE_AUTH_JWT_SECRET` | -- | `""` |
| OAuth callback URL | `auth.callback_url` | `VIRE_AUTH_CALLBACK_URL` | -- | `http://localhost:8080/auth/callback` |
| Portal URL | `auth.portal_url` | `VIRE_PORTAL_URL` | -- | `""` |
| Admin users | `admin_users` | `VIRE_ADMIN_USERS` | -- | `""` |
| Service key | `service.key` | `VIRE_SERVICE_KEY` | -- | `""` |
| Portal ID | `service.portal_id` | `VIRE_PORTAL_ID` | -- | hostname |
| Environment | `environment` | `VIRE_ENV` | -- | `prod` |
| Log level | `logging.level` | `VIRE_LOG_LEVEL` | -- | `info` |
| Log format | `logging.format` | `VIRE_LOG_FORMAT` | -- | `text` |
| Log outputs | `logging.outputs` | -- | -- | `["console", "file"]` |
| Log file path | `logging.file_path` | -- | -- | `logs/vire-portal.log` |
| Log max size (MB) | `logging.max_size_mb` | -- | -- | `10` |
| Log max backups | `logging.max_backups` | -- | -- | `5` |

The config file is auto-discovered from `vire-portal.toml` or `docker/vire-portal.toml`. Specify explicitly with `-c path/to/config.toml`.

The `[api]` section configures the MCP proxy. `api.url` points to the vire-server instance. User context is injected as X-Vire-* headers on every proxied request. All user data is managed by vire-server.

## MCP Endpoint

The portal hosts an MCP (Model Context Protocol) server at `POST /mcp` using [mcp-go](https://github.com/mark3labs/mcp-go) with Streamable HTTP transport. Claude and other MCP clients connect to this endpoint to access investment tools.

### Architecture

```
Claude Code / CLI                Claude Desktop
  |                                |
  | POST /mcp (Streamable HTTP)    | stdio
  |                                |
  |                          vire-mcp (bridge)
  |                                |
  |                                | POST /mcp (HTTP + OAuth)
  v                                v
vire-portal (:8881)
  |  internal/mcp/ package
  |  - Dynamic tool catalog from GET /api/mcp/tools
  |  - Generic proxy handler (path/query/body params)
  |  - X-Vire-* header injection from JWT sub claim
  |  - OAuth 2.1 (DCR, PKCE, token exchange)
  v
vire-server (:8080)
```

At startup, the portal fetches the tool catalog from vire-server's `GET /api/mcp/tools` endpoint with retry (3 attempts, 2s backoff). Each catalog entry defines the tool name, description, HTTP method, URL path template, and parameters. The portal validates each entry (non-empty name/method/path, method whitelist, `/api/` path prefix, no path traversal) and skips duplicates. Valid tools are dynamically registered as MCP tools and routed to the appropriate REST endpoints. If vire-server is unreachable after all retries, the portal starts with 0 tools (non-fatal).

All tool calls are proxied to vire-server. The portal does not parse or format responses -- it returns raw JSON from vire-server, letting the MCP client (Claude) format the output.

### Connecting Claude

The portal serves MCP over Streamable HTTP at `POST /mcp` with OAuth 2.1 authentication. All tool logic, catalog management, and version handling live in vire-portal. Clients connect either directly via HTTP (Claude Code, Claude CLI) or via the `vire-mcp` stdio bridge (Claude Desktop).

```
Claude Desktop → stdio → vire-mcp → HTTP + OAuth → vire-portal (:8881/mcp) → vire-server
Claude CLI/Web → HTTP + OAuth → vire-portal (:8881/mcp) → vire-server
```

#### Transport Compatibility

| Client | Transport | How to Connect |
|--------|-----------|----------------|
| Claude Code (CLI) | Streamable HTTP | Direct HTTP URL to portal |
| Claude Desktop (`claude_desktop_config.json`) | stdio | `vire-mcp` binary or Docker |
| Claude Desktop (Connectors UI) | HTTPS | Public HTTPS URL (production) |

#### Claude Code (CLI)

Claude Code supports Streamable HTTP and MCP OAuth natively. On first connection, Claude Code runs the OAuth flow automatically (opens browser, user logs in, receives token).

Add to `~/.claude.json` (global) or project `.mcp.json` (per-project):

```json
{
  "mcpServers": {
    "vire": {
      "type": "url",
      "url": "http://localhost:8881/mcp"
    }
  }
}
```

For a deployed portal with HTTPS:

```json
{
  "mcpServers": {
    "vire": {
      "type": "url",
      "url": "https://portal.vire.app/mcp"
    }
  }
}
```

#### Claude Desktop (local dev via vire-mcp)

Claude Desktop requires stdio transport for local MCP servers. `vire-mcp` is a stdio-to-HTTP bridge — it connects to vire-portal as an MCP client, discovers tools, and re-exposes them over stdio.

**Two connection modes:**

| Mode | Environment Variable | Auth | Use Case |
|------|---------------------|------|----------|
| **OAuth** | `VIRE_PORTAL_URL` | Browser OAuth flow, tokens cached | Local dev with interactive login |
| **Direct** | `VIRE_MCP_URL` | Encrypted UID in URL (no browser) | Docker, CI/CD, headless environments |

**Configuration:**

`vire-mcp` auto-discovers its TOML config from `vire-mcp.toml` or `config/vire-mcp.toml` (binary-relative paths checked first). Environment variables override config values:

| Env Var | Default | Description |
|---------|---------|-------------|
| `VIRE_PORTAL_URL` | `http://localhost:8080` | vire-portal URL (OAuth mode) |
| `VIRE_MCP_URL` | — | Full MCP endpoint URL with encrypted UID (direct mode, bypasses OAuth) |
| `VIRE_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

Logs are written to `bin/logs/vire-mcp.log` (relative to the binary). The startup log shows the resolved URL and mode:

```
level=INF message="loaded configuration" direct_mode=true mcp_url="http://localhost:8881/mcp/abc123..."
level=INF message="loaded configuration" direct_mode=false portal_url="http://localhost:8881"
```

**Direct Mode (Recommended for Docker/WSL)**

Direct mode uses an encrypted MCP endpoint URL that embeds user identity, bypassing OAuth entirely. Get your unique endpoint URL from the vire-portal dashboard (Profile > MCP Configuration).

```json
{
  "mcpServers": {
    "vire": {
      "command": "/path/to/bin/vire-mcp",
      "env": {
        "VIRE_MCP_URL": "http://localhost:8881/mcp/YOUR_ENCRYPTED_UID",
        "VIRE_LOG_LEVEL": "error"
      }
    }
  }
}
```

**OAuth Mode (Interactive Login)**

OAuth mode runs a browser flow on first launch; tokens are cached in `~/.vire/credentials.json`.

```json
{
  "mcpServers": {
    "vire": {
      "command": "/path/to/bin/vire-mcp",
      "env": {
        "VIRE_PORTAL_URL": "http://localhost:8881"
      }
    }
  }
}
```

**WSL on Windows (Direct Mode)**

When running on Windows with vire-mcp in WSL, use a shell wrapper to pass environment variables (the `env` object isn't passed through `wsl -e`):

```json
{
  "mcpServers": {
    "vire": {
      "command": "wsl",
      "args": [
        "-e",
        "/bin/bash",
        "-c",
        "VIRE_MCP_URL=http://localhost:8881/mcp/YOUR_ENCRYPTED_UID VIRE_LOG_LEVEL=error /home/bobmc/development/vire-portal/bin/vire-mcp"
      ]
    }
  }
}
```

> **Note:** The `env` object in Claude Desktop config doesn't pass variables through `wsl -e`. Use `/bin/bash -c "VAR=val command"` instead.

**Docker (Direct Mode - Ephemeral)**

Direct mode is ideal for Docker: no OAuth, no credential mounts, completely ephemeral:

```json
{
  "mcpServers": {
    "vire": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--network", "host",
        "-e", "VIRE_MCP_URL=http://localhost:8881/mcp/YOUR_ENCRYPTED_UID",
        "-e", "VIRE_LOG_LEVEL=error",
        "ghcr.io/bobmcallan/vire-mcp:latest"
      ]
    }
  }
}
```

**Docker (OAuth Mode - Persistent Credentials)**

OAuth mode requires a volume mount to persist tokens across container runs:

```json
{
  "mcpServers": {
    "vire": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--network", "host",
        "-e", "VIRE_PORTAL_URL=http://localhost:8881",
        "-v", "~/.vire:/root/.vire",
        "ghcr.io/bobmcallan/vire-mcp:latest"
      ]
    }
  }
}
```

> **Note:** For remote portals, replace `localhost:8881` with your portal URL (e.g., `https://portal.vire.app`).

#### Claude Desktop (production via Connectors)

For a deployed portal with HTTPS (e.g. `https://portal.vire.app`), add the server via **Settings > Connectors** in Claude Desktop. Connectors supports HTTPS URLs and handles OAuth natively — no `vire-mcp` bridge needed.

#### MCP OAuth 2.1 Flow

The portal implements OAuth 2.1 (RFC 9728) with PKCE S256 and Dynamic Client Registration. This flow is triggered automatically by `mcp-remote` or Claude Desktop Connectors:

```
1. Client discovers endpoints: GET /.well-known/oauth-authorization-server
2. Client registers: POST /register (Dynamic Client Registration)
3. Client redirects user: GET /authorize?client_id=...&code_challenge=...&state=...
4. Portal creates a pending session, sets mcp_session_id cookie, redirects to /
5. User logs in (email/password or Google/GitHub OAuth)
6. On login success, portal detects mcp_session_id cookie
7. Portal creates auth code, redirects to client's redirect_uri
8. Client exchanges code + code_verifier: POST /token
9. Portal verifies PKCE S256, mints JWT access token (1h) + refresh token (7d)
10. Client sends Authorization: Bearer <jwt> on all subsequent MCP requests
```

#### Authentication Chain

```
Claude (Bearer token or cookie)
  -> POST /mcp
    -> vire-portal: extract user ID from JWT "sub" claim
      -> proxy to vire-server with X-Vire-User-ID header
        -> vire-server: look up user's Navexa key from DB
          -> call Navexa API with user's key
```

The portal extracts user identity from either:
1. `Authorization: Bearer <jwt>` header (Claude Desktop via mcp-remote / Connectors)
2. `vire_session` cookie (web dashboard)

Bearer token takes priority. If neither is present, the request proceeds without user context.

### Tools

Tools are registered dynamically from vire-server's `GET /api/mcp/tools` catalog at startup. The catalog defines each tool's name, description, HTTP method, URL path template, and parameters (path, query, body). The portal builds MCP tool definitions and generic handlers from the catalog entries. See the [vire-server README](https://github.com/bobmcallan/vire) for the full tool catalog.

### X-Vire-* Headers

The proxy injects these headers on every request to vire-server:

| Header | Source | Description |
|--------|--------|-------------|
| `X-Vire-Portfolios` | `VIRE_DEFAULT_PORTFOLIO` env var | Comma-separated portfolio names |
| `X-Vire-Display-Currency` | `VIRE_DISPLAY_CURRENCY` env var | Currency for display values |
| `X-Vire-User-ID` | Session cookie (per-request) | Username from JWT sub claim |

Static headers are set from environment variables on every request. Per-request headers are set when a `vire_session` cookie is present -- the handler decodes the JWT sub claim and injects the user ID. vire-server resolves the user's navexa key internally from the user ID.

## Authentication Flow

The portal authenticates users via vire-server. Three login methods are supported: email/password, Google OAuth, and GitHub OAuth. vire-server handles credential validation and token exchange; the portal never touches passwords or OAuth secrets directly.

### Email/Password Login

```
1. User submits email + password on the landing page
2. Portal forwards to vire-server: POST /api/auth/login { username, password }
3. vire-server validates credentials, returns a signed JWT
4. Portal sets the JWT as an httpOnly "vire_session" cookie
5. User is redirected to /dashboard
```

### OAuth Login (Google / GitHub)

```
1. User clicks "Sign in with Google" (or GitHub) on /
   -> Browser navigates to portal: GET /api/auth/login/google
2. Portal makes a server-side request to vire-server: GET {API_URL}/api/auth/login/google?callback={callbackURL}
3. vire-server returns a 302 redirect to the OAuth provider's consent screen
4. Portal forwards the redirect Location to the browser (never exposing internal API URLs)
5. User authorises, provider redirects back to vire-server
6. vire-server exchanges code for tokens, creates/updates user, mints a JWT
7. vire-server redirects to portal: GET /auth/callback?token=<jwt>
8. Portal sets the JWT as an httpOnly "vire_session" cookie
9. User is redirected to /dashboard
```

The server-side proxy prevents internal Docker addresses (like `http://server:8080`) from being exposed to the browser.

The `callback_url` config setting tells the portal where vire-server should redirect after OAuth completes. This must match the URL registered with each OAuth provider.

### MCP OAuth 2.1 Flow (Claude Desktop)

When Claude Desktop connects, it performs a full OAuth 2.1 flow with PKCE:

```
1. Claude Desktop discovers endpoints: GET /.well-known/oauth-authorization-server
2. Claude Desktop registers: POST /register (Dynamic Client Registration)
3. Claude Desktop redirects user: GET /authorize?client_id=...&code_challenge=...&state=...
4. Portal creates a pending session, sets mcp_session_id cookie, redirects to /
5. User logs in (email/password or OAuth — same flows as above)
6. On login success, portal detects mcp_session_id cookie
7. Portal calls CompleteAuthorization: creates auth code, redirects to Claude Desktop's redirect_uri
8. Claude Desktop exchanges code + code_verifier: POST /token
9. Portal verifies PKCE S256, mints a JWT access token (1h) + refresh token (7d)
10. Claude Desktop sends Bearer token on all subsequent MCP requests
```

### JWT Claims

Tokens minted by the portal OAuth server contain:

| Claim | Description |
|-------|-------------|
| `sub` | User ID (from vire-server) |
| `scope` | Granted scopes (e.g. `openid portfolio:read tools:invoke`) |
| `client_id` | OAuth client ID |
| `iss` | Portal base URL |
| `iat` / `exp` | Issued-at / expiry (1 hour) |

Tokens are HMAC-SHA256 signed using the `auth.jwt_secret` config value.

### OAuth Provider Configuration

| Provider | Scopes |
|----------|--------|
| Google | `openid`, `email`, `profile` |
| GitHub | `read:user`, `user:email` |

## API Contract with vire-server

The portal communicates exclusively with vire-server's REST API. All request/response bodies are JSON.

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

Proxies the OAuth redirect through vire-server. The portal makes a server-side HTTP request to vire-server, which returns a 302 redirect to the OAuth provider. The portal forwards the redirect Location to the browser. This prevents internal Docker addresses from reaching the browser.

| Parameter | Type | Description |
|-----------|------|-------------|
| `:provider` | path | `google` or `github` |

**Response:** 302 redirect to provider OAuth URL (forwarded from vire-server). On error: 302 redirect to `/error?reason=auth_unavailable` or `/error?reason=auth_failed`.

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

The Profile page displays three key fields:

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

## GitHub Actions Workflows

**Test** (`.github/workflows/test.yml`): Runs on PR and push to main. Two jobs: `unit-test` (go vet + go test ./internal/...) and `ui-test` (headless Chrome via `./scripts/ui-test.sh all`). Test artifacts are uploaded on failure.

**Release** (`.github/workflows/release.yml`): Builds and pushes Docker images for vire-portal and vire-mcp to GHCR on push to main or version tags (matrix strategy):

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
│       ├── release.yml              # Docker build + GHCR push (matrix: portal + mcp)
│       └── test.yml                 # CI: go vet, unit tests, UI tests on PR/push
├── cmd/
│   ├── vire-portal/
│   │   └── main.go                  # Portal entry point (flag parsing, config, graceful shutdown)
│   └── vire-mcp/
│       ├── main.go                  # Stdio-to-HTTP bridge (connects to vire-portal)
│       ├── oauth.go                 # OAuth 2.1 browser flow + local callback server
│       └── tokenstore.go            # File-based OAuth token persistence (~/.vire/credentials.json)
├── internal/
│   ├── app/
│   │   └── app.go                   # Dependency container (Config, Logger, Handlers, OAuthServer)
│   ├── auth/
│   │   ├── authorize.go             # GET /authorize handler (PKCE, session tracking, auto-register)
│   │   ├── dcr.go                   # POST /register handler (RFC 7591 Dynamic Client Registration)
│   │   ├── discovery.go             # .well-known OAuth discovery endpoints
│   │   ├── pkce.go                  # PKCE S256 verification (constant-time compare)
│   │   ├── server.go                # OAuthServer (central state, JWT minting, auth completion)
│   │   ├── session.go               # SessionStore for pending MCP auth sessions (TTL 10 min)
│   │   ├── store.go                 # ClientStore, CodeStore, TokenStore (in-memory, mutex-protected)
│   │   └── token.go                 # POST /token handler (auth_code + refresh_token grants)
│   ├── config/
│   │   ├── config.go                # TOML loading with defaults -> file -> env -> CLI priority
│   │   ├── config_test.go
│   │   ├── defaults.go              # Default configuration values
│   │   ├── version.go               # Version info (ldflags + .version file)
│   │   └── version_test.go
│   ├── handlers/
│   │   ├── auth.go                  # OAuth auth handlers (dev login, Google/GitHub redirects, callback, logout, JWT validation)
│   │   ├── auth_test.go             # Auth handler tests (ValidateJWT, IsLoggedIn, OAuth flows)
│   │   ├── auth_integration_test.go # Integration tests (full login round-trip, OAuth chains)
│   │   ├── auth_stress_test.go      # Security stress tests (alg:none attack, tampering, timing, hostile inputs)
│   │   ├── dashboard.go             # GET /dashboard (portfolio management, holdings)
│   │   ├── strategy.go             # GET /strategy (portfolio strategy and plan editors)
│   │   ├── mcp_page.go             # GET /mcp-info (MCP connection config, tools catalog)
│   │   ├── handlers_test.go
│   │   ├── health.go                # GET /api/health
│   │   ├── helpers.go               # WriteJSON, RequireMethod, WriteError
│   │   ├── landing.go               # PageHandler (template rendering + static file serving)
│   │   ├── profile.go               # GET/POST /profile (user info + Navexa API key management)
│   │   └── version.go               # GET /api/version
│   ├── cache/
│   │   ├── cache.go                 # API response cache (TTL, max entries, prefix invalidation)
│   │   └── cache_test.go
│   ├── client/
│   │   ├── vire_client.go           # HTTP client for vire-server user API (GetUser, UpdateUser)
│   │   └── vire_client_test.go
│   ├── mcp/
│   │   ├── catalog.go               # Dynamic tool catalog types, FetchCatalog, BuildMCPTool, GenericToolHandler
│   │   ├── context.go               # UserContext (per-request user identity for proxy headers)
│   │   ├── handler.go               # MCP HTTP handler (Streamable HTTP + JWT auth, catalog fetch at startup)
│   │   ├── handler_test.go          # Tests: withUserContext, extractJWTSub
│   │   ├── handler_stress_test.go   # Stress tests: hostile cookies, concurrent access, binary garbage
│   │   ├── handlers.go              # errorResult helper, resolvePortfolio
│   │   ├── mcp_test.go              # Tests: catalog, validation, tools, handlers, proxy, integration
│   │   ├── proxy.go                 # HTTP proxy to vire-server with X-Vire-* headers
│   │   ├── tools.go                 # RegisterToolsFromCatalog (dynamic registration)
│   │   ├── version.go               # Combined get_version handler (vire_portal + vire_server)
│   │   └── version_test.go          # Version handler tests
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
├── tests/
│   ├── common/                       # Test utilities (browser, config, containers, screenshot)
│   │   ├── browser.go                # chromedp helpers (NewBrowserContext, NavigateAndWait, etc.)
│   │   ├── containers.go             # Docker test env (SurrealDB + vire-server + portal via testcontainers)
│   │   ├── testconfig.go             # Test config loader (TOML, results dir, timestamps)
│   │   └── screenshot.go             # Screenshot capture utility
│   ├── docker/                       # Docker configs for test environment
│   │   ├── docker-compose.yml        # Full test stack (SurrealDB + vire-server + portal)
│   │   ├── Dockerfile.server         # Portal test image (multi-stage build)
│   │   └── portal-test.toml          # Portal test configuration
│   └── ui/                           # UI browser tests
│       ├── test_config.toml          # Test configuration (server URL, browser settings)
│       ├── ui_helpers_test.go        # Test helpers (newBrowser, isVisible, etc.)
│       ├── smoke_test.go             # Smoke tests (landing, dashboard, branding)
│       ├── dashboard_test.go         # Dashboard tests (sections, panels, growth chart, design rules)
│       ├── strategy_test.go         # Strategy page tests (editors, nav, portfolio selector)
│       ├── capital_test.go          # Capital page tests (cash transactions, pagination, summary)
│       ├── nav_test.go               # Navigation tests (hamburger, dropdown, mobile)
│       └── auth_test.go              # Auth tests (Google/GitHub login redirects)
├── pages/
│   ├── dashboard.html                # Dashboard page (portfolio selector, holdings, capital performance, indicators, growth chart, refresh)
│   ├── strategy.html                # Strategy page (portfolio strategy and plan editors)
│   ├── capital.html                  # Capital page (cash transactions ledger, paged table)
│   ├── mcp.html                     # MCP info page (connection details, tools table)
│   ├── landing.html                  # Landing page (Go html/template)
│   ├── profile.html                  # Profile page (user info + Navexa API key management)
│   ├── partials/
│   │   ├── head.html                 # HTML head (IBM Plex Mono, Chart.js CDN, Alpine.js CDN)
│   │   ├── nav.html                  # Navigation bar
│   │   └── footer.html               # Footer
│   └── static/
│       ├── css/
│       │   └── portal.css            # 80s B&W aesthetic (no border-radius, no box-shadow)
│       └── common.js                 # Client logging, Alpine.js init, vireStore (fetch cache/dedup), portfolioDashboard() (growth chart), cashTransactions(), portfolioStrategy()
├── docker/
│   ├── Dockerfile                    # Portal multi-stage build (golang:1.25 -> alpine)
│   ├── Dockerfile.mcp               # MCP stdio binary build (golang:1.25 -> alpine)
│   ├── docker-compose.yml            # Portal + vire-server stack
│   ├── docker-compose.dev.yml        # Dev overlay (VIRE_ENV=dev, used by deploy.sh local)
│   ├── docker-compose.ghcr.yml       # GHCR pull + watchtower auto-update
│   ├── vire-portal.toml              # Portal configuration
│   └── README.md                     # Docker usage documentation
├── docs/
│   ├── requirements.md               # API contracts and architecture
│   ├── architecture-comparison.md
│   └── authentication/
│       └── mcp-oauth-implementation-steps.md  # MCP OAuth implementation plan (Phase 1 complete)
├── scripts/
│   ├── deploy.sh                     # Deploy orchestration (local/ghcr/down/prune)
│   ├── build.sh                      # Docker image builder (--portal, --mcp, or both)
│   ├── run.sh                        # Build + start/stop/restart server locally
│   ├── ui-test.sh                    # UI test runner (smoke, dashboard, nav, auth, all)
│   ├── verify-auth.sh                # Auth endpoint validation (health, login, OAuth, MCP)
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

### Service Stack (portal + vire-server)

The recommended deployment runs portal and vire-server together via docker-compose:

```bash
# Build portal and start services (uses dev compose overlay)
./scripts/deploy.sh local
```

This starts:
- **vire-portal** on port 8881 -- landing page + MCP endpoint (dev mode)
- **vire-server** on port 8882 -- backend API (pulled from GHCR)

Local deploys automatically use `docker-compose.dev.yml` as a compose overlay, which sets `VIRE_ENV=dev` to enable dev mode (dev login, etc.). The base `docker-compose.yml` stays unchanged for prod-like builds.

The portal connects to vire-server via `VIRE_API_URL=http://vire-server:8080` (Docker internal network). Claude Code connects to `http://localhost:8881/mcp` (portal). Claude Desktop uses `vire-mcp` via stdio (see Connecting Claude section above).

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

# Run on host port 8881
docker run -p 8881:8080 \
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

The portal runs alongside vire-server in a Docker Compose stack. `vire-mcp` is an ephemeral stdio-to-HTTP bridge launched by Claude Desktop:

```
     ┌───────────────┐    ┌──────────────────┐    ┌──────────────────┐
     │     User      │    │  Claude Code /    │    │  Claude Desktop  │
     │   (browser)   │    │  Claude CLI       │    │                  │
     └───────┬───────┘    └────────┬──────────┘    └────────┬─────────┘
             │ GET /               │ HTTP + OAuth           │ stdio
             │                     │                        │
             │                     │               ┌────────┴─────────┐
             │                     │               │  vire-mcp        │
             │                     │               │  - stdio ↔ HTTP  │
             │                     │               │  - OAuth 2.1     │
             │                     │               │  - Token cache   │
             │                     │               └────────┬─────────┘
             │                     │                        │ HTTP + OAuth
    ┌────────┴─────────────────────┴────────────────────────┴─────────┐
    │  vire-portal (:8881)                                            │
    │  - Landing page, dashboard, strategy, profile                    │
    │  - MCP endpoint (Streamable HTTP at POST /mcp)                  │
    │  - OAuth 2.1 (DCR, PKCE, token exchange)                        │
    │  - Dynamic tool catalog from vire-server                        │
    │  - X-Vire-* header injection, JWT auth                          │
    └────────────────────────────┬────────────────────────────────────┘
                                 │ REST API proxy
    ┌────────────────────────────┴────────────────────────────────────┐
    │  vire-server (:8080)                                            │
    │  - Portfolio analysis, market data                              │
    │  - Report generation, stock screening                           │
    │  - Strategy and plan management                                 │
    └─────────────────────────────────────────────────────────────────┘
```

All tool logic lives in vire-portal. `vire-mcp` is a pure transport adapter — it does not import `internal/mcp` or contact vire-server directly. The portal implements OAuth 2.1 so both Claude Code and Claude Desktop (via vire-mcp) authenticate users and receive per-user JWT tokens. vire-server resolves the user's API keys (Navexa, EODHD, etc.) from the user ID in the `X-Vire-User-ID` header.

## License

Private repository.
