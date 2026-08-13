# Textual data: versification, red letters, and where it all comes from

This document exists so that a reader who does not trust us can check us. Every
table BibleText ships that makes a claim *about the text* — which verse is which
across translations, which words are Christ's — is derived, not authored, and
this is the record of what it was derived from, by what rule, and what was done
to validate it.

**How claims here are marked.** Scholarship and software fail in the same way:
by repeating a plausible thing nobody checked. So each claim carries its status.

| mark | meaning |
| --- | --- |
| **[measured]** | Established from data in this repository, by a command written out below. Re-runnable. |
| **[source]** | Stated by the publisher of the text in question, quoted here. |
| **[standard]** | The ordinary scholarly account, given as orientation. Verify against the apparatus before relying on it — we have not. |

---

## 1. What the app ships

| id | edition | text supplier at runtime | licence |
| --- | --- | --- | --- |
| `web` | World English Bible | bible.helloao.org | public domain |
| `webc` | World English Bible, Catholic edition | bible.helloao.org | public domain |
| `bsb` | Berean Standard Bible | bible.helloao.org | free use, see the BSB's own terms |
| `nkjv` | New King James Version | api.scripture.api.bible | © 1982 Thomas Nelson; licensed, never redistributed by us |

The *derived tables* in this repository are built from the publishers' own
files (§6), not from the runtime supplier, because a publisher's USFM carries
editorial markup — verse boundaries, words-of-Jesus spans, footnotes — that the
runtime JSON has already flattened away.

---

## 2. Versification: verse numbers are not a shared address space

A reference like "Mark 9:44" is not a coordinate. It is a claim about a
particular edition's numbering, and editions disagree — not because anyone is
careless but because they descend from different textual decisions, some of them
centuries old.

The app has to carry references *between* editions in at least four places: a
shared link, a note written in one translation and read in another, the
words-of-Christ table (generated from one edition and applied to all), and any
comparison view. Getting this wrong does not fail loudly. It shows the reader a
different verse than the one they were sent, which is the worst failure a Bible
application has.

`versification.go` encodes four outcomes:

- **exact** — the same number means the same passage. ~31,000 verses.
- **moved** — the same passage, a different number.
- **absent** — the target edition does not contain this verse at all.
- **incommensurable** — the two editions' versions of this *book* do not
  correspond verse by verse, and no mapping is possible.

### 2.1 What we found, per edition

Measured against the WEB as reference **[measured]**:

| edition | absent | moved | present here but not in WEB | incommensurable |
| --- | --- | --- | --- | --- |
| BSB | 12 | 3 | 0 | — |
| NKJV | 0 | 3 | 4 | — |
| WEBC | 0 | 7 | 173 | Esther |

**The twelve the BSB lacks** **[measured]**: Matthew 17:21, 18:11, 23:14;
Mark 7:16, 9:44, 9:46, 11:26, 15:28; Luke 23:17; John 5:4; Acts 28:29; and
Romans 16:24.

The first eleven are the passages usually called the "missing verses" of modern
translations. They are present in the Byzantine tradition and therefore in the
Textus Receptus and the King James line, and absent or obelised in the earliest
Alexandrian witnesses; modern critical editions (Nestle-Aland, UBS) print them in
brackets or in the apparatus rather than the text **[standard]**. A translation
that follows the critical text omits them; one that follows the TR keeps them.
That is exactly the split we measure: the BSB omits, the NKJV keeps.

Note the WEB **keeps** all eleven **[measured]**, which is why it is the useful
reference here — its numbering is a superset for this class.

**The four the NKJV has and the WEB does not** **[measured]**: Acts 8:37,
Acts 15:34, Acts 24:7, Luke 17:36. Same story from the other side: Textus
Receptus verses **[standard]**.

**The Romans doxology** **[measured]**: the WEB places it at Romans 14:24-26; the
BSB and the NKJV place it at 16:25-27. This is the one genuine *relocation* in
the New Testament rather than a presence/absence question, and it is old: the
manuscript tradition places these verses after 14:23, after 15:33, after 16:23,
in more than one of those positions, or not at all **[standard]** — a crux
discussed in any critical commentary. Our table maps the three verses in both
directions, and a BSB→NKJV mapping composes two moves that cancel.

### 2.2 The Catholic edition is not the WEB plus extra books

This is the finding that surprised us, and it is a live defect in the shipping
app rather than a curiosity.

**[source]** The Catholic edition's own USFM does not contain the book codes
`EST` or `DAN` at all. It contains `ESG` and `DAG`:

```
$ grep -m3 -E '^\\(id|h)' usfmc/43-ESGeng-web-c.usfm
\id ESG
\h Esther (Greek)

$ grep -m3 -E '^\\(id|h)' usfmc/66-DAGeng-web-c.usfm
\id DAG World English Bible (WEB)
\h Daniel (Greek)
```

`ESG` and `DAG` are the standard USFM codes for Greek Esther and Greek Daniel —
formally distinct books, not editions of the Hebrew ones. The runtime supplier
presents both under the familiar English names, which is why the app has been
treating them as the same books all along.

**Greek Daniel** is the Hebrew book plus the Song of the Three Young Men inside
chapter 3, Susanna as chapter 13, and Bel and the Dragon as chapter 14
**[standard]**. The insertion pushes the end of chapter 3 down, and the edition
says so itself, in a footnote printed at the verse **[source]**:

> *Daniel 3:91, footnote:* "Verses 91-97 were numbered 24-30 in the traditional
> Hebrew text of Daniel."

Our derived table maps exactly those seven verses, 3:24-30 → 3:91-97
**[measured]** — arrived at independently by text alignment, then found to be
stated outright by the publisher. Chapter 3 runs to 97 verses in that edition
**[measured, source]**.

**Greek Esther** is a different matter. Its additions are not appended but
interleaved, and the resulting book does not correspond verse-by-verse to the
Hebrew at any point: **[measured]** all ten chapters differ, and the opening
verse is not the same sentence —

| | Esther 1:1 |
| --- | --- |
| WEB | "Now in the days of Ahasuerus (this is Ahasuerus who reigned from India even to Ethiopia…)" |
| WEBC | "[In the second year of the reign of Ahasuerus the great king, on the first day of Nisan, Mordecai…" |

The WEB's opening is the Hebrew book; the Catholic edition opens with Mordecai's
dream, which is Addition A **[standard]**. There is no honest verse mapping
between them, so the table marks the book *incommensurable* and the app is
expected to say so rather than guess.

### 2.3 How the table is derived, and why the obvious rules were wrong

`scripts/gen-versification.py` builds `versification_data.go` from the app's own
cache files. Two of its rules exist only because the naive version was measured
and found wrong:

1. **A move requires a destination the reference does not have.** Matching on
   text similarity alone claimed Mark 7:16 had "moved" to Mark 4:23. Both are
   "if anyone has ears to hear, let him hear" — a formula that recurs, at a verse
   that never moved anywhere. A genuine relocation lands in a *new* slot, which
   is what Romans 16:25-27 is in the BSB and NKJV.
2. **A vacated number must be recorded as new.** WEBC's Daniel 3:24 is the Song
   of the Three; the WEB's 3:24 is Nebuchadnezzar's astonishment, now at 3:91.
   Without recording that the number was reoccupied, mapping WEBC 3:24 back
   answers "3:24, exact" and a round trip silently fails to close.

### 2.4 Validation performed

- **[measured]** All 202 table entries checked against the actual verse text:
  every *moved* pair must be textually the same passage (≥0.85 token similarity
  within an edition, ≥0.30 across editions), every *absent* verse really missing,
  every *extra* verse really present. 0 problems.
- **[measured]** Round-trip property: a reference mapped out and back returns to
  where it started, for every edition pair (`TestMapVerseRoundTrips`).
- **[source]** The Daniel mapping is independently confirmed by the publisher's
  own footnote, quoted above.
- **[measured]** Cross-checked against the UBS/Paratext **`eng`** scheme, chapter
  by chapter, for all 66 books (§2.5).
- **[source]** The Daniel mapping is independently confirmed a *second* time by
  the UBS/Paratext `lxx.vrs`, which states it verbatim:

  ```
  DAG 3:91-97 = DAN 3:24-30
  ```

### 2.5 Against the external standard

`scripts/check-versification-refs.py` compares our editions' chapter lengths with
the UBS/Paratext `eng` scheme. Results **[measured]**:

| edition | chapters differing from `eng` | which |
| --- | --- | --- |
| WEB | 4 | Romans 14, Romans 16, 3 John 1, Revelation 12 |
| BSB | 2 | 3 John 1, Revelation 12 |
| NKJV | 2 | 3 John 1, Revelation 12 |

Three things follow.

**The WEB really is the outlier on Romans.** All three standard schemes — `org`,
`eng` and `vul` — give `ROM … 14:23 15:33 16:27`: chapter 14 ends at 23 and the
doxology is at 16:25-27. The BSB and the NKJV agree with the standard; the WEB
does not. Our reference edition is the unusual one here, which is worth knowing
but does not affect the mapping's correctness — it is why the mapping exists.

**Two deviations are shared by every edition we ship**, so they produce no
inter-edition delta and our table is silent about them:

- *3 John* runs to 14 verses for us, 15 in the scheme. Editions differ on whether
  the closing greeting is one verse or two **[standard]**.
- *Revelation 12* runs to 17 verses for us, 18 in the scheme. Editions differ on
  whether the last sentence closes chapter 12 or opens chapter 13 **[standard]**.

They matter anyway: a reference arriving from *outside* the app — pasted by a
reader, or a link from a Bible that numbers 3 John 15 — can name a verse none of
our editions has. That is a gap in what the app can accept, not an error in the
mapping between our editions.

**Our WEBC conforms to no published scheme, and that is expected.** Its Daniel 3
runs to 97 verses; `eng` says 30, `vul` says 100. It is the English chapter
arrangement carrying the Greek additions — a particular publication, not a
tradition. This is the strongest argument for deriving the table from the text we
actually ship rather than adopting a scheme wholesale: schemes describe
traditions, and a shipped edition need not conform to any of them exactly.

---

## 3. Red letters

### 3.1 The convention

Printing the words of Christ in red is an editorial convention of the modern
era, not a feature of the manuscripts; it is generally credited to Louis Klopsch,
whose red-letter New Testament appeared in 1899 **[standard]**. Nothing in the
Greek marks these words, which is why editions differ — and they do differ, on
questions such as where the red should stop in John 3 (Christ's discourse, or the
evangelist's commentary?), whether sayings *quoted* in the epistles are red, and
how much of Revelation is.

Because it is editorial, red-letter data is a property of an **edition**, not of
Scripture, and cannot be transferred between editions without an argument.

### 3.2 What each edition actually gives us **[measured]**

| edition | words-of-Jesus markup in the publisher's own file |
| --- | --- |
| WEB | 2,059 verses carry `\wj` |
| WEBC | 2,059 verses carry `\wj` |
| **BSB** | **none — 0 verses. The BSB is not a red-letter edition.** |
| NKJV | supplied per verse by API.Bible as `<span class="wj">` |

That the BSB ships no red-letter data at all is a constraint on what "per-edition
accurate" can mean for it, and a decision the project has to take deliberately
rather than by default.

### 3.3 What we currently ship, and its known defect

`red_letter_data.go` is generated from the WEB's `\wj` markers and covers nine
books: Matthew, Mark, Luke, John, Acts, 1 Corinthians, 2 Corinthians, 1 Timothy,
Revelation.

- **[measured]** It reproduces the WEB source exactly: 2,059 verses, 0 missing,
  0 extra, verified by re-deriving from the USFM and diffing.
- **[measured]** It aligns correctly onto the BSB and WEBC by verse number
  (§2 gives the exceptions).

Its defect is **granularity**. The table records *verses*, not spans, so a verse
that mixes narration and speech is reddened whole:

- **[measured]** 657 of 2,059 red verses (32%) contain text outside the `\wj`
  span.
- **[measured]** About 79 of those contain **another speaker's words** in red.
  Matthew 20:22 reddens "We are able" (James and John); Matthew 17:26 reddens
  "From strangers" (Peter); John 8:11 reddens "No one, Lord" (the woman);
  Matthew 22:21 reddens "Caesar's" (the Pharisees).

The remedy is span-level data, and the spans exist: **[measured]** 2,034 of the
2,059 `\wj` spans (98.8%) are found *verbatim* in the app's WEB verse text, 18
more match after punctuation normalisation, and 7 differ because eBible's WEB
revision is not identical to the runtime supplier's. Per-edition accuracy is
therefore achievable for the WEB and WEBC from their own USFM, and for the NKJV
from API.Bible's own spans; the BSB has no source to be accurate *to*.

---

## 4. Sources, exactly

Fetched 2026-08-12/13. SHA-256 given as the first 16 hex digits, enough to
detect a changed file.

| what | URL | bytes | sha256 (16) |
| --- | --- | --- | --- |
| eBible catalogue | `https://ebible.org/Scriptures/translations.csv` | 731,134 | `cb0d7241ec7c1f3d` |
| UBS `eng` scheme | `…/libpalaso/…/Resources/eng.vrs.txt` | 18,787 | MIT |
| UBS `org` scheme | `…/libpalaso/…/Resources/org.vrs.txt` | 11,817 | MIT |
| UBS `vul` scheme | `…/libpalaso/…/Resources/vul.vrs.txt` | 30,174 | MIT |
| UBS `lxx` scheme | `…/libpalaso/…/Resources/lxx.vrs.txt` | 29,247 | MIT |
| WEB, USFM | `https://ebible.org/Scriptures/eng-web_usfm.zip` | 3,244,102 | `d0dab133845cfbf2` |
| WEB Catholic, USFM | `https://ebible.org/Scriptures/eng-web-c_usfm.zip` | 3,082,019 | `4f8235bbcd0927d3` |
| BSB, USFM | `https://ebible.org/Scriptures/engbsb_usfm.zip` | 3,017,011 | `c065fa11decc4160` |
| NKJV verse markup | `https://api.scripture.api.bible/v1/bibles/{id}/verses/{ref}?content-type=html` | — | requires a key |

The eBible identifiers are the third column of the catalogue: `eng-web`,
`eng-web-c`, `engbsb`. The catalogue is the right starting point for any further
edition; do not guess identifiers.

---

## 5. Reproducing this

```sh
# versification table
python3 scripts/gen-versification.py \
    --web  ~/Library/Caches/bibletext/bibletext-web-v2.json \
    --bsb  ~/Library/Caches/bibletext/bibletext-bsb-v3.json \
    --webc ~/Library/Caches/bibletext/bibletext-webc-v2.json \
    --nkjv <a cache file containing the NKJV>       # licensed; never committed

# the pinned consequences
go test -run 'TestMapVerse|TestVerseExistsIn|TestWordsOfChrist' .
```

The cache files are the app's own, written under the platform cache directory or
wherever `BIBLETEXT_CACHE_PATH` points. The NKJV is licensed text: it may be used
to *derive* a table locally and must not be committed.

Regenerate whenever a translation's cache epoch changes (see `cacheEpoch`) or an
edition is added. The tests pin the cases a reader can actually hit, so a
regeneration cannot quietly change the app's behaviour toward someone's link
without a failure.

---

## 6. Where to look for more

External references, with licences established by reading each project's own
LICENSE file. Licence matters here: some of this may be embedded, some may only
be consulted.

- **Paratext / UBS versification schemes** — the industry standard, and the only
  family carrying explicit *mapping* lines rather than only verse counts. This is
  what confirmed our Daniel mapping and located our two standard deviations.
  **MIT** (SIL Global), so embeddable with attribution.

  ```
  https://raw.githubusercontent.com/sillsdev/libpalaso/master/SIL.Scripture/Resources/eng.vrs.txt
                                                              …/org.vrs.txt  …/vul.vrs.txt
                                                              …/lxx.vrs.txt  …/rsc.vrs.txt  …/rso.vrs.txt
  ```

  Note the `.txt` suffix — without it the URLs 404. `sillsdev/machine.py` carries
  a byte-identical copy, i.e. two independent MIT distributions of the same data.
  Format: one line per book of `CHAPTER:LASTVERSE` pairs, plus mapping lines
  containing `=`.
- **CrossWire SWORD "av11n"** canon headers — verses-per-chapter arrays for many
  traditions (`canon_catholic.h`, `canon_catholic2.h`, `canon_vulg.h`,
  `canon_lxx.h`, `canon_synodal.h`, `canon_nrsv.h`, and more) at
  `https://crosswire.org/svn/sword/trunk/include/`. **GPL-2.0-only** —
  consult, do not vendor.
- **Tyndale House / STEPBible**, TVTMS ("Translators Versification Traditions
  with Methodology for Standardisation"), an explicitly academic treatment:
  `https://github.com/STEPBible/STEPBible-Data` → `Versification/`. **CC BY 4.0**
  (the file header also asks that you link back rather than mirror it; a derived
  table with attribution satisfies both).
- **BibleOrgSys** versification XMLs — each data file declares
  `<rights>Public Domain</rights>` even though the surrounding code is GPL-3.0,
  so the *data* is usable: `Freely-Given-org/BibleOrgSys` →
  `BibleOrgSys/DataFiles/VersificationSystems/`.
- **Copenhagen Alliance versification-specification** — **CC BY-SA 4.0**, i.e.
  ShareAlike. Embedding it would put a copyleft obligation on our derived table;
  prefer the MIT sources.
- **ubsicap/versification_json** — MIT, a JSON rendering of the same schemes.
- **Nestle-Aland (NA28) and UBS5 apparatus**, and Metzger's *A Textual Commentary
  on the Greek New Testament*, for the omitted verses and the Romans doxology —
  the authorities behind every **[standard]** claim in §2.
- **The publishers themselves.** The most direct validation we have found came
  from a footnote in the text we ship (§2.2). Read the edition's own front matter
  and notes before reaching for a secondary source.

---

## 7. Anything else of this kind

Data in this repository that makes a claim about the text, and therefore belongs
under the same discipline:

| data | generated from | generator | status |
| --- | --- | --- | --- |
| `versification_data.go` | the app's cache files | `scripts/gen-versification.py` | committed, validated (§2.4) |
| `red_letter_data.go` | WEB USFM `\wj` markers | **none committed** | verified against source, but the generator was never committed — a gap |
| `assets/timings/*.json` | per-verse read-along alignment | `scripts/audio-align` | outside this document's scope |
| `assets/parallels` | cross-reference data | — | provenance not yet recorded here |

The red-letter generator gap is worth closing: the file says "regenerate from the
WEB USFM" but ships nothing to regenerate it with, which is how a derived table
quietly becomes an authored one.
