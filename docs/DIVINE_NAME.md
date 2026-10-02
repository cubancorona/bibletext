# The divine name

What this project knows about the Tetragrammaton — how each shipped edition
renders it, how the app draws it, what leaves the app when a reader shares a
verse, and where the handling is thinner than it looks.

Read this before touching small capitals, the `nd`/`sc` markers, or anything
that compares selected text against verse text. The last item is not obvious:
the divine name is the reason those two can differ, and a change that assumes
they are the same string will fail only on the verses this page is about.

---

## 1. The convention, and why there are two spellings

English Bibles have long marked the divine name typographically rather than
translating it. The convention:

| printed | stands for | Hebrew |
| --- | --- | --- |
| `LORD` in small capitals | the divine name | יהוה (YHWH) |
| `GOD` in small capitals | the same divine name | יהוה (YHWH) |
| `Lord` in ordinary case | a different word, meaning lord or master | אֲדֹנָי (*Adonai*) |
| `God` in ordinary case | a different word again | אֱלֹהִים (*Elohim*) |

`GOD` exists because of a collision. The pair *Adonai YHWH* occurs often — it is
characteristic of Ezekiel and Isaiah — and applying the usual rules to it gives
"Lord LORD", which reads as a stutter rather than as two different words. The
convention substitutes the second element: **"Lord GOD"**, where `GOD` is the
divine name and `Lord` is *Adonai*.

So `LORD` and `GOD` are not two different words treated inconsistently. They are
the same word, spelled differently to avoid colliding with the word beside it.

A third case exists: the short form יה (*Yah*), which some editions set as `YAH`
or `JAH` — Psalm 68:4 is the familiar one.

**The World English Bible states this itself**, in a footnote that appears 100
times across its text:

> When rendered in ALL CAPITAL LETTERS, "LORD" or "GOD" is the translation of
> God's Proper Name (Hebrew "יהוה", usually pronounced Yahweh).

and, for the short form:

> LORD or GOD in all caps is from the Hebrew יהוה Yahweh except when otherwise
> noted as being from the short form יה Yah.

A reader sees those notes only with the footnotes toggle on, in the
chapter-bottom section.

---

## 2. What each shipped edition actually does

**This is the fact to carry away: only the NKJV is drawn in true small
capitals.** Everything else in this document describes machinery that one
edition uses.

| edition | how the divine name arrives | drawn as |
| --- | --- | --- |
| World English Bible | literal capitals `LORD` / `GOD` in the verse text | ordinary full-size capitals |
| WEB Catholic | the same, in the 66 books | ordinary full-size capitals |
| Berean Standard Bible | literal capitals `LORD` / `GOD` | ordinary full-size capitals |
| New King James Version | a character span the feed marks `nd` or `sc` | **small capitals** |

Measured by decoding the cached feeds through the app's own decoders:

- `Verse.SmallCaps` is written in exactly **one** place in the tree —
  `apibible.go:1084`, inside the API.Bible / USX decoder. That is the NKJV path.
- WEB, WEBC and BSB decode to **zero** small-caps spans. This is structural, not
  an oversight: the helloao feed has no field that could express a character
  style. Its verse items come in six shapes only — a bare string, `{text,poem}`,
  `{lineBreak}`, `{noteId}`, `{text,wordsOfJesus}`, `{text,descriptive}`.
- Those editions carry the name as literal capitals instead: roughly 6,571
  occurrences of `LORD` in WEB and WEBC, 6,485 in the BSB.

Nothing in the app detects an all-capital `LORD` in public-domain text and
re-sets it as small capitals. There is no such fallback, by design and by
absence.

**The WEBC deuterocanon is different again.** Those nine books, translated from
Greek (*kyrios*) rather than Hebrew, carry essentially no all-capital divine
name: Sirach has 203 ordinary-case "Lord" and none in capitals. Two all-capital
`GOD`s in 2 Maccabees are battle watchwords, not the divine name.

A smaller difference a reader meets within seconds of switching: WEB and WEBC
write the possessive `LORD's` 1,177 times where the BSB prefers "of the LORD"
and writes it only 129 times.

---

## 3. The two span shapes the NKJV sends

Both aim at the same printed result — a **full-size initial letter followed by
small capitals** — but they encode it differently, and the app has to honour
both.

| source shape | span covers | what does the work | drawn |
| --- | --- | --- | --- |
| `Lord` | the whole word | **letter case** — the `L` is already a capital, so only `ord` shrinks | `Lᴏʀᴅ` |
| `G` + `OD` | only `OD` | **the span boundary** — the `G` is left outside so it cannot shrink | `Gᴏᴅ` |

For mixed case, case alone is sufficient instruction. For all capitals it is
not: applying small capitals to `GOD` whole would shrink all three letters and
give `ɢᴏᴅ`, with no full-size initial. So the marker excludes the `G`.

`applySmallCaps` (`small_caps_draw.go`) implements exactly this: *a span
containing lower case shrinks its lower case; a span of capitals alone shrinks
its capitals* — which is what OpenType's small-caps feature would do with the
same input.

In a live structural sample of ten chapters, the whole-word shape was
overwhelmingly dominant: 79 of 81 spans. Both capitals-only spans fell in
Psalm 68 — the `YAH` case.

---

## 4. The path from feed to what a reader sends

```
FEED        nd/sc marker          apibible.go — spanSentinels, stripSentinels
  │
  ▼
STORE       Verse.Text keeps the PUBLISHER'S OWN LETTERS.
            Verse.SmallCaps records where the feature applies.
  │
  ▼
DRAW        applySmallCaps substitutes Unicode small-capital CHARACTERS.
            Rune counts are preserved, because every offset the app records
            — notes, highlights, red letter — is measured in those runes.
  │
  ├──▶ TO A READER   sharedText keeps them as drawn.        "Lᴏʀᴅ"
  │                  Share, image card, verse of the day, Copy.
  │
  └──▶ TO A MACHINE  outboundText resolves them to CAPITALS. "LORD"
                     AI study.
```

**Headings and psalm titles carry the span too.** The publisher's section
headings ("The LORD Is My Shepherd") and the psalm titles that name the divine
name mark it with the same `nd`/`sc` span. Until 2 October 2026 the decoder
bracketed the span only inside a verse and read it as plain words in a heading or
a title, so every surface — the four app panes and the website — drew the
stored lower-case `Lord` there, under verses that drew `Lᴏʀᴅ`. They are kept now
as `Heading.SmallCaps` and `Superscription.SmallCaps`, offsets into the
publisher's own letters exactly as `Verse.SmallCaps` is, and every surface draws
the heading and the title through `DrawnText`, which runs the same
`applySmallCaps`. Search and speech still read the stored letters; the share
pipeline's heading repair looks for a heading as it is DRAWN, because the
selection it is matching came off the page. A supplied-word span inside a heading
is still read as plain words. The bracket is the `sc` style's as well as `nd`'s,
so a heading's other small capitals are drawn too: the NKJV sets the name of
each Hebrew letter heading Psalm 119's stanzas in small capitals, and the
twenty-two names are drawn so on every surface, after the letter in the Hebrew
face. Measured on the fresh fetch of 2 October 2026: 74 section headings carry
the divine name (64 `Lord`, 10 `Lord's`), 4 titles carry it 6 times, and those
22 stanza names are the only other small capitals in a heading. An installed app picks this up when its copy of
the NKJV is next decoded — at its next fetch, within the 30-day recency window —
or at once if the NKJV's `cacheEpoch` is bumped, which has deliberately not been
done (`versions.go`). The website fetches afresh on every build and has it at
once.

Three things about the way out matter more than they look.

**It is characters, not a font feature.** The app substitutes real codepoints
(`ʟ` U+029F, `ᴏ` U+1D0F, `ʀ` U+0280, `ᴅ` U+1D05, `ɢ` U+0262), rather than asking
the renderer for `font-variant: small-caps`. That is why the name survives being
copied at all.

**What a reader sends is what the page shows.** Since 22 September 2026 every
way a reader sends or copies text — the text share, the image card, the verse
of the day's share, the styled pane's Copy and the chapter copy icon — keeps the
small capitals, as the system Copy of the Apple and Android panes always did.
It is the account holder's choice, made after seeing a shared `Lᴏʀᴅ` arrive in a
message, with the costs weighed:

| where it lands | effect |
| --- | --- |
| a message, read by a person | reads like the printed page; Apple's system font carries all five characters |
| a later search for "Lord" or "LORD" | **does not find it** — the characters have no Unicode equivalence to ordinary letters |
| a document | depends on the font: Arial and Times New Roman carry all five, Helvetica two of five (the others come from a fallback, unevenly), Georgia none |
| an SMS | no change in length; the curly quotes already force Unicode encoding |

Before that date a share sent the capitals, and the chapter copy icon sent the
stored `Lord` — a third spelling no surface shows, and the one that erases the
distinction the small capitals exist to carry.

**A machine still gets capitals.** An AI request reads standard text and has no
reader to see the typography, so it goes through outboundText, which resolves
each small capital to the CAPITAL. A small capital records nothing about which
case it replaced, so there is no letter to go back to:

```
publisher "Lord"  ──drawn──►  "Lᴏʀᴅ"  ──to a machine──►  "LORD"    ≠ "Lord"
publisher "GOD"   ──drawn──►  "Gᴏᴅ"   ──to a machine──►  "GOD"     = "GOD"
```

Capitals are the plain-text convention every other edition uses, and they keep
the divine name distinct from an ordinary "Lord".

### Why the form reaches the share pipeline

Sharing locates the selection among the chapter's verses, so the citation can
name the verses actually being sent. The selection comes off the **page** (small
capitals); the verses are stored in the **publisher's** letters (`Lord`). So the
stored verses are drawn the same way before they are searched, and the two meet
in one form:

```
SELECTION  "The Lᴏʀᴅ ..."  ──sharedText───────────────────────►  "The Lᴏʀᴅ ..."
CORPUS     "The Lord ..."  ──applySmallCaps──►  "The Lᴏʀᴅ ..."  ──►  "The Lᴏʀᴅ ..."
```

`verseSharedText` (`outbound_text.go`) is that conversion, and it runs the real
`applySmallCaps` rather than re-deriving which letters shrink — a second
implementation of that rule would drift from the first, and the search would
fail on whichever verses the two disagreed about.

The form has moved once: it was the capitals while a share sent capitals. What
must never happen is the two sides being in different forms. Every structure
the pipeline matches against — `chapterProse`, `chapterShareStructure`, the
marker strip's verse bodies, the fallback's probes — is built by the same
function for that reason; when only one corpus was converted, the psalms
silently lost their authored line breaks, and when the marker strip compared
against the stored text, a whole-verse drag of any divine-name verse fell off
the positional path. `TestChapterProseAndShareStructureAgree` and the tests in
`share_small_caps_marker_test.go` hold both.

## 5. Where the small capitals are not there

- **Android below API 29.** Font coverage for the Latin small-capital block is
  not guaranteed on the older platform serif. What such a device renders has not
  been measured on hardware.
- **The website — subset for the public-domain pages, supplemented for the
  NKJV's.** The site ships the SAME typeface as the app — Junicode, same source
  file — but a tighter subset: `scripts/build-reading-fonts.sh` gives the app
  `U+1D00-1D7F` and `U+A700-A7FF` and withholds both from `WEB_RANGES`, because
  none of the public-domain editions uses them. Measured: the desktop Junicode
  carries 25 of 25, the web woff2 carries 0 of 25 and is 27 KB against 409 KB.
  The script checks the APP's subset kept all 25, because they are scattered
  across three Unicode blocks and a subset missing one loses letters from the
  divine name silently. `/web/`, `/bsb/` and `/webc/` carry no small-caps spans
  at all, so those pages contain ordinary capitals and nothing to render —
  verified on the live site, zero small-capital codepoints across those
  chapters. The NKJV, the one edition that needs them, is published only behind
  the switch in `cmd/websitegen/nkjv_text.go`. While it is on, its pages load
  supplementary faces holding exactly the 25 small capitals, one per cut the
  name is set in — `Junicode-SmallCaps.woff2` for a verse,
  `Junicode-BoldSmallCaps.woff2` for a section heading and
  `Junicode-ItalicSmallCaps.woff2` for supplied words and psalm titles (2 to
  3 KB each, in `assets/fonts/reading/web/`, built by
  `scripts/build-web-nkjv-fonts.sh` from the app's own table and checked 25 of
  25). A browser matches a bold or italic run against faces of that weight and
  style only, so before the bold and italic cuts shipped, the name inside a
  supplied word was drawn from a system italic. Each is declared after the face
  it supplements with a `unicode-range` of those code points, so only a page
  that draws one downloads it. While the switch is off, the `/nkjv/` path serves
  notices with no text to render.
- **Secondary surfaces.** Search result cards and cross-reference snippets show
  the edition's stored mixed-case form, not small capitals. Only the reading
  pane substitutes. Whether that is a defect or a deliberate one-line-of-text
  decision has never been settled.
- **Share cards.** Only one of the seven card typefaces, Cardo, carries the
  five small capitals. The other six draw the name from their OWN capitals at
  0.70 of the size (`cardText`, share_image.go) — real small capitals measure
  0.675 of a capital in Cardo and 0.629 in Junicode, and the slightly larger
  figure allows for a scaled capital's strokes coming out a touch lighter. Cardo
  uses its designed glyphs. So a divine-name card rotates through all seven
  typefaces under Regenerate, like any other card. Only a line containing such
  a small capital takes the new path; every other card was shown byte-identical
  to its previous rendering. `share_image_smallcaps_test.go` holds the four
  properties: measured width equals drawn width, ordinary lines draw exactly as
  before, no face draws a missing glyph, and the rotation reaches every face.

---

## 6. Known gaps

**`sc` and `nd` are collapsed into one field.** Both styles return the same
sentinel pair and land in the same `Verse.SmallCaps`, so nothing records which
publisher convention a given span used. Fine today; it would matter if an
edition ever used them to mean different things.

**There is no small-capital `x`.** Unicode provides small-capital forms for 25
of the 26 Latin letters; `x` has none, so a span containing one would leave that
letter full-size. Unreachable for the divine name, but it is the same shape of
untested assumption as the one below.

**A Psalm superscription can be shared as scripture.** The heading strip
consults `Bible.Headings` and never `Bible.Superscriptions`, so a drag from a
psalm's title into verse 1 quotes editorial matter under a verse reference. See
`docs/BACKLOG.md`.

**Counts of the NKJV's spans disagree across documents.** Four figures appear in
four places and none is checkable without a licensed fetch. Treat any specific
number as unverified unless it names the sweep that produced it.

---

## Related

- `docs/SOURCE_FIELDS.md` — the inventory of what each source sends; the
  `nd`/`sc` entry belongs to the same family as this page
- `docs/TEXTUAL-DATA.md` — derived tables, including red-letter spans, whose
  offsets share the rune-count invariant described above
- `docs/SCRIPTURE_WORKLIST.md` — the governing standard for text-pipeline work
- `docs/ADDITIONS_AND_DROPS.md` — what the app may add or drop from a text
