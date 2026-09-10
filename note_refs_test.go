package bibletext

import (
	"encoding/json"
	"strings"
	"testing"
)

// The citations inside an NKJV note are kept as spans (Footnote.Refs) beside
// text that is byte-identical to what the flattening produced before the ids
// were kept. Each case is checked three ways: the text, the spans sliced back
// out of it, and the ids; and a control shows the text carries nothing of the
// brackets that carried the ids through normalization.

func noteNode(t *testing.T, xt string) apiBibleNode {
	t.Helper()
	src := `{"name":"note","type":"tag","attrs":{"style":"x","caller":"-","id":"JHN.3.2!x.1","verseId":"JHN.3.2"},"items":[
	  {"name":"char","type":"tag","attrs":{"style":"xo"},"items":[{"type":"text","text":"3:2 "}]},
	  {"name":"char","type":"tag","attrs":{"style":"xt"},"items":[` + xt + `]}]}`
	var n apiBibleNode
	if err := json.Unmarshal([]byte(src), &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func refNode(id, text string) string {
	return `{"name":"ref","type":"tag","attrs":{"id":"` + id + `"},"items":[{"type":"text","text":"` + text + `"}]}`
}

func textNode(s string) string { return `{"type":"text","text":"` + s + `"}` }

// legacyNoteFlattening is the note text as the decoder produced it before the
// citation ids were kept — the oracle every case is held to.
func legacyNoteFlattening(n apiBibleNode) string {
	var b strings.Builder
	var walk func(nodes []apiBibleNode)
	walk = func(nodes []apiBibleNode) {
		for _, c := range nodes {
			style := strings.ToLower(c.Attrs.Style)
			if c.Name == "char" && (style == "xo" || style == "fr") {
				continue
			}
			if c.Text != "" {
				b.WriteString(c.Text)
			}
			walk(c.Items)
		}
	}
	walk(n.Items)
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

func TestAPIBibleNoteKeepsTheCitationSpans(t *testing.T) {
	for _, tc := range []struct {
		name string
		xt   string
		text string
		refs []NoteRef
	}{
		{"two tagged citations, the second a continuation",
			refNode("JHN.7.50", "John 7:50") + "," + textNode("; ") + "," + refNode("JHN.19.39", "19:39"),
			"John 7:50; 19:39",
			[]NoteRef{{"JHN.7.50", 0, 9}, {"JHN.19.39", 11, 16}}},
		{"a parenthesised compare citation, untagged",
			textNode("(Acts 10:38)"),
			"(Acts 10:38)", nil},
		{"a parenthesised group that straddles tagged and untagged citations",
			textNode("(John 1:13; ") + "," + refNode("GAL.6.15", "Gal. 6:15") + "," + textNode("; 1 John 3:9)"),
			"(John 1:13; Gal. 6:15; 1 John 3:9)",
			[]NoteRef{{"GAL.6.15", 12, 21}}},
		{"a range id",
			refNode("MAT.3.1-MAT.3.12", "Matt. 3:1–12"),
			"Matt. 3:1–12",
			[]NoteRef{{"MAT.3.1-MAT.3.12", 0, 12}}},
		{"a citation whose words carry stray spaces is trimmed to its words, the text normalized as before",
			refNode("PSA.5.12", " Ps. 5:12 ") + "," + textNode("; ") + "," + refNode("PSA.28.7", "28:7"),
			"Ps. 5:12 ; 28:7",
			[]NoteRef{{"PSA.5.12", 0, 8}, {"PSA.28.7", 11, 15}}},
		{"a citation nested in another resolves to its own id",
			`{"name":"ref","type":"tag","attrs":{"id":"JHN.7.50-JHN.7.52"},"items":[{"type":"text","text":"John 7:50–52 ("},` + refNode("JHN.7.51", "51") + `,{"type":"text","text":")"}]}`,
			"John 7:50–52 (51)",
			[]NoteRef{{"JHN.7.50-JHN.7.52", 0, 17}, {"JHN.7.51", 14, 16}}},
		{"a ref with an id and no words is dropped, its neighbours untouched",
			refNode("JHN.7.50", "John 7:50") + "," + refNode("JHN.9.16", "") + "," + textNode("; ") + "," + refNode("ACT.2.22", "Acts 2:22"),
			"John 7:50; Acts 2:22",
			[]NoteRef{{"JHN.7.50", 0, 9}, {"ACT.2.22", 11, 20}}},
		{"a ref without an id is words, not a citation",
			`{"name":"ref","type":"tag","attrs":{},"items":[{"type":"text","text":"John 7:50"}]}`,
			"John 7:50", nil},
		{"non-ASCII words keep rune offsets, not byte offsets",
			refNode("ISA.9.6", "Is. 9:6") + "," + textNode("; “see” ") + "," + refNode("JHN.1.18", "John 1:18"),
			"Is. 9:6; “see” John 1:18",
			[]NoteRef{{"ISA.9.6", 0, 7}, {"JHN.1.18", 15, 24}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := noteNode(t, tc.xt)
			got, refs := apiBibleNote(n)
			if got != tc.text {
				t.Errorf("text = %q, want %q", got, tc.text)
			}
			if body := apiBibleNoteBody(n); body != got {
				t.Errorf("apiBibleNoteBody = %q, differs from apiBibleNote's text %q", body, got)
			}
			if oracle := legacyNoteFlattening(n); oracle != got {
				t.Errorf("text = %q, but the flattening before the ids were kept gave %q", got, oracle)
			}
			if strings.ContainsAny(got, string([]rune{noteRefOpen, noteRefClose})) {
				t.Errorf("a bracket leaked into the text: %q", got)
			}
			if len(refs) != len(tc.refs) {
				t.Fatalf("refs = %+v, want %+v", refs, tc.refs)
			}
			runes := []rune(got)
			for i, r := range refs {
				if r != tc.refs[i] {
					t.Errorf("ref %d = %+v, want %+v", i, r, tc.refs[i])
				}
				if r.Start < 0 || r.End > len(runes) || r.Start >= r.End {
					t.Fatalf("ref %d span %d..%d outside the %d-rune text", i, r.Start, r.End, len(runes))
				}
				words := string(runes[r.Start:r.End])
				if strings.TrimSpace(words) != words || words == "" {
					t.Errorf("ref %d span is not trimmed to its words: %q", i, words)
				}
			}
		})
	}
}

// CONTROL: the spans point at the cited words, not merely at plausible
// offsets — slice them out and compare with what the ref tag held.
func TestAPIBibleNoteSpansSliceToTheCitedWords(t *testing.T) {
	n := noteNode(t, refNode("DAN.2.44", "Dan. 2:44")+","+textNode("; ")+","+refNode("MAL.4.6", "Mal. 4:6")+","+textNode("; ")+","+refNode("MAT.4.17", "Matt. 4:17"))
	got, refs := apiBibleNote(n)
	want := []string{"Dan. 2:44", "Mal. 4:6", "Matt. 4:17"}
	if len(refs) != len(want) {
		t.Fatalf("refs = %+v", refs)
	}
	runes := []rune(got)
	for i, r := range refs {
		if s := string(runes[r.Start:r.End]); s != want[i] {
			t.Errorf("span %d = %q, want %q", i, s, want[i])
		}
	}
	// And a deliberately wrong offset does NOT slice to the words: the
	// comparison above can fail.
	if s := string(runes[refs[1].Start+1 : refs[1].End]); s == want[1] {
		t.Fatal("a shifted span still matched; the check proves nothing")
	}
}

// The chapter fixture the footnote capture is proven on: the tagged note
// gains its spans, the untagged one stays bare, and every verse text is
// unchanged from what TestFootnotesNKJVCaptureAndPurity pins.
func TestNKJVFixtureNotesCarryTheirCitations(t *testing.T) {
	byCh, _, _, _, err := decodeAPIBiblePassage(json.RawMessage(nkjvNotedChapter), "John", 3)
	if err != nil {
		t.Fatal(err)
	}
	vs := byCh[3]
	if len(vs) != 3 {
		t.Fatalf("got %d verses, want 3", len(vs))
	}
	if got := vs[0].Footnotes[0].Refs; len(got) != 2 || got[0] != (NoteRef{"JHN.7.50", 0, 9}) || got[1] != (NoteRef{"JHN.19.39", 11, 16}) {
		t.Errorf("v2's citations = %+v", got)
	}
	if got := vs[1].Footnotes[0].Refs; got != nil {
		t.Errorf("the translator footnote grew citations: %+v", got)
	}
	if got := vs[2].Footnotes[0]; got.Text != "Numbers 21:9" || got.Refs != nil {
		t.Errorf("the untagged citation changed: %+v", got)
	}
	if vs[0].Text != "This man came to Jesus by night and said to Him." ||
		vs[2].Text != "As Moses lifted up the serpent,\neven so must the Son of Man be lifted up." {
		t.Errorf("verse text changed with the citation capture: %q / %q", vs[0].Text, vs[2].Text)
	}
}

// The cache envelope carries the spans, and a blob written before the field
// existed loads with none — the panel must treat that as words-only.
func TestNoteRefsRoundTripTheCacheAndOldBlobsHaveNone(t *testing.T) {
	fn := Footnote{Anchor: 3, Text: "John 7:50; 19:39", Kind: footnoteKindCrossref, Caller: "-",
		Refs: []NoteRef{{"JHN.7.50", 0, 9}, {"JHN.19.39", 11, 16}}}
	blob, err := json.Marshal(fn)
	if err != nil {
		t.Fatal(err)
	}
	var back Footnote
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Refs) != 2 || back.Refs[1] != fn.Refs[1] {
		t.Errorf("round trip lost the citations: %+v", back.Refs)
	}
	var old Footnote
	if err := json.Unmarshal([]byte(`{"anchor":3,"text":"John 7:50; 19:39","kind":"crossref","caller":"-"}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Refs != nil {
		t.Errorf("an old blob grew citations: %+v", old.Refs)
	}
	// CONTROL: a note without citations serialises without the field at all.
	plain, _ := json.Marshal(Footnote{Text: "(Acts 10:38)", Kind: footnoteKindCrossref})
	if strings.Contains(string(plain), "refs") {
		t.Errorf("an untagged note wrote an empty refs field: %s", plain)
	}
}
