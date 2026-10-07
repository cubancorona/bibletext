package bibletext

// The selection's verse span on the study surfaces (ai.go aiPassageFor): the
// answer panel's header, the Ask sheet's header, the Report mail and the
// panel's two ways back — the reopen a landed answer registers for a
// light/dark rebuild, and the fresh panel an answer opens when the rebuild
// evicted the one it was in flight for — all name the verses the request
// named, and hand the span on so the re-run hits the answer already paid for.
//
// The fixture is the repeated refrain of selection_span_test.go: selected at
// verse 2 it cites "Psalms 136:2" by position, while the text alone matches
// every verse and cites 1–3, so a surface that drops the span cites
// differently and is caught.
//
// A landed answer runs its completion on the request's own goroutine (the
// test driver runs fyne.Do inline), so these tests never read a panel the
// completion may still be painting: the request's context is cancelled when
// that goroutine returns, after the completion, and a receive on its Done is
// the edge after which the panel is whole.

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// studyRequest is one call at the request seam: what it carried, and the
// channel the test answers it on. Unanswered requests park for good, as the
// suite's other study stubs do.
type studyRequest struct {
	ctx    context.Context
	span   selSpan
	answer chan string
}

// stubStudyRequests makes every study request report itself and wait to be
// answered. An answer that lands re-measures the panel at once rather than on
// a timer, so the landing is whole when the request's goroutine returns.
func stubStudyRequests(t *testing.T) chan studyRequest {
	t.Helper()
	prevFit := studyRemeasure
	studyRemeasure = func(fit func()) { fit() }
	t.Cleanup(func() { studyRemeasure = prevFit })
	reqs := make(chan studyRequest, 4)
	never := make(chan struct{})
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, span selSpan) (string, error) {
		r := studyRequest{ctx: ctx, span: span, answer: make(chan string, 1)}
		reqs <- r
		select {
		case a := <-r.answer:
			return a, nil
		case <-never:
			return "", ctx.Err()
		}
	}
	t.Cleanup(func() { aiActionRun = prev })
	return reqs
}

func nextStudyRequest(t *testing.T, reqs chan studyRequest, what string) studyRequest {
	t.Helper()
	select {
	case r := <-reqs:
		return r
	case <-time.After(2 * time.Second):
		t.Fatalf("%s never reached the request seam", what)
		return studyRequest{}
	}
}

// settled waits for the request's goroutine to return — its completion, the
// panel it painted or opened, and all — which is when its context ends.
func settled(t *testing.T, r studyRequest) {
	t.Helper()
	select {
	case <-r.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the answered request never settled")
	}
}

// refrainPanelState is studyPanelState reading the refrain chapter.
func refrainPanelState(t *testing.T, w fyne.Window) *AppState {
	t.Helper()
	st := studyPanelState(t, w)
	r := refrainChapterState()
	st.Bible, st.CurrentBook, st.CurrentChapter = r.Bible, r.CurrentBook, r.CurrentChapter
	return st
}

// verseTwo is the refrain selected in verse 2, and the citation the share
// card gives it.
var verseTwo = selSpanFromNative(2, 2)

const verseTwoRef = "Psalms 136:2"

// The panel header names the verse the span resolved to, not the verses the
// words match. Control: the same open on a state with no Bible names the
// chapter alone.
func TestAIPanelHeaderNamesTheVerses(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	ctxs := stubAIActionParked(t)

	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := refrainPanelState(t, w)
	showAIPanel(st, aiActionExplain, refrain, "", verseTwo)
	nextCtx(t, ctxs)
	p := studyOverlay(t, w)
	if !treeHasText(p, verseTwoRef) {
		t.Errorf("the header does not name the selected verse; texts %v", treeTexts(p))
	}

	bareWin := test.NewWindow(widget.NewLabel("reading"))
	defer bareWin.Close()
	bare := refrainPanelState(t, bareWin)
	bare.Bible = nil
	showAIPanel(bare, aiActionExplain, refrain, "", verseTwo)
	nextCtx(t, ctxs)
	q := studyOverlay(t, bareWin)
	if !treeHasText(q, "Psalms 136") || treeHasText(q, verseTwoRef) {
		t.Errorf("control: without a Bible the header must name the chapter alone; texts %v", treeTexts(q))
	}
}

// The Ask sheet names the verse the answer panel that follows will name, and
// hands the span on to that panel. Control: a selection no verse resolves
// names the chapter alone.
func TestAskSheetNamesTheVerses(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	reqs := stubStudyRequests(t)

	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := refrainPanelState(t, w)
	promptAskQuestion(st, refrain, verseTwo)
	sheet := w.Canvas().Overlays().Top()
	if sheet == nil || !treeHasText(sheet, "Ask about this passage") {
		t.Fatalf("the Ask sheet did not open; texts %v", treeTexts(sheet))
	}
	if !treeHasText(sheet, verseTwoRef) {
		t.Errorf("the sheet does not name the selected verse; texts %v", treeTexts(sheet))
	}
	var entry *searchKeyEntry
	walkTree(sheet, func(o fyne.CanvasObject) {
		if e, ok := o.(*searchKeyEntry); ok && entry == nil {
			entry = e
		}
	})
	if entry == nil {
		t.Fatal("the Ask sheet has no question field")
	}
	entry.SetText("Why is it repeated?")
	test.Tap(findTreeButton(sheet, "Ask"))
	if r := nextStudyRequest(t, reqs, "the question"); r.span != verseTwo {
		t.Errorf("the panel asked with span %+v, want the sheet's %+v", r.span, verseTwo)
	}
	if p := studyOverlay(t, w); !treeHasText(p, verseTwoRef) {
		t.Errorf("the answer panel the sheet opened does not name the verse; texts %v", treeTexts(p))
	}

	bareWin := test.NewWindow(widget.NewLabel("reading"))
	defer bareWin.Close()
	bare := refrainPanelState(t, bareWin)
	promptAskQuestion(bare, "gods;", selSpan{}) // under the matching probe's floor
	q := bareWin.Canvas().Overlays().Top()
	if q == nil || !treeHasText(q, "Psalms 136") || treeHasText(q, verseTwoRef) {
		t.Errorf("control: an unresolved selection must name the chapter alone; texts %v", treeTexts(q))
	}
}

// The Ask sheet a light/dark rebuild brings back names the same verse.
func TestAskSheetReopensWithTheVerses(t *testing.T) {
	h := newAppearanceHarness(t, true)
	r := refrainChapterState()
	h.state.Bible, h.state.CurrentBook, h.state.CurrentChapter = r.Bible, r.CurrentBook, r.CurrentChapter
	promptAskQuestion(h.state, refrain, verseTwo)
	first := h.top()
	if !sheetHas(first, verseTwoRef) {
		t.Fatalf("control: the sheet did not open naming the verse; texts %v", sheetTexts(first))
	}
	h.flip()
	again := h.top()
	if again == nil || again == first || !sheetHas(again, "Ask about this passage") {
		t.Fatalf("the sheet did not come back; texts %v", sheetTexts(again))
	}
	if !sheetHas(again, verseTwoRef) {
		t.Errorf("the reopened sheet lost the verse; texts %v", sheetTexts(again))
	}
}

// Both ways back carry the span: the fresh panel an answer opens when a
// rebuild evicted the one it was in flight for, and the reopen a landed
// answer registers. Either way the re-run asks with the same span, so it is
// the answer already paid for that the cache serves, and the header names the
// same verse.
func TestStudyPanelReopenCarriesSpan(t *testing.T) {
	t.Run("evicted in flight", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		reqs := stubStudyRequests(t)
		w := test.NewWindow(widget.NewLabel("reading"))
		defer w.Close()
		st := refrainPanelState(t, w)

		showAIPanel(st, aiActionExplain, refrain, "", verseTwo)
		first := nextStudyRequest(t, reqs, "the study request")
		if first.span != verseTwo {
			t.Fatalf("control: the request carried %+v", first.span)
		}
		p := studyOverlay(t, w)
		p.Hide() // a rebuild's drain: not the reader's Close, so the answer may come back
		first.answer <- "an answer"
		second := nextStudyRequest(t, reqs, "the reopened panel's request")
		if second.span != verseTwo {
			t.Errorf("the reopened panel asked with %+v, want %+v", second.span, verseTwo)
		}
		settled(t, first)
		again := studyOverlay(t, w)
		if again == p || !treeHasText(again, verseTwoRef) {
			t.Errorf("the reopened panel does not name the verse; texts %v", treeTexts(again))
		}
	})

	t.Run("answered, then rebuilt", func(t *testing.T) {
		app := test.NewApp()
		defer app.Quit()
		reqs := stubStudyRequests(t)
		w := test.NewWindow(widget.NewLabel("reading"))
		defer w.Close()
		st := refrainPanelState(t, w)

		showAIPanel(st, aiActionExplain, refrain, "", verseTwo)
		first := nextStudyRequest(t, reqs, "the study request")
		p := studyOverlay(t, w)
		first.answer <- "an answer"
		settled(t, first)
		var reopen func()
		for _, r := range st.sheetReopens {
			if r.popup == p {
				reopen = r.capture()
			}
		}
		if reopen == nil {
			t.Fatal("the answered panel registered no way back")
		}
		reopen() // as the appearance gate runs it after the rebuild
		second := nextStudyRequest(t, reqs, "the reopened panel's request")
		if second.span != verseTwo {
			t.Errorf("the reopened panel asked with %+v, want %+v", second.span, verseTwo)
		}
		again := studyOverlay(t, w)
		if again == p || !treeHasText(again, verseTwoRef) {
			t.Errorf("the reopened panel does not name the verse; texts %v", treeTexts(again))
		}
	})
}

// The Report mail names the verses and the study action, and carries a
// question line only for Ask.
func TestAIReportMailNamesVerbAndVerses(t *testing.T) {
	body := aiReportMailBody(aiActionExplain, "John 3:16", "", "the answer")
	for _, want := range []string{"Reference: John 3:16\n", "Study action: Explanation\n", "\nResponse:\nthe answer\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("Explain body lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Question:") {
		t.Errorf("a fixed action carries a question line:\n%s", body)
	}
	ask := aiReportMailBody(aiActionAsk, "John 3:16–18", "Who is the world?", "the answer")
	for _, want := range []string{"Reference: John 3:16–18\n", "Study action: Answer\n", "Question: Who is the world?\n"} {
		if !strings.Contains(ask, want) {
			t.Errorf("Ask body lacks %q:\n%s", want, ask)
		}
	}

	// Through the panel: Report opens the mail with the verse the header
	// named.
	app := test.NewApp()
	defer app.Quit()
	reqs := stubStudyRequests(t)
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := refrainPanelState(t, w)
	showAIPanel(st, aiActionExplain, refrain, "", verseTwo)
	first := nextStudyRequest(t, reqs, "the study request")
	p := studyOverlay(t, w)
	first.answer <- "an answer"
	settled(t, first)
	openedInTests()
	report := findTreeButton(p, "Report")
	if report == nil || report.Disabled() {
		t.Fatalf("no enabled Report button on the answered panel; texts %v", treeTexts(p))
	}
	report.OnTapped()
	opened := openedInTests()
	if len(opened) != 1 {
		t.Fatalf("Report opened %v, want one mail link", opened)
	}
	u, err := url.Parse(opened[0])
	if err != nil || u.Scheme != "mailto" {
		t.Fatalf("Report opened %q", opened[0])
	}
	got := strings.ReplaceAll(u.Query().Get("body"), "\r\n", "\n")
	if !strings.Contains(got, "Reference: "+verseTwoRef+"\n") || !strings.Contains(got, "Study action: Explanation\n") {
		t.Errorf("the mail body does not name the verse and the action:\n%s", got)
	}
}
