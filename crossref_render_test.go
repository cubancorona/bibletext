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
// content fits the panel's width, and that nothing in the list wants more
// than the width the list was given: as it opens, with every group open and
// with every note closed.
//
// Each selection is written as it opens, then a view at a time down the list
// (-p2, -p3 …), with every parenthesised group open and scrolled to the first
// (-open; plus -b-open on John 3:16 for its lone group alone), and with every
// note of several pieces closed and scrolled to the first (-closed).
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
// view) and, for John 3:16 and Matthew 3:1, OFF as the control everyone has
// today — the second with a parallel row on screen, so the store build's
// footer carries both of its credits. The same canon is put through the
// apparatus census (publisher_xrefs_census_test.go), the assertion the live
// canon test makes of a fresh download.
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
	measured := 0 // lists the width check measured
	frames := []frame{
		{"john3-16-off", "John", 3, 16, 16, false},         // the control: today's panel
		{"matt3-1-off", "Matthew", 3, 1, 1, false},         // the control with parallels: two credits in the footer
		{"john3-16-on", "John", 3, 16, 16, true},           // three cards, a group, a lone group
		{"john3-1to3-on", "John", 3, 1, 3, true},           // keyed labels
		{"matt3-1-on", "Matthew", 3, 1, 1, true},           // parallels, a self-range, the Treasury
		{"psalm3-1-on", "Psalms", 3, 1, 1, true},           // the title's note
		{"psalm3-2-on", "Psalms", 3, 2, 2, true},           // the empty run
		{"1sam19-11-on", "1 Samuel", 19, 11, 11, true},     // a citation of a Psalm's title
		{"1sam19-12-on", "1 Samuel", 19, 12, 12, true},     // inside the stretch the feed hangs one caller late (Ruth 3:12 to 2 Samuel 24:25): the edition's note
		{"heb1-6-on", "Hebrews", 1, 6, 6, true},            // a citation with its basis, LXX and DSS
		{"gen7-2-on", "Genesis", 7, 2, 2, true},            // a citation of a whole chapter
		{"acts10-15-on", "Acts", 10, 15, 15, true},         // three groups among two direct citations
		{"john18-37-on", "John", 18, 37, 37, true},         // a note that is one group of nine and nothing else
		{"luke6-20-on", "Luke", 6, 20, 20, true},           // the book carried into a group from the citation before it
		{"acts26-6-on", "Acts", 26, 6, 6, true},            // the largest group, sixteen citations
		{"1sam2-1-on", "1 Samuel", 2, 1, 1, true},          // four notes on four words, one naming the passage itself
		{"1sam16-16-on", "1 Samuel", 16, 16, 16, true},     // in the same stretch, as the edition has it
		{"matt6-9-on", "Matthew", 6, 9, 9, true},           // the passage's own extent first, then its parallel and a group
		{"num4-21-on", "Numbers", 4, 21, 21, true},         // the note on the heading before the verse
		{"psalm51-1-on", "Psalms", 51, 1, 1, true},         // a title with a note, above the verse with one
		{"mark4-29-on", "Mark", 4, 29, 29, true},           // a verse the chapter does not have, inside a group
		{"amos2-1-on", "Amos", 2, 1, 1, true},              // a citation the feed did not link, first in its note
		{"rev15-3-on", "Revelation", 15, 3, 3, true},       // a note that is the verse itself and nothing else
		{"luke7-22-on", "Luke", 7, 22, 22, true},           // seven notes on one verse
		{"ruth3-11-on", "Ruth", 3, 11, 11, true},           // a note keyed to no word: no letter, an empty margin
		{"psalm119-132-on", "Psalms", 119, 132, 132, true}, // the second: a note keyed to no word after one keyed to a word
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
				// A frame with every parenthesised group open, scrolled to the
				// first: the group's citations as cards on their shaded ground.
				// Opening changes the list inside the scroll and nothing else.
				var groups []*crossRefGroupRow
				var listScroll *container.Scroll
				var listBox *fyne.Container
				walkTree(top, func(o fyne.CanvasObject) {
					switch x := o.(type) {
					case *crossRefGroupRow:
						if x.header != nil {
							groups = append(groups, x)
						}
					case *container.Scroll:
						if listScroll == nil {
							listScroll = x
						}
					case *container.ThemeOverride:
						if box, ok := x.Content.(*fyne.Container); ok && listBox == nil && listScroll != nil {
							listBox = box
						}
					}
				})
				// The popup's content cannot say what a row wants: the list
				// stands in a layout that reports no width, so that one wide
				// row cannot widen the panel. What can fail is the list
				// itself: nothing in it wants more than the width it was given.
				fits := func(state string) {
					if listScroll == nil {
						return // the panel shows a message, not a list
					}
					if listBox == nil {
						t.Errorf("%s, %s: no list under the panel's scroll to measure", name, state)
						return
					}
					measured++
					for i, problem := range crossRefListMisfits(listBox.Objects, listBox.Size().Width) {
						if i == 3 {
							break
						}
						t.Errorf("%s, %s: %s", name, state, problem)
					}
				}
				fits("as it opens")
				// The rest of the list as it opens, a view at a time: a note
				// that is one group, the notes below the first, the Treasury.
				if listScroll != nil {
					view := listScroll.Size().Height
					for page, y := 2, view-24; page <= 9 && y < listScroll.Content.Size().Height-24; page, y = page+1, y+view-24 {
						if most := listScroll.Content.Size().Height - view; y > most {
							y = most
						}
						listScroll.Offset = fyne.NewPos(0, y)
						listScroll.Refresh()
						writeRenderPNG(t, filepath.Join(out, fmt.Sprintf("%s-p%d.png", name, page)), w.Canvas().Capture())
					}
					listScroll.Offset = fyne.NewPos(0, 0)
					listScroll.Refresh()
				}
				if f.name == "john3-16-on" && listScroll != nil {
					var lone []*crossRefGroupRow
					for _, g := range groups {
						if g.letter != nil {
							lone = append(lone, g)
						}
					}
					if len(lone) > 0 {
						for _, g := range lone {
							test.Tap(g.header)
						}
						fits("lone group open")
						drv := fyne.CurrentApp().Driver()
						y := drv.AbsolutePositionForObject(lone[0]).Y - drv.AbsolutePositionForObject(listScroll).Y + listScroll.Offset.Y - 6
						if most := listScroll.Content.Size().Height - listScroll.Size().Height; y > most {
							y = most
						}
						if y < 0 {
							y = 0
						}
						listScroll.Offset = fyne.NewPos(0, y)
						listScroll.Refresh()
						writeRenderPNG(t, filepath.Join(out, name+"-b-open.png"), w.Canvas().Capture())
						for _, g := range lone {
							test.Tap(g.header)
						}
						listScroll.Offset = fyne.NewPos(0, 0)
						listScroll.Refresh()
					}
				}
				if len(groups) > 0 && listScroll != nil {
					size, pos := top.Content.Size(), top.Content.Position()
					for _, g := range groups {
						test.Tap(g.header)
					}
					if top.Content.Size() != size || top.Content.Position() != pos {
						t.Errorf("%s: opening the groups moved the panel: %v at %v, was %v at %v", name, top.Content.Size(), top.Content.Position(), size, pos)
					}
					if m := top.Content.MinSize().Width; m > inner {
						t.Errorf("%s: with the groups open the popup's content wants %.0f points and the panel gives it %.0f", name, m, inner)
					}
					fits("every group open")
					drv := fyne.CurrentApp().Driver()
					y := drv.AbsolutePositionForObject(groups[0]).Y - drv.AbsolutePositionForObject(listScroll).Y + listScroll.Offset.Y - 6
					if most := listScroll.Content.Size().Height - listScroll.Size().Height; y > most {
						y = most
					}
					if y < 0 {
						y = 0
					}
					listScroll.Offset = fyne.NewPos(0, y)
					listScroll.Refresh()
					writeRenderPNG(t, filepath.Join(out, name+"-open.png"), w.Canvas().Capture())
					members := 0
					for _, g := range groups {
						members += len(g.members.Objects)
					}
					t.Logf("%s: %d groups open, %d cards on their ground", name, len(groups), members)
					for _, g := range groups {
						test.Tap(g.header)
					}
					listScroll.Offset = fyne.NewPos(0, 0)
					listScroll.Refresh()
				}
				// A frame with every note of several pieces closed to its
				// printed line: the list as the edition's own column, each
				// note under its letter. Closing changes the list inside the
				// scroll and nothing else, as opening a group does.
				var noteRows []*crossRefNoteRow
				walkTree(top, func(o fyne.CanvasObject) {
					if x, ok := o.(*crossRefNoteRow); ok {
						noteRows = append(noteRows, x)
					}
				})
				if len(noteRows) > 0 {
					size, pos := top.Content.Size(), top.Content.Position()
					for _, n := range noteRows {
						test.Tap(n.header)
					}
					if top.Content.Size() != size || top.Content.Position() != pos {
						t.Errorf("%s: closing the notes moved the panel: %v at %v, was %v at %v", name, top.Content.Size(), top.Content.Position(), size, pos)
					}
					if m := top.Content.MinSize().Width; m > inner {
						t.Errorf("%s: with the notes closed the popup's content wants %.0f points and the panel gives it %.0f", name, m, inner)
					}
					fits("every note closed")
					// Scrolled to the first of them, which the passage and the
					// parallels can stand a screen above.
					if listScroll != nil {
						drv := fyne.CurrentApp().Driver()
						y := drv.AbsolutePositionForObject(noteRows[0]).Y - drv.AbsolutePositionForObject(listScroll).Y + listScroll.Offset.Y - 6
						if most := listScroll.Content.Size().Height - listScroll.Size().Height; y > most {
							y = most
						}
						if y < 0 {
							y = 0
						}
						listScroll.Offset = fyne.NewPos(0, y)
						listScroll.Refresh()
					}
					writeRenderPNG(t, filepath.Join(out, name+"-closed.png"), w.Canvas().Capture())
					t.Logf("%s: %d notes closed to their printed lines", name, len(noteRows))
					for _, n := range noteRows {
						test.Tap(n.header)
					}
					if listScroll != nil {
						listScroll.Offset = fyne.NewPos(0, 0)
						listScroll.Refresh()
					}
				}
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

	if measured == 0 {
		t.Error("control: the width check measured no list")
	}
	census := noteApparatusCensus(&bd)
	assertNoteCensus(t, census)
	t.Logf("census: %d notes, words %v, %d whole-chapter ids, %d title citations, %d bases, %d tagged citations inside parentheses, %d cards and %d remainders by the feed's tags over each chapter",
		census.Notes, census.Words, census.WholeChapters, census.TitleCitations, census.Bases, census.InParentheses, census.Cards, census.Lines)
	// The shorthand the group rows read their citations by, held to the
	// feed's own tags over the same canon.
	shorthand := noteShorthandCensus(&bd)
	assertShorthandCensus(t, shorthand)
	t.Logf("shorthand: %d of %d tagged citations read to the feed's id, %d mistagged by the feed inside a group; %d groups holding %d citations, %d of them tagged; %d read, %d in the loaded text; unread %q; unresolved %q",
		shorthand.Agree, shorthand.Tagged, len(shorthand.GroupMistags), shorthand.Groups, shorthand.GroupCitations, shorthand.GroupTagged, shorthand.Read, shorthand.Resolved, shorthand.Unread, shorthand.Unresolved)
	for _, m := range shorthand.GroupMistags {
		t.Logf("feed mistag: %s", m)
	}
	// And the notes as the panel draws them: every one whole, every letter
	// before its word, the passage the verse's own text.
	pieces := assertCanonPieceCensus(t, &bd)
	t.Logf("pieces: %d notes (%d on verses, %d on titles, %d on headings); %d cards, %d of the reader's own passage, %d words, %d groups holding %d cards, %d own and %d words; %d notes are one citation (%d of them the reader's own passage), %d one group (holding %d citations, %d with nothing to follow), %d words alone and %d several pieces; at most %d to a verse; %d letters drawn past white space, %d at the head of a split word, %d between two words run together; %d notes shown one caller back, %d keyed to no word, %d callers with no content; %d problems",
		pieces.Notes, pieces.OnVerses, pieces.OnTitles, pieces.OnHeadings, pieces.Cards, pieces.Own, pieces.Words, pieces.Groups, pieces.GroupCards, pieces.GroupOwn, pieces.GroupWords,
		pieces.Alone, pieces.AloneOwn, pieces.GroupAlone, pieces.GroupAloneCards, pieces.GroupAloneBare, pieces.WordsAlone, pieces.Folded, pieces.MostOnAVerse, pieces.Moved, pieces.WordStarts, pieces.BetweenWords,
		pieces.Shifted, pieces.Unkeyed, pieces.Silent, len(pieces.Problems))
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
		case *container.ThemeOverride:
			return walk(x.Content, at)
		case *tapCard:
			return walk(x.content, at)
		case *crossRefGroupRow:
			return walk(x.box, at)
		}
		return false
	}
	walk(scroll.Content, -scroll.Content.Position().Y)
	return scroll, y, found
}
