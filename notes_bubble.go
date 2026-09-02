package bibletext

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The note bubble, as the reading page draws it.
//
// ONE BUILDER, used by the reading banner and by the notes browser, because the
// two surfaces have to match and two hand-built lookalikes drift the moment
// either is touched.
//
// ── THE NOTE SPACING SPEC ────────────────────────────────────────────────────
//
// ONE table, four surfaces. Before 19 Aug 2026 each of the four note surfaces
// carried its own numbers and they disagreed on every one of them — the band
// above the card was 0 (iOS), a measured line (macOS), 8dp (Android) and 10
// (this pane); the pill was 30 / 24 / ~26 / 28 for the same object; and
// Android's card used a different internal rhythm again (6 top, 4 right, 10
// bottom, and a who row whose height was set by a verb glyph). The pill and the
// note's vertical spacing and margins have to measure the same on every one of
// the four surfaces, so the numbers live HERE and every surface reads them.
//
// The natives cannot import a Go constant into Objective-C or Java, so they
// carry named constants of their own and notes_spacing_spec_test.go PARSES
// those three sources and asserts each literal equals this table (the mechanism
// dev_links_guard_test.go uses on the release scripts). A push parameter was
// the alternative and was rejected: these are compile-time layout constants,
// not per-chapter data, and pushing them would put a wire format and a
// three-platform ABI bump in front of every future 1pt change.
//
// WHAT EACH NUMBER MEANS:
//
//	GapAbove   the air RESERVED above the card's top edge.
//	GapBelow   the air reserved below the drawn shape — the tail's apex for a
//	           card, the pill's bottom edge for a pill — to the first line of
//	           the passage. This is THE PINNED INVARIANT: it is the one
//	           distance a reader consciously reads (tail → the words it points
//	           at), so macOS places the sticker bottom-up off the passage and
//	           lets every measurement error fall into the gap above, where
//	           there is already air.
//	Pad        the card's inner padding, all four sides.
//	WhoGap     the who row's bottom edge → the message's first line.
//	WhoH       NOT a literal: ceil(whoSize × noteWhoRatio), because the correct
//	           box for an 11pt semibold system face (14) is the wrong box for a
//	           10pt one (13, macOS). A flat 14 in this table would be a bug on
//	           macOS.
//	Radius     the card's corner radius. Was 8 on this pane alone — borrowed
//	           from surface()'s chrome, never chosen for this card, while the
//	           three natives all drew 10.
//	PillH      the collapsed marker's height, spec'd INDEPENDENTLY of the verb
//	           buttons. All four used to derive it from their own button metric
//	           (30 iOS touch minimum, 24 macOS pointer, ~26 Android wrap, 28
//	           here), which is a touch-target decision leaking into a piece of
//	           content: the pill must not change height because a platform
//	           moved its tap targets.
//	PillPadX   the pill's own SIDE padding, and deliberately not Pad: the pill
//	           is one line of small chrome and wants more air beside its text
//	           than a paragraph of body does, which is why iOS wrote 14 here
//	           and 12 there. Each surface had picked its own (iOS 14, macOS 12,
//	           Android 12) so the same label came out three widths.
//	PillMinW   the width floor, so a short label ("Note") still reads as a
//	           deliberate object rather than a shrink-wrapped tag. Android had
//	           NO floor at all until 19 Aug; macOS floored at 76.
//
// THE PILL IS MORE THAN ITS BOX, and the rest of it is a contract too — held by
// notes_spacing_spec_test.go's shape checks rather than by this table, because
// they are not numbers:
//
//	label font   the WHO font, semibold (11pt on iOS/Android/styled, 10 on
//	             macOS). MEASURE AND DRAW MUST AGREE: the styled pane measured
//	             the pill at the who size and then let a widget.Button draw the
//	             title at the THEME's size and foreground ink — 18pt body ink —
//	             so the text was two-thirds larger than the box sized for it.
//	             That is why the styled pane now draws a canvas.Text and keeps
//	             the button as a transparent hit target (reading_styled_note.go).
//	label ink    MUTED, everywhere. The who line is the APP's chrome — a byline
//	             and a count — never the sender's words, and it is muted on all
//	             four surfaces so the sender's own text is the only thing on the
//	             sticker in the reading ink.
//	label align  CENTRED in the box, on both axes. With a width floor the box is
//	             wider than a short label, so alignment became visible: iOS and
//	             macOS centre by construction (a button title inside bounds),
//	             Android needed Gravity.CENTER rather than CENTER_VERTICAL, and
//	             the styled pane sets TextAlignCenter.
//
// WHAT THIS TABLE DOES NOT OWN — the residual, stated rather than pretended.
// GapAbove is a RESERVATION. The paragraph separator the reading layout puts
// between paragraphs belongs to the reading page, not to the note: it is the
// same for every paragraph in the chapter, note or no note, and the
// RESERVATION never cancels it — a noted paragraph keeps at least its
// neighbours' separation. It is 24px on iOS phones, 0 in the reporter layout
// (iPad/macOS/wide styled pane), one blank line on Android, and ParaGap on
// the narrow styled pane. The boundary is recorded in
// docs/NOTES_SPEC.md#sticker-spacing.
//
// What a reader SEES above the shape depends on WHICH shape:
//
//   - An OPEN card hangs GapAbove below its band's top, so its air above is
//     GapAbove + the separator, and the tail's distance to the passage stays
//     the pinned GapBelow. The card never centres: the tail must point at
//     the words, and lifting the card away from them breaks the one distance
//     a reader consciously reads.
//   - A COLLAPSED pill stack whose bottom neighbour is the PASSAGE centres in
//     the inter-paragraph air a reader SEES — the previous paragraph's ink
//     bottom to the noted paragraph's first ink top. Engines that split
//     their leading evenly around the glyphs (the styled pane centres them
//     in the line box; CSS half-leading is symmetric by spec) implement
//     that as box arithmetic — notePillSeparatorLift (separator/2) above
//     the band top, air = separator/2 + GapAbove each side. The natives
//     measure the ink instead (btIOSPillStackInkTop, btPillStackInkTop):
//     their imports pile the leading ABOVE each line's glyphs, so the box
//     answer sat visibly low, worse at larger text sizes. Where the
//     separator is 0 (the reporter layouts, the chapter-top inset) every
//     form stands down and the pill sits GapAbove into its band exactly as
//     before — one rule, no layout branch.
//   - A pill stack whose bottom neighbour is an OPEN card (the own-note-open
//     co-tenancy) does NOT lift: the symmetry argument is about the air
//     between two paragraphs, and there the card owns the bottom air.
//
// The lift is a PLACEMENT term only. The reservation amounts above never
// change with it, so the text never moves and the take-back arithmetic
// never sees it.
//
// ── THE FOUR RULES (they hold on every surface; break one and the tests bite) ─
//
//  1. THE BAND OPENS ABOVE THE PARAGRAPH carrying the highlighted verse, never
//     between two of that paragraph's lines. A card wedged mid-paragraph splits
//     the passage in half, and the scripture text must never be interrupted
//     mid-flow. (Whether it should open BELOW that paragraph instead is TABLED
//     and undecided — docs/NOTES_SPEC.md#future-work.)
//  2. THE BAND IS RESERVED SPACE, NOT LINE HEIGHT. It must be advance the
//     layout adds between line boxes, never height added to one — see the
//     techniques below for what each platform reserves with.
//  3. NO WASH MAY REACH INTO THE BAND. A verse's highlight covers that verse's
//     own glyphs; the reserved air is not part of any mark. Rule 2 is what
//     makes rule 3 structural rather than a guard: if the band belongs to no
//     line box, no line's background can paint it.
//  4. THE PILL IS ITS OWN OBJECT. Its height is PillH, never a verb button's
//     size; the reader's marker must not resize because a platform moved its
//     tap targets.
//
// ── HOW EACH PLATFORM RESERVES THE BAND (four engines, one result) ───────────
//
//	iOS      NSParagraphStyle.paragraphSpacingBefore on the anchor PARAGRAPH
//	         (reading_ios.go, btIOSInstallNote). TextKit reserves nothing
//	         finer, which is why rule 1 reads the way it does — the paragraph
//	         is the only granularity this API has. PECULIARITY: the spacing
//	         COLLAPSES at the top of a text container, so a note on the
//	         chapter's first paragraph is reserved with the container's top
//	         inset instead (gNoteTopInset) — a real defect once: the bubble sat
//	         on top of the opening verses.
//	macOS    the same mechanism (reading_macos.go, btMacInstallNote) PLUS a
//	         measured top-gap correction (btMacNoteTopGap, floored at
//	         GapAbove): at the reporter leading the previous line's ink
//	         overhangs its own fragment box, so a bare GapAbove read tight on
//	         real pixels. It is a MEASUREMENT correction, not a design choice —
//	         the one residual difference between the four, named here and in
//	         docs/NOTES_SPEC.md#sticker-spacing. macOS also
//	         places the sticker BOTTOM-UP off the passage, which is what makes
//	         GapBelow exact there and lets error land in the air above.
//	Android  a LineHeightSpan on the character BEFORE the paragraph, growing
//	         THAT line's descent (android/BtBridge.java, applyNoteBand +
//	         NoteBandSpan). TWO TRAPS, both paid for on the emulator and both
//	         invisible to any compile check:
//	           (a) LineHeightSpan is a PARAGRAPH span — chooseHeight runs for
//	               EVERY line of the paragraph, so an unguarded adjustment
//	               inflates all of them.
//	           (b) Android reuses ONE FontMetricsInt across those calls, so
//	               returning early is NOT neutral: the previous line's
//	               adjustment is still in the object and the next line inherits
//	               it. The span must put the metrics BACK on the following call.
//	         Reserving in the PRECEDING line's descent (rather than the anchor
//	         line's ascent) is what keeps rule 3: the verse's own background
//	         would otherwise grow with its inflated line and slide under the
//	         card.
//	Styled   advance added to the running y at the paragraph's top, exactly as
//	         ParaGap is (reading_styled_layout.go — BandVerse/BandH in,
//	         BandLine/BandY/BandH/BandLastLine out). No line's Y or H changes,
//	         the wrap and the selection text model stay byte-identical, and the
//	         chapter grows by exactly the band. This is the cheapest engine of
//	         the four and the only one where rules 2 and 3 are free.
//
// ── PLATFORM PECULIARITIES WORTH KNOWING BEFORE YOU CHANGE ANYTHING ──────────
//
//	Android units  dp tracks display density; sp tracks the READER's font-size
//	               setting. A spec height applied as a fixed dp box around sp
//	               text clips once the reader raises that setting (Android's
//	               slider reaches 1.3, accessibility 2.0). Spec heights are
//	               therefore FLOORS there (setMinHeight), never setHeight.
//	Android text   the who line is "<byline> · K of N in this chapter ›" with
//	               the counts at the END, so END-ellipsis eats exactly the half
//	               a reader must not lose. fitWho (Java) is btIOSFitWho's rule:
//	               the sender half gives way, the counts survive whole.
//	Apple fonts    fixed system faces, no Dynamic Type on this surface, so the
//	               WhoH ratio resolves once at build time (14 at iOS's 11pt, 13
//	               at macOS's 10pt).
//	Styled sizes   the chrome does NOT scale with the reader's scripture text
//	               size — the bubble is the app's furniture, not scripture.
//
// ── REPRODUCING WHAT YOU SEE ────────────────────────────────────────────────
//
//	Styled (Windows/Linux), 28 pictures, no device needed:
//	  BIBLETEXT_PANE_SNAPSHOT_DIR=/tmp/g go test -run TestStyledNoteGallery ./
//	  → 14 permutations × light/dark (one/two/three notes, pill, suppression,
//	    first verse, the 280-rune cap, narrow, wide, multi-verse, poetry, and a
//	    no-note control). Each asserts geometry BEFORE writing its PNG.
//	The same pane, live, on macOS:
//	  BIBLETEXT_MIMIC=linux go run -tags bibletextdev ./cmd/desktop
//	iOS simulator, three notes arriving on one passage:
//	  SIMCTL_CHILD_BIBLETEXT_DEV_NOTES=s10next xcrun simctl launch <udid> uk.co.bibletext
//	Android emulator: scripts/build-android.sh, install, then deliver a real
//	  link — adb shell am start -a android.intent.action.VIEW -d "<share url>"
//	  (docs/ANDROID.md has the emulator recipe).
//
// ── ADJUSTING IT ────────────────────────────────────────────────────────────
//
// Change the number HERE, then run, in this order:
//
//	go test -run 'TestNativeNoteSpacing|TestNoteSpacingShape' ./
//	   → fails per native source that still carries the old literal, naming the
//	     file and the constant. Update those three, and nothing else.
//	go test -run TestStyledNoteGallery ./        → the styled pane's geometry
//	scripts/view-test-gate.sh                    → the planted-defect gate
//	then LOOK: the gallery snapshots, and the platforms you changed.
//
// The parser can see that a native's CONSTANT matches this table and that its
// SHAPES are right (the band formulas, the card-height formula, macOS's
// bottom-up placement, the who row's floor). It cannot see a native computing
// something correct-looking from correct constants in a place nobody pinned —
// so when you add a new use site, add a `required` fragment for it too.
//
// THE TAIL is what makes it read as somebody speaking rather than as a card.
// Its geometry is copied from the native sticker (reading_macos.go,
// btMacNoteBubblePath): nine points deep, eighteen wide, twenty-four in from the
// left, pointing DOWN at the passage.
const (
	noteGapAbove  = 10
	noteGapBelow  = 10
	notePad       = 12
	noteWhoGap    = 4
	noteWhoRatio  = 1.27 // whoH = ceil(whoSize × this): 14 at 11pt, 13 at 10pt
	noteTailDepth = 9
	noteTailWidth = 18
	noteTailInset = 24
	// The FYNE bubble's corner — the notes browser's rows and the reading
	// banner. It is surface()'s radius on purpose (theme.go), so a bubble sits
	// inside the app's other cards without a different curve; the audio button,
	// the crossref panel, the search rows and the version picker are all 8 too.
	// The STICKER's own corner is noteStickerRad (the native panes use 10);
	// they were briefly one constant, which quietly moved the browser and the
	// banner away from the shared card radius.
	noteBubbleRad = 8
	// The in-text sticker's corner, shared by all four note surfaces.
	noteStickerRad = 10
	notePillH      = 28
	// The pill's own side padding and width floor. They are NOT the card's Pad:
	// the pill is a single line of small chrome and wants more air beside its
	// text than a paragraph does, which is why iOS wrote 14 here and 12 there.
	// Every surface used to pick its own pair — iOS 14/86, macOS 12/76, Android
	// 12 with NO floor at all, the styled pane 14/86 copied from iOS — so the
	// same "Notes · 3" came out four different widths. Same failure as PillH
	// before it, same fix: one number, four readers.
	notePillPadX = 14
	notePillMinW = 86
	// THE ARRIVAL LEAD: how far below the top of the viewport the thing being
	// arrived at is placed, so it does not kiss the edge. Five surfaces each
	// picked their own — 12 and 16 on the Apple panes (the split had no stated
	// reason), 24 on the styled pane, dp(16) on Android, and 1.2rem/6.5rem in
	// the web reader's scroll-margin — so the same arrival sat at four
	// different heights depending on which app you were holding.
	//
	// ONE number, and deliberately not one per class: two numbers put a
	// per-class decision back inside the renderer, which is the thing being
	// removed. If the classes ever must differ, that is a change to the
	// classifier, not to five files.
	noteArrivalLead = 16
)

// noteSpacing is the spec above as one value, so a consumer reads a NAMED
// field rather than a bare constant and the spec can be passed around whole.
type noteSpacing struct {
	GapAbove  float32
	GapBelow  float32
	Pad       float32
	WhoGap    float32
	TailDepth float32
	TailWidth float32
	TailInset float32
	Radius    float32
	PillH     float32
	PillPadX  float32
	PillMinW  float32
	Lead      float32
}

// --- the shared geometry decisions ------------------------------------------

// noteMeasure is the PLATFORM half of the sticker's geometry: the numbers only
// a renderer can know — its fonts, its verb button. Handed to the functions
// below so the DECISIONS (what a band reserves, what a card stacks, how a pill
// sizes, where a who line gives way) are made here, once, and are enumerable
// on a host with a stub measurer instead of a device.
type noteMeasure struct {
	BodyH float32              // one body line's height in this renderer's body font
	WhoSz float32              // the who row's (and the pill label's) font size
	Btn   float32              // one verb button's edge
	TextW func(string) float32 // width of a run at WhoSz, bold — the who row's own measure
}

// notePillW is the collapsed pill's width: the label plus the spec's side
// padding, floored at the spec's minimum, then capped at the space available —
// in that order, so a narrow pane still narrows the pill.
func notePillW(m noteMeasure, label string, maxW float32) float32 {
	w := m.TextW(label) + 2*noteMetrics().PillPadX
	if w < noteMetrics().PillMinW {
		w = noteMetrics().PillMinW
	}
	if w > maxW {
		w = maxW
	}
	return w
}

// noteCardH is the expanded card's height WITHOUT its tail: padding, the who
// row, the gap to the body, the body's lines, padding.
func noteCardH(m noteMeasure, bodyLines int) float32 {
	return noteMetrics().Pad + noteMetrics().WhoH(m.WhoSz) + noteMetrics().WhoGap +
		float32(bodyLines)*m.BodyH + noteMetrics().Pad
}

// noteBandH is what a layout reserves for a drawn shape: the spec's gap on
// BOTH sides (the symmetry rule every pane relearned separately), the shape,
// and the tail's depth when the card points at a passage.
func noteBandH(shapeH float32, hasTail bool) float32 {
	h := noteMetrics().GapAbove + shapeH + noteMetrics().GapBelow
	if hasTail {
		h += noteMetrics().TailDepth
	}
	return h
}

// notePillSeparatorLift is the collapsed stack's centering rule (the doctrine
// above says which shapes it applies to and why the card is exempt): a pill
// stack lifts half the paragraph separator above its band top, so the air on
// each side of the stack reads separator/2 + GapAbove. At separator 0 — the
// reporter layouts, the chapter-top inset — the lift is 0 and placement is
// unchanged, which is why no caller needs a layout branch. The styled pane
// calls this directly; the natives mirror the /2 with their own separator
// reads (each platform's separator lives in its own text engine), and
// notes_spacing_spec_test.go pins those mirrors to this spelling.
func notePillSeparatorLift(separator float32) float32 {
	return separator / 2
}

// noteFitWho fits a who line into a width. The SENDER half gives way to an
// ellipsis and everything from the first separator on survives whole: the
// counts are the honest part, and the byline is recoverable from the bubble
// itself. In the degenerate case — no width even for one sender rune — the
// answer is the ellipsis plus that surviving tail, never a line that lost its
// counts. A line that already fits, or has no separator to split at, returns
// unchanged.
func noteFitWho(m noteMeasure, who string, width float32) string {
	if who == "" || width <= 0 {
		return who
	}
	if m.TextW(who) <= width {
		return who
	}
	i := strings.Index(who, " · ")
	if i < 0 {
		return who
	}
	sender, counts := who[:i], who[i:]
	avail := width - m.TextW(counts)
	r := []rune(sender)
	for len(r) > 0 {
		cand := string(r) + "…"
		if m.TextW(cand) <= avail {
			return cand + counts
		}
		r = r[:len(r)-1]
	}
	return "…" + counts
}

// noteMetrics is THE table. Every surface reads this or is held to it by
// notes_spacing_spec_test.go.
//
// A function, not a package var: a var would let any code — or any test — assign
// to the spec at runtime, and a spec that can be reassigned is not a spec
// at all.
func noteMetrics() noteSpacing { return noteSpacingTable }

var noteSpacingTable = noteSpacing{
	GapAbove:  noteGapAbove,
	GapBelow:  noteGapBelow,
	Pad:       notePad,
	WhoGap:    noteWhoGap,
	TailDepth: noteTailDepth,
	TailWidth: noteTailWidth,
	TailInset: noteTailInset,
	Radius:    noteStickerRad,
	PillH:     notePillH,
	PillPadX:  notePillPadX,
	PillMinW:  notePillMinW,
	Lead:      noteArrivalLead,
}

// WhoH is the who row's box height for a given who-line font size — derived,
// never a literal, so the same rule serves iOS's 11pt semibold (14) and macOS's
// 10pt one (13).
func (s noteSpacing) WhoH(size float32) float32 {
	return float32(math.Ceil(float64(size) * noteWhoRatio))
}

// noteSVGHex spells a colour the way Fyne's SVG loader will accept it: SIX hex
// digits, alpha as its own attribute.
//
// Fyne rejects #RRGGBBAA outright ("color string ... is not length 3 or 6"),
// and the original 8-digit spelling here meant the tail image FAILED TO LOAD on
// every build since the bubble shipped — the bubble rendered as a plain card
// and the failure was one quiet log line in a test nobody grepped. It is shared
// with the styled pane's one-path builder (noteBubblePathSVG,
// reading_styled_note.go) so the second SVG author cannot re-learn it.
func noteSVGHex(c color.Color) (string, float64) {
	r, g, b, a := c.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8), float64(a>>8) / 255
}

// noteTailSVG draws the tail: filled, with the two SLANTED edges stroked and
// the mouth left open so it merges into the bubble above it.
//
// The top row of the fill deliberately overlaps the bubble's own bottom border
// by a point, and the tail is drawn after the bubble, so the border does not
// run visibly across the tail's mouth — which is what would make it read as a
// triangle stuck to a box rather than as one shape.
func noteTailSVG(fill, stroke color.Color) fyne.Resource {
	fillHex, fillA := noteSVGHex(fill)
	strokeHex, strokeA := noteSVGHex(stroke)
	// noteTailLidOverlap is how far the tail's fill reaches UP over the card's
	// bottom border. ONE point was not enough: magnified against the reading
	// pane's one-path bubble, the border still showed as a hairline lid straight
	// across the tail's mouth, so the tail read as a triangle stuck under a
	// closed box. The stroke is 1pt but lands on a fractional device pixel at
	// 2x and 3x, so hiding it needs more than 1pt of cover.
	//
	// The alternative — drawing card and tail as ONE path, the way the reading
	// pane does — is more correct and was tried. It costs a full-size SVG
	// RASTERISATION per distinct bubble height, and most rows have a distinct
	// height because each message wraps differently: measured on the real
	// browser, first paint went 350ms to 456ms at ten notes. A rounded rectangle
	// is drawn natively and an 18x11 tail is nothing, which is why this
	// construction is cheap — so the fix belongs in the overlap, not the shape.
	// THE GEOMETRY, because getting it wrong is invisible in code and obvious on
	// screen. The image is w wide and (lid + d) tall, and it is positioned so
	// that y = lid lands exactly on the card's bottom edge:
	//
	//	y = 0     ─┐  the LID strip: fill only, no stroke. It covers the card's
	//	           │  own bottom border across the mouth's width, which is what
	//	y = lid   ─┘  stops the border reading as a hairline across the tail.
	//	              the card's bottom edge — the mouth
	//	              the two slanted edges, stroked, meeting at
	//	y = lid+d     the point.
	//
	// My first attempt drew the triangle from y=0 and left the spare height at
	// the BOTTOM, so the whole tail sat a lid too high: its mouth was inside the
	// card, its strokes cut across the card's interior, and it hung 6pt instead
	// of 9. The fill hid the border either way, which is why it looked nearly
	// right and was not: on screen the tail read as a shape with no border.
	w, d, lid := noteTailWidth, noteTailDepth, noteTailLidOverlap
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+
			`<path d="M0 0 L%d 0 L%d %d L%d %d L0 %d Z" fill="%s" fill-opacity="%.3f"/>`+
			`<path d="M0 %d L%d %d L%d %d" fill="none" stroke="%s" stroke-opacity="%.3f" `+
			`stroke-width="1" stroke-linejoin="round"/></svg>`,
		w, lid+d, w, lid+d,
		w, w, lid, w/2, lid+d, lid, fillHex, fillA,
		lid, w/2, lid+d, w, lid, strokeHex, strokeA,
	)
	return fyne.NewStaticResource("note-tail.svg", []byte(svg))
}

// noteTailLidOverlap is the cover that hides the card's bottom border behind the
// tail's fill. The SVG draws this much extra above the tail's mouth and the
// layout lifts the image by the same amount — one number, two readers.
const noteTailLidOverlap = 3

// bubbleLayout puts the tail under the bubble's lower-left, overlapping its
// border by enough to hide it.
type bubbleLayout struct{}

func (bubbleLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	m := objects[0].MinSize()
	return fyne.NewSize(m.Width, m.Height+noteTailDepth)
}

func (bubbleLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	bodyH := size.Height - noteTailDepth
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, bodyH))
	if len(objects) < 2 {
		return
	}
	objects[1].Move(fyne.NewPos(noteTailInset, bodyH-noteTailLidOverlap))
	objects[1].Resize(fyne.NewSize(noteTailWidth, noteTailDepth+noteTailLidOverlap))
}

// noteBubble draws somebody's words the way the reading page draws them.
//
// The text inside is UNTRUSTED — another person's message. It is a Label, never
// markup, never RichText, and it is never the app speaking: whatever wraps this
// must attribute it. noteBubbleWithByline does that; use it unless you have a
// reason not to.
func noteBubble(text string, pal palette) fyne.CanvasObject {
	return noteBubblePadded(text, pal, theme.Padding())
}

// noteBubblePadded is noteBubble with the card's inner padding as a parameter.
// The bubble's IDENTITY — the rounded bordered card and the tail — is the
// same everywhere, so the browser's bubbles match the reading pane's; its
// DENSITY is not: the notes browser packs rows (browseBubblePad,
// notes_browse.go) while the reading banner keeps the page's own
// theme.Padding(), which is what the default above passes. surface() cannot be
// reused for the tight case because container.NewPadded reads the GLOBAL theme
// padding, out of reach of any row-scoped override.
func noteBubblePadded(text string, pal palette, pad float32) fyne.CanvasObject {
	body := widget.NewLabel(strings.TrimSpace(text))
	body.Wrapping = fyne.TextWrapWord
	return noteBubbleAround(body, pal, pad)
}

// noteBubbleAround is noteBubblePadded for a caller that already owns the
// label: the notes browser's reusable row, which refills ONE label per note
// rather than building a bubble per note. The shape — frame, radius, stroke,
// tail — is built here and only here, so a refilled bubble and a freshly built
// one cannot drift apart.
func noteBubbleAround(body fyne.CanvasObject, pal palette, pad float32) fyne.CanvasObject {
	frame := canvas.NewRectangle(pal.SurfaceAlt)
	frame.StrokeColor = pal.Border
	frame.StrokeWidth = 1
	frame.CornerRadius = noteBubbleRad
	card := container.NewStack(frame,
		container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), body))
	tail := canvas.NewImageFromResource(noteTailSVG(pal.SurfaceAlt, pal.Border))
	tail.FillMode = canvas.ImageFillStretch
	return container.New(bubbleLayout{}, card, tail)
}

// noteBubbleWithByline is the bubble plus the attribution that must always
// accompany it, and the attribution sits OUTSIDE the bubble.
//
// That is not decoration. Inside the bubble, a line saying who a note is from
// would read as part of the message — as though the sender had typed it.
// Outside, it reads as what it is: the app telling you where this came from.
// The bubble holds their words and nothing else.
//
// The TRANSLATION is not here any more: it rides in the heading beside the
// reference, in parentheses, where it belongs with the other fact about which
// passage this is.
func noteBubbleWithByline(text, byline string, pal palette) fyne.CanvasObject {
	rows := []fyne.CanvasObject{noteBubble(text, pal)}
	if who := strings.TrimSpace(byline); who != "" {
		lbl := widget.NewLabel(who)
		lbl.TextStyle = fyne.TextStyle{Italic: true}
		lbl.Importance = widget.LowImportance
		rows = append(rows, lbl)
	}
	return container.NewVBox(rows...)
}

// noteVersionAbbrev is the translation a note was written in, short enough to
// sit in a heading: "WEB", "BSB", "NKJV". An id the registry does not know
// degrades to the id itself rather than to nothing — a note is still from
// SOMEWHERE, and saying so beats implying it came from the translation on
// screen.
func noteVersionAbbrev(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if v, ok := versionByID(id); ok {
		if v.Abbrev != "" {
			return v.Abbrev
		}
		return v.Name
	}
	return strings.ToUpper(id)
}

// noteVersionName is the full translation name, for anywhere with room for it.
func noteVersionName(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if v, ok := versionByID(id); ok {
		return v.Name
	}
	return id
}
