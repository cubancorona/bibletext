package bibletext

import (
	"strings"
	"testing"
	"unicode"
)

// collapseSpaces returns text that is already collapsed as it came, without
// splitting it into words and joining them again (isCollapsed). The shortcut
// must agree with the split-and-join on every input: it may fire only where
// that would return the input itself, and it must recognise every character
// strings.Fields treats as whitespace, including the no-break space and the
// wide spaces the reading surfaces write.
func TestCollapseSpacesShortcutAgreesWithFields(t *testing.T) {
	inputs := []string{
		"", " ", "  ", "a", "a b", " a", "a ", "a  b", "a b ", " a b",
		"a\tb", "a\nb", "a\r\nb", "a \nb", "a b", "16 For",
		"  In the beginning", "“Truly,” he said — ‘yes’",
		"The Lᴏʀᴅ is my shepherd", "a\xffb", "\xff", "a \xff b", "a\xff  b",
	}
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			inputs = append(inputs, "a"+string(r)+"b", string(r)+"a", "a"+string(r), "a "+string(r)+" b")
		}
	}
	for _, s := range inputs {
		want := strings.Join(strings.Fields(s), " ")
		if got := collapseSpaces(s); got != want {
			t.Errorf("collapseSpaces(%q) = %q, want %q", s, got, want)
		}
		if got := isCollapsed(s); got != (want == s) {
			t.Errorf("isCollapsed(%q) = %v, but collapsing it gives %q", s, got, want)
		}
	}
}
