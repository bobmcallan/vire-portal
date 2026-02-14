# Docker Services

## Service

| Service | Description | Port | Image |
|---------|-------------|------|-------|
| vire-portal | Go server (html/template + Alpine.js) | 8080 | `vire-portal:latest` |

The portal is a Go binary serving HTML templates with Alpine.js for interactivity, backed by BadgerDB for embedded storage.

## Usage

```bash
# Build and run locally
./scripts/deploy.sh local

# Deploy from GHCR with auto-update
./scripts/deploy.sh ghcr

# Stop all containers
./scripts/deploy.sh down

# View logs
docker logs -f vire-portal

# Health check
curl http://localhost:8080/api/health
```

## Deploy Modes

| Mode | Description |
|------|-------------|
| `local` | Build from source and run. Smart rebuild detects changes in `*.go`, `go.mod`, `go.sum`, `Dockerfile`, `.version`. Use `--force` to bypass. |
| `ghcr` | Pull `ghcr.io/bobmcallan/vire-portal:latest` and run with watchtower auto-update. |
| `down` | Stop all vire-portal containers (both local and GHCR). |
| `prune` | Remove stopped containers, dangling images, and unused volumes. |

## Build Script

```bash
# Build Docker image with version injection
./scripts/build.sh

# Options
./scripts/build.sh --verbose   # Show build output
./scripts/build.sh --clean     # Remove existing images first
./scripts/build.sh --help      # Show usage
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VIRE_SERVER_HOST` | `localhost` | Server bind address |
| `VIRE_SERVER_PORT` | `8080` | Server port |
| `VIRE_BADGER_PATH` | `./data/vire` | BadgerDB storage path |
| `VIRE_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `VIRE_LOG_FORMAT` | `text` | Log format (text, json) |
| `PORTAL_PORT` | `8080` | Host port mapping (docker-compose only) |

## Versioning

The `.version` file at the project root is the single source of truth:

```
version: 0.1.2
build: 02-14-20-27-29
```

- `version:` is the semantic version
- `build:` is the timestamp of the last build, updated automatically
- Both `deploy.sh` and `build.sh` inject VERSION, BUILD, and GIT_COMMIT as Docker build args
- The CI workflow (`release.yml`) uses the same version extraction pattern

## Volumes

| Volume | Mount | Description |
|--------|-------|-------------|
| `portal-data` | `/app/data` | BadgerDB persistent storage |

## GHCR Images

The CI workflow publishes to GHCR:

- `ghcr.io/bobmcallan/vire-portal:latest`
- `ghcr.io/bobmcallan/vire-portal:<version>`
- `ghcr.io/bobmcallan/vire-portal:<short-sha>`

Deploy from GHCR:

```bash
./scripts/deploy.sh ghcr
```

## Watchtower Auto-Update

The `docker-compose.ghcr.yml` includes a watchtower sidecar that polls for new images every 120 seconds. The watchtower scope is `vire-portal` to avoid interfering with other watchtower instances (e.g., the vire backend).

**Supply chain warning:** Watchtower automatically deploys any image pushed to `ghcr.io/bobmcallan/vire-portal:latest`. This means a compromised CI pipeline or force-push to main would auto-deploy to any host running `deploy.sh ghcr`. For production deployments, consider:

- Using pinned version tags instead of `:latest`
- Deploying via Cloud Run + Terraform (the primary production path) which requires explicit `terraform apply`
- Disabling watchtower and using `docker compose pull && docker compose up -d` for manual updates

## Validation

Run the script test suite to verify all docker configs and scripts:

```bash
./scripts/test-scripts.sh
```

This validates file existence, permissions, script behavior, compose syntax, build arg consistency across files, Go build/vet, and version handling.
