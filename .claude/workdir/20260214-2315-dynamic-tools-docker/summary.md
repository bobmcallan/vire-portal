# Summary: Dynamic Tool Registration + Docker Restructure

**Date:** 2026-02-14
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `internal/mcp/catalog.go` | NEW: CatalogTool/CatalogParam types, FetchCatalog (1MB size limit), ValidateCatalogTool, ValidateCatalog (filters invalid + duplicates), BuildMCPTool, GenericToolHandler, resolveParamValue/resolveDefaultValue/resolveDefaultPortfolio (3-tier with API fallback), bodyOrNil |
| `internal/mcp/tools.go` | REWRITTEN: removed 571 lines of 26 hardcoded tools + 6 handler factories, replaced with 15-line RegisterToolsFromCatalog |
| `internal/mcp/handler.go` | UPDATED: startup retry (3 attempts, 2s backoff), calls ValidateCatalog before registration, non-fatal if unreachable |
| `internal/mcp/handlers.go` | KEPT: errorResult helper (still used), resolvePortfolio (used by legacy tests) |
| `internal/mcp/mcp_test.go` | REWRITTEN: 81 tests covering catalog parsing, validation (empty name/method/path, invalid method, path traversal, /api/ prefix, duplicates), size limit, FetchCatalog, BuildMCPTool (all param types), RegisterToolsFromCatalog, GenericToolHandler (all HTTP methods, path/query/body params, encoding, default_from, API fallback, errors), proxy, context, startup retry, integration |
| `docker/Dockerfile` | MOVED from project root to `docker/Dockerfile` |
| `docker/vire-portal.toml` | RENAMED from `docker/portal.toml` |
| `cmd/portal/main.go` | Auto-discovery updated: `vire-portal.toml` first, `portal.toml` fallback |
| `docker/docker-compose.yml` | `dockerfile: docker/Dockerfile`, two-service stack with vire-server |
| `scripts/build.sh` | Added `-f docker/Dockerfile` to docker build commands |
| `scripts/deploy.sh` | Updated Dockerfile path and config file name |
| `scripts/test-scripts.sh` | Updated file paths, added root Dockerfile removal check (131 tests) |
| `.github/workflows/release.yml` | `file: docker/Dockerfile` |
| `.dockerignore` | Updated comment for vire-portal.toml |
| `README.md` | Dynamic tools architecture, retry, validation, 81 tests, docker/Dockerfile, vire-portal.toml |
| `docs/requirements.md` | Updated references for Dockerfile location, config name, dynamic tools |
| `.claude/skills/develop/SKILL.md` | Docker build command, script count, dynamic catalog with retry |
| `docker/README.md` | Updated for Dockerfile location and config name |

## Tests
- 81 tests in `internal/mcp/mcp_test.go` -- all pass
- 8 tests in `internal/config/config_test.go` -- all pass
- All 5 test packages pass: config, handlers, mcp, server, storage/badger
- Race detector: 0 races (`go test -race ./internal/mcp/`)
- Go vet: clean
- Docker build: successful (`docker build -f docker/Dockerfile -t vire-portal:latest .`)
- Script validation: 131/131 pass (`./scripts/test-scripts.sh`)

## Documentation Updated
- `README.md` -- dynamic tool registration, startup retry, catalog validation, test count
- `docs/requirements.md` -- Dockerfile location, config name, dynamic tools
- `.claude/skills/develop/SKILL.md` -- Docker build command, script count, API integration
- `docker/README.md` -- Dockerfile location, config name

## Devils-Advocate Findings
- **MF1: Startup retry** -- Single attempt could fail in Docker Compose where portal starts before vire-server. Fixed: 3 attempts with 2s backoff.
- **MF2: Catalog validation** -- Empty name/method/path would register broken tools. Fixed: ValidateCatalogTool rejects empty fields, validates method whitelist.
- **MF3: Path template validation** -- Malicious catalog could inject paths outside /api/. Fixed: requires /api/ prefix, rejects .. traversal.
- **MF4: API fallback** -- resolveDefaultPortfolio only checked config headers. Fixed: 2-tier resolution (config headers -> GET /api/portfolios/default).
- **MF5: Duplicate tool names** -- Multiple tools with same name would silently overwrite. Fixed: ValidateCatalog skips duplicates with warning.
- **MF6: Catalog size limit** -- Unbounded catalog response could cause OOM. Fixed: 1MB limit on catalog response body.
- **Dead sentinel code** -- catalog.go had leftover debugging code (dummy GetFloat call). Fixed: removed.
- **Dead code in handlers.go** -- Old resolvePortfolio function is only called by legacy tests. Kept for backward compatibility of those tests.

## Notes
- Tools are now fully dynamic -- no hardcoded tool definitions in the portal
- The portal is a generic proxy: tool names, descriptions, HTTP methods, paths, and parameters all come from vire-server's catalog
- 3-tier default portfolio resolution: (1) explicit param, (2) X-Vire-Portfolios header, (3) GET /api/portfolios/default API fallback
- Non-fatal startup: if vire-server is unreachable after 3 retry attempts, portal starts with 0 tools
- Catalog validation prevents path injection and method abuse from a compromised or misconfigured vire-server
- No catalog refresh timer (startup-only) -- can be added later if needed
- Next steps: OAuth 2.1 authentication, user profiles in BadgerDB, catalog refresh timer
