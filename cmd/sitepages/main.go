// Command sitepages renders the hand-written root pages that are published
// alongside the generated web reader.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"

	bibletext "bibletext"
)

const (
	supportEmailDisplayMarker = "{{BIBLETEXT_SUPPORT_EMAIL_DISPLAY}}"
	supportEmailHrefMarker    = "{{BIBLETEXT_SUPPORT_EMAIL_HREF}}"
)

type projectPage struct {
	name                      string
	supportDisplayMarkerCount int
	supportHrefMarkerCount    int
}

var projectPages = []projectPage{
	{name: "index.html"},
	{name: "privacy.html", supportDisplayMarkerCount: 1, supportHrefMarkerCount: 1},
	{name: "support.html", supportDisplayMarkerCount: 1, supportHrefMarkerCount: 1},
}

func main() {
	source := flag.String("source", "docs", "directory containing the root-page templates")
	out := flag.String("out", "build/site", "directory receiving rendered root pages")
	flag.Parse()
	if err := renderSitePages(
		*source,
		*out,
		bibletext.SupportEmail(),
		bibletext.SupportMailtoRecipient(),
	); err != nil {
		log.Fatal(err)
	}
}

func renderSitePages(sourceDir, outDir, supportEmail, supportMailtoRecipient string) error {
	type renderedPage struct {
		name string
		data []byte
	}
	rendered := make([]renderedPage, 0, len(projectPages))
	displayMarker := []byte(supportEmailDisplayMarker)
	hrefMarker := []byte(supportEmailHrefMarker)
	displaySlot := []byte(">" + supportEmailDisplayMarker + "</a>")
	hrefSlot := []byte("href=\"mailto:" + supportEmailHrefMarker + "\"")
	for _, page := range projectPages {
		data, err := os.ReadFile(filepath.Join(sourceDir, page.name))
		if err != nil {
			return fmt.Errorf("read %s: %w", page.name, err)
		}
		if got := bytes.Count(data, displayMarker); got != page.supportDisplayMarkerCount {
			return fmt.Errorf("%s has %d support-email display markers; expected %d", page.name, got, page.supportDisplayMarkerCount)
		}
		if got := bytes.Count(data, hrefMarker); got != page.supportHrefMarkerCount {
			return fmt.Errorf("%s has %d support-email href markers; expected %d", page.name, got, page.supportHrefMarkerCount)
		}
		if got := bytes.Count(data, displaySlot); got != page.supportDisplayMarkerCount {
			return fmt.Errorf("%s has %d support-email display slots; expected %d", page.name, got, page.supportDisplayMarkerCount)
		}
		if got := bytes.Count(data, hrefSlot); got != page.supportHrefMarkerCount {
			return fmt.Errorf("%s has %d support-email href slots; expected %d", page.name, got, page.supportHrefMarkerCount)
		}
		if bytes.Contains(data, []byte(supportEmail)) {
			return fmt.Errorf("%s contains the configured support address instead of the template marker", page.name)
		}
		if supportMailtoRecipient != supportEmail && bytes.Contains(data, []byte(supportMailtoRecipient)) {
			return fmt.Errorf("%s contains the formatted support recipient instead of the template marker", page.name)
		}
		data = bytes.ReplaceAll(data, displayMarker, []byte(html.EscapeString(supportEmail)))
		data = bytes.ReplaceAll(data, hrefMarker, []byte(html.EscapeString(supportMailtoRecipient)))
		if bytes.Contains(data, displayMarker) || bytes.Contains(data, hrefMarker) {
			return fmt.Errorf("%s still contains an unresolved support-email marker", page.name)
		}
		rendered = append(rendered, renderedPage{name: page.name, data: data})
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	for _, page := range rendered {
		if err := os.WriteFile(filepath.Join(outDir, page.name), page.data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", page.name, err)
		}
	}
	return nil
}
