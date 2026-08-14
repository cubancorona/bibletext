package bibletext

// A shared link that names a translation this reader has not got.
//
// Before /nkjv/ was a link path this could not happen: every id a URL could
// carry was public-domain, and the code said so in as many words. The link now
// carries the sender's real translation, so it can, and the owner's instruction
// for that case was explicit — "If they don't have licensing they should get a
// message". These tests pin BOTH halves of the answer: the message goes up, and
// the passage still opens.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// lockedNKJVState is a reader in the WEB with no NKJV entitlement anywhere: no
// operator env, no bundled key, no key of their own. That is the ordinary state
// of every dev build and of any reader who cleared their key in Settings.
func lockedNKJVState(t *testing.T) *AppState {
	t.Helper()
	t.Setenv("BIBLE_API_KEY", "")
	t.Setenv("BIBLETEXT_LICENSE_NKJV", "")
	t.Setenv("BIBLETEXT_PROVIDER_ID_NKJV", "")
	// Off deliberately: BIBLETEXT_ENABLE_TESTING makes canSelect() true for a
	// locked version (internal QA of the placeholder flow), and with it set the
	// message would correctly NOT appear — which would make this test pass for
	// the wrong reason.
	t.Setenv("BIBLETEXT_ENABLE_TESTING", "")
	withFakeSharedKeys(t)

	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("nkjv not registered")
	}
	if v.canSelect() {
		t.Fatal("precondition: nkjv must be locked with no key anywhere")
	}

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible:          bd,
		CurrentBook:    "Genesis",
		CurrentChapter: 1,
		CurrentVersion: "web",
		loadPhase:      loadReady,
	}
}

// linkVersionUnavailable is the whole decision, and only one of its answers is
// a message. Getting the others wrong is how a link starts nagging about
// something the reader neither asked about nor can act on.
func TestLinkVersionUnavailableOnlySpeaksForALockedTranslation(t *testing.T) {
	st := lockedNKJVState(t)

	if got := linkVersionUnavailable(st, ShareTarget{VersionID: "nkjv", Book: "John", Chapter: 3}); got != "New King James Version" {
		t.Errorf("a locked translation must be named; got %q", got)
	}
	// An id from a future BibleText: "your app is too old" is not something we
	// can say accurately, so it degrades quietly (see
	// TestUnknownTranslationLinkStillOpens).
	if got := linkVersionUnavailable(st, ShareTarget{VersionID: "nosuchversion"}); got != "" {
		t.Errorf("an unknown id must stay silent; got %q", got)
	}
	// The translation the reader is already in, and a public-domain one they can
	// simply be switched to: nothing happened worth telling them about.
	if got := linkVersionUnavailable(st, ShareTarget{VersionID: "web"}); got != "" {
		t.Errorf("the current translation must stay silent; got %q", got)
	}
	if got := linkVersionUnavailable(st, ShareTarget{VersionID: "bsb"}); got != "" {
		t.Errorf("an available translation must stay silent; got %q", got)
	}
	if got := linkVersionUnavailable(nil, ShareTarget{VersionID: "nkjv"}); got != "" {
		t.Errorf("no state, nothing to say; got %q", got)
	}

	// Unlocked by the reader's own API.Bible key, the way BYOK actually works:
	// the message must stop, because the switch will now happen.
	fake := withFakeSharedKeys(t)
	fake.setBibleAPIKey("readers-own-key")
	if got := linkVersionUnavailable(st, ShareTarget{VersionID: "nkjv"}); got != "" {
		t.Errorf("an unlocked nkjv must stay silent; got %q", got)
	}
}

// The core owner requirement, end to end: the reader is TOLD, and is not
// silently moved. Before this, applyShareTarget navigated in whatever
// translation they happened to be in with nothing on screen to say so — and
// since verse numbering is not interchangeable (Romans 14/16), that can be
// different text from the one the sender pointed at.
func TestLockedTranslationLinkOpensThePassageAndSaysSo(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	win := app.NewWindow("nkjv link")
	win.Resize(fyne.NewSize(402, 812))

	st := lockedNKJVState(t)
	st.window, st.theme = win, th

	applyShareTarget(st, ShareTarget{VersionID: "nkjv", Book: "John", Chapter: 3, VerseLo: 16})

	// The passage opens. Withholding scripture over a licence would be a worse
	// answer than any wording — the verse is not the licensed part.
	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("the passage did not open: %s %d", st.CurrentBook, st.CurrentChapter)
	}
	if !st.HasHighlightedVerse || st.HighlightedVerse != 16 {
		t.Error("the shared verse was not highlighted")
	}
	// And nothing pretended the switch happened.
	if st.CurrentVersion != "web" {
		t.Errorf("the reader was moved to %q; a locked translation cannot load", st.CurrentVersion)
	}
	if st.pendingLink != nil || st.pendingLinkVersion != "" {
		t.Error("parked on a translation that can never arrive")
	}

	pop, _ := win.Canvas().Overlays().Top().(*widget.PopUp)
	if pop == nil {
		t.Fatal("no message — this is the silent downgrade the owner asked to replace")
	}
	said := popupText(pop)
	if !strings.Contains(said, "New King James Version") {
		t.Errorf("the message must name the translation the link asked for:\n%s", said)
	}
	if !strings.Contains(said, "World English Bible") {
		t.Errorf("the message must name the translation they are actually reading:\n%s", said)
	}
}

// A translation the reader CAN open must not raise the message: it just
// switches. Same link shape, opposite outcome, so a future edit cannot satisfy
// the test above by showing the card unconditionally.
func TestAvailableTranslationLinkSaysNothing(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	win := app.NewWindow("web link")
	win.Resize(fyne.NewSize(402, 812))

	st := lockedNKJVState(t)
	st.window, st.theme = win, th

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16})

	if top := win.Canvas().Overlays().Top(); top != nil {
		t.Errorf("a link in the reader's own translation put something on screen: %T", top)
	}
}

// A note arriving on an /nkjv/ link is filed under nkjv — including for the
// reader who cannot open the NKJV. Filing it under whatever they happen to be
// reading would attach somebody's remark to wording it was never about; filed
// correctly, it is simply waiting under nkjv if they ever unlock it.
func TestNoteOnALockedTranslationLinkIsFiledUnderThatTranslation(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	win := app.NewWindow("nkjv note")
	win.Resize(fyne.NewSize(402, 812))

	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	t.Cleanup(func() { deleteAllNotes(appPrefs()) })

	st := lockedNKJVState(t)
	st.window, st.theme = win, th

	url := "https://bibletext.co.uk/nkjv/john/3/#v16&n=" + EncodeNote("this verse carried me")
	if !HandleShareLink(st, url) {
		t.Fatal("an /nkjv/ link must be recognised as one of ours")
	}
	if st.ActiveNote != "this verse carried me" {
		t.Errorf("the sender's note was not surfaced: %q", st.ActiveNote)
	}
	if _, ok := readNotes(appPrefs())[noteKey("nkjv", "John", 3)]; !ok {
		t.Error("the note was not filed under nkjv, the translation it was written in")
	}
	if _, wrong := readNotes(appPrefs())[noteKey("web", "John", 3)]; wrong {
		t.Error("the note was filed under the reader's translation instead of the link's")
	}
}

// popupText walks a popup's widget tree and collects every label and text
// object, so a test can assert what the reader was actually told rather than
// that some card appeared.
func popupText(pop *widget.PopUp) string {
	var out []string
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Label:
			out = append(out, v.Text)
		case *widget.Button:
			out = append(out, v.Text)
		case *canvas.Text: // the card's title and subtitle are drawn, not widgets
			out = append(out, v.Text)
		}
		if c, ok := o.(*fyne.Container); ok {
			for _, ch := range c.Objects {
				walk(ch)
			}
			return
		}
		if wd, ok := o.(fyne.Widget); ok {
			for _, ch := range test.WidgetRenderer(wd).Objects() {
				walk(ch)
			}
		}
	}
	walk(pop.Content)
	return strings.Join(out, "\n")
}
