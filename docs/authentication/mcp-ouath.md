# Vire Portal — MCP OAuth Integration Guide

> Reference implementation for [bobmcallan/vire-portal](https://github.com/bobmcallan/vire-portal)
> Vire Portal for user interaction / setup with Vire MCP service

---

## Overview

Vire Portal serves as the authentication and user management layer for the Vire MCP server. It enables Claude Desktop, Claude Code, and claude.ai to securely connect to Vire's stock intelligence tools via the Model Context Protocol (MCP) with OAuth 2.1 authentication.

Users sign in to Vire using third-party identity providers (Google, GitHub), and Vire issues its own MCP tokens that Claude uses for all subsequent tool calls. This document covers how the OAuth flow works, what Vire needs to implement, and how it integrates with each Claude surface.

---

## Architecture

```
┌──────────────┐     ┌──────────────────┐     ┌─────────────────┐
│              │     │                  │     │                 │
│  Claude      │────▶│  Vire Portal     │────▶│  Google/GitHub  │
│  (MCP Client)│◀────│  (MCP + OAuth    │◀────│  (Identity      │
│              │     │   Server)        │     │   Provider)     │
└──────────────┘     └──────────────────┘     └─────────────────┘
                              │
                              ▼
                     ┌──────────────────┐
                     │  Vire MCP        │
                     │  Server          │
                     │  (Stock Tools)   │
                     └──────────────────┘
```

From Claude's perspective, **Vire is both the Resource Server and the Authorization Server**. Claude never sees Google or GitHub directly. The third-party identity provider is an internal implementation detail of Vire.

---

## The Two-Hop OAuth Flow

There are two distinct OAuth exchanges happening:

1. **Claude ↔ Vire** (MCP OAuth 2.1) — Claude authenticates with Vire to get an MCP access token
2. **Vire ↔ Google/GitHub** (Standard OAuth 2.0) — Vire delegates user authentication to the identity provider

### Sequence Diagram

```
Claude (MCP Client)          Vire Portal              Google/GitHub
       │                          │                         │
       │  1. GET /.well-known/    │                         │
       │     oauth-authorization- │                         │
       │     server               │                         │
       │─────────────────────────▶│                         │
       │  metadata response       │                         │
       │◀─────────────────────────│                         │
       │                          │                         │
       │  2. POST /register (DCR) │                         │
       │─────────────────────────▶│                         │
       │  client_id + secret      │                         │
       │◀─────────────────────────│                         │
       │                          │                         │
       │  3. Redirect user to     │                         │
       │     /authorize           │                         │
       │─────────────────────────▶│                         │
       │                          │  4. Redirect to         │
       │                          │     Google/GitHub       │
       │                          │     /authorize          │
       │                          │────────────────────────▶│
       │                          │                         │
       │                          │     User authenticates  │
       │                          │     with Google/GitHub   │
       │                          │                         │
       │                          │  5. Callback with       │
       │                          │     auth code           │
       │                          │◀────────────────────────│
       │                          │                         │
       │                          │  6. Exchange code for   │
       │                          │     Google/GitHub token  │
       │                          │────────────────────────▶│
       │                          │  access_token           │
       │                          │◀────────────────────────│
       │                          │                         │
       │                          │  7. Lookup/create user  │
       │                          │     in Vire DB          │
       │                          │                         │
       │  8. Redirect to Claude   │                         │
       │     callback with Vire   │                         │
       │     auth code            │                         │
       │◀─────────────────────────│                         │
       │                          │                         │
       │  9. POST /token          │                         │
       │     exchange code for    │                         │
       │     Vire MCP token       │                         │
       │─────────────────────────▶│                         │
       │  Vire access_token       │                         │
       │◀─────────────────────────│                         │
       │                          │                         │
       │  10. MCP tool calls      │                         │
       │      (Bearer token)      │                         │
       │─────────────────────────▶│                         │
       │  tool results            │                         │
       │◀─────────────────────────│                         │
```

### Step-by-Step Breakdown

| Step | Action | Detail |
|------|--------|--------|
| 1 | **Metadata Discovery** | Claude fetches `/.well-known/oauth-authorization-server` to learn Vire's OAuth endpoints |
| 2 | **Dynamic Client Registration** | Claude registers itself as an OAuth client via `/register` (DCR per RFC 7591) |
| 3 | **Authorization Request** | Claude redirects the user's browser to Vire's `/authorize` endpoint with PKCE challenge |
| 4 | **Identity Provider Redirect** | Vire stores the MCP session state and redirects the user to Google/GitHub for login |
| 5 | **IdP Callback** | Google/GitHub redirects back to Vire's `/auth/callback` with an authorization code |
| 6 | **Token Exchange (IdP)** | Vire exchanges the code with Google/GitHub for their access token and user info |
| 7 | **User Resolution** | Vire looks up the user by Google sub / GitHub ID. Creates the account if it's a new user |
| 8 | **MCP Authorization Code** | Vire generates its own authorization code and redirects back to Claude's callback URL |
| 9 | **Token Exchange (MCP)** | Claude exchanges Vire's auth code for a Vire-issued MCP access token |
| 10 | **Tool Calls** | Claude sends the Vire token as a Bearer token on every MCP tool call |

---

## User Signup Flow

Users do **not** need to create a Vire account manually before using the MCP connector. The recommended flow is:

1. User adds Vire as a connector in Claude (Settings → Connectors → paste URL)
2. Claude initiates OAuth → redirects to Vire → Vire redirects to Google/GitHub
3. User logs in with their Google/GitHub account
4. On first login, Vire **auto-creates** the user record:
   - Stores the identity provider ID (Google `sub` claim or GitHub `id`)
   - Stores email, display name from the IdP profile
   - Creates a default portfolio configuration
   - Assigns default permissions/tier
5. On subsequent logins, Vire matches the IdP identity to the existing user record

### User Identity Mapping (Vire DB)

```
┌─────────────────────────────────────────────────┐
│ users                                           │
├────────────┬────────────────────────────────────┤
│ id         │ uuid (primary key)                 │
│ email      │ user@example.com                   │
│ name       │ display name from IdP              │
│ provider   │ "google" | "github"                │
│ provider_id│ Google sub / GitHub user ID         │
│ created_at │ timestamp                          │
│ tier       │ "free" | "pro" | "enterprise"      │
└────────────┴────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│ oauth_clients (DCR registrations)               │
├────────────┬────────────────────────────────────┤
│ client_id  │ uuid (issued to Claude instances)  │
│ client_name│ "Claude" / "claude-code" etc.      │
│ redirect_uri│ Claude's callback URL             │
│ created_at │ timestamp                          │
└────────────┴────────────────────────────────────┘

┌─────────────────────────────────────────────────┐
│ tokens                                          │
├────────────┬────────────────────────────────────┤
│ access_token│ opaque or JWT                     │
│ user_id    │ FK → users.id                      │
│ client_id  │ FK → oauth_clients.client_id       │
│ scopes     │ "portfolio:read tools:invoke"       │
│ expires_at │ timestamp                          │
│ refresh_token│ for token renewal                │
└────────────┴────────────────────────────────────┘
```

---

## Required Endpoints

Vire Portal needs to expose these HTTP endpoints:

### OAuth Discovery & Registration

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/.well-known/oauth-authorization-server` | GET | Returns OAuth metadata (issuer, endpoints, supported grant types, PKCE methods) |
| `/.well-known/oauth-protected-resource` | GET | Returns resource server metadata |
| `/register` | POST | Dynamic Client Registration — Claude registers itself and receives a `client_id` |

### OAuth Flow

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/authorize` | GET | Starts the OAuth flow. Validates PKCE, stores session state, redirects to Google/GitHub |
| `/auth/callback/google` | GET | Receives callback from Google after user login |
| `/auth/callback/github` | GET | Receives callback from GitHub after user login |
| `/token` | POST | Exchanges authorization code for Vire access token. Also handles `refresh_token` grant |

### MCP Transport

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/mcp` | POST | Streamable HTTP MCP endpoint. Validates Bearer token, routes to tool handlers |

### User Portal (Web UI)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/` | GET | Landing page / dashboard |
| `/settings` | GET | User settings, portfolio configuration, API key management |
| `/connect` | GET | Instructions for connecting to Claude Desktop / Code |

---

## OAuth Metadata Response

`GET /.well-known/oauth-authorization-server` must return:

```json
{
  "issuer": "https://portal.vire.dev",
  "authorization_endpoint": "https://portal.vire.dev/authorize",
  "token_endpoint": "https://portal.vire.dev/token",
  "registration_endpoint": "https://portal.vire.dev/register",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "code_challenge_methods_supported": ["S256"],
  "token_endpoint_auth_methods_supported": ["client_secret_post", "none"],
  "scopes_supported": ["openid", "portfolio:read", "portfolio:write", "tools:invoke"]
}
```

---

## Claude Client Differences

Each Claude surface handles the OAuth flow slightly differently:

### Claude Code CLI

- Performs full DCR (calls `/register` first)
- Opens browser for user authorization
- Listens on a local callback port (e.g., `http://localhost:PORT/callback`)
- Stores tokens locally in `~/.claude.json`
- Works with the full MCP OAuth spec

```bash
claude mcp add --transport http vire https://portal.vire.dev/mcp
# Browser opens → user authenticates → token stored
```

### Claude Desktop (Connectors UI)

- Added via Settings → Connectors → "Add custom connector"
- May skip DCR and use its own pre-existing `client_id`
- Callback URL: `https://claude.ai/api/mcp/auth_callback`
- If DCR is not supported, users can specify custom Client ID and Client Secret in "Advanced settings"
- Stores tokens per-connector

**Known issue:** Some OAuth implementations that work with Claude Code fail on Claude Desktop due to differences in the DCR flow. Claude.ai may send a `client_id` that your server hasn't registered. To handle this, Vire should either:
- Support DCR and accept unknown `client_id` values via the `/register` endpoint
- Accept Claude's pre-registered `client_id` (available when adding via Connectors with "Advanced settings")

### claude.ai (Web)

- Same as Claude Desktop Connectors
- OAuth callback redirects back to `https://claude.ai/api/mcp/auth_callback`
- Supports the 3/26 and 6/18 MCP auth specs

---

## Implementation Notes for Vire Portal (Go)

### Project Structure (aligned with current repo layout)

```
vire-portal/
├── cmd/                    # Application entrypoints
│   └── portal/             # Main server binary
├── internal/
│   ├── auth/               # OAuth server implementation
│   │   ├── discovery.go    # .well-known endpoints
│   │   ├── dcr.go          # Dynamic Client Registration
│   │   ├── authorize.go    # /authorize handler
│   │   ├── token.go        # /token handler
│   │   └── providers/      # Google, GitHub OAuth client logic
│   │       ├── google.go
│   │       └── github.go
│   ├── mcp/                # MCP transport + tool registration
│   │   ├── handler.go      # Streamable HTTP handler
│   │   └── middleware.go   # Bearer token validation
│   ├── user/               # User management
│   │   ├── store.go        # User CRUD
│   │   └── model.go        # User, Token, Client models
│   └── portal/             # Web UI handlers
├── config/                 # Configuration files
├── docker/                 # Container definitions
├── pages/                  # Web UI templates
├── scripts/                # Build and deployment scripts
├── tests/                  # Integration tests
└── docs/                   # Documentation
```

### Key Implementation Considerations

**PKCE (Proof Key for Code Exchange):** Claude uses S256 PKCE for all authorization requests. Vire must validate the `code_challenge` on `/authorize` and verify the `code_verifier` on `/token`.

**Token Format:** Either opaque tokens (stored in DB, looked up on each request) or JWTs (self-contained, validated by signature). JWTs are simpler for the MCP handler since it doesn't need a DB call per tool invocation.

**Session State Management:** When Vire redirects to Google/GitHub, it needs to preserve the original MCP OAuth state (Claude's `redirect_uri`, `code_challenge`, `state`). Store this in a short-lived session (Redis, in-memory, or encrypted cookie).

**Tool Annotations:** Every MCP tool must include `readOnlyHint` or `destructiveHint` annotations. This is required for the Claude Connector Directory and accounts for 30% of directory submission rejections.

**Transport:** Build for Streamable HTTP (`POST /mcp`). SSE support is being deprecated.

---

## Environment Configuration

```env
# Vire Portal
VIRE_PORTAL_URL=https://portal.vire.dev
VIRE_MCP_URL=https://portal.vire.dev/mcp

# Google OAuth (console.cloud.google.com → Credentials)
GOOGLE_CLIENT_ID=xxxx.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=GOCSPX-xxxx
GOOGLE_CALLBACK_URL=https://portal.vire.dev/auth/callback/google

# GitHub OAuth (github.com/settings/developers → OAuth Apps)
GITHUB_CLIENT_ID=Iv1.xxxx
GITHUB_CLIENT_SECRET=xxxx
GITHUB_CALLBACK_URL=https://portal.vire.dev/auth/callback/github

# Database
DATABASE_URL=postgres://user:pass@localhost:5432/vire_portal

# Token signing (for JWT approach)
JWT_SIGNING_KEY=xxxx
TOKEN_EXPIRY=3600        # 1 hour
REFRESH_TOKEN_EXPIRY=604800  # 7 days
```

---

## Connector Directory Submission

To list Vire in the public directory at [claude.com/connectors](https://claude.com/connectors):

### Requirements Checklist

- [ ] OAuth 2.1 authentication with DCR support
- [ ] `readOnlyHint` / `destructiveHint` on all MCP tools
- [ ] Minimum 3 working examples demonstrating core functionality
- [ ] README with description, features, setup, examples
- [ ] Privacy policy URL
- [ ] Data processing terms / DPA link
- [ ] Support contact (email, docs link, issue tracker)
- [ ] Streamable HTTP transport
- [ ] Compliant with [Anthropic MCP Directory Policy](https://support.claude.com/en/articles/11697096-anthropic-mcp-directory-policy)

### Submission

Submit via the [MCP Directory Submission Form](https://docs.google.com/forms/d/e/1FAIpQLSeafJF2NDI7oYx1r8o0ycivCSVLNq92Mpc1FPxMKSw1CzDkqA/viewform).

### Example Tool Annotations

```json
{
  "name": "get_portfolio",
  "description": "Get current portfolio holdings with values, weights, and gains",
  "annotations": {
    "readOnlyHint": true,
    "destructiveHint": false
  },
  "inputSchema": {
    "type": "object",
    "properties": {
      "portfolio_name": {
        "type": "string",
        "description": "Name of the portfolio (e.g., 'SMSF')"
      }
    }
  }
}
```

---

## Testing

### MCP Inspector

Test your OAuth flow and tool definitions before connecting to Claude:

```bash
npx @modelcontextprotocol/inspector
```

In the inspector UI:
1. Set transport to "Streamable HTTP"
2. Enter `https://portal.vire.dev/mcp`
3. Click "Open Auth Settings" → "Quick OAuth Flow"
4. Complete the Google/GitHub login
5. Verify tools appear and execute correctly

### Claude Code CLI

```bash
# Add Vire as an MCP server
claude mcp add --transport http vire https://portal.vire.dev/mcp

# Verify connection
claude mcp list

# Test in conversation
claude "Show me my SMSF portfolio"
```

### Claude Desktop / claude.ai

1. Settings → Connectors → Add custom connector
2. URL: `https://portal.vire.dev/mcp`
3. Complete OAuth flow in browser popup
4. Enable the connector in a conversation via "+" → Connectors

---

## References

- [MCP OAuth Authorization Spec](https://spec.modelcontextprotocol.io/specification/2025-06-18/basic/authorization/)
- [Building Custom Connectors (Anthropic)](https://support.claude.com/en/articles/11503834-building-custom-connectors-via-remote-mcp-servers)
- [Remote MCP Server Submission Guide](https://support.claude.com/en/articles/12922490-remote-mcp-server-submission-guide)
- [Anthropic Connectors Directory FAQ](https://support.claude.com/en/articles/11596036-anthropic-connectors-directory-faq)
- [MCP TypeScript SDK](https://github.com/modelcontextprotocol/typescript-sdk)
- [MCP Python SDK](https://github.com/modelcontextprotocol/python-sdk)
- [Cloudflare MCP OAuth Example](https://developers.cloudflare.com/agents/guides/remote-mcp-server/)
- [Auth0 MCP Integration Guide](https://auth0.com/blog/an-introduction-to-mcp-and-authorization/)