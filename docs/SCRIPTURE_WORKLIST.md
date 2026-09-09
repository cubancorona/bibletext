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

## The standard this work is held to

It governs every item below, and it is stricter than the one the earlier
entries were written to.

**Nothing is dropped without an explicit decision, and nothing is added at
all.** Everything a publisher sends is captured and kept. Where something is
currently dropped or currently invented, that is a defect to address rather
than a position to defend.

Three consequences.

**Capture is not a product decision.** A thing is decoded and kept because the
publisher sent it. Whether it is then SHOWN is a separate question, taken
afterwards and on its own merits. "We skip it because we would not know where
to draw it" is no longer a reason to skip it.

**Anything the app adds is a defect until it is agreed.** Not only invented
structure, but inserted characters, synthesised whitespace, recased letters,
and emphasis painted onto words a publisher did not mark. The practical test
is whether it can leave the page: if it can be copied, shared, spoken,
searched or put in a link, it is text, and text must be the publisher's.

**A stated reason is not a decision.** Several rows in
docs/SOURCE_FIELDS.md read "skipped, on purpose". Each of those now needs an
explicit decision or a change, and is listed for that rather than treated as
closed.

The drawn page is outside this. The verse numbers the app prints, the note
chrome, the highlight wash and the footnote section's own furniture are the
app's and always were. The line is whether the thing can escape into text —
and several of them do, which is a defect rather than an exemption.

The full accounting against this standard is docs/ADDITIONS_AND_DROPS.md: what
the app adds that no publisher supplied, what it drops that a publisher sent,
which of those has a recorded decision, and which has none.

## Two rules that shape the order

**Cache epochs are expensive, so batch them.** Bumping an edition's
`cacheEpoch` makes every reader of that edition download the whole text
again. For the public-domain editions that is bandwidth; for the NKJV it is
also quota against a metered monthly allowance. Items that need the same
edition's epoch should ship together in one release, not one at a time.
Items S2 and S14 both need the helloao editions. S16's NKJV half was to wait
for another NKJV decoder change to travel with; it needed no epoch of its own
in the end, because capture had already spent one and drawing spends none.

**Checks before changes.** The decode-time checks in stage 2 cost no epoch
and no reader-visible change, and every later item is safer with them in
place: they are what would make a bad decode visible instead of silent.

## The leaks, and how the last one closed

Text leaving the app had the reading surface's own characters in it. Three
paths are fixed: the assistant received the raw selection and sent it to an
external provider, the desktop pane's Copy put its superscript verse numbers
on the clipboard, and the website joined every number to its verse with a
no-break space that a browser copy carried away. One cleaner now serves every
outbound path, and the native panes no longer put a no-break space in the text
at all, since the system's own Copy reads the storage directly and cannot be
reached.

THE LAST LEAK could not be fixed by stripping, and was not. In the presented
reporter layout a paragraph opened with an em-space and an en-space, written as
literal characters because the platform HTML importers ignore the CSS that
would indent it. The app's own verbs stripped them; the system's Copy on a phone
in landscape did not, so a paragraph copied out of the landscape reader began
with two spaces nobody typed. Removing them meant indenting some other way, and
that is what shipped, on both platforms it needed: a first-line head indent
applied to the imported paragraph style on the Apple panes, and on Android a
private-use marker rune that the bridge deletes as it imports, replacing it with
a real leading-margin span. No indent characters are written into the text on any
surface now. The outbound cleaner still maps the pair away, which costs nothing
and covers text that was captured before the change.

## Stage 1 — the defects

Found while measuring for the analysis. These are not product decisions.

All three are closed. S1 is done: the rule ended a paragraph on a typographic
closing quotation mark as well as a typewriter one. S14 is done with it, and
supersedes S1 entirely: every edition paragraphs where its publisher does, and
the rule S1 mended was then removed rather than kept as a fallback, so nothing
is left for it to run in. S3 turned out not to be a defect. S2 is done, and the
rule it needed turned out to be about POSITION rather than about words — see
below.

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S1 | paragraph rule ignores curly quotation marks | all | XS | none | done |
| S2 | Psalm 119 acrostic letters in verse text | WEB, WEBC | S | web, webc | done |
| S3 | "JESUS" set in capitals in four verses | NKJV | — | none | closed, not a defect |

**S1.** The paragraph rule allowed a break only when the previous verse ended
in a full stop, exclamation mark, question mark, or a *straight* quotation
mark. The text uses curly quotation marks, so a verse that closed reported
speech never started a new paragraph: 2,094 suppressed breaks in the BSB and
2,272 in the WEB, about a fifth of all eligible ones. Every edition and every
surface was affected, because they all went through this one function.
Paragraphs are computed at render time, so no epoch. The curly closing marks
were added to the suffix test and pinned with a case that failed on the old
code; S14 then took the rule away altogether, so neither the function nor the
test remains.

**S2. DONE**, at web and webc epoch 9. The WEB and WEB Catholic put the Psalm
119 acrostic letters in the text: ALEPH stored as the psalm's superscription
and drawn as its title, the other 21 appended to the last verse of each stanza,
so Psalm 119:8 ended "Don't utterly forsake me. BETH". They reached search,
sharing, copying, links, the website and speech.

**The rule is POSITION, not words**, and that was the thing worth finding. Both
editions and the Berean send these as `descriptive` runs, so a rule that treated
every descriptive run alike would have taken the Berean's Zechariah 12:1 —
"This is the burden of the word of the LORD concerning Israel." — out of its
verse too, which is scripture and not a label. The feeds distinguish them
structurally:

- an acrostic letter is the LAST item of its verse, and labels the stanza that
  FOLLOWS (verified: all 21, in both editions);
- Zechariah 12:1's oracle title is the FIRST item, followed by a line break and
  then the verse proper, and titles the verse it opens.

So a trailing descriptive run is lifted out and becomes a heading the next verse
claims; a descriptive run anywhere else stays exactly where it is. No content is
inspected, no list of Hebrew letter names is needed, and the Berean is untouched
— which is also why it owes no epoch.

ALEPH needed its own rule, because it arrives as a `hebrew_subtitle` rather than
as a descriptive run — which is why these editions carried 117 subtitles to the
Berean's 116. A subtitle whose whole text is a single all-capital word is an
acrostic letter and not a title. That is exact rather than approximate: across
the 350 subtitles the three editions send, ALEPH is the only one that matches,
and every real title is a sentence.

Nothing is dropped. The 22 letters are drawn as stanza headings, which is what
they are and what the NKJV decoder has always done with the equivalent (`qa`).
Psalm 119 now yields exactly 22 headings and no superscription in both editions,
and no verse of it ends in a letter.

**The Berean's Zechariah 12:1 stays in its verse**, which also answers open
decision 4 in docs/SOURCE_FIELDS.md for the case that actually occurs. Moving it
would take translated words out of a verse on a judgement call, and the
publisher includes it in verse 1 while styling it as a title. It is still worth
revisiting deliberately; it is not worth doing as a side effect of an acrostic
fix.

**S3.** CLOSED, not a defect. The publisher marks the name with the
small-caps style at those four verses and means to: its own web edition sets
them in small caps. The app had no small caps anywhere, and folded a
small-caps span to uppercase in the stored text, which is the standard
plain-text rendering of small caps and the same treatment that produced LORD
for the divine name. So "call His name JESUS" was a faithful flattening of
what the publisher set, not a decoder fault.

What remained was a display question rather than a correctness one, and it was
not on this list because it is not about the text. The reasoning for leaving it
was that true small caps could be drawn only by keeping the text in mixed case
and applying the style at render time, that the uppercase WAS the text, and
that changing it traded a typographic gain for a change to four pipelines. That
reasoning turned out to have a way past it, and S23 took it: the letterforms are
substituted into the DRAWN runs and the stored text keeps the publisher's own
characters, so nothing downstream changed at all. These four verses are set in
small capitals now like every other marked span.

## The paragraph rule is gone

The app no longer invents paragraphs. The character-count rule — a break after
320 characters at the next sentence end — came in with the first commit, when
the only source served bare verses and there was nothing else to go on, and no
printed or digital edition sets text that way. It has been removed rather than
kept as a fallback: a fallback fires only where a publisher deliberately left a
passage unbroken.

Two things replaced it, beyond the paragraph marks the feeds already carry.

The NKJV's skipped headings now open a paragraph BY POSITION. Their text is
still dropped, because it is not Scripture, but in print a heading always
begins a new unit and an acrostic letter marks a stanza. That feed sends no
blank-line instruction at all, so without this a chapter of poetry arrived
with nothing to break it. It takes the NKJV from 78% of chapters following its
publisher to 98%.

The helloao `descriptive` runs do the same. In the Psalms these are the
acrostic letters, which arrive at the end of the last verse of the previous
stanza, so the verse that follows one opens a stanza. This is what restores
Psalm 119's twenty-two stanzas in the WEB, where they had become a single
176-verse paragraph.

Where the editions now stand, measured over whole Bibles:

| edition | chapters following the publisher |
|---|---|
| BSB | 1,186 of 1,189 (99.7%) |
| NKJV | 1,163 of 1,189 (97.8%) |
| WEB Catholic | 1,202 of 1,328 (90.5%) |
| WEB | 1,065 of 1,189 (89.6%) |

The rest are poetry the publisher set continuously — Psalm 78, Psalm 18,
Proverbs 14, Job 31 — where one paragraph is the honest rendering and a
character count was never anything but noise. The one consequence worth
watching is that a note in such a chapter raises a single band for the whole
of it, because bands are reserved per paragraph.

## Stage 2 — the decode-time checks

No epoch, no reader-visible change, and they make every later item safer.
Shipped as one change, so all four editions report alike.

| id | item | editions | effort | status |
|---|---|---|---|---|
| S4 | verify each book's verse count against the feed's own | helloao | S | done |
| S5 | verify each note's own chapter and verse | helloao | S | done |
| S6 | count unknown node types and verse-item shapes | helloao | S | done |
| S7 | census paragraph and character styles, extend the heading families, deny non-Scripture character styles | NKJV | S | done — the census answered the other two, and the answer was no change |
| S8 | cross-check the feed's words-of-Jesus flag against the red-letter table | WEB, WEBC | S | done |
| S9 | probe whether skipped headings carry notes | NKJV | XS | done — answered |

**What the checks found when they first ran.** Every measurement this section
predicted was reproduced, which is the outcome worth recording: the identities
held, so the checks now stand as guards rather than as claims.

- S4: decoded verses plus distinct omitted verse numbers equals the feed's own
  total for all 205 books of the three editions, exactly. It also holds per
  CHAPTER against the `numberOfVerses` on the chapters[] wrapper, which this
  section had not noticed — chapter level is strictly better, because it
  localises a shortfall instead of leaving it somewhere in a 1,533-verse book.
- S5: 7,752 notes audited (BSB 4,853, WEB 1,226, WEB Catholic 1,673) and not
  one sits at a verse it does not name.
- S6: nothing is being dropped today. All three feeds send exactly four
  chapter-level node kinds — verse, line_break, heading, hebrew_subtitle — and
  seven verse-content item shapes, every one of which reaches a case. The
  census reports zero, which is the correct baseline: the hole was real, and
  nothing had fallen through it yet.
- S8: the feed's flag and the generated table agree on 2,059 verses in both
  the WEB and the WEB Catholic, in both directions, with no exceptions.
  Worth being clear about what this does NOT mean: red has never come from the
  feed. It comes from `red_letter_web_data.go`, generated offline from
  eBible's own `\wj` markers, and `wordsOfJesus` was not read anywhere in the
  app until this change. The flag's value is being a SECOND, INDEPENDENT
  derivation — eBible's USFM against helloao's own marking — so a divergence
  between two upstreams now reports itself instead of sitting unnoticed.
- S9: ANSWERED. Five passages chosen where a heading was most likely to carry
  a note (Matthew 3, Mark 1, 2 Chronicles 1, Psalm 119, Hebrews 1) returned 23
  headings, of which NONE carries a note. The styles seen were `s`×17 and
  `qa`×6; no parallel-passage `r` heading appeared at all, which is itself the
  answer to why none carried a reference. This is a five-passage sample, not
  the canon, and that is the honest scope of it — but the question was whether
  anything was being lost, and in the places most likely to lose it, nothing
  is. The probe is `TestLiveNKJVHeadingNotes`, key-gated, about five calls.

**S7's other two parts turned out to need no change, and the census is what
proved it.** Extending `apiBibleSkipPara` moves any block of that style off the
prose path — its words leave `Verse.Text` and it becomes a `Headings` entry that
`chapter_blocks.go` draws on all five surfaces — so it is a decoded-text change,
a cache epoch, and a re-download of a LICENSED edition against a metered quota.
Doing that blind, to guard against styles that might not exist, would have been
an expensive way to change nothing. So the census shipped first and was then run
over the whole canon (`BIBLETEXT_LIVE_FULL_CANON=1 BIBLETEXT_STYLE_INVENTORY=1`,
about 200 calls, 17 seconds).

The NKJV feed sends exactly seven paragraph styles and six character styles:

    paragraph  q2 21,543 · p 8,668 · q1 3,318 · s 2,822 · d 116 · qa 66 · pc 18
    character  it 18,375 · sc 7,085 · wj 3,539 · qs 74 · bd 47 · sls 4

Every one already has a considered answer, and no block of any style arrived
empty. NONE of the heading families the skip list would have gained occurs at
all — no `ms*`, `mr`, `sr`, `r`, `sp`, `cl`, `cd`, `s1`-`s5`, `sd`, `mt`, no
introduction or back-matter family. Neither does a single non-Scripture
character style: no `fig`, no `xt`/`xo`, no `rq`, no `va`/`vp`. Both extensions
would have been no-ops. The right change was no change: no epoch, no
re-download, and the census left in place as the standing guard for the day the
feed does send one.

One style was genuinely new: `sls`, a passage in a secondary language — the
Aramaic of Daniel 2:4b-7:28 (×3) and one span in Zechariah. The census found it
on its first run, which is the check doing what it was built for. It is walked
transparently, and that is right: those words are the text, whatever language
the translators rendered them from.

`it` at 18,375 spans is worth knowing: the NKJV marks its SUPPLIED words with
`it`, not with `add`, and `spanSentinels` already maps it to the supplied-word
span. `add` never appears.

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

**S9.** Five API calls. The NKJV's section headings may contain cross-reference
notes; nothing on disk can say how many. They were discarded with the block when
this was written, and the count was wanted before any NKJV heading work because
it changed what S16's NKJV half had to carry. That is settled — a note inside a
heading is now kept on the heading itself — so what remains is the count, which
is still unknown and still five calls away.

## Stage 3 — small, self-contained wins

| id | item | area | effort | epoch | status |
|---|---|---|---|---|---|
| S10 | website renders Psalm titles | website | S | none | done |
| S11 | search indexes Psalm titles | search | S | none | done |
| S12 | regenerate the offline seed from the current decoder | seed | S | none | done |
| S13 | Android long-press copy keeps poem lines | Android fallback | XS | none | done |

**S10. DONE.** An italic unnumbered `p.pst` ahead of the block loop, one style
rule, and two adjacent-sibling selectors. 116 titled psalms on each of the WEB,
WEBC and BSB; none outside the Psalter; Psalm 119 correctly has no title and 22
acrostic headings instead. No URL changes and no new files, so the frozen
contract and publish-site.sh's page counts are untouched.

Two things worth recording. The existing chapterBody golden survives byte for
byte because it is called with a nil BibleData, which yields an empty title and
skips the line — that nil-safety is what made this a safe change rather than a
golden rewrite. And putting a first child into `.text` silently broke two
`:first-child` rules: the first verse would have gained a reporter indent and a
note chip on verse 1 would have lost its margin, so both selectors name the
title case now.

**The link preview does NOT lead with the title.** WEB titles run to 224
characters (Psalm 60) and 211 (Psalm 18) against a ~200-character preview cap,
so a shared link to those psalms would unfurl with nothing but musical
directions and ascription and no scripture at all. About ten psalms would be
degraded to buy a small gain on the other hundred-odd, and the preview is the
one place the words have to do the work. `og:title` already carries the citation.

**Still open, and now visible on every BSB psalm page:** the title is drawn
ABOVE the publisher's section heading, while the feed sends heading, then
subtitle, then verse 1. All four app panes already draw the title first, so the
site is propagating an existing decision rather than inventing one — but it is a
decision nobody took deliberately. Fixing it means teaching the shared block
model about the title, which is S16-sized: the styled pane draws the title as
reserved geometry rather than as a block.

**S11. DONE.** A title hit is keyed to its CHAPTER at verse 0 — the key
`footnoteEntryKey` already uses for a superscription's notes — and carries NO
text on the Verse. That is the load-bearing part: a Verse whose Text held a
title could be shared, copied or sent to an assistant as though a translator had
written it as a verse, and the whole reason titles live outside `Verse.Text` is
that they must never be mistaken for one. The card reads the title back to draw
it.

Opening one needed no change: `openSearchResultRange` already guards its
highlight with `if verse.Verse > 0`, so a title hit opens the psalm at the top,
which is where the title is.

**A title hit is labelled** — a muted "Title" beside the reference, the shape
notes_browse.go already uses for a fact about which passage a row is. Without it
a row whose reference carries no verse number reads as a rendering fault, and
the "N matches" count silently includes rows that are not verses.

**Section and acrostic headings stay OUT.** The Berean carries 3,091 section
headings against 116 titles and the World English carries none at all, so
heading search would exist on two editions of four and change shape with the
version picker. The heading vocabulary is also the verse vocabulary — 161 BSB
headings contain "Jesus", 145 "God", 119 "Israel" — so those rows would compete
with the verses they name for the result cap.

**S12.** DONE. The embedded seed was the four Gospels with no poem breaks, no
notes and no paragraphing — a snapshot of a decoder several changes old, shown
at exactly the moment a new reader first sees the app. Regenerated from a
current decode: of its 3,778 verses, 76 now carry authored poem lines, 214
carry the translators' notes and 1,409 open a publisher's paragraph, against
none of each before. `scripts/gen-seed-gospels.py` rebuilds it, `seed.go` says
when to, and `seed_content_test.go` fails if it drifts back.

**S13. DONE**, in the direction that had a reason behind it. `chapterCopyText`
kept the authored poem lines and said why — a chapter copy is a plain-text
export, and poetry copying as poetry is the cited-text share's own principle —
while the verse and paragraph paths flattened with `ReplaceAll(v.Text, "\n", " ")`
and no reason anywhere. They keep the lines now.

Prose is unchanged: a paragraph carrying ANY authored break is poetry and its
verses join with a newline, otherwise they still join with a space. A rule that
made everything poetry would be as wrong as the one that made everything prose.

"Copy verse with reference" also gained the share layout's shape — a BLANK line
before the reference rather than a dash on the same line. `composeShareText`
explains why that blank line is load-bearing: a poetic quote already contains
single breaks, so a citation on a bare next line reads as one more poem line.

The rule moved OUT of `reading_mobile.go` (`//go:build android`) into an
untagged `copy_text.go`, because nothing on the host or in CI could compile a
test against it where it was. A decision about whether the translators' line
breaks survive a copy is too easy to get wrong to leave where no test can reach.

## Stage 4 — the larger questions

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S14 | honour the publishers' paragraph breaks | all | M–L | all four | done |
| S15 | section headings: captured | all | M | all four | done (capture) |
| S16 | section headings: drawing them | all | M–L | none | done |
| S17 | NKJV supplied words: captured | NKJV | M | nkjv | done (capture) |
| S18 | speech reads the Psalm title | all | M | none | todo |
| S19 | mark an omitted verse's gap in the text | all | M | none | todo |
| S20 | show the NKJV's cross references | NKJV | S | none | blocked |
| S21 | website renders the footnote section | website | M | none | done |
| S22 | in-text footnote markers | all | L | none | blocked |
| S23 | draw small caps as small caps | NKJV | M | nkjv | done |

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
The NKJV is the BSB's case again, not the WEB's. Its feed sends the
publisher's paragraph blocks and the decoder already walks every one of them;
it uses a block boundary only to decide whether a verse that straddles it
takes a space or a line break, and never records that the block opened a
paragraph. Marking the first verse of each block is a small addition to a
loop that already exists, so the NKJV needs no table and no new traversal.

Fix S1 first or in the same pass. The open design question is unchanged:
whether a source break also resets the length counter.

For scale, John 3 in all three editions. The publishers agree exactly on the
opening of the Nicodemus dialogue, breaking after 2, 3, 4, 8 and 9 — question
and answer, turn by turn. The app draws 1-5, 6-10, 11-13 in all three
editions alike, because a character counter is what is deciding, so the
dialogue's shape is lost and the first break falls inside Jesus's reply.

**S15.** 3,091 headings in the BSB, at least one per chapter, and five in the
WEB Catholic of which four are the names by which those passages are known.
The plan was to render behind a setting, default off, with a shape guard that
drops and counts any heading over about 120 characters or containing a
chapter-and-verse reference: that would reject exactly one malformed WEB
Catholic heading in Greek Daniel 3 and nothing in the BSB. One of the three
shipped and the other two did not. The shared model was built first, as planned,
and it is what let all four surfaces follow; but a heading draws
unconditionally, with neither the setting nor the shape guard, so the fused
510-character Greek Daniel 3 string is drawn whole and a reader has no way to
turn headings off. Both remain to be decided rather than done.

**S16.** DONE. A heading is not a fact about a verse — it stands BETWEEN two of
them, which is why it fitted none of the run machinery that carries the red
letter, the supplied words and the small capitals. So the chapter's own ORDER
became the shared thing instead: `chapterBlocksFor` returns a chapter as
heading-then-paragraph blocks, and each surface only has to know how to set
those two. A heading always opens a paragraph, and where a publisher's heading
names a verse partway through one of its own paragraphs the paragraph splits
rather than the heading moving, because the placement is the publisher's
statement about the text and the paragraph break is the softer claim.

It is edition-blind, so the NKJV's headings draw by the same path as the BSB's,
without the separate NKJV pass this item was written to describe and without an
NKJV epoch of its own — capture had already spent one. Their notes ride on the
heading rather than on a neighbouring verse, so S9's probe is no longer what
gates this; it is still worth running for the count, and is still listed as such.

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

**S21. DONE.** The apparatus is on the pages: the translators' wording and
manuscript notes, the ones explaining a verse the translation omits, and any
riding on a psalm's title. BSB 1,091 pages, WEB Catholic 664, WEB 570.

**Always visible, not a disclosure.** The site has no settings and no
scripting, so there is no toggle to honour and nothing to remember a choice in;
a `<details>` would ask every reader to discover the apparatus for themselves on
every page. The app shows the section to a reader who asked for it, and the
page's answer to "who asked" is that a reader who scrolled past the end of the
chapter did. It is set smaller and muted so it reads as an appendix rather than
as more verse.

It sits OUTSIDE `</article>`, deliberately: the article is the scripture and the
apparatus is about it. A test pins that nothing from it reaches the chapter body.

Ordering, the key and the cross-reference exclusion all come from one exported
row — `bibletext.ChapterFootnoteEntries` — rather than being worked out again in
the generator, so the page cannot drift from the app.

**The NKJV contributes nothing, for two independent reasons**, which is why
"all editions" needed no licensing decision. Its entire apparatus is cross
references and the section excludes those (S20, still blocked); and a licensed
chapter has no page here at all — `/nkjv/` is a notice page that never calls
`chapterBody`. Measured on the generated site: 0 NKJV pages carry a notes
section.

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

**S23.** DONE. A small-caps span was folded to uppercase in the stored text,
so LORD, GOD and the four names of Jesus were literally capitals. Drawing them
as small capitals was possible, and cheaper than it first looked.

The mechanism exists. Red letter already carries per-rune-range styling to
every surface: the Apple builder and the Android dialect wrap a run in
markup, and the styled canvas pane carries a flag on each run and measures it
itself. Small caps ride the same path.

It also needs no change to the text, which is what makes it safe — and the
route it took makes that even plainer than planned. The plan was to leave the
uppercase alone and have a renderer synthesise small caps by drawing the first
letter at full size and the rest smaller. What shipped instead restores the
publisher's own casing to the stored text and substitutes the real Unicode
small-capital CHARACTERS into the drawn runs, which is rune-count preserving
just the same, so every selection offset, share, copy, link, search and spoken
word is untouched. The earlier claim that this would cost four pipelines was
wrong.

One of the two costs was real and was paid: the decoder discarded which spans
were marked, so recovering them took an NKJV cache epoch, number 7. The other
dissolved. It looked as though the small capitals would have to be synthesised
by shrinking capitals, because the reading face had no small-capital glyphs —
Georgia has none, nor does the embedded Gelasio fallback, nor Times New Roman.
The shipped face carries 25 of the 26 as real characters in all four cuts, so
they are drawn rather than faked, and no surface needs OpenType feature control
to get them.

The structural work is shared with S17, as expected: the run type carries red
and italic as independent dimensions, and the small capitals substitute
characters within whatever runs that produces.

Whether to synthesise small capitals or wait for a face that has real ones was
left to the section below, "The reading face", and the answer it gave was: wait.
That is how it went. The wait ended with the face, nothing is synthesised, and
what ended it is set out under "The universal reading face".

## The reading face

S23 ends on a question this section answers: whether small capitals should be
synthesised, or wait for a reading face that has real ones. It waits. And the
face that has them is also the face that fixes a second problem, which is that
the same chapter is set in four different types depending on where it is read.

That answer held; the face it named did not. Everything in this section — what
is drawn today, what a switch would cost, and Spectral as the recommendation —
is the survey as it stood at that point, and it was overtaken by a second one,
"The universal reading face" below, which found that its comparisons were taken
at the wrong size and chose Junicode 2 with Ezra SIL instead. Read the two in
that order: this section is why waiting was right, and that one is what the wait
ended in.

### What is drawn today

| surface | face actually drawn |
|---|---|
| macOS, both panes | Georgia |
| iOS | Georgia |
| Windows | Georgia |
| Android | the platform's generic serif — Noto Serif |
| Linux | DejaVu Serif |
| the website | whatever the visitor's device has; a visitor on Android or Linux has none of the three named |

Georgia is not shipped with the app, it is borrowed from the operating system,
and only three of the six surfaces have one to borrow. That is the whole of the
inconsistency: nothing chose Noto Serif or DejaVu Serif, they are what is left
when the stack's first three names are absent. Georgia cannot be shipped to
close the gap because it is a licensed system font and not redistributable.
That is why Gelasio is already embedded — an open, metrically identical
substitute — but it is used only for share cards and as the desktop fallback.

### What a switch would cost

Line breaks are the thing a reader would notice, so the candidates were
measured by summing advance widths for one line of John 3:16. Vertical figures
are fractions of the em.

| face | line width | vs Georgia | x-height | real small caps |
|---|---|---|---|---|
| Georgia | 44.19 em | — | 0.481 | no |
| Gelasio | 44.19 em | 0.0% | 0.481 | no |
| Spectral | 44.33 em | +0.3% | 0.450 | yes, regular and bold |
| Cardo | 42.69 em | −3.4% | 0.439 | regular only |
| Crimson Text | 39.56 em | −10.5% | 0.420 | no |
| Libre Baskerville | 51.73 em | +17.1% | 0.530 | no |

Gelasio is metrically identical to Georgia by design, so it would change
nothing and solve nothing: it has no small capitals either. Spectral is the
outlier that matters. It sets a line within a third of a percent of Georgia's,
so a page of it wraps where Georgia's page wraps, and it is the only candidate
carrying small capitals in both weights.

### Small capitals need c2sc, not smcp

This is the fact that settles the choice between the two faces that have them.

The live NKJV sends the divine name split across a span boundary with the
remainder already in capitals, `G` + sc`OD`. Both halves are capitals, so the
substitution needed is capitals-to-small-capitals, `c2sc`, not the ordinary
lowercase-to-small-capitals `smcp`. Asking `smcp` to set text that is already
uppercase does nothing at all.

| face | smcp | c2sc |
|---|---|---|
| Spectral regular | yes | yes |
| Spectral bold | yes | yes |
| Cardo regular | yes | no |
| Cardo bold | none — the face carries no OpenType features at all | |
| Georgia | no | no |
| Gelasio | no | no |

So Cardo cannot render this text as small capitals without first lowercasing
it, which is a change to the text. Spectral can render it untouched. Spectral
is the recommendation.

It is the recommendation this survey reached, and not the face that shipped. It
rests on a line width measured at the same nominal size, which "Comparing at the
same size compares nothing" below shows says nothing between faces of different
x-height; and it rests on `c2sc`, which the app in the end never asks any face
for, because the small capitals are drawn as Unicode CHARACTERS rather than
requested as a feature.

### The rest of the bill

Four things come with it, and none is hidden.

The repository ships Spectral in regular and bold only. Italic and bold italic
have to be added, which is roughly another half megabyte, and italic is not
optional now that the NKJV's supplied words are captured and waiting to be
drawn.

Spectral's x-height is 6% smaller than Georgia's, so the same point size reads
smaller. Its default line box is a third taller — 1.522 em against Georgia's
1.136 — so every line-height constant is wrong until it is re-tuned, including
the website's, which were derived from screenshots of the app.

Georgia's digits are oldstyle by default and Spectral's are lining, so verse
numbers would change shape unless `onum` is turned on. Spectral has it.
Georgia, as it happens, carries almost no OpenType features at all, which means
the `onum` already requested in the reading stylesheet has never done anything
there.

And one surface still cannot draw real small capitals afterwards. The Apple
panes can ask for the feature in their markup, and Android can set it per span
on the text paint, but the styled canvas pane on Windows and Linux draws its
own glyphs through Fyne, which exposes no OpenType feature control. Those two
platforms would keep the uppercase realization. That is a narrowing of the
defect from six surfaces to two, not a fix on all six, and it should be
described that way rather than as done.

That reservation is the one part of this bill the change of technique cancelled
outright. Substituting Unicode small-capital characters asks no toolkit for a
feature, so the canvas pane draws them off the same `applySmallCaps` step as
every other surface, and the defect was fixed on all six rather than narrowed to
two.

### What is actually manufactured here

Separately from which face is used: the decoder used to uppercase every `sc` and
`nd` span into the stored text. The publisher did not spell the word `LORD` in
four capitals; it sent an initial capital and a small-capital remainder, and the
app replaced that with capital letters and then forgot where the span was. A
reader who copied a verse out of the app got a spelling no edition of the NKJV
prints. Under the standard at the top of this document that is an addition, and
it is the same shape as the verse-number superscript: keep the typography on
the page, and let the plain text leaving the app carry the conventional
uppercase realization, which is what every other edition's copy behaviour
produces.

The fix was the one already used for supplied words — mark the span with
sentinels, resolve it to rune offsets once the text has settled, store the
offsets and not a case change — and that is what shipped. It cost an NKJV cache
epoch of its own, epoch 7, rather than travelling in S17's; the red-letter
table was regenerated against the restored text in the same move, since the
case change was what its fingerprints had been tripping on.

## Display issues found while unifying the reading face

Checked by rendering or measurement, not by reading code. Corrections of
earlier conclusions are kept AS corrections, because the wrong version is the
one that sounds plausible and will be arrived at again.

### Must not ship

Nothing stands here. All three items that did — the divine name unrendered,
sixteen verses of lost red letter, and a macOS pane with no paragraph
separation — shipped their fixes, and are recorded below with what they were.

### Fixed

**The divine name drew as ordinary text, in 5,891 verses.** The decoder kept the
publisher's characters and recorded the small-capital spans, and nothing read
them on any surface. They are read now: `applySmallCaps` substitutes the Unicode
small capitals into the DRAWN runs, so every surface gets them from the one
shared step every red-letter path already ends in, and `outboundText` maps them
back to ordinary capitals on the way out. Not to the publisher's own letters: a
small capital does not record whether it stood for an upper or a lower case
letter, so `smallCapitalToLetter` chooses the capital deliberately. A verse the
publisher sent as `Lord` and the page draws as `Lᴏʀᴅ` comes back out of that
cleaner as `LORD`, which is the conventional plain-text realisation of a
small-capital divine name and what keeps the Tetragrammaton distinct from
Adonai — the same answer "What is actually manufactured here" reaches above, and
the same one `docs/SOURCE_FIELDS_DECISIONS.md` records at Defect 3. Where the publisher
sent capitals inside the span, as it does in `G` + `OD`, what comes back out is
exactly what it sent.

Larger than it first appeared, in two ways, and this is why the span data had to
be read rather than the case folded into the text. The feed sends the WHOLE word
inside the span in ordinary case far more often than it sends a capital outside
it — 286 of 288 spans in a live sweep — so almost every span changes letters, not
the handful assumed. And it is not only the divine name: `call His name JESUS`
became `call His name Jesus` in Matthew 1:21 and 1:25, Luke 1:31 and 2:21. Psalm
110:1 was the sharpest, reading `The Lord said to my Lord,` with nothing left to
tell the two apart, and Genesis 15:2 lost `Lord GOD` the same way.

The inscriptions are untouched, contrary to what was assumed when this was
planned: `MENE, MENE, TEKEL, UPHARSIN`, `THIS IS JESUS THE KING OF THE JEWS`,
`TO THE UNKNOWN GOD` and `MYSTERY, BABYLON` carry no span at all — they are
literal capitals in the feed and stay as they are.

**Sixteen verses lost their red-letter spans, seven of them visibly.** The
red-letter table refuses a verse unless a hash of its text matches, and a refusal
paints the whole verse red. The case change kept the rune count, so only the
fingerprint tripped — silently, since the licensed text is not in this
repository. Where Christ quotes the Old Testament and the quotation carries the
divine name, the narration was printed as his. Seven read wrongly — Matthew 4:7,
4:10, 21:42, 22:37, Mark 12:29, Luke 4:8 and 4:12 — seven differed only in
whitespace already black, and two improved. The website was never affected, since
it never publishes this edition. The epoch assertion that caught it still stands
and still fails the build on any drift; the table has been regenerated against
the decoder that removed the case change, and the two agree at NKJV cache epoch
7.

**macOS had no paragraph separation at all.** No blank line and no indent: the
reporter stylesheet emits `margin: 0`, and the first-line indent had been moved
out of the text and re-applied on iOS only. A new paragraph began flush left on
the next line, indistinguishable from a wrap, so the publishers' paragraphing was
invisible there. The AppKit twin of the iOS indent now exists, so the pane sets a
first-line head indent on its justified paragraphs — typography rather than text,
which is the whole point of having moved it out of the text.

**The verse number left its line.** Twice. The lift was first taken from the
face's DECLARED line box, which is a third larger in the shipped face; the
correction that replaced it had the wrong sign, because the numerals are
Unicode superscripts already and the shipped face draws them 0.232 em higher
than the borrowed one. It is a drop of 0.117 em, and both leadings are pinned
by a test, at the shipping body size — a bare test theme puts every width on the
same side of the reporter gate, so only one leading is otherwise reached.

### Was visible, and is not any more — re-checked 9 September 2026

All four entries this section held were STALE, and every one had been fixed by
work that landed after they were written. They are kept here rather than deleted
because a reader who remembers the symptom needs to find the answer, and because
four wrong "still open" entries in one section is itself the lesson: a section
naming live defects has to be re-run, not re-read.

**Greek footnote words fall out of the reading face. FIXED by the face itself.**
The claim was that the shipped face carried three codepoints of the Greek block,
so a Greek word kept its leading letter and drew the rest in a system fallback.
That described the face this app shipped BEFORE Junicode. Measured now:
**Greek Extended 233 of 233, basic Greek 95 of 135**, and both words the entry
named — `μονογενη` and `ἐπίσκοπον` — set entirely in the reading face.
Pinned by `reading_face_coverage_test.go`, which also walks every rune of all
1,226 shipped notes and finds nothing either face cannot draw.

**A share card can silently drop a letter. FIXED.** Prata has no macron vowels
and its `.notdef` is an invisible blank, so a card of Genesis 4:18 read
`" ´noch`. `typefaceForText` now skips any face that cannot set the verse it is
about to draw, and `share_image.go:248` passes it the verse text rather than the
reference alone — which is the half that makes the guard real.
`share_image_glyphs_test.go` walks every variant a reader can reach by tapping
Regenerate, and carries a control that skips the test if no face is actually
blind, so it cannot pass vacuously.

**Windows regressed to a different face. FIXED.** The claim was that the shipped
face reached the canvas pane alone, leaving three faces in the wild.
`reading_fonts_embed.go` carries no build tag, so Junicode is compiled into
every platform, and `ui_desktop.go` (`//go:build !ios && !android` — Windows and
Linux included) and `ui_mobile.go` both build the theme from
`loadReadingFonts()`. One face, everywhere.

**Android's line pitch is wrong. FIXED.** `setLineHeight` reads the CURRENT
paint's metrics and stores the difference, so a typeface set afterwards leaves
the stored extra measured against a font that is not the one drawing.
`BtBridge.java` sets the typeface FIRST now, and its comment says so and says
how long it had been the other way round.

### Corrections to earlier conclusions

**The leading does not need re-tuning.** The declared line boxes are 1.522 em
against 1.136 em, which was reported as meaning every constant was stale. The
ink extent over the characters scripture uses is 0.990 em against 0.973 em, and
rendered at the pane's own leading the two are indistinguishable. A declared
line box is not a claim about ink. Where the number does matter is a surface
that derives its leading from font metrics rather than setting it: Android.

**`font-feature-settings` is not dropped by the Apple importer.** It is filtered
to what the resolved face implements, and the borrowed serif implements none of
what is asked for, which looks identical to a drop. The old-style figures on
screen today are that face's defaults. After the swap the request genuinely
takes effect, so the Apple panes keep old-style figures — and the small-capitals
route works there.

### Text defects a reader sees, not caused by the face

- 259 NKJV verses run two words together where the source lost a space.
- WEB Catholic Mark 9:47 reads `the Gehenna oF fire`; the plain WEB has `of`.
- Psalm 119's acrostic letters print at the end of the preceding verse, so
  119:8 ends `Don't utterly forsake me. BETH`, and being inside the verse text
  they reach search, sharing, links, the website and speech.
- The website sets 990 poetic paragraphs justified and auto-hyphenated, because
  the paragraph's kind is decided by its first verse alone.

### Captured and still not drawn

Only the poetry indent depths remain. The small capitals, the translators'
supplied words and the publishers' section headings are all drawn now, on every
surface.

The three that landed followed one rule, and it is the rule worth keeping: a
decision the edition made about the text is taken ONCE, in a place all the
surfaces already pass through, and expressed as something every surface can
already draw. The small capitals became CHARACTERS, which needed no feature
control anywhere. The supplied words became a FLAG on a run, and every surface
could already set italic. The headings needed a bigger shape, because a heading
sits between verses rather than inside one, so the chapter's own order became
the shared thing instead.

The indent depths are the awkward one left, for the same reason headings were:
an indent belongs to a LINE, and a line is the one thing each surface still
works out for itself.

## The universal reading face

The face question was posed as a Latin one and that was the wrong question. The
app draws three scripts, and a face covering only Latin hands the other two to
whatever the platform supplies — which is what split a Greek word between two
typefaces mid-word.

Sixty-six faces were examined against what the app actually needs. **None of
them passes.** The ones that cover every script fail on the cuts and the
features; the ones with four properly featured cuts have no Hebrew.

### What the app must be able to draw

Counted over the four editions: 145 distinct characters. The count is not the
hard part.

| requirement | detail |
|---|---|
| polytonic Greek | 25 codepoints of the Greek block, 8 of Greek Extended |
| Hebrew | 9 letters and SEVEN COMBINING MARKS, cantillation and niqqud, needing real GPOS mark attachment rather than glyphs |
| small capitals | `smcp` suffices — the feed sends the whole word in ordinary case in 286 of 288 spans — but it is needed in EVERY cut, because the italic carries the translators' supplied words |
| superior figures | verse numbers are ¹²³, not digits, so the face's superior figures ARE the numerals |
| four cuts | the toolkit synthesises neither bold nor italic |
| licence | redistributable inside a shipped app and from a website |

Hebrew volume: 447 footnote runs. Greek: 67. Etnahta occurs 80 times, holam 80,
tevir twice.

### Comparing at the same size compares nothing

This invalidates every line-width figure recorded before it. A measure taken at
the same nominal size says nothing between faces of different x-height, because
they do not read as the same size. Normalised so each reads as large as Georgia
at 21px:

| face | raw measure | raw vs Georgia | x-height | size to match | measure at that size |
|---|---|---|---|---|---|
| Georgia | 44.19 | — | 0.481 | 21.0px | — |
| Junicode | 39.10 | −11.5% | 0.415 | 24.3px | **+2.6%** |
| Cardo | 42.69 | −3.4% | 0.439 | 23.0px | +5.8% |
| Spectral | 44.33 | +0.3% | 0.450 | 22.4px | **+7.2%** |

Spectral's "+0.3%", quoted as the headline reason to choose it, was the wrong
measurement. It reads 6.4% smaller at the same point size, so the match holds
only where the text is smaller. At equal apparent size Spectral is the FURTHEST
from Georgia's measure of the three, and Junicode the closest.

**This table was written and then not acted on, and the reader saw it.** The face
was swapped at the unchanged 21px and shipped that way, so Scripture drew 13.2%
smaller on every surface at once — confirmed on an iPhone before anything was
changed, where the drawn x-height fell from 30 device pixels to 26. The
correction now lives in `reading_face_scale.go` as a per-face optical scale of
1.15178, applied to glyph sizes and to every length reckoned in ems of the set
type, and deliberately NOT to a measure. Measured after the change, on the same
device and chapter: x-height back to 30 device px, and the line breaking in the
same places it broke under Georgia — the "+2.6%" above is the residue, and the
advance ratio cancels it to within a tenth of a percent when the column is held
still. Android went 23 → 27 device px on a Pixel by the same route, behind the
API-29 gate that decides whether the shipped face was got at all.

Two things this did NOT change, both checked rather than assumed. The Hebrew face
is only ever reached for Hebrew glyphs, so a uniform scale leaves its size
relative to the Latin around it exactly where it was. And the desktop canvas
pane's fallback text widget draws in the CHROME face, not this one, so it takes
no correction — scaling it would simply make it 15% too large.

### Why the face that covers everything still fails

Cardo covers all three scripts, and coverage was never the whole requirement.

- No bold italic exists. Confirmed five ways, including that the upstream source
  repository holds no bold-italic source to build one from.
- Small capitals work in the regular alone. The italic has no `smcp` and the
  bold has no GSUB table at all, so the divine name silently loses its small
  capitals in every italic and bold passage — and the italic is exactly where
  the translators' supplied words are set.
- **The Greek is one upright regular-weight design reused in all three cuts.**
  Ink area over the required Greek Extended characters differs from the regular
  by +0.1% in the italic and −0.0% in the bold, against −18.8% and +43.7% for
  Latin. Greek inside an italicised supplied-word span renders upright, and
  Greek in bold is not bold.
- Hebrew mark attachment exists only in the regular, and even there holam is
  outside mark coverage with no mark-to-mark lookups, so stacked point and
  cantillation collide.

It remains excellent as a regular-weight fallback for Greek and Hebrew, which is
a supporting role, not a text face.

### What to do instead

A chosen primary face plus a chosen, EMBEDDED secondary for Hebrew, so the
fallback is the app's own rather than the platform's.

**Junicode 2** for Latin and Greek. Verified against the binaries: `smcp` and
`c2sc` in all four cuts, with the "Lord" test passing in every one — the L
untouched, the ord substituted. Complete polytonic Greek in all four. SIL Open
Font License with NO reserved font name, so a subset keeps its name. About 1 MB
a cut raw and 24 KB subsetted to what this app draws. Closest to Georgia's
measure at equal apparent size. No Hebrew whatsoever.

**Ezra SIL** for Hebrew. All 27 letters, all seven marks in mark coverage, and
Genesis 1:1 shapes with no missing glyph. One cut, no Greek, no Latin macrons,
seven of the ten superior figures absent, no small capitals — it is a Hebrew
face and nothing else. "Ezra" and "SIL" are reserved names, so a subsetted build
must be renamed. Its vendor states it will not be extended.

### Both halves work in native Fyne, and the second one changes the plan

Tested by rendering through the toolkit rather than by reading its source.

**The pairing renders.** A run given the Hebrew face draws pointed Hebrew with
its marks in place; the same string given the Latin face falls through to the
system, and the two produce visibly different ink, so the per-run choice is
really taking effect. Polytonic Greek draws from the Latin face itself, with its
breathings and circumflexes.

**Small capitals render too, with no fork and no feature control.** Unicode has
real codepoints for the Latin small capitals, and Junicode carries 25 of the 26
in ALL FOUR cuts — every letter the divine name needs, in regular, italic and
bold. Drawing `Lᴏʀᴅ` gives a full-size L and small-capital ORD on the canvas
pane today.

That is the same technique the app already uses for verse numbers, which are
written as real superscript characters rather than asked for as a feature. And
it settles the question the same way on every surface at once: characters need
no CSS, no post-import sweep on the Apple panes, no per-span paint setting on
Android, and no patched toolkit on Windows and Linux.

Two properties make it safe rather than merely clever. The substitution happens
when the drawn runs are built, so the STORED text keeps the publisher's own
characters. And it is rune-count preserving — `Lord` is four runes and `Lᴏʀᴅ` is
four runes — so every offset the app stores stays valid: footnote anchors,
supplied-word spans, red-letter spans and selection alike.

What it needs is one addition to `outboundText`, which already exists to strip
the app's own typography from text leaving the app: it maps a superscript
numeral back to a digit today, and would map a small capital back to a letter.

### The actual work is the mechanism, not the file

A pairing cannot be adopted by swapping a resource. `bibleTheme.Font` returns
one resource per style, and the toolkit takes its fallback from the platform's
default font rather than an app-chosen one, so on Windows, Linux and Android a
Hebrew run would still reach whatever the system supplies — precisely the
behaviour being retired. The Apple panes can cascade through CoreText.

The hook does exist: the reading pane already assigns a font source PER RUN, and
already tokenises the text. Choosing the Hebrew face for a Hebrew run is a
change at that seam.

Also settled, and it constrains the choice: the toolkit never sets variation
coordinates, so a variable font renders at its default instance. Four STATIC
cuts have to ship.

## Getting the shipped faces into the Apple panes

Probed directly on macOS rather than reasoned about. The short answer is that
it works, but not through the stylesheet.

    register before any import          OK
    [NSFont fontWithName:@"Junicode"]   Junicode-Regular
    CSS asks for Junicode, Georgia      Georgia@13.9, Georgia@21.0
    CSS asks for Georgia                Georgia@13.9, Georgia@21.0
    after a post-import sweep           Junicode-Regular@13.9, Junicode-Regular@21.0

Registering the face succeeds and AppKit can find it by name, but the HTML
importer resolves families from a pool that app-registered fonts are not in:
asking for the shipped face gives exactly what asking for the system one gives.
It fails silently, which is why this has to be probed rather than assumed.

The route that does work is a POST-IMPORT SWEEP over the attributed string,
replacing each run's family at the run's OWN point size. The last line above is
the whole argument: the em-derived sizes survive exactly — 13.9 is 0.66 × 21,
the verse-number size — so the point-size thresholds the panes use to tell a
verse number from body text and to find the end of the content keep working
untouched.

Both panes already run a mutating sweep over that string for other reasons
(reading_ios.go:3019, reading_macos.go:1366, where the reporter indent is
applied), so this is a second pass beside an existing one rather than new
machinery.

Worth noting what this does NOT need any more. The small capitals are drawn as
characters, so nothing here depends on the importer honouring a font feature —
which was the other reason this pane was going to need special handling.

### The Hebrew needs no detection on three of the four surfaces

Probed on macOS, and it removes most of the work this looked like:

    Junicode alone                    hebrew -> LucidaGrande
    with a cascade list to Ezra SIL   hebrew -> EzraSIL

Register both faces, give the swept font a cascade list naming the Hebrew one,
and CoreText falls through PER GLYPH to the app's own choice. Latin and Greek
stay in the reading face; Hebrew resolves to the Hebrew face; nothing has to
recognise a Hebrew run at all.

The web has the same property for free — a two-family stack in `--scripture`
cascades per glyph in every browser. Android has it from API 29 through
Typeface.CustomFallbackBuilder, above the app's floor of 21, so below that it
keeps today's system fallback.

The canvas pane is the exception, and the only surface that needs the explicit
per-run rule already written for it: its toolkit falls back per rune too, but
only ever to a SYSTEM font, never to one the app chose.

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
