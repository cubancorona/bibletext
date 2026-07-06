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
)

const (
	selActionShareCite  = "share-cite"
	selActionShareImage = "share-image"
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
	case selActionCrossRef:
		showCrossRefs(state, text)
	}
}

// shareVerse formats the selection in Bluebook style (see formatBibleQuote /
// citationForSelection) and hands it to the native share sheet, as text or a
// rendered image. The translation is spelled OUT in the parenthetical, not given as
// an initialism — the Bluebook always names the version in full (e.g. "(King
// James)"), so we use "(World English Bible)" / "(Berean Standard Bible)".
func shareVerse(state *AppState, text string, asImage bool) {
	cite := citationForSelection(state, text)
	version := state.currentVersion().Name
	cleaned := cleanQuoteText(state, text)
	quote := formatBibleQuote(cleaned, originalSentenceTerminal(state, cleaned))
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

// composeShareText builds the plain-text share: the already-formatted quote, a line
// break, then the citation line.
func composeShareText(quote, cite, version string) string {
	return quote + "\n" + citationLine(cite, version)
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
// Documented deviations (deliberate, for a chat/card medium): the original's
// paragraph breaks are flattened in 50+ word blocks (Rule 5.1(a)(iii) would keep
// them); a third nesting level is not re-alternated back to double (5.1(b)(i)) —
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
	text = addEndOmission(text, term)
	text = balanceQuoteMarks(text)
	text = bracketStartCapital(text)
	if len(strings.Fields(text)) >= blockQuoteWords {
		return text // block quotation: reproduce the source's marks, no outer marks
	}
	// Inline: a verse's own CURLY double quotations are nested down to single marks
	// (Rule 5.1(b)) and the whole fragment is wrapped in outer double marks. Text that
	// carries STRAIGHT double quotes is out-of-domain — scripture (WEB/BSB) is always
	// curly, and a straight " may be an inch or ditto mark (5'10") rather than a
	// quotation — so it is left verbatim rather than risk mis-nesting it.
	switch {
	case strings.ContainsAny(text, "“”"):
		return "“" + nestInlineQuotes(text) + "”"
	case strings.Contains(text, "\""):
		return text
	default:
		return "“" + text + "”"
	}
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

// balanceQuoteMarks repairs unbalanced curly double-quotation marks in a shared
// fragment. Scanning left to right, every closing mark that appears with no open
// quotation in progress means the opener is in the text BEFORE the selection — so we
// prepend an opening mark; any quotation still open at the end means the closer is in
// the text AFTER the selection — so we append a closing mark. Result: a self-contained,
// balanced quotation. Examples:
//
//	“What is truth?” … told them, “I find…   (open, close, open)  -> append one ”
//	What is truth?” … told them, “I find…     (close, open)        -> prepend “, append ”
//	“Blessed are the poor in spirit…           (open, no close)    -> append one ”
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
	book, ch := state.CurrentBook, state.CurrentChapter
	if state == nil || state.Bible == nil {
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
