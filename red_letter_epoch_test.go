package bibletext

import "testing"

// nkjvRedLetterTableEpoch is the NKJV cacheEpoch whose decoder output the
// generated red-letter table was fingerprinted against.
//
// The table refuses a verse unless its rune count AND a hash of its text both
// match (tableSpansFor), and when it refuses, redLetterRuns paints the WHOLE
// verse red. That fallback is sound when offsets drift, but it is not sound
// when the TEXT changed: a verse in which Jesus quotes the Old Testament and
// the quotation carries the divine name is then printed as though the
// narrator's words were his too.
//
// Any decoder change that alters the text breaks the fingerprints silently,
// because the licensed text is not in this repository and nothing offline can
// compare the table against it. So the epoch is recorded here and checked, and
// the failure is a red build rather than fifteen mis-attributed verses.
//
// When this fails: regenerate with scripts/gen-nkjv-redletter.py against a
// cache written by the NEW decoder, then raise this constant. The script reads
// the cached HTML it already has, so it costs no API quota beyond one fresh
// download of the edition.
const nkjvRedLetterTableEpoch = 7

func TestNKJVRedLetterTableMatchesTheDecoderThatMadeIt(t *testing.T) {
	v, ok := versionByID("nkjv")
	if !ok {
		t.Fatal("no nkjv version registered")
	}
	if v.cacheEpoch != nkjvRedLetterTableEpoch {
		t.Fatalf(
			"the NKJV red-letter table was fingerprinted at cache epoch %d and the decoder is now at %d.\n"+
				"A verse whose text changed no longer matches its fingerprint, and a verse that fails to match\n"+
				"is painted red from end to end — the narrator's words attributed to Christ. Regenerate\n"+
				"red_letter_nkjv_data.go against the new decoder and raise nkjvRedLetterTableEpoch.",
			nkjvRedLetterTableEpoch, v.cacheEpoch)
	}
}
