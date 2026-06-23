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

// shareVerse formats the selection with a citation and hands it to the native
// share sheet, as text or as a rendered image. The quote is cleaned to proper
// Bible-quotation form first (verse numbers removed, quotation marks handled).
func shareVerse(state *AppState, text string, asImage bool) {
	cite := citationForSelection(state, text)
	abbrev := state.currentVersion().Abbrev
	quote := formatBibleQuote(cleanQuoteText(state, text))
	if asImage {
		if path, err := renderVerseImage(state, quote, cite, abbrev); err == nil {
			nativeShareImage(path)
			return
		}
		// If image rendering fails for any reason, share the text instead.
	}
	nativeShareText(fmt.Sprintf("%s\n— %s (%s)", quote, cite, abbrev))
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

// formatBibleQuote applies sensible quotation conventions to a clean verse string:
//   - strips an orphan opening or closing curly double-quote left over from quoting
//     a fragment of a longer quotation (e.g. a single Beatitude verse opens a quote
//     that only closes several verses later), so the share doesn't start/end on a
//     dangling mark;
//   - adds decorative outer quotation marks ONLY when the passage has no double
//     quotes of its own — otherwise the nesting (a quote within the quote) reads as
//     broken. Dialogue already in the text is left exactly as printed, and the
//     citation line marks the whole thing as a quotation regardless.
func formatBibleQuote(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	opens := strings.Count(text, "“")
	closes := strings.Count(text, "”")
	if opens > closes && strings.HasPrefix(text, "“") {
		text = strings.TrimSpace(text[len("“"):])
		opens--
	}
	if closes > opens && strings.HasSuffix(text, "”") {
		text = strings.TrimSpace(text[:len(text)-len("”")])
		closes--
	}
	if opens == 0 && closes == 0 && !strings.ContainsRune(text, '"') {
		return "“" + text + "”"
	}
	return text
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
		return fmt.Sprintf("%s %d:%d-%d", book, ch, lo, hi)
	}
}
