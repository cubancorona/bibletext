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
	"fyne.io/fyne/v2/theme"
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

// noteSort is how the browser orders the list.
type noteSort int

const (
	// sortNewest is the default, and deliberately so: these are messages. When
	// somebody sends you one, the thing you want on opening the list is the one
	// that just arrived — not wherever it happens to fall in the canon.
	sortNewest noteSort = iota
	sortBook            // Genesis first, Revelation last
)

const prefNotesSort = "notes.sort"

// notesSortPref / setNotesSortPref persist the choice, because a sort order the
// reader picks and the app forgets every launch is a setting that does not work.
func notesSortPref() noteSort {
	if p := appPrefs(); p != nil && p.String(prefNotesSort) == "book" {
		return sortBook
	}
	return sortNewest
}

func setNotesSortPref(s noteSort) {
	p := appPrefs()
	if p == nil {
		return
	}
	if s == sortBook {
		p.SetString(prefNotesSort, "book")
		return
	}
	p.SetString(prefNotesSort, "newest")
}

func (s noteSort) label() string {
	if s == sortBook {
		return "Bible order"
	}
	return "Newest first"
}

// sortedNotes orders the stored notes.
//
// bookOrder maps a book name to its position in the loaded canon; anything the
// current translation does not contain (a note taken in the Catholic canon, read
// back under a 66-book one) sorts to the end — still visible, still openable
// once that translation is picked again.
//
// Both orders are TOTAL: ties fall through to the canon, then chapter, then
// verse, then translation — the full storage key — so the list never reshuffles
// between two renders of the same data.
// Notes stored before Received existed have a zero timestamp and sort as the
// oldest, which is the truthful answer — they did arrive first.
func sortedNotes(notes map[string]SharedNote, bookOrder map[string]int, by noteSort) []SharedNote {
	out := make([]SharedNote, 0, len(notes))
	for _, n := range notes {
		out = append(out, n)
	}
	canonLess := func(a, b SharedNote) (bool, bool) {
		ia, oka := bookOrder[a.Book]
		ib, okb := bookOrder[b.Book]
		if oka != okb {
			return oka, true // known books before unknown ones
		}
		if !oka {
			if a.Book != b.Book {
				return a.Book < b.Book, true
			}
		} else if ia != ib {
			return ia < ib, true
		}
		if a.Chapter != b.Chapter {
			return a.Chapter < b.Chapter, true
		}
		if a.VerseLo != b.VerseLo {
			return a.VerseLo < b.VerseLo, true
		}
		// Translation last, and it is what MAKES the order total: notes are keyed
		// version|book|chapter (notes_store.go), so two notes can agree on book,
		// chapter and verse and still be different notes — one from a WEB link,
		// one from a BSB link on the same passage. Without this they compared
		// equal, sort.Slice is not stable, and the list could reshuffle between
		// two renders of unchanged data — the very thing the doc above promises
		// it never does.
		if a.VersionID != b.VersionID {
			return a.VersionID < b.VersionID, true
		}
		return false, false // the same note
	}
	sort.Slice(out, func(i, j int) bool {
		if by == sortNewest && out[i].Received != out[j].Received {
			return out[i].Received > out[j].Received // newest first
		}
		less, decided := canonLess(out[i], out[j])
		if decided {
			return less
		}
		return false
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

// browsableNotes is the list the Notes mode shows for the current query, and
// storedNoteCount is how many there are in total — the two numbers the header
// line needs to say "3 of 7".
func browsableNotes(state *AppState) (shown []SharedNote, total int) {
	if state == nil || !notesFeatureOn(state) {
		return nil, 0
	}
	all := sortedNotes(readNotes(appPrefs()), bookOrderOf(state), notesSortPref())
	return matchNotes(all, state.NotesQuery), len(all)
}

// notesHeaderLine says what the list is currently showing.
//
// It earns its place by answering the question a list of unknown length always
// raises — "is this everything?" — which an unlabelled list leaves the reader to
// guess at. With nothing typed it says so outright; with a query it says how
// much of the whole set survived, so a short list never looks like a short
// collection.
//
// It does NOT name the sort order: the control immediately beside it already
// does, and saying it twice reads like two different facts.
func notesHeaderLine(shown, total int, query string) string {
	q := strings.TrimSpace(query)
	switch {
	case total == 0:
		return ""
	case q == "" && total == 1:
		return "Your one note."
	case q == "":
		return "All " + strconv.Itoa(total) + " notes."
	case shown == 0:
		return "No notes match “" + q + "”."
	case shown == total:
		return "All " + strconv.Itoa(total) + " notes match “" + q + "”."
	default:
		return strconv.Itoa(shown) + " of " + strconv.Itoa(total) + " notes match “" + q + "”."
	}
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

// buildNotesBrowseView renders the Notes mode: a line saying what is on show,
// the sort control, then every matching note, each tapping through to its
// passage.
func buildNotesBrowseView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	notes, total := browsableNotes(state)

	// Nothing stored at all is a different situation from a filter that matched
	// nothing, and gets a different sentence: one explains the feature, the other
	// reports a result.
	if total == 0 {
		hint := widget.NewLabel("Notes people share with you appear here. " +
			"Open a link with a note and it will be kept, so you can come back to it.")
		hint.Wrapping = fyne.TextWrapWord
		hint.Alignment = fyne.TextAlignCenter
		return container.NewPadded(container.NewVBox(spacer(24), hint))
	}

	by := notesSortPref()
	line := canvas.NewText(notesHeaderLine(len(notes), total, state.NotesQuery), pal.TextMuted)
	line.TextSize = 12

	// One button that names the order it is IN, and swaps to the other on tap.
	// With exactly two orders a menu would be three taps to do what one does, and
	// showing the current state is what keeps that honest.
	sortBtn := widget.NewButton(by.label(), func() {
		if by == sortNewest {
			setNotesSortPref(sortBook)
		} else {
			setNotesSortPref(sortNewest)
		}
		state.refresh()
	})
	sortBtn.Importance = widget.LowImportance
	// Shrunk deliberately: at the app's normal chrome size a borderless button
	// this wide reads as a heading competing with the line beside it, rather than
	// as the small control it is.
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	sorter := container.NewThemeOverride(sortBtn, compactTheme{Theme: base, text: 13})

	head := container.NewBorder(nil, nil, nil, sorter, container.NewPadded(line))

	column := container.NewVBox()
	for _, n := range notes {
		column.Add(noteBrowseRow(state, n, pal))
	}
	// squeezeWidthLayout: a scroll widens its content to the content's MinSize
	// and clips the overflow sideways, with no bar to reach it (see sheet_fit.go).
	body := container.NewVScroll(container.New(squeezeWidthLayout{}, column))
	return container.NewBorder(head, nil, nil, nil, body)
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
