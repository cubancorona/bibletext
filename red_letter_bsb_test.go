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
		if _, ok := bsbRedLetterHashes[key]; !ok {
			t.Errorf("%s: spans without a recorded content fingerprint", key)
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

// The content guard must refuse text it was not computed against, including a
// same-length edit that the former rune-count-only guard accepted.
func TestBSBRedLetterRefusesChangedText(t *testing.T) {
	key := verseKeyFor("Mark", 8, 5)
	n := bsbRedLetterRunes[key]
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, bsbVerseFixture[key]); !ok {
		t.Error("refused the exact text used to generate the offsets")
	}
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, strings.Repeat("x", n)); ok {
		t.Error("accepted different text of the same length — stale offsets would be painted")
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
	"John 8:11":     "“No one, Lord,” she answered.\n“Then neither do I condemn you,” Jesus declared. “Now go and sin no more.”",
	"Matthew 20:22": "“You do not know what you are asking,” Jesus replied. “Can you drink the cup I am going to drink?”\n“We can,” the brothers answered.",
	"Matthew 22:21": "“Caesar’s,” they answered.\nSo Jesus told them, “Give to Caesar what is Caesar’s, and to God what is God’s.”",
	"Matthew 17:26": "“From others,” Peter answered.\n“Then the sons are exempt,” Jesus said to him.",
	"Mark 8:5":      "“How many loaves do you have?” Jesus asked.\n“Seven,” they replied.",
	"Luke 7:40":     "But Jesus answered him, “Simon, I have something to tell you.”\n“Tell me, Teacher,” he said.",
	"Mark 12:16":    "So they brought it, and He asked them, “Whose image is this? And whose inscription?”\n“Caesar’s,” they answered.",
	"John 11:34":    "“Where have you put him?” He asked.\n“Come and see, Lord,” they answered.",
	"John 8:33":     "“We are Abraham’s descendants,” they answered. “We have never been slaves to anyone. How can You say we will be set free?”",
}

func TestBSBTableMatchesPublisherVerseJudgements(t *testing.T) {
	if got, want := len(bsbRedLetterSpans), 2042; got != want {
		t.Fatalf("BSB marked verse count = %d, want publisher count %d", got, want)
	}
	runs := 0
	for _, spans := range bsbRedLetterSpans {
		runs += len(spans)
	}
	if want := 2294; runs != want {
		t.Fatalf("BSB marked run count = %d, want publisher count %d", runs, want)
	}

	for _, key := range []string{
		"Matthew 27:43", "Matthew 27:63", "Mark 5:31", "Mark 8:23",
		"Luke 24:7", "Revelation 4:1",
	} {
		if _, ok := bsbRedLetterSpans[key]; !ok {
			t.Errorf("%s: publisher marks the verse, but the table does not", key)
		}
	}
	for _, key := range []string{
		"1 Timothy 5:18", "John 5:12", "John 8:33", "John 8:52",
		"John 9:11", "John 12:34", "Luke 9:55", "Luke 9:56", "Luke 20:23",
		"Revelation 1:8", "Revelation 21:5", "Revelation 21:6",
		"Revelation 21:7", "Revelation 21:8", "Revelation 22:14", "Revelation 22:15",
	} {
		if _, ok := bsbRedLetterSpans[key]; ok {
			t.Errorf("%s: publisher leaves the verse black, but the table marks it", key)
		}
	}
}

func bsbSpanTextForTest(key string, s redLetterSpan) string {
	r := []rune(bsbVerseFixture[key])
	if s.End > len(r) {
		return ""
	}
	return string(r[s.Start:s.End])
}

// The switch has to be real: off, the app must fall back to exactly what it did
// before these spans existed, or "turn it off" is not an option we actually have.
func TestBSBRedLetterSwitch(t *testing.T) {
	key := verseKeyFor("Mark", 8, 5)
	text := bsbVerseFixture[key]

	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "0")
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, text); ok {
		t.Error("switched off, but span data was still handed out")
	}
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "1")
	if _, ok := bsbRedLetterSpansFor("Mark", 8, 5, text); !ok {
		t.Error("switched on, but no span data came back")
	}
	t.Setenv("BIBLETEXT_BSB_RED_LETTER", "")
	if got, want := bsbRedLetterSpansEnabled(), bsbRedLetterSpansOn; got != want {
		t.Errorf("with nothing set the constant must decide: got %v, want %v", got, want)
	}
}
