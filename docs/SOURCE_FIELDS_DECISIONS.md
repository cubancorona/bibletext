# Source fields: analysis and recommendations

The companion to `docs/SOURCE_FIELDS.md`. That file is the inventory: for
every field each source sends, whether the app keeps it, skips it for a
stated reason, or leaves the question open. This file takes each skip and
each open row in turn, says what is actually lost, what it would cost to
keep, and what to do.

Three of the items below are not decisions at all. They are defects found
while measuring, and they are listed first.

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
decoder drops the acrostic style on purpose, its comment records that these
headings once leaked into verse text, and the live canon test asserts that
Psalm 119:1 does not contain "Aleph". The NKJV canon is clean; the two WEB
editions are not.

**Fix it.** Route descriptive runs out of verse text into the chapter's
side-band, where superscriptions already live, and ignore a Hebrew subtitle
whose whole text is an acrostic letter. A denylist of the 22 letter names
would be smaller but would not survive a differently marked acrostic psalm.
Cache epoch bump for `web` and `webc`. Whether the letters are later shown as
stanza markers is the same product question as section headings and should
follow that decision.

### 2. The paragraph rule ignores curly quotation marks

Paragraphs are made by one shared function: a break happens once the
paragraph has reached 320 characters and the previous verse ends in a full
stop, exclamation mark, question mark, or a straight quotation mark. The text
uses curly quotation marks, so a verse that closes reported speech is not
recognised as a place to break, and the paragraph runs on.

| edition | break points past the length threshold | refused only for a curly quote |
|---|---|---|
| BSB | 11,536 | 2,094 (18.2%) |
| WEB | 11,978 | 2,272 (19.0%) |

Nearly a fifth of eligible breaks are suppressed by a character class the
rule does not know. Every edition and every surface is affected, because they
all funnel through this one function. It is a one-line fix and needs no cache
epoch bump, since paragraphs are computed at render time.

### 3. The NKJV names Jesus in capitals in four verses

The decoder renders a small-caps span as full uppercase, which is right for
the divine name and reassembles the source's split letters into GOD. Across
the canon that yields LORD 6,504 times and GOD 308 times, plus a set of
genuine inscriptions that print also capitalises: the notice on the cross in
all four Gospels, the writing on the wall in Daniel 5, "HOLINESS TO THE LORD"
on the priest's plate and the horses' bells, "THE LORD OUR RIGHTEOUSNESS",
"THE LORD IS THERE", "TO THE UNKNOWN GOD", and the names written in
Revelation 17 and 19. All of those read correctly.

Four verses are the exception, and they are the only ones: Matthew 1:21,
Matthew 1:25, Luke 1:31 and Luke 2:21 each now read "call His name JESUS".
The feed marks the name with the same small-caps style it uses for the divine
name, and the app has no small caps, so it uppercases.

Whether a printed NKJV sets small caps there cannot be checked from anything
on this machine. **Look at a printed copy before changing anything.** If print
does not, the fix is a narrow exception for that word; if it does, the
current output is as close as flat text gets.

## Decision table

Effort is XS for a line or two, S for under a day, M for a few days, L for
more. "Epoch" means a cache epoch bump, which forces every reader of that
edition to download the text again.

| # | item | editions | what is lost today | recommendation | effort | epoch | confidence |
|---|---|---|---|---|---|---|---|
| 1 | acrostic letters in verse text | WEB, WEBC | 21 verses plus one false title | fix | S | web, webc | high |
| 2 | curly quotes in the paragraph rule | all | ~2,100 breaks per edition | fix | XS | none | high |
| 3 | "JESUS" in capitals | NKJV | 4 verses | check print first | XS | none | medium |
| 4 | section headings | BSB, WEBC, NKJV | 3,091 / 5 / about 3,300 | capture and render behind a toggle | M–L | bsb, webc, later nkjv | medium |
| 5 | source paragraph boundaries | all | 13,894 BSB breaks, 742 WEB | honour as extra breaks, keep the rule as fallback | M | three helloao | medium-high |
| 6 | italics for supplied words | NKJV | about 3,000 in the New Testament | generate the table, then design the run type | M–L | none | medium |
| 7 | poem indent depth | all | every second-level line | keep skipping | — | — | medium |
| 8 | Selah placement | WEB, BSB | set differently in each | keep the source's placement | — | — | medium |
| 9 | descriptive runs | BSB | one oracle title | capture with item 1 | S | with item 1 | medium |
| 10 | words-of-Jesus flag | WEB, WEBC | nothing; tables agree exactly | keep tables, add a decode-time cross-check | S | none | high |
| 11 | NKJV cross references hidden | NKJV | 32,473 notes | wait for the licensing answer | — | — | high |
| 12 | note callers and anchors | all | in-text markers | keep dormant, fix the Android scan first | — | — | high |
| 13 | website: Psalm titles | WEB, WEBC, BSB | every title | ship | S | none | high |
| 14 | website: footnote section | WEB, WEBC, BSB | the whole apparatus | defer, needs a presentation model | M | none | medium |
| 15 | search does not index titles | all | "Absalom" finds nothing | index titles | S | none | medium-high |
| 16 | speech does not read titles | all | the title before verse 1 | do it after the read-along model allows it | M | none | medium |
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
length rule. But the WEB has 742 breaks in the whole Bible and none at all in
Genesis 1, so honouring the source alone would turn a WEB chapter into one
unbroken block.

**Honour the source's breaks as additional forced breaks and keep the current
rule as the fallback where the source is silent.** A paragraph-start flag on
the verse, set where the decoder currently skips a break node, plus one
branch in the shared grouping function; every surface inherits it without its
own change, which is what that shared function is for. Fix defect 2 in the
same pass, because the fallback keeps running inside long unmarked stretches.
One design question is open: whether a source break also resets the length
counter. The NKJV needs new decoder work to detect a verse that opens a
paragraph block, so it should follow rather than hold up the rest.

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
publisher's apparatus is still unsent. And an NKJV cache epoch bump costs
every reader a fresh download against a metered quota, so it should be
batched with another decoder change rather than spent alone.

**Follow the BSB decision, but sequence the NKJV after it**, and before
touching the decoder, run the five-call probe below to find out how many
notes the heading skip is discarding.

### Italics: the translators' supplied words

The NKJV inherits the King James convention of italicising words supplied for
English sense, which are not in the Hebrew or Greek. The feed marks them; the
decoder flattens them.

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

### The apparatus

Every NKJV note is a cross reference, and the section hides cross references,
so all 32,473 are captured and dark, including the 43 attached to Psalm
titles. Those 43 are the most interesting of the set, since they point at the
narrative a superscription alludes to, but they are the same policy question
as the rest.

Lifting the filter would add about 27 rows to an average chapter, against a
current maximum of 25 in the busiest BSB chapter. Routing them into the app's
own cross-reference panel fits worse than it appears: that panel is built
from a public-domain dataset with its own ordering and its own credit line,
and mixing a licensed publisher's citations into it would misattribute both.

**Wait for the licensing answer**, and keep the 43 title notes with the
general decision rather than carving them out.

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

**The website** renders verses only. Psalm titles are ready to ship: the
accessor is already exported, the change is an italic line ahead of the verse
loop and one style rule, and no URL changes, so the frozen contract is not
touched. The footnote section is a larger question, because the site has no
settings and therefore no toggle, so it would have to be always visible or
disclosed without scripting. Ship the titles, defer the section.

**Search** does not index titles, so "Absalom" does not find Psalm 3. This is
the pipeline where the omission most reads as a loss, and the fix is confined
to the index plus a decision about where a title hit opens. **Index them.**

**Speech** does not read titles. The text change is trivial; the work is the
read-along model, which has no slot for a title before verse 1. Do it when
that model gains one.

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

1. **Defect 2**, the curly quotation mark. One line, no epoch, improves every
   edition on every surface immediately.
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
   taken once.
7. **Section headings** behind a toggle, BSB and WEB Catholic first with the
   shape guard, NKJV after, batched with its epoch.
8. **NKJV supplied words**: table first, then the run-type design.
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
