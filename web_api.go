package bibletext

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

// IsWordsOfChrist reports whether a verse falls in a red-letter range.
func IsWordsOfChrist(book string, chapter, verse int) bool {
	return isWordsOfChrist(book, chapter, verse)
}
