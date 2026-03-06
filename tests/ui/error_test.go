package tests

import (
	"testing"

	"github.com/chromedp/chromedp"
)

func TestErrorPageSSRDisplaysMessage(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Error page is public (no login required), navigate directly
	err := navigateAndWait(ctx, serverURL()+"/error?reason=auth_failed")
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "error", "ssr-displays-message.png")

	// Verify the SSR-rendered error message is present in the page
	var taglineText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const el = document.querySelector('.landing-tagline');
			return el ? el.textContent.trim() : '';
		})()
	`, &taglineText))
	if err != nil {
		t.Fatalf("error getting tagline text: %v", err)
	}

	expected := "Authentication failed. Please try again."
	if taglineText != expected {
		t.Errorf("error message = %q, want %q", taglineText, expected)
	}
}

func TestErrorPageSSRDefaultMessage(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := navigateAndWait(ctx, serverURL()+"/error")
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "error", "ssr-default-message.png")

	var taglineText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const el = document.querySelector('.landing-tagline');
			return el ? el.textContent.trim() : '';
		})()
	`, &taglineText))
	if err != nil {
		t.Fatalf("error getting tagline text: %v", err)
	}

	expected := "Something went wrong. Please try again."
	if taglineText != expected {
		t.Errorf("error message = %q, want %q", taglineText, expected)
	}
}

func TestErrorPageSSRNoScript(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := navigateAndWait(ctx, serverURL()+"/error?reason=server_unavailable")
	if err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "error", "ssr-no-script.png")

	// Verify no inline script block exists (SSR removed the Alpine script)
	var scriptCount int
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		document.querySelectorAll('main script').length
	`, &scriptCount))
	if err != nil {
		t.Fatalf("error counting scripts: %v", err)
	}
	if scriptCount > 0 {
		t.Errorf("found %d script tags inside <main>, want 0 (SSR should remove Alpine script)", scriptCount)
	}
}
