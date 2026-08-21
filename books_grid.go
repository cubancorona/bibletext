package bibletext

// The canon's shape, as the books grid needs it.
//
// UNTAGGED ON PURPOSE. This is arithmetic about a book list — no widgets, no
// platform — and it started life inside the iOS/Android UI file, where the host
// test could not reach it and the desktop sidebar could not reuse it. Keeping
// the pure part separate from the drawing is what lets one rule about the canon
// serve every surface that ever wants to group it.

// The book cell's size: wide enough for "Song of Solomon" at the button's own
// text size, tall enough for Apple's 44pt touch target.
const (
	bookCellW float32 = 168
	bookCellH float32 = 44
	// Five columns' worth. Wider than an iPad in this layout needs, so the grid
	// is bounded by the pane there and by this on a large desktop window.
	bookGridMaxWidth float32 = 900
)

// bookSection is one headed run of the canon.
type bookSection struct {
	title string // "" when the set is a filter result rather than the canon
	books []string
}

// bookSections splits the books into testaments, or returns one unheaded run
// when the reader is filtering.
//
// The split is found by locating Matthew in the translation's OWN order rather
// than by counting 39 books, because the counts differ: the Catholic canon has
// 73, and a count would put its heading in the wrong place. A translation with
// no Matthew (never today) returns a single unheaded run, which is the honest
// answer rather than a guess.
func bookSections(all, filtered []string, filtering bool) []bookSection {
	if filtering {
		return []bookSection{{books: filtered}}
	}
	split := -1
	for i, b := range all {
		if b == "Matthew" {
			split = i
			break
		}
	}
	if split <= 0 || split >= len(all) {
		return []bookSection{{books: filtered}}
	}
	return []bookSection{
		{title: "OLD TESTAMENT", books: all[:split]},
		{title: "NEW TESTAMENT", books: all[split:]},
	}
}
