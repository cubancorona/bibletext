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
	if !strings.Contains(strings.ToLower(e.PlaceHolder), "included") {
		t.Errorf("placeholder should say the key is included, got %q", e.PlaceHolder)
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
	fake.setBibleAPIKey("some-key-in-the-store")

	st := sampleState()
	st.theme = th
	st.aiKeys = fake
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

// obfuscateForTest mirrors what scripts/embed-bible-key.sh generates.
func obfuscateForTest(key string) string {
	raw := []byte(key)
	out := make([]byte, len(raw))
	for i, b := range raw {
		out[i] = b ^ bundledKeyMask[i%len(bundledKeyMask)]
	}
	return base64.StdEncoding.EncodeToString(out)
}
