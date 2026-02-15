# Requirements: Remove dashboard title

**Date:** 2026-02-15
**Requested:** Remove the dashboard title from the page — it is redundant since the right menu item shows the current page.

## Scope
- Remove unused `.dashboard-title`, `.page-title`, `.dashboard-section-title` CSS classes
- The `<h1>` elements were already removed from dashboard.html and settings.html in a prior session
- Section titles (MCP CONNECTION, CONFIG, TOOLS, NAVEXA API KEY) remain — those are content headings, not page titles

## Approach
Clean up orphaned CSS that was left behind when the HTML titles were removed previously.

## Files Expected to Change
- `pages/static/css/portal.css` — remove 3 unused CSS classes
