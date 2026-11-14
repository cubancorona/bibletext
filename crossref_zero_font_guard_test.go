package bibletext

import (
	"encoding/binary"
	"testing"
)

func TestPlainZeroRejectsChangedGlyphGeometry(t *testing.T) {
	for _, tc := range []struct {
		name string
		font []byte
	}{
		{"regular", atkinsonRegular},
		{"bold", atkinsonBold},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if patchAtkinsonPlainZero(tc.font) == nil {
				t.Fatal("embedded zero was rejected")
			}
			font := append([]byte(nil), tc.font...)
			cmapOff, cmapLen, _, ok := sfntTable(font, "cmap")
			if !ok {
				t.Fatal("missing cmap")
			}
			gid, ok := cmapGlyphID(font[cmapOff:cmapOff+cmapLen], '0')
			if !ok {
				t.Fatal("missing zero")
			}
			headOff, _, _, ok := sfntTable(font, "head")
			if !ok {
				t.Fatal("missing head")
			}
			locaOff, _, _, ok := sfntTable(font, "loca")
			if !ok {
				t.Fatal("missing loca")
			}
			var start, end uint32
			switch binary.BigEndian.Uint16(font[headOff+50:]) {
			case 0:
				start = uint32(binary.BigEndian.Uint16(font[locaOff+uint32(gid)*2:])) * 2
				end = uint32(binary.BigEndian.Uint16(font[locaOff+uint32(gid+1)*2:])) * 2
			case 1:
				start = binary.BigEndian.Uint32(font[locaOff+uint32(gid)*4:])
				end = binary.BigEndian.Uint32(font[locaOff+uint32(gid+1)*4:])
			default:
				t.Fatal("invalid loca format")
			}
			glyfOff, glyfLen, glyfEntry, ok := sfntTable(font, "glyf")
			if !ok {
				t.Fatal("missing glyf")
			}
			raw := font[glyfOff+start : glyfOff+end]
			before, _, ok := decodeSimpleGlyph(raw)
			if !ok || len(before) != 3 {
				t.Fatal("zero must have three contours")
			}

			// Walk past endpoints, instructions and compressed flags to the
			// first x delta. Change one encoded coordinate without changing
			// the point count, contour structure or glyph record length.
			pos := 10 + len(before)*2
			pointCount := int(binary.BigEndian.Uint16(raw[pos-2:])) + 1
			instructionLength := int(binary.BigEndian.Uint16(raw[pos:]))
			pos += 2 + instructionLength
			var flags []byte
			for len(flags) < pointCount {
				flag := raw[pos]
				pos++
				flags = append(flags, flag)
				if flag&0x08 != 0 {
					repeats := int(raw[pos])
					pos++
					for i := 0; i < repeats; i++ {
						flags = append(flags, flag)
					}
				}
			}
			// Both embedded zeros start with a signed 16-bit x delta,
			// followed by a negative one-byte delta. Compensating the
			// second delta moves only the first point by one font unit.
			if flags[0]&0x12 != 0 || flags[1]&0x12 != 0x02 || raw[pos+2] == 255 {
				t.Fatal("unexpected zero coordinate encoding")
			}
			binary.BigEndian.PutUint16(raw[pos:], binary.BigEndian.Uint16(raw[pos:])+1)
			raw[pos+2]++
			after, _, ok := decodeSimpleGlyph(raw)
			if !ok || len(after) != len(before) {
				t.Fatal("changed glyph lost its contour structure")
			}
			for i := range before {
				if len(after[i]) != len(before[i]) {
					t.Fatal("changed glyph lost points")
				}
				for j, point := range before[i] {
					if i == 0 && j == 0 {
						point.x++
					}
					if after[i][j] != point {
						t.Fatal("control did not move exactly the first zero point")
					}
				}
			}

			binary.BigEndian.PutUint32(font[glyfEntry+4:], sfntChecksum(font[glyfOff:glyfOff+glyfLen]))
			binary.BigEndian.PutUint32(font[headOff+8:], 0)
			binary.BigEndian.PutUint32(font[headOff+8:], 0xB1B0AFBA-sfntChecksum(font))
			if patchAtkinsonPlainZero(font) != nil {
				t.Fatal("changed zero geometry was accepted")
			}
		})
	}
}
