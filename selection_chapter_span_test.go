package bibletext

// The span's chapter (selSpan.book, selSpan.chapter) as every consumer reads
// it, and as the native panes' exports build it (paneSelSpan). The fixture,
// an invented Psalm 135 and 136, is selection_chapter_test.go's.

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// Every consumer of the span reads the selection in the chapter the span
// names, after the reader's chapter has moved on. Control: the same span
// without a chapter is read in the reader's chapter, as every span was.
func TestAVerbReadsTheSelectionInTheChapterItWasMadeIn(t *testing.T) {
	st := turningChapterState()
	span := paneSelSpan(3, 3, "3 ", true, "Psalms", 135)
	before := aiPassageFor(st, turnWords, span)
	if before.Ref != turnRef {
		t.Fatalf("control: before the turn the selection is cited %q, want %s", before.Ref, turnRef)
	}
	if !advanceToNextChapter(st) || st.CurrentChapter != 136 {
		t.Fatalf("control: the chapter did not turn (%s %d)", st.CurrentBook, st.CurrentChapter)
	}
	version := st.currentVersion()

	if p := aiPassageFor(st, turnWords, span); p.Ref != turnRef || p.Book != "Psalms" || p.Chapter != 135 || p.Lo != 3 || p.Hi != 3 || p.PreviewText != turnWords {
		t.Errorf("the study passage is %+v, want %s quoting its own words", p, turnRef)
	}
	if got := citationForSelection(st, turnWords, span); got != turnRef {
		t.Errorf("citation = %q, want %s", got, turnRef)
	}
	if quote, cite, _, _ := prepareShareQuote(st, turnWords, span); cite != turnRef || quote != turnWords {
		t.Errorf("share quote = %q cited %q, want %q cited %s", quote, cite, turnWords, turnRef)
	}
	if lo, hi := linkVersesForSelection(st, turnWords, span); lo != 3 || hi != 3 {
		t.Errorf("link verses = %d–%d, want 3–3", lo, hi)
	}
	if vs := selectionVerses(st, turnWords, span); len(vs) != 1 || vs[0].Chapter != 135 || vs[0].Verse != 3 {
		t.Errorf("selection verses = %+v, want 135:3 alone", vs)
	}
	if text, lo, hi, _, ok := normalizeShareSelection(st, turnWords, span); !ok || text != turnWords || lo != 3 || hi != 3 {
		t.Errorf("normalize = %q %d–%d ok=%v, want the verse's words at 3", text, lo, hi, ok)
	}
	if p := selectionPassage(st, span); p.book != "Psalms" || p.chapter != 135 || p.versionID != version.ID {
		t.Errorf("the note composer's passage = %+v, want Psalms 135 in %s", p, version.ID)
	}
	if got := shareNoteReference(st, turnWords, span); got != turnRef {
		t.Errorf("the note composer's reference = %q, want %s", got, turnRef)
	}

	shared := captureShares(t)
	shareVerse(st, turnWords, false, span)
	shareVerseLink(st, turnWords, span)
	wantLink := ShareLinkURL(version.ID, "Psalms", 135, 3, 3)
	if len(*shared) != 2 {
		t.Fatalf("shared %q, want two messages", *shared)
	}
	if s := (*shared)[0]; !strings.Contains(s, "builder of the stone towers") || !strings.Contains(s, "— "+turnRef+" (") {
		t.Errorf("Share with citation sent %q, want the verse's words cited %s", s, turnRef)
	}
	if s := (*shared)[1]; !strings.HasPrefix(s, turnRef+" (") || !strings.HasSuffix(s, "\n"+wantLink) {
		t.Errorf("Share as link sent %q, want %s and %s", s, turnRef, wantLink)
	}

	// Control: a span that names no chapter is read in the reader's, so the
	// same verse number cites the chapter narration turned to.
	bare := selSpanWithLead(3, 3, "3 ")
	if got := aiPassageFor(st, turnWords, bare).Ref; got != turnNextRef {
		t.Errorf("control: a span with no chapter is cited %q, want the reader's %s", got, turnNextRef)
	}
}

// The span a native pane dispatches carries the chapter its text holds, and
// nothing but a book and a chapter number names one.
func TestThePaneSpanCarriesItsChapter(t *testing.T) {
	st := turningChapterState()
	st.CurrentChapter = 136
	for _, c := range []struct {
		name          string
		span          selSpan
		book          string
		chapter       int
		lo, hi        int
		hasLead       bool
		lead          string
		wantSelection string
	}{
		{"lead and chapter", paneSelSpan(3, 3, "3 ", true, "Psalms", 135), "Psalms", 135, 3, 3, true, "3 ", "Psalms 135"},
		{"no lead", paneSelSpan(2, 4, "", false, "Psalms", 135), "Psalms", 135, 2, 4, false, "", "Psalms 135"},
		{"no span, still its chapter", paneSelSpan(0, 0, "", false, "Psalms", 135), "Psalms", 135, 0, 0, false, "", "Psalms 135"},
		{"clamped, the lead dropped", paneSelSpan(0, 2, "x", true, "Psalms", 135), "Psalms", 135, 1, 2, false, "", "Psalms 135"},
		{"no book", paneSelSpan(3, 3, "3 ", true, "", 135), "", 0, 3, 3, true, "3 ", "Psalms 136"},
		{"no chapter", paneSelSpan(3, 3, "3 ", true, "Psalms", 0), "", 0, 3, 3, true, "3 ", "Psalms 136"},
	} {
		s := c.span
		if s.book != c.book || s.chapter != c.chapter || s.lo != c.lo || s.hi != c.hi || s.hasLead != c.hasLead || s.lead != c.lead {
			t.Errorf("%s: span = %+v", c.name, s)
		}
		if b, ch := selectionChapter(st, s); b+" "+strconv.Itoa(ch) != c.wantSelection {
			t.Errorf("%s: read in %s %d, want %s", c.name, b, ch, c.wantSelection)
		}
	}
	// The first chapter named is kept: a later stand-in does not replace the
	// pane's own.
	if s := paneSelSpan(3, 3, "", false, "Psalms", 135).inChapter("Psalms", 136); s.chapter != 135 {
		t.Errorf("inChapter replaced the pane's chapter: %+v", s)
	}
	if b, ch := versesChapter([]Verse{{Verse: 1}, {BookName: "Psalms", Chapter: 135, Verse: 2}}); b != "Psalms" || ch != 135 {
		t.Errorf("versesChapter = %s %d, want Psalms 135", b, ch)
	}
	if b, ch := versesChapter(nil); b != "" || ch != 0 {
		t.Errorf("versesChapter(nil) = %s %d, want nothing", b, ch)
	}
}

// turnedMenu is a desktop pane's selection menu, built over Psalm 135:3's
// words, after continuous narration has turned the reader to Psalm 136 under
// it: the chapter's audio ended, the controller moved the reader on and
// refreshed the reading view (audioController.advanceAndContinue), and the
// menu, an overlay, is still up.
// The native panes hand Go the chapter their text holds with every
// selection (paneSelSpan, held to every pane by the contract tests in
// selection_chapter_contract_test.go); dispatched after narration turned the
// chapter, the study verb and the share read the selection in it.
func TestANativeSelectionDispatchedAfterTheTurnKeepsItsChapter(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(900, 760))
	st := turningChapterState()
	st.window = w
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	grantAIPermission(st.aiKeys, defaultProviderID) // a reader who has allowed it (ai_permission.go)
	muteEngines(t)
	gAudio.stop()
	t.Cleanup(gAudio.stop)
	gAudio.advanceAndContinue(st, audioTTS, "", chapterAudioFingerprint(st))
	if st.CurrentChapter != 136 {
		t.Fatalf("control: narration did not turn the chapter (%d)", st.CurrentChapter)
	}
	// As bibleTextAIMenuTapped, bibleTextStudyMenuTapped and
	// btaSelectionAction build it from what the pane sent.
	span := paneSelSpan(3, 3, "3 ", true, "Psalms", 135)

	passages := make(chan aiPassage, 1)
	done := make(chan context.Context, 1)
	prevRun, prevFit := aiActionRun, studyRemeasure
	studyRemeasure = func(fit func()) { fit() }
	aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, _ selSpan) (string, error) {
		p, _ := aiPassageFromContext(ctx)
		passages <- p
		done <- ctx
		return "an answer", nil
	}
	t.Cleanup(func() { aiActionRun, studyRemeasure = prevRun, prevFit })
	dispatchAIAction(st, aiActionExplain, turnWords, span)
	select {
	case p := <-passages:
		if p.Ref != turnRef {
			t.Errorf("the request names %q, want %s", p.Ref, turnRef)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Explain never reached the study request")
	}
	<-(<-done).Done()
	studyOverlay(t, w).Hide()

	shared := captureShares(t)
	dispatchSelectionAction(st, selActionShareCite, turnWords, span)
	if len(*shared) != 1 || !strings.Contains((*shared)[0], "— "+turnRef+" (") {
		t.Errorf("shared %q, want it cited %s", *shared, turnRef)
	}
}

// Every chapter with a next one, over the local translation caches: verse 3
// of each (or the last, in a shorter chapter) selected whole, the span as a
// pane sends it, then the chapter turned as narration turns it. Every
// verb's reading of the selection is the same after the turn as before it.
// Before the span carried its chapter, 1,186 of the 1,188 chapter pairs in
// the WEB, and as many in the BSB, cited the next chapter's verse after the
// turn, and none stayed right. Control: the same span without its chapter,
// which the sweep must see read differently after the turn.
func TestEverySelectionKeepsItsChapterThroughATurnOverLocalCaches_LocalOnly(t *testing.T) {
	if os.Getenv("BIBLETEXT_EXHAUSTIVE") == "" {
		t.Skip("local-only and opt-in: every chapter pair over three translation caches; run with BIBLETEXT_EXHAUSTIVE=1")
	}
	for _, c := range localPublicDomainCaches(t) {
		pairs, wrong, bareMoved := 0, 0, 0
		for _, book := range c.bd.Books {
			for _, ch := range c.bd.GetChapterNumbersForBook(book) {
				verses := c.bd.GetChapter(book, ch)
				if len(verses) == 0 {
					continue
				}
				v := verses[min(2, len(verses)-1)]
				text := collapseSpaces(verseSharedText(v))
				if text == "" {
					continue
				}
				span := paneSelSpan(v.Verse, v.Verse, strconv.Itoa(v.Verse)+" ", true, book, ch)
				bare := paneSelSpan(v.Verse, v.Verse, strconv.Itoa(v.Verse)+" ", true, "", 0)
				st := &AppState{Bible: c.bd, CurrentVersion: c.id, CurrentBook: book, CurrentChapter: ch}
				read := func(span selSpan) string {
					p := aiPassageFor(st, text, span)
					lo, hi := linkVersesForSelection(st, text, span)
					vs := selectionVerses(st, text, span)
					_, cite, _, _ := prepareShareQuote(st, text, span)
					return strings.Join([]string{p.Ref, p.PreviewText, cite, citationForSelection(st, text, span),
						strconv.Itoa(lo), strconv.Itoa(hi), strconv.Itoa(len(vs))}, "|")
				}
				before, bareBefore := read(span), read(bare)
				if !advanceToNextChapter(st) {
					continue
				}
				pairs++
				if after := read(span); after != before {
					wrong++
					if wrong <= 5 {
						t.Errorf("%s %s %d:%d: read %q after the turn, %q before", c.id, book, ch, v.Verse, after, before)
					}
				}
				if read(bare) != bareBefore {
					bareMoved++
				}
			}
		}
		t.Logf("%s: %d chapter pairs, %d read differently after the turn; without the chapter, %d", c.id, pairs, wrong, bareMoved)
		if wrong != 0 {
			t.Errorf("%s: %d of %d selections were read in another chapter after the turn", c.id, wrong, pairs)
		}
		if bareMoved < pairs-2 {
			t.Errorf("control: without the chapter only %d of %d selections were read differently after the turn; the sweep cannot see the defect", bareMoved, pairs)
		}
	}
}
