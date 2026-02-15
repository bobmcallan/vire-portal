# Requirements: Add favicon

**Date:** 2026-02-15
**Requested:** Add a favicon to the website using a 'V' character, based on quaero's create-favicon.sh script.

## Scope
- Create `scripts/create-favicon.sh` adapted from quaero (black bg, white V)
- Generate favicon.ico in pages/static/ and pages/
- Add `<link rel="icon">` to head.html partial

## Approach
Adapt quaero's hex-based ICO generation script. Use black (#000000) background with white (#ffffff) V to match the portal's monochrome design. No ImageMagick dependency — pure bash printf.

## Files Expected to Change
- `scripts/create-favicon.sh` — new script
- `pages/static/favicon.ico` — generated
- `pages/favicon.ico` — copy for root serving
- `pages/partials/head.html` — add favicon link
