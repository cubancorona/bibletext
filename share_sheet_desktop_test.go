//go:build !ios && !android

package bibletext

// THE DESKTOP SHARE CONFIRMATION (share_sheet_desktop.go), driven as a
// Windows or Linux reader drives it: a real window holding CreateMainUI's
// tree with the styled reading pane, the share verbs delivering through the
// desktop fallback exactly as share_other.go routes them there, and every
// verb run from its own entry point — the selection menu's actions, the
// composer's Share, the verse-of-the-day card's icon, the image share's
// hand-off. What each holds: the sheet is on the overlay stack when the
// handler returns, inside the canvas and below the header at 1280x800 and
// at 520x640, in light and in dark; its words are the approved ones; the
// clipboard holds the text the box shows; a click outside does not dismiss
// it; Done, Escape and Return close it and give the canvas back; Copy again
// copies again; a light/dark change brings it back showing the same text,
// and the verse-of-the-day card beneath it; on the note path the sent note's
// card is on the page under it; Email… is offered only when the platform
// says there is a mail client, and hands over the citation, the text and the
// image's file.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// shareCompose is one Email… press as the mail seam saw it.
type shareCompose struct{ subject, body, attachment string }

// withoutMailProbe leaves the mail probe unanswered for the test, so a sheet
// opened for its size alone reaches out to no bus and shows no Email….
func withoutMailProbe(t *testing.T) {
	t.Helper()
	prev := shareEmailProbe
	shareEmailProbe = func(bool, func(bool)) {}
	t.Cleanup(func() { shareEmailProbe = prev })
}

// shareSheetHarness is the appearance harness with the desktop share path
// wired as Linux and Windows wire it, the platform seams the sheet reaches
// out through replaced — the mail probe answers at once with emailOK, a
// compose lands on composed instead of opening a client, the file-manager
// reveal is recorded instead of run — and the sheet's timers held.
type shareSheetHarness struct {
	*appearanceHarness
	st       *AppState
	emailOK  bool
	composed chan shareCompose
	revealed []string
	restored int // calls to showReadingOverlay, the desktop sheet-close consume point
	timers   *[]func()
}

func newShareSheetHarness(t *testing.T) *shareSheetHarness {
	t.Helper()
	prevPane := useStyledPane
	useStyledPane = func() bool { return true }
	t.Cleanup(func() { useStyledPane = prevPane })
	t.Cleanup(resetStyledWiring)
	h := &shareSheetHarness{
		appearanceHarness: newAppearanceHarness(t, false),
		emailOK:           true,
		composed:          make(chan shareCompose, 8),
	}
	h.st = h.state
	h.timers = holdSheetTimers(t)
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })

	prevActive := activeAIState
	activeAIState = h.st
	t.Cleanup(func() { activeAIState = prevActive })
	prevOut := shareTextOut
	shareTextOut = fallbackShareText
	t.Cleanup(func() { shareTextOut = prevOut })
	prevProbe := shareEmailProbe
	shareEmailProbe = func(_ bool, report func(bool)) { report(h.emailOK) }
	t.Cleanup(func() { shareEmailProbe = prevProbe })
	prevCompose := shareEmailCompose
	shareEmailCompose = func(subject, body, attachment string) error {
		h.composed <- shareCompose{subject, body, attachment}
		return nil
	}
	t.Cleanup(func() { shareEmailCompose = prevCompose })
	prevReveal := revealInFileManager
	revealInFileManager = func(p string) { h.revealed = append(h.revealed, p) }
	t.Cleanup(func() { revealInFileManager = prevReveal })
	prevShow := h.st.showReadingOverlay
	h.st.showReadingOverlay = func() {
		h.restored++
		if prevShow != nil {
			prevShow()
		}
	}
	return h
}

func (h *shareSheetHarness) canvas() fyne.Canvas { return h.st.window.Canvas() }

func (h *shareSheetHarness) clipboard() string { return fyne.CurrentApp().Clipboard().Content() }

func (h *shareSheetHarness) selection(lo, hi int) (string, selSpan) {
	return drawnSelection(h.t, h.st, lo, hi), selSpanFromNative(lo, hi)
}

// The verbs, each from its own entry point.

func (h *shareSheetHarness) shareCitation() {
	text, span := h.selection(1, 2)
	dispatchSelectionAction(h.st, selActionShareCite, text, span)
}

func (h *shareSheetHarness) shareLink() {
	text, span := h.selection(1, 2)
	dispatchSelectionAction(h.st, selActionShareLink, text, span)
}

func (h *shareSheetHarness) shareNote(words string) {
	h.t.Helper()
	text, span := h.selection(1, 2)
	promptShareNote(h.st, text, span)
	field := composerField(h.top())
	if field == nil {
		h.t.Fatalf("control: the composer has no field; texts %v", sheetTexts(h.top()))
	}
	field.SetText(words)
	test.Tap(findTreeButton(h.top().Content, "Share"))
}

// shareVerseOfDay opens the card and taps its Share icon; the card is
// returned so a test can look for it beneath the sheet.
func (h *shareSheetHarness) shareVerseOfDay() *widget.PopUp {
	h.t.Helper()
	showVerseOfDay(h.st)
	card := h.top()
	if card == nil || !sheetHas(card, "Verse of the day") {
		h.t.Fatal("control: the verse of the day card did not open")
	}
	share := findIconTapButton(card.Content, theme.MailSendIcon().Name())
	if share == nil {
		h.t.Fatal("control: the card has no Share icon")
	}
	test.Tap(share)
	return card
}

// shareImage hands a rendered card to the image fallback with a Downloads
// folder under a home of its own, and returns where the fallback put it.
func (h *shareSheetHarness) shareImage() string {
	h.t.Helper()
	home := h.t.TempDir()
	h.t.Setenv("HOME", home)
	downloads := filepath.Join(home, "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		h.t.Fatal(err)
	}
	src := filepath.Join(h.t.TempDir(), "card.png")
	if err := os.WriteFile(src, []byte("not a real png"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	shareImageSubject = "John 1:1 (Sample)"
	fallbackShareImage(src)
	entries, err := os.ReadDir(downloads)
	if err != nil || len(entries) != 1 {
		h.t.Fatalf("control: the image share must leave one file in Downloads; have %d (%v)", len(entries), err)
	}
	return filepath.Join(downloads, entries[0].Name())
}

// sheet is the confirmation on top of the canvas, or a failure.
func (h *shareSheetHarness) sheet() *widget.PopUp {
	h.t.Helper()
	p := h.top()
	if p == nil || !p.Visible() || !(sheetHas(p, shareSheetHeading) || sheetHas(p, shareSheetImageHeading)) {
		h.t.Fatalf("the share confirmation is not on top of the canvas; texts %v", sheetTexts(p))
	}
	return p
}

// boxLabel is the clipboard box's label: the one label inside a scroll on
// the sheet. nil for a sheet with no box.
func boxLabel(p *widget.PopUp) *widget.Label {
	var found *widget.Label
	walkTree(p, func(o fyne.CanvasObject) {
		if sc, ok := o.(*container.Scroll); ok && found == nil {
			walkTree(sc.Content, func(o fyne.CanvasObject) {
				if l, ok := o.(*widget.Label); ok && found == nil {
					found = l
				}
			})
		}
	})
	return found
}

// boxText is the clipboard box's text.
func boxText(p *widget.PopUp) (string, bool) {
	l := boxLabel(p)
	if l == nil {
		return "", false
	}
	return l.Text, true
}

// wantOnCanvas fails unless the sheet lies wholly inside the canvas and
// opens below the header.
func (h *shareSheetHarness) wantOnCanvas(p *widget.PopUp) {
	h.t.Helper()
	test.WidgetRenderer(p).Layout(p.Size())
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(p.Content)
	sz := p.Content.Size()
	cs := h.canvas().Size()
	if pos.X < 0 || pos.Y < 0 || pos.X+sz.Width > cs.Width || pos.Y+sz.Height > cs.Height {
		h.t.Errorf("the sheet spans (%.0f,%.0f) %.0fx%.0f, outside the %.0fx%.0f canvas", pos.X, pos.Y, sz.Width, sz.Height, cs.Width, cs.Height)
	}
	wantClearOfHeader(h.t, h.st, h.st.window, p)
}

// expectedCitationShare is what Share with citation composes for verses 1–2
// of the reader's chapter, through the share's own pipeline.
func (h *shareSheetHarness) expectedCitationShare() string {
	text, span := h.selection(1, 2)
	quote, cite := shareQuoteIn(h.st, h.st.CurrentBook, h.st.CurrentChapter, text, span)
	return composeShareText(quote, cite, h.st.currentVersion().Name)
}

// THE WORDS ARE THE APPROVED ONES, held as literals here so that the other
// tests, which read the sheet through the same constants the sheet is built
// from, cannot follow a misspelling. Mutation: any of them retyped.
func TestTheCopiedSheetWordingIsTheApprovedOne(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		{shareSheetHeading, "Copied — ready to paste"},
		{shareLineNote, "Your note and the link are on the clipboard. Paste them into a message or email to send your note."},
		{shareLineCitation, "The verse and its citation are on the clipboard. Paste them into a message, email or document."},
		{shareLineLink, "The link is on the clipboard. Paste it into a message or email."},
		{shareLineImage, "The picture is saved in Downloads and shown in your file manager."},
		{shareLineCopiedAgain, "Copied again."},
		{shareButtonCopyAgain, "Copy again"},
		{shareButtonEmail, "Email…"},
		{shareButtonDone, "Done"},
	} {
		if c.got != c.want {
			t.Errorf("the sheet says %q, want %q", c.got, c.want)
		}
	}
}

// THE VERB'S SHAPE TELLS THE VERB. Mutation: shareVerbOf answering citation
// for everything, or link for a note.
func TestShareVerbIsToldFromTheMessage(t *testing.T) {
	link := ShareLinkURL(defaultVersionID, "John", 3, 16, 16)
	for _, c := range []struct {
		name, msg string
		verb      shareVerb
		line      string
		subject   string
	}{
		{"citation", composeShareText("“For God so loved the world.”", "John 3:16", "World English Bible"),
			shareVerbCitation, shareLineCitation, "John 3:16 (World English Bible)"},
		{"link", "John 3:16 (World English Bible)\n" + link, shareVerbLink, shareLineLink, "John 3:16 (World English Bible)"},
		{"note", "read this with me\n\nJohn 3:16 (World English Bible)\n" + link, shareVerbNote, shareLineNote, "John 3:16 (World English Bible)"},
		{"a note of several lines", "one\ntwo\n\nJohn 3:16 (World English Bible)\n" + link, shareVerbNote, shareLineNote, "John 3:16 (World English Bible)"},
		{"a heading alone", composeShareText("", "John 3:16", "World English Bible"), shareVerbCitation, shareLineCitation, "John 3:16 (World English Bible)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := shareVerbOf(c.msg); got != c.verb {
				t.Errorf("shareVerbOf = %v, want %v", got, c.verb)
			}
			d := shareDoneForText(c.msg)
			if d.line != c.line || d.text != c.msg || d.body != c.msg || d.subject != c.subject {
				t.Errorf("shareDoneForText = %+v, want line %q, subject %q, the message as text and body", d, c.line, c.subject)
			}
		})
	}
}

// EVERY TEXT VERB ENDS IN THE SHEET, showing its line and the text the
// clipboard holds, inside the canvas at both window sizes, in light and in
// dark. Mutations: the clipboard write dropped (the clipboard is empty), the
// heading or a line misspelt, the sheet not registered as modal.
func TestEveryDesktopTextShareEndsInTheCopiedSheet(t *testing.T) {
	const words = "fixture sheet alpha: read this with me"
	for _, size := range []fyne.Size{{Width: 1280, Height: 800}, {Width: 520, Height: 640}} {
		for _, variant := range []fyne.ThemeVariant{light, dark} {
			for _, verb := range []struct {
				name string
				run  func(h *shareSheetHarness)
				line string
				text func(h *shareSheetHarness, got string) bool
			}{
				{"Share with citation", (*shareSheetHarness).shareCitation, shareLineCitation,
					func(h *shareSheetHarness, got string) bool { return got == h.expectedCitationShare() }},
				{"Share as link", (*shareSheetHarness).shareLink, shareLineLink,
					func(_ *shareSheetHarness, got string) bool {
						lines := strings.Split(got, "\n")
						return len(lines) == 2 && strings.HasPrefix(lines[1], shareLinkBase+"/")
					}},
				{"Share with note", func(h *shareSheetHarness) { h.shareNote(words) }, shareLineNote,
					func(_ *shareSheetHarness, got string) bool {
						lines := strings.Split(got, "\n")
						return strings.HasPrefix(got, words+"\n\n") && strings.HasPrefix(lines[len(lines)-1], shareLinkBase+"/")
					}},
				{"Verse of the day", func(h *shareSheetHarness) { h.shareVerseOfDay() }, shareLineCitation,
					func(_ *shareSheetHarness, got string) bool { return strings.Contains(got, "\n\n— ") }},
			} {
				name := verb.name + " " + map[fyne.ThemeVariant]string{light: "light", dark: "dark"}[variant]
				t.Run(fmt.Sprintf("%s at %.0fx%.0f", name, size.Width, size.Height), func(t *testing.T) {
					h := newShareSheetHarness(t)
					h.st.window.Resize(size)
					if h.variant != variant {
						h.flip()
					}
					verb.run(h)
					p := h.sheet()
					if !sheetHas(p, shareSheetHeading) || !sheetHas(p, verb.line) {
						t.Errorf("the sheet reads %v, want the heading %q and the line %q", sheetTexts(p), shareSheetHeading, verb.line)
					}
					got, ok := boxText(p)
					if !ok || got == "" {
						t.Fatal("the sheet has no clipboard box")
					}
					if h.clipboard() != got {
						t.Errorf("the clipboard holds %q, but the box shows %q", h.clipboard(), got)
					}
					if !verb.text(h, got) {
						t.Errorf("the text is not what %s composes: %q", verb.name, got)
					}
					for _, label := range []string{shareButtonCopyAgain, shareButtonEmail, shareButtonDone} {
						if b := findTreeButton(p.Content, label); b == nil || !b.Visible() {
							t.Errorf("no visible %q button", label)
						}
					}
					h.wantOnCanvas(p)
					pal := lightPalette
					if variant == dark {
						pal = darkPalette
					}
					if c := textColour(p, shareSheetHeading); c != pal.Text {
						t.Errorf("the heading is %v, want the palette's text colour %v", c, pal.Text)
					}
					if !registered(h.st, p) {
						t.Error("the sheet holds no reopen")
					}
				})
			}
		}
	}
}

// A STRAY CLICK CANNOT LOSE IT. Mutation: NewModalPopUp swapped for NewPopUp.
func TestTheCopiedSheetSurvivesAClickOutside(t *testing.T) {
	h := newShareSheetHarness(t)
	h.shareCitation()
	p := h.sheet()
	for _, at := range []fyne.Position{{X: 4, Y: 4}, {X: 4, Y: h.canvas().Size().Height - 4}, {X: h.canvas().Size().Width - 4, Y: 4}} {
		test.TapCanvas(h.canvas(), at)
		if h.top() != p || !p.Visible() {
			t.Fatalf("a click at %v dismissed the sheet", at)
		}
	}
	if h.restored != 0 {
		t.Errorf("the canvas was given back %d times while the sheet was up", h.restored)
	}
}

// typeOnCanvas sends a key the way the desktop driver does
// (glfw's processKeyPressed): to the widget holding the caret when one does,
// and only otherwise to the canvas's own handler.
func (h *shareSheetHarness) typeOnCanvas(key fyne.KeyName) {
	h.t.Helper()
	ev := &fyne.KeyEvent{Name: key}
	if f := h.canvas().Focused(); f != nil {
		f.TypedKey(ev)
		return
	}
	typed := h.canvas().OnTypedKey()
	if typed == nil {
		h.t.Fatal("control: the desktop canvas has no key handler")
	}
	typed(ev)
}

// DONE, ESCAPE AND RETURN CLOSE IT AND GIVE THE CANVAS BACK: the overlay
// stack is empty, the sheet-close consume point ran once, the canvas's key
// handler is the one the sheet found (Escape still closes the next sheet),
// and the sheet never comes back. Return reaches the canvas's handler
// because nothing in the sheet holds the caret — not on opening, and not
// after a button has been tapped, which gives the caret up. Mutations: Done
// not hiding; the handler not wrapped for Return, or not put back; the
// close not calling showReadingOverlay.
func TestTheCopiedSheetClosesOnDoneEscapeAndReturn(t *testing.T) {
	for _, way := range []struct {
		name  string
		close func(t *testing.T, h *shareSheetHarness, p *widget.PopUp)
	}{
		{"Done", func(t *testing.T, _ *shareSheetHarness, p *widget.PopUp) {
			test.Tap(findTreeButton(p.Content, shareButtonDone))
		}},
		{"Escape", func(t *testing.T, h *shareSheetHarness, _ *widget.PopUp) {
			h.typeOnCanvas(fyne.KeyEscape)
		}},
		{"Return", func(t *testing.T, h *shareSheetHarness, _ *widget.PopUp) {
			if f := h.canvas().Focused(); f != nil {
				t.Fatalf("control: the sheet must open with nothing holding the caret; focused %T", f)
			}
			h.typeOnCanvas(fyne.KeyReturn)
		}},
		{"Return after Copy again", func(t *testing.T, h *shareSheetHarness, p *widget.PopUp) {
			test.Tap(findTreeButton(p.Content, shareButtonCopyAgain))
			if f := h.canvas().Focused(); f != nil {
				t.Fatalf("control: a tapped button gives the caret up; focused %T", f)
			}
			h.typeOnCanvas(fyne.KeyEnter)
		}},
	} {
		t.Run(way.name, func(t *testing.T) {
			h := newShareSheetHarness(t)
			h.shareLink()
			p := h.sheet()
			way.close(t, h, p)
			if p.Visible() || h.overlays() != 0 {
				t.Fatalf("after %s the sheet is still up (visible %v, overlays %d)", way.name, p.Visible(), h.overlays())
			}
			if h.restored != 1 {
				t.Errorf("the canvas was given back %d times, want once", h.restored)
			}
			if registered(h.st, p) {
				t.Error("the closed sheet still holds its reopen")
			}
			h.flip()
			if h.overlays() != 0 {
				t.Errorf("a change brought back a sheet the reader closed: %v", sheetTexts(h.top()))
			}
			// The canvas's handler is the desktop's again: Escape closes the
			// next sheet, and Return does nothing to it.
			showVerseOfDay(h.st)
			card := h.top()
			if card == nil || !sheetHas(card, "Verse of the day") {
				t.Fatal("control: the verse of the day card did not open")
			}
			h.typeOnCanvas(fyne.KeyReturn)
			if !card.Visible() {
				t.Error("Return closed the verse of the day card: the sheet's wrapper outlived it")
			}
			h.typeOnCanvas(fyne.KeyEscape)
			if card.Visible() || h.overlays() != 0 {
				t.Error("Escape no longer closes the next sheet: the handler the sheet found was not put back")
			}
		})
	}
}

// A DRAG ACROSS THE BOX LEAVES ESCAPE AND RETURN WITH THE SHEET. A reader
// checking the text runs the pointer over it; nothing there may take the
// caret, or the driver hands the next key to what took it and the sheet
// stays. Mutation guarded: the box's label made selectable (the drag
// focuses its selection, which answers no key).
func TestADragAcrossTheBoxLeavesTheKeysWithTheSheet(t *testing.T) {
	for _, key := range []fyne.KeyName{fyne.KeyEscape, fyne.KeyReturn} {
		t.Run(string(key), func(t *testing.T) {
			h := newShareSheetHarness(t)
			h.shareNote("fixture drag alpha: a note of some length to run the pointer across")
			p := h.sheet()
			test.WidgetRenderer(p).Layout(p.Size())
			l := boxLabel(p)
			if l == nil {
				t.Fatal("control: the sheet has no clipboard box")
			}
			at := fyne.CurrentApp().Driver().AbsolutePositionForObject(l).Add(fyne.NewPos(12, 12))
			if !l.Visible() || l.Size().Width < 100 {
				t.Fatalf("control: the box's text is not laid out to drag across (%v)", l.Size())
			}
			test.Drag(h.canvas(), at, 80, 0)
			if f := h.canvas().Focused(); f != nil {
				t.Errorf("a drag across the box gave the caret to %T", f)
			}
			h.typeOnCanvas(key)
			if p.Visible() || h.overlays() != 0 {
				t.Fatalf("after a drag across the box %s left the sheet up", key)
			}
		})
	}
}

// COPY AGAIN COPIES AGAIN, and says so for a moment. Mutations: the button
// not writing the clipboard; the line not changing, or not coming back.
func TestCopyAgainCopiesTheTextAgain(t *testing.T) {
	h := newShareSheetHarness(t)
	h.shareCitation()
	p := h.sheet()
	want := h.clipboard()
	fyne.CurrentApp().Clipboard().SetContent("something else took the clipboard")
	test.Tap(findTreeButton(p.Content, shareButtonCopyAgain))
	if got := h.clipboard(); got != want {
		t.Errorf("after Copy again the clipboard holds %q, want the share %q", got, want)
	}
	if !sheetHas(p, shareLineCopiedAgain) || sheetHas(p, shareLineCitation) {
		t.Errorf("the line reads %v, want %q for the moment", sheetTexts(p), shareLineCopiedAgain)
	}
	if n := len(*h.timers); n != 1 {
		t.Fatalf("Copy again armed %d timers, want one", n)
	}
	(*h.timers)[0]()
	if !sheetHas(p, shareLineCitation) || sheetHas(p, shareLineCopiedAgain) {
		t.Errorf("after the moment the line reads %v, want %q back", sheetTexts(p), shareLineCitation)
	}
}

// A LIGHT/DARK CHANGE BRINGS IT BACK WITH THE SAME TEXT, in the new palette.
// Mutation: the reopen not registered (the sheet closes for good), or
// registered without the text (a different sheet comes back).
func TestTheCopiedSheetComesBackAfterALightDarkChange(t *testing.T) {
	h := newShareSheetHarness(t)
	h.shareCitation()
	p := h.sheet()
	text, _ := boxText(p)
	for _, want := range []struct {
		v fyne.ThemeVariant
		p palette
	}{{light, lightPalette}, {dark, darkPalette}} {
		h.flip()
		again := h.sheet()
		if again == p || h.overlays() != 1 {
			t.Fatalf("the sheet must come back rebuilt, alone; overlays %d", h.overlays())
		}
		if got, _ := boxText(again); got != text || !sheetHas(again, shareLineCitation) {
			t.Errorf("the reopened sheet shows %q under %v, want the same text and line", got, sheetTexts(again))
		}
		if c := textColour(again, shareSheetHeading); c != want.p.Text {
			t.Errorf("after the change to %v the heading is %v, want %v", want.v, c, want.p.Text)
		}
		if h.clipboard() != text {
			t.Errorf("the clipboard changed across the rebuild: %q", h.clipboard())
		}
		p = again
	}
}

// OVER THE VERSE OF THE DAY it stacks on the card, Escape closes the sheet
// and not the card, Done returns to the card with its Escape restored, and a
// light/dark change brings both back, the card beneath. Mutations: the
// capture not taking the card's reopen (the card is gone after the change);
// the close not restoring the card's Escape hook; the close giving the
// canvas back while the card still owns it.
func TestTheCopiedSheetStacksOverTheVerseOfTheDay(t *testing.T) {
	t.Run("Escape closes the sheet, then the card", func(t *testing.T) {
		h := newShareSheetHarness(t)
		card := h.shareVerseOfDay()
		p := h.sheet()
		if h.overlays() != 2 || h.canvas().Overlays().List()[0] != card {
			t.Fatalf("want the sheet over the card; overlays %d", h.overlays())
		}
		h.st.dismissSheet()
		if p.Visible() || h.top() != card || !card.Visible() {
			t.Fatalf("Escape must close the sheet and leave the card; top %v", sheetTexts(h.top()))
		}
		if h.restored != 0 {
			t.Errorf("the canvas was given back %d times while the card is still up", h.restored)
		}
		h.st.dismissSheet()
		if card.Visible() || h.overlays() != 0 {
			t.Fatal("Escape must then close the card")
		}
		if h.restored != 1 {
			t.Errorf("the card's close gave the canvas back %d times, want once", h.restored)
		}
	})
	t.Run("a light/dark change brings both back", func(t *testing.T) {
		h := newShareSheetHarness(t)
		card := h.shareVerseOfDay()
		p := h.sheet()
		text, _ := boxText(p)
		h.flip()
		again := h.sheet()
		list := h.canvas().Overlays().List()
		if len(list) != 2 || again == p {
			t.Fatalf("want the rebuilt sheet over the rebuilt card; overlays %d", len(list))
		}
		under, _ := list[0].(*widget.PopUp)
		if under == nil || under == card || !sheetHas(under, "Verse of the day") {
			t.Fatalf("the card beneath must come back; beneath is %v", sheetTexts(under))
		}
		if got, _ := boxText(again); got != text {
			t.Errorf("the reopened sheet shows %q, want %q", got, text)
		}
		test.Tap(findTreeButton(again.Content, shareButtonDone))
		if h.top() != under {
			t.Fatal("Done must return to the card")
		}
		if h.st.dismissSheet == nil {
			t.Fatal("the card's Escape hook was not restored")
		}
		h.st.dismissSheet()
		if h.overlays() != 0 {
			t.Error("Escape must close the card the sheet returned to")
		}
	})
}

// ON THE NOTE PATH THE SENT NOTE IS ON THE PAGE UNDER THE SHEET: the
// composer closed, the note kept and drawn as "Note from you", and the sheet
// over it with the note and the link. Mutation: the sheet opened before the
// note is shown would still pass; showSentNote dropped fails.
func TestTheCopiedSheetOpensOverTheSentNote(t *testing.T) {
	const words = "fixture under the sheet alpha"
	h := newShareSheetHarness(t)
	h.shareNote(words)
	p := h.sheet()
	if h.overlays() != 1 {
		t.Fatalf("the composer must be closed under the sheet; overlays %d", h.overlays())
	}
	assertSentNoteOnScreen(t, h.st, words)
	if got, _ := boxText(p); !strings.HasPrefix(got, words+"\n\n") {
		t.Errorf("the box shows %q, want the note first", got)
	}
	if !sheetHas(p, shareLineNote) {
		t.Errorf("the sheet reads %v, want %q", sheetTexts(p), shareLineNote)
	}
	test.Tap(findTreeButton(p.Content, shareButtonDone))
	assertSentNoteOnScreen(t, h.st, words) // the card stays once the sheet has gone
}

// EMAIL… IS OFFERED ONLY WHEN THERE IS A MAIL CLIENT, and hands over the
// citation as the subject and the text as the body. Mutations: the button
// shown whatever the probe says; the subject or body swapped.
func TestEmailIsOfferedOnlyWithAMailClient(t *testing.T) {
	t.Run("offered", func(t *testing.T) {
		h := newShareSheetHarness(t)
		h.shareCitation()
		p := h.sheet()
		b := findTreeButton(p.Content, shareButtonEmail)
		if b == nil || !b.Visible() {
			t.Fatal("Email… must be offered when the probe says there is a client")
		}
		test.Tap(b)
		select {
		case c := <-h.composed:
			text, _ := boxText(p)
			if c.body != text || c.subject != shareSubjectOf(text, shareVerbCitation) || c.attachment != "" {
				t.Errorf("Email… composed %+v, want the citation as subject and the text as body", c)
			}
			if !strings.HasPrefix(c.subject, "John 1:1") || strings.HasPrefix(c.subject, "—") {
				t.Errorf("the subject is %q, want the citation without its dash", c.subject)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Email… composed nothing")
		}
	})
	t.Run("withheld", func(t *testing.T) {
		h := newShareSheetHarness(t)
		h.emailOK = false
		h.shareCitation()
		p := h.sheet()
		if b := findTreeButton(p.Content, shareButtonEmail); b != nil && b.Visible() {
			t.Fatal("Email… must not be offered when the probe says there is no client")
		}
		for _, label := range []string{shareButtonCopyAgain, shareButtonDone} {
			if b := findTreeButton(p.Content, label); b == nil || !b.Visible() {
				t.Errorf("no visible %q button", label)
			}
		}
	})
}

// THE IMAGE SHARE ENDS IN THE SAME SHEET: saved to Downloads and revealed as
// before, the sheet saying so with no clipboard box and no Copy again, and
// Email… attaching the file. Mutations: the reveal dropped; the sheet not
// opened for the image; the attachment not handed over.
func TestTheImageShareEndsInTheSavedSheet(t *testing.T) {
	h := newShareSheetHarness(t)
	dst := h.shareImage()
	p := h.sheet()
	if !sheetHas(p, shareSheetImageHeading) || !sheetHas(p, shareLineImage) {
		t.Errorf("the sheet reads %v, want %q and %q", sheetTexts(p), shareSheetImageHeading, shareLineImage)
	}
	if _, ok := boxText(p); ok {
		t.Error("the image sheet must show no clipboard box")
	}
	if b := findTreeButton(p.Content, shareButtonCopyAgain); b != nil {
		t.Error("the image sheet must offer no Copy again")
	}
	if len(h.revealed) != 1 || h.revealed[0] != dst {
		t.Errorf("revealed %v, want the saved file %q", h.revealed, dst)
	}
	if !strings.HasPrefix(filepath.Base(dst), "BibleText verse ") {
		t.Errorf("the saved file is %q, want a BibleText verse name", dst)
	}
	h.wantOnCanvas(p)
	test.Tap(findTreeButton(p.Content, shareButtonEmail))
	select {
	case c := <-h.composed:
		if c.attachment != dst || c.subject != "John 1:1 (Sample)" {
			t.Errorf("Email… composed %+v, want the saved file attached under the citation", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Email… composed nothing")
	}
	test.Tap(findTreeButton(p.Content, shareButtonDone))
	if h.overlays() != 0 {
		t.Error("Done must close the image sheet")
	}
}

// THE MAIL HELPERS: a subject on one line under the portal's cap, and a
// mailto: link with %20 for spaces.
func TestMailHelpers(t *testing.T) {
	if got := mailSubjectLine("  John 3:16\n(World English Bible) "); got != "John 3:16 (World English Bible)" {
		t.Errorf("mailSubjectLine = %q", got)
	}
	long := strings.Repeat("x", mailSubjectMaxRunes+40)
	if got := mailSubjectLine(long); len([]rune(got)) != mailSubjectMaxRunes {
		t.Errorf("mailSubjectLine left %d runes, want %d", len([]rune(got)), mailSubjectMaxRunes)
	}
	u := mailtoURL("John 3:16 (WEB)", "line one\nline two & more")
	if got := u.String(); got != "mailto:?subject=John%203%3A16%20%28WEB%29&body=line%20one%0Aline%20two%20%26%20more" {
		t.Errorf("mailtoURL = %q", got)
	}
}
