# Scripture worklist

The tracker for work on the text itself: what the decoders read from each
source, what reaches the reader, and what is wrong with it today. It is
deliberately separate from `docs/BACKLOG.md`, which tracks the app around the
text (notes, panes, platforms, releases).

Three documents, three jobs:

- `docs/SOURCE_FIELDS.md` — the inventory. Every field each source sends and
  whether the app keeps it, skips it for a stated reason, or leaves it open.
- `docs/SOURCE_FIELDS_DECISIONS.md` — the reasoning. For each skip and each
  open row: what is lost, with counts; what keeping it would cost on each
  surface; and the recommendation.
- **This file** — the worklist. What to do, in what order, and where each
  item stands.

Status values are `todo`, `in progress`, `blocked`, `done`, or `decided
against`. When an item is finished, mark it here and fold the outcome back
into `docs/SOURCE_FIELDS.md` so the inventory never lags the code.

## Two rules that shape the order

**Cache epochs are expensive, so batch them.** Bumping an edition's
`cacheEpoch` makes every reader of that edition download the whole text
again. For the public-domain editions that is bandwidth; for the NKJV it is
also quota against a metered monthly allowance. Items that need the same
edition's epoch should ship together in one release, not one at a time.
Items S2 and S14 both need the helloao editions; S16's NKJV half
should wait for another NKJV decoder change to travel with.

**Checks before changes.** The decode-time checks in stage 2 cost no epoch
and no reader-visible change, and every later item is safer with them in
place: they are what would make a bad decode visible instead of silent.

## Stage 1 — the defects

Found while measuring for the analysis. These are not product decisions.

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S1 | paragraph rule ignores curly quotation marks | all | XS | none | todo |
| S2 | Psalm 119 acrostic letters in verse text | WEB, WEBC | S | web, webc | todo |
| S3 | "JESUS" set in capitals in four verses | NKJV | XS | none | blocked |

**S1.** `shouldBreakParagraph` in `reading.go` allows a paragraph break only
when the previous verse ends in a full stop, exclamation mark, question mark,
or a *straight* quotation mark. The text uses curly quotation marks, so a
verse that closes reported speech never starts a new paragraph: 2,094
suppressed breaks in the BSB and 2,272 in the WEB, about a fifth of all
eligible ones. Every edition and every surface is affected, because they all
go through this one function. Paragraphs are computed at render time, so no
epoch. Add the curly closing marks to the suffix test and pin it with a case
that fails on the current code.

**S2.** The WEB and WEB Catholic put the Psalm 119 acrostic letters in the
text. ALEPH is stored as the psalm's superscription and drawn as its title;
the other 21 letters are appended to the last verse of each stanza, so Psalm
119:8 ends "Don't utterly forsake me. BETH". They reach search, sharing,
copying, links, the website and speech. Route `descriptive` runs out of verse
text into the chapter's side-band in `bsbVerseTextMarked`'s content switch,
and ignore a Hebrew subtitle whose whole text is an acrostic letter, in the
`hebrew_subtitle` branch of `decodeHelloAOChapters`.

Take the BSB's one genuine descriptive run with it, since it is the same
code: Zechariah 12:1 is an oracle title, so decide whether it stays inside
the verse as today or becomes a title line. Batch the epochs with S14 if that
item is close behind.

**S3.** Blocked on one look at a printed NKJV. The decoder uppercases
small-caps spans, which is right for the divine name and for the genuine
inscriptions in the canon, but it also produces "call His name JESUS" at
Matthew 1:21, Matthew 1:25, Luke 1:31 and Luke 2:21, because the feed marks
that name with the same style. If print does not set small caps there, add a
narrow exception; if it does, close this as correct.

## Stage 2 — the decode-time checks

No epoch, no reader-visible change, and they make every later item safer.
Ship them as one change so all four editions report alike.

| id | item | editions | effort | status |
|---|---|---|---|---|
| S4 | verify each book's verse count against the feed's own | helloao | S | todo |
| S5 | verify each note's own chapter and verse | helloao | S | todo |
| S6 | count unknown node types and verse-item shapes | helloao | S | todo |
| S7 | census paragraph and character styles, extend the heading families, deny non-Scripture character styles | NKJV | S | todo |
| S8 | cross-check the feed's words-of-Jesus flag against the red-letter table | WEB, WEBC | S | todo |
| S9 | probe whether skipped headings carry notes | NKJV | XS | todo |

**S4.** The feed states a verse count per book, and it is exact: it equals
the decoded count in all 66 BSB books, and exceeds it in exactly three WEB
books by exactly the omitted verses the decoder turned into orphan notes
(Luke by one, Acts by three, Romans by one). So "decoded verses plus distinct
orphan verses equals the feed's count" holds everywhere. Report a book that
falls short by more than its orphans. This is the check that would catch a
silently truncated book, which is the failure a cached Bible hides best.

**S5.** Every note body carries the chapter and verse it belongs to, and it
agrees with the marker's position for all 7,752 notes across the three
editions, with title notes marked by verse zero. Note identifiers run
continuously across a book rather than per chapter, so a future change that
resolved a marker against the wrong chapter would mis-file a note silently.
Log a skip on disagreement rather than failing the fetch.

**S6.** The decoder names three chapter-level node kinds and drops the rest
without counting, and its verse-content switch has no default. Nothing is
lost today, but a feed change would be invisible. Count what is dropped and
log it once per fetch; pin the known sets in the fixture test.

**S7.** Three parts, one change. Extend the NKJV's exact-match skip list to
the full USFM heading families, so a new style cannot fall through to the
prose path and enter verse text. Add a denylist for character styles that are
not Scripture. And census every style seen, logged once per fetch and
asserted in the live canon test. Do not make an unknown style a fetch error:
that would take the edition down rather than logging a line. Note that the
titles-on and titles-off comparison already run only bounds styles the titles
flag strips, so the census is what covers the rest.

**S8.** The feed's flag and the generated table agree on all 2,059 WEB
verses, and no verse currently falls back to whole-verse red. Keep the table
as the source of red and use the flag as a second witness, so drift is
reported at decode time instead of degrading quietly at render time.

**S9.** Five API calls. The NKJV's skipped section headings may contain
cross-reference notes, which are discarded with the block; nothing on disk
can say how many. Count them before any NKJV heading work, since the answer
changes what S16's NKJV half has to carry.

## Stage 3 — small, self-contained wins

| id | item | area | effort | epoch | status |
|---|---|---|---|---|---|
| S10 | website renders Psalm titles | website | S | none | todo |
| S11 | search indexes Psalm titles | search | S | none | todo |
| S12 | regenerate the offline seed from the current decoder | seed | S | none | todo |
| S13 | Android long-press copy keeps poem lines | Android fallback | XS | none | todo |

**S10.** The site renders verses only. The accessor is already exported, so
this is an italic line ahead of the verse loop and one style rule. No URL
changes, so the frozen contract is untouched. Decide at the same time whether
a title should lead a psalm's link preview.

**S11.** A search for "Absalom" does not find Psalm 3, because the index is
built from verse text alone. Index the titles, keyed so a hit opens
somewhere sensible, and decide whether a title hit is labelled as such in the
results.

**S12.** The embedded seed is the four Gospels with no poem breaks, no notes
and no titles. It is used only when a first launch can neither read a cache
nor reach the network, which is exactly when a new reader first sees the app.
Regenerate it, and say in its comment that it must be regenerated whenever
the decoder's output changes.

**S13.** In the Android fallback pane's long-press menu, copying the chapter
keeps poem lines while copying a verse or a paragraph flattens them, with no
stated reason. Make them agree.

## Stage 4 — the larger questions

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S14 | honour the publishers' paragraph breaks | all | M–L | three helloao | todo |
| S15 | section headings behind a setting | BSB, WEBC | M–L | bsb, webc | todo |
| S16 | section headings, NKJV half | NKJV | M | nkjv | blocked on S9 |
| S17 | NKJV italics for supplied words | NKJV | M–L | none | todo |
| S18 | speech reads the Psalm title | all | M | none | todo |
| S19 | mark an omitted verse's gap in the text | all | M | none | todo |
| S20 | show the NKJV's cross references | NKJV | S | none | blocked |
| S21 | website renders the footnote section | website | M | none | todo |
| S22 | in-text footnote markers | all | L | none | blocked |

**S14.** The app makes its own paragraphs from a length-and-punctuation
rule, and both source editions are paragraphed by their publishers, so what
we draw is not what either translation committee set.

The BSB's feed carries that paragraphing in full: Genesis 1 breaks at the
days of creation, John 3 breaks at each turn of the exchange with Nicodemus,
Romans 8 breaks at its argument's joints. Honouring it there is reading a
field the decoder already receives and currently skips.

The WEB is a different problem, and the earlier reading of it was wrong. Its
feed carries 742 break nodes in the whole Bible and none at all in Genesis 1,
John 3 or Romans 8, which looked like an edition that simply does not
paragraph. It is not: the WEB's own published markup carries 9,254 paragraph
markers, plus 23,331 poetry line markers, and its Genesis 1 breaks at the
days exactly as the BSB's does. The supplier is dropping about 92% of the
edition's paragraphing, not the translators.

That changes the fix for the WEB. The app already has the pattern for
recovering structure a runtime feed has flattened: the red-letter tables are
generated offline from the publishers' own markup for exactly this reason,
and guarded at runtime by a rune count and a hash. Paragraph starts can be
carried the same way, as a generated table of verse references per edition,
with no dependence on the supplier ever fixing the feed.

So the shape is: a paragraph-start flag on the verse, fed from the feed where
the feed has it (BSB) and from a generated table where it does not (WEB, WEB
Catholic); the app's rule stays only as the fallback for an edition with
neither. Every surface inherits it through the one shared grouping function.
Fix S1 first or in the same pass. The open design question is unchanged:
whether a source break also resets the length counter. The NKJV needs its own
decoder work to detect a verse that opens a paragraph block, so it follows.

**S15.** 3,091 headings in the BSB, at least one per chapter, and five in the
WEB Catholic of which four are the names by which those passages are known.
Render behind a setting, default off, with a shape guard that drops and
counts any heading over about 120 characters or containing a chapter-and-verse
reference: that rejects exactly one malformed WEB Catholic heading in Greek
Daniel 3 and nothing in the BSB. Build the shared model first, then the
website and styled pane, then the Apple and Android panes, whose verse
indexes must learn to exclude a heading paragraph. That last part is the same
class of change as the selection-over-wash work and wants a device pass.

**S16.** The NKJV's roughly 3,300 headings, after S9 says what the skip is
discarding and after the licensing enquiry names headings. Its epoch should
travel with another NKJV decoder change.

**S17.** Roughly 3,000 spans of italicised supplied words in the New
Testament alone, the King James tradition's own way of telling a reader which
words the translators added. Two steps: generate the offsets table, which
reuses the red-letter generator's fetch, extraction, location and guard and
holds no licensed prose; then design how the shared run type carries two
independent style dimensions, since italics and red letter overlap on the
same characters and there is no coherent fallback for stale offsets. Do not
start the second step until the first is pinned.

**S18.** The text change is trivial; the work is the read-along model, which
has no slot for a title before verse 1. Do it when that model gains one.

**S19.** An omitted verse leaves a hole in the numbering with no explanation
unless footnotes are on, in 34 places across the corpus. Mark the gap only
when footnotes are already on, which extends a decision already taken and
keeps the page identical when they are off.

**S21.** The website renders no footnote section, so the whole apparatus is
absent from the pages: notes, omitted-verse orphans and title notes. The data
fields are already exported; what is missing is a presentation model, because
the site has no settings and therefore no toggle, so the section would have
to be always visible or disclosed without scripting. Decide that first. Ships
after S10, which is the same renderer.

**S22.** In-text footnote markers, the superscript caller at each note's
anchor. The data has been carried since the apparatus landed: every note
stores its caller and its rune anchor, and nothing reads them. The blocker is
Android, whose bridge treats the first non-numeric superscript in a chapter
as the end of the chapter's content, so an in-text marker would clamp
selection and copying for everything after it. Fix that scan first; the
documented order after it is the web reader, then the Apple panes, then the
styled pane, then Android.

**S20.** Blocked on the licensing enquiry about displaying the publisher's
apparatus, which is unsent. All 32,473 NKJV notes are cross references and
the section hides cross references, so they are captured and dark, including
43 attached to Psalm titles. Keep those 43 with the general decision rather
than carving them out.

## Decided against

Recorded here so they are not rediscovered as open questions. The reasoning
is in `docs/SOURCE_FIELDS_DECISIONS.md`.

| item | why |
|---|---|
| poem indent depth | cosmetic; needs a per-line side-band across four surfaces and three epochs, and a one-line poem verse already reads as prose |
| Selah placement | the publisher's placement is the text; normalising it is an editorial act on someone else's edition |
| book and chapter metadata | duplicates the app's own canon, which must also serve editions that name books differently |
| classifying helloao note kinds | only 8% of the BSB's notes are bare citations and none of the WEB's are, so a wording heuristic would risk hiding real glosses to solve very little |
| titles in share text | no whole-chapter share exists to attach one to |
| titles in copy | already correct on the native panes: the clamp bounds the app's own actions, the system's copy takes what the reader selected |
| the Fyne fallback panes' missing title and section | unreachable in shipping builds |
| flattening poem lines in cross-reference snippets | a deliberate one-line preview |
