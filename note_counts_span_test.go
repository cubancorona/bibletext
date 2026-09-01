package bibletext

import (
	"fmt"
	"strings"
	"testing"
)

// Every surface now finds the counts control by SEARCHING the who line for a
// substring it was handed. That is only safe while the chrome's grammar cannot
// appear in the half a sender controls: a name carrying " · " would put a second
// separator in the line, and the span cut at the first one would then be the
// sender's own words — accented, and wired to the next-tap.
//
// sanitizeSenderName maps U+00B7 to '-', so it cannot. This states that as a
// property of the composer rather than as a comment beside four parsers.
func TestCountsSpanCannotBeForgedByASender(t *testing.T) {
	forged := []string{
		"Amy · 9 of 9 in this chapter",
		"·",
		" · ",
		"A·B",
		"Amy · 2 of 2 in this chapter ›",
		"· 1 of 1 in this chapter",
	}
	for _, raw := range forged {
		clean := sanitizeSenderName(raw)
		if strings.Contains(clean, noteWhoSep) {
			t.Errorf("sanitizeSenderName(%q) = %q still carries the chrome's "+
				"separator, so a name can cut the counts span", raw, clean)
		}
		if strings.ContainsRune(clean, '·') {
			t.Errorf("sanitizeSenderName(%q) = %q still carries U+00B7", raw, clean)
		}
	}

	// And the control: a name the sanitizer leaves alone must still be able to
	// contain the WORDS, because it is only the separator that is reserved.
	if got := sanitizeSenderName("1 of 2 in this chapter"); got != "1 of 2 in this chapter" {
		t.Errorf("the sanitizer is eating ordinary words: %q", got)
	}
}

// The span cut, at its edges, stated once instead of transcribed into three
// languages. Each case is a line the composer can actually produce.
func TestNoteCountsSpan(t *testing.T) {
	const chev = noteChevron
	for _, tc := range []struct {
		name, who string
		next      bool
		want      string
	}{
		{"not a control", "Note from Friend", false, ""},
		{"control, no separator at all", "Note from Friend", true, ""},
		{"counts alone", "Note from Friend · 1 of 2 in this chapter" + chev, true,
			"1 of 2 in this chapter" + chev},
		{"counts then unplaced", "Amy · 2 of 105 in this chapter" + chev + " · 9 not shown here", true,
			"2 of 105 in this chapter" + chev},
		{"an ellipsised sender still cuts right", "Ann… · 1 of 3 in this chapter" + chev, true,
			"1 of 3 in this chapter" + chev},
		{"a bare ellipsis sender — the fit's floor", "… · 1 of 3 in this chapter" + chev, true,
			"1 of 3 in this chapter" + chev},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := noteCountsSpan(tc.who, tc.next)
			if got != tc.want {
				t.Errorf("noteCountsSpan(%q, %v) = %q, want %q", tc.who, tc.next, got, tc.want)
			}
			if got != "" && strings.Count(tc.who, got) != 1 {
				t.Errorf("the span %q is not unique in %q — a backwards search "+
					"would accent the wrong one", got, tc.who)
			}
		})
	}
}

// The unplaced tail is NOT the control, and the difference only shows when the
// counts are absent. Cutting at the first separator without asking `next` would
// accent "9 not shown here" and wire it to a tap that advances nothing.
func TestTheUnplacedTailIsNeverTheControl(t *testing.T) {
	who := fmt.Sprintf("Note from Friend · %d not shown here", 9)
	if got := noteCountsSpan(who, false); got != "" {
		t.Errorf("noteCountsSpan(%q, false) = %q; the unplaced tail is prose, "+
			"not a control, and accenting it promises a tap that does nothing", who, got)
	}
}

// The who line's separator is the chrome's own grammar, and it used to be
// parsed on four surfaces for two different questions: where the sender half
// ends (the FIT, which is genuinely native because it depends on measured
// width) and where the counts span is (the CUT, which is not — it needs no
// measurement at all and is now composed in Go).
//
// The cut is gone from every renderer. The fit remains, and needs the separator
// exactly once. So the invariant that can actually be checked is a COUNT: one
// occurrence is the fit; a second is the cut growing back.
func TestNoRendererParsesTheSeparatorTwice(t *testing.T) {
	for _, tc := range []struct {
		path, literal string
		want          int
		why           string
	}{
		{"reading_ios.go", `@"` + noteWhoSep + `"`, 1, "btIOSFitWho, which needs to know where the sender half ends"},
		{"reading_macos.go", `@"` + noteWhoSep + `"`, 1, "btMacFitWho, the same question"},
		// Java spells the middle dot as an escape, which is the same literal.
		{"android/BtBridge.java", `" \u00b7 "`, 1, "fitWho, the same question"},
		// The styled pane's fit is the SHARED function now (noteFitWho,
		// notes_bubble.go) — the one place the question is answered in Go, and
		// the place the enumeration holds to it (N16-fit-lost-the-counts,
		// mutation-verified). The pane spells no separator of its own.
		{"notes_bubble.go", `"` + noteWhoSep + `"`, 1, "noteFitWho, the ONE Go answer the styled pane consumes"},
		{"reading_styled_note.go", `"` + noteWhoSep + `"`, 0, "delegation: a separator here would be the cut growing back beside the shared fit"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			// Count CODE, not prose: these files explain the grammar in their
			// comments, and a comment quoting the separator is not a parser.
			got := strings.Count(stripLineComments(readNativeSource(t, tc.path)), tc.literal)
			if got == tc.want {
				return
			}
			if got > tc.want {
				t.Errorf("%s spells %s %d times, want %d. One is %s; a second is the "+
					"counts cut growing back — that span is composed once, in Go "+
					"(noteCountsSpan), and every surface only searches for it.",
					tc.path, tc.literal, got, tc.want, tc.why)
				return
			}
			t.Errorf("%s no longer spells %s at all, so its who-line fit cannot be "+
				"keeping the counts whole — a reader loses the count to an ellipsis "+
				"before the byline gives way.", tc.path, tc.literal)
		})
	}
}

// stripLineComments removes // line comments and /* */ blocks, crudely but
// adequately for counting literals in source that is mostly comment.
func stripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue
		}
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// The web reader's arrival, held to the same question as the other four. It is
// JavaScript inside a Go raw string, so this reads the source — the same shape
// every other cross-surface contract here uses.
func TestTheWebReaderAsksWhichParagraph(t *testing.T) {
	src := readNativeSource(t, "cmd/websitegen/assets.go")
	i := strings.Index(src, "function rescrollToHighlight()")
	if i < 0 {
		t.Fatal("rescrollToHighlight is gone; this contract guards nothing")
	}
	body := src[i:]
	if j := strings.Index(body, "\n  }"); j > 0 {
		body = body[:j]
	}
	if !strings.Contains(body, "card.nextElementSibling === para") {
		t.Error("the web reader does not ask whether the card belongs to the " +
			"paragraph being arrived at. Its only gate was a null-check on an " +
			"element assigned two statements earlier, so the note always won — " +
			"right on a fresh arrival, and wrong on a hashchange to another " +
			"passage, where it scrolled the reader back to the note they left.")
	}
	if strings.Contains(body, "if (noteBox || noteChip) {") {
		t.Error("the unconditional note-wins gate is back in rescrollToHighlight")
	}
	// The control: with both checks phrased as absences, a deleted function
	// would pass. The verse fallback must still be there.
	if !strings.Contains(body, "block: 'center'") {
		t.Error("the verse fallback is gone, so the assertions above may be " +
			"describing an empty function")
	}
}
