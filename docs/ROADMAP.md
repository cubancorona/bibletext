# Roadmap

The plan as of **6 October 2026**: which release each open item in
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

- **Minor releases come from `main`, without the `next` switch.** 1.2.20,
  1.2.21 and each one after are cut from `main` by [RELEASING.md](RELEASING.md)
  when they are ready, and built without the `next` tag: the last release's
  behaviour plus their own fixes. A minor release never waits for the major
  one.
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
  when readers need it sooner. One exception is made on purpose: the
  desktop's navigation rule, approved on 1 October 2026 for a minor release
  (Later minor releases, row 1).

---

## 1.2.20, the next minor release

Planned to go out about a week after this plan. Eight fixes readers can see,
the largest being the wrong verse cited for a drag that starts on the previous
verse's full stop; two checks on work already shipped and a look at one share
popover; guards on the release tools; and one documentation and tests pass.
Nothing in it changes stored data, makes readers download a translation again,
or needs new wording. The seven decisions it needed were taken on 6 October
2026, and each is in its row (rows 4, 7, 8, 12, 13 and 16).

Reader-visible fixes come first, then checks on what has shipped, then
internal work. **If the week runs short**, these move to 1.2.21, in this
order:

1. Row 9 (the popover look).
2. Row 14 (the intermittent test failure; keep logging it).
3. Rows 6 and 7 together, if the sweep is not committed by the middle of the
   week.
4. The Android half of row 8.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | Share as image says "Picture saved" for a card it could not read | Very rarely, a *Picture saved* sheet opens for a picture that was never saved, and its Email… does nothing. After the fix no sheet opens. | S / low | Linux tarball and AppImage; Windows only where its own Share sheet cannot open | None: the desktop's approved rule for a picture nothing true can be said of — no sheet, the cause logged, as the snap already does | No new wording. Telling the reader instead needs a line of its own, which goes with Later minor releases, row 2. The entry's host test proves it: a card path that does not exist, the reveal stubbed to say yes, and no sheet (control: the file present). |
| 2 | A dropped session bus spins a core until the portal wait ends (Linux) | Nothing in practice: at most one core busy for up to 15 seconds while the desktop session ends. | S / low | Linux (the snap; the tarball and AppImage where a portal runs) | None | Two lines (`sig, ok := <-responses`, an error on a closed channel) and a direct test. |
| 3 | A sheet turned from one landscape to the other keeps its side padding on the old side (Android) | A phone turned straight from one sideways position to the other with the note sheet open: the sheet runs under the camera cut-out or the button bar until it is reopened. | S / low | Android phones | None | The composer's watch compares the sheet area's position and width as well as the canvas size. Proved by a host test, since the emulator cannot turn 180° in one step. Ask's half waits until Ask returns to the menus. |
| 4 | Android: an activity destroyed with the process alive stops the narration | Narration can stop by itself during screen-off listening, or after Back on Android 11 and older, while the app still shows it playing. | S / medium | Android (iPhone keeps playing, so this is also a parity gap) | **Decided 6 October 2026** — Android narration when the system reclaims the screen: don't stop the audio, matching iPhone. `OnStopped` stops no audio on Android. | It breaks the case background narration exists for. Reproduce it first on the emulator with Developer options > "Don't keep activities" on (V9); if it does not reproduce, it becomes a watch item. |
| 5 | Android's compact page: a pill under a heading stands down; and a psalm title sits on verse 1 there | On an Android tablet, or a phone held sideways, a psalm title gets back its small gap above verse 1, and a closed note bubble under a section heading sits centred in its gap, as on every other device, not 7 px high. | S / low | Android (tablet, and phone sideways) | None | Both are in the same spacing code with known causes, and this brings the last surface under the one-bubble rule. The entry's sentence that a title's gap is a blank line on that page too is corrected in the same change. The title half is *not in BACKLOG.md*. |
| 6 | Verse attribution, finding 1: a drag that opens on the previous verse's punctuation can cite the wrong verse | A share or note whose selection starts on the full stop ending the previous verse cites the verse actually selected. Today about 1 in 800 such drags goes wrong, and up to about 1 in 40 two-word drags in the Beatitudes and Matthew 23 and 25. | M / medium | All six (shared share and notes code) | None. The technical route — the nearest match, or re-anchoring the region to the selection — is chosen by the sweep. | A wrong citation under Scripture is serious, and the fix is small and local in shared Go, so one change reaches all six. The 21 September measure ran on the 89 WEB gospel chapters the app embeds (`seed.go`, `assets/seed/web-gospels.json`), which `share_cut_sweep_test.go` already sweeps with no span. Extend it to the spanned sweep of about 35,000 drags, hermetic in CI, and commit it first with today's rates as the before. The fix brings the boundary drags to zero wrong without moving any other drag. One review with row 7. |
| 7 | Verse attribution, finding 2: a psalm superscription can be shared as Scripture | A drag from a psalm title into verse 1 stops sharing the title's words ("A Psalm by David, when he fled…") as if they were verse 1. | S / low | iPhone, iPad, Mac, Android (the Windows and Linux pane draws the title outside the text) | **Decided 6 October 2026** — a psalm title selected on its own: cite verse 1 with no quotation, as a heading does. | `stripHeadings` (`share.go`) and `headingOnlySelectionVerse` read `Headings` and never `Superscriptions`. The same strip-and-locate chain as row 6, so row 6's sweep covers it. |
| 8 | Psalm titles hyphenate on the Apple apps and Android (in "The reading page: what is left after 24 September 2026") | A long psalm title that wraps stops breaking a word with a hyphen, as on the website, Windows and Linux. | S / low | iPhone, iPad, Mac; Android 15 and later | **Decided 6 October 2026** — Psalm titles: no hyphenation on the Apple apps and Android 15+, recording Android 13–14 as a divergence. | Apple: `hyphens: none; -webkit-hyphens: none;` on `p.pst` (`reading.go`), checked with `scripts/check-ios-pane.sh` and a look. Android: `LineBreakConfigSpan.createNoHyphenationSpan()` (API 35), added by the pass that already walks every title there (`keepHeadingsRagged`, `android/BtBridge.java`); `readerText` and Copy drop the span. Check on the API 35 and API 33 emulators that the selectable, justified view honours it; if it does not, the Apple half ships alone and the Android half returns to the backlog. The divergence goes into PLATFORM_MATRIX.md with row 16. |
| 9 | iPad and Mac: where the share popover points | On iPad the popover may point at the middle of the page rather than the selection, and for the verse of the day at nothing; on the Mac it is not known whether the share choices appear at all. | S / low | iPad, Mac | None | **A look only.** The iPad simulator and the Mac. If it looks right the entry closes; any fix goes to 1.2.21. The Mac case matters most: it could be a Share button that never works. |
| 10 | Android 17: the 1.2.19 heading fix has not been checked | Nothing if it passes. If it fails, some headings on Android 17 sit pushed to the right or stretched. | S / low | Android 17 | None | A check on an API 37 emulator, so no phone is needed; fixed only if it fails. *Not in BACKLOG.md.* |
| 11 | Windows: use the native Share sheet, from the Store install; the Store checks never run (in "Microsoft Store: from the reserved name to the first submission"); the audio smoke's natural end | Nothing if it works. If not, a picture share from the Store install fails or saves somewhere unexpected. | S / low | Windows (the Store install and the direct download, on the Windows 11 arm64 virtual machine) | None | One virtual-machine session. The share from the Store install; the three Store checks never run (`WINDOWS_STORE_LISTING.md`, *Resuming with a Windows machine*: web links opening the app through the shell, the browsers' prompts for `bibletext:`, whether the direct download's loopback listener draws a firewall prompt); and step 3 of the audio smoke, the natural end, now that the virtual machine's audio is known to be real. Then the share entry is marked shipped, "The Windows audio smoke is probably testing a silent sink" closes, and PLATFORM_MATRIX's Sharing notes are corrected. x64, Windows 10 and a late sheet stay watch notes. |
| 12 | A Play upload's commit restarts a review in progress | None (internal): an upload stops sending a release already in Play's review back to the start. | S / low | Google Play release tooling | **Decided 6 October 2026** — Play uploads: wait for a review already in progress instead of restarting it. | Commit the upload as `promote` does, so that Play refuses it while anything is in review, and say on the refusal to run the same command once the review clears; a test in `play/test_play_publish.py`. In place before the 1.2.20 upload. It changes the documented release steps: stage 6 of RELEASING.md is rewritten with the code, not before, and until then its check of the Publishing overview is the procedure. |
| 13 | The App Review notes guard reads only the first line | None (internal): App Review is never sent notes that describe the wrong release. | S / low | App Store (iPhone, iPad, Mac) | **Decided 6 October 2026** — notes for Apple's reviewers: stop naming old versions in them. | So the check refuses any x.y.z anywhere in the notes other than the packaged version, with no allow-list. 1.2.19's iPhone notes name 1.2.18 twice, in "(1.2.18: …)" comparisons, which the 1.2.20 notes leave out. It lands before the 1.2.20 notes are written. |
| 14 | An intermittent failure of the verse-of-the-day card test | None (internal). | M / low | The suite, on every CI system | None | `TestTheCardSetsThePassageInTheReadingFace` (`verse_of_day_test.go`) failed on 30 September and 2 October with nothing to explain it, and was not investigated. A random red run on release day holds up the tag. At most a day: if the cause is not found, keep logging it and do not mask it. *Not in BACKLOG.md.* |
| 15 | Release-tool fixes with no decision | None (internal). | S each / low | Apple, Google Play, Microsoft Store, snap and GitHub release tooling; the Mac reading view for (g) | None | Each a few lines, guarding the next submission. (a) `msstore/submit.py` keeps the last release's `commitStatus` in a new submission: start a fresh state on `create`, with a test. (b) `scripts/run-ios-device.sh` and `scripts/release-ios.sh` build only from the repository root: use `go -C`. A locked phone costs a full rebuild, so keep the signed bundle in a private folder until it installs. (c) Every release path proves the atomic preferences writer reached its binary: the patch (`patches/fyne-2.7.4-atomic-prefs.patch`) changes Fyne's `app/preferences_nonweb.go`, which every platform but the web builds, and only `scripts/release-mac-store.sh` checks its marker today. Add the same check, with its control, to `release-ios.sh`, `build-android.sh --release` and the `release`, `msstore` and `linux-stores` workflows. The failure it guards against is every note emptied after a crash mid-save. (d) `release-ios.sh` reads `UIDeviceFamily` back from the exported .ipa, since a build could drop iPad support without a word. (e) Stage 10 of RELEASING.md names the arm64 snap rerun. (f) The public-surfaces check requires the App Store and Snap Store links in the release notes, ideally exactly the set of stores the download page links. (g) The Mac note card's outline walk (`btMacCGPathCreate`, `reading_macos.go`) names two element constants AppKit marks macOS 14 (`NSBezierPathElementCubicCurveTo`, `NSBezierPathElementQuadraticCurveTo`), and the package builds with `-Werror=unguarded-availability-new`, so an Xcode that starts flagging them in a `switch` would fail the Mac release on its day. Use `NSBezierPathElementCurveTo` for the cubic (the same value, only deprecated) and the raw value for the quadratic, which has no older name. (b) is from "scripts/run-ios-device.sh: a locked phone costs a full rebuild"; (c) and (d) from "What we ship has largely never been run"; (e) from the release pipeline entry; (f) from "Rework the download page"; (g) from "macOS 12 and 13: an open note card calls a macOS 14 method". |
| 16 | Documentation and tests | None (internal). | S / low | The documents and the suite | **Decided 6 October 2026** — Linux release date: the day the store files are built. The oddly named probe test file: rename it as a proper saved-note test. | **Docs.** Bring the stale lines up to date: the release pipeline entry's Play promotion (done, first used for 1.2.19) and screenshot push; the screenshots entry's heading; PLATFORM_MATRIX's divergence 29, its Sharing proof notes and its Flatpak mentions, and ARCHITECTURE.md's; SCRIPTURE_WORKLIST's S21 `/nkjv/` line, the Psalm 119 acrostic in its defects list, the S15 and S17 statuses, and its "990 poetic paragraphs" defect line, which disagrees with the 24 September reading-page record (Later minor releases, row 6). Add to PLATFORM_MATRIX's Divergences the ragged page on Android 13 and 14 (closed as by design on 6 October 2026) and, after row 8, the titles' hyphenation there. Correct "Three comments that describe code that changed under them", and ANDROID.md on the note sticker and the old "2 of 3 on this passage". Write the date rule into `linux/releases.toml`'s header and stage 1 of RELEASING.md. Retitle the CI race-detector entry, and mark the Windows audio smoke's steps 1 and 2 done. **Tests.** Verse attribution, finding 3: CI walks the embedded WEB Gospels for the omitted-verse strip, hermetically; an opt-in walk of every downloaded translation goes through `realCachePath`, which skips in CI by the hermetic rule; a planted `[12]` token is a control that must fail. Rename `zz_probe_refute_test.go`, which checks that an old note round-trips byte for byte, as a saved-note compatibility test. |

Three entries close once 1.2.20 has run its checks: "Windows: use the native
Share sheet" and "The Windows audio smoke is probably testing a silent sink"
after row 11, and "iPad and Mac: where the share popover points" if row 9
finds nothing wrong.

---

## Later minor releases: 1.2.21 and after

The order:

- **1.2.21** leads with the desktop's navigation rule (row 1).
- Also in 1.2.21, once their decisions are taken: shares that cannot finish
  (row 2), the WEB Catholic Daniel 3 heading (row 15), and the Greek Esther
  if it is decided to leave the switch then (Next major version, row 3).
- The small native fixes (rows 17 to 22) can fill any minor release.
- The rest follow as their decisions arrive.

| # | Item (BACKLOG.md entry) | What a reader notices | Size / risk | Platforms | Decision | Notes |
|---|---|---|---|---|---|---|
| 1 | The desktop takes the phones' navigation rule | A Mac, Windows or Linux window taller than it is wide shows Read, Books and Search on a bar along the bottom, as phones and tablets do, instead of a rail down its narrow side. | L / medium | Mac, Windows, Linux (retires divergence 29) | Approved 1 October 2026 for a minor release. Open: the dead zone (proposed: the rail from 10% wider than tall, the bar from 10% taller) and the settle of about 250 ms; and whether it ships unswitched as approved (recommended: 1.2.21's headline) or is built behind `next` and graduates early, which NEXT.md does not yet describe. | The exception to "new goes in the major". A bar appears on tall desktop windows, and rebuilding the window mid-resize on the Mac's native reading view is new, so it needs a pass on the Mac and on both virtual machines. Tiled window managers are its main audience and one of the watch triggers of the closed first-frame entry, so centring is checked there in the same pass. |
| 2 | Shares that cannot finish say so, in one approved line | When a share cannot finish, the app says so in one short line instead of ending without a word: a picture never saved, a picture no folder took, a card that cannot be drawn, a mail program that fails behind Email…. A text share stops opening two emails for one press (not seen). | M / medium | Linux, the Windows fallback sheet and the macOS mimic for the save and Email… cases; all six for a card that cannot be drawn; iOS for a refused presentation | Recorded: a line under the buttons when the compose fails, its wording to be approved; stop at the first route that started a compose. Open: one heading and line for every desktop case; whether "Could not share the card." (the phones' approved words) is reused where it fits; whether to detect iOS refusing to show the sheet. | Gathers "Email… on the desktop share sheet fails without a word", the open tails of "Android: a text share has no failure path" and "Linux and Windows: an in-app share sheet in place of the 1.4-second notice", and the line 1.2.20's row 1 leaves out. The 20 September rule, that a share the reader started does not end in silence, and the desktop's no-sheet rule pull opposite ways here; one change with one approved line settles both on every platform. It needs a mail program on the Linux virtual machine to see the double compose. A drawing failure cannot really happen with the fonts the app bundles. |
| 3 | Two verse pictures saved in the same second overwrite each other | Two pictures saved within one second are both kept, instead of the second replacing the first. | S / low | Linux; the Windows fallback | None (a " (2)" suffix, as browsers use) | `shareImageName` (`share_parts.go`) names a picture to the second. Each share passes a preview first, so it is almost impossible to reach. Done with row 2. *Not in BACKLOG.md.* |
| 4 | An iPhone launched sideways draws the bottom bar first (in "One navigation rule on phones and tablets") | An iPhone opened while held sideways shows the bottom bar for about 0.4 s before the rail or the full-screen page. | S / medium | iPhone (an Android phone must still never flash a rail) | The technical route only: answer the first layout from the orientation the scene reports at launch, or hold it until the canvas has a size | Minor, at launch only, and it touches the first-frame layout. |
| 5 | The Windows and Linux footnote section stays ragged (in "The reading page: what is left after 24 September 2026") | On Windows and Linux, the notes at the foot of a chapter are justified like the text above them. | M / medium | Windows, Linux | None | A parity improvement. Clicking within the notes has to follow the separately drawn words. The hyphenation half is the next major's (row 12 there). |
| 6 | Website and apps: the remaining layout differences (in "The reading page: what is left after 24 September 2026") | The website and the apps space two headings in a row alike and share a column width, and a paragraph that opens in prose and turns to poetry follows one rule for justification. | M / low | The website and every app | Open: one gap for two headings in a row (site 1.1em, iPhone 0.35em, Windows and Linux 1.45em); the column width; and whether such a paragraph stops being justified (and hyphenated) whole on the website and Android 15+, which the 24 September reading-page record makes their shared rule while the worklist's 7 September defects list counts the website's 990 such paragraphs as a defect. | One rule everywhere, and no data changes. The gap for a title followed by a heading is in the next major (row 7 there), because which gap applies depends on which comes first. |
| 7 | One way to turn a verse into drawn words, not two | None (internal): the card's lost spaces were fixed on 22 September; this removes the duplicate code that caused them. | M / medium | The Windows and Linux reading view; the verse-of-the-day card on all six | None. Unify the word model, carrying the card's letter-by-letter colour splitting, or two BSB verses change colour; and run the `spacingaudit` guard in CI, which never runs today. | Readers see no change, so it is not bug-fix material. Row 8 builds on it. |
| 8 | Fyne's RichText breaks a word mid-word at the start of a segment | Search results stop splitting a word in two ("Bein" / "g therefore") where a highlight starts. | L / medium | All six (search results) | Recorded: ship only if the search-card count falls to zero. To weigh: drawing search results with row 7's word model instead of patching the toolkit. | Visible but cosmetic, and real toolkit work. Fyne 2.8 rewrote RichText wrapping, so check there before patching 2.7.4. |
| 9 | The NKJV's missing spaces: report upstream, and decide on a correction list | About 286 places in 275 NKJV verses where two words run together ("Vanityof vanities", Ecclesiastes 12:8) read correctly, in the apps and on the website. | M / medium | All six and the website's NKJV pages | Open: send the report to API.Bible (drafted first, for approval); whether the app carries a correction list meanwhile, every entry checked against the printed edition, once the API.Bible and NKJV terms are confirmed to allow a corrected reading. | The report needs no release. The list, applied after the text loads, needs no re-download, and 286 checks against print. The WEB Catholic "oF" typo would join it. |
| 10 | The recovery breadcrumb for a key that did not survive a device move (in "Target audience is 13+, and it was meant to include 9-12") | An Android reader who restores onto a new phone is told their AI key, or their own API.Bible key, stayed on the old phone, instead of finding it silently gone. | S / low | Android (Windows and Linux restores checked at the same time) | Open: the message's wording | It needs a way to detect a restore. It is also the stated prerequisite for listing the 9–12 audience. |
| 11 | Android: Translate and drag carry invisible layout characters | A selection that includes a heading or a psalm title, sent to Translate or dragged out, carries invisible spacing characters instead of plain spaces. | S / medium | Android | Open: clean the dragged and translated text through `readerText`, as Copy already is (recommended: iPad and Mac keep native drag, so blocking it on Android alone opens a divergence), or block dragging Scripture out of the page. | Not noticed in practice, and the fix touches the fragile selection toolbar. The drag can be cleaned from the long press the bridge already overrides; whether Translate's text can be reached is not known, since the platform builds it inside the text editor, and if it cannot, that is recorded as a divergence. With the next Android selection work. *Not in BACKLOG.md.* |
| 12 | Android: taps may land about 8 points above the finger (measure first) | If real, taps register slightly above the finger, so a small control such as the narration card's ✕ can be missed. | M / high if real | Android | None until measured | Seen once, through a synthetic tap, with no reader report. Measure with a real touch path first. If real, the fix is a toolkit patch that touches every tap, and the minor-or-major question is decided then. *Not in BACKLOG.md.* |
| 13 | What we ship has largely never been run | None (internal: confidence in what ships). | M / low | The Mac (every row "builds"), iPad, the Android universal APK, the Windows .zip and arm64, the installed Linux tarball and the AppImage | Open: what "tested" should mean for a store binary CI cannot run at all. A Linux hardware pass needs an x86_64 machine. | The cheap halves are in 1.2.20, row 15 (c) and (d). |
| 14 | Recapture the App Store and Play screenshots: the Play phone set | People browsing the Play listing see light shots with dark, legible status-bar icons, and perhaps the phone shots at the display size most phones use. | M / low | The Google Play listing | Open, and discussed before any retake: retake the phone set at 420 (where Books shows one column by design), or keep 356 and retake only the light images; and whether the tablet's light images are retaken too. | What it waited for shipped in 1.2.18. The shots are taken from 1.2.20 after 1.2.20 clears Play's review: one Play edit at a time. Three reviewers and approval of the images come before the upload (SCREENSHOT_PLAYBOOK.md). |
| 15 | WEB Catholic Daniel 3: a translator's note fused into the heading (SCRIPTURE_WORKLIST.md, S15's guard) | In the World English Bible (Catholic) at Daniel 3, in the app and on the website, a paragraph about the addition stops running into "THE SONG OF THE THREE HOLY CHILDREN" as one long bold heading. | S / medium | All six and the website (WEB Catholic only) | Open: (1) keep the title and set the note apart from it (recommended, nothing dropped), drop the whole heading (the recorded shape guard, which loses the real title too), or leave it; (2) at drawing (recommended: nobody downloads again) or at decoding (every WEB Catholic reader downloads again). The choice is recorded in ADDITIONS_AND_DROPS.md. | A visible text fault, confirmed live, but with two open decisions; it touches all six and needs `scripts/publish-site.sh`. The recommended option restructures publisher text under the worklist's standard: nothing dropped without an explicit decision, nothing added. 1.2.21 by default once decided. |
| 16 | Android centres the book page on the window, the iPad on its pane (in "The reading page: what is left after 24 September 2026") | On an Android tablet with the side rail, the reading column sits centred beside the rail, as on the iPad, instead of about half the rail's width to the left. | S / medium | Android tablets (a phone held sideways must not change) | None. The entry's rule: centre on the window only when the pane spans it less the system insets. | One condition, but this rotation code has gone wrong before: emulator passes on a phone with a camera cut-out in both rotations and on the 10-inch tablet, too much for 1.2.20's week. |
| 17 | iPad: the tab bar's style is chosen when the window is built (in "iOS 27: after the scene life-cycle fix") | An iPad window resized across 560 points wide while it stays taller than wide switches to the right tab-bar style at once. | S / low | iPad | None | Adding the style to `renderedLayout` (`layout.go`, which holds class, rail and landscape) is one line. Check on the iPad simulator what a rebuild at 560 points does to an open sheet; rebuilds wait while an overlay is open since 27 September (`deferOrRebuild`). |
| 18 | A reader who scrolled away from a note they arrived at is taken back by the next width change (in "A note link that opened the chapter and stayed at the top") | After scrolling away from a note reached by a link, a rotation or a window resize leaves the reader where they are. | S / low | iPhone, iPad, Mac | None | The reader's own scroll clears the restore and the marker (`scrollViewDidScroll`, `reading_ios.go`) but not `gNoteArrival`. The recorded fix sets it to nothing beside the restore disarm there and in the Mac's `btMacUserScrolled`. The iPad simulator and the Mac can both show it. |
| 19 | On iPhones whose screen scale is not the drawing scale, the reading pane is off by about 4% (in "iOS 27: after the scene life-cycle fix") | On the iPhone 12 and 13 mini, the Plus models and any iPhone with Display Zoom on, the reading pane's frame and clipping line up exactly. | M / medium | iPhone (those models and Display Zoom) | None | The recorded fix measures with the drawable's scale in `goAppReportSize` and `sendTouches`, a toolkit patch; `patches/fyne-2.7.4-ios-window-size.patch` does not cover it. Look in the simulator with Display Zoom first; touch mapping changes on those devices, so check on one before shipping. |
| 20 | Mac: narration with the reading view hidden scrolls each verse to the very top (in "A light/dark change with a sheet open left the app half in each theme") | On the Mac, narration that follows the text while Books or Search is open, or under a sheet, keeps the verse in the comfortable band. | S / low | Mac | None | `bibleTextMacHighlightVerse` reads `gTextView.visibleRect` (`reading_macos.go`), which a hidden pane gives as zero; the capture code already reads the clip view. First check, with narration playing while Books is open, whether follow runs while the pane is hidden at all. |
| 21 | Android: the night-mode patch races a foreground light/dark change (in the same entry) | Switching between light and dark while the app is in front always takes effect at once, instead of sometimes waiting for the next redraw. | S / low | Android | None | `patches/fyne-2.7.4-android-night-mode.patch` compares `currentSize.DarkMode != darkMode` before `updateTheme` writes it. Reading the night bit from the configuration the branch already builds (`AConfiguration_getUiModeNight`) removes the race. A toolkit patch, so with its patch test. |
| 22 | The narration card's ✕: a double tap opens full screen (in "The open narration card covers the phone header's controls") | A double tap on the open card's ✕ only closes the card; today the second tap opens full screen in 45 of 48 phone layouts measured. | S / low | iPhone, Android phones | Open: a `fyne.DoubleTappable` ✕, which closes on either tap at the cost of the double-tap interval on every close, or a shield over the card's place for that interval, which leaves the controls it covered unreachable for that time. | Separable from the narration bar (Next major version, row 6), and need not wait for it. |
| 23 | A reader who cleared their key on purpose is told at every launch that the NKJV could not be opened (VERSION_STATES.md, *Open decisions*) | A reader who deliberately cleared their key in Settings stops being told, at every launch, that the NKJV could not be opened. | S / low | All six | Open: skip the record when the store's negative is deliberate; the launch cells `L-A` and `L-D` then fire on `key-cleared` and have to say the reader gave the translation up, which needs wording. | Safe as it stands. Small once decided; the version-state walks already hold the cell. The other two version-state questions stay in the backlog (row 19 there). |

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
| 3 | The Greek Esther mapped verse for verse, like Daniel 3 (NEXT.md, piece 2) | Reading Esther in the WEB Catholic, notes, highlights and cross-references cross from the other editions for about 164 verses, where today they stop at the book, and the app and website stop saying the two texts' verses do not correspond. | M / medium | All six and the website | Switch: on `main` (ported in ad2080a1f) | Open: NEXT.md E-A, E-B, E-C (recommended: keep the gap marks, yes to one line for every uncovered addition, approve the two sentences as a safety net). **Open recommendation:** let it leave the switch in 1.2.21, rather than wait for the major as approved on 6 October 2026. | `main` tells readers something untrue: the site's caveat (`caveat`, `cmd/websitegen/notice.go`) and the note line (`placementCopy`, `notes_anchor.go`) say the verses do not correspond, yet 164 of the WEB Catholic's 167 Esther verses keep the Hebrew numbering. It changes no stored data, and NEXT.md lets a fix both states need be written once, outside the switch. Once A–C are answered and the port is verified it can graduate; then the panel's deepest read is measured again (row 1). |
| 4 | Cross-references panel: the smaller presentation points from the 2 October audit | Tidier lists: no row repeats a parallel or points back into the selection, parallels are capped for long selections, half-verses are not doubled, and no selected verse is left empty by the 40-row cut. Previews keep red letters, italics and poem lines. | M / low | All six | Switch | Approved to bundle later. Open: which of PAR-7/C-9, PAR-8, C-7, C-8, C-11 and C-6 to take (recommended for C-6: reuse the verse-of-the-day paragraph). | The major changes this panel already (row 1), so one pass here avoids doing it twice. The NKJV panel's second pass folds in if licensing says yes. |
| 5 | Headings: quoting, notes and links need one deliberate pass | Selecting a section heading, a parallel-passage line, a Psalm 119 letter or a major section head follows a deliberate rule for each kind, instead of today's one rule. | L / high | All six and the website | Switch | Open: what selecting a heading should mean (the section it introduces, the heading as a label, or nothing at all); whether the heading-only rule is revisited; whether the native rule looks forward. | Behaviour readers will notice. A note anchored by a heading must still land in the same place, so the stored note format does not change. After 1.2.20 rows 6 and 7. The audit that keeps every heading out of sharing in one place could come earlier, in a minor. |
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
| 3 | Android sets a Psalm title's word gaps narrower than its space (in "The reading page: what is left after 24 September 2026") | On Android 15 and later, the gaps in a psalm's italic title are about 1.5 px narrower than elsewhere. | S / low | Android | Recorded option: a `ScaleXSpan` of 278/250 over each gap, which `readerText` would have to drop | Close to invisible, and a span per gap. A rework of the titles for another reason. The titles' hyphenation on Android 13 and 14 becomes a recorded divergence once 1.2.20's row 8 ships. |
| 4 | NKJV cross references: the panel's second pass | NKJV readers see the edition's own 32,473 cross references as cards. | M / medium | All six (NKJV) | Recorded gate: nothing displays in a store build until API.Bible's licensing reply says it may | Blocked on that reply (enquiry sent 10 September). On a yes it becomes a next-major feature, done with that panel pass (Next major version, row 4). Its own `nkjvxrefs` tag stays separate from `next`. |
| 5 | The direct downloads do not register `bibletext:` links | None today: no page on the site emits such a link. | M / medium | The Windows .zip, the Linux AppImage, the Mac direct .zip | Open: register silently on first run, by a Settings switch, or not at all | Nothing has needed it since the NKJV text went live on the site on 3 October, and the Mac part conflicts with LINKS.md (Universal Links only). A page that emits the scheme again. |
| 6 | Age ratings: aim for all ages in every market | Store listings show an all-ages rating everywhere, instead of USK 12, ESRB Everyone 10+ and 18+ in Russia. | S / low | Google Play, Microsoft Store | Open: what IARC intends by its questions for a work of literature rather than a game ("Violence, Blood, or Gory Images" is the hinge) | Console work, not a build. The first gap after 1.2.20 clears both stores, retaking both questionnaires in one sitting with row 7. |
| 7 | Target audience is 13+, and it was meant to include 9-12 | Google Play can list the app for children aged 9–12; who may install it does not change. | S / medium | The Google Play Console | Open: the COPPA/GDPR certification is the account holder's alone to sign. Worth weighing: a Families reviewer will look at the AI feature. | A decision to sign, after the recovery breadcrumb (Later minor releases, row 10). |
| 8 | Desktop sheets in a window under about 420pt tall start inside the header (in "Recapture the App Store and Play screenshots") | In a very short Mac, Windows or Linux window, some sheets open over the lower edge of the header. | S / low | Mac, Windows, Linux | Open: raise the window's minimum height to about 420pt, or let a sheet that cannot clear the header cover it; and the iPad Settings footer's separator, a design choice | An edge case with no reader report. A choice between the two, which moves it to a minor. |
| 9 | CI runs with the race detector, so 14 tests never run there | None (internal). | M / low | CI | None | Out of date: the tests run in CI's second pass, and Android compile checks exist. Retitled in 1.2.20 (row 16). Making them pass under the race detector is low value. |
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
| 24 | iOS: if all three HTML imports of a rebuild fail, the fallback keeps the old palette's colour (in "A light/dark change with a sheet open left the app half in each theme") | In theory, text in the old colour on the new paper after a light/dark switch, until the next push. | S / low | iPhone, iPad (and the Mac twin) | None | Theoretical: only single failures are known. Setting `textColor` explicitly on the fallback closes it. Any report of a failed import. |
| 25 | GNOME on Wayland may open Files behind the window (in "Linux and Windows: an in-app share sheet in place of the 1.4-second notice") | A shared picture's folder may open behind BibleText, with a "ready" notice, while the sheet says the picture is shown. | S / low | Linux (GNOME on Wayland) | None | Not seen: the portal is handed no activation token or parent window. A reader report, or the share work of Later minor releases, row 2. |
| 26 | iOS ships `CFBundleExecutable: main` (in "FIXED: the executable was named after `cmd/desktop`, not after the app") | None: it shows only in crash reports and Xcode's Organizer. | S / low | iPhone, iPad | None | Low priority by its own entry. The next change to `release-ios.sh`'s packaging. |

---

## Decisions still open

The seven decisions 1.2.20 needed were taken on 6 October 2026 and are in
their rows above (1.2.20, rows 4, 7, 8, 12, 13 and 16). What is still open,
by when it is needed. A recommendation is marked where one is made. Rows
are cited by section: *Later* for later minor releases, *Next major* and
*Stays*.

### Before the 1.2.21 work starts

1. **The desktop's navigation rule** (Later, row 1): the dead zone and the
   settle; whether it ships unswitched in 1.2.21 as approved (recommended)
   or is built behind `next` and graduates early.
2. **Shares that cannot finish** (Later, row 2): one heading and line for the
   desktop sheet; whether "Could not share the card." is reused where it
   fits; whether to detect iOS refusing to show the sheet.
3. **WEB Catholic Daniel 3** (Later, row 15): keep the title and set the note
   apart (recommended), drop the whole heading, or leave it; at drawing
   (recommended, no re-download) or at decoding.
4. **The Greek Esther** (Next major, row 3): E-A, E-B and E-C (recommended:
   keep the gap marks, yes to the one line, approve the two sentences); and
   **whether it leaves the switch in 1.2.21 (recommended)** or waits for the
   major as approved on 6 October 2026.
5. **The narration card's ✕ double tap** (Later, row 22): a double-tappable
   ✕, or a shield for the double-tap interval.
6. **A key cleared on purpose** (Later, row 23): stop telling the reader at
   every launch, and the wording that then says they gave the translation
   up.
7. **The NKJV's run-together words** (Later, row 9): send the report upstream
   (drafted for approval first); whether the app carries a correction list,
   checked entry by entry against print, once the terms are confirmed to
   allow it.
8. **A restored Android phone** (Later, row 10): the wording of the "your key
   stayed on the old phone" message.
9. **Android drag and Translate** (Later, row 11): clean the text
   (recommended, keeping parity with iPad and Mac), or block dragging.
10. **Website and app layout** (Later, row 6): one gap for two headings in a
    row; the column width; whether a paragraph that opens in prose and turns
    to poetry stops being justified whole on the website and Android 15+.
11. **Play phone screenshots** (Later, row 14): retake at 420, or keep 356 and
    retake only the light images; and whether the tablet's light images are
    retaken.
12. **Store builds CI cannot run** (Later, row 13): what "tested" should mean
    for them.

### For the next major version

13. **"Same saying, another occasion"** (Next major, row 1): NEXT.md P1–P6 —
    the label, R01, the 22 verses as new sets, the "Cf." pairings, the
    credit, the hiding rules.
14. **The NKJV refresh** (Next major, row 2): **drop the epoch-8 refresh and
    keep the offline bridge for the next decoder change (recommended)**, or
    keep epoch 8; and NEXT.md N1, the picker's sentence.
15. **The cross-references panel** (Next major, row 4): which presentation
    points to take.
16. **The narration bar on phones** (Next major, row 6): where it sits, when
    it shows, how it is dismissed, whether it carries the source chip.
17. **Headings** (Next major, row 5): what selecting a heading should mean,
    and whether the native rule looks forward.
18. **Psalm title or section heading first** (Next major, row 7), and then
    the gap between them.
19. **Footnote markers** (Next major, row 8): whether to show them, and
    whether to tie them to the footnotes setting.
20. **Highlights** (Next major, row 9): how a search mark differs from a
    note's, graphite or violet in dark mode, and the multi-note tint.
21. **The daily verse notification** (Next major, row 10): on the desktops
    too, or a recorded divergence; for Android's manifest, a tag-aware
    manifest or inert entries shipped first in a minor; the optional
    `/today` page, sparkle and dot.
22. **Share-card typefaces** (Next major, row 11): Junicode, and EB Garamond
    after a licence read; accepting the change to default pictures.
23. **Windows and Linux hyphenation** (Next major, row 12): a dictionary's
    size and licence, as a Go module or as data.
24. **Toolkit and toolchain** (Next major, row 13, and *Outside the
    backlog*): whether the major carries the Fyne 2.8 port or the fork
    switch, and whether oto 3.5 (Go 1.25) goes with it. These are the only
    pieces that keep a branch.

### Whenever suits (they stay in the backlog until then)

25. **The release pipeline** (Stays, row 1): the Play upload key and service
    account in the repository's secrets (Play from CI); whether to skip the
    local mirror of CI before a push.
26. **`bibletext:` links in the direct downloads** (Stays, row 5): register
    silently, by a Settings switch, or not at all.
27. **Age ratings** (Stays, row 6): how to answer IARC's questions for a work
    of literature.
28. **The 9–12 audience** (Stays, row 7): signing the COPPA/GDPR
    certification.
29. **Short desktop windows** (Stays, row 8): raise the minimum height, or let
    a sheet cover the header; and the iPad Settings footer's separator.
30. **Several open note bubbles at once** (Stays, row 10): yes or no.
31. **A setting to hide section headings** (Stays, row 13): yes or no, and its
    default.
32. **BSB Zechariah 12:1** (Stays, row 14): move the oracle title out of the
    verse, or keep it.
33. **Version states** (Stays, row 19): whether a tap on the checked
    translation accepts a substitution; whether a launch falls back to a
    translation whose canon holds the saved place.
34. **The Snap Store and AppImageHub** (Stays, row 16): categories and
    screenshots in the Snap dashboard, and the AppImageHub PR.
35. **Snap preferences** (Stays, row 17): move them to `$SNAP_USER_COMMON`
    with a one-time copy, or leave them.
36. **The repository's GitHub branches:** the merged branch
    `publisher-paragraphs-only` is still on GitHub; deleting it is a push,
    so it waits for an OK.
