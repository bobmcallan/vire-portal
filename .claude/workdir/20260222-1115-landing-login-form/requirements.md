# Requirements: Landing Page Login Form

**Date:** 2026-02-22
**Requested:** Update landing page to show username/password login always, remove dev login button, pre-populate in dev mode.

## Scope
- Add username/password login form (always visible, not just dev mode)
- Remove the hidden dev login form
- In dev mode, pre-populate username/password fields with dev credentials
- Update UI tests to work with new form structure

## Approach
1. Replace the hidden dev login form with a visible login form containing:
   - Username input field
   - Password input field
   - Submit button
2. Use Go template to pre-populate fields with dev credentials when `.DevMode` is true
3. Update CSS for the new form styling
4. Update tests to use the new form structure

## Files Expected to Change
- `pages/landing.html` - Update login form
- `pages/static/css/portal.css` - Add styles for login form
- `tests/ui/dev_auth_test.go` - Update tests for new form
- `tests/common/browser.go` - Update form selector if needed
