//go:build !ios && !android

package bibletext

// A SHARE THAT CANNOT FINISH SAYS SO ON THE DESKTOP (share_failed.go;
// docs/ROADMAP.md, Later, row 2), driven through the share sheet harness as
// a Linux reader drives it, or a Windows reader whose Share sheet cannot
// open, or a Mac reader through the mimic: Email… whose compose fails says
// so in one line under the buttons and nothing else changes; a picture no
// folder took and the file manager did not show, a card that cannot be read
// at the tap and a card that cannot be drawn each end in the sheet with
// "Could not share the card." and Done alone. Before, each of these ended
// without a word. Each case has a control that must not say it.

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// THE WORDS ARE THE APPROVED ONES, held as literals: the new line under the
// buttons, approved on 15 October 2026, and the phones' two lines, reused.
func TestTheWordsForAShareThatCannotFinishAreTheApprovedOnes(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{shareLineEmailFailed, "Could not open your email program."},
		{shareFailedCard, "Could not share the card."},
		{shareFailedPassage, "Could not share the passage."},
	} {
		if c.got != c.want {
			t.Errorf("the app says %q, want %q", c.got, c.want)
		}
	}
}

// failedSheetProblems is what keeps the top of the canvas from being the
// sheet for a share that could not finish, saying line: that line and Done,
// and nothing else, all of it shown; nil when it is that sheet.
func failedSheetProblems(h *shareSheetHarness, line string) []string {
	h.t.Helper()
	p := h.top()
	if p == nil || !p.Visible() {
		return []string{"no sheet is on top of the canvas"}
	}
	var problems []string
	if texts, want := sheetTexts(p), []string{line, shareButtonDone}; !reflect.DeepEqual(texts, want) {
		problems = append(problems, "the sheet reads "+strings.Join(texts, " | ")+", want "+strings.Join(want, " | "))
	}
	walkTree(p.Content, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok && l.Text == line && !l.Visible() {
			problems = append(problems, "the line is hidden")
		}
	})
	return problems
}

// labelOn is the label reading text on p, or nil.
func labelOn(p *widget.PopUp, text string) *widget.Label {
	var found *widget.Label
	walkTree(p.Content, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.Label); ok && found == nil && l.Text == text {
			found = l
		}
	})
	return found
}

// emailFailedShown reports whether p shows the line under its buttons.
func emailFailedShown(p *widget.PopUp) bool {
	l := labelOn(p, shareLineEmailFailed)
	return l != nil && l.Visible()
}

// tapEmail presses Email… on p and takes the compose it made, which the
// harness runs in place (shareEmailAside); the wait is for a tree that runs
// it on a goroutine of its own.
func (h *shareSheetHarness) tapEmail(p *widget.PopUp) {
	h.t.Helper()
	b := findTreeButton(p.Content, shareButtonEmail)
	if b == nil || !b.Visible() {
		h.t.Fatalf("control: the sheet offers no Email…; texts %v", sheetTexts(p))
	}
	test.Tap(b)
	select {
	case <-h.composed:
	case <-time.After(5 * time.Second):
		h.t.Fatal("control: Email… composed nothing")
	}
}

// AN EMAIL… THAT CANNOT OPEN THE MAIL PROGRAM SAYS SO, in one line under the
// buttons, on the text sheet and the picture's, and the rest of the sheet
// stays as it was: the heading, its line, the box with the clipboard's text
// and the three buttons where they were in their row. A later compose that
// opens takes the line away; one the reader cancelled at the portal says
// nothing, and takes away a line an earlier press left. Control: a compose
// that opens says nothing. Before, a failed compose wrote a log line and the
// sheet stayed as it was, the button pressed and nothing opened.
func TestAnEmailThatCannotOpenSaysSoUnderTheButtons(t *testing.T) {
	failed := errors.Join(errors.New("the Email portal answered 2: the desktop has nothing to compose with"),
		errors.New("xdg-email exited 3"), errors.New("xdg-open exited 4"))

	t.Run("text", func(t *testing.T) {
		h := newShareSheetHarness(t)
		h.shareCitation()
		p := h.sheet()
		before := sheetTexts(p)
		box, _ := boxText(p)

		h.tapEmail(p)
		if emailFailedShown(p) {
			t.Fatal("control: a compose that opened says it could not")
		}

		h.composeErr = failed
		h.tapEmail(p)
		if h.top() != p {
			t.Fatal("the sheet was replaced or closed by the failure")
		}
		if !emailFailedShown(p) {
			t.Fatalf("a failed compose says nothing; the sheet reads %v", sheetTexts(p))
		}
		if got, want := sheetTexts(p), append(append([]string{}, before...), shareLineEmailFailed); !reflect.DeepEqual(got, want) {
			t.Errorf("the sheet reads %v, want what it read before and the line: %v", got, want)
		}
		if got, _ := boxText(p); got != box || h.clipboard() != box {
			t.Errorf("the box or the clipboard changed: %q, %q, want %q", got, h.clipboard(), box)
		}
		test.WidgetRenderer(p).Layout(p.Size())
		d := fyne.CurrentApp().Driver()
		line := labelOn(p, shareLineEmailFailed)
		lineTop := d.AbsolutePositionForObject(line).Y
		for _, label := range []string{shareButtonEmail, shareButtonCopyAgain, shareButtonDone} {
			b := findTreeButtonObject(p.Content, label)
			if bottom := d.AbsolutePositionForObject(b).Y + b.Size().Height; lineTop < bottom {
				t.Errorf("the line starts at %.0f, above the foot of %s at %.0f", lineTop, label, bottom)
			}
		}
		h.wantOnCanvas(p)

		h.composeErr = nil
		h.tapEmail(p)
		if emailFailedShown(p) {
			t.Error("a compose that opened after a failed one leaves the line")
		}

		h.composeErr = failed
		h.tapEmail(p)
		h.composeErr = portalEmailResponse(1)
		h.tapEmail(p)
		if emailFailedShown(p) {
			t.Error("a compose the reader cancelled leaves the line an earlier one left")
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		h := newShareSheetHarness(t)
		h.shareLink()
		p := h.sheet()
		h.composeErr = portalEmailResponse(1)
		h.tapEmail(p)
		if emailFailedShown(p) {
			t.Error("a compose the reader cancelled says the mail program could not be opened")
		}
		// Control: an answer of 3 from the portal is a failure.
		h.composeErr = portalEmailResponse(3)
		h.tapEmail(p)
		if !emailFailedShown(p) {
			t.Error("control: the portal's failure says nothing")
		}
	})

	t.Run("picture", func(t *testing.T) {
		h := newShareSheetHarness(t)
		h.shareImage()
		p := h.sheet()
		h.composeErr = failed
		h.tapEmail(p)
		if !emailFailedShown(p) || !sheetHas(p, shareSheetImageHeading) || !sheetHas(p, shareLineImage) {
			t.Errorf("the picture's sheet reads %v, want its heading, its line and %q", sheetTexts(p), shareLineEmailFailed)
		}
	})
}

// THE LINE ANSWERS THE LAST PRESS, AND ONLY WHILE THE SHEET IS UP: an earlier
// press answering after a later one has changes nothing, and an answer that
// comes after Done finds no sheet to change. Mutations: no press count (the
// late failure puts the line back); no check that the sheet is still up
// (the line is added to a sheet that has gone).
func TestTheEmailLineAnswersTheLastPressOnly(t *testing.T) {
	h := newShareSheetHarness(t)
	var pending []func()
	shareEmailAside = func(compose func() error, done func(error)) {
		err := compose()
		pending = append(pending, func() { done(err) })
	}
	h.shareCitation()
	p := h.sheet()

	h.composeErr = errors.New("the first press failed")
	h.tapEmail(p)
	h.composeErr = nil
	h.tapEmail(p)
	if len(pending) != 2 {
		t.Fatalf("%d of the two composes handed their answer back to the sheet", len(pending))
	}
	pending[1]()
	pending[0]()
	if emailFailedShown(p) {
		t.Error("the first press's late failure is said though the second opened a compose")
	}

	pending = nil
	h.composeErr = errors.New("this press failed")
	h.tapEmail(p)
	test.Tap(findTreeButton(p.Content, shareButtonDone))
	if len(pending) != 1 {
		t.Fatalf("the compose handed back %d answers, want 1", len(pending))
	}
	pending[0]()
	if labelOn(p, shareLineEmailFailed) != nil || h.overlays() != 0 {
		t.Errorf("an answer after Done changed the closed sheet; overlays %d", h.overlays())
	}
}

// THE LINE COMES BACK WITH THE SHEET after a light/dark change, and the
// sheet stays one sheet. Mutation: the reopen opening the sheet as first
// shared.
func TestTheEmailLineComesBackAfterALightDarkChange(t *testing.T) {
	h := newShareSheetHarness(t)
	h.shareCitation()
	h.composeErr = errors.New("no mail program")
	h.tapEmail(h.sheet())
	h.flip()
	again := h.sheet()
	if h.overlays() != 1 || !emailFailedShown(again) {
		t.Errorf("after the change the sheet reads %v (overlays %d), want the line back", sheetTexts(again), h.overlays())
	}
}

// AN EMAIL… ANSWER THAT COMES AFTER A LIGHT/DARK CHANGE IS SAID ON THE SHEET
// THAT CAME BACK. A compose can take a while — the Email portal's wait, or
// xdg-email waiting on a mail program it started — and a light/dark change
// meanwhile rebuilds the window and brings the sheet back in its place.
// Before, the answer went to the sheet the rebuild had taken down: a failure
// was never said, and a compose that opened left the line an earlier press
// had brought back. Control: a press on the sheet that came back is the last
// press, and the earlier one answering after it changes nothing.
// Mutations: the sheet that came back not taking the answers over; the
// reopen opening the sheet with no Email… state.
func TestAnEmailAnswerAfterALightDarkChangeIsSaidOnTheSheetThatCameBack(t *testing.T) {
	// held keeps each compose's answer until the test hands it back.
	held := func() *[]func() {
		var pending []func()
		shareEmailAside = func(compose func() error, done func(error)) {
			err := compose()
			pending = append(pending, func() { done(err) })
		}
		return &pending
	}
	flipped := func(h *shareSheetHarness, before *widget.PopUp) *widget.PopUp {
		h.t.Helper()
		h.flip()
		again := h.sheet()
		if again == before || before.Visible() || h.overlays() != 1 {
			h.t.Fatalf("control: the light/dark change did not bring the sheet back in its place (overlays %d)", h.overlays())
		}
		return again
	}

	t.Run("a failure", func(t *testing.T) {
		h := newShareSheetHarness(t)
		pending := held()
		h.shareCitation()
		first := h.sheet()
		h.composeErr = errors.New("no mail program")
		h.tapEmail(first)
		again := flipped(h, first)
		(*pending)[0]()
		if !emailFailedShown(again) {
			t.Errorf("a failure answered after the change is not said; the sheet reads %v", sheetTexts(again))
		}
		if h.overlays() != 1 || first.Visible() {
			t.Errorf("the answer brought back the sheet the change took down (overlays %d)", h.overlays())
		}
	})

	t.Run("a compose that opened", func(t *testing.T) {
		h := newShareSheetHarness(t)
		pending := held()
		h.shareCitation()
		first := h.sheet()
		h.composeErr = errors.New("no mail program")
		h.tapEmail(first)
		(*pending)[0]()
		if !emailFailedShown(first) {
			t.Fatal("control: the failed compose is not said")
		}
		h.composeErr = nil
		h.tapEmail(first)
		again := flipped(h, first)
		if !emailFailedShown(again) {
			t.Fatal("control: the line does not come back with the sheet")
		}
		(*pending)[1]()
		if emailFailedShown(again) {
			t.Error("a compose that opened, answered after the change, leaves the line an earlier press left")
		}
	})

	t.Run("control: the last press is on the sheet that came back", func(t *testing.T) {
		h := newShareSheetHarness(t)
		pending := held()
		h.shareCitation()
		first := h.sheet()
		h.composeErr = errors.New("no mail program")
		h.tapEmail(first)
		again := flipped(h, first)
		h.composeErr = nil
		h.tapEmail(again)
		(*pending)[1]()
		(*pending)[0]()
		if emailFailedShown(again) {
			t.Error("the earlier press's failure is said though the press after the change opened a compose")
		}
	})
}

// A PORTAL THAT TOOK THE COMPOSE AND GAVE NO ANSWER IS NOT SAID TO HAVE
// FAILED. Once the Email portal has taken the call, portalRequest can still
// come back with no answer: none within portalAnswerWait, the session bus
// closing first, or an answer with no code. The compose ends there, as it
// did, since a slow portal may yet open one; and the line under the buttons
// says nothing, since nothing says the mail program could not be opened,
// and takes away a line an earlier press left. Before, the sheet said "Could
// not open your email program." once the app stopped waiting. Controls: a
// call the portal did not take hands on to the next route; an answer of 3 is
// a failure and is said; 1 is the reader's cancel; 0 opened.
func TestAPortalThatGaveNoAnswerIsNotSaidToHaveFailed(t *testing.T) {
	unanswered := []string{
		"no answer from the portal",
		"the session bus closed before the portal answered",
		"the portal answered with no response code",
	}
	for _, why := range unanswered {
		err := portalComposeOutcome(0, true, errors.New(why))
		var stop mailRouteStop
		if !errors.As(err, &stop) {
			t.Errorf("%s: %v does not end the compose", why, err)
		}
		if shareEmailFailed(err) {
			t.Errorf("%s: the sheet would say the mail program could not be opened", why)
		}
	}
	tried := 0
	next := func(string, string, string) error { tried++; return nil }
	err := composeByRoutes([]mailRoute{
		func(string, string, string) error {
			return portalComposeOutcome(0, false, errors.New("no portal on the bus"))
		},
		next,
	}, "s", "b", "")
	if err != nil || tried != 1 {
		t.Errorf("control: a call the portal did not take: composeByRoutes = %v after the next route ran %d times, want it run once", err, tried)
	}
	for _, c := range []struct {
		code   uint32
		failed bool
	}{{0, false}, {1, false}, {3, true}} {
		if got := shareEmailFailed(portalComposeOutcome(c.code, true, nil)); got != c.failed {
			t.Errorf("control: the portal answering %d is said to have failed: %v, want %v", c.code, got, c.failed)
		}
	}

	// The Email portal's route hands what portalRequest returned to
	// portalComposeOutcome, as it is.
	linux := javaBlockAfter(t, readSourceFile(t, "share_email_linux.go"), "func composeEmailViaPortal(subject, body, attachment string) error")
	if !strings.Contains(linux, `return portalComposeOutcome(portalRequest(conn, portalEmailIface+".ComposeEmail", options, portalAnswerWait, ""))`) {
		t.Errorf("composeEmailViaPortal must end in portalComposeOutcome of the portal's answer:\n%s", linux)
	}

	h := newShareSheetHarness(t)
	h.shareCitation()
	p := h.sheet()
	h.composeErr = portalEmailResponse(3)
	h.tapEmail(p)
	if !emailFailedShown(p) {
		t.Fatal("control: the portal's failure is not said")
	}
	h.composeErr = portalComposeOutcome(0, true, errors.New(unanswered[0]))
	h.tapEmail(p)
	if emailFailedShown(p) {
		t.Error("a compose the portal took and gave no answer to is said not to have opened")
	}
}

// A PICTURE NO FOLDER TOOK, WHICH THE FILE MANAGER DID NOT SHOW, SAYS THE
// CARD COULD NOT BE SHARED: the sheet with that line and Done alone, which
// Done, Escape and Return close as they close the Picture saved sheet, and
// which comes back after a light/dark change. Before, no sheet opened and
// the share ended without a word. Control: the file manager said it showed
// the picture, and the Picture saved sheet opens instead.
func TestAPictureNoFolderTookSaysTheCardCouldNotBeShared(t *testing.T) {
	noHome := func(t *testing.T) {
		gone := filepath.Join(redirectHome(t), "gone")
		t.Setenv("HOME", gone)
		t.Setenv("USERPROFILE", gone)
	}
	for _, way := range []string{"Done", "Escape", "Return"} {
		t.Run(way, func(t *testing.T) {
			h := imageShareHarness(t, false)
			noHome(t)
			imageShare(h)
			if p := failedSheetProblems(h, shareFailedCard); p != nil {
				t.Fatal(p)
			}
			if len(h.probed) != 0 {
				t.Errorf("the sheet asked the mail probe %v; it offers no Email…", h.probed)
			}
			if way == "Done" {
				h.flip()
				if p := failedSheetProblems(h, shareFailedCard); p != nil || h.overlays() != 1 {
					t.Fatalf("after a light/dark change (overlays %d): %v", h.overlays(), p)
				}
			}
			switch way {
			case "Done":
				test.Tap(findTreeButton(h.top().Content, shareButtonDone))
			case "Escape":
				h.st.dismissSheet()
			case "Return":
				h.typeOnCanvas(fyne.KeyReturn)
			}
			if h.overlays() != 0 {
				t.Errorf("%s left %d sheets up", way, h.overlays())
			}
		})
	}
	t.Run("control: the file manager showed it", func(t *testing.T) {
		h := imageShareHarness(t, true)
		noHome(t)
		_, _, texts := imageShare(h)
		if !hasText(texts, shareLineImageShown) || failedSheetProblems(h, shareFailedCard) == nil {
			t.Errorf("the sheet reads %v, want the Picture saved sheet", sheetTexts(h.top()))
		}
	})
}

// A CARD THAT COULD NOT BE READ AT THE TAP SAYS THE CARD COULD NOT BE SHARED,
// under Linux's rules and under Windows', whose Share sheet hands its
// fallback the renderer's file when its own copy fails; nothing is saved or
// shown, as before (TestACardThatCouldNotBeReadOpensNoSheet). Before, the
// share ended without a word. Control: the card there is saved and confirmed.
func TestACardThatCouldNotBeReadSaysTheCardCouldNotBeShared(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		t.Run(goos, func(t *testing.T) {
			h := imageShareHarness(t, true)
			shareImageGOOS = goos
			mkdirs(t, filepath.Join(redirectHome(t), "Downloads"))
			fallbackShareImage(filepath.Join(t.TempDir(), "gone.png"))
			if p := failedSheetProblems(h, shareFailedCard); p != nil {
				t.Error(p)
			}
			if len(h.revealed) != 0 {
				t.Errorf("the file manager was asked to show %v", h.revealed)
			}
		})
		t.Run(goos+", control: the card there", func(t *testing.T) {
			h := imageShareHarness(t, true)
			shareImageGOOS = goos
			mkdirs(t, filepath.Join(redirectHome(t), "Downloads"))
			_, _, texts := imageShare(h)
			if texts == nil || failedSheetProblems(h, shareFailedCard) == nil {
				t.Errorf("the sheet reads %v, want the Picture saved sheet", sheetTexts(h.top()))
			}
		})
	}
}

// A CARD THAT CANNOT BE DRAWN OPENS NO PREVIEW AND SAYS THE CARD COULD NOT BE
// SHARED, the reading overlay left where it was. Before, the preview opened
// on an empty box whose Share closed it and handed nothing over. Control: a
// card that draws opens the preview on it. On iOS and Android the same call
// says it in an alert and a toast (share_failed_test.go).
func TestACardThatCannotBeDrawnSaysTheCardCouldNotBeShared(t *testing.T) {
	h := newShareSheetHarness(t)
	prev := previewCardRender
	t.Cleanup(func() { previewCardRender = prev })
	previewCardRender = func(*AppState, string, string, string, int) (string, error) {
		return "", errors.New("the card's file could not be written")
	}
	hidden := 0
	prevHide := h.st.hideReadingOverlay
	h.st.hideReadingOverlay = func() {
		hidden++
		if prevHide != nil {
			prevHide()
		}
	}
	showShareImagePreview(h.st, "“In the beginning was the Word.”", "John 1:1", "Sample")
	if p := failedSheetProblems(h, shareFailedCard); p != nil {
		t.Fatal(p)
	}
	if h.overlays() != 1 {
		t.Errorf("%d sheets are up, want the one saying so", h.overlays())
	}
	test.Tap(findTreeButton(h.top().Content, shareButtonDone))
	if h.overlays() != 0 || hidden != 1 || h.restored != 1 {
		t.Errorf("after Done: overlays %d, overlay hidden %d and shown %d times, want 0, 1, 1", h.overlays(), hidden, h.restored)
	}

	// Control: a card that draws.
	card := filepath.Join(t.TempDir(), "card.png")
	if err := os.WriteFile(card, []byte("not a real png"), 0o644); err != nil {
		t.Fatal(err)
	}
	previewCardRender = func(*AppState, string, string, string, int) (string, error) { return card, nil }
	showShareImagePreview(h.st, "“In the beginning was the Word.”", "John 1:1", "Sample")
	if p := h.top(); p == nil || !sheetHas(p, "Share as image") || failedSheetProblems(h, shareFailedCard) == nil {
		t.Errorf("control: a card that draws opens %v, want the preview", sheetTexts(h.top()))
	}
}
