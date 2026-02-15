package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// serverURL returns the portal URL.
// Set VIRE_TEST_URL when running via scripts/test-ui.sh against Docker.
// Falls back to localhost:4241.
func serverURL() string {
	if url := os.Getenv("VIRE_TEST_URL"); url != "" {
		return url
	}
	return "http://localhost:4241"
}

// newBrowser creates a headless Chrome context with a 30s timeout.
func newBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}
	return ctx, cancel
}

// jsErrorCollector listens for JS exceptions and console.error calls.
// Call before chromedp.Navigate.
type jsErrorCollector struct {
	mu     sync.Mutex
	errors []string
}

func newJSErrorCollector(ctx context.Context) *jsErrorCollector {
	c := &jsErrorCollector{}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		c.mu.Lock()
		defer c.mu.Unlock()

		switch e := ev.(type) {
		case *runtime.EventExceptionThrown:
			desc := e.ExceptionDetails.Text
			if e.ExceptionDetails.Exception != nil && e.ExceptionDetails.Exception.Description != "" {
				desc = e.ExceptionDetails.Exception.Description
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
					// Ignore noisy but harmless errors
					if !strings.Contains(msg, "favicon") {
						c.errors = append(c.errors, fmt.Sprintf("console.error: %s", msg))
					}
				}
			}
		}
	})

	return c
}

func (c *jsErrorCollector) Errors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.errors))
	copy(out, c.errors)
	return out
}

// navigateAndWait navigates to a page, waits for body, and gives Alpine time to init.
func navigateAndWait(ctx context.Context, url string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond), // Alpine.js init
	)
}

// loginAndNavigate authenticates via the dev auth endpoint, then navigates to
// the target URL. This is needed for pages that render nav only when logged in.
func loginAndNavigate(ctx context.Context, targetURL string) error {
	base := serverURL()
	return chromedp.Run(ctx,
		// Navigate to any page first so we have the right origin for fetch
		chromedp.Navigate(base+"/"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		// POST to login endpoint to set the session cookie
		chromedp.Evaluate(fmt.Sprintf(`
			(async () => {
				const body = new URLSearchParams({ username: 'dev_user', password: 'dev123' });
				const r = await fetch('%s/api/auth/login', {
					method: 'POST',
					headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
					credentials: 'same-origin',
					body: body,
				});
				return r.status;
			})()
		`, base), nil),
		chromedp.Sleep(300*time.Millisecond),
		// Now navigate to the target page with the session cookie set
		chromedp.Navigate(targetURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
	)
}

// isHidden checks if an element is display:none or not in the DOM.
func isHidden(ctx context.Context, selector string) (bool, error) {
	var hidden bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return true;
				return getComputedStyle(el).display === 'none';
			})()
		`, selector), &hidden),
	)
	return hidden, err
}

// isVisible checks if an element exists and is not display:none.
func isVisible(ctx context.Context, selector string) (bool, error) {
	var visible bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`
			(() => {
				const el = document.querySelector('%s');
				if (!el) return false;
				return getComputedStyle(el).display !== 'none';
			})()
		`, selector), &visible),
	)
	return visible, err
}

// elementCount returns how many elements match the selector.
func elementCount(ctx context.Context, selector string) (int, error) {
	var count int
	err := chromedp.Run(ctx,
		chromedp.Evaluate(fmt.Sprintf(`document.querySelectorAll('%s').length`, selector), &count),
	)
	return count, err
}
