package tests

import (
	"testing"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestSettingsPageLayout(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "page-layout.png")

	// main element must use .page class for min-height/flex layout
	pageVisible, err := isVisible(ctx, "main.page")
	if err != nil {
		t.Fatal(err)
	}
	if !pageVisible {
		t.Error("main.page not found — settings page missing .page layout class")
	}

	// inner wrapper must use .page-body for max-width/centering/padding
	bodyVisible, err := isVisible(ctx, ".page-body")
	if err != nil {
		t.Fatal(err)
	}
	if !bodyVisible {
		t.Error(".page-body not found — settings page missing centered container")
	}
}

func TestSettingsPageBodyPadding(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "page-body-padding.png")

	// .page-body must have non-zero padding
	hasPadding, err := common.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.page-body');
			if (!el) return false;
			const cs = getComputedStyle(el);
			return parseFloat(cs.paddingTop) > 0 && parseFloat(cs.paddingLeft) > 0;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasPadding {
		t.Error(".page-body has no padding — content flush against edges")
	}
}

func TestSettingsPageBodyMaxWidth(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "page-body-maxwidth.png")

	// .page-body must have a max-width constraint (64rem = 1024px)
	hasMaxWidth, err := common.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.page-body');
			if (!el) return false;
			const cs = getComputedStyle(el);
			const mw = cs.maxWidth;
			return mw !== 'none' && mw !== '0px' && mw !== '';
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMaxWidth {
		t.Error(".page-body has no max-width — content spans full viewport")
	}
}

func TestSettingsNavexaSectionVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "navexa-section.png")

	// At least one .dashboard-section should be visible (Navexa API Key)
	sectionVisible, err := isVisible(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}
	if !sectionVisible {
		t.Error(".dashboard-section not visible — settings sections missing")
	}
}

func TestSettingsSectionHasBorder(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "section-border.png")

	// .dashboard-section must have a visible border
	hasBorder, err := common.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.dashboard-section');
			if (!el) return false;
			const cs = getComputedStyle(el);
			return cs.borderStyle !== 'none' && parseFloat(cs.borderWidth) > 0;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBorder {
		t.Error(".dashboard-section has no border — sections lack visual separation")
	}
}

func TestSettingsSectionHasPadding(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "section-padding.png")

	// .dashboard-section must have internal padding
	hasPadding, err := common.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.dashboard-section');
			if (!el) return false;
			const cs = getComputedStyle(el);
			return parseFloat(cs.padding) > 0 || parseFloat(cs.paddingTop) > 0;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasPadding {
		t.Error(".dashboard-section has no padding — content flush against border")
	}
}

func TestSettingsNavexaFormElements(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "form-elements.png")

	// API key input field
	inputVisible, err := isVisible(ctx, `input#navexa_key`)
	if err != nil {
		t.Fatal(err)
	}
	if !inputVisible {
		t.Error("Navexa API key input not visible")
	}

	// Save button
	btnVisible, err := isVisible(ctx, `button.btn-primary`)
	if err != nil {
		t.Fatal(err)
	}
	if !btnVisible {
		t.Error("Save button not visible")
	}
}

func TestSettingsSectionTitle(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "section-title.png")

	if err := assertTextContains(ctx, ".section-title", "NAVEXA API KEY", "settings section title"); err != nil {
		t.Error(err)
	}
}

func TestSettingsNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on settings page:\n  %s", joinErrors(jsErrs))
	}
}

func TestSettingsNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "nav-visible.png")

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav bar not visible on settings page")
	}
}

func TestSettingsFooterVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	// Scroll to bottom to ensure footer is rendered
	err = chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil))
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "settings", "footer-visible.png")

	footerVisible, err := isVisible(ctx, ".footer")
	if err != nil {
		t.Fatal(err)
	}
	if !footerVisible {
		t.Error("footer not visible on settings page")
	}
}
