# Requirements: Build Vire Portal with 80s Black & White Design

**Date:** 2026-02-14
**Requested:** Build the entire Vire portal frontend (6 pages, auth, API integration) with an 80s-inspired black and white design aesthetic based on noeinoi.com reference.

## Scope

### In Scope
- Full project scaffolding (package.json, Vite, TypeScript, Preact, Tailwind CSS)
- All 6 pages: Landing, OAuth Callback, Dashboard, Settings, Connect, Billing
- Core modules: api.ts, auth.ts, router.ts, state.ts, main.ts
- Reusable components: layout, key-input, usage-chart, copy-block
- Docker + nginx configuration
- 80s black & white design: monochrome palette, high contrast, retro typography, minimal decoration, brutalist/geometric aesthetic
- All API integration with vire-gateway (fetch wrapper, JWT, refresh tokens)
- OAuth sign-in flow (Google + GitHub via gateway redirects)
- BYOK key management UI
- MCP config copy-to-clipboard
- Stripe billing integration
- Tests for all features

### Out of Scope
- Backend/gateway implementation
- Actual OAuth provider registration
- Stripe account setup
- Cloud Run deployment
- CI/CD pipeline (release.yml already exists)

## Design Direction

**Reference:** https://noeinoi.com/ — 80s black and white aesthetic

**Design Principles:**
- **Palette:** Pure black (#000) and white (#fff) with minimal grey accents
- **Typography:** Monospace or bold geometric sans-serif, retro feel
- **Layout:** Clean grid-based, generous whitespace, strong visual hierarchy
- **Decoration:** Minimal — no gradients, no shadows, no rounded corners. Hard edges, solid borders
- **Interactions:** Simple hover states with color inversion (black→white, white→black)
- **Overall:** Brutalist-inspired, high contrast, function-first, retro terminal aesthetic

## Approach

Build the entire SPA from scratch following the docs/requirements.md specification:
1. Scaffold project with Vite + Preact + TypeScript + Tailwind
2. Implement core infrastructure (router, API client, auth, state)
3. Build all 6 pages with 80s B&W design
4. Create reusable components
5. Configure Docker + nginx for Cloud Run deployment
6. Write tests for all features
7. Verify build and Docker image

## Files Expected to Change
- `package.json` (new)
- `tsconfig.json` (new)
- `vite.config.ts` (new)
- `tailwind.config.js` (new)
- `postcss.config.js` (new)
- `index.html` (new, root level for Vite)
- `nginx.conf` (new)
- `Dockerfile` (new)
- `.version` (new)
- `.env` (new, for local dev)
- `src/main.ts` (new)
- `src/api.ts` (new)
- `src/auth.ts` (new)
- `src/router.ts` (new)
- `src/state.ts` (new)
- `src/pages/*.tsx` (6 new page files)
- `src/components/*.tsx` (4+ new component files)
- `src/styles/main.css` (new)
- `.gitignore` (update for Node.js project)
