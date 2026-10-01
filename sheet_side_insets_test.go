package bibletext

// THE PHONE SHEETS KEEP CLEAR OF THE SIDE SAFE INSETS.
//
// An iPhone held sideways has its Dynamic Island, or its notch, at one short
// edge and a safe inset as wide at the other, and nothing an app wants read
// belongs under either. The note composer and the Ask sheet span the canvas,
// and their cards spanned it too, so the first characters of a line and the
// left end of the text box were drawn under the island (clearOfSideInsets,
// sheet_fit.go). These lay the real window out as a phone's driver does —
// inside the safe area its insets leave, less a mobile window's padding —
// open the sheets over it as the reader opens them, and read where
// everything a sheet draws lies on the canvas. Every sheet on every phone
// held sideways is measured in sheet_side_insets_matrix_test.go.

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// insetScreen is a device's canvas in points and the safe insets its driver
// reports there.
type insetScreen struct {
	name                     string
	w, h                     float32
	top, bottom, left, right float32
}

// sideways reports whether s is a phone on its side, whose Read tab reads
// full-screen (phone_landscape.go).
func (s insetScreen) sideways() bool { return s.w > s.h && s.h < 500 }

// area is the safe area s reports, as the driver hands it to the sheets
// (canvasArea).
func (s insetScreen) area(c fyne.Canvas) (fyne.Position, fyne.Size) {
	return fyne.NewPos(s.left, s.top), c.Size().SubtractWidthHeight(s.left+s.right, s.top+s.bottom)
}

// proMaxSideways is an iPhone 17 Pro Max held sideways, with the insets UIKit
// reports for its window on the iOS 26.5 simulator: 62 points at each side,
// the Dynamic Island's and its twin, and 20 under the home indicator.
var proMaxSideways = insetScreen{"iPhone 17 Pro Max, landscape", 956, 440, 0, 20, 62, 62}

// safeAreaLayout places the window's content as the mobile driver does
// (sizeContent): inside the safe area, less the theme's padding on every edge.
type safeAreaLayout struct {
	s   insetScreen
	pad float32
}

func (safeAreaLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.Size{} }

func (l safeAreaLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(l.s.left+l.pad, l.s.top+l.pad))
		o.Resize(size.SubtractWidthHeight(l.s.left+l.s.right+2*l.pad, l.s.top+l.s.bottom+2*l.pad))
	}
}

// insetWindow lays the real window out on s, on tab, inside a root widget like
// the layout watcher a phone's window holds (CreateMainUI), on a device that
// answers mobile, and gives the sheets the safe area s reports, with no
// keyboard up. On a phone held sideways the Read tab is the full-screen
// presentation, as it is by default.
func insetWindow(t *testing.T, s insetScreen, tab int) (*AppState, fyne.Window) {
	t.Helper()
	app := test.NewTempApp(t)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	fyne.SetCurrentApp(phoneTestApp{app})
	prevArea, prevFoot, prevLandscape := canvasArea, keyboardFreeFoot, phoneLandscapeReading
	keyboardFreeFoot.known, keyboardFreeFoot.foot = false, 0
	canvasArea = s.area
	sideways := s.sideways()
	phoneLandscapeReading = func() bool { return sideways }
	t.Cleanup(func() { canvasArea, keyboardFreeFoot, phoneLandscapeReading = prevArea, prevFoot, prevLandscape })
	softKeyboard(t, false)

	st := sampleState()
	w := app.NewWindow("phone")
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(s.w, s.h))
	st.window, st.app, st.theme = w, app, th
	// No assistant key: Settings would fetch the provider's model list for one.
	st.aiKeys = newKeyStoreWith(newFakePrefs())
	st.CurrentTab = tab
	w.SetPadded(false)
	root := &wrappingRoot{phoneRoot{content: buildCompactUI(st)}}
	root.ExtendBaseWidget(root)
	w.SetContent(container.New(safeAreaLayout{s, th.Size(theme.SizeNamePadding)}, root))
	w.Resize(fyne.NewSize(s.w, s.h))
	if fullScreen := tab == 0 && sideways; st.readingFullScreen() != fullScreen {
		t.Fatalf("control: %s, tab %d: reading full-screen %v, want %v", s.name, tab, st.readingFullScreen(), fullScreen)
	}
	return st, w
}

// sheetPart is one thing a sheet draws, and where it lies on the canvas.
type sheetPart struct {
	what           string
	x0, x1, y0, y1 float32
}

func (p sheetPart) String() string {
	return fmt.Sprintf("%s at x %.1f..%.1f, y %.1f..%.1f", p.what, p.x0, p.x1, p.y0, p.y1)
}

// sheetParts is everything popup draws inside its box: each visible object
// under its content that is not a container or a spacer, at its place on the
// canvas. A widget is read as its own box, which holds what it draws.
func sheetParts(popup *widget.PopUp) []sheetPart {
	test.WidgetRenderer(popup).Layout(popup.Size())
	var out []sheetPart
	var walk func(o fyne.CanvasObject, at fyne.Position)
	walk = func(o fyne.CanvasObject, at fyne.Position) {
		if o == nil || !o.Visible() {
			return
		}
		switch v := o.(type) {
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c, at.Add(c.Position()))
			}
			return
		case *container.Scroll:
			walk(v.Content, at.Add(v.Content.Position()))
			return
		case *container.ThemeOverride:
			walk(v.Content, at.Add(v.Content.Position()))
			return
		case *layout.Spacer:
			return
		}
		sz := o.Size()
		if sz.Width <= 0 || sz.Height <= 0 {
			return
		}
		what := fmt.Sprintf("%T", o)
		if texts := treeTexts(o); len(texts) > 0 {
			what += fmt.Sprintf(" %q", strings.Join(texts, " "))
		}
		out = append(out, sheetPart{what, at.X, at.X + sz.Width, at.Y, at.Y + sz.Height})
	}
	walk(popup.Content, popup.Content.Position())
	return out
}

// partsSpan is the least box holding every part.
func partsSpan(parts []sheetPart) sheetPart {
	if len(parts) == 0 {
		return sheetPart{what: "nothing"}
	}
	u := parts[0]
	for _, p := range parts[1:] {
		u.x0, u.x1, u.y0, u.y1 = min(u.x0, p.x0), max(u.x1, p.x1), min(u.y0, p.y0), max(u.y1, p.y1)
	}
	u.what = "the sheet's card"
	return u
}

// underASideInset lists the parts that reach under one of s's side insets.
func underASideInset(s insetScreen, parts []sheetPart) []string {
	var out []string
	for _, p := range parts {
		if p.x0 < s.left || p.x1 > s.w-s.right {
			out = append(out, p.String())
		}
	}
	return out
}

// sheetBoxOn is the box popup takes on the canvas: its content's, and the
// popup's own padding round it (sheetBox).
func sheetBoxOn(t *testing.T, popup *widget.PopUp) sheetPart {
	t.Helper()
	top, bottom := sheetBox(t, popup)
	pad := popup.Theme().Size(theme.SizeNameInnerPadding)
	x0 := popup.Content.Position().X - pad/2
	return sheetPart{"the sheet's box", x0, x0 + popup.Content.Size().Width + pad, top, bottom}
}

// phoneSheetOpener opens one of the two sheets that span the canvas.
type phoneSheetOpener struct {
	name string
	open func(*AppState)
}

var spanningSheets = []phoneSheetOpener{
	{"the note composer", func(st *AppState) { promptShareNote(st, "For God so loved the world", selSpan{}) }},
	{"the Ask sheet", func(st *AppState) { promptAskQuestion(st, "For God so loved the world") }},
}

func TestSideInsets(t *testing.T) {
	for _, tc := range []struct {
		name        string
		canvasW     float32
		pos         fyne.Position
		sz          fyne.Size
		left, right float32
	}{
		{"no area reported (a desktop window)", 1280, fyne.Position{}, fyne.Size{}, 0, 0},
		{"a phone held upright", 440, fyne.NewPos(0, 62), fyne.NewSize(440, 860), 0, 0},
		{"an iPad", 834, fyne.NewPos(0, 24), fyne.NewSize(834, 1150), 0, 0},
		{"an iPhone held sideways", 956, fyne.NewPos(62, 0), fyne.NewSize(832, 420), 62, 62},
		{"an Android phone's cutout on the left", 803, fyne.NewPos(21, 0), fyne.NewSize(782, 360), 21, 0},
		{"an Android phone's three-button bar on the right", 803, fyne.NewPos(0, 21), fyne.NewSize(761, 339), 0, 42},
		{"an area wider than the canvas", 400, fyne.NewPos(-2, 0), fyne.NewSize(410, 300), 0, 0},
	} {
		if l, r := sideInsets(tc.canvasW, tc.pos, tc.sz); l != tc.left || r != tc.right {
			t.Errorf("%s: side insets %v, %v; want %v, %v", tc.name, l, r, tc.left, tc.right)
		}
	}
}

// THE COMPOSER AND THE ASK SHEET KEEP THEIR CARDS CLEAR OF THE SIDE INSETS.
// On an iPhone 17 Pro Max held sideways, over the full-screen Read tab (where
// a selection opens them) and over Books, nothing either sheet draws reaches
// under the Dynamic Island or the inset opposite it, nor does the composer's
// native field, which is parked over its slot (note_entry_ios.go). The card
// still fills the safe area's width less its 4-point margin, so a card that
// shrank would fail here too. The sheet's box is where it was: across the
// whole canvas, so a tap beside the card lands on the sheet and closes
// nothing (the composer's close takes the note with it), and from the safe
// area's top to its foot, so neither sheet moves vertically and neither
// crosses the home indicator's inset. Shown to fail on the tree before
// clearOfSideInsets, where the card drew from x 4 to 952.
func TestTheComposerAndAskKeepClearOfTheSideInsets(t *testing.T) {
	s := proMaxSideways
	for _, tab := range []int{0, 1} {
		for _, sh := range spanningSheets {
			t.Run(fmt.Sprintf("tab %d, %s", tab, sh.name), func(t *testing.T) {
				holdSheetTimers(t)
				holdNoteSheetTimers(t)
				st, w := insetWindow(t, s, tab)
				frames := nativeNoteField(t, w)
				sh.open(st)
				popup := topPopup(t, w)
				pad := popup.Theme().Size(theme.SizeNameInnerPadding)
				parts := sheetParts(popup)
				for _, p := range underASideInset(s, parts) {
					t.Errorf("%s draws %s, under a side inset (the safe area runs from x %.0f to %.0f)", sh.name, p, s.left, s.w-s.right)
				}
				if card := partsSpan(parts); card.x0 != s.left+pad/2 || card.x1 != s.w-s.right-pad/2 {
					t.Errorf("%s spans x %.1f..%.1f, want the safe area less the card's margin, %.1f..%.1f",
						sh.name, card.x0, card.x1, s.left+pad/2, s.w-s.right-pad/2)
				}
				if sh.name == "the note composer" {
					if len(*frames) == 0 {
						t.Fatal("control: the native field was never given a frame")
					}
					f := (*frames)[len(*frames)-1]
					if f.pos.X < s.left || f.pos.X+f.sz.Width > s.w-s.right {
						t.Errorf("the native field is parked at x %.1f..%.1f, under a side inset", f.pos.X, f.pos.X+f.sz.Width)
					}
				}
				box := sheetBoxOn(t, popup)
				if box.x0 != 0 || box.x1 != s.w {
					t.Errorf("%s's box spans x %.1f..%.1f; it must still span the canvas, 0..%.0f", sh.name, box.x0, box.x1, s.w)
				}
				if box.y0 != s.top || box.y1 != s.h-s.bottom {
					t.Errorf("%s's box spans y %.1f..%.1f; it must keep the safe area's top and foot, %.0f..%.0f",
						sh.name, box.y0, box.y1, s.top, s.h-s.bottom)
				}
				popup.Tapped(&fyne.PointEvent{Position: fyne.NewPos(s.left/2, s.h/2)})
				if !popup.Visible() {
					t.Errorf("a tap beside the card, under the island, closed %s", sh.name)
				}
			})
		}
	}
}

// WHERE NO SIDE IS INSET, THE TWO SHEETS ARE WHERE THEY WERE. Every phone
// held upright, every iPad, an Android phone held sideways with nothing at
// its sides and a phone with no cutout held sideways: each sheet's box spans
// the canvas from the safe area's top to its foot, and its card lies 4
// points inside that box on every side, the frame both sheets had before
// clearOfSideInsets. Shown to pass on the tree before it, and to fail with
// the card held clear of a side inset where there is none.
func TestTheComposerAndAskStayPutWithoutASideInset(t *testing.T) {
	for _, s := range []insetScreen{
		{"iPhone 17 Pro Max, portrait", 440, 956, 62, 34, 0, 0},
		{"iPhone 16 Pro, portrait", 402, 874, 62, 34, 0, 0},
		{"iPhone SE, portrait", 375, 667, 20, 0, 0, 0},
		{"iPhone SE, landscape", 667, 375, 0, 0, 0, 0},
		{"a 320-point phone", 320, 568, 20, 0, 0, 0},
		{"Android phone, portrait", 360, 803, 21, 21, 0, 0},
		{"Android phone, portrait, three-button navigation", 360, 803, 21, 42, 0, 0},
		{"Android phone, landscape", 803, 360, 21, 0, 0, 0},
		{"iPad, portrait", 834, 1194, 24, 20, 0, 0},
		{"iPad, landscape", 1194, 834, 24, 20, 0, 0},
		{"iPad Pro 13-inch, portrait", 1032, 1376, 24, 20, 0, 0},
	} {
		for _, sh := range spanningSheets {
			t.Run(s.name+", "+sh.name, func(t *testing.T) {
				holdSheetTimers(t)
				holdNoteSheetTimers(t)
				st, w := insetWindow(t, s, 0)
				sh.open(st)
				popup := topPopup(t, w)
				pad := popup.Theme().Size(theme.SizeNameInnerPadding)
				if box := sheetBoxOn(t, popup); box.x0 != 0 || box.x1 != s.w || box.y0 != s.top || box.y1 != s.h-s.bottom {
					t.Errorf("%s's box is %v; want x 0..%.0f, y %.0f..%.0f", sh.name, box, s.w, s.top, s.h-s.bottom)
				}
				want := sheetPart{"the sheet's card", pad / 2, s.w - pad/2, s.top + pad/2, s.h - s.bottom - pad/2}
				if card := partsSpan(sheetParts(popup)); card != want {
					t.Errorf("%s's card is %v; want %v", sh.name, card, want)
				}
			})
		}
	}
}

// THE COMPOSER TAKES THE SIDE INSETS OF A CANVAS THAT CHANGES SIZE. The
// composer refits itself when the canvas changes size with it open (an iPad
// or Android window resized, which rebuilds nothing), and the refit reads
// the side insets again: a window that grows under a cutout moves the card,
// and the native field over its slot, clear of it. The control is the card
// before the refit runs, whose right end is under the new inset.
func TestTheComposerTakesTheSideInsetsOfANewCanvas(t *testing.T) {
	held := holdNoteSheetTimers(t)
	holdSheetTimers(t)
	away := insetScreen{"a window clear of the cutout", 900, 440, 0, 20, 0, 0}
	st, w := insetWindow(t, away, 1)
	frames := nativeNoteField(t, w)
	promptShareNote(st, "For God so loved the world", selSpan{})
	popup := topPopup(t, w)

	s := proMaxSideways
	canvasArea = s.area // insetWindow puts the driver's area back
	w.Resize(fyne.NewSize(s.w, s.h))
	if len(underASideInset(s, sheetParts(popup))) == 0 {
		t.Fatal("control: before the refit the card should reach under the new right inset")
	}
	runHeld(held)
	for _, p := range underASideInset(s, sheetParts(popup)) {
		t.Errorf("after the refit the composer draws %s, under a side inset", p)
	}
	if len(*frames) == 0 {
		t.Fatal("control: the native field was never given a frame")
	}
	if f := (*frames)[len(*frames)-1]; f.pos.X < s.left || f.pos.X+f.sz.Width > s.w-s.right {
		t.Errorf("after the refit the native field is parked at x %.1f..%.1f, under a side inset", f.pos.X, f.pos.X+f.sz.Width)
	}
	if box := sheetBoxOn(t, popup); box.x0 != 0 || box.x1 != s.w {
		t.Errorf("after the refit the box spans x %.1f..%.1f; want the canvas, 0..%.0f", box.x0, box.x1, s.w)
	}
}
