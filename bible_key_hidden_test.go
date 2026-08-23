package bibletext

// The bundled API.Bible key must never be readable from Settings.
//
// It is a PasswordEntry, which hides characters but keeps them: one tap on the
// reveal eye would print the project's production credential on screen. That key
// is shared, on a shared quota, so a reader who lifts it costs every other
// reader — and unlike the copy inside the binary, this needs no skill at all.
//
// A key the READER pasted is a different matter: it is theirs, and being able to
// look at it is how they spot a truncated paste. So the rule is provenance, not
// secrecy in general.

import (
	"encoding/base64"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func findKeyEntry(o fyne.CanvasObject) *widget.Entry {
	switch v := o.(type) {
	case *widget.Entry:
		return v
	case *fyne.Container:
		for _, c := range v.Objects {
			if e := findKeyEntry(c); e != nil {
				return e
			}
		}
	}
	return nil
}

func TestBundledBibleKeyIsNotReadableInSettings(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)

	const bundled = "pretend-this-shipped-with-the-app"
	fake.setBibleAPIKey(bundled)

	st := sampleState()
	st.theme = th
	// bibleKeySection reads state.keys(), which builds its OWN store — pointing
	// it at the fake is what makes the assertions mean anything.
	st.aiKeys = fake
	pal := st.pal()

	// Compile a bundled key in for the duration of this test. Without it the
	// branch under test never runs — `go test` builds carry no embedded key, and
	// the first version of this test passed while proving nothing.
	prev := bundledBibleKeyEnc
	defer func() { bundledBibleKeyEnc = prev }()
	bundledBibleKeyEnc = obfuscateForTest(bundled)
	if bundledBibleKey() != bundled {
		t.Fatalf("test setup: bundledBibleKey() = %q, want %q", bundledBibleKey(), bundled)
	}
	fake.setBibleAPIKey(bundled)
	if !fake.usingBundledBibleKey() {
		t.Fatal("test setup: the store should now hold the bundled key")
	}

	rows, _ := bibleKeySection(st, pal, nil)
	e := findKeyEntry(rows)
	if e == nil {
		t.Fatal("no key field in the section")
	}
	if strings.Contains(e.Text, bundled) {
		t.Error("the bundled key is sitting in the field — one tap on reveal exposes it")
	}
	if e.Text != "" {
		t.Errorf("field should be empty for a bundled key, got %q", e.Text)
	}
	// The reader must still be TOLD the key is included — but the placeholder is
	// not where that has to live, and it is the wrong place for it: a sentence
	// long enough to say "included with BibleText" AND "paste your own to replace
	// it" measured 458pt inside a field with ~199pt of usable width on a 320pt
	// phone, and simply ran off the end. The message now sits on the status line
	// under the box, so this asserts the SECTION says it rather than pinning it
	// to one widget.
	var said bool
	for _, s := range collectText(rows) {
		if strings.Contains(strings.ToLower(s), "included") {
			said = true
			break
		}
	}
	if !said {
		t.Errorf("nothing in the section tells the reader the key is included; texts = %q, placeholder = %q",
			collectText(rows), e.PlaceHolder)
	}
	// And whatever the placeholder says, it has to FIT. Measured at the app's own
	// body size (bibleTheme.Size(SizeNameText) = 18) — theme.TextSize() is the
	// stock 14 and would flatter every string by 29%, which is how the overrun got
	// through in the first place.
	const narrowestUsable = 199 // 320pt phone: box inset, both paddings, reveal icon
	if w := fyne.MeasureText(e.PlaceHolder, 18, fyne.TextStyle{}).Width; w > narrowestUsable {
		t.Errorf("placeholder %q measures %.1fpt, overruns the %dpt field on a 320pt phone",
			e.PlaceHolder, w, narrowestUsable)
	}
}

// Hiding the characters must not disable the controls that act on the key.
func TestBundledKeyStillClearable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	// The regression this guards is BUNDLED-key specific: with the bundled key in
	// force the field renders EMPTY, so emptying it fires no OnChanged and Clear
	// would leave the key in the store. Testing with a reader-typed key exercises
	// the easy path and proves nothing, which is what the first draft did.
	prev := bundledBibleKeyEnc
	defer func() { bundledBibleKeyEnc = prev }()
	const bundled = "the-bundled-one"
	bundledBibleKeyEnc = obfuscateForTest(bundled)
	fake.setBibleAPIKey(bundled)

	st := sampleState()
	st.theme = th
	st.aiKeys = fake
	if !fake.usingBundledBibleKey() {
		t.Fatal("setup: the store should be holding the bundled key")
	}
	rows, _ := bibleKeySection(st, st.pal(), nil)

	clear := findTreeButton(rows, "Clear")
	if clear == nil {
		t.Fatal("no Clear button")
	}
	if clear.Disabled() {
		t.Fatal("Clear is disabled while a key is stored")
	}
	clear.OnTapped()
	if got := fake.bibleAPIKey(); got != "" {
		t.Errorf("Clear left the key in the store: %q", got)
	}
}

// obfuscateForTest mirrors what [redacted-retired-private-reference] generates.
func obfuscateForTest(key string) string {
	raw := []byte(key)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ bundledKeyMask[i%len(bundledKeyMask)]
	}
	return base64.StdEncoding.EncodeToString(out)
}

// The "Get a key ↗" link must sit top-right of the REAL key box, on the label
// row, clear of the label — checked against the section the app actually builds
// rather than against a lookalike tree the test assembles itself. The first
// version of this test did the latter and would have passed no matter what
// bibleKeySection did.
func TestRealKeySectionPutsTheLinkTopRight(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	th := &bibleTheme{fonts: loadBookFonts(), uiFonts: loadUIFonts()}
	app.Settings().SetTheme(th)
	t.Setenv("BIBLE_API_KEY", "")
	fake := withFakeSharedKeys(t)
	st := sampleState()
	st.theme = th
	st.aiKeys = fake

	rows, _ := bibleKeySection(st, st.pal(), nil)
	// The tree must live in a canvas before absolute positions mean anything —
	// off-canvas every object reports the same origin, which is how the earlier
	// draft of this test "passed".
	win := app.NewWindow("keys")
	defer win.Close()
	win.Resize(fyne.NewSize(400, 700))
	win.SetContent(rows)
	rows.Resize(fyne.NewSize(354, rows.MinSize().Height))

	var link *widget.Hyperlink
	var label *widget.Label
	var walk func(o fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Hyperlink:
			link = v
		case *widget.Label:
			if label == nil && strings.Contains(v.Text, "key") {
				label = v
			}
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c)
			}
		}
	}
	walk(rows)
	if link == nil || label == nil {
		t.Fatalf("real section missing link=%v label=%v", link != nil, label != nil)
	}
	drv := fyne.CurrentApp().Driver()
	lp := drv.AbsolutePositionForObject(link)
	bp := drv.AbsolutePositionForObject(label)
	if lp.Y > bp.Y+5 {
		t.Errorf("link dropped below the label row (link y=%v, label y=%v)", lp.Y, bp.Y)
	}
	if lp.X <= bp.X {
		t.Errorf("link is not to the right of the label (link x=%v, label x=%v)", lp.X, bp.X)
	}
}
