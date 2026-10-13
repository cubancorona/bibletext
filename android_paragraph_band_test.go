package bibletext

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// androidImportedPage models what Html.fromHtml makes of the Android page,
// as androidImportedText does (the importer's newline rules, held to the
// emulator there), and also says where the bridge's verse index puts each
// verse, its number's first character (buildVerseIndex reads the <sup>
// spans), and where each <p> block's first character falls. Offsets are
// bytes of the modelled text; the rules below only compare them.
func androidImportedPage(markup string, compact bool) (text string, nums, starts, blocks []int) {
	margin := 2
	if compact {
		margin = 1
	}
	var b strings.Builder
	appendNewlines := func(min int) {
		s := b.String()
		if s == "" {
			return
		}
		n := 0
		for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
			n++
		}
		for ; n < min; n++ {
			b.WriteByte('\n')
		}
	}
	inSup, supAt, supText := false, 0, ""
	for _, m := range regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)[^>]*>|[^<]+`).FindAllStringSubmatch(markup, -1) {
		switch tag := strings.ToLower(m[2]); {
		case tag == "":
			s := html.UnescapeString(m[0])
			if inSup {
				supText += s
			}
			b.WriteString(s)
		case tag == "p" && m[1] == "":
			appendNewlines(margin)
			blocks = append(blocks, b.Len())
		case tag == "p":
			appendNewlines(margin)
		case tag == "br" && m[1] == "":
			b.WriteByte('\n')
		case tag == "sup" && m[1] == "":
			inSup, supAt, supText = true, b.Len(), ""
		case tag == "sup":
			inSup = false
			if n, err := strconv.Atoi(strings.TrimSpace(supText)); err == nil && n > 0 {
				nums = append(nums, n)
				starts = append(starts, supAt)
			}
		}
	}
	return b.String(), nums, starts, blocks
}

// bridgeParagraphStart is BtBridge's paragraphStartOf with paraStartAt, line
// for line: from the start of the last pushed paragraph opener at or before
// off (off itself when there is none), back to the last '\n'.
func bridgeParagraphStart(text string, nums, starts, openers []int, off int) int {
	from := off
	for i := range nums {
		if starts[i] > off {
			break
		}
		for _, o := range openers {
			if o == nums[i] {
				from = starts[i]
				break
			}
		}
	}
	for from > 0 && text[from-1] != '\n' {
		from--
	}
	return from
}

// paragraphBandChapter is an invented chapter with a stanza of four lines,
// a prose paragraph, and a stanza of two, the shapes a note's band meets.
func paragraphBandChapter() *AppState {
	v := func(n int, para bool, text string) Verse {
		return Verse{BookName: "Psalms", Book: "Psalms", Chapter: 150, Verse: n, ParaStart: para, Text: text}
	}
	bd := &BibleData{Books: []string{"Psalms"}, Verses: map[string]map[int][]Verse{"Psalms": {150: {
		v(1, true, "Sing to the hills,\nsing to the valleys below."),
		v(2, false, "Let the rivers answer,\nlet the morning hear."),
		v(3, false, "The fields are wide,\nthe barns are full."),
		v(4, false, "Sing until the evening,\nand sing through the night."),
		v(5, true, "Then the people went home and slept, for the work of the day was done."),
		v(6, false, "In the morning they rose and went out again."),
		v(7, true, "The hills are bright,\nthe valleys wake."),
		v(8, false, "The rivers run,\nthe cattle low."),
	}}}}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 150}
}

// A note's band, card or pill opens above the page paragraph that carries its
// verse, on the phone page and on the book page alike, and the arrival at
// that band scrolls to the same place. The bridge found the paragraph by
// walking back from the verse to the last '\n', and the page writes each poem
// line as a <br>, which imports as a '\n' too, so for any verse below a
// stanza's first line it found the line: a hole between two lines of the
// poem. Go now pushes the verse that opens each paragraph
// (androidParagraphOpeners), and the bridge walks back from that verse's
// start. The control is the same walk with nothing pushed, which is the
// bridge before: it lands on the paragraph only for the verses that open one.
func TestAndroidBandsHangAboveTheParagraphNotThePoemLine(t *testing.T) {
	orig := reporterLayout
	t.Cleanup(func() { reporterLayout = orig })
	for _, book := range []bool{false, true} {
		reporterLayout = func() bool { return book }
		st := paragraphBandChapter()
		verses := st.Bible.GetChapter("Psalms", 150)
		text, nums, starts, blocks := androidImportedPage(buildChapterHTMLAndroid(st, verses), book)
		if len(nums) != len(verses) || len(blocks) != 3 {
			t.Fatalf("book page %v: the modelled page has %d verse numbers and %d blocks, want %d and 3", book, len(nums), len(blocks), len(verses))
		}
		openers := androidParagraphOpeners(st, verses)
		if want := []int{1, 5, 7}; !equalInts(openers, want) {
			t.Errorf("book page %v: the paragraph openers are %v, want %v", book, openers, want)
		}
		movedBefore := 0
		for i, n := range nums {
			want := 0
			for _, b := range blocks {
				if b <= starts[i] {
					want = b
				}
			}
			if got := bridgeParagraphStart(text, nums, starts, openers, starts[i]); got != want {
				t.Errorf("book page %v: verse %d's band hangs at %d (%q), want the paragraph's start, %d (%q)",
					book, n, got, lineAt(text, got), want, lineAt(text, want))
			}
			if bridgeParagraphStart(text, nums, starts, nil, starts[i]) != want {
				movedBefore++
			}
		}
		// Verses 2, 3, 4 and 8 open a poem line, not a paragraph.
		if movedBefore != 4 {
			t.Errorf("book page %v: with no openers the walk misses the paragraph for %d verses, want 4; the fixture no longer shows the defect", book, movedBefore)
		}
	}
}

func lineAt(text string, off int) string {
	end := strings.IndexByte(text[off:], '\n')
	if end < 0 {
		end = len(text) - off
	}
	return text[off : off+end]
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The bridge reads the paragraph from the pushed openers at every place a
// band is reserved, placed or arrived at: the reservations (applyNoteBand),
// the card's and each pill's placement, the card-beside-pill test and the
// ARRIVE_BAND scroll. The line walk, paraStartAt, is called by
// paragraphStartOf alone; it was called at each of those places, and inline
// at the arrival, which is how every one of them stopped at a poem line.
// Go pushes the openers with the band specs, from the chapter it pushes.
func TestBtBridgeReadsTheParagraphFromThePushedOpeners(t *testing.T) {
	java := readRepoFile(t, "android/BtBridge.java")
	if n := strings.Count(java, "paraStartAt("); n != 2 {
		t.Errorf("paraStartAt( appears %d times in BtBridge.java, want 2 (its definition and paragraphStartOf's call)", n)
	}
	i := strings.Index(java, "private static int paragraphStartOf(")
	if i < 0 {
		t.Fatal("BtBridge.java has no paragraphStartOf")
	}
	body := java[i:]
	body = body[:strings.Index(body, "\n    }\n")]
	for _, want := range []string{"noteParaOpeners", "verseStarts[i]", "return paraStartAt(sp, from);"} {
		if !strings.Contains(body, want) {
			t.Errorf("paragraphStartOf does not read %q:\n%s", want, body)
		}
	}
	if n := strings.Count(java, "paragraphStartOf("); n < 6 {
		t.Errorf("paragraphStartOf( appears %d times, want its definition and the five places a band is reserved, placed or arrived at", n)
	}
	if strings.Contains(java, "cs.charAt(paraOff - 1) != '\\n'") {
		t.Error("the ARRIVE_BAND scroll still walks back from the verse inline")
	}
	if !strings.Contains(java, "noteParaOpeners = o;") {
		t.Error("setNoteBands does not keep the pushed openers")
	}
	goSide := readRepoFile(t, "reading_android.go")
	if !strings.Contains(goSide, "androidParagraphOpeners(state, verses)") {
		t.Error("reading_android.go does not push androidParagraphOpeners with the band specs")
	}
}
