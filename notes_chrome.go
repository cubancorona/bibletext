package bibletext

import "strings"

// notes_chrome.go — the ONE value every surface reads to draw a note.
//
// WHY THIS FILE EXISTS. Note chrome is drawn four times: the styled pane
// (Fyne canvas, Windows and Linux), two Apple panes (Objective-C in a cgo
// preamble, TextKit) and Android (HTML in a WebView). The MODEL behind them —
// chapterPlan, the stable order, the open cap, suppression, the who-line
// grammar — has been shared since S9 and has produced no defects. Everything
// each surface DERIVES for itself has produced several, and every one was an
// omission rather than a wrong decision: a fix applied to one surface and not
// the other three.
//
//	"land on the note"        fixed on iOS and macOS, two near-identical edits
//	restore over an arrival   fixed on iOS and macOS; Android had it all along
//	the tail on an anchorless note   fixed on the styled pane only
//	per-paragraph pills       exist on the styled pane only
//
// So the rule is: a decision is made ONCE, here, and a surface only renders it.
//
// ADMISSION RULE, and it is the only thing between this and a forty-field blob:
// a field may join only if it is a pure function of (state, plan, verses).
// Anything needing a wrapped height, a string width or a fragment rect stays in
// the renderer that measured it. The drawing is NOT unified and cannot be —
// TextKit, a canvas and HTML are irreducibly different.
//
// The full design, including the steps not yet taken, is in
// docs/NOTE_CHROME_UNIFICATION.md.

// noteVerbSet is which controls the card carries. A received note is deleted
// and shows a bin; an own note is only dismissed and shows ✕. The glyph is a
// promise about what the press does, so the two must never be chosen
// separately from the verb that runs.
type noteVerbSet uint8

const (
	noteVerbsNone     noteVerbSet = iota
	noteVerbsReceived             // minimize (en dash) beside delete (bin)
	noteVerbsOwn                  // dismiss (✕) alone
)

func (v noteVerbSet) String() string {
	return [...]string{"none", "received", "own"}[v]
}

// noteChevron is the affordance appended after the counts when the counts are a
// control. One literal: it was four, and they had already drifted (Android
// used two spaces).
const noteChevron = " ›"

// noteChrome is every decision about a note's chrome that is neither a
// measurement nor a drawing.
//
// The first six fields are the S9 tuple, unchanged in meaning and already
// crossing the ABI today (appleStickerPush). The rest are decisions each
// surface currently makes for itself.
type noteChrome struct {
	Text   string // the sender's words, ALONE. "" = nothing open here
	Who    string // the app's chrome: byline · counts · "N not shown here"
	Pill   bool   // minimized, suppressed, or unplaced-only
	Next   bool   // the counts region is a CONTROL (more than one placed note)
	Own    bool   // authored locally and explicitly focused
	Anchor int    // the verse the band opens above; 0 = park at the chapter top

	// Counts is the SUBSTRING OF Who that is a control — the counts phrase and
	// its chevron, together, exactly as they appear in the line. "" when the
	// counts are not a control.
	//
	// It is a field rather than a method because it is not derivable from the
	// tuple ALONE: a surface can only find it by parsing Who, and four surfaces
	// parsing the same grammar in four languages is what this whole plan is
	// about. Three of them had transcribed the first-separator split
	// (btIOSWhoCountRange, btMacWhoCountRange, styledWhoSplit) and the fourth
	// had never had it, so Android drew no accent at all. Composed here, once,
	// each surface only has to FIND a string it was handed — a backwards search,
	// which survives a pane that has already ellipsised the sender half.
	Counts string

	// Arrival is WHERE this render places the view, and ArrivalVerse is the
	// verse it is expressed against — the target for arriveVerse, and the
	// fallback for arriveBand when a renderer cannot resolve its reservation
	// yet. Decided by chapterNoteArrival (notes_arrival.go); five surfaces
	// used to decide it themselves, in five dialects, and they were never the
	// same rule.
	Arrival      noteArrival
	ArrivalVerse int

	// Bands is every reservation this chapter needs, in drawing order: WHICH
	// group, WHERE it hangs, and HOW MANY notes it speaks for.
	//
	// NOT how tall. A band's height is measured by each surface against its own
	// fonts and width, and measurement is exactly what this value may not carry
	// — the admission rule above is "a pure function of (state, plan, verses)",
	// and a height is a function of a pane. So the identity crosses and the
	// geometry stays local, which is also the split that lets the natives adopt
	// this without a measurement callback.
	//
	// Empty unless the per-paragraph gate is on, because chapterNoteGroups is:
	// with one chapter pill there is one reservation and Anchor already names
	// it. Every surface therefore runs this list at length 0 or 1 today.
	Bands []noteBandSpec

	// EVERYTHING DERIVABLE FROM THE TUPLE IS A METHOD, NOT A FIELD.
	//
	// The first draft made presence, collapsedness, the tail and the verb set
	// into fields set by the composer. That is a trap in a package that builds
	// this value with composite literals: a literal that omits a field gets the
	// zero value silently, and `present()` reading a field made every existing
	// literal report "no sticker" — pills stopped being drawn at all. The
	// literals were not wrong; the shape was.
	//
	// So a decision that is a pure function of the tuple is computed on demand
	// below, and cannot disagree with the tuple it came from. Only ShownAs is a
	// field, because it needs the plan and the paragraph groups, which a literal
	// genuinely cannot supply.

	// ShownAs is HOW this chapter's received notes are represented (N9). It was
	// typed on the styled pane's own struct, which is why X16 — the natives
	// representing the set nowhere while an own note is open — is a documented
	// hole rather than a value they can read.
	ShownAs receivedShownAs
}

// present reports whether a sticker exists at all. Four surfaces spelled this
// out for themselves over a tuple this package had already composed.
func (c noteChrome) present() bool { return c.Text != "" || c.Who != "" }

// collapsed reports whether this is the closed presentation. Four spellings,
// and they were not the same expression: the styled one omitted the second
// clause, harmless only because appleStickerPush never returns that pair.
func (c noteChrome) collapsed() bool { return c.Pill || (c.Who != "" && c.Text == "") }

// hasTail reports whether this card points at a passage — NOT whether it is
// expanded. The two were conflated, and the tail's depth is still gated on the
// wrong one inside the Apple band formulae. An anchorless note is parked at the
// chapter top, so a tail there claims verse 1.
func (c noteChrome) hasTail() bool { return c.Anchor > 0 }

// verbs is which controls the card carries, from the same predicate the verbs
// themselves branch on. The glyph is a promise about what the press does.
func (c noteChrome) verbs() noteVerbSet {
	switch {
	case !c.present():
		return noteVerbsNone
	case c.Own:
		return noteVerbsOwn
	default:
		return noteVerbsReceived
	}
}

// chevron is the counts affordance, or "" when the counts are not a control.
// One literal: it was four, and they had already drifted.
func (c noteChrome) chevron() string {
	if c.Next {
		return noteChevron
	}
	return ""
}

// receivedSetShownAs is the ONE answer, so the pane and the enumeration cannot
// disagree about what the reader is looking at. groups is the number of noted
// paragraphs chapterNoteGroups found, which is zero whenever the pill gate is
// off — so the gate needs no separate argument here.
// stickerIsTheReceivedSet reports whether the single sticker IS this chapter's
// received notes in their collapsed form — the ONE state in which the pills are
// a second spelling of it and must replace it.
//
// Pill alone is not that question. Pill means "the sticker is
// CLOSED", and a focused own note is closed too whenever a foreign mark
// suppresses the chapter — appleStickerPush's own-note arm returns pill=true
// under notesSuppressed (notes_plan.go). Keying on Pill therefore treated the
// reader's own note as the friends' chip: reachable by arriving at the chapter
// through a search result and then opening your own note, which blanked it.
func stickerIsTheReceivedSet(note noteChrome) bool {
	return note.Pill && !note.Own
}

func receivedSetShownAs(plan chapterPlan, note noteChrome, groups int) receivedShownAs {
	if len(plan.Notes) == 0 || !note.present() {
		return shownAsNothing
	}
	// OWN FIRST, and the order is the fix rather than a preference: Own says
	// WHAT the sticker is showing, Pill only says whether it is closed, and the
	// two overlap under suppression. The narrower fact has to win.
	switch {
	case note.Own:
		// The sticker is busy with a note that is NOT in this set and carries no
		// count of it, so only the pills can speak for the set — at any group
		// count, including one. Where there is no pill row (the three native
		// surfaces, or the gate off) groups is zero and the set is represented
		// nowhere: X16, and the reason this function returns a value rather
		// than a bool.
		if groups >= 1 {
			return shownAsPills
		}
		return shownAsNothing
	case note.Pill:
		// The sticker IS the set's collapsed form. The pills replace it only
		// where they say strictly more: two or more noted paragraphs. With one,
		// the sticker's count and its position already agree.
		if groups >= 2 {
			return shownAsPills
		}
		return shownAsSticker
	default:
		// An open received note: its who line carries the count.
		return shownAsCount
	}
}

// chapterNoteChrome composes the one value, for every surface.
//
// It calls appleStickerPush for the six tuple fields rather than re-deriving
// them, so that function keeps its signature and its other two callers
// (androidStickerPush, styledStickerPush) are untouched by construction.
func chapterNoteChrome(state *AppState, plan chapterPlan, verses []Verse) noteChrome {
	if state == nil || !notesFeatureOn(state) {
		return noteChrome{}
	}
	text, who, pill, next := appleStickerPush(state, plan)
	c := noteChrome{
		Text:   text,
		Who:    who,
		Pill:   pill,
		Next:   next,
		Anchor: state.NoteVerseLo,
		// The predicate the VERBS branch on. Asking the plan instead gave a
		// second answer that disagreed wherever the mirror still named an own
		// note the plan no longer offered — a bin drawn on a note the press
		// would only dismiss.
		Own: isOwnLiveNote(state),
	}
	// The chevron is part of the LINE, not something four natives append to it.
	// It was four literals — Android's was two spaces wide — and the Go constant
	// naming it was read by nobody.
	if c.Next {
		c.Who += noteChevron
	}
	c.Counts = noteCountsSpan(c.Who, c.Next)
	// WHERE THE VIEW GOES, from the same tuple. following/restoreArmed/explicit
	// are facts about the render, not about the note, so they are read here
	// rather than threaded through every caller.
	groups := chapterNoteGroups(state, plan, verses)
	c.Bands = noteBandSpecs(groups)
	c.Arrival, c.ArrivalVerse = chapterNoteArrival(state, c, verses, groups,
		gAudio != nil && gAudio.readAlongFollowActive(),
		state.restore != nil, state.forceReposition)
	c.ShownAs = receivedSetShownAs(plan, c, len(groups))
	return c
}

// noteCountsSpan is the part of a who line that is a CONTROL: the counts phrase
// with its chevron, returned as the exact substring so a surface can find it by
// searching rather than by re-deriving the grammar.
//
// The line is "<byline> · <counts> [· <N> not shown here]" plus the chevron, so
// the span runs from after the first separator to the next one, or to the end.
// That is safe against a sender's name by construction: sanitizeSenderName maps
// U+00B7 to '-' (notes_byline.go), so the chrome's own grammar is not available
// to names — the fact TestCountsSpanCannotBeForgedByASender pins.
//
// Returns "" when the counts are not a control, which is the only state in
// which a surface should draw no accent.
func noteCountsSpan(who string, next bool) string {
	if !next {
		return ""
	}
	i := strings.Index(who, noteWhoSep)
	if i < 0 {
		return ""
	}
	span := who[i+len(noteWhoSep):]
	if j := strings.Index(span, noteWhoSep); j >= 0 {
		span = span[:j]
	}
	return span
}

// noteWhoSep is the who line's own separator. One literal: the split was
// transcribed into three languages, and the fit rule reads it too.
const noteWhoSep = " · "

// noteBandSpec is one reservation: the group it belongs to, the verse whose
// paragraph carries it, and the counts its label speaks for. Height is absent
// on purpose (see noteChrome.Bands).
type noteBandSpec struct {
	// Key is the group's identity, and the ONLY thing a press or a placement
	// may match on. The chapter-top group deliberately shares paragraph 0's
	// verse, so a verse names no single band — a defect that reached the pills'
	// own verb before it was keyed.
	Key int
	// Verse is the verse whose paragraph this band hangs above. 0 = the chapter
	// top, the anchorless placement, which is also what suppresses the tail.
	Verse int
	// Count and Unplaced are what the band's label says: how many notes this
	// paragraph carries, and how many of the chapter's notes this translation
	// cannot place at all.
	Count, Unplaced int
}

// noteBandSpecs is the reservation list for a chapter's note groups. One place,
// so a surface adopting the plural model gets the same list in the same order
// as every other, rather than deriving its own from the groups.
func noteBandSpecs(groups []noteParagraphGroup) []noteBandSpec {
	if len(groups) == 0 {
		return nil
	}
	out := make([]noteBandSpec, 0, len(groups))
	for _, g := range groups {
		verse := g.BandVerse
		if g.ParaIndex == chapterTopGroup {
			// The chapter-top group speaks for notes that point at no
			// paragraph. Zero is the anchorless placement, exactly as on the
			// single card, and it is what takes the tail away.
			verse = 0
		}
		out = append(out, noteBandSpec{
			Key: g.Key, Verse: verse, Count: len(g.Notes), Unplaced: g.Unplaced,
		})
	}
	return out
}
