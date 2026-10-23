package bibletext

// No study verb for a selection that holds no word of Scripture: a word of a
// publisher's heading or the heading whole, a psalm's title, a verse number
// selected on its own. The share pipeline cites each by a verse and quotes
// nothing (decision 23, A), and a study verb on one has no Scripture to
// read: sent as the passage of the verse below, a heading's words went to
// the provider as Scripture that verse does not hold. No menu offers the
// verbs for such a selection, and the dispatch sends nothing for one; a
// share of it is the citation line alone. Nor for a selection whose only
// Scripture is punctuation: a verse's closing full stop or another mark of
// its words selected alone, or a drag from a verse's closing punctuation
// into the heading below it that ends in the heading. The share quotes the
// mark alone, as it did, and the prompt sent it alone as the passage. A
// selection that takes a word of a verse, a drag from a heading into its
// verse among them and one from the closing punctuation on into the verse
// below, is studied as before.
//
// These tests hold Go's half: the desktop panes' menu (selectionStudyMenu,
// shared by the Windows and Linux pane and the legacy pane), the dispatch
// every pane reaches (dispatchAIAction), and the native panes' rule
// modelled on a page drawn as they draw it (paneHoldsScripture).
// no_scripture_study_contract_test.go holds the native panes' code.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// nb is the no-break space every pane joins a verse's number to its words
// with.
const nb = string(noBreakSpace)

// paneHoldsScripture is the native panes' rule (btIOSSelectionHoldsScripture,
// btMacSelectionHoldsScripture, BtBridge.selectionHoldsScripture) on a
// drawnHeadingPage: whether page[a:b] holds a letter or figure that stands
// in a verse's words, after its number and the space joined to it and
// before its end. A heading, a verse's number, the breaks between verses
// and the verses' punctuation hold none. A selection that begins in a
// heading, with no letter or figure of a verse's words
// before the first character of the heading's words it holds, does not
// count a word it ends inside, which reaches back to the white space or the
// closed-up dash before it, as Go's end repair cuts back: Go quotes nothing
// for a drag from a heading that ends inside the first word of the verse
// below, "Solomon’" of "Solomon’s" or "Well-" of "Well-being" among them,
// or takes only the punctuation that verse opens with. A selection begun
// on punctuation in the verse above follows the same boundary rule.
func paneHoldsScripture(page string, pvs []headingPageVerse, a, b int) bool {
	h := b         // the first character of a heading's words; b for none
	above := false // a letter or figure of a verse's words stands before h
	for k := 0; k+1 < len(pvs) && h == b; k++ {
		above = above || pageVerseHolds(page, pvs[k], a, b, isWordRune)
		if from, to := max(pvs[k].end, a), min(pvs[k+1].num, b); from < to {
			if i := strings.IndexFunc(page[from:to], func(r rune) bool { return !unicode.IsSpace(r) }); i >= 0 {
				h = from + i
			}
		}
	}
	if next, _ := utf8.DecodeRuneInString(page[min(b, len(page)):]); h < b && !above && isWordRune(next) {
		for b > h {
			r, size := utf8.DecodeLastRuneInString(page[:b])
			if unicode.IsSpace(r) || r == '—' {
				break
			}
			b -= size
		}
	}
	for _, p := range pvs {
		if pageVerseHolds(page, p, a, b, isWordRune) {
			return true
		}
	}
	return false
}

// pageVerseHolds reports whether the words of verse p of a
// drawnHeadingPage, within page[a:b], hold a rune that in reports.
func pageVerseHolds(page string, p headingPageVerse, a, b int, in func(rune) bool) bool {
	for i := max(p.body, a); i < min(p.end, b); {
		r, size := utf8.DecodeRuneInString(page[i:])
		if in(r) {
			return true
		}
		i += size
	}
	return false
}

// studyMenuItems is what the desktop selection menu built for the selection
// offers: Study with AI, Share and Cross-references, and the order of the
// three it holds.
func studyMenuItems(st *AppState, sel string, span selSpan) (study, share, xref bool, order string) {
	var labels []string
	for _, it := range selectionStudyMenu(st, sel, span, func() {}, func() {}).Items {
		switch it.Label {
		case "Study with AI":
			study = true
		case "Share":
			share = true
		case "Cross-references":
			xref = true
		default:
			continue
		}
		labels = append(labels, it.Label)
	}
	return study, share, xref, strings.Join(labels, ", ")
}

// studySelection is one selection: what the pane hands over, with its span,
// and the state that draws it.
type studySelection struct {
	name, text string
	span       selSpan
	st         *AppState
}

// studySelections are, with no Scripture: a heading's words one at a time
// and whole, with the span the native panes send for them
// (paneHeadingSpan); a verse number selected on its own and the rest of one
// begun inside it, with the span and the lead the panes send there; a
// psalm's title, alone and run on to verse 1's number, with no span, as a
// page that cannot place it hands it over; verse 1's number alone, which is
// what the panes' cut leaves of a drag from the title into the psalm; and,
// with only punctuation for Scripture, a verse's closing full stop and a
// comma selected alone, and drags from the closing full stop above the
// heading that end in the heading or on the number below, with the panes'
// span and lead. The controls are words of the verses, a drag from the
// heading into the verse below and one from the full stop above it on into
// that verse, with the panes' span and lead.
func studySelections(t *testing.T) (none, controls []studySelection) {
	t.Helper()
	const heading = "The Line of the Wanderer"
	f := headingFixture("Genesis", 4, map[int]string{
		16: "So the Wanderer went out and settled east of the garden.",
		17: "The Wanderer knew his wife, and she bore a son, and he built a city.",
	}, Heading{Text: heading, BeforeVerse: 17})
	at := strings.Index(f.page, heading)
	if at < 0 {
		t.Fatalf("fixture: the heading is not on the page:\n%s", f.page)
	}
	for _, w := range strings.Fields(heading) {
		i := at + strings.Index(f.page[at:], w)
		none = append(none, studySelection{"the heading's " + w, w, paneHeadingSpan(f.page, f.pvs, i, i+len(w)), f.st})
	}
	none = append(none,
		studySelection{"the heading whole", heading, paneHeadingSpan(f.page, f.pvs, at, at+len(heading)), f.st},
		studySelection{"verse 17's number alone", "17", selSpanWithLead(17, 17, ""), f.st},
		studySelection{"the 7 of verse 17's number", "7", selSpanWithLead(17, 17, "1"), f.st},
	)
	for _, w := range []string{"Wanderer", "garden."} {
		v := f.pvs[0]
		i := v.body + strings.Index(f.page[v.body:v.end], w)
		controls = append(controls, studySelection{"verse 16's " + w, w, paneHeadingSpan(f.page, f.pvs, i, i+len(w)), f.st})
	}
	v := f.pvs[1]
	controls = append(controls,
		studySelection{"verse 17's first word", "The", paneHeadingSpan(f.page, f.pvs, v.body, v.body+len("The")), f.st},
		studySelection{"verse 17's number and first word", "17" + nb + "The", selSpanWithLead(17, 17, ""), f.st},
		studySelection{"the heading run on into verse 17", heading + "\n17" + nb + "The Wanderer knew",
			paneHeadingSpan(f.page, f.pvs, at, v.body+len("The Wanderer knew")), f.st},
	)
	stop := f.pvs[0].end - len(".")
	comma := v.body + strings.Index(f.page[v.body:v.end], ",")
	for _, d := range []struct {
		name string
		a, b int
	}{
		{"verse 16's closing full stop", stop, stop + len(".")},
		{"a comma of verse 17", comma, comma + len(",")},
		{"a comma of verse 17 and the space after it", comma, comma + len(", ")},
		{"verse 16's full stop into the heading's first word", stop, at + len("The")},
		{"verse 16's full stop into the heading, to its end", stop, at + len(heading)},
		{"verse 16's full stop through verse 17's number", stop, v.body},
		{"verse 16's full stop into an unfinished word below the heading", stop, v.body + len("Th")},
	} {
		none = append(none, studySelection{d.name, f.page[d.a:d.b], paneHeadingSpan(f.page, f.pvs, d.a, d.b), f.st})
	}
	controls = append(controls, studySelection{"verse 16's full stop on into verse 17's first word",
		f.page[stop : v.body+len("The")], paneHeadingSpan(f.page, f.pvs, stop, v.body+len("The")), f.st})

	psalm := psalmTitleState(false)
	title := collapseSpaces(psalm.Bible.SuperscriptionFor("Psalms", 3).DrawnText())
	none = append(none,
		studySelection{"the psalm's title", title, selSpan{}, psalm},
		studySelection{"the psalm's title run on to verse 1's number", title + "\n1", selSpan{}, psalm},
		studySelection{"verse 1's number, all a drag from the title leaves", "1", selSpanWithLead(1, 1, ""), psalm},
	)
	controls = append(controls, studySelection{"verse 1's first words", "1" + nb + "Yahweh, how my", selSpanWithLead(1, 1, ""), psalm})
	return none, controls
}

// The desktop panes' menu offers no Study with AI for a selection that holds
// no word of Scripture, and keeps Share and Cross-references, in the order
// they hold with the assistant on. The controls, words of the verses, are
// offered it. With the assistant set to None no selection is offered it, and
// Cross-references leads, as before.
func TestNoStudyVerbIsOfferedForASelectionThatHoldsNoScripture(t *testing.T) {
	none, controls := studySelections(t)
	for _, s := range none {
		study, share, xref, order := studyMenuItems(s.st, s.text, s.span)
		if study {
			t.Errorf("%s (%q, span %+v): the menu offers Study with AI", s.name, s.text, s.span)
		}
		if !share || !xref || order != "Share, Cross-references" {
			t.Errorf("%s: the menu holds %q, want Share, Cross-references", s.name, order)
		}
	}
	for _, s := range controls {
		if study, _, _, order := studyMenuItems(s.st, s.text, s.span); !study || order != "Study with AI, Share, Cross-references" {
			t.Errorf("control: %s (%q): the menu holds %q, want Study with AI, Share, Cross-references", s.name, s.text, order)
		}
	}

	off := controls[0].st
	off.aiKeys = newKeyStoreWith(newFakePrefs())
	off.aiKeys.setAIEnabled(false)
	for _, s := range []studySelection{none[0], controls[0]} {
		if study, _, _, order := studyMenuItems(off, s.text, s.span); study || order != "Cross-references, Share" {
			t.Errorf("assistant None, %s: the menu holds %q, want Cross-references, Share", s.name, order)
		}
	}
}

// A study verb dispatched for such a selection, as a menu built for another
// selection or a pane's rule that let one through would deliver it, opens
// nothing and sends nothing, whichever verb, Ask among them: the dispatch
// is the backstop behind every pane's menu. The key is set and the provider
// allowed, so a dispatch that went on would reach the request. The controls
// open the answer panel and reach the request.
func TestNoStudyVerbIsSentForASelectionThatHoldsNoScripture(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctxs := stubAIActionParked(t)

	none, controls := studySelections(t)
	win := app.NewWindow("reading")
	defer win.Close()
	for _, s := range append(none, controls...) {
		s.st.window = win
		if s.st.aiKeys == nil {
			s.st.aiKeys = newKeyStoreWith(newFakePrefs())
			s.st.aiKeys.setAPIKey(defaultProviderID, "test-key")
			grantAIPermission(s.st.aiKeys, defaultProviderID)
		}
	}
	for _, s := range none {
		for _, v := range aiStudyVerbs {
			dispatchAIAction(s.st, v.Action, s.text, s.span)
			if top := win.Canvas().Overlays().Top(); top != nil {
				t.Errorf("%s, %s: the dispatch opened %T; texts %v", s.name, v.Label, top, treeTexts(top))
				win.Canvas().Overlays().Remove(top)
			}
			select {
			case <-ctxs:
				t.Errorf("%s, %s: a study request reached the provider seam", s.name, v.Label)
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	for _, s := range controls {
		dispatchAIAction(s.st, aiActionExplain, s.text, s.span)
		select {
		case <-ctxs:
		case <-time.After(2 * time.Second):
			t.Errorf("control: %s: Explain sent no request", s.name)
		}
		if top := win.Canvas().Overlays().Top(); top == nil {
			t.Errorf("control: %s: Explain opened no panel", s.name)
		} else {
			win.Canvas().Overlays().Remove(top)
		}
	}
}

// On the Windows and Linux pane, where a heading and a psalm's title are
// drawn outside the text a reader selects, a verse number is what can be
// selected alone: a double-click on verse 2's number selects it, and the
// pane's menu offers no Study with AI for it. The control is a double-click
// on a word of the verse.
func TestTheStyledPaneOffersNoStudyVerbForAVerseNumberAlone(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	p := newTestPane(t, psalm23NoteState(), 420)

	num := -1
	for _, ln := range p.lay.Lines {
		for _, r := range ln.Runs {
			if r.Kind == runVerseNum && r.Verse == 2 && num < 0 {
				num = r.Offset
			}
		}
	}
	if num < 0 {
		t.Fatal("fixture: the pane laid out no number for verse 2")
	}
	p.DoubleTapped(&fyne.PointEvent{Position: posForOffset(p, num)})
	if got := plainSelection(p.selectedRaw()); got != "2" {
		t.Fatalf("fixture: a double-click on verse 2's number selected %q, want \"2\"", got)
	}
	if hasMenuItem(p.studyMenu(), "Study with AI") {
		t.Errorf("verse 2's number alone, span %+v: the pane's menu offers Study with AI", p.selectedVerseSpan())
	}
	if !hasMenuItem(p.studyMenu(), "Share") || !hasMenuItem(p.studyMenu(), "Cross-references") {
		t.Error("verse 2's number alone: the pane's menu lost Share or Cross-references")
	}
	p.clearSelection()

	off := len([]rune(p.lay.Text[:strings.Index(p.lay.Text, "green")]))
	p.DoubleTapped(&fyne.PointEvent{Position: posForOffset(p, off+2)})
	if got := p.selectedRaw(); got != "green" {
		t.Fatalf("control: a double-click on a verse's word selected %q, want \"green\"", got)
	}
	if !hasMenuItem(p.studyMenu(), "Study with AI") {
		t.Error("control: \"green\" in verse 2: the pane's menu offers no Study with AI")
	}
}

func hasMenuItem(m *fyne.Menu, label string) bool {
	for _, it := range m.Items {
		if it.Label == label {
			return true
		}
	}
	return false
}

// A verse number selected on its own where the verse's words hold the same
// figure, as the BSB's Genesis 5 gives the ages in figures: the pane says
// the selection begins on the verse's number (its lead is empty), and it is
// that number and nothing else, so it is the number, not the "3" of "130".
// It quotes nothing and cites the verse, as every other number selected on
// its own does, and the menu offers no study verb for it. Searched for among
// the verse's words, the figure was found inside "130" and quoted, and the
// study verbs were offered for it. The controls: the figure selected inside
// "130", with the lead the panes send there, and the number run on into the
// verse's first word.
func TestAVerseNumberAloneIsTheNumberWhereTheVerseHoldsTheFigure(t *testing.T) {
	st := leadChapter("Genesis", 5, map[int]string{
		3: "When Adam was 130 years old, he had a son in his own likeness.",
		4: "And after he had become the father of Seth, Adam lived 800 years.",
		5: "So Adam lived a total of 930 years, and then he died.",
	})
	for _, v := range []int{3, 4, 5} {
		num := strconv.Itoa(v)
		quote, cite, _, _ := prepareShareQuote(st, num, selSpanWithLead(v, v, ""))
		if want := verseRangeCitation("Genesis", 5, v, v); quote != "" || cite != want {
			t.Errorf("verse %d's number alone: shared (%q, %q), want (\"\", %q)", v, quote, cite, want)
		}
		if study, _, _, _ := studyMenuItems(st, num, selSpanWithLead(v, v, "")); study {
			t.Errorf("verse %d's number alone: the menu offers Study with AI", v)
		}
	}
	if quote, cite, _, _ := prepareShareQuote(st, "130", selSpanWithLead(3, 3, "3"+nb+"When Adam was ")); quote != "130" || cite != "Genesis 5:3" {
		t.Errorf("control: \"130\" in verse 3 shared (%q, %q), want (\"130\", Genesis 5:3)", quote, cite)
	}
	if study, _, _, _ := studyMenuItems(st, "3"+nb+"When", selSpanWithLead(3, 3, "")); !study {
		t.Error("control: verse 3's number and first word: the menu offers no Study with AI")
	}
}

// The model of the native panes' rule on the fixture page: each heading
// word, the heading whole and verse 17's number hold no Scripture, nor does
// a drag from the heading that ends inside verse 17's first word, a mark of
// a verse's words alone, or a drag from verse 16's closing full stop that
// ends in the heading or on verse 17's number; each word of the verses
// holds some, and so do a drag from the heading through verse 17's first
// word and one from verse 16's full stop into it, which are studied as
// before.
func TestThePanesRuleHoldsNoScriptureWhereGoQuotesNothing(t *testing.T) {
	const heading = "The Line of the Wanderer"
	f := headingFixture("Genesis", 4, map[int]string{
		16: "So the Wanderer went out and settled east of the garden.",
		17: "The Wanderer knew his wife, and she bore a son, and he built a city.",
	}, Heading{Text: heading, BeforeVerse: 17})
	at := strings.Index(f.page, heading)
	v16, v17 := f.pvs[0], f.pvs[1]
	stop := v16.end - len(".")
	comma := v17.body + strings.Index(f.page[v17.body:v17.end], ",")
	for _, c := range []struct {
		name  string
		a, b  int
		holds bool
	}{
		{"verse 16's closing full stop", stop, v16.end, false},
		{"a comma of verse 17", comma, comma + len(","), false},
		{"a comma of verse 17 and the space after it", comma, comma + len(", "), false},
		{"verse 16's full stop into the heading", stop, at + len(heading), false},
		{"verse 16's full stop through verse 17's number", stop, v17.body, false},
		{"verse 16's full stop into the T of verse 17's first word", stop, v17.body + len("T"), false},
		{"verse 16's full stop and verse 17's first word", stop, v17.body + len("The"), true},
		{"the heading whole", at, at + len(heading), false},
		{"the heading with the breaks on either side", at - 1, at + len(heading) + 1, false},
		{"the heading's \"Line\"", at + 4, at + 8, false},
		{"verse 17's number", v17.num, v17.body - len(nb), false},
		{"the heading and verse 17's number", at, v17.body, false},
		{"the heading into the T of verse 17's first word", at, v17.body + len("T"), false},
		{"verse 16's last word", v16.end - len("garden."), v16.end, true},
		{"verse 16's last word and the heading", v16.end - len("garden."), at + len(heading), true},
		{"the heading and verse 17's first word", at, v17.body + len("The"), true},
		{"verse 17's number and first word", v17.num, v17.body + len("The"), true},
		{"the T of verse 17's first word", v17.body, v17.body + len("T"), true},
	} {
		if got := paneHoldsScripture(f.page, f.pvs, c.a, c.b); got != c.holds {
			t.Errorf("%s (%q): the panes' rule says %v, want %v", c.name, f.page[c.a:c.b], got, c.holds)
		}
	}
}

// besideHeadingFixture is a chapter whose verses below its headings open
// with the shapes a drag from a heading can end inside: a word with an
// apostrophe in it, a hyphened word, a word closed up to the next by a dash,
// an ellipsis and a space before the first word, and an opening quotation
// mark.
func besideHeadingFixture() headingFixtureChapter {
	return headingFixture("Genesis", 4, map[int]string{
		16: "So the Wanderer went out and settled east of the garden.",
		17: "Solomon’s son was Rehoboam, and he reigned.",
		18: "Well-being came to the city; then it ended.",
		19: "Then—the people rejoiced and sang.",
		20: "… For this reason I kneel before the Father.",
		21: "“Sing to the Lord,” they said.",
	},
		Heading{Text: "The Line of the Wanderer", BeforeVerse: 17},
		Heading{Text: "The Well", BeforeVerse: 18},
		Heading{Text: "A Saying", BeforeVerse: 19},
		Heading{Text: "The Prayer", BeforeVerse: 20},
		Heading{Text: "A Song", BeforeVerse: 21},
	)
}

// headingMatter is where the headings between verse k and verse k+1 of a
// drawnHeadingPage stand, from the first character of their words to the
// last; ok is false when none stands there.
func headingMatter(page string, pvs []headingPageVerse, k int) (hs0, hs1 int, ok bool) {
	if k+1 >= len(pvs) {
		return 0, 0, false
	}
	matter := page[pvs[k].end:pvs[k+1].num]
	i := strings.IndexFunc(matter, func(r rune) bool { return !unicode.IsSpace(r) })
	if i < 0 {
		return 0, 0, false
	}
	return pvs[k].end + i, pvs[k].end + len(strings.TrimRightFunc(matter, unicode.IsSpace)), true
}

// wordsEnd is the page offset where the first n runs of characters that
// are not white space after from end, and to when fewer end before it.
func wordsEnd(page string, from, to, n int) int {
	in := false
	for i, r := range page[from:to] {
		switch space := unicode.IsSpace(r); {
		case !space:
			in = true
		case in:
			in = false
			if n--; n == 0 {
				return from + i
			}
		}
	}
	return to
}

// firstWordAt is where the first word of the verse words page[body:end]
// starts and ends, its first run of letters and figures, and whether white
// space stands between that word and the punctuation the verse opens with.
// ok is false for verse words with no letter or figure.
func firstWordAt(page string, body, end int) (w0, w1 int, spaced, ok bool) {
	words := page[body:end]
	i := strings.IndexFunc(words, isWordRune)
	if i < 0 {
		return 0, 0, false, false
	}
	n := strings.IndexFunc(words[i:], func(r rune) bool { return !isWordRune(r) })
	if n < 0 {
		n = len(words) - i
	}
	return body + i, body + i + n, strings.IndexFunc(words[:i], unicode.IsSpace) >= 0, true
}

// Beside a heading the panes' rule answers as Go does. A drag from the
// heading that ends anywhere in the first two words of the verse below, and
// a drag from the closing punctuation of the verse above into the heading
// and on into the verse below, are offered the study verbs by the panes'
// rule exactly when the desktop menu offers them: no pane shows an item the
// dispatch then drops, and none hides one Go would study. From the heading,
// a word the drag ends inside reaches back to the white space or the
// closed-up dash before it, as Go's end repair cuts back, so "Solomon’" and
// "Well-" are the heading's alone and "Then—t" takes "Then". From the
// closing punctuation above, a drag that ends in the heading, on the number
// below or inside its first word holds no complete word and is offered
// none. A complete word below, or one before the heading, still offers the
// verbs. The same boundary rule applies when punctuation and white space
// open the verse below.
func TestThePanesRuleAgreesWithGoBesideAHeading(t *testing.T) {
	f := besideHeadingFixture()
	offered := func(a, b int) bool {
		study, _, _, _ := studyMenuItems(f.st, f.page[a:b], paneHeadingSpan(f.page, f.pvs, a, b))
		return study
	}
	want := map[string]bool{
		"Solomon": true, "Solomon’": false, "Solomon’s": true,
		"Well": true, "Well-": false, "Well-b": false, "Well-being": true,
		"Then": true, "Then—": true, "Then—t": true, "Then—the": true,
		"… F": false, "… For": true,
		"“S": false, "“Sing": true,
	}
	// What a drag from the closing punctuation above the heading is offered,
	// by where it ends in the verse below.
	wantAbove := map[string]bool{
		"": false, "S": false, "Solomon’": false, "Well-": false,
		"…": false, "… ": false, "… F": false, "“": false, "“S": false,
	}
	seenAbove := map[string]bool{}
	seen := 0
	for k := range f.pvs {
		hs0, hs1, ok := headingMatter(f.page, f.pvs, k)
		if !ok {
			continue
		}
		above, below := f.pvs[k], f.pvs[k+1]
		_, w1, _, ok := firstWordAt(f.page, below.body, below.end)
		if !ok {
			t.Fatalf("fixture: verse %d has no word", below.verse)
		}
		for e := below.body; e <= wordsEnd(f.page, below.body, below.end, 2); {
			got, menu := paneHoldsScripture(f.page, f.pvs, hs0, e), offered(hs0, e)
			if got != menu {
				t.Errorf("%q: the panes' rule says %v and the menu %v", f.page[hs0:e], got, menu)
			}
			if w, ok := want[f.page[below.body:e]]; ok {
				seen++
				if got != w {
					t.Errorf("%q: the panes' rule says %v, want %v", f.page[hs0:e], got, w)
				}
			}
			if e == below.end {
				break
			}
			_, size := utf8.DecodeRuneInString(f.page[e:])
			e += size
		}
		r, size := utf8.DecodeLastRuneInString(f.page[above.body:above.end])
		if isWordRune(r) {
			t.Fatalf("fixture: verse %d does not end on punctuation", above.verse)
		}
		from := above.end - size
		for _, e := range []int{wordsEnd(f.page, hs0, hs1, 1), hs1, below.body} {
			if got, menu := paneHoldsScripture(f.page, f.pvs, from, e), offered(from, e); got || menu {
				t.Errorf("%q, from verse %d's closing punctuation: the panes' rule says %v and the menu %v, want both false", f.page[from:e], above.verse, got, menu)
			}
		}
		for e := below.body; e <= wordsEnd(f.page, below.body, below.end, 2); {
			got, menu := paneHoldsScripture(f.page, f.pvs, from, e), offered(from, e)
			if got != menu {
				t.Errorf("%q, from verse %d's closing punctuation: the panes' rule says %v and the menu %v", f.page[from:e], above.verse, got, menu)
			}
			if w, ok := wantAbove[f.page[below.body:e]]; ok {
				seenAbove[f.page[below.body:e]] = true
				if got != w {
					t.Errorf("%q, from verse %d's closing punctuation: the panes' rule says %v, want %v", f.page[from:e], above.verse, got, w)
				}
			}
			if e == below.end {
				break
			}
			_, size := utf8.DecodeRuneInString(f.page[e:])
			e += size
		}
		if !offered(from, w1) {
			t.Errorf("control: %q, from verse %d's closing punctuation through the first word below: the menu offers no study verb", f.page[from:w1], above.verse)
		}
	}
	if seen != len(want) {
		t.Errorf("fixture: %d of the %d expected end points were tried", seen, len(want))
	}
	if len(seenAbove) != len(wantAbove) {
		t.Errorf("fixture: %d of the %d expected end points from the punctuation above were tried", len(seenAbove), len(wantAbove))
	}
}

// A piece of a verse's number is the number, as the number whole is: the
// first digit or digits of a number of two digits or more, with the empty
// lead a selection begun on the number has; a digit inside a number, begun
// inside it as the lead says; and a number begun in the space before it,
// after a lead that holds the whole verse above, as the Apple and Android
// panes send a selection begun past a verse's end. Each quotes nothing and
// cites the verse the number labels, and the menu offers no study verb for
// it. The first digit went out as the selection, a figure alone, quoted as
// Scripture and offered the study verbs, and the number begun after a
// verse's end was found among the next verse's words where they hold the
// same figure, the "3" of "130". The controls: the number whole, and the
// number run on into the verse's first word.
func TestAPieceOfAVerseNumberIsTheNumber(t *testing.T) {
	st := leadChapter("Genesis", 5, map[int]string{
		2:  "He created them male and female.",
		3:  "When Adam was 130 years old, he had a son.",
		15: "Then the father lived some years.",
		16: "Seth lived many years, and he had a son.",
		17: "And he lived 105 more years.",
	})
	psalm := leadChapter("Psalms", 119, map[int]string{
		122: "Ensure your servant's well-being.",
		123: "My eyes fail, looking for your salvation.",
		124: "Deal with your servant by your loving devotion.",
	})
	corpus, spans := chapterProseIn(st, "Genesis", 5)
	page, pvs := drawnHeadingPage(corpus, spans, nil)
	v2, v3 := pvs[0], pvs[1]
	if v2.verse != 2 || v3.verse != 3 {
		t.Fatalf("fixture: the page opens with verses %d and %d", v2.verse, v3.verse)
	}
	for _, c := range []struct {
		name, sel string
		st        *AppState
		span      selSpan
		cite      string
	}{
		{"verse 16's first digit, the words holding no 1", "1", st, selSpanWithLead(16, 16, ""), "Genesis 5:16"},
		{"verse 17's first digit, the words holding 105", "1", st, selSpanWithLead(17, 17, ""), "Genesis 5:17"},
		{"verse 123's first two digits", "12", psalm, selSpanWithLead(123, 123, ""), "Psalms 119:123"},
		{"verse 123's middle digit, begun inside it", "2", psalm, selSpanWithLead(123, 123, "1"), "Psalms 119:123"},
		{"verse 3's number begun in the space after verse 2", page[v2.end : v3.body-len(nb)], st,
			headingPageSpan(page, pvs, v2.end, v3.body-len(nb)), "Genesis 5:3"},
	} {
		if quote, cite, _, _ := prepareShareQuote(c.st, c.sel, c.span); quote != "" || cite != c.cite {
			t.Errorf("%s (%q, span %+v): shared (%q, %q), want (\"\", %q)", c.name, c.sel, c.span, quote, cite, c.cite)
		}
		if study, _, _, _ := studyMenuItems(c.st, c.sel, c.span); study {
			t.Errorf("%s (%q): the menu offers Study with AI", c.name, c.sel)
		}
	}
	if quote, cite, _, _ := prepareShareQuote(st, "16", selSpanWithLead(16, 16, "")); quote != "" || cite != "Genesis 5:16" {
		t.Errorf("control: verse 16's number whole shared (%q, %q), want (\"\", Genesis 5:16)", quote, cite)
	}
	if study, _, _, _ := studyMenuItems(st, "16"+nb+"Seth", selSpanWithLead(16, 16, "")); !study {
		t.Error("control: verse 16's number and first word: the menu offers no Study with AI")
	}
}

// A drag from a heading that ends inside the number of the verse below,
// before its last digit, holds no word of Scripture: it is answered as the
// heading is, the verse below cited and nothing quoted, the menu offers no
// study verb for it, and the panes' rule holds none. Its digits are no
// verse number to confirm the heading by, and the heading's words were
// quoted as the verse's: "The Fourth Day 1" as Genesis 1:13–14. "Day 2"
// ends the headings above verses 20 and 24 alike, and the span names which.
// The controls: the heading run on through the whole number, which is
// answered so already, and on into the verse's first word, which is
// studied.
func TestAHeadingRunOnIntoPartOfTheNextNumberQuotesNothing(t *testing.T) {
	const heading = "The Fourth Day"
	f := headingFixture("Genesis", 1, map[int]string{
		13: "And there was evening, and there was morning, the third day.",
		14: "And God said, Let there be lights in the expanse of the sky.",
		19: "And there was evening, and there was morning, the fourth day.",
		20: "And God said, Let the waters teem with living creatures.",
		23: "And there was evening, and there was morning, the fifth day.",
		24: "And God said, Let the earth bring forth living creatures.",
	},
		Heading{Text: heading, BeforeVerse: 14},
		Heading{Text: "The Fifth Day", BeforeVerse: 20},
		Heading{Text: "The Sixth Day", BeforeVerse: 24},
	)
	for k := range f.pvs {
		hs0, hs1, ok := headingMatter(f.page, f.pvs, k)
		if !ok {
			continue
		}
		below := f.pvs[k+1]
		_, digit := utf8.DecodeRuneInString(f.page[below.num:])
		want := verseRangeCitation("Genesis", 1, below.verse, below.verse)
		for _, a := range []int{hs0, hs1 - len("Day")} {
			b := below.num + digit
			sel, span := f.page[a:b], paneHeadingSpan(f.page, f.pvs, a, b)
			if quote, cite, _, _ := prepareShareQuote(f.st, sel, span); quote != "" || cite != want {
				t.Errorf("%q (span %+v): shared (%q, %q), want (\"\", %q)", sel, span, quote, cite, want)
			}
			if study, _, _, _ := studyMenuItems(f.st, sel, span); study {
				t.Errorf("%q into verse %d: the menu offers Study with AI", sel, below.verse)
			}
			if paneHoldsScripture(f.page, f.pvs, a, b) {
				t.Errorf("%q into verse %d: the panes' rule holds Scripture", sel, below.verse)
			}
		}
	}
	at := strings.Index(f.page, heading)
	v14 := f.pvs[1]
	whole := v14.body - len(nb)
	if quote, cite, _, _ := prepareShareQuote(f.st, f.page[at:whole], paneHeadingSpan(f.page, f.pvs, at, whole)); quote != "" || cite != "Genesis 1:14" {
		t.Errorf("control: the heading and verse 14's number shared (%q, %q), want (\"\", Genesis 1:14)", quote, cite)
	}
	word := v14.body + len("And")
	if study, _, _, _ := studyMenuItems(f.st, f.page[at:word], paneHeadingSpan(f.page, f.pvs, at, word)); !study {
		t.Error("control: the heading run on into verse 14's first word: the menu offers no Study with AI")
	}
}

// On the Windows and Linux pane, a drag over the first digit of verse 16's
// number alone gets no Study with AI, and shares the citation alone, as the
// number whole does. The digit went out as a figure quoted as Scripture,
// with the study verbs offered, whether or not the verse's words hold it.
func TestTheStyledPaneReadsAVerseNumbersFirstDigitAsTheNumber(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	for _, words := range []string{
		"Seth lived 105 years, and he had a son.",
		"Seth lived many years, and he had a son.",
	} {
		st := leadChapter("Genesis", 5, map[int]string{
			15: "Then the father lived some years and had a son.",
			16: words,
			17: "And Seth died.",
		})
		p := newTestPane(t, st, 420)
		num := -1
		for _, ln := range p.lay.Lines {
			for _, r := range ln.Runs {
				if r.Kind == runVerseNum && r.Verse == 16 && num < 0 {
					num = r.Offset
				}
			}
		}
		if num < 0 {
			t.Fatal("fixture: the pane laid out no number for verse 16")
		}
		p.selStart, p.selEnd = num, num+1
		raw, span := p.selectedRaw(), p.selectedVerseSpan()
		if got := plainSelection(raw); got != "1" {
			t.Fatalf("fixture: the selection is %q, want \"1\"", got)
		}
		if hasMenuItem(p.studyMenu(), "Study with AI") {
			t.Errorf("%q: verse 16's first digit (span %+v): the pane's menu offers Study with AI", words, span)
		}
		if quote, cite, _, _ := prepareShareQuote(st, plainSelection(raw), span); quote != "" || cite != "Genesis 5:16" {
			t.Errorf("%q: verse 16's first digit shared (%q, %q), want (\"\", Genesis 5:16)", words, quote, cite)
		}
		p.clearSelection()
	}
}

// headingDrag is one selection beside a heading for the sweep below, page[a:b],
// with what the menu and the panes' rule are to answer: offered, not offered,
// or either so long as the two agree (agree).
type headingDrag struct {
	name   string
	a, b   int
	expect int
}

const (
	dragNotOffered = iota
	dragOffered
	dragAgree
)

// Over the local WEB, BSB and WEB Catholic caches, opt-in by
// BIBLETEXT_EXHAUSTIVE=1 as the other sweeps over them are. Every verse
// number of every chapter, selected alone with the span and lead the native
// panes send, each first part of a number of two digits or more with the
// empty lead, a number begun in the white space after the verse above with
// a lead that holds that verse, and every psalm's title, alone and run on to
// its first verse's number with no span, are offered no study verb by the
// desktop menu and hold no Scripture by the panes' rule on the page; the
// pieces of numbers, and the drags into them below, also share nothing
// quoted. (The heading words are swept with the rest of their answers in
// heading_word_selection_test.go.) Beside each heading between two verses:
// the first word of the verse below, the last word of the verse above, a
// drag from the headings through that first word and one from that last
// word into the headings are offered the verbs by both; so are drags from
// the closing punctuation of the verse above through the first word below,
// and into its first character only when that is a complete word. From that
// punctuation into the headings, through their first word or to the number
// below, a drag holds punctuation alone and is offered none by either, nor
// is one that takes only the verse's opening mark. A drag from the headings
// that ends one letter into the first word below, or takes only the
// punctuation the verse opens with, or ends inside the number below before
// its last digit, from the headings' first or last word, is offered none by
// either, as Go quotes nothing for it; and a drag begun inside the last
// word above that runs into the headings, and one from the headings that
// ends at any other point of the first two words below, is answered the
// same by both. Punctuation and white space before an unfinished word do
// not make it complete, whether the drag starts in the heading or on the
// punctuation above it.
func TestNoStudyVerbForANumberOrATitleOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every verse number and psalm title of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		var numbers, pieces, titles, drags, dragQuiet, wrong int
		miss := func(format string, a ...any) {
			wrong++
			if wrong <= 8 {
				t.Logf(c.id+": "+format, a...)
			}
		}
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: c.bd, CurrentBook: book, CurrentChapter: ch}
				if title, first, ok := chapterTitle(st, book, ch); ok {
					for _, sel := range []string{title, title + "\n" + strconv.Itoa(first)} {
						titles++
						if study, _, _, _ := studyMenuItems(st, sel, selSpan{}); study {
							miss("%s %d, %q: offered the study verbs", book, ch, sel)
						}
					}
				}
				corpus, spans := chapterProseIn(st, book, ch)
				if len(spans) == 0 {
					continue
				}
				hs := headingsFor(c.bd, book, ch)
				page, pvs := drawnHeadingPage(corpus, spans, hs)
				// none is a selection that holds no Scripture: no study
				// verb from either, and, when share is set, nothing quoted.
				none := func(name string, a, b int, span selSpan, share bool) {
					sel := page[a:b]
					offered, _, _, _ := studyMenuItems(st, sel, span)
					held := paneHoldsScripture(page, pvs, a, b)
					quote := ""
					if share {
						quote, _, _, _ = prepareShareQuote(st, sel, span)
					}
					if offered || held || quote != "" {
						miss("%s %d, %s, %q: menu %v, panes' rule %v, quoted %q", book, ch, name, sel, offered, held, quote)
					}
				}
				for k, p := range pvs {
					numbers++
					none(fmt.Sprintf("verse %d's number alone", p.verse), p.num, p.body-len(nb), headingPageSpan(page, pvs, p.num, p.body-len(nb)), false)
					num := page[p.num : p.body-len(nb)]
					for i := range num {
						if i > 0 {
							pieces++
							none(fmt.Sprintf("the first part of verse %d's number", p.verse), p.num, p.num+i, selSpanWithLead(p.verse, p.verse, ""), true)
						}
					}
					if k > 0 && strings.TrimSpace(page[pvs[k-1].end:p.num]) == "" {
						pieces++
						none(fmt.Sprintf("verse %d's number begun after the verse above", p.verse), pvs[k-1].end, p.body-len(nb), headingPageSpan(page, pvs, pvs[k-1].end, p.body-len(nb)), true)
					}
					hs0, hs1, ok := headingMatter(page, pvs, k)
					if !ok {
						continue
					}
					// Headings stand between verse k and k+1, from hs0 to
					// hs1 on the page. The first word of the verse below is
					// its first run of letters and figures, after whatever
					// opens the verse before it (the BSB's Ephesians 3:14
					// opens with an ellipsis).
					below := pvs[k+1]
					w0, w1, _, ok := firstWordAt(page, below.body, below.end)
					last := strings.Fields(page[p.body:p.end])
					if !ok || len(last) == 0 {
						continue
					}
					lastWord := last[len(last)-1]
					lastAt := p.end - len(lastWord)
					headLast := hs1 - len(strings.Fields(page[hs0:hs1])[len(strings.Fields(page[hs0:hs1]))-1])
					for _, a := range []int{hs0, headLast} {
						for i := range page[below.num : below.body-len(nb)] {
							if i > 0 {
								pieces++
								none("the headings into the first part of the number below", a, below.num+i, paneHeadingSpan(page, pvs, a, below.num+i), true)
							}
						}
					}
					open := strings.TrimRightFunc(page[below.body:w0], unicode.IsSpace)
					ds := []headingDrag{
						{"the first word below", w0, w1, dragOffered},
						{"the last word above", lastAt, p.end, dragOffered},
						{"the last word above into the headings", lastAt, hs1, dragOffered},
					}
					if r, size := utf8.DecodeRuneInString(lastWord); isWordRune(r) && len(lastWord) > size {
						ds = append(ds, headingDrag{"begun inside the last word above, into the headings", lastAt + size, hs1, dragAgree})
					}
					if r, size := utf8.DecodeLastRuneInString(page[p.body:p.end]); !isWordRune(r) {
						for _, e := range []int{hs1, wordsEnd(page, hs0, hs1, 1), below.body} {
							ds = append(ds, headingDrag{"from the closing punctuation above, ending before the verse below", p.end - size, e, dragNotOffered})
						}
						into := dragNotOffered
						first, n := utf8.DecodeRuneInString(page[below.body:])
						next, _ := utf8.DecodeRuneInString(page[below.body+n:])
						if isWordRune(first) && !isWordRune(next) {
							into = dragOffered
						}
						ds = append(ds,
							headingDrag{"from the closing punctuation above into one character below", p.end - size, below.body + n, into},
							headingDrag{"from the closing punctuation above through the first word below", p.end - size, w1, dragOffered})
					}
					_, letter := utf8.DecodeRuneInString(page[w0:])
					for e := below.body; e <= wordsEnd(page, below.body, below.end, 2); {
						d := headingDrag{"the headings into the verse below", hs0, e, dragAgree}
						switch {
						case e > w0 && e < w1:
							d.expect = dragNotOffered
						case e == w0+letter && w1-w0 > letter, open != "" && e == below.body+len(open):
							d.expect = dragNotOffered
						case e == w1:
							d.expect = dragOffered
						}
						ds = append(ds, d)
						if e == below.end {
							break
						}
						_, size := utf8.DecodeRuneInString(page[e:])
						e += size
					}
					for _, d := range ds {
						drags++
						offered, _, _, _ := studyMenuItems(st, page[d.a:d.b], paneHeadingSpan(page, pvs, d.a, d.b))
						held := paneHoldsScripture(page, pvs, d.a, d.b)
						if !offered {
							dragQuiet++
						}
						if offered != held || d.expect == dragOffered && !offered || d.expect == dragNotOffered && offered {
							miss("%s %d, %s, %q: menu %v, panes' rule %v", book, ch, d.name, page[d.a:d.b], offered, held)
						}
					}
				}
			}
		}
		t.Logf("%s: %d verse numbers alone, %d pieces of numbers and %d title selections, none of which the rule offers a study verb; %d drags beside a heading, %d of them offered none by the menu; answered otherwise than the rule, or by the menu and the panes' rule differently: %d",
			c.id, numbers, pieces, titles, drags, dragQuiet, wrong)
		if wrong != 0 {
			t.Errorf("%s: %d selections were offered the study verbs otherwise than the rule says", c.id, wrong)
		}
	}
}

// Over the local WEB, BSB and WEB Catholic caches, opt-in by
// BIBLETEXT_EXHAUSTIVE=1 as the other sweeps over them are: a selection
// whose only Scripture is punctuation is offered no study verb, by the
// desktop menu or by the panes' rule modelled on the page. Every verse's
// closing mark selected alone, every other mark of a verse's words selected
// alone, each with the span and lead the native panes send, and, at every
// place between two verses that holds a heading, a drag from the closing
// punctuation of the verse above that ends at the end of the headings,
// which the share quotes as that mark alone. The control at each of those
// places: the same drag run on through the first word of the verse below
// is offered the verbs by both, and the study prompt's passage holds that
// word. On rel/1.2.23 at 91e1ec683 the menu offers the verbs for every one
// of the punctuation selections.
func TestNoStudyVerbForPunctuationAloneOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every mark of every verse of three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		var closing, marks, drags, controls, wrong int
		miss := func(format string, a ...any) {
			wrong++
			if wrong <= 8 {
				t.Logf(c.id+": "+format, a...)
			}
		}
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				st := &AppState{Bible: c.bd, CurrentVersion: c.id, CurrentBook: book, CurrentChapter: ch}
				corpus, spans := chapterProseIn(st, book, ch)
				if len(spans) == 0 {
					continue
				}
				page, pvs := drawnHeadingPage(corpus, spans, headingsFor(c.bd, book, ch))
				// none is a selection of punctuation alone: no study verb
				// from the menu or the panes' rule.
				none := func(what string, verse, a, b int) {
					offered, _, _, _ := studyMenuItems(st, page[a:b], paneHeadingSpan(page, pvs, a, b))
					if held := paneHoldsScripture(page, pvs, a, b); offered || held {
						miss("%s %d:%d, %s, %q: menu %v, panes' rule %v", book, ch, verse, what, page[a:b], offered, held)
					}
				}
				for k, p := range pvs {
					words := page[p.body:p.end]
					last, size := utf8.DecodeLastRuneInString(words)
					for i, r := range words {
						if isWordRune(r) || unicode.IsSpace(r) {
							continue
						}
						if at := p.body + i; at == p.end-size {
							closing++
							none("the closing mark alone", p.verse, at, p.end)
						} else {
							marks++
							none("a mark alone", p.verse, at, at+utf8.RuneLen(r))
						}
					}
					_, hs1, ok := headingMatter(page, pvs, k)
					if !ok || isWordRune(last) {
						continue
					}
					below := pvs[k+1]
					w0, w1, _, ok := firstWordAt(page, below.body, below.end)
					if !ok {
						continue
					}
					drags++
					none("from the closing mark into the headings", p.verse, p.end-size, hs1)
					controls++
					sel, span := page[p.end-size:w1], paneHeadingSpan(page, pvs, p.end-size, w1)
					offered, _, _, _ := studyMenuItems(st, sel, span)
					held := paneHoldsScripture(page, pvs, p.end-size, w1)
					passage := aiPassageFor(st, strings.TrimSpace(sharedText(sel)), span).Text
					if word := outboundText(page[w0:w1]); !offered || !held || !strings.Contains(passage, word) {
						miss("%s %d:%d, control: from the closing mark through %q below, %q: menu %v, panes' rule %v, the prompt quotes %q",
							book, ch, p.verse, word, sel, offered, held, passage)
					}
				}
			}
		}
		t.Logf("%s: %d closing marks and %d other marks of verses selected alone, and %d drags from a verse's closing mark into the headings below it, none of which the rule offers a study verb; %d controls run on through the first word below, offered the verbs and quoting it; answered otherwise: %d",
			c.id, closing, marks, drags, controls, wrong)
		if wrong != 0 {
			t.Errorf("%s: %d selections were answered otherwise than the rule says", c.id, wrong)
		}
	}
}
