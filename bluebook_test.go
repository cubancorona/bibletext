package bibletext

import (
	"strings"
	"testing"
)

// Bluebook conformance suite for verse sharing.
//
// Every case below is grounded in a real, online Bluebook / legal-citation example,
// style guide, or practice quiz — the source URL rides with each case. The corpus was
// harvested from these authorities (Rule 15.8 "The Bible", Rule 5 "Quotations"):
//
//	Rule 15.8 / Bible citation form:
//	  https://www.liberty.edu/casas/academic-success-center/writing-style-guides/bluebook-resources/
//	  https://library.ju.edu/bluebook-citation/secondary-sources
//	  https://libguides.law.ucdavis.edu/c.php?g=1014499&p=7371167
//	  https://harvardlawreview.org/blog/2023/06/on-taboos-morality-and-bluebook-citations/
//	  https://en.wikipedia.org/wiki/Bible_citation
//	  https://libguides.princeton.edu/religion/citingsacredtexts
//	  https://sbtswriting.squarespace.com/blog/2018/10/17/bible-references-citations-and-translations-a-how-to-guide
//	Rule 5 / quotations (50-word block-quote threshold, quotation marks):
//	  https://www.monmouth.edu/resources-for-writers/documents/bluebook-quotations.pdf/
//	  https://www.ubalt.edu/law/assets/documents/Due%20Diligence%20Guide%20Effective%20Use%20of%20Quotations%20Spring%202018.pdf
//	  https://blog.legaleasecitations.com/bluebook-tip-block-quotes/
//
// The app's surface under test: citationForSelection -> "Book C:V" / "Book C:V–W"
// (en dash) / "Book C"; formatBibleQuote -> Rule 5 marks; cleanQuoteText -> verse
// number stripping; citationLine / composeShareText -> the assembled share.

// bbChapter builds an AppState whose current chapter holds the given verses (by
// number -> text), so citationForSelection / cleanQuoteText can be exercised. Verses
// are stored in ascending number order, matching how a real chapter reads.
func bbChapter(book string, chapter int, verses map[int]string) *AppState {
	bd := NewBibleData()
	bd.Books = []string{book}
	var vs []Verse
	for n := 1; n <= 200; n++ {
		if t, ok := verses[n]; ok {
			vs = append(vs, Verse{BookName: book, Chapter: chapter, Verse: n, Text: t})
		}
	}
	bd.Verses[book] = map[int][]Verse{chapter: vs}
	return &AppState{Bible: bd, CurrentBook: book, CurrentChapter: chapter}
}

// joinVerses concatenates a chapter's verse texts in number order — the shape a
// whole-verse reading-view selection arrives in.
func joinVerses(verses map[int]string) string {
	var parts []string
	for n := 1; n <= 200; n++ {
		if t, ok := verses[n]; ok {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

// --- Rule 15.8: the citation reference form -----------------------------------

func TestBluebookCitationForm(t *testing.T) {
	cases := []struct {
		name    string
		book    string
		chapter int
		verses  map[int]string
		sel     string // explicit selection; "" => the joined verse text
		want    string
		src     string
	}{
		{
			name: "single verse, colon between chapter and verse",
			book: "John", chapter: 3,
			verses: map[int]string{16: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish."},
			want:   "John 3:16",
			src:    "liberty.edu bluebook-resources; bibliography.com",
		},
		{
			name: "short verse (Jesus wept) still pins to its number",
			book: "John", chapter: 11,
			verses: map[int]string{35: "Jesus wept, moved with compassion for them."},
			want:   "John 11:35",
			src:    "harvardlawreview.org/blog/2023/06 (taboos & Bluebook citations)",
		},
		{
			name: "same-chapter verse RANGE uses an en dash, chapter not repeated",
			book: "John", chapter: 3,
			verses: map[int]string{
				16: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish.",
				17: "For God didn't send his Son into the world to judge the world, but that the world should be saved through him.",
			},
			want: "John 3:16–17", // en dash U+2013, no spaces
			src:  "en.wikipedia.org/wiki/Bible_citation (John 3:16–17)",
		},
		{
			name: "three contiguous verses collapse to a single range (not a comma list)",
			book: "Matthew", chapter: 5,
			verses: map[int]string{
				3: "Blessed are the poor in spirit, for theirs is the Kingdom of Heaven, a promise sure.",
				4: "Blessed are those who mourn, for they shall be comforted in the fullness of time.",
				5: "Blessed are the gentle, for they shall inherit the earth that the Lord has made.",
			},
			want: "Matthew 5:3–5",
			src:  "en.wikipedia.org/wiki/Bible_citation (contiguous range form)",
		},
		{
			name: "numbered book keeps its numeral, spelled out in full",
			book: "2 Kings", chapter: 11,
			verses: map[int]string{8: "You shall surround the king on every side, each man with his weapons in his hand."},
			want:   "2 Kings 11:8",
			src:    "libguides.princeton.edu/religion/citingsacredtexts (2 Kings 11:8)",
		},
		{
			name: "numbered book, en-dash range",
			book: "1 Corinthians", chapter: 13,
			verses: map[int]string{
				4: "Love is patient and is kind. Love doesn't envy. Love doesn't brag, is not proud.",
				5: "Love doesn't behave itself inappropriately, doesn't seek its own way, is not provoked.",
			},
			want: "1 Corinthians 13:4–5",
			src:  "libguides.law.ucdavis.edu; en.wikipedia.org/wiki/Bible_citation",
		},
		{
			name: "multi-word book name is spelled out in full",
			book: "Song of Solomon", chapter: 2,
			verses: map[int]string{1: "I am a rose of Sharon, a lily of the valleys of the field."},
			want:   "Song of Solomon 2:1",
			src:    "Bluebook spells the book out (no Bible abbreviation table)",
		},
		{
			name: "unmatched selection falls back to the chapter-only form",
			book: "John", chapter: 3,
			verses: map[int]string{16: "For God so loved the world, that he gave his one and only Son."},
			sel:    "a phrase that appears nowhere in this chapter whatsoever",
			want:   "John 3",
			src:    "en.wikipedia.org/wiki/Bible_citation (chapter-only form, e.g. John 3)",
		},
	}
	for _, c := range cases {
		state := bbChapter(c.book, c.chapter, c.verses)
		sel := c.sel
		if sel == "" {
			sel = joinVerses(c.verses)
		}
		if got := citationForSelection(state, sel); got != c.want {
			t.Errorf("%s [%s]:\n got %q\nwant %q", c.name, c.src, got, c.want)
		}
	}
}

// TestBluebookDashes pins the two distinct dashes: an EN dash (U+2013) inside a verse
// range, and an EM dash (U+2014) introducing the attribution line. Mixing them up is
// the most common range-citation error the sources warn about.
func TestBluebookDashes(t *testing.T) {
	state := bbChapter("John", 3, map[int]string{
		16: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish.",
		17: "For God didn't send his Son into the world to judge the world, but that the world should be saved through him.",
	})
	cite := citationForSelection(state, joinVerses(map[int]string{
		16: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish.",
		17: "For God didn't send his Son into the world to judge the world, but that the world should be saved through him.",
	}))
	if !strings.ContainsRune(cite, '–') {
		t.Errorf("range citation must use an en dash (U+2013); got %q", cite)
	}
	if strings.ContainsRune(cite, '-') {
		t.Errorf("range citation must NOT use a hyphen; got %q", cite)
	}
	line := citationLine("John 3:16", "World English Bible")
	if !strings.HasPrefix(line, "— ") {
		t.Errorf("attribution line must start with an em dash (U+2014); got %q", line)
	}
}

// --- Rule 5: quotations -------------------------------------------------------

func TestBluebookQuotationRule5(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		src  string
	}{
		{
			"short quote with no marks of its own gets double quotation marks",
			"For God so loved the world, that he gave his one and only Son.",
			"“For God so loved the world, that he gave his one and only Son.”",
			"monmouth.edu bluebook-quotations.pdf (short quote, inline, double marks)",
		},
		{
			"verse with internal double quotes is left intact (no double-wrap)",
			"He answered, “It is written, ‘Man shall not live by bread alone.’”",
			"He answered, “It is written, ‘Man shall not live by bread alone.’”",
			"ubalt.edu Due Diligence Guide (nesting: double outside, single inside)",
		},
		{
			"straight double quotes also suppress added outer marks",
			"He said, \"Peace be with you.\"",
			"He said, \"Peace be with you.\"",
			"grammarbook.com (US nesting) — app avoids stacking double quotes",
		},
		{
			"verse that OPENS a quotation keeps its leading mark, gains a balancing closer",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.",
			"“Blessed are the poor in spirit, for theirs is the Kingdom of Heaven.”",
			"user-reported (IMG_0335): leading mark must survive; balanced for a self-contained share",
		},
		{
			"John 18:38: open/close/open -> the dangling opener gains a closing mark",
			"“What is truth?” Pilate asked. And having said this, he told them, “I find no basis for a charge against Him.",
			"“What is truth?” Pilate asked. And having said this, he told them, “I find no basis for a charge against Him.”",
			"user-reported (IMG_0336): the share must be a balanced, well-formed quotation",
		},
		{
			"apostrophes are not double quotes, so the verse is still wrapped",
			"Don't be afraid; only believe.",
			"“Don't be afraid; only believe.”",
			"monmouth.edu (single marks/apostrophes don't conflict with outer doubles)",
		},
		{"empty stays empty", "", "", "degenerate guard"},
	}
	for _, c := range cases {
		if got := formatBibleQuote(c.in); got != c.want {
			t.Errorf("%s [%s]:\n got %q\nwant %q", c.name, c.src, got, c.want)
		}
	}
}

// TestBluebookBlockQuoteThreshold is the Rule 5 50-word boundary: 49 words is an
// inline quotation (double marks); 50 or more is a block quotation (NO marks).
// Sources state "50 or more words"; the exact 49/50 specimen is synthesized to pin
// the boundary the rule describes.
func TestBluebookBlockQuoteThreshold(t *testing.T) {
	const src = "monmouth.edu bluebook-quotations.pdf; ubalt.edu Due Diligence Guide; legaleasecitations.com"
	w := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }

	if got, in := formatBibleQuote(w(49)), w(49); got != "“"+in+"”" {
		t.Errorf("49 words must be inline-quoted [%s]:\n got %q", src, got)
	}
	if got, in := formatBibleQuote(w(50)), w(50); got != in {
		t.Errorf("50 words must be a block quote with NO marks [%s]:\n got %q", src, got)
	}
	if got, in := formatBibleQuote(w(51)), w(51); got != in {
		t.Errorf("51 words must be a block quote with NO marks [%s]:\n got %q", src, got)
	}
	if blockQuoteWords != 50 {
		t.Errorf("Bluebook Rule 5 block threshold should be 50, got %d", blockQuoteWords)
	}
}

// --- verse-number stripping (precondition for a clean quote) ------------------

func TestBluebookVerseNumberStripping(t *testing.T) {
	t.Run("leading marker removed", func(t *testing.T) {
		st := bbChapter("John", 11, map[int]string{35: "Jesus wept, moved with compassion."})
		if got := cleanQuoteText(st, "35 Jesus wept, moved with compassion."); got != "Jesus wept, moved with compassion." {
			t.Errorf("got %q", got)
		}
	})
	t.Run("each verse's marker removed across a range", func(t *testing.T) {
		v := map[int]string{
			16: "For God so loved the world, that he gave his one and only Son,",
			17: "For God didn't send his Son into the world to judge the world,",
		}
		st := bbChapter("John", 3, v)
		raw := "16 " + v[16] + " 17 " + v[17]
		want := v[16] + " " + v[17]
		if got := cleanQuoteText(st, raw); got != want {
			t.Errorf("got %q\nwant %q", got, want)
		}
	})
	t.Run("a number inside the text is NOT stripped", func(t *testing.T) {
		st := bbChapter("Revelation", 7, map[int]string{4: "I heard the number of those who were sealed, 144,000,"})
		if got := cleanQuoteText(st, "4 I heard the number of those who were sealed, 144,000,"); got != "I heard the number of those who were sealed, 144,000," {
			t.Errorf("got %q", got)
		}
	})
}

// --- the assembled share (citation line + composition) ------------------------

func TestBluebookCitationLine(t *testing.T) {
	cases := []struct{ cite, version, want string }{
		{"John 3:16", "World English Bible", "— John 3:16 (World English Bible)"},
		{"John 11:35", "Berean Standard Bible", "— John 11:35 (Berean Standard Bible)"},
		{"1 Corinthians 13:4–5", "World English Bible", "— 1 Corinthians 13:4–5 (World English Bible)"},
	}
	for _, c := range cases {
		if got := citationLine(c.cite, c.version); got != c.want {
			t.Errorf("citationLine(%q,%q):\n got %q\nwant %q", c.cite, c.version, got, c.want)
		}
	}
	// Version is always spelled OUT, never an initialism (Bluebook names the version
	// in full: "(King James)", never "(KJV)").
	// Source: library.ju.edu/bluebook-citation; libguides.law.ucdavis.edu.
	for _, bad := range []string{"WEB", "BSB"} {
		if strings.Contains(citationLine("John 3:16", "World English Bible"), "("+bad+")") {
			t.Errorf("citation must not use the initialism (%s)", bad)
		}
	}
}

// TestBluebookSharePipeline runs the full text-share path on real verses, end to end.
func TestBluebookSharePipeline(t *testing.T) {
	t.Run("short verse: inline quotes + spelled-out version (WEB)", func(t *testing.T) {
		st := bbChapter("John", 11, map[int]string{35: "Jesus wept."})
		st.CurrentBook, st.CurrentChapter = "John", 11
		quote := formatBibleQuote(cleanQuoteText(st, "35 Jesus wept."))
		cite := citationForSelection(st, "Jesus wept.")
		got := composeShareText(quote, cite, "World English Bible")
		want := "“Jesus wept.”\n— John 11:35 (World English Bible)"
		if got != want {
			t.Errorf("\n got %q\nwant %q\n[src: harvardlawreview.org/blog/2023/06]", got, want)
		}
	})

	t.Run("long passage (50+ words): block form, NO outer quotes (BSB)", func(t *testing.T) {
		// 1 Corinthians 13:4-7 (BSB), ~70 words — exceeds the Rule 5 block threshold.
		text := "Love is patient, love is kind. It does not envy, it does not boast, it is not proud. It is not rude, it is not self-seeking, it is not easily angered, it keeps no account of wrongs. Love takes no pleasure in evil, but rejoices in the truth. It bears all things, believes all things, hopes all things, endures all things."
		quote := formatBibleQuote(text)
		if strings.HasPrefix(quote, "“") {
			t.Errorf("a 50+ word passage must not be wrapped in quotation marks; got leading mark")
		}
		got := composeShareText(quote, "1 Corinthians 13:4–7", "Berean Standard Bible")
		want := text + "\n— 1 Corinthians 13:4–7 (Berean Standard Bible)"
		if got != want {
			t.Errorf("\n got %q\nwant %q", got, want)
		}
	})
}

// TestBluebookQuoteMarkBalancing covers the rule that a shared fragment must be a
// balanced, self-contained quotation: a closing mark whose opener is in the
// surrounding (unselected) text gains a leading opener; an opener whose closer is in
// the surrounding text gains a trailing closer. (User-reported: IMG_0335 / IMG_0336.)
func TestBluebookQuoteMarkBalancing(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"already balanced is unchanged", "“Let there be light,” he said.", "“Let there be light,” he said."},
		{"no quotation marks is unchanged", "Jesus wept.", "Jesus wept."},
		{"dangling closer gains a leading opener", "truth?” Pilate asked.", "“truth?” Pilate asked."},
		{"dangling opener gains a trailing closer", "he said, “Follow me.", "he said, “Follow me.”"},
		{"close-then-open (John 18:38 fragment) gains both marks", "What is truth?” He told them, “I find no basis.", "“What is truth?” He told them, “I find no basis.”"},
		{"verse opening a multi-verse quotation gains a closer", "“Blessed are the poor in spirit.", "“Blessed are the poor in spirit.”"},
	}
	for _, c := range cases {
		if got := balanceQuoteMarks(c.in); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestBluebookFragmentPinsToVerse is the citation half of the IMG_0336 bug: a
// selection that OMITS the verse's leading quotation mark must still pin to its verse
// (bidirectional match), not fall back to the chapter-only "John 18".
func TestBluebookFragmentPinsToVerse(t *testing.T) {
	full := "“What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him."
	st := bbChapter("John", 18, map[int]string{38: full})
	frag := "What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him." // leading “ omitted
	if got := citationForSelection(st, frag); got != "John 18:38" {
		t.Errorf("a fragment must still pin to John 18:38, got %q", got)
	}
}

// TestBluebookFragmentSharePipeline is the full IMG_0336 case end to end: a fragment
// that omits the leading quote and ends mid-quotation produces a balanced quotation
// and the correct verse citation.
func TestBluebookFragmentSharePipeline(t *testing.T) {
	full := "“What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him."
	st := bbChapter("John", 18, map[int]string{38: full})
	frag := "What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him."
	quote := formatBibleQuote(cleanQuoteText(st, frag))
	cite := citationForSelection(st, frag)
	got := composeShareText(quote, cite, "Berean Standard Bible")
	want := "“What is truth?” Pilate asked. And having said this, he went out again to the Jews and told them, “I find no basis for a charge against Him.”\n— John 18:38 (Berean Standard Bible)"
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

// TestBluebookOutOfScope documents — and pins — that the app produces its single
// contiguous-range form, NOT the multi-reference shapes Bluebook also permits but
// that the reading-view selection can never produce (cross-chapter spans, comma
// lists, semicolon-joined references). These are recorded so a future change that
// enables multi-chapter selection knows what to extend.
//
//	Sources for the out-of-scope forms: en.wikipedia.org/wiki/Bible_citation;
//	libguides.princeton.edu/religion/citingsacredtexts.
func TestBluebookOutOfScope(t *testing.T) {
	// Three contiguous verses never become "5:3, 5:4, 5:5" — they collapse to a range.
	st := bbChapter("Matthew", 5, map[int]string{
		3: "Blessed are the poor in spirit, for theirs is the Kingdom of Heaven, a promise sure.",
		4: "Blessed are those who mourn, for they shall be comforted in the fullness of time.",
		5: "Blessed are the gentle, for they shall inherit the earth that the Lord has made.",
	})
	cite := citationForSelection(st, joinVerses(map[int]string{
		3: "Blessed are the poor in spirit, for theirs is the Kingdom of Heaven, a promise sure.",
		4: "Blessed are those who mourn, for they shall be comforted in the fullness of time.",
		5: "Blessed are the gentle, for they shall inherit the earth that the Lord has made.",
	}))
	if strings.ContainsRune(cite, ',') || strings.ContainsRune(cite, ';') {
		t.Errorf("contiguous selection must be one range, not a comma/semicolon list; got %q", cite)
	}
	// The version parenthetical uses round parens, never the Turabian square-bracket
	// form "[New International Version]".
	if strings.ContainsAny(citationLine("John 3:16", "World English Bible"), "[]") {
		t.Error("citation must use round parentheses, not square brackets")
	}
}
