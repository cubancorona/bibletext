package bibletext

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// The OS puts a launched URL on the command line; the intake must take the
// first one that is ours, wherever it sits, and nothing else.
func TestStartupShareLinkTakesTheFirstSiteURL(t *testing.T) {
	args := []string{"BibleText.exe", "-x",
		"https://bibletext.co.uk/web/john/3/#v16",
		"https://bibletext.co.uk/bsb/psalms/23/"}
	got, ok := startupShareLink(args)
	if !ok || got != "https://bibletext.co.uk/web/john/3/#v16" {
		t.Fatalf("startupShareLink = %q, %v; want the first site URL", got, ok)
	}
}

func TestStartupShareLinkIgnoresForeignArguments(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"BibleText.exe"},
		{"BibleText.exe", "https://example.com/web/john/3/"},
		{"BibleText.exe", "bibletext.co.uk/web/john/3/"}, // no scheme: not from the OS
		{"BibleText.exe", "https://bibletext.co.uk.example.com/web/john/3/"},
		{"BibleText.exe", "https://notbibletext.co.uk/web/john/3/"},
		{"BibleText.exe", "https://bibletext.co.uk@evil.example/web/john/3/"}, // userinfo spoof
		{"BibleText.exe", "https://evil.example/?u=bibletext.co.uk/web/john/3/"},
		{"BibleText.exe", "https://bibletext.co.uk:8443/web/john/3/"}, // not the site's port
		{"BibleText.exe", "https://[::1]/web/john/3/"},
		{"BibleText.exe", "ftp://bibletext.co.uk/web/john/3/"},
		{"BibleText.exe", "B\u0130BLETEXT://bibletext.co.uk/web/john/3/"}, // a non-ASCII scheme letter
		{"BibleText.exe", "https://bibletext.co.uk/web/john/3/#v16&n=" + strings.Repeat("a", maxStartupLinkLen)},
	} {
		if got, ok := startupShareLink(args); ok {
			t.Errorf("startupShareLink(%q) = %q, want nothing", args, got)
		}
	}
	// The control: a real link is taken, or the refusals above prove nothing.
	if _, ok := startupShareLink([]string{"BibleText.exe", "https://bibletext.co.uk/web/john/3/#v16"}); !ok {
		t.Fatal("a real site link was refused")
	}
}

func TestStartupShareLinkStripsShellQuotesAndSwapsTheScheme(t *testing.T) {
	for in, want := range map[string]string{
		`"https://bibletext.co.uk/web/john/3/#v16"`:          "https://bibletext.co.uk/web/john/3/#v16",
		"bibletext://bibletext.co.uk/web/john/3/#v16&n=YWJj": "https://bibletext.co.uk/web/john/3/#v16&n=YWJj",
		"BIBLETEXT://bibletext.co.uk/web/john/3/#v16":        "https://bibletext.co.uk/web/john/3/#v16",
		"HTTPS://WWW.bibletext.co.uk/web/john/3/#v16":        "HTTPS://WWW.bibletext.co.uk/web/john/3/#v16",
		"https://bibletext.co.uk":                            "https://bibletext.co.uk",
		"https://bibletext.co.uk?x=1":                        "https://bibletext.co.uk?x=1",
		"https://bibletext.co.uk:443/web/john/3/#v16":        "https://bibletext.co.uk/web/john/3/#v16", // the site's own port, canonicalised
		"https://bibletext.co.uk./web/john/3/#v16":           "https://bibletext.co.uk/web/john/3/#v16", // a trailing dot names the same host
	} {
		got, ok := startupShareLink([]string{"exe", in})
		if !ok || got != want {
			t.Errorf("startupShareLink(%q) = %q, %v; want %q", in, got, ok, want)
		}
		if _, ok := ParseShareLink(got); strings.Contains(in, "/john/") && !ok {
			t.Errorf("the normalised %q is not a link ParseShareLink accepts", got)
		}
	}
}

// The scheme the site's "Open in BibleText" button emits must round-trip:
// for every version a link path may name, bibletext:// + the https link's
// remainder → the intake → ParseShareLink gives the passage and the note.
func TestSchemeLinkRoundTripsThroughTheApp(t *testing.T) {
	for v := range linkPathVersionIDs {
		https := ShareLinkURLWithNote(v, "John", 3, 16, 0, "a note for the round trip")
		scheme := bibleTextScheme + strings.TrimPrefix(https, "https://")
		got, ok := startupShareLink([]string{"exe", scheme})
		if !ok {
			t.Fatalf("%s: the scheme spelling was refused", v)
		}
		tgt, ok := ParseShareLink(got)
		if !ok || tgt.VersionID != v || tgt.Book != "John" || tgt.Chapter != 3 || tgt.VerseLo != 16 {
			t.Errorf("%s: parsed %+v from %q", v, tgt, got)
		}
		if tgt.Note != "a note for the round trip" {
			t.Errorf("%s: the note did not cross: %q", v, tgt.Note)
		}
	}
}

// A startup link arrives before the Bible has loaded: it must park, and the
// load must open it — the same shape as a cold Universal Link on the Mac.
func TestStartupLinkParksBeforeTheBibleLoads(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	st := NewLoadingState()
	link := "https://bibletext.co.uk/web/john/3/#v16"
	if !deliverStartupLink(st, link) {
		t.Fatal("the startup link was not claimed")
	}
	if st.pendingLink == nil || st.pendingLinkRaw != link {
		t.Fatalf("nothing parked: pendingLink=%v raw=%q", st.pendingLink, st.pendingLinkRaw)
	}
	if st.CurrentBook != "" {
		t.Fatalf("navigated before the Bible loaded (CurrentBook=%q)", st.CurrentBook)
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st.Bible = bd
	st.CurrentVersion = "web"
	st.loadPhase = loadReady
	consumePendingLink(st)
	if st.CurrentBook != "John" || st.CurrentChapter != 3 || st.hlLo() != 16 {
		t.Errorf("after the load: %s %d verse %d, want John 3:16", st.CurrentBook, st.CurrentChapter, st.hlLo())
	}
	if st.pendingLink != nil {
		t.Error("the park was not cleared")
	}
}

// A note-bearing startup link with notes off parks, and the load re-asks the
// question. With no window to put the offer on (this state has none) the
// offer's answer is the browser, exactly once — never the note unasked.
func TestStartupLinkWithANoteAndNoWindowGoesToTheBrowserWhenNotesAreOff(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(false)
	defer setNotesEnabled(true)
	opened := stubBrowser(t)
	st := NewLoadingState()
	link := ShareLinkURLWithNote("web", "John", 3, 16, 0, "fixture note")
	if !deliverStartupLink(st, link) || st.pendingLink == nil {
		t.Fatal("a note-bearing startup link did not park")
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st.Bible = bd
	st.CurrentVersion = "web"
	st.loadPhase = loadReady
	consumePendingLink(st)
	if st.ActiveNote != "" {
		t.Errorf("the note was shown unasked with notes off: %q", st.ActiveNote)
	}
	if st.CurrentBook != "" {
		t.Errorf("the passage opened without the question being answered: %s", st.CurrentBook)
	}
	if got := *opened; len(got) != 1 || got[0] != link {
		t.Errorf("browser opened with %q, want the link once (the window-less answer)", got)
	}
}

// Ours but not a passage (a book index the handler's /web/* claim matches):
// an activated desktop app has no OS fallback, so the browser is opened here.
func TestStartupLinkThatIsNotAPassageGoesToTheBrowser(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	opened := stubBrowser(t)
	st := NewLoadingState()
	if deliverStartupLink(st, "https://bibletext.co.uk/web/john/") {
		t.Fatal("a book index was claimed as a passage")
	}
	if got := *opened; len(got) != 1 || got[0] != "https://bibletext.co.uk/web/john/" {
		t.Fatalf("browser opened with %q, want the book index once", got)
	}
	if st.pendingLink != nil {
		t.Error("a declined link was parked")
	}
}

// Run's order is load-bearing: forward before the app exists (so a second
// process never makes a window), claim the record as soon as the state
// exists and before the window (so two launches cannot both be primaries),
// deliver after CreateMainUI (so the offer has a window) and before the load
// (so the link reaches the one startup rebuild).
func TestRunWiresTheStartupLinkBetweenTheUIAndTheLoad(t *testing.T) {
	body := readSourceFile(t, "app.go")
	run := body[strings.Index(body, "func Run() {"):]
	run = run[:strings.Index(run, "\n}\n")]
	if problem := runOrderProblem(run); problem != "" {
		t.Error(problem)
	}
	// The control: the same check must object to the order it guards against.
	swapped := strings.Replace(run, "deliverStartupLink(state, startup)", "", 1)
	swapped = strings.Replace(swapped, "window := myApp.NewWindow", "deliverStartupLink(state, startup)\n\twindow := myApp.NewWindow", 1)
	if runOrderProblem(swapped) == "" {
		t.Fatal("the order check accepted a startup link delivered before CreateMainUI")
	}
}

func runOrderProblem(run string) string {
	idx := func(s string) int { return strings.Index(run, s) }
	for _, s := range []string{"forwardToRunningInstance(", "app.NewWithID(", "NewLoadingState()",
		"claimSingleInstance(", "myApp.NewWindow(", "CreateMainUI(", "deliverStartupLink(", "StartBackgroundLoad(myApp"} {
		if idx(s) < 0 {
			return "Run() lacks " + s
		}
	}
	switch {
	case idx("forwardToRunningInstance(") > idx("app.NewWithID("):
		return "the forward must come before the app exists"
	case idx("claimSingleInstance(") < idx("NewLoadingState()") || idx("claimSingleInstance(") > idx("myApp.NewWindow("):
		return "the record must be claimed after the state exists and before the window"
	case idx("deliverStartupLink(") < idx("CreateMainUI(") || idx("deliverStartupLink(") > idx("StartBackgroundLoad(myApp"):
		return "the startup link must be delivered after CreateMainUI and before the load"
	}
	return ""
}
