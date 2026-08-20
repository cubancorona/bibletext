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
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// --- the row's density budget (owner, 2026-08-19: "the notes browser takes up
// far too much space", reported on BOTH form factors — one shared row serves
// them, so one set of sizes fixes both). The named sizes below are what the
// density seen-test derives its budget from (notes_browse_density_test.go):
// change a size here and the budget moves with it; grow the row's STRUCTURE
// and the test is what says so.
const (
	browseRefTextSize   float32 = 13 // the reference heading (was 18)
	browseMetaTextSize  float32 = 11 // abbrev, byline, date, the minimized marker
	browseBodyTextSize  float32 = 13 // the bubble's message text (app body is 18)
	browseTrashIconSize float32 = 13 // the row's delete mark; smaller than the app's inline icon
	browseRowGap        float32 = 3  // between the row's stacked pieces (VBox gap)
	browseRowPad        float32 = 3  // around the whole row, replacing NewPadded's 7
	browseBubblePad     float32 = 3  // inside the bubble card (the banner keeps theme.Padding)
	browseSepGap        float32 = 2  // between the card and its separator

	// The visible body is a PREVIEW, wrap-limited: the row's tap already
	// navigates to the passage where a received note shows in full, so the cap
	// costs nothing there — and it is what holds a long message to a few lines
	// instead of a screenful. (For a Kind=mine note the tap lights the passage
	// but the text lives only here; the cap still applies — the owner chose a
	// dense LIST, and a scrapbook row is a scrapbook row.)
	browsePreviewMaxRunes = 220
	browsePreviewMaxLines = 4
)

// browseRowTheme is the row-scoped override that makes the browser dense on
// BOTH form factors: smaller body text, tighter Label inner padding, tighter
// wrapped-line spacing. Desktop had NO row override at all — the content
// rendered at the app's full chrome sizes (18pt body, 8pt inner padding) —
// which is precisely the owner's "fonts and spacing need to be resized" on
// mimic/desktop.
type browseRowTheme struct{ fyne.Theme }

func (t browseRowTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return browseBodyTextSize
	case theme.SizeNameInnerPadding:
		return browseLabelInnerPad
	case theme.SizeNameLineSpacing:
		return 4
	case theme.SizeNameInlineIcon:
		// The row's bin, smaller than the app's default inline icon (owner,
		// twice: "not too large. Subtle", then "a bit smaller still"). The row
		// is dense chrome — an 11pt reference and a smaller byline — and the
		// delete is furniture beside the words, not a feature of them. The
		// BUTTON keeps its own tap area; only the drawing shrinks.
		return browseTrashIconSize
	}
	return t.Theme.Size(name)
}

// browseLabelInnerPad is the Label box padding inside a row (the app's is 8).
const browseLabelInnerPad float32 = 3

// notePreview caps the note text a browser row shows: at most
// browsePreviewMaxLines authored lines and browsePreviewMaxRunes runes, with
// an ellipsis marking the cut. Deterministic (a string function, not a layout
// truncation) so the row's height cannot depend on render order.
func notePreview(text string) string {
	s := strings.TrimSpace(text)
	cut := false
	if lines := strings.Split(s, "\n"); len(lines) > browsePreviewMaxLines {
		s = strings.TrimSpace(strings.Join(lines[:browsePreviewMaxLines], "\n"))
		cut = true
	}
	if r := []rune(s); len(r) > browsePreviewMaxRunes {
		s = strings.TrimSpace(string(r[:browsePreviewMaxRunes]))
		cut = true
	}
	if cut {
		s += " …"
	}
	return s
}

// noteReference is the passage a note is attached to, as a reader would write
// it: "John 11:35", "Psalms 23:1-4", or just "Psalms 23" for a whole chapter.
func noteReference(n StoredNote) string {
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
// Mum" is another position on this same filter rather than a fourth screen. See
// docs/NOTES_SCRAPBOOK.md.
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
func (w noteWho) keeps(n StoredNote) bool {
	switch w {
	case whoOthers:
		return n.Kind != noteKindMine
	case whoMe:
		return n.Kind == noteKindMine
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
func filterNotesByWho(notes []StoredNote, w noteWho) []StoredNote {
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
func sortedNotes(notes []StoredNote, bookOrder map[string]int, by noteSort) []StoredNote {
	out := make([]StoredNote, 0, len(notes))
	out = append(out, notes...)
	canonLess := func(a, b StoredNote) (bool, bool) {
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
		// Two notes can agree on book, chapter and verse and still be
		// different notes — the store has no passage key to collide on — so the
		// order keeps falling through: translation, kind, arrival time, and
		// finally the ID, which is unique and makes the order TOTAL. Without a
		// total order sort.Slice (not stable) could reshuffle the list between
		// two renders of unchanged data — the very thing the doc above promises
		// it never does.
		if a.VersionID != b.VersionID {
			return a.VersionID < b.VersionID, true
		}
		if a.Kind != b.Kind {
			return a.Kind != noteKindMine, true // a friend's note before your own
		}
		if a.Received != b.Received {
			return a.Received < b.Received, true
		}
		if a.ID != b.ID {
			return a.ID < b.ID, true
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
// plain substring: over a store of short notes there is nothing here worth
// an index, and a fuzzy match would only produce results the reader cannot
// explain.
//
// An empty query matches everything, which is what makes the mode a browser
// first and a search second.
func matchNotes(notes []StoredNote, query string) []StoredNote {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return notes
	}
	out := make([]StoredNote, 0, len(notes))
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
func browsableNotes(state *AppState) (shown []StoredNote, total int) {
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
//
// IT RAISES NO RESULTS TRAIL (owner, 2026-08-19): a browser row is a
// NAVIGATION, not a search hit, so CanReturnToSearchResults stays false and no
// "back to results" bar appears over the passage. The way back to the list is
// the Search tab's Notes mode (the tab bar on the phones, the sidebar's notes
// bubble on desktop), which still holds the list — and its scroll position.
func openNote(state *AppState, n StoredNote) {
	if state == nil {
		return
	}
	// Leaving the list for the passage: remember where in the list the reader
	// was, so coming back to Notes returns them to this note's neighbourhood.
	// The windowed list has no continuous scroll callback (see notesScrollRead),
	// so the moment of leaving is when the offset is harvested.
	if state.notesScrollRead != nil {
		state.notesScroll = state.notesScrollRead()
	}
	if n.Minimized {
		// By the note's own identity, handed here by the row that drew it —
		// never a key rebuilt from its fields.
		setNoteMinimizedByID(appPrefs(), n.ID, false)
	}
	// A leftover foreign mark stands aside NOW, before any park or navigation:
	// choosing a note is the reader choosing it as the page's reason, exactly
	// as the chip tap and the pill press declare (restoreCurrentNote,
	// advanceNoteFocus). Without this the mark rode along — a search hit still
	// lit from before the reader switched the tab to Notes — and the parked
	// path in particular waited out a whole download only to land the chosen
	// note suppressed to the pill.
	if state.mark.live() && !state.mark.fromNote() {
		state.clearMark()
		state.suppressionTookOpen = false
	}
	// Go to the note's TRANSLATION first — the passage the note is about is a
	// passage of the translation it was written against, and the derive can
	// only follow it elsewhere where the numbering corresponds. The same
	// deferral the shared-link path uses: an in-memory
	// translation switches synchronously and we navigate below; a real download
	// parks the target and applyLoadedVersion finishes the job.
	if switchToLinkVersion(state, ShareTarget{
		VersionID: n.VersionID, Book: n.Book, Chapter: n.Chapter,
		VerseLo: n.VerseLo, VerseHi: n.VerseHi,
	}) {
		// THE PARK REMEMBERS THE SHOW INTENT. switchToLinkVersion parks a bare
		// ShareTarget, and the generic consume (applyShareTarget) would treat
		// the eventual arrival as a LINK arrival: its hlLinkSpan is foreign to
		// the note, so the note the reader tapped arrived suppressed to the
		// pill — mechanism 2 of the owner's "often still minimized pill".
		// consumePendingLink sees this id and re-runs THIS verb instead, now
		// that the translation is in memory (see AppState.pendingNoteOpenID).
		state.pendingNoteOpenID = n.ID
		// The navigation (and its own derive) runs in the load's apply tail,
		// but the un-minimize above is already WRITTEN — so end on the
		// projection and a repaint now, or the reader waits out the fetch (or
		// its failure) looking at a list row still marked hidden, over a store
		// that says otherwise.
		applyNoteForCurrentChapter(state)
		state.refresh()
		return
	}
	// The note's book may still be absent from the canon now loaded — a webc
	// deuterocanon note read back under WEB when WEBC could not be loaded.
	// selectBook would set the book regardless, stranding the reader on a blank
	// pane AND persisting a dead book, which makes the NEXT launch fail its
	// restore and drop them at Genesis 1. Leave them where they are instead.
	if state.Bible == nil || state.Bible.GetChaptersForBook(n.Book) == 0 {
		// The verb aborts, but the un-minimize is already stored: same rule as
		// the parked return — the list the reader is still standing in must not
		// keep drawing the row's hidden marker over a store that disagrees.
		applyNoteForCurrentChapter(state)
		state.refresh()
		return
	}
	if state.window != nil {
		if c := state.window.Canvas(); c != nil {
			c.Unfocus()
		}
	}
	// Clamp the chapter to the loaded book — the twin of applyShareTarget's
	// clamp. A note can name a chapter this canon does not reach (Daniel 13
	// stored under a wider canon, read back after a refused switch), and a raw
	// assignment landed on an invalid chapter AND persisted it, failing the
	// next launch's restore.
	chapter := clampChapter(state.Bible, n.Book, n.Chapter)
	if n.Kind == noteKindMine {
		// YOUR OWN NOTE, and it now behaves exactly like a received one —
		// because the plan draws it while focus names it (chapterPlan.Own).
		//
		// WHAT THIS REPLACES, and why it was worse than "a dead tap": this
		// branch used to raise an hlVerseOfDay mark on the note's verses. That
		// origin is not fromNote (mark.go), so notesSuppressed was TRUE, and
		// the Open loop stood every note on the chapter down. Tapping your own
		// row did not merely fail to show your words — it collapsed a FRIEND's
		// open note on that passage into a pill. You touched your own note and
		// hers appeared to go away.
		//
		// So: no hand-set mark and no suppression capture. Focus it and let the
		// projection raise the note's own hlNote mark and wash, which is what
		// the received branch below has always done. The two branches are one
		// behaviour now, differing only in the byline the projection composes
		// ("Note from you", senderByline) — and the note is drawn only until
		// the reader navigates away, because navigation resets focus.
		selectBook(state, n.Book, false)
		state.CurrentChapter = chapter
		addRecentChapter(state, n.Book, chapter)
		state.forceReposition = true
		state.restore = nil
		state.focusNote(n.ID)
		applyNoteForCurrentChapter(state)
		state.IsSearching = false
		state.CanReturnToSearchResults = false
		state.refresh()
		if state.surfaceReading != nil {
			state.surfaceReading()
		}
		return
	}
	// The arrival is the NOTE'S OWN — never a search result's. This used to
	// route through openSearchResultRange, which set an hlSearch mark on the
	// note's very verses: a FOREIGN mark, so the plan stood the note down and
	// the reader who had just tapped it in the list was greeted by the pill
	// (field report: "takes me to the reading pane with a minimized pill").
	// Selecting a note — its chip, its link, the count-tap, this row — is the
	// reader choosing it as the page's reason (the identity table), so the
	// arrival focuses it and lets the projection raise the note's own mark
	// and wash. The navigation plumbing mirrors openSearchResultRange minus
	// that mark; focus is set AFTER addRecentChapter, whose own derive resets
	// it (the navigation-reset rule), and the projection re-derives with the
	// focus in hand.
	selectBook(state, n.Book, false)
	state.CurrentChapter = chapter
	addRecentChapter(state, n.Book, chapter)
	state.forceReposition = true
	state.restore = nil
	state.focusNote(n.ID)
	applyNoteForCurrentChapter(state)
	state.IsSearching = false
	state.CanReturnToSearchResults = false
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
}

// notesCapacityNoticeAt is where the browser starts saying, once and quietly,
// that the scrapbook has grown very large. It is a NOTICE, not a cap: the store
// keeps everything it is given, always (docs/NOTES_SCRAPBOOK.md — eviction is a
// data-loss event, and the old store's silent 200-entry eviction is the defect
// this replaced). The number marks where an unbounded collection starts to have
// a cost worth naming — load parse and preferences rewrite grow with the store —
// years before it bites on any current device.
const notesCapacityNoticeAt = 2000

// notesCapacityLine is the one quiet sentence shown at and past the threshold.
// The app's own chrome voice: no link, no button, no action asked of anyone —
// because there is nothing the reader must do. It states the promise (nothing
// is removed) before the cost (a very large scrapbook can be slow), in that
// order, so it cannot be misread as a warning that notes are about to go away.
func notesCapacityLine(stored int) string {
	if stored < notesCapacityNoticeAt {
		return ""
	}
	return "Every note is kept — nothing is ever removed — though a scrapbook this large can be slow on older devices."
}

// buildNotesBrowseView renders the Notes mode: a line saying what is on show,
// the sort control, then every matching note, each tapping through to its
// passage.
func buildNotesBrowseView(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	// A rebuild while Notes mode is up (sort flip, theme change, a note
	// arriving) replaces the list wholesale; ask the OLD list where the reader
	// was before building the new one at the same place.
	if state != nil && state.notesScrollRead != nil {
		state.notesScroll = state.notesScrollRead()
	}
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
	// as the small control it is. browseRowTheme (13pt text, 4pt inner padding)
	// keeps the header chrome on the same density budget as the rows below it.
	var base fyne.Theme = theme.DefaultTheme()
	if state.theme != nil {
		base = state.theme
	}
	sorter := container.NewThemeOverride(sortBtn, browseRowTheme{Theme: base})

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
			container.NewThemeOverride(whoBtn, browseRowTheme{Theme: base}),
			sorter,
		)
	}

	// THE WAY OUT. On desktop the list claims the whole results pane, and the
	// only other exit is the sidebar's Search/Find/Notes control — which the
	// reader never touched if they arrived from Settings → the note count, so it
	// still reads "Search" and does not look like the thing holding them here.
	// Owner-reported: "no way to go from the notes view back to the reading
	// pane". A view that takes over the pane owns its own exit.
	//
	// Desktop only: the phones leave through the tab bar, which is always
	// visible and already says where you are.
	linePadded := container.New(layout.NewCustomPaddedLayout(2, 2, browseRowPad, browseRowPad), line)
	head := container.NewBorder(nil, nil, nil, controls, linePadded)
	// surfaceSearch is set only by the phones (it is how they bring the Search
	// tab forward) — the same signal showNotesList uses to tell the layouts
	// apart, rather than a second way of asking the same question.
	if state.surfaceSearch == nil {
		done := widget.NewButtonWithIcon("Done", theme.NavigateBackIcon(), func() {
			closeNotesList(state)
		})
		done.Importance = widget.LowImportance
		head = container.NewBorder(nil, nil,
			container.NewThemeOverride(done, browseRowTheme{Theme: base}),
			controls, linePadded)
	}

	// THE CAPACITY NOTICE (S11b). One quiet sentence in the header once the
	// stored count — the whole store, not the filtered view — reaches the
	// threshold. It rides under the header line rather than in the list, so it
	// is said once, not per scroll, and never pushes a note off screen.
	if capacity := notesCapacityLine(storedNoteCount(appPrefs())); capacity != "" {
		quiet := widget.NewLabel(capacity)
		quiet.Wrapping = fyne.TextWrapWord
		quiet.Importance = widget.LowImportance
		head = container.NewVBox(head,
			container.NewThemeOverride(quiet, compactTheme{Theme: base, text: 12}))
	}

	// THE WINDOWED LIST (S11a). A widget.List builds rows for the viewport, not
	// for the store: the VBox-per-note it replaces built all 2,000 rows of a
	// 2,000-note scrapbook to show the six that fit on screen (measured at 9.9 s
	// on an M3 Max — docs/NOTES_SCRAPBOOK.md hard case 14 called this column the
	// browser's only real ceiling). The ROW is unchanged: the same noteBrowseRow
	// the column added, separator and all — the List's own separators are hidden
	// so the row keeps drawing its own, exactly as before.
	//
	// Row heights vary (a bubble wraps its message), so each row measures itself
	// as it comes into view and tells the list via SetItemHeight: resizing to the
	// list's width first is what makes the wrapped label report its true height.
	// Off-screen rows fall back to the template's height until first seen — the
	// standard windowed-list trade: the scrollbar's extent is approximate until
	// the rows near it have been visited, and no reader-visible row is ever
	// wrong. SetItemHeight re-enters updateItem once via RefreshItem and then
	// stops, because the re-measure of an unchanged row is byte-stable.
	list := widget.NewList(
		func() int { return len(notes) },
		func() fyne.CanvasObject {
			// The template row sizes the list's idea of an unvisited row. A real
			// row from a real one-line note, so the estimate is a typical row and
			// the first layout builds a viewport's worth of rows, not hundreds.
			return container.NewVBox(noteBrowseRow(state, StoredNote{
				Book: "Psalms", Chapter: 23, VerseLo: 1, Text: "A note.",
			}, pal))
		},
		nil, // set below: UpdateItem needs the list itself for SetItemHeight
	)
	list.HideSeparators = true
	list.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
		if id < 0 || id >= len(notes) {
			return
		}
		slot := o.(*fyne.Container)
		row := noteBrowseRow(state, notes[id], pal)
		slot.Objects = []fyne.CanvasObject{row}
		slot.Refresh()
		if w := list.Size().Width; w > 0 {
			// Two passes: the first gives the wrapping labels their width (their
			// MinSize height is only honest once they know it), the second reads
			// the settled answer.
			row.Resize(fyne.NewSize(w, row.MinSize().Height))
			row.Resize(fyne.NewSize(w, row.MinSize().Height))
			list.SetItemHeight(id, row.MinSize().Height)
		}
	}
	// Keep the reader's place in the list. Opening a note and coming back — or
	// any rebuild while Notes mode is up — otherwise dropped them at the top of
	// what can be a long list. Restored after layout, because a list clamps an
	// offset against a content height it has not measured yet; harvested back
	// via notesScrollRead (see openNote and the top of this builder).
	// FOOTGUN (review): this closure aliases the list built by THIS call.
	// A build whose result is never shown repoints the harvest at a fresh,
	// unscrolled list and the reader's place dies at the next harvest. Every
	// shipped path shows what it builds; a future speculative build must not.
	state.notesScrollRead = list.GetScrollOffset
	if state.notesScroll > 0 {
		off := state.notesScroll
		time.AfterFunc(16*time.Millisecond, func() {
			fyne.Do(func() {
				list.ScrollToOffset(off)
			})
		})
	}
	return container.NewBorder(head, nil, nil, nil, list)
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

func noteBrowseRow(state *AppState, n StoredNote, pal palette) fyne.CanvasObject {
	ref := canvas.NewText(noteReference(n), pal.Accent)
	ref.TextStyle = fyne.TextStyle{Bold: true}
	ref.TextSize = browseRefTextSize

	// The translation in parentheses after the reference, quiet and small
	// (owner). It belongs here rather than under the bubble because it is
	// another fact about WHICH PASSAGE this is, and it reads as one heading
	// with the reference instead of as a second line making a second claim.
	// Abbreviated — "John 3:16 (WEB)" — because the full name at this size
	// competes with the reference it is qualifying.
	head0 := fyne.CanvasObject(ref)
	if abbrev := noteVersionAbbrev(n.VersionID); abbrev != "" {
		v := canvas.NewText("("+abbrev+")", pal.TextMuted)
		v.TextSize = browseMetaTextSize
		head0 = container.NewHBox(ref, container.NewCenter(v))
	}

	// The byline and the date fold into ONE muted fact on the heading's far
	// edge — "From you · Today" — instead of the byline holding a full-height
	// Label row of its own under the bubble (the single biggest line item in
	// the owner's "takes up far too much space"). The attribution still
	// accompanies every bubble, as it must; it is chrome, so it rides with the
	// row's other chrome. Centred vertically so the smaller text sits on the
	// reference's optical middle.
	meta := noteByline(n)
	if when := noteDateLabel(n.Received, time.Now()); when != "" {
		meta += " · " + when
	}
	stamp := canvas.NewText(meta, pal.TextMuted)
	stamp.TextSize = browseMetaTextSize
	head := container.NewBorder(nil, nil, head0, container.NewCenter(stamp))

	// The note's words, in the SAME bubble the reading page draws (owner: list
	// bubbles match reading bubbles — identity is the tailed shape, which the
	// row-scoped theme shrinks without changing). The body is a wrap-limited
	// PREVIEW (notePreview); the tap-through shows the whole message on the
	// passage. A collapsed note still shows its text here; the browser is
	// where you read them, the chapter is where you chose how much to see.
	body := noteBubblePadded(notePreview(n.Text), pal, browseBubblePad)

	// DELETE, BESIDE THE BUBBLE AND COSTING NO HEIGHT (owner). A bin, the same
	// mark the reading page uses where a press destroys — so the two places a
	// note can be deleted say it the same way, and neither depends on the
	// reader remembering which control means what.
	//
	// It rides in a Border to the bubble's RIGHT rather than in a row of its
	// own, so the row's height is whatever the bubble needed and the control
	// costs nothing vertically. That is a real constraint, not a hope:
	// TestNoteRowTrashCostsNoHeight measures the row with and without it and
	// fails on a single pixel of growth. The bubble keeps the whole remaining
	// width, so nothing about the message's wrap changes either.
	body = container.NewBorder(nil, nil, nil, noteRowTrash(state, n, pal), body)

	rows := container.New(layout.NewCustomPaddedVBoxLayout(browseRowGap), head, body)
	if n.Minimized {
		quiet := canvas.NewText("Minimized in the chapter", pal.TextMuted)
		quiet.TextSize = browseMetaTextSize
		rows.Add(quiet)
	}

	var base fyne.Theme = theme.DefaultTheme()
	if state != nil && state.theme != nil {
		base = state.theme
	}
	inner := container.New(
		layout.NewCustomPaddedLayout(browseRowPad, browseRowPad, browseRowPad, browseRowPad),
		container.NewThemeOverride(rows, browseRowTheme{Theme: base}))
	card := newNoteBrowseCard(state, n, inner, pal)
	return container.New(layout.NewCustomPaddedVBoxLayout(browseSepGap), card, widget.NewSeparator())
}

// noteRowTrash is the row's delete control: quiet, icon-only, and vertically
// centred so it sits on the bubble's middle whatever height the message wrapped
// to.
//
// It deletes without asking, which is the same contract the reading page's bin
// has and is defensible for the same reason: this is a list of your notes, the
// row you press is unambiguous, and the store is the reader's to prune. If a
// mis-tap here ever proves to be a real problem the answer is an undo, not a
// confirmation dialog on every delete — the store's charter is that it keeps
// what it is given, and an undo keeps that promise where a dialog only slows
// the deliberate case down.
func noteRowTrash(state *AppState, n StoredNote, pal palette) fyne.CanvasObject {
	btn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if state == nil {
			return
		}
		deleteNoteByID(appPrefs(), n.ID)
		// FOCUS MUST NOT OUTLIVE THE RECORD IT NAMES. Deleting from here left
		// noteFocus pointing at an id the store no longer has, and the reading
		// page then fell to the next note in the plan and OPENED it — a
		// stranger's message appearing, expanded, because you deleted your own
		// from a list. Measured:
		//
		//	showing: active="my own words" NoteID=2 focus={true 2}
		//	after:   active="friend words" NoteID=1 focus={true 2}  ← deleted id
		//
		// Every other delete path resets focus; this one was added without it.
		// Only when the deleted note is the one focus names — deleting some
		// other row must not close what the reader has open.
		//
		// focusNone, NOT resetNoteFocus. The reading page's own delete returns
		// focus to the DEFAULT RULE on purpose, so the rest of the set comes
		// back — the reader is looking at the passage and expects to see what
		// remains. Deleting from the LIST is a different act: the reader is
		// pruning a list, not reading, and the default rule would open the next
		// note on a page they are not even looking at. Closing is the honest
		// answer; the passage still shows a pill saying notes are there.
		if state.noteFocus.set && state.noteFocus.id == n.ID {
			state.focusNone()
		}
		// The reading page may be showing the very note just deleted, so the
		// projection re-derives before the repaint — the verb-to-screen rule.
		applyNoteForCurrentChapter(state)
		state.refresh()
	})
	btn.Importance = widget.LowImportance
	var base fyne.Theme = theme.DefaultTheme()
	if state != nil && state.theme != nil {
		base = state.theme
	}
	sized := container.NewThemeOverride(btn, browseRowTheme{Theme: base})
	// CENTRED ON THE BUBBLE'S BODY, NOT ON THE WHOLE SHAPE (owner). The bubble
	// is body + tail, and centring on the pair pushed the bin down by half the
	// tail's depth — it sat visibly low against the words it belongs to. The
	// bottom padding takes the tail's depth back out, so the mark lines up with
	// the middle of the message.
	return container.New(layout.NewCustomPaddedLayout(0, noteTailDepth, 0, 0),
		container.NewCenter(sized))
}

// newNoteBrowseCard is a search-result card that opens a note instead of a
// verse — same widget, so the row's hover, tap target and spacing cannot drift
// from the search hits beside it.
func newNoteBrowseCard(state *AppState, n StoredNote, content fyne.CanvasObject, pal palette) *searchResultCard {
	c := newSearchResultCard(state, Verse{BookName: n.Book, Chapter: n.Chapter, Verse: n.VerseLo}, content, pal)
	c.onTap = func() { openNote(state, n) }
	return c
}

// setNotesMode is the ONE place the Notes flag moves, so leaving the mode
// always HARVESTS the list's scroll position first. The browser remembers
// where the reader was for the whole session (owner, 2026-08-19: "the notes
// browser should remember its scroll position" — reversing the earlier
// start-at-the-top rule): leave by ANY route — a row tap (openNote harvests on
// its own, before this), the Done button, the sidebar/mobile mode toggles, a
// tab switch — and coming back lands in the same neighbourhood, because every
// build of the list restores notesScroll (buildNotesBrowseView). Three
// callers set this flag (the desktop sidebar toggle, the mobile toggle, and
// the mobile reset), and putting the harvest here is what makes the memory
// complete instead of route-by-route.
//
// The READER func is dropped on the way out: it aliases the list being torn
// down, and the next build installs its own. The OFFSET stays.
func setNotesMode(state *AppState, on bool) {
	if state == nil {
		return
	}
	if !on {
		if state.notesScrollRead != nil {
			state.notesScroll = state.notesScrollRead()
		}
		state.notesScrollRead = nil
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
	// match a bare canvas.Text renderer (the rule CLAUDE.md records), and a
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
