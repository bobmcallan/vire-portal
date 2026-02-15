# /vire-portal-develop - Vire Portal Development Workflow

Develop and test Vire portal features using an agent team.

## Usage
```
/vire-portal-develop <feature-description>
```

## Outputs

Every invocation produces documentation in `.claude/workdir/<datetime>-<taskname>/`:
- `requirements.md` — what was requested, scope, approach chosen
- `summary.md` — what was built, files changed, tests added, outcome

## Procedure

### Step 1: Create Work Directory

Generate the work directory path using the current datetime and a short task slug:
```
.claude/workdir/YYYYMMDD-HHMM-<task-slug>/
```

Example: `.claude/workdir/20260214-1430-oauth-handler/`

Create the directory and write `requirements.md`:

```markdown
# Requirements: <feature-description>

**Date:** <date>
**Requested:** <what the user asked for>

## Scope
- <what's in scope>
- <what's out of scope>

## Approach
<chosen approach and rationale>

## Files Expected to Change
- <file list>
```

### Step 2: Investigate and Plan

Before creating the team, the team lead investigates the codebase directly:

1. Use the Explore agent to understand relevant files, patterns, and existing implementations
2. Determine the approach, files to change, and any risks
3. Write this into `requirements.md` (created in Step 1) under the Approach section
4. Use this knowledge to write detailed task descriptions — teammates should NOT need to re-investigate

### Step 3: Create Team and Tasks

Call `TeamCreate`:
```
team_name: "vire-portal-develop"
description: "Developing: <feature-description>"
```

Create 7 tasks across 3 phases using `TaskCreate`. Set `blockedBy` dependencies via `TaskUpdate`.

**Phase 1 — Implement** (no dependencies):
- "Write tests and implement <feature>" — owner: implementer
  Task description includes: approach, files to change, test strategy, and acceptance criteria.
- "Review implementation and tests" — owner: reviewer, blockedBy: [implement task]
  Scope: code quality, pattern consistency, test coverage.
- "Stress-test implementation" — owner: devils-advocate, blockedBy: [implement task]
  Scope: security, failure modes, edge cases, hostile inputs.

**Phase 2 — Verify** (blockedBy: review + stress-test):
- "Build, test, and deploy to Docker" — owner: implementer
- "Validate deployment" — owner: reviewer, blockedBy: [build task]

**Phase 3 — Document** (blockedBy: validate):
- "Update affected documentation" — owner: implementer
- "Verify documentation matches implementation" — owner: reviewer, blockedBy: [update docs task]

### Step 4: Spawn Teammates

Spawn all three teammates in parallel using the `Task` tool:

**implementer:**
```
name: "implementer"
subagent_type: "general-purpose"
model: "sonnet"
mode: "bypassPermissions"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the implementer on a development team. You write tests and code.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal
  Read .claude/skills/develop/SKILL.md Reference section for conventions, routes, config, and API details.

  Workflow:
  1. Read TaskList, claim your tasks (owner: "implementer") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  For implement tasks: write tests first, then implement to pass them.
  For verify tasks: run go test ./..., go vet ./..., then deploy:
    ./scripts/deploy.sh local --force
    curl -s http://localhost:4241/api/health
  For documentation tasks: update affected files in docs/, README.md, and .claude/skills/.

  Do NOT send status messages. Only message teammates for: blocking issues, review findings, or questions.
  Mark tasks completed via TaskUpdate — the system handles notifications.
```

**reviewer:**
```
name: "reviewer"
subagent_type: "general-purpose"
model: "haiku"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the reviewer on a development team. You review for code quality, pattern
  consistency, test coverage, and documentation accuracy.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal
  Read .claude/skills/develop/SKILL.md Reference section for conventions, routes, config, and API details.

  Workflow:
  1. Read TaskList, claim your tasks (owner: "reviewer") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  When reviewing code: read changed files and surrounding context, check for bugs, verify
  consistency with existing patterns, validate test coverage is adequate.
  When reviewing docs: check accuracy against implementation, verify examples work.
  When validating deployment: confirm health endpoint responds, test key routes.

  Send findings to "implementer" via SendMessage only if fixes are needed.
  Do NOT send status messages. Mark tasks completed via TaskUpdate — the system handles notifications.
```

**devils-advocate:**
```
name: "devils-advocate"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the devils-advocate on a development team. Your scope: security vulnerabilities,
  failure modes, edge cases, and hostile inputs.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal
  Read .claude/skills/develop/SKILL.md Reference section for conventions, routes, config, and API details.

  Workflow:
  1. Read TaskList, claim your tasks (owner: "devils-advocate") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  Stress-test the implementation: input validation, injection attacks, broken auth flows,
  missing error states, race conditions, resource leaks, crash recovery. Write stress tests
  where appropriate. Play the role of a hostile input source.

  Send findings to "implementer" via SendMessage only if fixes are needed.
  Do NOT send status messages. Mark tasks completed via TaskUpdate — the system handles notifications.
```

### Step 5: Coordinate

As team lead, your job is lightweight coordination:

1. **Relay information** — If one teammate's findings affect another, forward via `SendMessage`.
2. **Resolve conflicts** — If the devils-advocate and implementer disagree, make the call.
3. **Apply direct fixes** — For trivial issues (typos, missing imports), fix them directly rather than round-tripping through the implementer.

### Step 6: Completion

When all tasks are complete:

1. Verify the code quality checklist:
   - All new code has tests
   - All tests pass (`go test ./...`)
   - Go vet is clean (`go vet ./...`)
   - Docker container builds and deploys (`./scripts/deploy.sh local --force`)
   - Health endpoint responds (`curl -s http://localhost:4241/api/health`)
   - Script validation passes (`./scripts/test-scripts.sh`)
   - README.md updated if user-facing behaviour changed
   - API contract documentation matches implementation
   - Devils-advocate has signed off

2. Write `summary.md` in the work directory:

```markdown
# Summary: <feature-description>

**Date:** <date>
**Status:** <completed | partial | blocked>

## What Changed

| File | Change |
|------|--------|
| `path/to/file` | <brief description> |

## Tests
- <tests added or modified>
- <test results: pass/fail>

## Documentation Updated
- <list of docs/README changes>

## Devils-Advocate Findings
- <key issues raised and how they were resolved>

## Notes
- <anything notable: trade-offs, follow-up work, risks>
```

3. Shut down teammates:
   ```
   SendMessage type: "shutdown_request" to each teammate
   ```

4. Clean up:
   ```
   TeamDelete
   ```

5. Summarise what was built, changed, and tested.

## Reference

### Key Directories

| Component | Location |
|-----------|----------|
| Entry Point | `cmd/vire-portal/` |
| Application | `internal/app/` |
| Configuration | `internal/config/` |
| HTTP Handlers | `internal/handlers/` |
| MCP Server | `internal/mcp/` |
| User Importer | `internal/importer/` |
| Storage Interfaces | `internal/interfaces/` |
| User Model | `internal/models/` |
| HTTP Server | `internal/server/` |
| BadgerDB Storage | `internal/storage/badger/` |
| HTML Templates | `pages/` |
| Template Partials | `pages/partials/` |
| Static Assets | `pages/static/` |
| Docker | `docker/` (Dockerfile, compose, config) |
| CI/CD Workflows | `.github/workflows/` |
| Documentation | `docs/`, `README.md` |
| Scripts | `scripts/` |
| Skills | `.claude/skills/` |

### Routes

| Route | Handler | Auth |
|-------|---------|------|
| `GET /` | PageHandler | No |
| `GET /dashboard` | DashboardHandler | No |
| `GET /static/*` | PageHandler | No |
| `POST /mcp` | MCPHandler | No |
| `GET /api/health` | HealthHandler | No |
| `GET /api/version` | VersionHandler | No |
| `POST /api/auth/dev` | AuthHandler | No (dev mode only, 404 in prod) |
| `POST /api/auth/logout` | AuthHandler | No |
| `GET /settings` | SettingsHandler | No |
| `POST /settings` | SettingsHandler | No (requires session cookie) |

### Configuration

Config priority: defaults < TOML file < env vars (VIRE_ prefix) < CLI flags.

| Setting | Env Var | Default |
|---------|---------|---------|
| Server port | `VIRE_SERVER_PORT` | `8080` |
| Server host | `VIRE_SERVER_HOST` | `localhost` |
| API URL | `VIRE_API_URL` | `http://localhost:8080` |
| Default portfolio | `VIRE_DEFAULT_PORTFOLIO` | `""` |
| Display currency | `VIRE_DISPLAY_CURRENCY` | `""` |
| Import users | -- | `false` (TOML: `import.users`) |
| Import users file | -- | `data/users.json` (TOML: `import.users_file`) |
| BadgerDB path | `VIRE_BADGER_PATH` | `./data/vire` |
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
- Per-request headers: X-Vire-User-ID, X-Vire-Navexa-Key (from session cookie + user lookup)
- Tools: dynamic catalog from `GET /api/mcp/tools` (registered at startup via `internal/mcp/catalog.go`, 3-attempt retry, validated)
- Response format: raw JSON from vire-server (no markdown formatting)
- Timeouts: 300s proxy + 300s server WriteTimeout (for slow tools like generate_report)

Future gateway integration (deferred):
- Auth: JWT in `Authorization: Bearer` header
- Error responses follow consistent `{ error: { code, message } }` shape
- Token refresh: `POST /api/auth/refresh` (automatic on 401)

### Documentation to Update

When the feature affects user-facing behaviour or API contracts, update:
- `README.md` — if new capabilities, changed routes, or prerequisites
- `docs/requirements.md` — if API contracts or architecture changed
- `.claude/skills/` — affected skill files
