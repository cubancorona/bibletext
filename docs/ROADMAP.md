# Roadmap

The plan as of **21 October 2026**: which release each open item in
[BACKLOG.md](BACKLOG.md) is planned for, in what order, and the decisions still
open. It is the plan, not a promise, and it is updated as releases go out: a
row leaves when its release ships, an item moves when a decision settles it,
and the date above changes with each update.

**This page holds the order and the decisions. It does not hold the detail of
any item** — that stays in the item's BACKLOG.md entry, and each open entry
there names the release it is planned for. Where this page and an entry
disagree about the detail, the entry is right and this page needs fixing.
A few rows are marked *not in BACKLOG.md*: small items found after 1.2.19
whose row here says all there is to act on.

Sizes: S is up to about a day, M a few days, L a week or more. Risk is shown
with the size. "All six" means iPhone, iPad, Mac, Android, Windows and Linux.

---

## How releases flow

- **Minor releases come from `main`, without the `next` switch.** Each one
  is cut from `main` by [RELEASING.md](RELEASING.md) when it is ready, and
  built without the `next` tag: the last release's behaviour plus its own
  fixes. A minor release never waits for the major one.
- **The next major version is built on `main`, behind `next`.** What the
  switch is, how work goes behind it, what keeps it out of every minor
  release, the few changes that keep a branch instead, and the steps that
  turn it into a release are in [NEXT.md](NEXT.md), and are not repeated
  here.
- **What goes where.** A fix to what readers already have goes in a minor
  release. Something new that readers will notice goes in the major, behind
  the switch. A change no build tag can hold — a toolkit or module version,
  the minimum OS, how stored data is laid out — keeps a branch for the major
  (NEXT.md, *What needs a branch instead*), or ships unswitched in a minor
  when readers need it sooner. The desktop navigation rule (Later minor
  releases, row 1) and 1.2.20's AI study and passage-preview additions are
  explicit exceptions.

---

## Released

A row leaves this page when its release ships; what it did is in its
BACKLOG.md entry.

### 1.2.22, 16 October 2026

1.2.22 (build 193) went out on 16 October 2026 from one tree, `v1.2.22`
at `ee9340f49`, on every channel: the App Store for the iPhone, the iPad
and the Mac, Google Play's production track, the Microsoft Store, the
Snap Store's stable channel, GitHub and the website, where the halves of
its rows 8, 9, 15 and 16 that the website carries went live with it. It carries
the 31 rows prepared on 15 October 2026 and one made on the day it went
out, before the tag: the Android bridge compares the Activity across
resumes by its Java object (row 32). Its rows are kept below, under
"1.2.22, released 16 October 2026", until they are folded into their
BACKLOG.md entries.

### 1.2.21, 13 October 2026

1.2.21 (build 192) went out on 13 October 2026, the day 1.2.20 did, with two
changes of its own and no new feature. That day it went live on GitHub, the
Snap Store, the Microsoft Store and the website, was submitted to the App
Store for the iPhone, the iPad and the Mac, and was promoted to Google Play's
production track; at the App Store and Google Play it was still in review when
this page was last updated, and the day each store approves it is added here
when it comes. The Apple privacy manifests declare no collected data types
again, as they did up to 1.2.19: Study with AI sends a reader's text from the
device straight to the provider the reader chose, under the reader's own key,
and nothing reaches the project. The required-reason declarations stay, the
manifests carry no comments, and the App Store's privacy answers did not
change. And Android 7.0 (API 24) became the minimum for the Play bundle and
the APK alike, which keeps Google Play's installer protection on, with the
package checks verifying every signature scheme across the supported range.
1.2.21 also carries 1.2.20's changes to the App Store and Google Play, where
1.2.20 itself was never submitted, so they reach readers there when it clears
review.

### 1.2.20, 13 October 2026

1.2.20 was the release this page planned on 6 October 2026, its 21 rows done
but one: the verse cited for a drag begun on the previous verse's full stop; a
drag from a psalm title quoting only the psalm; no hyphen in a psalm title on
the Apple apps and Android 15 and later; narration that plays on when Android
destroys the app's screen; the Android sheet and spacing fixes; the verse of
the day's Share on the Mac and the iPad; Compare perspectives and Ask a
question in Study with AI, with shared passage previews; the iOS rendering
boundary during app switching and font tables shared across study cards;
guards in the release tools; documentation and tests. It went out on GitHub,
the Snap Store, the Microsoft Store and the website. It was not submitted to
the App Store, whose 1.2.20 builds declared collected data types, nor to
Google Play, which refused its upload because Play's installer protection
needs Android 7.0 and 1.2.20 still supported Android 5.0; 1.2.21 carries it to
both. What its rows left open has rows of its own: the Windows Store checks
and a look at the Windows fallback for a card that could not be read (rows 11
and 1 there; Later minor releases, row 40), the selection's place in its
verse and the digit a drag begun inside a verse number shares (row 6; 1.2.22,
row 11), the dash end repair (row 6; 1.2.22, row 17), the BSB heading repair
(row 7; 1.2.22, row 13), Share with note's popover arrow (row 9; 1.2.22, row
14), Back on Android 11 and older and the checks on hardware its study rows
left (rows 4 and 18 to 21; Later minor releases, row 42) and two stale
comments (row 16; 1.2.22, row 21). The stylesheet comment row 8 left was
corrected with 1.2.22's row 8.
Rows 10 and 18 to 21 have no entry of their own; their outcomes are kept in
BACKLOG.md, "1.2.20's roadmap rows with no entry of their own".

---

## 1.2.22, released 16 October 2026

Ten fixes to where the app places the reader, about seven working days of
work, approved as a set on 13 October 2026, and an eleventh, the selection's
place in its verse (row 11), moved in from Later minor releases on 14 October
2026 with the digit a drag begun inside a verse number shares. Each is a place
where the app works out the verse, the paragraph or the scroll position by
searching the text or the page when the surface already has the position, or
could carry it cheaply.
Each of the ten is in the shared Go, the Android bridge with its Go half, or
the website's reader script, and none touches the Apple panes' Objective-C;
row 11 changes every pane's seam, the Apple panes' included. None changes
stored data or makes a reader download a translation again. Each row's detail
is in "Where the app re-finds the reader's position from text" (BACKLOG.md),
under the label the row gives (AND-2, AI-1 and so on). Lines are at
`672086d41` (v1.2.21) and unchanged at `6bf27aee4`; sizes are working days,
tests and platform checks included.

The rows are in order of what a reader gains per day of work, so if the time
runs short the last rows move to the next release first. Rows 8 and 9 reach
readers with the next `scripts/publish-site.sh`, whenever that runs. All
eleven are done; where each stands is after the table. On 14 October 2026
more rows followed them (rows 12 to 29, after that): the defect the device
checks found under row 2; the bugs among Later minor releases rows 26 to 44
that need neither a decision nor new wording, AND-4 among them; two older
defects in the same share repairs; and, in a second pass, the bugs among
Later rows 1 to 23 and Stays in the backlog that could be fixed and shown
here, with the reason each of the others stayed.

What shipped: 1.2.22, build 193, prepared on `main` on 15 October 2026 and
released on 16 October 2026 from `v1.2.22` (`ee9340f49`), carries rows 1
to 32. Rows 1 to 11 are the set; rows 12 to 29 came on 14 October 2026;
row 30, the ChatGPT faster-model fix, and row 31, the guard on the GitHub
release notes, on 15 October 2026; and row 32, the Android Activity
compared across resumes, on 16 October 2026. Rows 8 and 9 and the website
halves of rows 15 and 16 reached readers with the site's publish that
day, and rows 10, 19 to 21 and 31 change nothing a reader of the release
sees. AI-4 and P3 (Later, rows 27
and 28) are not in it: they went to 1.2.23 (Decisions still open, 2).

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | Android opens a note's band, card or pill inside a poem's stanza (AND-2) | On Android, a note on a verse below its stanza's first line opens its band, card or pill above the stanza, as on every other surface, instead of in a gap it opens between two of the stanza's lines; a link to such a verse scrolls to the band. | S–M, 1.5 days / medium | Android | None | `paraStartAt` (`android/BtBridge.java:3004`, with an inline copy at :3488 for a link's arrival) finds a paragraph by walking back to the previous line break, and the Android page writes each poem line as a line break of its own, so in poetry it finds the line. 6,106 of the WEB's 31,098 verses (19.6%, 452 chapters) and 6,252 of the BSB's 31,086 (20.1%, 835 chapters) would open inside their paragraph; Psalms, Job, Proverbs, Isaiah and Jeremiah most. Fix: Go pushes the paragraph's first verse with the band specs (`notes_chrome.go:319`), the sticker (`reading_android.go:694`, :733) and the arrival (`notes_arrival.go`), and the five `paraStartAt` calls (`android/BtBridge.java:2423`, :2487, :2505, :2960, :2974) and the inline walk take `verseRange(firstVerse)[0]`. Checked with `scripts/check-android-pane.sh`, `scripts/check-android-java.sh` and a noted psalm, with a link into its stanza, on the emulator. With row 2, the two fixes here a reader is likeliest to see. |
| 2 | Android restores the reading position by a fraction of the scroll range, not by verse (AND-3) | On Android, turning the phone, entering split screen, changing the text size or returning to the app keeps the verse that was at the top of the screen, instead of moving it (John 3 at verse 3 came back at verse 2; Psalm 119 at 7 at 8). | S–M, 1.5 days / medium | Android | None | `captureReadingAnchor` (`reading_android.go:923`) returns only the bridge's fraction of the scroll range (`android/BtBridge.java:1733`), which leaves out the viewport's height, and the bridge re-applies it at :3528, :3543 and :3600, so any change of content or viewport height moves the top verse. A rotation moves it in 32–49% of positions, in 97–98% of chapters; a text-size change in 58–68%. Fix: capture the top verse and the offset into it, as a fraction of the verse's own height, from the layout the bridge already has (`verseAtOffset` of the top line), restore verse-first as iOS does (`reading_ios.go:3073-3085`) and keep the fraction as the fallback; `readingState` already carries `AnchorVerse` and `AnchorDelta`. The emulator checks: both rotations on a phone with a camera cut-out, and a text-size change. iOS keeps its offset into the verse in pixels, the shape that overshoots a verse that got shorter, not measured there; taking the same fraction is Objective-C, which this set leaves alone, so if this ships first PLATFORM_MATRIX.md records the difference until iOS takes it (BACKLOG.md, AND-3). |
| 3 | The fixed study actions resolve their passage again on every request (AI-1) | Explain, Analyze context, Analyze translation and Compare perspectives keep the passage selected through Try again, the faster-model switch and a light/dark change. Today, once a link or the narration has changed the chapter under the open panel, they cite the same verse numbers in the new chapter over the old selection's words. | S, 1 day / low | All six | None | Only Ask captures its passage (`ai_panel.go:63-64`, :608-610); for the others `runAIAction` resolves it again against the chapter then current (`ai.go:471`), and so does the reopen (`ai_panel.go:97-103`), so the prompt, cache key and header can name one passage and the Report mail another. When the chapter moves, 24,921 of the WEB's 31,077 whole-verse selections (80%) cite a specific wrong verse and the rest the wrong chapter. Fix: capture for every action as Ask does; `runAIAction`'s fallback becomes a test seam, and `TestStudyPanelWrapperCapturesOnlyAsk` flips. Go only. |
| 4 | A selection the positional path declines borrows another copy's ending (D, the interim) | A drag begun or ended inside a word, or around a mark, that the app cannot place is quoted with its own ending: "oul" dragged inside "would" in Genesis 2:19 stops being shared as "[O]ul." with verse 7's full stop. The citation is already right. | S, 0.5 day / low | All six | None | On a decline (`share.go:457-458`) the legacy path finds the fragment by its first copy in the whole chapter (`share.go:1263`; also :1954 and :637) and takes that copy's sentence end and omission mark. Of the WEB's 1,151,120 one-word drags begun or ended inside a word of four letters or more that take this path, 117,933 (10.25%) ship a different quotation, 115,019 of them from another verse; the BSB 9.52%. Interim: search only the span's region, as `locateShareSelection` does, which removes 97.5% of them. The root is row 11; D alone counts 3 days, interim and root. |
| 5 | A verse number that ends another verse's number is stripped from inside it (E, AI-3) | A drag begun on a later digit of a verse number stops losing the next verse's number from inside it: "14 “You shall not commit" stops being shared as "1“You shall not commit" (WEB Exodus 20:13–14), in the share and in the Ask sheet's and the answer panel's preview. | S, 0.5 day / low | All six | None | `cleanQuoteTextIn` (`share.go:415`) removes each verse marker with a 12-rune text probe and `strings.ReplaceAll`, so a verse whose number is a digit-suffix of a later one's, with the same opening, eats the later token. 339 verses are prone (193 WEB, 146 BSB), and 5,440 of 9,724 drags that reach this path come out mangled. Used at `share.go:457` and `ai.go:189`. Fix: the located path's boundary-checked strip, `stripVerseMarkersProse` over `chapterProseIn`, right on all 62,184 verses; one change for both surfaces. |
| 6 | The cross-references panel places a selection with no span by its opening words (B, tier 1) | On a page with no verse position, chiefly the Apple plain-text fallback, the Cross-references panel names the verse the share card names: Psalm 136's refrain pulls one verse's references, not all 26 verses' (40 rows, the cap), and John 3:3 stops naming 3–11. | S, 0.5 day / low | All six (on such a page) | Taken 13 October 2026 with this set: the panel takes the card's order. It closes PLATFORM_MATRIX.md divergence 33 (as numbered then). | `crossRefsForSelection` (`crossrefs.go:1182`) never normalizes and goes straight to a zero-span probe of each verse's first 24 or 8 runes (`crossrefs.go:1429`). On that page 1,661 of the WEB's 31,098 whole-verse drags (5.34%, 380 chapters) and 1,350 of the BSB's (4.34%) are over-attributed. Tier 1: normalize first and probe only on a decline, as the card and the prompts do; the verse is then right for 94%, and the controls in `ai_test.go:589-606` flip. Tier 2 is the fallback's geometry (Later minor releases, row 28). |
| 7 | Study with AI takes a zero-span probe's range as the passage (AI-2, tier 1) | On the same pages, a study request for part of a long word cites the chapter alone, instead of a range of verses the reader did not select ("Romans 3:5–26", "Psalms 119:6–176"). | S, 0.2 day / low | All six (on such a page) | None | When the normalize declines on a zero span, `aiPassageFor` takes `selectionVersesIn`'s answer as the range (`ai.go:198-199`), so one partial word of 8 runes or more names every verse holding the longer word. On that page 15.3% (WEB) and 15.9% (BSB) of selections reach the probe, and 43.5% and 40.0% of those name verses the selection was not made in. Tier 1: take the probe's answer only when it names one verse, and otherwise cite the chapter, as the comment at `ai.go:178` says. Tier 2 is Later minor releases, row 28; row 11's lead does not reach a page with no span. |
| 8 | The website does not land on a verse whose link carries a note it cannot read (W3) | On the website, a link to a verse that carries a damaged or newer-format note lands on the verse with the notice beside it, instead of at the top of the chapter with both off-screen. | S, 0.5 day / low | The website | None | The fragment is a key list (`#v16&n=…`) that matches no id, so the browser cannot scroll to it; `highlightRange` scrolls only a range (`cmd/websitegen/assets.go:832`), and `showNotice` (:1715) never scrolls, where NOTE_WIRE_FORMAT.md says the passage opens in all three cases. BSB John 11:35 with a damaged note stays at the top, the verse 3,954 px down; across 2,109 verses the verse is off-screen for 87.8% and the notice for 50.9%. Fix: scroll the already-resolved verse for any fragment but a bare `#v<lo>`, and have `showNotice` end with `rescrollToHighlight`, which takes the notice as it takes the card. The same publish corrects the stylesheet's comment on the Psalm title (`p.pst`), which still says the Apple apps and Android hyphenate the title and Android from API 35 justifies it (1.2.20, row 8). |
| 9 | The website re-anchors a note to the previous link's paragraph (W1) | On the website, Back or Forward to a note link with no verse, a second note link pasted into the same tab, or a fragment edited by hand parks the note at the top of the chapter, instead of beside the paragraph the previous link lit, with a tail pointing at it. | S, 0.5 day / low | The website | None | `anchorToPassage` (`cmd/websitegen/assets.go:1643`) falls back to `noteAnchorPara`, which it sets and nothing resets, so a fragment that lights no verse puts the note before the last-lit paragraph and scrolls there; on 95–99% of pages that paragraph is not where the note should park. Fix: `anchorToPassage` takes the current fragment's paragraph from its caller and parks without a tail when there is none; `noteAnchorPara` becomes a value `showNote` sets for `minimizeNote` alone. Ships with row 8. |
| 10 | The legacy Entry pane hands Go no verse position (G) | Nothing: the pane ships in no build. Switched back on, its shares and cross-references would stop naming other verses (Psalm 136:20's refrain shared as 136:1; a cross-references header naming 26 verses). | S, 0.5 day / low | Windows, Linux (the dead pane behind the styled pane) | Taken 14 October 2026 with the set's merge: the span, as planned (Decisions still open, 1). | `chapterText.menuForSelection` passes `selSpan{}` (`reading.go:1559`) while the cursor offset is at hand (:1466-1492), so every verb takes the zero-span path; switched on (`reading_styled_platform_on.go:8`), 0.34% of whole verses and 6.2% of drags of a verse's last four words cite another verse. Fix: compute the span from the cursor offset and `verseLines`, recording each verse's offset in `rewrap`; or delete the pane. |
| 11 | Every pane drops the selection's place in its verse (A, IOS-SEL-OFFSET-DROPPED, MAC-SEL-OFFSET-DROPPED, AND-1; in "Where the app re-finds the reader's position from text", P1), with the digit a drag begun inside a verse number shares | A word or phrase that stands more than once in a verse is shared, noted and quoted from the copy the reader selected, with that copy's ending and line breaks: the second "Abraham!" of Genesis 22:11 stops being quoted as "Abraham . . . !". A drag begun on a verse number's later digit stops carrying the stray digit into its quotation ("39 Wh"). The citation is already right. | L, about 5 days / medium | All six | Moved into 1.2.22 on 14 October 2026, as one change on all four panes (Decisions still open, 3 and 4). Until then Later minor releases, rows 24 and 25. | Each pane now sends, with the span, the selection's lead: its own text of verse lo from the first character of the verse's number to the selection's first character (docs/SELECTION_OFFSET.md), checked byte for byte against the verse before it is used, so a wrong lead costs the improvement and never the answer the selection gets without one. Go places the selection there (`placeByLead`, `walkSelectionFrom`, `share_lead.go`); the seams are `selection_span.go`, `reading_ios.go`, `reading_macos.go`, `ai_menu_darwin.go`, `android/BtBridge.java`, `reading_jni_android.c`, `reading_android_export.go` and `reading_styled_select.go`. Without a lead every path is the one the rows above leave. It closes row 4's root and the digit, and Later row 26 where the walk from the lead succeeds; it closes the two of the dash end repair's four that carry a span, as the phrase sweep run with the lead showed on 14 October 2026 (row 17 takes the other two). It does not carry the chapter (Later row 27) and does not reach a page with no span (Later row 28). Made on `fu/selection-offset` from `6bf27aee4`. |

### Where each row stands, 14 October 2026

All eleven rows were made on 13 October 2026 and are on the release branch,
`rel/1.2.22`, row 11 merged from its own branch on 14 October 2026. FIXED
means the change is in with a test that fails on the tree before it
(`6bf27aee4`); what is left is said beside it. Nothing here changes stored
data, makes a reader download a translation again or adds wording.

| # | Outcome | What was done, and what is left |
|---|---|---|
| 1 | FIXED; seen on the emulator | Go pushes, with the band specs, the verse that opens each of the page's paragraphs (`androidParagraphOpeners`, a third array to `setNoteBands`), and `paragraphStartOf` walks from that opener at the five places and in the arrival's scroll, so a band hangs above the page paragraph, not a poem line. `TestAndroidBandsHangAboveTheParagraphNotThePoemLine` and `TestBtBridgeReadsTheParagraphFromThePushedOpeners`; the JNI descriptor contract holds the widened signature. `scripts/check-android-pane.sh` in both switch states and `scripts/check-android-java.sh` compile it. Seen on 14 October 2026 on the emulator (Pixel 7, API 35, the WEB's Psalm 121): notes on verses 5 and 7 hang one card, and their pill, above verse 3's stanza, a note on verse 2 above verse 1, and a link to verse 5 scrolls to the stanza. |
| 2 | FIXED on Android; seen on the emulator | The bridge keeps the verse at the top, how far into its band the top stands as a fraction of that band, and the scroll fraction (`lastAnchor`, `getScrollAnchor`), and the restore, the chapter's placement and the width reflow place the verse first and the fraction only where the text does not hold the verse. The saved format is unchanged. `TestAndroidKeepsTheReadingPositionByVerse` and `TestAVerseAnchorKeepsTheTopVerseWhereAFractionMovesIt`, whose model keeps a verse of the top line at every position short of the end of the range, where the fraction, the control, moves it at 39 to 120 positions of each change. Seen on 14 October 2026 on the emulator (Pixel 7, API 35): four rotations and the Large, Extra large and Normal text sizes kept Psalm 119:61 within 0.002 of its place, where the fraction alone would have moved the view 286 to 365 px. After a link's arrival the position stayed at the arrival point through rotation and relaunch, an older defect that defeated this one; row 12 fixes it. The iPhone, iPad, Mac and the Windows and Linux pane keep the offset into the verse in pixels; PLATFORM_MATRIX.md records the difference as divergence 33 until they take the fraction. |
| 3 | FIXED | `showAIPanel` captures the passage for every action, as Ask's composer does, and the prompt, cache key, header, reopen and Report read that one capture. `TestFixedStudyActionsRetainTheirPassage`; `TestStudyPanelWrapperCapturesOnlyAsk` became `TestStudyPanelWrapperCapturesEveryAction`. Shared Go, so all six. Seen on 14 October 2026 on the iPhone simulator, with no provider called: Explain on John 3:16, the reader moved to John 4 under the panel, and Try again still names John 3:16, where resolving it again gives John 4:16. |
| 4 | FIXED (the interim) | A declined selection is placed inside its span's verses before the completion, the retained terminal and the line-break restore read it, so they take their positional branches. `TestDeclinedSelectionIsPlacedInItsSpan`. It leaves the repeats inside the span's own verses, 0.95% of those WEB drags and 0.85% of the BSB's, which row 11's lead places. A piece of a word run on into the heading below its verse ends with that verse (`a2a6aafca`, `TestAPieceOfAWordRunOnIntoTheHeadingBelowEndsWithItsVerse`), where the chapter-wide search quoted the heading's words: 6,651 BSB, 23 WEB and 31 WEBC drags without a lead. |
| 5 | FIXED | `cleanQuoteTextIn` runs the located path's strip, `stripVerseMarkersProse` over `chapterProseIn`, for the share and the AI preview alike. `TestFallbackMarkerStripLeavesALaterNumberWhole`. The leading digit of a drag begun inside a verse number is not a marker and stays without a lead; row 11's lead takes it off. |
| 6 | FIXED | `selectionVersesIn` normalizes first for the zero span too, so the panel's header and the verses it pulls references for agree with the card, the link and the prompt. `TestAIPassageRefAgreesWithShareCitation`, `TestSelectionSpanZeroFallsBackToMatching` and `TestSelectionSpanShortSelectionResolves`. The divergence it closed, then numbered 33, is gone from PLATFORM_MATRIX.md. |
| 7 | FIXED | The matching's answer is taken only when it names one verse, and otherwise the chapter is cited; the rule is in the shared resolver, so the card and the panel cite as the prompt does. `TestAPieceOfAWordWithNoSpanNamesOneVerseOrTheChapter`. The cost: 301 of the WEB's 19,421 and 324 of the BSB's 19,402 no-span drags begun on a later digit of a verse number and run into the next verse cite the chapter where they named those two verses. |
| 8 | FIXED | `highlightRange` scrolls the verse it lit for any fragment but a bare `#v<lo>`, and `showNotice` ends with `rescrollToHighlight`. `TestTheWebPageLandsOnAVerseWhoseLinkCarriesAKey`. The stylesheet's comment on `p.pst` is corrected (`745b0c35b`), and a minimized note's marker now lands on the note with one scroll (`TestTheNoteMarkerLandsOnTheNoteWithOneScroll`). Left: the next `scripts/publish-site.sh`. |
| 9 | FIXED | `renderNote` hands the note and the notice the paragraph of the fragment's own span (`fragmentParagraph`), and `anchorToPassage` parks with no tail when there is none; the minimized marker reads the note's own place (`notePara`). `TestTheWebNoteParksAtTheTopWhenItsFragmentLightsNoVerse`. Left: the next `scripts/publish-site.sh`. |
| 10 | FIXED (the span) | `rewrap` records where each verse's first token begins (`verseStarts`), and `menuForSelection` passes every verb the span `selectionSpan` reads from the cursor. `TestLegacyPaneSharesASelectionFromItsOwnVerse` and `TestLegacyPaneRecordsWhereEachVerseBegins`. The pane still ships in no build. |
| 11 | FIXED in Go and in every pane; seen on the iPhone simulator, the Mac and the emulator | Every pane sends the selection's lead with the span; Go checks it against the verse and places the selection there (`placeByLead`, `walkSelectionFrom`), and without a lead every path is the one rows 4 to 7 leave. With the lead nothing is placed wrong: 229,224 selections of one to three words over the embedded Gospels, with their 13,853 later copies and 123,177 pieces of a word (`TestEverySelectionInTheGospelsIsPlacedByItsLead`); the 90,264 drags of `TestSpannedBoundaryDragSweepWithLeads`, none of the 8,931 digit drags sharing a stray digit; 156,594 selections through the styled pane's layout; 23,567 drags into invented headings; and over the local WEB, BSB and WEBC caches, 6,722,046 selections, 3,611,457 pieces of a word, 6,973,409 drags begun or ended inside a word and 51,740 drags into the publishers' headings. `TestTheSelectionExportsMatchTheirCDeclarations` and `TestEverySelectionDispatchPassesTheLead` hold the native seams; `scripts/check-ios-pane.sh` and `scripts/check-android-pane.sh` in both switch states and `scripts/check-android-java.sh` compile them. Seen on 14 October 2026: the second "Abraham" of Genesis 22:11, the second "you" of Matthew 25:12 and a second word of John 3 shared from the reader's copy ("Abraham!", "[Y]ou.") on the iPhone 16 Pro simulator (BSB), the Mac (WEB) and the emulator (WEB), each byte for byte what Go gives for that lead. The phrase sweep run with the lead places none of the 6,332,213 phrases wrong; the two BSB phrases it left with no span are row 17's. |

### Added 14 October 2026: the defect the device checks found, and the deferred items that are bugs

On 14 October 2026 the items deferred to later minor releases were triaged
against `rel/1.2.22` at `10b6cbf19`, on `fu/1222-bugs`, in two passes. The
first took Later minor releases rows 26 to 44, with the decisions and the
BACKLOG entries they point to: every one of them that is a bug and needs
neither a decision nor new wording was fixed for 1.2.22 (rows 13 to 21),
with the defect the device checks found under row 2 (row 12), except AI-4
and P3, below, which go to 1.2.23 (decision 2). Row 15 had a regression,
put right in the same row, and the repairs of rows 13 and 17 two older
defects (rows 22 and 23). The second pass took the bugs in Later rows 1 to
23 and in Stays in the backlog that the first had left out, and fixed
those a test here can tell apart and a device or simulator here can show
(rows 24 to 29, with an older Mac defect that row 26's arrival had hidden,
row 27); what it left, and why, is after the table. Each row taken from
Later minor releases has left that table. FIXED means the change is in
with a test that fails on the tree before it. Nothing here changes stored
data, makes a reader download a translation again or adds wording. Rows
15 and 16 change the website too, whose half reaches readers with the next
`scripts/publish-site.sh`; their pinned digests moved with `reader.js` in
both states of the switch.

| # | Item (where it was) | What a reader notices | Platforms | Outcome |
|---|---|---|---|---|
| 12 | Android keeps a link's arrival point as the reading position (found by row 2's device check) | On Android, after following a link or closing Settings, a reader who scrolls on and then turns the phone or reopens the app keeps their place, instead of going back to where the link landed (a relaunch asked for John 3:13 landed at 11). | Android | FIXED (`b45fc17c0`, `74d1cd1f8`). The phone chapter header asks for more than a phone is wide, so the mobile driver's paint laid the reading host out wider than the canvas and then at its real width, and both frames reached the bridge: a reflow was armed that nothing applied, the scroll listener dropped every scroll until the next chapter, and a placement pending then was made at the wide width. Go now hands the phone panes only a frame that fits the canvas (`nativeHostFrame`; `TestAReadingHostWiderThanItsCanvasIsNotHandedToThePane`). The bridge re-places an arrival or a restore when the text lays out at another width straight after it, carries it into a reflow armed meanwhile, and drops a reflow whose width comes back before the text laid out (`TestAPlacementMadeAtOneWidthIsKeptWhenTheTextLaysOutAtAnother`, `TestAReflowArmedByAFrameThatComesBackIsDropped`). On the emulator with each case forced, the build before kept John 3:14 in 1 of 3 runs and dropped 62 scrolls in each of 3; the fix kept it in 3 of 3 and dropped none, and four rotations kept the top verse. The defect reached 1.2.21. |
| 13 | A drag from a heading into its verse (Later, row 36: the heading repair) | A drag from inside a publisher's heading through the next verse's number into its first words shares that verse's words, cited to it, instead of the heading as Scripture or the verse cut short ("The LORD Is My Shepherd The LORD", Genesis 4's "And" for "And Cain"). | All six (the BSB: 15,081 of 189,996 such drags under 682 headings; the WEB Catholic: 7) | FIXED (`7d304b96f`). The heading repair takes the front off first, then only a heading below that verse may lose its front from the end; a piece confirmed by the verse beneath comes off before the search; a stack of title and headings is one piece; a heading holding its own verse's number is confirmed after it. `TestADragFromAHeadingIntoItsVerseSharesTheVersesWords`, `TestAHeadingPieceRunOnToAVerseWithNoWordsQuotesNothing` and `TestTheHeadingRepairKeepsTheOtherDrags`; over the three local caches every such drag now shares its verse's words (`TestEveryDragFromAHeadingIntoItsVerseOverLocalCaches_LocalOnly`). The lead does not reach it: the selection begins inside the heading. |
| 14 | Share with note's popover arrow (Later, row 41) | On the iPad and the Mac, Share with note's popover points at the selected words, below the new "Note from you" card, instead of into the card above them. | iPad, Mac | FIXED (`dcccf211c`). The share carries the words' place in the pane's text, and both Apple panes present the sheet beside them after the re-import that puts the card on the page; without a re-import within a second, at the selection as before. `TestShareWithNoteIsAnchoredToTheSelectedWords`. Seen with development builds: the Mac's picker 0.8 pt below the words, the iPad simulator's source rectangle on them, against 88 and 89 pt above them before. The distance before is the new card's height, which follows the note's length: the 7 October 2026 check, with a shorter note, measured 52 pt. |
| 15 | A link to a verse the edition omits (Later, row 30: AND-4) | A link to a verse the open edition leaves out (Matthew 17:21 in the BSB) opens at the verse after the hole, where its "[21]" mark is, instead of at the top of the chapter with nothing lit. | All six and the website | FIXED (`24e9bb0ec`). Go resolves the arrival verse once (`drawnArrivalVerse`) for every surface; Android pushes it, the canvas pane falls back to the arrival verse's line, and the web reader lands an unlit link on the next verse. `TestALinkToAnOmittedVerseArrivesAtTheVerseAfterTheHole`, `TestTheCanvasPaneLandsALinkToAnOmittedVerseOnTheVerseAfterIt` and `TestTheWebPageLandsALinkToAnOmittedVerseOnTheVerseAfterIt`. A note on the omitted verse alone, whose card every surface parks at the chapter top, was then scrolled out of view on every app surface, while the website still landed on it; it keeps the arrival it had before, none, so the chapter opens at its top with the card in view (`49c472a21`, `arrivalVerseOnPage`; `TestANoteOnAnOmittedVerseKeepsItsCardInView`, `TestTheCanvasPaneOpensANoteOnAnOmittedVerseAtItsCard`). PLATFORM_MATRIX.md, divergence 34, records where a card shown from its link alone stands. Whether the mark itself lights, and where that card stands, were decided on 15 October 2026 (decision 18) and built for 1.2.23 (1.2.23, row 7). Seen on 14 October 2026 on the emulator (Pixel 7, API 35), the reader on the WEB, which omits Acts 8:37, with the NKJV switched off: `/nkjv/` links to 8:37, 8:37–38 and 8:38 land on verse 38; a note link on 8:37 opens at the chapter top with its card in view, where the build before the exception landed on 38 with the card out of view; a note link on 8:37–38 lands on its card above verse 38. |
| 16 | Four rules for a shared note's link (Later, row 35) | The app and the website read every shared note link alike: white space inside a payload is damage on both, a payload's ends lose ASCII white space only, the website cleans a note's text as the app does, and a note the app opens also opens on the website. | All six and the website | FIXED (`67f90c576`). 10 of 16 probe payloads decoded differently in the app and on the website; the four rules now hold in both decoders. `TestTheAppAndTheWebReaderReadNoteLinksAlike` runs the app, the shipped decoder under node and the Go mirror over the payloads a corpus line cannot hold, and `testdata/note_vectors.txt` gains append-only cleaning and empty-table vectors. |
| 17 | A phrase ending on a dash (Later, row 37: the dash end repair) | A phrase that ends on a closing dash keeps its last word ("defiled herself—" stops being shared as "defiled"; a drag ending on "If the gate is shut—" keeps "shut"). | All six (the BSB) | FIXED (`251e04633`). The end repair no longer cuts after an em dash. `TestAPhraseEndingOnADashKeepsItsWords`; the local phrase sweep's known count for the BSB falls from 4 to none. A drag cut inside a word at such a dash is row 22. |
| 18 | A Linux text share opens two mail composes (Later, row 2, in part) | Email… on Linux opens one compose, where `xdg-email` launched the mail program and then reported the launch failed and the `mailto:` link opened a second. | Linux | FIXED (`5165c3659`). `xdg-email`'s exit 4 now ends the share; exits 1, 2, 3 and 5, which refuse before launching anything, still hand on. `TestXDGEmailEndsTheComposeOnlyWhereItMayHaveOpenedOne`. Seen on 14 October 2026 on the Linux virtual machine (Ubuntu 24.04, GNOME on X11, xdg-utils 1.1.3) with stand-in mail handlers in place of a client: one named thunderbird, which `xdg-email` runs itself, composes and exits 1, and `xdg-email` exits 4 after its compose, where the `mailto:` link through `xdg-open` then opened a second; through `gio`, the path for any other client, `xdg-email` exits 0 once the handler is launched, whatever it returns, and with the default handler's program missing it exited 0 too, `gio` opening another. Exit 4 with nothing launched was not seen there. The trade-off: should another desktop's launcher fail with nothing opened and exit 4, the share now ends with no compose and no word, where the `mailto:` link was tried; that silence is the case row 2's approved line answers. The rest of row 2 waits on its wording (decision 7). |
| 19 | The versification generator moves a missing verse to a repeat (Later, row 34: VERS-1) | Nothing today: no shipped edition has the shape, and the tables are unchanged. | None until a regeneration | FIXED (`276929af1`). A move is refused when the candidate slot matches a verse the edition kept at least as well; the dropped verse is absent and the slot extra. `TestTheVersificationGeneratorDoesNotMoveAVerseToARepeat` runs the generator on invented editions of both shapes, with Romans' doxology as the control; `TestTheVersificationGeneratorReproducesTheShippedTables_LocalOnly` holds its output on the local caches to the BSB's and the WEB Catholic's rows of both shipped tables byte for byte. The NKJV's rows were not regenerated. Seeding moves from the SIL mapping, in the recorded fix, was not needed for the shape found. |
| 20 | The NKJV's card route and selection route in the opt-in walk (Later, row 38) | Nothing (internal). | The suite | FIXED (`abea88a87`): the test's stand-in, not the app. It handed the selection route the publisher's "Lord" where every pane hands over the drawn "Lᴏʀᴅ"; given the NKJV's shape, the WEB's Genesis, Exodus, Psalms and Isaiah differ in 11,884 of 29,441 passages with the old stand-in and in none with the drawn text. A fixture with the divine name in small capitals joins `TestPassageShareEqualsSelectionRoute`. That the NKJV's 46,837 come from the stand-in is shown on the WEB given the NKJV's marking, and on 15 October 2026 on the NKJV itself: the walk passes on an NKJV cache, and the old stand-in finds the same 46,837 there (Later minor releases, row 45). |
| 21 | Two comments describing the code before 1.2.20 (Later, row 43) | Nothing (internal). | The code | FIXED (`9a71f9c60`): `notes_sticker_native_on.go` and `bibleTextDismissMenu` (`reading_ios.go`) say what the code does. |
| 22 | A drag cut inside a word at a closed-up dash (the end repair of row 17) | A drag that stops inside the word after a closed-up dash keeps the word before it ("was morning—t", Genesis 1:5, stops being shared as "was"), and one begun inside the word before such a dash keeps the word after it ("g—the first" stops being shared as "first"). | All six (the BSB; the WEB and the WEB Catholic where they close a dash up) | FIXED (`ac2d445d1`). Older than row 17, and with the span and the lead alike: at every closed-up dash between words of two letters or more in the three local caches, each shape lost its word at the dash in all of the BSB's 2,052 drags, the WEB's 428 and the WEB Catholic's 476. The end repair cuts back to the later of the last space and the last such dash, the start repair forward to the earlier of the first space and the first, and the dash comes off as a clause mark does. `TestAPhraseEndingOnADashKeepsItsWords`; `TestADragCutInsideAWordAtADashKeepsTheWholeWordOverLocalCaches_LocalOnly` finds none wrong; every other local sweep passes as before. |
| 23 | A drag across a stack of headings (the heading repair of row 13) | A drag from the end of a verse across two headings over the next verse into its first words shares the words on either side, cited to both verses, instead of quoting both headings as Scripture (the BSB's Proverbs 22:16–17 across "Thirty Sayings of the Wise" and "Saying 1"). | All six, on the pages that send no lead: the Apple plain-text fallback and the legacy pane | FIXED (`76c8ee5ae`). Each heading was removed whole only where the number of the verse beneath stood straight after it, which the upper heading of a stack never has; the repair now takes the stack off whole. `TestADragAcrossAStackOfHeadingsSharesTheVersesOnEitherSide`; over the three caches, all 24 of the BSB's 22,728 drags across the headings between two verses that cross a stack (over Proverbs 22:17 and Hebrews 11:30), with the span and without, were wrong and none is now (`TestEveryDragAcrossTheHeadingsBetweenTwoVersesOverLocalCaches_LocalOnly`). |
| 24 | Two verse pictures saved in the same second overwrite each other (Later, row 3) | Two pictures shared within one second are both kept, the second named " (2)" as browsers name a second download. | Linux; the Windows fallback and the Windows Share sheet's copy | FIXED (`bb816a88e`, `freeShareImagePath`). `TestTwoPicturesSharedInOneSecondAreBothKept`. |
| 25 | The tab bar's dress is chosen when the window is built (Later, row 17) | An iPad or Android tablet window resized across 560 points while taller than wide rebuilds in the other dress at once. | iPad, Android tablets | FIXED (`f49b25091`). `renderedLayout` carries the dress where a bar is drawn, read from the canvas as the bar and the rail read it, so the watcher's two readings cannot disagree; the rail and the full-screen tree carry none. `TestATallWindowResizedAcrossTheBarsWidthRebuildsTheBar`. Not seen on a device: the iPad simulator in its windowing mode refuses an app's own request to change its window. A sheet open at the rebuild is taken and reopened, as for any layout change. |
| 26 | A reader who scrolled away from a link's arrival is taken back by the next width change (Later, row 18) | After scrolling on from a link's verse or note, a window resize or a rotation that rebuilds nothing leaves the reader where they are. | iPhone, iPad, Mac | FIXED (`291c5c550`). The reader's scroll ends the arrival as it ends the restore (`btIOSEndArrival`; `btMacUserScrolled`). `TestAReadersScrollEndsTheArrival`. Seen on 14 October 2026 with development builds before and after, each arriving at Psalm 119:100 (WEB) and then scrolled 1,500 points on as the reader, to verse 121: on the iPad Pro 13-inch (M5) simulator, in full-screen reading, a 250-point narrowing of the pane through its frame path put the build before back at verse 100 and left the fix at 121; on the Mac, a window narrowed from 1280 to 1000 points put the build before back at verse 100 and, with row 27, left the fix at 121. |
| 27 | A Mac window resize moves a reader who scrolled (hidden by row 26's arrival) | On the Mac, a window resized narrower or wider keeps the verse at the top of the view. | Mac | FIXED (`d07557382`). Setting the pane's frame wrapped the text at the old insets first, AppKit moved the clip with that passing wrap, and the insets put right after left the clip there: a reader at Psalm 119:73 was at verse 112 after a window narrowed from 1280 to 1000 points, and one at verse 121 at the end of the psalm. `btMacApplyFrame` reads the verse at the top and how far into it before a change of width and puts it back after; a restore or an arrival still places over it. `TestTheMacFrameChangeKeepsTheReadersPlace`. Seen with the development builds: verse 73 and verse 121 kept through narrowing and widening. The iPad's frame path kept the reader's place before the change and was not touched. |
| 28 | Mac narration that follows with the reading view hidden puts each verse at the very top (Later, row 20) | On the Mac, narration that follows the text while Books or Search is open, or under a sheet, keeps the verse in the comfortable band. | Mac | FIXED (`e3104ec92`). Follow runs while the pane is hidden, and the follow-scroll read where the view stands from `visibleRect`, which AppKit gives a hidden view as zero; it reads the clip view now, as the capture does. `TestTheMacFollowScrollReadsTheClipViewNotVisibleRect`. Seen with the development builds, narration following Psalm 119:60 with the reader on Books: the build before put the verse 0 points below the top of the view, the fix 191 points down a 636-point view, the band's 30%, where it still stood back on Read. |
| 29 | Android's night-mode patch reads its flag before it is written (Later, row 21) | Switching between light and dark while the app is in front always takes effect at once. | Android | FIXED (`aa2da8d5c`). Stock Fyne's activity sends the configuration to the driver before it writes the flag the driver reads; the patch writes the flag first, so the send orders the two. `TestAndroidNightModePatchWritesTheFlagBeforeTheConfigurationIsSent`. A first attempt read the night bit of the configuration the driver builds instead, which the asset manager's configuration does not carry, and left the app light through `scripts/test-android-dark-mode.sh`. With `aa2da8d5c` the app follows both ways in that check, and 20 switches 1.5 s apart were each followed, as by the build before; the race was not seen on the emulator in either. |

What the triage found and left, and why. The first pass:

- **Study verbs that lose their chapter** (Later, row 27, AI-4) and **the
  Apple plain-text fallback page** (Later, row 28, P3) are bugs that need no
  decision of their own and were left all the same, for their size and
  risk: each is about 2.5 days, AI-4 changes every pane's seam and P3 both
  Apple panes' Objective-C, which was more than the rest of the set
  together. Decision 2, taken that day, keeps both out of 1.2.22: they go
  to 1.2.23, each built on its own branch.
- **The marker strip with no lead** (Later, row 26, M) now happens only on
  the fallback page or where the lead fails its check, as it does for a
  drag begun inside a heading, and goes with row 28.
- **The red-letter generators** (Later, rows 31 to 33) can be fixed only
  with the regeneration that proves a byte-identical table, which needs the
  publishers' USFM downloaded and the NKJV opened once (decision 43). All
  three were fixed on 14 October 2026, not as rows of 1.2.22 (`78b9ce81b`,
  `3a8bbe3e3`, `d98440704`, and the lead-in check of all three,
  `c9149e93a`): the BSB's and the WEB's tables regenerate byte for byte
  from the publishers' USFM, downloaded that day. The NKJV's generator,
  run on 15 October 2026, refused 18 verses at words the source runs
  together; its check for an end inside a word was fixed that day, and it
  now writes the shipped table byte for byte (Later, row 31).
- **An emoji in Share with note on Android**, which `btaShareText` passes
  through `NewStringUTF`, where CheckJNI may refuse four-byte UTF-8, is not
  a bug where it can be checked: on the API 35 emulator with CheckJNI on, a
  message holding two emoji crossed it whole, with no JNI error. On 14
  October 2026 it was checked on an Android 11 (API 30) emulator image as
  well, with CheckJNI on and shown live first: a test app's `NewStringUTF`
  call aborted on malformed UTF-8 and took four-byte sequences. Notes
  holding U+1F64F, U+1F4D6, U+1D50A and U+10437 crossed `btaShareText` code
  point for code point, with no JNI error, abort or crash. A joined emoji
  lost its joiners before that call, which is Later, row 46. No API 24 to
  29 image is installed.
- The bugs that wait on a decision or on wording are Later rows 29, 2 (its
  line), 15, 22, 23, 11 and 9, Next major row 3 and Stays row 8, worst first
  row 29 (decision 23). Rows 39, 40, 42 and 44 are not bugs.

The second pass, Later rows 1 to 23 and Stays in the backlog. Rows 3, 17,
18, 20 and 21 of Later are rows 24 to 26, 28 and 29 above. The rest:

- **Later 4**, an iPhone opened sideways that draws the bottom bar for
  about 0.4 s: its decision is the technical route (answer the first layout
  from the orientation the scene reports, or hold it until the canvas has a
  size), and it changes the first frame at launch, which a simulator
  launched in landscape could show only with input on the host.
- **Later 8**, search results breaking a word where a highlight starts: large,
  and its recorded condition (ship only if the search-card count falls to
  zero, and weigh drawing results with row 7's word model) is still to be
  weighed.
- **Later 12**, taps that may land 8 points high: not established as a bug;
  it was seen once through a synthetic tap, and measuring it needs a real
  touch on a device this work does not use.
- **Later 16**, the Android tablet's column half a rail off centre: one
  condition, in rotation code that has gone wrong before; it needs emulator
  passes on a phone with a camera cut-out in both rotations and on the
  10-inch tablet, which this pass did not make.
- **Later 19**, the reading pane 4% off on iPhones whose screen scale is not
  the drawing scale: a toolkit patch that changes touch mapping on those
  devices, to be checked on one before it ships.
- **Stays 18**, Windows' first window not centred: needs the Windows virtual
  machine, which was not reachable for row 40 either.
- **Stays 21 and 22**, the iPadOS window buttons over the title and an
  external display's scale: toolkit patches that need a Stage Manager window
  and an external display to show them.
- **Stays 23**, Android never ending a cancelled touch: no reader effect is
  known and it cannot panic; a toolkit patch, with the tap measurement of
  Later 12 as its check.
- **Stays 24**, the plain-text fallback keeping the old palette's colour if
  all three imports of a rebuild fail: theoretical, and it needs the
  palette's ink pushed to both Apple panes; it goes with row 28, in 1.2.23
  (decision 2).
- **Stays 26**, iOS shipping `CFBundleExecutable: main`: renaming the
  executable changes the store package and `release-ios.sh`, which only a
  store build and upload can prove, and this work makes neither.
- Seen on the emulator with row 15, older, and not changed: Android parks a
  card whose verse the page lacks over the chapter's first line, where the
  iPhone and iPad reserve an inset for it (BACKLOG.md, AND-4).

### Added 15 October 2026: the ChatGPT faster-model fix and the notes guard

Two rows joined 1.2.22 on 15 October 2026: a defect found and fixed that day
for 1.2.23 (row 30; Later minor releases, row 49 until then), which joined
before the version bump, and the guard on the GitHub release notes (row 31;
Later, row 48 until then), which went in with the bump that rewrote them.
FIXED means, as above, that the change is in with a test that fails on the
tree before it (`149b85a91`). Neither changes the app's wording or stored
data.

| # | Item (where it was) | What a reader notices | Platforms | Outcome |
|---|---|---|---|---|
| 30 | The faster-model offer for ChatGPT switched to a model that then read as unavailable (Later, row 49) | Taking "Switch to a faster model" with ChatGPT gets an answer from the economy model, instead of being told on every request that the model isn't available with the reader's key until the model is changed in Settings. | All six | FIXED (`c3e4c1c50`). `gpt-4o-mini` caps completion tokens at 16384 and refused the shared 32768 budget with a 400 that read as model-not-found. The refusal is now its own error category and the one request is sent again at the cap the provider named, which also serves devices that stored the old id; the economy model is `gpt-5-mini`, of the capable default's own family. `TestAPinnedModelWithALowerCompletionCapAnswersAtItsCap` and `TestTheChatGPTFastModelIsInTheCapableDefaultsFamily` fail on the tree before. Live on 15 October 2026: both models answered the pinned-defaults guard, and a stored `gpt-4o-mini` override answered by way of one 400 and one 200. Still open beside it: a 429 that means an exhausted quota or spending cap reads as a busy service (the same BACKLOG.md entry). |
| 31 | The GitHub release notes are held to no release (Later, row 48) | Nothing in the app. The GitHub release page describes the release it carries, from 1.2.22's on, where v1.2.21's carried 1.2.20's notes word for word, from "New study tools and improvements since 1.2.19". | The GitHub release | FIXED (`85bd257d2`), with 1.2.22's version bump (`618653cc0`), which rewrote the notes. Stage 1 of RELEASING.md lists the `NOTES` printf in `.github/workflows/release.yml` as its tenth coupled surface. `TestTheGitHubReleaseNotesAreForThisRelease` holds the head, everything above the store and download footer: its lead line names the release before this one, the second `[[release]]` in `linux/releases.toml`, and no release but that one and this, and below that line the head is not the previous release's as its tag holds it, where the clone has the tag. `TestStage1ListsTheGitHubReleaseNotes` holds the stage to the list, and `TestGitHubReleaseNotesChecksFire` proves each check. On the tree before the bump (`149b85a91`) both checks fail: the notes for 1.2.21 open "since 1.2.19" and are v1.2.20's word for word. |

### Added 16 October 2026: the Android Activity compared across resumes

One row joined 1.2.22 on the day it went out, after the release builds
of 15 October 2026, which were made again from it before the tag. It
changes no wording and no stored data.

| # | Item (where it was) | What a reader notices | Platforms | Outcome |
|---|---|---|---|---|
| 32 | The Android bridge took a resume of the same Activity for a new one (*not in BACKLOG.md*) | On Android, coming back to the app no longer risks the reading pane being set up again and an incoming verse, such as a link's, replaced with the previous chapter's scroll position. | Android | FIXED (`ee9340f49`). The toolkit replaces and deletes its global reference to the Activity on every resume, even for the same Activity, so comparing that handle with the one the bridge kept could start the bridge's recovery for an Activity that had not changed, and the recovery put the previous chapter's scroll fraction back over an incoming verse. The bridge now keeps a global reference of its own and compares Java objects (`btaSameActivity`, `IsSameObject`, `reading_android.go`); recovery still runs for a different Activity, bridge setup and dispatch are serialized, and a JNI failure during setup is retried. The commit adds no test. |

### Beside 1.2.22: the account holder's console and policy work

Three corrections that need no build, each the account holder's to make; the
detail is in "Google Play's Data safety answers and the privacy policy: three
corrections" (BACKLOG.md).

| # | Item | What a reader notices | Where | Notes |
|---|---|---|---|---|
| 1 | Play's Data safety form gives Analytics as the purpose for Other user-generated content | The Play listing's data-safety section gives App functionality as the purpose, as the privacy policy, which says the developer runs no analytics, does. | The Play Console's Data safety form | Changed from Analytics to App functionality in the Play Console on 21 October 2026, with row 2, and sent for Google's review; while that review runs Play refuses an upload or a promotion, so the Publishing overview is read before 1.2.23's Play upload. Google requires the form and the policy to agree. One Play edit at a time. |
| 2 | Play's Data safety form shows that a reader can ask for the data to be deleted | The listing stops offering a deletion request: the policy sends a reader to the provider for that, the developer being unable to view or delete what a provider holds. | The same form | Set to No on 21 October 2026, with row 1. Both data types stay collected, optional, not processed ephemerally and encrypted in transit. |
| 3 | The privacy policy does not mention the AI panel's Report email | The policy says that Report opens the reader's email app with a message to the support address naming the passage and the study action, with the question and the answer, which reaches the developer only if the reader sends it, beside its sentence that the developer does not receive AI requests. | `docs/privacy.html` | The sentence is in `docs/privacy.html` (`d03c3f36a`, 21 October 2026); it names the passage, since the mail carries the reference and not the passage's text. The Report button (`reportBtn`, `ai_panel.go`) opens the pre-filled email. It reached readers with the site's publish of 21 October 2026 (`f86615091` on the site's branch), which carries the sentence. |

---

## 1.2.23, on `rel/1.2.23`

Eighteen rows, each built and reviewed on its own branch and merged into
`rel/1.2.23`; none is on `main`. Rows 1 to 6, from Later minor releases,
were built from `149b85a91` and merged on 15 October 2026, in this order:
rows 3 and 4 (`ec5439c80`), row 6 (`3baf40bc9`), row 5 (`e47b3d628`),
row 2 (`a9d64c6a6`) and row 1 (`837b07d4f`). The same day `rel/1.2.23`
took `main` at `9425d2b6d`, 1.2.22 as prepared for release (build 193).
On 21 October 2026 it took `main` at `d03c3f36a`: 1.2.22 as released on
16 October 2026 (`v1.2.22`, `ee9340f49`), its row 32 among them, and the
privacy policy's sentence on the AI panel's Report email (Beside 1.2.22,
row 3). So the branch holds every row of 1.2.22, and its version number
is 1.2.22's until 1.2.23's bump. Rows 7 to 15, each built from
`rel/1.2.23` at `be1440fe0`, were merged on 21 October 2026, in this
order: rows 8 and 9 (`ff87dcf96`); the record of the Windows checks of 15
October 2026, which stays in Later minor releases, row 40, for what is
left (`15918c2cb`); row 10 (`212490d05`), row 7 (`c31905e34`), row 11
(`3c3baaa74`), row 12 (`ee3cf9808`), row 13 (`8f9a3a2c4`), row 14
(`0bd04f11a`) and row 15 (`1d5acc555`); `b791120a4` holds rows 13 and
15 to row 14's rule. On 22 October 2026 four follow-ups, each built and
reviewed on its own branch, were merged, in this order: row 13's
permission that ends with its key (`fu/1223-consent-2`, `442a21938`);
rows 16 and 17, a key removed or not restored (`fu/1223-key-gone`,
`993b52639`); row 12's no study verb for a selection that holds no
Scripture (`fu/1223-no-scripture-study`, `a2cbc8c46`); and row 14's AI
answer waiting line (`fu/1223-ai-answer-short`, `f4460eb9b`). Six more
followed the same day, in this order: row 7's mark lit only where its
verse is named (`fu/1223-gap-marks-named`, `bf24fb6a3`); row 14's
covering sheets at their natural height and the Settings footer with no
rule (`fu/1223-short-windows-2`, `910d72f53`); row 18, Android's drag
and Translate (`fu/1223-android-drag-clean`, `22eb8eef4`); rows 16 and
17's removed key named and the line's ✕ (`fu/1223-key-gone-2`,
`dc312b8ee`); row 13's policy sentences placed and its date checked
(`fu/1223-privacy-2`, `556984026`); and row 12's study prompt that
quotes only Scripture (`fu/1223-ai-no-headings`, `a04829c5f`). Each row
keeps its earlier number in brackets: what this page and BACKLOG.md say
of Later minor releases, rows 2, 4, 10, 11, 15, 16, 23, 27, 28, 29, 31,
39, 45, 46 and 47, of Next major version, row 3, and of Stays in the
backlog, row 8, is said of these rows. None makes a reader download a
translation again or changes the form of data a shipped build wrote.
Rows 13 and 17 add records of their own to the settings, the
permission's key fingerprint and the saved-key mark, which hold no part
of a key, and under row 16 the next save names the WEB for a reader who
removed their API.Bible key. Row 16 writes "included" in the record a
removal has always written, beside the "1" every release writes, and
any value keeps the included key off as before; row 17 keeps a
device-scoped dismissal per key field. Android excludes that record
from backups; Apple's ThisDeviceOnly item can return to the same device
but does not migrate to another. Rows 10, 13, 15 and 16 add wording approved on 15
October 2026; row 10's line for a deuterocanonical book, row 13's
message after Not now and its question's body, row 16's line for the
included key and row 17's two lines were approved on 21 October 2026
(*Wording and decisions still open for 1.2.23*, below); row 17's ✕ has
no words; the rest add none. Rows 5 and 6 are toolkit patches, Patches 13 and 12 in
`patches/README.md`, applied by `scripts/setup-fyne-patch.sh` in that
order after Patch 11. Rows 7, 10 and 11 change the website too, which
reaches readers with the next `scripts/publish-site.sh`; its pinned
digests moved with them in both states of the switch. Row 13 changes the
privacy policy (`docs/privacy.html`), which the same publish carries, with
its effective date set to the day of that publish (docs/RELEASING.md,
stage 11).

The four round-four follow-ups are integrated locally in `rel/1.2.23`
on 23 October 2026, uncommitted: unfinished words across headings,
the small inline dismissal with confined touch-only geometry, the AI
waiting/error quote fit, and the neutral missing-key rule on Android and
signed Apple builds. Combined checks and runtime verification remain.
Earlier branch hashes and 22 October measurements below are historical
evidence for those earlier builds, not verification of these revisions.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | An iPhone launched sideways draws the bottom bar first (Later, row 4; in "One navigation rule on phones and tablets") | A phone or tablet is laid out for the way it is held from its first frame: an iPhone or an Android phone opened sideways no longer shows the bottom bar for a few frames (3 to 15 at 30 frames a second on the simulators) before the rail or the full-screen page, nor an iPad opened upright the rail before the bar. | S / medium | iPhone, iPad, Android phone and tablet (an Android phone must still never flash a rail); the desktop has a size from its first layout | The technical route only: answer the first layout from what the window reports at launch, or hold it until the canvas has a size. The fix built takes the first. Open: on Android, whether the tablet idiom and the bar's dress are judged in dp once the window has a size too, which would close the narrow band where they still change at the first paint (BACKLOG); and a check on hardware. | Minor, at launch only, and it touches the first-frame layout. Built for 1.2.23 on its own branch, `fu/1223-ios-sideways-launch`: before the canvas has a size the layout and the first chapter push decide by the size the window reports, on iOS its window scene's, read once on a goroutine of its own so that no launch waits for it, and on Android its activity's configured size. In the simulator and emulator pass of 15 October 2026 no launch drew the other navigation first (counted in recordings on iOS, in the layout log on Android), where main did in every iOS launch of those cases and in most Android launches whose first build came before the canvas had a size, and upright iPhone launches reached their first paint as soon as main's. It ran on the iOS 26.5 simulators, with the sideways iOS cases launched from a landscape-only copy of the bundle, so a launch of the shipping app held sideways on hardware is still to be seen. |
| 2 | Android centres the book page on the window, the iPad on its pane (Later, row 16; in "The reading page: what is left after 24 September 2026") | On an Android tablet with the side rail, the reading column sits centred beside the rail, as on the iPad, instead of about half the rail's width to the left. | S / medium | Android tablets (a phone held sideways must not change) | The entry's rule: centre on the window only when the pane spans it less the system insets. Open: the tablet held upright, where the rule moved the column 6.5 px to the pane's centre, as on the iPad; keep that, or count the reading body's right pad as part of the pane so the column stays where it was. | One condition, but this rotation code has gone wrong before: emulator passes on a phone with a camera cut-out in both rotations and on the 10-inch tablet, too much for 1.2.20's week. FIXED 15 October 2026 for 1.2.23 (`60f26f5d2`), on its own branch, `fu/1223-android-tablet-rail`. "Spans" is read as standing evenly, since a phone's pane held sideways keeps margins of its own, the same on both sides: the column takes the window's centre where the pane's two margins in the window less its side insets are equal to within 2 px, and the pane's otherwise (`BtReadingColumn.sides`). The emulator passes ran on 15 October. On the 10-inch tablet sideways both ways the column beside the rail stands on the pane's centre, where main's stood 54.5 px left of it, and phones held sideways on API 35, 33 and 30 take the padding main gives them. Held upright both ways the tablet's pane stands 14 px from the window's left and 28 px from its right, the right carrying the reading body's own theme pad, so the column takes the pane's centre, 7 px left of the screen's, where main's took the screen's: the iPad's reading body has the same pad and centres its column in its text view. `TestAndroidCentresTheBookPageOnItsPaneBesideARail` runs that Java with the JDK over the geometry and padding the bridge logged on the emulators, and over panes 1, 2 and 3 px uneven that hold the slack to 2 px. A phone's Read tab shows a rail only with the landscape presentation switched off, a development build's switch; its column follows the rail's rule. |
| 3 | The study verbs do not carry the chapter they were chosen in (Later, row 27; AI-4) | A study action, a share or the cross-references chosen after continuous narration has turned the chapter read the selection in the chapter it was made in, not the next (Psalm 135:3 selected, Explain clicked after the turn: "Psalms 135:3", where 1.2.22 names "Psalms 136:3"). | M, 2.5 days (about 1 of them Go) / low | All six | None for the fix. Its release was taken on 14 October 2026: 1.2.23, not 1.2.22 (Decisions still open, 2). | Fixed for 1.2.23 with row 28 (`b96c49c38`, `f407e5ded`, `5a9216df1`): built first on its own branch, `fu/ai4-chapter`, then taken with row 28 onto `fu/1223-ai4-p3`, where it sits on row 28's fallback page. On 1.2.22 the span carries verse numbers only and the dispatch reads them against the chapter then current (`ai.go:185`, `selection_span.go:16`); the desktop menu survives the refresh, and the Mac's menu tracking holds the queued turn until the menu closes. Now the span carries the book and chapter of the text the selection was made in (`selSpan.book`, `selectionChapter`), and every consumer of the span reads the selection there: the study verbs with their panel, header, cache key and Report mail, Share with citation, as image and as link, Share with note's composer and link, and the cross-references with the next release's coverage note and the edition's own block (`selectedChapter`); Share with note anchors its sheet to the selected words only while the page holds their passage (`notePassage.onScreen`). Each pane names the chapter: the two Fyne panes from the verses they laid out, with `selectionStudyMenu` stamping the chapter on screen on a span that has none; the iPhone, iPad and Mac panes key the page they install, an import or row 28's fallback page, with the chapter Go announces ahead of its HTML (`bibleTextTVSetChapter`, `bibleTextMacTVSetChapter`), taken once behind row 28's check that the push is still the latest, and hand it on through `bibleTextAIMenuTapped` and `bibleTextStudyMenuTapped`; Android likewise through `BtBridge.setChapter` and `btaSelectionAction`. One rule on all six, so no divergence. `TestTheDesktopMenuReadsTheSelectionInItsChapterAfterNarrationTurnsIt` turns the chapter with the narration controller under both desktop panes' menus and clicks each verb; `TestAVerbReadsTheSelectionInTheChapterItWasMadeIn` and `TestANativeSelectionDispatchedAfterTheTurnKeepsItsChapter` hold the consumers and the native route; `selection_chapter_contract_test.go` holds every pane's seam and every Go function that takes a span or the selected verses. On `main` at `149b85a91` the desktop test names Psalms 136:3 for every verb and the contract tests fail; the tests that need the span's chapter fail on this tree with it set aside (BACKLOG, AI-4). Opt-in over the local caches, every chapter pair: none of the WEB's 1,188, the BSB's 1,188 or the WEB Catholic's 1,327 is read differently after the turn, where 1.2.22 cited the next chapter's verse for 1,186 of 1,188 in the WEB and in the BSB and kept none right. Seen on 14 October 2026 with its first commit as made on its own branch from 1.2.22, before its two follow-ups and before row 28, on the Mac (WEB), the iPhone 16 Pro simulator (WEB and BSB) and the emulator (API 35, WEB, a real long-press and real taps): Explain, the three text shares, Share with note and Cross-references, chosen after the turn, each cite Psalms 135:3, where the same steps on 1.2.22 cite 136:3 for all six. Seen again on 15 October 2026 on the combined tree at `638bc88e8`, against `main` at `149b85a91`: every verb chosen after the turn, Share with note with a typed note and Explain's request to a stubbed provider among them, cited Psalms 135:3 on the Mac and the iPhone 16 Pro simulator (WEB and BSB), an iPad Air simulator (WEB) and the emulator (API 35, WEB), and on the Mac and the iPhone simulator on the fallback page a refused import leaves, where `main` cited 136:3 on an imported page and the chapter, Psalms 136, on the fallback page. Not run: the Windows and Linux virtual machines, the BSB on the emulator, and the iPad on the fallback page (BACKLOG, AI-4). |
| 4 | The Apple plain-text fallback page has no verse geometry (Later, row 28; C, IOS-PLAIN-FALLBACK-ZERO-SPAN, MAC-PLAIN-FALLBACK-ZERO-SPAN; P3) | On the page the iPhone, iPad and Mac show when a chapter fails to import three times, a selection cites the verse it was made in, the psalm title stays out of quotations, and highlights, notes and search arrivals land. Today every verb there searches the whole chapter (Psalm 136:10's refrain cited 136:3; "light" in Genesis 1:5 cited 1:3). | M, about 2.5 days / medium | iPhone, iPad, Mac | None for the fix. Its release was taken on 14 October 2026: 1.2.23, not 1.2.22 (Decisions still open, 2). | Fixed for 1.2.23 with row 27 (`12bd8ea67`, `51f7efe66`): built first on its own branch, `fu/p3-fallback-geometry`, then taken with row 27 onto `fu/1223-ai4-p3`, where row 27 keys the fallback page with its chapter as it keys an import. Built as the fix below says, with the walk in Go (`appleFallbackPageOf`) and one install for both pages (`btIOSInstallPage`, `btMacInstallPage`); the host and source tests are in the BACKLOG entry, and with this tree's tests over the panes of `main` at `149b85a91` seven of the eight source tests and the title test fail. The simulator and Mac checks passed on 14 October 2026 with its first commit as made on its own branch from 1.2.22 (`12bd8ea67` here), against 1.2.22 as the control, and again on 15 October 2026 on the combined tree at `638bc88e8`, which adds `51f7efe66` (the poem lines broken as the importer breaks them, the push's own generation, the face checked by name) and row 27's chapter key, against `main` at `149b85a91`, with an iPad Air simulator for the refrain and a note on a verse inside a psalm's stanza: on the fallback page the verse's paragraph is the imported page's stanza, and with the note open its band sits there on both pages, where on 1.2.22's fallback page it sits at the chapter's top. A note left and returned to shows no band on either build, since the reader's own note is drawn only while focused. On 1.2.22 the fallback empties the verse index (`reading_ios.go:3560-3564`; `reading_macos.go:1723-1725`, `gMacContentStart` 0; converters at :3478 and :1668). No reader has been seen on the page; on it 60.2% (WEB) and 57.7% (BSB) of double-clicked words cite an earlier verse, 0.88% and 0.99% of poem lines and 0.34% and 0.35% of whole verses. Fix: one walk over the cut HTML builds an attributed fallback (numerals at 0.66 em, title italic, headings bold) and runs the import's own derivations, so every walker answers and Go gets a real span; the three fallback contract tests change, and a development flag forces the import to fail for a simulator check. It retires the no-span title and heading rules, is tier 2 of 1.2.22 rows 6 and 7, answers 1.2.20 row 7's question whether such a selection should cite the chapter, and can close Stays in the backlog, row 24. Not in 1.2.22: Objective-C on both Apple panes, for a page no reader has reported. |
| 5 | Android drops the zero-width joiner and non-joiner typed into a text field (Later, row 46; in "Android drops the zero-width joiner and non-joiner typed into a text field") | On Android, a note typed with a joined emoji (a family, a profession, a flag) is stored, shared and linked as it was typed, instead of as separate emoji, and Persian and Indic text keep their zero-width non-joiner. The same holds in the fields the toolkit draws on the iPhone and iPad, such as search. On Android a backspace after an emoji no longer deletes the characters typed before it. | S / medium | Android (every version, every text field the toolkit draws; seen on API 35 and API 30 emulator images); the fields the toolkit draws on the iPhone and iPad (seen on the simulators) | None for the fix. Built 15 October 2026 on its own branch, `fu/1223-android-joiners` (`078182df2` and `849b3ab8f`, from `149b85a91`), and merged into `rel/1.2.23` the same day. | Fyne's mobile driver passes each typed character through `runeToPrintable` (`internal/driver/mobile/driver.go:594` in Fyne v2.7.4), which drops whatever `strconv.IsPrint` rejects, U+200C and U+200D among them, so neither reaches the field, while `share_note.go`'s note cleaning keeps both on purpose. Seen on 14 October 2026 on an Android 11 (API 30) emulator image: the input method sent a family emoji's two joiners, and the shared message and the link's payload held neither. Fix: a toolkit patch that lets them through, with a test that types a joined sequence into a field. Built as `patches/fyne-2.7.4-mobile-typed-characters.patch`, which types every character but a control, the desktop's rule, so the spaces, private-use characters and characters newer than Go's Unicode tables that the filter also dropped now arrive too; `normalizeNote` still decides what a note keeps. The same patch sends Android's deletions one character at a time rather than one UTF-16 unit, which deleted the characters typed before an emoji: one backspace after "abcdef" and a family emoji left "abc". Checked on 15 October 2026, each against a control build: on the API 35 and API 30 emulator images a 29-character note arrived whole in the message and the link (control 24), and one backspace after "abcdef" and a family emoji left "abcdef" (control "abc"); in the search field on the iPhone 17 and iPad Air simulators, 29 (control 24); the native composer and the Mac kept 29 on both builds. A backspace still removes a whole joined emoji on Android and one character elsewhere: divergence 35 in PLATFORM_MATRIX.md. The tests and the details are in the BACKLOG entry. A bug that needs no decision, for 1.2.23. |
| 6 | A space after an emoji, or after a character from a fallback font, draws wrongly in text the toolkit draws (Later, row 47; in "A space after an emoji or a fallback-font character draws wrongly in text the toolkit draws") | In the Share with note field and the notes browser, "a 🙏 b" keeps its space instead of drawing as "a 🙏b", and a space after a character such as 𝔊 is a space, not a missing-glyph box. | M / medium | All six, wherever the toolkit draws note text: the notes browser everywhere, Share with note everywhere but iOS; seen on Android | None | FIXED 15 October 2026 in a toolkit patch (`3b4041c9d` and `5df78e565`, `patches/fyne-2.7.4-fallback-space.patch`, Patch 12); the device checks of that day ran at `01e83122b`, before the corrections in `5df78e565` (BACKLOG). The stored and shared text was right. Cause: go-text's face split (`splitByFace`, `ignoreFaceChange`, v0.3.4) keeps a space in the run of the face before it, and the painter measures and draws it there. In the bundled emoji face the space glyph has no bitmap, and go-text's shaper gives a glyph without extents no advance (stock Fyne's emoji face gave it one, so a build without the patches does not show this); a fallback face with no space shapes it as a missing glyph, which the painter draws as a U+FFFD box but advanced by the face's glyph 0, counted twice; macOS's LastResort face, which the toolkit takes 𝔊 from, draws the space as a placeholder; any other fallback face gives it its own width. A space alone in its cluster, shaped in a face other than its own, now takes its advance from its own face and draws nothing (a space carrying a combining mark is left as shaped, so the mark is drawn), and a missing glyph advances once, by the U+FFFD drawn for it. The patch's tests that fail on the tree without it measure "a 🙏 b" one space wider than "a 🙏b", give a space after a symbol-face, monospace-face or LastResort character the text's width, with no ink where the space is after the symbol-face and LastResort ones, and keep a missing glyph's box clear of the next glyph; others guard the marks, the ellipsis cut and the cost of measuring. The app's suite passes against the patched toolkit. A bug that needs no decision, for 1.2.23. |
| 7 | An omitted verse's mark lights, and a note on the hole hangs after it (decision 18; AND-4) | With the footnotes on, an `/nkjv/` link to Matthew 17:21 opened in the BSB without the NKJV lights the "[21]" mark beside verse 22, where it lit nothing, so the reader sees where the link pointed; the website, which always draws the mark, too. A link to 17:20–21 lights verse 20 and the mark as one band. A note lights the marks of the verses it names in the same way, one from another edition too: an NKJV note on 17:21–22, which the chapter's plan files on verse 22, lights the mark and verse 22. A link to the verse after the hole alone leaves the mark bare: 17:22 lights verse 22, and in the WEB Catholic Edition a link to Sirach 26:28 lights verse 28 with the nine marks "[19]" to "[27]" before it bare, as the iPhone, iPad and Mac left them in 1.2.22, where the other surfaces lit them with it. A note shown from its link alone on 17:21, or on a range from it, hangs above verse 22's paragraph, where the mark stands, on every surface, where Android and the Windows and Linux pane drew it at the chapter top, and every surface did for the verse alone. | S, 1–1.5 days + site / medium (Objective-C on both Apple panes) | All six and the website | Decision 18, taken 15 October 2026 as recommended: light the mark; draw such a card above the verse after the hole on every surface. Of the three rules the build added beyond it (BACKLOG, AND-4), two were confirmed on 21 October 2026, AND-4's first and third: the card of a note on the omitted verse alone hangs above the verse after the hole too, and no verse's narration lights a mark. The other, AND-4's second, the mark lit with the verse after the hole, was rejected; the mark now lights only when a link or a note names the verse it stands for, so a link to Sirach 26:28 in the WEB Catholic Edition lights verse 28 alone. | Built on its own branch, `fu/1223-omitted-verse-mark` (`a86fcd9c2` and `67c199afc`, from `be1440fe0`). The mark takes its own tint (`gapOf`): lit only when a span names the omitted verse, the same in both HTML dialects, the Windows and Linux pane, the Apple panes' runs and the website (`lightMarksIn`). The build lit it with the verse after the hole as well; that rule was taken out on `fu/1223-gap-marks-named` (`2aa12b083`, from `3261a1e13`), merged into `rel/1.2.23` on 22 October 2026 (`bf24fb6a3`), where the website's band stopped lighting the marks before a lit verse and lights a space only between two lit verses or marks. A note from another edition, filed by the chapter's plan on the verses this edition has, lights the marks of the holes its own span names (`noteMarkSpan`, `4ce5575e4`, on the same branch): an NKJV note on 17:21–22 read in the BSB lights the mark and verse 22, where it lit verse 22 alone once the rejected rule was out; one on 17:20–21 lights verse 20 and the mark, and one on 17:20–22 verse 20, the mark and verse 22, where each lit verse 20 alone, with the rejected rule and without it. A link to the same span lights the same, in the app and on the website. The Apple panes paint from runs of verse numbers, where a verse ran on to the next number, so the mark was the verse before the hole's: a mark on verse 20 alone washed it, and one on 17:22 left bare what the markup lit. The runs now carry the mark in its place, and both panes index the marks apart from the verses, end a verse at the mark after it and resolve an omitted number to its mark. The chrome hands every pane the verse the page draws for a note (`drawnNoteAnchor`), so such a card hangs above the verse after the hole and the arrival lands on its band; this closes divergence 34 in PLATFORM_MATRIX.md. No verse's narration lights a mark: Android's verse index ends a verse at the mark after it, as the Apple panes' does, and the Windows and Linux pane's narration skips the mark, where in 1.2.22 the narration of the verse before a hole lit it on the iPhone, iPad, Mac and Android, and that of the verse after it on the Windows and Linux pane. The host tests, each failing on the code before its change, are in the BACKLOG entry. Not yet published to the site. Seen on 21 October 2026 with development builds of `19b1e1094` and of `3261a1e13`, on the Mac, the iPhone 16 Pro simulator and the emulator (API 35): links to Sirach 26:28 in the WEB Catholic Edition and to Acts 8:38 in the WEB leave the marks bare, where `3261a1e13` lit them, and links to 17:21, 17:20–21 and Acts 8:37 light the mark on both. A note's marks (`4ce5575e4`) not yet seen on a device. |
| 8 | NKJV red letters: the generator places a run at the first copy after its cursor, and retries from the verse's start (Later, row 31; RL-NKJV-1, RL-NKJV-2) | Nothing: the regenerated table is the shipped one byte for byte. Before the fix, a regenerated table could redden another copy of a run's words, inside a longer word or in narration, or leave a marked poem line black, and generation would report success. | S, 0.5 day / low | None (a build tool) | Taken 15 October 2026 (Decisions still open, 43): the NKJV was opened once for the first run and once more for the run after the fix. | DONE, closed 15 October 2026. FIXED in the generator on 14 October 2026 (`d98440704`, `c9149e93a`), and its check for an end inside a word on 15 October 2026 (`b7f5cb9a7`). The parser keeps each verse's non-space characters, leaving out the verse number and headings as the app's decoder does, and each run is placed at their count, only where the app's verse has the same characters before it; the retry from the verse's start is gone. Where the app's text before a run differs from the markup's, or the run is not at its count, generation fails, naming the verse and the run's number; with `--allow-relocated` such a run is placed instead by its one copy in the verse, after the run before it, and listed. No copy or several, an end inside a word, or a chapter missing from the cache fails generation either way, where a missing chapter's verses were left out of the table, which reads as black letter. `--no-fetch` derives from the cached chapters with no key read and no call. On invented markup: `TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace` fails on the derive logic before the fix in three verses ("Go" in "Gospel", the narration's copy, with and without a heading in the verse), and each refusal in `TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace` (a second run folded into the first, a count that falls on a later copy, an end inside a word, a missing chapter) wrote a table and reported it made. Run on 15 October 2026, with `--no-fetch` and without `--allow-relocated`, on the NKJV fetched that day through the app's own path (197 requests) and the generator's 174 chapters, both kept outside the repository and deleted after, it wrote no table. Every run of the 2,054 marked verses is at its count, with the same characters before it, so none needs `--allow-relocated`; the check for an end inside a word refused 18 verses: Matthew 5:3, 9:15, 10:17, 10:19, 10:22, 15:8 and 22:44, Mark 7:9, Luke 4:18 and 6:24, Acts 1:8, 9:11, 9:15, 9:16, 11:16, 23:11 and 26:18, and Revelation 2:27. Each refused edge, 26 places in all, is where the app's verse anchors a note and the publisher's text has no space on either side of it, so two words run together in the markup and in the app's text alike (row 9). In 17 of the verses the place lies between two runs that touch and merge into one span; in Matthew 15:8 a run ends there and black text follows. The fix (`b7f5cb9a7`) counts a place where the app's verse anchors a note as the edge of a word, and still refuses an edge between two letters or digits with no note there; checking only the merged spans' ends would still have refused Matthew 15:8. `TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace` places three invented verses, where two runs touch, where a run ends before black text and where a run begins after black text, each with a note anchored at the joined place, with and without `--allow-relocated`, and the generator before the fix refuses all three in both modes; `TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace` refuses each of the three shapes with no note there, and each with a note elsewhere in the verse (`7d1950543`). Run again the same day after the fix, on the NKJV fetched once more the same way (371 requests, no 429), with `--no-fetch` and without `--allow-relocated`, the generator writes the shipped `red_letter_nkjv_data.go` byte for byte, all 2,054 verses, none placed by its only copy: 51 run edges fall between two letters or digits, all of them at the 26 places where a note is anchored. The control, the generator before the fix on that text, refuses the same 18 verses and no others, and `TestNKJVRedLetterTableIsAcceptedByItsOwnGuard` accepts all 2,054 entries against it. A build tool, and no table readers have changes. |
| 9 | The NKJV's card route and selection route: the opt-in walk on an NKJV cache (Later, row 45; in "The NKJV's card route and selection route disagree in the opt-in walk") | Nothing (internal). | S / low | The suite | Taken 15 October 2026 (decision 43): the NKJV was opened once, and the walk compared without printing the text. | PASSED 15 October 2026; the entry is closed. 1.2.22's row 20 fixed the test's stand-in and showed, on the WEB given the NKJV's marking, that the stand-in accounts for differences of the NKJV's shape. On the NKJV fetched that day through the app's own path, `TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly` compares 143,627 passages and none differs. The control, the same walk with the stand-in as it was before `abea88a87`, finds 46,837 of the 143,627 differing, the figure recorded before that fix, so those differences were the stand-in's. |
| 10 | The Greek Esther mapped verse for verse, like Daniel 3 (Next major version, row 3; NEXT.md, piece 2, *Work graduated for 1.2.23*) | Reading Esther in the WEB Catholic, notes, highlights, shared links and cross-references cross from the other editions for 164 verses, where today they stop at the book, and the app and website stop saying the two texts' verses do not correspond. The cross-references panel says why it lists nothing for a verse a translation adds (the Greek Esther's additions, the Song of the Three, Susanna, Bel and the NKJV's four) or for a deuterocanonical book, before loading anything. | M / medium | All six and the website | Taken 15 October 2026 (decision 9): it leaves the switch in a minor release; E-A, keep the gap marks; E-B, the one line for every uncovered addition; E-C, the two sentences approved as drafted. The line for a deuterocanonical book, "The cross-references don't cover the deuterocanonical books.", part of the piece and shown by a shipping build for the first time, was approved on 21 October 2026. | Built 15 October 2026 on its own branch, `fu/1223-greek-esther`, from `rel/1.2.23`, and merged into it on 21 October 2026, out of the switch. The tables, the panel's line and its answer before loading, and the two sentences no longer read the switch; both generators lost `--next` and reproduce the shipped tables from the caches. E-B is one rule in `crossRefCoverage`, with an edition whose own apparatus the panel shows (`nkjvxrefs`) left out (`panelCoverage`). The deepest Treasury row a panel reads without the tag is now the eighteenth, first at the WEB's Matthew 10:1 (was the twentieth, WEB Catholic's Genesis 41:42); twenty-one with it, unchanged. On the site the forty Esther chapter pages change and no other file, so the site needs a publish with 1.2.23. Changes no stored data. Seen on 15 and 16 October 2026 at `839dbacc9`, against `rel/1.2.23` at `be1440fe0`, on the Mac, the iPhone 16 Pro and iPad Air 11 simulators, the emulator (API 35), the Linux and Windows virtual machines and the site built offline: the WEB Catholic's Esther 4:1 maps to the WEB's, a WEB note on 4:1 is placed there and one on 4:6 says the verse is not in the translation being read, a highlight on 4:17 crosses to the WEB, the panel answers Esther 4:20, Daniel 3:24, 13:1 and 14:1 and Tobit offline without asking to load, the gap marks show in Esther 4 and 9, and the site's switcher keeps the verse, where `rel/1.2.23` does none of these. Not run: the emulator at API 30, and the NKJV on a device. On the real NKJV, fetched through the app's own path on 16 October 2026, the rule names Acts 8:37, 15:34 and 24:7 and Luke 17:36 and no other of its 31,102 verses, the Treasury lists nothing for those four and something for the verse before each, and the panel answers each offline without loading; walked with the other three, it reads no deeper than the eighteenth row without the tag and the twenty-first with it. |
| 11 | WEB Catholic Daniel 3: a translators' note fused into the heading (Later, row 15; SCRIPTURE_WORKLIST.md, S15) | In the World English Bible (Catholic) at Daniel 3, in the app and on the website, the heading above verse 24 reads "THE SONG OF THE THREE HOLY CHILDREN", as the publisher sets it, and the translators' note about the addition is a footnote filed under 24, among the chapter's footnotes when they are shown (always, on the website), instead of running into the heading as one 510-character bold heading. | S / medium | All six and the website (WEB Catholic only) | Settled 15 October 2026 by the source (decision 8). The publisher's own USFM sets the line as a section heading carrying a footnote whose reference is 3:24, so the 510-character heading was a decoding defect, not the publisher's text, and it is fixed to match the source: the title is the heading and the note is that heading's footnote, shown as every other footnote is, with nothing dropped, set apart when the page is drawn so nobody downloads again. Recorded in ADDITIONS_AND_DROPS.md. | Built 15 and 16 October 2026 for 1.2.23 on its own branch, `fu/1223-heading-footnote` (`5ff2cbd77`, `f8d65a99b`, `e1c91e199`), from `rel/1.2.23` at `be1440fe0`, and merged into it on 21 October 2026. In eBible's WEB Catholic USFM (`66-DAGeng-web-c.usfm`, downloaded 14 October 2026) the line is an `\s1` heading with `\f + \fr 3:24 \ft` and the note; the helloao feed sends heading, reference and note as one string, the reference straight after "CHILDREN", and every cache of the edition holds that string. `headingsFor` sets it apart (`settleHeadingNote`): a reference to the chapter and the verse the heading stands above, straight after a letter or punctuation, then the note's words, in a helloao heading only. The cache keeps the feed's string, so there is no epoch and no download. `chapterFootnoteEntries` files a drawn heading's notes under the verse it stands above, ahead of that verse's own, on the Apple pages (iPhone, iPad, Mac), the Android page, the Windows and Linux pane and the website, and the share code reads the headings through `headingsFor`, so a drag across the heading is matched against the words the page draws. Only a helloao heading's notes are listed (`feedHeadingStyle`): the notes the API.Bible decoder keeps on an NKJV heading stay unlisted, as before, since how many of them are not cross-references has not been measured. Every read of a chapter's headings outside the tests goes through `headingsFor`. One rule everywhere, so no divergence; the website changes at its next `scripts/publish-site.sh`. `TestTheSongOfTheThreeIsDrawnAsItsPublisherSetsIt` decodes the feed's own nodes, `TestACacheWrittenBeforeTheHeadingNoteWasSetApartDrawsIt` loads a cache in the on-disk form, and `TestTheSongOfTheThreeIsSetAsItsPublisherSetsItOnThePage` builds the website's page; all three fail on `be1440fe0`. `TestEverySurfaceSetsTheHeadingNoteInTheFootnoteSection`, `TestTheSongOfTheThreeIsNeverQuotedAsScripture`, `TestOnlyTheFeedsRunOnIsReadAsAHeadingNote` and `TestAHeadingsNoteIsFiledUnderTheVerseBelowAheadOfItsOwn` each fail with their part of the change taken out. `TestAnNKJVHeadingsKeptNotesAreNotListed` fails with every heading's notes listed, the whole heading shared with no span in `TestTheSongOfTheThreeIsNeverQuotedAsScripture` fails with that one read (`wholeHeadingVerse`) put back to the stored headings, and `TestEveryReadOfAChaptersHeadingsGoesThroughHeadingsFor` fails with any one of the share code's six reads put back. The opt-in sweeps over the local caches that drag across, from and into every heading read the headings as the page draws them and pass with `BIBLETEXT_EXHAUSTIVE=1`. In the WEB, WEB Catholic and BSB USFM this is the only section heading carrying a footnote. Psalm titles carry them in all three (the WEB 4, one of them on Psalm 151, which the app's WEB does not carry; the WEB Catholic 3; the BSB 31, with 36 notes), and each of those notes is in its chapter's section under "Title", decoded from the helloao feeds and loaded from the caches, before the change and after it. Over the three editions the Daniel 3 heading is the only one whose drawn text changed, and its chapter the only one whose footnote section did. Single heading words in the WEB Catholic that a selection inside the heading cites from the verse above (Later, row 29): 1 of 34, where it was 27 of 113. Device checks, on scratch builds against `be1440fe0`, each with a WEB Catholic cache written before the change: on 15 and 16 October 2026, at `cc3324182`, on the Mac, the iPhone 16 Pro and iPad Air simulators, Android API 35 and 30 and the Linux virtual machine, the heading is the 35-character title and the footnote section lists 1, 24, 77 and 91, where the control draws the 510-character string and lists 1, 77 and 91; BSB Psalm 6's title note is the same on both builds, and the cache file is unchanged after the run. On the iPad the pane's heading paragraph reads 35 characters, all bold, but its footnote rows were not read back on either build; its text holds the note's words with footnotes on. The website's page has the 35-character heading between verses 23 and 24 and lists 24, where the control's heading is the 510-character string and its list has no 24. The Windows build of that run was made without the patched toolkit and opened no window. On 21 October 2026, at `e1c91e199`, built as the control is (windows/arm64, `gles`, the patched toolkit, ANGLE beside it), the Windows 11 arm64 virtual machine draws the 35-character heading above verse 24 and, with footnotes on, the note as row 24 of the section between 1 and 77, and with them off no section and no note; the control draws the 510-character heading over eleven lines and lists 1, 77 and 91. `f8d65a99b` and `e1c91e199` leave every footnote section of the WEB, BSB and WEB Catholic as it was (3,706 chapters compared), so the other surfaces' checks hold for them. |
| 12 | A selection inside a publisher's heading resolves to the verse above (Later, row 29; APPLE-HEADING-MATTER-TO-VERSE-ABOVE; P4) | On the iPhone, iPad, Mac and Android, a word selected inside a publisher's heading is cited by the verse below the heading and quotes nothing, as the heading selected whole is, where it was cited by the verse above and quoted as Scripture ("Cain" in "The Descendants of Cain" was shared as Genesis 4:16; the "The" of "The Fourth Day" as 1:13). No study verb is offered for it, nor for a psalm's title or a verse number selected on its own, and a text share of it is the citation line alone. On Windows and Linux, where a heading's words cannot be selected, a double-click on one no longer selects the last word of the verse above. | S–M, 2 days / medium | iPhone, iPad, Mac, Android; the double-click on Windows and Linux; the study verbs' rule on all six | Taken 15 October 2026 (Decisions still open, 23): A, a heading's words mean the section the heading introduces. Built the same day on its own branch, `fu/1223-heading-words` (`b89c475c9`, from `be1440fe0`), merged into `rel/1.2.23` on 21 October 2026; the text share on 21 October (`99dbcd728`), and Share with note's anchor (`ca1f500a6`). The follow-up, taken 21 October 2026: no study verb for a selection that holds no Scripture, in place of the study prompt `99dbcd728` built; on its own branch, `fu/1223-no-scripture-study`, from `rel/1.2.23` at `3261a1e13`, merged into it on 22 October 2026 (`a2cbc8c46`). The prompt's passage, taken 22 October 2026: a study prompt's passage holds only Scripture, so a selection run across a heading or a psalm's title sends the verses' words alone, the heading's left out as the share leaves them out; built on its own branch, `fu/1223-ai-no-headings`, from `rel/1.2.23` at `da90dfb55` (`e86fc00be`), merged into it on 22 October 2026. Settled on 22 October 2026: what else the follow-up built is confirmed as built, and a selection whose only Scripture is punctuation, a verse's closing full stop or any other mark of its words selected alone, or a drag from a verse's closing punctuation into the heading below it that ends in the heading, is offered no study verb on any pane, the rule for a selection that holds no Scripture, and the dispatch refuses one; built on its own branch, `fu/1223-no-study-punctuation`, from `rel/1.2.23` at `91e1ec683` (`ca86cca13`) (*Wording and decisions still open for 1.2.23*, below). | FIXED for 1.2.23. Each of the four panes tests the selection against the heading range it keeps beside its verse numbers (`btIOSHeadingSelectionVerse` over the verse index's end and tail, `btMacHeadingSelectionVerse` over `btMacIsHeadingParagraph` and `btMacHeadingBlockEndAt`, `BtBridge.headingSelectionVerse` over `verseEnds` and `isHeadingParagraph`) and hands a selection wholly inside a heading over as the verse below it, with a heading flag and no lead (`selSpan.heading`). Go answers such a span before any search (`normalizeShareSelectionProse`), so the text and image shares, the link and the note kept against it, the note composer, the cross-references and the study panel's citation all name the verse below and quote nothing. The study prompt put the selected words in quotation marks as that verse's passage, so a heading's words went to the provider as Scripture the verse does not hold (the BSB's "Fourth" as Genesis 1:14). No study verb is offered for a selection that holds no Scripture, a heading's words, a psalm's title or a verse number alone, the selections the share answers with nothing quoted: the desktop menu leaves Study with AI out (`aiStudyOffered`, `selectionStudyMenu`), each native pane keeps the rule over the page it draws (`btIOSSelectionHoldsScripture`, `btMacSelectionHoldsScripture`, `BtBridge.selectionHoldsScripture`, which Android asks again each time its toolbar is prepared again), and `dispatchAIAction` drops one that a menu delivers anyway; Share and Cross-references keep their places. Beside a heading the panes' rule answers as Go does (`0f9ca1d57`): a word a drag from the heading ends inside reaches back to the white space or closed-up dash before it, so "Solomon’" of the BSB's 1 Chronicles 3:10 gets no study verb, where the rule offered one the dispatch dropped, and the closing punctuation of the verse above counted, so a drag from it into the heading was studied, as Go studied it; since `ca86cca13` neither studies a drag that holds that punctuation alone (below). This replaces `99dbcd728`'s reading of the cited verses as the passage. A selection run across a heading, from the end of one verse through the heading or a psalm's title into the next, is studied, and its prompt put the heading's words inside its quotation marks as the cited verses' words: in the BSB, "waters." through "The First Day" into "And God said," went to the provider with the heading and verse 3's number inside it, as Genesis 1:2–3. Since `e86fc00be` the prompt quotes what the share quotes for the same selection, before the share frames it (`aiPassageFor`, `selectionQuotationIn`, which `shareQuoteIn` frames), in the form a machine reads: the heading's and the title's words out, and with them the verse numbers, as the citation names the verses, a word cut short trimmed back, and the line breaks the chapter's own. Quoted without the share's framing, a selection of words that ends on a comma, a semicolon, a colon or a dash is sent without it and with no ellipsis in its place, and in the WEB and the WEB Catholic a drag into Isaiah 62:12 that ends after "Redeemed" is sent with the sentence's full stop and without the closing quotation mark before it (BACKLOG). The dispatch keeps the divine name's small capitals as drawn until the passage has been read, as the share reads it: written as capitals first, a drag across a heading on a chapter that sets the divine name in small capitals, as the NKJV does, was not found in the chapter and kept the heading's words in the prompt and the study panel, and one begun inside the heading was cited by the verse above as well. A drag from the closing punctuation of the verse above into a heading, ending there, sent that punctuation alone, as a mark of a verse's words selected alone sent that mark; since `ca86cca13` neither is offered a study verb (below). One rule on all six, built in Go; the web reader has no selection verbs. Over the local caches, drags across the headings at every place between two verses that holds them (WEB 21, BSB 1,894, WEB Catholic 23; the BSB's 1,894 hold 1,896 headings, a stack of two at two of them) and from every psalm title into its psalm with no span (116 in each), 284, 15,259 and 300 in all, each offered the study verbs, send the share's quotation, Scripture alone (`TestAStudyPromptQuotesNoHeadingOverLocalCaches_LocalOnly`); at `da90dfb55` every one quoted otherwise, and 263, 13,374 and 277 held the headings' or the title's words. Seen on 22 October 2026 at `a244cb2ed`, against `rel/1.2.23` at `da90dfb55`, with the request logged and not sent: on the Android emulators (API 35 and 30), the iPhone 16 Pro and iPad Pro 13-inch simulators and the Mac app, in the BSB, drags across a heading, across two, begun inside a heading's last word, from Psalm 3's title into the psalm and from John 3:16 into 3:17 sent the verses' words alone, and Ask and Compare quoted the first as Explain did, where the control sent the headings' words or the verse numbers; the drag from Genesis 1:2's full stop into "The First Day" sent "." alone, where the control sent the heading with it. On the NKJV, on the Mac and the iPhone simulator, a drag across Genesis 2's heading into the divine name sent none of the heading's words and the name in capitals, where the control sent the heading's words. On Windows and Linux it was seen only in the Mac's Linux and Windows mimic, for the drags from John 3:16 into 3:17 and from Genesis 4:16 into 4:17; not on the virtual machines, and not on the WEB (BACKLOG). A verse number selected on its own where the verse's words hold the same figure is read as the number (`placeByLead`), where the BSB's Genesis 5:3 quoted the "3" of "130" and the menu offered the verbs; so is a piece of one, its first digits or a number begun after the verse above (`0f9ca1d57`), which quoted the figure, and a drag from a heading into the first digits of the number below is answered as the heading is (`e5389ed3d`), where "The Fourth Day 1" was quoted as Genesis 1:13–14. A text share that quotes nothing is the citation line alone, where it was that line under two empty lines (`composeShareText`). Share with note sent with a note on a heading's word is anchored to the number of the verse below (`wordsAnchor`); with no anchor, the iPad's popover hung from the top of the reading view once the page had moved to the note's card. A heading above verse 1 stays outside the app's verbs on those panes, as the psalm title does. The Windows and Linux pane draws a heading outside its text model, and its double-click there now selects nothing (PLATFORM_MATRIX.md, divergence 36). The web reader has no selection verbs. With the Daniel 3 heading set apart (row 11), the sweep reads each chapter's headings as the page draws them (`headingsFor`). Over the local caches every word of every heading between two verses (WEB 23 words under 21 headings, BSB 7,373 under 1,896, WEB Catholic 113 under 23) and every such heading whole is answered so; with the span the panes sent before, 2,023 BSB words under 1,301 headings and 27 WEB Catholic words were cited by the verse above, and 3,941 words were quoted. A host probe of the Mac pane's Objective-C, and of the iOS verse index read with AppKit's fonts, over those 1,940 headings imported from their chapters' HTML, named the verse below for all 13,329 selections tried and took none of 3,880 verse words for a heading's. The sweep holds that none of its selections is offered a study verb, by the desktop menu or by the panes' rule modelled on the page: 9,370 with the Daniel 3 heading set apart (WEB 44, BSB 9,269, WEB Catholic 57), all of which `rel/1.2.23` at `3261a1e13` offers the verbs. Over the same caches `TestNoStudyVerbForANumberOrATitleOverLocalCaches_LocalOnly` offers none for 97,563 verse numbers selected alone, 159,878 pieces of numbers and 696 psalm-title selections, and answers 36,489 drags beside a heading as the rule says, the menu and the panes' rule alike but for two drags at the BSB's Ephesians 3:14 held apart; at `6ef8ae9e3`, 5,155 of them are answered otherwise, and the panes' rule as it was there answered 7,693 of the drags otherwise than the menu. The device checks on `005c19143` passed on the Mac app, the iPhone and iPad simulators, the Android emulators (API 35 and 30) and the Windows and Linux virtual machines, each against `rel/1.2.23`; they read the study panel, not the prompt sent, and the text share before `99dbcd728`. On 21 October the Mac app and the iPhone and iPad simulators, against `005c19143`, sent the citation line alone and the verse below as the prompt's passage, and the iPad presented Share with note on "Cain" beside the number of Genesis 4:17. The follow-up was seen on 21 October 2026 at `6ef8ae9e3` against `3261a1e13`, and on 22 October at `e5389ed3d` against `6ef8ae9e3`, on the Mac app, the iPhone 16 Pro and iPad Pro 13-inch simulators, the Mac's Linux mimic and the Linux and Windows virtual machines; and on 22 October 2026 at `da90dfb55`, against `9a308352d`, on the Android emulators, API 35 and API 30, with real long presses on the BSB: no Study with AI for "Cain" in a heading, for verse 17's number alone, for "Psalm" in Psalm 3's title or for Genesis 5:3's number alone, whose Share with citation sent the citation line alone, where the control offered Study with AI for each and quoted "3", while "Cain" in verse 16 kept it; and on API 30, with a selection moved in code under a toolbar that was up, not by its handles, Study with AI hid on the heading's words, showed once the selection reached verse 17 and hid again, where the control kept it throughout (BACKLOG). No study verb for a selection whose only Scripture is punctuation (`ca86cca13`, taken on 22 October 2026): Go studies a selection only when the share's quotation of it, the passage the prompt sends, holds a letter or figure (`aiStudyOffered`), and each native pane's rule counts only a letter or figure of a verse's words; a drag begun on the closing punctuation above a heading keeps the piece of the first word below that it ends inside, as Go quotes it (". S"), and with no verse index a pane asks for a letter or figure anywhere in the selection. Share and Cross-references keep their places, and the share of such a selection is unchanged. One rule on all six. Over the local caches `TestNoStudyVerbForPunctuationAloneOverLocalCaches_LocalOnly` offers no study verb, by the desktop menu or by the panes' rule modelled on the page, for 96,992 verses' closing marks (WEB 31,000, BSB 30,727, WEB Catholic 35,265) and 326,721 other marks of verses' words (112,846, 89,088 and 124,787), each selected alone, and for the 1,938 drags from a verse's closing mark into the headings below it (21, 1,894 and 23), and the same drags run on through the first word below are offered the verbs and their prompts quote that word; at `91e1ec683` the menu offers the verbs for every one of those punctuation selections. Not yet seen on a device. |
| 13 | Explicit permission before text goes to a third-party AI (Later, row 39; App Store guideline 5.1.2(i)) | Before the first request to each provider, from a study action, Ask or Find, the app asks once, "Send text to Claude (Anthropic)?" with that provider's name, and sends nothing until the reader allows it; choosing None, or clearing or changing that provider's key, withdraws it, and the app asks again. The note under the key field in Settings is body text instead of small grey text. | M / medium | All six (one rule) | Taken 15 October 2026: one question for each provider before its first request, on all six, in the wording the BACKLOG entry gives, withdrawn by choosing None or clearing that provider's key (Decisions still open, 5). Settled 21 October 2026: the question's body names a search, "the study action, and any question or search you type"; the permission ends with its key, so any change of that provider's key, cleared, replaced by another or missing after an Android restore that brings the preferences back without it, ends it and the app asks again before the next request; "Nothing is sent until you allow it." is approved as the study panel's and Find's message after Not now; Settings' model list and Test key may reach the provider with the key before any Allow, sending none of the reader's text; and the privacy policy's three sentences on the permission are approved. Decided on 22 October 2026 for the policy: each of the three sentences goes in the section it was drafted for, its words unchanged; after the record's comes a fourth, approved, "On Android this record is part of the app's settings, which Android may include in your device backup; it holds no key and none of your text."; and its effective date is the day 1.2.23 goes live. | Built on `fu/1223-ai-consent` from `rel/1.2.23` at `be1440fe0` (`754107e49`, `3789c0eaf`), and merged into it on 21 October 2026, where the question joins the short-window sweep of row 14 and clears the header on every window it opens (`b791120a4`). The question is a sheet the toolkit draws (`ai_permission.go`), so it is one rule on every platform: every pane's study menu reaches it through `dispatchAIAction` and the study panel, Ask through the same panel, and both Find panes through `findAsksPermission`. Allow records `ai.allowed.<provider>` in the app's preferences; since `7ef80e58b` it holds a fingerprint of the key the question was asked about, never the key (`aiAllowedRecord`), any change of the key (`keyStore.setAPIKey`) or choosing None (`keyStore.setAIEnabled`) erases it, and every check compares it with the key the request would send. `runAIActionForPassage` and `runAISearch` refuse a provider with no record for that key before the request leaves the device, whatever path reached them. A light/dark change or a rotation with the question up over the study panel brings the panel back about the same passage, with Ask's question as typed, saying nothing was sent, and the question over it, sending and recording nothing (`3789c0eaf`); over a Find pane the question closes, and the pane comes back saying nothing was sent, with the search in its field. Settings' Test key and its model list are not asked about, sending none of the reader's text. The key rule and the question's new body were built on `fu/1223-consent-2` from `rel/1.2.23` at `3261a1e13` (`7ef80e58b`), and merged on 22 October 2026 (`442a21938`): before it a key replaced by another kept the permission, and the same key pasted again after an Android restore was sent with no question. Over `3261a1e13`'s code `TestAReplacedKeyAsksAgainBeforeTheNextRequest`, `TestTheRequestsRefuseAKeyThePermissionWasNotGivenWith`, `TestPreferencesRestoredWithoutTheKeyHoldNoPermission`, `TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain` and `TestChangingTheKeyOrChoosingNoneWithdrawsThePermission` fail, the request going out with the replaced key, with the key changed while the question was open and with the restored record, and the question tests fail on the body; the contract test, which holds both requests to checking with the key they send, fails over its sources. `TestAKeyChangedWhileTheQuestionIsOpenIsAskedAboutAgain` (`79cfb6759`) also fails with Allow recording the key in place at the tap instead of the one the question was asked about. The privacy policy carries the three approved sentences in `docs/privacy.html`, to reach readers with 1.2.23's publish of the site. Built on `fu/1223-privacy-2` from `rel/1.2.23` at `da90dfb55`, and merged into it on 22 October 2026: each sentence stands in the section it was drafted for, the question in the AI section, its withdrawal in *Your choices, retention, and deletion* and the record in *Information stored on your device*, with the approved Android sentence after the record's (`17abf2caa`); `TestThePrivacyPolicyPlacesEachPermissionSentenceInItsSection` fails over `da90dfb55`'s policy. Its effective date still reads 11 October 2026, and is set on the day of the site's publish, which does not wait for the stores: stage 11 of docs/RELEASING.md gains the step, and `scripts/publish-site.sh` refuses a policy whose words or links differ from the live copy's unless its date is the day of the run, whatever the live copy's date, and one whose words and links are the live copy's while its date has moved to a day that is neither the live copy's nor the day of the run (`cmd/sitepages -check-policy-date`, `612b32a84`, `59f59be42`), before the reader is generated, so a publish it refuses spends no API.Bible quota, while a dry run goes on to its drift report and ends refused (`7f77c510a`). Against the policy live since 21 October 2026 it passes a render of the commit that policy was published from and refuses one of `da90dfb55`; the tests of the script's call and of stage 11 fail over `da90dfb55`, and the policy-date tests and the stage 11 test fail over `14ee36e32` on a second change dated the day the first went live, a date moved with nothing changed and a link that leads elsewhere. README.md's account of where each platform keeps an AI key, and of the permission's record, and docs/VERSION_STATES.md's rows on a restore that does not bring a key back, are made true of what is built (`478561c49`, `efd46978c`); the README's wording is a draft. `ai_permission_test.go` sends to a stand-in provider through the seams below the check; over 1.2.23's code, given only the study request's seam, the ten tests of `754107e49` fail, and `TestTheQuestionComesBackOverItsPanelAfterARebuild` fails over `3789c0eaf` with the question's reopen taken out. `ai_permission_contract_test.go` holds the panes, the two seams and the native sources; three of its four fail there, and the fourth, which held before, fails with a control file naming a provider host. Seen on 15 and 16 October 2026, each beside a 1.2.23 control that sent at once with no question: the question before any request, Not now sending nothing, Clear and None withdrawing, Find asking first and the Settings note as body text, on the Mac, an iPhone 16 Pro, an iPad Air 11, the API 35 and API 30 emulators and the Linux and Windows virtual machines; every word and both buttons on screen on an iPhone SE (375 points) in portrait and on the API 30 phone in both orientations; and, with `3789c0eaf`, the panel and the question back after a rotation and after night mode on both emulators and after a light/dark change on the iPhone 16 Pro, for Explain and for Ask with its typed question, with nothing sent. The key rule and the new body seen on 21 October 2026 at `97bd47e73`, beside `rel/1.2.23` at `3261a1e13`, in development builds with a throwaway probe in a scratch copy and both request seams answered in the app, on the Mac, the iPhone 16 Pro and iPhone SE simulators and the Linux virtual machine: the new body; Allow recording the key's fingerprint; the same key saved again asking nothing; a key replaced, the first key put back, and the key pasted again after it was taken out of where the app keeps it, past `keyStore.setAPIKey`, so the record stayed, each asking again, with nothing sent until Allow; and on the Mac, relaunched with the preferences kept and only the key taken out, no permission, and the key pasted again asking again. The control showed the old body and sent each of those requests with no question. Every word of the new body and both buttons on screen on the iPhone SE in portrait and in landscape. The Mac's development build keeps its keys in the preferences, so its relaunch stands in for a restore and is not one. On 22 October 2026 at `da90dfb55`, against `9a308352d`, on the API 35 and API 30 emulators, with real long presses on WEB Psalm 136, Gemini and two synthetic keys in the Keystore store, alike on both: the new body above Not now and Allow, with nothing sent; Allow recording the key's fingerprint; the second key saved clearing it, and Explain with that key asking again with nothing sent; after a restore made by installing the app again and putting its settings back, not through a backup transport, the record no longer matching and no key, Explain showing the panel that offers Open AI settings, with no question and nothing sent, and the second key pasted back asking again with nothing sent. The control showed the earlier body, recorded "1", and sent with the second key and with the key pasted back, with no question. On API 30 the body and both buttons fit whole in portrait; landscape was not captured. Not run: rotation on iOS, Find in the iPad's sidebar (the iPad Air in portrait draws the compact layout), and Ask on the Mac. |
| 14 | Desktop sheets in a window under about 420pt tall start inside the header (Stays in the backlog, row 8; in "Recapture the App Store and Play screenshots") | In a very short Mac, Windows or Linux window, a sheet too tall to open below the header covers the header, where it opened over its lower edge or partway down the Go to chip: a card sized by its text is only as tall as its content, and Go to's lists fill the room. | S / low | Mac, Windows, Linux; the Delete all notes question's height on all six | Taken 15 October 2026 (decision 35): a sheet that cannot clear the header covers it, and the window gets no new minimum height. Settled 21 October 2026: a covering sheet takes its natural height, capped at the room from the header's top, so a card sized by its text has no blank band under its content and a sheet whose lists or scroll areas use the room, Go to's, still fills it; and no separator above the Settings footer on any platform, the iPad included. Taken 22 October 2026: the AI answer's error state is kept whole, its message readable and its buttons in sight, by the rule that keeps its waiting line whole, covering the header where it must, on every desktop platform; a canvas too small even for that may still cut it, kept as it is and recorded. | FIXED 15 and 21 October 2026 for 1.2.23 (`b1daedf48`, `330f935ed`), on its own branch, `fu/1223-short-windows`, merged into `rel/1.2.23` on 21 October 2026. A sheet that, where it settles, starts above the room below the header and its 8pt gap covers the header instead, from the header's top edge (`coverHeaderUnlessClear`), after each sizing and with the window; as merged it took the window's height from there to as far above the window's foot. At 1280x386 eight do: Go to, the AI answer while it waits, the cross-references while their list loads, a link notice, the note-link offer, Ask, the note composer, which keeps all three excerpt rows, and the share confirmation with a long text. Each covers up to a height and clears the header above it: the link notice up to 440pt, the AI answer while it waits up to 413pt, and for Ask up to 434pt with a one-line question and higher with a longer one (up to 392pt before its waiting line was kept whole, below), the others up to between 395 and 418pt (the share confirmation up to 430pt at 507pt wide). The Delete all notes question and the model picker, which Settings opens over itself and which were not kept clear of the header before, now open below it. The tests and the measurements are in the BACKLOG entry; each test fails on the code before. Seen on Mac, Linux and Windows windows, and the Settings rule on Android, an iPhone and two iPads held upright, on 15 and 16 October, before `330f935ed`; not the iPad Pro held sideways. Seen again on Mac, Linux and Windows windows on 21 October at `f10f53496`: every sheet clear of the header or covering it, and the Delete all notes question, and on the Mac and Windows the model picker, clear where the code before started them inside it. The AI answer's waiting state, on a window just tall enough for the panel to clear the header, left a scroll shorter than "Reading the passage…" that cut the line through the middle: 39.6pt for a line 38.3pt tall starting 15pt down it at 1280x401 in the tests, and 36.8pt on a 568x320 phone held sideways. FIXED 21 October 2026 for 1.2.23 on its own branch, `fu/1223-ai-answer-short`, merged into `rel/1.2.23` on 22 October 2026 (`f4460eb9b`): the waiting scroll is never shorter than its column down to the foot of that line, Ask's question included, held to the most a sheet may be there (`sheetMostHeight`), so the panel covers the header up to 413pt for Explain and Compare and up to 434pt for Ask with a one-line question (419 and 440pt with the header 3pt lower, as on real windows), higher for a longer one, where all three covered it up to 392pt, and moves to clear the header when the answer arrives, or an error whose message and button fit below it (below). The line is whole on every window and screen the tests lay out, but where even the most a sheet may be is too short it can still be cut: for Ask with a one-line question on a 640x320 canvas under a 24pt status bar, where the code before cut all three, and for all three on a 732x240 canvas, on both builds; how to show it there is a decision (BACKLOG). One rule on all six, so no divergence. `TestTheAIWaitingLineIsWholeOnEveryWindow`, `TestTheAIWaitingLineIsWholeOverTheStyledPane` (the Windows and Linux pane), `TestTheAIWaitingLineStaysWholeAsTheWindowChanges`, `TestTheAIWaitingLineIsWholeOnPhonesHeldSideways` and `TestAQuestionTooLongForTheWindowKeepsTheWaitingPanelOnIt` fail on `rel/1.2.23` at `3261a1e13`, as do `TestTheAIWaitingLineStaysWholeAsTheQuoteOpensAndCloses` and `TestTheAIWaitingLineStaysWholeUnderAWrappedModelName`, which hold the line as the quote under the citation opens and closes and as a long model name wraps. Seen on 21 October 2026 at `19d77ab22`, against `3261a1e13`: on the Mac, over its own pane and the Linux and Windows panes, and on the Linux virtual machine, the line whole at 1280x386 (laid out at 401) for Explain and Ask, and on the Mac for Compare, at 1280x410 for Explain and at 1280x430 for Ask, and on the Windows virtual machine at 760x401 and for Ask at 760x420, where the control cut it each time. Not run on Android, whose test emulator reaches a canvas short enough only under a display density override, or on an iPhone. The error state kept the short scroll: in the tests, with a passage of a few words quoted, its two-line message was cut on 1280pt windows from 393 to 436pt (at 401pt through its first line) and Try again was out of sight, a scroll away, from 393 to 503pt, and to 524 and 525pt for Ask with a one-line question and for the three-line message. FIXED 22 October 2026 for 1.2.23 on its own branch, `fu/1223-ai-error-short` (`abba857dc`, `9e7eb94fd`), from `rel/1.2.23` at `91e1ec683`, not yet merged: the error keeps its body by the waiting line's rule (`wholeThrough`, `fitBody`'s least), never shorter than its whole column, the message and its button, Ask's question included, held to the most a sheet may be; where the panel cannot clear the header it covers it at its natural height. Settled 23 October 2026: the quotation opens automatically only where it leaves that state's minimum body as well as the usual few answer rows. The old 96pt allowance could open the quote when the error needed 108.6–153.2pt, covering the header unnecessarily with a long passage or cutting Try again on an iPhone 16 Pro held sideways. The citation and disclosure remain visible; the reader can open the quote, and explicit open or closed choices survive failure, retry and window resizing. No icon, button or spacing dimensions change. A question too long for the window can still put the error below the visible scroll, which opens at the question's head; showing the error above the question or scrolling its head away remains a separate decision. A whole safe area too short for the column retains its scroll and height cap. The regressions cover short desktop windows, styled panes, phone landscape safe areas, quote toggles and resizes, plus automatic collapse with a long quote at 1280x620 and a missing key at 1280x480, long waiting questions, and explicit disclosure choices across errors and retries. The earlier error tests fail on `91e1ec683` apart from the already-capped 732x240 cases. The 22 October Mac checks apply to `a7b1b670d`, before the automatic quote rule: its error was whole at 1280x401, 436, 470, 503 and 520, and through a resize and retry, but its quote made it cover the header again at 520 after clearing it at 503. The new quote rule awaits integration and device checks; the Linux and Windows panes mimicked on the Mac, both virtual machines, an iPhone and Android remain unverified for this branch. The two points left open were settled on 21 October 2026 and built on their own branch, `fu/1223-short-windows-2` (`0c2e48e40`, `a4598d375`), from `rel/1.2.23` at `3261a1e13`, and merged into `rel/1.2.23` on 22 October 2026. A covering sheet takes its natural height: its preferred height, what it would take with unlimited room, never less than its content's settled least height, capped at the room from the header's top, with its top edge at the header's. A card sized from its content asks for nothing beyond it; Go to asks for the 680pt its lists take on a roomy screen, the cross-references for their 460pt panel and the AI answer for its whole body, so they fill the room where it is shorter. With the waiting line kept whole, the waiting panel covers no shorter than its chrome and the line's least, and its whole column is taller than the room wherever it covers, so it fills the room; `TestACoveringWaitingPanelTakesItsNaturalHeightWithItsLineWhole` holds both on every 1280pt window from 386 to 440pt, and fails on `fu/1223-short-windows-2` alone, which cut the line, and with the panel covering at its least alone, as a card does. Fyne centres a modal on its canvas, so a sheet shorter than the room is laid out on the window's canvas raised by the difference (`raiseSheet`), its popup and shade still covering the whole window, which the sweep holds (`9c3da08a4`); the raise rests on Fyne 2.7.4's popup, and has to be built again for Fyne 2.8, which places a popup through an overlay container (`5bc64cbb8`). At 1280x386 a link notice, the note-link offer, Ask, the note composer and the share confirmation are as tall as their content, where the first four showed 108.7 to 116.7pt of blank under their last rows and the share confirmation 17.3pt, the study panel with a short answer or an error shows its whole body, as on a window with room to spare and no taller, and Go to, the cross-references while they load and the AI answer while it waits fill the room; at 1280x401, the least height a Mac window's canvas is laid out at, the share confirmation, the cross-references while they load, the AI answer while it waits, a link notice and the offer cover the header, the cross-references and the waiting AI answer filling the room, and the link notice and the offer showed 123.7 and 131.7pt. The separator above the Settings footer is gone on all six (`a4598d375`), as in 1.2.22. `TestACoveringSheetTakesItsNaturalHeight` and `TestTheSettingsFooterHasNoRule` fail on `3261a1e13`. Seen on 21 October 2026 in scratch development builds of `0873d2665` beside `3261a1e13`, neither with the waiting line's least: on the Mac, in a window asked for at 1280x386 and laid out at 1280x401, where the header stands 3pt lower than in the tests, the note-link offer, a link notice, Ask and the note composer cover the header from its top edge as tall as their content, where the control's showed 125.7, 117.7, 82.5 and 82.5pt of blank (the tests' 401pt window gives 131.7 and 123.7pt for the offer and the notice), each raised popup spanning the window to its foot; Go to and the waiting cross-references fill the room and the waiting AI answer clears the header on both builds; the offer resized from 401 to 800pt and back clears the header and covers it again; and at 1280x800 Settings has no rule above its footer. The Linux and Windows panes, mimicked on the Mac, gave the same numbers, and an iPhone 16 Pro simulator and the Android API 35 emulator showed no rule above the footer, where the control drew one 7pt below the body. Not seen on an iPad, nor on the Linux and Windows virtual machines. |
| 15 | Shares that cannot finish say so, in one approved line (Later, row 2) | When a share cannot finish, the app says so in one short line instead of ending without a word: "Could not open your email program." under the desktop sheet's buttons when Email… fails, and "Could not share the card." for a picture never saved, a picture no folder took and a card that cannot be drawn; on the iPhone and the iPad a share sheet iOS would not show opens once the sheet before it has closed, or the share says it could not be made. | M / medium | Linux, the Windows fallback sheet and the macOS mimic for the save and Email… cases, and the Mac's own share for a card gone at the tap; all six for a card that cannot be drawn; iOS for a refused presentation | Taken 15 October 2026 (decision 7, Draft A): one line under the buttons when the compose fails, "Could not open your email program."; the phones' "Could not share the card." for the picture failures and a card that cannot be drawn; on iOS the refusal detected, the sheet presented from the top view controller and, still refused, the existing "Could not share the passage." or "Could not share the card.". The stop at the first route that started a compose went into 1.2.22 on 14 October 2026 (row 18 there) and is kept. The three rules the build adds were confirmed on 21 October 2026: the Mac's own share of a card gone at the tap says "Could not share the card."; a compose the portal took and gave no answer to says nothing; and on iOS a second share asked for while the first sheet is still held back is not made. | FIXED for 1.2.23 on its own branch, `fu/1223-share-fail-line`, merged into `rel/1.2.23` on 21 October 2026. On the desktop sheet (Linux, the Windows fallback, the macOS mimic) a compose that fails adds the line under the buttons once its answer is back, the rest of the sheet as it was; the line answers the last press, and goes when a later compose opens, the reader cancels one at the portal (its answer 1), or the portal takes one and gives no answer (`mailUnanswered`), since the portal may yet open it; a compose still running across a light/dark change answers to the sheet the change brought back. Every failure the desktop answered is taken at its word, `xdg-email`'s exit 4 among them, so a mail program `xdg-email` ran itself that opened a compose and then exits with a failure while the sheet is still open gets the line too. On a short window the line is sized in by the sheet's own fit, so decision 35's rule, that a sheet which cannot clear the header covers it, holds for the line with both merged; the branch alone let the line move the sheet into the gap under the header (420 by 470 points), where `TestTheEmailLineKeepsTheShareSheetClearOfOrOverTheHeader` fails (`b791120a4`). A card that cannot be read at the tap, and a picture no folder took that the file manager did not show, end in the sheet with "Could not share the card." and Done alone (`sayShareFailed`); a picture no folder took that the file manager did show keeps its Picture saved sheet. On all six a card the preview cannot draw opens no preview and says "Could not share the card.": on the desktops in that sheet, on iOS in the share's alert, on Android in its toast (`BtBridge.shareFailed`). The Mac's own share of a card gone at the tap says it too, where its picker was handed the missing file's address (PLATFORM_MATRIX.md, divergence 27). On iOS every share sheet and the notice go through `btIOSPresent`, which waits out the top view controller's transition, presents from the top view controller, counts the presentation taken by its completion, its presenter, its window or its place among the presented view controllers, judges it only 5 seconds later, since the share sheet holds its own presentation back while its services load, and, refused, tries once more and then says the share's words; one presentation is on its way at a time. One rule on all six, so no divergence. `TestAnEmailThatCannotOpenSaysSoUnderTheButtons`, `TestTheEmailLineAnswersTheLastPressOnly`, `TestAPictureNoFolderTookSaysTheCardCouldNotBeShared`, `TestACardThatCouldNotBeReadSaysTheCardCouldNotBeShared`, `TestACardThatCannotBeDrawnSaysTheCardCouldNotBeShared`, `TestTheMacsShareOfACardGoneAtTheTapSaysSo`, `TestIOSSaysWhenItWillNotShowTheShareSheet` and `TestThePhonesSayACardThePreviewCannotDraw` fail on `rel/1.2.23` at `be1440fe0`; `TestAnEmailAnswerAfterALightDarkChangeIsSaidOnTheSheetThatCameBack`, `TestAPortalThatGaveNoAnswerIsNotSaidToHaveFailed` and the no-code case of `TestTheEmailPortalIsHeardAsItAnswers` fail on the branch's first commit, `d7f6b9156`. Seen on 21 October 2026 in development builds on the Linux VM, the Mac (Linux mimic) and the iPhone 16 Pro and iPad Air simulators, against `d7f6b9156` and `be1440fe0` (BACKLOG.md, the Email… entry): on a freshly booted simulator `d7f6b9156` said "Could not share the passage." over a share sheet that had opened in every run (6 on the iPhone, 5 on the iPad) and this change in none, every sheet opening. Not seen this time on Windows or Android. Gathers "Email… on the desktop share sheet fails without a word", the open tails of "Android: a text share has no failure path" and "Linux and Windows: an in-app share sheet in place of the 1.4-second notice", and the line 1.2.20's row 1 leaves out. The 20 September rule, that a share the reader started does not end in silence, and the desktop's no-sheet rule pull opposite ways here; one change with one approved line settles both on every platform. The double compose that 1.2.22 stops was seen on the Linux virtual machine with a stand-in mail client (1.2.22, row 18). A drawing failure cannot really happen with the fonts the app bundles. |
| 16 | A reader who cleared their key on purpose is told at every launch that the NKJV could not be opened (Later, row 23; VERSION_STATES.md, *Open decisions*) | A reader who removed their API.Bible key in Settings is no longer told at every launch that the NKJV could not be opened and will be tried again: the launch gives it up, and the picker says "You removed your API.Bible key, so World English Bible is shown in place of New King James Version.", or, where the key removed was the one included with the app, "You removed the included API.Bible key, so World English Bible is shown in place of New King James Version." | S / low | All six | Taken 15 October 2026 (decision 11): stop trying the NKJV at every launch, and those words in the picker. Decided on 21 October 2026: the line names the key removed, the words above for the reader's own, unchanged, and for the key included with the app the second sentence above. Confirmed on 22 October 2026, as built: a key typed into the field over the included one and deleted again is named as the reader's own, since the field saved it as theirs, and a removal recorded by a release before 1.2.23, which wrote the same record for either key, is named as the reader's own. | FIXED for 1.2.23 on its own branch, `fu/1223-key-gone` (`e43d8e64e`), with row 17, brought up to `rel/1.2.23` at `3261a1e13` by a merge (`158de2181`), and merged into `rel/1.2.23` on 22 October 2026 (`993b52639`). Where the store answers that it holds no key and the reader emptied the field, the record that also keeps the included key off (`bibleKeyRemovedOnPurpose`), the launch records the NKJV in `keyRemovedVersion`, which nothing saves, rather than `preferredVersion`: the next save names the WEB and no launch tries the NKJV again. The sentence shows while the WEB is on screen in its place and the key is still gone; any landing spends it, and a key added again takes it away. Emptying the field, with Clear or by hand, while a launch's record of the NKJV stands (`D9`) gives it up the same way. A key entered in the field again before anything else lands makes the NKJV the remembered choice again, as `D9` records it, since the key was the only reason it was given up (`takeBackKeyedTranslation`, `d5bed801e`): without it, a reader replacing their key by emptying the field and pasting it again lost the NKJV with nothing said. A store that cannot answer keeps `D9`'s record. A reader of a release build who taps Clear while only the key included with the app is in force turns that key off for good and meets the same condition: the next launch gives up the NKJV saved as their choice. The picker names that key, on its own branch, `fu/1223-key-gone-2` (`a206ffa18`), from `rel/1.2.23` at `da90dfb55`, merged into it on 22 October 2026 (`dc312b8ee`): emptying the field, with Clear or by hand, records which key was in force when it went (`removeBibleKey`) in the record that already keeps the included key off, `bible.key.cleared`: `included` for the key included with the app, or for the copy an older release seeded on the device, and `1`, the value every release has written, for the reader's own. The picker reads it (`removedBibleKeyWasIncluded`). A key typed into the field and deleted again was the reader's own when it went, since the field saves each change, and is named as theirs. A removal with no key in force removes nothing and keeps a record of the included key (`072595af8`): where the store refuses the delete, Clear stays enabled under "Couldn't remove the stored key. Please try again.", and a second tap wrote `1`, so the picker named the reader's own key once the store answered. A removal recorded by a release before 1.2.23 holds `1` whichever key it was, and is named as the reader's own: the app cannot tell which it was. `TestTheRemovedKeyIsNamedInThePicker` holds Clear with the included key in force, in the session and at a launch whose saved place still names the NKJV, the reader's own key removed with Clear or by hand in the same build, and a key typed in and deleted again; its cases for the included key fail on `rel/1.2.23` at `da90dfb55`, whose picker named the reader's own key for both. `TestAClearTappedAgainOverARefusedDeleteNamesTheIncludedKey` holds the second tap over a store that refuses the delete and then answers, and fails on `15b23f15b`. The launch walk's `key-cleared-included` fate, a release build whose included key is cleared through `removeBibleKey`, puts `L-A` and `L-H` to the included key removed, and the credential walk holds it as a tenth knowledge state (`fa89b14f5`); with the picker naming the reader's own key, as before `a206ffa18`, the fate's two cells fail `L-H`. Seen on 22 October 2026 at `15b23f15b` on the Mac, in a development build with a synthetic included key linked in as a release build's is, the NKJV saved and its source out of reach, beside `rel/1.2.23` at `da90dfb55`: Clear with the included key in force recorded `included` and the picker said "You removed the included API.Bible key, so World English Bible is shown in place of New King James Version.", where the control recorded `1` and said "You removed your API.Bible key, so World English Bible is shown in place of New King James Version."; a key of the reader's own typed in and cleared was named as theirs on both. After Clear the app saves the WEB as the reading place, so a plain relaunch says nothing; a relaunch whose saved place still names the NKJV, as when the app stops before saving, says the included sentence again. Not seen on any other platform. Settings and the picker are the toolkit's on all six, so it is one rule; the web reader has no keys. In the launch walk `L-A` asks of `key-cleared` that the next save name the WEB, and `L-H` that the picker say why word for word, give no cause the launch did not have and promise no launch for a translation given up; `TestAKeyRemovedOnPurposeGivesTheTranslationUp`, `TestAKeyClearedWhileRememberedIsGivenUpThere` and the hand-off test's new case (`D18`) hold the next launch, the session and the hand-off. On `rel/1.2.23` at `be1440fe0` all of them fail. `TestAKeyEnteredAgainMakesTheTranslationTheChoiceAgain` fails at `eee620033`, before the key entered again was handled, and `TestAStoreThatCannotAnswerKeepsTheRecord` holds the unanswered store. The other two version-state questions stay in the backlog (row 19 there). |
| 17 | Recovery message for a missing saved key (Later, row 10; decision 13) | Each empty AI or API.Bible key field whose saved key is definitively absent says: "Your saved key isn't available on this device. Paste it again here." A small first-line-aligned ✕ dismisses that account's line on the device. | S / low | Android, iPhone, iPad and signed Mac builds. Windows, Linux and the Mac's direct download keep keys in Preferences and have no separate store to compare. | Settled 23 October 2026: a saved mark, a separate credential store's definitive item-not-found answer and no nonblank Preferences copy establish the same missing-key condition on Android and Apple. Store errors do not qualify; the app does not infer a restore or cause. The dismissal remains device-scoped and per account. The ✕ keeps its 14pt glyph and 24pt face aligned with the line's first row; the original button minimum still sizes its row and column. An invisible touch-only target is capped at the lesser of its allocated column and 44pt on each axis. It does not grow the row, cross the column, borrow the text's padding or shift upward toward Clear. Desktop has no extra proxy. | Integrated locally in `rel/1.2.23`, uncommitted, from the round-four branches. The current wording, signed-Mac parity and confined touch geometry await the combined checks. Apple keys remain `AfterFirstUnlock`; only the dismissal is `AfterFirstUnlockThisDeviceOnly`, containing `1` and no key. A same-device Apple backup restore may retain it; another-device restore does not. Android stores it in `no_backup/`, lost after reinstall or reset. PLATFORM_MATRIX.md divergence 38 is resolved. Historical checks on 22 October 2026 covered recovery fields and dismissal on Android API 35/API 30, and missing-key display on portrait iPhone 16 Pro and iPad Air simulators; Keychain resets modelled absence, not real backup transport. Earlier branch target measurements that grew the row or column do not describe this geometry. `TestTheMacKeychainReportsDefinitivelyMissingSavedKeys` checks the Apple gate with in-memory reads; `TestTheNotRestoredLineStaysDismissedAfterSameDeviceRestore` models retained dismissal state. Existing field, picker, mark and dismissal regressions remain. Real backup transport, Mac App Store runtime, new touch geometry on devices and screen readers remain unverified. |
| 18 | Android: Translate and drag carry invisible layout characters (Later, row 11) | On Android, a selection that takes in a heading or a psalm title, dragged into another app or handed to Translate, carries the text as Copy already gives it: plain spaces where Android 15 and later set a heading's or a title's gaps as four-per-em spaces, and no direction mark at the head of a heading or a title that opens on a Hebrew letter, such as the NKJV's Psalm 119 stanza headings. The reading toolbar, which offered no Translate, offers in its overflow each action on selected text that an installed app offers, a translation app's Translate among them. | M / medium | Android | Taken 15 October 2026: clean the dragged and translated text as Copy is cleaned, and keep the drag; taken 21 October 2026: add Translate, the system's text-processing actions reachable from the reading toolbar and handed the text as Copy cleans it, read-only (Decisions still open, 14). Open there: a text an app returns, and whether the Mac, Windows and Linux offer Translate as well. | Built for 1.2.23 on its own branch, `fu/1223-android-drag-clean` (`2980987e5`, with `c5f626aa1`, `627e514da`, `f2f628ea5` and `598191943` after review, and Translate in `61cb2b9c5`), from `rel/1.2.23` at `be1440fe0`, with `rel/1.2.23` at `3261a1e13` merged in (`c8bced090`), and merged into `rel/1.2.23` on 22 October 2026 (`22eb8eef4`), where the toolbar's `onPrepareActionMode` both sets each such item's listener, on every preparing, and keeps Study with AI to a selection that holds Scripture (row 12). Two of those messages say more than is so. `2980987e5`'s says a toolbar item that hands the selection to another app took the view's own text and now takes the text as read, when no such item was offered before `61cb2b9c5`; that every other path stayed the platform's, when the accessibility node changed (`627e514da`); and it names the system's Translate beside the Mac's drag, when the Mac's menu offers no Translate. `f2f628ea5`'s writes the two characters themselves where it means their escape spellings, a backslash, the letter u and the four hex digits, so it reads as if the left-to-right mark itself, planted in a string, had passed the earlier check, when what passed was the mark's escape spelling in an Objective-C string. One function, `selectionAsRead`, gives the selection as read (`readerText`) where that differs from the view's text, and Copy (`copyAsRead`) and the drag take their text from it. The drag: a long-click listener on the reading view starts the drag the platform starts on a long press inside the selection while the toolbar is up, with that text, the platform's global flag and a shadow of its first 20 characters, and closes the toolbar after (`dragAsRead`); every other long press, and a selection with nothing to give back, runs as before. Inside the selection is the platform's own test, on the lowest and the highest offset under the touch's fingers, kept and cleared where the platform keeps and clears its own (`noteTouch`, `resetTouchOffsets`). From Android 13 the listener would show the whole reading text to accessibility services as long-clickable, so the view takes the long click back out of its node, as the platform does for such text with no listener. A drag started with a mouse or a trackpad is the platform's own (the mouse branch of `Editor`'s selection controller) and still carries the characters; that is from the platform's source, not seen on a device. So, by the same source, do the reading view's accessibility Share action and the Share of a mouse's right-click menu, which the platform performs with the view's own text and no citation (`TextView.onTextContextMenuItem`, `ID_SHARE`), the plain-text Share the toolbar leaves out; this item does not change them. Translate, and every other app's action on selected text (`ACTION_PROCESS_TEXT`): until `61cb2b9c5` none was reached, since the platform adds those items, and the matching accessibility actions, only to a view with an id (`TextView.canProcessText`, in its source for Android 7 to 16) and the reading view had none, and from Android 11 an app sees other apps' text actions only through a `<queries>` entry, which the manifest lacked. Now the reading view has an id and the manifest the entry. Each item's click goes to `processTextAsRead`, for every selection, with the selection as Copy gives it (`selectionTextAsRead`), in a copy of the item's intent marked read-only (`EXTRA_PROCESS_TEXT_READONLY`), started for a result nothing reads, so nothing an app returns can reach the page; the platform's own handling of a result for text that cannot be edited, a toast of any text returned and the selection collapsed on a cancel, is not done. The accessibility actions the platform offers for the same apps, which it would perform with the view's own text, are replaced in the reading view's node by its own, performed with `processTextAsRead` (`processTextActions`, `performAccessibilityAction`). A Translate a text classifier suggests stays off with smart selection (`TextClassifier.NO_OP`), so Translate appears where an installed app offers it, in the toolbar's overflow, where the platform puts every such item. PLATFORM_MATRIX.md, divergence 37, records which platforms offer Translate. `TestAndroidDragAndTranslateCarryTheTextAsRead` holds both paths and fails on `be1440fe0`, its item's half on `c8bced090`; `TestAndroidReadingToolbarOffersOtherAppsTextActions` holds the id, the `<queries>` entry, the accessibility actions and that no result is read, and fails on `c8bced090`; `TestAndroidDragListenerKeepsThePlatformsAccessibilityNode` holds the node and fails on `c5f626aa1`. On the emulators, with an invented heading that opens on a Hebrew letter added to WEB Psalm 3: on 16 October at `4a1822e62`, a Select all dragged onto the page dropped the same text as Copy on API 35 and API 30, where the control on `be1440fe0` dropped twelve four-per-em spaces and a direction mark (API 35) or the mark alone (API 30), and no process-text item appeared on either API or build, even with a `<queries>` entry for `ACTION_PROCESS_TEXT` and an app that offers one; on 21 October at `598191943`, on API 35 only, the same drag again dropped Copy's text where `be1440fe0` dropped the twelve spaces and the mark, a long press outside a selection selected the word there and started no drag, a one-word drag gave the same text on both builds, and the reading text's node read not long-clickable, idle and with the toolbar up, as on `be1440fe0`, where `4a1822e62` read long-clickable. API 30 was not run again, since the system image it starts from is no longer on disk. Translate was seen on 22 October 2026, in development builds of `d0eae99b0` beside `rel/1.2.23` at `da90dfb55`, with a throwaway app that offers an action on selected text standing in for a translation app, which the emulator images lack: on API 35, after Select all on BSB Psalm 3, the toolbar offered that app's item, where the control offered no other app's item at all, and the app was handed text identical to Copy's, without the 13 four-per-em spaces the psalm's title and heading hold, read-only and started for a result, and so was a selection from the BSB's Genesis 4:16 through the heading into 4:17, which holds 3; on API 30 the item was offered and handed Copy's text, but the pages there carry neither layout character, so only the offer could fail. Not run: a text an app returns, the selection on return, the accessibility actions, and the drag and drop with the item in place. The iPad's and the Mac's text views keep their native drag, and the iPhone's and the iPad's menus keep the system's Translate (the Mac's menu leaves it out). Their pages carry neither character: the Apple dialect sets a heading and a title ragged by its stylesheet and writes no character for the page's layout, so their drag and Translate hand on what their Copy does, and nothing changes there (`TestTheApplePanesCarryNoAndroidLayoutCharacter`; on the iPad simulator on 16 October neither character was in the text of WEB or BSB Psalm 3). *Not in BACKLOG.md.* |

What is left open, each in its row and its entry:

- Row 1: a launch of the shipping app held sideways, still to be seen on
  hardware; the read of the window scene on iOS 15 to 25, not run, since
  only the iOS 26.5 and 27.0 simulators were at hand; and on Android a
  window near the 600 dp tablet line or the 560 dp bar dress, whose layout
  can still change at the first paint.
- Row 2: the tablet held upright, where the column moved 6.5 px to the
  pane's centre, as on the iPad. Keep that, or count the reading body's
  right pad as part of the pane.
- Row 3: checks not run, the Windows and Linux virtual machines, the BSB
  on the emulator, and the iPad on row 4's fallback page.
- Row 5: divergence 35 in PLATFORM_MATRIX.md, a backspace that removes one
  character of a joined emoji everywhere but Android.
- Row 6: the corrections in `5df78e565`, the box drawn for a glyph no face
  has and a mark on a space, were made after the device checks.
- Row 7: the site's publish. `fu/1223-gap-marks-named` was merged on 22
  October 2026: its `2aa12b083`, which leaves the mark bare beside the
  verse after the hole, was seen on 21 October 2026 on the Mac, the
  iPhone 16 Pro simulator and the emulator (API 35); its `4ce5575e4`,
  which lights the marks of the holes a note from another edition names,
  has not been seen on a device. Of the three rules the
  build added to decision 18, two were confirmed on 21 October 2026 and
  one rejected and taken out (row 7). Seen on 15 and 16 October 2026,
  before `67c199afc`, on the Mac, the iPhone 16 Pro and iPad Air 11
  simulators, the emulator (API 35 and 30), the Linux and Windows virtual
  machines and the website built offline, each against `rel/1.2.23`. On the
  iPhone and iPad the case for where the verse before a hole ends could
  not fail, a heading standing between them in Matthew 17, so the WEB's
  Acts 8 is still to run there; and the narration that lights no mark
  (`67c199afc`) has not been seen on a device. In the apps the mark is drawn
  only with the footnotes on, not the default, so with them off such a
  link still lights nothing.
- Rows 8 and 9: none; both are closed. `fu/1223-nkjv-checks`, whose
  record they carry as a cherry-pick (`5b4f39d14`), was deleted on 21
  October 2026, its one commit's changes being on this branch with only
  this page's date line left out. The second word of
  Matthew 15:8, black at the source beside its note, can be checked only
  against a printed red-letter edition.
- Row 10: the site's publish, which changes the forty Esther chapter
  pages; not run, the emulator at API 30 and the NKJV on a device.
- Row 11: the site's publish; on the iPad the footnote rows were not read
  back.
- Row 12: the follow-up of 21 October 2026, no study verb for a selection
  that holds no Scripture, merged and seen on every pane, Android's on
  22 October 2026, where its toolbar followed a selection moved in code
  under it, not one dragged by its handles; the prompt's passage of 22
  October 2026, merged that day, seen on the Android emulators, the
  iPhone and iPad simulators and the Mac, on Windows and Linux only in
  the Mac's mimic, and on no WEB chapter; no study verb for a selection
  whose only Scripture is punctuation, taken on 22 October 2026 with the
  follow-up's points confirmed (below) and built that day on
  `fu/1223-no-study-punctuation` (`ca86cca13`), not yet merged or seen
  on a device; the unfinished-boundary-word fix below, whose updated
  tests and native platform checks have not yet run; and divergence 36
  in PLATFORM_MATRIX.md, a heading's words
  that cannot be selected on Windows and Linux, open until the next
  major's headings pass (Next major version, row 5).
- Row 13: the privacy policy's effective date, set on the day of the
  site's publish (docs/RELEASING.md, stage 11), which was confirmed on 22
  October 2026 as the day the website is published with 1.2.23; and
  README.md's paragraph on where each platform keeps an AI key, a draft
  (below). The key rule and
  the question's new body were seen on the Android emulators on 22
  October 2026, with a restore made by installing the app again and
  putting its settings back, not through a backup transport, and the
  body's fit on the API 30 phone in portrait only. Not run: rotation on
  iOS, Find in the iPad's sidebar, and Ask on the Mac.
- Row 14: the natural height of a covering sheet and the Settings footer
  without its separator, settled on 21 October 2026, were merged on 22
  October 2026. Seen on the Mac, with the Linux and Windows panes
  mimicked there, and the footer on an iPhone and Android; not yet on an
  iPad, nor on the Linux and Windows virtual machines' own windows. How
  to show the AI answer's waiting line where even the most a sheet may be
  is too short (below); the waiting line not run on Android or an iPhone.
  On real Mac, Linux and Windows windows the header ends 3pt lower than
  in the tests, so each covering threshold there is about 3pt higher; the
  blank band a covering card had before this change was 117.7pt in a link
  notice and 125.7pt in the offer on the Mac's 401pt window, 6pt less
  than the tests' 123.7 and 131.7pt.
- Row 15: this change not yet seen on Windows or Android. The Windows
  Store install's own share is Later minor releases, row 40's.
- Row 16: the line for the key included with the app, decided on 21
  October 2026 and merged on 22 October 2026; seen on the Mac on 22
  October 2026 and on no other platform, and said again at a launch only
  while the saved place still names the NKJV. Its two points that waited
  for confirmation were confirmed on 22 October 2026 (below).
- Row 17: the ✕ that dismisses the field's line, decided on 21 October
  2026 and merged on 22 October 2026; seen with the words of 21 October
  on the API 35 and API 30 emulators on 22 October 2026, after a restore
  made by installing the app again and putting its settings back, not
  through a backup transport. Not run: TalkBack, and the Windows and
  Linux virtual machines. Its 23 October revisions are integrated locally,
  uncommitted, with the neutral field line, signed-Mac parity, device-scoped
  dismissal and confined mobile target; combined checks and new runtime
  verification remain.
- Row 18: a text an app returns, and whether the Mac, Windows and Linux
  offer Translate as well (decision 14). Not run on the emulators: the
  accessibility actions, a text an app returns, the selection on return,
  and the drag and drop with the item in place; on API 30 the pages carry
  no layout character, so there only the offer could fail. Left as they
  were: the reading view's accessibility Share and a mouse's right-click
  Share, which send the view's own text with no citation.

Seen on the merged tree at `a94a9ce5f` on 21 October 2026, in development
builds with a throwaway trigger in a scratch copy, values read back from
the app: on the iPhone 16 Pro simulator (BSB), "Cain" in "The Descendants
of Cain" shared as the citation line alone, Genesis 4:17, and "Cain" in
verse 16 quoted from 4:16 (row 12); the first Explain, with a stand-in
key and the request seam stubbed, showed "Send text to Gemini (Google)?"
over the panel, Not now left no request and no permission recorded and
the panel saying "Nothing is sent until you allow it.", and Allow on the
next let one request through to the stub (row 13). On the emulator (API
35, BSB), a real long-press on that heading's "Cain" and Share with
citation from the toolbar sent the citation line alone, Genesis 4:17,
and "Cain" in verse 17 was quoted (row 12); a link to Matthew 17:21 with
the footnotes on landed at verse 22 with only the "[21]" mark lit (row
7). On the Mac (WEB Catholic, footnotes on), the pane drew Daniel 3's
heading as the 35-character title above verse 24, and its footnote
section listed 1, 24, 77 and 91 (row 11); in a 1280 by 430 window the
link notice covered the header, and in a 1280 by 800 one it cleared it
(row 14).

Seen on the merged tree at `f4460eb9b` on 22 October 2026, in
development builds with throwaway triggers in a scratch copy, stand-in
keys and the request seams answered in the app, each beside a build of
`rel/1.2.23`'s code before the four merges (`3261a1e13`, the code of
`9a308352d`). On the iPhone 16 Pro simulator, the first Explain showed
the question with its body of 21 October word for word, where the
control showed the earlier body (row 13); and "Cain" selected in the
heading "The Descendants of Cain" (BSB, Genesis 4) and the heading "The
Fourth Day" selected whole were offered Share and Cross-references and no
Study with AI, the menu on screen reading Copy, Share, Cross-references,
where the control offered Study with AI for the heading's "Cain", while
"Cain" in verse 16 kept Study with AI and Explain opened the panel on
4:16 (row 12). On the Mac and the iPhone simulator, a key replaced by
another, the first key put back, and a key pasted again after it was
taken out past `keyStore.setAPIKey` each brought the question back with
nothing sent until Allow, where the control sent each with no question,
the replaced key's request with that key (row 13). On the Mac at the
shortest window, 1280 by 386 laid out at 401, the AI answer's waiting
line was whole for Explain, Compare and Ask, the panel covering the
header, where the control's scroll, 33.6pt tall, cut it (row 14). Not
run: Android, whose emulators have no approved start script, so rows 13,
16 and 17 still have Android checks to run.

Those Android checks ran later on 22 October 2026 at `da90dfb55`, on the
API 35 and API 30 emulators, each beside `9a308352d`, with the restore
made by installing the app again and putting its settings back, not
through a backup transport: rows 12 and 13 as their rows say; and row
17's lines, the picker saying "Your API.Bible key wasn't restored, so
World English Bible is shown in place of New King James Version." and
both key fields "Your key wasn't restored — keys never leave the device
they were added on, so a backup or reinstall doesn't bring them back.
Paste it again here.", where the control gave the sentence of decision
9 and the fields' hints. The Keystore held neither key after the
restore on either build.

Seen on the merged tree at `b3c3b4312` on 22 October 2026, after the six
later merges, in development builds with throwaway probes in scratch
copies, synthetic keys and the request seams answered in the app, values
read back from the app. On the emulator (API 35): after Select all on the
BSB's Psalm 3, whose title and heading hold the four-per-em spaces, a real
tap on a stand-in app's item in the toolbar's overflow handed it text
identical to Copy's, with no layout character, read-only and started for
a result, the toolbar offering Study with AI beside it (row 18); a key
replaced by another asked again before the next Explain, with nothing
sent until Allow (row 13); and after a restore made by installing the app
again and putting its settings back, a real touch on the ✕ beside the AI
key field's line put that line away, leaving the API.Bible field's, and
it stayed away at the next launch (row 17). On the iPhone 16 Pro
simulator (BSB): the first Explain showed the question with its body word
for word, and after Allow a drag from Genesis 1:2 through "The First Day"
into 1:3 reached the stubbed request as "waters." and "And God said"
under Genesis 1:2–3, with none of the heading's words (rows 12 and 13).
On the Mac: in a window asked for at 1280x386 and laid out at 401, the
note-link offer covered the header from its top edge as tall as its
content, 0.0pt beyond it, its popup spanning the window (row 14); at
1280x800 Settings drew no rule above its footer (row 14); and a link to
the WEB Catholic Edition's Sirach 26:28 with the footnotes on washed
verse 28 alone, the nine marks before it bare (row 7).

### Wording and decisions still open for 1.2.23

The two lines a reader sees that were in the tree as drafts were approved
on 21 October 2026:

- "Nothing is sent until you allow it." (row 13): a sentence of the
  approved question, reused as the study panel's and the Find pane's
  message after Not now (`aiPermissionNotSent`, `ai_permission.go`).
- "The cross-references don't cover the deuterocanonical books." (row
  10): the panel's second line for Tobit, Judith, Wisdom, Sirach, Baruch
  and 1 and 2 Maccabees, on all six (`crossRefDeuterocanonLine`,
  `crossref_list.go`). The words are not new, but no shipping build
  showed them before.

The same day the question's body (row 13) became "Study with AI sends
the passage you select, the study action, and any question or search you
type straight from this device to {Provider}, using your key. They may
keep it under your account terms. Nothing is sent until you allow it. To
withdraw, choose None in Settings or clear your key.", merged on 22
October 2026 (`442a21938`). Rows 16 and 17 carry decision 11's line,
taken on 15 October 2026, "You removed your API.Bible key, so World
English Bible is shown in place of New King James Version.", and
decision 13's two lines in the words of 21 October 2026, under an empty
key field "Your key wasn't restored — keys never leave the device they
were added on, so a backup or reinstall doesn't bring them back. Paste
it again here." and in the picker "Your API.Bible key wasn't restored,
so World English Bible is shown in place of New King James Version.",
merged on 22 October 2026 (`993b52639`). Also decided on 21 October
2026: a second form of row 16's line, for the key included with the
app, "You removed the included API.Bible key, so World English Bible is
shown in place of New King James Version.", and a ✕ that dismisses row
17's field line, with no words of its own; both were merged on 22
October 2026 (`dc312b8ee`).

The current recovery messages describe only a saved key's absence. The key
field reads "Your saved key isn't available on this device. Paste it again
here." The picker reads "Your saved API.Bible key isn't available on this
device, so World English Bible is shown in place of New King James Version."
The same definitive-absence predicate applies to Apple Keychain builds and
Android's separate key store; neither message attributes the absence to a
backup. Earlier restore-specific sentences above are superseded.

Outside the app, the privacy policy's three sentences on the permission
were approved on 21 October 2026. On 22 October 2026 each was placed in
the section it was drafted for, and a fourth sentence was approved after
the record's, "On Android this record is part of the app's settings,
which Android may include in your device backup; it holds no key and
none of your text." All four are in `docs/privacy.html`, from
`fu/1223-privacy-2`, merged on 22 October 2026 (`556984026`), to reach
readers with 1.2.23's publish of the site; a sentence for the review
notes (row 13) is drafted outside the repository. The policy's effective
date still reads
"Effective date: 11 October 2026", and the policy says a change is
posted with a new effective date; the Report sentence published on 21
October 2026 (`d03c3f36a`) kept that date too. The date chosen on 22
October 2026 is the day 1.2.23 goes live. Confirmed the same day: the
day meant is the day the website is published with 1.2.23, which the
step stage 11 of docs/RELEASING.md gains sets and which does not wait
for the stores;
`scripts/publish-site.sh` refuses a changed policy not dated the day of
the run (row 13). Not built: a sentence in the
study prompt naming the heading whose words were selected (row 12),
approved but not needed: since the follow-up to
decision 23 of 21 October 2026 no selection of a heading's words alone
reaches the study prompt, and since 22 October 2026 a drag across a
heading sends none of its words.

Choices built that wait for confirmation:

- Row 13 (decision 5): README.md's paragraph on where each platform keeps
  an AI key and on the permission's record, a fingerprint of the allowed
  key kept in the app's preferences on every platform (`478561c49`,
  `efd46978c`), is drafted to match the code and its wording waits for
  approval.
- Row 14 (decision 35): the Delete all notes question, sized to its
  content, is shorter on phones and tablets as well, which the natural
  height settled for a covering desktop sheet does not itself decide.
  Since `f4460eb9b` the AI answer covers the header to keep its waiting
  line whole, up to 413pt for Explain and Compare and up to 434pt for Ask
  with a one-line question, higher for a longer one (419 and 440pt with
  the header 3pt lower), and moves to clear the header when the answer
  arrives, or an error whose message and button fit below it. Where even
  the most a sheet may be is too short, the line can still be cut: for
  Ask with a one-line question on a 640x320 canvas under a 24pt status
  bar, and for all three on a 732x240 canvas. Showing it there means
  scrolling the head of Ask's question out of sight or setting the line
  above the question. The error state, whose two-line message was cut on
  393 to 436pt windows (at 1280x401 through the first line) and whose Try
  again was out of sight on 393 to 503pt windows with a passage of a few
  words quoted (399 to 442pt and 399 to 509pt with the header 3pt lower),
  is kept whole by the same rule on `fu/1223-ai-error-short`
  (`abba857dc`, `9e7eb94fd`, decision 35 as taken on 22 October 2026),
  seen on the Mac alone before the quote rule changed.
  Settled 23 October 2026: automatic quote expansion leaves room for the
  waiting or error state's minimum too, rather than only 96pt of body.
  The citation and disclosure stay visible, and an explicit open or
  closed choice survives failure, retry and resizing. No visual control
  dimensions change. The quote can then close on an 874x402 landscape
  screen to leave the error's whole column in sight; where even the
  whole safe area is too short, or a desktop question is too long for
  the window, the scroll and height cap remain. Showing the waiting or
  error line above a question too long for the window remains open.
  The new quote rule is integrated locally, uncommitted; combined checks
  and device verification remain.
- Row 16 (decision 11): the line names the key in force when the field
  went empty. A key typed into the field over the included one and
  deleted again is named as the reader's own, since the field saved it
  as theirs, though the key they had before was the included one; and a
  removal recorded by a release before 1.2.23, which wrote the same
  record for either key, is named as the reader's own, as it was before.
  Both kept as built until confirmed.
- Row 17 (decision 13): the ✕ has no accessibility label in the current
  toolkit. The 22 October 2026 Android accessibility tree exposed only
  the toolkit surface, and TalkBack was not exercised. The missing-key
  wording, signed-Mac rule, device-scoped dismissal and confined mobile
  tap target were settled on 23 October 2026 and are integrated locally,
  uncommitted; those decisions no longer await confirmation. Combined
  checks and runtime verification of the revised geometry remain.

Confirmed on 21 October 2026, as built: row 13's Settings model list and
Test key reaching the provider with the key, and none of the reader's
text, before any Allow (decision 5); and row 15's three rules (decision
7): the Mac's own share of a card gone at the tap says "Could not share
the card.", a case beyond those the decision lists, which closes
divergence 27; a compose the portal took and gave no answer to says
nothing; and on iOS a second share asked for while the first sheet is
still held back, up to about 3 seconds, is not made.

Confirmed on 22 October 2026, as built: row 16's two namings (decision
11), a key typed into the field over the included one and deleted again
named as the reader's own, since the field saved it as theirs, and a
removal recorded by a release before 1.2.23, which wrote the same record
for either key, named as the reader's own; and row 13's policy date
(decision 5), the day the website is published with 1.2.23, which stage
11 of docs/RELEASING.md sets and which does not wait for the stores.

Decided on 22 October 2026, row 17 (decision 13): the ✕ is small and
level with the line's first row. Its earlier branch used a 14pt glyph
and 24pt face with a target that enlarged the row and column. Superseded
on 23 October 2026: the glyph and face remain small and first-line
aligned, while the original button minimum sizes the row and column.
Only mobile gets an invisible target, bounded by its allocated column
and capped at 44pt on each axis; desktop has no extra proxy. The target
borrows no text padding and has no upward offset toward Clear. These
changes are integrated locally in `rel/1.2.23`, uncommitted, and await
combined checks and device verification.

Settled on 21 October 2026 and merged on 22 October 2026: row 7's three
rules beyond decision 18 (AND-4), the first and third confirmed, the
card of a note on the omitted verse alone hung above the verse after the
hole and no verse's narration lighting a mark, and the second rejected
and taken out, so the mark lights only where a link or a note names its
verse (`bf24fb6a3`); row 14's two points (decision 35), a covering
desktop sheet at its natural height, a card sized from its text as tall
as its content and a sheet of lists filling the room, and no separator
above the Settings footer on any platform, the iPad included
(`910d72f53`); row 16's line naming the key removed (decision 11), the
included key or the reader's own; and row 17's ✕ (decision 13), which
puts away the field's line for that restore, as decided, the picker's
line, which the decision did not name, staying as before (`dc312b8ee`).

Settled on 22 October 2026, row 12 (decision 23): a study prompt's
passage holds only Scripture, so a drag across a heading, from the end of
one verse through the heading or a psalm's title into the next, sends the
verses' words alone, the heading's left out as the share leaves them out
(built on `fu/1223-ai-no-headings`, `e86fc00be`, merged into
`rel/1.2.23` on 22 October 2026).

Settled on 22 October 2026 too, row 12 (decision 23): the rules built
with the follow-up of 21 October 2026 are confirmed as built. A drag
from a heading that ends inside the first word of the verse below, an
apostrophe or a hyphen inside that word counting as inside it
("Solomon’", "Well-"), or takes only the punctuation that verse opens
with, is offered no study verb, as Go quotes nothing for it; on Android
Study with AI is hidden for a selection inside a psalm's title or the
footnote section, where it was shown and did nothing; with Study with AI
left out, Share and Cross-references keep the order they have beside it,
not the order the assistant set to None gives them; and the verse-number
share fixes built with them stand: a verse number selected alone where
the verse holds the same figure, the first digits of a verse number, a
number begun in the space after the verse above, and a drag from a
heading into the first digits of the number below are each shared as the
citation alone. The two drags at the BSB's Ephesians 3:14 that the
panes' rule and Go's answer apart, from the heading through the ellipsis
and space that verse opens with into one or two letters of its first
word, stay measured and not held; the share quotes each of them with the
heading's words as Scripture under Ephesians 3:13–14, and Go's prompt
would quote it so (BACKLOG). And a selection whose only Scripture
is punctuation, a verse's closing full stop or any other mark of its
words selected alone, or a drag from a verse's closing punctuation into
the heading below it that ends in the heading, is offered no study verb,
the same rule as a selection that holds no Scripture, on every pane's
menu (the iPhone and iPad edit menu, the Mac context menu, Android's
selection toolbar, the Windows and Linux pane's menu), and the dispatch
refuses one as a backstop; a selection with any words of Scripture is
studied as before, a drag from the closing punctuation on into the verse
below among them. Built on its own branch, `fu/1223-no-study-punctuation`,
from `rel/1.2.23` at `91e1ec683` (`ca86cca13`); not yet seen on a device
(row 12).

Settled on 23 October 2026, row 12: punctuation followed by an unfinished
word across a heading offers no study. The shared end repair retains its
located punctuation remainder so the fallback cannot restore the partial
word; each native pane applies the same boundary rule when no letter or
figure of Scripture precedes the heading. This supersedes the partial-word
behavior and the two measured disagreements described above, including a
verse that opens with punctuation and a space. A complete word before or
after the heading remains studyable. Source changes and focused regressions
are integrated locally in `rel/1.2.23`, uncommitted; updated tests and
platform checks remain to run (BACKLOG, P4).

---

## 1.2.24

| # | Item | Behaviour and scope | Verification |
|---|---|---|---|
| 1 | Supporting AI Settings note | Restore the note under the AI key controls to muted caption text, as before 1.2.23, so it occupies less space and has the same weight as other settings guidance. | Settings and permission checks in both build states, plus native phone rendering in both appearances and orientations. |

## Later minor releases: 1.2.23 and after

The order:

- **1.2.23** had one of two large items to lead it (Decisions still open,
  3): the panes handing Go the selection's place in its verse, which went
  into 1.2.22 on 14 October 2026 (row 11 there), or the desktop's navigation
  rule (row 1), which was to lead 1.2.21 before that release went out as the
  privacy-declaration and Android 7.0 release.
- Rows 4, 16, 27, 28, 46 and 47 went into 1.2.23 on 15 October 2026
  (rows 1 to 6 there), merged into `rel/1.2.23`. Rows 31, 45, 15, 29, 39
  and 2 went into it on 21 October 2026 (rows 8, 9, 11, 12, 13 and 15
  there), with Next major version, row 3, Stays in the backlog, row 8
  (rows 10 and 14 there), and decision 18's mark (row 7 there). Rows 23
  and 10 went into it on 22 October 2026 (rows 16 and 17 there), and row
  11 the same day (row 18 there), once seen on the Android emulators.
- Rows 26 and 27 rode the selection's place in its verse, which 1.2.22 now
  carries. With a lead a verse number is removed only where its verse
  starts, so M (row 26) is left with the selections that have none; AI-4's
  native half (row 27) can return the chapter through the seams it widened.
  Rows 27 and 28 are bugs that need no decision of their own; on 14
  October 2026 both were kept out of 1.2.22 and planned for 1.2.23, each
  built on its own branch (Decisions still open, 2).
- Of the items the 7 October 2026 decisions moved to the release after
  1.2.20, rows 35 to 38 went into 1.2.22 on 14 October 2026 (rows 13, 16,
  17 and 20 there), with rows 30, 34, 41 and 43 and the double compose of
  row 2; row 40 ran in part on the Windows virtual machine on 15 October
  2026, and what is left is in its row. The same decisions
  deferred the selection's place in its verse with no release named, and the
  digit (row 25 until then) with it; both went into 1.2.22 on 14 October
  2026.
- Rows 3, 17, 18, 20 and 21 went into 1.2.22 on 14 October 2026 (rows 24
  to 26, 28 and 29 there). The small native fixes left (rows 19 and 22) can
  fill any minor release. The red-letter generators' fixes (rows 31 to
  33), made on 14 October 2026 and changing no table readers have, are in
  1.2.22's tree; the NKJV's regeneration closed on 15 October 2026, after
  a further fix to its generator's check for an end inside a word (row
  31), and the NKJV walk passed that day (row 45), both in 1.2.23.
- Rows 46 to 48 were found on 14 October 2026: two bugs in text the
  toolkit takes in and draws, from the Android 11 checks, and a release
  surface no test holds. None needs a decision. Rows 46 and 47 went into
  1.2.23 (rows 5 and 6 there); row 48 went in with 1.2.22's version bump,
  which rewrote the notes it guards (row 31 there).
- Row 49, the faster-model offer for ChatGPT that switched readers to a
  model that then read as unavailable, was found and fixed on 15 October
  2026 and went into 1.2.22 the same day (row 30 there).
- The rest follow as their decisions arrive.

Rows 26, 32 and 33 come from the same work as 1.2.22's rows. Each is a fix to what
readers already have, or to a build tool, so by the rule above each goes in a
minor release and none behind `next`; each row says why it is not in 1.2.22.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | The desktop takes the phones' navigation rule | A Mac, Windows or Linux window taller than it is wide shows Read, Books and Search on a bar along the bottom, as phones and tablets do, instead of a rail down its narrow side. | L / medium | Mac, Windows, Linux (retires divergence 29) | Approved 1 October 2026 for a minor release. Open: the dead zone (proposed: the rail from 10% wider than tall, the bar from 10% taller) and the settle of about 250 ms; and whether it ships unswitched as approved (recommended: a minor release's headline; which one is Decisions still open, 3) or is built behind `next` and graduates early, which NEXT.md does not yet describe. | The exception to "new goes in the major". A bar appears on tall desktop windows, and rebuilding the window mid-resize on the Mac's native reading view is new, so it needs a pass on the Mac and on both virtual machines. Tiled window managers are its main audience and one of the watch triggers of the closed first-frame entry, so centring is checked there in the same pass. |
| 5 | The Windows and Linux footnote section stays ragged (in "The reading page: what is left after 24 September 2026") | On Windows and Linux, the notes at the foot of a chapter are justified like the text above them. | M / medium | Windows, Linux | None | A parity improvement. Clicking within the notes has to follow the separately drawn words. The hyphenation half is the next major's (row 12 there). |
| 6 | Website and apps: the remaining layout differences (in "The reading page: what is left after 24 September 2026") | The website and the apps space two headings in a row alike and share a column width, and a paragraph that opens in prose and turns to poetry follows one rule for justification. | M / low | The website and every app | Open: one gap for two headings in a row (site 1.1em, iPhone 0.35em, Windows and Linux 1.45em); the column width; and whether such a paragraph stops being justified (and hyphenated) whole on the website and Android 15+, which the 24 September reading-page record makes their shared rule while the worklist's 7 September defects list counts the website's 990 such paragraphs as a defect. | One rule everywhere, and no data changes. The gap for a title followed by a heading is in the next major (row 7 there), because which gap applies depends on which comes first. |
| 7 | One way to turn a verse into drawn words, not two | None (internal): the card's lost spaces were fixed on 22 September; this removes the duplicate code that caused them. | M / medium | The Windows and Linux reading view; the verse-of-the-day card on all six | None. Unify the word model, carrying the card's letter-by-letter colour splitting, or two BSB verses change colour; and run the `spacingaudit` guard in CI, which never runs today. | Readers see no change, so it is not bug-fix material. Row 8 builds on it. |
| 8 | Fyne's RichText breaks a word mid-word at the start of a segment | Search results stop splitting a word in two ("Bein" / "g therefore") where a highlight starts. | L / medium | All six (search results) | Recorded: ship only if the search-card count falls to zero. To weigh: drawing search results with row 7's word model instead of patching the toolkit. | Visible but cosmetic, and real toolkit work. Fyne 2.8 rewrote RichText wrapping, so check there before patching 2.7.4. |
| 9 | The NKJV's missing spaces: report upstream, and decide on a correction list | About 286 places in 275 NKJV verses where two words run together ("Vanityof vanities", Ecclesiastes 12:8) read correctly, in the apps and on the website. | M / medium | All six and the website's NKJV pages | Open: send the report to API.Bible (drafted first, for approval); whether the app carries a correction list meanwhile, every entry checked against the printed edition, once the API.Bible and NKJV terms are confirmed to allow a corrected reading. | The report needs no release. The list, applied after the text loads, needs no re-download, and 286 checks against print. The WEB Catholic "oF" typo would join it. |
| 12 | Android: taps may land about 8 points above the finger (measure first) | If real, taps register slightly above the finger, so a small control such as the narration card's ✕ can be missed. | M / high if real | Android | None until measured | Seen once, through a synthetic tap, with no reader report. Measure with a real touch path first. If real, the fix is a toolkit patch that touches every tap, and the minor-or-major question is decided then. *Not in BACKLOG.md.* |
| 13 | What we ship has largely never been run | None (internal: confidence in what ships). | M / low | The Mac (every row "builds"), iPad, the Android universal APK, the Windows .zip and arm64, the installed Linux tarball and the AppImage | Open: what "tested" should mean for a store binary CI cannot run at all. A Linux hardware pass needs an x86_64 machine. | The cheap halves went out in 1.2.20 (row 15 (c) and (d) there); its Mac package verified both linked OS floors, and no run on macOS 12 or 13 has happened. |
| 14 | Recapture the App Store and Play screenshots: the Play phone set | People browsing the Play listing see light shots with dark, legible status-bar icons, and perhaps the phone shots at the display size most phones use. | M / low | The Google Play listing | Open, and discussed before any retake: retake the phone set at 420 (where Books shows one column by design), or keep 356 and retake only the light images; and whether the tablet's light images are retaken too. | What it waited for shipped in 1.2.18. The shots are taken from 1.2.21, the first release on Play with 1.2.20's features, after it clears Play's review: one Play edit at a time. Three reviewers and approval of the images come before the upload (SCREENSHOT_PLAYBOOK.md). |
| 19 | On iPhones whose screen scale is not the drawing scale, the reading pane is off by about 4% (in "iOS 27: after the scene life-cycle fix") | On the iPhone 12 and 13 mini, the Plus models and any iPhone with Display Zoom on, the reading pane's frame and clipping line up exactly. | M / medium | iPhone (those models and Display Zoom) | None | The recorded fix measures with the drawable's scale in `goAppReportSize` and `sendTouches`, a toolkit patch; `patches/fyne-2.7.4-ios-window-size.patch` does not cover it. Look in the simulator with Display Zoom first; touch mapping changes on those devices, so check on one before shipping. |
| 22 | The narration card's ✕: a double tap opens full screen (in "The open narration card covers the phone header's controls") | A double tap on the open card's ✕ only closes the card; today the second tap opens full screen in 45 of 48 phone layouts measured. | S / low | iPhone, Android phones | Open: a `fyne.DoubleTappable` ✕, which closes on either tap at the cost of the double-tap interval on every close, or a shield over the card's place for that interval, which leaves the controls it covered unreachable for that time. | Separable from the narration bar (Next major version, row 6), and need not wait for it. |
| 26 | The marker strip confirms a verse number by any overlap with that verse's opening (M) | Nothing in the shipped translations. A number that is part of a verse's text, followed by a word that opens the verse it names, would be stripped as that verse's number and the rest looked for elsewhere. | S–M, 2.5 days / low | All six | None | `stripVerseMarkersAnchored` (`share.go:1723`) accepts an "N " token when what follows overlaps verse N's opening by any length (an invented "The city had 12 gates" cited verse 3). No such digit token in the WEB, the BSB or the WEB Catholic; the NKJV not measured. With a lead, 1.2.22's row 11 reads the selection from the lead's place and removes a number only where its verse starts and the token is that verse's number (`walkSelectionFrom`; `TestAFigureInAVerseIsNotTakenForAVerseNumber`), so what is left is the text strip for a selection with no lead or whose walk fails. The recorded fix: the panes pass the number runs' positions and the strip removes exactly those. Latent, so not in 1.2.22. |
| 32 | BSB red letters: the generator takes the first copy from its cursor (RL-BSB-1) | Nothing: the regenerated table is the shipped one byte for byte. Before the fix, regenerated through its normalized path alone, the table would have moved Acts 9:10's red letters to the narration's copy of the words. | S, 1 day / low | None (a build tool) | None. The publisher's `bsb_usfm.zip` was downloaded for the proof on 14 October 2026 (Decisions still open, 43). | FIXED 14 October 2026 (`78b9ce81b`, `c9149e93a`). Each run is placed at its offset in its verse, counted in letters and digits on the USFM walk with heading lines left out, only where the runtime verse has the same letters before it, and both the exact match and the typography-normalized one are tried only there. Where the runtime verse differs before a run, or its letters are not at its offset, generation fails; with `--allow-relocated` the run is placed instead by the one copy of its letters in the verse, after the run before it, and listed. No copy or several, or an end inside a word, fails generation either way. `TestTheBSBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` fails on the generator before the fix in three verses, and each refusal in `TestTheBSBRedLetterGeneratorRefusesARunItCannotPlace` wrote a table before it (BACKLOG.md, RL-BSB-1, says which cases guard what). Regenerated from that download, before the fix and after it, the table is the shipped one byte for byte, every run at its offset (2,251 exact, 43 typography-normalized, none by its only copy). The archive differs from the one recorded on 23 August (its ACT.usfm and LUK.usfm are dated 2 October 2026); the table does not. `TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats the check where the sources are unpacked. Any minor release, as row 31. |
| 33 | WEB red letters: the same first-copy search (RL-WEB-1) | Nothing: both regenerated tables are the shipped ones byte for byte. Before the fix, an invented verse showed the shape, Peter's "I am he." reddened for Jesus's. | S, 0.75 day / low | None (a build tool) | None. eBible's USFM zips were downloaded for the proof on 14 October 2026 (Decisions still open, 43). | FIXED 14 October 2026 (`3a8bbe3e3`, `c9149e93a`), as row 32 with the punctuation-preserving match alone, a verse whose run is not placed falling to the pinned recoveries before it fails; a run of punctuation alone, a closing mark a note parts from its words (John 8:58), is placed between the letters on either side of its offset. `TestTheWEBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` fails on the generator before the fix in two verses, and each refusal in `TestTheWEBRedLetterGeneratorRefusesARunItCannotPlace` wrote a table before it (BACKLOG.md, RL-WEB-1). Regenerated from eBible's zips of 13 October 2026, which differ from the copies recorded on 23 August, before the fix and after it, both tables are the shipped ones byte for byte: the WEB's 2,053 verses at their offsets and 6 by the pinned recoveries, the WEB Catholic's 2,052 and 7, none by its only copy. `TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats it. Any minor release, as row 31. |
| 40 | Windows: the Store checks deferred from 1.2.20, and a look at its share fallback (in "Windows: use the native Share sheet"; "The Windows audio smoke is probably testing a silent sink"; "Microsoft Store: from the reserved name to the first submission") | Nothing if they pass. If not, a picture share from the Store install fails or saves somewhere unexpected. | S / low | Windows (the Store install and the direct download, on the Windows 11 arm64 virtual machine) | Decided 7 October 2026: moved to the release after 1.2.20. Which release now is open (Decisions still open, 2). | 1.2.20's row 11: the share from the Store install; the three Store checks (`WINDOWS_STORE_LISTING.md`, *Resuming with a Windows machine*, steps 3, 4 and 8); step 3 of the audio smoke; and a picture share whose card could not be read, through the fallback (1.2.20, row 1; in the entry on Share as image saying "Picture saved" for a card it could not read). Run in part on 15 October 2026 on the virtual machine, reached through its guest agent, with 1.2.21's arm64 package as submitted to the Store: its layout registered under a renamed identity, `bibletext.co.uk.BibleText.ShareCheck`, in development mode, as on 1 October 2026, and its executable, with ANGLE beside it, run from a folder without package identity, as the direct download runs; the release's zip itself was not run, and its executable is built by the same commands as the package's (`scripts/build-windows-exe.sh`, held to the release job's by `scripts/test-release-key-flow.sh`). The Store's own install there is 1.2.19, Store-signed, which carries the Share sheet; it was not launched. Passed: Share as image from the renamed package, the sheet showing the copy with its thumbnail and the copy in the user's own temp folder, as on 1 October; an https link to the site pasted into the Run box (step 4, pass (a)), which Windows offered to BibleText or Edge and which, given to BibleText, opened the app at John 3:16 with the whole link on its command line, while the same link clicked inside Edge stayed in Edge; that executable's listener, on 127.0.0.1 only, with no firewall prompt and no rule made, where a control listener on every address raised the prompt (step 8); its handoff, where a second launch with a link while the window was minimised kept one process and restored the window at the link's verse, and a cold launch opened at its verse (step 8); step 3 of the audio smoke, a test binary built in the guest with cgo, which reached the natural end and then idle with WASAPI in use, with no control run without an audio device; and, from that executable, a picture whose card was deleted between the preview and Share, which by 1.2.21's rule opened neither sheet, saved nothing and logged why, where the control with the card in place opened Windows' sheet. Run halfway: Edge's prompt for a `bibletext:` link on a local page (step 3), seen with its "Always allow" choice and cancelled; it named "Pick an app"; two packages claimed the scheme there, where a reader with the Store's install has one, and the prompt with one was not seen, so why it named no app is not established. Left: Share as image from the Store's own install, and where its copy lands; an https link clicked in Outlook or Teams, which had no signed-in account there, and pass (b), which needs the Store's package family name, the only one the consent file names, so whether Windows accepts the consent file is still open; why Windows' lookup of the link's handlers named the renamed package alone and not the Store's install, although Settings lists two BibleText entries for the site, both on, which is not established; Edge's prompt with one package claiming the scheme, what it names and whether Open starts the app, Chrome's and Firefox's, which are not installed there, and pass (b) after uninstalling; and the unreadable card with package identity, whose expected outcome row 2 changes from no sheet to a line. Outside the three entries, three passages are now out of date and are to be brought up to date: PLATFORM_MATRIX's *Not yet proven* bullet on Windows arm64, which names the runs of 30 September and 1 October 2026 only; its Sharing note that the Windows fallback for a card that could not be read has not been seen on Windows; and `WINDOWS_STORE_LISTING.md`'s *Resuming with a Windows machine*, still as of 16 September 2026, whose step 8 asks the firewall question answered here. The audio smoke's entry closes; the other two stay open for what is left. |
| 42 | 1.2.20's rows: the checks still to run on hardware (in "1.2.20's roadmap rows with no entry of their own"; "Android: an activity destroyed with the process alive stops the narration") | Nothing if they pass. | M / low | iPhone, iPad, Windows (Android's part passed on 14 October 2026) | None | Back on Android 11 and older (1.2.20, row 4): PASSED 14 October 2026 on an Android 11 (API 30) emulator image. With the WEB's John 3 narrated, Back finished and destroyed the root activity in a process that lived on; 5 and 35 seconds later the media session was playing, the player started, audio focus held and the service in the foreground, and the app reopened about 75 seconds later on the narration still playing, its read-along following. A swipe from Recents, the control, ended all four. Ask (1.2.20, row 18): reading, dictation and VoiceOver on a physical device. The shared panel (row 19): a native Windows run, and the physical hit region of the 44-point control under the toolkit's eight-point touch compensation. iOS rendering during app switching and font memory (rows 20 and 21): the cause of the physical exits seen before them, which the simulator runs do not establish. |
| 44 | The release procedure does not say why Play's publish-on-approval is checked by hand (in "The release pipeline: what still waits on something it need not") | None (internal). | S / low | Google Play release procedure | None | Apple's release type is set by `appstore/submit-version.py` and read back before a submission; Play's, Managed publishing off, is read in the Publishing overview and recorded (RELEASING.md stage 6; PLAY_LISTING.md, *Publishing mode*), with no reason given for its being a hand check. The only asymmetry between the Apple and Play legs of the release procedure still open at `6bf27aee4`. |

---

## Next major version

**Switch or branch.** Every row here but one is built on `main` behind the
`next` switch, as [NEXT.md](NEXT.md) describes. **Row 13, oto 3.5, keeps a
branch**, because it changes a module version and the Go version every build
uses. No row needs a higher minimum OS version. Rows 1 and 2 are already on
`main` behind the switch; NEXT.md's register is their record, with their
tests and open decisions. Row 3 was too, and left the switch for 1.2.23 (row 10 there).

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Switch or branch | Decision | Notes |
|---|---|---|---|---|---|---|---|
| 1 | "Same saying, another occasion" among the Gospel parallels (NEXT.md, piece 3) | Selecting a saying of Jesus that he repeated on another occasion shows a new group of links among the Gospel parallels: 102 groups, 274 pairs. | M / medium | All six | Switch: on `main` | Open: NEXT.md P1–P6. Recommended: hold R01 back, place the 22 verses as new sets with the existing sets unchanged, keep the "Cf." pairings out. | It shares `crossrefs.go` with row 3, so the panel's deepest read is measured again once both settle. |
| 2 | The NKJV refreshed at cache epoch 8, with an offline bridge (NEXT.md, piece 1) | With epoch 8 dropped (recommended), nothing changes for readers now. The bridge waits for the next NKJV decoder change: a reader offline when that update is due then keeps the previous copy instead of dropping to the WEB. | M / medium | All six | Switch: on `main` | **Open recommendation:** drop the epoch-8 refresh and keep the offline bridge for the next decoder change. Also NEXT.md N1, the picker's sentence. | Epoch 8 was to reach existing copies faster than the 30-day refresh. 1.2.19 shipped the decoder fixes on 3 October, so copies refreshed under 1.2.19 have them within 30 days, by about 2 November for readers who have updated. At the major, a forced refetch costs about 197 metered requests and about 13 MB per NKJV reader on the shared key, and helps only readers who skipped every minor release in between. Without the epoch this row changes no stored data; with it, a device moving between `next` and release builds crosses epochs, and each crossing costs a refetch. |
| 4 | Cross-references panel: the smaller presentation points from the 2 October audit | Tidier lists: no row repeats a parallel or points back into the selection, parallels are capped for long selections, half-verses are not doubled, and no selected verse is left empty by the 40-row cut. Previews keep red letters, italics and poem lines. | M / low | All six | Switch | Approved to bundle later. Open: which of PAR-7/C-9, PAR-8, C-7, C-8, C-11 and C-6 to take (recommended for C-6: reuse the verse-of-the-day paragraph). | The major changes this panel already (row 1), so one pass here avoids doing it twice. The NKJV panel's second pass folds in if licensing says yes. |
| 5 | Headings: quoting, notes and links need one deliberate pass | Selecting a section heading, a parallel-passage line, a Psalm 119 letter or a major section head follows a deliberate rule for each kind, instead of today's one rule. | L / high | All six and the website | Switch | Taken 15 October 2026 (Decisions still open, 23): what selecting a heading means, A, the section it introduces, cited by the verse below with nothing quoted, the rule already applied to a heading selected whole on its own; with it the native panes look forward for a selection wholly inside a heading. Open: a rule for each kind of heading; whether the heading-only rule is revisited. | Behaviour readers will notice. A note anchored by a heading must still land in the same place, so the stored note format does not change. After 1.2.20 rows 6 and 7. The audit that keeps every heading out of sharing in one place could come earlier, in a minor. The Apple panes' heading fact (Later minor releases, row 29) was built for 1.2.23 once the first question was answered, on all four panes that draw a heading in their selectable text. |
| 6 | The open narration card covers the phone header's controls | On phones, narration gets its own full-width bar on narrow layouts, instead of a card over the next-chapter arrow and the full-screen button. | L / medium | iPhone, Android phones (and narrow desktop reading views); tablets and wide windows keep the card | Switch | Open: where the bar sits (under the header, above the tab bar, or over the foot of the text), when it shows, how it is dismissed, whether it carries the source chip. The own-row layout was rejected on 26 September. | A redesign readers will see; the pop-up is the chosen presentation until then. A design pass with screenshots. It builds on the unmerged `fu/narration-card` (9c744ea25, b23e5283f), which is kept. The double tap on the ✕ is Later minor releases, row 22. |
| 7 | A psalm's title drawn above the publisher's section heading (SCRIPTURE_WORKLIST.md, S10) | In psalms with both (Psalm 3 in the BSB and NKJV, for example) the section heading comes first, as the publisher sends it, and the gap between a title and a heading is the same on the website and in the apps. | L / medium | All six and the website | Switch | Open: keep the title first (as now; the item then closes as by design) or follow the publisher's order; then one gap for a title followed by a heading (site 1.1em, apps 0.55em). | It changes the layout of every such psalm, and the store shot 08 would need retaking. |
| 8 | In-text footnote markers (SCRIPTURE_WORKLIST.md, S22) | A small mark in the text at each translators' note leads to the note, as in a printed Bible. | L / high | All six and the website | Switch | Open: whether to show markers at all, and whether to tie them to the footnotes setting, as the omitted-verse marks are. | It touches selection on every reading view. Its first step, Android's scan of the end of the text, is internal and can land in any minor. |
| 9 | A reader cannot tell why a verse is lit | A verse lit because the reader arrived from search, a link or Go to looks or reads differently from one lit by a note. | M / medium | All six and the website | Switch | Open: a tint for each cause, a small label, or clearing a search mark sooner; and with it graphite or the current violet for dark mode, and wiring the multi-note tint. Three palette approvals move with it. | A visible change to highlights everywhere. |
| 10 | Verse of the day: possible later additions — one opt-in daily notification | Readers can choose one notification a day with the day's verse, on iPhone first, then Android. | L / medium | iPhone, iPad, Android. **Divergence:** Mac, Windows and Linux. | Switch, with one part that is not | Open: build it on the desktops too, or record the divergence; how Android's manifest gets its entries; the optional `/today` page, the sparkle in phone landscape, a first-open dot. | On Android, scheduling needs an alarm receiver and `RECEIVE_BOOT_COMPLETED`, both in the static `cmd/mobile/AndroidManifest.xml`, which the Go tag does not reach (`POST_NOTIFICATIONS` is already declared). So either `scripts/build-android.sh` writes a tag-aware manifest, or inert entries land first in a minor. An inexact daily alarm needs no special permission; exact alarms would add a Play permissions declaration. iOS needs a new bridge. |
| 11 | More share-card typefaces with true small capitals | Verse pictures gain the reading face (Junicode), and perhaps EB Garamond. Most verses' default picture changes typeface. | S (M with EB Garamond) / low | All six | Switch | Recorded: add Junicode; EB Garamond only after a read of its licence's reserved-name terms. Open: accept that a reader sharing the same verse again sees a different look. | It changes every reader's default picture, so it goes with the new features. |
| 12 | The Windows and Linux pane does not hyphenate (in "The reading page: what is left after 24 September 2026") | On Windows and Linux the justified page hyphenates, so its lines sit tighter, as on the website and Android. | M–L / medium | Windows, Linux | Switch | Open: accept a hyphenation dictionary's size and licence, as a Go module or as data. Copy and selection keep words whole. | A Go module used only by `next` files still enters `go.mod` and `go.sum` for every build, though release binaries do not link it; the release guards (`verify-not-next.sh`) still hold. Shipping the patterns as data avoids the module. |
| 13 | oto 3.5 on Linux: pure-Go PulseAudio (in "Linux stores: from the listing source to two live channels") | Nothing if it works. Linux narration goes through PulseAudio, and the snap loses its ALSA plumbing and the fallback question inside its confinement. | M / medium | Linux (the Go version change reaches every build) | **Branch** | Open: whether to do it at all, and whether it travels with the Fyne toolkit work. | oto 3.5 needs Go 1.25; `go.mod` says `go 1.24.0` with `oto/v3 v3.4.0`, and every workflow pins `go-version: "1.24"`. No build tag chooses a module version, so it cannot sit behind the switch. It would drop `libasound2-dev`. Go 1.25 needs macOS 12, already the app's minimum. Paired with the toolkit work if that goes ahead, for one toolchain pass; otherwise alone, after a Linux audio pass on the virtual machine. |

**Outside the backlog:** the Fyne 2.8 upgrade, or the move to the
`cubancorona/fyne` fork, also keeps a branch, because it changes the toolkit
every build compiles (NEXT.md, *What needs a branch instead*; FYNE_28_PORT.md;
FYNE_FORK_POLICY.md). The 2.8 port is not to be adopted yet: the shared
shaper's data race is open. The fork switch waits for a go-ahead.

---

## Stays in the backlog

Each row says what would move it.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | What would move it |
|---|---|---|---|---|---|---|
| 1 | The release pipeline: what still waits on something it need not | None (internal: shorter release days). | M / low | Release tooling for every channel | Recorded: Play from CI needs the upload key and the service account in the repository's secrets, which is the account holder's decision; skipping the local mirror of CI before a push costs a red public run when something slips. Any change to the documented order needs an OK. | A standing entry, so it never closes. One candidate per release, dry-run on a day that is not a release day and recorded in RELEASING.md: retry the arm64 snap build automatically; have `submit-version.py` call `push-screenshots.py`; have `publish-site.sh` check every `releases/latest` link the page carries exists (from the closed ARM entry); a conductor for the Apple leg; parallel store builds in worktrees; Play from CI. The Play promotion item is done. |
| 2 | The known differences kept: the Apple prose-into-poetry paragraph, the web's Notes label, browsers without container queries (in "The reading page: what is left after 24 September 2026") | Nothing changes. | S / low | The Apple apps; the website | None | Deliberate or watch-only. An importer route that keeps a second alignment, or reports of trouble on old browsers. |
| 3 | Android sets a Psalm title's word gaps narrower than its space (in "The reading page: what is left after 24 September 2026") | On Android 15 and later, the gaps in a psalm's italic title are about 1.5 px narrower than elsewhere. | S / low | Android | Recorded option: a `ScaleXSpan` of 278/250 over each gap, which `readerText` would have to drop | Close to invisible, and a span per gap. A rework of the titles for another reason. The titles' hyphenation before Android 15, from Android 6, is recorded as divergence 31 with 1.2.20's row 8. |
| 4 | NKJV cross references: the panel's second pass | NKJV readers see the edition's own 32,473 cross references as cards. | M / medium | All six (NKJV) | Recorded gate: nothing displays in a store build until API.Bible's licensing reply says it may | Blocked on that reply (enquiry sent 10 September). On a yes it becomes a next-major feature, done with that panel pass (Next major version, row 4). Its own `nkjvxrefs` tag stays separate from `next`. |
| 5 | The direct downloads do not register `bibletext:` links | None today: no page on the site emits such a link. | M / medium | The Windows .zip, the Linux AppImage, the Mac direct .zip | Open: register silently on first run, by a Settings switch, or not at all | Nothing has needed it since the NKJV text went live on the site on 3 October, and the Mac part conflicts with LINKS.md (Universal Links only). A page that emits the scheme again. |
| 6 | Age ratings: aim for all ages in every market | Store listings show an all-ages rating everywhere, instead of USK 12, ESRB Everyone 10+ and 18+ in Russia. | S / low | Google Play, Microsoft Store | Open: what IARC intends by its questions for a work of literature rather than a game ("Violence, Blood, or Gory Images" is the hinge) | Console work, not a build. The first gap after 1.2.21 clears both stores, retaking both questionnaires in one sitting with row 7. |
| 7 | Target audience is 13+, and it was meant to include 9-12 | Google Play can list the app for children aged 9–12; who may install it does not change. | S / medium | The Google Play Console | Open: the COPPA/GDPR certification is the account holder's alone to sign. Worth weighing: a Families reviewer will look at the AI feature. | A decision to sign, after the recovery breadcrumb (Later minor releases, row 10). |
| 9 | The tests tagged `!race` run only in CI's second pass, never under the detector | None (internal). | M / low | CI | None | Out of date: the tests run in CI's second pass, and Android compile checks exist. Retitled in 1.2.20 (row 16). Making them pass under the race detector is low value. |
| 10 | One pill, several noted paragraphs: several open bubbles at once | A chapter with several notes would show several open bubbles at once. | S / medium | iPhone, iPad, Mac, Android | Recorded: a choice rather than a defect, weighed against a page carrying three open bubbles | Options 1 and 2 are done. A wish for it moves it to the major (one constant). |
| 11 | Candidate: a neutral graphite wash for the dark-mode highlight | In dark mode a highlighted verse is grey rather than violet: easier to read, quieter to spot. | S / medium | All six and the website | Recorded: three approvals move with it | A candidate, not a fault. Decided with Next major version, row 9. |
| 12 | Deferred-UI timers under the test driver | None (tests only). | S / low | The suite | Recorded rule: a gate lands only with a demonstrated race | Watch-only by its own rule. A test that first starts one of the four remaining timers. |
| 13 | A setting to turn section headings off (SCRIPTURE_WORKLIST.md, S15) | Readers could hide the publishers' section headings. | M / medium | All six | Open: whether, and its default; headings have been on for everyone since they shipped | No reader has asked, and it adds a setting on six platforms. A reader request, or a decision for a plain-text mode. |
| 14 | BSB Zechariah 12:1: the oracle title stays inside the verse (SCRIPTURE_WORKLIST.md) | None today. | S / low | All six | Recorded: worth revisiting deliberately, not as a side effect | The publisher puts it in the verse. Only a deliberate editorial decision. |
| 15 | WEB Catholic Mark 9:47 reads "the Gehenna oF fire" (SCRIPTURE_WORKLIST.md, display issues) | A capital-F typo in one verse, in the apps and on the website. | S / low | All six (WEB Catholic) | Open: report it upstream; whether a correction list is allowed | Blocked on the publisher. It joins the NKJV correction list (Later minor releases, row 9) if that is approved. |
| 16 | Linux stores: the Snap Store listing's categories and screenshots, and the AppImageHub PR (in "Linux stores: from the listing source to two live channels") | People browsing the Snap Store see the app's category and four screenshots; AppImage users can find it on AppImageHub. | S / low | The Snap Store, AppImageHub | Console and PR work for the account holder. Setting categories and screenshots in the dashboard is safe; hand-editing the summary, description or icon there turns off update-metadata-on-release for good (LINUX_STORES.md). | No build. The four screenshots are in `docs/screenshots/linux/`. Whenever the dashboard is open. |
| 17 | Snap preferences under the per-revision data folder (in the same entry) | A snap reader who reverts to an older revision gets back older notes. | S–M / medium | Linux (the snap) | Open: move the preferences to `$SNAP_USER_COMMON` (`XDG_CONFIG_HOME=$SNAP_USER_COMMON/.config`) with a one-time copy, or leave them | Stored data: it changes where notes live, so it cannot go behind the switch, and if chosen it ships in a minor with the copy and a revert test. It bites only on a manual `snap revert`. A report of notes lost after a revert, or the next snap packaging change. |
| 18 | Windows: the first window's position (in "The first frame is laid out for a window the app did not get") | On Windows the first window is placed by the system's cascade, not centred in the work area. | S / low | Windows | None | Fyne's `doCenterOnScreen` centres against the video mode rather than the work area, and the app never calls it. Cosmetic, with no reader report. The next Fyne change to the window path, or one of the first-frame entry's watch triggers. |
| 19 | Version states: two open questions (VERSION_STATES.md, *Open decisions*; in "Bible version states: transition diagram + comprehensive tests") | Nothing changes until decided. | S / low | All six | Open: (1) whether tapping the translation shown while a substitution is in force accepts it; (2) whether a launch that cannot open the remembered translation prefers a public-domain one whose canon holds the saved place (the WEB Catholic in Tobit) | Each is safe as it stands, and moves to a minor once decided. The third question is Later minor releases, row 23. |
| 20 | Android: after a switch from gesture to three-button navigation, the new bar keeps white buttons until restart (in "Android: the status-bar icons were white on the light page") | Rarely, after a navigation-mode change, the three-button bar's buttons stay white on a light page until the app restarts. | S / low | Android | None | Watch only: it looks like SystemUI's new bar missing the appearance. A reader report, or a reproduction on a second Android release. |
| 21 | iPadOS window buttons drawn over the header title (in "iOS 27: after the scene life-cycle fix") | In a small iPadOS 26 or 27 window, the window controls cover the start of "BibleText". | S / low | iPad | None | Cosmetic. The fix feeds a left inset from UIKit's corner-adapted layout region into a toolkit patch. The next change to the iPad window patch. |
| 22 | An iPad window moved to an external display keeps the iPad's scale (in the same entry) | In Stage Manager, the window moved to an external screen keeps the iPad's scale and size. | S / low | iPad (M-series, Stage Manager) | None | No reader report; `view.window.screen` would be the source. With Later minor releases, row 19, which touches the same scale code. |
| 23 | Android never ends a cancelled touch (in the same entry) | A touch the system takes over is never ended, so a drag it began is not ended. | S / low | Android | None | It cannot panic, and no reader effect is known. A touch fault found by the tap measurement (Later minor releases, row 12). |
| 24 | iOS: if all three HTML imports of a rebuild fail, the fallback keeps the old palette's colour (in "A light/dark change with a sheet open left the app half in each theme") | In theory, text in the old colour on the new paper after a light/dark switch, until the next push. | S / low | iPhone, iPad (and the Mac twin) | None | Theoretical: only single failures are known. Setting `textColor` explicitly on the fallback closes it. Any report of a failed import, or the fallback's geometry (Later minor releases, row 28), which can set the colour as it builds the page; row 28, fixed for 1.2.23, does, on both panes, so this closes with that release. |
| 25 | GNOME on Wayland may open Files behind the window (in "Linux and Windows: an in-app share sheet in place of the 1.4-second notice") | A shared picture's folder may open behind BibleText, with a "ready" notice, while the sheet says the picture is shown. | S / low | Linux (GNOME on Wayland) | None | Not seen: the portal is handed no activation token or parent window. A reader report, or the share work of Later minor releases, row 2. |
| 26 | iOS ships `CFBundleExecutable: main` (in "FIXED: the executable was named after `cmd/desktop`, not after the app") | None: it shows only in crash reports and Xcode's Organizer. | S / low | iPhone, iPad | None | Low priority by its own entry. The next change to `release-ios.sh`'s packaging. |

---

## Decisions still open

The seven decisions 1.2.20 needed were taken on 6 October 2026 and went out
with it; each is in its BACKLOG.md entry. On 7 October 2026 the items then
in Later minor releases, rows 25, 35 to 38 and 40, were moved to the release
after 1.2.20, and row 24 was deferred with no release named. 1.2.21 went out
without those six items, and what is left of them is decision 2. On 13
October 2026 1.2.22's set was approved, with the cross-references panel
taking the share card's order (1.2.22, row 6), and Later minor releases, row
24, was started on its own branch. On 14 October 2026 the ten fixes were
merged for the release, which took decision 1, and row 24 joined 1.2.22 as
its row 11, one change on all four panes, with row 25, which took decision 4
and left decision 3 one candidate. Later that day the deferred items were
triaged in two passes. The bugs among Later rows 26 to 44 that need no
decision or wording were fixed for 1.2.22 (rows 13 to 21 there, beside row
12, the defect the device checks found), but for AI-4 and P3, the two larger
ones; and of the bugs in Later rows 1 to 23 and Stays, those a check here
could show were fixed too (rows 24 to 29 there), with the reason each of the
others stayed given under that table. The same day decision 2 was taken for
AI-4 and P3: 1.2.22 ships without them, and both go to 1.2.23. On 15
October 2026 the ChatGPT faster-model fix, Later row 49 until then, joined
1.2.22 as its row 30, 1.2.22's release notes were approved, and the guard
on the GitHub release notes, Later row 48, went in with its version bump as
its row 31. What is still open, by when it is needed. A recommendation is
marked where one is made. Rows are cited by section: *1.2.22*, *Later* for
later minor releases, *Next major* and *Stays*.

### Before the 1.2.22 work starts

1. **The legacy Entry pane** (1.2.22, row 10): taken 14 October 2026; it
   takes the cursor's span, as planned. Deleting the dead pane stays possible
   in any later release.
2. **The items moved on 7 October 2026 to the release after 1.2.20, and two
   larger bugs**: rows 35 to 38 of Later went into 1.2.22 on 14 October 2026
   (rows 13, 16, 17 and 20 there), as the digit, row 25 until then, had with
   row 11. AI-4 and P3 (Later, rows 27 and 28): taken 14 October 2026;
   1.2.22 ships without them, and both go to 1.2.23, each built on its own
   branch (`fu/ai4-chapter`, `fu/p3-fallback-geometry`). They are bugs that
   need no decision of their own and were left out of 1.2.22's fixes for
   their size and risk: about 2.5 days each, AI-4 changing every pane's seam
   and P3 both Apple panes' Objective-C. Still open: row 40. The Windows
   virtual machine was reachable on 15 October 2026 and its checks ran in
   part; which release carries what is left, listed in its row, is open.

### Before the work after 1.2.22 starts

3. **What leads 1.2.23**: the selection's place in its verse went into
   1.2.22 on 14 October 2026 (row 11 there), so what is left is whether the
   desktop's navigation rule (Later, row 1) leads it.
4. **The selection's place in its verse** (1.2.22, row 11): taken 14 October
   2026; one change on all four panes, as the parity rule asks, in 1.2.22.
5. **Permission before text goes to a third-party AI** (Later, row 39):
   taken 15 October 2026: one question for each provider before its first
   request, on all six, in the wording the BACKLOG entry gives, withdrawn by
   choosing None or clearing that provider's key; the Settings note becomes
   body text. Built on `fu/1223-ai-consent` and merged into `rel/1.2.23` on
   21 October 2026 (1.2.23, row 13). Settled on 21 October 2026: the
   question's body names a search, "the study action, and any question or
   search you type"; the permission ends with its key, so any change of
   that provider's key, cleared, replaced by another or missing after an
   Android restore that brings the preferences back without it, ends it,
   and the app asks again before the next request; "Nothing is sent until
   you allow it." is approved as the study panel's and Find's message after
   Not now; Settings' model list and Test key may reach the provider with
   the key before any Allow, sending none of the reader's text; and the
   privacy policy's three sentences on the permission are approved. The
   key rule and the new body were built on `fu/1223-consent-2`
   (`7ef80e58b`) and merged on 22 October 2026. Decided on 22 October
   2026 for the policy: each of the three sentences goes in the section
   it was drafted for; a fourth, approved, says that on Android the record
   is part of the app's settings, which Android may include in a device
   backup, and holds no key and none of the reader's text; and the
   effective date is the day 1.2.23 goes live. Built on
   `fu/1223-privacy-2` (`612b32a84`, `7f77c510a`, `17abf2caa`,
   `59f59be42`), merged on 22 October 2026, with the date set on the day
   of the site's publish (docs/RELEASING.md, stage 11), which does not
   wait for the stores. Confirmed on 22 October 2026: the effective date
   is the day the website is published with 1.2.23 (1.2.23, row 13).
6. **The desktop's navigation rule** (Later, row 1): the dead zone and the
   settle; whether it ships unswitched in a minor release as approved
   (recommended) or is built behind `next` and graduates early.
7. **Shares that cannot finish** (Later, row 2): taken 15 October 2026,
   Draft A: "Could not open your email program." under the buttons when
   Email… fails; "Could not share the card." reused for the picture
   failures and a card that cannot be drawn; iOS's refusal detected, and
   said in the existing words when it persists. Built for 1.2.23 on
   `fu/1223-share-fail-line` and merged on 21 October 2026 (1.2.23, row
   15). The three rules the build adds were confirmed on 21 October 2026:
   the Mac's own share of a card gone at the tap says "Could not share the
   card."; a compose the portal took and gave no answer to says nothing;
   and on iOS a second share asked for while the first sheet is still held
   back is not made.
8. **WEB Catholic Daniel 3** (Later, row 15): settled by the source on 15
   October 2026, not a choice among the options. The publisher's own USFM
   (eBible's WEB Catholic, `66-DAGeng-web-c.usfm`) marks "THE SONG OF THE
   THREE HOLY CHILDREN" as a section heading carrying a footnote, reference
   3:24; the helloao feed runs the footnote's text into the heading's words,
   so the 510-character bold heading was a decoding defect. It is fixed to
   match the source on `fu/1223-heading-footnote`, merged into `rel/1.2.23`
   on 21 October 2026 (1.2.23, row 11): the title is the heading,
   and the note is that heading's footnote, listed with the chapter's other
   footnotes and shown whenever they are. It is set apart when the page is
   drawn, so nobody downloads again.
9. **The Greek Esther** (Next major, row 3): taken 15 October 2026, as
   recommended: it leaves the switch in 1.2.23; E-A, keep the gap marks at
   Esther 4:6, 9:5 and 9:30; E-B, the line "Nothing is listed for …, which the
   cross-references don't cover." for the Song of the Three, Susanna, Bel and
   the NKJV's extra verses too; E-C, the two sentences as drafted. Built on
   `fu/1223-greek-esther` and merged on 21 October 2026 (1.2.23, row 10).
   The line for a deuterocanonical book, which the piece brings to
   shipping builds, "The cross-references don't cover the deuterocanonical
   books.", was approved on 21 October 2026.
10. **The narration card's ✕ double tap** (Later, row 22): a double-tappable
    ✕, or a shield for the double-tap interval.
11. **A key cleared on purpose** (Later, row 23): taken 15 October 2026. The
    launch stops trying the NKJV, and the picker says "You removed your
    API.Bible key, so World English Bible is shown in place of New King
    James Version." Built for 1.2.23 on `fu/1223-key-gone` and merged on 22
    October 2026 (1.2.23, row 16). Decided on 21 October 2026: the line
    names the key removed, for the key included with the app "You removed
    the included API.Bible key, so World English Bible is shown in place of
    New King James Version.", and for the reader's own the words above.
    Built on `fu/1223-key-gone-2` (`a206ffa18`) and merged on 22 October
    2026 (1.2.23, row 16). Confirmed on 22 October 2026, as built: a key
    typed into the field over the included one and deleted again is named
    as the reader's own, since the field saved it as theirs, and a removal
    recorded by a release before 1.2.23, which wrote the same record for
    either key, is named as the reader's own (1.2.23, row 16).
12. **The NKJV's run-together words** (Later, row 9): send the report upstream
    (drafted for approval first); whether the app carries a correction list,
    checked entry by entry against print, once the terms are confirmed to
    allow it.
13. **A missing saved key** (Later, row 10; 1.2.23, row 17): settled
    23 October 2026. Under each empty AI or API.Bible field whose saved
    key is definitively absent, say "Your saved key isn't available on
    this device. Paste it again here." Android and signed Apple builds
    use the same condition: a saved mark, a separate credential store's
    item-not-found answer and no remaining Preferences copy. Store errors
    do not qualify, and the app does not infer a restore or cause.
    Dismissal remains per account on the device. Apple's ThisDeviceOnly
    record can survive a same-device backup restore but does not migrate
    to another device; Android's `no_backup/` record is lost after
    reinstall or reset. The ✕ keeps its 14pt glyph, 24pt face and
    first-row alignment, with the original button minimum sizing its
    row and column. A mobile-only target fits inside that column, capped
    at 44pt per axis, without an upward shift or borrowed text padding;
    desktop has no extra proxy. Integrated locally in `rel/1.2.23`,
    uncommitted. Mac parity closes PLATFORM_MATRIX.md divergence 38.
    The picker's existing not-restored sentence remains unchanged while
    replacement wording is pending.
14. **Android drag and Translate** (Later, row 11; 1.2.23, row 18): taken
    15 October 2026;
    the dragged and translated text is cleaned as Copy is, and the drag
    stays, as recommended. Built on `fu/1223-android-drag-clean`; the iPad's
    and the Mac's pages carry neither character, so they keep their native
    drag unchanged. Android's reading toolbar offered no Translate at all,
    because the platform offers other apps' text actions only on a view
    with an id and the reading view had none. Taken 21 October 2026: add
    Translate, the system's text-processing actions (`ACTION_PROCESS_TEXT`)
    reachable from the reading toolbar and handed the text as Copy cleans
    it, read-only. Built in `61cb2b9c5`: the reading view has an id and the
    manifest a `<queries>` entry for `ACTION_PROCESS_TEXT`, and the
    toolbar's items and the matching accessibility actions send the text as
    Copy gives it, marked read-only, with nothing an app returns read. The
    iPhone and the iPad offer the system's Translate; the Mac's menu leaves
    it out, as do Windows and Linux (PLATFORM_MATRIX.md, divergence 37).
    Seen on the API 35 and API 30 emulators on 22 October 2026, an item
    another app offers handed Copy's text, and merged into `rel/1.2.23`
    that day (1.2.23, row 18). Still open: whether a text an app returns for
    text that cannot be edited is shown, as the platform shows it in a
    toast, which needs Fyne's activity to hand the bridge the results it
    now ignores; and whether the Mac, and the Windows and Linux pane, offer
    Translate as well, or divergence 37 stands: the Mac's menu has left the
    system's Translate out since 26 June 2026 (`menuForEvent:`,
    `reading_macos.go`).
15. **Website and app layout** (Later, row 6): one gap for two headings in a
    row; the column width; whether a paragraph that opens in prose and turns
    to poetry stops being justified whole on the website and Android 15+.
16. **Play phone screenshots** (Later, row 14): retake at 420, or keep 356 and
    retake only the light images; and whether the tablet's light images are
    retaken.
17. **Store builds CI cannot run** (Later, row 13): what "tested" should mean
    for them.
18. **An omitted verse's mark under a mark that covers it** (AND-4, apart
    from its fix, which is 1.2.22's row 15): taken 15 October 2026, as
    recommended. The "[21]" mark lights when a link or a note names the
    verse it stands for, and a note shown from its link alone on a range
    that starts at such a verse is drawn above the verse after the hole on
    every surface, which closes divergence 34 in PLATFORM_MATRIX.md. Built
    for 1.2.23 (row 7 there). Three rules in the build went beyond the
    decision (BACKLOG, AND-4). On 21 October 2026 two were confirmed,
    AND-4's first and third: a note on the omitted verse alone hangs above
    the verse after the hole too, and no verse's narration lights a mark.
    The other, AND-4's second, the mark lit with the verse after the hole,
    was rejected; the mark now lights only when a link or a note names the
    verse it stands for, so a link to the WEB Catholic Edition's Sirach
    26:28 lights verse 28 alone and the nine marks "[19]" to "[27]" before
    it stay bare (`2aa12b083`). A note from another edition, which the
    chapter's plan files on the verses this edition has, lights the marks
    of the holes its own span names, as a link to that span does
    (`4ce5575e4`). Both are on `fu/1223-gap-marks-named`, merged on 22
    October 2026 (1.2.23, row 7).

### For the next major version

19. **"Same saying, another occasion"** (Next major, row 1): NEXT.md P1–P6 —
    the label, R01, the 22 verses as new sets, the "Cf." pairings, the
    credit, the hiding rules.
20. **The NKJV refresh** (Next major, row 2): **drop the epoch-8 refresh and
    keep the offline bridge for the next decoder change (recommended)**, or
    keep epoch 8; and NEXT.md N1, the picker's sentence.
21. **The cross-references panel** (Next major, row 4): which presentation
    points to take.
22. **The narration bar on phones** (Next major, row 6): where it sits, when
    it shows, how it is dismissed, whether it carries the source chip.
23. **Headings** (Next major, row 5): what selecting a heading means was
    taken on 15 October 2026: A, the section the heading introduces, cited
    by the verse below with nothing quoted, the rule already applied to a
    heading selected whole on its own. It was built for 1.2.23 as Later,
    row 29 (1.2.23, row 12, since 21 October 2026), and with it the native panes look forward for a selection
    wholly inside a heading. Its follow-up was taken on 21 October 2026: a
    study verb, Explain, Compare, Ask and every other, is not offered for
    a selection that holds no Scripture, a heading's words alone, a psalm
    title alone or a verse number alone, on any pane, and the dispatch
    refuses one as a backstop; a share of such a selection keeps A. A
    selection that takes Scripture, a drag from a heading into its verse
    among them, is studied as before. It replaces the study prompt that
    read the cited verses (`99dbcd728`), and the sentence approved for the
    prompt naming the heading is not needed. Built on
    `fu/1223-no-scripture-study` and merged on 22 October 2026 (1.2.23,
    row 12). Taken on 22 October 2026: a study prompt's passage holds
    only Scripture, so a selection run across a heading or a psalm's
    title sends the verses' words alone, as the share quotes them. Built
    on `fu/1223-ai-no-headings` and merged on 22 October 2026 (1.2.23,
    row 12). Settled on 22 October 2026: the rules built with the
    follow-up are confirmed as built (no study verb for a drag from a
    heading that ends inside the first word below or takes only the
    punctuation it opens with; on Android, Study with AI hidden in a
    psalm's title and the footnote section; Share and Cross-references
    keeping their order; the verse-number share fixes); and a selection
    whose only Scripture is punctuation, a verse's closing full stop or
    any other mark of its words alone, or a drag from a verse's closing
    punctuation into the heading below it that ends in the heading, is
    offered no study verb, the rule for a selection with no Scripture, on
    every pane, and the dispatch refuses one. Built on
    `fu/1223-no-study-punctuation` (1.2.23, row 12). Open for the major:
    a rule for each kind of heading (a parallel-passage line, a Psalm 119
    letter, a major section head), and whether the heading-only rule is
    revisited.
24. **Psalm title or section heading first** (Next major, row 7), and then
    the gap between them.
25. **Footnote markers** (Next major, row 8): whether to show them, and
    whether to tie them to the footnotes setting.
26. **Highlights** (Next major, row 9): how a search mark differs from a
    note's, graphite or violet in dark mode, and the multi-note tint.
27. **The daily verse notification** (Next major, row 10): on the desktops
    too, or a recorded divergence; for Android's manifest, a tag-aware
    manifest or inert entries shipped first in a minor; the optional
    `/today` page, sparkle and dot.
28. **Share-card typefaces** (Next major, row 11): Junicode, and EB Garamond
    after a licence read; accepting the change to default pictures.
29. **Windows and Linux hyphenation** (Next major, row 12): a dictionary's
    size and licence, as a Go module or as data.
30. **Toolkit and toolchain** (Next major, row 13, and *Outside the
    backlog*): whether the major carries the Fyne 2.8 port or the fork
    switch, and whether oto 3.5 (Go 1.25) goes with it. These are the only
    pieces that keep a branch.

### Whenever suits (they stay in the backlog until then)

31. **The release pipeline** (Stays, row 1): the Play upload key and service
    account in the repository's secrets (Play from CI); whether to skip the
    local mirror of CI before a push.
32. **`bibletext:` links in the direct downloads** (Stays, row 5): register
    silently, by a Settings switch, or not at all.
33. **Age ratings** (Stays, row 6): how to answer IARC's questions for a work
    of literature.
34. **The 9–12 audience** (Stays, row 7): signing the COPPA/GDPR
    certification.
35. **Short desktop windows** (Stays, row 8): taken 15 October 2026; a sheet
    that cannot clear the header covers it, with no new minimum window
    height; and the iPad Settings footer's separator, mirroring the rule
    under the title. Built for 1.2.23 on its own branch,
    `fu/1223-short-windows` (`b1daedf48`, `330f935ed`), merged on 21 October
    2026 (1.2.23, row 14), the separator on all
    six because the sheet is one component everywhere. Settled 21 October
    2026: a covering sheet takes its natural height, its preferred height
    capped at the room from the header's top, so a card sized by its text
    is only as tall as its content and Go to's lists still fill the room;
    and the separator on no platform, the iPad included. Built on
    `fu/1223-short-windows-2` (`0c2e48e40`, `a4598d375`, `9c3da08a4`,
    `5bc64cbb8`) and merged on 22 October 2026; seen on the Mac on 21
    October 2026, and the footer on an iPhone and Android, not yet on an
    iPad or the Linux and Windows virtual machines. The AI answer's waiting
    line, cut by its scroll where the panel cleared the header, is kept
    whole since `fu/1223-ai-answer-short` was merged on 22 October 2026,
    the panel covering the header instead up to 413pt, and for Ask up to
    434pt with a one-line question and higher with a longer one, never
    shorter than the line's least and filling the room, which its column
    is taller than wherever it covers; open there, how to show the line
    where even the most a sheet may be is too short. Taken 22 October
    2026: the error state is kept whole, its message readable and its
    buttons in sight, by the same rule, covering the header where it must,
    on every desktop platform, and a canvas too small even for that may
    still cut it, kept as it is and recorded. Built on
    `fu/1223-ai-error-short` (`abba857dc`, `9e7eb94fd`),
    and seen on the Mac alone before the quote rule changed. Settled
    23 October 2026: the quote opens automatically only where it leaves
    the waiting or error state's minimum body whole as well as the usual
    few answer rows. The reader can still open it, and explicit open and
    closed choices survive failure, retry and window resizing. The new
    quote rule is integrated locally, uncommitted; combined checks and
    device verification remain. A whole safe area
    too short for the column, or a question too long for the window, can
    still require scrolling; changing the question's order or initial
    scroll position remains open.
36. **Several open note bubbles at once** (Stays, row 10): yes or no.
37. **A setting to hide section headings** (Stays, row 13): yes or no, and its
    default.
38. **BSB Zechariah 12:1** (Stays, row 14): move the oracle title out of the
    verse, or keep it.
39. **Version states** (Stays, row 19): whether a tap on the checked
    translation accepts a substitution; whether a launch falls back to a
    translation whose canon holds the saved place.
40. **The Snap Store and AppImageHub** (Stays, row 16): categories and
    screenshots in the Snap dashboard, and the AppImageHub PR.
41. **Snap preferences** (Stays, row 17): move them to `$SNAP_USER_COMMON`
    with a one-time copy, or leave them.
42. **The repository's GitHub branches:** the merged branch
    `publisher-paragraphs-only` is still on GitHub; deleting it is a push,
    so it waits for an OK.
43. **The NKJV red-letter regeneration** (Later, row 31) **and the NKJV
    walk** (Later, row 45): taken 15 October 2026, and both closed that day.
    The NKJV was opened twice that day, each time through the app's own
    fetch (197 requests) and the generator's own chapters (174), kept
    outside the repository and deleted after the checks; only counts and
    verse references were recorded. The other half,
    downloading the eBible USFM zips and the publisher's `bsb_usfm.zip`, was
    approved on 14 October 2026, and the sources were downloaded that day:
    the BSB's and the WEB's generators were fixed then (`78b9ce81b`,
    `3a8bbe3e3`, and the lead-in check of all three, `c9149e93a`), and
    before the fix and after it they write the shipped tables from those
    sources byte for byte (rows 32 and 33). The NKJV's generator was fixed
    with them (`d98440704`). On the NKJV its placement wrote the shipped
    table byte for byte, but its check for an end inside a word refused 18
    verses at words the source runs together; that check was fixed the same
    day (`b7f5cb9a7`), and the run after the fix, on the NKJV opened the
    second time, writes the shipped table byte for byte (row 31, closed).
    The walk passes: 143,627 passages, none differing (row 45, closed).
