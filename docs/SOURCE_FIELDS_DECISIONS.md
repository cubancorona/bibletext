# Source fields: analysis and recommendations

The companion to `docs/SOURCE_FIELDS.md`. That file is the inventory: for
every field each source sends, whether the app keeps it, skips it for a
stated reason, or leaves the question open. This file takes each skip and
each open row in turn, says what is actually lost, what it would cost to
keep, and what to do.

Three of the items below were raised as defects found while measuring, and
they are listed first. Two were real and are fixed; the third turned out to
be the publisher's own intent.

Figures come from the raw whole-translation captures under
`build/biblecache/`, from the app's own decoded caches, and from a full NKJV
download made the way the app makes it. Print and other-app practice is
noted as general knowledge and labelled where it is not verifiable here.

## Defects

### 1. The Psalm 119 acrostic letters are in WEB and WEB Catholic verse text

The feeds send the 22 Hebrew acrostic letters of Psalm 119 in two ways, and
the decoder keeps both as Scripture.

The letter ALEPH arrives as the chapter's Hebrew subtitle, so it is stored as
Psalm 119's title and drawn in italics above verse 1 on every reading pane.
The psalm has no title in these editions. This is the whole of the difference
between the WEB's 117 subtitles and the BSB's 116: the BSB sends nothing
there, and every other short WEB title is genuine.

The other 21 letters arrive as descriptive runs at the end of the last verse
of each stanza, so they are appended to that verse's text. Psalm 119:8 now
reads "I will observe your statutes. / Don't utterly forsake me. BETH", and
the same happens at verses 16, 24, 32 and so on to 168.

Because the letters are in the verse text, they are in the search index, in
shared text and images, in copied text, in the link payload, on the website,
and they are read aloud. A search for "beth" matches Psalm 119:8.

The app already treats this as a defect in the other direction. The NKJV
decoder keeps the acrostic style out of verse text on purpose — it holds the
letters in the chapter's side-band as headings, under their own style name —
its comment records that these headings once leaked into verse text, and the
live canon test asserts that Psalm 119:1 does not contain "Aleph". The NKJV
canon is clean; the two WEB editions are not.

**Fix it.** Route descriptive runs out of verse text into the chapter's
side-band, where superscriptions already live, and ignore a Hebrew subtitle
whose whole text is an acrostic letter. A denylist of the 22 letter names
would be smaller but would not survive a differently marked acrostic psalm.
Cache epoch bump for `web` and `webc`. Whether the letters are later shown as
stanza markers is the same product question as section headings and should
follow that decision.

### 2. The paragraph rule ignores curly quotation marks

The rule this describes no longer exists, and the figures below are why it went
rather than why it was mended. Paragraphs were made by one shared function: a
break happened once the paragraph had reached 320 characters and the previous
verse ended in a full stop, exclamation mark, question mark, or a straight
quotation mark. The text uses curly quotation marks, so a verse that closed
reported speech was not recognised as a place to break, and the paragraph ran
on.

The character count was removed outright rather than kept as a fallback, once
every edition could be paragraphed from its publisher's own marks (item 5). The
shared function now opens a paragraph only where a verse carries the
publisher's paragraph start, so a chapter a publisher left unbroken stays
unbroken.

| edition | break points past the length threshold | refused only for a curly quote |
|---|---|---|
| BSB | 11,536 | 2,094 (18.2%) |
| WEB | 11,978 | 2,272 (19.0%) |

Nearly a fifth of eligible breaks were suppressed by a character class the
rule did not know. Every edition and every surface was affected, because they
all funnel through this one function. The one-line fix was made first — the
curly closing marks were added to the suffix test — and cost no cache epoch,
since paragraphs are computed at render time. The rule it mended was then
retired entirely.

### 3. The NKJV names Jesus in capitals in four verses

The decoder rendered a small-caps span as full uppercase, which was right for
the divine name and reassembled the source's split letters into GOD. Across
the canon that yielded LORD 6,504 times and GOD 308 times, plus a set of
genuine inscriptions that print also capitalises: the notice on the cross in
all four Gospels, the writing on the wall in Daniel 5, "HOLINESS TO THE LORD"
on the priest's plate and the horses' bells, "THE LORD OUR RIGHTEOUSNESS",
"THE LORD IS THERE", "TO THE UNKNOWN GOD", and the names written in
Revelation 17 and 19. All of those read correctly. The inscriptions still do,
and always did so on their own: they carry no span at all and are literal
capitals in the feed.

Four verses were the exception, and they were the only ones: Matthew 1:21,
Matthew 1:25, Luke 1:31 and Luke 2:21 each read "call His name JESUS".
The feed marks the name with the same small-caps style it uses for the divine
name, and the app had no small caps, so it uppercased.

This is not a defect. The publisher marks those four names with the
small-caps style deliberately, and its own web edition sets them in small
caps. The app had no small caps: it folded a small-caps span to uppercase in
the stored text, which is the standard plain-text rendering of small caps and
the same treatment that yielded LORD for the divine name. The output was a
faithful flattening of what the publisher set.

The residue was typographic, not textual, and the recommendation was to leave
it: drawing true small caps would mean keeping the text in mixed case and
applying the style at render time, and the uppercase WAS the text — which is
what made a search for LORD behave, and what kept sharing, speech and links
agreeing with the page.

That recommendation has been overtaken, and by a route it did not consider.
The span is kept as offsets, so the stored text is the publisher's own casing
again; the small capitals are real Unicode CHARACTERS substituted into the
drawn runs, which preserves the rune count exactly; and text on its way out of
the app is mapped back to ordinary capitals, which is the conventional
plain-text realisation this defect was arguing for all along. Search, sharing,
speech and links therefore behave as before, and these four verses are set in
small capitals like every other marked span. Nothing here was wrong; it simply
assumed that keeping the typography meant changing the text, and it does not.

## Decision table

Effort is XS for a line or two, S for under a day, M for a few days, L for
more. "Epoch" means a cache epoch bump, which forces every reader of that
edition to download the text again.

| # | item | editions | what is lost today | recommendation | effort | epoch | confidence |
|---|---|---|---|---|---|---|---|
| 1 | acrostic letters in verse text | WEB, WEBC | 21 verses plus one false title | fix | S | web, webc | high |
| 2 | curly quotes in the paragraph rule | all | ~2,100 breaks per edition | fix — SHIPPED, and the rule it mended has since been removed altogether | XS | none | high |
| 3 | "JESUS" in capitals | NKJV | nothing; the publisher marks it | no change — SINCE OVERTAKEN: the uppercasing is gone, the span is kept as offsets and drawn in small capitals | — | nkjv (spent) | high |
| 4 | section headings | BSB, WEBC, NKJV | 3,091 / 5 / about 3,300 | capture and render behind a toggle — SHIPPED on all four editions and all four surfaces, but with neither the toggle nor the shape guard | M–L | bsb, webc, nkjv (all spent) | medium |
| 5 | source paragraph boundaries | all | 13,894 BSB breaks, 742 WEB | honour as extra breaks, keep the rule as fallback — SHIPPED, except that the rule was removed rather than kept | M | three helloao, plus nkjv (all spent) | medium-high |
| 6 | italics for supplied words | NKJV | about 3,000 in the New Testament | generate the table, then design the run type — SHIPPED as decoder-kept offsets rather than a generated table, and the run type carries the two dimensions | M–L | nkjv (spent), not none | medium |
| 7 | poem indent depth | all | every second-level line | keep skipping | — | — | medium |
| 8 | Selah placement | WEB, BSB | set differently in each | keep the source's placement | — | — | medium |
| 9 | descriptive runs | BSB | one oracle title | capture with item 1 | S | with item 1 | medium |
| 10 | words-of-Jesus flag | WEB, WEBC | nothing; tables agree exactly | keep tables, add a decode-time cross-check | S | none | high |
| 11 | NKJV cross references hidden | NKJV | 32,473 notes | built behind the `nkjvxrefs` tag: the publisher's notes lead the panel, the Treasury one tap away under its own credit; display waits for the licensing answer (S20) | M | none | high |
| 12 | note callers and anchors | all | in-text markers | keep dormant, fix the Android scan first | — | — | high |
| 13 | website: Psalm titles | WEB, WEBC, BSB | every title | ship | S | none | high |
| 14 | website: footnote section | WEB, WEBC, BSB | the whole apparatus | defer, needs a presentation model | M | none | medium |
| 15 | search does not index titles | all | "Absalom" finds nothing | index titles | S | none | medium-high |
| 16 | speech does not read titles | all | the title before verse 1 | done: speech reads the title as the read-along's verse-0 row, and the recorded tables carry the row (S18) | M | none | medium |
| 17 | share omits titles | all | the title with a shared psalm | low priority, no hook exists | — | — | medium |
| 18 | copy omits titles | Apple | nothing on native panes | no change | — | — | high |
| 19 | poem breaks in the Android long-press menu | Android fallback | line structure | match its own chapter copy | XS | none | medium |
| 20 | poem breaks in cross-reference snippets | all | line structure | judgement call, write it down | — | — | medium |
| 21 | omitted verses unmarked in the text | all | 34 verse numbers vanish | mark the gap when footnotes are on | M | none | medium |
| 22 | Fyne fallback panes | Android | title and section | no feature work, pin the release path | XS | none | high |
| 23 | offline seed is flattened | WEB | breaks, notes, titles | regenerate from the current decoder | S | none | high |
| 24 | feed verse counts unused | helloao | a free integrity check | adopt as a decode-time check | S | none | high |
| 25 | note reference field unused | helloao | a free integrity check | adopt as a soft check | S | none | high |
| 26 | unknown node and item shapes uncounted | helloao | future feed changes invisible | count and log | S | none | high |
| 27 | unknown paragraph and char styles | NKJV | future feed changes invisible | extend the lists, census, do not fail | S | none | high |
| 28 | notes inside skipped headings | NKJV | unknown quantity | five-call live probe | XS | none | high |
| 29 | book and chapter metadata | helloao | nothing | keep skipping | — | — | high |

## The helloao editions: WEB, WEB Catholic, BSB

### Section headings

The BSB carries 3,091 headings, at least one in every chapter: Luke 154,
Matthew 153, Psalms 150, Genesis 148. They are the reader's map of a chapter,
they are the speaker labels in the Song of Songs, and they are CC0. The WEB
carries none, so the default edition is unaffected either way. The WEB
Catholic carries five, four of which are the names by which Catholic readers
know those passages: the Letter of Jeremiah, the Song of the Three, Susanna,
Bel and the Dragon.

The fifth is a hazard. One WEB Catholic heading in Greek Daniel 3 is a
510-character string that fuses a title with what reads as a translator's
note, with no space between them. Rendered as a heading it would drop a
paragraph of editorial prose into the middle of the chapter.

General knowledge: essentially every modern print Bible sets the publisher's
headings between paragraphs, and the major apps show them inline, hideable
where a setting exists. Audio Bibles do not read them.

**Capture and render behind a Settings toggle, default off**, with a shape
guard that drops and counts any heading over about 120 characters or
containing a chapter-and-verse reference, which rejects exactly the fused
Daniel string and nothing in the BSB. Build it in the order the risk allows:
the shared paragraph model first, then the website and the styled pane, which
are safe by construction, then the Apple and Android panes, whose verse
indexes must learn to exclude a heading paragraph. That last part is the same
class of change as the selection-over-wash work, so it deserves a device pass
over a mid-chapter heading before it ships.

Capture and rendering both shipped, and the shared-model-first order held: one
block model decides a chapter's order as heading-then-paragraph and all four
surfaces read it. The toggle and the shape guard did not ship. So a heading is
drawn on every surface with no way to turn it off, and the fused 510-character
Greek Daniel 3 string is drawn whole, which is the one case this recommendation
was written to catch. Both are still worth doing, and the guard is the cheaper
and the more urgent of the two.

### Source paragraph boundaries

The two editions differ enormously in how much paragraphing they carry, which
is why neither pure answer works.

| chapter | source paragraphs | app paragraphs |
|---|---|---|
| BSB Genesis 1 | 20 | 11 |
| BSB John 3 | 15 | 9 |
| BSB Matthew 5 | 16 | 13 |
| BSB Acts 2 | 18 | 12 |
| BSB Romans 8 | 12 | 11 |
| BSB Psalm 23 | 2 | 2 |
| WEB Genesis 1 | 1 | 10 |

What the app merges away in the BSB is mostly dialogue: the Beatitudes as
eight separate lines, each exchange with Nicodemus, each crowd reaction in
Acts 2. Those are the translators' own decisions and they read better than a
length rule.

The WEB's feed looks unparagraphed: 742 breaks in the whole Bible and none at
all in Genesis 1, John 3 or Romans 8. Its published markup says otherwise.
That carries 9,254 paragraph markers and 23,331 poetry line markers, and its
Genesis 1 breaks at the days of creation exactly as the BSB's does. About 92%
of the edition's paragraphing is lost in the supply, not absent from the
translation. So honouring the source means the publisher rather than the
feed, and for this edition the structure has to be recovered offline from the
published markup, which is the pattern the red-letter tables already use for
the same reason.

**Honour the publishers' breaks as additional forced breaks and keep the
current rule as the fallback only where neither the feed nor a generated
table has one.** A paragraph-start flag on
the verse, set where the decoder currently skips a break node, plus one
branch in the shared grouping function; every surface inherits it without its
own change, which is what that shared function is for. Fix defect 2 in the
same pass, because the fallback keeps running inside long unmarked stretches.
One design question is open: whether a source break also resets the length
counter. The NKJV needs new decoder work to detect a verse that opens a
paragraph block, so it should follow rather than hold up the rest.

The flag and the one branch are what shipped, and the NKJV followed rather than
held up the rest, exactly as sequenced. The fallback did not survive: with all
four editions paragraphed from their publishers' own marks there was nowhere
left for a character count to fire that a publisher had not deliberately left
unbroken, so the rule was deleted instead of demoted, and the open design
question about resetting the counter went with it. A chapter no publisher broke
is now one paragraph, which is what such a chapter is in print.

### Poem indent depth

Every poem clause carries a level: the WEB has 8,247 first-level and 10,736
second-level clauses, the BSB 11,756 and 12,751, and the WEB Catholic adds
seven at a third level. Only the presence is used; the value is discarded, so
every poem line is drawn flush left everywhere.

What is lost is the visual shape of Hebrew parallelism, which print shows by
indenting the second half of a couplet. Carrying it needs a per-line
side-band, because a level belongs to a line and the verse text is a flat
string that feeds search, share and speech.

**Keep skipping, and record the reason.** The gain is real but cosmetic, the
cost lands on four rendering surfaces plus three cache epochs, and a
single-line poem verse already reads as prose, which caps the benefit.
Revisit only alongside source paragraphs, which need the same side-band.

### Selah

The word appears in 75 WEB verses and 74 BSB verses. In the WEB it is its own
clause, and therefore its own line, in two of them; in the BSB it is always
joined to the end of the previous line. So the same word is set differently
in two editions of the same app, and in neither the way print sets it, which
is usually apart from the line because it is a performance direction.

**Keep the source's placement.** The app's rule elsewhere is that the
publisher's text is the text, and inventing a placement the source did not
encode is a larger step than it looks. The inconsistency is invisible unless
the two editions are read side by side.

### Descriptive runs

The flag marks 21 runs in the WEB and WEB Catholic, which are the acrostic
letters of defect 1, and exactly one in the BSB: Zechariah 12:1, "This is the
burden of the word of the LORD concerning Israel", which is the oracle's
title and reads acceptably today because a line break follows it.

**Capture descriptive runs into the chapter's side-band with defect 1**, and
render the BSB's single oracle title as a title line. The acrostic letters
then wait on the headings decision.

### Words of Jesus

The WEB and WEB Catholic feeds flag 2,289 runs across 2,059 verses; the BSB
feed flags none. Red letter comes instead from tables of rune offsets
generated from the publishers' own markup, guarded by a rune count and a
hash, with whole-verse red as the fallback.

Comparing the two sources for the WEB:

| measure | result |
|---|---|
| verses flagged in the feed | 2,059 |
| verses in the generated table | 2,059 |
| in one but not the other | 0 |
| verses where the table's offsets select exactly the flagged text | 2,059 |
| verses currently falling back to whole-verse red | 0 |

The agreement is not circular: the table is built from the publisher's markup
fetched separately, the flag comes from the runtime feed, and two independent
pipelines agree completely. The deuterocanonical books carry neither, which
is correct.

**Keep the tables as the source of red, and add the flag as a decode-time
cross-check.** The table is better at render time because it is validated at
generation and fails closed; the flag's value is as a second witness that
would notice drift as soon as either side moved, instead of leaving the app
to degrade quietly to whole-verse red. Replacing the tables would trade a
validated pipeline for a dependency on the feed continuing to carry the flag,
and the BSB and NKJV would still need tables.

### The apparatus

Classifying every note body by its own wording:

| edition | notes | pure cross reference | reference plus gloss | textual variant | other |
|---|---|---|---|---|---|
| BSB | 4,853 | 389 (8%) | 274 (6%) | 3,345 (69%) | 845 (17%) |
| WEB | 1,226 | 0 | 4 | 579 (47%) | 643 (52%) |
| WEB Catholic | 1,673 | 41 (2%) | 26 (2%) | 703 (42%) | 903 (54%) |

This resolves the asymmetry the inventory flagged, where the public-domain
editions' cross references render and the NKJV's do not. The public-domain
apparatus is overwhelmingly what a reader wants at the foot of a chapter:
alternative renderings, manuscript readings, unit conversions, name meanings.
Only 8% of the BSB's notes, and none of the WEB's, are the bare citations the
NKJV's apparatus consists entirely of. The rule is hiding a category these
editions barely have. The classification is a text heuristic, so the
percentages are close rather than exact.

**Leave the classification alone.** Inferring a kind from wording would trade
a clean structural rule for a heuristic one, where a false positive silently
hides a translator's gloss.

### Metadata and the integrity checks the feed is giving away

The decoder never reads the feed's book names, chapter counts, verse counts,
audio links and timings, or the translation block. Book names agree exactly
with the app's own canon for all 66 books, and the app's tables are the
better source because they must also serve editions that name books
differently.

Two fields are worth taking, because they are exact and free:

- **The feed's own verse count per book.** For the BSB it equals the decoded
  count in all 66 books. For the WEB it exceeds it in exactly three books, by
  exactly the omitted verses the decoder turned into orphan notes: Luke by
  one, Acts by three, Romans by one. So "decoded verses plus distinct orphan
  verses equals the feed's count" holds for every book of both editions.
- **Each note's own chapter and verse.** It agrees with the marker's position
  for every note in all three editions, with title notes marked by verse
  zero. Note identifiers run continuously across a whole book rather than per
  chapter, so a future change that resolved a marker against the wrong
  chapter would mis-file a note silently.

**Adopt both as decode-time checks.** Report a book that is short by more
than its orphans, and log a note whose reference disagrees rather than
failing the fetch, since the field has been exactly right across 7,752 notes.
Neither needs a cache epoch bump.

### The offline seed

The embedded seed is the four Gospels of the WEB, 3,778 verses, with no poem
breaks, no notes and no titles. It is used only when a first launch can
neither read a cache nor reach the network, and it is replaced on the next
connected launch. But that is exactly when a new reader first sees the app,
and in that state its poetry reads as prose.

**Regenerate it from the current decoder**, and note in its own comment that
it must be regenerated whenever the decoder's output changes.

## NKJV

### Section headings and what rides with them

The feed sends headings only when titles are requested, which the app now
does for the Psalm titles. In the 174 New Testament chapters available
offline there are 814 headings, about 1.09 times the BSB's density in the
same chapters, which extrapolates to roughly 3,300 in the canon. Four in five
sit between verses rather than at a chapter's head.

Two things differ from the BSB case. The headings are the publisher's
copyrighted editorial content, and the licensing enquiry about displaying the
publisher's apparatus has been sent and is awaiting a reply. And an NKJV cache epoch bump costs
every reader a fresh download against a metered quota, so it should be
batched with another decoder change rather than spent alone.

**Follow the BSB decision, but sequence the NKJV after it**, and before
touching the decoder, run the five-call probe below to find out how many
notes the heading skip is discarding.

The NKJV did follow the BSB, and it now follows it all the way: the headings are
captured with the publisher's own style name — 2,721 of them, 2,674 section
heads and 47 acrostic letters — and drawn by the same block model every other
edition uses. Two things about that are worth saying plainly. The epoch was
spent, batched with the capture; drawing costs none. And the block model is
edition-blind, so nothing in the drawing path consults this edition's licensing
at all — the sequencing this recommendation asked for is not enforced anywhere
in the code, and holds only for as long as someone remembers it. The probe was
never run; what the skip was discarding is moot now that a note inside a heading
is kept on the heading, but the count is still unknown.

### Italics: the translators' supplied words

The NKJV inherits the King James convention of italicising words supplied for
English sense, which are not in the Hebrew or Greek. The feed marks them; the
decoder flattened them, which is the loss this section was written about.

In the 174 chapters available offline there are 2,244 such spans across 1,661
verses, most often a supplied copula or pronoun ("For My yoke *is* easy") and
the genealogical formula "the son of". Density in the epistles is two to
three times that in the Gospels. That extrapolates to roughly 3,000 in the
New Testament. A canon-wide figure cannot be estimated safely from a New
Testament sample, because Hebrew omits the copula more readily than Greek.

Every one of the 2,244 spans was located in the decoded canon by the same
algorithm the red-letter generator uses, with no failures, so the data is
fully recoverable from a cache that already exists.

The table is the easy half. It would reuse the red-letter generator's fetch,
extraction, location and its rune-count and hash guard, and would come to
about two thirds of the red-letter table's size. On licensing, the
repository's position is that an offset is a fact about the text and not the
text itself, and the generated file holds no prose; a supplied-words table is
the same category.

The rendering is the real work. Italics and red letter overlap on the same
characters, because a supplied word can sit inside a saying of Jesus, so the
shared run type must carry two independent flags rather than one, across
three rune-level panes and the token-level styled pane. And there is no
coherent fallback: a stale red-letter verse degrades to whole-verse red, but
a verse cannot degrade to whole-verse italic.

**Worth doing, in two steps.** Generate and pin the table first, which is
mechanical. Then design the two-dimension run type before touching any pane.

Both steps are done, and the first took a different road. There is no generated
table: the decoder keeps the `it` spans as rune offsets on the verse itself
(`Verse.Supplied`), which is the same licensing category — offsets, never
characters, so the stored text is byte-identical whether they are captured or
not — and costs an NKJV epoch instead of a generator run. The second step went
as described: the run type carries red and italic as two independent
dimensions, the supplied spans SPLIT the runs and the small capitals substitute
characters within them, and every surface draws the result, the token-level
styled pane included.

### The apparatus

Every NKJV note is a cross reference, and the section hides cross references,
so all 32,473 are captured and dark, including the 43 attached to Psalm
titles. Those 43 are the most interesting of the set, since they point at the
narrative a superscription alludes to, but they are the same policy question
as the rest.

Lifting the filter would add about 27 rows to an average chapter, against a
current maximum of 25 in the busiest BSB chapter, and the section is the
wording-and-manuscript apparatus; it keeps excluding cross references.
Routing them into the app's own cross-reference panel fits worse than it
appears IF they are mixed in: that panel is built from a public-domain dataset
with its own ordering and its own credit line, and a licensed publisher's
citations interleaved with it would misattribute both.

So they are not mixed. The panel's list is composed in blocks
(`crossref_list.go`), and with the edition's flag on the publisher's notes are
its primary block — one row per note, verbatim, the feed's tagged citations as
links, the parenthesised ones as words, under the edition's own heading and
LicenseNotice — while the Treasury waits in a separately credited disclosure
beneath, because a third of the edition's verses carry no note and a reader
should not be left with an empty block and no way on. The decoder keeps the
feed's citation ids as spans (`Footnote.Refs`) without an epoch bump. The flag
is set for the NKJV only behind the `nkjvxrefs` build tag: **the licensing
answer still decides whether any of it displays**, and no store build can do
so before it comes (docs/SCRIPTURE_WORKLIST.md, S20). The 43 title notes ride
the same rule as the rest, keyed "Title".

### Unknown styles

Nothing leaks today. The paragraph styles the feed actually sends are the
ones the decoder knows, and the character styles are the ones it handles. But
the skip list is exact by design, so a new publisher style would fall through
to the prose path and enter verse text silently.

One caveat the inventory should carry: the titles-on and titles-off
comparison that found no changed verses can only catch a style that the
titles flag strips. A style sent regardless of that flag would appear in both
decodes identically.

**Extend the exact lists to the full heading families, add a decode-time
census of every style seen, and assert the known set in the live canon
test.** Do not make an unknown style a fetch error: an unrecognised but
benign style would take the edition down rather than logging a line. Do the
same for character styles, as a denylist of the families that are not
Scripture, because dropping an unlisted Scripture span is the worse failure.

## Downstream surfaces

**The website** rendered verses only when this was written; it draws the
publisher's section headings now, and still nothing else beside the verses.
Psalm titles are ready to ship: the
accessor is already exported, the change is an italic line ahead of the verse
loop and one style rule, and no URL changes, so the frozen contract is not
touched. The footnote section is a larger question, because the site has no
settings and therefore no toggle, so it would have to be always visible or
disclosed without scripting. Ship the titles, defer the section.

**Search** does not index titles, so "Absalom" does not find Psalm 3. This is
the pipeline where the omission most reads as a loss, and the fix is confined
to the index plus a decision about where a title hit opens. **Index them.**

**Speech** reads titles (S18). The read-along model's rows are verse 0 for
the title and n ≥ 1 for verse n, with a negative sentinel for nothing
narrated; the spoken text leads with the title as a sentence of its own, and
the recorded tables were re-aligned with the title in the transcript. Every
pane paints verse 0 as the title, and nothing in a chapter without one.

**Sharing** omits titles, and there is no whole-chapter share to attach one
to, so this is low priority.

**Copying** is already correct on the native Apple panes: the clamp bounds
the app's own actions, while the system's own copy takes what the reader
selected, title included. No change.

**Poem breaks** are flattened in two places that never said why. In the
Android fallback pane's long-press menu, copying a verse or a paragraph
flattens the lines while copying the chapter, in the same menu, keeps them.
Make them agree. In cross-reference snippets the flattening is defensible for
a short preview, but it should be written down rather than left silent.

**The Fyne fallback panes** are genuinely unreachable in what ships. One is
not compiled on macOS at all and needs a source edit to reach elsewhere; the
other is reachable only by building Android without the documented release
script. No feature work is warranted. Pin the release path with a check, so
the claim keeps being true.

**Omitted verses** leave a hole in the numbering with no explanation unless
footnotes are on, and 34 chapters across the corpus do this. The least
invasive answer is to mark the gap only when footnotes are already on, which
extends a decision already taken and keeps the page identical when they are
off.

## Recommended order of work

The worklist that tracks this, item by item with a status, is
`docs/SCRIPTURE_WORKLIST.md`. The order below is its stages. It is kept as
written, with what has since been done marked, because the order was the
argument: several of these ran ahead of their place in it, and the last two
entries are where that shows.

1. **Defect 2**, the curly quotation mark. One line, no epoch, improves every
   edition on every surface immediately. DONE, and then superseded by 6.
2. **Defect 1**, the acrostic leak, with descriptive-run capture. Removes
   wrong text from search, sharing and speech. Epochs for `web` and `webc`.
3. **The defensive checks**, all of them together, so the four editions
   report alike: the feed's verse counts, the note reference field, the
   helloao node and shape census, the NKJV style census with the extended
   lists and the character denylist, and the words-of-Jesus cross-check. No
   epochs, and they make every later change safer.
4. **The NKJV heading probe**, five calls, which turns the last unknown into
   a number.
5. **Website Psalm titles**, and **search indexing of titles**. Small, and
   they finish work already done everywhere else.
6. **Source paragraph boundaries**, hybrid, with the three helloao epochs
   taken once. DONE, and not hybrid: the app's own rule was removed.
7. **Section headings** behind a toggle, BSB and WEB Catholic first with the
   shape guard, NKJV after, batched with its epoch. Captured and DRAWN, on all
   four editions, without the toggle and without the shape guard — and ahead
   of 3 and 4, which were to make it safe.
8. **NKJV supplied words**: table first, then the run-type design. DONE, with
   decoder-kept offsets in place of the table.
9. **Defect 3** whenever a printed NKJV is to hand.

Small items that can ride with any of the above: regenerating the offline
seed, the Android long-press copy, the release-path pin, and the omitted-verse
marker.

## What stays skipped, and why

To be written into `docs/SOURCE_FIELDS.md` so those rows stop reading as open
questions:

- **Poem indent depth**: cosmetic, needs a per-line side-band across four
  surfaces and three epochs, and a single-line poem verse already reads as
  prose. Revisit only with source paragraphs.
- **Selah**: the publisher's placement is the text; normalising it is an
  editorial act on someone else's edition.
- **Book and chapter metadata**: names come from the app's own canon, which
  must serve editions that name books differently; the app self-hosts its
  audio and timings.
- **helloao note kinds**: the apparatus is 8% citations at most, so
  classifying by wording would risk hiding real glosses to solve very little.
- **The Fyne fallback panes**: unreachable in shipping builds, so their
  missing title and section are not reader-facing.
- **Cross-reference snippet flattening**: a deliberate one-line preview.
