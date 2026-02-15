# Requirements: Docker Dev Compose Override & Deploy Script Update

**Date:** 2026-02-15
**Requested:** Create docker-compose.dev.yml for local dev deployments with VIRE_ENV=dev; update deploy script to use it

## Scope

### In scope
- Create `docker/docker-compose.dev.yml` as a compose override that sets `VIRE_ENV=dev` for vire-portal
- Add `environment = "dev"` to the TOML config used in dev mode (or keep env var approach)
- Update `scripts/deploy.sh` local mode to use `-f docker-compose.yml -f docker-compose.dev.yml`
- Ensure the TOML `environment` field and `VIRE_ENV` env var are both set to `dev`

### Out of scope
- Changes to ghcr deployment mode
- Changes to the Dockerfile itself
- New routes or handlers

## Approach

### 1. Create `docker/docker-compose.dev.yml`
Docker Compose supports file merging via multiple `-f` flags. The dev file extends/overrides the base:
```yaml
services:
  vire-portal:
    environment:
      - VIRE_ENV=dev
```
This is minimal — it only overrides what's different for dev mode. The base `docker-compose.yml` stays clean for production-like local builds. `VIRE_ENV=dev` env var takes precedence over the TOML and Dockerfile defaults per the config loading priority.

### 2. Update `scripts/deploy.sh`
In the `local` mode section:
- Change all `docker compose -f docker-compose.yml` calls to include `-f docker-compose.dev.yml`
- This applies to: build, up, and the down teardown of local containers
- Add the dev compose file to the smart rebuild check (file modification detection)

### 3. TOML consideration
Since `VIRE_ENV=dev` env var overrides the TOML `environment` field (env vars > TOML in config priority), we don't need a separate dev TOML file. The env var approach is cleaner.

## Files Expected to Change
- `docker/docker-compose.dev.yml` — new file
- `scripts/deploy.sh` — update local mode to use dev compose overlay
