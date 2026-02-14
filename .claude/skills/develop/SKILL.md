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

### Step 2: Create Team and Tasks

Call `TeamCreate`:
```
team_name: "vire-portal-develop"
description: "Developing: <feature-description>"
```

Create tasks grouped by phase using `TaskCreate`. Set `blockedBy` dependencies via `TaskUpdate` so later phases cannot start until earlier ones complete.

**Phase 1 — Investigate** (no dependencies):
- "Investigate codebase and propose approach" — owner: implementer
- "Challenge the proposed approach" — owner: devils-advocate, blockedBy: [investigate task]
- "Review approach for pattern consistency" — owner: reviewer, blockedBy: [investigate task]

**Phase 2 — Test** (blockedBy all Phase 1):
- "Write failing tests for the feature" — owner: implementer
- "Challenge test strategy and coverage" — owner: devils-advocate, blockedBy: [write tests task]

**Phase 3 — Implement** (blockedBy all Phase 2):
- "Implement feature to pass tests" — owner: implementer
- "Review implementation for bugs and quality" — owner: reviewer, blockedBy: [implement task]
- "Stress-test implementation" — owner: devils-advocate, blockedBy: [implement task]

**Phase 4 — Verify** (blockedBy all Phase 3):
- "Build and run full verification suite" — owner: implementer
- "Validate end-to-end integration" — owner: reviewer, blockedBy: [build task]

**Phase 5 — Document** (blockedBy all Phase 4):
- "Update affected documentation" — owner: implementer
- "Verify documentation matches implementation" — owner: reviewer, blockedBy: [update docs task]

### Step 3: Spawn Teammates

Spawn all three teammates in parallel using the `Task` tool:

**implementer:**
```
name: "implementer"
subagent_type: "general-purpose"
model: "sonnet"
mode: "plan"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the implementer on a development team. You write code and tests.

  Your team name is "vire-portal-develop". Read your tasks from the task list with TaskList.
  Claim tasks assigned to you (owner: "implementer") by setting status to "in_progress".
  Work through tasks in ID order. Mark each completed before moving to the next.

  When you enter plan mode, write your plan and call ExitPlanMode. The team lead
  will review and approve before you proceed with implementation.

  Key conventions:
  - Working directory: /home/bobmc/development/vire-portal
  - Tech stack: Go 1.25+, net/http, html/template, Alpine.js (CDN), BadgerDB via badgerhold
  - Build: go build ./cmd/portal/
  - Tests: go test ./... (4 test packages: config, handlers, server, storage/badger)
  - Vet: go vet ./...
  - Docker build: docker build -t vire-portal:latest .
  - Script validation: ./scripts/test-scripts.sh (130 tests)
  - The portal is a Go server rendering HTML templates, served via Docker on Cloud Run
  - Design: 80s B&W aesthetic -- IBM Plex Mono, no border-radius, no box-shadow, monochrome only
  - All backend calls go through the vire-gateway REST API
  - Auth: direct OAuth (Google/GitHub) via gateway, JWT in memory, refresh via httpOnly cookie
  - Config: TOML with defaults -> file -> env (VIRE_ prefix) -> CLI flags
  - Storage: BadgerDB embedded, interface-based for future swap

  Documentation tasks: update affected files in docs/, README.md, and
  .claude/skills/ to reflect the changes made.

  Send messages to teammates via SendMessage when you need input.
  After completing each task, check TaskList for your next task.
  If all your tasks are done or blocked, send a message to the team lead.
```

**reviewer:**
```
name: "reviewer"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the reviewer on a development team. You review code for bugs, quality,
  and consistency with existing codebase patterns.

  Your team name is "vire-portal-develop". Read your tasks from the task list with TaskList.
  Claim tasks assigned to you (owner: "reviewer") by setting status to "in_progress".
  Work through tasks in ID order. Mark each completed before moving to the next.

  Working directory: /home/bobmc/development/vire-portal

  When reviewing:
  - Read the changed files and surrounding context
  - Check for bugs, logic errors, and edge cases
  - Verify consistency with existing patterns in the codebase
  - For Go: check error handling, interface compliance, goroutine safety, context usage
  - For handlers: check HTTP method validation, response content types, status codes
  - For templates: check html/template escaping, partial includes, Alpine.js bindings
  - For config: check TOML parsing, env var mapping (VIRE_ prefix), CLI flag precedence
  - For storage: check BadgerDB transaction handling, connection lifecycle, interface compliance
  - Validate test coverage is adequate
  - Report findings via SendMessage to "implementer" (for fixes) and to the team lead (for status)

  When reviewing documentation:
  - Check that README.md accurately reflects new/changed functionality
  - Check that API contracts match the gateway documentation
  - Verify examples and usage instructions work

  After completing each task, check TaskList for your next task.
  If all your tasks are done or blocked, send a message to the team lead.
```

**devils-advocate:**
```
name: "devils-advocate"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the devils-advocate on a development team. You critically challenge
  every decision to catch problems early.

  Your team name is "vire-portal-develop". Read your tasks from the task list with TaskList.
  Claim tasks assigned to you (owner: "devils-advocate") by setting status to "in_progress".
  Work through tasks in ID order. Mark each completed before moving to the next.

  Working directory: /home/bobmc/development/vire-portal

  Your job is to:
  - Challenge design choices: Are there simpler alternatives? What assumptions are being made?
  - Poke holes in test strategy: What edge cases are missing? Could tests pass with a broken implementation?
  - Stress-test implementation: Input validation? Template injection? Broken auth flows? Missing error states?
  - For Go handlers: Race conditions? Goroutine leaks? Unclosed response bodies? Missing context cancellation?
  - For BadgerDB: Data corruption on crash? Concurrent write conflicts? Disk space exhaustion?
  - For templates: XSS via unescaped output? Missing Alpine.js error states? Broken partial includes?
  - For API integration: What if the gateway is down? What if tokens expire mid-operation? Race conditions?
  - For OAuth: CSRF risks? Open redirect vulnerabilities? State parameter validation?
  - For Docker: Missing healthcheck? Unbounded data volume growth? Missing security headers?
  - Question scope: Too broad? Too narrow? Right abstraction level?
  - Play the role of a hostile input source

  You must be convinced before any task is considered complete.
  Send findings via SendMessage to "implementer" (for action) and to the team lead (for awareness).

  After completing each task, check TaskList for your next task.
  If all your tasks are done or blocked, send a message to the team lead.
```

### Step 4: Coordinate

As team lead, your job is coordination only:

1. **Approve plans** — When the implementer submits a plan via ExitPlanMode, review it and use `SendMessage` with `type: "plan_approval_response"` to approve or reject with feedback.
2. **Relay information** — If one teammate's findings affect another, forward via `SendMessage`.
3. **Resolve conflicts** — If the devils-advocate and implementer disagree, make the call.
4. **Unblock tasks** — When a phase completes, verify all tasks in that phase are done before confirming teammates can proceed.

### Step 5: Completion

When all tasks are complete:

1. Verify the code quality checklist:
   - All new code has tests
   - All tests pass (`go test ./...`)
   - Go vet is clean (`go vet ./...`)
   - Docker container builds successfully (`docker build -t vire-portal:latest .`)
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
| Entry Point | `cmd/portal/` |
| Application | `internal/app/` |
| Configuration | `internal/config/` |
| HTTP Handlers | `internal/handlers/` |
| Storage Interfaces | `internal/interfaces/` |
| HTTP Server | `internal/server/` |
| BadgerDB Storage | `internal/storage/badger/` |
| HTML Templates | `pages/` |
| Template Partials | `pages/partials/` |
| Static Assets | `pages/static/` |
| Docker | `Dockerfile`, `docker/` |
| CI/CD Workflows | `.github/workflows/` |
| Documentation | `docs/`, `README.md` |
| Scripts | `scripts/` |
| Skills | `.claude/skills/` |

### Routes

| Route | Handler | Auth |
|-------|---------|------|
| `GET /` | PageHandler | No |
| `GET /static/*` | PageHandler | No |
| `GET /api/health` | HealthHandler | No |
| `GET /api/version` | VersionHandler | No |

### Configuration

Config priority: defaults < TOML file < env vars (VIRE_ prefix) < CLI flags.

| Setting | Env Var | Default |
|---------|---------|---------|
| Server port | `VIRE_SERVER_PORT` | `8080` |
| Server host | `VIRE_SERVER_HOST` | `localhost` |
| BadgerDB path | `VIRE_BADGER_PATH` | `./data/vire` |
| Log level | `VIRE_LOG_LEVEL` | `info` |
| Log format | `VIRE_LOG_FORMAT` | `text` |

### API Integration

All API calls go through the vire-gateway REST API:
- Auth: JWT in `Authorization: Bearer` header
- Go HTTP client handles cookies and auth headers server-side
- Error responses follow consistent `{ error: { code, message } }` shape
- Token refresh: `POST /api/auth/refresh` (automatic on 401)

### Documentation to Update

When the feature affects user-facing behaviour or API contracts, update:
- `README.md` — if new capabilities, changed routes, or prerequisites
- `docs/requirements.md` — if API contracts or architecture changed
- `.claude/skills/` — affected skill files
