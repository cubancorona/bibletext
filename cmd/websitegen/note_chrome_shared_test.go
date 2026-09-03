package main

// The web reader's note chrome comes from the SAME Go functions the app panes
// consume — step 9 of docs/NOTE_CHROME_UNIFICATION.md. These pins hold the
// seam in both directions: the template carries no private spelling of a
// shared value, and the generate-time fill emits exactly what the shared
// functions answer. Divergences that remain (the verse target's
// header-inclusive margin, the own verb arm's absence, minimize's global
// highlight suppress) are stated in the template beside the code they excuse.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

func TestWebReaderNoteChromeComesFromTheSharedFunctions(t *testing.T) {
	for _, ph := range []string{"__NOTE_BYLINE__", "__NOTE_PILL_LABEL__"} {
		if !strings.Contains(readerJSTemplate, ph) {
			t.Errorf("readerJSTemplate lost %s — the value is composed in Go and must be emitted", ph)
		}
	}
	if strings.Contains(readerJSTemplate, "'Note from Friend'") {
		t.Error("readerJSTemplate spells the byline itself again — senderByline is the one author")
	}
	if !strings.Contains(readerCSSTemplate, "__NOTE_LEAD__") {
		t.Error("readerCSSTemplate lost __NOTE_LEAD__ — the arrival lead is the spec's, not this file's")
	}

	js := readerJS(nil)
	wantByline, err := json.Marshal(bibletext.WebNoteByline())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, string(wantByline)) {
		t.Errorf("the generated reader.js does not carry the shared byline %s", wantByline)
	}
	wantPill, err := json.Marshal(bibletext.WebNotePillLabel())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(js, string(wantPill)) {
		t.Errorf("the generated reader.js does not carry the shared pill label %s", wantPill)
	}
	for _, ph := range []string{"__NOTE_BYLINE__", "__NOTE_PILL_LABEL__"} {
		if strings.Contains(js, ph) {
			t.Errorf("%s survives into the generated reader.js unfilled", ph)
		}
	}

	css := readerCSS("r.woff2", "b.woff2")
	if want := fmt.Sprintf("scroll-margin-top:%dpx", bibletext.WebNoteArrivalLeadPx()); !strings.Contains(css, want) {
		t.Errorf("the generated reader.css does not carry the shared arrival lead %q", want)
	}
	if strings.Contains(css, "__NOTE_LEAD__") {
		t.Error("__NOTE_LEAD__ survives into the generated reader.css unfilled")
	}
}

// The tail rule: a card has a tail iff its anchor names a passage
// (noteChrome.hasTail). The web's spelling is a class the chapter-top parking
// sets and the stylesheet gates the tail on.
func TestWebReaderTailObeysTheSharedRule(t *testing.T) {
	for _, frag := range []string{
		"el.classList.add('notail');",
		"el.classList.remove('notail');",
	} {
		if !strings.Contains(readerJSTemplate, frag) {
			t.Errorf("readerJSTemplate lost %q — an anchorless card would grow a tail claiming the first paragraph", frag)
		}
	}
	if !strings.Contains(readerCSSTemplate, ".note.notail::after{display:none}") {
		t.Error("readerCSSTemplate lost the notail gate — the class would be set and change nothing")
	}
}

// One verb vocabulary: the bubble and the card offer the same action under the
// same name. Counted over CODE — the template's comments narrate the old
// second vocabulary and must not count as a parser.
func TestWebReaderSpeaksOneVerbVocabulary(t *testing.T) {
	src := stripJSLineComments(readerJSTemplate)
	if strings.Contains(src, "'Hide note'") {
		t.Error("the second vocabulary is back: 'Hide note' and 'Minimize note' are one action")
	}
	if got := strings.Count(src, "'Minimize note'"); got != 2 {
		t.Errorf("'Minimize note' appears %d times, want 2 — the card's button and the bubble's", got)
	}
	if got := strings.Count(src, "'Delete note'"); got != 2 {
		t.Errorf("'Delete note' appears %d times, want 2 — the card's button and the bubble's", got)
	}
}

func stripJSLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
