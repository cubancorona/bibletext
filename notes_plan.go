package bibletext

// The chapter plan (docs/NOTES_SPEC.md#chapter-plan-and-presentation-state).
// The model goes plural; the view does not, yet.
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
// Open, Notes still present, and NOTHING is written. That is the rule
// "minimize while search results are being displayed, then restore" made
// mechanical: a suppression that restores is only possible because it never
// wrote to the store (docs/NOTES_SPEC.md#chapter-plan-and-presentation-state).
//
// READ-ONLY, BY CONSTRUCTION. buildChapterPlan mutates nothing: not the store,
// not AppState, not the mark. The only store write any focus change may make is
// an explicit Hide's Minimized, and that happens in the verb, not here.

import (
	"fmt"
	"sort"
	"strings"
)

// planOpenLimit is how many notes may be expanded at once — the TEMPORARY cap:
// at most one, and ZERO is legal (an explicit minimize is honoured and nothing
// auto-expands). It lives HERE and nowhere else, as a build constant rather
// than a preference, and it writes nothing: the cap is maintained by the view
// and leaves zero bytes in the store, so lifting it a year from now cannot
// greet a reader with chips they never closed.
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
	// and the rest of the set rides in its WHO line as an honest count
	// (appleStickerPush). It is also the note the cap opens (the plan's
	// Open), so the bubble and the mirror can never disagree.
	display int

	// Own is YOUR OWN note, drawn on the passage only while noteFocus names
	// it — the reader asked to see it, from the notes browser or by tapping
	// their own link, and navigating away puts it back out of sight because
	// navigation resets focus (state.go, resetNoteFocus).
	//
	// IT IS A SLOT, NOT A MEMBER OF Notes, and that is the whole design. A
	// member that exists only while focused would make "K of N on this
	// passage" change under the reader's finger — N is a property of the
	// PASSAGE, not of what they happen to be looking at — and would enter the
	// next-tap rotation, where one tap past it strands it with no chip and no
	// way back (advanceNoteFocus walks Notes). Kept in its own slot, the
	// counts stay received-only and honest, the rotation is untouched, and
	// every surface can draw it by asking one question.
	Own    drawnNote
	HasOwn bool
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
// there is no second copy
// (docs/NOTES_SPEC.md#chapter-plan-and-presentation-state).
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
	focusedOwn := state.noteFocus.set && state.noteFocus.id != 0
	for _, n := range s.notes {
		// Own notes are not drawn automatically. An explicitly focused own note
		// is drawn in its own slot until navigation clears that focus. Everything
		// else about own notes is
		// unchanged; they never join Notes, never affect the counts, and never
		// become the default display.
		if n.Kind == noteKindMine {
			if !focusedOwn || n.ID != state.noteFocus.id || !displayableNote(n) || n.Book != book {
				continue
			}
			pl := resolveNoteAnchor(n, versionID, bible)
			if _, on := placementRunOn(pl, chapter); !on {
				continue
			}
			label := ""
			if pl.Kind != placedNative {
				label = noteVersionAbbrev(n.VersionID)
			}
			plan.Own, plan.HasOwn = drawnNote{Note: n, Placement: pl, Label: label, Open: true}, true
			continue
		}
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
// OPEN, THE DISPLAY CHOICE AND EACH NOTE'S MINIMIZED FLAG ARE FOLDED — INTO
// THE WHOLE, NOT INTO noteFP. S7 deliberately omitted Open because no pixel
// depended on it; S8 draws it (the banner's bubble vs chip is Open), so the S7
// trap is sprung in the same commit: the presentation joins Fingerprint as its
// own middle section. All three stay OUT of noteFP on purpose — noteFP is the
// Apple BODY half (chapterBodyFingerprint, reading.go), the identity of the
// imported NSAttributedString, and the imported string does not depend on
// WHICH note is expanded: the sticker is the native side's own subviews,
// re-derived against the string already on screen whenever the pushed tuple
// changes (bibleTextSetNote / bibleTextMacSetNote compare-and-refresh), and
// the wash is the tint half. Folding any of them into the body half would
// turn a suppression flip, an explicit close, a Hide/Show, or the next-tap's
// focus advance (advanceNoteFocus — the who-line's own selector) into a full
// NSAttributedString re-import, the exact cost the body/tint split exists to
// avoid. What noteFP DOES fold is the set itself — which notes exist here,
// their text lengths, labels and resolved runs — so an arrival or a delete
// still re-imports (the honest price of a changed chapter identity), while a
// change of presentation among unchanged notes rides the cheap paths. The
// body-vs-push contract is pinned by
// TestAppleStickerPushIsFoldedByTheBodyFingerprint and
// TestNextTapLeavesTheBodyFingerprintAlone.
func (p *chapterPlan) foldFingerprint() {
	var b strings.Builder
	if p.Notice != "" {
		fmt.Fprintf(&b, "N%q;", p.Notice)
	}
	fold := func(prefix string, list []drawnNote) {
		for _, d := range list {
			fmt.Fprintf(&b, "%s%d.%d.%s.%d", prefix, d.Note.ID, len(d.Note.Text), d.Label, d.Placement.Kind)
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
	// The presentation section: the display choice, every stored minimize, and
	// Open — everything a surface draws that is not the imported body. Slices
	// in the plan's stable order, so it is as flap-proof as the note half.
	var pres strings.Builder
	if p.display >= 0 {
		fmt.Fprintf(&pres, "d%d;", p.Notes[p.display].Note.ID)
	}
	for _, list := range [][]drawnNote{p.Notes, p.Unplaced} {
		for _, d := range list {
			if d.Note.Minimized {
				fmt.Fprintf(&pres, "m%d;", d.Note.ID)
			}
			if d.Open {
				fmt.Fprintf(&pres, "o%d;", d.Note.ID)
			}
		}
	}
	p.Fingerprint = p.noteFP + "&" + pres.String() + "&" + p.Tints.fingerprint()
}

// --- the Apple sticker's push (S8 count, S9 byline) --------------------------
//
// The Apple surfaces still ride the single-sticker ABI (bibleTextSetNote /
// bibleTextMacSetNote), and since S9 that ABI carries a WHO string beside the
// text: the bubble holds ONLY the sender's words, and everything the app
// itself has to say — the byline, the honest counts, the R4 sentence — is the
// WHO line, the sticker's own chrome, attributed to nobody but the app. S8
// folded the counts into the body through the then-frozen ABI and recorded
// the lie it bent ("1 more note on this passage" could read as something the
// sender typed); S9 moves that chrome into the dedicated WHO field.
//
// appleStickerPush is the WHOLE pushed presentation, computed in one place so
// the two panes cannot diverge:
//
//   - text — the sender's words, alone. "" when no note is open here.
//   - who  — expanded: senderByline + " · K of N on this passage" when the
//     plan holds more + " · U not shown here" when unplaced exist. Pill:
//     "Note" / "Notes · N" (+" · U not shown"), so minimizing the open note
//     preserves the rest of the set's visible count. An
//     unplaced-ONLY chapter pushes text="" and the sentence as the who, and
//     the native side collapses to the pill for it — no sender text exists,
//     and an empty sender bubble must never render. Within the ABI, non-empty
//     who text with an empty body selects the pill presentation.
//   - pill — minimized, OR the plan is suppressed (a foreign mark owns the
//     page): the sticker stands down to the pill and restores by itself when
//     the mark clears, matching the banner's chips-only presentation. Nothing
//     is stored for this temporary suppression. Also forced for unplaced-only.
//
// WHAT GATES THE REDRAW. The native side compares the pushed tuple itself and
// refreshes the sticker alone when it changed (btIOSRefreshNote /
// btMacRefreshNote) — re-deriving band, subviews and placement against the
// attributed string already on screen. That is the ONLY gate the sticker
// needs: since S10 the body fingerprint folds the note SET (which notes exist
// here, their lengths, labels, runs) and deliberately NOT the presentation
// (which one is displayed, who is minimized, Open — foldFingerprint's middle
// section), so a suppression flip, a Hide/Show, and the next-tap's focus
// advance are all sticker-only repaints plus (where the mark moved) a tint
// mutation — never a chapter re-import. An arrival or a delete still changes
// the set, still moves the body half, and still re-imports. Pinned by
// TestAppleStickerPushIsFoldedByTheBodyFingerprint and
// TestNextTapLeavesTheBodyFingerprintAlone.

// appleStickerPush composes what pushNoteToPane hands the native ABI. next is
// S10's addition to the tuple: the expanded sticker's count region is a
// CONTROL — a tap advances focus to the next note in the plan's stable order
// (advanceNoteFocus) — exactly when there is more than one placed note to
// advance through. The pill never carries it (tapping the pill opens the
// focused note, as always), and the native side draws the affordance (the
// accent on the counts span, a chevron) only when this is true.
func appleStickerPush(state *AppState, plan chapterPlan) (text, who string, pill, next bool) {
	if state == nil {
		return "", "", false, false
	}
	unplaced := len(plan.Unplaced)
	if state.ActiveNote == "" {
		if unplaced == 0 {
			return "", "", false, false // nothing here: no sticker at all
		}
		return "", stickerUnplacedOnlyWho(unplaced), true, false
	}

	// The open note's 1-based position in the plan's stable order, and the
	// total. A mirror-only session note (an arrival the store refused) is in
	// no plan: it leads the count and every plan note counts after it.
	// A focused own note is not a member of this passage's received-note set, so
	// it neither joins nor leads the count. "K of N on this passage" describes
	// received notes, and displaying an own note must not change N. It carries
	// only its byline ("Note from you") and has no next-tap because it is not in
	// the rotation.
	if plan.HasOwn && state.NoteID != 0 && state.NoteID == plan.Own.Note.ID {
		who = senderByline(plan.Own.Note)
		if unplaced > 0 {
			who += fmt.Sprintf(" · %d not shown here", unplaced)
		}
		if state.NoteMinimized || notesSuppressed(state) {
			return state.ActiveNote, who, true, false
		}
		return state.ActiveNote, who, false, false
	}

	placed := len(plan.Notes)
	pos, inPlan := 1, false
	for i := range plan.Notes {
		if state.NoteID != 0 && plan.Notes[i].Note.ID == state.NoteID {
			pos, inPlan = i+1, true
			break
		}
	}
	if !inPlan {
		placed++
	}

	if state.NoteMinimized || notesSuppressed(state) {
		return state.ActiveNote, stickerPillWho(placed, unplaced), true, false
	}

	var n StoredNote // zero value reads as a received note: "Note from Friend"
	if inPlan && plan.display >= 0 {
		n = plan.Notes[plan.display].Note
	} else if state.NoteID != 0 {
		// MIRROR-ONLY, and it may still be YOUR note. This arm runs when the
		// note is not in this chapter's plan — an arrival filed on a passage
		// this canon cannot reach, so the chapter was clamped. The zero value
		// above reads as a received note, which is right for a stranger's note
		// and wrong for your own: it would attribute your words to "Friend" on
		// the one path where nothing else can correct it. Ask the store whose
		// note this actually is.
		if own, ok := findNoteByID(appPrefs(), state.NoteID); ok {
			n = own
		}
	}
	who = senderByline(n)
	if placed > 1 {
		who += fmt.Sprintf(" · %d of %d on this passage", pos, placed)
	}
	if unplaced > 0 {
		who += fmt.Sprintf(" · %d not shown here", unplaced)
	}
	return state.ActiveNote, who, false, placed > 1
}

// nextNoteFocusID is the identity of the note AFTER the one on the sticker, in
// the plan's stable order, wrapping past the end — the note a next-tap opens.
// 0 when there is nothing to advance to (fewer than two candidates).
//
// The order is the WHO line's own: the plan's stable order, with a mirror-only
// session note (state.NoteID absent from the plan — an arrival the store
// refused) leading the count exactly as appleStickerPush counts it, so "2 of
// 3" always names the note the first tap lands on. A mirror-only note cannot
// be tapped BACK to — it has no stored identity for focus to name — so the
// wrap lands on the plan's first note: the honest floor for a note that
// exists nowhere but the mirror. Unplaced notes are not in the rotation; they
// have nothing on this page to open (their home is the notes browser).
func nextNoteFocusID(state *AppState, plan chapterPlan) uint64 {
	if state == nil || len(plan.Notes) == 0 {
		return 0
	}
	if state.NoteID != 0 {
		for i := range plan.Notes {
			if plan.Notes[i].Note.ID == state.NoteID {
				if len(plan.Notes) < 2 {
					return 0
				}
				return plan.Notes[(i+1)%len(plan.Notes)].Note.ID
			}
		}
	}
	// The sticker's note leads the count from outside the plan (or names
	// nothing): the next note is the plan's first.
	return plan.Notes[0].Note.ID
}

// advanceNoteFocus selects the next note in the plan's stable order. Selection
// restores a minimized target, clears a foreign mark, focuses the target by
// identity, and re-projects the state mirror from the plan.
//
// A note on another verse is also an explicit placement request. Without it,
// the note and wash move while the viewport stays at the previous note, making
// the selected note appear to vanish. Notes sharing an anchor keep the current
// viewport so cycling between them does not cause needless motion.
func advanceNoteFocus(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	id := nextNoteFocusID(state, plan)
	if id == 0 {
		return
	}
	for i := range plan.Notes {
		if plan.Notes[i].Note.ID == id {
			if plan.Notes[i].Note.Minimized {
				// The tap IS the Show verb for a stored-minimized note: the
				// one durable restore, by the note's own identity, handed
				// here by the plan and never rebuilt.
				setNoteMinimizedByID(appPrefs(), id, false)
			}
			break
		}
	}
	if state.mark.live() && !state.mark.fromNote() {
		state.clearMark()
	}
	previousVerse := state.NoteVerseLo
	state.focusNote(id)
	applyNoteForCurrentChapter(state)
	if state.NoteVerseLo != previousVerse {
		state.forceReposition = true
	}
	// The newly selected note outranks a pending saved-position restore. The
	// platform render path either performs the requested placement or retains
	// the current viewport when both notes share an anchor.
	state.restore = nil
}

// androidStickerPush is the Android full-screen sticker's tuple —
// a thin alias of the Apple composition, so the WHO line, the honest counts,
// the pill labels and the derived suppression are BYTE-identical across the
// three native stickers and can never drift. It exists as a name (rather than
// Android calling appleStickerPush directly) so the seam is visible and
// host-pinned: assertStickerAgreesWithStore (notes_verb_screen_test.go) holds
// the two to equality across every verb, and a future Android-only divergence
// has to announce itself here.
func androidStickerPush(state *AppState, plan chapterPlan) (text, who string, pill, next bool) {
	return appleStickerPush(state, plan)
}

// stickerPillWho is the collapsed sticker's label: short (the pill sizes to
// it), but the whole set — "Note", "Notes · 3", "Note · 2 not shown".
func stickerPillWho(placed, unplaced int) string {
	s := "Note"
	if placed > 1 {
		s = fmt.Sprintf("Notes · %d", placed)
	}
	if unplaced > 0 {
		s += fmt.Sprintf(" · %d not shown", unplaced)
	}
	return s
}

// stickerUnplacedOnlyWho is the R4-only chapter's pill sentence — the same
// copy the S8 count line used, now in the chrome where it belongs.
func stickerUnplacedOnlyWho(u int) string {
	if u == 1 {
		return "1 note cannot be shown in this translation"
	}
	return fmt.Sprintf("%d notes cannot be shown in this translation", u)
}

// --- Notes grouped by the paragraph that carries them -----------------------
//
// A note anchors to a VERSE, but the band that holds its sticker opens above
// the whole PARAGRAPH carrying that verse — paraCarriesVerse
// (reading_styled_layout.go) is the join, and iOS/macOS reach the same place
// through paragraphSpacingBefore. That is why a bubble for a note on v2 sits
// above v1: they share a paragraph.
//
// The collapsed state has no equivalent join. One pill is drawn, anchored at
// one paragraph, labelled with a count of the WHOLE CHAPTER (stickerPillWho
// takes len(plan.Notes)). So five notes — two in one paragraph, three in
// another — produce a single pill reading "Notes · 5" over one of the two,
// and the other paragraph is indistinguishable from one carrying no notes.
// The label's scope and the pill's position disagree, and the reader can only
// see the position.
//
// This groups the chapter's placed notes the way the band already places
// them, so a surface can draw one pill per noted paragraph with that
// paragraph's own count. Paragraphs come from groupVersesIntoParagraphs, which
// every surface already shares: the styled pane and Android call it directly,
// iOS and macOS receive its output inside buildChapterHTML.
type noteParagraphGroup struct {
	// ParaIndex is the paragraph's position in the chapter, so groups come out
	// in reading order rather than in the plan's newest-first order.
	ParaIndex int
	// BandVerse is the earliest anchor verse in the group — the verse the band
	// is found by, and the one a surface should open the band above.
	BandVerse int
	// Notes are every note this paragraph carries, keeping the plan's order.
	Notes []drawnNote
}

// noteAnchorVerse is the verse a drawn note hangs from: the first verse of its
// first run that lands in this chapter. Zero when it has no home here.
func noteAnchorVerse(d drawnNote) int {
	for _, r := range d.Placement.Here {
		if r.Lo > 0 {
			return r.Lo
		}
	}
	return 0
}

// groupNotesByParagraph collects notes under the paragraph that carries each
// one. A note whose anchor no paragraph carries is dropped rather than forced
// into a neighbour: it has no band to open, and inventing one would put a
// sticker over a passage the note is not about.
func groupNotesByParagraph(paras [][]Verse, notes []drawnNote) []noteParagraphGroup {
	if len(paras) == 0 || len(notes) == 0 {
		return nil
	}
	byPara := map[int]int{} // paragraph index -> position in out
	var out []noteParagraphGroup
	for _, d := range notes {
		v := noteAnchorVerse(d)
		if v == 0 {
			continue
		}
		pi := -1
		for i, para := range paras {
			if paraCarriesVerse(para, v) {
				pi = i
				break
			}
		}
		if pi < 0 {
			continue
		}
		at, seen := byPara[pi]
		if !seen {
			byPara[pi] = len(out)
			out = append(out, noteParagraphGroup{ParaIndex: pi, BandVerse: v})
			at = len(out) - 1
		}
		if v < out[at].BandVerse {
			out[at].BandVerse = v
		}
		out[at].Notes = append(out[at].Notes, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ParaIndex < out[j].ParaIndex })
	return out
}

// notesPillPerParagraph switches the collapsed state between the shipped model
// and the one being refined:
//
//	false — ONE sticker for the chapter, anchored at plan.display's paragraph,
//	        labelled with the chapter's whole count (stickerPillWho). What
//	        ships today, and what every surface draws.
//	true  — one pill per noted PARAGRAPH, each carrying that paragraph's own
//	        count, from groupNotesByParagraph above.
//
// A var, not a build constant, so the dev build can flip it while the app is
// running and the two can be compared on the same chapter without a reinstall.
// It defaults to the shipped behaviour and no release surface writes it, so
// nothing changes for a reader until a surface is taught to draw the groups
// AND this is flipped deliberately.
//
// It writes nothing and is not persisted: a relaunch is back to the shipped
// model, which is the right default for a switch that exists to be experimented
// with rather than configured.
var notesPillPerParagraph = false

// chapterNoteGroups is the collapsed state's groups for the current chapter, or
// nil when the shipped single-sticker model is in force. Surfaces ask this one
// question rather than reaching for the flag and the grouping separately, so
// there is one place to change when the flag eventually goes.
func chapterNoteGroups(state *AppState, plan chapterPlan, verses []Verse) []noteParagraphGroup {
	if !notesPillPerParagraph || state == nil {
		return nil
	}
	return groupNotesByParagraph(groupVersesIntoParagraphs(verses), plan.Notes)
}

// focusNoteAtVerse opens the note belonging to the paragraph a pill sits on.
// It is the per-paragraph pill's verb, and the thing the single pill could
// never do: that one could only restore whichever note the plan had chosen,
// wherever in the chapter it happened to be.
//
// The verse is a GROUP's band verse, not necessarily a note's own anchor, so
// the group is what is matched — the paragraph is the unit the reader tapped.
// An explicit minimize is lifted, because tapping the pill IS the Show verb
// (the same rule advanceNoteFocus follows for a stored-minimized note).
func focusNoteAtVerse(state *AppState, verse int) {
	if state == nil || verse <= 0 || state.Bible == nil {
		return
	}
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	for _, g := range groupNotesByParagraph(groupVersesIntoParagraphs(verses), plan.Notes) {
		if g.BandVerse != verse || len(g.Notes) == 0 {
			continue
		}
		n := g.Notes[0].Note
		if n.Minimized {
			setNoteMinimizedByID(appPrefs(), n.ID, false)
		}
		state.focusNote(n.ID)
		applyNoteForCurrentChapter(state)
		return
	}
}
