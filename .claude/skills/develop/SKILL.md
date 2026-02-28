# /vire-portal-develop - Vire Portal Development Workflow
---
name: develop
description: Develop and test Vire portal features using an agent team.
---

## Usage
```
/vire-portal-develop <feature-description>
```

## Team

Six teammates with distinct roles. The team lead (you) investigates, plans, spawns, and coordinates.

| Role | Model | Purpose |
|------|-------|---------|
| **implementer** | opus | Writes tests first, then code. Fixes issues raised by reviewers. Handles build/verify/docs. |
| **architect** | sonnet | Guards portal architecture. Reviews handler patterns, template structure, auth flows against `docs/`. |
| **reviewer** | haiku | Code quality, pattern consistency, test coverage. Quick, focused reviews. |
| **devils-advocate** | opus | Security, failure modes, edge cases, hostile inputs. Deep adversarial analysis. |
| **test-creator** | sonnet | Creates/reviews UI tests in `tests/ui/` following test-common and test-create-review skills. |
| **test-executor** | haiku | Runs UI tests via `./scripts/ui-test.sh`, reports results. Read-only for test code. |

## Docker Safety

**Non-negotiable.** Test containers use the `-tc` suffix and are managed by `containers.go` and `ui-test.sh`.

1. **NEVER run `docker rm`, `docker stop`, `docker kill`, or any destructive Docker command** manually. The test infrastructure handles stale container cleanup automatically.
2. **NEVER touch containers without the `-tc` suffix.** The user's dev stack (`vire-server`, `vire-surrealdb`, etc.) must never be affected.
3. If a Docker container conflict occurs during testing, it is a bug in `containers.go` — fix the code, don't run manual Docker commands.

## Workflow

### Step 0: Cleanup Stale State

Sessions can end before `TeamDelete` runs (user closes conversation, context exhausted, crash).
This leaves stale team configs with phantom "in-process" members that appear to still be running.

**Always run this before creating a new team:**

1. Check if team `vire-portal-develop` already exists: `Read ~/.claude/teams/vire-portal-develop/config.json`
2. If it exists, call `TeamDelete` to remove the stale team and its tasks
3. Also clean up any stale task directories: `rm -rf ~/.claude/tasks/vire-portal-develop/`

This ensures a clean slate regardless of how the previous session ended.

### Step 1: Plan

1. Create work directory: `.claude/workdir/YYYYMMDD-HHMM-<slug>/`
2. Use the Explore agent to investigate relevant files, patterns, existing code
3. Write `requirements.md` with scope, approach, files expected to change
4. Use investigation results to write detailed task descriptions so teammates don't re-investigate

### Step 2: Create Team and Tasks

Call `TeamCreate` with team_name `"vire-portal-develop"`.

Create tasks across 5 phases using `TaskCreate`. Set `blockedBy` via `TaskUpdate`.

**Phase 1 — Implement** (no dependencies):
- implementer: "Write tests and implement <feature>"
  **MANDATORY:** If UI elements are added, removed, or renamed, include corresponding tests in `tests/ui/`.

**Phase 2 — Review** (parallel, blockedBy: Phase 1):
- architect: "Review architecture alignment and update docs"
- reviewer: "Review code quality and patterns"
- devils-advocate: "Stress-test implementation"

**Phase 3 — UI Tests** (blockedBy: Phase 2; MANDATORY when web pages changed):
Applies when the feature touches: `pages/`, `pages/static/`, `pages/partials/`, HTML templates, CSS, or JS files.
See `.claude/skills/test-common/SKILL.md` and `.claude/skills/test-create-review/SKILL.md` for the full procedure.
- test-creator: "Review/create UI tests"

**Phase 4 — Test Execution** (blockedBy: Phase 3):
- test-executor: "Execute all tests and report results"

**Phase 5 — Verify** (blockedBy: Phase 4):
- implementer: "Build, vet, run locally, update docs"
- reviewer: "Validate docs match implementation"

### Step 3: Spawn Teammates

Spawn all six teammates in parallel using `Task` with `run_in_background: true`. Each teammate reads the task list and works through their tasks autonomously.

**implementer:**
```
name: "implementer"
subagent_type: "general-purpose"
model: "opus"
mode: "bypassPermissions"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the implementer. You write tests first, then production code to pass them.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal
Docs: docs/

Workflow:
1. Read TaskList, claim tasks (owner: "implementer") by setting status to "in_progress"
2. Work through tasks in order, mark completed before moving on
3. Check TaskList for next available task after each completion

For implement tasks: write tests first, then implement to pass them.
  If UI elements change, create/update tests in tests/ui/.
For verify tasks:
  go test ./...
  go vet ./...
  ./scripts/run.sh restart
  curl -s http://localhost:${PORTAL_PORT:-8881}/api/health
  Leave server running.
For docs tasks: update README.md and affected skill files.

Only message teammates for blocking issues or questions. Mark tasks via TaskUpdate.
```

**architect:**
```
name: "architect"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the architect. You guard the portal architecture and ensure implementations
align with established patterns.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal
Docs: docs/ (authentication, features, assessments)

Workflow:
1. Read TaskList, claim tasks (owner: "architect") by setting status to "in_progress"
2. Work through tasks in order, mark completed before moving on

For architecture review tasks:
- Read the implementation files and relevant docs
- Verify handler patterns, template structure, auth flows follow existing conventions
- Check that new routes follow the established pattern in internal/handlers/
- If the feature changes architecture, update relevant docs in docs/
- Consider: does this introduce new dependencies? Does it break existing contracts?
  Does the data flow make sense? Are the right abstractions being used?

Send findings to "implementer" via SendMessage only if fixes are needed.
Mark tasks via TaskUpdate.
```

**reviewer:**
```
name: "reviewer"
subagent_type: "general-purpose"
model: "haiku"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the reviewer. Quick, focused code quality checks.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal
Docs: docs/

Workflow:
1. Read TaskList, claim tasks (owner: "reviewer") by setting status to "in_progress"
2. Work through tasks in order, mark completed before moving on

For code review: check for bugs, verify pattern consistency, validate test coverage.
For docs review: check accuracy against implementation.

Send findings to "implementer" via SendMessage only if fixes are needed.
Mark tasks via TaskUpdate.
```

**devils-advocate:**
```
name: "devils-advocate"
subagent_type: "general-purpose"
model: "opus"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the devils-advocate. Your job is adversarial analysis — find what can break.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal
Docs: docs/

Workflow:
1. Read TaskList, claim tasks (owner: "devils-advocate") by setting status to "in_progress"
2. Work through tasks in order, mark completed before moving on

Scope: input validation, injection attacks, broken auth flows, session fixation, CSRF,
missing error states, race conditions, resource leaks, XSS in templates.
Write stress tests where appropriate.

Send findings to "implementer" via SendMessage only if fixes are needed.
Mark tasks via TaskUpdate.
```

**test-creator:**
```
name: "test-creator"
subagent_type: "general-purpose"
model: "sonnet"
mode: "bypassPermissions"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the test-creator. You write and review UI tests following project conventions.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal

IMPORTANT — read these before writing any tests:
1. .claude/skills/test-common/SKILL.md — mandatory rules
2. .claude/skills/test-create-review/SKILL.md — templates and compliance

Workflow:
1. Read TaskList, claim tasks (owner: "test-creator") by setting status to "in_progress"
2. Read implementation files to understand what was built
3. Review test files for selector accuracy against current HTML templates in pages/
4. Fix stale selectors, create new tests if UI elements were added
5. All tests must comply with test-common mandatory rules

Only message teammates for blocking issues. Mark tasks via TaskUpdate.
```

**test-executor:**
```
name: "test-executor"
subagent_type: "general-purpose"
model: "haiku"
mode: "bypassPermissions"
team_name: "vire-portal-develop"
run_in_background: true
```
```
You are the test-executor. You run tests and report results. NEVER modify test files.

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal

DOCKER SAFETY: NEVER run docker rm, docker stop, docker kill, or any destructive Docker
command. If containers conflict, report the error — do not attempt to fix it yourself.
The test infrastructure (containers.go, ui-test.sh) handles cleanup automatically.

FILE SAFETY: NEVER create files in the project root. All test output goes to tests/logs/.
Do not redirect test output to ad-hoc log files.

Read before executing:
1. .claude/skills/test-common/SKILL.md — mandatory rules (including Docker and file safety)
2. .claude/skills/test-execute/SKILL.md — execution workflow

Workflow:
1. Read TaskList, claim tasks (owner: "test-executor") by setting status to "in_progress"
2. Validate test structure compliance (Rules 1-6 from test-common)
3. Run tests via wrapper script (NEVER raw `go test`):
   ./scripts/ui-test.sh all
   # Or individual suites: smoke, dashboard, nav, devauth, mcp, settings
4. Read summary.md from tests/logs/{timestamp}/ and send to team lead

FEEDBACK LOOP (critical):
- PASS: mark task completed with results
- FAIL: send failure details to "implementer" via SendMessage. Wait for fix, re-run.
  Max 3 rounds, then document remaining failures.

Mark tasks via TaskUpdate.
```

### Step 4: Coordinate

Lightweight coordination as team lead:
1. **Relay** — Forward findings between teammates when needed
2. **Resolve** — Break deadlocks between teammates
3. **Fix trivially** — Typos, missing imports — fix directly rather than round-tripping
4. **Monitor test loop** — Ensure implementer receives test-executor failures. Intervene only if the cycle stalls.
5. **Log activity** — Append key events to `activity.log` in the work directory as they happen
6. **Docker safety** — NEVER run destructive Docker commands (`docker rm`, `docker stop`, `docker kill`) to unblock tests. Container conflicts are handled by `containers.go` automatically.

#### Activity Log

Maintain `.claude/workdir/<task>/activity.log` throughout the session. Append timestamped entries for:
- Phase transitions (e.g. "Phase 2 started — reviewers spawned")
- Task completions (e.g. "Task #1 completed by implementer")
- Blockers and resolutions (e.g. "test-creator: stale selector in settings_test.go — relayed to fix")
- Teammate messages relayed (e.g. "Forwarded devils-advocate findings to implementer")
- Test results (e.g. "test-executor: 8/8 UI tests pass, all suites green")

Format:
```
HH:MM  <event description>
```

This provides a chronological record of the development session alongside the structured `requirements.md` and `summary.md`.

### Step 5: Complete

When all tasks finish:

1. Verify checklist:
   - New code has tests
   - All tests pass (`go test ./...`) — verified by reviewing actual command output
   - `go vet ./...` clean
   - Server builds and runs (`./scripts/run.sh restart`) — leave it running
   - Health endpoint responds (`curl -s http://localhost:${PORTAL_PORT:-8881}/api/health`)
   - Script validation passes (`./scripts/test-scripts.sh`)
   - **If web pages changed:** UI tests executed via `./scripts/ui-test.sh` (never raw `go test`). Confirm by checking `tests/logs/` for results.
   - Architecture docs updated (architect signed off)
   - Devils-advocate signed off
   - README.md updated if user-facing behaviour changed

2. Write `summary.md` in work directory:
   ```markdown
   # Summary: <feature>

   **Status:** completed | partial | blocked

   ## Changes
   | File | Change |
   |------|--------|

   ## Tests
   - Unit tests added/modified
   - UI tests created/updated
   - Test results: pass/fail
   - Fix rounds: N

   ## Architecture
   - Docs updated by architect

   ## Devils-Advocate
   - Key findings and resolutions

   ## Notes
   - Trade-offs, follow-up work, risks
   ```

3. Shutdown teammates: `SendMessage type: "shutdown_request"` to each
4. `TeamDelete`
5. Summarise to user

## Test Commands

| Command | Scope |
|---------|-------|
| `go test ./...` | Full unit test suite |
| `go vet ./...` | Static analysis |
| `./scripts/ui-test.sh all` | All UI test suites |
| `./scripts/ui-test.sh smoke` | Smoke tests only |
| `./scripts/ui-test.sh dashboard` | Dashboard tests |
| `./scripts/ui-test.sh nav` | Navigation tests |
| `./scripts/ui-test.sh profile` | Profile page tests |
| `./scripts/test-scripts.sh` | Script validation |

## Reference

### Key Directories

| Component | Location |
|-----------|----------|
| Entry Point | `cmd/vire-portal/` |
| Application | `internal/app/` |
| API Client | `internal/client/` |
| Configuration | `internal/config/` |
| Auth / OAuth Discovery | `internal/auth/` |
| HTTP Handlers | `internal/handlers/` |
| MCP Server | `internal/mcp/` |
| API Response Cache | `internal/cache/` |
| HTTP Server | `internal/server/` |
| HTML Templates | `pages/` |
| Template Partials | `pages/partials/` |
| Static Assets | `pages/static/` |
| Docker | `docker/` (Dockerfile, compose, config) |
| CI/CD Workflows | `.github/workflows/` |
| Documentation | `docs/`, `README.md` |
| Scripts | `scripts/` |
| Skills | `.claude/skills/` |

The portal is stateless -- all user data is managed by vire-server via REST API (`internal/client/vire_client.go`).

### Routes

| Route | Handler | Auth |
|-------|---------|------|
| `GET /.well-known/oauth-authorization-server` | OAuthServer | No |
| `GET /.well-known/oauth-protected-resource` | OAuthServer | No |
| `POST /register` | OAuthServer | No (RFC 7591 DCR) |
| `GET /authorize` | OAuthServer | No (starts MCP OAuth flow) |
| `POST /token` | OAuthServer | No (code exchange / refresh) |
| `GET /` | PageHandler | No |
| `GET /dashboard` | DashboardHandler | No |
| `GET /strategy` | StrategyHandler | No |
| `GET /capital` | CapitalHandler | No |
| `GET /mcp-info` | MCPPageHandler | No |
| `GET /docs` | PageHandler | No |
| `GET /static/*` | PageHandler | No |
| `POST /mcp` | MCPHandler | Bearer token or session cookie |
| `GET /api/health` | HealthHandler | No |
| `GET /api/server-health` | ServerHealthHandler | No |
| `GET /api/version` | VersionHandler | No |
| `POST /api/auth/login` | AuthHandler | No (forwards to vire-server) |
| `POST /api/auth/logout` | AuthHandler | No |
| `GET /api/auth/login/google` | AuthHandler | No (proxies OAuth redirect from vire-server) |
| `GET /api/auth/login/github` | AuthHandler | No (proxies OAuth redirect from vire-server) |
| `GET /auth/callback` | AuthHandler | No (OAuth callback, sets session cookie or completes MCP flow) |
| `POST /api/shutdown` | Server | No (dev mode only, 403 in prod) |
| `GET /profile` | ProfileHandler | No |
| `POST /profile` | ProfileHandler | No (requires session cookie) |

### Configuration

Config priority: defaults < TOML file < env vars (VIRE_ prefix) < CLI flags.

| Setting | Env Var | Default |
|---------|---------|---------|
| Server port | `VIRE_SERVER_PORT` | `8080` |
| Server host | `VIRE_SERVER_HOST` | `localhost` |
| API URL | `VIRE_API_URL` | `http://localhost:8080` |
| JWT secret | `VIRE_AUTH_JWT_SECRET` | `""` (empty = skip signature verification) |
| OAuth callback URL | `VIRE_AUTH_CALLBACK_URL` | `http://localhost:8080/auth/callback` |
| Portal URL | `VIRE_PORTAL_URL` | `""` (empty = derive from host:port) |
| Default portfolio | `VIRE_DEFAULT_PORTFOLIO` | `""` |
| Display currency | `VIRE_DISPLAY_CURRENCY` | `""` |
| Admin users | `VIRE_ADMIN_USERS` | `""` |
| Service key | `VIRE_SERVICE_KEY` | `""` |
| Portal ID | `VIRE_PORTAL_ID` | hostname |
| Environment | `VIRE_ENV` | `prod` |
| Log level | `VIRE_LOG_LEVEL` | `info` |
| Log format | `VIRE_LOG_FORMAT` | `text` |
| Log outputs | -- | `["console", "file"]` |
| Log file path | -- | `logs/vire-portal.log` |

### API Integration

MCP tool calls are proxied to vire-server with X-Vire-* header injection:
- MCP endpoint: `POST /mcp` (mcp-go StreamableHTTPServer, stateless)
- Proxy: `internal/mcp/proxy.go` forwards to vire-server (default `http://localhost:8080`)
- Static headers: X-Vire-Portfolios, X-Vire-Display-Currency (from config)
- Per-request headers: X-Vire-User-ID (from session cookie JWT sub claim)
- Tools: dynamic catalog from `GET /api/mcp/tools` (registered at startup via `internal/mcp/catalog.go`, 3-attempt retry, validated)
- Response format: raw JSON from vire-server (no markdown formatting)
- Timeouts: 300s proxy + 300s server WriteTimeout (for slow tools like generate_report)
- User data: fetched from vire-server via `internal/client/vire_client.go` (GET/PUT `/api/users/{id}`)
- Navexa key: resolved by vire-server from X-Vire-User-ID (portal never handles the raw key)

### Documentation to Update

When the feature affects user-facing behaviour or API contracts, update:
- `README.md` — if new capabilities, changed routes, or prerequisites
- `docs/` — if architecture, auth flows, or feature design changed
- `.claude/skills/` — affected skill files
