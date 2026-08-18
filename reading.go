package bibletext

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// buildReadingPane returns the right-hand content: search results when a search
// is active, otherwise the chapter reading view. On macOS the reading text is a
// native NSTextView overlay; setReadingOverlayVisible hides it while the search
// results are shown (no-op on other platforms).
func buildReadingPane(state *AppState) fyne.CanvasObject {
	if state.IsSearching {
		setReadingOverlayVisible(false)
		return buildSearchResultsView(state)
	}
	setReadingOverlayVisible(true)
	return buildReadingView(state)
}

func buildReadingView(state *AppState) fyne.CanvasObject {
	chapterNumbers := state.Bible.GetChapterNumbersForBook(state.CurrentBook)
	normalizeCurrentChapter(state, chapterNumbers)
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)

	// The scrollable text area is platform-specific: a Fyne chapterText (with
	// drag-selection) on Linux/Windows, and a native NSTextView overlay (with
	// the system selection menu) on macOS — see reading_fyne.go / reading_macos.go.
	paper := readingScrollArea(state, verses, state.pal())

	top := container.NewVBox()
	if bar := buildHistoryBar(state); bar != nil {
		top.Add(bar)
	}
	if state.CanReturnToSearchResults {
		top.Add(backToResultsBar(state))
	}
	top.Add(chapterHeader(state, chapterNumbers))
	// The chapter's shared note, when one is stored: the desktop/Android answer
	// to the iOS in-text sticker (notes_banner.go). Above the pane rather than
	// inside it, so the native overlays and the styled pane need no per-platform
	// note machinery to reach feature parity.
	if banner := buildNoteBanner(state); banner != nil {
		top.Add(banner)
	}

	// One uniform pad around the whole pane keeps the header and the page on the
	// same left/right margin.
	return container.NewPadded(container.NewBorder(top, nil, nil, nil, paper))
}

// chapterHeader renders the book + chapter heading, a small inline copy icon,
// the chapter picker with prev/next arrows clustered beside it, and a focus
// (distraction-free) toggle on the right, followed by a divider. It mirrors the
// iOS chapter toolbar, adapted to desktop sizing.
//
//	┌─────────────────────────────────────────────────────┐
//	│ Genesis 1 ⧉                                    ⤢    │
//	│ Chapter 1 of 50 ▾   ←  →                            │
//	└─────────────────────────────────────────────────────┘
func chapterHeader(state *AppState, chapterNumbers []int) fyne.CanvasObject {
	pal := state.pal()
	total := len(chapterNumbers)

	// "Book N ⌄" — one cohesive tap target (text + a clear dropdown chevron) that
	// opens the combined reference picker (book list + chapter grid).
	const titleBoxH = 38
	ref := newReferenceButton(fmt.Sprintf("%s %d", state.CurrentBook, state.CurrentChapter), pal.Text, headingTextSize, titleBoxH, func() {
		showChapterPicker(state)
	})

	// Small copy icon tucked beside the heading — close to the text it copies.
	var copyBtn *iconTapButton
	copyBtn = newIconTapButton(state, theme.ContentCopyIcon(), 17, titleBoxH, func() {
		copyChapter(state)
		copyBtn.flashIcon(theme.ConfirmIcon(), 1200*time.Millisecond)
	})
	titleRow := container.NewHBox(ref, hgap(8), copyBtn)

	const navBoxH = 34

	// Quiet chapter context below the heading — also a picker target, so the
	// whole "Chapter N of M" line opens the picker too.
	chapText := fmt.Sprintf("Chapter %d of %d", state.CurrentChapter, total)
	if total <= 1 {
		chapText = fmt.Sprintf("Chapter %d", state.CurrentChapter)
	}
	chapterLine := newTapTextStyled(chapText, pal.TextMuted, subheadingTextSize, navBoxH, false, func() {
		showChapterPicker(state)
	})

	idx := indexOf(chapterNumbers, state.CurrentChapter)

	prev := newIconTapButton(state, theme.NavigateBackIcon(), 20, navBoxH, func() {
		if moveChapter(state, -1) {
			state.refresh()
		}
	})
	prev.disabled = idx <= 0

	next := newIconTapButton(state, theme.NavigateNextIcon(), 20, navBoxH, func() {
		if moveChapter(state, 1) {
			state.refresh()
		}
	})
	next.disabled = idx < 0 || idx >= total-1

	// The chapter line and arrows sit directly in the HBox (no spacer-VBox
	// wrapper): each control carries its own boxH so they share a baseline, and
	// the picker anchor needs a first-class hit box rather than a nested one.
	chapterRow := container.NewHBox(chapterLine, hgap(8), prev, next)

	// Focus toggle on the right: enter distraction-free reading (hide the
	// sidebar + app header) or, when already in it, restore the full layout.
	focusIcon := theme.ViewFullScreenIcon()
	if state.IsFullScreen {
		focusIcon = theme.ViewRestoreIcon()
	}
	focusBtn := widget.NewButtonWithIcon("", focusIcon, func() {
		state.IsFullScreen = !state.IsFullScreen
		rebuildWindow(state)
	})
	focusBtn.Importance = widget.LowImportance

	// Play this chapter's audio (recorded where available, otherwise read aloud
	// where a speech engine exists). Every platform has an audio engine now —
	// native on iOS/macOS/Android, oto (recordings only) on Windows/Linux — so
	// the button hides only when chapterAudioAvailable() says this chapter has
	// nothing playable. Clustered with the focus toggle, sharing the arrows'
	// baseline.
	var rightControls fyne.CanvasObject = focusBtn
	if chapterAudioAvailable(state) {
		rightControls = container.NewHBox(audioControl(state, navBoxH), hgap(8), focusBtn)
	}

	left := container.NewVBox(titleRow, chapterRow)
	right := container.NewVBox(layout.NewSpacer(), rightControls, layout.NewSpacer())
	row := container.NewBorder(nil, nil, left, right, nil)

	rule := canvas.NewLine(pal.Border)
	rule.StrokeWidth = 1
	return container.NewVBox(row, rule)
}

// --- Shared compact toolbar controls -----------------------------------------
//
// iconTapButton and chapterPickerAnchor were first written for the iOS reading
// header; they're shared here so the desktop chapter toolbar can use the same
// small, low-chrome controls.

// minTapTarget is the smallest comfortable touch target (Apple HIG ~44pt) —
// the audio card's transport row is sized to it. The reading header's own boxes
// are smaller (boxH 36 on iOS; the two-line header can't fit 44pt rows); the
// picker text anchors get generous horizontal padding (tapTextHPad) instead.
const (
	minTapTarget float32 = 44
	tapTextHPad  float32 = 18
)

// iconTapButton is a small, low-chrome tappable icon — lighter than
// widget.Button (no background, no fixed padding). The icon is rendered at
// iconSize, centred inside a box of boxH height so it can line up vertically
// with adjacent text of a different size. A disabled button renders faint and
// ignores taps.
type iconTapButton struct {
	widget.BaseWidget
	state    *AppState
	icon     fyne.Resource
	iconSize float32
	boxH     float32
	disabled bool
	onTapped func()

	img      *canvas.Image // the rendered glyph, for in-place icon swaps
	flashGen int           // supersession guard for overlapping flashes
}

func newIconTapButton(state *AppState, icon fyne.Resource, iconSize, boxH float32, onTapped func()) *iconTapButton {
	b := &iconTapButton{state: state, icon: icon, iconSize: iconSize, boxH: boxH, onTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

// flashIcon swaps the glyph for d, then restores the original — the pressed
// feedback for fire-and-forget actions with no visible result of their own
// (the copy-chapter button: tap → checkmark → back). A generation counter
// keeps rapid re-taps from restoring early; the timer marshals back through
// fyne.Do. Safe if the window rebuilt meanwhile (the old widget is detached
// and the restore touches only it).
func (b *iconTapButton) flashIcon(res fyne.Resource, d time.Duration) {
	b.flashGen++
	gen := b.flashGen
	b.setIcon(res)
	time.AfterFunc(d, func() {
		fyne.Do(func() {
			if b.flashGen == gen {
				b.setIcon(b.baseIcon())
			}
		})
	})
}

// baseIcon is the glyph the button was constructed with.
func (b *iconTapButton) baseIcon() fyne.Resource { return b.icon }

// setIcon repaints the rendered image in place (CreateRenderer runs once per
// widget, so the canvas.Image it built is the one on screen).
func (b *iconTapButton) setIcon(res fyne.Resource) {
	if b.img == nil {
		return
	}
	b.img.Resource = theme.NewColoredResource(res, colorNameMuted)
	b.img.Refresh()
}

func (b *iconTapButton) Tapped(*fyne.PointEvent) {
	if b.disabled || b.onTapped == nil {
		return
	}
	b.onTapped()
}

func (b *iconTapButton) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(theme.NewColoredResource(b.icon, colorNameMuted))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(b.iconSize, b.iconSize))
	b.img = img
	if b.disabled {
		img.Translucency = 0.6 // faint when there's no chapter to move to
	}
	// GridWrap pins the cell to a fixed size; NewCenter vertically centres the
	// smaller icon within that box so it aligns with neighbouring text. The cell
	// is at least as wide as it is tall, so a small glyph still gets a square,
	// finger-friendly hit area rather than a thin sliver.
	w := b.iconSize + 16
	if w < b.boxH {
		w = b.boxH
	}
	box := container.NewGridWrap(fyne.NewSize(w, b.boxH), container.NewCenter(img))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*iconTapButton)(nil)

// tapText is a small tappable bit of bold text with a solid GridWrap hit box.
// A bare canvas.Text renderer is not reliably matched by Fyne's mobile-driver
// tap hit-test (it once left the chapter picker unresponsive on iOS); pinning
// the text inside a fixed-size cell gives a full-height hit rectangle and
// vertically centres it against taller neighbouring controls. Used for the
// tappable book name and chapter number in the reading header.
type tapText struct {
	widget.BaseWidget
	text  string
	tint  color.NRGBA
	size  float32
	boxH  float32
	bold  bool
	onTap func()
}

// newTapText makes a bold tappable label (the book + chapter heading).
func newTapText(text string, tint color.NRGBA, size, boxH float32, onTap func()) *tapText {
	return newTapTextStyled(text, tint, size, boxH, true, onTap)
}

// newTapTextStyled is newTapText with control over the weight, so the quiet
// "Chapter N of M" line can be tappable without going bold.
func newTapTextStyled(text string, tint color.NRGBA, size, boxH float32, bold bool, onTap func()) *tapText {
	t := &tapText{text: text, tint: tint, size: size, boxH: boxH, bold: bold, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapText) CreateRenderer() fyne.WidgetRenderer {
	lbl := canvas.NewText(t.text, t.tint)
	lbl.TextSize = t.size
	lbl.TextStyle = fyne.TextStyle{Bold: t.bold}
	// Pad the hit box well beyond the glyphs so the heading is an easy phone
	// target, not a thin strip the width of the text. The extra width goes on the
	// RIGHT — the text is pinned to the box's LEFT edge (and vertically centred),
	// the same way referenceButton pins the book heading, so the "Chapter N of M"
	// line sits flush under the book title above it instead of drifting ~half the
	// pad to the right the way a centred label would.
	w := fyne.MeasureText(t.text, t.size, lbl.TextStyle).Width + tapTextHPad
	box := container.NewGridWrap(fyne.NewSize(w, t.boxH),
		container.NewVBox(layout.NewSpacer(),
			container.NewHBox(container.NewCenter(lbl), layout.NewSpacer()),
			layout.NewSpacer()))
	return widget.NewSimpleRenderer(box)
}

func (t *tapText) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// Make sure Fyne dispatches taps to us.
var _ fyne.Tappable = (*tapText)(nil)

// referenceButton is the tappable book+chapter heading ("John 10 ⌄") that opens
// the combined reference picker. It renders the bold reference text followed
// immediately by a clear, full-size dropdown chevron — one cohesive, unambiguous
// affordance in a single comfortable hit box. (It replaces a heading + a separate
// tiny muted caret, which read as a small ambiguous mark floating a wide gap from
// the text.)
type referenceButton struct {
	widget.BaseWidget
	text  string
	tint  color.NRGBA
	size  float32
	boxH  float32
	onTap func()
}

func newReferenceButton(text string, tint color.NRGBA, size, boxH float32, onTap func()) *referenceButton {
	b := &referenceButton{text: text, tint: tint, size: size, boxH: boxH, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *referenceButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *referenceButton) CreateRenderer() fyne.WidgetRenderer {
	style := fyne.TextStyle{Bold: true}
	lbl := canvas.NewText(b.text, b.tint)
	lbl.TextSize = b.size
	lbl.TextStyle = style
	textW := fyne.MeasureText(b.text, b.size, style).Width

	// A solid dropdown chevron in the heading colour — far clearer as a "tap to
	// change book/chapter" affordance than a tiny muted caret. Sized generously so
	// the visible glyph reads big (the icon SVG carries some internal whitespace).
	chevSize := b.size * 0.8
	chev := canvas.NewImageFromResource(theme.NewColoredResource(theme.MenuDropDownIcon(), theme.ColorNameForeground))
	chev.FillMode = canvas.ImageFillContain
	chev.SetMinSize(fyne.NewSize(chevSize, chevSize))

	// Text then chevron, tight (one theme pad between them — not a wide gap). The
	// content is pinned to the LEFT (and vertically centred) inside the fixed-height
	// hit box, so the heading's left edge is flush with the box — it doesn't drift
	// right the way a centred layout would.
	inner := container.NewHBox(container.NewCenter(lbl), container.NewCenter(chev))
	w := textW + chevSize + theme.Padding()
	box := container.NewGridWrap(fyne.NewSize(w, b.boxH),
		container.NewVBox(layout.NewSpacer(), container.NewHBox(inner, layout.NewSpacer()), layout.NewSpacer()))
	return widget.NewSimpleRenderer(box)
}

var _ fyne.Tappable = (*referenceButton)(nil)

// rebuildWindow swaps in a fresh CreateMainUI tree. Use this (rather than
// state.refresh(), which only repaints the reading pane) when a change affects
// the whole window chrome — e.g. entering or leaving the distraction-free
// reading mode, which hides/shows the sidebar (desktop) or bottom tabs and
// header (mobile). afterRebuild is a build-tagged hook: a no-op on desktop,
// and an overlay re-pin on iOS.
// windowRebuildGen counts full window rebuilds (UI goroutine only). Popup
// close-out handlers capture it at open: when a rebuild drained the popup, the
// window was already rebuilt from live preferences, so the handler's own
// "changed while open" rebuild/refresh work is a duplicate and is skipped.
var windowRebuildGen uint64

func rebuildWindow(state *AppState) {
	if state.app == nil || state.window == nil {
		return
	}
	// A background data swap that landed while a sheet was open deferred its
	// rebuild (applyFullDownload); ANY full rebuild satisfies it. Down the
	// flag first — that is what makes consumeDeferredFullRebuild's own call
	// non-recursive — and honour the seed-parked link the deferred path was
	// carrying, so this rebuild paints the shared passage it was waiting on.
	if state.fullRebuildDeferred {
		state.fullRebuildDeferred = false
		consumeSeedParkedLink(state)
	}
	if os.Getenv("BT_SHEET_DEBUG") != "" {
		if _, file, line, ok := runtime.Caller(1); ok {
			fmt.Fprintf(os.Stderr, "[sheet] rebuildWindow from %s:%d\n", filepath.Base(file), line)
		}
	}
	windowRebuildGen++
	// Belt-and-braces: drain any lingering overlays before swapping content. On
	// mobile, SetContent only reassigns the content tree — it never touches the
	// overlay stack — so a modal still on the stack at rebuild time would be
	// stranded (its widget subtree kept alive, and any ProgressBarInfinite inside
	// it left spinning, repainting the canvas forever). Every modal already closes
	// before navigating, so this is normally a no-op; it makes the invariant
	// explicit and survives any future path that rebuilds without closing first.
	// Stop any infinite progress bars inside an overlay BEFORE evicting it:
	// Hide() hides the popup but a running ProgressBarInfinite animation keeps
	// marking the canvas dirty (~20fps full-tree repaints) until its owner's
	// completion path finally lands. Stop() is idempotent, so the owner's own
	// later stop is harmless.
	var stopInfiniteBars func(fyne.CanvasObject)
	stopInfiniteBars = func(o fyne.CanvasObject) {
		switch t := o.(type) {
		case *widget.ProgressBarInfinite:
			t.Stop()
		case *fyne.Container:
			for _, c := range t.Objects {
				stopInfiniteBars(c)
			}
		case *container.Scroll:
			stopInfiniteBars(t.Content)
		case *widget.PopUp:
			stopInfiniteBars(t.Content)
		}
	}
	cnv := state.window.Canvas()
	for o := cnv.Overlays().Top(); o != nil; o = cnv.Overlays().Top() {
		stopInfiniteBars(o)
		if p, ok := o.(*widget.PopUp); ok {
			// Hide (not bare Remove) so Visible() flips false: the popup
			// watchdog timers (settings sheet, go-to picker, audio menu, Ask
			// sheet) poll Visible() to run their close/restore duties — a
			// bare Remove left them polling forever after every rebuild.
			p.Hide()
			if cnv.Overlays().Top() == o {
				cnv.Overlays().Remove(o) // defensive: Hide should have done this
			}
			continue
		}
		cnv.Overlays().Remove(o)
	}
	// A popup's own restore may not fire (or not yet — the watchdogs poll on
	// 150-200ms timers), and a drained modal already called
	// state.hideReadingOverlay() on open — leaving the native reading view
	// latched hidden (a blank verse pane that survives tab switches). Clear
	// the latch here (idempotent with any late watchdog restore);
	// afterRebuild re-asserts the correct visibility for the rebuilt view.
	if state.showReadingOverlay != nil {
		state.showReadingOverlay()
	}
	state.window.SetContent(CreateMainUI(state.app, state, state.window))
	afterRebuild(state)
}

// lastPushedChapterFP is the fingerprint of the chapter currently held by the
// ANDROID reading overlay, and by nothing else any more.
//
// It is the COMBINED question — chapterRenderFingerprint, body and wash together
// — which is the only question Android can ask: its TextView is handed a whole
// `Spanned`, so there is no way to change a wash except by re-rendering. The
// Apple panes used to share this variable and now ask in two halves instead
// (lastPushedBodyFP / lastPushedTintFP, reading_tint_apple.go), because a wash
// change there is one attribute over a known range of the string already on
// screen and should not pay for a rebuild. Written only from the UI goroutine;
// unused on the platforms with no native overlay at all (Linux/Windows).
var lastPushedChapterFP string

// reporterMeasureEm is the U.S. Reports text measure in ems, measured from a
// Supreme Court slip opinion (302.6pt line ÷ 11pt body = 27.5em → 58-60
// characters per line). The iPad reading pane centres a column of exactly
// this measure — at the Normal 21px base that is ~577pt, which on an 11"
// iPad in portrait reproduces the octavo page's ~15.7%-per-side margins
// almost exactly (and 21px itself is within 4% of the printed page's
// physical type size at that screen's points-per-inch). Scaling the text
// setting scales the measure with it, so the line always wraps at the
// reporter's character count.
const reporterMeasureEm = 27.5

// chapterRenderFingerprint captures everything buildChapterHTML's output depends
// on, so the native push can detect a no-op. It MUST include the theme variant
// (colours are inlined) and the highlight identity (arriving at the same chapter
// from a search hit vs. prev/next is the same book+chapter but renders the
// highlighted verse differently).
func chapterRenderFingerprint(state *AppState) string {
	return chapterFingerprint(state, chapterTint(state).fingerprint())
}

// tintSlotFolded stands in the tint's slot when the caller is asking for the
// BODY identity alone. Any constant would do; it is spelled as an impossible
// tint fingerprint so a body fingerprint can never collide with a real render's.
const tintSlotFolded = "-"

// chapterBodyFingerprint is the identity of everything on the pane EXCEPT what
// each verse is washed in.
//
// It exists because the two halves have completely different repair costs. A
// change to the body — a different chapter, a light/dark flip, a data swap —
// can only be applied by rebuilding the HTML and re-importing the whole
// NSAttributedString, which on Psalm 119 is tens of milliseconds. A change to
// the WASH is one attribute over a known character range on the attributed
// string that is already there: no rebuild, no re-import, no scroll to
// re-assert. Folding both into one string, as this file did until now, made the
// cheap change pay the expensive change's price — every "clear highlight" tap,
// every arriving mark, every note focus once notes go plural.
//
// So the Apple panes compare the two SEPARATELY (pushChapterHTML,
// newMacReadingHost): body differs → rebuild; body same and tint differs →
// applyNativeTint, a live range mutation. Android has no such primitive (the
// Java TextView is handed a whole Spanned), so it keeps asking the combined
// question through chapterRenderFingerprint.
func chapterBodyFingerprint(state *AppState) string {
	return chapterFingerprint(state, tintSlotFolded)
}

// chapterFingerprint is the one format both questions are asked in, with the
// tint arriving as an argument rather than being read here — so the body and
// the whole-render identity cannot drift apart into two hand-maintained lists
// of what a render depends on.
func chapterFingerprint(state *AppState, hl string) string {
	var variant fyne.ThemeVariant
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}
	red := 0
	if redLetterEnabled() {
		red = 1
	}
	// THE TINT SOURCE FOLDS ITSELF (tint.go), and it is handed IN rather than
	// read here. This clause used to read the mark out of AppState and format it
	// at this call site, which is fine while the tint IS the mark and wrong the
	// moment it stops being: a fingerprint assembled at the call site is a
	// fingerprint that a new tint input can be added without.
	// chapterTints.fingerprint is written beside the fields it folds, so the
	// function that widens the tint is the function that widens this.
	//
	// state.Bible's pointer identity is part of the fingerprint: a background
	// data swap (the Gospels-seed → full download, or the stale-epoch refresh)
	// changes the TEXT without changing version/book/chapter, and the gate
	// must not skip that re-render — the reader would keep the old decode
	// (e.g. flattened poetry) until their next navigation.
	// The notes are part of what the pane draws — a note reserves a band in
	// the text and floats a sticker in it — so they are part of the identity
	// of a render. Without this the skip gate below would swallow every
	// appear, hide, restore and delete: the reader would tap "Delete note"
	// and watch nothing happen.
	//
	// SINCE S7 THE PLAN FOLDS ITSELF (notes_plan.go), replacing the
	// hand-rolled len(ActiveNote)+minimized+verse clause: the plan's note
	// half carries the display note's identity, every note's minimized state,
	// text length and resolved runs, the R4 group, and the notice — written
	// beside the fields it folds, so the function that widens the plan is the
	// function that widens this. The WASH stays in the hl slot (the plan's
	// own Fingerprint carries both halves; the body question must exclude the
	// wash so a tint change stays a live mutation, not a rebuild).
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	note := plan.noteFP
	// The one thing the plan cannot see: a mirror-only session note — an
	// arrival the store refused (NoteID 0), or one filed on another passage
	// than the reader landed on. The mirror alone carries those, so the
	// fingerprint folds the mirror's own clause for exactly those, or the
	// pane would never repaint to show them.
	if state.ActiveNote != "" &&
		(plan.display < 0 || plan.Notes[plan.display].Note.ID != state.NoteID) {
		m := 0
		if state.NoteMinimized {
			m = 1
		}
		note += fmt.Sprintf("!%d.%d.%d.%d", state.NoteID, len(state.ActiveNote), m, state.NoteVerseLo)
	}
	return fmt.Sprintf("%s|%s|%d|v%d|r%d|h%s|t%s|d%p|n%s",
		state.CurrentVersion, state.CurrentBook, state.CurrentChapter, variant, red, hl, readingTextSizeID(), state.Bible, note)
}

// --- Native-overlay chapter HTML (iOS UITextView + macOS NSTextView) ---------
//
// buildChapterHTML emits an HTML document that the AppKit/UIKit HTML importer
// turns into a richly-styled attributed string for the native text overlay
// (shared by the iOS and macOS reading views). All colours are inlined so
// light/dark mode tracks the active palette on every rebuild.
//
// The font stack leads with Georgia — a warm, screen-optimised book serif that
// is present on both macOS and iOS and matches the desktop chrome — with Iowan
// Old Style and Times as fallbacks. On phones, generous line-height + blank-line
// paragraph gaps give an unhurried feel; iPads use the U.S. Reports set — 1.3
// leading, first-line indents (see reporterLayoutActive / reporterMeasureEm).
// Kerning + ligatures + old-style numerals add a faint warmth on both.
// reporterLayout is a test seam over reporterLayoutActive. NOTE the default on
// a Mac the development environment: reporterLayoutActive is true on darwin as well as iOS (the
// desktop reads as the reporter page too — reporter_macos.go), so host tests get
// the REPORTER layout unless they pin this seam. A test that means to assert the
// phone layout must set it false explicitly.
var reporterLayout = reporterLayoutActive

func buildChapterHTML(state *AppState, verses []Verse) string {
	pal := state.pal()
	textHex := nrgbaToHex(pal.Text)
	numHex := nrgbaToHex(pal.VerseNumber)
	redLetterHex := nrgbaToHex(pal.RedLetter)
	redLetter := redLetterEnabled()
	// ONE tint answer for the whole chapter (tint.go), asked per verse below.
	// Nothing here decides what a wash looks like any more — it asks the tint,
	// and writes the markup the tint's row carries.
	tints := chapterTint(state)

	// The reader's chosen text size scales the whole page: body px here, and the
	// verse-number superscripts via their em sizing. 21px is the "Normal" base.
	bodyPx := int(math.Round(21 * readingTextScale()))
	reporter := reporterLayout()

	// Line spacing + paragraph treatment: phones keep the airy 2.0 leading with
	// blank-line paragraph gaps; the iPad reporter layout (reporterLayoutActive)
	// uses the book set measured from the U.S. Reports — 1.2 print leading
	// (opened slightly to 1.3 so the raised superscript verse numbers don't
	// perturb the line rhythm) and first-line indents with NO gap between
	// paragraphs, the octavo page's paragraph grammar. The line LENGTH half of
	// the reporter page (27.5em measure, centred) is native: the UITextView's
	// textContainerInset, driven by bibleTextSetReadingMeasure.
	lineHeight, paraCSS := "2.0", `p {
		margin: 0 0 24px 0;
		text-align: justify;
		hyphens: auto;
		-webkit-hyphens: auto;
	}`
	if reporter {
		// NOTE: no text-indent here — the AppKit/UIKit HTML importer drops it
		// (verified on the iPad sim), so the indent is a literal em+en space
		// prepended to each paragraph's text below.
		lineHeight, paraCSS = "1.3", `p {
		margin: 0;
		text-align: justify;
		hyphens: auto;
		-webkit-hyphens: auto;
	}`
	}

	var b strings.Builder
	b.WriteString("<html><head><style>")
	fmt.Fprintf(&b, `body {
		font-family: Georgia, "Iowan Old Style", "Times New Roman", serif;
		font-size: %dpx;
		color: %s;
		line-height: %s;
		letter-spacing: 0.004em;
		margin: 0; padding: 0;
		-webkit-text-size-adjust: 100%%;
		-webkit-font-smoothing: antialiased;
		font-feature-settings: "kern" 1, "liga" 1, "calt" 1, "onum" 1;
	}`, bodyPx, textHex, lineHeight)
	b.WriteString(paraCSS)
	fmt.Fprintf(&b, `sup.v {
		color: %s;
		font-weight: 600;
		font-size: 0.66em;
		letter-spacing: 0;
		margin-right: 2px;
	}`, numHex)
	// One stylesheet rule per tint that paints one, from the tint's own row
	// (appleTintHTML, tint.go) — which is where the reasons for what the rule
	// does NOT say are recorded, and where a second wash adds its own. Two
	// rules today: .hl, and .hlm for the multi-note wash — whose class no
	// markup references yet (tintMulti is unreachable; tint.go), so the rule
	// rides unused. Emitting it with the class rather than with the first use
	// is the table's own guarantee: a named class always has its rule.
	for tint := verseTint(0); tint < tintCount; tint++ {
		rule := appleTintHTML[tint].CSS
		c, ok := tint.wash(pal)
		if rule == "" || !ok {
			continue
		}
		fmt.Fprintf(&b, rule, nrgbaToHex(c))
	}
	fmt.Fprintf(&b, `.wj { color: %s; }`, redLetterHex)
	// Poetic paragraphs set ragged-right: justification would stretch short
	// poem lines full-width (TextKit does not reliably exempt forced-break
	// lines the way CSS — and Android's INTER_WORD mode — do).
	b.WriteString(`p.pm { text-align: left; }`)
	b.WriteString("</style></head><body>")

	for _, para := range groupVersesIntoParagraphs(verses) {
		poetic := false
		for _, v := range para {
			if verseIsPoetic(v.Text) {
				poetic = true
				break
			}
		}
		if poetic {
			b.WriteString(`<p class="pm">`)
		} else {
			b.WriteString("<p>")
		}
		if reporter && !verseIsPoetic(para[0].Text) {
			// The U.S. Reports paragraph grammar: a ~1.5em first-line indent
			// instead of a blank line. Emitted as em+en space characters
			// because the HTML importer ignores the text-indent CSS property.
			// Poetry is never first-line indented in print, so a paragraph
			// that OPENS on a poem line skips it — but a mixed paragraph
			// opening with prose keeps the indent (in reporter mode it is the
			// only paragraph-boundary marker; dropping it would visually
			// merge the paragraph into the previous one).
			b.WriteString("&#8195;&#8194;")
		}
		for i, v := range para {
			// The tint decides the markup, and the markup comes from the tint's
			// own row (appleTintHTML, tint.go) rather than from a branch here.
			// Each format takes exactly ONE argument — see tintHTML for why that
			// is a measurement and not a preference.
			mk := markupFor(appleTintHTML[:], tints.of(v))
			if i > 0 {
				switch {
				case poeticJoin(para[i-1].Text, v.Text):
					b.WriteString("<br>")
				case tints.joins(para[i-1], v):
					// THE JOINING SPACE IS PART OF THE BAND. Written bare it
					// belongs to neither verse's span, so a highlighted range
					// came out with a notch punched through it at every join
					// that happened to fall mid-line — observed in practice, and the
					// same hole the verse NUMBER used to leave. Joins that fall
					// at a line wrap hid it, which is why only some showed.
					//
					// tints.joins, not two "is it tinted" tests: the space is
					// inside the band only when BOTH verses are under the SAME
					// wash. Identical at k=1 (there is one tint), and the rule
					// that stops one note's band running into another's.
					b.WriteString(mk.JoinSpace)
				default:
					b.WriteByte(' ')
				}
			}
			// The number joins the wash when its verse carries one: leaving it
			// out punched a pale hole in the middle of the band, which reads as
			// a rendering fault rather than as a highlight. Which of those two
			// shapes gets written is mk.Number's business now.
			fmt.Fprintf(&b, mk.Number, v.Verse)
			// Authored poem lines become explicit <br> — a literal "\n" would
			// be ordinary HTML whitespace (renders as a space). Escape first;
			// htmlEscape leaves "\n" alone.
			// Runs, not one blob: in the BSB the words of Christ are a SPAN
			// inside the verse, so "he said" and the other speaker's reply beside
			// it must not be reddened with them (red_letter_runs.go). Every other
			// edition still yields a single run, which is the old behaviour
			// exactly.
			runs := trimRuns(redLetterRuns(state.CurrentVersion, v, redLetter))
			if len(runs) > 1 {
				for _, run := range runs {
					piece := strings.ReplaceAll(htmlEscape(run.Text), "\n", "<br>")
					writeTintedHTML(&b, mk, run.Red, piece)
				}
				continue
			}
			body := strings.ReplaceAll(htmlEscape(strings.TrimSpace(v.Text)), "\n", "<br>")
			// From the runs, NOT from isWordsOfChrist again: the edition's own
			// table has already had the final word, and asking the WEB's gate a
			// second time here would overrule it.
			writeTintedHTML(&b, mk, len(runs) == 1 && runs[0].Red, body)
		}
		b.WriteString("</p>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// nrgbaToHex formats an image/color.NRGBA as a #RRGGBB string for CSS.
func nrgbaToHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// htmlEscape escapes the characters that would break out of a content span.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func backToResultsBar(state *AppState) fyne.CanvasObject {
	pal := state.pal()
	label := state.ActiveSearchQuery
	// Same guard as buildSearchResultsView: with the assistant on "None" a
	// leftover aiSearchActive must not surface the stale AI query as the label.
	if aiFeaturesEnabled(state) && state.aiSearchActive {
		label = state.aiSearchQuery
	}
	// The query can be long (especially an AI question); Fyne buttons don't truncate,
	// so a full label overruns the bar. Keep the button short and fixed; the query is
	// shown truncated only if it's brief enough to fit.
	text := "Back to results"
	if r := []rune(label); len(r) > 0 && len(r) <= 18 {
		text = fmt.Sprintf("Results: %q", label)
	}
	// Both verbs clear through clearHighlightAndRederive, not the bare clear:
	// the search mark may have been suppressing a note on this chapter, and
	// releasing the suppression re-opens the bubble at the next render — but
	// only the projection re-raises the note's own hlNote wash (the mark the
	// search REPLACED). The bare clear left the re-opened bubble's verse
	// unwashed until the next navigation — the every-platform twin of the
	// native Clear-highlight tap (bibleTextHighlightCleared).
	back := widget.NewButtonWithIcon(text, theme.NavigateBackIcon(), func() {
		clearHighlightAndRederive(state)
		if state.surfaceSearch != nil {
			state.surfaceSearch() // mobile: jump to the real Search tab (restores its state)
		} else {
			state.IsSearching = true // desktop: results show in the reading pane
			state.refreshReadingOnly()
		}
	})
	back.Importance = widget.LowImportance

	// X dismisses the back-to-results trail and clears the search highlight.
	clear := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		state.CanReturnToSearchResults = false
		clearHighlightAndRederive(state)
		state.refreshReadingOnly()
	})
	clear.Importance = widget.LowImportance

	return surface(container.NewBorder(nil, nil, nil, clear, back), pal.SurfaceAlt, pal.Border, fyne.Size{})
}

// chapterText renders an entire chapter as one read-only, selectable text block.
// A single widget means selection (and copy) spans the whole chapter, not just a
// paragraph. It uses Wrapping=Off + Scroll=None so Fyne creates no inner scroll
// area: the block grows to its full height and reads like a printed page, while
// the surrounding page scroll handles movement. Wrapping is performed manually
// and redone on resize, so it stays responsive.
type chapterText struct {
	widget.Entry

	paragraphs [][]Verse
	// tints is the chapter's tint answer (tint.go), captured at construction —
	// the same source the other four renderers read. It replaced a
	// hand-rolled copy of isVerseHighlighted's comparison (a VerseRef, a last
	// verse and a bool, tested inline in rewrap), which is exactly the sixth
	// copy of the rule this seam exists to delete.
	//
	// This pane draws ONE band over a line RANGE and therefore cannot render a
	// second tint truthfully; when one arrives it must either grow per-line
	// rects like the styled pane or stop claiming to draw the wash. Asking the
	// shared answer is what will make that visible instead of silent.
	tints        chapterTints
	clipboard    fyne.Clipboard
	parentScroll *container.Scroll
	state        *AppState // drives the selection study menu (TappedSecondary)

	// The scripture body renders at the reader's chosen size (Settings → Text
	// size) — matching the native platforms' scaled HTML. Rendering comes from
	// readingScrollArea's size-only theme override; rewrap measures at the SAME
	// size so the wrap is exact. (The face stays the app font on purpose — see
	// readingPaneTheme for why a per-widget font override is unsound on stock
	// fyne.)
	textSize float32 // 0 ⇒ theme default (bare test constructions)

	lastWidth        float32
	highlightLine    int // first line of the highlighted range after wrapping (-1 = none)
	highlightEndLine int // last line of the highlighted range (drives the band height)
	totalLines       int

	// verseLines records each verse's first wrapped line, in chapter order —
	// the Fyne twin of the native overlays' per-verse glyph geometry. It powers
	// the within-chapter scroll anchor (capture: top-visible verse; restore:
	// scroll back to it) on the platforms without a native text view.
	verseLines []verseLine

	// hardBreakRows records the global row indexes whose trailing newline is
	// an AUTHORED poem-line break (not a soft width-wrap). copySelection uses
	// it so poetry copies as poetry while re-flowed prose copies clean — the
	// two break kinds are byte-identical in Entry.Text.
	hardBreakRows map[int]bool
}

// verseLine pairs a verse number with the wrapped line its text begins on.
type verseLine struct {
	verse int
	line  int
}

// entryScrollNone is widget.ScrollNone, assignable to Entry.Scroll as an untyped
// constant (the field's type lives in an internal package).
const entryScrollNone = 3

func newChapterText(state *AppState, verses []Verse) *chapterText {
	c := &chapterText{
		paragraphs:    groupVersesIntoParagraphs(verses),
		tints:         chapterTint(state),
		highlightLine: -1,
		state:         state,
		textSize:      theme.TextSize() * float32(readingTextScale()),
	}
	if state.window != nil {
		c.clipboard = state.window.Clipboard()
	}
	c.ExtendBaseWidget(c)
	c.MultiLine = true
	c.Wrapping = fyne.TextWrapOff
	c.Scroll = entryScrollNone // no internal scroll area is created
	c.rewrap(720)              // initial; corrected once the real width is known
	return c
}

// rewrap lays the chapter out to the given width by inserting line breaks: a
// single newline for a soft wrap and a blank line between paragraphs. It records
// the line where the highlighted verse begins so it can be scrolled into view.
func (c *chapterText) rewrap(width float32) {
	avail := width - 4*theme.InnerPadding()
	if avail < 80 {
		avail = 80
	}
	textSize := c.textSize
	if textSize <= 0 {
		textSize = theme.TextSize() // bare construction (tests) — theme default
	}
	var style fyne.TextStyle
	// Measure at the same size the pane renders with (the reading theme
	// override); the face is the app font on both sides, so the wrap is exact.
	measure := func(s string) float32 {
		return fyne.MeasureText(s, textSize, style).Width
	}
	spaceW := measure(" ")

	c.highlightLine = -1
	c.highlightEndLine = -1
	c.verseLines = c.verseLines[:0]
	c.hardBreakRows = make(map[int]bool)
	lineNo := 0
	paras := make([]string, 0, len(c.paragraphs))

	for pi, para := range c.paragraphs {
		if pi > 0 {
			lineNo++ // the blank line produced by joining paragraphs with "\n\n"
		}
		var lines []string
		var cur strings.Builder
		curW := float32(0)
		// hardBreak ends the line under construction — an authored poem-line
		// boundary rather than a width wrap. It flows through the same `lines`
		// accumulation, so verseLines/highlight/totalLines accounting stays
		// truthful (lineY's proportional model depends on that), and records
		// the row in hardBreakRows so copySelection keeps the break.
		hardBreak := func() {
			if cur.Len() > 0 {
				lines = append(lines, cur.String())
				cur.Reset()
				curW = 0
				c.hardBreakRows[lineNo+len(lines)-1] = true
			}
		}
		for vi, v := range para {
			if vi > 0 && poeticJoin(para[vi-1].Text, v.Text) {
				// Print poetry breaks at every verse boundary inside a poem.
				hardBreak()
			}
			inRange := c.tints.of(v) != tintNone
			// The band opens on the span's OWN Lo verse, which is what this
			// tested before the tint seam landed (`v.Verse ==
			// c.highlightRef.Verse`, and highlightRef.Verse was the span's Lo).
			//
			// It was briefly rewritten as "the first tinted verse we reach", on
			// the reasoning that the tinted verses are contiguous and ascend
			// from Lo so the two must agree. They do not agree when Lo is not in
			// the rendered list, and that is reachable rather than theoretical:
			// WEB and BSB omit Mark 9:44, 9:46, 11:26, Matthew 17:21, John 5:4
			// and Acts 8:37 outright, and a mark can come from a stored note's
			// span or a link fragment with nothing checking the verse exists
			// here. Old behaviour: no band at all. New: a band from the first
			// verse that IS present.
			//
			// The newer behaviour is very likely the better one — leaving
			// highlightLine at -1 while highlightEndLine is set is an incoherent
			// state that silently suppresses the wash. But S3's whole claim is
			// that it changes nothing, and this subsystem has produced six
			// defects out of improvements smuggled into commits that said they
			// were invisible. The change is filed separately, on its own
			// evidence, where it can be judged as what it is.
			hl := inRange && v.Verse == c.tints.at.Lo
			if hl {
				c.highlightLine = lineNo + len(lines)
			}
			// Provisionally record the line under construction for the scroll
			// anchor; the first-token placement below PATCHES it (and the
			// highlight) when the token wraps onto a fresh line instead of
			// joining the previous verse's partial line.
			c.verseLines = append(c.verseLines, verseLine{verse: v.Verse, line: lineNo + len(lines)})
			vlIdx := len(c.verseLines) - 1
			first := true
			for _, w := range verseTokens(v) {
				if w == "\n" {
					// Poem-line sentinel from verseTokens — never measured,
					// never emitted as text. Consecutive sentinels are an
					// authored blank poem line ("a\n\nb"): keep the empty row
					// so this pane matches the HTML surfaces (<br><br>) and
					// the share restore ("\n\n").
					if cur.Len() == 0 {
						lines = append(lines, "")
						c.hardBreakRows[lineNo+len(lines)-1] = true
					} else {
						hardBreak()
					}
					continue
				}
				ww := measure(w)
				add := ww
				if cur.Len() > 0 {
					add += spaceW
				}
				if cur.Len() > 0 && curW+add > avail {
					lines = append(lines, cur.String())
					cur.Reset()
					cur.WriteString(w)
					curW = ww
				} else {
					if cur.Len() > 0 {
						cur.WriteString(" ")
					}
					cur.WriteString(w)
					curW += add
				}
				if first {
					// The line the verse's first token ACTUALLY landed on (the
					// wrap branch above starts it one line later than recorded).
					line := lineNo + len(lines)
					c.verseLines[vlIdx].line = line
					if hl {
						c.highlightLine = line
					}
					first = false
				}
			}
			if inRange {
				// The line under construction holds this verse's final token —
				// the running end of the highlight band (extends per range verse).
				c.highlightEndLine = lineNo + len(lines)
			}
		}
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
		}
		lineNo += len(lines)
		paras = append(paras, strings.Join(lines, "\n"))
	}

	c.totalLines = lineNo + 1
	c.Entry.SetText(strings.Join(paras, "\n\n"))
}

// verseTokens splits a verse into wrap tokens, keeping the superscript number
// attached to the first word so a number never wraps onto its own line.
// Authored poem-line boundaries are preserved as "\n" sentinel tokens (a
// token can never otherwise be whitespace); rewrap turns each into a hard
// line break instead of measuring it.
func verseTokens(v Verse) []string {
	var words []string
	for li, line := range strings.Split(strings.TrimSpace(v.Text), "\n") {
		if li > 0 {
			words = append(words, "\n")
		}
		words = append(words, strings.Fields(line)...)
	}
	num := superscriptNumber(v.Verse)
	if num == "" {
		return words
	}
	if len(words) == 0 {
		return []string{num}
	}
	words[0] = num + " " + words[0]
	return words
}

// Resize re-wraps to the new width (responsive) before laying out.
func (c *chapterText) Resize(size fyne.Size) {
	if size.Width > 1 && size.Width != c.lastWidth {
		c.lastWidth = size.Width
		c.rewrap(size.Width)
	}
	c.Entry.Resize(size)
}

// highlightY is the approximate Y of the highlighted verse, for scroll-to.
func (c *chapterText) highlightY() float32 {
	if c.highlightLine < 0 || c.totalLines <= 0 {
		return 0
	}
	return float32(c.highlightLine) / float32(c.totalLines) * c.MinSize().Height
}

// highlightBand is the Y offset and height (content coordinates) of the wrapped
// lines the highlighted verse range occupies — the geometry behind the Fyne
// pane's visible highlight wash (the platforms with native text views color the
// verse itself; a single-style Entry can't, so readingColumn draws a translucent
// band over these lines instead). ok=false when nothing is highlighted.
func (c *chapterText) highlightBand() (y, h float32, ok bool) {
	if c.highlightLine < 0 || c.totalLines <= 0 {
		return 0, 0, false
	}
	end := c.highlightEndLine
	if end < c.highlightLine {
		end = c.highlightLine
	}
	lineH := c.MinSize().Height / float32(c.totalLines)
	return float32(c.highlightLine) * lineH, float32(end-c.highlightLine+1) * lineH, true
}

// lineY is the approximate top Y of a wrapped line — the same proportional
// model as highlightY (every Entry line renders at one height, so line/total
// of the content height is exact up to rounding).
func (c *chapterText) lineY(line int) float32 {
	if line <= 0 || c.totalLines <= 0 {
		return 0
	}
	return float32(line) / float32(c.totalLines) * c.MinSize().Height
}

// verseAtY reports the top-visible verse at content offset y — the last verse
// whose first line starts at or above y — plus how far past that verse's top
// the offset sits. This is the same anchor shape the native overlays capture
// (verse + delta), so the persisted position round-trips across platforms.
func (c *chapterText) verseAtY(y float32) (verse int, delta float64) {
	if y <= 0 || len(c.verseLines) == 0 {
		return 0, 0
	}
	best := verseLine{}
	for _, vl := range c.verseLines {
		if c.lineY(vl.line) <= y {
			best = vl
		} else {
			break
		}
	}
	if best.verse == 0 {
		return 0, 0
	}
	return best.verse, float64(y - c.lineY(best.line))
}

// yForVerse is verseAtY's inverse: the content Y where a verse's text begins.
// ok is false when the verse isn't in this chapter's wrap index (stale anchor
// after a translation switch trimmed the chapter, say) — callers fall back to
// the fraction anchor.
func (c *chapterText) yForVerse(verse int) (float32, bool) {
	for _, vl := range c.verseLines {
		if vl.verse == verse {
			return c.lineY(vl.line), true
		}
	}
	return 0, false
}

// Read-only: ignore typed input but keep cursor movement, selection and copy.
func (c *chapterText) TypedRune(rune) {}

func (c *chapterText) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyLeft, fyne.KeyRight, fyne.KeyUp, fyne.KeyDown,
		fyne.KeyHome, fyne.KeyEnd, fyne.KeyPageUp, fyne.KeyPageDown:
		c.Entry.TypedKey(key)
	}
}

func (c *chapterText) TypedShortcut(sc fyne.Shortcut) {
	switch sc.(type) {
	case *fyne.ShortcutCopy:
		// Copy clean text: drop the soft wraps we inserted, keep authored
		// poem lines and paragraph breaks.
		if c.clipboard != nil {
			c.clipboard.SetContent(c.copySelection())
			return
		}
		c.Entry.TypedShortcut(sc)
	case *fyne.ShortcutSelectAll:
		c.Entry.TypedShortcut(sc)
	}
}

// cleanCopy turns soft-wrap newlines back into spaces while preserving the blank
// line between paragraphs, so copied passages read naturally. It cannot tell an
// authored poem break from a width wrap (both are "\n" in Entry.Text) — the
// clipboard Copy paths use copySelection, which can; this stays the shape the
// share/AI pipeline expects (it collapses whitespace and re-derives structure).
func cleanCopy(s string) string {
	const para = "\x00"
	s = strings.ReplaceAll(s, "\n\n", para)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, para, "\n\n")
}

// copySelection returns the selected text for the clipboard: soft width-wraps
// flatten to spaces, but AUTHORED breaks survive — poem lines stay lines
// (hardBreakRows) and paragraph blanks stay blank — so poetry copies as
// poetry, matching the native overlays and the share restore. The selection is
// located in Entry.Text via the cursor (it sits at one end of a selection);
// if it cannot be located, fall back to the flatten-everything cleanCopy.
func (c *chapterText) copySelection() string {
	sel := c.SelectedText()
	if sel == "" || !strings.Contains(sel, "\n") {
		return sel
	}
	text := c.Entry.Text

	// Byte offset of the cursor from its rune-based row/column.
	off := 0
	for row := 0; row < c.CursorRow && off < len(text); row++ {
		i := strings.IndexByte(text[off:], '\n')
		if i < 0 {
			break
		}
		off += i + 1
	}
	rowEnd := strings.IndexByte(text[off:], '\n')
	if rowEnd < 0 {
		rowEnd = len(text) - off
	}
	rowStr := text[off : off+rowEnd]
	col := c.CursorColumn
	if r := []rune(rowStr); col <= len(r) {
		off += len(string(r[:col]))
	} else {
		off += len(rowStr)
	}

	// The selection is the contiguous run ending or starting at the cursor.
	start := -1
	if off >= len(sel) && text[off-len(sel):off] == sel {
		start = off - len(sel)
	} else if off+len(sel) <= len(text) && text[off:off+len(sel)] == sel {
		start = off
	} else if i := strings.Index(text, sel); i >= 0 && strings.Index(text[i+1:], sel) < 0 {
		start = i // unique occurrence — cursor bookkeeping mismatch fallback
	}
	if start < 0 {
		return cleanCopy(sel)
	}

	row := strings.Count(text[:start], "\n")
	var out strings.Builder
	for i := 0; i < len(sel); i++ {
		ch := sel[i]
		if ch != '\n' {
			out.WriteByte(ch)
			continue
		}
		if i+1 < len(sel) && sel[i+1] == '\n' {
			out.WriteString("\n\n") // paragraph (or authored blank line) pair
			row += 2
			i++
			continue
		}
		if c.hardBreakRows[row] {
			out.WriteByte('\n')
		} else {
			out.WriteByte(' ')
		}
		row++
	}
	return out.String()
}

// superToDigit reverses superscriptNumber's mapping, so a selection's rendered
// verse markers (⁴²) become plain digits (42) before matching or prompting.
var superToDigit = map[rune]rune{
	'⁰': '0', '¹': '1', '²': '2', '³': '3', '⁴': '4',
	'⁵': '5', '⁶': '6', '⁷': '7', '⁸': '8', '⁹': '9',
}

// plainSelection converts the pane's rendered selection into the shape the
// native overlays hand the study actions: soft wraps back to spaces and the
// superscript verse digits back to plain digits — so citation matching
// (selectionVerses), cross-references, and the AI prompts see ordinary text,
// identical to what a macOS/iOS/Android selection produces.
func plainSelection(s string) string {
	s = cleanCopy(strings.TrimSpace(s))
	return strings.Map(func(r rune) rune {
		if d, ok := superToDigit[r]; ok {
			return d
		}
		return r
	}, s)
}

// selectionMenu builds the study menu for the current selection — the Fyne
// twin of the native selection menus. Layout mirrors them exactly: Copy +
// Select all, then Study with AI (only when an assistant is chosen in
// Settings), Share (with citation / as image), Cross-references — and with AI
// off, Cross-references takes the study slot ahead of Share.
func (c *chapterText) selectionMenu() *fyne.Menu {
	return c.menuForSelection(plainSelection(c.SelectedText()))
}

func (c *chapterText) menuForSelection(sel string) *fyne.Menu {
	// The legacy Entry pane has no positional selection model (Entry exposes
	// only the selected STRING), so its dispatches carry the zero span and the
	// text-matching fallback attributes the verses.
	return selectionStudyMenu(c.state, sel, selSpan{},
		func() {
			if c.clipboard != nil {
				c.clipboard.SetContent(c.copySelection())
			}
		},
		func() { c.TypedShortcut(&fyne.ShortcutSelectAll{}) })
}

// selectionStudyMenu builds the desktop selection menu — Copy / Select all,
// then (with a selection) Study with AI, Share, Cross-references, in the same
// order and gating as the native menus. Shared by BOTH desktop panes
// (chapterText and the styled pane), so the verb set can never diverge. span is
// the selection's positionally-resolved verse range (zero from the legacy
// Entry pane, which has none).
func selectionStudyMenu(state *AppState, sel string, span selSpan, copyFn, selectAllFn func()) *fyne.Menu {
	copyItem := fyne.NewMenuItem("Copy", copyFn)
	copyItem.Disabled = sel == ""
	selectAll := fyne.NewMenuItem("Select all", selectAllFn)
	items := []*fyne.MenuItem{copyItem, selectAll}

	if sel != "" {
		items = append(items, fyne.NewMenuItemSeparator())

		shareChildren := []*fyne.MenuItem{
			fyne.NewMenuItem("Share with citation", func() {
				dispatchSelectionAction(state, selActionShareCite, sel, span)
			}),
			fyne.NewMenuItem("Share as image", func() {
				dispatchSelectionAction(state, selActionShareImage, sel, span)
			}),
			fyne.NewMenuItem("Share as link", func() {
				dispatchSelectionAction(state, selActionShareLink, sel, span)
			}),
		}
		// Writing a note is offered only while the feature is on, the same way
		// the AI verbs vanish when the assistant is set to None.
		if notesFeatureOn(state) {
			shareChildren = append(shareChildren, fyne.NewMenuItem("Share with note", func() {
				dispatchSelectionAction(state, selActionShareNote, sel, span)
			}))
		}
		shareItem := fyne.NewMenuItem("Share", nil)
		shareItem.ChildMenu = fyne.NewMenu("", shareChildren...)
		xrefItem := fyne.NewMenuItem("Cross-references", func() {
			dispatchSelectionAction(state, selActionCrossRef, sel, span)
		})

		if aiFeaturesEnabled(state) {
			aiItem := fyne.NewMenuItem("Study with AI", nil)
			aiItem.ChildMenu = fyne.NewMenu("",
				fyne.NewMenuItem("Explain", func() {
					dispatchAIAction(state, aiActionExplain, sel, span)
				}),
				fyne.NewMenuItem("Analyze context", func() {
					dispatchAIAction(state, aiActionContext, sel, span)
				}),
				fyne.NewMenuItem("Analyze translation", func() {
					dispatchAIAction(state, aiActionTranslation, sel, span)
				}),
			)
			items = append(items, aiItem, shareItem, xrefItem)
		} else {
			items = append(items, xrefItem, shareItem)
		}
	}

	return fyne.NewMenu("", items...)
}

// TappedSecondary replaces Entry's stock Cut/Copy/Paste menu with the study
// menu above. Cut and Paste made no sense in a read-only pane (they were
// silently inert), and this is where Share / Cross-references / Study with AI
// become reachable on the platforms without a native selection menu.
func (c *chapterText) TappedSecondary(ev *fyne.PointEvent) {
	if c.state == nil || c.state.window == nil {
		return
	}
	cnv := c.state.window.Canvas()
	if cnv == nil {
		return
	}
	widget.ShowPopUpMenuAtPosition(c.selectionMenu(), cnv, ev.AbsolutePosition)
}

// Scrolled forwards the wheel to the page so the whole chapter scrolls.
func (c *chapterText) Scrolled(ev *fyne.ScrollEvent) {
	if c.parentScroll != nil {
		c.parentScroll.Scrolled(ev)
	}
}

func (c *chapterText) CreateRenderer() fyne.WidgetRenderer {
	return &plainEntryRenderer{base: c.Entry.CreateRenderer()}
}

// plainEntryRenderer strips the entry's box and border so the text reads as prose.
type plainEntryRenderer struct{ base fyne.WidgetRenderer }

func (r *plainEntryRenderer) Destroy()                     { r.base.Destroy() }
func (r *plainEntryRenderer) Objects() []fyne.CanvasObject { return r.base.Objects() }
func (r *plainEntryRenderer) Layout(size fyne.Size)        { r.base.Layout(size); r.makePlain() }
func (r *plainEntryRenderer) Refresh()                     { r.base.Refresh(); r.makePlain() }

func (r *plainEntryRenderer) MinSize() fyne.Size {
	m := r.base.MinSize()
	if trim := theme.InputBorderSize() * 2; m.Height > trim {
		m.Height -= trim
	}
	return m
}

func (r *plainEntryRenderer) makePlain() {
	objs := r.base.Objects()
	if len(objs) < 2 {
		return
	}
	if box, ok := objs[0].(*canvas.Rectangle); ok {
		box.FillColor = color.Transparent
		box.CornerRadius = 0
		canvas.Refresh(box)
	}
	if border, ok := objs[1].(*canvas.Rectangle); ok {
		border.StrokeColor = color.Transparent
		border.StrokeWidth = 0
		border.CornerRadius = 0
		canvas.Refresh(border)
	}
}

func copyChapter(state *AppState) {
	if state.window == nil {
		return
	}
	verses := state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter)
	if len(verses) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d\n\n", state.CurrentBook, state.CurrentChapter)
	for _, v := range verses {
		// Verse.Text may carry authored poem-line breaks — kept on purpose:
		// a chapter copy is a plain-text export, and poetry copying as poetry
		// is the same principle as the cited-text share layout.
		fmt.Fprintf(&b, "%d %s\n", v.Verse, strings.TrimSpace(v.Text))
	}
	state.window.Clipboard().SetContent(b.String())
}

// readingColumn centres its single child and caps the line length for
// comfortable reading. When the child is a chapterText with a highlighted verse,
// it scrolls that verse into view during layout — on the render thread, so there
// is no goroutine and no data race.
type readingColumn struct {
	maxWidth float32
	scroll   *container.Scroll
	chapter  *chapterText
	band     *canvas.Rectangle // translucent wash over the highlighted verse (objects[1])
	scrolled bool
}

func (l *readingColumn) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	child := objects[0]

	w := size.Width
	if w > l.maxWidth {
		w = l.maxWidth
	}
	if w < 0 {
		w = 0
	}
	x := (size.Width - w) / 2
	if x < 0 {
		x = 0
	}

	// First resize sets the width so wrapping content reflows; then size to the
	// resulting height.
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Resize(fyne.NewSize(w, child.MinSize().Height))
	child.Move(fyne.NewPos(x, 0))

	// The highlight band tracks the wrapped lines of the highlighted verse. It
	// sits ON TOP of the Entry (a translucent highlighter-pen wash) because the
	// Entry paints an opaque input background; canvas primitives take no events,
	// so selection and the study menu pass straight through it.
	if l.band != nil {
		if by, bh, ok := l.bandGeometry(); ok {
			l.band.Move(fyne.NewPos(x, by))
			l.band.Resize(fyne.NewSize(w, bh))
			l.band.Show()
		} else {
			l.band.Hide()
		}
	}

	if l.scroll != nil && l.chapter != nil && l.chapter.highlightLine >= 0 && !l.scrolled {
		y := l.chapter.highlightY() - 24
		if y < 0 {
			y = 0
		}
		l.scroll.Offset = fyne.NewPos(0, y)
		l.scrolled = true
		l.scroll.Refresh()
	}

	// Apply any pending within-chapter scroll restore (reopening where the
	// reader left off / a history tap) now that sizes are real. A per-platform
	// hook: the Fyne-scroll platforms (Linux/Windows) implement it; the native-
	// overlay platforms no-op — their overlays restore themselves.
	if l.scroll != nil && l.chapter != nil {
		applyFyneReadingRestore(l)
	}
}

// bandGeometry is the highlight band's rect within the column, from the
// chapter's wrap geometry.
func (l *readingColumn) bandGeometry() (y, h float32, ok bool) {
	if l.chapter == nil {
		return 0, 0, false
	}
	return l.chapter.highlightBand()
}

func (l *readingColumn) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	// Report height only. The width is deliberately 0: the child is a Wrapping=Off
	// text whose MinSize width is its longest line. If that fed upward, the
	// enclosing HSplit would size its divider from it (split.go computeSplitLengths
	// clamps to leading/trailing minimums), and a transient narrow layout could
	// feed back and let the sidebar expand to fill the window. The real layout
	// width comes from the parent in Layout, not from here.
	return fyne.NewSize(0, objects[0].MinSize().Height)
}

func indexOf(values []int, target int) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return -1
}

// --- Paragraph grouping -----------------------------------------------------

// verseIsPoetic reports whether a verse's text carries authored poem line
// breaks (the decoder emits "\n" between poem clauses). A verse that is
// exactly ONE poem line decodes with no internal break and reads as prose
// here — the shared limitation, same as the share pipeline's.
func verseIsPoetic(text string) bool { return strings.Contains(text, "\n") }

// poeticJoin reports whether the boundary between two adjacent verses in a
// paragraph is a poetry line boundary: in print, every verse boundary inside
// a poem is also a line boundary, so a join touching a poetic verse breaks.
// The share pipeline's chapterShareStructure applies the same rule, keeping
// displayed lines and shared/restored lines identical.
func poeticJoin(prevText, curText string) bool {
	return verseIsPoetic(prevText) || verseIsPoetic(curText)
}

func groupVersesIntoParagraphs(verses []Verse) [][]Verse {
	if len(verses) == 0 {
		return nil
	}

	paragraphs := make([][]Verse, 0, len(verses)/4+1)
	current := make([]Verse, 0, 6)
	charCount := 0

	for i, verse := range verses {
		if len(current) > 0 {
			prev := current[len(current)-1]
			if shouldBreakParagraph(prev.Text, charCount) {
				paragraphs = append(paragraphs, current)
				current = make([]Verse, 0, 6)
				charCount = 0
			}
		}
		current = append(current, verse)
		charCount += len([]rune(verse.Text)) + 1

		if i == len(verses)-1 && len(current) > 0 {
			paragraphs = append(paragraphs, current)
		}
	}
	return paragraphs
}

func shouldBreakParagraph(prevVerseText string, currentParagraphChars int) bool {
	if currentParagraphChars < 320 {
		return false
	}
	trimmed := strings.TrimSpace(prevVerseText)
	return strings.HasSuffix(trimmed, ".") ||
		strings.HasSuffix(trimmed, "!") ||
		strings.HasSuffix(trimmed, "?") ||
		strings.HasSuffix(trimmed, "\"") ||
		strings.HasSuffix(trimmed, "'")
}

func superscriptNumber(n int) string {
	if n <= 0 {
		return ""
	}
	mapper := map[rune]rune{
		'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴',
		'5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	}
	var b strings.Builder
	for _, d := range fmt.Sprintf("%d", n) {
		if s, ok := mapper[d]; ok {
			b.WriteRune(s)
		}
	}
	return b.String()
}

// --- Reference picker -------------------------------------------------------
//
// One combined book + chapter picker on a single screen, opened from the reading
// header (tapping the book name or the chapter number): a scrollable book list
// on the left and a calendar-style chapter-number grid on the right that updates
// as you select a book. Tapping a chapter navigates there. Shared by desktop and
// iOS via the same header.

// fixedWidthLayout pins its content to a fixed width while filling the available
// height — used for the picker's left-hand book column.
type fixedWidthLayout struct{ width float32 }

func (f fixedWidthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objs {
		if m := o.MinSize(); m.Height > h {
			h = m.Height
		}
	}
	return fyne.NewSize(f.width, h)
}

func (f fixedWidthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(f.width, size.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}

// denseGridPadding is the chapter grid's fixed inter-cell gap (the tight look the dense
// theme used to give via 3pt padding). It is baked into denseGridWrapLayout instead of
// read from theme.Padding() on purpose: a LAYOUT resolves theme.Padding() from the
// rendering-theme stack, which only holds the override's dense value WHILE the override's
// own renderer.Layout is running. Swapping a book re-lays the grid out without that push,
// so theme.Padding() returned the app's loose 7pt default and the grid visibly spread —
// the "grid changes size when you pick a book" bug. A constant can't drift. (Widgets, by
// contrast, DO see a theme override through the per-widget cache; only layouts miss it,
// which is why no amount of Refresh() on the override fixed this.)
const denseGridPadding = 3

// denseGridWrapLayout mirrors Fyne's gridWrapLayout — fixed-size cells flowing left to
// right and wrapping to the available width — but spaces them with the constant
// denseGridPadding rather than theme.Padding(), so the spacing is identical on every relayout.
type denseGridWrapLayout struct {
	cell fyne.Size
	rows int
}

func (g *denseGridWrapLayout) cols(width float32) int {
	if width <= g.cell.Width {
		return 1
	}
	return int((width + denseGridPadding) / (g.cell.Width + denseGridPadding))
}

func (g *denseGridWrapLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	cols := g.cols(size.Width)
	g.rows = 0
	i, x, y := 0, float32(0), float32(0)
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		if i%cols == 0 {
			g.rows++
		}
		child.Move(fyne.NewPos(x, y))
		child.Resize(g.cell)
		if (i+1)%cols == 0 {
			x, y = 0, y+g.cell.Height+denseGridPadding
		} else {
			x += g.cell.Width + denseGridPadding
		}
		i++
	}
}

func (g *denseGridWrapLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	rows := g.rows
	if rows < 1 {
		rows = 1
	}
	return fyne.NewSize(g.cell.Width, g.cell.Height*float32(rows)+float32(rows-1)*denseGridPadding)
}

// referenceChapterGrid is the right pane: the chapter-number grid for one book, with the
// book name + chapter count above it. onPick fires with the chapter. It returns the grid,
// a setBook(book, selected) that repopulates it for a different book IN PLACE, a
// reselect(chapter) callback that re-highlights the selected chapter IN PLACE (just the
// buttons' importance), and scrollToSelected() which scrolls the grid so the selected
// chapter is visible (used when the keyboard shrinks the pane). setBook and reselect reuse
// the same grid + scroll, so a book change never rebuilds them.
func referenceChapterGrid(state *AppState, pal palette, book string, selected int, onPick func(int)) (fyne.CanvasObject, func(string, int), func(int), func()) {
	head := canvas.NewText("", pal.TextMuted)
	head.TextSize = 12

	// Fixed 34pt cells laid out by denseGridWrapLayout, which spaces them with a constant
	// gap (denseGridPadding) — so the grid keeps identical spacing whenever setBook swaps a
	// book's buttons in. (The old theme-override approach lost its tight padding on a swap
	// because layouts read theme.Padding() from the rendering stack, not the widget cache.)
	grid := container.New(&denseGridWrapLayout{cell: fyne.NewSize(34, 34)})
	scroll := container.NewVScroll(grid)
	btns := map[int]*widget.Button{}
	cur := selected

	// setBook fills the grid for one book IN PLACE: just the buttons + heading change.
	setBook := func(bk string, sel int) {
		nums := state.Bible.GetChapterNumbersForBook(bk)
		head.Text = fmt.Sprintf("%s · %d chapters", bk, len(nums))
		head.Refresh()
		cur = sel
		btns = make(map[int]*widget.Button, len(nums))
		objs := make([]fyne.CanvasObject, 0, len(nums))
		for _, c := range nums {
			ch := c
			btn := widget.NewButton(fmt.Sprintf("%d", ch), func() { onPick(ch) })
			btn.Importance = widget.LowImportance
			if ch == sel {
				btn.Importance = widget.HighImportance
			}
			btns[ch] = btn
			objs = append(objs, btn)
		}
		grid.Objects = objs
		grid.Refresh()
		scroll.ScrollToOffset(fyne.NewPos(0, 0)) // a new book starts at the top
	}
	setBook(book, selected)

	reselect := func(sel int) {
		cur = sel
		for ch, b := range btns {
			imp := widget.LowImportance
			if ch == sel {
				imp = widget.HighImportance
			}
			if b.Importance != imp {
				b.Importance = imp
				b.Refresh()
			}
		}
	}

	scrollToSelected := func() { scrollChildIntoView(scroll, btns[cur]) }

	obj := container.NewBorder(container.NewPadded(head), nil, nil, nil, scroll)
	return obj, setBook, reselect, scrollToSelected
}

// scrollChildIntoView scrolls a VScroll so target (a descendant of its content) is
// visible — only when it's currently outside the viewport, so it never jumps a selection
// that's already on screen. Used when the keyboard shrinks the Goto picker's panes.
func scrollChildIntoView(scroll *container.Scroll, target fyne.CanvasObject) {
	if scroll == nil || target == nil {
		return
	}
	top := target.Position().Y
	bottom := top + target.Size().Height
	viewTop := scroll.Offset.Y
	viewBottom := viewTop + scroll.Size().Height
	switch {
	case top < viewTop:
		scroll.ScrollToOffset(fyne.NewPos(0, top))
	case bottom > viewBottom:
		scroll.ScrollToOffset(fyne.NewPos(0, bottom-scroll.Size().Height))
	}
}

// pickerHeader is a stage's top row: a leading element (title, or back+title) on
// the left and a close button on the right, above a separator.
func pickerHeader(leading fyne.CanvasObject, onClose func()) fyne.CanvasObject {
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onClose)
	closeBtn.Importance = widget.LowImportance
	bar := container.NewBorder(nil, nil, leading, container.NewVBox(closeBtn, layout.NewSpacer()), nil)
	return container.NewVBox(bar, widget.NewSeparator())
}

// navigateToReference jumps to a specific book + chapter and records the visit.
func navigateToReference(state *AppState, book string, chapter int) {
	selectBook(state, book, false)
	state.CurrentChapter = chapter
	clearHighlightedVerse(state)
	addRecentChapter(state, book, chapter)
	state.refresh()
	if state.surfaceReading != nil {
		state.surfaceReading()
	}
}

// pickerCanvas returns the canvas to host a picker modal.
func pickerCanvas(state *AppState) fyne.Canvas {
	if state.window != nil {
		return state.window.Canvas()
	}
	if d := fyne.CurrentApp().Driver(); d != nil {
		if ws := d.AllWindows(); len(ws) > 0 {
			return ws[0].Canvas()
		}
	}
	return nil
}

// pickerSplitSize gives the split picker a roomy size (it needs width for the
// book column plus the chapter grid) capped to the screen.
func pickerSplitSize(cnv fyne.Canvas) (float32, float32) {
	w, h := float32(560), float32(560)
	if _, sz := cnv.InteractiveArea(); sz.Width > 0 {
		w = sz.Width - 24
		if w > 640 {
			w = 640
		}
		if w < 320 {
			w = 320
		}
		h = sz.Height * 0.8
		if h > 680 {
			h = 680
		}
		if h < 300 {
			h = 300
		}
	}
	return w, h
}

// pickerVerseSize sizes the MOBILE Goto (verse) picker: a TOP-anchored, non-modal popup
// opened near full-screen so the alphabet grid, book list and chapter grid are all
// visible. The verse row lives at the BOTTOM of the card, where the soft keyboard
// covers it — goto.go lifts it by growing a transparent spacer beneath it rather
// than resizing the popup, which is why the popup is never resized after opening
// (resizing moved the field out from under an in-flight tap and self-dismissed
// the picker).
//
// Unlike pickerSplitSize (which feeds a MODAL popup whose renderer clamps content down
// to the canvas, so an over-wide value is harmless), the non-modal renderer grows the
// popup to its content's min size with NO clamp — so we MUST size against the real
// canvas width to avoid running off the right edge. cnv.Size() is what that renderer
// clamps against; basing width on it guarantees the card fits.
func pickerVerseSize(cnv fyne.Canvas) (float32, float32) {
	cw := cnv.Size().Width
	ch := cnv.Size().Height
	if _, sz := cnv.InteractiveArea(); sz.Height > 0 {
		ch = sz.Height
	}
	w := cw - 24
	if w > 560 {
		w = 560
	}
	if w < 300 {
		w = 300
	}
	h := ch - 24 // near full-screen; show the whole grids/lists
	if h < 300 {
		h = 300
	}
	return w, h
}

func chapterPickerColumns(total int) int {
	if total <= 0 {
		return 1
	}
	columns := int(math.Ceil(math.Sqrt(float64(total))))
	if columns < 2 {
		columns = 2
	}
	if columns > 8 {
		columns = 8
	}
	return columns
}
