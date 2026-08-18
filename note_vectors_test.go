package bibletext

// The Go half of the conformance corpus. testdata/note_vectors.txt is
// APPEND-ONLY and is walked by BOTH decoders — this test for share_note.go's,
// and cmd/websitegen's note_vectors test for the web reader's — so the two
// implementations cannot drift apart the way they already once did over
// invalid UTF-8 (docs/NOTE_WIRE_FORMAT.md, "Conformance corpus").

import (
	"os"
	"strings"
	"testing"
)

// parseNoteVector splits one corpus line. Returns ok=false for blanks and
// comments.
func parseNoteVector(line string) (payload, expected, text string, ok bool) {
	line = strings.TrimRight(line, "\r")
	if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
		return "", "", "", false
	}
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return "", "", "", false
	}
	payload, expected = parts[0], parts[1]
	if len(parts) == 3 {
		text = parts[2]
	}
	return payload, expected, text, true
}

func TestNoteVectorCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/note_vectors.txt")
	if err != nil {
		t.Fatal(err)
	}
	vectors := 0
	for i, line := range strings.Split(string(raw), "\n") {
		payload, expected, text, ok := parseNoteVector(line)
		if !ok {
			continue
		}
		vectors++
		var want NoteOutcome
		switch expected {
		case "ok":
			want = NoteOutcomeOK
		case "newer":
			want = NoteOutcomeNewer
		case "damaged":
			want = NoteOutcomeDamaged
		default:
			t.Fatalf("line %d: unknown expected outcome %q", i+1, expected)
		}
		rec, got := DecodeNote(payload)
		if got != want {
			t.Errorf("line %d: outcome %d, want %s (%s)", i+1, got, expected, payload)
			continue
		}
		if want == NoteOutcomeOK && text != "" && rec.Text != text {
			t.Errorf("line %d: text\n got %q\nwant %q", i+1, rec.Text, text)
		}
		if want != NoteOutcomeOK && rec.Text != "" {
			t.Errorf("line %d: a %s payload returned text %q", i+1, expected, rec.Text)
		}
	}
	if vectors < 25 {
		t.Fatalf("only %d vectors parsed — the corpus should never shrink", vectors)
	}
}
