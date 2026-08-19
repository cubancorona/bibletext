package bibletext

// THE GALLERY: every note state the styled pane can draw, rendered to a real
// software canvas and snapshotted (owner, 19 Aug: "verify all the permutations
// visually — multiple notes, expanded, collapsed, highlights, spacing, etc").
//
// It is a TEST, not a screenshot dump: each case asserts the structural truths
// that make its picture meaningful — the card is inside the band, the band is
// clear of every line box, the wash lands on the note's own verses and nowhere
// else, the counts span is accented when it is a control and absent when it is
// not — and THEN writes the PNG. A case that renders something wrong fails
// here rather than waiting to be noticed in a picture.
//
// Snapshots: BIBLETEXT_PANE_SNAPSHOT_DIR=<dir> go test -run TestStyledNoteGallery

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	fyneTheme "fyne.io/fyne/v2/theme"
)

// galleryCase is one picture: a state, a pane width, and what must be true of
// the render before the snapshot is worth keeping.
type galleryCase struct {
	name string
	// build returns the state and the chapter to render.
	build func(t *testing.T) (*AppState, []Verse, string, int)
	w, h  float32
	// wantSticker: a card is drawn. wantPill: it is the collapsed marker.
	// wantCounts: the counts span is a live control (accented).
	wantSticker, wantPill, wantCounts bool
	// washVerses are the verses that must carry the note/search wash; every
	// other verse must be clean.
	washVerses []int
}

func galleryFixture(t *testing.T, verses []int, texts []string) (*AppState, []Verse, string, int) {
	t.Helper()
	st, _ := styledNoteFixture(t, verses, texts)
	return st, st.Bible.GetChapter("Ruth", 1), "Ruth", 1
}

// contextFixture is three paragraphs with the note anchored in the MIDDLE one,
// so a picture of it carries the paragraph above, the note, and the paragraph
// below — the owner's framing for judging the air on both sides.
func contextFixture(t *testing.T, note string, pill bool) (*AppState, []Verse, string, int) {
	t.Helper()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })

	// shouldBreakParagraph closes a paragraph once it passes 320 characters and
	// the previous verse ended on a terminal, so a verse over that length is a
	// paragraph of its own — which is what gives this picture one paragraph
	// above the note and one below.
	long := "Then she arose with her daughters in law that she might return from the " +
		"country of Moab, for she had heard in the country of Moab how the LORD had " +
		"visited His people in giving them bread, and she went out from the place " +
		"where she was, and her two daughters in law with her, and they went on the " +
		"way to return to the land of Judah. "
	vs := make([]Verse, 0, 3)
	for i := 1; i <= 3; i++ {
		vs = append(vs, Verse{BookName: "Ruth", Book: "Ruth", Chapter: 3, Verse: i, Text: long})
	}
	bd := &BibleData{Books: []string{"Ruth"}, Verses: map[string]map[int][]Verse{"Ruth": {3: vs}}}
	st := &AppState{Bible: bd, CurrentBook: "Ruth", CurrentChapter: 3, CurrentVersion: "web"}

	paras := groupVersesIntoParagraphs(vs)
	if len(paras) < 3 {
		t.Fatalf("fixture must make at least three paragraphs, made %d", len(paras))
	}
	anchor := paras[1][0].Verse // the middle paragraph's first verse
	if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
		Book: "Ruth", Chapter: 3, VerseLo: anchor, Text: note}); !ok {
		t.Fatal("seeding failed")
	}
	applyNoteForCurrentChapter(st)
	if pill {
		hideCurrentNote(st)
	}
	return st, vs, "Ruth", 3
}

func TestStyledNoteGallery(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	realTheme := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	dir := os.Getenv("BIBLETEXT_PANE_SNAPSHOT_DIR")

	cases := []galleryCase{
		{
			name: "01-single-note-expanded",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{2}, []string{
					"Read this at the hospital this morning and thought of you."})
			},
			w: 520, h: 420, wantSticker: true, washVerses: []int{2},
		},
		{
			name: "02-two-notes-counts",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{1, 2}, []string{
					"The first voice on this passage.", "A second voice on the same page."})
			},
			w: 520, h: 420, wantSticker: true, wantCounts: true, washVerses: []int{2},
		},
		{
			name: "03-three-notes-counts",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{1, 2, 3}, []string{
					"One.", "Two — the middle voice.", "Three, and the count says so."})
			},
			w: 520, h: 420, wantSticker: true, wantCounts: true, washVerses: []int{3},
		},
		{
			name: "04-minimized-pill",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				st, ch, b, c := galleryFixture(t, []int{2}, []string{"Collapsed to its marker."})
				hideCurrentNote(st)
				return st, ch, b, c
			},
			w: 520, h: 420, wantSticker: true, wantPill: true,
		},
		{
			name: "05-suppressed-by-search",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				st, ch, b, c := galleryFixture(t, []int{2}, []string{
					"Stood down while a search owns the page."})
				// A foreign mark on ANOTHER verse: the note stands down to the
				// pill and the search's own wash is what the reader sees.
				goToVerseRange(st, "Ruth", 1, 3, 3)
				return st, ch, b, c
			},
			w: 520, h: 420, wantSticker: true, wantPill: true, washVerses: []int{3},
		},
		{
			name: "06-note-on-first-verse",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{1}, []string{
					"Anchored at the very first verse — no line above to hang from."})
			},
			w: 520, h: 420, wantSticker: true, washVerses: []int{1},
		},
		{
			name: "07-long-note-at-the-cap",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{2}, []string{devLongNoteText()})
			},
			w: 520, h: 560, wantSticker: true, washVerses: []int{2},
		},
		{
			name: "08-narrow-pane",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{2}, []string{
					"A narrow column wraps the message and the band grows with it."})
			},
			w: 300, h: 460, wantSticker: true, washVerses: []int{2},
		},
		{
			name: "09-wide-pane-reporter",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return galleryFixture(t, []int{2}, []string{
					"A wide pane centres the measure; the card follows the column."})
			},
			w: 1100, h: 420, wantSticker: true, washVerses: []int{2},
		},
		{
			name: "10-multi-verse-range",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				setNotesEnabled(true)
				deleteAllNotes(appPrefs())
				t.Cleanup(func() { deleteAllNotes(appPrefs()) })
				if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived,
					VersionID: "web", Book: "Ruth", Chapter: 1, VerseLo: 2, VerseHi: 3,
					Text: "Two verses under one note — the wash covers both."}); !ok {
					t.Fatal("seeding failed")
				}
				st := bandFixtureState()
				applyNoteForCurrentChapter(st)
				return st, st.Bible.GetChapter("Ruth", 1), "Ruth", 1
			},
			w: 520, h: 460, wantSticker: true, washVerses: []int{2, 3},
		},
		{
			name: "11-poetry-chapter",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				setNotesEnabled(true)
				deleteAllNotes(appPrefs())
				t.Cleanup(func() { deleteAllNotes(appPrefs()) })
				st := psalm23State()
				if _, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived,
					VersionID: st.CurrentVersion, Book: "Psalms", Chapter: 23, VerseLo: 2,
					Text: "The poem lines must keep their own shape around the band."}); !ok {
					t.Fatal("seeding failed")
				}
				applyNoteForCurrentChapter(st)
				return st, st.Bible.GetChapter("Psalms", 23), "Psalms", 23
			},
			w: 520, h: 460, wantSticker: true, washVerses: []int{2},
		},
		{
			// THE CONTEXT SHOT (owner): a paragraph above, the note, and a
			// paragraph below, so the air on both sides can be judged against
			// the passage rather than in isolation.
			name: "13-context-expanded",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return contextFixture(t, "A note with a paragraph above it and a paragraph below.", false)
			},
			w: 560, h: 620, wantSticker: true, washVerses: []int{2},
		},
		{
			name: "14-context-pill",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				return contextFixture(t, "Collapsed, with the same paragraphs around it.", true)
			},
			w: 560, h: 620, wantSticker: true, wantPill: true,
		},
		{
			name: "12-no-note-control",
			build: func(t *testing.T) (*AppState, []Verse, string, int) {
				setNotesEnabled(true)
				deleteAllNotes(appPrefs())
				st := bandFixtureState()
				applyNoteForCurrentChapter(st)
				return st, st.Bible.GetChapter("Ruth", 1), "Ruth", 1
			},
			w: 520, h: 420,
		},
	}

	for _, variant := range []struct {
		name    string
		pal     palette
		variant fyne.ThemeVariant
	}{
		{"light", lightPalette, fyneTheme.VariantLight},
		{"dark", darkPalette, fyneTheme.VariantDark},
	} {
		for _, tc := range cases {
			t.Run(variant.name+"/"+tc.name, func(t *testing.T) {
				app.Settings().SetTheme(forcedVariant{Theme: realTheme, v: variant.variant})
				st, verses, book, chapter := tc.build(t)
				_, _ = book, chapter

				p := newStyledReadingPane(st, verses)
				p.pal = variant.pal
				w := test.NewWindow(container.NewStack(canvas.NewRectangle(variant.pal.Surface), p))
				defer w.Close()
				w.Resize(fyne.NewSize(tc.w, tc.h))
				p.Refresh()
				img := w.Canvas().Capture()

				// --- the structural truths behind the picture -----------------
				if got := p.noteGeom.present; got != tc.wantSticker {
					t.Fatalf("sticker present = %v, want %v", got, tc.wantSticker)
				}
				if tc.wantSticker {
					if got := p.noteGeom.pill; got != tc.wantPill {
						t.Errorf("pill = %v, want %v", got, tc.wantPill)
					}
					// The card must sit INSIDE the reserved band — the whole
					// point of the layout-engine reservation.
					//
					// card.H, not cardH: cardH is the card WITHOUT its tail
					// (measureStyledNote), so the old spelling here would have
					// passed a tail hanging clean out of the band's bottom into
					// the passage — the one thing the band exists to prevent.
					cardTop, cardBot := p.noteGeom.card.Y, p.noteGeom.card.Y+p.noteGeom.card.H
					bandTop, bandBot := p.lay.BandY, p.lay.BandY+p.lay.BandH
					if cardTop < bandTop-0.6 || cardBot > bandBot+0.6 {
						t.Errorf("the card (%.1f..%.1f) escapes its band (%.1f..%.1f)",
							cardTop, cardBot, bandTop, bandBot)
					}
					// And every distance in the picture is the SPEC's.
					assertNoteSpacing(t, p)
					// The counts span is a CONTROL only when there is a set to
					// walk; when it is one, it must be drawn in the accent.
					hasCounts := p.noteGeom.counts.W > 0
					if hasCounts != tc.wantCounts {
						t.Errorf("counts control = %v, want %v", hasCounts, tc.wantCounts)
					}
					if tc.wantCounts {
						if n := countColorNear(img, variant.pal.Accent,
							int(p.noteGeom.counts.X)-2, int(p.noteGeom.counts.Y)-2,
							int(p.noteGeom.counts.W)+4, int(p.noteGeom.counts.H)+4); n < 3 {
							t.Errorf("the counts span is not accented (%d px) — it must read as pressable", n)
						}
					}
				}
				// The wash lands on the named verses and NOWHERE else: the
				// lesson three platforms paid for, asserted per picture.
				assertWashExactly(t, p, tc.washVerses)

				if dir != "" {
					writePNG(t, filepath.Join(dir, fmt.Sprintf("gallery-%s-%s.png", variant.name, tc.name)), img)
				}
			})
		}
	}
}

// assertWashExactly holds the wash to exactly the verses named: every one of
// them carries a tinted run, no other verse does, and no tinted run reaches
// into the reserved band.
func assertWashExactly(t *testing.T, p *styledReadingPane, verses []int) {
	t.Helper()
	want := map[int]bool{}
	for _, v := range verses {
		want[v] = true
	}
	seen := map[int]bool{}
	// The wash as the RENDERER paints it: one span per tinted stretch of a
	// line, in the layout's own coordinates.
	for _, sp := range tintSpansForLayout(p.lay) {
		ln := p.lay.Lines[sp.Line]
		for _, run := range ln.Runs {
			if run.Tint == tintNone {
				continue
			}
			seen[run.Verse] = true
			if !want[run.Verse] {
				t.Errorf("verse %d is washed and should not be", run.Verse)
			}
		}
		// And no painted line may overlap the reserved band — the property the
		// band's ADVANCE geometry is supposed to guarantee.
		if p.lay.BandH > 0 && ln.Y < p.lay.BandY+p.lay.BandH-0.6 && ln.Y+ln.H > p.lay.BandY+0.6 {
			t.Errorf("a washed line reaches into the band (line %.1f..%.1f, band %.1f..%.1f)",
				ln.Y, ln.Y+ln.H, p.lay.BandY, p.lay.BandY+p.lay.BandH)
		}
	}
	for v := range want {
		if !seen[v] {
			t.Errorf("verse %d should be washed and is not", v)
		}
	}
}

// devLongNoteText is a note at the shared cap, for the tallest bubble there
// can be (the dev Links tab's devLongNote, which lives behind a build tag).
func devLongNoteText() string {
	s := "This is what a note looks like when somebody uses every character they are " +
		"given, which is worth seeing because the bubble reserves its own band in the " +
		"text and the band is measured from the height of this. Read it slowly and check " +
		"nothing below is covered up. "
	r := []rune(s)
	for len(r) < NoteMaxRunes {
		r = append(r, 'x')
	}
	return string(r[:NoteMaxRunes])
}

// assertNoteSpacing holds EVERY distance in the picture to the shared spec
// (noteMetrics, notes_bubble.go) — the assertion the owner's ask turns into
// ("measure the pill and note vertical spacing and margins visually … make sure
// it's the same there"). It runs on all twelve permutations × light/dark, in
// both the expanded and the pill state, so a spacing change on this pane cannot
// land without either matching the table or moving it.
//
// It deliberately asserts the SAME quantities the three natives are held to by
// name in notes_spacing_spec_test.go: gap above, gap below, card padding, who
// row, who→body gap, tail depth, pill height. This pane is the only one whose
// pixels can be measured on the dev machine, so it is where the numbers are
// checked as GEOMETRY rather than as source text.
func assertNoteSpacing(t *testing.T, p *styledReadingPane) {
	t.Helper()
	g := p.noteGeom
	if !g.present {
		return
	}
	const tol = 0.01
	eq := func(what string, got, want float32) {
		t.Helper()
		if got < want-tol || got > want+tol {
			t.Errorf("%s = %.2f, want %.2f (noteMetrics)", what, got, want)
		}
	}

	// --- the band: the same air above the drawn shape as below it -------------
	eq("gap above the card", g.card.Y-p.lay.BandY, noteMetrics().GapAbove)
	eq("gap below the drawn shape",
		(p.lay.BandY+p.lay.BandH)-(g.card.Y+g.card.H), noteMetrics().GapBelow)

	if g.pill {
		// The pill is a piece of CONTENT: its height is the spec's, never the
		// verb button's (which is what all four surfaces used to derive it from).
		eq("pill height", g.card.H, noteMetrics().PillH)
		eq("pill height (cardH)", g.cardH, noteMetrics().PillH)
		return
	}

	// --- the drawn shape: card + tail -----------------------------------------
	eq("tail depth", g.card.H-g.cardH, noteMetrics().TailDepth)

	// --- the card's internal rhythm: pad / who / gap / message / pad ----------
	if g.sender.W <= 0 {
		t.Fatal("precondition: an expanded card always draws a who line")
	}
	eq("card left padding", g.sender.X-g.card.X, noteMetrics().Pad)
	eq("card top padding (who row's top)", g.sender.Y-g.card.Y, noteMetrics().Pad)
	eq("who row height", g.sender.H, noteMetrics().WhoH(styledNoteWhoSz))

	if len(g.bodyLines) == 0 {
		t.Fatal("precondition: an expanded card always draws a message")
	}
	first, last := g.bodyLines[0], g.bodyLines[len(g.bodyLines)-1]
	eq("who row bottom → message top",
		first.Y-(g.sender.Y+g.sender.H), noteMetrics().WhoGap)
	eq("message left padding", first.X-g.card.X, noteMetrics().Pad)
	eq("card bottom padding",
		g.cardH-((last.Y+last.H)-g.card.Y), noteMetrics().Pad)
}
