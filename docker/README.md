# Docker Services

## Services

| Service | Description | Port | Image |
|---------|-------------|------|-------|
| vire-portal | Go server (landing page + MCP endpoint) | 4241 | `vire-portal:latest` |
| vire-mcp | Standalone MCP server (25+ tools) | 4243 | `vire-mcp:latest` |
| vire-server | Backend API (portfolios, market data, reports) | 4242 | `ghcr.io/bobmcallan/vire-server:latest` |

The portal serves the landing page and MCP endpoint at `/mcp`. vire-mcp is a standalone MCP server with built-in tool definitions. Both proxy tool calls to vire-server.

## Usage

### Three-Service Stack (recommended)

```bash
# Build and start all services in dev mode (recommended)
./scripts/deploy.sh local

# Force rebuild (no cache)
./scripts/deploy.sh local --force

# View logs
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev.yml logs -f

# Stop all services
./scripts/deploy.sh down
```

Local deploys use `docker-compose.dev.yml` as a compose overlay, which sets `VIRE_ENV=dev` to enable dev mode (dev login, etc.). The base `docker-compose.yml` is unchanged for prod-like builds.

This starts vire-portal on port 4241, vire-mcp on port 4243, and vire-server on port 4242. Claude connects to `http://localhost:4241/mcp` (portal) or `http://localhost:4243/mcp` (standalone mcp).

### Portal Only

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
curl http://localhost:4241/api/health
```

## Deploy Modes

| Mode | Description |
|------|-------------|
| `local` | Build from source and run with dev overlay (`VIRE_ENV=dev`). Smart rebuild detects changes in `*.go`, `go.mod`, `go.sum`, `docker/Dockerfile`, `docker/Dockerfile.mcp`, `docker/docker-compose.dev.yml`, `docker/vire-portal.toml`, `docker/vire-mcp.toml`, `.version`. Use `--force` to bypass. |
| `ghcr` | Pull `ghcr.io/bobmcallan/vire-portal:latest` and run with watchtower auto-update. |
| `down` | Stop all vire-portal containers (both local and GHCR). |
| `prune` | Remove stopped containers, dangling images, and unused volumes. |

## Build Script

```bash
# Build both Docker images with version injection
./scripts/build.sh

# Build only one image
./scripts/build.sh --portal    # Build only vire-portal
./scripts/build.sh --mcp       # Build only vire-mcp

# Options
./scripts/build.sh --verbose   # Show build output
./scripts/build.sh --clean     # Remove existing images first
./scripts/build.sh --help      # Show usage
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VIRE_ENV` | `prod` | Environment: dev \| development, prod \| production |
| `VIRE_SERVER_HOST` | `localhost` | Server bind address |
| `VIRE_SERVER_PORT` | `8080` | Server port |
| `VIRE_API_URL` | `http://localhost:8080` | vire-server URL for MCP proxy |
| `VIRE_PORTAL_URL` | `http://localhost:4241` | Portal URL (required for vire-mcp container) |
| `VIRE_DEFAULT_PORTFOLIO` | `""` | Default portfolio name |
| `VIRE_DISPLAY_CURRENCY` | `""` | Display currency (e.g., AUD, USD) |
| `EODHD_API_KEY` | `""` | EODHD market data API key |
| `NAVEXA_API_KEY` | `""` | Navexa portfolio sync API key |
| `GEMINI_API_KEY` | `""` | Google Gemini AI API key |
| `VIRE_BADGER_PATH` | `./data/vire` | BadgerDB storage path |
| `VIRE_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `VIRE_LOG_FORMAT` | `text` | Log format (text, json) |
| `PORTAL_PORT` | `4241` | Host port mapping for portal (docker-compose only) |
| `MCP_PORT` | `4243` | Host port mapping for MCP server (docker-compose only) |

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

| Volume | Mount | Service | Description |
|--------|-------|---------|-------------|
| `portal-data` | `/app/data` | vire-portal | BadgerDB persistent storage |
| `vire-data` | `/app/data` | vire-server | vire-server data (two-service stack only) |

## GHCR Images

The CI workflow publishes both images to GHCR (matrix strategy):

- `ghcr.io/bobmcallan/vire-portal:latest` / `:<version>` / `:<short-sha>`
- `ghcr.io/bobmcallan/vire-mcp:latest` / `:<version>` / `:<short-sha>`

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
