package bibletext

import (
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// muteEngines swaps the per-platform audio engines for no-ops so controller
// tests are deterministic: the real engines would fetch and play the chapter,
// and how far they get before an assert is runner-dependent (network + audio
// device). Restores the real engines when the test ends.
//
// It leaves the artwork hook alone, which TestMain sets once for the whole
// binary (artworkHookReached). startChapter starts the engine and then
// renders the lock-screen artwork on a goroutine of its own, which reads
// imageRenderDir and then the artwork hook, so a render still running when
// its test ended read both while the next test wrote them. Each engine start
// here counts the render that follows it, and the test does not end until
// every render counted has reached the hook.
func muteEngines(t *testing.T) {
	t.Helper()
	origURL, origTTS, origToggle := engineStartURL, engineStartTTS, engineToggle
	origStop, origSkip := engineStop, engineSkip
	engineStartURL = func(string, string, string) { artworkRenderBegun() }
	engineStartTTS = func(string, string, string) { artworkRenderBegun() }
	engineToggle = func() {}
	engineStop = func() {}
	engineSkip = func(float64) {}
	t.Cleanup(func() {
		awaitArtworkRenders(t)
		engineStartURL, engineStartTTS, engineToggle = origURL, origTTS, origToggle
		engineStop, engineSkip = origStop, origSkip
	})
}

// artworkRenders counts the artwork renders begun under muted engines and
// those that have reached the artwork hook.
var artworkRenders struct {
	sync.Mutex
	begun, reached int
}

// artworkRenderBegun counts the render startChapter begins after it starts
// the engine.
func artworkRenderBegun() {
	artworkRenders.Lock()
	artworkRenders.begun++
	artworkRenders.Unlock()
}

// artworkHookReached is the artwork hook for the whole test binary.
func artworkHookReached(string) {
	artworkRenders.Lock()
	artworkRenders.reached++
	artworkRenders.Unlock()
}

// awaitArtworkRenders returns once every render begun has reached the hook.
// Taking the lock the hook takes orders everything a render read before
// whatever the caller writes next. A render that fails returns before the
// hook, so after 30 s the test fails and the count is settled, which keeps
// the tests after it from waiting for that render too.
func awaitArtworkRenders(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		artworkRenders.Lock()
		begun, reached := artworkRenders.begun, artworkRenders.reached
		late := reached < begun && time.Now().After(deadline)
		if late {
			artworkRenders.reached = begun
		}
		artworkRenders.Unlock()
		switch {
		case reached >= begun:
			return
		case late:
			t.Errorf("%d of %d artwork renders begun under muted engines never reached the artwork hook in 30 s", begun-reached, begun)
			return
		}
		time.Sleep(time.Millisecond)
	}
}

// A narration test ends only once the lock-screen artwork it started has
// been rendered. startChapter draws the artwork on a goroutine of its own,
// which reads imageRenderDir when it writes the file and then the artwork
// hook; a render left running read them while the next test wrote them, a
// narration test through muteEngines and a share test through
// imageRenderDir. The first subtest starts narration as those tests do. The
// second checks no render is still running, and that muteEngines leaves the
// artwork hook as it found it. It then writes what the next test writes and
// goes on while any render runs, waiting for it without synchronising with
// it, so under the race detector a render still running then is reported
// here. The control shows the wait sees the render goroutine while it runs.
func TestANarrationTestEndsOnlyOnceItsArtworkIsRendered(t *testing.T) {
	rendering := funcName(renderChapterArtwork) + "("
	artwork := funcName((*audioController).startChapter) + ".func"
	hook := reflect.ValueOf(engineSetArtwork).Pointer()
	dir := t.TempDir()
	t.Run("narration starts", func(t *testing.T) {
		muteEngines(t)
		gAudio.stop()
		t.Cleanup(gAudio.stop)
		gAudio.startChapter(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20},
			chapterAudio{Kind: audioRecorded, URL: "https://example.invalid/WEB_43_020.mp3",
				Title: "John 20", Subtitle: "World English Bible"}, "web|John|20")
		if !goroutineIn(artwork) {
			t.Fatal("control: no goroutine of startChapter's is running just after it started the artwork, so the wait below sees nothing")
		}
	})
	t.Run("the next test", func(t *testing.T) {
		if goroutineIn(rendering) {
			t.Error("the artwork the previous test started is still rendering")
		}
		muteEngines(t)
		if reflect.ValueOf(engineSetArtwork).Pointer() != hook {
			t.Error("muteEngines swapped the artwork hook, which a render reads from its own goroutine")
		}
		prev := imageRenderDir
		imageRenderDir = func() string { return dir }
		t.Cleanup(func() { imageRenderDir = prev })
		deadline := time.Now().Add(30 * time.Second)
		for goroutineIn(artwork) {
			if time.Now().After(deadline) {
				t.Fatal("the artwork goroutine was still running 30 s after narration started")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}

// funcName is the name the runtime prints in a stack for fn.
func funcName(fn any) string {
	return runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
}

// goroutineIn reports whether any goroutine's stack holds a frame whose
// printed form contains frame. It reads the stacks without synchronising with
// the goroutines, so it orders nothing for the race detector.
func goroutineIn(frame string) bool {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			return strings.Contains(string(buf[:n]), frame)
		}
		buf = make([]byte, 2*len(buf))
	}
}

func TestChapterAudioFingerprint(t *testing.T) {
	got := chapterAudioFingerprint(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20})
	if got != "web|John|20" {
		t.Fatalf("fingerprint = %q, want web|John|20", got)
	}
	if got := chapterAudioFingerprint(nil); got != "" {
		t.Fatalf("nil fingerprint = %q, want empty", got)
	}
}

func TestAudioControllerStop(t *testing.T) {
	c := &audioController{loaded: true, loadedFP: "web|John|20", kind: audioRecorded, state: audioPlaying}
	c.stop()
	if c.isPlaying() {
		t.Fatal("isPlaying() true after stop")
	}
	if fp := c.playingFingerprint(); fp != "" {
		t.Fatalf("playingFingerprint() = %q after stop, want empty", fp)
	}
}

func TestStopAudioForNav(t *testing.T) {
	muteEngines(t)
	gAudio.stop() // clean slate (other tests share the global)
	defer gAudio.stop()

	gAudio.startChapter(
		&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20},
		chapterAudio{Kind: audioRecorded, URL: "https://github.com/cubancorona/bibletext-audio/releases/download/web-williams-nt-v1/WEB_43_020.mp3"},
		"web|John|20",
	)
	// A recorded start starts life BUFFERING (spinner) until the native layer
	// reports audible playback — but it is loaded and bound to its chapter.
	if !gAudio.buffering("web|John|20") || gAudio.playingFingerprint() != "web|John|20" {
		t.Fatalf("startChapter did not load: buffering=%v fp=%q",
			gAudio.buffering("web|John|20"), gAudio.playingFingerprint())
	}

	// Re-landing on the SAME chapter must leave playback alone.
	stopAudioForNav(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20})
	if gAudio.playingFingerprint() != "web|John|20" {
		t.Fatal("same-chapter navigation stopped audio; it should continue")
	}

	// Navigating to a DIFFERENT chapter must stop it.
	stopAudioForNav(&AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 21})
	if gAudio.isPlaying() || gAudio.playingFingerprint() != "" {
		t.Fatalf("different-chapter navigation did not stop audio: playing=%v fp=%q",
			gAudio.isPlaying(), gAudio.playingFingerprint())
	}
}

func TestAdvanceToNextChapter(t *testing.T) {
	muteEngines(t)
	gAudio.stop() // the controller is a shared global; start clean
	defer gAudio.stop()

	bd := &BibleData{
		Books: []string{"BookA", "BookB"},
		Verses: map[string]map[int][]Verse{
			"BookA": {1: {}, 2: {}},
			"BookB": {1: {}, 2: {}},
		},
	}
	state := &AppState{Bible: bd, CurrentVersion: "web", CurrentBook: "BookA", CurrentChapter: 1}

	// Within a book: ch1 → ch2.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookA" || state.CurrentChapter != 2 {
		t.Fatalf("within-book advance = %s %d, want BookA 2", state.CurrentBook, state.CurrentChapter)
	}
	// Across a book boundary: BookA's last chapter → BookB ch1.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookB" || state.CurrentChapter != 1 {
		t.Fatalf("cross-book advance = %s %d, want BookB 1", state.CurrentBook, state.CurrentChapter)
	}
	// Within BookB: ch1 → ch2.
	if !advanceToNextChapter(state) || state.CurrentBook != "BookB" || state.CurrentChapter != 2 {
		t.Fatalf("BookB advance = %s %d, want BookB 2", state.CurrentBook, state.CurrentChapter)
	}
	// End of the Bible (last book, last chapter): no next, state unchanged.
	if advanceToNextChapter(state) {
		t.Fatal("advancing past the last chapter of the last book should return false")
	}
	if state.CurrentBook != "BookB" || state.CurrentChapter != 2 {
		t.Fatalf("end-of-Bible advance mutated state to %s %d", state.CurrentBook, state.CurrentChapter)
	}
}

func TestAudioSourceIconForKind(t *testing.T) {
	// Read-aloud (TTS) → the waveform glyph; recorded → something else (the person).
	if got := audioSourceIcon(audioTTS, false); got != iconAudioWave {
		t.Fatalf("TTS source icon = %v, want iconAudioWave", got)
	}
	if got := audioSourceIcon(audioRecorded, false); got == iconAudioWave {
		t.Fatal("recorded source icon should not be the waveform glyph")
	}
}

// TestSelectSourceNarratorStaleness locks the narrator-aware staleness rule:
// choosing a DIFFERENT recording of the same kind for the loaded chapter must
// stop the now-stale audio (so Play starts the chosen narrator cleanly), while
// re-choosing the recording that's already loaded leaves it playing.
func TestSelectSourceNarratorStaleness(t *testing.T) {
	state := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20}
	fp := chapterAudioFingerprint(state)
	load := func() {
		gAudio.mu.Lock()
		gAudio.loaded = true
		gAudio.loadedFP = fp
		gAudio.kind = audioRecorded
		gAudio.loadedRecID = "web-williams"
		gAudio.state = audioPlaying
		gAudio.mu.Unlock()
	}
	reset := func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.loadedRecID = false, "", ""
		gAudio.kind, gAudio.state = audioRecorded, audioIdle
		gAudio.hasPreferred, gAudio.preferredFP, gAudio.preferredRecID = false, "", ""
		gAudio.mu.Unlock()
	}
	defer reset()

	// Same recording re-chosen → stays loaded.
	load()
	gAudio.selectSource(state, audioRecorded, "web-williams")
	if !gAudio.isPlaying() {
		t.Fatal("re-choosing the loaded recording must not stop it")
	}

	// A different narrator chosen → the loaded audio is stale and stops.
	gAudio.selectSource(state, audioRecorded, "web-somebody-else")
	if gAudio.isPlaying() {
		t.Fatal("choosing a different recording must stop the stale loaded audio")
	}

	// And the preference survives for the play button.
	if kind, recID := gAudio.effectiveSource(state); kind != audioTTS && recID == "web-williams" {
		t.Fatalf("effectiveSource after choosing another narrator = (%v, %q)", kind, recID)
	}
	reset()

	// effectiveSource default: the version's recording where it covers the chapter…
	if kind, recID := gAudio.effectiveSource(state); kind != audioRecorded || recID != "web-williams" {
		t.Fatalf("default effectiveSource = (%v, %q), want (recorded, web-williams)", kind, recID)
	}
	// …and where Williams stops, the WEB-Catholic's OTHER recording takes over
	// rather than the chapter dropping to read-aloud: the deuterocanon now has a
	// synthetic narration, so the default source there is that recording.
	tobit := &AppState{CurrentVersion: "webc", CurrentBook: "Tobit", CurrentChapter: 1}
	if kind, recID := gAudio.effectiveSource(tobit); kind != audioRecorded || recID != webbeRecordingID {
		t.Fatalf("deuterocanon effectiveSource = (%v, %q), want (recorded, %q)", kind, recID, webbeRecordingID)
	}
	// A version with no recording at all is still read-aloud.
	nkjv := &AppState{CurrentVersion: "nkjv", CurrentBook: "John", CurrentChapter: 3}
	if kind, recID := gAudio.effectiveSource(nkjv); kind != audioTTS || recID != "" {
		t.Fatalf("unrecorded version effectiveSource = (%v, %q), want (TTS, \"\")", kind, recID)
	}
}

// TestReadAlongFollowSuspend locks the scroll-tug-of-war contract: a user scroll
// during read-along suspends the follow (highlight keeps tracking) and surfaces
// the floating "Follow narration" pill, resume re-attaches and drops it, arming
// a new chapter resets it, and playback ending takes the pill down with it. The
// pill transitions are recorded through the showFollowButton seam — the exact
// calls the shipped UI performs.
func TestReadAlongFollowSuspend(t *testing.T) {
	state := &AppState{CurrentVersion: "web", CurrentBook: "John", CurrentChapter: 20}
	fp := chapterAudioFingerprint(state)

	var pill []bool
	showFollowButton = func(show bool) { pill = append(pill, show) }
	defer func() { showFollowButton = readAlongFollowButton }()
	lastPill := func() bool { return len(pill) > 0 && pill[len(pill)-1] }

	reset := func() {
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP, gAudio.loadedRecID = false, "", ""
		gAudio.kind, gAudio.state = audioRecorded, audioIdle
		gAudio.readAlong, gAudio.readAlongVerse, gAudio.followSuspended = nil, readAlongNone, false
		gAudio.mu.Unlock()
	}
	defer reset()

	gAudio.mu.Lock()
	gAudio.loaded, gAudio.loadedFP, gAudio.state = true, fp, audioPlaying
	gAudio.readAlong = []verseTiming{{1, 0, 1}, {2, 10, 20}}
	gAudio.mu.Unlock()

	// A user scroll suspends the follow and shows the pill exactly once; a
	// second report is a no-op (no duplicate show).
	gAudio.onReadAlongUserScroll()
	if !gAudio.followSuspendedFor(fp) {
		t.Fatal("user scroll did not suspend the follow")
	}
	if len(pill) != 1 || !pill[0] {
		t.Fatalf("pill transitions after first scroll = %v, want [true]", pill)
	}
	gAudio.onReadAlongUserScroll()
	if !gAudio.followSuspendedFor(fp) {
		t.Fatal("second scroll report flipped the suspension")
	}
	if len(pill) != 1 {
		t.Fatalf("second scroll re-showed the pill: transitions %v", pill)
	}

	// Ticks keep updating the tracked verse while suspended.
	gAudio.onTimeUpdate(12)
	gAudio.mu.Lock()
	v := gAudio.readAlongVerse
	gAudio.mu.Unlock()
	if v != 2 {
		t.Fatalf("verse tracking stopped while suspended: verse=%d, want 2", v)
	}

	// Resume re-attaches and drops the pill.
	gAudio.resumeReadAlongFollow()
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("resume did not clear the suspension")
	}
	if lastPill() {
		t.Fatalf("resume left the pill up: transitions %v", pill)
	}

	// Arming a new chapter always starts attached (pill down).
	gAudio.onReadAlongUserScroll()
	gAudio.armReadAlong(state, chapterAudio{Kind: audioTTS})
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("armReadAlong did not reset the suspension")
	}
	if lastPill() {
		t.Fatalf("armReadAlong left the pill up: transitions %v", pill)
	}

	// Playback ending on its own takes the pill (and the suspension) down —
	// the pill must never advertise a narration that is no longer playing.
	gAudio.mu.Lock()
	gAudio.loaded, gAudio.loadedFP, gAudio.state = true, fp, audioPlaying
	gAudio.readAlong, gAudio.followSuspended = []verseTiming{{1, 0, 1}}, false
	gAudio.boundState = nil // no continuation in unit tests
	gAudio.mu.Unlock()
	gAudio.onReadAlongUserScroll()
	if !lastPill() {
		t.Fatalf("scroll during playback did not show the pill: transitions %v", pill)
	}
	gAudio.applyNativeState(audioEnded)
	if lastPill() {
		t.Fatalf("playback end left the pill up: transitions %v", pill)
	}
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("playback end left the follow suspended")
	}

	// No read-along armed (or nothing loaded) → scroll reports are ignored.
	reset()
	pill = nil
	gAudio.onReadAlongUserScroll()
	if gAudio.followSuspendedFor(fp) {
		t.Fatal("scroll with no read-along armed suspended something")
	}
	if len(pill) != 0 {
		t.Fatalf("scroll with nothing armed touched the pill: transitions %v", pill)
	}
}

// TestAdvanceToNextChapterPreservesSearchContext pins the boundary fix: the
// advance runs from a BACKGROUND audio event (a chapter finishing), so it must
// never tear down foreground search state. Within-book advances go through
// moveChapter, which leaves search alone; the book-boundary branch used to go
// through selectBook, which reset IsSearching/CanReturnToSearchResults and
// yanked the results the reader was browsing.
func TestAdvanceToNextChapterPreservesSearchContext(t *testing.T) {
	muteEngines(t)
	gAudio.stop()
	defer gAudio.stop()

	bd := &BibleData{
		Books: []string{"BookA", "BookB"},
		Verses: map[string]map[int][]Verse{
			"BookA": {1: {}, 2: {}},
			"BookB": {1: {}, 2: {}},
		},
	}
	state := &AppState{Bible: bd, CurrentVersion: "web", CurrentBook: "BookA", CurrentChapter: 2,
		IsSearching: true, CanReturnToSearchResults: true, ActiveSearchQuery: "shepherd"}

	if !advanceToNextChapter(state) || state.CurrentBook != "BookB" || state.CurrentChapter != 1 {
		t.Fatalf("cross-book advance = %s %d, want BookB 1", state.CurrentBook, state.CurrentChapter)
	}
	if !state.IsSearching || !state.CanReturnToSearchResults {
		t.Errorf("book-boundary advance tore down search context: IsSearching=%v CanReturnToSearchResults=%v",
			state.IsSearching, state.CanReturnToSearchResults)
	}
	if len(state.RecentChapters) == 0 || state.RecentChapters[0].Book != "BookB" || state.RecentChapters[0].Chapter != 1 {
		t.Errorf("boundary advance must still record history; head = %+v", state.RecentChapters)
	}
}

// TestReadAlongTitleTickHighlights pins the title's path through the controller.
// The controller dedupes ticks against the row it last announced, and that row
// used to start life as 0 — the value the title now occupies — so a titled
// psalm's opening tick was swallowed and the title never lit. The none value is
// readAlongNone, nothing-narrated reaches the surface as an explicit clear, and
// the follow resume and the reassert re-issue the title like any verse. The
// calls are recorded through the highlightVerse/clearHighlight seams — the
// exact calls the shipped controller makes — and the old initial value is forced
// back in as the control.
func TestReadAlongTitleTickHighlights(t *testing.T) {
	type call struct {
		verse  int
		follow bool
	}
	var calls []call
	clears := 0
	highlightVerse = func(v int, follow bool) { calls = append(calls, call{v, follow}) }
	clearHighlight = func() { clears++ }
	showFollowButton = func(bool) {}
	defer func() {
		highlightVerse, clearHighlight, showFollowButton = readAlongHighlight, readAlongClear, readAlongFollowButton
	}()
	defer func() {
		gAudio.clearReadAlong()
		gAudio.mu.Lock()
		gAudio.loaded, gAudio.loadedFP = false, ""
		gAudio.mu.Unlock()
	}()

	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {3: {
			{Verse: 1, Text: "LORD, how my foes have increased!"},
			{Verse: 2, Text: "Many say of me."},
		}}},
		Superscriptions: map[string]map[int]Superscription{"Psalms": {3: {Text: "A Psalm of David."}}},
	}
	state := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 3, CurrentVersion: "bsb"}
	gAudio.armReadAlong(state, chapterAudio{Kind: audioTTS}) // the 0 row comes from speechVerseOffsets, not a hand table
	offs := speechVerseOffsets(state)
	if len(offs) != 3 || offs[0].verse != readAlongTitle {
		t.Fatalf("fixture offsets = %+v, want the title row first", offs)
	}

	calls = nil
	gAudio.onSpeechRange(0)
	if len(calls) != 1 || calls[0] != (call{readAlongTitle, true}) {
		t.Fatalf("the title's first tick produced %v, want one highlight of the title with follow", calls)
	}
	gAudio.onSpeechRange(5) // still inside the title: deduped
	if len(calls) != 1 {
		t.Fatalf("a tick inside the title re-announced it: %v", calls)
	}
	gAudio.onSpeechRange(int(offs[1].start))
	if len(calls) != 2 || calls[1] != (call{1, true}) {
		t.Fatalf("verse 1's tick produced %v, want a highlight of verse 1", calls)
	}

	// CONTROL: with the old initial value forced back in, the same title tick
	// is swallowed — the defect the sentinel exists for.
	gAudio.mu.Lock()
	gAudio.readAlongVerse = 0
	gAudio.mu.Unlock()
	calls = nil
	gAudio.onSpeechRange(0)
	if len(calls) != 0 {
		t.Fatalf("with the last row forced to 0 the title tick still announced %v; the control does not reproduce the swallow", calls)
	}

	// A seek back before the first row is an explicit clear, never a highlight
	// of the none value.
	gAudio.mu.Lock()
	gAudio.readAlong = []verseTiming{{0, 3, 6}, {1, 6.5, 12}}
	gAudio.readAlongVerse = 1
	gAudio.mu.Unlock()
	calls, clears = nil, 0
	gAudio.onTimeUpdate(1.0)
	if clears != 1 || len(calls) != 0 {
		t.Fatalf("seek before the first row: %d clears, highlights %v; want one clear and no highlight", clears, calls)
	}
	gAudio.onTimeUpdate(1.5) // still nothing: deduped against readAlongNone
	if clears != 1 {
		t.Fatalf("a second tick in the intro cleared again (%d clears)", clears)
	}

	// With the title current, the follow resume and the reassert both re-issue
	// it — formerly skipped by a `v > 0` guard.
	gAudio.mu.Lock()
	gAudio.loaded, gAudio.loadedFP = true, chapterAudioFingerprint(state)
	gAudio.readAlongVerse, gAudio.followSuspended = readAlongTitle, true
	gAudio.mu.Unlock()
	calls = nil
	gAudio.resumeReadAlongFollow()
	if len(calls) != 1 || calls[0] != (call{readAlongTitle, true}) {
		t.Fatalf("resume with the title current produced %v, want a highlight of the title", calls)
	}
	gAudio.reassertReadAlong()
	if len(calls) != 2 || calls[1] != (call{readAlongTitle, true}) {
		t.Fatalf("reassert with the title current produced %v, want a second highlight of the title", calls)
	}
	// And with nothing current, neither says anything.
	gAudio.mu.Lock()
	gAudio.readAlongVerse, gAudio.followSuspended = readAlongNone, true
	gAudio.mu.Unlock()
	calls = nil
	gAudio.resumeReadAlongFollow()
	gAudio.reassertReadAlong()
	if len(calls) != 0 {
		t.Fatalf("with nothing narrated, resume/reassert highlighted %v", calls)
	}
}
