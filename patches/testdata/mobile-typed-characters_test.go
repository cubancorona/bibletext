//go:build !windows || !ci

package mobile

// Regression tests for patches/fyne-2.7.4-mobile-typed-characters.patch.
// scripts/setup-fyne-patch.sh copies this file into the generated toolkit as
// internal/driver/mobile/bt_typed_characters_test.go; run it from there with
//
//	go test ./internal/driver/mobile -run '^TestBTTyped' -count=1
//
// A keyboard on Android or iOS hands the driver a string, and the driver types
// it into the focused field one character at a time: app.keyboardTyped makes a
// key press of each character, and the driver's event loop passes each press
// to typeDownCanvas. Stock Fyne put every character through strconv.IsPrint
// there, which drops format characters (the zero-width joiner and non-joiner
// among them), every space but U+0020, private-use characters and characters
// newer than Go's Unicode tables. A joined emoji typed into a note arrived as
// separate emoji, and Persian and Indic text lost its joiners.
//
// The build constraint is canvas_test.go's, whose TestMain runs the driver's
// queue: the field is typed into on the toolkit's main goroutine, as the
// event loop does.

import (
	"io"
	"log"
	"os"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/internal/driver/mobile/event/key"
	"fyne.io/fyne/v2/widget"
)

// btTypeIntoField focuses an empty entry and types s into it as
// app.keyboardTyped delivers a keyboard's string: one key press per
// character, each through typeDownCanvas. keyboardTyped gives a key code to
// the ASCII letters, digits, space, newline and some punctuation, and
// key.CodeUnknown to every other character, so the fixtures hold no
// printable ASCII.
func btTypeIntoField(t *testing.T, s string) string {
	t.Helper()
	// Focusing an entry asks the host's stand-in device for a keyboard, which
	// logs that it has none.
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	c := newCanvas(fyne.CurrentDevice()).(*canvas)
	entry := widget.NewEntry()
	c.SetContent(entry)
	var got string
	d.DoFromGoroutine(func() {
		c.Focus(entry)
		for _, r := range s {
			if r >= 0x20 && r < 0x7f {
				t.Errorf("fixture %+q holds printable ASCII %q, which keyboardTyped may send with a key code", s, r)
			}
			d.typeDownCanvas(c, r, key.CodeUnknown, 0)
		}
		got = entry.Text
	}, true)
	return got
}

func TestBTTypedJoinersReachTheField(t *testing.T) {
	for name, typed := range map[string]string{
		"a family, three people joined by U+200D":           "\U0001F468‍\U0001F469‍\U0001F467",
		"a profession, a person and a symbol joined":        "\U0001F469‍⚕️",
		"a rainbow flag, a flag and a rainbow joined":       "\U0001F3F3️‍\U0001F308",
		"a Persian word with its zero-width non-joiner":     "می‌خواهم",
		"a Devanagari half form asked for with U+200D":      "क्‍ष",
		"a Devanagari conjunct refused with U+200C":         "क्‌ष",
		"two emoji kept apart by U+200C, as a keyboard may": "\U0001F468‌\U0001F469",
	} {
		t.Run(name, func(t *testing.T) {
			if got := btTypeIntoField(t, typed); got != typed {
				t.Errorf("typed %+q; the field holds %+q", typed, got)
			}
		})
	}
}

// The other characters strconv.IsPrint refused, each of which a keyboard can
// type: the spaces input methods insert, a format character, a private-use
// character, and one that Go's Unicode 15.0 tables do not know.
func TestBTTypedSpacesAndNewerCharactersReachTheField(t *testing.T) {
	for name, typed := range map[string]string{
		"a no-break space":                         " ",
		"a narrow no-break space":                  " ",
		"the ideographic space":                    "　",
		"a soft hyphen":                            "­",
		"a private-use character":                  "",
		"an emoji added in Unicode 16.0 (U+1FAE9)": "\U0001FAE9",
	} {
		t.Run(name, func(t *testing.T) {
			if got := btTypeIntoField(t, typed); got != typed {
				t.Errorf("typed %+q; the field holds %+q", typed, got)
			}
		})
	}
}

// What the patch keeps out: a control character reaches no field, as before.
// keyboardTyped sends a newline with its own key code, which types the
// Return key rather than the character.
func TestBTTypedControlCharactersStayOutOfTheField(t *testing.T) {
	typed := "\x00\x1b\x7f\u0085\u009f"
	if got := btTypeIntoField(t, typed); got != "" {
		t.Errorf("typed the controls %+q; the field holds %+q, want nothing", typed, got)
	}
}

// With nothing focused the driver hands each character to the canvas's own
// handler instead, through the same filter.
func TestBTTypedJoinersReachTheCanvasWithNothingFocused(t *testing.T) {
	c := newCanvas(fyne.CurrentDevice()).(*canvas)
	var got []rune
	c.SetOnTypedRune(func(r rune) { got = append(got, r) })
	typed := "\U0001F468‍\U0001F469‍\U0001F467"
	d.DoFromGoroutine(func() {
		for _, r := range typed {
			d.typeDownCanvas(c, r, key.CodeUnknown, 0)
		}
	}, true)
	if string(got) != typed {
		t.Errorf("typed %+q with nothing focused; the canvas was handed %+q", typed, string(got))
	}
}

// btGLFWTypes is the rule GLFW's _glfwInputChar (src/input.c) applies on
// every desktop before Fyne's glfw driver types a character: a C0 control,
// DEL or a C1 control is dropped, and everything else is handed on.
func btGLFWTypes(r rune) bool {
	return !(r < 32 || (r > 126 && r < 160))
}

// The mobile driver types the characters GLFW's _glfwInputChar passes on all
// three desktops. The Mac's GLFW also skips U+F700 to U+F7FF, the range AppKit
// gives function keys, before that call; no mobile path is known to send it.
func TestBTTypedCharactersFollowTheDesktopRule(t *testing.T) {
	var wrong []rune
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if !utf8.ValidRune(r) {
			continue // a surrogate half: ranging over a string never yields one
		}
		// typeDownCanvas types the result when it is above zero.
		got := runeToPrintable(r)
		if got > 0 && got != r {
			t.Fatalf("runeToPrintable(%U) = %U: a character is typed as itself or not at all", r, got)
		}
		if typed := got > 0; typed != btGLFWTypes(r) {
			wrong = append(wrong, r)
		}
	}
	if len(wrong) > 0 {
		shown := wrong
		if len(shown) > 12 {
			shown = shown[:12]
		}
		t.Errorf("%d characters are typed on one side and not the other, first %U", len(wrong), shown)
	}

	// Not characters at all. keyboardTyped never sends one, but the Android
	// key path can: its rune lookup returns -1 for a key with no character,
	// and a dead key's value carries KeyCharacterMap.COMBINING_ACCENT, the
	// sign bit.
	for _, r := range []rune{-1, -0x80000000 | 0x0301, 0xD800, 0xDFFF, utf8.MaxRune + 1} {
		if got := runeToPrintable(r); got != 0 {
			t.Errorf("runeToPrintable(%d) = %d, want 0: it is not a character", r, got)
		}
	}
}
