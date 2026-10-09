//go:build next

package bibletext

import (
	"math"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func exactPreviewCitation(t *testing.T, root fyne.CanvasObject, want string) *widget.RichText {
	t.Helper()
	var found *widget.RichText
	count := 0
	walkTree(root, func(o fyne.CanvasObject) {
		if rich, ok := o.(*widget.RichText); ok && segmentText(rich.Segments) == want {
			found = rich
			count++
		}
	})
	if count != 1 {
		t.Fatalf("found %d exact citations %q; texts %v", count, want, treeTexts(root))
	}
	if len(found.Segments) != 1 {
		t.Fatalf("citation has %d segments, want one", len(found.Segments))
	}
	segment, ok := found.Segments[0].(*widget.TextSegment)
	if !ok || segment.Text != want || segment.Style.Alignment != fyne.TextAlignLeading || found.Wrapping != fyne.TextWrapWord {
		t.Fatalf("citation must be one exact left-aligned wrapping string %q", want)
	}
	return found
}

func TestPassageExcerptPreviewWrapsCitationOutsideQuoteInsets(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 900, 640)
	win.SetPadded(false)
	const ref = "Synthetic Book with a Long Name for a Narrow Passage Card 123:45–67"
	const abbrev = "SYN"
	text := strings.Repeat("A synthetic passage follows a quiet path beside the field. ", 12)
	preview, excerpt := newPassageExcerptPreview(ref, abbrev, text, 3, st.pal())
	win.SetContent(preview)
	win.Resize(fyne.NewSize(900, 640))
	preview.Refresh()
	citation := exactPreviewCitation(t, preview, ref+" (SYN)")
	wideHeight := citation.Size().Height
	win.Resize(fyne.NewSize(320, 640))
	preview.Refresh()
	if citation.Size().Height <= wideHeight {
		t.Errorf("the narrow citation must wrap into more rows: narrow %vpt, wide %vpt", citation.Size().Height, wideHeight)
	}
	if preview.Size().Width > 320 {
		t.Errorf("the preview widened the 320pt canvas to %vpt", preview.Size().Width)
	}
	driver := fyne.CurrentApp().Driver()
	outer := driver.AbsolutePositionForObject(preview)
	for name, object := range map[string]fyne.CanvasObject{"quote": excerpt, "citation": citation} {
		position := driver.AbsolutePositionForObject(object)
		left := position.X - outer.X
		right := preview.Size().Width - left - object.Size().Width
		wantInset := float32(0)
		if name == "quote" {
			wantInset = 16
		}
		if math.Abs(float64(left-wantInset)) > 0.5 || math.Abs(float64(right-wantInset)) > 0.5 {
			t.Errorf("%s needs %vpt on both sides; left %vpt, right %vpt", name, wantInset, left, right)
		}
	}
	assertPassagePreviewSpacing(t, excerpt, citation)
	renderer := test.WidgetRenderer(excerpt).(*noteExcerptRenderer)
	if len(renderer.drawn()) != 3 {
		t.Errorf("a long portrait quote must take exactly three rows: %v", renderer.drawn())
	}
	for i, row := range renderer.rows {
		if row.TextStyle.Italic || row.FontSource != styledPaneFont() {
			t.Errorf("quote row %d must use the upright reading face", i)
		}
	}
}

func TestPassagePreviewsKeepQuoteCitationAndNativeFieldInOrder(t *testing.T) {
	for _, feature := range []string{"Ask", "Share with note"} {
		t.Run(feature, func(t *testing.T) {
			st := psalm23NoteState()
			win := noteSheetWindow(t, st, true, 320, 1024)
			held := holdNoteSheetTimers(t)
			_, _, _, frames := nativeAskEditor(t, win)
			var wantCitation string
			if feature == "Ask" {
				passage := askComposerFixture()
				wantCitation = "Fixture Book 2:1–2 (FXT)"
				promptAskQuestionForPassage(st, passage, "", selSpan{})
			} else {
				wantCitation = "Psalms 23:1–2 (WEB)"
				promptShareNote(st, drawnSelection(t, st, 1, 2), selSpanFromNative(1, 2))
			}
			popup := topPopup(t, win)
			excerpt, renderer := noteSheetExcerpt(t, popup)
			citation := exactPreviewCitation(t, popup, wantCitation)
			assertPassagePreviewSpacing(t, excerpt, citation)
			driver := fyne.CurrentApp().Driver()
			quoteBottom := driver.AbsolutePositionForObject(excerpt).Y + float32(len(renderer.drawn()))*renderer.rowH
			citationPosition := driver.AbsolutePositionForObject(citation)
			citationBottom := citationPosition.Y + citation.Size().Height
			if citationPosition.Y < quoteBottom {
				t.Errorf("citation overlaps the quote: top %v, quote bottom %v", citationPosition.Y, quoteBottom)
			}
			for _, f := range append([]func(){}, (*held)...) {
				f()
			}
			if len(*frames) == 0 {
				t.Fatal("the native field received no frames")
			}
			for i, frame := range *frames {
				if math.Abs(float64(frame.pos.X-citationPosition.X)) > 0.5 {
					t.Errorf("citation must align with the native field's main margin: citation X %v, frame %d X %v", citationPosition.X, i, frame.pos.X)
				}
				if frame.pos.Y < citationBottom || frame.pos.X+frame.sz.Width > 320 {
					t.Errorf("native frame %d overlaps the citation or leaves the canvas: %+v; citation bottom %v", i, frame, citationBottom)
				}
			}
			test.Tap(findTreeButton(popup, "Cancel"))
		})
	}
}

func TestAskCapturesSelectedPreviewBeforeNavigation(t *testing.T) {
	st := psalm23NoteState()
	win := noteSheetWindow(t, st, true, 393, 852)
	holdNoteSheetTimers(t)
	typed, _, _, _ := nativeAskEditor(t, win)
	reqs := stubCapturedStudyRequests(t)
	const selection = "3 He restoreth my"
	const previewText = "He restoreth my"
	span := selSpanFromNative(3, 3)
	version := st.currentVersion()
	promptAskQuestion(st, selection, span)
	first := topPopup(t, win)
	excerpt, _ := noteSheetExcerpt(t, first)
	if excerpt.text != previewText {
		t.Fatalf("preview must contain only selected words, with no trailing sentence completion: %q", excerpt.text)
	}
	exactPreviewCitation(t, first, "Psalms 23:3 (WEB)")
	*typed = "How does this fixture phrase connect to its verse?"
	reopen := takeTopSheetReopen(st)
	if reopen == nil {
		t.Fatal("the composer did not register a reopen")
	}
	first.Hide()
	st.CurrentBook, st.CurrentChapter, st.CurrentVersion = "John", 1, "bsb"
	reopen()
	again := topPopup(t, win)
	exactPreviewCitation(t, again, "Psalms 23:3 (WEB)")
	if excerpt, _ := noteSheetExcerpt(t, again); excerpt.text != previewText {
		t.Errorf("the reopened preview changed the selected words: %q", excerpt.text)
	}
	test.Tap(findTreeButton(again, "Ask"))
	r := nextCapturedStudyRequest(t, reqs, "captured preview Ask")
	if !r.captured || r.passage.Text != selection || r.text != selection || !r.passage.HasPreview ||
		r.passage.PreviewText != previewText || r.passage.VersionAbbrev != "WEB" || r.passage.Version != version.Name ||
		r.passage.Book != "Psalms" || r.passage.Chapter != 23 || r.passage.Lo != 3 || r.passage.Hi != 3 || r.span != span {
		t.Errorf("request changed its raw text or captured preview identity: %+v", r)
	}
	finishCapturedStudyRequest(t, r, "A synthetic answer.", nil)
	panel := topPopup(t, win)
	exactPreviewCitation(t, panel, "Psalms 23:3 (WEB)")
	if excerpt, _ := noteSheetExcerpt(t, panel); excerpt.text != previewText {
		t.Errorf("answer header changed the captured preview: %q", excerpt.text)
	}
	test.Tap(findTreeButton(panel, "Close"))
}
