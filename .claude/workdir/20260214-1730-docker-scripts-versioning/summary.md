# Summary: Docker Directory, Build/Deploy Scripts, and Versioning Strategy

**Date:** 2026-02-14
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `docker/docker-compose.yml` | Local build compose — single service, build args, env vars, healthcheck |
| `docker/docker-compose.ghcr.yml` | GHCR production compose — watchtower with vire-portal scope, pinned image |
| `docker/README.md` | Docker usage documentation with watchtower risk warning |
| `scripts/deploy.sh` | Deployment orchestration — local/ghcr/down/prune modes, smart rebuild, version sync |
| `scripts/build.sh` | Standalone Docker image builder with version injection |
| `scripts/test-scripts.sh` | Validation suite — 94 tests across 11 categories |
| `.gitignore` | Added docker/.last_build |
| `.dockerignore` | Added docker/ and scripts/ to exclude from build context |
| `README.md` | Updated project structure, docker/scripts usage, versioning docs |
| `.claude/skills/develop/SKILL.md` | Updated test count |

## Tests
- Script validation suite: 94/94 passed
- Application tests: 118/118 passed (npm test)
- TypeScript compilation: clean (npm run build)
- ESLint: clean (npm run lint)
- Docker build: successful
- Docker compose config: both files validate

## Documentation Updated
- README.md — project structure, Docker deployment, build scripts, versioning
- docker/README.md — service table, deploy modes, env vars, watchtower warning
- .claude/skills/develop/SKILL.md — test count

## Devils-Advocate Findings
- **sed injection in sync_version** — fixed with safe delimiter
- **Empty version validation** — added exit on empty version
- **Footer after down/prune** — suppressed when containers stopped
- **Watchtower pinned** — explicit version tag instead of :latest
- **Safe env sourcing** — guarded docker/.env loading
- **BUILD_ARGS quoting** — fixed word splitting risk
- **.version in rebuild check** — added to smart rebuild file list
- **docker/README.md watchtower risk** — documented supply chain concern
- docker-compose for single service, build.sh existence, deploy.sh modes — overruled (user explicitly requested these patterns)

## Notes
- Dockerfile remains at project root (referenced by both compose and CI)
- .version is the single source of truth — build scripts sync version to package.json
- CI and scripts use different timestamp formats (compact UTC vs human-readable local) — documented, intentional
- Watchtower scope is `vire-portal` (separate from backend `vire` scope)
- Cloud Run is recommended for production; watchtower/GHCR compose is for self-hosted deployments
