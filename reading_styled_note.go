package bibletext

// The IN-TEXT note sticker for the styled (Windows/Linux) reading pane — the
// fourth surface to draw the shared sticker composition, and the last platform
// to leave the banner behind.
//
// WHAT THE READER GETS. One card drawn IN THE TEXT, in a band reserved above
// the note's own verse, with a speech TAIL pointing down at the passage: the
// byline and the honest "K of N on this passage ›" counts on one row, the
// sender's words below, and the − / ✕ verbs at the top right — the same shape
// iOS (reading_ios.go btIOSInstallNote/btIOSLayoutNote), macOS (its twin) and
// Android (BtBridge.buildNoteBubble) already draw. It replaces a Fyne BANNER
// above the pane which was unanchored, tall, and pushed the passage down.
//
// WHY IT COULD LAND HERE AND NOWHERE ELSE WITHOUT CGO. This pane owns its own
// layout engine, so the band is a LAYOUT change (styledLayoutParams.BandVerse /
// BandH → chapterLayout.BandLine/BandY/BandH) rather than a trick over the top,
// and every part of it — the reservation, the geometry, the hit targets, the
// pixels — is testable on the dev machine.
//
// THE FOUR LESSONS THE OTHER THREE PLATFORMS PAID FOR, and where each is held:
//
//  1. The band is NOT part of any washed range. Held by geometry, in
//     layoutChapter: the band is ADVANCE (like ParaGap), so no line box covers
//     it and every wash this pane paints is bounded to a line box.
//  2. The band belongs to ONE line only. Same mechanism — the advance is added
//     once, at the anchor verse's first line, and lay.BandLine is written once.
//  3. The bubble is ONE shape. noteBubblePathSVG emits a single outline that
//     detours into the tail on its way along the bottom edge, so there is no
//     bottom border to run across the tail's mouth (notes_bubble.go records the
//     8-digit-hex defect that made the old tail invisible for months; this
//     builder shares that file's six-digit hex helper).
//  4. Selection, hit-testing and draw share ONE ruler. Every rect below is
//     placed from insetX() and the layout's own BandY, and the pane's mouse
//     handlers test the SAME table.

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fyneTheme "fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Metrics, mirroring the native stickers (reading_ios.go kNotePad/kNoteGap/
// kNoteBtn and notes_bubble.go's tail constants, which are reused as-is).
const (
	styledNotePad    = float32(12)
	styledNoteGap    = float32(10)
	styledNoteBtn    = float32(28)
	styledNoteWhoH   = float32(14)
	styledNoteWhoSz  = float32(11)
	styledNoteWhoGap = float32(4)
)

// styledNote is the pushed presentation, exactly as the three native stickers
// receive it, plus the verse the band opens above.
type styledNote struct {
	Text   string // the sender's words, alone. "" = nothing open here
	Who    string // the app's own chrome: byline + counts + "N not shown here"
	Pill   bool   // minimized, suppressed, or unplaced-only
	Next   bool   // the counts region is a CONTROL (more than one placed note)
	Anchor int    // the verse the band opens above; 0 = park at the top
}

func (n styledNote) present() bool { return n.Text != "" || n.Who != "" }

// styledStickerPush is the styled pane's alias of the shared composition — the
// same seam androidStickerPush is, and for the same reason: the who-line
// grammar, the counts, the pill labels and the derived suppression are computed
// in ONE place (appleStickerPush) so four surfaces cannot drift, and the alias
// makes that visible and host-pinnable.
func styledStickerPush(state *AppState, plan chapterPlan) (text, who string, pill, next bool) {
	return appleStickerPush(state, plan)
}

// styledNoteFor derives the sticker for the chapter the reader is on. Called
// ONCE per pane (in the constructor), never per relayout: a note change goes
// through a full reading-view rebuild, exactly as the banner does.
func styledNoteFor(state *AppState) styledNote {
	if state == nil || !notesFeatureOn(state) {
		return styledNote{}
	}
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	text, who, pill, next := styledStickerPush(state, plan)
	n := styledNote{Text: text, Who: who, Pill: pill, Next: next, Anchor: state.NoteVerseLo}
	if !n.present() {
		return styledNote{}
	}
	return n
}

// --- geometry ---------------------------------------------------------------

// styledNoteRect is one placed rectangle in the pane's own widget coordinates.
type styledNoteRect struct{ X, Y, W, H float32 }

func (r styledNoteRect) contains(p fyne.Position) bool {
	return r.W > 0 && r.H > 0 && p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

func (r styledNoteRect) pos() fyne.Position { return fyne.NewPos(r.X, r.Y) }
func (r styledNoteRect) size() fyne.Size    { return fyne.NewSize(r.W, r.H) }

// styledNoteGeom is EVERY rectangle the sticker draws and hit-tests with — one
// table, filled at measure time in band-relative coordinates and moved into
// absolute ones by place(). Draw and hit-testing read this and nothing else,
// which is what keeps them on one ruler.
type styledNoteGeom struct {
	present bool
	pill    bool
	next    bool

	// card is the whole drawn shape (the rounded card PLUS the tail below it);
	// cardH is the card alone. For a pill, card IS the pill and cardH == card.H.
	card  styledNoteRect
	cardH float32

	// The who row, split so the counts half can carry the accent and the
	// press affordance (the iOS attributed-range treatment, without an
	// attributed string).
	sender, counts, tail, restR styledNoteRect
	senderTx, countsTx          string
	tailTx, restTx              string

	body      []string
	bodyLines []styledNoteRect

	hide, del, nextHit styledNoteRect
	pillText           string
}

// bandH is what the layout must reserve: a gap, the drawn shape, and the gap to
// the verse it points at.
//
// THE GAP ABOVE IS NOT DECORATION. iOS reserves with paragraphSpacingBefore, so
// its card always inherits the anchor PARAGRAPH's own gap above it; this pane
// anchors to the verse's LINE, which is more precise (the tail points at the
// verse the note is about, not at whatever verse opened the paragraph) but
// leaves the card butting straight against the line above when the note lands
// mid-paragraph — 0pt above against 19pt below, which the owner saw at once.
// Reserving the same gap on both sides restores the rhythm iOS gets for free
// and keeps the more precise anchor. Pinned by TestStyledNoteBandIsSymmetric.
func (g styledNoteGeom) bandH() float32 {
	if !g.present {
		return 0
	}
	return styledNoteGap + g.card.H + styledNoteGap
}

// hits reports whether a position is inside the sticker at all — the guard
// every mouse handler asks before touching the selection.
func (g styledNoteGeom) hits(p fyne.Position) bool {
	return g.present && g.card.contains(p)
}

// --- measuring ---------------------------------------------------------------

// styledUISize is the chrome's body size. The bubble is the APP's furniture,
// not scripture, so it does not scale with the reader's scripture text size —
// exactly as the banner's widget.Label did not.
func styledUISize() float32 {
	if app := fyne.CurrentApp(); app != nil {
		return fyneTheme.TextSize()
	}
	return 14
}

func styledUIMeasure(s string, size float32, bold bool) fyne.Size {
	return fyne.MeasureText(s, size, fyne.TextStyle{Bold: bold})
}

// styledNoteWrap is a greedy word wrap that honours authored newlines. The
// note's own line breaks are the sender's, so they survive; width wraps are
// ours.
func styledNoteWrap(text string, width, size float32) []string {
	var out []string
	for _, para := range strings.Split(strings.TrimSpace(text), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			cand := line + " " + w
			if styledUIMeasure(cand, size, false).Width <= width {
				line = cand
				continue
			}
			out = append(out, line)
			line = w
		}
		out = append(out, line)
	}
	return out
}

// styledFitWho is the Go twin of btIOSFitWho: when the who line does not fit,
// the SENDER half is ellipsised and the counts survive whole. A reader must
// never lose "· 2 of 105 on this passage" to an ellipsis while the constant
// byline survives — the count is the honest part, the byline is recoverable
// from the bubble itself.
func styledFitWho(who string, width float32) string {
	if who == "" || width <= 0 {
		return who
	}
	if styledUIMeasure(who, styledNoteWhoSz, true).Width <= width {
		return who
	}
	i := strings.Index(who, " · ")
	if i < 0 {
		return who
	}
	sender, counts := who[:i], who[i:]
	avail := width - styledUIMeasure(counts, styledNoteWhoSz, true).Width
	r := []rune(sender)
	for len(r) > 0 {
		cand := string(r) + "…"
		if styledUIMeasure(cand, styledNoteWhoSz, true).Width <= avail {
			return cand + counts
		}
		r = r[:len(r)-1]
	}
	return "…" + counts
}

// styledWhoSplit cuts a fitted who line into the sender half, the counts span
// and whatever follows it — the same first-separator idiom btIOSWhoCountRange
// uses, and safe against a sender's name by construction: sanitizeSenderName
// maps the middle dot away (notes_byline.go), so the chrome's grammar is not
// available to names.
func styledWhoSplit(who string) (sender, counts, rest string) {
	i := strings.Index(who, " · ")
	if i < 0 {
		return who, "", ""
	}
	sender = who[:i]
	after := who[i+len(" · "):]
	if j := strings.Index(after, " · "); j >= 0 {
		return sender, " · " + after[:j], " · " + after[j+len(" · "):]
	}
	return sender, " · " + after, ""
}

// measureStyledNote sizes the sticker for a card `width` points wide and fills
// the geometry table in BAND-RELATIVE coordinates (origin = the band's top
// left). place() then moves it into the pane's own coordinates.
//
// It runs BEFORE layoutChapter, because the band's height is this measurement —
// that ordering is the whole trick, and it is a single pass with no feedback
// loop because the card width is a function of the pane's width alone, never of
// the text layout. (iOS needs a `reconciling` latch only because its container
// width is not final at measure time; here it is.)
func measureStyledNote(n styledNote, width float32) styledNoteGeom {
	var g styledNoteGeom
	if !n.present() || width < 60 {
		return g
	}
	g.present = true
	g.pill = n.Pill
	g.next = n.Next && !n.Pill

	if n.Pill {
		g.pillText = n.Who
		if g.pillText == "" {
			g.pillText = "Note"
		}
		w := styledUIMeasure(g.pillText, styledNoteWhoSz, true).Width + 28
		if w < 86 {
			w = 86
		}
		if w > width {
			w = width
		}
		g.card = styledNoteRect{X: 0, Y: 0, W: w, H: styledNoteBtn}
		g.cardH = styledNoteBtn
		return g
	}

	bodySz := styledUISize()
	inner := width - 2*styledNotePad
	if inner < 40 {
		inner = 40
	}
	whoW := inner - 2*styledNoteBtn
	if whoW < 40 {
		whoW = 40
	}

	who := n.Who
	if who == "" {
		who = "Note from Friend" // a person, never "from BibleText"
	}
	fitted := styledFitWho(who, whoW)
	sender, counts, rest := "", "", ""
	if g.next {
		sender, counts, rest = styledWhoSplit(fitted)
	} else {
		sender = fitted
	}
	// The counts span is the only pressable part of the who line; the trailing
	// "· N not shown here" is chrome too and rides on the sender's own colour.
	g.senderTx, g.countsTx, g.restTx = sender, counts, rest
	if counts != "" {
		g.tailTx = " ›"
	}

	x := styledNotePad
	whoY := styledNotePad - 2
	put := func(s string) styledNoteRect {
		if s == "" {
			return styledNoteRect{}
		}
		w := styledUIMeasure(s, styledNoteWhoSz, true).Width
		r := styledNoteRect{X: x, Y: whoY, W: w, H: styledNoteWhoH}
		x += w
		return r
	}
	g.sender = put(g.senderTx)
	g.counts = put(g.countsTx)
	g.tail = put(g.tailTx)
	g.restR = put(g.restTx)

	g.body = styledNoteWrap(n.Text, inner, bodySz)
	lineH := styledUIMeasure("Ag", bodySz, false).Height
	if lineH <= 0 {
		lineH = bodySz * 1.35
	}
	bodyY := styledNotePad + styledNoteWhoH + styledNoteWhoGap
	g.bodyLines = make([]styledNoteRect, len(g.body))
	for i := range g.body {
		g.bodyLines[i] = styledNoteRect{
			X: styledNotePad, Y: bodyY + float32(i)*lineH, W: inner, H: lineH,
		}
	}
	g.cardH = bodyY + float32(len(g.body))*lineH + styledNotePad
	g.card = styledNoteRect{X: 0, Y: 0, W: width, H: g.cardH + noteTailDepth}

	// Minimize first, delete second: the destructive one is never what a hand
	// reaches by accident (the native stickers order them the same way).
	g.hide = styledNoteRect{X: width - 2*styledNoteBtn - 2, Y: 2, W: styledNoteBtn, H: styledNoteBtn}
	g.del = styledNoteRect{X: width - styledNoteBtn - 2, Y: 2, W: styledNoteBtn, H: styledNoteBtn}
	if g.next && g.counts.W > 0 {
		// The press target is the counts span itself, widened a little.
		bx := g.counts.X - 6
		bw := g.counts.W + g.tail.W + 12
		if bx < 0 {
			bx = 0
		}
		g.nextHit = styledNoteRect{X: bx, Y: 0, W: bw, H: styledNotePad + styledNoteWhoH + 2}
	}
	return g
}

// place moves the whole table from band-relative into the pane's own widget
// coordinates. ONE call, so nothing can be placed on a different ruler.
// place moves every rect from band-relative to absolute coordinates. y is the
// BAND's top; the card hangs one gap below it (bandH's symmetry).
func (g *styledNoteGeom) place(x, y float32) {
	if !g.present {
		return
	}
	y += styledNoteGap
	shift := func(r *styledNoteRect) {
		if r.W == 0 && r.H == 0 {
			return
		}
		r.X += x
		r.Y += y
	}
	shift(&g.card)
	shift(&g.sender)
	shift(&g.counts)
	shift(&g.tail)
	shift(&g.restR)
	for i := range g.bodyLines {
		shift(&g.bodyLines[i])
	}
	shift(&g.hide)
	shift(&g.del)
	shift(&g.nextHit)
}

// --- drawing ------------------------------------------------------------------

// buildNote creates the sticker's canvas objects and appends them to the
// renderer's object list. Called at the END of rebuild() so the sticker paints
// over the glyphs, exactly as the native sticker is a subview above the text.
//
// The three controls are real widget.Buttons, not painted glyphs: that is what
// keeps them keyboard- and hover-reachable and what lets the existing
// screen-level helpers (seenBannerButton, test.Tap) drive this surface
// unchanged. The counts control is a TRANSPARENT LowImportance button laid over
// an accent-coloured canvas.Text — a Fyne button's label colour is theme-owned,
// so the accent affordance has to be painted beside it rather than in it.
func (r *styledPaneRenderer) buildNote() {
	p := r.pane
	r.noteTexts = r.noteTexts[:0]
	r.noteBtns = r.noteBtns[:0]
	r.noteCard, r.notePill = nil, nil
	g := p.noteGeom
	if !g.present {
		return
	}
	pal := p.pal

	if g.pill {
		// The collapsed marker: no tail, so it is one shape already.
		frame := canvas.NewRectangle(pal.SurfaceAlt)
		frame.StrokeColor = pal.Border
		frame.StrokeWidth = 1
		frame.CornerRadius = g.card.H / 2
		r.notePill = frame
		r.objects = append(r.objects, frame)

		btn := widget.NewButton(g.pillText, func() {
			restoreCurrentNote(p.state)
			p.state.refreshReadingOnly()
		})
		btn.Importance = widget.LowImportance
		r.noteBtns = append(r.noteBtns, btn)
		r.objects = append(r.objects, btn)
		return
	}

	if key := noteCardKey(g.card.W, g.cardH, pal); r.noteCardRes == nil || r.noteCardKey != key {
		r.noteCardRes = noteBubblePathSVG(g.card.W, g.cardH, pal.SurfaceAlt, pal.Border)
		r.noteCardKey = key
	}
	// The SVG is generated at the EXACT card size, so the stretch is 1:1 and
	// the corner radii and the tail are not distorted.
	card := canvas.NewImageFromResource(r.noteCardRes)
	card.FillMode = canvas.ImageFillStretch
	r.noteCard = card
	r.objects = append(r.objects, card)

	// The next-tap control goes in BEFORE the who texts so its hover fill can
	// never paint over the accent span it is an affordance for.
	if g.next {
		nxt := widget.NewButton("", func() {
			advanceNoteFocus(p.state)
			p.state.refreshReadingOnly()
		})
		nxt.Importance = widget.LowImportance
		r.noteBtns = append(r.noteBtns, nxt)
		r.objects = append(r.objects, nxt)
	}

	addText := func(s string, c color.Color, size float32, bold bool) {
		if s == "" {
			return
		}
		t := canvas.NewText(s, c)
		t.TextSize = size
		t.TextStyle = fyne.TextStyle{Bold: bold}
		r.noteTexts = append(r.noteTexts, t)
		r.objects = append(r.objects, t)
	}
	// The who line is the APP's chrome — never the sender's words — so it is
	// muted throughout except the counts span, which is the press target.
	addText(g.senderTx, pal.TextMuted, styledNoteWhoSz, true)
	addText(g.countsTx, pal.Accent, styledNoteWhoSz, true)
	addText(g.tailTx, pal.Accent, styledNoteWhoSz, true)
	addText(g.restTx, pal.TextMuted, styledNoteWhoSz, true)
	// The message: TEXT, never markup, in the reader's own body colour.
	bodySz := styledUISize()
	for _, line := range g.body {
		addText(line, pal.Text, bodySz, false)
	}

	// Minimize first, delete second.
	hide := widget.NewButton("–", func() { // en dash, as on the native stickers
		hideCurrentNote(p.state)
		p.state.refreshReadingOnly()
	})
	hide.Importance = widget.LowImportance
	del := widget.NewButton("✕", func() { // multiplication x
		dropCurrentNote(p.state)
		p.state.refreshReadingOnly()
	})
	del.Importance = widget.LowImportance
	r.noteBtns = append(r.noteBtns, hide, del)
	r.objects = append(r.objects, hide, del)
}

// noteCardKey identifies a generated bubble resource: regenerate only when the
// size or the colours move.
func noteCardKey(w, h float32, pal palette) string {
	return fmt.Sprintf("%.0fx%.0f|%v|%v", w, h, pal.SurfaceAlt, pal.Border)
}

// positionNote places every sticker object from the geometry table — and the
// table alone, so nothing can be placed on a different ruler than the one
// hit-testing reads.
func (r *styledPaneRenderer) positionNote() {
	g := r.pane.noteGeom
	if !g.present {
		// HIDE the surplus, never merely skip it: an object left unpositioned
		// keeps painting at its old geometry (the tint rects' "11 ghost washes"
		// lesson, position() above).
		for _, o := range r.noteObjects() {
			o.Hide()
		}
		return
	}
	for _, o := range r.noteObjects() {
		o.Show()
	}

	if g.pill {
		if r.notePill != nil {
			r.notePill.Move(g.card.pos())
			r.notePill.Resize(g.card.size())
		}
		if len(r.noteBtns) > 0 {
			r.noteBtns[0].Move(g.card.pos())
			r.noteBtns[0].Resize(g.card.size())
		}
		return
	}

	if r.noteCard != nil {
		r.noteCard.Move(g.card.pos())
		r.noteCard.Resize(g.card.size())
	}
	i := 0
	place := func(rect styledNoteRect, txt string) {
		if txt == "" || i >= len(r.noteTexts) {
			return
		}
		t := r.noteTexts[i]
		i++
		// canvas.Text draws from its top-left at its intrinsic height; centre
		// the small chrome inside the row it was measured for.
		h := t.MinSize().Height
		t.Move(fyne.NewPos(rect.X, rect.Y+(rect.H-h)/2))
		t.Resize(fyne.NewSize(rect.W, h))
	}
	place(g.sender, g.senderTx)
	place(g.counts, g.countsTx)
	place(g.tail, g.tailTx)
	place(g.restR, g.restTx)
	for li := range g.body {
		place(g.bodyLines[li], g.body[li])
	}

	b := 0
	if g.next {
		if b < len(r.noteBtns) {
			r.noteBtns[b].Move(g.nextHit.pos())
			r.noteBtns[b].Resize(g.nextHit.size())
		}
		b++
	}
	if b < len(r.noteBtns) {
		r.noteBtns[b].Move(g.hide.pos())
		r.noteBtns[b].Resize(g.hide.size())
	}
	if b+1 < len(r.noteBtns) {
		r.noteBtns[b+1].Move(g.del.pos())
		r.noteBtns[b+1].Resize(g.del.size())
	}
}

// noteObjects is every object the sticker owns, for the show/hide sweep.
func (r *styledPaneRenderer) noteObjects() []fyne.CanvasObject {
	var out []fyne.CanvasObject
	if r.noteCard != nil {
		out = append(out, r.noteCard)
	}
	if r.notePill != nil {
		out = append(out, r.notePill)
	}
	for _, t := range r.noteTexts {
		out = append(out, t)
	}
	for _, b := range r.noteBtns {
		out = append(out, b)
	}
	return out
}

// --- the one-path bubble ------------------------------------------------------

// noteBubblePathSVG is the bubble's WHOLE outline — rounded card and speech
// tail — as ONE continuous path, for a card w wide and h tall (the image itself
// is h + noteTailDepth tall, the extra being the tail).
//
// ONE path, not two shapes, for the reason btIOSNoteBubblePath and
// NoteBubbleDrawable.shape() both record: drawing a rounded rect and a triangle
// separately means the rect's bottom stroke runs straight across the mouth of
// the tail, and no z-ordering removes it. An outline that simply detours into
// the tail on its way along the bottom edge has no crossing line to hide.
//
// SIX hex digits, alpha as its own attribute — Fyne's SVG loader rejects
// #RRGGBBAA outright, and the 8-digit spelling is what made noteTailSVG's tail
// fail to load on every build for months behind one quiet log line.
func noteBubblePathSVG(w, h float32, fill, stroke color.Color) fyne.Resource {
	fillHex, fillA := noteSVGHex(fill)
	strokeHex, strokeA := noteSVGHex(stroke)

	const in = 0.5 // half the 1pt stroke, kept inside the bounds
	r := float32(noteBubbleRad)
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	tx0 := float32(noteTailInset)
	if tx0+noteTailWidth > w-r {
		tx0 = w - r - noteTailWidth
	}
	if tx0 < r {
		tx0 = r
	}
	tx1 := tx0 + noteTailWidth
	apexX := tx0 + noteTailWidth/2
	right, bottom := w-in, h-in
	apexY := bottom + noteTailDepth

	d := fmt.Sprintf("M%.1f %.1f L%.1f %.1f Q%.1f %.1f %.1f %.1f L%.1f %.1f Q%.1f %.1f %.1f %.1f "+
		"L%.1f %.1f L%.1f %.1f L%.1f %.1f L%.1f %.1f Q%.1f %.1f %.1f %.1f L%.1f %.1f Q%.1f %.1f %.1f %.1f Z",
		r+in, in, right-r, in, // top edge
		right, in, right, r+in, // top-right corner
		right, bottom-r, // right edge
		right, bottom, right-r, bottom, // bottom-right corner
		tx1, bottom, // bottom edge, right of the tail
		apexX, apexY, // down to the apex
		tx0, bottom, // back up
		r+in, bottom, // bottom edge, left of the tail
		in, bottom, in, bottom-r, // bottom-left corner
		in, r+in, // left edge
		in, in, r+in, in) // top-left corner

	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f">`+
			`<path d="%s" fill="%s" fill-opacity="%.3f" stroke="%s" stroke-opacity="%.3f" `+
			`stroke-width="1" stroke-linejoin="round"/></svg>`,
		w, h+noteTailDepth, w, h+noteTailDepth, d, fillHex, fillA, strokeHex, strokeA)
	return fyne.NewStaticResource("note-bubble.svg", []byte(svg))
}
