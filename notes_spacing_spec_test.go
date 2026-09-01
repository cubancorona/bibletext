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
// fails in both the default host test suite and CI for every platform's numbers
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
			"PillPadX": "kNotePillPadX", "PillMinW": "kNotePillMinW",
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
			"PillPadX": "kMacNotePillPadX", "PillMinW": "kMacNotePillMinW",
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
			"PillPadX": "NOTE_PILL_PAD_X", "PillMinW": "NOTE_PILL_MIN_W",
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
// windows ALONE, invisibly to every local run on macOS. That is the third
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
				"PillPadX":  noteMetrics().PillPadX,
				"PillMinW":  noteMetrics().PillMinW,
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
				"ps.paragraphSpacingBefore = gNoteBandH": "the assignment form of the " +
					"reservation — see the required += twin",
				"gNoteMinimized ? 0 : kNoteTail": "the shape asking \"is it collapsed\" when it " +
					"means \"does it point at a passage\" — two different questions that agreed " +
					"until a note could be parked at the chapter top",
			},
			required: map[string]string{
				"btIOSTrashImage(kNoteTrashPt": "the closing control's bin must be DRAWN " +
					"(SF Symbols, template-tinted) rather than typed as an emoji, which renders " +
					"at the button's font size in its own colours — a loud mark on a quiet card",
				"if (gNoteVerbs == kNoteVerbsOwn) {": "the mark must still say what the press " +
					"does: a bin deletes someone else's message, ✕ only puts your own away — and " +
					"it must read the PUSHED verb set, not an own flag this pane re-reads",
				"M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12z": "the card's bin must be the " +
					"SAME drawing as theme.DeleteIcon() — this is Fyne's own path, quoted so the " +
					"two cannot drift. SF Symbols' trash is a different, tapered bin, and using " +
					"it here put two designs on one screen",
				"chip.titleLabel.font = btNoteWhoFont();": "the pill's label must be the WHO font; " +
					"a pill measured at one size and drawn at another overflows its own box",
				"[chip setTitleColor:btNoteColor(gNoteMuted)": "the pill's label is the app's chrome " +
					"and is muted, like every who line on every surface",
				"tw + 2 * kNotePillPadX": "the pill's side padding must be the spec's, not a bare 28",
				"cw < kNotePillMinW":     "the pill's width floor must be the spec's, not a bare 86",
				"gNoteBandH = kNoteGapAbove + h +": "the band must reserve air ABOVE the card, " +
					"not only below it (this pane had no top term at all, so an iPad's card " +
					"touched the line above)",
				"+ kNoteGapBelow;":   "the band's bottom gap must be the spec's",
				"cw, kNotePill)":     "the pill's height must be the spec's",
				"+ kNoteGapAbove;\n": "the sticker must hang the reserved gap below the band's top",
				// The USE SITE, not only the constants: with the literals left
				// correct, "kNotePad + 20 + 8 + …" is a 10pt-taller card that is
				// silently off-spec, and the parser could not see it unless the
				// complete card-height formula is checked.
				"return kNotePad + kNoteWho + kNoteWhoGap + ceil(r.size.height) + kNotePad;": "the " +
					"card's height must BE the spec's formula — pad, who row, who gap, message, pad " +
					"— not merely use the spec's constants somewhere",
				"[fitted rangeOfString:gNoteCounts options:NSBackwardsSearch]": "the counts " +
					"control must be FOUND in the line the Go side composed, not cut out of it " +
					"here. Backwards, because the fit above may have ellipsised the sender half",
				"CGFloat y = btIOSStickerBandY(g.location);": "the placement must take the band " +
					"top from THE ONE ANSWER (btIOSBandTopY); it and the scroll target held " +
					"two hand-maintained copies of the formula, with a comment asking them " +
					"to stay identical",
				"return btIOSStickerBandY(g.location);": "the scroll target's half of the same rule",
				"ps.paragraphSpacingBefore += gNoteBandH;": "the reservation must ADD to the " +
					"paragraph's spacing, never assign over it — assignment is the " +
					"single-tenant assumption in write form, and it silently swallows a " +
					"co-tenant's band",
				"CGFloat left = ps.paragraphSpacingBefore - mine;": "the take-back must " +
					"SUBTRACT what this band contributed, not zero the total, for the same " +
					"co-tenancy reason — with one tenant the two are identical, which is why " +
					"this landed before any second band exists",
				"btIOSClearReservedBands(gReadingTV.textStorage);": "the take-back must be a SWEEP over the " +
					"reservation list, never a single tracked field: a second reservation " +
					"through a scalar handle orphans the first with no reference left to " +
					"take it back by — Android's one-span field is the cautionary case",
				"gNoteBands[gNoteBandCount++]": "every reservation must be RECORDED in " +
					"the list the sweep walks; an unrecorded band is unreachable by the " +
					"take-back and survives as a phantom gap",
				"gNoteShapeExtra = gNoteTail ? kNoteTail : 0;": "the tail's contribution to the " +
					"card's shape must be resolved ONCE, in SetNote, so every band formula reads " +
					"one scalar and none of them can branch differently",
				"if (gNoteTail) {": "the outline must gate the tail detour — a card that points " +
					"at nothing must not draw a point",
			},
		},
		{
			path: "reading_macos.go",
			banned: map[string]string{
				"kMacNotePad - 2, whoW":                "the who row's -2 shim",
				"kMacNoteTail - 1":                     "the tail's -1",
				"cw, kMacNoteBtn)":                     "the pill borrowing the VERB BUTTON's size (24) for its height",
				"gMacNoteMinimized ? 0 : kMacNoteTail": "the iOS twin's wrong question",
				"ps.paragraphSpacingBefore = gMacNoteBandH": "the assignment form of the " +
					"reservation — see the required += twin",
			},
			required: map[string]string{
				"btMacTrashImage(kMacNoteTrashPt)": "the closing control's bin must be DRAWN, " +
					"and drawn from FYNE's path so the app has one bin rather than two designs",
				"if (gMacNoteVerbs == kMacNoteVerbsOwn) {":              "the iOS twin's reason",
				"chip.font = btMacNoteWhoFont();":                       "the pill's label must be the WHO font",
				"chip.contentTintColor = btMacNoteColor(gMacNoteMuted)": "the pill's label is muted chrome",
				"tw + 2 * kMacNotePillPadX": "the pill's side padding must be the spec's; this pane had " +
					"its own 24, which made the same label narrower here than on the phone",
				"cw < kMacNotePillMinW": "the pill's width floor must be the spec's; this pane had its own 76",
				"const CGFloat floorGap = kMacNoteGapAbove;": "the measured top-gap correction " +
					"must be floored at the SPEC's reservation, not at a private 10",
				"textTop - kMacNoteGapBelow - stickerH": "the pinned invariant: the sticker hangs " +
					"the spec's gap above the passage it points at",
				"cw, kMacNotePill)": "the pill's height must be the spec's",
				"return kMacNotePad + kMacNoteWho + kMacNoteWhoGap + ceil(r.size.height) + kMacNotePad;": "the " +
					"card's height must BE the spec's formula (the iOS twin's reason)",
				"[fitted rangeOfString:gMacNoteCounts options:NSBackwardsSearch]": "the iOS " +
					"twin's reason: found, not cut",
				"ps.paragraphSpacingBefore += gMacNoteBandH;": "the iOS twin's rule: the " +
					"reservation ADDS, never assigns",
				"btMacClearReservedBands(gTextView.textStorage);": "the take-back must be " +
					"a SWEEP over the reservation list (the iOS twin's rule and reasons)",
				"gMacNoteBands[gMacNoteBandCount++]": "every reservation must be RECORDED " +
					"in the list the sweep walks",
				"CGFloat left = ps.paragraphSpacingBefore - mine;": "and the take-back " +
					"SUBTRACTS this band's own contribution",
				"gMacNoteShapeExtra = gMacNoteTail ? kMacNoteTail : 0;": "the iOS twin's reason: " +
					"resolved once, read as a scalar everywhere",
				"if (gMacNoteTail) {": "the outline must gate the tail detour",
			},
		},
		{
			path: "android/BtBridge.java",
			banned: map[string]string{
				"who.setTextColor(noteNextable ? noteAccent : noteMuted)": "the WHOLE who line " +
					"painted accent because it was pressable somewhere. Only the counts span is " +
					"the control, and only it wears the accent now",
				"dp(12), dp(6), dp(4)": "the card's old asymmetric padding (6 top, 4 right) — a " +
					"different internal rhythm from the other three",
				"blp.rightMargin": "the message's compensating right margin, which existed only " +
					"to patch the 4dp right padding back for the body alone",
				"final int gap = dp(8)": "the locally chosen 8dp band gap",
				"who.setOnClickListener": "the next-tap riding the who TextView, whose box is " +
					"now the spec's 14dp",
				"chip.setHeight(": "a FIXED pill height — clips sp text at raised font scales",
				"who.setHeight(":  "a FIXED who height — same clipping",
				"\\uD83D\\uDDD1": "the bin as an EMOJI. It is drawn now (noteTrashDrawable), " +
					"from the same path every other surface uses",
				"who.setEllipsize(": "END-truncation on the WHO line, which eats the counts a " +
					"reader must never lose; fitWho gives way on the sender half instead. " +
					"(fitWho's own TextUtils.ellipsize call is the mechanism, not the defect, " +
					"and the pill's short label may still truncate.)",
			},
			required: map[string]string{
				"noteVerbs == VERBS_OWN": "the closing control's mark must say what the press " +
					"does, off the PUSHED verb set",
				"path.cubicTo(6 * u, 20.1f * u, 6.9f * u, 21 * u, 8 * u, 21 * u)": "the bin must be " +
					"DRAWN from Fyne's own delete path, as the Apple panes draw it. The emoji this " +
					"replaced rendered at the button's font size in the system's own colours — a " +
					"loud mark on a quiet card, and a second bin design beside the history bar's",
				"int i = line.lastIndexOf(noteCounts);": "the counts control must be FOUND " +
					"in the composed line. This pane had no split at all and painted the WHOLE " +
					"who line accent when it was nextable, so the sender's byline wore the " +
					"app's \"you can press this\" colour",
				"tv.setText(noteWhoSpanned(fitted))": "the fit produces a NEW string, and " +
					"setText keeps only the spans it is given — a plain setText here drops the " +
					"accent exactly when the line is too wide, which is when it matters",
				"noteVerbSlots() * dp(NOTE_BTN)": "the verb corner's width must come from the " +
					"verb SET. Reserved at a flat two slots, an own note's who line gave way a " +
					"whole button sooner here than on the Apple panes",
				"chip.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, 11f)": "the pill's label " +
					"must be the WHO size in SP, so it scales with the reader's own text setting",
				"chip.setTypeface(android.graphics.Typeface.DEFAULT_BOLD)": "the pill's label is " +
					"semibold on the Apple panes; DEFAULT_BOLD is this platform's nearest",
				"chip.setTextColor(noteMuted)": "the pill's label is muted chrome",
				"chip.setPadding(dp(NOTE_PILL_PAD_X), 0, dp(NOTE_PILL_PAD_X), 0)": "the pill's side " +
					"padding must be the spec's, not the card's NOTE_PAD",
				"chip.setMinWidth(dp(NOTE_PILL_MIN_W))": "the pill needs the spec's width floor; without " +
					"it a short label made a visibly smaller pill here than on iOS",
				"chip.setGravity(Gravity.CENTER)": "with a width floor the box is wider than a short " +
					"label, and iOS centres its title in the same box — left-aligned reads as a mistake",
				"sticker != null ? gapAbove + noteH + gapBelow : 0": "the band must reserve the " +
					"spec's gap on both sides",
				"+ gapTop + pillPart + gapAbove;": "the sticker must hang the reserved gap " +
					"below the band's top, past any pill share stacked above it",
				"dp(NOTE_PAD), dp(NOTE_PAD), dp(NOTE_PAD),\n                dp(NOTE_PAD) + (noteTail ? dp(NOTE_TAIL) : 0)": "the " +
					"card's padding must be the spec's on all four sides, with the tail's depth " +
					"carried in the bottom — and ONLY when there is a tail to carry",
				"noteTail ? dp(NOTE_TAIL) : 0, dp(NOTE_TAIL_W)": "the bubble's tail depth must " +
					"follow the PUSHED decision; drawn unconditionally, a note parked at chapter " +
					"scope grows a tail that points at verse 1",
				"noteTail = tail;": "the pushed tail must actually be stored, or the field keeps " +
					"its initial value and the gate above is decorative",
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

// TestNoteSpecIsSelfConsistent guards the table itself: the numbers recorded in
// docs/NOTES_SPEC.md#sticker-spacing.
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
		{"PillPadX", noteMetrics().PillPadX, 14},
		{"PillMinW", noteMetrics().PillMinW, 86},
	} {
		if tc.got != tc.want {
			t.Errorf("noteMetrics().%s = %v, want %v — if this is a deliberate change, "+
				"docs/NOTES_SPEC.md#sticker-spacing has to move with it",
				tc.name, tc.got, tc.want)
		}
	}
	// The tail must fit inside a card of the minimum width the panes allow, or
	// noteBubblePathSVG's clamp silently moves it.
	if min := noteMetrics().TailInset + noteMetrics().TailWidth + noteMetrics().Radius; min > 60 {
		t.Errorf("the tail needs %v of card width; the panes refuse to draw below 60", min)
	}
}

// EVERY RESERVATION APPLY HAS A RECORD — counted, because the two record sites
// share one substring and a Contains-style fragment cannot see one of them
// vanish (a mutation proved it: dropping the paragraph-style record left the
// inset record satisfying the fragment).
//
// An apply is either the paragraph-style add or the inset assignment; each must
// push an entry the sweep can find, or the band it reserved is unreachable by
// the take-back and survives as a phantom gap.
func TestEveryIOSReservationIsRecorded(t *testing.T) {
	for _, tc := range []struct {
		path    string
		applies []string
		record  string
	}{
		{"reading_ios.go", []string{
			"ps.paragraphSpacingBefore += gNoteBandH;",
			"gNoteTopInset = gNoteBandH;",
			"ps.paragraphSpacingBefore += bandH;",
			"gNoteTopInset += bandH;",
		}, "gNoteBands[gNoteBandCount++]"},
		{"reading_macos.go", []string{
			"ps.paragraphSpacingBefore += gMacNoteBandH;",
			"gMacNoteTopInset = gMacNoteBandH;",
			"ps.paragraphSpacingBefore += bandH;",
			"gMacNoteTopInset += bandH;",
		}, "gMacNoteBands[gMacNoteBandCount++]"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			src := readNativeSource(t, tc.path)
			applies := 0
			for _, a := range tc.applies {
				applies += strings.Count(src, a)
			}
			records := strings.Count(src, tc.record)
			if applies < 2 {
				t.Fatalf("only %d reservation applies found — the spellings have "+
					"moved and this count is checking nothing", applies)
			}
			if records != applies {
				t.Errorf("%d reservation applies but %d records: an unrecorded band "+
					"cannot be taken back by the sweep", applies, records)
			}
		})
	}
}
