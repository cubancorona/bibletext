package bibletext

// THE BOOKS GRID'S GROUPING, and why the split is found rather than counted.
//
// The books tab was 66 rows of 44pt — about 2,900pt of scrolling to reach
// Revelation, each row spending the pane's whole width on one short word

// with testament headings, which puts the whole canon on one screen.
//
// The heading placement is the part that can silently go wrong, so it is the
// part that is tested: a hardcoded "39" would put the New Testament label in
// the middle of the Catholic canon's histories, and nothing on screen would say
// so — the books would all still be there, under the wrong heading.

import "testing"

func TestBookSectionsSplitAtMatthewNotAtACount(t *testing.T) {
	// The 73-book canon: the deuterocanon pushes Matthew well past index 39, so
	// a count-based split would land inside the Old Testament.
	catholic := append([]string{}, "Genesis", "Exodus", "Tobit", "Judith", "Wisdom",
		"Sirach", "Baruch", "1 Maccabees", "2 Maccabees", "Malachi", "Matthew", "Mark", "Revelation")

	secs := bookSections(catholic, catholic, false)
	if len(secs) != 2 {
		t.Fatalf("got %d sections, want 2 (Old and New Testament)", len(secs))
	}
	if secs[0].title != "OLD TESTAMENT" || secs[1].title != "NEW TESTAMENT" {
		t.Errorf("sections are titled %q and %q", secs[0].title, secs[1].title)
	}
	if last := secs[0].books[len(secs[0].books)-1]; last != "Malachi" {
		t.Errorf("the Old Testament ends at %q, want Malachi — the split counted books "+
			"instead of finding Matthew, so the deuterocanon moved the heading", last)
	}
	if secs[1].books[0] != "Matthew" {
		t.Errorf("the New Testament starts at %q, want Matthew", secs[1].books[0])
	}
	// Nothing may be lost between the two runs.
	if n := len(secs[0].books) + len(secs[1].books); n != len(catholic) {
		t.Errorf("the sections hold %d books of %d — the grid would silently omit some", n, len(catholic))
	}
}

// A FILTERED SET IS A RESULT, NOT THE CANON. Headings over three matches would
// be furniture pretending to be structure, and "OLD TESTAMENT" above a lone
// John would be actively wrong.
func TestBookSectionsDropTheHeadingsWhileFiltering(t *testing.T) {
	all := []string{"Genesis", "Malachi", "Matthew", "John", "Revelation"}
	hits := []string{"John"}

	secs := bookSections(all, hits, true)
	if len(secs) != 1 {
		t.Fatalf("a filtered set produced %d sections, want 1 unheaded run", len(secs))
	}
	if secs[0].title != "" {
		t.Errorf("the filtered run is headed %q — a result set has no testaments", secs[0].title)
	}
	if len(secs[0].books) != 1 || secs[0].books[0] != "John" {
		t.Errorf("the filtered run holds %v, want just the matches", secs[0].books)
	}
}

// AND A CANON WITH NO MATTHEW DEGRADES HONESTLY: one unlabelled run, rather
// than a heading placed by guesswork. Not reachable with today's translations —
// which is exactly why it is worth pinning before one arrives.
func TestBookSectionsWithNoMatthewAreUnheaded(t *testing.T) {
	only := []string{"Genesis", "Exodus", "Leviticus"}
	secs := bookSections(only, only, false)
	if len(secs) != 1 || secs[0].title != "" {
		t.Errorf("got %d sections titled %q, want one unheaded run", len(secs), secs[0].title)
	}
	if len(secs[0].books) != len(only) {
		t.Errorf("the run holds %d books of %d", len(secs[0].books), len(only))
	}
}
