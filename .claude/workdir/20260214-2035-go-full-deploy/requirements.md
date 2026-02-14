# Requirements: Remove SPA, fully deploy Go architecture

**Date:** 2026-02-14
**Requested:** Go is the correct implementation, fully deploy the Go solution. No reason to maintain backward compat — this has not been deployed yet.

## Scope

### In Scope
- Remove all SPA/Node.js files (src/, index.html, nginx.conf, package.json, package-lock.json, tsconfig.json, vite.config.ts, eslint.config.js, .env, .dockerignore, node_modules/)
- Remove SPA Dockerfile (Dockerfile) — Go uses Dockerfile.portal
- Rename Dockerfile.portal → Dockerfile (now the only Dockerfile)
- Consolidate docker-compose files (remove SPA compose files, keep/update Go compose)
- Update scripts/deploy.sh and scripts/build.sh for Go builds
- Update scripts/test-scripts.sh validation suite
- Update README.md — remove all SPA sections, make Go the primary documentation
- Update .claude/skills/develop skill for Go stack
- Verify go build, go test, Docker build all pass

### Out of Scope
- Implementing new Go pages (dashboard, settings, etc.) — that's a separate task
- Changing the Go architecture or adding features
- CI/CD workflow changes (release.yml will need updating but is a separate concern)

## Approach
Full removal of SPA artifacts. The Go scaffold becomes the sole implementation. Dockerfile.portal becomes the canonical Dockerfile. Docker compose, build/deploy scripts, and documentation are updated to reflect Go-only architecture.

## Files Expected to Change

### Remove
- `src/` (entire directory)
- `index.html`
- `nginx.conf`
- `package.json`
- `package-lock.json`
- `tsconfig.json`
- `vite.config.ts`
- `eslint.config.js`
- `.env`
- `.dockerignore` (review — may need updating for Go)
- `Dockerfile` (SPA)
- `docker/docker-compose.yml` (SPA local)
- `docker/docker-compose.ghcr.yml` (SPA GHCR)

### Rename
- `Dockerfile.portal` → `Dockerfile`

### Update
- `docker/docker-compose.go.yml` → `docker/docker-compose.yml` (now the primary)
- `scripts/deploy.sh` — update for Go builds
- `scripts/build.sh` — update for Go builds
- `scripts/test-scripts.sh` — update validations
- `README.md` — remove SPA sections, Go-first documentation
- `.claude/skills/develop` — update for Go stack
- `.github/workflows/release.yml` — update Dockerfile reference
