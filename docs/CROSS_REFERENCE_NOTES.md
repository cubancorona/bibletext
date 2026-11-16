# Publisher cross-reference notes: what a note carries, and where it is shown

A ledger for the cross-references panel's publisher run
(docs/SCRIPTURE_WORKLIST.md, S20). Decided 11 November 2026: nothing the
edition's notes carry is to be lost — not a citation, not the order, not the
grouping, not the punctuation that says what belongs with what, not the word a
note hangs from. This document lists each thing a note carries, how many the
canon holds, where the panel shows it, and the test that holds it, so that a
change to the panel can be checked against the list rather than against a
memory of it.

The NKJV's apparatus below is behind the `nkjvxrefs` build tag, which no store
build compiles while the licensing enquiry waits on a reply (docs/BACKLOG.md,
"NKJV cross references: the panel's second pass"); the same publisher-run
model ships in every build for the public-domain and CC0 editions' own
recovered cross-references (`publisher_xrefs_public.go`, below).

The NKJV figures are of the canon fetched on 6 November 2026. Those marked
"census" are pinned by `canonPieceCensus` in `publisher_xrefs_census_test.go`
and asserted against a live fetch by `TestLiveAPIBibleFullCanon`; the census
measures a canon through the same functions the panel draws from, so it
counts what is shown, not what was meant. A figure marked "counted" was
counted once on 11 November 2026 and is not held by a test.

## What the NKJV feed sends, and what the decoder keeps

Every note in the feed is a cross-reference note (style `x`), and its caller
is `-`, the form that prints no caller. Inside a note the feed marks the
verse the note belongs to (`xo`), the citations (`xt`) and, in three notes, a
basis (`xta`). The decoder keeps a note as `Footnote`: its place in the
verse's text as a rune offset (`Anchor`), its text without the `xo` span
(`Text`), its kind, its caller, and each citation the feed tagged with a
machine-readable target as a span of the text (`Refs`). Notes on a Psalm's
title are kept on the title (`Superscription.Footnotes`) and a note on a
section heading on the heading (`Heading.Footnotes`); the decoder keeps no
place within a heading.

The feed carries no letter for a note. The print edition keys each note with
a small raised letter before a word; the letters in the panel are the
panel's own, in the order the notes stand.

## The ledger

| What a note carries | In the canon | Where the panel shows it | Held by |
|---|---|---|---|
| **Where it stands** | | | |
| It hangs from a verse | 32,473 notes on 21,249 of 31,102 verses (census; the verses counted) | A letter before its word in the passage at the head of the list; the note directly under the passage under the single `NKJV REFERENCES` label (before `GOSPEL PARALLELS` and the Treasury), in selection order | `TestThePassageIsTheSelectedTextWithALetterBeforeEachNotesWord`, `TestCrossRefListHasOnePublisherSectionForTheSelection` |
| It hangs from a Psalm's title | 43 notes on 38 of 116 titles (census; the titles counted) | The title stands before verse 1 in the passage, in italic as the page sets a title, opening the selection's letter run; its notes lead under the single `NKJV REFERENCES` label before the verses' notes | `TestThePassageIsTheSelectedTextWithALetterBeforeEachNotesWord`, `TestCrossRefListHasOnePublisherSectionForTheSelection` |
| It hangs from a section heading | 1, before Numbers 4:21 (census) | The heading stands in the bold cut and text colour before its verse in the passage, as the reading page sets a heading, the letter at its head; the note stands in order under the single `NKJV REFERENCES` label | `TestAHeadingsNoteStandsWithTheVerseItIsSetBefore`, `TestCrossRefListHasOnePublisherSectionForTheSelection` |
| The word it hangs from | every note but 2 (census: `Unkeyed`) | The letter is drawn directly before the word (`noteMarkAt`), with `readingMarkGap` (`1.5pt`) of air after an italic supplied word and standalone opening quotation marks held with the word that follows across line breaks. 1,998 offsets stand before white space and are drawn after it: a line break 1,952 times, a space 46 (census: `Moved`; the split counted) | `TestANotesLetterAndWhereItStands`, `TestPassageLetteringTouchesItalicGapAndOpeningQuote`; the census reports a letter that no word follows |
| The order of a selection's notes | at most 7 on a verse (census); on single verses, letters a 21,287, b 8,241, c 2,316, d 536, e 113, f 19, g 3 (counted) | One continuous spreadsheet run across the whole selection — title, noted headings and verses alike — a, b, c … z, aa, ab …; double-letter marks step down to 10 points inside the 18-point margin | `TestANotesLetterAndWhereItStands`, `TestMultiVerseSelectionRunsOneSeriesOfLettersAcrossTitleAndVerses`; the census checks every index |
| **The printed text** | | | |
| The note as printed | 32,517 notes | A note of several pieces: verbatim on its own row, in both states of the fold. A note that is one parenthesised group: verbatim on that group's row, closed until tapped. A note that is one citation: its card, whose reference is the citation spelled out (one such note cites the reader's own verse and is its reference, shown and not followed) | `TestANoteOfSeveralPiecesFoldsToItsPrintedLine`, `TestANoteThatIsOneGroupIsDrawnAsAGroupRow`; `TestTheShorthandPrintsWhatItReads` (the shorthand printed back from the card is the note's text for all 12,576 notes of one citation) |
| The separators: "; " between citations, ", " before a further verse | every gap of every note is one of the two, and nothing stands before the first piece or after the last (census: no exception) | On the printed line, which turns after "; " and not inside a citation unless the citation is longer than a row (`crossref_printed.go`); the pieces under it are in the same order | `noteGapProblems` and `printedTurnProblems`, run over the canon by the census; `TestAPrintedLineTurnsBetweenCitations` |
| A book or chapter carried from the citation before ("John 7:50; 19:39") | — | The printed line shows the continuation as printed; its card is labelled in full ("John 19:39") | `TestTheShorthandReadsEveryShapeTheEditionPrints` |
| **The citations** | | | |
| A citation the reader can follow | 65,296 at a note's level and 8,323 inside groups (census) | A card: the reference, a preview from the loaded edition, the whole card the tap | `TestANoteIsItsPiecesInTheOrderItPrintsThem`, `TestAGroupRowOpensIntoItsCitations` |
| The target the feed tagged | 66,306 tags; 66,294 agree with the print's reading and 12 do not, all inside groups | Outside a group the tag is followed; inside a group the print is | `TestNoteShorthandCensusHoldsTheShorthandToTheTags`; the live canon test pins 4,850 groups, 8,324 group citations and the 12 |
| A citation of a verse in the selection | 9 at a note's level and 0 inside groups on single-verse selections (census) | Its reference in full, with no preview and nothing to tap, in its printed place, set as a line of the edition's print (muted, 14 points); a range citation that runs beyond the selection is a tappable card, and the same rule governs Treasury rows | `TestANoteIsItsPiecesInTheOrderItPrintsThem`, `TestASelfRangeBeyondTheSelectionIsACardAndExactSelfStaysMuted` |
| A citation that cannot be followed | 3 (census: 2 at a note's level, 1 inside a group): Ezekiel 33:12 "Ezek. 8:21" and Mark 4:29 the "39" of "(Mark 13:30, 39)", verses the text does not have; Amos 2:1 "Is. 15:1—16", truncated from "Is. 15:1—16:14" (printed 1983–2018 as "Is. 15:1–16", emended in 2023 to "Is. 16:1–6") | Its words as printed, muted, in their place | the census (`Words`, `GroupWords`); `TestTheShorthandLeavesWhatItCannotReadUnread` |
| A citation of a Psalm's title ("Ps. 59:title") | 47 (the apparatus census, `noteApparatusCensus`; the live canon test asserts that title, LXX and DSS are the apparatus's only words) | A card labelled by its chapter, tagged "Title", previewing the superscription, navigating to the chapter's head without washing verse 1; the whole psalm is a different card | `TestATitleCitationResolvesToTheTitle`, `TestATitleCardPreviewsTheSuperscription`, `TestATitleAndTheWholePsalmAreTwoCards`, `TestATitleCardNavigatesWithoutWashingVerseOne` |
| A citation of a whole chapter | 70 tagged (117 whole-chapter ids in the apparatus census, which includes the 47 followed by ":title") | A card labelled as a chapter ("Leviticus 11", "2 Kings 18–20") | `TestPublisherCrossRefsLinkWholeChapters` |
| A basis (", LXX"; ", LXX, DSS") | 3 notes (the apparatus census) | A tag on the citation's card; after a list of verses it is the whole citation's ("Ps. 45:4, 5, LXX", Revelation 6:2) | `TestABasisRidesItsCitation`, `TestABasisAfterAListOfVersesTagsEachOfThem` |
| **Grouping** | | | |
| A parenthesised group | 4,850 (census); 1,985 are the whole of their note | Wherever it stands — inside a note of several pieces, or as the whole of its note beside the letter's margin — a row carrying the group as printed with its parentheses, closed until it is tapped, that opens in place into the group's pieces on a well of its own | `TestANotesGroupsStandWhereTheEditionPrintsThem`, `TestAGroupRowOpensIntoItsCitations`, `TestAGroupOpensInsideThePanelWithoutMovingIt`, `TestANoteThatIsOneGroupIsDrawnAsAGroupRow` |
| What stands inside a group | 8,323 cards, 0 of the reader's own passage, 1 that is words (census) | Each in its printed order under the opened row, by the rules above | `TestAGroupRowOpensIntoItsCitations` |
| The book carried into a group and out of it | — | The cards are labelled in full | `TestTheShorthandReadsEveryShapeTheEditionPrints`, `TestTheShorthandFindsGroupsByTheirParentheses` |
| Which rows are one note's | 32,517 notes; 17,956 of several pieces open over their rows, and 1,985 that are one group open onto their well when tapped (census) | Each note stands on a ground of its own with a gap before the next; its letter stands alone in a margin at the ground's left, and everything else of the note beside the margin; a rule hangs from the letter down the margin beside a note's rows and beside an opened lone group's well, and while an open note or opened lone group scrolls past the top of the list its letter stays pinned near the top of the margin above its rule (`crossref_section.go`, `crossref_note.go`, `crossref_group.go`) | `TestEachNoteStandsOnAGroundOfItsOwn`, `TestALetterStaysInViewWhileItsNoteRunsPastTheTopOfTheList` |
| **Repeats** | | | |
| Two notes of a verse citing one passage | the repealed rule "one card, the first" had withheld 245 cards on 224 single-verse selections (counted) | Each note keeps its card | `TestANoteIsKeptWholeThoughAnotherNoteHasItsPassage` |
| **The text the notes hang from** | | | |
| The verse as the page draws it | every verse with a note | The passage: small capitals, the translators' supplied words in italic, the words of Christ in red where the reader has that on, poem lines on rows of their own. With the letters taken out it is the text, rune for rune | the passage test; the census compares every verse |

What a note is (census): 12,576 one citation; 1,985 one parenthesised group
and 17,956 several pieces. A note of one citation is drawn as its card,
except the one that cites the verse the reader is on (Revelation 15:3),
drawn as its reference, muted; a note of one parenthesised group is drawn as
that group's row, closed until tapped; and a note of several pieces is drawn
as a row that folds, open as the panel opens. These four figures are of what
a note is; the census's other figures are of what is shown.

## What the panel adds

Publisher note wording stays unchanged. The panel adds navigational labels
and marks, none of which is in `Verse.Text` or leaves the panel
(docs/ADDITIONS_AND_DROPS.md):

- the publisher-note letters, for the reason above;
- a verse's number in the passage, where the passage holds more than one
  verse or a title before the verse, so that a letter is read against the
  right verse;
- an upright origin-verse number beside a Treasury group, or an actual
  heading or episode title with its source location beside that group's
  references;
- the disclosure icon on a row that opens.

And devices of layout that are no mark in the text: a note's ground, the
margin its letter stands in, and the rule that hangs from the letter beside
the note's rows. Together they show what the edition shows by listing a
letter's references together.

Source relationships are preserved independently. A destination present in a
publisher note, a Gospel episode and a Treasury group remains in all three;
a destination contributed by two selected verses remains in each verse's
Treasury group. Neither an exact match nor an overlapping passage in another
source suppresses it. Duplicate Treasury or Gospel lookup hits inside the
same origin group are removed by destination identity; publisher notes retain
their printed pieces.
Publisher parenthesised groups retain their own rule: closed initially,
whether inside a note or forming the whole note.

While the Treasury zip downloads on first use (or retries after an offline
failure), the loaded publisher references and embedded `GOSPEL PARALLELS` rows
show immediately from the loaded `BibleData`, with the loading indicator or
inline `Try again` scoped under `TREASURY OF SCRIPTURE KNOWLEDGE` and open
folds and scroll position preserved when the Treasury rows land
(`TestPublisherNotesShowImmediatelyWhileTreasuryLoads`).

A citation of the reader's own passage is shown as its reference in full,
so where the edition prints it without its book the book is supplied on that
row; the note's printed line above it is unchanged.

The selected passage heads every populated reference list, including
Treasury-only, Gospel-only and heading-only BSB results. Publisher letters
appear only where a note has an actual word anchor; the other excerpts show
the selected verses with their verse numbers. Empty results and standalone
loading/error states retain their existing presentation.

Letters and verse numbers inside the selected passage use the matching
groups' accent and bold UI faces: italic letters and upright plain-zero
numbers. They are 14 points at Normal, at the printed-citation size, with a
modest raised baseline; Scripture remains 17 points before optical correction.
An adjacent verse number and reference letter have a quarter-em gap, measured
at the marker size, without added punctuation or a break between them.
This emphasis is confined to the cross-reference excerpt. A Treasury
group's margin key is instead an upright source-verse numeral at the card
reference size; its measured margin widens for three digits without shrinking
them or changing publisher-letter geometry. References in the panel's
header, cards and own-passage lines and printed shorthand lines on note and
group rows draw from plain-zero cuts derived in memory from the embedded
Atkinson Hyperlegible faces (`crossref_zero_font.go`,
`TestCrossRefPlainZeroFacesAndPanelReferences`). Only the reviewed zero-glyph
fingerprints are transformed. Both derived weights are published together; if
either font revision is unrecognised, both use the theme's ordinary faces
(`TestPlainZeroRejectsChangedGlyphGeometry`, `TestPlainZeroFacesArePublishedTogether`). The list scales with the
reader's `Normal`, `Large` and `Extra large` text size (`crossref_sizes.go`,
`TestCrossRefPanelScalesWithReaderTextSize`,
`TestTheCanonFitsTheNarrowestPhoneWithEveryGroupOpen`), scaling the passage
and card previews in full and stepping headings, margin letters, margins,
printed shorthand rows, tags, wrapped pericope titles and inline captions up
in strict hierarchy while keeping pinned footer credits steady. On landscape
phones and short windows where the available height is below
`crossRefPanelHeight`, the source credits and edition notice move into the
foot of the scroll body above Close
(`TestShortAndLandscapePanelsScrollCreditsWithTheList`).

## What is known to be wrong in the feed, and what the panel does

| The defect | What the panel does | Status |
|---|---|---|
| Two stretches hang every note one caller late: Ruth 3:12 to 2 Samuel 24:25 (1,640 callers) and Psalm 119:133 to Psalm 150:2 (323). The feed lacks a caller on Ruth 3:11 and a second on Psalm 119:132, and the last caller of each stretch has no content | Shows each caller in a stretch with the content delivered on the next (1,961 notes), the first content of each stretch with its verse and no letter (2), nothing for the second caller of 2 Samuel 24:25 (whose first note is shown), and `"The NKJV has a cross-reference here that isn't in our source."` at Psalm 150:2 where the verse has no other note (`publisher_xref_displaced.go`, `crossref_list.go`, `TestANoteTheFeedHangsOneCallerLateIsShownOnItsOwn`, `TestPsalm150Verse2SaysTheCrossReferenceIsMissingFromOurSource`). Only while the loaded text carries the marks of the feed as found; the stored notes are unchanged | Built (decided 12 November 2026) |
| The heading's note. A second publication of the apparatus has it in Numbers 4:22, before the word the heading ends with | Shown where the feed has it, on the heading: the heading stands in the bold cut and text colour in the passage, and the note stands in order under the single `NKJV REFERENCES` label (`TestAHeadingsNoteStandsWithTheVerseItIsSetBefore`) | Built (decided 12–13 November 2026) |
| Twelve tags inside groups name the wrong book: where a group opens with a numbered book, a later citation of the group that omits its book is tagged with the book of the citation before the group ("(1 Cor. 1:23; 2:13, 14; 4:10)" after "John 10:20" is tagged John 2:13) | Inside a group the print is followed | Built |
| Three callers stand inside a word (Genesis 42:20, Exodus 12:25, Exodus 19:17) | The letter is drawn at the head of the word (`wordSplitCallers`, `TestACallerInsideAWordIsDrawnBeforeTheWord`) | Built |
| 257 callers stand where the source runs two words together (docs/SOURCE_FIELDS.md) | The letter is drawn where it stands, between the two words | Built; the text is the source's |
| Three citations name a verse the text does not have, or are cut short | Their words are shown as printed and are not followed | Built; 1983–2018 Thomas Nelson print editions read `"Is. 15:1–16"` at Amos 2:1 (Isaiah 15 has 9 verses; truncated from `"Is. 15:1—16:14"` on Is. 15:1 and Jer. 48:1) while the 2023 Evangelical Study Bible reads `"Is. 16:1–6"` (checked 11–13 November 2026) |

A report to the provider is drafted for the owner to review and send.

## The other cross-reference sets alongside it

- `GOSPEL PARALLELS` groups destinations by the actual synopsis event and
  source passage. Each group names its episode once and shows the source
  span in the displayed edition's numbering; discontinuous spans stay
  separate rather than becoming one invented continuous range. Different
  events retain separate groups even when their titles or destinations match.
  A single destination stands directly under the group heading; several
  destinations form an independently expandable group, initially open. The
  same source-group model handles the other-occasion relationships only in
  `next` builds, with their relation label once per group. All remain
  unfiltered against publisher and Treasury references. `parseGospelRef`
  strictly rejects empty,
  malformed, non-positive or inverted spans;
  `TestEveryGospelParallelSpanNamesRealVerses` pins all 473 shipping spans
  (481 with `-tags next`) and all 102 other-occasion groups to valid verses;
  and `TestGospelParallelsPanelCensusAcrossEditions` holds all 15,103
  single-verse Gospel panels across WEB, WEB Catholic, BSB and NKJV to the
  embedded synopsis row for row.
- `treasuryRowsFor` keeps its `maxCrossRefsPerVerse = 16` vote-ranked cap for
  each contributing verse (`maxCrossRefsKept = 32` in the index across all
  builds). A source row resolved into two disjoint spans consumes one place
  and displays both. There is no selection-wide 40-row cap: later verses'
  groups cannot disappear because earlier verses filled it. The selected
  verses appear in Scripture order, with vote order retained inside each
  group. A single-verse selection stays flat and open. A multi-verse
  selection uses an upright verse key for every contributing group; several
  such groups begin closed, while a sole contributing group begins open.
  The group header retains its complete abbreviated citation summary in
  either state. Cards are created only on the first opening and retained
  through later closes. Other sources never consume a verse's allowance, and
  `TestEveryTreasuryDatasetRowIsAccountedFor` (`BIBLETEXT_XREF_REALDATA=1`)
  accounts for all 344,799 dataset rows with zero silent drops.
  Cross-book destination spans retain both book names, and `newCrossRefRef` turns oversized
  cross-book card headings cleanly after their en dash
  (`"Jeremiah 52:1–"` / `"Lamentations 1:5"` at Extra large on Ezekiel 36:3, and
  `"1 Thessalonians 5:28–"` / `"2 Thessalonians 1:2"` among Romans 1:7's kept
  index rows). The dataset width test retains its 283pt list-width case
  (`TestEveryRealCrossReferenceCardFitsTheNarrowestPhone`); the minimum supported
  canvas is 320pt, with a 280pt popup and a 244pt mounted list. At Extra large,
  a direct note gives its heading 206pt and an opened group gives it 190pt.
  The conservative 160pt single-line cutoff keeps longer headings on the
  width-aware renderer. `TestCrossRefHeadingsFitMinimumPanelWidth` mounts the
  minimum popup in all three text sizes and checks direct and grouped headings;
  a same-book verse range stays together when the reference wraps, including
  its minimum-width calculation (`TestCrossRefMinimumWidthKeepsSameBookRangeWhole`).
- The public-domain and CC0 editions' own cross-references — stripped by
  `bible.helloao.org`'s JSON export — are recovered offline from their
  published USFM (`scripts/gen-publisher-xrefs.py`,
  `publisher_xrefs_public_data.go`, `publisher_xrefs_public.go`) and shown in
  every build on the same publisher-run model (`WEB REFERENCES`,
  `WEBC REFERENCES`, `BSB REFERENCES`): 340 in-verse `\x` notes across 322 WEB
  verses (424 citations), 343 across 325 WEB Catholic verses (430 citations,
  adding Tobit 2:6, Wisdom 14:27 and Sirach 6:30), and 1,325 section-heading
  `\r` parallel-passage lines across 634 BSB chapters (2,032 citations plus
  Hebrews 11:30's words-only `"Joshua–Malachi"`, whose leaked duplicate
  `"(Joshua–Malachi)"` feed heading is stripped on attachment). Because these
  three sets are sparse (`BibleVersion.PublisherApparatus == false`), unnoted
  selections have no empty publisher banner. The Gospel and Treasury sections
  still name their sources when they have references
  (`TestShippedPublicPublisherCrossRefTablesAreCompleteAndSound`,
  `TestEveryPublicPublisherCrossRefNoteFitsTheNarrowestPhone`). Whether to
  replace OpenBible's vote-ranked TSK with CrossWire's public-domain head-word
  TSK remains an open product question (`docs/SOURCE_FIELDS.md`).

WEB and WEB Catholic retain their genuine source insertion points and the
lettered publisher-note presentation. The letters are panel navigation keys,
not characters in the main reading page. BSB's recovered apparatus belongs to
section headings: its groups show the actual heading and `Heading before verse
N`, using the existing exact `BeforeVerse` inclusion rule. Heading-only BSB
results retain the selected excerpt above those groups without artificial
letters.
A one-piece heading note is displayed directly; a multi-piece note opens under
its complete printed summary, initially open.

`crossref_origin_list.go` composes these source groups and
`crossref_source_group.go` supplies their shared compact disclosure row.
Independent fold state is keyed by origin identity, so a Treasury download,
retry or panel rebuild can restore each group's choice without confusing two
groups that cite the same passage. A numeric key stays visible beside its
opened group's stem while that group scrolls past the viewport's top. These
changes do not alter translation footnotes: the optional chapter-bottom
wording-and-manuscript notes remain separate from cross-reference apparatus.

Recovered notes carry `Footnote.PublisherTable`, persisted in the cache as
`publisher_table`. Before applying the current tables, attachment removes only
cross-reference notes carrying that marker. A changed or deleted table entry
therefore replaces the old recovered note while genuine feed notes remain in
order. Unmarked notes are treated as feed-owned: an experimental cache written
before the marker must be refreshed rather than having its provenance guessed
from matching citation text.

BSB heading attachment first requires a unique heading-text hash at the same
verse. If the wording has changed, it falls back only when both the saved
`HeadingCount` and the current candidate count are one. Multiple candidates
remain ambiguous; their old order is not treated as proof of identity. The
count is taken after removing a leaked reference heading. Attachment updates
the stored heading slice directly, so an unrelated run-in heading does not
turn the operation into a write to a filtered copy.

## Checking it again

`crossref_origin_list_test.go` covers per-origin Treasury groups, the flat
single-verse case, repeated destinations across sources, BSB heading labels,
WEB-family letters and source credits during Treasury loading and failure.
`crossref_source_group_test.go` covers independent disclosure, lazy creation,
retained children, complete wrapping, three-digit numeric keys at every text
size and sticky-key bounds.

`TestPublisherTableVerseCacheRefresh` and `TestPublisherTableHeadingCacheRefresh`
exercise table updates and deletions across a cache round trip while preserving
feed notes; the heading case also retains notes beside a run-in heading.
`TestPublisherHeadingFallbackRequiresOriginalSingleton` holds the original and
current sibling counts separately. With `BIBLETEXT_XREF_REALDATA=1`,
`TestPublicPublisherBSBCanonAllAttachments` reapplies the tables to an in-memory
copy of the cached BSB canon and requires all 1,325 notes to reach their uniquely
matched headings; it does not rewrite the cache file.

`TestLiveAPIBibleFullCanon`, with `BIBLETEXT_FULL_CANON_OUT` set, fetches the
canon, asserts the census and writes the decoded canon, which is licensed
text and stays outside the repository. With that file,
`BIBLETEXT_RENDER_XREFS=<that file> BIBLETEXT_RENDER_OUT=<dir> go test -tags
nkjvxrefs -run TestRenderCrossRefPanel .` draws the panel for 26 selections
at two phone sizes in both appearances, with every group opened and every
note closed where a selection has them (586 PNGs).

An independent reading of all 32,517 notes, written from the raw feed
without the app's code, was compared with the panel's model note by note on
11 November 2026. Read as delivered the two differ on 4 notes, all intended:
the three callers inside a word and Revelation 6:2's basis. Both inputs are
licensed data, so the comparison is a record and not a repository test.
