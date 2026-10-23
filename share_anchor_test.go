package bibletext

// The verse-of-the-day card shares beside its Share icon (shareAnchor,
// share.go). The Mac and iPad panes anchor a share to the reading view,
// which the card hides: the Mac's picker then showed nothing and the share
// was lost, and the iPad's popover pointed at the middle of the screen. The
// card hands the icon's rectangle over for the length of its share, a
// selection share hands none, and the two Apple panes present beside the
// rectangle while the Android, Windows and Linux shares never read it.

import (
	"go/ast"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// anchorCardState is the card test's fixture: a reader on Matthew 5 with a
// verse of the day the rotation can show.
func anchorCardState(t *testing.T) (*AppState, fyne.Window) {
	t.Helper()
	st, win := smallPhone(t)
	votdSynchronousRemeasure(t)
	st.Bible = &BibleData{
		Books: []string{"Matthew", "John"},
		Verses: map[string]map[int][]Verse{
			"Matthew": {5: {{BookName: "Matthew", Chapter: 5, Verse: 3, Text: "Blessed are the poor in spirit."}}},
			"John": {3: {
				{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world, that he gave his only born Son."},
				{BookName: "John", Chapter: 3, Verse: 17, Text: "For God didn't send his Son into the world to judge the world."},
			}},
		},
	}
	st.CurrentBook, st.CurrentChapter = "Matthew", 5
	return st, win
}

// catchAnchoredShares replaces the platform's share with one that records
// the anchor each share is handed over with, as the Apple panes read it.
func catchAnchoredShares(t *testing.T) *[]struct {
	at shareAnchor
	ok bool
} {
	t.Helper()
	got := &[]struct {
		at shareAnchor
		ok bool
	}{}
	prev := shareTextOut
	shareTextOut = func(string) {
		at, ok := shareAnchorNow()
		*got = append(*got, struct {
			at shareAnchor
			ok bool
		}{at, ok})
	}
	t.Cleanup(func() { shareTextOut = prev })
	return got
}

// THE CARD'S SHARE IS HANDED OVER WITH ITS ICON'S RECTANGLE: the icon's
// position on the canvas and its size, inside the card, and not the origin.
// Mutation: the card calls sharePassageText bare — no anchor, the Mac's
// picker anchored to the hidden reading view again.
func TestTheCardSharesBesideItsShareIcon(t *testing.T) {
	st, win := anchorCardState(t)
	shares := catchAnchoredShares(t)
	showVerseOfDay(st)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())
	icon := findIconTapButton(p.Content, theme.MailSendIcon().Name())
	if icon == nil {
		t.Fatal("the card has no share control")
	}
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(icon)
	sz := icon.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("control: the icon has no size (%v)", sz)
	}
	if pos.X <= 0 || pos.Y <= 0 {
		t.Fatalf("control: the icon sits at the canvas origin (%v); the anchor could not be told from none", pos)
	}

	icon.Tapped(&fyne.PointEvent{})

	if len(*shares) != 1 {
		t.Fatalf("the tap handed over %d shares, want 1", len(*shares))
	}
	s := (*shares)[0]
	if !s.ok {
		t.Fatal("the card's share was handed over with no anchor")
	}
	want := shareAnchor{x: pos.X, y: pos.Y, w: sz.Width, h: sz.Height}
	if s.at != want {
		t.Errorf("the share's anchor is %+v; the icon occupies %+v", s.at, want)
	}
	cardPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(p.Content)
	cardSz := p.Content.Size()
	if s.at.x < cardPos.X || s.at.y < cardPos.Y ||
		s.at.x+s.at.w > cardPos.X+cardSz.Width || s.at.y+s.at.h > cardPos.Y+cardSz.Height {
		t.Errorf("the anchor %+v lies outside the card at %v %v", s.at, cardPos, cardSz)
	}
	if _, ok := shareAnchorNow(); ok {
		t.Error("the card's anchor outlived its share: the next selection share would present beside the icon")
	}
}

// A SHARE WITH NO CONTROL BEHIND IT HAS NO ANCHOR: the passage route called
// outside the card, as the selection verbs call their routes, hands over
// none, so the Apple panes keep the selection as the anchor; and a control
// that has not been laid out, an empty rectangle, counts as none rather than
// a point at the origin.
func TestASelectionShareHasNoAnchor(t *testing.T) {
	st, _ := anchorCardState(t)
	shares := catchAnchoredShares(t)
	d, ok := verseOfTheDay(st)
	if !ok {
		t.Fatal("fixture: no verse of the day")
	}

	sharePassageText(st, d.Book, d.Chapter, d.Lo, d.Hi)
	shareAnchoredAt(shareAnchor{}, func() { sharePassageText(st, d.Book, d.Chapter, d.Lo, d.Hi) })
	shareAnchoredAt(shareAnchor{x: 10, y: 10, w: 0, h: 22}, func() {
		sharePassageText(st, d.Book, d.Chapter, d.Lo, d.Hi)
	})
	// The control: a laid-out control's rectangle is an anchor.
	shareAnchoredAt(shareAnchor{x: 10, y: 10, w: 33, h: 22}, func() {
		sharePassageText(st, d.Book, d.Chapter, d.Lo, d.Hi)
	})

	if len(*shares) != 4 {
		t.Fatalf("%d shares were handed over, want 4", len(*shares))
	}
	for i, s := range (*shares)[:3] {
		if s.ok {
			t.Errorf("share %d was handed over with the anchor %+v, want none", i, s.at)
		}
	}
	if s := (*shares)[3]; !s.ok || s.at != (shareAnchor{x: 10, y: 10, w: 33, h: 22}) {
		t.Errorf("control: the anchored share was handed over with %+v, %v", s.at, s.ok)
	}
	if _, ok := shareAnchorNow(); ok {
		t.Error("an anchor was left behind")
	}
}

// callsSelector reports whether fn's body calls pkg.name.
func callsSelector(fn *ast.FuncDecl, pkg, name string) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == pkg {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// THE TWO APPLE PANES PRESENT AN ANCHORED SHARE BESIDE ITS ANCHOR, AND THE
// OTHER THREE SHARES NEVER READ ONE. Each platform's nativeShareText is read
// as source, since no host compiles the iOS pane and none presents a share
// under test: the Mac's and the iPad's read the anchor and hand it to the
// anchored presentation their preamble defines, with the four numbers in the
// canvas's order; Android's, Windows' and Linux's do neither, which is what
// keeps their verse-of-the-day share as it was. Mutation: a pane's
// nativeShareText calls bibleTextShareText alone — the card's anchor is read
// and dropped, and the Mac's share is lost again.
func TestTheApplePanesPresentTheCardShareAtItsAnchor(t *testing.T) {
	pkg := loadSharePackage(t, repoRoot(t))
	apple := map[string]bool{"reading_macos.go": true, "reading_ios.go": true}
	const defined = "void bibleTextShareTextAt(const char *text, double x, double y, double w, double h)"
	const definedWords = "void bibleTextShareTextAtWords(const char *text, int verse, int at, int len)"

	var files []string
	for _, d := range pkg.decls(t, "nativeShareText") {
		files = append(files, d.file)
		reads := callsFunc(d.fn, "shareAnchorNow")
		presentsAt := callsSelector(d.fn, "C", "bibleTextShareTextAt")
		presentsAtWords := callsSelector(d.fn, "C", "bibleTextShareTextAtWords")
		presentsPlain := callsSelector(d.fn, "C", "bibleTextShareText")
		switch {
		case apple[d.file] && !(reads && presentsAt && presentsAtWords && presentsPlain):
			t.Errorf("%s: nativeShareText reads the anchor %v, presents beside it %v, beside the words %v, at the selection %v; "+
				"want all four", d.file, reads, presentsAt, presentsAtWords, presentsPlain)
		case apple[d.file] && !strings.Contains(pkg.src[d.file], defined):
			t.Errorf("%s: the preamble does not define %q", d.file, defined)
		case apple[d.file] && !strings.Contains(pkg.src[d.file], definedWords):
			t.Errorf("%s: the preamble does not define %q", d.file, definedWords)
		case !apple[d.file] && (reads || presentsAt || presentsAtWords):
			t.Errorf("%s: nativeShareText reads the anchor %v, presents beside it %v; this share has no anchor",
				d.file, reads, presentsAt)
		}
	}
	sort.Strings(files)
	want := []string{"reading_android.go", "reading_ios.go", "reading_macos.go", "share_other.go", "share_windows.go"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Errorf("nativeShareText is defined in %v, want %v", files, want)
	}
}

// SHARE WITH NOTE IS HANDED OVER WITH THE SELECTED WORDS AS ITS ANCHOR. Its
// sheet goes out before the note it kept is put on the page, which re-imports
// the chapter with the note's card above the passage, so the Mac and iPad
// panes, anchored to the selection as it was, pointed the sheet's arrow into
// the new card. The anchor names the words by their place in the pane's own
// text, the verse the selection begins in, the lead's length in UTF-16 units
// and the selection's, which the panes find again in the re-imported text.
// A send with no note puts no card on the page and keeps the selection's
// anchor, and so does a selection that came with no lead. A selection the
// pane found inside a heading has no lead and no words in the verse it is
// cited by; it is anchored to that verse's number, which the panes find
// again as they find words: kept at the selection, the iPad's sheet hung
// from the top of the reading view once the page had moved to the note's
// card and the heading was scrolled away. Mutation: the composer shares
// bare — no anchor, the arrow back in the card.
func TestShareWithNoteIsAnchoredToTheSelectedWords(t *testing.T) {
	for _, c := range []struct {
		name, note string
		span       selSpan
		want       shareAnchor
	}{
		{"a note, with the lead", "fixture anchor note", selSpanWithLead(1, 1, "1 The Lord is my shepherd; "), shareAnchor{verse: 1, wordsAt: 27, wordsLen: 14}},
		{"no note", "", selSpanWithLead(1, 1, "1 The Lord is my shepherd; "), shareAnchor{}},
		{"a note, no lead", "fixture anchor note", selSpanFromNative(1, 1), shareAnchor{}},
		{"a note, a heading's word", "fixture anchor note", paneSelSpan(3, 3, "", false, "Psalms", 23).inHeading(true), shareAnchor{verse: 3}},
		{"no note, a heading's word", "", paneSelSpan(3, 3, "", false, "Psalms", 23).inHeading(true), shareAnchor{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := sentNoteState()
			win := noteSheetWindow(t, st, false, 900, 760)
			holdNoteSheetTimers(t)
			setNotesEnabled(true)
			shares := catchAnchoredShares(t)
			const words = "I shall not \U0001F600"
			promptShareNote(st, words, c.span)
			p := topPopup(t, win)
			field := composerField(p)
			if field == nil {
				t.Fatalf("control: the composer has no field; texts %v", sheetTexts(p))
			}
			field.SetText(c.note)
			test.Tap(findTreeButton(p.Content, "Share"))
			if len(*shares) != 1 {
				t.Fatalf("%d shares were handed over, want 1", len(*shares))
			}
			s := (*shares)[0]
			if s.at != c.want || s.ok != (c.want.verse > 0) {
				t.Errorf("the share was handed over with %+v (ok %v), want %+v", s.at, s.ok, c.want)
			}
			if _, ok := shareAnchorNow(); ok {
				t.Error("an anchor was left behind")
			}
		})
	}
}

// Share with note is anchored to the words only on the passage they were
// selected on. The anchor names a verse number and a place after it, and the
// Mac and iPad panes find them in the text they hold when the sheet is
// presented. Once the reader has left the passage, that text is another
// chapter's: narration turned the chapter under an open menu, or a link moved
// the reader while the composer was open, and the sheet pointed at the same
// verse number in the chapter now on screen, after waiting a second for a
// re-import that never comes, since the note is shown only on its own passage
// (showSentNote). Off the passage the share is handed over with no anchor, as
// a share with no note is, and the panes present it at the selection.
// Mutation: the composer anchors every send with a note to the words.
func TestShareWithNoteIsAnchoredToTheWordsOnlyOnTheirPassage(t *testing.T) {
	const lead = "1 The Lord is my shepherd; "
	words := shareAnchor{verse: 1, wordsAt: 27, wordsLen: 14}
	for _, c := range []struct {
		name string
		// turned moves the reader before the composer opens, as narration
		// turns the chapter under an open menu; moved, after it has opened,
		// as a link does.
		turned, moved bool
		want          shareAnchor
	}{
		{"on the passage", false, false, words},
		{"after narration turned the chapter", true, false, shareAnchor{}},
		{"after a link moved the reader", false, true, shareAnchor{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			st := sentNoteState()
			win := noteSheetWindow(t, st, false, 900, 760)
			holdNoteSheetTimers(t)
			setNotesEnabled(true)
			shares := catchAnchoredShares(t)
			span := paneSelSpan(1, 1, lead, true, "Psalms", 23)
			if c.turned && (!advanceToNextChapter(st) || st.CurrentChapter != 24) {
				t.Fatal("control: narration must have turned the reader to Psalm 24")
			}
			promptShareNote(st, "I shall not \U0001F600", span)
			p := topPopup(t, win)
			field := composerField(p)
			if field == nil {
				t.Fatalf("control: the composer has no field; texts %v", sheetTexts(p))
			}
			if c.moved && (!moveChapter(st, 1) || st.CurrentChapter != 24) {
				t.Fatal("control: the link must have moved the reader to Psalm 24")
			}
			field.SetText("fixture anchor note")
			test.Tap(findTreeButton(p.Content, "Share"))
			if len(*shares) != 1 {
				t.Fatalf("%d shares were handed over, want 1", len(*shares))
			}
			s := (*shares)[0]
			if s.at != c.want || s.ok != (c.want.verse > 0) {
				t.Errorf("the reader on %s %d; the share was handed over with %+v (ok %v), want %+v",
					st.CurrentBook, st.CurrentChapter, s.at, s.ok, c.want)
			}
		})
	}
}
