package bibletext

import (
	"strings"
	"testing"
)

// The spans exist to stop the app reddening words Christ did not say. These are
// the verses that were wrong before — each one puts another speaker's words, or
// plain narration, inside the red.
func TestBSBRedLetterSpansExcludeOtherSpeakers(t *testing.T) {
	for _, tc := range []struct {
		book           string
		chapter, verse int
		mustBeRed      string
		mustNotBeRed   string
	}{
		{"John", 8, 11, "Then neither do I condemn you", "No one, Lord"},
		{"Matthew", 20, 22, "You do not know what you are asking", "We can"},
		{"Matthew", 22, 21, "Give to Caesar what is Caesar", "they answered"},
		{"Matthew", 17, 26, "Then the sons are exempt", "From others"},
		{"Mark", 8, 5, "How many loaves do you have", "Seven"},
		{"Luke", 7, 40, "Simon, I have something to tell you", "Tell me, Teacher"},
		{"Mark", 12, 16, "Whose image is this", "they answered"},
		{"John", 11, 34, "Where have you put him", "Come and see, Lord"},
		// Only the second quotation is the saying of Christ; the first is
		// Deuteronomy 25:4, quoted by Paul in the same breath.
		{"1 Timothy", 5, 18, "The worker is worthy of his wages", "muzzle an ox"},
	} {
		key := verseKeyFor(tc.book, tc.chapter, tc.verse)
		spans, ok := bsbRedLetterSpans[key]
		if !ok {
			t.Errorf("%s: no span data at all", key)
			continue
		}
		var red strings.Builder
		for _, s := range spans {
			red.WriteString(bsbSpanTextForTest(key, s))
		}
		got := red.String()
		if !strings.Contains(got, tc.mustBeRed) {
			t.Errorf("%s: %q is not in the red text\n  red = %q", key, tc.mustBeRed, got)
		}
		if strings.Contains(got, tc.mustNotBeRed) {
			t.Errorf("%s: %q IS in the red text — that is not Christ speaking\n  red = %q",
				key, tc.mustNotBeRed, got)
		}
	}
}

// A verse where the crowd speaks throughout must carry no red at all, even
// though the WEB's verse-level mark reddens it (its \wj span runs on from the
// previous verse).
func TestBSBRedLetterLeavesACrowdVerseAlone(t *testing.T) {
	if spans, ok := bsbRedLetterSpans[verseKeyFor("John", 8, 33)]; ok && len(spans) > 0 {
		t.Errorf("John 8:33 is the crowd answering Jesus, but %d span(s) are marked red", len(spans))
	}
}

// Every span must lie inside the verse it belongs to, and they must not overlap.
func TestBSBRedLetterSpansAreWellFormed(t *testing.T) {
	for key, spans := range bsbRedLetterSpans {
		n, ok := bsbRedLetterRunes[key]
		if !ok {
			t.Errorf("%s: spans without a recorded verse length", key)
			continue
		}
		prev := -1
		for _, s := range spans {
			if s.Start < 0 || s.End > n || s.Start >= s.End {
				t.Errorf("%s: span {%d,%d} outside a verse of %d runes", key, s.Start, s.End, n)
			}
			if s.Start < prev {
				t.Errorf("%s: spans overlap or are unsorted at {%d,%d}", key, s.Start, s.End)
			}
			prev = s.End
		}
	}
}

// The rune-length guard must refuse to answer for text it was not computed
// against, or a supplier's edit would silently repaint arbitrary words.
func TestBSBRedLetterRefusesChangedText(t *testing.T) {
	key := verseKeyFor("Mark", 8, 5)
	n := bsbRedLetterRunes[key]
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, strings.Repeat("x", n)); !ok {
		t.Error("refused text of the right length")
	}
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, strings.Repeat("x", n+1)); ok {
		t.Error("accepted text of the WRONG length — stale offsets would be painted")
	}
}

// bsbVerseFixture is the BSB text of exactly the verses these tests name, so
// the tests are self-contained: the app's 6MB cache is not in the repository and
// a test that skipped without it would prove nothing. Regenerate alongside
// red_letter_bsb_data.go if the BSB text is ever updated — the rune offsets and
// this fixture must describe the same text.
var bsbVerseFixture = map[string]string{
	"John 8:11":      "“No one, Lord,” she answered.\n“Then neither do I condemn you,” Jesus declared. “Now go and sin no more.”",
	"Matthew 20:22":  "“You do not know what you are asking,” Jesus replied. “Can you drink the cup I am going to drink?”\n“We can,” the brothers answered.",
	"Matthew 22:21":  "“Caesar’s,” they answered.\nSo Jesus told them, “Give to Caesar what is Caesar’s, and to God what is God’s.”",
	"Matthew 17:26":  "“From others,” Peter answered.\n“Then the sons are exempt,” Jesus said to him.",
	"Mark 8:5":       "“How many loaves do you have?” Jesus asked.\n“Seven,” they replied.",
	"Luke 7:40":      "But Jesus answered him, “Simon, I have something to tell you.”\n“Tell me, Teacher,” he said.",
	"Mark 12:16":     "So they brought it, and He asked them, “Whose image is this? And whose inscription?”\n“Caesar’s,” they answered.",
	"John 11:34":     "“Where have you put him?” He asked.\n“Come and see, Lord,” they answered.",
	"1 Timothy 5:18": "For the Scripture says, “Do not muzzle an ox while it is treading out the grain,” and, “The worker is worthy of his wages.”",
	"John 8:33":      "“We are Abraham’s descendants,” they answered. “We have never been slaves to anyone. How can You say we will be set free?”",
}

func bsbSpanTextForTest(key string, s redLetterSpan) string {
	r := []rune(bsbVerseFixture[key])
	if s.End > len(r) {
		return ""
	}
	return string(r[s.Start:s.End])
}
