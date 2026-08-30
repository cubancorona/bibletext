# Backlog

Deferred work, one entry per item. An entry carries enough scope to be picked
up cold; delete it when the work lands.

## Bible version states: transition diagram + comprehensive tests

**Storage space DONE 2026-08-28** — `docs/VERSION_STATES.md` models the machine
and `version_state_flow_test.go` enumerates it (15 cells, every one reached,
zero incoherent states standing). The enumeration found and closed V1 (an
unusable current-epoch cache served the previous epoch silently, with the
refresh switched off and the picker mute) and its root cause V2 (a cache write
renamed without an fsync).

Two spaces remain to enumerate, both scouted with the cells already chosen:

- **The launch space** — storage shape x saved-reading x seed-usability x
  fetch outcome, driven through `loadStartupBible`, which takes its three
  loaders as parameters and is therefore already injectable.
- **The download space** — `fullPending` x `seedOnly` x `fullDownloading` x
  backoff delay, across the apply / retry / foreground / picker-open events,
  built on an `AppState` literal.

### Reported defects — status, severity, and the order to take them

Each was raised while scouting the runtime and cache lanes. **PROBED** means the
scout executed it against a copy of the repo; **TRACED** means it is a code walk
with exact paths but no execution. None is claimed in docs/VERSION_STATES.md,
because that document only names what the enumeration has driven. Confirm each
with a cell as its space is enumerated.

| # | Defect | Evidence | Costs the reader | Fix |
|---|---|---|---|---|
| ~~D1~~ | ~~`purgeUnavailableLicensedCaches` deletes on an answer the app could not verify~~ | **CONFIRMED by the M2 enumeration and FIXED 2026-08-28** | — | done |
| ~~D2~~ | ~~Superseded epochs of a licensed version are never age-checked and never purged~~ | **FIXED 2026-08-28** | — | done |
| ~~D3~~ | ~~A non-default translation served from a superseded epoch is silently stale~~ | **FIXED 2026-08-28** | — | done |
| ~~D4~~ | ~~`seedOnly` is not cleared when the download lands while the reader is away~~ | **CONFIRMED by the M3 trajectory walk and FIXED 2026-08-28** | — | done |
| ~~D5~~ | ~~The picker's manual retry makes the waiting notice unreachable~~ | **CONFIRMED by a reachability assertion and FIXED 2026-08-28** | — | done |
| ~~D6~~ | ~~A successful fetch that cannot be persisted is discarded entirely~~ | **FIXED 2026-08-28** | — | done |
| ~~D7~~ | ~~Registry-resolved vs value-resolved cache paths can collide~~ | **GUARDED 2026-08-28** | — | done |
| ~~D8~~ | ~~The cache-only read's miss branches disagree about the mode~~ | **FIXED 2026-08-28** | — | done |

**All eight are closed** (2026-08-28), each confirmed against real code and
fixed with a test that fails without the fix.

**M5 x M6 x M7 — launch, reading position and canon shape — enumerated
2026-08-29**, together rather than separately, because the failure they were
built for lives in their intersection. Two more defects, both **durable**
(they rewrite the only record of something the reader cannot re-derive):

| | Defect | Status |
|---|---|---|
| ~~D9~~ | ~~A translation merely unselectable this launch has the reader's choice overwritten by the fallback, permanently~~ | **FIXED 2026-08-29** |
| ~~D10~~ | ~~The reader is shown a different translation than they chose, and nothing says so~~ | **FIXED 2026-08-29** |

The history-erasure invariant itself held in all sixteen cells, including the
73-book-trail-meets-66-book-fallback shape of the original incident.

**M4 — active selection — enumerated 2026-08-29.** One more defect:

| | Defect | Status |
|---|---|---|
| ~~D11~~ | ~~A stale version's notice is retired by the disk while its previous decode is still on screen~~ | **FIXED 2026-08-29** |

Not reachable today (nothing writes a non-default version's current epoch
while its previous decode is in memory), but one obvious feature away, and the
fix also closes a live liveness hole: a stale non-default translation
previously had no way to stop being stale within a session.

**The arrivals layer — enumerated 2026-08-29, as JOURNEYS** (780 to depth
four), because an arrival is a promise kept or broken over time and every way
it breaks is a sequence. One more defect:

| | Defect | Status |
|---|---|---|
| ~~D12~~ | ~~A link whose translation failed to load keeps its park; a later unrelated switch to that translation honours the dead link and moves the reader~~ | **FIXED 2026-08-29** |

**A second, model-free scouting pass over the same four arrival surfaces**
(each candidate refuted or confirmed by two adversarial verifiers on separate
lenses) found four more, every one on a coupling the map had not drawn:

| | Defect | Severity | Status |
|---|---|---|---|
| ~~D13~~ | ~~A link switching translations spends the reader's remembered fallback choice~~ | durable | **FIXED 2026-08-29** |
| ~~D16~~ | ~~A wider canon's trail is offered dead after a switch and deleted at the next launch~~ | durable | **FIXED 2026-08-29** |
| ~~D15~~ | ~~Search results survive a switch: old wording under a new name; a tap writes a dead reference~~ | session | **FIXED 2026-08-29** |
| ~~D14~~ | ~~A link displaced by another translation's load is dropped with nothing said~~ | session | **FIXED 2026-08-29** |

**All seven machines and the arrivals layer are enumerated, and every defect
found is closed.** Two lessons worth keeping. Third confirmation that
cross-products are blind to flow (the arrivals defect is invisible to every
cross-product in the suite; the journeys found it by four routes). And: a
harness can only look where its model points. The journeys found D12 and went
quiet; re-reading the same ground with no model in hand found four more,
including the original incident arriving through a version switch rather than
a launch.

## NKJV Psalm superscriptions

The NKJV prints the Psalm titles too, but its API.Bible feed delivers them as
`d` (descriptive title) paragraphs, which decodeAPIBiblePassage currently
skips via apiBibleSkipPara. Rendering them means capturing `d` content into
BibleData.Superscriptions during the passages walk (anchoring any note
markers the way the helloao branch does), bumping the nkjv cacheEpoch, and
nothing else — the renderers and the section already handle titles for every
version. Worth batching with the next NKJV decode change.

## Show a note you just sent, the way opening one from the browser does

Sending a note stores it and draws nothing. `share.go` says so where it saves
the record — "never drawn in the text, and visible in the notes browser — that
visibility is deliberate" — and `notes_plan.go` enforces it: a `noteKindMine`
record joins the plan only while `noteFocus` names it, so the browser can show
one and the send path cannot.

The proposal is to let sending focus the note it just stored, exactly as
`openNoteFromBrowser` does: `state.focusNote(stored.ID)` then
`applyNoteForCurrentChapter(state)` after `saveMyNote` returns. That inherits
the transient behaviour already in place — `resetNoteFocus` runs on every
chapter arrival, so the note goes away on navigating away, with no new lifetime
rule to define and nothing persisted that was not persisted before.

This REVERSES a stated decision rather than fixing a defect, which is why it is
written down instead of done. What argues for it: sending is the one moment a
reader has no confirmation that their words were kept, and the browser is
several taps away.

The report that raised it was a misreading worth recording, because the app
invited it. A highlight was still standing after a send with no note beside it,
which read as a note that had lost its text; it was a search mark, from arriving
at the passage through Results. `hlOrigin` (mark.go) records provenance but does
not change the tint, so a note's mark and a search mark are indistinguishable to
a reader. Whether or not the change above is made, that ambiguity is its own
item: a reader cannot tell why a verse is lit.

## One pill, several noted paragraphs: what the count says and where it points

With more than one note on a chapter, the native reading pane draws ONE
anchored sticker — `planOpenLimit` is 1 — over the focused note's verse, and
puts the rest behind a counter that rotates focus and scrolls to the next one
(`advanceNoteFocus`). Two things about that are worth revisiting.

**The count says "passage" and means "chapter."** `placed` is
`len(plan.Notes)` — every placed received note in the CHAPTER — so a sticker
sitting over one paragraph can read "1 of 3 on this passage" while notes 2 and
3 are on paragraphs elsewhere. A reader looking at a pill over one paragraph
reads "this passage" as the paragraph under it, and the sentence then claims
three notes on a verse range that has one. The wording is only accurate in the
case it was written for, several notes sharing one range (the S10 scenario in
`dev_links_on.go`).

**Nothing marks the paragraphs the other notes are on.** The counter is the
only route to them, so a reader who does not tap it has no way to learn they
exist. Note that the platforms diverge here: the Fyne banner
(`notes_banner.go`, Windows and Linux) draws a CHIP PER NOTE, so those readers
do see the whole set while the native ones see one.

Three ways to take it, cheapest first, not exclusive:

1. **Say the true scope.** "K of N in this chapter" when the placements
   differ, keeping "on this passage" for notes that genuinely share a range.
   Small, and it stops the app asserting something untrue — worth doing
   whatever else happens.
2. **Mark the other noted passages in the text**, so the pill over one
   paragraph no longer implies it is the only one. This is the discovery
   problem, and it is what the banner already gives the non-native platforms.
3. **Lift the cap.** `planOpenLimit`'s own comment invites it: "TO LIFT THE
   CAP: raise this number (or drop the counter). Nothing else changes — not the
   store, not drawnNote, not the fingerprint." Largest change, and worth
   weighing against a page carrying three open bubbles at once.

Read from the code, not yet watched on a device with three notes on three
paragraphs; do that first, since it will show whether (2) alone is enough.

## Candidate: a neutral graphite wash for the dark-mode highlight

The dark highlight is `#3A326F`, a violet. Held against seven alternatives on a
real screenshot, graphite `#2C343E` was the one worth keeping in mind: the
violet carries enough chroma to read like a system text selection, where a
neutral band reads more like a mark someone made. Light mode is not in
question — `#FFE08A` amber stays, and it is the one hue that says "highlighted"
while leaving red letters red.

Measured, so the trade is not a matter of taste alone:

| | vs the #191715 ground | red letters on it | body text on it |
|---|---|---|---|
| `#3A326F` violet (now) | 1.59:1 | 3.77:1 | 9.88:1 |
| `#2C343E` graphite | **1.42:1** | **4.22:1** | **11.05:1** |

So graphite is easier to read text ON and harder to spot AT A GLANCE. The
second half is the risk: 1.42:1 against a near-black page is quiet in daylight.

**What blocks a straight swap.** `TestMultiNoteWashKeepsScriptureLegible` pins a
hue relationship, not just a contrast floor: in dark the primary wash must be
violet (`B > R > G`) and the multi-note wash slate-blue (`B > G > R`), so
"several notes here" cannot be misread as "one strong note". Graphite is
`B > G > R` — it lands in the multi wash's own family and the pair collapses.
The test is right to refuse it.

**The way through, if this is ever taken up.** Give the violet the multi-note
job: primary graphite (neutral), multi `#3A326F` (chromatic). A neutral against
a chromatic separates more sharply than the current violet-against-slate pair,
and it keeps a colour worth keeping. `HighlightMulti` is unreachable today
(`tintMulti` is not wired), so that half costs nothing visually until it is.

Three approvals would move with it, and each is a deliberate gate rather than a
formality: `TestApprovedHighlightTokensStayPinned` and
`TestMultiNoteWashKeepsScriptureLegible` in `theme_contrast_test.go`, and
`TestWebReaderPaletteValues` in `cmd/websitegen` — the web reader's dark
highlight is meant to match the app's, and that parity is intended, so any
change here changes the web reader too.

## Per-paragraph note pills: notes that belong to no paragraph

Gated behind `notesPillPerParagraph` (off by default), so nothing ships blind.

The pills are per paragraph, and two kinds of note belong to no paragraph. Both
are counted by the chapter-scope single pill and both fall out of the reading
view once the pills take over:

- **unplaced notes** — filed on this book, with no home in the translation
  being read. The single pill discloses them (`Notes · 2 · 1 not shown`); no
  pill mentions them.
- **chapter-level notes** — anchored at `VerseLo 0`, on the whole chapter.
  They sit in `plan.Notes` with `Here = [{c 0 0}]`, so `noteAnchorVerse`
  returns 0 and `groupNotesByParagraph` skips them. With two ordinary notes
  and one chapter-level note, the pills account for 2 of 3.

One decision covers both: where does a paragraph-less note's pill go? Options:

- a chapter-top pill carrying them, reusing `stickerUnplacedOnlyWho` for the
  unplaced sentence — principled (chapter-scope things go to the top, the same
  reasoning that puts the collapsed single pill there), but it can collide with
  a first-paragraph pill at the same band verse, and the placement loop keeps
  only the first match per band
- a suffix on the first or last pill — misattributes a chapter-scope fact to
  one paragraph
- suppressing the pills whenever either kind is present — preserves today's
  disclosure exactly but silently disables the feature

Until it is decided the pills' counts do not sum to the chapter's note total,
which is the honesty property the single pill has always had.
`TestTheSinglePillStillDisclosesUnplacedNotes` pins the shipped gate-off
guarantee so the ungated path cannot regress meanwhile.
