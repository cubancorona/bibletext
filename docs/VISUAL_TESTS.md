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
| macOS native | `go run -tags bibletextdev ./cmd/desktop` | the dev Links tab |
| Styled pane (Windows/Linux) | `BIBLETEXT_MIMIC=linux go run -tags bibletextdev ./cmd/desktop` (or `windows`) | the dev Links tab |
| Web reader | `go run ./cmd/websitegen -out build/site -offline`, serve `build/site` | mint links with `ShareLinkURLWithNote` (a throwaway test printing them) |

More drivers for the sections below: `SIMCTL_CHILD_BIBLETEXT_DEV_OPEN=settings|goto|versions|votd`
auto-opens a sheet on the simulator for screenshots; `BIBLETEXT_ENABLE_TESTING=1`
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

## V1 — the collapsed state (pills), the sharpest fixture

Store: 4 received notes on 4 paragraphs + 1 chapter-scope, all minimized.

- [ ] Every noted paragraph carries a pill at its own reservation, and
      NO reserved band is empty (an empty band = a placement bug, the iOS
      inset-hijack shape).
- [ ] Mid-chapter pills read CENTERED between the paragraphs on layouts with
      a paragraph separator (phones, narrow styled, narrow web): the air
      above ≈ the air below (the centering rule, notePillSeparatorLift —
      the pill rises half the separator above its band top). On reporter
      layouts (iPad/macOS/wide) nothing moved: GapAbove into the band as
      always. The OPEN card never centres — its tail stays the pinned 10
      above the passage.
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

- [ ] Prose justifies with flush right edges (iOS, macOS, Android, web);
      the styled pane and the legacy Entry pane never justify — an EXPECTED
      difference, not a bug. Poetry is always ragged.
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
      wash AND pill (activity recreation).
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
- [ ] Landscape, on Books and Search: Android phones and all tablets move
      the bar to a left rail; the iPhone keeps its bottom bar. On the Read
      tab a phone reads full-screen instead (the V16 row); raising the soft
      keyboard must NOT flip the layout (the 3,000-rebuilds/min trap).

## V12 — sharing and links

- [ ] Share with citation: quote keeps authored poetry breaks; the
      citation spells the version in full. The styled pane copies to the
      clipboard with a notice instead of a sheet — expected.
- [ ] Share as image: the preview modal appears BEFORE anything leaves the
      app; Regenerate cycles schemes; the native overlay must not paint
      over the modal; the same verse always opens on the same look.
- [ ] Share with note: the counter wraps rather than pushing Share off a
      narrow screen; past 280 runes it says "Too long by N"; sharing the
      same words twice dedups — your own second link shows YOUR note,
      never "Note from Friend".
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
      justified notes. Drive with BSB Ezekiel 40 (25 notes) or WEBC
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
- [ ] Own notes: never drawn in scripture until their row is tapped;
      byline "From you"; the bubble carries ONE ✕ (no −); ✕ dismisses
      without touching the store; only the browser's bin deletes.
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
      on the same verse; with a sheet open the rebuild defers to sheet
      close; the iOS app-switcher double-snapshot must not yank the sheet.
- [ ] Palette spot-checks in BOTH variants: unchecked boxes visible on
      dark cards, sapphire accent (never stock Fyne blue), no white frame
      around popups, disabled controls quieter but readable, red letters
      legible over the wash.
- [ ] Changing ONLY an AI key must not flicker the reading pane on close;
      red-letter/text-size/footnotes changes refresh reading only; the
      notes/assistant switches rebuild the window once.

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
- [ ] Android rotation recreates the activity: the overlay re-renders —
      a blank pane or stale sticker after rotating is the failure.
- [ ] The old sidebar+HSplit regular layout is DEAD by policy: any iPad
      state showing a sidebar toggle or split divider is a classifier
      regression (`BIBLETEXT_DESKTOP_TABS=sidebar` is the only legit way
      to see it, on desktop, announced on stderr).
- [ ] The bottom bar is a centred pill on wide surfaces; dev builds' 4th
      tab still fits; full-screen reading looks like a phone everywhere.
- [ ] Phone landscape reading (on by default on iPhone and Android phones;
      the dev Links tab's two "Landscape …" switches turn either half off,
      and `BIBLETEXT_DEV_PHONE_LANDSCAPE=on|typo|off scripts/run-ios-sim.sh
      --dev` seeds a scripted run): on the Read tab, rotate mid-chapter → the reading pane
      alone, no header, toolbar or bar, the muted "Book Chapter" label with
      the chapter arrows and NO restore button — an arrow turns the page in
      place, opens the new chapter at the top, and greys out at the ends of
      the book; the text starts clear of the Dynamic Island in both
      landscapes and ends clear of the far edge; with typography on, the
      centred column with indents and no paragraph gaps (the LEADING is
      unchanged on purpose — it is measured to match, not nominal). The
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
9. Harness lesson, not a pixel: wipe the store before counting anything —
   stale fixtures made a correct "Notes · 3" label look like a wire bug.
