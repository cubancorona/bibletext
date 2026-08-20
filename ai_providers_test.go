package bibletext

import (
	"regexp"
	"strings"
	"testing"
)

// THE ASSISTANT LIST IS ONE FORM: "Product (Vendor)".
//
// The picker had "Google Gemini" beside "ChatGPT (OpenAI)", "Claude
// (Anthropic)" and "Grok (SpaceXAI)" — one row vendor-first with no
// parenthetical, three product-first with one. Product first is the right way
// round because the product is what a reader chooses by; the vendor is the
// attribution, and it belongs in the parens.
//
// This also pins ShortName as the leading word, which is what keeps the terse
// places honest: the key row says "Gemini key", and it should be the same word
// the picker led with rather than an independently chosen abbreviation.
func TestAssistantNamesAreCanonical(t *testing.T) {
	// SpaceXAI is CORRECT and current, not a typo for xAI: SpaceX absorbed xAI
	// in an all-stock deal (Feb 2026) and the company rebranded to SpaceXAI on
	// 6 July 2026, while the assistant kept the name Grok. Anyone whose
	// knowledge predates that will read it as a mistake — it is not, and this
	// note is here so nobody "fixes" it back.
	form := regexp.MustCompile(`^(.+) \(([^()]+)\)$`)
	for _, p := range aiProviders() {
		m := form.FindStringSubmatch(p.Name)
		if m == nil {
			t.Errorf("%s: Name %q is not \"Product (Vendor)\" — the assistant list "+
				"reads as one column and a row in another form is the one that looks "+
				"like a mistake", p.ID, p.Name)
			continue
		}
		if m[1] != p.ShortName {
			t.Errorf("%s: Name %q leads with %q but ShortName is %q — the terse places "+
				"(the key row, \"%s key\") would then use a different word than the picker",
				p.ID, p.Name, m[1], p.ShortName, p.ShortName)
		}
		if strings.TrimSpace(m[2]) == "" {
			t.Errorf("%s: Name %q has an empty vendor", p.ID, p.Name)
		}
	}
}
