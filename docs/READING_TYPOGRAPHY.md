# How Scripture is set

The reference for the reading text itself: which faces set it, how large, how far
apart the lines sit, and how wide the column runs — on all six surfaces at once.

Read this before changing any of those four numbers. Each of them is shared, each
has at least one surface where the obvious edit does the wrong thing, and three of
the four were once wrong in ways that shipped.

`docs/SCRIPTURE_WORKLIST.md` is the investigation that produced these decisions,
including the sixty-six faces measured and rejected. This file is the outcome.

---

## The faces

| | |
|---|---|
| **Junicode** | Latin and polytonic Greek, four real cuts. Subsetted into `assets/fonts/reading/`. |
| **Ezra SIL** | Pointed Hebrew. Shipped unmodified — "Ezra" and "SIL" are Reserved Font Names. |

Two, because no single face passes. The faces that cover all three scripts have
three cuts, or put their small capitals in the regular alone, or reuse one upright
regular-weight Greek in every cut — which would set Greek upright inside an
italicised supplied word. The faces with four properly featured cuts have no
Hebrew at all.

Rebuild both with `scripts/build-reading-fonts.sh`, which checks that the subset
kept the small capitals, the Greek Extended and the superior figures. The small
capitals are scattered across three Unicode blocks and a subset that misses one
loses letters from the divine name silently.

---

## Four numbers

All four live in `reading_face_scale.go`.

| quantity | value | what it means |
|---|---|---|
| optical scale | `1.15178` | how much the nominal size opens up so the type reads its true size |
| leading | `1.2222` | baseline to baseline, as a multiple of the size the type is SET at |
| body base | `21` | the "Normal" reading size before the reader's own setting |
| measure | `27.5 em` | the reading column's width, in ems of the UNSCALED size |

### The optical scale, and the one thing it must not touch

Two faces set at the same nominal size do not draw the same size. What the eye
reads as "how big is this text" is the x-height, and the share of the em a face
spends on it varies enormously. Junicode spends 0.418; the screen serif the panes
used to resolve spends 0.4814. Swapping one for the other at an unchanged nominal
size shrank Scripture by 13.2% with no size in the code changing — on a phone, the
drawn x-height fell from 30 device pixels to 26.

The scale buys that back. It reaches **glyph sizes** and **every length reckoned in
ems of the set type** — leading, first-line indents, the space between paragraphs.
Those are marks made in the type's own units and they grow with it.

It does **not** reach a **MEASURE**. That exception is the whole design, and the
reason is not obvious:

```
Junicode's mean lowercase advance ÷ the reference face's  =  0.8691
0.8691 × 1.15178                                          =  1.0009
```

The narrower face already fits more characters into a column of a given width —
63 where the reporter measure was cut for 55. Holding that column at its old
physical width while the glyphs inside it grow puts the line back to its old
character count, within a tenth of a percent. **The two errors cancel.** Scale the
measure as well and both survive: the column widens 15%, the line keeps the
characters it should not have, and the reading column overtakes the list column
that `readable_column.go` exists to keep it in sympathy with — on a tablet at the
largest text size it stops fitting the page at all.

So: `readingReferencePx()` is for the measure and nothing else.
`readingGlyphPx()` is for everything else.

### The leading, which used to be an accident

The stylesheet asks for a unitless `line-height`. The HTML importer turns that
into a **minimum line height on each RUN**, and a paragraph takes its leading from
its **first** run — which in this document is always the 0.66em verse numeral. The
drawn pitch was therefore `2.0 × 0.66 × the body`: a number no reader of the
stylesheet could have predicted, and one that would have moved silently if that
numeral were ever resized.

It is set explicitly now, in each pane's paragraph sweep, from **each paragraph's
own largest font** — so the footnote apparatus and the section headings lead in
proportion to their own size rather than the body's.

> **The trap.** The importer hands out paragraph styles **per run**, not per
> paragraph. Taking "the largest font in the range the sweep is handed" therefore
> reads the verse numeral, sets the whole page's leading from a superscript, and
> collides the lines. Widen with `paragraphRangeForRange:` first. On macOS the
> sweep also mutates its string in place, so the font lookup reads an immutable
> copy taken before the sweep begins.

**Judge leading against the face's real ink, not its declared line box.**
Junicode's `hhea` box is 1.27 em; its actual ink over the characters Scripture
uses is 0.999 em. The declared box overstates by 27%, and believing it leads to
the conclusion that the face needs far more room than it does.

| setting | pitch | clearance between one line's deepest descender and the next line's tallest ascender |
|---|---|---|
| the old face at 21 px | 83 device px | 7.2 pt |
| the accident, 24 px | 95 device px | 7.7 pt |
| **chosen, 24 px** | **88 device px** | **5.4 pt** |

---

## Where each number lives, per surface

| surface | glyph size | leading | measure |
|---|---|---|---|
| Apple panes (iOS + macOS) | `reading.go` — the CSS `font-size`, from `readingGlyphPx()` | the native paragraph sweep, from `readingLinePitchEm` | `reporterMeasureEm × readingReferencePx()` across the bridge |
| Fyne canvas pane | `styledPaneTextSize()` | `p.textSize × 1.55` (cozy) / `× 1.3` (reporter) | `reporterMeasureEm × p.referenceSize()` |
| Android overlay | Java: `textSizeDp × density × opticalScale` | Java: `pitch × textSizePx` | `androidReadingMeasureDp(reporter, referenceDp)` |
| Verse-of-the-day card | `readingGlyphPx()`, the Apple/Android body size, as a `canvas.Text` `FontSource` | the face's own line box (`readingParagraph`) | the card's inner width — no reporter measure; it is one passage |
| generated site | `remSize(webScriptureBaseRem)` | `ReadingLinePitchEm()` | `.wrap{max-width:40rem}` — root rem, deliberately not em |
| share card | its own face rotation, auto-fitted | n/a | n/a |

Two decisions that look like oversights and are not:

- **Android applies the optical scale in Java, not in the Go push.** Only that
  side knows whether it got the shipped face: the custom family needs API 29, and
  below that the overlay still draws in the platform serif, which needs no
  correction and would simply come out 15% too large. The leading is gated the
  same way, and that fleet keeps the value its own metrics were measured against.
- **The Fyne pane's fallback text widget takes no correction at all.** It draws in
  the chrome face — `bibleTheme.Font` returns the UI family for every
  non-monospace style — so scaling it would make it 15% too large.

**Hebrew takes the scale too, and should.** Ezra SIL is only ever reached for
Hebrew glyphs, through a cascade on Apple and a custom fallback on Android, so a
uniform scale leaves Hebrew's size relative to the Latin around it exactly where
it was. Do not "correct" it by comparing Ezra's Latin `x` against the reference —
that glyph never draws.

---

## Changing any of this safely

The tests that will catch a mistake, and what each one is for:

| test | catches |
|---|---|
| `TestOpticalScaleMatchesTheShippedFace` | a rebuilt subset whose proportions moved. Reads the **outline**, not the OS/2 field — they disagree by 0.003 em in this family, always in the same direction. |
| `TestHoldingTheMeasureRestoresTheOldLineLength` | the advance ratio and the scale ceasing to be reciprocal, which is what makes leaving the measure alone correct |
| `TestTheCanvasPaneKeepsTheMeasureOffTheOpticalScale` | the desktop pane's two sizes collapsing into one |
| `TestScriptureIsSetAtTheOpticallyCorrectedSize` | the site drifting from the app on size or leading |
| `testdata/chapter_tint_golden.txt` | any change to the emitted stylesheet, on every surface and tint at once |

Then look at it. Three separate automated measurements of line pitch — an
autocorrelation, a peak detector, a band merge — each reported a plausible number
for a page whose lines were visibly overlapping. The only measurement that proved
stable was first-line-top to last-line-top divided by a line count taken from the
image. **A screenshot read by eye caught what none of them did.**

`BIBLETEXT_MIMIC=windows go run -tags bibletextdev ./cmd/desktop` previews the
canvas pane on a Mac (see `docs/PLATFORM_MIMIC.md`); `scripts/run-ios-sim.sh` and
an Android install cover the rest.
