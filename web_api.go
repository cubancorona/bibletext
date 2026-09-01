package bibletext

import "image/color"

// Exported seams for cmd/websitegen — the static web reader's generator.
//
// The generator lives in this module so it can render from the SAME decoders,
// poem-line rule and red-letter data the app renders from; these thin wrappers
// are the only surface it needs. Keeping them here (rather than exporting the
// internals directly) makes the site's dependency on the package explicit and
// small: if one of these signatures has to change, the blast radius is one
// program, not the whole codebase.

// DecodeCanonical66 decodes a 66-book helloao complete.json (WEB, BSB).
func DecodeCanonical66(body []byte) (*BibleData, error) { return decodeCanonical66(body) }

// DecodeHelloAOCatholic decodes the 73-book WEB Catholic complete.json.
func DecodeHelloAOCatholic(body []byte) (*BibleData, error) { return decodeHelloAOCatholic(body) }

// VerseIsPoetic reports whether a verse carries authored poem-line breaks.
func VerseIsPoetic(text string) bool { return verseIsPoetic(text) }

// PoeticJoin reports whether the boundary between two adjacent verses is a
// poetry line boundary — the rule that keeps the web page, the reading pane and
// a shared quote breaking in identical places.
func PoeticJoin(prevText, curText string) bool { return poeticJoin(prevText, curText) }

// GroupVersesIntoParagraphs applies the app's paragraph rule (break after a
// sentence-ending verse once a paragraph is long enough). Exported so the web
// page breaks paragraphs in exactly the same places the reading pane does.
func GroupVersesIntoParagraphs(verses []Verse) [][]Verse {
	return groupVersesIntoParagraphs(verses)
}

// IsWordsOfChrist reports whether a verse falls in a red-letter range.
//
// DEPRECATED FOR RENDERING. This is the WEB's verse-level judgement and it is
// version-blind, so a page rendered through it reddens the WHOLE verse and
// reddens it identically in every translation. Use RedLetterRuns instead, which
// asks the edition's own table. Kept because it is still the honest answer to
// the question it actually asks ("does Christ speak in this verse at all"), and
// removing an exported symbol is not free.
func IsWordsOfChrist(book string, chapter, verse int) bool {
	return isWordsOfChrist(book, chapter, verse)
}

// TextRun is a stretch of a verse that is uniformly Christ's words or is not —
// the exported shape of what every reading pane walks to colour a verse.
type TextRun struct {
	Text string
	Red  bool
}

// RedLetterRuns splits one verse of one translation into its red and not-red
// stretches, using that translation's OWN span table.
//
// It exists because the web reader was the fifth rendering surface and the only
// one left behind when per-edition spans landed: it asked IsWordsOfChrist, which
// answers for the WEB and answers per VERSE. Two things were wrong on the page
// as a result. Mixed-speaker verses came out entirely red — John 4:9 put the
// Samaritan woman's words in Christ's colour — and every translation was
// rendered with the WEB's marks, so BSB and NKJV pages could not preserve their
// publishers' different editorial decisions.
//
// versionID is the translation being rendered ("web", "bsb", "webc", "nkjv").
// An unknown id returns black-letter runs. Red-letter decisions are editorial,
// so no edition is allowed to inherit another translation's marks.
//
// The returned runs concatenate back to the verse's text exactly, so a caller
// can escape and emit each in turn without having to reason about offsets.
//
// trimRuns is applied here, not left to the caller, because every pane in the
// app applies it (reading.go, android_chapter_html.go,
// reading_mobile_segments.go) and the page must not be the one surface that
// does not. It trims the verse AS A WHOLE — the old web code called
// strings.TrimSpace on the verse text, and trimming per run instead would glue
// "he said" onto the quotation beside it.
func RedLetterRuns(versionID string, v Verse) []TextRun {
	runs := trimRuns(redLetterRuns(versionID, v, true))
	out := make([]TextRun, 0, len(runs))
	for _, r := range runs {
		out = append(out, TextRun{Text: r.Text, Red: r.Red})
	}
	return out
}

// AlphabeticalBooks orders books the way the app's "Go to" picker does — a
// leading numeral read as an ordinal, so 1/2/3 John group under John.
func AlphabeticalBooks(books []string) []string { return alphabeticalBooks(books) }

// FirstLetter is the letter a book is filed under in that picker's alphabet
// grid ("1 John" → "J"). Exported with AlphabeticalBooks so the web picker
// groups and orders identically to the app rather than re-deriving the rule in
// JavaScript, where "1 John" would file under "1".
func FirstLetter(book string) string { return string(firstLetter(book)) }

// HighlightTintClass is the CSS class a highlighted verse carries, from the app's
// own tint table (verseTint.htmlClass, tint.go).
//
// The web reader is the ONE surface that cannot consume chapterTint: its pages
// are static and the tint is chosen at read time, in reader.js, from the URL
// fragment. What it can share is the VOCABULARY — and it must, because a shared
// link opened in the browser and the same link opened in the app are meant to
// light the same verses the same way. Exported so cmd/websitegen's tests can
// assert its hand-written CSS and JS still spell the class the way the app's
// emitter does; adding a second tint means adding a name here and a rule there,
// and the test is what says so out loud instead of the site quietly rendering
// the old single wash.
func HighlightTintClass() string { return tintHighlight.htmlClass() }

// WebReaderPalette is the subset of the app palette used by the static reader.
// Its translucent control colours reuse the Fyne theme's hover and selection
// tints, while the separate opaque highlight remains reserved for scripture.
type WebReaderPalette struct {
	Background       color.NRGBA
	Surface          color.NRGBA
	Text             color.NRGBA
	TextMuted        color.NRGBA
	Accent           color.NRGBA
	Border           color.NRGBA
	VerseNumber      color.NRGBA
	RedLetter        color.NRGBA
	Highlight        color.NRGBA
	ControlHover     color.NRGBA
	ControlSelection color.NRGBA
}

// WebReaderPalettes returns the light and dark static-reader palettes. Keeping
// this seam beside the decoder and tint exports lets cmd/websitegen derive its
// CSS from the app palette instead of maintaining another set of colour values.
func WebReaderPalettes() (light, dark WebReaderPalette) {
	return webReaderPalette(lightPalette), webReaderPalette(darkPalette)
}

func webReaderPalette(p palette) WebReaderPalette {
	return WebReaderPalette{
		Background:       p.Background,
		Surface:          p.Surface,
		Text:             p.Text,
		TextMuted:        p.TextMuted,
		Accent:           p.Accent,
		Border:           p.Border,
		VerseNumber:      p.VerseNumber,
		RedLetter:        p.RedLetter,
		Highlight:        p.Highlight,
		ControlHover:     paletteHover(p),
		ControlSelection: paletteSelection(p),
	}
}

// WebNoteByline is the shared byline for a received note, exactly as the app
// panes attribute one (senderByline). Sender names do not ship, so this is a
// generate-time constant for the static reader; the day names ship,
// TestWebReaderNoteChromeComesFromTheSharedFunctions holds the seam and the
// template learns names rather than silently keeping the constant.
func WebNoteByline() string { return senderByline(StoredNote{Kind: noteKindReceived}) }

// WebNotePillLabel is the shared collapsed-pill label for the web reader's
// structural case — one placed note, nothing unplaced — from the same
// function every pane's pill reads (stickerPillWho).
func WebNotePillLabel() string { return stickerPillWho(1, 0) }

// WebNoteArrivalLeadPx is the shared arrival lead (noteMetrics().Lead): how
// far below the top of the viewport an arrival places its target.
func WebNoteArrivalLeadPx() int { return int(noteMetrics().Lead) }
