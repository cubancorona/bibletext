// Command sitepages renders the hand-written root pages that are published
// alongside the generated web reader.
//
// A sentence in them can be true in one state of the site's NKJV switch
// (cmd/websitegen/nkjv_text.go) and false in the other — that NKJV links show
// no text in the browser, say. Such a passage is wrapped in
// <!--nkjv-text:off-->…<!--/nkjv-text:off--> (or on), and -nkjv-text keeps the
// passages of the state the site is published in and drops the others.
// scripts/publish-site.sh passes the state the generator itself reports, so
// the pages and the reader cannot be built in different states. A page with no
// marked passage renders exactly as it did before markers existed, and so does
// a marked page in the state its passage describes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"strings"

	bibletext "github.com/cubancorona/bibletext"
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
	nkjvText := flag.String("nkjv-text", "",
		"the state of the site's NKJV switch, on or off; required, and passed by scripts/publish-site.sh "+
			"from the generator's own -print-nkjv-text")
	flag.Parse()
	if err := renderSitePages(
		*source,
		*out,
		bibletext.SupportEmail(),
		bibletext.SupportMailtoRecipient(),
		*nkjvText,
	); err != nil {
		log.Fatal(err)
	}
}

func renderSitePages(sourceDir, outDir, supportEmail, supportMailtoRecipient, nkjvText string) error {
	if nkjvText != "on" && nkjvText != "off" {
		return fmt.Errorf("-nkjv-text is %q; it must be on or off, the state cmd/websitegen reports", nkjvText)
	}
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
		if data, err = keepNKJVTextPassages(page.name, data, nkjvText); err != nil {
			return err
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

// keepNKJVTextPassages keeps the passages of data marked for state, drops those
// marked for the other state, and removes every marker. Passages do not nest,
// and a marker that does not open or close a passage is refused rather than
// published.
func keepNKJVTextPassages(name string, data []byte, state string) ([]byte, error) {
	const open, closing = "<!--nkjv-text:", "<!--/nkjv-text:"
	src := string(data)
	var out strings.Builder
	for {
		i := strings.Index(src, open)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+len(open):]
		end := strings.Index(rest, "-->")
		if end < 0 {
			return nil, fmt.Errorf("%s has an unterminated nkjv-text marker", name)
		}
		which := rest[:end]
		if which != "on" && which != "off" {
			return nil, fmt.Errorf("%s marks a passage for nkjv-text state %q; the states are on and off", name, which)
		}
		rest = rest[end+len("-->"):]
		stop := closing + which + "-->"
		j := strings.Index(rest, stop)
		if j < 0 {
			return nil, fmt.Errorf("%s opens an nkjv-text:%s passage and never closes it", name, which)
		}
		inner := rest[:j]
		if strings.Contains(inner, open) || strings.Contains(inner, closing) {
			return nil, fmt.Errorf("%s nests an nkjv-text passage inside another", name)
		}
		if which == state {
			out.WriteString(inner)
		}
		src = rest[j+len(stop):]
	}
	if strings.Contains(src, closing) {
		return nil, fmt.Errorf("%s closes an nkjv-text passage it never opened", name)
	}
	out.WriteString(src)
	return []byte(out.String()), nil
}
