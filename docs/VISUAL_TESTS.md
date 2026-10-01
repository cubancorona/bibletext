# Visual test playbook — shared notes, run on every platform

Host tests prove the model; these are the checks only a screen can answer.
Run the whole list on a surface before shipping a change that touches note
chrome, bands, arrivals, or the reader layout — and run the single sharpest
fixture (V1) on EVERY surface, because each defect in the history below was
invisible on the platform it was not found on.

## The surfaces, and how to drive each

| Surface | Run | Fixture route |
|---|---|---|
| iOS simulator | `scripts/run-ios-sim.sh --dev`, then `SIMCTL_CHILD_BIBLETEXT_DEV_NOTES=s12pills xcrun simctl launch <udid> uk.co.bibletext` | the scenario, or the dev Links tab |
| iPhone | `scripts/run-ios-device.sh --dev` | the dev Links tab |
| Android emulator | `BT_ANDROID_TAGS=bibletextdev scripts/build-android.sh`, `adb install -r cmd/mobile/BibleText.apk` | the dev Links tab; share links via `adb shell am start -a android.intent.action.VIEW -d "<url>"` (HOME between links — a foregrounded activity swallows repeats) |
| macOS native | `go run -tags bibletextdev ./cmd/bibletext` | the dev Links tab |
| Styled pane (Windows/Linux) | `BIBLETEXT_MIMIC=linux go run -tags bibletextdev ./cmd/bibletext` (or `windows`) | the dev Links tab |
| Web reader | `go run ./cmd/websitegen -out build/site -offline`, serve `build/site` | mint links with `ShareLinkURLWithNote` (a throwaway test printing them) |

More drivers for the sections below: `SIMCTL_CHILD_BIBLETEXT_DEV_OPEN=<name>`
auto-opens a sheet on the simulator for screenshots. The names cover every
sheet a phone or tablet opens over the page, in its tallest and shortest
forms and its waiting states: `settings`, `goto`, `chapters`, `versions`,
`versions-more`, `votd` (today's), `votd-one`, `votd-long`, `audio`, `note`,
`ask`, `ai-waiting`, `xrefs-waiting`, `xrefs`, `share-image`, `note-offer`,
`link-notice`, `version-loading` and `version-error` (`devSheets` in
`dev_open_sheets_on.go` says what each opens, and how the waiting states are
held without a download or an AI request).
`SIMCTL_CHILD_BIBLETEXT_DEV_TAB=read|books|search` brings the launch up on
that tab (`dev_open_tab_on.go`), so Books and Search, where a phone held
sideways keeps its header and navigation, can be captured without a tap; a
sheet named with it opens over that tab. `scripts/run-ios-sim.sh` forwards
both. `BIBLETEXT_ENABLE_TESTING=1`
unlocks placeholder translations (header grows a TESTING badge — its absence in
a release-default run is itself a check); `BIBLETEXT_DESKTOP_TABS=rail|bar|sidebar`
switches the desktop layout and announces the choice on stderr;
`BT_SCROLL_DEBUG=1`, `BT_SHEET_DEBUG=1` and `BIBLETEXT_DEBUG_READALONG=1`
narrate scroll landings, sheet lifecycle and read-along arming when a check
disputes what happened. The dev Links tab carries the full share-link scenario
inventory (~33 rows), the version-cache panel, the note seeds and the
highlight colour lab.

The one scenario that exercises most of the NOTES list at once: **s12pills**
(dev builds) — four received notes on four John 11 paragraphs plus a
chapter-scope note, collapsed, then a plain re-entry. The dev tab's
**SPREAD 1–4 of 4** rows are the same fixture by hand (John 3), and
**"Seed 3 of MY notes"** exists to prove own notes DON'T join the counts.
Wipe the store first ("Delete all stored notes") — stale notes from earlier
sessions change every count and every band, and an evening was once lost to
exactly that.

The one scenario for the READING PANE'S AIR: **headnote** (dev builds) — a
received note on BSB John 11:17, the paragraph the section heading "Jesus
Comforts Martha and Mary" opens, so one frame carries the paragraph before,
the heading with its lead and tail, the note band and the noted paragraph:
every vertical quantity `reading_spacing.go` governs, beside the note band,
whose numbers live in the spec table in `notes_bubble.go`. The arrival pins the band to the top of the pane on every
surface, so scroll up a few lines for the picture (the styled pane and the
natives all take a real wheel or drag; the armed restore is not enough).
Run it on every surface after touching spacing, and read it against the
"Vertical spacing of the reading pane" table in `docs/READING_TYPOGRAPHY.md`.
On Android and the web the same fixture is the link
`ShareLinkURLWithNote("bsb", "John", 11, 17, 17, …)` opened by `am start`
or in the browser.

Its twin for the WASH around a heading: **headwash** (dev builds). A note on
BSB John 11:16, the verse just ABOVE that heading, which `headnote` cannot
show because its note is below it. Three frames on a fixed clock: the note's
wash on v16 (about 5 s), the narration on v16 (about 11 s), and a second link
marking v16-17 across the heading (about 21 s). In all three the heading
stands on plain paper — a verse's range ends where a heading begins
(`btIOSBuildVerseIndex`, `btMacReadAlongRange`, Android's
`endBeforeHeading`) — and a tap on it is not a tap on the mark. Launched
with the app on another translation (`BIBLETEXT_DEV_SWITCH=web` first), the
first link is also the arrival that switches translation, which is the route
on which a deferred re-assert used to pin the view to the top: with
`BT_SCROLL_DEBUG=1` the trace must read "landed on highlight" and then only
"re-assert: nothing to place", never "pinned to TOP" after the landing.

## V1 — the collapsed state (pills), the sharpest fixture

Store: 4 received notes on 4 paragraphs + 1 chapter-scope, all minimized.

- [ ] Every noted paragraph carries a pill at its own reservation, and
      NO reserved band is empty (an empty band = a placement bug, the iOS
      inset-hijack shape).
- [ ] Mid-chapter pills read CENTERED between the paragraphs on layouts with
      a paragraph separator (phones, narrow styled, narrow web): the air
      above ≈ the air below (the centering rule, notePillSeparatorLift —
      the pill rises half the separator above its band top). Under a
      section heading or a psalm title the separator is the heading's tail
      or the title's gap, on every layout including the reporter ones, and
      the pill centres in that (the `headnote` fixture, minimized) — except
      Android's compact page, whose import has no separator line to measure
      and stands the pill down there (docs/BACKLOG.md). On
      reporter layouts (iPad/macOS/wide) a pill under a plain paragraph has
      not moved: GapAbove into the band as always. The OPEN card never
      centres — its tail stays the pinned 10 above the passage.
- [ ] The chapter top carries the co-tenant STACK: the chapter-scope band
      and the first paragraph's own, two pills, fully visible, not
      overlapping, not clipped.
- [ ] Labels are per-group counts ("Note", "Notes · 2") — never the
      chapter-wide total on a paragraph pill.
- [ ] No chapter-wide sticker anywhere (the stand-down): the pills replace
      it, and drawing both counts the same notes twice.
- [ ] Pill x aligns with the TEXT COLUMN's left edge (the sticker's own x).
      Check on iPad / a wide window especially: the column centres, and a
      fixed pad drew pills in the margin twice (macOS, then iOS).
- [ ] Scroll the whole chapter down and back: pills stay in their bands
      through reflow (the frame path once re-placed the sticker but never
      the pills).
- [ ] Rotate (or resize): everything re-places.
- [ ] Dev toggle "Pill per paragraph" OFF: single sticker returns, chips
      gone, no stale reservation left behind (the empty push clears).

## V2 — the expanded card

- [ ] A note with a verse: card above ITS paragraph, speech tail pointing
      down at the passage, wash on exactly the note's verses (a range note
      washes the whole range and stops).
- [ ] A chapter-scope note (no verse): card parks at the CHAPTER TOP with
      NO tail — a tail there claims verse 1 (the web shipped this until
      step 9).
- [ ] An open RECEIVED card shows NO pills anywhere — its who line ("· 2 of
      5 in this chapter ›") is the set's one representation, and pills
      beside it would say it twice.
- [ ] An open OWN card keeps the pills (your card carries no count of the
      friends' set — without them it is represented nowhere); one sharing
      the card's paragraph stacks ABOVE it.
- [ ] The who line: byline, counts, chevron when the counts are a control;
      tapping the counts cycles the set, wash moving with it.
- [ ] Verbs by ownership: a received note carries − and the bin; your own
      note carries ✕ alone, and its who row reserves ONE slot's width.
- [ ] The unplaced arm: a note this translation cannot place reads
      "… not shown here" and rides the chapter top; no tail, no wash.

## V3 — arrivals (where the view lands)

- [ ] A note LINK: lands with the band in view — the card (or its pill),
      not the bare verse with the message above the fold.
- [ ] A link to another verse of the note's own paragraph: still the band
      (the Android same-VERSE reading failed exactly this).
- [ ] A keyed pill tap: THAT paragraph's note opens and arrives in view
      (the group's own, never the plan's default choice).
- [ ] PLAIN entry (chapter arrows / the strip) into a noted chapter the
      session has not visited: the chapter opens AT THE TOP. No drag to
      any pill. (The rule this playbook exists for.)
- [ ] Returning to a chapter you left mid-read: where you left off — the
      restore outranks everything but an explicit arrival. KNOWN POLISH: a
      saved offset inside the top band region shows a chopped pill.
- [ ] A restore from the chapter pill with the note far below (s11pill):
      the bubble arrives IN VIEW.

## V4 — verb round-trips

- [ ] Minimize → the paragraph pills appear (or the single pill, gate
      off); restore → the card returns, wash returns.
- [ ] Delete → card and wash both go; on the WEB the fragment is stripped
      so reload cannot resurrect it.
- [ ] Notes off (Settings, "keep them") → everything stands down, nothing
      deleted; on → it all comes back.

## V5 — cross-translation

- [ ] Switch translation with a note live: it carries, renumbered; the
      wash follows (the doxology case if in doubt: Romans 16:25 ↔ 14:24).
- [ ] A note from ANOTHER translation wears its version chip.
- [ ] The web reader: the version pills carry `v` and `n` in their hrefs.

## V6 — theming and text

- [ ] Dark and light both: card surface, border, tail, wash, pill chrome
      all from the palette; red letters legible over the wash.
- [ ] Emoji and accents in a note body render as text (no tofu, band
      height correct); markup in a note appears LITERALLY.
- [ ] A 280-rune note: tallest card, nothing underneath obscured.

## V7 — the web reader's own five

- [ ] Byline "Note from Friend", pill label "Note" (emitted, not typed).
- [ ] Anchorless card tail-free; versed card tailed.
- [ ] One verb vocabulary (Minimize/Delete) in card AND tap bubble.
- [ ] Corrupt payload → the quiet notice, passage still opens.
- [ ] NKJV notice page with a note link: the note still renders above the
      get-the-app offer.

## V8 — reading surface and typography

- [ ] Prose justifies with flush right edges (iOS, macOS, Android, web, and
      the Windows and Linux pane); the legacy Entry pane never justifies — an
      EXPECTED difference, not a bug. Poetry is always ragged, and so is a
      paragraph's last line. On the Windows and Linux pane, highlight a verse
      and drag a selection across a justified line: the wash and the selection
      cover the spread words and their gaps, with no word outside them (dev
      build: toggle "Justify the Windows and Linux pane" on the Links tab to
      see the same chapter ragged; `readingJustifyProse`).
- [ ] Psalm 23: two poem lines per verse, breaks at every verse boundary
      inside the poem; a width-wrapped poem line continues flush left (no
      hanging indent exists anywhere, by design). Copy a wrapped psalm and
      paste it: authored breaks survive, wrap points flatten to spaces.
- [ ] Red letter: BSB John 4:9 — the Samaritan woman's words must NOT be
      red (the recorded web failure); mixed verses go red only from the
      quote. Check both themes, and over a highlight wash.
- [ ] A wash never re-typesets: arrive on a search hit, clear it — the
      paragraph must not reflow (bold-Georgia rewrap was a live defect);
      no pale notch at verse joins mid-line; a washed verse's NUMBER is
      washed too.
- [ ] iOS: long-press a word inside a washed verse — the selection tint
      shows OVER the wash (sampled on the simulator: light #CCC490, dark
      #263E82; no #FFE08A-family pixel inside the selected word), the
      unselected washed lines stay exactly #FFE08A (dark #3A326F), an
      unwashed control word selects to #BECBD4 on light paper, the band's
      per-line extents are unchanged, no seam at verse-number joins or line
      ends, and Copy / Study with AI / Share stay up. Clear the wash while
      the selection is live — the menu must NOT dismiss and the band must
      vanish with no cross-fade. Flip the appearance with the chapter open:
      the band re-renders in the other theme's colour with no stale band.
- [ ] Android: the same long-press inside a washed verse — the selection
      lightens the word over the wash (dark wash #3A326F reads ≈ #5D5584
      under the selection; an unwashed word lightens by about the same);
      the wash itself is unchanged in colour and shape, with no hairline at
      the joins between a verse number, its body and the join space.
- [ ] Reporter layout (iPad, macOS, wide styled pane, web ≥46rem): centred
      ~59-char column, 1.3 leading, first-line indents, no paragraph gaps —
      and a poetry-opening paragraph takes no indent. Phones keep the airy
      2.0 leading with gaps. Dragging a desktop window across the width
      gate glides between the two without a jump to the top.
- [ ] Psalm superscription (Psalm 3): italic, unnumbered, present with
      footnotes OFF; a selection straddling title and verse 1 cites verse 1
      only (title words once leaked into a citation).
- [ ] Text size Normal → XL: scripture re-renders on sheet close, position
      held by verse anchor; reporter column widens with the scale; the
      radio stacks on narrow phones with no clipped "Extra l…"; note bands
      re-measure. XL + read-along is the recorded still-open eyeball.
- [ ] Selection menus per platform: exactly ONE Share; "Study with AI" only
      with an assistant chosen; Cross-references takes the study slot when
      AI is off; a selection inside the footnote section gets system verbs
      only. Android's toolbar must appear IMMEDIATELY on long-press.

## V9 — audio and read-along

- [ ] The speaker expands in place to the card WITHOUT changing header
      height (the card once drew past its hit rect — visible ▶ hit
      skip-back); "Read aloud ▾" must not slide under the ✕.
- [ ] Recorded streams show a buffering spinner until sound is audible;
      taps during buffering are ignored.
- [ ] The amber narration wash tracks the voice verse-by-verse; nothing
      washes during a recording's intro; on the styled pane it is per-line
      rects bounded to the verse, never a full-column band. A silent
      dropped-to-TTS stream looks exactly like "highlighting is broken" —
      BIBLETEXT_DEBUG_READALONG=1 tells them apart.
- [ ] Follow scroll obeys the comfort band (only scrolls when the verse
      drifts above the top or below 70%, lands 30% down); a hand scroll
      raises the "Follow narration" pill once, the wash keeps tracking,
      the pill snaps back on tap and never outlives playback.
- [ ] Walking the narration over a noted/washed verse leaves that wash
      INTACT after the voice moves on (the narration once deleted the
      note's mark on exit).
- [ ] Chapter end rolls to the next chapter — across book boundaries, pane
      following, same source; WEBC Daniel 12 → 13 must hand over to the
      synthetic voice, never silent TTS; playback stops at Revelation 22
      without a parked lock-screen card.
- [ ] Source menu: a machine voice must NEVER wear the person glyph or the
      word "Narrator"; selecting a source only chooses — Play starts it.
- [ ] Lock screen: iOS/macOS card shows the share-image-style artwork,
      ±15s skips (never next-track); Android has the MediaStyle scrubber
      for recordings and deliberately NO scrubber for TTS; swiping the app
      from recents stops playback. Rotating Android mid-narration restores
      wash AND pill (a rotation keeps the activity: the same instance and
      process on the Android 15 emulator, 27 September 2026).
- [ ] Android, narration playing: with Developer options > "Don't keep
      activities" on, press Home mid-narration and wait. Note
      whether the narration keeps playing and whether the transport, wash
      and pill say what is true. From the code, the destroyed activity's
      stop ends the narration and leaves the controller showing it
      playing (docs/BACKLOG.md, "Android: an activity destroyed with the
      process alive stops the narration", open).
- [ ] Theme flip mid-play must NOT stop audio; navigation away does.

## V10 — search and the AI assistant

- [ ] Empty state is the calm centred prompt, never `Results for ""` over
      an empty box. Results update only after the typing pause and always
      match the FINAL text — after Enter, results must never flicker back
      to a prefix's matches (the pinned debounce race).
- [ ] The whole result row is one tap target (desktop adds hover wash);
      tapping the SAME result twice still snaps the view; the keyboard
      drops; no ghost pixels of the search field linger in the header.
- [ ] Search under WEBC, switch to WEB, tap a stale Tobit hit: the "Not in
      this translation" notice — never a blank chapter with dead arrows.
- [ ] The results trail is ONE compact row (`‹ Results … ✕`); both exits
      clear the wash AND re-raise a suppressed note's own wash; verse-of-
      day and Go-to arrivals show NO trail.
- [ ] Exactly one of Search|Find|Notes-bubble is lit at any time; the pair
      vanishes with Assistant=None, the bubble with notes off.
- [ ] Find: Cancel appears on the FIRST search of a session; cancelling
      says "Search cancelled." — never "no matching passages"; the
      faster-model offer appears only when something faster exists; every
      teardown route (✕, toggle, sidebar collapse, Assistant→None) kills
      the in-flight state — no immortal "Searching with AI…".
- [ ] The AI answer panel: quote ellipsizes, Cancel exists while thinking,
      truncated answers carry the honest cut-off line, the reading text
      never paints THROUGH the panel, and a no-key error offers settings.
- [ ] Every AI wait names the model at work, on all five platforms: the
      Study with AI panel and Find show one line under the bar, over
      "Capable models can take a minute or more.", in that hint's size and
      muted colour and centred on its axis — `Gemini (Google) ·
      gemini-pro-latest`, the Settings pickers' own names; Settings → Test
      key shows the same line under "Testing…". It fills a moment after the
      wait appears and moves nothing when it does. Pin a model in Settings:
      the line names the pinned model. Tap "Switch to a faster model": the
      line changes to the fast model. With `BIBLETEXT_DESKTOP_TABS=sidebar`,
      the sidebar's Find wait carries it too.
- [ ] The model line on a small screen: pin a long model id (a Gemini
      preview); on a 375pt phone it wraps to a second line and never widens
      the card or the Search tab. On a phone in landscape (iPhone SE, a Pro
      Max, an Android phone) the Study wait still scrolls to Cancel with the
      wrapped line, and nothing paints past the card or over its footer.
      The empty line, before the name arrives, sits as close under the bar
      and over the hint as the name will, and the name moves nothing.
- [ ] Find on a phone in landscape (iPhone SE, an iPhone Pro Max and an
      Android phone, each with the rail): ask a Find, then turn
      the phone. "Searching with AI…" and the bar sit at the top of the
      results area, just under the Find field, never under it; the rest
      scrolls, and a drag reaches Cancel and "Switch to a faster model"
      clear of the tab bar, with the wrapped long model id too. Cancel
      stops the Find. Back in portrait the wait is centred as before, with
      nothing to scroll.
- [ ] Find mid-flight: flip light/dark while it searches — the rebuilt tab
      still names the model; ask again while it searches — the new search
      names its own model and its answer lands (it used to stay "Searching
      with AI…" until Cancel). The retry onto a self-healed replacement
      needs a retired default to show on a device; the host tests cover it.
- [ ] Assistant=None leaves no stale AI surface: results pane reverts to
      keyword, notes bubble survives alone.

## V11 — navigation chrome

- [ ] Go-to picker: book/chapter taps only SELECT (grid repopulates in
      place, no jump); only Go commits; junk verse falls back to chapter
      top with no error UI. The chapter-picker flavour (tap the heading)
      navigates immediately on a chapter tap.
- [ ] Mobile keyboard: the verse row lifts to sit exactly above the
      keyboard (Android once overstated the lift ~2.2× in landscape); the
      popup itself never resizes; on a short landscape canvas the panes
      collapse so the verse row stays visible; rotation refits the card
      (the Go button once hung off-screen).
- [ ] Arrows disable (faint, tap-inert) at book ends and never cross
      books; only audio auto-advance does.
- [ ] History strip: current chapter never appears in it; horizontal
      scroll, never wraps; tapping a chapter returns to WHERE YOU WERE;
      re-adding the playing chapter must not stop audio; the bin clears it
      durably across relaunch.
- [ ] D16: read Tobit in WEBC, switch to WEB — the deuterocanon entries
      VANISH from the strip (not greyed); switching back restores them.
- [ ] Version switch mid-chapter: same book+chapter stay open, an open
      note keeps focus, any wash renumbers or drops (never the wrong verse
      lit), on-screen search results re-run under the new translation.
- [ ] Books grid: width buys columns (capped ~5); typing in the filter
      hides the testament headings; the WEBC 73-book canon groups
      correctly; the downloading banner shows on a fresh install.
- [ ] Loading screen: exactly ONE spinner ever animates — flip the OS
      theme during the load and confirm scrolling stays smooth afterwards
      (the orphaned-bar 20fps repaint was a live defect); the first-run
      progress line actually advances per book.
- [ ] Landscape, on Books and Search: every phone and tablet moves the bar
      to a left rail, the iPhone included, and back to the bar upright. On
      an iPhone the rail stands beside the Dynamic Island, not under it,
      with the phone turned either way; on an Android phone beside its
      cutout or side navigation bar. On the Read tab a phone reads
      full-screen instead (the V16 row); raising the soft keyboard must NOT
      flip the layout (the 3,000-rebuilds/min trap).

## V12 — sharing and links

- [ ] Share with citation: quote keeps authored poetry breaks; the
      citation spells the version in full. On Linux the styled pane copies
      to the clipboard and opens the "Copied — ready to paste" sheet (the
      row after next) in place of a system sheet: divergence 27 in
      docs/PLATFORM_MATRIX.md, the recorded counterpart there. On Windows
      it opens the Windows Share sheet (the next row).
- [ ] The Windows Share sheet, from a Windows build of the app (the
      direct-download zip, and the Store's MSIX where it can be installed),
      in light and in dark: Share with citation, Share as link, Share with
      note, the verse of the day's Share icon and the image preview's Share
      each open the system Share sheet over the window, and nothing opens
      in the app. Share as link and Share with note: the sheet's header
      reads "Share link", with the citation, the link, a QR-code button and
      Copy link (on Windows 11 a link icon with no label); Copy link puts
      the link on the clipboard. Share with citation and the verse
      of the day: the sheet lists the apps to share to; pick a mail app
      where one is set up and check the message carries the quote and its
      citation, and for a note the note, the citation and the link — and
      whether the link then appears twice. Share as image: the sheet shows
      the card's thumbnail under a name of the form "BibleText verse
      2026-09-30 14.02.11.png", and Copy copies the picture. Share with
      note: the note card is on the page behind the sheet. Cancel each
      sheet with its X (on Windows 11 an Escape sent to it left it up):
      nothing else opens, and the next share opens its sheet again.
      Share again while a sheet is open and note what Windows does with the
      second: it ends in a sheet, the system's or the app's, and never in
      nothing. The sheet may not open where Windows
      refuses it — then the in-app sheet (the next row) opens instead, at
      once or within five seconds, never nothing; the app's log names the
      step and whether the build was packaged. Should the system sheet
      still appear after the in-app one has opened, it carries the share,
      not an empty package. Run in part on 30 September 2026, on Windows 11
      arm64 with the arm64 Store package's executable run unpackaged, in
      dark only (the Windows proof note under Sharing in
      docs/PLATFORM_MATRIX.md), and again on 1 October 2026 with the arm64
      package registered with package identity, also in dark; still to
      run: light, the package as the Store installs it, x64, a mail app,
      the clipboard after Copy link, the picture's Copy, and a share while
      a sheet is open.
- [ ] The desktop share confirmation, Linux, and Windows where its Share
      sheet cannot open: Share with
      citation, Share as link, Share with note and the verse of the day's
      Share icon each end in a modal sheet headed "Copied — ready to
      paste", with the verb's own line beneath it, the exact clipboard text
      in a bordered box (a long note scrolls), and Email…, Copy again and
      Done, in that order. A click on the page around it does nothing;
      Done, Escape and Return each close it, Return after Copy again too,
      and Escape and Return still do after a drag across the box. Press
      Tab: the caret goes to Email…, then Copy again, then Done; with it on
      any of them Escape and Return still close the sheet, and Space
      presses that button. Put something else on the clipboard, press Copy
      again: the share is back on the clipboard and the line reads "Copied
      again." for a moment, and neither the box nor a button moves while
      it does. Email… appears a moment after the sheet; Copy again and Done
      do not move when it does. At the 520-pt minimum window and at
      1280x800 the sheet sits inside the window and below the header, and
      stays so when the window is resized under it. Flip light/dark with
      it open: it comes back in the new palette with the same text; opened
      from the verse of the day, the card comes back beneath it and Done
      returns to the card, which Escape then closes. Share as image: the PNG
      lands in Downloads, the file manager opens on it, and the sheet reads
      "Picture saved" with no box and no Copy again, and comes back after a
      light/dark flip; with no Downloads folder it lands in the home and the
      line says only that the picture is shown in the file manager. In the
      snap: the PNG lands in the reader's own ~/Downloads, not under
      ~/snap/bibletext, the file manager opens on it selected, and the sheet
      says so; with ~/.config/user-dirs.dirs naming a localised download
      folder it lands there and the line says only that it is shown; with no
      Downloads folder it lands in the home and the line says only that it
      is shown; with the home plug disconnected and ~/snap/bibletext/common
      made read-only, nothing is saved and no sheet opens (then undo both);
      and where the file manager does not open (the portal refusing:
      gsettings org.gnome.desktop.lockdown disable-application-handlers
      true, then reset) the line says only where the picture is saved.
      Outside the snap, with no xdg-open on the PATH, the line says only
      that it is saved in Downloads, or, with no Downloads folder, names the
      home. With ~/.config/user-dirs.dirs moved aside and a named pipe
      (mkfifo) in its place just before Share, the window still answers
      (Books opens) while the save waits; writing the file into the pipe
      lets the picture land and the sheet open; then put the file back
      (docs/BACKLOG.md, the Linux and Windows share-sheet entry).
      Email… on the text
      sheets: the button shows whenever the desktop names a mailto:
      handler, and a browser counts — a stock Ubuntu desktop names the
      Firefox snap, so the button shows there with no mail client
      installed, and pressing it opens the mailto: link in Firefox, which
      asks what should handle it. With no handler named, or nothing to ask,
      the button is absent; in the snap on a portal older than 1.19.1 it is
      absent too. Email… on the image sheet: Linux only, outside the snap,
      and only when the handler is a mail client — absent with Firefox as
      the handler. With a mail client, a new message opens carrying the
      citation as its subject and the text as its body — for the picture,
      the quote and its citation, and the picture as an attachment. On
      Windows, a whole-chapter share's Email… opens a message whose text
      is cut at a word with an ellipsis, the citation whole beneath it.
- [ ] Share as image: the preview modal appears BEFORE anything leaves the
      app; Regenerate cycles schemes; the native overlay must not paint
      over the modal; the same verse always opens on the same look.
- [ ] Share with note: the counter wraps rather than pushing Share off a
      narrow screen; past 280 runes it says "Too long by N"; sharing the
      same words twice dedups — your own second link shows YOUR note,
      never "Note from Friend".
- [ ] Share with note, the selected words: under the reference, the words
      in the reading face, small and muted, with no verse numbers. In the
      NKJV select Psalm 23:1-2: "Lᴏʀᴅ" is drawn in real small capitals, no
      boxes and no fallback face. A long selection stops after three rows
      with "…", on a phone in landscape after one. iOS, in portrait, in
      landscape and with the keyboard up in each: the native field's outline
      sits below the last row, never over it, and lines up with the gap the
      sheet leaves for it; in landscape the typed note is still visible
      above the keyboard. Android: the same, with the Fyne field. Android,
      with the keyboard up: Share and then Back out of the chooser, and
      Cancel, each leave the page whole — the reading text fills the pane
      and the tab bar sits at the foot, not mid-screen with the bottom
      third empty (the Fyne driver's inset read raced the keyboard's
      departure); Back to put the keyboard away and then Cancel is the
      same. Desktop:
      the card ends under the buttons with its usual padding and no empty
      row. Flip light/dark with the composer open: the words come back in
      the new palette. Select a heading on its own, one in the middle of a
      chapter and one at its top (most NKJV chapters open with one), and
      then the heading with the number of the verse under it: each time
      the sheet shows the reference to the verse beneath the heading and no
      words. Double-tap (desktop: double-click) a verse number: the sheet
      shows that verse's reference and no words, not the digits.
- [ ] Share with note shows the note it kept, on every surface (iPhone,
      iPad, Android, macOS, the styled pane): select two verses, Share with
      note, write a line, Share. Behind the share sheet (on Linux, under
      the "Copied — ready to paste" sheet; on Windows, under its Share
      sheet) the page now shows the note's card over the selected verses,
      "Note from you" with one ✕ and no −, the verses washed, the view
      placed on it; Return in the desktop
      field does the same. Cancel the share sheet: the card stays, and the
      note is in the notes browser. On an iPad and on a Mac the share sheet
      opens beside the selection, not mid-page, and the card appearing
      under it does not move it. On an iPhone, an iPad and a Mac the text
      does not flash, jump or show at a wrong size as the card appears: the
      Apple panes rebuild the reading pane and re-import the chapter for it.
      Arrive on a verse through Results, select another, send: the search
      wash goes and the card opens, never the pill. Flip light/dark with the
      share sheet still up: the card comes back in the new palette. Flip
      light/dark with the composer open (on iOS, the native field showing),
      then Share: the card appears as it does without the flip. Go to the
      next chapter and back: the card is gone and stays gone. Android: pick
      an app in the chooser, send, come back: the card is still there.
      Share with the field left empty: no card, and a search wash the
      reader arrived on stays lit.
- [ ] Share with note, then a link: select two verses, Share with note,
      start typing, and open a BibleText link to another book from Messages
      (desktop: from the browser or by pasting it) without closing the
      composer. The reader moves to the linked passage and the composer
      stays up with what was typed and the first passage's words and
      reference. Share: the message cites the first passage and its link
      opens the first passage at the selected verses; the notes browser
      lists the note under the first passage, and opening it from there
      goes there. No card is drawn on the linked passage. iPhone with the
      native field, Android with the Fyne field, iPad, Mac, Windows and
      Linux.
- [ ] Share with note on iPad, at a third of an 11-inch iPad in Split View
      and in Slide Over (320pt wide), with a verse of about eighty
      characters selected: the native field's outline sits below the third
      row. Then open the composer at half width, type a line, and drag the
      divider to a third, with the keyboard down and again with it up:
      within a moment the card, the words and the field fit the narrower
      window, Share is on screen, the field sits below the last row and the
      typed line is kept; drag back to half and it widens again. With the
      keyboard up, dismiss it after the resize: the card still reaches the
      bottom of the window, with no band of the page showing beneath it.
      Android: the same in a split-screen or freeform window, with the Fyne
      field.
- [ ] Add a note on a Mac, Windows or Linux window dragged as short as it
      goes (about 386pt of content), with two or three verses selected: the
      card's top edge stands at the header's lower edge, never cutting the
      Go to chip, and the selected words take one row; drag the window to
      about 440pt and the words take two rows with the card wholly below
      the header; at the launch size they take three. With the card open,
      drag the height down and back up: the rows go and come back.
- [ ] Inbound links per platform: iOS/Android land with the band in view;
      NON-claimed URLs (/web/john/ index, /privacy.html) fall through to
      the browser — the app must never just foreground; macOS store build
      claims links (watch the silent delegate loss after a GLFW bump);
      Windows/Linux/unsigned-macOS paste-into-search must arrive LIT (the
      wiped-mark regression).
- [ ] Notes-off offer card: buttons STACK on narrow phones (the right
      button once painted on top of the left); plain links never raise it.
- [ ] NKJV link without a key: passage opens in the current translation
      FIRST, then the "Shared in New King James Version" card OVER it.
- [ ] Seed-install parking: a link to an undownloaded book parks with the
      honest sentence and opens BY ITSELF when the download lands; a
      second link replaces the first with the replaced wording.
- [ ] Cold-start link: exactly ONE rebuild landing on the shared chapter —
      no flash of last session's chapter, and the saved position must not
      steal the scroll afterwards.
- [ ] Hostile rows (dev tab 22-30): markup renders literally, bidi
      overrides stripped, unknown fragment keys ignored, wrong host
      declined to the browser.

## V13 — footnotes and cross-references

- [ ] No in-text markers, ever: with the toggle on, the text above the
      rule is pixel-identical to toggle-off; copy/share/TTS never carry
      note words.
- [ ] The chapter-bottom section: continuous hairline rule (gapped dashes
      on iOS was the measured quirk), semibold verse keys, muted smaller
      justified notes (ragged on the Windows and Linux pane, a known
      difference: docs/BACKLOG.md, `readingJustifyProse`). Drive with BSB Ezekiel 40 (25 notes) or WEBC
      2 Maccabees 12 (26). A note-free chapter shows nothing at all.
- [ ] Omitted-verse orphans: WEB Luke 17 reads "…35 … 37…" with no 36 in
      the body, and "36 Some Greek manuscripts add…" sits between 35's and
      37's notes below the rule.
- [ ] Selection clamps at the rule: app verbs never appear on apparatus
      text; Android's system Copy/Select-all must still work there (their
      silent death was a found failure).
- [ ] At every text size, verse navigation/read-along must never jump to a
      section KEY as if it were a verse; the last verse's narration tint
      ends at the rule.
- [ ] NKJV stays dark: no rule, no section, no crossref rows on any NKJV
      chapter — the Settings card still present.
- [ ] Cross-references panel: loading bar STOPS on close (a leaked
      infinite bar once pinned the canvas at ~20fps); Gospel selections
      show embedded PARALLEL rows even offline; rows never display a verse
      number the WEB lacks (the doxology's 92 rows are the probe).

## V14 — the notes browser and scrapbook

- [ ] Row anatomy: bold accent reference, "(WEB)" abbrev, stamp with the
      date grammar (Today/Yesterday/N days ago/2 Jan), the SAME tailed
      bubble the reading page draws, preview capped at 4 lines/220 runes.
- [ ] Density: roughly twice the rows per screen of the old layout, on
      desktop too; the bin clears the hover-expanded scrollbar; wide panes
      centre the list in a readable column.
- [ ] A row tap lands ON the note, open — even one stored minimized, even
      over a leftover search wash, with NO results trail. Psalm 119:105
      (dev row) is the sharpest check.
- [ ] Deleting the OPEN note from the list: returning to Read must show
      the passage with its remaining pills — never a stranger's note
      expanded in its place (the measured focus-fell-through defect).
- [ ] Own notes: never drawn in scripture until their row is tapped,
      their own link is opened or they are sent (V12); byline "From you";
      the bubble carries ONE ✕ (no −); ✕ dismisses without touching the
      store; only the browser's bin deletes.
- [ ] The S10 count-tap (“1 of 3 ›”): same-range trio swaps bubbles with
      the wash NEVER moving; different-range trio moves the wash within
      the paragraph; the view repositions only when the anchor changes.
- [ ] Sort persists across launch; Bible order sends canon-less books to
      the end, still openable; the WHO filter appears only once you have
      own notes.
- [ ] Scroll position survives opening a row, sort flips and theme
      changes; refilled rows keep NOTHING from the previous note.

## V15 — settings and theming

- [ ] The sheet: title and footer pinned, body scrolls, ✕ always
      reachable; outside taps do nothing (modal by design); cards read as
      raised cards, not giant text fields.
- [ ] Assistant→None: key form swaps to the one caption, sheet refits (the
      stale-tap-boundary phantom-dismiss was live), Find pair disappears
      on close, native menus lose Study-with-AI immediately, keys are
      KEPT for the way back.
- [ ] Key rows: auto-save wording (Keychain on Apple, on-device
      elsewhere); Paste/Test/Clear in that order everywhere; the bundled
      API.Bible key must NEVER show its characters behind the reveal eye;
      Clear works even when the field shows the bundled placeholder.
- [ ] The model picker is a floating sheet with margins — never a
      full-screen takeover; provider A's slow model list must never fill
      provider B's dropdown.
- [ ] Notes off with notes stored: Cancel restores the checkbox; Keep
      preserves the store and re-enabling re-projects the chapter's note
      IMMEDIATELY; Delete clears the on-screen note and tint at once (the
      sticker once kept drawing a deleted note).
- [ ] System theme flip mid-chapter: whole palette swaps, viewport stays
      on the same verse. With a sheet open (Go to with a book, chapter and
      verse typed; Verse of the day; Translation; Settings; a note being
      written) the whole app re-lights at once and the sheet comes back in
      the new palette showing the same thing — never a dark card with dark
      letters, never a page half in each theme. Flip from Control Centre
      and by schedule with the app backgrounded. Background the app with a
      sheet open and bring it back WITHOUT changing appearance: the iOS
      app-switcher double snapshot must leave the sheet exactly as it was
      (the Go to picker keeps its typed verse), with no rebuild:
      `BT_SHEET_DEBUG=1` logs one `[sheet] appearance …` line per event —
      exited-foreground, each variant-changed with background=true and
      rebuild=false, then entered-foreground with rebuild=false. On
      Android a change made while away usually logs the other way round:
      entered-foreground with rebuild=false (the return is heard before
      the update), then variant-changed with rebuild=true. When the
      night-mode patch's configuration branch wins its race the update
      comes first and entered-foreground rebuilds instead. One rebuild
      either way, never two.
- [ ] Flip with the keyboard up in a sheet: typing a verse in Go to, the
      number pad goes down with the old sheet and comes back up over the
      reopened one with the caret in the same field; in the note composer
      (iOS: the native field) the note comes back whole and the keyboard
      types into it. Long-press a Go to verse field for Cut/Copy/Paste and
      flip under the menu: the picker comes back, the menu does not.
- [ ] iPhone and iPad, flip with the keyboard up and then put the keyboard
      away: in the note composer (tap the iPad keyboard's dismiss key), in
      Ask (tap a blank part of the card) and in Settings with a key field
      focused (the keyboard goes down with the flip), the reopened sheet
      still reaches the home indicator once the keyboard is down — no page
      showing beneath it, and a tap on the lower part of the screen lands on
      the sheet, not on the page (`sheetArea`). Flip Verse of the day and Go
      to the same way; the Go to card's verse row sits where it did before
      the flip.
- [ ] Settings, a key pasted, Test key, and while "Testing…" shows switch
      the system between light and dark (a Mac: System Settings >
      Appearance; a phone: Control Centre, or a scheduled switch with the
      app in front): Settings comes back at its top still reading
      "Testing…" under the key, the assistant's wait naming the model once
      the request has said, and "✓ Key works." (or the error) lands on the
      sheet that came back. Then the ✕ and Settings again: no line under
      the key. The same under the API.Bible key.
- [ ] Android, API 30+: open Go to, tap the verse field so the number pad
      rises, type a digit, press Back so the pad hides with the field still
      focused, then flip dark mode from the quick-settings tile: the picker
      comes back with the digit and the pad stays down. Flip with the pad
      up instead: it comes back over the reopened picker, as before.
- [ ] iOS and Android, flip with Verse of the day or Go to open: the verses
      must not flash over the sheet for a frame. If they do, the frame is
      the OLD palette's chapter — the rebuild un-suppresses the pane before
      its re-render lands, and the reopened sheet suppresses it again a
      moment later (appearance.go; left open, docs/BACKLOG.md). Likewise
      flip on the Books tab and then tap Read: a frame of the old-palette
      chapter over the new chrome is the same known gap.
- [ ] Flip while "Downloading …" is up (a translation's first download):
      the spinner comes back in the new palette and goes when the
      translation lands.
- [ ] Windows, on Windows 10 22H2 AND on Windows 11: flip Settings >
      Personalisation > Colours > app mode, both ways, with the app open
      beside Settings (unfocused), and again with the app FOCUSED while an
      auto dark-mode switch fires. The title bar follows at once (a black
      bar over the dark page, a white one over parchment), without a
      resize or a click elsewhere, and a focused window's caption stays
      drawn active (an unfocused one inactive); before, it kept the launch
      mode until relaunch. Windows 10 is the one that needs the caption
      repaint (title_bar_windows.go).
- [ ] Windows and Linux, focus mode with a note open as a card: flip both
      ways. The card's fill and border are the new palette's, under the
      new ink — never a pale card under pale text, or a dark one under dark.
      Then drag the window's width back and forth for a few seconds with
      the card open: memory settles, not climbing by a raster per width.
      The Notes list (Search tab, notes bubble) on any platform: every
      bubble's tail is the new palette's.
- [ ] macOS: go to Books, flip, then open and close Settings (gear, then X),
      Verse of the day and Go to. No verses appear over the Books tab. Tap
      Read: the chapter is in the new palette, at the reader's place — not
      at verse 1.
- [ ] macOS, mid-chapter on Read: flip with no sheet up, then flip with
      Settings open and close it. Each time the chapter comes back in the
      new palette at the reader's place (the import lands in the hidden
      pane; the capture reads the clip view, which it keeps). Then scroll
      mid-chapter, go to Books, quit, relaunch: the chapter opens where it
      was left, not at the top.
- [ ] Android, AI on: select verse text, tap Study with AI, and while the
      Explain / Analyze popup is up flip dark mode by schedule or
      `adb shell cmd uimode night yes`: the popup closes; it never floats in
      the old palette over the re-lit page.
- [ ] Android, after the activity is recreated with the process alive —
      turn on Developer options > "Don't keep activities", press Home and
      come back; or, on Android 11 or older,
      press Back out of the app and open it again — flip the theme
      (`adb shell cmd uimode night yes`, then `no`, or by schedule): the
      whole app follows, every time, as it does before any recreation. Do
      it twice quickly and flip again: still followed. Before, the first
      recreation left the app ignoring every later flip for as long as the
      process lived (activity_life.go).
- [ ] Android, the same recreations with a translation's first download
      running (Translation > Berean Standard Bible, "Downloading …" up):
      press Home with "Don't keep activities" on and come back
      after the download has had time to land. The translation opens, no
      spinner is left up, and a later download starts normally. Then pick
      another translation and do it again, twice: the reader stays on the
      one picked, and no error card or spinner comes back. Before, a
      download landing between the two activities left the spinner up for
      good and refused every later download.
- [ ] Android, fresh install offline (the Gospels seed and its banner):
      recreate the activity ("Don't keep activities", Home, back), then go
      online, press Home and come back. The full text
      arrives and the banner goes without a relaunch.
- [ ] Search tab, Find: flip while "Searching with AI…" shows — the spinner
      and Cancel come back, Cancel still stops it, and the answer lands on
      the rebuilt tab. Flip over an error card and over "AI didn't find
      matching passages": each comes back as it was, never as the empty
      prompt.
- [ ] Flip with the caret in the Search, Find, notes or Books filter field
      (no sheet up): on a phone the keyboard goes down and comes back up
      over the rebuilt field with the caret where it was and nothing typed
      lost; on desktop typing carries on. On a phone, type in the Books
      filter, tap Done (Android: Back to close the keyboard), then flip from
      Control Centre or a schedule: the keyboard stays down and the filter
      keeps its text. Scroll the Books grid halfway and flip: it stays
      where it was.
- [ ] Phones, Find: flip mid-Find (and rotate an iPad or Android phone
      mid-Find), then tap Read before the answer lands: nothing keeps
      repainting (no sustained GPU load, scrolling stays smooth).
- [ ] Palette spot-checks in BOTH variants: unchecked boxes visible on
      dark cards, sapphire accent (never stock Fyne blue), no white frame
      around popups, disabled controls quieter but readable, red letters
      legible over the wash.
- [ ] Changing ONLY an AI key must not flicker the reading pane on close;
      red-letter/text-size/footnotes changes refresh reading only; the
      notes/assistant switches rebuild the window once.
- [ ] Settings on every platform, light and dark: "Get a key" (the
      assistant's and API.Bible's) and "Privacy Policy" end in a thin arrow
      in the link's own colour, standing on the baseline — never a blue
      emoji tile. Tapping the arrow opens the page, as the words do; on a
      desktop the pointer turns to a hand over the arrow too. On iOS and
      Android this is the Fyne sheet over the native reading view, so check
      it there, not only on the Mac.
- [ ] Settings on a 13-inch iPad with the included API.Bible key: the
      status reads "✓ Included with BibleText — or paste your own." with
      its full stop and nothing cut; open Settings after the header has
      shown its translation name, as it always has. On a 320pt iPhone SE the
      Gemini hint breaks onto a second line inside the card; switch to each
      assistant and back and the sheet grows and shrinks with its status
      rather than cutting it.
- [ ] The translation picker on a 320pt iPhone SE and a 320dp Android
      phone, while the default translation is updating on a previous
      edition (Settings > nothing to do: the first launch after an update
      shows it) and with the included key: the sheet stands inside the
      screen with a margin on both sides, the NKJV's publisher line, the
      WEBC name and the "Unlocks with your own free API.Bible key" caption
      break onto a second line inside the list, and the sentences under the
      rows end inside the list with their last words. On a 375pt phone and
      wider every row stands on one line as before.
- [ ] Settings on a 320pt iPhone SE and a 375pt iPhone (an SE of the
      second or third generation, or a mini), with an assistant chosen:
      under both key fields Paste and Test key share a line and Clear sits
      under Paste, all three inside the card with its margin on the right,
      and each responds to a tap. On a 393pt or wider iPhone, an iPad and a
      Mac the three stand on one line as before. On Android, a 360dp phone
      puts Clear under Paste and a 411dp phone keeps one line.
- [ ] Settings on a 320pt iPhone SE, light and dark: "Show the
      translators' footnotes", "Show notes people share with you" and "Show
      the words of King Jesus in red" each take two lines inside their
      cards, nothing cut, the box centred against the two lines, and a tap
      on either line ticks the box. On a 375pt iPhone only the red-letter
      label takes two lines; on a 393pt or wider iPhone, an iPad and a Mac
      all three stand on one line as before. On Android, a 360dp phone
      breaks the shared-notes and red-letter labels and a 411dp phone none.
- [ ] Settings with the included API.Bible key: the NKJV caption sits
      under the TRANSLATIONS card, READING the usual gap below it. Clear the
      key, then paste a key of your own: the caption stays through both,
      unchanged, and a build with no included key shows it too.
- [ ] Mac at 1280x800, then a short window (1280x600): open Settings, Go
      to, Translation, Verse of the day, the audio source menu, an AI answer,
      cross-references and a share image. No sheet starts partway down the
      header — no arc of the Go to chip above a sheet's top edge; each
      opens below the header's rule. Cross-references once the list has
      loaded, not only while it loads. Windows and Linux the same.
- [ ] Mac, each of those sheets open in turn: maximise the window and open
      the sheet, then restore the window; drag the bottom edge up to about
      440pt tall and back down; open a sheet at 1280x600 and make the window
      taller. The sheet follows the window live, its top staying below the
      header's rule and never on the Go to chip, and it grows back into a
      taller window. Nothing typed or scrolled in it is lost (type a key or
      a verse, scroll Settings part way, then resize). Windows and Linux the
      same.
- [ ] Mac at 1280x440: the share image preview opens below the header with
      a smaller card, the whole card in view and Share and Cancel on
      screen.
- [ ] Translation, built with `-tags nrsv,lsb`: at 1280x800 the notice, key
      and evaluation lines sit above Close as before. At 1280x480 and
      1280x440 the sheet opens below the header's rule and those lines
      follow the last translation inside the list, reached by scrolling it,
      none missing; make the window taller and they return above Close. On
      an iPhone SE on its side they follow the rows too, with more of the
      list in view than before.
- [ ] iPhone and iPad, cross-references with a list: the panel stands 40pt
      from the top and bottom of the screen, as an AI answer does (it stood
      about 25pt taller), with Close and the credit line fully on screen.
      Android the same.
- [ ] iPad in Split View, Settings open with the keyboard up in a key
      field: resizing the window leaves the sheet at the size it opened at
      and the caret where it was (sheets are sized again on a desktop
      window only).
- [ ] 13-inch iPad, Settings: scroll the sheet to the end. SHARED NOTES'
      card and WORDS OF JESUS CHRIST OF NAZARETH come fully into view above
      the pinned footer; nothing is ever drawn under the footer.

## V16 — platform layout matrix

- [ ] iPad: reporter column centred at every text size; Split View to ~1/3
      width keeps a thin margin and re-centres live; a first-paragraph
      note band survives rotation/resize (the iPad-only inset overwrite
      once drew the card over the opening verses).
- [ ] Rotation re-places EVERYTHING: bands, pills, sticker, follow pill —
      no empty reserved band, no pill stranded at a pre-reflow Y.
- [ ] A width change never yanks a mid-chapter reader to the top; a
      height-only change (keyboard) never re-places at all; nothing moves
      while a finger owns the scroll.
- [ ] Notched devices: text starts below the header in both orientations,
      never under the Dynamic Island.
- [ ] A phone held sideways, the Dynamic Island on either side: the note
      composer and Ask (`note`, `ask`) keep their cards, their text and the
      left end of the text box clear of the island and of the inset
      opposite it, over the full-screen Read tab and over Books
      (`BIBLETEXT_DEV_TAB=books`); the bands beside the card are the
      sheet's own ground, and a tap there leaves the sheet open.
- [ ] Android rotation recreates the activity: the overlay re-renders —
      a blank pane or stale sticker after rotating is the failure.
- [ ] The old sidebar+HSplit regular layout is DEAD by policy: any iPad
      state showing a sidebar toggle or split divider is a classifier
      regression (`BIBLETEXT_DESKTOP_TABS=sidebar` is the only legit way
      to see it, on desktop, announced on stderr).
- [ ] The bottom bar is a centred pill on wide surfaces; dev builds' 4th
      tab still fits; full-screen reading looks like a phone everywhere.
- [ ] Phone landscape reading (on by default on iPhone and Android phones;
      the dev Links tab's "Landscape reading mode" switch turns it off, and
      `BIBLETEXT_DEV_PHONE_LANDSCAPE=on|off scripts/run-ios-sim.sh --dev`
      seeds a scripted run): on the Read tab, rotate mid-chapter → the reading pane
      alone, no header, toolbar or bar, the muted "Book Chapter" label with
      the chapter arrows and NO restore button — an arrow turns the page in
      place, opens the new chapter at the top, and greys out at the ends of
      the book; the text starts clear of the Dynamic Island in both
      landscapes and ends clear of the far edge; the book page, because the
      width allows it (reading_page.go) — the centred column with indents
      and no paragraph gaps, at the same leading as portrait. The
      verse under the top edge is the same verse after the rotation, both
      directions, with a selection live (the selection drops — the
      re-import replaces the string; it must not strand a menu) and with
      narration playing (the wash survives). Rotate back → the portrait
      chrome returns with the tab that was selected; full-screen chosen in
      portrait before rotating comes back with its restore button; a sheet
      open during a rotation closes (rebuildWindow drains overlays). Books
      and Search keep their ordinary layout in landscape — the presentation
      is a reading mode. Portrait, Go-to open with the keyboard up → no
      rebuild storm. Android phones get both halves too: the centred column,
      the indent on every prose paragraph and no blank line between them
      (justified on Android 15+, ragged below — the same SDK rule as
      portrait), the TextView keeping its place through the rotation by
      scroll fraction, and a note pill still placed though it loses its
      stack-centring (there is no blank line to centre in). In the LIGHT
      theme the short edge on the cutout side is paper, not black
      (BtBridge.extendIntoTheCutout), and the column is centred on the screen,
      not on the cutout-inset overlay — measure the left and right margins.
      Rotate the emulator with its toolbar button (`adb emu rotate`); if
      nothing happens, check auto-rotate is on and try the next click (the
      180° posture is refused on some images). Tablets: unchanged (rail in
      landscape).
- [ ] macOS reading position and note card: launch into a saved position on
      a chapter with an open note (John 11 with a note on verse 35 is the
      fixture) → the saved verse is at the top and the card sits 10pt above
      its paragraph with the band's air ABOVE it, not under its tail. Scroll
      with the wheel to another verse, flip the system appearance (or change
      the text size) → the pane stays where the reader left it; drag the
      window corner → stays. Before 2026-09-05 the flip returned the reader
      to the launch anchor and the card sat a line high.
- [ ] Desktop full-screen reading: the chapter toolbar's focus button drops the
      app header and the rail (or bottom bar), the "‹ Results" trail stays
      out, and the reading pane takes the whole window; its restore button
      brings the chrome back with the same tab selected. Check the macOS
      build and the Windows/Linux mimic.
- [ ] Web: resize across 46rem flips gaps ↔ reporter indents; poetry never
      takes the indent; print CSS hides the chrome.
- [ ] Procedure trap: `simctl io screenshot` can store landscape captures
      in a portrait buffer — judge the pixels, never the filename.

## Regression pins (what broke, so it stays visible)

Each line was a real screen defect this list would have caught:

1. iOS: chapter-scope note present → every mid-chapter pill stacked at the
   top, bands empty (`btIOSBandTopY` inset hijack).
2. iOS: two bands at paragraph 0 drew at one Y (no co-tenant stacking).
3. iOS: pills stranded at pre-reflow Ys after any frame change (the frame
   path re-placed only the sticker).
4. iOS: a key-0 pill could find the STICKER's band (sticker recorded under
   key 0, which is a real group key).
5. macOS: pill drawn at the view edge instead of the text column.
6. Android: a second band orphaned the first span (single-field handle);
   two spans on one paragraph hit the FontMetricsInt trap (coalesce, sweep
   by class).
7. Web: unconditional `::after` gave anchorless cards a tail claiming v1.
8. Every surface: plain entry dragged the reader to a collapsed note's
   pill (`chapterNoteArrival` targeting the note's own verse without an
   explicit arrival).
10. The natives: pills drawn beside an OPEN received bubble — the band push
    was not gated on the representation (`ShownAs`), so the set was said
    twice everywhere but the styled pane.
11. Every renderer: plain entries (arrows, picker) landed on any lit wash —
    the highlight cadences answered the classifier's question a second time
    instead of obeying the pushed `arriveNothing`.
12. The styled pane: the explicit-arrival flag was consumed on WIRING, so
    the double rebuild's second pane derived as plain and a tapped note
    link never placed. Consumed at placement now.
13. Web: John 4:9 reddened the Samaritan woman — red letter must follow
    the span data, not the verse.
14. iOS/macOS: bold-weight highlight re-wrapped the paragraph (~17% wider
    Georgia); a wash may change colour only.
15. All: a leaked infinite progress bar (load screen theme-flip, closed
    cross-refs panel) pinned the canvas dirty at ~20fps and made scrolling
    judder until force-quit.
16. Browser: deleting your open note made a stranger's note appear
    expanded — focus outlived the deleted id and fell to the next note.
17. Android: the Go-to keyboard lift arrived in dp not px (~2.2× overstated)
    and landscape left the verse row under the keyboard.
18. Tablets: reading orientation from a laid-out child instead of the
    canvas turned the soft keyboard into "rotation" — 3,000 rebuilds/min.
19. Desktop (shipped rail layout): the focus button swapped its icon while
    the header and rail stayed — CreateMainUI returned the shared layout
    before its own full-screen branch, and the shared layout never read
    IsFullScreen. The full-screen tree now lives in the shared layout.
20. Android: the note composer closed under the keyboard left the app
    laid out at keyboard height — text in a band, tab bar mid-screen,
    bottom third empty — until Home and back. The driver reads the insets
    only on a decor layout pass, and the keyboard's departure brought none.
9. Harness lesson, not a pixel: wipe the store before counting anything —
   stale fixtures made a correct "Notes · 3" label look like a wire bug.
