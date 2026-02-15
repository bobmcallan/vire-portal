# Requirements: Hamburger dropdown menu + authenticated screenshots

**Date:** 2026-02-15
**Requested:**
1. Create a hamburger icon on the far right of the nav bar (desktop) that opens a dropdown with Settings and Logout. Dashboard stays as a visible nav link.
2. Fix browser-check screenshots to capture the nav bar by adding dev auth support.

## Scope
- Replace flat Settings/Logout nav links with a hamburger dropdown on desktop
- Keep Dashboard as a visible nav link
- Existing mobile menu (slide-out panel) stays as-is
- Add `-login` flag to browser-check tool for dev auth before screenshots
- Update UI tests for new nav structure

## Approach

### Nav restructure
Current nav: `VIRE | Dashboard | Settings | Logout | [P] [S] | ☰(mobile)`
Target nav:  `VIRE | Dashboard | [P] [S] | ☰(dropdown: Settings, Logout)`

- Remove Settings and Logout `<li>` items from `.nav-links`
- Replace mobile-only hamburger with an always-visible hamburger on the far right
- On desktop: hamburger opens a small dropdown below it (using Alpine `dropdown()` component from common.js)
- On mobile: hamburger opens the existing slide-out mobile menu (using `mobileMenu()` component)
- Merge both behaviors: single hamburger button, desktop shows dropdown, mobile shows slide-out

Implementation: Use a combined Alpine component. On desktop viewport (>48rem), clicking the hamburger toggles a small dropdown. On mobile (<48rem), it opens the full slide-out menu.

Alternative (simpler): Keep separate controls. Desktop gets a new hamburger that toggles a dropdown. Mobile keeps the existing hamburger for the slide-out. But this means two hamburger buttons which is confusing.

**Chosen approach**: Single hamburger button, always visible, far-right. Uses Alpine to detect viewport:
- Desktop click → small dropdown below hamburger with Settings + Logout
- Mobile click → existing slide-out panel with Dashboard + Settings + Logout

### Browser-check auth
- Add `-login` flag to `tests/browser-check/main.go`
- When set, navigates to server root, POSTs to `/api/auth/dev` via JS fetch, then proceeds to target URL
- Screenshots will then capture the full page including nav

## Files Expected to Change
- `pages/partials/nav.html` — restructure nav, single hamburger, dropdown
- `pages/static/css/portal.css` — dropdown CSS, remove mobile-only hamburger restriction
- `pages/static/common.js` — update/merge mobileMenu and dropdown components
- `tests/browser-check/main.go` — add `-login` flag
- `tests/ui/ui_test.go` — update nav tests for new structure
- `internal/handlers/handlers_test.go` — update nav template test
