package bibletext

// Browsing the notes people have sent you.
//
// WHY IT LIVES ON THE SEARCH TAB. A note is a message about a passage, so the
// only useful thing to do with one — besides read it — is go to the passage. The
// Search tab already is the "find something and tap through to it" surface: it
// owns the field, the results list, and rows that navigate. Notes are a third
// mode beside Search and Find rather than a place of their own, so a note row
// behaves exactly like a search hit and the desktop/iPad sidebar gets it for
// free (both surfaces render through buildSearchResultsView).
//
// It is NOT in Settings. Settings is configuration; these are somebody's
// messages, and a list that grows to 200 entries has no business in a sheet
// whose height is already the thing we had to fix.

import (
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// noteReference is the passage a note is attached to, as a reader would write
// it: "John 11:35", "Psalms 23:1-4", or just "Psalms 23" for a whole chapter.
func noteReference(n SharedNote) string {
	ref := n.Book + " " + strconv.Itoa(n.Chapter)
	switch {
	case n.VerseLo > 0 && n.VerseHi > n.VerseLo:
		ref += ":" + strconv.Itoa(n.VerseLo) + "-" + strconv.Itoa(n.VerseHi)
	case n.VerseLo > 0:
		ref += ":" + strconv.Itoa(n.VerseLo)
	}
	return ref
}

// sortedNotes returns the stored notes in canonical reading order — Genesis
// first, Revelation last — rather than the alphabetical-by-key order the blob
// happens to be written in. bookOrder maps a book name to its position in the
// loaded canon; anything the current translation does not contain (a note taken
// in the Catholic canon, read back under a 66-book one) sorts to the end, still
// visible, still openable once that translation is picked again.
func sortedNotes(notes map[string]SharedNote, bookOrder map[string]int) []SharedNote {
	out := make([]SharedNote, 0, len(notes))
	for _, n := range notes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool {
		bi, oki := bookOrder[out[i].Book]
		bj, okj := bookOrder[out[j].Book]
		if oki != okj {
			return oki // known books before unknown ones
		}
		if !oki {
			if out[i].Book != out[j].Book {
				return out[i].Book < out[j].Book
			}
		} else if bi != bj {
			return bi < bj
		}
		if out[i].Chapter != out[j].Chapter {
			return out[i].Chapter < out[j].Chapter
		}
		return out[i].VerseLo < out[j].VerseLo
	})
	return out
}

// matchNotes filters notes by a query, matching the note's TEXT and its
// REFERENCE — a reader looking for "john 11" means the passage, and one looking
// for "hospital" means the message; both are the same box. Case-insensitive,
// plain substring: with at most notesMax short notes there is nothing here worth
// an index, and a fuzzy match would only produce results the reader cannot
// explain.
//
// An empty query matches everything, which is what makes the mode a browser
// first and a search second.
func matchNotes(notes []SharedNote, query string) []SharedNote {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return notes
	}
	out := make([]SharedNote, 0, len(notes))
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n.Text), q) ||
			strings.Contains(strings.ToLower(noteReference(n)), q) {
			out = append(out, n)
		}
	}
	return out
}

// bookOrderOf indexes the loaded canon so notes sort in reading order.
func bookOrderOf(state *AppState) map[string]int {
	order := map[string]int{}
	if state == nil || state.Bible == nil {
		return order
	}
	for i, b := range state.Bible.Books {
		order[b] = i
	}
	return order
}

// browsableNotes is the list the Notes mode shows for the current query.
func browsableNotes(state *AppState) []SharedNote {
	if state == nil || !notesFeatureOn(state) {
		return nil
	}
	return matchNotes(sortedNotes(readNotes(appPrefs()), bookOrderOf(state)), state.NotesQuery)
}

// openNote goes to the passage a note is attached to.
//
// It reuses the search-result path so arriving from a note and arriving from a
// search hit are the same act — same history entry, same unfocus-first
// keyboard handling, same jump to the Read tab. addRecentChapter along that path
// is what re-surfaces the note itself.
//
// A minimized note is restored on the way. The reader has just tapped the note
// in a list of notes; landing on a chapter showing only a collapsed marker would
// be answering a different question from the one they asked.
func openNote(state *AppState, n SharedNote) {
	if state == nil {
		return
	}
	if n.Minimized {
		setNoteMinimized(appPrefs(), n.VersionID, n.Book, n.Chapter, false)
	}
	openSearchResultRange(state, Verse{BookName: n.Book, Chapter: n.Chapter, Verse: n.VerseLo}, n.VerseHi)
}

// buildNotesBrowseView renders the Notes mode: every stored note matching the
// query, newest canon order first, each tapping through to its passage.
func buildNotesBrowseView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	notes := browsableNotes(state)

	if len(notes) == 0 {
		msg := "Notes people share with you appear here."
		if strings.TrimSpace(state.NotesQuery) != "" {
			msg = "No notes match “" + strings.TrimSpace(state.NotesQuery) + "”."
		}
		hint := widget.NewLabel(msg)
		hint.Wrapping = fyne.TextWrapWord
		hint.Alignment = fyne.TextAlignCenter
		return container.NewPadded(container.NewVBox(spacer(24), hint))
	}

	column := container.NewVBox()
	for _, n := range notes {
		column.Add(noteBrowseRow(state, n, pal))
	}
	return container.NewVScroll(column)
}

// noteBrowseRow is one note in the list: its passage, then its message. The
// whole card is the tap target, matching the search results it sits beside.
func noteBrowseRow(state *AppState, n SharedNote, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(noteReference(n), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 18

	// The note's own words, wrapped, never styled as the app's voice and never
	// as markup — the same rule the bubble follows. A collapsed note still shows
	// its text here; the browser is where you read them, the chapter is where
	// you chose how much of it to see.
	body := widget.NewLabel(strings.TrimSpace(n.Text))
	body.Wrapping = fyne.TextWrapWord

	rows := container.NewVBox(ref, body)
	if n.Minimized {
		quiet := canvas.NewText("Minimized in the chapter", pal.TextMuted)
		quiet.TextSize = 12
		rows.Add(quiet)
	}

	inner := container.NewPadded(rows)
	card := newNoteBrowseCard(state, n, inner, pal)
	return container.NewVBox(card, widget.NewSeparator())
}

// newNoteBrowseCard is a search-result card that opens a note instead of a
// verse — same widget, so the row's hover, tap target and spacing cannot drift
// from the search hits beside it.
func newNoteBrowseCard(state *AppState, n SharedNote, content fyne.CanvasObject, pal palette) *searchResultCard {
	c := newSearchResultCard(state, Verse{BookName: n.Book, Chapter: n.Chapter, Verse: n.VerseLo}, content, pal)
	c.onTap = func() { openNote(state, n) }
	return c
}
