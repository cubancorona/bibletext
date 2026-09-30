package bibletext

// THE KEY STATUS LINES SHOW THEIR WHOLE TEXT.
//
// The 13-inch iPad's Settings sheet drew "✓ Included with BibleText — or paste
// your own" with no full stop and the last letter cut square. Two things did
// it, and both are held here:
//
//   - The line was a canvas.Text, which never breaks, so any status longer
//     than its row ran into the scroll's edge. At 320pt the error lines do.
//   - The ✓ is in neither UI face, so Fyne takes it from a system font, and
//     the sheet's theme override gave the sheet font caches of its own: the
//     text was measured through the app's cache and drawn through the
//     sheet's, which had picked a wider ✓. A text is painted into a texture
//     exactly as wide as it measured, so the end of the line fell off it.

import (
	"image"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/image/font/sfnt"
)

// settingsWithIncludedKey opens the real Settings sheet on a w x h canvas with
// the key that ships with the app in force and the assistant on, so both key
// sections show a status. before, if not nil, runs once the app and its theme
// are up and the sheet is not yet open.
func settingsWithIncludedKey(t *testing.T, w, h, scale float32, before func()) (*AppState, fyne.Window, *widget.PopUp) {
	t.Helper()
	app, th := settingsApp(t)
	return openSettingsWithIncludedKey(t, app, th, w, h, scale, before)
}

// settingsApp is a new test app in the app's real theme, quit when t ends.
func settingsApp(t *testing.T) (fyne.App, *bibleTheme) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	return app, th
}

// openSettingsWithIncludedKey is settingsWithIncludedKey in an app settingsApp
// built, so a sweep can open the sheet many times in one app. Each call opens
// it in a window of its own, with keys, state and preferences of its own: the
// app's preferences are emptied first, which is what a new app has, so no
// call opens the sheet on what an earlier one left.
func openSettingsWithIncludedKey(t *testing.T, app fyne.App, th *bibleTheme, w, h, scale float32, before func()) (*AppState, fyne.Window, *widget.PopUp) {
	t.Helper()
	prefs, ok := app.Preferences().(interface{ WriteValues(func(map[string]any)) })
	if !ok {
		t.Fatalf("setup: the test app's preferences (%T) cannot be emptied", app.Preferences())
	}
	prefs.WriteValues(func(m map[string]any) { clear(m) })
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	prev := bundledBibleKeyEnc
	t.Cleanup(func() { bundledBibleKeyEnc = prev })
	bundledBibleKeyEnc = obfuscateForTest("the-included-key")
	fake.setAIEnabled(true)
	if !fake.usingBundledBibleKey() {
		t.Fatal("setup: the included key should be in force")
	}
	win := app.NewWindow("Settings")
	t.Cleanup(win.Close)
	if scale != 1 {
		win.Canvas().(test.WindowlessCanvas).SetScale(scale)
	}
	win.Resize(fyne.NewSize(w, h))
	st := sampleState()
	st.window = win
	st.theme = th
	st.aiKeys = fake
	if before != nil {
		before()
	}
	popup := pickerPopup(t, st, showAISettings)
	t.Cleanup(popup.Hide)
	test.WidgetRenderer(popup).Layout(popup.Size())
	return st, win, popup
}

func statusLinesIn(o fyne.CanvasObject) []*statusLine {
	var out []*statusLine
	walkTree(o, func(n fyne.CanvasObject) {
		if s, ok := n.(*statusLine); ok {
			out = append(out, s)
		}
	})
	return out
}

func drawnLines(s *statusLine) []*canvas.Text {
	var out []*canvas.Text
	for _, o := range test.WidgetRenderer(s).Objects() {
		if tx, ok := o.(*canvas.Text); ok {
			out = append(out, tx)
		}
	}
	return out
}

func TestKeyStatusLinesShowWholeAtEverySheetWidth(t *testing.T) {
	const included = "✓ Included with BibleText — or paste your own."
	// Everything either key section's status can say.
	says := []string{
		included,
		"✓ Saved on this device.",
		"✓ Saved in the Keychain.",
		"✓ Saved in the Android Keystore.",
		"Couldn't save this key securely. Please try again.",
		"Couldn't remove the stored key. Please try again.",
		"Free for personal use — no card, no charge.",
	}
	for _, p := range aiProviders() {
		says = append(says, p.KeyHint)
	}

	for _, sc := range []struct {
		name string
		w, h float32
	}{
		{"320pt phone", 320, 568},
		{"375pt phone", 375, 812},
		{"440pt phone", 440, 956},
		{"11-inch iPad", 834, 1194},
		{"13-inch iPad", 1032, 1376},
		{"desktop", 1280, 800},
	} {
		t.Run(sc.name, func(t *testing.T) {
			_, _, popup := settingsWithIncludedKey(t, sc.w, sc.h, 1, nil)
			lines := statusLinesIn(popup)
			if len(lines) != 2 {
				t.Fatalf("found %d key status lines, want the assistant's and API.Bible's", len(lines))
			}
			drv := fyne.CurrentApp().Driver()
			card := popup.Content
			cardRight := drv.AbsolutePositionForObject(card).X + card.Size().Width
			if got := lines[1].text; got != included {
				t.Errorf("API.Bible's status reads %q, want %q", got, included)
			}
			most := 0
			for _, s := range lines {
				width := s.Size().Width
				drawn := drawnLines(s)
				most = max(most, len(drawn))
				var joined []string
				for i, tx := range drawn {
					joined = append(joined, tx.Text)
					if w := tx.MinSize().Width; w > width+0.01 {
						t.Errorf("%q: a line %q is %.1fpt in a %.1fpt row", s.text, tx.Text, w, width)
					}
					// Every line is drawn, and each below the one before it,
					// not over it.
					if !tx.Visible() {
						t.Errorf("%q: its line %d, %q, is hidden", s.text, i+1, tx.Text)
					}
					if i > 0 {
						prev := drawn[i-1]
						if top, floor := tx.Position().Y, prev.Position().Y+prev.MinSize().Height; top < floor-0.01 {
							t.Errorf("%q: its line %d, %q, starts at %.1fpt, over the line above, which ends at %.1fpt",
								s.text, i+1, tx.Text, top, floor)
						}
					}
				}
				if got := strings.Join(joined, " "); got != s.text {
					t.Errorf("the status draws %q, want the whole of %q", got, s.text)
				}
				// The row the sheet gives the line holds every line it draws,
				// measured from the lines themselves: a line drawn below its
				// row lies over whatever the card puts under it.
				if n := len(drawn); n > 0 {
					last := drawn[n-1]
					if end := last.Position().Y + last.MinSize().Height; end > s.Size().Height+0.01 {
						t.Errorf("%q: its %d lines end %.1fpt down a row %.1fpt tall", s.text, n, end, s.Size().Height)
					}
				}
				if right := drv.AbsolutePositionForObject(s).X + width; right > cardRight+0.5 {
					t.Errorf("%q: the row ends at %.1f, past the card's edge at %.1f", s.text, right, cardRight)
				}
				// And every other thing it can say fits the same row.
				for _, say := range says {
					broken := statusLines(say, width, s.size)
					for _, l := range broken {
						if w := fyne.MeasureText(l, s.size, fyne.TextStyle{}).Width; w > width+0.01 {
							t.Errorf("%q breaks to a line %q of %.1fpt in a %.1fpt row", say, l, w, width)
						}
					}
					if strings.Join(broken, " ") != say {
						t.Errorf("%q breaks to %q, which is not the whole text", say, broken)
					}
				}
			}
			// Control: at the narrowest width a status really does wrap, so
			// the row check above has a second line to hold.
			if sc.w == 320 && most < 2 {
				t.Errorf("control: at 320pt no status line wrapped (at most %d line)", most)
			}
		})
	}
}

func TestStatusLinesBreakBetweenWords(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(&bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()})
	const s = "✓ Included with BibleText — or paste your own."
	if got := statusLines(s, 0, 12); len(got) != 1 || got[0] != s {
		t.Errorf("not yet laid out, the text is one line; got %q", got)
	}
	if got := statusLines("", 200, 12); got != nil {
		t.Errorf("no text, no lines; got %q", got)
	}
	// Just wide enough for "✓ Included with BibleText" and not the dash after
	// it: the dash goes down with "BibleText" rather than opening a line.
	w := fyne.MeasureText("✓ Included with BibleText", 12, fyne.TextStyle{}).Width + 1
	got := statusLines(s, w, 12)
	for _, l := range got {
		if strings.HasPrefix(l, "—") {
			t.Errorf("at %.1fpt a line opens with the dash: %q", w, got)
		}
	}
	if strings.Join(got, " ") != s {
		t.Errorf("at %.1fpt the lines %q are not the whole text", w, got)
	}
	// A word wider than the row has a line to itself rather than being cut.
	if got := statusLines("a extraordinarily b", 20, 12); len(got) != 3 || got[1] != "extraordinarily" {
		t.Errorf("a word wider than the row: %q", got)
	}
}

// THE SHEET IS MEASURED AGAIN WHEN A KEY STATUS CHANGES HEIGHT. A status
// taking a second line, when a saved key is cleared and the line that replaces
// it wraps on a narrow screen, makes the body taller. On a window with room for
// the whole sheet, the sheet has to grow with it, or its last rows sit below a
// fold nothing points to. Measured on windows tall enough that the sheet is
// never capped.
func TestSettingsSheetGrowsWithAKeyStatus(t *testing.T) {
	// wantWhole fails if the body wants more height than its view has.
	wantWhole := func(t *testing.T, popup *widget.PopUp, grewFrom float32, when string) {
		t.Helper()
		test.WidgetRenderer(popup).Layout(popup.Size())
		body := findScroll(popup.Content)
		if body == nil {
			t.Fatal("no scrolling body in the Settings sheet")
		}
		want := body.Content.MinSize().Height
		if want < grewFrom+10 {
			t.Fatalf("control: %s the body went from %.1fpt to %.1fpt; it should have grown", when, grewFrom, want)
		}
		if got := body.Size().Height; got < want-0.5 {
			t.Errorf("%s the body wants %.1fpt and its view is %.1fpt: the sheet was not measured again",
				when, want, got)
		}
	}
	bodyHeight := func(popup *widget.PopUp) float32 {
		return findScroll(popup.Content).Content.MinSize().Height
	}

	t.Run("the API.Bible status takes a second line", func(t *testing.T) {
		// The reader's own key is saved, and clearing it fails in a
		// credential store that cannot write: the status becomes the error
		// line, which wraps at 320pt where the saved line does not. The key
		// is still there, so nothing but the status's height asks for the
		// sheet to be measured again.
		app := test.NewApp()
		t.Cleanup(app.Quit)
		th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
		app.Settings().SetTheme(th)
		t.Setenv("BIBLE_API_KEY", "")
		prev := bundledBibleKeyEnc
		t.Cleanup(func() { bundledBibleKeyEnc = prev })
		bundledBibleKeyEnc = ""
		const key = "a-key-of-the-readers-own"
		failing := &keyStore{prefs: newFakePrefs(),
			secrets: &fakeSecrets{m: map[string]string{bibleKeyID: key}, failWrite: true}}
		prevShared := sharedKeys
		sharedKeys = func() *keyStore { return failing }
		t.Cleanup(func() { sharedKeys = prevShared })
		win := app.NewWindow("Settings")
		t.Cleanup(win.Close)
		win.Resize(fyne.NewSize(320, 3000))
		st := sampleState()
		st.window, st.theme, st.aiKeys = win, th, failing
		popup := pickerPopup(t, st, showAISettings)
		t.Cleanup(popup.Hide)

		var entry *widget.Entry
		walkTree(popup, func(o fyne.CanvasObject) {
			if e, ok := o.(*widget.Entry); ok && e.Text == key {
				entry = e
			}
		})
		if entry == nil {
			t.Fatal("no API.Bible key field holding the reader's key")
		}
		lines := statusLinesIn(popup)
		status := lines[len(lines)-1]
		if n := len(drawnLines(status)); n != 1 {
			t.Fatalf("control: with the key saved the API.Bible status %q should be one line at 320pt, got %d", status.text, n)
		}
		before := bodyHeight(popup)
		entry.SetText("")
		if n := len(drawnLines(status)); n < 2 {
			t.Fatalf("control: the status %q should take two lines at 320pt, got %d", status.text, n)
		}
		if failing.bibleAPIKey() != key {
			t.Fatal("control: the failed clear should leave the key in place")
		}
		wantWhole(t, popup, before, "once the status wrapped")
	})

	t.Run("an assistant status takes a second line", func(t *testing.T) {
		_, listed := holdKeyTests(t)
		app := test.NewApp()
		t.Cleanup(app.Quit)
		th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
		app.Settings().SetTheme(th)
		t.Setenv("BIBLE_API_KEY", "")
		fake := withFakeSharedKeys(t)
		fake.setAIEnabled(true)
		const key = "a-key-of-the-readers-own"
		fake.setAPIKey(defaultProviderID, key)
		win := app.NewWindow("Settings")
		t.Cleanup(win.Close)
		win.Resize(fyne.NewSize(320, 3000))
		st := sampleState()
		st.window, st.theme, st.aiKeys = win, th, fake
		popup := pickerPopup(t, st, showAISettings)
		t.Cleanup(popup.Hide)
		waitParked(t, listed, "the model list fetch for the saved key")

		var entry *widget.Entry
		walkTree(popup, func(o fyne.CanvasObject) {
			if e, ok := o.(*widget.Entry); ok && e.Text == key {
				entry = e
			}
		})
		if entry == nil {
			t.Fatal("no assistant key field holding the saved key")
		}
		status := statusLinesIn(popup)[0]
		if n := len(drawnLines(status)); n != 1 {
			t.Fatalf("control: with a key saved the assistant's status should be one line at 320pt, got %d", n)
		}
		before := bodyHeight(popup)
		entry.SetText("")
		if n := len(drawnLines(status)); n < 2 {
			t.Fatalf("control: the provider's hint %q should take two lines at 320pt, got %d", status.text, n)
		}
		wantWhole(t, popup, before, "once the hint wrapped")
	})
}

// fallbackRunes is the characters of s that neither UI face nor the emoji face
// has — the ones Fyne has to go to the system's fonts for.
func fallbackRunes(t *testing.T, s string) []rune {
	t.Helper()
	var faces []*sfnt.Font
	for _, r := range []fyne.Resource{
		(&bibleTheme{uiFonts: loadUIFonts()}).Font(fyne.TextStyle{}),
		theme.DefaultTextFont(),
		theme.DefaultEmojiFont(),
	} {
		if f, err := sfnt.Parse(r.Content()); err == nil {
			faces = append(faces, f)
		}
	}
	if len(faces) < 2 {
		t.Fatal("could not read the UI faces")
	}
	var buf sfnt.Buffer
	var out []rune
	for _, r := range s {
		found := false
		for _, f := range faces {
			if g, err := f.GlyphIndex(&buf, r); err == nil && g != 0 {
				found = true
				break
			}
		}
		if !found {
			out = append(out, r)
		}
	}
	return out
}

func TestSettingsTextIsDrawnAsItWasMeasured(t *testing.T) {
	// THE MECHANISM, on any machine: a text holding a character that comes
	// from a system font must not sit under a theme override, because the
	// override's font cache is not the one it is measured in.
	t.Run("no override over system-font text", func(t *testing.T) {
		_, _, popup := settingsWithIncludedKey(t, 1032, 1376, 1, nil)
		checked := 0
		var walk func(o fyne.CanvasObject, under bool)
		walk = func(o fyne.CanvasObject, under bool) {
			switch v := o.(type) {
			case *container.ThemeOverride:
				walk(v.Content, true)
			case *container.Scroll:
				walk(v.Content, under)
			case *widget.PopUp:
				walk(v.Content, under)
			case *fyne.Container:
				for _, c := range v.Objects {
					walk(c, under)
				}
			case *canvas.Text:
				if rs := fallbackRunes(t, v.Text); len(rs) > 0 {
					checked++
					if under {
						t.Errorf("%q holds %q, which comes from a system font, and is drawn under a theme "+
							"override — measured in one font cache, drawn in another", v.Text, string(rs))
					}
				}
			case fyne.Widget:
				for _, c := range test.WidgetRenderer(v).Objects() {
					walk(c, under)
				}
			}
		}
		walk(popup, false)
		if checked == 0 {
			t.Fatal("no text in the sheet needed a system font; the check examined nothing (the ✓ status should)")
		}
	})

	// THE SYMPTOM, on a host whose system fonts reproduce it: once the
	// header's ▾ has been measured, the ✓ status line must draw exactly as the
	// same text does on its own — every column of ink where a lone copy puts
	// it, the full stop included.
	t.Run("the status line draws to its end", func(t *testing.T) {
		const scale = 2
		// What the app has always done before Settings opens: the header's
		// translation name, with its ▾, measured through the app's cache.
		_, win, popup := settingsWithIncludedKey(t, 1032, 1376, scale, func() {
			fyne.MeasureText("World English Bible  ▾", 13, fyne.TextStyle{})
		})
		lines := statusLinesIn(popup)
		if len(lines) != 2 {
			t.Fatalf("found %d status lines", len(lines))
		}
		drawn := drawnLines(lines[1])
		if len(drawn) != 1 {
			t.Fatalf("at 13-inch width the status should be one line, got %d", len(drawn))
		}
		tx := drawn[0]
		p := fyne.CurrentApp().Driver().AbsolutePositionForObject(tx)
		m := tx.MinSize()
		img := win.Canvas().Capture()
		// A margin all round: the painter draws a popup's content a couple of
		// points from where the driver reports it, and the comparison below is
		// of shape, not of place.
		band := image.Rect(int(p.X*scale)-16, int(p.Y*scale)-8, int((p.X+m.Width)*scale)+16, int((p.Y+m.Height)*scale)+8)
		inSheet := inkColumns(img, band)

		ref := canvas.NewText(tx.Text, tx.Color)
		ref.TextSize = tx.TextSize
		rw := test.NewWindow(nil)
		defer rw.Close()
		rw.SetPadded(false)
		rw.Canvas().(test.WindowlessCanvas).SetScale(scale)
		rw.SetContent(container.NewWithoutLayout(ref))
		ref.Move(fyne.NewPos(0, 0))
		ref.Resize(m)
		rw.Resize(m.AddWidthHeight(4, 0))
		rimg := rw.Canvas().Capture()
		alone := inkColumns(rimg, rimg.Bounds())

		if len(alone) < 200 {
			t.Fatalf("the lone copy drew only %d columns of ink; the comparison would prove nothing", len(alone))
		}
		if miss := columnMismatch(inSheet, alone); miss > 3 {
			t.Errorf("the sheet draws %q differently from the same text on its own: %d of %d ink columns "+
				"differ (%d in the sheet) — drawn in a font it was not measured in, and cut at the width it measured",
				tx.Text, miss, len(alone), len(inSheet))
		}
	})
}

// inkColumns lists, relative to r's left edge, the columns of r holding a pixel
// that differs from r's ground.
func inkColumns(img image.Image, r image.Rectangle) map[int]bool {
	r = r.Intersect(img.Bounds())
	ground := dominant(img, r)
	gr, gg, gb, _ := ground.RGBA()
	out := map[int]bool{}
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			d := absDiff(cr, gr) + absDiff(cg, gg) + absDiff(cb, gb)
			if d > 3*40*257 {
				out[x-r.Min.X] = true
				break
			}
		}
	}
	return out
}

// columnMismatch is how many ink columns the two sets disagree on at the
// offset that aligns them best. A text drawn in another font for one of its
// characters cannot be aligned: everything after that character moves, and
// the character itself does not.
func columnMismatch(a, b map[int]bool) int {
	best := -1
	for off := -40; off <= 40; off++ {
		n := 0
		for x := range a {
			if !b[x+off] {
				n++
			}
		}
		for x := range b {
			if !a[x-off] {
				n++
			}
		}
		if best < 0 || n < best {
			best = n
		}
	}
	return best
}
