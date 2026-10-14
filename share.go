package bibletext

// Share a verse from the selection menu. Two actions, both handed to the native
// OS share sheet (Messages, Mail, Notes, …):
//   - "Share with citation": plain text — the quoted selection plus a reference,
//     ready to drop into a message.
//   - "Share as image": a rendered card (see share_image.go).
//
// The dispatcher here is also where future selection-menu actions
// (cross-references, word study) are routed.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"fyne.io/fyne/v2"
)

const (
	selActionShareCite  = "share-cite"
	selActionShareImage = "share-image"
	selActionShareLink  = "share-link"
	selActionShareNote  = "share-link-note"
	selActionCrossRef   = "crossref"
)

// dispatchSelectionAction routes a non-AI selection-menu action from the native
// callback (already on the Fyne UI goroutine). span is the selection's
// positionally-resolved verse range (selection_span.go); zero when the source
// pane has no positional model.
func dispatchSelectionAction(state *AppState, action, text string, span selSpan) {
	if state == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	switch action {
	case selActionShareCite:
		shareVerse(state, text, false, span)
	case selActionShareImage:
		shareVerse(state, text, true, span)
	case selActionShareLink:
		shareVerseLink(state, text, span)
	case selActionShareNote:
		promptShareNote(state, text, span)
	case selActionCrossRef:
		showCrossRefs(state, text, span)
	}
}

// shareVerseLink shares a link to the passage on the web reader
// (bibletext.co.uk/<version>/<book-slug>/<chapter>/), so the recipient can read
// it — and keep reading — without installing anything. There is no /read/
// prefix: the version id is the FIRST path segment, and that shape is part of
// the frozen URL contract (share_link.go).
//
// The verse span comes from the SAME positional attribution the citation uses
// (normalizeShareSelection), so the reference in the message and the verses the
// page highlights can never disagree. It degrades in tiers rather than failing:
// an unpinnable selection still shares the chapter, and a book with no slug
// (impossible today, guarded by tests) falls back to the plain-text share.
// shareVerseLinkWithNote is shareVerseLink carrying the sender's note. An empty
// note on the reader's own passage produces exactly the link shareVerseLink
// would have, so the two paths cannot drift apart.
//
// at is the passage the note was written on, recorded by the composer when it
// opened (promptShareNote), and everything here is made against it: the
// citation, the link's translation, chapter and verses, the record, and the
// text share a link cannot be built for. The reader can be moved while the
// composer is open — a link arriving navigates without closing it, since the
// reader may be halfway through a sentence — and the words were selected on
// at, so the chapter the reader is on when Share is pressed plays no part.
// The words are read from state.Bible, which is at's translation: a
// translation switch rebuilds the window, whose drain closes the composer,
// and the one rebuild that brings it back, a light/dark change, switches no
// translation.
//
// It returns the record the note was kept as, and kept=false when nothing was
// stored: no note, or a store that stood the write down (saveMyNote). The
// composer shows a kept note on the passage (showSentNote, notes_mine.go).
// Everything here runs before it returns, the save included: the share sheet
// is handed its message and presented later, on its own, and reports nothing
// back, so a sheet the reader then cancels leaves the note kept.
func shareVerseLinkWithNote(state *AppState, text, note string, span selSpan, at notePassage) (stored StoredNote, kept bool) {
	_, cite, _, _ := prepareShareQuoteIn(state, at.book, at.chapter, text, span)
	version := at.version(state)
	lo, hi := linkVersesForSelectionIn(state, at.book, at.chapter, text, span)
	// One nonce, minted here, used twice: it rides in the link and it is kept on
	// the record. That is what lets this device recognise its own note when the
	// reader taps their own link — without asking the words, which cannot answer
	// (a friend may write the same thing on the same verse).
	nonce := newNoteNonce()
	// SAVE FIRST, THEN BUILD THE LINK, and the order is the whole point.
	//
	// Sharing the same words on the same passage a second time is ONE note:
	// addNote dedups on content and hands back the record already stored, which
	// keeps the FIRST share's nonce. Minting a fresh nonce for the second link
	// and using it produced a link whose identity the store did not hold — so
	// tapping your own second link stored a received duplicate and showed your
	// words as "Note from Friend", which is the exact defect the nonce exists to
	// prevent, reachable again just by re-sharing.
	//
	// So the record decides the identity, not this call: whatever nonce comes
	// back is what goes in the link.
	//
	// And keep it. The reader's own words used to vanish with the share sheet
	// and survive only in whatever messenger they went through, so the app
	// could show every note the reader had RECEIVED and none they had sent.
	// The record is a Kind=mine note in the scrapbook store, listed in the
	// notes browser, and never a chapter's default note: it is drawn on the
	// passage only while focus names it. The composer focuses it once this
	// returns (showSentNote), so the reader sees the note on the passage at
	// once, exactly as opening it from the browser shows it, and the next
	// navigation puts it away again. Nothing about the showing is stored.
	if n := strings.TrimSpace(note); n != "" {
		stored, kept = saveMyNote(appPrefs(), StoredNote{
			VersionID: version.ID,
			Book:      at.book,
			Chapter:   at.chapter,
			VerseLo:   lo,
			VerseHi:   hi,
			Text:      n,
			Nonce:     nonce,
		})
		if kept && len(stored.Nonce) == noteNonceLen {
			nonce = stored.Nonce
		}
	}
	url := ShareLinkURLWithNoteNonce(version.ID, at.book, at.chapter, lo, hi, note, nonce)
	if url == "" {
		shareVerseTextIn(state, at.book, at.chapter, version.Name, text, span)
		return stored, kept
	}
	// The note goes in the MESSAGE too, not only inside the link. It is how
	// people share things anyway, it reaches a recipient who never taps, and it
	// reaches one whose app is too old to read the note out of the fragment.
	msg := cite + " (" + version.Name + ")\n" + url
	if n := strings.TrimSpace(note); n != "" {
		msg = n + "\n\n" + msg
	}
	shareTextOut(msg)
	return stored, kept
}

func shareVerseLink(state *AppState, text string, span selSpan) {
	_, cite, _, _ := prepareShareQuote(state, text, span)
	version := state.currentVersion()
	lo, hi := linkVersesForSelection(state, text, span)
	url := ShareLinkURL(version.ID, state.CurrentBook, state.CurrentChapter, lo, hi)
	if url == "" {
		shareVerse(state, text, false, span) // no link possible — share the words instead
		return
	}
	// Citation first, then the URL on its own line: messengers unfurl a link on
	// its own line, and the reference still reads if the preview doesn't render.
	shareTextOut(cite + " (" + version.Name + ")\n" + url)
}

// linkVersesForSelection is the URL fragment's verse range: the positional
// attribution when the selection normalizes, else the span's verses (clamped
// through selectionVerses, whose span path runs the SAME normalize — so a
// declined normalize falls to the span verbatim there). The second tier is what
// keeps the message coherent when normalize declines a selection the span still
// resolves (a single partial word): the citation beside the URL names the
// verse, so a chapter-level link under it read as a broken fragment. Both zero
// = an honest chapter link, exactly as before.
func linkVersesForSelection(state *AppState, text string, span selSpan) (lo, hi int) {
	book, chapter := readerChapter(state)
	return linkVersesForSelectionIn(state, book, chapter, text, span)
}

// linkVersesForSelectionIn is linkVersesForSelection read against a named
// chapter: the note composer's, which is the chapter the selection was made
// on whether or not the reader is still there (shareVerseLinkWithNote).
func linkVersesForSelectionIn(state *AppState, book string, chapter int, text string, span selSpan) (lo, hi int) {
	if _, l, h, _, ok := normalizeShareSelectionIn(state, book, chapter, text, span); ok {
		return l, h
	}
	if span.valid() {
		if vs := selectionVersesIn(state, book, chapter, text, span); len(vs) > 0 {
			return vs[0].Verse, vs[len(vs)-1].Verse
		}
	}
	return 0, 0
}

// shareVerse formats the selection in Bluebook style (see formatBibleQuote /
// citationForSelection) and hands it to the native share sheet, as text or a
// rendered image. The translation is spelled OUT in the parenthetical, not given as
// an initialism — the Bluebook always names the version in full (e.g. "(King
// James)"), so we use "(World English Bible)" / "(Berean Standard Bible)".
func shareVerse(state *AppState, text string, asImage bool, span selSpan) {
	version := state.currentVersion().Name
	if asImage {
		quote, cite := shareQuoteIn(state, state.CurrentBook, state.CurrentChapter, text, span)
		// Don't share blind: show the rendered card for review (with Regenerate)
		// and only hand it to the OS share sheet once the reader taps Share.
		showShareImagePreview(state, quote, cite, version)
		return
	}
	shareVerseTextIn(state, state.CurrentBook, state.CurrentChapter, version, text, span)
}

// shareVerseTextIn is the text share of a selection read against book/chapter
// and cited in the translation named version: the reader's own chapter for
// Share with citation, the note's passage when a note's link cannot be built.
func shareVerseTextIn(state *AppState, book string, chapter int, version, text string, span selSpan) {
	quote, cite := shareQuoteIn(state, book, chapter, text, span)
	shareTextOut(composeShareText(quote, cite, version))
}

// shareTextOut hands a composed text share to the platform. It is a variable
// so a test can read what a surface shares without a share sheet or a
// clipboard — the test driver's window clipboard is single-use, so nothing
// written to it can be read back. Every text share delivers through it: the
// citation, from a selection or a passage, and the two links.
var shareTextOut = func(s string) { nativeShareText(s) }

// shareAnchor is the rectangle a share is presented beside: a control on the
// canvas, in the canvas's coordinates (top-left origin, points), as
// AbsolutePositionForObject and Size give it. The Mac and iPad panes anchor
// a share to the reading view's selection, and the verse-of-the-day card
// hides that view: anchored to a hidden view, the Mac's picker never
// appeared and the share was lost without a word, and the iPad's popover
// fell back to the middle of the screen, over the card. The card hands its
// Share icon's rectangle over instead, and those panes present beside it. A
// selection share hands none and keeps its selection anchor, and the
// Android, Windows and Linux shares read none: their sheets have no anchor.
type shareAnchor struct {
	x, y, w, h float32
}

// currentShareAnchor is the anchor of the share being handed over, set for
// the length of one share by shareAnchoredAt and read inside it by the panes
// that present beside one (nativeShareText in reading_macos.go and
// reading_ios.go). It rides beside shareTextOut rather than through it, so
// that the one seam every text share and every test passes keeps its shape.
// The share is handed over, and presented or queued for presenting, before
// shareAnchoredAt returns, all on the UI goroutine.
var currentShareAnchor struct {
	at shareAnchor
	ok bool
}

// shareAnchoredAt runs share with at as its anchor, and leaves no anchor
// behind for the next share. An empty rectangle is no anchor: a control that
// has not been laid out gets the selection path, not a point at the origin.
func shareAnchoredAt(at shareAnchor, share func()) {
	currentShareAnchor.at, currentShareAnchor.ok = at, at.w > 0 && at.h > 0
	defer func() { currentShareAnchor.at, currentShareAnchor.ok = shareAnchor{}, false }()
	share()
}

// shareAnchorNow is the anchor of the share being handed over, if it has one.
func shareAnchorNow() (shareAnchor, bool) {
	return currentShareAnchor.at, currentShareAnchor.ok
}

// anchorOf is the rectangle o occupies on its canvas, as a share anchor.
func anchorOf(o fyne.CanvasObject) shareAnchor {
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	sz := o.Size()
	return shareAnchor{x: pos.X, y: pos.Y, w: sz.Width, h: sz.Height}
}

// shareQuoteIn is the whole text pipeline for one selection read against ONE
// chapter, book/chapter: the located-and-normalized quote, the original
// sentence terminal, the authored line breaks, then the Bluebook framing. It
// is the single body behind both share routes — the selection menu (the
// reader's chapter) and the verse-of-the-day card (whatever chapter the verse
// is in) — so they cannot drift: what the card shares for a passage is, by
// construction, what selecting that passage in its own chapter would share.
//
// BOTH shares retain authored structure: source poetry lines and the reading
// view's paragraph boundaries. They are rebuilt from the chapter data, never
// copied from rendered wrapping, so resizing the reader cannot change what is
// shared. prepareShareQuote collapses every "\n" out of the selection, so
// without this the card could not break a psalm even though its renderer
// knows how (poemSegments) — a card is the one surface where a flattened
// psalm is most conspicuous.
func shareQuoteIn(state *AppState, book string, chapter int, raw string, span selSpan) (quote, cite string) {
	cleaned, cite, at, baseLen := prepareShareQuoteIn(state, book, chapter, raw, span)
	terminal := originalSentenceTerminalIn(state, book, chapter, cleaned, at, baseLen)
	cleaned = restoreShareLineBreaksIn(state, book, chapter, cleaned, at, baseLen)
	return formatBibleQuote(cleaned, terminal), cite
}

// shareQuoteForPassage returns exactly what the selection route would share
// for the whole of verses lo..hi of book/chapter selected in their own
// chapter: the Bluebook-framed quote and its citation. It is the share route
// for a passage the reader is NOT on — the verse-of-the-day card — where the
// selection route's every stage reads the reader's current chapter and would
// cite Psalm 23:1 as "Matthew 5". The passage goes through the same pipeline
// as a selection of its verses (shareQuoteIn, narrowed to lo..hi by the span
// exactly as a native pane's selection is), so the trailing clause-mark strip,
// the sentence completion, the original terminal and the poem/paragraph
// restore all apply — a bare formatBibleQuote of the verse text skips the
// first of those and ships "…glory of God; . . . ." for a verse ending in ";".
// The reader is never moved: nothing here reads or writes state.CurrentBook or
// state.CurrentChapter. ok is false when there is nothing to share — no
// Bible, an empty or inverted range, verses the chapter does not have.
func shareQuoteForPassage(state *AppState, book string, chapter, lo, hi int) (quote, cite string, ok bool) {
	if state == nil || state.Bible == nil || lo < 1 || hi < lo {
		return "", "", false
	}
	// The "selection": the verses' own text, joined as chapterProse joins them,
	// so the locate lands on the passage's copy of the words and every verse in
	// lo..hi contributes to the citation. No verse-number markers are prepended
	// — stripVerseMarkers would only take them off again.
	var b strings.Builder
	for _, v := range state.Bible.GetChapter(book, chapter) {
		if v.Verse < lo || v.Verse > hi {
			continue
		}
		// The same shared form chapterProse is built in, for the same
		// reason: this synthetic selection is matched against that corpus, and
		// a passage holding a divine name would otherwise be assembled in the
		// publisher's mixed case and never found.
		t := collapseSpaces(verseSharedText(v))
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t)
	}
	raw := b.String()
	if raw == "" {
		return "", "", false
	}
	quote, cite = shareQuoteIn(state, book, chapter, raw, selSpanFromNative(lo, hi))
	if quote == "" {
		return "", "", false
	}
	return quote, cite, true
}

// sharePassageText hands the passage to the native share sheet as text: the
// same message the selection route's "Share with citation" builds
// (composeShareText — the quote, a blank line, the citation line naming the
// translation in full). On Linux nativeShareText is the clipboard fallback
// and the confirmation sheet (share_fallback.go, share_sheet_desktop.go), and
// on Windows the Windows Share sheet (share_windows.go), as for a selection.
func sharePassageText(state *AppState, book string, chapter, lo, hi int) {
	msg, ok := sharePassageMessage(state, book, chapter, lo, hi)
	if !ok {
		return
	}
	shareTextOut(msg)
}

// sharePassageMessage is the text sharePassageText hands over, kept apart
// from the hand-off so it can be pinned without presenting a share sheet.
func sharePassageMessage(state *AppState, book string, chapter, lo, hi int) (string, bool) {
	quote, cite, ok := shareQuoteForPassage(state, book, chapter, lo, hi)
	if !ok {
		return "", false
	}
	return composeShareText(quote, cite, state.currentVersion().Name), true
}

// citationLine is the Bluebook reference line shown under both the plain-text share
// and the image card: an em dash, the reference, and the translation spelled out in
// parentheses — e.g. "— John 3:16 (World English Bible)".
func citationLine(cite, version string) string {
	return "— " + cite + " (" + version + ")"
}

// composeShareText builds the plain-text share: the already-formatted quote, a
// BLANK line, then the citation line. The blank line is load-bearing since text
// shares began preserving poetry: a poetic quote already contains single line
// breaks, so a citation on a bare next line reads as one more poem line — the
// empty line is what marks it as attribution (and matches Rule 5.1's setting
// the citation off from the quoted matter).
func composeShareText(quote, cite, version string) string {
	return quote + "\n\n" + citationLine(cite, version)
}

// cleanQuoteText turns a raw reading-view selection into clean, quotable verse
// text. The superscript verse numbers rendered before each verse ride along in the
// selection as a leading integer token ("16 For God so loved…"); they are stripped
// here by matching each chapter verse's own opening text, so legitimate numbers
// inside a verse are never touched. Whitespace — including the poetic line breaks
// in the source — is collapsed to single spaces. The user's actual selection
// (whole verses or a phrase) is otherwise preserved.
func cleanQuoteText(state *AppState, raw string) string {
	book, chapter := readerChapter(state)
	return cleanQuoteTextIn(state, book, chapter, raw)
}

// cleanQuoteTextIn is the legacy fallback's text pass, for a selection the
// positional locate declined. It is still a way out of the app, so sharedText
// runs here too — the fallback takes the page's typography off exactly as the
// located path does, and it used to ship an omitted verse's gap mark through
// untouched. The verse numbers go by the located path's own strip, which
// removes "N " only where it stands as a token of its own and is followed by
// verse N's body. The pass used to remove "N " plus the verse's first twelve
// runes wherever that string stood, with no token boundary, so a verse whose
// number ends a later verse's number and whose opening is the later verse's
// too (Genesis 1:6 and 1:26 in the WEB) stripped the later number from
// inside: "26 God said" became "2God said" in the share and in the AI
// panel's preview, on 193 WEB and 146 BSB verses.
func cleanQuoteTextIn(state *AppState, book string, chapter int, raw string) string {
	s := sharedText(collapseSpaces(raw))
	if state == nil || state.Bible == nil {
		return s
	}
	corpus, spans := chapterProseIn(state, book, chapter)
	return stripVerseMarkersProse(corpus, spans, s)
}

// prepareShareQuote is the share pipeline's front half: it turns a raw
// reading-view selection into (quotable text, citation) with the two agreeing —
// the citation names EXACTLY the verses that contribute at least one word to
// the text (a citation is a provenance record: any quoted word from a verse
// puts that verse in the range, and no named verse may be absent from the
// text). The ONLY content edit beyond stripping the verse-number markers is
// the mid-word repair: a drag that stops (or starts) partway through a word
// is trimmed back to the last (or forward to the next) whole word — Bluebook
// has no notation for a fragment of a word (Rule 5.3's ellipsis omits WORDS;
// Rule 5.2's empty brackets adapt a real root word), so a chopped word can
// neither be quoted as-is nor marked. A dangling verse-number token whose
// verse lost its only (partial) word is apparatus with nothing to introduce
// and is dropped with it. Whenever the selection cannot be located in the
// chapter's prose, it falls back to the selection as it stands, its verse
// numbers stripped only as whole tokens (cleanQuoteTextIn), and completed
// from its copy in the span's verses where it can be placed there
// (placeInSpan), or, run on into the heading below them, at their end with
// the heading's words taken off (verseEndBeforeHeading), else from its first
// copy in the chapter.
// The last two returns carry the LOCATION so the pipeline's later stages never
// re-find the text: at = the located start of the quote in chapterProse's
// coordinates (-1 when the fallback could not place it), baseLen = the byte
// length of the located portion (completeTrailingSentence only ever APPENDS, so
// text[:baseLen] is exactly the located string). Re-finding was the same
// first-match defect the span fixed for the citation, one stage later: for
// repeated wording the completion and the line-break restore ran against the
// EARLIER copy's surroundings (sweep findings share.go:332/:565/:846).
func prepareShareQuote(state *AppState, raw string, span selSpan) (text, cite string, at, baseLen int) {
	book, chapter := readerChapter(state)
	return prepareShareQuoteIn(state, book, chapter, raw, span)
}

func prepareShareQuoteIn(state *AppState, book string, chapter int, raw string, span selSpan) (text, cite string, at, baseLen int) {
	if state != nil && state.Bible != nil {
		// One build of the chapter's prose serves the marker strip, the locate
		// and the completion: all three read the same chapter, unchanged
		// between them, so building it for each would only repeat the result.
		corpus, spans := chapterProseIn(state, book, chapter)
		if t, lo, hi, idx, ok := normalizeShareSelectionProse(state, book, chapter, corpus, spans, raw, span); ok {
			return completeTrailingSentenceProse(corpus, t, idx), verseRangeCitation(book, chapter, lo, hi), idx, len(t)
		}
		// The normalize declined: a fragment inside one word, or pieces
		// either side of a mark, which have no whole word to quote. The
		// selection is shared as it stands, and the span still says which
		// verses the reader dragged over.
		//
		// With the pane's lead it is still where the reader made it, and is
		// completed, terminated and restored there. The lead moves the
		// quotation only: it is cited as the fallback cites it, by the
		// span, which is what the selection's link and its cross-references
		// name for a selection the locate declines (selectionVersesIn). A
		// place that reaches outside the span's verses is no place for it,
		// and the fallback below answers, as it does for a lead that does
		// not resolve.
		if span.hasLead {
			if p := placeByLead(corpus, spans, pageHeadingsIn(state, book, chapter), span, sharedText(collapseSpaces(raw))); p.text != "" {
				if lo, hi := leadVerses(spans, p.at, len(p.text)); lo >= span.lo && hi <= span.hi {
					return completeTrailingSentenceProse(corpus, p.text, p.at), citationForSelectionIn(state, book, chapter, raw, span), p.at, len(p.text)
				}
			}
		}
		// Without a lead the fragment is placed in the span's verses, as
		// locate places a selection, so the completion, the retained
		// terminal and the line breaks read a copy the reader dragged over.
		// Found anywhere in the chapter instead, a drag inside "would" in
		// Genesis 2:19 (WEB) was completed from verse 7's "soul." and shared
		// as "[O]ul.". Of the WEB's one-word drags begun or ended inside a
		// word, 1,151,120 end up here, and 117,933 of them took another
		// copy's ending, 115,019 from another verse. A repeat inside the
		// span's own verses is told apart only by the lead.
		cleaned := cleanQuoteTextIn(state, book, chapter, raw)
		if at := placeInSpan(corpus, spans, cleaned, span); at >= 0 {
			return completeTrailingSentenceProse(corpus, cleaned, at), citationForSelectionIn(state, book, chapter, raw, span), at, len(cleaned)
		}
		// A piece of a word run on past the end of the span's last verse
		// into the heading drawn below it is not in the span's verses, and
		// the chapter-wide search below quoted the heading's words as the
		// verse's: "aters.\nThe First Day", begun inside the last word of
		// Genesis 1:2 in the BSB, was shared as "[A]ters. The First Day
		// . . . .". So the front of the heading comes off its end, as it
		// does on the located path (verseEndBeforeHeading), and what is
		// left, when it is how the span's verses end, is completed there.
		// Without a lead 6,651 of the BSB's 50,680 drags into its headings,
		// 23 of the WEB's 503 and 31 of the WEBC's 557 were shared so, and
		// each is now shared as it is with the lead. Where the span's verses
		// hold the selection with the heading's word ("im.\nThe" from
		// Matthew 27:31 in the BSB, whose verse has "Him. Then"), a drag
		// inside the verse makes the same text, so placeInSpan answers
		// above; only the lead tells the two apart.
		if start, end, ok := spanRegion(spans, span); ok {
			if rest, at, ok := verseEndBeforeHeading(state, book, chapter, corpus, spans, start, end, cleaned); ok {
				return completeTrailingSentenceProse(corpus, rest, at), citationForSelectionIn(state, book, chapter, raw, span), at, len(rest)
			}
		}
	}
	cleaned := completeTrailingSentenceIn(state, book, chapter, cleanQuoteTextIn(state, book, chapter, raw), -1)
	return cleaned, citationForSelectionIn(state, book, chapter, raw, span), -1, 0
}

// placeInSpan is where s stands in the corpus inside the span's verses, by
// the order locate searches a region in: the first copy there that splits no
// word, else the first copy of any kind. It is -1 when the span is not valid,
// names no verse the corpus holds, or its verses do not hold s.
func placeInSpan(corpus string, spans []verseSpan, s string, span selSpan) int {
	if s == "" {
		return -1
	}
	rStart, rEnd, haveRegion := spanRegion(spans, span)
	if !haveRegion {
		return -1
	}
	if i := wordAlignedIndex(corpus[rStart:rEnd], s); i >= 0 {
		return rStart + i
	}
	if i := strings.Index(corpus[rStart:rEnd], s); i >= 0 {
		return rStart + i
	}
	return -1
}

// spanRegion is the stretch [start, end) of the corpus that the span's verses
// cover; ok is false when the span is not valid or names no verse the corpus
// holds.
func spanRegion(spans []verseSpan, span selSpan) (start, end int, ok bool) {
	if !span.valid() {
		return 0, 0, false
	}
	for _, sp := range spans {
		if sp.verse >= span.lo && sp.verse <= span.hi {
			if !ok {
				start, ok = sp.start, true
			}
			end = sp.end
		}
	}
	return start, end, ok
}

// Every stage of the pipeline from here down reads ONE chapter, and the
// ...In spelling names it outright as (book, chapter). The bare spelling is
// the selection route's: it reads the reader's current chapter (readerChapter)
// and is what every existing caller and test holds to. A passage the reader
// is not on — the verse-of-the-day card — runs the ...In spellings with its
// own chapter (shareQuoteForPassage), so it is located, normalized and cited
// in the chapter it belongs to, and the reader stays where they are.
func readerChapter(state *AppState) (book string, chapter int) {
	if state == nil {
		return "", 0
	}
	return state.CurrentBook, state.CurrentChapter
}

// chapterProse joins a chapter's verses (marker-free, spaces collapsed) into
// one string, recording each verse's [start,end) span — the positional ground
// truth for locating a selection and attributing verses.
type verseSpan struct{ verse, start, end int }

func chapterProse(state *AppState) (string, []verseSpan) {
	book, chapter := readerChapter(state)
	return chapterProseIn(state, book, chapter)
}

func chapterProseIn(state *AppState, book string, chapter int) (string, []verseSpan) {
	var b strings.Builder
	var spans []verseSpan
	for _, v := range state.Bible.GetChapter(book, chapter) {
		// The corpus is what a normalized selection is matched against, so it
		// has to be in the same shared form the selection is put into —
		// see verseSharedText.
		t := collapseSpaces(verseSharedText(v))
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		start := b.Len()
		b.WriteString(t)
		spans = append(spans, verseSpan{verse: v.Verse, start: start, end: b.Len()})
	}
	return b.String(), spans
}

// shareTextBreak identifies a space in flattened chapter prose that represents
// authored structure. replacement is either one newline (a source poetry line)
// or two (a paragraph boundary).
type shareTextBreak struct {
	offset      int
	replacement string
}

// verseShareStructure flattens a verse exactly as collapseSpaces does while also
// recording source-authored newline positions. Leading/trailing whitespace from
// API payloads is ignored; a blank source line remains a paragraph break.
func verseShareStructure(s string) (string, []shareTextBreak) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")

	var b strings.Builder
	var breaks []shareTextBreak
	previousContentLine := -1
	for lineNo, line := range lines {
		line = collapseSpaces(line)
		if line == "" {
			continue
		}
		if b.Len() > 0 {
			replacement := "\n"
			if lineNo-previousContentLine > 1 {
				replacement = "\n\n"
			}
			breaks = append(breaks, shareTextBreak{offset: b.Len(), replacement: replacement})
			b.WriteByte(' ')
		}
		b.WriteString(line)
		previousContentLine = lineNo
	}
	return b.String(), breaks
}

// chapterShareStructure is chapterProse plus its non-visual structure. Paragraph
// breaks come from the same grouping the reader displays; line breaks inside a
// verse come from the translation source retained in Verse.Text.
func chapterShareStructure(state *AppState) (string, []shareTextBreak) {
	book, chapter := readerChapter(state)
	return chapterShareStructureIn(state, book, chapter)
}

func chapterShareStructureIn(state *AppState, book string, chapter int) (string, []shareTextBreak) {
	if state == nil || state.Bible == nil {
		return "", nil
	}
	verses := state.Bible.GetChapter(book, chapter)
	var b strings.Builder
	var breaks []shareTextBreak
	prevPoetic := false
	for paragraphIndex, paragraph := range groupVersesIntoParagraphs(verses) {
		wroteInParagraph := false
		for _, verse := range paragraph {
			// THE SAME SHARED FORM chapterProse IS BUILT IN, and for the same
			// reason: these two corpora are matched against each OTHER.
			// restoreShareLineBreaks takes the quote -- which came out of
			// chapterProse -- and locates it in this one to know where the
			// authored breaks belong. A verse that reads differently here than
			// there does not fail loudly; the locate simply misses and the text
			// is returned unbroken, so a psalm ships as one running line.
			//
			// That is exactly what happened when chapterProse moved to a new
			// form and this did not: the divergence was invisible on
			// every verse WITHOUT a SmallCaps span, which is every verse the
			// shipped WEB gospels contain.
			source := verseSharedText(verse)
			text, verseBreaks := verseShareStructure(source)
			if text == "" {
				continue
			}
			// A join touching a poetic verse restores as "\n" (a paragraph
			// break still wins with "\n\n") — the same verseIsPoetic rule the
			// reading pane renders with, so displayed lines and shared lines
			// stay identical.
			curPoetic := verseIsPoetic(source)
			if b.Len() > 0 {
				replacement := ""
				if paragraphIndex > 0 && !wroteInParagraph {
					replacement = "\n\n"
				} else if prevPoetic || curPoetic {
					replacement = "\n"
				}
				if replacement != "" {
					breaks = append(breaks, shareTextBreak{offset: b.Len(), replacement: replacement})
				}
				b.WriteByte(' ')
			}
			start := b.Len()
			b.WriteString(text)
			for _, br := range verseBreaks {
				breaks = append(breaks, shareTextBreak{
					offset:      start + br.offset,
					replacement: br.replacement,
				})
			}
			wroteInParagraph = true
			prevPoetic = curPoetic
		}
	}
	return b.String(), breaks
}

// restoreShareLineBreaks replaces only the structural spaces that fall wholly
// inside the selected text. The selection is located POSITIONALLY when the
// caller carries the offset (at/baseLen from prepareShareQuote —
// chapterShareStructure's flat string shares chapterProse's coordinates, an
// invariant TestChapterProseAndShareStructureAgree pins), so soft wrapping from
// any platform is neither inspected nor retained and repeated wording restores
// the SELECTED copy's structure, not the first copy's. The text search survives
// only for the legacy unlocated path — where it also quietly failed whenever
// the completion had appended past a skipped quote-closer (the completed text
// is no longer a contiguous substring), losing the quote's poem breaks; the
// positional path restores those too, because only text[:baseLen] must match.
func restoreShareLineBreaks(state *AppState, text string, at, baseLen int) string {
	book, chapter := readerChapter(state)
	return restoreShareLineBreaksIn(state, book, chapter, text, at, baseLen)
}

func restoreShareLineBreaksIn(state *AppState, book string, chapter int, text string, at, baseLen int) string {
	flat, breaks := chapterShareStructureIn(state, book, chapter)
	if text == "" || flat == "" || len(breaks) == 0 {
		return text
	}
	start, end := -1, -1
	if at >= 0 && baseLen > 0 && baseLen <= len(text) && at+baseLen <= len(flat) &&
		flat[at:at+baseLen] == text[:baseLen] {
		start, end = at, at+baseLen
	} else if i := strings.Index(flat, text); i >= 0 {
		start, end = i, i+len(text)
	}
	if start < 0 {
		return text
	}

	var b strings.Builder
	last := 0
	for _, br := range breaks {
		if br.offset <= start || br.offset >= end {
			continue
		}
		rel := br.offset - start
		if rel < last || rel >= len(text) || text[rel] != ' ' {
			continue
		}
		b.WriteString(text[last:rel])
		b.WriteString(br.replacement)
		last = rel + 1 // replace the flattened separator space
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}

// isWordRune reports a rune that can sit INSIDE a word — the test for whether a
// selection boundary landed mid-word.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordAlignedIndex is the offset of the first copy of sub in s that splits no
// word of s, or -1: the copy neither begins nor ends between two runes of one
// word. "he" is found in "and he heard" and not in "there", and neither "s"
// nor "’s" nor "man" is found in "man’s".
func wordAlignedIndex(s, sub string) int {
	if sub == "" {
		return -1
	}
	for from := 0; from+len(sub) <= len(s); {
		i := strings.Index(s[from:], sub)
		if i < 0 {
			return -1
		}
		i += from
		if !splitsWord(s, i) && !splitsWord(s, i+len(sub)) {
			return i
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		from = i + size
	}
	return -1
}

// splitsWord reports whether offset i of s falls between two runes of one
// word.
func splitsWord(s string, i int) bool {
	if i <= 0 || i >= len(s) {
		return false
	}
	_, size := utf8.DecodeLastRuneInString(s[:i])
	return inWordAt(s, i-size) && inWordAt(s, i)
}

// inWordAt reports whether the rune at offset i of s belongs to a word: a
// letter or a digit, or an apostrophe with one on each side, as in "man’s"
// and "Isn’t". A double-click takes such a word whole.
func inWordAt(s string, i int) bool {
	r, size := utf8.DecodeRuneInString(s[i:])
	if isWordRune(r) {
		return true
	}
	if (r != '’' && r != '\'') || i == 0 || i+size >= len(s) {
		return false
	}
	before, _ := utf8.DecodeLastRuneInString(s[:i])
	after, _ := utf8.DecodeRuneInString(s[i+size:])
	return isWordRune(before) && isWordRune(after)
}

// normalizeShareSelection cleans a raw selection for sharing and attributes it
// to its verse range positionally. ok=false when the selection can't be pinned
// in the chapter (caller falls back to the legacy path).
//
// span narrows WHERE the locate looks: strings.Index alone finds the FIRST
// occurrence, so a selection of repeated wording (Psalm 136's refrain) located
// at the earlier verse's copy and was cited there. With a valid span the search
// runs inside the span verses' region of the corpus first — the attribution
// below still derives from the located position, so a trim that drops a verse
// still drops it from the citation. The whole-corpus search remains as the
// safety net for a span that somehow disagrees with the words.
//
// The region is wider than the drag: it runs from the start of the span's
// first verse, and a drag can begin at that verse's last word or its full
// stop. So the region is the last resort, not the first. A selection that
// carries a verse number is placed where the number says, which is exact
// (stripVerseMarkersAnchored), and once the selection is placed it is never
// searched for again: each repair below only cuts it shorter, and the shorter
// text's place follows from the cut. Searching again found the first copy of
// the shorter words in the region, which for a drag begun on the full stop
// ending verse N−1 was often a copy in N−1 itself — "When did we see you"
// cited as Matthew 25:38 when the reader dragged into 25:39.
//
// Since nothing searches again, the one search has to find the copy a drag
// from word to word could have made: in the region, or in the chapter when
// there is no span, it takes the first copy that splits no word
// (wordAlignedIndex) before the first copy of any kind. Placed at the first
// copy of any kind, "the wicked" stood inside "the wickedness", the end
// repair cut it to "the", and the reader's last word was lost; with no span,
// the verse of that copy was cited too. A copy that splits a word is still
// taken where there is no other, as a drag begun or ended inside a word
// needs; but the text alone cannot tell such a drag from a phrase whose first
// copy stands inside a longer word, so a drag begun or ended inside a word
// whose letters also stand as whole words later in the region is quoted from
// that later copy ("leavened bread shall", begun two letters into Exodus
// 13:7's "Unleavened", quotes "leavened bread shall" from later in the
// verse). With a span the citation is the same verse. A pane that also sends
// the selection's lead (selSpan.lead) tells the two apart: the selection is
// placed where it was made (placeByLead) and quotes "bread shall", and this
// search places only a selection that comes without one, or whose lead does
// not resolve.
func normalizeShareSelection(state *AppState, raw string, span selSpan) (text string, lo, hi, at int, ok bool) {
	book, chapter := readerChapter(state)
	return normalizeShareSelectionIn(state, book, chapter, raw, span)
}

func normalizeShareSelectionIn(state *AppState, book string, chapter int, raw string, span selSpan) (text string, lo, hi, at int, ok bool) {
	if state == nil || state.Bible == nil {
		return "", 0, 0, -1, false
	}
	corpus, spans := chapterProseIn(state, book, chapter)
	return normalizeShareSelectionProse(state, book, chapter, corpus, spans, raw, span)
}

// normalizeShareSelectionProse is normalizeShareSelectionIn for a caller that
// has already built the chapter: corpus and spans must be
// chapterProseIn(state, book, chapter). state must be non-nil with a Bible.
func normalizeShareSelectionProse(state *AppState, book string, chapter int, corpus string, spans []verseSpan, raw string, span selSpan) (text string, lo, hi, at int, ok bool) {
	flat := collapseSpaces(raw)
	// The app's own typography comes off here, and BEFORE stripVerseMarkers.
	// That strip confirms a verse-number token by comparing the text after it
	// with the verse's body, and the two must be in the same form or the
	// comparison fails at the first character that differs. The selection
	// arrives as the page drew it — "1 The Lᴏʀᴅ is…" — and the bodies are
	// built from the publisher's letters; stripping first left the token in
	// place on every divine-name verse, the locate missed, and a whole-verse
	// share of exactly those verses fell back to the legacy probe path. So
	// the selection goes to the shared form first, and stripVerseMarkers
	// compares against bodies in that same form (verseSharedText).
	//
	// An earlier version of this comment gave the opposite order a reason —
	// that the strip matched SUPERSCRIPT tokens which the conversion would have
	// turned into digits. It does not: it matches ordinary digits, and every
	// pane hands over ordinary digits. The reason was false and the order it
	// justified was the defect.
	//
	// collapseSpaces has already dealt with the no-break join and the
	// paragraph indent, because strings.Fields treats them as whitespace. What
	// sharedText adds is the omitted verse's "[36]" mark, which would ride out
	// into the reader's message as a character no publisher sent and which
	// chapterProse does not contain, so a selection carrying it could never be
	// located. The divine name's small capitals it deliberately KEEPS — a share
	// sends what the page shows (outbound_text.go) — and chapterProse is built
	// in that same drawn form, so they locate.
	out := sharedText(flat)

	// A psalm's title is the publisher's, not a verse. The Apple and Android
	// panes draw it in the same text as the verses, above verse 1, but their
	// app verbs have been clamped to start at verse 1's number since the
	// titles were first drawn (btIOSContentStart, hbScriptureSelection,
	// BtBridge's contentStart): a selection inside the title gets Copy and
	// Look Up alone, and one from the title into the psalm is cut at verse
	// 1's number before it gets here. What does hand a title over is a page
	// with no verse geometry: the plain-text fallback the Mac and the iPhone
	// and iPad show when the chapter fails to import (gMacContentStart is 0
	// there, as btIOSContentStart is with its verse index emptied, and
	// nothing has a span). The dead legacy pane does not: rewrap lays out
	// the verses alone, with no title, and every verb gets the selection's
	// span (chapterText.selectionSpan). The fallback sets each
	// paragraph on a line of its own (bibleTextMacPlainFromHTML,
	// bibleTextPlainFromHTML), so the title, a heading under it and verse 1's
	// number reach a selection as separate words. For those pages, two drags
	// are answered here:
	//
	//   - a selection with no span inside the title is the title's only
	//     where the psalm does not have it. Every selection on those pages
	//     comes with no span, so nothing tells the title's letters from the
	//     same letters selected in a verse, and a double-click on a verse's
	//     word is far likelier there than a drag begun inside a word of the
	//     title. So the psalm is searched first, for a copy that splits no
	//     word, and the selection is quoted from the first such copy, as any
	//     selection with no span is placed at its first copy: "Yahweh" in a
	//     psalm whose title names him is the psalm's word, and so is "is" in
	//     one whose title says "his". A selection of verse numbers the psalm
	//     does not have as words is then answered by the verse it labels, as
	//     a verse number is below ("12" under a title that says "12,000").
	//     Anything else, the title whole, a phrase of its own, letters the
	//     psalm has only inside longer words, or what holds nothing
	//     quotable, a lone full stop, is answered as a heading selected on
	//     its own: verse 1, no quotation. So is the title run on into a
	//     heading under it, or to verse 1's number;
	//   - a drag from the title on into the psalm carries the title, or the
	//     end of it, then verse 1's number. Confirmed by that number, the
	//     title comes off, and what follows is the selection. Just the
	//     number left is the title alone again.
	//
	// A heading between the title and verse 1 takes the heading repair below,
	// which removes the title with it.
	editorial := false // the selection carried a heading or a psalm title, now removed
	titleVerse := 0    // a selection with no span inside the title: the verse cited if the psalm does not answer it
	if title, first, ok := chapterTitle(state, book, chapter); ok {
		if !span.valid() {
			if _, ok := afterTitle(flat, title); ok {
				if v, ok := headingOnlySelectionVerse(state, book, chapter, flat, span); ok {
					if !strings.Contains(title, flat) {
						return "", v, v, -1, true
					}
					titleVerse = v
				}
			}
		}
		if titleVerse == 0 {
			num := strconv.Itoa(first)
			if t := trimLeadingHeading(out, title, num); t != out {
				if t == num {
					return "", first, first, -1, true
				}
				out, editorial = t, true
			}
		}
	}
	// A drag begun inside a heading, or a stack of them, run on through the
	// number of the verse beneath, with a span that reaches that verse: the
	// heading's piece comes off the front here, as the title's does above,
	// before anything searches. The search cannot place it. The piece is in
	// no verse, so the selection is found only where its words happen to
	// stand together elsewhere, the number stripped, and from there the
	// repairs cut it: "d\n9 He" from the end of Isaiah 53's "A Grave
	// Assigned" was quoted as "He" from verse 11, and "The Friends\n11 We
	// will" from verse 4 of the BSB's Song of Solomon 1, where the same
	// speaker's label stands inside the verse. The heading repair that
	// follows a failed search never ran for them. The pane's lead cannot place
	// it either, since it reads its way through the heading and fails its
	// check.
	//
	// What is left may hold no word of the verse: the number alone, or the
	// number and the ellipsis the BSB's Ephesians 3:14 opens with. Then the
	// drag's words were the heading's, answered as a heading selected on its
	// own is.
	if !editorial && titleVerse == 0 && span.valid() {
		if t, v, ok := cutOpeningMatter(state, book, chapter, out); ok && v >= span.lo && v <= span.hi {
			if strings.IndexFunc(strings.TrimPrefix(t, strconv.Itoa(v)), isWordRune) < 0 {
				return "", v, v, -1, true
			}
			out, editorial = t, true
		}
	}
	// A heading further down selected whole, with no span. The heading-only
	// answer below asks for a span naming the verse above the heading, which
	// tells a heading from the same words in a verse, and a page with no span
	// has none: the plain-text fallback the Mac and the iPhone and iPad show
	// when a chapter fails to import, which sets each heading on a line of its
	// own, so that selecting its paragraph takes exactly the heading. There it
	// was shared as Scripture under the chapter. So a selection that is a
	// heading whole, perhaps run on to the number below it (the BSB's "Saying
	// 12" stands above verse 12, so its own figure reads as that number), is
	// answered as a heading selected on its own is, the verse below and no
	// quotation, unless the chapter has the heading's words as words, which
	// are then quoted as any selection with no span is. A piece of a heading
	// is not answered so: the same piece may stand in another heading.
	if !span.valid() && titleVerse == 0 && !editorial {
		if v, words, ok := wholeHeadingVerse(state, book, chapter, flat); ok && wordAlignedIndex(corpus, words) < 0 {
			return "", v, v, -1, true
		}
	}
	text, lo, hi, at, ok = locateShareSelection(state, book, chapter, corpus, spans, flat, out, span, editorial, titleVerse > 0)
	if titleVerse > 0 && !ok {
		return "", titleVerse, titleVerse, -1, true
	}
	return text, lo, hi, at, ok
}

// locateShareSelection is normalizeShareSelectionProse's search among the
// verses' words and the repairs that follow it. out is the selection in the
// shared form, with a psalm title it opened with already taken off, and flat
// the selection as it came, which the heading test reads. wholeWords looks
// for out only where it splits no word of the psalm, at the first such copy,
// and answers a selection the psalm does not have that way only if it is
// nothing but verse numbers: it is set for a selection with no span inside
// the title, which is the psalm's only if the psalm has it so.
func locateShareSelection(state *AppState, book string, chapter int, corpus string, spans []verseSpan, flat, out string, span selSpan, editorial, wholeWords bool) (text string, lo, hi, at int, ok bool) {
	// The pane's lead, when it sent one, says where in verse lo the selection
	// begins (placeByLead). Where the selection reads as the corpus from
	// there, it is placed there with its verse numbers, and the headings
	// drawn between its verses, taken out by position, and the strip's text
	// matching does not decide it; otherwise its place is the first one
	// locate tries. Not when a title already came off the front: the
	// selection then no longer begins where the lead says.
	lead := leadPlacement{at: -1, out: out}
	if !editorial && !wholeWords && span.hasLead {
		lead = placeByLead(corpus, spans, pageHeadingsIn(state, book, chapter), span, out)
	}
	if lead.number {
		// The rest of verse lo's number alone, begun inside it: apparatus,
		// as a verse number selected on its own is (below).
		return "", span.lo, span.lo, -1, true
	}
	out = lead.out
	s, anchor := stripVerseMarkersAnchored(corpus, spans, out)
	if lead.text != "" {
		s, anchor = lead.text, lead.at
	}
	if s == "" || corpus == "" {
		return "", 0, 0, -1, false
	}
	validNum := make(map[string]bool, len(spans))
	for _, sp := range spans {
		validNum[strconv.Itoa(sp.verse)] = true
	}
	if wholeWords {
		// With no span there is no region, so locate takes this place as it
		// stands, and nothing after it searches again.
		if anchor = wordAlignedIndex(corpus, s); anchor < 0 {
			// Verse numbers alone, which the page draws and the psalm's
			// words need not hold: the rule for them below.
			if lo, hi := verseNumberTokenRange(s); lo > 0 && allVerseNumbers(s, validNum) {
				return "", lo, hi, -1, true
			}
			return "", 0, 0, -1, false
		}
	}

	rStart, rEnd, haveRegion := 0, len(corpus), false
	if span.valid() {
		for _, sp := range spans {
			if sp.verse >= span.lo && sp.verse <= span.hi {
				if !haveRegion {
					rStart = sp.start
					haveRegion = true
				}
				rEnd = sp.end
			}
		}
	}
	// locate is the one search, for the selection before it is placed. The
	// lead's place comes first, taken on the same terms as the anchor: the
	// corpus has the selection there, inside the span's verses. anchor
	// is where its verse number puts it (-1 for none), taken when the corpus
	// has the selection there and the span, when there is one, agrees: a drag
	// begun on the second digit of 29 arrives as "9 He said to", which the
	// strip reads as verse 9's number when verse 9 opens with those words
	// too, and the span, 29 to 29, is what says it is not. Without an anchor
	// a copy that splits no word comes first: in the region when there is
	// one, and in the chapter when there is none (see
	// normalizeShareSelection).
	//
	// With a region, a selection the region does not hold is found in the
	// chapter at its first copy of any kind, as before the region preferred
	// a copy that splits no word. Such a selection is not a phrase of the
	// span's verses: a drag that runs on past the region into the first
	// letters of the next verse's word or of a heading ("the LORD. The",
	// "The" of "Then"), a verse number selected on its own, a piece of a
	// heading. A whole copy elsewhere in the chapter is no likelier the
	// reader's, and preferring it moved such a drag to a verse it never
	// touched (2 Chronicles 20:13's "the LORD.\nThe" cited 20:32–33). The
	// first copy of any kind is no likelier either for a drag that runs on
	// into the heading below the region, which is placed by that heading
	// instead when this search does not find it in the region
	// (verseEndBeforeHeading).
	inRegion := func(at int) bool { return !haveRegion || (at >= rStart && at < rEnd) }
	locate := func(s string, anchor int) int {
		if lead.at >= 0 && inRegion(lead.at) && strings.HasPrefix(corpus[lead.at:], s) {
			return lead.at
		}
		if anchor >= 0 && inRegion(anchor) && strings.HasPrefix(corpus[anchor:], s) {
			return anchor
		}
		if haveRegion {
			if i := wordAlignedIndex(corpus[rStart:rEnd], s); i >= 0 {
				return rStart + i
			}
			if i := strings.Index(corpus[rStart:rEnd], s); i >= 0 {
				return rStart + i
			}
		} else if i := wordAlignedIndex(corpus, s); i >= 0 {
			return i
		}
		return strings.Index(corpus, s)
	}
	// dropNumber is called when a drag's trailing number has come off, with
	// what is left, s: that number's verse had nothing in the drag but its
	// number, so where the verses before it hold s, the region ends before
	// it. The words before the number end where that verse begins, and the
	// region's preference for a copy that splits no word would otherwise take
	// a whole copy in that verse over the reader's: "in Christ 9", begun two
	// letters into Philippians 3:8's "gain", was quoted from 3:9's "in Christ"
	// and cited there. Where they do not hold s, the words are not the end of
	// a verse, a heading's last word say, and the region stays as it was.
	dropNumber := func(num, s string) {
		if !haveRegion {
			return
		}
		n, err := strconv.Atoi(num)
		if err != nil {
			return
		}
		for k, sp := range spans {
			if sp.verse == n && k > 0 && sp.start > rStart && sp.start < rEnd {
				if end := spans[k-1].end; strings.Contains(corpus[rStart:end], s) {
					rEnd = end
				}
				return
			}
		}
	}

	// A trailing bare verse-number token (the selection stopped right after a
	// superscript marker) never appears in the prose corpus — drop it so the
	// locate below can succeed. Verified against the chapter's real verse
	// numbers (validNum) so a legitimate number inside verse text is never
	// touched (that one locates fine with the number, so this branch is
	// skipped).
	numbers := s
	for locate(s, anchor) < 0 {
		i := strings.LastIndexByte(strings.TrimSpace(s), ' ')
		last := strings.TrimSpace(s)
		if i >= 0 {
			last = last[i+1:]
		}
		if !validNum[last] {
			break
		}
		if i < 0 {
			// The selection was nothing but verse numbers — a double-tap or
			// a long-press on a superscript selects it as a word. A number is
			// the page's apparatus, not a word of scripture, so it is quoted
			// no more than a heading selected on its own is (below): the quote
			// is empty, and the reference names the verse the number labels,
			// which the number itself says whatever span came with it.
			lo, hi := verseNumberTokenRange(numbers)
			return "", lo, hi, -1, true
		}
		s = strings.TrimSpace(strings.TrimSpace(s)[:i])
		dropNumber(last, s)
	}

	idx := locate(s, anchor)
	// A drag that runs on past the end of the span's last verse stops in the
	// heading the page draws below that verse: "gold.\nT" from the BSB's 2
	// Chronicles 3:4, which ends "pure gold." above a heading that opens
	// "The", spanned 4 to 4. The region does not hold that, and the chapter's
	// first copy of it ran from verse 8's "fine gold." into verse 9's first
	// word, which the end repair cut back to verse 8's "gold." and cited. So
	// where the region does not hold the selection, the front of the heading
	// comes off its end first, and what is left, when it is how the region
	// ends, is placed there: the words a drag carried into a heading are the
	// last words of the verse above it. It is the selection as the drag ended
	// it that ends with the heading, before the loop above took a number off:
	// a drag to the number of the verse below a heading named "Saying 10"
	// ends "Saying 10 9", and with both numbers taken off, "Saying" also opens
	// the next verse's heading, "Saying 11", which that drag never reached.
	if haveRegion && (idx < rStart || idx+len(s) > rEnd) {
		if r, at, ok := verseEndBeforeHeading(state, book, chapter, corpus, spans, rStart, rEnd, numbers); ok {
			s, idx, editorial = r, at, true
		}
	}
	if idx < 0 {
		// A selection that will not locate may be carrying a publisher's
		// heading, which is drawn between verses but is in no verse and so is
		// absent from the corpus. Remove it and try once more.
		//
		// It runs ONLY here, and only its result is adopted, which is a second
		// line of defence rather than the first. The first is that every trim
		// demands confirmation — the verse number the heading stands above, or
		// a fragment that is genuinely a prefix or suffix of the heading's own
		// words. Moving this up front does not break anything today, so the
		// placement is not what makes it safe; it is what keeps it safe if the
		// matching is ever loosened. A repair that can only touch a selection
		// already known to be unresolvable cannot move an ordinary reader's
		// note to the wrong verse, whatever the matching decides.
		// The retry works from the shared form, as the first pass did: the
		// corpus it searches is in that form, and a candidate rebuilt from
		// the raw selection would still carry the gap mark or the superscript
		// that made the first locate miss.
		// What is left must be verse text: a verse number left on its own
		// (the matter and the number of the verse it stands above, nothing
		// after) is the page's apparatus, answered below, and is not looked
		// for among the verses' words.
		// A drag that ran on to the next verse's number ends with it, as the
		// first pass found, and it comes off here the same way.
		if h := stripHeadings(state, book, chapter, out); h != out {
			if r, a := stripVerseMarkersAnchored(corpus, spans, h); r != "" && !validNum[r] {
				for locate(r, a) < 0 {
					i := strings.LastIndexByte(r, ' ')
					if i < 0 || !validNum[r[i+1:]] {
						break
					}
					r = strings.TrimSpace(r[:i])
				}
				if j := locate(r, a); j >= 0 {
					s, idx, editorial = r, j, true
				}
			}
		}
		// Nothing located, and nothing left after the headings were removed:
		// the selection was ONLY editorial matter. It still belongs somewhere,
		// and the model already says where — a heading stands above a verse.
		if idx < 0 {
			if v, ok := headingOnlySelectionVerse(state, book, chapter, flat, span); ok {
				// No quotation: the reader selected a heading, so quoting the
				// verse would attribute words they did not choose, and quoting
				// the heading would put editorial matter into scripture. The
				// reference alone is the honest answer.
				return "", v, v, -1, true
			}
		}
	}
	if idx < 0 {
		return "", 0, 0, -1, false
	}

	// From here s is corpus[idx:idx+len(s)], and the repairs only cut it
	// shorter: cutFront drops its first n bytes and the space after them,
	// cutBack keeps its first n. Where the shorter text stands follows from
	// the cut, so neither searches (see normalizeShareSelection).
	cutFront := func(n int) {
		rest := s[n:]
		trimmed := strings.TrimLeftFunc(rest, unicode.IsSpace)
		idx += n + len(rest) - len(trimmed)
		s = strings.TrimRightFunc(trimmed, unicode.IsSpace)
	}
	cutBack := func(n int) {
		s = strings.TrimRightFunc(s[:n], unicode.IsSpace)
	}

	// Mid-word END repair: if the character right after the selection is a
	// word rune, the drag stopped inside a word — trim back to the last whole
	// word. A verse number the drag ended on never gets this far: the
	// marker strip took it with the start of its verse, or the loop above
	// dropped it with nothing after it, and what is left is verse text.
	for {
		end := idx + len(s)
		if end < len(corpus) {
			if r, _ := utf8.DecodeRuneInString(corpus[end:]); isWordRune(r) {
				cut := strings.LastIndexByte(s, ' ')
				if cut <= 0 {
					// A single partial word — nothing left. When it opens a
					// verse whose heading or title the drag also took, the
					// drag's only words were that editorial matter, which
					// is answered as a heading selected on its own is: the
					// verse it stands above, and no quotation. Anywhere
					// else it is a fragment, and the caller's fallback
					// shares it as one.
					if editorial {
						for _, sp := range spans {
							if sp.start == idx {
								return "", sp.verse, sp.verse, -1, true
							}
						}
					}
					return "", 0, 0, -1, false
				}
				cutBack(cut)
				continue
			}
		}
		break
	}

	// Orphan punctuation the drag swept in from NEIGHBORING sentences: a
	// leading ". " (the previous sentence's terminal) or closing mark belongs
	// to text OUTSIDE the quotation and can never open one; a trailing
	// clause mark (comma, colon, semicolon, dash) directly at an end-cut is
	// dropped per the usual Bluebook practice of omitting punctuation
	// adjacent to an omission (e.g. Maizebook 5.3(d)). Terminals stay — they
	// mark a COMPLETE sentence; opening marks stay — they introduce quoted
	// content inside the selection.
	for s != "" {
		r, size := utf8.DecodeRuneInString(s)
		if r == '.' || r == ',' || r == ';' || r == ':' || r == '!' || r == '?' ||
			r == '…' || r == '—' || r == ')' || r == ']' || r == '”' || r == '’' {
			cutFront(size)
			if s == "" {
				return "", 0, 0, -1, false
			}
			continue
		}
		break
	}
	for s != "" {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == ',' || r == ';' || r == ':' || r == '—' {
			cutBack(len(s) - size)
			if s == "" {
				return "", 0, 0, -1, false
			}
			continue
		}
		break
	}

	// Mid-word START repair, symmetrically.
	for idx > 0 {
		r, _ := utf8.DecodeLastRuneInString(corpus[:idx])
		if !isWordRune(r) {
			break
		}
		cut := strings.IndexByte(s, ' ')
		if cut < 0 {
			return "", 0, 0, -1, false
		}
		cutFront(cut + 1)
		if s == "" {
			return "", 0, 0, -1, false
		}
	}

	// Attribute: every verse whose span overlaps the final selection.
	end := idx + len(s)
	for _, sp := range spans {
		if sp.start < end && idx < sp.end {
			if lo == 0 {
				lo = sp.verse
			}
			hi = sp.verse
		}
	}
	if lo == 0 {
		return "", 0, 0, -1, false
	}
	return s, lo, hi, idx, true
}

// completeTrailingSentence restores the ORIGINAL final punctuation when a
// selection stops after a sentence's last word but before its terminal — the
// reader quoted every word of the sentence and merely didn't drag across the
// period. Rule 5.3's ellipsis marks omitted WORDS, so marking this case with
// the four-dot form would claim an omission that never happened (field-
// reported: "…seen and heard . . . ." for a selection missing only the "."
// of Acts 4:20). The check is positional: locate the selection in the chapter
// prose and scan forward — if nothing but closing quotation marks stands
// between the cut and the sentence's terminal punctuation, append that
// terminal (addEndOmission then sees a complete sentence and adds no mark);
// if any word intervenes, leave the text alone and let the ellipsis do its
// honest work. No-op whenever the selection can't be located.
// at is the selection's located offset in chapterProse coordinates when the
// caller has one (normalizeShareSelection computed it), -1 otherwise. The
// positional path is what keeps repeated wording honest: located at the FIRST
// copy, the forward scan read the wrong copy's surroundings and completed (or
// refused to complete) the wrong sentence.
func completeTrailingSentence(state *AppState, s string, at int) string {
	book, chapter := readerChapter(state)
	return completeTrailingSentenceIn(state, book, chapter, s, at)
}

func completeTrailingSentenceIn(state *AppState, book string, chapter int, s string, at int) string {
	if state == nil || state.Bible == nil || s == "" || endsOnTerminal(s) {
		return s
	}
	corpus, _ := chapterProseIn(state, book, chapter)
	return completeTrailingSentenceProse(corpus, s, at)
}

// endsOnTerminal reports a text that already ends on its sentence's terminal
// punctuation, inside any closing quotation marks: nothing to complete.
func endsOnTerminal(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(s, " \t”’\"'"))
	return r == '.' || r == '!' || r == '?' || r == '…'
}

// completeTrailingSentenceProse is completeTrailingSentenceIn for a caller
// that has already built the chapter: corpus must be chapterProseIn's.
func completeTrailingSentenceProse(corpus, s string, at int) string {
	if s == "" || endsOnTerminal(s) {
		return s // already complete
	}
	idx := -1
	if at >= 0 && at+len(s) <= len(corpus) && strings.HasPrefix(corpus[at:], s) {
		idx = at
	} else {
		idx = strings.Index(corpus, s)
	}
	if idx < 0 {
		return s
	}
	// Punctuation may stand between the cut and the terminal without any WORD
	// being omitted — John 1:41 (WEB) ends "…, Christ)." and a drag stopping
	// after "Christ" omits only ")racket-and-period". Closing parens/brackets
	// are carried into the completion only when their opener is inside the
	// selection (never introduce an unbalanced mark); quotation closers are
	// traversed but left to balanceQuoteMarks; any word rune aborts — that
	// omission is real and earns its four-dot mark.
	surplusParen := strings.Count(s, "(") - strings.Count(s, ")")
	surplusBrack := strings.Count(s, "[") - strings.Count(s, "]")
	var pending strings.Builder
	for _, r := range corpus[idx+len(s):] {
		switch {
		case r == '.' || r == '!' || r == '?' || r == '…':
			return s + pending.String() + string(r)
		case r == '”' || r == '’' || r == '"' || r == '\'' || r == ' ':
			continue // closing quotation marks / spacing
		case r == ')':
			if surplusParen > 0 {
				surplusParen--
				pending.WriteRune(r)
			}
			continue
		case r == ']':
			if surplusBrack > 0 {
				surplusBrack--
				pending.WriteRune(r)
			}
			continue
		default:
			return s // a word (or clause punctuation implying more words) intervenes
		}
	}
	return s
}

// stripVerseMarkers removes the superscript verse-number tokens that ride along
// in a selection ("… heard.” 21 After …" → "… heard.” After …"). A number is
// treated as a marker ONLY when the text after it matches the opening of that
// very verse — for ANY overlap length, so a verse cut two characters in ("21
// Af") is recognized just like a whole one (the legacy 12-rune probe missed
// short tails, which is how a marker once leaked into a shared card). A number
// appearing inside verse prose never matches its own verse's opening, so it is
// never touched.
// stripHeadings removes the publisher's section headings from a raw selection.
//
// A heading is editorial matter. It is drawn between verses but belongs to no
// verse, so it is absent from the prose corpus the normalize searches
// (chapterProseIn builds that from verse text alone). A selection that opens at
// a heading therefore cannot be located at all: the normalize declines, and the
// caller keeps the raw span the native side reported — which began at the verse
// ABOVE the heading, because a heading sits after that verse's text and before
// the next verse's number. The note then highlights a verse nobody selected.
//
// Only an occurrence the rendered text CONFIRMS is removed: the heading must be
// followed by the number of the verse it stands above (Heading.BeforeVerse).
// That is what stops a heading whose words also occur in scripture from eating
// them — Psalm 23's "The LORD Is My Shepherd" stands directly above a verse
// that opens with those very words, and a looser match would delete the verse's
// own opening and leave the selection unlocatable in a new way. The number is
// left in place for stripVerseMarkers, which runs next and knows how to check
// it against the verse body.
//
// The front comes off first, and it decides what the end may lose. A drag
// that began inside the matter above verse N and ran on into it carries a
// piece of that matter, N, and N's first words, and trimTrailingHeading,
// which has no number to confirm it, would take those words for the front of
// a heading: "The LORD Is My Shepherd 1 The LORD" lost "The LORD" to the very
// heading it had just shed, and Genesis 4's "Descendants of Cain 17 And Cain"
// lost "Cain" to "Cain and Abel" above verse 1, and shared "And". A heading
// whose front a drag can end in stands below the verse the drag began in, so
// once the front has come off at N only a heading below N can lose its front
// from the end. And the drag reached that front through the end of the verse
// the heading stands under, so what is left must end as that verse does:
// Genesis 10's "The Japhethites 2 The" keeps its "The", which "The Hamites"
// below verse 5 would otherwise take, since verse 5 does not end "The". Of
// the BSB's 189,996 drags of one to three words from inside a heading
// through the next verse's number, 15,081 shared the heading as Scripture or
// cut the verse short, under 682 headings.
func stripHeadings(state *AppState, book string, chapter int, s string) string {
	if state == nil || state.Bible == nil {
		return s
	}
	opened := 0 // the verse whose matter the selection opened in, once cut
	if t, v, ok := cutOpeningMatter(state, book, chapter, s); ok {
		s, opened = t, v
	}
	for _, h := range state.Bible.Headings[book][chapter] {
		// As DRAWN: the selection is taken from the page, where the heading's
		// divine name is set in small capitals.
		text := collapseSpaces(h.DrawnText())
		if text == "" || h.BeforeVerse <= 0 {
			continue
		}
		if opened == 0 || h.BeforeVerse > opened {
			if t := trimTrailingHeading(s, text); t != s && endsVerseAbove(state, book, chapter, h.BeforeVerse, t) {
				s = t
			}
		}
		s = removeWholeHeadings(s, text, strconv.Itoa(h.BeforeVerse))
	}
	return strings.TrimSpace(s)
}

// cutOpeningMatter takes off the front of s the piece of editorial matter a
// drag begun inside it carries, and reports the verse the matter stands
// above, v, whose number s then opens with. ok is false when s does not open
// so.
//
// The matter above a verse is what the page draws there, top to bottom: a
// psalm's title above the chapter's first verse and any heading there, then
// the headings standing above the verse, a stack of them joined as a
// flattened selection joins them. A drag begun inside it gives a piece that
// ends it (trimLeadingHeading), confirmed by v's number after it.
//
// A psalm's title is editorial matter too. A page that hands it over (the
// plain-text fallback of the Mac and the iPhone and iPad; the panes with
// verse geometry cut a selection at verse 1's number, see
// normalizeShareSelectionProse) has it above the chapter's first verse and
// any heading there, so a drag from it into the psalm carries the title, or
// the end of it, at its front.
//
// What stands before v's number on the page is the matter, and before that
// the end of the verse above. A piece that is also how that verse ends ("d"
// of a verse ending "and"), with nothing between it and the number, could be
// either, and is left for the search to place.
func cutOpeningMatter(state *AppState, book string, chapter int, s string) (rest string, v int, ok bool) {
	above := map[int]string{}
	var order []int
	if title, first, ok := chapterTitle(state, book, chapter); ok {
		above[first] = title
		order = append(order, first)
	}
	for _, h := range state.Bible.Headings[book][chapter] {
		text := collapseSpaces(h.DrawnText())
		if text == "" || h.BeforeVerse <= 0 {
			continue
		}
		if m, ok := above[h.BeforeVerse]; ok {
			above[h.BeforeVerse] = m + " " + text
			continue
		}
		above[h.BeforeVerse] = text
		order = append(order, h.BeforeVerse)
	}
	for _, v := range order {
		i := leadingHeadingNumber(s, above[v], strconv.Itoa(v))
		if i < 0 || endsVerseAbove(state, book, chapter, v, strings.TrimSpace(s[:i])) {
			continue
		}
		return strings.TrimSpace(s[i:]), v, true
	}
	return s, 0, false
}

// endsVerseAbove reports whether s, a selection in the shared form, ends as
// the verse above verse n does: the words after its last number token, which
// are those a drag carried out of that verse into the heading over n, are
// how the verse's text ends. A heading at the top of a chapter has no verse
// above it, and nothing ends that way.
func endsVerseAbove(state *AppState, book string, chapter, n int, s string) bool {
	above, ok := verseAbove(state, book, chapter, n)
	if !ok {
		return false
	}
	var text string
	for _, v := range state.Bible.GetChapter(book, chapter) {
		if v.Verse == above {
			text = collapseSpaces(verseSharedText(v))
			break
		}
	}
	words := strings.Fields(s)
	k := len(words)
	for k > 0 && strings.IndexFunc(words[k-1], func(r rune) bool { return !unicode.IsDigit(r) }) >= 0 {
		k--
	}
	tail := strings.Join(words[k:], " ")
	return text != "" && strings.IndexFunc(tail, isWordRune) >= 0 && strings.HasSuffix(text, tail)
}

// chapterTitle is the chapter's psalm title as the page draws it, and the
// first verse the page draws, which the title stands above. ok is false for
// a chapter without one.
func chapterTitle(state *AppState, book string, chapter int) (title string, first int, ok bool) {
	title = collapseSpaces(state.Bible.SuperscriptionFor(book, chapter).DrawnText())
	if title == "" {
		return "", 0, false
	}
	for _, v := range state.Bible.GetChapter(book, chapter) {
		if collapseSpaces(v.Text) != "" {
			return title, v.Verse, true
		}
	}
	return "", 0, false
}

// afterTitle reports whether s opens with the psalm title, or a piece of
// it, and what follows. A drag inside the title gives any piece of it; one
// that runs on past it gives a piece that ends it, perhaps begun mid-word,
// and then whatever the page draws next.
func afterTitle(s, title string) (rest string, ok bool) {
	if strings.Contains(title, s) {
		return "", true
	}
	for i := len(s) - 1; i > 0; i-- {
		if s[i] == ' ' && strings.HasSuffix(title, s[:i]) {
			return strings.TrimSpace(s[i+1:]), true
		}
	}
	return "", false
}

// headingOnlySelectionVerse reports the verse a selection belongs to when that
// selection is nothing but a publisher's heading, or a piece of one.
//
// Heading.BeforeVerse already records the answer — "the verse this heading
// stands above" — and it is the only answer that survives the note leaving this
// device. A note anchored to a heading would not place for a reader on another
// translation, and a shared link opens the public-domain web reader, whose
// headings are not the licensed edition's.
//
// TWO conditions, and the second is what keeps this from stealing ordinary
// selections. The words must be part of the heading, AND the span the native
// side reported must name the verse immediately ABOVE that heading — which is
// what a drag inside the heading produces, because the last verse number before
// a heading belongs to the verse above it. A reader who selects the word
// "Gentiles" down in a verse gets that verse's own span and is left alone, even
// though the same word appears in the heading. Without the span test this would
// quietly move such a selection to the heading's verse.
//
// A heading at the top of the chapter has no verse above it, so no span can
// name one. The native panes report a drag inside it as no span at all (the
// last verse number before it is none: iOS, macOS and Android alike), or,
// when the drag reaches the first verse's number, as that verse, since
// selSpanFromNative clamps an opening above verse 1 to verse 1. Either is the
// shape of a selection there, and neither can belong to a verse further down,
// whose own span starts at that verse. The words must still be the heading's.
//
// A heading further down selected whole with no span is answered before the
// search, by wholeHeadingVerse; a piece of one with no span is left alone
// here, since the same piece may stand in another heading.
//
// The words may end with the number of the verse the heading stands above: a
// drag that ran on past the heading's last word stops at the next word the
// page draws, which is that number. The number is the heading's own
// confirmation (stripHeadings asks for the same), not a word of the verse, so
// the selection is still the heading alone.
func headingOnlySelectionVerse(state *AppState, book string, chapter int, s string, span selSpan) (int, bool) {
	if state == nil || state.Bible == nil {
		return 0, false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	// A psalm's title, or a piece of it, stands above the chapter's first
	// verse as a heading at the top of a chapter does, and is answered the
	// same way: the verse it stands above, by the same span test. A heading
	// the publisher set under the title may follow it in the selection, and
	// what follows the title is then checked as a heading below.
	if title, first, ok := chapterTitle(state, book, chapter); ok && (!span.valid() || span.lo <= first) {
		if rest, ok := afterTitle(s, title); ok {
			if rest == "" || rest == strconv.Itoa(first) {
				return first, true
			}
			s = rest
		}
	}
	for _, h := range state.Bible.Headings[book][chapter] {
		// As drawn, for the reason stripHeadings gives.
		text := collapseSpaces(h.DrawnText())
		if text == "" || h.BeforeVerse <= 0 {
			continue
		}
		words := s
		if num := " " + strconv.Itoa(h.BeforeVerse); strings.HasSuffix(words, num) {
			words = strings.TrimSpace(strings.TrimSuffix(words, num))
		}
		if words == "" || !strings.Contains(text, words) {
			continue
		}
		above, ok := verseAbove(state, book, chapter, h.BeforeVerse)
		if ok && span.valid() && span.lo == above {
			return h.BeforeVerse, true
		}
		if !ok && (!span.valid() || span.lo <= h.BeforeVerse) {
			return h.BeforeVerse, true
		}
	}
	return 0, false
}

// wholeHeadingVerse is the verse a heading of the chapter stands above when
// s is that heading whole, as drawn, perhaps run on to the number of that
// verse, and the heading's words in the shared form; the first such heading
// when the chapter has more than one of those words.
func wholeHeadingVerse(state *AppState, book string, chapter int, s string) (verse int, words string, ok bool) {
	s = strings.TrimSpace(s)
	for _, h := range state.Bible.Headings[book][chapter] {
		text := collapseSpaces(h.DrawnText())
		if text != "" && h.BeforeVerse > 0 && (s == text || s == text+" "+strconv.Itoa(h.BeforeVerse)) {
			return h.BeforeVerse, sharedText(text), true
		}
	}
	return 0, "", false
}

// allVerseNumbers reports whether every word of s is one of the chapter's
// verse numbers (valid).
func allVerseNumbers(s string, valid map[string]bool) bool {
	fields := strings.Fields(s)
	for _, f := range fields {
		if !valid[f] {
			return false
		}
	}
	return len(fields) > 0
}

// verseNumberTokenRange is the lowest and highest of the verse numbers s is
// made of, for a selection normalizeShareSelectionIn has found to be nothing
// else.
func verseNumberTokenRange(s string) (lo, hi int) {
	for _, tok := range strings.Fields(s) {
		n, err := strconv.Atoi(tok)
		if err != nil {
			continue
		}
		if lo == 0 || n < lo {
			lo = n
		}
		if n > hi {
			hi = n
		}
	}
	return lo, hi
}

// verseAbove is the chapter's last verse before n — the verse a heading that
// stands above n sits after, and therefore the verse a drag inside that heading
// is reported against.
func verseAbove(state *AppState, book string, chapter, n int) (int, bool) {
	best, found := 0, false
	for _, v := range state.Bible.GetChapter(book, chapter) {
		if v.Verse < n && v.Verse > best {
			best, found = v.Verse, true
		}
	}
	return best, found
}

// trimLeadingHeading removes PART of a heading from the front of a selection.
//
// A drag that began inside the heading's own words leaves a fragment of it at
// the front, and because a drag can start in the middle of a word the fragment
// is a byte suffix of the heading rather than a word-aligned one —
// "pirit Falls on the Gentiles" is a shape a reader really produces.
//
// What separates such a fragment from a selection that began in the verse
// ABOVE the heading is that the latter carries that verse's words first and
// then the heading ENTIRE, so it is not a suffix of the heading at all. The
// verse number still has to follow, as everywhere here, and the number is left
// in place for stripVerseMarkers.
//
// The heading may hold the number itself as a word: the BSB's "Saying 12"
// stands above verse 12, and the Catholic WEB's note fused into the heading
// over Daniel 3:24 says "24 of the traditional Hebrew Bible". So each place
// the number stands as a word is tried, from the front, and the first with
// a piece of the heading before it is the one the drag ran through.
func trimLeadingHeading(s, text, num string) string {
	if i := leadingHeadingNumber(s, text, num); i >= 0 {
		return strings.TrimSpace(s[i:])
	}
	return s
}

// leadingHeadingNumber is where num stands as a word in s after a piece of
// text that s opens with, or -1 (trimLeadingHeading).
func leadingHeadingNumber(s, text, num string) int {
	for from := 0; from < len(s); {
		i := numberTokenIndex(s[from:], num)
		if i < 0 {
			return -1
		}
		i += from
		if head := strings.TrimSpace(s[:i]); head != "" && strings.HasSuffix(text, head) {
			return i
		}
		from = i + len(num)
	}
	return -1
}

// trimTrailingHeading removes a heading, or the FRONT of one, from the end of a
// selection — the shape a drag makes when it begins in a verse and stops inside
// the heading below it. It is the mirror of trimLeadingHeading: a fragment left
// at the end is a byte prefix of the heading, where one left at the front is a
// byte suffix.
//
// There is no verse number after a trailing heading to confirm the match, so
// the confirmation is the caller's instead: this only runs when the selection
// failed to locate, and the result is only adopted when the trimmed selection
// locates. A trim that removed something real would not locate either, and is
// discarded.
func trimTrailingHeading(s, text string) string {
	for n := len(text); n > 0; n-- {
		frag := strings.TrimSpace(text[:n])
		if frag == "" {
			continue
		}
		if strings.HasSuffix(s, " "+frag) {
			return strings.TrimSpace(s[:len(s)-len(frag)-1])
		}
	}
	return s
}

// verseEndBeforeHeading takes off the end of s the front of the heading the
// page draws below the verse whose text ends the region [start, end) of the
// corpus, or, where the page stacks more than one heading there, the
// headings before it whole and then its front, as trimTrailingHeading takes
// off any heading's. It reports what is left and where that stands, at, only
// when what is left holds a word and is how the region ends: the drag ran
// from those words on into the heading, so they end where the verse does.
// Anything else, ok false, leaves the selection to the search.
func verseEndBeforeHeading(state *AppState, book string, chapter int, corpus string, spans []verseSpan, start, end int, s string) (rest string, at int, ok bool) {
	above, below := 0, 0
	for k, sp := range spans {
		if sp.end == end {
			above = sp.verse
			if k+1 < len(spans) {
				below = spans[k+1].verse
			}
			break
		}
	}
	if above == 0 || below == 0 {
		return "", -1, false
	}
	var heads []string
	for _, h := range state.Bible.Headings[book][chapter] {
		if h.BeforeVerse > above && h.BeforeVerse <= below {
			if text := sharedText(collapseSpaces(h.DrawnText())); text != "" {
				heads = append(heads, text)
			}
		}
	}
	region := corpus[start:end]
	for k := len(heads) - 1; k >= 0; k-- {
		whole := strings.Join(heads[:k], " ")
		for n := len(heads[k]); n > 0; n-- {
			piece := strings.TrimSpace(heads[k][:n])
			if piece == "" {
				continue
			}
			if whole != "" {
				piece = whole + " " + piece
			}
			if !strings.HasSuffix(s, " "+piece) {
				continue
			}
			rest = strings.TrimSpace(s[:len(s)-len(piece)-1])
			if strings.IndexFunc(rest, isWordRune) >= 0 && strings.HasSuffix(region, rest) {
				return rest, end - len(rest), true
			}
		}
	}
	return "", -1, false
}

// removeWholeHeadings removes complete headings wherever they sit in the
// selection — a drag that runs from the verse above, through the heading, into
// the verses below carries one in its middle.
func removeWholeHeadings(s, text, num string) string {
	marker := text + " " + num
	for from := 0; from <= len(s)-len(marker); {
		i := strings.Index(s[from:], marker)
		if i < 0 {
			break
		}
		i += from
		end := i + len(marker)
		// The heading must start at a word boundary, and the verse number must
		// end at one — otherwise "4" would match inside "44", and a heading
		// that happens to end a longer phrase would be clipped out of the
		// middle of it.
		if (i != 0 && s[i-1] != ' ') || (end < len(s) && s[end] != ' ') {
			from = i + len(text)
			continue
		}
		s = s[:i] + s[i+len(text)+1:]
		from = i
	}
	return s
}

// numberTokenIndex is the offset of num in s as a whole space-delimited token,
// or -1. It is what stops "4" being found inside "44".
func numberTokenIndex(s, num string) int {
	for from := 0; from <= len(s)-len(num); {
		i := strings.Index(s[from:], num)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(num)
		if (i == 0 || s[i-1] == ' ') && (end == len(s) || s[end] == ' ') {
			return i
		}
		from = i + len(num)
	}
	return -1
}

// stripVerseMarkers takes a selection in the SHARED form (sharedText has run
// on it) and compares each token's aftermath with the verse body in that same
// form. The two must agree byte for byte for the token to be confirmed, which
// is why the body is verseSharedText and not the stored Verse.Text: a
// divine-name verse is drawn in small capitals and stored in the publisher's
// mixed case, and only the drawn form is shared between the selection and the
// corpus.
func stripVerseMarkers(state *AppState, book string, chapter int, s string) string {
	corpus, spans := chapterProseIn(state, book, chapter)
	return stripVerseMarkersProse(corpus, spans, s)
}

// stripVerseMarkersProse is stripVerseMarkers against a chapter the caller has
// already built with chapterProseIn. Each span's slice of the corpus is that
// verse's body in exactly the form the strip compares against —
// collapseSpaces(verseSharedText(v)) — in chapter order, with the verses whose
// body is empty left out, as the strip leaves them out.
func stripVerseMarkersProse(corpus string, spans []verseSpan, s string) string {
	s, _ = stripVerseMarkersAnchored(corpus, spans, s)
	return s
}

// stripVerseMarkersAnchored is stripVerseMarkersProse that also says where its
// result stands in the corpus, when the selection carries a verse number to
// say so. A number the strip confirms is the page's own statement that its
// verse's text begins at that point of the selection, so the result begins
// that many bytes before the verse's start: at is that corpus offset, from
// the number nearest the front, or -1 when no number was removed. It is a
// claim for the caller to check against the corpus, not a promise: anything
// in the selection that is not verse text, a heading say, makes it wrong.
func stripVerseMarkersAnchored(corpus string, spans []verseSpan, s string) (out string, at int) {
	pos, verseStart := -1, 0 // where in s the nearest-the-front number's verse begins
	for _, sp := range spans {
		body := corpus[sp.start:sp.end]
		marker := strconv.Itoa(sp.verse) + " "
		for from := 0; ; {
			i := strings.Index(s[from:], marker)
			if i < 0 {
				break
			}
			i += from
			atBoundary := i == 0 || s[i-1] == ' '
			rest := s[i+len(marker):]
			overlap := len(rest)
			if len(body) < overlap {
				overlap = len(body)
			}
			if atBoundary && overlap > 0 && rest[:overlap] == body[:overlap] {
				s = s[:i] + rest
				// A number removed after the recorded one leaves it where
				// it is; one removed before it is nearer the front.
				if pos < 0 || i <= pos {
					pos, verseStart = i, sp.start
				}
				from = i
				continue
			}
			from = i + len(marker)
		}
	}
	trimmed := strings.TrimLeftFunc(s, unicode.IsSpace)
	if pos >= 0 {
		pos -= len(s) - len(trimmed)
	}
	out = strings.TrimRightFunc(trimmed, unicode.IsSpace)
	if pos < 0 || verseStart-pos < 0 {
		return out, -1
	}
	return out, verseStart - pos
}

// verseRangeCitation renders "Book C:lo" / "Book C:lo–hi" (en dash per Bluebook).
func verseRangeCitation(book string, ch, lo, hi int) string {
	if lo == hi {
		return fmt.Sprintf("%s %d:%d", book, ch, lo)
	}
	return fmt.Sprintf("%s %d:%d–%d", book, ch, lo, hi)
}

// blockQuoteWords is the Bluebook Rule 5.1 threshold: a quotation of 50 or more
// words is set off as a block quotation rather than run inline in quotation marks.
const blockQuoteWords = 50

// formatBibleQuote prepares a clean verse string for sharing, in Bluebook style. A
// share always presents the quotation STANDING AS ITS OWN SENTENCE(S) — Rule 5.3(b)
// therefore governs both ends: a selection that begins mid-sentence takes a
// bracketed capital, never a leading ellipsis (5.3(b)(i)); one that ends
// mid-sentence takes the four-dot form (5.3(b)(iii)). Formatting per Rule 5.1:
//   - 50+ words → a BLOCK quotation: set off WITHOUT surrounding quotation marks (the
//     card's centered, wide-margined block is the faithful analog of the "indented
//     both sides" block form). The verse's own marks are reproduced exactly as in the
//     source — including internal DOUBLE marks, since a block has no enclosing pair to
//     nest inside (B5.2).
//   - under 50 words → an INLINE quotation: wrap the whole fragment in outer DOUBLE
//     quotation marks. Any quotation that appears WITHIN the verse is a
//     quote-within-a-quote, so its marks drop one level to SINGLE (“ ” → ‘ ’) before
//     the outer pair is added (Rule 5.1(b) nesting) — e.g. John 18:38 becomes
//     “‘truth?’ Pilate asked … told them, ‘I find no basis…’”.
//   - a selection lying ENTIRELY inside a quotation in the source (Jesus speaking)
//     correctly gets ONE set of plain double marks — Rule 5.2(f)(i): omit the
//     enclosing internal marks when the whole quotation is itself a quotation.
//
// Documented deviations (deliberate, for a chat/card medium): a third nesting
// level is not re-alternated back to double (5.1(b)(i)) — the closing single
// glyph ’ doubles as the apostrophe, so auto-flipping is unreliable; and the
// card centers its citation rather than starting it at the left margin on the
// following line. Authored structure is kept on BOTH share paths, as Rule
// 5.1(a)(iii) asks of a block: restoreShareLineBreaks runs before either, the
// text share carries its "\n"/"\n\n" verbatim, and the card lays each poem
// line out on its own (poemSegments) — a paragraph boundary there is a line
// break too, its blank line dropped.
//
// It stays faithful to the SELECTION otherwise: balanceQuoteMarks repairs marks whose
// partner sits in the unselected surrounding text; no words are added or dropped.
// terminal, when given, is the punctuation that ends the sentence the selection was
// cut from — Rule 5.3(b)(iii) retains it in the four-dot slot (" . . . ?" for a
// question cut short); it defaults to a period.
func formatBibleQuote(text string, terminal ...rune) string {
	term := '.'
	if len(terminal) > 0 && terminal[0] != 0 {
		term = terminal[0]
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	// Mark a mid-sentence cut (Rule 5.3(b)(iii)), balance the verse's own double
	// marks so the omission and any added marks sit at the right level, then give a
	// mid-sentence START its bracketed capital (Rule 5.3(b)(i) + 5.2(a)).
	// Count the QUOTED words before any marks are added: "[o]mitted words and
	// ellipses should not be considered in the word count" (Yale S.R. 5.1(a),
	// implementing Rule 5.1) — the four-dot mark alone would otherwise push a
	// 46–49-word cut selection into block form.
	quotedWords := len(strings.Fields(text))
	text = addEndOmission(text, term)
	text = resolveEnclosingQuotes(text)
	text = balanceQuoteMarks(text)
	text = bracketStartCapital(text)
	if quotedWords >= blockQuoteWords {
		return text // block quotation: reproduce the source's marks, no outer marks
	}
	// Inline: a verse's own CURLY double quotations are nested down to single marks
	// (Rule 5.1(b)) and the whole fragment is wrapped in outer double marks. Text that
	// carries STRAIGHT double quotes is out-of-domain — scripture (WEB/BSB) is always
	// curly, and a straight " may be an inch or ditto mark (5'10") rather than a
	// quotation — so it is left verbatim rather than risk mis-nesting it.
	switch {
	case strings.ContainsAny(text, "“”"):
		return "“" + closeInnerSingles(nestInlineQuotes(text)) + "”"
	case strings.Contains(text, "\""):
		return text
	default:
		return "“" + closeInnerSingles(text) + "”"
	}
}

// closeInnerSingles closes an inner SINGLE quotation (Rule 5.1(b): a quote within
// a quote takes single marks, and marks must be matched) whose closer sits in the
// unselected text — the single-mark twin of balanceQuoteMarks' trailing repair.
// Scripture quotes an embedded speaker's citation in single marks that can span
// several verses (Acts 2:25 opens David's psalm ‘I saw the Lord…’, which does not
// close until v28), so a selection of just the opening verse carries an OPENING
// single with no partner — the app used to wrap it in outer doubles and ship the
// dangling ‘ (“…: ‘I saw … shaken.”), an unbalanced-marks violation.
//
// It appends one closing ’ per unmatched opener and is apostrophe-SAFE by
// construction: only the OPENING glyph ‘ (U+2018) is counted as an opener — it is
// never an apostrophe — while EVERY ’ (U+2019) is counted as a potential closer,
// including apostrophes (God’s, can’t). So the append count (opens − all ’) can
// never exceed the number of genuinely-unclosed openers: it adds a mark only to
// close a real dangling citation and never a spurious one. The cost is a rare
// under-close (an opener followed by a curly apostrophe before the cut, e.g.
// ‘God’s people…), which merely preserves the prior behaviour — no regression.
// Runs only on the inline branches, after nestInlineQuotes, so a balanced pair
// derived from source double marks contributes 0 and is never double-closed; a
// block quotation reproduces the source's marks verbatim and is left untouched.
func closeInnerSingles(s string) string {
	if n := strings.Count(s, "‘") - strings.Count(s, "’"); n > 0 {
		return s + strings.Repeat("’", n)
	}
	return s
}

// nestInlineQuotes drops a verse's own DOUBLE quotation marks one nesting level
// (“ ” → ‘ ’) so that, once the fragment is wrapped in outer double marks, its internal
// quotations read as single marks — Bluebook Rule 5.1(b) (a quotation within a quotation
// takes single marks). Only the unambiguous curly double glyphs are converted: the
// closing single glyph ’ doubles as the apostrophe (God’s, didn’t), so existing single
// marks are left untouched — a rare second internal level (Jesus quoting scripture,
// “… ‘…’ …”) is therefore not further alternated back to double, which a shareable
// fragment almost never needs.
func nestInlineQuotes(s string) string {
	return strings.NewReplacer("“", "‘", "”", "’").Replace(s)
}

// endOmissionEllipsis is the Bluebook Rule 5.3 spaced ellipsis — three periods, one
// space between each and one before the first. Never the single-glyph "…". In the
// four-dot form it is followed by a space and the sentence's final punctuation.
const endOmissionEllipsis = " . . ."

// addEndOmission marks an omission when the quoted text is cut off MID-SENTENCE — the
// reader's selection ends before the original sentence does, so the rest of the
// sentence is omitted. A share stands as its own sentence, so Rule 5.3(b)(iii)
// applies: insert the spaced ellipsis between the last quoted word and "the final
// punctuation of the sentence being quoted" — the four-dot form " . . . .", or
// " . . . ?" / " . . . !" when the sentence the selection was cut from ends that way
// (terminal carries it; see originalSentenceTerminal). The mark sits just inside any
// trailing closing quotation mark. A quotation already ending on sentence-terminal
// punctuation (. ! ? …) is complete and gets no mark — which also makes the function
// idempotent, since the mark itself ends on terminal punctuation. A selection that
// merely BEGINS mid-sentence takes no ellipsis either — Rule 5.3(b)(i) uses the
// bracketed capital instead (bracketStartCapital).
func addEndOmission(s string, terminal rune) string {
	trimmed := strings.TrimRight(s, " \t\n")
	core := strings.TrimRight(trimmed, " \t\n”’\"'")
	if core == "" {
		return s
	}
	switch r := []rune(core); r[len(r)-1] {
	case '.', '!', '?', '…':
		return s // a complete sentence — nothing omitted at the end
	}
	switch terminal {
	case '.', '!', '?':
	default:
		terminal = '.'
	}
	return core + endOmissionEllipsis + " " + string(terminal) + trimmed[len(core):]
}

// bracketStartCapital gives a share that BEGINS mid-sentence its Rule 5.3(b)(i)
// treatment: quoted language used as a full sentence must not open with an ellipsis;
// instead "capitalize the first letter of the quoted language and place it in
// brackets if it is not already capitalized" (the bracket disclosing the alteration
// per Rule 5.2(a)) — "raised Him from the dead" shares as "[R]aised Him from the
// dead . . . ." Scripture capitalizes every sentence opening, so a lowercase first
// letter reliably means the selection started mid-sentence. Leading quotation marks
// (including ones balanceQuoteMarks prepended) are skipped; a first letter that is
// already capital — or a digit, bracket, or other non-letter — is left verbatim.
func bracketStartCapital(s string) string {
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case r == '“' || r == '‘' || r == '"' || r == '\'':
			continue // opening quotation marks — the letter we care about is inside
		case unicode.IsLower(r):
			return string(rs[:i]) + "[" + string(unicode.ToUpper(r)) + "]" + string(rs[i+1:])
		}
		break // already capital, a digit, an existing bracket, … — verbatim
	}
	return s
}

// originalSentenceTerminal finds the punctuation that ends the sentence the
// selection stops inside, by locating the selection in the chapter's own prose and
// scanning forward — Rule 5.3(b)(iii) retains "the final punctuation of the sentence
// being quoted" in the four-dot slot, so a question cut short shares as " . . . ?".
// Falls back to a period when the selection can't be pinned in the chapter.
//
// at/baseLen are the pipeline's located coordinates (-1/0 when unlocated): the
// scan then starts after the LOCATED text, so repeated wording reads the
// selected copy's sentence — and reads it even when the completion has appended
// past a skipped quote-closer, where the string search cannot match at all.
func originalSentenceTerminal(state *AppState, sel string, at, baseLen int) rune {
	book, chapter := readerChapter(state)
	return originalSentenceTerminalIn(state, book, chapter, sel, at, baseLen)
}

func originalSentenceTerminalIn(state *AppState, book string, chapter int, sel string, at, baseLen int) rune {
	if state == nil || state.Bible == nil {
		return '.'
	}
	prose, _ := chapterProseIn(state, book, chapter)
	s := collapseSpaces(sel)
	idx, scanFrom := -1, 0
	if at >= 0 && baseLen > 0 && baseLen <= len(s) && at+baseLen <= len(prose) &&
		prose[at:at+baseLen] == s[:baseLen] {
		idx, scanFrom = at, at+baseLen
	} else if i := strings.Index(prose, s); i >= 0 {
		idx, scanFrom = i, i+len(s)
	}
	if idx < 0 {
		return '.'
	}
	for _, r := range prose[scanFrom:] {
		switch r {
		case '.', '…':
			return '.'
		case '!', '?':
			return r
		}
	}
	return '.'
}

// resolveEnclosingQuotes implements Bluebook Rule 5.2(f)(i): when the ENTIRE
// selection lies inside one quotation in the source — Jesus' speech selected with
// its opening “ but not the closing mark (which sits chapters away), or ending
// exactly on the speech's closer whose opener precedes the selection — the
// enclosing marks are OMITTED and the share is presented as a first-degree
// quotation ("only one set of quotation marks need be employed"). Detection is
// structural: unmatched marks make the selection partially-quoted; if, treating
// each unmatched closer as closing a quotation open since the selection's start,
// NO letter of the selection sits OUTSIDE a quotation, then the quotation IS the
// whole share — so its unmatched (enclosing/continuation) marks are stripped,
// while matched pairs remain as genuine inner quotations. If any narration sits
// outside (a mixed selection), everything is left for balanceQuoteMarks to
// repair instead — Rule 5.2(f)(ii): marks of a PARTIAL embedded quotation are
// retained. Before this the Beatitudes (whose continued-speech paragraphs
// re-open “ without closing) shared with a spurious doubled closer ("…you.””").
func resolveEnclosingQuotes(s string) string {
	// Pass 1: virtual depth — unmatched closers count as closing a quotation
	// that opened before the selection.
	depth, minDepth := 0, 0
	for _, r := range s {
		switch r {
		case '“':
			depth++
		case '”':
			depth--
			if depth < minDepth {
				minDepth = depth
			}
		}
	}
	lead := -minDepth
	if lead == 0 && depth == 0 {
		// Balanced marks. One more 5.2(f)(i) case hides here: the selection may
		// CARRY the quotation's own opening and closing marks ("Judge … heard.")
		// with nothing outside them — the quoted matter is still wholly a
		// quotation, so the enclosing pair is dropped the same as when the
		// marks were cut off. Applies only when ONE outermost pair spans the
		// entire selection (depth returns to zero only at the final rune);
		// anything at depth zero — narration before/after/between quotations —
		// keeps the marks for nesting.
		rs := []rune(strings.TrimSpace(s))
		if len(rs) >= 2 && rs[0] == '“' && rs[len(rs)-1] == '”' {
			d, whole := 0, true
			for i, r := range rs {
				switch r {
				case '“':
					d++
				case '”':
					d--
					if d == 0 && i != len(rs)-1 {
						whole = false
					}
				}
				if !whole {
					break
				}
			}
			if whole {
				return strings.TrimSpace(string(rs[1 : len(rs)-1]))
			}
		}
		return s // balanced with matter outside the marks — nesting handles it
	}

	// Pass 2: any letter at virtual depth 0 is narration OUTSIDE the
	// quotation(s) → the quote is only part of the selection → keep marks.
	d := lead
	for _, r := range s {
		switch {
		case r == '“':
			d++
		case r == '”':
			d--
		case d == 0 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			return s
		}
	}

	// Wholly inside one quotation: strip the unmatched marks, keep matched pairs.
	rs := []rune(s)
	drop := make(map[int]bool)
	var opens []int
	real := 0
	for i, r := range rs {
		switch r {
		case '“':
			opens = append(opens, i)
			real++
		case '”':
			if real == 0 {
				drop[i] = true // unmatched closer — the enclosing quotation's end
			} else {
				real--
				opens = opens[:len(opens)-1]
			}
		}
	}
	for _, i := range opens {
		drop[i] = true // unmatched opens — the enclosing / continuation marks
	}
	var b strings.Builder
	for i, r := range rs {
		if drop[i] {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// balanceQuoteMarks repairs unbalanced curly double-quotation marks in a shared
// fragment whose quotation is only PART of the selection (Rule 5.2(f)(ii) retains
// such marks; resolveEnclosingQuotes has already handled the wholly-enclosed
// case). Scanning left to right, every closing mark that appears with no open
// quotation in progress means the opener is in the text BEFORE the selection — so we
// prepend an opening mark; any quotation still open at the end means the closer is in
// the text AFTER the selection — so we append a closing mark. Result: a self-contained,
// balanced quotation. Examples:
//
//	“What is truth?” … told them, “I find…   (open, close, open)  -> append one ”
//	What is truth?” … told them, “I find…     (close, open)        -> prepend “, append ”
func balanceQuoteMarks(s string) string {
	depth, minDepth := 0, 0
	for _, r := range s {
		switch r {
		case '“':
			depth++
		case '”':
			depth--
			if depth < minDepth {
				minDepth = depth
			}
		}
	}
	leadOpens := -minDepth           // closing marks with no opener → opens to prepend
	trailCloses := depth + leadOpens // opening marks left unclosed → closes to append
	if leadOpens > 0 {
		s = strings.Repeat("“", leadOpens) + s
	}
	if trailCloses > 0 {
		s = s + strings.Repeat("”", trailCloses)
	}
	return s
}

// citationForSelection derives a "Book C:V" (or "…:V-W") reference for the
// selected text by matching it against the verses of the current chapter, so a
// shared selection carries an accurate citation. Falls back to "Book C" when the
// selection can't be pinned to specific verses (e.g. a partial phrase).
func citationForSelection(state *AppState, text string, span selSpan) string {
	book, chapter := readerChapter(state)
	return citationForSelectionIn(state, book, chapter, text, span)
}

func citationForSelectionIn(state *AppState, book string, ch int, text string, span selSpan) string {
	if state == nil {
		return ""
	}
	if state.Bible == nil {
		return fmt.Sprintf("%s %d", book, ch)
	}
	// A valid span resolves the verses positionally; without one, selectionVerses
	// places the selection as the normalize does and, where that declines,
	// matches in BOTH directions (the selection contains a verse, or a verse
	// contains the selection) so a partial selection — e.g. one that omits the
	// verse's leading quotation mark — still pins to the verse it falls within,
	// rather than dropping to the chapter-only fallback.
	matched := selectionVersesIn(state, book, ch, text, span)
	if len(matched) == 0 {
		return fmt.Sprintf("%s %d", book, ch)
	}
	return verseRangeCitation(book, ch, matched[0].Verse, matched[len(matched)-1].Verse)
}
