package bibletext

// The IN-TEXT note sticker for the styled (Windows/Linux) reading pane — the
// fourth surface to draw the shared sticker composition, and the last platform
// to leave the banner behind.
//
// WHAT THE READER GETS. One card drawn IN THE TEXT, in a band reserved above
// the note's own verse, with a speech TAIL pointing down at the passage: the
// byline and the honest "K of N in this chapter ›" counts on one row, the
// sender's words below, and the − / ✕ verbs at the top right — the same shape
// iOS (reading_ios.go btIOSInstallNote/btIOSLayoutNote), macOS (its twin) and
// Android (BtBridge.buildNoteBubble) already draw. It replaces a Fyne BANNER
// above the pane which was unanchored, tall, and pushed the passage down.
//
// WHY IT COULD LAND HERE AND NOWHERE ELSE WITHOUT CGO. This pane owns its own
// layout engine, so the band is a LAYOUT change (styledLayoutParams.BandVerse /
// BandH → chapterLayout.BandLine/BandY/BandH) rather than a trick over the top,
// and every part of it — the reservation, the geometry, the hit targets, the
// pixels — is covered by host-side tests without cgo.
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

// Metrics. Every vertical number here is READ FROM THE SHARED SPEC
// (noteMetrics, notes_bubble.go) rather than restated — this pane is the one
// surface that can consume the table directly, and it is the reference the
// three natives are held to by notes_spacing_spec_test.go.
//
// styledNoteBtn is the only local number: it is the VERB BUTTON's size, a
// pointer-target decision this platform owns (iOS 30 for a thumb, macOS 24 for
// a pointer). It is deliberately no longer the pill's height — that is
// noteMetrics().PillH, spec'd for all four.
const (
	styledNoteBtn   = float32(28)
	styledNoteWhoSz = float32(11)
)

var (
	styledNotePad    = noteMetrics().Pad
	styledNoteGapAbv = noteMetrics().GapAbove
	styledNoteGapBlw = noteMetrics().GapBelow
	styledNoteWhoH   = noteMetrics().WhoH(styledNoteWhoSz) // 14 at 11pt
	styledNoteWhoGap = noteMetrics().WhoGap
)

// styledNote is the pushed presentation, exactly as the three native stickers
// receive it, plus the verse the band opens above.
type styledNote struct {
	Text   string // the sender's words, alone. "" = nothing open here
	Who    string // the app's own chrome: byline + counts + "N not shown here"
	Pill   bool   // minimized, suppressed, or unplaced-only
	Next   bool   // the counts region is a CONTROL (more than one placed note)
	Anchor int    // the verse the band opens above; 0 = park at the top
	// Own marks an explicitly focused note authored locally. It determines the
	// closing control: a received note is deleted and uses a bin, while an own
	// note is dismissed and uses ✕. Distinct glyphs keep deletion unambiguous.
	Own bool
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
	n := styledNote{Text: text, Who: who, Pill: pill, Next: next, Anchor: state.NoteVerseLo,
		Own: plan.HasOwn && state.NoteID != 0 && state.NoteID == plan.Own.Note.ID}
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

	// anchorVerse is the verse whose paragraph this sticker belongs to. The
	// single sticker never needed it — there was one, and the layout reserved
	// one band. With a pill per paragraph the geometry and the band have to be
	// matched by identity rather than by order, because the layout drops a
	// request whose paragraph it cannot find.
	anchorVerse int
}

// bandH is what the layout must reserve: a gap, the drawn shape, and the gap to
// the verse it points at.
//
// THE GAP ABOVE IS NOT DECORATION, and both gaps are now SPEC
// (noteMetrics().GapAbove / GapBelow, notes_bubble.go) rather than this pane's
// own numbers: the first cut here reserved only below and left the card butting
// against the line above — 0pt above against 19pt below, which was obvious on
// sight. Pinned by TestStyledNoteBandIsSymmetric and by the gallery's
// per-picture spacing assertions.
func (g styledNoteGeom) bandH() float32 {
	if !g.present {
		return 0
	}
	return styledNoteGapAbv + g.card.H + styledNoteGapBlw
}

// hits reports whether a position is inside the sticker at all — the guard
// every mouse handler asks before touching the selection.
// hitsAnyPill reports whether a press landed on one of the per-paragraph
// pills. The select path asks this alongside noteGeom.hits so a tap on a pill
// is never also the start of a text selection under it.
func (p *styledReadingPane) hitsAnyPill(pos fyne.Position) bool {
	for i := range p.pillGeoms {
		if p.pillGeoms[i].hits(pos) {
			return true
		}
	}
	return false
}

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
// never lose "· 2 of 105 in this chapter" to an ellipsis while the constant
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
		w := styledUIMeasure(g.pillText, styledNoteWhoSz, true).Width + 2*noteMetrics().PillPadX
		if w < noteMetrics().PillMinW {
			w = noteMetrics().PillMinW
		}
		if w > width {
			w = width
		}
		// The pill's height is SPEC, not the verb button's size (noteMetrics
		// records why the two were ever the same number).
		g.card = styledNoteRect{X: 0, Y: 0, W: w, H: noteMetrics().PillH}
		g.cardH = noteMetrics().PillH
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
	// The who row's box starts at the card's own padding, FULL STOP. It used to
	// start at pad-2 on this pane and on both natives — a shim nobody could
	// justify, whose only effect was to make the stated rhythm (12 + 14 + 4)
	// describe a card whose real who→body gap was 6 and whose real top padding
	// was 10. The table now says what the pixels do.
	whoY := styledNotePad
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
	y += styledNoteGapAbv
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
	r.buildParagraphPills()
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

		// THE LABEL IS DRAWN, NOT LABELLED — and that is the whole difference
		// between this pill and iOS's. A widget.Button renders its title in the
		// THEME's text size and foreground colour: 18pt in the body ink on this
		// pane, where iOS draws 11pt semibold in the MUTED ink (btNoteWhoFont,
		// gNoteMuted). The pill was measured at 11pt all along
		// (measureStyledNote, styledNoteWhoSz) and then drawn at 18, so the text
		// overflowed a box sized for something smaller and shouted in a colour
		// the note's own chrome never uses — plainly wrong beside the iOS pill.
		//
		// So the button keeps the TAP and gives up the TEXT — the same split the
		// next-tap control next door already uses: an empty LowImportance button
		// over drawn text. Both halves are positioned from the one geometry
		// table in positionNote, so the hit target cannot drift off the label.
		btn := widget.NewButton("", func() {
			restoreCurrentNote(p.state)
			p.state.refreshReadingOnly()
		})
		btn.Importance = widget.LowImportance
		r.noteBtns = append(r.noteBtns, btn)
		r.objects = append(r.objects, btn)

		label := canvas.NewText(g.pillText, pal.TextMuted)
		label.TextSize = styledNoteWhoSz
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Alignment = fyne.TextAlignCenter
		r.noteTexts = append(r.noteTexts, label)
		r.objects = append(r.objects, label)
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

	// Minimize first, delete second — but NOT ON YOUR OWN NOTE, where − and ✕
	// run the identical three lines (hideCurrentNote / dropCurrentNote) and −
	// promises a pill an own note can never have: it enters the plan only while
	// focus names it, built Open. See the iOS twin for the full account.
	r.noteHasHide = !p.note.Own
	var hide *widget.Button
	if r.noteHasHide {
		hide = widget.NewButton("–", func() { // en dash, as on the native stickers
			hideCurrentNote(p.state)
			p.state.refreshReadingOnly()
		})
		hide.Importance = widget.LowImportance
	}
	// THE GLYPH SAYS WHAT THE PRESS DOES. A bin where it deletes — someone
	// else's message, read, and the store is yours to prune — and ✕ where it
	// only puts your own note away, which is what navigating away would do a
	// moment later anyway. The same mark for both would have made the
	// destructive one the ambiguous one.
	del := widget.NewButton("", func() {
		dropCurrentNote(p.state)
		p.state.refreshReadingOnly()
	})
	if p.note.Own {
		del.SetText("✕") // multiplication x: dismiss, never destroy
	} else {
		// THE APP'S OWN DELETE ICON, not an emoji. An emoji is set as a TITLE,
		// so it renders at the button's font size in its own colours — which is
		// how a small quiet control became a large loud one. The theme icon is
		// tinted like the rest of the card's furniture and is sized by the
		// theme, which the sticker keeps deliberately small and subtle.
		del.SetIcon(fyneTheme.DeleteIcon())
	}
	del.Importance = widget.LowImportance
	if hide != nil {
		r.noteBtns = append(r.noteBtns, hide)
		r.objects = append(r.objects, hide)
	}
	r.noteBtns = append(r.noteBtns, del)
	r.objects = append(r.objects, del)
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
	r.positionParagraphPills()
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
		// The label centres in the pill on BOTH axes, as iOS's chip does
		// (chip.frame = gNoteView.bounds on a UIButton, which centres its
		// title). Horizontally that is Alignment centre over the pill's full
		// width; vertically it is the same "canvas.Text draws from its top-left
		// at its intrinsic height" correction the card's chrome uses below.
		if len(r.noteTexts) > 0 {
			t := r.noteTexts[0]
			h := t.MinSize().Height
			t.Move(fyne.NewPos(g.card.X, g.card.Y+(g.card.H-h)/2))
			t.Resize(fyne.NewSize(g.card.W, h))
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
	if r.noteHasHide {
		if b < len(r.noteBtns) {
			r.noteBtns[b].Move(g.hide.pos())
			r.noteBtns[b].Resize(g.hide.size())
		}
		b++
	}
	if b < len(r.noteBtns) {
		r.noteBtns[b].Move(g.del.pos())
		r.noteBtns[b].Resize(g.del.size())
	}
}

// noteObjects is every object the sticker owns, for the show/hide sweep.
// noteObjects is the SINGLE sticker's objects and only those. The
// per-paragraph pills are deliberately absent: positionNote hides everything
// this returns whenever the single sticker is not present, which is exactly
// when the pills ARE the collapsed state. Putting them here positioned them
// and then hid them one line later — reserved bands with nothing in them.
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
	r := float32(noteStickerRad)
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

// receivedShownAs names HOW this chapter's received notes are represented on
// the page. It exists because the invariant needs it: the set must be
// represented exactly once (docs/NOTES_STATE.md, N9), and "exactly once" cannot
// be stated without a value to count. Before it was named, the rule was spelled
// as "do any pills exist?" — a question about the pills rather than about the
// set — and that is how opening the reader's own note came to represent the set
// zero times.
type receivedShownAs int

const (
	shownAsNothing receivedShownAs = iota // no received notes here to represent
	shownAsSticker                        // the chapter-wide collapsed chip, "Notes · N"
	shownAsPills                          // one pill per noted paragraph
	shownAsCount                          // the open received note's "K of N in this chapter"
)

func (r receivedShownAs) String() string {
	return [...]string{"nothing", "sticker", "pills", "count"}[r]
}

// receivedSetShownAs is the ONE answer, so the pane and the enumeration cannot
// disagree about what the reader is looking at. groups is the number of noted
// paragraphs chapterNoteGroups found, which is zero whenever the pill gate is
// off — so the gate needs no separate argument here.
// stickerIsTheReceivedSet reports whether the single sticker IS this chapter's
// received notes in their collapsed form — the ONE state in which the pills are
// a second spelling of it and must replace it.
//
// styledNote.Pill alone is not that question. Pill means "the sticker is
// CLOSED", and a focused own note is closed too whenever a foreign mark
// suppresses the chapter — appleStickerPush's own-note arm returns pill=true
// under notesSuppressed (notes_plan.go). Keying on Pill therefore treated the
// reader's own note as the friends' chip: reachable by arriving at the chapter
// through a search result and then opening your own note, which blanked it.
func stickerIsTheReceivedSet(note styledNote) bool {
	return note.Pill && !note.Own
}

func receivedSetShownAs(plan chapterPlan, note styledNote, groups int) receivedShownAs {
	if len(plan.Notes) == 0 || !note.present() {
		return shownAsNothing
	}
	// OWN FIRST, and the order is the fix rather than a preference: Own says
	// WHAT the sticker is showing, Pill only says whether it is closed, and the
	// two overlap under suppression. The narrower fact has to win.
	switch {
	case note.Own:
		// The sticker is busy with a note that is NOT in this set and carries no
		// count of it, so only the pills can speak for the set — at any group
		// count, including one. Where there is no pill row (the three native
		// surfaces, or the gate off) groups is zero and the set is represented
		// nowhere: X16, and the reason this function returns a value rather
		// than a bool.
		if groups >= 1 {
			return shownAsPills
		}
		return shownAsNothing
	case note.Pill:
		// The sticker IS the set's collapsed form. The pills replace it only
		// where they say strictly more: two or more noted paragraphs. With one,
		// the sticker's count and its position already agree.
		if groups >= 2 {
			return shownAsPills
		}
		return shownAsSticker
	default:
		// An open received note: its who line carries the count.
		return shownAsCount
	}
}

// measureParagraphPills builds the collapsed state as one pill per noted
// paragraph: the band request the layout needs, and the geometry the pane will
// place once the layout says where each band landed.
//
// Empty unless notesPillPerParagraph is on AND the state is actually collapsed.
// An open note is a bubble about ONE passage and is unaffected by this; the
// pills are only ever the closed state, which is the state whose single pill
// could not say where the chapter's notes were.
func (p *styledReadingPane) measureParagraphPills(width float32) ([]bandRequest, []styledNoteGeom) {
	if p == nil || p.state == nil || !notesPillPerParagraph {
		return nil, nil
	}
	plan := buildChapterPlan(p.state, appPrefs(), p.state.Bible)
	groups := chapterNoteGroups(p.state, plan, p.verses)
	// One question, asked in one place: is this the state in which the pills are
	// how the received set is represented? Every reason to draw or not draw them
	// lives in receivedSetShownAs, so the enumeration can ask the same question
	// without re-deriving the answer and drifting from it.
	if receivedSetShownAs(plan, p.note, len(groups)) != shownAsPills {
		return nil, nil
	}
	req := make([]bandRequest, 0, len(groups))
	geoms := make([]styledNoteGeom, 0, len(groups))
	for _, g := range groups {
		n := styledNote{
			Pill:   true,
			Who:    stickerPillWho(len(g.Notes), 0),
			Anchor: g.BandVerse,
		}
		geom := measureStyledNote(n, width)
		geom.anchorVerse = g.BandVerse
		req = append(req, bandRequest{Verse: g.BandVerse, H: geom.bandH(), Count: len(g.Notes)})
		geoms = append(geoms, geom)
	}
	return req, geoms
}

// buildParagraphPills draws the collapsed state as one pill per noted
// paragraph. Same shape as the single pill next door — a rounded frame, drawn
// text at the muted 11pt the note chrome uses, and an empty LowImportance
// button over it for the tap — repeated per paragraph, each labelled with its
// own count.
func (r *styledPaneRenderer) buildParagraphPills() {
	p := r.pane
	r.pillFrames, r.pillLabels, r.pillBtns = nil, nil, nil
	if len(p.pillGeoms) == 0 {
		return
	}
	pal := p.pal
	for i := range p.pillGeoms {
		g := p.pillGeoms[i]
		frame := canvas.NewRectangle(pal.SurfaceAlt)
		frame.StrokeColor = pal.Border
		frame.StrokeWidth = 1
		frame.CornerRadius = g.card.H / 2
		r.pillFrames = append(r.pillFrames, frame)
		r.objects = append(r.objects, frame)

		// Tapping a paragraph's pill opens THAT paragraph's note, which is the
		// whole reason the pills are per paragraph: the single pill could only
		// ever open whichever note the plan had chosen.
		verse := g.anchorVerse
		btn := widget.NewButton("", func() {
			focusNoteAtVerse(p.state, verse)
			p.state.refreshReadingOnly()
		})
		btn.Importance = widget.LowImportance
		r.pillBtns = append(r.pillBtns, btn)
		r.objects = append(r.objects, btn)

		label := canvas.NewText(g.pillText, pal.TextMuted)
		label.TextSize = styledNoteWhoSz
		label.TextStyle = fyne.TextStyle{Bold: true}
		label.Alignment = fyne.TextAlignCenter
		r.pillLabels = append(r.pillLabels, label)
		r.objects = append(r.objects, label)
	}
}

// positionParagraphPills places every pill from its own geometry, and HIDES
// any surplus rather than leaving it unpositioned — an object left behind
// keeps painting where it last was.
func (r *styledPaneRenderer) positionParagraphPills() {
	p := r.pane
	for i := range r.pillFrames {
		if i >= len(p.pillGeoms) {
			r.pillFrames[i].Hide()
			continue
		}
		g := p.pillGeoms[i]
		place := func(o fyne.CanvasObject, rect styledNoteRect) {
			o.Move(fyne.NewPos(rect.X, rect.Y))
			o.Resize(fyne.NewSize(rect.W, rect.H))
			o.Show()
		}
		place(r.pillFrames[i], g.card)
		if i < len(r.pillBtns) {
			place(r.pillBtns[i], g.card)
		}
		if i < len(r.pillLabels) {
			// Centred on BOTH axes, by the same rule the single pill uses:
			// horizontally by Alignment over the card's full width, vertically
			// by the text's INTRINSIC height. A canvas.Text draws from its
			// top-left at MinSize().Height, which exceeds TextSize by the
			// ascender and descender — centring on the point size instead sat
			// the word ~2pt low in its frame.
			lbl := r.pillLabels[i]
			h := lbl.MinSize().Height
			lbl.Move(fyne.NewPos(g.card.X, g.card.Y+(g.card.H-h)/2))
			lbl.Resize(fyne.NewSize(g.card.W, h))
			lbl.Show()
		}
	}
}
