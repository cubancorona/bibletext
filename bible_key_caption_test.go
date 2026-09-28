package bibletext

// THE NKJV CAPTION SHOWS IN EVERY KEY STATE.
//
// Under the API.Bible key card the caption says how the NKJV downloads with a
// key of the reader's own. It shows whatever key is in force: the key that
// ships with the app, none at all, the reader's own, or the included one
// cleared. With the included key in force it reads together with the "Get a
// key" link on the section's label row and the status line's "or paste your
// own": the included key works as it is, and a key of the reader's own is the
// other way to the NKJV. The gap after it is the one every captioned section
// leaves (sheetGapAfterCaption).

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

const nkjvCaptionText = "The New King James Version downloads with your own free API.Bible key — " +
	"create a key, add the NKJV to it, then choose NKJV from the translation picker."

// nkjvCaptionIn finds the caption in o and reports whether it is drawn: it and
// everything above it visible. A caption left visible inside a hidden
// container is not drawn, so its own Visible alone is not enough.
func nkjvCaptionIn(o fyne.CanvasObject) (found *widget.RichText, drawn bool) {
	var walk func(o fyne.CanvasObject, shown bool)
	walk = func(o fyne.CanvasObject, shown bool) {
		if o == nil || found != nil {
			return
		}
		shown = shown && o.Visible()
		if rt, ok := o.(*widget.RichText); ok && segmentText(rt.Segments) == nkjvCaptionText {
			found, drawn = rt, shown
			return
		}
		switch v := o.(type) {
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c, shown)
			}
		case *container.Scroll:
			walk(v.Content, shown)
		case *container.ThemeOverride:
			walk(v.Content, shown)
		case *widget.PopUp:
			walk(v.Content, shown)
		}
	}
	walk(o, true)
	return found, drawn
}

// openSettingsForCaption opens the real Settings sheet with the assistant off,
// so the API.Bible key row is the only key row, on a store that prepare (if
// not nil) has put into the state under test. included decides whether the
// build carries a key of its own.
func openSettingsForCaption(t *testing.T, included bool, prepare func(*keyStore)) (*keyStore, *widget.PopUp) {
	t.Helper()
	app := test.NewApp()
	t.Cleanup(app.Quit)
	th := &bibleTheme{fonts: loadReadingFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	fake.setAIEnabled(false)
	prev := bundledBibleKeyEnc
	t.Cleanup(func() { bundledBibleKeyEnc = prev })
	bundledBibleKeyEnc = ""
	if included {
		bundledBibleKeyEnc = obfuscateForTest("the-included-key")
	}
	if prepare != nil {
		prepare(fake)
	}
	win := app.NewWindow("Settings")
	t.Cleanup(win.Close)
	win.Resize(fyne.NewSize(834, 2400))
	st := sampleState()
	st.window, st.theme, st.aiKeys = win, th, fake
	popup := pickerPopup(t, st, showAISettings)
	t.Cleanup(popup.Hide)
	test.WidgetRenderer(popup).Layout(popup.Size())
	return fake, popup
}

// bibleKeyControls is the API.Bible key field and its Clear button in the
// sheet, which with the assistant off hold the only ones of each.
func bibleKeyControls(t *testing.T, popup *widget.PopUp) (*widget.Entry, *widget.Button) {
	t.Helper()
	var entries []*widget.Entry
	var clears []*widget.Button
	walkTree(popup, func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Entry:
			if v.Password {
				entries = append(entries, v)
			}
		case *widget.Button:
			if v.Text == "Clear" {
				clears = append(clears, v)
			}
		}
	})
	if len(entries) != 1 || len(clears) != 1 {
		t.Fatalf("setup: want the API.Bible key field and Clear alone, found %d key fields and %d Clear buttons",
			len(entries), len(clears))
	}
	return entries[0], clears[0]
}

func TestNKJVCaptionShowsInEveryKeyState(t *testing.T) {
	const own = "a-key-of-the-readers-own"
	for _, tc := range []struct {
		name     string
		included bool
		prepare  func(*keyStore)
		// act, if not nil, changes the key in the open sheet.
		act func(t *testing.T, popup *widget.PopUp)
		// bundled and key are the state the case is named for; with bundled
		// set, the key is the included one.
		bundled bool
		key     string
	}{
		{name: "the included key in force", included: true, bundled: true},
		{name: "no key at all", included: false},
		{name: "the reader's own key", included: true,
			prepare: func(k *keyStore) { k.setBibleAPIKey(own) },
			key:     own},
		{name: "the included key cleared", included: true,
			prepare: func(k *keyStore) { k.noteBibleKeyCleared(true) }},
		{name: "the reader's own key pasted over the included one", included: true,
			act: func(t *testing.T, popup *widget.PopUp) {
				entry, _ := bibleKeyControls(t, popup)
				entry.SetText(own)
			},
			key: own},
		{name: "the included key cleared with Clear", included: true,
			act: func(t *testing.T, popup *widget.PopUp) {
				_, clearBtn := bibleKeyControls(t, popup)
				clearBtn.OnTapped()
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake, popup := openSettingsForCaption(t, tc.included, tc.prepare)
			bibleKeyControls(t, popup)
			if tc.act != nil {
				if _, drawn := nkjvCaptionIn(popup); !drawn {
					t.Fatal("the NKJV caption is hidden before the key changes; it shows in every key state")
				}
				tc.act(t, popup)
				test.WidgetRenderer(popup).Layout(popup.Size())
			}
			want := tc.key
			if tc.bundled {
				want = bundledBibleKey()
			}
			if got := fake.usingBundledBibleKey(); got != tc.bundled {
				t.Fatalf("setup: the included key in force is %v, want %v", got, tc.bundled)
			}
			if got := fake.bibleAPIKey(); got != want {
				t.Fatalf("setup: the key in force is %q, want %q", got, want)
			}
			note, drawn := nkjvCaptionIn(popup)
			if note == nil {
				t.Fatal("the NKJV caption is not in the Settings sheet")
			}
			if !drawn {
				t.Error("the NKJV caption is hidden; it shows in every key state")
			}
			if h := note.Size().Height; h <= 0 {
				t.Errorf("the NKJV caption is laid out %.1fpt tall", h)
			}
		})
	}
}

// The caption closes the TRANSLATIONS section, and the gap after it is the one
// after every caption: a section that ends at a card leaves that much more, the
// caption's own trailing padding (captionTrailingPad), so the two render the
// same gap.
func TestNKJVCaptionIsFollowedByTheCaptionGap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		included bool
	}{
		{"the included key in force", true},
		{"no key at all", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, popup := openSettingsForCaption(t, tc.included, nil)
			var form *fyne.Container
			walkTree(popup, func(n fyne.CanvasObject) {
				if c, ok := n.(*fyne.Container); ok && form == nil {
					for _, o := range c.Objects {
						if tx, ok := o.(*canvas.Text); ok && tx.Text == "TRANSLATIONS" {
							form = c
						}
					}
				}
			})
			if form == nil {
				t.Fatal("no Settings form holding the TRANSLATIONS label")
			}
			at := func(label string) int {
				for i, o := range form.Objects {
					if tx, ok := o.(*canvas.Text); ok && tx.Text == label {
						return i
					}
				}
				t.Fatalf("no %s label in the form", label)
				return -1
			}
			bottom := func(o fyne.CanvasObject) float32 { return o.Position().Y + o.Size().Height }
			tr, rd, sn := at("TRANSLATIONS"), at("READING"), at("SHARED NOTES")
			note, drawn := nkjvCaptionIn(popup)
			if note == nil || !drawn {
				t.Fatal("the caption should be drawn")
			}
			if form.Objects[tr+2] != fyne.CanvasObject(note) {
				t.Fatal("the caption should follow the TRANSLATIONS card")
			}
			cardToLabel := form.Objects[sn].Position().Y - bottom(form.Objects[rd+1])
			captionToLabel := form.Objects[rd].Position().Y - bottom(note)
			if want := cardToLabel - captionTrailingPad; math.Abs(float64(captionToLabel-want)) > 0.5 {
				t.Errorf("%.1fpt from the caption to READING, want %.1fpt "+
					"(the %.1fpt after READING's card less the caption's own %dpt)",
					captionToLabel, want, cardToLabel, captionTrailingPad)
			}
		})
	}
}
