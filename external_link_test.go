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

// openExternalURL must survive a nil URL and a missing app rather than
// panicking inside a tap handler.
func TestOpenExternalURLIsSafeOnNil(t *testing.T) {
	openExternalURL(nil) // must not panic
	app := test.NewApp()
	defer app.Quit()
	u, _ := url.Parse("mailto:someone@example.invalid")
	openExternalURL(u) // must not panic; the test app records rather than opens
}
