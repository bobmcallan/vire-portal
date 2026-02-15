# Requirements: Flatten nav — remove dropdown

**Date:** 2026-02-15
**Requested:** Remove the dropdown menu and replace with flat inline links: Dashboard | Settings | Logout

## Scope
- Replace dropdown in nav with flat links: Dashboard, Settings, Logout
- Remove dropdown Alpine component usage from nav (keep component definition in common.js for potential future use)
- Remove dropdown CSS
- Logout becomes an inline styled link (actually a form POST for CSRF)
- Mobile menu already has flat links — no change needed there

## Approach
1. Replace the dropdown `<li>` in nav.html with two new `<li>` elements for Settings and Logout
2. Logout uses a form POST styled as a nav link (existing pattern from mobile menu)
3. Add `active` class support for settings page
4. Remove the entire DROPDOWN CSS section from portal.css
5. Style the logout button to match nav links

## Files Expected to Change
- `pages/partials/nav.html` — flatten the nav links
- `pages/static/css/portal.css` — remove dropdown CSS, add nav-links button style
