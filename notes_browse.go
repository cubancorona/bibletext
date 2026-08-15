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
	"time"

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
// noteWho filters the list by who a note is from.
//
// Three positions, defaulting to everything. It earns its place as soon as the
// list is long, and it is the control the person layer later extends — "from
// neutral contact" is another position on this same filter rather than a fourth screen. See
// [redacted-retired-private-reference].
type noteWho int

const (
	whoAnyone noteWho = iota
	whoOthers
	whoMe
)

const prefNotesWho = "notes.browse.who"

func (w noteWho) label() string {
	switch w {
	case whoOthers:
		return "From others"
	case whoMe:
		return "From you"
	default:
		return "Everyone"
	}
}

// keeps reports whether a note belongs in this filter's list.
func (w noteWho) keeps(n SharedNote) bool {
	switch w {
	case whoOthers:
		return !n.Mine
	case whoMe:
		return n.Mine
	default:
		return true
	}
}

func notesWhoPref() noteWho {
	p := appPrefs()
	if p == nil {
		return whoAnyone
	}
	switch p.String(prefNotesWho) {
	case "others":
		return whoOthers
	case "me":
		return whoMe
	}
	return whoAnyone
}

func setNotesWhoPref(w noteWho) {
	p := appPrefs()
	if p == nil {
		return
	}
	switch w {
	case whoOthers:
		p.SetString(prefNotesWho, "others")
	case whoMe:
		p.SetString(prefNotesWho, "me")
	default:
		p.SetString(prefNotesWho, "")
	}
}

// filterNotesByWho keeps only the notes the filter admits.
func filterNotesByWho(notes []SharedNote, w noteWho) []SharedNote {
	if w == whoAnyone {
		return notes
	}
	out := notes[:0:0]
	for _, n := range notes {
		if w.keeps(n) {
			out = append(out, n)
		}
	}
	return out
}

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
func sortedNotes(notes []SharedNote, bookOrder map[string]int, by noteSort) []SharedNote {
	out := make([]SharedNote, 0, len(notes))
	out = append(out, notes...)
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
		// Own notes are a LIST, not a keyed map, so two of them can agree on
		// every field above and still be different notes — that is the whole
		// reason they are a list. Without these last two the order stops being
		// total and the list can reshuffle between renders of unchanged data.
		if a.Mine != b.Mine {
			return b.Mine, true // a friend's note before your own
		}
		if a.Received != b.Received {
			return a.Received < b.Received, true
		}
		if a.Text != b.Text {
			return a.Text < b.Text, true
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
	all := sortedNotes(allNotesForBrowsing(appPrefs()), bookOrderOf(state), notesSortPref())
	// The WHO filter narrows the pool before the text query, and `total` counts
	// the filtered pool — so "3 of 7" answers "of the notes I asked to see",
	// which is the question the reader is actually holding. A total that
	// silently counted notes the filter had excluded would read as a bug.
	all = filterNotesByWho(all, notesWhoPref())
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
	// Go to the note's TRANSLATION first. Notes are keyed version|book|chapter,
	// so navigating without switching looks the note up under whichever
	// translation the reader happens to be in, finds nothing, and clears it —
	// the reader taps a note in a list of notes and lands on a chapter with no
	// note. The same deferral the shared-link path uses: an in-memory
	// translation switches synchronously and we navigate below; a real download
	// parks the target and applyLoadedVersion finishes the job.
	if switchToLinkVersion(state, ShareTarget{
		VersionID: n.VersionID, Book: n.Book, Chapter: n.Chapter,
		VerseLo: n.VerseLo, VerseHi: n.VerseHi,
	}) {
		return
	}
	// The note's book may still be absent from the canon now loaded — a webc
	// deuterocanon note read back under WEB when WEBC could not be loaded.
	// selectBook would set the book regardless, stranding the reader on a blank
	// pane AND persisting a dead book, which makes the NEXT launch fail its
	// restore and drop them at Genesis 1. Leave them where they are instead.
	if state.Bible == nil || state.Bible.GetChaptersForBook(n.Book) == 0 {
		return
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
		hint := widget.NewLabel("Notes appear here — the ones people share with you, " +
			"and the ones you send. Open a link with a note, or share a passage with " +
			"a note of your own, and it will be kept so you can come back to it.")
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

	// WHO the notes are from, cycling Everyone → From others → From you. Same
	// shape as the sort control beside it and for the same reason: it names the
	// state it is IN, so the reader can never wonder whether a short list is a
	// filter or a small collection. It appears only once there is something to
	// separate — with no notes of your own it would be a control with one
	// meaningful position.
	who := notesWhoPref()
	controls := fyne.CanvasObject(sorter)
	if mine, _ := readMyNotes(appPrefs()); len(mine) > 0 || who != whoAnyone {
		whoBtn := widget.NewButton(who.label(), func() {
			switch who {
			case whoAnyone:
				setNotesWhoPref(whoOthers)
			case whoOthers:
				setNotesWhoPref(whoMe)
			default:
				setNotesWhoPref(whoAnyone)
			}
			state.refresh()
		})
		whoBtn.Importance = widget.LowImportance
		controls = container.NewHBox(
			container.NewThemeOverride(whoBtn, compactTheme{Theme: base, text: 13}),
			sorter,
		)
	}

	// THE WAY OUT. On desktop the list claims the whole results pane, and the
	// only other exit is the sidebar's Search/Find/Notes control — which the
	// reader never touched if they arrived from Settings → the note count, so it
	// still reads "Search" and does not look like the thing holding them here.

	// pane". A view that takes over the pane owns its own exit.
	//
	// Desktop only: the phones leave through the tab bar, which is always
	// visible and already says where you are.
	head := container.NewBorder(nil, nil, nil, controls, container.NewPadded(line))
	// surfaceSearch is set only by the phones (it is how they bring the Search
	// tab forward) — the same signal showNotesList uses to tell the layouts
	// apart, rather than a second way of asking the same question.
	if state.surfaceSearch == nil {
		done := widget.NewButtonWithIcon("Done", theme.NavigateBackIcon(), func() {
			closeNotesList(state)
		})
		done.Importance = widget.LowImportance
		head = container.NewBorder(nil, nil,
			container.NewThemeOverride(done, compactTheme{Theme: base, text: 13}),
			controls, container.NewPadded(line))
	}

	column := container.NewVBox()
	for _, n := range notes {
		column.Add(noteBrowseRow(state, n, pal))
	}
	// squeezeWidthLayout: a scroll widens its content to the content's MinSize
	// and clips the overflow sideways, with no bar to reach it (see sheet_fit.go).
	body := container.NewVScroll(container.New(squeezeWidthLayout{}, column))
	// Keep the reader's place in the list. Opening a note and coming back — or
	// any rebuild while Notes mode is up — otherwise dropped them at the top of
	// what can be a long list. Restored after layout, because a scroll clamps an
	// offset against a content height it has not measured yet.
	body.OnScrolled = func(p fyne.Position) { state.notesScroll = p.Y }
	if state.notesScroll > 0 {
		off := state.notesScroll
		time.AfterFunc(16*time.Millisecond, func() {
			fyne.Do(func() {
				body.Offset = fyne.NewPos(0, off)
				body.Refresh()
			})
		})
	}
	return container.NewBorder(head, nil, nil, nil, body)
}

// noteBrowseRow is one note in the list: its passage, then its message. The
// whole card is the tap target, matching the search results it sits beside.
// noteDateLabel says when a note arrived, quietly.
//
// Recent notes read as words ("Today", "Yesterday") because that is how someone
// thinks about a message that just came; past a week it becomes a date, and the
// year appears only when it is not this one — the same grammar a mail client
// uses, and the reason the line can sit beside the reference without competing
// with it.
//
// Returns "" for a note with no timestamp. Received was added after the first
// notes shipped, so an early note can legitimately have none, and inventing
// "Today" for one that arrived weeks ago would be worse than saying nothing.
// A timestamp in the FUTURE (a clock that moved) reads as "Today" rather than
// as a negative day count.
func noteDateLabel(ts int64, now time.Time) string {
	if ts <= 0 {
		return ""
	}
	t := time.Unix(ts, 0).In(now.Location())
	midnight := func(x time.Time) time.Time {
		y, m, d := x.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, x.Location())
	}
	days := int(midnight(now).Sub(midnight(t)).Hours() / 24)
	switch {
	case days <= 0:
		return "Today"
	case days == 1:
		return "Yesterday"
	case days < 7:
		return strconv.Itoa(days) + " days ago"
	case t.Year() == now.Year():
		return t.Format("2 Jan")
	default:
		return t.Format("2 Jan 2006")
	}
}

func noteBrowseRow(state *AppState, n SharedNote, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(noteReference(n), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = 18

	// The date rides on the reference's own line, muted and small, pushed to the
	// far edge. Given a line of its own it would read as a second fact about the
	// note; up here it reads as part of the heading, which is what a date is.
	// Centred vertically so the smaller text sits on the reference's optical
	// middle rather than hanging from the top of the row.
	head := fyne.CanvasObject(ref)
	if when := noteDateLabel(n.Received, time.Now()); when != "" {
		stamp := canvas.NewText(when, pal.TextMuted)
		stamp.TextSize = 12
		head = container.NewBorder(nil, nil, ref, container.NewCenter(stamp))
	}

	// The note's own words, in the SAME bubble the reading page draws, with the
	// byline and the translation OUTSIDE it (owner directive) — inside, they
	// would read as part of the message rather than as the app saying where it
	// came from. A collapsed note still shows its text here; the browser is
	// where you read them, the chapter is where you chose how much to see.
	body := noteBubbleWithByline(n.Text, noteByline(n), noteVersionName(n.VersionID), pal)

	rows := container.NewVBox(head, body)
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

// setNotesMode is the ONE place the Notes flag moves, so leaving the mode always
// forgets the list's scroll position. Coming back to Notes should start at the
// top of the list; only a rebuild WHILE in Notes should return the reader to
// where they were. Three callers set this flag (the desktop sidebar toggle, the
// mobile toggle, and the mobile reset), and a scroll offset left behind by one
// of them would silently reappear on a later, unrelated visit.
func setNotesMode(state *AppState, on bool) {
	if state == nil {
		return
	}
	if !on {
		state.notesScroll = 0
	}
	state.NotesMode = on
}

// showNotesList takes the reader to the notes list in the Search pane, from
// wherever they are. Used by the count beside Settings → "Delete all notes":
// deciding whether to delete them is exactly when you want to look at them.
//
// The two layouts get there differently, which is why this exists rather than a
// call to either toggle. On DESKTOP the results pane only exists while
// IsSearching, so Notes has to claim it the way the sidebar's own toggle does.
// On MOBILE the Search tab owns it and surfaceSearch is what brings that tab
// forward; without it the flag would flip under a reading view and nothing would
// appear to happen.
func showNotesList(state *AppState) {
	if state == nil || !notesFeatureOn(state) {
		return
	}
	setNotesMode(state, true)
	state.IsSearching = true // desktop: Notes claims the results pane on entry
	if state.surfaceSearch != nil {
		state.surfaceSearch() // mobile: bring the real Search tab forward
	}
	state.refresh()
}

// closeNotesList is showNotesList's inverse, behind the list's own "Done" —
// leave Notes and give the pane back to the reading view.
//
// It hands the pane back only when nothing ELSE is owed it: a reader who ran a
// keyword search, switched to Notes and then pressed Done should land back on
// their results, not have them silently discarded. That is the same condition
// the sidebar's toggle applies when it leaves Notes (sidebar.go), and it is
// stated once here so the two exits cannot drift apart.
func closeNotesList(state *AppState) {
	if state == nil {
		return
	}
	setNotesMode(state, false)
	if strings.TrimSpace(state.ActiveSearchQuery) == "" {
		state.IsSearching = false
	}
	state.refresh()
}

// notesCountLink is the "(3)" beside Settings → "Delete all notes": it reports
// how many notes "all" means, and taps through to them.
//
// A plain tappable rather than a Hyperlink: a Hyperlink means "this leaves the
// app" everywhere else in Settings ("Get a key ↗", "Privacy Policy ↗"), and this
// goes somewhere inside it. It keeps the accent colour so it still reads as
// something you can press.
type notesCountLink struct {
	widget.BaseWidget
	state    *AppState
	count    int
	onTapped func()
}

func newNotesCountLink(state *AppState, onTapped func()) *notesCountLink {
	c := &notesCountLink{state: state, onTapped: onTapped}
	c.ExtendBaseWidget(c)
	return c
}

func (c *notesCountLink) setCount(n int) {
	c.count = n
	c.Refresh()
}

func (c *notesCountLink) label() string {
	if c.count <= 0 {
		return ""
	}
	return "(" + strconv.Itoa(c.count) + ")"
}

func (c *notesCountLink) CreateRenderer() fyne.WidgetRenderer {
	pal := c.state.pal()
	txt := canvas.NewText(c.label(), pal.Accent)
	txt.TextStyle = fyne.TextStyle{Bold: true}
	txt.TextSize = 15
	// A solid box behind the glyphs: Fyne's mobile hit-testing does not reliably
	// match a bare canvas.Text renderer (the rule [redacted-retired-private-reference] records), and a
	// two-character target needs all the tap area it can get.
	box := container.NewGridWrap(fyne.NewSize(52, 40), container.NewCenter(txt))
	return &notesCountRenderer{link: c, txt: txt, box: box}
}

type notesCountRenderer struct {
	link *notesCountLink
	txt  *canvas.Text
	box  fyne.CanvasObject
}

func (r *notesCountRenderer) Layout(s fyne.Size)           { r.box.Resize(s) }
func (r *notesCountRenderer) MinSize() fyne.Size           { return r.box.MinSize() }
func (r *notesCountRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.box} }
func (r *notesCountRenderer) Destroy()                     {}
func (r *notesCountRenderer) Refresh() {
	r.txt.Text = r.link.label()
	r.txt.Color = r.link.state.pal().Accent
	r.txt.Refresh()
}

func (c *notesCountLink) Tapped(*fyne.PointEvent) {
	if c.count > 0 && c.onTapped != nil {
		c.onTapped()
	}
}

var _ fyne.Tappable = (*notesCountLink)(nil)
