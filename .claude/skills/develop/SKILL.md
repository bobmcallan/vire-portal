# /vire-portal-develop - Vire Portal Development Workflow

Develop and test Vire portal features using an agent team optimized for Claude models.

## Usage
```
/vire-portal-develop <feature-description>
```

## Model Selection Guide

| Model | Best For | Avoid For |
|-------|----------|-----------|
| **haiku** | Simple reads, file searches, quick validations, repetitive tasks | Complex reasoning, security analysis, multi-file refactors |
| **sonnet** | Most implementation work, code review, testing, documentation | Very complex architectural decisions |
| **opus** | Complex reasoning, security auditing, architectural decisions, stress-testing | Simple tasks (wasteful) |

**Default teammates use sonnet** — good balance of speed and capability. Switch to:
- `haiku` for reviewer's documentation verification tasks
- `opus` for devils-advocate when security is critical

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

**Efficiency tip:** Write specific file paths and line numbers into task descriptions. This saves teammates from searching and reduces token usage.

### Step 3: Create Team and Tasks

Call `TeamCreate`:
```
team_name: "vire-portal-develop"
description: "Developing: <feature-description>"
```

Create tasks across 3–4 phases using `TaskCreate`. Set `blockedBy` dependencies via `TaskUpdate`.
Use 3 phases for backend-only changes. Add **Phase 2b** when the feature touches web pages
(`pages/`, `pages/static/`, `pages/partials/`, CSS, JS, or handler template rendering).

**Phase 1 — Implement** (no dependencies):
- "Write tests and implement <feature>" — owner: implementer
  Task description includes: approach, files to change, test strategy, and acceptance criteria.
  **MANDATORY:** If UI elements are added, removed, or renamed, the implementer MUST update or create corresponding tests in `tests/ui/` as part of this task. The task MUST NOT be marked complete without test files that cover the new/changed UI elements.
- "Review implementation and tests" — owner: reviewer, blockedBy: [implement task]
  Scope: code quality, pattern consistency, test coverage.
- "Stress-test implementation" — owner: devils-advocate, blockedBy: [implement task]
  Scope: security, failure modes, edge cases, hostile inputs.

**Phase 2 — Verify** (blockedBy: review + stress-test):
- "Build, test, and run locally" — owner: implementer
  Run `go test ./...`, `go vet ./...`, then `./scripts/run.sh restart` (rebuilds and restarts; leaves the server running for subsequent verification tasks).
- "Validate running server" — owner: reviewer, blockedBy: [build task]

**Phase 2b — UI Verification** (MANDATORY when web pages changed; blockedBy: build task):
Applies when the feature touches: `pages/`, `pages/static/`, `pages/partials/`, HTML templates, CSS, or JS files.
See `.claude/skills/test-common/SKILL.md` and `.claude/skills/test-execute/SKILL.md` for the full procedure.

**CRITICAL — Test execution is a hard gate. Tasks MUST NOT be marked complete without actual test execution AND captured results.**

- "Review and run UI tests" — owner: test-executor, blockedBy: [build task]
  Follow the `/test-execute` skill procedure:

  **Step 1 — Validate:** Check test files for structural compliance (see `/test-common` mandatory rules).

  **Step 2 — Execute:** Run tests via the wrapper script. **NEVER run `go test` directly** — the wrapper captures output, generates `summary.md`, and collects screenshots.
  ```bash
  # Run individual suites via wrapper script
  ./scripts/ui-test.sh smoke
  ./scripts/ui-test.sh dashboard
  ./scripts/ui-test.sh nav
  ./scripts/ui-test.sh devauth
  ./scripts/ui-test.sh mcp

  # Or run all suites at once
  ./scripts/ui-test.sh all
  ```

  **Step 3 — Report:** Read the results and send them to the team lead.
  ```bash
  # Find latest results
  LATEST=$(ls -td tests/logs/*/ | head -1)
  cat "$LATEST/summary.md"
  ```

  The test-executor MUST:
  1. Send the `summary.md` contents to the team lead
  2. If any suite fails, notify the implementer to fix and re-run
  3. NOT mark this task complete without results in `tests/logs/`

  **DO NOT mark this task complete unless wrapper script execution produced results AND the summary was sent to the team lead.** Marking complete without execution is a workflow violation.

- "Review UI test compliance" — owner: test-creator, blockedBy: [implement task]
  Follow the `/test-create-review` skill procedure:
  - Review test files for selector accuracy against current HTML templates
  - Fix stale selectors before execution
  - Create new tests if UI elements were added

**Phase 3 — Document** (blockedBy: validate, and UI verification if applicable):
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

  ## Key Files to Read First
  - `.claude/skills/develop/SKILL.md` — Reference section for conventions, routes, config
  - Task description in TaskList — contains approach, files, acceptance criteria

  ## Workflow
  1. Read TaskList, claim your tasks (owner: "implementer") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  ## Task Types
  **Implement:** Write tests first, then implement to pass them. Use `go test -run TestName` for targeted testing.
  **Verify:** Run `go test ./...`, `go vet ./...`, then `./scripts/run.sh restart`. Verify with `curl -s http://localhost:${PORTAL_PORT:-8881}/api/health`. Leave server running.
  **UI Verification:** Follow `.claude/skills/test-common/SKILL.md` and `.claude/skills/test-execute/SKILL.md` — review tests for compliance, then execute via `./scripts/ui-test.sh` (NEVER raw `go test`). Read `summary.md` from `tests/logs/{timestamp}/` and send contents to team lead.
  **Documentation:** Update affected files in docs/, README.md, and .claude/skills/.

  ## Communication Rules
  - Do NOT send status messages — use TaskUpdate for completion
  - Message teammates only for: blocking issues, review findings that need fixes, questions
  - Keep messages concise and actionable
```

**reviewer:**
```
name: "reviewer"
subagent_type: "general-purpose"
model: "haiku"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the reviewer on a development team. You review for code quality, pattern consistency, test coverage, and documentation accuracy.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal

  ## Key Files to Read First
  - `.claude/skills/develop/SKILL.md` — Reference section for conventions, routes, config
  - Task description in TaskList — scope and acceptance criteria

  ## Workflow
  1. Read TaskList, claim your tasks (owner: "reviewer") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  ## Review Checklist
  **Code:** Read changed files + surrounding context. Check: bugs, pattern consistency, test coverage, error handling.
  **Docs:** Verify accuracy against implementation, check that examples work.
  **Deployment:** Confirm health endpoint responds (`curl -s http://localhost:${PORTAL_PORT:-8881}/api/health`), test key routes.

  ## Communication Rules
  - Send findings to "implementer" via SendMessage ONLY if fixes are needed
  - Format findings as: file, line, issue, suggested fix
  - Do NOT send status messages — use TaskUpdate for completion
```

**devils-advocate:**
```
name: "devils-advocate"
subagent_type: "general-purpose"
model: "opus"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the devils-advocate on a development team. Your scope: security vulnerabilities, failure modes, edge cases, and hostile inputs.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal

  ## Key Files to Read First
  - `.claude/skills/develop/SKILL.md` — Reference section for conventions, routes, config
  - `.claude/skills/test-common/SKILL.md` — If testing web endpoints
  - Changed files from implementation

  ## Workflow
  1. Read TaskList, claim your tasks (owner: "devils-advocate") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  ## Attack Surface Analysis
  Check these categories systematically:
  - **Input validation:** SQL injection, XSS, path traversal, command injection
  - **Auth flows:** Broken auth, session fixation, CSRF, missing tokens
  - **Error states:** Missing error handling, information leakage, panic recovery
  - **Concurrency:** Race conditions, deadlocks, resource leaks
  - **Edge cases:** Empty inputs, max values, unicode, special characters, nil/null

  Write stress tests where appropriate. Think like an attacker.

  ## Communication Rules
  - Send findings to "implementer" via SendMessage ONLY if fixes are needed
  - Format findings as: severity (critical/high/medium/low), location, issue, exploit scenario, fix
  - Do NOT send status messages — use TaskUpdate for completion
```

**test-executor:**
```
name: "test-executor"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the test-executor on a development team. You run UI tests and report results.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal

  ## Key Files to Read First
  - `.claude/skills/test-common/SKILL.md` — Mandatory rules and infrastructure docs
  - `.claude/skills/test-execute/SKILL.md` — Execution workflow
  - Task description in TaskList — scope and acceptance criteria

  ## Workflow
  1. Read TaskList, claim your tasks (owner: "test-executor") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  ## Execution Rules
  - **NEVER modify test files** — this role is read-only
  - Always use `./scripts/ui-test.sh` for suite execution (never raw `go test`)
  - After execution, read `summary.md` and send contents to the team lead
  - Check `tests/logs/{timestamp}/container.log` for container-level errors
  - If tests fail, notify the implementer with failure details

  ## Communication Rules
  - Send test results to team lead via SendMessage after each execution
  - Message "implementer" only if failures require code fixes
  - Do NOT send status messages — use TaskUpdate for completion
```

**test-creator:**
```
name: "test-creator"
subagent_type: "general-purpose"
model: "sonnet"
team_name: "vire-portal-develop"
run_in_background: true
prompt: |
  You are the test-creator on a development team. You create and review UI tests.

  Team: "vire-portal-develop". Working directory: /home/bobmc/development/vire-portal

  ## Key Files to Read First
  - `.claude/skills/test-common/SKILL.md` — Mandatory rules and infrastructure docs
  - `.claude/skills/test-create-review/SKILL.md` — Test creation and review workflow
  - Task description in TaskList — scope and acceptance criteria

  ## Workflow
  1. Read TaskList, claim your tasks (owner: "test-creator") by setting status to "in_progress"
  2. Work through tasks in ID order, mark each completed before moving to the next
  3. After each task, check TaskList for your next available task

  ## Task Types
  - **Review:** Check test files for selector accuracy against current HTML templates in `pages/`. Fix stale selectors.
  - **Create:** Scaffold new test files using the template from `/test-create-review`. Ensure compliance with `/test-common` rules.
  - **Audit:** Report compliance issues without making changes.

  ## Communication Rules
  - Send findings to "implementer" only if test code needs fixes
  - Do NOT send status messages — use TaskUpdate for completion
```

### Step 5: Coordinate

As team lead, your job is lightweight coordination:

1. **Relay information** — If one teammate's findings affect another, forward via `SendMessage`.
2. **Resolve conflicts** — If the devils-advocate and implementer disagree, make the call.
3. **Apply direct fixes** — For trivial issues (typos, missing imports), fix them directly rather than round-tripping through the implementer.

## Token Efficiency Tips

When working with Claude models, reduce context usage:

| Technique | How |
|-----------|-----|
| **Read selectively** | Use `offset` and `limit` in Read tool for large files |
| **Search first** | Use Grep to find relevant sections before reading entire files |
| **Task descriptions** | Include only essential context; teammates read files directly |
| **Avoid duplication** | Don't repeat information across task descriptions |
| **Parallel reads** | Read multiple small files in one message, not sequentially |
| **Summarize early** | Write findings to files, don't keep re-reading same content |

**For teammates:** Read the task description, then read only the files mentioned. Don't re-explore the codebase — the lead already did that in Step 2.

### Step 6: Completion

When all tasks are complete:

1. Verify the code quality checklist (team lead MUST verify each item, not trust task status alone):
   - All new code has tests
   - All tests pass (`go test ./...`) — verified by reviewing actual command output
   - Go vet is clean (`go vet ./...`)
   - Server builds and runs (`./scripts/run.sh restart`) — leave it running
   - Health endpoint responds (`curl -s http://localhost:${PORTAL_PORT:-8881}/api/health`)
   - Script validation passes (`./scripts/test-scripts.sh`)
   - **If web pages changed: UI tests MUST have been executed via `./scripts/ui-test.sh`** (never raw `go test`). The team lead must confirm test execution occurred by checking `tests/logs/` for a timestamp directory created during this session containing `summary.md`, `{suite}.log`, and `container.log` files. If no test results exist, the test-executor must re-run via wrapper script before completion.
   - README.md updated if user-facing behaviour changed
   - API contract documentation matches implementation
   - Devils-advocate has signed off
   - Server is left running after completion

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
| `GET /mcp-info` | MCPPageHandler | No |
| `GET /static/*` | PageHandler | No |
| `POST /mcp` | MCPHandler | Bearer token or session cookie |
| `GET /api/health` | HealthHandler | No |
| `GET /api/server-health` | ServerHealthHandler | No |
| `GET /api/version` | VersionHandler | No |
| `POST /api/auth/login` | AuthHandler | No (forwards to vire-server) |
| `POST /api/auth/logout` | AuthHandler | No |
| `GET /api/auth/login/google` | AuthHandler | No (redirects to vire-server) |
| `GET /api/auth/login/github` | AuthHandler | No (redirects to vire-server) |
| `GET /auth/callback` | AuthHandler | No (OAuth callback, sets session cookie or completes MCP flow) |
| `POST /api/shutdown` | Server | No (dev mode only, 403 in prod) |
| `GET /settings` | SettingsHandler | No |
| `POST /settings` | SettingsHandler | No (requires session cookie) |

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

Future gateway integration (deferred):
- Auth: JWT in `Authorization: Bearer` header
- Error responses follow consistent `{ error: { code, message } }` shape
- Token refresh: `POST /api/auth/refresh` (automatic on 401)

### Documentation to Update

When the feature affects user-facing behaviour or API contracts, update:
- `README.md` — if new capabilities, changed routes, or prerequisites
- `docs/requirements.md` — if API contracts or architecture changed
- `.claude/skills/` — affected skill files

## Claude-Specific Patterns

These patterns improve reliability when working with Claude models:

### Prompt Structure
Good prompts for teammates follow this structure:
1. **Role** — Clear statement of what they are
2. **Context** — Team name, working directory, key files to read
3. **Workflow** — Numbered steps for their tasks
4. **Task-specific guidance** — What to do for each task type
5. **Communication rules** — When and how to message teammates

### Avoid These Anti-Patterns
- ❌ Vague instructions like "do your best" or "be thorough"
- ❌ Long prose without structure — use headers and lists
- ❌ Repeating the same information multiple times
- ❌ Asking teammates to "explore and understand" — lead does this in Step 2

### Leverage Claude Strengths
- ✅ **Pattern matching** — Claude excels at finding similar code patterns
- ✅ **Code review** — Good at catching inconsistencies and missing edge cases
- ✅ **Security analysis** — Opus especially good at identifying vulnerabilities
- ✅ **Structured output** — Ask for specific formats (tables, checklists, severity ratings)

### Team Communication
Teammates should message each other only when:
- Blocking issue discovered that prevents progress
- Review found issues requiring fixes
- Clarification needed on requirements

All other updates go through TaskUpdate status changes — the system handles notifications automatically.
