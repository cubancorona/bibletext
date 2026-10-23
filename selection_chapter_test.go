package bibletext

// A selection is read in the chapter it was made in (selSpan.book and
// selSpan.chapter, selection_span.go), whatever the reader's chapter is by
// the time a verb runs. Continuous narration turns the chapter when one
// ends, and a desktop menu built over the old chapter stays open across the
// turn; the verses and the lead the span carries are the old chapter's, and
// read against the new one Psalm 135:3's words were cited as Psalms 136:3.
//
// The fixture is an invented pair of chapters with other words under the
// same verse numbers, so a verb that reads the selection in the wrong
// chapter cites a verse the words are not in.

import (
	"context"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// turnWords is the whole of the invented Psalm 135:3, the verse selected in
// every test here.
const turnWords = "Praise the builder of the stone towers, for his walls are strong."

// turnRef is the verse turnWords is, and turnNextRef the same verse number
// in the chapter narration turns to.
const (
	turnRef     = "Psalms 135:3"
	turnNextRef = "Psalms 136:3"
)

func turningChapterState() *AppState {
	v := func(ch, n int, text string) Verse {
		return Verse{BookName: "Psalms", Book: "Psalms", Chapter: ch, Verse: n, Text: text}
	}
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {
			135: {
				v(135, 1, "Sing to the keeper of the high gate, all you who stand watch."),
				v(135, 2, "Lift your lamps in the courts of the morning."),
				v(135, 3, turnWords),
				v(135, 4, "He gathers the rain into his barns."),
			},
			136: {
				v(136, 1, "Give thanks to the gardener of the valley, for his harvest is good."),
				v(136, 2, "Give thanks to the shepherd of the far hills."),
				v(136, 3, "Give thanks to the singer of the long night."),
				v(136, 4, "He opened the river at the edge of the plain."),
			},
		}},
	}
	return &AppState{Bible: bd, CurrentVersion: "web", CurrentBook: "Psalms", CurrentChapter: 135}
}

// captureShares routes every text share to the returned slice for the
// length of the test.
func captureShares(t *testing.T) *[]string {
	t.Helper()
	var out []string
	prev := shareTextOut
	shareTextOut = func(s string) { out = append(out, s) }
	t.Cleanup(func() { shareTextOut = prev })
	return &out
}

type turnedMenu struct {
	st   *AppState
	w    fyne.Window
	menu *fyne.Menu
	// paneSpan is the span the pane handed the menu, before the menu's own
	// stamp.
	paneSpan selSpan
}

func menuBuiltThenNarrationTurns(t *testing.T, pane string) turnedMenu {
	t.Helper()
	return menuBuiltOverThenNarrationTurns(t, pane, true)
}

// menuBuiltOverThenNarrationTurns builds pane's menu over the invented Psalm
// 135:3 and turns the chapter. With named false the pane lays out the
// chapter's verses with their book taken off, so it cannot name the chapter
// they are.
func menuBuiltOverThenNarrationTurns(t *testing.T, pane string, named bool) turnedMenu {
	t.Helper()
	w := test.NewWindow(widget.NewLabel("reading"))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(900, 760))
	st := turningChapterState()
	st.window = w
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.aiKeys.setAPIKey(defaultProviderID, "test-key")
	grantAIPermission(st.aiKeys, defaultProviderID) // a reader who has allowed it (ai_permission.go)
	verses := st.Bible.GetChapter("Psalms", 135)
	if !named {
		verses = append([]Verse(nil), verses...)
		for i := range verses {
			verses[i].BookName = ""
		}
	}

	var menu *fyne.Menu
	var paneSpan selSpan
	switch pane {
	case "styled":
		p := newStyledReadingPane(st, verses)
		w.SetContent(p)
		p.Resize(fyne.NewSize(700, 600))
		runes := []rune(strings.ReplaceAll(p.lay.Text, "\n", " "))
		num := indexRunes(string(runes), superscriptNumber(3))
		if num < 0 {
			t.Fatal("verse 3's number is not in the styled pane")
		}
		at := indexRunes(string(runes[num:]), "Praise")
		if at < 0 {
			t.Fatal("verse 3's words are not in the styled pane")
		}
		start := num + at
		p.setSelection(start, start+len([]rune(turnWords)))
		paneSpan = p.selectedVerseSpan().inChapter(versesChapter(p.verses))
		menu = p.studyMenu()
	case "legacy":
		c := newChapterText(st, verses)
		w.SetContent(c)
		w.Canvas().Focus(c)
		legacyPaneSelect(t, c, 3, turnWords)
		paneSpan = c.selectionSpan().inChapter(c.paneChapter())
		menu = c.selectionMenu()
	default:
		t.Fatalf("no pane %q", pane)
	}

	// The reading view rebuilt for the chapter it turns to, as the window's
	// own refresh does.
	st.showReading = func() {
		w.SetContent(newStyledReadingPane(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)))
	}
	muteEngines(t)
	gAudio.stop()
	t.Cleanup(gAudio.stop)
	gAudio.advanceAndContinue(st, audioTTS, "", chapterAudioFingerprint(st))
	if st.CurrentBook != "Psalms" || st.CurrentChapter != 136 {
		t.Fatalf("control: narration did not turn the chapter (%s %d)", st.CurrentBook, st.CurrentChapter)
	}
	return turnedMenu{st: st, w: w, menu: menu, paneSpan: paneSpan}
}

// The edition's own cross-reference block (publisherCrossRefsFor), opened
// after narration has turned the chapter under a selection, is read in the
// selection's chapter, as the verses it is handed are: the Psalm's title
// notes are the selected chapter's, and a citation of a passage that holds
// the note's own verse is set as the passage on screen, bold and inert,
// while one of the next chapter is a link. Invented Psalm 135 and 136 titles
// and notes. Control: the same selection with the reader still on Psalm 135.
func TestTheEditionsCrossReferenceBlockIsTheSelectionsChapterAfterATurn(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	licenseNKJVForTest(t)
	withPublisherCrossRefs(t, "nkjv", true)
	xr := func(text string, refs ...NoteRef) Footnote {
		return Footnote{Kind: footnoteKindCrossref, Caller: "-", Text: text, Refs: refs}
	}
	st := turningChapterState()
	st.CurrentVersion = "nkjv"
	ps135 := st.Bible.Verses["Psalms"][135]
	ps135[0].Footnotes = []Footnote{xr("Ps. 135:1-3; 136:1",
		NoteRef{ID: "PSA.135.1-PSA.135.3", Start: 0, End: 11}, NoteRef{ID: "PSA.136.1", Start: 13, End: 18})}
	st.Bible.Superscriptions = map[string]map[int]Superscription{"Psalms": {
		135: {Text: "A song of the gate.", Footnotes: []Footnote{xr("Neh. 12:45")}},
		136: {Text: "A song of the harvest.", Footnotes: []Footnote{xr("Ezra 3:11")}},
	}}
	span := paneSelSpan(1, 1, "1 ", true, "Psalms", 135)
	words := ps135[0].Text

	for _, turned := range []bool{false, true} {
		if turned && (!advanceToNextChapter(st) || st.CurrentChapter != 136) {
			t.Fatalf("control: narration did not turn the chapter (%s %d)", st.CurrentBook, st.CurrentChapter)
		}
		lst := buildCrossRefList(st, selectionVerses(st, words, span), nil, nil, st.pal(), func(crossRef) {})
		all := joinedTexts(lst.Objects)
		links := hyperlinksIn(lst.Objects)
		if !strings.Contains(all, "Neh. 12:45") || strings.Contains(all, "Ezra 3:11") {
			t.Errorf("reader on Psalm %d: the block does not carry Psalm 135's title note alone:\n%s", st.CurrentChapter, all)
		}
		if len(links) != 1 || links[0] != "136:1" {
			t.Errorf("reader on Psalm %d: the block links %q, want 136:1 alone, with 135:1-3 the verse's own passage", st.CurrentChapter, links)
		}
		if lst.PublisherRows != 2 {
			t.Errorf("reader on Psalm %d: %d publisher rows, want the title's and verse 1's", st.CurrentChapter, lst.PublisherRows)
		}
	}
}

// item is the menu item at path, by label, or a failure.
func (m turnedMenu) item(t *testing.T, path ...string) *fyne.MenuItem {
	t.Helper()
	items := m.menu.Items
	var it *fyne.MenuItem
	for _, label := range path {
		it = nil
		for _, cand := range items {
			if cand.Label == label {
				it = cand
			}
		}
		if it == nil {
			t.Fatalf("no %q in the menu (%v)", label, path)
		}
		if it.ChildMenu != nil {
			items = it.ChildMenu.Items
		}
	}
	return it
}

// Each verb of a desktop pane's menu, clicked after narration turned the
// chapter under it, reads the selection in the chapter it was made in: the
// study request and its panel, Share with citation, Share as link, Share
// with note and Cross-references. Both desktop panes, the styled one Windows
// and Linux ship and the legacy Entry pane behind it.
func TestTheDesktopMenuReadsTheSelectionInItsChapterAfterNarrationTurnsIt(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, pane := range []string{"styled", "legacy"} {
		t.Run(pane+"/Explain", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			passages := make(chan aiPassage, 2)
			done := make(chan context.Context, 2)
			prevRun, prevFit := aiActionRun, studyRemeasure
			studyRemeasure = func(fit func()) { fit() }
			aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, _ selSpan) (string, error) {
				p, _ := aiPassageFromContext(ctx)
				passages <- p
				done <- ctx
				return "an answer", nil
			}
			t.Cleanup(func() { aiActionRun, studyRemeasure = prevRun, prevFit })
			m.item(t, "Study with AI", "Explain").Action()
			var p aiPassage
			select {
			case p = <-passages:
			case <-time.After(2 * time.Second):
				t.Fatal("Explain never reached the study request")
			}
			<-(<-done).Done()
			if p.Ref != turnRef || p.Chapter != 135 || p.PreviewText != turnWords {
				t.Errorf("the request names %q (chapter %d, %q), want %s", p.Ref, p.Chapter, p.PreviewText, turnRef)
			}
			if body := aiReportMailBodyForPassage(aiActionExplain, p, "", "an answer"); !strings.Contains(body, "Reference: "+turnRef+"\n") {
				t.Errorf("the Report mail reads %q, want it to name %s", body, turnRef)
			}
			panel := studyOverlay(t, m.w)
			t.Cleanup(panel.Hide)
			if !treeHasText(panel, expectedStudyPanelCitation(m.st, turnRef)) {
				t.Errorf("the panel header does not name %s; texts %v", turnRef, treeTexts(panel))
			}
		})
		t.Run(pane+"/Share with citation", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			shared := captureShares(t)
			m.item(t, "Share", "Share with citation").Action()
			if len(*shared) != 1 || !strings.Contains((*shared)[0], "builder of the stone towers") || !strings.Contains((*shared)[0], "— "+turnRef+" (") {
				t.Errorf("shared %q, want the verse's words cited %s", *shared, turnRef)
			}
		})
		t.Run(pane+"/Share as image", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			var quotes, cites []string
			prev := previewCardRender
			previewCardRender = func(_ *AppState, quote, cite, _ string, _ int) (string, error) {
				quotes, cites = append(quotes, quote), append(cites, cite)
				return "", context.Canceled // nothing drawn; the card's words are what is read
			}
			t.Cleanup(func() { previewCardRender = prev })
			m.item(t, "Share", "Share as image").Action()
			if sheet := m.w.Canvas().Overlays().Top(); sheet != nil {
				t.Cleanup(sheet.Hide)
			}
			if len(cites) != 1 || cites[0] != turnRef || !strings.Contains(quotes[0], "builder of the stone towers") {
				t.Errorf("the card was drawn with %q cited %q, want the verse's words cited %s", quotes, cites, turnRef)
			}
		})
		t.Run(pane+"/Share as link", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			shared := captureShares(t)
			m.item(t, "Share", "Share as link").Action()
			want := ShareLinkURL(m.st.currentVersion().ID, "Psalms", 135, 3, 3)
			if len(*shared) != 1 || !strings.HasPrefix((*shared)[0], turnRef+" (") || !strings.HasSuffix((*shared)[0], "\n"+want) {
				t.Errorf("shared %q, want %s and %s", *shared, turnRef, want)
			}
		})
		t.Run(pane+"/Share with note", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			shared := captureShares(t)
			m.item(t, "Share", "Share with note").Action()
			sheet := m.w.Canvas().Overlays().Top()
			if sheet == nil {
				t.Fatal("the note composer did not open")
			}
			t.Cleanup(sheet.Hide)
			if !treeHasText(sheet, expectedPreviewCitation(turnRef, m.st.currentVersion().Abbrev)) {
				t.Errorf("the composer does not name %s; texts %v", turnRef, treeTexts(sheet))
			}
			send := findTreeButton(sheet, "Share")
			if send == nil {
				t.Fatalf("the composer has no Share button; texts %v", treeTexts(sheet))
			}
			test.Tap(send)
			want := ShareLinkURL(m.st.currentVersion().ID, "Psalms", 135, 3, 3)
			if len(*shared) != 1 || !strings.HasPrefix((*shared)[0], turnRef+" (") || !strings.HasSuffix((*shared)[0], "\n"+want) {
				t.Errorf("shared %q, want %s and %s", *shared, turnRef, want)
			}
		})
		t.Run(pane+"/Cross-references", func(t *testing.T) {
			m := menuBuiltThenNarrationTurns(t, pane)
			prev := crossRefsRun
			crossRefsRun = func(func()) {} // the dataset never lands; the header is what is read
			t.Cleanup(func() { crossRefsRun = prev })
			m.item(t, "Cross-references").Action()
			sheet := m.w.Canvas().Overlays().Top()
			if sheet == nil {
				t.Fatal("the cross-references panel did not open")
			}
			t.Cleanup(sheet.Hide)
			if !treeHasText(sheet, turnRef) || treeHasText(sheet, turnNextRef) {
				t.Errorf("the panel does not cite %s alone; texts %v", turnRef, treeTexts(sheet))
			}
		})
	}
}

// A pane that cannot name the chapter of the verses it laid out hands the
// menu a span with no chapter, and the menu stamps the chapter on screen as
// it is built (selectionStudyMenu), so a verb clicked after narration has
// turned the chapter still reads the selection there. Both desktop panes,
// over the invented Psalm 135 with the book taken off the verses they lay
// out. Control: the span each pane hands over names verse 3 and no chapter,
// so the chapter the verbs read is the menu's.
func TestAMenuOverASpanWithNoChapterReadsItInTheChapterItWasBuiltIn(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	for _, pane := range []string{"styled", "legacy"} {
		t.Run(pane+"/Share with citation", func(t *testing.T) {
			m := menuBuiltOverThenNarrationTurns(t, pane, false)
			if m.paneSpan.lo != 3 || m.paneSpan.hi != 3 || m.paneSpan.book != "" || m.paneSpan.chapter != 0 {
				t.Fatalf("control: the pane handed over %+v, want verse 3 with no chapter", m.paneSpan)
			}
			shared := captureShares(t)
			m.item(t, "Share", "Share with citation").Action()
			if len(*shared) != 1 || !strings.Contains((*shared)[0], "builder of the stone towers") || !strings.Contains((*shared)[0], "— "+turnRef+" (") {
				t.Errorf("shared %q, want the verse's words cited %s", *shared, turnRef)
			}
		})
		t.Run(pane+"/Share as link", func(t *testing.T) {
			m := menuBuiltOverThenNarrationTurns(t, pane, false)
			if m.paneSpan.book != "" {
				t.Fatalf("control: the pane named %s %d", m.paneSpan.book, m.paneSpan.chapter)
			}
			shared := captureShares(t)
			m.item(t, "Share", "Share as link").Action()
			want := ShareLinkURL(m.st.currentVersion().ID, "Psalms", 135, 3, 3)
			if len(*shared) != 1 || !strings.HasPrefix((*shared)[0], turnRef+" (") || !strings.HasSuffix((*shared)[0], "\n"+want) {
				t.Errorf("shared %q, want %s and %s", *shared, turnRef, want)
			}
		})
	}
}
