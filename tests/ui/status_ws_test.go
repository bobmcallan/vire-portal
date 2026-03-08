package tests

import (
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestStatusWebSocket_Connected(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Wait for WebSocket connection and first ping/response cycle.
	// The WS client sends a ping immediately on connect, server responds
	// with JSON setting portal/server status. Allow time for the round-trip.
	chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

	takeScreenshot(t, ctx, "status_ws", "connected.png")

	// Both status dots should have the "status-up" class after WS response
	if err := assertEval(ctx, `
		(() => {
			const dots = document.querySelectorAll('.status-dot');
			if (dots.length < 2) return false;
			const p = dots[0];
			const s = dots[1];
			return p.classList.contains('status-up') && s.classList.contains('status-up');
		})()
	`, "both status dots should be green (status-up)"); err != nil {
		t.Error(err)
	}

	// Verify P dot has title "Portal" and S dot has title "Server"
	if err := assertEval(ctx, `
		(() => {
			const dots = document.querySelectorAll('.status-dot');
			if (dots.length < 2) return false;
			return dots[0].getAttribute('title') === 'Portal' &&
			       dots[1].getAttribute('title') === 'Server';
		})()
	`, "status dots should have correct title attributes"); err != nil {
		t.Error(err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on page:\n  %s", joinErrors(jsErrs))
	}
}

func TestStatusWebSocket_ServerDown(t *testing.T) {
	// In the Docker test environment, vire-server is always running.
	// We cannot stop it without affecting other tests. This test verifies
	// the DOM structure is correct so that when the server reports "down",
	// the S dot would show status-down. We validate by injecting the
	// Alpine component state directly via JS.
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Wait for Alpine to initialise and WS to connect
	chromedp.Run(ctx, chromedp.Sleep(3*time.Second))

	// Simulate server-down by setting the Alpine component's reactive state.
	// This validates that the DOM binding works: setting server='down' makes
	// the S dot show status-down, while portal='up' keeps P dot green.
	var simOk bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const el = document.querySelector('.status-indicators');
				if (!el) return false;
				const data = Alpine.$data(el);
				if (!data) return false;
				data.server = 'down';
				data.portal = 'up';
				return true;
			})()
		`, &simOk),
	)
	if err != nil {
		t.Fatalf("failed to simulate server-down: %v", err)
	}
	if !simOk {
		t.Fatal("could not access Alpine component data to simulate server-down")
	}

	// Allow Alpine to react to state change
	chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "status_ws", "server-down.png")

	// P dot should be green (status-up), S dot should be red (status-down)
	if err := assertEval(ctx, `
		(() => {
			const dots = document.querySelectorAll('.status-dot');
			if (dots.length < 2) return false;
			const p = dots[0];
			const s = dots[1];
			return p.classList.contains('status-up') && s.classList.contains('status-down');
		})()
	`, "P dot should be status-up, S dot should be status-down"); err != nil {
		t.Error(err)
	}

	// Verify neither dot has the startup class anymore
	if err := assertEval(ctx, `
		(() => {
			const dots = document.querySelectorAll('.status-dot');
			if (dots.length < 2) return false;
			return !dots[0].classList.contains('status-startup') &&
			       !dots[1].classList.contains('status-startup');
		})()
	`, "no status dots should have status-startup class"); err != nil {
		t.Error(err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on page:\n  %s", joinErrors(jsErrs))
	}
}
