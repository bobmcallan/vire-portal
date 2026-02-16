# Requirements: MCP OAuth Phase 1 — Verify Current Dev Login End-to-End

**Date:** 2026-02-16
**Requested:** Implement Phase 1 from `docs/authentication/mcp-oauth-implementation-steps.md` — verify the existing email/password and OAuth login flows work locally with vire-server, and fix any issues found.

## Scope

**In scope:**
- Verify portal login flow (email/password → vire-server → JWT cookie → dashboard)
- Verify Google/GitHub OAuth redirect chains are correct
- Ensure config is correct for local dev (port 4241 portal, port 4242 vire-server)
- Fix the TOML config gap: `config/vire-portal.toml` is missing `[auth]` section (only the `.example` has it)
- Add integration test that validates the full login round-trip against a mock vire-server
- Verify MCP handler correctly extracts user context from JWT cookie
- Verify `/settings` page shows JWT debug info in dev mode

**Out of scope:**
- MCP OAuth 2.1 endpoints (Phase 2+)
- Bearer token support on /mcp (Phase 6)
- Tunneling / Claude Desktop integration (Phase 8-9)

## Approach

Phase 1 is primarily a verification and hardening pass on existing auth code. The main deliverables are:

1. **Config fix**: Add missing `[auth]` section to `config/vire-portal.toml` (the live config) to match the `.example` file. Currently the live TOML has no auth section, so defaults are used.

2. **Integration test**: Add `internal/handlers/auth_integration_test.go` with a test that spins up a mock vire-server and exercises the full login flow: POST login → receive JWT → cookie set → validate claims. This proves the portal ↔ vire-server handshake works without needing a real vire-server running.

3. **OAuth redirect validation test**: Add test cases that verify the Google/GitHub redirect URLs are constructed correctly and include the callback parameter.

4. **MCP user context test**: Add test in `internal/mcp/` that verifies `withUserContext` correctly extracts `sub` from a JWT cookie and attaches it to the request context.

5. **Manual verification script**: Add `scripts/verify-auth.sh` that curls the health, login, and callback endpoints to validate the running server's auth flow.

## Files Expected to Change

- `config/vire-portal.toml` — add missing `[auth]` section
- `internal/handlers/auth_integration_test.go` — new: end-to-end login flow test
- `internal/mcp/handler_test.go` — new: MCP user context extraction tests
- `scripts/verify-auth.sh` — new: manual verification script
