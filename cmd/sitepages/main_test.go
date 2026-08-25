package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	bibletext "bibletext"
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
	if err := renderSitePages(source, out, syntheticEmail, syntheticRecipient); err != nil {
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
