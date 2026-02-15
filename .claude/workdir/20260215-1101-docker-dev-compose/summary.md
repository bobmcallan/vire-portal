# Summary: Docker Dev Compose Override & Deploy Script Update

**Date:** 2026-02-15
**Status:** Completed

## What Changed

| File | Change |
|------|--------|
| `docker/docker-compose.dev.yml` | New — compose override setting `VIRE_ENV=dev` for vire-portal |
| `scripts/deploy.sh` | Updated `local` mode to use `-f docker-compose.yml -f docker-compose.dev.yml`; added dev compose to smart rebuild check; updated `down` mode |
| `README.md` | Updated deployment docs for dev compose overlay |
| `docs/requirements.md` | Updated deployment section |
| `.claude/skills/develop/SKILL.md` | Updated reference section |

## Tests
- `bash -n scripts/deploy.sh` — syntax valid
- `docker compose -f docker-compose.yml -f docker-compose.dev.yml config` — VIRE_ENV=dev confirmed in merged output
- Docker deployment validated — health endpoint responding, dev mode active

## Documentation Updated
- README.md — local development section updated
- docs/requirements.md — deployment architecture updated
- .claude/skills/develop/SKILL.md — reference updated

## Devils-Advocate Findings
- No critical issues found

## Notes
- The dev overlay only sets `VIRE_ENV=dev` — minimal and clean
- `ghcr` mode is unchanged (production deployment stays in prod mode)
- Env var approach is correct: `VIRE_ENV=dev` overrides both Dockerfile's `ENV VIRE_ENV=prod` and TOML defaults per config priority chain
