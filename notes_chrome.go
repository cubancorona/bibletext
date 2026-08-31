package bibletext

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
	c.ShownAs = receivedSetShownAs(plan, c, len(chapterNoteGroups(state, plan, verses)))
	return c
}
