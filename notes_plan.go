package bibletext

// The chapter plan — S7 of docs/NOTES_SCRAPBOOK.md. The model goes plural; the
// view does not, yet.
//
// buildChapterPlan is THE derive: one pass over the store answering "what does
// this passage carry?" with a SET — every note that lands on the chapter, every
// note for this book that cannot land in this translation, what each verse is
// washed in, the could-not-read-a-payload sentence, and one deterministic
// fingerprint over all of it. Every reading surface will consume this one value
// (S8); in this step the surfaces still render from the AppState mirror, and
// the mirror has become a PROJECTION of this plan (applyNoteForCurrentChapter,
// notes_store.go) — so the rendered result is byte-identical to the arity-1
// display while the model underneath already says the whole truth.
//
// THE ONE GATE. notesFeatureOn()==false yields an EMPTY plan — no Notes, no
// Unplaced, nothing open — with the Notice still carried and the passage view
// (the tint of a search or link mark) unaffected. There is no second feature
// check for a surface to diverge from (invariant S14 of the design).
//
// SUPPRESSION IS DERIVED, NEVER STORED. A live mark not owned by a note —
// state.mark.live() && !state.mark.fromNote() — stands the notes down: zero
// Open, Notes still present, and NOTHING is written. That is the owner's
// "minimize while search results are being displayed, then restore" made
// mechanical: a suppression that restores is only possible because it never
// wrote to the store (docs/NOTES_SCRAPBOOK.md, "Suppression, not minimize").
//
// READ-ONLY, BY CONSTRUCTION. buildChapterPlan mutates nothing: not the store,
// not AppState, not the mark. The only store write any focus change may make is
// an explicit Hide's Minimized, and that happens in the verb, not here.

import (
	"fmt"
	"sort"
	"strings"
)

// planOpenLimit is how many notes may be expanded at once — the TEMPORARY cap
// (owner, 2026-08): at most one, and ZERO is legal (an explicit minimize is
// honoured and nothing auto-expands). It lives HERE and nowhere else, as a
// build constant rather than a preference, and it writes nothing: the cap is
// maintained by the view and leaves zero bytes in the store, so lifting it a
// year from now cannot greet a reader with chips they never closed.
//
// TO LIFT THE CAP: raise this number (or drop the counter). Nothing else
// changes — not the store, not drawnNote, not the fingerprint.
const planOpenLimit = 1

// noteFocus is WHICH note is expanded this session — session-only, never
// stored, and deliberately three-valued:
//
//	unset (the zero value) — no choice made; the default rule applies
//	none  (set, id 0)      — the reader explicitly closed the open note; NOTHING
//	                          opens, and focus never falls to another note (N3:
//	                          a different message must not take the closed one's
//	                          place under the reader's eyes)
//	a NoteID (set, id > 0) — the reader opened this one (Show, or an arriving
//	                          link)
//
// It rides on AppState because it is the reader's session state, but it is a
// pure INPUT to buildChapterPlan: Open is computed while walking the set and is
// never stored on the model, so there is no second copy to go stale — the
// design's recorded risk of "moving the unwatched variable rather than
// removing it" is answered by the harness growing a focus axis
// (notes_state_flow_test.go), not by another mirror.
type noteFocus struct {
	set bool
	id  uint64
}

// focusNote records that the reader opened this note (Show, arrival).
func (s *AppState) focusNote(id uint64) {
	if s == nil {
		return
	}
	s.noteFocus = noteFocus{set: true, id: id}
}

// focusNone records that the reader explicitly closed the open note (Hide).
// Focus falls to NONE, never to another note.
func (s *AppState) focusNone() {
	if s == nil {
		return
	}
	s.noteFocus = noteFocus{set: true}
}

// resetNoteFocus returns focus to the default rule. Navigation does this —
// every chapter arrival starts from the default — while a version switch does
// NOT: focus is a NoteID and the note it names survives the switch (hard case
// 12), so a message the reader deliberately opened is not swapped for another
// by changing translation.
func (s *AppState) resetNoteFocus() {
	if s == nil {
		return
	}
	s.noteFocus = noteFocus{}
}

// drawnNote is one note as the plan carries it: the stored record, where it
// landed in the translation being read, whether it is expanded RIGHT NOW, and
// the translation label its chrome will need (R2).
//
// Open is computed while walking the set and lives only in the plan — a bool
// on the record would be a derived mirror with one writer and many readers,
// which is the quadruple's failure shape at quarter scale.
type drawnNote struct {
	Note      StoredNote
	Placement placement
	Open      bool
	// Label is the translation abbreviation for the byline/heading (R2), and
	// "" for a note written against the very translation on screen
	// (placedNative) — the chrome must not print a label there.
	Label string
}

// sentence is the placement copy for an unplaced note — the R4 group's quiet
// explanation, from placementCopy. "" for every placed kind.
func (d drawnNote) sentence() string { return placementCopy(d.Placement.Kind) }

// chapterPlan is everything the reading surfaces need about one passage. Built
// only by buildChapterPlan; every field computed at construction and never
// written again.
type chapterPlan struct {
	// Notes is every note that lands ON this chapter, in a stable order:
	// Received descending, ID descending as the tiebreak — NEVER map order.
	// The same store always yields the same slice.
	Notes []drawnNote

	// Unplaced is the R4 group: notes filed under this BOOK whose anchor has
	// no home in the translation being read (book absent, incommensurable
	// numbering, verses absent). Nothing is tinted for them; their sentence
	// (drawnNote.sentence) says why. Same stable order.
	Unplaced []drawnNote

	// Tints is what each verse of the chapter is washed in (k<=1 today: the
	// one mark, whoever owns it). Carried on the plan so S8's surfaces read
	// ONE value; today's renderers still call chapterTint themselves and get
	// the identical answer.
	Tints chapterTints

	// Notice is the S4 could-not-read-the-payload sentence
	// (state.NoteNotice), carried even when the plan is otherwise empty —
	// the reader is told about a payload whether or not notes drew anything.
	Notice string

	// Fingerprint is this plan's identity: deterministic, byte-stable
	// run-to-run over an unchanged world, changing whenever anything a
	// surface draws from the plan changes — Open included, since S8 first
	// drew it. See foldFingerprint for the three-part split.
	Fingerprint string

	// noteFP is the note half of Fingerprint — everything except the wash —
	// for the body/tint split the Apple panes repair at different costs
	// (chapterBodyFingerprint, reading.go).
	noteFP string

	// display is the index in Notes of the ONE note the arity-1 mirror draws
	// (-1: nothing), replicating the shipped selection exactly — see
	// planDisplayIndex. Since S8 it is the APPLE seam: the Fyne surfaces read
	// the plan itself (notes_banner.go), while the mirror survives as the
	// native sticker's projection — the display note is the sticker's bubble
	// and the rest of the set rides in its text as an honest count
	// (appleStickerText). It is also the note the cap opens (the plan's
	// Open), so the bubble and the mirror can never disagree.
	display int
}

// openNote returns the plan's expanded note, if any. At most one exists
// (planOpenLimit).
func (p chapterPlan) openNote() (drawnNote, bool) {
	for _, d := range p.Notes {
		if d.Open {
			return d, true
		}
	}
	return drawnNote{}, false
}

// suppressed is the derived stand-down: some other reason owns the page.
// Nothing is stored, nothing is restored, and nothing can fall out of sync —
// there is no second copy (docs/NOTES_SCRAPBOOK.md, "Suppression, not
// minimize").
func notesSuppressed(state *AppState) bool {
	return state != nil && state.mark.live() && !state.mark.fromNote()
}

// buildChapterPlan derives the plan for the chapter the reader is on. p is the
// preference store the notes live in; bible is the READING translation's
// loaded data (the authority on which books exist — nil skips that test,
// matching resolveNoteAnchor).
func buildChapterPlan(state *AppState, p prefStore, bible *BibleData) chapterPlan {
	plan := chapterPlan{display: -1}
	if state == nil {
		plan.foldFingerprint()
		return plan
	}
	plan.Notice = state.NoteNotice
	// The tint is the passage's, not the notes': a search or link mark washes
	// its verses whether notes are on, off or suppressed, so it is computed
	// before the gate and survives it.
	plan.Tints = chapterTint(state)
	if !notesFeatureOn(state) {
		// Off means off: an EMPTY plan, one gate, no second check for any
		// surface to diverge from. The stored notes stay where they are.
		plan.foldFingerprint()
		return plan
	}

	versionID := state.currentVersion().ID
	book, chapter := state.CurrentBook, state.CurrentChapter

	// One pass over the store. Only notes filed under the book being read can
	// concern this chapter: a resolution renumbers chapters and verses, never
	// books, so the book filter is a fact about the anchor model, not an
	// optimisation gamble.
	s := readNoteStore(p)
	for _, n := range s.notes {
		// Own notes are stored but never drawn in the scripture text (owner
		// directive); only received notes reach the reading page.
		if n.Kind != noteKindReceived || !displayableNote(n) || n.Book != book {
			continue
		}
		pl := resolveNoteAnchor(n, versionID, bible)
		label := ""
		if pl.Kind != placedNative {
			label = noteVersionAbbrev(n.VersionID)
		}
		if !pl.Kind.placed() && pl.Kind != placedOtherChapter {
			// The R4 group: on this book, no home anywhere in this
			// translation (book absent, incommensurable, verses absent). Not
			// drawn in the text; never lost from the model.
			plan.Unplaced = append(plan.Unplaced, drawnNote{Note: n, Placement: pl, Label: label})
			continue
		}
		if _, on := placementRunOn(pl, chapter); !on {
			// Placed, but on another chapter of this book (including the
			// whole of a placedOtherChapter anchor) — the note appears on the
			// chapter its passage lives on (X13's fix), so skipping it here
			// is not a loss.
			continue
		}
		plan.Notes = append(plan.Notes, drawnNote{Note: n, Placement: pl, Label: label})
	}

	// Stable order: Received descending, ID descending as the tiebreak. Never
	// map order — the store slice is ID-ascending already, but the rule is
	// stated (and sorted) here so the plan cannot inherit an ordering change.
	byNewest := func(list []drawnNote) func(i, j int) bool {
		return func(i, j int) bool {
			a, b := list[i].Note, list[j].Note
			if a.Received != b.Received {
				return a.Received > b.Received
			}
			return a.ID > b.ID
		}
	}
	sort.SliceStable(plan.Notes, byNewest(plan.Notes))
	sort.SliceStable(plan.Unplaced, byNewest(plan.Unplaced))

	plan.display = planDisplayIndex(state, p, bible, plan.Notes, versionID, book, chapter)

	// Open, computed while walking the set, capped by planOpenLimit, and
	// never stored. Three things keep a note closed: the reader closed it
	// (Minimized, the only durable one), the reader explicitly closed the
	// session's open note (focus none), and a foreign mark owning the page
	// (suppression — derived, writes nothing, restores by itself).
	suppressed := notesSuppressed(state)
	focusedNone := state.noteFocus.set && state.noteFocus.id == 0
	opened := 0
	for i := range plan.Notes {
		if opened >= planOpenLimit {
			break
		}
		if i != plan.display {
			// The arity-1 truncation: only the display note may open today.
			// When S8 draws the set this branch goes with plan.display.
			continue
		}
		if plan.Notes[i].Note.Minimized || focusedNone || suppressed {
			continue
		}
		plan.Notes[i].Open = true
		opened++
	}

	plan.foldFingerprint()
	return plan
}

// planDisplayIndex picks the ONE note the S7 mirror draws.
//
// An explicitly focused note wins — that is Show and an arriving link, the
// reader's own session choice, and it survives a version switch (hard case
// 12). It falls through to the default ONLY when the focused id is absent from
// this chapter entirely, never merely because the note is minimized or
// suppressed.
//
// The DEFAULT is the shipped arity-1 selection, verbatim: noteForChapter's
// native-first, newest-first choice, including a collapsed native note masking
// an expanded followed one (COLLAPSED_MASK). That is a deliberate S7 decision,
// recorded here: this step must render byte-identically to today, so the
// default stays the selection readers already have — with its X7 masking debts
// intact and enumerated — and the design's "newest note not stored-Minimized"
// default lands in S8, where the whole set is drawn and surfacing a masked
// note is a chip appearing rather than a silent substitution.
func planDisplayIndex(state *AppState, p prefStore, bible *BibleData, notes []drawnNote, versionID, book string, chapter int) int {
	if f := state.noteFocus; f.set && f.id != 0 {
		for i := range notes {
			if notes[i].Note.ID == f.id {
				return i
			}
		}
		// Absent from this chapter (deleted, or unplaced here): default rule.
	}
	n, ok := noteForChapter(p, versionID, book, chapter, bible)
	if !ok {
		return -1
	}
	for i := range notes {
		if notes[i].Note.ID == n.ID {
			return i
		}
	}
	// noteForChapter and the plan walk the same store with the same
	// resolution, so a miss here would mean the two derives disagree; -1 keeps
	// the failure visible (no note drawn) rather than guessed around.
	return -1
}

// displayCopy is the display note renumbered into the translation being read —
// exactly the copy the pre-S7 derive handed the mirror. Native notes come home
// byte-exact (VersionID, Chapter, VerseLo untouched); a followed note keeps its
// filing (VersionID says where it LIVES) and takes the covering run's numbers.
func displayCopy(d drawnNote, versionID string, chapter int) StoredNote {
	n := d.Note
	if strings.EqualFold(n.VersionID, versionID) {
		return n
	}
	run, on := placementRunOn(d.Placement, chapter)
	if !on {
		return n // defensive: display notes always land on the chapter
	}
	out := n
	out.Chapter = chapter
	if n.VerseLo > 0 {
		out.VerseLo = run.Lo
		out.VerseHi = run.Hi
	}
	return out
}

// foldFingerprint computes the plan's identity, into noteFP (the note half)
// and Fingerprint (the whole, wash included).
//
// DETERMINISTIC BY CONSTRUCTION: everything folded comes from slices in the
// plan's stable order — no map is ranged anywhere on the way here — so an
// unchanged world folds to identical bytes on every build
// (TestChapterPlanFingerprintDoesNotFlap holds it to 50 in a row).
//
// WHAT IS FOLDED: the notice; the display note's identity; per placed note its
// ID, Minimized, text length (notes are replaced wholesale, never edited, so
// length stands in for content exactly as the old hand-rolled clause did), its
// label and placement kind, and its resolved runs NORMALISED (Hi==0 and Hi==Lo
// are one spelling, the chapterTints.fingerprint lesson); per unplaced note
// its ID and kind. Then the wash, as its own half.
//
// OPEN IS FOLDED — INTO THE WHOLE, NOT INTO noteFP. S7 deliberately omitted
// Open because no pixel depended on it; S8 draws it (the banner's bubble vs
// chip is Open), so the S7 trap is sprung in the same commit: Open joins
// Fingerprint as its own middle section. It stays OUT of noteFP on purpose —
// noteFP is the Apple BODY half (chapterBodyFingerprint, reading.go), and the
// Apple sticker draws the MIRROR plus the count lines (appleStickerText), none
// of which depend on Open. Folding Open there would turn every suppression
// flip (a search arriving on a chapter with a note) into a full
// NSAttributedString re-import, the exact cost the body/tint split exists to
// avoid; the invariant that the body half folds everything the sticker text
// depends on is pinned by TestAppleStickerTextIsFoldedByTheBodyFingerprint.
func (p *chapterPlan) foldFingerprint() {
	var b strings.Builder
	if p.Notice != "" {
		fmt.Fprintf(&b, "N%q;", p.Notice)
	}
	if p.display >= 0 {
		fmt.Fprintf(&b, "d%d;", p.Notes[p.display].Note.ID)
	}
	fold := func(prefix string, list []drawnNote) {
		for _, d := range list {
			m := 0
			if d.Note.Minimized {
				m = 1
			}
			fmt.Fprintf(&b, "%s%d.%d.%d.%s.%d", prefix, d.Note.ID, m, len(d.Note.Text), d.Label, d.Placement.Kind)
			for _, r := range d.Placement.Here {
				hi := r.Hi
				if hi <= r.Lo {
					hi = r.Lo
				}
				fmt.Fprintf(&b, ",r%d:%d-%d", r.Chapter, r.Lo, hi)
			}
			b.WriteByte(';')
		}
	}
	fold("n", p.Notes)
	fold("u", p.Unplaced)
	p.noteFP = b.String()
	if p.noteFP == "" {
		p.noteFP = "0"
	}
	var open strings.Builder
	for _, d := range p.Notes {
		if d.Open {
			fmt.Fprintf(&open, "o%d;", d.Note.ID)
		}
	}
	p.Fingerprint = p.noteFP + "&" + open.String() + "&" + p.Tints.fingerprint()
}

// --- the Apple sticker's text (S8) -------------------------------------------
//
// The first plural release ships the Apple surfaces through the EXISTING
// single-sticker ABI (bibleTextSetNote / bibleTextMacSetNote): the open note
// keeps the native sticker exactly as before, and the REST of the set is an
// honest count folded into the sticker's own text. Zero new ObjC, zero new C
// ABI, on the two surfaces where a defect is least testable
// (docs/NOTES_SCRAPBOOK.md, S8: "de-risk it"). A legal, non-lying subset:
// nothing is invisible — the sticker says how many more notes the passage
// carries, and how many cannot be shown in this translation — and platforms
// differ in richness, not truth; the Fyne banner draws the same set as chips.

// stickerCountLines are the count sentences, in the app's own quiet voice.
// openID is the note the sticker's bubble already shows (0 for a mirror-only
// session note, which is in no plan — every plan note then counts as "more").
func stickerCountLines(plan chapterPlan, openID uint64) []string {
	others := 0
	for _, d := range plan.Notes {
		if d.Note.ID != openID {
			others++
		}
	}
	var lines []string
	switch {
	case others == 1:
		lines = append(lines, "1 more note on this passage")
	case others > 1:
		lines = append(lines, fmt.Sprintf("%d more notes on this passage", others))
	}
	switch u := len(plan.Unplaced); {
	case u == 1:
		lines = append(lines, "1 note cannot be shown in this translation")
	case u > 1:
		lines = append(lines, fmt.Sprintf("%d notes cannot be shown in this translation", u))
	}
	return lines
}

// appleStickerText is what pushNoteToPane hands the existing ABI: the mirror's
// note (the Apple projection, unchanged) plus the count lines, blank-line
// separated so they read as the sticker's own footer rather than as the
// sender's words running on. Everything here is folded by the Apple body
// fingerprint — the mirror through chapterFingerprint's own clauses, the
// counts through noteFP's per-note entries — so the sticker text cannot change
// without the body half changing (verified by test, not assumed).
func appleStickerText(state *AppState, plan chapterPlan) string {
	if state == nil || state.ActiveNote == "" {
		return ""
	}
	lines := stickerCountLines(plan, state.NoteID)
	if len(lines) == 0 {
		return state.ActiveNote
	}
	return state.ActiveNote + "\n\n" + strings.Join(lines, "\n")
}
