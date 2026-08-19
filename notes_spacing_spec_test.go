package bibletext

// THE NATIVES ARE HELD TO THE TABLE.
//
// noteMetrics (notes_bubble.go) is the note sticker's spacing spec, and the
// styled pane simply reads it. The other three surfaces cannot: their layout
// code is Objective-C inside a cgo preamble (reading_ios.go, reading_macos.go)
// and Java (android/BtBridge.java), and neither language can import a Go
// constant. So each carries named constants of its own and THIS FILE PARSES
// THOSE SOURCES and asserts every literal equals the Go table.
//
// WHY PARSING RATHER THAN A PUSH PARAMETER. The alternative was to send the
// numbers over the existing push/ABI (bibleTextIOSSetNote & co), which would
// make the table the single runtime source. It was rejected: these are
// compile-time layout constants, not per-chapter data, so pushing them would put
// a wire format, a three-platform signature change and a version skew in front
// of every future 1pt change, and would still not stop a native from ignoring
// the pushed value in favour of a literal. Parsing costs nothing at runtime,
// fails on the the development environment and in CI on EVERY platform's numbers at once
// (including the two that cannot even be compiled here), and is the mechanism
// this repo already uses to hold untestable sources honest —
// dev_links_guard_test.go over the release scripts, share_link_test.go's slug
// golden, licensed_exclusion_test.go over the generated site.
//
// WHAT IT CANNOT SEE: that a native USES its constant where it should. The
// shape assertions at the bottom cover the specific misuses this unification
// removed, and the placement arithmetic is asserted by name rather than by
// value.

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// nativeNoteSource is one platform's note code and the names it spells the
// spec with.
type nativeNoteSource struct {
	label  string
	path   string
	whoSz  float32 // the who line's font size on this platform
	names  map[string]string
	whoRef string // a literal that must still be present, proving whoSz is real
}

// The three. Android's numbers are dp, the Apple panes' are points; the spec is
// unitless by design (a dp and a point are the same design unit).
var nativeNoteSources = []nativeNoteSource{
	{
		label: "iOS (reading_ios.go)", path: "reading_ios.go", whoSz: 11,
		whoRef: "systemFontOfSize:11 weight:UIFontWeightSemibold",
		names: map[string]string{
			"GapAbove": "kNoteGapAbove", "GapBelow": "kNoteGapBelow",
			"Pad": "kNotePad", "WhoH": "kNoteWho", "WhoGap": "kNoteWhoGap",
			"TailDepth": "kNoteTail", "TailWidth": "kNoteTailW", "TailInset": "kNoteTailX",
			"Radius": "kNoteRad", "PillH": "kNotePill",
		},
	},
	{
		label: "macOS (reading_macos.go)", path: "reading_macos.go", whoSz: 10,
		whoRef: "systemFontOfSize:10 weight:NSFontWeightSemibold",
		names: map[string]string{
			"GapAbove": "kMacNoteGapAbove", "GapBelow": "kMacNoteGapBelow",
			"Pad": "kMacNotePad", "WhoH": "kMacNoteWho", "WhoGap": "kMacNoteWhoGap",
			"TailDepth": "kMacNoteTail", "TailWidth": "kMacNoteTailW", "TailInset": "kMacNoteTailX",
			"Radius": "kMacNoteRad", "PillH": "kMacNotePill",
		},
	},
	{
		label: "Android (android/BtBridge.java)", path: "android/BtBridge.java", whoSz: 11,
		whoRef: "who.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 11f)",
		names: map[string]string{
			"GapAbove": "NOTE_GAP_ABOVE", "GapBelow": "NOTE_GAP_BELOW",
			"Pad": "NOTE_PAD", "WhoH": "NOTE_WHO_H", "WhoGap": "NOTE_WHO_GAP",
			"TailDepth": "NOTE_TAIL", "TailWidth": "NOTE_TAIL_W", "TailInset": "NOTE_TAIL_X",
			"Radius": "NOTE_RADIUS", "PillH": "NOTE_PILL_H",
		},
	},
}

// readNativeSource reads one of the three, failing loudly if it moved — a
// renamed file must not silently turn this suite into a no-op.
//
// CRLF IS NORMALISED AWAY, and it is not cosmetic. Git for Windows checks out
// with autocrlf on by default, so on the windows CI runner every *.go and
// *.java in the tree arrives with \r\n — .gitattributes exempts testdata/ and
// patches/, not source. A required fragment that spans a line break (there is
// one: "+ kNoteGapAbove;\n") then never matches, and this suite goes red on
// windows ALONE, invisibly to every local run on this Mac. That is the third
// time this repo has met this failure — a14b3fc0e fixed it once for the byte
// precise fixtures — so it is fixed here at the read, where it cannot recur for
// the next fragment somebody adds: line-ending STYLE is a property of the
// checkout, never of the source this suite is asserting about.
func readNativeSource(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (the spec's parser cannot find the source it holds)", path, err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// constValue pulls `name = <number>` out of a C or Java declaration. The word
// boundary matters: kNoteWhoGap must not answer for kNoteWho.
func constValue(t *testing.T, src, file, name string) float32 {
	t.Helper()
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=\s*([0-9]+(?:\.[0-9]+)?)`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s: no declaration of %s — the spec constant was renamed or deleted, "+
			"which is exactly the drift this test exists to catch", file, name)
	}
	v, err := strconv.ParseFloat(m[1], 32)
	if err != nil {
		t.Fatalf("%s: %s = %q is not a number", file, name, m[1])
	}
	return float32(v)
}

// TestNativeNoteSpacingMatchesTheSpec is the whole point of the file: every
// number the three native stickers lay out with equals noteMetrics().
func TestNativeNoteSpacingMatchesTheSpec(t *testing.T) {
	for _, ns := range nativeNoteSources {
		t.Run(ns.label, func(t *testing.T) {
			src := readNativeSource(t, ns.path)

			// The who ROW is a rule, not a literal: its box is derived from the
			// platform's own who font size, so the same spec yields 14 at 11pt
			// and 13 at 10pt. Prove the font size the derivation assumes is
			// really the one the source uses before trusting the derived box.
			if !strings.Contains(src, ns.whoRef) {
				t.Fatalf("%s: the who font is no longer %v (%q is gone), so the spec's "+
					"derived who-row height cannot be checked against it",
					ns.path, ns.whoSz, ns.whoRef)
			}

			want := map[string]float32{
				"GapAbove":  noteMetrics().GapAbove,
				"GapBelow":  noteMetrics().GapBelow,
				"Pad":       noteMetrics().Pad,
				"WhoH":      noteMetrics().WhoH(ns.whoSz),
				"WhoGap":    noteMetrics().WhoGap,
				"TailDepth": noteMetrics().TailDepth,
				"TailWidth": noteMetrics().TailWidth,
				"TailInset": noteMetrics().TailInset,
				"Radius":    noteMetrics().Radius,
				"PillH":     noteMetrics().PillH,
			}
			for field, name := range ns.names {
				got := constValue(t, src, ns.path, name)
				if got != want[field] {
					t.Errorf("%s: %s = %v, but noteMetrics().%s is %v — "+
						"the four note surfaces have started to disagree again",
						ns.path, name, got, field, want[field])
				}
			}
		})
	}
}

// TestStyledNoteReadsTheSpecDirectly is the fourth surface's half of the same
// contract. It needs no parser — it consumes the table — so this asserts only
// that it still does, i.e. that nobody has reintroduced a local literal.
func TestStyledNoteReadsTheSpecDirectly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		got, want float32
	}{
		{"gap above", styledNoteGapAbv, noteMetrics().GapAbove},
		{"gap below", styledNoteGapBlw, noteMetrics().GapBelow},
		{"card padding", styledNotePad, noteMetrics().Pad},
		{"who row", styledNoteWhoH, noteMetrics().WhoH(styledNoteWhoSz)},
		{"who gap", styledNoteWhoGap, noteMetrics().WhoGap},
	} {
		if tc.got != tc.want {
			t.Errorf("styled pane %s = %v, want %v", tc.name, tc.got, tc.want)
		}
	}
	// The who row is the derived one; 11pt must still give 14 or the ratio has
	// drifted under the platforms that hardcode the answer.
	if got := noteMetrics().WhoH(11); got != 14 {
		t.Errorf("WhoH(11) = %v, want 14 (iOS/Android/styled carry that literal)", got)
	}
	if got := noteMetrics().WhoH(10); got != 13 {
		t.Errorf("WhoH(10) = %v, want 13 (macOS carries that literal)", got)
	}
}

// TestNoteSpacingShapeInTheNatives catches the misuses the unification removed.
// A constant can hold the right value and still be applied in the wrong place;
// these are the specific wrong places the four surfaces were measured in.
func TestNoteSpacingShapeInTheNatives(t *testing.T) {
	type check struct {
		path string
		// banned: a substring that must NOT appear, and why.
		// required: a substring that MUST appear, and why.
		banned, required map[string]string
	}
	for _, c := range []check{
		{
			path: "reading_ios.go",
			banned: map[string]string{
				"kNotePad - 2, whoW": "the who row's -2 shim: it made the stated 12+14+4 rhythm " +
					"describe a card whose real top padding was 10",
				"kNoteTail - 1": "the tail's -1: a survival of the two-shape era that drew " +
					"the tail a point shorter than the constant naming it",
				"cw, kNoteBtn)": "the pill borrowing the VERB BUTTON's size (30) for its height",
			},
			required: map[string]string{
				"gNoteBandH = kNoteGapAbove + h +": "the band must reserve air ABOVE the card, " +
					"not only below it (this pane had no top term at all, so an iPad's card " +
					"touched the line above)",
				"+ kNoteGapBelow;":   "the band's bottom gap must be the spec's",
				"cw, kNotePill)":     "the pill's height must be the spec's",
				"+ kNoteGapAbove;\n": "the sticker must hang the reserved gap below the band's top",
				// The USE SITE, not only the constants: with the literals left
				// correct, "kNotePad + 20 + 8 + …" is a 10pt-taller card that is
				// silently off-spec, and the parser could not see it (verification

				"return kNotePad + kNoteWho + kNoteWhoGap + ceil(r.size.height) + kNotePad;": "the " +
					"card's height must BE the spec's formula — pad, who row, who gap, message, pad " +
					"— not merely use the spec's constants somewhere",
			},
		},
		{
			path: "reading_macos.go",
			banned: map[string]string{
				"kMacNotePad - 2, whoW": "the who row's -2 shim",
				"kMacNoteTail - 1":      "the tail's -1",
				"cw, kMacNoteBtn)":      "the pill borrowing the VERB BUTTON's size (24) for its height",
			},
			required: map[string]string{
				"const CGFloat floorGap = kMacNoteGapAbove;": "the measured top-gap correction " +
					"must be floored at the SPEC's reservation, not at a private 10",
				"textTop - kMacNoteGapBelow - stickerH": "the pinned invariant: the sticker hangs " +
					"the spec's gap above the passage it points at",
				"cw, kMacNotePill)": "the pill's height must be the spec's",
				"return kMacNotePad + kMacNoteWho + kMacNoteWhoGap + ceil(r.size.height) + kMacNotePad;": "the " +
					"card's height must BE the spec's formula (the iOS twin's reason)",
			},
		},
		{
			path: "android/BtBridge.java",
			banned: map[string]string{
				"dp(12), dp(6), dp(4)": "the card's old asymmetric padding (6 top, 4 right) — a " +
					"different internal rhythm from the other three",
				"blp.rightMargin": "the message's compensating right margin, which existed only " +
					"to patch the 4dp right padding back for the body alone",
				"final int gap = dp(8)": "the locally chosen 8dp band gap",
				"who.setOnClickListener": "the next-tap riding the who TextView, whose box is " +
					"now the spec's 14dp",
				"chip.setHeight(": "a FIXED pill height — clips sp text at raised font scales",
				"who.setHeight(":  "a FIXED who height — same clipping",
				"who.setEllipsize(": "END-truncation on the WHO line, which eats the counts a " +
					"reader must never lose; fitWho gives way on the sender half instead. " +
					"(fitWho's own TextUtils.ellipsize call is the mechanism, not the defect, " +
					"and the pill's short label may still truncate.)",
			},
			required: map[string]string{
				"applyNoteBand(r[0], gapAbove + noteH + gapBelow);": "the band must reserve the " +
					"spec's gap on both sides",
				"+ gapTop + gapAbove;": "the sticker must hang the reserved gap below the band's top",
				"dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD) + dp(NOTE_TAIL)": "the " +
					"card's padding must be the spec's on all four sides, with the tail's depth " +
					"carried in the bottom",
				"chip.setMinHeight(dp(NOTE_PILL_H));": "the pill's height must be the spec's, not " +
					"whatever its label wrapped to",
				"Gravity.TOP | Gravity.END": "the verbs must float over the card, out of its " +
					"vertical flow — in flow, an 18sp glyph set the who row's height",
				"who.setMinHeight(dp(NOTE_WHO_H));": "the who " +
					"row must be a FIXED spec box, not a flow row a glyph can grow",
				"dp(NOTE_PAD + NOTE_WHO_H + 2), Gravity.TOP | Gravity.START": "with the who row " +
					"pinned to 14dp, the next-tap needs its own overlay target (iOS's nxt " +
					"height) — 14dp of glyph is not a touch target",
			},
		},
	} {
		t.Run(c.path, func(t *testing.T) {
			src := readNativeSource(t, c.path)
			for frag, why := range c.banned {
				if strings.Contains(src, frag) {
					t.Errorf("%s still contains %q — %s", c.path, frag, why)
				}
			}
			for frag, why := range c.required {
				if !strings.Contains(src, frag) {
					t.Errorf("%s no longer contains %q — %s", c.path, frag, why)
				}
			}
		})
	}
}

// TestNoteSpecIsSelfConsistent guards the table itself: the numbers a reader of
// [redacted-retired-private-reference] is told to expect.
func TestNoteSpecIsSelfConsistent(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  float32
		want float32
	}{
		{"GapAbove", noteMetrics().GapAbove, 10},
		{"GapBelow", noteMetrics().GapBelow, 10},
		{"Pad", noteMetrics().Pad, 12},
		{"WhoGap", noteMetrics().WhoGap, 4},
		{"TailDepth", noteMetrics().TailDepth, 9},
		{"TailWidth", noteMetrics().TailWidth, 18},
		{"TailInset", noteMetrics().TailInset, 24},
		{"Radius", noteMetrics().Radius, 10},
		{"PillH", noteMetrics().PillH, 28},
	} {
		if tc.got != tc.want {
			t.Errorf("noteMetrics().%s = %v, want %v — if this is a deliberate change, "+
				"[redacted-retired-private-reference]'s per-platform table has to move with it",
				tc.name, tc.got, tc.want)
		}
	}
	// The tail must fit inside a card of the minimum width the panes allow, or
	// noteBubblePathSVG's clamp silently moves it.
	if min := noteMetrics().TailInset + noteMetrics().TailWidth + noteMetrics().Radius; min > 60 {
		t.Errorf("the tail needs %v of card width; the panes refuse to draw below 60", min)
	}
}
