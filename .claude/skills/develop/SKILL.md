# /vire-portal-develop - Vire Portal Development Workflow

Develop and test Vire portal frontend features using an agent team.

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

Example: `.claude/workdir/20260214-1430-oauth-callback/`

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
  - Tech stack: TypeScript, Preact, Vite, Tailwind CSS
  - Dev server: npm run dev
  - Build: npm run build (TypeScript check + Vite build)
  - Tests: npm test (vitest, 118 tests across 13 files)
  - Docker build: docker build -t vire-portal:latest .
  - Linting: npm run lint (ESLint 9 flat config with typescript-eslint)
  - The portal is a static SPA served via nginx in Docker on Cloud Run
  - Design: 80s B&W aesthetic -- IBM Plex Mono, no rounded corners, no shadows, no colours
  - All backend calls go through the vire-gateway REST API
  - Auth: direct OAuth (Google/GitHub) via gateway, JWT in memory, refresh via httpOnly cookie
  - Environment config: injected at runtime via nginx envsubst (/config.json endpoint)

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
  - For TypeScript/Preact: check component structure, hook usage, type safety
  - For Tailwind: check responsive design, consistent spacing/colour usage
  - For API calls: check error handling, auth token management, CORS (credentials: 'include')
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
  - Stress-test implementation: XSS vulnerabilities? Token leakage? Broken auth flows? Missing error states?
  - For Preact components: Accessibility issues? Missing loading/error states? Memory leaks from subscriptions?
  - For API integration: What if the gateway is down? What if tokens expire mid-operation? Race conditions in concurrent requests?
  - For OAuth: CSRF risks? Open redirect vulnerabilities? State parameter validation?
  - For Docker/nginx: Security headers missing? CSP policy? Cache invalidation issues?
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
   - All tests pass
   - TypeScript compiles without errors (`npm run build`)
   - Docker container builds successfully (`docker build -t vire-portal:latest .`)
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
| Source Code | `src/` |
| Pages | `src/pages/` |
| Components | `src/components/` |
| Styles | `src/styles/` |
| Public Assets | `public/` |
| Docker | `Dockerfile`, `nginx.conf` |
| CI/CD Workflows | `.github/workflows/` |
| Documentation | `docs/` |
| Skills | `.claude/skills/` |

### Pages

| Route | File | Auth |
|-------|------|------|
| `/` | `src/pages/landing.tsx` | No |
| `/auth/callback` | `src/pages/callback.tsx` | No |
| `/dashboard` | `src/pages/dashboard.tsx` | Yes |
| `/settings` | `src/pages/settings.tsx` | Yes |
| `/connect` | `src/pages/connect.tsx` | Yes |
| `/billing` | `src/pages/billing.tsx` | Yes |

### API Integration

All API calls go through the vire-gateway REST API:
- Base URL configured via `API_URL` environment variable
- Auth: JWT in `Authorization: Bearer` header
- All fetch calls must include `credentials: 'include'` for httpOnly cookie
- Error responses follow consistent `{ error: { code, message } }` shape
- Token refresh: `POST /api/auth/refresh` (automatic on 401)

### Documentation to Update

When the feature affects user-facing behaviour or API contracts, update:
- `README.md` — if new capabilities, changed pages, or prerequisites
- `docs/requirements.md` — if API contracts or architecture changed
- `.claude/skills/` — affected skill files
