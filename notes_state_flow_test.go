package bibletext

// The notes subsystem, enumerated. See docs/NOTES_STATE.md.
//
// WHY THIS IS AN ENUMERATION AND NOT A LIST OF CASES. In one day this subsystem
// produced five distinct defects and every one was found by accident: a
// highlight with no note to explain it, a Delete that deleted the wrong note, a
// note silently swapped for a different one, a Hide reversed by a trailing
// section, and a note displayed under a translation it is not stored under. Each
// was reachable by a combination nobody thought to write a test for. A test that
// checks the cases we thought of is exactly what failed us, so this one walks
// the cross-product instead — the same shape share_link_flow_test.go has, and
// for the same reason.
//
// HOW IT FAILS USEFULLY. Every violation reachable TODAY is pinned in
// knownIncoherent below, by the name docs/NOTES_STATE.md gives it. The
// assertion is on set EQUALITY — so it fails both when a new incoherent state
// appears AND when one is fixed without being struck off. The second half is the
// point: a fix that leaves this list stale makes the next reader trust a
// document that lies.
//
// THE HARNESS CARRIES A VARIABLE THE APP DOES NOT. The second enumeration below
// tracks where a highlight CAME FROM. AppState has no such field — which is why
// applyNoteForCurrentChapter has to infer ownership from a bare verse number
// (notes_store.go:307-310), and why that inference is wrong in both directions.
// Needing the variable here in order to state the invariant is the finding, not
// a convenience.

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// --- the variables ----------------------------------------------------------

// notePlacement is where the chapter's notes live relative to the translation
// being read. This is the axis loadNote branches on (notes_store.go:165-171).
type notePlacement int

const (
	placeNone     notePlacement = iota // nothing stored for this passage anywhere
	placeOwn                           // one note, under the translation being read
	placeFollowed                      // one note, under ANOTHER translation
	placeBoth                          // one under each: the exact key masks the other
)

func (p notePlacement) String() string {
	return [...]string{"none", "own", "followed", "both"}[p]
}

// noteVerb is what the reader does next. Only the verbs a surface actually
// offers are exercised — see runNotesFlow's `offered`.
type noteVerb int

const (
	verbNone noteVerb = iota
	verbHide
	verbShow
	verbDelete
	verbNotesOff // Settings → the switch, answering "Keep them"
)

func (v noteVerb) String() string {
	return [...]string{"none", "hide", "show", "delete", "notes-off"}[v]
}

// notesWorld is the slice of the world the notes subsystem branches on. Kept as
// its own type so the enumeration is readable, and so adding a variable to the
// subsystem means adding it HERE, where the cross-product picks it up.
type notesWorld struct {
	featureOn bool
	placement notePlacement
	collapsed bool // the note under the exact key is stored Minimized
	foreignHL bool // a highlight from another origin is already on the chapter
	arrival   bool // a note-bearing link landed on this chapter after the derive
	verb      noteVerb
}

func (w notesWorld) id() string {
	return fmt.Sprintf("on=%v place=%s collapsed=%v foreignHL=%v arrival=%v verb=%s",
		w.featureOn, w.placement, w.collapsed, w.foreignHL, w.arrival, w.verb)
}

// --- what is broken today ---------------------------------------------------

// pinnedDefect is one incoherent state from docs/NOTES_STATE.md, plus the exact
// region of the state space it accounts for.
//
// WHY A PREDICATE AND NOT A LIST OF STRINGS. The cross-product finds 117
// violations, which is not 117 defects — it is eight defects, each of which
// happens to be reachable from many combinations (every placeBoth cell violates
// N4 for one reason). A flat list would bury that, and worse, it would let a
// genuinely new violation hide among a hundred pinned ones that look the same.
//
// The assertion is still set EQUALITY, in both directions and at the same
// strength: a violation no predicate covers is a NEW incoherent state and fails;
// a predicate that covers nothing means the defect is FIXED, and fails until it
// is struck off here and in the document. What the predicate buys is that every
// violation must be attributed to a named, documented defect rather than merely
// counted.
type pinnedDefect struct {
	name   string // as docs/NOTES_STATE.md names it
	what   string // one line, for the failure message
	covers func(w notesWorld, inv string) bool
}

// X1 and X2 were struck on 2026-08-15, fixed by 31bc97630 ("Write the live
// note's four fields as one value"): both verbs now address the note
// NoteVersionID names because every arrival path writes that field. The
// enumeration proved them dead — each covered zero violations — and this list
// must not outlive its defects.
//
// Striking them is what made X12 visible, and the two facts belong together:
// while Delete was missing the arriving note, it was also masking the
// substitution that follows a successful one.
var knownIncoherent = []pinnedDefect{
	{
		"X4", "turning notes off keeps the highlight the note put there",
		func(w notesWorld, inv string) bool {
			return inv == "N1-orphan-highlight" && w.verb == verbNotesOff
		},
	},
	{
		"X5", "Hide addresses noteStoreVersion(), Show addresses currentVersion()",
		func(w notesWorld, inv string) bool {
			return (inv == "N2-show-missed" || inv == "N5-show-not-honoured") &&
				w.placement == placeFollowed && w.collapsed && !w.arrival && w.verb == verbShow
		},
	},
	{
		"X6", "deleting the exact-key note lets another translation's note take its place",
		func(w notesWorld, inv string) bool {
			return inv == "N3-substituted" && w.placement == placeBoth && w.verb == verbDelete
		},
	},
	{
		// X6's mechanism in the region X1 used to occupy. It is not a new
		// defect and 31bc97630 did not create it: loadNote has always fallen
		// through to noteFromAnotherTranslation, so a second note has always
		// been waiting to take the deleted one's place. What that commit
		// changed is that Delete now WORKS on an arriving note, and a delete
		// that misses cannot expose the note behind it.
		//
		// So the trade is a real improvement, not a wash: before, the reader
		// destroyed a note they were not looking at (X1, data gone); now the
		// right note dies and a different one appears unannounced (confusing,
		// nothing lost). Both violate N3, and only the arity-1 read fixes it.
		"X12", "delete the arriving note and the followed one silently takes its place",
		func(w notesWorld, inv string) bool {
			return inv == "N3-substituted" &&
				w.placement == placeFollowed && w.arrival && w.verb == verbDelete
		},
	},
	{
		"X7", "loadNote returns one note; the rest of the passage's notes have no trace",
		func(w notesWorld, inv string) bool {
			return inv == "N4-store-note-invisible" &&
				(w.placement == placeBoth || (w.placement == placeFollowed && w.arrival))
		},
	},
	{
		"X10", "Hide and Delete clear the highlight unconditionally, including one they do not own",
		func(w notesWorld, inv string) bool {
			return inv == "N1-foreign-mark-destroyed" && w.foreignHL &&
				(w.verb == verbHide || w.verb == verbDelete)
		},
	},
}

// knownOriginIncoherent is the same pin for the highlight-origin enumeration.
var knownOriginIncoherent = []pinnedOrigin{
	{
		"X4", "turning notes off keeps the highlight the note put there",
		func(o hlOrigin, e hlEvent, inv string) bool {
			return inv == "N1-orphan-highlight" && e == evNotesOff
		},
	},
	{
		"X10", "deleting a note clears a highlight that was never the note's",
		func(o hlOrigin, e hlEvent, inv string) bool {
			return inv == "N1-foreign-mark-destroyed" && e == evNoteDeleted && o != fromNote
		},
	},
	{
		"X11", "the highlight has no version frame, so a switch leaves it in the old numbering",
		func(o hlOrigin, e hlEvent, inv string) bool {
			return inv == "N7-stale-frame" && e == evSwitchVersion && o != fromNote
		},
	},
}

type pinnedOrigin struct {
	name   string
	what   string
	covers func(o hlOrigin, e hlEvent, inv string) bool
}

// --- enumeration 1: the notes space -----------------------------------------

// TestNotesStateSpace walks the cross-product and asserts N1-N6 from
// docs/NOTES_STATE.md. Anything that violates one must be pinned, by name.
func TestNotesStateSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	// Pin the clock: noteFromAnotherTranslation orders candidates by Received
	// (notes_store.go:202-207), so a real clock would make which note follows
	// depend on how fast the test ran.
	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()

	unexplained := []string{}
	hits := map[string]int{}
	seen, skipped, total := 0, 0, 0

	for _, featureOn := range []bool{true, false} {
		for _, placement := range []notePlacement{placeNone, placeOwn, placeFollowed, placeBoth} {
			for _, collapsed := range []bool{false, true} {
				for _, foreignHL := range []bool{false, true} {
					for _, arrival := range []bool{false, true} {
						for _, verb := range []noteVerb{verbNone, verbHide, verbShow, verbDelete, verbNotesOff} {
							w := notesWorld{featureOn, placement, collapsed, foreignHL, arrival, verb}
							seen++
							obs, offered := runNotesFlow(t, w)
							if !offered {
								skipped++
								continue
							}
							for _, inv := range checkNotesInvariants(w, obs) {
								total++
								named := ""
								for _, d := range knownIncoherent {
									if d.covers(w, inv) {
										named = d.name
										hits[d.name]++
										break
									}
								}
								if named == "" {
									unexplained = append(unexplained, w.id()+" | "+inv)
								}
							}
						}
					}
				}
			}
		}
	}

	if len(unexplained) > 0 {
		sort.Strings(unexplained)
		t.Errorf("NEW incoherent states — %d violations no defect in docs/NOTES_STATE.md accounts for.\n"+
			"Each is a combination where a mark has no meaning, a verb missed, a note was substituted,\n"+
			"or a stored note is invisible:\n  %s",
			len(unexplained), strings.Join(unexplained, "\n  "))
	}
	for _, d := range knownIncoherent {
		if hits[d.name] == 0 {
			t.Errorf("%s is FIXED (%s): it no longer accounts for any violation.\n"+
				"Strike it from knownIncoherent AND from docs/NOTES_STATE.md — a pinned list that "+
				"outlives its defect makes the next reader trust a document that lies.", d.name, d.what)
		}
	}
	names := make([]string, 0, len(hits))
	for _, d := range knownIncoherent {
		names = append(names, fmt.Sprintf("%s×%d", d.name, hits[d.name]))
	}
	t.Logf("enumerated %d states (%d skipped: that surface offers no such verb there); "+
		"%d violations, all attributed: %s",
		seen, skipped, total, strings.Join(names, " "))
}

// notesObs is everything the reader and the store can be asked about, at the two
// moments that matter: right after the verb, and after the next navigation —
// which is when a store that disagrees with the mirror finally shows.
type notesObs struct {
	shownText string // what was on screen when the reader reached for the verb
	shownKey  string // the store key the mirror said that note lived under

	text string // immediately after the verb
	min  bool
	hlOn bool

	afterText string // after the next navigation re-derives from the store
	afterMin  bool
	afterHLOn bool

	before map[string]SharedNote // the store, before the verb
	after  map[string]SharedNote // the store, after
	mapped int                   // notes in the store that belong to this passage HERE
}

// runNotesFlow drives the REAL functions for one combination: the store helpers,
// applyNoteForCurrentChapter, the three verbs, and addRecentChapter as the
// navigation. It reports `offered=false` for a verb no surface would present —
// the banner's Hide/Delete exist only when ActiveNote is set (notes_banner.go:38),
// and the iOS menu's pair is gated on gHasNote (reading_ios.go:2005-2011). Driving
// them anyway would invent failures the app does not have, which is the harness
// artefact share_link_flow_test.go warns about in its own comment.
func runNotesFlow(t *testing.T, w notesWorld) (notesObs, bool) {
	t.Helper()
	var obs notesObs

	deleteAllNotes(appPrefs())
	setNotesEnabled(true) // seed with the feature on, then set the world's value

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}

	// The exact-key note carries the world's collapsed flag; the followed one in
	// placeBoth stays expanded, which is the masking case (COLLAPSED_MASK / X7).
	own := SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "note under web", Minimized: w.collapsed}
	other := SharedNote{VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Text: "note under bsb"}
	switch w.placement {
	case placeOwn:
		saveNote(appPrefs(), own)
	case placeFollowed:
		other.Minimized = w.collapsed
		saveNote(appPrefs(), other)
	case placeBoth:
		saveNote(appPrefs(), other)
		saveNote(appPrefs(), own)
	}

	setNotesEnabled(w.featureOn)

	// Derive. With a foreign highlight, use the REAL writer that puts one there —
	// goToVerseRange is what the verse of the day, cross-references and the Go-to
	// box all call (verse_of_day.go:275-292) — so the don't-clobber guard at
	// notes_store.go:327-330 sees exactly what it sees in the app.
	if w.foreignHL {
		goToVerseRange(st, "John", 3, 1, 1)
	} else {
		addRecentChapter(st, "John", 3)
	}

	// A note-bearing link landing on the chapter the reader is already on. This
	// is the one event that writes the mirror BY HAND rather than deriving it
	// (share_link_open.go:308-313) — three fields of four — which is why it
	// belongs on the cross-product and not in a case somebody thought of.
	//
	// With notes OFF the link never reaches applyShareTarget at all: it takes the
	// offer card (share_link_open.go:62-65), which mutates nothing. Driving the
	// arrival anyway would invent a state the app does not have, so those cells
	// report as not-offered.
	if w.arrival {
		if !w.featureOn {
			return obs, false
		}
		applyShareTarget(st, ShareTarget{
			VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Note: "note from the link",
		})
	}

	obs.shownText = st.ActiveNote
	obs.shownKey = st.noteStoreVersion()
	obs.before = readNotes(appPrefs())

	// Which stored notes belong to THIS passage as the reader is reading it? The
	// reading view can show at most one, whatever this number is.
	for _, n := range obs.before {
		if n.Book != "John" || n.Chapter != 3 {
			continue
		}
		obs.mapped++
	}

	offered := true
	switch w.verb {
	case verbHide, verbShow, verbDelete:
		// No note on screen, no verb on screen.
		offered = w.featureOn && st.ActiveNote != ""
	}
	if !offered {
		return obs, false
	}

	switch w.verb {
	case verbHide:
		hideCurrentNote(st)
	case verbShow:
		restoreCurrentNote(st)
	case verbDelete:
		dropCurrentNote(st)
	case verbNotesOff:
		// The "Keep them" answer: ai_settings.go:498-501 runs exactly this pair
		// and nothing else. clearLiveNote is on the DELETE branch only.
		setNotesEnabled(false)
	}

	obs.text, obs.min, obs.hlOn = st.ActiveNote, st.NoteMinimized, st.HasHighlightedVerse
	obs.after = readNotes(appPrefs())

	addRecentChapter(st, "John", 3) // the next navigation
	obs.afterText, obs.afterMin, obs.afterHLOn = st.ActiveNote, st.NoteMinimized, st.HasHighlightedVerse
	return obs, true
}

// checkNotesInvariants returns the names of the invariants this state violates.
// Each is stated as a property of the reader's experience, not of a field.
func checkNotesInvariants(w notesWorld, o notesObs) []string {
	var bad []string

	// N1 — no mark without a meaning. A highlight is on screen and nothing on
	// screen explains it: no note, and no other origin that put it there.
	if o.afterHLOn && o.afterText == "" && !w.foreignHL {
		bad = append(bad, "N1-orphan-highlight")
	}
	// N1 again, the other way: a mark that belongs to somebody else must survive
	// a verb aimed at the note. The derive goes out of its way not to clobber a
	// foreign highlight (notes_store.go:327-330); the verbs then destroy it
	// unconditionally (:395, :422).
	if w.foreignHL && (w.verb == verbHide || w.verb == verbDelete) && !o.hlOn {
		bad = append(bad, "N1-foreign-mark-destroyed")
	}

	// N2 — a verb reaches what the reader aimed it at.
	switch w.verb {
	case verbDelete:
		if o.shownText != "" && storeHolds(o.after, o.shownText) {
			bad = append(bad, "N2-delete-missed")
		}
		if lost := lostOthers(o.before, o.after, o.shownText); lost != "" {
			bad = append(bad, "N2-delete-collateral")
		}
	case verbHide:
		if o.shownText != "" && !minimizedInStore(o.after, o.shownText) {
			bad = append(bad, "N2-hide-missed")
		}
		if flippedOthers(o.before, o.after, o.shownText) {
			bad = append(bad, "N2-hide-collateral")
		}
	case verbShow:
		if o.shownText != "" && minimizedInStore(o.after, o.shownText) {
			bad = append(bad, "N2-show-missed")
		}
		if flippedOthers(o.before, o.after, o.shownText) {
			bad = append(bad, "N2-show-collateral")
		}
	}

	// N3 — no silent substitution. Turning the feature off is the one event
	// entitled to change what is on screen to nothing; nothing is entitled to
	// change it to somebody else's words.
	if w.verb != verbNotesOff && o.afterText != "" && o.afterText != o.shownText {
		bad = append(bad, "N3-substituted")
	}

	// N4 — nothing in the store is invisible from the reading view. The reading
	// view shows at most one note; anything beyond that has no separator, no
	// count, and no trace.
	if w.featureOn && w.verb != verbNotesOff && o.mapped > 1 {
		bad = append(bad, "N4-store-note-invisible")
	}

	// N5 — an explicit minimize is honoured, and an explicit restore sticks.
	if w.verb == verbHide && o.afterText != "" && !o.afterMin {
		bad = append(bad, "N5-hide-not-honoured")
	}
	if w.verb == verbShow && o.afterText != "" && o.afterMin {
		bad = append(bad, "N5-show-not-honoured")
	}

	// N6 — the mirror agrees with the store, under the key the verbs address.
	if o.afterText != "" && !storeHolds(o.after, o.afterText) {
		bad = append(bad, "N6-mirror-only")
	}
	return bad
}

func storeHolds(notes map[string]SharedNote, text string) bool {
	for _, n := range notes {
		if n.Text == text {
			return true
		}
	}
	return false
}

func minimizedInStore(notes map[string]SharedNote, text string) bool {
	for _, n := range notes {
		if n.Text == text {
			return n.Minimized
		}
	}
	return false // not there at all is not "minimized"
}

// lostOthers names a note that disappeared from the store although the reader
// aimed Delete at a different one.
func lostOthers(before, after map[string]SharedNote, aimedAt string) string {
	for k, n := range before {
		if n.Text == aimedAt {
			continue
		}
		if _, still := after[k]; !still {
			return k
		}
	}
	return ""
}

// flippedOthers reports a note whose collapsed state changed although the reader
// aimed Hide or Show at a different one.
func flippedOthers(before, after map[string]SharedNote, aimedAt string) bool {
	for k, n := range before {
		if n.Text == aimedAt {
			continue
		}
		if a, still := after[k]; still && a.Minimized != n.Minimized {
			return true
		}
	}
	return false
}

// --- enumeration 2: the highlight's origin ----------------------------------

// hlOrigin is THE VARIABLE THE APP DOES NOT RECORD. Five writers put a highlight
// into AppState (docs/NOTES_STATE.md, "the variable nobody has written down") and
// nothing distinguishes them afterwards, so ownership is inferred from a bare
// verse number. This enumeration exists to state the invariant that inference is
// standing in for.
type hlOrigin int

const (
	fromNothing hlOrigin = iota
	fromNote
	fromSearch     // openSearchResultRange, state.go:772-778
	fromVerseOfDay // goToVerseRange, verse_of_day.go:282-286
	fromLinkSpan   // applyShareTarget, share_link_open.go:271-276
)

func (o hlOrigin) String() string {
	return [...]string{"nothing", "note", "search", "verse-of-day", "link-span"}[o]
}

type hlEvent int

const (
	evNavigate hlEvent = iota
	evNoteDeleted
	evNotesOff
	evSwitchVersion
)

func (e hlEvent) String() string {
	return [...]string{"navigate", "note-deleted", "notes-off", "switch-version"}[e]
}

// TestHighlightOriginSpace asserts N1 and N7. It uses Romans 14 because that is
// where the numbering actually diverges: MapVerse puts WEB Romans 14:24 at BSB
// 16:25 (the doxology, documented at share_link_open.go:108-111), so a highlight
// that survives a switch unchanged is demonstrably in the wrong frame rather
// than merely unproven.
func TestHighlightOriginSpace(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	defer deleteAllNotes(appPrefs())
	defer setNotesEnabled(true)

	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()

	var unexplained []string
	hits := map[string]int{}
	seen, total := 0, 0

	for _, origin := range []hlOrigin{fromNothing, fromNote, fromSearch, fromVerseOfDay, fromLinkSpan} {
		for _, ev := range []hlEvent{evNavigate, evNoteDeleted, evNotesOff, evSwitchVersion} {
			seen++
			for _, inv := range runOriginFlow(t, origin, ev) {
				total++
				named := ""
				for _, d := range knownOriginIncoherent {
					if d.covers(origin, ev, inv) {
						named = d.name
						hits[d.name]++
						break
					}
				}
				if named == "" {
					unexplained = append(unexplained,
						fmt.Sprintf("origin=%s event=%s | %s", origin, ev, inv))
				}
			}
		}
	}

	if len(unexplained) > 0 {
		sort.Strings(unexplained)
		t.Errorf("NEW incoherent states in the highlight-origin space — %d violations "+
			"docs/NOTES_STATE.md does not account for:\n  %s",
			len(unexplained), strings.Join(unexplained, "\n  "))
	}
	for _, d := range knownOriginIncoherent {
		if hits[d.name] == 0 {
			t.Errorf("%s is FIXED (%s). Strike it from knownOriginIncoherent AND from docs/NOTES_STATE.md.",
				d.name, d.what)
		}
	}
	names := make([]string, 0, len(hits))
	for _, d := range knownOriginIncoherent {
		names = append(names, fmt.Sprintf("%s×%d", d.name, hits[d.name]))
	}
	t.Logf("enumerated %d origin/event states; %d violations, all attributed: %s",
		seen, total, strings.Join(names, " "))
}

// romansBible is the sample canon plus the two chapters the Romans doxology moves
// between, so the frame invariant has something real to measure against.
// romansBible is the sample canon plus all sixteen chapters of Romans. All
// sixteen, not just the two that matter: GetChaptersForBook returns a COUNT
// (bible.go:266-268), and applyShareTarget clamps a link's chapter against it
// (share_link_open.go:250-252), so a sparse book would silently redirect every
// arrival to chapter 2 and the enumeration would measure the wrong passage.
func romansBible() *BibleData {
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	for c := 1; c <= 16; c++ {
		bd.Verses["Romans"][c] = []Verse{{BookName: "Romans", Chapter: c, Verse: 1, Text: "romans"}}
	}
	bd.Verses["Romans"][14] = []Verse{
		{BookName: "Romans", Chapter: 14, Verse: 23, Text: "whatever is not of faith is sin"},
		{BookName: "Romans", Chapter: 14, Verse: 24, Text: "now to him who is able"},
	}
	bd.Verses["Romans"][16] = []Verse{
		{BookName: "Romans", Chapter: 16, Verse: 25, Text: "now to him who is able"},
	}
	return bd
}

func runOriginFlow(t *testing.T, origin hlOrigin, ev hlEvent) []string {
	t.Helper()
	deleteAllNotes(appPrefs())
	setNotesEnabled(true)

	bd := romansBible()
	st := &AppState{
		Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}

	// A note on every chapter, so "the note is deleted" is a real event in every
	// cell. It sits on v23, which MapVerse leaves alone; the FOREIGN origins take
	// v24, which moves. That separation is what lets one assertion tell an
	// unrenumbered mark from a correctly-placed one.
	saveNote(appPrefs(), SharedNote{VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 23, Text: "the note"})
	addRecentChapter(st, "Romans", 14)

	switch origin {
	case fromNothing:
		clearHighlightedVerse(st)
	case fromNote:
		// applyNoteForCurrentChapter has already put it on v23.
	case fromSearch:
		openSearchResultRange(st, Verse{BookName: "Romans", Chapter: 14, Verse: 24}, 0)
	case fromVerseOfDay:
		goToVerseRange(st, "Romans", 14, 24, 24)
	case fromLinkSpan:
		applyShareTarget(st, ShareTarget{VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 24})
	}

	hadHL := st.HasHighlightedVerse
	fromVer := st.CurrentVersion

	switch ev {
	case evNavigate:
		addRecentChapter(st, "Romans", 14)
	case evNoteDeleted:
		if st.ActiveNote != "" {
			dropCurrentNote(st)
		} else {
			deleteNote(appPrefs(), "web", "Romans", 14)
		}
		addRecentChapter(st, "Romans", 14)
	case evNotesOff:
		setNotesEnabled(false)
		addRecentChapter(st, "Romans", 14)
	case evSwitchVersion:
		v, ok := versionByID("bsb")
		if !ok {
			t.Fatal("bsb is not registered; the frame invariant needs two translations")
		}
		applyLoadedVersion(st, v, romansBible(), modeReal)
	}

	var bad []string

	// N1 — no mark without a meaning. A note's highlight must go when its note
	// goes; a mark somebody else's action put there must NOT.
	switch origin {
	case fromNothing, fromNote:
		// A live expanded note IS a meaning, so these two ask the same question.
		// fromNothing earns its row anyway: it shows that a highlight the reader
		// dismissed comes back on the next navigation while its note is still
		// expanded — correct, but only because the note re-asserts it, which is
		// the same mechanism that strands one when the note goes.
		if st.HasHighlightedVerse && st.ActiveNote == "" {
			bad = append(bad, "N1-orphan-highlight")
		}
	default:
		if hadHL && !st.HasHighlightedVerse && ev != evSwitchVersion {
			bad = append(bad, "N1-foreign-mark-destroyed")
		}
	}

	// N7 — one ruler. After a version switch a surviving highlight must be in the
	// numbering of the translation now being read.
	if ev == evSwitchVersion && st.HasHighlightedVerse {
		ch, v, res := MapVerse(fromVer, st.CurrentVersion, st.HighlightedBook, st.HighlightedChapter, st.HighlightedVerse)
		if res != verseMapExact && (ch != st.HighlightedChapter || v != st.HighlightedVerse) {
			bad = append(bad, "N7-stale-frame")
		}
	}
	return bad
}

// The pinned lists must stay honest about themselves: every entry has to name an
// incoherent state the document actually describes, or the two have drifted.
func TestPinnedNamesRealIncoherentStates(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range knownIncoherent {
		if !strings.HasPrefix(d.name, "X") || d.what == "" {
			t.Errorf("%q does not name an incoherent state in docs/NOTES_STATE.md", d.name)
		}
		if seen[d.name] {
			t.Errorf("%s is pinned twice in knownIncoherent; two predicates for one defect "+
				"means the first one silently swallows the second's region", d.name)
		}
		seen[d.name] = true
	}
	seen = map[string]bool{}
	for _, d := range knownOriginIncoherent {
		if !strings.HasPrefix(d.name, "X") || d.what == "" {
			t.Errorf("%q does not name an incoherent state in docs/NOTES_STATE.md", d.name)
		}
		if seen[d.name] {
			t.Errorf("%s is pinned twice in knownOriginIncoherent", d.name)
		}
		seen[d.name] = true
	}
}

// --- the single-state facts the cross-products cannot reach -----------------

// X8 and X9: a BARE chapter link (no verse) landing on a chapter that already
// carries a note. Not on the cross-product because "the link carries a verse" is
// a property of the link, not of the chapter's notes — and adding the axis would
// double the space to say this once.
func TestBareLinkStripsTheNotesHighlightAndLeavesAGhost(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	saveNote(appPrefs(), SharedNote{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "look at 16"})
	addRecentChapter(st, "John", 3)
	if !st.HasHighlightedVerse {
		t.Fatal("precondition: the note should have raised its highlight")
	}

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3}) // no verse, no note

	// X8 — the note survives (the share_link_open.go:308 guard) but its mark does not.
	if st.ActiveNote == "" {
		t.Fatal("the bare link blanked the stored note; that is a different defect from X8")
	}
	if st.HasHighlightedVerse {
		t.Error("X8 is FIXED: the note kept its highlight across a bare chapter link. " +
			"Strike X8 from docs/NOTES_STATE.md.")
	}
	// X9 — and the location fields it left behind.
	if st.HighlightedBook == "" && st.HighlightedChapter == 0 && st.HighlightedVerse == 0 {
		t.Error("X9 is FIXED: the else arm now clears the location too. " +
			"Strike X9 from docs/NOTES_STATE.md.")
	}
}

// X9, the second route: openSearchResultRange sets the three location fields and
// then derives the flag from the verse, so a CHAPTER-LEVEL note (VerseLo 0)
// tapped in the browser lands with a location and no flag.
func TestChapterLevelNoteTappedInTheBrowserLeavesAGhost(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	n := SharedNote{VersionID: "web", Book: "John", Chapter: 3, Text: "about the whole chapter"}
	saveNote(appPrefs(), n)
	openNote(st, n)

	if st.HasHighlightedVerse {
		t.Fatal("a chapter-level note should not raise a verse highlight; that is a new state")
	}
	if st.HighlightedBook == "" && st.HighlightedChapter == 0 {
		t.Error("X9 is FIXED on the browser route: the location is cleared with the flag. " +
			"Strike that half of X9 from docs/NOTES_STATE.md.")
	}
}

// --- the single-state facts the cross-product cannot reach ------------------

// X3: a note stored and evicted by the same write. The cross-product cannot see
// this — it needs the store already at notesMax, which is a capacity axis, not a
// state axis. Pinned here so a fix to writeNotes turns this red.
func TestArrivingNoteEvictedByItsOwnSave(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	for i := 0; i < notesMax; i++ {
		saveNote(appPrefs(), SharedNote{VersionID: "web", Book: "Psalms", Chapter: i + 1, Text: fmt.Sprintf("filler %d", i)})
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	// "web|John|3" sorts below every "web|Psalms|…", so this key is in the head
	// writeNotes discards. The link names the translation already open, so no
	// switch is involved and the arrival is applied rather than parked.
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Note: "just arrived"})

	if st.ActiveNote != "just arrived" {
		t.Fatalf("precondition: the arriving note should be on screen, got %q", st.ActiveNote)
	}
	if _, stored := readNotes(appPrefs())["bsb|John|3"]; stored {
		t.Error("X3 is FIXED: the arriving note survived its own write. " +
			"Strike X3 from docs/NOTES_STATE.md and delete this test's expectation.")
	}
	addRecentChapter(st, "John", 3)
	if st.ActiveNote != "" {
		t.Error("X3 is FIXED: the note survived the next navigation. Update docs/NOTES_STATE.md.")
	}
}

// HL_FRAME: the highlight is not renumbered on a version switch, though the note
// beside it is. Stated once, plainly, because it is the clearest evidence that
// the highlight has no version frame at all.
func TestHighlightKeepsThePreviousTranslationsNumbering(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	ch, v, res := MapVerse("web", "bsb", "Romans", 14, 24)
	if res == verseMapExact {
		t.Skip("the versification table no longer moves Romans 14:24; pick another divergence")
	}
	bd := romansBible()
	st := &AppState{
		Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	goToVerseRange(st, "Romans", 14, 24, 24)

	other, ok := versionByID("bsb")
	if !ok {
		t.Fatal("bsb is not registered")
	}
	applyLoadedVersion(st, other, romansBible(), modeReal)

	if !st.HasHighlightedVerse {
		t.Error("HL_FRAME is FIXED: the switch cleared the highlight. Update docs/NOTES_STATE.md.")
		return
	}
	if st.HighlightedChapter == ch && st.HighlightedVerse == v {
		t.Error("HL_FRAME is FIXED: the highlight was renumbered. Update docs/NOTES_STATE.md.")
		return
	}
	// Still in the old frame. The note on the same passage HAS been mapped.
	if st.HighlightedChapter != 14 || st.HighlightedVerse != 24 {
		t.Errorf("unexpected: the highlight moved to %d:%d, which is neither frame",
			st.HighlightedChapter, st.HighlightedVerse)
	}
}

// N8, the temporary cap, stated as the thing the model cannot yet say. This is
// the only assertion here about INTENDED behaviour, and it is written to fail
// the moment the model can express it — at which point the SET work has landed
// and docs/NOTES_STATE.md's rework section is history rather than a plan.
func TestTheAtMostOneExpandedCapIsNotRepresentable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	for _, id := range []string{"web", "bsb", "webc"} {
		saveNote(appPrefs(), SharedNote{VersionID: id, Book: "John", Chapter: 3, VerseLo: 16, Text: "note under " + id})
	}
	addRecentChapter(st, "John", 3)

	shown, total := browsableNotes(st)
	if total != 3 || len(shown) != 3 {
		t.Fatalf("precondition: the browser should list all three, got %d of %d", len(shown), total)
	}
	// The reading view carries one note in four flat fields, so "which of the
	// three is expanded" is not a question AppState can be asked. When it can,
	// this test should be deleted along with the rework section.
	if st.ActiveNote == "" {
		t.Fatal("precondition: one of the three should be live")
	}
	expanded := 0
	for _, n := range readNotes(appPrefs()) {
		if !n.Minimized {
			expanded++
		}
	}
	if expanded != 3 {
		t.Errorf("the cap is now enforceable somewhere: %d of 3 notes are expanded at rest. "+
			"If the SET model has landed, delete this test and update docs/NOTES_STATE.md.", expanded)
	}
}
