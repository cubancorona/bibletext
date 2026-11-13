package bibletext

// Plain-zero cuts of the chrome face (Atkinson Hyperlegible) for references
// and printed citation lines in the cross-references panel — S20.
//
// Atkinson Hyperlegible draws '0' with an internal reverse diagonal bar so a
// standalone zero cannot be misread as capital O in general UI chrome. In a
// dense run of Scripture citations ("Prov. 12:4; 31:10–31", "1 John 4:10"),
// where every '0' already stands among colons and numerals, the diagonal bar
// reads as visual noise across two-digit chapters and verses. Rather than ship
// duplicate font binaries or alter '0' in the rest of the app's chrome, we
// derive regular and bold StaticResources once in memory from the embedded
// Atkinson TTFs, merging the two split counter halves of glyph '0' back into a
// single smooth inner oval while leaving every metric, kerning pair, and other
// glyph untouched.

import (
	"encoding/binary"
	"sync"

	"fyne.io/fyne/v2"
)

var (
	plainZeroOnce    sync.Once
	plainZeroRegular fyne.Resource
	plainZeroBold    fyne.Resource
)

func initPlainZeroFaces() {
	plainZeroOnce.Do(func() {
		if b := patchAtkinsonPlainZero(atkinsonRegular); len(b) > 0 {
			plainZeroRegular = fyne.NewStaticResource("AtkinsonHyperlegible-PlainZero-Regular.ttf", b)
		}
		if b := patchAtkinsonPlainZero(atkinsonBold); len(b) > 0 {
			plainZeroBold = fyne.NewStaticResource("AtkinsonHyperlegible-PlainZero-Bold.ttf", b)
		}
	})
}

func crossRefPlainZeroRegular() fyne.Resource {
	initPlainZeroFaces()
	return plainZeroRegular
}

func crossRefPlainZeroBold() fyne.Resource {
	initPlainZeroFaces()
	return plainZeroBold
}

// crossRefZeroFace returns the plain-zero Atkinson cut matching style when the
// active theme uses Atkinson Hyperlegible for that style, or nil otherwise so
// tests running under Fyne's default theme keep that theme's own metrics.
func crossRefZeroFace(style fyne.TextStyle) fyne.Resource {
	app := fyne.CurrentApp()
	if app == nil || app.Settings() == nil || app.Settings().Theme() == nil {
		return nil
	}
	f := app.Settings().Theme().Font(style)
	if f == nil {
		return nil
	}
	switch f.Name() {
	case "AtkinsonHyperlegible-Regular.ttf":
		return crossRefPlainZeroRegular()
	case "AtkinsonHyperlegible-Bold.ttf":
		return crossRefPlainZeroBold()
	default:
		return nil
	}
}

type ttfPoint struct {
	x, y int16
	on   bool
}

func patchAtkinsonPlainZero(src []byte) []byte {
	if len(src) < 12 {
		return nil
	}
	cmapOff, cmapLen, _, okCmap := sfntTable(src, "cmap")
	headOff, headLen, _, okHead := sfntTable(src, "head")
	maxpOff, maxpLen, _, okMaxp := sfntTable(src, "maxp")
	locaOff, locaLen, _, okLoca := sfntTable(src, "loca")
	glyfOff, glyfLen, glyfEntry, okGlyf := sfntTable(src, "glyf")
	if !okCmap || !okHead || !okMaxp || !okLoca || !okGlyf {
		return nil
	}
	if headLen < 54 || maxpLen < 6 {
		return nil
	}
	gid, ok := cmapGlyphID(src[cmapOff:cmapOff+cmapLen], '0')
	if !ok {
		return nil
	}
	numGlyphs := binary.BigEndian.Uint16(src[maxpOff+4 : maxpOff+6])
	if gid >= numGlyphs {
		return nil
	}
	idxFmt := int16(binary.BigEndian.Uint16(src[headOff+50 : headOff+52]))
	loca := src[locaOff : locaOff+locaLen]
	var g0, g1 uint32
	switch idxFmt {
	case 0:
		if int(gid+1)*2+2 > len(loca) {
			return nil
		}
		g0 = uint32(binary.BigEndian.Uint16(loca[gid*2:gid*2+2])) * 2
		g1 = uint32(binary.BigEndian.Uint16(loca[(gid+1)*2:(gid+1)*2+2])) * 2
	case 1:
		if int(gid+1)*4+4 > len(loca) {
			return nil
		}
		g0 = binary.BigEndian.Uint32(loca[gid*4 : gid*4+4])
		g1 = binary.BigEndian.Uint32(loca[(gid+1)*4 : (gid+1)*4+4])
	default:
		return nil
	}
	if g1 <= g0 || g1 > glyfLen {
		return nil
	}
	raw := src[glyfOff+g0 : glyfOff+g1]
	contours, bbox, ok := decodeSimpleGlyph(raw)
	if !ok || len(contours) != 3 {
		return nil
	}
	c0, c1, c2 := contours[0], contours[1], contours[2]
	if len(c1) < 6 || len(c2) < 9 {
		return nil
	}
	// Contour 0 is the outer clockwise oval; contours 1 and 2 are the two
	// counter-clockwise D-halves split by the diagonal bar. Rejoin their
	// surviving outer arcs and mirror the two missing quadrants to restore a
	// symmetric inner counter of the face's exact stroke weight.
	inner := []ttfPoint{
		c2[0],
		{x: c1[4].x, y: c2[8].y, on: false},
		{x: c1[3].x, y: c2[7].y, on: false},
		c1[2],
		c1[3],
		c1[4],
		c1[5],
		{x: c2[8].x, y: c1[4].y, on: false},
		{x: c2[7].x, y: c1[3].y, on: false},
		c2[6],
		c2[7],
		c2[8],
	}
	encoded := encodeSimpleGlyph([][]ttfPoint{c0, inner}, bbox)
	if len(encoded) > len(raw) {
		return nil
	}

	out := append([]byte(nil), src...)
	slot := out[glyfOff+g0 : glyfOff+g1]
	clear(slot)
	copy(slot, encoded)

	// Recompute glyf table checksum and whole-font head.checkSumAdjustment.
	binary.BigEndian.PutUint32(out[glyfEntry+4:glyfEntry+8], sfntChecksum(out[glyfOff:glyfOff+glyfLen]))
	binary.BigEndian.PutUint32(out[headOff+8:headOff+12], 0)
	binary.BigEndian.PutUint32(out[headOff+8:headOff+12], 0xB1B0AFBA-sfntChecksum(out))
	return out
}

func sfntTable(data []byte, want string) (offset, length uint32, entry int, ok bool) {
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	if 12+numTables*16 > len(data) {
		return 0, 0, 0, false
	}
	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if string(data[off:off+4]) == want {
			toff := binary.BigEndian.Uint32(data[off+8 : off+12])
			tlen := binary.BigEndian.Uint32(data[off+12 : off+16])
			if uint64(toff)+uint64(tlen) > uint64(len(data)) {
				return 0, 0, 0, false
			}
			return toff, tlen, off, true
		}
	}
	return 0, 0, 0, false
}

func cmapGlyphID(cmap []byte, cp uint16) (uint16, bool) {
	if len(cmap) < 4 {
		return 0, false
	}
	numSub := int(binary.BigEndian.Uint16(cmap[2:4]))
	if 4+numSub*8 > len(cmap) {
		return 0, false
	}
	for i := 0; i < numSub; i++ {
		soff := int(binary.BigEndian.Uint32(cmap[8+i*8 : 12+i*8]))
		if soff+14 > len(cmap) {
			continue
		}
		if binary.BigEndian.Uint16(cmap[soff:soff+2]) != 4 {
			continue
		}
		segCountX2 := int(binary.BigEndian.Uint16(cmap[soff+6 : soff+8]))
		segCount := segCountX2 / 2
		endOff := soff + 14
		startOff := endOff + segCountX2 + 2
		deltaOff := startOff + segCountX2
		rangeOff := deltaOff + segCountX2
		if rangeOff+segCountX2 > len(cmap) {
			continue
		}
		for s := 0; s < segCount; s++ {
			ec := binary.BigEndian.Uint16(cmap[endOff+s*2 : endOff+s*2+2])
			sc := binary.BigEndian.Uint16(cmap[startOff+s*2 : startOff+s*2+2])
			if cp < sc || cp > ec {
				continue
			}
			delta := int16(binary.BigEndian.Uint16(cmap[deltaOff+s*2 : deltaOff+s*2+2]))
			ro := int(binary.BigEndian.Uint16(cmap[rangeOff+s*2 : rangeOff+s*2+2]))
			if ro == 0 {
				return uint16(int32(cp)+int32(delta)) & 0xFFFF, true
			}
			idx := rangeOff + s*2 + ro + int(cp-sc)*2
			if idx+2 > len(cmap) {
				return 0, false
			}
			gid := binary.BigEndian.Uint16(cmap[idx : idx+2])
			if gid == 0 {
				return 0, false
			}
			return uint16(int32(gid)+int32(delta)) & 0xFFFF, true
		}
	}
	return 0, false
}

func decodeSimpleGlyph(raw []byte) ([][]ttfPoint, [4]int16, bool) {
	var bbox [4]int16
	if len(raw) < 12 {
		return nil, bbox, false
	}
	nCont := int(int16(binary.BigEndian.Uint16(raw[0:2])))
	if nCont <= 0 {
		return nil, bbox, false
	}
	for i := 0; i < 4; i++ {
		bbox[i] = int16(binary.BigEndian.Uint16(raw[2+i*2 : 4+i*2]))
	}
	pos := 10
	if pos+nCont*2+2 > len(raw) {
		return nil, bbox, false
	}
	endPts := make([]int, nCont)
	for i := 0; i < nCont; i++ {
		endPts[i] = int(binary.BigEndian.Uint16(raw[pos : pos+2]))
		pos += 2
	}
	instLen := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
	pos += 2 + instLen
	if pos > len(raw) {
		return nil, bbox, false
	}
	nPts := endPts[nCont-1] + 1
	flags := make([]byte, 0, nPts)
	for len(flags) < nPts {
		if pos >= len(raw) {
			return nil, bbox, false
		}
		f := raw[pos]
		pos++
		flags = append(flags, f)
		if f&0x08 != 0 {
			if pos >= len(raw) {
				return nil, bbox, false
			}
			rep := int(raw[pos])
			pos++
			for r := 0; r < rep && len(flags) < nPts; r++ {
				flags = append(flags, f)
			}
		}
	}
	xs := make([]int16, nPts)
	var x int16
	for i, f := range flags {
		switch {
		case f&0x02 != 0:
			if pos >= len(raw) {
				return nil, bbox, false
			}
			dx := int16(raw[pos])
			pos++
			if f&0x10 != 0 {
				x += dx
			} else {
				x -= dx
			}
		case f&0x10 == 0:
			if pos+2 > len(raw) {
				return nil, bbox, false
			}
			x += int16(binary.BigEndian.Uint16(raw[pos : pos+2]))
			pos += 2
		}
		xs[i] = x
	}
	ys := make([]int16, nPts)
	var y int16
	for i, f := range flags {
		switch {
		case f&0x04 != 0:
			if pos >= len(raw) {
				return nil, bbox, false
			}
			dy := int16(raw[pos])
			pos++
			if f&0x20 != 0 {
				y += dy
			} else {
				y -= dy
			}
		case f&0x20 == 0:
			if pos+2 > len(raw) {
				return nil, bbox, false
			}
			y += int16(binary.BigEndian.Uint16(raw[pos : pos+2]))
			pos += 2
		}
		ys[i] = y
	}
	contours := make([][]ttfPoint, nCont)
	start := 0
	for c, ep := range endPts {
		if ep < start || ep >= nPts {
			return nil, bbox, false
		}
		pts := make([]ttfPoint, ep-start+1)
		for i := start; i <= ep; i++ {
			pts[i-start] = ttfPoint{x: xs[i], y: ys[i], on: flags[i]&0x01 != 0}
		}
		contours[c] = pts
		start = ep + 1
	}
	return contours, bbox, true
}

func encodeSimpleGlyph(contours [][]ttfPoint, bbox [4]int16) []byte {
	var all []ttfPoint
	endPts := make([]uint16, len(contours))
	for i, c := range contours {
		all = append(all, c...)
		endPts[i] = uint16(len(all) - 1)
	}
	out := make([]byte, 0, 10+len(contours)*2+2+len(all)*5)
	out = binary.BigEndian.AppendUint16(out, uint16(len(contours)))
	for _, b := range bbox {
		out = binary.BigEndian.AppendUint16(out, uint16(b))
	}
	for _, ep := range endPts {
		out = binary.BigEndian.AppendUint16(out, ep)
	}
	out = binary.BigEndian.AppendUint16(out, 0) // instructionLength = 0
	for _, p := range all {
		if p.on {
			out = append(out, 0x01)
		} else {
			out = append(out, 0x00)
		}
	}
	var px int16
	for _, p := range all {
		out = binary.BigEndian.AppendUint16(out, uint16(p.x-px))
		px = p.x
	}
	var py int16
	for _, p := range all {
		out = binary.BigEndian.AppendUint16(out, uint16(p.y-py))
		py = p.y
	}
	return out
}

func sfntChecksum(b []byte) uint32 {
	var sum uint32
	n := len(b) &^ 3
	for i := 0; i < n; i += 4 {
		sum += binary.BigEndian.Uint32(b[i : i+4])
	}
	if rem := b[n:]; len(rem) > 0 {
		var tail [4]byte
		copy(tail[:], rem)
		sum += binary.BigEndian.Uint32(tail[:])
	}
	return sum
}
