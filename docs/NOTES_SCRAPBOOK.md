# The note scrapbook — architecture

> **This supersedes `NOTES_DESIGN.md`**, which assumed one note per chapter and
> no sender. Read `NOTES_STATE.md` first for what the code does today; this is
> what replaces it.

## The brief

> *"It's not just a JIT note reader — it's like a note scrapbook (that can be
> turned off of course). You may receive lots of notes over time for different or
> same translations even with overlapping highlights (you need to handle this).
> … we need to prepare for the situation where someone receives lots of notes
> from the same person, different people, the same translation, different
> translations, the same highlight, different highlights, overlapping highlights,
> different dates, etc etc etc. This should be like a complete scrapbook
> architecture for a complex scrapbook keeping program. We haven't deployed this
> so now is your chance to get it right from scratch! Don't worry about what is
> already there too much. The only migration will really be mine — and it's ok if
> that fails. It's the users that matter, and they have no notes yet."*
>
> *"And the storage and data structures should be ready to accommodate new
> features, new fields, new displays, etc etc. Think long term — very long term.
> Build the best foundation!"*

— the owner, 2026-08-15.

## The clean slate is real

Notes and note-bearing links first appeared in `d472c9a6a` / `83f1b035c` on
2026-08-09, **after** the last shipped tag `v1.1.7`. Nothing about notes is in
any released build. No user has a note. Therefore the store format, the wire
format and the key are all free, and this document does not owe compatibility to
anything.

**That freedom has an expiry date, and it is the one time-critical decision in
here.** It ends the moment a build containing the current note codec reaches the
App Store. After that, a reader on that build sees *nothing* for a link written
by any later build — a note silently lost, with no way to recover it. 1.1.8 is
built and verified but on hold, so the freedom holds; it must not be released
with notes enabled until the new codec lands.

## What was measured, and by whom

The design below came out of a ten-agent panel (three constraint analyses, three
independent architectures, three judges, one synthesis). **The following load-
bearing facts I re-ran myself against this tree and confirm:**

| Fact | Verified |
|---|---|
| Cross-chapter mapping is **24 cases, all the Romans doxology**. Joel, Malachi, 3 John and the psalm superscriptions do **not** diverge — every shipping translation uses the same English chapter division | ✅ enumerated over the real caches |
| `MapVerse(webc→web, Tobit 1:1)` returns **`exact`** — but the WEB has no Tobit. The table answers "the numbering agrees" where it has no knowledge | ✅ |
| `MapVerse(esv→bsb, …)` returns **`exact`** — an unknown translation id silently means "assume identical versification" | ✅ |
| `MapVerse(web→bsb, Mark 9:43/44/45)` = exact / **absent** / exact — a span with a **hole** in it, which `VerseLo`/`VerseHi` cannot express | ✅ |
| The app's fragment parser read only the first field while the published web reader reads all of them | ✅ — fixed, `share_link_parse.go` |
| A cross-chapter note is **relocated correctly**, not dropped; the defect is that no verb can reach it | ✅ — X13, `notes_crosschapter_test.go` |
| There are **four** note-drawing surfaces, not five; both HTML builders draw only the highlight and reserve the band | ✅ |

**Taken on the panel's measurement, not re-run by me** (each is reported with a
probe and a method, and the three judges checked them, but I note the distinction
honestly): the AppKit importer collapsing nested backgrounds to one background
colour per character; `text-decoration-color` being dropped while `underline`
survives; the styled pane's full-width band spilling onto 41% of lines that carry
two verses; the palette contrast figures for the proposed second and third tints.
**The palette numbers in particular must be judged on a rendered page in both
themes before shipping** — `theme.go:128-130` records that this palette has only
ever been decided that way.

## The shape, in one sentence

**A note is a document with its own identity and its own anchor; the anchor is
resolved into whatever translation is on screen and answers with a SET of runs
plus a REASON; how many notes may be open at once is a rule the render applies
and never writes down; and every mark carries who put it there.**

Three things the current model cannot say, each of which is a measured reality
rather than a hypothetical:

1. **A resolution is a set, not a span.** Mark 9:43-46 lands in the BSB as
   `[43,43]` and `[45,45]`. Romans 14:24-26 lands in two chapters at once.
2. **A resolution has a reason, and the reason is consulted before the numbers.**
   Eight arms, because "the book is not in this translation", "the numbering does
   not correspond", and "it is here under other numbers" are different sentences
   — and because `MapVerse` cannot distinguish the first from "the numbering
   agrees".
3. **A note has a sender.** There is no sender anywhere in the model today, so
   *"lots of notes from the same person"* is unanswerable as designed.

## Staging

~30 working days, ~8 of them cgo. Every step leaves the tree green and shippable.

**S0–S2 are fixes to shipped code paths and are worth landing whatever happens to
the rest of this design.** S0 is done.

| Step | What | Days | Status |
|---|---|---|---|
| **S0** | Fragment parser: order-independent key list, matching the web reader | 0.5 | ✅ landed |
| **S1** | Styled pane: full-width band → per-line, X-bounded rects. Removes the 41% spill | 2 | |
| **S2** | `restoreTint(verse)` on the Apple panes — cgo, isolate. Also turns a focus change into a range mutation instead of a 20–36 ms re-import | 2 | |
| **S3** | `chapterTint` + all five emitters, **k=1 only** — byte-identical output, pinned by goldens | 3 | |
| **S4** | The record codec + `normalizeSenderName`. **This is the 1.1.8 deadline step** | 2 | |
| **S5** | The store: line-framed, quarantined, counter-keyed. `noteKey`, `notesMax`, `loadNote`, `noteStoreVersion` all deleted | 3 | |
| **S6** | `notes_anchor.go`: `newAnchor`, `resolveAnchor`, all eight arms, three sentences | 2.5 | |
| **S7** | `buildChapterPlan`, one gate, set fingerprint. The `ActiveNote` quadruple leaves `AppState` | 2 | |
| **S8** | Surfaces consume the plan. **Riskiest** — de-risked by folding the count into the existing sticker ABI first | 4 | |
| **S9** | Sender identity + the bidi-isolated byline on all four surfaces | 2 | |
| **S10** | The people layer: alias, link/split, mute. Independently droppable | 3 | |
| **S11** | Scrapbook: windowed list, four axes | 3 | |
| **S12** | Palette + contrast — **owner judgement on a rendered page** | 0.5 | |

**Discarded, not migrated:** `SharedNote`, `noteKey`, `notesMax`, the passage-keyed
store, `noteFromAnotherTranslation`, `noteStoreVersion`, the `ActiveNote` /
`NoteMinimized` / `NoteVerseLo` / `NoteVersionID` quadruple, and the owner's own
existing notes (accepted, per the brief).

## Long-term foundation — the requirement that cannot be retrofitted

The owner's last instruction is the one with no second chance, so it is called out
separately from the design body:

1. **Unknown fields must survive a read-modify-write.** Go's `json.Unmarshal`
   drops what it does not know, so an older build that rewrites the store
   destroys fields a newer one wrote. Every record keeps its unparsed remainder.
2. **Per-record framing, not one big array.** One corrupt record must cost one
   record, not the scrapbook. Unparseable lines are **quarantined verbatim** —
   the app never deletes what it cannot parse. (Today's read filter drops junk and
   the next write makes that permanent.)
3. **Stable ids that are never reused**, so later features — threading, replies,
   tags, favourites, archive — have something to point at.
4. **The anchor is already a set**, so multi-passage notes and finer-than-verse
   granularity are additions rather than rewrites.
5. **One annotation model, not two.** `annotation.go:28` has carried an unwired
   `Annotation` type — *"the data foundation for upcoming annotation/research
   features"* — with `Note`, `Color`, `Created`, `Updated`, in parallel with
   `SharedNote`, for months. A received note is **one kind** of annotation
   alongside your own notes, highlights with no text, and bookmarks. Unifying
   these is the difference between a scrapbook and an inbox, and it is why the
   record carries a kind from day one even though only one kind exists at first.
6. **The wire format is append-only extensible**, and skips keys it does not know
   rather than failing. Decode paths for retired keys are kept forever; only the
   emit paths retire.

---

The full synthesis — types, wire grammar, store shape, the overlap rendering
model per surface, sender trust and privacy rules, and the hard-cases table —
follows below as produced by the panel, with the corrections above applied.


######################################################################
## DECISION
######################################################################
## BASE: The Anchor Model. GRAFTS: Correspondence's people layer + palette method; Mailbox's document discipline.

### Why Anchor is the base

The owner's brief names one thing the other two designs cannot express: *"sometimes the mapping will be unusual and not straightforward."* Only Anchor's `resolve(anchor, translation) -> SET of runs + reason` can say the true answer for the cases that actually occur. I verified all of them myself:

```
MapVerse(webc->web, Tobit 1:1)     = 1:1  exact          <- THE TABLE LIES. web has no Tobit.
MapVerse(webc->web, Wisdom 3:1)    = 3:1  exact          <- same
MapVerse(esv->bsb,  Rom 16:25)     = 16:25 exact         <- unknown id => "numbering agrees"
MapVerse(webc->web, Esther 1:1)    = 0:0  incommensurable
MapVerse(webc->web, Daniel 13:1)   = 0:0  absent
MapVerse(web->bsb,  Rom 14:24)     = 16:25 moved         <- ONE span, TWO chapters
MapVerse(web->bsb,  Mark 9:44)     = 0:0  absent         <- a HOLE inside a span
```

Correspondence and Mailbox both keep today's `VerseLo`-probe-then-degrade behaviour, so on WEB Mark 9:43-46 read in BSB they lose the whole note, and on WEB Romans 14:23-24 they show half of it. Anchor is the only design whose model has somewhere to put `[43,43],[45,45]` and `Here + Elsewhere`.

Judge B's objection to Anchor is real but is a **staging** cost, not a model flaw: 8 placement arms × 4 surfaces of copy, two of them behind cgo. I resolve it by keeping all 8 arms in the model and collapsing surface copy to **three sentences** (see `overlap`). The model stays total; the UI stays small.

### Grafted IN

| From | What | Why it beat the base |
|---|---|---|
| **Anchor** (base) | set-valued `resolve`, `placedNoBook` as its own arm, `refClaim` tri-state on the wire, `Anchor.Version` always carried | verified above; the path is lossy (`ShareLinkURLWithNote` forces `webc` for deuterocanon, falls back to `web` for unknown ids) |
| **Correspondence** | the `Person` entity — reader-owned alias, link/split, mute, per-note `ClaimedName` with history, `Mine` | the only answer to *"lots of notes from the same person"*. A sender who reinstalls or uses two devices becomes two strangers in Anchor and Mailbox, and neither names it |
| **Correspondence** | tint stated as **distance from the paper**, with measured hex for both themes | dissolves the light/dark inversion the other two keep restating; and it is the only design that checked its arithmetic against `theme.go`'s own hand-judged figure |
| **Correspondence** | at-most-one-expanded retires the Apple multi-band risk | `docs/NOTES_DESIGN.md` risk 1 is stacking bands at multiple anchors behind cgo. One focused note ⇒ one band at one anchor, exactly as today. Nobody else spotted this |
| **Mailbox** | the arrival constructor may not read `AppState` | verified live: `rememberIncomingNote` writes `Chapter: state.CurrentChapter` — the reader's position, not the link's. It is only ever right because `applyShareTarget` happens to navigate first |
| **Mailbox** | "fail **open** toward showing, fail **closed** toward writing", with a `default:` arm that shows the note | both halves have been broken in shipped code, in opposite directions |
| **Mailbox** | quarantine unparseable lines verbatim; nothing the app cannot parse is deleted by the app | today's read filter drops junk (`notes_store.go:100-104`) and `writeNotes` makes it permanent |
| **All three** | delete `noteKey`, delete `notesMax`, line-frame the blob, preserve `readNotesChecked`'s `ok=false` stand-down, group by opaque id never by name, quote the claimed name | correct in all three; not re-litigated |

### Rejected, with reasons

**Mailbox's content-digest `NoteID`.** `base32(SHA-256(seed ‖ passage ‖ canonical record stream))` makes the store's primary key a function of the wire encoder's byte layout. Three consequences: the store cannot land before the codec (one coupled commit on the subsystem that produced six defects in two days); any later fix to `normalizeSenderName` or the canonical ordering silently forks dedup, so a re-opened link mints a *second* copy of a note the reader already has; and a legacy `p`/`z` payload **has no record stream**, so its ID is undefined — Mailbox never says. Replaced with a locally-minted `NoteID` and a `(OriginID, contentHash)` dedup test, which reaches the same idempotence without the coupling.

**Mailbox's tint model** (focused note only; depth 2 solely at intersections). Three notes on John 3 with nothing focused draws *no tint at all*; two unfocused notes are invisible in the text. The owner wrote "overlapping highlights (you need to handle this)". Rejected.

**Mailbox's three preference keys** (log / flags / seal). Three write paths and three tear modes, and the durability asymmetry it is built on does not exist: Fyne re-encodes the *entire* `preferences.json` on any `Set`, so "the log is only appended to" is not true at the storage layer. One blob, one sibling counter.

**Mailbox pooling anonymous notes into one group.** Three strangers drawn as one correspondent is a quiet misattribution in a system whose entire purpose is attribution. Each anonymous note is its own group.

**Correspondence's dark `tintQuiet = #3A2B0C`.** `theme.go:114-117` records that exact colour being rejected: ΔE 22.9 where the light band is 44.7, "easy to lose while scanning". My own measurement puts it at 20.5. Replaced with `#48370F` (ΔE 27.2 — 84% of the shipped dark band, and its red-letter contrast *improves* from 3.39 to 3.84).

**Correspondence's `byPassage` multi-key insert precomputed against "the four known translations."** Verified brittle: an unknown sender id maps `exact`, so a future-translation doxology note is filed only under its own chapter and is unreachable from the other. Replaced with Anchor's reference-space index, which is translation-independent by construction.

**Correspondence's 4-arm `placementKind`.** No book-missing arm, so a WEBC Tobit note read in the WEB is classified `placedByMapping` at a chapter that does not exist.

**Correspondence's `storeLost` monotone high-water mark that refuses every write forever.** Two separate `SetString`s with no specified ordering; one crash between them and the store is permanently read-only and every arriving note is discarded. Replaced with Anchor's narrow trigger (empty blob + count > 0), which cannot false-positive into a brick.

**Correspondence's claim that DEADTAP dies with a per-verse tap table.** Verifiably the wrong mechanism — DEADTAP is `openNote` at `notes_browse.go:250-257`, a *browser-row* tap gated on `GetChaptersForBook(n.Book) == 0`. Fixed properly, in the scrapbook, by `placedNoBook` + a "Read in WEBC" action.

**All three designs' loss tripwire.** Every one of them puts the seal/count/hwm inside the same `preferences.json` whose truncation it is meant to detect — the truncation takes the tripwire with it. Judge C caught this and no design fixed it. Honest resolution below: the counter detects a **value-level** wipe (empty string, bad encode, hand edit), *not* whole-file truncation, and shipped builds already apply `patches/fyne-2.7.4-atomic-prefs.patch` so file-level truncation cannot recur in a shipped build. Say that in the comment rather than overclaiming.

**Sub-verse spans.** Nothing in store, wire, `VerseSpan` or any of five emitters can express one, and `share.go:975-986` already rounds a selection to whole verses. Not added; explicitly documented as out of scope so the document model does not look capable of something the stack is not.

######################################################################
## TYPES
######################################################################
```go
// ============================================================================
// notes_anchor.go — WHERE A NOTE POINTS
//
// A verse number is not an address. Verified against this tree's own tables:
//
//   MapVerse(web->bsb,  Romans 14:24) = 16:25 moved     one span, two chapters
//   MapVerse(web->bsb,  Mark   9:44)  = 0:0   absent    a hole INSIDE a span
//   MapVerse(webc->web, Esther 1:1)   = 0:0   incommensurable
//   MapVerse(webc->web, Tobit  1:1)   = 1:1   EXACT     <- the table LIES
//   MapVerse(esv->bsb,  Romans 16:25) = 16:25 EXACT     <- unknown id, silent
//
// The last two are why this file exists. versificationDeltas has no entry for
// "web" at all, so nothing stops a WEBC Tobit note claiming an exact landing in
// a translation that does not contain the book; and toReference for an unknown
// id returns "assume the numbering agrees", which is the right default and
// silently wrong for a translation added after this build shipped.
//
// So resolution returns a SET plus a REASON, never a span, and the reason is
// consulted before the numbers.
// ============================================================================

// AnchorRun is a contiguous verse run inside ONE chapter, in ONE numbering.
//
// The Chapter is on the run, not on the anchor, because a resolution can land
// in two chapters at once (the Romans doxology, every day). Without it the
// second half has nowhere to go and the note degrades to the verse it starts
// at — which is exactly what noteFromAnotherTranslation does today.
type AnchorRun struct {
	Chapter int `json:"c"`
	// Lo == 0 means CHAPTER-LEVEL: a note about the chapter, not about verses in
	// it. A real state (share.go emits chapter links) and a sentinel is cheaper
	// than a second type. Without it a chapter note is indistinguishable from a
	// note on verse 1 and would tint one verse of a chapter it is about whole.
	Lo int `json:"lo,omitempty"`
	// Hi == 0 or <= Lo means a single verse. Same reason the shipped VerseSpan
	// does it: the common note is one verse.
	Hi int `json:"hi,omitempty"`
}

// refClaim is what the SENDER said about this passage's place in the reference
// (WEB) numbering. Three values, and the third is the one that matters.
//
// WITHOUT THE THIRD VALUE a reader cannot tell "the sender said nothing, work it
// out" from "the sender said this passage has NO reference expression". A reader
// whose build predates the sender's translation resolves the second case by
// assuming WEB numbering — which for Greek Esther points a note confidently at
// an unrelated passage. That is the one failure this whole file exists to stop.
type refClaim uint8

const (
	// refUnclaimed — no anchor record. Asserts the sender's own table found the
	// projection to be the identity. We project with ours.
	refUnclaimed refClaim = iota
	// refNone — the sender asserts this passage has no reference expression.
	// Readable ONLY in translations sharing the sender's numbering for this book.
	refNone
	// refRuns — the sender's projection, which WINS over ours. The sender is the
	// only party guaranteed to hold the delta for its own translation.
	refRuns
)

// Anchor is where a note points, in two frames at once. Built ONCE by
// newAnchor and never written again.
type Anchor struct {
	// Book is the one component of a reference that is already translation-
	// independent: every decoder normalises to these names (catholic.go),
	// BookSlug/BookFromSlug round-trip through them, and the deltas are keyed by
	// them. A book is never renumbered; only chapters and verses are.
	Book string `json:"b"`

	// Version is the SENDER's translation id — ALWAYS stored, NEVER inferred
	// from the URL path.
	//
	// The path is LOSSY BY CONSTRUCTION: ShareLinkURLWithNote (share_link.go:
	// 128-130) forces "webc" for any book outside protestantCanonBooks whatever
	// the sender was reading, and any id outside linkPathVersionIDs falls back to
	// "web". Reconstructing this from context is identity-rebuilt-from-context,
	// the exact shape that produced X1 and X5. Measured cost: 5 encoded chars.
	Version string `json:"v"`

	// Run is the span in Version's own numbering — what the sender selected.
	// SINGLE, not a slice: share.go:975-986 already rounds a selection to whole
	// verses within one chapter, so a slice here is a shape nothing can produce.
	Run AnchorRun `json:"r"`

	// Claim/Ref carry the sender's reference projection. Ref is meaningful only
	// when Claim == refRuns, and may hold several runs because a projection
	// genuinely splits.
	Claim refClaim    `json:"rc,omitempty"`
	Ref   []AnchorRun `json:"rf,omitempty"`
}

// PlacementKind is TOTAL over every way an anchor can meet a translation. The
// derive has NO `continue` that loses a note; its only skip is "belongs to
// another chapter", which is not a loss because the note appears there.
//
// EIGHT ARMS IN THE MODEL, THREE SENTENCES ON SCREEN (see placementCopy). The
// arms exist so the code is honest; the copy is small so four surfaces — two of
// them behind cgo — stay buildable.
type PlacementKind uint8

const (
	// placedNative — the reading translation IS the note's own. No mapping ran,
	// so the note comes home byte-exact even if the delta tables are wrong for
	// its translation. Distinct from placedExact because the chrome must NOT
	// print a translation label here.
	placedNative PlacementKind = iota
	// placedExact — mapped, every verse kept its number.
	placedExact
	// placedMoved — present here under different numbers, and/or partly in
	// another chapter of this book (see Placement.Elsewhere).
	placedMoved
	// placedPartial — some verses are here and some are not in this translation
	// at all (a span crossing one of the BSB's twelve omissions, e.g. WEB
	// Mark 9:43-46 in the BSB = [43,43] and [45,45], verified). Its own arm
	// because a partial tint that looked complete would misrepresent the sender.
	placedPartial
	// placedOtherChapter — the whole span maps into a DIFFERENT chapter of this
	// book. Nothing tinted here; the reader is told where it went and is one tap
	// from it. Without this arm the note is simply gone from the chapter the
	// reader thinks it is on.
	placedOtherChapter
	// unplacedAbsent — the book is here, none of the note's verses are.
	unplacedAbsent
	// unplacedIncommensurable — the book is here and does not correspond verse
	// by verse (WEBC's Greek Esther).
	unplacedIncommensurable
	// unplacedNoBook — this translation does not contain the book at all.
	//
	// NOT DERIVABLE FROM MapVerse. VERIFIED: versificationDeltas has no "web"
	// entry, and webc's incommensurable map lists only Esther, so
	// MapVerse("webc","web","Tobit",1,1) answers 1:1 EXACT. This arm exists
	// because the mapping tables have a hole and the anchor must not.
	unplacedNoBook
)

// placed reports whether this kind puts anything on the page here.
func (k PlacementKind) placed() bool { return k <= placedPartial }

// Placement is the answer to "where does this anchor land, here?".
type Placement struct {
	// Kind names the WORST thing that happened, ranked exactly as
	// ChapterNumberingDifference already ranks its answers (versification.go):
	// a reader told only about the milder of two problems acts on the wrong one.
	Kind PlacementKind
	// Here is what to tint, in the READING translation's numbering, disjoint and
	// ascending. Empty for every unplaced kind and for placedOtherChapter.
	Here []AnchorRun
	// Elsewhere is the rest of this anchor, in other chapters of this book.
	Elsewhere []AnchorRun
	// Why is the reason string for the unplaced arms, from IncommensurableBook
	// or composed here. Empty when placed.
	Why string
}

// ============================================================================
// notes_model.go — WHAT A NOTE IS, AND WHO SENT IT
// ============================================================================

// NoteID is a note's identity in THIS reader's scrapbook, and the ONLY thing a
// verb ever addresses. Minted LOCALLY on arrival:
//
//	"n" + <base36 unix seconds> + "-" + <8 base36 chars of crypto/rand>
//
// Leading with the timestamp makes ID order arrival order, which gives the
// serialiser a byte-stable sort with no map range — load-bearing, because Fyne's
// set() short-circuits only on byte equality and an unstable order would rewrite
// preferences.json on every chapter turn.
//
// WITHOUT IT the passage is the key. Measured on a realistic distribution,
// version|book|chapter silently overwrites 24% of a 200-note store and 72% of a
// 5,000-note one, and every verb has to rebuild a key from where the reader is
// standing — X1, X2, X5, X13, four named defects with one cause.
//
// DELIBERATELY NOT A CONTENT DIGEST. A digest over the wire's canonical record
// stream couples the store's primary key to the encoder's byte layout: the store
// could not land before the codec, a later fix to name normalisation would fork
// dedup silently, and a legacy 'p'/'z' payload has no record stream to hash.
// Dedup is a separate, explicit test (see dedupKey) and does that job better.
type NoteID string

// SenderID is the opaque handle a sending device stamps into its links: six
// bytes of crypto/rand, lowercase hex. "" means the link carried none.
//
// IT AUTHENTICATES NOBODY. No server, no key, no signature, and there cannot be
// one in this architecture. Anyone can lift an id out of a link they received
// and forge links that group with it. Grouping is a CLAIM of common origin,
// never proof. No surface may print "verified" or state a name as fact.
type SenderID string

// PersonID is the READER's grouping unit, and it is why this is not SenderID
// renamed. One human can hold two devices or reset their id, and the reader is
// the only party who can know that.
//
// WITHOUT IT a correspondent who reinstalls silently becomes a second stranger
// and "what did my mother send me" under-answers with nothing on screen saying
// so — a wrong answer that looks complete.
type PersonID string

// StoredNote is one message from one person about one passage. IMMUTABLE after
// arrival except the three reader-owned facts at the bottom.
//
// NOTHING IN THIS TYPE IS EVER BUILT FROM AppState. Verified live:
// rememberIncomingNote files `Chapter: state.CurrentChapter` — the chapter the
// READER is standing on, not the one the link names — and is only ever right
// because applyShareTarget happens to navigate first. A record that can see
// where the reader is standing will eventually record where the reader is
// standing. newStoredNote takes a ShareTarget and a clock, and nothing else.
type StoredNote struct {
	ID NoteID `json:"id"`

	// Seq is a per-device monotonic arrival counter from the store header. Not a
	// timestamp, never ties. It is what gives two notes arriving in the same
	// second a total order, so the scrapbook does not reshuffle between two
	// renders of unchanged data.
	Seq int64 `json:"q"`

	// OriginID is the note nonce the sender's link carried (wire tag 'k'), hex,
	// or "" for a legacy or hand-written link. DEDUP ONLY — never a key.
	//
	// It exists for exactly one case: two ANONYMOUS senders writing the identical
	// sentence on the identical verse in the same minute. Without it they hash
	// identically and one person's message is destroyed — the one invariant with
	// no recovery. That is the whole justification for ~11 URL characters.
	OriginID string `json:"o,omitempty"`

	// Anchor — ONE value, not five loose fields. Five undiscriminated fields with
	// no frame is precisely the shape mark.go has just finished deleting from
	// AppState, for the reason in its header (ORPHAN_HL, X9, X10, GHOST_LOC), and
	// it cannot express a projection at all.
	Anchor Anchor `json:"a"`

	// Text is the message. UNTRUSTED: rendered as TEXT on every surface, never as
	// markup, always attributed to a person, never in the app's own voice. It has
	// passed normalizeNote and that is ALL it guarantees.
	Text string `json:"t"`

	// Sender is the grouping axis. "" is an honest, drawable state ("someone"),
	// and every "" note is its OWN group — never folded in with any other.
	Sender SenderID `json:"sx,omitempty"`

	// ClaimedName is what THIS note's link said the sender was called — not the
	// person's current name. UNTRUSTED, attacker-chosen, normalizeSenderName'd,
	// <= 24 runes, single line.
	//
	// Frozen per note, like an email's From: header. WITHOUT the per-note freeze a
	// rename rewrites history for every note that sender ever sent, so both a
	// legitimate rename and an impersonation attempt become invisible.
	ClaimedName string `json:"cn,omitempty"`

	// Sent is the sender's clock (unix seconds, from the wire's minute stamp), or
	// 0. UNTRUSTED — skewed, forged, or a factory-reset device claiming 1970.
	// DISPLAY ONLY, and only when sane: inside [feature epoch, Received + 24h].
	// It must NEVER influence order, or a forged future timestamp pins a
	// stranger's note to the top of the reader's scrapbook forever.
	Sent int64 `json:"st,omitempty"`

	// Received is THIS device's clock at arrival. The only clock allowed to order.
	Received int64 `json:"ts"`

	// Reseen is the last time this EXACT note arrived again (a re-opened or
	// re-forwarded link), or 0. The most droppable field here.
	Reseen int64 `json:"rs,omitempty"`

	// --- the reader's own three facts. The only mutable fields. ---

	// FirstSeen is when the reader actually had it open; 0 = never. The only
	// fact an unread badge can honestly be built on.
	FirstSeen int64 `json:"fs,omitempty"`

	// Minimized: the reader collapsed THIS note. ONE meaning, written only by a
	// reader's press. The view's at-most-one-expanded cap is maintained by the
	// VIEW and writes nothing — a cap-by-action minimize would be byte-identical
	// to a genuine one and no migration could ever tell them apart.
	Minimized bool `json:"m,omitempty"`

	// Mine marks a note the READER composed and sent, stored at compose time.
	// WITHOUT IT the sender's own words vanish when the compose sheet closes —
	// the confusion the NKJV report describes.
	Mine bool `json:"me,omitempty"`
}

// dedupKey is the identity test for "have I already got this?". BOTH halves,
// compared together.
//
//	(OriginID, sha256(version|book|chapter|lo|hi|text|claimedName|senderID|sent))
//
// NOT in it: Received, FirstSeen, Minimized, Reseen, which surface it arrived
// on, who forwarded it.
//
// OriginID alone would let a buggy or malicious sender reuse one nonce with
// different text and destroy the second note — the cardinal sin. The hash alone
// merges two genuinely different messages in the anonymous corner. Together,
// identical only when both agree.
//
// ACCEPTED LOSS, NAMED: a legacy 'p'/'z' link carries no OriginID, so two
// anonymous byte-identical notes on one passage dedup as one. Bounded to
// pre-v2 links; the price of decoding them at all.
type dedupKey struct {
	Origin string
	Hash   [32]byte
}

// Person is what the READER owns about a correspondent. Every field was typed or
// pressed by the reader; nothing is derived and nothing is required.
//
// NOT LOAD-BEARING, BY CONSTRUCTION: a note names a SenderID, never a PersonID.
// Lose every Person record and no note is lost — labels fall back to the newest
// ClaimedName, then to "Someone". That is what makes it safe to keep people in
// the same blob as notes.
type Person struct {
	ID PersonID `json:"id"` // local, "p"+base36 counter. Never on the wire.

	// Senders is the set of sender ids the reader considers one human. Normally
	// one; two when they pressed "Same person as...". REVERSIBLE, because a wrong
	// merge attributes one person's words to another.
	Senders []SenderID `json:"sx"`

	// Alias is the READER's name for them — the one string in the system a person
	// typed themselves. Beats every ClaimedName.
	Alias string `json:"al,omitempty"`

	// Muted: stored and browsable, never drawn in the reading pane, never counted
	// unread. WITHOUT IT there is no defence against unwanted notes short of
	// deleting somebody's messages — the app cannot stop links reaching it.
	Muted  bool `json:"mu,omitempty"`
	Pinned bool `json:"pn,omitempty"`
}

// storeHeader is line 1 of the blob.
type storeHeader struct {
	Kind    string `json:"k"`   // "h"
	Schema  int    `json:"s"`   // 2. A HIGHER schema is read best-effort and NEVER written back.
	NextSeq int64  `json:"seq"`
	Count   int    `json:"n"`
	Written int64  `json:"w"`
}

// ============================================================================
// notes_index.go — THE QUERYABLE VIEW. Built at load, NEVER persisted.
//
// A persisted index is a second copy of the truth that goes stale on precisely
// the events this subsystem already gets wrong: arrival, delete, version switch.
// ============================================================================

type refKey struct {
	Book    string
	Chapter int // in the REFERENCE (WEB) numbering
}

type notesIndex struct {
	notes []StoredNote // Seq ascending == arrival order == file order. No sort.
	byID  map[NoteID]int

	// ref[i] is note i's SETTLED projection into the reference numbering, or nil
	// meaning "this anchor has no reference expression". Settled at LOAD, not at
	// arrival, so a later versification fix reaches notes already received.
	ref [][]AnchorRun

	// byRef is THE index, keyed in REFERENCE space. THIS IS THE PAYOFF: it is
	// translation-independent, so a version switch invalidates nothing and the
	// reading pane and the scrapbook's "notes on this passage" ask one question.
	//
	// NOT keyed per translation, and NOT precomputed against "the known
	// translations" — verified that an unknown/future id maps EXACT, so a
	// precomputed multi-key insert files a future doxology note under one chapter
	// only and it is unreachable from the other.
	byRef map[refKey][]int

	// byNative holds the notes whose ref is nil — anchors with no reference
	// expression, which can only surface in translations sharing their frame.
	// Without a separate bucket they are unreachable from byRef and invisible.
	byNative map[string][]int // "<version>|<Book>|<chapter>"

	// refChapters[vid][refKey] is every reference chapter that can land in this
	// translation's chapter. Built from the deltas' `moved` lists — 3 entries for
	// bsb, 3 for nkjv, 7 for webc (counted). One map lookup on the hot path.
	refChapters map[string]map[refKey][]int

	people   []Person
	personOf map[SenderID]PersonID
	view     map[PersonID]*personView
	bySender map[SenderID][]int
	byDay    map[int32][]int

	dedup map[dedupKey]int

	// quarantine holds lines that were present and did not parse, VERBATIM. They
	// are rewritten untouched, at the tail, after a "#" marker. NOTHING THE APP
	// CANNOT PARSE IS EVER DELETED BY THE APP — today's read filter drops junk and
	// the next unrelated write makes that permanent (a 3-entry blob became 1 after
	// an unrelated minimize).
	quarantine []string

	status  storeStatus
	damaged int // quarantined this load; > 0 is TOLD to the reader, once
	unread  int
}

type storeStatus int

const (
	storeOK         storeStatus = iota
	storeDamaged                // some lines bad, the rest good. SAFE TO WRITE.
	storeWiped                  // blob empty but the sibling count says otherwise
	storeUnreadable             // no header, or nothing parsed
	storeFuture                 // schema newer than this build
)

// writable is readNotesChecked's ok=false contract, preserved verbatim and for
// the same reason: every mutation is a read-modify-write, so a failed read that
// answered "no notes" would serialise emptiness over the reader's collection and
// their NEXT action would be the thing that destroyed it.
func (s storeStatus) writable() bool { return s == storeOK || s == storeDamaged }

// personView is everything about a person that is DERIVED. Rebuilt with the
// index, never stored, so it cannot disagree with the notes.
type personView struct {
	ID            PersonID
	Label         string
	LabelSource   labelSource
	ClaimedNames  []nameClaim // every distinct name this id has claimed, newest first
	Linked        bool        // len(Senders) > 1 — the UI must say "you linked these"
	Muted, Pinned bool
	First, Last   int64
	Count, Unread int
}

type nameClaim struct {
	Name string
	Last int64
}

// labelSource travels WITH the label into every surface, so no surface can
// render an attacker's string in the same voice as the reader's own word.
type labelSource int

const (
	labelNone    labelSource = iota // "someone" — the app's own quiet voice
	labelClaimed                    // QUOTED:  Note from “Anna”
	labelAlias                      // unquoted: Note from Mum  (the reader typed it)
)

// ============================================================================
// notes_plan.go — THE ONLY THING ANY READING SURFACE READS
// ============================================================================

// ChapterNote is one note as the panes need it. Built ONLY by buildChapterPlan;
// every field computed at construction and never written again.
type ChapterNote struct {
	ID   NoteID
	Text string // UNTRUSTED
	By   byline

	// Cite is the SENDER's citation in the SENDER's frame — "Romans 16:25 (BSB)".
	// NOT the resolved one. A note is a remark about particular wording, and
	// rewriting the reference the sender wrote is putting words in their mouth.
	Cite string

	// Place is the resolution into the translation on screen. The panes never
	// touch Anchor.Version/Chapter/Lo — one ruler, enforced by there being only
	// one span the panes can reach.
	Place Placement

	// Label is the sender's translation display name, "" when placedNative. Its
	// presence is how NOTE_OWN and NOTE_FOLLOWED stop being indistinguishable.
	Label     string
	Received  int64
	Sent      int64 // 0 unless sane
	Unread    bool
	Minimized bool
	Mine      bool
}

// byline is the ONE way a person reaches a surface: label AND provenance,
// together, so the quoting rule cannot be forgotten one surface at a time.
type byline struct {
	Label  string
	Source labelSource
	Person PersonID
}

// noteTint is what a character's background says. FOUR constants, and at most
// TWO are ever live at once — the measured colour budget, not a taste call.
type noteTint uint8

const (
	tintNone noteTint = iota
	// tintNote IS the shipped palette Highlight. A chapter with exactly one shown
	// note therefore renders BYTE-IDENTICALLY to today, which is what lets the
	// flattening layer land first as a pure refactor.
	tintNote
	// tintOther — one step TOWARD the paper. Live only when a note is focused:
	// "another note is here too". Light #FFE9A8, dark #48370F.
	tintOther
	// tintMulti — one step AWAY from the paper. Live only when NOTHING is
	// focused: "more than one note covers this verse". Light #F5C24A, dark
	// #5C4412. tintOther and tintMulti are never co-present.
	tintMulti
)

// tintRun is a maximal run of verses at one tint, disjoint and ascending. At
// most 2k-1 runs for k notes. Computed ONCE in Go and handed identically to all
// five emitters, because four of the five physically cannot layer a second
// background: on the AppKit/UIKit importer a nested span REPLACES its parent
// (one NSBackgroundColorAttributeName value per character), Html.fromHtml agrees,
// and the styled pane draws one rectangle.
type tintRun struct {
	Lo, Hi int
	Tint   noteTint
}

// drawnNote is the RENDER PROJECTION. Built once per render, dead with the
// frame. `Open` lives HERE and nowhere else — a bool on ChapterNote would be a
// derived mirror with one writer and four readers, which is the quadruple's
// failure shape at quarter scale.
type drawnNote struct {
	Note ChapterNote
	Open bool
	// Sentence is the placement copy, from placementCopy. EIGHT model arms
	// collapse to THREE sentences here, which is what keeps four surfaces (two
	// behind cgo) buildable.
	Sentence string
}

// chapterPlan is the whole answer to "what does this chapter show". The ZERO
// plan is "notes are off, or there are none here", so the off switch is one
// assignment and every surface goes quiet through one gate.
type chapterPlan struct {
	Notes []drawnNote // stable order; NO arm is dropped
	Focus NoteID      // "" = nothing expanded. SESSION ONLY, zero bytes of residue.
	Tints []tintRun
	// Elsewhere counts notes on this passage that cannot be drawn in the
	// translation being read. Drawn as a sentence, never as a silent skip.
	Elsewhere int
	// Fingerprint folds the WHOLE plan. Computed HERE, inside the derive, so a
	// new field is folded in by the same function that added it. reading.go:
	// 487-492 already records what a scalar costs: "Delete note" did nothing
	// visible. A set model with today's scalar reintroduces that, five times.
	Fingerprint uint64
}

// ============================================================================
// mark.go — ONE addition. Everything else in that file is right and stays.
// ============================================================================

// type Mark struct {
//     Origin hlOrigin
//     At     VerseSpan
//     Note   NoteID // meaningful only when Origin == hlNote
// }
//
// WITHOUT Mark.Note, "which note owns this highlight" goes back to being
// answered by coincidence once a chapter holds a set — X10 by another road.
```

######################################################################
## WIRE
######################################################################
## Grammar

Keep the fragment key list. Keep the key name `n=`. There is never an `n2=` — extensibility lives **inside** the blob.

```
https://bibletext.co.uk/<version>/<book-slug>/<chapter>/#v<lo>[-<hi>][&n=<base64url(blob)>]

blob:
  byte 0   format tag
             'p'  LEGACY bare text, plain      DECODE-ONLY, never emitted, kept FOREVER
             'z'  LEGACY bare text, DEFLATE    DECODE-ONLY, never emitted, kept FOREVER
             'r'  records, raw
             'd'  records, raw-DEFLATE (emitted only when it comes out smaller)
  bytes 1..  a record stream

record:  <tag: 1 byte> <len: uvarint> <value: len bytes>
```

| tag | field | value | required |
|---|---|---|---|
| `a` | anchor: reference runs | `uvarint n`, then n × (`uvarint chapter`, `uvarint lo`, `uvarint hi`) | no |
| `f` | sender display name | UTF-8, ≤24 runes, single line | no |
| `i` | sender id | exactly 6 bytes | no |
| `k` | note nonce | exactly 6 bytes | no, but always emitted |
| `s` | sent time | uvarint, **minutes** since 2020-01-01Z | no |
| `t` | **note text** | UTF-8, ≤280 runes | **yes** |
| `v` | sender's translation id | ASCII lowercase, ≤8 bytes | **yes** |

**Canonical form: records ascending by tag, at most one of each.** A non-canonical stream is rejected outright.

**Minutes, not seconds, for `s`:** the same 4 varint bytes until 2276, and a second-precision stamp is a finer fingerprint of when someone was reading scripture for no display benefit.

**`v` is required, not conditional.** I depart from the "only when it differs from the path" recommendation. Verified: `ShareLinkURLWithNote` (`share_link.go:128-130`) forces `webc` for any book outside `protestantCanonBooks` whatever the sender was reading, and any id outside `linkPathVersionIDs` falls back to `web`. Reconstructing the sender's translation from the path is identity rebuilt from context. Measured cost: **5 characters**.

## The `a` record, and why the sender computes it

Emitted under exactly one rule:

- **absent** — the sender's own `toReference` found the projection to be the identity (≈31,000 of ≈31,100 verses). The reader projects with its own table.
- **present, n = 0** — the sender asserts this passage has **no** reference expression (WEBC Greek Esther). The reader must never project it.
- **present, n > 0** — the sender's projection, which **wins** over the reader's.

Verified why this must be the *sender*: `toReference` for an unknown id returns "assume the numbering agrees with the WEB" (`versification.go:105-109`) — the right default, and silently wrong for a translation added after this build. I confirmed `MapVerse("esv","bsb","Romans",16,25)` = `16:25 exact`. Without `a`, a note written in a later translation lands on the wrong verse, confidently, with no signal.

**Measured cost: +8 URL characters, on roughly 130 verses of the whole Bible.**

## Fail closed

No note at all — never a partial one — for: not base64 · fewer than 2 bytes · unknown format tag · a record whose length runs past the end · trailing bytes that are not a whole record · duplicate tag · records out of order · a fixed-width field of the wrong length · missing `t` · missing `v` · empty `t` after normalisation · invalid UTF-8 · a deflate bomb (the existing `NoteMaxRunes*4+1` `LimitReader`). **A malformed payload must also never trigger a write.**

**Append-only extensible:** an unknown tag is skipped by its length and the note is still shown. This is the only reason a field can be added later without stranding links.

## Real, generated bytes

**A — the ordinary named note. 175 characters** (records 96 B, blob 97 B, tag `r`; deflate declined):

```
https://bibletext.co.uk/bsb/john/3/#v16-18&n=cmYEQW5uYWlpBpo_AcdSjmsGTRK7cArhcwTwxtQBdD1SZWFkIHRoaXMgb25lIHNsb3dseSDigJQgaXQncyB0aGUgbGluZSBJIGtlZXAgY29taW5nIGJhY2sgdG8udgNic2I
```
`f`=Anna · `i`=9a3f01c7528e · `k`=4d12bb700ae1 · `s`=3482480 · `t`="Read this one slowly — it's the line I keep coming back to." · `v`=bsb

| case | tag | payload | URL |
|---|---|---|---|
| A — named, typical, `/bsb/john/3/#v16-18` | r | 130 | **175** |
| B — **`a` record firing**, BSB Romans 16:25 whose reference is WEB 14:24, `/bsb/romans/16/#v25` | r | 126 | **171** |
| C — anonymous minimal, `/nkjv/psalms/23/#v1-4` | r | 51 | **98** |
| D — worst realistic: 280 English runes, 23-rune name, `a` with zero runs, every field, longest path `/webc/song-of-solomon/150/#v119-176` | **d** | 151 | **212** |

Deflate earns its keep exactly where it should: it declines on short notes and takes a 339-byte record stream to a 151-character payload on the long one. Today's v1 codec puts case A at ~127 characters, so the whole correspondence + anchor layer costs **+48 characters** on a typical note.

## Sender fields ride only with a note

`ShareLinkURL` — the plain "Share as link" verb — emits byte-for-byte what it emits today: no `i`, no `k`, no `f`. Otherwise every shared verse leaks the reader's stable correlator downstream of the message for a feature they did not invoke. **Hard rule, not a default.**

## Deliberately not carried

- **The verse span.** Already in `v16-18`; the web reader needs it there for CSS `:target`; two sources of truth for one fact is how a note and its highlight drift apart.
- **A reply-to / thread id.** The sending device has no record of which notes the recipient holds, so it would routinely name a note the reader never received. Threading is derived from (person, book, chapter), which cannot dangle.
- **Anything scrapbook-shaped.** The wire delivers one note with enough identity to be filed correctly. Grouping, ordering, unread counts are computed on the receiving device.

## Two things that must be fixed FIRST

**1. The fragment is not the order-free key list it is documented to be.** `share_link.go:9-17` promises "UNKNOWN KEYS ARE IGNORED, NEVER REJECTED — by every parser". `parseVersePayload` (`share_link_parse.go:136-141`) **cuts the fragment at the first `&`** and then only accepts a leading `v`/`v=`. I read the code: `#n=<payload>&v16-18` loses the verse entirely, and so does any fragment with any key before `v`. The web reader's `fragKeys()` accepts `v=` anywhere, so app and site **already disagree**. Route `v` through `fragmentKey`, accept the bare `v16` token at any position, pin both spellings at both positions in both decoders. Independent, no UI, shippable alone.

**2. The clean slate closes on submission.** Verified: `git ls-tree v1.1.7` has no `share_note.go`, no `notes_store.go`, no `share_link.go`. But `cmd/mobile/FyneApp.toml` reads Version 1.1.8 / Build 160 with the v1 codec on main. The instant 1.1.8 ships, a population of v1 readers exists that shows **nothing** for a record-format link — a note silently lost, the one invariant with no recovery. **Either the record codec lands before 1.1.8 is submitted, or notes are held out of 1.1.8.** There is no third option; you cannot tell an old app anything.

Keep `'p'`/`'z'` decode-only **forever**. Two branches, never removed, so any link ever emitted by a dev build or the currently-published `reader.js` still opens.

######################################################################
## STORE
######################################################################
## Shape: one preference key, line-framed, plus one sibling counter

**`shared.notes`** — line 1 is a header (deliberately not a note object, so it can never be mistaken for one); then people; then notes in `Seq` order; then a `{"k":"q"}` marker and every unparseable line, verbatim.

```
{"k":"h","s":2,"seq":186,"n":4,"w":1755250912}
{"k":"p","id":"p1","sx":["9a3f01c7528e","b0117d44a201"],"al":"Mum"}
{"k":"p","id":"p2","sx":["c81de2f04477"],"mu":true}
{"k":"n","id":"nm1x9z-4kf2q7bd","q":179,"o":"4d12bb700ae1","sx":"9a3f01c7528e","cn":"Anna",
 "a":{"b":"John","v":"bsb","r":{"c":3,"lo":16,"hi":18}},
 "t":"Read this one slowly.","ts":1755250000,"st":1755249300,"fs":1755250120}
{"k":"n","id":"nm1xa1-9plz3vqn","q":180,"a":{"b":"John","v":"web","r":{"c":3,"lo":16}},
 "t":"praying for tomorrow","ts":1755253300,"m":true}
{"k":"n","id":"nm1xb4-2vv0kd7c","q":181,"o":"77aa10bc9e02","sx":"9a3f01c7528e","cn":"Anna R.",
 "a":{"b":"Romans","v":"bsb","r":{"c":16,"lo":25},"rc":2,"rf":[{"c":14,"lo":24}]},
 "t":"the doxology sits at the end here","ts":1755330000}
{"k":"n","id":"nm1xd1-pz07k3ab","q":185,"sx":"9a3f01c7528e0","me":true,
 "a":{"b":"Psalms","v":"nkjv","r":{"c":23,"lo":1,"hi":4}},"t":"sent to Dad","ts":1755400000}
{"k":"q"}
{"k":"n","id":"nm1x77-badline","a":{"b":"Jo
```

**Line framing, not one JSON array.** The measurement is the whole argument and it is decisive: on 1,000 notes, one corrupt byte recovers **0** from an array and **999** line-framed; a 50% truncation recovers **0** versus ~500. A note exists nowhere else. Cost is +28% on disk (826 KB → ~1.06 MB at 5,000 notes), which is nothing.

**Byte-stable by construction.** Header, people by ID, notes by `Seq` ascending, quarantine. No map range anywhere in the serialiser, so an unchanged store hands Fyne an identical string and Fyne's `set()` short-circuits with no file write at all.

**`shared.notes.n`** — the sibling counter, decimal, holding the last written note count. Written in the same mutation as the blob.

### What the counter actually detects — and what it does not

All three candidate designs claimed their seal/count/hwm catches the Fyne `os.Create` truncation. **It does not, and this is my correction to all three:** the counter lives in the same `preferences.json` the truncation empties, so it goes with it.

What it *does* catch is a **value-level** wipe: an empty `shared.notes` string with a counter > 0 — a bad encode, a hand edit, a partial write of one key, a `deleteAllNotes` that ran without its sibling. That is `storeWiped`: refuse every write, tell the reader once. And shipped builds already apply `patches/fyne-2.7.4-atomic-prefs.patch` (verified in `.github/workflows/release.yml` and every `scripts/*.sh` release path), so file-level truncation cannot recur in a shipped build; only `go run ./cmd/desktop` is unpatched, because `go.mod` ships stock Fyne.

I deliberately reject the monotone high-water-mark variant. `header.NextSeq < hwm ⇒ refuse every write` on two independently-written preference keys can be tripped by one crash between the two saves, and the store is then **permanently read-only** with every arriving note discarded. A tripwire that can brick the feature is worse than the failure it guards. The narrow trigger (empty blob + count > 0) cannot false-positive.

**`deleteAllNotes` must write a HEADER with `NextSeq` preserved and zero note lines, and set the counter to 0 — never the empty string.** Verified: `notes_setting.go:57-62` currently writes `""`, which under this design would trip its own alarm and tell the reader their notes went missing when they deleted them.

### The parse contract

```go
func readStore(p prefStore) *notesIndex   // .status, .damaged, .quarantine
```

Four rules, in order of how much they matter:

1. **`status.writable() == false` stands every writer down.** This is `readNotesChecked`'s `ok=false` contract preserved verbatim and for the same reason: every mutation is a read-modify-write, so a failed read that answered "no notes" would serialise emptiness over the reader's collection and *their next action* would be the thing that destroyed it. It is now *narrower* — one bad line no longer poisons the read.
2. **A bad line is QUARANTINED, never dropped.** Carried verbatim through every rewrite. Nothing the app cannot parse is ever deleted by the app. Today's filter (`notes_store.go:100-104`) drops on read and `writeNotes` makes it permanent.
3. **A higher `Schema` is read best-effort and never written back.** A newer build's store is not this build's to rewrite.
4. **`damaged > 0` is TOLD, once:** *"Some notes could not be read. Nothing has been deleted."* The reader cannot distinguish a note they never received from one the app lost, so the app must say which it is.

## Capacity: NO cap, NO eviction, ever

`notesMax = 200` is deleted along with the tail-truncation.

Today's behaviour, verified in `writeNotes`: it sorts by **storage key** and keeps the **tail**, so survival is decided by the alphabetical order of the translation id. Measured, 300 notes written with cap 200: survivors `webc` 75, `web` 75, `nkjv` 50, **`bsb` 0** — every BSB note destroyed, and the window kept notes from 300 hours ago while discarding everything newer than 76 hours. At cap, an arriving note is discarded by the very write that stores it while the reader is looking at it on screen (X3).

10,000 notes is ~1.5–3 MB and tens of milliseconds to parse, once. A reader would have to receive one note a day for 27 years. **The cap was defending against a cost that does not exist, using a rule nobody can perceive.**

Instead, a **soft notice at 2,000** — a line in Settings, not a modal: *"2,143 notes from 14 people since March"*, with Export-all and a filtered delete beside it. **Nothing is ever removed without the reader naming it.**

The tradeoff, named: unbounded store means an unbounded load parse (~9 ms at 5,000 notes on an M3 Max, plausibly 30 ms on a phone, on the existing `StartBackgroundLoad` goroutine beside a Bible parse an order of magnitude larger) and an unbounded whole-file `preferences.json` rewrite, which the app already pays on every chapter turn for `reading.state`. The notice at 2,000 exists to catch a pathological store years before it bites.

Deletion is **per note, per person, and all** — three verbs, each destructive, each confirmed, each saying that nothing can fetch them back. "Delete all from this person" is the one a reader actually reaches for and the current design cannot express at all.

######################################################################
## OVERLAP
######################################################################
## The model

Overlap is a **per-verse set union over small integers**, flattened in Go into disjoint runs before anything is drawn, and handed **identically** to all five emitters.

This is forced by measurement, not chosen. On the real AppKit importer, `<span style=background:#ffff00>bbb<span style=background:#0000ff>ccc</span>ddd</span>` imports as three runs — yellow, blue, yellow. The inner **replaces** the outer; there is exactly one `NSBackgroundColorAttributeName` value per character. Android's `Html.fromHtml` agrees, and the styled pane draws a single rectangle. **Four of five surfaces physically cannot layer tints.**

Granularity is per-VERSE everywhere — verified: no character offset in `mark.go`'s `VerseSpan`, none in the store, none in the wire, and `share.go:975-986` already rounds a mid-verse selection to whole verses. So k notes flatten to at most 2k−1 runs.

```go
func chapterTint(notes []drawnNote, focus NoteID, lastVerse int) []tintRun
```

Only notes that are **placed**, **not minimized**, and whose **sender is not muted** contribute. Skipping minimized notes is the shipped semantics: hiding a note takes its highlight with it.

## The tint table — at most TWO values live at once

| focus? | verse covered by | tint | means |
|---|---|---|---|
| none | 1 shown note | `tintNote` | a note is here |
| none | ≥2 shown notes | `tintMulti` | **more than one** note is here |
| set | the focused note (whether or not others too) | `tintNote` | the note you have open |
| set | only other shown notes | `tintOther` | another note is here too |

`tintNote` **is** the shipped palette `Highlight`, so **a chapter with exactly one shown note renders byte-identically to today**. That is what lets the flattening layer land first as a pure refactor with `k=1` as its own case.

`tintOther` and `tintMulti` are **never co-present**, so the reader never has to hold three meanings at once. Where the focused note and another overlap, the verse is `tintNote` and the overlap is reachable by tapping it — the chooser names every note covering that verse. Colour never carries a count.

## The palette, measured

I computed these in-tree. My ΔE reproduces `theme.go`'s own recorded light-band figure of **44.7 exactly**, and ranks the dark values the same way its comment does, so the arithmetic is calibrated against a number the owner already judged on rendered output.

**LIGHT** (paper `#FDFCF8`, red `#B23A2E`):

| tint | hex | ΔE from paper | red-on-band | body-on-band |
|---|---|---|---|---|
| `tintNote` (shipped) | `#FFE08A` | 44.7 | 4.60 | 12.29 |
| `tintOther` | `#FFE9A8` | 33.0 | **4.94** | 13.19 |
| `tintMulti` | `#F5C24A` | 65.5 | 3.59 | 9.58 |

**DARK** (paper `#221F1C`, red `#E57373`):

| tint | hex | ΔE from paper | red-on-band | body-on-band |
|---|---|---|---|---|
| `tintNote` (shipped) | `#543E10` | 32.3 | 3.39 | 7.94 |
| `tintOther` | **`#48370F`** | 27.2 | **3.84** | 9.00 |
| `tintMulti` | `#5C4412` | 35.5 | 3.07 | 7.18 |

**This is the correction to Correspondence's dark palette.** It proposed `#3A2B0C` for the quiet tint — the exact colour `theme.go:114-117` records rejecting ("ΔE 22.9 where the LIGHT band measures 44.7... easy to lose while scanning"). My measurement puts it at 20.5. `#48370F` sits at 27.2 — **84% of the shipped band's presence, 33% more than the rejected value** — and its red-letter contrast *improves* to 3.84.

**Ceiling, from `theme.go`'s own note:** red-on-band falls 4.6 → 3.4 → 3.1 at `#5C4412` → **1.7 by `#8A6828`, where His words visibly sink into the gold.** There is no third depth away from the paper. Extend `theme_contrast_test.go` to assert red-letter-on-`tintMulti` and on-`tintOther` in both themes, and **judge both on the rendered pane before shipping** — `theme.go:128-130` says that is the only way this palette has ever been decided, and I have not rendered anything.

## Per surface

| surface | mechanism | verdict |
|---|---|---|
| **browser** | per-verse `<span class="v">` already exists; add `.hl2`/`.hl3`; `highlightRange()` → `applyRuns()` | trivial |
| **iOS / macOS** | three CSS classes stamped per verse at the **five existing** `.hl` sites in `buildChapterHTML` (`reading.go:646, 655, 674/676, 705/707`) — verse number, joining nbsp, joining space, red-letter run, whole verse. Runs are disjoint so nested spans never occur | with work |
| **Android** | identical runs as inline `style="background-color:#..."` (`fromHtml` ignores `<style>`) | with work |
| **styled pane (Win/Linux)** | see prerequisite 1 | with work, **first** |

## The chrome carries identity, count and attribution

Nothing badge-shaped survives the attributed-string import — measured: `border-left`/`padding-left`/`radius` on a `<p>` import as `headIndent = 4.0` and nothing else, `textBlocks = 0`; `reading_ios.go:525-530` records the same from iOS 26.5. A per-note gutter stripe is **not implementable** on Apple through this path, and the anchor is wrong for it anyway (a gutter is per-paragraph; verses share paragraphs).

So: the **focused note's bubble** plus a **chip row** — one chip per other note on this chapter, each with its person label, the sender's own citation, and the translation when it differs.

**Eight model arms, three sentences.** This is the graft that answers Judge B's cost objection:

| arms | sentence on screen |
|---|---|
| `placedNative`, `placedExact`, `placedMoved` | *(none — just the tint and the chip)* |
| `placedPartial`, `placedOtherChapter` | "This note also covers Romans 16:25 here." / "This note is at 16:25 in this translation." |
| `unplacedAbsent`, `unplacedIncommensurable`, `unplacedNoBook` | "Not in this translation. **Read it in <T>**" — the `Why` string plus a one-tap action |

Three strings × four surfaces, two of them behind cgo, instead of eight.

**And the at-most-one-expanded cap retires the biggest cgo risk in the prior design.** `docs/NOTES_DESIGN.md` risk 1 is the Apple multi-band layout: bands at multiple anchors, stacking where two notes share a paragraph, behind cgo, untestable. With one focused note there is **one band at one anchor, exactly as today** — verified that `gNoteView`, `gNoteAnchorVerse`, `gNoteBandH` and `paragraphSpacingBefore` (`reading_ios.go:542-548, 686-706`) all stay singular. Only the chip strip is new, and for the first plural release it can be text folded into the existing sticker through the existing ABI.

## Three prerequisites, in this order

**1. The styled pane's band must stop being one full-width rectangle over a line range.** Verified: `reading_styled_layout.go:92` holds a single `HighlightStart, HighlightEnd` line-index pair and `reading_styled_pane.go:278, 335-347` draws one `canvas.Rectangle` at full column width. Measured, 41% of John 3's lines carry two verses, so a 16-18 mark already lights runs of v15 and v19 — a truth divergence from the other four surfaces **today**. With two adjacent notes that spill *is* the difference between them.

*Build it from run geometry, not offsets.* `reading_styled_layout.go:41-56` already gives each `styledRun` a `Verse`, an `X`, a `W` and a `Highlight bool`. Replace the bool with a tint, walk `lay.Lines[i].Runs`, coalesce contiguous same-tint runs per line into rects. Smaller than routing through `selectionSpans()`/`xForOffset`, and it lands as a **pure bug fix** producing today's output minus the spill — shippable before any note work.

**2. Native bookkeeping → a per-note range table.** `reading_ios.go:1353-1360` **unions** every background run into one `gReadingHighlightRange`, justified at `:1350` as "safe because at most one passage is ever highlighted"; `reading_macos.go:656-661` takes the **first** run and stops. Both gate the tap target and the clear/hide/delete menu. Replace with `verse -> []NoteID`. Tapping a verse under one note focuses it; under two or more it offers a chooser naming each by person and date. **Never a guess.**

**3. Read-along must restore, not erase.** `reading_ios.go:1010, 1033-1041` (macOS twin at `:426, 451-458`) **removes** `NSBackgroundColorAttributeName` over the narrated verse and re-adds its own, so narration passing over a noted verse erases the note's tint from live text storage; the styled pane reconciles with a latch (`styledHighlightCeded`), not a model. Once `chapterTint` exists, export `tintUnder(verse) -> colour` and have the native side call `restoreTint(verse)`. One place decides a verse's background; the latch dies.

**This same primitive is the answer to a cost all three designs walked past.** `reading_ios.go:2061` skips the rebuild only when the fingerprint is unchanged, and the tint depends on the focused note — so *every chip tap* changes the fingerprint, rebuilds the HTML and re-imports the whole chapter's `NSAttributedString`: 20–36 ms on Psalms 119 on an M3 Max, plausibly 60–150 ms on a phone, per tap, on both cgo surfaces. **A focus change must mutate the affected ranges live through `restoreTint`, exactly as read-along already does, without touching the fingerprint.** Build the primitive once; it solves both.

## Performance is not the binding constraint

Psalms 119 (176 verses), 25 iterations, real WEB text, M3 Max: plain import 20.0–25.1 ms · one span 20.0 · 10 bands 19.0 · 25 bands 21.9 · 50 bands 26.5 · every verse its own band 36.2. Go-side `buildChapterHTML` is 106–152 µs. **The ceiling is the two-step colour headroom and, on the styled pane, the per-line spill.** Both bite long before band count does.

######################################################################
## SENDER
######################################################################
## The shape: opaque id ALWAYS, name OPTIONAL and off by default, plus a reader-owned layer

Four options; only one answers *"is this the same person who sent me the other three?"*

| | answers it? | privacy |
|---|---|---|
| typed per share | **no** — names collide, one person types differently each time | per-share consent, its one virtue |
| persistent name only | badly — a name is a **label**, not an identity | worst: set once, then leaked into every link forever with no fresh decision |
| opaque id only | **exactly**, and it is the only option that does | a stable cross-recipient correlator |
| **id + optional name** | **yes** | **chosen** |

**The argument that decides it.** Everything in a link is visible to every onward recipient, forever, with no revocation. A stable opaque id *is* a correlator — the same six bytes in every link the device sends, so anyone holding two links from different recipients can prove common origin. That is real. But the identical argument applies to a **name** with full force, and a name *additionally* identifies a person to a stranger. **An opaque id leaks strictly less than a name.** So: id always, name optional and off.

## The rules

- **`i` — sender id.** Six bytes from `crypto/rand`, minted once, stored in preferences, **never derived** from the device name, IDFV, MAC, or an install id. Offline, no account, nothing registered. ~1.8e-9 collision at 1,000 correspondents.
- **"Reset my sender id"** in Settings, beside the notes switch. The only answer to the correlator problem, and it costs one button. Its subtitle states the consequence honestly: *your future notes will look like they came from someone new to everyone you write to.* **"Delete all notes" resets it too, and says so** — a reader clearing everything plainly wants their own correlator gone.
- **`f` — display name. Empty by default.** Set in Settings; **pre-filled, editable and clearable in the compose sheet on every share**, so disclosure is a live decision rather than a forgotten one. `share_note_ui.go:168-218` already has the form and live counter; the field slots in. **Never** auto-filled from the device name, the contact card or an account — "Willow's iPhone" is a real name the sender never chose to disclose.
- **One refinement on "off by default"**, because a scrapbook of "someone" is a dead scrapbook: on the **first** compose, the sheet shows the name field empty with one line of explanation and an obvious way to leave it blank. Still an explicit choice, privacy-equivalent to a stored default of off, but asked at the moment it means something.
- **Sender fields ride only with a note.** `ShareLinkURL` emits exactly what it emits today.
- **The app NEVER groups by name.** Grouping is by `SenderID`, then by the reader's `Person`. Two ids that both say "Mum" are two groups, drawn as two groups. Every note with no id is **its own group** — never pooled with other anonymous notes, because three strangers drawn as one correspondent is a quiet misattribution in a system whose whole purpose is attribution.

## Validation — `normalizeSenderName`

A sibling of `normalizeNote`, running symmetrically on encode and decode — ONE function, so the two sides cannot disagree (the discipline `share_note.go:119-124` already establishes).

1. Newline/tab/CR → space, collapse runs, trim. **A name is one line** (unlike note text, which keeps its newlines).
2. Strip exactly what `normalizeNote` strips: C0, DEL, C1, LRM/RLM, LRE..RLO, PDF, the isolates U+2066–2069, BOM.
3. Cap at 24 runes.
4. **Reject anything whose case-folded, space-stripped form contains `"bibletext"`.** "BibleText Support", "bibletext  security", "BIBLETEXT", "Anna\nBibleText Support" all → `""`. This is the phishing case `docs/SHARED_NOTES.md:100-107` names, and the only impersonation a blocklist can honestly catch.
5. NFC-normalise before storing and before comparing. **Owner decision** — Go's stdlib has no NFC (`golang.org/x/text/unicode/norm` is a new dependency; the JS side has `String.prototype.normalize`). Without it, two canonically-equal spellings of one name are two groups.
6. **Homoglyphs cannot be filtered.** "Аnna" with a Cyrillic А survives, as it must — it is also a legitimate name. The answer is display, not filtering.

## Display: the app composes the frame, the sender fills only the slot

`byline{Label, Source, Person}` travels as a **type** into every surface, so the quoting rule cannot be forgotten one surface at a time:

| source | rendering |
|---|---|
| `labelNone` | `Note from someone` — the app's own quiet voice, never a fabricated name |
| `labelClaimed` | `Note from “Anna”` — **quoted**. The quote marks are the cheapest honest signal that the string was quoted from a link, not asserted by the app |
| `labelAlias` | `Note from Mum` — unquoted, because the **reader** typed it |

Plus: muted attribution styling, never bold, never branded — exactly what the literal `"Note from Friend"` already does at `reading_ios.go:765`, `reading_macos.go:997`, `notes_banner.go:62`, `assets.go:876`. **The frame stays; the word "Friend" becomes the slot.** The name is bidi-isolated (`<bdi>` on the web, its own label elsewhere). The name appears **only** inside the note's own bubble, chip or row — never in a notification, a share-sheet title, a window title, or any OS string.

**One caveat I am adding, which Correspondence missed:** `labelAlias` is unquoted and reads as the reader's own trusted word, but it is keyed to a **forgeable** six bytes. Someone who receives one link from Mum can set their own sender id to hers and have their words drawn under the alias the reader believes they own. So `labelAlias` must remain visually distinguishable from a fact the app is asserting — the same muted attribution weight, and never bold — and the person header must carry the sentence *"grouped by an id anyone with one of their links could copy."*

## Say plainly, in code and copy

**An opaque id authenticates nobody.** The app may say "3 notes from the same sender". It must **never** say "verified", "confirmed", or state the name as fact.

## The reader-owned layer

This is what the people lens buys that neither the wire nor the store can:

- **Alias.** The reader names a person. The one trusted string in the system.
- **Name history.** Every distinct `ClaimedName` an id has used, newest first, with when it was last seen. Without it a rename is invisible: it looks like a different person, or an impersonation slides past. The person view says *"used to say Anna"*.
- **Link / split.** One human with two devices, or an id that got reset, is two `SenderID`s. "Same person as…" merges them; splitting undoes it. **Reversible**, because a wrong merge attributes one person's words to another. A linked person's rows must say *"you linked these"*, never present the merge as a fact about the world.
- **Mute.** Stored and browsable, never drawn, never counted unread. Without it there is no defence against unwanted notes short of deleting somebody's messages — verified: the app cannot stop links reaching it (`notes_setting.go` says so, and it is right: entitlement + manifest are build-time).
- **"You".** The reader's own composed notes stored with `Mine: true`, so they group as You, stay out of the unread count, and stop vanishing when the compose sheet closes.

## What the fragment does and does not buy

Private **from servers** — the argument in `docs/SHARED_NOTES.md:36-40`, and it is sound. Private from **nobody who holds the link**, and `shareVerseLinkWithNote` already puts the note text in the message body in plain text as well. Everything on the wire is visible to every onward forward, forever. **Never put the note or the name in the path or the query.**

######################################################################
## HARD_CASES
######################################################################
| # | Case | Defined answer | Mechanism |
|---|---|---|---|
| 1 | **Same passage, same translation, different senders** | Two notes, both stored, both drawn. Verses covered by both → `tintMulti` (or `tintNote`/`tintOther` when one is focused). Two chips, two person labels. Deleting one leaves the other untouched. | No map keyed by passage exists. `noteKey` is deleted; the store is a list and identity is a locally-minted `NoteID`. This is the case today's key destroys — measured 24% loss at 200 notes, 72% at 5,000. |
| 2 | **Same passage, different translations** | Both stored under their own `Anchor.Version`, both in the same `byRef` bucket, both drawn, each resolved independently. The scrapbook's Passage axis shows them under **one heading**. Each chip carries the sender's own citation + translation label. | `byRef` is keyed in **reference space**, so a version switch invalidates nothing. `Anchor.Version` is never rewritten — the note stays a remark on particular wording. |
| 3 | **Overlapping spans** | Flattened to disjoint verse runs before any emitter sees them; at most 2k−1 runs. Two tint values live at once. Verbs never come from a tint — they come from a chip, or from a verse tap resolved through the per-verse `[]NoteID` table, which offers a **chooser** naming each note when more than one is under the finger. | Forced: nested spans on the Apple importer make the inner **replace** the outer; legible depth is 2 in both themes. |
| 4 | **A span that maps to a different chapter** | WEB Romans 14:23-24 read in BSB → `placedMoved`, `Here=[{14,23,23}]`, `Elsewhere=[{16,25,25}]`: tinted on 14 **and** on 16. BSB Romans 16:25 read in WEB → `placedOtherChapter` on ch.16 (nothing lit, but "This note is at 14:24 here") and `placedMoved` on ch.14. | Verified `MapVerse(web→bsb, Rom 14:24) = 16:25 moved`. `refChapters` makes the note a candidate on **both** chapters. Verbs take a `NoteID`, so Delete reaches it — X13 has no code path, because nothing rebuilds book+chapter from where the reader is standing. |
| 5 | **A span with a HOLE** *(judge-added)* | WEB Mark 9:43-46 read in BSB → `placedPartial`, `Here = [{9,43,43},{9,45,45}]`, sentence names what is missing. Two disjoint tint runs. | Verified: `MapVerse(web→bsb, Mark 9:44) = absent`, `9:43 = exact`. Only a **set**-valued resolve can say this; a `Lo`-probe design loses the whole note. |
| 6 | **A span in a book absent from this translation** | Three arms, three sentences. `unplacedNoBook` (Tobit in the WEB), `unplacedIncommensurable` (Greek Esther), `unplacedAbsent` (Daniel 13 under WEB). Nothing tinted, nothing guessed; counted into `chapterPlan.Elsewhere` so the chapter can say "2 notes here are on passages this translation does not have"; **always** in the scrapbook with a **"Read in WEBC"** action. | **`unplacedNoBook` is NOT derivable from `MapVerse`** — verified: `MapVerse("webc","web","Tobit",1,1) = 1:1 exact`, because `versificationDeltas` has no `"web"` entry. Derived from `protestantCanonBooks` + the loaded canon instead. This is the hole two of three designs would ship with, and it also closes DEADTAP (`notes_browse.go:250-257`). |
| 7 | **A translation the reader's build has never heard of** *(judge-added)* | The sender's `a` record carries their projection, or asserts `refNone`. The reader uses it rather than projecting. | Verified: `MapVerse("esv","bsb","Romans",16,25) = 16:25 exact` — `toReference` for an unknown id assumes WEB numbering. Without `a`, the note lands on the wrong verse, confidently. Cost +8 chars on ~130 verses. |
| 8 | **Two notes with identical content** | **Same link opened twice** → one note; `Received`, `FirstSeen`, `Minimized` preserved; only `Reseen` updates. **Same link forwarded by two people** → one note (it *is* one message; the link cannot know who passed it on). **Two people, same sentence, same verse** → **two** notes. **Two anonymous senders, same text, same minute** → two notes, because `OriginID` differs. | `dedupKey = (OriginID, hash(passage‖content))`, both halves together. Re-opening a link is the documented way a reader returns to a note (`share_link_open.go:262-265`); a naive upsert would un-minimize one they closed and shuffle it to the top. **Accepted loss, named:** legacy `p`/`z` links carry no `OriginID`, so the last row merges there. |
| 9 | **Same person, two devices / a reset id** | Two `SenderID`s. A first arrival from an unrecognised sender surfaces as a distinct event ("a new sender") with a **Link** control right there. The reader may merge them into one `Person`, and split again. Merged groups are drawn as *"you linked these"*, never as fact. | This is the graft from Correspondence. Anchor and Mailbox both silently under-answer here, and neither names it. |
| 10 | **The reader's own sent notes** | Stored with `Mine: true` under the reader's own sender id. Group as **You**, excluded from unread, kept out of "who wrote to me". | One field at one call site. Today "Share with note" stores nothing, so the sender's own words exist only in a messenger thread. |
| 11 | **A note arriving for a chapter the reader is not standing on** *(judge-added)* | Filed under the **link's** chapter. | Verified live bug: `rememberIncomingNote` writes `Chapter: state.CurrentChapter`. `newStoredNote` takes a `ShareTarget` and a clock and cannot read `AppState`. |
| 12 | **The focused note becomes unplaceable on a version switch** *(judge-added; no design covered it)* | `Focus` is a `NoteID` and survives the switch. If it resolves to an unplaced arm in the new frame, the note **stays focused and stays expanded**, its bubble carries the reason sentence and the "Read in <T>" action, and it contributes **no tint**. Focus is not silently reassigned — a message the reader deliberately opened must not be swapped for another. | `resolveFocus` falls through to the newest unread / newest not-minimized / `""` **only when the focused id is absent from this chapter entirely**, never when it is present but unplaced. |
| 13 | **One message split across several links** by the 280-rune cap *(judge-added)* | Each link is its own note. They land adjacent in the scrapbook (same sender, same passage, same minute) and read in order. **The wire does not stitch them** — a part-of-N field would let a lost part make a note look truncated when it was never sent. | Named as a deliberate non-feature. If it ever matters, it is a new record tag, which is what append-only is for. |
| 14 | **5,000 notes** | ~1.06 MB line-framed inside `preferences.json`. Parsed + indexed **once** at launch, ~9 ms on an M3 Max, ~30 ms on a phone, on the existing background goroutine. Per-navigation < 5 µs typical, < 20 µs at the worst realistic per-chapter cluster. Free-text filter 914 µs, unindexed. No cap; soft notice at 2,000; browse list **windowed**. | Against today's full-blob re-parse on **every navigation**: 0.25 ms at 200, 6.1 ms at 5,000, 11.9 ms at 10,000. The browse list's widget-per-note VBox (`notes_browse.go:330-333`) is the only real ceiling and it arrives thousands of notes before storage does. |
| 15 | **A corrupt store** | Line framing turns total loss into per-line loss (1,000 notes: one corrupt byte recovers 999, not 0). Bad lines **quarantined verbatim**, never dropped. `status.writable() == false` stands every writer down. Damage TOLD once: *"Some notes could not be read. Nothing has been deleted."* | `readNotesChecked`'s contract preserved verbatim; today's junk filter (`notes_store.go:100-104`) drops on read and the next unrelated write makes it permanent. |
| 16 | **The store is empty but shouldn't be** | Empty blob + sibling count > 0 → `storeWiped`: refuse every write, tell the reader. `deleteAllNotes` writes a header with `NextSeq` preserved and count 0, **never** `""`, so a deliberate delete cannot trip its own alarm. | Verified `notes_setting.go:57-62` currently writes `""`. **Honest limit:** this catches a value-level wipe, **not** whole-file truncation — the counter lives in the same file. Shipped builds already apply the atomic-prefs patch. |
| 17 | **Notes off, and a link with a note arrives** | The passage opens. The note is **not stored**. The offer card asks once, in the app's own voice, never quoting the note. Accepting turns the feature on and applies the parked target. | Storing a message the reader has said they do not want is worse than losing it — and the link is still in their messenger. |
| 18 | **A note for a chapter the reader cannot reach yet** (PARKED) | The document is complete the instant the link parses — id, anchor, sender, text — and **is appended immediately**, before navigation. A note for a chapter not yet downloaded is an ordinary row. | Closes PARKED, which today lives only in `pendingLink`, one process death from never having existed. Possible only because the passage is an attribute, not an address. |

######################################################################
## STAGING
######################################################################
Every step leaves `go build ./...` and `go vet .` green (both are green now, verified) and is shippable on its own. Estimates are working days for one person who knows this tree; the cgo steps carry the most variance.

---

**S0 — Fix the fragment parser. 0.5 d. No UI. No model. ZERO RISK.**
Route `v` through `fragmentKey` in `parseVersePayload`, accept the bare `v16` token at any position, mirror it in `cmd/websitegen/assets.go`'s `fragKeys()`. Pin both spellings at both positions in `share_link_parse_test.go` and in the JS. Closes a live app/web disagreement (`#n=…&v=16-18` gives no verse in the app and 16-18 on the site). **Do this first regardless of everything below.**

**S1 — Styled pane: band → per-line, X-bounded rects. 2 d. Isolated. Pure bug fix.**
Replace `styledRun.Highlight bool` with a tint value; walk `lay.Lines[i].Runs`; coalesce contiguous same-tint runs per line into rects. Delete `HighlightStart/HighlightEnd`. Output for one mark is today's minus the measured 41%-of-lines spill. **Ships on its own with no note work at all**, and it is the prerequisite that stops Windows and Linux drawing a lie about who marked what.

**S2 — `restoreTint(verse)` on the Apple panes. 2 d. cgo. RISKY — isolate.**
Give the native side a way to put back what was underneath instead of removing the attribute. Retires `styledHighlightCeded`, fixes read-along erasing a note's tint, and — the part all three designs missed — makes a focus change a live range mutation instead of a full `NSAttributedString` re-import (20–36 ms on Psalms 119 on an M3 Max, per tap). Verify on the simulator with the CGEvent harness.

**S3 — `chapterTint` + all five emitters, k=1 ONLY. 3 d. Pure refactor.**
Land the flattening layer with today's single-note behaviour. `tintNote` is the shipped `Highlight`, so **every surface must produce byte-identical output** — pin that with golden HTML for `buildChapterHTML` and `buildChapterHTMLAndroid`, a layout snapshot for the styled pane, and a DOM check for `cmd/websitegen`. Nothing user-visible changes. This is the commit that makes the multi-note case a data change rather than five simultaneous drawing changes on a subsystem whose last six commits each introduced a defect while fixing the previous one.

**S4 — The record codec + `normalizeSenderName`, emitting only `t` and `v`. 2 d.**
Encoder, decoder, the twelve fail-closed cases, the append-only skip, `'p'`/`'z'` decode-only forever. Port the identical table to `reader.js`. Sender fields decoded but **never written**. **THIS IS THE 1.1.8 DEADLINE STEP** — after this, an old reader can no longer be created. See risks.

**S5 — The store: `notes_model.go`, `notes_index.go`, line framing, quarantine, counter. 3 d.**
`StoredNote`, `Person`, `readStore`, `writeStore`, the in-memory index, `dedupKey`. Delete `noteKey`, `notesMax`, `saveNote`, `loadNote`, `deleteNote`, `setNoteMinimized`, `noteFromAnotherTranslation`, `noteStoreVersion`. `deleteAllNotes` writes a header, not `""`. Verbs take a `NoteID`. **Owner's own notes may be lost here and that is accepted.**

**S6 — `notes_anchor.go`: `newAnchor`, `resolveAnchor`, `refChapters`, `placementCopy`. 2.5 d. Pure Go, heavily testable.**
All eight arms, the three sentences, the reference-space index. Table-test every verified case in `hard_cases` rows 4–7. `newStoredNote` cannot see `AppState`.

**S7 — `buildChapterPlan` + the one gate + the set fingerprint. 2 d.**
`applyNoteForCurrentChapter` and the `ActiveNote`/`NoteMinimized`/`NoteVerseLo`/`NoteVersionID` quadruple are **deleted from `AppState`**. `chapterRenderFingerprint` folds `plan.Fingerprint` and drops its hand-rolled note clause. Add `notesFeatureOn == false` as its own column in the enumeration harness, asserting an **empty plan**, not merely an empty bubble.

**S8 — Surfaces read the plan; chips; per-note tap table. 4 d. cgo. THE RISKIEST STEP.**
iOS + macOS + Fyne banner + Android + `cmd/websitegen` all consume `[]drawnNote` and `[]tintRun`. Replace `gReadingHighlightRange`'s union and `gMacHighlightRange`'s first-wins with `verse -> []NoteID`. **De-risk it:** ship the first plural release with the chip count folded into the *existing* sticker text through the *existing* `bibleTextSetNote` ABI — zero new ObjC, zero new C ABI, on the two surfaces where a defect is least testable. The real chip strip is a separate, later step.

**S9 — Sender identity: id, name field, reset control, emit `i`/`k`/`s`/`f`/`a`. 2 d.**
Plus the framed, quoted, bidi-isolated `byline` on all four surfaces, replacing the literal `"Note from Friend"`.

**S10 — The people layer: `Person`, alias, link/split, mute, name history, `Mine`. 3 d.**
Reader-owned, non-load-bearing, and independently droppable if the owner wants to see notes ship sooner.

**S11 — Scrapbook: windowed `widget.List`, four axes, the "not readable here" section, the 2,000 notice. 3 d.**
Replaces the VBox-per-note in `notes_browse.go:330-333`. Must land before the store is pointed at a real scrapbook.

**S12 — Palette + contrast. 0.5 d + owner judgement.**
Add `tintOther`/`tintMulti` to both palettes; extend `theme_contrast_test.go`. **Then judge on the rendered pane in both themes** — `theme.go:128-130` records that this palette has only ever been decided that way, and I have rendered nothing.

**Total ≈ 30 working days**, of which ~8 are cgo and carry the variance.

---

## Throwaway, because nothing is deployed

Nothing here is a migration and nothing is written for compatibility with anything shipped. Specifically **discarded, not migrated**:

- `SharedNote`, `noteKey`, `notesMax` and the whole passage-keyed store (S5).
- `noteFromAnotherTranslation` and `noteStoreVersion` (S5/S6).
- The `ActiveNote`/`NoteMinimized`/`NoteVerseLo`/`NoteVersionID` quadruple on `AppState` (S7).
- The `'p'`/`'z'` **emit** path (S4) — the decode path is kept forever, but only two branches of it.
- The owner's own existing notes. Accepted, per the brief.

**Not throwaway, and must not be treated as such:** S0, S1 and S2 are fixes to *shipped* code paths (`parseVersePayload`, the styled band's spill, read-along erasing a background). They are worth landing whatever happens to the notes design.

## Reordering if 1.1.8 goes out first

If 1.1.8 ships with notes in it, S4 stops being free and the deadline is gone: a v1 population exists that shows **nothing** for a v2 link. The recovery is to keep emitting `'p'`/`'z'` alongside — which cannot carry a sender, an anchor or a nonce — i.e. this architecture's wire is dead. **Hold notes out of 1.1.8 rather than accept that.**

######################################################################
## INVARIANTS
######################################################################
## The honest limit, stated first

This is **one Go package**, so unexported fields buy nothing across files, and **Go has no sum types** — `PlacementKind` is a `uint8` any file can assign a bogus value to. "Structural" here means precisely:

> **The operation is REMOVED, not guarded.** `notes[key] = n` is unwritable because there is no map. `state.ActiveNote = ""` is unwritable because there is no field. `noteStoreVersion()` is unwritable because the function is deleted. Nobody writes those lines by accident, and writing them on purpose means adding the field or the map back — a visible diff in one named file whose comment explains why not.

Two things Go *does* buy and they are worth naming: **distinct named string types** (`NoteID`, `SenderID`, `PersonID`) do not interconvert implicitly and a bare `string` will not assign to any of them, so an accidental conversion is one grep; and **a struct's zero value can be made to mean something** — which is how `Mark{}` already makes "no highlight, no location left behind" unwritable.

## Structural — the operation does not exist

| # | Invariant | How |
|---|---|---|
| **S1** | **The passage is never a key.** | The store is a `[]StoredNote`. The only passage-keyed maps are index maps holding `[]int`, built by one function and written nowhere else. `noteKey` is deleted. The 24–72% measured silent overwrite has no assignment site. |
| **S2** | **A verb cannot address a note by where the reader is standing.** | Every verb takes a `NoteID`. `noteStoreVersion()` is deleted; nothing rebuilds book/chapter from `state.CurrentBook`/`CurrentChapter`. No function in the package returns a `NoteID` from a book, chapter or version — the only producers are `newNoteID()` and the index. X1, X2, X5, X13 have no code path. |
| **S3** | **No mirror to go stale.** | `AppState` holds no note text, no minimized flag, no note verse, no note version id. The quadruple has no pieces, so it cannot be assigned in pieces — the pattern that produced X1 and X2, and that leaked into `dev_links_on.go:145` on the very day the "the four are one value" commit landed. |
| **S4** | **A record cannot record where the reader is standing.** | `newStoredNote` takes a `ShareTarget` and a clock. It does not accept an `*AppState` and `notes_model.go` imports nothing that provides one. Verified this is a live bug today. |
| **S5** | **No mark without a meaning.** | Already shipped in `mark.go`: the zero `Mark` **is** absence, so there is no bool to set false while a location survives. Extended with `Mark.Note`, so ownership stays an equality once a chapter holds a set. |
| **S6** | **No arity-1 read, so no masking and no promotion.** | `buildChapterPlan` returns the set. There is no "the note on this chapter", so nothing can be "the one that surfaces when you delete the one in front of it". X6, X12, NOTE_MASKED, NOTE_SUBSTITUTED die together. |
| **S7** | **Placement is never stored, so it cannot be stale.** | `resolveAnchor` is a pure function of a document and a frame, run on every read. The note-side equivalent of `HL_FRAME` cannot happen. |
| **S8** | **Nothing in the store is invisible.** | `resolveAnchor` is total over 8 arms; the derive's only `continue` is "belongs to another chapter", which is not a loss because the note appears there. All 8 arms are drawn (as 3 sentences). |
| **S9** | **Tints cannot overlap.** | Every emitter's only tint input is `[]tintRun`, disjoint by construction. No API on any surface takes a note and a colour, so "two backgrounds on one character" is not expressible — which matters, because on four of five surfaces it is not drawable either. |
| **S10** | **Colour cannot carry a count.** | `noteTint` is a 4-value enum and `chapterTint` is its only producer. There is no path from `depth` to a colour index; the collapse is a switch over two cases, in the file that holds the measured contrast numbers. |
| **S11** | **Unparseable data is never on a path that destroys it.** | The serialiser's input is `notes ++ quarantine`. There is no filter between read and write. |
| **S12** | **The view's cap leaves zero bytes.** | `Minimized` is written only by a reader's press. "At most one expanded" lives in `chapterPlan.Focus`, session-only. Lift the cap in a year and no reader opens a chapter to four chips they never closed. |
| **S13** | **A person cannot lose a note.** | A note names a `SenderID`; a `Person` names `SenderID`s. No note lives inside a person, so a corrupt or missing `Person` degrades to a fallback label, never to a lost message. That is what makes it safe to keep both in one blob. |
| **S14** | **"Off" is one gate.** | `buildChapterPlan` returns the zero `chapterPlan`. Every surface consumes that one value; there is no second `notesFeatureOn` check to diverge from the first. Today's divergence is real — verified: `notes_banner.go:38` is gated and the native sticker path is not. |
| **S15** | **The fingerprint cannot forget a field.** | Computed inside the derive, over the plan's own contents, so a new field is folded in by the same function that added it. |

## Checked — with the reason each cannot be structural

| # | Invariant | Why not structural | Mitigation and residual |
|---|---|---|---|
| **C1** | The index agrees with the blob | Cache coherence is not a type | ONE writer; rebuilt from the blob on every mutation; count assertion in debug. **This is the principal residual risk**, and it is worth saying that this subsystem's whole recorded history is a mirror disagreeing with a store. I removed the quadruple and put a bigger, better-organised mirror in its place. |
| **C2** | The reader's versification table agrees with the sender's | The reader cannot verify a table it does not have | The `a` record covers the sender side. **No cover for a stale or wrong READER table.** I declined a table-version field on the wire: bytes on every link forever against a bug that has never happened. |
| **C3** | Immutability of `StoredNote` | Go cannot enforce it | No pointer-receiver methods in `notes_model.go`, one constructor, values passed by copy, one test. A convention with teeth, not a guarantee. |
| **C4** | Dedup correctness | A hash comparison is a runtime fact | Table test over all five cases including the anonymous corner. |
| **C5** | The codec fails closed | A parser property | Twelve malformed payloads, each asserting NO NOTE, plus "never triggers a write". |
| **C6** | The tint palette stays legible | Contrast is a measurement | `theme_contrast_test.go` extended; **and judged on the rendered pane**, which is the only way this palette has ever been decided. |
| **C7** | Read-along restores the right colour | Lives behind cgo as a live attribute mutation | `tintUnder(verse)` is offered; the native side must call it. |
| **C8** | Byte-stable serialisation | A property of a sort call | Serialise twice, compare bytes. An unstable order rewrites `preferences.json` on every navigation and makes the fingerprint flap. |
| **C9** | No surface reads the store directly | A convention | Grep test for `readStore(`/`writeStore(` outside the store file. |
| **C10** | The wipe detector | A heuristic over two preference keys in the same file | Catches a value-level wipe. **Does not catch whole-file truncation** — the counter goes with it. Shipped builds are already atomic. |
| **C11** | Sender authenticity | Not checkable at all | There is no server and cannot be one. Grouping is a claim; no UI may say "verified". Permanently a copy discipline. |
| **C12** | The URL fits every messenger | Nobody has measured any messenger | Bounded by `NoteMaxRunes`. Measured ceiling ~989 chars; **no device test exists.** |

## The governing rule, stated on its own

> **Fail OPEN toward showing. Fail CLOSED toward writing.**
>
> An unrecognised placement, an unknown wire record, an unexpected schema: **show it**, because the alternative is a message the reader never learns exists. `PlacementKind`'s switch has a `default:` arm that shows the note with the vaguest honest sentence.
>
> An unreadable store, a wipe signal, a failed parse: **do not write**, because a blob we cannot read today may be readable tomorrow and an overwritten one is gone.

Both halves have been broken in shipped code, in opposite directions, which is why they need saying together.

## What I deliberately did NOT make structural

**"At most one note expanded."** It stays a **view** rule, in `drawnNotes`, writing zero bytes. Capping by *action* — expanding B writes `Minimized = true` onto A — leaves a counterfeit minimize **byte-identical to a genuine one**. The store has one bit where the model would need two, so the information a migration would need was never recorded.

######################################################################
## OPEN_FOR_OWNER
######################################################################
- **Does 1.1.8 ship with notes, or without?** This is the only genuinely time-critical decision. Nothing about notes is in any shipped release (verified), so the wire format is free — but only until 1.1.8 goes to the App Store. After that, a reader on 1.1.8 sees *nothing* for a link written by any later build, which is a note silently lost with no way to recover it. Two options, no third: (a) hold notes out of 1.1.8 and ship the rest, or (b) delay 1.1.8 until the new codec lands (~1 week of the work below). I recommend (a) — 1.1.8 is already built and verified, and this buys the notes work all the time it needs.
- **Should the app remember the notes YOU send, not just the ones you receive?** Today "Share with note" keeps nothing: the moment the share sheet closes, your own words exist only in the messenger thread. Storing them costs one field and turns the scrapbook into a record of a conversation rather than an inbox. The cost is that your own notes appear in your own list, which some people will want and some will find cluttered. Recommend yes, marked "from you" and kept out of any unread count.
- **Should the app let you say "these two are the same person"?** A friend who reinstalls the app, or writes from both a phone and a tablet, will show up as two separate senders — and nothing on screen would say so, which means "everything Mum has sent me" would quietly give you half the answer. The fix is a button that merges them (and un-merges them). It is the single most valuable thing in the people side of this design, and also the most extra UI. Recommend yes, but it is droppable if you want notes shipping sooner.
- **Two new highlight shades need your eye, not a measurement.** When more than one note covers a verse, the app needs a second and third shade of the highlight band. I have chosen values by measurement — light `#FFE9A8` and `#F5C24A`, dark `#48370F` and `#5C4412` — and checked the red-letter text stays legible on all of them. But your own note in `theme.go` says this palette has only ever been decided by looking at the real page, never at swatches. Please look at all four on a red-letter chapter in both light and dark before it ships.
- **Do you want a name field on the share sheet at all?** Notes can travel with a name you choose ("Anna"), or anonymously. Anonymous leaks less and is the safer default — but a scrapbook where every note says "from someone" is not much of a scrapbook. My recommendation: leave it blank by default, and the *first* time you write a note, the sheet asks once, with an obvious way to skip. Say if you'd rather it never ask.

######################################################################
## RISKS
######################################################################
- **The index is a second copy of the truth, and this subsystem's entire recorded history is a mirror disagreeing with a store.** I have deleted the four-field `ActiveNote` quadruple and replaced it with a larger, better-organised mirror. It is a better mirror. It is still a mirror. One writer, rebuilt on every mutation, an enumeration axis, and a debug count assertion — that is discipline, not a type. If anything in this design fails in the field, this is the most likely place.
- **S8 is behind cgo on two surfaces and it is where the multi-note tint, the per-note tap table and the chip row all land at once.** `gReadingHighlightRange` unions every background run into one range and is *commented* as safe only because at most one passage is ever lit; `gMacHighlightRange` takes the first and stops. Both gate clear/hide/delete. Getting this wrong means a verb reaching the wrong person's message — the sharpest failure available. Mitigated by shipping the first plural release through the existing `bibleTextSetNote` ABI with the count in the sticker's own text, but the range table still has to be right.
- **The wipe detector does not detect the failure everyone cites.** All three source designs claimed their seal/count/hwm catches the Fyne `os.Create` truncation; it cannot, because it lives in the same `preferences.json` the truncation empties. What it catches is a value-level wipe. Shipped builds already apply the atomic-prefs patch, so file truncation cannot recur in a *shipped* build — but `go.mod` ships stock Fyne, so a dev build still has the non-atomic writer, and a bad value, a partial encode, a hand edit and filesystem damage all remain outside the detector's reach.
- **Placement is recomputed every read, so a versification-table change silently relocates existing notes.** That is the correct behaviour — a stored placement becomes wrong — but a reader who saw a note on Romans 16 last month may find it on Romans 14 today with no event and nothing to tell them. There is no good UI for it and I have not invented one.
- **The reader's versification table has no cover.** The `a` record protects against the *sender* having a table the reader lacks. Nothing protects against the reader's own table being stale or wrong for a translation both parties have. I declined a table-version field on the wire on cost grounds; if a delta is ever regenerated incorrectly, notes move and nobody is told.
- **Complexity paid up front by everyone for a payoff concentrated in ~130 verses.** Roughly 31,000 of ~31,100 verses map exactly across every shipping translation. The median reader will have single-digit notes for years and will pay for a tri-state anchor claim, eight placement arms, two index buckets, a quarantine, a nonce on the wire, a people layer and a windowed browser. The defence is that the failures these prevent have no recovery. The cost is real and this is the strongest argument against the design.
- **On day one the people axis carries no information.** Every note in flight and every note from an older build arrives with no sender id, and each is its own group labelled "someone". Alias, link/split and mute have nothing to attach to. The mitigation — surfacing a first arrival from an unrecognised sender with a Link control right there — does not recover the anonymous notes that arrived before it existed.
- **Windows and Linux stay the thinnest surface.** Both draw the note in a Fyne banner *above* the pane, so "the chip beside the verses it is about" is literally not beside them there. The design's most visible feature is weakest on the two platforms with no native sticker, and S1 must land before overlap is drawn at all or two adjacent notes are indistinguishable.
- **A focus change re-imports the chapter unless S2 lands first.** `reading_ios.go:2061` skips only on an unchanged fingerprint, and the tint depends on the focused note — so every chip tap is a full HTML rebuild plus an `NSAttributedString` re-import: 20–36 ms on Psalms 119 on an M3 Max, plausibly 60–150 ms on a phone, per tap, on both cgo surfaces. All three source designs walked past this. S2's `restoreTint` is the fix; if S2 slips, chip tapping will feel bad on a phone.
- **Every number in this document is an M3 Max, and nothing was rendered.** No phone timings, no simulator, no Windows or Linux build, no screenshots. The AppKit importer probes behind the overlap section are AppKit, not UIKit. The contrast figures are computed, not sampled — though my ΔE reproduces `theme.go`'s own recorded light-band figure of 44.7 exactly, which is the one calibration I have.
- **Delete-one rewrites the whole blob.** A reader tidying up will do it thirty times in a row, each a full-file write, on the reader's most destructive verb. The tempting escape — a tombstone with deferred compaction — I reject: the reader was told the message is gone and the text would still be on disk. If this bites, the honest fix is to accept the cost or change what Delete promises, not to leave the words there quietly.
- **Sub-verse spans remain impossible and now look easy.** The document model would carry a character offset without blinking; nothing else in the stack can — not the store, not `#v<lo>[-<hi>]`, not `VerseSpan`, not any of the five emitters, and `share.go:975-986` already rounds a mid-verse selection to whole verses. Making the model look capable of something the stack is not invites somebody to try.
---

# Identity — the decision record

> Settled with the owner on 2026-08-15, after the panel. **This section overrides
> the panel's `sender` section above wherever they disagree.**

## The constraint, stated plainly

**There are no accounts and no servers, and there is no plan for either.** The app
is offline, on-device, and has never had a login. Therefore:

> **No two notes can be known to come from the same person unless the reader says
> so.** Any grouping beyond a single install is USER-INITIATED, always.

The owner's words: *"I don't think we can do that without some kind of
user-initiated linking because we don't have accounts or servers. But, again, we
would like to build with these things in mind so we are not frozen out later."*

## What a sender id is, and what it is NOT

Each install mints one opaque `SenderID` the first time its reader shares a note,
and every note it sends carries it. It is not a name, not an account, and not a
fingerprint of the device — just a value that is stable for as long as that
install lives.

| It CAN | It CANNOT |
|---|---|
| group every note from one install of one app | recognise the same person on a second device |
| survive the sender renaming themselves | survive a reinstall, or a restored backup on new hardware |
| give a merge a durable thing to merge | tell you WHO anybody is |

So the id does the small, honest part automatically, and the reader does the rest
by hand. **Getting this wrong in the confident direction would be worse than not
having it** — an app that silently claims two people are one has misattributed
somebody's message, which is the one thing this subsystem must never do.

## What ships now

- Every stored note carries `Kind` (`mine` | `received`) and `SenderID`.
- **Own notes are STORED but never drawn in the reading text** (owner directive).
  They appear in the notes browser, which needs a way to show them — a filter or
  a section, marked "from you".
- The byline says **"You"** or **"Friend"**. No name is collected, no name is
  displayed, and the share sheet has no name field.
- **Delete-all clears your own notes too** (owner directive). It is one store.

## What is designed but NOT built

Reserved so that adding them later is additive rather than a migration:

1. **A name field.** `SenderName` rides in the record and on the wire, decoded and
   stored from day one, simply never written and never shown. When the share sheet
   grows the field, old notes keep saying "Friend" and nothing has to be rewritten.
   The field is untrusted text and its validation rules — length, no newlines,
   bidi isolation on display, no impersonation of app chrome — belong with the
   display, not with the store.
2. **The person layer.** A reader-owned mapping from a SET of `SenderID`s to one
   `Person`, with a reader-chosen alias. Merge and un-merge are both user actions.
   Nothing in the note record points at a `Person` — the mapping is a separate,
   rebuildable structure, so losing it loses a preference and never a note.
3. **An account id, if servers ever exist.** The record has room for a second
   identity field alongside `SenderID`. Because unknown fields are preserved
   across read-modify-write, a build that knows nothing about accounts will carry
   an account id through untouched rather than destroying it.

## The rule that keeps all three possible

**Identity is additive metadata on an immutable record, never part of its
address.** A note is addressed by its own `NoteID` and nothing else. So a merge,
a rename, a late-arriving name, or an account id arriving years from now changes
what the app can SAY about a note — never which note a verb reaches, never where
it is stored, and never whether it survives.
