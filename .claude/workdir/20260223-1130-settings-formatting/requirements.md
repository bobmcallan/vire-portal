# Requirements: Fix settings page formatting

**Date:** 2026-02-23
**Requested:** The /settings page has lost its formatting — content has no padding, centering, or section spacing.

## Root Cause

The settings page template (`pages/settings.html`) uses CSS classes that don't exist in `pages/static/css/portal.css`:

| Class in template | CSS definition | Effect |
|---|---|---|
| `dashboard` (on `<main>`) | Missing | No min-height or flex layout |
| `dashboard-inner` (on wrapper `<div>`) | Missing | No max-width, no centering, no padding |
| `dashboard-section` (on `<section>`) | Missing | No border, no padding, no margin |
| `dashboard-field` (on field rows) | Missing | No layout for label+value pairs |
| `dashboard-label` (on labels) | Missing | No bold, no color, no spacing |

The dashboard page (`pages/dashboard.html`) uses `class="page"` and `class="page-body"` which ARE styled in portal.css — that's why dashboard works but settings doesn't.

## Scope

**In scope:**
- Fix the layout classes in settings.html to use existing CSS classes (`page`, `page-body`)
- Add CSS for `dashboard-section`, `dashboard-field`, `dashboard-label` (since they represent a useful component pattern for settings/detail views)
- Create a UI test to verify the settings page renders correctly

**Out of scope:**
- No changes to settings page functionality
- No changes to the handler (Go code)
- No changes to other pages

## Approach

Two-part fix:

1. **settings.html** — Change `<main class="dashboard">` to `<main class="page">` and `<div class="dashboard-inner">` to `<div class="page-body">` to match the dashboard pattern.

2. **portal.css** — Add styles for the section/field/label classes used in settings:
   - `.dashboard-section` — bordered panel with padding and bottom margin (like `.panel`)
   - `.dashboard-field` — flex row for label+value pairs with bottom border
   - `.dashboard-label` — bold uppercase label matching the 80s B&W aesthetic

## Files Expected to Change

- `pages/settings.html` — Fix layout wrapper classes
- `pages/static/css/portal.css` — Add dashboard-section/field/label styles
- `tests/ui/settings_test.go` — New: verify settings page layout and formatting
