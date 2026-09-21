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
  ▼
OUT         outboundText resolves each small capital to the CAPITAL.
            "Lᴏʀᴅ" -> "LORD".    "Gᴏᴅ" -> "GOD".
```

Two things about that last step matter more than they look.

**It is characters, not a font feature.** The app substitutes real codepoints
(`ʟ` U+029F, `ᴏ` U+1D0F, `ʀ` U+0280, `ᴅ` U+1D05, `ɢ` U+0262), rather than asking
the renderer for `font-variant: small-caps`. That is why the name survives being
copied, and why it needs undoing on the way out.

**The resolution goes to the capital, and this is deliberate.** A small capital
records nothing about which case it replaced, so there is no letter to go back
to:

```
publisher "Lord"  ──drawn──►  "Lᴏʀᴅ"  ──out──►  "LORD"    ≠ "Lord"
publisher "GOD"   ──drawn──►  "Gᴏᴅ"   ──out──►  "GOD"     = "GOD"
```

Lower-casing everything would turn `GOD` into `God`; upper-casing diverges from
`Lord`. Capitals are chosen because they are the plain-text convention every
other edition and reader puts on a clipboard, and because they keep the divine
name distinct from an ordinary "Lord". The cost is that outgoing text and stored
text genuinely differ for the `Lord` shape.

### Why that cost reaches the share pipeline

Sharing locates the selected text inside a corpus built from the chapter's
verses, so the citation can name the verses actually being sent. The selection
comes off the **page** (small capitals); the corpus was built from the
**publisher's** text (ordinary letters). Neither can be converted into the
other, for the reason above.

So both are taken to the same third form — the outbound one — before being
compared:

```
SELECTION  "The Lᴏʀᴅ ..."  ──outboundText──────────────────────►  "The LORD ..."
CORPUS     "The Lord ..."  ──applySmallCaps──►  "The Lᴏʀᴅ ..."  ──►  "The LORD ..."
```

`verseOutboundText` (`outbound_text.go`) is that conversion, and it runs the
real `applySmallCaps` rather than re-deriving which letters shrink — a second
implementation of that rule would drift from the first, and the search would
fail on whichever verses the two disagreed about.

**Both corpora must be in that form.** `chapterProse` and
`chapterShareStructure` are matched against each other, and when only one was
converted the psalms silently lost their authored line breaks.
`TestChapterProseAndShareStructureAgree` holds them equal and now carries a
fixture with a small-caps span, because every other fixture — and every verse of
the shipped WEB gospels — has none, so nothing could break it.

---

## 5. Where the small capitals are not there

- **Android below API 29.** Font coverage for the Latin small-capital block is
  not guaranteed on the older platform serif. What such a device renders has not
  been measured on hardware.
- **The website — latent, not live.** The web reader's own font does not carry
  the block. It does not matter today and the reason is worth knowing: the
  reader publishes only `/web/`, `/bsb/` and `/webc/`, and those three editions
  carry no small-caps spans at all, so the pages contain ordinary capitals and
  nothing to render. Verified on the live site — zero small-capital codepoints
  across those chapters. The NKJV, the one edition that would need them, is
  licensed and its path serves a notice instead of text. This becomes real the
  day the reader publishes an edition that marks the divine name, and not
  before.
- **Secondary surfaces.** Search result cards and cross-reference snippets show
  the edition's stored mixed-case form, not small capitals. Only the reading
  pane substitutes. Whether that is a defect or a deliberate one-line-of-text
  decision has never been settled.
- **Share cards.** The rendered image is built from outbound text, so small
  capitals never reach it and its fonts do not matter.

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
