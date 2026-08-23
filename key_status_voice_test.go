package bibletext

// THE TWO KEY-TEST RESULTS SPEAK THE SAME WAY.
//
// A settings sheet holds two key fields — the AI provider's and API.Bible's —
// and both report the same event: a key was tested and it works. They said it
// differently ("Working" against "Key works.") and, because one used a plain
// Label and the other a caption-styled RichText, at two different SIZES, a few
// hundred points apart in one scroll — visibly mismatched with both on screen.
//
// This pins the shape rather than the sentences: a confirmation that names what
// works, then a line saying what it unlocks. Wording can be edited; the two
// drifting apart again is the thing worth failing on.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestKeyTestResultsShareOneVoice(t *testing.T) {
	for _, f := range []string{"ai_settings.go", "bible_key_settings.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		s := string(src)

		// 1. Status voice, not headline: the result renders at caption size.
		if !strings.Contains(s, "SizeNameCaptionText") {
			t.Errorf("%s: the key-test result must render at caption size — a test "+
				"result is a status, not a headline, and the other field already "+
				"does this", f)
		}

		// 2. The success line names what works, in the same words as its twin.
		if !regexp.MustCompile(`"✓ Key works\.`).MatchString(s) {
			t.Errorf("%s: the success message should read \"✓ Key works.\" — the "+
				"same event in both fields should read the same way", f)
		}

		// 3. And says what it unlocks, on its own line.
		if !regexp.MustCompile(`"✓ Key works\.\\n`).MatchString(s) {
			t.Errorf("%s: after confirming the key, say what it makes available "+
				"(the other field names the translation it unlocks)", f)
		}
	}
}

// THE TWO KEY FIELDS OFFER THE SAME THREE VERBS IN THE SAME ORDER.
//
// They did not: the assistant's key read Paste, Clear, Test and API.Bible's
// read Paste, Test, Clear — the same three buttons, in one sheet, teaching
// different muscle memory. Worse, the first order put the only destructive
// control between the two a reader reaches for most, so a mis-tap aimed at
// either neighbour wiped the key.
//
// Asserted on the source because the rows are built in different files and
// there is no shared constructor to test through; a shared constructor would
// be the better fix if a third key field ever appears.
func TestKeyFieldButtonsShareOneOrder(t *testing.T) {
	order := regexp.MustCompile(`NewHBox\(pasteBtn, testBtn, clearBtn`)
	for _, f := range []string{"ai_settings.go", "bible_key_settings.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		if !order.MatchString(string(src)) {
			t.Errorf("%s: the key row should read Paste, Test key, Clear — the same "+
				"order as the other key field, with the destructive control last "+
				"rather than between the two most-used ones", f)
		}
	}
}
