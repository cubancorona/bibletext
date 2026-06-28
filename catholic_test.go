package bibletext

import (
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestDecodeHelloAOCatholic checks the three things that differ from the 66-book path:
// books are matched by USFM id (not helloao's order), the Greek Esther/Daniel come
// through under their plain names, and BibleData.Books is emitted in traditional Catholic
// order (deuterocanon interleaved, NOT appended as the raw data ships it).
func TestDecodeHelloAOCatholic(t *testing.T) {
	// A tiny eng_webc-shaped doc: a couple of protocanon books plus deuterocanon, given
	// in helloao's APPENDED order (Tobit/Esther-Greek/Daniel-Greek at the end).
	body := []byte(`{"books":[
	  {"id":"GEN","order":1,"chapters":[{"chapter":{"number":1,"content":[{"type":"verse","number":1,"content":["In the beginning"]}]}}]},
	  {"id":"NEH","order":16,"chapters":[{"chapter":{"number":1,"content":[{"type":"verse","number":1,"content":["The words of Nehemiah"]}]}}]},
	  {"id":"TOB","order":65,"chapters":[{"chapter":{"number":1,"content":[{"type":"verse","number":1,"content":["The book of Tobit"]}]}}]},
	  {"id":"ESG","order":67,"chapters":[{"chapter":{"number":1,"content":[{"type":"verse","number":1,"content":["In the second year"]}]}}]},
	  {"id":"DAG","order":73,"chapters":[{"chapter":{"number":13,"content":[{"type":"verse","number":1,"content":["There was a man living in Babylon"]}]}}]}
	]}`)
	bd, err := decodeHelloAOCatholic(body)
	if err != nil {
		t.Fatal(err)
	}
	// ESG → "Esther", DAG → "Daniel" (not "Esther (Greek)" / "Daniel (Greek)").
	for _, want := range []string{"Tobit", "Esther", "Daniel"} {
		if _, ok := bd.Verses[want]; !ok {
			t.Errorf("expected verses for %q (Verses keys=%v)", want, keysOf(bd.Verses))
		}
	}
	// Catholic order: the deuterocanon is interleaved, not appended — Tobit sits right
	// after Nehemiah, and Esther after Tobit; Daniel sits among the prophets, well before
	// the New Testament would start.
	idx := map[string]int{}
	for i, b := range bd.Books {
		idx[b] = i
	}
	if !(idx["Nehemiah"] < idx["Tobit"] && idx["Tobit"] < idx["Esther"]) {
		t.Errorf("want Nehemiah < Tobit < Esther in Catholic order; got %v", bd.Books)
	}
	if !(idx["Esther"] < idx["Daniel"]) {
		t.Errorf("want Esther before Daniel; got %v", bd.Books)
	}
}

func keysOf(m map[string]map[int][]Verse) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestCatholicBookListConsistent guards the two static tables against drift: the order
// list is the full 73-book canon, and every id maps to a name that exists in it.
func TestCatholicBookListConsistent(t *testing.T) {
	if len(catholicBooks) != 73 {
		t.Errorf("catholicBooks = %d, want 73", len(catholicBooks))
	}
	if len(usfmToCatholicName) != 73 {
		t.Errorf("usfmToCatholicName = %d ids, want 73", len(usfmToCatholicName))
	}
	inOrder := map[string]bool{}
	for _, b := range catholicBooks {
		if inOrder[b] {
			t.Errorf("duplicate book %q in catholicBooks", b)
		}
		inOrder[b] = true
	}
	for id, name := range usfmToCatholicName {
		if !inOrder[name] {
			t.Errorf("usfmToCatholicName[%q]=%q is not in catholicBooks", id, name)
		}
	}
}

// TestDecodeRealWebCatholic decodes the live eng_webc complete.json end-to-end. Network +
// multi-MB, so it is skipped unless BIBLETEXT_NETWORK_TEST=1.
func TestDecodeRealWebCatholic(t *testing.T) {
	if os.Getenv("BIBLETEXT_NETWORK_TEST") != "1" {
		t.Skip("set BIBLETEXT_NETWORK_TEST=1 to run the live eng_webc decode")
	}
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Get(webcCompleteURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	bd, err := decodeHelloAOCatholic(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateBibleData(bd); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(bd.Books) != 73 {
		t.Errorf("decoded %d books, want 73", len(bd.Books))
	}
	// The 9 deuterocanonical books must all be present with text.
	for _, b := range []string{"Tobit", "Judith", "Wisdom", "Sirach", "Baruch", "1 Maccabees", "2 Maccabees", "Esther", "Daniel"} {
		ch, ok := bd.Verses[b]
		if !ok || len(ch) == 0 {
			t.Errorf("deuterocanon book %q missing or empty", b)
		}
	}
}
