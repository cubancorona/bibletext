package bibletext

import (
	"strconv"
	"strings"
	"testing"
)

// The Android reading position crosses the bridge as a verse: the bridge
// keeps {verse, within, frac} from the last scroll (getScrollAnchor), Go
// saves it and hands it back on the restore (armRestore) and with the
// chapter (setHtml), and the bridge places the verse first, the fraction only
// where the text does not hold the verse. The bridge used to hand Go the
// fraction alone, and captureReadingAnchor returned verse 0, so every saved
// and restored Android position was a fraction of the scroll range.
func TestAndroidKeepsTheReadingPositionByVerse(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	for _, want := range []string{
		"public static float[] getScrollAnchor()",
		"public static void armRestore(final int verse, final float within, final float frac)",
		"final int restoreVerse, final float restoreWithin)",
		"lastAnchor = new float[]{a != null ? a[0] : 0f, a != null ? a[1] : 0f, lastFrac};",
		"pendingRestoreVerse = restoreVerse > 0 ? restoreVerse : 0;",
	} {
		if !strings.Contains(java, want) {
			t.Errorf("BtBridge.java lacks %q", want)
		}
	}
	if strings.Contains(java, "getScrollFrac") {
		t.Error("BtBridge.java still hands Go the scroll fraction alone (getScrollFrac)")
	}
	applyStart := strings.Index(java, "private static void applyPendingScroll()")
	applyEnd := strings.Index(java[applyStart:], "/** armRestore")
	if applyStart < 0 || applyEnd < 0 {
		t.Fatal("applyPendingScroll has no stable boundary")
	}
	apply := java[applyStart : applyStart+applyEnd]
	verse := strings.Index(apply, "anchorY(pendingRestoreVerse, pendingRestoreWithin)")
	frac := strings.Index(apply, "ownScrollTo(Math.round(pendingFrac * scrollRange()));")
	if verse < 0 || frac < 0 || verse > frac {
		t.Error("the restore does not place the verse before it falls back to the fraction")
	}

	goSrc := readNativeSource(t, "reading_android.go")
	for _, want := range []string{
		`"getScrollAnchor", "()[F"`,
		`"armRestore", "(IFF)V"`,
		`"setHtml", "(Ljava/lang/String;FIIF)V"`,
		"return int(a[0]), float64(a[1]), float64(a[2]), true",
		"C.btaArmRestore(C.uintptr_t(env), C.int(verse), C.float(delta), C.float(f))",
		"restoreVerse, restoreWithin = state.restore.Verse, float32(state.restore.Delta)",
	} {
		if !strings.Contains(goSrc, want) {
			t.Errorf("reading_android.go lacks %q", want)
		}
	}
}

// wrapPage is a model of a laid-out chapter: lines of equal pitch, each
// starting at a character offset, greedily wrapped at width characters, and
// a viewport of height pixels. It is the shape BtBridge's layout reads have
// (getLineForOffset, getLineTop, getLineForVertical), enough to compare two
// ways of keeping a position across a change of width or text size.
type wrapPage struct {
	lineStarts           []int
	pitch, viewport      int
	nums, starts, ends   []int
	textLen              int
	paraBreakBeforeVerse map[int]bool
}

func layWrapPage(verses []string, paraBefore map[int]bool, width, pitch, viewport int) wrapPage {
	p := wrapPage{pitch: pitch, viewport: viewport, paraBreakBeforeVerse: paraBefore}
	off, lineLen := 0, 0
	p.lineStarts = []int{0}
	newLine := func() {
		p.lineStarts = append(p.lineStarts, off)
		lineLen = 0
	}
	for i, body := range verses {
		n := i + 1
		if paraBefore[n] && lineLen > 0 {
			newLine()
		}
		words := append([]string{strconv.Itoa(n)}, strings.Fields(body)...)
		for k, w := range words {
			if lineLen > 0 && lineLen+1+len(w) > width {
				newLine()
			}
			if lineLen > 0 {
				off++
				lineLen++
			}
			if k == 0 {
				p.nums = append(p.nums, n)
				p.starts = append(p.starts, off)
			}
			off += len(w)
			lineLen += len(w)
		}
	}
	p.textLen = off
	for i := range p.starts {
		if i+1 < len(p.starts) {
			p.ends = append(p.ends, p.starts[i+1])
		} else {
			p.ends = append(p.ends, off)
		}
	}
	return p
}

func (p wrapPage) lineForOffset(off int) int {
	l := 0
	for i, s := range p.lineStarts {
		if s <= off {
			l = i
		}
	}
	return l
}
func (p wrapPage) lineTop(l int) int    { return l * p.pitch }
func (p wrapPage) lineBottom(l int) int { return (l + 1) * p.pitch }
func (p wrapPage) lineForVertical(y int) int {
	l := y / p.pitch
	if l >= len(p.lineStarts) {
		l = len(p.lineStarts) - 1
	}
	return l
}
func (p wrapPage) scrollRange() int {
	return max(0, len(p.lineStarts)*p.pitch-p.viewport)
}

// verseBand, anchorAt and anchorY are BtBridge's, line for line, over the
// model (no padding: the text's top is the scroll content's top).
func (p wrapPage) verseBand(i int) (int, int) {
	top := p.lineTop(p.lineForOffset(p.starts[i]))
	bottom := -1
	if i+1 < len(p.nums) {
		bottom = p.lineTop(p.lineForOffset(p.starts[i+1]))
	}
	if bottom <= top {
		bottom = p.lineBottom(p.lineForOffset(max(p.starts[i], p.ends[i]-1)))
	}
	if bottom <= top {
		bottom = top + 1
	}
	return top, bottom
}

func (p wrapPage) anchorAt(scrollY int) (verse int, within float64, ok bool) {
	if scrollY <= 0 {
		return 0, 0, false
	}
	off := p.lineStarts[p.lineForVertical(scrollY)]
	idx := -1
	for i := range p.nums {
		if p.starts[i] > off {
			break
		}
		idx = i
	}
	if idx < 0 {
		return 0, 0, false
	}
	top, bottom := p.verseBand(idx)
	w := float64(scrollY-top) / float64(bottom-top)
	return p.nums[idx], min(max(w, 0), 1), true
}

func (p wrapPage) anchorY(verse int, within float64) int {
	for i := range p.nums {
		if p.nums[i] != verse {
			continue
		}
		top, bottom := p.verseBand(i)
		y := top + int(within*float64(bottom-top)+0.5)
		return min(max(y, 0), p.scrollRange())
	}
	return -1
}

// versesOnTopLine is every verse with a word on the line at the top of the
// view when it stands at scrollY: what the reader sees there.
func (p wrapPage) versesOnTopLine(scrollY int) map[int]bool {
	l := p.lineForVertical(scrollY)
	from, to := p.lineStarts[l], p.textLen
	if l+1 < len(p.lineStarts) {
		to = p.lineStarts[l+1]
	}
	out := map[int]bool{}
	for i := range p.nums {
		if p.starts[i] < to && p.ends[i] > from {
			out[p.nums[i]] = true
		}
	}
	return out
}

func sharesAVerse(a, b map[int]bool) bool {
	for v := range a {
		if b[v] {
			return true
		}
	}
	return false
}

// The bridge's verse anchor keeps the verse at the top of the view across a
// rotation and a text-size change, at every scroll position of an invented
// chapter, wherever the restore is not held at the end of the scroll range;
// the scroll fraction, which the bridge restored by before, moves it. The
// model's geometries are a phone's: 34 characters to a line in portrait on a
// 1900-pixel view, 55 in landscape on a 990-pixel one, 27 at a larger text
// size with a taller line.
func TestAVerseAnchorKeepsTheTopVerseWhereAFractionMovesIt(t *testing.T) {
	// The model's verseBand, anchorAt and anchorY are the bridge's: the
	// lines that carry their arithmetic are held to BtBridge.java.
	java := readNativeSource(t, "android/BtBridge.java")
	for _, line := range []string{
		"int bottom = i + 1 < verseNums.length ? lay.getLineTop(lay.getLineForOffset(verseStarts[i + 1])) : -1;",
		"int last = Math.max(verseStarts[i], verseEnds[i] - 1);",
		"int off = lay.getLineStart(lay.getLineForVertical(y));",
		"return new float[]{verseNums[idx], clamp01((float) (y - b[0]) / (b[1] - b[0]))};",
		"int y = text.getTop() + text.getTotalPaddingTop() + b[0] + Math.round(clamp01(within) * (b[1] - b[0]));",
		"return Math.max(0, Math.min(y, scrollRange()));",
	} {
		if !strings.Contains(java, line) {
			t.Errorf("BtBridge.java no longer carries the arithmetic this model follows: %q", line)
		}
	}
	var verses []string
	lengths := []int{9, 23, 14, 31, 6, 18, 27, 11, 40, 8, 16, 22, 35, 12, 19, 7, 26, 15, 30, 10}
	words := strings.Fields("the people went out at first light to the fields and the hills " +
		"and they worked until the evening came and the lamps were lit in every house")
	for i, n := range lengths {
		var b []string
		for k := 0; k < n; k++ {
			b = append(b, words[(i*7+k)%len(words)])
		}
		verses = append(verses, strings.Join(b, " ")+".")
	}
	paras := map[int]bool{1: true, 6: true, 12: true, 17: true}
	portrait := layWrapPage(verses, paras, 34, 60, 1900)
	landscape := layWrapPage(verses, paras, 55, 60, 990)
	larger := layWrapPage(verses, paras, 27, 75, 1900)
	for _, c := range []struct {
		name     string
		from, to wrapPage
	}{
		{"portrait to landscape", portrait, landscape},
		{"landscape to portrait", landscape, portrait},
		{"to a larger text size", portrait, larger},
		{"to a smaller text size", larger, portrait},
	} {
		moved, movedByFrac, positions := 0, 0, 0
		for y := 1; y <= c.from.scrollRange(); y += c.from.pitch / 3 {
			v, within, ok := c.from.anchorAt(y)
			if !ok {
				continue
			}
			positions++
			before := c.from.versesOnTopLine(y)
			if yB := c.to.anchorY(v, within); yB >= 0 && yB < c.to.scrollRange() &&
				!sharesAVerse(before, c.to.versesOnTopLine(yB)) {
				moved++
				if moved <= 3 {
					t.Errorf("%s: verse %d at %.2f of its height, top line %v, restores to a top line of %v",
						c.name, v, within, before, c.to.versesOnTopLine(yB))
				}
			}
			yF := int(float64(y)/float64(c.from.scrollRange())*float64(c.to.scrollRange()) + 0.5)
			if yF < c.to.scrollRange() && !sharesAVerse(before, c.to.versesOnTopLine(yF)) {
				movedByFrac++
			}
		}
		if moved > 0 {
			t.Errorf("%s: the verse anchor moved the top verse at %d of %d positions", c.name, moved, positions)
		}
		if movedByFrac == 0 {
			t.Errorf("%s: the scroll fraction moved the top verse at none of %d positions; the model no longer shows what the anchor fixes", c.name, positions)
		}
		t.Logf("%s: %d positions; the fraction moved the top verse at %d, the verse anchor at %d", c.name, positions, movedByFrac, moved)
	}
}
