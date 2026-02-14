# Vire Portal

User-facing web application for the Vire managed service. Sign in, manage API keys, provision your MCP endpoint, and connect Claude to Vire.

The portal is a single-page application served from a Docker container on Cloud Run. It calls the vire-gateway (control plane) REST API for all backend operations. It does **not** call vire-server or vire-mcp directly.

> **Repository layout:** This repo contains the portal frontend code. Portal infrastructure (Cloud Run deployment) is managed by [vire-infra](https://github.com/bobmcallan/vire-infra) Terraform (`infra/modules/portal/`). The Docker image published here (`ghcr.io/bobmcallan/vire-portal:latest`) is consumed by that Terraform module -- the same pattern as `vire`, which publishes `ghcr.io/bobmcallan/vire-server` and `ghcr.io/bobmcallan/vire-mcp` images consumed by vire-infra.

## Pages

| Route | Page | Auth Required | Purpose |
|-------|------|---------------|---------|
| `/` | Landing | No | Product overview, "Sign in with Google" and "Sign in with GitHub" buttons |
| `/auth/callback` | OAuth Callback | No | Receives OAuth redirect, sends auth code to gateway, stores session |
| `/dashboard` | Dashboard | Yes | Usage stats (requests this month, quota bar, daily trend, top endpoints), instance status (running/stopped), plan info |
| `/settings` | Settings | Yes | Profile info, API key management (BYOK), preferences (default portfolio, exchange) |
| `/connect` | Connect | Yes | MCP config generator with copy-to-clipboard for Claude Code and Claude Desktop, URL regeneration |
| `/billing` | Billing | Yes | Plan selection (Free/Pro), Stripe checkout, invoice history via Stripe billing portal |

## Tech Stack

- **TypeScript** with **Preact** (lightweight React-compatible framework, ~3KB gzipped)
- **Vite** for build tooling and dev server
- **No SSR** -- static SPA served via nginx in a Docker container
- **Port 8080** -- required by Cloud Run
- **Tailwind CSS** for styling
- **No Firebase Auth SDK** -- OAuth is handled via direct HTTP redirects and gateway API calls
- **No heavy frameworks** -- no React, no Next.js, no Angular. The portal is 6 pages with simple forms and display components. Preact + Vite keeps the bundle small and the dependency surface minimal.

## Authentication Flow

The portal uses direct OAuth with Google and GitHub. The gateway handles token exchange and issues its own JWTs. No Firebase Auth SDK is involved.

### Sign-in Flow

The sign-in buttons are **anchor tags** (`<a>`) pointing directly at the gateway's login endpoint, not `fetch()` calls. This is necessary because the gateway returns a 302 redirect to the OAuth provider, and you cannot follow cross-origin 302 redirects from `fetch()`.

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

### Frontend Auth Implementation

- Store JWT in a module-scoped variable (not localStorage, not sessionStorage)
- On page load, call `POST /api/auth/refresh` to restore session from httpOnly cookie
- Attach JWT to every API request: `Authorization: Bearer <jwt>`
- Set `credentials: 'include'` on every `fetch()` call (required for httpOnly cookie to be sent cross-origin)
- On 401 response, attempt refresh; if refresh fails, redirect to `/`
- On logout, call `POST /api/auth/logout` (clears httpOnly cookie server-side)

## API Contract with Gateway

> **Note:** The canonical API design is in the [architecture document](https://github.com/bobmcallan/vire-infra/blob/main/docs/architecture-per-user-deployment.md) (Stage 1). This section derives from it and specifies the portal-facing contract -- request/response shapes the portal must handle. If the gateway API evolves, the architecture doc is authoritative.

The portal communicates exclusively with the vire-gateway (control plane) REST API. Base URL is configured via the `API_URL` environment variable (e.g., `https://api.vire.app`).

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

### CORS

The portal at `https://vire.app` calls the gateway at `https://api.vire.app` -- a cross-origin request. The gateway handles CORS headers (`Access-Control-Allow-Origin`, `Access-Control-Allow-Credentials`). The portal must set `credentials: 'include'` on all `fetch()` requests so that the httpOnly refresh token cookie is sent:

```typescript
fetch(`${API_URL}/api/profile`, {
  headers: { 'Authorization': `Bearer ${jwt}` },
  credentials: 'include',  // Required for httpOnly cookie
});
```

Without `credentials: 'include'`, the browser will not send the refresh token cookie, and `POST /api/auth/refresh` will fail.

### Auth Routes (Unauthenticated)

#### `GET /api/auth/login/:provider`

Redirects the browser to the OAuth provider's consent screen. The portal links to this URL directly via anchor tags (`<a href="...">`), not via `fetch()`. The gateway generates a `state` token for CSRF protection, constructs the `redirect_uri` from its domain config, and returns a 302 redirect.

| Parameter | Type | Description |
|-----------|------|-------------|
| `:provider` | path | `google` or `github` |

**Response:** 302 redirect to provider OAuth URL with `client_id`, `redirect_uri`, `scope`, `state` params.

**Portal usage:** `<a href="${API_URL}/api/auth/login/google">Sign in with Google</a>` -- full page navigation.

#### `POST /api/auth/callback`

Exchanges an OAuth authorization code for a session. The `state` parameter is generated by the gateway during `GET /api/auth/login/:provider` and passed through the OAuth flow. The portal extracts it from the callback URL query params and sends it back for CSRF validation -- the portal does not generate or verify state itself.

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

Also sets `refresh_token` as httpOnly, Secure, SameSite=Lax cookie. (SameSite=Lax is required because the callback is a cross-site redirect from the OAuth provider; SameSite=Strict would block the cookie on the redirect.)

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

Frontend redirects the browser to `checkout_url` via `window.location.href = checkout_url`. Stripe handles payment. On completion, Stripe redirects back to `https://${DOMAIN}/billing?session_id={CHECKOUT_SESSION_ID}` -- the gateway configures this return URL when creating the session (the portal does not pass a return URL). The gateway receives a Stripe webhook to update the user's plan. The billing page should check for a `session_id` query param on load and display a success/pending message.

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
- **Configured** -- solid border, `[OK] ••••abcd` showing last 4 characters, "UPDATE" and "DELETE" buttons
- **Invalid** -- solid border, `[ERR]` label with error message from validation

### Security

- Keys are encrypted with Cloud KMS and stored in GCP Secret Manager
- Keys are never stored in Firestore (profile holds only a reference)
- Keys are never logged or shared
- After entry, only the last 4 characters are visible
- Each key is injected directly into the user's dedicated MCP proxy as a Cloud Run secret reference

## Environment Variables

The portal Docker container receives two environment variables from Terraform:

| Variable | Description | Example |
|----------|-------------|---------|
| `API_URL` | Gateway (control plane) base URL | `https://api.vire.app` |
| `DOMAIN` | Portal domain name | `vire.app` |

### Runtime Injection

Since the portal is a static SPA, environment variables cannot be read at runtime by client-side JavaScript. Use one of these approaches:

**Option A -- nginx template substitution (recommended):**

Create a template file (`/etc/nginx/templates/default.conf.template`) that nginx's `envsubst` processes at container startup. Serve a `/config.json` endpoint that the SPA fetches on load:

```nginx
location /config.json {
    default_type application/json;
    return 200 '{"apiUrl":"${API_URL}","domain":"${DOMAIN}"}';
}
```

The SPA fetches `/config.json` on initialization and uses the values for all API calls. This approach avoids modifying built assets and works with content hashing.

## Dockerfile

Multi-stage build matching the vire ecosystem pattern. Stage 1 builds the SPA; stage 2 serves it via nginx.

```dockerfile
# Build stage
FROM node:20-alpine AS builder

WORKDIR /build

# Version build arguments
ARG VERSION=dev
ARG BUILD=unknown
ARG GIT_COMMIT=unknown

# Copy package files first for better caching
COPY package.json package-lock.json ./
RUN npm ci

# Copy source code
COPY . .

# Inject version info into the build
ENV VITE_APP_VERSION=${VERSION}
ENV VITE_APP_BUILD=${BUILD}
ENV VITE_APP_COMMIT=${GIT_COMMIT}

# Build static assets
RUN npm run build

# Runtime stage
FROM nginx:1.27-alpine

LABEL org.opencontainers.image.source="https://github.com/bobmcallan/vire-portal"

# Copy built assets from builder
COPY --from=builder /build/dist /usr/share/nginx/html

# Copy nginx config with env substitution template
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Copy version file
COPY .version /usr/share/nginx/html/.version

# nginx:alpine uses envsubst on templates in /etc/nginx/templates/
# and writes output to /etc/nginx/conf.d/ at startup

EXPOSE 8080

# Restrict envsubst to only API_URL and DOMAIN.
# Without this, envsubst replaces nginx's own $uri, $request_uri, etc. with
# empty strings, breaking SPA routing and proxy directives.
ENV NGINX_ENVSUBST_FILTER="API_URL|DOMAIN"

# nginx:alpine default entrypoint handles template substitution
# No custom entrypoint needed
```

### nginx.conf

The nginx config is placed at `nginx.conf` in the repo root and copied into the image as an envsubst template. The `NGINX_ENVSUBST_FILTER` env var in the Dockerfile restricts substitution to `API_URL` and `DOMAIN` only -- without this, nginx's own variables (`$uri`, `$request_uri`) would be replaced with empty strings, breaking routing.

```nginx
server {
    listen 8080;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # Security headers
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self' ${API_URL}; frame-ancestors 'none';" always;
    add_header Permissions-Policy "camera=(), microphone=(), geolocation=()" always;

    # SPA routing -- serve index.html for all non-file routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Runtime config endpoint (env vars injected by nginx envsubst)
    location /config.json {
        default_type application/json;
        return 200 '{"apiUrl":"${API_URL}","domain":"${DOMAIN}"}';
    }

    # Cache static assets aggressively
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Health check for Cloud Run
    location /health {
        access_log off;
        return 200 'ok';
        add_header Content-Type text/plain;
    }
}
```

## GitHub Actions Workflow

Create `.github/workflows/release.yml` matching the vire repo's pattern:

```yaml
name: Release

on:
  push:
    branches:
      - main
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: "Image tag (e.g. latest, v1.2.3)"
        required: false
        default: "latest"

env:
  REGISTRY: ghcr.io

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract version metadata
        id: meta
        run: |
          VERSION=$(grep '^version:' .version | awk '{print $2}')
          echo "version=${VERSION}" >> "$GITHUB_OUTPUT"
          echo "build=$(date -u +%Y%m%d%H%M%S)" >> "$GITHUB_OUTPUT"
          echo "commit=${GITHUB_SHA::8}" >> "$GITHUB_OUTPUT"

      - name: Docker metadata
        id: docker-meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ github.repository_owner }}/vire-portal
          tags: |
            type=sha,prefix=,format=short
            type=raw,value=${{ steps.meta.outputs.version }}
            type=raw,value=latest,enable=${{ github.ref == 'refs/heads/main' }}

      - name: Build and push
        uses: docker/build-push-action@v6
        with:
          context: .
          file: Dockerfile
          push: true
          tags: ${{ steps.docker-meta.outputs.tags }}
          labels: ${{ steps.docker-meta.outputs.labels }}
          build-args: |
            VERSION=${{ steps.meta.outputs.version }}
            BUILD=${{ steps.meta.outputs.build }}
            GIT_COMMIT=${{ steps.meta.outputs.commit }}
          cache-from: type=gha,scope=vire-portal
          cache-to: type=gha,mode=max,scope=vire-portal
```

### Differences from vire's release.yml

| Aspect | vire | vire-portal |
|--------|------|-------------|
| Matrix strategy | Yes (2 services: server, mcp) | No (single image) |
| Dockerfile path | `docker/Dockerfile.server`, `docker/Dockerfile.mcp` | `Dockerfile` (root) |
| Image name | `vire-server`, `vire-mcp` | `vire-portal` |
| Cache scope | Per service (`vire-server`, `vire-mcp`) | Single (`vire-portal`) |

Everything else is identical: triggers, GHCR registry, buildx, login, version extraction from `.version`, docker metadata tags (sha, version, latest), build-args (VERSION, BUILD, GIT_COMMIT), GHA caching.

## .version File

Create `.version` in the repository root:

```
version: 0.1.0
build: 02-14-16-01-19
```

Format matches vire's `.version` file. The `version` field is the single source of truth -- build scripts (`deploy.sh`, `build.sh`) sync it to `package.json` on each run. The `build` field is updated by build scripts and CI.

## Project Structure

```
vire-portal/
├── .github/
│   └── workflows/
│       └── release.yml        # Docker build + GHCR push
├── docker/
│   ├── docker-compose.yml     # Local build + run
│   ├── docker-compose.ghcr.yml # GHCR pull + watchtower auto-update
│   └── README.md              # Docker usage documentation
├── scripts/
│   ├── deploy.sh              # Deploy orchestration (local/ghcr/down/prune)
│   ├── build.sh               # Standalone Docker image builder
│   └── test-scripts.sh        # Validation suite for scripts and configs
├── src/
│   ├── main.tsx               # Entry point -- app component, routing, auth state
│   ├── api.ts                 # Gateway API client (fetch wrapper with JWT, 401 retry)
│   ├── auth.ts                # JWT management, OAuth helpers, token refresh
│   ├── router.ts              # Route definitions and auth guard helpers
│   ├── state.ts               # Pub/sub app state (user, jwt, config)
│   ├── types.ts               # All TypeScript interfaces
│   ├── vite-env.d.ts          # Vite/ImportMeta type declarations
│   ├── test-setup.ts          # Test setup (cleanup, mock restore)
│   ├── __tests__/             # 13 test files, 118 tests
│   │   ├── state.test.ts
│   │   ├── auth.test.ts
│   │   ├── api.test.ts
│   │   ├── landing.test.tsx
│   │   ├── callback.test.tsx
│   │   ├── dashboard.test.tsx
│   │   ├── settings.test.tsx
│   │   ├── connect.test.tsx
│   │   ├── billing.test.tsx
│   │   ├── layout.test.tsx
│   │   ├── key-input.test.tsx
│   │   ├── copy-block.test.tsx
│   │   └── usage-chart.test.tsx
│   ├── pages/
│   │   ├── landing.tsx        # / -- product info + sign-in buttons
│   │   ├── callback.tsx       # /auth/callback -- OAuth callback handler
│   │   ├── dashboard.tsx      # /dashboard -- usage stats, instance status
│   │   ├── settings.tsx       # /settings -- profile, API keys, preferences
│   │   ├── connect.tsx        # /connect -- MCP config + copy button
│   │   └── billing.tsx        # /billing -- plan selection, Stripe checkout
│   ├── components/
│   │   ├── layout.tsx         # Page shell (nav, skip-to-content, footer)
│   │   ├── key-input.tsx      # API key input with B&W status indicators
│   │   ├── usage-chart.tsx    # Quota progressbar and daily trend bars
│   │   └── copy-block.tsx     # Code block with copy-to-clipboard button
│   └── styles/
│       └── main.css           # Tailwind v4 theme, B&W global styles
├── index.html                 # SPA shell (in repo root, not public/)
├── nginx.conf                 # nginx config template (envsubst, security headers)
├── Dockerfile                 # Multi-stage build (node builder + nginx runtime)
├── .dockerignore              # Excludes node_modules, dist, .git from build context
├── .version                   # Version metadata (source of truth, synced to package.json)
├── .env                       # Local dev env vars (VITE_API_URL)
├── package.json
├── package-lock.json
├── tsconfig.json
├── vite.config.ts             # Vite + Preact + Tailwind v4 config
├── eslint.config.js           # ESLint 9 flat config with TypeScript
├── .gitignore
├── LICENSE
└── README.md
```

## Go Server (Scaffold)

The repository now includes a Go-based server scaffold alongside the existing SPA. This is the foundation for migrating to a server-rendered architecture following the [Quaero project](https://github.com/ternarybob/quaero) patterns.

### Go Project Structure

```
cmd/portal/main.go              # Entry point (flag parsing, config, graceful shutdown)
internal/
  app/app.go                    # Dependency container (Config, Logger, StorageManager, Handlers)
  config/
    config.go                   # TOML loading with defaults -> file -> env -> CLI priority
    defaults.go                 # Default configuration values
    version.go                  # Version info (ldflags + .version file)
  handlers/
    landing.go                  # PageHandler (template rendering + static file serving)
    health.go                   # GET /api/health
    version.go                  # GET /api/version
    helpers.go                  # WriteJSON, RequireMethod, WriteError
  interfaces/storage.go         # StorageManager + KeyValueStorage interfaces
  models/
    user.go                     # User model
    session.go                  # Session model
  server/
    server.go                   # HTTP server (net/http, timeouts, graceful shutdown)
    routes.go                   # Route registration
    middleware.go               # Correlation ID, logging, CORS, recovery
    route_helpers.go            # RouteByMethod, RouteResourceCollection
  storage/
    factory.go                  # Storage factory (creates BadgerDB manager)
    badger/
      connection.go             # BadgerDB connection via badgerhold
      manager.go                # StorageManager implementation
      kv_storage.go             # KeyValueStorage implementation
pages/
  landing.html                  # Landing page (Go html/template)
  partials/
    head.html                   # HTML head (IBM Plex Mono, Alpine.js CDN)
    nav.html                    # Navigation bar
    footer.html                 # Footer
  static/
    css/portal.css              # 80s B&W aesthetic (no border-radius, no box-shadow)
    common.js                   # Alpine.js component skeleton
portal.toml                     # Configuration file
Dockerfile.portal               # Multi-stage Go build (golang:1.25 -> alpine)
docker/docker-compose.go.yml    # Docker Compose for Go server
```

### Go Prerequisites

- Go 1.25+

### Go Development

```bash
# Build the server binary
go build ./cmd/portal/

# Run the server (auto-discovers portal.toml)
go run ./cmd/portal/

# Run with custom port
go run ./cmd/portal/ -p 9090

# Run with custom config
go run ./cmd/portal/ -c custom.toml

# Run all tests (52 tests)
go test ./...

# Run tests verbose
go test -v ./...

# Vet for issues
go vet ./...
```

The Go server runs on `http://localhost:8080` by default.

### Go Routes

| Route | Handler | Description |
|-------|---------|-------------|
| `GET /` | PageHandler | Landing page (server-rendered HTML template) |
| `GET /static/*` | PageHandler | Static files (CSS, JS) |
| `GET /api/health` | HealthHandler | Health check (`{"status":"ok"}`) |
| `GET /api/version` | VersionHandler | Version info (JSON) |

### Go Configuration

Configuration priority (highest wins): CLI flags > environment variables > TOML file > defaults.

| Setting | TOML Key | Environment Variable | Default |
|---------|----------|---------------------|---------|
| Server port | `server.port` | `VIRE_SERVER_PORT` | `8080` |
| Server host | `server.host` | `VIRE_SERVER_HOST` | `localhost` |
| BadgerDB path | `storage.badger.path` | `VIRE_BADGER_PATH` | `./data/vire` |
| Log level | `logging.level` | `VIRE_LOG_LEVEL` | `info` |
| Log format | `logging.format` | `VIRE_LOG_FORMAT` | `text` |

### Go Docker

```bash
# Build Go Docker image
docker build -f Dockerfile.portal -t vire-portal-go:latest \
  --build-arg VERSION=$(grep '^version:' .version | awk '{print $2}') \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) .

# Run Go Docker container
docker run -p 8080:8080 \
  -e VIRE_SERVER_HOST=0.0.0.0 \
  vire-portal-go:latest

# Docker Compose
docker compose -f docker/docker-compose.go.yml up
```

### Data Layer (Interface Pattern)

The storage layer uses interfaces so BadgerDB can be swapped for a centralised database later:

```go
type StorageManager interface {
    KeyValueStorage() KeyValueStorage
    DB() interface{}
    Close() error
}

type KeyValueStorage interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key, value string) error
    Delete(ctx context.Context, key string) error
    GetAll(ctx context.Context) (map[string]string, error)
}
```

To switch from BadgerDB to PostgreSQL or another database, implement these interfaces and update `internal/storage/factory.go`.

### SPA and Go Coexistence

Both the SPA and Go scaffold coexist in this repository:
- SPA code: `src/`, `index.html`, `vite.config.ts`, `package.json`, `Dockerfile`
- Go code: `cmd/`, `internal/`, `pages/`, `portal.toml`, `Dockerfile.portal`
- Both build and test independently (`npm test` and `go test ./...`)

## Development (SPA)

### Prerequisites

- Node.js >= 20

### Setup

```bash
# Install dependencies
npm install

# Start dev server (with hot reload)
npm run dev
```

The dev server runs on `http://localhost:5173` by default (Vite).

### Local Environment

Create a `.env` file for local development pointing at the gateway:

```
# Points at a locally running vire-gateway (default port from control plane)
VITE_API_URL=http://localhost:8080
VITE_DOMAIN=localhost
```

Note: In local dev (Vite), env vars use the `VITE_` prefix so they are exposed to client-side code. In the Docker container, the non-prefixed `API_URL` and `DOMAIN` are injected via nginx envsubst (see [Runtime Injection](#runtime-injection)).

For development against the deployed dev environment:

```
VITE_API_URL=https://vire-gateway-dev-xxxx.a.run.app
VITE_DOMAIN=dev.vire.app
```

### Testing

```bash
# Run all tests (118 tests across 13 test files)
npm test

# Run tests in watch mode
npm run test:watch
```

### Linting

```bash
# ESLint with TypeScript rules
npm run lint
```

### Build

```bash
# Production build (TypeScript check + Vite build, outputs to dist/)
npm run build

# Preview production build locally
npm run preview
```

### Docker (local)

The project includes deployment scripts matching the [vire](https://github.com/bobmcallan/vire) project patterns. See `docker/README.md` for full details.

```bash
# Build and run locally (smart rebuild, version injection)
./scripts/deploy.sh local

# Build and run with forced rebuild (no cache)
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
docker build -t vire-portal:latest .

# Run on host port 3000 (portal listens on 8080 inside the container)
# API_URL points at the gateway on a different port, not the portal itself
docker run -p 3000:8080 \
  -e API_URL=http://host.docker.internal:8080 \
  -e DOMAIN=localhost \
  vire-portal:latest
```

The portal is then available at `http://localhost:3000`. `API_URL` must point at the gateway (control plane), not the portal. The example assumes the gateway runs on host port 8080. Use `host.docker.internal` to reach host services from inside the container.

### Environment Defaults

For local Docker deployments, create `docker/.env` with defaults:

```
API_URL=http://host.docker.internal:8080
DOMAIN=localhost
PORTAL_PORT=8080
```

The `deploy.sh local` mode sources this file automatically.

## Cloud Run Deployment

The portal runs on Cloud Run, deployed via vire-infra Terraform. The Terraform module is at `infra/modules/portal/main.tf` in the vire-infra repo.

### Configuration from Terraform

```hcl
# Simplified from vire-infra/infra/modules/portal/main.tf
# The actual module accepts var.image with a placeholder fallback:
#   image = var.image != "" ? var.image : "gcr.io/cloudrun/placeholder"
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
      env { name = "DOMAIN";  value = var.domain }
      env { name = "API_URL"; value = var.gateway_url }
      resources {
        limits = { cpu = "1", memory = "256Mi" }
      }
    }
  }
}
```

Key properties:
- **Image:** `ghcr.io/bobmcallan/vire-portal:latest` (published by this repo's GitHub Actions; the Terraform module accepts `var.image` override)
- **Port:** 8080
- **Ingress:** All (public website)
- **Auth:** Unauthenticated (public access via IAM allUsers)
- **Scaling:** 0-3 instances (scales to zero when idle)
- **Resources:** 1 CPU, 256Mi memory

### Environment Variables (set by Terraform)

| Variable | Source | Example Value |
|----------|--------|---------------|
| `DOMAIN` | `var.domain` | `vire.app` |
| `API_URL` | `var.gateway_url` | `https://vire-gateway-xxxx.a.run.app` |

## Releasing

Push a version tag to trigger the GitHub Actions workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds and pushes `ghcr.io/bobmcallan/vire-portal` with the version tag and `:latest` to GHCR.

Pushing to `main` also triggers a build with the `:latest` tag. You can trigger a build manually from the Actions tab using "Run workflow".

The vire-infra Terraform references `ghcr.io/bobmcallan/vire-portal:latest`. After a new image is pushed, re-apply Terraform or update the Cloud Run service to pull the new image.

## Architecture Context

The portal is one component of the Vire multi-user cloud architecture:

```
     ┌───────┐
     │ User  │
     └───┬───┘
         │ browser
         │
    ┌────┴─────────────────────┐
    │  vire-portal (this repo) │   <- Static SPA on Cloud Run
    │  vire.app                │
    └────┬─────────────────────┘
         │ REST API (JWT auth)
         │
    ┌────┴─────────────────────┐
    │  vire-gateway            │   <- Control plane (Go, Cloud Run)
    │  api.vire.app            │      OAuth, Firestore, Secret Manager,
    │  (vire-infra repo)       │      Cloud Run provisioning
    └──────────────────────────┘
```

The portal does **not** interact with:
- `vire-server` (shared backend API -- accessed only by MCP proxies)
- `vire-mcp` (per-user MCP proxy -- accessed only by Claude)
- Firestore, Secret Manager, or GCS directly (all accessed via the gateway API)

The portal is a pure API consumer. Every operation (sign-in, key management, provisioning, usage stats, billing) goes through the gateway REST API.

## License

Private repository.
