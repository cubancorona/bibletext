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
	quote := formatBibleQuote(cleanQuoteText(state, text))
	if asImage {
		// Don't share blind: show the rendered card for review (with Regenerate)
		// and only hand it to the OS share sheet once the reader taps Share.
		showShareImagePreview(state, quote, cite, version)
		return
	}
	nativeShareText(fmt.Sprintf("%s\n— %s (%s)", quote, cite, version))
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

// blockQuoteWords is the Bluebook Rule 5 threshold: a quotation of 50 or more words
// is set off as a block quotation rather than run inline in quotation marks.
const blockQuoteWords = 50

// formatBibleQuote prepares a clean verse string for sharing, in Bluebook style. It
// is deliberately FAITHFUL to the selection: it never removes or alters the verse's
// own quotation marks. A verse may legitimately open or close a longer quotation —
// e.g. Matthew 5:3 opens the Beatitudes, and John 18:38 reads «“What is truth?” …
// told them, “I find…» (two opens, one close) — and the reader may select those
// marks on purpose; dropping any would misquote the text.
//
// Quotation marks are handled per Bluebook Rule 5:
//   - 50+ words → a BLOCK quotation: set off WITHOUT surrounding quotation marks
//     (the set-off itself, plus the citation line, marks it as a quotation; the
//     image card's centered, wide-margined block is the faithful analog of the
//     "indented both sides" block form).
//   - under 50 words → an INLINE quotation: add outer double quotes — but only when
//     the verse has no double quotes of its own, so dialogue isn't wrapped into
//     broken nesting.
func formatBibleQuote(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if len(strings.Fields(text)) >= blockQuoteWords {
		return text // block quotation: no surrounding quotation marks
	}
	// Inline: has its own double quotes (curly or straight)? Leave them as selected.
	if strings.ContainsAny(text, "“”\"") {
		return text
	}
	return "“" + text + "”"
}

// citationForSelection derives a "Book C:V" (or "…:V-W") reference for the
// selected text by matching it against the verses of the current chapter, so a
// shared selection carries an accurate citation. Falls back to "Book C" when the
// selection can't be pinned to specific verses (e.g. a partial phrase).
func citationForSelection(state *AppState, text string) string {
	book, ch := state.CurrentBook, state.CurrentChapter
	if state.Bible == nil {
		return fmt.Sprintf("%s %d", book, ch)
	}
	norm := collapseSpaces(text)
	lo, hi := 0, 0
	for _, v := range state.Bible.GetChapter(book, ch) {
		vt := collapseSpaces(v.Text)
		if len([]rune(vt)) < 8 {
			continue
		}
		probe := vt
		if r := []rune(vt); len(r) > 24 {
			probe = string(r[:24])
		}
		if strings.Contains(norm, probe) {
			if lo == 0 {
				lo = v.Verse
			}
			hi = v.Verse
		}
	}
	switch {
	case lo == 0:
		return fmt.Sprintf("%s %d", book, ch)
	case lo == hi:
		return fmt.Sprintf("%s %d:%d", book, ch, lo)
	default:
		// Bluebook uses an en dash (not a hyphen) for a span of verses.
		return fmt.Sprintf("%s %d:%d–%d", book, ch, lo, hi)
	}
}
