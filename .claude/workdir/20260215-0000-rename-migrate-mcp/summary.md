# Summary: Rename Binary, Migrate vire-mcp, Add to Docker

**Date:** 2026-02-15
**Status:** Completed

## What Changed

### vire-portal repo

| File | Change |
|------|--------|
| `cmd/portal/` → `cmd/vire-portal/` | RENAMED: portal binary entry point |
| `cmd/vire-mcp/` | NEW: migrated from vire repo (main.go, proxy.go, handlers.go, formatters.go, tools.go, tests) |
| `internal/vire/common/` | NEW: copied from vire repo, import paths rewritten |
| `internal/vire/models/` | NEW: copied from vire repo, import paths rewritten |
| `internal/vire/interfaces/` | NEW: copied from vire repo, import paths rewritten |
| `docker/Dockerfile` | UPDATED: build path `./cmd/vire-portal` |
| `docker/Dockerfile.mcp` | NEW: multi-stage build for vire-mcp, Alpine 3.21, port 4243 |
| `docker/docker-compose.yml` | UPDATED: 3-service stack (portal + mcp + vire-server), project name `vire` |
| `docker/docker-compose.ghcr.yml` | UPDATED: added vire-mcp service with watchtower |
| `docker/vire-mcp.toml` | NEW: MCP config for local use |
| `docker/vire-mcp.toml.docker` | NEW: MCP config for Docker/CI |
| `scripts/build.sh` | REWRITTEN: builds both images, --portal/--mcp flags, build_image helper |
| `scripts/deploy.sh` | UPDATED: 3-service stack, smart rebuild checks both Dockerfiles/configs |
| `scripts/test-scripts.sh` | UPDATED: vire-mcp checks, cmd/vire-portal references, 151 tests |
| `.github/workflows/release.yml` | UPDATED: matrix strategy for vire-portal + vire-mcp |
| `.gitignore` | UPDATED: /vire-portal, /vire-mcp binaries |
| `.dockerignore` | UPDATED: vire-portal, vire-mcp binaries |
| `go.mod` / `go.sum` | UPDATED: Go 1.25.5, added phuslu/log, ternarybob/arbor, spf13/viper deps |
| `README.md` | UPDATED: project structure, cmd/vire-portal, vire-mcp, Docker docs |
| `docs/requirements.md` | UPDATED: cmd/vire-portal references |
| `.claude/skills/develop/SKILL.md` | UPDATED: build command, entry point reference |

### vire repo (removals)

| File | Change |
|------|--------|
| `cmd/vire-mcp/` | DELETED: entire directory |
| `docker/Dockerfile.mcp` | DELETED |
| `docker/vire-mcp.toml` | DELETED |
| `docker/vire-mcp.toml.docker` | DELETED |
| `docker/docker-compose.yml` | UPDATED: removed vire-mcp service |
| `docker/docker-compose.ghcr.yml` | UPDATED: removed vire-mcp service |
| `scripts/build.sh` | UPDATED: removed vire-mcp build |
| `.github/workflows/release.yml` | UPDATED: removed vire-mcp from matrix, single vire-server build |
| `docker/README.md` | UPDATED: removed vire-mcp docs, added migration note |

## Tests

- 14 Go packages tested, all pass (`go test ./...`)
- `go vet ./...` clean
- Both binaries build: `go build ./cmd/vire-portal/` and `go build ./cmd/vire-mcp/`
- Portal Docker image builds: `docker build -f docker/Dockerfile -t vire-portal:latest .`
- MCP Docker image builds: `docker build -f docker/Dockerfile.mcp -t vire-mcp:latest .`
- Script validation: 151/151 pass (`./scripts/test-scripts.sh`)
- Vire repo still builds: `go build ./cmd/vire-server/`

## Documentation Updated

- `README.md` -- project structure tree, build commands, Docker docs
- `docs/requirements.md` -- cmd/vire-portal references
- `.claude/skills/develop/SKILL.md` -- build command, entry point
- `docker/README.md` (vire repo) -- removed vire-mcp, added migration note

## Devils-Advocate Findings

- **Excessive code copying**: 4,035 lines across 3 internal packages. Accepted -- Go's internal package rules require it for cross-module imports.
- **Viper dependency**: Eliminated per DA recommendation. Config rewritten to use pelletier/go-toml/v2 (already in go.mod).
- **Go version mismatch**: Bumped to 1.25.5 for arbor compatibility.
- **Namespace collision**: internal/vire/ avoids conflict with portal's internal/interfaces/.
- **Divergence risk**: Source commit headers added. Accepted trade-off for clean module boundary.

## Architecture

```
vire-portal repo now contains two binaries:

1. vire-portal (:8080) -- Web portal + dynamic MCP endpoint
   - Landing page, health/version API
   - Dynamic MCP tools from catalog (GET /api/mcp/tools)
   - Proxies to vire-server

2. vire-mcp (:4243) -- Standalone MCP server
   - 25+ hardcoded tools with formatted responses
   - Stdio + HTTP transport
   - Proxies to vire-server

Docker stack: 3 services
  vire-portal:8080 ──┐
  vire-mcp:4243    ──┼──> vire-server:4242
                     │
```

## Notes

- vire-mcp was migrated as-is (no rewrite). Hardcoded tools and formatters preserved.
- Viper was eliminated from vire-mcp config, replaced with go-toml/v2 + manual env overrides.
- The two MCP endpoints serve different purposes: portal's is dynamic (catalog-driven, raw JSON), vire-mcp's is static (hardcoded tools, formatted responses).
- Future work: consider merging the two MCP approaches, or deprecating vire-mcp once portal's dynamic catalog is mature.
