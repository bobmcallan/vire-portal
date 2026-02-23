# Requirements: Self-Contained Test Docker Environment

**Date:** 2026-02-23
**Requested:** Make the test Docker environment completely self-contained in `tests/docker/` with compose file for server and SurrealDB. No dependency on sibling `../vire` repo. Enables isolated testing and GitHub CI workflows.

## Scope
- **In scope:**
  - Create `tests/docker/docker-compose.yml` with SurrealDB + vire-server using published GHCR images
  - Update `tests/common/containers.go` to use pre-built `ghcr.io/bobmcallan/vire-server:latest` instead of building from `../vire` source
  - Remove `buildServerImage()` function (no longer needed)
  - Add GitHub Actions workflow for running UI tests on PR/push
  - Update `scripts/ui-test.sh` to support compose-based mode
  - Update documentation

- **Out of scope:**
  - Changing test logic or test files
  - Changing the portal Dockerfile or build process
  - Changing production Docker setup in `docker/`

## Approach

### 1. `tests/docker/docker-compose.yml` (NEW)
Create a compose file defining the full test stack:
- `surrealdb` service: `surrealdb/surrealdb:v3.0.0`, port 8000, healthcheck
- `vire-server` service: `ghcr.io/bobmcallan/vire-server:latest`, port 8080, depends_on surrealdb, uses `vire-service.toml` config
- `vire-portal` service: builds from project root using `tests/docker/Dockerfile.server`, port 8080, depends_on vire-server

This compose file can be used standalone (`docker compose up`) or by testcontainers.

### 2. `tests/docker/vire-service.toml` (NEW)
Copy the vire-server test config from the sibling repo with storage address pointing to `surrealdb` Docker DNS hostname.

### 3. `tests/common/containers.go` — Refactor
- Remove `buildServerImage()` and `serverBuildOnce`/`serverBuildError` vars
- Change `startTestEnvironment()` to pull `ghcr.io/bobmcallan/vire-server:latest` instead of building from source
- Remove dependency on `VIRE_SERVER_ROOT` env var and `../vire` path
- Keep `buildPortalImage()` as-is (portal still builds from current repo)

### 4. GitHub Actions CI workflow (NEW)
- `.github/workflows/test.yml` — runs on PR and push to main
- Sets up Go, Docker, Chrome
- Runs `go test ./...` and `go vet ./...`
- Runs UI tests via `./scripts/ui-test.sh all`

### 5. Documentation updates
- README.md — update testing section
- `.claude/skills/develop/SKILL.md` — note self-contained test env

## Files Expected to Change
- `tests/docker/docker-compose.yml` — NEW
- `tests/docker/vire-service.toml` — NEW
- `tests/common/containers.go` — refactor to use GHCR image
- `.github/workflows/test.yml` — NEW CI workflow
- `README.md` — update testing docs
