package bibletext

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// A link the app shows must open through openExternalURL, never through
// Fyne's own OpenURL. The difference is invisible in a normal desktop run and
// decisive in a sandboxed one, which is exactly the kind of divergence a
// reviewer would find before a test does — so the test enforces it in source.
//
// Two shapes are refused: widget.NewHyperlink with a non-nil URL (Hyperlink
// then opens the URL itself), and a direct OpenURL call.
func TestAppLinksGoThroughTheCheckedOpener(t *testing.T) {
	// Hyperlink used purely as a styled button (nil URL plus OnTapped) opens
	// nothing, so it is allowed; ai_faster_model.go relies on that.
	bareHyperlink := regexp.MustCompile(`widget\.NewHyperlink\([^)]*,\s*(?:&?url\.|u\b|mu\b)`)
	directOpen := regexp.MustCompile(`\.OpenURL\(`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var offenders, scanned []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// external_link.go and its platform files are the one place allowed to
		// name Fyne's OpenURL: they are the wrapper.
		if strings.HasPrefix(name, "external_link") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			continue
		}
		body := string(data)
		scanned = append(scanned, name)
		// Android has its own platform link handling and is not sandboxed the
		// way macOS is; its files are out of scope.
		if strings.Contains(body, "//go:build android") {
			continue
		}
		if bareHyperlink.MatchString(body) {
			offenders = append(offenders, name+": widget.NewHyperlink with a URL — use externalLink")
		}
		if directOpen.MatchString(body) {
			offenders = append(offenders, name+": calls OpenURL directly — use openExternalURL")
		}
	}
	if len(scanned) < 50 {
		t.Fatalf("only scanned %d files; the sweep is not reaching the package", len(scanned))
	}
	// CONTROL: the patterns must be able to fire. If neither matches a source
	// string known to contain both shapes, the zero above proves nothing.
	const known = "x := widget.NewHyperlink(\"a\", u)\ny := app.OpenURL(u)"
	if !bareHyperlink.MatchString(known) || !directOpen.MatchString(known) {
		t.Fatal("the offender patterns cannot match a known offending source; the sweep is broken")
	}
	if len(offenders) > 0 {
		t.Errorf("links must open through externalLink/openExternalURL:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// externalLink must behave like the Hyperlink it replaces, and must route a
// tap to our opener rather than Fyne's.
func TestExternalLinkRoutesTapsToTheOpener(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	u, _ := url.Parse("https://example.invalid/page")
	hl := externalLink("Label ↗", u)
	if hl == nil {
		t.Fatal("externalLink returned nil")
	}
	if hl.Text != "Label ↗" {
		t.Errorf("label = %q, want the text passed in", hl.Text)
	}
	// The URL must stay nil: a non-nil URL is exactly what would let Hyperlink
	// open it through Fyne behind our back.
	if hl.URL != nil {
		t.Error("externalLink left URL set; Hyperlink would bypass the opener")
	}
	if hl.OnTapped == nil {
		t.Fatal("externalLink set no tap handler, so the link does nothing")
	}

	// A nil target yields an inert link rather than a panic.
	if plain := externalLink("Plain", nil); plain == nil || plain.OnTapped != nil {
		t.Error("a nil URL should give an inert Hyperlink with no handler")
	}
}

// openExternalURL must survive a nil URL, and must route a real one to the
// opener exactly once.
//
// The opener is SUBSTITUTED here on purpose. Calling the platform one would
// open a browser or a mail composer on whatever machine runs the suite — which
// this test did, on every run, until it was caught. A test that asserts
// routing must not perform the thing it is describing.
func TestOpenExternalURLRoutesWithoutOpeningAnything(t *testing.T) {
	restore := externalOpener
	defer func() { externalOpener = restore }()

	var got []string
	externalOpener = func(u *url.URL) error {
		got = append(got, u.String())
		return nil
	}

	openExternalURL(nil)
	if len(got) != 0 {
		t.Errorf("a nil URL reached the opener: %v", got)
	}

	u, _ := url.Parse("mailto:someone@example.invalid")
	openExternalURL(u)
	if len(got) != 1 || got[0] != "mailto:someone@example.invalid" {
		t.Errorf("opener saw %v, want the one mailto URL", got)
	}
}

// A tap on an externalLink must reach the opener — the whole point of the
// wrapper — and again without opening anything.
func TestExternalLinkTapReachesTheOpener(t *testing.T) {
	restore := externalOpener
	defer func() { externalOpener = restore }()
	var seen string
	externalOpener = func(u *url.URL) error { seen = u.String(); return nil }

	u, _ := url.Parse("https://example.invalid/page")
	hl := externalLink("Label", u)
	hl.OnTapped()
	if seen != "https://example.invalid/page" {
		t.Errorf("tap delivered %q to the opener, want the link target", seen)
	}
}

// No test in this package may reach the real platform opener. One did — it
// launched a mail composer on the developer's machine on every run of the
// suite — and the cost of that mistake is entirely invisible in CI, where
// nobody sees the window open. Any test needing the opener substitutes
// externalOpener; this fails the build if one calls openExternalURL or
// openExternalURLPlatform without first replacing it.
func TestNoTestOpensSomethingForReal(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	callsOpener := regexp.MustCompile(`\bopenExternalURL(?:Platform)?\(`)
	substitutes := regexp.MustCompile(`externalOpener\s*=`)
	var offenders, scanned []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		body := string(data)
		scanned = append(scanned, name)
		if callsOpener.MatchString(body) && !substitutes.MatchString(body) {
			offenders = append(offenders, name)
		}
	}
	if len(scanned) == 0 {
		t.Fatal("scanned no test files; the sweep is not reaching the package")
	}
	// CONTROL: both patterns must fire on a known-offending source, or the
	// clean result above means only that the regexes are broken.
	if !callsOpener.MatchString("openExternalURL(u)") ||
		!substitutes.MatchString("externalOpener = func(u *url.URL) error { return nil }") {
		t.Fatal("the sweep patterns cannot match known sources")
	}
	if len(offenders) > 0 {
		t.Errorf("these tests reach the real opener without substituting it: %v", offenders)
	}
}
