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

That rule is now stricter. Under the standard at the top of
docs/SCRIPTURE_WORKLIST.md, an explanation is not enough: everything a
publisher sends is captured unless there is an explicit decision to drop it,
and anything the app adds of its own is a defect until decided. Every
"skipped" row below is therefore a candidate for capture, not a closed
question.

Each skip and each OPEN row is analysed, costed and given a recommendation in
docs/SOURCE_FIELDS_DECISIONS.md, which also records three defects the
measuring turned up.

How to re-census the sources:

- helloao editions (WEB, WEB Catholic, BSB): the raw `complete.json`
  captures under `build/biblecache/` (9 Aug 2026), read with any JSON tool.
- NKJV: `TestLiveAPIBibleFullCanon` in `apibible_live_test.go` (opt-in,
  about 200 requests of the monthly 5,000) downloads the whole canon the way
  the app does, checks the counts below, and with
  `BIBLETEXT_FULL_CANON_COMPARE` reproduces an earlier decoded canon byte
  for byte. `TestLiveAPIBibleProbe` is the cheap five-call check.


## A defect in the licensed source: spaces lost at cross references

Ecclesiastes 12:8 reads `“Vanityof vanities,” says the Preacher,` in the app.
The space is missing, and it is missing because the source never sends it.

The chapter arrives as a sequence of text fragments. Between `“Vanity` and
`of vanities,” says the Preacher,` sits a cross-reference note, `style="x"`
with `caller="-"`, which is the marker-less form. The decoder concatenates
fragments raw, deliberately, because inserting a space corrupts the
constructions where a span abuts punctuation or splits a word. So the two
fragments meet with nothing between them.

This is not the decoder dropping a field. The space is absent from the
provider's data in all three of its serializations: the JSON the app reads,
the HTML, and the provider's own plain text, which renders the verse as
`“Vanityof vanities,”`.

### The shape of it

259 verses across 35 books run two words together. Each one has been checked
against the provider's own `/verses` endpoint, and all 259 reproduce there.

| book | verses |
|---|---|
| Job | 38 |
| Isaiah | 37 |
| Jeremiah | 24 |
| Acts | 18 |
| Matthew | 12 |
| Proverbs | 11 |
| Hosea | 11 |
| Psalms | 10 |
| the other 27 books | 98 |

257 of them sit at a cross-reference note and 2 at an italic span. The
complete list, with the current and correct reading of each, is kept outside
the repository because it quotes a licensed text.

### It is specific to this edition, not to the platform

Counted across the whole canon, over every text fragment that directly follows
a note element:

| edition | fragment after a note begins with whitespace |
|---|---|
| NKJV `63097d2a0a2f7db3-01` | 9 of 30,085 — 0.03% |
| KJV `de4e12af7f28f599-01` | 4,208 of 5,801 — 72.5% |

Character spans behave the same in both editions, 42.3% against 41.7%, so this
is not how the platform serializes in general. In the NKJV the space is
normally carried on the fragment *before* the note instead, 83.7% of the time.
The 259 defects are the places where it is carried on neither.

### Why it cannot be repaired locally

The response does not distinguish a marker anchored between two words from one
anchored inside a word. Both arrive as two fragments meeting at a letter.

| verse | fragments | correct reading |
|---|---|---|
| Ecclesiastes 12:8 | `“Vanity` + note + `of vanities,”` | a space belongs here |
| Genesis 42:20 | `young` + note + `est brother` | `youngest`, no space |
| Exodus 12:25 | `j` + note + `ust as He promised` | `just`, no space |
| Ephesians 2:20 | `the chief corner` + span + `stone,` | `cornerstone`, no space |

Inserting a space wherever two word characters meet across a note would repair
Ecclesiastes and corrupt the other three. Fourteen such legitimate joins were
found and confirmed. A dictionary test on the joined form was tried and is not
sound enough to ship: the joined text of Job 22:2 is `Cana`, a place in the
canon, and the available word lists disagree about common inflections.

So there is no local signal, and manufacturing one would be an editorial act on
someone else's edition. The verses stand as the provider sends them.

### What to do about it

Report it upstream. It is the provider's defect, and only the provider can fix
it at the source for every application reading this edition.

Add the check to the decode-time checks so the count is tracked rather than
rediscovered: a note boundary joining two word characters is worth counting on
every download, and a change in the count is the signal that the provider has
acted.

## The helloao editions — WEB, WEB Catholic, BSB

All three come from `bible.helloao.org` as one `complete.json` per
translation and go through the same decoder (`decodeHelloAOChapters` in
`bsb.go`; the Catholic canon is mapped by USFM id in `catholic.go`). The
struct the decoder reads has exactly these fields: book `id` and `order`,
chapter `number`, chapter `content`, and the chapter's `footnotes` with
`noteId`, `caller`, `text` and `reference`. Anything else in the file is
never unmarshalled.

The WEB's feed is the one that arrives impoverished: it sends 742 break
nodes for a whole Bible and none at all in Genesis 1, John 3 or Romans 8,
while the edition's published USFM carries thousands and breaks Genesis 1 at
the days of creation. About 92% of its paragraphing is lost in the supply,
so it is recovered from the publisher's files into a generated table of verse
references (`paragraph_web_data.go`, `scripts/gen-web-paragraphs.py`), the
same answer the red-letter span tables give to the same problem. The BSB's
feed carries its own paragraphing in full and needs no table.

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
| poem indent level `N` | kept | `Verse.PoemLevels`, one entry per line of the verse: 1 opens a Hebrew couplet, 2 answers it, and the Catholic edition sends seven at a third depth. Print sets the answering half indented under the opening one, which is the pairing a reader sees; every line still draws flush left until a surface reads this |
| `{lineBreak: true}` inside a verse (prose lists such as Genesis 10) | kept | a `"\n"` in `Verse.Text` |
| `wordsOfJesus: true` on a run (WEB and WEB Catholic only; the BSB feed carries none) | skipped | the text is kept, the flag is not read. Red letter comes from the span tables generated from the publishers' USFM (`red_letter_web_data.go`, `red_letter_bsb_data.go`), which cover every edition the same way and are guarded by rune count and hash. The flag would be an alternative source for two of the four editions; it is not used because the table already covers them |
| `descriptive: true` on a run (Zechariah 12:1 in the BSB; 21 runs in WEB and in WEBC) | position kept; text kept or lifted, by POSITION | the verse that FOLLOWS one opens a paragraph, which is what restores Psalm 119's twenty-two stanzas. What happens to the TEXT now depends on where the run sits, because the two uses are structurally different: a run in the LAST position labels the stanza that follows (all 21 acrostic letters) and is lifted out into a heading the next verse claims; a run anywhere else titles the verse it opens (the Berean's single one) and stays in the verse. No words are inspected and nothing is dropped — the letters are drawn as headings, as the NKJV's `qa` always has been. See the worklist, S2 |
| footnote markers `{noteId}` with a chapter-level body | kept | `Verse.Footnotes{Anchor, Text, Caller}`; the anchor is the rune count of the text before the marker; the marker itself adds no characters. Shown in the chapter-bottom section when the Settings toggle is on |
| footnote `caller` (always `+` here) | kept, unread | stored on the note; no surface draws callers (the section numbers notes itself) |
| footnote `reference{chapter, verse}` | skipped | the join is by `noteId`, which is exact; every reference in the captures agrees with the verse the marker sits in (`docs/FOOTNOTES.md`) |
| a marker with no body, or an empty body | skipped | nothing to show |
| a verse node with a marker and no text (Luke 17:36, Acts 8:37, 15:34, 24:7, Romans 16:25; 24 versification gaps in Sirach) | kept | `BibleData.OrphanFootnotes`, so the chapter-bottom section can say why the number is absent; no verse number is drawn |
| `hebrew_subtitle` (the Psalm titles: 117 WEB, 116 BSB; 3 and 36 with notes) | kept | `BibleData.Superscriptions`, drawn as an italic unnumbered line above verse 1 on every reading pane; its notes are keyed "Title" in the section. Never in `Verse.Text`; searched (a verse-0 hit) and spoken (the read-along's verse-0 row, S18); still absent from share, copy and links |
| chapter-level `line_break` (the source's paragraph boundaries, and its stanza breaks — the feeds send `\b` this way too) | kept | the verse that follows one is marked `Verse.ParaStart`, and `groupVersesIntoParagraphs` opens a paragraph there. It is now the ONLY thing that opens one: the app's character-count rule is gone, so a chapter the publisher left unbroken stays unbroken, which is what a poem with no stanza break is in print |
| chapter-level `heading` (BSB 3,091 — "The Creation"; WEB Catholic 5; WEB 0) | kept, and drawn | `BibleData.Headings`, each named against the verse it stands above, and each also opening a paragraph as it does in print. Never in `Verse.Text`, so it cannot reach search, speech, sharing or a link. Every reading surface draws it — the Apple panes, the Android bridge, the styled desktop pane and the website — through one shared block model (`chapter_blocks.go`), which decides a chapter's order once as heading-then-paragraph and hands every renderer the same answer. It draws unconditionally: no Settings toggle, and no length guard, so the one 510-character WEB Catholic heading in Greek Daniel 3, which fuses a title to a translator's note, is drawn whole |
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
changes nothing but the titles. That comparison bounds only the styles the
flag strips; a style the feed sends either way would appear in both decodes
alike, which is what the style census in docs/SOURCE_FIELDS_DECISIONS.md is
for.

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
| `p` and every other non-skipped, non-`q` paragraph style (`m`, `pi`, `nb`, `pc`, …) | kept | two things: a single space where a verse flows across the boundary, and the block's first verse marked `Verse.ParaStart`, so the app paragraphs where the publisher does. Poetry blocks are excluded from the second, since a `q` block is a line inside a paragraph and marking those would make every line of a psalm its own paragraph |
| char `sc` and `nd` | kept, as offsets | `Verse.SmallCaps`, rune offsets into the text. The span used to be folded to UPPERCASE in the stored text, which kept LORD and Lord apart and reassembled "G" + "OD" but put a spelling no edition prints into everything that left the app. The publisher's own characters are stored now and the small capitals are drawn from the offsets, rune for rune, so the distinction is carried by the letterforms as the publisher carries it. The two conventions share one field: nothing records afterwards which span was `sc` and which was `nd` |
| char `wj` | kept as plain text | the tag is discarded; red letter comes from the generated offsets table (`red_letter_nkjv_data.go`), guarded by rune count and hash, with whole-verse red when the text no longer matches |
| char `it` (the NKJV's italicised supplied words) | kept | `Verse.Supplied`, as rune offsets into the text — 17,940 spans across 12,459 verses. It is the edition's own disclosure of where a translator added a word for English sense, and it was being flattened away. Offsets, never characters, so the text is byte-identical whether it is captured or not. `bd` (bold) is still flattened; the feed's New Testament carries 30 |
| notes of style `x` / `ex` (the feed's only note kind) | kept; shown only behind the `nkjvxrefs` build tag | `Verse.Footnotes` with `Kind` cross-reference, anchored where the note sits, and each `ref` tag's target id kept as a rune span into the note's text (`Footnote.Refs`) — 89% of the citations; the parenthesised "compare" citations arrive untagged and stay words. The chapter-bottom section excludes cross-reference notes on purpose (`footnote_section.go`); the cross-references panel leads with them, one row per note verbatim, when the edition's flag is on (`docs/SCRIPTURE_WORKLIST.md`, S20). The print edition's NU-/M-Text apparatus is not in the feed at all (`docs/FOOTNOTES.md`) |
| notes of any other style (`f`, `fe`, …) | kept | same path, shown in the section; proven by fixture only, the live feed carries none |
| a note in a verse whose markup decodes to no words | kept | `OrphanFootnotes` |
| a note that opens a verse | kept, anchored at 0 | the note's sentinel is not text: the poem break or prose space owed at the paragraph boundary is decided on the words alone, so the verse's first word stays first. (The earlier decode wrote the break in front of it: 1,995 poetry verses opened with a blank line. Fixed at cache epoch 2) |
| the `d` paragraph (Psalm superscription), with its notes | kept | `BibleData.Superscriptions`, exactly as for the helloao editions. Read through the same walk as a verse, so char spans and notes are handled alike; attached at the next verse marker, because on the passages endpoint chapter N+1's title is read while the decoder is still in chapter N. A title left over at a chunk's tail is attached only when its own `verseId` names the chapter, never guessed; the live feed carries no `verseId` on titles and re-serves a title with a range that starts at its chapter's first verse, so none is left over. A title's cross-reference note is captured and dark like every other NKJV note |
| `qa` (acrostic letters), `s`/`s1-4`, `ms`/`ms1-3`, `mr`, `sr`, `r`, `sp`, `cl`, `cd` — the publisher's section headings, major-section headings, parallel-passage reference lines and acrostic letters | kept, and drawn | captured into `BibleData.Headings` with the publisher's own style name, so a section head can be told from an acrostic letter without guessing — 2,721 of them, 2,674 section heads and 47 acrostic letters. They were once dropped whole; the text is kept now, and so is any note inside the block, which rides on the heading (`Heading.Footnotes`) instead of landing on whichever verse happened to be current. Those notes have no consumer yet. The position also opens a paragraph, because in print a heading always begins a new unit and an acrostic letter marks a stanza. This feed sends no blank-line instruction at all, so without it a chapter of poetry would arrive with nothing to break it — it is what takes the NKJV from 78% of chapters following its publisher to 98%. Drawn on every surface through the same shared block model as the helloao `heading` row, and on the same terms: no toggle, no length guard |
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
| footnotes, orphans and titles — the website (`cmd/websitegen`) | rendered, always visible | the section follows the chapter outside the article, ordered and keyed by the same exported row the apps use (`ChapterFootnoteEntries`), so it cannot drift; the site has no settings, so there is no toggle to honour. The licensed edition contributes nothing: its apparatus is cross references, and it has no chapter page here at all |
| the hole an omitted verse leaves | marked, on the footnotes toggle | a muted "[36]" where the verse would stand, on every reading surface, from an offline table (`omitted_verses_data.go`) rather than from the numbering; never a superscript, so no native verse index can read it as a verse; stripped on every outbound path. The Berean's sixteen carry no note at all, which is why the table exists |
| authored poem breaks — the search index and cards, the website unfurl text, the legacy clean-copy | flattened to spaces | on purpose: one-line contexts |
| authored poem breaks — cross-reference snippets, the Android Fyne long-press menu | flattened | OPEN: nothing states why these two differ from the panes |
| red letter — search cards, verse of the day, cross-reference snippets, share text and image, AI, the legacy Entry pane | monochrome | OPEN: the red is a reading-pane feature today; nothing states whether the secondary surfaces should carry it |
| red-letter precision | whole-verse red when the runtime text no longer matches the table | on purpose: the table is keyed to a text revision and guards itself |
| the source's paragraph boundaries | nowhere — they reach every surface | shown. Every surface used to draw app-synthesized paragraphs instead; the character-count rule that made them is gone, and a paragraph now opens only where the publisher's own mark does (see the `line_break` row) |

## What the feed sends that we do not read, and what watches it

Every field above is classified, but a classification is a statement about the
feed as it was measured. Four decode-time checks now hold that statement to the
feed as it arrives (docs/SCRIPTURE_WORKLIST.md, Stage 2). None of them changes a
decode, fails a fetch, or drops anything; each is silent unless it has something
to say.

| check | what it watches | where |
|---|---|---|
| verse counts | decoded verses + distinct omitted verse numbers against the feed's own `totalNumberOfVerses`, per book | `verse_count.go` |
| feed census | chapter-level node types, and verse-content item shapes that reach no case in the content switch | `helloao_census.go` |
| note references | each note body's stated chapter and verse against the position of the marker that pointed at it | `helloao_checks.go` |
| words of Jesus | the feed's `wordsOfJesus` flag against the generated red-letter table, both directions | `helloao_checks.go` |
| NKJV styles | paragraph and character styles the decoder has no considered answer for, split by whether the block carried text | `apibible_styles.go` |

Two things this made explicit that the inventory above had only implied.

`wordsOfJesus` (WEB and WEB Catholic, 2,289 items) was not read anywhere in the
app before this. That is not a gap in the red-letter feature: red comes from
`red_letter_web_data.go`, generated offline from eBible's own `\wj` markers,
which carries rune offsets the flag cannot and ships with the rune counts and
hashes that catch runtime text drift. The flag is redundant as a SOURCE. It is
now read as a second, independent witness, and it agrees with the table on all
2,059 verses.

The NKJV's section headings carry no notes — probed live across the five
passages most likely to have them, 23 headings, none with a note, styles `s` and
`qa` only. Recorded here so the question is not asked a third time; the scope is
a five-passage sample, not the canon.

## Open decisions

The OPEN rows above, gathered. The first two have been answered; they stay
listed, with the answer, because the answer is part of the record.

1. Section headings (BSB, WEB Catholic, NKJV) — show them or keep skipping.
   TAKEN: captured on all four editions and shown on every surface,
   unconditionally.
2. The source's paragraph boundaries — honour them or keep the app's rule.
   TAKEN: honoured; the app's own rule was removed rather than kept as a
   fallback.
3. Poem indent depth — one depth on every pane, or the source's two (three).
4. `descriptive` runs — a style of their own, or plain. PARTLY TAKEN: the
   acrostic letters are lifted out of verse text and drawn as stanza headings
   (S2). The Berean's Zechariah 12:1 oracle title stays inside its verse, on
   the ground that moving it takes translated words out of a verse on a
   judgement call; what remains open is only whether it should eventually be
   drawn as a title line.
5. Selah — leave each edition's placement, or normalise to its own line.
6. Reference-style helloao notes — render as now, or mark as cross
   references and keep them dark like the NKJV's.
7. The website — verses only, or footnotes and titles too.
8. Poem breaks in cross-reference snippets and the Android long-press menu.
9. Red letter on the secondary surfaces.

Every one of these is analysed and answered in
docs/SOURCE_FIELDS_DECISIONS.md; the answers are folded back into this file
as they are taken. Until then the current behaviour stands and is treated as
deliberate; a change to any of them is a change to this file first.
