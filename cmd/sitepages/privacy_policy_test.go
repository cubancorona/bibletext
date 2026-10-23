package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The privacy policy's sentences on the AI permission, each in the section it
// was approved for, word for word (docs/ROADMAP.md, 1.2.23, row 13): the
// question in the AI section; its withdrawal among the reader's choices,
// beside None and Clear, which its "also" follows; and the record the app
// keeps, with what Android's backup does with it, among what is stored on the
// device. The Android sentence follows the record's at once, since "this
// record" is that one.
var privacyPermissionSentences = []struct{ section, sentence string }{
	{"Optional AI study features (bring your own key)",
		"Before the first request to each provider, the app asks whether it may send your text there, and sends nothing until you allow it."},
	{"Your choices, retention, and deletion",
		"Choosing None, or clearing or changing a provider's key, also withdraws the permission you gave that provider; the app asks again before its next request."},
	{"Information stored on your device",
		"The app also keeps a record, on your device, of each AI provider you have allowed it to send text to."},
	{"Information stored on your device",
		"On Android this record is part of the app's settings, which Android may include in your device backup; it holds no key and none of your text."},
}

var policySectionHeading = regexp.MustCompile(`(?s)<h2>(.*?)</h2>`)

// policySections maps each <h2> heading of page to the words a reader reads
// under it, up to the next heading (policyWords).
func policySections(page string) map[string]string {
	sections := map[string]string{}
	heads := policySectionHeading.FindAllStringSubmatchIndex(page, -1)
	for i, h := range heads {
		end := len(page)
		if i+1 < len(heads) {
			end = heads[i+1][0]
		}
		sections[policyWords([]byte(page[h[2]:h[3]]))] = policyWords([]byte(page[h[1]:end]))
	}
	return sections
}

func TestThePrivacyPolicyPlacesEachPermissionSentenceInItsSection(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "privacy.html"))
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	words := policyWords(b)
	sections := policySections(page)
	for _, s := range privacyPermissionSentences {
		if n := strings.Count(words, s.sentence); n != 1 {
			t.Errorf("the policy carries %q %d times, want once", s.sentence, n)
		}
		section, ok := sections[s.section]
		if !ok {
			t.Errorf("the policy has no section %q", s.section)
			continue
		}
		if !strings.Contains(section, s.sentence) {
			t.Errorf("%q is not in the section %q", s.sentence, s.section)
		}
		// Word for word in the source too, not only once tags are dropped
		// and white space collapsed.
		if !strings.Contains(page, s.sentence) {
			t.Errorf("docs/privacy.html does not carry %q on one line, as approved", s.sentence)
		}
	}
	record, android := privacyPermissionSentences[2].sentence, privacyPermissionSentences[3].sentence
	if !strings.Contains(words, record+" "+android) {
		t.Error("the sentence on Android's backup does not follow the record's at once")
	}
}

// CONTROL: the section reader finds every heading of the real policy and
// nothing under one heading reaches into the next.
func TestThePolicySectionReaderSeesTheRealSections(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "privacy.html"))
	if err != nil {
		t.Fatal(err)
	}
	sections := policySections(string(b))
	for _, h := range []string{"The short version", "Information stored on your device",
		"Bible text, study data, and audio", "Optional AI study features (bring your own key)",
		"Shared verse links and notes", "Your choices, retention, and deletion",
		"Children's privacy", "Changes to this policy", "Contact"} {
		if _, ok := sections[h]; !ok {
			t.Errorf("the section reader does not see %q", h)
		}
	}
	if s := sections["The short version"]; strings.Contains(s, "Your reading position") || !strings.HasPrefix(s, "BibleText has no user accounts") {
		t.Errorf("the short version's section is not cut at the next heading: %.80q", s)
	}
}
