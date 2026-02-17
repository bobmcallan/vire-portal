# Requirements: Docker Isolation, Component Library, Port Defaults

**Date:** 2026-02-15
**Requested:** Three changes: (1) Fix Docker compose killing other containers (2) Implement component library from pages_ref (3) Default all services to port 8080 internally

## Scope
- **In scope:** Docker compose project isolation, deploy script fixes, full CSS/JS component library adoption, port configuration changes, TOML/Dockerfile updates, test updates
- **Out of scope:** Changing vire-server's own code/config (it's a separate repo), adding new pages/routes, restructuring Go handlers

## Task 1: Docker Compose Isolation

**Problem:** `docker-compose.yml` uses `name: vire` as the project name. When `deploy.sh` runs `docker compose up -d --force-recreate --remove-orphans`, Docker Compose treats any container in the "vire" project not in the compose file as an "orphan" and removes it — killing vire-server if it shares the project name.

**Approach:**
1. Change `name: vire` to `name: vire-portal` in `docker/docker-compose.yml` (aligns with ghcr compose which already uses `name: vire-portal`)
2. This isolates vire-portal's project namespace — `--remove-orphans` only affects vire-portal containers

**Files:**
- `docker/docker-compose.yml` — change project name

## Task 2: Component Library Integration

**Problem:** Current `pages/` uses a stripped-down CSS and empty Alpine.js init. The `pages_ref/` directory contains a complete component library with panels, tabs, collapsibles, dropdowns, mobile menu, toasts, badges, grids, sidebar layout, and comprehensive form styling.

**Approach:**
1. Replace `pages/static/css/portal.css` with `pages_ref/portal.css` content, plus vire-portal-specific additions:
   - `[x-cloak]` rule at top
   - `.btn-dev` / `.landing-dev-login` styles (dev login button)
   - `.warning-banner` / `.success-banner` styles
   - Keep `.settings-*` styles but migrate to use `.form-input`/`.form-group` where possible
2. Merge `pages_ref/common.js` Alpine components INTO `pages/static/common.js` — keep existing debug logger, CSRF injector, and add all Alpine.data() registrations (dropdown, mobileMenu, tabs, collapse, toasts, confirm)
3. Update `pages/partials/nav.html` to adopt ref pattern: desktop nav-links + hamburger mobile menu. Adapt for actual routes (Dashboard, Settings/Logout in dropdown). Use `.Page` template var for active state. Keep session-aware conditional, CSRF-protected logout POST form
4. Update `pages/partials/footer.html` to include toast container + GitHub link (from ref)
5. Update `pages/partials/head.html`: load common.js WITHOUT defer (ref loads synchronously before Alpine so Alpine.data registrations are ready). Keep DevMode debug script
6. Update `pages/dashboard.html` to use component library classes (panel-headed for sections, tool-table with table-wrap, badge for tool status)
7. Update `pages/settings.html` to use form-group/form-input/form-label classes from component library
8. No structural changes to `pages/landing.html` (already aligned with ref styling)

**Template data:** Handlers already pass `"Page"` field. The ref nav uses `{{.PageID}}` — adapt nav to use `{{.Page}}`. Add `"PageTitle"` to handler data (simple uppercase of page name shown in nav center).

**Files:**
- `pages/static/css/portal.css` — replace with full component library
- `pages/static/common.js` — merge Alpine components
- `pages/partials/nav.html` — adopt ref nav pattern
- `pages/partials/footer.html` — add toast container
- `pages/partials/head.html` — remove defer from common.js
- `pages/dashboard.html` — use component library classes
- `pages/settings.html` — use form-group/form-input classes
- Handler files (landing.go, dashboard.go, settings.go) — add PageTitle to template data

## Task 3: Port Defaults to 8080

**Problem:** Docker uses port 8500 internally for vire-portal and 4242 for API URL. The Go default is already 8080 but Docker overrides it. User wants all services to default to 8080 internally, external ports unchanged.

**Approach:**
1. `internal/config/defaults.go`: Change API.URL from `http://localhost:4242` to `http://localhost:8080`
2. `docker/docker-compose.yml`: Port mapping `8500:8080`, remove `VIRE_SERVER_PORT=8500` (default is 8080), change `VIRE_API_URL=http://vire-server:8080`, healthcheck to `http://localhost:8080/api/health`
3. `docker/docker-compose.ghcr.yml`: Same port/API changes for portal. vire-server mapping `4242:8080`. vire-mcp mapping stays but uses port 8080 internally if applicable
4. `docker/vire-portal.toml`: Change port from 8500 to 8080, api.url to `http://localhost:8080`
5. `docker/Dockerfile`: EXPOSE 8080
6. `internal/config/config_test.go`: Update `TestNewDefaultConfig_APIDefaults` to expect `http://localhost:8080`

**Files:**
- `internal/config/defaults.go`
- `internal/config/config_test.go`
- `docker/docker-compose.yml`
- `docker/docker-compose.ghcr.yml`
- `docker/vire-portal.toml`
- `docker/Dockerfile`
- `scripts/deploy.sh` — update health check URL in output message if needed

## Files Expected to Change
- `docker/docker-compose.yml`
- `docker/docker-compose.ghcr.yml`
- `docker/vire-portal.toml`
- `docker/Dockerfile`
- `scripts/deploy.sh`
- `internal/config/defaults.go`
- `internal/config/config_test.go`
- `pages/static/css/portal.css`
- `pages/static/common.js`
- `pages/partials/nav.html`
- `pages/partials/footer.html`
- `pages/partials/head.html`
- `pages/dashboard.html`
- `pages/settings.html`
- `internal/handlers/landing.go`
- `internal/handlers/dashboard.go`
- `internal/handlers/settings.go`
- `internal/handlers/handlers_test.go`
