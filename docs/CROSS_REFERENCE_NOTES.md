# The NKJV's cross-reference notes: what a note carries, and where it is shown

A ledger for the cross-references panel's publisher run
(docs/SCRIPTURE_WORKLIST.md, S20). Decided 11 November 2026: nothing the
edition's notes carry is to be lost — not a citation, not the order, not the
grouping, not the punctuation that says what belongs with what, not the word a
note hangs from. This document lists each thing a note carries, how many the
canon holds, where the panel shows it, and the test that holds it, so that a
change to the panel can be checked against the list rather than against a
memory of it.

Everything here is behind the `nkjvxrefs` build tag, which no store build
compiles; whether any of it displays waits on the licensing reply
(docs/BACKLOG.md, "NKJV cross references: the panel's second pass").

The figures are of the canon fetched on 6 November 2026. Those marked
"census" are pinned by `canonPieceCensus` in `publisher_xrefs_census_test.go`
and asserted against a live fetch by `TestLiveAPIBibleFullCanon`; the census
measures a canon through the same functions the panel draws from, so it
counts what is shown, not what was meant. A figure marked "counted" was
counted once on 11 November 2026 and is not held by a test.

## What the feed sends, and what the decoder keeps

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
| It hangs from a verse | 32,473 notes on 21,249 of 31,102 verses (census; the verses counted) | A letter before its word in the passage at the head of the list; the note under NKJV REFERENCES, keyed "· VERSE n" where the selection spans verses or a note's key is not the selected verse | `TestThePassageIsTheSelectedTextWithALetterBeforeEachNotesWord`, `TestCrossRefListKeysTheRunOnlyWhereItInforms` |
| It hangs from a Psalm's title | 43 notes on 38 of 116 titles (census; the titles counted) | The title stands before verse 1 in the passage with its own letters; the notes under NKJV REFERENCES · TITLE, when the selection reaches verse 1 | the same passage test |
| It hangs from a section heading | 1, before Numbers 4:21 (census) | The heading stands, muted, before its verse in the passage, the letter at its head; the note is lettered and listed with that verse's | `TestAHeadingsNoteStandsWithTheVerseItIsSetBefore` |
| The word it hangs from | every note but 2 (census: `Unkeyed`) | The letter is drawn directly before the word (`noteMarkAt`). 1,998 offsets stand before white space and are drawn after it: a line break 1,952 times, a space 46 (census: `Moved`; the split counted) | `TestANotesLetterAndWhereItStands`; the census reports a letter that no word follows |
| The order of a verse's notes | at most 7 on a verse (census); letters a 21,287, b 8,241, c 2,316, d 536, e 113, f 19, g 3 (counted) | Letters a, b, c in that order, restarting at each verse and at a title; the notes listed in that order | `TestANotesLetterAndWhereItStands`; the census checks every index |
| **The printed text** | | | |
| The note as printed | 32,517 notes | A note of several pieces, or one that is one parenthesised group: verbatim on its own row, in both states of the fold. A note that is one citation: its card, whose reference is the citation spelled out | `TestANoteOfSeveralPiecesFoldsToItsPrintedLine`; `TestTheShorthandPrintsWhatItReads` (the shorthand printed back from the card is the note's text for all 12,576 notes of one citation) |
| The separators: "; " between citations, ", " before a further verse | every gap of every note is one of the two, and nothing stands before the first piece or after the last (census: no exception) | On the printed line; the pieces under it are in the same order | `noteGapProblems`, run over the canon by the census |
| A book or chapter carried from the citation before ("John 7:50; 19:39") | — | The printed line shows the continuation as printed; its card is labelled in full ("John 19:39") | `TestTheShorthandReadsEveryShapeTheEditionPrints` |
| **The citations** | | | |
| A citation the reader can follow | 65,257 at a note's level and 8,320 inside groups (census) | A card: the reference, a preview from the loaded edition, the whole card the tap | `TestANoteIsItsPiecesInTheOrderItPrintsThem`, `TestAGroupRowOpensIntoItsCitations` |
| The target the feed tagged | 66,306 tags; 66,294 agree with the print's reading and 12 do not, all inside groups | Outside a group the tag is followed; inside a group the print is | `TestNoteShorthandCensusHoldsTheShorthandToTheTags`; the live canon test pins 4,850 groups, 8,324 group citations and the 12 |
| A citation of the verse or passage the reader is on | 48 at a note's level and 3 inside groups (census) | Its reference in full, with no preview and nothing to tap, in its printed place, set as a line of the edition's print (muted, 14 points) | `TestANoteIsItsPiecesInTheOrderItPrintsThem` |
| A citation that cannot be followed | 3 (census: 2 at a note's level, 1 inside a group): Ezekiel 33:12 "Ezek. 8:21" and Mark 4:29 the "39" of "(Mark 13:30, 39)", verses the text does not have; Amos 2:1 "Is. 15:1—16", a form the shorthand has no rule for | Its words as printed, muted, in their place | the census (`Words`, `GroupWords`); `TestTheShorthandLeavesWhatItCannotReadUnread` |
| A citation of a Psalm's title ("Ps. 59:title") | 47 (the apparatus census, `noteApparatusCensus`; the live canon test asserts that title, LXX and DSS are the apparatus's only words) | A card labelled by its chapter, tagged "Title", previewing the superscription; the whole psalm is a different card | `TestATitleCitationResolvesToTheTitle`, `TestATitleCardPreviewsTheSuperscription`, `TestATitleAndTheWholePsalmAreTwoCards` |
| A citation of a whole chapter | 117 (the apparatus census) | A card labelled as a chapter ("Leviticus 11", "2 Kings 18–20") | `TestPublisherCrossRefsLinkWholeChapters` |
| A basis (", LXX"; ", LXX, DSS") | 3 notes (the apparatus census) | A tag on the citation's card; after a list of verses it is the whole citation's ("Ps. 45:4, 5, LXX", Revelation 6:2) | `TestABasisRidesItsCitation`, `TestABasisAfterAListOfVersesTagsEachOfThem` |
| **Grouping** | | | |
| A parenthesised group | 4,850 (census); 1,985 are the whole of their note | Inside a note: a row in the place the edition printed it, carrying the group as printed with its parentheses, that opens in place into the group's pieces on a ground of its own; closed until it is tapped. As the whole of a note: the note's own row, the group as printed, with its citations under it, open as every note is when the panel opens | `TestANotesGroupsStandWhereTheEditionPrintsThem`, `TestAGroupRowOpensIntoItsCitations`, `TestAGroupOpensInsideThePanelWithoutMovingIt` |
| What stands inside a group | 8,320 cards, 3 of the reader's own passage, 1 that is words (census) | Each in its printed order under the opened row, by the rules above | `TestAGroupRowOpensIntoItsCitations` |
| The book carried into a group and out of it | — | The cards are labelled in full | `TestTheShorthandReadsEveryShapeTheEditionPrints`, `TestTheShorthandFindsGroupsByTheirParentheses` |
| Which rows are one note's | 32,517 notes; 17,956 of several pieces and 1,985 that are one group (census) have rows under their own | Each note stands on a ground of its own with a gap before the next; its letter stands alone in a margin at the ground's left, and everything else of the note beside the margin; a rule hangs from the letter down the margin beside the note's rows (`crossref_section.go`) | `TestEachNoteStandsOnAGroundOfItsOwn` |
| **Repeats** | | | |
| Two notes of a verse citing one passage | the repealed rule "one card, the first" had withheld 245 cards on 224 single-verse selections (counted) | Each note keeps its card | `TestANoteIsKeptWholeThoughAnotherNoteHasItsPassage` |
| **The text the notes hang from** | | | |
| The verse as the page draws it | every verse with a note | The passage: small capitals, the translators' supplied words in italic, the words of Christ in red where the reader has that on, poem lines on rows of their own. With the letters taken out it is the text, rune for rune | the passage test; the census compares every verse |

What a note is (census): 12,576 one citation, drawn as a card; 1,985 one
parenthesised group and 17,956 several pieces, each drawn as a row that
folds.

## What the panel adds

Nothing in words. Three marks, none of which is in `Verse.Text` or leaves the
panel (docs/ADDITIONS_AND_DROPS.md):

- the letters, for the reason above;
- a verse's number in the passage, where the passage holds more than one
  verse or a title before the verse, so that a letter is read against the
  right verse;
- the disclosure icon on a row that opens.

And devices of layout that are no mark in the text: a note's ground, the
margin its letter stands in, and the rule that hangs from the letter beside
the note's rows. Together they show what the edition shows by listing a
letter's references together.

The Treasury's rows under the publisher's run leave out a passage the
publisher's cards already show as the panel opens: a note's direct
citations, and the citations of a note that is one parenthesised group. A
group inside a note starts closed, and its citations are not counted.

A citation of the reader's own passage is shown as its reference in full,
so where the edition prints it without its book the book is supplied on that
row; the note's printed line above it is unchanged.

Letters and numbers are set as the page sets a superscript
(docs/READING_TYPOGRAPHY.md, "Verse numbers").

## What is known to be wrong in the feed, and what the panel does

| The defect | What the panel does | Status |
|---|---|---|
| Two stretches hang every note one caller late: Ruth 3:12 to 2 Samuel 24:25 (1,640 callers) and Psalm 119:133 to Psalm 150:2 (323). The feed lacks a caller on Ruth 3:11 and a second on Psalm 119:132, and the last caller of each stretch has no content | Shows each caller in a stretch with the content delivered on the next (1,961 notes), the first content of each stretch with its verse and no letter (2), nothing for the last caller of each (2). Only while the loaded text carries the marks of the feed as found; the stored notes are unchanged (`publisher_xref_displaced.go`, `TestANoteTheFeedHangsOneCallerLateIsShownOnItsOwn`) | OPEN: corrected (as built), as delivered, or withheld until the feed is right |
| The heading's note. A second publication of the apparatus has it in Numbers 4:22, before the word the heading ends with | Shown where the feed has it, on the heading | OPEN |
| Twelve tags inside groups name the wrong book: where a group opens with a numbered book, a later citation of the group that omits its book is tagged with the book of the citation before the group ("(1 Cor. 1:23; 2:13, 14; 4:10)" after "John 10:20" is tagged John 2:13) | Inside a group the print is followed | Built |
| Three callers stand inside a word (Genesis 42:20, Exodus 12:25, Exodus 19:17) | The letter is drawn at the head of the word (`wordSplitCallers`, `TestACallerInsideAWordIsDrawnBeforeTheWord`) | Built |
| 257 callers stand where the source runs two words together (docs/SOURCE_FIELDS.md) | The letter is drawn where it stands, between the two words | Built; the text is the source's |
| Three citations name a verse the text does not have, or are cut short | Their words are shown as printed and are not followed | Built; scans of the printed edition read the same (checked 11 November 2026) |

A report to the provider is owed, whatever is decided.

## What is open

- The two stretches: corrected, as delivered, or withheld.
- The heading's note: on the heading, or in Numbers 4:22.
- Whether every note starts open (as built) or closed, each letter one
  row. A group inside a note starts closed either way.
- Whether a note's letter is kept in view while a note taller than the list
  scrolls. Not built.
- The Treasury's filter removes a Treasury row that a title card or a
  whole-chapter card "carries" though the card shows something else. It
  belongs to the audit of the other cross-references (the Treasury, the
  Gospel parallels, any other edition's), which follows the NKJV.

## Checking it again

`TestLiveAPIBibleFullCanon`, with `BIBLETEXT_FULL_CANON_OUT` set, fetches the
canon, asserts the census and writes the decoded canon, which is licensed
text and stays outside the repository. With that file,
`BIBLETEXT_RENDER_XREFS=<that file> BIBLETEXT_RENDER_OUT=<dir> go test -tags
nkjvxrefs -run TestRenderCrossRefPanel .` draws the panel for 23 selections
at two phone sizes in both appearances, with every group opened and every
note closed where a selection has them.

An independent reading of all 32,517 notes, written from the raw feed
without the app's code, was compared with the panel's model note by note on
11 November 2026. Read as delivered the two differ on 4 notes, all intended:
the three callers inside a word and Revelation 6:2's basis. Both inputs are
licensed data, so the comparison is a record and not a repository test.
