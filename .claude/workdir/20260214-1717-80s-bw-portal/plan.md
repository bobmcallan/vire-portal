# Implementation Plan: Vire Portal

## 1. Project Scaffolding

### Dependencies (package.json)

**Runtime:**
- `preact` ^10.25 - UI framework (~3KB gzipped)
- `preact-iso` ^2.7 - Official Preact router + lazy loading (lightweight, maintained by Preact team, includes `useRoute`, `useLocation`, lazy components)

**Dev:**
- `typescript` ^5.7
- `vite` ^6.1
- `@preact/preset-vite` ^2.9 - Preact plugin for Vite (JSX transform, prefresh HMR)
- `tailwindcss` ^4.0 - CSS framework (v4 uses Vite plugin, no PostCSS config needed)
- `@tailwindcss/vite` ^4.0 - Tailwind Vite plugin
- `vitest` ^3.0 - Test runner (Vite-native)
- `@testing-library/preact` ^3.2 - Component testing
- `jsdom` ^26.0 - DOM environment for tests
- `eslint` ^9.0 - Linter
- `eslint-plugin-preact` - Preact-specific lint rules (if available, otherwise skip)

### Why preact-iso for routing

- Official Preact router, maintained by the Preact team
- Built-in lazy loading with `lazy()`
- History-mode routing (matches SPA nginx config)
- ~1.5KB gzipped, purpose-built for Preact
- Simpler API than alternatives (wouter, preact-router)
- Includes `useRoute()` and `useLocation()` hooks

### State Management

No external state library. Use a simple pub/sub store in `state.ts`:
- Module-scoped state object holding `user`, `jwt`, `config`
- `subscribe(listener)` / `getState()` / `setState(partial)` pattern
- Custom `useAppState(selector)` hook that subscribes Preact components
- JWT stored in module-scoped variable (never localStorage)
- Config loaded from `/config.json` at startup

This is sufficient for 6 pages with shared auth state. No need for Redux/Zustand/signals.

## 2. Tailwind v4 Theme Configuration (80s B&W Aesthetic)

Tailwind v4 uses CSS-first configuration. Theme defined in `src/styles/main.css`:

```css
@import "tailwindcss";

@theme {
  --color-primary: #000;
  --color-secondary: #fff;
  --color-accent: #888;
  --color-border: #000;
  --color-error: #000;
  --color-success: #000;

  --font-mono: 'IBM Plex Mono', 'Space Mono', ui-monospace, monospace;
  --font-sans: 'IBM Plex Mono', 'Space Mono', ui-monospace, monospace;

  --radius-none: 0;
  --shadow-none: none;
}
```

**Key design rules:**
- All `border-radius: 0` (no rounded corners)
- All `box-shadow: none` (no shadows)
- No gradients anywhere
- Monospace font everywhere (IBM Plex Mono via Google Fonts, Space Mono fallback)
- Borders: solid 1-2px black
- Hover: invert colors (bg-black text-white <-> bg-white text-black)
- Focus states: thick black outline (2-3px solid black)
- Buttons: black bg, white text, border, invert on hover
- Inputs: black border, no rounded corners, monospace font
- Links: underlined, invert on hover

## 3. Component Architecture

### Layout (src/components/layout.tsx)
- Top nav bar: black background, white text, site title "VIRE" left-aligned
- Nav links: white text, uppercase, spaced, invert on hover
- Active link: underlined or inverted
- Footer: minimal, version info, copyright
- Content area: max-width container, generous padding
- Mobile: hamburger menu (simple toggle, no animation)

### Pages

**Landing (/):**
- Large "VIRE" title, tagline in monospace
- Two sign-in buttons (Google, GitHub) as `<a>` tags pointing to gateway
- Brief feature list in grid layout
- High contrast hero section

**OAuth Callback (/auth/callback):**
- Extracts `code` and `state` from URL query params
- POSTs to gateway `/api/auth/callback`
- Shows loading state ("AUTHENTICATING...")
- On success: stores JWT, redirects to /dashboard
- On error: shows error, link to retry

**Dashboard (/dashboard):**
- Usage stats: requests count, quota bar (pure CSS, no SVG needed)
- Daily trend: simple ASCII-style bar chart or pure CSS bars
- Top endpoints: table with monospace styling
- Instance status: running/stopped indicator
- Plan badge

**Settings (/settings):**
- Profile info section (read-only: email, name, avatar)
- API keys section: 3 key-input components (EODHD, Navexa, Gemini)
- Preferences: portfolio selector, exchange selector
- Delete account button (with confirmation)

**Connect (/connect):**
- MCP proxy status
- Claude Code config block with copy button
- Claude Desktop config block with copy button
- URL regeneration option

**Billing (/billing):**
- Current plan display (Free/Pro)
- Upgrade button -> Stripe checkout
- Manage subscription button -> Stripe billing portal
- Check for `session_id` query param on load (post-checkout)

### Reusable Components

**key-input.tsx:**
- States: empty, configured (masked ****last4), connected, invalid
- Input field + action button
- Error display

**usage-chart.tsx:**
- Quota bar: pure CSS width percentage
- Daily bars: CSS flex with height percentage
- Monochrome styling

**copy-block.tsx:**
- Pre-formatted code block
- Copy button with "COPIED" feedback
- Monospace font, bordered

## 4. Core Modules

### api.ts
- `createApiClient(config)` - factory that returns typed fetch wrapper
- Attaches `Authorization: Bearer <jwt>` header
- Sets `credentials: 'include'` on all requests
- Auto-refreshes JWT on 401 response (calls `/api/auth/refresh`)
- Retries original request after successful refresh
- On refresh failure: clears state, redirects to `/`
- Typed methods for each endpoint: `getProfile()`, `updateProfile()`, `getUsage()`, etc.

### auth.ts
- `handleCallback(code, state)` - POST to gateway callback endpoint
- `refreshToken()` - POST to gateway refresh endpoint
- `logout()` - POST to gateway logout, clear state, redirect
- `isAuthenticated()` - check if JWT exists and not expired
- `getJwt()` - return current JWT
- JWT expiry detection: decode base64 payload, check `exp` claim

### router.ts
- Uses `preact-iso` Router component
- Route definitions with lazy loading
- Auth guard: redirects to `/` if not authenticated for protected routes
- Public routes: `/`, `/auth/callback`
- Protected routes: `/dashboard`, `/settings`, `/connect`, `/billing`

### state.ts
- App state type: `{ user: User | null, jwt: string | null, config: Config | null }`
- `subscribe()`, `getState()`, `setState()`
- `useAppState(selector)` hook
- `initializeApp()` - fetch `/config.json`, attempt token refresh

### main.ts
- Entry point
- Loads config from `/config.json`
- Attempts session restore via token refresh
- Renders app with Router

## 5. Build Configuration

### vite.config.ts
- Preact preset plugin
- Tailwind v4 Vite plugin
- Build target: `es2020` (modern browsers)
- Output to `dist/`
- Define `VITE_APP_VERSION`, `VITE_APP_BUILD`, `VITE_APP_COMMIT` from env

### tsconfig.json
- Target: ES2020
- Module: ESNext
- JSX: react-jsx with jsxImportSource "preact"
- Strict mode
- Path aliases: none (keep it simple)

### index.html (root level for Vite)
- Minimal HTML shell
- Link to Google Fonts (IBM Plex Mono)
- Mount point `<div id="app">`
- Script tag pointing to `src/main.ts`

## 6. Docker + Nginx

### Dockerfile
- Exactly as specified in requirements.md
- Multi-stage: node:20-alpine builder + nginx:1.27-alpine runtime
- Build args: VERSION, BUILD, GIT_COMMIT
- NGINX_ENVSUBST_FILTER for API_URL|DOMAIN

### nginx.conf
- Exactly as specified in requirements.md
- SPA routing (try_files -> index.html)
- /config.json endpoint with envsubst
- /assets/ caching
- /health endpoint

## 7. Testing Strategy

### Unit Tests (vitest)
- `api.test.ts` - fetch wrapper, JWT attachment, 401 refresh flow, error handling
- `auth.test.ts` - callback handling, token refresh, JWT decoding, expiry detection
- `state.test.ts` - state management, subscribe/unsubscribe, useAppState hook
- `router.test.ts` - route matching, auth guards

### Component Tests (@testing-library/preact)
- `landing.test.tsx` - renders sign-in buttons with correct hrefs
- `callback.test.tsx` - extracts query params, calls auth handler
- `dashboard.test.tsx` - displays usage data, status indicators
- `settings.test.tsx` - key input interactions, profile display, preferences
- `connect.test.tsx` - config blocks, copy functionality
- `billing.test.tsx` - plan display, checkout/portal buttons, session_id handling
- `layout.test.tsx` - nav rendering, active states, mobile toggle
- `key-input.test.tsx` - all display states, input/button interactions
- `copy-block.test.tsx` - code display, copy button functionality

## 8. Build Order

Phase 1 - Foundation:
1. package.json, tsconfig.json, vite.config.ts
2. Tailwind theme (src/styles/main.css)
3. index.html
4. .version, .env, .gitignore update
5. nginx.conf, Dockerfile

Phase 2 - Core Infrastructure:
6. src/state.ts
7. src/api.ts
8. src/auth.ts
9. src/router.ts
10. src/main.ts

Phase 3 - Components:
11. src/components/layout.tsx
12. src/components/copy-block.tsx
13. src/components/key-input.tsx
14. src/components/usage-chart.tsx

Phase 4 - Pages:
15. src/pages/landing.tsx
16. src/pages/callback.tsx
17. src/pages/dashboard.tsx
18. src/pages/settings.tsx
19. src/pages/connect.tsx
20. src/pages/billing.tsx

Phase 5 - Tests:
21. All unit tests
22. All component tests

Phase 6 - Verification:
23. npm run build (TypeScript + Vite)
24. npm run lint
25. npm test
26. docker build -t vire-portal:latest .
