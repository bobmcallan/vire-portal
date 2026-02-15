// tests/browser-check/main.go
//
// Lightweight browser validation tool using chromedp.
// Claude Code calls this via skill after deploying changes.
//
// Usage:
//   go run ./tests/browser-check -url http://localhost:4241/dashboard
//   go run ./tests/browser-check -url http://localhost:4241/dashboard -check '.dropdown-menu|hidden' -check '.nav-brand|visible'
//   go run ./tests/browser-check -url http://localhost:4241/dashboard -click '.dropdown-trigger' -check '.dropdown-menu|visible'
//   go run ./tests/browser-check -url http://localhost:4241/dashboard -viewport 375x812 -check '.nav-links|hidden'
//   go run ./tests/browser-check -url http://localhost:4241/dashboard -eval 'document.querySelectorAll(".panel").length > 0'
//   go run ./tests/browser-check -url http://localhost:4241/dashboard -screenshot /tmp/dash.png

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// multiFlag allows repeated -check or -click or -eval flags.
type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ", ") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

type result struct {
	name   string
	pass   bool
	detail string
}

func main() {
	var (
		url        string
		viewport   string
		screenshot string
		waitMs     int
		checks     multiFlag
		clicks     multiFlag
		evals      multiFlag
	)

	flag.StringVar(&url, "url", "", "URL to test (required)")
	flag.StringVar(&viewport, "viewport", "", "Viewport as WxH, e.g. 375x812")
	flag.StringVar(&screenshot, "screenshot", "", "Save screenshot to path")
	flag.IntVar(&waitMs, "wait", 1000, "Wait ms after load for Alpine/JS init")
	flag.Var(&checks, "check", "selector|state  (state: visible, hidden, text=X, count>N)")
	flag.Var(&clicks, "click", "CSS selector to click (in order, before -check)")
	flag.Var(&evals, "eval", "JS expression that must return truthy")
	flag.Parse()

	if url == "" {
		fmt.Fprintln(os.Stderr, "ERROR: -url is required")
		flag.Usage()
		os.Exit(2)
	}

	// ── Browser setup ──────────────────────────────────
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	defer ctxCancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	// ── JS error collector ─────────────────────────────
	var (
		jsErrors []string
		jsMu     sync.Mutex
	)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		jsMu.Lock()
		defer jsMu.Unlock()

		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			desc := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				desc = e.ExceptionDetails.Exception.Description
			}
			// chromedp Evaluate triggers CSP violations — ignore these
			if strings.Contains(desc, "Content Security Policy") {
				return
			}
			jsErrors = append(jsErrors, desc)
		case *runtime.EventConsoleAPICalled:
			if e.Type == runtime.APITypeError {
				var parts []string
				for _, arg := range e.Args {
					if arg.Value != nil {
						parts = append(parts, string(arg.Value))
					} else if arg.Description != "" {
						parts = append(parts, arg.Description)
					}
				}
				msg := strings.Join(parts, " ")
				if msg != "" && !strings.Contains(msg, "favicon") && !strings.Contains(msg, "Content Security Policy") {
					jsErrors = append(jsErrors, msg)
				}
			}
		}
	})

	// ── Navigate ───────────────────────────────────────
	actions := []chromedp.Action{}

	// Viewport
	if viewport != "" {
		parts := strings.SplitN(viewport, "x", 2)
		if len(parts) == 2 {
			w, _ := strconv.Atoi(parts[0])
			h, _ := strconv.Atoi(parts[1])
			if w > 0 && h > 0 {
				actions = append(actions, chromedp.EmulateViewport(int64(w), int64(h)))
			}
		}
	}

	actions = append(actions,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)

	if err := chromedp.Run(ctx, actions...); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: navigate %s: %v\n", url, err)
		os.Exit(1)
	}

	var results []result

	// ── JS errors check (always) ───────────────────────
	jsMu.Lock()
	if len(jsErrors) > 0 {
		results = append(results, result{
			name:   "js-errors",
			pass:   false,
			detail: strings.Join(jsErrors, "; "),
		})
	} else {
		results = append(results, result{name: "js-errors", pass: true, detail: "none"})
	}
	jsMu.Unlock()

	// ── Clicks (in order) ──────────────────────────────
	for _, sel := range clicks {
		err := chromedp.Run(ctx,
			chromedp.Click(sel, chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
		)
		if err != nil {
			results = append(results, result{
				name:   fmt.Sprintf("click(%s)", sel),
				pass:   false,
				detail: err.Error(),
			})
		} else {
			results = append(results, result{
				name:   fmt.Sprintf("click(%s)", sel),
				pass:   true,
				detail: "ok",
			})
		}
	}

	// ── Checks ─────────────────────────────────────────
	for _, c := range checks {
		parts := strings.SplitN(c, "|", 2)
		if len(parts) != 2 {
			results = append(results, result{name: c, pass: false, detail: "bad format, need selector|state"})
			continue
		}

		sel, state := parts[0], parts[1]
		r := runCheck(ctx, sel, state)
		results = append(results, r)
	}

	// ── Evals ──────────────────────────────────────────
	for _, expr := range evals {
		var val interface{}
		err := chromedp.Run(ctx, chromedp.Evaluate(expr, &val))
		if err != nil {
			results = append(results, result{
				name:   fmt.Sprintf("eval(%s)", truncate(expr, 50)),
				pass:   false,
				detail: err.Error(),
			})
		} else {
			truthy := isTruthy(val)
			results = append(results, result{
				name:   fmt.Sprintf("eval(%s)", truncate(expr, 50)),
				pass:   truthy,
				detail: fmt.Sprintf("returned: %v", val),
			})
		}
	}

	// ── Screenshot ─────────────────────────────────────
	if screenshot != "" {
		var buf []byte
		err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: screenshot failed: %v\n", err)
		} else {
			os.WriteFile(screenshot, buf, 0644)
			fmt.Printf("  screenshot: %s\n", screenshot)
		}
	}

	// ── Report ─────────────────────────────────────────
	fmt.Println()
	failed := 0
	for _, r := range results {
		icon := "✓"
		if !r.pass {
			icon = "✗"
			failed++
		}
		fmt.Printf("  %s %s — %s\n", icon, r.name, r.detail)
	}

	fmt.Printf("\n  %d/%d passed\n", len(results)-failed, len(results))

	if failed > 0 {
		os.Exit(1)
	}
}

// runCheck evaluates a selector|state assertion.
func runCheck(ctx context.Context, sel, state string) result {
	name := fmt.Sprintf("check(%s|%s)", sel, state)

	switch {
	case state == "hidden":
		var hidden bool
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return true;
				return getComputedStyle(el).display === 'none';
			})()
		`, escJS(sel)), &hidden))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		return result{name: name, pass: hidden, detail: fmt.Sprintf("hidden=%v", hidden)}

	case state == "visible":
		var visible bool
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return false;
				return getComputedStyle(el).display !== 'none';
			})()
		`, escJS(sel)), &visible))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		return result{name: name, pass: visible, detail: fmt.Sprintf("visible=%v", visible)}

	case state == "exists":
		var exists bool
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, escJS(sel)), &exists))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		return result{name: name, pass: exists, detail: fmt.Sprintf("exists=%v", exists)}

	case state == "gone":
		var gone bool
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') === null`, escJS(sel)), &gone))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		return result{name: name, pass: gone, detail: fmt.Sprintf("gone=%v", gone)}

	case strings.HasPrefix(state, "text="):
		expected := state[5:]
		var actual string
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				return el ? el.textContent.trim() : '';
			})()
		`, escJS(sel)), &actual))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		pass := strings.Contains(actual, expected)
		return result{name: name, pass: pass, detail: fmt.Sprintf("got: %s", truncate(actual, 60))}

	case strings.HasPrefix(state, "count"):
		// count>0, count=3, count>=2
		var count int
		err := chromedp.Run(ctx, chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, escJS(sel)), &count))
		if err != nil {
			return result{name: name, pass: false, detail: err.Error()}
		}
		pass := evalCountExpr(state, count)
		return result{name: name, pass: pass, detail: fmt.Sprintf("count=%d", count)}

	default:
		return result{name: name, pass: false, detail: fmt.Sprintf("unknown state: %s", state)}
	}
}

// evalCountExpr handles count>N, count>=N, count=N, count<N
func evalCountExpr(expr string, actual int) bool {
	expr = strings.TrimPrefix(expr, "count")
	if strings.HasPrefix(expr, ">=") {
		n, _ := strconv.Atoi(expr[2:])
		return actual >= n
	}
	if strings.HasPrefix(expr, ">") {
		n, _ := strconv.Atoi(expr[1:])
		return actual > n
	}
	if strings.HasPrefix(expr, "<=") {
		n, _ := strconv.Atoi(expr[2:])
		return actual <= n
	}
	if strings.HasPrefix(expr, "<") {
		n, _ := strconv.Atoi(expr[1:])
		return actual < n
	}
	if strings.HasPrefix(expr, "=") {
		n, _ := strconv.Atoi(expr[1:])
		return actual == n
	}
	return false
}

func escJS(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func isTruthy(v interface{}) bool {
	switch val := v.(type) {
	case bool:
		return val
	case float64:
		return val != 0
	case string:
		return val != ""
	case nil:
		return false
	default:
		return true
	}
}
