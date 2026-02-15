# Browser Validation Skill

After making portal UI changes (templates, CSS, JS, Alpine components), validate them using the `browser-check` tool.

## Setup

The tool is at `tests/browser-check/main.go`. It uses chromedp (Go + Chrome DevTools Protocol). Zero external deps beyond Chrome.

The portal port is configured via `PORTAL_PORT` env var (default: `4241`). Examples below use the default.

Ensure the server is running:
```bash
./scripts/run.sh restart
```

Wait for health:
```bash
until curl -sf http://localhost:${PORTAL_PORT:-4241}/api/health > /dev/null; do sleep 1; done
```

## Usage

```bash
go run ./tests/browser-check -url <URL> [flags]
```

### Flags

| Flag | Description | Example |
|---|---|---|
| `-url` | URL to test (required) | `http://localhost:${PORTAL_PORT:-4241}/dashboard` |
| `-check` | `selector\|state` assertion (repeatable) | `-check '.dropdown-menu\|hidden'` |
| `-click` | Click selector before checks (repeatable, ordered) | `-click '.dropdown-trigger'` |
| `-eval` | JS expression, must return truthy (repeatable) | `-eval 'typeof Alpine !== "undefined"'` |
| `-viewport` | Set viewport WxH | `-viewport 375x812` |
| `-screenshot` | Save screenshot | `-screenshot /tmp/page.png` |
| `-wait` | Wait ms after load (default 1000) | `-wait 2000` |

### Check states

- `visible` — element exists and display != none
- `hidden` — element missing or display: none
- `exists` — element in DOM
- `gone` — element not in DOM
- `text=X` — textContent contains X
- `count>N` — querySelectorAll count (also `>=`, `=`, `<`, `<=`)

JS errors are **always checked** automatically.

## After every UI change, run validation

Pick checks based on what you changed. Examples:

### Changed nav/dropdown/menu
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard \
  -check '.nav-brand|visible' \
  -check '.nav-brand|text=VIRE' \
  -check '.dropdown-menu|hidden' \
  -click '.dropdown-trigger' \
  -check '.dropdown-menu|visible'
```

### Changed mobile menu
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard \
  -viewport 375x812 \
  -check '.nav-links|hidden' \
  -check '.nav-hamburger|visible' \
  -click '.nav-hamburger' \
  -check '.mobile-menu|visible'
```

### Changed panels/sections
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard \
  -check '.dashboard-section|count>=2' \
  -eval 'document.querySelector(".dashboard-title").textContent.includes("DASHBOARD")'
```

### Changed Alpine components
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard \
  -eval 'typeof Alpine !== "undefined"' \
  -check '.dropdown-menu|hidden' \
  -check '.panel-collapse-body|hidden'
```

### Changed CSS
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard \
  -eval '!Array.from(document.querySelectorAll("*")).some(e => getComputedStyle(e).borderRadius !== "0px")' \
  -eval '!Array.from(document.querySelectorAll("*")).some(e => { const s = getComputedStyle(e).boxShadow; return s && s !== "none" })'
```

### Quick smoke test (any change)
```bash
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/dashboard
go run ./tests/browser-check -url http://localhost:${PORTAL_PORT:-4241}/
```

This runs with zero -check flags and still catches JS errors on load.

## Rules

1. **Always run at least a smoke test** after any template/CSS/JS change
2. **Add specific checks** relevant to what you changed
3. **Exit code 0 = pass, 1 = fail** — if it fails, fix before committing
4. **JS errors always checked** — you don't need to add them explicitly
5. **Chain clicks before checks** — clicks execute in order, then checks run
6. **Test both desktop and mobile** if you touched responsive code
