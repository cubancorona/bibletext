package bibletext

import (
	"fyne.io/fyne/v2"
	"testing"
)

func TestNativeComposeKeyboardAreaIntersectsDriverInsets(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		size, scale, px, want float32
	}{
		{"dialog owns keyboard", 760, 2, 300, 630},
		{"driver already excludes keyboard", 480, 2, 300, 480},
		{"hidden", 760, 2, 0, 760},
		{"unsized scale", 760, 0, 300, 760},
		{"no room", 760, 1, 900, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pos, size := composeAreaAboveKeyboard(fyne.NewPos(4, 20), fyne.NewSize(360, tc.size), 800, tc.scale, tc.px)
			if pos != fyne.NewPos(4, 20) || size.Width != 360 || size.Height != tc.want {
				t.Fatalf("area is %v/%v", pos, size)
			}
		})
	}
}
