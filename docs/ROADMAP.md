# Roadmap

The plan as of **14 October 2026**: which release each open item in
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

## 1.2.22, the next minor release

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
and P3, below. Row 15 had a regression, put right in the same row, and the
repairs of rows 13 and 17 two older defects (rows 22 and 23). The second
pass took the bugs in Later rows 1 to 23 and in Stays in the backlog that
the first had left out, and fixed those a test here can tell apart and a
device or simulator here can show (rows 24 to 29, with an older Mac defect
that row 26's arrival had hidden, row 27); what it left, and why, is after
the table. Each row taken from Later minor releases has left that table.
FIXED means the change is in with a test that fails on the tree before it.
Nothing here changes stored data, makes a reader download a translation
again or adds wording. Rows 15 and 16 change the website too, whose half
reaches readers with the next `scripts/publish-site.sh`; their pinned
digests moved with `reader.js` in both states of the switch.

| # | Item (where it was) | What a reader notices | Platforms | Outcome |
|---|---|---|---|---|
| 12 | Android keeps a link's arrival point as the reading position (found by row 2's device check) | On Android, after following a link or closing Settings, a reader who scrolls on and then turns the phone or reopens the app keeps their place, instead of going back to where the link landed (a relaunch asked for John 3:13 landed at 11). | Android | FIXED (`b45fc17c0`, `d473150aa`). The phone chapter header asks for more than a phone is wide, so the mobile driver's paint laid the reading host out wider than the canvas and then at its real width, and both frames reached the bridge: a reflow was armed that nothing applied, the scroll listener dropped every scroll until the next chapter, and a placement pending then was made at the wide width. Go now hands the phone panes only a frame that fits the canvas (`nativeHostFrame`; `TestAReadingHostWiderThanItsCanvasIsNotHandedToThePane`). The bridge re-places an arrival or a restore when the text lays out at another width straight after it, carries it into a reflow armed meanwhile, and drops a reflow whose width comes back before the text laid out (`TestAPlacementMadeAtOneWidthIsKeptWhenTheTextLaysOutAtAnother`, `TestAReflowArmedByAFrameThatComesBackIsDropped`). On the emulator with each case forced, the build before kept John 3:14 in 1 of 3 runs and dropped 62 scrolls in each of 3; the fix kept it in 3 of 3 and dropped none, and four rotations kept the top verse. The defect reached 1.2.21. |
| 13 | A drag from a heading into its verse (Later, row 36: the heading repair) | A drag from inside a publisher's heading through the next verse's number into its first words shares that verse's words, cited to it, instead of the heading as Scripture or the verse cut short ("The LORD Is My Shepherd The LORD", Genesis 4's "And" for "And Cain"). | All six (the BSB: 15,081 of 189,996 such drags under 682 headings; the WEB Catholic: 7) | FIXED (`7d304b96f`). The heading repair takes the front off first, then only a heading below that verse may lose its front from the end; a piece confirmed by the verse beneath comes off before the search; a stack of title and headings is one piece; a heading holding its own verse's number is confirmed after it. `TestADragFromAHeadingIntoItsVerseSharesTheVersesWords`, `TestAHeadingPieceRunOnToAVerseWithNoWordsQuotesNothing` and `TestTheHeadingRepairKeepsTheOtherDrags`; over the three local caches every such drag now shares its verse's words (`TestEveryDragFromAHeadingIntoItsVerseOverLocalCaches_LocalOnly`). The lead does not reach it: the selection begins inside the heading. |
| 14 | Share with note's popover arrow (Later, row 41) | On the iPad and the Mac, Share with note's popover points at the selected words, below the new "Note from you" card, instead of into the card above them. | iPad, Mac | FIXED (`0e0b7ddb2`). The share carries the words' place in the pane's text, and both Apple panes present the sheet beside them after the re-import that puts the card on the page; without a re-import within a second, at the selection as before. `TestShareWithNoteIsAnchoredToTheSelectedWords`. Seen with development builds: the Mac's picker 0.8 pt below the words, the iPad simulator's source rectangle on them, against 88 and 89 pt above them before. The distance before is the new card's height, which follows the note's length: the 7 October 2026 check, with a shorter note, measured 52 pt. |
| 15 | A link to a verse the edition omits (Later, row 30: AND-4) | A link to a verse the open edition leaves out (Matthew 17:21 in the BSB) opens at the verse after the hole, where its "[21]" mark is, instead of at the top of the chapter with nothing lit. | All six and the website | FIXED (`24e9bb0ec`). Go resolves the arrival verse once (`drawnArrivalVerse`) for every surface; Android pushes it, the canvas pane falls back to the arrival verse's line, and the web reader lands an unlit link on the next verse. `TestALinkToAnOmittedVerseArrivesAtTheVerseAfterTheHole`, `TestTheCanvasPaneLandsALinkToAnOmittedVerseOnTheVerseAfterIt` and `TestTheWebPageLandsALinkToAnOmittedVerseOnTheVerseAfterIt`. A note on the omitted verse alone, whose card every surface parks at the chapter top, was then scrolled out of view on every app surface, while the website still landed on it; it keeps the arrival it had before, none, so the chapter opens at its top with the card in view (`3d8240697`, `arrivalVerseOnPage`; `TestANoteOnAnOmittedVerseKeepsItsCardInView`, `TestTheCanvasPaneOpensANoteOnAnOmittedVerseAtItsCard`). PLATFORM_MATRIX.md, divergence 34, records where a card shown from its link alone stands. Whether the mark itself lights stays open (decision 18). Seen on 14 October 2026 on the emulator (Pixel 7, API 35), the reader on the WEB, which omits Acts 8:37, with the NKJV switched off: `/nkjv/` links to 8:37, 8:37–38 and 8:38 land on verse 38; a note link on 8:37 opens at the chapter top with its card in view, where the build before the exception landed on 38 with the card out of view; a note link on 8:37–38 lands on its card above verse 38. |
| 16 | Four rules for a shared note's link (Later, row 35) | The app and the website read every shared note link alike: white space inside a payload is damage on both, a payload's ends lose ASCII white space only, the website cleans a note's text as the app does, and a note the app opens also opens on the website. | All six and the website | FIXED (`67f90c576`). 10 of 16 probe payloads decoded differently in the app and on the website; the four rules now hold in both decoders. `TestTheAppAndTheWebReaderReadNoteLinksAlike` runs the app, the shipped decoder under node and the Go mirror over the payloads a corpus line cannot hold, and `testdata/note_vectors.txt` gains append-only cleaning and empty-table vectors. |
| 17 | A phrase ending on a dash (Later, row 37: the dash end repair) | A phrase that ends on a closing dash keeps its last word ("defiled herself—" stops being shared as "defiled"; a drag ending on "If the gate is shut—" keeps "shut"). | All six (the BSB) | FIXED (`251e04633`). The end repair no longer cuts after an em dash. `TestAPhraseEndingOnADashKeepsItsWords`; the local phrase sweep's known count for the BSB falls from 4 to none. A drag cut inside a word at such a dash is row 22. |
| 18 | A Linux text share opens two mail composes (Later, row 2, in part) | Email… on Linux opens one compose, where `xdg-email` launched the mail program and then reported the launch failed and the `mailto:` link opened a second. | Linux | FIXED (`f4f09bc04`). `xdg-email`'s exit 4 now ends the share; exits 1, 2, 3 and 5, which refuse before launching anything, still hand on. `TestXDGEmailEndsTheComposeOnlyWhereItMayHaveOpenedOne`. Seen on 14 October 2026 on the Linux virtual machine (Ubuntu 24.04, GNOME on X11, xdg-utils 1.1.3) with stand-in mail handlers in place of a client: one named thunderbird, which `xdg-email` runs itself, composes and exits 1, and `xdg-email` exits 4 after its compose, where the `mailto:` link through `xdg-open` then opened a second; through `gio`, the path for any other client, `xdg-email` exits 0 once the handler is launched, whatever it returns, and with the default handler's program missing it exited 0 too, `gio` opening another. Exit 4 with nothing launched was not seen there. The trade-off: should another desktop's launcher fail with nothing opened and exit 4, the share now ends with no compose and no word, where the `mailto:` link was tried; that silence is the case row 2's approved line answers. The rest of row 2 waits on its wording (decision 7). |
| 19 | The versification generator moves a missing verse to a repeat (Later, row 34: VERS-1) | Nothing today: no shipped edition has the shape, and the tables are unchanged. | None until a regeneration | FIXED (`276929af1`). A move is refused when the candidate slot matches a verse the edition kept at least as well; the dropped verse is absent and the slot extra. `TestTheVersificationGeneratorDoesNotMoveAVerseToARepeat` runs the generator on invented editions of both shapes, with Romans' doxology as the control; `TestTheVersificationGeneratorReproducesTheShippedTables_LocalOnly` holds its output on the local caches to the BSB's and the WEB Catholic's rows of both shipped tables byte for byte. The NKJV's rows were not regenerated. Seeding moves from the SIL mapping, in the recorded fix, was not needed for the shape found. |
| 20 | The NKJV's card route and selection route in the opt-in walk (Later, row 38) | Nothing (internal). | The suite | FIXED (`4af99b741`): the test's stand-in, not the app. It handed the selection route the publisher's "Lord" where every pane hands over the drawn "Lᴏʀᴅ"; given the NKJV's shape, the WEB's Genesis, Exodus, Psalms and Isaiah differ in 11,884 of 29,441 passages with the old stand-in and in none with the drawn text. A fixture with the divine name in small capitals joins `TestPassageShareEqualsSelectionRoute`. That the NKJV's 46,837 come from the stand-in is shown on the WEB given the NKJV's marking: the walk has not been run on an NKJV cache, and that run is Later minor releases, row 45. |
| 21 | Two comments describing the code before 1.2.20 (Later, row 43) | Nothing (internal). | The code | FIXED (`cdd6a2a3a`): `notes_sticker_native_on.go` and `bibleTextDismissMenu` (`reading_ios.go`) say what the code does. |
| 22 | A drag cut inside a word at a closed-up dash (the end repair of row 17) | A drag that stops inside the word after a closed-up dash keeps the word before it ("was morning—t", Genesis 1:5, stops being shared as "was"), and one begun inside the word before such a dash keeps the word after it ("g—the first" stops being shared as "first"). | All six (the BSB; the WEB and the WEB Catholic where they close a dash up) | FIXED (`6fdd95736`). Older than row 17, and with the span and the lead alike: at every closed-up dash between words of two letters or more in the three local caches, each shape lost its word at the dash in all of the BSB's 2,052 drags, the WEB's 428 and the WEB Catholic's 476. The end repair cuts back to the later of the last space and the last such dash, the start repair forward to the earlier of the first space and the first, and the dash comes off as a clause mark does. `TestAPhraseEndingOnADashKeepsItsWords`; `TestADragCutInsideAWordAtADashKeepsTheWholeWordOverLocalCaches_LocalOnly` finds none wrong; every other local sweep passes as before. |
| 23 | A drag across a stack of headings (the heading repair of row 13) | A drag from the end of a verse across two headings over the next verse into its first words shares the words on either side, cited to both verses, instead of quoting both headings as Scripture (the BSB's Proverbs 22:16–17 across "Thirty Sayings of the Wise" and "Saying 1"). | All six, on the pages that send no lead: the Apple plain-text fallback and the legacy pane | FIXED (`6437ef777`). Each heading was removed whole only where the number of the verse beneath stood straight after it, which the upper heading of a stack never has; the repair now takes the stack off whole. `TestADragAcrossAStackOfHeadingsSharesTheVersesOnEitherSide`; over the three caches, all 24 of the BSB's 22,728 drags across the headings between two verses that cross a stack (over Proverbs 22:17 and Hebrews 11:30), with the span and without, were wrong and none is now (`TestEveryDragAcrossTheHeadingsBetweenTwoVersesOverLocalCaches_LocalOnly`). |
| 24 | Two verse pictures saved in the same second overwrite each other (Later, row 3) | Two pictures shared within one second are both kept, the second named " (2)" as browsers name a second download. | Linux; the Windows fallback and the Windows Share sheet's copy | FIXED (`ca540fbad`, `freeShareImagePath`). `TestTwoPicturesSharedInOneSecondAreBothKept`. |
| 25 | The tab bar's dress is chosen when the window is built (Later, row 17) | An iPad or Android tablet window resized across 560 points while taller than wide rebuilds in the other dress at once. | iPad, Android tablets | FIXED (`ecf49e80c`). `renderedLayout` carries the dress where a bar is drawn, read from the canvas as the bar and the rail read it, so the watcher's two readings cannot disagree; the rail and the full-screen tree carry none. `TestATallWindowResizedAcrossTheBarsWidthRebuildsTheBar`. Not seen on a device: the iPad simulator in its windowing mode refuses an app's own request to change its window. A sheet open at the rebuild is taken and reopened, as for any layout change. |
| 26 | A reader who scrolled away from a link's arrival is taken back by the next width change (Later, row 18) | After scrolling on from a link's verse or note, a window resize or a rotation that rebuilds nothing leaves the reader where they are. | iPhone, iPad, Mac | FIXED (`e16733c5f`). The reader's scroll ends the arrival as it ends the restore (`btIOSEndArrival`; `btMacUserScrolled`). `TestAReadersScrollEndsTheArrival`. Seen on 14 October 2026 with development builds before and after, each arriving at Psalm 119:100 (WEB) and then scrolled 1,500 points on as the reader, to verse 121: on the iPad Pro 13-inch (M5) simulator, in full-screen reading, a 250-point narrowing of the pane through its frame path put the build before back at verse 100 and left the fix at 121; on the Mac, a window narrowed from 1280 to 1000 points put the build before back at verse 100 and, with row 27, left the fix at 121. |
| 27 | A Mac window resize moves a reader who scrolled (hidden by row 26's arrival) | On the Mac, a window resized narrower or wider keeps the verse at the top of the view. | Mac | FIXED (`332b4d752`). Setting the pane's frame wrapped the text at the old insets first, AppKit moved the clip with that passing wrap, and the insets put right after left the clip there: a reader at Psalm 119:73 was at verse 112 after a window narrowed from 1280 to 1000 points, and one at verse 121 at the end of the psalm. `btMacApplyFrame` reads the verse at the top and how far into it before a change of width and puts it back after; a restore or an arrival still places over it. `TestTheMacFrameChangeKeepsTheReadersPlace`. Seen with the development builds: verse 73 and verse 121 kept through narrowing and widening. The iPad's frame path kept the reader's place before the change and was not touched. |
| 28 | Mac narration that follows with the reading view hidden puts each verse at the very top (Later, row 20) | On the Mac, narration that follows the text while Books or Search is open, or under a sheet, keeps the verse in the comfortable band. | Mac | FIXED (`79089f7df`). Follow runs while the pane is hidden, and the follow-scroll read where the view stands from `visibleRect`, which AppKit gives a hidden view as zero; it reads the clip view now, as the capture does. `TestTheMacFollowScrollReadsTheClipViewNotVisibleRect`. Seen with the development builds, narration following Psalm 119:60 with the reader on Books: the build before put the verse 0 points below the top of the view, the fix 191 points down a 636-point view, the band's 30%, where it still stood back on Read. |
| 29 | Android's night-mode patch reads its flag before it is written (Later, row 21) | Switching between light and dark while the app is in front always takes effect at once. | Android | FIXED (`a0e5975b8`). Stock Fyne's activity sends the configuration to the driver before it writes the flag the driver reads; the patch writes the flag first, so the send orders the two. `TestAndroidNightModePatchWritesTheFlagBeforeTheConfigurationIsSent`. `8c5677310` read the night bit of the configuration the driver builds instead, which the asset manager's configuration does not carry: built with it, the app stayed light through `scripts/test-android-dark-mode.sh`. With `a0e5975b8` the app follows both ways in that check, and 20 switches 1.5 s apart were each followed, as by the build before; the race was not seen on the emulator in either. |

What the triage found and left, and why. The first pass:

- **Study verbs that lose their chapter** (Later, row 27, AI-4) and **the
  Apple plain-text fallback page** (Later, row 28, P3) are bugs that need no
  decision of their own and were left all the same, for their size and
  risk: each is about 2.5 days, AI-4 changes every pane's seam and P3 both
  Apple panes' Objective-C, which was more than the rest of the set
  together. Whether either joins 1.2.22 is decision 2. AI-4's Go half,
  about a day, would fix Windows and Linux alone, with a recorded
  divergence.
- **The marker strip with no lead** (Later, row 26, M) now happens only on
  the fallback page or where the lead fails its check, as it does for a
  drag begun inside a heading, and goes with row 28.
- **The red-letter generators** (Later, rows 31 to 33) can be fixed only
  with the regeneration that proves a byte-identical table, which needs the
  publishers' USFM downloaded and the NKJV opened once (decision 43). All
  three were fixed on 14 October 2026, outside 1.2.22: the BSB's and the
  WEB's tables regenerate byte for byte from the publishers' USFM, downloaded
  that day, and the NKJV's regeneration is still to run (Later, row 31).
- **An emoji in Share with note on Android**, which `btaShareText` passes
  through `NewStringUTF`, where CheckJNI may refuse four-byte UTF-8, is not
  a bug where it can be checked: on the API 35 emulator with CheckJNI on, a
  message holding two emoji crossed it whole, with no JNI error. API 24 to
  30 images are not installed.
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
  palette's ink pushed to both Apple panes; it goes with row 28 (decision
  2).
- **Stays 26**, iOS shipping `CFBundleExecutable: main`: renaming the
  executable changes the store package and `release-ios.sh`, which only a
  store build and upload can prove, and this work makes neither.
- Seen on the emulator with row 15, older, and not changed: Android parks a
  card whose verse the page lacks over the chapter's first line, where the
  iPhone and iPad reserve an inset for it (BACKLOG.md, AND-4).

### Beside 1.2.22: the account holder's console and policy work

Three corrections that need no build, each the account holder's to make; the
detail is in "Google Play's Data safety answers and the privacy policy: three
corrections" (BACKLOG.md).

| # | Item | What a reader notices | Where | Notes |
|---|---|---|---|---|
| 1 | Play's Data safety form gives Analytics as the purpose for Other user-generated content | The Play listing's data-safety section gives App functionality as the purpose, as the privacy policy, which says the developer runs no analytics, does. | The Play Console's Data safety form | Change the purpose from Analytics to App functionality. Google requires the form and the policy to agree. One Play edit at a time. |
| 2 | Play's Data safety form shows that a reader can ask for the data to be deleted | The listing stops offering a deletion request: the policy sends a reader to the provider for that, the developer being unable to view or delete what a provider holds. | The same form | Set the deletion request mechanism to No. Both data types stay collected, optional, not processed ephemerally and encrypted in transit. |
| 3 | The privacy policy does not mention the AI panel's Report email | The policy says that Report sends the support address an email holding the passage, the question and the answer, beside its sentence that the developer does not receive AI requests. | `docs/privacy.html` | One sentence, its wording to be approved. The Report button (`reportBtn`, `ai_panel.go`) opens the pre-filled email. Published with `scripts/publish-site.sh`, so it can go with rows 8 and 9 above. |

---

## Later minor releases: 1.2.23 and after

The order:

- **1.2.23** had one of two large items to lead it (Decisions still open,
  3): the panes handing Go the selection's place in its verse, which went
  into 1.2.22 on 14 October 2026 (row 11 there), or the desktop's navigation
  rule (row 1), which was to lead 1.2.21 before that release went out as the
  privacy-declaration and Android 7.0 release.
- Rows 26 and 27 rode the selection's place in its verse, which 1.2.22 now
  carries. With a lead a verse number is removed only where its verse
  starts, so M (row 26) is left with the selections that have none; AI-4's
  native half (row 27) can return the chapter through the seams it widened.
  Rows 27 and 28 are bugs that need no decision; whether either joins
  1.2.22 is Decisions still open, 2.
- Of the items the 7 October 2026 decisions moved to the release after
  1.2.20, rows 35 to 38 went into 1.2.22 on 14 October 2026 (rows 13, 16,
  17 and 20 there), with rows 30, 34, 41 and 43 and the double compose of
  row 2; row 40 waits for the Windows virtual machine. The same decisions
  deferred the selection's place in its verse with no release named, and the
  digit (row 25 until then) with it; both went into 1.2.22 on 14 October
  2026.
- Also once their decisions are taken: shares that cannot finish (row 2),
  the WEB Catholic Daniel 3 heading (row 15), permission before text goes to
  a third-party AI (row 39), and the Greek Esther if it is decided to leave
  the switch (Next major version, row 3).
- Rows 3, 17, 18, 20 and 21 went into 1.2.22 on 14 October 2026 (rows 24
  to 26, 28 and 29 there). The small native fixes left (rows 19 and 22) can
  fill any minor release, and so can the red-letter generators' fixes (rows
  31 to 33), made on 14 October 2026 and changing no table readers have,
  with the NKJV's regeneration still to run, and the NKJV walk (row 45).
- The rest follow as their decisions arrive.

Rows 26 to 33 come from the same work as 1.2.22's rows. Each is a fix to what
readers already have, or to a build tool, so by the rule above each goes in a
minor release and none behind `next`; each row says why it is not in 1.2.22.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | The desktop takes the phones' navigation rule | A Mac, Windows or Linux window taller than it is wide shows Read, Books and Search on a bar along the bottom, as phones and tablets do, instead of a rail down its narrow side. | L / medium | Mac, Windows, Linux (retires divergence 29) | Approved 1 October 2026 for a minor release. Open: the dead zone (proposed: the rail from 10% wider than tall, the bar from 10% taller) and the settle of about 250 ms; and whether it ships unswitched as approved (recommended: a minor release's headline; which one is Decisions still open, 3) or is built behind `next` and graduates early, which NEXT.md does not yet describe. | The exception to "new goes in the major". A bar appears on tall desktop windows, and rebuilding the window mid-resize on the Mac's native reading view is new, so it needs a pass on the Mac and on both virtual machines. Tiled window managers are its main audience and one of the watch triggers of the closed first-frame entry, so centring is checked there in the same pass. |
| 2 | Shares that cannot finish say so, in one approved line | When a share cannot finish, the app says so in one short line instead of ending without a word: a picture never saved, a picture no folder took, a card that cannot be drawn, a mail program that fails behind Email…. | M / medium | Linux, the Windows fallback sheet and the macOS mimic for the save and Email… cases; all six for a card that cannot be drawn; iOS for a refused presentation | Recorded: a line under the buttons when the compose fails, its wording to be approved; stop at the first route that started a compose. The stop, which needed no wording, went into 1.2.22 on 14 October 2026 (row 18 there), reading `xdg-email`'s exit 4 as such a start; the trade-off that reading carries is written there. Open: one heading and line for every desktop case; whether "Could not share the card." (the phones' approved words) is reused where it fits; whether to detect iOS refusing to show the sheet. | Gathers "Email… on the desktop share sheet fails without a word", the open tails of "Android: a text share has no failure path" and "Linux and Windows: an in-app share sheet in place of the 1.4-second notice", and the line 1.2.20's row 1 leaves out. The 20 September rule, that a share the reader started does not end in silence, and the desktop's no-sheet rule pull opposite ways here; one change with one approved line settles both on every platform. The double compose that 1.2.22 stops was seen on the Linux virtual machine with a stand-in mail client (1.2.22, row 18). A drawing failure cannot really happen with the fonts the app bundles. |
| 4 | An iPhone launched sideways draws the bottom bar first (in "One navigation rule on phones and tablets") | An iPhone opened while held sideways shows the bottom bar for about 0.4 s before the rail or the full-screen page. | S / medium | iPhone (an Android phone must still never flash a rail) | The technical route only: answer the first layout from the orientation the scene reports at launch, or hold it until the canvas has a size | Minor, at launch only, and it touches the first-frame layout. |
| 5 | The Windows and Linux footnote section stays ragged (in "The reading page: what is left after 24 September 2026") | On Windows and Linux, the notes at the foot of a chapter are justified like the text above them. | M / medium | Windows, Linux | None | A parity improvement. Clicking within the notes has to follow the separately drawn words. The hyphenation half is the next major's (row 12 there). |
| 6 | Website and apps: the remaining layout differences (in "The reading page: what is left after 24 September 2026") | The website and the apps space two headings in a row alike and share a column width, and a paragraph that opens in prose and turns to poetry follows one rule for justification. | M / low | The website and every app | Open: one gap for two headings in a row (site 1.1em, iPhone 0.35em, Windows and Linux 1.45em); the column width; and whether such a paragraph stops being justified (and hyphenated) whole on the website and Android 15+, which the 24 September reading-page record makes their shared rule while the worklist's 7 September defects list counts the website's 990 such paragraphs as a defect. | One rule everywhere, and no data changes. The gap for a title followed by a heading is in the next major (row 7 there), because which gap applies depends on which comes first. |
| 7 | One way to turn a verse into drawn words, not two | None (internal): the card's lost spaces were fixed on 22 September; this removes the duplicate code that caused them. | M / medium | The Windows and Linux reading view; the verse-of-the-day card on all six | None. Unify the word model, carrying the card's letter-by-letter colour splitting, or two BSB verses change colour; and run the `spacingaudit` guard in CI, which never runs today. | Readers see no change, so it is not bug-fix material. Row 8 builds on it. |
| 8 | Fyne's RichText breaks a word mid-word at the start of a segment | Search results stop splitting a word in two ("Bein" / "g therefore") where a highlight starts. | L / medium | All six (search results) | Recorded: ship only if the search-card count falls to zero. To weigh: drawing search results with row 7's word model instead of patching the toolkit. | Visible but cosmetic, and real toolkit work. Fyne 2.8 rewrote RichText wrapping, so check there before patching 2.7.4. |
| 9 | The NKJV's missing spaces: report upstream, and decide on a correction list | About 286 places in 275 NKJV verses where two words run together ("Vanityof vanities", Ecclesiastes 12:8) read correctly, in the apps and on the website. | M / medium | All six and the website's NKJV pages | Open: send the report to API.Bible (drafted first, for approval); whether the app carries a correction list meanwhile, every entry checked against the printed edition, once the API.Bible and NKJV terms are confirmed to allow a corrected reading. | The report needs no release. The list, applied after the text loads, needs no re-download, and 286 checks against print. The WEB Catholic "oF" typo would join it. |
| 10 | The recovery breadcrumb for a key that did not survive a device move (in "Target audience is 13+, and it was meant to include 9-12") | An Android reader who restores onto a new phone is told their AI key, or their own API.Bible key, stayed on the old phone, instead of finding it silently gone. | S / low | Android (Windows and Linux restores checked at the same time) | Open: the message's wording | It needs a way to detect a restore. It is also the stated prerequisite for listing the 9–12 audience. |
| 11 | Android: Translate and drag carry invisible layout characters | A selection that includes a heading or a psalm title, sent to Translate or dragged out, carries invisible spacing characters instead of plain spaces. | S / medium | Android | Open: clean the dragged and translated text through `readerText`, as Copy already is (recommended: iPad and Mac keep native drag, so blocking it on Android alone opens a divergence), or block dragging Scripture out of the page. | Not noticed in practice, and the fix touches the fragile selection toolbar. The drag can be cleaned from the long press the bridge already overrides; whether Translate's text can be reached is not known, since the platform builds it inside the text editor, and if it cannot, that is recorded as a divergence. With the next Android selection work. *Not in BACKLOG.md.* |
| 12 | Android: taps may land about 8 points above the finger (measure first) | If real, taps register slightly above the finger, so a small control such as the narration card's ✕ can be missed. | M / high if real | Android | None until measured | Seen once, through a synthetic tap, with no reader report. Measure with a real touch path first. If real, the fix is a toolkit patch that touches every tap, and the minor-or-major question is decided then. *Not in BACKLOG.md.* |
| 13 | What we ship has largely never been run | None (internal: confidence in what ships). | M / low | The Mac (every row "builds"), iPad, the Android universal APK, the Windows .zip and arm64, the installed Linux tarball and the AppImage | Open: what "tested" should mean for a store binary CI cannot run at all. A Linux hardware pass needs an x86_64 machine. | The cheap halves went out in 1.2.20 (row 15 (c) and (d) there); its Mac package verified both linked OS floors, and no run on macOS 12 or 13 has happened. |
| 14 | Recapture the App Store and Play screenshots: the Play phone set | People browsing the Play listing see light shots with dark, legible status-bar icons, and perhaps the phone shots at the display size most phones use. | M / low | The Google Play listing | Open, and discussed before any retake: retake the phone set at 420 (where Books shows one column by design), or keep 356 and retake only the light images; and whether the tablet's light images are retaken too. | What it waited for shipped in 1.2.18. The shots are taken from 1.2.21, the first release on Play with 1.2.20's features, after it clears Play's review: one Play edit at a time. Three reviewers and approval of the images come before the upload (SCREENSHOT_PLAYBOOK.md). |
| 15 | WEB Catholic Daniel 3: a translator's note fused into the heading (SCRIPTURE_WORKLIST.md, S15's guard) | In the World English Bible (Catholic) at Daniel 3, in the app and on the website, a paragraph about the addition stops running into "THE SONG OF THE THREE HOLY CHILDREN" as one long bold heading. | S / medium | All six and the website (WEB Catholic only) | Open: (1) keep the title and set the note apart from it (recommended, nothing dropped), drop the whole heading (the recorded shape guard, which loses the real title too), or leave it; (2) at drawing (recommended: nobody downloads again) or at decoding (every WEB Catholic reader downloads again). The choice is recorded in ADDITIONS_AND_DROPS.md. | A visible text fault, confirmed live, but with two open decisions; it touches all six and needs `scripts/publish-site.sh`. The recommended option restructures publisher text under the worklist's standard: nothing dropped without an explicit decision, nothing added. The first minor release after it is decided, by default. |
| 16 | Android centres the book page on the window, the iPad on its pane (in "The reading page: what is left after 24 September 2026") | On an Android tablet with the side rail, the reading column sits centred beside the rail, as on the iPad, instead of about half the rail's width to the left. | S / medium | Android tablets (a phone held sideways must not change) | None. The entry's rule: centre on the window only when the pane spans it less the system insets. | One condition, but this rotation code has gone wrong before: emulator passes on a phone with a camera cut-out in both rotations and on the 10-inch tablet, too much for 1.2.20's week. |
| 19 | On iPhones whose screen scale is not the drawing scale, the reading pane is off by about 4% (in "iOS 27: after the scene life-cycle fix") | On the iPhone 12 and 13 mini, the Plus models and any iPhone with Display Zoom on, the reading pane's frame and clipping line up exactly. | M / medium | iPhone (those models and Display Zoom) | None | The recorded fix measures with the drawable's scale in `goAppReportSize` and `sendTouches`, a toolkit patch; `patches/fyne-2.7.4-ios-window-size.patch` does not cover it. Look in the simulator with Display Zoom first; touch mapping changes on those devices, so check on one before shipping. |
| 22 | The narration card's ✕: a double tap opens full screen (in "The open narration card covers the phone header's controls") | A double tap on the open card's ✕ only closes the card; today the second tap opens full screen in 45 of 48 phone layouts measured. | S / low | iPhone, Android phones | Open: a `fyne.DoubleTappable` ✕, which closes on either tap at the cost of the double-tap interval on every close, or a shield over the card's place for that interval, which leaves the controls it covered unreachable for that time. | Separable from the narration bar (Next major version, row 6), and need not wait for it. |
| 23 | A reader who cleared their key on purpose is told at every launch that the NKJV could not be opened (VERSION_STATES.md, *Open decisions*) | A reader who deliberately cleared their key in Settings stops being told, at every launch, that the NKJV could not be opened. | S / low | All six | Open: skip the record when the store's negative is deliberate; the launch cells `L-A` and `L-D` then fire on `key-cleared` and have to say the reader gave the translation up, which needs wording. | Safe as it stands. Small once decided; the version-state walks already hold the cell. The other two version-state questions stay in the backlog (row 19 there). |
| 26 | The marker strip confirms a verse number by any overlap with that verse's opening (M) | Nothing in the shipped translations. A number that is part of a verse's text, followed by a word that opens the verse it names, would be stripped as that verse's number and the rest looked for elsewhere. | S–M, 2.5 days / low | All six | None | `stripVerseMarkersAnchored` (`share.go:1723`) accepts an "N " token when what follows overlaps verse N's opening by any length (an invented "The city had 12 gates" cited verse 3). No such digit token in the WEB, the BSB or the WEB Catholic; the NKJV not measured. With a lead, 1.2.22's row 11 reads the selection from the lead's place and removes a number only where its verse starts and the token is that verse's number (`walkSelectionFrom`; `TestAFigureInAVerseIsNotTakenForAVerseNumber`), so what is left is the text strip for a selection with no lead or whose walk fails. The recorded fix: the panes pass the number runs' positions and the strip removes exactly those. Latent, so not in 1.2.22. |
| 27 | The study verbs do not carry the chapter they were chosen in (AI-4) | A study action chosen while continuous narration turns the chapter cites the chapter the selection was made in, not the next (Psalm 135:3 selected, Explain clicked after the turn: "Psalms 136:3"). | M, 2.5 days (about 1 of them Go) / low | All six | None | The span carries verse numbers only and the dispatch reads them against the chapter then current (`ai.go:185`, `selection_span.go:16`); the desktop menu survives the refresh, and the Mac's menu tracking holds the queued turn until the menu closes. Rare to trigger; once it fires, 0 of 1,188 chapter pairs stay right. Fix: book and chapter travel with the span; `selectionStudyMenu` stamps them when the menu is built, which fixes Windows and Linux in Go and can go in any minor release; the native panes return the chapter through `bibleTextAIMenuTapped`, `bibleTextStudyMenuTapped` and `btaSelectionAction`, the seams 1.2.22's row 11 widened for the lead, which does not carry the chapter. Not in 1.2.22: its native half is another change to every pane's seam. |
| 28 | The Apple plain-text fallback page has no verse geometry (C, IOS-PLAIN-FALLBACK-ZERO-SPAN, MAC-PLAIN-FALLBACK-ZERO-SPAN; P3) | On the page the iPhone, iPad and Mac show when a chapter fails to import three times, a selection cites the verse it was made in, the psalm title stays out of quotations, and highlights, notes and search arrivals land. Today every verb there searches the whole chapter (Psalm 136:10's refrain cited 136:3; "light" in Genesis 1:5 cited 1:3). | M, about 2.5 days / medium | iPhone, iPad, Mac | None | The fallback empties the verse index (`reading_ios.go:3560-3564`; `reading_macos.go:1723-1725`, `gMacContentStart` 0; converters at :3478 and :1668). No reader has been seen on the page; on it 60.2% (WEB) and 57.7% (BSB) of double-clicked words cite an earlier verse, 0.88% and 0.99% of poem lines and 0.34% and 0.35% of whole verses. Fix: one walk over the cut HTML builds an attributed fallback (numerals at 0.66 em, title italic, headings bold) and runs the import's own derivations, so every walker answers and Go gets a real span; the three fallback contract tests change, and a development flag forces the import to fail for a simulator check. It retires the no-span title and heading rules, is tier 2 of 1.2.22 rows 6 and 7, answers 1.2.20 row 7's question whether such a selection should cite the chapter, and can close Stays in the backlog, row 24. Not in 1.2.22: Objective-C on both Apple panes, for a page no reader has reported. |
| 29 | A selection inside a publisher's heading resolves to the verse above (APPLE-HEADING-MATTER-TO-VERSE-ABOVE; P4) | On the iPhone, iPad and Mac, a heading word selected on its own stops citing, and quoting as Scripture, the verse above ("Cain" in "The Descendants of Cain" cites Genesis 4:16; the "The" of "The Fourth Day" is quoted from 1:13). | S–M, 2 days / medium | iPhone, iPad, Mac; Android for parity | Waits on Next major version, row 5: what selecting a heading means (Decisions still open, 23). | `btIOSVerseAtIndex` ignores the heading range the index keeps (`reading_ios.go:273`), the Mac never asks `btMacIsHeadingParagraph` (`reading_macos.go:284`, :292), and Android has the same backward rule (`android/BtBridge.java:1593`); Go's repair (`headingOnlySelectionVerse`, `share.go:1427`, with its `span.lo == above` test at :1462) fires only when the words are not found first. BSB: 2,023 of 7,373 single heading words (27.4%), and 1,301 of 1,896 headings have such a word; the WEB Catholic 27 of 113; the WEB none of 23. Fix: each pane tests the selection against its heading range, then (A) reports the verse below, with a Go companion, or (B) clamps heading matter out as the title is clamped. A fix to what readers have, so a minor release's; but (B) forecloses answers row 5 keeps open, so its decision comes first. |
| 31 | NKJV red letters: the generator places a run at the first copy after its cursor, and retries from the verse's start (RL-NKJV-1, RL-NKJV-2) | Nothing today. Before the fix, a regenerated table could redden another copy of a run's words, inside a longer word or in narration, or leave a marked poem line black, and generation would report success. | S, 0.5 day / low | None until a regeneration; then all six and the website (NKJV) | The regeneration needs the NKJV opened in the app once, to regenerate and compare (Decisions still open, 43). | FIXED in the generator on 14 October 2026; the table is not yet regenerated with it. The parser counts each verse's non-space characters, leaving out the verse number and headings as the app's decoder does, and each run is placed at its count; the retry from the verse's start is gone. A run whose characters are not at its count is placed only by their one copy in the verse; no copy or several, an end inside a word, or a chapter missing from the cache fails generation, where a missing chapter's verses were left out of the table, which reads as black letter. `--no-fetch` derives from the cached chapters with no key read and no call. Shown on invented markup only: `TestTheNKJVRedLetterGeneratorPlacesEachRunAtItsOwnPlace` and `TestTheNKJVRedLetterGeneratorRefusesWhatItCannotPlace` fail on the derive logic before the fix ("Go" in "Gospel", the narration's copy, a second run folded into the first and the table reported as made, a missing chapter). Still to do: no NKJV text was opened, so the run on the app's NKJV cache, which must write the shipped `red_letter_nkjv_data.go` byte for byte, has not been made; a verse where the count and the app's text disagree fails that run by name instead of writing a table. Any minor release: a build tool, and no table readers have changes. |
| 32 | BSB red letters: the generator takes the first copy from its cursor (RL-BSB-1) | Nothing: the regenerated table is the shipped one byte for byte. Before the fix, regenerated through its normalized path alone, the table would have moved Acts 9:10's red letters to the narration's copy of the words. | S, 1 day / low | None (a build tool) | None. The publisher's `bsb_usfm.zip` was downloaded for the proof on 14 October 2026 (Decisions still open, 43). | FIXED 14 October 2026. Each run is placed at its offset in its verse, counted in letters and digits on the USFM walk with heading lines left out, and both the exact match and the typography-normalized one are tried only there. A run whose letters are not at its offset is placed only by their one copy in the verse; no copy or several, or an end inside a word, fails generation. `TestTheBSBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` and `TestTheBSBRedLetterGeneratorRefusesARunItCannotPlace` fail on the generator before the fix. Regenerated from that download, before the fix and after it, the table is the shipped one byte for byte, every run at its offset (2,251 exact, 43 typography-normalized, none by its only copy). The archive differs from the one recorded on 23 August (its ACT.usfm and LUK.usfm are dated 2 October 2026); the table does not. `TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats the check where the sources are unpacked. Any minor release, as row 31. |
| 33 | WEB red letters: the same first-copy search (RL-WEB-1) | Nothing: both regenerated tables are the shipped ones byte for byte. Before the fix, an invented verse showed the shape, Peter's "I am he." reddened for Jesus's. | S, 0.75 day / low | None (a build tool) | None. eBible's USFM zips were downloaded for the proof on 14 October 2026 (Decisions still open, 43). | FIXED 14 October 2026, as row 32 with the punctuation-preserving match alone; a run of punctuation alone, a closing mark a note parts from its words (John 8:58), is placed between the letters on either side of its offset. `TestTheWEBRedLetterGeneratorPlacesEachRunAtItsOwnPlace` and `TestTheWEBRedLetterGeneratorRefusesARunItCannotPlace` fail on the generator before the fix. Regenerated from eBible's zips of 13 October 2026, which differ from the copies recorded on 23 August, before the fix and after it, both tables are the shipped ones byte for byte: the WEB's 2,053 verses at their offsets and 6 by the pinned recoveries, the WEB Catholic's 2,052 and 7, none by its only copy. `TestTheRedLetterGeneratorsReproduceTheShippedTables_LocalOnly` repeats it. Any minor release, as row 31. |
| 39 | Explicit permission before text goes to a third-party AI (App Store guideline 5.1.2(i)) | Before the first study request to a provider, the app asks once whether to send text to it, and sends nothing until the reader allows it. | M / medium | All six (one rule), or the Apple apps with a recorded divergence | Open: the wording, to be approved (a draft exists outside the repository); how the permission is withdrawn (Decisions still open, 5). | The guideline asks for clear disclosure of where personal data goes, third-party AI by name, and explicit permission before it is sent. The note under the key field (`ai_settings.go`) meets the disclosure; the permission is only implied by entering a key, and Ask, which sends text the reader types, first reached App Review with 1.2.21. A confirmation for each provider at its first request is the smallest change that gives it. A minor release's, not the major's: it concerns what readers already have, and a rejection on this guideline would make it the next release's first item. |
| 40 | Windows: the Store checks deferred from 1.2.20, and a look at its share fallback (in "Windows: use the native Share sheet"; "The Windows audio smoke is probably testing a silent sink"; "Microsoft Store: from the reserved name to the first submission") | Nothing if they pass. If not, a picture share from the Store install fails or saves somewhere unexpected. | S / low | Windows (the Store install and the direct download, on the Windows 11 arm64 virtual machine) | Decided 7 October 2026: moved to the release after 1.2.20. Which release now is open (Decisions still open, 2). | 1.2.20's row 11, not run: the virtual machine's guest agent was unavailable, SSH timed out and its console dropped keys, so the machine has to be reachable first. Then the share from the Store install; the three Store checks (`WINDOWS_STORE_LISTING.md`, *Resuming with a Windows machine*), of which the browsers' prompts for `bibletext:` need a local page; and step 3 of the audio smoke, which needs a Windows ARM build with cgo or a stub. The three entries then close and PLATFORM_MATRIX's Sharing notes are corrected. In the same sitting, a picture share whose card could not be read, through the in-app sheet's fallback (1.2.20, row 1; in the entry on Share as image saying "Picture saved" for a card it could not read): it ends the share by the code and by `TestACardThatCouldNotBeReadOpensNoSheet` under Windows' rules, and has not been seen on Windows. |
| 42 | 1.2.20's rows: the checks still to run on hardware (in "1.2.20's roadmap rows with no entry of their own"; "Android: an activity destroyed with the process alive stops the narration") | Nothing if they pass. | M / low | iPhone, iPad, Android, Windows | None | Back on Android 11 and older (1.2.20, row 4): narration plays on after Back finishes the root activity, on an emulator at API 30 or below, none having been run; Android 7.0 to 11 (API 24 to 30) are still supported. Ask (1.2.20, row 18): reading, dictation and VoiceOver on a physical device. The shared panel (row 19): a native Windows run, and the physical hit region of the 44-point control under the toolkit's eight-point touch compensation. iOS rendering during app switching and font memory (rows 20 and 21): the cause of the physical exits seen before them, which the simulator runs do not establish. |
| 44 | The release procedure does not say why Play's publish-on-approval is checked by hand (in "The release pipeline: what still waits on something it need not") | None (internal). | S / low | Google Play release procedure | None | Apple's release type is set by `appstore/submit-version.py` and read back before a submission; Play's, Managed publishing off, is read in the Publishing overview and recorded (RELEASING.md stage 6; PLAY_LISTING.md, *Publishing mode*), with no reason given for its being a hand check. The only asymmetry between the Apple and Play legs of the release procedure still open at `6bf27aee4`. |
| 45 | The NKJV's card route and selection route: the opt-in walk on an NKJV cache (in "The NKJV's card route and selection route disagree in the opt-in walk") | Nothing (internal). | S / low | The suite | With decision 43: the walk needs the NKJV opened in the app once, so that a cache is there to walk, compared without printing the text. | 1.2.22's row 20 fixed the test's stand-in and showed, on the WEB given the NKJV's marking, that the stand-in accounts for differences of the NKJV's shape. `TestPassageShareEqualsSelectionRouteOverLocalCaches_LocalOnly` has not been run on an NKJV cache; the entry closes when it passes there, or names what is left. |

---

## Next major version

**Switch or branch.** Every row here but one is built on `main` behind the
`next` switch, as [NEXT.md](NEXT.md) describes. **Row 13, oto 3.5, keeps a
branch**, because it changes a module version and the Go version every build
uses. No row needs a higher minimum OS version. The first three rows are
already on `main` behind the switch; NEXT.md's register is their record, with
their tests and open decisions.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Switch or branch | Decision | Notes |
|---|---|---|---|---|---|---|---|
| 1 | "Same saying, another occasion" among the Gospel parallels (NEXT.md, piece 3) | Selecting a saying of Jesus that he repeated on another occasion shows a new group of links among the Gospel parallels: 102 groups, 274 pairs. | M / medium | All six | Switch: on `main` | Open: NEXT.md P1–P6. Recommended: hold R01 back, place the 22 verses as new sets with the existing sets unchanged, keep the "Cf." pairings out. | It shares `crossrefs.go` with row 3, so the panel's deepest read is measured again once both settle. |
| 2 | The NKJV refreshed at cache epoch 8, with an offline bridge (NEXT.md, piece 1) | With epoch 8 dropped (recommended), nothing changes for readers now. The bridge waits for the next NKJV decoder change: a reader offline when that update is due then keeps the previous copy instead of dropping to the WEB. | M / medium | All six | Switch: on `main` | **Open recommendation:** drop the epoch-8 refresh and keep the offline bridge for the next decoder change. Also NEXT.md N1, the picker's sentence. | Epoch 8 was to reach existing copies faster than the 30-day refresh. 1.2.19 shipped the decoder fixes on 3 October, so copies refreshed under 1.2.19 have them within 30 days, by about 2 November for readers who have updated. At the major, a forced refetch costs about 197 metered requests and about 13 MB per NKJV reader on the shared key, and helps only readers who skipped every minor release in between. Without the epoch this row changes no stored data; with it, a device moving between `next` and release builds crosses epochs, and each crossing costs a refetch. |
| 3 | The Greek Esther mapped verse for verse, like Daniel 3 (NEXT.md, piece 2) | Reading Esther in the WEB Catholic, notes, highlights and cross-references cross from the other editions for about 164 verses, where today they stop at the book, and the app and website stop saying the two texts' verses do not correspond. | M / medium | All six and the website | Switch: on `main` (ported in ad2080a1f) | Open: NEXT.md E-A, E-B, E-C (recommended: keep the gap marks, yes to one line for every uncovered addition, approve the two sentences as a safety net). **Open recommendation:** let it leave the switch in a minor release (1.2.21 went out without it), rather than wait for the major as approved on 6 October 2026. | `main` tells readers something untrue: the site's caveat (`caveat`, `cmd/websitegen/notice.go`) and the note line (`placementCopy`, `notes_anchor.go`) say the verses do not correspond, yet 164 of the WEB Catholic's 167 Esther verses keep the Hebrew numbering. It changes no stored data, and NEXT.md lets a fix both states need be written once, outside the switch. Once A–C are answered and the port is verified it can graduate; then the panel's deepest read is measured again (row 1). |
| 4 | Cross-references panel: the smaller presentation points from the 2 October audit | Tidier lists: no row repeats a parallel or points back into the selection, parallels are capped for long selections, half-verses are not doubled, and no selected verse is left empty by the 40-row cut. Previews keep red letters, italics and poem lines. | M / low | All six | Switch | Approved to bundle later. Open: which of PAR-7/C-9, PAR-8, C-7, C-8, C-11 and C-6 to take (recommended for C-6: reuse the verse-of-the-day paragraph). | The major changes this panel already (row 1), so one pass here avoids doing it twice. The NKJV panel's second pass folds in if licensing says yes. |
| 5 | Headings: quoting, notes and links need one deliberate pass | Selecting a section heading, a parallel-passage line, a Psalm 119 letter or a major section head follows a deliberate rule for each kind, instead of today's one rule. | L / high | All six and the website | Switch | Open: what selecting a heading should mean (the section it introduces, the heading as a label, or nothing at all); whether the heading-only rule is revisited; whether the native rule looks forward. | Behaviour readers will notice. A note anchored by a heading must still land in the same place, so the stored note format does not change. After 1.2.20 rows 6 and 7. The audit that keeps every heading out of sharing in one place could come earlier, in a minor. The Apple panes' heading fact (Later minor releases, row 29) waits on the first open question here, since one of its two shapes forecloses answers this row keeps open. |
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
| 8 | Desktop sheets in a window under about 420pt tall start inside the header (in "Recapture the App Store and Play screenshots") | In a very short Mac, Windows or Linux window, some sheets open over the lower edge of the header. | S / low | Mac, Windows, Linux | Open: raise the window's minimum height to about 420pt, or let a sheet that cannot clear the header cover it; and the iPad Settings footer's separator, a design choice | An edge case with no reader report. A choice between the two, which moves it to a minor. |
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
| 24 | iOS: if all three HTML imports of a rebuild fail, the fallback keeps the old palette's colour (in "A light/dark change with a sheet open left the app half in each theme") | In theory, text in the old colour on the new paper after a light/dark switch, until the next push. | S / low | iPhone, iPad (and the Mac twin) | None | Theoretical: only single failures are known. Setting `textColor` explicitly on the fallback closes it. Any report of a failed import, or the fallback's geometry (Later minor releases, row 28), which can set the colour as it builds the page. |
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
ones decision 2 now holds; and of the bugs in Later rows 1 to 23 and Stays,
those a check here could show were fixed too (rows 24 to 29 there), with the
reason each of the others stayed given under that table. What is still
open, by when it is needed. A recommendation is marked where one is made.
Rows are cited by section: *1.2.22*, *Later* for later minor releases, *Next
major* and *Stays*.

### Before the 1.2.22 work starts

1. **The legacy Entry pane** (1.2.22, row 10): taken 14 October 2026; it
   takes the cursor's span, as planned. Deleting the dead pane stays possible
   in any later release.
2. **The items moved on 7 October 2026 to the release after 1.2.20, and two
   larger bugs**: rows 35 to 38 of Later went into 1.2.22 on 14 October 2026
   (rows 13, 16, 17 and 20 there), as the digit, row 25 until then, had with
   row 11. Row 40 can join 1.2.22 or follow 1.2.23's lead, and needs the
   Windows virtual machine reachable first. AI-4 and P3 (Later, rows 27 and
   28) are bugs that need no decision of their own and were left out of
   1.2.22's fixes all the same, for their size and risk: about 2.5 days
   each, AI-4 changing every pane's seam and P3 both Apple panes'
   Objective-C. What is open is whether each joins 1.2.22, or AI-4's Go half
   alone (about a day, Windows and Linux, with a recorded divergence), or
   neither.

### Before the work after 1.2.22 starts

3. **What leads 1.2.23**: the selection's place in its verse went into
   1.2.22 on 14 October 2026 (row 11 there), so what is left is whether the
   desktop's navigation rule (Later, row 1) leads it.
4. **The selection's place in its verse** (1.2.22, row 11): taken 14 October
   2026; one change on all four panes, as the parity rule asks, in 1.2.22.
5. **Permission before text goes to a third-party AI** (Later, row 39): the
   wording, to be approved; how the permission is withdrawn.
6. **The desktop's navigation rule** (Later, row 1): the dead zone and the
   settle; whether it ships unswitched in a minor release as approved
   (recommended) or is built behind `next` and graduates early.
7. **Shares that cannot finish** (Later, row 2): one heading and line for the
   desktop sheet; whether "Could not share the card." is reused where it
   fits; whether to detect iOS refusing to show the sheet.
8. **WEB Catholic Daniel 3** (Later, row 15): keep the title and set the note
   apart (recommended), drop the whole heading, or leave it; at drawing
   (recommended, no re-download) or at decoding.
9. **The Greek Esther** (Next major, row 3): E-A, E-B and E-C (recommended:
   keep the gap marks, yes to the one line, approve the two sentences); and
   **whether it leaves the switch in a minor release (recommended)** or waits
   for the major as approved on 6 October 2026.
10. **The narration card's ✕ double tap** (Later, row 22): a double-tappable
    ✕, or a shield for the double-tap interval.
11. **A key cleared on purpose** (Later, row 23): stop telling the reader at
    every launch, and the wording that then says they gave the translation
    up.
12. **The NKJV's run-together words** (Later, row 9): send the report upstream
    (drafted for approval first); whether the app carries a correction list,
    checked entry by entry against print, once the terms are confirmed to
    allow it.
13. **A restored Android phone** (Later, row 10): the wording of the "your key
    stayed on the old phone" message.
14. **Android drag and Translate** (Later, row 11): clean the text
    (recommended, keeping parity with iPad and Mac), or block dragging.
15. **Website and app layout** (Later, row 6): one gap for two headings in a
    row; the column width; whether a paragraph that opens in prose and turns
    to poetry stops being justified whole on the website and Android 15+.
16. **Play phone screenshots** (Later, row 14): retake at 420, or keep 356 and
    retake only the light images; and whether the tablet's light images are
    retaken.
17. **Store builds CI cannot run** (Later, row 13): what "tested" should mean
    for them.
18. **An omitted verse's mark under a mark that covers it** (AND-4, apart
    from its fix, which is 1.2.22's row 15): every surface ties the "[21]"
    mark to the verse after it, so a one-verse mark on an omitted verse
    lights nothing; whether such a mark lights it, as a change of its own.
    With it, where a note shown from its link alone on a range that starts
    at such a verse is drawn (PLATFORM_MATRIX.md, divergence 34): at the
    verse after the hole or at the chapter top, on every surface.

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
23. **Headings** (Next major, row 5): what selecting a heading should mean,
    and whether the native rule looks forward. The Apple panes' heading fact
    (Later, row 29) waits on the first, since one of its two shapes
    forecloses answers to it.
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
35. **Short desktop windows** (Stays, row 8): raise the minimum height, or let
    a sheet cover the header; and the iPad Settings footer's separator.
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
    walk** (Later, row 45): an OK to open the NKJV in the app once, to
    regenerate its table and compare and to walk its card route against its
    selection route. The other half, downloading the eBible USFM zips and the
    publisher's `bsb_usfm.zip`, was approved on 14 October 2026: the BSB's
    and the WEB's generators were fixed that day and their tables regenerate
    byte for byte (rows 32 and 33). The NKJV's generator was fixed with them
    and is shown on invented markup only; no NKJV text has been opened for
    this work, and its fix is proven only when its table regenerates byte for
    byte.
