package bibletext

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TestRenderCrossRefPanel mounts the cross-references panel itself — the
// modal popup showCrossRefs shows, the app's own theme and fonts, the real
// Treasury index from the cache and a real decoded canon — on test canvases
// the size of two phones, in both appearances, for the selections that
// exercise every shape the list can take, and writes one PNG per frame. It is
// how the panel's composition is looked at without a device; the first pass's
// defects (a fold whose title widened the list past the panel, the notice
// shown twice, two row idioms) were found this way, with a harness that
// mounted the list alone at a guessed size, which is why this one mounts the
// real popup on a canvas the size of the phone and asserts the popup's
// content fits the panel's width.
//
// Gated on two environment variables, so the default run never needs the
// files: BIBLETEXT_RENDER_XREFS names a decoded canon (the JSON
// TestLiveAPIBibleFullCanon writes to BIBLETEXT_FULL_CANON_OUT; licensed
// text, so it lives outside the repository) and BIBLETEXT_RENDER_OUT the
// directory for the PNGs. The Treasury zip is taken from the machine's cache
// when it is there (open the panel once in the app), and fetched otherwise.
//
//	BIBLETEXT_RENDER_XREFS=/path/to/nkjv-canon.json BIBLETEXT_RENDER_OUT=/tmp/xrefs \
//	  go test -run TestRenderCrossRefPanel -v .
//
// Selections are rendered with the publisher's run ON (the nkjvxrefs build's
// view) and, for the first, OFF as the control everyone has today. The same
// canon is put through the apparatus census (publisher_xrefs_census_test.go),
// the assertion the live canon test makes of a fresh download.
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
	licenseNKJVForTest(t)
	// The suite keeps its own cache, so hand it the machine's Treasury zip
	// (read, never written); without one, ensureCrossRefs fetches into the
	// suite's directory, not the machine's.
	if zip, err := os.ReadFile(realCachePath(crossRefCachePath())); err == nil {
		if err := os.WriteFile(crossRefCachePath(), zip, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureCrossRefs(); err != nil {
		t.Fatalf("no Treasury index, neither the machine's nor fetched: %v", err)
	}
	// The panel's load runs where the test stands, after its first frame: the
	// panel opens on its spinner, the chrome is laid out, and the list lands
	// on the next frame, which is the order the app has.
	var pending func()
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	crossRefsRun = func(work func()) { pending = work }
	crossRefsLoad = func() error { return nil }
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	prevVariant := appearanceVariant
	t.Cleanup(func() { appearanceVariant = prevVariant })

	type frame struct {
		name, book string
		ch, lo, hi int
		flag       bool
	}
	frames := []frame{
		{"john3-16-off", "John", 3, 16, 16, false},     // the control: today's panel
		{"john3-16-on", "John", 3, 16, 16, true},       // three cards, a group, a lone group
		{"john3-1to3-on", "John", 3, 1, 3, true},       // keyed labels
		{"matt3-1-on", "Matthew", 3, 1, 1, true},       // parallels, a self-range, the Treasury
		{"psalm3-1-on", "Psalms", 3, 1, 1, true},       // the title's note
		{"psalm3-2-on", "Psalms", 3, 2, 2, true},       // the empty run
		{"1sam19-12-on", "1 Samuel", 19, 12, 12, true}, // a citation of a Psalm's title
		{"heb1-6-on", "Hebrews", 1, 6, 6, true},        // a citation with its basis, LXX and DSS
		{"gen7-2-on", "Genesis", 7, 2, 2, true},        // a citation of a whole chapter
	}
	phones := []struct {
		name string
		size fyne.Size
	}{{"402x874", fyne.NewSize(402, 874)}, {"375x667", fyne.NewSize(375, 667)}}
	scrolledFrames := map[string]struct{ suffix, label string }{
		"john3-16-on": {"-treasury", treasuryLabel},
		"matt3-1-on":  {"-nkjv", publisherRunLabel("NKJV", 0, false)},
	}

	for _, phone := range phones {
		for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			look := "light"
			if variant == theme.VariantDark {
				look = "dark"
			}
			v := variant
			appearanceVariant = func(*AppState) fyne.ThemeVariant { return v }
			app.Settings().SetTheme(forcedVariant{Theme: th, v: v})
			for _, f := range frames {
				name := fmt.Sprintf("%s-%s-%s", f.name, phone.name, look)
				withPublisherCrossRefs(t, "nkjv", f.flag)
				w := test.NewWindow(widget.NewLabel("reading"))
				w.Resize(phone.size)
				st := &AppState{Bible: &bd, CurrentBook: f.book, CurrentChapter: f.ch, CurrentVersion: "nkjv", theme: th, window: w}
				pending = nil
				showCrossRefs(st, "", selSpan{lo: f.lo, hi: f.hi})
				if pending == nil {
					t.Fatalf("%s: the panel did not ask for the Treasury", name)
				}
				w.Canvas().Capture() // the spinner frame: lays the chrome out
				pending()
				top, ok := w.Canvas().Overlays().Top().(*widget.PopUp)
				if !ok {
					t.Fatalf("%s: no popup on the canvas", name)
				}
				ps := sheetPanelSize(st, w.Canvas())
				img := w.Canvas().Capture()
				// The modal lays its content out inset by the theme's inner
				// padding, and never narrower than the content's MinSize: a
				// content wanting more than that inset width would widen the
				// popup past the panel, which is the defect the first pass had.
				inner := ps.Width - theme.InnerPadding()
				if m := top.Content.MinSize().Width; m > inner {
					t.Errorf("%s: the popup's content wants %.0f points and the panel gives it %.0f: something in it cannot wrap", name, m, inner)
				}
				if got := top.Content.Size().Width; got != inner {
					t.Errorf("%s: the popup's content is %.0f wide, want the panel's %.0f", name, got, inner)
				}
				writeRenderPNG(t, filepath.Join(out, name+".png"), img)
				all := strings.Join(treeTexts(top), "\n")
				if n := strings.Count(all, "Thomas Nelson"); n != 1 {
					t.Errorf("%s: the notice stands %d times in the panel, want once (the footer)", name, n)
				}
				if strings.Contains(strings.ToLower(all), "compare") {
					t.Errorf("%s: a compare word or legend is in the panel", name)
				}
				t.Logf("%s: panel %.0f×%.0f, content wants %.0f wide; %d texts", name, ps.Width, ps.Height, top.Content.MinSize().Width, len(treeTexts(top)))
				if extra, ok := scrolledFrames[f.name]; ok {
					// A second frame scrolled to a label further down: where the
					// Treasury's run begins under John 3:16's cards, and where
					// the publisher's begins under Matthew 3:1's parallels.
					scroll, y, found := labelOffset(top, extra.label)
					if !found {
						t.Errorf("%s: no %q label to scroll to", name, extra.label)
					} else {
						scroll.Offset = fyne.NewPos(0, y-6)
						scroll.Refresh()
						writeRenderPNG(t, filepath.Join(out, name+extra.suffix+".png"), w.Canvas().Capture())
					}
				}
				top.Hide()
				w.Close()
			}
		}
	}

	census := noteApparatusCensus(&bd)
	assertNoteCensus(t, census)
	t.Logf("census: %d notes, words %v, %d whole-chapter ids, %d title citations, %d bases, %d tagged citations inside parentheses, %d cards and %d caption lines over single-verse selections",
		census.Notes, census.Words, census.WholeChapters, census.TitleCitations, census.Bases, census.InParentheses, census.Cards, census.Lines)
}

func writeRenderPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// labelOffset finds the list's scroll inside the popup and a section label's
// offset within the scroll's content, for a frame scrolled to it.
func labelOffset(top *widget.PopUp, label string) (scroll *container.Scroll, y float32, found bool) {
	walkTree(top, func(o fyne.CanvasObject) {
		if s, ok := o.(*container.Scroll); ok && scroll == nil {
			scroll = s
		}
	})
	if scroll == nil {
		return nil, 0, false
	}
	var walk func(o fyne.CanvasObject, at float32) bool
	walk = func(o fyne.CanvasObject, at float32) bool {
		at += o.Position().Y
		if txt, ok := o.(*canvas.Text); ok && txt.Text == label {
			y, found = at, true
			return true
		}
		switch x := o.(type) {
		case *fyne.Container:
			for _, c := range x.Objects {
				if walk(c, at) {
					return true
				}
			}
		case *tapCard:
			return walk(x.content, at)
		}
		return false
	}
	walk(scroll.Content, -scroll.Content.Position().Y)
	return scroll, y, found
}
