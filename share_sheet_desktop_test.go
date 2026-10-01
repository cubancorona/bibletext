//go:build !ios && !android

package bibletext

// THE DESKTOP SHARE CONFIRMATION (share_sheet_desktop.go), driven as a
// Linux reader drives it, or a Windows reader whose Share sheet cannot open:
// a real window holding CreateMainUI's tree with the styled reading pane, the
// share verbs delivering through the desktop fallback exactly as
// share_other.go routes Linux there, and every
// verb run from its own entry point — the selection menu's actions, the
// composer's Share, the verse-of-the-day card's icon, the image share's
// hand-off. What each holds: the sheet is on the overlay stack when the
// handler returns, inside the canvas and below the header at 1280x800 and
// at 520x640, in light and in dark; its words are the approved ones; the
// clipboard holds the text the box shows; a click outside does not dismiss
// it; Done, Escape and Return close it and give the canvas back, after Tab
// has put the caret on a button too; Copy again copies again; neither Copy
// again nor Email…'s late arrival moves a button; a light/dark change brings
// it back showing the same text, and the verse-of-the-day card beneath it;
// on the note path the sent note's card is on the page under it; Email… is
// offered only when the platform says there is a mail client — one that
// takes a file, for the image — and hands over the citation, the text, and
// the image's file with its quote.

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
// reveal is recorded instead of run and answers at once with revealShown —
// the sheet's timers held, and a home directory of the test's own
// (redirectHome), with no Downloads folder until homeForImage makes one, so
// no image share reaches the machine's.
type shareSheetHarness struct {
	*appearanceHarness
	st          *AppState
	emailOK     bool
	revealShown bool   // what the file manager answers to a reveal
	probed      []bool // withAttachment, as each sheet asked the mail probe
	composed    chan shareCompose
	revealed    []string
	restored    int // calls to showReadingOverlay, the desktop sheet-close consume point
	timers      *[]func()
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
		revealShown:       true,
		composed:          make(chan shareCompose, 8),
	}
	h.st = h.state
	redirectHome(t)
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
	shareEmailProbe = func(withAttachment bool, report func(bool)) {
		h.probed = append(h.probed, withAttachment)
		report(h.emailOK)
	}
	t.Cleanup(func() { shareEmailProbe = prevProbe })
	prevCompose := shareEmailCompose
	shareEmailCompose = func(subject, body, attachment string) error {
		h.composed <- shareCompose{subject, body, attachment}
		return nil
	}
	t.Cleanup(func() { shareEmailCompose = prevCompose })
	prevReveal := revealInFileManager
	revealInFileManager = func(p string, report func(bool)) {
		h.revealed = append(h.revealed, p)
		report(h.revealShown)
	}
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

// The image mail the harness's image share carries (shareImageMail, which
// the preview sets before its hand-off).
const (
	sampleImageSubject = "John 1:1 (Sample)"
	sampleImageBody    = "“In the beginning was the Word.”\n\n— John 1:1 (Sample)"
)

// homeForImage gives the test a home of its own, with a Downloads folder or
// without one, and the image mail the preview would have set.
func (h *shareSheetHarness) homeForImage(withDownloads bool) (downloads string) {
	h.t.Helper()
	home := redirectHome(h.t)
	downloads = filepath.Join(home, "Downloads")
	if withDownloads {
		if err := os.MkdirAll(downloads, 0o755); err != nil {
			h.t.Fatal(err)
		}
	}
	prev := shareImageMail
	shareImageMail.subject, shareImageMail.body = sampleImageSubject, sampleImageBody
	h.t.Cleanup(func() { shareImageMail = prev })
	return downloads
}

// renderedCard is a card file where the renderer would leave one.
func (h *shareSheetHarness) renderedCard() string {
	h.t.Helper()
	src := filepath.Join(h.t.TempDir(), "card.png")
	if err := os.WriteFile(src, []byte("not a real png"), 0o644); err != nil {
		h.t.Fatal(err)
	}
	return src
}

// shareImage hands a rendered card to the image fallback with a Downloads
// folder under a home of its own, and returns where the fallback put it.
func (h *shareSheetHarness) shareImage() string {
	h.t.Helper()
	downloads := h.homeForImage(true)
	fallbackShareImage(h.renderedCard())
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
// after a button has been tapped, which gives the caret up. The handler is
// checked before any light/dark change, whose rebuild installs the canvas's
// handler afresh and would hide one the close failed to put back.
// Mutations: Done not hiding; the handler not wrapped for Return, or not put
// back; the close not calling showReadingOverlay.
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
			found := reflect.ValueOf(h.canvas().OnTypedKey()).Pointer()
			h.shareLink()
			p := h.sheet()
			if reflect.ValueOf(h.canvas().OnTypedKey()).Pointer() == found {
				t.Fatal("control: the sheet must wrap the canvas's key handler while it shows")
			}
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
			if reflect.ValueOf(h.canvas().OnTypedKey()).Pointer() != found {
				t.Error("the canvas's key handler is not the one the sheet found")
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
			h.flip()
			if h.overlays() != 0 {
				t.Errorf("a change brought back a sheet the reader closed: %v", sheetTexts(h.top()))
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

// A LIGHT/DARK CHANGE BRINGS IT BACK WITH THE SAME TEXT, in the new palette,
// and the image sheet with the same picture: its heading and line, no box,
// and Email… still attaching the saved file with its subject and text.
// Mutation: the reopen not registered (the sheet closes for good), or
// registered without the text or the picture (a different sheet comes
// back).
func TestTheCopiedSheetComesBackAfterALightDarkChange(t *testing.T) {
	t.Run("text", testTheTextSheetComesBack)
	t.Run("image", func(t *testing.T) {
		h := newShareSheetHarness(t)
		dst := h.shareImage()
		p := h.sheet()
		for range 2 {
			h.flip()
			again := h.sheet()
			if again == p || h.overlays() != 1 {
				t.Fatalf("the image sheet must come back rebuilt, alone; overlays %d", h.overlays())
			}
			if !sheetHas(again, shareSheetImageHeading) || !sheetHas(again, shareLineImage) {
				t.Errorf("the reopened image sheet reads %v", sheetTexts(again))
			}
			if _, ok := boxText(again); ok {
				t.Error("the reopened image sheet has a clipboard box")
			}
			b := findTreeButton(again.Content, shareButtonEmail)
			if b == nil || !b.Visible() {
				t.Fatal("the reopened image sheet has no Email…")
			}
			test.Tap(b)
			select {
			case c := <-h.composed:
				if c != (shareCompose{sampleImageSubject, sampleImageBody, dst}) {
					t.Errorf("the reopened sheet's Email… composed %+v, want the saved file %q with its subject and text", c, dst)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("the reopened sheet's Email… composed nothing")
			}
			p = again
		}
		if len(h.revealed) != 1 {
			t.Errorf("the reopen revealed the file again: %v", h.revealed)
		}
	})
}

func testTheTextSheetComesBack(t *testing.T) {
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
// Email… attaching the file, with the quote and its citation as the mail's
// text so that no route that drops the picture sends a blank message.
// Mutations: the reveal dropped; the sheet not opened for the image; the
// attachment or the text not handed over.
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
		if c != (shareCompose{sampleImageSubject, sampleImageBody, dst}) {
			t.Errorf("Email… composed %+v, want the saved file attached under the citation, with the quote as its text", c)
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
// mailto: link with %20 for spaces and CRLF for a line break (RFC 6068,
// section 5), a CRLF already there not doubled. Mutation: the body's line
// breaks left as LF.
func TestMailHelpers(t *testing.T) {
	if got := mailSubjectLine("  John 3:16\n(World English Bible) "); got != "John 3:16 (World English Bible)" {
		t.Errorf("mailSubjectLine = %q", got)
	}
	long := strings.Repeat("x", mailSubjectMaxRunes+40)
	if got := mailSubjectLine(long); len([]rune(got)) != mailSubjectMaxRunes {
		t.Errorf("mailSubjectLine left %d runes, want %d", len([]rune(got)), mailSubjectMaxRunes)
	}
	for _, body := range []string{"line one\nline two & more", "line one\r\nline two & more"} {
		u := mailtoURL("John 3:16 (WEB)", body)
		if got := u.String(); got != "mailto:?subject=John%203%3A16%20%28WEB%29&body=line%20one%0D%0Aline%20two%20%26%20more" {
			t.Errorf("mailtoURL for %q = %q", body, got)
		}
	}
}

// AFTER TAB, ESCAPE AND RETURN STILL CLOSE IT, and Space presses the button
// the caret is on. Tab is the one way the caret reaches the sheet, and the
// desktop driver then hands every key to the button holding it, never to
// the canvas's handler. Each button in turn, each key: Escape, Return and
// Enter close the sheet and give the canvas back once; Space does what a
// press of that button does and leaves the sheet up, but Done's, which
// closes it. Mutation: the buttons' own key handling dropped (a plain
// widget.Button answers Space alone, so Escape and Return then do nothing).
func TestAfterTabTheKeysStillCloseTheSheet(t *testing.T) {
	order := []string{shareButtonEmail, shareButtonCopyAgain, shareButtonDone}
	for i, label := range order {
		for _, key := range []fyne.KeyName{fyne.KeyEscape, fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace} {
			t.Run(fmt.Sprintf("%s then %s", label, key), func(t *testing.T) {
				h := newShareSheetHarness(t)
				h.shareCitation()
				p := h.sheet()
				for range i + 1 {
					h.canvas().FocusNext()
				}
				f := h.canvas().Focused()
				if f == nil {
					t.Fatalf("control: Tab put the caret nowhere")
				}
				if b := asTreeButton(f.(fyne.CanvasObject)); b == nil || b.Text != label {
					t.Fatalf("control: after %d Tabs the caret is on %v, want %q", i+1, sheetTexts(p), label)
				}
				fyne.CurrentApp().Clipboard().SetContent("something else took the clipboard")
				h.typeOnCanvas(key)
				closes := key != fyne.KeySpace || label == shareButtonDone
				if closes {
					if p.Visible() || h.overlays() != 0 {
						t.Fatalf("%s on %s left the sheet up", key, label)
					}
					if h.restored != 1 {
						t.Errorf("the canvas was given back %d times, want once", h.restored)
					}
					return
				}
				if !p.Visible() || h.top() != p {
					t.Fatalf("Space on %s closed the sheet", label)
				}
				switch label {
				case shareButtonEmail:
					select {
					case <-h.composed:
					case <-time.After(5 * time.Second):
						t.Error("Space on Email… composed nothing")
					}
				case shareButtonCopyAgain:
					if want, _ := boxText(p); h.clipboard() != want {
						t.Errorf("Space on Copy again left %q on the clipboard", h.clipboard())
					}
				}
			})
		}
	}
}

// findTreeButtonObject is the button labelled label under o as the canvas
// holds it — the widget itself, where findTreeButton gives the button a
// widget embeds — for asking the driver where it is.
func findTreeButtonObject(o fyne.CanvasObject, label string) fyne.CanvasObject {
	var found fyne.CanvasObject
	walkTree(o, func(n fyne.CanvasObject) {
		if b := asTreeButton(n); b != nil && found == nil && b.Text == label {
			found = n
		}
	})
	return found
}

// settledSheet lays the sheet out the way a painting canvas does before each
// frame (paintedFrames), and returns where each named button, and the box,
// sit on the canvas afterwards.
func settledSheet(t *testing.T, frames *paintedFrames, p *widget.PopUp, labels ...string) map[string]fyne.Position {
	t.Helper()
	frames.frame()
	d := fyne.CurrentApp().Driver()
	at := map[string]fyne.Position{}
	for _, label := range labels {
		b := findTreeButtonObject(p.Content, label)
		if b == nil || !b.Visible() {
			t.Fatalf("control: no visible %q to place", label)
		}
		at[label] = d.AbsolutePositionForObject(b)
	}
	if l := boxLabel(p); l != nil {
		at["box"] = d.AbsolutePositionForObject(l)
	}
	return at
}

// EMAIL… ARRIVES WITHOUT MOVING ANYTHING. It shows when the mail probe
// answers, a moment after the sheet, and it takes the row's left end, so
// Copy again and Done stay where the reader's pointer is heading: a tap
// there after the arrival still copies, and composes nothing. Mutation:
// Email… between Copy again and Done, as it was (it pushed Copy again left
// by its own width and took its place).
func TestEmailArrivingMovesNothing(t *testing.T) {
	h := newShareSheetHarness(t)
	var report func(bool)
	shareEmailProbe = func(_ bool, r func(bool)) { report = r }
	h.shareCitation()
	p := h.sheet()
	if report == nil {
		t.Fatal("control: the sheet never asked the mail probe")
	}
	frames := newPaintedFrames(newLayoutWalker(t), p)
	before := settledSheet(t, frames, p, shareButtonCopyAgain, shareButtonDone)
	copyAgain := findTreeButtonObject(p.Content, shareButtonCopyAgain)
	aim := before[shareButtonCopyAgain].Add(fyne.NewPos(copyAgain.Size().Width/2, copyAgain.Size().Height/2))

	report(true)
	after := settledSheet(t, frames, p, shareButtonEmail, shareButtonCopyAgain, shareButtonDone)
	for _, label := range []string{shareButtonCopyAgain, shareButtonDone} {
		if after[label] != before[label] {
			t.Errorf("Email…'s arrival moved %s from %v to %v", label, before[label], after[label])
		}
	}
	if after[shareButtonEmail].X >= after[shareButtonCopyAgain].X {
		t.Errorf("Email… at %v is not to the left of Copy again at %v", after[shareButtonEmail], after[shareButtonCopyAgain])
	}
	fyne.CurrentApp().Clipboard().SetContent("something else took the clipboard")
	test.TapCanvas(h.canvas(), aim)
	if want, _ := boxText(p); h.clipboard() != want {
		t.Errorf("a tap where Copy again was did not copy: the clipboard holds %q", h.clipboard())
	}
	if len(h.composed) != 0 {
		t.Errorf("a tap where Copy again was composed a mail: %+v", <-h.composed)
	}
}

// COPY AGAIN MOVES NOTHING. "Copied again." takes one line where the verb's
// line takes two at the sheet's width; the line's slot keeps the verb's
// height, so the box and the buttons stay put while it shows and when the
// verb's line comes back, under a pointer on its way to Done or to a second
// Copy again. The control holds that the verb's line does wrap past
// "Copied again." here, so the test can see a slot that shrank. Mutation:
// the line laid out in the form without its held slot.
func TestCopyAgainMovesNothing(t *testing.T) {
	for _, size := range []fyne.Size{{Width: 1280, Height: 800}, {Width: 520, Height: 640}} {
		t.Run(fmt.Sprintf("%.0fx%.0f", size.Width, size.Height), func(t *testing.T) {
			h := newShareSheetHarness(t)
			h.st.window.Resize(size)
			h.shareCitation()
			p := h.sheet()
			frames := newPaintedFrames(newLayoutWalker(t), p)
			labels := []string{shareButtonEmail, shareButtonCopyAgain, shareButtonDone}
			before := settledSheet(t, frames, p, labels...)

			var line *widget.Label
			walkTree(p, func(o fyne.CanvasObject) {
				if l, ok := o.(*widget.Label); ok && line == nil && l.Text == shareLineCitation && l.Visible() {
					line = l
				}
			})
			if line == nil {
				t.Fatal("control: the verb's line is not on the sheet")
			}
			one := widget.NewLabel(shareLineCopiedAgain)
			one.Wrapping = fyne.TextWrapWord
			one.Resize(fyne.NewSize(line.Size().Width, 0))
			if one.MinSize().Height >= line.Size().Height {
				t.Fatalf("control: the verb's line (%.0f high) does not wrap past %q (%.0f): nothing could move",
					line.Size().Height, shareLineCopiedAgain, one.MinSize().Height)
			}

			test.Tap(findTreeButton(p.Content, shareButtonCopyAgain))
			if !sheetHas(p, shareLineCopiedAgain) {
				t.Fatalf("control: the line reads %v, want %q", sheetTexts(p), shareLineCopiedAgain)
			}
			during := settledSheet(t, frames, p, labels...)
			(*h.timers)[0]()
			if !sheetHas(p, shareLineCitation) {
				t.Fatalf("control: the verb's line did not come back: %v", sheetTexts(p))
			}
			after := settledSheet(t, frames, p, labels...)
			for _, what := range append(labels, "box") {
				if during[what] != before[what] || after[what] != before[what] {
					t.Errorf("%s moved: %v, then %v while the line read %q, then %v", what, before[what], during[what], shareLineCopiedAgain, after[what])
				}
			}
		})
	}
}

// EMAIL… IS ASKED FOR WITH THE FILE ONLY FOR THE PICTURE: the text sheets
// ask the platform for a mail client, the image sheet for one that takes a
// file, which a mailto: link never carries (Windows, the macOS mimic) and a
// browser handling mailto: drops (Linux). Mutation: the sheet asking without
// the file for the image (Email… then shows where pressing it does nothing).
func TestEmailIsAskedForWithTheFileOnlyForThePicture(t *testing.T) {
	for _, c := range []struct {
		name  string
		share func(h *shareSheetHarness)
		want  bool
	}{
		{"citation", (*shareSheetHarness).shareCitation, false},
		{"link", (*shareSheetHarness).shareLink, false},
		{"note", func(h *shareSheetHarness) { h.shareNote("fixture probe alpha") }, false},
		{"verse of the day", func(h *shareSheetHarness) { h.shareVerseOfDay() }, false},
		{"image", func(h *shareSheetHarness) { h.shareImage() }, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newShareSheetHarness(t)
			c.share(h)
			h.sheet()
			if len(h.probed) != 1 || h.probed[0] != c.want {
				t.Errorf("the sheet asked the mail probe %v, want [%v]", h.probed, c.want)
			}
		})
	}
}

// THE PREVIEW'S SHARE CARRIES THE CITATION AND THE QUOTE TO THE MAIL. Share
// as image from the selection menu, through the preview's own Share button
// into the desktop fallback: Email… composes with the citation naming the
// translation in full as the subject, the quote and its citation — what
// Share with citation copies — as the text, and the saved file. Mutations:
// the subject set without the translation; the text left empty.
func TestTheImagePreviewHandsItsCitationToTheMail(t *testing.T) {
	h := newShareSheetHarness(t)
	downloads := h.homeForImage(true)
	shareImageMail.subject, shareImageMail.body = "", ""
	prevOut := shareImageOut
	shareImageOut = fallbackShareImage
	t.Cleanup(func() { shareImageOut = prevOut })

	text, span := h.selection(1, 2)
	dispatchSelectionAction(h.st, selActionShareImage, text, span)
	preview := h.top()
	if preview == nil || !sheetHas(preview, "Share as image") {
		t.Fatalf("control: the preview did not open; top %v", sheetTexts(preview))
	}
	test.Tap(findTreeButton(preview.Content, "Share"))
	p := h.sheet()
	if !sheetHas(p, shareSheetImageHeading) {
		t.Fatalf("the preview's Share did not end in the image sheet: %v", sheetTexts(p))
	}
	entries, _ := os.ReadDir(downloads)
	if len(entries) != 1 {
		t.Fatalf("control: want one file in Downloads, have %d", len(entries))
	}
	test.Tap(findTreeButton(p.Content, shareButtonEmail))
	select {
	case c := <-h.composed:
		_, cite := shareQuoteIn(h.st, h.st.CurrentBook, h.st.CurrentChapter, text, span)
		want := shareCompose{cite + " (" + h.st.currentVersion().Name + ")", h.expectedCitationShare(), filepath.Join(downloads, entries[0].Name())}
		if cite == "" || h.st.currentVersion().Name == "" || want.body == "" {
			t.Fatalf("control: nothing to compare against (%+v)", want)
		}
		if c != want {
			t.Errorf("Email… composed %+v, want %+v", c, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Email… composed nothing")
	}
}

// A PICTURE SHARE'S FALLBACK SAVES THE CARD THAT WAS SHARED. On Windows the
// in-app sheet can open seconds after the tap, when the Share sheet has not
// asked for the share in time (share_windows.go), and a preview opened in
// between renders its own card over the renderer's file and sets
// shareImageMail for it. The share takes both at the tap (takeSharedImage):
// its fallback saves the first card's bytes into Downloads and mails them
// under the first card's citation, which is also the title the Share sheet
// is given — or the product's name where the preview set none. Mutations:
// the fallback handed the renderer's file (the second card is saved), the
// mail not put back (the second citation is composed).
func TestALateImageFallbackSavesTheCardThatWasShared(t *testing.T) {
	h := newShareSheetHarness(t)
	downloads := h.homeForImage(true)
	renders := t.TempDir()
	prevDir := imageRenderDir
	imageRenderDir = func() string { return renders }
	t.Cleanup(func() { imageRenderDir = prevDir })
	card := filepath.Join(renders, "bibletext-verse-0.png")
	if err := os.WriteFile(card, []byte("the first card"), 0o644); err != nil {
		t.Fatal(err)
	}

	pic := takeSharedImage(card, time.Now())
	if pic.err != nil || pic.file == card {
		t.Fatalf("control: the share took %q (%v), want a copy of the card", pic.file, pic.err)
	}
	if got := pic.title(); got != sampleImageSubject {
		t.Errorf("the Share sheet's title is %q, want the citation %q", got, sampleImageSubject)
	}

	// The next preview, before the fallback opens.
	if err := os.WriteFile(card, []byte("the second card"), 0o644); err != nil {
		t.Fatal(err)
	}
	shareImageMail = shareMail{"Romans 8:28 (Sample)", "“All things work together for good.”\n\n— Romans 8:28 (Sample)"}

	pic.fallback()
	entries, _ := os.ReadDir(downloads)
	if len(entries) != 1 {
		t.Fatalf("control: want one file in Downloads, have %d", len(entries))
	}
	saved := filepath.Join(downloads, entries[0].Name())
	if b, err := os.ReadFile(saved); err != nil || string(b) != "the first card" {
		t.Errorf("the fallback saved %q (%v), want the card that was shared", b, err)
	}
	p := h.sheet()
	test.Tap(findTreeButton(p.Content, shareButtonEmail))
	select {
	case c := <-h.composed:
		if c != (shareCompose{sampleImageSubject, sampleImageBody, saved}) {
			t.Errorf("Email… composed %+v, want the shared card's citation and quote, with the saved file", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Email… composed nothing")
	}

	shareImageMail = shareMail{}
	if got := takeSharedImage(card, time.Now()).title(); got != ProductName() {
		t.Errorf("with no citation the Share sheet's title is %q, want %q", got, ProductName())
	}
}

// A picture shared from the preview goes nowhere: its Share, tapped outside
// the share sheet tests, reaches the suite's recorder with the card rendered
// in the suite's own directory, nothing lands in a Downloads folder, and no
// file manager is asked to show one. The home and the file manager are the
// test's own as well, so that were the recorder ever gone, the fallback would
// save into this home, not the machine's, and open no window. The control
// puts an out that records nothing in its place and shows the check fires.
func TestNoTestSavesASharedPictureIntoTheMachinesDownloads(t *testing.T) {
	downloads := filepath.Join(redirectHome(t), "Downloads")
	if err := os.MkdirAll(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	var revealed []string
	prevReveal, prevMail := revealInFileManager, shareImageMail
	revealInFileManager = func(p string, report func(bool)) {
		revealed = append(revealed, p)
		report(true)
	}
	t.Cleanup(func() { revealInFileManager, shareImageMail = prevReveal, prevMail })
	h := newAppearanceHarness(t, false)
	share := func() []string {
		imageSharesInTests() // drain anything an earlier test recorded
		showShareImagePreview(h.state, "For God so loved the world", "John 3:16", "WEB")
		p := h.top()
		if p == nil || !sheetHas(p, "Share as image") {
			t.Fatal("control: the preview did not open")
		}
		b := findTreeButton(p.Content, "Share")
		if b == nil {
			t.Fatal("control: the preview has no Share button")
		}
		test.Tap(b)
		return imageSharesInTests()
	}

	got := share()
	if len(got) != 1 || filepath.Dir(got[0]) != filepath.Clean(imageRenderDir()) {
		t.Errorf("the preview's Share reached %v, want the suite's recorder with one card rendered in %s", got, imageRenderDir())
	}
	if entries, _ := os.ReadDir(downloads); len(entries) > 0 {
		t.Errorf("the preview's Share saved %d file(s) into Downloads, first %s", len(entries), entries[0].Name())
	}
	if len(revealed) > 0 {
		t.Errorf("the preview's Share asked the file manager to show %v", revealed)
	}
	if t.Failed() {
		return
	}

	prevOut := shareImageOut
	shareImageOut = func(string) {}
	t.Cleanup(func() { shareImageOut = prevOut })
	if got := share(); len(got) != 0 {
		t.Fatalf("control: with an out that records nothing the check still saw %v, so its pass above proves nothing", got)
	}
}

// WITHOUT A DOWNLOADS FOLDER THE SHEET SAYS ONLY WHAT IS TRUE, under
// Windows' rules, which keep the copy they were handed (Linux saves in the
// home instead: TestOutsideASnapWithNoDownloadsThePictureIsSavedInTheHome):
// the file manager opens on the temp copy, and the line says the picture is
// shown there, not that it was saved in Downloads. Mutation: the saved line
// shown whatever the copy did.
func TestTheImageSheetWithoutDownloadsSaysOnlyWhereItIsShown(t *testing.T) {
	h := newShareSheetHarness(t)
	prevGOOS := shareImageGOOS
	shareImageGOOS = "windows"
	t.Cleanup(func() { shareImageGOOS = prevGOOS })
	downloads := h.homeForImage(false)
	src := h.renderedCard()
	fallbackShareImage(src)
	p := h.sheet()
	if !sheetHas(p, shareLineImageShown) || sheetHas(p, shareLineImage) {
		t.Errorf("the sheet reads %v, want %q and not %q", sheetTexts(p), shareLineImageShown, shareLineImage)
	}
	if len(h.revealed) != 1 || h.revealed[0] != src {
		t.Errorf("revealed %v, want the temp copy %q", h.revealed, src)
	}
	if _, err := os.Stat(downloads); !os.IsNotExist(err) {
		t.Errorf("control: the share made a Downloads folder (%v)", err)
	}
	test.Tap(findTreeButton(p.Content, shareButtonEmail))
	select {
	case c := <-h.composed:
		if c.attachment != src {
			t.Errorf("Email… attached %q, want the temp copy %q", c.attachment, src)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Email… composed nothing")
	}
}
