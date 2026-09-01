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

The one scenario that exercises most of this list at once: **s12pills**
(dev builds) — four received notes on four John 11 paragraphs plus a
chapter-scope note, collapsed, then a plain re-entry. The dev tab's
**SPREAD 1–4 of 4** rows are the same fixture by hand (John 3), and
**"Seed 3 of MY notes"** exists to prove own notes DON'T join the counts.
Wipe the store first ("Delete all stored notes") — stale notes from earlier
sessions change every count and every band, and an evening was once lost to
exactly that.

## V1 — the collapsed state (pills), the sharpest fixture

Store: 4 received notes on 4 paragraphs + 1 chapter-scope, all minimized.

- [ ] Every noted paragraph carries a pill, in its own reserved band, and
      NO reserved band is empty (an empty band = a placement bug, the iOS
      inset-hijack shape).
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
- [ ] A pill sharing the open card's paragraph stacks ABOVE the card.
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
9. Harness lesson, not a pixel: wipe the store before counting anything —
   stale fixtures made a correct "Notes · 3" label look like a wire bug.
