package bibletext

// Decision 13's ✕ beside a key field's missing-saved-key line
// (docs/ROADMAP.md, 1.2.23, row 17) is small and level with the line's first
// row. As built, its glyph is smaller than the Settings header's ✕, and the
// glyph's centre lies on the centre of the line's first row, under the
// API.Bible key's field and each AI provider's, on a phone and on a desktop,
// at every text size. Its touch target uses only the column's existing space
// and takes no tap from buttons above the line or below its row.
//
// These open the real Settings sheet and measure what it draws, in the
// reader's own theme and typefaces.

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// notRestoredSheet opens the real Settings sheet on a phone whose settings a
// restore brought back without its keys, with the assistant on, so the
// assistant's key field and the API.Bible key's each say the line with a ✕
// beside it. phone makes the device a phone, width is the canvas's, and size
// is the reader's text size.
func notRestoredSheet(t *testing.T, phone bool, width float32, size string) *widget.PopUp {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	if phone {
		fyne.SetCurrentApp(phoneTestApp{app})
	}
	setReadingTextSizeID(size)
	if got := readingTextSizeID(); got != size {
		t.Fatalf("control: the text size reads %q, want %q", got, size)
	}
	t.Setenv("BIBLE_API_KEY", "")
	withFakeSharedKeys(t)
	info, ok := providerByID(defaultProviderID)
	if !ok {
		t.Fatal("control: the default provider is not registered")
	}
	keys := restoredOntoNewPhone(t, newDeviceBoundTestStore(), info.ID, bibleKeyID)
	keys.setActiveProvider(info.ID)
	keys.setAIEnabled(true)
	win := app.NewWindow("Settings")
	t.Cleanup(win.Close)
	win.Resize(fyne.NewSize(width, 2400))
	st := sampleState()
	st.window, st.theme, st.aiKeys = win, th, keys
	popup := pickerPopup(t, st, showAISettings)
	t.Cleanup(popup.Hide)
	test.WidgetRenderer(popup).Layout(popup.Size())
	return popup
}

// notRestoredCloses is the sheet's two ✕, the assistant's field's and the
// API.Bible key's, each showing beside the line.
func notRestoredCloses(t *testing.T, popup *widget.PopUp) []statusClose {
	t.Helper()
	closes := keyStatusCloses(popup)
	if len(closes) != 2 {
		t.Fatalf("control: want a ✕ beside each of the sheet's two key fields' lines, found %d", len(closes))
	}
	for i, c := range closes {
		if c.text != approvedKeyNotRestoredLine || !c.showing {
			t.Fatalf("control: field %d reads %q with the ✕ showing %v; want the line and its ✕", i, c.text, c.showing)
		}
	}
	return closes
}

// glyphOf is the icon o draws: the one image under it.
func glyphOf(t *testing.T, tree []drawnObject, o fyne.CanvasObject) drawnObject {
	t.Helper()
	var found []drawnObject
	for _, d := range tree {
		if _, ok := d.obj.(*canvas.Image); ok && d.under(o) {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one glyph drawn under the %T, found %d", o, len(found))
	}
	return found[0]
}

// lineRows are the rows a status line draws, top first.
func lineRows(tree []drawnObject, line *statusLine) []drawnObject {
	var rows []drawnObject
	for _, d := range tree {
		if txt, ok := d.obj.(*canvas.Text); ok && d.obj != line && d.under(line) && txt.Text != "" {
			rows = append(rows, d)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pos.Y < rows[j].pos.Y })
	return rows
}

func centreY(d drawnObject) float32 { return d.pos.Y + d.obj.Size().Height/2 }

// drawnAt names a drawn object by its type, place and size, for a failure.
func drawnAt(d drawnObject) string {
	if d.obj == nil {
		return "nothing"
	}
	s := d.obj.Size()
	return fmt.Sprintf("%T at %.1f,%.1f %.1fx%.1f", d.obj, d.pos.X, d.pos.Y, s.Width, s.Height)
}

// closeSheetCases are the sheets measured: phone canvases from the first
// iPhone SE's to a current phone's, desktop windows from a small laptop's to
// a wide one, each at the smallest and the largest text size.
func closeSheetCases() []struct {
	phone bool
	width float32
	size  string
} {
	var out []struct {
		phone bool
		width float32
		size  string
	}
	for _, size := range []string{"normal", "xl"} {
		for _, w := range []float32{320, 393} {
			out = append(out, struct {
				phone bool
				width float32
				size  string
			}{true, w, size})
		}
		for _, w := range []float32{834, 1440} {
			out = append(out, struct {
				phone bool
				width float32
				size  string
			}{false, w, size})
		}
	}
	return out
}

func caseName(phone bool, width float32, size string) string {
	device := "desktop"
	if phone {
		device = "phone"
	}
	return fmt.Sprintf("%s %.0f %s", device, width, size)
}

// TestTheNotRestoredCloseIsSmallAndLevelWithTheLinesFirstRow: the ✕'s glyph
// is smaller than the Settings header's, and so is the button drawn around
// it; the glyph's centre is within 2pt of the centre of the first row of the
// line beside it.
func TestTheNotRestoredCloseIsSmallAndLevelWithTheLinesFirstRow(t *testing.T) {
	for _, tc := range closeSheetCases() {
		t.Run(caseName(tc.phone, tc.width, tc.size), func(t *testing.T) {
			popup := notRestoredSheet(t, tc.phone, tc.width, tc.size)
			tree := drawnTree(t, popup)
			header := sheetCloseButton(t, popup)
			headerGlyph := glyphOf(t, tree, header)
			headerBox := drawnAs(t, tree, header).obj.Size()
			for i, c := range notRestoredCloses(t, popup) {
				glyph := glyphOf(t, tree, c.btn)
				gs, hs := glyph.obj.Size(), headerGlyph.obj.Size()
				if gs.Width >= hs.Width || gs.Height >= hs.Height {
					t.Errorf("field %d: the ✕'s glyph is %.1fx%.1f, the header ✕'s %.1fx%.1f; want it smaller", i, gs.Width, gs.Height, hs.Width, hs.Height)
				}
				box := drawnAs(t, tree, c.btn).obj.Size()
				if box.Width >= headerBox.Width || box.Height >= headerBox.Height {
					t.Errorf("field %d: the ✕'s button is %.1fx%.1f, the header ✕'s %.1fx%.1f; want it smaller", i, box.Width, box.Height, headerBox.Width, headerBox.Height)
				}
				rows := lineRows(tree, c.line)
				if len(rows) == 0 {
					t.Fatalf("control: field %d's line draws no rows", i)
				}
				off := centreY(glyph) - centreY(rows[0])
				if math.Abs(float64(off)) > 2 {
					t.Errorf("field %d: the ✕'s glyph is centred %.1fpt from the centre of the line's first row (of %d); want within 2pt", i, off, len(rows))
				}
				right := rows[0].pos.X + rows[0].obj.Size().Width
				if glyph.pos.X < right {
					t.Errorf("field %d: the ✕'s glyph starts at x %.1f, inside the line's first row, which ends at %.1f", i, glyph.pos.X, right)
				}
				// The ✕ takes no more of the row's width from the line than
				// its tap target's.
				line, col := drawnAs(t, tree, c.line), drawnAs(t, tree, c.col)
				rowW := col.pos.X + col.obj.Size().Width - line.pos.X
				if lw := line.obj.Size().Width; lw < rowW-minTapTarget-0.01 {
					t.Errorf("field %d: the line is %.1f wide in a row %.1f wide; the ✕ takes %.1f of it, more than its %.0fpt target", i, lw, rowW, rowW-lw, minTapTarget)
				}
			}
		})
	}
}

// matcherFor is the question a driver asks of the object under a point for
// the named event (driverMatchers).
func matcherFor(t *testing.T, event string) func(fyne.CanvasObject) bool {
	t.Helper()
	for _, m := range driverMatchers {
		if m.event == event {
			return m.match
		}
	}
	t.Fatalf("no driver matcher for %q", event)
	return nil
}

// closeTarget is the ✕'s tap target: the largest object in its column that
// takes a tap and covers the glyph.
func closeTarget(t *testing.T, tree []drawnObject, c statusClose, glyph drawnObject) drawnObject {
	t.Helper()
	var best drawnObject
	var area float32
	for _, d := range tree {
		if !d.under(c.col) || d.obj == c.col {
			continue
		}
		if _, ok := d.obj.(fyne.Tappable); !ok {
			continue
		}
		g := glyph.obj.Size()
		gc := fyne.NewPos(glyph.pos.X+g.Width/2, glyph.pos.Y+g.Height/2)
		if !d.contains(gc) {
			continue
		}
		s := d.obj.Size()
		if a := s.Width * s.Height; a > area {
			best, area = d, a
		}
	}
	if best.obj == nil {
		t.Fatal("nothing in the ✕'s column takes a tap over its glyph")
	}
	return best
}

// The touch-only target uses available space without imposing a larger row;
// desktop pointer clicks keep the small button's own bounds.
func TestTheNotRestoredCloseUsesOnlyAvailableTapSpace(t *testing.T) {
	for _, tc := range closeSheetCases() {
		t.Run(caseName(tc.phone, tc.width, tc.size), func(t *testing.T) {
			event := "a mouse click"
			if tc.phone {
				event = "a touch up (tap, long press, double tap)"
			}
			match := matcherFor(t, event)
			popup := notRestoredSheet(t, tc.phone, tc.width, tc.size)
			tree := drawnTree(t, popup)
			closes := notRestoredCloses(t, popup)
			targets := make([]drawnObject, len(closes))
			for i, c := range closes {
				target := closeTarget(t, tree, c, glyphOf(t, tree, c.btn))
				targets[i] = target
				s := target.obj.Size()
				col := drawnAs(t, tree, c.col)
				if tc.phone {
					want := fyne.NewSize(fyne.Min(col.obj.Size().Width, minTapTarget), fyne.Min(col.obj.Size().Height, minTapTarget))
					if s != want || target.pos.X < col.pos.X || target.pos.X+s.Width > col.pos.X+col.obj.Size().Width {
						t.Errorf("field %d: touch target %s exceeds its existing column %s", i, drawnAt(target), drawnAt(col))
					}
				} else if target.obj != c.btn || s != fyne.NewSquareSize(lineCloseBox) {
					t.Errorf("field %d: desktop target %s is larger than the small button", i, drawnAt(target))
				}
				// Every point of it reaches the ✕'s column, not a control
				// drawn over it.
				for y := target.pos.Y; y < target.pos.Y+s.Height; y += 2 {
					for x := target.pos.X; x < target.pos.X+s.Width; x += 2 {
						p := fyne.NewPos(x, y)
						hit, ok := hitAt(tree, p, match)
						if !ok || !hit.under(c.col) {
							t.Fatalf("field %d: %s at %.0f,%.0f, inside the ✕'s tap target, goes to %s, not the ✕", i, event, x, y, drawnAt(hit))
						}
					}
				}
			}
			if t.Failed() {
				return
			}
			// A tap on each target puts its own field's line away.
			for i, target := range targets {
				switch v := target.obj.(type) {
				case *widget.Button:
					v.OnTapped()
				case fyne.Tappable:
					v.Tapped(&fyne.PointEvent{})
				}
				after := keyStatusCloses(popup)
				if after[i].text == approvedKeyNotRestoredLine || after[i].showing {
					t.Errorf("field %d: a tap on the ✕'s tap target left the line: %q, ✕ showing %v", i, after[i].text, after[i].showing)
				}
				for j := i + 1; j < len(after); j++ {
					if after[j].text != approvedKeyNotRestoredLine || !after[j].showing {
						t.Errorf("field %d's ✕ put away field %d's line too: %q", i, j, after[j].text)
					}
				}
			}
		})
	}
}

// TestTheNotRestoredCloseTakesNoTapFromTheControlsAbove: nothing in the ✕'s
// column that takes a tap overlaps a button or a field, the Paste, Test key
// and Clear row above the line among them.
func TestTheNotRestoredCloseTakesNoTapFromTheControlsAbove(t *testing.T) {
	for _, tc := range closeSheetCases() {
		t.Run(caseName(tc.phone, tc.width, tc.size), func(t *testing.T) {
			popup := notRestoredSheet(t, tc.phone, tc.width, tc.size)
			tree := drawnTree(t, popup)
			closes := notRestoredCloses(t, popup)
			var controls []drawnObject
			for _, d := range tree {
				switch d.obj.(type) {
				case *widget.Button, *widget.Entry:
					controls = append(controls, d)
				}
			}
			for i, c := range closes {
				var own []drawnObject
				for _, d := range tree {
					if _, ok := d.obj.(fyne.Tappable); ok && d.under(c.col) {
						own = append(own, d)
					}
				}
				if len(own) == 0 {
					t.Fatalf("control: nothing in field %d's ✕ column takes a tap", i)
				}
				for _, o := range own {
					for _, d := range controls {
						if d.under(c.col) {
							continue
						}
						if o.overlaps(d) {
							t.Errorf("field %d: the ✕'s %s overlaps %s", i, drawnAt(o), drawnAt(d))
						}
					}
				}
			}
		})
	}
}

// TestTheNotRestoredCloseTakesNoTapBelowItsRow: the ✕'s tap target lies
// within the row of the line beside it, from the row's top to its foot, and
// nothing else in the ✕'s column that takes a tap reaches below the row,
// however few rows the line breaks into. The small button can reach into the
// clear gap above the line to align its glyph, but never below the row.
func TestTheNotRestoredCloseTakesNoTapBelowItsRow(t *testing.T) {
	for _, tc := range closeSheetCases() {
		t.Run(caseName(tc.phone, tc.width, tc.size), func(t *testing.T) {
			popup := notRestoredSheet(t, tc.phone, tc.width, tc.size)
			tree := drawnTree(t, popup)
			for i, c := range notRestoredCloses(t, popup) {
				row := drawnAs(t, tree, c.row)
				top, foot := row.pos.Y, row.pos.Y+row.obj.Size().Height
				target := closeTarget(t, tree, c, glyphOf(t, tree, c.btn))
				if (tc.phone && target.pos.Y < top-0.01) || target.pos.Y+target.obj.Size().Height > foot+0.01 {
					t.Errorf("field %d: the ✕'s tap target, %s, is not within the line's row, from y %.1f to %.1f", i, drawnAt(target), top, foot)
				}
				for _, d := range tree {
					if _, ok := d.obj.(fyne.Tappable); !ok || !d.under(c.col) {
						continue
					}
					if bottom := d.pos.Y + d.obj.Size().Height; bottom > foot+0.01 {
						t.Errorf("field %d: the ✕'s %s reaches %.1fpt below the line's row, which ends at y %.1f", i, drawnAt(d), bottom-foot, foot)
					}
				}
			}
		})
	}
}

// A smaller glyph must not change the original status column, line width or
// row height. Compare with the previous button-and-spacer allocation using
// the same current sentence, so copy changes do not disguise layout growth.
func TestTheNotRestoredCloseKeepsTheOriginalRowAllocation(t *testing.T) {
	for _, tc := range closeSheetCases() {
		t.Run(caseName(tc.phone, tc.width, tc.size), func(t *testing.T) {
			popup := notRestoredSheet(t, tc.phone, tc.width, tc.size)
			tree := drawnTree(t, popup)
			for i, c := range notRestoredCloses(t, popup) {
				row := drawnAs(t, tree, c.row)
				line := drawnAs(t, tree, c.line)
				col := drawnAs(t, tree, c.col)
				legacyLine := newStatusText(c.line.text, c.line.size, c.line.style, c.line.color)
				legacyButton := widget.NewButtonWithIcon("", theme.CancelIcon(), nil)
				legacyButton.Importance = widget.LowImportance
				legacyCol := container.NewVBox(legacyButton, layout.NewSpacer())
				legacy := container.NewBorder(nil, nil, nil, legacyCol, legacyLine)
				legacy.Resize(row.obj.Size())
				if c.col.MinSize() != legacyCol.MinSize() || c.row.MinSize() != legacy.MinSize() {
					t.Errorf("field %d: original minimum changed: column %v/%v row %v/%v", i, c.col.MinSize(), legacyCol.MinSize(), c.row.MinSize(), legacy.MinSize())
				}
				if legacyLine.Size() != line.obj.Size() || legacyCol.Size() != col.obj.Size() || legacyLine.Position() != c.line.Position() {
					t.Errorf("field %d: smaller close changed the line or column allocation", i)
				}
			}
		})
	}
}
