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
editorial markup — verse boundaries, words-of-Jesus spans, footnotes — in one
form for every edition, where the runtime feeds do not: the BSB feed has no
words-of-Jesus markup at all, the WEB feeds flag runs, and the NKJV feed nests
char spans. What each runtime feed carries, and what the app keeps of it, is
listed in docs/SOURCE_FIELDS.md.

---

## 2. Versification: verse numbers are not a shared address space

A reference like "Mark 9:44" is not a coordinate. It is a claim about a
particular edition's numbering, and editions disagree — not because anyone is
careless but because they descend from different textual decisions, some of them
centuries old.

The app has to carry references *between* editions in shared links, notes and
highlights written in one translation and read in another, and comparison
views. Getting this wrong does not fail loudly. It shows the reader a different
verse than the one they were sent, which is the worst failure a Bible
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
| BSB | 2,042 verses carry `\wj` in the publisher's official third-printing USFM |
| NKJV | supplied per verse by API.Bible as `<span class="wj">` |

The eBible `engbsb` mirror still contains zero `\wj` markers. It is not the
publisher's current authoritative red-letter source: the BSB's own download does
contain them. The table must therefore be generated from the Berean source, not
inferred from WEB or from the unmarked mirror.

### 3.3 The WEB and WEB Catholic

`red_letter_data.go` is generated from the WEB's `\wj` markers and covers nine
books: Matthew, Mark, Luke, John, Acts, 1 Corinthians, 2 Corinthians, 1 Timothy,
Revelation.

- **[measured]** It reproduces the WEB source exactly: 2,059 verses, 0 missing,
  0 extra, verified by re-deriving from the USFM and diffing.

Both now ship SPAN data, generated by `scripts/gen-web-redletter.py` from those
same `\wj` markers — **[measured]** all 2,059 verses in each edition have
guarded offsets: WEB 2,290 spans and WEBC 2,289 spans.

The runtime supplier's older WEB-family revision differs from the current
eBible source in six verses in both editions (John 3:7; John 4:48; Luke 8:6–8;
Luke 12:51), plus WEBC Mark 9:47's `oF` typo. The generator explicitly records
and shape-checks these boundary recoveries. John 4:48 keeps its narrative lead-in
black, Luke 8:8 keeps the narration between two speeches black, and the other
five recovery shapes are wholly inside `\wj` in the publisher source. WEBC Mark
9:47 is emitted as one semantic span although the source's footnote divides it
into two adjacent `\wj` blocks. Any new or malformed mismatch fails generation.

The generated 2,059-verse marked sets remain as edition-local protection for a
future runtime text change that invalidates an offset fingerprint. Other
editions never consult this fallback.

### 3.3a The NKJV

**[measured]** API.Bible serves the NKJV's own words-of-Jesus marks as
`<span class="wj">`, so the NKJV uses the publisher's judgement rather than the
WEB's, which is what it used before. 2,054 verses, every span located, no misses.

`scripts/gen-nkjv-redletter.py` caches every raw response under
`~/.cache/bibletext-nkjv-html` and never re-fetches: 174 chapters cost 174 calls
once, and a warm cache costs none. **The account's monthly allowance is small —
do not point this at a cold cache casually.** The generated Go file holds
OFFSETS ONLY; no NKJV text is stored anywhere in this repository.

An absent NKJV table entry is authoritative black letter. This matters for nine
verses the WEB marks but the NKJV source does not: 1 Timothy 5:18; Matthew 8:32;
Mark 10:49; Revelation 21:5–8; and Revelation 22:14–15.

### 3.4 The BSB

The [official BSB download](https://berean.bible/downloads.htm) is the authority,
not the unmarked eBible mirror. Its current third-printing USFM contains 1,420
balanced `\wj … \wj*` blocks covering 2,042 verses and 2,294 per-verse runs.

`scripts/gen-bsb-redletter.py` extracts those markers directly and locates their
text in the runtime BSB cache. It first requires an exact punctuation-preserving
match with flexible whitespace, then permits a case-insensitive alphanumeric
match for typography differences such as spacing around em dashes. Generation
fails if any marked run cannot be located.

The generated result is 2,042 verses and 2,294 runs: 2,251 source runs locate
exactly and 43 require typography normalization. There are no hand judgements
and no cross-edition inputs.

The previous generator inferred BSB spans from WEB, BSB punctuation, NKJV verse
numbers, and 52 hand adjudications because it inspected the unmarked eBible
mirror. That table is not retained as an editorial fallback: an absent official
BSB marker now means black letter.

**Guarding.** Every span table records both the rune length and a 64-bit FNV-1a
fingerprint of the exact runtime verse. The accessor refuses offsets unless both
match. This detects same-length supplier edits as well as length changes. A
marked verse with stale offsets degrades to whole-verse red from that edition's
own table; an absent entry remains black.

**Diagnostics switch.** `BIBLETEXT_BSB_RED_LETTER=0|1` changes only BSB span
rendering. Off, publisher-marked BSB verses render whole red for comparison. It
does not disable WEB, WEBC, or NKJV tables and is not a reader preference.

### 3.5 Build-tagged evaluation versions

LSB (`-tags lsb`) and NRSV (`-tags nrsv`) are unlicensed placeholder-only
registrations, not shipped versions, and have no publisher text or red-letter
source configured. They therefore render black in red-letter mode. They do not
borrow WEB's judgement; a version-specific source and generated table are
required before either can display red letters.

## 4. Sources, exactly

Versification sources were fetched 2026-08-12/13; red-letter archives were
re-fetched and verified 2026-08-23. SHA-256 is shown as the first 16 hex digits.

| what | URL | bytes | sha256 (16) |
| --- | --- | --- | --- |
| eBible catalogue | `https://ebible.org/Scriptures/translations.csv` | 731,134 | `cb0d7241ec7c1f3d` |
| UBS `eng` scheme | `…/libpalaso/…/Resources/eng.vrs.txt` | 18,787 | MIT |
| UBS `org` scheme | `…/libpalaso/…/Resources/org.vrs.txt` | 11,817 | MIT |
| UBS `vul` scheme | `…/libpalaso/…/Resources/vul.vrs.txt` | 30,174 | MIT |
| UBS `lxx` scheme | `…/libpalaso/…/Resources/lxx.vrs.txt` | 29,247 | MIT |
| WEB, USFM | `https://ebible.org/Scriptures/eng-web_usfm.zip` | 3,244,742 | `fbe7006864ae34c6` |
| WEB Catholic, USFM | `https://ebible.org/Scriptures/eng-web-c_usfm.zip` | 3,081,452 | `900ac0e7e4372d6a` |
| BSB official third-printing USFM | `https://bereanbible.com/bsb_usfm.zip` | 1,598,046 | `a91f7b6744879814` |
| WEBBE (British ed. with deuterocanon), verse-per-line | `https://ebible.org/Scriptures/eng-webbe_vpl.zip` | 5,350,711 | `7eea2a3362560978` |
| eBible BSB mirror (unmarked; not used for red letters) | `https://ebible.org/Scriptures/engbsb_usfm.zip` | 3,017,011 | `c065fa11decc4160` |
| NKJV verse markup | `https://api.scripture.api.bible/v1/bibles/{id}/verses/{ref}?content-type=html` | — | requires a key |

The eBible identifiers are the third column of the catalogue: `eng-web`,
`eng-web-c`, `engbsb`. The catalogue is the right starting point for any further
edition; do not guess identifiers.

---

## 5. Rendering and reproduction

### Which pane renders what

Five surfaces render scripture: four in-app panes in three markup dialects plus
one widget tree, and the static website generator. All five consult the same
`redLetterRuns` / `redLetterSpansFor` decision (the website through the exported
`RedLetterRuns` wrapper):

| pane | platforms | file |
| --- | --- | --- |
| Apple HTML | iOS, iPadOS, macOS | `reading.go` |
| Android HTML | Android with the Java bridge | `android_chapter_html.go` |
| styled desktop | Windows, Linux | `reading_styled_layout.go` |
| Fyne RichText | Android without the bridge | `reading_mobile_segments.go` |
| static website | generated reader pages | `cmd/websitegen/render.go` |

The styled desktop pane colours whole TOKENS, the other four split at rune
level. Two BSB spans end mid-token (Mark 7:34, Acts 20:35), so that pane paints
3 characters red there that the others leave black. Deliberate, and the only
known cross-platform difference.

### Regenerating the data

```sh
# versification table
python3 scripts/gen-versification.py \
    --web  ~/Library/Caches/bibletext/bibletext-web-v2.json \
    --bsb  ~/Library/Caches/bibletext/bibletext-bsb-v3.json \
    --webc ~/Library/Caches/bibletext/bibletext-webc-v2.json \
    --nkjv <a cache file containing the NKJV>       # licensed; never committed

# red letters — the WEB and WEB Catholic, from their own USFM (no API calls)
python3 scripts/gen-web-redletter.py \
    --web-usfm  <unpacked eng-web_usfm.zip>   --web-cache  ~/Library/Caches/bibletext/bibletext-web-v2.json \
    --webc-usfm <unpacked eng-web-c_usfm.zip> --webc-cache ~/Library/Caches/bibletext/bibletext-webc-v2.json

# red letters — BSB publisher markers (no API calls)
python3 scripts/gen-bsb-redletter.py --usfm <unpacked bsb_usfm.zip>/bsb_usfm \
    --bsb ~/Library/Caches/bibletext/bibletext-bsb-v3.json

# red letters — the NKJV, from API.Bible. CACHED: a warm cache makes ZERO calls.
python3 scripts/gen-nkjv-redletter.py

# the pinned consequences
go test -run 'TestMapVerse|TestVerseExistsIn|WordsOfChrist|TestBSBRedLetter|TestNKJV|TestWEBAndWEBC|TestApplePane|TestAndroidPane|TestStyled|TestFyne' .
```

### If you need to change or undo this

- `bsbRedLetterSpansOn` and `BIBLETEXT_BSB_RED_LETTER=0|1` affect BSB only.
  Off, marked BSB verses use BSB whole-verse red; other editions are unchanged.
- `redLetterSpansFor` in `red_letter_bsb.go` is the single lookup. Adding an
  edition is one `case`; removing one is deleting that case.
- Every table is guarded by rune length and a content fingerprint. If a supplier
  changes the text, the guard refuses the offsets and falls back using that same
  edition's marked-verse set rather than colouring arbitrary words.

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
| `red_letter_data.go` | WEB USFM `\wj` markers | historical verse-set projection | retained for the public WEB verse-level API |
| `red_letter_web_data.go` | WEB/WEBC USFM `\wj` markers | `scripts/gen-web-redletter.py` | committed spans, lengths, and content fingerprints |
| `red_letter_bsb_data.go` | official BSB third-printing USFM `\wj` markers | `scripts/gen-bsb-redletter.py` | committed; all 2,042 marked verses and 2,294 runs located |
| `red_letter_nkjv_data.go` | API.Bible NKJV `<span class="wj">` | `scripts/gen-nkjv-redletter.py` | committed offsets/fingerprints only; no licensed text |
| `scripts/data/bsb-redletter-adjudications.json` | retired derived table | — | historical record; no longer used by generation or runtime |
| `scripts/data/nkjv-wj-verses.json` | retired BSB derivation input | — | historical verse numbers; no longer used by BSB generation |
| `assets/timings/bsb.json`, `web.json` | per-verse read-along alignment | `scripts/audio-align` | timing only; no claim about the text |
| `assets/timings/webbe.json` | per-verse read-along alignment | `scripts/audio-align` | timing, **and** the coverage claim in §8 — a chapter's presence here is what makes the app offer its recording |
| `assets/parallels` | cross-reference data | — | provenance not yet recorded here |

All active span tables now have committed generators. Regeneration is fail-closed:
an unlocatable publisher marker stops generation instead of being replaced by a
different edition's red-letter judgement.

---

## 8. The WEB-Catholic's Greek books and their recording

The app plays a recorded narration for the WEB-Catholic's Greek books — the seven
deuterocanonical books, the Greek Esther, and Daniel 3, 13 and 14 — that is *not*
the David Williams WEB narration and *not* a human reading. It is eBible.org's
synthetic narration of the World English Bible British Edition with Deuterocanon
(WEBBE), mirrored to the project's audio host. Two claims hold that arrangement up.

**The recording's text is the text the app displays. [measured]** The app's WEB
Catholic text comes from helloao (`eng_webc`); the recording was made from
eBible's WEBBE. Those are different publications, so before adopting the audio the
two were compared verse by verse across all 150 chapters — every verse number in
every affected chapter, not a sample:

```
# WEBBE verse-per-line vs the app's own cached WEBC
python3 - <<'EOF'
import json, re, collections
webbe = collections.defaultdict(lambda: collections.defaultdict(dict))
for line in open("vpl/eng-webbe_vpl.txt", encoding="utf-8"):
    m = re.match(r"^(\S+)\s+(\d+):(\d+)\s?(.*)$", line.rstrip("\n"))
    if m: webbe[m.group(1)][int(m.group(2))][int(m.group(3))] = m.group(4)
webc = json.load(open("~/Library/Caches/bibletext/bibletext-webc-v2.json"))["data"]["Verses"]
MAP = {"Tobit":"TOB","Judith":"JDT","Esther":"ESG","Wisdom":"WIS","Sirach":"SIR",
       "Baruch":"BAR","1 Maccabees":"1MA","2 Maccabees":"2MA","Daniel":"DNG"}
EOF
```

Result: the verse numbering agrees exactly, book for book and chapter for chapter,
including the Greek Daniel's 97-verse chapter 3 and its 64- and 42-verse chapters
13 and 14. The only difference anywhere is 24 verse numbers in Sirach that WEBBE
lists and the app's WEBC does not (Sirach 1:5, 1:7, 1:21, 3:19, 10:21, 11:15,
13:14, 16:15, 17:5, 17:9, 17:16, 17:18, 17:21, 18:3, 19:18, 19:21, 20:3, 20:32,
22:9, 23:28, 24:18, 24:24, 25:12, 26:19) — all of them **empty**: they carry
no text in WEBBE either, so nothing is spoken that the reader cannot see. Note the
book code trap: eBible's verse-per-line export calls the Greek Daniel `DNG` while
its own audio filenames call it `DAG`.

**Which chapters the recording covers. [measured]** `assets/timings/webbe.json` is
generated only from chapters that were actually aligned against a downloaded MP3,
and the app treats presence in that table as proof a recording exists
(`recordingHasChapter`). It holds exactly the 150 chapters listed above and no
others, which is what keeps the synthetic voice out of every chapter David Williams
actually read. `TestWEBCRecordingsNeverOverlap` re-checks that from the shipped
table rather than from a hand-written list.

**Source. [source]** eBible.org states of the WEBBE: "The World English Bible
British Edition is not copyrighted. It is in the Public Domain." The audio is
published on the same site as "Free MP3 audio" with no separate recording
copyright and no named narrator; the app therefore credits it as a synthetic voice
rather than as a person, and NOTICE records the same. eBible also states that
"World English Bible" is a trademark, which is why the mirrored files keep eBible's
own names and the app does not present the recording as its own production.
