<!-- A design plan. WHAT IS LANDED so far:
       c619743e4  the own-note predicate is one function
       0a4ffcef3  notes_chrome.go composes the value once; styledNote aliases it;
                  receivedSetShownAs and stickerIsTheReceivedSet move onto it;
                  two conformance sweeps over the enumeration's states
       a4b42f249  shouldCaptureScrollRestore is one question, asked by all three
                  panes, with the contracts tightened to require the call
     (step 3, Present / Collapsed / Chevron, landed with the pills work)
     <pending>  step 4, Tail: one appended flag on all three native ABIs. iOS and
                macOS gained gNoteTail + gNoteShapeExtra (resolved once in
                SetNote, read as a scalar by every band formula, the outline's
                detour gated); Android gained a trailing boolean on
                BtBridge.setNote and gates both the tail drawable and the bottom
                pad. Two guards came with it: the Go source contract
                (TestJNIDescriptorsMatchBtBridge) and a BYTECODE check the
                Android build now runs over the shipped dex
                (verify_android_jni_descriptors) — a JNI descriptor is only a
                string, and a stale one fails nowhere but on a device.
     <pending>  step 5, Verbs: the verb set crosses all three native ABIs as one
                appended int and the panes branch on it. Two Android divergences
                fell out — the bin was an emoji (drawn now, from the same Fyne
                path the Apple panes transcribe) and the verb corner reserved two
                slots whichever verbs were drawn. A contract reads the three
                numbers out of each native's own enum and holds them against the
                iota. `own` stays on the wire, unbranched-on, until an appended
                field makes the arity change visible.
     <pending>  step 6, Counts: the counts span crosses as the exact substring
                of the who line, found by a backwards search on each surface.
                Deletes btIOSWhoCountRange, btMacWhoCountRange and
                styledWhoSplit; Android gains the accent it never had (it
                painted the WHOLE line accent when nextable). The chevron moved
                into the composed line, so the contract requiring it in four
                renderers now requires its ABSENCE from all four. What is
                checkable is a COUNT, not a ban: the fit rule still needs the
                separator once per renderer, and a second occurrence is the cut
                growing back.
     <pending>  a reader's report, fixed alongside: pressing the chapter pill
                expanded the bubble without moving the page. Opening a note is a
                declared placement (openedNotePlacesTheView), and only the
                next-tap declared it; the exception — a next-tap between two
                notes on one verse — is stated at its own call site.
     <pending>  step 7, Arrival + Lead: one rule for all five surfaces
                (notes_arrival.go), three classes crossing as one appended int,
                Lead joining the spacing table. Deletes
                btIOSNoteSharesHighlightPara, btMacNoteSharesHighlightPara,
                Android's same-VERSE test and the web's absent one. Two
                precedences that were accidents of ordering — narration owns the
                viewport, a reopen's saved position beats both targets — are
                declarations now. Rests on a new invariant: every surface's
                paragraphs ARE the model's
                (TestEverySurfaceBreaksParagraphsWhereTheModelDoes), which four
                call sites had only by convention.
     <pending>  step 8 is SPLIT, because notesPillPerParagraph is a runtime var
                and chapterNoteGroups is gated on it: the plural path runs at
                N<=1 until the flag flips, so the mechanism can land as
                no-render commits. Done so far:
                  8a  the pill's verb is KEYED, not versed. The chapter-top
                      group shares paragraph 0's verse by design, so two pills
                      opened one note and paragraph 0's was unreachable.
                  8b  performNoteAction: one verb entry point that can name
                      which note. Every existing export becomes that call with
                      noteKeyFocused. Focus happens BEFORE the verb.
                  8c  noteChrome.Bands — every reservation the chapter needs,
                      WITHOUT heights (a height is a function of a pane, and
                      this value admits only functions of state/plan/verses).
                Also landed alongside: the Java-to-native JNI seam is checked
                (TestEveryJavaNativeHasAMatchingThunk). Short-name mangling
                omits the signature, so widening a callback to carry a band key
                would have linked clean and read garbage on a device.
                  8d-iOS  DONE: reservations are a swept list; the install is
                          two halves under one sweep; the specs cross as an
                          array with Go-composed labels; pills are a keyed view
                          registry whose press carries the group key into the
                          keyed verb export (bibleTextNoteAction). Verified on
                          the simulator both ways — flag off byte-identical,
                          flag on drew the first native per-paragraph pill.
                  8d-mac DONE: the full twin (swept BTMacNoteBandRes list,
                          bibleTextMacSetNoteBands, NSButton registry with the
                          keyed tag press). Window-captured: "Notes · 2" aligned
                          over its paragraph after a full navigation cycle.
                  8d-And  DONE: NoteBandSpan reservations coalesce ONE SPAN PER
                          PARAGRAPH with the heights summed (chooseHeight runs
                          per paragraph over one shared FontMetricsInt, so two
                          live spans there walk into the documented traps); the
                          take-back sweeps BY CLASS so no span is orphaned by an
                          overwritten handle; pills stack above the sticker's
                          share when a paragraph carries both. setNoteBands
                          crosses as ([I[I[B)V with '\n'-joined Go-composed
                          labels (sanitizeSenderName keeps names newline-free);
                          nativeNoteAction carries the keyed verb back. Emulator-
                          verified with four received notes on four John 3
                          paragraphs: four chips in their own bands, single
                          sticker stood down, keyed restore opened the pressed
                          paragraph's own group, co-tenant chip stacked above the
                          open bubble, and the flag-off push (n=0) cleared every
                          chip back to the shipped model. The release guard now
                          also asserts build-android.sh REFUSES extra build tags
                          on the release path (the debug APK accepts them, which
                          is how the dev page reaches an emulator).
                  8f     DONE: notesPillPerParagraph defaults TRUE — the
                         collapsed state ships as one pill per noted paragraph
                         on every surface. X16 struck as the shipped
                         experience; the enumeration's pin stays on the gate's
                         off value (the dev toggle's comparison state and the
                         one-line reversion), naming the debt a reversion
                         would reincur.
                  9      DONE: cmd/websitegen emits the byline, the pill
                         label and the arrival lead from the shared functions
                         at generate time (web_api.go seams, __NOTE_*__
                         placeholders beside __BOOKS__); the tail obeys
                         hasTail through a notail class on the chapter-top
                         parking; the bubble and the card speak one verb
                         vocabulary. The divergences that remain are stated
                         beside the code they excuse — the verse target's
                         header-inclusive margin, the own arm's absence, and
                         minimize's global highlight suppress on a page with
                         one highlight source — and the seams are pinned by
                         cmd/websitegen/note_chrome_shared_test.go, each pin
                         verified able to fail. The live site takes all of it
                         on the next publish-site.sh run.
                The nine steps are COMPLETE. Also landed beside them: the
                enumeration hardening (N11-N16 chrome tripwires with an
                unplaced axis and a stub measurer; every assertion
                mutation-verified) and the anchor fix it forced — placement
                no longer trusts a run naming a verse the loaded text
                demonstrably lacks.
     TWO HAZARDS, unaddressed and live today:
       - macOS saved reading positions and the first-paragraph band share
         textContainerInset.height. REFINED after doing the algebra: the
         verse-anchored path is inset-INVARIANT — delta is captured relative to
         the verse's own line (offY - (rr + inset)) and restored as
         rr + insetNow + delta, so an inset change cancels. The residual
         exposure is only (a) the whole-chapter FRACTION fallback, whose
         scrollable length includes insetH*2, and (b) the ordering of
         btMacInstallNote against the restore apply: a restore that resolves
         before the band installs uses the pre-band inset and lands bandH off.
         Neither is N-specific; both predate this work.
       - Android's noteBandSpan is BOTH the removal handle and the placement
         input, so the first moment a second band exists the first span is
         orphaned on the live text with no reference — a permanent gap, and no
         host test can see it because that file does not compile here. -->

# Unifying note chrome: one value, four adapters, one conformance table

## The decision, and the alternative it was taken over

Two designs were weighed: an adoption **registry**, and a pushed geometry plan. The registry is the spine, because the four defects were omissions — a fix applied to one surface and not the other three — and only the registry turns non-adoption into a test failure. Sharing the decision is not the same as landing it.

The pushed-plan design was right about one concrete thing, and that part is grafted in. Defect 3 (the tail on a note that points at nothing) does not live where a `Tail` bool would fix it. It lives inside the band formula, transcribed six times:

```objc
reading_ios.go:1127   gNoteBandH = kNoteGapAbove + h + (btIOSNotePill() ? 0 : kNoteTail) + kNoteGapBelow;
reading_ios.go:1379   CGFloat want = kNoteGapAbove + h + (btIOSNotePill() ? 0 : kNoteTail) + kNoteGapBelow;
reading_macos.go:1699 gMacNoteBandH = btMacNoteTopGap(ts, para) + h + (btMacNotePill() ? 0 : kMacNoteTail) + kMacNoteGapBelow;
reading_macos.go:1939 CGFloat want = btMacNoteTopGap(ts, para) + h + (btMacNotePill() ? 0 : kMacNoteTail) + kMacNoteGapBelow;
reading_macos.go:1903 ... + (btMacNotePill() ? 0 : kMacNoteTail);
```

The bug is that the tail's depth is gated on `btIOSNotePill()` — "is this collapsed" — when the question is "does this note point at a passage". Push a `Tail` bool and patch one branch and you have fixed the instance while leaving five copies of a locally-derived conditional standing. So the *shape* is taken — the term is resolved once, outside the formula — without the geometry push, which `notes_bubble.go` already rejected for reasons that still hold. The native resolves `gNoteShapeExtra = gNoteTail ? kNoteTail : 0` **once, at push time**, and every band formula reads that scalar with no branch in it. The numbers stay compile-time and parsed; only the *decision* crosses the wire. The registry's `banned` fragment then forbids `btIOSNotePill() ?` from appearing anywhere in `btIOSInstallNote` or `btIOSLayoutNote`, which is a check that can actually fail.

Three more places where one design was taken over another, stated up front:

- **No `HeadH`/`FootH`/`GapAbove` on the wire.** `notes_bubble.go` rejected pushing the spacing table because these are compile-time layout constants and a wire format in front of every 1pt change is a bad trade. That argument is still correct. The counter-argument — that pushing them avoids a synchronous native-to-Go measurement callback — is answered by not having a callback at all. Nothing measured ever flows into Go in this plan.
- **No per-span ABI calls for the who line.** Push the counts substring itself and let each native locate it with a *backwards* search. That beats both alternatives: it sidesteps the Go-UTF-8 / NSString-UTF-16 / Java-UTF-16 index mismatch entirely, and it survives a native that ellipsises the sender half before it splits. It is safe because `sanitizeSenderName` (notes_byline.go:120) maps U+00B7 to `-`, so the middle dot appears only as our own separator — a fact that gets its own test rather than a comment.
- **The registry gets a second table for shared predicates.** The registry reflects over struct fields; defect 2's restore-capture guard is a condition in three Go blocks, not a field, and the winning design was structurally blind to it. Fixed by putting the fact on the value (`ExplicitArrival bool`) and by adding an explicit `chromePredicates` list the same test walks. Say plainly: fields are covered automatically, functions are covered by hand-listing, and the hand-list is the weaker half.
- **Drop the macOS cgo oracle.** It was considered and set aside as the riskiest single piece of any of the designs. Refactoring shipping AppKit layout helpers off globals in order to test them, against a headless TextKit fixture that will drift from the real view's configuration, is touching the drawing code to test the drawing code. The twin diff carries the defect class that actually occurred and costs nothing.

---

## The seam

New file: `notes_chrome.go`.

`noteChrome` is a strict widening of the tuple that already exists (`appleStickerPush`, notes_plan.go:517), not a second seam beside it. Every field is a pure function of `(AppState, plan, verses)`. That admission rule is the only thing standing between this and a forty-field blob, and it goes in the file as a comment with teeth: **a field that needs a measured number does not belong here.**

```go
// noteChrome is every decision about the note's chrome that is NOT a
// measurement and NOT a drawing. Composed once; every surface reads it.
//
// ADMISSION RULE: a field may join only if it is a pure function of
// (state, plan, verses). Anything that needs a wrapped height, a string
// width or a fragment rect stays in the renderer that measured it.
type noteChrome struct {
    // The S9 tuple, unchanged in meaning. Already crossing the ABI today.
    Text   string // the sender's words, alone. "" = nothing open here
    Who    string // the app's chrome: byline · counts · "N not shown here"
    Pill   bool
    Next   bool
    Own    bool
    Anchor int    // the verse the band opens above; 0 = park at chapter top

    // The decisions each surface re-derives today.
    Present   bool            // Text != "" || Who != ""            (4 copies)
    Collapsed bool            // Pill || (Who != "" && Text == "")  (4 copies)
    Tail      bool            // this card points at a passage      (defect 3)
    Verbs     noteVerbSet     // which controls the card carries    (4 copies)
    Counts    string          // the counts run inside Who, "" if none
    Chevron   string          // " ›" — one literal, not four

    // The scroll half. ExplicitArrival is a FIELD rather than a predicate so
    // the conformance registry can see it (defect 2 hid in a condition).
    ExplicitArrival bool
    Arrival         noteArrival

    // The band list. One entry today; N once notesPillPerParagraph flips.
    Bands   []noteBandSpec
    ShownAs receivedShownAs // N9/X16, today typed on styledNote
}

type noteVerbSet uint8

const (
    noteVerbsNone     noteVerbSet = iota
    noteVerbsReceived             // minimize (en dash) outboard of delete (bin)
    noteVerbsOwn                  // dismiss (✕) alone
)

// noteBandSpec is one reservation request, renderer-independent. The three
// natives hold a single scalar today (gNoteReservedPara, gMacNoteReservedPara,
// noteBandSpan); turning that scalar into this keyed slice IS defect 4's port.
type noteBandSpec struct {
    Key   int  // matches reading_styled_layout.go:135 bandRequest.Key
    Verse int  // 0 = anchorless: park at the chapter top
    Count int
    Pill  bool
    Tail  bool
    Label string       // the pill's own label, when Pill
    Press noteAction   // what a press on this band does
}

type noteArrivalKind uint8

const (
    arriveNothing noteArrivalKind = iota // fall through to restore / top
    arriveVerse                          // the washed verse's own line
    arriveBand                           // the band above the note's paragraph
)

type noteArrival struct {
    Kind noteArrivalKind
    Key  int     // which band, when arriveBand
    Verse int    // which verse, when arriveVerse
    Lead float32 // breathing room above the target (today 12 / 16 / 24 / dp16)
}

// noteAction is a verb as a VALUE, so a new pressable thing needs no new
// native plumbing — only a new row in the registry. Trigger is here from the
// start: a new GESTURE is four native edits that nothing would otherwise catch.
type noteActionKind uint8

const (
    actNone noteActionKind = iota
    actOpenNoteAtVerse
    actNextNote
    actHideNote
    actDeleteNote
)

type noteActionTrigger uint8

const (
    pressShort noteActionTrigger = iota
    pressLong
)

type noteAction struct {
    Kind    noteActionKind
    Trigger noteActionTrigger
    Verse   int    // actOpenNoteAtVerse: the GROUP's band verse
    NoteID  uint64
}

// --- composition ------------------------------------------------------------

func chapterNoteChrome(state *AppState, plan chapterPlan, verses []Verse) noteChrome

func (c noteChrome) hasTail() bool { return c.Present && !c.Collapsed && c.Anchor > 0 }

// noteArrivalFor is the classifier that replaces four disagreeing predicates.
// It is answerable in pure Go because ALL FIVE surfaces build their paragraphs
// from the same groupVersesIntoParagraphs — reading.go:716 (Apple HTML),
// android_chapter_html.go:50, web_api.go:32, reading_styled_layout.go. That is
// the enabling fact; write it down or the next reader re-litigates it.
func noteArrivalFor(state *AppState, plan chapterPlan, verses []Verse, c noteChrome) noteArrival

// shouldCaptureScrollRestore: false on a render the reader ASKED for.
// Absorbs Android's preserveTop and carryDataSwapArrival arms (reading_android.go
// :645, :679, :695), which are strictly richer than the Apple panes' and which
// nobody ported back because there was no one place to put them.
func shouldCaptureScrollRestore(state *AppState, sameChapter, bodyChanged bool) bool

// performNoteAction is the ONE dispatcher. It must carry notes_store.go's
// identity discipline — NoteVersionID is an address, not a label — because one
// dispatcher means one bad Verse or NoteID reaches a destructive verb.
func performNoteAction(state *AppState, a noteAction)
```

Below a divider in the same file, the formula half — the fifteen decisions that are "one formula wrapped around exactly one measured number". These take the measurement as an argument. They are shared *where a surface can call Go* (styled pane, websitegen, and the enumeration with a stub measurer); the three natives keep their transcriptions and are held to the numbers by the parser, as today.

```go
// noteMeasure is what a renderer supplies. Exactly one field is genuinely
// per-engine; the rest are that surface's compile-time constants.
type noteMeasure struct {
    BodyH float32              // wrapped message height at this width
    WhoSz float32              // 11 / 10 (macOS) / 11sp / 11
    Btn   float32              // 28 styled / 30 iOS / 24 macOS / NOTE_BTN
    TextW func(string) float32 // string width at the who size
}

func noteCardH(c noteChrome, m noteMeasure) float32
func noteBandH(c noteChrome, m noteMeasure) float32
func notePillW(label string, m noteMeasure, colW float32) float32
func noteWhoRowW(inner float32, c noteChrome, m noteMeasure) float32
func noteFitWho(who string, avail float32, w func(string) float32) string
```

### The registry

`notes_chrome_conformance_test.go`. This is the whole unification mechanism, and it drives `notes_spacing_spec_test.go`'s existing per-surface `names` idiom from `reflect` instead of from a hand-written literal.

```go
type chromeSurface struct {
    label  string
    path   string
    spell  map[string]string // field -> the token proving this surface CONSUMES it
    banned map[string]string // field -> the local re-derivation it replaces
    weak   map[string]bool   // field -> no banned expression exists; see below
}

// pendingAdoption names, per field, the surfaces that have not yet adopted.
// SET EQUALITY, exactly as knownIncoherent is (notes_state_flow_test.go:220):
// a surface that adopts without being struck off fails as ADOPTED-BUT-PINNED,
// and one struck off that has not adopted fails as NOT ADOPTED. Every entry
// carries a reason and a docs/BACKLOG.md line.
var pendingAdoption = map[string][]string{ ... }

// chromePredicates covers what reflection cannot: shared FUNCTIONS. Hand-listed
// on purpose, and the weaker half of the mechanism — say so in the file.
var chromePredicates = []struct{ fn string; surfaces []string }{
    {"shouldCaptureScrollRestore(", []string{"reading_ios.go", "reading_macos.go", "reading_android.go"}},
}

func TestEverySurfaceConsumesEveryChromeField(t *testing.T)
```

A field with no `spell` entry for a surface and no `pendingAdoption` entry is a hard failure naming both. Adoption is proved by the **disappearance of the banned expression**, not only the appearance of the new token — that is the answer to "the registry can be satisfied cosmetically". Fields describing genuinely new behaviour have no banned expression; those are marked `weak` in the registry, and the marking itself is asserted so nobody can quietly widen the weak set.

### The twin diff

`notes_chrome_twins_test.go`, landing with the first native adoption. `NSTextStorage`/`NSLayoutManager`/`NSTextContainer` are the same classes under UIKit and AppKit, so the iOS and macOS chrome functions are twins by construction. Extract both bodies with `nativeFunctionSource` (the technique already in note_cycle_apple_contract_test.go:9), apply `kNote↔kMacNote`, `btIOS↔btMac`, `gNote↔gMacNote`, `UI↔NS`, normalise whitespace, require equality. Legitimate divergences — `btMacNoteTopGap`, the bottom-up placement, the trash image source — sit on a `divergentTwins` set asserted by set equality, so a divergence that goes away must be struck off. Defects 1 and 2 were *literally* two near-identical edits; this is the test that fails when only one is made.

---

## The first commit

**`notes_chrome.go` + `notes_chrome_conformance_test.go`. No native file touched. No ABI change. No cgo. One live defect fixed.**

What changes:

1. `notes_chrome.go` with the types above. `chapterNoteChrome` calls `appleStickerPush` for the six existing fields and composes `Present`, `Collapsed`, `Own`, `Verbs`, `Tail`, `Counts`, `Chevron`, `Bands`, `ExplicitArrival`, `Arrival`, `ShownAs` alongside. `appleStickerPush` keeps its signature and becomes three lines over it, so `androidStickerPush`, `styledStickerPush` and `assertStickerAgreesWithStore` are untouched by construction.
2. `receivedSetShownAs` and `stickerIsTheReceivedSet` move out of `reading_styled_note.go:814-891` and are re-typed from `styledNote` onto `noteChrome`. This is why X16 is a documented hole today: the answer is typed on a struct only one surface has.
3. `styledNote` (reading_styled_note.go:73) is deleted. `styledNoteFor` returns a `noteChrome`; `measureStyledNote` takes one; `g.hasTail` reads `c.hasTail()`; the verb if/else switches on `c.Verbs`. The styled pane becomes the reference consumer rather than a fourth implementation.
4. **The Own divergence closes.** `isOwnLiveNote` (notes_store.go:409) becomes the single spelling. All three natives already use it (reading_ios.go:3231, `macOwnFlag` at reading_macos.go:2570, reading_android.go:605); only reading_styled_note.go:104 asks the plan instead — `plan.HasOwn && state.NoteID != 0 && state.NoteID == plan.Own.Note.ID`. They disagree on the clamped-chapter / mirror-only arrival, where the store says the note is yours and the plan does not hold it, so the styled pane would offer a **delete bin on the reader's own note** where the natives offer ✕. Latent, shipping, unpinned.
5. `notes_state_flow_test.go`: `planSnap` gains `chrome noteChrome`; `takePlanSnap` fills it (it already calls `chapterNoteGroups` and `styledNoteFor`, so this is one line replacing two).
6. `docs/NOTES_STATE.md` gains N11–N13 and a struck line on "What it cannot reach. The panes." `docs/BACKLOG.md` gains the `pendingAdoption` cross-reference.

What proves it:

- **`TestStyledNoteGallery` (reading_styled_note_gallery_test.go:91) must come out byte-identical.** Twenty-eight snapshots, each asserting its geometry before writing its PNG, plus `reading_styled_note_test.go`, `reading_styled_pill_behaviour_test.go`, `reading_styled_multiband_test.go`. This is a stronger check than any assertion in the commit: if the shared value changed one pixel on the one surface that already draws all of it, the commit is wrong.
- `TestNoteChromeIsOneValueForEverySurface`: for every case, the value composed once equals what each of the four Go-side push sites composes — `assertStickerAgreesWithStore`'s existing byte-equality idea, widened from four scalars to the whole struct.
- `TestEverySurfaceConsumesEveryChromeField` with `pendingAdoption` seeded for `Tail`, `Verbs`, `Counts`, `Arrival`, `ExplicitArrival`, `Bands`.
- A named single-state regression test for the Own fix, in the shape `docs/NOTES_STATE.md` already uses for X8/X9, which must fail on the parent commit.
- The case table `noteChromeCases` is **generated** by walking the existing `notesWorld` cross-product and deduplicating on the resulting value. Hand-writing it would reintroduce exactly the failure `notes_state_flow_test.go`'s own header warns about — a test that checks the cases we thought of.

Mutation verification, per the standing rule that a check must be able to fail. Run each of these, confirm red, revert:

```
go build ./... && go vet ./...
go test -run 'NoteChrome|NotesStateSpace|VerbScreen|NoteSpacing|StyledNote' ./
BIBLETEXT_PANE_SNAPSHOT_DIR=/tmp/g1 go test -run TestStyledNoteGallery ./
scripts/view-test-gate.sh
```

1. Delete `"Tail"` from one surface's `pendingAdoption` — the conformance test must go red naming that surface and that field.
2. Add a `spell` entry for a field that surface has not adopted — must go red as ADOPTED-BUT-PINNED.
3. Flip `hasTail()` to `return true` — the enumeration must report cells.
4. Flip `Verbs` to always return `noteVerbsReceived` — same.
5. Change one `noteMetrics()` number by 1 — the gallery must go red.

No `2>/dev/null`, no `|| echo 0` anywhere in the gate.

Why it is safe alone: zero native source touched, zero wire-format change, `appleStickerPush`'s signature intact, and the only behaviour that moves is one predicate on one Go-testable surface, on a path the enumeration already reaches. If it is wrong, `git revert` restores a tree where nothing else depended on it.

---

## The order after that

Each lands alone. Each ABI commit follows two hard rules: **exactly one new field, appended at the end, never two at once** (the ABI is positional and already twenty-one arguments — `void bibleTextSetNote(const char*, const char*, int, int, int, int, + 15 doubles)`, reading_ios.go:885 — where two transposed ints of the same C type compile clean and fail on a device); and **the Go half plus the conformance fragment land first, deliberately red** for the platforms a development host cannot build. The test fails on the host you have, for the platform you do not. That directly inverts the failure this repo has already paid for.

**2. `shouldCaptureScrollRestore` — defect 2.** Pure Go, no ABI, no native source. Three call sites: reading_ios.go:3079, reading_macos.go:2470, reading_android.go:645. Android's `preserveTop` and `carryDataSwapArrival` arms move into the shared predicate, so the Apple panes get behaviour nobody ever ported back. `TestReadingPanesDoNotCaptureARestoreOverAnArrival` (note_cycle_apple_contract_test.go:117) tightens from "the condition mentions forceReposition" to "the condition calls `shouldCaptureScrollRestore(`" — strictly stronger and much shorter. Cheapest live-defect fix in the set; it goes second for that reason.

**3. `Present` / `Collapsed` / `Chevron`.** Natives touched, no ABI: they read pushed values instead of re-deriving. Deletes `btIOSNotePresent`/`btIOSNotePill` (reading_ios.go:852-853), `btMacNotePresent`/`btMacNotePill` (:1527-1528), `notePresent`/`notePillNow` (BtBridge.java:1168-1169). Normalises Android's `"  ›"` two-space chevron. Nothing user-visible should move; if it does, one of the four expressions was already wrong, which is the point.

**4. `Tail` — defect 3.** First ABI widening, one appended `int`, deliberately the smallest possible field so the wire mechanism is proved on something nobody can argue about. Each native sets `gNoteShapeExtra = gNoteTail ? kNoteTail : 0` **once**, in its `SetNote`, and every band formula reads that scalar. `banned` fragments forbid `btIOSNotePill() ?` / `btMacNotePill() ?` inside the install and layout functions. Lands on iOS, macOS and Android. **It changes no pixel on those three surfaces yet, and the plan was wrong to claim it would.** A census over the enumeration (TestDerivedChromeDecisionsAgreeWithTheirTuple) found 13 expanded cards and *none* of them anchorless: a note with no passage on this chapter has nothing to open and stands down to the pill, and a pill was already tail-free on every surface. What the step actually buys is that the three natives now ask the right question — resolved once, read as a scalar — so the anchorless card the bands step puts in front of them renders correctly the first time instead of growing a tail that points at verse 1. The tripwire in that census fails if the anchorless state ever becomes reachable through the single-card push, which is the signal to get a device picture of it. The twin diff lands here.

**5. `Verbs`.** One appended enum. Android's 🗑 emoji (BtBridge.java:1514) and its non-own-aware who-row width (`wlp.rightMargin = 2*dp(NOTE_BTN)`, :1451 — two verb slots reserved even for an own note, so its who line fits one button sooner than everyone's) fall out as adoption failures rather than as someone noticing.

**6. `Counts`.** Push the counts substring; each native locates it with a backwards search (`NSBackwardsSearch` / `lastIndexOf`) and accents that range. Deletes `styledWhoSplit`, `btIOSWhoCountRange`, `btMacWhoCountRange`; Android gets the split it never had. Guard the enabling fact with a host test: for every who line `chapterNoteChrome` can compose, the counts substring occurs exactly once, and it cannot be forged by a sender name because `sanitizeSenderName` maps U+00B7 to `-`.

**7. `Arrival` + `Lead` — defect 1.** One appended int plus one number joining `noteMetrics()`. Deletes `btIOSNoteSharesHighlightPara` / `btMacNoteSharesHighlightPara` and their `paragraphRangeForRange` calls, Android's verse-match and backward `'\n'` scan (BtBridge.java:1764-1769), and `cmd/websitegen/assets.go:1341`'s unconditional "prefer the note" in `rescrollToHighlight`. `highlightY` (reading_styled_pane.go:428) keeps its Y lookup and gates on the pushed class. Four predicates that were never the same rule — same-paragraph, same-paragraph, same-verse, and nothing at all — become one. This touches five surfaces' arrival scroll at once, which is the path a shared link exercises, so it lands after the ABI mechanism has been used twice. Verify per surface: the gallery, `SIMCTL_CHILD_BIBLETEXT_DEV_NOTES=s10next` on the simulator, a real `am start` link on the emulator, the web reader.

**8. `Bands` + `noteAction` — defect 4.** Turns each native's single scalar reservation into a keyed list and wires presses through `performNoteAction`. When it lands, `notesPillPerParagraph` (notes_plan.go:843) flips true and X16 comes out of `docs/NOTES_STATE.md`. Last on purpose; see risks.

**9. `cmd/websitegen`.** Emits the label, the tail rule, the verb set and the arrival class from the same Go functions at generate time. Its hardcoded byline and its minimize-suppresses-the-highlight semantics become a fifth surface's adoption or an explicit, reasoned divergence — not an untested difference.

Everything through step 7 is worth shipping whether or not step 8 ever happens.

---

## What the enumeration can watch afterwards

Today `notes_state_flow_test.go` walks 12,800 cells over `AppState` and the plan, plus — since N10 — two booleans scraped off a built styled pane. `docs/NOTES_STATE.md:1376` states the limit: it cannot reach the panes. It cannot see tail presence, the verb set, band count, band verses, or the scroll target, and two of the four surfaces do not compile on this host at all.

With `snap.chrome` and `snap.arrival` on `planSnap`, all of that becomes a model value asserted in every cell, with no cgo, no device, and no pane built:

- **N11 — a card has a tail iff it points at a passage.** `Tail == (Anchor > 0)`, and per band once `Bands` lands. Defect 3's axis, which nothing in the repo can currently see in any cell.
- **N12 — the verb set is a function of `Own` alone**, and `Own` agrees with `isOwnLiveNote` in every cell. This is the assertion that would have caught the clamped-chapter divergence by measurement instead of by reading.
- **N13 — `Present` and `Collapsed` are exactly their definitions**, asserted once instead of trusted four times, including the "who without text collapses" clause the styled pane currently omits.
- **N14 — an arrival targets a band only when that band's paragraph carries the highlighted verse.** Defect 1 as a value at every cell, replacing a source-level grep on two surfaces and nothing at all on the other two.
- **N9 / X16 per surface.** `ShownAs` moves off `styledNote`, so X16's 168 cells stop being asserted by inspection with `file:line` and become enumerated. X16 can then be closed by measurement.
- **Band count and band verses.** The walk has never seen a band. `len(Bands)` must equal `len(chapterNoteGroups(...))` in both states of `notesPillPerParagraph` — which is the gate that decides when the flag may flip.
- **The `partly` decisions, with a stub measurer.** Feed `noteMeasure{BodyH: 40, WhoSz: 11, Btn: 28, TextW: func(s string) float32 { return float32(len(s)) * 6 }}` and `noteBandH`, `noteCardH`, `notePillW` and `noteFitWho` are assertable at every cell with no device — including the degenerate who-line case where the four current implementations already disagree. Band height has never been enumerable at all.

What it still cannot watch, honestly: whether a native **obeys** what it was pushed. That stays `notes_spacing_spec_test.go`'s parsing, the registry's spell/banned pairs, and the twin diff — the right tools for the fourteen decisions that must stay in Objective-C and Java, and the wrong tools for the twelve that never needed to be there. And a semantic bug present in **both** Apple twins passes the twin diff by construction. That hole cannot be closed by any host-side test: it needs the platforms themselves.

---

## What stays duplicated, permanently

- **The four drawing engines.** `UIBezierPath`, `NSBezierPath`, `android.graphics.Path`, SVG. Four vector APIs. No cross-platform drawing layer is proposed and none should be.
- **The wrap and its height.** Fyne greedy wrap, `boundingRectWithSize`, `view.measure()`. This is *the* one platform measurement, and the whole design is arranged around it so it can stay where it is.
- **The reservation mechanism.** `paragraphSpacingBefore` ×2, `LineHeightSpan`, advance-added-to-y. Rule 2 — the band is an ADVANCE, never a line height — is enforced here and nothing in this plan goes near it. Note the residual honestly: a native that reserved the band as a line height would satisfy every field in `noteChrome`. Conformance on decisions is not conformance on mechanism; only the banned fragments see that, and imperfectly.
- **The band and card arithmetic.** After step 7 the band formula is still six transcriptions in three files, held by the parser. This is the deliberate cost of not pushing the spacing table, accepted with eyes open. What improves is that the *branch* comes out of the formula (step 4), so what is transcribed is a straight sum of named terms with no local decision in it.
- **`btMacNoteTopGap`** (reading_macos.go:1505) and macOS's bottom-up placement off the used rect. A measurement correction that keeps `GapBelow` — the pinned invariant — exact. Documented residual, stays.
- **Fonts, verb button sizes, hit-testing, z-order, the reconcile latches.** Correctly per-platform and already correctly excluded from the spec.
- **`noteMetrics()` as a parsed table.** The table owns numbers; `noteChrome` owns decisions. They are siblings on the right side of one line: compile-time constants get parsed, per-chapter data gets pushed. Extend the note in `notes_bubble.go` to say so, or the next reader will read the existing rejection as covering this too.

This is acceptable because none of it is a *decision*. Every item above is a mechanism, a measurement, or a constant, and none of the four defects lived in any of them.

---

## Risks, and how each is made reversible

**A positional twenty-one-argument C ABI.** Two transposed ints of the same type compile clean and fail on a device you cannot test here. *Reversible by:* one appended field per commit, never two, so a bad commit is a one-argument revert on three files; and by never reordering, so an older native reading a shorter arg list is a compile error rather than a silent misread.

**The window where the host is green and a device is red.** Steps 4-8 each need three native adoptions, two of which no single development host can compile. *Reversible by:* landing the Go half and the conformance fragment first so the host suite is loudly red naming the file and the constant, and by `pendingAdoption` — which lets a field be declared and consumed by the styled pane in a commit that touches no native source at all, then adopted one platform at a time, with the un-adopted ones dated and attributed rather than invisible. Watch CI after every push: a darwin host cannot see the linux and windows jobs.

**Step 7 changes arrival scroll on five surfaces at once**, and arrival is the path a shared link exercises — the feature's whole reason. *Reversible by:* it is one pushed int; reverting restores each surface's local predicate, all of which are deleted in one diff and restored in one. Land it after two ABI commits, and verify on each of the five before moving on.

**Step 8 is the riskiest change in this plan and everyone said so.** `gNoteReservedPara` is one NSRange that must be taken back when the sticker moves; `btIOSClearReservedPara` exists because a next-tap once left a phantom band at the previous note's verse. N ranges is that failure at N times the surface area. Android is worse: `NoteBandSpan` is a `LineHeightSpan`, paragraph-scoped with a reused `FontMetricsInt`, and two spans on one paragraph is produced *by construction* the moment the chapter-top group meets paragraph 0's own group. Both known traps there were only visible on the emulator. *Reversible by:* `notesPillPerParagraph` is already a runtime `var`, not a build constant, so the model reverts without a reinstall; and everything through step 7 is valuable with the flag off, which is why it is last.

**The registry can be satisfied cosmetically** — a surface can name a field in a dead branch. `notes_spacing_spec_test.go` records this limit about itself. *Mitigated by:* pairing every `spell` with the `banned` re-derivation it replaces, so adoption is proved by a disappearance. *Not mitigated for* genuinely new behaviour, where no banned expression exists; those are the `weak` cells, they are marked as such, the marking is asserted, and they lean on the twin diff and a named single-state test instead.

**Fragment brittleness.** A native rename fails the suite as a defect rather than as a rename. *Mitigated by:* failure text that names the surface, the field, the expected token and the reason, good enough that the next reader does not simply update the string. (`readNativeSource` already normalises CRLF; the repo has met this three times on line endings alone.)

**One dispatcher for destructive verbs.** `performNoteAction` centralises delete, so one bad `Verse` or `NoteID` reaches it through a path that used to be four separate closures. *Mitigated by:* the `NoteVersionID` identity discipline `notes_store.go` already records — an address, not a label — plus a delete-path test in the enumeration.

**Untrusted note text.** `Text` is the sender's words. It stays a length-counted `byte[]` / UTF-8 buffer set as TEXT on a label — never `NewStringUTF` (a note with emoji is four-byte UTF-8 and aborts under CheckJNI), never a Spannable-bearing or markup path. The span mechanism this design adds is app chrome only and must never be reachable from note text. Assert it in the registry as a banned pairing.

**Runtime cost.** Composition is the same work `appleStickerPush` already does plus a small slice; `refreshNoteInPlace` (notes_refresh_apple.go) already runs `buildChapterPlan` and `foldFingerprint`. The enumeration grows by one composition per cell, well inside the current "0.6ms a pane, under two seconds for the space" budget.

**Explicitly not in this plan:** any synchronous C→Go or JNI→Go geometry query. A measurement callback on the main thread inside a layout pass, in a codebase that already carries a reconcile latch for exactly this timing class, is a deadlock waiting for a slow device. Nothing measured flows into Go here, which is what makes that refusal free rather than a compromise.