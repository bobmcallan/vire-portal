# Summary: Docker Isolation, Component Library, Port Defaults

**Date:** 2026-02-15
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `docker/docker-compose.yml` | Changed project name `vire` to `vire-portal`, port mapping `8500:8080`, removed `VIRE_SERVER_PORT`, API URL to `http://vire-server:8080`, healthcheck to port 8080 |
| `docker/docker-compose.ghcr.yml` | All services default to internal port 8080, portal `8500:8080`, mcp `4243:8080`, server `4242:8080`, API URLs updated |
| `docker/Dockerfile` | `EXPOSE 8080` (was 8500) |
| `docker/vire-portal.toml` | `port = 8080`, `url = "http://localhost:8080"` |
| `internal/config/defaults.go` | API URL default: `http://localhost:8080` (was `http://localhost:4242`) |
| `internal/config/config_test.go` | Updated `TestNewDefaultConfig_APIDefaults` to expect port 8080 |
| `pages/static/css/portal.css` | Replaced with full component library from `pages_ref/portal.css` (936 lines) plus vire-specific additions (x-cloak, btn-dev, warning/success banners) |
| `pages/static/common.js` | Merged: debug logger + CSRF injector + 6 Alpine.js components (dropdown, mobileMenu, tabs, collapse, toasts, confirm) |
| `pages/partials/nav.html` | Adopted ref pattern: desktop nav-links + dropdown menu + mobile hamburger/slide-out. Uses `.Page` for active state, POST logout |
| `pages/partials/footer.html` | Added toast notification container + GitHub link |
| `pages/partials/head.html` | Removed `defer` from common.js (Alpine.data registrations must run before Alpine init) |
| `pages/dashboard.html` | Updated to use component library panel/table classes |
| `pages/settings.html` | Updated to use form-group/form-input/form-label classes |
| `internal/handlers/landing.go` | Added `PageTitle` to template data |
| `internal/handlers/dashboard.go` | Added `PageTitle` to template data |
| `internal/handlers/settings.go` | Added `PageTitle` to template data |
| `internal/handlers/handlers_test.go` | Tests for PageTitle, component library integration, stress tests |
| `README.md` | Updated port references, Docker instructions |
| `docs/requirements.md` | Updated API integration ports |
| `.claude/skills/develop/SKILL.md` | Updated Configuration table and API Integration section |
| `docker/README.md` | Updated port references |

## Tests
- All Go tests pass (`go test ./...`)
- Go vet clean
- Docker container deployed and healthy on `localhost:8500` (mapped to internal port 8080)
- Config test updated for new API URL default
- Stress tests added for race conditions, XSS in toast notifications, keyboard navigation

## Documentation Updated
- README.md — port references updated
- docs/requirements.md — API integration ports
- .claude/skills/develop/SKILL.md — Configuration table, API Integration section
- docker/README.md — Docker port documentation

## Devils-Advocate Findings
- Race condition identified and fixed
- Toast notification XSS: mitigated by Alpine.js `x-text` (auto-escapes HTML)
- CSRF on mobile logout form: covered by common.js CSRF injector
- All stress tests pass

## Notes
- Docker project name changed from `vire` to `vire-portal` — prevents `--remove-orphans` from killing vire-server
- All services now default to port 8080 internally; external ports unchanged (8500 portal, 4242 server, 4243 mcp)
- Component library CSS is the full `pages_ref/portal.css` plus vire-specific additions
- common.js loads synchronously (no defer) so Alpine.data() registrations are ready before Alpine initializes
- `pages_ref/` directory kept as reference — not deleted
