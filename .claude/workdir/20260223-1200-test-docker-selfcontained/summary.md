# Summary: Self-Contained Test Docker Environment

**Date:** 2026-02-23
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `tests/docker/docker-compose.yml` | NEW — Full test stack (SurrealDB + vire-server + portal) using GHCR images, follows vire-stack.yml pattern |
| `tests/common/containers.go` | Removed `buildServerImage()`, use GHCR image `ghcr.io/bobmcallan/vire-server:latest`, explicit container names with `-tc` suffix to avoid dev stack conflicts |
| `.github/workflows/test.yml` | NEW — CI workflow runs unit tests and UI tests on PR/push to main |
| `README.md` | Updated testing section with self-contained env docs, compose usage, CI workflow |

## Key Design Decisions
- **Container names**: `-test` suffix for compose, `-tc` suffix for testcontainers — avoids conflicts with dev stack (`vire-db`, `vire-server`, `vire-portal`)
- **Port 8882**: Test portal exposed on 8882 (compose) to avoid conflict with dev stack's 8881
- **SurrealDB not exposed**: Matches vire-stack.yml — DB only accessible via server on internal network
- **GHCR images**: No dependency on sibling `../vire` repo — vire-server pulled from ghcr.io/bobmcallan/vire-server:latest
- **Network isolation**: Dedicated `vire-test` bridge network for test containers

## Tests
- `go vet ./...` clean
- `docker compose -f tests/docker/docker-compose.yml config` validates
- No references to `../vire` or `VIRE_SERVER_ROOT` remain

## Documentation Updated
- README.md — testing section rewritten for self-contained env

## Devils-Advocate Findings
- Security review completed, findings sent to implementer and addressed
- Image supply chain, CI security, container isolation all reviewed

## Notes
- Orphaned `surreal-test` container from previous runs was cleaned up
- Compose pattern aligned with `vire-infra/docker/vire-stack.yml` (networks, user, start_period, naming)
