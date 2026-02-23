# Summary: Fix settings page formatting

**Date:** 2026-02-23
**Status:** completed

## What Changed

| File | Change |
|------|--------|
| `pages/settings.html` | Changed `<main class="dashboard">` to `<main class="page">` and `<div class="dashboard-inner">` to `<div class="page-body">` — these are the existing layout classes used by other pages |
| `pages/static/css/portal.css` | Added `.dashboard-section` (bordered panel), `.dashboard-field` (flex label+value row), `.dashboard-label` (uppercase muted label) |
| `tests/ui/settings_test.go` | New: 11 UI tests with validation screenshots |
| `.claude/skills/test-common/SKILL.md` | Added Rule 4: validation screenshots mandatory in every UI test |
| `.claude/skills/test-create-review/SKILL.md` | Updated template, compliance checklist, and structural checklist to require validation screenshots |

## Root Cause

The settings template used CSS classes (`dashboard`, `dashboard-inner`) that had no styles in portal.css. The dashboard page uses `page` and `page-body` which are styled. The section/field/label classes also had no CSS definitions.

## Tests
- 11 new tests in `tests/ui/settings_test.go`
- All pass with 11 validation screenshots in `tests/logs/20260223-114131/settings/`
- Tests cover: layout classes, padding, max-width, section borders, section padding, form elements, section title, JS errors, nav visibility, footer visibility

## Skill Updates
- **test-common Rule 4**: Every UI test must call `takeScreenshot()` after page load, before assertions
- **test-create-review**: Template updated to show mandatory screenshot placement; compliance checklist and structural checklist updated
