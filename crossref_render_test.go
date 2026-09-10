package bibletext

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// TestRenderCrossRefPanel draws the cross-references panel at phone size —
// the popup's own dimensions on a 6.9" iPhone, the app's own theme and fonts,
// the real Treasury index from the cache and a real decoded canon — for a set
// of selections that exercise every block the panel can compose, and writes
// one PNG per selection. It is how the panel's composition is looked at
// without a device, which is what found its first-pass defects (the accordion
// title forcing the list wider than the panel, the notice shown twice, the
// two row idioms) — see docs/BACKLOG.md, "NKJV cross references: the panel's
// second pass".
//
// Gated on two environment variables, so the default run never needs the
// files: BIBLETEXT_RENDER_XREFS names a decoded canon (the JSON
// TestLiveAPIBibleFullCanon writes to BIBLETEXT_FULL_CANON_OUT; licensed
// text, so it lives outside the repository) and BIBLETEXT_RENDER_OUT the
// directory for the PNGs. The Treasury zip must already be cached (open the
// panel once in the app, or run the app's fetch).
//
//	BIBLETEXT_RENDER_XREFS=/path/to/nkjv-canon.json BIBLETEXT_RENDER_OUT=/tmp/xrefs \
//	  go test -run TestRenderCrossRefPanel -v .
//
// Selections are rendered with the publisher's block ON (the nkjvxrefs
// build's view) and, for the first, OFF as the control everyone has today.
func TestRenderCrossRefPanel(t *testing.T) {
	src, out := os.Getenv("BIBLETEXT_RENDER_XREFS"), os.Getenv("BIBLETEXT_RENDER_OUT")
	if src == "" || out == "" {
		t.Skip("BIBLETEXT_RENDER_XREFS and BIBLETEXT_RENDER_OUT not both set — render skipped")
	}
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var bd BibleData
	if err := json.Unmarshal(body, &bd); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	licenseNKJVForTest(t)
	if err := ensureCrossRefs(); err != nil {
		t.Fatalf("the Treasury index must be cached for this render: %v", err)
	}

	// The popup's size on a 6.9" phone: aiPanelSize of a 430×932 canvas,
	// then the panel's own 460-point ceiling (showCrossRefs).
	ps := fyne.NewSize(382, 460)

	render := func(name, book string, ch, lo, hi int, flag, open bool) {
		withPublisherCrossRefs(t, "nkjv", flag)
		st := &AppState{Bible: &bd, CurrentBook: book, CurrentChapter: ch, CurrentVersion: "nkjv", theme: th}
		pal := st.pal()
		span := selSpan{lo: lo, hi: hi}
		lst := buildCrossRefList(st, selectionVerses(st, "", span), crossRefsForSelection(st, "", span), nil, pal, func(crossRef) {})
		if open && lst.Disclosure != nil {
			lst.Disclosure.Open(0)
		}

		// The same chrome showCrossRefs mounts around the list.
		title := canvas.NewText("Cross-references", pal.Text)
		title.TextStyle = fyne.TextStyle{Bold: true}
		title.TextSize = 22
		cite := canvas.NewText(citationForSelection(st, "", span), pal.Accent)
		cite.TextStyle = fyne.TextStyle{Bold: true}
		cite.TextSize = subheadingTextSize
		header := container.NewVBox(title, cite, widget.NewSeparator())
		scroll := container.NewVScroll(container.NewVBox(lst.Objects...))
		scroll.SetMinSize(fyne.NewSize(ps.Width-44, ps.Height-150))
		sources, notice := crossRefFooterCredit(st)
		credit := canvas.NewText(sources, pal.TextMuted)
		credit.TextSize = 11
		footerLines := []fyne.CanvasObject{widget.NewSeparator(), credit}
		if notice != "" {
			footerLines = append(footerLines, crossRefCaption(notice))
		}
		footerLines = append(footerLines, container.NewHBox(layout.NewSpacer(), widget.NewButton("Close", func() {})))
		content := surface(container.NewPadded(container.NewBorder(header, container.NewVBox(footerLines...), nil, nil, container.NewStack(scroll))), pal.SurfaceAlt, pal.Border, fyne.Size{})

		w := test.NewWindow(content)
		w.Resize(ps)
		img := w.Canvas().Capture()
		w.Close()
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: parallels=%d publisher=%d treasury=%d objects=%d", name, lst.Parallels, lst.PublisherRows, lst.TSKRows, len(lst.Objects))
	}
	render("john3-16-off", "John", 3, 16, 16, false, false) // the control: today's panel
	render("john3-16-on", "John", 3, 16, 16, true, false)
	render("john3-16-on-open", "John", 3, 16, 16, true, true)
	render("john3-1to3-on", "John", 3, 1, 3, true, false)
	render("matt3-1-on", "Matthew", 3, 1, 1, true, false) // parallels + publisher + Treasury
	render("psalm3-1-on", "Psalms", 3, 1, 1, true, false) // a title note
	render("psalm3-2-on", "Psalms", 3, 2, 2, true, false) // the empty state
}
