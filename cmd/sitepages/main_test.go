package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func TestRenderSitePages(t *testing.T) {
	source := t.TempDir()
	out := t.TempDir()
	writePage := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePage("index.html", "landing")
	contact := `<a href="mailto:` + supportEmailHrefMarker + `">` +
		supportEmailDisplayMarker + `</a>`
	writePage("privacy.html", contact)
	writePage("support.html", contact)

	const syntheticEmail = "support+site@example.invalid"
	const syntheticRecipient = "support+site@example.invalid"
	if err := renderSitePages(source, out, syntheticEmail, syntheticRecipient, "off"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"privacy.html", "support.html"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(data, []byte(syntheticEmail)); got != 2 {
			t.Errorf("%s contains the rendered support address %d times; expected 2", name, got)
		}
		if bytes.Contains(data, []byte(supportEmailDisplayMarker)) ||
			bytes.Contains(data, []byte(supportEmailHrefMarker)) {
			t.Errorf("%s contains an unresolved marker", name)
		}
	}
}

func TestRenderSitePagesSeparatesDisplayAndHrefEscaping(t *testing.T) {
	source := t.TempDir()
	out := t.TempDir()
	for _, page := range projectPages {
		body := "plain page"
		if page.supportDisplayMarkerCount != 0 {
			body = `<a href="mailto:` + supportEmailHrefMarker + `">` +
				supportEmailDisplayMarker + `</a>`
		}
		if err := os.WriteFile(filepath.Join(source, page.name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSitePages(
		source,
		out,
		"support<&?tag@example.invalid",
		"support%3C%26%3Ftag@example.invalid",
		"off",
	); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"privacy.html", "support.html"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(data, []byte(`href="mailto:support%3C%26%3Ftag@example.invalid"`)) {
			t.Errorf("%s did not use the URL-escaped href recipient", name)
		}
		if !bytes.Contains(data, []byte(`>support&lt;&amp;?tag@example.invalid</a>`)) {
			t.Errorf("%s did not keep display text separate from the href recipient", name)
		}
	}
}

func TestRenderSitePagesRejectsIncompleteTemplate(t *testing.T) {
	source := t.TempDir()
	for _, page := range projectPages {
		body := "plain page"
		if page.name == "support.html" {
			body = supportEmailDisplayMarker
		}
		if err := os.WriteFile(filepath.Join(source, page.name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSitePages(
		source,
		t.TempDir(),
		"support@example.invalid",
		"support@example.invalid",
		"off",
	); err == nil {
		t.Fatal("incomplete support-page templates were accepted")
	}
}

func TestRenderSitePagesRejectsSwappedSupportMarkers(t *testing.T) {
	source := t.TempDir()
	for _, page := range projectPages {
		body := "plain page"
		if page.supportDisplayMarkerCount != 0 {
			body = `<a href="mailto:` + supportEmailDisplayMarker + `">` +
				supportEmailHrefMarker + `</a>`
		}
		if err := os.WriteFile(filepath.Join(source, page.name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := renderSitePages(
		source,
		t.TempDir(),
		"support@example.invalid",
		"support@example.invalid",
		"off",
	); err == nil {
		t.Fatal("swapped support display and href markers were accepted")
	}
}

func TestTrackedProjectPagesUseSupportMarker(t *testing.T) {
	for _, page := range projectPages {
		data, err := os.ReadFile(filepath.Join("..", "..", "docs", page.name))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(data, []byte(supportEmailDisplayMarker)); got != page.supportDisplayMarkerCount {
			t.Errorf("docs/%s has %d support-email display markers; expected %d", page.name, got, page.supportDisplayMarkerCount)
		}
		if got := bytes.Count(data, []byte(supportEmailHrefMarker)); got != page.supportHrefMarkerCount {
			t.Errorf("docs/%s has %d support-email href markers; expected %d", page.name, got, page.supportHrefMarkerCount)
		}
		if bytes.Contains(data, []byte(bibletext.SupportEmail())) {
			t.Errorf("docs/%s duplicates the configured support address", page.name)
		}
	}
}

// --- the NKJV switch's passages -------------------------------------------------

func TestNKJVTextPassagesFollowTheState(t *testing.T) {
	const page = "Before.<!--nkjv-text:off--> Only while off.<!--/nkjv-text:off-->" +
		"<!--nkjv-text:on--> Only while on.<!--/nkjv-text:on--> After."
	for state, want := range map[string]string{
		"off": "Before. Only while off. After.",
		"on":  "Before. Only while on. After.",
	} {
		got, err := keepNKJVTextPassages("page.html", []byte(page), state)
		if err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		if string(got) != want {
			t.Errorf("%s: rendered %q, want %q", state, got, want)
		}
	}
	plain := []byte("A page with no marked passage.\n")
	for _, state := range []string{"on", "off"} {
		got, err := keepNKJVTextPassages("page.html", plain, state)
		if err != nil || !bytes.Equal(got, plain) {
			t.Errorf("%s: an unmarked page came back as %q (%v)", state, got, err)
		}
	}
}

func TestNKJVTextPassagesRefuseMalformedMarkers(t *testing.T) {
	for name, page := range map[string]string{
		"unclosed":     "a<!--nkjv-text:off--> b",
		"wrong close":  "a<!--nkjv-text:off--> b<!--/nkjv-text:on-->",
		"stray close":  "a b<!--/nkjv-text:off-->",
		"nested":       "<!--nkjv-text:off-->a<!--nkjv-text:on-->b<!--/nkjv-text:on--><!--/nkjv-text:off-->",
		"unknown":      "<!--nkjv-text:maybe-->a<!--/nkjv-text:maybe-->",
		"unterminated": "<!--nkjv-text:off a",
	} {
		for _, state := range []string{"on", "off"} {
			if got, err := keepNKJVTextPassages("page.html", []byte(page), state); err == nil {
				t.Errorf("%s (%s): accepted as %q", name, state, got)
			}
		}
	}
}

// No state, no pages: a build that has not said which state the reader is in
// cannot pick one for the pages beside it.
func TestRenderSitePagesRequiresTheNKJVState(t *testing.T) {
	for _, state := range []string{"", "yes", "On"} {
		err := renderSitePages(filepath.Join("..", "..", "docs"), t.TempDir(),
			bibletext.SupportEmail(), bibletext.SupportMailtoRecipient(), state)
		if err == nil {
			t.Errorf("-nkjv-text %q was accepted", state)
		}
	}
}

// The support page in both states. Off, it is the page as it was before the
// switch, and says NKJV links show no text in the browser; on, that sentence is
// gone, because the NKJV's links then show the passage and the note there as
// every other link does.
func TestTheSupportPageInBothNKJVStates(t *testing.T) {
	const exception = "NKJV links are the one exception"
	src, err := os.ReadFile(filepath.Join("..", "..", "docs", "support.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte("<!--nkjv-text:off-->")) {
		t.Fatal("docs/support.html marks no passage for the switch")
	}
	render := func(state string) string {
		t.Helper()
		out := t.TempDir()
		if err := renderSitePages(filepath.Join("..", "..", "docs"), out,
			bibletext.SupportEmail(), bibletext.SupportMailtoRecipient(), state); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
		b, err := os.ReadFile(filepath.Join(out, "support.html"))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("nkjv-text")) {
			t.Errorf("%s: a marker reached the page", state)
		}
		return string(b)
	}
	off, on := render("off"), render("on")
	if !strings.Contains(off, "right\n  in their browser. "+exception) {
		t.Error("off: the page no longer says NKJV links show no text in the browser")
	}
	if strings.Contains(on, exception) || strings.Contains(on, "can't be published") {
		t.Error("on: the page still says the NKJV's text cannot be on the web")
	}
	if !strings.Contains(on, "right\n  in their browser.</p>") {
		t.Error("on: the sentence the exception followed is not closed where it was")
	}
}
