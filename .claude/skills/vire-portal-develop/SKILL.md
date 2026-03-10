# /vire-portal-develop - Vire Portal Development Workflow
---
name: vire-portal-develop
description: Develop and test Vire portal features using complexity-scaled workflows. Use when the user says /develop, "implement feature", "add feature", or describes a code change to vire-portal. Scales from quick fixes (solo, no plan) to major initiatives (full team + worktree).
---

## Usage
```
/vire-portal-develop <feature-description>
```

## Step 0: Complexity Assessment

Before any code changes, assess the task and select a workflow tier.

**Evaluate these dimensions:**
- **Scope**: How many files/modules are affected?
- **Ambiguity**: Is the approach obvious, or are there multiple valid strategies?
- **Risk**: Could this break existing functionality?
- **Dependencies**: Does this require coordinating across components?

| Tier | When | Workflow |
|------|------|----------|
| **Quick Fix** | Single file, obvious change (typo, config tweak, small bug) | Implement directly -- no plan mode, no team. Just do it. |
| **Standard Task** | 2-5 files, clear scope, one approach | `EnterPlanMode` to outline approach. `TaskCreate` to track steps. Implement solo. |
| **Complex Feature** | Multi-file, architectural decisions needed, cross-cutting concerns | Plan mode + `TeamCreate` with 2-3 agents. Tasks with dependencies. |
| **Major Initiative** | System-wide changes, multiple components, high risk | Full team + `EnterWorktree` for isolated work. Comprehensive task board. |

A 3-file change that touches auth might be "Complex" while a 10-file rename might be "Standard." Reason about risk, not just file count.

---

## Quick Fix Workflow

1. Implement the change directly
2. Run `go test ./...` and `go vet ./...`
3. If web pages changed: run `./scripts/ui-test.sh` for affected suites
4. Proceed to **Deploy & Verify**

---

## Standard Task Workflow

### Plan

1. Call `EnterPlanMode`
2. Use `Agent` (subagent_type: `Explore`) to investigate relevant files
3. Write the plan (scope, file changes, test cases)
4. Call `ExitPlanMode` for user approval

### Implement

1. Create tasks with `TaskCreate` (imperative subjects, acceptance criteria in description)
2. Work through tasks sequentially, marking `in_progress` then `completed`
3. Write tests first, then implement
4. Run `go test ./...` and `go vet ./...`
5. If web pages changed: run `./scripts/ui-test.sh` for affected suites

### Verify

1. All tests pass
2. `go vet ./...` clean
3. Check `.claude/skills/vire-portal-architect/SKILL.md` rules against changes
4. Proceed to **Deploy & Verify**

---

## Complex Feature Workflow

### Plan

1. Call `EnterPlanMode`
2. Use `Agent` (subagent_type: `Explore`) to investigate codebase
3. Spawn a **Plan agent** (opus) to produce detailed implementation spec:

```
name: "planner"
subagent_type: "Plan"
model: "opus"
```
```
You are planning the implementation of a feature for the Vire Portal.

Working dir: /home/bobmc/development/vire-portal

Produce a detailed implementation spec and write it to the work directory as requirements.md.

The spec must be detailed enough for a Sonnet-class model to implement without making
architectural decisions. Include:

1. **Scope** -- what the feature does, what it does NOT do
2. **File changes** -- for each file: path, what to add/change, patterns to follow
3. **Function signatures** -- exact Go function/method signatures
4. **Template structure** -- HTML, Alpine.js bindings, CSS classes (if UI changes)
5. **Test cases** -- unit test functions and what each validates
6. **UI test cases** -- UI test functions and what each validates (if pages change)
7. **Edge cases** -- error states, auth boundaries, empty data scenarios
```

4. Create work directory: `.claude/workdir/YYYYMMDD-HHMM-<slug>/`
5. Review the plan, call `ExitPlanMode` for user approval

### Create Team and Tasks

1. Call `TeamCreate` with team_name `"vire-portal-develop"`
2. Create tasks with `TaskCreate` across phases, using `addBlockedBy`/`addBlocks` for dependencies

**Phase 1 -- Implement** (no dependencies):
- implementer: "Write tests and implement <feature>"

**Phase 2 -- Review** (blockedBy: Phase 1, parallel):
- architect: "Review architecture alignment"
- reviewer: "Review code quality and patterns"
- devils-advocate: "Stress-test implementation"

**Phase 3 -- UI Tests** (blockedBy: Phase 2, when web pages changed):
- test-creator: "Review/create UI tests"

**Phase 4 -- Test Execution** (blockedBy: Phase 3):
- test-executor: "Execute all tests and report results"

**Phase 5 -- Verify** (blockedBy: Phase 4):
- implementer: "Build and vet"

### Spawn Teammates

Spawn all teammates in parallel with `run_in_background: true`.

| Role | Model | Mode | Purpose |
|------|-------|------|---------|
| **implementer** | sonnet | bypassPermissions | Executes implementation spec. Writes tests first, then code. |
| **architect** | haiku | -- | Guards portal architecture against `.claude/skills/vire-portal-architect/SKILL.md` |
| **reviewer** | haiku | -- | Code quality, pattern consistency, test coverage |
| **devils-advocate** | opus | -- | Security, failure modes, edge cases, hostile inputs. Writes stress tests. |
| **test-creator** | haiku | bypassPermissions | Creates/reviews UI tests per `vire-portal-test-common` and `vire-portal-test-create` |
| **test-executor** | haiku | bypassPermissions | Runs tests via `./scripts/ui-test.sh`. Read-only for test code. |

**Every teammate prompt MUST include:**
```
You are a teammate on the vire-portal project. Your role is: {role_description}

Team: "vire-portal-develop". Working dir: /home/bobmc/development/vire-portal

TASK WORKFLOW:
1. Check TaskList for your assigned tasks
2. Use TaskGet to read full task details before starting
3. Mark tasks in_progress with TaskUpdate when you begin
4. Mark tasks completed with TaskUpdate when done
5. Check TaskList again for newly unblocked work

COMMUNICATION: Use SendMessage to message teammates by name.

SHUTDOWN: When all your tasks are completed and no more work remains, send a message
to "team-lead" confirming you are done, then wait. When you receive a shutdown_request,
you MUST immediately call the shutdown_response tool with approve: true and the request_id
from the message. Do not ignore shutdown requests.
```

### Coordinate

Lightweight coordination as team lead:
1. **Relay** -- Forward findings between teammates when needed
2. **Resolve** -- Break deadlocks between teammates
3. **Fix trivially** -- Typos, missing imports -- fix directly rather than round-tripping
4. **Monitor test loop** -- Ensure implementer receives test-executor failures. Intervene only if the cycle stalls.
5. **Log activity** -- Append key events to `activity.log` in the work directory

### Complete

1. Verify checklist:
   - New code has tests
   - All tests pass (`go test ./...`)
   - `go vet ./...` clean
   - If web pages changed: UI tests executed via `./scripts/ui-test.sh`
   - Architecture review signed off
   - Devils-advocate signed off

2. Write `summary.md` in work directory
3. Shutdown teammates: `SendMessage type: "shutdown_request"` to each
4. `TeamDelete`
5. Proceed to **Deploy & Verify**

---

## Major Initiative Workflow

Same as Complex Feature, with these additions:

### Worktree Isolation

Use `EnterWorktree` to create an isolated copy of the repo before implementation begins. All work happens in the worktree. Review and verify before merging back. Use `ExitWorktree` with `action: "keep"` if more work is needed, or `action: "remove"` when done.

### Extended Team

Scale team size as needed. Consider multiple implementers for parallel work streams. Use `SendMessage` for cross-stream coordination.

### Comprehensive Task Board

Create all tasks upfront with full dependency graphs via `addBlockedBy`/`addBlocks`. Include acceptance criteria in every task description. Use `activeForm` for meaningful spinner text.

---

## Docker Safety

**Non-negotiable.** Test containers use the `-tc` suffix and are managed by `containers.go` and `ui-test.sh`.

1. **NEVER run `docker rm`, `docker stop`, `docker kill`, or any destructive Docker command** manually
2. **NEVER touch containers without the `-tc` suffix.** The user's dev stack must never be affected.
3. Container conflicts are bugs in `containers.go` -- fix the code, don't run manual Docker commands.

---

## Deploy & Verify

After all tasks pass and verification is complete.

### Commit & Push

**NON-NEGOTIABLE:** Always use the `/commit-push` skill -- NEVER run `git commit` and `git push` manually.
The `/commit-push` skill handles version bumping (`.version` patch increment + build timestamp),
formatting, and conventional commit format.

### Wait for Deployment

```bash
gh run list --branch main --limit 1 --json databaseId,status,conclusion
gh run watch <run-id> --exit-status
```

If the workflow fails: `gh run view <run-id> --log-failed` -- report and stop.

### Verify Version via MCP

Loop (max 10 attempts, 30s apart):
1. Call `mcp__vire__system_get_version`
2. Compare `commit` field to pushed commit hash
3. Match = deployment confirmed

### Post-Deploy Health Check

1. `mcp__vire__system_get_diagnostics` with `source: "portal"`, `limit: 20` -- check for errors since deploy
2. `mcp__vire__system_list_mcp_tools` -- verify tool catalog loaded
3. `mcp__vire__get_version` -- confirm deployed version

### Report

Deploy status: **GREEN** (no errors) or **YELLOW** (warnings/errors found). Include version, build, commit, diagnostics summary.

---

## Task Management

### At Session Start

Check `TaskList` for existing tasks from previous sessions:
- Review pending tasks -- are they still relevant?
- Check for stale `in_progress` tasks (interrupted work)
- Don't delete user-created tasks without confirmation

### Cleanup Stale Teams

Sessions can end before `TeamDelete` runs. Always check before creating a new team:
1. Check if team `vire-portal-develop` exists: `Read ~/.claude/teams/vire-portal-develop/config.json`
2. If exists, `TeamDelete` to remove stale team
3. Clean up stale task directories

---

## Test Commands

| Command | Scope |
|---------|-------|
| `go test ./...` | Full unit test suite |
| `go vet ./...` | Static analysis |
| `./scripts/ui-test.sh all` | All UI test suites |
| `./scripts/ui-test.sh smoke` | Smoke tests only |
| `./scripts/ui-test.sh dashboard` | Dashboard tests |
| `./scripts/ui-test.sh stock` | Stock page tests |
| `./scripts/ui-test.sh nav` | Navigation tests |
| `./scripts/ui-test.sh profile` | Profile page tests |
| `./scripts/test-scripts.sh` | Script validation |

## Reference

### Key Directories

| Component | Location |
|-----------|----------|
| Entry Point | `cmd/vire-portal/` |
| MCP CLI | `cmd/vire-mcp/` |
| Application | `internal/app/` |
| API Client | `internal/client/` |
| Configuration | `internal/config/` |
| Auth / OAuth | `internal/auth/` |
| HTTP Handlers | `internal/handlers/` |
| MCP Server | `internal/mcp/` |
| API Response Cache | `internal/cache/` |
| HTTP Server | `internal/server/` |
| HTML Templates | `pages/` |
| Template Partials | `pages/partials/` |
| Static Assets | `pages/static/` |
| Docker | `docker/` |
| CI/CD Workflows | `.github/workflows/` |
| Scripts | `scripts/` |
| Skills | `.claude/skills/` |
| UI Tests | `tests/ui/` |
| Test Common | `tests/common/` |

The portal is stateless -- all user data is managed by vire-server via REST API (`internal/client/vire_client.go`).

### Routes

| Route | Handler | Auth |
|-------|---------|------|
| `GET /.well-known/oauth-*` | OAuthServer | No |
| `POST /register` | OAuthServer | No (RFC 7591 DCR) |
| `GET /authorize` | OAuthServer | No (starts MCP OAuth flow) |
| `POST /token` | OAuthServer | No (code exchange / refresh) |
| `GET /` | PageHandler | No |
| `GET /dashboard[/{portfolio}]` | DashboardHandler | Yes |
| `GET /m[/{portfolio}]` | MobileDashboardHandler | Yes |
| `GET /stock/{ticker}` | StockHandler | Yes |
| `GET /strategy` | StrategyHandler | Yes |
| `GET /cash` | CashHandler | Yes |
| `GET /profile` | ProfileHandler | Yes |
| `POST /profile` | ProfileHandler | Yes |
| `GET /mcp-info` | MCPPageHandler | No |
| `GET /help, /changelog, /glossary, /docs, /error` | PageHandler | No |
| `GET /admin/users` | AdminUsersHandler | Yes (admin) |
| `GET /admin/gemini` | AdminGeminiHandler | Yes (admin) |
| `GET /admin/prompts[/{name}]` | AdminPromptsHandler | Yes (admin) |
| `POST /api/auth/login` | AuthHandler | No |
| `POST /api/auth/logout` | AuthHandler | No |
| `GET /api/auth/login/{google,github}` | AuthHandler | No |
| `GET /auth/callback` | AuthHandler | No |
| `GET /api/health` | HealthHandler | No |
| `GET /api/server-health` | ServerHealthHandler | No |
| `GET /api/version` | VersionHandler | No |
| `POST /api/shutdown` | Server | No (dev only) |
| `GET /api/*` | Proxy -> vire-server | Yes (cookie JWT) |
| `POST /mcp` | MCPHandler | Bearer/session |
| `GET /ws/status` | WSStatusHandler | No |
| `GET /static/*` | FileServer | No |

### Configuration

Config priority: defaults < TOML file < env vars (VIRE_ prefix) < CLI flags.

| Setting | Env Var | Default |
|---------|---------|---------|
| Server port | `VIRE_SERVER_PORT` | `8080` |
| Server host | `VIRE_SERVER_HOST` | `localhost` |
| API URL | `VIRE_API_URL` | `http://localhost:8080` |
| JWT secret | `VIRE_AUTH_JWT_SECRET` | `""` (skip sig in dev) |
| OAuth callback URL | `VIRE_AUTH_CALLBACK_URL` | `http://localhost:8080/auth/callback` |
| Portal URL | `VIRE_PORTAL_URL` | `""` (derive from host:port) |
| Default portfolio | `VIRE_DEFAULT_PORTFOLIO` | `""` |
| Display currency | `VIRE_DISPLAY_CURRENCY` | `""` |
| Admin users | `VIRE_ADMIN_USERS` | `""` |
| Service key | `VIRE_SERVICE_KEY` | `""` |
| Portal ID | `VIRE_PORTAL_ID` | hostname |
| Environment | `VIRE_ENV` | `prod` |
| Log level | `VIRE_LOG_LEVEL` | `info` |

### API Integration

MCP tool calls are proxied to vire-server with X-Vire-* header injection:
- MCP endpoint: `POST /mcp` (mcp-go StreamableHTTPServer, stateless)
- Proxy: `internal/mcp/proxy.go` forwards to vire-server
- Static headers: X-Vire-Portfolios, X-Vire-Display-Currency (from config)
- Per-request headers: X-Vire-User-ID (from session cookie JWT sub claim)
- Tools: dynamic catalog from `GET /api/mcp/tools` (registered at startup, 3-attempt retry)
- Timeouts: 300s proxy + 300s server WriteTimeout (for slow tools like generate_report)
