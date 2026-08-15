//go:build bibletextdev && darwin

package bibletext

// The wash-change benchmark, Apple panes only.
//
// It lives apart from dev_autoopen_on.go because it reaches for
// lastPushedBodyFP, which exists only where there is a native overlay to hold a
// chapter (reading_tint_apple.go). Its twin for every other platform is
// dev_tintbench_other.go.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
)

// devPerfMark labels the log line the C timers are about to print, so a
// "tint-mutate 2.9 ms" can be attributed to the arrival that caused it rather
// than guessed at from ordering.
func devPerfMark(step string) {
	fmt.Fprintf(os.Stderr, "bibletext-perf: step %s\n", step)
}

// devForceBodyRebuild makes the next render take the full HTML rebuild +
// NSAttributedString re-import, which is what a BODY change costs.
//
// The read-along scenario uses it to reproduce, mid-playback, what hiding or
// deleting a note, flipping the theme, changing the text size or a background
// data swap all do to the pane: throw the attributed string away and import a
// fresh one, with no wash on it anywhere. That used to lose the narration until
// the next verse tick — up to ~10s — while the pane still claimed to be playing.
// Reproduced this way rather than by actually hiding a note, because every real
// trigger persists something (a preference, the note state) that a screenshot run
// should not leave behind.
func devForceBodyRebuild(state *AppState) {
	lastPushedBodyFP = ""
	state.refreshReadingOnly()
}

// devAutoTintBench measures what a WASH change costs on the longest chapter in
// the canon, both ways, on the device runtime.
//
// The claim the S2 seam rests on is that changing what a verse is washed in does
// not need the chapter rebuilt. That is only worth believing with numbers from
// the runtime that does the work — AND from the functions a reader's finger
// actually reaches. The first version of this bench called state.setMark +
// refreshReadingOnly directly, which is the one way to place a mark that no user
// action takes: every real arrival goes through openSearchResultRange,
// applyShareTarget or goToVerseRange, and all three declare forceReposition. So
// each round here is a REAL arrival:
//
//	go-to      goToVerseRange — the Go-to box, a cross-reference, verse of the day
//	search     openSearchResultRange — a tapped search result
//	link       HandleShareLink — a shared link arriving for the chapter already open
//	clear      clearHighlightedVerse — the "Clear highlight" tap
//
// then the same four again with the body fingerprint forgotten first, which is
// exactly what each of them cost BEFORE the split (the wash was part of the one
// fingerprint, so any change to it rebuilt).
//
//	SIMCTL_CHILD_BIBLETEXT_DEV_TINTBENCH=1 SIMCTL_CHILD_BIBLETEXT_PERF=1 \
//	  xcrun simctl launch <udid> uk.co.bibletext
//
// Set the variable to a number of milliseconds to slow the rounds down
// (BIBLETEXT_DEV_TINTBENCH=2500), which is what makes the OTHER half of the claim
// visible: an arrival must still PLACE the view. Each round marks a verse ~4
// apart, so on a screenshot burst the passage should walk down Psalm 119 under
// the mutation alone, with no html-import in the log.
//
// Read the numbers out of the device log; both are printed by the C side around
// the operation itself, not around the Go call, so neither includes the other's
// setup — and NEITHER INCLUDES RE-LAYOUT. tint-mutate brackets
// beginEditing/endEditing, which invalidates layout and leaves TextKit to
// regenerate glyphs off-timer; html-import brackets the NSAttributedString init
// alone. Both are lower bounds on their path's true cost, so the ratio is a lower
// bound on the win rather than an end-to-end measurement of it.
func devAutoTintBench(state *AppState) {
	spec := strings.TrimSpace(os.Getenv("BIBLETEXT_DEV_TINTBENCH"))
	if spec == "" || state == nil {
		return
	}
	const rounds = 8
	gap := 500
	if n, err := strconv.Atoi(spec); err == nil && n >= 100 {
		gap = n
	}
	step := func(d time.Duration, f func()) { time.AfterFunc(d, func() { fyne.Do(f) }) }
	step(2*time.Second, func() {
		HandleShareLink(state, ShareLinkURL(state.currentVersion().ID, "Psalms", 119, 0, 0))
	})
	vid := func() string { return state.currentVersion().ID }
	// The four arrivals, each labelled in the log so a line can be attributed to
	// the path that produced it.
	arrivals := []struct {
		name string
		do   func(i int)
	}{
		{"go-to", func(i int) { goToVerseRange(state, "Psalms", 119, 100+i%20, 100+i%20) }},
		{"search", func(i int) {
			openSearchResultRange(state, Verse{
				BookName: "Psalms", Chapter: 119, Verse: 100 + i%20,
				Text: "", Ref: "",
			}, 100+i%20)
		}},
		{"link", func(i int) {
			HandleShareLink(state, ShareLinkURL(vid(), "Psalms", 119, 100+i%20, 100+i%20))
		}},
		{"clear", func(int) {
			clearHighlightedVerse(state)
			state.refreshReadingOnly()
		}},
	}
	t := 5000
	for i := 0; i < rounds; i++ {
		a := arrivals[i%len(arrivals)]
		i := i
		step(time.Duration(t)*time.Millisecond, func() {
			devPerfMark("arrival/" + a.name)
			a.do(i)
		})
		t += gap
	}
	t += 1000
	for i := 0; i < rounds; i++ {
		a := arrivals[i%len(arrivals)]
		i := i
		step(time.Duration(t)*time.Millisecond, func() {
			devPerfMark("before-the-split/" + a.name)
			// What this cost BEFORE the split: the wash was part of the one
			// fingerprint, so any change to it missed the gate and rebuilt.
			lastPushedBodyFP = ""
			a.do(i)
		})
		t += gap + 200
	}
}
