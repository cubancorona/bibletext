# The notes subsystem, reworked — SUPERSEDED

> **Superseded 2026-08-15 by [`NOTES_SCRAPBOOK.md`](NOTES_SCRAPBOOK.md).**
> This design assumed at most one note per chapter per translation, and no
> sender. The owner then specified a scrapbook: many notes over years, from
> different people, across translations, with overlapping highlights. Kept for
> the reasoning that survived — the cap living in one function, the projection
> type, the store left alone — and because its staging order for the HIGHLIGHT
> family (S1/S2 there) was executed and is sound. Do not build from it.

# The notes subsystem, reworked

> **Companion to [`NOTES_STATE.md`](NOTES_STATE.md).** That document is the
> OBSERVED record — what the code does today, 110 violations, twelve named
> defects, measured against `31bc97630`. This one is the INTENDED design: what
> replaces it, in what order, and what is deliberately left alone.
>
> Nothing here is implemented yet beyond **S0**. Do not read a type in §2 as a
> type that exists.

## 0. Why a rework and not a sixth patch

Five defects in one day, and then a sixth. The pattern is not that the fixes were
wrong — every one of them was correct and is still in the tree. The pattern is
that **each fix exposed the next defect**, because the model underneath cannot
express what the subsystem is being asked to do:

| Fix | What it exposed |
|---|---|
| notes follow the passage across translations | a note DISPLAYED under a translation it is not STORED under |
| ...which made Hide and Delete address the wrong key | `X1`, `X2` — the wrong person's message destroyed |
| ...fixed by writing all four mirror fields together | `X12` — Delete works, so the note behind it surfaces unannounced |

A Delete that misses cannot expose the note standing behind it. Fixing the verb
was strictly an improvement — the reader no longer destroys a message they were
not looking at — and it was still not the end, because the thing actually wrong
is that `loadNote` returns **one** note from a store that holds several, and the
owner has since specified a set.

The convention that was supposed to hold the mirror together leaked the same day
it was written: `dev_links_on.go:145` assigns three of the four fields. It is
dev-only and ships nothing, which is why it is evidence rather than a bug — a
rule that cannot survive one day of its own author is not a rule.

## 1. The model in one sentence

**A chapter holds a SET of notes; each note carries the key it is stored under
and a placement already renumbered into the translation being read; how many may
be open at once is a rule the RENDER applies and never writes down; and every
mark on the page carries who put it there and which numbering it is in.**

Three missing facts, each killing a family of defects:

| Missing fact | Where it is missing today | Kills |
|---|---|---|
| **the plural** | `loadNote` returns `(SharedNote, bool)` — `notes_store.go:165-171` | X6, X7, X12, NOTE_MASKED, COLLAPSED_MASK, NOTE_SUBSTITUTED, UNREACHABLE |
| **identity carried, not reconstructed** | `noteStoreVersion()` rebuilds a third of a key from where the reader stands — `notes_store.go:379-387` | X5, COLLAPSED_STUCK, and X1/X2 structurally rather than by guard |
| **the mark's origin and frame** | five writers into five undiscriminated fields — `state.go:65-69` | X4, X8, X9, X10, X11, ORPHAN_HL, GHOST_LOC, HL_FRAME |

**The store does not change.** `version|book|chapter`, `SharedNote`, `noteKey`,
`saveNote`, `deleteNote`, `setNoteMinimized`, `readNotesChecked`, `writeNotes`:
untouched. No migration, no new preferences key, nothing to roll back.

### What becomes structural, and what stays checked

| | Verdict | How |
|---|---|---|
| **N1** no mark without meaning | STRUCTURAL | `HasHighlightedVerse` is deleted. Absence *is* `Origin == hlNone`, so there is no flag to set false while the location stays behind. |
| **N2** a verb reaches what the reader aimed at | STRUCTURAL | `noteStoreVersion()` is deleted. Every verb takes a `NoteKey` handed to it by the surface that drew the note. There is no "current note" to be wrong about. |
| **N3** no silent substitution | STRUCTURAL via arity | Nothing is masked, so nothing is promoted. Which note is *expanded* falls back to none, never to another note. |
| **N4** nothing in the store is invisible | STRUCTURAL | The derive is one loop with **no `continue`**: every candidate lands in one of five `placementKind` arms, and all five are drawn. |
| **N5** explicit minimize honoured | STRUCTURAL for the persisted half | Only a reader's press writes `Minimized`. |
| **N6** mirror agrees with store | CHECKED | Cache coherence cannot be a type. Mitigated: one writer, re-derived on every store mutation. |
| **N7** one ruler | STRUCTURAL AT READ | `VerseSpan` carries the `VersionID` it is numbered in; the read accessor returns the span mapped into the current frame **or nothing** — an unrenumbered mark lights nothing rather than the wrong verse. |
| **N8** at most one expanded (temporary) | CHECKED, residue STRUCTURAL | §3. |

**Honest limit.** The app is one Go package, so unexported fields buy nothing
across files, and Go has no sum types. "Unrepresentable" here means: one
constructor, values never mutated after construction, accessors that force the
missing arm, and an enumeration that fails on any new incoherent state. That is
weaker than a type system and should not be described as if it were not.

## 2. Types

```go
// NoteKey is a note's IDENTITY: the key it is stored under. Constructed ONCE,
// at the derive, from the SharedNote itself — never rebuilt from where the
// reader happens to be standing. That reconstruction is X5.
//
// OPAQUE: nothing outside notes_store.go destructures it or builds one from
// parts, which is what makes widening the key later (the same-translation
// collision at notes_store.go:15-19) a change to this struct and one function
// rather than to every pane.
type NoteKey struct {
	VersionID string
	Book      string
	Chapter   int
}

// VerseSpan is a location AND the numbering it is expressed in. The frame is
// not optional: N7 is violated today because a verse number can be written
// without saying which translation numbers it that way.
type VerseSpan struct {
	VersionID string
	Book      string
	Chapter   int
	Lo, Hi    int // Lo == 0 means chapter-level
}

// placementKind is a SUM, not a bool. R4 has to write a sentence saying WHY a
// note has no home here, and "that book does not exist in this translation" and
// "the numbering does not line up" are different sentences. A bool forces every
// surface to invent the reason.
type placementKind int

const (
	placedHere              placementKind = iota // exact-key hit
	placedByMapping                              // followed, renumbered by MapVerse
	unplacedAbsent                               // verseMapAbsent
	unplacedIncommensurable                      // Greek Esther: a different book
	unplacedOtherChapter                         // maps into a chapter that is not this one
)

// ChapterNote is one note as the panes need it. Built ONLY by
// deriveChapterNotes; every field is computed at construction and never written
// again.
type ChapterNote struct {
	Key       NoteKey   // what the verbs address, and what R2 attributes
	Text      string    // UNTRUSTED — rendered as TEXT, never markup, always attributed
	At        VerseSpan // ALREADY mapped into the reading translation
	Placement placementKind
	Minimized bool  // PERSISTED. ONE meaning: the reader closed this.
	Received  int64 // the set's stable order
}

// drawnNote is the RENDER PROJECTION — a different type on purpose. Built once
// per render, consumed by the fingerprint and every surface, dead with the
// frame. `Open` lives HERE and nowhere else: a bool on ChapterNote would be a
// derived mirror with one writer and four readers, which is the quadruple's
// failure shape at quarter scale.
type drawnNote struct {
	Note  ChapterNote
	Open  bool
	Label string // the translation's display name — R2
}

// Mark replaces HasHighlightedVerse + the four location fields. Absence is
// Origin == hlNone, so GHOST_LOC is unwritable rather than merely inert.
type hlOrigin int

const (
	hlNone hlOrigin = iota
	hlNote
	hlSearch     // openSearchResultRange, and the notes browser through it
	hlVerseOfDay // goToVerseRange: verse of the day, cross-reference, Go-to
	hlLinkSpan   // a shared link's verse range
)

type Mark struct {
	Origin  hlOrigin
	At      VerseSpan
	NoteKey NoteKey // meaningful only when Origin == hlNote
}
```

In `AppState`, replacing `ActiveNote` / `NoteMinimized` / `NoteVerseLo` /
`NoteVersionID` **and** the five highlight fields:

```go
chapterNotes []ChapterNote // every note on this passage, stable order. nil = BARE.
                           // ONE writer: deriveChapterNotes. Read via accessor.
noteFocus    noteFocus     // session-only; goes inert when the cap lifts
mark         Mark
```

### Two types deliberately not written

- **No `Expanded bool` on `ChapterNote`.** A derived mirror inside the model is
  the exact shape that produced the quadruple. `drawnNote.Open` is computed
  *while walking the set*, so "open but absent from the set" cannot be said.
- **No index anywhere** — not into `chapterNotes`, not across cgo. An index is a
  `NoteKey` with the identity filed off, and it goes stale on precisely the
  events this subsystem gets wrong: a note arriving, a delete, a version switch.

## 3. The cap: maintained by the view, zero bytes of residue

The owner's rule is *"for now, at most one expanded (possibly none) — but we
will want to change this, so think ahead."* The design question is what is left
behind when the cap lifts.

**Cap by ACTION** — expanding B writes `Minimized=true` onto A — leaves a
counterfeit minimize on every note the reader ever had open while opening
another, **byte-identical to a genuine one**. The store has one bit where the
model would need two, so the information a migration would need was never
recorded. Lift the cap a year later and the reader opens a chapter to four chips
and no message, having never pressed minimize in their life. It also makes the
browser lie — `notes_browse.go:415-419` prints "Minimized in the chapter" from
that same bit — and turns a view gesture into a whole-blob write. Rejected.

**Cap by VIEW** — residue is zero bytes; every `Minimized=true` in the store was
typed by a reader.

**Where the cap lives:** one function, `drawnNotes(state) []drawnNote`. It is the
only thing in the tree that knows a number exists. Every surface consumes
`[]drawnNote` and asks nothing else, so lifting the cap is a change to one
function body and is invisible to the panes, to `ChapterNote`, and to the store.

## 4. The four surfaces that draw a note

A correction to how this has been described, including by me: **there are four
note-drawing surfaces, not five.**

| Surface | Draws | File |
|---|---|---|
| iOS sticker | the note | `reading_ios.go` (native `UIView`) |
| macOS sticker | the note | `reading_macos.go` (native `NSView`) |
| Fyne banner | the note — **Windows, Linux, Android** | `notes_banner.go` |
| browser | the note | `cmd/websitegen` |

`buildChapterHTML` and `buildChapterHTMLAndroid` draw the note's **highlight**
and reserve its **band** via `paragraphSpacingBefore`; neither emits note markup
(`android_chapter_html.go:75` is a comment, and nothing else matches). The
reasoning is recorded at `reading_ios.go:523-542`: the HTML importer drops
border, radius, padding and shadow, so the bubble has to be a native view.

Three HTML panes gate on `chapterRenderFingerprint` — iOS, macOS **and** Android
(`reading_android.go:491`).

## 5. Staging

Eight steps. Every one leaves the tree green, compiling on all platforms, and
shippable. Three are marked **STOP** — real resting points where the work can sit
for weeks without leaving the subsystem half-migrated. That matters more than
usual: a big-bang landing on a subsystem that produced six defects in two days is
itself the largest risk in the plan.

| Step | What | Closes | Cost |
|---|---|---|---|
| **S0** ✅ | Truth maintenance: land the harness, strike X1/X2, pin X12, correct the doc to 110 | — | done |
| **S1** | `Mark`: origin + identity. Fold five fields into one struct, delete `HasHighlightedVerse`, set `Origin` at all five writers | ORPHAN_HL, X4, X8, X9, X10, GHOST_LOC | 2d |
| | **STOP 1** — shippable alone, closes the oldest defect, zero pixels move | | |
| **S2** | `Mark`'s frame. `applyLoadedVersion` renumbers through `MapVerse` or clears | X11, HL_FRAME → origin space reaches **zero** | 1d |
| | **STOP 2** — the entire highlight family done, no visual change, no new UI to review | | |
| **S3** | Identity carried. `NoteKey` opaque, `noteStoreVersion()` deleted, verbs take a key | X5, COLLAPSED_STUCK; cements X1/X2 structurally | 2d |
| **S4** | The set. `deriveChapterNotes`, `placementKind`, `drawnNotes` + focus — behind a flag whose plan is still truncated to one note | nothing visible; the whole model lands | 3d |
| | **STOP 3** — where the risk lives, and it ships with the view unchanged | | |
| **S5** | Flip the flag on the Fyne banner: **Windows, Linux, Android go plural** | X7, X12, NOTE_MASKED, COLLAPSED_MASK, UNREACHABLE, NOTE_SUBSTITUTED + **R2, R4** → notes space reaches **zero** | 2d |
| **S6** | The Apple sticker draws the set: array C API, one band per anchor, stacking | full parity | 4–6d |
| **S7** | The browser bubble: shared builder, version label outside | the owner's browser directive | 1–2d |
| **S8** | The tap menu addresses the note under the finger (`gHasNote` → per-verse key map) | the last place a verb can miss | 1d |

**Why the highlight first.** It is 46 of the 110 violations, it needs no UI
change at all, and it is independently shippable. **Why S4 and S5 must not be
merged:** the whole point of S4 is that the model can be wrong in a way no reader
sees, and the enumeration can catch it *there*. Merging them puts a new model and
a new view in one commit on a subsystem where the last five commits each
introduced a defect while fixing the previous one.

**Apple during S5.** iOS/macOS stay capped at one drawn note **plus an honest
count chip** — "2 more notes on this passage" — pushed through the existing
single-sticker C API with no native work. N4 is satisfied because nothing is
invisible; the platforms differ in richness rather than in truth.

## 6. Risks

1. **The Apple multi-band layout is the real cost and no test can see it.** The C
   side holds exactly one `gNoteView` / `gNoteAnchorVerse` / `gNoteBandH`, and
   the "layout converges in ONE pass" argument was reasoned for a single band.
   Multiple bands at multiple anchors, stacking where two notes share one, is a
   new layout problem behind cgo. Staged last, behind a phase that is honest
   without it — but if anything overruns, this is it.
2. **`noteFocus` has the same shape as `NoteVersionID`, which produced X1.**
   Milder class — a forgotten write opens the wrong bubble, it does not destroy a
   message — because focus is never a verb's address, is scoped to
   (version, book, chapter), and self-clears on navigation. **The harness must
   grow a focus axis** (unset / none / key-present / key-absent / key-minimized)
   or this design has moved the unwatched variable rather than removed it.
3. **A flapping fingerprint is not benign.** If the render list is ever built from
   a map range instead of the derive's order, the fingerprint differs run to run,
   the skip never fires, and every navigation pays an HTML rebuild plus an
   `NSAttributedString` re-import on three gated panes. That is the iOS
   scroll-lag budget the gate exists for, and it would present as a vague
   performance regression rather than a bug.
4. **Deleting `loadNote` touches 15 test references.** Staged: it survives S3
   called by the new derive, and retires inside S4 after its references migrate.
   Rushed, real assertions get weakened rather than moved — and this subsystem's
   tests are the only thing that caught five of the six defects.
5. **The off path gains one gate instead of two.** Today the Fyne banner is gated
   and the native stickers are not, which is why OFF_STUCK is platform-divergent.
   One plan builder kills that — and means a bug in the single gate takes out
   every surface at once. Pin `notesFeatureOn=false` as its own enumeration
   column asserting an EMPTY PLAN, not merely an empty bubble.
6. **Seven of the thirty states are untouched** and must not be assumed away:
   X3 (a note evicted by its own save), WIPED, JUNK_PURGED, UNREADABLE-is-silent,
   PARKED, the live `/nkjv/` 404, and DEADTAP's swallowed tap. **The ceiling here
   is 23 of 30, not 30.**

## 7. Decisions that are the owner's, not mine

1. **The key collision.** `version|book|chapter` means at most ONE note per
   translation per chapter, and `saveNote` replaces on collision. Two people
   sending notes on John 3 *in the same translation* still overwrite each other
   silently, and the set model does not change that — it makes the reading view
   able to show many while the store can supply at most one per translation. The
   header's reasoning was sound when a chapter showed one bubble; once a chapter
   shows a set it expires. Widening the key needs a migration, is out of scope
   here, and is the next thing that will force a decision.
2. **R4's sentences — three or one?** The derive distinguishes three reasons a
   note has no home here: the book does not exist in this translation, the
   numbering does not line up, it maps into a different chapter. Three is more
   truthful and more words on screen; one is quieter and slightly wrong in two
   cases out of three.
3. **Does "which note I had open" survive a relaunch?** This design says no —
   focus is session-only, so a relaunch expands the newest never-closed note.
   Nothing is hidden (every note is a chip, one tap away) and the cap's residue
   stays at exactly zero. Persisting it needs a new preferences key, not a store
   change. A feel question, not a correctness one.
4. **PARKED notes live only in memory.** A note held in `pendingLink` because the
   translation is downloading, or the book is not in the seed, is one process
   death away from never having existed — and a note exists nowhere else.
   Storing on park would fix it, and would mean a note can be in the store for a
   passage the reader has not reached yet. Separate decision, flagged because
   this rework touches the arrival path and it would be cheap to do at the same
   time.
