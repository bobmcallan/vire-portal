# Browser Check Workflow

Validate portal UI changes using chromedp browser tests. Run this after any frontend changes (templates, CSS, JS, Alpine components).

## Prerequisites

1. Ensure server is running:
```bash
./scripts/run.sh status
```

If not running, start it:
```bash
./scripts/run.sh restart
```

2. Wait for health:
```bash
until curl -sf http://localhost:8881/api/health > /dev/null; do sleep 1; done
```

## Steps

1. Set variables:
```bash
BASE_URL=http://localhost:8881
OUTDIR=.kilocode/workdir/$(date +%Y-%m-%d-%H-%M-%S)
mkdir -p "$OUTDIR"
```

2. Run smoke tests on main pages:

**Landing page (login elements):**
```bash
go run ./tests/browser-check -url "$BASE_URL/" -screenshot "$OUTDIR/landing.png" \
  -check 'a[href="/api/auth/login/google"]|visible' \
  -check 'a[href="/api/auth/login/github"]|visible'
```

**Dev login flow (dev mode):**
```bash
if [ "${PORTAL_ENV:-}" = "dev" ]; then
  go run ./tests/browser-check -url "$BASE_URL/" -screenshot "$OUTDIR/dev-login.png" \
    -clicknav '.landing-dev-login button' \
    -check '.nav|visible'
fi
```

**Dashboard:**
```bash
go run ./tests/browser-check -url "$BASE_URL/dashboard" -screenshot "$OUTDIR/dashboard.png"
```

3. If specific components changed, add targeted checks:

**Nav/menu changes:**
```bash
go run ./tests/browser-check -url "$BASE_URL/dashboard" -login \
  -screenshot "$OUTDIR/nav-default.png" \
  -check '.nav-brand|text=VIRE' \
  -check '.nav-hamburger|visible' \
  -check '.nav-dropdown|hidden'
```

**Mobile menu changes:**
```bash
go run ./tests/browser-check -url "$BASE_URL/dashboard" -login \
  -viewport 375x812 \
  -screenshot "$OUTDIR/mobile-nav.png" \
  -check '.nav-hamburger|visible'
```

**Alpine component changes:**
```bash
go run ./tests/browser-check -url "$BASE_URL/dashboard" \
  -screenshot "$OUTDIR/alpine.png" \
  -eval 'typeof Alpine !== "undefined"'
```

## Check States

- `visible` — element exists and display != none
- `hidden` — element missing or display: none
- `exists` — element in DOM
- `gone` — element not in DOM
- `text=X` — textContent contains X
- `count>N` — querySelectorAll count (also `>=`, `=`, `<`, `<=`)

## Failure Handling

- Exit code 0 = pass, 1 = fail
- JS errors are always checked automatically
- If tests fail, fix issues before committing
- Review screenshots in `$OUTDIR/` to diagnose issues
