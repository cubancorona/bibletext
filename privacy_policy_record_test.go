package bibletext

import (
	"strings"
	"testing"
)

// The privacy policy's three sentences on the AI permission were approved on
// 21 October 2026; their sections, the fourth sentence and the effective date
// were decided on 22 October 2026. The records date each on its own day:
// docs/ROADMAP.md's row 13, decision 5 and its account of the wording, and the
// BACKLOG entry, whose list of what was settled on 21 October holds the three
// sentences alone and whose list of 22 October holds the rest. Mutation: any
// of the three decisions dated 21 October, or "the same day" as the
// sentences' approval, again.
func TestThePolicyDecisionsAreDatedTheDayTheyWereTaken(t *testing.T) {
	flat := func(path string) string {
		return strings.Join(strings.Fields(readRepoFile(t, path)), " ")
	}
	docs := map[string]string{
		"docs/ROADMAP.md": flat("docs/ROADMAP.md"),
		"docs/BACKLOG.md": flat("docs/BACKLOG.md"),
	}
	for _, c := range []struct{ doc, where, want string }{
		{"docs/ROADMAP.md", "row 13",
			"are approved. Decided on 22 October 2026 for the policy: each of the three sentences goes in the section it was drafted for"},
		{"docs/ROADMAP.md", "decision 5",
			"merged on 22 October 2026. Decided on 22 October 2026 for the policy: each of the three sentences goes in the section"},
		{"docs/ROADMAP.md", "the account of the wording",
			"were approved on 21 October 2026. On 22 October 2026 each was placed in the section it was drafted for"},
		{"docs/ROADMAP.md", "the account of the wording", "The date chosen on 22 October 2026 is the day 1.2.23 goes live."},
		{"docs/BACKLOG.md", "the entry's opening",
			"its fourth sentence and its effective date, decided on 22 October 2026"},
		{"docs/BACKLOG.md", "the answer to the two points",
			"The policy's two points that waited for confirmation were answered on 22 October 2026"},
	} {
		if !strings.Contains(docs[c.doc], c.want) {
			t.Errorf("%s, %s, does not date the policy's decisions 22 October 2026: %q", c.doc, c.where, c.want)
		}
	}

	backlog := docs["docs/BACKLOG.md"]
	second := strings.Index(backlog, "Settled on 22 October 2026:")
	if second < 0 {
		t.Fatal("docs/BACKLOG.md has no list of what was settled on 22 October 2026")
	}
	first := strings.LastIndex(backlog[:second], "Settled on 21 October 2026:")
	if first < 0 {
		t.Fatal("docs/BACKLOG.md's list of 22 October 2026 does not follow the list of 21 October 2026")
	}
	settled21, settled22 := backlog[first:second], backlog[second:]
	if end := strings.Index(settled22, " Built on "); end >= 0 {
		settled22 = settled22[:end]
	}
	for _, decided := range []string{"the section it was drafted for", "On Android this record", "goes live"} {
		if strings.Contains(settled21, decided) {
			t.Errorf("docs/BACKLOG.md lists %q among what was settled on 21 October 2026", decided)
		}
		if !strings.Contains(settled22, decided) {
			t.Errorf("docs/BACKLOG.md does not list %q among what was settled on 22 October 2026", decided)
		}
	}
	if !strings.Contains(settled21, "the privacy policy gains three approved sentences") {
		t.Error("docs/BACKLOG.md's list of 21 October 2026 no longer holds the policy's three sentences")
	}
}
