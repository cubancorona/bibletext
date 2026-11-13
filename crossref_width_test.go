//go:build !race

package bibletext

// THE LIST FITS THE NARROWEST PHONE, BY MEASUREMENT. A vertical scroll lays
// its content out at the content's own minimum width and clips the excess
// sideways with no bar to reach it (sheet_fit.go), so one child wanting more
// width than the panel has clips every row: the fold this list replaced did
// exactly that, its title at 18 bold with a button's chrome. The list is built
// of bounded kinds only (crossref_list.go) and the panel wraps it in
// squeezeWidthLayout as a belt; this is the test that can fail, under the
// app's own theme and fonts, which is why it stays out of the race run (the
// theme switch clears the font cache; widgettree_test.go).

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// narrowestPhoneBody is the width the list has on the narrowest phone: the
// panel's width on a 375×667 canvas less the 44 points of chrome around the
// list (showCrossRefs). The figure every bound in this file is written
// against is 283; the test fails if the arithmetic moves.
func narrowestPhoneBody(t *testing.T) float32 {
	t.Helper()
	limit := aiPanelSize(fyne.NewSize(375, 667)).Width - 44
	if limit != 283 {
		t.Fatalf("control: the narrowest body is %v points; the bounds here are written against 283", limit)
	}
	return limit
}

func appThemeForTest(t *testing.T) *bibleTheme {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	return th
}

func TestCrossRefListFitsTheNarrowestPhone(t *testing.T) {
	th := appThemeForTest(t)
	licenseNKJVForTest(t)
	limit := narrowestPhoneBody(t)
	bd := publisherFixture()
	bd.Verses["2 Samuel"] = map[int][]Verse{15: {{BookName: "2 Samuel", Book: "2 Samuel", Chapter: 15, Verse: 13, Text: "A fixture messenger came."}}}
	st := &AppState{Bible: bd, CurrentVersion: "nkjv", theme: th}
	type selection struct {
		book       string
		ch, lo, hi int
	}
	selections := []selection{
		{"John", 3, 1, 3}, {"John", 3, 2, 2}, {"Psalms", 3, 1, 1}, {"Psalms", 3, 2, 2},
		{"1 Samuel", 19, 12, 12}, {"Genesis", 7, 2, 2}, {"Hebrews", 1, 6, 6},
	}
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	checked, openedLone := 0, 0
	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		for _, flag := range []bool{false, true} {
			withPublisherCrossRefs(t, "nkjv", flag)
			for _, s := range selections {
				st.CurrentBook, st.CurrentChapter = s.book, s.ch
				lst := buildCrossRefList(st, versesBetween(bd, s.book, s.ch, s.lo, s.hi), listFixtureRefs(), nil, st.pal(), func(crossRef) {})
				if len(lst.Objects) == 0 {
					t.Fatalf("control: size %s flag %v, %s %d:%d-%d built an empty list", sizeOpt.ID, flag, s.book, s.ch, s.lo, s.hi)
				}
				checked++
				if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
					t.Errorf("size %s flag %v, %s %d:%d-%d: %s", sizeOpt.ID, flag, s.book, s.ch, s.lo, s.hi, strings.Join(problems, "; "))
				}
				// No selection here holds a parenthesised group inside a note:
				// each of the fixture's groups is the whole of its note and
				// starts closed as that group's row, so open the whole-note ones
				// and measure them too. The width with a group opened inside a
				// note is held by
				// TestTheListFitsTheNarrowestPhoneWithAGroupOpenInsideANote.
				inner := 0
				var lone []*crossRefGroupRow
				for _, g := range groupRowsIn(lst.Objects) {
					if g.letter == nil {
						inner++
					} else if g.header != nil {
						lone = append(lone, g)
					}
				}
				if inner != 0 {
					t.Fatalf("control: size %s flag %v, %s %d:%d-%d holds %d group rows inside notes; this test expects none", sizeOpt.ID, flag, s.book, s.ch, s.lo, s.hi, inner)
				}
				if len(lone) > 0 {
					for _, g := range lone {
						g.toggle()
						openedLone++
					}
					if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
						t.Errorf("size %s flag %v, %s %d:%d-%d (lone groups open): %s", sizeOpt.ID, flag, s.book, s.ch, s.lo, s.hi, strings.Join(problems, "; "))
					}
					for _, g := range lone {
						g.toggle()
					}
				}
				all := joinedTexts(lst.Objects)
				if strings.Contains(all, "Thomas Nelson") {
					t.Errorf("size %s flag %v, %s %d: the notice is in the list", sizeOpt.ID, flag, s.book, s.ch)
				}
				if strings.Contains(strings.ToLower(all), "compare") {
					t.Errorf("size %s flag %v, %s %d: a legend or compare word is in the list:\n%s", sizeOpt.ID, flag, s.book, s.ch, all)
				}
				// The notice stands once, in the footer, whatever the list shows.
				footer, sources := crossRefFooter(st, widget.NewButton("Close", func() {}))
				line, _ := crossRefFooterCredit(st, lst.Sections)
				setCrossRefSources(sources, line, limit)
				if n := strings.Count(strings.Join(treeTexts(footer), "\n"), "Thomas Nelson"); n != 1 {
					t.Errorf("size %s flag %v, %s %d: the notice stands %d times in the footer, want once", sizeOpt.ID, flag, s.book, s.ch, n)
				}
			}
		}
	}
	if checked != len(textSizeOptions)*2*len(selections) || openedLone == 0 {
		t.Fatalf("control: %d lists checked, %d lone groups opened", checked, openedLone)
	}

	// CONTROL: the fold this list replaced is reported twice over — for its
	// kind, and for the width its title wants.
	acc := widget.NewAccordion(widget.NewAccordionItem("More references — Treasury of Scripture Knowledge", widget.NewLabel("x")))
	if acc.MinSize().Width <= limit {
		t.Fatalf("control: the accordion's title fits in %v points, so the width check cannot be shown to fail", limit)
	}
	problems := crossRefListMisfits([]fyne.CanvasObject{acc}, limit)
	if len(problems) != 2 || !strings.Contains(problems[0], "Accordion") || problems[1] != "a Accordion in the list" {
		t.Errorf("an accordion in the list is reported as %q", problems)
	}
}

// THE TEXTS THAT DO NOT WRAP FIT TOO. A card's reference (16 bold), a
// parallel card's pericope title (12 italic) and the section labels (11 bold
// monospace) are the list's widest unwrapped texts, so each is measured at
// its widest: every title and passage label the synopsis can give a card,
// the widest run label the Psalter can produce, and the widest book names in
// a range label. A publisher's card stands on its note's padded ground beside
// the letter's margin (283 − 2×8 − 18 = 249 points), and 233 points inside an
// opened group's well, with its tag ("Title", "LXX", "LXX, DSS") beside the
// reference, so its reference line is held to those widths rather than the
// whole list's 283.
func TestEveryUnwrappedLineOfTheListFitsTheNarrowestPhone(t *testing.T) {
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	limit := narrowestPhoneBody(t)
	gospelOnce.Do(loadGospelParallels)
	if len(gospelPericopes) == 0 {
		t.Fatal("control: no pericopes loaded")
	}
	widest := func(texts []string, size float32, style fyne.TextStyle) (string, float32) {
		var text string
		var w float32
		for _, s := range texts {
			if m := fyne.MeasureText(s, size, style).Width; m > w {
				text, w = s, m
			}
		}
		return text, w
	}
	var titles, labels []string
	var parRefs []crossRef
	for _, p := range gospelPericopes {
		titles = append(titles, p.title)
		for _, g := range gospelColumns {
			for _, s := range p.spans[g.book] {
				ref := spanToCrossRef(g.book, s, p.title)
				labels = append(labels, ref.label())
				parRefs = append(parRefs, ref)
			}
		}
	}
	labels = append(labels, "Deuteronomy 31:30–32:45", "Song of Solomon 2:8–3:5", "2 Thessalonians 2:16–17", "1 Thessalonians 5:12–13", "Leviticus 11", "2 Kings 18–20")

	crossBookLabels := []string{
		"Leviticus 27:34–Numbers 1:1",
		"2 Chronicles 36:22–Ezra 1:3",
		"2 Chronicles 36:22–Ezra 1:4",
		"2 Chronicles 36:21–Ezra 1:5",
		"2 Chronicles 36:21–Ezra 1:2",
		"2 Chronicles 36:22–Ezra 1:2",
		"Jeremiah 52:1–Lamentations 1:5",
		"Haggai 2:20–Zechariah 1:1",
		"2 John 1:1–3 John 1:14",
		"Acts 28:17–Romans 1:1",
		"Ephesians 6:23–Philippians 1:2",
		"1 Thessalonians 5:28–2 Thessalonians 1:2",
		"2 John 1:13–3 John 1:14",
	}

	empty := &AppState{Bible: &BibleData{}, theme: th}
	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		sz := crossRefPanelSizes()

		// Parallel pericope titles stay single-line at Normal and wrap cleanly at
		// Large and Extra large without exceeding the narrowest phone's width.
		for _, title := range titles {
			tw := newCrossRefTitle(title, sz.title, lightPalette)
			tw.Resize(fyne.NewSize(limit, 100))
			r := test.WidgetRenderer(tw).(*crossRefPrintedRenderer)
			r.Layout(tw.Size())
			if sizeOpt.ID == "normal" && len(r.rows) != 1 {
				t.Errorf("Normal pericope title %q wrapped into %d rows; want 1", title, len(r.rows))
			}
			if tw.MinSize().Width > limit {
				t.Errorf("%s: pericope title %q MinSize.Width = %.1f, over %.0f", sizeOpt.ID, title, tw.MinSize().Width, limit)
			}
			for _, row := range r.rows {
				if rw := fyne.MeasureText(row.Text, row.TextSize, row.TextStyle).Width; rw > limit {
					t.Errorf("%s: pericope title row %q wants %.1f points, over %.0f", sizeOpt.ID, row.Text, rw, limit)
				}
			}
		}
		widestTitle, _ := widest(titles, sz.title, fyne.TextStyle{Italic: true})
		for _, badge := range []bool{false, true} {
			pCard := crossRefRowRuled(empty, passageCard(crossRef{Book: "Mark", Chapter: 5, Verse: 21, Parallel: true, Title: widestTitle}, badge), lightPalette, func(crossRef) {}, false)
			if problems := crossRefListMisfits([]fyne.CanvasObject{pCard}, limit); len(problems) != 0 {
				t.Errorf("%s: parallel card badge=%v with widest title %q: %s", sizeOpt.ID, badge, widestTitle, strings.Join(problems, "; "))
			}
		}
		for _, ref := range parRefs {
			bCard := crossRefRowRuled(empty, passageCard(ref, true), lightPalette, func(crossRef) {}, false)
			if problems := crossRefListMisfits([]fyne.CanvasObject{bCard}, limit); len(problems) != 0 {
				t.Errorf("%s: badged parallel card %q: %s", sizeOpt.ID, ref.label(), strings.Join(problems, "; "))
			}
		}
		occBadge := otherOccasionBadge(lightPalette)
		if occBadge.MinSize().Width > limit {
			t.Errorf("%s: otherOccasionBadge wants %.1f points, over %.0f", sizeOpt.ID, occBadge.MinSize().Width, limit)
		}

		// Every cross-book Treasury card and self-row fits the narrowest phone,
		// staying on one row while it fits and turning cleanly after the en dash
		// ("1 Thessalonians 5:28–" / "2 Thessalonians 1:2") when wider than 283pt.
		for _, cb := range crossBookLabels {
			cObj := crossRefRowRuled(empty, crossRefCard{Label: cb}, lightPalette, func(crossRef) {}, false)
			if problems := crossRefListMisfits([]fyne.CanvasObject{cObj}, limit); len(problems) != 0 {
				t.Errorf("%s: cross-book card %q: %s", sizeOpt.ID, cb, strings.Join(problems, "; "))
			}
			cObj.Resize(fyne.NewSize(limit, 200))
			walkTree(cObj, func(o fyne.CanvasObject) {
				if pr, ok := o.(*crossRefPrinted); ok {
					rr := test.WidgetRenderer(pr).(*crossRefPrintedRenderer)
					rr.Layout(pr.Size())
					for _, row := range rr.rows {
						if rw := rr.measure(row.Text); rw > limit+0.5 {
							t.Errorf("%s: cross-book row %q wants %.1f points, over %.0f", sizeOpt.ID, row.Text, rw, limit)
						}
					}
					switch cb {
					case "1 Thessalonians 5:28–2 Thessalonians 1:2":
						if got := strings.Join(rr.rowTexts(), " | "); got != "1 Thessalonians 5:28– | 2 Thessalonians 1:2" {
							t.Errorf("%s: %q wrapped as %q; want break after en dash", sizeOpt.ID, cb, got)
						}
					case "Leviticus 27:34–Numbers 1:1":
						if len(rr.rows) != 1 {
							t.Errorf("%s: %q wrapped into %d rows on 283pt; want 1", sizeOpt.ID, cb, len(rr.rows))
						}
					case "Jeremiah 52:1–Lamentations 1:5":
						wantRows := 1
						if sizeOpt.ID == "xl" {
							wantRows = 2
						}
						if len(rr.rows) != wantRows {
							t.Errorf("%s: %q wrapped into %d rows; want %d", sizeOpt.ID, cb, len(rr.rows), wantRows)
						}
					}
				}
			})
			sObj := crossRefSelfRow(cb, "", lightPalette, false)
			if problems := crossRefListMisfits([]fyne.CanvasObject{sObj}, limit); len(problems) != 0 {
				t.Errorf("%s: cross-book self-row %q: %s", sizeOpt.ID, cb, strings.Join(problems, "; "))
			}
		}
		wideDesktop := newCrossRefRef("1 Thessalonians 5:28–2 Thessalonians 1:2", sz.reference, fyne.TextStyle{Bold: true}, lightPalette.Accent)
		wideDesktop.Resize(fyne.NewSize(420, 100))
		rr := test.WidgetRenderer(wideDesktop).(*crossRefPrintedRenderer)
		rr.Layout(wideDesktop.Size())
		if len(rr.rows) != 1 {
			t.Errorf("%s: cross-book reference wrapped into %d rows on 420pt wide panel; want 1", sizeOpt.ID, len(rr.rows))
		}

		// Hyphenated and em-dashed compounds in the passage stay whole when they
		// fit a row of their own, and turn cleanly after the dash when wider than
		// the narrowest phone at Extra large.
		para := newReadingParagraph([]cardRun{{Text: "Opening words unrighteousness—indignation and a short-term phrase."}},
			readingGlyphSize(sz.passage), lightPalette.Text, lightPalette.RedLetter, styledPaneFont(), cardItalicFont())
		para.holdFlush = true
		para.Resize(fyne.NewSize(limit, 10))
		for _, o := range test.WidgetRenderer(para).(*readingParagraphRenderer).objs {
			if edge := o.Position().X + o.Size().Width; edge > limit+0.5 {
				t.Errorf("%s: compound passage row reaches %.1f, over %.0f", sizeOpt.ID, edge, limit)
			}
		}

		if text, w := widest(labels, sz.reference, fyne.TextStyle{Bold: true}); w > limit {
			t.Errorf("%s: the card reference %q wants %.1f points at %v bold, over %.0f", sizeOpt.ID, text, w, sz.reference, limit)
		}

		probeGroup := newCrossRefGroupRow(empty, publisherGroup{
			Text:   "(Rom. 5:8)",
			Pieces: []publisherPiece{{Target: &publisherTarget{Ref: crossRef{Book: "Romans", Chapter: 5, Verse: 8}}}},
		}, "", false, lightPalette, func(crossRef) {}, &crossRefRelayout{}, false)
		probeGroup.toggle()
		noteLimit := limit - 2*crossRefSectionPadX - sz.letterMargin
		groupPad := probeGroup.inset.Objects[0].MinSize().Width - probeGroup.members.MinSize().Width
		groupLimit := noteLimit - groupPad
		if sizeOpt.ID == "normal" && (noteLimit != 249 || groupLimit != 233) {
			t.Fatalf("control: Normal publisher card width is %.0f on a note and %.0f in a group; want 249 and 233", noteLimit, groupLimit)
		}

		for _, c := range []struct {
			label, tag string
			inGroup    bool
		}{
			{"Song of Solomon 2:8–3:5", "", true},
			{"2 Thessalonians 2:16–17", "", true},
			{"1 Thessalonians 5:12–13", "", true},
			{"1 Thessalonians 4:14–17", "", true},
			{"Leviticus 11", "", true},
			{"2 Kings 18–20", "", true},
			{"Psalm 150", "Title", true},
			{"Psalm 119", "Title", true},
			{"Deuteronomy 32:43", "LXX, DSS", true},
			{"2 Thessalonians 2:16–17", "LXX", true},
		} {
			card := crossRefRowRuled(empty, crossRefCard{Label: c.label, Tag: c.tag}, lightPalette, func(crossRef) {}, false)
			w := card.MinSize().Width
			desc := c.label
			if c.tag != "" {
				desc += " [" + c.tag + "]"
			}
			if w > noteLimit {
				t.Errorf("%s: publisher card heading %q wants %.1f points, over the note's %.0f", sizeOpt.ID, desc, w, noteLimit)
			}
			if c.inGroup {
				if w > groupLimit {
					t.Errorf("%s: publisher group card heading %q wants %.1f points, over the group's %.0f", sizeOpt.ID, desc, w, groupLimit)
				}
				card.Resize(fyne.NewSize(groupLimit, 200))
				walkTree(card, func(o fyne.CanvasObject) {
					if tx, ok := o.(*canvas.Text); ok && tx.Text != "" {
						if edge := tx.Position().X + tx.Size().Width; edge > groupLimit+0.5 {
							t.Errorf("%s: %q text %q reaches %.1f inside %.0f group well", sizeOpt.ID, desc, tx.Text, edge, groupLimit)
						}
						if sizeOpt.ID == "normal" && c.tag != "" && tx.Text == c.tag && tx.Position().X == 0 {
							t.Errorf("Normal: %q wrapped tag onto a second line inside %.0f group well", desc, groupLimit)
						}
					}
				})
			}
		}
	}

	setReadingTextSizeID("normal")
	runs := []string{parallelsLabel, treasuryLabel, publisherRunLabel("NKJV")}
	if text, w := widest(runs, 11, fyne.TextStyle{Bold: true, Monospace: true}); w > limit {
		t.Errorf("the section label %q wants %.0f points, over %.0f", text, w, limit)
	}
	// CONTROL: the measure can exceed the limit, and an oversized tagged
	// reference wraps its tag onto line 2 inside a group well instead of
	// overflowing groupLimit.
	if _, w := widest([]string{strings.Repeat("Thessalonians ", 4)}, 16, fyne.TextStyle{Bold: true}); w <= limit {
		t.Fatalf("control: a 56-character reference measures %.0f, within %.0f", w, limit)
	}
	wideCard := crossRefRowRuled(empty, crossRefCard{Label: "Song of Solomon 2:8–3:5", Tag: "LXX, DSS, Title"}, lightPalette, func(crossRef) {}, false)
	wideCard.Resize(fyne.NewSize(233, 200))
	wrappedTag := false
	walkTree(wideCard, func(o fyne.CanvasObject) {
		if tx, ok := o.(*canvas.Text); ok && tx.Text == "LXX, DSS, Title" && tx.Position().Y > 0 {
			wrappedTag = true
		}
	})
	if !wrappedTag || wideCard.MinSize().Width > 233 {
		t.Fatalf("control: oversized tag wrapped=%v MinSize.Width=%.1f; want wrapped inside 233", wrappedTag, wideCard.MinSize().Width)
	}
}

// THE PANEL SCALES PROPORTIONALLY WITH THE READER'S TEXT SIZE. Normal leaves
// every baseline metric untouched; Large and Extra large grow the passage,
// previews, references, margin letters, printed shorthand, inline retry, tags,
// pericope titles and inline list captions in strict hierarchy while keeping
// pinned footer chrome steady and double-letter marks clear of their margin.
func TestCrossRefPanelScalesWithReaderTextSize(t *testing.T) {
	th := appThemeForTest(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	withCrossRefIndex(t, "From Verse\tTo Verse\tVotes\nMatt.3.1\tJohn.3.2\t10\n")

	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }

	bd := publisherFixture()
	bd.Verses["Matthew"][3][0].Footnotes = []Footnote{
		{Anchor: 14, Text: "John 7:50; (Gal. 6:15)", Kind: footnoteKindCrossref, Caller: "-", Refs: []NoteRef{{"JHN.7.50", 0, 9}, {"GAL.6.15", 12, 21}}},
	}
	bd.Verses["Mark"][1] = []Verse{
		{BookName: "Mark", Book: "Mark", Chapter: 1, Verse: 1, Text: "Synthetic parallel opening."},
		{BookName: "Mark", Book: "Mark", Chapter: 1, Verse: 6, Text: "Synthetic parallel closing."},
	}
	bd.PrepareSearchIndex()

	var prev crossRefSizes
	for i, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		sz := crossRefPanelSizes()
		if !(sz.caption < sz.title && sz.title < sz.printed && sz.printed < sz.reference && sz.reference == sz.letter && sz.reference < sz.preview && sz.preview == sz.passage) {
			t.Fatalf("%s: broken hierarchy: %+v", sizeOpt.ID, sz)
		}
		if i == 0 {
			want := crossRefSizes{
				passage: 17, preview: 17, reference: 16, letter: 16,
				doubleLetter: 10, letterMargin: 18, printed: 14, title: 12, caption: 11,
			}
			if sz != want {
				t.Fatalf("Normal sizes = %+v, want %+v", sz, want)
			}
		} else {
			if !(sz.passage > prev.passage && sz.preview > prev.preview && sz.reference > prev.reference &&
				sz.printed > prev.printed && sz.title > prev.title && sz.caption > prev.caption &&
				sz.doubleLetter > prev.doubleLetter && sz.letterMargin > prev.letterMargin) {
				t.Fatalf("%s did not strictly grow every role over previous: got %+v after %+v", sizeOpt.ID, sz, prev)
			}
		}
		dbl := crossRefLetter("ww", lightPalette)
		wSize, _ := fyne.CurrentApp().Driver().RenderedTextSize(dbl.Text, dbl.TextSize, dbl.TextStyle, dbl.FontSource)
		if wSize.Width >= sz.letterMargin-1 {
			t.Errorf("%s: double letter %q renders %.2fpt wide inside %.1fpt margin", sizeOpt.ID, dbl.Text, wSize.Width, sz.letterMargin)
		}

		w := test.NewTempWindow(t, canvas.NewRectangle(lightPalette.Background))
		w.Resize(fyne.NewSize(375, 667))
		st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 3, CurrentVersion: "nkjv", window: w, theme: th}
		showCrossRefs(st, "", selSpan{lo: 1, hi: 1})
		top := w.Canvas().Overlays().Top().(*widget.PopUp)

		var (
			passage   *readingParagraph
			preview   *widget.Label
			reference *canvas.Text
			noteRow   *crossRefNoteRow
			sources   *widget.RichText
		)
		walkTree(top, func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *readingParagraph:
				if passage == nil {
					passage = x
				}
			case *widget.Label:
				if preview == nil && strings.Contains(x.Text, "Synthetic") {
					preview = x
				}
			case *canvas.Text:
				if reference == nil && x.Text == "John 7:50" {
					reference = x
				}
			case *crossRefNoteRow:
				if noteRow == nil {
					noteRow = x
				}
			case *widget.RichText:
				if sources == nil && strings.Contains(segmentText(x.Segments), "OpenBible.info") {
					sources = x
				}
			}
		})
		if passage == nil || preview == nil || reference == nil || noteRow == nil || sources == nil {
			t.Fatalf("%s: missing mounted widget (passage=%v preview=%v ref=%v note=%v sources=%v)",
				sizeOpt.ID, passage != nil, preview != nil, reference != nil, noteRow != nil, sources != nil)
		}
		if got, want := passage.size, readingGlyphSize(sz.passage); got != want {
			t.Errorf("%s: passage glyph size = %v, want %v", sizeOpt.ID, got, want)
		}
		if got := preview.Theme().Size(theme.SizeNameText); got != sz.preview {
			t.Errorf("%s: preview theme text size = %v, want %v", sizeOpt.ID, got, sz.preview)
		}
		if got := preview.Theme().Size(theme.SizeNameCaptionText); got != sz.caption {
			t.Errorf("%s: list scope caption size = %v, want %v", sizeOpt.ID, got, sz.caption)
		}
		if reference.TextSize != sz.reference || noteRow.letter.TextSize != sz.letter {
			t.Errorf("%s: reference=%v letter=%v, want %v and %v", sizeOpt.ID, reference.TextSize, noteRow.letter.TextSize, sz.reference, sz.letter)
		}
		rows := test.WidgetRenderer(noteRow.label).(*crossRefPrintedRenderer).rows
		if len(rows) == 0 || rows[0].TextSize != sz.printed {
			t.Errorf("%s: printed row size = %v, want %v", sizeOpt.ID, rows, sz.printed)
		}
		// Pinned footer legal/source credits remain steady 11pt panel chrome.
		if got := sources.Theme().Size(theme.SizeNameCaptionText); got != 11 {
			t.Errorf("%s: footer credit size = %v, want steady 11pt chrome", sizeOpt.ID, got)
		}
		top.Hide()
		prev = sz
	}
}
