package bibletext

// THE TEST KEY LINE OUTLIVES THE SHEET THAT PAINTED IT.
//
// Under each key field in Settings, Test key shows a wait ("Testing…", then
// the model the request reports), and then its verdict. Those were painted by
// closures of the sheet the button was tapped on, into that sheet's own line.
// A light/dark change drains the sheet and reopens a fresh one
// (sheet_reopen.go), so a test tapped before the change reported to a sheet
// nobody could see: the reopened sheet showed neither the wait nor the verdict,
// and the reader had no sign the test had finished. Settings otherwise loses
// nothing to a reopen, because everything in it saves as it changes; a test in
// flight is the one thing that does not.
//
// So what a test has shown so far lives on AppState (state.keyTests), one slot
// per key section, and the sheet that is showing registers how it paints the
// slot's line. The sheet paints from the slot as it is built, so a reopened
// sheet shows the wait of a test still running and the verdict of one that
// landed while no sheet was up, and the test's own reports paint through
// whichever sheet is showing when they arrive. The slot is forgotten when the
// sheet is closed by its ✕ (the test still runs, and its verdict goes nowhere,
// as before) and when Settings is opened as the reader opens it, so a later
// visit never opens on a stale verdict; only the reopen keeps it.
//
// A test tapped again before the last one answered replaces the slot's test:
// the older test's later reports find another test in the slot and are
// dropped, not painted over the newer wait. UI goroutine only.

// keyTestProgress is what one Test key has shown so far.
type keyTestProgress struct {
	provider string       // the key it tests: an AI provider's id, or "" for the API.Bible key
	running  bool         // the request is in flight
	model    aiModelInUse // the model it reported, if any (the AI key)
	text     string       // the verdict, or the answer given without a request
}

// keyTestSlot is one key section's Test key line: the test it shows, and the
// showing sheet's way of painting it.
type keyTestSlot struct {
	test  *keyTestProgress
	paint func()
}

// The key sections, each with a slot of its own.
const (
	keyTestAI    = "ai"
	keyTestBible = "bible"
)

// keyTest is the slot for one key section, made on first use.
func (s *AppState) keyTest(section string) *keyTestSlot {
	if s.keyTests == nil {
		s.keyTests = map[string]*keyTestSlot{}
	}
	sl, ok := s.keyTests[section]
	if !ok {
		sl = &keyTestSlot{}
		s.keyTests[section] = sl
	}
	return sl
}

// attach registers the showing sheet's painter and paints the slot's line
// with it at once, so a reopened sheet shows what the test has shown so far.
func (sl *keyTestSlot) attach(paint func()) {
	sl.paint = paint
	sl.repaint()
}

// begin starts a new test's line: running with no verdict yet, or, for a key
// that cannot be tested, its answer. Any older test loses the line.
func (sl *keyTestSlot) begin(provider string, running bool, text string) *keyTestProgress {
	t := &keyTestProgress{provider: provider, running: running, text: text}
	sl.test = t
	sl.repaint()
	return t
}

// report names the model t is sending to, if t still owns the line.
func (sl *keyTestSlot) report(t *keyTestProgress, m aiModelInUse) {
	if sl.test != t {
		return
	}
	t.model = m
	sl.repaint()
}

// finish lands t's verdict, if t still owns the line.
func (sl *keyTestSlot) finish(t *keyTestProgress, text string) {
	if sl.test != t {
		return
	}
	t.running = false
	t.text = text
	sl.repaint()
}

func (sl *keyTestSlot) repaint() {
	if sl.paint != nil {
		sl.paint()
	}
}

// forgetKeyTests drops every section's test, so the next sheet opens with no
// line. Its painters stay: a painter belongs to a sheet, and the next sheet
// registers its own.
func forgetKeyTests(state *AppState) {
	if state == nil {
		return
	}
	for _, sl := range state.keyTests {
		sl.test = nil
	}
}
