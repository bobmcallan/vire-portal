# Summary: Remove SPA, fully deploy Go architecture

**Date:** 2026-02-14
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `src/` | Deleted (entire Preact SPA source) |
| `node_modules/` | Deleted |
| `dist/` | Deleted |
| `index.html` | Deleted |
| `nginx.conf` | Deleted |
| `package.json` | Deleted |
| `package-lock.json` | Deleted |
| `tsconfig.json` | Deleted |
| `tsconfig.tsbuildinfo` | Deleted |
| `vite.config.ts` | Deleted |
| `eslint.config.js` | Deleted |
| `.env` | Deleted |
| `Dockerfile` (SPA) | Replaced by Go Dockerfile |
| `Dockerfile.portal` | Renamed to `Dockerfile` |
| `docker/docker-compose.go.yml` | Renamed to `docker/docker-compose.yml` (added name, container_name, healthcheck, build args, volumes) |
| `docker/docker-compose.yml` (SPA) | Replaced by Go compose |
| `docker/docker-compose.ghcr.yml` | Rewritten for Go (VIRE_* env vars, portal-data volume, watchtower 1.7.1) |
| `docker/README.md` | Rewritten for Go stack |
| `scripts/deploy.sh` | Removed sync_version, SPA rebuild detection; added Go file detection (*.go, go.mod, go.sum, pages/, docker/portal.toml) |
| `scripts/build.sh` | Removed sync_version, updated header for Go |
| `scripts/test-scripts.sh` | Complete rewrite for Go-only validation (130 tests) |
| `.dockerignore` | Rewritten for Go context (selective docker/ excludes, portal binary exclude) |
| `.gitignore` | Rewritten for Go (removed node_modules, added bin/, /portal, data/, go.work) |
| `README.md` | Rewritten — Go is primary, SPA sections removed |
| `.claude/skills/develop/SKILL.md` | Rewritten for Go stack |
| `docs/requirements.md` | Updated for Go stack |

## Tests
- `go build ./cmd/portal/` — compiles clean
- `go test ./...` — all 4 test packages pass
- `go vet ./...` — clean
- `docker build -t vire-portal:latest .` — builds successfully
- `scripts/test-scripts.sh` — 130 tests, 0 failures
- `scripts/build.sh --help` — exits 0
- `scripts/deploy.sh` usage — exits 1 on invalid mode

## Documentation Updated
- `README.md` — Go-first, SPA content removed
- `docker/README.md` — Go deployment instructions
- `docs/requirements.md` — Go architecture
- `.claude/skills/develop/SKILL.md` — Go conventions, routes, config

## Devils-Advocate Findings
- **sync_version() dead code** — removed entirely (was writing to deleted package.json)
- **Smart rebuild silent failure** — rewritten for Go paths (was checking SPA files)
- **Docker build fails** — .dockerignore blanket `docker/` exclusion blocked portal.toml COPY; fixed with selective excludes
- **Healthcheck path mismatch** — compose/scripts used `/health` but Go serves `/api/health`; all fixed to `/api/health`
- **Missing wget** — Alpine BusyBox includes wget; confirmed working in Docker image
- **portal binary in build context** — added to .dockerignore to reduce context size
- **Dockerfile convention alignment** (ENTRYPOINT vs CMD, alpine pinning, LABEL) — deferred as non-blocking enhancement

## Notes
- docs/architecture-comparison.md intentionally retained with SPA references (comparison document)
- README Pages table lists 6 planned pages; only landing is implemented (scaffold phase)
- Dockerfile convention improvements (ENTRYPOINT, pinned alpine, LABEL) are valid future enhancements
