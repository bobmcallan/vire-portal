package common

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type BrowserConfig struct {
	Headless bool
	Timeout  time.Duration
}

func DefaultBrowserConfig() *BrowserConfig {
	return &BrowserConfig{
		Headless: true,
		Timeout:  30 * time.Second,
	}
}

func NewBrowserContext(cfg *BrowserConfig) (context.Context, context.CancelFunc) {
	if cfg == nil {
		cfg = DefaultBrowserConfig()
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", cfg.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	ctx, timeoutCancel := context.WithTimeout(ctx, cfg.Timeout)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
	return ctx, cancel
}

type JSErrorCollector struct {
	mu     sync.Mutex
	errors []string
}

func NewJSErrorCollector(ctx context.Context) *JSErrorCollector {
	c := &JSErrorCollector{}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		c.mu.Lock()
		defer c.mu.Unlock()

		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			desc := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				desc = e.ExceptionDetails.Exception.Description
			}
			if strings.Contains(desc, "Content Security Policy") {
				return
			}
			c.errors = append(c.errors, fmt.Sprintf("EXCEPTION: %s", desc))

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
				if len(parts) > 0 {
					msg := strings.Join(parts, " ")
					if !strings.Contains(msg, "favicon") && !strings.Contains(msg, "Content Security Policy") {
						c.errors = append(c.errors, fmt.Sprintf("console.error: %s", msg))
					}
				}
			}
		}
	})

	return c
}

func (c *JSErrorCollector) Errors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.errors))
	copy(out, c.errors)
	return out
}

func (c *JSErrorCollector) HasErrors() bool {
	return len(c.Errors()) > 0
}

func ServerURL() string {
	if url := os.Getenv("VIRE_TEST_URL"); url != "" {
		return url
	}
	return "http://localhost:8881"
}

func NavigateAndWait(ctx context.Context, url string, waitMs int) error {
	if waitMs == 0 {
		waitMs = 800
	}
	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)
}

func LoginAndNavigate(ctx context.Context, targetURL string, waitMs int) error {
	base := ServerURL()
	if waitMs == 0 {
		waitMs = 800
	}
	var currentURL string
	return chromedp.Run(ctx,
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// Submit the dev login form by clicking the submit button
		chromedp.Click(".landing-dev-login button[type='submit']", chromedp.ByQuery),
		// Wait for navigation to complete (URL should change to dashboard)
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Location(&currentURL),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)
}

func SetViewport(ctx context.Context, width, height int64) error {
	return chromedp.Run(ctx, chromedp.EmulateViewport(width, height))
}

func IsHidden(ctx context.Context, selector string) (bool, error) {
	var hidden bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return true;
				return getComputedStyle(el).display === 'none';
			})()
		`, escJS(selector)), &hidden),
	)
	return hidden, err
}

func IsVisible(ctx context.Context, selector string) (bool, error) {
	var visible bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return false;
				return getComputedStyle(el).display !== 'none';
			})()
		`, escJS(selector)), &visible),
	)
	return visible, err
}

func Exists(ctx context.Context, selector string) (bool, error) {
	var exists bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelector('%s') !== null`, escJS(selector)), &exists),
	)
	return exists, err
}

func ElementCount(ctx context.Context, selector string) (int, error) {
	var count int
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, escJS(selector)), &count),
	)
	return count, err
}

func TextContains(ctx context.Context, selector, expected string) (bool, string, error) {
	var actual string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				return el ? el.textContent.trim() : '';
			})()
		`, escJS(selector)), &actual),
	)
	if err != nil {
		return false, "", err
	}
	return strings.Contains(actual, expected), actual, nil
}

func EvalBool(ctx context.Context, expr string) (bool, error) {
	var result bool
	err := chromedp.Run(ctx, chromedp.Evaluate(expr, &result))
	return result, err
}

func Click(ctx context.Context, selector string, waitMs int) error {
	if waitMs == 0 {
		waitMs = 300
	}
	return chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)
}

func ClickNav(ctx context.Context, selector string, waitMs int) error {
	if waitMs == 0 {
		waitMs = 800
	}
	return chromedp.Run(ctx,
		chromedp.Click(selector, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)
}

func Screenshot(ctx context.Context, path string) error {
	var buf []byte
	err := chromedp.Run(ctx, chromedp.FullScreenshot(&buf, 90))
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

type CheckResult struct {
	Name   string
	Pass   bool
	Detail string
}

func RunCheck(ctx context.Context, selector, state string) CheckResult {
	name := fmt.Sprintf("check(%s|%s)", selector, state)

	switch {
	case state == "hidden":
		hidden, err := IsHidden(ctx, selector)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		return CheckResult{Name: name, Pass: hidden, Detail: fmt.Sprintf("hidden=%v", hidden)}

	case state == "visible":
		visible, err := IsVisible(ctx, selector)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		return CheckResult{Name: name, Pass: visible, Detail: fmt.Sprintf("visible=%v", visible)}

	case state == "exists":
		exists, err := Exists(ctx, selector)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		return CheckResult{Name: name, Pass: exists, Detail: fmt.Sprintf("exists=%v", exists)}

	case state == "gone":
		exists, err := Exists(ctx, selector)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		return CheckResult{Name: name, Pass: !exists, Detail: fmt.Sprintf("gone=%v", !exists)}

	case strings.HasPrefix(state, "text="):
		expected := state[5:]
		pass, actual, err := TextContains(ctx, selector, expected)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		return CheckResult{Name: name, Pass: pass, Detail: fmt.Sprintf("got: %s", truncate(actual, 60))}

	case strings.HasPrefix(state, "count"):
		count, err := ElementCount(ctx, selector)
		if err != nil {
			return CheckResult{Name: name, Pass: false, Detail: err.Error()}
		}
		pass := evalCountExpr(state, count)
		return CheckResult{Name: name, Pass: pass, Detail: fmt.Sprintf("count=%d", count)}

	default:
		return CheckResult{Name: name, Pass: false, Detail: fmt.Sprintf("unknown state: %s", state)}
	}
}

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

func IsTruthy(v interface{}) bool {
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

type CheckRequest struct {
	URL        string
	Viewport   string
	Screenshot string
	WaitMs     int
	Login      bool
	Checks     []string
	Clicks     []string
	ClickNavs  []string
	Evals      []string
}

type CheckResponse struct {
	Results []CheckResult
	Passed  int
	Failed  int
}

func RunChecks(ctx context.Context, req CheckRequest) (*CheckResponse, error) {
	resp := &CheckResponse{}

	actions := []chromedp.Action{}

	if req.Viewport != "" {
		parts := strings.SplitN(req.Viewport, "x", 2)
		if len(parts) == 2 {
			w, _ := strconv.Atoi(parts[0])
			h, _ := strconv.Atoi(parts[1])
			if w > 0 && h > 0 {
				actions = append(actions, chromedp.EmulateViewport(int64(w), int64(h)))
			}
		}
	}

	waitMs := req.WaitMs
	if waitMs == 0 {
		waitMs = 1000
	}

	actions = append(actions,
		chromedp.Navigate(req.URL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
	)

	if err := chromedp.Run(ctx, actions...); err != nil {
		return nil, fmt.Errorf("navigate %s: %w", req.URL, err)
	}

	if req.Login {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(async () => {
					const body = new URLSearchParams({ username: 'dev_user', password: 'dev123' });
					const r = await fetch('/api/auth/login', {
						method: 'POST',
						headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
						credentials: 'same-origin',
						body: body,
					});
					return r.status;
				})()
			`, nil),
			chromedp.Sleep(300*time.Millisecond),
			chromedp.Navigate(req.URL),
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.Sleep(time.Duration(waitMs)*time.Millisecond),
		)
		if err != nil {
			return nil, fmt.Errorf("login + reload %s: %w", req.URL, err)
		}
	}

	for _, sel := range req.Clicks {
		err := Click(ctx, sel, 300)
		if err != nil {
			resp.Results = append(resp.Results, CheckResult{Name: fmt.Sprintf("click(%s)", sel), Pass: false, Detail: err.Error()})
			resp.Failed++
		} else {
			resp.Results = append(resp.Results, CheckResult{Name: fmt.Sprintf("click(%s)", sel), Pass: true, Detail: "ok"})
			resp.Passed++
		}
	}

	for _, sel := range req.ClickNavs {
		err := ClickNav(ctx, sel, waitMs)
		if err != nil {
			resp.Results = append(resp.Results, CheckResult{Name: fmt.Sprintf("clicknav(%s)", sel), Pass: false, Detail: err.Error()})
			resp.Failed++
		} else {
			resp.Results = append(resp.Results, CheckResult{Name: fmt.Sprintf("clicknav(%s)", sel), Pass: true, Detail: "ok"})
			resp.Passed++
		}
	}

	for _, c := range req.Checks {
		parts := strings.SplitN(c, "|", 2)
		if len(parts) != 2 {
			resp.Results = append(resp.Results, CheckResult{Name: c, Pass: false, Detail: "bad format, need selector|state"})
			resp.Failed++
			continue
		}
		sel, state := parts[0], parts[1]
		r := RunCheck(ctx, sel, state)
		resp.Results = append(resp.Results, r)
		if r.Pass {
			resp.Passed++
		} else {
			resp.Failed++
		}
	}

	for _, expr := range req.Evals {
		var val interface{}
		err := chromedp.Run(ctx, chromedp.Evaluate(expr, &val))
		if err != nil {
			resp.Results = append(resp.Results, CheckResult{
				Name: fmt.Sprintf("eval(%s)", truncate(expr, 50)), Pass: false, Detail: err.Error(),
			})
			resp.Failed++
		} else {
			truthy := IsTruthy(val)
			resp.Results = append(resp.Results, CheckResult{
				Name: fmt.Sprintf("eval(%s)", truncate(expr, 50)), Pass: truthy, Detail: fmt.Sprintf("returned: %v", val),
			})
			if truthy {
				resp.Passed++
			} else {
				resp.Failed++
			}
		}
	}

	if req.Screenshot != "" {
		if err := Screenshot(ctx, req.Screenshot); err != nil {
			return resp, fmt.Errorf("screenshot failed: %w", err)
		}
	}

	return resp, nil
}

func Truncate(s string, n int) string {
	return truncate(s, n)
}
