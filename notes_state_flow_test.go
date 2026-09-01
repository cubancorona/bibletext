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

	"fyne.io/fyne/v2"
	"testing"

	"fyne.io/fyne/v2/test"
)

// enumerationChapter is John 3 as the ENUMERATION reads it, and it exists
// because the sample data's John 3 is a SINGLE VERSE (v16). One verse is one
// paragraph, so chapterNoteGroups could never return more than one group and
// PILLS_SET — the state the per-paragraph pills exist for — was not enumerated
// at all while the axis was being added for it. Measured: sample John 3 = 1
// verse, 1 paragraph, and every received note the harness files lands on v16.
func enumerationChapter() []Verse {
	long := "This verse is deliberately long so that the paragraph splitter reaches " +
		"its character threshold and breaks at the next sentence ending, which is here."
	out := make([]Verse, 0, 20)
	for i := 1; i <= 20; i++ {
		out = append(out, Verse{BookName: "John", Book: "John", Chapter: 3, Verse: i, Text: long})
	}
	return out
}

// enumerationSpreadVerse is a verse in a DIFFERENT paragraph from 16, where the
// harness files every other received note.
func enumerationSpreadVerse() int {
	for _, para := range groupVersesIntoParagraphs(enumerationChapter()) {
		if !paraCarriesVerse(para, 16) {
			return para[0].Verse
		}
	}
	return 0
}

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

// noteFocusAxis is the SESSION-FOCUS axis S7 added to the model
// (AppState.noteFocus, notes_plan.go). The design's recorded risk was that
// without enumerating it the rework would have "moved the unwatched variable
// rather than removed it" — so the harness walks it: no choice made, the open
// note explicitly closed, the exact-key note opened, a followed note opened.
type noteFocusAxis int

const (
	focusUnset        noteFocusAxis = iota // the default rule applies
	focusNoneAx                            // the reader closed the open note
	focusExactKey                          // the reader opened the exact-key note
	focusFollowedNote                      // the reader opened a followed note
	focusOwnAx                             // the reader opened one of THEIR OWN notes
)

func (f noteFocusAxis) String() string {
	return [...]string{"unset", "none", "exact", "followed", "own"}[f]
}

// notesWorld is the slice of the world the notes subsystem branches on. Kept as
// its own type so the enumeration is readable, and so adding a variable to the
// subsystem means adding it HERE, where the cross-product picks it up.
// unplacedAxis is the unplaced-note axis: no such note, the R4 kind the delta
// tables produce, or a run beyond the chapter's own end.
type unplacedAxis int

const (
	unplacedNo unplacedAxis = iota
	unplacedR4
	unplacedBeyond
)

func (u unplacedAxis) String() string {
	return [...]string{"no", "r4", "beyond"}[u]
}

type notesWorld struct {
	featureOn bool
	placement notePlacement
	collapsed bool          // the note under the exact key is stored Minimized
	foreignHL bool          // a highlight from another origin is already on the chapter
	focus     noteFocusAxis // the reader's session focus, set after the derive
	arrival   bool          // a note-bearing link landed on this chapter after the derive
	verb      noteVerb

	// ownNote — the reader has a note of THEIR OWN on this chapter. It is a
	// separate axis from placement because an own note is not a member of the
	// received set (chapterPlan.Own is a slot): it changes what the sticker is
	// showing without changing N, which is exactly the combination that let the
	// received set fall off the page (X15).
	ownNote bool

	// spread — a second received note, in a DIFFERENT paragraph from the one at
	// v16. Without it chapterNoteGroups tops out at one group and the pills draw
	// only in the own-note case, so the multi-paragraph states go unvisited.
	spread bool

	// unplaced — a received note on this BOOK that this translation cannot
	// place. Without this axis the who-without-text tuple — the state whose
	// mis-draw is an empty sender bubble — was UNREACHABLE, so N13's guard on
	// it was dead: measured, a mutation forcing Pill=false on that tuple
	// passed the whole walk. It also reaches the chapter-top band (Verse 0)
	// and the anchorless arrival arm, which no placed-note cell can.
	//
	// Two flavours, because they exercise two different absences: R4 is the
	// delta tables saying a verse has no home here (NKJV John 5:4 read under
	// WEB), and BEYOND is a run naming a verse the chapter's own data does
	// not carry (an inflated link). The first walk with BEYOND found the
	// anchor machinery trusting such runs verbatim — a placed note anchored
	// on a verse with no line, N11 at 3,404 cells — which is why the flavour
	// stays enumerated after the fix: it is the tripwire against re-trusting.
	unplaced unplacedAxis

	// pills — notesPillPerParagraph, the presentation gate. Off is every
	// shipped build and all three native surfaces; on is the styled pane's
	// pill row. It belongs on the cross-product because it decides WHICH of the
	// three representations is available, and N9 is about which one is in force.
	pills bool
}

func (w notesWorld) id() string {
	return fmt.Sprintf("on=%v place=%s collapsed=%v foreignHL=%v focus=%s arrival=%v verb=%s own=%v spread=%v unplaced=%s pills=%v",
		w.featureOn, w.placement, w.collapsed, w.foreignHL, w.focus, w.arrival, w.verb, w.ownNote, w.spread, w.unplaced, w.pills)
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

// X10 was struck on 2026-08-15 by S1 (mark.go). Hide and Delete used to clear
// the highlight unconditionally; ownership is RECORDED now, so clearMarkFromNote
// drops only a mark hlNote placed. It covered 28 cells here and 3 in the origin
// space — the largest single defect in the subsystem after X7.
//
// X1 and X2 were struck on 2026-08-15, fixed by 69d3f4ab3 ("Write the live
// note's four fields as one value"): both verbs now address the note
// NoteVersionID names because every arrival path writes that field. The
// enumeration proved them dead — each covered zero violations — and this list
// must not outlive its defects.
//
// Striking them is what made X12 visible, and the two facts belong together:
// while Delete was missing the arriving note, it was also masking the
// substitution that follows a successful one.
//
// X5 was struck with S5 (the scrapbook store): Hide, Show and Delete all
// address StoredNote.ID — the identity the derive hands the mirror — so the
// two verbs of one pair can no longer address different objects. It covered
// 4 cells. X13, its cross-chapter sibling (pinned in
// notes_crosschapter_test.go rather than here), died of the same change.
//
// X6, X7, X12 and X14 were struck with S8 (the surfaces consume the plan).
// All four were the arity-1 DISPLAY's debts — one note drawn, the rest
// invisible (X7, 224 cells), and three routes to a different note silently
// taking the drawn one's place (X6×32, X12×24, X14×12) — and the set display
// is what retires them: every placed note is a bubble or a chip and every
// unplaced note a chip with its reason (N4 judged over the plan), so a change
// of which note is EXPANDED is a change among notes already on screen, not a
// substitution (N3 judged over the plan, below). The Apple sticker stays
// arity-1 by design and stays honest by the count line in its own text —
// richness differs per platform, truth does not.
//
// X4 and X11 — the LAST TWO — were struck on 2026-08-18, and the lists below
// are EMPTY: zero named violations in both spaces, for the first time since
// the enumeration was written. X4 (55 cells here, 1 in the origin space) died
// of turnNotesOff + the off-branch's clearMarkFromNote: every route to "off"
// now puts out exactly the mark the live note owns and no other. X11 (3
// cells) died of renumberMarkForVersion (mark.go): the version switch maps
// the mark's span into the new translation through the notes' own anchor
// machinery — the first use VerseSpan.VersionID has ever had — and clears it
// on anything but a clean landing. An empty list is still load-bearing: any
// violation now fails as a NEW incoherent state, with nothing to hide behind.
var knownIncoherent = []pinnedDefect{
	{
		name: "X16",
		what: "an open own note leaves the received set represented nowhere, wherever there is no pill row",
		covers: func(w notesWorld, inv string) bool {
			// Every surface without a pill row: the three native ones, and the
			// styled pane with the gate off, which is every shipped build. The
			// sticker is busy with the reader's own note, that note carries no
			// count of the received set by design, and nothing else on the page
			// speaks for it. With the gate ON the pills speak for it and these
			// cells come out clean — which is the fix, measured.
			return strings.HasPrefix(inv, "N9-set-unrepresented") &&
				w.focus == focusOwnAx && !w.pills
		},
	},
}

// knownOriginIncoherent is the same pin for the highlight-origin enumeration —
// also empty since X4 and X11 were struck. See the note above.
var knownOriginIncoherent = []pinnedOrigin{}

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

	// Pin the clock: the derive orders candidates by Received, so a real clock
	// would make which note follows depend on how fast the test ran.
	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()

	// The enumeration drives the presentation gate; put it back.
	origPills := notesPillPerParagraph
	defer func() { notesPillPerParagraph = origPills }()

	unexplained := []string{}
	hits := map[string]int{}
	seen, skipped, total := 0, 0, 0

	for _, featureOn := range []bool{true, false} {
		for _, placement := range []notePlacement{placeNone, placeOwn, placeFollowed, placeBoth} {
			for _, collapsed := range []bool{false, true} {
				for _, foreignHL := range []bool{false, true} {
					for _, focus := range []noteFocusAxis{focusUnset, focusNoneAx, focusExactKey, focusFollowedNote, focusOwnAx} {
						for _, arrival := range []bool{false, true} {
							for _, verb := range []noteVerb{verbNone, verbHide, verbShow, verbDelete, verbNotesOff} {
								for _, ownNote := range []bool{false, true} {
									for _, spread := range []bool{false, true} {
										for _, unplaced := range []unplacedAxis{unplacedNo, unplacedR4, unplacedBeyond} {
											for _, pills := range []bool{false, true} {
												w := notesWorld{featureOn, placement, collapsed, foreignHL, focus, arrival, verb, ownNote, spread, unplaced, pills}
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
	// The COUNTS are asserted, not merely logged. If navigation stops resetting
	// focus, per-defect totals can diverge from docs/NOTES_STATE.md while
	// attribution and liveness alone let a rule rot as long as some cell still
	// hits each pin.
	// The counts are deterministic, so drift means either a fix (strike and
	// re-measure, per the contract) or a regression (this failure).
	// NOTES-SPACE counts only. docs/NOTES_STATE.md's headline figures COMBINE
	// this enumeration with the origin-space one (X4 read ×56 there: 55 cells
	// here + 1 there), and the first version of this map copied the combined
	// number and failed its own first run. Kept as a warning: when updating
	// after a re-measure, take the per-space split from the run output, not
	// the doc's combined line. EMPTY since the sixth pass — zero named
	// violations — and the set-equality assertion above is what now holds it
	// there.
	expectedHits := map[string]int{"X16": 504}
	for name, want := range expectedHits {
		if hits[name] != want {
			t.Errorf("%s covers %d cells, docs/NOTES_STATE.md records %d — re-measure "+
				"and update BOTH, or find the regression", name, hits[name], want)
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
	// st is the world runNotesFlow built, so a second sweep can judge the same
	// states without a second copy of the seeding. The conformance sweep
	// (notes_chrome_conformance_test.go) reads it; nothing else should.
	st *AppState

	shownText string // what was on screen when the reader reached for the verb
	shownID   uint64 // the identity the mirror said the verbs would address

	shownWasOwn bool // the note on screen was the reader's OWN (chapterPlan.Own)

	text string // immediately after the verb
	min  bool
	hlOn bool

	afterText string // after the next navigation re-derives from the store
	afterMin  bool
	afterHLOn bool

	before []StoredNote // the store, before the verb
	after  []StoredNote // the store, after

	// The chapter PLAN (notes_plan.go), snapshotted at three moments: what
	// the reader was looking at when they reached for the verb, right after
	// the verb, and after the next navigation. Since S8 the plan IS the
	// reading surface's model (the banner draws the whole set), so N3 and N4
	// are judged over these snapshots, and the V-invariants hold the plan's
	// own coherence in every cell.
	snapShown planSnap
	snapVerb  planSnap
	snapNav   planSnap
}

// planSnap is one buildChapterPlan answer plus the facts its invariants are
// judged against, taken at the same instant.
type planSnap struct {
	plan         chapterPlan
	suppressed   bool // a live mark not owned by a note stood the notes down
	featureOn    bool
	passageNotes int // received notes filed on the passage, in the store, now

	// shownAs is HOW the received set is represented at this moment, read from
	// the product's own answer (receivedSetShownAs) rather than re-derived here
	// — a second derivation would drift from the pane and the enumeration would
	// then be checking itself. groups is what that answer was given.
	shownAs receivedShownAs
	groups  int

	// chrome is THE ONE COMPOSED VALUE every surface consumes
	// (chapterNoteChrome), snapshotted at the same instant as the plan so the
	// N11-N15 tripwires can hold its self-consistency in every cell. groupList
	// is the slice the count above summarises, kept because the band
	// assertions need the entries, not just how many there were.
	chrome    noteChrome
	groupList []noteParagraphGroup

	// liveKindMine is what the STORE says about the note the mirror names:
	// st.NoteID resolves to a Kind=mine record right now. Read through the
	// browsing list rather than findNoteByID, so the two accessors answering
	// differently — or the mirror naming a purged record — trips N12 instead
	// of hiding inside one shared lookup.
	liveKindMine bool

	// AND WHAT THE PANE ACTUALLY DREW. The three fields above are the model's
	// account of itself; these two are the styled pane's. Both are needed,
	// because the defect this axis was added for lives in the SEAM: the model
	// said the reader's own note was on screen while the pane, having zeroed its
	// geometry for the pills, drew nothing for it. An enumeration that reads
	// only the model cannot see that — measured: breaking the pane's guard
	// changed the model-only run not at all.
	//
	// 0.6ms a pane, so the whole space costs well under two seconds.
	paneSticker bool
	panePills   int
	ownFocused  bool
	activeNote  string
}

// withPane says whether to build the styled pane for this snapshot. Only the
// two moments N10 judges need it; the pre-verb snapshot reads none of the pane
// fields, and a pane costs 0.6ms across 12800 cells.
func takePlanSnap(st *AppState, withPane bool) planSnap {
	snap := planSnap{
		plan:       buildChapterPlan(st, appPrefs(), st.Bible),
		suppressed: notesSuppressed(st),
		featureOn:  notesFeatureOn(st),
	}
	// BOOK-scoped, not chapter-scoped: the plan lists placed notes for the
	// chapter and unplaced ones for the whole book, and in this harness every
	// John note is one or the other — so the book count is exactly "what the
	// plan must account for". The old chapter filter left the R4 seed
	// (John 5:4) invisible to N4 and let V3 miscount an unplaced chapter-3
	// note as "emptied".
	for _, n := range allNotesForBrowsing(appPrefs()) {
		if n.Kind == noteKindReceived && n.Book == "John" {
			snap.passageNotes++
		}
	}
	verses := st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter)
	snap.groupList = chapterNoteGroups(st, snap.plan, verses)
	snap.groups = len(snap.groupList)
	snap.shownAs = receivedSetShownAs(snap.plan, styledNoteFor(st), snap.groups)
	snap.chrome = chapterNoteChrome(st, snap.plan, verses)
	snap.ownFocused = isOwnLiveNote(st)
	snap.activeNote = st.ActiveNote
	if st.NoteID != 0 {
		for _, n := range allNotesForBrowsing(appPrefs()) {
			if n.ID == st.NoteID {
				snap.liveKindMine = n.Kind == noteKindMine
				break
			}
		}
	}
	if withPane && len(verses) > 0 {
		pane := newStyledReadingPane(st, verses)
		pane.Resize(fyne.NewSize(320, 900))
		snap.paneSticker = pane.noteGeom.present
		snap.panePills = len(pane.pillGeoms)
	}
	return snap
}

// runNotesFlow drives the REAL functions for one combination: the store helpers,
// applyNoteForCurrentChapter, the three verbs, and addRecentChapter as the
// navigation. It reports `offered=false` for a verb no surface would present —
// the verbs ride on the open bubble (the banner's, or the Apple sticker's,
// both projections of the same display note the mirror carries), and the iOS
// menu's pair is gated on gHasNote (reading_ios.go). Driving them anyway would
// invent failures the app does not have, which is the harness artefact
// share_link_flow_test.go warns about in its own comment.
func runNotesFlow(t *testing.T, w notesWorld) (notesObs, bool) {
	t.Helper()
	var obs notesObs

	deleteAllNotes(appPrefs())
	setNotesEnabled(true) // seed with the feature on, then set the world's value

	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	bd.Verses["John"][3] = enumerationChapter()
	// John 5 with its REAL shape: the WEB omits verse 4, and placement now
	// refuses a verse a loaded chapter demonstrably lacks — which is exactly
	// what makes the R4 seed below unplaceable HERE while staying a perfectly
	// ordinary note in the translation it was written under.
	john5 := make([]Verse, 0, 20)
	for i := 1; i <= 20; i++ {
		if i == 4 {
			continue
		}
		john5 = append(john5, Verse{BookName: "John", Book: "John", Chapter: 5, Verse: i,
			Text: "A verse of the fifth chapter, present in this translation."})
	}
	bd.Verses["John"][5] = john5
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}

	// The exact-version note carries the world's collapsed flag; the followed
	// one in placeBoth stays expanded, which is the masking case
	// (COLLAPSED_MASK / X7).
	own := StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "note under web", Minimized: w.collapsed}
	other := StoredNote{Kind: noteKindReceived, VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Text: "note under bsb"}
	switch w.placement {
	case placeOwn:
		addNote(appPrefs(), own)
	case placeFollowed:
		other.Minimized = w.collapsed
		addNote(appPrefs(), other)
	case placeBoth:
		addNote(appPrefs(), other)
		addNote(appPrefs(), own)
	}

	// The second received note, in another paragraph. Not offered on placeNone,
	// whose whole meaning is that the passage carries nothing.
	if w.spread {
		if w.placement == placeNone {
			return obs, false
		}
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: enumerationSpreadVerse(),
			Text: "a note in another paragraph"})
	}

	// The unplaced note: on this book, with no home in this translation. The
	// plan carries it in Unplaced, the who line grows its "not shown here"
	// arm, and on a chapter with nothing else the whole sticker is the
	// who-without-text pill. Orthogonal to placement: an unplaced note rides
	// every chapter of its book, including an otherwise empty one.
	switch w.unplaced {
	case unplacedR4:
		// NKJV John 5:4 read under WEB — a real versification hole: the NKJV
		// carries the verse, the WEB's own John 5 (loaded above, without a
		// v4) does not, so the mapped arm's existence test answers R4.
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "nkjv",
			Book: "John", Chapter: 5, VerseLo: 4,
			Text: "a note this translation cannot place"})
	case unplacedBeyond:
		// A same-version run naming a verse past the chapter's end — only the
		// text itself can refuse this one, which it now does.
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web",
			Book: "John", Chapter: 3, VerseLo: 999,
			Text: "a note beyond the chapter's end"})
	}

	// The reader's OWN note, on a DIFFERENT verse from the received ones so it
	// lands in its own paragraph where the sample chapter allows: what matters
	// is that it is a Kind=mine record, which the plan carries in its Own slot
	// rather than in Notes.
	if w.ownNote {
		nonce := make([]byte, noteNonceLen)
		nonce[0] = 42
		saveMyNote(appPrefs(), StoredNote{VersionID: "web", Book: "John", Chapter: 3,
			VerseLo: 1, Text: "a note of my own", Nonce: nonce})
	}

	setNotesEnabled(w.featureOn)

	// The presentation gate, restored by the caller's defer. Set BEFORE the
	// derive so every snapshot sees the same world.
	notesPillPerParagraph = w.pills

	// Derive. With a foreign highlight, use the REAL writer that puts one there —
	// goToVerseRange is what the verse of the day, cross-references and the Go-to
	// box all call (verse_of_day.go:275-292) — so the don't-clobber guard at
	// notes_store.go:327-330 sees exactly what it sees in the app.
	if w.foreignHL {
		goToVerseRange(st, "John", 3, 1, 1)
	} else {
		addRecentChapter(st, "John", 3)
	}

	// The session focus, applied AFTER the derive exactly as the reader
	// applies it (a chip or note tapped after landing on the chapter), and
	// re-projected the way Show re-projects. A focus value naming a note the
	// world does not contain is not a reachable state — reported as
	// not-offered, like a verb no surface presents.
	switch w.focus {
	case focusNoneAx:
		if w.placement == placeNone {
			return obs, false
		}
		st.focusNone()
		applyNoteForCurrentChapter(st)
	case focusExactKey:
		if w.placement != placeOwn && w.placement != placeBoth {
			return obs, false
		}
		st.focusNote(storedIDByText(t, "note under web"))
		applyNoteForCurrentChapter(st)
	case focusFollowedNote:
		if w.placement != placeFollowed && w.placement != placeBoth {
			return obs, false
		}
		st.focusNote(storedIDByText(t, "note under bsb"))
		applyNoteForCurrentChapter(st)
	case focusOwnAx:
		// Not a reachable state without one to open: the browser can only
		// offer a row the store holds.
		if !w.ownNote {
			return obs, false
		}
		st.focusNote(storedIDByText(t, "a note of my own"))
		applyNoteForCurrentChapter(st)
	}

	// A note-bearing link landing on the chapter the reader is already on. The
	// arrival stores the note and then writes the mirror by hand — including
	// the stored identity the verbs address — which is why it belongs on the
	// cross-product and not in a case somebody thought of.
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

	obs.st = st
	obs.shownText = st.ActiveNote
	obs.shownID = st.NoteID
	obs.shownWasOwn = isOwnLiveNote(st)
	obs.snapShown = takePlanSnap(st, false)
	obs.before = allNotesForBrowsing(appPrefs())

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
		// The "Keep them" answer: both Settings off-routes run turnNotesOff
		// (ai_settings.go) — the preference flips and the mark the live note
		// owns goes out, no other. clearLiveNote is on the DELETE branch only.
		turnNotesOff(st)
	}

	obs.text, obs.min, obs.hlOn = st.ActiveNote, st.NoteMinimized, st.hasMark()
	obs.after = allNotesForBrowsing(appPrefs())
	obs.snapVerb = takePlanSnap(st, true)

	addRecentChapter(st, "John", 3) // the next navigation
	obs.afterText, obs.afterMin, obs.afterHLOn = st.ActiveNote, st.NoteMinimized, st.hasMark()
	obs.snapNav = takePlanSnap(st, true)
	return obs, true
}

// storedIDByText finds a seeded note's identity, the way a surface would hand
// it to focus — from the store, never rebuilt.
func storedIDByText(t *testing.T, text string) uint64 {
	t.Helper()
	for _, n := range allNotesForBrowsing(appPrefs()) {
		if n.Text == text {
			return n.ID
		}
	}
	t.Fatalf("no stored note with text %q to focus", text)
	return 0
}

// checkNotesInvariants returns the names of the invariants this state violates.
// Each is stated as a property of the reader's experience, not of a field.
func checkNotesInvariants(w notesWorld, o notesObs) []string {
	var bad []string

	// N1 — no mark without a meaning. A highlight is on screen and nothing on
	// screen explains it: no note, and no other origin that put it there.
	// An ARRIVAL supersedes whatever was lit: the reader tapped a link naming a
	// passage, and the app is right to move the mark to it. So in an arrival
	// cell the foreign mark is gone for a legitimate reason, and any mark left
	// after the note is deleted is unexplained — which is why foreignHL stops
	// excusing this cell once arrival is true. Before S1 those 16 cells hid a
	// real orphan behind the exclusion.
	if o.afterHLOn && o.afterText == "" && (!w.foreignHL || w.arrival) {
		bad = append(bad, "N1-orphan-highlight")
	}
	// N1 again, the other way: a mark that belongs to somebody else must survive
	// a verb aimed at the note. The derive goes out of its way not to clobber a
	// foreign highlight (notes_store.go:327-330); the verbs then destroy it
	// unconditionally (:395, :422).
	// ...but only where the foreign mark was still the reader's reason for the
	// lit verse. See the note above: an arrival replaces it first, so its absence
	// afterwards is not the verb's doing.
	if w.foreignHL && !w.arrival && (w.verb == verbHide || w.verb == verbDelete) && !o.hlOn {
		bad = append(bad, "N1-foreign-mark-destroyed")
	}

	// N2 — a verb reaches what the reader aimed it at.
	//
	// An OWN note takes a DIFFERENT MEASUREMENT, and a stricter one rather than
	// a weaker one. ✕ and − on an own note are DISMISS, not delete and minimize
	// (notes_store.go, dropCurrentNote and hideCurrentNote): the record is the
	// reader's only copy of something they wrote, the reading card is transient,
	// and a durable act on it belongs to the browser where the row identifies
	// the record unambiguously. So "the verb reached it" means the note left the
	// page — and, in the same breath, that the store was not touched at all.
	//
	// Measuring these by "did the store change?" is what the received-note arms
	// below do, and it reported 64 cells of N2-*-missed the first time this axis
	// was enumerated. That was the harness looking where its model pointed, not
	// a defect: the model had one kind of note in it.
	if o.shownWasOwn && (w.verb == verbDelete || w.verb == verbHide) {
		if len(o.after) != len(o.before) {
			bad = append(bad, "N2-own-dismiss-wrote-the-store")
		}
		if o.text == o.shownText {
			bad = append(bad, "N2-own-dismiss-missed")
		}
	} else {
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
	}

	// N3 — no silent substitution, judged over what the reader can SEE. Since
	// S8 the banner draws the whole set, so the note that is OPEN may change
	// only to a note that was already on screen — a chip becoming the bubble
	// is a change among visible things, not a swap under the reader's eyes.
	// A violation is an open note after the navigation that is neither the
	// one the reader had open nor anything they could see when they acted.
	// (The Apple sticker is the arity-1 subset of this: its bubble swap is
	// announced by the count line riding in the sticker's own text.)
	if w.verb != verbNotesOff {
		shownOpenID := planOpenID(o.snapShown.plan)
		if navID := planOpenID(o.snapNav.plan); navID != 0 && navID != shownOpenID &&
			!planSees(o.snapShown.plan, navID) {
			bad = append(bad, "N3-substituted")
		}
	}

	// N4 — nothing in the store is invisible from the reading view: every
	// received note on the passage is on the banner as a bubble, a chip, or
	// an unplaced chip with its reason (S8). Judged at both moments the
	// reader could be looking; verbNotesOff's post-verb snapshot reports
	// featureOn=false and is rightly quiet.
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"verb", o.snapVerb}, {"nav", o.snapNav}} {
		if s.snap.featureOn &&
			s.snap.passageNotes > len(s.snap.plan.Notes)+len(s.snap.plan.Unplaced) {
			bad = append(bad, "N4-store-note-invisible@"+s.when)
		}
	}

	// N5 — an explicit minimize is honoured, and an explicit restore sticks.
	// Judged on the note the reader AIMED the verb at: once the display can
	// fall to a different note (the arity-1 debt N3 already names), an
	// expanded stranger after the navigation is a substitution, not a
	// reversed minimize — the hidden note itself staying minimized is what
	// this invariant asserts.
	if w.verb == verbHide && o.afterText == o.shownText && o.afterText != "" && !o.afterMin {
		bad = append(bad, "N5-hide-not-honoured")
	}
	if w.verb == verbShow && o.afterText == o.shownText && o.afterText != "" && o.afterMin {
		bad = append(bad, "N5-show-not-honoured")
	}

	// N9 — the received set is represented EXACTLY ONCE. Judged at the same two
	// moments N4 uses, and only where there is a set to represent: zero notes
	// need no representation. shownAsNothing with notes present is the
	// violation, and it is the one X15 produced — the sticker busy with the
	// reader's own note and the pills stood down for it, so the friends' notes
	// were on the page nowhere at all.
	//
	// There is no "twice" arm to check because the three values are exclusive by
	// construction: receivedSetShownAs returns one. That exclusivity is the
	// point of naming the value — the bug existed while the same question was
	// being answered independently in two places.
	// Three moments, including the one the reader was actually looking at when
	// they reached for the verb. N9 is a model-only question, so judging it
	// there costs nothing — unlike N10, which needs a pane and is judged only
	// where one was built. (The pre-verb state is not unwatched even for N10:
	// the verb=none cells make it a post-verb state with the same axes.)
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"shown", o.snapShown}, {"verb", o.snapVerb}, {"nav", o.snapNav}} {
		if !s.snap.featureOn || len(s.snap.plan.Notes) == 0 {
			continue
		}
		if s.snap.shownAs == shownAsNothing {
			bad = append(bad, "N9-set-unrepresented@"+s.when)
		}
	}

	// N10 — what the mirror says is on screen is actually DRAWN. The model and
	// the pane are two accounts of one page and they can disagree: the pane
	// decides its own geometry, and zeroing the wrong one blanks a note the
	// model still believes is there. The reader's own note is the sharp case,
	// because nothing else on the page speaks for it — a received note at least
	// has the rest of its set — so it gets its own arm.
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"verb", o.snapVerb}, {"nav", o.snapNav}} {
		if !s.snap.featureOn {
			continue
		}
		if s.snap.ownFocused && !s.snap.paneSticker {
			bad = append(bad, "N10-own-note-not-drawn@"+s.when)
		}
		if s.snap.activeNote != "" && !s.snap.paneSticker && s.snap.panePills == 0 {
			bad = append(bad, "N10-nothing-drawn@"+s.when)
		}
	}

	// N6 — the mirror agrees with the store, under the key the verbs address.
	if o.afterText != "" && !storeHolds(o.after, o.afterText) {
		bad = append(bad, "N6-mirror-only")
	}

	// N11-N15 — the CHROME's self-consistency (notes_chrome.go), asserted at
	// all three moments. These are tripwires over the composed value the four
	// surfaces consume: each states a property that is true by construction
	// TODAY, so that the derivation growing an input, a filter, or a second
	// opinion trips a cell here instead of a reader's screen. The paragraphs
	// come from the same grouping every surface renders — that identity is
	// enforced elsewhere (TestEverySurfaceBreaksParagraphsWhereTheModelDoes),
	// so leaning on it here is the convention, not a blind spot.
	paras := groupVersesIntoParagraphs(enumerationChapter())
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"shown", o.snapShown}, {"verb", o.snapVerb}, {"nav", o.snapNav}} {
		c := s.snap.chrome

		// N11 — a card has a tail iff it points at a passage, and only at a
		// passage this chapter carries; every band likewise names verse 0 (the
		// chapter top) or a verse with a paragraph here. Defect 3's axis: a
		// tail on an anchorless card claims verse 1.
		if c.hasTail() != (c.Anchor > 0) {
			bad = append(bad, "N11-tail-diverged-from-anchor@"+s.when)
		}
		if c.Anchor > 0 && noteParagraphOf(paras, c.Anchor) < 0 {
			bad = append(bad, "N11-anchor-points-nowhere@"+s.when)
		}
		for _, b := range c.Bands {
			if b.Verse != 0 && noteParagraphOf(paras, b.Verse) < 0 {
				bad = append(bad, "N11-band-points-nowhere@"+s.when)
			}
		}

		// N12 — the verb set is a function of (present, Own) alone, by the
		// stated rule; and Own agrees with what the store says about the note
		// the mirror names. The second arm is the assertion that would have
		// caught the clamped-chapter divergence by measurement.
		wantVerbs := noteVerbsReceived
		switch {
		case !c.present():
			wantVerbs = noteVerbsNone
		case c.Own:
			wantVerbs = noteVerbsOwn
		}
		if c.verbs() != wantVerbs {
			bad = append(bad, "N12-verbs-not-own-function@"+s.when)
		}
		if c.Own != s.snap.liveKindMine {
			bad = append(bad, "N12-own-disagrees-with-store@"+s.when)
		}

		// N13 — the tuple never reaches the states the surfaces cannot draw:
		// sender words always carry a byline, and "who without text" is always
		// marked collapsed — an empty sender bubble must never render, and the
		// styled pane's shorter collapsed test was harmless ONLY while
		// appleStickerPush kept this promise.
		if c.Text != "" && c.Who == "" {
			bad = append(bad, "N13-words-without-byline@"+s.when)
		}
		if c.Text == "" && c.Who != "" && !c.Pill {
			bad = append(bad, "N13-empty-bubble-reachable@"+s.when)
		}

		// N14 — an arrival names a verse this chapter carries, and targets a
		// band only when a reservation actually exists on the arriving verse's
		// paragraph: a group's band, the anchored card's own, or the
		// chapter-top parking of an anchorless card. Defect 1 as a value.
		switch c.Arrival {
		case arriveNothing:
			if c.ArrivalVerse != 0 {
				bad = append(bad, "N14-nothing-with-a-target@"+s.when)
			}
		case arriveVerse, arriveBand:
			if c.ArrivalVerse <= 0 || noteParagraphOf(paras, c.ArrivalVerse) < 0 {
				bad = append(bad, "N14-target-not-on-chapter@"+s.when)
			}
		}
		if c.Arrival == arriveBand {
			arriving := noteParagraphOf(paras, c.ArrivalVerse)
			reserved := false
			for _, b := range c.Bands {
				bandPara := 0
				if b.Verse != 0 {
					bandPara = noteParagraphOf(paras, b.Verse)
				}
				if bandPara == arriving {
					reserved = true
					break
				}
			}
			if !reserved && c.present() {
				if c.Anchor > 0 && noteParagraphOf(paras, c.Anchor) == arriving {
					reserved = true // the single card's own reservation
				}
				if c.Anchor <= 0 && arriving == 0 {
					reserved = true // the anchorless card parks at the chapter top
				}
			}
			if !reserved {
				bad = append(bad, "N14-band-without-reservation@"+s.when)
			}
		}

		// N15 — the bands mirror the groups: one per group, keyed by index
		// (the sort contract placement and the verb both lean on), never
		// empty-handed, and none at all while the gate is off. This is the
		// gate the flag's flip was decided over.
		if len(c.Bands) != len(s.snap.groupList) {
			bad = append(bad, "N15-bands-groups-diverge@"+s.when)
		}
		if !w.pills && len(c.Bands) != 0 {
			bad = append(bad, "N15-bands-with-gate-off@"+s.when)
		}
		for i, b := range c.Bands {
			if b.Key != i {
				bad = append(bad, "N15-band-key-not-index@"+s.when)
			}
			if b.Count == 0 && b.Unplaced == 0 {
				bad = append(bad, "N15-empty-band@"+s.when)
			}
		}

		// N9 again, as a seam: the chrome's own ShownAs and the answer the
		// harness read through the styled path (a second plan, a second
		// composition, same instant) must agree.
		if c.ShownAs != s.snap.shownAs {
			bad = append(bad, "N9-shownas-routes-disagree@"+s.when)
		}
	}

	// N10, extended — the styled pane's pill row is EXACTLY the model's
	// answer: groups pills where ShownAs says pills, none anywhere else.
	// Judged only at the two moments a pane was built.
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"verb", o.snapVerb}, {"nav", o.snapNav}} {
		if !s.snap.featureOn {
			continue
		}
		wantPills := 0
		if s.snap.shownAs == shownAsPills {
			wantPills = s.snap.groups
		}
		if s.snap.panePills != wantPills {
			bad = append(bad, "N10-pane-pills-diverge@"+s.when)
		}
	}

	// V — the plan's own invariants (S7, notes_plan.go), asserted over both
	// snapshots. These are properties of the new model, expected to hold in
	// EVERY cell: any V violation is a new incoherent state, and no pinned
	// defect may cover one.
	for _, s := range []struct {
		when string
		snap planSnap
	}{{"verb", o.snapVerb}, {"nav", o.snapNav}} {
		open := 0
		for _, d := range s.snap.plan.Notes {
			if !d.Open {
				continue
			}
			open++
			if d.Note.Minimized {
				bad = append(bad, "V2-open-minimized@"+s.when)
			}
		}
		if open > planOpenLimit {
			bad = append(bad, "V1-open-cap-exceeded@"+s.when)
		}
		if s.snap.suppressed && s.snap.featureOn {
			if open > 0 {
				bad = append(bad, "V3-suppressed-but-open@"+s.when)
			}
			if s.snap.passageNotes > 0 && len(s.snap.plan.Notes)+len(s.snap.plan.Unplaced) == 0 {
				bad = append(bad, "V3-suppression-emptied-the-plan@"+s.when)
			}
		}
		if !s.snap.featureOn && (len(s.snap.plan.Notes) > 0 || len(s.snap.plan.Unplaced) > 0) {
			bad = append(bad, "V4-off-plan-not-empty@"+s.when)
		}
	}
	return bad
}

// planOpenID is the identity of the plan's open note, 0 when nothing is open.
func planOpenID(p chapterPlan) uint64 {
	if d, ok := p.openNote(); ok {
		return d.Note.ID
	}
	return 0
}

// planSees reports whether a note is anywhere on the plan's surface — the
// bubble, a chip, or the unplaced group. That is exactly what a reader could
// see on the S8 banner.
func planSees(p chapterPlan, id uint64) bool {
	for _, d := range p.Notes {
		if d.Note.ID == id {
			return true
		}
	}
	for _, d := range p.Unplaced {
		if d.Note.ID == id {
			return true
		}
	}
	return false
}

func storeHolds(notes []StoredNote, text string) bool {
	for _, n := range notes {
		if n.Text == text {
			return true
		}
	}
	return false
}

func minimizedInStore(notes []StoredNote, text string) bool {
	for _, n := range notes {
		if n.Text == text {
			return n.Minimized
		}
	}
	return false // not there at all is not "minimized"
}

func noteByIDIn(notes []StoredNote, id uint64) (StoredNote, bool) {
	for _, n := range notes {
		if n.ID == id {
			return n, true
		}
	}
	return StoredNote{}, false
}

// lostOthers names a note that disappeared from the store although the reader
// aimed Delete at a different one.
func lostOthers(before, after []StoredNote, aimedAt string) string {
	for _, n := range before {
		if n.Text == aimedAt {
			continue
		}
		if _, still := noteByIDIn(after, n.ID); !still {
			return n.Text
		}
	}
	return ""
}

// flippedOthers reports a note whose collapsed state changed although the reader
// aimed Hide or Show at a different one.
func flippedOthers(before, after []StoredNote, aimedAt string) bool {
	for _, n := range before {
		if n.Text == aimedAt {
			continue
		}
		if a, still := noteByIDIn(after, n.ID); still && a.Minimized != n.Minimized {
			return true
		}
	}
	return false
}

// --- enumeration 2: the highlight's origin ----------------------------------

// The origin used to be declared HERE, as a private variable of the harness,
// under the heading "THE VARIABLE THE APP DOES NOT RECORD" — five writers put a
// highlight into AppState and nothing distinguished them afterwards, so
// ownership was inferred from a bare verse number.
//
// S1 moved it into the production model (mark.go), which is what this harness
// existed to argue for: a test that has to carry a variable the model lacks is
// telling you the model lacks it. The names below are aliases onto the real
// type, kept so the enumeration reads the same as when it was written.
const (
	fromNothing    = hlNone
	fromNote       = hlNote
	fromSearch     = hlSearch
	fromVerseOfDay = hlVerseOfDay
	fromLinkSpan   = hlLinkSpan
)

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
	seeded, _ := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 23, Text: "the note"})
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

	hadHL := st.hasMark()
	spBefore, _ := st.markSpan() // the pre-switch span, for the frame check
	fromVer := st.CurrentVersion

	switch ev {
	case evNavigate:
		addRecentChapter(st, "Romans", 14)
	case evNoteDeleted:
		if st.ActiveNote != "" {
			dropCurrentNote(st)
		} else {
			deleteNoteByID(appPrefs(), seeded.ID)
		}
		addRecentChapter(st, "Romans", 14)
	case evNotesOff:
		turnNotesOff(st) // the real off verb — Settings and the dev toggle both
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
		if st.hasMark() && st.ActiveNote == "" {
			bad = append(bad, "N1-orphan-highlight")
		}
	default:
		if hadHL && !st.hasMark() && ev != evSwitchVersion {
			bad = append(bad, "N1-foreign-mark-destroyed")
		}
	}

	// N7 — one ruler. After a version switch a surviving highlight must be
	// expressed in the numbering of the translation now being read: the span's
	// own frame must say so, and its location must agree with mapping the
	// PRE-switch span into the new translation. (Before the X11 fix the check
	// mapped the surviving span forward and asked whether it had moved — that
	// spelling only made sense while the switch left the span untouched.)
	if ev == evSwitchVersion {
		if sp, ok := st.markSpan(); ok {
			if !strings.EqualFold(sp.VersionID, st.CurrentVersion) {
				bad = append(bad, "N7-stale-frame")
			} else if spBefore.Lo > 0 {
				ch, v, res := MapVerse(fromVer, st.CurrentVersion, spBefore.Book, spBefore.Chapter, spBefore.Lo)
				if (res == verseMapExact || res == verseMapMoved) && (sp.Chapter != ch || sp.Lo != v) {
					bad = append(bad, "N7-stale-frame")
				}
			}
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
	addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Text: "look at 16"})
	addRecentChapter(st, "John", 3)
	if !st.hasMark() {
		t.Fatal("precondition: the note should have raised its highlight")
	}

	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3}) // no verse, no note

	// X8 — the note survives (the share_link_open.go:308 guard) but its mark does not.
	if st.ActiveNote == "" {
		t.Fatal("the bare link blanked the stored note; that is a different defect from X8")
	}
	if st.hasMark() {
		t.Error("X8 is FIXED: the note kept its highlight across a bare chapter link. " +
			"Strike X8 from docs/NOTES_STATE.md.")
	}
	// X9 is STRUCTURALLY dead as of S1 and was struck from docs/NOTES_STATE.md.
	// It used to be reachable because HasHighlightedVerse=false left the book,
	// chapter and verse behind — a location outliving the flag that said to
	// ignore it. There are no separate fields now: absence IS hlNone, and the
	// span goes with it. The assertion that remains is that nothing can be read
	// back out, which is a property of the type rather than of this code path.
	if sp, ok := st.markSpan(); ok {
		t.Errorf("a cleared mark still reports a span: %+v", sp)
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
	n, ok := addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "John", Chapter: 3, Text: "about the whole chapter"})
	if !ok {
		t.Fatal("the note was not stored")
	}
	openNote(st, n)

	if st.hasMark() {
		t.Fatal("a chapter-level note should not raise a verse highlight; that is a new state")
	}
	// The browser half of X9, likewise structural now. See the note above.
	if sp, ok := st.markSpan(); ok {
		t.Errorf("a chapter-level note left a span behind: %+v", sp)
	}
}

// --- the single-state facts the cross-product cannot reach ------------------

// X3 — STRUCK by S5. The old store held at most 200 notes and evicted past the
// cap by ALPHABETICAL ORDER of the storage key, so an arriving note could be
// discarded by the very write that stored it while the reader was looking at
// it on screen. The scrapbook store has NO cap and NO eviction — eviction is a
// data-loss event — so this now pins the fixed behaviour: an arrival onto a
// store far past the old cap is stored, shown, and still there after the next
// navigation, and nothing else went missing.
func TestArrivingNoteSurvivesItsOwnSaveOnAFullStore(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	const fillers = 250 // past the old notesMax of 200
	for i := 0; i < fillers; i++ {
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Psalms", Chapter: i + 1, Text: fmt.Sprintf("filler %d", i)})
	}
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "John", CurrentChapter: 3,
		CurrentVersion: "web", loadPhase: loadReady,
		loadedVersions: map[string]*BibleData{"web": bd},
	}
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16, Note: "just arrived"})

	if st.ActiveNote != "just arrived" {
		t.Fatalf("precondition: the arriving note should be on screen, got %q", st.ActiveNote)
	}
	if got := storedNoteCount(appPrefs()); got != fillers+1 {
		t.Errorf("the store evicted: %d notes, want %d — eviction is a data-loss event", got, fillers+1)
	}
	addRecentChapter(st, "John", 3)
	if st.ActiveNote != "just arrived" {
		t.Errorf("the arriving note did not survive the next navigation: %q", st.ActiveNote)
	}
}

// HL_FRAME / X11 — FIXED 2026-08-18. The version switch renumbers the
// highlight through the SAME anchor machinery that renumbers the note beside
// it (renumberMarkForVersion in mark.go, called from applyLoadedVersion), so
// the two marks are measured by one ruler at last — the first use
// VerseSpan.VersionID has ever had. On anything but a clean landing the mark
// is CLEARED rather than left lighting the wrong text. This test pins the
// fixed behaviour across every arm the resolver can answer:
//
//   - the doxology (moved CROSS-CHAPTER): WEB Romans 14:24 IS BSB 16:25.
//     Before the fix the mark survived the switch still saying 14:24 — the
//     wrong verse lit, beside a note that WAS renumbered.
//   - the identity case: the numbering agrees, the mark survives untouched
//     (same numbers, new frame).
//   - an absent verse (a BSB omission), Greek Esther (incommensurable), and a
//     book the new translation does not contain: cleared, every one.
func TestHighlightRenumberedAcrossVersionSwitch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	switchTo := func(t *testing.T, st *AppState, id string, data *BibleData) {
		t.Helper()
		v, ok := versionByID(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		applyLoadedVersion(st, v, data, modeReal)
	}

	t.Run("doxology moved cross-chapter", func(t *testing.T) {
		ch, v, res := MapVerse("web", "bsb", "Romans", 14, 24)
		if res != verseMapMoved {
			t.Fatalf("precondition: web Romans 14:24 should MOVE into the bsb, got %d:%d (%v)", ch, v, res)
		}
		bd := romansBible()
		st := &AppState{
			Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
			CurrentVersion: "web", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"web": bd},
		}
		goToVerseRange(st, "Romans", 14, 24, 24)

		switchTo(t, st, "bsb", romansBible())

		sp, ok := st.markSpan()
		if !ok {
			t.Fatal("the mark did not survive a cleanly-mapped switch")
		}
		if sp.Chapter != ch || sp.Lo != v {
			t.Errorf("the mark was not renumbered: %d:%d, want %d:%d", sp.Chapter, sp.Lo, ch, v)
		}
		if sp.VersionID != "bsb" {
			t.Errorf("the surviving span's frame is %q, want the reading translation's", sp.VersionID)
		}
		if st.mark.Origin != hlVerseOfDay {
			t.Errorf("renumbering changed the mark's origin: %v", st.mark.Origin)
		}
	})

	t.Run("identity: same numbering survives untouched", func(t *testing.T) {
		bd := romansBible()
		st := &AppState{
			Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
			CurrentVersion: "web", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"web": bd},
		}
		goToVerseRange(st, "Romans", 14, 23, 23) // 14:23 maps exactly

		switchTo(t, st, "bsb", romansBible())

		sp, ok := st.markSpan()
		if !ok {
			t.Fatal("an identically-numbered mark did not survive the switch")
		}
		if sp.Chapter != 14 || sp.Lo != 23 {
			t.Errorf("an exact mapping moved the mark: %d:%d, want 14:23", sp.Chapter, sp.Lo)
		}
		if sp.VersionID != "bsb" {
			t.Errorf("the span's frame is %q, want the reading translation's", sp.VersionID)
		}
	})

	t.Run("absent verse clears", func(t *testing.T) {
		if _, _, res := MapVerse("web", "bsb", "Mark", 9, 44); res != verseMapAbsent {
			t.Fatalf("precondition: web Mark 9:44 should be ABSENT from the bsb, got %v", res)
		}
		mk := func() *BibleData {
			bd := NewBibleData()
			bd.PopulateWithSampleVerses()
			bd.Verses["Mark"][9] = []Verse{
				{BookName: "Mark", Chapter: 9, Verse: 43, Text: "mark"},
				{BookName: "Mark", Chapter: 9, Verse: 44, Text: "mark"},
			}
			return bd
		}
		bd := mk()
		st := &AppState{
			Bible: bd, CurrentBook: "Mark", CurrentChapter: 9,
			CurrentVersion: "web", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"web": bd},
		}
		goToVerseRange(st, "Mark", 9, 44, 44)

		switchTo(t, st, "bsb", mk())

		if sp, ok := st.markSpan(); ok {
			t.Errorf("a mark on a verse the new translation omits must clear, still lights %d:%d", sp.Chapter, sp.Lo)
		}
	})

	t.Run("incommensurable clears (Greek Esther)", func(t *testing.T) {
		if _, _, res := MapVerse("webc", "web", "Esther", 1, 1); res != verseMapIncommensurable {
			t.Fatalf("precondition: webc Esther should be INCOMMENSURABLE with web, got %v", res)
		}
		bd := NewBibleData()
		bd.PopulateWithSampleVerses()
		st := &AppState{
			Bible: bd, CurrentBook: "Esther", CurrentChapter: 1,
			CurrentVersion: "webc", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"webc": bd},
		}
		goToVerseRange(st, "Esther", 1, 1, 1)

		web := NewBibleData()
		web.PopulateWithSampleVerses()
		switchTo(t, st, "web", web)

		if sp, ok := st.markSpan(); ok {
			t.Errorf("an incommensurable mark must clear, still lights %d:%d", sp.Chapter, sp.Lo)
		}
	})

	t.Run("book the new translation lacks clears", func(t *testing.T) {
		bd := NewBibleData()
		bd.PopulateWithSampleVerses()
		st := &AppState{
			Bible: bd, CurrentBook: "John", CurrentChapter: 3,
			CurrentVersion: "webc", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"webc": bd},
		}
		// Set directly: Tobit is not in the 66-book fixture canon, so no real
		// writer can be driven here — which is the point of the arm: the
		// mapping TABLES claim Tobit "maps exactly" into a WEB that does not
		// contain it, and only the book-existence test says otherwise.
		st.setMark(hlSearch, VerseSpan{VersionID: "webc", Book: "Tobit", Chapter: 1, Lo: 1})

		web := NewBibleData()
		web.PopulateWithSampleVerses()
		switchTo(t, st, "web", web)

		if sp, ok := st.markSpan(); ok {
			t.Errorf("a mark on a book the new translation lacks must clear, still lights %s %d:%d", sp.Book, sp.Chapter, sp.Lo)
		}
	})
}

// The other half of the X11 fix, pinned: an hlNote mark is NOT renumbered by
// renumberMarkForVersion — the note projection owns it in both directions.
// applyNoteForCurrentChapter, run by the same apply tail, re-derives the note
// into the new translation and re-places its mark from the note itself (in
// the reading translation's frame), or clears it when the note has no home on
// this chapter any more.
func TestNoteMarkIsRederivedNotRenumberedOnSwitch(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setNotesEnabled(true)
	deleteAllNotes(appPrefs())
	defer deleteAllNotes(appPrefs())

	origNow := noteNow
	noteNow = func() int64 { return 1_700_000_000 }
	defer func() { noteNow = origNow }()

	t.Run("re-placed, in the new frame", func(t *testing.T) {
		deleteAllNotes(appPrefs())
		bd := romansBible()
		st := &AppState{
			Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
			CurrentVersion: "web", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"web": bd},
		}
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 23, Text: "on 23"})
		addRecentChapter(st, "Romans", 14)
		if !st.mark.fromNote() {
			t.Fatal("precondition: the note should own the mark")
		}

		v, ok := versionByID("bsb")
		if !ok {
			t.Fatal("bsb is not registered")
		}
		applyLoadedVersion(st, v, romansBible(), modeReal)

		if st.ActiveNote != "on 23" {
			t.Fatalf("the note did not follow the switch: %q", st.ActiveNote)
		}
		sp, ok := st.markSpan()
		if !ok || !st.mark.fromNote() {
			t.Fatal("the followed note did not re-raise its mark")
		}
		if sp.Chapter != 14 || sp.Lo != 23 {
			t.Errorf("the note's mark landed at %d:%d, want 14:23", sp.Chapter, sp.Lo)
		}
		if sp.VersionID != "bsb" {
			t.Errorf("the note's mark is framed %q, want the reading translation's — "+
				"a followed note's span carries renumbered NUMBERS and must not carry its filing as the frame", sp.VersionID)
		}
	})

	t.Run("cleared when the note leaves the chapter", func(t *testing.T) {
		deleteAllNotes(appPrefs())
		bd := romansBible()
		st := &AppState{
			Bible: bd, CurrentBook: "Romans", CurrentChapter: 14,
			CurrentVersion: "web", loadPhase: loadReady,
			loadedVersions: map[string]*BibleData{"web": bd},
		}
		// The doxology: in the BSB this note's passage lives on chapter 16, so
		// after the switch chapter 14 holds no note — and must hold no mark.
		addNote(appPrefs(), StoredNote{Kind: noteKindReceived, VersionID: "web", Book: "Romans", Chapter: 14, VerseLo: 24, Text: "doxology"})
		addRecentChapter(st, "Romans", 14)
		if !st.mark.fromNote() {
			t.Fatal("precondition: the note should own the mark")
		}

		v, ok := versionByID("bsb")
		if !ok {
			t.Fatal("bsb is not registered")
		}
		applyLoadedVersion(st, v, romansBible(), modeReal)

		if st.ActiveNote != "" {
			t.Fatalf("chapter 14 should hold no note in the bsb, got %q", st.ActiveNote)
		}
		if sp, ok := st.markSpan(); ok {
			t.Errorf("the departed note's mark was left behind at %d:%d — that is defect 1", sp.Chapter, sp.Lo)
		}

		// And on the chapter the passage actually lives on, the note surfaces
		// with its mark in the new numbering — nothing was lost, only moved.
		st.CurrentChapter = 16
		addRecentChapter(st, "Romans", 16)
		if st.ActiveNote != "doxology" {
			t.Fatalf("the note did not surface on its bsb chapter: %q", st.ActiveNote)
		}
		sp, ok := st.markSpan()
		if !ok || !st.mark.fromNote() {
			t.Fatal("the note did not re-raise its mark on its new chapter")
		}
		if sp.Chapter != 16 || sp.Lo != 25 || sp.VersionID != "bsb" {
			t.Errorf("the note's mark is %s %d:%d, want bsb 16:25", sp.VersionID, sp.Chapter, sp.Lo)
		}
	})
}

// N8, the temporary cap: this file used to hold a sentinel test asserting the
// cap was NOT representable — "delete this test when the SET model lands". S7
// landed the model: which note is expanded is a question the PLAN answers
// (drawnNote.Open, capped by planOpenLimit), the V-invariants above enumerate
// it, and TestTheCapOpensOneAndLeavesZeroStoreResidue (notes_plan_test.go)
// pins the half that must stay true forever: the cap writes nothing.
