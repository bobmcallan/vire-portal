# Task 1: Approach for docker/, scripts/, and versioning

## Summary

Adapt the vire reference patterns (docker-compose files, deploy.sh, build.sh, .version) for vire-portal. The portal is a single static SPA container (nginx), not a multi-service Go backend, so the scripts are simpler.

## Current State

- **Dockerfile** at project root: multi-stage Node 20 builder + nginx 1.27-alpine runtime. Already accepts VERSION/BUILD/GIT_COMMIT build args and injects them as VITE_APP_* env vars.
- **nginx.conf** at project root: SPA routing, /config.json runtime config endpoint, /health check.
- **.version**: `version: 0.1.0`, `build: 02-14-17-30-00`
- **.github/workflows/release.yml**: builds on main push/tags, pushes to `ghcr.io/bobmcallan/vire-portal` with version/sha/latest tags.
- **No** docker/ directory, no scripts/ directory.

## Proposed Files

### 1. `docker/docker-compose.yml` (local build)

Pattern follows vire reference but adapted for single service:

```yaml
name: vire-portal

services:
  vire-portal:
    build:
      context: ..
      dockerfile: Dockerfile
      args:
        VERSION: ${VERSION:-dev}
        BUILD: ${BUILD:-unknown}
        GIT_COMMIT: ${GIT_COMMIT:-unknown}
    image: vire-portal:latest
    container_name: vire-portal
    ports:
      - "${PORTAL_PORT:-8080}:8080"
    environment:
      - API_URL=${API_URL:-http://localhost:4242}
      - DOMAIN=${DOMAIN:-localhost}
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    restart: unless-stopped
```

Key differences from vire reference:
- Single service (no MCP, no volumes for data/logs)
- Dockerfile is at root (context: .., dockerfile: Dockerfile) -- consistent with CI
- Environment vars API_URL and DOMAIN for nginx envsubst (runtime config)
- Port 8080 (nginx)
- No named volumes needed (static SPA, no persistent data)

### 2. `docker/docker-compose.ghcr.yml` (GHCR + watchtower)

```yaml
name: vire-portal

services:
  vire-portal:
    image: ghcr.io/bobmcallan/vire-portal:latest
    pull_policy: always
    container_name: vire-portal
    ports:
      - "${PORTAL_PORT:-8080}:8080"
    environment:
      - API_URL=${API_URL:-http://localhost:4242}
      - DOMAIN=${DOMAIN:-localhost}
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3
    restart: unless-stopped
    labels:
      - "com.centurylinklabs.watchtower.scope=vire-portal"

  watchtower:
    image: containrrr/watchtower
    container_name: vire-portal-watchtower
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DOCKER_API_VERSION=1.44
      - WATCHTOWER_CLEANUP=true
      - WATCHTOWER_POLL_INTERVAL=120
      - WATCHTOWER_SCOPE=vire-portal
    labels:
      - "com.centurylinklabs.watchtower.scope=vire-portal"
    restart: unless-stopped
```

Uses `vire-portal` scope (not `vire`) to avoid conflicting with the backend watchtower if both run on same host.

### 3. `scripts/deploy.sh`

Follows the exact same structure as vire reference deploy.sh with these adaptations:

- **local mode**: Extracts version from .version, updates build timestamp, checks for changes in `src/`, `package.json`, `package-lock.json` (instead of `*.go`, `go.mod`, `go.sum`). Builds via docker-compose.yml. No config file copying (portal has no .toml files).
- **ghcr mode**: Stops local containers, pulls and starts from docker-compose.ghcr.yml.
- **down mode**: Stops both local and ghcr containers.
- **prune mode**: Docker cleanup (containers, images, volumes).
- **--force flag**: Forces rebuild with --no-cache and image removal.
- **Footer**: Shows running containers, logs hint, health URL adapted for portal (port 8080/health).

Smart rebuild check uses `.last_build` sentinel file in docker/ directory, checking `src/`, `package.json`, `package-lock.json`, `index.html`, `vite.config.ts`, `tsconfig.json` for changes.

### 4. `scripts/build.sh`

The vire reference build.sh builds Go binaries to ./bin. For the portal, this script builds a Docker image (there are no standalone binaries for a web frontend). It will:

- Extract version from .version, update build timestamp
- Get git commit hash
- Build Docker image with `docker build` passing VERSION/BUILD/GIT_COMMIT as build-args
- Tag as `vire-portal:latest` and `vire-portal:<version>`
- Support `--verbose`, `--clean` (removes existing images), `--help` flags
- Print image size summary

This is a standalone build script (does not start containers). Useful for CI or manual image builds without docker-compose.

### 5. `.version` file

Keep current format, unchanged:
```
version: 0.1.0
build: 02-14-17-30-00
```

Both deploy.sh and build.sh update the `build:` timestamp on each invocation, matching the vire reference pattern.

### 6. `.gitignore` additions

Add `docker/.last_build` to .gitignore (build sentinel, same as vire reference).

### 7. Dockerfile

Stays at project root. No changes needed -- it already handles VERSION/BUILD/GIT_COMMIT build args correctly.

## Decision: Dockerfile Location

The task description says "Keep Dockerfile at root (referenced by docker-compose and CI)". The docker-compose files use `context: ..` and `dockerfile: Dockerfile` to reference it. CI already uses `file: Dockerfile`. No change needed.

## What Is NOT Needed

- No `.toml` config files in docker/ (portal uses nginx envsubst, not config files)
- No `.toml.docker` template files
- No named volumes (static SPA)
- No Go build logic in build.sh
- No MCP-related configuration
