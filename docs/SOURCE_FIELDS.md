# What each source carries, and what the app keeps

Every field and paragraph kind the decoders receive is listed here with one
of three verdicts:

- **kept** — where it lands in `BibleData` and which surfaces read it;
- **skipped** — dropped on purpose, with the reason;
- **OPEN** — dropped today without a stated reason. Nothing in this list is
  an accident any more, but each is a product decision that has not been
  taken. Taking one means either a reason written here or a change.

The rule this document enforces: a skip that is not explained here is a
defect. When a decoder changes, this file changes with it.

How to re-census the sources:

- helloao editions (WEB, WEB Catholic, BSB): the raw `complete.json`
  captures under `build/biblecache/` (9 Aug 2026), read with any JSON tool.
- NKJV: `TestLiveAPIBibleFullCanon` in `apibible_live_test.go` (opt-in,
  about 200 requests of the monthly 5,000) downloads the whole canon the way
  the app does, checks the counts below, and with
  `BIBLETEXT_FULL_CANON_COMPARE` reproduces an earlier decoded canon byte
  for byte. `TestLiveAPIBibleProbe` is the cheap five-call check.

## The helloao editions — WEB, WEB Catholic, BSB

All three come from `bible.helloao.org` as one `complete.json` per
translation and go through the same decoder (`decodeHelloAOChapters` in
`bsb.go`; the Catholic canon is mapped by USFM id in `catholic.go`). The
struct the decoder reads has exactly these fields: book `id` and `order`,
chapter `number`, chapter `content`, and the chapter's `footnotes` with
`noteId`, `caller`, `text` and `reference`. Anything else in the file is
never unmarshalled.

Census of the captures (chapter-level nodes / verse items):

| edition | verse | heading | line_break | hebrew_subtitle | footnote bodies | wordsOfJesus runs |
|---|---|---|---|---|---|---|
| WEB (66 books) | 31,103 | 0 | 742 | 117 | 1,226 | 2,289 |
| WEB Catholic (73 books) | 35,408 | 5 | 1,045 | 117 | 1,673 | 2,289 |
| BSB (66 books) | 31,086 | 3,091 | 13,894 | 116 | 4,853 | 0 |

| content | verdict | where / why |
|---|---|---|
| verse text (plain strings and `{text}` runs) | kept | `Verse.Text`, joined with single spaces and tidied (`bsbTidySpacing`); read by every pane, search, share, speech, links, the website |
| poetry runs `{text, poem: N}` | kept (presence) | a `"\n"` is written before each poem run that follows earlier content, so every surface draws it as a line; a verse that is one poem run has no break and reads as prose |
| poem indent level `N` | OPEN | only presence is tested; the level (1, 2, and one level 3 in WEB Catholic) is discarded. Keeping it would mean a second indent depth on every pane |
| `{lineBreak: true}` inside a verse (prose lists such as Genesis 10) | kept | a `"\n"` in `Verse.Text` |
| `wordsOfJesus: true` on a run (WEB and WEB Catholic only; the BSB feed carries none) | skipped | the text is kept, the flag is not read. Red letter comes from the span tables generated from the publishers' USFM (`red_letter_web_data.go`, `red_letter_bsb_data.go`), which cover every edition the same way and are guarded by rune count and hash. The flag would be an alternative source for two of the four editions; it is not used because the table already covers them |
| `descriptive: true` on a run (Zechariah 12:1 in the BSB; 21 runs in WEB) | OPEN | text kept, styling flag dropped; nothing renders a descriptive run differently |
| footnote markers `{noteId}` with a chapter-level body | kept | `Verse.Footnotes{Anchor, Text, Caller}`; the anchor is the rune count of the text before the marker; the marker itself adds no characters. Shown in the chapter-bottom section when the Settings toggle is on |
| footnote `caller` (always `+` here) | kept, unread | stored on the note; no surface draws callers (the section numbers notes itself) |
| footnote `reference{chapter, verse}` | skipped | the join is by `noteId`, which is exact; every reference in the captures agrees with the verse the marker sits in (`docs/FOOTNOTES.md`) |
| a marker with no body, or an empty body | skipped | nothing to show |
| a verse node with a marker and no text (Luke 17:36, Acts 8:37, 15:34, 24:7, Romans 16:25; 24 versification gaps in Sirach) | kept | `BibleData.OrphanFootnotes`, so the chapter-bottom section can say why the number is absent; no verse number is drawn |
| `hebrew_subtitle` (the Psalm titles: 117 WEB, 116 BSB; 3 and 36 with notes) | kept | `BibleData.Superscriptions`, drawn as an italic unnumbered line above verse 1 on every reading pane; its notes are keyed "Title" in the section. Render-only: never in `Verse.Text`, so never in search, speech, share, copy or links |
| chapter-level `line_break` (the source's paragraph boundaries) | skipped | the app paragraphs every edition with its own rule (about 320 characters ending at terminal punctuation, `reading.go`) so that all editions paragraph alike. The source boundaries are not honoured on any surface. OPEN only in the sense that honouring them is a possible future choice; today's rule is deliberate |
| chapter-level `heading` (BSB 3,091 — "The Creation"; WEB Catholic 5; WEB 0) | OPEN | skipped whole. The BSB is the edition where this carries real editorial content. Showing headings is a product decision about what the page is |
| any other chapter-level node type | skipped | the decoder names the three kinds it knows and drops the rest; none other was observed in the captures |
| any object inside verse content that is not text, a marker or a line break | skipped | dropped without a count; none observed. A count in the decode log would make a new shape visible |
| Selah | kept as the source has it | in the WEB, Psalm 3:2's "Selah." is its own poem run and so its own line; in the other 73 verses it ends a longer run. In the BSB it is a plain string and joins the end of the last line with a space. The two editions therefore place it differently; nothing normalises this |
| book `name`, `commonName`, `title`; chapter `numberOfVerses`, audio links and timings; the `translation` block | skipped | book names come from the app's own canon tables; the app self-hosts its audio and timings |
| books with `order` outside 1..66 (66-book editions) | skipped | the 66-book canon is the edition's definition |
| the Catholic canon's seven deuterocanonical books, Greek Esther and Greek Daniel | kept | under the Catholic names and order (`catholic.go`); Greek Esther keeps the source's square brackets in the text; the versification note lands in Daniel 3:91's notes. The 66-book features (cross references, red letter, verse of the day) skip these books by design |
| the offline seed `assets/seed/web-gospels.json` | kept as a flattened fallback | the four Gospels with no poem breaks and no notes; used only when nothing else can be loaded |

## NKJV — API.Bible

Source: the `passages` endpoint in ranges of 200 verses, with the `chapters`
endpoint as the fallback (`fetchAPIBible` in `apibible.go`), asking for
`content-type=json`, `include-titles=true`, `include-notes=true` and
`include-chapter-numbers=false`. The wire node the decoder reads has exactly
`name`, `type`, `text`, `items` and the attributes `style`, `number`,
`verseId`, `caller` and `sid`; every other attribute (`id`, `eid`, `closed`,
`strong`, `srcloc`, …) is never unmarshalled.

Why titles are requested: API.Bible counts the Psalm superscription (the
USX `d` paragraph) as a title, and with titles off it is simply absent from
the feed. Titles on also brings the publisher's section headings (`s`) and
the acrostic letters of Psalm 119 (`qa`), which `apiBibleSkipPara` drops.
The whole canon was downloaded both ways with the same decoder: the flag
changes nothing but the titles.

The canon as decoded (verified 5 Sep 2026 against the app's own decode of
23 Aug 2026, byte for byte on every verse):

| measure | value |
|---|---|
| books / verses | 66 / 31,102 |
| verses with a poem line break | 8,349 |
| verses naming the LORD | 5,622 |
| notes (all cross references) | 32,473 |
| Psalm titles | 116 (the 34 untitled: 1, 2, 10, 33, 43, 71, 91, 93–97, 99, 104–107, 111–119, 135–137, 146–150) |

| content | verdict | where / why |
|---|---|---|
| text nodes in a non-skipped paragraph after the first verse marker | kept | `Verse.Text`, keyed by the node's `verseId` when present, else by the running verse. Fragments are concatenated raw: the source carries its own spacing, and any inserted space splits words ("G" + "OD") or detaches punctuation |
| verse marker `number` and `sid` | kept | set the running verse and chapter; a verse range keys under its first number; the `sid` is how one passage chunk splits into chapters |
| the verse marker's own children (the printed "10") | skipped | the app draws its own verse numbers; the subtree is presentation |
| `q*` paragraphs other than `qa` | kept | poetry: a `"\n"` at each paragraph boundary inside a verse |
| `p` and every other non-skipped, non-`q` paragraph style (`m`, `pi`, `nb`, `pc`, …) | kept | prose: a single space where a verse flows across the boundary |
| char `sc` and `nd` | kept, uppercased | small caps and the divine name read as UPPERCASE in plain text, which is what keeps LORD and Lord apart and reassembles "G" + "OD" |
| char `wj` | kept as plain text | the tag is discarded; red letter comes from the generated offsets table (`red_letter_nkjv_data.go`), guarded by rune count and hash, with whole-verse red when the text no longer matches |
| char `it` (the NKJV's italicised supplied words) and `bd` | kept as plain text | italics and bold are not carried on any surface. `it` nested inside `wj` counts as red in the offsets table |
| notes of style `x` / `ex` (the feed's only note kind) | kept, not shown | `Verse.Footnotes` with `Kind` cross-reference, anchored where the note sits. The chapter-bottom section excludes cross-reference notes on purpose (`footnote_section.go`), so these are captured and dark. The print edition's NU-/M-Text apparatus is not in the feed at all (`docs/FOOTNOTES.md`) |
| notes of any other style (`f`, `fe`, …) | kept | same path, shown in the section; proven by fixture only, the live feed carries none |
| a note in a verse whose markup decodes to no words | kept | `OrphanFootnotes` |
| a note that opens a verse | kept, anchored at 0 | the note's sentinel is not text: the poem break or prose space owed at the paragraph boundary is decided on the words alone, so the verse's first word stays first. (The earlier decode wrote the break in front of it: 1,995 poetry verses opened with a blank line. Fixed at cache epoch 2) |
| the `d` paragraph (Psalm superscription), with its notes | kept | `BibleData.Superscriptions`, exactly as for the helloao editions. Read through the same walk as a verse, so char spans and notes are handled alike; attached at the next verse marker, because on the passages endpoint chapter N+1's title is read while the decoder is still in chapter N. A title left over at a chunk's tail is attached only when its own `verseId` names the chapter, never guessed; the live feed carries no `verseId` on titles and re-serves a title with a range that starts at its chapter's first verse, so none is left over. A title's cross-reference note is captured and dark like every other NKJV note |
| `qa` (acrostic letters) | skipped | not Scripture; and the `q` poetry prefix would otherwise claim it. The letters carry a `verseId`, which is why the exact-match skip exists |
| `s`, `s1`–`s4`, `ms`, `ms1`–`ms3`, `mr`, `sr`, `r`, `sp`, `cl`, `cd` | OPEN (skipped) | the publisher's section headings and cross-reference lines, dropped whole, including any note inside the block. Same decision as the helloao `heading` row |
| heading-like styles outside that exact list (`s5`, `ms4`, `mt*`, `mte`, `imt`, `is`, `ip`, `io*`, `sd*`) | skipped by consequence | would fall to the prose path: dropped before verse 1, appended to the verse after it. In the chapters sampled live and the 174 New Testament chapters of the red-letter generator's cache, the paragraph styles that arrive are `s`, `d`, `qa`, `p`, `q1`, `q2` and `pc`; and a leak into verse text anywhere in the canon would have shown in the byte-for-byte comparison with the titles-off decode, which found none |
| char styles whose text is not Scripture (`rq`, `fig`, `va`, `vp`, `w`, `wh`, `wg`) | skipped by consequence | none observed; a leak would appear as changed verse text in the full-canon comparison |
| text or a note before the first verse marker in a non-skipped paragraph | skipped | nothing to key it to; the title is now read before this rule applies |
| whitespace-only text, doubled spaces, empty poem lines | skipped | collapsed by `normalizeVerseSpaces`; an authored blank poem line cannot survive, and none exists in the canon |
| a note count that does not match its sentinels | skipped | the verse (or title) ships with no notes rather than mis-anchored ones |
| intro pseudo-chapters, books outside the 66, verses duplicated across chunk boundaries | skipped | the canon is the 66 books; chunk overlaps keep the first decode |

## Decoded, then not shown or not indexed

| what | where it stops | verdict |
|---|---|---|
| `Verse.Footnotes` — search | the index is built from `Verse.Text` only | kept out on purpose: apparatus is not Scripture |
| `Footnote.Anchor` and `Caller` | stored, no consumer | kept for the section's future in-text markers; the section lists notes without them today |
| cross-reference notes (every NKJV note; title notes included) | excluded by the section | on purpose: the cross-reference panel is the surface for references, and the NKJV's are the publisher's, not the app's |
| helloao note bodies that are themselves references (about a tenth of the BSB's) | rendered in the section like any note | OPEN: they carry no `Kind`, so they render while the NKJV's stay dark; nothing states the asymmetry |
| `Superscription.Text` — search, speech, share, copy, links, AI | never enters `Verse.Text`; the selection verbs clamp above verse 1 | on purpose, pinned by tests |
| `Superscription.Text` and the footnote section — the Fyne fallback panes | not drawn | on purpose: the fallback panes are unreachable in shipping builds |
| footnotes, orphans and titles — the website (`cmd/websitegen`) | not rendered | OPEN: the web reader draws verses only; the section is listed as a planned surface in `docs/FOOTNOTES.md` |
| authored poem breaks — the search index and cards, the website unfurl text, the legacy clean-copy | flattened to spaces | on purpose: one-line contexts |
| authored poem breaks — cross-reference snippets, the Android Fyne long-press menu | flattened | OPEN: nothing states why these two differ from the panes |
| red letter — search cards, verse of the day, cross-reference snippets, share text and image, AI, the legacy Entry pane | monochrome | OPEN: the red is a reading-pane feature today; nothing states whether the secondary surfaces should carry it |
| red-letter precision | whole-verse red when the runtime text no longer matches the table | on purpose: the table is keyed to a text revision and guards itself |
| the source's paragraph boundaries | app-synthesized paragraphs on every surface | on purpose (see the `line_break` row) |

## Open decisions

The OPEN rows above, gathered:

1. Section headings (BSB, WEB Catholic, NKJV) — show them or keep skipping.
2. The source's paragraph boundaries — honour them or keep the app's rule.
3. Poem indent depth — one depth on every pane, or the source's two (three).
4. `descriptive` runs — a style of their own, or plain.
5. Selah — leave each edition's placement, or normalise to its own line.
6. Reference-style helloao notes — render as now, or mark as cross
   references and keep them dark like the NKJV's.
7. The website — verses only, or footnotes and titles too.
8. Poem breaks in cross-reference snippets and the Android long-press menu.
9. Red letter on the secondary surfaces.

Until a decision is written here, the current behaviour stands and is
treated as deliberate; a change to any of them is a change to this file
first.
