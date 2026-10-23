# The selection's place in its verse

Every reading pane works out which verses a selection covers by position and
hands Go the selection's text with that verse span, `(text, lo, hi)`
(`selection_span.go`). What it does not hand over is where in verse `lo` the
selection begins, so Go finds the words again by text. This document records
the change that carries that place across the boundary: the value each pane
sends, every seam it crosses, how Go uses it, and how it is tested. Line
numbers are at 6bf27aee4. 1.2.22 carries the change with that release's
other fixes to the same pipeline, several of which change what a selection
without a lead gets; where they do, this document says so.

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
  another verse. Since 1.2.22 the fallback first places the fragment in the
  span's verses (`placeInSpan`), which leaves the repeats inside them.
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
  same probe (ai.go:189). Since 1.2.22 the probe is gone: `cleanQuoteTextIn`
  strips a number only as a whole token followed by its verse's body
  (`stripVerseMarkersProse`), so no later number is eaten, and without a lead
  the stray digit still stays.
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
hi)` keeps its meaning, so the legacy Entry pane, which since 1.2.22 hands
over a span and no lead (`chapterText.selectionSpan`, reading.go), the dev
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

## The selection's chapter

The verses and the lead are the chapter the selection was made in, and
Go's reader may no longer be on that chapter when a verb runs: continuous
narration turns the chapter when one ends, and a desktop menu built over
the old chapter stays open across the turn. Read against the chapter then
current, Psalm 135:3's words were cited as Psalms 136:3. So the span also
carries the chapter (`selSpan.book`, `selSpan.chapter`), and every function
that takes a span reads the selection in it (`selectionChapter`), the
reader's own chapter only where the span names none. The change is for
1.2.23, not in 1.2.22 (ROADMAP.md, Later minor releases, row 27). Every pane
hands it over by one rule:

| Seam | Where | What |
|---|---|---|
| The span | selection_span.go | `book` and `chapter`; `inChapter`, which keeps a chapter already named; `selectionChapter`; `paneSelSpan`, which the native exports build the span with; `versesChapter` |
| iOS, iPhone and iPad | reading_ios.go | `bibleTextTVSetChapter` announces the chapter on the main queue just ahead of `bibleTextTVSetHTML`, whose block takes it and keys the text with it (`gTVBook`, `gTVChapter`) on each path that installs a page, each import and the fallback page a failed import leaves (`btIOSInstallPage`); the edit menu reads the key with the selection's text, span and lead |
| Mac | reading_macos.go | the same through `bibleTextMacTVSetChapter` (`gMacTextBook`, `gMacTextChapter`), on each import and the fallback page (`btMacInstallPage`); `hbSendAI` and `hbSendStudy` read the key when the action runs, with the rest of the selection |
| Apple, Go side | ai_menu_darwin.go, reading_ios.go, reading_macos.go | `bibleTextAIMenuTapped` and `bibleTextStudyMenuTapped` take `char *book, int chapter` after the lead; `NULL` and 0 name no chapter; each push calls the chapter's setter before the HTML's |
| Android | android/BtBridge.java | `setChapter`, called just before `setHtml` on the same thread, which keys the text it sets (`textBook`, `textChapter`); `onActionItemClicked` captures the key with the span and the lead, and `showStudyPopup` carries it to its rows; `nativeSelectionAction(…, String book, int chapter)` |
| | reading_jni_android.c, reading_android_export.go, reading_android.go | the thunk converts a non-null book and releases it; `btaSelectionAction` takes `char *book, int chapter`; the push calls `setChapter` before `setHtml` |
| Windows and Linux | reading_styled_select.go, reading.go | the styled pane and the legacy Entry pane name the chapter of the verses they laid out; `selectionStudyMenu` stamps the chapter on screen on a span that has none, as the menu is built |

`selection_chapter_contract_test.go` holds each of these seams, and every
Go function that takes a span to reading it in the span's chapter.

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
   lead may open on one (it is empty, or holds all of verse `lo`). Past a
   verse's end, where the selection does not have the next verse's number,
   it holds what the page draws between the two verses: the publisher's
   headings above the next verse, which the prose does not hold. The walk
   reads through them whole and in order (`throughHeadings`), and the
   number must follow; a selection that ends inside them ends with the verse
   above, whatever words the verse below opens with. Every pane draws a
   verse's number before its words, so the words of a heading that the next
   verse also opens with are the heading's. Everything else must match byte
   for byte. When the walk consumes the whole selection, its place and its
   words replace the text strip's: the selection is placed where the reader
   made it, and no number or heading is removed by text.
3. When the walk fails (words between two verses that are neither the
   headings drawn there nor the next verse's number, a drawn character the
   prose lacks), the text strip runs as it does without a lead, and
   `locate` takes the
   lead's place before the number's anchor and before any search, when the
   prose has the stripped words there inside the span's verses.
4. The repairs that follow (the mid-word end and start repairs, the orphan
   punctuation) are unchanged: they only cut the placed text shorter.

**The fallback.** When the locate declines, a piece of one word, and the lead
resolves and the walk succeeds, `prepareShareQuoteIn` completes the piece at
the lead's place and returns that place, so the terminal and the line-break
restore read it too; it is not handed to `cleanQuoteTextIn` and the
chapter-wide search. The lead moves the quotation only. The piece is cited
as it is without a lead, by the span (`citationForSelectionIn`), which is
what its link (`linkVersesForSelectionIn`) and its cross-references
(`selectionVersesIn`) name for a selection the locate declines, so the verbs
still agree: a drag from inside a verse's last word to the next verse's
number is cited as both verses by each of them. A place that reaches
outside the span's verses is not taken, and the legacy path answers.

Without a lead nothing above runs, and every path is as it was.

## What this closes, and what it does not

- **The later copy:** closed on each pane once it sends the lead,
  including a later copy of the words that end a verse dragged on into the
  heading below it. The Windows and Linux pane sends it in the Go change.
- **A drag into a heading that opens with the next verse's words:** with
  the lead it ends with the verse above, placed and cited there. Without a
  lead the text strip still reads the heading's words as the verse's where
  the region holds them (`verseEndBeforeHeading` answers only where it does
  not).
- **The fragment fallback:** at the root, for every pane that sends a
  lead. Without one, or for a lead that fails its check, the fallback places
  the fragment in the span's verses first (`placeInSpan`, 1.2.22), which
  leaves the repeats inside them, and ends a fragment run on into the
  heading below them with their last verse (`verseEndBeforeHeading`, as the
  located path does), which leaves one run on through a heading to the verse
  below; the page with no span keeps the search of the whole chapter.
- **The number strip by text:** at the root where the walk succeeds; the
  text strip stays for selections with no lead and for a walk that fails.
- **The digit drag and the 12-rune probe, in the share and the AI preview:** a
  drag begun inside the number is placed, so neither the share nor the AI
  preview reaches `cleanQuoteTextIn` for it. The probe itself is gone in
  1.2.22: without a lead the stray digit stays, and no later number is
  eaten.
- **The no-span probe in the AI passage:** not by this change. The
  plain-text page the Apple panes show when a chapter fails to import had no
  verse geometry, so it sent no span and no lead. It now carries its verse
  numbers, title and headings (`reading_apple_fallback.go`) and is installed
  as an imported page is, so it sends both like any other page. 1.2.22 takes
  the probe's answer only when it names one verse, and otherwise cites the
  chapter; that rule stays for any selection that still arrives with no
  span.
- **The chapter a dispatch was made in:** not by this change, which carries
  verse numbers only; a chapter that advanced between the menu and the
  click was cited instead. The span now carries its chapter too, through
  the same externs (The selection's chapter, above).
- **A selection inside a publisher's heading** (1.2.23, decision 23): each
  pane that draws headings in its selectable text hands a selection wholly
  inside one over as the verse below, with the heading flag and no lead
  (`selSpan.heading`), and Go answers it by that verse with nothing
  quoted. The flag travels after `hi` in every native seam: `heading C.int`
  in `bibleTextAIMenuTapped` and `bibleTextStudyMenuTapped`
  (`ai_menu_darwin.go`) and in `btaSelectionAction`
  (`reading_android_export.go`), `boolean heading` in
  `nativeSelectionAction` (`android/BtBridge.java`) and `jboolean jHeading`
  in its JNI thunk (`reading_jni_android.c`). Before it, such a selection
  resolved to the verse above, its lead ran through the heading and failed
  the check, and the selection was placed as it was without a lead.

## Tests

Go side, with the Windows and Linux pane (share_lead_test.go,
share_lead_heading_test.go, reading_styled_lead_test.go, and the sweeps
beside the tests they extend):

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
  "3" is still there, and "14" comes off whole by the fallback's strip. The
  "9" of 29 selected alone cites verse 29 with no quotation.
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
  the later copies of the first section, pinned, with the WEBC's 15,100 of
  204,817. The pieces of a word that share another place's answer without
  it are 12,925 of the WEB's 1,160,304, 11,218 of the BSB's 1,125,726 and
  13,764 of the WEBC's 1,325,427 since 1.2.22's fallback places a piece in
  its span's verses, pinned; completed from the chapter's first copy, as
  before that, they were 118,136, 106,972 and 127,888.
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
- **Drags into a heading** (share_lead_heading_test.go), on a page drawn as
  the Apple and Android panes draw it: superscript numbers with the no-break
  join, each heading on a line of its own above the verse it stands over,
  indented paragraphs. With invented verses
  (`TestADragRunOnIntoAHeadingIsCitedByTheVerseItTouched`,
  `TestALaterCopyRunOnIntoAHeadingIsSharedFromItsOwnPlace`,
  `TestAPieceOfAWordIsCitedAsItsLinkCitesIt`): a drag begun inside the last
  word of a verse into a heading that opens with the next verse's first
  word is placed in the verse above and cited by it, as its link and its AI
  passage cite it; a later copy of a verse's last words dragged into the
  heading below is shared from its own place; a drag through a heading that
  repeats the next verse's opening words, to that verse's number, quotes the
  verse above alone; a piece of a word is cited as its link cites it.
  `TestEveryDragIntoAHeadingIsPlacedInsideItselfWithItsLead`, in CI, invents
  headings over every fourth verse of the embedded Gospels, among them
  headings that repeat the next verse's opening words and headings that
  open with the word that follows an earlier copy of the last word above
  them, and makes the 23,567 drags from the last three words of a verse into
  the heading below, through it, to the next verse's number or first words:
  with the lead none is placed outside the stretch of verse words it covers,
  and each is cited as its link cites it; without it 75 are placed outside,
  pinned. A walk that reads on past a verse's start without its number
  places 828 of them outside and cites 766 otherwise than linked. Over the
  publishers' own headings in the local caches
  (`TestEveryDragIntoAHeadingIsPlacedInsideItselfOverLocalCaches_LocalOnly`)
  the same holds for the WEB's 503, the BSB's 50,680 and the WEBC's 557
  drags (BSB 16 placed outside without the lead, pinned, two of them placed
  in their verse by 1.2.22's fallback where the chapter-wide one gave no
  place, 14 before it; 262 with such a walk). Without the lead the drags the
  fallback places nowhere, quoting the heading's words, are pinned too: 480
  in the Gospels and 0, 46 and 0 in the caches, each a piece of a word run
  on through a heading to the verse below or a drag begun on a verse's
  closing marks; 3,604, 23, 6,697 and 31 before a piece run on into the
  heading below its verse ended with that verse.

Every test of the placement above fails on 6bf27aee4 with only the field
added (share.go and reading_styled_select.go as they were), which shows it
measures the consumer and not the plumbing. The two resolution tests are
the plumbing's own: they hold `leadStart` and `selSpanWithLead`
(selection_span.go) and pass there.

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
