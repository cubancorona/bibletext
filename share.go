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
)

const (
	selActionShareCite  = "share-cite"
	selActionShareImage = "share-image"
	selActionShareLink  = "share-link"
	selActionShareNote  = "share-link-note"
	selActionCrossRef   = "crossref"
)

// dispatchSelectionAction routes a non-AI selection-menu action from the native
// callback (already on the Fyne UI goroutine).
func dispatchSelectionAction(state *AppState, action, text string) {
	if state == nil {
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	switch action {
	case selActionShareCite:
		shareVerse(state, text, false)
	case selActionShareImage:
		shareVerse(state, text, true)
	case selActionShareLink:
		shareVerseLink(state, text)
	case selActionShareNote:
		promptShareNote(state, text)
	case selActionCrossRef:
		showCrossRefs(state, text)
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
// note produces exactly the link shareVerseLink would have, so the two paths
// cannot drift apart.
func shareVerseLinkWithNote(state *AppState, text, note string) {
	_, cite := prepareShareQuote(state, text)
	version := state.currentVersion()
	lo, hi := 0, 0
	if _, l, h, ok := normalizeShareSelection(state, text); ok {
		lo, hi = l, h
	}
	url := ShareLinkURLWithNote(version.ID, state.CurrentBook, state.CurrentChapter, lo, hi, note)
	if url == "" {
		shareVerse(state, text, false)
		return
	}
	// And keep it. Until now your own words vanished with the share sheet and
	// survived only in whatever messenger you sent them through — which meant
	// the app could show you every note you had RECEIVED and none you had sent.
	// Stored as a Kind=mine record in the scrapbook store, never drawn in the text,
	// and visible in the notes browser (owner directive).
	if n := strings.TrimSpace(note); n != "" {
		saveMyNote(appPrefs(), StoredNote{
			VersionID: version.ID,
			Book:      state.CurrentBook,
			Chapter:   state.CurrentChapter,
			VerseLo:   lo,
			VerseHi:   hi,
			Text:      n,
		})
	}

	// The note goes in the MESSAGE too, not only inside the link. It is how
	// people share things anyway, it reaches a recipient who never taps, and it
	// reaches one whose app is too old to read the note out of the fragment.
	msg := cite + " (" + version.Name + ")\n" + url
	if n := strings.TrimSpace(note); n != "" {
		msg = n + "\n\n" + msg
	}
	nativeShareText(msg)
}

func shareVerseLink(state *AppState, text string) {
	_, cite := prepareShareQuote(state, text)
	version := state.currentVersion()
	lo, hi := 0, 0
	if _, l, h, ok := normalizeShareSelection(state, text); ok {
		lo, hi = l, h
	}
	url := ShareLinkURL(version.ID, state.CurrentBook, state.CurrentChapter, lo, hi)
	if url == "" {
		shareVerse(state, text, false) // no link possible — share the words instead
		return
	}
	// Citation first, then the URL on its own line: messengers unfurl a link on
	// its own line, and the reference still reads if the preview doesn't render.
	nativeShareText(cite + " (" + version.Name + ")\n" + url)
}

// shareVerse formats the selection in Bluebook style (see formatBibleQuote /
// citationForSelection) and hands it to the native share sheet, as text or a
// rendered image. The translation is spelled OUT in the parenthetical, not given as
// an initialism — the Bluebook always names the version in full (e.g. "(King
// James)"), so we use "(World English Bible)" / "(Berean Standard Bible)".
func shareVerse(state *AppState, text string, asImage bool) {
	cleaned, cite := prepareShareQuote(state, text)
	version := state.currentVersion().Name
	terminal := originalSentenceTerminal(state, cleaned)
	// BOTH shares retain authored structure: source poetry lines and the reading
	// view's paragraph boundaries. They are rebuilt from the chapter data, never
	// copied from rendered wrapping, so resizing the reader cannot change what is
	// shared. prepareShareQuote collapses every "\n" out of the selection, so
	// without this the card could not break a psalm even though its renderer
	// knows how (poemSegments) — a card is the one surface where a flattened
	// psalm is most conspicuous.
	cleaned = restoreShareLineBreaks(state, cleaned)
	quote := formatBibleQuote(cleaned, terminal)
	if asImage {
		// Don't share blind: show the rendered card for review (with Regenerate)
		// and only hand it to the OS share sheet once the reader taps Share.
		showShareImagePreview(state, quote, cite, version)
		return
	}
	nativeShareText(composeShareText(quote, cite, version))
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
	s := collapseSpaces(raw)
	if state == nil || state.Bible == nil {
		return s
	}
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
		body := collapseSpaces(v.Text)
		if body == "" {
			continue
		}
		probe := firstRunes(body, 12)
		marker := strconv.Itoa(v.Verse) + " " + probe
		s = strings.ReplaceAll(s, marker, probe)
	}
	return strings.TrimSpace(s)
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
// and is dropped with it. Falls back to the legacy probe-based path whenever
// the selection cannot be located in the chapter's prose.
func prepareShareQuote(state *AppState, raw string) (text, cite string) {
	if t, lo, hi, ok := normalizeShareSelection(state, raw); ok {
		return completeTrailingSentence(state, t), verseRangeCitation(state, lo, hi)
	}
	cleaned := completeTrailingSentence(state, cleanQuoteText(state, raw))
	return cleaned, citationForSelection(state, raw)
}

// chapterProse joins the current chapter's verses (marker-free, spaces
// collapsed) into one string, recording each verse's [start,end) span — the
// positional ground truth for locating a selection and attributing verses.
type verseSpan struct{ verse, start, end int }

func chapterProse(state *AppState) (string, []verseSpan) {
	var b strings.Builder
	var spans []verseSpan
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
		t := collapseSpaces(v.Text)
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
	if state == nil || state.Bible == nil {
		return "", nil
	}
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	var b strings.Builder
	var breaks []shareTextBreak
	prevPoetic := false
	for paragraphIndex, paragraph := range groupVersesIntoParagraphs(verses) {
		wroteInParagraph := false
		for _, verse := range paragraph {
			text, verseBreaks := verseShareStructure(verse.Text)
			if text == "" {
				continue
			}
			// A join touching a poetic verse restores as "\n" (a paragraph
			// break still wins with "\n\n") — the same verseIsPoetic rule the
			// reading pane renders with, so displayed lines and shared lines
			// stay identical.
			curPoetic := verseIsPoetic(verse.Text)
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
// inside the selected text. The selection is first located in the flattened
// chapter, so soft wrapping from any platform is neither inspected nor retained.
func restoreShareLineBreaks(state *AppState, text string) string {
	flat, breaks := chapterShareStructure(state)
	if text == "" || flat == "" || len(breaks) == 0 {
		return text
	}
	start := strings.Index(flat, text)
	if start < 0 {
		return text
	}
	end := start + len(text)

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

// normalizeShareSelection cleans a raw selection for sharing and attributes it
// to its verse range positionally. ok=false when the selection can't be pinned
// in the chapter (caller falls back to the legacy path).
func normalizeShareSelection(state *AppState, raw string) (text string, lo, hi int, ok bool) {
	if state == nil || state.Bible == nil {
		return "", 0, 0, false
	}
	s := stripVerseMarkers(state, collapseSpaces(raw))
	corpus, spans := chapterProse(state)
	if s == "" || corpus == "" {
		return "", 0, 0, false
	}

	// A trailing bare verse-number token (the selection stopped right after a
	// superscript marker) never appears in the prose corpus — drop it so the
	// locate below can succeed. Verified against the chapter's real verse
	// numbers so a legitimate number inside verse text is never touched (that
	// one locates fine WITH the number, so this branch is skipped).
	validNum := make(map[string]bool, len(spans))
	for _, sp := range spans {
		validNum[strconv.Itoa(sp.verse)] = true
	}
	for strings.Index(corpus, s) < 0 {
		i := strings.LastIndexByte(strings.TrimSpace(s), ' ')
		last := strings.TrimSpace(s)
		if i >= 0 {
			last = last[i+1:]
		}
		if !validNum[last] {
			break
		}
		if i < 0 {
			return "", 0, 0, false // the selection was only the number
		}
		s = strings.TrimSpace(strings.TrimSpace(s)[:i])
	}

	idx := strings.Index(corpus, s)
	if idx < 0 {
		return "", 0, 0, false
	}

	// Mid-word END repair: if the character right after the selection is a
	// word rune, the drag stopped inside a word — trim back to the last whole
	// word (and re-drop a verse-number token left dangling by that trim).
	for {
		end := idx + len(s)
		if end < len(corpus) {
			if r, _ := utf8.DecodeRuneInString(corpus[end:]); isWordRune(r) {
				cut := strings.LastIndexByte(s, ' ')
				if cut <= 0 {
					return "", 0, 0, false // a single partial word — nothing left
				}
				s = strings.TrimSpace(s[:cut])
				for {
					j := strings.LastIndexByte(s, ' ')
					last := s
					if j >= 0 {
						last = s[j+1:]
					}
					if !validNum[last] || strings.Index(corpus, s) >= 0 {
						break
					}
					if j < 0 {
						return "", 0, 0, false
					}
					s = strings.TrimSpace(s[:j])
				}
				idx = strings.Index(corpus, s)
				if idx < 0 {
					return "", 0, 0, false
				}
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
			s = strings.TrimSpace(s[size:])
			idx = strings.Index(corpus, s)
			if idx < 0 || s == "" {
				return "", 0, 0, false
			}
			continue
		}
		break
	}
	for s != "" {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == ',' || r == ';' || r == ':' || r == '—' {
			s = strings.TrimSpace(s[:len(s)-size])
			if s == "" {
				return "", 0, 0, false
			}
			idx = strings.Index(corpus, s)
			if idx < 0 {
				return "", 0, 0, false
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
			return "", 0, 0, false
		}
		s = strings.TrimSpace(s[cut+1:])
		if s == "" {
			return "", 0, 0, false
		}
		idx = strings.Index(corpus, s)
		if idx < 0 {
			return "", 0, 0, false
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
		return "", 0, 0, false
	}
	return s, lo, hi, true
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
func completeTrailingSentence(state *AppState, s string) string {
	if state == nil || state.Bible == nil || s == "" {
		return s
	}
	if r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(s, " \t”’\"'")); r == '.' || r == '!' || r == '?' || r == '…' {
		return s // already complete
	}
	corpus, _ := chapterProse(state)
	idx := strings.Index(corpus, s)
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
func stripVerseMarkers(state *AppState, s string) string {
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
		body := collapseSpaces(v.Text)
		if body == "" {
			continue
		}
		marker := strconv.Itoa(v.Verse) + " "
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
				from = i
				continue
			}
			from = i + len(marker)
		}
	}
	return strings.TrimSpace(s)
}

// verseRangeCitation renders "Book C:lo" / "Book C:lo–hi" (en dash per Bluebook).
func verseRangeCitation(state *AppState, lo, hi int) string {
	book, ch := state.CurrentBook, state.CurrentChapter
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
// Documented deviations (deliberate, for a chat/card medium): paragraph/line
// structure is preserved only on the TEXT share path (restoreShareLineBreaks) —
// the image card still flattens (Rule 5.1(a)(iii) would keep it in 50+ word
// blocks); a third nesting
// level is not re-alternated back to double (5.1(b)(i)) —
// the closing single glyph ’ doubles as the apostrophe, so auto-flipping is
// unreliable; and the card centers its citation rather than starting it at the
// left margin on the following line.
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
func originalSentenceTerminal(state *AppState, sel string) rune {
	if state == nil || state.Bible == nil {
		return '.'
	}
	var b strings.Builder
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
		t := collapseSpaces(v.Text)
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t)
	}
	chapter := b.String()
	s := collapseSpaces(sel)
	idx := strings.Index(chapter, s)
	if idx < 0 {
		return '.'
	}
	for _, r := range chapter[idx+len(s):] {
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
func citationForSelection(state *AppState, text string) string {
	if state == nil {
		return ""
	}
	book, ch := state.CurrentBook, state.CurrentChapter
	if state.Bible == nil {
		return fmt.Sprintf("%s %d", book, ch)
	}
	// selectionVerses matches in BOTH directions (the selection contains a verse, or a
	// verse contains the selection) so a partial selection — e.g. one that omits the
	// verse's leading quotation mark — still pins to the verse it falls within, rather
	// than dropping to the chapter-only fallback.
	matched := selectionVerses(state, text)
	if len(matched) == 0 {
		return fmt.Sprintf("%s %d", book, ch)
	}
	lo, hi := matched[0].Verse, matched[len(matched)-1].Verse
	switch {
	case lo == hi:
		return fmt.Sprintf("%s %d:%d", book, ch, lo)
	default:
		// Bluebook uses an en dash (not a hyphen) for a span of verses.
		return fmt.Sprintf("%s %d:%d–%d", book, ch, lo, hi)
	}
}
