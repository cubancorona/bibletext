# Backlog

Deferred work, one entry per item, and the record of the work that has closed.
An entry carries enough scope to be picked up cold. The rule here was to delete
an entry when the work landed; it is kept instead — marked DONE, or FIXED with
the date — and says what shipped and why. Closed entries earn their place: this
is the file to read before re-investigating a defect that may already be fixed,
and a fix's reasoning is the expensive half to reconstruct.

Work built for the next major version is not listed here. It is on `main`
behind the `next` build tag, and [NEXT.md](NEXT.md) is its register: each
piece, what readers will see, its tests and its open decisions. The NKJV's
epoch-8 refresh with its offline bridge and the Gospel parallels' "same
saying, another occasion" are there, not here. The Greek Esther mapped verse
for verse left the switch for 1.2.23, and NEXT.md keeps its record too
(*Work graduated for 1.2.23*).

## The release pipeline: what still waits on something it need not

Planned for: stays, a standing entry — see docs/ROADMAP.md. Stage 10 names
the arm64 snap rerun from 1.2.20; the reason Play's publishing mode is a
hand check (below) goes in a later minor release.

A standing entry, not a one-off: every release is the moment to ask what
sat idle waiting on a step it did not depend on, in the release itself and
in the testing and deployment around it. Two reorderings landed on
29 September 2026 (`docs/RELEASING.md`): the Windows Store package is now
built at the pushed commit alongside the Apple builds rather than after the
tag, and the tag goes as soon as every store artefact has been read back
rather than after the Apple submission, so the GitHub release, both snaps and
the Windows package no longer wait behind Apple's processing. What is left,
each wanting a dry run on a day that is not a release day:

- **The three store builds run one at a time** only because they swap
  `go.mod` and `FyneApp.toml` in one shared root under an `EXIT` trap. Each
  script already takes its root from its own path, so one worktree per build
  removes the collision; `third_party/` is generated, so each worktree runs
  `scripts/setup-fyne-patch.sh` first. About twenty-five minutes down to ten.
- **The local mirror of CI before a push** repeats what CI runs, some
  forty minutes, and exists because a Mac cannot see a Linux or Windows vet
  failure. When the tree has just been through a verified review that ran
  every suite under every tag and under the race detector, the two pane
  checks (`check-ios-pane.sh` alone, then `check-android-pane.sh`) are the
  part CI cannot stand in for; the rest can be CI's. The cost is a red public
  run when something slips.
- **Screenshots go up by their own tool**, `appstore/push-screenshots.py`
  (29 September 2026), run as its own command in stage 7 between the two
  `submit-version.py` calls, and `play/push-screenshots.py` for Play (30
  September), outside any release. `submit-version.py --write` could call
  the Apple one when a ready set named for the version exists, so the upload
  is not a step to remember.
- **A conductor for the Apple leg** — build, read back, upload, wait for
  `VALID`, `submit-version.py --write` — as one script with a ledger of what
  each step printed, leaving only the submit click.
- **Play from CI at the tag**, as the Windows package is: needs the upload
  key and the service account in the repository's secrets, which is the
  account holder's decision. Play has granted production since 2 October
  2026 (below), so the upload from CI would leave only the promotion, on
  the account holder's OK.

What 1.2.18 (1–2 October 2026) adds:

- **Play steps one at a time.** Every Play step opens an edit, all of them
  as the one service account, and Play lets each user hold one open edit:
  a new edit invalidates the one that user already has open. So a status
  read beside an upload cost the upload its edit ("A Play status read
  during an upload deletes the upload's edit", below). Uploads,
  `release-status.py` and screenshot pushes go in turn, never as parallel
  steps; stage 6 of `docs/RELEASING.md` says so, and the upload says what
  happened when it meets an ended edit.
- **The Play promotion is a scripted step — DONE 2 October 2026.** Until
  then a release stopped on the alpha track and the promotion to
  production was a hand step in the Play Console, because the service
  account was scoped so it could not reach production. It was granted
  "release to production" for this app only that day, and
  `scripts/play-publish.py promote alpha production --confirm-version <v>`
  (efb3e092e) now takes the alpha release to production, dry run first,
  on the account holder's OK, as Apple's `--submit` and the Microsoft
  Store's `commit` are run (`docs/RELEASING.md`, stage 7). It refuses
  anything but this ledger's release on alpha, carries its name and notes
  unchanged, refuses a release production already carries, and commits so
  that Play refuses rather than restarts a review in progress; `upload`
  refuses the production track. It refuses a production versionCode above
  the ledger's Build, and a production rollout still going out or a draft,
  which are settled in the Play Console first; a halted staged rollout
  gives way to the promotion, since that is where a fix is promoted from.
  An option the script does not know, such as `--dryrun` or
  `--rollout=0.2`, is refused before anything is read (de8ef27e2). A
  staged rollout is raised, completed or halted in the Play Console. Not
  yet run against Play: held by stand-in tests
  (`play/test_play_publish.py`, `Promote`). First run against Play for
  1.2.19 on 3 October 2026, the dry run and then the commit, which took
  production to that release's versionCode and into Play's review.
- **The arm64 snap build can fail before it builds anything.** On 1 October
  2026 the Linux stores workflow's `snap (arm64)` job, on the push of the
  release commit (e6fa1b6cc), failed inside `snapcore/action-build@v1`
  while its build instance was being set up ("Launching managed ubuntu
  24.04 instance... Failed to read instance config in environment at
  /etc/craft-instance.conf"), before any of the project was built;
  `gh run rerun <id> --failed` passed with nothing changed. That workflow
  (`.github/workflows/linux-stores.yml`) only verifies, but `release.yml`'s
  `build snap (arm64)` job runs the same action on the same runner image
  and publishes to edge, and stage 10's promotion takes every architecture
  or none, so the same failure in a tag's run holds the snap until the job
  is rerun. The tag's own run built both snaps first time. Either retry the
  build step once on that error, in both workflows, or name the rerun in
  stage 10 of `docs/RELEASING.md`. Stage 10 names it (6 October 2026, for
  1.2.20): check both snap jobs passed, and rerun a failed one with
  `gh run rerun <run-id> --failed` before promoting.
  `TestStage10NamesTheArm64SnapRerun` (`releasing_doc_test.go`) holds the
  stage to that while `release.yml` builds the arm64 snap with
  `snapcore/action-build`. A retry in the workflows is still open.
- **Apple's processing was quick.** It took about three minutes for 1.2.18,
  so the wait for `VALID` in stage 7 need not be planned as a long one.

What 1.2.21 (13 October 2026) adds:

- **Why Play's publish-on-approval is checked by hand is not written.**
  Apple's release type is set by `appstore/submit-version.py` and read back
  by the version's id before a submission. Play's, Managed publishing off, is
  read in the Play Console's Publishing overview and recorded (stage 6 of
  `docs/RELEASING.md`; `docs/PLAY_LISTING.md`, *Publishing mode*), and
  neither page says why it is a hand check rather than a scripted read as
  Apple's is. It is the one difference between the Apple and Play legs of
  the release procedure still open at `6bf27aee4`: the alpha review that
  holds back a promotion, the read-back after a Play commit, the Play notes
  held by a test, the route after a rejection in review and the stale store
  state in the store documents were settled there. One sentence beside the
  check.

## Where the app re-finds the reader's position from text — found 13 October 2026

Released in 1.2.22 on 16 October 2026: eleven fixes, made on 13 October 2026, the eleventh
(P1) on its own branch and merged for the release on 14 October 2026; AND-4,
VERS-1 and AND-3's arrival point, fixed for 1.2.22 on 14 October 2026.
Planned for: the rest in later minor releases, each item below naming its own — see
docs/ROADMAP.md.

Each item is a place where the app decides where the reader is — the verse,
the place in it, the paragraph, a heading, a note's anchor, the scroll
position — by searching the text or the page when the surface that made the
selection already had the position or could carry it cheaply; or re-finds
text after changing it; or, at build time, places a run by its first copy.
Twenty-seven such places are defects, in the twenty changes below; thirteen
of the same shape are deliberate and six are not defects (the end of this
entry). The labels are the ones docs/ROADMAP.md uses. Lines are at
`672086d41` (v1.2.21) and unchanged at `6bf27aee4`. Every measure is over the
WEB, the BSB and the WEB Catholic as the app downloads them; the NKJV was not
measured.

Most of it comes back to one fact every pane has: the selection's place in
its verse (P1, below). From 1.2.22 every shipping pane hands it over with the
span; the plain-text fallback page, which has no span, does not (P3). Ten of
the eleven changes for 1.2.22 leave the Apple panes alone, and P1 changes
every pane's seam; the rest wait for a decision or for nothing in particular.

### AND-2. Android opens a note's band, card or pill inside a poem's stanza — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) — see
docs/ROADMAP.md.

`paraStartAt` (`android/BtBridge.java:3004`) finds a verse's paragraph by
walking back to the previous line break, and an inline copy at :3488 does the
same for a link's arrival. The Android chapter page writes each poem line
with a `<br>` (`android_chapter_html.go:176`), which the importer turns into
a line break, so in poetry the walk stops at the line, not the stanza. The
note sticker (:2423), each pill (:2487), the open card's co-tenant test
(:2505) and the band reservations (:2960, :2974) all take it: a note on a
verse below its stanza's first line opens its band, card or pill in a gap
between two poem lines, and a link into the stanza scrolls to that line. The
iPhone, iPad, Mac, the Windows and Linux pane and the website reserve above
the paragraph. Go already knows the paragraph (`groupNotesByParagraph`,
`notes_plan.go:838`; `noteParagraphOf`, `notes_arrival.go:56`) and pushes
only the anchor verse (`reading_android.go:694`) and each band's lowest noted
verse (:733-734).

6,106 of the WEB's 31,098 verses (19.6%, in 452 chapters) and 6,252 of the
BSB's 31,086 (20.1%, 835 chapters) would open inside their paragraph; Psalms,
Job, Proverbs, Isaiah and Jeremiah most.

Fix, 1.5 days: push the paragraph's first verse with each band spec
(`noteBandSpecs`, `notes_chrome.go:319`, through `btaSetNoteBands`), with the
sticker (`btaSetNote`) and with the arrival (`chapterNoteArrival`), and
replace the five `paraStartAt` calls and the inline walk with
`verseRange(firstVerse)[0]`. The Android fragments of the bridge's contract
tests change with it; then a noted psalm, with a link into its stanza, on the
emulator. docs/ANDROID.md, which still places the band on the anchor verse's
first character, is corrected in the same change.

**FIXED 13 October 2026, for 1.2.22** (`e4996f6b2`). Go pushes, with the
band specs, the verse that opens each of the page's paragraphs
(`androidParagraphOpeners`: the first verse of every block
`buildChapterHTMLAndroid` writes as a `<p>`), as a third array to
`setNoteBands`, and `paragraphStartOf` starts the walk from the opener at or
before the offset, at all five places and in the arrival's scroll.
`paraStartAt` stays the line walk, and with nothing pushed the bridge walks
from the offset as before. docs/ANDROID.md says where the band goes.
`TestAndroidBandsHangAboveTheParagraphNotThePoemLine` holds every verse's
band to its block's first character on the phone page and the book page, and
`TestBtBridgeReadsTheParagraphFromThePushedOpeners` holds the bridge to the
openers at all five places; both fail on `6bf27aee4`. Seen on the emulator
on 14 October 2026 (Pixel 7, API 35, the WEB's Psalm 121, whose stanzas open
at verses 1 and 3): notes on verses 5 and 7 hang one card, labelled "1 of 2
in this chapter", and its pill above verse 3, a note on verse 2 above verse
1, and a link to verse 5 scrolls from the stanza.

### AND-3. Android restores the reading position by a fraction of the scroll range — FIXED 13 October 2026 on Android, for 1.2.22; the arrival point FIXED 14 October 2026

Released in 1.2.22 on 16 October 2026: the Android change and the arrival point, fixed on its release branch (the end of this entry);
the same change on iOS is not yet placed — see docs/ROADMAP.md.

`captureReadingAnchor` (`reading_android.go:923`) reads only the bridge's
scroll fraction and returns verse 0 (:934), so every shared restore path —
the rotation capture (`reading_state.go:130`), the snapshot (:251), the
foreground recovery (`reading_android.go:551-558`) and a same-chapter push
(:783-789) — keeps no verse on Android. The fraction is the scroll offset
over the scroll range, which is the content's height less the viewport's
(`android/BtBridge.java:1733-1735`, :1771-1775), and the bridge re-applies it
after every push (:3525-3528), on an armed restore (:3538-3544) and after a
width change (:3592-3603). It names a point in the range, not in the text, so
any change of content or viewport height moves the verse at the top: John 3
at verse 3 comes back at verse 2, Psalm 119 at 7 at 8. `reading_state.go:14-19`
sets out the other design, a verse anchor that re-resolves after a re-wrap
with the fraction as the fallback, and iOS follows it
(`bibleTextTVCaptureAnchor`, `reading_ios.go:3831-3865`; the restore at
:3073-3085).

A rotation moves the top verse in 32–49% of positions, in 97–98% of chapters;
a text-size change in 58–68%.

Fix, 1.5 days: record the top verse and the offset into it beside the
fraction, from the layout the bridge already has (`verseAtOffset` of the top
line; the offset as a fraction of the verse's own height, since a pixel
offset overshoots a verse that got shorter in a quarter of the turns from
portrait to landscape); hand both through `captureReadingAnchor` and
`armReadingRestore`, and place verse-first in `applyPendingScroll` and
`applyPendingReflow`, keeping the fraction as the fallback, as
`btIOSApplyRestore` does. `readingState` already carries `AnchorVerse` and
`AnchorDelta`.

The same change on iOS, for one rule on both: iOS keeps the offset into the
top verse in pixels (`bibleTextTVCaptureAnchor`, `reading_ios.go:3831-3865`),
the shape that overshoots a verse that got shorter on a rotation; it was
measured on the Android model, not on iOS. Taking the fraction there too, or
clamping the offset to the verse's new height, is Objective-C, which 1.2.22's
set otherwise leaves alone. If AND-3 ships without it, PLATFORM_MATRIX.md
records the difference in the same change, until iOS takes the fraction.

**FIXED 13 October 2026 on Android, for 1.2.22** (`b2df111ce`). The bridge
keeps the position as the verse at the top (the last verse number at or
before the top line's first character), how far into that verse the top
stands, as a fraction of its band from its first line's top to the next
verse's line (`verseBand`), and the scroll fraction. The scroll listener
records it (`lastAnchor`), `getScrollAnchor` replaces `getScrollFrac`,
`armRestore` and `setHtml` take the verse and the fraction into it, and the
restore, the chapter's placement and the width reflow place the verse first
(`anchorY`) and the scroll fraction only where the text does not hold the
verse. The stored `AnchorDelta` is that fraction on Android; the saved format
is unchanged, and a position saved before restores by its fraction.
`TestAndroidKeepsTheReadingPositionByVerse` holds both sides of the bridge to
the verse, and `TestAVerseAnchorKeepsTheTopVerseWhereAFractionMovesIt` runs
the bridge's arithmetic over a wrapped-layout model across both rotations and
both text-size changes: the verse anchor keeps a verse of the top line at
every position short of the end of the range, where the fraction moves it at
39 to 120 positions of each change. iOS was not
changed: the iPhone, iPad, Mac and the Windows and Linux pane keep the offset
into the verse in pixels, and PLATFORM_MATRIX.md records the difference as
divergence 33 until they take the fraction.

Seen on the emulator on 14 October 2026 (Pixel 7, API 35): in Psalm 119,
four rotations and the Large, Extra large and Normal text sizes kept verse 61
within 0.002 of its place, 0.266 into its band, where placing by the fraction
alone would have put the view 286 to 365 px away; John 3:14 held through a
rotation. After a link's arrival it failed: the reader's later scrolls were
dropped, and a rotation or a relaunch put them back at the arrival point.

**The arrival point, FIXED 14 October 2026, for 1.2.22** (`b45fc17c0`,
`74d1cd1f8`; ROADMAP.md, 1.2.22, row 12). The phone chapter header asks for
more than a phone is wide (about 432 units with John 3 on the page, against a
360-unit canvas), so wherever its minimum changed the mobile driver's paint
laid the reading host out about 425 units wide and then at its real 339, and
both frames reached the bridge in one UI tick. With no placement pending the
wide frame armed a reflow that the narrow one undid before the text laid out,
so `applyPendingReflow` never ran: until the next chapter the scroll listener
dropped every scroll and `lastAnchor` stayed at the arrival point, which a
background save kept (1.2.21 saved the live fraction instead). With a
placement pending, the placement was made in the wide layout and nothing
re-placed it: a relaunch asked for John 3:13 landed at 11. The same paint
reached 1.2.21. Two changes:

- Go hands the phone panes only a frame that fits the canvas
  (`nativeHostFrame`, `reading_host_frame.go`); the settled frame follows in
  the same paint. `TestAReadingHostWiderThanItsCanvasIsNotHandedToThePane`
  lays the real phone header out over a stand-in host at 320 to 402 units
  and replays the paint.
- The bridge keeps a one-shot placement through a relayout at another width:
  for 600 ms after an arrival or a restore, until the reader scrolls or a
  chapter arrives, a change of the text's width re-places it
  (`replaceAfterWidth`) and a reflow armed meanwhile carries it rather than
  the scroll position read against the other layout
  (`placementAwaitsReplace`); and `setFrame` drops an armed reflow when a
  frame brings the width back before the text laid out.
  `TestAPlacementMadeAtOneWidthIsKeptWhenTheTextLaysOutAtAnother` runs the
  bridge's arithmetic over the wrapped-layout model, where the scroll
  position read against the other layout moves the top verse at 13 of 91
  placements and the carried placement at none, and holds BtBridge.java to
  the change; `TestAReflowArmedByAFrameThatComesBackIsDropped` holds the drop.

On the emulator, with scratch builds forcing each case: a restore of John
3:14 made at 1017 px with the text laid out at 954 px straight after landed
at 13 in 2 of 3 runs before the bridge's change and left a reflow armed in
the third, and kept 14 in 3 of 3 after it; a 1276 px frame and the real one
pushed together after the reader's first scroll dropped 62 scrolls in each of
3 runs before, saving 14 with the reader on 17, and none after, saving and
restoring 17. Four rotations then kept verse 17 at 0.728 into it.

### AI-1. The fixed study actions resolve their passage again on every request — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) — see
docs/ROADMAP.md.

Only Ask captures its passage when the panel opens (`ai_panel.go:63-64`,
attached to the request at :608-610). For Explain, Analyze context, Analyze
translation and Compare perspectives, `runAIAction` resolves it again at fetch
time, on a background goroutine, against the chapter then current
(`ai.go:469-471`), and so does the reopen after a light/dark change
(`ai_panel.go:97-103`): every Try again, the faster-model switch and the
reopen. The span carries verse numbers only, so once a link or the narration
has moved the chapter under the open panel, the prompt, the cache key and the
header name the new chapter's verses over the old selection's words, and
Report mails the passage the header captured, so after a Try again the mail
and the prompt can name different passages.
`TestStudyPanelWrapperCapturesOnlyAsk` holds the difference as a test seam's
convenience; no entry decided it.

When the chapter moves, 24,921 of the WEB's 31,077 whole-verse selections
(80%) cite a specific wrong verse and the rest the wrong chapter; none is
right.

Fix, 1 day, Go only: capture for every action as Ask does, so the reopen and
the fetch always take the captured passage; `runAIAction`'s fallback becomes
a test seam; `TestStudyPanelWrapperCapturesOnlyAsk` flips, with a
fixed-action twin of `TestAskPanelRetainsCapturedPassage`.

**FIXED 13 October 2026, for 1.2.22** (`d38d75620`). `showAIPanel`
captures the passage for every action, on the UI goroutine, as Ask's
composer does; the request's context carries it, the reopen keeps it, and the
prompt, the cache key, the header and Report read the one capture.
`runAIAction`'s resolve from text and span stays for a caller with no panel.
`TestStudyPanelWrapperCapturesOnlyAsk` became
`TestStudyPanelWrapperCapturesEveryAction`, and
`TestFixedStudyActionsRetainTheirPassage` moves the reader to the next
chapter under the open panel and holds every fixed action's requests, header
and Report to the selected verse through Try again, the rebuild of an
answered panel and the eviction of one in flight. Seen on 14 October 2026 on
the iPhone simulator, with the request seam stubbed so no provider was
called: Explain on John 3:16, the reader moved to John 4 under the open
panel, and Try again still sent John 3:16, under the header "John 3:16
(BSB)", where resolving the passage again gives John 4:16.

### D. A selection the positional path declines borrows another copy's ending — the interim FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026: the interim, fixed on its release branch (the end of this entry),
and the root with P1 — see docs/ROADMAP.md.

When `normalizeShareSelectionProse` declines — a drag begun or ended inside a
word, pieces either side of a mark, a drag begun inside a verse number — the
share falls back to the legacy path (`share.go:457-458`), which finds the
fragment by `strings.Index` over the whole chapter
(`completeTrailingSentenceProse`, `share.go:1263`; also :1954 and :637) and
takes the sentence end and omission mark of that first copy: "oul", dragged
inside "would" in Genesis 2:19, ships as "[O]ul." with verse 7's full stop.
The citation stays right, by the span. "Verse attribution: three findings"
(item 1) records the verbatim fragment on this path, not the borrowed ending.

Of the WEB's 1,151,120 one-word drags begun or ended inside a word of four
letters or more that fall back, 117,933 (10.25%) ship a different quotation,
115,019 of them from another verse; the BSB 9.52%.

Fix: the interim, 0.5 day, searches only the span's region, as
`locateShareSelection` does, which removes 97.5% of them; the root is the
selection's place in its verse (P1). 3 days counted on its own.

**The interim, FIXED 13 October 2026, for 1.2.22** (`fa80bbdb5`). The
fallback places the fragment inside the span's verses in the order the locate
searches a region (the first copy there that splits no word, else the first
of any kind) and hands that place on, so the completion, the retained
terminal and the line-break restore take their positional branches. What is
left is the repeats inside the span's own verses, 0.95% of those WEB drags
and 0.85% of the BSB's, which need P1, also in 1.2.22: with the lead the
fragment is completed at its own place. A selection with no span, or one its
verses do not hold, keeps the chapter-wide search.
`TestDeclinedSelectionIsPlacedInItsSpan`.

**Run on into a heading, FIXED 14 October 2026, for 1.2.22** (`a2a6aafca`). A
piece of a word run on from a verse's last word into the heading drawn below
it is a selection its verses do not hold, and the chapter-wide search quoted
the heading's words as the verse's: "aters.\nThe First Day", begun inside
the last word of Genesis 1:2 in the BSB, was shared as "[A]ters. The First
Day . . . .". Without a lead, 6,651 of the BSB's 50,680 drags into its
headings, 23 of the WEB's 503 and 31 of the WEBC's 557 were shared so. The
fallback now takes the heading's front off the end, as the located path
does (`verseEndBeforeHeading`), and completes what is left where it ends the
span's verses, so each of them is shared as it is with the lead
(`TestAPieceOfAWordRunOnIntoTheHeadingBelowEndsWithItsVerse`; the heading
sweeps of `share_lead_heading_test.go` pin the drags placed nowhere). Left
without a lead: 46 BSB drags begun inside a verse's last word and run
through a heading to the verse below still quote the heading's words, and so
do the BSB's "im.\nThe" at Matthew 27:31 and Mark 15:20, whose verse has
"Him. Then" earlier, where a drag inside the verse makes the same text.
Every shipping pane sends the lead, with which none of them is wrong.

### E and AI-3. A verse number that ends another's is stripped from inside it — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) — see
docs/ROADMAP.md.

`cleanQuoteTextIn` (`share.go:415`) removes each verse marker by replacing a
probe of the number and the verse's first 12 runes with `strings.ReplaceAll`.
Where a verse's number is a digit-suffix of a later verse's and both open with
the same 12 runes, the probe matches inside the later token: "14 “You shall
not commit" becomes "1“You shall not commit" (WEB Exodus 20:13–14), and
"22 “Take a census" becomes "2“Take" (BSB Numbers 4:21–22). It runs on the
share's declined path (`share.go:457`) and on the Ask sheet's and the answer
panel's preview (`ai.go:189`); a located drag never reaches it.

339 verses are prone (193 WEB, 146 BSB, 0.5–0.6%). A drag begun on a later
digit of the previous verse's number reaches the probe, and 5,440 of 9,724
such declined drags reach the reader mangled.

Fix, 0.5 day: the located path's boundary-checked strip,
`stripVerseMarkersProse` over `chapterProseIn`, which is right on all 62,184
verses; one change covers both surfaces. The root is P1's.

**FIXED 13 October 2026, for 1.2.22** (`3df7693d9`). `cleanQuoteTextIn`
runs `stripVerseMarkersProse` over `chapterProseIn`, which removes a number
only at a token boundary and only when verse N's body follows it, for the
share's declined path and the AI preview alike.
`TestFallbackMarkerStripLeavesALaterNumberWhole` holds the strip, the quote
and the preview, with the span and without. The leading digit of a drag
begun inside a verse number is not a marker and stays ("A drag begun inside
a verse number shares the digit"), and P1's lead, also in 1.2.22, takes
it off.

### B. The cross-references panel places a selection with no span by its opening words — tier 1 FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026: tier 1, fixed on its release branch (the end of this entry);
tier 2 with P3 — see docs/ROADMAP.md.

`crossRefsForSelection` (`crossrefs.go:1182`) never normalizes: with a zero
span it goes straight to a probe of each verse's first 24 runes, or 8
(`crossrefs.go:1429`). The panel then names every verse whose opening the
selection shares, where the share card and the prompts name one: Psalm 136's
refrain pulls cross-references for all 26 verses (40 rows, the cap), and John
3:3 names 3–11. It shows on the Apple plain-text fallback page and on any
other zero-span dispatch; it was PLATFORM_MATRIX.md divergence 33, whose
closing waited on this decision.

On that page 1,661 of the WEB's 31,098 whole-verse drags (5.34%, 380
chapters) and 1,350 of the BSB's (4.34%) are over-attributed; refrain-shaped
lines 3.1% and 3.3%.

Fix: tier 1, 0.5 day, decided with 1.2.22's set on 13 October 2026 — the
panel takes the card's order, normalizing first and probing only on a
decline; the verse is then right for 94%, divergence 33 closes, and the
controls in `ai_test.go:589-606` flip. Tier 2 is P3's geometry. 3 days with
tier 2.

**Tier 1, FIXED 13 October 2026, for 1.2.22** (`9249f1ba6`).
`selectionVersesIn` runs the normalize first for the zero span too, so the
panel's header, the verses it pulls references for, the card, the link and
the prompt give one answer for one drag, and the matching answers only where
the normalize declines. PLATFORM_MATRIX.md's divergence 33 for it is removed;
the number now names another divergence.
`TestAIPassageRefAgreesWithShareCitation` holds the panel's citation and verse
list to the prompt's, with a span and without, and
`TestSelectionSpanZeroFallsBackToMatching` and
`TestSelectionSpanShortSelectionResolves` pin the panel at the card's first
copy. Tier 2 stays with P3.

### AI-2. Study with AI takes a zero-span probe's range as the passage — tier 1 FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026: tier 1, fixed on its release branch (the end of this entry);
tier 2 with P3 — see docs/ROADMAP.md. P1, also in 1.2.22, does not reach it:
a page with no span sends no lead.

When the normalize declines on a zero span, `aiPassageFor` takes
`selectionVersesIn`'s answer as the range (`ai.go:198-199`), so one partial
word of 8 runes or more (a drag one letter inside "righteousness") names every
verse holding the longer word: "Romans 3:5–26", "Psalms 119:6–176", in the
prompt, the panel header, the Report mail and the cache's scope.

On the fallback page 15.3% of the WEB's selections and 15.9% of the BSB's
reach the probe, and 43.5% and 40.0% of those name verses the selection was
not made in, all as a range.

Fix: tier 1, 0.2 day — take the probe's answer only when it names one verse,
and otherwise cite the chapter alone, as the comment at `ai.go:178` says;
tier 2 is P3, the root P1. 2 days counted whole.

**Tier 1, FIXED 13 October 2026, for 1.2.22** (`d7554042a`). The
matching's answer is taken only when it names one verse, and otherwise the
selection cites the chapter alone. The rule is in the shared resolver
(`selectionVersesIn`), so the card and the cross-references panel cite as the
prompt does. Over the same selections the matching now answers 33,662 WEB and
39,473 BSB selections, all its right answers before, with no range named. The
cost: a no-span drag begun on a later digit of a verse number and run into the
next verse's first words, 301 of 19,421 in the WEB and 324 of 19,402 in the
BSB, cites the chapter where it named those two verses.
`TestAPieceOfAWordWithNoSpanNamesOneVerseOrTheChapter`. Tier 2 stays with P3;
P1's lead does not reach a page with no span.

### W3. The website does not land on a verse whose link carries a note it cannot read — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, the website with it; fixed on its release branch (the end of this entry) — see docs/ROADMAP.md.

A shared note link's fragment is a key list (`#v16&n=…`), which matches no
id (`cmd/websitegen/render.go:268` writes `v16`), so the browser's own
fragment scroll cannot fire and `:target` matches nothing. `highlightRange`
resolves the verse but scrolls only a range (`cmd/websitegen/assets.go:832`).
A readable note is landed by `showNote` through `rescrollToHighlight`; a
damaged or newer-format note goes to `showNotice` (:1715), which places the
notice before the verse's paragraph and never scrolls. The reader stays at
the top of the chapter with the verse and the notice off-screen, where
NOTE_WIRE_FORMAT.md says the passage opens in all three cases and the app's
panes land it first (`share_link_open.go:461-470`).

BSB John 11:35 with a damaged note stays at the top, the verse 3,954 px down.
Across 2,109 verses the verse is off-screen for 87.8% and the notice for
50.9%; about 90% on the BSB's paragraphed pages.

Fix, 0.5 day: scroll the already-resolved verse for any fragment other than
a bare `#v<lo>`, and have `showNotice` end with `rescrollToHighlight`, which
takes the notice as it takes the card. The reader script is content-hashed,
so the next `scripts/publish-site.sh` reaches every page.

**FIXED 13 October 2026, for 1.2.22** (`355956c88`, with `16edab3f0`).
`highlightRange` scrolls the verse it lit for any fragment but a bare
`#v<lo>`, which the browser scrolls itself, and `showNotice` ends with
`rescrollToHighlight`, the notice counting as the card there.
`TestTheWebPageLandsOnAVerseWhoseLinkCarriesAKey` runs the reader script's
fragment, highlight and placement against a stand-in page. A minimized note's
marker then scrolled twice, to the verse and then to the note; the lighting is
now `lightFragment`, which the marker calls without scrolling
(`TestTheNoteMarkerLandsOnTheNoteWithOneScroll`). The script is
content-hashed, so the fixture site's digests are rewritten in both switch
states, and the change reaches readers with the next
`scripts/publish-site.sh`.

### W1. The website re-anchors a note to the previous link's paragraph — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, the website with it; fixed on its release branch (the end of this entry) — see docs/ROADMAP.md.

`anchorToPassage` (`cmd/websitegen/assets.go:1641-1658`) takes the paragraph
of the lit verse, or else `noteAnchorPara` (:1643), which it sets (:1645) and
nothing resets. A same-document fragment change that lights no verse — Back
or Forward through a note entry with no verse, a second note link pasted into
the tab whose verse the page lacks, a fragment edited by hand — puts the note,
with its tail, before the paragraph the previous fragment lit, and scrolls
there. The comment at :1674-1682 calls that paragraph stale but guards only
the scroll; NOTE_CHROME_UNIFICATION.md gives a note a tail only when its
anchor names a passage.

The triggers are narrow; on 95–99% of pages the remembered paragraph is not
where the note should park.

Fix, 0.5 day: `anchorToPassage` takes the paragraph of the current fragment's
span from its caller and parks without a tail when there is none;
`noteAnchorPara` becomes a value `showNote` sets for `minimizeNote` alone.
Ships with W3.

**FIXED 13 October 2026, for 1.2.22** (`089a7c3fd`). `renderNote` hands
`showNote` and `showNotice` the paragraph of the fragment's own span
(`fragmentParagraph`), and `anchorToPassage` places before the paragraph it
is given and parks with no tail when it is given none; the minimized marker
reads the note's own place (`notePara`).
`TestTheWebNoteParksAtTheTopWhenItsFragmentLightsNoVerse`. With W3, at the
next `scripts/publish-site.sh`.

### G. The legacy Entry pane hands Go no span — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) — see
docs/ROADMAP.md.

`chapterText.menuForSelection` passes `selSpan{}` (`reading.go:1559`) while
the cursor offset is at hand (:1466-1492), so every verb takes the zero-span
path: Psalm 136:20's refrain shares as 136:1, and the cross-references header
names 26 verses. The pane ships in no build. It is the fallback behind the
styled Windows and Linux pane, dead unless `reading_styled_platform_on.go:8`
is flipped; switched on, 0.34% of whole verses and 6.2% of drags of a verse's
last four words would cite another verse.

Fix, 0.5 day: compute the span from the cursor offset and `verseLines`,
recording each verse's offset in `rewrap`; or delete the dead pane, which was
an open decision until 14 October 2026, when the span was taken.

**FIXED 13 October 2026, for 1.2.22, with the span** (`7518f2857`).
`rewrap` records the byte offset at which each verse's first token begins
(`verseStarts`, beside `verseLines`), `selectionBounds` gives the copy and
the new `selectionSpan` the selection's bytes, and `menuForSelection` passes
every verb the span from the verse at or before its first byte to the one at
or before its last. `TestLegacyPaneSharesASelectionFromItsOwnVerse` shares the
refrain selected in verse 3 of the Psalm 136 fixture as 136:3, and
`TestLegacyPaneRecordsWhereEachVerseBegins` holds every recorded offset at
five widths. The pane still ships in no build; deleting it stays possible.

### P1: A, IOS-SEL-OFFSET-DROPPED, MAC-SEL-OFFSET-DROPPED, AND-1. Every pane drops the selection's place in its verse — FIXED 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, made on its own branch and merged for the release on 14
October 2026 (the end of this entry) — see docs/ROADMAP.md.

Every pane works out where a selection lies in its verse and hands Go only
the text and its verses (`selSpan`, `selection_span.go:16`): the iPhone and
iPad (`reading_ios.go:272`), the Mac (`reading_macos.go:285`), Android
(`android/BtBridge.java:1590`) and the Windows and Linux pane
(`reading_styled_select.go:450`). `locateShareSelection` (`share.go:961`)
then takes the first copy of the words in the span's verses, so a later copy
of repeated words is quoted from the earlier one, its sentence ending,
retained terminal and poem breaks read from there: the second "Abraham!" of
Genesis 22:11 is shared as "Abraham . . . !". It reaches Share with citation,
Share as image and the note composer's excerpt on all six; the study verbs
and cross-references cite by verse, which a repeat inside one verse does not
change. This is the open question at the end of "Verse attribution: three
findings" (item 1), deferred on 7 October 2026 with the cost recorded there
for drags begun or ended inside a word.

Of the WEB's 2,171,994 selections of one to three words, 185,053 are placed
at an earlier copy and 14,209 ship different text: 1.44% of one-word
selections, 0.34% of two-word and 0.12% of three-word. The BSB: 11,785
(1.29%, 0.27%, 0.08%). The Mac's character-granular drags add the shape
`share_phrase_sweep_test.go:124-160` pins ("leavened bread shall").

Fix, about 5 days as one change; counted apart, the shared seam and consumer
with the styled pane (A) 3, iOS 4, the Mac 2 and Android 2. `selSpan` gains
the offset within verse lo, -1 when unknown; each pane fills it where it has
it — iOS `range.location` less `btIOSVerseAtIndex`'s out location, the Mac
`sel.location` less the numeral run, Android `s0 - verseStarts[i]`, the styled
pane `p.selStart - run.Offset` — and `locateShareSelection` takes it as the
anchor it already honours (`share.go:957`), with today's search as the
fallback. The seams: `reading_ios.go:272-296` and its externs (:31, :33);
`reading_macos.go:280-295` (:94, :96); `ai_menu_darwin.go:68`, :87;
`android/BtBridge.java:1590-1613` and :970, `reading_jni_android.c:28-36`,
`reading_android_export.go:27-30`; `reading_styled_select.go:425-451`; the
consumer, `share.go:956-971`. On the Mac, handing over the verse's words
before the selection rather than a UTF-16 offset lets the normalisation the
selection already goes through place it. Where a span comes with the
selection, it also closes D's root, M, the root of E/AI-3 and "A drag begun
inside a verse number shares the digit", and turns the trade-off
`share_phrase_sweep_test.go` pins into a positive test. It does not reach
AI-2's root, whose page sends no span (P3). It is expected to close the two
dash phrases of "A phrase ending on a dash, where its verse has the same words
earlier, is cut short" that carry a span, which the phrase sweep run with the
offset has to show. Open: one change on all four panes, or pane by pane with
-1 as the fallback; taken 14 October 2026, one change.

**FIXED 13 October 2026, for 1.2.22** (`28be4ea08` to `708542b7e`, merged
for the release on 14 October 2026), as one change on all four panes. Each
pane sends, with the span, the selection's lead: its own text of verse lo
from the first character of the verse's number to the selection's first
character (docs/SELECTION_OFFSET.md has why that value and not an offset,
and every seam). `selSpan` carries it (`lead`, `hasLead`; no lead is the
zero value, so every caller that sends none behaves as before), and
`leadStart` checks it byte for byte against verse lo before it is used.
`placeByLead` and `walkSelectionFrom` (`share_lead.go`) read the selection
against the chapter from that place, take a verse number out only where its
verse starts, and read the headings a pane draws between two verses. The
iPhone and iPad cut the lead at `btIOSVerseAtIndex`'s location, the Mac at
the location `btMacVerseSpanForRange` reports, Android from `verseStarts`
through `readerText`, and the Windows and Linux pane from its layout; the
two Apple exports and `btaSelectionAction` take it as a fifth argument,
`NULL` or `null` for none, held by
`TestTheSelectionExportsMatchTheirCDeclarations` and
`TestEverySelectionDispatchPassesTheLead`. With the lead nothing is placed
wrong in CI: 229,224 selections of one to three words over the embedded
Gospels, their 13,853 later copies and 123,177 pieces of a word
(`TestEverySelectionInTheGospelsIsPlacedByItsLead`); the 90,264 drags of
`TestSpannedBoundaryDragSweepWithLeads`, no digit drag among them sharing a
stray digit; 156,594 selections made through the styled pane's own layout;
23,567 drags into invented headings. Over the local WEB, BSB and WEBC caches
(`BIBLETEXT_EXHAUSTIVE=1`) nothing is wrong with the lead either: 2,171,994,
2,080,560 and 2,469,492 selections, 1,160,304, 1,125,726 and 1,325,427
pieces of a word, 2,249,198, 2,163,292 and 2,560,919 drags begun or ended
inside a word, and 503, 50,680 and 557 drags into the publishers' headings.
Without a lead every path is the one the other fixes in this entry leave,
and the counts without it are pinned: the pieces of a word that share
another place's answer are 12,925, 11,218 and 13,764 there, where completing
them from the chapter's first copy, before D's interim, gave 118,136,
106,972 and 127,888. Seen on 14 October 2026 on the iPhone 16 Pro
simulator (iOS 26.5, the BSB), a development build on the Mac and the
emulator (API 35), both of those in the WEB: the second "Abraham" of Genesis
22:11, the second "you" of Matthew 25:12 and a later word of John 3 (the
second "flesh" of verse 6; the BSB's second "born" of verse 4) share from the
reader's copy, "Abraham!" and "[Y]ou." where the first copy gives "Abraham
. . . !" and "[Y]ou . . . .", each byte for byte what Go gives for that lead.
The phrase sweep run with the lead places none of the WEB's 2,047,710, the
BSB's 1,956,416 and the WEB Catholic's 2,328,087 phrases wrong.

### M. The marker strip confirms a verse number by any overlap with that verse's opening

Planned for: a later minor release; with a lead, closed in 1.2.22 by P1 (the
end of this entry) — see docs/ROADMAP.md.

`stripVerseMarkersAnchored` (`share.go:1723`) takes an "N " token as verse
N's number when what follows it overlaps verse N's opening by any length. A
digit that is verse text, followed by a word that opens verse N, would be
stripped as N's number and the shortened words searched for again, possibly
in another verse: an invented "The city had 12 gates" was cited verse 3.

Latent: no such in-verse digit token in the WEB, the BSB (50 candidates, each
followed by a lower-case word) or the WEB Catholic, whose one Daniel 3
heading of the shape declines safely; the NKJV was not measured.

Fix, 2.5 days, on P1's change: the panes pass the positions of the number
runs inside the selection, which they all have, and the strip removes exactly
those.

**With a lead, closed 13 October 2026, for 1.2.22**, by P1: the walk from the
lead (`walkSelectionFrom`) takes a verse number out only where its verse
starts and the token is that verse's number, so a figure followed by the
words that open the verse of that number is kept
(`TestAFigureInAVerseIsNotTakenForAVerseNumber`, which also holds the strip
without the lead). The text strip still decides for a selection with no
lead, and where the walk from the lead fails.

### AI-4. The study verbs do not carry the chapter they were chosen in

Planned for: 1.2.23, both halves, decided 14 October 2026: 1.2.22 ships
without it. Fixed for 1.2.23 with P3: built first on its own branch,
`fu/ai4-chapter`, then taken with P3 onto `fu/1223-ai4-p3`, where it sits
on P3's fallback page (`b96c49c38`, `f407e5ded`, `5a9216df1`), and merged
with it into `rel/1.2.23` on 15 October 2026 — see docs/ROADMAP.md, 1.2.23,
row 3 (Later minor releases, row 27 until then).

The span carries verse numbers only, and the dispatch applies them to
whichever chapter is current (`readerChapter`, `ai.go:185`). Continuous
narration that turns the chapter between the menu and the click cites the
next chapter: the desktop menu survives the refresh, and the Mac's menu
tracking holds the queued turn until the menu closes. Psalm 135:3 selected,
Explain clicked after the turn, the header says "Psalms 136:3". The share
and cross-reference routes rest on the same assumption.

Rare to trigger; once it fires, 0 of 1,188 chapter pairs stay right (1,186
cite the next chapter's verse, one the next chapter alone, and one quotes the
next chapter's own words, Ezekiel 38:3 to 39:1).

The fix, as built: the span carries the book and chapter of the text the
selection was made in (`selSpan.book` and `selSpan.chapter`), and every
function that takes a span reads the selection in that chapter
(`selectionChapter`), the reader's own only where the span names none: the
study verbs, whose panel captures the passage once (AI-1), with its header,
cache key and Report mail; Share with citation, as image and as link; Share
with note's composer, its link and the record it keeps; and the
cross-references, with their coverage note. Each pane names the
chapter. The two Fyne panes read it off the verses they laid out, and
`selectionStudyMenu` stamps the chapter on screen on a span that has none.
The iPhone and iPad pane keys the page it installs, each import and P3's
fallback page alike, with the chapter Go announces on the main queue just
ahead of the chapter's HTML (`bibleTextTVSetChapter`), and the edit menu
takes it with the selection's text, span and lead; the Mac pane does the
same (`bibleTextMacTVSetChapter`) and reads it when the action runs. The
HTML's block takes the announced chapter once, behind P3's check that its
push is still the latest, so a push already replaced takes nothing and each
retry keys the page with its own push's chapter; the fallback page is keyed
in the branch that installs it, so a push with no fallback page leaves the
text under the key it has. Android keys the text in `setHtml` from
`BtBridge.setChapter` and captures it when the toolbar item is chosen,
through the study popup. All three hand it on as the last two arguments of
`bibleTextAIMenuTapped`, `bibleTextStudyMenuTapped` and
`btaSelectionAction`, and Go builds the span with it in `paneSelSpan`. A
function handed the selected verses rather than the span reads their own
chapter (`selectedChapter`): the cross-references list, for the next
release's coverage note and for the edition's own block, whose Psalm title
notes and whose citations set as the verse's own passage were the chapter
on screen's. Share with note anchors its sheet to the selected words only
while the page holds the passage they were selected on
(`notePassage.onScreen`): the Mac and iPad panes find the words by verse
number in the text on screen, and after a turn they pointed the sheet at
the same verse number in the next chapter, a second late, waiting for a
note the page does not show there. Off the passage the sheet is presented
as a share with no note is; the same holds for a link that moves the
reader while the composer is open.

Tests: `TestTheDesktopMenuReadsTheSelectionInItsChapterAfterNarrationTurnsIt`
builds each desktop pane's menu over an invented Psalm 135:3, turns the
chapter with the narration controller (`advanceAndContinue`) and clicks
Explain, the three text shares, Share with note and Cross-references.
`TestAVerbReadsTheSelectionInTheChapterItWasMadeIn` and
`TestANativeSelectionDispatchedAfterTheTurnKeepsItsChapter` hold the
consumers and the native route, with the reader's chapter as the control
for a span with none. `selection_chapter_contract_test.go` holds each pane's
dispatch to the chapter its text holds, each pane's text to the chapter it
was pushed as, the chapter to being taken once behind the push's check and
ahead of the first import and the retries, each push to announcing it
first, and every Go function that takes a span, or the selected verses, to
reading the selection in its own chapter; its checks of where the chapter
is taken and where the fallback page is keyed carry controls that move
each out of place.
`TestTheEditionsCrossReferenceBlockIsTheSelectionsChapterAfterATurn` and
`TestShareWithNoteIsAnchoredToTheWordsOnlyOnTheirPassage` hold the block
and the anchor, each with the reader still on the passage as the control.
`TestAMenuOverASpanWithNoChapterReadsItInTheChapterItWasBuiltIn` holds the
menu's stamp: both desktop panes lay out the invented Psalm 135 with the
book taken off its verses, hand the menu verse 3 with no chapter (its
control), and Share with citation and Share as link, clicked after the
turn, cite Psalms 135:3; with the stamp taken out of `selectionStudyMenu`
they cite 136:3.
On `main` at `149b85a91`, whose panes and Go are 1.2.22's, the desktop test
names Psalms 136:3 for every verb on both panes, and the five contract tests
fail, the parse naming 13 functions that read the reader's chapter; the
tests that build a span with its chapter do not compile there, and with the
span's chapter, or the selected verses', set aside for the reader's they
fail on this tree (136:3 again; the edition's block linking 135:1-3), as
the anchor test does with the anchor handed over off the passage. Opt-in
over the local caches, none of the WEB's 1,188 chapter pairs, the BSB's
1,188 or the WEB Catholic's 1,327 is read differently after the turn.

Seen on 14 October 2026 with AI-4's first commit as made on its own branch
from 1.2.22, before the two follow-ups above and before P3, with a
throwaway trigger in a scratch build: Psalm 135 opened, verse 3 selected,
the chapter turned as narration turns it, then Explain, the three text
shares, Share with note (sent with no note) and Cross-references chosen.
Every one cited Psalms 135:3 on the Mac (WEB), the iPhone 16 Pro simulator
(WEB and BSB) and the emulator (API 35, WEB, one word selected with a real
long-press and the verbs chosen with real taps), where the same trigger on
1.2.22 cited Psalms 136:3 for all six on each. On the Mac and the simulator
the verbs ran in the app, through the pane's own action methods and its
edit menu's own actions. Not run: an iPad simulator, which runs the
iPhone's pane; the Windows and Linux virtual machines, where the styled
pane's test above stands; the BSB on the Mac and the emulator; and Share
with note with a note after a turn on an iPad or a Mac.

Seen again on 15 October 2026 on the combined tree, with development
builds of this branch at `638bc88e8` and of `main` at `149b85a91` as the
control, and the same steps. On the Mac and the iPhone 16 Pro simulator, in
the WEB and the BSB, and on an iPad Air simulator in the WEB, every verb
cited Psalms 135:3: Explain, whose request went to a stubbed provider that
sends nothing and whose prompt names 135:3; the three text shares; Share
with note, with a note typed into its composer, linking 135:3 and handed
over with no anchor to the words; and Cross-references. The control cited
Psalms 136:3 for every one and handed Share with note over anchored to
verse 3 of the page then on screen, Psalm 136's. On the emulator (API 35,
WEB, a real long-press, the note typed through an input method) the fix
cited 135:3 for all six and the control 136:3. With every import refused
(`BIBLETEXT_DEV_FAIL_IMPORT=1`), verse 3 selected on P3's fallback page
was handed over on the Mac and the iPhone simulator, in both editions,
with its verse, its lead and Psalm 135, and every verb cited 135:3; on the
control the page gave no span and every verb cited the chapter, Psalms
136. Not run: the Windows and Linux virtual machines, the BSB on the
emulator, and the iPad with every import refused.

### P3: C, IOS-PLAIN-FALLBACK-ZERO-SPAN, MAC-PLAIN-FALLBACK-ZERO-SPAN. The Apple plain-text fallback page has no verse geometry

Planned for: 1.2.23, decided 14 October 2026: 1.2.22 ships without it.
Fixed for 1.2.23 with AI-4: built first on its own branch,
`fu/p3-fallback-geometry`, then taken with AI-4 onto `fu/1223-ai4-p3`
(`12bd8ea67`, `51f7efe66`), and merged with it into `rel/1.2.23` on
15 October 2026 — see docs/ROADMAP.md, 1.2.23, row 4 (Later minor
releases, row 28 until then).

When a chapter's HTML import fails three times, the iPhone and iPad empty the
verse index (`btIOSBuildVerseIndex(nil)`, `reading_ios.go:3560-3564`) and the
Mac sets `gMacContentStart` to 0 (`reading_macos.go:1723-1725`), over the text
of the plain converters (:3478, :1668). Every verb there gets a zero span and
searches the whole chapter: Psalm 136:10's refrain is cited 136:3, "light" in
Genesis 1:5 is cited 1:3, and a link's or note's anchor and the AI reference
follow. The psalm title's clamp is disarmed (a BSB drag from the title through
the heading shares "A Psalm of David. The LORD Is My Shepherd 1 The LORD"
under the chapter; on the Mac a WEB Psalm 18 title word is quoted as
Scripture), and no highlight, note or search arrival lands. The no-span title
and heading rules of "Verse attribution: three findings" (item 2, decided 6
October 2026) exist for this page.

No reader has been seen on the page, and the import's failure rate has never
been measured. On it, 60.2% of the WEB's and 57.7% of the BSB's double-clicked
words cite an earlier verse, as do 0.88% and 0.99% of poem lines and 0.34%
and 0.35% of whole verses; all 31,098 numerals are recoverable.

Fix, about 2.5 days for both panes (2 for iOS, 1.5 for the Mac): one walk
over the cut HTML builds an attributed string — numerals at 0.66 em, the
title italic, headings bold — and the import's own derivations run over it
in order, so every existing walker answers and Go receives a real span. The
three fallback tests (`reading_native_fallback_page_test.go`,
`reading_native_fallback_contract_test.go`,
`readalong_title_native_contract_test.go`) change, and a development flag
that forces the import to fail gives a simulator check. It retires the no-span
rules, is tier 2 of B and AI-2, and settles 1.2.20's open question whether such
a selection should cite the chapter instead.

**Built 14 October 2026 on its own branch, `fu/p3-fallback-geometry`**
from 1.2.22 (`7a68743cb`), and taken with AI-4 onto `fu/1223-ai4-p3` from
`main` at `149b85a91`, where its first commit is `12bd8ea67` and the fixes
after the device checks are `51f7efe66`, and where AI-4 keys the fallback
page with its chapter as it keys an imported page; decision 2 puts both in
1.2.23. As recorded above, with the walk in Go rather than in each pane:

- Go builds the page with every push, from the HTML it pushes
  (`appleFallbackPageOf`, `reading_apple_fallback.go`): cut at the
  translators' notes, the markup out, a line feed at the end of each
  paragraph and U+2028 for each `<br>`, as the importer writes them, and
  now with runs saying where each verse number, the title and each heading
  stand, in UTF-16 units. The page and its style (the body's and the
  numeral's sizes, the text's and the numbers' colours) cross with the HTML
  (`bibleTextTVSetHTML`, `bibleTextMacTVSetHTML`). On a chapter the size of
  Psalm 119 the walk costs about as much as building the HTML.
- Each pane sets the runs in the faces the importer gives the same markup
  (`btIOSFallbackPage`, `btMacFallbackPage`: the numeral at its size, bold,
  in the numbers' colour; the title italic; headings bold) and installs the
  page through the one install both pages now share, split out of the
  import (`btIOSInstallPage`, `btMacInstallPage`). The verse index, content
  start, title, highlight, note, wash, resume marker and placement are
  derived from it as from an imported page, and the selection verbs read a
  span and a lead from that geometry. The Mac paints the chapter's wash on
  it from the model, since its wash otherwise comes from the stylesheet.
- Every step of the ladder, the first import included, and the fallback act
  only while their push is still the latest: the install confirms the
  chapter, so a step left from a replaced push must not put the older
  chapter in. The announcement (`bibleTextIOSBeginChapterPush`,
  `bibleTextMacBeginChapterPush`) issues the push's generation on the
  caller's thread, moves the pending one only forward and returns it, and Go
  hands it over with the push. A generation read on the main queue would be
  shared by two pushes made in one main-thread turn, as the Mac's are, and a
  retry of the first could install the older chapter over the second.
- The reading face is checked by its PostScript name
  (`btReadingFaceNamed`, `reading_fonts_apple.go`). CoreText answers a face
  it does not have with Helvetica, upright and regular, so had the faces'
  registration failed, both pages would have lost the italic title and the
  bold headings the panes find by their traits; such a face is now no face,
  and the run keeps its own. Registration succeeds today.
- A development build forces every import to fail with
  `BIBLETEXT_DEV_FAIL_IMPORT=1` (`dev_fail_import_on.go`; a release build's
  answer is always no, `TestDevFailImportIsInertInReleaseBuilds`).
- The no-span rules stay, for any selection that still arrives without a
  span: AI-2's one-verse rule (`selectionVersesIn`) and the title and
  heading rules (`normalizeShareSelectionProse`). On this page they no
  longer decide anything.

Tests: the page and its runs (`reading_native_fallback_page_test.go`), what
a selection on it is answered with, modelling the panes' clamp, verse lookup
and lead over the page (`reading_apple_fallback_test.go`), and the panes'
source (`reading_native_fallback_contract_test.go`, with
`readalong_title_native_contract_test.go`). Over a WEB Psalm 136:1–11 and
Genesis 1:1–5 fixture every double-clicked word is cited from its own verse
(0 of 150 and 0 of 83 wrong; the same page without its runs, as it was
shown, 97 and 30). With the runs taken out of the walk the reproducers fail
as the defect did (136:10's refrain cited 136:3, "light" in 1:5 cited 1:3, a
title handed over with no span). As P3 wrote them, six of the eight source
tests in `reading_native_fallback_contract_test.go` fail on 1.2.22's panes,
as does `TestNativePanesFindTheTitleAsTheItalicParagraphBeforeVerseOne`;
the other two, `TestHighlightRangeHasOneWriterOnEachPane` and
`TestTheSelectionVerbsSendTheSpanAndLeadTheInstallGives`, are guards that
pass on both.

Seen on 14 October 2026 on the iPhone 16 Pro simulator (iOS 26.5) and on
the Mac, with development builds of P3's first commit as made on its own
branch from 1.2.22 (its change is `12bd8ea67` here) and of 1.2.22
(`7a68743cb`) as the control, every import refused for runs 1 to 5:

| Run | P3's first commit | 1.2.22 |
|---|---|---|
| 0, Psalm 136:10's refrain on an imported page (WEB) | 136:10 by every verb | the same |
| 1, the same refrain on the fallback page | span 10–10 with its lead; the share, link, cross-references and AI request cite 136:10 | no span; all cite 136:3 |
| 2, "light" in Genesis 1:5 (WEB) | 1:5 by every verb | no span; 1:3 |
| 3, a drag from Psalm 23's title into verse 1 (BSB) | cut at verse 1's number and handed over as 1:1 with its lead | the title and heading handed over with no span and sent to the AI request; the share the same as with the fix |
| 4, Psalm 23's title alone | the system's verbs only | the app's verbs; the share an empty quotation under 23:1 |
| 5, a link to Psalm 119:100 (WEB) | verse 100 lit and in view | nothing lit, the view at the top |

In that commit the page broke each poem line with a line feed, which
NSString takes for the end of a paragraph, where the importer writes U+2028
and ends a paragraph only at `</p>`. Both panes reserve a note's band, place
its sticker, card and pills, and test which notes share a paragraph, by the
paragraph that holds the anchor verse's number, so on this page a note
opened between two lines of a stanza, as on Android before AND-2's fix:
6,106 of the WEB's 31,098 verses, 6,252 of the BSB's 31,086 and 7,610 of
the WEB Catholic's 35,379 had their number's paragraph open elsewhere than
on the imported page. With U+2028 for each
`<br>` none does, and every selection's answer stays equal to the imported
page's (`51f7efe66`;
`TestTheFallbackPageEndsAParagraphWhereTheImportedPageDoes`). The same
commit issues each push's generation at its announcement
(`TestEveryImportStepActsOnlyForTheLatestPush`) and checks the reading face
by its name (`TestAReadingFaceCoreTextDoesNotHaveIsNoFace`). The runs above
were made before it and before AI-4.

Seen again on 15 October 2026 on the combined tree, with development
builds of this branch at `638bc88e8` and of `main` at `149b85a91` as the
control, on the iPhone 16 Pro simulator and the Mac: runs 0 to 5 gave each
build the answers in the table above, and run 1 did the same on an iPad Air
simulator. A sixth run put a note on a verse inside a psalm's stanza (BSB
Psalm 23:2, WEB Psalm 121:4). On the fallback page the paragraph holding
the verse's number is the imported page's, the stanza that opens at verse
1 in eleven lines and the one that opens at verse 3 in twelve, where
1.2.22's fallback page has no such verse in its index. As first planned,
the note written and the chapter left and opened again, the run shows no
band on either build: the reader's own note is drawn only while it is
focused, and navigating resets focus (`notes_plan.go`). Written on the
page the launch opens and read while its card is open, the note's band
sits on the stanza's paragraph on the imported page and the fallback page
alike, on both panes, where on 1.2.22's fallback page it sits at the
chapter's top. Not run on an iPad simulator: runs 0 and 2 to 6.

On the combined tree the tests still tell the fix from the defect. With
this tree's tests and Go over the two panes of `main` at `149b85a91`, which
are 1.2.22's, seven of the eight source tests and the title test fail.
AI-4 widened `TestTheSelectionVerbsSendTheSpanAndLeadTheInstallGives` to
the book and chapter the panes now hand on, which 1.2.22's panes do not,
so `TestHighlightRangeHasOneWriterOnEachPane` is the one guard that passes
on both. The Go tests of the page do not compile on `main`, since the walk
is new; on this tree, with the runs taken out of the walk the four
reproducers fail as the defect did (97 of 150 words cited from another
verse) and so does the runs test, with a line feed for each `<br>` the
paragraph test fails (Psalm 136: 22 paragraphs against 1), and with the
face's name check taken out the face test fails (Helvetica).

The fallback page is still plainer than an imported page: no red letters,
no justification, first-line indent or paragraph gaps, the verse number not
raised, an omitted verse's mark at body size.

### P4: APPLE-HEADING-MATTER-TO-VERSE-ABOVE. A selection inside a heading resolves to the verse above — FIXED 15 October 2026, for 1.2.23

Planned for: 1.2.23. Decision 23 was taken on 15 October 2026 (A), and the
fix is built on its own branch, `fu/1223-heading-words` (`b89c475c9`, from
`be1440fe0`; the text share in `99dbcd728`, Share with note's anchor in
`ca1f500a6`), merged into `rel/1.2.23` on 21 October 2026. Its follow-up,
no study verb for a selection that holds no Scripture, is built on
`fu/1223-no-scripture-study` and merged into `rel/1.2.23` on 22 October
2026 (`a2cbc8c46`; below). The prompt's passage, taken on 22 October 2026,
is built on `fu/1223-ai-no-headings` (`e86fc00be`, from `da90dfb55`) and
merged into `rel/1.2.23` on 22 October 2026 (`a04829c5f`); it was seen
on 22 October 2026 on the Android emulators, the iPhone and iPad
simulators and the Mac, and on Windows and Linux only in the Mac's mimic
(below). The follow-up's rules were confirmed on 22 October 2026, and a
selection whose only Scripture is punctuation is offered no study verb
since `ca86cca13`, on `fu/1223-no-study-punctuation` (from `91e1ec683`;
below), not yet seen on a device. The device checks passed on 15 and 16
October, and on 21 October for the later two commits on the Mac and the
iPhone and iPad simulators; the follow-up's on 21 and 22 October on the
Mac, the iPhone and iPad simulators and the Linux and Windows virtual
machines, and on 22 October on the Android emulators (the end of this
entry) — see docs/ROADMAP.md, 1.2.23, row 12.

`btIOSVerseAtIndex` ignores the heading range the index keeps
(`reading_ios.go:273`), the Mac never asks `btMacIsHeadingParagraph`
(`reading_macos.go:284`, :292), and Android has the same backward rule
(`android/BtBridge.java:1593`): a selection inside a publisher's heading
resolves to the verse above at both ends. Go's repair
(`headingOnlySelectionVerse`, `share.go:1427`; the `span.lo == above` test at
:1462) fires only when the words are not found first, so a heading
word the verse above also has is cited from that verse by the share, the note
anchor, the cross-references and the AI reference: "Cain" in "The
Descendants of Cain" cites Genesis 4:16, and the "The" of "The Fourth Day"
cites 1:13 and is quoted as Scripture. "Headings: quoting, notes and links
need one deliberate pass" records the backward rule.

The BSB: 2,023 of 7,373 single heading words (27.4%), and 1,301 of its 1,896
headings (68.6%) have such a word. The WEB Catholic 27 of 113 (the fused
Daniel 3 heading), and 1 of 34 once 1.2.23 sets that heading's note apart
(docs/ROADMAP.md, 1.2.23, row 11); the WEB none of 23.

Fix, 2 days: each pane tests the selection against the heading range it
already keeps (iOS's end and tail, the Mac's `btMacIsHeadingParagraph`), then
either (A) reports the verse below, with a Go companion that accepts a span
whose lo is that verse, or (B) clamps heading matter out as the psalm title
is clamped; Android the same for parity. (B) forecloses answers the headings'
pass keeps open (docs/ROADMAP.md, Next major version, row 5), so its first
question, what selecting a heading means, is answered first.

**The decision (23, 15 October 2026): A.** A word selected inside a
publisher's heading means the section the heading introduces: it is cited
by the verse below the heading and quotes nothing, the rule the app already
applied to a heading selected whole on its own. The other options were to
trim heading words out of a selection as a psalm title is trimmed, to quote
the heading marked as the publisher's, or to wait for the major's pass.

**The follow-up (21 October 2026).** A study verb, Explain, Compare, Ask
and every other, is not offered for a selection that holds no Scripture:
a publisher's heading's words alone, a psalm title alone or a verse
number alone. Every pane's menu leaves Study with AI and its items out
for one, and the dispatch refuses one as a backstop. A share of such a
selection keeps A: the verse cited and nothing quoted. A selection that
takes Scripture, a drag from a heading into its verse among them, is
studied as before. This replaces the study prompt `99dbcd728` built,
which read the verses such a selection is cited by; and the sentence
approved for the prompt naming the heading is not needed, since no
selection of a heading's words alone reaches it.

**The prompt's passage (22 October 2026).** A study prompt's passage holds
only Scripture. A selection that runs across a publisher's heading or a
psalm's title, from the end of one verse into the next, sends the verses'
words alone: the heading's words are left out as the share leaves them
out.

**Confirmed, and punctuation alone (22 October 2026).** The rules the
follow-up built are confirmed as built: no study verb for a drag from a
heading that ends inside the first word of the verse below or takes only
the punctuation that verse opens with; on Android, Study with AI hidden
for a selection inside a psalm's title or the footnote section; Share and
Cross-references keeping the order they have beside Study with AI; and
the verse-number share fixes. A selection whose only Scripture is
punctuation, a verse's closing full stop or any other mark of its words
selected alone, or a drag from a verse's closing punctuation into the
heading below it that ends in the heading, is offered no study verb, the
same rule as a selection that holds no Scripture, on every pane's menu,
and the dispatch refuses one as a backstop. A selection with any complete
words of Scripture is studied as before.

**Unfinished boundary words (23 October 2026).** A drag from a verse's
closing punctuation through a heading into an unfinished word below is
offered no study verb. The same boundary rule applies on every pane:
punctuation before the heading does not count as a complete word, and the
unfinished word is trimmed back to its space or closed-up dash. The shared
quotation keeps any source-located punctuation left by that trim rather
than letting the fallback restore the unfinished word. This also keeps
the Go and native rules together when punctuation and a space open the
verse below. A complete word before or after the heading still offers
study; a single partial word selected inside a verse keeps its existing
fragment behavior. `TestAnUnfinishedWordAcrossAHeadingLeavesNoStudyableFragment`
checks the shared quotation, menu, prompt and native rule model, with and
without a positional lead; `TestACompleteWordAboveAHeadingKeepsStudyAfterTheBoundaryTrim`
keeps the complete-word control. The existing dispatch regression also
includes the punctuation-plus-unfinished-word shape. Source changes are
integrated locally in `rel/1.2.23`, uncommitted; the updated tests and
native platform checks have not yet run.

**The fix.** Each pane that draws a heading in the text a reader selects
tests the selection against the heading range it already keeps beside its
verse numbers, after cutting it to the scripture its verbs may take, and
hands a selection wholly inside a heading over as the verse below it, with
a heading flag and no lead:

- iOS (`btIOSHeadingSelectionVerse`, reading_ios.go): the selection, less
  the white space at either end, lies between a verse entry's `end` and
  `tail`, the heading block `btIOSBuildVerseIndex` records after the verse;
  the verse below is the next entry's.
- Mac (`btMacHeadingSelectionVerse`, reading_macos.go): the selection opens
  in a heading paragraph (`btMacIsHeadingParagraph`) and ends inside the
  block of headings that starts there (`btMacHeadingBlockEndAt`); the verse
  below is the first number after the block. `hbSendAI` and `hbSendStudy`
  ask it.
- Android (`BtBridge.headingSelectionVerse`): the selection, less white
  space, the heading's gaps and a paragraph's direction mark, lies between
  a verse's end (`verseEnds`, where `endBeforeHeading` stopped it) and the
  next verse's number, on heading lines only (`isHeadingParagraph`).
  `onActionItemClicked` asks it and the study popup carries the answer.
- The flag crosses `bibleTextAIMenuTapped`, `bibleTextStudyMenuTapped`, the
  JNI thunk and `btaSelectionAction` as one more argument after `hi`, and
  `selSpan.inHeading` marks the span with it (selection_span.go).
- Go (`normalizeShareSelectionProse`, share.go) answers a span with the
  flag before any search: the verse below and nothing quoted. Every verb
  reads the selection through it, so the text and image shares, the link
  and the note kept against it, the note composer, the cross-references and
  the study panel's citation all name the verse below.
- No study verb is offered for a selection the share answers with
  nothing quoted, a heading's words, a psalm's title or a verse number
  selected on its own (the follow-up). The study prompt put the selected
  words in quotation marks as the cited verse's passage, so a heading's
  words went to the provider as Scripture the verse does not hold: the
  BSB's "Fourth" of "The Fourth Day" as Genesis 1:14. `99dbcd728` sent
  the verses such a selection is cited by instead; that is taken out.
  Go's rule is `aiStudyOffered` (ai.go): the desktop menu
  (`selectionStudyMenu`, reading.go) leaves Study with AI out for such a
  selection, Share and Cross-references keeping their places, and
  `dispatchAIAction` drops a study verb for one, opening nothing and
  sending nothing. Each native pane keeps the rule over the page it
  draws, cut to the scripture its verbs take: a selection holds Scripture
  when it holds a letter or figure of a verse's words, after the verse's
  number and before its end, which leaves out verse numbers, headings, an
  omitted verse's mark and, since `ca86cca13`, the verses' punctuation. A
  selection that begins in a heading, with no character of a verse's
  words before the first of the heading's words it holds, does not count
  a word it ends inside, which reaches back to the white space or the
  closed-up dash before it, as Go's end repair cuts back: Go quotes
  nothing for a drag from a heading that ends inside the first word of
  the verse below, "Solomon’" of "Solomon’s" or "Well-" of "Well-being"
  among them, or takes only the punctuation that verse opens with. One
  begun in the verse above, its closing punctuation among the rest, keeps
  that word, as Go quotes it (`btIOSSelectionHoldsScripture` over the
  verse index, `btMacSelectionHoldsScripture` over the font runs and
  heading paragraphs, `BtBridge.selectionHoldsScripture` over the verse
  index and `isHeadingParagraph`). Before `ca86cca13` any character of a
  verse's words that was not white space counted until the first of a
  heading's words, so a mark selected alone and a drag from the verse
  above's closing punctuation into the heading were offered the verbs, as
  Go offered them. Before `0f9ca1d57` the rule stepped back over
  letters and figures only, and counted only them in the whole of a
  selection that held a heading's words: it offered Study with AI for
  "Solomon’", which the dispatch then dropped, and hid it for a drag from
  the verse above's closing full stop into the heading, which Go studies.
  The iPhone and iPad edit menu and the Mac context menu ask it when they
  are built. Android's toolbar, built once per selection, is built to show
  or hide the item each time it is prepared again (`onPrepareActionMode`),
  which no device has yet shown as the handles move, and asks again when
  the item is chosen. A selection that quotes words is studied as before.
- The prompt's passage (`e86fc00be`) is the share's quotation of the same
  selection, before the share frames it, in the form a machine reads
  (`aiPassageFor`, ai.go; `selectionQuotationIn`, share.go, which
  `shareQuoteIn` frames): a heading's or a psalm title's words a drag
  runs across come out by the share's own repairs (`cutOpeningMatter`,
  `stripHeadings`, `removeWholeHeadings`, `verseEndBeforeHeading`, the
  title's `trimLeadingHeading`), and with them the verse numbers the
  selection carries, as the citation names the verses; a word cut short
  is trimmed back, and the line breaks are the chapter's. The prompt put
  the selection inside its quotation marks as it came: in the BSB,
  "waters." through "The First Day" into "And God said," went to the
  provider with the heading and verse 3's number inside it, as Genesis
  1:2–3; it now quotes "waters." and "And God said" a paragraph apart.
  `dispatchAIAction` keeps the divine name's small capitals as drawn
  (`sharedText`) until `aiPassageFor` has read the passage, and writes
  them as capitals for the provider after: written as capitals first, on
  a chapter that sets the name in small capitals, as the NKJV does, a
  drag across a heading was not found in the chapter and kept the
  heading's words in the prompt and the study panel, and one begun inside
  the heading was cited by the verse above as well. The study panel's
  preview is the selected words, as before, and shows the name as the
  page draws it. Quoted without the share's framing, the passage differs
  from the chapter's words in two ways the framing covers in the share.
  Clause punctuation a selection of words ends on, a comma, a semicolon,
  a colon or a dash, comes off for the share's closing ellipsis and is
  sent with no ellipsis in its place: "gods;" goes as "gods"
  (`TestAIPassageRefAgreesWithShareCitation`), and a whole verse that
  ends on one, as 11,837 do over the WEB, BSB and WEB Catholic caches
  (3,935, 3,497 and 4,405), goes without it, where `da90dfb55` sent each
  with it. And a selection that stops just short of a sentence's end is
  completed with the sentence's full stop, read past a closing quotation
  mark (`completeTrailingSentenceProse`) that the prompt then leaves
  out, where the share's balancing closes the quotation: in the WEB and
  the WEB Catholic a drag from the middle of Isaiah 62:11 that ends after
  "Redeemed" in 62:12 sends "Redeemed." with the quotation before it left
  open, where the chapter reads "Redeemed”.". Of 91,919 drags from the
  middle of one verse into the middle of the next with no heading between
  (WEB 29,888, BSB 28,003, WEB Catholic 34,028), those two are the only
  ones whose prompt is not a stretch of the chapter's words. A drag from
  the closing punctuation of the verse above into a heading, ending
  there, was studied and sent that punctuation alone, while the panel
  showed the words selected, the heading's among them, and a mark of a
  verse's words selected alone inside the verse was studied and sent that
  mark, as on `da90dfb55`; since `ca86cca13` neither is offered a study
  verb (the next item). One rule on all six; the web reader has no
  selection verbs.
- No study verb for punctuation alone (`ca86cca13`). Go studies a
  selection only when the share's quotation of it, which is the prompt's
  passage (`selectionQuotationIn`), holds a letter or figure
  (`aiStudyOffered`); a text with no letter or figure is refused before
  it is read, with a Bible or without. The desktop menu asks that rule,
  and `dispatchAIAction` drops a study verb for a selection that fails
  it. Each native pane counts only a letter or figure of a verse's words
  (above), and with no verse index asks for a letter or figure anywhere
  in the selection, where it offered the verbs for any selection. A drag
  from the closing punctuation above a heading into the first word below
  is studied only once it contains a complete word. Through that word,
  its prompt quotes the word; ending inside it trims the unfinished word
  and leaves no studyable fragment. Before the boundary fix above, Go's
  fallback restored the full stop and the partial word, and the native
  panes kept that word too. Share and Cross-references keep their places.
- A verse number selected on its own where the verse's words hold the
  same figure is read as the number: the pane's empty lead says the
  selection begins on it (`placeByLead`, share_lead.go). Searched for
  among the words, the BSB's Genesis 5:3 number was found in "130" and
  quoted as "3", and the menu offered the study verbs for it. A piece of
  a number is read as the number too (`0f9ca1d57`): its first digits with
  that empty lead, the digits of a drag begun and ended inside it, and a
  number begun in the white space after the verse above, which the Apple
  and Android panes send with a lead that holds that verse, and which
  cites the next verse (`leadPlacement.verse`). The first digit went out
  as a figure quoted as Scripture, with the study verbs offered, the
  Windows and Linux pane's among them; a number begun after the verse
  above was quoted as the figure and cited by both verses.
- A drag from a heading that ends inside the number of the verse below,
  before its last digit, is answered as the heading is: the verse below
  cited and nothing quoted (`openingMatterIntoNumber`, share.go,
  `e5389ed3d`). Its digits are no verse number to confirm the heading by,
  and the heading's words were quoted as the verse's: "The Fourth Day 1"
  as Genesis 1:13–14, the WEB's "GIMEL 1" as Psalm 119:16–17. The native
  panes' rule already offered no study verb for it; Go's menu did.
- A text share that quotes nothing is the citation line alone
  (`composeShareText`, `99dbcd728`), where it was that line under two empty
  lines.
- Share with note sent with a note on such a selection is anchored to the
  number of the verse below (`wordsAnchor`, `ca1f500a6`), which the Mac and
  iPad panes find again once the page has put the note's card in, as they
  find a selection's words. With no lead it had no anchor and presented at
  the selection as it stood; on the iPad the page moved to the card, the
  selection was gone and the heading out of sight, and the popover hung
  from the top of the reading view.
- A heading above verse 1 stays outside the app's verbs on those panes, as
  the psalm title does (each pane's content start), and is not changed.
- The Windows and Linux pane draws a heading outside its text model
  (`appendHeadingLines`), so no word of one can be selected; but the model
  offset under a heading's line is the end of the verse above, and a
  double-click there selected that verse's last word, which the verbs then
  quoted and cited by the verse above. `DoubleTapped` now selects nothing on
  a heading's line.
- The web reader has no selection verbs and is not changed.

**Tests.** heading_word_selection_test.go, on a page drawn as the native
panes draw it with their rule modelled on it (`paneHeadingSpan`):
`TestAWordSelectedInAHeadingMeansTheSectionBelowIt` (invented verses: each
word of a heading, the heading whole, a drag begun inside a word and the
break on either side, through every verb and Share with citation's
message; the same words selected in the verses above and below, the
controls, are quoted and cited where they stand),
`TestASelectionThePaneFoundInAHeadingIsItsVerseAlone` (Go's half and
`inHeading`), `TestAShareThatQuotesNothingIsItsCitationLineAlone`, the
heading cases of `TestShareWithNoteIsAnchoredToTheSelectedWords`, and,
opt-in by `BIBLETEXT_EXHAUSTIVE=1` over the local caches,
`TestEveryWordOfEveryHeadingMeansTheSectionBelowOverLocalCaches_LocalOnly`,
whose answers include that no study verb is offered, by the desktop menu
or by the panes' rule modelled on the page (`paneHoldsScripture`). The
follow-up's, in no_scripture_study_test.go:
`TestNoStudyVerbIsOfferedForASelectionThatHoldsNoScripture` (the menu,
for each word of a heading, the heading whole, a verse number and the end
of one, a psalm's title alone and run on to verse 1's number, and verse
1's number alone; the controls, words of the verses and a drag from the
heading into the verse below, are offered it),
`TestNoStudyVerbIsSentForASelectionThatHoldsNoScripture` (every verb,
Ask among them, dispatched for the same selections opens nothing and
reaches no request, with a key set and the provider allowed),
`TestTheStyledPaneOffersNoStudyVerbForAVerseNumberAlone` (a double-click
on a number on the Windows and Linux pane),
`TestAVerseNumberAloneIsTheNumberWhereTheVerseHoldsTheFigure`, and the
opt-in `TestNoStudyVerbForANumberOrATitleOverLocalCaches_LocalOnly`;
no_scripture_study_contract_test.go holds each pane's rule and menu, with
controls that mutate the source. Each fails on `rel/1.2.23` at
`3261a1e13`. With `0f9ca1d57` and `e5389ed3d`:
`TestThePanesRuleAgreesWithGoBesideAHeading` (a drag from a heading to
every point of the first two words below, over openings with an
apostrophe, a hyphen, a closed-up dash, an ellipsis and a space, and an
opening quotation mark, and drags from the closing punctuation above into
the heading and on: the panes' rule answers as the menu does, the one
shape held apart excepted), `TestAPieceOfAVerseNumberIsTheNumber`,
`TestTheStyledPaneReadsAVerseNumbersFirstDigitAsTheNumber` and
`TestAHeadingRunOnIntoPartOfTheNextNumberQuotesNothing`, which fail on
`6ef8ae9e3`, the first with the panes' rule modelled as it was there; the
contract test holds the new step back and count, with a control for each.
With `ca86cca13`: the menu, dispatch and model tests above take a
verse's closing full stop and a comma selected alone, and drags from the
full stop above the heading that end in it or on the number below, with
a drag from that full stop into the verse below as a control;
`TestThePanesRuleAgreesWithGoBesideAHeading` drags from the closing
punctuation above to every point of the first two words below, offered
the verbs by both once the drag takes a letter or figure; and the
contract test holds each pane's count of letters and figures, its step
back for a selection begun in a heading alone, and its test with no
verse index, with a mutation control for each. On `91e1ec683` the menu,
dispatch and agreement tests and the contract test fail.
A host build of the two Apple rules as the source has them, on a page set
as the panes set it, answers as the model does: on `6ef8ae9e3` both
offered the verbs for "Solomon’" and for "Well-" through "Well-bein",
and none for the full stop above the heading; now neither offers them for
the first and both for the second.
heading_word_selection_contract_test.go holds each pane's test and every
seam the flag crosses, with controls that mutate the source. The prompt's
passage, in ai_passage_heading_test.go:
`TestAStudyPromptLeavesOutTheHeadingADragRunsAcross` (Explain dispatched
with a key and the provider allowed, the prompt read where the provider
receives it: drags from the last word above into the first words below,
the verses above and below whole, begun inside a heading's last word,
from the closing punctuation above, from the last word above into the
heading, across a stack of two headings and from the stack into the
verse below, each on a chapter with the divine name in small capitals
too, which the panel must show as drawn; the controls are the share's
messages of the same drags, which hold none of the headings' words and
cite what the prompt cites), `TestAStudyPromptLeavesOutAPsalmTitleADragRunsFrom`
(a psalm's title, with a heading under it and without, run on into the
psalm with no span), and `TestAIPassageRefAgreesWithShareCitation`, which
now holds the prompt's passage equal to the share's quotation as well as
the citations, over its earlier selections and drags across a heading, a
stack of two and a psalm's title. Each fails on `da90dfb55` with the
share's quotation set apart as `selectionQuotationIn` (the share
unchanged); with only the dispatch's reading form put back, the small
capitals cases fail. `TestAIPassageSendsTheShareQuotationAndCapturesItsDisplay`
and `TestAskCapturesSelectedPreviewBeforeNavigation` take the passage
without the verse number, and `TestRunAIActionCacheKeyCarriesVerseSpan`
files its answers under the passage the prompt quotes. Opt-in, `TestAStudyPromptQuotesNoHeadingOverLocalCaches_LocalOnly`:
over the WEB, BSB and WEB Catholic caches, drags across the headings at
every place between two verses that holds them (21, 1,894 and 23; the
BSB's 1,894 hold 1,896 headings, a stack of two at two of them) as the
native panes hand them over and from every psalm title into its psalm
with no span (116 each), 284, 15,259 and 300 in all, each offered the
study verbs, send the share's
quotation, which stands in the chapter's verses as one stretch and keeps
the verses' words the drag holds, with no heading's or title's word; on
`da90dfb55` every one quoted otherwise, and 263, 13,374 and 277 held the
headings' or the title's words. On `91e1ec683` the 21, 1,894 and 23
drags from the closing punctuation above into the headings are studied
and send that punctuation alone, and the study panel's preview of each
shows the heading's words; since `ca86cca13` they are offered no study
verb, and the sweep holds that no drag it studies sends punctuation
alone (263, 13,365 and 277 studied). A probe outside the tree measured
the same of a mark of a verse's words selected alone inside the verse,
with the span the panes send, with `e86fc00be` and on `da90dfb55` alike:
the menu and the panes' rule modelled on the page offer it the study
verbs, and its prompt quotes that mark alone, for each verse's closing
punctuation, 96,992 selections (WEB 31,000, BSB 30,727, WEB Catholic
35,265), and for every other mark of a verse's words, 326,721 more
(112,846, 89,088 and 124,787). Opt-in,
`TestNoStudyVerbForPunctuationAloneOverLocalCaches_LocalOnly` holds them
since `ca86cca13`: no study verb, by the menu or the panes' rule
modelled on the page, for those 96,992 closing marks and 326,721 other
marks, each selected alone, and for the 1,938 drags from a closing mark
into the headings below; the same drags run on through the first word
below are offered the verbs, and their prompts quote that word. On
`91e1ec683` the menu offers the verbs for all 425,651 punctuation
selections.
`TestADoubleClickOnAHeadingSelectsNothingOnTheStyledPane` holds the Windows
and Linux pane. The seam tests (`TestEverySelectionDispatchPassesTheLead`,
`TestTheNativeExportsBuildTheSpanWithItsChapter`,
`TestTheSelectionVerbsSendTheSpanAndLeadTheInstallGives`) take the new
argument.

The sweep, over the WEB, BSB and WEB Catholic caches: every word of every
heading between two verses (23 words under 21 headings, 7,373 under 1,896,
113 under 23) and every such heading whole is cited by the verse below with
nothing quoted, through every verb. With the span the panes sent before,
the verse above and a lead through the heading, 2,023 of the BSB's words
under 1,301 headings and 27 of the WEB Catholic's under 2 were cited by the
verse above, and 3,903 and 38 were quoted; the WEB's none. On `be1440fe0`
with only the flag's plumbing added (selection_span.go), the sweep and the
two Go tests above fail (BSB 9,267 of its 9,269 selections answered
otherwise), as do the contract tests and the double-click test. On
`005c19143`, before `99dbcd728`, every one of the sweep's 9,449 selections
(WEB 44, BSB 9,269, WEB Catholic 136) sent the selected words as the
prompt's passage. With the Daniel 3 heading set apart the sweep holds
9,370 (WEB 44, BSB 9,269, WEB Catholic 57); on `3261a1e13` the menu
offers every one of them the study verbs, and with the follow-up none.
The follow-up's own sweep, as `e5389ed3d` widens it to every chapter,
offers no study verb for 97,563 verse numbers selected alone (WEB 31,098,
BSB 31,086, WEB Catholic 35,379), 159,878 pieces of numbers (their first
digits, a number begun after the verse above, and a drag from a heading
into the first digits of the number below, each of which also shares
nothing quoted) and 696 psalm-title selections (232 in each), and answers
36,489 drags beside a heading as the rule says, the menu and the panes'
rule alike, 7,253 of them offered none. On `6ef8ae9e3`, 5,155 of them are
answered otherwise. With the panes' rule modelled as it was there, it
answered 7,693 of the 28,737 drags from a heading into the first two
words below and from the punctuation above otherwise than the menu: 2
offered a verb the dispatch then dropped, at the BSB's 1 Chronicles 3:10
and 27:32, and 7,691 offered none that Go studies, all but the 2 below
from the punctuation above. Since `ca86cca13` the sweep offers none for
the drags from the closing punctuation above that end in the headings or
on the number below, or take only the punctuation the verse below opens
with: 13,105 of the 36,489 are offered none (WEB 137, BSB 12,817, WEB
Catholic 151), and on `91e1ec683` 5,852 are answered otherwise. Two
drags are measured apart and not held,
both at the BSB's Ephesians 3:14: from the heading through the ellipsis
and space that verse opens with into one or two letters of its first
word, which the panes' rule offers no study verb and Go would study as a
fragment. For each of the two the share quotes the heading's words with
the letters, as Scripture under Ephesians 3:13–14 ("Paul’s Prayer for
the Ephesians ... f"), and Go's prompt would quote the same; a drag on
through the whole first word quotes "for" alone under 3:14.

A host probe, not in the tree, compiled the Mac pane's preamble as it
stands and the iOS verse-index functions read with AppKit's fonts, imported
each of the 805 chapters holding those 1,940 headings from its own HTML
(`buildChapterHTML`) through the Mac pane's importer, and selected each
word of each heading, the heading whole and the heading with the break
before or after it: 13,329 selections. Both tests named the verse below for
all of them, where the span each pane sent named the verse above for all of
them; the first word of the verse above and of the verse below, 3,880
selections, were taken for a heading's by neither.

**Device checks, 15 and 16 October 2026, on `005c19143`.** Throwaway
triggers in scratch copies, each build against `rel/1.2.23` as the control.
On the Mac app, the iPhone 16 Pro and iPad Air 11 simulators and the
Android emulators (API 35 and 30), words selected in the BSB's headings
"The Descendants of Cain" and "The Fourth Day" were answered by Genesis
4:17 and 1:14 with no words through every verb tried, where the control
quoted "Cain" from 4:16 and "The" from 1:13; "Cain" selected in verse 16
was quoted from 4:16 on both. On the Linux and Windows virtual machines a
double-click on a heading's line selected nothing, where the control
selected the last word of the verse above. The study check read the
panel's citation and words, not the prompt the provider was sent, and the
text share then had two empty lines above the citation.

**Device checks, 21 October 2026, on `99dbcd728` and `ca1f500a6`.** The
same triggers, now logging the study prompt's passage, each build against
`005c19143` as the control, on the Mac app and the iPhone 16 Pro and iPad
Air 11 simulators. For "Cain" in "The Descendants of Cain" and "The" and
"Fourth" in "The Fourth Day", Share with citation sent the citation line
alone ("— Genesis 4:17 (Berean Standard Bible)", "— Genesis 1:14 (…)")
and Explain's prompt read the text of Genesis 4:17 or 1:14 as the
passage; the control sent the same line under two empty lines and the
passages "Cain", "The" and "Fourth". "Cain" selected in verse 16 was
quoted from 4:16 and sent as the passage under 4:16 by both. Share with
note with a note typed on the heading's "Cain": on the iPad, before
`ca1f500a6`, the popover hung from the top of the reading view, as above;
with it, beside verse 17's number. On the Mac the page did not move, the
selection stayed on the heading's word, and verse 17's number, which the
picker is now anchored to, was in view.

**Device checks of the follow-up, 21 October 2026, on `6ef8ae9e3`.**
Throwaway triggers in scratch copies, against `rel/1.2.23` at `3261a1e13`,
in the BSB, with a stand-in key and the study request and the text share
logged, not sent. The pane's own menu was built over each selection. On
the Mac app and the iPhone 16 Pro simulator (its menus also put on
screen), and for most of the cases on the iPad Pro 13-inch simulator:
"Cain" in "The Descendants of Cain", "The Fourth Day" whole, verse 17's
number and a drag from the heading into one letter of verse 17 got no
Study with AI, Share and Cross-references kept, where the control offered
it; Psalm 3's title the system's verbs alone on both; Genesis 5:3's number
shared the citation line alone, where the control quoted "3"; a drag from
the heading through "And" and "Cain" in verse 16 were studied by both;
and a study verb handed to the dispatch for the heading's "Cain" opened
nothing, where the control opened the panel on Genesis 4:17. On the
Windows and Linux pane, in the Mac's Linux mimic and the Linux and Windows
virtual machines, where a heading or a title cannot be selected: verse
17's number and Genesis 5:3's number as above, and "Cain" in verse 16
studied by both.

**Device checks, 22 October 2026, on `e5389ed3d`.** The same triggers with
eight cases more, against `6ef8ae9e3`, on the Mac app, the iPhone 16 Pro
and iPad Pro 13-inch simulators, the Mac's Linux mimic and the Linux and
Windows virtual machines. On the Mac, the iPhone and the iPad: drags from
the headings over 1 Chronicles 3:10 and 27:32 that end at "Solomon’" and
"David’" got no Study with AI, where the control offered it and its
Explain opened nothing; ending at "Solomon" both studied. Drags from
Genesis 1:2's closing full stop into "The First Day", and on into "An" of
verse 3, were offered Study with AI, and the first studied under 1:2,
where the control offered none. "The Fourth Day" run on into
the first digit of verse 14's number, the first digit of Genesis 5:16's
number, and verse 3's number begun after verse 2 got no study verb on
either and shared the citation line alone, under 1:14, 5:16 and 5:3,
where the control quoted "The Fourth Day 1", "1" and "3" (the last under
5:2–3). On the Windows and Linux pane, in the Mac's Linux mimic and the
Linux and Windows virtual machines, the first digit of 5:16's number got
no Study with AI and shared the citation line alone, where the control
offered Study with AI and quoted "1". The first pass's cases gave the
same answers as before: all of them on the Mac and the iPhone, five on
the iPad, and on the Windows and Linux pane verse 17's number, Genesis
5:3's number and "Cain" in verse 16.

**Device checks of the prompt's passage, 22 October 2026, on
`a244cb2ed`.** Throwaway triggers in scratch copies, against `rel/1.2.23`
at `da90dfb55`, with stand-in AI keys and the study request and the text
share logged, not sent. On the Android emulators (API 35 and 30), through
the toolbar's own Study with AI and its popup, the iPhone 16 Pro and iPad
Pro 13-inch simulators, through the pane's edit menu, and the Mac app,
through its context menu, in the BSB: Explain for a drag from "waters."
across "The First Day" into "And God said" sent "waters." and "And God
said" a paragraph apart under Genesis 1:2–3, where the control sent the
heading and verse 3's number between them; a drag across "The
Descendants of Cain" (Genesis 4:16–17), one begun inside that heading's
last word, one across the two headings above Proverbs 22:17, one from
Psalm 3's title into verse 1 and one from John 3:16 into 3:17 sent the
verses' words alone, where the control sent the headings' words or the
verse numbers; Ask with a one-line question and Compare quoted the
first drag as Explain did; and a drag from Genesis 1:2's full stop into
"The First Day" was offered Study with AI on both builds and sent "."
under Genesis 1:2, where the control sent the heading with it, the
panel showing ". The First Day" on both. The
shares of the same drags and the panel's words and citation were the
same on both builds. On the NKJV, on the Mac and the iPhone simulator, a
drag from the end of Genesis 2:7 across the chapter's one heading into
the divine name in 2:8 sent none of the heading's words and the name in
capitals, cited Genesis 2:7–8 as the share does, and the panel showed
the name in small capitals; the control sent the heading's words, and
its panel showed them, with the name in capitals. The Windows and Linux
pane was run only in the Mac's Linux and Windows mimic: the drags from
John 3:16 into 3:17 and from Genesis 4:16 into 4:17 sent no verse
number, where the control sent "17". The virtual machines were not run,
and no case was on the WEB.

**On the Android emulators**, on 22 October 2026 at `da90dfb55` against
`9a308352d`, on API 35 and API 30 with real long presses on the BSB, the
toolbar read from the action mode's menu and in screenshots: no Study
with AI for "Cain" in the heading "The Descendants of Cain", for verse
17's number alone, for "Psalm" in Psalm 3's title or for Genesis 5:3's
number alone, whose Share with citation sent the citation line alone,
"— Genesis 5:3 (Berean Standard Bible)", where the control offered
Study with AI for each and its share quoted "3"; and "Cain" in verse 16
kept Study with AI on both. On API 30 a selection moved in code under a
toolbar that was up, not dragged by its handles, hid Study with AI on
the heading's words, showed it once it reached verse 17 and hid it
again, where the control kept it throughout.

**Still to do.** That the toolbar's item follows the handles as a finger
drags them on Android, which depends on the toolbar being prepared again
as they move. For `ca86cca13`, against `rel/1.2.23` at `91e1ec683`: the
device checks on every pane, a closing full stop and a comma selected
alone, drags from Genesis 1:2's full stop into "The First Day" and
through verse 3's number, and the controls run on into "And". The share
and the prompt of the two drags at the BSB's Ephesians 3:14 above, which
quote the heading's words. For the prompt's passage (`e86fc00be`), against
`rel/1.2.23`: the Linux and Windows virtual machines, where it was seen
only in the Mac's mimic, and a drag across one of the WEB's headings on
any pane. The follow-up was merged into `rel/1.2.23` on 22 October
2026, and the prompt's passage the same day (`a04829c5f`). docs/SELECTION_OFFSET.md
describes the heading flag in every native seam, and
docs/PLATFORM_MATRIX.md records divergence 36: on Windows and Linux a
heading's words cannot be selected and a double-click on one selects
nothing. Not changed, and open for the major's headings
pass (docs/ROADMAP.md, Next major version, row 5): a heading above verse 1,
which gets no app verbs on the native panes, and the Windows and Linux
pane, where a heading's words cannot be selected at all.

### AND-4. A link to a verse the edition omits opens at the top — FIXED 14 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry).
Whether the mark itself lights, and where a note on the hole is drawn, were
decided on 15 October 2026 (decision 18), built for 1.2.23 on
`fu/1223-omitted-verse-mark` and merged into `rel/1.2.23` on 21 October
2026 (the end of this entry); of the three rules the build added, two were
confirmed that day and one rejected, and taken out on
`fu/1223-gap-marks-named` — see docs/ROADMAP.md, 1.2.23, row 7.

A link to a verse the open edition leaves out (Matthew 17:21 in the BSB)
should open beside the "[21]" mark at verse 22. Android goes to the top with
nothing lit on an exact `verseRange` miss (`android/BtBridge.java:3452`) for
a range whose first verse is the omitted one, and `share_link_open.go:314-320`
does not move a single omitted verse to one that exists, on any surface.

Reachable only by an `/nkjv/` link to one of the 16 verses the NKJV has and
the BSB leaves out, opened by a reader without an NKJV key (with one, the app
switches to the sender's edition). All 58 omissions in the table are interior.

Fix, 1.5 days: resolve the arrival verse once in Go (the walk `gapsBefore`
does) for all six; Android takes the first verse after the omitted one on a
miss; the website scrolls the `.vg` mark. `reading_legacy_band_test.go:14-36`
pins the legacy pane's behaviour as not decided.

Open, apart from the fix, until decision 18 (15 October 2026, below): whether
the omitted verse's mark is lit when a mark covers that verse. In 1.2.22 every
page tied the mark to the verse after it, drawn in that verse's tint or
outside its highlight (`android_chapter_html.go:172`, :193-194;
`reading.go:946-947`; `reading_styled_layout.go:553-556`;
`cmd/websitegen/render.go:265-266`, as they stood in 1.2.22), so a one-verse
mark on an omitted verse lit nothing on any surface. "Headings: quoting, notes
and links need one deliberate pass" recorded the neighbouring mismatch, a gap
mark before a verse washed by the markup but not by a single-verse mark's
native range, which the 1.2.23 change below removes.

**FIXED 14 October 2026, for 1.2.22** (`24e9bb0ec`; ROADMAP.md, 1.2.22, row
15). Arriving at 21, Android's bridge found no row and opened the chapter at
the top with nothing lit, and so did the canvas pane and the website; the
Apple panes had no line for it either, and Android did the same for a link
to 21–22, whose verse 22 the other surfaces lit. Of the 16 New Testament
holes a link can name, the BSB omits all 16, the WEB and the WEB Catholic 4.
The chrome now resolves the arrival verse once (`drawnArrivalVerse`,
`notes_arrival.go`): the verse itself where the chapter has it, otherwise the
first verse after it, where `gapsBefore` files the hole's mark. Android
pushes that verse, the canvas pane falls back to the arrival verse's line
when the mark lights none, as the Apple panes already fall back to it, and
the web reader lands an unlit link on the page's next verse. A verse past
the chapter's last is left as named. `TestALinkToAnOmittedVerseArrivesAtTheVerseAfterTheHole`,
`TestTheCanvasPaneLandsALinkToAnOmittedVerseOnTheVerseAfterIt` and
`TestTheWebPageLandsALinkToAnOmittedVerseOnTheVerseAfterIt`. The website's
half reaches readers with the next `scripts/publish-site.sh`. Seen on the
emulator, below. Whether the mark lights was left for decision 18.

In 1.2.22 a note sent on the omitted verse was the exception. Its card's own
verse was missing too, so every surface drew the card at the chapter top, and
moving the arrival on to verse 22 scrolled the note out of view on every app
surface, while the website still landed on it. Where the page omitted both
the arrival's verse and the open card's own verse, the arrival placed
nothing, as it did before the fix, and Android pushed no verse
(`arrivalVerseOnPage`, `notes_arrival.go`; the test then named
`TestANoteOnAnOmittedVerseKeepsItsCardInView`, and
`TestTheCanvasPaneOpensANoteOnAnOmittedVerseAtItsCard`). A note on a range
that starts at the hole is filed by the chapter's plan at the first verse the
page has, so its card stands above that verse's paragraph and the arrival
lands on its band. Only a note shown from the link alone (filed on another
passage, or refused by the store) kept the named verse, and then the Apple
panes and the website drew its card above the first lit verse's paragraph and
Android and the canvas pane at the top; PLATFORM_MATRIX.md recorded it as
divergence 34. Drawing such a card in one place on every surface was left for
decision 18. For 1.2.23 such a card, and the card of a note on the omitted
verse alone, hangs above the verse after the hole on every surface and the
arrival lands on it (the 1.2.23 section below;
`TestANoteOnAnOmittedVerseArrivesAtItsCard`, the 1.2.22 test renamed with
the rule).

Seen on 14 October 2026 on the Android emulator (Pixel 7, API 35), with
development builds of the branch before this exception and after it, the
reader on the WEB, which omits Acts 8:37, and the NKJV switched off: links to
`/nkjv/acts/8/#v37`, `#v37-38` and `#v38` land on verse 38 in both builds,
Acts 8 ending four verses later, so the view stands at the end of the
chapter; a note link on 8:37 alone lands on verse 38 with its card parked out
of view above it before the exception, and at the top of the chapter with the
card in view after it; a note link on 8:37–38 is filed at verse 38 and lands
on its band in both. Seen beside it, older and not changed: the bridge parks
such a card over the chapter's first line, where the iPhone and iPad reserve
an inset for it (the parked sticker reserves no band, `refreshNoteSticker`).

**Decided 15 October 2026 (decision 18), built for 1.2.23** (`a86fcd9c2` and
`67c199afc`, on `fu/1223-omitted-verse-mark`). As recommended: the mark lights
when a link or a note names the verse it stands for, and a note shown from its
link alone on a range that starts at such a verse is drawn above the verse
after the hole on every surface, which closes divergence 34.

Three rules in the build went beyond what the decision names. On 21 October
2026 the first and the third were confirmed as built, and the second was
rejected:

1. Confirmed. A note on the omitted verse alone follows the rule for a range
   that starts there: its card hangs above the verse after the hole, where in
   1.2.22 every surface parked it at the chapter top. Keeping it at the top
   would need an exception on the Apple panes once the mark lights.
2. Rejected. The build kept the mark tied to the verse after it as well, as
   every surface but the Apple panes drew it in 1.2.22: a link to 17:22 lit
   "[21]" with verse 22, and where holes stand together a link to the one
   verse after them lit every mark: in the WEB Catholic Edition a link to
   Sirach 26:28 lit the nine marks "[19]" to "[27]" with verse 28 as one
   band, where in 1.2.22 the iPhone, iPad and Mac left them bare. A mark now
   lights only when a link or a note names its own verse (below).
3. Confirmed. No verse's narration lights a mark (below). In 1.2.22 the
   narration of the verse before a hole lit its mark on the iPhone, iPad, Mac
   and Android, and the narration of the verse after it did on the Windows
   and Linux pane.

- **The mark.** It takes its own tint (`gapOf`, `tint.go`): lit only when the
  span names the omitted verse. A link to Matthew 17:21 lights "[21]" alone.
  A span ending at the hole (17:20–21) lights verse 20, the space and the
  mark as one band (`joinsAhead`, `writeGapMarks`); 17:21–22 and 17:20–22
  draw as before. A span on the verse after the hole alone leaves the mark
  bare (choice 2, rejected; `2aa12b083`, on `fu/1223-gap-marks-named`): a
  link to 17:22 lights verse 22, one to Sirach 26:28 in the WEB Catholic
  Edition verse 28 with its nine marks bare, and one to Acts 8:38 in the WEB
  verse 38, where one to 8:37 lights "[37]". That rule is the same in both
  HTML dialects (`reading.go`, `android_chapter_html.go`), the Windows and
  Linux pane (`layoutChapter`), the Apple panes' runs (`nativeTintRuns`) and
  the website, where the fragment lights the marks it names (`lightMarksIn`,
  `cmd/websitegen/assets.go`) and a space lights only where a lit verse or
  mark stands on both sides of it (`bridgeAfter`); the band lit the marks
  before a lit verse, and those between two, until `2aa12b083`. A note's
  mark lights by the same rule. A note from another edition is filed by the
  chapter's plan on the verses this edition has (`displayCopy`), and its
  mark keeps the holes its own span names (`noteMarkSpan`, `notes_plan.go`;
  `4ce5575e4`), running on through such a hole to the note's next run: an
  NKJV note on 17:21–22 read in the BSB lights the mark and verse 22, one on
  17:20–21 verse 20 and the mark, and one on 17:20–22 verse 20, the mark
  and verse 22, as a link to the same span does. Its card stays where the
  plan files it, above verse 22 for 17:21–22.
- **The Apple panes' wash.** These panes paint from runs of verse numbers
  resolved against the imported text, not from the markup, and a verse ran on
  to the next verse's number, so the mark belonged to the verse before the
  hole. A mark on verse 20 alone washed "[21]". One on 17:21–22 or 17:22 left
  it bare where the markup lit it: the mismatch "Headings: quoting, notes and
  links need one deliberate pass" records. Now `nativeTintRuns` walks each
  mark in its place, the omitted number standing for it, so a span across the
  hole is one run, 20–22, and a link to 21 is the run 21–21. Both panes index
  the marks apart from the verses (`gGapIndex` and `btGapMarkNumber`,
  `reading_ios.go`; `btMacGapMarkNumber` and `btMacGapRange`,
  `reading_macos.go`), end a verse at the mark after it, and resolve an
  omitted number in a run to its mark (`btIOSRunSpanRange`,
  `btMacRunSpanRange`).
- **The plain fallback page** (`appleFallbackPageOf`, shown when a chapter
  fails to import three times) draws the mark at body size, so neither pane
  indexes it there and 1.2.22's rule holds on that page: a run from the
  omitted number starts at the verse after it, a link to the omitted verse
  alone lights nothing, and where no heading stands between the mark and the
  verse before it, a mark or the narration on that verse alone runs on over
  the mark ("[37]" with Acts 8:36 in the WEB). By the code; not seen on a
  device.
- **The narration.** No verse's narration lights a hole's mark on any
  surface (choice 3): the edition has no text for the omitted verse, so
  nothing is read there. The Apple panes end a verse at the mark after it
  (above). Android's import sizes the mark with a span class of its own
  (`GapMarkSpan`, `android/BtBridge.java`), and `buildVerseIndex` ends a
  verse at the first mark after its number (`endAtMark`), where the verse
  before the hole ran on to the next verse's number. The Windows and Linux
  pane files the mark's run under the verse after the hole, and its
  narration band now skips that run (`verseSpansForLayout`).
- **The note.** The chrome hands every pane the verse the page draws for the
  note (`drawnNoteAnchor`, `notes_arrival.go`): verse 22 for a note shown from
  its link alone on 17:21 or on 17:21–22. The iPhone, iPad, Mac and Android
  push that verse in place of the note's own, the canvas pane opens its band
  above it, the website hangs the note above the paragraph of the verse after
  the hole (`fragmentParagraph`), and every arrival lands on the card's band.
  A note on the omitted verse alone, which every surface parked at the chapter
  top while its mark lit nothing, follows the same rule, above its mark
  (choice 1). A card past the chapter's last verse still parks at the top,
  the arrival placing nothing (`arrivalVerseOnPage`).

Tests, each failing on `rel/1.2.23` at `be1440fe0` with the new tests copied
in: `TestAMarkNamingAnOmittedVerseLightsItsMark` (both HTML dialects),
`TestNativeTintRunsReachTheMarkOfAnOmittedVerse`,
`TestTheWashCoversAnOmittedVersesMarkAsTheMarkupDoes` (the Apple panes' rule
modelled in Go over the imported markup, in prose and across a paragraph, in
both layouts, with the rule it replaced shown to disagree),
`TestTheApplePanesResolveARunToAHolesMark` and
`TestEveryPanePushesTheNotesDrawnVerse` (the native source),
`TestTheCanvasPaneLightsTheMarkOfAnOmittedVerse`,
`TestANoteOnAnOmittedVerseHangsAboveTheVerseAfterTheHole`,
`TestANoteOnAnOmittedVerseArrivesAtItsCard`,
`TestTheCanvasPaneOpensANoteOnAnOmittedVerseAtItsCard`, and on the website
`TestTheWebPageLightsAnOmittedVersesMarkAndHangsItsNoteThere` and the
highlight band's run. For the narration, failing on `3918e0e28`, the branch
before it: `TestTheCanvasPaneNarratesNoOmittedVersesMark` and
`TestAndroidEndsAVerseAtAnOmittedVersesMark` (the bridge's source). The
website's half reaches readers with the next
`scripts/publish-site.sh`; its pinned digests moved with `reader.js` in both
states of the next switch. Seen on 15 and 16 October 2026, before
`67c199afc`, against `rel/1.2.23` at `be1440fe0`: on the Mac, the iPhone 16
Pro and iPad Air 11 simulators, the emulator (API 35 and 30), the Linux and
Windows virtual machines and the website built offline, a link to 17:21
lit "[21]" alone, one to 17:20–21 lit verse 20 and the mark, one to 17:22
lit the mark with verse 22 (the rejected rule), and a note link on 17:21 or
17:21–22 hung its card above verse 22 with the arrival on its band, where
the control lit nothing for 17:21 and anchored the note at 21. On the
iPhone and iPad the case for where the verse before the hole ends could not
fail, since a heading stands between verse 20 and the mark in Matthew 17;
the WEB's Acts 8 is still to run there. The narration over a mark
(`67c199afc`) has not been seen on a device. Merged into `rel/1.2.23` on 21
October 2026 (docs/ROADMAP.md, 1.2.23, row 7).

The rejected rule was taken out in `2aa12b083`, on `fu/1223-gap-marks-named`
from `rel/1.2.23` at `3261a1e13`, merged into `rel/1.2.23` on 22 October
2026 (`bf24fb6a3`). Tests, each failing on
`3261a1e13` and passing after: `TestAnOmittedVersesMarkLightsOnlyWhereItsVerseIsNamed`
(Matthew 17:21, 17:20–21 and 17:22 in the BSB, Sirach 26:28 in the WEB
Catholic Edition, and Acts 8:37 and 8:38 in the WEB, on both HTML dialects,
the Apple panes' wash modelled over the imported markup and the Windows and
Linux pane), the 17:22 cases of `TestAMarkNamingAnOmittedVerseLightsItsMark`,
`TestNativeTintRunsReachTheMarkOfAnOmittedVerse`,
`TestTheCanvasPaneLightsTheMarkOfAnOmittedVerse` and
`TestTheWashCoversAnOmittedVersesMarkAsTheMarkupDoes`, and on the website
the highlight band's run (Sirach 26:28, Acts 8:38, 17:22 and 17:22–23
leaving the marks bare) and the `#v5` case of
`TestTheWebPageLightsAnOmittedVersesMarkAndHangsItsNoteThere`. The site's
pinned digests moved with `reader.js` in both states of the next switch.
Seen on 21 October 2026 with development builds of `19b1e1094` and of
`3261a1e13`, on the Mac, the iPhone 16 Pro simulator and the emulator (API
35), each pane's wash read back from the pane: links to Sirach 26:28 in the
WEB Catholic Edition and to Acts 8:38 in the WEB leave the marks bare, where
`3261a1e13` lit them, and links to 17:21, 17:20–21 and Acts 8:37 light the
mark on both.

Taking the rule out left a note from another edition on 17:21–22 with its
mark bare. The chapter's plan files such a note on the verses this edition
has, so its mark lit verse 22 alone, although the note names verse 21. A
note on 17:20–21 lit verse 20 alone, and one on 17:20–22 verse 20 alone,
before the rule was taken out as after. The same spans on a link lit the
mark, in the app and on the website. Fixed in `4ce5575e4`, on the same
branch, merged with it (the mark, above). Test, failing on `19b1e1094` and
passing after: `TestANoteFromAnotherEditionLightsTheMarksItNames`, which
opens each note on a real note link, by a reader without the NKJV
(Matthew 17:21–22, 17:20–21, 17:20–22 and 17:22 in the BSB, and Acts
8:37–38 and 8:38 in the WEB), and the reader's own note on 17:21–22, on
both HTML dialects, the Apple panes' runs and wash and the Windows and
Linux pane. Not yet seen on a device.

Left open, outside the decision:

- In the apps the mark is drawn only with the footnotes on, which is not the
  default (`footnotesEnabled`). With them off a link to an omitted verse
  still lands on the verse after the hole with nothing lit. The website always
  draws the mark.

### RL-NKJV-1 and RL-NKJV-2. The NKJV red-letter generator places a run after its cursor, and retries from the verse's start — FIXED in the generator 14 October 2026; its word check FIXED 15 October 2026; DONE, closed 15 October 2026

Planned for: nothing left; the generator fixed on 14 and 15 October 2026,
and its run on the NKJV after the fix writes the shipped table byte for byte
(the end of this entry) — see docs/ROADMAP.md.

`scripts/gen-nkjv-redletter.py:181` takes the first copy of a run's letters at
or after the cursor, with no word boundary, so a run can land inside a longer
word ("Go" in "Gospel") or on an earlier copy in narration. When that fails,
:182-183 search again from the verse's start, the run always merges into the
previous span (`derive()`, :243-244), and generation reports success where
the script's own contract (:252-256) says it must fail: a marked poem line
ships black.

Latent: reproducing the public-domain tables through the same logic, no
retry fires and 3 of 16,830 run locates have the shape; the realistic
trigger, drift at the run's place with an earlier copy, exists once in 713
later runs (BSB Matthew 13:15). The shipped NKJV table was not checked:
that needs the NKJV opened once (ROADMAP.md, Decisions still open, 43).

Fix, 0.5 day: anchor each run by the parser's running count of non-space
characters and only verify the text; delete the retry; regenerate and require
a byte-identical table, which needs the NKJV opened in the app once. Longer
term, the decoder already walks the words-of-Jesus span
(`apibible.go:934-981`) and could carry it as `Verse.Supplied` and
`SmallCaps` are carried.

**FIXED in the generator, 14 October 2026; the table is not yet regenerated
with it** (`d98440704`, `c9149e93a`; ROADMAP.md, Later minor releases, row
31). The HTML parser keeps
each verse's non-space characters as it reads them, leaving out what the
app's decoder leaves out of a verse (the verse number, headings and the
psalm title's paragraph), and records those before each run; `place` takes
the run at their count, and only when the app's verse has the same
characters before it. The retry from the verse's start is gone. Where the
app's text before a run differs from the markup's, or the run's characters
are not at its count, generation fails, naming the verse and the run's
number, never its words. With `--allow-relocated` such a run is placed
instead where its characters occur once in the whole verse and after the
run before it, and the summary lists it; no copy, two or more, or one
before the run before it fails all the same. An end inside a word fails
generation, and so does a chapter missing from the cache: `derive()` used
to skip it, leaving its verses out of the table, which reads as black
letter. Only `fetch()` reads the key; `--no-fetch` with `--html-cache`,
`--app-cache` and `--out` derives with no key read and no call, and drops
`BIBLE_API_KEY` from the environment unread so that nothing it starts
inherits it.
`TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace` runs it on
invented markup: "Go" in "Gospel", the narration's copy of a run, and the
narration's copy with a heading inside the verse, fail on the derive logic
before the fix (run from a copy of the script with its fetch taken out);
italic and small-capital words nested in a run and a run carried into the
next verse pass before and after. Each refusal in
`TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace` wrote a table
before the fix: a second run the app's verse holds only before the first
(it was folded into the first), a run whose verse differs before it so that
its count falls on a later copy of its words, an end inside a word and a
missing chapter; each is refused with and without `--allow-relocated`.
`TestTheRedLetterGeneratorsPlaceARunByItsOnlyCopyOnlyWhenAllowed` holds the
one-copy placement to the option, and
`TestTheNKJVRedLetterGeneratorKeepsNoKeyWhenItDoesNotFetch` loads the
script with an invented key in its environment and finds it in none of its
globals after a `--no-fetch` run.

**Run on the NKJV, 15 October 2026: no table written; the placement proven,
the check for an end inside a word refuses 18 verses** (ROADMAP.md,
Decisions still open, 43). The NKJV was fetched once through the app's own
path (`fetchAPIBible`, 197 requests: 66 books, 31,102 verses, saved as the
app saves its cache) and the generator's 174 chapters with its own fetch,
both outside the repository and deleted after; nothing below quotes the
text. Run with `--no-fetch` and without `--allow-relocated`, the generator
fails, naming 18 verses and no others: Matthew 5:3, 9:15, 10:17, 10:19,
10:22, 15:8 and 22:44; Mark 7:9; Luke 4:18 and 6:24; Acts 1:8, 9:11, 9:15,
9:16, 11:16, 23:11 and 26:18; Revelation 2:27, each because a run would
begin or end inside a word.

What the run shows, by counts:

- Every run of the 2,054 marked verses is at its count, with the app's verse
  holding the same characters before it: no run differs before it, so none
  needs `--allow-relocated`, and the option would change nothing here.
- The refused edges are 51 run edges at 26 places. At every one of the 26
  the app's verse anchors a note, and the publisher's text has no space on
  either side of the note, so two words run together in the markup fetched
  without notes and in the app's text alike: the run-together class of the
  NKJV's missing spaces (ROADMAP.md, Later minor releases, row 9).
- In 17 of the verses each such place lies between two runs that touch (50
  of the edges): one words-of-Jesus span closes and the next opens where
  the note is, and the two merge into one span, which has no edge there. In
  Matthew 15:8 a run ends at such a place and black text follows (one
  edge), so the span's own end falls between the joined words. The black
  text is one word, the second of the two run together, and the next run
  begins after it: the publisher's markup leaves that word outside the
  words-of-Jesus spans on either side, so the app draws the first of the
  joined words red and the second black. Whether the publisher's printed
  red-letter edition reddens it has not been checked.
- Placed without that check, the runs write the shipped
  `red_letter_nkjv_data.go` byte for byte, all 2,054 verses, and
  `TestNKJVRedLetterTableIsAcceptedByItsOwnGuard`, pointed at the fetched
  cache, accepts all 2,054 entries against that day's text. The shipped
  table is what the fixed placement writes. The control: the table as it
  stood before `a5e0ab738` differs from that output.

The defect is the generator's, not the table's: its check for an end inside
a word takes a place where the source runs two words together for the inside
of one word. Reproduced on invented text with `--no-fetch`: a verse whose
markup holds two touching runs with no space between them, and whose app
text runs the two words together, is refused with and without
`--allow-relocated`, and so is a run followed by black text with no space;
the same two verses with the space present write a table.

**FIXED, 15 October 2026: a place where the app's verse anchors a note is
the edge of a word** (`b7f5cb9a7`). `derive()` reads each verse's note
anchors from the app's cache (`Footnote.Anchor`), and `place` counts each as
the edge of a word, so two runs that touch there make one span and a run may
end there before black text; an edge between two letters or digits with no
note there is still refused. Checking only the merged spans' ends would
still have refused Matthew 15:8, where the run merges with nothing.
`TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace` gains the two
invented verses above, each with a note anchored at the joined place, and
runs with and without `--allow-relocated`: the generator before the fix
refuses both verses in both modes. The fixtures write the notes with the
app's own `Footnote` type. `TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace`
gains the same two shapes with no note there, and the touching runs with a
note elsewhere in the verse, each refused in both modes.

Those cases held the check at a run's end only: a generator that never
checked a run's start, or that let a run's start or its end lie between two
letters wherever the verse had any note, passed them all. Since `7d1950543`
the refusal test has three shapes, two runs touching, a run ending at the
joined place and a run beginning there, each with no note there and each
with a note elsewhere in the verse, and the placement test a run beginning
after black text at a place where a note is anchored, which the generator
before the fix refuses in both modes. Each of those three weaker generators
fails a new case in both modes. With the fix changed to take any note in
the verse for an edge, the three cases with a note elsewhere fail; with the
start check taken out, the two beginning cases fail; with the end check
taken out, the two ending cases fail, as does the end inside a word; with
the check taken out entirely, all six fail, as does the end inside a word.

**Run again on the NKJV after the fix, 15 October 2026: the shipped table
byte for byte; DONE, closed** (ROADMAP.md, Later minor releases, row 31;
Decisions still open, 43). The NKJV was fetched once more through the app's
own path (`fetchAPIBible`, 197 requests, each answered with HTTP 200: 66
books, 31,102 verses, saved as the app saves its cache) and the generator's
174 chapters with its own fetch (174 requests, none refused), 371 requests
in all and no 429, both kept outside the repository and deleted after;
nothing here quotes the text. Run with `--no-fetch` and without
`--allow-relocated`, the generator writes `red_letter_nkjv_data.go` byte for
byte, all 2,054 verses, none placed by its only copy. 51 run edges fall
between two letters or digits, at 26 places in the 18 verses named above,
and the app's verse anchors a note at every one of them; none falls between
two letters or digits with no note there. The control: the generator before
the fix, run the same way on the same text, fails naming the same 18 verses
and no others. `TestNKJVRedLetterTableIsAcceptedByItsOwnGuard`, pointed at
the fetched cache, accepts all 2,054 entries. A correction list for the
missing spaces, if one is approved (ROADMAP.md, Later minor releases, row
9), changes these verses' text and so their entries' fingerprints: the table
is regenerated with it.

### RL-BSB-1. The BSB red-letter generator takes the first copy from its cursor — FIXED 14 October 2026

Planned for: a later minor release, any; fixed on 14 October 2026 (the end
of this entry) — see docs/ROADMAP.md.

`exact_locate` and `normalized_locate` (`scripts/gen-bsb-redletter.py:114`,
:137) search unanchored from the cursor and the first copy wins; the runtime
guard checks only the whole verse's hash, so a wrong copy would ship as right.
Latent: the shipped table is clean, with no run starting or ending inside a
word and no exact ambiguity; regenerated through the normalized path alone,
Acts 9:10's run would move to the narration's copy of its words.

Fix, 1 day: carry the run's alphanumeric offset from the USFM walk, verify at
that position, and fail generation on an ambiguity or an edge inside a word;
regenerate byte-identical.

**FIXED 14 October 2026** (`78b9ce81b`, `c9149e93a`; ROADMAP.md, Later
minor releases, row 32). `wj_texts` carries the letters and digits of each
run's verse ahead of it,
casefolded as `alphanumeric_map` gives them, with footnotes,
cross-references, alternate and published numbers (`\va`, `\vp`, `\ca`) and
heading lines (`\s`, `\r` and the like) left out; their count is the run's
offset. `place` puts the run at that offset when the runtime verse has the
same letters before it: the exact match first, from a rune between the
letter before the offset and the one at it, then the typography-normalized
one. A run of punctuation alone lies between those two letters. Where the
runtime verse differs before the run, or the run's letters are not at its
offset, generation fails; with `--allow-relocated` the run is placed instead
where its letters occur once in the whole verse and after the run before
it, and the summary lists it. No copy, two or more, or an end inside a word
fails generation either way. Chapter and verse markers are read exactly, so
`\cl` and `\cd` reach the heading rule and `\va` and `\vp` no longer stop the
walk.

`TestTheBSBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` fails on the
generator before the fix in three verses: "Go" in "Gospel", and the
narration's copy on the exact path and on the normalized one. Its other
verses, a heading inside the verse, a run carried into the next verse and a
closing mark marked on its own, pass before and after; they hold the new
count to the runtime text, and with heading lines counted the heading verse
fails. `TestTheBSBRedLetterGeneratorRefusesARunItCannotPlace` refuses, with
and without `--allow-relocated`, two copies after a changed lead-in, two
copies where the changed lead-in puts the offset on the later one, and an
end inside a word; each wrote a table before the fix.
`TestTheRedLetterGeneratorsPlaceARunByItsOnlyCopyOnlyWhenAllowed` and
`TestTheRedLetterGeneratorsReadChapterAndVerseMarkersExactly` fail before
it too: the first because the one copy was taken unasked, the second
because the walk stopped at `\cl`.

The publisher's `bsb_usfm.zip`, downloaded on 14 October 2026 (1,598,038
bytes, SHA-256 `f0c56438eb89c98e…`), differs from the copy recorded on 23
August (TEXTUAL-DATA.md, section 4): its ACT.usfm and LUK.usfm are dated 2
October 2026. Run on it and the local BSB cache (epoch 9), the generator
before the fix and after it each write the shipped `red_letter_bsb_data.go`
byte for byte: 2,042 verses and 2,294 runs, 2,251 exact and 43
typography-normalized, every one at its offset and none by its only copy.
`TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats the
comparison where `BIBLETEXT_BSB_USFM` names the unpacked source; with one
marker pair taken out of the source it fails.

### RL-WEB-1. The WEB red-letter generator takes the first copy too — FIXED 14 October 2026

Planned for: a later minor release, any; fixed on 14 October 2026 (the end
of this entry) — see docs/ROADMAP.md.

`locate` (`scripts/gen-web-redletter.py:104`) has RL-BSB-1's shape: in an
invented verse it reddens Peter's "I am he." for Jesus's. Latent: the WEB and
WEB Catholic tables are clean (no relocation mismatch, no edge inside a word,
one harmless double copy at Revelation 1:8). The seven pinned recoveries at
:33-56 are deliberate (docs/TEXTUAL-DATA.md).

Fix, 0.75 day: carry the run's offset within the USFM verse and verify there;
regenerate byte-identical, which needs the eBible USFM zips downloaded.

**FIXED 14 October 2026** (`3a8bbe3e3`, `c9149e93a`; ROADMAP.md, Later
minor releases, row 33), as RL-BSB-1 with the punctuation-preserving match
alone: `wj_texts` carries
the letters and digits of each run's verse ahead of it, heading lines and
alternate or published numbers left out, and `locate` matches only at their
count, when the runtime verse has the same letters before it. A run of
punctuation alone, a closing mark a note parts from its words (John 8:58,
Luke 12:59, Luke 14:24 and Revelation 1:8 in the WEB), lies between the
letters on either side of its offset. Where the runtime verse differs before
the run, or the run's letters are not at its offset, the run is unplaced,
and its verse falls to the pinned recoveries or fails generation; with
`--allow-relocated` it is placed instead where its letters occur once in the
whole verse and after the run before it, and listed. An end inside a word
leaves it unplaced either way. Chapter and verse markers are read exactly,
as in RL-BSB-1.
`TestTheWEBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` fails on the
generator before the fix in two verses, the other speaker's "I am he." and
"Go" in "Gospel"; its two speeches, a closing mark marked on its own and a
heading inside the verse pass before and after and hold the new count to
the runtime text, the heading verse failing with heading lines counted.
`TestTheWEBRedLetterGeneratorRefusesARunItCannotPlace` has RL-BSB-1's three
refusals, each of which wrote a table before the fix, and the WEB runs
under the two shared tests named there as well. Luke 8:8 is the one marked
verse whose runtime letters before a run differ from eBible's (in both
editions); it is a pinned recovery before and after.

eBible's zips, downloaded on 14 October 2026 (`eng-web_usfm.zip` 3,244,779
bytes, SHA-256 `614f08ed7c1bf86e…`; `eng-web-c_usfm.zip` 3,082,467 bytes,
`9fc9407dd8b3957f…`; both dated 13 October 2026), differ from the copies
recorded on 23 August. Run on them and the local caches (epoch 9), the
generator before the fix and after it each write the shipped
`red_letter_web_data.go` byte for byte: the WEB's 2,059 verses, 2,053 at
their offsets and 6 by the pinned recoveries, and the WEB Catholic's 2,059,
2,052 and 7, none by its only copy.
`TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats the
comparison where `BIBLETEXT_WEB_USFM` and `BIBLETEXT_WEBC_USFM` name the
unpacked sources.

### VERS-1. The versification generator declares a missing verse moved to its best text match — FIXED 14 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) —
see docs/ROADMAP.md.

`scripts/gen-versification.py:246` declares a verse an edition lacks moved to
the candidate scoring highest at 0.30 or more, so a repeated refrain would be
moved to an unrelated extra slot: Mark 9 with 9:44 and 9:46 omitted and an
added 9:51 repeating 9:48 yields 9:44 → 9:51 (WEB text). Everything that reads
`versificationDeltas` would follow it: the translation switch,
cross-references, note anchors, link open and the website's switcher.

Latent: the shipped table regenerates identically and no shipped edition has
the shape; no test catches an added row.

Fix, 1.5 days: seed moves from known positions (a hand list and the SIL
`.vrs` mapping lines `scripts/check-versification-refs.py` already
downloads), let similarity confirm them, refuse an ambiguous move, and pin
each edition's complete row set.

**FIXED 14 October 2026, for 1.2.22** (`276929af1`; ROADMAP.md, 1.2.22, row
19). A new slot that repeats a verse the edition kept matches the dropped
verse as well as any relocation would, and says nothing about where it went.
A move is now refused when the candidate slot matches some verse the edition
kept at least as well (`repeats_kept_verse`); the dropped verse is absent and
the slot extra. Besides Mark 9, an edition numbering Romans' doxology
16:25–27 and repeating 16:20's benediction at 16:28 had 16:24 moved to 16:28;
it is now absent. `TestTheVersificationGeneratorDoesNotMoveAVerseToARepeat`
runs the generator on invented editions of both shapes, with the doxology's
real relocation as the control, and
`TestTheVersificationGeneratorReproducesTheShippedTables_LocalOnly` holds its
output on the local caches to the BSB's and the WEB Catholic's rows of both
shipped tables, byte for byte before and after. The NKJV's rows were not
regenerated, since no NKJV source was opened for it (ROADMAP.md, Decisions
still open, 43), and the shipped tables are unchanged.
Seeding moves from the SIL mapping was not needed for the shape found, and
the row sets are pinned only by the local-only test.

### Of the same shape, and deliberate or not defects

Each with what settles it, so that none is taken up again by mistake:

- **F**, a selection with no span answered by the title and heading rules:
  the design note at `share.go:803-834`, decided 6 October 2026; entered only
  with no span, and P3 retires it.
- **H**, Android's mobile Fyne fallback guesses the touched verse:
  SOURCE_FIELDS_DECISIONS.md, Decision 22, and PLATFORM_MATRIX.md divergence
  32; the pane is unreachable in every shipping build. Its comment's "usually
  right" is not: 16.1% of the touchable WEB paragraph height is attributed to
  another verse.
- **I, AND-6, AND-7 and APPLE-NOTE-COUNTS-SPAN-BY-SEARCH**, the note chrome's
  who-line and counts searched for in their own line, which holds each count
  once (NOTE_CHROME_UNIFICATION.md; `notes_chrome_conformance_test.go`).
  Apart from that, a test fixture describes a line the composer does not
  produce: `note_counts_span_test.go:57` puts the chevron before " · N not
  shown here", where the composer appends it last (`notes_chrome.go:240`,
  after `notes_plan.go:600`). The conformance sweep
  (`notes_chrome_conformance_test.go:70-74`) covers the real shape; the
  fixture is corrected with any change to that test.
- **J**, the share parts read back from the message's own shape, with no text
  search (`share_parts.go`).
- **L**, the prompt quoting the raw selection: the text is the provider's
  input, and the position travels in the span (`ai.go:144-145`).
- **MAC-VERSE-LOOKUPS-WALK-PAST-CONTENT-END and
  APPLE-VERSE-INDEX-REDERIVED-BY-TYPOGRAPHY**: the Apple panes' geometry is
  the carried position (FOOTNOTES.md, the 0.85 em pact); 3,706 chapters
  through the AppKit importer showed no mismatch.
- **W2**, a link's `a` record written from the same verses as its fragment.
- **W4**, a psalm title alone not addressable on the website, as decided 6
  October 2026 (Verse attribution, item 2).
- **RL-WEB-2**, the seven pinned recoveries in `gen-web-redletter.py`
  (TEXTUAL-DATA.md).

Not defects: **K** (the verse of the day's card carries its span),
**MAC-FNSEP-TAG-SEARCH** (the footnote separator is the app's own escaped
sentinel), **MAC-LOOKUP-MENU-BY-TITLE** (it finds a menu item by its English
title, not a position; the bundle has no localisations), **AND-5** (the
superscript span is the importer's own carried position), **W5** (the
notice's title is built from the same reference as its heading) and **W6**
(the website's verse ids are unique; the NKJV feed is unchecked, which a
build-time uniqueness assertion would close).

Observed and not verified: the live website's WEB John pages carry one
paragraph per chapter, so even a readable note there lands its card under the
heading with the verse 800 to 1,500 px below (John 3:16: the card at 16 px,
the verse at 1,403). `cmd/websitegen/render.go:218` takes paragraphs from the
app's rule, so either the live pages predate the WEB's paragraph data for
John or the WEB has none there. W3's fix lands the verse when the card is not
beside it, which would cover this too.

Not measured: the NKJV, for M, E, W6, the shipped NKJV red-letter table and
VERS-1. And the text-to-speech and chapter strings cross JNI as modified
UTF-8 (`NewStringUTF`), which would shift later offsets for a character
outside the Basic Multilingual Plane; none is known in the shipped texts.

## Four rules for a shared note's link, in the app and on the website — decided 7 October 2026; FIXED 14 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry), the
website's half with it — see docs/ROADMAP.md. Moved on 7
October 2026 to the release after 1.2.20; 1.2.21 went out without it.

The app's note decoder (`DecodeNote`, `share_note.go`) and the website's
(`cmd/websitegen/assets.go`) must give every payload the same outcome
(NOTE_WIRE_FORMAT.md, *Conformance corpus*). Four rules are settled and in
neither yet:

1. **Whitespace inside a link's note payload reads as damaged.** The wire
   format's base64 is exactly the URL-safe alphabet with correctly formed
   trailing padding (*Accepted hazards*), so a payload a messenger broke
   with a space or a line break is damaged, not skipped over.
2. **A link's ends are trimmed of ASCII whitespace only**, on both decoders,
   so the two agree on what a trim takes.
3. **The website cleans a note's text as the app does**: `cleanNote` in the
   reader script does what `normalizeNote` (`share_note.go`) does.
4. **The website accepts a compressed block whose distance table is empty.**
   Go's `compress/flate` accepts an empty distance code tree, which is valid
   for a block with no back-references; the website's table builder refuses
   every incomplete table but the one-code case, so such a note opens in the
   app and reads as damaged on the website.

Each goes into `testdata/note_vectors.txt`, which both decoders are held to,
with a payload the rule changes. The website's half reaches readers with the
next `scripts/publish-site.sh`.

**FIXED 14 October 2026, for 1.2.22** (`67f90c576`; ROADMAP.md, 1.2.22, row
16). Of 16 probe payloads, 10 decoded differently: a line feed inside opened
in the app and was damaged on the website, two spaces inside the reverse (Go's
base64 decoder skipped CR and LF, the browser's `atob` spaces); a trailing
U+FEFF or U+2028 came off on the website only and U+0085 or U+00A0 in the app
only; the website kept the zero-width space, the word joiner, the tag
characters and U+2028 and U+2029, so five U+2028 drew five blank lines; and a
dynamic block with an empty distance table opened in the app and was damaged
on the website. Each decoder now refuses a byte outside the url-safe alphabet
and its padding, trims ASCII white space only, and the website's `cleanNote`
does what `normalizeNote` does, U+2028 and U+2029 reading as line feeds
before blank lines collapse; its table builder accepts the empty distance
table. The corpus gains the cleaning and empty-table vectors, append-only;
`TestTheAppAndTheWebReaderReadNoteLinksAlike` holds the app, the shipped
decoder under node and the Go mirror to the white-space and newline payloads
a corpus line cannot hold. NOTE_WIRE_FORMAT.md states the rules. The
website's pinned digests moved with `reader.js` in both states of the switch.

## A phrase ending on a dash, where its verse has the same words earlier, is cut short — deferred from 1.2.20, 7 October 2026; FIXED 14 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) —
see docs/ROADMAP.md. Moved on 7 October 2026 to the release after 1.2.20.

Two BSB phrases end on a dash where their verse ends, and the verse has the
same words earlier with a word straight after the dash ("herself—or" in
Numbers 5:14). Placed at the earlier copy, the end repair reads the letter
after the dash as the rest of a word and cuts the phrase back, so it is
quoted with other words than those selected. Over every phrase of two, three
and five words in every verse of the WEB, the BSB and the WEB Catholic, each
with the span and without, these are the four left wrong ("Verse
attribution: three findings", item 1); `share_phrase_sweep_test.go:280-287`
holds them as known.

The four are two phrases, each wrong once with the span and once without.
The selection's place in its verse ("Where the app re-finds the reader's
position from text", P1, in 1.2.22) is expected to close the two with the
span, placing the phrase at the reader's copy, but that is not shown: the
chapter's next verse still follows that copy's dash, and
`TestEveryPhraseInAVerseSharesItsOwnWordsOverLocalCaches_LocalOnly` sends no
lead, so that sweep run with the lead settles it. The two with no span, which
no offset reaches, need the end repair's own rule, a dash that ends a word,
which changes what other drags quote and wants the phrase sweep before and
after. The known count goes to 0 only when both halves are done.

**FIXED 14 October 2026, for 1.2.22** (`251e04633`; ROADMAP.md, 1.2.22, row
17). The phrase sweep run with the lead the panes send places none of the
6,332,213 phrases wrong, so P1 closed the two with the span. The two with no
span, Numbers 5:14's "defiled herself—", placed at the verse's earlier
"herself—or" and shared as "defiled", and 2 John 1's "the truth—" shared as
"the", were the end repair's: it cut a selection back to its last whole word
whenever the letter after it was a word rune, and the BSB sets its dashes
closed up. A drag ending exactly on a dash inside a verse, which the sweep's
whole-word phrases never form, was cut the same way with a span or a lead
("If the gate is shut—" shared "If the gate is"). The repair no longer cuts
after an em dash; the dash comes off as a clause mark does at a cut.
`TestAPhraseEndingOnADashKeepsItsWords`; the local phrase sweep's known count
for the BSB falls from 4 to none.

**Beside it, FIXED 14 October 2026, for 1.2.22** (ROADMAP.md, 1.2.22, row
22). A drag that stops one letter or more into the word AFTER a closed-up
dash was cut back to the last space, losing the whole word before the dash
("was morning—t", Genesis 1:5, shared "was"), and with no space in it was
shared as a fragment ("shut—o"). The start repair had the mirror fault: a
drag begun inside the word before such a dash lost the word after it
("g—the first" shared "first"). Older than the change above, and with the
span and the lead alike: at every closed-up dash between words of two
letters or more, each shape lost its word at the dash, with the span and
with the lead, in all 2,052 of the BSB's drags, 428 of the WEB's and 476 of
the WEB Catholic's. The end repair now cuts back to the later of the last
space and the last such dash, the start repair forward to the earlier of
the first space and the first such dash, and the dash comes off as above.
`TestAPhraseEndingOnADashKeepsItsWords` gains both cases,
`TestADragCutInsideAWordAtADashKeepsTheWholeWordOverLocalCaches_LocalOnly`
sweeps every such dash in the three caches, none wrong, and the model in
`TestEveryDragBegunOrEndedInsideAWordIsCutBackAtItsOwnPlace` takes the
whole word across a dash too.

## A heading that opens as its verse does is shared whole (the BSB's "The LORD Is My Shepherd") — deferred from 1.2.20, 7 October 2026; FIXED 14 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed on its release branch (the end of this entry) —
see docs/ROADMAP.md. Moved on 7 October 2026 to the release after 1.2.20; it
had been left with the headings' decision.

In 55 BSB psalms the heading under the title opens as verse 1 does ("The
LORD Is My Shepherd", "The LORD is my shepherd"), and headings elsewhere open
as their verse does. A drag from such a heading through its verse's number
that stops in or at the end of those first letters ("He Restores 3 He") is
shared whole on every pane, heading and number as Scripture: having cut the
heading's end from the front of the selection, the heading repair takes the
letters at its end for the start of the same heading (`trimTrailingHeading`,
`share.go`), and what is left cannot be placed. The letters need not begin
the same heading: Genesis 9's "Noah’s Shame and Canaan’s Curse 18 The" ends
in the first word of the heading above verse 1, "The Covenant of the
Rainbow".

With the span the panes report, of the drags from each character of a
heading through its verse's number to the end of that verse's first, second
or third word, the BSB shares 7,587 of 218,070 as they came, under 275
headings; the WEB Catholic 6 of 2,124; the WEB none of 297. On the
plain-text fallback page, 11,021 of the BSB's 316,023 drags from a title into
its psalm are shared so ("Verse attribution: three findings", item 2).

The remedy recorded there: once a heading's end has been cut from the front
of a selection, no heading's start is cut from its end. The heading sweeps
(`TestPsalmTitleDragSweep`, `TestADragRunOnIntoTheHeadingBelowSweep`) give
the before and after.

**FIXED 14 October 2026, for 1.2.22** (`7d304b96f`; ROADMAP.md, 1.2.22, row
13). Measured again on `10b6cbf19`, over drags of one to three words from
inside a heading through the next verse's number: the BSB shared 15,081 of
189,996 wrong, under 682 headings, 6,126 of them under a heading above a
chapter's first verse; the WEB Catholic 7 of 1,824; the WEB none. The lead
does not help: it reads through the heading and fails its check. Four
causes, each fixed in the heading repair (`share.go`):

- The end trim, with no verse number to confirm it, took the verse's first
  words for the front of a heading ("The LORD Is My Shepherd 1 The LORD"
  lost "The LORD"; Genesis 4's "Descendants of Cain 17 And Cain" lost "Cain"
  to "Cain and Abel" above verse 1). The front now comes off first, after
  which only a heading below that verse may lose its front from the end, and
  what is left must end as the verse above that heading does.
- A heading piece whose words stand together in another verse was found
  there and never reached the repair. With a span that reaches the verse
  beneath, a piece confirmed by that verse's number comes off before the
  search, as a psalm title's does.
- A stack of title and headings over one verse is matched as one piece, so a
  drag begun in the upper one is recognised.
- A heading holding its own verse's number as a word (the note fused into
  the WEB Catholic's heading over Daniel 3:24) is confirmed at the number
  after the heading.

A piece run on to the number and to no word of the verse is answered as a
heading selected on its own. `TestADragFromAHeadingIntoItsVerseSharesTheVersesWords`,
`TestAHeadingPieceRunOnToAVerseWithNoWordsQuotesNothing` and
`TestTheHeadingRepairKeepsTheOtherDrags`; over the three local caches every
such drag now shares its verse's words, cited to that verse
(`TestEveryDragFromAHeadingIntoItsVerseOverLocalCaches_LocalOnly`). A word
selected inside a heading on its own is the different defect P4 (ROADMAP.md,
Later, row 29), which waits on what selecting a heading means.

**Beside it, FIXED 14 October 2026, for 1.2.22** (ROADMAP.md, 1.2.22, row
23). A drag from the end of a verse across a STACK of two headings over the
next verse, into that verse's first words, with no lead, was not placed and
was quoted with both headings as Scripture: each heading was removed whole
only where the number of the verse beneath stood straight after it, which
the upper heading of a stack never has. The BSB has the shape over Proverbs
22:17 ("Thirty Sayings of the Wise", "Saying 1") and Hebrews 11:30; with the
lead, or with one heading, the drag was right. Older than the change above.
The heading repair now takes a stack off whole, joined as a flattened
selection joins it, as `cutOpeningMatter` takes one a drag begins inside.
`TestADragAcrossAStackOfHeadingsSharesTheVersesOnEitherSide`, and
`TestEveryDragAcrossTheHeadingsBetweenTwoVersesOverLocalCaches_LocalOnly`
over the three caches, where all 24 of the BSB's 22,728 drags across the
headings between two verses that cross a stack were wrong, with the span and
without, and none is now.

## The NKJV's card route and selection route disagree in the opt-in walk — deferred from 1.2.20, 7 October 2026; the test FIXED 14 October 2026, for 1.2.22; DONE, closed 15 October 2026

Planned for: nothing left; the test's stand-in fixed on 1.2.22's release
branch, and the walk passes on an NKJV cache (the end of this entry) — see
docs/ROADMAP.md. Moved on 7 October 2026 to the release after 1.2.20.

`TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly`
(`share_passage_test.go`), opt-in by `BIBLETEXT_EXHAUSTIVE=1`, holds the
passage route (`shareQuoteForPassage`) equal to the selection route over
every verse of every downloaded translation. Its walked list includes the
NKJV, and with an NKJV cache present it reports 46,837 passages that differ,
with small capitals on one route only. The likelier cause is a stand-in in
the test that no longer does what the app does, rather than the app; that is
not yet shown. Settle which through `realCachePath`, comparing without
printing the text, fix the test or the route, and the walk passes on all
four translations.

**The test's stand-in, FIXED 14 October 2026, for 1.2.22** (`abea88a87`;
ROADMAP.md, 1.2.22, row 20). `selectionRouteRaw`, the test's stand-in for
what a pane hands the selection route, built each verse from `v.Text`, the
publisher's "Lord"; every pane hands over the drawn form, `smallCapsText`,
"Lᴏʀᴅ" (`android_chapter_html.go`, `reading.go`,
`reading_styled_layout.go`). No NKJV cache was opened for this: the WEB was
given the NKJV's shape, "LORD" written "Lord" under a small-capitals span: in
its Genesis, Exodus, Psalms and Isaiah 11,884 of 29,441 passages differ with
the old stand-in and none with the drawn text, and the WEB as it is, the
control, none with either. The stand-in now draws the verse, and
`TestPassageShareEqualsSelectionRoute` gains a fixture with the divine name in
small capitals, which the old stand-in fails in all 26 of its 40 passages
that reach a marked verse. That the NKJV's 46,837 come from the stand-in is
therefore shown on the WEB given the NKJV's marking, not on the NKJV: the
opt-in walk over an NKJV cache is still to run, and until it passes this
entry is not closed (ROADMAP.md, Later minor releases, row 45).

**DONE, closed 15 October 2026: the walk passes on the NKJV** (ROADMAP.md,
Later minor releases, row 45; Decisions still open, 43). The NKJV was
fetched once through the app's own path (`fetchAPIBible`) and saved as the
app saves its cache, outside the repository, and deleted after; the walk
was pointed at it by `BIBLETEXT_CACHE_PATH`, and only counts were read from
it. `TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly` compares
143,627 NKJV passages, every verse of every chapter at widths one to five,
and none differs. The control, the same walk with `selectionRouteRaw` built
from `v.Text` as before `abea88a87`, finds 46,837 of the 143,627 differing,
the figure this entry recorded before the fix: the NKJV's differences were
the stand-in's, and the card route and the selection route agree on the
NKJV.

## Explicit permission before text goes to a third-party AI (App Store guideline 5.1.2(i)) — found 13 October 2026; decided and built 15 October 2026, for 1.2.23; the key rule and the words settled 21 October 2026

Planned for: 1.2.23; decided on 15 October 2026 (docs/ROADMAP.md, Decisions
still open, 5) and built on `fu/1223-ai-consent` from `rel/1.2.23` at
`be1440fe0` (`754107e49`, `3789c0eaf`), merged into `rel/1.2.23` on 21 October 2026 — see
docs/ROADMAP.md, 1.2.23, row 13 (Later minor releases, row 39). What was
settled on 21 October 2026 is built on `fu/1223-consent-2` from
`rel/1.2.23` at `3261a1e13` (`7ef80e58b`), merged into `rel/1.2.23` on 22
October 2026 (`442a21938`). The privacy policy's sections, its fourth
sentence and its effective date, decided on 22 October 2026, are built on
`fu/1223-privacy-2` from `rel/1.2.23` at `da90dfb55` (`612b32a84`,
`7f77c510a`, `17abf2caa`, `478561c49`, `59f59be42`, `efd46978c`), merged
into `rel/1.2.23` on 22 October 2026.

Guideline 5.1.2(i) of the App Store Review Guidelines, as last updated on 8
June 2026, asks an app to disclose clearly where personal data goes, naming
third-party AI, and to obtain explicit permission before sending it. Study
with AI sends the selected passage, the study action, any question the
reader types and the Find search from the device to the provider the reader
chose, under the reader's own key. The note under the key field in Settings
(`ai_settings.go`) says what is sent, to whom, under whose key and that the
provider may keep it, and links the privacy policy, so the disclosure is
there. The permission is only implied by entering a key. Ask a question,
which sends text the reader types, first reached App Review with 1.2.21.

The smallest change that gives the permission is a one-time confirmation for
each provider, before the first request to it; until the reader allows it,
nothing is sent. Google Play's prominent-disclosure rule applies only to a
flow that would surprise the reader, which a request made on purpose is not,
so the requirement is Apple's; the platform-parity rule asks for one rule on
all six, or a divergence recorded in PLATFORM_MATRIX.md.

The decision, 15 October 2026: before the first request to each provider,
on all six platforms, the app asks once, with the provider's display name
filled in (here Claude (Anthropic)):

- title: "Send text to Claude (Anthropic)?"
- body: "Study with AI sends the passage you select, the study action and
  any question you type straight from this device to Claude (Anthropic),
  using your key. They may keep it under your account terms. Nothing is
  sent until you allow it. To withdraw, choose None in Settings or clear
  your key."
- buttons: "Not now" and "Allow".

Choosing None as the assistant, or clearing that provider's key, withdraws
the permission, and the app asks again next time. Find and Ask are covered
too: nothing is sent to a provider without that provider's recorded
permission. In 1.2.23 the note under the key field was set in body text,
with the same words. One rule on all six, so no divergence is recorded.

In 1.2.24 the supporting Settings note returns to the muted caption style
used before 1.2.23, giving the account controls visual priority. The paragraph
and Privacy Policy link remain available. The first-use permission question
retains its presentation and behaviour.

Settled on 21 October 2026:

- the body names a search, which Find sends: "Study with AI sends the
  passage you select, the study action, and any question or search you
  type straight from this device to Claude (Anthropic), using your key.
  They may keep it under your account terms. Nothing is sent until you
  allow it. To withdraw, choose None in Settings or clear your key.";
- the permission ends with its key: any change of that provider's key,
  cleared, replaced by another, or missing after an Android restore that
  brings the preferences back without it, ends the permission, and the
  app asks again before the next request;
- "Nothing is sent until you allow it." is approved as the study panel's
  and Find's message after Not now;
- Settings' Test key and model list may reach the provider with the key
  before any Allow, since they send none of the reader's text;
- the privacy policy gains three approved sentences on the question, its
  withdrawal and the record the app keeps, in `docs/privacy.html`; they
  reach readers with 1.2.23's publish of the site.

Settled on 22 October 2026:

- each of the policy's three sentences goes in the section it was drafted
  for: the question in the AI section, its withdrawal in *Your choices,
  retention, and deletion* and the record in *Information stored on your
  device*; after the record's comes a fourth approved sentence, "On
  Android this record is part of the app's settings, which Android may
  include in your device backup; it holds no key and none of your text.";
- the policy's effective date is the day 1.2.23 goes live, confirmed the
  same day as the day the website is published with 1.2.23, which stage
  11 of docs/RELEASING.md sets and which does not wait for the stores.

Built on 15 October 2026 on `fu/1223-ai-consent` (`754107e49`, from
`rel/1.2.23` at `be1440fe0`), merged into `rel/1.2.23` on 21 October 2026. `ai_permission.go` holds the
question, a sheet the toolkit draws, so the same sheet is drawn on the
iPhone, iPad, Mac, Android, Windows and Linux. The study panel asks in its
`startFetch`, before any request: that covers the four study verbs from
every pane's menu, and Ask, whose composer sends its question through the
panel. The desktop sidebar's Find and the phone's Search tab ask through
`findAsksPermission`, after the key check. Under the question, and after
Not now, the panel and the Find pane show "Nothing is sent until you allow
it." with Try again, which asks again. A light/dark change or a rotation
with the question up over the study panel brings the panel back about the
same passage, with Ask's question as typed, saying nothing was sent, and the
question over it; nothing is sent or recorded across the change. Until
`3789c0eaf` (16 October 2026) the change drained both and brought nothing
back, so the reader lost the panel and the question they had typed, where
the same change during a request keeps the panel. Over a Find pane the
question closes, and the pane comes back from state with the search in its
field, saying nothing was sent, with Try again. Allow records
`ai.allowed.<provider>` in the app's preferences. As built on 15 October
2026 the record held `1`, and only an empty key erased it, so a key
replaced by another kept the permission, and on Android, whose restore
brings the preferences back without the key (`no_backup/`), the same key
pasted again was sent with no question.
Since `7ef80e58b` the record holds a fingerprint of the key the question
was asked about, never the key: a form tag, which a later wording can
change to ask everyone again, and eight bytes of SHA-256 over the provider
and the key (`aiAllowedRecord`). Every check compares it with the key the
request would send (`keyStore.aiAllowed`), so it is no permission beside
another key or none, however the key changed. `keyStore.setAPIKey` erases
it on any change of the key, cleared, replaced, or entered where there was
none, so a key put back after a change, or pasted again after a restore,
asks again; the same key saved again, as the Settings field saves on every
edit, keeps it. A clear erases it even where the credential store fails
the delete, and `keyStore.setAIEnabled(false)` erases every one.
`runAIActionForPassage` and `runAISearch` refuse a provider with no record
for the key they send before the request leaves the device, whatever path
reached them, and the refusal reads as the same message. Settings' Test key
and its model list are not asked about: they send the key, and Test key a
fixed probe, none of the reader's text.

Tests. `ai_permission_test.go` sends to a stand-in provider on the test
machine through the seams below the check (`aiStudyGenerate`, new, and
`aiSearchGenerate`):

- nothing reaches it before Allow for Explain, Ask and the desktop Find;
- the phone's Find starts nothing before Allow, and comes back from state
  after a rebuild;
- Not now records nothing;
- Allow sends once, with the passage and Ask's question, and the next
  request asks nothing;
- the permission is kept for each provider;
- Clear and None in the Settings sheet withdraw it;
- the question carries the approved words for all four providers on a
  desktop window and a 320-point phone canvas, wrapped inside its card;
- the supporting Settings note is a muted caption;
- a light/dark change or a rotation with the question up over the panel
  brings both back, with Ask's question as typed, sends and records
  nothing, and Allow then sends the question with the passage once;
- over a Find pane the same change closes the question, starts nothing,
  and leaves the pane saying nothing was sent, with the search in its field;
- a key replaced by another asks again before the next request, and the
  first key put back asks again too, while the same key saved again asks
  nothing (`TestAReplacedKeyAsksAgainBeforeTheNextRequest`,
  `TestChangingTheKeyOrChoosingNoneWithdrawsThePermission`);
- a key changed while the question is open: Allow records the permission
  for the key the question was asked about, nothing is sent, and the
  question is asked again for the changed key, whose Allow sends once
  (`TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain`, `79cfb6759`);
- a key in the credential store other than the one allowed is refused by
  both requests (`TestTheRequestsRefuseAKeyThePermissionWasNotGivenWith`);
- preferences restored without the key, through Android's own store with
  a fresh `no_backup/` directory, hold no permission, and the same key
  pasted again asks again (`TestPreferencesRestoredWithoutTheKeyHoldNoPermission`);
- the record holds no part of the key's text
  (`TestThePermissionRecordHoldsNoKey`).

Over `rel/1.2.23`'s code at `3261a1e13`, given an adapter for the old
record calls, `TestAReplacedKeyAsksAgainBeforeTheNextRequest`,
`TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain`,
`TestTheRequestsRefuseAKeyThePermissionWasNotGivenWith`,
`TestPreferencesRestoredWithoutTheKeyHoldNoPermission` and the
replaced-key half of `TestChangingTheKeyOrChoosingNoneWithdrawsThePermission`
fail: the request goes out with the replaced key, with the key changed
while the question was open and with the restored record, and a replaced
key, or the first key put back, keeps the permission. The question tests
fail there on the body. `TestThePermissionRecordHoldsNoKey` calls
`aiAllowedRecord`, which that code does not have, so it has no such run
and guards only the record's form.
`TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain` fails too with
Allow recording the key in place at the tap instead of the one the
question was asked about: the request reaches the provider at the first
Allow.

Over 1.2.23's code (`be1440fe0`, given only the study request's seam and
the names the tests call), the ten tests of `754107e49` all fail: the
requests reach the stand-in at once with no question, nothing is withdrawn,
and the note is a muted caption.
`TestTheQuestionComesBackOverItsPanelAfterARebuild` fails for both changes
over `3789c0eaf` with only the question's reopen taken out: no question is
left on top after the change.
`TestAFindQuestionClosesOnARebuildAndThePaneSaysNothingWasSent` holds the
Find pane's behaviour, which `3789c0eaf` leaves as it was.

`ai_permission_contract_test.go` holds:

- every pane's study menu to `dispatchAIAction`;
- the reader's text to the two seams behind the check, with every call of a
  client's `generate` accounted for, and, since `7ef80e58b`, each check
  made with the key its request sends, which fails over `3261a1e13`'s
  sources;
- both Find panes to asking before a search starts;
- the native sources to naming no provider host.

The first three fail over 1.2.23's code. The fourth held before, and fails
with a control Java file naming a host.

Seen on devices on 15 and 16 October 2026, from scratch builds that answer
every request in the app with a synthetic reply, each beside a 1.2.23
control that sent at once with no question:

- the question before any request, Not now sending nothing, Clear and None
  withdrawing, Find asking first, and the Settings note as body text: on
  the Mac, an iPhone 16 Pro, an iPad Air 11, the API 35 and API 30
  emulators, and the Linux and Windows virtual machines;
- every word of the question and both buttons on screen: an iPhone SE
  (375 points, the narrowest simulator here) in portrait, and the API 30
  phone in portrait and landscape;
- with `3789c0eaf`, the panel and the question back after a rotation and
  after night mode on both emulators, and after a light/dark change on the
  iPhone 16 Pro, for Explain and for Ask with its typed question, with no
  request sent.

The key rule and the new body were seen on 21 October 2026, at
`97bd47e73` beside `rel/1.2.23` at `3261a1e13`, in development builds
with a throwaway probe in a scratch copy that answers both request seams
in the app with a synthetic reply, on the Mac, the iPhone 16 Pro and
iPhone SE simulators and the Linux virtual machine:

- the question with the new body, word for word;
- Allow recording the key's fingerprint, and the same key saved again
  asking nothing;
- a key replaced by another, the first key put back, and the key pasted
  again after it was taken out of where the app keeps it, past
  `keyStore.setAPIKey`, so the record stayed, each asking again, with
  nothing sent until Allow;
- on the Mac, relaunched with the preferences kept and only the key taken
  out: no permission, and the key pasted again asking again; the Mac's
  development build keeps its keys in the preferences, so this stands in
  for a restore and is not one;
- every word of the new body and both buttons on screen on the iPhone SE
  in portrait and in landscape.

The control showed the old body, and sent each of those requests with no
question.

Not run: on iOS, the panel and the question back after a rotation; Find
in the iPad's sidebar, since the iPad Air in portrait draws the compact
layout, where Find was seen; and Ask on the Mac, which the scripted run
there does not reach.

On the Android emulators, on 22 October 2026 at `da90dfb55` against
`9a308352d`, API 35 and API 30 alike, with WEB Psalm 136, Gemini, two
synthetic keys in the Keystore store, and each Explain a real long press
on a verse's word, then the toolbar's Study with AI and the popup's
Explain:

- the first Explain showed the question with its new body above Not now
  and Allow, with nothing sent, where the control showed the earlier
  body;
- Allow sent the request and recorded the key's fingerprint, where the
  control recorded "1";
- the second key saved cleared the record, and Explain with it asked
  again with nothing sent, where the control kept "1" and sent with the
  second key, no question;
- after a restore made by uninstalling the app, which takes `no_backup/`
  and the Keystore alias with it, installing it again and putting its
  settings back, not through a backup transport: the record still held,
  matching no key, and no key; Explain showed the panel that offers Open
  AI settings, with no question and nothing sent, on both builds;
- the second key pasted back asked again with nothing sent, where the
  control sent with no question.

On API 30 the body and both buttons fit whole in portrait; landscape was
not captured.

Three points of the rule as built on 15 October 2026 were answered on 21
October 2026, as listed above: before a Find the question named the
passage, the study action and any question typed, and now names a search
too; Settings' model list, fetched whenever the key area shows with a key
and whenever a key is pasted, and Test key, which sends a fixed prompt,
may reach the provider with the key before any Allow; and the permission
ends with its key.

The policy's two points that waited for confirmation were answered on 22
October 2026, as listed above. The three sentences, which stood together
in one paragraph of the AI section, each go in the section they were
drafted for, where the "also" of the second and third was written against
those sections. The record holds eight bytes of SHA-256 over a fixed tag,
the provider and the key, and on Android it is kept in the preferences
under `files/`, which Auto Backup carries, while the key stays under
`no_backup/`, so a backup holds a fingerprint against which a guessed key
could be checked; the fourth sentence says the record may be in the
device backup and holds no key and none of the reader's text.

Built on `fu/1223-privacy-2` from `rel/1.2.23` at `da90dfb55`:

- `17abf2caa` places the four sentences;
  `TestThePrivacyPolicyPlacesEachPermissionSentenceInItsSection` holds
  each once, in its section and on one source line, and the Android
  sentence straight after the record's, and fails over `da90dfb55`'s
  policy on the second, third and fourth;
- `612b32a84` holds the effective date to the policy's words at publish.
  `cmd/sitepages -check-policy-date` compares the policy about to be
  published with the live copy by the words a reader reads, and refuses
  one whose words differ while its date is the live copy's or is not the
  day of the run; `scripts/publish-site.sh` runs it with London's date,
  and stage 11 of docs/RELEASING.md gains the step that sets the date,
  on the day of the site's publish, which does not wait for the stores.
  Since `7f77c510a` the check runs before the reader is generated, so a
  publish it refuses spends no API.Bible quota, and a publish that cannot
  read the live copy stops there too; a dry run goes on to its drift
  report and ends refused. Since `59f59be42` the day of the run is asked
  first: a second change on the day the first went live, dated that day,
  passes, where no date could pass until the next day. A policy whose
  words are the live copy's carries the live copy's date or the day of
  the run and no other, where a date moved back with nothing changed
  passed; and where each link leads is compared too, where a link
  pointed elsewhere kept the old date. Stage 11's step names the words a
  reader reads and where a link leads, where it named any change of the
  file but its date. Against
  the policy live since 21 October 2026, a render of the commit it was
  published from (`d03c3f36a`) is the live copy byte for byte and passes,
  and a render of `da90dfb55` is refused.
  `TestThePolicyDateIsHeldToItsWords` and the tests beside it hold the
  comparison; `TestThePublishHoldsThePolicyDateToTheLiveCopy` and
  `TestStage11DatesAChangedPrivacyPolicyOnThePublishDay` fail over
  `da90dfb55`'s script and RELEASING.md, and the policy-date tests and the
  stage 11 test over `14ee36e32`'s check and RELEASING.md;
- `478561c49` makes README.md say where each platform keeps an AI key and
  what the preferences keep of an allowed one, and docs/VERSION_STATES.md
  say that a restore that does not bring a key back is also a reinstall or
  a factory reset of the same phone; its two tests fail over `da90dfb55`.
  `efd46978c` says the record is in the app's preferences on every
  platform, where the sentence read as if it were kept only where the
  directly downloaded Mac build, Windows and Linux keep the key; the
  README test fails over `59f59be42`. The paragraph's wording is a draft
  that waits for approval.

Still to do: approval of README.md's paragraph; and the
site's publish with 1.2.23, which carries the policy's sentences, with
the policy's effective date, which still reads "Effective date: 11
October 2026" (the Report sentence published on 21 October 2026,
`d03c3f36a`, kept that date too), set to the day of that publish, which
was confirmed on 22 October 2026 as the policy's effective date: the day
the website is published with 1.2.23.

## Google Play's Data safety answers and the privacy policy: three corrections — found 13 October 2026

Planned for: the account holder, in the Play Console and in
`docs/privacy.html`, beside 1.2.22, with no build — see docs/ROADMAP.md.
All three were made on 21 October 2026: corrections 1 and 2 in the Play
Console, sent together for Google's review, and correction 3 in
`docs/privacy.html` (`d03c3f36a`), which reaches readers with the next
`scripts/publish-site.sh`.

Google counts as collected whatever an app sends off the device, so Play's
Data safety form rightly declares the in-app search history and other
user-generated content as collected and shared; Apple counts only what the
developer, or code the developer added, can reach, so the App Store's "Data
Not Collected" is right too. Two of Play's answers disagree with
`docs/privacy.html`, and Google requires the two to agree:

1. **Other user-generated content is declared collected for Analytics.** The
   policy says the developer runs no analytics; the purpose is App
   functionality.
2. **The form says a reader can ask for the data to be deleted.** The policy
   sends a reader to the provider for that, the developer being unable to
   view or delete what a provider holds; the deletion request mechanism is
   No.

Both data types stay collected, optional, not processed ephemerally and
encrypted in transit. One Play edit at a time (docs/RELEASING.md, stage 6).

3. **The policy does not mention the AI panel's Report button**, which opens
   an email to the support address filled in with the passage, the question
   and the answer (`reportBtn`, `ai_panel.go`), beside the policy's sentence
   that the developer does not receive AI requests. One sentence, published
   with `scripts/publish-site.sh`. The sentence made names the passage and
   the study action, with the question and the answer, since the mail
   carries the passage's reference and not its text, and says it reaches the
   developer only if the reader sends it. The App
   Store's label can stay as it is: a message the reader chooses to send to
   support is the customer-service case Apple lets an app leave undeclared.

## Android drops the zero-width joiner and non-joiner typed into a text field — found 14 October 2026; FIXED 15 October 2026, for 1.2.23

Planned for: 1.2.23, a bug that needs no decision; merged into
`rel/1.2.23` on 15 October 2026 — see docs/ROADMAP.md (1.2.23, row 5;
Later minor releases, row 46 until then).

On Android, a note typed with a joined emoji (a family, a profession, a flag
joined by U+200D) is stored, shared in the message and encoded in its link
as separate emoji, and Persian or Indic text loses its zero-width
non-joiner. `normalizeNote` (`share_note.go`) keeps U+200C and U+200D on
purpose, since joined emoji and Indic and Persian text need them, so this
undoes a deliberate rule.

The cause is in the toolkit's mobile driver: every typed character passes
through `runeToPrintable` (`internal/driver/mobile/driver.go:594` in Fyne
v2.7.4), which returns 0 for any character `strconv.IsPrint` rejects, and a
0 is never sent to the field as `TypedRune`. Both joiners are format
characters, which `strconv.IsPrint` rejects. The desktop driver hands typed
characters on unfiltered (`processCharInput`,
`internal/driver/glfw/window.go`), and the iPhone and iPad note composer is
a native field (`note_entry_ios.go`), which was not checked. The filter is
in the driver, so every Android version has it, in every text field the
toolkit draws there, not only the note's.

Seen on 14 October 2026 on an Android 11 (API 30) emulator image, with a
debug build of the tree at `7a68743cb`: in Share with note, a note entered
through a test input method's `commitText`, the call a keyboard's emoji
panel makes, holding a family emoji (U+1F468 U+200D U+1F469 U+200D U+1F467)
then U+1F64F and U+10437. The input method sent both joiners; the shared
message carried the other five code points and no U+200D, and neither did
the link's payload. A keyboard's own emoji panel was not tried.

Fix: a toolkit patch that lets U+200C and U+200D through the mobile
driver's filter (or every format character, which `normalizeNote` already
sorts by its own rule), with a test that feeds the driver's typed-character
path a joined sequence and reads the field's text back.

Built on 15 October 2026 on `fu/1223-android-joiners` (`078182df2`, from
`149b85a91`, with the deletion fix below in `849b3ab8f`), merged into
`rel/1.2.23` the same day.
`patches/fyne-2.7.4-mobile-typed-characters.patch` makes the filter drop
only C0 controls, DEL, C1 controls and values that are not characters, the
rule GLFW applies before the desktop driver types a character. So besides
the two joiners, the other format characters, every
space (U+00A0, U+202F, U+3000), private-use characters and characters newer
than Go's Unicode 15.0 tables (the Unicode 16.0 emoji U+1FAE9), all of which
the filter also dropped, now reach the field. Every character rather than
the two joiners, because the desktop already types them all, and
`normalizeNote` sorts the format characters by its own rule, dropping the
bidirectional controls, the zero-width space, the word joiner, the byte-order
mark and tag characters; the mobile typing section of `patches/README.md`
has the reasoning.

Tests. The patch's own, `patches/testdata/mobile-typed-characters_test.go`,
copied into the generated toolkit and run by the macOS CI job, type a
family, a profession and a flag emoji, a Persian word with U+200C,
Devanagari with each joiner, the spaces and the newer characters through
`typeDownCanvas` into a focused entry, and a family emoji into a canvas with
nothing focused; they keep controls out and hold the filter to GLFW's rule
at every code point. Against the driver without the patch every one fails
but the control-character guard. `mobile_typed_characters_test.go` holds the
patch, its order after the patches it was made against, the copied tests and
the CI step, and checks that the composer's text is the message's note
line, the link's note and the kept record's words on the desktop card, the
phone sheet's own field and the native field.

Parity, from the code. On the Mac, Windows and Linux, GLFW hands the desktop
driver every character but a C0 or C1 control (the Mac's also skips U+F700
to U+F7FF, AppKit's function-key range), so the joiners were already typed
there. The iPhone and iPad note composer and Ask are a native `UITextView`,
whose text reaches Go unfiltered (`note_entry_ios.go`). Their other fields
are drawn by the toolkit (search, AI Find, the notes search, the book filter,
the key fields), take Android's path through the same filter, and are fixed
by the same patch. With it, the mobile driver follows the rule GLFW's
`_glfwInputChar` applies on all three desktops; only the Mac's skip of
AppKit's function-key range is not on the phones, and no mobile path is
known to send that range.

Device checks, 15 October 2026, of a debug build of `6b065be44`, each
against a build of `149b85a91` as the control, with one invented note of 29
code points (a family and a profession emoji, U+1FAE9 and a Persian word with
U+200C). On the API 35 and API 30 emulator images, a test input method's
`commitText` of it into Share with note arrived whole in the shared
message's note line and in the link's note; the control gave 24, with no
joiner and no U+1FAE9. On the iPhone 17 and iPad Air simulators, typed into
the search field through the toolkit's `keyboardTyped`, the field held 29
against the control's 24. The native composer on both simulators, and the
Mac's composer through GLFW's `insertText:`, kept all 29 on both builds, as
the code says they would.

A backspace after an emoji deleted the characters before it on Android,
seen in the same checks and present on `main`. The keyboard edits a hidden
`EditText`, and `GoNativeActivity.beforeTextChanged` sent the field one
delete per UTF-16 unit of the run an edit removed, while each delete removes
one character and `keyboardTyped` types one per code point. After "abcdef"
and a family emoji (five characters in eight units), one backspace sent as
`KEYCODE_DEL` left "abc", and one sent as
`deleteSurroundingTextInCodePoints(1, 0)` removed two characters, U+200D and
U+1F467. On `main` a backspace after "abc" and U+1F600 left "ab"; there the
joiners never reached the field, so by the code a backspace after a joined
emoji deletes more still. Fixed in `849b3ab8f`: the patch's second half
counts the removed run in code points, so the field loses what the keyboard
removed. `patches/testdata/BtTypedDeletesCheck.java`, run on the host's JVM
by `scripts/check-android-java.sh` against the patched activity, counts the
deletes for runs with and without characters outside the Basic Multilingual
Plane; against the unpatched activity it does not compile, and with the
count in units it fails six of its nine cases. On the API 35 and API 30
emulator images, with a debug build of `bbd2bf066`, one `KEYCODE_DEL` after
"abcdef" and the family emoji left "abcdef" in the message and the link,
`deleteSurroundingTextInCodePoints(1, 0)` removed U+1F467 alone, and the
29-character note still arrived whole. The control, a build of `6b065be44`,
gave "abc" and lost two characters.

What is left. On Android a backspace removes what the keyboard removed from
its hidden field, which for `KEYCODE_DEL` is a whole joined emoji. The
toolkit's entry removes one character per backspace, which is what the Mac,
Windows and Linux and the fields the toolkit draws on the iPhone and iPad
give it, so there one backspace after the family emoji leaves the man, a
joiner, the woman and a joiner (seen in the toolkit's test driver). This is
divergence 35 in `docs/PLATFORM_MATRIX.md`; closing it means the entry
deleting a whole grapheme cluster on every platform, a change to text
editing that is not part of this entry.

## A space after an emoji or a fallback-font character draws wrongly in text the toolkit draws — found 14 October 2026; FIXED 15 October 2026, for 1.2.23

Planned for: 1.2.23, a bug that needs no decision; merged into
`rel/1.2.23` on 15 October 2026 — see docs/ROADMAP.md (1.2.23, row 6;
Later minor releases, row 47 until then).

In note text the toolkit draws, the Share with note field and the notes
browser, "a 🙏 b" draws as "a 🙏b": the space after the emoji takes no
width. A space after a character drawn from a fallback font, such as 𝔊
(U+1D50A), draws as a missing-glyph box. "e🙏f" and "g é h" draw right. The
stored and shared text is right, and the native note card draws the same
note right. Seen on 14 October 2026 on an Android 11 (API 30) emulator
image.

It is not Android's. With the patched toolkit and its default theme on
macOS, the toolkit's measurement gives "a 🙏 b" and "a 🙏b" the same width,
where "a b" and "ab", and "g é h" and "g éh", differ by the width of a
space; the test driver there has no font for 𝔊, so the box was seen on
Android alone. So it should hold wherever the toolkit draws note text.

The cause, measured on the host with the patched toolkit: go-text's face
split (`splitByFace` and `ignoreFaceChange`, `shaping/input.go`, v0.3.4)
never changes face for a space, on the assumption that every font has one,
so a space joins the run of the face before it, the emoji face after an
emoji and the fallback face after a fallback character, and the toolkit's
painter (`internal/painter/font.go`) measures and draws it in that face.
What it becomes there depends on the face:

- The emoji face is the bundled Noto Color Emoji, a bitmap font. Its space
  glyph has an advance in the font's metrics and no bitmap, so it has no
  extents, and go-text's `HarfbuzzShaper.Shape` copies a glyph's advance only
  after its extents: the space comes out with no advance. Stock Fyne's
  EmojiOneColor gave its space an advance, so this case shows only in builds
  that apply the toolkit patches, which are every mobile build and every
  desktop release.
- A fallback face with no U+0020 shapes the space as glyph 0, which the
  painter draws as U+FFFD. The painter advanced a missing glyph by the run
  face's glyph 0 and counted that advance twice, once when it drew the box
  and again at the next missing glyph or the end of the run, so the text
  after it moved right and the measured width grew by that advance. The box
  drawn is wider than glyph 0 in the default face, so the second advance
  was mostly taken up by the box itself: at 40 px in Noto Sans the box is 40
  px against glyph 0's 24, and "a", a missing glyph and "b" left 13 px
  between the box and the "b". With the bundled symbol face, which has
  U+2318 and no space, as the fallback, "c ⌘ d" measured two of its
  missing-glyph advances wider than "c ⌘d" instead of one space.
- On macOS the toolkit takes 𝔊 from the system's LastResort face, which has
  a glyph for every character and draws each as a placeholder, the space
  after 𝔊 included: drawn with the app's theme, "c 𝔊 d" showed two
  placeholders, and the space measured almost four times the text's.
- Any other fallback face gives the space its own width, not the text's.

Fixed on 15 October 2026 (`3b4041c9d` and `5df78e565`) by a toolkit
patch, `patches/fyne-2.7.4-fallback-space.patch` (Patch 12 in
patches/README.md), applied after the font-cache patch. A space separator
alone in its cluster, shaped in a face other than the one the font map gives
it, takes the advance it has in that face, shaped alone, and draws nothing,
so it measures and draws as a space of the text wherever it falls; between
words of a fallback face (Hebrew or CJK text from a system face) it now has
the text face's width. A space with a combining mark after it shares the
mark's cluster and is left as shaped, so the mark is still drawn on it. No
run is split, so right-to-left text keeps its order. A missing glyph
advances once, by the U+FFFD the painter draws for it, so the box ends
before the next glyph and inside the text's measured width. Each such space
or U+FFFD is shaped alone once in a string, so text with many emoji measures
in about the time it did without the patch. RichText's ellipsis cut uses the
same advances. A rule limited to spaces that are missing or have no advance
would leave the LastResort placeholder.

The patch's tests (`patches/testdata/fallback-space_test.go`) that fail on
the tree without it: "a 🙏 b" is one space wider than "a 🙏b" through the
painter's `MeasureString`, `fyne.MeasureText` and a `Label`; a space after a
symbol-face, a monospace-face or a LastResort character has the text's
width; drawn, the space after the emoji opens the gap before the next letter
by a space, and the space after U+2318, or after 𝔊 in LastResort, holds no
ink; and a missing glyph measures the U+FFFD drawn for it, with the box's
ink ending where the next glyph starts. The monospace and LastResort cases
also fail with the narrower rule. The rest pass on the tree without the
patch and guard what it could break, each failing with one part of it
changed: a combining mark on a space in a fallback-face run is drawn and
keeps the run's shaped width, in left-to-right and right-to-left text; the
advances the ellipsis cut is given equal the painter's, and a label is cut
with an ellipsis where it draws; and forty spaces after emoji take about the
allocations of one. The app's whole suite, with and without `-tags next`,
passes against the patched toolkit, the reading-face and small-caps tests
among them.

Seen on 15 October 2026 at `01e83122b`, before the corrections in
`5df78e565`, against `149b85a91` as the control: read in the app, a space
after an emoji, after a fallback character and a no-break space measured
as a space, the same widths on the Mac, the iPhone 17 Pro and iPad Air
simulators and the API 35 and API 30 emulator images, where the control
measured the space after an emoji at no width. In the notes browser
"Amen 🙏 📖 end" drew the gap after 🙏 as wide as the space after "Amen",
19 px against the control's 8 on the iPhone and both emulator images, and
Share with note's field drew the spaces on the Mac and Android; on iOS that
field is native and drew the same on both builds. The corrections in
`5df78e565`, the box's advance and a mark on a space, were made after
these checks.

## The GitHub release notes are not among the release's coupled surfaces — found 14 October 2026; FIXED 15 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed with its version bump (the end of this entry) —
see docs/ROADMAP.md (1.2.22, row 31).

The GitHub release's notes are the argument of the `printf` that fills
`NOTES` in `.github/workflows/release.yml`. Stage 1 of `docs/RELEASING.md`
lists nine coupled surfaces that must name the version being released,
each held by a test; the notes are not among them, and no test ties their
text to the release. `scripts/check-public-surfaces.py` holds their store
links and their advice for the unsigned Mac download, not their version.
The text, which opens "New study tools and improvements since 1.2.19", was
set before v1.2.20 and has not changed since, so v1.2.21's GitHub release
carries 1.2.20's notes word for word, and 1.2.22's would too.

Fix: list the printf in stage 1 as a coupled surface, and add a test that
holds its text to the release being made, as the stores' What's New files
are held: for instance, that it names no earlier release as the current one
and is not a copy of the last tag's notes. The test goes in with 1.2.22's
version bump, when the notes are rewritten; added now, it would fail until
then.

FIXED 15 October 2026 (`85bd257d2`), after 1.2.22's version bump
(`618653cc0`) rewrote the notes. Stage 1 of `docs/RELEASING.md` lists the
printf as its tenth coupled surface, and `github_release_notes_test.go`
holds the notes' head, everything above the store and download footer
every release carries, to the release:

- `TestTheGitHubReleaseNotesAreForThisRelease`: the head's lead line names
  the release before this one, read as the second `[[release]]` in
  `linux/releases.toml` once the first is checked against the desktop
  ledger, and no release but that one and this; and below that line the
  head is not the previous release's, as `git show v<previous>:` gives the
  workflow at that release's tag. The second check needs the tag in the
  clone: CI's Linux job fetches the whole history and runs it, and a clone
  without the tag logs that it did not run. The printf's text is read as
  printf prints it, and an escape or conversion other than `\n`, `\\` and
  `%%` is reported, since the text printed would not be the text read.
- `TestStage1ListsTheGitHubReleaseNotes`: stage 1 names the workflow, the
  `NOTES` printf and the test.
- `TestGitHubReleaseNotesChecksFire`: each check against a workflow built
  to trip it, beside the controls that must pass.

On the tree before the bump (`149b85a91`) both checks fail: the notes for
1.2.21 open "since 1.2.19", and below that line they are v1.2.20's word
for word. With the bump both pass, and two changes to the bumped notes
each fail: a lead line naming 1.2.20, and 1.2.21's bullets under 1.2.22's
lead line.

## The faster-model offer for ChatGPT switched to a model that then read as unavailable — found 15 October 2026; FIXED 15 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, fixed for it (`c3e4c1c50`) — see docs/ROADMAP.md
(1.2.22, row 30). Fixed for 1.2.23 and moved into 1.2.22 the same day,
before its version bump; the 429 question at the end of this entry stays
open.

The waiting screens offer "Switch to a faster model", which pins the
provider's economy model as the Settings picker would. For ChatGPT the
economy model was `gpt-4o-mini`, and every request sends the shared output
budget, `aiMaxOutputTokens` (32768), as `max_completion_tokens`. That model
accepts at most 16384 completion tokens, so OpenAI refused each request with
an HTTP 400 whose message names the cap ("This model supports at most 16384
completion tokens"). `safeAIErrorDetail` (`ai_providers.go`) filed any 400
whose message contains "model" as "model unavailable", `isModelNotFound`
(`ai_model_resolve.go`) took that as the model being gone, and because the
pin is an explicit override the resolver returned `modelGoneError`: a reader
who took the offer was told the model isn't available with their key, on
every request, until they changed model in Settings. The live guard
`TestLivePinnedDefaultsExist` reported the fast model as not callable.
Verified live on 15 October 2026: the same request with
`max_completion_tokens` 16384 answers, and `gpt-5-mini` accepts 32768.

The fix has two halves. A 400 refusing the completion budget is filed under
its own category, `completionCapDetail` with the figure the provider named
and nothing else of the message, so it is not model-not-found; and the
OpenAI client, which Grok shares, sends that one request again at the cap,
once and only downwards (`openAIClient.generate`, `completionCapIn`,
`completionCapOf`). This also serves every device that stored the old
override, since `applyFasterModel` wrote the literal id. And
`openAIFastModel` is now `gpt-5-mini`, the economy model of the capable
default's own family, which takes the budget and, through
`openAIFixedTemperature`, is sent no temperature. The self-heal semantics
are otherwise unchanged: a 404 on a pinned model still reads as gone with no
retry, and a 404 on the Recommended path still self-heals. No wording
changes; README.md's model table names the new economy model.

Tests: `TestAPinnedModelWithALowerCompletionCapAnswersAtItsCap` (two
requests against a stand-in for OpenAI's endpoint, the second at 16384, no
self-heal, the pin kept), `TestAPinnedModelTheProviderLacksStillReadsAsGone`,
`TestTheRecommendedPathStillSelfHealsOnA404`,
`TestTheChatGPTFastModelIsInTheCapableDefaultsFamily`,
`TestACompletionCapRefusalIsFiledUnderItsOwnCategory` and
`TestACapAtOrAboveTheBudgetIsNotRetried`; the first and the fourth fail on
the tree before (`149b85a91`). Live on 15 October 2026: `gpt-5` and
`gpt-5-mini` answered the pinned-defaults guard, and a stored `gpt-4o-mini`
override answered by way of one 400 and one 200, where the tree before got
the 400 alone and reported the model gone.

Still open, found beside it: every HTTP 429 reads to the reader as the
service being busy, where a provider's 429 can also mean the reader's own
quota or spending cap is exhausted, which no retry will mend. Telling the two
apart per provider, and the line for the cap, need a decision.

## Three comments 1.2.20 left describing the code before it — all three corrected for 1.2.22

The website's was corrected on 13 October 2026, for 1.2.22, and the other two
on 14 October 2026 (`9a71f9c60`; ROADMAP.md, 1.2.22, row 21).

- The stylesheet's comment on the psalm title (`.text p.pst`,
  `cmd/websitegen/assets.go`) says Android from API 35 justifies a wrapped
  title's lines and that the Apple panes and Android hyphenate the title.
  Android sets the title ragged (`isRaggedParagraph`), and since 1.2.20
  neither hyphenates it (`p.pst` in `reading.go`; `keepTitleUnhyphenated`).
  The comment is part of every published page, so it waits for the next
  change to the site. Corrected 13 October 2026 (`745b0c35b`): it says the
  title is ragged and unhyphenated on every app pane, with Android 7 to 14
  the one difference left (PLATFORM_MATRIX.md, divergence 31).
- `notes_sticker_native_on.go` says the Windows and Linux pane has no
  sticker and so keeps the banner; that pane draws its own band, card and
  tail and the banner stands down for it (`notes_sticker_native_off.go`).
  Corrected 14 October 2026.
- `bibleTextDismissMenu`'s comment (`reading_ios.go`) says resigning first
  responder clears the selection; it hides it, and the text view keeps the
  range ("iPad and Mac: where the share popover points"). Corrected 14
  October 2026.

## 1.2.20's roadmap rows with no entry of their own — released 13 October 2026

docs/ROADMAP.md planned 1.2.20 in 21 rows on 6 October 2026, and they left
that page when it shipped. A row with an entry here has its outcome there.
These five had none, so their outcomes as recorded on 11 October 2026 are
kept here; what each left open is a row of docs/ROADMAP.md's Later minor
releases.

- **Row 10, Android 17: the 1.2.19 heading fix had not been checked.
  PASSED, 11 October 2026.** Two fresh Android 17 / API 37 emulator runs,
  including stable emulator 37.2.12, show natural word gaps in wrapped
  headings, left-aligned Hebrew-initial headings, and unhyphenated
  superscriptions. Portrait, landscape and return to portrait pass with the
  app process retained. No further heading fix is needed. This verifies the
  runtime, not final release-package identity.
- **Row 18, Ask a question about the selected passage. BUILT; host and
  native editor checks pass.** Ask is enabled independently of next. Final
  ordinary, next and development suites, the shipping-toolkit race suite,
  build and vet pass. Focused live prompt assessment and earlier ordinary
  native Mac editor/request checks pass. Final iPad native typing, Unicode
  draft, rotation and submission checks pass; Android keyboard dismissal,
  return and rotation preserve the exact edited draft. Physical reading,
  dictation and VoiceOver checks remain outstanding.
- **Row 19, Consistent passage previews and short-card fitting. BUILT; final
  native resize checks pass.** Shared quotation/citation styling and
  adaptive details are unconditional. Same-orientation resizing preserves
  the answer, explicit quotation choice and reading position; full
  navigation rebuilds retain the cached answer but reset its scroll and
  initial quotation choice. Final iPad and Android resize checks pass, and
  native Linux light/dark and composer checks pass 1,990 assertions across
  all five result actions. The earlier ordinary native Mac check covers the
  preceding shared panel; a current native Windows runtime remains
  unverified. The mobile toolkit's existing eight-point touch compensation
  shifts the 44-point control's physical hit region; native visible-edge
  coverage is not certified.
- **Row 20, iOS rendering during app switching. BUILT; native Simulator
  checks pass.** The native guard prevents background display calls; the
  owning callback drains submitted graphics work without waiting for Go, and
  inactive snapshots still draw. Simulator checks cover twelve background
  transitions with zero background display calls after the change. The
  correction is installed in the development phone preview. A captured
  physical termination has no established cause; Simulator evidence does
  not prove that the reported exits are resolved.
- **Row 21, Font memory retained by repeated study cards. BUILT;
  shipping-toolkit and native stress checks pass.** Font-resource tables are
  shared while mutable faces remain isolated. Eight painter race tests and
  rendering comparisons pass. Repeated native iOS answer-card cycles plateau
  near 298 MiB in the controlled run; that result does not establish the
  cause of earlier physical exits. The shipping wrappers apply this patch on
  desktop and mobile; plain host builds use stock Fyne.

## Compare perspectives in Study with AI — built 7 October 2026, for 1.2.20

Released in 1.2.20, 13 October 2026. The cross-references panel's order
(below) is fixed for 1.2.22 — see docs/ROADMAP.md.

A fourth Study with AI verb, action string `compare`, menu label "Compare
perspectives", panel title "Perspectives", on all six surfaces: one line in
each native menu (`reading_ios.go`, `reading_macos.go` with its selector,
`android/BtBridge.java` with the Go case in `reading_android_export.go`)
and the Fyne submenu ranging over `aiStudyVerbs` (`ai_compare.go`), held
equal by `study_verbs_source_test.go`. The prompt shares the prose study
instructions, asks for the main readings and whether they can be held together,
and uses Markdown for formatting while avoiding tables and HTML. It names no
groups or examples and prescribes no section structure. A combined reading must
preserve each reading's actual claims. The complete prompt is public in README.md
and held to the builder by `TestREADMEMirrorsPrompts`.

Ask a question uses a concise question-focused prompt. It separates passage
statements, reported speech and inference, corrects unsupported assumptions,
and mentions differing readings only when relevant to the question. Only the
supplied passage is quoted; surrounding context and other Scripture are
paraphrased. Markdown formatting is permitted without tables, HTML or
follow-up questions. README.md also mirrors this prompt.

The earlier structured scorer and its planted controls remain test-only
regressions. They depend on the former fixed sections and are not quality
scores for the current free-form prompt. Their optional live and recorded
rounds reject an unsupported prompt before making provider calls or scoring
answers. Fresh samples require direct review of the readings, assumptions,
textual support and compatibility reasoning, with exact prompt fingerprints.

With it, for every AI verb: the selection's verse position reaches the
request (`aiPassage`, `aiPassageFor`), so the prompt, the panel header, the
Report mail and the cache key name the verses the share card would cite
("Psalms 136:2", not "Psalms 136"); a passage the provider declines (a
blocked prompt, a withheld reply, a refusal — `aiDeclinedError`) reads as
declined, with Try again, where it read as a dropped connection; an action
string the app has no prompt for is dropped before any text leaves the
selection; the `%q` quoting that wrote a poem-line break as `\n` is gone
(ADDITIONS_AND_DROPS item 19).

The legacy fairness bench (`ai_compare_bench_test.go`): ten World English Bible
passages (`testdata/compare/passages.json`), a scorer of ten checks each
with a control that trips it and no other (`testdata/compare/_controls/`),
and a doubly gated live recorder (`ai_compare_bench_live_test.go`) that
writes the answers outside the tree for a person to read.
Recorded-answer evaluation is optional development tooling. Run it with
`BIBLETEXT_COMPAREBENCH_RECORDED=1 go test -run '^TestCompareBenchRecordedAnswersAreFair$' .`.
It requires the former structured prompt and stops before evaluating the current
free-form prompt. For a supported prompt it requires every default-provider
passage, checks the recorded prompt fingerprint and fails on an incomplete or
failing set. The ordinary suite
runs the deterministic prompt, request, cache, rendering, cancellation,
provider-error, menu and scorer-control tests without requiring paid responses
or approval of saved answers. The app never reads those response files.

Still to close: the six-surface pass on the World English Bible, never the NKJV (iPhone and
iPad simulators, the Mac, the Android emulator in both orientations with
the popup below and above the line, the Windows and Linux virtual machines
with the child menu near the bottom edge; `compare` and `compare-waiting`
open the panel without a key); the What's New copy; the screenshot retake
(ROADMAP row 14). The cross-references panel placed a selection with no
verse position by the probe matching alone, where the share card and the
prompts place it first at its first copy — PLATFORM_MATRIX divergence 33 as
numbered then. Since 13 October 2026, for 1.2.22, the panel takes the card's
order and the divergence is removed ("Where the app re-finds the reader's
position from text", B).

Privacy, walked against docs/PRIVACY_RELEASE_CHECKLIST.md: the reference
line of every study request now carries the verse range with the book and
chapter, where it carried the chapter alone; the selected text already
identified the verses. No new data type, recipient or trigger: the text
still goes from the reader's device to the provider the reader chose, with
the reader's own key, and nothing to project infrastructure.
`cmd/mobile/PrivacyInfo.xcprivacy`, the App Privacy answers, the Play Data
safety answers and the in-app disclosure under the key field
(`ai_settings.go`) are unchanged; `docs/privacy.html` and
`docs/support.html` name the verb in their feature lists (draft wording,
marked in the page). The Report button that Apple 1.2 and Play's
generative-AI policy rely on now names the verb and the verses. Not signed
in this cycle: the Play 9-12 declaration.

## macOS 12 and 13: an open note card calls a macOS 14 method — FIXED 2 October 2026

The walk's `switch` named two element constants AppKit marks macOS 14
(`NSBezierPathElementCubicCurveTo`, `NSBezierPathElementQuadraticCurveTo`)
— FIXED 6 October 2026, for 1.2.20, at the end of this entry.

`btMacLayoutNote` gives the note card its shape from
`btMacNoteBubblePath(w, h).CGPath` (`reading_macos.go:2783`). AppKit
declares `NSBezierPath`'s `CGPath` property `API_AVAILABLE(macos(14.0))`
(`NSBezierPath.h`), and the Mac's floor is macOS 12.0
(`macMinimumOSVersion` in `config/product.json`), which both Mac builds
compile and stamp with (`-mmacosx-version-min` and `LSMinimumSystemVersion`
in `scripts/release-mac-store.sh` and in the macOS job of
`.github/workflows/release.yml`). Nothing guards the call: there is no
`@available` or `respondsToSelector:` anywhere in `reading_macos.go`, and
no fallback. Built for the release's 12.0 target, the compiler says so at
that line: "'CGPath' is only available on macOS 14.0 or newer
[-Wunguarded-availability-new]". A one-line file using the property warns
the same way at a 12.0 target and not at 14.0 (checked 2 October 2026).

It is on the path of ordinary reading. Every note card shown open on the
native pane is laid out there — a note opened from the notes browser or a
shared link, and a note just sent — and only a pill (a minimised note, or
one with no text: `btMacNotePill`, line 1969) returns before that line. On
macOS 12 or 13, unless AppKit answers the selector privately, that is an
unrecognised-selector exception the moment such a card is laid out. The
line came in v1.2.0 (b8fcceb0b, 14 August 2026), but until v1.2.4
(fe569cc20, 27 August) the binary carried the SDK's version as its minimum
and launched only on the newest macOS, so the exposure dates from 1.2.4.
Not seen: the app has not been tested on macOS 12 or 13, and no crash
report showing this is known, though both Mac builds are offered there. A
run on either, opening any note, would show it.

The fix: build the `CGPath` by walking the bezier path's elements
(`elementAtIndex:associatedPoints:` into a `CGMutablePath`, the usual way
before macOS 14), or use the property under `@available(macOS 14, *)` with
that walk as the fallback; and make the availability warning an error in
the Mac builds' compiler flags, so the next such call fails the build.

**FIXED 2 October 2026.** The card's outline is copied from the bezier path
element by element (`btMacCGPathCreate`, `reading_macos.go`): each move,
line, curve and close as `NSBezierPath` holds it, read with
`elementAtIndex:associatedPoints:` into a `CGMutablePath`, which needs
nothing newer than the floor. Building the outline afresh with
`CGPathAddArc` was the other way, and was not taken: Core Graphics makes its
own curves for an arc, whose control points differ from AppKit's by some
3e-10 of a point (17 elements against 18 on the card with its tail), so the
card would have been near the old one rather than the same. The walk is
the same. Against the macOS 14 property on macOS 26, 84 card sizes (seven
widths, six heights, with the tail and without) gave the same elements to
the last bit, the same bounding boxes and, drawn at 2x by a `CAShapeLayer`
set up as the card's is, the same pixels: none of 21 million painted pixels
differed, where a copy moved a quarter of a point differed in 913. Both
slices of the built app no longer name the `CGPath` selector; the tree
before the fix did.

The package's Mac cgo now carries `-Werror=unguarded-availability-new`
(`reading_macos.go`; cgo joins a package's CFLAGS, so it covers every
preamble a Mac build compiles). At the release builds' 12.0 target the old
line put back fails the build ("'CGPath' is only available on macOS 14.0 or
newer"), and so does a macOS 14 Foundation call put in
`ai_secure_store_darwin.go`, a file with no CFLAGS line of its own. With the
fix the package compiles at 12.0 with no availability warning, for arm64 and
amd64, with and without the `bibletextdev` tag: the card was the only
unguarded call. A build without the floor, which is a development build and
CI's macOS job, targets the macOS it runs on and cannot see such a call, so
`mac_floor_availability_contract_test.go` holds the flag, and the card's use
of the walk, at the source. Not run on macOS 12 or 13.

**The walk's two macOS 14 names — FIXED 6 October 2026.** Today's clang
(Apple clang 21) lets a `case` label name `NSBezierPathElementCubicCurveTo`
and `NSBezierPathElementQuadraticCurveTo` at a 12.0 target without a word,
even with `-Wunguarded-availability-new`, so the release builds were never
failing; but a compiler that started counting those uses would fail the Mac
release on its day. The cubic's older name, `NSBezierPathElementCurveTo`,
was not the way out. It is `API_DEPRECATED` from macOS 14, so it compiles
quietly at the 12.0 floor but warns in every build that targets macOS 14 or
later, which is every host build and CI's macOS job (checked with
`clang -fsyntax-only` at 12.0, 14.0 and 26.0). Both curve kinds are now
matched by the values the SDK gives them, 2 and 4 (`btMacPathCubic`,
`btMacPathQuadratic`), which compile with no warning at any of the three
targets, with `-Wall -Wextra -Wswitch-enum`. The values are the constants'
own, so the walk takes the same branches as before. The package builds at
the 12.0 floor and at the host's target. `mac_floor_availability_contract_test.go`
now also fails if the walk names either macOS 14 kind or the deprecated
name, or stops handling any of the five kinds.

## Share as image says "Picture saved" for a card it could not read — FIXED 6 October 2026, for 1.2.20

The line telling the reader, "Could not share the card.", went in on 15
October 2026, for 1.2.23, with the other shares that cannot finish (the
entry on Email… below; docs/ROADMAP.md, Later minor releases, row 2).

On the desktop, when the rendered card cannot be read at the Share tap, the
image share still asks the file manager to show it, and can then open the
sheet under *Picture saved* saying that it is shown. `fallbackShareImage`
(`share_fallback.go:59-85`) reads the renderer's file at the tap (line 64);
on a read error it records that path as the picture, in no folder (lines
69-70), and runs the reveal on it all the same (line 74). `savedImage.line`
(`share_image_folder.go:258-271`) gives a picture in no folder that the
file manager showed *The picture is shown in your file manager.* (lines
264-265), and the sheet opens with that line and the missing file as
Email…'s attachment (line 82). Whether the file manager says yes decides
it:

- **Linux, the tarball and the AppImage.** xdg-open on the picture's folder
  (`share_reveal_linux.go:47`), which is the temp folder and is there.
  Where xdg-open exits 0 within the five-second wait (`revealByCommand`,
  `share_fallback.go:156-170`; `revealAnswerWait`, line 150), the sheet
  opens. Where it runs the file manager in the foreground, as it does on a
  desktop it does not recognise, the wait passes, the picture counts as
  not shown, and a picture in no folder gets no line and no sheet
  (`share_image_folder.go:268-269`) — that wait is recorded in "Linux and
  Windows: an in-app share sheet in place of the 1.4-second notice",
  below. Where the sheet does open and offers Email… (outside a sandbox,
  with a mail client as the mailto: handler) the compose has no file to
  attach and fails without a word (the entry below).
- **The snap.** The portal route opens the file before it asks the portal
  (`revealViaPortal`, `share_reveal_linux.go:55-58`); the open fails,
  nothing is shown, and no sheet opens — the approved rule for a picture
  nothing true can be said of.
- **Windows, only where its Share sheet falls back to the in-app one.** The
  copy for the Share sheet is made at the tap (`takeSharedImage`,
  `share_parts.go:96-104`); with the renderer's file gone the copy fails,
  `shareImageFile` ends the share at once (`share_windows.go:410-412`), and
  the fallback hands `fallbackShareImage` the missing file. Explorer counts
  as shown once it starts (`share_reveal_other.go:24-30`), so the sheet
  opens. Windows' image sheet offers no Email….

Very hard to reach: the renderer's file has to go between the preview
drawing it and the tap, by a temp-folder cleaner for instance. Read from
the code; not seen.

The fix: when the card cannot be read, skip the save and the reveal and
open no *Picture saved* sheet, logging why, as for a picture no folder took
that the file manager did not show; or, if the reader is to be told, a line
of its own — *The share could not be prepared.*, already approved for
Windows' own sheet (`windowsShareFailText`, `share_windows.go:121`), under
a heading still to be worded. Either way, no Email… for a file that is not
there. A host test: a card path that does not exist, the reveal stubbed to
say yes, and no sheet (control: the file present).

**FIXED 6 October 2026, for 1.2.20.** `fallbackShareImage`
(`share_fallback.go`) now ends the share at the read: a card that cannot
be read at the tap is neither saved nor shown, the file manager is not
asked, no sheet opens, and the cause is logged ("the shared picture's card
could not be read, so it was neither saved nor shown", with the read's
error). It is the desktop's rule for a picture nothing true can be said
of, which the snap already followed, and it uses no new words. It covers
the tarball and the AppImage, the Windows fallback (whose own Share sheet
hands it the renderer's file when the copy at the tap fails), and the
macOS mimic. `TestACardThatCouldNotBeReadOpensNoSheet`
(`share_image_folder_test.go`) holds it under Linux's rules and Windows':
the card's file absent and the file manager answering yes, no sheet, no
reveal, nothing in Downloads, and the cause in the log. With the old
function put back it fails on both, the sheet reading *Picture saved*,
*The picture is shown in your file manager.*, Email… and Done; the
control, the card there, is saved, shown and confirmed. Seen on the Linux
VM (GNOME, X11) with development builds of the tree before and after, the
image preview opened at launch (`BIBLETEXT_DEV_OPEN=share-image`) and its
card deleted before Share was pressed: before, Files opened on the empty
temp folder and the sheet said the picture was shown; after, nothing
opened and the cause was logged; and with the card left in place, the
fixed build saved the picture in Downloads, showed it, and said so.
iOS and Android say *Could not share the card.* on the same case; the
Mac's share picker is handed the missing file's address (not seen). The
Windows fallback ends the same way by the code and by the test under
Windows' rules, and was seen doing so on 15 October 2026 on the Windows
11 arm64 virtual machine, with 1.2.21's executable run without package
identity, as the direct download runs: with the card's file deleted
between the preview and Share, neither Windows' sheet nor the in-app
sheet opened, nothing was saved, and the cause above was logged, where
the control, the card in place, opened Windows' sheet. Not seen with
package identity — see docs/ROADMAP.md, Later minor releases, row 40.

## Email… on the desktop share sheet fails without a word — found 1 October 2026; FIXED 15 October 2026, for 1.2.23

Fixed with the other shares that cannot finish, on `fu/1223-share-fail-line`,
merged into `rel/1.2.23` on 21 October 2026 (below, and docs/ROADMAP.md, 1.2.23, row 15).
The three rules the build adds beyond decision 7, each described below,
were confirmed on 21 October 2026: the Mac's own share of a card gone at
the tap says "Could not share the card."; a compose the Email portal took
and gave no answer to says nothing; and on iOS a second share asked for
while the first sheet is still held back is not made.

Email… on the in-app share sheet (Linux, and Windows where its own Share
sheet cannot open) hands the share to the mail client on a goroutine and,
when that fails, writes a log line and nothing else
(`share_sheet_desktop.go:280-287`; the comment above it, lines 274-279,
says as much). Nothing comes back to the sheet, which stays as it was: the
button is pressed and nothing opens. The share itself has already
succeeded — the text is on the clipboard, the picture saved — so this is a
dead button, not a false message; but by the rule the share fixes have kept
since 20 September 2026, a share the reader started does not end in silence
("Android: a text share has no failure path", below).

The routes (`composeShareEmail`): on Linux the Email portal, then
`xdg-email`, then a `mailto:` link through xdg-open
(`share_email_linux.go:113-119`); on Windows a `mailto:` link through
Fyne's `OpenURL` (`share_email_windows.go:77-82`), which runs `rundll32
url.dll,FileProtocolHandler` and has only that command's exit status to
report.

A second fault rides on the Linux order. `composeEmailViaXDGEmail`
(`share_email_linux.go:162-172`) runs `xdg-email` to its end, and any
non-zero exit is an ordinary error, which `composeByRoutes`
(`share_email.go:157-171`) takes to mean nothing was reached, handing on to
the `mailto:` route. But `xdg-email` waits on what it starts: in current
xdg-utils, with Thunderbird as the handler it runs Thunderbird's `-compose`
itself, which stays in the foreground when Thunderbird was not already
running, and otherwise it ends with xdg-open's status. So the call can last
until the mail client exits, and for a text share a client that then
exits non-zero is followed by a `mailto:` link: a second compose for one
press. A picture share cannot reach that route, because
`composeEmailViaMailto` refuses an attachment at once
(`share_email_linux.go:177-180`). Not seen; no
mail client is installed on the Linux VM.

The fix: post the outcome back to the sheet — a line under the buttons when
the compose fails, its wording to be approved — and stop at the first route
that started a compose, rather than handing on after any non-zero exit.

**The second compose, FIXED 14 October 2026, for 1.2.22** (`5165c3659`;
ROADMAP.md, 1.2.22, row 18). `xdg-email`'s exit 4, "the action failed", is
the launcher's failure reported after it ran the mail client, which may by
then have opened a compose; it now ends the share (`xdgEmailExit`, in the
shared `share_email.go` so every platform's test run holds it). Exits 1, 2,
3 and 5 refuse before anything is launched and still hand on to the
`mailto:` link. `TestXDGEmailEndsTheComposeOnlyWhereItMayHaveOpenedOne`.
The line under the buttons waits on its wording.

Seen on 14 October 2026 on the Linux VM (Ubuntu 24.04, GNOME on X11,
xdg-utils 1.1.3), with stand-in mail handlers in place of a client, each
recording a compose when started. A handler named thunderbird, which
`xdg-email` runs itself with `-compose`, that composes and exits 1: `xdg-email`
exits 4 after its compose, and the `mailto:` link through `xdg-open` then
opened a second, the double compose the stop ends. Any other handler is
opened through `gio`, and `xdg-email` exits 0 once it is launched, whatever
the handler then returns; with the default handler's program missing it
exited 0 as well, `gio` opening another program. Exit 4 with nothing
launched was not seen. The trade-off the stop carries: should another
desktop's launcher fail with nothing opened and exit 4, the share now ends
with no compose and no word, where the `mailto:` link was tried, likely
through the same failing launcher. That silence is the one the line under
the buttons answers.

**FIXED 15 October 2026, for 1.2.23**, with the wording approved that day
(decision 7, Draft A). A compose now runs through `shareEmailAside`, off
the UI goroutine as before, and hands its outcome back to the sheet; one
that failed adds "Could not open your email program." in one line under
the buttons (`emailFailedLine`), and nothing else on the sheet changes,
since the copy or the save has succeeded. The line answers the last press
only, and only while the sheet is up: a later compose that opens takes it
away, and so does one the reader cancelled at the portal, whose answer 1
is now `mailCancelled` and says nothing (`shareEmailFailed`). Nor does a
compose the Email portal took and then gave no answer to — none within
the 15 seconds the app waits, the session bus closing first, or an answer
with no code: it ends the compose, as before, since the portal may yet
open one, and is `mailUnanswered` (`portalComposeOutcome`), since nothing
says the mail program could not be opened; it takes away a line an
earlier press left. Every failure the desktop answered is taken at its
word: every route refusing before it reached a mail program, a portal
answer other than 0, 1 and 2, and `xdg-email`'s exit 4, which the stop of
1.2.22 keeps ending the compose. Where that exit 4 comes from a mail
program `xdg-email` ran itself, as it runs Thunderbird, it arrives when the
program exits, so a program that opened a compose and then exits with a
failure while the sheet is still open is said not to have opened; the app
cannot tell that from a launcher that failed with nothing opened. A
light/dark change brings the line back with the sheet, and a compose still
running across the change answers to the sheet it brought back
(`shareEmailLine`, which the reopen carries): before, its answer went to
the sheet the change had taken down, so a failure was never said and a
compose that opened left the line in place.

On a window too short for the sheet with the line to open below the
header the line moves the sheet up into the gap kept under the header, or
further: the sheet is centred, and its box is already at its least. At 420
by 470 points the sheet clears the header without the line and starts at
75.8 with it, against the 79.1 every sheet keeps; at 420 by 430, at 55.8,
partway down Go to. The line is sized in by the sheet's own fit, the one a
window resize runs, so the rule that a sheet which cannot clear the header
covers it whole (decision 35, `fu/1223-short-windows`) holds for the line
now both changes are in the release branch (21 October 2026): with the
two merged, the sheet with the line covers the header at 420 by 470, 420
by 430 and 360 by 380, and
`TestTheEmailLineKeepsTheShareSheetClearOfOrOverTheHeader` (`b791120a4`)
holds it; on the share-fail branch alone it fails at those three sizes.

The same change ends the other desktop silences in the phones' words,
"Could not share the card.", in the sheet with that line and Done alone
(`sayShareFailed`, `shareDone.failed`): a card that cannot be read at the
tap, and a picture no folder took that the file manager did not show
(`fallbackShareImage`), on Linux, the Windows fallback and the macOS
mimic; the Mac's own share of a card gone at the tap, whose picker was
handed the missing file's address (`shareCardGone`); and, on all six, a
card the preview cannot draw, which opens no preview (`showShareImagePreview`;
on iOS in the share's alert, on Android in its toast, `BtBridge.shareFailed`).
On iOS every share sheet and the notice are presented through
`btIOSPresent`, which waits out the top view controller's transition and
presents from the top view controller as it then stands. A presentation
counts as taken once its completion has run, or while it has a presenter,
its view is in a window or it stands among the presented view
controllers, and is judged only 5 seconds after it was presented
(`btIOSPresentSettle`): the share sheet holds its own presentation back
while its share services load, with no presenter meanwhile, and UIKit
refuses it, when it does, about three seconds after it was asked for.
Judged at 0.35 seconds by its presenter, as first built, a sheet still
held back was presented a second time and the share then said "Could not
share the passage." over the first sheet once it opened. Refused, a
presentation is tried once more, and refused again the share says "Could
not share the passage." or "Could not share the card."; one presentation
is on its way at a time, and another asked for meanwhile, such as a second
tap on Share while the first sheet is held back, is not made and is
logged. Held by
`share_failed_desktop_test.go`, `share_failed_macos_test.go` and
`share_failed_test.go`, whose behaviour tests fail on `rel/1.2.23` at `be1440fe0`:
there Email…'s failure, the two picture failures and the Mac's gone card
say nothing, the card that cannot be drawn opens an empty preview, and the
iOS and Android sources have none of the new steps.

Seen on 21 October 2026 in development builds of the change driven by a
throwaway trigger, against `d7f6b9156`, its first build, and `be1440fe0`,
before it. On the arm64 Linux VM (X11): Email…'s line after a stand-in
`xdg-email` that exits 4 and none after one that exits 0; a failure, and a
compose that opened, answered after a light/dark change, said on the sheet
the change brought back, where `d7f6b9156` said nothing of either; and
"Could not share the card." with Done for a card gone at the tap, a card
that cannot be drawn and a picture no folder took. On the Mac, in the Linux
mimic, the same Email… cases, twice each. On the iPhone 16 Pro and iPad Air
13-inch (M4) simulators (iOS 26.5): a share chosen as the share sheet
before it closed opened its sheet and said nothing in 12 runs of 12 on
each, three from a fresh install, where `be1440fe0` showed no sheet and
said nothing in every run; chosen as an alert before it closed on a freshly
booted simulator, where the sheet held its presentation back 0.40 to 0.44
seconds, it opened its sheet and said nothing in 6 runs of 6 on the iPhone
and 5 of 5 on the iPad, where `d7f6b9156` presented a second sheet, which
UIKit refused, and said "Could not share the passage." over the open
sheet in every run (6 and 5); text and image shares opened their sheets;
and with UIKit made to refuse the sheet, presented from a view controller
with no window, it was tried again 5.4 seconds later and opened when
refused once, and said "Could not share the passage." 10.8 seconds after
the share when refused twice, UIKit logging each refusal 3.1 to 3.2
seconds after its presentation. Not seen this time on Windows, whose sheet is the same
file as Linux's, or on Android, which these fixes do not touch.

## A dropped session bus spins a core until the portal wait ends (Linux) — FIXED 6 October 2026, for 1.2.20

`portalRequest` (`share_portal_linux.go:34-79`) waits for the portal's
Response on a channel registered with godbus, in a loop that passes over a
nil signal: `case sig := <-responses: if sig == nil || ... { continue }`
(lines 62-66). godbus v5.1.0 closes every registered signal channel when
the connection ends: a read error in `inWorker` closes the connection
(`conn.go:387-397`), and the default signal handler's `Terminate` closes
each channel (`default_handler.go:271-284`). A closed channel answers every
receive at once with nil, so if the session bus drops after the portal has
taken the call, the loop spins on a core until the request's deadline: five
seconds for the image share's reveal (`revealAnswerWait`,
`share_fallback.go:150`) and fifteen for Email… (`portalAnswerWait`,
`share_email_linux.go:43`). It is bounded, off the UI goroutine, and ends
as "no answer from the portal", as a portal that never answered does, so
the reader is told nothing wrong. Not seen: a session bus drops when the
desktop session is ending.

The fix: receive with `sig, ok := <-responses` and, on a closed channel,
return an error at once (the connection closed), with `taken` true, since
the portal had the call.

**FIXED 6 October 2026, for 1.2.20.** `portalRequest` receives with
`sig, open := <-responses` and, on a closed channel, returns at once with
`taken` true and the error "the session bus closed before the portal
answered", which Email… takes as the end of its compose, as it does any
error once the portal has the call, and the reveal as a picture not
shown. `TestADroppedSessionBusEndsThePortalWaitAtOnce`
(`share_portal_linux_test.go`) has the test's portal take the call and
never answer, then kills the private bus's dbus-daemon: the request must
return within a second of the drop, as the bus closing; the control, the
bus left up, is still waiting then. On the Linux VM, with the old loop
put back, it returned as "no answer from the portal" 3.9 s after the drop
of a 4 s wait, using 3.92 s of CPU in 4.02 s; fixed, it returned at once,
using none.

## A sheet turned from one landscape to the other keeps its side padding on the old side (Android) — found 1 October 2026; the composer FIXED 6 October 2026

The note composer and Ask both retain their side-inset refit for 1.2.20.
The note composer's native Android turn is verified below. Ask's watch now
checks the sheet area's position and width as well as canvas and input-area
changes (`ai_ask.go`); an unchanged canvas size no longer prevents its refit.

Since 1.2.18 the note composer and Ask hold their card clear of the side
safe insets of a phone held sideways (`clearOfSideInsets`,
`sheet_fit.go:190-196`; the second fault under "Phone and tablet sheets
over the header", below). The insets are read as the sheet opens. The
composer reads them again only in its refit (`share_note_ui.go:428-429`),
and its watch runs the refit only when the canvas's size has changed
(`share_note_ui.go:452`). Ask never reads them again: it throws its refit
away (`sheet, _ := clearOfSideInsets(card, sideL, sideR)`, `ai_ask.go:162`),
and its watch looks only for the sheet closing.

A phone turned straight from landscape to the other landscape keeps the
canvas's size, so nothing refits, and nothing rebuilds the window, since
the navigation's answer (the rail) has not changed; but the inset moves: an
Android phone's display cutout, and its three-button navigation bar, go to
the other side. The card keeps its padding on the side that no longer needs
it and runs under the inset on the side that does, until the sheet is
closed and opened again. Cosmetic, and no worse on the inset's side than
1.2.17, where the card always ran under it. The iPhone is not affected:
UIKit reports the same inset at each side of a phone held sideways (62
points on an iPhone 17 Pro Max, `sheet_side_insets_test.go`), and an iPad's
side insets are 0. Read from the code; not seen on a device.

The fix: refit when the safe area changes, not only the size — the
composer's watch comparing the sheet area's position and width as well as
`cnv.Size()` — and give Ask its refit and the same watch.

**The composer, FIXED 6 October 2026** (for 1.2.20). Its watch
(`share_note_ui.go`) now refits when the sheet area starts somewhere else or
changes width, as well as when the canvas changes size; never on the area's
height, which a keyboard changes. `TestTheComposerFollowsTheSideInsetsAcrossALandscapeTurn`
(`sheet_side_insets_test.go`) turns a phone over three ways at an unchanged
canvas size — the cutout alone, whose area keeps its width and only starts
elsewhere; the cutout with a three-button bar opposite it; and a bar that
appears with the turn — and fails on the tree before the change, where the
card stayed padded on the old side and ran under the new inset, the native
field with it. The API 35 emulator makes the same turn: it refuses the
upside-down posture (`mAllowAllRotations=false`), so `adb emu rotate` takes
its display from ROTATION_270 straight to ROTATION_90 at an unchanged size.
With the composer opened at ROTATION_270 and the phone turned over, the build
before the change left the card at x 11..2252, under the 136px cutout now on
the left, and the fixed build moved it to x 147..2388, clear of it; so the
driver does hand the composer the new insets on that turn. Ask now keeps
its refit and watches the same safe-area changes. That code correction is
separate from the recorded native Android proof for the note composer.

## A Play upload's commit restarts a review in progress — FIXED 6 October 2026, for 1.2.20

`scripts/play-publish.py upload` commits its edit with Play's default for
changes in review (`upload()`, the `:commit` call with no query).
edits.commit's `changesInReviewBehavior` defaults to
CANCEL_IN_REVIEW_AND_SUBMIT: a commit made while other changes are in
review cancels that review and sends everything again. So an alpha upload
committed while a production release, or a listing change, is in Play's
review sends that review back to the start. `promote` (efb3e092e) and
`play/push-screenshots.py` commit with ERROR_IF_IN_REVIEW, so Play refuses
them instead; the upload is the one Play commit that still restarts a
review. With the upload and the promotion one after the other, the case is
an upload for the next release made before the last production release has
cleared review: 1.2.18's production release was in review when this was
written. Stage 6 of `docs/RELEASING.md` says to check the Play Console's
Publishing overview first.

The fix: commit the upload with `IN_REVIEW_QUERY` too, and on Play's
refusal say that the bundle is not on the track, the versionCode is still
unused and the edit is discarded, and to run the same command once the
review clears, as `promote` says of its own refused commit; a test in
`play/test_play_publish.py` pins the query on the upload's commit and the
refusal's message. It changes what an upload does while anything is in
review — it waits instead of restarting the review — so it goes in as a
change of its own, with stage 6's check then reading as the reason a
refusal can happen.

**Decided 6 October 2026** — Play uploads: wait for a review already in
progress instead of restarting it. The fix above goes in as its own change
in 1.2.20, and stage 6 of `docs/RELEASING.md` is rewritten with it, not
before.

**The fix.** `play-publish.py upload` commits with `IN_REVIEW_QUERY`, as
`promote` does. When Play refuses the commit with an HTTP 4xx, the upload
keeps call()'s message whole and adds that the bundle is not on the track,
that the versionCode is still unused, and that the same command is to be run
again once the Publishing overview shows nothing in review. It then
discards the edit. An edit Play has ended is still explained as an ended
edit. A 5xx at the commit is left exactly as call() gave it, since Play may
have taken that commit. Stage 6 of `docs/RELEASING.md` now says the upload
waits, with the Publishing overview check as the reason a refusal can
happen, and its "When a stage fails" row names the refused commit. Held by
`play/test_play_publish.py`. The stand-in for Play models a review in
progress: a commit without the query is taken and recorded as restarting
that review, and one with it is refused. `UploadWhileInReview` checks that
an upload made during a review restarts nothing and commits nothing, that
the same command goes up once the review clears, and that a refusal for any
other reason gets the same explanation and discard. `UploadNotes` pins the
query on the commit. Four of these fail against the previous script.

## A Play status read during an upload deletes the upload's edit — FIXED 2 October 2026

On 2 October 2026 a `scripts/play-publish.py upload` of the 1.2.18 bundle,
about 88 MB posted into an edit (`play-publish.py:115-118`), failed with
HTTP 400 FAILED_PRECONDITION, "This Edit has been deleted.", because
`scripts/release-status.py` ran at the same moment from a parallel step and
opened an edit of its own to read the tracks (`play()`,
`release-status.py:90-115`: the edit opened at line 93 and discarded at
line 108). Google's Edits documentation
(developers.google.com/android-publisher/edits) lets each user hold one
open edit at a time, and a new edit invalidates the one that user already
has open. The status read signs in as the same service account as the
upload (`play-publish.py`'s `access_token()`, line 36, which
`release-status.py` calls at line 92), so its edit took the upload's place.
The same page gives a second way to lose one: a commit by anyone, or any
change to the app in the Play Console, invalidates every other edit for the
app. Nothing changed on Play and the versionCode was not used: the
identical command, run again on its own, succeeded. Every Play step opens
an edit as that one account — `play-publish.py upload` and `tracks` (line
99), `release-status.py`, `play/push-screenshots.py` (line 311, its token
from `play-publish.py` too) — so any two at once can do this, as can a
screenshot push's commit or a change in the Play Console while an upload
is in flight; and `release-status.py`'s docstring, which calls it safe to
run at any point in a release or outside one (lines 8-9), is wrong for Play
while an upload is in flight.

The fix: say so in `docs/RELEASING.md` stage 6 — Play steps one at a time,
the status read never beside an upload — and in its "When a stage fails"
table, whose row for `play-publish.py upload` assumes a re-run meets a used
versionCode (not yet changed); name the exception in `release-status.py`'s
docstring; and have `play-publish.py upload` recognise the error in the
upload command itself (lines 108-139), at whichever of its calls on the
edit meets it — the bundle post (lines 117-118) on 2 October — and say that
the edit was invalidated during the upload, by another edit, a commit or a
Play Console change, and to run it again once nothing else is touching
Play. Not in `call()` (lines 54-64), which today exits with the bare HTTP
text: it is shared. `release-status.py`'s `play()` uses it,
`play/push-screenshots.py` runs every request through it (`Client._call`,
lines 224-231) and reads Play's status back out of its exit text
(`HTTP_STATUS`, line 100) to tell a refused commit from one whose outcome
is unknown (`commit()`, line 429), and `play/test_push_screenshots.py` pins
its signature (`test_the_token_and_the_call_are_play_publishs_own`). Any
change to its message must keep the `-> HTTP <code>` text.

**The fix** (7ed341878). `play-publish.py upload` recognises Play's "This
Edit has been deleted" at any of its calls on the edit and adds, after
call()'s message, which it leaves whole, that another edit opened as the
same service account, a commit or a Play Console change ended the edit,
that nothing in it reached Play and the versionCode is unused, and to run
the same command again once nothing else is touching Play. `call()` is
unchanged. `promote`, which arrived the same day (efb3e092e), says the
same of its own edit. Stage 6 of `docs/RELEASING.md` says Play steps run
one at a time; its "When a stage fails" row for the upload tells a used
versionCode from an ended edit; "What a conductor refuses" names two Play
steps at once; and `release-status.py`'s docstring, and the runbook's
opening, no longer call it safe beside another Play step. Held by
`play/test_play_publish.py`, `EditEndedMidUpload`: the explanation at the
bundle post and at the commit, with call()'s text kept whole; any other
refusal left exactly as call() gave it; and call()'s own `-> HTTP <code>`
message pinned. Each fails with its rule taken out.

## Play release notes reach Play as one paragraph — FIXED 2 October 2026

`--notes` in `scripts/play-publish.py` reads the file and folds all of its
whitespace into single spaces (`notes = " ".join(f.read().split())`, line
89), so the line breaks between the opening line and its bullets are lost
and Play shows one run-on paragraph with the bullets inline. It has done so
since the option arrived (2aa2a2abb, 10 September 2026). 1.2.17's alpha
release notes on Play, read back on 2 October 2026, are folded that way.
1.2.18's went up through a copy of the script whose one change kept the
lines — trailing spaces stripped from each line, and the whole stripped at
its ends — and Play stored the five lines of the 1.2.18 notes in
`docs/PLAY_LISTING.md` intact (read back the same day).

The fix: make that the script's behaviour, with a test that a notes file
of several lines reaches the track release's `releaseNotes` text with its
line breaks, and that the 500-character cap (lines 92-93) counts them.
Nothing tests `--notes`, or anything else `play-publish.py` does itself:
`play/test_push_screenshots.py` only checks that the script defines
`access_token()` and `call()` with the signature `push-screenshots.py`
relies on (`test_the_token_and_the_call_are_play_publishs_own`), and its
`PlayClient` tests use a stand-in for `call()`. A test of the notes under
`play/` would run with the Python tests CI already discovers there
(`docs/RELEASING.md`, stage 2).

**The fix** (7ed341878). `--notes` does what the 1.2.18 copy did
(`read_notes()`): the whole loses the whitespace at its two ends, which
takes the blank lines there and the first line's indent, each line loses
its trailing spaces, and the line breaks stay, so Play stores the lines;
the 500-character cap counts them. Held by
`play/test_play_publish.py`, `UploadNotes`, which drives the script
through stand-ins for `access_token()` and `call()`: a notes file of
several lines, with trailing spaces or CRLF ends, reaches the track
release's `releaseNotes` text with its line breaks; 501 characters
counting both breaks of a blank line are refused before anything is sent,
and 500 go up. Each fails against the previous script, which folded the
breaks into spaces. 1.2.17's alpha notes on Play stay folded.

## The App Review notes guard reads only the first line — FIXED 6 October 2026, for 1.2.20; the 1.2.20 notes must drop 1.2.19's comparisons

The checks on the tracked App Review notes that look for a release's
version read the heading alone. `TestAppReviewNotesAreForThisRelease` takes
the first line (`appstore_review_notes_test.go:76`) and fails on any x.y.z
in it other than the packaged version (lines 81-86); `load_notes` in
`appstore/push-review-notes.py` requires the first line to open with
`VERSION <TARGET_VERSION>` (lines 140-145) and reads nothing else for a
version. `scripts/check-release-identity.py` (lines 45-47) only requires
the iOS file to start with `VERSION <mobile version>`. Notes whose heading
was bumped over the last release's body pass all three, and would go to
App Review describing the wrong release. The Mac's notes are the open case:
1.2.17's `appstore/review-notes-macos.txt` with
only its first line made `VERSION 1.2.18 (macOS)`, and
`WHAT IS NEW IN 1.2.17` and `HOW TO EXERCISE 1.2.17` below it, passes
every check on that file — the review-notes Go tests and `load_notes`, run
on 2 October 2026 on an exported copy of the tree holding it. The iPhone's
notes have a partial net:
`TestAppReviewNotesPlaceTheSidewaysIPhoneNavigationByTheRule`
(`appstore_review_notes_test.go:215-238`), which reads only
`appstore/review-notes.txt`, fails 1.2.17's body there under a bumped
heading (run the same way), because its navigation paragraph keeps the
bottom tab bar on an iPhone held sideways. That checks one paragraph; a
stale iOS body that does not trip it passes as the Mac's does. Not hit:
1.2.18's notes were rewritten whole, and today the only x.y.z in either
file is 1.2.18.

The fix: in both, refuse any x.y.z anywhere in the notes other than the
packaged version, with an allow-list for one a release means to name (none
today). The test's control: the 1.2.17 macOS notes under a bumped heading,
which pass every check on that file as they stand and fail only the new
version check.

**Decided 6 October 2026** — notes for Apple's reviewers: stop naming old
versions in them. So the check needs no allow-list: any x.y.z in the notes
other than the packaged version fails. 1.2.19's iPhone notes name 1.2.18
twice, in comparisons; the 1.2.20 notes leave them out.

**The fix.** Both checks read the whole notes. `load_notes` in
`appstore/push-review-notes.py` refuses any x.y.z other than
`TARGET_VERSION`, wherever it stands, with no allow-list. Run on today's
tree with `--local-only`, it refuses 1.2.19's iPhone notes for naming
1.2.18. `TestAppReviewNotesAreForThisRelease` applies the same rule to the
tracked files from 1.2.20 on (`reviewNotesWholeTextFrom`). For 1.2.19 and
earlier it still reads the heading alone, because 1.2.19's notes were
written and sent under that rule, and the tree keeps them until the bump.
So the suite stays green now, and the bump to 1.2.20 is what makes the
comparisons fail. **The 1.2.20 preparation must rewrite those two
"(1.2.18: …)" comparisons in `appstore/review-notes.txt`**, which this fix
leaves untouched since the notes are the account holder's copy. On a copy of
the tree bumped to 1.2.20, today's notes with every 1.2.19 changed to 1.2.20
fail on 1.2.18, and the Mac's with only the heading bumped fail on 1.2.19;
both passed the previous checks. `TestTheReviewNotesVersionRuleReadsTheWholeNotes`
is the control on made notes: a bumped heading over the last release's body,
a comparison, and an operating system's version are caught, while verse
references and two-part numbers are not. `appstore/test_push_review_notes.py`
holds `load_notes` to the same cases, and to the same pattern as the Go test.
`scripts/check-release-identity.py` still checks only the iPhone notes'
first line, and needs no more with these two in place.

## linux/releases.toml dates a release the day before its tag — FIXED 6 October 2026, for 1.2.20

The file's header says each entry carries its annotated tag's date
(`linux/releases.toml:1-3`), and `cmd/linuxmeta` renders that date into the
AppStream metainfo's `<release>` lines. Three entries are a day early:
1.2.13 is dated 2026-09-20 and was tagged on 21 September, 1.2.17 2026-09-28
and tagged on 29 September, and 1.2.18 2026-10-01 and tagged at 05:45 on
2 October (`git for-each-ref --format='%(refname:short) %(taggerdate:short)'
refs/tags`). Every other annotated tag agrees with its entry. The cause is
the order of a release (`docs/RELEASING.md`): the entry is written in stage
1, the store artefacts are built from that tree in stage 3, and the tag
comes in stage 5, after them, so the tree cannot know the tag's date, and
correcting it afterwards would move the tree off artefacts already built.
`TestNewestReleaseIsTheLedgerVersionAndDatesDescend`
(`cmd/linuxmeta/main_test.go:79-106`) checks the dates only for being in
the past and descending (lines 91-101); otherwise it holds the newest entry
to the ledger version and checks the bullets.

The fix, one of two: date the entry the day the artefacts are built and
say so in the header and in stage 1 — what the tree can know, and the one
to take; or have stage 0 or stage 5 check that the newest date is within a
day of the tag. The past entries can stand, each within a day of its tag.

**Decided 6 October 2026** — Linux release date: the day the store files
are built. The header of `linux/releases.toml` and stage 1 of
`docs/RELEASING.md` say so in 1.2.20.

**Done.** The header now dates an entry by the day its store files are
built, says why, and leaves the three early entries as they are. Stage 1 says
the same. `TestTheLinuxReleaseDateIsTheBuildDay` (`releasing_doc_test.go`)
holds both to that wording, and fails if either goes back to the annotated
tag's date. `go run ./cmd/linuxmeta render` changes nothing, since only the
comment moved.

## msstore/submit.py keeps the last release's commit status in a new submission's state — FIXED 6 October 2026, for 1.2.20

`save_state` merges what it is given into `build/msstore/run-state.json`
(`msstore/submit.py:213-219`). `create` writes the new submission's id with
`committed` and `verified` cleared (lines 612-615) but leaves
`commitStatus`, which only `commit` (line 852) and `poll` (line 878) write,
and `abort` (line 932) leaves it too. After 1.2.18's `create` on
2 October 2026 the file named the new submission and still said
`"commitStatus": "CommitStarted"` from the previous release, while the
server called the submission PendingCommit. Nothing reads `commitStatus`:
`commit` goes by `committed`, `verified` and the server's own status (lines
832, 840 and 845), and `poll` by the server, so `commit` and `poll` were
unaffected. Only someone reading the file on its own would be misled.

The fix: `create` starts a fresh state for its submission — writing the
whole state rather than merging into the last, or clearing `commitStatus`
with the other flags — with a test in `msstore/test_submit.py` that a
create after a committed run leaves no `commitStatus` behind.

**The fix.** `create` writes its state whole (`new_state`) instead of
merging it into the last run's, so a new submission keeps nothing of the
previous one: no `commitStatus`, and no `verified` or `committed`, which it
already cleared. Every later step of the same submission still merges
(`save_state`). `abort` is unchanged; the next `create` starts fresh.
`msstore/test_submit.py`, `test_a_create_after_a_committed_run_keeps_nothing_of_it`,
creates after a state the last release's commit and poll left. It fails
against the previous script, which kept `"commitStatus": "CommitStarted"`.
The test that the id is on disk before the response is judged now reads the
file back instead of watching `save_state`.

## scripts/run-ios-device.sh: a locked phone costs a full rebuild — FIXED 6 October 2026, for 1.2.20

The developer install to a physical iPhone fails at `xcrun devicectl device
install app` (`scripts/run-ios-device.sh:276`) when the phone is locked,
with CoreDevice error 12040 or 10003, or "Failed to acquire assertion"
(seen on 2 October 2026). The script stops there (`set -e`), and its `EXIT`
trap (line 96) removes the work directory, which holds the signed
`BibleText.app` (lines 196-197), so each retry patches Fyne, builds the
fyne CLI, packages, cross-compiles and signs again — minutes each — to
repeat the one step that wanted an unlocked phone. The removal is
deliberate: the executable is linked with the project's bundled key (lines
109-111), so a kept bundle has to stay private and go once installed.

It must also be run from inside the module. The other steps that need the
repository either change directory in a subshell (lines 106, 113 and 192)
or run a script that finds its own root, but the cross-compile runs
`go build ... "$REPO_ROOT/cmd/mobile"` from the caller's directory (lines
203-207), and Go takes the main module from the working directory, not from
the package's path: from outside any module the build fails with "go.mod
file not found in current directory or any parent directory", and from
inside another module with "outside main module or its selected
dependencies" (both checked 2 October 2026) — after the slow steps before
it have run.

The fix: retry the install a few times on those errors, asking for the
phone to be unlocked; on a failed install keep the signed bundle in its
private directory and say how to install it again, removing it once an
install succeeds; and run the cross-compile as `go -C "$REPO_ROOT" build`,
so the module is the script's own.

**The fix.** The install goes through `scripts/ios-device-install.sh`, which
`run-ios-device.sh` sources. It tries again while devicectl answers 12040,
10003, "Failed to acquire assertion" or "Locked": five tries, ten seconds
apart, asking for the phone to be unlocked. Any other failure is not tried
again. A bundle that still did not go on is moved into `build/ios-device-kept`,
made afresh with mode 0700, and the script stops saying so.
`scripts/run-ios-device.sh --install-kept` then installs it with nothing
built and no key loaded. It drops any raw key in the environment before
devicectl runs, and is handled before the key is read. It removes the kept
bundle once it is on the phone; so does the next successful build. The
cross-compile is `go -C "$REPO_ROOT" build ... ./cmd/mobile` in
`run-ios-device.sh` and in `release-ios.sh`, which had the same flaw.
`release-ios.sh` had one more: `scripts/check-version-not-spent.sh`, which it
runs first, asked whatever repository the caller stood in. From outside one
it failed, and from inside another it compared that repository's HEAD with
that repository's origin, so it could pass a spent number. It now runs git
with `-C` at its own repository root. Held by `ios_device_tools_test.go`.
`TestALockedPhoneIsTriedAgainAndItsBundleKept` drives the helper with a
stand-in for xcrun: a phone locked for two tries installs on the third, one
locked throughout is tried four times of four, a refused signature once,
and the kept folder is 0700 and holds the moved bundle.
`TestRunIOSDeviceKeepsTheBundleItCouldNotInstall` and
`TestTheIOSBuildsCompileTheirOwnModule` read the two scripts.
`TestTheVersionCheckReadsItsOwnRepository` runs the check from another
repository, with its own as the control. Each fails against the previous
scripts. Not run against a phone: no device was attached for this change.

## Windows: use the native Share sheet — BUILT 30 September 2026, seen in the app unpackaged the same day and packaged on 1 October

Planned for: a later minor release, a check from the Store install on the
Windows virtual machine, after which this entry is marked shipped — see
docs/ROADMAP.md, Later minor releases, row 40. Deferred from 1.2.20, whose
run of it could not reach the virtual machine, and moved on 7 October 2026
to the release after 1.2.20, which went out without it. On 15 October 2026
1.2.21's package was checked there under a renamed identity (below); the
Store's own install was not launched, so the check is still to make.

On Windows every text share (Share with note, with citation, as link, and the
verse of the day's Share) copied to the clipboard and opened the in-app
confirmation sheet (the entry below, which replaced a 1.4-second notice on
30 September 2026), and Share as image saved the card to Downloads and opened
Explorer on it. Windows has a system Share sheet that a desktop app can open
for its own window, and every verb now opens it over the window, as the
picker opens on macOS — divergence 28 in `docs/PLATFORM_MATRIX.md`, whose
Sharing table records every platform. The in-app sheet stays behind it for
whenever it cannot open.

**The route.** `IDataTransferManagerInterop`, from the activation factory of
`Windows.ApplicationModel.DataTransfer.DataTransferManager`:

- `GetForWindow(hwnd)` gives the window's DataTransferManager. The HWND is
  reached as `title_bar_windows.go` reaches it: `driver.NativeWindow`,
  `RunNative`, `WindowsWindowContext.HWND`.
- A `DataRequested` handler fills the package: the citation as the title,
  which the sheet will not open without; `SetText` with the message every
  platform shares; `SetWebLink` as well for Share as link and a note's link;
  `SetStorageItems` with the rendered PNG for Share as image.
- `ShowShareUIForWindow(hwnd)` opens the sheet.

**Proven first, on the Windows VM (30 September 2026).** A standalone probe —
a plain Win32 window driving the same calls, with no package identity —
opened the sheet for text, a link and a picture on Windows 11 arm64 (build
26200), natively and as an x64 executable under emulation; every HRESULT was
S_OK. So the unpackaged zip can use the interop there, whatever Microsoft's
pages say (they disagree: the share guide says packaged or unpackaged, the
table of WinRT APIs in desktop apps lists the DataTransferManager as needing
package identity, and the interop's reference says UWP apps only). What the
sheet showed: for text, the apps to share to and no Copy; for a link, "Share
link", the title, the link, a QR code and Copy link; for a picture, the
file's name where the title would be, a thumbnail, Edit and Copy — which is
why the picture goes as a copy named for the reader. The sheet asked for the
share 140–549 ms after it was asked to open (the longest the first time),
always on the window's thread from its message loop, never inside the call,
and gave a deadline of about two seconds. The picture's file resolved on a
thread-pool thread the first time and inside `put_Completed` after that,
and Windows asked the completion handler for `IAgileObject` before taking
it. The DataPackage walked the one-item collection with First, HasCurrent,
Current and MoveNext on the window's thread and let it go. After the handler
is removed the DataTransferManager keeps one reference to it until the next
registration.

**What was built.**

- `share_windows.go` (`//go:build windows`) defines `nativeShareText` and
  `nativeShareImage`; `share_other.go` narrows to
  `!darwin && !android && !windows`, so Linux keeps the in-app sheet.
- The COM is hand-written on `golang.org/x/sys/windows`
  (`share_winrt_windows.go`, `share_winrt_object_windows.go`): HSTRINGs,
  calls through vtable slots read from the SDK's headers, and the objects
  Windows calls — the DataRequested handler, the file's completion handler
  and the picture's `IIterable<IStorageItem>` — as pinned Go objects with
  callback vtables made once, agile through the aggregated free-threaded
  marshaler. The parameterized interface ids are held to the hash they are
  derived from by a host test (`share_winrt_iids.go`).
- The window's thread joins a single-threaded apartment on the first share
  (`RoInitialize`); nothing else in the process initialises COM there. The
  thread is checked, since RunNative on Windows does not switch to it, and a
  verb started from a click dispatched inside one of the share's own calls
  waits for that share to be handed over.
- The picture is copied to "BibleText verse <date> <time>.png" in a folder
  of the app's under the temp directory, cleared of copies over a day old,
  and resolved as a StorageFile before the sheet opens, as .NET MAUI does,
  rather than inside the request, which by the documentation a deferral
  must answer within 200 ms. The copy is made at the tap, and the mail the
  preview set for the card is taken with it (`takeSharedImage`): the
  renderer writes every card to the same file, so a preview opened before
  the sheet asks, or before the fallback opens, would otherwise change the
  card that is shared or saved.
- `share_session.go` carries each share from the hand-over to the sheet's
  request and ends it exactly once: delivered, or in the verb's fallback,
  the in-app sheet, after a refused step, a package that would not fill,
  or five seconds without the sheet asking (the probe's slowest was 549
  ms). A sheet asked to open does not know the app has stopped waiting,
  and a sheet whose host starts slowly could appear after that; so the
  share keeps its handler and the package's parts after that fallback,
  and a late request still fills the package, until the sheet asks, a
  newer share starts, or 30 seconds pass — the time Chromium gives the
  same sheet before it gives up on a share. A newer share replaces one
  still waiting; a reader's cancel opens nothing, as on the other
  platforms. The handler is removed as the share lets go of it, never
  inside `ShowShareUIForWindow`.
- A package that cannot be filled fails the sheet's request with the
  words *The share could not be prepared.* (`windowsShareFailText`,
  `share_windows.go`), which Windows' own sheet shows, and the in-app sheet
  opens as well. That wording was approved on 1 October 2026.
- Tests: the session's rules, the split of a message into title, text and
  link, the picture's copy and the interface ids on every platform
  (`share_session_test.go`, `share_parts_test.go`), among them the late
  sheet that still gets the share, the kept handler let go by the keep or
  a newer share, and a late image fallback that saves the card that was
  shared (`share_sheet_desktop_test.go`); in the Windows CI job,
  against Windows itself with no sheet shown (`share_windows_test.go`), the
  objects answering COM, a real DataPackage filled as the request fills it
  and read back, and every step of a text and a picture share up to the
  sheet's opening with a window that is never shown, each refused step
  ending in one fallback, and the verbs with no native window ending in the
  in-app sheet. Those Windows tests passed on the arm64 VM (Windows 11,
  build 26200), natively and as x64 under emulation, from a test binary
  cross-compiled without cgo and carried in on an ISO; a control build
  with IAgileObject unanswered failed the object test, and one with the
  file handler's interface id mistyped failed the picture test, Windows
  refusing the handler with E_NOINTERFACE. A control with the DataRequested
  handler's id mistyped still registered: `add_DataRequested` does not ask
  the handler for its interface, so that id rests on its derivation and on
  the probe's sheet having asked its handler for the share. That VM run
  came before the handler kept for a late sheet and the picture taken at
  the tap; the tests for those ran on the host. At 1517e3771 the Windows
  CI job, on its x64 server image with cgo and the race detector, passed
  the whole suite; it runs without `-v`, so its log does not say whether
  that image let the Windows share tests reach the sheet's step or
  skipped them where it refuses a share.

**Seen in the app (30 September 2026).** The Microsoft Store package
workflow built 1517e3771 in the release configuration, cgo and `-tags
gles`, and packed and smoke-installed it for x64 and arm64 (run
36720846058). The arm64 package's executable, with ANGLE beside it, ran
on the Windows VM (Windows 11 arm64, build 26200) from a folder, without
package identity, as the direct download runs; the installed MSIX was
not run. Every verb from its own entry point — the selection menu's
Share with citation and Share as link, the composer's Share, the image
preview's Share and the verse-of-the-day card's icon — opened Windows'
own Share sheet over the window within about 2–4 s, and nothing opened
in the app. The text sheets, for the citation and the verse of the day,
were headed "Share" and listed the apps, with no title, preview or Copy.
The link and note sheets were headed "Share link", with the citation,
the link (the note's with its `n=` part), a QR-code button and a link
button with no label, which on the note sheet turned to a check when
pressed. The picture sheet named the file "BibleText verse 2026-09-30
13.51.34.png", with its size, the card's thumbnail, Edit and a copy
button with no label; the thumbnail means the sheet read the copy in the
unpackaged app's temp folder. Each sheet closed with its X, and no
in-app sheet had opened 6 to 18 s after any of the seven closes; Share
with citation and the verse of the day each opened the sheet again the
second time. An Escape sent through the VM's input left one sheet up;
its X closed it. The note card showed between verses 9 and 10 after the
page moved, probably at the head of the paragraph that holds verse 16;
its anchoring was not checked. No share target was pressed. The
selection was one word, taken by a double-click, since the VM's input
has no drag, and the app ran in dark only. The Sharing table's Windows
row is `hardware` on this run, as an exception scoped to it, until a
release carries the code (`docs/PLATFORM_MATRIX.md`, the Windows proof
note under Sharing).

**Seen as a packaged app (1 October 2026).** The Microsoft Store package
workflow built faae44ef1 (run 36840901241). Its arm64 artifact,
BibleText-1.2.17.0-arm64.msix (sha256
291b4210adf26f80b94e416832e61208b79ccbc6b8c0fbf978b25a5c8e6ccee7), was
registered on the same VM (Windows 11 arm64, build 26200) as a packaged
app: its layout, less the block map and the content-types part, with
`Add-AppxPackage -Register` and Developer Mode on for the run. The
Store's own 1.2.16 is installed there under the reserved identity, so the
manifest's identity name was changed to
`bibletext.co.uk.BibleText.ShareCheck` and nothing else. Started through
the shell, the running executable had package identity
(`GetPackageFullName` answered the package's full name; an unpackaged
shell answered none, the Start menu's host a package name). Every verb,
from its own entry point, opened Windows' own Share sheet over the window
by the first screenshot, two seconds after the tap, with the same sheets
as unpackaged; no in-app sheet opened after any of the closes, and no
target was pressed. The picture sheet named "BibleText verse 2026-10-01
09.27.18.png", 22.6 KB, with the card's thumbnail. The picture's copy was
not redirected: it went to the user's real temp folder,
`%LOCALAPPDATA%\Temp\bibletext-share`, not the package's `LocalCache`,
and did so again with that folder moved aside, when the packaged app made
it afresh (sheet: "BibleText verse 2026-10-01 09.33.08.png", 21.3 KB,
thumbnail shown). Why the temp folder was not redirected is not
established; Microsoft's account of packaged desktop apps' AppData writes
does not mention it.
Developer Mode, the registration and its folder were removed afterwards.
(`docs/PLATFORM_MATRIX.md`, the Windows proof note under Sharing.)

**Seen from a release's package (15 October 2026).** 1.2.21's arm64 MSIX
as submitted to the Store (sha256 39d71225…327d) was registered on the
same virtual machine by the 1 October recipe: its layout, under the
identity name `bibletext.co.uk.BibleText.ShareCheck`, with Developer Mode
on for the run. The Store's own install there is 1.2.19, Store-signed,
under the reserved identity; it carries this code, as every release since
1.2.18 does, and it was not launched. The running process had the renamed
package's identity (`GetPackageFullName`; Explorer answered no package,
and the Start menu's host its own). With "loved" in John 3:16 selected,
in the dark theme, Share as image opened Windows' Share sheet with
"BibleText verse 2026-10-15 10.43.18.png", 22.6 KB, and the card's
thumbnail; the copy was written to `%LOCALAPPDATA%\Temp\bibletext-share`,
not the package's `LocalCache`, as on 1 October; nothing was saved to
Downloads and no in-app sheet opened. The package's executable, with
ANGLE beside it, was also run from a folder without package identity, as
the direct download runs, for a picture whose card could not be read,
which 1.2.21's fallback ends without a sheet: with the renderer's file,
`bibletext-verse-0.png` in the temp folder, deleted between the preview
and Share, the log held the share handed to its fallback
(`share_session.go`) and the fallback's cause, "the shared picture's card
could not be read, so it was neither saved nor shown"
(`share_fallback.go`), both at 10:34:21 by the virtual machine's clock;
neither Windows' sheet nor the in-app sheet was open in a screenshot
showing that clock at 10:35, at least 39 s later, nothing was saved to
Downloads, and no share copy was made. The control, the card left in
place, opened Windows' sheet with "BibleText verse 2026-10-15
10.32.31.png", 26 KB.
That case was not run with package identity. No share target was
pressed. The registration, its folder and Developer Mode were removed
afterwards.

**Still open.**

- **The package as the Store installs it.** The runs above registered a
  package's layout in place, unsigned, in development mode, on
  15 October 2026 1.2.21's as submitted to the Store. The Store-signed
  install under `C:\Program Files\WindowsApps` has not been launched for
  this check, although every release since 1.2.18 carries the code and
  the virtual machine's install is 1.2.19. Left: Share as image from the
  Store's own install, and where its copy lands.
- **What a target receives.** No app in the sheet was chosen, and
  Windows 11's text sheet shows neither the quote nor its citation, so
  the text each verb hands over has not been seen arriving. What the
  note sheet's link button put on the clipboard was not read back, and
  the picture sheet's copy button was not pressed.
- **A mail app with a link share.** Whether a target that takes both the
  text and the web link shows the link twice; not seen.
- **Windows 10.** The interop is there from Windows 8 and the MSIX's floor
  is 10.0.19041, but only Windows 11 has been seen.
- **x64, the light theme, a whole-verse selection, and a share while a
  sheet is open.** The app ran on arm64 only, in dark, with a one-word
  selection, and no share was started over an open sheet
  (`docs/VISUAL_TESTS.md`, V12). The probe, not the app, ran under x64
  emulation.
- **The in-app sheet on Windows.** The Share sheet opened every time it
  was asked to. The fallback has run in the app on Windows once, with
  1.2.21 on 15 October 2026, for a card that could not be read, which
  1.2.21's fallback ended without a sheet; the in-app sheet itself, the
  image share's Explorer reveal and Email… have not run in the app on
  Windows.
- **A sheet that asks late.** A request after the five seconds, which the
  kept handler answers, has not been seen: the probe's sheets all asked
  within 549 ms, and the app's were up within about 2–4 s of the click. A
  slow Windows 10 machine, or a first share with the share host starting
  cold, is where it would show.

**Sources.**

- The route, for packaged and unpackaged apps:
  https://learn.microsoft.com/en-us/windows/apps/develop/windows-integration/integrate-sharesheet-send
- WinRT support in desktop apps:
  https://learn.microsoft.com/en-us/windows/apps/desktop/modernize/winrt-api-desktop-app-support
- The cancel event and its floor:
  https://learn.microsoft.com/en-us/uwp/api/windows.applicationmodel.datatransfer.datapackage.sharecanceled
- The interop, and the 200 ms a deferral has:
  https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-idatatransfermanagerinterop ;
  https://learn.microsoft.com/en-us/uwp/api/windows.applicationmodel.datatransfer.datarequest.getdeferral
- How a parameterized interface's id is derived:
  https://learn.microsoft.com/en-us/uwp/winrt-cref/winrt-type-system
- The implementations read before writing ours:
  https://github.com/chromium/chromium/blob/main/chrome/browser/webshare/win/show_share_ui_for_window_operation.cc ;
  https://github.com/chromium/chromium/blob/main/chrome/browser/webshare/win/show_share_ui_for_window_operation.h
  (its 30-second `kMaxExecutionTime`) ;
  https://github.com/mozilla-firefox/firefox/blob/main/widget/windows/WindowsUIUtils.cpp ;
  https://github.com/dotnet/maui/blob/main/src/Essentials/src/Share/Share.windows.cs ;
  https://github.com/greenshot/greenshot/blob/main/src/Greenshot/Native/DataTransferManagerHelper.cs
- The Go options that were ruled out: https://github.com/go-ole/go-ole ;
  https://github.com/saltosystems/winrt-go ;
  https://github.com/gioui-plugins/gio-plugins/tree/main/share

## Linux and Windows: an in-app share sheet in place of the 1.4-second notice — FIXED 30 September 2026

Still open: GNOME on Wayland may open Files behind the window (stays); see
docs/ROADMAP.md. A picture no folder could take, which the file manager did
not show, says "Could not share the card." since 15 October 2026, for
1.2.23 (the entry on Email… on the desktop share sheet).

On Linux and Windows the text shares copied to the clipboard and showed
"Copied to the clipboard" for 1.4 seconds: a 13 pt pill at the window's
foot, its fill 1.07:1 against the page in light and about 1.2:1 in dark,
hidden early by any click. On Share with note it appeared in the frame the
composer closed and the note card drew at the top of the passage, so it was
easy to miss entirely (divergence 27, `docs/PLATFORM_MATRIX.md`). Seen on
the arm64 VM's X11 desktop, 29–30 September 2026; Windows ran the same code.

**No system sheet to call.** xdg-desktop-portal has no Share portal; the
request has been open since 22 June 2016. GNOME's interface guidelines have
no share pattern, and KDE's Purpose share menu works only inside KDE. What
others do: Firefox and Chrome offer Copy Link (and a QR code) on Linux;
GNOME Maps opens a dialog with the link, Copy and email; Flutter's
share_plus sends a mailto: link. Comparable confirmations stay up 4–7
seconds (libadwaita's toast, Kirigami's passive notification); this one
lasted 1.4. Windows has a native Share sheet, which it now opens (the entry
above); it shares through this sheet only when that cannot open.

**The fix** (`share_sheet_desktop.go`, from `share_fallback.go`). Share
with note, with citation, as link and the verse of the day's Share copy to
the clipboard as before and then open one modal sheet in the app, styled
like the composer, in place of the notice:

- the heading **Copied — ready to paste**, and one line by verb — note:
  *Your note and the link are on the clipboard. Paste them into a message
  or email to send your note.*; citation: *The verse and its citation are
  on the clipboard. Paste them into a message, email or document.*; link:
  *The link is on the clipboard. Paste it into a message or email.* The
  verb is told from the message's own shape (`shareVerbOf`), since the
  platform seam carries the text alone; a note share with the note left
  empty is the link share, and reads as one;
- a read-only box with the exact clipboard text, wrapping, and scrolling
  when it is long, its height given up a step at a time where the
  sheet would otherwise start inside the header;
- **Email…** only when the desktop has a mail client, **Copy again** (after
  it the line reads *Copied again.* for a moment, in a slot that keeps the
  verb's line's height, so nothing below it moves), and **Done**. Email…
  is first in the row: it appears when the probe answers, a moment after
  the sheet, and at the row's left end it moves nothing; between Copy again
  and Done it pushed Copy again aside and took its place under the
  pointer. Escape and Return close the sheet, through the canvas's key
  handler, wrapped while the sheet is on top — nothing on the sheet takes
  the caret by itself, which is why the box is not selectable: a selection
  in a Fyne label takes every key and answers none — and after Tab has put
  the caret on a button, through the buttons' own key handling
  (`shareSheetButton`): Return is Done there too, wherever the caret is,
  and Space presses the button that holds it. A stray click cannot dismiss
  it. Registered for the light/dark reopen and the window refit; below the
  header (`clearOfHeader`). The image share keeps its save to ~/Downloads
  and the file-manager reveal and ends in the same sheet, *Picture saved*
  over *The picture is saved in Downloads and shown in your file manager.*
  (or, where there is no Downloads folder, *The picture is shown in your
  file manager.*, the file manager opening on the temp copy), with no
  clipboard box, and Email… where the platform can attach the file.
- **Over the verse-of-the-day card** the sheet stacks on the card, Escape
  closes the sheet and then the card, and a light/dark change brings both
  back, the card beneath: the sheet's capture takes the card's registration
  too (`takeReopenBeneath`, `sheet_reopen.go`), the one exception to
  reopening only the top sheet, because the sheet is the end of a verb
  begun from the card, not a choice inside it.
- **Email…** (`share_email.go`): on Linux the xdg-desktop-portal Email
  interface — `ComposeEmail` over godbus with the subject (the citation,
  one line, at most 200 characters), the body (the clipboard text; for the
  image, the quote and its citation, what Share with citation copies) and,
  for the image, the file as a descriptor; the request's `Response` is
  watched. Only 2 (no mail client) hands on to `xdg-email` and then a
  `mailto:` link through xdg-open: 1 (the reader cancelled), any other
  answer, and no answer once the portal has taken the call end the
  compose, so a late compose is never joined by a second
  (`composeByRoutes`, `portalEmailResponse`). The button is offered only
  when the desktop says a mailto: handler exists (`linuxMailOffered`), asked
  on a goroutine, so the sheet is never delayed by the bus:
  - text: OpenURI's `SchemeSupported` wherever the portal can answer it
    (1.19.1 and later); otherwise `xdg-mime`'s default handler for the
    scheme, and none when there is no `xdg-mime` to ask. Inside the snap or
    the Flatpak, `xdg-mime` reads the sandbox's own configuration and not
    the desktop's, so there, without `SchemeSupported`, Email… is withheld:
    the snap on Ubuntu 24.04 (portal 1.18.4) offers no Email….
  - image: only outside a sandbox, with the portal's Email interface or
    `xdg-email` to take the file, and only when the handler's desktop entry
    is in the Email category and not WebBrowser. Both routes end in that
    handler, and the GTK backend (like `xdg-email`) hands it a `mailto:`
    link with the file's path as `attachment=`, which a mail client reads
    and a browser does not: the stock Ubuntu desktop's handler, the Firefox
    snap, opened a compose with no picture and no text and the local path
    in its link. A sandboxed app cannot tell a mail client from a browser,
    so the snap and the Flatpak withhold the image's Email…, as Windows
    does.

  On Windows a `mailto:` link through the shell when the shell resolves a
  handler for the scheme — `AssocQueryStringW`, which reads the reader's
  own choice (UserChoice) before the class's registration, counting a
  handler whose executable is on disk or a packaged app's DelegateExecute
  handler, and not a key an uninstalled client left behind. A link carries
  no file, so the image sheet has no Email… there. The macOS mimic of this
  path opens a `mailto:` link too. Every `mailto:` link the app builds
  carries line breaks as CRLF (RFC 6068) and is kept to 2,000 characters,
  the length past which some Windows mail handlers cut or refuse one
  (`mailtoURL`): the passage or the note is cut at a word and ends in an
  ellipsis, inside the closing quotation mark of a quotation, and the
  citation, and a link share's link, stay whole. A whole chapter as a link
  runs to some 5,900 characters (John 3) or 18,000 (Psalm 119); the whole
  text is still on the clipboard, which the sheet says, to paste over the
  cut one.

The notice, `showShareNotice`, and its self-dismissing-overlay
registration in `sheet_reopen.go` are gone; nothing else used them. The
Sharing table's Windows and Linux rows now name the sheet, and
`TestTheSharingTableNamesTheFileThatSharesOnEachPlatform` holds the cells
to `showShareCopiedSheet` as it held them to the notice.

**Held by** `share_sheet_desktop_test.go`, with the share verbs delivering
through the desktop fallback as `share_other.go` routes them: each verb
from its own entry point — the selection menu's actions, the composer's
Share, the card's icon, the image hand-off — leaves the sheet on the
overlay stack, inside the canvas and below the header at 1280x800 and at
520x640, in light and in dark, with the approved words, the box showing
what the clipboard holds and the message the verb composes; a click outside
does not dismiss it; Done, Escape and Return close it and run the sheet-close
consume point once and give the canvas its own key handler back, and Escape
and Return still do after a drag across the box and after Tab has put the
caret on each button in turn, where Space presses that button; Copy again
re-copies and moves neither the box nor a button, and nor does Email…
arriving late; a light/dark change brings it back with the same text, and
with the verse-of-the-day card beneath it, and the image sheet with its
picture; the sent note's card is on the page under it; Email… is offered
only when the probe says so, asked with the file only for the picture, and
hands over the citation, the text, and the image's file with its quote —
the image's from the preview's own Share button — and, where there is no
Downloads folder, the image sheet says only where the picture is shown.
`share_email_test.go` holds the decisions behind Email… on every platform:
the 2,000-character link and where it is cut, which portal answers hand on
and which end the compose, the handler's desktop entry, and when Linux
offers the button, in and out of a sandbox; `share_email_other_test.go` and
`share_email_windows_test.go` hold that a `mailto:` link never carries the
picture, and the Windows one, in the Windows CI job, that the handler check
counts a handler on disk and a packaged one and not a stale key.
`TestDesktopSheetsOpenBelowTheHeader` and the refit test cover both forms
of the sheet with the other desktop sheets. Driven on the Linux VM in light
and dark, every verb, with the clipboard read back each time
(`docs/VISUAL_TESTS.md`, V12, and the Linux proof note under Sharing in
`docs/PLATFORM_MATRIX.md`). No mail client is installed there, and the
session names the Firefox snap as the mailto: handler, so the text sheets
offer Email…; pressed, the portal's `ComposeEmail` answered 0 and Firefox
opened the `mailto:` link, subject and body in it, and asked what should
handle it. The first build offered Email… on the image sheet as well, and
there the same answer of 0 hid a failure: the portal handed Firefox a link
whose body was empty and whose `attachment=` was the PNG's local path, and
the picture went nowhere. Since the fix the image sheet has no Email… on
that desktop; with a fixture desktop entry in the Email category named as
the handler in the app's own scratch home, it offers Email…, and pressing
it sends the subject, the quote and its citation, and the picture (the
session's portal, which still names Firefox, then opened Firefox as
before). Also seen there: Tab walking Email…, Copy again and Done with
Escape and Return still closing the sheet, Copy again moving no button,
and Copy again and Done at the same pixels with and without Email…; and
the snap, packed from the same executable and installed for the run,
offering no Email… on either sheet. Not seen: a mail client's compose,
Wayland, and this sheet on Windows, where Windows' own Share sheet opened
for every verb in the app (the entry above) — neither the handler check
nor the 2,000-character link has run on Windows.

**Approved 1 October 2026.** The image sheet's heading, *Picture saved*,
is kept, and the three image lines beyond the wording above are approved:
where the picture is not in a folder called Downloads, *The picture is
shown in your file manager.*; and, where the file manager did not say it
showed the picture (the fix below), *The picture is saved in Downloads.*
and *The picture is saved in <folder>.*, the folder named by its path. So
are five behaviours of the fix:

- A download folder with another name (a localised *Téléchargements*, a
  renamed *Incoming*) is honoured: the picture is saved there. The sheet
  calls only a folder called Downloads by that name, so it gets the shown
  line or the path line.
- Where the file manager did not say it showed the picture — xdg-open
  failed, could not start or was not there, or the portal answered
  anything but 0 — the sheet says only that the picture is saved, in
  Downloads or in the folder it names, and never that it is shown.
- The sheet waits for the file manager's answer, at most five seconds, so
  where xdg-open runs the file manager in the foreground the sheet comes
  five seconds after the tap (the fix below).
- Outside a snap, on Linux — the tarball and the AppImage — a reader with
  no Downloads folder now gets the picture saved in the home itself, under
  its *BibleText verse* name, where before it stayed in the temp folder
  until the next card was rendered over it. That is a new place those
  builds write to: it gives the sheet a folder to name when the file
  manager does not open, and it is the snap's rule, so the two follow one.
- A picture no folder took that the file manager did show opens the sheet
  under *Picture saved*, over the line saying only that it is shown, though
  the file is still the copy in the temp folder, which the next card is
  rendered over. On Linux that is only where even the reader's home takes
  no copy; on Windows (the Share sheet's fallback) and the macOS mimic it
  is wherever there is no `~/Downloads`, as it was before the fix — a
  Windows reader whose Downloads folder has been moved, for one. The line
  says nothing of a save; the heading does, and is kept there rather than
  opening no sheet, as for a picture the file manager did not show.

One case stays outside the approved wording: a picture no folder could
take, which the file manager did not show, opens no sheet at all, its
cause logged, since no approved line says the picture could not be saved
(a short line, with a heading of its own, would need wording).

The keys after Tab are fixed on this sheet only: on every other sheet in
the app, a button that Tab has given the caret still takes Return and
Escape and answers only Space.

**Fixed 1 October 2026: Share as image in the snap saved nothing the
reader could find (found 30 September 2026).** Inside the snap `HOME` is
the snap's own `~/snap/bibletext/<revision>` (the reader's is in
`SNAP_REAL_HOME`), which has no Downloads folder, so `fallbackShareImage`
kept the PNG in the snap's private temp folder, which nothing outside the
snap can open; the file-manager reveal, xdg-open, opened nothing, since
snapd's xdg-open inside a snap hands on web links and not folders; and the
sheet said *Picture saved* and that the picture is shown in the file
manager, neither of which was so. Seen again on the arm64 VM with the tree
before the fix packed into the store's 1.2.17 snap: Downloads untouched,
the PNG only under `/tmp/snap-private-tmp/snap.bibletext/tmp`, no file
manager, and the line claiming one.

The fix, in four parts:

- **Where the picture is saved** (`share_image_folder.go`). The reader's
  Downloads folder is looked for in the reader's own home —
  `SNAP_REAL_HOME` inside a snap — first as the XDG download directory
  that home's `.config/user-dirs.dirs` names, read as xdg-user-dirs reads
  it (`$HOME` is that home, a localised name such as *Téléchargements* is
  honoured, and a line naming the home itself switches the directory off),
  then as `~/Downloads`. The snap's desktop plug may read `user-dirs.dirs`,
  and its home plug writes the home's folders that are not hidden. With no
  Downloads folder the picture is saved in the reader's home itself, and in
  the snap with the home plug disconnected, so that the home cannot be
  reached, in `SNAP_USER_COMMON` (`~/snap/bibletext/common`, kept across
  the snap's revisions); the file manager can open both. Outside a snap the
  same lookup runs on Linux, from `XDG_CONFIG_HOME`, so a tarball or
  AppImage reader whose download folder has a localised name gets the
  picture there too, and one with no Downloads folder gets it in the home,
  as in the snap: it used to stay in the temp folder, where the next card is
  rendered over it under the same name. Only when no folder takes the copy
  is the picture left where the share was handed it, in the temp folder,
  which the sheet never names (below). `SNAP` counts only on Linux: a stray
  one on Windows or the macOS mimic is passed over. Windows (the Share
  sheet's fallback) and the macOS mimic keep `~/Downloads` alone, and with
  none the copy they were handed (`docs/PLATFORM_MATRIX.md`, the Windows
  proof note under Sharing).
- **How it is shown** (`share_reveal_linux.go`). Inside the snap through
  the desktop portal: `OpenURI.OpenDirectory`, handed the picture as a
  read-only descriptor (the portal refuses a writable one from a sandboxed
  app, and one whose path the host cannot see), asks the file manager to
  show it selected (`FileManager1.ShowItems`), and the request's `Response`
  is the answer, 0 meaning shown. The request and its answer go through
  `portalRequest` (`share_portal_linux.go`), which Email… uses too.
  Outside a sandbox, xdg-open on the folder as before, its exit status now
  the answer; Explorer, whose exit status means nothing, still counts as
  shown once it starts.
- **What the sheet says** (`savedImage.line`). The sheet waits for that
  answer, at most five seconds (on the VM the portal answered in 0.37 s
  with the file manager not yet running), and then says *The picture is
  saved in Downloads and shown in your file manager.* only when both are
  so, *The picture is shown in your file manager.* for a picture shown in
  another folder, and, where the file manager did not say it showed it,
  *The picture is saved in Downloads.* or *The picture is saved in
  <folder>.* with the folder's path. Downloads is said only of a folder
  called Downloads; the XDG download directory under another name is
  another folder here. No line says a picture no folder took is saved, or
  names the temp folder it is in: shown, the line says only that it is
  shown, under the sheet's heading, *Picture saved* (approved, above);
  not shown, no sheet opens, and why is logged. It never says shown and
  then takes it back. Where xdg-open runs the file manager in the
  foreground, as it does on a desktop it does not recognise, the sheet
  waits the five seconds and then says only where the picture is saved,
  the file manager open beside it.
- **Off the UI goroutine** (`fallbackShareImage`, `shareImageAside`). The
  save and the reveal run off the UI goroutine, and the sheet back on it.
  snapd's home interface marks its rules for AppArmor prompting, so where
  prompting is on (an opt-in on recent Ubuntu releases) the first write
  into the reader's home waits until the reader answers a permission
  prompt; made on the UI goroutine, as the save had been, it would freeze
  the window until then. Before the fix the snap wrote only under
  `~/snap/bibletext`, which no prompt guards. The card is read at the tap,
  on the UI goroutine, since the next card is rendered over the same file.

**Held by** `share_image_folder_test.go`, on every host the suite runs on
(the Linux rules chosen through `shareImageGOOS`, the snap through the
environment snapd sets), each check shown failing on the case it tells
apart: the picture in the real home's Downloads and not the snap's home
(control: `SNAP` unset); a localised download directory, in the snap and
outside it (control: no `user-dirs.dirs`); the reader's home with no
Downloads folder (control: `SNAP` unset, the temp copy); the common folder
when the home cannot be reached (control: no `SNAP_USER_COMMON`); no claim
that the file manager showed a picture it did not, in Downloads, in the
home and in the temp folder (control: a file manager that said yes);
outside a snap the picture still in `HOME`'s Downloads whatever
`SNAP_REAL_HOME` says (control: `SNAP` set); the sheet waiting for the
answer with this share's mail; `user-dirs.dirs` parsing; the folders tried
per platform, a stray `SNAP` on Windows and macOS among them (control: the
same variables on Linux); a download folder called Downloads only when that
is its name (controls: no `user-dirs.dirs`, and a download directory
elsewhere called Downloads); outside a snap with no Downloads folder, the
picture in the home (control: Windows' rules, the temp copy); a picture no
folder took never said by the line to be saved, and no sheet unless it is
shown, in the snap and outside it (control: a file manager that said yes); and xdg-open's
exit status believed only on a success, with a command that fails, cannot
start or hangs past the wait (the success the control). Each check for the
name rule, the picture no folder took, the home outside a snap and a stray
`SNAP` fails with `share_image_folder.go` and `share_fallback.go` as the
snap fix first had them. `share_image_blocked_test.go` (not on Windows,
which has no named pipes) holds the save off the UI goroutine: with
`user-dirs.dirs` a named pipe, which keeps a read of it waiting until
something writes to it as a prompt keeps a write into the home waiting,
the share returns while the save waits, and the picture saved is the card
as the tap left it, though the next card has been written over the file
since. `TestShareImageAsideRunsTheWorkOffTheCallersGoroutine` holds that
the work leaves the caller's goroutine and that the sheet comes only after
it. Each of four mutations fails them: the save made before it is set
aside, the card read only when the save runs, the work run in place, and
the sheet not waiting for the work. `share_portal_linux_test.go`, in the
Linux CI job (which installs `dbus` for it, and fails it without
`dbus-daemon`), holds the portal: on a session bus of its own, a portal of
its own answers OpenDirectory and ComposeEmail as each case chooses, and
the reveal is believed only on 0, to this request, with the picture handed
read-only, an answer that comes before the call's own reply not missed;
Email… ends or hands on as its answer says. Each of five mutations — the
check on the code inverted, the picture opened for writing, any request's
Response taken, the Response listened for only after the call, a taken
compose handed on — fails it. godbus cuts a message short where it reads an
array of descriptors, so the test's portal cannot see Email…'s attachment;
the desktop's portal, GLib's, reads it. `redirectHome` now points
`XDG_CONFIG_HOME` into the test's home and clears `SNAP`, `SNAP_REAL_HOME`
and `SNAP_USER_COMMON`, and stops a test whose image share could save
outside it.

**Seen** on the arm64 VM (GNOME on X11, xdg-desktop-portal 1.18.4, Files
46), with the fixed tree's development build packed into the store's
1.2.17 snap (its `bin/bibletext` hashed equal to the build) and the preview
opened at launch (`BIBLETEXT_DEV_OPEN=share-image`): the picture landed in
`~/Downloads`, Files opened on Downloads with it selected, and the sheet
said it was saved in Downloads and shown; with `user-dirs.dirs` naming
`$HOME/Téléchargements` it landed there, and Files opened on Téléchargements;
with no Downloads folder it landed in the home, Files opened on Home with it
selected, and the line said only that it is shown; with the portal refusing
(the desktop's `disable-application-handlers` lockdown, answered NotAllowed)
it was in Downloads and the line said only that; with the home plug
disconnected it landed in `~/snap/bibletext/common`, which Files showed. The
same build unconfined, the tarball's route: saved in Downloads and shown
through xdg-open, and with no xdg-open on its `PATH` the line said only that
it is saved in Downloads. Not seen: Wayland, KDE or another file manager,
the AppImage itself, and a release build in the snap (the development build
differs only by the sheets it can open at launch). The portal route serves
any sandbox `linuxSandboxed` recognises; there is no Flatpak build to try it
in.

**Seen again** on the same VM the same day, with the name rule, the
picture no folder took and the home outside a snap in place, the same way
(the build hashed equal to the snap's `bin/bibletext`): in the
snap, the picture in `~/Downloads`, Files open on it selected, and the line
saying it is saved in Downloads and shown; with `user-dirs.dirs` naming
`$HOME/Téléchargements`, the picture there, Files open on Téléchargements,
and the line saying only that it is shown; with the home plug disconnected
and `~/snap/bibletext/common` made read-only, nothing saved, the portal
answering 2 to the snap's private temp file (a Response, not an error), no
sheet, and the log naming the refused copy; with the folder writable again,
the picture in it and Files open on it. The same build unconfined, with no
Downloads folder: the picture in the home, Files open on Home, the line
saying only that it is shown; and with no xdg-open on its `PATH`, the
line naming the home by its path.

**Seen a third time** on the same VM the same day, with the save and the
reveal off the UI goroutine, the development build packed into the store's
1.2.17 snap (its `bin/bibletext` hashed equal to the build): the picture
in `~/Downloads`, Files open on it selected, and the line saying it is
saved in Downloads and shown. A save made to wait —
`~/.config/user-dirs.dirs` replaced by a named pipe just before Share was
pressed, so that reading it waited until the pipe was written to — left
the window answering: the preview closed, Books opened while the save
waited, and once the pipe was written to the picture landed in Downloads
under the name of the moment Share was pressed, Files opened on it, and
the sheet came up over the Books page. A build from before the move,
unconfined, the same way: the preview stayed drawn, Books did nothing,
and GNOME said BibleText was not responding and offered to force it to
quit; once the pipe was written to, it went on as before. The new build unconfined behaved as it did in the
snap. Not seen: AppArmor prompting itself, which the VM's kernel and
snapd support (`snap debug sandbox-features` lists `prompt`, and snapd's
home rules carry the prompt marker) but which is off there and was not
turned on; and GNOME on Wayland, which can keep a window that did not ask
for the focus behind the one that has it: the portal is handed neither an
activation token nor a parent window, so Files may open behind BibleText,
with a notice that it is ready, while the portal answers 0 and the sheet
says the picture is shown.

**Sources.**

- https://github.com/flatpak/xdg-desktop-portal/issues/12 (the Share portal
  request) and the portal list,
  https://flatpak.github.io/xdg-desktop-portal/docs/api-reference.html
- https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.Email.html ;
  https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.OpenURI.html ;
  https://flatpak.github.io/xdg-desktop-portal/docs/for-app-developers.html
- Which backends provide Email, and the silent failure:
  https://github.com/flatpak/xdg-desktop-portal-gtk/blob/main/data/gtk.portal.in ;
  https://invent.kde.org/plasma/xdg-desktop-portal-kde/-/blob/master/data/kde.portal ;
  https://github.com/flatpak/xdg-desktop-portal-gtk/blob/ca24c36695e3dcde6a799b883a48a09cb164f536/src/email.c
- https://developer.gnome.org/hig/patterns.html ; https://invent.kde.org/frameworks/purpose
- What others do: https://gitlab.gnome.org/GNOME/gnome-maps/-/blob/main/src/sendToDialog.js ;
  https://github.com/fluttercommunity/plus_plugins/tree/main/packages/share_plus/share_plus/lib/src ;
  https://github.com/mozilla-firefox/firefox/blob/main/browser/components/sharing/SharingUtils.sys.mjs
- Notice lengths: https://gnome.pages.gitlab.gnome.org/libadwaita/doc/main/property.Toast.timeout.html ;
  https://invent.kde.org/frameworks/kirigami/-/blob/master/src/controls/private/PassiveNotificationsManager.qml
- Temporary files and portals: https://github.com/flatpak/xdg-desktop-portal/issues/1012 ;
  the snap's desktop interface, https://github.com/canonical/snapd/blob/master/interfaces/builtin/desktop.go

## Android: closing the Add a note sheet with the keyboard up left the app at keyboard height — FIXED 30 September 2026

**The defect** (found the same day, by the share audit). When the Add a note
sheet closed while the soft keyboard was up — by Share or by a plain Cancel —
the app stayed laid out as though the keyboard were still there: the reading
text squashed into a band, the Read / Books / Search bar mid-screen, and the
bottom 38 percent or so of the screen empty page. Six seconds later it was
unchanged; a tap on the empty part did not restore it; Home and back to the
app did. Reproduced on the emulator with 1.2.17, captured at each step: after
Back from the share sheet, six seconds later, after a tap, after Home and
relaunch, and the Cancel-only control with its own six-second capture. The
control shows the share sheet is not the cause; putting the keyboard away
with Back and then closing the sheet never showed it. It came and went from
one run to the next. Not checked on a physical phone.

**The cause.** Fyne's Android driver learns the window's insets in one
place: `GoNativeActivity.updateLayout`, run from a layout-change listener
on the decor, reads the system-window insets and hands them to the canvas,
which lays its content out inside them. NativeActivity makes its window
adjust-resize, and those insets then count a raised keyboard as the bottom
inset — on the Android 15 emulator, 883 px with the keyboard up against 63
without — so the canvas is laid out above the keyboard by whichever decor
layout pass happens while it is up. (The canvas's size never changes, so
the tablet test, which reads it, is untouched.) On that emulator the decor
and the content root keep the screen's full height under the keyboard — the
app targets API 36, and Android 15 draws such windows edge-to-edge — so the
keyboard's departure arrives as an insets dispatch and brings no layout pass
of its own. The one pass near it is the driver's, from hiding its EditText,
and that pass races the IME's inset update: it ran twenty-five milliseconds
before the inset was withdrawn, read the keyboard still in, and nothing read
the insets again — the canvas kept an interactive area 460 units tall on an
800 unit canvas. When the pass ran second the layout came back, which is why
the defect was intermittent. The 29 September entry below took Android never
to count its keyboard in the insets; Android 15 does. Whether older releases
count it, or lay the decor out again by themselves when the keyboard goes,
was not measured — no older image was to hand — and the fix costs nothing
where they do.

**The fix** (`installKeyboardWatcher`, `android/BtBridge.java`). The
keyboard watcher on the decor — installed from API 30, where the `ime()`
inset type it reads exists — sees every insets dispatch, and now asks the
decor for a layout pass whenever the keyboard's inset changes, so the driver
reads the insets as they are in the same traversal: the canvas comes back to
its full area within a frame of the keyboard going, and takes the keyboard's
arrival in the same frame instead of on the next incidental pass. Held by
`TestAndroidKeyboardInsetChangeAsksForALayoutPass`, a source contract on
the watcher, since the stale value lives in the driver's own inset where no
Go seam reaches it; the contract matches the request as a whole line, so a
request commented out fails it as a missing one does. Device check:
`docs/VISUAL_TESTS.md`, V12 (the note composer) and regression pin 20.

## iPad and Mac: where the share popover points — seen on both; the verse of the day's share FIXED 7 October 2026; Share with note's arrow FIXED 14 October 2026

Planned for: Share with note's arrow (the end of this entry) fixed for 1.2.22
(ROADMAP.md, 1.2.22, row 14); the rest was looked at and fixed in 1.2.20 — see
docs/ROADMAP.md. The iPad half was looked at on 6 October 2026 (below):
every selection verb points at the selection, and the verse of the day's
Share pointed at the middle of the screen, not at its icon. The Mac half was
looked at on 7 October 2026 (further below): the selection verbs point at the
selection, and the verse of the day's Share did nothing at all. That is a
Share that never works, so the fix for both went into 1.2.20 (the last
section of this entry).

Read from the code on 30 September 2026; no simulator was booted and the Mac
app was not launched. The first bullet's guess did not hold (the simulator,
below).

- **iPad, Share with note and Share as image.** `bibleTextPresentShare`
  (`reading_ios.go`) anchors the popover to the reading view's selected
  range and falls back to the middle of the view when there is none. On
  these two verbs the reading view gives up first responder before the
  share (the composer or the image preview opens first), which by its own
  comment clears the selection, so the popover most likely points at the
  middle of the page. V12 in `docs/VISUAL_TESTS.md` expects it beside the
  selection and is not ticked. Fix: take the selection's rectangle when the
  menu action fires and hand it to the share.
- **iPad and Mac, the verse of the day's Share.** The card's Share icon
  shares through `sharePassageText`, and the popover (iPad) or picker (Mac)
  is anchored to the reading view — `gReadingTV`, `gTextView` — which is
  hidden while the card is up, not to the icon; with no selection both fall
  back to the middle of that view. On the Mac it is not known that the
  picker shows at all while the view it is anchored to is hidden. Likely
  fix: anchor to the icon's rectangle, which the Fyne side knows.

Share with citation and Share as link keep the selection and are expected to
point at it on both. See each on a device before changing anything; the
Sharing table in `docs/PLATFORM_MATRIX.md` marked the four cells above
"probably": the iPad's Share with note and Share as image, and the verse of
the day on iPad and on Mac. The Mac's is still marked so.

**Seen on the iPad simulator, 6 October 2026.** iPad Pro 13-inch (M5),
iPadOS 26.5, portrait, the World English Bible at Psalm 82, a development
build of `main` at 1db51a02b. The simulator takes no taps, so a throwaway
trigger compiled into that build alone, and never committed, stood in for
the reader's hand: it made the reading view first responder and set its
selected range to 14 characters in verse 3, starting and ending mid-word,
as a touch selection would not, which does not move the popover's anchor;
ran the body of the selection menu's handler for the verb
(`bibleTextStudyMenuTapped` with the text and verse span the menu
computes); and then pressed the Share button of the preview, the composer
or the verse-of-the-day card itself (`Tapped` on the widget each had
built). The presented `UIActivityViewController`'s source view and
rectangle were read back and logged, and each popover was screenshotted.

- **Share with citation and Share as link.** Beside the selection, the arrow
  at its right end: the source rectangle is the selection's first rectangle.
  As expected.
- **Share as image and Share with note.** Also beside where the selection
  was, not mid-page. Giving up first responder hides the selection but
  leaves the text view's `selectedRange` set, and the reading view is shown
  again before the share is presented, so `firstRectForRange` answers the
  selection's rectangle; the comment in `bibleTextDismissMenu` that it
  "clears the selection" holds for what is drawn, not for the range. With a
  note written, the "Note from you" card is put on the page above the verse
  after the rectangle is taken, so the arrow points at the new card, where
  the selected words had been, with the verse one card lower. Nothing to
  fix as seen.
- **The verse of the day.** The popover's arrow points at the middle of the
  hidden reading view — the middle of the screen, over the lower half of the
  card — and not at the card's Share icon at its top right, and the popover
  covers the card's text and its Close and Read in context row. The same
  with no selection and with one made before the card opened: the hidden
  text view answers no rectangle for the range it keeps, and the fallback,
  the middle of the view, is taken. This is the case predicted above; the
  fix is the icon's rectangle, for 1.2.21.

Not seen on the iPad: landscape, where the reading view is the right-hand
pane beside the sidebar; Split View and Slide Over widths; a selection made
by touch rather than set on the view; and a note typed in the composer's
field, since the trigger filled the note in, so a keyboard closing as Share
is pressed was not seen. Also seen and not explained: in the Share as image
popover the share sheet's header showed the system's blank image
placeholder, not the card's thumbnail, though Save Image was offered. The
card's file is not deleted after the share (`share_image.go`), so this is
probably the simulator; worth a glance on a device.

**Seen on the Mac, 7 October 2026.** macOS 26.6.2, the light appearance, a
1280x892 window, the World English Bible at Psalm 82, a development build
of ebc9c98f3. No synthetic input was sent to the desktop, so, as on the
iPad, a throwaway trigger compiled into that build alone selected 14
characters inside verse 3, starting and ending mid-word, ran the body of
the selection menu's handler for each verb (`hbSendStudy`, what a pick from
the Control-click menu runs, so `dispatchSelectionAction` ran unchanged),
and pressed the preview's, the composer's and the card's own Share. The
picker's window was read back from the application's window list, and each
window the app owns was rendered in-process (`cacheDisplayInRect:`, as
`debug_capture_macos.go` does), which draws the picker's frame and arrow at
their true place but not its service list, a remote view, nor the Fyne
canvas.

- **Share with citation and Share as link.** At the selection: the arrow,
  the picker frame's top-centre vertex, 0.7 pt below the selection
  rectangle's bottom edge and 0.4 pt from its centre; the picker hangs
  below the words.
- **Share as image, then the preview's Share.** At the selection, the same
  numbers: the selected range survives the modal preview (the view is
  suppressed, not cleared) and the overlay is shown again before the
  queued presentation runs.
- **Share with note, then the composer's Share.** Beside where the
  selection was: the arrow 52 pt above the shared words, inside the new
  "Note from you" card, and the picker's body over the verse. The
  presentation is queued on the main queue by `shareTextOut` before
  `showSentNote` runs; when it runs the arrival scroll has happened but
  the card has not yet pushed the words down, so the anchor is the words'
  place before the card — the iPad's arrow at the new card, the verse one
  card lower. The selection is empty afterwards, the storage replaced.
- **The verse of the day, with and without a selection.** No picker at
  all. `bibleTextShareText` asked for the picker relative to the reading
  view, hidden under the card (`isHiddenOrHasHiddenAncestor`, a zero
  `visibleRect`, so the no-selection fallback was the view's origin); the
  call returned, the window list stayed at the app's own windows, nothing
  appeared and nothing was logged. The share was lost without a word. The
  control: the same instrumented call for Share with citation is followed
  by the picker's window.

Not seen on the Mac: the Control-click menu itself; a selection made with
the mouse; the dark appearance; other window sizes.

**Fixed 7 October 2026, in 1.2.20: the verse of the day's share on the Mac
and the iPad.** The card hands its Share icon's rectangle over for the
length of its share (`shareAnchor`, `share.go`), read from the canvas at
the tap, and the two panes present beside it: the Mac from the content
view the canvas fills, which is on screen while the reading view is
hidden, with y flipped for the unflipped view and the picker hanging below
the icon (`bibleTextShareTextAt`, `reading_macos.go`); the iPad from the
presenting view controller's view, the rectangle moved inside the window's
safe area as the reading view's frame is (`reading_ios.go`). A selection
share hands no anchor and keeps the selection, and the Android, Windows
and Linux shares never read one, so nothing changes there
(`reading_android.go`, `share_windows.go` and `share_other.go` take the
text as before). Seen with development builds of fa4072f73 the same day,
through the same triggers: on the Mac the picker's window now appears
below the icon, its arrow 0.8 pt below the icon's bottom edge and 0.5 pt
from its centre, with and without a selection, and Share with citation's
picker is where it was to the point; on the iPad the popover's source
rectangle is the icon, 33x22 points, 32 points below its canvas position
for the safe area, and the popover opened to the left of the card with its
arrow at the icon's left edge. Held by `share_anchor_test.go`: the card's
share is handed over with the icon's rectangle, inside the card and not
the origin, and nothing is left behind; a share with no control behind it,
or an empty rectangle, has none; and, read as source, the two Apple panes'
`nativeShareText` read the anchor and present beside it while the other
three never read one.

Share with note's arrow in the new note card on both: **FIXED 14 October
2026, for 1.2.22.** Moving the presentation after `showSentNote` was not
enough, since by then the selection is gone with the replaced storage and
the fallback would point at the middle of the view. The share now carries
the selected words' place in the pane's own text (`wordsAnchor`,
`share.go`: the verse the selection begins in, the lead's length and the
selection's, in UTF-16 units), and the two panes hold the sheet back until
the re-import, finding the words again in the new text 0.35 s after it and
presenting beside them (`bibleTextShareTextAtWords`); with no import a
second later, the sheet is presented at the selection as before. An iPhone
presents its sheet at once, as before. Seen with development builds and
the same kind of trigger, with and without the change: the Mac's picker
0.8 pt below the words against 88 pt above them, the iPad's source
rectangle on the words against 89 points above them (docs/PLATFORM_MATRIX.md,
the Sharing table).

## Android: the status-bar icons were white on the light page — FIXED 30 September 2026

Still open: the three-button bar's white buttons after a navigation-mode
change — stays, watch only; see docs/ROADMAP.md.

In the light theme the clock, battery and signal icons were white on the
cream paper, 1.19:1 on the Android 16 emulator, and a three-button
navigation bar's buttons were as faint; the light Play images show it. The
bars have no background here — the NativeActivity window's view hierarchy
never draws, so the canvas's paper shows through them (the landscape
cutout entry below) — and nothing asked for their icons, which therefore
came from the activity's theme, the platform's dark one, on either page. A
gesture bar's handle samples what is under it and was already dark.

The icons now follow the page: dark over the light paper (5.44:1 measured),
light over the dark (17.9:1). Android's system bars take the Windows title
bar's place in the window-chrome seam (`followTitleBar`,
`title_bar_android.go`): `chromeAtStart` records that they start in the
dark theme's appearance, so a light page has them sent at startup, and every
rebuild that moves the variant sends them again.
`BtBridge.setSystemBarsLight` sets `APPEARANCE_LIGHT_STATUS_BARS` and
`APPEARANCE_LIGHT_NAVIGATION_BARS` on API 30+ and the
`SYSTEM_UI_FLAG_LIGHT_*` flags on 23–29 (the navigation one from 26), keeps
the answer, and gives it to a recreated activity's new window from `init`.
Seen on the Android 16 emulator: light and dark launches, the system
switched both ways while the app ran, a rotation to landscape and back, and
the activity recreated in the same process by a font-size change — which
went back to white icons in a control build without the re-apply in `init`.
The rotation did not recreate the activity on that emulator. Held by
`android_system_bars_test.go`, and by `android_system_bars_polarity_test.go`
for the value's sense through the Go wrapper, the C shim and `BtBridge`'s
store and read-back: any one of them turned round brings the white icons
back while the Go helper's tests still pass.

Not seen below Android 15; no older emulator was driven for this. AOSP's
SystemUI darkens a bar's icons only while that bar is transparent, so where
a release draws a bar opaque itself its icons stay light whatever is asked.
Still open: after the navigation mode was switched while the app was open
(gesture to three-button), the new three-button bar kept white buttons until
the app was next started, the status bar correct throughout. The activity
was recreated with the flags set, so this looks like SystemUI's new bar
missing the appearance; not investigated further. The light Play images of
1.2.17 keep their white icons until a set is taken from a release with this
fix (`docs/SCREENSHOT_PLAYBOOK.md`, §7).

## Android: a text share has no failure path — FIXED 30 September 2026

A presentation iOS refuses and a card that fails to render say so since 15
October 2026, for 1.2.23, with the other shares that cannot finish (the
entry on Email… on the desktop share sheet; docs/ROADMAP.md, Later minor
releases, row 2).

`BtBridge.shareText` (`android/BtBridge.java`) calls `startActivity` with
no try/catch, so a failure to start the chooser goes uncaught on the UI
thread and the reader is told nothing. `shareImage` beside it wraps the same
call in a try/catch that shows "Could not share the card." through
`shareNotice`, part of the 20 September 2026 fix recorded below ("A share
the reader started cannot end in silence, whatever the cause"). That catch
is the difference. Both methods open with the same bare return when there is
no activity, and neither can say anything there: with no activity there is
nothing to show a message on. Never seen to fail; this is hardening under
that rule: catch around `startActivity` in `shareText` and show the same
kind of message `shareImage` shows. The same reading of the iOS bridge finds
`bibleTextPresentShare` returning silently with no view controller to
present from, and `bibleTextShareImageFile` with an image it cannot read
(`reading_ios.go`) — worth the same treatment. Every text verb on Android
goes through `shareText`: note, citation, link and the verse of the day.

**FIXED 30 September 2026.** It was worse than silence. With the system
chooser disabled on the Android 16 emulator (`pm disable-user` on
`com.android.intentresolver`, re-enabled after), Share with citation in
1.2.17 threw `ActivityNotFoundException` out of the posted Runnable and the
app closed. `shareText` now starts the chooser inside a try whose catch logs
and shows "Could not share the passage." through `shareNotice`, the words
matched to Share as image's "Could not share the card."; the same run on the
fixed build showed that toast and the app stayed open, Share as image showed
its own, and with the chooser enabled the sheet opened as before. On iOS,
`bibleTextShareImageFile` with an image it cannot read now says "Could not
share the card." in an alert with an OK button (`bibleTextShareNotice`,
presented from the view controller the share sheet would have used), since
iOS has no toast; that is compiled (`scripts/check-ios-pane.sh`) and held at
the source, not seen. `bibleTextPresentShare` with no view controller keeps
its bare return, as Android's missing activity does: the view controller is
nil only when `bibleTextFindWindow` finds no window in an attached scene, and
then there is nothing on screen to say anything in. Held by
`share_failure_notice_test.go`, each check failing with its line taken out.
Still open: a presentation UIKit refuses (a view controller already
presenting, or being dismissed) fails silently on iOS and is not detected.
And on every platform, a Share as image card that fails to render
(`renderVerseImage` returning an error: a font that will not parse, a file
that cannot be written) leaves the preview empty, and its Share then closes
the sheet and hands nothing over, without a word (`share_preview.go`).

## A phone sheet reopened under the keyboard ended at the keyboard's top — FIXED 29 September 2026

Every phone sheet with a field — the note composer, Settings, Ask, and the
Go to picker — and Verse of the day take their height from the canvas's
interactive area as they open, and none is sized again for the soft keyboard.
Fyne's iOS driver reports a raised keyboard as the whole bottom inset
(`getDevicePadding`: `inset.bottom = keyboardHeight`) and moves it back only
from the later `UIKeyboardWillHide` notification. An ordinary open never reads
the area that way, since the keyboard comes up after the sheet is sized; the
light/dark reopen did. `observeAppearance` unfocuses the canvas, rebuilds and
runs the sheet's reopen in one call, before the keyboard's notification has
moved the inset, and with the composer's native field the keyboard is not even
taken down (the driver resigns its own input view, not the note's). The
reopened sheet was sized to the canvas less the keyboard and stayed that
short once the keyboard went down — the composer's refit keeps the foot gap
it opened with, Settings is sized again only on a status change, and Ask and
Verse of the day never — so the page showed beneath it, and a tap there closed
the non-modal composer or Ask sheet with what had been typed. Reached by a
change made in the foreground (a scheduled switch, Control Centre on an iPad)
or reconciled on return while a field had the keyboard; Android never counts
its keyboard in the insets, so only iOS showed it.

**The fix** (`sheetArea`, `sheet_fit.go`). Every phone sheet reads the area
through `sheetArea`, which remembers the canvas's bottom inset from each read
made with no keyboard in it — the keyboard reported down by the observers
(`softKeyboardShown`) and the inset no deeper than a safe inset can be — and,
on a read that finds the inset deeper than that foot, gives back the
difference. The keyboard's own height is not subtracted, since the driver
replaces the safe inset with it rather than adding to it. The driver's inset
and the observers' report travel by different paths, so either can be a frame
ahead; a keyboard-deep inset is never remembered as the foot, and a read made
before any foot is known gives the area as reported, which is what every read
gave before. `noteSheetArea`, the composer's seam, defaults to it. Held by
`TestPhoneSheetsReopenedUnderTheKeyboardKeepTheirFoot`: each of the six sheets
opened on a phone with the keyboard down, its box read, the keyboard raised in
the inset and in the report, the window flipped, the keyboard dropped and the
watchdogs run, and the reopened sheet spans what the first did; and
`TestSheetAreaGivesTheKeyboardBack` for the rule itself. Device check:
`docs/VISUAL_TESTS.md`, V15.

## A Test key started before a light/dark change reported to no one — FIXED 29 September 2026

Under each key field in Settings, Test key shows "Testing…", then the model
the request reports (the assistant's key), then its verdict — and those were
painted by closures of the sheet the button was tapped on, into that sheet's
own line. A light/dark change drains Settings and reopens a fresh one
(`sheet_reopen.go`), so a test tapped before the change reported to a sheet
nobody could see: the reopened sheet showed neither the wait nor the verdict,
and the reader had no sign the test had finished; a second tap ran a new test
and showed that one. Settings otherwise loses nothing to a reopen, since
everything in it saves as it changes; a test in flight was the one thing that
did not.

**The fix** (`key_test_progress.go`). What a test has shown so far — running,
the model it reported, its verdict — lives on AppState, one slot per key
section, and the sheet that is showing registers how it paints the slot's line
and paints it as it is built. So a reopened sheet shows the wait of a test
still running and the verdict of one that landed while no sheet was up, and a
test's reports paint through whichever sheet is showing when they arrive. The
slot is forgotten when the sheet is closed by its ✕ and when Settings is opened
as the reader opens it (`showAISettings`; the reopen calls `openAISettings`
and keeps it), so a later visit never opens on a stale verdict. A test tapped
again before the last one answered still owns the line: the older test's
reports find another test in the slot and are dropped. Held by
`TestTheKeyTestOutlivesALightDarkReopen` (desktop and phone: the wait, the
model's report and the verdict on the reopened sheet, the ✕ forgetting it,
the next open showing nothing) and `TestTheAPIBibleKeyTestOutlivesALightDarkReopen`
(the API.Bible key, its probe held at a local server), with
`TestTheKeyTestNamesTheModel` still holding the second test's ownership.

## The Go to picker's reopen raised a put-away number pad — FIXED 29 September 2026

The picker's reopen put the caret back in the verse field that had it,
which on a phone raises the keyboard, without the gate a page field's caret
has (`pageCaretComesBack`): Fyne still counts a field focused after the
reader has put the keyboard away with Back on Android, so a light/dark change
then threw the number pad back over the reopened grids unasked. The capture
now asks the same question the page fields ask, from the keyboard report read
at the take, before the gate's unfocus; the typed verse comes back either way,
and on desktop, where focus raises nothing, so does the caret. On iOS a
focused number pad cannot be put away, so the two states never diverged
there. Held by `TestAppearanceChangeDropsTheCaretAndPutsItBack`.

## A light/dark change with a sheet open left the app half in each theme — FIXED 27 September 2026

Still open: the iOS fallback's old colour (stays); see docs/ROADMAP.md. The
Mac's follow-scroll with the pane hidden and the Android night-mode patch's
race were FIXED 14 October 2026, for 1.2.22 (the bullets below).

With a sheet open — Go to, Verse of the day, the translation list, Settings —
a system switch between light and dark left the app in both at once. The sheet
kept the colours it was built with while Fyne re-lit its stock widgets and
everything resolved through a theme colour name, so Go to became a dark card
with dark-on-dark letters and light entry boxes, and the page behind the sheet
was mixed the same way until the sheet closed. Reproduced on the iOS simulator
and on an iPhone.

**The cause.** `ObserveSystemThemeChanges` sent the rebuild through
`deferOrRebuild`, which parks it while any overlay is open and runs it when the
sheet closes. That deferral was there for the app switcher: iOS renders a
backgrounding app in both appearances for its snapshot, so the variant flips
away and back, and rebuilding for each leg drained the sheet the reader had
left open. The deferral kept the sheet, and in exchange every REAL change made
with a sheet up waited, half applied, for as long as the sheet stayed.

**The fix** (`appearance.go`, `sheet_reopen.go`). On iOS and Android a change
heard while the app is out of the foreground is ignored at the time; the gate
closes at the start of `OnExitedForeground` and opens at `OnEnteredForeground`,
which reconciles once: the settled variant against the one the window was last
built with (`rebuildWindow` records it, whatever asked for the rebuild). A round
trip nets to nothing and the sheet stays exactly as it was; a real change —
overnight, by schedule, or from Control Centre, which takes the app out of the
foreground while it is open — rebuilds. A change heard in the foreground
rebuilds at once. Every light/dark rebuild takes the reopen closure of the sheet
on top, drops the caret (so a phone's keyboard goes down with the sheet),
rebuilds (draining it), and reopens it in the new palette showing the same
thing. Desktop has no snapshot, so the gate never closes there and a window
behind others still repaints. The listener's closures run in no fixed order
against the updates and the lifecycle hooks, so each reads the live variant when
it runs; `TestAppearanceRoundTripInEveryOrder` walks all eight orders of the
round trip's two closures through the gate and through a real window with the
Go to picker open. On Android the return is heard BEFORE the update that
carries a change made while away (the redraw queues the lifecycle event ahead of
the size event), so there the reconcile finds nothing and the update's own
closure rebuilds — still once. `deferOrRebuild` stays for the other
translations' background upgrade (D17), which is not a theme change.

**What reopens:** Go to, both flavours (the same book and chapter selected, the
navigator at the same letter, the verse fields' text, the caret back in the
field that had it); Verse of the day (the same verse, even after midnight); the
translation picker (its notice read again from state, since the rebuild may be
the one that paints a download that landed under it, and no second manual
retry); Settings, at its top; the audio source menu; cross-references; the Ask
sheet with its typed question; the note composer with its note, on every
platform (a drained composer tears down only the native field it still owns);
an AI answer that has landed (from aiCache, no second request); the Downloading
modal, until its download lands (the download's dismissal closes whichever copy
is up); the note-link offer; the link notices, and the seed park's notice only
while its passage is still waiting; the translation-download error. A sheet
registers when it opens, and the registration dies with its popup, however it
closes. Fyne's menus (an entry's Cut/Copy/Paste, a Select's list) are not
sheets: the take looks past them to the sheet beneath. The desktop share
confirmation was not one either when this was written, a notice that took
itself down; since 30 September 2026 it is a registered sheet, which comes
back with the same text, and with the verse-of-the-day card beneath it when
it was opened from there (the Linux and Windows share-sheet entry above).
**What closes:** an AI answer still in flight (its own landing
already reopens it); the AI panel showing an error or asking for a key
(reopening would send the request again); the share-image preview; the model
picker and the notes questions over Settings (only the top sheet ever comes
back, and these answer into the one beneath); the menus themselves.
Responsive-layout rebuilds capture and reopen the registered top sheet too,
preserving entered questions and notes and completed study answers.

**Outside the sheets, what a switch also left behind, and does no longer:**

- **The Windows title bar** kept the immersive dark mode the window was
  created with: Fyne sets `DWMWA_USE_IMMERSIVE_DARK_MODE` once, at creation,
  and its settings listener never again, so a dark page sat under a white
  bar (or the reverse) until relaunch. Every rebuild that moves the variant
  — whatever asked for it, so a tab tapped in the moment before the
  listener's closure runs cannot swallow the change — now sends the
  attribute again through the window's own handle (`followTitleBar`,
  `title_bar_windows.go`, the `RunNative` seam `restoreNative` uses), then a
  `WM_NCACTIVATE` pair that leaves the caption drawn active or inactive as it
  was: Windows 10 repaints a visible caption only on an activation change,
  and a frame-changed `SetWindowPos` or `RedrawWindow` does not do it.
  Windows 11 applies the attribute at once; the pair is sent there too.
  Attribute 20 only, as Fyne sends: builds before 20H1 never had a dark
  frame at launch either. Seen working on the Windows 11 VM (28 September
  2026: the released 1.2.16 kept a white caption over a dark app after a
  switch; this code follows dark and back to light, with the app focused
  during the switch too). Windows 10 22H2 is still unchecked
  (`docs/VISUAL_TESTS.md`).
- **The styled pane's note card** (Windows and Linux, plainly in focus mode,
  where no widget's theme override happened to clear the cache) could keep
  the old palette: Fyne caches a rasterised SVG by resource name and pixel
  size, a switch clears that cache and then refreshes the OLD tree —
  re-rasterising the old card's old-palette bytes under the same name — and
  a rebuilt card, same name, same size, rasterised after that refresh is
  handed that raster. It did not show on the Linux VM, before this change or
  after it (focus mode, light → dark → light, card and tail matching the
  page each time), so which runs first decides it; the name below takes the
  order out of it. The generated SVGs
  with literal colours (`note-bubble`, `note-tail`) are now named after
  their look — fill and stroke with alpha, and the shape — never their size
  (`lookNamedSVG`): Fyne's own size check replaces a raster at another size
  under the same name, where a name that folded in the width would keep one
  full-card raster per width through a resize drag (over 100 MB for one
  sweep, measured). The
  notes browser's tails were never caught — each row is its own theme
  override, whose rasters Fyne keys under a scope no rebuilt row shares — so
  the tail's rename is hardening. The themed icons carry a colour name and
  were never affected.
- **macOS** showed the NSTextView over the Books and Search tabs whenever a
  sheet closed there — its restore closure asked `!IsSearching`, the former
  sidebar's rule — holding the chapter it last had, which after a switch made
  on that tab was in the palette just left. All three native panes now
  restore through one function that asks `overlayShouldShow`
  (`restoreNativeReadingOverlay`), and macOS will not show a pane holding
  the other palette's chapter until the Read build's push replaces it
  (`nativePaneStale`) — so its light/dark rebuild on Read imports into the
  hidden pane, the launch path. The same-chapter capture that keeps the
  reader's place across that import read the text view's `visibleRect`,
  which AppKit gives as zero for a hidden view: a switch made with a sheet
  open, or on Books before going back to Read, put the reader at verse 1.
  It reads the clip view's bounds now, which a hidden pane keeps (a Cocoa
  probe, scrolled to 500 and hidden: visibleRect 0, clip bounds still 500).
  The reading-state flush
  asks the same capture, so a window closed from the Books tab or over a
  sheet should no longer save the top either (follows from the code; not
  run).
- **Android's Study with AI popup** (Explain / Analyze context / Analyze
  translation) floated on in the old palette over the re-lit page; `setStyle`
  now closes it when the text or paper colour moves, and `hide` and
  `suppress` close it with the Dialog. Seen on the Android 15 emulator
  (29 September 2026): 1.2.16 keeps the white popup over the dark page
  four seconds after the switch; this build closes it.
- **The Search tab's Find** is rendered from state (`renderFind`), in
  `buildSearchResultsView`'s order, and a landing repaints through the tab on
  the canvas (`state.repaintFind`): a rebuild mid-Find had brought back the
  empty prompt with no spinner and no Cancel while the answer painted into
  the detached tree, and an error card or a finished "found nothing" came
  back as the prompt too. A Find stopped by Cancel, a mode switch or leaving
  the tab stays a stopped Find, never "found nothing". Each rebuild of the
  tab mid-Find draws a bar of its own, so the bar is held on state
  (`state.findBar`, `stopFindBar`) and every stop reaches the one on the
  canvas: on the phones the root is a widget (`layoutWatcher`) that a walk
  over the old tree would stop at, and a rebuilt tab's bar would run on after
  the reader left the tab. `rebuildWindow` also stops it, and its walk goes
  into that root (`contentWrapper`).
- **The caret** is dropped before every light/dark rebuild, not only with a
  sheet up, so a phone's keyboard no longer stays up typing into nothing; a
  page field that had it (Search, Find, the notes filter, the Books filter)
  gets back what was typed, and the caret too — on a phone only if its
  keyboard was up (`pageCaretComesBack`, from the keyboard reports the Go to
  picker already had), since focus raises the keyboard there and the reader
  may have put it away with Done or Back. **The Books grid** keeps its
  scroll offset across rebuilds (`booksScrollY`).

**Left open, on purpose:**

- On iOS and Android the rebuild un-suppresses the native pane before its
  re-render lands, so a frame or so of the OLD palette's chapter can show —
  over the sheet before the reopen suppresses it again, or over the new
  chrome when the reader goes to Read after a switch made on Books or
  Search. Keeping the pane down until the push lands means importing into a
  hidden view, which is the path the reading-position restore and the
  Android note sticker skip, or a native pending-generation gate that, if it
  ever misses a landing, leaves the pane invisible. Neither can be proved
  without a device; `appearance.go` says so beside the call, and
  `docs/VISUAL_TESTS.md` asks the device question. macOS does not flash (its
  hide, its import and its show drain in one main-queue pass).
- If all three of an iOS rebuild's HTML imports fail, the plain-text
  fallback takes its text colour from the previous import — the old
  palette — over the new paper, until the next push. Theoretical: the
  failure rate has never been measured and only single failures are known.
  Setting `textColor` explicitly on the fallback (and the macOS twin's
  foreground attribute), or recording the body fingerprint only once the
  native side reports the import applied, would close it. P3, fixed for
  1.2.23 (`12bd8ea67` on `fu/1223-ai4-p3`), sets the fallback page's text
  and verse-number colours from the palette of the push that built it, on
  both panes, which closes this with that release.

**Found alongside, not fixed here** (all older than this change):

- macOS read-along's follow-scroll places the narrated verse from the text
  view's `visibleRect` too (`bibleTextMacHighlightVerse`), which a hidden
  pane gives as zero. If it follows while the pane is hidden — narration
  playing with the reader on Books or Search, or under a sheet — it reads a
  zero-height viewport and scrolls each verse to the very top instead of
  keeping it in the comfortable band. From the code alone: whether follow is
  asked while the pane is hidden was not traced. The fix would be the
  capture's, reading the clip view. FIXED 14 October 2026, for 1.2.22
  (`e3104ec92`; ROADMAP.md, 1.2.22, row 28): follow is asked while the pane
  is hidden (the audio controller's `onTimeUpdate` passes follow whatever
  the pane shows), and the follow-scroll reads the clip view now.
  `TestTheMacFollowScrollReadsTheClipViewNotVisibleRect`. Seen with
  development builds, narration following Psalm 119:60 with the reader on
  Books: before, the verse 0 points below the top of the view; after, 191
  points down a 636-point view, where it still stood back on Read.

- On Android, an activity destroyed while the process lives on (a swipe-away
  with the audio service holding the process) sends `OnStopped`, which sets
  `state.stopping`, and nothing cleared it for the next activity. DONE 27
  September 2026: the next activity's start clears it — see "An Android
  activity destroyed with the process alive left the app stopping" below.
- The Android night-mode patch (`patches/fyne-2.7.4-android-night-mode.patch`)
  compares `currentSize.DarkMode` with `darkMode` in the configuration-change
  branch, but `GoNativeActivity.onConfigurationChanged` calls `super` (which
  hands the change to that branch) BEFORE `updateTheme` writes `darkMode`: the
  read races the write, unsynchronised, and when it loses a foreground change
  waits for the next redraw. Reading the night bit from the `AConfiguration`
  the branch already builds (`AConfiguration_getUiModeNight`) was the fix
  proposed here, and does not work: the asset manager's configuration does
  not carry the switch, and a first attempt built that way left the app
  light through `scripts/test-android-dark-mode.sh`. FIXED 14 October 2026, for
  1.2.22 (`aa2da8d5c`; ROADMAP.md, 1.2.22, row 29): the patch moves
  `updateTheme` ahead of `super` in `onConfigurationChanged`, so the flag is
  written before the configuration is sent and the send orders the two.
  `TestAndroidNightModePatchWritesTheFlagBeforeTheConfigurationIsSent`; the
  dark-mode check passes both ways, and 20 switches 1.5 s apart on the
  emulator were each followed, as by the build before. The race itself was
  not seen there.

## An Android activity destroyed with the process alive left the app stopping — DONE 27 September 2026

`InstallReadingStateFlush` set `state.stopping` in `OnStopped`: the flag that
tells late background work the app is tearing down, so a result landing then
drops itself. On Android that hook runs when the ACTIVITY is destroyed, and
the process can live on: the audio service holds the process through a
swipe-away, Back finishes the root activity on Android 11 and older, and the
system reclaims an activity in the background. A rotation does not: the
manifest's `configChanges` lacks `screenSize`, yet on the Android 15 emulator
a rotation kept the same activity instance and process (27 September 2026,
the released 1.2.16 build, `adb emu rotate`: no create or destroy event, the
same `Local Activity` in `dumpsys activity top`). Fyne runs
`main` once per process, so the next activity started in the same process, on
the same state, and nothing cleared the flag. For the rest of the process the
appearance gate returned at once, so a light/dark change was never applied;
the refresh (`triggerFullDownload`, `ensureUpgradeScheduled`) stood down; and
a translation download that landed between the two activities was dropped
with `versionLoading` still set, which left the Downloading spinner up in the
next activity and refused every later download as already in flight.

**The lifecycle, traced** in Fyne 2.7.4's drivers and the repo's patches.
Android: each activity's first redraw takes the stage from Dead straight to
Focused, one event that runs `OnStarted` and then `OnEnteredForeground`; its
`onDestroy` takes the stage to Dead, `OnStopped`; going to the background and
back moves between Focused and Alive and runs the foreground hooks alone; and
a real exit is the process being killed, which runs no Go, so from Go it is a
stop that no start follows. iOS, under the scene life-cycle patch: `OnStarted`
once at launch, `OnStopped` only from `applicationWillTerminate`, and a scene
the system disconnects goes back to Alive, not Dead. Desktop: `OnStarted` once
as the run loop begins, then the close intercept and `OnStopped` as it ends.
Only on Android does anything start after a stop.

**The fix** (`activity_life.go`, `app.go`). `OnStarted` clears `stopping`:
the new activity is live, whatever an earlier activity's stop said. It is the
one point that can tell. At the stop, a destroyed activity and a process about
to be killed look the same; `OnEnteredForeground` also runs on every return
from the background and, on desktop, on every regained focus, which would
clear a desktop teardown. On desktop and iOS the one start comes before any
stop and changes nothing, and a real teardown still sets the flag with nothing
to clear it after. One order needed its own answer: the driver runs
`OnStarted` inside the lifecycle event but QUEUES `OnStopped` on the
lifecycle's own queue, which hands it back to the UI goroutine, so when a
recreation's destroy and create are both waiting as the UI loop comes free, the
old activity's stop can run after the new one's start. Each start therefore
registers the stop hook for its own activity, with the activity's generation.
The driver takes the hook as the stage crosses to Dead, so a stop for an older
activity than the one started last still flushes the reading position and
stops the audio, as before, and leaves `stopping` clear.

**What the next activity picks up.** Every `stopping.Load()` consumer was
checked. The appearance gate: `OnEnteredForeground` follows `OnStarted` in the
same event and reconciles, and a change carried by the new activity's own size
event arrives after that and rebuilds. The refresh: the same hook retries it,
and a landing `upgradeLanded` dropped in between had written its cache first,
so the retry reads it from disk (or fetches again if that write failed, D6). `consumeDeferredFullRebuild`: its flag waits
for the next sheet close, `refresh()` or rebuild, and the Android overlay
recovery rebuilds as soon as the native pane finds itself in a new activity. The interactive translation load
(`switchVersionInteractive`, `versions_ui.go`) was the one left half done, and
nothing retried it: its landing is now held while stopping
(`holdWhileStopped`) and landed by the next start, through `fyne.Do`, so it
runs after the start's own event, as it would have run had it landed while the
activity was live. After a real exit nothing lands it. On desktop, where the
glfw driver runs the landing inline off the main thread once the loop has
drained, it is held and never applied, as it was dropped and never applied
before.

`OnStopped` still stops the audio and flushes the reading position on every
activity destroy. Whether an activity destroy should stop the audio is a
separate question, recorded under "Android: an activity destroyed with the
process alive stops the narration" below; since 6 October 2026 it does not, on
Android.

The host tests (`activity_life_test.go`) run the hooks as installed, in each
driver's order: the next activity starts not stopping and follows a light/dark
change made while no activity was live and one made in its foreground; the
refresh that stood down, and the fetch whose landing was dropped, run again
and land in the next activity; a translation load landing between activities
lands in the next one, with its spinner down and the next load accepted, and
a later recreation, after the reader has moved on, does not land it again
(the start lets go of what it lands); the old activity's stop run after the
new one's start marks nothing, while the new activity's own stop still marks;
a stop with no start after it still marks, and a refresh landing, a
translation landing and the refresh are all refused. A light/dark change is
refused there too, but on Android by the appearance gate's background flag,
which the destroy's own foreground exit closed before the stop; `stopping` is
what refuses one where no foreground exit comes first, and the tests prove it
in both such places: desktop, whose gate acts on every change (the close
intercept, focus moving, the window focused during its teardown, and a
light/dark change heard then), and an iOS termination that comes while the
app is still active, one event from Focused to Dead in which the driver's
Focused crossing returns before the foreground-exit hook once the Visible
crossing has dropped the GL context. Desktop and iOS (launch, background round
trips, termination) otherwise stop as they did. What only a device can
confirm is in `docs/VISUAL_TESTS.md` under V15 and V9.

## Android: an activity destroyed with the process alive stops the narration — FIXED 6 October 2026

Fixed for 1.2.20 (the end of this entry).

From the code, found while tracing the stop above; not run on a device.
`OnStopped` calls `nativeAudioStop()` on every activity destroy, not only when
the process is ending (`InstallReadingStateFlush`, `app.go`). On Android that
is `BtAudio.stop()`, which ends the listening session — both engines, audio
focus, the MediaSession and the foreground service — and, as its comment says,
posts no callback, because Go drove the stop. But the Go controller (`gAudio`,
`audio_controller.go`) never hears of it: it still holds the chapter as loaded
and playing, so the transport, the read-along wash and the follow pill carry
on as if the narration were live.

The destroys it runs for with the process alive:

- the system reclaiming the activity while the reader listens with the screen
  off, the case the foreground service exists for;
- Back finishing the root activity on Android 11 and older, where iOS keeps
  playing when the reader leaves the app.

A swipe-away already stops playback in the service's `onTaskRemoved`
(`BtAudioService.java`), which posts `ST_IDLE` so Go is told; the hook's stop
adds nothing there. A killed process takes the audio with it and runs no hook.
The raw stop exists for the desktop quit, where `OnStopped` can run off the
main thread and must not `fyne.Do` (`ARCHITECTURE.md`, Threading); on Android
the hook runs on the UI goroutine. Candidates: on Android, stop no audio in
`OnStopped` at all, or stop through `gAudio.stop()` so the controller follows.
A rotation is not among these destroys: on the Android 15 emulator it keeps
the activity and the process (see the entry above), so V9's rotation check
does not reach this path. Confirm first on a device: with Developer options >
"Don't keep activities" on, press Home mid-narration and wait (V9).

**Decided 6 October 2026** — Android narration when the system reclaims
the screen: don't stop the audio, matching iPhone. Of the two candidates,
the first: on Android, `OnStopped` stops no audio.

**Reproduced and FIXED 6 October 2026** (for 1.2.20). On the API 35
emulator, with "Don't keep activities" on, narration of WEB Matthew 6
playing and Home pressed: the activity was destroyed (`wm_destroy_activity
… always-finish`) in the same process, and within the same 60 ms the
narration's audio focus was abandoned, the foreground service stopped and
the media session destroyed. Back in the app, the new activity showed the
transport's pause button and the read-along wash on verse 1 over silence.
Turning the setting on with the app in the background did the same.

The fix (`activityStopEndsAudio`, `activity_life.go`; `app.go`): the stop
hook sends the engine's raw stop (`engineStop`, which is
`nativeAudioStop`) only where a stop ends the app, which is everywhere but
Android. On the same emulator and steps, the media session stayed PLAYING
and the player started through the destroy, and the new activity's
read-along went on from verse 3 to verse 4 with the pane following; its
Pause and Play drove the same session, and a swipe-away from recents still
ended it (`onTaskRemoved`). `TestAnAndroidActivityStopLeavesTheNarrationPlaying`
(`activity_life_test.go`) runs the hooks in the Android glue's order and
fails if a destroyed activity's stop, on time or late, sends the engine a
stop, or if the desktop's or iOS's stop does not send one. Back on Android 11
and older now leaves the narration playing as the iPhone does when the
reader leaves the app; it was not run, the emulators here being 13 and later.
Android 7.0 to 11 (API 24 to 30) are still supported, so the check is open,
on an emulator at API 30 or below — see docs/ROADMAP.md.

**Back on Android 11, PASSED 14 October 2026** (ROADMAP.md, Later minor
releases, row 42), on an Android 11 (API 30) emulator image, with a debug
build of the tree at `7a68743cb`. With the WEB's John 3 narrated from its
recording and the read-along on verse 4, one Back finished the root
activity and destroyed it (`wm_finish_activity`, `wm_destroy_activity …
finish-imm:idle`) in a process that lived on. Five and 35 seconds later, in
the same process and with no activity, the media session was PLAYING, the
player started, the app held audio focus and the audio service was in the
foreground. Opened again about 75 seconds after Back, the app came up warm
in the same process with the transport showing Pause and the read-along on
verse 19, then on 22 with the pane following. The control: a swipe from
Recents, the documented stop (`onTaskRemoved`), left no session, player,
focus or service in the same reading.

## Rework the download page — DONE 26 September 2026; Google Play filled its slot, the release notes followed, and the APK row returned, 30 September 2026

The public-surfaces check did not require the App Store and Snap Store
links in the release notes. FIXED 6 October 2026, for 1.2.20:
`scripts/check-public-surfaces.py` (rule 2d) holds the release notes'
printf to exactly the stores the download page links, in both directions
and listing for listing. A store is told by its host and its listing by the
part of the link that names it: the App Store's numeric id, Play's package,
the Microsoft Store's product id, the snap's name. Locale or name segments
and query strings do not count. So a store on the page and missing from
the notes fails, as does one in the notes the page lost, or one linked at
another listing. The page's badge and the printf's line now go in
together. Its self-test fails, as every case must, against the rule
switched off: the App Store and the Snap Store each missing from the notes,
linked there without a badge, and linked at another listing, and an App
Store link kept only in a comment beside the notes. The real tree with
either line taken out of the printf fails. A store on neither page is still
beyond it.

The page had store buttons by platform at the top and a separate "Desktop —
direct downloads" half with its own sections, three button styles, and the
macOS first-launch steps in a note above three platforms' downloads. It now
has two sections with one vocabulary each. "Get it from a store": the
stores' own badges (App Store, Microsoft Store, Snap Store), the licensed
form each store publishes for this, kept under `docs/badges/` so the page
loads nothing from elsewhere, each captioned with its devices, with a dashed
"Google Play" placeholder until the listing is live (the store's rules cover
the badge only as a link to a live listing; `docs/badges/google-play.png` is
ready for then). "Download a file instead", a card that opens on a tap, since
most readers want a store edition:
one row per file (Mac .zip, Android .apk, Windows .zip, Linux .tar.xz, Linux
AppImage) with a button per processor on the right, every row carrying the
same two lines, what it covers or how it runs and whether it opens shared
links. The macOS first-launch steps sit under the Mac row behind a disclosure
that needs no scripting, and a closing note says what shared links are and
what ARM means. The masthead, the feature list, the
open-source paragraph and the footer are unchanged.

Google Play went live on 30 September 2026 (production, 1.2.17, build 187),
and the placeholder became a link to the listing showing the Play badge,
captioned "Android phones and tablets" like the other three. The file is the
store's own, unaltered. It carries its clear space inside the image, a
transparent 41px around a 168px panel, so the page draws it larger by that
margin and takes the margin back out of the layout (`.badge img.padded`): its
panel stands 40px tall like the others, where the plain 40px image height
would have shrunk it to 27px, smaller than the badges beside it.

The Android .apk row left "Download a file instead" the same day, and the
card's line named Mac, Windows or Linux, to offer Android through Google Play
only: a sideloaded APK never updates itself, and it cannot update into the
Play build, which is signed with a different key, so a reader who started
from it could reach Play only by uninstalling it, which deletes their notes.
REVERSED 30 September 2026, in case it is useful to someone without Google
Play: the row is back in its old place, after Mac and before Windows, in its
old wording, with one line more, taken from the README's caution: "Never
updates itself; moving to Google Play later means uninstalling it, notes and
all". The card's line names Android again, and its lead says again that the
desktop files are the unsigned ones, since the APK is signed. The Play badge
stays first. Every GitHub release carries the APK (`docs/RELEASING.md`,
stage 8), uploaded by hand. `scripts/check-public-surfaces.py` no longer has
a `NOT_LINKED` exception: it holds the page to linking the APK like every
released file, read from the upload command commented in the release
workflow, counting only links a reader is shown, so a row inside a markup
comment offers nothing. It fails on a link to any release asset by a tagged
release's URL rather than `releases/latest/download/`, comment or not. Its
self-test proves both for every file its fixture page links, not the APK
alone: each gets a tagged link beside its latest one, and each has its only
link commented out. A tagged or dead link inside a comment still fails, since
it is one edit from being shown.
`scripts/release-status.py` expects the APK on the latest release because the
page links it. `scripts/publish-site.sh` copies `docs/badges/` into the site
and refuses a tree missing a badge the page shows. The checker still holds
every release asset, the Microsoft Store link and the macOS steps to the page.

The GitHub release notes, the `NOTES` printf in
`.github/workflows/release.yml`, linked the App Store for iOS but offered
Android readers only the APK, in a line addressed to whoever publishes the
release, with no word of Google Play, on every release page. FIXED 30 September
2026: the notes say "Android ships via Google Play" with the listing's link,
in the form of the App Store line above it, and keep the APK for devices
without Google Play in the README's terms: it is attached below, it never
updates itself, and moving to Google Play later means uninstalling it, which
deletes the reader's notes. They carry no single quote and no percent sign,
since the printf reads them as its format. `scripts/check-public-surfaces.py`
now requires a Play link in the notes' printf, on the download page and in
the README, reading only what a reader is shown there (the printf alone; a
page less its markup comments), and fails on a Play link to any package but
the `appID` in `config/product.json`, or when that id cannot be read. Its
self-test takes each of the three places without the link, with a listing
for another app whose id begins with the product's, and with the link only
in a comment, and passes a link with parameters after the id.

The same day the release sequence gained the step this drift slipped
through: a channel that goes live for the first time is drafted into the
notes' printf as well as linked from the page and the README
(`docs/RELEASING.md` stage 11, `docs/APP_STORE_SUBMISSION.md` step 9), and
`CONTRIBUTING.md` describes the checker's Play rule beside its Microsoft Store
one.

Simplified 30 September 2026. The notes now open with the four stores, each
a line (App Store for iPhone, iPad and Mac; Google Play; the Microsoft Store;
the Snap Store), then say the attached files are for use without a store and
updated by hand, then give the APK caution, the Mac file's first-launch steps
and the Windows files' one. The 1.2.13 Linux rename note is gone. The same text
was put on the published v1.2.17 release with `gh release edit` the same day,
at the account holder's word, so the latest release no longer offers Android
readers only the APK. The checker now also holds a Microsoft Store link in the
notes' printf to `msstore/identity.json`.

Still open: the checker does not require the App Store or Snap Store links in
the notes, so a store line could be dropped unnoticed; requiring exactly the
set of stores the download page links would also catch the next channel to go
live. The notes still say nothing of what changed in a release; each store's
own What's New carries that.

Whether a Play-signed install verifies its App Links is answered: on
30 September 2026 a Pixel with 1.2.17 from Google Play reported
`bibletext.co.uk: verified`, and a bibletext.co.uk/web/ link opened in the app
(`docs/LINKS.md`, the matrix and section 6).

## Reword the Linux note on the download page — DONE 26 September 2026

The paragraph above the Linux direct downloads on the download page
(`docs/index.html`) read awkwardly: it recommended the Snap, then said "Of these
two, the .tar.xz also installs a desktop entry, so those links work once you run
make install", where "these two" meant two formats though four buttons
followed, and it ended with the AppImage not registering "them", some way from
the shared links that word stood for.

What shipped: the paragraph is gone, and each direct download is one row per
format, with a button per processor on its right (Intel / AMD, ARM), so the
reader picks the format and then their machine. Each row says in two short
lines how to run it and whether it opens shared links: the .tar.xz does once
`make install` has run, the AppImage and the Windows .zip do not, and the macOS
button says the same of its .zip (`docs/LINKS.md` has the table). Windows got the same row, so its two buttons
became one. The general note above them still says the store editions and the
Snap are the easier routes. The README's AppImage note already agreed.

## Say which model is working on the AI waiting screen — DONE 27 September 2026

While a Study with AI request runs, the panel's waiting state (`setThinking`
in `ai_panel.go`) shows "Reading the passage…", an endless progress bar,
"Capable models can take a minute or more." and Cancel, with the faster-model
offer under it when there is one (`ai_faster_model.go`). It never said which
model was working. The wait depends on exactly that, and a reader who changed
the model in Settings, or took the faster-model offer, could not see from the
screen what they were waiting on.

What shipped: every AI wait names the provider and model at work, in one
quiet line under the bar and over the "Capable models…" hint, in that hint's
style (caption size, muted, centred) and at its measure: 260pt wide, and one
line as tall as the hint's box makes a line (`captionHeightFor`), not the
taller padded row a RichText measures for itself. The waits are
the Study with AI panel, which every study verb opens (`showAIPanel`); Find on
the Search tab, which all five platforms draw from the shared layout
(`findSearchingView` in `ui_compact.go`), and in the former sidebar layout
(`aiSearchingView` in `search.go`, `BIBLETEXT_DESKTOP_TABS=sidebar`); and
Settings' Test key, whose "Testing…" gains the same line under it, muted like
the model caption above. The wording is the Settings pickers' own: the
provider as the assistant picker names it, then the model as the model
picker lists it, which is by its id: "Claude (Anthropic) · claude-opus-5".
Before the request has said, the line is empty but holds its height, so the
report moves nothing. The cross-references panel's "Finding related
passages…" is the Treasury of Scripture Knowledge dataset, not AI, and is
unchanged.

The line comes from the request, not from `activeModelFor`.
`modelResolver.generate` reports each model it is about to send to (the
reader's override, a self-healed pick or the shipped default, and on a
model-not-found error the replacement it discovered, before the retry) to an
observer the screen puts on the request's context (`ai_model_in_use.go`). The
observer hops to the UI goroutine with `fyne.Do`, and each screen drops a
report from a request it is no longer waiting on: the panel by its fetch
generation and only while its bar runs; Find by its session token, holding
the model on state (`aiSearchModel`) so a tab rebuilt mid-Find names it
again; Test key by a sequence number, which now also keeps an older test's
verdict off a newer test's wait. The faster-model offer starts a new request,
whose waiting state draws its line empty and fills it from that request's own
report, so the line follows the switch.

A long model id wraps inside the line's measure rather than widening the
column, so no sheet or card grows past a 375pt phone, and every wait lays its
column out again the moment a name wraps. On the Study panel that is a
re-fit through `fitBody`, so the panel grows to show the column where the
screen has room, and on a landscape phone the column stays in its scroll with
Cancel in reach.

Both Find waits now scroll as the Study panel's waiting column does
(`findWaitScroll` in `search.go`). An iPhone kept its bottom bar in
landscape, which leaves the Search tab's results area, between the Find
field and the bar, less than half the wait's height (in the host layout at
667×375, 128pt of about 297pt). The wait's column was centred there and ran
out of it at both ends: "Searching with AI…" and the bar under the Find
field, Cancel and the faster-model offer under the tab bar, where nothing
could bring them back; the model line pushed Cancel almost wholly under. The
column now starts at the top of the area with the rest a scroll away, and
wherever there is room the scroll never engages and the column centres as
before. Since 1.2.18 a phone held sideways has the rail, which leaves more
room than the bar did but still not enough at any iPhone's or an Android
phone's sideways size, so the scroll still holds Cancel in reach. The
desktop wait (`aiSearchingView`) scrolls the same way in a short window.

Asking Find again while a Find was in flight left the new Find searching
until Cancel, on both Find surfaces. The resubmit takes its session token,
then runs its predecessor's teardown hook (`installAISearchCancel`), and that
hook invalidated whatever submission was latest, the new one included, so the
new Find's answer and its model report were both dropped as stale. The hook
now gives up only its own submission (`aiSearchSession.Abandon`).

The host tests are in `ai_model_in_use_test.go`, with the session case in
`search_race_test.go`; what only a screen can confirm is in
`docs/VISUAL_TESTS.md` under V10.

## Show the selected words on the Add a note sheet — DONE 27 September 2026

The share-with-note sheet (`promptShareNote` in `share_note_ui.go`) showed the
title "Add a note", the passage's reference and the note field, but not the
words the reader selected, and the reading text is hidden while the sheet is
open (`hideReadingOverlay`), so writing about a phrase meant remembering it. The
selected words now sit under the reference and above the field, small and
muted: a reminder of what the note is about, not a second reading view.

The words are the share's own. `shareNoteQuote` runs `prepareShareQuote` once
and gives the sheet both the excerpt and the reference, from the same call the
share makes for its citation, so the three cannot disagree: the verse numbers
the selection carries are gone, a word the drag cut in half is repaired as the
share repairs it, the divine name keeps the small capitals the page draws it
in, and a publisher's heading never shows, because it never reaches the quote.
A heading selected on its own quotes nothing, so the sheet shows the reference
to the verse beneath it and no excerpt. The words are the quote before its
Bluebook framing, without the quotation marks, bracketed capital or omission
dots a quotation standing on its own takes.

Two selections still carried the page's apparatus into the quote, and so onto
the sheet, and both are fixed in the share pipeline (`share.go`) so the share
and the excerpt keep agreeing. A heading at the top of a chapter, selected on
its own, was quoted as if it were scripture: the heading-only rule
(`headingOnlySelectionVerse`) asked for a span naming the verse above the
heading, and a chapter's opening heading has none. The native panes report a
drag inside it as no span at all, or as the first verse once the drag reaches
that verse's number, and the rule now takes either for a heading with no verse
above it; a span naming a verse further down still leaves the selection where
it puts it. Most chapters of the licensed editions open with a heading, so this
was the common case. The rule also takes a heading followed by the number of
the verse it stands above, the word a drag that runs one word past the heading
stops at, which had quoted the heading and the number together and cited the
verse above as well. And a verse number selected on its own (a double-tap or a
long-press on the superscript) was quoted as the digits:
`normalizeShareSelectionIn` declined it and the legacy path passed it through.
It now quotes nothing and names the verse the number labels, with the span the
pane reports and without one, so the text share, the link, the note's anchor
and the cross-references all name that verse. Pinned by
`TestAHeadingAtTheTopOfAChapterSelectedOnItsOwnQuotesNothing` and
`TestAVerseNumberSelectedOnItsOwnQuotesNothing`
(`share_heading_selection_test.go`), and on the sheet by
`TestAHeadingNeverReachesTheNoteExcerpt` and
`TestTheNoteSheetShowsTheSharesQuote`. A heading in the middle of a chapter
selected with no span at all would still be quoted; no pane produces that
shape, since every native pane reports a drag inside one against the verse
above it and the styled pane cannot select a heading. A text share of a
selection that quotes nothing, heading or number, sends a blank line and then
the citation, as a heading selected on its own already did.

The excerpt (`noteExcerpt`, `share_note_excerpt.go`) is set in the reading
face, at a 14pt reference size through `readingGlyphSize`, in the muted ink,
because the chrome face has no small-capital glyphs; its rows are
`canvas.Text`, as in the verse-of-the-day card, since that is the toolkit text
that takes a font source. It wraps and stops after three rows, the last cut
with an ellipsis: at about forty-five characters a row on a phone, three rows
hold a phrase and most of an average verse. On a phone in landscape (a canvas
under 480pt tall, on iOS and Android alike) it takes one row, because the
phone sheet is not resized for the keyboard and every row pushes the field a
row further under it: on a 393pt canvas, under a keyboard about 200pt tall,
one row leaves three lines of the note above the keyboard and three rows leave
one. Desktops keep three rows at any window height, a window as short as a
phone in landscape included, since no soft keyboard covers their sheet. Its
minimum width is zero, like the counter's, so it never widens the card past the
canvas, and a word wider than a row is broken between letters.

On iOS the note field is a native text view parked over the sheet's slot
(`noteEntrySlot`), which pushes its absolute rect on every Move and Resize. The
excerpt sits above the slot in the form, so the slot moves down by exactly the
excerpt's height and the field follows it. The popup lays the form out as it
is resized and again as it is shown, so the first frame the field is given is
already below the last row; the relayout the phone canvas makes when the
keyboard's inset changes pushes the same frame again. The excerpt's height is
that of its rows at the width it was last laid out at, which on a canvas 320pt
wide (iPad Slide Over, a third of an 11-inch iPad in Split View) is narrower
than the width an unsized excerpt wraps for; a selection that takes a row more
there than at that width is what shows the field below the last row rather
than over it. No other platform floats a native field: Android and the
desktops draw the Fyne entry in the same place. The excerpt comes back with the
composer after a light/dark change, in the new palette, since it is built from
the selection on every open, the reopen's included.

The phone sheet also refits when the canvas changes size, which it did not
before this item either. The toolkit never resizes a popup for a new canvas;
it only lays it out again at the size it was given. Narrowing an iPad's Split
View or Slide Over window, or an Android window, with the composer open changes
neither the layout class nor the rail, so no rebuild drains the sheet, and the
card, the excerpt and the slot kept their old width, with the native field
parked past the canvas's right edge and Share beyond it. The composer's
watchdog now notices the new canvas size, as the Go to picker's does, and sizes
the card for it: the excerpt re-wraps, it takes the row budget a sheet opened
on that canvas would take, and the field follows the slot. The card keeps the
top and the gap at the canvas's foot it opened with, and never reads its height
from the interactive area again, which shrinks while the keyboard is up: fitted
to the keyboard's top, the card would stay short once the keyboard went down.
The one open that reads the area while the keyboard is up, the light/dark
reopen, reads it through `sheetArea` (`sheet_fit.go`), which gives the
keyboard back (below, "A phone sheet reopened under the keyboard").
The native field is told only where the slot settles (`noteEntrySlot.settle`),
not the rects the relayout passes through on the way, since the popup lays its
content out before it moves it and each push reaches the view on the main
queue, which may draw between two of them. Until the watchdog's next pass,
within 150ms, the sheet stands at its old size, as the Go to picker does. The
Ask sheet (`promptAskQuestion`, `ai_ask.go`) is the same full-canvas phone
sheet and still has no refit: narrowed with it open, its card and Ask stay at
the old width. It has no native field, so it needs only the resize, not the
settle.

The desktop card also stood a row taller than its content before this: its
height was read from the layout Show makes at the form's narrow minimum width,
where the counter wraps into more rows than at the card's own width. It is now
sized in two passes, the second taking the height the card's width gives, which
the excerpt needs as well.

The composer's platform choices are seams (`noteEntryNative`,
`noteEntryFrameTo`, `noteSheetAfter`, `noteSheetArea`) so the host tests can
lay out the sheet iOS gets: they make the field native, read the frames it
would be given, hold the slot's second push and the phone sheet's watchdog to
run on their own goroutine (see "Deferred-UI timers under the test driver"),
and give the canvas a device's safe insets and a raised keyboard. Pinned by
`TestTheNoteSheetShowsTheSharesQuote`, `TestTheNoteSheetCutsALongSelection`,
`TestNoteExcerptLines`, `TestTheNoteSheetIsNoWiderThanTheCanvas`,
`TestTheNoteFieldSlotSitsBelowTheExcerpt`,
`TestTheNoteExcerptComesBackWithTheSheet`,
`TestAHeadingNeverReachesTheNoteExcerpt`,
`TestTheDesktopNoteCardIsAsTallAsItsContent` and
`TestTheNoteSheetRefitsWhenTheCanvasChanges` (`share_note_excerpt_test.go`),
each proved against a mutation. What only a screen can confirm is listed under
V12 in `docs/VISUAL_TESTS.md`.

## The desktop takes the phones' navigation rule — planned for a minor release after 1.2.22

Planned for: a minor release after 1.2.22, as its headline; which one is an
open decision — see docs/ROADMAP.md. It was to lead 1.2.21, which went out
as the privacy-declaration and Android 7.0 release instead.

Since 1.2.18 every phone and tablet puts Read, Books and Search on a rail
while its window is wider than it is tall and on a bottom bar otherwise
(`mobileRailWanted` in `layout.go`; the entry below). The desktop does not:
`compactNavRail` in `ui_compact_desktop.go` answers the rail whatever the
window's shape, so a window tiled to a phone's shape keeps a rail down its
narrow side, and the desktop never installs the layout watcher, so nothing
would notice a resize across square anyway (divergence 29 in
`docs/PLATFORM_MATRIX.md`). One rule everywhere is simpler to explain and to
hold, and the tiled phone-shaped window is exactly where the bar serves.

The plan:

- **The rule.** The desktop's `compactNavRail` asks `railForWindow`, as the
  phones do, when `BIBLETEXT_DESKTOP_TABS` is unset or unrecognised. The
  variable stays an override, so a gallery or a comparison can still pin
  either: `sidebar`, `bar`, and `rail`, which exists today and keeps its
  effect, the rail whatever the shape. What changes is that `rail` stops
  meaning the default. `desktopNav` folds unset, `rail` and an unrecognised
  value into one answer, `desktopNavRail`, and its doc comment documents
  `rail` as "the rail, stated explicitly (same as unset)". `desktopNav`
  needs a fourth answer for the shape rule, and its comment and
  `announceDesktopNav`'s names change with it. That comment warns that the
  variable's two readers, whether the compact layout runs at all and how
  it draws its navigation, once disagreed over `rail`;
  `desktop_nav_preview_test.go` pins their agreement, and its table (the
  rail for unset, `rail` and `nonsense`) and
  `TestDesktopDefaultsToTheRailWithNoEnvironment` are rewritten for the new
  default while `TestRailImpliesTheCompactLayoutIsOn` keeps holding for
  every value. An unsized desktop canvas must answer the rail, the
  desktop's first frame today; `railForWindow` as it stands gives any
  unsized window that is not a tablet the bar. The tests that set the
  variable to `""` for the shipped desktop navigation
  (`fullscreen_layout_test.go`) then get the shape rule and are rechecked;
  those that set `rail` or `bar` keep what they force.
- **The watcher.** `layoutWatcher` moves out of the mobile-only
  `ui_regular.go` into an untagged file, and `CreateMainUI` in
  `ui_desktop.go` wraps its root in it, so a resize that changes the answer
  rebuilds the window.
- **What a rebuild keeps.** A desktop window rebuilds today for a tab
  change, a light/dark switch and the like; one in the middle of a resize is
  new. It must keep the reading place, an open sheet and a live selection,
  on the macOS native pane (`reading_macos.go`: its scroll restore and the
  note card) and on the shared Windows/Linux styled pane, and it must not
  re-push the chapter unless the page itself changes.
- **No flapping.** A dead zone around square, so a window a few points
  either side of it does not change navigation (for example the rail from
  10% wider than tall, the bar from 10% taller, and the layout as built in
  between), and a switch only when a live resize settles (one decision some
  250 ms after the last size change), so dragging a corner across square
  does not rebuild on every frame. The phones keep the plain rule: their
  rotations are discrete.
- **Tests.** The dead zone and the settle as pure functions over a fake
  clock; the watcher installed on the desktop; the place, the sheet and the
  selection kept across a resize rebuild on the styled pane, and the macOS
  pane by its desktop test recipe; the desktop rows of the Navigation table
  in `docs/PLATFORM_MATRIX.md` brought to the rule and divergence 29
  retired.
- **A pass on each desktop.** The Mac, the Windows VM and the Linux VM:
  drag a window across square both ways mid-chapter, with a sheet open and
  a selection live, and tile a window to a phone's shape.

## One navigation rule on phones and tablets — DONE 1 October 2026, for 1.2.18

Still open: a sideways launch draws the bar first (the follow-up below) —
fixed for 1.2.23 on its own branch for every phone and tablet, merged into
`rel/1.2.23` on 15 October 2026, and seen fixed on the simulators and
emulators; a launch on hardware is still to be seen; see docs/ROADMAP.md,
1.2.23, row 1.

Read, Books and Search sit on a rail along the leading edge while the window
is wider than it is tall, and on a bottom bar otherwise, on every phone and
tablet, iOS and Android alike (`mobileRailWanted` in `layout.go`; a square
window counts as wide, `w >= h`, as before). The rule used to carry an
exception: tablets on both platforms and Android phones took the rail when
wider than tall, and the iPhone kept the bar in every orientation
(`phoneLandscapeNavRail`, false on iOS). The iPhone's Read tab reads
full-screen sideways, so the exception showed on Books and Search, where the
bar was 874 or 956 points wide, wider than `tabBarSpreadMaxWidth`, and wore
the iPad's centred dress with its tabs bunched in the middle (the entry
below).

What changed:

- **The rule.** `mobileRailWanted(tablet, w, h)` is `w >= h`. Before the
  canvas has a size it answers by the device, as it did: the rail for an
  iPad, the bar for an iPhone and for any Android window, whose idiom is its
  size, so an Android phone never flashes a rail before its size arrives.
  `railForWindow` asks it of the live canvas, which the soft keyboard does
  not change, and the phones' `compactNavRail` is that call.
  `phoneLandscapeNavRail` and its three platform definitions are gone.
- **The watcher on every phone and tablet.** `layoutMayChange` is gone too.
  It was true on Android and on an iPad, and on an iPhone only while the
  landscape presentation's preference was on (a development switch); with
  it off an iPhone would have kept across a rotation the navigation it was
  built with.
- **The side insets need nothing of their own.** Both mobile drivers lay the
  window's tree inside the insets they report (UIKit's safe area; on Android
  the system window insets Fyne reads), so the rail stands beside the
  Dynamic Island and the content beside the rail keeps clear of the inset
  opposite, as Android's rail always has beside whatever side inset its
  canvas reports. The native reading panes add the same insets back to their
  frames. The two sheets that span the canvas need `clearOfSideInsets` only
  because a sheet is placed on the canvas, outside that tree; the same
  padding on the rail would hold it 62 points further in on an iPhone 17 Pro
  Max.
- **The centred bar** is now drawn only by a window taller than wide and
  wider than 560 points: an iPad or an Android tablet held upright, or a
  tall tablet window that wide. No phone draws it.

Only the iPhone changes on screen. Laid out on the host with a phone's
insets, every object in the window was compared with the tree before the
change on seven devices (three iPhones, an Android phone, two iPads and an
Android tablet), upright and sideways, on Read, Books and Search: 36 of the
42 layouts are identical object for object, every upright one among them,
and the six that differ are the three iPhones' Books and Search held
sideways, where the bar became the rail. On the iOS 26.5 simulator a
development build on an iPhone 17 Pro Max drew the rail on Books and Search
in all 8 sideways captures, turned each way and launched each way, between
the safe area's edge 62 points in and its hairline at 122, and the bar at
the foot in all 10 upright ones.

Tests: `TestOneNavigationRuleOnEveryPhoneAndTablet` (the rule on every device
both ways up, in iPad and Android windows, square and unsized; on the rule
before it the three iPhones held sideways fail and nothing else),
`TestRailForWindowReadsTheCanvas` (on the rule before it, 956x440 and
641x641 fail), `TestTheRailKeepsClearOfTheSideInsets` (an iPhone 17 Pro Max
and an iPhone 16 Pro held sideways with UIKit's insets, the island's side
alone inset on the left and on the right, an Android phone's cutout and its
three-button bar; laid across the whole canvas the check finds the tree
under the inset, and with the rail given the sheets' padding every case
with a left inset fails) and `TestTheCentredBarIsAnUprightTablets` (on the
rule before it the three iPhones held sideways draw the centred bar). The
App Review notes (`appstore/review-notes.txt`), whose navigation paragraph
carries from release to release, now say the same, and
`TestAppReviewNotesPlaceTheSidewaysIPhoneNavigationByTheRule` holds them to
the rule: the paragraph before it, which kept the bar on a sideways iPhone,
fails, and so do notes that never name the rail.

Left as it is: with the landscape presentation switched off, an iPhone's
Read tab held sideways now shows the rail beside the reading pane and
rebuilds on rotation as an iPad's does, with no Go-side anchor, which
`captureRotationAnchor` takes only when the presentation flips.

**Follow-up, found 1 October 2026: a sideways launch draws the bar first.**
An iPhone launched while already held sideways draws the bottom bar for
about 0.4 s before what its shape calls for (seen on the iOS simulator
while the rule was built): the rail on Books and Search, and on the Read
tab the full-screen page, not a rail. The first layout runs before the
canvas has a size. `mobileRailWanted` answers an unsized canvas by the
device (in 1.2.22's tree, `layout.go:103-113`, reached through
`railForWindow`, lines 122-128): the rail for an iPad, the bar for an
iPhone. On the Read tab `readingFullScreen` (`phone_landscape.go:90-92`
there) is false until then too, since `phoneLandscapeReadingWanted` needs a
sized canvas (`w > 0 && h > 0`, line 58), so the Read tab gets the bar as
well. When
the size arrives the watcher finds the rail, or the full-screen page,
wanted and rebuilds. Minor, and at launch only. The fix: answer the first
layout's two decisions, rail and full-screen page, from the size or
orientation the window scene reports at launch, or hold the first layout
until the canvas has a size; either way an Android phone must still not
flash a rail before its size arrives, which is why the unsized answer is
the bar.

Fixed for 1.2.23 on 15 October 2026, on `fu/1223-ios-sideways-launch`, by
the first of those routes, on every phone and tablet. The same flash had two
more forms: an iPad opened upright drew the rail for a few frames before the
bar, and an Android phone opened sideways drew the bar first, or pushed the
phone page and then imported the chapter again as the book page.

- **The size the window reports.** Before the canvas has a size the layout
  decides by the size the platform reports for the window, as it would by
  the canvas's (`layoutSize`, `launch_window.go`): the rail, the phone's
  landscape presentation, the bar's dress and, on Android, the tablet
  idiom. The rule stays the canvas's own, `w >= h`, in Go; the platforms
  report sizes, not orientations. iOS reports its window scene's size and
  the left and right safe-area insets of the app's window
  (`bibleTextWindowReport`, `device_ios.go`), the window's own size in Split
  View and Stage Manager too, so it serves the iPad as it does the iPhone.
  Android reports its activity's configured size in dp over the bridge
  (`windowWidthDp`, and `windowHeightDp`, new). The desktop reports nothing.
  With no report the device answers as before. A build asks for the report
  once, at its first question, and decides by that answer throughout
  (`heldLaunchSize`): a build decides its tree at its start and the layout
  watcher records the decision at its end, and Android's configuration,
  read live, could otherwise change between the two in a rotation, leaving
  the watcher with a layout the tree does not have, which no later resize
  would see as a change.
- **The first chapter push** takes the same report: iOS gives the scene's
  width less the side insets as the cold width (`launchPaneWidth`,
  `readingColdWidth`), as Android gives its configured width, so a phone
  held sideways opens on the page its full-screen pane reads.
- **The iOS read never holds up the layout.** UIKit's scene belongs to the
  main thread, which is busy with UIKit's own launch for the first few
  hundred milliseconds. Read with a `dispatch_sync` from the Fyne goroutine
  whenever the layout asks while the canvas is unsized (36 to 43 times a
  launch), the first read waits about 400 ms and holds back every launch's
  first build: measured that way, upright launches reached their first
  paint about 45 ms (Read) and 85 ms (Books) later than main's. Rejected for
  that. The scene is read once instead, on a goroutine of its own
  (`keptWindowReport`), and the result is handed to the Fyne goroutine,
  which keeps it between two of its tasks (`keepWindowReport`) so that it
  cannot change in the middle of a build: a build decides its tree at its
  start and the layout watcher records the decision at its end. Keeping it
  asks the watcher again (`relayoutUnsized`, `ui_regular.go`), and a tree
  built before it from the device's answer is rebuilt then, before the
  first paint. An upright iPhone's report agrees with its device answer and
  rebuilds nothing.
- **A single-pixel canvas is not a size.** Android's first surface can be
  one pixel, a third of a unit each way on the emulator: square, so it read
  as wide, and its width chose the phone page. A canvas a unit or less on a
  side now counts as unsized (`canvasSized`), for the layout and for the
  window width the page is chosen from (`widestWindowWidth`).

The simulator and emulator pass, 15 October 2026, with a launch probe in
scratch builds of the branch and of main (none of it committed), on the
iOS 26.5 simulators and the API 35 emulator. Two builds of the branch ran:
the final one, and one without the one-answer-per-build hold, which changes
nothing on iOS and on Android only a rotation in the middle of a build. A
sideways iOS launch comes from a copy of the bundle that declares landscape
only: an iPhone turned in the app and relaunched comes back upright. Bar
and rail frames are counted in screen recordings at 30 frames a second.

| Case | Branch | Main |
|---|---|---|
| iPhone 17 Pro sideways: Read, Books, Extra large, presentation off | no bar frame in 30 of 30 launches; the tree built before the read lands is rebuilt before the first paint, and the first paint rebuilds nothing; the first push after the read is at 750 points, the book page (the phone page at Extra large), with no second import | Read: the bar for 7 frames in 5 of 5, then a rebuild and a second import |
| iPhone SE sideways, Normal and Large | no bar frame in 13 of 13; first push at 667 points (book page at Normal, phone page at Large), no second import | the device pass's 3 to 8 bar frames |
| iPhone 17 Pro upright, Read and Books, launches interleaved with main | first paint 704 ms median (Read) and 814 ms (Books), 15 each; one read a launch, landing 545 to 594 ms after process start; no extra build | 716 ms and 820 ms |
| iPad Pro 11-inch (M5) upright, Read and Books | no rail frame in 18 of 18; the bar, centred, before the first paint | the rail for 5 to 14 frames in 10 of 10, then the bar |
| iPad Pro 11-inch (M5) sideways | the rail, unchanged, in 9 of 9 | the rail |
| Android phone (`bibletext_test`) sideways | 35 launches; in the 6 whose first build had no canvas size it was the full-screen page from the configured 914 by 411 dp, with the book page pushed at 914 dp; no rebuild at the first paint and no second import in any | 13 launches; 6 whose first build had no canvas size: the bar first in 4, the phone page pushed at a third of a unit and imported again in 2 |
| Android phone upright | 10 of 10 the bar from the first build (one unsized, by its configuration) | 5 of 5 |
| Android tablet (`bibletext_tablet`) upright | 8 of 8 the centred bar from the first build, 2 of them unsized and answered a tablet by their configuration | one launch of 3 built unsized as a phone, with the spread bar, and rebuilt at the first paint |
| Android tablet sideways | the rail in 6 of 6, one of them unsized | 3 of 3 |

Left: on Android a canvas unit is a dp only approximately, because Fyne
rounds the screen's density to a few scales (a unit is a dp at 160, 240,
320, 480 and 640 dpi, and at 420 dpi it is 8/7 of one), so a window whose
short side falls between 600 dp and 600 units, or an upright one whose
width falls between 560 dp and 560 units, is answered one way by its
configuration and the other by its canvas: the tablet idiom or the bar's
dress changes at the first paint there. Removing it means either judging
those two lines in dp once the window has a size too, which moves where
some tablets and split-screen windows land today, or converting dp to
canvas units with a copy of Fyne's density table; both are open. Not seen:
the iOS 15 to 25 branch of the read (the scene's own `coordinateSpace`,
deprecated from iOS 26), since only iOS 26.5 and 27.0 simulators are
installed, and a launch of the shipping app held sideways on hardware.

Tests: `TestASidewaysLaunchLaysOutSidewaysFromTheFirstFrame` lays out
iPhones opened sideways on Books, Read and Search, and upright, first with
no size and the reported window and then at their size, with a control
that shows the rebuild the comparison catches;
`TestAnUprightIPadLaunchDrawsTheBarFromTheFirstFrame`,
`TestTheBarsDressTakesTheReportedWidth` and
`TestRailForAnUnsizedWindowTakesTheReportedSize` hold the iPad, the dress
and the rail; `TestASinglePixelCanvasIsNotASize` holds Android's first
surface, for the layout and the page; `TestTheReportedPaneWidthIsTheWindowLessItsSideInsets`
the cold width; `TestTheLayoutNeverWaitsForTheWindowRead` that the layout's
question never waits for the read, that a read made before the scene is
connected is made again, that only the Fyne goroutine keeps the report, and
that keeping it asks the layout again once;
`TestTheReportsLandingAsksTheLayoutAgain` the hand-off itself; and
`TestAReportedSizeIsHeldThroughABuild` one answer through a build. Each was
shown to fail with its part of the fix taken back out (`go test -overlay`):
the report ignored, any positive size counted as a size, the read made where
the layout asks, the report kept by the reading goroutine, the hand-off
dropped, the side insets kept, the single pixel counted as a width, the
dress read without the report, nothing held through a build, and a newly
kept report leaving the old answer held. `TestTheLayoutNeverWaitsForTheWindowScene`
and `TestAndroidReportsItsConfiguredWindow` hold the Objective-C, the
bridge and the Go that consults them; they fail on a copy with any one of
fourteen changes (the screen's bounds for the scene's, the right inset
dropped, no wait for the app's window, the read made off the main queue, a
read made where the layout asks, the dress without the report, a relayout
of a sized canvas, the landing handed to nothing, Android's width for its
height, its tablet idiom without the report, the bridge reading the width,
a build keeping the last build's answer or releasing it only after its
first decision, a kept report leaving the old answer held) and pass on the
copy unchanged.

## Phone and tablet sheets over the header — BY DESIGN, not a bug (1 October 2026)

A sheet on a phone or a tablet is a pop-up: it opens over what is under it,
and when it is tall enough it covers part of the header, its top edge
sometimes crossing the Go to chip or the title. That is how a pop-up is
meant to behave, and it stays.

A rule that moved such sheets clear of the header, or grew them to cover
it whole, was built on 30 September 2026 and taken out again on 1 October
before any release carried it. On the iOS simulator it cost content that
1.2.17 shows: on an iPhone SE held upright the Go to picker hid chapter 28
of Matthew and clipped 25 to 27, the waiting AI answer pushed Cancel and
"Switch to a faster model" out of view, and the translation list showed
three translations instead of four; cards grown to cover the header carried
blank space, and the audio menu, opened sideways, ended across the tab bar.
The desktop's own rule, that a sheet opens below a desktop window's header
(`headerClearance` in `sheet_fit.go`), predates it and is unchanged.

Two older layout faults the same simulator pass showed, both in 1.2.17:

- On an iPhone 16 Pro launched sideways, the tab bar looked laid out too
  narrow, its items spaced as if at the upright width. Measured on
  1 October 2026 the bar was not too narrow: on Books and Search held
  sideways, 60 captures of 60 on an iPhone 16 Pro and an iPhone 17 Pro Max,
  launched sideways and turned both ways, drew a bar 874 or 956 points wide,
  wider than `tabBarSpreadMaxWidth`, in the iPad's centred dress: its tabs
  bunched in the middle at their designed slots, which is what read as the
  upright spacing. Superseded in 1.2.18: a phone held sideways has the
  rail, not a bar ("One navigation rule on phones and tablets" above).
- Sideways on an iPhone with a Dynamic Island, the note composer and Ask
  ran under the island, which hid the start of a line of their text and the
  left end of the text box. Fixed: both sheets still span the canvas, so a
  tap beside the card closes nothing, and their cards keep clear of the
  side safe insets (`clearOfSideInsets` in `sheet_fit.go`, pinned by
  `sheet_side_insets_test.go`). The rule reads the side insets the canvas
  reports, so an Android phone held sideways with its cutout or a
  three-button navigation bar at one side gets it too; where no side is
  inset the sheets are where they were. Their foot was already the safe
  area's: UIKit reports a 20-point inset under the home indicator of an
  iPhone 17 Pro Max held sideways, and the sheets end there.

## The open narration card covers the phone header's controls — kept as a pop-up for now

Planned for: next major, the bar on phones behind `next`; the double tap on
the ✕ in 1.2.23+ — see docs/ROADMAP.md.

On a phone the open narration card lies over the next-chapter arrow and the
full-screen button, and with a long heading or on the narrowest phones over the
copy icon and the end of the heading too. The phone header
(`chapter_header_mobile.go`) centres the audio control on the gap between the
chapter block and the full-screen button, in a cell reserved at the open card's
size so opening it never moves anything, and on a phone that gap is narrower
than the card's 161 units. On a 375-wide canvas (an iPhone SE) the card covers
the copy icon, the next arrow and the full-screen button in Deuteronomy 34; at
402 it still covers the full-screen button in five of the six books the tests
lay out; and in Song of Solomon it covers something up to a canvas 502 wide.
The desktop toolbar's card does the same to the chapter block on a reading
pane narrower than about 570 (Song of Solomon again), and so does the Android
fallback pane's, which uses that toolbar on a phone. A small control whose
glyph the card touches is hidden while it is open (below): on the phone header
that is the full-screen button in the last chapter of 69 of the 73 books on a
402-wide canvas, and something in Song of Solomon up to a canvas 487 wide; on
the toolbar, something in Song of Solomon up to a canvas 557 wide.

It is kept, for now, as a pop-up: the card's ✕ collapses it back to the speaker
while narration carries on, and what it covered is back. Since 26 September 2026
it covers properly. Until then the covered arrow and full-screen glyph were
drawn over the card's skip button and corner, and a tap on the card's
background fell through to them: the header's Border drew its centre first, and
the card's frame is a plain rectangle no driver hands a tap to. Now the phone
row draws the audio control above its neighbours while the card is open (and
beneath them, as before, while it is the speaker, whose box on the narrow
phones reaches under the copy icon and the full-screen button), and the card
swallows every tap and touch across its rectangle (`tapShield`,
`audio_button.go`).

Drawn above them, the card still covered a control only where it lay over it,
and its edge seldom falls between two controls: on a 402-wide iPhone in
Matthew 27 the → arrow's tail showed to the left of the card and a bracket of
the full-screen glyph to its right, and on a 360-wide Android phone in Psalm 23
half the copy icon showed to its left. A fragment of an icon beside the card
is not covered. So each small control whose glyph the card touches — the copy
icon, the chapter arrows, and on the phone header the full-screen button — is
now hidden altogether while the card is open: not drawn, not tappable anywhere
on its box, and passed over by Tab (it gives up the focus if it held it). It
is back when the card closes. "Touches" is decided by the glyph, not the tap
box, which is about twice as wide, so a control whose box the card only
grazes stays whole beside it — save for the full-screen button's focus
highlight, which fills its whole box while it has the keyboard focus: a card
over the highlight hides the button too, until the card closes. The heading
and the chapter line name the chapter and are only covered. Each hideable
control sits in a slot that keeps its size shown or hidden, so hiding one
moves nothing, and what the card hides is worked out again when it opens or
closes, after every layout of the header row, a rotation's included, and when
the full-screen button gains or loses the focus (`cardCover`,
`audio_button.go`; `chapter_header_hide_test.go` holds both headers to it for
the first and last chapters of every book, at every tenth width from 300 to
1020, at 1024, and at 375, 393 and 402). The phone row lifts the card by
moving the audio control between two fixed places in the row, one drawn
beneath its neighbours and one above them, rather than by reordering the row:
a painting canvas lays a row out again on the next frame when its parts change
order, and the test canvas never showed it.

Covering made one more change necessary. Centred on the gap, the open card ran
past the header's right edge wherever a long heading or a narrow phone put the
gap far enough right: in every book on a 320-wide canvas, and in Song of
Solomon up to a 430-wide one. Its ✕ went with it, partly or wholly off the
screen in 68 of the 73 books at 320, in 12 at 360, and in Song of Solomon up to
402 — and a card that covers the full-screen button and cannot be closed traps
the reader. So the open card now stops at the header's right edge
(`phoneAudioCellLayout`). That moves it left by up to 56 units at 320, 36 at
360 and 15 at 402, and not at all from 460, nor wherever it already fitted; the
closed speaker, and everything but the open card, is exactly where it was.
`chapter_header_cover_test.go` holds both headers to all of this, including
that every control of the open card, the ✕ first, is on the canvas.

**Tried and not kept:** a header that places its own parts
(`chapterHeaderLayout`) and, where the card does not fit beside the chapter
block, gives the open card a row of its own under the header. The header grew
by that row each time the card opened, so the reading text was pushed down on
every open and pulled back on every close. Its measurements, on canvases 320
to 1280 wide:

- The card fits beside the chapter block in every book from a canvas 520 wide;
  below that it needs the row of its own in some book.
- The full-screen button and the speaker fit beside the whole chapter block in
  every book from 399 wide. Where they did not, they moved into a column at the
  right edge (the button beside the title row, the speaker beside the chapter
  row): twelve books at 360 (Deuteronomy, 1 and 2 Chronicles, Ecclesiastes,
  Song of Solomon, Lamentations, 1 and 2 Corinthians, 1 and 2 Thessalonians,
  1 and 2 Maccabees; 192 chapters), three at 375, one at 390, and all but five
  books at 320, nine of them on a row of their own.
- The place had to be chosen by the widest rows the book's chapters make: by
  each chapter's own rows, a 360-wide canvas moved the controls between
  1 Chronicles 20, 21 and 22.
- It also moved the ✕ 3 units inside the card's corner, so its tighter corner
  followed the card's rounder one instead of standing out past it.

That work is on the branch `fu/narration-card`, commits 9c744ea25 (the
placement, its fit tests and the measurements above) and b23e5283f (the Android
fallback pane given the phone header). The branch was kept locally and not
pushed, so those hashes resolve only in the repository that holds it; what
mattered from them is written out here.

**The direction to investigate first:** the narration controls as an
additional horizontal bar, used depending on the layout. Where the layout has
room for the card beside the chapter block (a tablet, the desktop, a wide
window) the card can stay where it is; where it has not (a phone in portrait),
the controls would take a full-width bar of their own instead of a card over
the header. What the investigation has to settle: where the bar sits (under the
header, above the tab bar, or over the foot of the reading text), when it is
shown (while the reader has it open, or for as long as narration plays), how it
is dismissed, whether it carries the source chip as well as the transport, and
what it does to the reading text and the native text overlay's frame. The row
of its own tried above is the nearest thing built so far, and differs in being
a card, flush right under the header, that pushed the text down.

**What a better placement has to answer or satisfy:**

- Whether opening or closing the controls may move the reading text. The row
  of its own above moved it on every open and every close and was not kept; a
  bar placed elsewhere, or laid over the text, might not need to. Today's card
  keeps the header's height, so the native text overlay is not pushed again.
- Nothing the reader needs is hidden while the card is open, or, if something
  is, it is unreachable while it is and back the moment the ✕ is tapped, and
  no control shows in part beside the card (today's rule, which the tests
  keep: the card hides each small control whose glyph it touches, and covers
  only the part of the heading and the chapter line it lies over).
- The whole card, ✕ included, is on the screen, at every width and in every
  book (today's header moves the card in from its right edge for this).
- It holds from a 320-wide canvas, with the longest headings and chapter lines
  (Deuteronomy 34, Song of Solomon 8, 2 Thessalonians 3), up to a tablet, on
  iOS, Android and the Android fallback pane alike.
- The transport row keeps its 44-unit targets and the source chip keeps its
  label; the ✕ stays in the card's corner.
- The closed speaker gets a tap area of its own. Today, at 320, its box
  reaches under the copy icon and the full-screen button, which keep those
  taps: in Song of Solomon the button lies over 31 of the speaker's 40.
- Keyboard focus reaches nothing the card hides. Today's header keeps this:
  Tab passes over the controls the card hides, the full-screen button gives
  up the focus when the card opens over it, and a hidden button takes no key,
  even where an overlay that was up when it was hidden kept the page's focus
  for it. Where the card overlaps only the edge of the button's box, the
  button's glyph is whole beside the card and Tab reaches it; but its focus
  highlight fills that box, so the moment Tab lands on it there it is hidden
  as well, and stays hidden until the card closes. A reader tabbing round the
  header sees the button go.
- What a double tap on the ✕ does. The ✕ closes the card on the first tap,
  which puts back what the card was hiding, so the second tap lands there: on
  most phones the full-screen button, which the reader is then taken into
  (in 45 of 48 layouts measured: eight chapters, the longest headings among
  them, at six phone widths from 320 to 402). A ✕ that waited to see whether
  a second tap followed (a `fyne.DoubleTappable` ✕, closing on either) would
  take both taps, at the cost of the double-tap interval on every close; a
  shield left over the card's place for that interval would take them too,
  but leaves the controls it covered unreachable for that time after the
  card has closed. Not yet decided.
- The place is the same in every chapter of a book, so the controls do not jump
  as the reader moves between chapters (the branch chose it by the widest
  rows the book's chapters make for this reason).

## The reading page: what is left after 24 September 2026

Released in 1.2.20: the Psalm title's hyphenation and the pill under a
heading on Android's book page. Planned for: the Windows and Linux footnote
section, Android's centring and the prose-into-poetry rule in 1.2.23+;
hyphenation on Windows and Linux in the next major; the Android title gaps
and the other kept differences stay — see docs/ROADMAP.md.

Every surface takes its page from `reading_page.go` since 24 September 2026
(docs/READING_TYPOGRAPHY.md, "The reading page", keeps the per-surface list).
What the spec work left, each one a known difference rather than a guess:

- **The Windows and Linux pane justifies — DONE 24 September 2026.** It was
  ragged where the web, the Apple panes and Android from API 35 justify. The
  layout spreads each line a prose paragraph breaks for width (`spreadLine`,
  `reading_styled_layout.go`), moving only X and never a poetic verse's rows;
  the pane draws a justified line's words one object each (`mergeDrawRuns`),
  which the washes and selection already read; and a pointer in a widened gap
  goes to the nearer word (`offsetAtPos`), or a drag past a word would take the
  space and a double-click beside it the next word. The rejected alternative
  merged runs only where the gap was a natural space and taught hit-testing
  about widened gaps: more code in more places for the same page. Gated by
  `readingJustifyProse` in `reading_page.go` (false gives back the ragged page;
  docs/READING_TYPOGRAPHY.md, "Justified prose", says what carries the name),
  with a dev toggle to compare the two. Held by `reading_styled_justify_test.go`.
  On the Linux VM, Matthew 26 with the same scripted drag and scroll: about 4%
  more CPU dragging and 6% scrolling, the selection landing in the same place.
- **The Windows and Linux pane does not hyphenate** where the web and Android
  hyphenate (and the Apple stylesheet asks for it), so its justified lines on
  the phone page stand looser. Hyphenating it means a hyphenation dictionary and
  breaking a word across two runs while the copy and selection text keeps it
  whole. `readingJustifyProse`.
- **The Windows and Linux pane's footnote section stays ragged** where every
  other justifying surface justifies its notes. `styledFnWrap`
  (`reading_styled_footnotes.go`) draws each wrapped line as one string;
  justifying it means drawing each line's words apart as the body does, with
  the section's hit-testing following. `readingJustifyProse`.
- **Android centres the book page on the window, the iPad on its pane —
  FIXED 15 October 2026, for 1.2.23 (`60f26f5d2`); the tablet held upright
  is open.** The window is right for a landscape phone (the column
  clears the camera cutout evenly, `BtBridge.applyReadingPadding`); on a
  tablet with the side rail it set the column about half the rail's width
  left of the pane's centre. Centring on the window only when the pane spans
  it less the system insets would give both, and that is the fix, with
  "spans" read as standing evenly: a phone's pane held sideways keeps
  margins of its own, the same on both sides, so the column takes the
  window's centre where the pane's two margins in the window less its side
  insets are equal to within 2 px, which the frame's rounding allows, and
  the pane's centre otherwise, as on the iPad. The arithmetic is
  `BtReadingColumn.sides` (`android/BtReadingColumn.java`), with no platform
  types.

  The emulator passes ran on 15 October, the column's centre read back from
  the bridge and from the screenshot's ink. On the 10-inch tablet held
  sideways both ways the pane stands 136 px from the window's left, beside
  the rail, and 28 px from its right; the column stands on the pane's
  centre, where main's stood 54.5 px left of it. Phones held sideways on
  API 35, 33 and 30 take the padding main gives them. Held upright both ways
  the tablet's pane stands 14 px from the window's left and 28 px from its
  right: the right margin carries the reading body's own theme pad, which
  `buildReadingViewMobile` sets on the right only, so the pane is not even
  in the window and the column takes its centre, 7 px left of the screen's,
  where main's took the screen's, 6.5 px right of the pane's. The iPad's
  reading body has the same pad and centres its column in its text view, so
  the upright tablet now matches it. Open: keep that, or count the reading
  body's right pad as part of the pane (Go pushing it to the bridge) so the
  upright column stays where main draws it.

  `TestAndroidCentresTheBookPageOnItsPaneBesideARail` compiles and runs
  `BtReadingColumn` with the JDK over the geometry the bridge logged on the
  emulators, holding it to the padding logged there, and over geometry built
  from it: the cutout on the other side, side insets on the tablet, the
  reader's own full screen, a phone's rail, and panes 1, 2 and 3 px uneven,
  which hold the slack to 2 px. The rule before is its control: beside the
  rail it leaves the column 54.5 px off the pane's centre, and every phone
  fixture takes the same padding under both rules. Without a JDK the test
  skips, except on CI's Linux job.
  `TestTheBridgeTakesTheColumnFromBtReadingColumn` holds the bridge to it. A
  phone's Read tab shows a rail only with the landscape presentation switched
  off, which only a development build can do; its column follows the rail's
  rule. READING_TYPOGRAPHY.md's paragraph on Android's page and ANDROID.md's
  sentence on the reporter column give this rule since the branch was
  merged into `rel/1.2.23` on 15 October 2026.
- **A note pill under a heading on Android's book page — FIXED 6 October
  2026.** It sat 7px higher than on the phone page (the entry below, which
  has the fix).
- **Android sets a Psalm title's word gaps 28 units narrower than its space.**
  From API 35 a title is set ragged there as everywhere, since 1.2.19, by the
  swap that sets a heading ragged: its spaces are four-per-em spaces in the
  view, which the justified layout does not spread (`keepHeadingsRagged`,
  `isRaggedParagraph`, `android/BtBridge.java`; docs/READING_TYPOGRAPHY.md,
  "Justified prose"). That gap is 250 units in both cuts, and the italic cut
  the title is set in has a 278-unit space, so each gap is about 1.5 px
  narrower on a 420-dpi phone than on the other surfaces (in the bold cut a
  heading is set in, the space is 243). No breaking space matches 278 in
  Junicode Italic; the punctuation space (254) is 4 units nearer there and 4
  further in the bold cut. A `ScaleXSpan` of 278/250 over each of the title's
  gaps would close it, at the cost of a span per gap, which `readerText` would
  have to drop so a pasted title does not carry widened spaces.
- **The Apple panes and Android hyphenate a Psalm title that wraps,** where the
  web and the Windows and Linux pane leave its words whole. **The Apple half
  is DONE 6 October 2026, for 1.2.20:** `p.pst` sets `hyphens: none;
  -webkit-hyphens: none;`, and the title imports with a hyphenation factor of
  0 on the Mac and on the iPhone simulator, where it had 1; Psalm 18's title,
  laid out at 200 and 320 points, stopped breaking "Mu-sician", "de-livered"
  and "Yah-weh" (docs/READING_TYPOGRAPHY.md, "Justified prose";
  `TestAppleTitleIsNotHyphenated`). What follows is the entry as it was. The Apple `p.pst`
  (`reading.go`) sets no hyphenation, so it keeps the `hyphens: auto` of the
  `p` rule; Android sets `HYPHENATION_FREQUENCY_NORMAL` on the whole text view
  (`BtBridge.java`). On macOS, adding `hyphens: none; -webkit-hyphens: none;`
  to `p.pst` gives the imported title a hyphenation factor of 0 and breaks no
  word in it, so the Apple half is one rule, but it changes the Apple panes and
  wants its own check on them, iOS included (scripts/check-ios-pane.sh).
  **Decided 6 October 2026** — Psalm titles: no hyphenation on the Apple apps
  and Android 15+, recording Android 13–14 as a divergence. On Android it is
  `LineBreakConfigSpan.createNoHyphenationSpan()` over each title, a span
  API 35 brought, which earlier releases lack.
  **Android: FIXED 6 October 2026** (for 1.2.20). From API 35 the pass
  that sets the title's gaps (`keepHeadingsRagged`) sets that span over the
  title's paragraph (`keepTitleUnhyphenated`, `android/BtBridge.java`), and
  the selectable, justified view honours it: on the API 35 emulator WEB
  Psalm 102's title, which broke "over-whelmed" at the default size on a
  phone held upright, breaks between words, while the body still
  hyphenates ("be-came", "Ammi-nadab" in Matthew 1). The span shapes the
  page and is a framework ParcelableSpan, so the text that leaves the page
  drops it: `readerText`, and `copyAsRead`, which takes Copy over whenever a
  selection holds one. On the API 33 emulator nothing changes, the phone
  page's Psalm 102 pixel-identical to the base build: without the span,
  Android can still hyphenate a title that wraps from API 23, where the view
  starts hyphenating, to API 34 (the decision named 13 and 14; the same
  holds back to 6), a divergence docs/PLATFORM_MATRIX.md records.
  `TestAndroidPsalmTitleIsNotHyphenatedFromAPI35`
  (`android_title_hyphenation_test.go`) holds the bridge to it, with
  controls.
- **A paragraph that opens in prose and turns to poetry** is indented on the
  web, Android and the Windows and Linux pane (justified where those surfaces
  justify — whole on the web and Android from API 35, its prose only on the
  Windows and Linux pane, `readingJustifyProse`), and unindented and ragged on
  the
  Apple panes. The Apple dialect marks any paragraph holding
  a poem line `p.pm` and left-aligns it (`reading.go`), because TextKit would
  stretch its poem lines if it were justified, and the native indent keys on
  justification (`firstLineHeadIndent` in `reading_ios.go`/`reading_macos.go`),
  the one paragraph property the importer keeps. Matching the others there
  needs a second signal through the importer for "opens in prose" — an
  alignment the stylesheet can set and the importer keeps, such as natural
  against left — and the prose opening would still be ragged.
- **The web's footnotes keep their "Notes" label and full-width rule,** where
  the apps draw a short hairline: a web page has no app header to carry the
  section's name.
- **Browsers without container queries** (Safari before 16, Chrome before 105)
  show the web's phone page at every width.
- **The dev text-size slider — DONE 24 September 2026.** On the Read tab rather
  than the Links tab, because the Links tab takes the reading pane's place: a
  dev build's Links tab turns on a slider above the pane that moves the size
  from ×0.8 to ×1.6 and re-renders the pane in place, with the spec's numbers
  at that size beneath it (docs/READING_TYPOGRAPHY.md).

## iOS 27: after the scene life-cycle fix — FIXED 23 September 2026

Still open: the scale on iPhones whose nativeScale is not their scale
(1.2.23+); the iPadOS window buttons, an external display's scale and
Android's cancelled touch (stay); see docs/ROADMAP.md. The tab bar's style
was FIXED 14 October 2026, for 1.2.22 (the bullet below).

The launch failure itself (an Xcode 27 build refused at launch on every iOS 27
device) is fixed by `patches/fyne-2.7.4-ios-scene-lifecycle.patch` and
`scripts/ios-scene-manifest.sh`; `patches/README.md` (Patch 8) has the account.
Four pre-existing defects sat next to it. None was caused by that fix; all four
are fixed.

1. **FIXED: a resized iPad window drew the app for the whole screen.** iPadOS
   26 and 27 let the reader resize BibleText's window (Windowed Apps, Stage
   Manager), and Fyne sized its canvas from `[UIScreen mainScreen].nativeBounds`,
   so a smaller window showed a squashed header, no tab bar, and a reading pane
   past the window's edge — on the old build and the scene build alike.
   `patches/fyne-2.7.4-ios-window-size.patch` (Patch 10) reports the view's own
   size; a full-screen window reports exactly what it did before. On the
   iPadOS 27 simulator a window dragged to 375 × 643 lays out as a phone, and
   back at full size matches the full-size layout. `UIRequiresFullScreen`
   still takes effect on that simulator but UIKit warns it "will soon be
   ignored", so it was never the answer.
2. **FIXED: Fyne spelled the cancel method `touchesCanceled:`.** A touch the
   system took over was never ended; twelve in a row panicked with "out of
   touchIDs" (reproduced on the simulator). `patches/fyne-2.7.4-ios-touch-cancel.patch`
   (Patch 9) ends each cancelled touch off the canvas, where it cannot tap.
3. **FIXED: a tab answered only from the top of its icon down.** Fyne moves
   every touch up 8pt (`tapYOffset`, Android too), and the bar's 12pt of air
   above and below sat outside the tab cells, so a tap on the top half of an
   icon hit nothing. The air now lives inside each cell (`tabCell.padY`), so a
   tab answers from the rule down to the bar's bottom: after the 8pt shift the
   top half of an icon selects it, and only a finger within a few points of the
   rule still lands above the cells. The drawn bar is unchanged to the point
   (every rule, icon and label measured identical before and after).
   `TestNavBarIsTappableAcrossItsHeight` taps the old dead bands through the
   toolkit's own hit test and fails on the old layout; on the simulator a
   finger placed where the old layout missed now opens Search.
4. **FIXED: `scripts/check-ios-pane.sh` compiled against stock Fyne.** It now
   compiles twice, the second time against the patched toolkit, regenerated
   each run and reached through a scratch copy of go.mod. A deliberate
   `#error` in a patch fails it; `TestTheIOSCheckCompilesThePatchedToolkit`
   pins the script's text. It builds its copy in `build/ios-check/fyne`, never
   in `third_party/fyne`, so it cannot rewrite the tree a packaging build is
   compiling from. The macOS CI job runs the same script.

Still open, from the same work:

- **iPadOS window buttons sit over the header's title.** In a small iPadOS 26
  or 27 window the red, yellow and green window controls cover the start of
  "BibleText" in the top-left of the header. Old and new builds alike; the
  insets Fyne receives are the window's safe area, which does not include the
  controls. UIKit offers a layout region that avoids them
  (`UIView.LayoutRegion.margins(cornerAdaptation:)`); feeding a left inset for
  the header row from it would clear them. Cosmetic.
- **The tab bar's style is chosen when the window is built.** `tabBarStyleFor`
  picks the edge-to-edge or centred bar from the width at build time, and
  `layoutWatcher` rebuilds only when class, rail or landscape change. A resized
  iPad window that crosses 560pt while it stays taller than wide keeps the old
  style until something else rebuilds. Adding the style to `renderedLayout` is
  one line. The concern that held it back, an iPhone rebuilding on rotation on
  Books and Search, no longer applies: a phone held sideways draws the rail,
  never the bar, and its rotation rebuilds for the rail already (1.2.18).
  FIXED 14 October 2026, for 1.2.22 (`f49b25091`; ROADMAP.md, 1.2.22, row
  25): `renderedLayout` carries the dress where a bar is drawn, read from
  the canvas as the bar and the rail read it, since the width the watcher
  is handed is the canvas's less the safe area and a term read from each
  could disagree for ever. `TestATallWindowResizedAcrossTheBarsWidthRebuildsTheBar`.
  Not seen on a device: the iPad simulator's windowing mode refuses an
  app's own request to change its window.
- **On a device whose nativeScale is not its scale, a Fyne unit is not a
  point.** The 12 and 13 mini, the Plus models and Display Zoom. The canvas is
  counted in `nativeScale` pixels (nativeBounds) while Fyne's canvas scale and
  the GLKView drawable follow `UIScreen.scale`, so clip rects and the native
  reading pane's frame are off by the ratio (about 4% on a 12 mini).
  Pre-existing and unchanged by the window-size patch, which keeps full-screen
  values exactly. The likely fix measures with the drawable's scale in both
  `goAppReportSize` and `sendTouches`; it changes those devices' numbers, so it
  needs a device to check on.
- **A window moved to an external display keeps the iPad's scale.** On an
  M-series iPad in Stage Manager the app's one window can move to an external
  screen; the delegate still reads `[UIScreen mainScreen]` for scale and size.
  Pre-existing; `view.window.screen` would be the source.
- **Android never ends a cancelled touch either.** Its driver maps
  `ACTION_CANCEL` to a move (`android.go`), so a touch the system takes over is
  never ended there. It cannot panic (Android numbers touches by pointer id),
  but a drag it began is not ended. The iOS fix (Patch 9) does not cover it.

## A note link that opened the chapter and stayed at the top — FIXED 23 September 2026

The last open part, a reader who scrolled away from a note they arrived at
taken back by the next width change, was FIXED 14 October 2026, for 1.2.22
(the end of this entry; ROADMAP.md, 1.2.22, row 26).

Reported on iOS three times: 10 August (a note tapped in the dev Links tab
while on its chapter), 18 September (a note link from Messages, the app
backgrounded) and 23 September (once in several tries, the reader's own note).
The chapter opened, the note and its wash were there, and the view stayed at
the top of the chapter.

**The mechanism that was reproduced and fixed.** The Apple panes re-assert the
scroll position after an import settles — a turn later and 200ms later — and
after a width change. A re-assert reads the arrival class when it RUNS, and
only the render a link asked for is explicit: every render after it pushes
`arriveNothing`. When such a render landed between a link's import and its
re-asserts, the re-assert found nothing to place and pinned the view to the
top, over the note the link had just placed it on. On the iPad simulator, with
the app on the WEB and a note link naming the BSB (so the arrival also switches
translation), `BT_SCROLL_DEBUG=1` read: "landed on highlight", then
"arrival=nothing", then "pinned to TOP" twice. The appearance round trip iOS
makes to snapshot a backgrounded app for the switcher rebuilds the window the
same way, which is the owner's route. The re-asserts are now
`btIOSReassertPlacement` (both import re-asserts, both width re-asserts) and
`btMacReassertPlacement` (the macOS frame change): restore, then arrival, and
never the top-pin. `arriveNothing` was never meant to mean the top
(notes_arrival.go). The synchronous resolver keeps its top-pin, which is how a
plain entry opens a new chapter. Same reproduction after the fix: "landed on
highlight", then "re-assert: nothing to place — view left where it is", and
the view on the note. `TestDeferredReassertsNeverPinToTheTop` holds it by what
the re-asserts do, not only by the names they call: its first version passed a
re-assert that ended in the pinning resolver, or in a `setContentOffset:`, and
it now fails on both. The dev scenario is `headwash`
(docs/VISUAL_TESTS.md).

A side effect worth knowing: a reader who scrolled away from a lit wash and
then rotated the phone, or resized the Mac window, used to be thrown to the
top of the chapter by the same re-assert. The view now stays put.

**Found by the same investigation and deliberately not changed, with why:**

- *A reader who scrolls away from a note they arrived at is taken back to it
  by the next width change* (a rotation that does not rebuild the layout, an
  iPad or Mac window resize). The arrival class is written only when Go pushes
  the note (`bibleTextSetNote`, the macOS twin); the reader's own scroll clears
  the restore and the "you left off here" marker but not the class, so a width
  re-assert still resolves it. Older than the fix above, which neither causes
  nor cures it. The likely fix is to set the class to nothing beside the restore
  disarm in `scrollViewDidScroll` and `btMacUserScrolled`: a scroll by the
  reader is the end of an arrival. Needs a device to confirm the rotation path.

- *A re-import the reader did not ask for could capture the position before
  the link's own import lands.* It cannot on iOS: `bibleTextTVCaptureAnchor`
  runs inline only on the main thread, and Go calls it from its own thread (the
  trace prints the native side's `arrival=nothing` before Go's own `push:` line
  for the same push), so the capture waits behind the queued import and
  records the placed position. A generation check in the capture, and carrying
  the arrival class across renders until the reader scrolls, were designed
  (`appleArrivalForPush`) and not built: they close a window this threading
  already closes, at the cost of new state with its own clearing rules.
- *The import's retry ladder does not know which push it belongs to*
  (`bibleTextTVSetHTML`: three attempts on the same data, whichever succeeds
  marks the newest generation applied). It matters only when an HTML import
  fails, which the code's comments say is likeliest right after returning to
  the foreground; the failure rate has never been measured. The fix is to
  stamp each attempt with `gBodyGenPending` read on the main queue and drop a
  superseded one. Fixed with P3 for 1.2.23, with the generation stamped at
  the push instead: read on the main queue it is shared by two pushes made
  in one main-thread turn, as the Mac's are, so the announcement issues it
  and Go hands it over with the push, and every attempt and the fallback
  check it before acting (`51f7efe66` on `fu/1223-ai4-p3`, P3's entry).
- *The plain-text fallback never places the view* and does not tell Go the
  import failed. The owner saw a fully styled chapter each time, which rules
  this path out for the reports so far. P3's fallback page (for 1.2.23) is
  installed as an imported page is, so it places the view
  (`bibleTextScrollReadingTV`, `bibleTextMacScrollTV`); it still does not
  tell Go the import failed.
- *The fast path can consume an arrival on a pane that has gone blank*, and
  the foreground recovery then rebuilds without it. Rests on two premises
  nobody has verified.

Android was checked by reading and is not affected in this way: it captures
the reader's top line before a theme re-render and preserves it across the
re-render (reading_android.go), rather than re-asserting from a class a later
render can overwrite.

**A reader who scrolls away from a note they arrived at, FIXED 14 October
2026, for 1.2.22** (`291c5c550`; ROADMAP.md, 1.2.22, row 26). The fix
recorded in the first bullet above: the reader's scroll sets the arrival
class to nothing beside the restore's disarm, in `scrollViewDidScroll`
(`btIOSEndArrival`) and `btMacUserScrolled`, as the next push would.
`TestAReadersScrollEndsTheArrival`. Seen with development builds before and
after, arriving at Psalm 119:100 (WEB) and scrolled 1,500 points on, to verse
121: on the iPad Pro 13-inch (M5) simulator in full-screen reading, whose
windowing mode refuses an app's request to rotate, a 250-point narrowing of
the pane through `bibleTextTVSetFrame` put the build before back at verse 100
and left the fix at 121; on the Mac a window narrowed from 1280 to 1000
points put the build before back at verse 100.

An older defect on the Mac, which the arrival's re-assert had hidden: a
change of the pane's width moved any reader who had scrolled. Setting the scroll view's
frame wrapped the text at the old insets first, AppKit moved the clip with
that passing wrap, and the insets put right straight after left the clip
there: a reader 5,000 points into Psalm 119, at verse 73, was at verse 112
after the window narrowed, and one at verse 121 at the end of the psalm.
FIXED the same day (`d07557382`; ROADMAP.md, 1.2.22, row 27):
`btMacApplyFrame` reads the verse at the top and how far into it before a
change of width and puts it back after the insets and the note's bands, and
the re-assert still places a restore or an arrival over it.
`TestTheMacFrameChangeKeepsTheReadersPlace`. With it the Mac kept verse 73
and verse 121 through the narrowing and back. The iPad's frame path kept
the reader's place before the change, as UIKit keeps the content offset.

## The NKJV's missing spaces: report upstream, and decide on a correction list

Planned for: 1.2.23+ once decided; the upstream report needs no release —
see docs/ROADMAP.md.

The licensed feed runs two words together at 286 places in 275 verses
(re-counted 22 September 2026, `scripts/nkjv-upstream-joins.py`; the method and
the exclusions are in docs/SOURCE_FIELDS.md). Every surface shows them
faithfully, because that is what the provider sends; they concentrate in the
words of Christ and in the prophetic poetry, which is why they are noticed.

Two actions, independent:

1. **Report it to API.Bible.** A report with the complete list was drafted on
   7 September; whether it was sent is not recorded. Only the provider can fix
   the source for everyone.
2. **Decide whether the app carries a correction list** meanwhile — one entry
   per affected verse, each restoring the printed reading at one position, and
   each keyed to a hash of the verse's current text so it switches itself off
   the moment the provider changes that verse. This reverses the 7 September
   position, which rejected a RULE (insert a space wherever a note meets two
   letters) because a rule damages the legitimate joins — the closed em-dash in
   60 places, `young|est`, `j|ust`. A reviewed list has no such reach: a verse
   not in it is untouched. What it does require is that every entry be checked
   against the printed edition before it ships. An entry needs no licensed
   text at all — book, chapter, verse, a rune offset and a hash of the verse —
   so the list can be committed like the red-letter tables are. The
   decode-time count becomes the tripwire that says when entries retire.

## One way to turn a verse into drawn words, not two

Planned for: 1.2.23+ — see docs/ROADMAP.md.

Found 22 September 2026 by the spacing audit (`spacing_audit_test.go`). Two
surfaces draw a verse word by word on a canvas rather than handing text to an
engine: the styled desktop pane and the verse-of-the-day card. They tokenise
differently. The pane tokenises the WHOLE verse text once and colours each
token by offset (verseTokens, redLetterTokenFlags), so it never has to decide
where a space goes. The card tokenised each RUN and re-derived the spaces
between runs — a second implementation of spacing — and dropped any space that
was a run of its own, which red-letter spans produce constantly: 1,302 lost
spaces in the NKJV, 129 in the WEB. That is fixed and property-tested, but the
second implementation is still there.

The durable fix is one shared function — verse (or passage) in, a sequence of
words out, each word carrying its styled pieces and whether a space or a line
break precedes it — derived by tokenising the verse text once, so a space
cannot live at a run boundary. Both word-drawing surfaces consume it. Carry
the card's rune-level colour splitting into it rather than the pane's
whole-token rule, or two BSB verses (Mark 7:34, Acts 20:35) change colour on
the card. Keep the audit as the cross-surface guard either way.

## More share-card typefaces with true small capitals

Planned for: next major, behind `next` — see docs/ROADMAP.md.

Surveyed 23 September 2026, and optional: since `cardText` (share_image.go)
every one of the seven card faces already draws the divine name, six of them
from their own capitals at 0.70. An added face would bring designed small
capitals and more variety, not correctness.

Eleven SIL-OFL serifs from Google Fonts' repository were checked for the five
characters the name uses (`ʟ ᴏ ʀ ᴅ ɢ`); five carry all of them, and each was
rendered through the real card renderer as a Psalm 23:1–2 card:

| typeface | size (regular) | verdict |
| --- | --- | --- |
| **Junicode** | already bundled (reading face) | **add** — costs nothing, and ties the card to the page it was shared from. Its default figures are old-style, so the citation's "23:1–2" sits low; a visible difference, not a fault. |
| **EB Garamond** | ~830 KB (variable) | **add, if one more** — the traditional Bible face and the finest of the five on the card. Google ships it variable-only, and `x/image/font/opentype` cannot pick a weight from a variable font, so it needs the static Regular and SemiBold from the designer's own release, and a read of the licence's reserved-name terms first. |
| Gentium Book Plus | ~800 KB | skip — built for linguistics; reads utilitarian on a card |
| Charis SIL | ~735 KB | skip — same family of purpose |
| Noto Serif | ~1.8 MB (variable) | skip — the most generic and the largest |

None of Source Serif 4, Newsreader, Literata, Libre Caslon Text, Crimson Pro or
Alegreya carries any of the five.

Two effects of adding any face, both worth saying before it ships:

- The rotation stays whole. Regenerate walks colour schemes (13) and faces
  together, and reaches every pairing only if the two counts share no factor;
  13 is prime, so 8 or 9 faces are both fine.
- Most verses' DEFAULT card changes typeface. The face is chosen from a hash of
  the reference modulo the number of faces, so adding one re-deals the
  defaults. Nothing breaks; a reader sharing the same verse again sees a
  different look.

To re-render the comparison: parse each candidate with `opentype.Parse`, set
`shareTypefaces` to a one-face list in a tagged test (after `loadShareTypefaces`
has run once), and call `renderVerseImage`. The four `share_image_smallcaps`
tests cover any new face automatically, since they iterate the typeface list.

## Fyne's RichText breaks a word mid-word at the start of a segment

Planned for: 1.2.23+ — see docs/ROADMAP.md.

Found 22 September 2026, same audit. When a segment begins partway along a
line and its first word does not fit what is left, `lineBounds` (Fyne 2.7.4,
widget/richtext.go) measures against the remaining width, finds no space
inside it, and `findSpaceIndex` returns the character fallback — so the word
is cut ("Bein" / "g therefore") instead of moving to the next line. The right
behaviour when the row is a partial first line is to emit an empty boundary and
wrap the whole word, as the `fallback < 1` branch already does.

Reachable on every platform through **search result cards**, whose highlight
segments start mid-line; also on the Android RichText fallback pane, which a
working install does not reach. A highlight that starts inside a word — "he"
in "the" — puts a segment boundary there, so the cut can land mid-word even
in a short term.

**Two mechanisms, measured 22 September 2026, and only one is a small patch.**

1. *A word cut inside one segment.* Fyne already intends to start a new row
   when a partly-filled row has no space in the part that fits — `if high ==
   fallback && subWidth <= max.Width { // add a newline… }` — but the test is
   broken twice: `high` is an absolute index and `fallback` is relative to
   `low`, so it only works at the start of a segment; and it demands that the
   WHOLE rest of the segment fit one full row, which a long run of text never
   does. Adding `|| (measureWidth < max.Width && no space in sub[:fallback+1])`
   fixes it. Measured with the spacing audit against the patched toolkit: the
   mobile RichText pane went from 1,392 / 1,090 / 1,549 / 1,159 split words
   (WEB / BSB / WEBC / NKJV) to 0 / 0 / 0 / 3, the three being correct breaks
   after an em-dash or hyphen, and Fyne's own widget suite still passes. The
   draft diff is small; it lives outside the repository until it ships.
2. *A word split across two segments.* A highlight inside a word — "he" in
   "the" — makes "t" and "he" separate segments, and Fyne treats every
   segment boundary as a legal break. This is what search cards show, and
   patch 1 does not move it (628 / 687 / 734 / 994 before, 631 / 690 / 738 /
   994 after; the small rises are the corrected rows shifting, not new cuts).
   Fixing it means keeping a word together ACROSS segments in RichText's row
   layout: when a segment's first row is empty and the previous row ends in a
   non-space that the new segment continues, the tail of that word has to move
   down with it. That is a real change to Fyne's layout, not a one-line fix.

Held out of 1.2.14 on the rule agreed for it — ship only if the search-card
count fell to zero — since patch 1 alone does not touch what readers see on a
working install. The natural next step is patch 1 plus the cross-segment fix
together, verified with the same lens.

An earlier version of this entry also proposed word-bounded highlighting in
`matchRanges`; that was wrong. Keyword search itself matches substrings
(`BibleData.Search` is `strings.Contains` on the lowered text), so a verse
found for "he" was found because of the "he" in "the", and highlighting it
there is the honest answer to why it matched. Word-bounded highlighting would
hide some splits by misreporting the match.

One real mismatch sat beside it, FIXED 22 September 2026: search matches the
query as a PHRASE, while the highlighter split it into words, so "he said"
found the phrase but highlighted every "he" and "said" in the card. The card
now highlights the phrase (`highlightTermsFor`, search.go).

## One universal macOS download instead of two — DONE 17 September 2026

The direct download offers `BibleText-macOS-AppleSilicon.zip` and
`BibleText-macOS-Intel.zip`; the Mac App Store build is already universal
(`scripts/release-mac-store.sh` builds both slices and joins them with
`lipo -create`). The direct path is the odd one out, not a considered
difference.

Measured on the 1.2.10 artifacts rather than estimated: the two zips are
21.1 MB and 21.6 MB, their executables 40.8 MB and 41.6 MB, and the bundle is
essentially all executable — so a real `lipo` join of the two published
binaries produces an 82.5 MB executable and a **42.2 MB zip**. There is no
shared-resource saving to be had; universal is almost exactly double.

Three reasons it is still probably right:

- One of the two wrong choices is fatal. An Apple Silicon build on an Intel
  Mac does not launch at all; an Intel build on Apple Silicon runs under
  Rosetta, slower but working. The page cannot tell which machine a reader
  has.
- It deletes a hazard this repository has already paid for. The macOS job
  packages twice from one checkout, and `fyne package` rewrites the ledger's
  `Build` after each success — 1.2.5 shipped two zips stamped 46 and 47 for
  the same commit. The job now carries a saved ledger, a restore function, a
  trap and a two-plist comparison purely to contain that. One universal
  package needs none of it.
- It removes a button from a page whose whole virtue is that it is short, and
  a decision from a reader who may not know which Mac they own.

The cost is one thing only: about 21 MB of dead weight in every download.

Shipped in the release workflow the same day, following the Store script:
build each slice, `lipo -create`, package once. The asset is now
`BibleText-macOS.zip`; the two old names are gone, so a bookmark of either
returns 404. The job asserts both slices are in the packaged binary — a
packager that rebuilt instead of copying would ship one slice and still
look right from outside — and checks each slice's own `minos`, which is
what the loader enforces. The ledger save-and-restore stays, because the
packager still rewrites `Build` and the verification reads it back; what
went is the two-packages-one-checkout hazard that stamped 1.2.5 with two
different builds.

## The first frame is laid out for a window the app did not get — FIXED 18 September 2026

Still open: the first window's position on Windows ("Still uncovered by
either", below) — stays; see docs/ROADMAP.md.

`app.go:548` asks for a fixed window size and never reconciles it with what the
desktop actually granted:

    window := myApp.NewWindow("BibleText")
    window.Resize(fyne.NewSize(1280, 860))

There is no clamp to the work area. On a desktop that cannot give 1280x860 the
window comes back smaller, but the first layout pass has already run against the
requested size and nothing re-runs it. The reading column is centred in
`styledColumn.Layout` (`reading_styled_area.go:386`) with

    x := (size.Width - w) / 2

so it is placed for a canvas nobody has: a wide dead gutter down the left and
the text crowding the right edge. It is not a flash. It persists until something
forces a fresh layout pass -- maximising, or dragging any edge -- after which it
is correct for the rest of the session.

THIS IS ALREADY IN THE REPOSITORY. `docs/windows-store-smoke-john3.jpg`, added
by 84536fd9a on 16 September, is a capture of exactly this defect from the
`windows-latest` runner (1024x768). Measured: the pane runs x=62..899, the text
x=379..806, margins 317px left against 93px right. It predates the ANGLE switch
by a day, so Direct3D has nothing to do with it. The reason it was committed
without comment is that the only check looking at that image asked whether the
app had DRAWN -- and it had.

What pins the trigger to the missing resize rather than to the width: MAXIMISING
cures it. A maximised window on an 800x600 guest is about 800x552, far narrower
than 1280, and its layout is correct. So it is not "the window must be at least
1280 wide" -- any resize event at any size reconciles it, and a launch that never
receives one stays wrong.

Invisible on the machines it is built on, for two independent reasons. A display
of at least 1280x860 grants the request exactly, so there is no mismatch to
correct, and this development Mac clears it with room to spare: GLFW reports a
videomode of 1728x1117 and a work area of 1728x1022 at y=33, measured on the
machine rather than inferred.
And the pane that mis-places the column is Windows/Linux only: `useStyledPane()`
is false on macOS, which uses the native NSTextView overlay in
`reading_macos.go` instead. The configurations that do NOT clear the size are
ordinary: 1366x768 at 125% scaling is 1093x614 logical, and 1920x1080 at 150% is
1280x720.

THE GATE IS IN PLACE BEFORE THE FIX, deliberately.
`scripts/check-reading-centred.py` measures a screenshot's ink profile, finds the
text block and its pane, and requires the two margins to agree within 15% of the
pane width. It self-tests against synthetic images with a known answer on every
run, and `msstore.yml` runs it against the committed capture above with
`--expect-fail` before judging the live one, so a rule that has stopped working
fails loudly instead of passing. Mutation-proved four ways: a limit too
permissive, a limit too strict, a blinded ink detector and a blinded column
threshold all trip the self-test rather than returning a false pass.

An earlier attempt at this gate asserted that the WINDOW fits the work area.
That check would have passed on the committed failure: in that capture the
caption buttons are fully visible and the content stops at the taskbar, so the
window was clamped correctly and it is the canvas that was not. Recorded because
the wrong invariant looked obviously right.

FIXED, by asking for a window the desktop can grant. `app.go` now calls
`startupWindowSize(startupWorkArea(myApp))`:

  - `window_size.go` holds the arithmetic and nothing else, so it is testable on
    a host with any screen or none. The work area arrives as a parameter.
  - `window_workarea_desktop.go` measures it. Fyne exposes no screen accessor at
    all -- no `Window.Size`, no monitors on `fyne.Driver`, only
    `Settings().Scale()` -- but GLFW is already linked into the binary (it is
    what the desktop driver is built on), and `Monitor.GetWorkarea` is the
    number the toolkit never asks for. `glfw.Init` is idempotent and Fyne sets
    no init hints before its own, so calling it early discards nothing;
    verified, along with a second `Init` being a harmless no-op.
  - `window_workarea_mobile.go` stubs it for ios||android, because `app.go`
    carries no build tag and so `Run` compiles there even though neither mobile
    entry point calls it.

The scale term is the whole correctness argument and it is the part the original
design shipped untested. Fyne multiplies points by
`round(system*user*10)/10`, and `SystemScaleForWindow` is per-platform: a hard
1.0 on macOS because its scaling happens at the texture level, the monitor's
content scale on Windows. Two things fell out of testing it that would each have
produced a wrong window:

  - Go rounds a half away from zero, so a 125% display becomes a scale of **1.3**,
    not 1.25. Dividing by 1.25 would leave the budget about 4% too generous --
    the same defect, just smaller.
  - This Mac reports a content scale of 2.00 while GLFW's work area is already in
    POINTS. Using content scale uniformly would compute an 864x511 work area and
    clamp the window to 860x471 on every Retina Mac. The darwin branch is
    load-bearing, and is now verified against the machine.

Verified: the seven-mutation battery on `window_size.go` (each clamp removed,
each frame allowance zeroed, the fallback inverted, the rounding dropped, the
scale ignored) is caught by the tests; the full suite, the iOS pane check and all
nine hygiene checks pass; and the app launches on this Mac with the early
`glfw.Init` and opens at 1280x892 outer -- the preferred 1280x860 content plus a
32pt title bar, granted unchanged, exactly as the probe predicted for a 1728x1022
work area.

NOT yet verified end to end: that a Windows build carrying this renders centred
at 1024x768. That is what the `msstore.yml` gate is for, and it cannot run from
this host.

TABLED, with triggers: an 8th Fyne patch making the toolkit keep the size it
was GRANTED. The root cause is that `internal/driver/glfw/window.go` sets
`w.canvas.size = size` synchronously and then calls `processResized` with the
requested figure; the patch would read back `w.view().GetSize()` after
`SetSize` and pass that instead. It was scored down during design on the claim
that a patch never reaches a build, which is FALSE -- `release.yml`,
`msstore.yml`, `linux-stores.yml` and the release scripts all inject
`go mod edit -replace fyne.io/fyne/v2=./third_party/fyne`.

Tabled anyway, for three reasons that outweigh being the root cause:

  - It cannot be tested where it runs. There is no `replace` in the committed
    go.mod, so local `go build` and `go test` use stock Fyne; the patch could
    only be held in place by asserting its TEXT, the way
    dark_mode_follow_test.go does. The clamp is plain Go with a seven-mutation
    battery behind it, and that is a much better verification loop for the case
    that actually bit.
  - Real blast radius. `processResized` also feeds fixed-size windows and
    `fitContent`, and the branch above it records the requested size
    deliberately, with a comment saying an invisible window may never get the
    event. Reading `GetSize()` back before the window is shown is precisely
    where that becomes delicate.
  - Its marginal value is over configurations there is no evidence of hitting,
    against a permanent cost at every Fyne bump.

What makes tabling safe is that the symptom now has a DETECTOR:
`scripts/check-reading-centred.py` runs on every Store build and fails with a
screenshot whenever the column is off-centre, whatever the cause. Fix the known
cause cheaply, watch for the unknown ones, and spend the expensive effort only
if the watch fires.

Do it when any of these happens:

  - the centring gate fails on a build that already carries the clamp -- that is
    a mismatch from a cause no clamp can predict, and the screenshot will say so;
  - a mixed-DPI multi-monitor report arrives. This is the clamp's one
    non-hypothetical weakness: it reads the PRIMARY monitor's content scale
    while Fyne uses the scale of the monitor the window actually lands on. It
    mostly does not bite at launch because the Win32 CW_USEDEFAULT cascade puts
    the first window on the primary;
  - tiling window managers become a supported configuration (i3, sway, yabai
    assign geometry regardless of what is asked for, so the clamp is inert
    there);
  - a Fyne bump changes the Resize path, at which point the patch and the clamp
    should be reconsidered together.

Note for whoever picks it up: upstream issue 6368 was filed and closed, so this
belongs on the fork (cubancorona/fyne, bt-main) rather than being re-filed. And
do not pin the assertion to a removed line -- a regenerated diff may emit that
hunk as pure insertions.

Still uncovered by either: window POSITION. Win32 places the first window by the
CW_USEDEFAULT cascade, the app never calls `CenterOnScreen`, and
`doCenterOnScreen` (`window_desktop.go:155-177`) centres against `GetVideoMode`
rather than the work area, so a clamped window is still a cascaded one.

**Closed 6 October 2026, verified end to end.** The fix (7371b1a92,
18 September 2026) has shipped in every release since 1.2.12, and the
check this entry was waiting for has run: on the 1.2.19 release commit
(8a135f998, 3 October 2026) the Microsoft Store workflow's centring gate
refused the committed failure and passed the live render on both
architectures, its margins 190 and 188 px on x64 and 128 and 93 px on
arm64, inside the 15% limit. The four triggers above stay as the watch
list for the tabled patch. The window's position, which neither covers,
stays in the backlog (docs/ROADMAP.md).

## FIXED: Windows arm64

Attempted, withdrawn and fixed on 19 September 2026. The `windows-11-arm` image
ships an **x86_64 mingw gcc**, so cgo handed `runtime/cgo`'s aarch64 assembly to
an x86 assembler:

    gcc_arm64.S:30: Error: no such instruction: `stp x29,x30,[sp,'

The machine is ARM; its compiler was not. `scripts/fetch-llvm-mingw.ps1`
installs llvm-mingw's aarch64-hosted, aarch64-targeting toolchain, pinned by
sha256 the way ANGLE and the AppImage tools are. Both the Store package and the
release zip build it.

Verified on the runner rather than assumed: the MSIX declares
`ProcessorArchitecture="arm64"`, and `BibleText.exe`, `libEGL.dll`,
`libGLESv2.dll` and `d3dcompiler_47.dll` are all genuine ARM64 images
(`0xAA64`). It installed and passed the render smoke.

Neither blocker anyone expected was real: ANGLE publishes `angle-arm64` at the
tag already pinned, and the compiler was solvable by shipping one.

Still `runner`, not `field` — nobody has run it on a real Windows ARM machine.
The local VM is the obvious next step.

## The site must not be published before the release that first ships ARM — NO LONGER TRUE 6 October 2026

`docs/index.html` now offers **Linux — ARM** and **Windows — ARM** downloads,
pointing at `releases/latest/download/BibleText-Linux-arm64.tar.xz` and
`BibleText-Windows-arm64.zip`. Nothing produces those names yet: the jobs that
do are in this same change and have never run, so the current `latest` release
carries neither.

`scripts/check-public-surfaces.py` is deliberately forward-looking — it holds
the page equal to what the release WORKFLOW uploads, not to what is published —
so it is satisfied, correctly. The gap is one of ordering, and it is invisible
to every check in the repository:

  - publish the site before the next release and both buttons 404;
  - `scripts/publish-site.sh` is the only publisher and has no coupling to
    release state, so nothing stops that happening.

So: **cut the release first, confirm both assets are attached, then publish the
site.** Worth a check in publish-site.sh that every
`releases/latest/download/<name>` the page links actually exists on the latest
release — that would close it permanently rather than relying on remembering.

**No longer true, 6 October 2026.** v1.2.12 was the first release with
both ARM assets, and v1.2.19, the latest release, carries
`BibleText-Linux-arm64.tar.xz` and `BibleText-Windows-arm64.zip`, so both
links on the download page resolve. The lasting guard, a check in
`scripts/publish-site.sh` that every `releases/latest/download/` link the
page carries exists, stays a candidate for the release pipeline
(docs/ROADMAP.md, "Stays in the backlog").

## FIXED: `make install` on the Linux tarball

Found on 19 September 2026 the first time anything ever ran the installer, and
fixed the same day. It had affected every architecture since the scheme-handler
work, because it was in the packaging template rather than the build.

The packager wrote the Makefile's icon variable without a file extension
(`Icon := "uk.co.bibletext"`) while packaging the icon as
`uk.co.bibletext.png`, so `make install` could not find it. Two consequences,
and the second is the one that mattered:

  - the target exited NON-ZERO after installing the executable and the desktop
    entry, leaving a reader an application with no icon and an error they had no
    way to interpret; and
  - because make stops at the failed line, it **never reached
    `update-desktop-database`** — so a tarball install produced no
    `mimeinfo.cache`, and a `bibletext:` link opened nothing. The scheme handler
    this whole patch exists for did not work for anyone installing from the
    tarball.

Fixed in `patches/fyne-tools-1.7.2-linux-scheme-handler.patch`: the four lines
that use the icon as a FILE NAME now say `$(Icon).png`, while the `Icon :=`
assignment and the .desktop entry's `Icon=` keep the bare name, which is what
freedesktop icon-theme lookup expects. The patch was regenerated against the
checksum-verified module zip rather than hand-edited, and applies with `-F 0`
and no fuzz. Its own test asserts both directions and was mutation-proved.

Verified end to end on an arm64 machine: `make install` exits 0, installs all
three files, and produces `mimeinfo.cache`. `scripts/check-linux-package.sh`
now requires all of that on every packaged tarball — proven to pass the fixed
tarball and fail the old one.

## FIXED: the executable was named after `cmd/desktop`, not after the app — 19 September 2026

Still open: iOS ships `CFBundleExecutable: main` (below) — stays; see
docs/ROADMAP.md.

`fyne package` names the executable after its SOURCE DIRECTORY. Ours was
`cmd/desktop`, so every channel that did not override the name shipped a binary
called `desktop`. Each consequence was user-visible:

- the Linux tarball installed **`/usr/bin/desktop`** — a generic command in the
  reader's PATH that any other Fyne app packaged from a `desktop/` directory
  installs over, and that `make uninstall` then removes from under it;
- `desktop` was the command a reader had to type to start the app;
- a Linux volume control listed the application playing narration as
  **`desktop`**;
- macOS shipped `CFBundleExecutable: desktop`, so Activity Monitor, `ps`,
  Console and every crash report named the app `desktop`, on BOTH the direct
  download and the Mac App Store build.

Windows was never affected: its packaging line already passed `--executable
BibleText.exe`, and the shipped zip and `.msix` both carry `BibleText.exe`.

**How it was found, and why nothing caught it.** The shipped 1.2.12 arm64
artefacts were run on a real Linux desktop. The snap registered with the sound
server as `ALSA plug-in [bibletext]`, the AppImage as `PipeWire ALSA
[bibletext]`, and the tarball as `PipeWire ALSA [desktop]` — the snap and the
AppImage each rename the executable on their way in, and only the plain tarball
shipped it raw. Cause was settled by copying one binary to a second filename and
running it under each name: same sha256, and the sound server's client name
moved with the filename. Meanwhile `scripts/check-linux-package.sh` had
*hardcoded* `grep -q '^Exec=desktop %u$'`, so the check ratified the name
instead of questioning it, and derived the installed-entry name from whatever
the archive happened to contain — a check that follows the artifact cannot
notice the artifact going wrong.

**Passing `--executable` per channel was not enough.** It repaired the packaged
artefacts but could never reach the two routes `README.md` advertises, because
those build on the reader's own machine from the module path: `fyne install
github.com/cubancorona/bibletext/cmd/desktop@latest` (the command the public
Fyne apps directory prints) and `go install` / `go run` of the same path.
**`fyne install` has no `--executable` flag at all**, so no packaging change
could have fixed that route.

**The root fix: `cmd/desktop` is now `cmd/bibletext`.** Every packager then
defaults to the right name instead of having to be told, the module-path routes
install `bibletext`, and macOS falls out with it. The per-platform spellings are
deliberately different, each the convention of the place it lands:

| Channel | Executable | Why that spelling |
| --- | --- | --- |
| Linux — tarball, snap, AppImage | `bibletext` | a command in the reader's PATH |
| Windows — zip, Microsoft Store | `BibleText.exe` | Task Manager's Description column and the AppxManifest |
| macOS — direct download, Mac App Store | `BibleText` | `Contents/MacOS/`, as `Safari.app/Contents/MacOS/Safari` reads |

Verified rather than assumed: a locally packaged bundle now carries
`CFBundleExecutable: BibleText` with `Contents/MacOS/BibleText`, and the running
process reports `BibleText` to `ps` where it read `desktop` before. On an arm64
Linux machine the renamed tree still packages a tarball containing
`usr/local/bin/bibletext`, and `scripts/build-appimage.sh` still produces an
AppImage whose desktop entry reads `Exec=bibletext %u`.

**What the macOS rename does NOT disturb**, each keyed on the bundle id
`uk.co.bibletext` or on the `NewWithID` identifier rather than on the executable
name: the App Sandbox container, the Fyne preferences directory holding notes
and reading position, the Keychain service, Universal Links, and the
code-signing identifier. The app requests no TCC-gated capability, and the Store
update path replaces the bundle wholesale. An earlier draft of this entry
deferred the macOS half on the grounds that the binary is "signed, notarised and
shipped through the App Store"; two thirds of that was wrong and the record
should not keep it — there is **no notarisation step anywhere in this
repository** (`docs/MAC_APP_STORE.md` says the Store was chosen over plain
notarisation), and the direct-download zip is not signed either. The real cost
was a signing re-run on one channel.

The one residual macOS hazard is a reader who unzips the direct download *over*
an existing `BibleText.app` instead of letting Finder replace it: they end up
with both `desktop` and `BibleText` inside `Contents/MacOS`. The bundle still
launches, because `CFBundleExecutable` names the new one, so this is untidy
rather than broken.

**Upgrade wrinkle on Linux, deliberately left to a release note.** A reader who
ran `sudo make install` from any tarball up to 1.2.12 keeps an orphaned
`/usr/local/bin/desktop` (or `/usr/bin/desktop`); the next package's `make
uninstall` removes only the new name. An `-rm …/bin/desktop` line in the
uninstall target was considered and REJECTED: it would delete a file we can no
longer prove is ours, which is the exact PATH collision this entry is about. It
is also the narrowest possible reach — it fires only for someone who installed
≤1.2.12, *then* installs the next tarball, *then* runs `make uninstall` from the
new package, while everyone who simply upgrades keeps the orphan regardless. The
numbers support the light touch: **58 tarball downloads across all 18
releases**, and until 19 September 2026 `make install` exited non-zero on the
icon line, so the population that completed an install is smaller still. The
orphan is inert — the new install overwrites the desktop entry, so nothing
launches it; the only live harm is someone typing `desktop` from shell history
and silently running 1.2.12 forever.

**DONE 20 September 2026:** the line is in the release-notes string in
`.github/workflows/release.yml`, so the first release carrying the rename tells
a reader how to remove the binary the new package's `make uninstall` no longer
names. It is a migration note, not a standing one — the comment beside it says
to drop it once the releases that shipped `desktop` are far enough back.

**Action before the next tag: the Fyne apps directory still prints the OLD
command.** Verified live on 19 September 2026 — apps.fyne.io/apps/uk.co.bibletext/
prints `fyne install github.com/cubancorona/bibletext/cmd/desktop@latest`. That
works TODAY, because `fyne install` resolves `@latest` to the newest v-tag and
shallow-clones it, and v1.2.12 still contains `cmd/desktop`. **It breaks the
moment the next tag is cut**, and it is the install route the README points
readers at. The listing lives outside this repository, so nothing here can fix
it and no check can catch it — it has to be updated by hand when the rename
ships. The same applies to any other external page that prints the command.

**Still open, lower priority: iOS ships `CFBundleExecutable: main`.** The same
class of defect with a different generic name, but it surfaces only in crash
reports and Xcode Organizer, nothing lands in a PATH, and
`scripts/release-ios.sh` passes no `--executable`. Read off the packaging line,
not observed on a built bundle.

## Verse attribution: three findings from measuring it, 21 September 2026

Released in 1.2.20, 13 October 2026: all three findings and the test
litter. What they left open is planned in entries of their own — the
selection's place in its verse ("Where the app re-finds the reader's
position from text", P1) and the digit a drag begun inside a verse number
shares, both fixed for 1.2.22; the dash end repair and the BSB heading that
opens as its verse does — see docs/ROADMAP.md.

Prompted by a plain question — does the app really work out verse numbers by
matching text, and what about verses that repeat? The short answer is that
position is authoritative and the matching is a fenced fallback, and the
measurements bear that out: across **34,917 sampled drags over all 89 real WEB
gospel chapters**, a drag that begins inside a verse and carries an honest span
was cited correctly **every time**. Psalm 136's refrain — identical in all 26
verses — is attributed correctly in all 26 with a span. No shipped pane
dispatches a zero span: macOS, iOS, Android, the styled Windows/Linux pane and
the verse-of-the-day card all supply one, and the only zero-span dispatcher is
the legacy Fyne Entry pane, which is dead unless `styledPaneEnabledOnPlatform`
is flipped back. (Zero-span attribution on the same corpus is wrong 0.34% of the
time, which is what the span is buying.) Since 13 October 2026 the legacy pane
hands over a span as well ("Where the app re-finds the reader's position from
text", G); what still hands over none is the plain-text page the Mac, iPhone
and iPad show when a chapter fails to import, and a whitespace-only selection
on the styled pane.

Three things did turn up.

### 1. A drag that opens on the previous verse's punctuation can cite the wrong verse — DONE 6 October 2026, for 1.2.20

Fixed in `normalizeShareSelectionProse` (`share.go`), which the share, the
link, the note and the cross-references all resolve a selection through, so
all six platforms take it. Two changes, both measured by the spanned sweep
(`TestSpannedBoundaryDragSweep`, `share_cut_sweep_test.go`), which drags
across every verse boundary of the 89 embedded WEB gospel chapters with the
verse numbers in the selection and the span the panes report: 81,333 drags,
hermetic, committed first with the rates below as the before.

- **Once placed, a selection is never searched for again.** Each repair only
  cuts it shorter, so where the shorter text stands follows from the cut. This
  is the slide described below: 974 of the sweep's 59,199 drags that open on a
  mark ending the previous verse were cited from an earlier copy (one in seven
  at one word, 2.8% at two, none from ten).
- **A selection that carries a verse number is placed where the number says**
  (`stripVerseMarkersAnchored`), before the span's region is searched. The
  sweep found the same slide in the first search, with no repair involved:
  "this world. 24 I" in John 8, whose verse 23 says "this world. I" twice, was
  placed at the copy inside verse 23 and cited 8:23 alone. Five of the 11,067
  drags that begin on the previous verse's last word did this. The number's
  place is taken only inside the span's verses: a drag begun on the second
  digit of Mark 7:29's number hands over "9 He said to", which the strip reads
  as verse 9's number because verse 9 opens with the same words, and placed
  there it was cited 7:9. The sweep's `digit` family, drags begun on a later
  digit of a verse's number, had 141 of its 8,931 drags cited from the verse
  the digit names until the span had to agree, and has none since
  (`TestADragBegunOnAVerseNumbersLaterDigitStaysInItsSpan`).

After it every drag in the sweep is cited from the verses its words came from
and placed inside the drag, but for the digit drags the pipeline cannot place,
which are shared as they came and cited by their span (below). A wider scratch
sweep compared it with 1.2.19's code: 1,602,762 spanned drags, at every third
boundary, from every character from 30 bytes before the verse's number
through the number, to every word end up to 45 bytes into the verse. No drag
that was right changed in any way. 6,253 that were placed outside the drag
and cited the wrong verses cite the right ones; 21,099 placed outside the
drag with the right citation, and 6,096 that fell back to the legacy path,
are placed where they were dragged; 146 that fell back are placed outside the
drag, the citation the same. 699 drags that hold no whole word of verse text
now fall back verbatim, as a drag of part of one word already did
(`TestShareSelectionSubWordFragment`): a piece of one word, "ou," out of
"you,", or pieces of two either side of a mark or a verse number, "an, 38
th". 695 of them had been placed on another copy of a word outside the drag,
the citation the same, and four cited the wrong verse; they were junk before
and are junk now. A drag whose words have no verse number before them, none
at all or only one it stopped on ("above all. 32" in John 3, whose verse 31
says "above all" twice), is still placed at the first copy of its words in
the span's verses, which can be an earlier copy inside its own verse (5,534
of the wider sweep's drags); the citation is the same, but the completion and
the line-break restore read that copy's surroundings. The panes report the
verse, not the place in it.

**Corrected, 6 October 2026.** With nothing searched for again,
the one search has to find the right copy, and it took the first copy of any
kind. Where that copy stood inside a longer word, the end repair cut the
selection back to the words before it: the WEB's "the wicked" in Psalm 7:9,
which has "the wickedness" first, was shared as "the", with the span 7:9 and
without. With no span the verse of that copy was cited as well: "in you" in
Psalm 5:11 was shared as "in", cited 5:5. Before this change a repair searched
again, failed, and the selection fell back to the legacy path, which shared it
whole under the chapter; so on a page with no span (the plain-text fallback of
the Mac and the iPhone and iPad, and the dead legacy pane) the cut was new
with `fa77eb301`, and with a span it was already in 1.2.19. The search now
takes the first copy that splits no word (`wordAlignedIndex`, as the
psalm-title rule in item 2 already did), in the span's verses, or in the
chapter when there is no span, and a copy that splits a word only where there
is no other, as a drag begun or ended inside a word needs. Over every phrase
of two, three and five words in every verse of the WEB, the BSB and the WEBC,
each with the span a pane reports and without
(`TestEveryPhraseInAVerseSharesItsOwnWordsOverLocalCaches_LocalOnly`, local
and opt-in), 12,161, 9,339 and 13,855 had shared other words than those
selected, or fallen back; 0, 4 and 0 do now. The BSB's four are two phrases
that end on a dash as their verse does, where the verse has the same words
earlier with a word straight after the dash ("herself—or" in Numbers 5:14):
the end repair reads the letter after the dash as the rest of a word, as it
did before. `TestEveryPhraseInAVerseSharesItsOwnWords` holds the same over the
embedded Gospels in CI, 214,115 phrases each way, of which 59 with the span
and 1,247 without had been wrong. Every drag across a verse boundary, from one
to three words before the number to one to three after it, with the span and
without, answers as before, byte for byte: 1,689,252 drags over the three
translations.

With a span, a selection the span's verses do not hold is found in the
chapter at its first copy of any kind, as before, but for a drag run on into
the heading below them (corrected again, below). Taking the chapter's first
whole copy instead, as the change first did, moved such selections to verses
they never touched: a drag from the end of a verse into the first letters of
the heading below, whose copy in the text runs on into the next verse's first
word (the BSB's "the LORD.\nThe" at the end of 2 Chronicles 20:13, which verse
14 follows with "Then", cited 20:32–33); a verse number selected on its own
that a verse holds as a figure (Genesis 32's "20" cited 32:14, which has the
figure after "200"); and a word of a heading selected on its own (the
"The" of Genesis 1's "The Fourth Day", span 13, cited 1:12). And where a drag
ends on the next verse's number, that verse, whose only part in the drag was
its number, leaves the region when the verses before it hold the rest: "in
Christ 9", begun two letters into "gain" at the end of the WEB's Philippians
3:8, had been quoted from verse 9's own "in Christ" and cited 3:9, and the
BSB's "he said.\n57", begun inside "she" in Luke 22:56, cited 22:57
(`TestASelectionItsVersesDoNotHoldIsNotMovedToAWholeCopyElsewhere`,
`TestADragEndedOnTheNextVersesNumberIsNotPlacedInThatVerse`). Over a sweep of
the page models of every pane, of drags inside each verse, across each verse
boundary, from and to each verse number and around each heading (40,139,729
selections in the WEB, 38,654,375 in the BSB and 45,677,198 in the WEBC, each
with the span its pane reports), those two corrections change 1,186 of the
BSB's answers, 44 of the WEB's and 60 of the WEBC's, none of them drags inside
one verse's words: a number or a heading selected on its own cites its own
verse again, as before the change, and the drag ending on a number stays in the
drag. Against the search before the change, the only answers of that sweep
that move away from the reader's copy are drags begun or ended inside a word
(below); 48 of the BSB's drags ending on a verse number, 60 of the WEB's and
64 of the WEBC's, counted on both page models, are now placed in the drag or
cited by its verses, where they fell back to the legacy path or cited another
verse.

The cost is in drags begun or ended inside a word. The text alone cannot tell
such a drag from a phrase whose first copy stands inside a longer word, so
where the drag's letters also stand as whole words in the span's verses it is
quoted from that whole copy, which need not hold the reader's words; the
citation stays the same verse. "leavened bread shall", begun two letters into
"Unleavened" in the WEB's Exodus 13:7, quotes "leavened bread shall" from
later in the verse, where it had quoted "bread shall"; the BSB's "righteous
continue", begun inside "unrighteous" in Revelation 22:11, quotes "righteous
continue" for "continue"
(`TestADragBegunOrEndedInsideAWordIsQuotedFromAWholeCopyInItsVerse`). Over
every phrase of one, two, three, four, six and ten words in every verse, begun
one or two letters into its first word, ended one or two short of its last, or
both, with the span of its verse, 938 of the WEB's 25,479,806 such drags, 683
of the BSB's 24,234,120 and 1,043 of the WEBC's 28,996,684 are now quoted from
a copy other than the reader's, 839, 615 and 939 of them with other words.
With no span the whole copy is looked for in the chapter, and the citation
moves with it. Taking drags of three words begun one letter into the first
word or ended one letter short of the last, where that word has three letters
or more, over every verse of each translation (991,511 in the WEB, 965,485 in
the BSB and 1,130,541 in the WEBC, each with the span and without): with the
span 31, 15 and 33 quote more letters than before, citing the same verse; with
no span 311, 202 and 338 that cited their own verse now cite another (the
WEB's "from the tree", ended one letter short of "trees" in Genesis 3:2, cited
3:11). Telling the reader's copy from another needs the selection's place in
its verse, which every pane has (the Apple panes' verse index, `BtBridge`'s
verse starts, the styled pane's runs) and none hands over; whether to hand it
over is open. With no span a double-click is placed the same way, at the first
copy of its word that splits no word: in the WEB's titled psalms, 2,633 of
29,618 double-clicks, on words the psalm first has inside a longer word, were
shared as they came and cited to the chapter, and each is now cited by its
first whole copy, the verse clicked for 1,158 and an earlier copy's for 1,475
(the BSB: 1,118 and 1,307 of 2,425). That is the rule for any selection with
no span, and whether a selection with no span whose words stand more than once
should cite the chapter instead is open.

**Corrected again, 6 October 2026.** The first copy of any kind is not the
reader's either for a drag from a verse's last words run on past its full stop
into the heading drawn below it, which the Apple and Android panes span to the
verse above. Placed at that copy and kept, such a drag was cited from another
verse with the same words: the BSB's "gold.\nT" at the end of 2 Chronicles
3:4, spanned 4 to 4, was found running from verse 8's "fine gold." into verse
9, cut back by the end repair to verse 8's "gold." and cited 3:8; Philippians
1:2's "Christ.\nT" was cited 1:27, 2 Chronicles 36:14's "Jerusalem.\nThe"
36:19, and the WEB's Psalm 119:64 "statutes.\nT" 119:22. 1.2.19 cited all four
from their own verse, its end repair searching the span's verses again for
what it cut. Where the span's verses do not hold the selection, the front of
the heading drawn below the last of them now comes off the selection's end
first, or, where headings are stacked there, those before it whole and then
its front, and what is left is placed at the end of that verse when it is how
the verse ends (`verseEndBeforeHeading`). The end is read from the selection
before a trailing verse number comes off, so a drag through a heading named
"Saying 10" to the number below is not taken for a drag into the next heading,
"Saying 11". Anything else is searched for as before, as is a selection the
verse holds itself, since a drag inside the verse can make the same text with
the same span. The 2 Chronicles 20:13 drag above, "the LORD.\nThe", is now
placed this way, at the same place and with the same citation. Over a sweep of
the page models of every pane (45,889,289 selections in the WEB, 44,315,627 in
the BSB and 52,192,485 in the WEBC, each with the Apple and Android span, the
styled pane's span and none), 436 answers change, every one a drag from a
verse's last one or two words into the heading below with the Apple and
Android span, and every one is now cited from the verse its words came from
and quotes only them: 4 in the WEB, 426 in the BSB and 6 in the WEBC. 182 of
them (4, 172 and 6) had been answered right by 1.2.19; the other 254, all in
the BSB, had been cited from another verse or had shared heading words as
Scripture in 1.2.19 as well. A further 61 BSB drags of that shape keep their
quotation and citation and are now placed at the end of the verse, not at an
earlier copy in it. No other answer changes, with any span or none
(`TestADragRunOnIntoTheHeadingBelowCitesItsOwnVerse`,
`TestADragRunOnIntoTheHeadingBelowSweep`,
`TestADragThroughAHeadingToTheNumberBelowIsNotTakenForTheNextHeading`). With
no span, on the plain-text fallback page, nothing names the verse above the
heading, and such a drag is placed at the chapter's first copy, as any
selection with no span is: of the BSB's 18,396 such drags of one or two words,
41 that 1.2.19 cited from their own verse cite another (Numbers 3:26's
"items.\nT", cited 3:31) and 99 it answered wrong are right, all as before
this correction, and none of the WEB's or the WEBC's is answered worse than in
1.2.19. Whether to place them by the heading as well is open.

The original finding:

The real residual risk, and it is NOT the fallback. A `selSpan` narrows the
search REGION — `[start of verse lo, end of verse hi]` — which is always wider
than the drag itself. The pipeline then runs a chain of repairs (dangling
verse-number trim, mid-word start/end repair, orphan-punctuation strip, heading
strip), and **every one of them re-runs the locate from the start of that
region, first match wins**. So a trim can slide the match backwards into a verse
the reader never touched.

Begin a drag on the full stop ending verse N−1 and the span is honestly
(N−1, N). The orphan punctuation is correctly stripped from the quote — and the
re-locate then matches the earlier copy:

```
drag starts at verse 39's number   span=(39,39)  ->  Matthew 25:39   correct
drag starts on verse 38's '?'      span=(38,39)  ->  Matthew 25:38   WRONG
```
(identical shared words in both cases)

Measured on the real WEB gospels, for drags of that shape: 2.63% wrong at two
words, 1.08% at three, 0.27% at four, 0.00% by twelve. It concentrates exactly
where repetition does — the Beatitudes, Matthew 23's "He who swears by",
Matthew 25's "When did we see". Overall about 0.12% of boundary drags.

The fix is small and local: after a repair, prefer the match nearest the
previously-located index, or re-anchor the region to the SELECTION's own extent
rather than the span verses' full extent. Not done here because it is a
behaviour change to the attribution core and wants its own pass with the sweep
harness above as the before/after measure.

### 2. A Psalm superscription can be shared as scripture — DONE 6 October 2026, for 1.2.20; not reachable on the panes that draw it

**Corrected, 6 October 2026.** The finding below was made in Go
alone, and the panes it names never hand Go such a drag. Since the titles were
first drawn (27 August 2026, `d2385d5d6`), the iPhone, iPad, Mac and Android
panes start their app verbs at verse 1's number (`btIOSContentStart` in
`reading_ios.go`; `hbScriptureSelection` and the menu check in
`reading_macos.go`; `contentStart` in `BtBridge.java`'s action handler). A
selection wholly inside the title gets Copy and Look Up only, with no Share,
Note or Cross-references, and a drag from the title into the psalm is cut at
verse 1's number before Go sees it. So no shipped pane shared a title as
Scripture from a chapter it imported, and decision 5 below (a title selected
on its own cites verse 1 with no quotation) is never seen there: a reader gets
no share verbs on a title at all, which may be what is wanted. The pages that
do hand a title over are the plain-text fallbacks the Mac and the iPhone and
iPad show when a chapter fails to import as HTML (`gMacContentStart` is 0 on
the Mac's, and the iPhone and iPad's empties its verse index, so
`btIOSContentStart` is 0 as well; no selection on either has a span), with the
dead legacy pane. That is what the change below serves. With no span, a
selection inside the title was answered as the title, and so were the same
letters selected in a later verse ("Yahweh" in Psalm 18, whose title names
him, or "is", which that title has inside "his" and "this"): no quotation,
verse 1, where before they were quoted from their first copy in the psalm.
Double-clicking every word of every titled psalm with no span, 29,618 words in
the WEB and 28,670 in the BSB, 2,581 and 2,817 quoted nothing at `4db93bf64`,
and 758 and 727 at `69d176b7c`, which looked in the psalm only for whole words
of the title; none did at `1db51a02b`, and none do now, the psalm being
searched first (below). Each is quoted from the first copy of its word that
splits no word, as any selection with no span is placed, so of the WEB's 758,
321 now cite the verse clicked and 437 an earlier copy's verse, and of the
BSB's 727, 317 and 410. On every pane, the change's visible effect is on
headings, as the paragraph on headings below says, and that wider effect was
not part of the decision.

**Corrected again, 6 October 2026.** The fallback's page ran its paragraphs
together. Its tag strip took out every `<p>` and put nothing in its place, so
the title's last word and verse 1's number were one word ("his son.1 LORD"),
the number could not confirm the title, and every drag from a title into the
psalm on that page was shared whole, the title's words as Scripture under the
chapter. Of the drags from each character of a title to each boundary up to 30
bytes into verse 1's text, that was all 176,571 in the WEB's titled psalms and
all 305,428 in the BSB's, at every tree back to `1db51a02b`. The page also
opened with the stylesheet's text, which the strip leaves standing, and showed
a straight quotation mark as its entity. Both converters
(`bibleTextMacPlainFromHTML` in `reading_macos.go`, `bibleTextPlainFromHTML`
in `reading_ios.go`) now end each paragraph with a line break, drop the
`<head>`, and read `&quot;`. On the page they now write, none of the WEB's
182,112 such drags is shared so. In the BSB, 11,021 of 316,023 still are, in
55 psalms whose heading under the title opens as verse 1 does ("The LORD Is My
Shepherd", "The LORD is my shepherd"): when the drag stops in or at the end of
those first letters, the heading repair, having cut the heading from the front
of the selection, takes them at its end for the start of the same heading
(`trimTrailingHeading`), and what is left cannot be placed. That shape is the
heading repair's, not the title's. A drag from any heading through its verse's
number to such letters ("He Restores 3 He") is shared whole on every pane,
with a span or without, as at `1db51a02b`, and it is left with the headings'
decision below. With the span the panes report, of the drags from each
character of a heading through its verse's number to the end of that verse's
first, second or third word, the BSB shares 7,587 of 218,070 as they came,
7,468 of them whole, under 275 headings, against 7,818 at `1db51a02b`; the
WEBC 6 of 2,124 and the WEB none of 297. The letters need not begin the same
heading: Genesis 9's "Noah’s Shame and Canaan’s Curse 18 The" ends in the
first word of the heading above verse 1, "The Covenant of the Rainbow". A
remedy: once a heading's end has been cut from the front of a selection, no
heading's start is cut from its end.

What was done: fixed in `normalizeShareSelectionProse` and `stripHeadings`
(`share.go`), which every verb resolves a selection through. A drag from the
title into the psalm shares the psalm's words alone, cited by the verses they
span: the title, or its end, comes off when verse 1's number confirms it, and
with a heading between them the heading repair takes both. A title selected
on its own, or run on to verse 1's number or into part of its first word,
quotes nothing and cites verse 1, as decided. A selection with no span inside
the title is looked for in the psalm first, at a copy that splits no word
there, and quoted from the first such copy, cited by its verse, as any
selection with no span is placed at its first copy: nothing tells the
title's letters from the same letters double-clicked in a verse, whether the
title has them as a word ("Yahweh") or inside a longer one ("is" in "his").
An apostrophe between letters is part of its word, so "’s" or "s" selected
from "Saul’s" is not found in "king’s". Verse numbers the psalm's words do
not hold are answered as a verse number is, by the verse each labels ("12"
under the BSB's title of Psalm 60, which says "12,000";
`TestAVerseNumberATitleAlsoHoldsCitesItsVerse`), and so is a digit taken
from inside that figure ("2" cites 60:2), as verse 2's number double-clicked
on that page would be. Only the title whole, a phrase only the title has,
letters the psalm has only inside longer words, or what holds nothing
quotable, a lone full stop, is answered as the title.
`TestPsalmTitleWordsWithNoSpanSweep` (`share_cut_sweep_test.go`) selects
every piece of an invented title that shares words with its psalm, 13,791
selections. The psalm holds 44 of them as words, 3 of those inside longer
words of the title ("his", "is", "word"), and all 44 are quoted from their
first such copy. At `4db93bf64` all 44 were cited as verse 1 with nothing
quoted; at `69d176b7c` the 3 still were, and "’s" was quoted as the "s" of
"king’s"; at `1db51a02b`, before the title was answered at all, 38 were
quoted from that copy, and "the", "he", "all" and "is", whose first letters
in the psalm stand inside a longer word, were shared verbatim under the
chapter. Over every piece of every title in the WEB, the BSB and the WEBC
with no span, against `69d176b7c`: 1,116 pieces cited as verse 1 are now
quoted from the psalm; 30 that the psalm has only inside a possessive or a
contraction ("’s", "’t", "David" against "David’s"), which had been quoted
from inside that word, are now cited as verse 1; 4 move to a later copy that
splits no word; and 3 pieces holding a verse number, "12", " 12" and "2", 2
numbers in all, cite their verse. Counted as drags, from every character of a
title to every boundary with no span, 650 of the WEB's 225,628 and 607 of the
BSB's 205,779 moved from verse 1 to a quotation from the psalm, each the
letters selected and each inside a longer word of the title ("is" from
"his"), and 30 of the WEB's moved back to verse 1. A heading the BSB sets
under a title is answered the same for a piece the title also holds: 187
drags inside such headings moved from verse 1 to a quotation from the psalm
("in", "a"), as a piece of a heading with no span is answered elsewhere, and
with the search of item 1, 188 more, which the psalm first holds inside a
longer word and which were shared as they came, are quoted from its first
whole copy. The title rule changes no answer with a span. The link's verses,
which a sent note is kept against, agree.
`TestPsalmTitleDragSweep` (`share_cut_sweep_test.go`) drags from every
character of a title to every boundary through a heading, verse 1 and into
verse 2, 14,124 drags on the page the panes draw, and as many again on the
page the fallback writes, which
`TestTheFallbackPageAnswersADragFromATitleAsThePanesWould`
(`reading_native_fallback_page_test.go`) builds by running each converter's
own steps on the chapter's HTML; `share_heading_selection_test.go` pins the
shapes.
The same repair answers a heading run on through its verse's number into part
of that verse's first word, which quoted the heading and cited the verse above
with it: it now quotes nothing and cites the verse below, as a heading
selected on its own does. And the heading repair now drops a verse number the
drag stopped on, as the first pass does, so a drag from a verse through the
heading under it to the next verse's number shares the verse alone, cited by
it, where it shared the heading and the number as Scripture under both verses
(`TestAHeadingRepairDropsTheNumberTheDragStoppedOn`); and a heading with its
verse's number and nothing after is answered as the heading, where the bare
number was looked for in the verses' words and could be quoted and cited from
wherever the same figure stands (`TestAHeadingAndItsVerseNumberAreNotLookedForInTheText`).
The review swept every drag from Acts 10:43 through a heading into verse 45 of
the heading fixture, 19,110 drags, against 1.2.19's code: 410 change, with
item 1's fix, 409 of them from the legacy path's verbatim share, and 352 had
quoted the heading's words as Scripture. These are reached on every pane, the
only part of this change that is, and keep the existing rule that a heading
is never quoted; they await the owner's word as part of 1.2.20, the headings'
own pass being the next major's (docs/ROADMAP.md, next major row 5).

Also fixed, 6 October 2026: on the fallback's page a heading
further down, selected whole, was shared as Scripture under the chapter. The
heading-only answer asks for a span naming the verse above the heading, which
is what tells a heading from the same words in a verse, and that page never
has one. The old page reached this only by an exact drag, its headings run
into the verses beside them; the new one sets each heading on a line of its
own, so selecting the paragraph takes it. Over the caches, each heading alone,
with the line break after it, and run on to its verse's number: 5,615 of the
BSB's 9,273 such selections (3,091 headings) were shared as they came, 6 cited
a verse that no heading of those words stands above, and 5 quoted other words,
the heading repair having cut the next heading's first word from the end
("Jacob and" for Genesis 25's "Jacob and Esau", whose next heading opens
"Esau"); 63 of the WEB's 66 and 69 of the WEBC's 81 were shared as they came.
Such a selection is now answered before the search as a heading selected on
its own is, the verse below and no quotation, unless the chapter has the
heading's words as words, which are then quoted as any selection with no span
is (`wholeHeadingVerse` in `normalizeShareSelectionProse`); with more than one
heading of those words, the first is taken. None of the three shapes is now
answered otherwise in any of the three translations
(`TestEveryHeadingSelectedWholeWithNoSpanOverLocalCaches_LocalOnly`, local and
opt-in; `TestAHeadingSelectedWholeWithNoSpanQuotesNothing`). A piece of a
heading further down with no span is not answered so, since the same piece may
stand in another heading, though a piece that is another heading whole is
answered as that heading ("The Parable of the Weeds", the start of Matthew
13's "The Parable of the Weeds Explained", cites 13:24). Any other piece is
placed by item 1's search, as any selection with no span is, at the first copy
of its words that splits no word. Of the BSB's 29,465 words, prefixes and
suffixes of headings selected with no span, 1,845 that were shared as they
came, the heading's words as Scripture under the chapter, are now quoted from
that copy and cited by its verse; and 18 quote fewer words than they hold, the
heading repair having cut from their end what the chapter does not have, as
the start of another heading ("Feast" of "The Feast", taken for the start of
Leviticus 23's "Feasts and Sabbaths"), and the rest being quoted from its first
whole copy ("The", 23:5). 7 of those 18 had quoted nothing, the first copy of
"The" standing inside the "Then" that opens a verse, which answered them as a
heading under that verse (23:1, no heading of those words standing above it);
the other 11 had been shared as they came. The WEBC has 13 of its 306 pieces
the first way. With a span no piece moves out of the span's verses: 620 of the
BSB's 66,091 and 3 of the WEBC's 852, which the legacy path shared verbatim
under the span, are now placed in those verses, quoting the same words; with a
span of one verse the citation is the same, and with two it names the one that
holds the words. This applies the rule that a heading is never quoted to a
shape no span reached, on the fallback's page alone (and the dead legacy
pane), and goes with the change to headings if the decision on headings above
limits that change to titles. What a piece of a heading with no span should
share is that decision's too.

Two shapes stay as they were, being the headings' rather than the title's:
with no span, the cross-references resolve a title selected on its own by
matching words, as they do a heading at the top of a chapter; and a piece of a
word inside a heading that is also in the psalm is found in the psalm. A
verse's number with only part of its first word ("39 Wh") still shares both,
title or no title: the legacy path's verbatim fragment.

The original finding:

`stripHeadings` and `headingOnlySelectionVerse` consult `Bible.Headings` and
never `Bible.Superscriptions`. On iOS/macOS/Android the title lives in the same
selectable text storage as the verses (not on the styled pane, which draws it
outside `p.lay.Text`), so a drag from the title into verse 1 shares
`"A Psalm by David, when he fled from Absalom his son. Yahweh, ..."` cited as
`Psalms 3:1` — editorial matter quoted as the text, under a verse reference.
The heading path deliberately returns reference-only for exactly this case;
superscriptions were simply missed when it was written. Related to the open
"quoting, notes and links" heading pass.

**Decided 6 October 2026** — a psalm title selected on its own: cite
verse 1 with no quotation, as a heading does.

### 3. The omitted-verse strip rests on an untested assumption

`outboundText` strips a `[digits]` token by SHAPE, justified by a comment
claiming `outbound_text_test.go` "walks the feeds to keep that true". It does
not, and there is no sign it ever did — the file is 59 lines of hand-written
cases. The comment has been corrected to say so. The exposure is narrow but
real, and it is not only cosmetic: the strip changes the BYTE LENGTH, so a
publisher verse containing a `[12]`-shaped token would both lose it and shift
the offsets the share pipeline matches on. A superscript digit or an em/en space
in publisher text does the same. Wants a test that walks the shipped
translations.

**DONE 6 October 2026, for 1.2.20** (`outbound_text_walk_test.go`). The walk
reads every verse, heading and Psalm title and reports each one
`outboundText` would change, by reference and by kind: a bracketed number,
a superscript digit, a no-break space, an em or en space, a small capital.
It never quotes the text, since the NKJV is licensed.
`TestTheEmbeddedGospelsPassThroughTheOutboundCleanerUnchanged` walks the
embedded WEB Gospels in every run, CI included. Its control plants `[12]` in
John 3:16 and a superscript digit in 3:17 of a copy and requires both
reported. `TestEveryDownloadedTranslationPassesThroughTheOutboundCleanerUnchanged`
walks each translation downloaded on the machine running the suite, through
`realCachePath`, and skips where there is none, which is every CI runner.
On the machine it was written on, the WEB (31,236 texts), the BSB (34,293)
and the WEB Catholic (35,522) passed unchanged; no NKJV was downloaded there.
On 6 October 2026 the NKJV, fetched through the app's own API.Bible path into
a cache outside the repository and deleted afterwards, passed too (34,094
texts), with the other three again. The comment in `outbound_text.go` now
points at the walk instead of saying the belief is untested.

### Also: committed test litter

`zz_probe_refute_test.go` is tracked, from commit `a0f34302d`. It looks like a
throwaway probe someone committed by accident. Left alone rather than deleted
on someone else's behalf.

**Decided 6 October 2026** — the oddly named probe test file: rename it as
a proper saved-note test. It checks that an old note's record round-trips
byte for byte through the current codec, which is worth keeping under a
name that says so.

**Done 6 October 2026.** The file is `notes_record_compat_test.go`. The probe
only logged the old record's bytes beside the new and checked that the later
build's field survived, so it would have passed a codec that changed the
bytes. `TestAnOldSavedNoteRoundTripsByteForByte` now requires the bytes to be
identical: emitting `"ar"` when empty, which writes `"ar":null` into every
old record, fails it and passed the probe. `TestASavedNotesAnchorRunsRoundTrip`
pins where the runs are written, that they stay out of the unknown fields,
and that a second pass writes the same bytes. The second probe, which only
logged what hostile anchor spellings parse to, is
`TestAMalformedAnchorSpellingGivesNoRuns`: a malformed spelling gives no
runs, a well-formed one a run per verse, and no run starts below verse 1 or
ends before it starts.

## A drag begun inside a verse number shares the digit — found 6 October 2026; FIXED with the lead 13 October 2026, for 1.2.22

Released in 1.2.22 on 16 October 2026, closed by the selection's place in its verse ("Where
the app re-finds the reader's position from text", P1), at the end of this
entry — see docs/ROADMAP.md. Moved on 7 October 2026 to the release after
1.2.20; 1.2.21 went out without it.

A drag that begins on the second digit of a two-digit verse number hands over
"0 Hezekiah" for Matthew 1:10. Unless the verse whose number is that digit
opens with the same words, the marker strip leaves the digit, nothing
locates, and the legacy path shares the selection as it came: the citation is
right, by the span, and the quotation carries the stray "0". The spanned
sweep's `digit` family (`share_cut_sweep_test.go`) holds the count: 8,769 of
its 8,931 drags, 98%, and the other 162 share the words alone. A verse number
with only a mark before it or only part of the first word after it does the
same: "? 39" on its own shares "? 39", and "39 Wh" shares both. Readers see a
digit, never a wrong verse. A fix would drop a leading run of digits that the
span says is the end of its first verse's number; it is a change to what is
quoted, so it wants its own sweep.

**FIXED with the lead, 13 October 2026, for 1.2.22.** The lead says the drag
began inside the number (it is a proper prefix of the number), so the rest
of the number at the front of the selection is taken off as the page's
apparatus; the rest of a number selected on its own cites its verse with no
quotation. With the lead every pane sends, none of the spanned sweep's 8,931
digit drags shares a stray digit (`TestSpannedBoundaryDragSweepWithLeads`),
and `TestADragBegunInsideAVerseNumberSharesNoDigit` holds WEB Exodus
20:13-14 begun on the "3" of 13 to what the two verses selected whole share.
Without a lead, as from the legacy pane, the 8,769 are as before, pinned by
`TestSpannedBoundaryDragSweep`; the Go-only interim was not needed.

## The web reader cannot render small capitals — DONE 2 October 2026, before it served an edition that uses them

Recorded because it reads like a live defect and is not one, and because the
condition that would make it real is easy to meet by accident.

The reading pane draws the divine name with Unicode small-capital characters
(see `docs/DIVINE_NAME.md`). The site ships the SAME typeface as the app, but a
deliberately tighter subset: `scripts/build-reading-fonts.sh` gives the app
`U+1D00-1D7F` and `U+A700-A7FF` and withholds both from `WEB_RANGES`, with the
reason stated in the script — the site publishes no edition that uses them.
Measured with fontTools: the desktop Junicode carries 25 of 25 small capitals at
409 KB, the web woff2 carries 0 of 25 at 27 KB. So a small capital reaching a
page would render as a fallback glyph or as tofu.

This is a considered decision, not an oversight, and the script already verifies
the APP's subset kept all 25 — they are scattered across three Unicode blocks
and a subset missing one loses letters from the divine name silently.

**It does not reach one today.** The reader publishes `/web/`, `/bsb/` and
`/webc/` only, and all three editions spell the divine name with ordinary
capitals — they carry no small-caps spans at all, because the feed behind them
has no field that could express a character style. Verified on the live site on
21 September 2026: zero small-capital codepoints across those chapters, and
`LORD` in ordinary capitals throughout. The NKJV is the only edition that marks
the name, and it is licensed: `/nkjv/` serves a notice rather than text.

**What would make it real**, any one of:

- publishing an edition whose feed carries `nd`/`sc` spans
- rendering shared-note TEXT on a page rather than a reference (today a note
  link carries a reference and the recipient's own copy supplies the words)
- any future page that echoes text the app drew rather than text a publisher
  sent

The fix, when needed, is two ranges: add `U+1D00-1D7F` and `U+A700-A7FF` to
`WEB_RANGES` in `scripts/build-reading-fonts.sh` and rebuild — the app's own
range list already names both. Cheap, but not free: the web cuts exist to be
small, and that is the trade the current ranges were chosen to make.

Note that Unicode provides no small-capital `x`, so a subset can never be
complete — see `docs/DIVINE_NAME.md`.

**Done before it was needed.** The site began serving an edition that
uses them, the NKJV, on 3 October 2026, and its pages draw the divine name
in small capitals: 6f35bdcab and 63d7556cc (both in v1.2.19) add a
small-capital supplement holding exactly the 25 letters of the app's
table, for the regular, bold and italic cuts, which the NKJV pages'
stylesheet loads by that table's unicode-range (checked on the live site
on 6 October 2026). It took a different route from the two ranges
proposed above: a separate supplement leaves the Regular and Bold web
subsets, whose bytes are hashed into every public-domain page, as they
were, and those pages still carry no small capitals, since none of their
editions marks the name. The other two conditions above, shared-note text
on a page and a page echoing text the app drew, would need the
supplement on those pages too.

## What we ship has largely never been run

Planned for: 1.2.23+ — see docs/ROADMAP.md. Its cheap halves, `UIDeviceFamily`
read back and the preferences writer's marker on every release path, are
FIXED 6 October 2026, for 1.2.20 (end of this entry).

Found by a five-platform survey on 18 September 2026, each platform's claims
then checked by an adversarial reader that refuted 48 of them. The full picture
is docs/PLATFORM_MATRIX.md; these are the items that need a decision rather
than a document.

None of this is about architecture. It is about the gap between "CI is green"
and "somebody ran the thing we give people".

  - **macOS: every row is `builds`.** Nothing we ship has ever been launched
    from the artefact we ship. PARTLY ADDRESSED 19 Sep 2026: the local rehearsal
    now applies the Fyne patches and verifies the atomic preferences writer
    reached the binary, with a control string — measured, a stock build carries
    0 markers and a patched one 1, so the rehearsal really had been running the
    unpatched writer. Three axes remain: arm64-only rather than universal, a
    development certificate rather than the Store one, and no provisioning
    profile.

  - **No shipped Android artefact has been run.** Every recorded run installs
    the DEBUG apk; the universal sideload APK and the Play AAB have never been
    installed or launched, and the whole channel is host-locked to a single
    machine. DONE 19 Sep 2026: the compile half — `scripts/check-android-pane.sh`
    cross-compiles android/arm64 (preferring the pinned NDK r27, falling back to
    whichever NDK is present and saying which it used) and runs in CI beside
    the iOS gate, proven by a type error that `go build ./...` cannot see. What
    remains is RUNNING a shipped artefact.

  - **The iOS store artefact has never run on a device.** The on-device evidence
    in the repository concerns development builds. `UIDeviceFamily` — the single
    property that makes the app universal iPhone+iPad — is never read back out
    of the exported .ipa, though other Info.plist keys are.

  - **The Linux tarball's installer now runs, and fails.** See the entry above:
    `scripts/check-linux-package.sh` exercises it on every packaged tarball and
    the target exits non-zero on the icon line. What remains unproven is
    any workflow, script or doc. And the AppImage's type-2 runtime is never
    executed: the smoke runs `./squashfs-root/AppRun`, the extracted payload.

  - **windows-build.yml has zero runs, ever.** It is registered and
    dispatch-only, so no push has compiled it either.

  - The owner's outstanding Linux hardware pass needs an x86_64 machine or a
    cloud desktop; the arm64 VM explicitly cannot stand in for it.

The cheap wins are probably the compile checks (a `GOOS=android` gate costs
nothing) and launching the packaged macOS artefact rather than a development
rebuild. The expensive one is deciding what "tested" should mean for a store
binary we cannot run in CI at all.

**The two cheap halves — FIXED 6 October 2026, for 1.2.20.**

- *The atomic preferences writer on every release path.* Only
  `release-mac-store.sh` looked for the patched writer's message in what it
  built. `scripts/verify-atomic-prefs.py` now does it everywhere, with the
  same control string. It checks each slice of a universal binary on its
  own and each native library in an .apk or .aab, and fails closed on a
  package with none. It runs after the last compile of every path:
  `release-ios.sh` on the cross-compiled binary, and `build-android.sh
  --release` on the AAB and on the APK. `release-mac-store.sh` runs it on
  both slices, in place of its own loop. `build-windows-exe.sh`, which
  `msstore.yml` builds through, runs it after the packager's rebuild, and
  `check-linux-package.sh` on the tarball's executable. In `release.yml` it
  runs on the Mac's universal binary, on each AppImage's extracted
  executable and on each Windows executable. In `linux-stores.yml` it runs
  on the binary its packages share. It was left out of
  `verify-release-package.sh`, whose regression test
  (`scripts/test-release-key-flow.sh`) runs it on a test binary built
  without the Fyne patches. Built from this tree, `cmd/bibletext` with stock
  Fyne is refused and the same build with `setup-fyne-patch.sh` applied
  passes.
- *`UIDeviceFamily` read back.* `scripts/check-ios-device-family.py` reads
  the key with plistlib and requires exactly iPhone and iPad, or iPhone
  alone for a `BIBLETEXT_IPAD=0` build. `release-ios.sh` runs it on the
  archived app, the bundle the upload path sends, and on the exported .ipa
  before it is published.

Held by `atomic_prefs_release_test.go` and `ios_device_family_test.go`.
`TestEveryReleasePathChecksTheAtomicPreferencesWriter` reads every path and
every compiling workflow job for the check after its last compile.
`TestVerifyAtomicPrefsRefusesAStockWriter` runs the verifier on made files:
a stock binary, one with no control string, a universal binary with one
stock slice, an .aab with one stock library, and an .apk with none.
`TestReleaseIOSReadsItsDeviceFamiliesBack` and `TestCheckIOSDeviceFamily`
cover the read-back. The path checks fail against the previous scripts and
workflows, and each made-file case fails with its rule taken out of the
verifier. Neither check has run on a real store build yet; the first will
be 1.2.20's.

## Headings: quoting, notes and links need one deliberate pass

Planned for: next major, behind `next` — see docs/ROADMAP.md. The Apple
panes' heading fact ("Where the app re-finds the reader's position from
text", P4) goes in a minor release once the first question below, what
selecting a heading means, is answered.

**FIXED 23 September 2026: a heading belonged to the verse above it.** On the
native panes a verse's character range ran from its own number to the next
verse's number, so a publisher's heading standing between two verses belonged
to the verse ABOVE it. A note, a search hit or a link on that verse washed the
heading too (reported on NKJV 2 Corinthians 12:10, whose next heading stands
before v11), the narration lit it, a tap on it counted as a tap on the mark,
and the verse's range reached down past it. It shipped from 1.2.7, when
headings first rendered. A heading now belongs to no verse: iOS records each
verse's end in its verse index (`btIOSBuildVerseIndex`: the first heading
paragraph before the next number) and the heading block's end, so the wash view
leaves a heading inside a multi-verse mark bare and hit-tests only the painted
pieces; macOS clamps `btMacReadAlongRange` and extends `btMacUnwashBreaks`;
Android clamps its narration ranges (`endBeforeHeading`) — its note wash comes
from the markup and was always right. A heading is recognised as a paragraph
holding no verse number whose text is bold at body size, the Apple twin of
Android's `isHeadingParagraph`. The styled pane and the web reader were never
affected: a heading line there carries no runs. Held by
`TestChapterWashCoversExactlyTheMarkedUpCharacters` (the Go model: nine heading
cases, including one asking every verse range directly, since the pixels come
out right from the bare ranges alone), `TestNativeVerseRangesStopAtAHeading`
and `TestHeadingsReachTheNativePanesBold`; every guard mutation-proved, including
the exit that stops the bare-range loop at a range ending exactly where a heading
starts (the narration on the verse above one), which the first guard left free.
The iOS pass finds each verse's paragraph end by scanning that verse's own span,
so it reads the chapter once; asking for the paragraph around every verse
number cost about 5ms per import on a psalm set as one paragraph (measured on
an AppKit port). Seen on the iPadOS simulator with the `headwash` scenario. Still open from the same
work: `btIOSNoteAnchorRange` falls back to the whole highlight range for a note
with no verse, which would restyle a heading inside a multi-verse mark (no note
has no verse today). An omitted verse's gap mark before a verse was washed
by the markup but not by a single-verse mark's native range, a mismatch older
than this fix; 1.2.23 walks each mark in its place in the native panes, so
the native range and the markup agree (AND-4, decision 18).

Four heading defects were found and fixed on 18 September 2026 (selection led
by a heading, a drag begun inside one, a drag that ended in one, and a heading
selected alone). Each was found by pulling the thread of the one before it,
which is the signal that this area has never had a deliberate pass -- only
repairs. What follows is what is still open, or was decided quickly and should
be decided properly.

THE RULE IS ASSERTED IN ONE PLACE AND BROKEN IN OTHERS. `headings_test.go`
states that a heading must never reach sharing, yet two live breaches turned up
in an afternoon: a drag ending inside a heading carried the heading's words into
the quotation, and a heading selected alone quoted itself as scripture. Both are
fixed, but neither was caught by anything. The rule needs enforcing at the point
every quotation is produced, not asserted once and hoped for. Audit every path
that yields quoted text: `prepareShareQuoteIn`, `cleanQuoteTextIn`, the share
card, the note composer's preview, the AI panel's quotation, the web reader.
(The note composer's excerpt is `prepareShareQuote`'s own text, so it is
covered by that entry; `TestAHeadingNeverReachesTheNoteExcerpt`. Building it
found a fifth breach, fixed 27 September 2026: a heading at the top of a
chapter, selected on its own, still quoted itself, because the heading-only
rule asked for a span naming the verse above and there is none. A lone verse
number is now held to the same rule. See "Show the selected words on the Add a
note sheet".)

HEADING STYLES ARE NOT ALL ALIKE, AND THE CODE TREATS THEM AS IF THEY WERE.
`Heading.Style` carries the publisher's own classification -- `s`/`s1`..`s4` for
section heads, `ms` for a major section, `r`/`mr`/`sr` for a parallel-passage
reference line, `qa` for an acrostic letter. `stripHeadings` treats all of them
the same. They are not the same thing to a reader:

  - a reference line is "(Matthew 3:1-12)" -- arguably a citation the reader
    might legitimately want to copy;
  - an acrostic letter is a single Hebrew letter name standing over Psalm 119's
    stanzas, and the heading-only rule would anchor a note on "ALEPH" to the
    stanza's first verse, which may be right, or may be surprising;
  - a major section head spans many chapters' worth of material.

PSALM SUPERSCRIPTIONS ARE A DIFFERENT TYPE ENTIRELY and were not touched.
`Superscription` is its own struct, the timing tables carry verse-0 rows for it,
and nothing in this work asked what happens when one is selected, noted or
shared. It is the nearest neighbour to a heading and behaves differently.

THE HEADING-ONLY DECISION WAS TAKEN QUICKLY. A heading selected alone now
anchors to `Heading.BeforeVerse` and quotes NOTHING -- reference only. The
reasoning: a note anchored to a heading cannot place for a recipient, because
headings are the publisher's editorial matter and a shared link opens the
public-domain web reader whose headings are not the licensed edition's; and
quoting the verse would attribute words the reader did not select. That holds
up, but it was one judgement made in one exchange, and it deserves to be
revisited alongside the style question above rather than standing as settled.

THE NATIVE RULE IS STILL BACKWARD AT THE START. All three native panes resolve
an offset to a verse by the last verse-number run at or before it, so an offset
inside a heading resolves to the verse ABOVE. The fix corrects the attribution
downstream, in the shared normalizer, which is why one change reached macOS, iOS
and Android at once. But the raw span is still wrong at source, and any future
consumer of it inherits the defect. Worth deciding whether the native rule
should look FORWARD when the offset falls in heading matter -- it is the same
asymmetry noted in the arrival work: `hi` uses the inclusive last character and
is right, `lo` uses the same backward rule and is not.

WHAT A READER ACTUALLY WANTS IS THE UNANSWERED QUESTION. Everything above is
mechanism. Nobody has asked what selecting a heading should MEAN: the section it
introduces, the heading as a label, or nothing at all. The answer probably
differs by style, and it decides the rest.

## The Windows audio smoke is probably testing a silent sink — DONE 15 October 2026

Closed 15 October 2026: step 3 ran on the Windows virtual machine with the
Store checks deferred from 1.2.20 (the end of this entry) — see
docs/ROADMAP.md, Later minor releases, row 40.

`windows-audio-smoke.yml` has failed repeatedly at the natural-end step, and
the investigation has been looking for a defect in the app. The likelier
explanation is that the test's central premise does not hold on the machine it
runs on.

Its header says it "plays a REAL narration chapter through the REAL desktop
audio engine (oto/WASAPI) on a real Windows machine", and that "samples are
decoded and submitted to WASAPI, playback position advances in real time". But
a GitHub-hosted Windows runner is a headless VM with NO audio endpoint. oto
v3.4.0 tries WASAPI, then WinMM, and when both report a missing device it falls
back to `nullContext` -- a sink that accepts samples and discards them. Nothing
is submitted to WASAPI because there is nothing to submit to, and a wait for
playback to reach its natural end is then waiting on a clock that is not the
one the test assumes.

The counter-evidence, measured 18 September 2026 on the UTM Windows guest,
which DOES have an audio device: with a chapter playing, `AUDIOSES.DLL` and
`MMDevAPI.dll` are both mapped into the process, so oto opened a real WASAPI
render session there and took no fallback. The same check on a runner would
show neither. So the app is fine and the harness is measuring something else.

What to do, in order:

  1. Make the smoke SAY which backend it got, and fail loudly rather than
     mysteriously when it is the null sink. A test that cannot distinguish
     "audio works" from "there is no audio device" is not evidence either way,
     and right now its failure is being read as the former. The cheap probe is
     the one used on the guest: the process's loaded modules, filtered for
     `audioses|mmdevapi`. Assert the expectation explicitly so a runner without
     a device reports that fact instead of timing out.
  2. Correct the workflow header and docs/PLATFORM_MIMIC.md, which both state
     that this proves the WASAPI backend. On a device-less runner it cannot.
  3. Only then look again at the natural-end path, on a machine that has an
     audio device -- the UTM guest is one, and so is any real Windows box.

Until 1 is done, a red run of this workflow says nothing about whether Windows
audio works, and the repeated failures should not be read as a shipping risk.

**Steps 1 and 2 DONE 22 September 2026** (2e99c6b50). The smoke names the
backend that carried the samples and stops with that diagnosis on the null
sink. Only `AUDIOSES.DLL` mapped counts as proof, since oto maps
`MMDevAPI.dll` and `winmm.dll` while looking for a device and leaves them
mapped when it finds none; `audio_backend_proof.go` holds the decision and
its test. The workflow's header and docs/PLATFORM_MIMIC.md no longer say it
proves WASAPI on a runner. Step 3, the natural end on a machine with an
audio device, is what is left.

**Step 3 DONE 15 October 2026.** On the Windows 11 arm64 virtual machine,
which has an audio device, `TestDesktopAudioEndToEnd` (`-tags
audiosmoke`) ran from a native arm64 test binary built in the guest with
cgo, from `e52273ccd` (1.2.22 as prepared), against the toolkit as
published, without the repository's patches, none of which touches audio.
Run as the desktop user, it named WASAPI as the backend that carried the
samples (`AUDIOSES.DLL` mapped), went through the skip, the pause and the
resume, seeked to the end, and reached the natural end and then idle:
PASS in 4.06 s, exit 0. No control run without an audio device was made
there; that branch, the null sink's diagnosis, is held by
`audio_backend_proof_test.go`. The natural end passes on a machine with
an audio device, which is what step 3 was to show.

## The direct downloads do not register `bibletext:` links

Planned for: stays — see docs/ROADMAP.md.

Three channels hand a shared link back to the browser because nothing has put
a handler in front of the system. The packaged builds are fine: the Linux
tarball installs a desktop entry with `MimeType=x-scheme-handler/bibletext;`,
and the Store MSIX declares `uap3:Protocol`. The downloads people actually
take from the site mostly do not.

| Channel | State | What is missing |
| --- | --- | --- |
| Linux tarball | works | — |
| Microsoft Store MSIX | works | — |
| Snap | works | — |
| **AppImage** | no | nothing installs its desktop entry, and the entry inside says `Exec=bibletext`, a command that is not on `$PATH` |
| **Windows .zip** | no | no `HKCU\Software\Classes\bibletext` registry entry |
| **macOS direct .zip** | no | no `CFBundleURLTypes` in the packaged plist |

The AppImage is the one worth understanding, because the metadata is already
right and still does nothing. `linux/uk.co.bibletext.BibleText.desktop` is
packed inside the image and carries both `Exec=bibletext %u` and the MimeType;
the published file was checked and it is there. It fails twice over. A desktop
entry only becomes a handler once it is in `~/.local/share/applications` and
`update-desktop-database` has run, and running a single file installs nothing
— that is the format's whole point, and the entry inside is read only by
integration daemons most people do not run. And `Exec=bibletext` names a
binary on `$PATH`; an AppImage lives wherever the reader dropped it, with its
real path in `$APPIMAGE`, so even a copied-out entry would point at nothing.

Each fix is a first write outside the app's own directories on its platform,
which is why all three are deferred together rather than one at a time:

- **AppImage** — a Settings switch that, when `$APPIMAGE` is set, writes a
  user desktop entry with `Exec="$APPIMAGE" %u` and the MimeType, then runs
  `update-desktop-database`.
- **Windows .zip** — `HKCU\Software\Classes\bibletext`, gated on not being
  packaged (the MSIX must never race its own manifest), with a Settings
  switch to remove it.
- **macOS direct .zip** — `CFBundleURLTypes` in the packaged plist. The
  cheapest of the three: the delegate's `openURLs` entry point already
  exists, so this is plist work rather than new code.

Settle the stance once and apply it to all three: whether the app registers
itself silently on first run, on a Settings switch, or not at all. A reader
who downloads a zip and expects a link to open in the app has a reasonable
expectation; a reader who finds their URL handlers rearranged without asking
does not.

## The download page, grouped by platform — DONE

**Closed 21 September 2026.** The page is grouped by platform, it offers the
Snap Store under its own heading beside the App Store and Microsoft Store
buttons with the `snap install bibletext` one-liner beneath, and it carries the
note that picks between the Linux downloads.


`docs/index.html` lists desktop downloads as a flat run of buttons: two
macOS, one Windows, and since 1.2.10 two Linux. Flat is still the right shape
at that size, and the canonical alternative — a heading per operating system
with the Linux formats as a sub-list — is what projects with many Linux
formats settle on because Linux always accumulates them.

The trigger is the Snap Store going live. Linux then has three ways in
(tarball, AppImage, Snap) and a flat list stops answering the only question
a reader has, which is which one they want. Restructure then, not before.

Two things to fold in at the same time:

- A line saying which Linux download to take. They are not equivalent: the
  tarball registers `bibletext:` links and the AppImage does not.
- Whether macOS is still two buttons by then (see the universal entry above).

Not worth doing: operating-system detection that picks for the reader. It is
the other canonical pattern and it is what the large projects do, but it
needs a no-JavaScript fallback that lists everything anyway, which is the
page as it stands.

## Age ratings: aim for all ages in every market

Planned for: stays, console work after 1.2.21 clears both stores — see
docs/ROADMAP.md.

The IARC questionnaire answered for the Microsoft Store on 17 September 2026
mirrors the answers Google Play already carries, so the two stores say the same
thing. It yields IARC 3+, PEGI 3, Microsoft 3+, ClassInd L (all ages), CCC 8+,
ESRB Everyone 10+, USK 12 and PCBP 18+ — adults only in Russia, and twelve and
over in Germany, for a reader whose content is Scripture set as text with no
images at all.

The goal is all ages, everywhere. Three answers carry the rating up, and all
three came across from Play: violence against humans in a realistic setting
with realistic reactions and mild or limited blood and gore; references to
sexual activity without descriptive detail; and references to sexual violence.
Each is defensible for the text of Scripture, and each is exactly what a
ratings board reacts to.

Settle one question before changing any of them: what IARC intends by these
questions for a work of literature rather than a game. Several of them, and
their own tooltips, are written for interactive products — "the perpetrator is
able to commit them without penalty", uninvolved characters "visibly vulnerable
to the attacks" — and this app depicts nothing. It renders text; the violence
in it is referred to, never shown. Whether "Violence, Blood, or Gory Images"
is answerable as No at all for a Bible reader is the hinge, because every
raised rating follows from that one Yes.

Doing it properly means re-taking the questionnaire on BOTH stores in the same
sitting, so they do not drift apart, and reading each summary back afterwards.
The Store questionnaire is re-takeable from the submission's Age ratings page;
Play's is under App content -> Content ratings -> Start new questionnaire. A
retake replaces the certificate, so do it while no submission is in review.

## Verse of the day: possible later additions

Planned for: next major, the daily notification behind `next` — see
docs/ROADMAP.md.

The 10 Sep 2026 rework (branch verse-of-day: civil-date key with the device
zone on phones, frozen hash order, passages with alternates for the Catholic
edition's own books, the reading face on the card, Share in the passage's own
chapter, the desktop Escape guard, an offline snapshot of the rotation) left
these undone by decision or by scope. Each is small enough to pick up cold;
none is owed.

- **A page at bibletext.co.uk/today** — one verse, one "Read in context" link,
  no archive. Static, so the page must pick with the same civil-date
  arithmetic in JavaScript and be generated from the same commit as the app,
  or the two drift. Needs the site-root allow-list
  (cmd/websitegen/licensed_exclusion_test.go), its own hashed CSS/JS pair, and
  the Smart App Banner treatment for the link (a same-domain link stays in
  Safari on iPhone). Days of site work. Settle first whether one verse and one
  link fits "not a page".
- **A first-open-of-the-day surfacing** — a small dot on the sparkle until
  opened, at most. A modal on first open was declined: the desktop foreground
  hook fires on every alt-tab back, and an unbidden daily modal is the closest
  thing to an engagement mechanic on the list.
- **Yesterday / tomorrow** — declined as a feed by another name; the reader
  who missed a day has the reading pane. Revisit only if the stance changes.
- **A spoken label for the sparkle** — the toolkit has no accessibility layer,
  so nothing Fyne-side can name the button for VoiceOver or TalkBack. The
  Settings caption is the mitigation. If accessibility is ever taken on, do
  the header as a whole, not this one control.
- **The sparkle in phone landscape** — landscape strips every header control
  but the chapter arrows (reading_fullscreen_row.go), so the card is
  unreachable there. A 22 pt newIconTapButton beside the arrows, plus the
  count-pinning tests. A policy exception, not a bug, so it waits on a
  decision rather than a fix.
- **A feast overlay** (Christmas, Good Friday, Easter as substitutions on the
  civil date) — declined 10 Sep 2026. Hours if ever wanted; Western Easter
  only.

The next pieces that ARE wanted: one opt-in local notification a day, iOS
first, then Android after the Play closed-testing review clears — see the
rework's proposal for the design (calendar triggers carrying the full date,
re-issued on foreground, the passage computed with the fixed day key).

## Android's compact page: a pill under a heading stands down — FIXED 6 October 2026

Fixed for 1.2.20, with the psalm title's gap on the same page (the end of
this entry).

The one surface left out of the pill rule (below). On Android's compact page
— the book page, which since 24 September 2026 is any pane wide enough for it:
a phone in landscape, a tablet — `Html.fromHtml` puts no blank separator line
between paragraphs, so the heading's tail rides as an ascent-mode `AirSpan` on
the next paragraph's first line and `btPillStackInkTop` has no separator line
to measure; it returns -1 and the pill keeps the band-top placement. Measured
on the API 35 emulator under John 11's "Jesus Comforts Martha and Mary": the
heading-to-verse distance is the phone page's to the pixel (166px), and the
pill sits 7px higher inside it. Centring there needs the ascent-mode air read
back off the span rather than off a line. (Until 24 September the compact
page drew no heading air at all — the bridge read any blank line as the phone
page — so there was no air to centre in.)

CORRECTED 6 October 2026: this entry said a psalm title's gap was a blank
line on this page too, written by the dialect's `<br>`. It was not. The
importer ends a block by adding newlines only until the text ends in as many
as the mode asks for (`HtmlToSpannedConverter.appendNewlines`), one in
COMPACT, and counts a `<br>`'s newline among them, so the one `<br>` before
the title's `</p>` was the block's own end and nothing more: on the book
page the title sat on verse 1 with no gap at all (the API 35 and API 33
emulators, WEB Psalm 102, a phone held sideways).

**FIXED 6 October 2026** (for 1.2.20), both halves.

- The title's gap. The book page's title writes two line breaks
  (`android_chapter_html.go`), so its text is the title, a blank line and
  verse 1, as on the phone page, and the bridge sets that line to
  `TITLE_GAP_EM` on both pages (from API 24: below it the import is LEGACY
  and the bridge collapses every blank line of the book page, the title's
  with them, so there the title still sits on verse 1, as it always has).
  On both emulators the title now stands the
  title gap above verse 1 on the book page. `TestAndroidPsalmTitleKeepsItsGapOnTheBookPage`
  (`android_compact_page_air_test.go`) holds it through a model of the
  importer's newline rule, which `TestTheAndroidImportModelMatchesTheEmulator`
  holds to what the emulator drew before the change.
- The pill. `btPillStackInkTop` reads the air off the span where there is no
  separator line: when the noted paragraph's first character carries ascent
  air (`airInAscentAt`), the stack centres between the line above it, the
  heading's last, and the noted line, as it centres across a separator on
  the phone page; between two of the book page's plain paragraphs, which
  meet with no air, it still stands down. Measured under "Jesus Comforts
  Martha and Mary" from the heading's baseline to the next cap line: on the
  API 35 emulator held sideways, 52 above the pill and 65 below before,
  59 and 58 after, against the phone page's 59 and 59; on the API 33
  emulator held sideways, 34 and 43 before, 38 and 39 after.
  `TestAndroidPillCentresUnderAHeadingOnTheBookPage` holds the bridge to it,
  with controls.

## A collapsed note pill directly under a section heading — DONE 12 Sep 2026

The surfaces disagreed: iOS and the web stood the centering lift down when
the air above was not the page's rhythm, Android centred in the measured
ink, and the styled pane lifted by the paragraph gap even under a heading,
where the air above is only the 0.35em tail. One rule now, the spec's own
doctrine: the stack centres in whatever air the page put above its paragraph
— the paragraph gap, a heading's tail, a psalm title's gap — and stands down
only at 0 or beside an open card. The styled layout records each band's
separator (`noteBand.SepAbove`), iOS reads the previous paragraph's
after-spacing without the rhythm gate, macOS gained the same mirror
(`btMacPillSeparatorLift`), and the web's `.text .sec + .notechip` and
`.text p.pst + .notechip` centre in `--htail` and `--tgap`.

## A non-default translation served from a superseded cache epoch is never refreshed — FIXED 25 September 2026 as D17

`loadVersionFromCacheOnly` serves a superseded-epoch cache by design (an
offline upgrader keeps their Bible), and `triggerFullDownload` upgrades it in
the background — for the DEFAULT translation only (`fullPending` is computed
for it; app.go's refresh targets `defaultVersionID`). A reader whose current
translation is another one, served from an old epoch at startup, keeps the old
decode until they switch away and back (the switch path, `loadVersionData`,
drops a stale cache and refetches). Seen 11 Sep 2026 on a Mac reading the WEB
Catholic edition from an epoch-5 cache four epochs behind. Fix: record that
the current translation was served superseded and refresh it as well; the
version-state tests (docs/VERSION_STATES.md) are the place to pin it.

Closed by `D17` in the version-state change of 25 September, and shipped in
1.2.16: the refresh now owes an upgrade to every public-domain translation
recorded as showing its previous edition (`state.staleVersions`, read by
`owedUpgrades` in app.go), not only the default, and swaps the current edition
in place when it lands. `TestAPreviousEditionIsUpdatedWhileTheAppRuns`,
`TestAStaleVersionUpgradesInPlaceWhenTheCurrentEpochArrives` and
`TestANonDefaultStaleVersionIsNotSilent` hold it; docs/VERSION_STATES.md has
the record. This entry was left open by mistake when that change landed.

## Microsoft Store: from the reserved name to the first submission — DONE

Still open: parts of the three checks of step 5 that need a Windows client,
deferred from 1.2.20 and run in part on 15 October 2026 on the Windows
virtual machine (step 5, below). The firewall prompt passed; the https link
and the browsers' prompts for `bibletext:` have parts still to run — see
docs/ROADMAP.md, Later minor releases, row 40.

**Closed 21 September 2026.** Submission 1 published on 17 September (1.2.10,
x64) and submission 2 was committed on the 21st through `msstore/submit.py`,
carrying 1.2.13.0 x64 **and arm64** — the first ARM package the Store has had.
The reservation stopped being a clock the moment the first one published, and
the OpenGL risk below was answered by ANGLE rather than mitigated: the package
renders through Direct3D and passed certification. The history is kept because
the reasoning — why ANGLE and not Mesa, why the first submission could not use
the API — is the part worth having next time.


The name `BibleText` was reserved on 16 September 2026 (Store ID
9NDCCZH9RB9K; identity in msstore/identity.json) and the package pipeline
exists (.github/workflows/msstore.yml builds the unsigned MSIX and
smoke-installs a signed copy). docs/WINDOWS_STORE_LISTING.md is the listing.
A reservation lapses three months after it is made, so the first submission
is due by mid-December 2026. Left to do, in order:

1. **Windows renders through OpenGL, which Windows does not guarantee.** The
   toolkit draws with desktop OpenGL 2.1, and a Windows machine with no
   graphics driver offers only the generic OpenGL 1.1 from 1996, so the app
   does not start there. Ordinary laptops all have drivers; virtual machines
   and clean server installs do not, and the GitHub runner demonstrably does
   not. Whether Microsoft's certification hosts do is unknown: nothing has
   been submitted, and the claim that they are driverless is an analogy, not
   an observation. Two Store policies bite if they are (10.1.2 fully
   functional, 10.4.2 must start and stay responsive).

   **The route to take is ANGLE, not Mesa** (established 17 Sep 2026 against
   primary sources; the earlier entry here prescribed `SetDllDirectory` and
   was wrong twice over, so it is replaced rather than amended):

   - Windows guarantees Direct3D on a driverless machine, through the
     in-box software rasteriser WARP, and guarantees no useful OpenGL. That
     asymmetry is why Store games are Direct3D and why the apps that do need
     OpenGL ship a translator. Chrome, Firefox, Qt 5 and Krita all bundle
     ANGLE, which turns OpenGL ES into Direct3D; Microsoft's own porting
     guidance names it.
   - The toolkit already has that path: `-tags gles` selects
     `internal/painter/gl/gl_es.go` and an ES 2.0 context. Those same ES
     shaders ship from this repository to iOS and Android every day.
   - So the Windows build switches to the ES path and carries ANGLE's
     `libEGL.dll`, `libGLESv2.dll` and `d3dcompiler_47.dll`, in the Store
     package and the zip alike. One code path: the GPU renders where there
     is one, WARP where there is not. **This deletes the driver detection,
     the registry probe and the policy 10.4.1 message entirely**, rather
     than solving them.
   - Measured: 10.4 MB on disk from a pinned third-party build of ANGLE
     (there are no official binaries), against Mesa's larger bundle.

   **A correction to keep:** a bundled Mesa `opengl32.dll` beside the
   executable *does* override the system copy, because the executable's own
   folder is searched before the system folder and it is not a protected
   name. Our own Store smoke proves it. The static import only defeats the
   `SetDllDirectory` variant. So Mesa beside the exe, inside the MSIX only,
   is a zero-code fallback if ANGLE does not work out, at the cost of
   putting every Store reader on a software or wrapper renderer.

   **The decisive test comes before the work**
   (`.github/workflows/windows-gl-probe.yml`): build with the tag on the
   runner, put ANGLE beside it, launch, and assert on PIXELS rather than on
   the process still being alive, with a control run that deletes the DLLs
   and must fail. A pass that cannot fail proves nothing. Open until that
   runs: whether it links under mingw, whether ANGLE's Direct3D backend
   initialises on WARP in a runner session, and the frame cost.

2. **Screenshots** — DONE 16 Sep 2026: four captured on the runner at
   1600×960 (the window's client area, above the Store's floor) by
   `.github/workflows/windows-screenshots.yml`, committed under
   `docs/screenshots/windows/`, uploaded by hand at submission.
3. **Entra tenant and application** — DONE 16 Sep 2026: tenant
   `bibletext.onmicrosoft.com`, application `bibletext-store-api` with the
   Manager role, credentials in the login Keychain, `scripts/msstore-env.sh`
   and the read side of `msstore/msstore.py` proven against the account. The
   write side (create, upload, commit, poll) is written against the second
   release.
4. **First submission by hand** from the listing document, with the
   publishing hold on; then `msstore/msstore.py` for later releases.
5. **Links** — done and pushed 16 Sep 2026, proven on the runner for the
   `bibletext:` scheme and the single-instance handoff; the web-to-app
   handler (https links through the shell) and the browsers' scheme prompts
   still need a Windows 11 client. The ordered checklist for that sitting,
   and the sideload script that installs the workflow's artifact, are in
   `docs/WINDOWS_STORE_LISTING.md` under "Resuming with a Windows machine".
   Still unproven anywhere: the window coming to the front under XWayland
   on Linux (on that checklist).
   Run on 15 October 2026 on the Windows 11 arm64 virtual machine, with
   1.2.21's arm64 package as submitted to the Store, registered under a
   renamed identity in development mode beside the Store's own install of
   1.2.19, and with its executable run without package identity, as the
   direct download runs. The release's zip itself was not run; its
   executable is built by the same commands as the package's
   (`scripts/build-windows-exe.sh`, held to the release job's by
   `scripts/test-release-key-flow.sh`).
   - The firewall prompt (the checklist's step 8): passed. The executable
     run that way listened on 127.0.0.1 only; no Windows Security prompt
     appeared and no firewall rule was made, where a control, a PowerShell
     listener on every address, raised the prompt. Its handoff passed
     there too: a second launch with a link while the window was
     minimised kept one process and restored the window at the link's
     verse, a cold launch opened at its verse, and
     `single-instance.bibletext.direct.json` was there while the app ran
     and gone after it quit.
   - The web-to-app handler (step 4): pass (a) passed for the Run box.
     Windows' lookup of the link's handlers named the renamed package
     alone. The link pasted into the Run box brought up Windows' choice of
     BibleText or Edge, and BibleText, chosen once, started with the whole
     link, `#v16` included, on its command line and opened at John 3:16.
     The same link clicked inside Edge stayed in Edge, and Settings → Apps
     → Apps for websites lists two BibleText entries for bibletext.co.uk,
     both on. Not run: a link clicked in Outlook or Teams, which had no
     signed-in account there, and pass (b), which needs the Store's own
     package family name, the only one the consent file names; so whether
     Windows accepts the consent file, which the site serves with status
     200 as `application/octet-stream`, is still open. Why the lookup did
     not name the Store's install is not established.
   - The browsers' prompts for `bibletext:` (step 3): run halfway, in
     Edge, on a local page standing in for the notice page, with the link
     shape the site uses on Windows. Edge's prompt said the site was
     trying to open an app, offered to always allow that page to open such
     links, and had Open and Cancel; Cancel was pressed. It named "Pick an
     app"; two packages claimed the scheme there, where a reader with the
     Store's install has one, and the prompt with one was not seen, so
     why it named no app is not established. Not run: the prompt with one
     package claiming the scheme, what it names and whether Open starts
     the app, Chrome and Firefox, which are not installed there, and
     pass (b) after uninstalling.

   Deferred from the same change: an "Open in BibleText" affordance on the
   reader pages, not only the notice pages. The Windows registry entry and
   the macOS `CFBundleURLTypes` moved to "The direct downloads do not
   register `bibletext:` links", with the AppImage, because all three are
   the same decision.

## Linux stores: from the listing source to two live channels — SNAP DONE

Planned for: the AppImageHub PR, the snap listing's fields and the snap
preferences stay; oto 3.5 in the next major, on a branch — see
docs/ROADMAP.md.

**The snap half closed 21 September 2026**: `linux-stores.yml` has run on both
architectures, the name was registered on 18 September, the credential is set,
four releases have shipped the AppImage, and the snap reached `latest/stable`
on both architectures at 15:53 UTC. What remains is the AppImageHub PR,
owner-only, plus the snap listing's console-only fields — categories and
screenshots are still unset. See docs/LINUX_STORES.md.


Prepared 16 September 2026 (`docs/LINUX_STORES.md` is the listing and the
runbook; `linux/listing.toml` the source; `cmd/linuxmeta` the generator).
Nothing has run on a Linux runner yet. In order: the account holder's
decisions (developer name, metadata licence) and the Snap Store name
registration; the first `linux-stores.yml` dispatch; the next release; an
afternoon on a Linux desktop; the snap promotion, the AppImageHub PR. The four
listing screenshots are captured (`docs/screenshots/linux/`, pinned by
`screenshots.ref`; re-dispatch `linux-screenshots.yml` to replace them).
Deferred with it:

- **AppImage registering the `bibletext:` scheme itself** — moved to "The
  direct downloads do not register `bibletext:` links", which gathers it
  with the Windows and macOS halves of the same decision.
- **oto 3.5 on Linux (pure-Go PulseAudio)** — needs Go 1.25 across CI and
  the release; would drop `libasound2-dev`, the snap's ALSA plumbing and the
  fallback question inside the snap's confinement.
- **Snap preferences under the per-revision data dir** — a revert restores
  older notes; `XDG_CONFIG_HOME=$SNAP_USER_COMMON/.config` would opt out.

## The Flatpak channel — REMOVED 29 September 2026

Prepared beside the snap and the AppImage from the same listing source: a
manifest rendered by `cmd/linuxmeta` with the offline module sources beside
it, a keyless metainfo variant, a verification-only build and smoke lane in
`linux-stores.yml`, a domain-verification file on the site and a submission
runbook in `docs/LINUX_STORES.md`. The submission went to Flathub on
29 September 2026 and was declined: the store judged the extent of
AI-generated material in the project too great for inclusion. The channel is
no longer planned, on the account holder's decision, and everything Flatpak
and Flathub was removed from the tree in commit `43e08c6c1`. The Linux
channels are the tarball, the AppImage and the snap; the
`uk.co.bibletext.BibleText` id stays on the snap's metainfo and the AppImage,
and the 1.2.10 release note that promised a Flatpak stands as shipped.

## Recapture the App Store and Play screenshots — App Store sets RECAPTURED 28 September 2026 and uploaded 29 September; the Play 1.2.17 set committed 30 September; the Play phone set waits on a decision

Planned for: 1.2.23+, the Play phone set once its shape is decided and after
1.2.21 clears Play's review; the short desktop windows and the Settings
footer, settled with no rule, in 1.2.23 (decision 35) — see
docs/ROADMAP.md.

1.2.7 shipped with the screenshot set inherited from 1.2.5, which in turn
inherits from the last captured set (build/appstore/screenshots-1.2.3/). Those
images predate two changes 1.2.7 makes to the text itself, so the store pages
show typography the installed app no longer uses:

- the reading face is now Junicode, with Ezra SIL for Hebrew, and the divine
  name is set in small capitals; and
- section headings and paragraph breaks now follow each publisher's own
  typesetting rather than a rule of our own.

Five of the eight iPad shots are reading views, so most of the set is affected.
`appstore/preflight.py` flags this every run — screenshots are a PER_RELEASE
field, and an inherited value is reported as "INHERITED from <previous>" under
"PER-RELEASE FIELDS THAT WERE NOT WRITTEN FOR THIS RELEASE". Expect that
warning until the set is recaptured; it is accurate, not noise.

What a recapture needs:

- iOS: the 6.9" iPhone and 13" iPad slots, in the pixel sizes
  `ACCEPTED_SCREENSHOT_SIZES` lists in appstore/preflight.py. An off-list size
  does not error at upload — it reaches assetDeliveryState FAILED silently
  later — so validate locally first, which the preflight does for
  `build/appstore/screenshots-ready-<version>/en-GB/` and `.../ipad13/`.
- macOS: landscape only, per the same table.
- Play: the phone and 10-inch tablet recipes are in
  `docs/SCREENSHOT_PLAYBOOK.md` §4, including `scripts/play-shot-check.py`
  and the set-the-appearance-before-the-app-starts rule. The set the Play
  listing shows, `docs/play-assets/2026-09-1.2.5/`, has the same staleness
  and is noted in `docs/PLAY_LISTING.md`.

Worth capturing the new reading face deliberately rather than incidentally: a
passage with the divine name in small capitals, and a chapter whose publisher
paragraphing differs visibly from the old uniform rule, are the two images that
show what changed.

**28 September 2026:** the iPhone 6.9", iPad 13" and Mac sets were retaken on
the 1.2.17 tree, the same eight shots each, and are ready locally under
`build/appstore/screenshots-ready-1.2.17/`, and were uploaded to the 1.2.17
version records on 29 September 2026. Matthew 1 shows the NKJV's own heading
and paragraphing in the Junicode face, Psalm 23 the divine name in small
capitals, and Psalm 82 its superscription and heading: the images asked for
above.
`docs/APP_STORE_SUBMISSION.md` ("The 1.2.17 set") keeps the set's history,
and `docs/SCREENSHOT_PLAYBOOK.md` how each shot is taken. The Play set was
retaken to the same eight scenes under `build/play/screenshots-1.2.17/`, with
the listing's first 10-inch tablet images, and `play/push-screenshots.py`
committed it on 30 September 2026 (`docs/SCREENSHOT_PLAYBOOK.md`, §3). What is
open is the phone set's shape: retaken at the default display size, or kept
at the Small size with only the light images retaken
(docs/ROADMAP.md, Later minor releases, row 14).

The iPad shots carry a small grey resize grip in the bottom-right corner: the
iPadOS 26 and 27 simulators open in "windowed apps" mode, and that mode is not
a preference that can be set from outside the simulator. It was left as it is
by decision; retaking in "Full Screen Apps" needs the setting changed by hand in
the simulated iPad's Settings > Multitasking & Gestures.

The review of the new images found five small things in the app itself.
Three are FIXED and two are left, 28 September 2026; what only a screen can
confirm is in `docs/VISUAL_TESTS.md` V15:

- FIXED: on the iPad Settings sheet the API.Bible status line was cut short:
  it drew "✓ Included with BibleText — or paste your own" with no full stop
  and a hard edge on the last letter, where `bible_key_settings.go` has the
  full stop. The width was not the cause (the line fits the iPad's row with
  280pt to spare); the ✓ was. Neither UI face has it, so Fyne takes it from
  a system font, and it chooses that font per font cache by what the cache
  has already loaded. The sheet sat inside a `compactTheme` override at 18,
  the size the app theme already gives, and an override gives its subtree
  caches of its own: the line was measured through the app's cache, where
  the header's ▾ had already brought in a narrower ✓, and drawn through the
  sheet's, and a text is painted into a texture exactly as wide as it
  measured. The override is gone (`ai_settings.go`), so the sheet is
  measured and drawn through one cache. Both key sections' status lines are
  also a `statusLine` now (`status_line.go`), which breaks between words
  when its row is narrower than its text: at 320pt the Gemini hint (288pt)
  and both error lines (253pt, 256pt) were wider than the 252pt row and
  lost their ends at the scroll's edge. Held by `status_line_test.go` from
  a 320pt phone to a 13-inch iPad and the desktop, where the row each line
  is given is checked against the lines it draws, not against its own
  measure, and by a check that no text needing a system font sits under a
  theme override. The sheet is measured again when either key's status
  takes a line more or less, so on a window with room for all of it
  nothing moves below a fold; held there too, on the real sheet.
- LEFT, by decision: under the status saying the key is included, the
  caption says the NKJV "downloads with your own free API.Bible key". The
  caption stays in every state, because read with the section header's
  "Get a key" link and the status line's "or paste your own" it states the
  choice plainly. `bible_key_caption_test.go` holds it shown with the
  included key in force, with no key, with the reader's own and with the
  included one cleared, and followed by the gap after a caption.
- FIXED: "Get a key ↗" and "Privacy Policy ↗" drew the arrow as a colour
  emoji tile on every platform, the only emoji in the interface. Fyne picks
  a face per character — the theme's Atkinson Hyperlegible, its own Noto
  Sans, then the bundled Noto Color Emoji (patches/), then system fonts —
  and only the emoji face has U+2197; it holds colour bitmaps only.
  U+FE0E, the text-presentation selector, changes nothing: the face is
  chosen by glyph coverage, not by the selector. The words are now the
  label of an `outboundLink` (`external_link.go`) that draws the arrow
  from `assets/icons/arrow_outward.svg` in the link colour, on the
  baseline, cap height tall; a tap on it opens the link. Held by pixel
  tests (`outbound_link_test.go`) on the link alone and on the three links
  the sheet builds, and by a sweep refusing U+2197 in any string in the
  package. The website's `docs/index.html` keeps its ↗: a browser draws
  U+2197 as text unless asked otherwise.
- FIXED: on the Mac the top edge of the header's Go to chip showed as a
  short arc above the open Settings sheet. The desktop sheets are modal and
  Fyne centres a modal on the window, so a sheet's top edge follows from its
  height alone; Settings, capped only by the window, started at 18.5pt on a
  1280x800 window, about a point below the top of the chip's outline. Every
  desktop sheet now opens below the header (`headerClearance`,
  `clearOfHeader` in `sheet_fit.go`): Settings, Go to, Translation, Verse of
  the day, the audio source menu and the AI answer, cross-reference and
  share-image sheets. At 1280x800 Go to and Translation already did; at a
  1280x600 window they reached into the header too and no longer do. The
  note composer and the ask sheet are sized to their content and already
  open well clear. Two sheets needed more than a cap: the cross-reference
  list guessed its chrome at 150pt against the 175pt it has, so the listed
  panel stood 25pt over its cap (at 1280x800 its top was partway down the
  chip) and now measures it (`crossref_panel.go`); and the share-image
  preview kept its image at least 200pt, which put the sheet's top inside
  the header on windows under about 490pt tall and partway down the chip
  under about 460pt, so on a desktop window the image now follows the cap
  down (`shareImageSide`, `share_preview.go`). The translation picker
  needed more too, seen only with `-tags nrsv,lsb`, the build CI tests: the
  sentences under its rows (the notice about the edition on screen, the key
  and evaluation lines) were pinned above Close, and their height is data.
  With the NRSV or the LSB compiled in, the evaluation line took the pinned
  part to 339pt, over the 322pt below the header at 1280x480, and a modal is
  never sized below its content's MinSize, so the picker started inside the
  header, and partway down the chip at 440pt. The sentences now stay pinned
  while the sheet has room for them and otherwise follow the rows inside
  the scroll, in the same words and order (`versions_ui.go`); at 1280x800
  they are pinned in every build, as before. The picker also measured them
  before it was laid out at its own width, where they wrap onto more lines,
  so one opened at 1280x1000 was 22pt taller than one sized again to that
  size; it now measures at its width. The same rule holds on a phone or
  tablet, where the room is the screen less 80pt: on a 568x320 iPhone SE on
  its side the sentences now scroll after the rows, and the sheet is 240pt
  tall with a 68pt list where it was 272pt with a 32pt one; the other phone
  and tablet sizes keep them pinned. On a 320pt phone (the first iPhone SE,
  a 320dp Android) the sentences that followed the rows into the scroll
  kept the width they had wrapped at pinned, at the popup's minimum width,
  and a scroll widens its content to that width and clips it sideways: in
  a 284pt view they were 302pt wide and each lost its last word, whenever
  two of the notice's facts held at once in the default build (the first
  launch after an epoch bump while updating) or one with the NRSV or LSB
  compiled in. The list now sits in `squeezeWidthLayout`, as the Settings
  body does, so it takes the view's width and the sentences wrap to it;
  and the rows' name, publisher and caption, which were `canvas.Text` and
  drew their whole string on one line, are statusLines that break between
  words, since on that phone the sheet is 280pt wide (the 280pt floor, the
  canvas less 48pt being narrower) and the NKJV publisher line (301.5pt),
  the WEBC name and the key caption ran past the list's edge and off the
  screen — the rows' minimum width had also forced the sheet to 364pt on a
  320pt canvas, its border on both screen edges. Held by
  `TestTranslationPickerFitsA320Phone`: the sheet inside the screen and
  every text in the list ending inside the list's edge, in three notice
  states with the build's translations. The toolkit never sizes an open sheet
  again: on a window resize it only
  re-centres it at the size it opened at, so restoring a maximised window,
  or dragging an edge up, brought the arc back. Every desktop sheet whose
  size comes from the window now registers its fit and is sized again,
  exactly as opening it at the new size would, whenever the window changes
  size (`sheet_refit.go`, run by the window's root, `windowRoot`); a sheet
  opened in a short window also takes the room a taller one gives it. Held
  by `sheet_header_clearance_test.go`: every sheet, the cross-references
  waiting and listed, and the translation picker with the build's
  translations and with eight more under evaluation and every notice at
  once, so what is covered does not depend on the build tags, opened at
  eight window sizes from 1280x860 to 1280x440 and 507x440, and resized
  under eight changes up to a 1280x2400 portrait display, each ending where
  a sheet opened at the new size sits; and by a test that the picker's
  sentences are pinned at 1280x800, follow the last row at 1280x440, and
  are pinned again, unchanged, when that window grows. Divergence: phones
  and tablets keep their sheets sized to the safe area (`sheetMaxHeight`),
  read through `sheetArea` so that a raised keyboard, which the iOS driver
  counts as the bottom inset, never shortens a sheet the light/dark reopen
  brings back (below, "A phone sheet reopened under the keyboard");
  at their cap
  those start above the header's controls rather than below the header,
  and a tablet sheet a little shorter than its cap could still start
  partway down the Go to chip. Not seen in the new images; left until a
  screen shows it. Mobile AI result cards now refit in place when the canvas
  changes size, including an iPad floating window resized without rotating.
  They retain the answer, reading offsets, explicit quotation choice and any
  pending request while rewrapping to the current reading width and height.
  Keyboard-only root layout changes do not refit them. Other mobile sheets
  retain their existing sizing policy so a keyboard does not move a sheet
  under the reader's typing: a rotation that moves the navigation rebuilds
  the window and reopens its registered top sheet; the Go to picker and the
  note composer refit themselves; the remaining sheets keep the size they
  opened at, which the modal clamps to the canvas.
  Then left: a desktop window under about 420pt tall (the window allows
  386pt) had too little room below the header for the fixed parts of some
  sheets, so cross-references while the list loads (below 418pt), the AI
  answer and Go to (below about 400pt) started inside the
  header's lower band, below the Go to chip; at 386pt cross-references
  while the list loads reached furthest in, its top 7pt below the chip's
  outline. Translation, which used to lead this list, clears the header at
  every height the window allows, in every build. The note composer's
  desktop card is sized from its content and centred, and the selected
  words it gained took it from the header's edge at about 400pt to partway
  down the Go to chip at 440pt with a selection of three rows; where the
  card is taller than the room below the header the excerpt now gives up
  rows down to one, and takes them back as the window grows
  (`share_note_ui.go`, `TestTheNoteComposerGivesUpExcerptRowsToClearTheHeader`),
  so the card clears the header at 440pt and, at one row, stood at the
  header's edge from 400pt down to 386pt as it did before the excerpt. The
  other content-sized modals — the link notices, the note-link offer and
  Ask — are centred the same way with no cap and started inside the
  header's lower band below about 400pt, and Ask below about 390pt (at
  386pt the link notice's top was 51pt down, partway down the chip).
- FIXED for 1.2.23, decision 35 (taken 15 October 2026), on its own branch,
  `fu/1223-short-windows` (`b1daedf48`, `330f935ed`), merged into `rel/1.2.23` on 21 October 2026:
  a sheet that cannot
  clear the header on a short desktop window covers it, and no window is
  made taller. Measured on the laid-out window at 1280x386 (the header
  4..71.1pt, the Go to chip 17.5..56.5pt, the room for a sheet from 79.1pt,
  below the 8pt gap): the cross-references while the list loads started at
  63.5pt, a link notice at 51.2pt, the note-link offer at 62.3pt, the share
  confirmation with a long text at 67.6pt (56.5pt at 507pt wide), the note
  composer and Ask at 72.3pt, Go to at 73.7pt and the AI answer while it
  waits at 75.4pt; a link notice still started at 78.2pt at 1280x440. A
  sheet that starts above the room where it settles took, as merged, the
  window's height from the header's top edge to as far above its foot,
  4..382pt at 386pt (`coverHeaderUnlessClear`, `sheet_fit.go`); it now
  takes its natural height from the header's top edge (below). It runs
  after each such
  sheet's sizing and its refit, so a sheet sized again in a taller window
  opens below the header again. The composer and Ask give up excerpt rows,
  and the share confirmation its box's height, only where that clears the
  header, and otherwise keep them and cover it (`fitBelowOrOverHeader`): at
  386pt the composer covers the header with all three rows. The
  cross-references are sized again whenever their body changes, so they
  cover the header at 386pt while the list loads and open below it once
  listed. The link notice and the note-link offer now refit with the window,
  at the size they opened at.
  A sheet is judged where it settles, with half a point's allowance, the
  same test for every sheet (`sheetStartsClearOf`, `330f935ed`). Judged
  straight after its resize, before its wrapped paragraph had measured
  itself at the card's width, a link notice was 4pt higher than where it
  settles, and covered the header on 441 to 448pt windows that it clears
  (78.7 to 82.2pt down), while one opened at another size and sized again
  there cleared it; Ask, the composer and the share confirmation covered
  the header at the one height where they clear it by less than half a
  point (399pt, 408pt, and 431pt at 507pt wide).
  Each sheet that can cover does so up to a height and clears the header
  above it: the link notice up to 440pt, since it opens 14.3pt taller than
  its content measures once laid out, as before; the note-link offer up to
  418pt, the cross-references while loading up to 416pt, the share
  confirmation up to 407pt (430pt at 507pt wide), the note composer and Ask
  up to 398pt, Go to up to 395pt and the AI answer up to 392pt (since
  `fu/1223-ai-answer-short`, the next entry, 413pt, and for Ask 434pt
  with a one-line question and higher with a longer one).
  As merged, a covering card sized from its content was grown to the
  window's height and showed the extra as space under its last row: at
  1280x386, 108.7pt in a link notice and 116.7pt in the note-link offer,
  Ask and the composer.
  Blank space in cards grown to cover the header was one reason the
  30 September rule for phone and tablet sheets was taken out on 1 October
  ("Phone and tablet sheets over the header" above). Whether a covering
  card keeps that height was settled on 21 October 2026: it does not
  (below).
  Two sheets Settings opens over itself were not kept clear of the header,
  before decision 35 as after it, and now are (`330f935ed`). The Delete all
  notes question was sized once, before its paragraph had wrapped at the
  card's width, so it was 81pt taller than its content and started inside
  the header or its gap on every window from 386 to 462pt tall; it is now
  sized twice, covers the header where it still cannot clear it, is sized
  again with the window, and clears the header at 386pt. On a phone or
  tablet it now takes its content's height too: laid out at 320x568,
  390x844 and 834x1194, the space under its buttons is 21pt, where it was
  35, 80 and 102pt. The model picker was capped by a
  140pt margin alone, so a list longer than the window has room for put
  its top 70pt down, 1.1pt inside the header, at any window height; it is
  now capped below the header with its gap, as every desktop sheet is, and
  sized again with the window.
  Held by `sheet_header_cover_test.go`: every sheet below the header with
  its gap or covering it at fifteen windows from 386 to 448pt, either side
  of each sheet's last covering height, with the sheets that cover pinned
  at each; the download card, the Delete all notes question and the model
  picker with twenty models among them; a covering sheet sized again with
  the window to where one opened at the new size sits, among the resizes
  386 to 442pt and 800 to 445pt; the cross-references through their load;
  the model picker below the header at 1280x860, 1280x800 and 507x600 and
  through a resize. And by
  `TestTheNoteComposerGivesUpExcerptRowsToClearTheHeader`. Each fails on the
  code before. Seen on 15 and 16 October, before `330f935ed`: on Mac and
  Linux windows 401 and 420 high and a Windows window 401, 420 and 800 high,
  every sheet swept (Settings, Go to, the share confirmation, the notices,
  the AI answer and the cross-references while they wait) clear of the
  header or covering it whole; on the Windows window the code before left
  Go to, a link notice, the share confirmation and the waiting
  cross-references partway over it. Seen again on 21 October at
  `f10f53496`, with the branch before `330f935ed` as the control, on a Mac
  window and a Linux window laid out 401 (the least they allow), 420, 445
  and 800 high, and a Windows window 760 wide laid out 401, 420, 445 and
  560 high (its screen is 800x600), where the header ends at 74.1pt, 3pt
  lower than in the tests: every sheet clears it from 82.1pt or covers it
  whole. On both builds and all three, Go to, a link notice, the note-link
  offer, the share confirmation, Ask and the waiting cross-references cover
  it at 401, the link notice, the offer and the cross-references at 420, the
  link notice alone at 445, and none above. The Delete all notes question
  clears it at 88.6, 98.1 and 110.6pt on the 401, 420 and 445 windows,
  where the control started it at 47.9, 57.4 and 69.9pt, the first two
  partway down the Go to chip. On the Mac and Windows windows the model
  picker with twenty models clears it at 82.1pt at every height, where the
  control started it at 70.0pt, 4.1pt inside the header; on the Linux
  window its list had not arrived when it opened, so it showed one row, and
  cleared the header on both builds.
- FIXED 21 October 2026 for 1.2.23, on its own branch,
  `fu/1223-ai-answer-short`, merged into `rel/1.2.23` on 22 October 2026
  (`f4460eb9b`): the AI answer's waiting line was
  cut through the middle on a window just tall enough for the panel to
  clear the header. The waiting column ("Reading the passage…", the bar,
  the model line, the hint and Cancel, with Ask's question above them)
  sits in a scroll, and the scroll was given only the room left once the
  title, the citation and the footer were placed, down to its own 32pt
  least. It was not a design choice: the column's comment has the scroll
  engage so that Cancel stays in reach, and decision 35 has a sheet that
  cannot clear the header cover it; neither leaves the line cut. In the
  tests at 1280x401 the scroll was 39.6pt tall, where the line is 38.3pt
  tall and starts 15pt down it; the line was cut on every 1280pt window
  from 393 to 414pt high, and to 434pt for Ask with a one-line question,
  at 507x401, and for Ask at 507x420. A Mac window laid out at 1280x401
  had the panel at 82.1..318.9pt, 236.8pt tall; the tests give the same
  with the header 3pt lower, where Mac, Linux and Windows windows put it,
  and a scroll 33.6pt tall. On a 568x320 screen held sideways, the
  smallest iPhone iOS 15 runs on, the scroll was 36.8pt. The waiting
  scroll is now never shorter than its column down to the foot of the
  waiting line, Ask's question included (`setThinking`'s `leadFoot`,
  `fitBody`'s least), held to the most a
  sheet may be there: on a desktop window with the header on screen the
  height a sheet covering it takes, elsewhere the canvas less its deeper
  safe inset at both edges (`sheetMostHeight`, `sheet_fit.go`). Where the
  panel then cannot clear the header it covers it, as decision 35 has
  every sheet do: in the tests Explain and Compare cover it up to 413pt
  and Ask with a one-line question up to 434pt, where all three covered
  it up to 392pt, and each clears it above that with the line whole; with
  the header 3pt lower, up to 419 and 440pt. A longer question covers it
  higher still. When the answer or an error arrives on a window in that
  band, the panel moves from covering the header to clearing it, as
  decision 35 has a sheet that can clear it do: at 1280x401 the waiting
  panel spans 4.0..397.0pt and a one-line answer 79.1..321.9pt. On the
  568x320 screen the panel is 256.5pt tall where it was 240pt. A question
  too long for the line to show under it
  gives the panel all a sheet may have, and no more: on a desktop window
  it covers the header, and on a phone it reaches the safe area's edges
  (20..420pt on an iPhone 17 Pro Max held sideways, where it was
  40..400pt). Where even the most a sheet may be is too short for the
  line, it can still be cut. Laid out as the tests lay out a phone, on a
  640x320 canvas under a 24pt status bar, Explain and Compare show it
  whole, but Ask with a
  one-line question takes the most, 24.0..296.0pt, and its scroll ends
  5.2pt above the line's foot; before the change all three were cut there
  by a 36.8pt scroll. On a 732x240 canvas, a phone's split screen, the
  sheet fills the canvas and all three are cut, on both builds. Showing
  the line there means scrolling the column so that the line's foot meets
  the scroll's, which puts the head of Ask's question out of sight, or
  setting the line above the question: a decision, not built. The answer
  and error states are sized as before. Held by
  `ai_panel_waiting_line_test.go`: the line whole for Explain, Compare and
  Ask on every 1280pt window from 386 to 440pt, at 507x386, 507x401 and
  507x420 and at 1280x800, over the styled pane Windows and Linux ship at
  1280 and 760pt wide, through five window resizes, and on four phones
  held sideways; as the quote under the citation is opened and closed at
  401, 410, 430 and 440pt, and as a model name too long for one line
  wraps at 401, 410 and 420pt; and the long question on 386, 401 and
  440pt windows, the 568x320 screen and an iPhone 17 Pro Max held
  sideways, where the panel spans 20..420pt. On `rel/1.2.23` at
  `3261a1e13` the tests fail on the windows named above, over the styled
  pane at 401, 413 and 414pt and for Ask at 760x420, the resizes to 401
  and 405pt, the 568x320 screen, at every height of the quote case, under
  the wrapped model name at 401 and 410pt and for Ask at 420pt, and for
  the long question at 401 and 440pt, on the 568x320 screen (240pt tall)
  and on the Pro Max (40..400pt); at 386pt and on the three larger phones
  the line was already whole. The sweep in `sheet_header_cover_test.go`
  now lists the AI answer as covering on 393 to 413pt windows. Mutations
  guarded: the least without Ask's question, without the bound, dropped
  on a window resize, on the quote's refit or on the model line's
  relayout, and the most a sheet may be taken from the canvas or from its
  top inset alone. One rule on all six, so no divergence. Left as it was
  then, for a decision taken on 22 October 2026 (the entry below): the
  error state kept the short scroll. In the tests, with a passage of a
  few words quoted, its two-line message was cut on 393 to 436pt windows
  (at 1280x401 through its first line), and Try again was out of sight,
  a scroll away, on 393 to 503pt windows; with the header 3pt lower, 399
  to 442pt and 399 to 509pt. Seen on 21 October 2026 at `19d77ab22`,
  against `3261a1e13`: on the Mac, over its own pane and the Linux and
  Windows panes, and on the Linux virtual machine, the line whole at
  1280x386 (laid out at 401) for Explain and Ask, and on the Mac for
  Compare, at 1280x410 for Explain and at 1280x430 for Ask, and on the
  Windows virtual machine at 760x401 and for Ask at 760x420; the control
  cut it in each, its scroll 33.6pt tall at 401. The header there ends at
  74.1pt, and the panel covering it spans 7.0..394.0pt at 401. Not run on
  Android, whose test emulator reaches a canvas short enough only under a
  display density override, or on an iPhone.
- FIXED 22 October 2026 for 1.2.23, decision 35 as taken that day for
  the AI answer's error state, on its own branch, `fu/1223-ai-error-short`
  (`abba857dc`, `9e7eb94fd`), from `rel/1.2.23` at `91e1ec683`: the
  error state's message was cut and its button out of sight
  on a window just tall enough for the panel to clear the header. At
  1280x401, the old panel showed a 39.6pt scroll for the message and Try
  again, whose column needed 130.9pt. The error state now keeps its body
  by the waiting line's rule (`wholeThrough`, `fitBody`'s least): never
  shorter than its content down to the foot of what that state must show
  whole, the waiting line or the error's whole column, Ask's question
  included, held to the most a sheet may be there (`sheetMostHeight`).
  Where the panel cannot clear the header it covers it at its natural
  height (`coverHeaderUnlessClear`).
  Settled 23 October 2026: the quote under the citation opens
  automatically only where it leaves room for that minimum as well as
  the usual few answer rows. The previous 96pt allowance was shorter
  than the error's 108.6–153.2pt minimum: with a long quotation, it made
  the panel cover the header at 1280x620 even though closing the quote
  let the error clear it, and on an iPhone 16 Pro held sideways it cut
  Ask's Try again and the three-line message's button. `fitBody` now
  takes the larger of the usual allowance and the state's minimum, in
  the waiting and error states alike. The citation and disclosure stay
  visible, and the reader can open the quote; an explicit open or closed
  choice survives a request failing, Try again, and window resizing.
  No icon, button or spacing dimensions change.
  A question too long for the window can still put the error below the
  visible scroll, which opens at the head of the question. The panel
  takes the most a sheet may be. Showing the error there would require
  scrolling the question's head out of sight or setting the error above
  it, as for the waiting line; that remains a separate decision.
  Closing the quotation also cannot make a column taller than the whole
  safe area fit. The existing cases on the first-generation iPhone SE
  held sideways, a 640x320 canvas under a 24pt status bar and a 732x240
  split screen retain their scroll and height cap. The iPhone 16 Pro's
  landscape error no longer belongs in that expected-cut list: its
  quotation can close to leave room for every tested error form.
  Regression coverage is in `ai_panel_error_whole_test.go` and
  `ai_panel_waiting_line_test.go`: the error stays whole on short
  desktop windows and over the styled pane, on resizing and quote
  toggles, and on phone safe areas; a long question keeps the sheet at
  its height cap. `TestTheAIErrorClosesTheQuoteBeforeCoveringTheHeader`
  covers long quotations at 1280x620 and the no-key error at 1280x480;
  `TestTheAIWaitingQuoteClosesWhenItsQuestionNeedsTheRoom` covers the
  waiting and error states under a long question; and
  `TestTheAIQuoteChoiceSurvivesFailureAndRetry` covers both explicit
  disclosure choices through failure, retry and resizing. The earlier
  tests fail on `91e1ec683`, apart from the already-capped 732x240 cases.
  Verification recorded on 22 October 2026 applies to the earlier
  `a7b1b670d` build, before the automatic-quote change: on the Mac the
  error was whole at 1280x401, 436, 470, 503 and 520, with its footer
  buttons inside the sheet, through a resize from 401 to 800pt and back
  and after Try again at 401pt. A quote that opened at 520pt made that
  build cover the header again after clearing it at 503pt. The new
  quote rule is integrated locally in `rel/1.2.23`, uncommitted; combined
  checks and device verification remain.
  Not run for this branch: the quote opened and closed in the app, the
  Linux and Windows panes mimicked on the Mac, the Linux and Windows
  virtual machines, an iPhone and Android.
- CHANGED for 1.2.23, decision 35 as settled on 21 October 2026, on its own
  branch, `fu/1223-short-windows-2` (`0c2e48e40`, `9c3da08a4`,
  `5bc64cbb8`), from `rel/1.2.23` at `3261a1e13`, merged into
  `rel/1.2.23` on 22 October 2026: a sheet that covers the header takes
  its natural height, capped at the room from the header's top, with its
  top edge at the header's. The natural height is the sheet's preferred
  height, what it would take with unlimited room, never less than its
  content's least height once that has settled (`coverHeaderUnlessClear`,
  `sheet_fit.go`, one rule for every sheet). A card sized from its content
  asks for nothing beyond it, so it is only as tall as its content; a sheet
  whose lists or scroll areas use the room asks for the height it takes on
  a window with room to spare: Go to the 680pt its lists take
  (`pickerSplitMaxHeight`), the cross-references their 460pt panel
  (`crossRefPanelHeight`), the AI answer its whole body. Where the room is
  shorter those fill it, so Go to's book and chapter lists stay as usable
  as before. The model picker asks for its whole list too, but its list
  scrolls and it clears the header on every window the app allows, so it
  never covers it. Fyne centres a modal on the size of its popup, which it
  holds to its canvas's size, so a sheet shorter than the room centred
  with its top inside the header; it is now laid out on the window's
  canvas raised by the difference (`raisedCanvas`, `raiseSheet`): the
  popup reaches that far above the window's top edge and still covers the
  window to its foot, so its shade dims the whole page, and the sheet
  centred on it starts at the header's top; with an overlay up, a tap goes
  to the top overlay alone, so none reaches the page. The raise rests on
  Fyne 2.7.4's popup, which is itself the overlay and is placed by nothing
  else; Fyne 2.8 places a popup through an overlay container the overlay
  stack sizes from the window's canvas, so the raise has to be built again
  for it (`raisedCanvas`, `5bc64cbb8`). Each fit lays the sheet out on the
  window again before judging where it settles (`centreOnWindow`), so a
  sheet given room again clears the header as before. Laid out at
  1280x386, a link notice, the note-link offer, Ask, the note composer and
  the share confirmation cover the header as tall as their content, where
  they were 108.7, 116.7, 116.7, 116.7 and 17.3pt taller than it, and the
  study panel with a short answer or an error covers it with its whole
  body shown, the body as tall as on a 1280x800 window and no taller; Go
  to, the cross-references while they load and the AI answer while it
  waits, whose waiting column scrolls and is taller than the room with the
  panel's chrome, fill the room from the header's top.
  At 1280x401, the least height a Mac window's canvas is laid out at, the
  share confirmation, the link notice, the offer, the cross-references
  while they load and the AI answer while it waits cover the header: the
  first three as tall as their content, where they were 8.3, 123.7 and
  131.7pt taller, and the cross-references and the waiting AI answer
  filling the room; Go to and the AI answer with an answer or with an
  error clear it there; with an error it covers it there to stay whole
  on `fu/1223-ai-error-short` (the entry above). On the branch alone the
  waiting AI answer cleared the header at 401pt, its waiting line cut by
  the scroll; merged with `fu/1223-ai-answer-short` (the entry above) the
  panel never covers shorter than its chrome and the line's least, and as
  its whole column with the chrome is taller than the room on every
  window where it covers, to 409pt for Explain and Compare and 430pt for
  Ask with a one-line question in
  `TestACoveringWaitingPanelTakesItsNaturalHeightWithItsLineWhole`, it
  fills the room there. That test holds the line whole, the popup
  spanning the window and no blank band under the column on every 1280pt
  window from 386 to 440pt; it fails on `fu/1223-short-windows-2` alone,
  the line cut from 393 to 414pt and for Ask to 434pt, and with the panel
  covering at its least alone, as a card does, which stopped it short of
  the room's foot with the column cut short.
  `TestACoveringSheetTakesItsNaturalHeight` holds both windows and fails on
  `3261a1e13` for the five cards; with Go to asking for nothing beyond its
  content its Go to case fails, the sheet ending at 242.5pt of a room to
  382pt; with the study panel asking for 10000pt beyond its whole body,
  its short answer and its error fill the room, 4.0..382.0, and their
  cases fail (`9c3da08a4`). The sweep
  (`TestDesktopSheetsClearOrCoverTheHeaderOnShortWindows`) now requires a
  covering sheet's top edge at the header's and its foot no lower than the
  room, and every sheet's popup to span the window, takes the 401pt window
  too, and measures a sheet's box in canvas coordinates; with the sheet
  left centred at its natural height it fails, the cards starting 58 to
  78pt down, in the header or the gap below it, and the share confirmation
  7 to 16pt down; with the raised popup laid out at the window's top left
  on a canvas shorter by the raise, each covering sheet still starts at
  the header's top but its popup and shade stop short of the window's
  foot, and the sweep, the natural-height test and the resize test fail
  (`9c3da08a4`); and `TestASheetCoveringTheHeaderIsSizedAgainWithTheWindow`
  fails with a raised sheet never laid out on the window again.
  Seen on 21 October 2026 in scratch development builds of `0873d2665`
  beside `3261a1e13`, values read back in the app. On the Mac, in a window
  asked for at 1280x386 and laid out at 1280x401, the header at
  7.0..74.1pt, 3pt lower than in the tests: the note-link offer, a link
  notice, and Ask and the note composer, which cover there, cover the
  header from 7.0pt to 268.3, 276.3, 311.5 and 311.5pt, as tall as their
  content, where the control's ran to 394.0pt with 125.7, 117.7, 82.5 and
  82.5pt of blank under their last rows (the tests' 401pt window, its
  header 3pt higher, gives 131.7 and 123.7pt for the offer and the
  notice); each raised popup spans the canvas from above its top edge to
  its foot at 401.0pt; Go to and the waiting cross-references fill
  7.0..394.0 and the waiting AI answer clears the header at 82.1pt, on
  both builds, neither of which had the waiting line's least; the offer resized with it open from 401 to 800pt opens
  below the header, at 269.3pt as one opened there does, and back at 401pt
  covers it again from 7.0pt. At 1280x800 the offer, Ask, a link notice
  and Go to open where the control opens them. The Linux and Windows panes,
  mimicked on the Mac, gave the same numbers for the offer, Ask and Go to.
  Not seen on the Linux and Windows virtual machines, so not under their
  own window systems.
- CHANGED for 1.2.23 by decision 35 (`b1daedf48`), not a defect: the iPad
  Settings sheet ends on a SHARED NOTES heading with its card below the
  pinned "Changes save automatically." footer. The body is a scroll that
  ends above the footer, nothing is drawn under the footer, and at that
  size the body is about 150pt taller than its view, so the sheet opens at
  the top with the last sections below the fold, the scroll bar showing it.
  Scrolled to the end, the last card sits wholly above the footer.
  `settings_footer_test.go` holds both at the iPad sizes. A rule now sets
  the footer off from the body, mirroring the one under the title, so it is
  plain that the body continues: the same width and position across the
  sheet, the same gap to the body (`TestSettingsFooterIsRuledOffLikeTheTitle`,
  at the iPad sizes and on a desktop window; it fails on the code before).
  The decision named the iPad's sheet; the sheet is one component on every
  platform, so the rule was built on all six.
  Seen on 15 and 16 October on Android (API 35 and 30), an iPhone 16 Pro, an
  iPad Air 11 and an iPad Pro 13 held upright, and on the Mac, Linux and
  Windows windows; not yet on the iPad Pro held sideways. Settled on
  21 October 2026: the rule on no platform, the iPad included. It is taken
  off on all six (`a4598d375`, on `fu/1223-short-windows-2`, merged into
  `rel/1.2.23` on 22 October 2026), so the footer sits under the body as
  in 1.2.22, and no
  divergence is recorded. `TestTheSettingsFooterHasNoRule`, at the iPad
  sizes, a phone and a desktop window, fails on `3261a1e13` at all five
  with the rule 7pt below the body, and replaces
  `TestSettingsFooterIsRuledOffLikeTheTitle`; `settings_footer_test.go`
  still holds the body ending above the footer. Seen on 21 October 2026 in
  scratch development builds of `0873d2665` beside `3261a1e13`: no rule
  between the body and the footer on the Mac at 1280x800, under the Linux
  and Windows panes mimicked there, on an iPhone 16 Pro simulator and on
  the Android API 35 emulator, where the control draws one 7pt below the
  body; the title keeps its rule. Not seen on an iPad, nor on the Linux and
  Windows virtual machines.

## Settings: Clear runs past the key card at 320pt — DONE 28 September 2026

At 320pt, the iPhone SE's width, the Paste / Test key / Clear row under each
key field in Settings, the assistant's and API.Bible's, was wider than its
card, and Clear was cut off: it spanned 250.3–337.9pt, where the key card
ends at 291.0pt and the sheet at 298.0pt (Paste 32.0–122.3, Test key
129.3–243.3), measured on the real sheet. The row was an HBox, which lays its
buttons out at their own widths whatever width it is given: the three need
305.9pt, and the card gives its rows 252pt there. It predated the Settings
fixes above. Measured the same way, 375pt was short too: the rows get 299pt,
and Clear ended at 341.9pt, 6.9pt past the end of the key field above it, over
the card's padding and against its border, 0.1pt inside the card's edge.

Both rows are now one `keyActionsRow` (`key_actions_row.go`), used by
`ai_settings.go` and `bible_key_settings.go` alike, so the two sections cannot
lay their buttons out differently. It places them exactly as the HBox did
wherever they fit, and the button that would run past the row starts a line of
its own, one padding below: at 320pt and 375pt Clear sits under Paste, and the
card grows by that line. Nothing is shrunk or stretched, and the labels and
icons are the same. Clear moves from beside Test key to under Paste, one
padding from its nearest neighbour, as it was on one line. The row asks for
the height of the lines it broke into at the width it was last laid out at, as
a `statusLine` does, which the sheet's measuring twice whenever it sizes
itself already provides for. From 393pt up the buttons are where they were: at
393pt, 440pt, 834pt, 1032pt and a 1280x800 window every button's position was
measured at bec3f265a and after, and none moved.

`key_actions_row_test.go` holds, in both sections at 320pt, 375pt, 393pt,
440pt, the 11-inch and 13-inch iPad widths and the desktop, that every button
is visible, laid out at exactly its own size, neither squeezed nor stretched
on a line of its own, inside the width the card gives its rows (the key
field's), wholly in the body's view, between the field and the status line,
clear of the other two, and reached by a tap at its centre; that at 320pt and
375pt Clear starts the second line under Paste; and that at the wider widths
each button is where the HBox put it. A second test holds the layout's
breaking rule on its own. The switch labels in the same sheet ran past their
cards at the same widths; they are the entry below. What only a screen can
confirm is in `docs/VISUAL_TESTS.md` V15.

## Settings: the switch labels run past their cards on narrow phones — DONE 28 September 2026

The labels of the three Settings switches ran past their cards the way Clear
did: "Show the translators' footnotes", "Show notes people share with you"
and "Show the words of King Jesus in red". Each was a `widget.Check`, whose
label is one `canvas.Text`, and Fyne paints a `canvas.Text` at its full
width whatever width it is given. The checks ask for 279.4pt, 304.2pt and
322.8pt, and the cards give their rows 252pt at 320pt, 284pt at 360pt, 299pt
at 375pt and 317pt at 393pt. At 320pt all three labels ran to the scroll's
edge, which cut them off; at 360pt the second and third did; at 375pt the
third ended at 350.8pt against the scroll's clip at 342.0pt and read "in re".
At 393pt only the room widget.Check keeps after its label ran over: the words
end 2.2pt inside the row. The Settings sheet shows all three on iOS, Android
and macOS, and on Windows and Linux with the styled pane. It predated the
key-row change above.

The three are now `wrapCheck` (`wrap_check.go`, built in `ai_settings.go`):
`widget.Check` itself, with its box, focus ring, colours, taps and keys,
and a renderer that draws the check's own label exactly where the check
draws it wherever the words fit the row, and otherwise breaks them between
words onto further lines, by the rule the key status lines use
(`statusLines`), centred down the check as one line is. It asks for
widget.Check's size wherever the row has it; broken, it asks for the lines'
height and the widest line's width beside the box. The room widget.Check
keeps after its label is asked for only as far as the row has it, so a label
whose words fit does not break for empty space after it. Taps and hover reach
the box and every word, since widget.Check takes them across the size it last
asked for. At 320pt all three labels take two lines, at 360pt the second and
third, at 375pt the third; from 393pt up every label is drawn as before. No
wording, colour or size changed, and the sheet is the same Fyne sheet on
every platform.

`wrap_check_test.go` holds, at 320pt, 360pt, 375pt, 393pt, 440pt, the 11-inch
and 13-inch iPad widths and the desktop, that every word of each label is
drawn inside its card's row and inside the switch, each line under the one
before and the lines centred, that the switch is laid out at least at the size
it asks for, that taps at the box, the middle of the last line and the end of
the widest line reach it, and how many labels break at each width; and where
a label is one line, that it is the check's own label at widget.Check's place
with widget.Check's size. A second test holds the breaking rule on its own,
including that a label stays on one line down to the width of its words.
`TestSettingsSheetSqueezesNothing` (`sheet_fit_test.go`) walks the whole
sheet at those widths and at 402pt, with no assistant and with each of the
four, and finds no visible object laid out smaller than it asks: it fails on
the key row as an HBox and on any switch as a widget.Check. What only a
screen can confirm is in `docs/VISUAL_TESTS.md` V15.

## NKJV cross references: the panel's second pass — PAUSED 10 September 2026

Planned for: stays, gated on the licensing reply — see docs/ROADMAP.md.

The edition's own cross-reference apparatus is captured, resolved and drawn
in the cross-references panel behind the `nkjvxrefs` build tag (commits
ecdeb1cad, 94385ac27; the design memo and the decision are in
`docs/SCRIPTURE_WORKLIST.md` S20 and `docs/SOURCE_FIELDS_DECISIONS.md`).
What is DONE and stays: the decoder keeps each tagged citation as a span
(`Footnote.Refs`, no cache-epoch bump); `publisher_xrefs.go` resolves the
notes; `crossref_list.go` composes the panel in blocks; the flag is set for
the NKJV only in `versions_nkjvxrefs.go`; `scripts/run-ios-device.sh
--nkjvxrefs` builds it for a phone; the canon census and the copyright-line
assertion are in `apibible_live_test.go`; the tests carry controls.

What the first look on the phone found — and `TestRenderCrossRefPanel`
reproduces at phone size without a device — is that the presentation is
wrong, in ways a reader sees as "confusing and not consistent":

1. Rows run off the right edge. The Treasury's accordion title ("More
   references — Treasury of Scripture Knowledge") is wider than the panel
   and forces the whole list to that width, so every row is clipped, the
   parallel rows' previews included. A defect, not a design choice, and the
   biggest single cause.
2. Two row idioms in one list: the Treasury and parallel rows are cards
   (bold accent reference, preview, tap the card); the publisher's rows are
   a small grey verse number over a paragraph with inline links.
3. Two heading styles: the small bold block heading and the accordion's
   large bold button with a chevron.
4. The verse number repeated on every row of a single-verse selection.
5. The edition's notice shown twice, in the block and in the footer.
6. A lone parenthesised "compare" citation standing as a whole row,
   explained only by a legend further down.
7. The empty state (a verse with no note) is a heading, a line, a long
   notice, then the accordion — the notice dominates a screen with nothing
   to show.

WHERE IT IS GOING — agreed in principle, not yet built. One panel, not two
stacked in one:

- One row idiom everywhere. The publisher's references become the same
  cards the Treasury uses — bold reference, preview, tap the card — one
  card per cited passage, in the publisher's order, never re-sorted or
  capped, labelled from the tagged id ("John 19:39", so a continuation
  such as "19:39" is readable). The parenthesised "compare" citations
  become one muted "Compare: (1 John 4:9, 10; Rev. 1:5)" line under the
  cards of the note they belong to — kept, self-explanatory, no legend.
  Full-card taps also settle the small-tap-target concern the memo raised.
  This reverses the memo's verbatim-per-note row; the reason the memo gave
  (unreadable continuation labels) does not hold when the label comes from
  the id rather than the citation text.
- One heading style: small muted section labels — NKJV · Treasury of
  Scripture Knowledge — with the Treasury folded behind a plain "Show N
  more references" link rather than an accordion, which also removes the
  width defect.
- Verse labels only where they carry information: a small "Verse N"
  divider when the selection spans several verses, nothing otherwise.
- The notice once, in the footer, with the sources line naming each set
  once: NKJV (Thomas Nelson) · Treasury: OpenBible.info (CC-BY) · Gospel
  parallels.
- The empty state in one line — "No NKJV cross references for this
  verse" — with the Treasury shown open beneath.

This touches the presentation layer only — `crossref_list.go`, plus a small
helper in `publisher_xrefs.go` that splits a note's untagged "compare"
remainder from its tagged citations — and the tests in
`crossref_list_test.go`, which pin the current composition and must be
rewritten with it. The decoder, the data, the resolver's order and the
build tag are unchanged by it.

GATES. Nothing displays in a store build until the API.Bible licensing
reply (sent; awaiting) says it may; the second pass can be built and looked
at on a tagged device build meanwhile. On a yes: move the one line in
`versions_nkjvxrefs.go` into the registry entry and drop the tag; add the
dedicated copyright page the terms' §7 asks for (a standing pre-ship item,
`versions_ui.go`). On a no: nothing to unwind.

TO RESUME. Build for the phone: `scripts/run-ios-device.sh --nkjvxrefs`.
See the panel without a device: run `TestLiveAPIBibleFullCanon` with
`BIBLETEXT_FULL_CANON_OUT` set (19 s on the Pro plan; the decoded canon is
licensed text and must stay outside the repository), then
`BIBLETEXT_RENDER_XREFS=<that file> BIBLETEXT_RENDER_OUT=<dir> go test -run
TestRenderCrossRefPanel -v .` writes seven phone-sized PNGs (John 3:16 on and
off, John 3:1-3, Matthew 3:1 with parallels, Psalm 3:1 with a title note,
Psalm 3:2 empty). Look at those before and after the change; the seven
findings above are all visible in the "before" set.

## FIXED: Share as image failed silently on Android 6.0-9.0 (API 23-28) — 20 September 2026

Not a 1.2.7 regression: the behaviour is as old as the feature, and Android has
never shipped through a store, so nothing in the wild is affected. It surfaced
after 1.2.7 was already with Google for review, which is why the entry weighs
superseding the in-review build at the end.

`BtBridge.shareImage` (android/BtBridge.java) publishes the rendered verse card
through `MediaStore` rather than a FileProvider, and the manifest declares
`WRITE_EXTERNAL_STORAGE` capped at `maxSdkVersion="28"` to justify it
(cmd/mobile/AndroidManifest.xml). On API 29 and later, scoped storage means the
`MediaStore` insert needs no permission at all and the feature works. On API
23-28 that permission is runtime-granted, and the app never calls
`requestPermissions` for it — the only runtime request anywhere in the app is
POST_NOTIFICATIONS in BtAudio.java. So the insert fails, and because the whole
body is wrapped in `catch (Throwable)` that only logs, the reader taps "Share as
image" and nothing happens: no share sheet, no error, no toast.

Two things to fix, and they are separable:

1. The silence. A swallowed `Throwable` on a user-initiated action should say
   something. Cheapest correct fix even if the permission work is deferred.
2. The permission. Either request WRITE_EXTERNAL_STORAGE at runtime on API
   23-28 before the insert, or drop the API 23-28 path and gate the "Share as
   image" action off below API 29 so the action is absent rather than broken.
   Dropping it also removes the last justification for declaring the permission
   at all, which is worth having: an unused declared permission is the kind of
   thing a Play reviewer's unused-permission check flags, and the manifest
   comment already records that READ_EXTERNAL_STORAGE was removed for exactly
   that reason.

Whether to supersede the in-review build for this is a judgement call about
reach: API 23-28 is a small and shrinking slice, and the closed testers will
almost certainly be on API 29+. Fixing it in the next release is defensible.

**FIXED 20 September 2026, both halves.**

The silence went first, because it is the part that applies on every API: the
two ways out of `shareImage` that said nothing — a bare `return` when the
insert yields a null URI, and the `catch (Throwable)` that only reached logcat
— now both put a message on screen. A share the reader started cannot end in
silence, whatever the cause.

The permission is requested on 23..28, fire-and-forget, matching
`maybeRequestNotifPermission`: GoNativeActivity overrides no
`onRequestPermissionsResult`, so there is no callback to resume on, and the
honest behaviour is to ask and say "allow it, then share again" rather than
leave a share the reader thinks is still coming. The feature was NOT gated off
below 29 — the option that also drops the manifest declaration — because
telling a reader why something needs permission is better than removing the
action from under them; if the unused-permission concern returns, gating is
still available and this entry records the trade.

**The floor is 23, not 21.** `Context.checkSelfPermission` and
`Activity.requestPermissions` do not exist below API 23, and the app's minSdk
is 21 (asserted twice in `scripts/build-android.sh`). On 21-22 the manifest
declaration is granted at install, so there is nothing to ask for; the guard
reads `SDK_INT >= 23 && SDK_INT < 29`. This is the kind of detail that is
invisible to reading and obvious to a compiler — which is why the second half
of this fix is a new gate.

### The gap that let it live: nothing compiled the Java

`scripts/check-android-pane.sh` closed this gap for the android-tagged GO
sources. The Java on the other side of the JNI boundary — `BtBridge` and
`BtAudio`, 78 classes carrying the native selectable TextView, the selection
menu, the media session and the foreground audio service — had no gate
anywhere. `go build ./...`, `go vet`, the whole suite and both mobile pane
checks are blind to it, and the only thing that ever compiled it was a full
`scripts/build-android.sh` run on one machine with the SDK installed.

`scripts/check-android-java.sh` now compiles it in seconds with the same recipe
the real build uses (`javac --release 8 -Xlint:-options -cp android-36/android.jar`),
fails rather than skips on a runner with no SDK, and refuses to pass if it
produced implausibly few classes — a gate that compiles nothing reads as a
pass. It runs in `ci.yml` beside its Go twin. Proved against two controls: an
ordinary typo, and a Java 9 API used in an API 8 source, which is what makes
`--release 8` load-bearing rather than decorative.


## Phone landscape reading — shipped on both phone platforms — DONE

Shipped and on by default on both phone platforms, verified on an iPhone
simulator and now on a physical Pixel. Nothing outstanding; the entry below is
the record of what was built and why.

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
bridge re-places its own view, by scroll fraction until 1.2.22 and by the verse
at the top from it). Simulator-verified on the
iPhone 16 Pro: presentation, typography, the reading position across
rotation in both directions at the chapter top and mid-chapter, chosen
full-screen round-tripping with its restore button, a live selection
dropping cleanly, Books and Search keeping their layout in landscape.
Emulator-verified on Android 13 (API 33) and Android 15 (API 35): the Read
tab's landscape presentation with the position kept across both rotations, a
link's verse wash surviving them, and the Books tab keeping its rail.
Corrected 13 October 2026: Android keeps a fraction of the scroll range, not
the verse, so a rotation moves the verse at the top in about a third to a
half of positions ("Where the app re-finds the reader's position from
text", AND-3). Fixed the same day for 1.2.22: the bridge keeps the verse at
the top and places it first, with the fraction as the fallback.
`BIBLETEXT_DEV_PHONE_LANDSCAPE=on|off` seeds a scripted simulator run. The
mode's typography half was retired on 24 September 2026: the page is chosen by
width on every surface now (docs/READING_TYPOGRAPHY.md, "The reading page"),
and a landscape phone reads the book page because its pane is wide enough.

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

Still open: two questions in docs/VERSION_STATES.md, which stay. The third,
a key cleared on purpose, was decided on 15 October 2026, built for 1.2.23
on `fu/1223-key-gone` (`e43d8e64e`) and merged into `rel/1.2.23` on 22
October 2026 (`993b52639`): the launch gives the translation up and
the picker says the reader removed their key (Later minor releases, row 23,
in docs/ROADMAP.md). The launch walk now has 26 cells, with the key a
restore did not bring back (row 10 there), and the credential walk 27.
Decided on 21 October 2026: decision 11's line names the key removed,
for the key included with the app "You removed the included API.Bible
key, so World English Bible is shown in place of New King James
Version.", and for the reader's own the words of 15 October. It is built
on `fu/1223-key-gone-2` (`a206ffa18`; row 16 there), where the launch
walk's `key-cleared-included` fate and a tenth knowledge state in the
credential walk hold it, 28 and 30 cells (`fa89b14f5`). Confirmed on 22
October 2026, as built: a key typed into the field over the included one
and deleted again is named as the reader's own, since the field saved it
as theirs, and a removal recorded by a release before 1.2.23, which wrote
the same record for either key, is named as the reader's own.

Both halves are in docs/VERSION_STATES.md: the storage diagram (M1–M3) and,
since 2026-09-05, a second diagram for M4, the launch space and the arrivals
layer drawn as the enumerations drive them, with a table of every space's
own count as its test logs it (18, 10, 160 + 310 journeys, 8, 22, and 1142
journeys over 4780 steps in six worlds since 2026-09-25). The document's
closing section had still said M4–M7 were unenumerated; it now says what is,
and what the enumerations do not claim. The record of the work follows. Eight
defects found on 2026-09-25 were fixed the same day; they are listed at the
end of this entry, with three questions for the owner that came with them.


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

Both have since been enumerated. The launch space went in joined to reading
position and canon shape rather than alone (`version_launch_flow_test.go`,
M5 x M6 x M7), because the failure it was built for lives in that intersection
and in none of the three separately; the block below is its record. The
download space went in as M3, refresh and download
(`version_refresh_flow_test.go`): 160 cells over those axes plus the active
version, and 310 journeys to depth four.

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

Not reachable when it was found (nothing wrote a non-default version's
current epoch while its previous decode was in memory), but one obvious
feature away. Its record also claimed the fix closed a live liveness hole —
that a stale non-default translation previously had no way to stop being
stale within a session. It did not: the fix waits for a current epoch that
nothing in a session wrote, for the reason the sentence before gives. That
hole was D17, fixed 2026-09-25 (below), and D17 was that feature: the
refresh now writes the current epoch, so the reload is live only between the
refresh's cache write and its tail on the UI goroutine.

**The arrivals layer — enumerated 2026-08-29, as JOURNEYS** (780 to depth
four at the time), because an arrival is a promise kept or broken over time and every way
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

**All seven machines and the arrivals layer are enumerated. Every defect
found up to 2026-08-29 is closed; four found on 2026-09-25 were fixed the same
day, with a fifth (D21) found while fixing them and three more (D22–D24)
found by verifying the fix.**
Two lessons worth keeping. Third confirmation that
cross-products are blind to flow (the arrivals defect is invisible to every
cross-product in the suite; the journeys found it by four routes). And: a
harness can only look where its model points. The journeys found D12 and went
quiet; re-reading the same ground with no model in hand found four more,
including the original incident arriving through a version switch rather than
a launch.

**Found and fixed 2026-09-25.** The arrivals walk was given the disk that
decides an arrival (the link's translation holding only its previous edition),
the reader's remembered licensed translation, and a load of the reader's own
still in flight when a link arrives; reading where its first finding reaches
led to the launch's hand-off. Each is recorded in docs/VERSION_STATES.md with
its shortest route, and the tests that reproduced them are now the fixes'
guards.

| | Defect | Severity | Status |
|---|---|---|---|
| ~~D18~~ | ~~The launch hands its state to the screen field by field and drops the two records the restore makes: the reader's chosen translation and the stale-edition mark. D9, D10 and D3's launch site are fixed on a state the reader never sees~~ | durable | **FIXED 2026-09-25** |
| ~~D19~~ | ~~The arrival mark is spent by a load it was not set for (a failed link load leaves it set; a link parked behind the reader's own load gives it to that load), so the reader's own choice does not spend the remembered translation~~ | durable, once D18 is fixed | **FIXED 2026-09-25** |
| ~~D20~~ | ~~A previous edition behind the substitution sentence: the picker says a translation was shown instead, and nothing about the edition~~ | session, once D18 is fixed | **FIXED 2026-09-25** |
| ~~D17~~ | ~~A translation shown from its previous edition is not updated while the app runs: nothing in a session fetches a non-default translation that is already in memory~~ | session | **FIXED 2026-09-25** |
| ~~D21~~ | ~~The default's refresh sentences say its text is shown while another translation is on screen~~ | session | **FIXED 2026-09-25** |
| ~~D22~~ | ~~A remembered translation saved beside a place from a wider canon is dropped by the launch that can open it~~ | durable, once D18 is fixed | **FIXED 2026-09-25** |
| ~~D23~~ | ~~A fetch that lands and cannot be written purges the only copy on disk; the D17 refresh reaches it in session~~ | durable | **FIXED 2026-09-25** |
| ~~D24~~ | ~~The substitution sentence promises a return nothing in the running app performs~~ | session, once D18 is fixed | **FIXED 2026-09-25** |

They were closed in one change, because D18's fix makes D19 and D20 live. The
launch now hands its state over through one named function, adoptLaunch,
which carries both records (D18). Who asked for a translation is an argument
of the load, passed by the call that starts it and spent by its own tail, so
no load can leave a mark for another or take one (D19). The picker footer
says every true fact, one per line, in place of the highest-ranked one (D20),
and the default's own sentences only while the default is on screen (D21).
The refresh owes an upgrade to every public-domain translation recorded as
showing its previous edition, never a licensed one, and swaps the current
edition in place when it lands (D17). What closed each, and the guard that
holds it, is in docs/VERSION_STATES.md.

Verifying that change found three more, each made live or moved by it. A
remembered translation is saved beside wherever an arrival took the reader,
and when that was a book its canon lacks, the launch that could open it
declined the whole restore and the next save wrote the default over it; the
restore now keeps the translation and drops only the place, which waits in
the trail (D22). The refresh can fetch a translation whose only copy on disk
is its previous edition, and a fetch whose write failed still purged it; the
purge now waits for the current edition on disk (D23). The substitution
sentence ended "comes back when it can", which nothing in a running app does;
it now says the choice is tried again each time the app starts, and offers
choosing it now while its row can be chosen (D24). The verification also
found that the walks reproduced, rather than ran, what the app does on the
far side of a goroutine; the three doors that work leaves through are now
seams the walk holds, so the load's cause, the refresh's choice of what to
fetch, and what its timer does are the app's own.

Three questions are the owner's, recorded in docs/VERSION_STATES.md under
"Open decisions": whether tapping the translation shown while a substitution
is in force should accept it; whether a key the reader cleared on purpose
should still be remembered as a substitution at every launch; and whether a
launch that cannot open the remembered translation should prefer a
translation whose canon holds the saved place.

## The tests tagged `!race` run only in CI's second pass, never under the detector

Planned for: stays — see docs/ROADMAP.md. Retitled 6 October 2026, for
1.2.20. It was "CI runs with the race detector, so 14 tests never run there",
which stopped being true when the second pass below arrived.

DONE for the cheap half. The Linux job now runs the suite a second time
without the race detector, so the tests in the files tagged `!race` compile
and run on every push. There were 14 when this was written, covering focus
order, render output, wrap layout, the download-status row, the audio card,
the search mode row and the legacy pane; seven files carry the tag today.
Retiring the `!race` tag by teaching them to stand up under the detector is
the real fix and is still open. It is not only tidiness: on 6 October 2026
the verse-of-the-day card test's occasional failure turned out to be a race
with a timer one of these files armed (`ui_render_test.go`), which the race
run never saw because that file is not in it (the deferred-UI timers entry).

The Android half of the old entry is out of date as well. CI cross-compiles
the Android pane (`scripts/check-android-pane.sh`, with and without `next`)
and compiles the Java (`scripts/check-android-java.sh`). What no CI job does
is build or run an APK (docs/PLATFORM_MATRIX.md, divergence 13).

## Source fields: three defects and a decision list — DONE, closed 6 October 2026

Tracked in `docs/SCRIPTURE_WORKLIST.md`, not here. That file is the worklist
for work on the TEXT — what the decoders read from each source and what
reaches the reader; this backlog stays with the app around the text. The
analysis behind each item is in `docs/SOURCE_FIELDS_DECISIONS.md`, and the
inventory of every field is in `docs/SOURCE_FIELDS.md`.

docs/SOURCE_FIELDS_DECISIONS.md analyses every field the decoders skip or
half-keep, with costs and a recommendation for each. Three of its findings
are defects rather than decisions, in the order they should be fixed:

1. The paragraph rule tests only straight quotation marks, so a verse ending
   in curly-quoted speech never starts a new paragraph. About 2,100 eligible
   breaks per edition are suppressed. One line, no cache epoch.
2. Psalm 119's acrostic letters are in WEB and WEB Catholic verse text: ALEPH
   is stored as the psalm's title, and 21 verses end with a stray letter, so
   they reach search, sharing, copying, the website and speech. The NKJV
   decoder already drops exactly this. Needs epochs for web and webc.
3. The NKJV reads "call His name JESUS" in Matthew 1:21, Matthew 1:25, Luke
   1:31 and Luke 2:21, because the feed marks the name with the small-caps
   style the app uppercases for the divine name. Check a printed copy before
   changing anything; every other capitalised phrase in the canon is a
   genuine inscription.

The same document recommends a set of decode-time checks that cost no epoch
(the feed's own verse counts, the note reference field, a census of unknown
node and style shapes, and the words-of-Jesus cross-check), and sequences the
larger questions: source paragraph boundaries, section headings, and the
NKJV's italics for supplied words.

**Closed 6 October 2026.** `docs/SCRIPTURE_WORKLIST.md` records the three
defects and the decode-time checks as finished: S1 and S2 done, S3 closed
as not a defect, S4 to S9 done. What remains of the larger questions is
tracked there and in docs/ROADMAP.md: a setting to turn section headings
off (S15) and in-text footnote markers (S22). The WEB Catholic Daniel 3
heading, which S15's shape guard was to catch, is drawn as the publisher
sets it from 1.2.23, its note a footnote on the heading (docs/ROADMAP.md,
decision 8).

## NKJV Psalm superscriptions — DONE 5 September 2026

DONE. decodeAPIBiblePassage reads the `d` (descriptive title) paragraph into
BibleData.Superscriptions instead of skipping it: text and notes go through
the same walk as a verse's, the title is attached at the next verse marker
(on the passages endpoint it is read while the decoder is still in the
previous chapter), and a title left over at a chunk's tail is attached only
when its own verseId says which chapter. nkjv cacheEpoch 1 → 2. The
renderers and the footnote section already handled titles for every version,
so nothing else moved there. The feed only carries `d` with
include-titles=true, so the query now asks for titles and apiBibleSkipPara
drops the section headings and acrostic letters that come with them. The
same epoch fixes a join defect the note capture had introduced: a verse
opening with a cross-reference took the paragraph boundary's poem break in
front of its first word (1,995 poetry verses began with a blank line).
Pinned by apibible_superscription_test.go and apibible_note_join_test.go;
TestLiveAPIBibleProbe checks Psalm 3 and the 3→4 passage boundary, and
TestLiveAPIBibleFullCanon (opt-in, ~200 requests) downloads the whole
canon and reproduces the 2026-08-23 text byte for byte with 116 titles
added. What every source carries and what is kept: docs/SOURCE_FIELDS.md.

**Closed 6 October 2026.** The text above already said DONE; only the
heading lacked the mark. The decoder still reads the `d` paragraph into
`BibleData.Superscriptions` (`apibible.go`, 83e9bca55), and
apibible_superscription_test.go passes.

## Show a note you just sent, the way opening one from the browser does — DONE 27 September 2026

Sending a note stored it and drew nothing. A `noteKindMine` record joins a
chapter's plan only while `noteFocus` names it (`notes_plan.go`), and only the
notes browser and the reader's own link ever set that focus, so the send path
could not show one; `share.go` recorded that as a decision. The decision is
reversed: sending is the one moment a reader has no other sign that their
words were kept, and the browser is several taps away.

What shipped: Share on the note composer (`promptShareNoteWith`,
`share_note_ui.go`) shows the note it kept through `showSentNote`
(`notes_mine.go`), which does what opening an own note from the browser does
(`openNote`, `notes_browse.go`): a foreign mark stands aside, `focusNote`
names the record, `applyNoteForCurrentChapter` projects it with its own wash,
and `openedNotePlacesTheView` places the view on it. The reader sees the card
the browser opens, over the verses they selected, bylined "Note from you" and
carrying its one ✕, on the iOS, Android and macOS native panes and on the
Windows and Linux styled pane alike, since every one of them draws from the
same plan and the same chrome. Nothing new is stored. Focus is session state,
so the note goes on the next navigation (`addRecentChapter` resets focus)
exactly as a note opened from the browser does, and a window rebuild, which
builds the page from state and never touches focus, keeps it: a light/dark
switch made while the share sheet is up brings the card back in the new
palette, and one made while the composer is still open brings the composer
back on the passage it opened on, so Share on it shows the note all the same.

The chapter is the one on screen, so the repaint is a note verb's, not a
navigation's (`refreshNoteOnly`), and it is the only one: the composer closed
before the send, and nothing after it repaints the reading pane. Android and
the styled pane rebuild the reading pane. The Apple panes would push the
sticker and the wash onto the text they hold, but the body fingerprint folds a
mirror naming any note other than the chapter's display note (the mirror
clause in `chapterFingerprint`, `reading.go`), and an own note is never that,
so they refuse the in-place push and are rebuilt too, the chapter re-imported.
Whether that rebuild shows on an iPhone, an iPad or a Mac as the flash
`refreshNoteOnly` exists to avoid is for a device to say (V12); if it does,
that clause is where to look.

Every route to it is one closure: the composer's Share button and Return, on
the desktop card (macOS, Windows, Linux), on the phone sheet with the Fyne
field (Android) and with iOS's native field floated over it. Only the link
share carries a note; Share with citation, Share as image and Share as link
store nothing and show nothing. Where no link can be built (a book with no
slug, which no shipped book is) the note route falls back to the text share,
which carries no note, but the note was kept before that, so it is shown too.

The send is synchronous. `shareVerseLinkWithNote` (`share.go`) saves the
record, builds the link and hands the message to the platform before it
returns, and the OS share sheet is presented afterwards on its own and reports
nothing back. So a share sheet the reader cancels still leaves the note kept,
as it always did: the reader sees it on the passage and in the browser, though
it may never have reached anyone. The message is handed over before the note
is shown. On an iPad and on a Mac the sheet opens beside the selection,
measured on the main queue after the hand-off, and the note's card, the
chapter's re-import and the pane's new frame are all queued behind it on that
queue, which runs in order, so the sheet is placed against the text as it
stood. A note that was not kept (a store that stood the write down, or Share
with the field left empty) shows nothing and takes no focus, and a search
result the reader arrived on stays lit.

It is shown only on the passage its words were selected on. The composer
records the translation, book and chapter when it first opens (`notePassage`)
and carries them through a light/dark reopen, and `showSentNote` shows nothing
if the reader is elsewhere when it runs: a link arriving while the composer is
open moves the reader without closing it. The check is made when the show
runs, not when the note was saved, so it would hold for a send that completed
later. It also moves focus only to a note the chapter's plan draws: focus
naming a note that is not there falls to the default rule, which would reopen
a friend's note the reader had just closed on the chapter.

Over a search result, a link's span or the verse of the day's mark, the note
takes the page as a browser tap does: the foreign mark is cleared, as
`openNote`, a note chip and the pill's Show clear one, and the card opens
where the mark would have stood it down to its pill. The results trail
(`CanReturnToSearchResults`) is left as it was, since the reader has not moved.
No tint changed: that a note's mark and a search mark look alike is the item
below.

The comments that stated the old decision now describe this one: `share.go`
where it saves the record, the plan's `Own` slot and its walk in
`notes_plan.go`, the header of `notes_mine.go`, `noteForChapter` in
`notes_store.go` and `senderByline` in `notes_byline.go`. Both link shares now
deliver through `shareTextOut`, the seam every text share uses, and the iOS
field's text is read through `noteEntryTyped`, so a host test can drive every
route. The host tests are in `share_note_sent_test.go`: every route to Share,
the state it leaves and what a pane built from that state draws, and, in a
window built by `CreateMainUI`, that the Windows and Linux pane on screen is
repainted with the card, after a light/dark rebuild too and from a composer
the rebuild brought back. `share_note_sent_macos_test.go` holds that the macOS
pane is given the chapter with the note, its wash and its placement. What only
a device can confirm is in `docs/VISUAL_TESTS.md` under V12, and V14's
own-note row now names the send.

## A reader cannot tell why a verse is lit

Planned for: next major, behind `next` — see docs/ROADMAP.md.

The report that raised the item above was a misreading worth recording,
because the app invited it. A highlight was still standing after a send with
no note beside it, which read as a note that had lost its text; it was a
search mark, from arriving at the passage through Results. `hlOrigin`
(mark.go) records provenance but does not change the tint, so a note's mark
and a search mark are indistinguishable to a reader. A send no longer leaves
that scene (the note it keeps replaces the search mark), but the ambiguity is
its own item and stays open: a reader cannot tell why a verse is lit.

## A note is shared and filed against the chapter the reader is on at Share, not the one it was written on — DONE 28 September 2026

The note composer holds the words and the span selected on one chapter, but
`shareVerseLinkWithNote` read the passage from the reader's state when Share
was pressed: the citation, the link's translation, chapter and verses and the
stored record all took the chapter the reader was on then. A link arriving
while the composer is open navigates without closing it (`applyShareTarget`),
so the message then named the wrong passage and the note was filed on it,
where `showSentNote` (which checks the passage the composer recorded) at least
did not draw it.

The share now takes the passage the composer recorded when it opened
(`notePassage`: translation, book and chapter) and makes everything against
it. `shareVerseLinkWithNote` (`share.go`) is handed the passage by the
composer's send (`promptShareNoteWith`, `share_note_ui.go`) and cites the
selection through `prepareShareQuoteIn`, reads the link's verses through
`linkVersesForSelectionIn`, new beside `linkVersesForSelection`, builds the
link and files the record on that book and chapter in that translation
(`notePassage.version`, `notes_mine.go`), and, for a book no link can be built
for, shares the quote and citation of that passage (`shareVerseTextIn`, the
text share's body, which Share with citation now uses too). The sheet's words
and reference come from the same passage (`shareNoteQuote`), so a light/dark
reopen after the reader was moved still shows what Share sends; the reopen
already carried the recorded passage, and still does. The composer is not
closed on navigation: the reader may be halfway through a sentence. The words
are read from `state.Bible`, which is the recorded translation while the
composer is open: a translation switch rebuilds the window and the rebuild's
drain closes the composer, and the one rebuild that brings it back, a
light/dark change, switches no translation. The sent note is still shown only
on its own passage, so a reader moved elsewhere finds it in the notes browser,
filed where it was written.

The host tests are in `share_note_passage_test.go`. On the desktop card, the
phone sheet with the Fyne field and the phone sheet with iOS's native field,
the composer opens on Psalm 23:3–4 and a real link (`HandleShareLink`) moves
the reader to John 1 before Share: the sheet still shows Psalm 23's words and
reference, the message cites Psalm 23:3–4 in its translation, its link opens
Psalm 23 at verses 3–4 with the note, the record is filed there, and nothing
is focused on John 1. The same routes without the move share and file exactly
as before. A note on a book with no link path goes out as that passage's quote
and citation after the move, and a composer brought back by a light/dark
change after the move shows, shares and files the passage it first opened on.
The link's verses are found by locating the selected words in the passage and,
where they cannot be, from the span; a further case selects words within Psalm
23:3 and follows a link to a chapter that has the same words at verse 2, as
Psalm 108 repeats Psalm 57, and the link and the record still name 23:3.
Each was run against the tree before the change (the moved cases fail) and
against mutations: the send reading the reader's passage again, the composer
handing it the reader's passage, the record, the link's verses, the text share
or the sheet's words read from the reader's chapter, the words alone located
in the reader's chapter, and the reopen handing on the reader's passage each
fail a test. The link's verses read wholly from the reader's chapter fail the
first test only because its fixture John 1 has no verses 3 and 4; the
repeated-words case fails for either.

`showSentNote` still checks the passage the composer recorded, but on another
chapter the plan now refuses the note too, since it is filed on its own
passage: the check alone refuses it only on the same chapter in another
translation, which a send cannot reach while a translation switch closes the
composer. `share_note_sent_test.go` holds that case with a late show, and its
notes say which mutations the other cases catch. What only a device can
confirm is in `docs/VISUAL_TESTS.md` V12.

## One pill, several noted paragraphs: what the count says and where it points

Planned for: stays — see docs/ROADMAP.md.

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

Planned for: stays, decided with the next major's highlight work — see
docs/ROADMAP.md.

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

## Opening your own note hides every trace of everyone else's — DONE

DONE by side effect: per-paragraph note pills default on, which is what made
the other notes visible again. The description below predates that.

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

Planned for: stays — see docs/ROADMAP.md.

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

- `goto.go` (60ms inset scroll; 200ms self-rearming `watchDismiss`),
  `audio_menu.go` (150ms self-rearming watch) and `ai_ask.go` (the same
  150ms watch) — FIXED 29 September 2026. The sheet tests had begun opening
  the audio source menu on a test window and hiding it with a deferred
  Hide, and its watch, armed straight on a timer, read the popup's
  `Visible()` against that Hide and ran its close-out — the native reading
  pane's restore, which reads the appearance seam — on the timer's
  goroutine while the next test was writing the seam: `-race` reported it
  on a probe replaying the tests' sequence. The Ask sheet's watch was armed
  the same way under the appearance tests, and the phone tests that now
  open the Go to picker on the phone device would have armed its. All
  three go through `sheetAfter` (`sheet_timers.go`), which `holdSheetTimers`
  replaces to hold the closures; `desktopSheets` and the appearance harness
  hold it, a test runs a pass where it needs the close-out, and
  `TestSheetWatchdogsArmThroughTheSeam` holds each of the three armed
  through the seam and nothing running on a timer. The note composer's
  `noteSheetAfter` now goes through the same seam.
- `share_note_ui.go` (50ms slot push; 150ms self-rearming watch) — FIXED 27
  September 2026. The selected-words tests open the phone sheet, through the
  phone device wrapper, and a `-race -count=30` run of them with the arms on
  real timers reported the watchdog's `closeSheet`, run from one test's timer,
  racing the next test's open on `noteEntryOwner`. Both arms now go through
  `noteSheetAfter`, which the tests replace to hold the closures, running
  them on the test goroutine where a test needs them (the second push and a
  pass of the watch, in `TestTheNoteFieldSlotSitsBelowTheExcerpt`); the same
  run is clean.
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
- `state.go` (160ms repaint of the whole window after a search arrival,
  `openSearchResultRange`) — FIXED 6 October 2026. The audit missed it, and
  it was the cause of the verse-of-the-day card test's occasional failures
  (`TestTheCardSetsThePassageInTheReadingFace`, 30 September and 2 October,
  each a nil dereference inside Fyne: once in `theme.Padding`, once in a
  button's `MinSize`). `TestFullUIBuildsAndExercisesPaths` opens a search
  result on a real test window, and 160ms later the timer laid that window
  out on its own goroutine while the card test, the first heavy layout after
  it, laid out the card. Fyne keeps the theme a `ThemeOverride` lays out in
  on one global stack with no lock (`internal/theme`'s `PushRenderingTheme`
  and `PopRenderingTheme`). A pop sets the top entry to nil and then shortens
  the stack, so a read between those two writes takes a nil theme, and a
  method call on it faults at address 0x30, both crashes' address. The race
  run never saw it because `ui_render_test.go` is `!race`. A probe with that
  test's body and no tag, run with the card test under `-race -count=30`,
  reported 51 races between the timer's goroutine and the card test's, on
  that stack and on Fyne's font caches, and failed 7 of the 30 card runs. The
  repaint now goes through `arrivalRepaintAfter`, which TestMain shuts so it
  only counts arms (`arrivalRepaintsArmed`), as it does `upgradeRetryAfter`.
  The same probe is clean. One full run with a Fyne copy that logged every
  override theme pushed off a test goroutine found no other timer laying out
  during another test; the only other such pushes came from the API.Bible
  key test's request goroutine, inside the test that waits for it. `TestASearchArrivalRepaintsThroughItsDoor` holds the
  arm and runs the repaint on the test's goroutine, and
  `TestTheSuiteHoldsEveryArrivalRepaint` shows nothing repaints on a timer;
  both fail with the arm put back on `time.AfterFunc`.

The audit's rule, kept: a gate lands only with a demonstrated race (a
`-race -count=30` loop that fails before and is clean after) — no
prophylactic gates over dead code.


## `fyne package` bumps the desktop Build ledger after every package — DONE 6 September 2026

DONE. Both packaging paths now save the ledger before packaging and restore it
afterwards, which is the pattern `build-android.sh` already used. The store
script restores it in the same EXIT trap that restores go.mod, so a build can
no longer leave an uncommitted bump in the tree. The release workflow restores
it between the two macOS architectures and again at the end, so both downloads
carry the committed Build rather than the second one carrying a number nobody
chose, and it then unzips both and checks their CFBundleVersion against the
ledger rather than trusting the restore. `test-release-key-flow.sh` asserts all
of it and fails if a restore is removed.

**Closed 6 October 2026.** The text above already said DONE; only the
heading lacked the mark. The fix is a2ad17881 (6 September 2026), and
`scripts/test-release-key-flow.sh` still asserts the save and both
restores, in `release.yml` and in the store script's trap, and passes.

## The verse wash was measured in two conventions — FIXED 2026-09-08

`Layout.getHorizontal` never applies justification on any release from 26 to 36,
while the line-level extents (`getLineLeft` / `getLineRight`, through
`getLineExtent`) do. WashSpan took one edge from each: line extents where the
verse reached a line edge, `getPrimaryHorizontal` where it stopped mid-line.

So on a justified line — Android 15 and newer, which is where the pane justifies
— a verse ending mid-line had its right edge measured as if the line were ragged,
which is left of where the glyphs are. The full stop closing the verse fell
OUTSIDE the wash. Verified against the ragged Android 13 build, where the two
conventions agree and the same verse washed correctly; that control is what ruled
out the obvious alternative, that the verse range simply excluded the period.

Fixed by reconstructing the platform's own INTER_WORD arithmetic from public API:
`TextLine.justify` spreads (target − unjustified) over the line's stretchable
spaces, so the drawn x of an offset is its blind x plus one share for every
stretchable space before it (`drawnHorizontal`, android/BtBridge.java). It is
self-disabling — on a ragged line the extra comes out at or below zero and the
blind value is returned unchanged, which a pixel diff of the Android 13 build
confirms at zero differing pixels.

The selection popup's anchor had the same blind measurement and is routed through
the same helper.

## Android before 15: the reading text cannot be justified while it stays selectable — BY DESIGN (8 September 2026)

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

Ways back, if ragged text before Android 15 ever matters enough: a custom
`TextView` that draws its own justified lines from a `StaticLayout` while
keeping a selectable `DynamicLayout` for hit-testing (two layouts, one text);
or reflection on the protected `Layout.setJustificationMode`, which the
hidden-API policy may refuse. Neither is worth it for a rendering that
Android 15 already gets right.

**REVISITED 2026-09-08, and the gate STAYS.** Two things were established and
then weighed the other way round from the sentence above.

The fleet share is worse than that sentence assumes: Android 15 and 16 together
are roughly a quarter to a third of active devices, so justification is the
MINORITY rendering here and will be for years. Levelling everything down to
ragged was therefore considered — it would have deleted a split in a pane that
already forks once on API 29 for the reading face, and ragged is defensible at a
phone measure (the justified page's word spaces vary nearly threefold from line
to line, the rivers a narrow column always gives).

It was rejected, and the principle is worth keeping: **give each platform the
best rendering it can manage, rather than holding everyone to what the weakest
can do.** A reader on a capable release should not get a worse page so that it
matches an older one. Ragged stays where the platform cannot do better, which is
what the gate already expresses.

So: the gate is correct, neither workaround above should be built, and the
inconsistency between fleet bands is accepted rather than overlooked.

**Closed 6 October 2026 as by design,** on the 8 September decision
above (4a3b3de68). The gate stands: `justifiesInterWord`
(`android/BtBridge.java`) justifies only from API 35. A deliberate
difference between platforms is recorded where the platforms are
compared: it is divergence 30 of docs/PLATFORM_MATRIX.md (6 October
2026), which `TestTheMatrixRecordsTheRaggedAndroidPage` holds there while
the gate stands.

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

## A shared verse carries the app's own small capitals, and then cannot find itself — FIXED, and DECIDED 22 Sep 2026

**The locate half is fixed and the send half is an open preference.** 1.2.13
(cda1575b7) routed the share path through `outboundText`, and a later change
(463da7cd8) made the marker strip, the heading retry and the legacy fallback
compare in that same outbound form, so a whole-verse drag of a divine-name
verse now locates and cites correctly. What a share SENDS followed the rule
below: `LORD`, capitals, the plain-text convention.

The account holder has since said they prefer the shared text to keep the
small capitals as drawn — `Lᴏʀᴅ` — having seen it in a message and liked it.
That is a coherent choice and not the defect this entry first described: the
defect was that carrying `Lᴏʀᴅ` also broke the LOCATE. The two can now be
separated — locate in the outbound form, as the pipeline does, and emit the
drawn form in the text that leaves. The cost to weigh: a pasted `Lᴏʀᴅ` is
not found by a search box and renders unevenly in fonts without the
small-capital block.

**Decided 22 September 2026: the drawn form.** Everything a reader sends or
copies — the text share, the image card, the verse of the day, the styled
pane's Copy and the chapter copy icon — keeps `Lᴏʀᴅ`; only an AI request gets
capitals. The pipeline now meets in the drawn form rather than the capitals
(`sharedText`, `verseSharedText`), which turned out simpler than locating in
one form and emitting another. docs/DIVINE_NAME.md section 4 is the account.
The original entry follows as the record.

Found while checking whether a sentence in docs/ADDITIONS_AND_DROPS.md was true.
It was not, and the reason is a live defect.

`outboundText` (outbound_text.go) is the cleaner that strips the app's own
typography from text on its way out — superscript verse numbers back to digits,
the no-break join back to a space, the paragraph indent dropped, and the drawn
small capitals mapped back. It has exactly two call sites: `ai.go:266` and the
styled pane's Copy at `reading_styled_select.go:390`. **Share is not one of
them.**

The share path takes `plainSelection` (reading.go:1409) instead, which is
`cleanCopy` plus superscript-to-digit and nothing else, and then
`stripVerseMarkers(collapseSpaces(raw))` at share.go:435. Neither maps a small
capital back. Two consequences, on any verse the edition sets in small capitals
— which is the whole point of the NKJV divine-name work:

1. The shared text carries the substituted characters. A reader pastes `Lᴏʀᴅ`
   where every other edition and reader gives `LORD`.
2. `normalizeShareSelection` then looks for that text inside `chapterProse`
   (share.go:260), which is built from `Verse.Text` — the publisher's own
   letters. `Lᴏʀᴅ` cannot match `Lord`, so the selection is not located and the
   share drops to the legacy probe-based citation path.

The fix is probably to route the share path through `outboundText` as the AI
path already does, but the locate is the part to think about: `chapterProse` is
publisher text, so whatever the share carries has to be in those letters by the
time it is matched. Worth a test that shares a divine-name verse and asserts
both the outgoing text and that the selection was located rather than fell back.

## Three comments that describe code that changed under them — DONE 6 October 2026, for 1.2.20

Each verified against the source; all are comments, none change behaviour.

- `outbound_text.go:61-62` says the small-capital branch maps "Back to the
  publisher's own letter". It does not. `smallCapitalToLetter`
  (small_caps_draw.go:47-62) maps to the CAPITAL, deliberately, and gives the
  reason two lines down: a small capital does not record its original case, and
  uppercase is the conventional plain-text realisation that keeps the
  Tetragrammaton distinct from Adonai. The call-site comment contradicts the
  map's own reasoning.
- `dev_mimic_test.go:72` says a Mac, a Windows machine and a Linux machine "all
  draw the same Spectral" while the assertion two lines below requires
  `Junicode`. The face changed; the sentence did not. `dev_mimic_on.go`'s file
  header still lists "and font candidates" among the seams, though that seam is
  gone from the tree.
- `app.go:508-511` carries a duplicated fragment from the same commit that
  retired the font seam: "Must run before CreateMainUI (installSheetCloseConsume
  reads a seam) and / (installSheetCloseConsume reads a seam)." The sentence
  never completes. `dev_mimic_on.go:56-57` has the same kind of damage.

**Done.** The first had already been put right: `outbound_text.go` now says
the small capital goes "To the CAPITAL, not back to the publisher's own
letter", with the reason. `dev_mimic_test.go` says Junicode, the face its
assertion requires. `dev_mimic_on.go`'s header no longer lists font
candidates among the seams. The broken sentences in `app.go` (`Run`) and in
`dev_mimic_on.go` (`devApplyMimic`) now say once that the mimic must run
before `CreateMainUI`, whose sheet-close consume reads a seam it sets.
Comments only, so there is no test to fail; `go vet -tags bibletextdev`
compiles both tagged files.

## docs/ANDROID.md and reading_android.go disagree about the note sticker — DONE 6 October 2026, for 1.2.20

docs/ANDROID.md:183-185 says the Android shared-note sticker "is gated to
`IsFullScreen` in Go (`pushNoteToOverlay`, reading_android.go) precisely so the
reader never sees the note twice." The function it cites opens by saying the
opposite: "BOTH reading modes now, not full-screen alone (settled by comparing
the two platforms side by side) ... Same sticker, both modes, and the Fyne
banner stands down" (reading_android.go:615).

The code is the newer of the two. Left alone rather than guessed at because the
sentence also asserts a reason — that the gate exists to prevent a double
draw — and whether that concern is now handled by the banner standing down, or
was simply overtaken, is worth confirming before rewriting it.

**Done.** Confirmed from the code: the concern is handled by the banner
standing down. `pushNoteToOverlay` pushes the sticker in both modes, and
`buildNoteBanner` returns nothing wherever `nativeNoteSticker` is true, which
on Android it always is (`nativeNoteStickerOnPlatform`,
`notes_sticker_native_on.go`). So the note is drawn once in either mode.
docs/ANDROID.md's section is retitled "Shared-note sticker" and says so. Its
example WHO line now reads "2 of 3 in this chapter", the count
`appleStickerPush` writes, in place of the old "2 of 3 on this passage".

## Target audience is 13+, and it was meant to include 9-12

Planned for: stays; its recovery breadcrumb built for 1.2.23 on
`fu/1223-key-gone` and merged into `rel/1.2.23` on 22 October 2026 — see
docs/ROADMAP.md, 1.2.23, row 17 (Later minor releases, row 10).
The 23 October 2026 revisions are integrated locally in `rel/1.2.23`,
uncommitted: iPhone, iPad and signed Mac builds use Android's same
definitive-missing-key condition, a saved mark, an item-not-found answer
and no remaining Preferences copy. The field says "Your saved key isn't
available on this device. Paste it again here." It does not infer a
restore or assign a cause. PLATFORM_MATRIX.md divergence 38 is resolved.
Apple keeps the per-account dismissal ThisDeviceOnly: it may survive a
same-device backup restore but does not migrate to another device.
Android's `no_backup/` record is lost after reinstall or reset.

The Play target-audience declaration was saved as **13-15, 16-17, 18 and over**.
That is not the intended answer. The intent was to include **9-12** so the app
is offered to children as well, and the content rating supports it: ESRB
Everyone 10+ and PEGI 3 put the floor at 9, not 13.

It was set to 13+ deliberately, to get the app-content checklist finished.
Including any band under 13 turns the app into a Families-policy app and adds a
step the console will not let anyone else complete: a legal certification that
the app, including all APIs, SDKs and ads, complies with COPPA and GDPR. That is
the owner's to sign, and only the owner's.

Nothing is lost in the meantime. The rating is unchanged, so a child can still
find and install the app; target audience governs Families-programme placement
and the policy obligations that come with it, not who may download it.

To change it: App content -> Target audience, tick 9-12, and sign the
certification at step 2 of 5. Note the selection is NOT saved until the wizard's
final Save, and leaving the page discards it, so it has to be finished in one
sitting.

Before signing, the two things a Families reviewer would look at are worth
having closed: the AI answer markdown vector (fixed) and the storage permissions
(fixed), plus the recovery breadcrumb for a missing saved key, built for
1.2.23 and not yet released. Its ✕ contains no new words. The 14pt glyph
and 24pt face are aligned with the line's first row, while the original
button minimum still sizes its row and column. An invisible mobile-only
target fits within that allocated column, capped at 44pt on each axis;
desktop has no extra proxy. No row growth, wider column or upward offset
toward Clear is introduced. This refines the earlier branch's target
geometry; its 22 October measurements do not validate the new geometry.
The saved mark and separate per-account dismissal hold no key or personal
identifier. The app has no ads at all, which is the single biggest
Families-policy failure mode, and the audit in docs/PLAY_LISTING.md
found no identifiers, analytics or crash reporting anywhere.
