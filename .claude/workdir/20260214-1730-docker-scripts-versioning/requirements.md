# Requirements: Docker Directory, Build/Deploy Scripts, and Versioning Strategy

**Date:** 2026-02-14
**Requested:** Create docker deployment directory, build/deploy scripts, and implement versioning strategy — all modeled after the patterns in `/home/bobmc/development/vire/`.

## Scope

### In Scope
- Create `docker/` directory with docker-compose files for local and GHCR deployments
- Move Docker-related config (Dockerfile, nginx.conf) into `docker/` or reference from there
- Create `scripts/deploy.sh` — orchestration for local builds, GHCR pulls, teardown, and cleanup
- Create `scripts/build.sh` — standalone Docker image builder with version injection
- Enhance the `.version` file strategy so build scripts update the build timestamp
- Ensure CI/CD workflow (`.github/workflows/release.yml`) stays consistent with new structure

### Out of Scope
- Changing the Preact/Vite application code
- Modifying the nginx.conf security headers or routing logic
- Adding new CI/CD workflows beyond updating the existing one
- Backend (vire-server/vire-mcp) changes

## Approach

Mirror the patterns established in `/home/bobmc/development/vire/`:

1. **docker/** directory containing:
   - `docker-compose.yml` — local development (build from source)
   - `docker-compose.ghcr.yml` — production (pull from GHCR with watchtower auto-updates)
   - README.md with usage instructions

2. **scripts/** directory containing:
   - `deploy.sh` — supports `local`, `ghcr`, `down`, `prune` modes; extracts version from `.version`; injects VERSION/BUILD/GIT_COMMIT into docker build
   - `build.sh` — builds Docker image locally with version metadata

3. **Versioning:**
   - `.version` file format: `version: X.Y.Z` + `build: MM-DD-HH-MM-SS`
   - Build scripts update the `build:` timestamp on each build
   - Version injected into Docker build args and available at runtime
   - Dockerfile and CI workflow reference `.version` for metadata

## Files Expected to Change
- `docker/docker-compose.yml` (new)
- `docker/docker-compose.ghcr.yml` (new)
- `docker/README.md` (new)
- `scripts/deploy.sh` (new)
- `scripts/build.sh` (new)
- `.version` (may be updated)
- `.github/workflows/release.yml` (update if Dockerfile path changes)
- `.gitignore` (add any new ignores)
- `README.md` (document new scripts/docker usage)
