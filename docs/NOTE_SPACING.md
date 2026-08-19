# Where the note's spacing lives

A map, deliberately holding **no numbers**. Every value and every reason lives
in the code, because a document that restates them drifts the first time
someone changes one and the compiler says nothing.

## The one source of truth

**`notes_bubble.go`** — the `noteMetrics` table and, above it, the whole
narrative: what each number means, the four rules that hold on every surface,
how each platform reserves the band, the peculiarities that will bite you
(Android's dp-vs-sp, the who line's ellipsis, TextKit's collapsing paragraph
spacing), how to reproduce the pictures, and how to adjust the spec safely.

Read that comment first. Everything below is only "which file, and why it is
interesting".

## The four surfaces

| Surface | File | Reserves the band with |
|---|---|---|
| iOS | `reading_ios.go` (`btIOSInstallNote`) | `paragraphSpacingBefore`, plus a top-inset case for the first paragraph |
| macOS | `reading_macos.go` (`btMacInstallNote`) | the same, plus a measured top-gap correction; places the sticker bottom-up |
| Android | `android/BtBridge.java` (`applyNoteBand`, `NoteBandSpan`) | a `LineHeightSpan` on the preceding line's descent |
| Windows/Linux | `reading_styled_layout.go` (`BandVerse`/`BandLine`) | advance at the paragraph's top, like `ParaGap` |

## What enforces it

- **The pill** (the collapsed marker) is spec'd like the card but has its own
  side padding and a width floor — the pill is chrome, not a paragraph, and a
  shrink-wrapped one reads as an accident. Its LABEL is a contract too: the who
  font, muted ink, centred both ways. The interesting part is that measuring and
  drawing can disagree — the styled pane measured at 11pt and drew at the
  theme's 18 — so `TestStyledPillMatchesTheApplePill` asserts the DRAWN objects,
  not the constants that produced them.

- **`notes_spacing_spec_test.go`** parses the three native sources and fails if
  a literal or a shape leaves the Go table behind. It runs on a Mac for
  platforms a Mac cannot compile.
- **`reading_styled_note_gallery_test.go`** renders 14 permutations × light and
  dark to a real software canvas, asserting the geometry before it writes each
  PNG.
- **`scripts/view-test-gate.sh`** plants defects in the note surfaces and
  requires a test to catch each one.

## Open question

Whether the note should sit **below** its paragraph rather than above is
tabled, high priority, and undecided — see `docs/NOTES_SCRAPBOOK.md`.
