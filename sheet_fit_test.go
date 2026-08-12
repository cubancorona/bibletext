package bibletext

// The regression net for the bug that shipped: the Settings sheet grew a section
// and its bottom went off the screen, on the LARGEST phone made. Nothing in the
// suite measured a sheet against a screen, so nothing complained.
//
// TestSettingsSheetFitsEveryScreen is the test that would have caught it: it lays
// the real sheet out on real canvas sizes and asserts the box it occupies lands
// inside the visible area.

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func TestSheetMaxHeight(t *testing.T) {
	// A phone with insets: measure to the bottom of the interactive area, not the
	// bottom of the glass — a row under the home indicator is as unreachable as a
	// row off the screen.
	if got := sheetMaxHeight(956, 62, 860, 78); got != 922-78-sheetBottomMargin {
		t.Errorf("with safe insets: got %v, want %v", got, 922-78-sheetBottomMargin)
	}
	// No insets reported (desktop, and the test canvas): the raw height is honest.
	if got := sheetMaxHeight(600, 0, 0, 28); got != 600-28-sheetBottomMargin {
		t.Errorf("without insets: got %v, want %v", got, 600-28-sheetBottomMargin)
	}
	// A degenerate canvas (mid-rotation, a window dragged to nothing) floors
	// rather than going to zero or negative, which would collapse the sheet.
	if got := sheetMaxHeight(40, 0, 0, 28); got != minSheetHeight {
		t.Errorf("degenerate canvas: got %v, want the %v floor", got, minSheetHeight)
	}
}

func TestScrollingSheetHeight(t *testing.T) {
	// popupMin here is the sheet measured with its scroll collapsed to the scroll
	// widget's own minimum (32), so the chrome is 200-32 = 168.
	const popupMin, scrollMin = 200, 32

	// Body fits: natural height, no cap, and therefore no scrollbar.
	if got := scrollingSheetHeight(popupMin, scrollMin, 300, 900); got != 168+300 {
		t.Errorf("under the cap: got %v, want %v", got, 168+300)
	}
	// Body overflows: capped, and the remainder becomes scrollable rather than
	// invisible.
	if got := scrollingSheetHeight(popupMin, scrollMin, 3000, 900); got != 900 {
		t.Errorf("over the cap: got %v, want the 900 cap", got)
	}
	// Exactly at the cap is not over it.
	if got := scrollingSheetHeight(popupMin, scrollMin, 900-168, 900); got != 900 {
		t.Errorf("at the cap: got %v, want 900", got)
	}
}

// sheetBox is the rectangle a popup actually paints, in canvas coordinates.
//
// widget.PopUp keeps its box in unexported innerPos/innerSize, so it is
// reconstructed from the laid-out content: the renderer resizes the content to
// innerSize minus one innerPadding and moves it to innerPos plus half of one.
func sheetBox(t *testing.T, popup *widget.PopUp) (top, bottom float32) {
	t.Helper()
	// popUpRenderer.Layout ignores the size it is passed and works from
	// innerPos/innerSize, so calling it directly is deterministic — no repaint
	// scheduling to wait on.
	test.WidgetRenderer(popup).Layout(popup.Size())
	pad := popup.Theme().Size(theme.SizeNameInnerPadding)
	top = popup.Content.Position().Y - pad/2
	return top, top + popup.Content.Size().Height + pad
}

// findScroll returns the first container.Scroll in a tree.
func findScroll(o fyne.CanvasObject) *container.Scroll {
	var found *container.Scroll
	walkTree(o, func(n fyne.CanvasObject) {
		if s, ok := n.(*container.Scroll); ok && found == nil {
			found = s
		}
	})
	return found
}

// TestSettingsSheetFitsEveryScreen lays the real Settings sheet out at each
// screen size the app ships on and asserts it lands inside the glass.
//
// The sizes are points, portrait and landscape, from the smallest supported
// phone to the largest — including the iPhone 16 Pro Max, where the clipping was
// found. Landscape matters most: a phone on its side is barely 375pt tall, less
// than half the sheet's natural height.
func TestSettingsSheetFitsEveryScreen(t *testing.T) {
	screens := []struct {
		name string
		w, h float32
	}{
		{"iPhone SE portrait", 320, 568},
		{"iPhone SE landscape", 568, 320},
		{"iPhone 13 mini portrait", 375, 812},
		{"iPhone 16 Pro Max portrait", 440, 956},
		{"iPhone 16 Pro Max landscape", 956, 440},
		{"iPad portrait", 834, 1194},
	}
	for _, sc := range screens {
		for _, aiOn := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/ai=%v", sc.name, aiOn), func(t *testing.T) {
				app := test.NewApp()
				defer app.Quit()
				win := app.NewWindow("Settings")
				win.Resize(fyne.NewSize(sc.w, sc.h))

				// The app's real theme, not the test stub: the sheet's text
				// measurements — which are the whole point here — have to come
				// off the faces the app actually ships.
				th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
				app.Settings().SetTheme(th)

				state := sampleState()
				state.window = win
				state.theme = th
				state.aiKeys = newKeyStoreWith(newFakePrefs())
				state.aiKeys.setAIEnabled(aiOn)

				popup := pickerPopup(t, state, showAISettings)
				top, bottom := sheetBox(t, popup)

				if top < 0 {
					t.Errorf("sheet starts above the screen at y=%v", top)
				}
				if bottom > sc.h {
					t.Errorf("sheet runs off the bottom: box is %v..%v on a %vpt-tall screen "+
						"(%vpt of it unreachable)", top, bottom, sc.h, bottom-sc.h)
				}

				// Fitting by truncation would satisfy the check above, so also
				// assert the way OUT of the overflow exists and reaches all of it:
				// a scroll whose content wants the sheet's full natural height.
				scroll := findScroll(popup.Content)
				if scroll == nil {
					t.Fatal("no scroll in the sheet — content taller than the screen would be unreachable")
				}
				if got := scroll.Content.MinSize().Height; got <= 0 {
					t.Errorf("the scroll reports no content height (%v)", got)
				}
			})
		}
	}
}

// The horizontal half of the same trap. A scroll widens its content to the
// content's MinSize and clips the overflow sideways with no bar to reach it, so
// putting the body in one cost the sheet the end of "Get a key ↗" until
// squeezeWidthLayout went in. Nothing in the sheet may extend past the card.
func TestSettingsSheetDoesNotOverflowSideways(t *testing.T) {
	for _, sc := range []struct {
		name string
		w, h float32
	}{
		{"iPhone SE", 320, 568},
		{"iPhone 13 mini", 375, 812},
		{"iPhone 17 Pro", 402, 874},
		{"iPhone 16 Pro Max", 440, 956},
	} {
		t.Run(sc.name, func(t *testing.T) {
			app := test.NewApp()
			defer app.Quit()
			th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
			app.Settings().SetTheme(th)
			win := app.NewWindow("Settings")
			win.Resize(fyne.NewSize(sc.w, sc.h))
			state := sampleState()
			state.window = win
			state.theme = th
			state.aiKeys = newKeyStoreWith(newFakePrefs())
			state.aiKeys.setAIEnabled(true)

			popup := pickerPopup(t, state, showAISettings)
			test.WidgetRenderer(popup).Layout(popup.Size())

			card := popup.Content
			right := card.Position().X + card.Size().Width

			// Only the containers are walked, not every leaf: a canvas.Text draws
			// at its natural width regardless of the box it is given, so a leaf
			// sticking out says its parent squeezed it — which is the intended
			// behaviour — while a CONTAINER sticking out is the scroll having
			// widened the body, which is the bug.
			var worst float32
			var worstOf string
			// Descend through WIDGETS as well as containers — the body hangs off a
			// *widget.Scroll, and a walk that only follows *fyne.Container stops
			// dead at the one node the check is about.
			var walk func(o fyne.CanvasObject, ox float32)
			walk = func(o fyne.CanvasObject, ox float32) {
				x := ox + o.Position().X
				if c, ok := o.(*fyne.Container); ok {
					if r := x + o.Size().Width; r > worst {
						worst, worstOf = r, fmt.Sprintf("%T", c.Layout)
					}
					for _, ch := range c.Objects {
						walk(ch, x)
					}
					return
				}
				if w, ok := o.(fyne.Widget); ok {
					for _, ch := range test.WidgetRenderer(w).Objects() {
						walk(ch, x)
					}
				}
			}
			walk(card, 0)
			if worst > right+0.5 {
				t.Errorf("sheet content runs %vpt past the card's right edge (%v vs %v); widest was a %s",
					worst-right, worst, right, worstOf)
			}
		})
	}
}

// The sheet must not gratuitously fill a big screen: on a roomy canvas it stays
// its natural height, so it still reads as a card rather than a full-screen page.
func TestSettingsSheetStaysACardWhenThereIsRoom(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	win := app.NewWindow("Settings")
	win.Resize(fyne.NewSize(1200, 2400)) // absurdly tall: nothing can need capping
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	state := sampleState()
	state.window = win
	state.theme = th
	state.aiKeys = newKeyStoreWith(newFakePrefs())

	popup := pickerPopup(t, state, showAISettings)
	top, bottom := sheetBox(t, popup)
	if h := bottom - top; h > 1600 {
		t.Errorf("sheet grew to %vpt on a 2400pt canvas — it should stay its natural height", h)
	}
}
