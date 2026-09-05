# Backlog

Deferred work, one entry per item. An entry carries enough scope to be picked
up cold; delete it when the work lands.

## Phone landscape reading — shipped on both phone platforms

Landscape on a phone was the layout nothing was designed for. What shipped
before this work (`compactNavRail` in ui_mobile.go, `mobileRailWanted` in
layout.go, docs/IPAD.md): iPhone keeps the bottom tab bar in every orientation; Android
phones move the destinations to the left rail in landscape because the
fixed-height header, history strip, chapter toolbar and bottom bar can consume
the whole short edge; iPad and the desktop use the rail in landscape. So an
iPhone turned sideways gives the reading pane the least height of any surface,
and the chrome that costs it — header, history, chapter toolbar, bar — is the
portrait design carried over unchanged.

Direction (2026-09-04), iOS first: a phone turned to landscape reads like the
iPad. Rotating to landscape on the Read tab enters the distraction-free
presentation — the reading pane alone, no restore button because rotation is
the way out — and the text takes the iPad typography (the centred reporter
measure, 1.3 leading, first-line indents, no paragraph gaps: docs/IPAD.md);
rotating back restores the portrait layout and the reader's own full-screen
choice. Tablets are distinguished by construction (deviceIsTablet: the UIKit
idiom on iOS, the small-side-600dp rule on Android) and keep their rail. Books
and Search keep their landscape layout too — the bar on iPhone, the rail on
Android — because the mode applies to the Read tab only.

What the reader gives up in landscape, by design: every control — chapter
arrows, the picker, Go-to, search, narration, the "‹ Results" trail — is
reached by rotating back to portrait, the way out the mode is built around.
The label row keeps the reference and the selection menu still works, and it
carries the chapter arrows (fullScreenExitRow, shared
by both native panes): running out of chapter is the one thing that happens
while READING, and rotating out and back is a poor way to turn a page. The
reader's own full-screen keeps its restore button and only that.
On Android, tablet identity follows the live window, so a tablet pane split or
floated narrower than 600dp reads as a phone and takes the presentation, which
is the height problem the mode exists for.

Status (2026-09-04): ON BY DEFAULT for phones on iOS and Android
(phone_landscape.go), as two preferences a switch can turn off — the dev
Links tab carries both; a user-facing Settings row was considered and
declined. BOTH halves now ship on both platforms. The Android reporter page
(reporter_android.go) reaches the reader by three routes, because this dialect
has no stylesheet: the first-line indent is markup (android_chapter_html.go),
the paragraph gap is closed by importing in COMPACT rather than LEGACY mode
(BtBridge.setHtml — the gap is the importer's blank line between blocks, and
the markup keeps its <p> blocks so a note band still knows where a paragraph
begins), and the measure is pushed as a width the bridge centres against the
live view (androidReadingMeasureDp, BtBridge.applyReadingPadding, the shape of
the iOS textContainerInset). The leading is deliberately NOT changed: the
Apple dialect writes 2.0 and 1.3, but the UIKit importer honours neither, and
the drawn pitch measured 83px portrait against 82px landscape on the iPhone 16
Pro simulator — so following the CSS would have opened a 54% gap between panes
meant to match. The reporter flag
folds into the body fingerprint so a rotation re-imports under the new
grammar; the layout watcher carries the presentation as its own term and, on
iOS, captures the reading anchor before the rotation's frame lands (Android's
bridge re-places its own view by scroll fraction). Simulator-verified on the
iPhone 16 Pro: presentation, typography, the reading position across
rotation in both directions at the chapter top and mid-chapter, chosen
full-screen round-tripping with its restore button, a live selection
dropping cleanly, Books and Search keeping their layout in landscape.
Emulator-verified on Android 13 (API 33) and Android 15 (API 35): the Read
tab's landscape presentation with the position kept across both rotations, a
link's verse wash surviving them, and the Books tab keeping its rail.
`BIBLETEXT_DEV_PHONE_LANDSCAPE=on|typo|off` seeds a scripted simulator run.

## Android landscape: the status-bar strip is black in the light theme — DONE

On the API 33 emulator (720×1600) in landscape, the strip down the short edge
was black behind the light paper. It was not the status bar and not the
window: it was the WINDOW MANAGER'S CUTOUT LETTERBOX. In the default cutout
mode a window may sit under a display cutout only when that edge is at the
top (portrait); in landscape the system letterboxes the window away from the
cutout's whole short edge and paints the letterbox itself, in black. Dark
paper hid it; light paper showed it, and the landscape reading mode made it
the first thing seen.

The first attempt painted the activity window's background and system bars
in the paper colour from the style push, and changed nothing — this is a
NativeActivity window (takeSurface), so the view hierarchy never draws and a
window background or bar colour set on it is never rendered. The fix is
BtBridge.extendIntoTheCutout, from init: LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
on API 28 to 34, so the cutout arrives as an inset the canvas already honours
and the strip becomes paper (from Android 15 the platform forces "always" on
every window of an app targeting 35 or later — this app targets 36 — which is
why the API 35 emulator never showed it). The same pass found the reporter
column centred in the overlay rather than on the screen — the canvas keeps
the reading object out of the display cutout, so the overlay sits 42px from
one edge and 178px from the other and its centre is 67px off — and
applyReadingPadding now centres the column on the window, as the iPhone does. It centres against the overlay's FRAME
(frameX plus the canvas inset, the sum applyFrame positions the window by),
not the view's on-screen location: on a rotation the first layout runs before
the overlay window has moved, a location read then said x=0, and the column
landed ~130px off once the window moved — and only with the cutout on the
left, which is what made it look intermittent. applyFrame re-centres after
every move.

Emulator note: the rotation helper used to write `accelerometer_rotation 0` + `user_rotation`, which
switches auto-rotate OFF and leaves the emulator ignoring its own toolbar
rotate button afterwards. Rotate emulators with `adb emu rotate` (what the
button does) and never write those settings. Separately, the API 33 image
has mAllowAllRotations=false, so the 180° posture in the button's four-step
cycle is refused and that one click looks dead; the next click works.


## Android split-screen: the reading overlay doubled the task origin — DONE

applyFrame places the overlay Dialog at frame + windowContentOrigin, and
windowContentOrigin added the decor's ON-SCREEN location — but the window
manager resolves a dialog's x/y against the TASK bounds, so in a task that
does not start at the display origin (the bottom or right pane of
split-screen, a freeform window) the origin was counted twice and the overlay
was pinned below the tab bar with the reading area empty. windowContentOrigin
now subtracts the window metrics' bounds origin on API 30+, a no-op for a
full-screen task. Reproduced and verified on the API 35 emulator in a
freeform window (`settings put global enable_freeform_support 1` and
`force_resizable_activities 1`, then `am start … --windowingMode 5`): the
overlay moved from (297,1426) to (21,730) against a task at (276,696), and
sat over the reading rect. Below API 30 the task origin is not cheaply
readable, so a split pane there keeps the old placement.

## macOS: a launch restore that never died, and a note card placed before the text settled — DONE

Two defects in the NSTextView pane, both measured on the desktop app on
2026-09-05 and fixed in reading_macos.go.

The pane had no user-scroll hook, where iOS has scrollViewDidScroll. So the
Go side's `state.restore` was never consumed on macOS: after a launch (or a
history tap) every same-chapter rebuild — an appearance flip, a text-size
change, a note verb on the slow path — re-armed the launch anchor and moved
the reader back to it, however far they had read (measured: verse 37 back to
verse 1 on an appearance flip). Natively, the arm was disarmed only by the
first changed frame, which re-applied a superseded width and then let a later
resize re-apply the stale anchor. Now the clip view's bounds observer calls
`btMacUserScrolled` for any movement that is not one of the pane's own — the
wheel, the trackpad, the scroller, keyboard paging, a drag-select autoscroll,
an accessibility scrollbar write — and narration's follow-scroll calls it
too; every programmatic scroll (the restore, a highlight, an import's clamp,
a frame change) runs under a latch. It disarms natively at once and calls
`bibleTextReadingScrolled` once the motion settles, the iOS decision in
AppKit's vocabulary, and the arm survives frame changes as it does on iOS and
the Fyne pane. A `scrollWheel:` override was tried first and rejected: it saw
only the wheel and the trackpad, and on the document view it switches off
AppKit's responsive scrolling.

The note card sat a line above its passage with the band's air under its
tail on any launch into a saved position with an open note. `BT_NOTE_GEOM`
showed the card placed exactly where the arithmetic put it and the passage
31pt lower a second later: the paragraph's fragment had answered at the
previous line's y at placement time. The card is a subview placed FROM the
layout and nothing moved it when the text moved. `HBLayoutWatcher`, the
layout manager's delegate, re-places the card and the pills after every
completed layout pass — placement, not a refresh, so it converges — and the
card now sits 10pt above its passage. Pinned by
`reading_native_scroll_contract_test.go`; the pane itself cannot run on the
host, so the measurements are the evidence.

## Bible version states: transition diagram + comprehensive tests — DONE

Both halves are in docs/VERSION_STATES.md: the storage diagram (M1–M3) and,
since 2026-09-05, a second diagram for M4, the launch space and the arrivals
layer drawn as the enumerations drive them, with a table of every space's
own count as its test logs it (15, 10, 160 + 310 journeys, 8, 16, and 780
journeys over 2930 steps). The document's closing section had still said
M4–M7 were unenumerated; it now says what is, and what the enumerations do
not claim. The record of the work follows.


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

1. ~~**Say the true scope.**~~ DONE. The count now reads "K of N in this
   chapter" unconditionally, rather than only where the placements differ.
   A conditional wording would have kept "on this passage" for notes that
   genuinely share a range, which is more precise when true; it was rejected
   because the string would then change under the reader with no way to tell
   which rule was in force, while the rotation it describes is chapter-wide in
   every case. The replacement is the same 15 characters, so the iOS and macOS
   WHO-line fitting is unaffected.

   One inaccuracy survives it, and is separate: N counts RECEIVED notes only,
   so a chapter holding five received and three of the reader's own still says
   "of 5". "Passage" was vague enough to hide that; "chapter" is checkable.
   What keeps it tolerable is that the byline names whose notes are counted,
   and an own note never displays a count at all — see the next item.
2. **Mark the other noted passages in the text**, so the pill over one
   paragraph no longer implies it is the only one. This is the discovery
   problem, and it is what the banner already gives the non-native platforms.
   ADDRESSED EVERYWHERE: the per-paragraph pills
   (`notesPillPerParagraph`) draw one pill per noted paragraph with that
   paragraph's own count on the styled pane, and iOS, macOS and Android draw
   the same groups through their band-spec pushes (`bibleTextSetNoteBands`
   and its twins). The gate defaults ON now — the shipped collapsed state
   says where the chapter's notes are on every surface.
3. **Lift the cap.** `planOpenLimit`'s own comment invites it: "TO LIFT THE
   CAP: raise this number (or drop the counter). Nothing else changes — not the
   store, not drawnNote, not the fingerprint." Largest change, and worth
   weighing against a page carrying three open bubbles at once.

(2) was watched on the API 35 emulator with four received notes on four
John 3 paragraphs (8d-And): four chips in their own bands, the single sticker
stood down, and a pressed pill opened its own paragraph's group. What remains
open here is (3) alone, and it is a choice rather than a defect.

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

## Per-paragraph note pills: notes that belong to no paragraph — DONE

Decided and built as the first option below: a chapter-top group carries
them. `groupNotesByParagraph` sends a whole-chapter note (anchored at verse 0)
to the chapter-top group, and `chapterNoteGroups` puts the book's unplaced
notes on that same group — creating it when nothing else needs it — with the
"· N not shown" suffix the single pill always carried. The collision with a
first-paragraph pill at the same band verse is resolved by matching bands by
KEY rather than by verse (8a). Pinned by
`TestTheTopGroupCarriesTheNotesThatBelongToNoParagraph` and
`TestTheTopPillDisclosesUnplacedNotes`, so the counts sum to the chapter's
total again. The original statement of the gap follows for the record.


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

## Opening your own note hides every trace of everyone else's

Not introduced by the pills — the shipped single-sticker path does it too, on
all five platforms.

The pills and the sticker are both the collapsed state, so opening any note
stands the pills down. For a RECEIVED note that costs nothing: the who line
becomes "Note from Friend · K of N in this chapter", which still says the
others exist and still offers the count control that rotates to them. For the
reader's OWN note it costs everything: an own note is deliberately not a member
of N and has no next-tap, so its who line reads "Note from you" alone. Measured
on a chapter with three received notes in two paragraphs plus one own note:

    a received note open  -> pills 0, who "Note from Friend · 3 of 3 in this chapter"
    your own note open    -> pills 0, who "Note from you"

So the one case where the reader loses all evidence of their friends' notes is
the case where they opened something of their own — and nothing on the page
tells them to close it to get that evidence back.

A principled fix exists: stand the pills down only for an open RECEIVED note,
whose who line then carries the count. An own note is not in the pills' set at
all (they count received notes), so leaving them up alongside it double-counts
nothing — which is the reason the one-collapsed-state-at-a-time rule exists.
That still leaves the three native surfaces, which have no pills to leave up;
for them the answer would have to be a count in the own note's who line, which
contradicts "displaying an own note must not change N" unless it is written as
a separate clause rather than folded into N.

## Per-paragraph pills on every surface — DONE

The port and the unification were one job, and both are complete: see the
status block at the top of
[docs/NOTE_CHROME_UNIFICATION.md](NOTE_CHROME_UNIFICATION.md), which records
all nine steps landed. iOS, macOS and Android draw the paragraph groups
through their band-spec pushes (`bibleTextSetNoteBands` and its twins, 8d),
`notesPillPerParagraph` defaults on (8f), and the web reader takes its chrome
from the same functions (step 9). X16 in docs/NOTES_STATE.md — an open own
note leaving the received set represented nowhere on the native surfaces — is
closed by it. The dev toggle's off value remains the one-line reversion.

## Deferred-UI timers under the test driver (audited 2026-09-02, no live races)

`verse_of_day.go`'s re-measure timer is the precedent: under Fyne's TEST
driver `fyne.Do` runs its closure on the timer's own goroutine — there is no
UI thread to marshal to — so a pending `time.AfterFunc` races the test
goroutine (including a `t.Cleanup` `Hide`), and even a `Visible()` read is a
data race. The shipped fix stands the timer down under `testing.Testing()`
because the tests drive that re-measure synchronously.

Every other `AfterFunc`+`fyne.Do` site was audited against the same hazard.
None races today — a full `-race` pass of the suite is clean — but each is
safe for a DIFFERENT reason, and the first test that changes that reason
must bring the fix with it:

- `goto.go` (60ms inset scroll; 200ms self-rearming `watchDismiss`) and
  `share_note_ui.go` (50ms slot push; 150ms self-rearming watch): the arms
  live on the `IsMobile()` / native-entry branches, which no host test
  compiles into. If a mobile-tagged test target ever opens these, gate the
  arms with `testing.Testing()` AND add a direct synchronous call for the
  watch's close-out work — both watches are functional (tap-outside close,
  overlay restore), not cosmetic, so a bare gate would orphan real behavior.
- `audio_menu.go` (150ms self-rearming watch): no test constructs the
  source menu (the card tests stub `onSrc` deliberately). Same rule as the
  two above: gate plus a synchronous driver, never a bare gate.
- `search.go:~25` (60ms scroll restore) and `notes_browse.go:~783` (16ms
  scroll restore): armed only when a remembered scroll offset is positive,
  which no test produces. The natural regression test for either restore
  (build, scroll, rebuild, assert offset) arms the timer and reproduces the
  verse-of-day race verbatim — write the `testing.Testing()` stand-down in
  the same change and drive the restore synchronously in the test.
- `ai_panel.go:~305` (40ms re-fit): armed only by a delivered answer, which
  every current test's stub avoids. A test that lets `setResult` run needs
  the stand-down (the closure measures fonts with no `Visible()` guard).
- `reading.go:~209` (`flashIcon`): the one arming test passes `time.Hour`
  and drives the restore synchronously — a deliberate crutch. A test that
  taps a real copy button (1200ms flash) needs the arm gated.

The audit's rule, kept: a gate lands only with a demonstrated race (a
`-race -count=30` loop that fails before and is clean after) — no
prophylactic gates over dead code.


## `fyne package` bumps the desktop Build ledger after every package

`fyne package` rewrites `cmd/desktop/FyneApp.toml`'s `Build` after a
successful package. Two consequences, both seen on the 1.2.5 cut:
`release-mac-store.sh` leaves an uncommitted 46→47 in the working tree (harmless
if discarded; it produced the earlier 44→46 drift when it was committed
by accident), and `release.yml` packages Apple Silicon then Intel in one
checkout, so the second zip stamps CFBundleVersion one higher than the first
(1.2.5: 46 and 47 for the same commit). Fix: restore the ledger between the
two `fyne package` runs in `release.yml` (or pass the build explicitly), and
have `release-mac-store.sh` restore `FyneApp.toml` on exit as
`build-android.sh` already does. Until then the next desktop Store build is
48, not 47.


## Android 13/14: the reading text cannot be justified while it stays selectable

The native reading pane is a selectable `TextView`, which Android lays out
with a `DynamicLayout`. Before Android 15 that layout never hands the
justification mode to the `Layout` that draws: the line breaker still breaks
in justified mode — which lets a line exceed the width by the amount its
spaces could shrink — but nothing shrinks them, so on API 33 every such line
spilled past the right edge and the rest read ragged (stock emulator, measured:
layout width == view width, line width > both; the android13/14-release
`DynamicLayout` constructors omit the hand-off that android15-release makes).
The app now justifies only on API 35+, and older releases read ragged but
whole. Non-selectable text (`StaticLayout`) justifies on every release, but
selection is the native feature the pane exists for.

Ways back, if ragged text on Android 13/14 ever matters enough: a custom
`TextView` that draws its own justified lines from a `StaticLayout` while
keeping a selectable `DynamicLayout` for hit-testing (two layouts, one text);
or reflection on the protected `Layout.setJustificationMode`, which the
hidden-API policy may refuse. Neither is worth it for a rendering that
Android 15 already gets right.

## Tag 1.2.6 so `go install` and `go run …@latest` resolve the module — DONE

Cut as an ANNOTATED tag (the kind docs/VERSIONING.md asked the next cut to
settle on). Deliberately a tag and NOTHING else: no release assets, no store
submission, and the two FyneApp.toml ledgers deliberately left at 1.2.5.

The ledgers stay because in this repository the ledger version IS the prepared
store submission — check-release-identity.py holds it against
appstore/review-notes.txt, and the review-notes tests hold that against the
writer's version pin and a What's New file named for it. Moving the ledgers to
1.2.6 would have prepared a submission nobody is sending.

The cost, stated so a bug report stays readable: a binary built from the
v1.2.6 tag reports 1.2.5 in Settings, because no binary channel carries 1.2.6.
A source build is already distinguishable — it carries no bundled NKJV key
(the entry below) — and the next store release takes 1.2.7 with both ledgers
and both review-notes files moving together.

Every tag up to v1.2.5 carries the old bare `module bibletext` line, which Go's
module resolution rejects: `go install github.com/cubancorona/bibletext/cmd/desktop@latest`
and `go run …@latest` resolve `@latest` to the newest tag and stop at "module
declares its path as: bibletext but was required as:
github.com/cubancorona/bibletext". main declares the repository path
(`module_path_test.go` holds it) and `…@main` already builds; those routes
reach `@latest` when v1.2.6 is tagged.

The directory entry (apps.fyne.io/apps/uk.co.bibletext/) prints
`fyne install github.com/cubancorona/bibletext/cmd/desktop@latest`, which takes
a different route — `git ls-remote` for the newest v-tag, a depth-1 clone of
that tag, then `go build` inside the clone — so the module line never enters
into it and that command works today on v1.2.5. A source build by either
route carries no release ldflags, so it has no bundled NKJV key: the reader
adds their own API.Bible key in Settings for that translation.
