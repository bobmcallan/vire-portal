package tests

import (
	"testing"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestProfilePageLayout(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "page-layout.png")

	// main element must use .page class for min-height/flex layout
	pageVisible, err := isVisible(ctx, "main.page")
	if err != nil {
		t.Fatal(err)
	}
	if !pageVisible {
		t.Error("main.page not found — profile page missing .page layout class")
	}

	// inner wrapper must use .page-body for max-width/centering/padding
	bodyVisible, err := isVisible(ctx, ".page-body")
	if err != nil {
		t.Fatal(err)
	}
	if !bodyVisible {
		t.Error(".page-body not found — profile page missing centered container")
	}
}

func TestProfilePageBodyPadding(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "page-body-padding.png")

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

func TestProfilePageBodyMaxWidth(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "page-body-maxwidth.png")

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

func TestProfileNavexaSectionVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "navexa-section.png")

	// At least one .dashboard-section should be visible (Navexa API Key)
	sectionVisible, err := isVisible(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}
	if !sectionVisible {
		t.Error(".dashboard-section not visible — profile sections missing")
	}
}

func TestProfileSectionHasBorder(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "section-border.png")

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

func TestProfileSectionHasPadding(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "section-padding.png")

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

func TestProfileNavexaFormElements(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "form-elements.png")

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

func TestProfileSectionTitle(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "section-title.png")

	if err := assertTextContains(ctx, ".section-title", "USER PROFILE", "profile section title"); err != nil {
		t.Error(err)
	}
}

func TestProfileNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on profile page:\n  %s", joinErrors(jsErrs))
	}
}

func TestProfileNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "nav-visible.png")

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav bar not visible on profile page")
	}
}

func TestProfileFooterVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	// Scroll to bottom to ensure footer is rendered
	err = chromedp.Run(ctx, chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil))
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "footer-visible.png")

	footerVisible, err := isVisible(ctx, ".footer")
	if err != nil {
		t.Fatal(err)
	}
	if !footerVisible {
		t.Error("footer not visible on profile page")
	}
}

// TestProfileUserInfoSection verifies the USER PROFILE section is present and shows
// email, name, and auth method labels.
func TestProfileUserInfoSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "user-info-section.png")

	// USER PROFILE section title must be present
	sectionExists, err := common.Exists(ctx, `.section-title`)
	if err != nil {
		t.Fatal(err)
	}
	if !sectionExists {
		t.Fatal(".section-title not found — profile sections missing")
	}

	// Check for USER PROFILE section title text
	hasSectionTitle, err := common.EvalBool(ctx, `
		(() => {
			const titles = document.querySelectorAll('.section-title');
			for (const t of titles) {
				if (t.textContent.includes('USER PROFILE')) return true;
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSectionTitle {
		t.Error("USER PROFILE section title not found — user info section missing")
	}

	// EMAIL label must be present in a dashboard-label
	hasEmailLabel, err := common.EvalBool(ctx, `
		(() => {
			const labels = document.querySelectorAll('.dashboard-label');
			for (const l of labels) {
				if (l.textContent.trim() === 'EMAIL') return true;
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasEmailLabel {
		t.Error("EMAIL label not found in profile user info section")
	}

	// NAME label must be present
	hasNameLabel, err := common.EvalBool(ctx, `
		(() => {
			const labels = document.querySelectorAll('.dashboard-label');
			for (const l of labels) {
				if (l.textContent.trim() === 'NAME') return true;
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNameLabel {
		t.Error("NAME label not found in profile user info section")
	}

	// AUTH METHOD label must be present
	hasAuthLabel, err := common.EvalBool(ctx, `
		(() => {
			const labels = document.querySelectorAll('.dashboard-label');
			for (const l of labels) {
				if (l.textContent.trim() === 'AUTH METHOD') return true;
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAuthLabel {
		t.Error("AUTH METHOD label not found in profile user info section")
	}
}

// TestProfileEmailLockedForOAuth verifies that in dev mode (provider="dev"), the email
// field is displayed as text (not an editable input). OAuth users (google/github) get a
// locked display; dev mode uses "dev" provider so email is also shown as read-only text.
func TestProfileEmailLockedForOAuth(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/profile")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "profile", "email-locked.png")

	// In dev mode (provider="dev") the email field must NOT be an editable input.
	// The template renders email as a locked display field when IsOAuth=true,
	// and as plain text otherwise — in neither case is there an <input> for email.
	emailInputExists, err := common.Exists(ctx, `input[name="email"]`)
	if err != nil {
		t.Fatal(err)
	}
	if emailInputExists {
		t.Error("email should not be an editable input field on the profile page")
	}
}
