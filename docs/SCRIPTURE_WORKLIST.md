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
Items S2 and S14 both need the helloao editions; S16's NKJV half
should wait for another NKJV decoder change to travel with.

**Checks before changes.** The decode-time checks in stage 2 cost no epoch
and no reader-visible change, and every later item is safer with them in
place: they are what would make a bad decode visible instead of silent.

## The leaks, and the one that is left

Text leaving the app had the reading surface's own characters in it. Three
paths are fixed: the assistant received the raw selection and sent it to an
external provider, the desktop pane's Copy put its superscript verse numbers
on the clipboard, and the website joined every number to its verse with a
no-break space that a browser copy carried away. One cleaner now serves every
outbound path, and the native panes no longer put a no-break space in the text
at all, since the system's own Copy reads the storage directly and cannot be
reached.

ONE LEAK REMAINS, and it cannot be fixed by stripping. In the presented
reporter layout a paragraph opens with an em-space and an en-space, written as
literal characters because the platform HTML importers ignore the CSS that
would indent it. The app's own verbs strip them; the system's Copy on a phone
in landscape does not. Removing them means indenting some other way: a
first-line indent applied to the imported paragraph style on the Apple panes,
and a leading-margin span on Android. Native work on two platforms, and until
it is done a paragraph copied out of the landscape reader begins with two
spaces nobody typed.

## Stage 1 — the defects

Found while measuring for the analysis. These are not product decisions.

S1 is done: the rule now ends a paragraph on a typographic closing quotation
mark as well as a typewriter one. S14 is done with it, and supersedes most of
S1's effect: every edition now paragraphs where its publisher does, and the
rule runs only in a chapter that carries no marks at all. S3 turned out not
to be a defect. S2 is the one still open.

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S1 | paragraph rule ignores curly quotation marks | all | XS | none | done |
| S2 | Psalm 119 acrostic letters in verse text | WEB, WEBC | S | web, webc | todo |
| S3 | "JESUS" set in capitals in four verses | NKJV | — | none | closed, not a defect |

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

**S3.** CLOSED, not a defect. The publisher marks the name with the
small-caps style at those four verses and means to: its own web edition sets
them in small caps. The app has no small caps anywhere, and folds a
small-caps span to uppercase in the stored text, which is the standard
plain-text rendering of small caps and the same treatment that produces LORD
for the divine name. So "call His name JESUS" is a faithful flattening of
what the publisher set, not a decoder fault.

What remains is a display question rather than a correctness one, and it is
not on this list because it is not about the text. True small caps could be
drawn on the panes that render markup, but only by keeping the text in mixed
case and applying the style at render time; today the uppercase IS the text,
which is what lets a search for LORD behave, and what keeps sharing, speech
and links agreeing with the page. Changing that trades a typographic gain for
a change to four pipelines, and nothing suggests the trade is wanted.

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
| S12 | regenerate the offline seed from the current decoder | seed | S | none | done |
| S13 | Android long-press copy keeps poem lines | Android fallback | XS | none | todo |

**S10.** The site renders verses only. The accessor is already exported, so
this is an italic line ahead of the verse loop and one style rule. No URL
changes, so the frozen contract is untouched. Decide at the same time whether
a title should lead a psalm's link preview.

**S11.** A search for "Absalom" does not find Psalm 3, because the index is
built from verse text alone. Index the titles, keyed so a hit opens
somewhere sensible, and decide whether a title hit is labelled as such in the
results.

**S12.** DONE. The embedded seed was the four Gospels with no poem breaks, no
notes and no paragraphing — a snapshot of a decoder several changes old, shown
at exactly the moment a new reader first sees the app. Regenerated from a
current decode: of its 3,778 verses, 76 now carry authored poem lines, 214
carry the translators' notes and 1,409 open a publisher's paragraph, against
none of each before. `scripts/gen-seed-gospels.py` rebuilds it, `seed.go` says
when to, and `seed_content_test.go` fails if it drifts back.

**S13.** In the Android fallback pane's long-press menu, copying the chapter
keeps poem lines while copying a verse or a paragraph flattens them, with no
stated reason. Make them agree.

## Stage 4 — the larger questions

| id | item | editions | effort | epoch | status |
|---|---|---|---|---|---|
| S14 | honour the publishers' paragraph breaks | all | M–L | all four | done |
| S15 | section headings: captured | all | M | all four | done (capture) |
| S16 | section headings: drawing them | all | M–L | none | todo |
| S17 | NKJV supplied words: captured | NKJV | M | nkjv | done (capture) |
| S18 | speech reads the Psalm title | all | M | none | todo |
| S19 | mark an omitted verse's gap in the text | all | M | none | todo |
| S20 | show the NKJV's cross references | NKJV | S | none | blocked |
| S21 | website renders the footnote section | website | M | none | todo |
| S22 | in-text footnote markers | all | L | none | blocked |
| S23 | draw small caps as small caps | NKJV | M | nkjv | todo |

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

**S23.** Today a small-caps span is folded to uppercase in the stored text,
so LORD, GOD and the four names of Jesus are literally capitals. Drawing them
as small capitals is possible, and cheaper than it first looks.

The mechanism exists. Red letter already carries per-rune-range styling to
every surface: the Apple builder and the Android dialect wrap a run in
markup, and the styled canvas pane carries a flag on each run and measures it
itself. Small caps ride the same path.

It also needs no change to the text, which is what makes it safe. Because the
text is ALREADY uppercase, a renderer synthesises small caps by drawing the
first letter at full size and the rest smaller — no case change, so the rune
count is identical and every selection offset, share, copy, link, search and
spoken word is untouched. The earlier claim that this would cost four
pipelines was wrong.

Two things it does cost. The decoder discards which spans were marked, so
recovering them is an NKJV cache epoch. And the reading face has no
small-capital glyphs — Georgia has none, nor does the embedded Gelasio
fallback, nor Times New Roman — so they would be synthesised by shrinking
capitals, which reads thinner than the text around it. Two faces already in
the repository do carry real ones, Cardo and Spectral, but they are share-card
faces, not the reading face.

So it is the same structural work as S17: the run type carries one flag today
and would need to carry independent style dimensions. Do them together. Whether to
synthesise small capitals or wait for a face that has real ones is answered
below, under "The reading face": wait.

## The reading face

S23 ends on a question this section answers: whether small capitals should be
synthesised, or wait for a reading face that has real ones. It waits. And the
face that has them is also the face that fixes a second problem, which is that
the same chapter is set in four different types depending on where it is read.

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

### What is actually manufactured here

Separately from which face is used: the decoder uppercases every `sc` and `nd`
span into the stored text. The publisher did not spell the word `LORD` in four
capitals; it sent an initial capital and a small-capital remainder, and the app
replaced that with capital letters and then forgot where the span was. A reader
who copies a verse out of the app gets a spelling no edition of the NKJV
prints. Under the standard at the top of this document that is an addition, and
it is the same shape as the verse-number superscript: keep the typography on
the page, and let the plain text leaving the app carry the conventional
uppercase realization, which is what every other edition's copy behaviour
produces.

The fix is the one already used for supplied words — mark the span with
sentinels, resolve it to rune offsets once the text has settled, store the
offsets and not a case change. It is an NKJV cache epoch either way, so it
should be done in the same epoch as S17's supplied words rather than a second
one.

## Display issues found while unifying the reading face

Checked by rendering or measurement, not by reading code. Corrections of
earlier conclusions are kept AS corrections, because the wrong version is the
one that sounds plausible and will be arrived at again.

### Must not ship

**The divine name draws as ordinary text.** The decoder keeps the publisher's
characters and records the small-capital spans; nothing reads them yet, so the
NKJV shows `Lord` where it showed `LORD` and the distinction the edition
carries in its letterforms is invisible.

**Fifteen verses paint the narrator's words red.** The red-letter table refuses
a verse unless a hash of its text matches, and a refusal paints the whole verse
red. The case change keeps the rune count, so only the fingerprint trips —
silently, since the licensed text is not in this repository. Where Christ quotes
the Old Testament and the quotation carries the divine name, the narration is
printed as his. Guarded now by an epoch assertion that fails the build; the
table must be regenerated against the new decoder.

**macOS has no paragraph separation at all.** No blank line and no indent: the
reporter stylesheet emits `margin: 0`, and the first-line indent was moved out
of the text and re-applied on iOS only. A new paragraph begins flush left on the
next line, indistinguishable from a wrap, so the publishers' paragraphing is
invisible there.

### Fixed

**The verse number left its line.** Twice. The lift was first taken from the
face's DECLARED line box, which is a third larger in the shipped face; the
correction that replaced it had the wrong sign, because the numerals are
Unicode superscripts already and the shipped face draws them 0.232 em higher
than the borrowed one. It is a drop of 0.117 em, and both leadings are pinned
by a test, at the shipping body size — a bare test theme puts every width on the
same side of the reporter gate, so only one leading is otherwise reached.

### Visible, still open

**Greek words are split between two faces mid-word.** The shipped face carries
exactly three codepoints of the Greek block — Δ, μ and π, kept for mathematics
— so `ἐπίσκοπον` draws two letters in the reading face and seven in a system
fallback, differing in weight, colour and fit inside one word. This is the
corrected form of an earlier claim that Greek would be LOST: it is not, the
toolkit falls back per rune, and that is exactly what makes the split.

**A share card can silently drop a letter.** Prata has no `Ē`, and its .notdef
is an invisible blank, so a card of Genesis 4:18 reads `“ ´noch`. The seven
faces are cycled deliberately, so a reader who taps Regenerate reaches it. An
earlier check called every face complete; it was made against the
public-domain editions, which contain no `Ē`.

**Verse numbers change shape on Windows and Linux only.** The borrowed serif's
figures are oldstyle by default and the shipped face's are lining. Three
surfaces can ask for `onum`; the canvas pane cannot, because the toolkit exposes
no OpenType feature control — the same gap that blocks small capitals there.

**One chapter, four faces.** Only the canvas surfaces took the shipped face. The
Apple panes, Android and the website still ask for the borrowed one.

**Android's line pitch is wrong, and was already.** `setLineHeightPx` reads the
CURRENT paint's metrics and stores the difference; the pane sets the typeface
afterwards, so the pitch is out by the gap between the two fonts. Lines carrying
a verse number are 10-11px taller than their neighbours, so a wash looks stepped.

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

Section headings, the translators' supplied words, poetry indent depths, and now
the small capitals.

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
