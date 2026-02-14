# Plan: Rename cmd/portal -> cmd/vire-portal + Migrate vire-mcp + Docker Integration

## Investigation Summary

### Current vire-portal repo structure
- **Binary**: `cmd/portal/main.go` -- needs renaming to `cmd/vire-portal/main.go`
- **Internal packages**: `internal/{app,config,handlers,interfaces,mcp,server,storage}` -- portal's own packages
- **Portal's `internal/interfaces/storage.go`**: defines `StorageManager` (BadgerDB-specific, 2 methods) -- different from vire's `StorageManager` (10+ storage accessors)
- **Docker**: `docker/Dockerfile` builds `cmd/portal/` into `vire-portal` binary
- **Scripts**: `scripts/build.sh`, `scripts/deploy.sh`, `scripts/test-scripts.sh` reference `cmd/portal/`
- **Config**: `docker/vire-portal.toml`, `internal/config/` with TOML + env overrides
- **go.mod**: `github.com/bobmcallan/vire-portal`, Go 1.25.3

### vire-mcp in source vire repo
- **Source files** (5,397 LOC total in `cmd/vire-mcp/`):
  - `main.go` (126L) -- entry point, viper config, creates MCP server
  - `proxy.go` (231L) -- HTTP proxy to vire-server REST API
  - `handlers.go` (1,465L) -- MCP tool handlers
  - `formatters.go` (1,704L) -- text formatters for MCP responses
  - `tools.go` (364L) -- MCP tool definitions
  - `proxy_test.go` (366L), `handlers_test.go` (247L), `formatters_test.go` (894L)

### vire-mcp's internal imports (only 2 packages used):
1. **`internal/common`** -- used for: `Logger`, `LoggingConfig`, `NewLoggerFromConfig`, `LoadVersionFromFile`, `GetVersion`, `FormatMoney`, `FormatSignedMoney`, `FormatSignedPct`, `FormatMarketCap`, `IsETF`, `FormatMoneyWithCurrency`, `FormatSignedMoneyWithCurrency`, `IsFresh`, `FreshnessRealTimeQuote`
2. **`internal/models`** -- used for: `RealTimeQuote`, `Portfolio`, `Holding`, `HoldingReview`, `PortfolioReview`, `PortfolioStrategy`, `PortfolioSnapshot`, `GrowthDataPoint`, `SnipeBuy`, `ScreenCandidate`, `StockData`, `TickerSignals`, `FunnelResult`, `FunnelStage`, `SearchRecord`, `PortfolioPlan`, `PlanItem`, `PortfolioWatchlist`, `Fundamentals`, `ComplianceResult`, `ComplianceStatusCompliant`, `ComplianceStatusNonCompliant`, `TrendType`, `NavexaTrade`, `CompanyTimeline`, `FilingSummary`, `NewsIntelligence`

Note: vire-mcp does NOT import `internal/interfaces` directly -- only `common` and `models`. However, `common/config.go` imports `internal/interfaces` (for `KeyValueStorage` in `ResolveDefaultPortfolio`/`ResolveAPIKey`), and `common/format.go` imports `internal/models` (for `HoldingReview`).

### Third-party deps needed by vire-mcp (not already in vire-portal):
- `github.com/spf13/viper` -- used by vire-mcp main.go for config loading
- `github.com/phuslu/log` -- transitive dep of arbor (used by common/logging.go)
- `github.com/ternarybob/arbor` -- used by common/logging.go
- Note: `mcp-go` and `pelletier/go-toml/v2` are already in vire-portal's go.mod

---

## Approach

### Part 1: Rename `cmd/portal/` to `cmd/vire-portal/`

1. **Move the directory**: `cmd/portal/` -> `cmd/vire-portal/`
2. **Update references** in:
   - `docker/Dockerfile` -- `go build ./cmd/portal/` -> `go build ./cmd/vire-portal/`
   - `docker/docker-compose.yml` -- no change needed (builds to image name, not binary path)
   - `scripts/build.sh` -- any references to `cmd/portal/`
   - `scripts/deploy.sh` -- any references to `cmd/portal/`
   - `scripts/test-scripts.sh` -- checks `cmd/portal/main.go` existence and `cmd/portal` directory
   - `.gitignore` -- `/portal` binary entry -> `/vire-portal`
   - `.dockerignore` -- `portal` binary entry -> `vire-portal`
   - `README.md` -- references to `cmd/portal`
3. **Update the Dockerfile binary name**: `vire-portal` binary name in the build command (already named `vire-portal` in Dockerfile, but verify the `go build -o` target matches)

### Part 2: Copy vire's shared internal packages to `internal/vire/`

Since vire-mcp imports `common` and `models` (and `common` transitively imports `interfaces`), we need all three packages:

4. **Create directory structure**:
   - `internal/vire/common/`
   - `internal/vire/models/`
   - `internal/vire/interfaces/`

5. **Copy files** (source -> destination):
   - `vire/internal/common/*.go` -> `internal/vire/common/` (all .go files including tests)
   - `vire/internal/models/*.go` -> `internal/vire/models/` (all .go files including tests)
   - `vire/internal/interfaces/*.go` -> `internal/vire/interfaces/` (all .go files)

6. **Rewrite import paths** in all copied files:
   - `github.com/bobmcallan/vire/internal/common` -> `github.com/bobmcallan/vire-portal/internal/vire/common`
   - `github.com/bobmcallan/vire/internal/models` -> `github.com/bobmcallan/vire-portal/internal/vire/models`
   - `github.com/bobmcallan/vire/internal/interfaces` -> `github.com/bobmcallan/vire-portal/internal/vire/interfaces`

### Part 3: Copy vire-mcp source to `cmd/vire-mcp/`

7. **Copy vire-mcp files**: `vire/cmd/vire-mcp/*.go` -> `cmd/vire-mcp/`

8. **Rewrite import paths** in copied vire-mcp files:
   - `github.com/bobmcallan/vire/internal/common` -> `github.com/bobmcallan/vire-portal/internal/vire/common`
   - `github.com/bobmcallan/vire/internal/models` -> `github.com/bobmcallan/vire-portal/internal/vire/models`

### Part 4: Add new Go dependencies

9. **Add missing dependencies** to go.mod:
   - `github.com/spf13/viper`
   - `github.com/phuslu/log`
   - `github.com/ternarybob/arbor`
   - Run `go mod tidy` to resolve all transitive deps

### Part 5: Docker integration for vire-mcp

10. **Create `docker/Dockerfile.mcp`** (based on vire's Dockerfile.mcp):
    - Multi-stage build: golang:1.25 builder + alpine:3.19 runtime
    - Build `./cmd/vire-mcp` with ldflags injecting version into `internal/vire/common.Version` (updated path)
    - Copy `docker/vire-mcp.toml` as config
    - Expose port 4243, ENTRYPOINT `./vire-mcp`

11. **Create `docker/vire-mcp.toml`** -- config for MCP server in Docker (based on vire's `docker/vire-mcp.toml.docker`)

12. **Update `docker/docker-compose.yml`** -- add `vire-mcp` service:
    - Build from `docker/Dockerfile.mcp`
    - Expose port 4243
    - Depends on `vire-server` (healthy)
    - Healthcheck: `HEALTHCHECK NONE` (MCP servers are stdio-based, no HTTP health endpoint; or add a simple TCP check on 4243)
    - Environment: `VIRE_MCP_PORT=4243`, `VIRE_MCP_SERVER_URL=http://vire-server:4242`

13. **Update `docker/docker-compose.ghcr.yml`** -- add `vire-mcp` service (GHCR pull variant)

14. **Update `scripts/build.sh`** -- support building vire-mcp Docker image (add `--mcp` or `--all` flag, or build both by default)

15. **Update `scripts/deploy.sh`** -- deploy both portal and mcp containers

16. **Update `scripts/test-scripts.sh`** -- add validation checks for:
    - `cmd/vire-mcp/main.go` existence
    - `docker/Dockerfile.mcp` existence
    - `docker/vire-mcp.toml` existence
    - Dockerfile.mcp build args
    - docker-compose vire-mcp service

### Part 6: Update .gitignore / .dockerignore

17. **`.gitignore`**: Add `/vire-mcp` binary entry
18. **`.dockerignore`**: Add `vire-mcp` binary entry

### Part 7: Verification

19. **`go build ./cmd/vire-portal/`** -- verify portal still compiles
20. **`go build ./cmd/vire-mcp/`** -- verify MCP compiles
21. **`go test ./...`** -- all tests pass (portal + mcp + internal/vire/* tests)
22. **`go vet ./...`** -- no vet issues
23. **`docker build -f docker/Dockerfile -t vire-portal:latest .`** -- portal Docker build
24. **`docker build -f docker/Dockerfile.mcp -t vire-mcp:latest .`** -- MCP Docker build
25. **`./scripts/test-scripts.sh`** -- all script validation tests pass

---

## Key Design Decisions

1. **Namespace: `internal/vire/`** -- Avoids conflict with portal's existing `internal/interfaces/storage.go`. The vire packages are a separate domain (vire-server's data models and utilities), not portal-specific.

2. **Copy all files from common/models/interfaces, including tests** -- Tests validate the copied code works correctly after the import path rewrite. The `strategy_test.go`, `config_test.go`, `format_test.go`, `logging_test.go`, `userctx_test.go` all test logic that vire-mcp depends on.

3. **vire-mcp stays as `package main` in `cmd/vire-mcp/`** -- Keeps it as a separate binary, matching the convention of `cmd/vire-portal/`.

4. **Version injection via ldflags** -- For vire-mcp Docker, ldflags target changes from `github.com/bobmcallan/vire/internal/common` to `github.com/bobmcallan/vire-portal/internal/vire/common`.

5. **vire-mcp config uses spf13/viper** -- The source vire-mcp uses viper for TOML + env config binding. We keep this rather than rewriting to match portal's pelletier/go-toml approach, since it's a separate binary with its own config conventions.

---

## Risk Areas

- **Import path completeness**: The `common/config.go` file imports `internal/interfaces` for `KeyValueStorage` type in function signatures. Must ensure `internal/vire/interfaces/` is properly copied and import-rewritten.
- **Transitive dependency conflicts**: vire uses newer versions of some shared deps (e.g., `cespare/xxhash/v2 v2.3.0` vs portal's `v2.2.0`). `go mod tidy` should resolve to compatible versions.
- **Logging stress tests**: `common/logging_stress_test.go` (329L) may have race conditions or timing-sensitive behavior. If flaky, can be excluded.
- **test-scripts.sh** needs careful updates to check both binaries and Dockerfiles.

## Files Changed (Summary)

### Renamed
- `cmd/portal/main.go` -> `cmd/vire-portal/main.go`

### New files (copied from vire repo + import rewrite)
- `cmd/vire-mcp/` (7 files: main.go, proxy.go, handlers.go, formatters.go, tools.go, + 3 test files)
- `internal/vire/common/` (11 files)
- `internal/vire/models/` (10 files)
- `internal/vire/interfaces/` (3 files)
- `docker/Dockerfile.mcp`
- `docker/vire-mcp.toml`

### Modified
- `docker/Dockerfile` (binary path update)
- `docker/docker-compose.yml` (add vire-mcp service)
- `docker/docker-compose.ghcr.yml` (add vire-mcp service)
- `scripts/build.sh` (add vire-mcp build support)
- `scripts/deploy.sh` (add vire-mcp deploy support)
- `scripts/test-scripts.sh` (add vire-mcp validation)
- `.gitignore` (add vire-mcp binary, update portal binary name)
- `.dockerignore` (add vire-mcp binary, update portal binary name)
- `go.mod` / `go.sum` (new dependencies)
