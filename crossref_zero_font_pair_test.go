package bibletext

import "testing"

func TestPlainZeroFacesArePublishedTogether(t *testing.T) {
	invalid := []byte("not a font")
	for _, tc := range []struct {
		name          string
		regular, bold []byte
		available     bool
	}{
		{"both valid", atkinsonRegular, atkinsonBold, true},
		{"regular unavailable", invalid, atkinsonBold, false},
		{"bold unavailable", atkinsonRegular, invalid, false},
		{"both unavailable", invalid, invalid, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			regular, bold := makePlainZeroFaces(tc.regular, tc.bold)
			if (regular != nil) != tc.available || (bold != nil) != tc.available {
				t.Fatalf("plain-zero pair: regular=%v bold=%v; both must be available=%v", regular != nil, bold != nil, tc.available)
			}
			if tc.available && (len(regular.Content()) == 0 || len(bold.Content()) == 0) {
				t.Fatal("available face has no font data")
			}
		})
	}
}
