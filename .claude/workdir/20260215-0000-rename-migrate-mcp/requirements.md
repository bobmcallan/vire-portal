# Requirements: Rename Binary, Migrate vire-mcp, Add to Docker

**Date:** 2026-02-15
**Requested:** (1) Rename portal binary/cmd to vire-portal, (2) migrate vire-mcp from vire repo into this repo and remove from vire, (3) add vire-mcp to docker build/deploy in vire project namespace.

## Scope

### In Scope
- Rename `cmd/portal/` to `cmd/vire-portal/`, update all references (.gitignore, scripts, tests, docs)
- Copy `cmd/vire-mcp/` from `/home/bobmc/development/vire/` into this repo
- Copy vire's `internal/common/`, `internal/models/`, `internal/interfaces/` into `internal/vire/` (namespaced to avoid conflicts with portal's own internal packages)
- Update all import paths in migrated code from `github.com/bobmcallan/vire/internal/...` to `github.com/bobmcallan/vire-portal/internal/vire/...`
- Add missing Go dependencies to go.mod (phuslu/log, ternarybob/arbor, pelletier/go-toml, spf13/viper)
- Create `docker/Dockerfile.mcp` for building vire-mcp
- Add vire-mcp service to `docker/docker-compose.yml` (project name: `vire`, 3-service stack)
- Copy `docker/vire-mcp.toml` config into this repo
- Update `scripts/build.sh` to build both vire-portal and vire-mcp images
- Update `scripts/deploy.sh` for 3-service stack
- Update `scripts/test-scripts.sh` for new binary name and vire-mcp additions
- Update `.github/workflows/release.yml` to build vire-mcp image (matrix strategy)
- Remove vire-mcp from vire repo: delete `cmd/vire-mcp/`, `docker/Dockerfile.mcp`, `docker/vire-mcp.toml*`, remove from compose files, scripts, CI workflow

### Out of Scope
- Rewriting vire-mcp to use dynamic catalog (it keeps its hardcoded tools and formatters)
- Merging vire-mcp and portal's MCP endpoints into one
- OAuth / multi-user features
- Removing vire's internal/common, models, interfaces (vire-server still uses them)

## Approach

### Change 1: Rename cmd/portal → cmd/vire-portal
- `git mv cmd/portal cmd/vire-portal`
- Update Dockerfile: `-o vire-portal ./cmd/vire-portal`
- Update .gitignore: `/portal` → `/vire-portal`
- Update scripts: `go build ./cmd/portal/` → `go build ./cmd/vire-portal/`
- Update test-scripts.sh references

### Change 2: Migrate vire-mcp
- Copy files from vire repo (not git mv, since vire-server still needs the shared packages)
- Namespace under `internal/vire/` to avoid conflict with portal's `internal/interfaces/`
- Import path rewrite: `github.com/bobmcallan/vire/internal/common` → `github.com/bobmcallan/vire-portal/internal/vire/common` (etc.)
- Add Go dependencies: phuslu/log, ternarybob/arbor, pelletier/go-toml/v2, spf13/viper
- Delete from vire repo: cmd/vire-mcp/, docker/Dockerfile.mcp, docker/vire-mcp.toml*, remove from compose/scripts/CI

### Change 3: Docker build/deploy for vire-mcp
- Create `docker/Dockerfile.mcp` (multi-stage, same pattern as Dockerfile)
- Add vire-mcp to `docker/docker-compose.yml` as third service (port 4243)
- Update build.sh to build both images
- Update deploy.sh for 3-service stack
- Update release.yml with matrix strategy for both images
- Copy vire-mcp.toml config

## Files Expected to Change

### vire-portal repo
- `cmd/portal/` → `cmd/vire-portal/` (rename)
- `cmd/vire-mcp/` — NEW (migrated from vire)
- `internal/vire/common/` — NEW (copied from vire)
- `internal/vire/models/` — NEW (copied from vire)
- `internal/vire/interfaces/` — NEW (copied from vire)
- `docker/Dockerfile` — update build path
- `docker/Dockerfile.mcp` — NEW
- `docker/docker-compose.yml` — add vire-mcp service
- `docker/docker-compose.ghcr.yml` — add vire-mcp service
- `docker/vire-mcp.toml` — NEW
- `scripts/build.sh` — build both images
- `scripts/deploy.sh` — 3-service stack
- `scripts/test-scripts.sh` — new binary name, vire-mcp checks
- `.github/workflows/release.yml` — matrix build
- `.gitignore` — update binary name
- `go.mod` / `go.sum` — new dependencies
- `README.md` — updated structure and docs
- `docker/README.md` — 3-service stack
- `.claude/skills/develop/SKILL.md` — updated references

### vire repo (removals)
- `cmd/vire-mcp/` — DELETE
- `docker/Dockerfile.mcp` — DELETE
- `docker/vire-mcp.toml` — DELETE
- `docker/vire-mcp.toml.docker` — DELETE
- `docker/docker-compose.yml` — remove vire-mcp service
- `docker/docker-compose.ghcr.yml` — remove vire-mcp service
- `scripts/build.sh` — remove vire-mcp build
- `.github/workflows/release.yml` — remove vire-mcp from matrix
