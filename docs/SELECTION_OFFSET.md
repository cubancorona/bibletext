# The selection's place in its verse

Every reading pane works out which verses a selection covers by position and
hands Go the selection's text with that verse span, `(text, lo, hi)`
(`selection_span.go`). What it does not hand over is where in verse `lo` the
selection begins, so Go finds the words again by text. This document records
the change that carries that place across the boundary: the value each pane
sends, every seam it crosses, how Go uses it, and how it is tested. Line
numbers are at 6bf27aee4.

## What is lost without it

`locateShareSelection` (share.go:956-971) looks for the selection inside the
span's verses and takes the first copy that splits no word. For wording a
verse repeats, a later copy is placed at the earlier one, and the three
stages that read the selection's surroundings read the earlier copy's: the
sentence completion (`completeTrailingSentenceProse`), the original terminal
(`originalSentenceTerminalIn`) and the poem-line restore
(`restoreShareLineBreaksIn`). Genesis 22:11 ends "Abraham, Abraham!"; the
second "Abraham" is shared as “Abraham . . . !” where its own place gives
“Abraham!”. Over every selection of one, two and three words made letter to
letter in every verse:

| Edition | Selections | Placed at an earlier copy | Ship different text | 1 word | 2 words | 3 words |
|---|---|---|---|---|---|---|
| WEB | 2,171,994 | 185,053 | 14,209 | 1.44% | 0.34% | 0.12% |
| BSB | 2,080,560 | 145,335 | 11,785 | 1.29% | 0.27% | 0.08% |

The citation is right (the span fixes the verse); the quotation is not. The
verbs that carry the quotation are Share with citation, Share as image and the
note composer's excerpt.

The same missing value is behind four more shapes:

- **The legacy path for a fragment.** A selection the locate declines, a
  piece of one word, falls back to `cleanQuoteTextIn` and a completion that
  searches the whole chapter (share.go:457-458), so it borrows the
  surroundings of the first copy of its letters anywhere in the chapter: of
  the WEB's 1,160,304 pieces of a word of four or more characters begun or
  ended one letter inside it, 118,136 ship a different quote from their own
  place's (10.2%; BSB 106,972 of 1,125,726, 9.5%), almost all of them from
  another verse.
- **A drag begun inside a verse number.** A drag that begins on the "9" of 29
  hands over "9 He said to"; the digit stays, the selection does not locate,
  and the fallback quotes the stray digit (8,769 of the 8,931 `digit` drags of
  `TestSpannedBoundaryDragSweep`, BACKLOG.md "A drag begun inside a verse
  number shares the digit"). Where a verse whose number ends a later verse's
  number opens with the same twelve characters as that verse (Exodus 20:4 and
  20:14 in the WEB), `cleanQuoteTextIn`'s probe (share.go:415) takes the end
  of the later number for the earlier verse's marker: a drag begun on the "3"
  of 13 shares "14 “You shall not commit" as "1“You shall not commit". 339
  verses have that shape (WEB 193, BSB 146). The AI panel's preview takes the
  same probe (ai.go:189).
- **The number strip by text.** `stripVerseMarkersAnchored` (share.go:1706)
  removes a token "N " when the text after it matches the opening of verse N,
  wherever the token stands, so a figure that is verse text followed by the
  words that open the verse with that number would be removed. No verse of
  the WEB, BSB or WEBC has the shape; it is latent.
- **A drag begun inside a word.** "leavened bread shall", begun two letters
  into Exodus 13:7's "Unleavened", is quoted from the whole "leavened bread
  shall" later in the verse
  (`TestADragBegunOrEndedInsideAWordIsQuotedFromAWholeCopyInItsVerse` pins
  it).

## The value: the lead

Each pane sends one more value, the **lead**: its own text of verse `lo` from
the first character of verse `lo`'s number up to the selection's first
character, exactly as the pane holds it. A selection that begins on the
number's first digit has an empty lead; one that begins on the "9" of 29 has
the lead "2"; one that begins on "loved" in John 3:16 has the lead
"16 For God so " in whatever form that pane draws it (superscript digits, a
no-break join, a paragraph indent, a line break).

Two other shapes were considered and rejected:

1. **A UTF-16 offset from the number.** Each pane has it for one subtraction
   (`range.location` minus the number's location), but it counts the pane's
   drawn string, and the drawn strings differ from the chapter prose Go
   locates in (`chapterProseIn`) and from each other: the Apple import's
   paragraph breaks and indents, Android's left-to-right marks and heading
   gaps, the styled pane's superscript digits, an omitted verse's mark ahead
   of a number. Turning the offset into a place in the prose would need a
   model of every pane's drawn text in Go, which is the re-derivation this
   change exists to remove.
2. **A count of letters and digits before the selection.** It survives the
   differences in spacing, but it needs the same definition of a letter in
   Go, Objective-C and Java, which disagree at the edges (combining marks, the
   small capitals the divine name is drawn in); it cannot say that a
   selection began inside the number; and Go would have nothing to check it
   against.

The lead has neither problem:

- **Each pane already holds both ends.** The iOS verse index gives the
  number's location (`btIOSVerseAtIndex`'s `outLoc`), the Mac's span walk
  meets the number's run, BtBridge has `verseStarts`, the styled pane has the
  number's run and its offset. The lead is one substring of the string the
  selection itself was cut from, so it is in the same form as the selection.
- **Go normalises it with the selection's own code.** `collapseSpaces` and
  `sharedText` are what already make a selection match the chapter prose, so
  what makes the drawn form differ falls away by the same rules for both. No
  new matching is introduced.
- **It is checked, not trusted.** After the number, the lead must be a prefix
  of verse `lo`'s body as the prose has it, byte for byte. A stale chapter, a
  heading inside the verse, or any drawn character the prose lacks fails the
  check, and the selection is placed as it is without a lead. A wrong lead can
  cost the improvement, never the answer the selection gets without one.
- **It says when the selection began inside the number,** which no count can:
  the lead is then a proper prefix of the number, and the rest of the number
  at the front of the selection is the page's apparatus.

The cost is one more string per menu action, at most a verse long.

### The unknown value

A lead is either sent or not. An empty lead is a real value (the selection
begins on the number), so "unknown" is the absence of one: `NULL` for the C
exports, `null` for the JNI method, and `hasLead == false` in `selSpan`. The
zero `selSpan` keeps meaning "position unknown", and `selSpanFromNative(lo,
hi)` keeps its meaning, so the legacy Entry pane (reading.go:1559), the dev
sheets, the verse-of-the-day route (share.go:337) and the 196 `selSpan`
literals in the tests behave exactly as before. A pane sends no lead when it
has no number run for `lo`, or when `lo` was clamped (a selection begun above
verse 1's number reports `lo` 0); `selSpanWithLead` drops a lead whenever the
clamp or the swap in `selSpanFromNative` changes `lo`, since the lead was
measured from another verse.

## Every seam

| Seam | Where | Change |
|---|---|---|
| The span | selection_span.go:16 | `selSpan` gains `lead string` and `hasLead bool`; `selSpanWithLead(lo, hi, lead)` beside `selSpanFromNative` |
| iOS, iPhone and iPad | reading_ios.go:272-273 | `btIOSVerseAtIndex(…, range.location, &loLoc)`; the lead is `[textView.text substringWithRange:NSMakeRange(loLoc, range.location - loLoc)]` when `spanLo > 0` and `loLoc <= range.location`, else `nil` |
| | reading_ios.go:278, :296 | the two blocks pass the lead's `UTF8String` (`NULL` for `nil`) |
| | reading_ios.go:31, :33 | the externs gain `char *lead` after `hi` |
| Mac | reading_macos.go:553-572 | `btMacVerseSpanForRange` also writes the location of `lo`'s number run (`r.location` where it sets `lo`) |
| | reading_macos.go:280-295 | `hbSendAI` / `hbSendStudy` cut the lead from `self.textStorage.string` between that location and `sel.location` |
| | reading_macos.go:94, :96 | the externs gain `char *lead` |
| Apple, Go side | ai_menu_darwin.go:68, :87 | `bibleTextAIMenuTapped` / `bibleTextStudyMenuTapped` take `cLead *C.char`; `nil` is no lead |
| Android | android/BtBridge.java:1590-1594 | the lead is `readerText(text.getText(), verseStarts[i], s0)` for the index entry `verseAtOffset` takes for `s0`, so it goes through the same reader-text pass as the selection (heading gaps back to spaces, direction marks dropped) |
| | android/BtBridge.java:1600, :1148, :1218 | `showStudyPopup` carries the lead to its rows' dispatch |
| | android/BtBridge.java:970, :1613 | `nativeSelectionAction(String, String, int, int, String)` |
| | reading_jni_android.c:17, :28-35 | the thunk takes `jstring jLead`, converts it only when non-null, and releases it |
| | reading_android_export.go:27-30 | `btaSelectionAction` takes `cLead *C.char` |
| Windows and Linux | reading_styled_select.go:425-451 | `selectedVerseSpan` cuts the lead from `lay.Text` between the offset of `lo`'s `runVerseNum` run and `selStart`; a selection begun in the space before that number has the empty lead |
| Consumer | share.go:899-971 (`locateShareSelection`), :447-459 (`prepareShareQuoteIn`) | see below |

Every consumer of the span takes the lead without a change of its own:
`aiPassageFor` (ai.go:195), `selectionVersesIn` (crossrefs.go:1412),
`linkVersesForSelectionIn` (share.go:181) and the note composer
(share_note_ui.go:499) all pass the span to `normalizeShareSelectionIn` or
`prepareShareQuoteIn`.

## How Go uses it

**Resolution** (`selSpan.leadStart` in selection_span.go, against the
chapter prose and its verse spans). The lead is put into the selection's form,
`sharedText(collapseSpaces(lead))`, and read against verse `lo`'s number `n`:

- empty: the selection begins on the number, at the start of verse `lo`'s
  body, and may open with `n`;
- `n` itself: it begins just after the number, at the same place;
- a proper prefix of `n` ("2" of "29"): the selection began inside the
  number, and its first token must be the rest of it ("9");
- `n`, a space, then a prefix of verse `lo`'s body: the selection begins that
  many bytes into the body, past one space when the body has a space there
  (the lead's trailing space is collapsed away). A lead that holds the whole
  verse is a selection begun in the space before the next verse's number, and
  may open with that number;
- anything else: no lead.

**Placement** (`placeByLead` in share_lead.go, called from
`locateShareSelection` when no title has already been taken off the
selection's front):

1. A selection begun inside the number loses the rest of the number from its
   front. If nothing is left, it was the number alone: verse `lo`, no
   quotation, as a number selected on its own is answered.
2. **The walk** (`walkSelectionFrom`) reads the selection against the prose
   from the lead's place. A verse number is taken out only where the prose
   is at the start of that verse and the token is that verse's number (at
   the end of the selection, any first part of it: a drag ended on the
   number's first digit); the number at the lead's own place only when the
   lead may open on one (it is empty, or holds all of verse `lo`). Everything
   else must match byte for byte. When the walk
   consumes the whole selection, its place and its words replace the text
   strip's: the selection is placed where the reader made it and no number is
   removed by text.
3. When the walk fails (a heading inside the selection, a drawn character
   the prose lacks), the text strip runs as it does without a lead, and
   `locate` takes the
   lead's place before the number's anchor and before any search, when the
   prose has the stripped words there inside the span's verses.
4. The repairs that follow (the mid-word end and start repairs, the orphan
   punctuation) are unchanged: they only cut the placed text shorter.

**The fallback.** When the locate declines, a piece of one word, and the lead
resolves and the walk succeeds, `prepareShareQuoteIn` completes the piece at
the lead's place and returns that place, so the terminal and the line-break
restore read it too, and cites the verses that place overlaps; it is not
handed to `cleanQuoteTextIn` and the chapter-wide search.

Without a lead nothing above runs, and every path is as it was.

## What this closes, and what it does not

- **The later copy:** closed on each pane once it sends the lead. The Windows and Linux pane sends it in the Go change.
- **The fragment fallback:** at the root, for every pane that sends a
  lead. Narrowing the fallback's search to the span's verses remains worth
  doing for the no-span page and for a lead that fails its check.
- **The number strip by text:** at the root where the walk succeeds; the
  text strip stays for selections with no lead and for a walk that fails.
- **The digit drag and the 12-rune probe, in the share and the AI preview:** a
  drag begun inside the number is placed, so neither the share nor the AI
  preview reaches `cleanQuoteTextIn` for it. Replacing the probe with the
  boundary-checked strip still matters for the no-span page.
- **The no-span probe in the AI passage:** not by this change. The
  plain-text page the Apple panes show when a chapter fails to import has no
  verse geometry, so it sends no span and no lead; once that page keeps the
  numerals' geometry it sends both like any other page.
- **Not addressed:** the chapter a dispatch was made in (the span carries
  verse numbers only, so a chapter that advances between the menu and the
  click is cited instead); it crosses the same externs and is a separate
  change. A selection inside a publisher's heading still resolves to the
  verse above; its lead runs through the heading, fails the check, and the
  selection is placed as it is without a lead.

## Tests

Go side, with the Windows and Linux pane (share_lead_test.go,
reading_styled_lead_test.go, and the sweeps beside the tests they extend):

- **Resolution** (`TestALeadResolvesToTheSelectionsPlace`,
  `TestALeadIsKeptOnlyForTheVerseItWasMeasuredFrom`): each case above, the
  drawn form (superscript digits, the no-break join, a line break), a lead
  with a heading in it, other words, another verse's number, a verse the
  chapter lacks, and the clamp and the swap dropping the lead.
- **The later copy** (`TestALaterCopyIsSharedFromItsOwnPlaceWithItsLead`,
  `TestTheSecondAbrahamIsSharedFromItsOwnPlace`): invented verses repeating
  "said peace", a poem line and "holy, holy, holy", and Genesis 22:11 in the
  WEB; with the lead each copy is placed at its own offset and shares what
  that place gives; without it the first copy is taken, as before.
- **The pinned trade-off**
  (`TestADragBegunOrEndedInsideAWordIsQuotedFromAWholeCopyInItsVerse`): with
  the lead, "leavened bread shall" begun inside "Unleavened" quotes "bread
  shall", and "that the" ended inside "they" quotes "that"; without it the
  existing pin holds.
- **The digit drag and the probe**
  (`TestADragBegunInsideAVerseNumberSharesNoDigit`): WEB Exodus 20:13-14
  begun on the "3" of 13 shares what the two verses selected whole share, and
  the AI passage previews them without a digit; without the lead the stray
  "3" and the eaten "14" are still there. The "9" of 29 selected alone cites
  verse 29 with no quotation.
- **The number strip** (`TestAFigureInAVerseIsNotTakenForAVerseNumber`): a
  figure followed by the words that open the verse with that number is kept
  with the lead; without it the strip still takes it.
- **The embedded Gospels, in CI**
  (`TestEverySelectionInTheGospelsIsPlacedByItsLead`): all 229,224 selections
  of one, two and three words made letter to letter are placed at their own
  offsets with their own words; the 13,853 later copies share exactly what
  their own place gives; 123,177 pieces of a word are completed at their own
  place. Without the lead, 1,605 of the later copies share something else;
  the count is pinned, so the path without a lead is held unchanged.
- **Drags begun or ended inside a word**
  (`TestEveryDragBegunOrEndedInsideAWordIsCutBackAtItsOwnPlace`): the
  237,232 drags of two and three words over the Gospels begun one letter into
  the first word or ended one letter short of the last are each cut back to
  their whole words at their own place with the lead; without it 4,050 are
  placed at another copy of their letters or not placed, pinned. Over the
  local caches the same holds for the WEB's 2,249,198, the BSB's 2,163,292
  and the WEBC's 2,560,919 (61,234, 38,786 and 65,616 placed otherwise
  without the lead).
- **The local WEB, BSB and WEBC caches**
  (`TestEverySelectionIsPlacedByItsLeadOverLocalCaches_LocalOnly`, opt-in by
  `BIBLETEXT_EXHAUSTIVE=1`): the same sweep with the whole quotation compared
  for the pieces of a word too; 0 wrong with the lead in each, and without it
  the counts in the first section, pinned, with the WEBC's 15,100 of 204,817
  later copies and 127,888 of 1,325,427 pieces of a word.
- **The spanned boundary sweep** (`TestSpannedBoundaryDragSweepWithLeads`):
  all 90,264 drags with the lead each pane would send, 0 wrong in every
  family, and none of the 8,931 drags begun on a later digit of a number
  shares a stray digit (8,769 do without the lead).
- **The styled pane** (`TestTheStyledPanesSelectionsArePlacedByTheirLead`,
  `TestTheStyledPaneSendsTheLeadFromTheVerseNumber`,
  `TestStyledPaneSelectedVerseSpan`): 156,594 selections of one and two words
  made through the pane's own layout of every Gospel chapter are placed at
  their own offsets by the lead the pane builds; the 13,254 later copies among
  them are placed elsewhere without it.

Every test above that carries a lead fails on 6bf27aee4 with only the field
added (share.go and reading_styled_select.go as they were), which shows it
measures the consumer and not the plumbing.

Native panes, in their own change:

- `scripts/check-ios-pane.sh` and `--next`, the host build and vet for the Mac
  pane, `scripts/check-android-pane.sh` and `--next`,
  `scripts/check-android-java.sh`;
- `TestTheSelectionExportsMatchTheirCDeclarations`: the `extern`
  declarations of the selection exports in reading_ios.go, reading_macos.go
  and reading_jni_android.c take the Go exports' parameters in order (the C
  compiler sees one file at a time and cannot), and
  `TestEverySelectionDispatchPassesTheLead` that every call passes five
  arguments; the Java declaration and its thunk are held by the existing
  `TestEveryJavaNativeHasAMatchingThunk`;
- on a simulator, the Mac and an Android emulator: a double-click on the
  second "Abraham" of Genesis 22:11 shares “Abraham!”.

## Parity

The lead is one value on all six surfaces. The Windows and Linux pane sends it
from the Go change; iPhone, iPad, Mac and Android from the native change. A
pane that sends no lead behaves exactly as before, so the two changes can land
apart, but a release that carries one without the other records the
divergence in docs/PLATFORM_MATRIX.md ("Sharing").
