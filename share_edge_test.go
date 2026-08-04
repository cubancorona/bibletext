package bibletext

// Edge and contract tests for the share pipeline, from the coverage audit:
// nil contracts, degenerate selections, dangling markers with no following
// text, unterminated final verses, the '…' rune, and newline-spanning drags.

import (
	"strings"
	"testing"
)

func john3EdgeState() *AppState {
	bd := &BibleData{
		Books: []string{"John"},
		Verses: map[string]map[int][]Verse{"John": {3: {
			{BookName: "John", Book: "John", Chapter: 3, Verse: 16,
				Text: "For God so loved the world, that he gave his one and only Son."},
			{BookName: "John", Book: "John", Chapter: 3, Verse: 17,
				Text: "For God didn’t send his Son into the world to judge the world."},
			{BookName: "John", Book: "John", Chapter: 3, Verse: 18, Text: ""}, // empty verse: skipped everywhere
		}}},
	}
	return &AppState{Bible: bd, CurrentBook: "John", CurrentChapter: 3}
}

// The nil/degenerate contract, in one sweep: nothing panics, nothing invents.
func TestShareNilAndDegenerateContracts(t *testing.T) {
	st := john3EdgeState()

	if text, cite := prepareShareQuote(nil, "abc"); text != "abc" || cite != "" {
		t.Errorf("nil state: got (%q, %q), want (abc, \"\")", text, cite)
	}
	if _, _, _, ok := normalizeShareSelection(nil, "x"); ok {
		t.Error("nil state must not normalize")
	}
	if _, _, _, ok := normalizeShareSelection(st, "   "); ok {
		t.Error("whitespace-only selection must not normalize")
	}
	if _, _, _, ok := normalizeShareSelection(st, "17"); ok {
		t.Error("a bare verse number alone shares nothing via the normalized path")
	}
	if got := completeTrailingSentence(nil, "abc"); got != "abc" {
		t.Errorf("nil state completion must be verbatim: %q", got)
	}
	if got := completeTrailingSentence(st, "zzz not here"); got != "zzz not here" {
		t.Errorf("unlocatable text must be verbatim: %q", got)
	}
	if got := completeTrailingSentence(st, ""); got != "" {
		t.Errorf("empty completion: %q", got)
	}
	if got := addEndOmission("”", '.'); got != "”" {
		t.Errorf("closing-mark-only input must take no mark: %q", got)
	}
	if got := citationForSelection(nil, "x"); got != "" {
		t.Errorf("nil-state citation: %q", got)
	}
	if got := citationForSelection(&AppState{CurrentBook: "John", CurrentChapter: 3}, "x"); got != "John 3" {
		t.Errorf("nil-Bible citation falls back to chapter: %q", got)
	}
	// The dispatcher's guards are side-effect-free for nil/blank/unknown.
	dispatchSelectionAction(nil, selActionShareCite, "x")
	dispatchSelectionAction(st, selActionShareCite, "   ")
	dispatchSelectionAction(st, "no-such-action", "x")
}

// A drag ending exactly on a dangling next-verse marker with NOTHING after it
// ("…Son. 17") drops the bare number; sub-word-only drags ("Fo", "od") cannot
// be repaired and fall back to sharing the fragment verbatim — the documented
// degenerate behavior the ragged-cut sweeps exclude.
func TestShareSelectionDegenerateFragments(t *testing.T) {
	st := john3EdgeState()

	text, cite := prepareShareQuote(st, "For God so loved the world, that he gave his one and only Son. 17")
	if strings.Contains(text, "17") {
		t.Errorf("trailing bare marker survived: %q", text)
	}
	if cite != "John 3:16" {
		t.Errorf("citation = %q, want John 3:16", cite)
	}

	// Sub-word fragments: the normalized path declines; legacy shares verbatim.
	if _, _, _, ok := normalizeShareSelection(st, "Fo"); ok {
		t.Error("a lone leading word fragment must not normalize")
	}
	if _, _, _, ok := normalizeShareSelection(st, "od"); ok {
		t.Error("a lone trailing word fragment must not normalize")
	}
	if text, _ := prepareShareQuote(st, "Fo"); text != "Fo" {
		t.Errorf("sub-word fragment falls back to verbatim: %q", text)
	}
}

// TestShareSelectionSubWordFragment is referenced by the ragged-cut sweeps'
// exclusion comment: the pinned behavior for a drag that never contains a
// whole word.
func TestShareSelectionSubWordFragment(t *testing.T) {
	st := john3EdgeState()
	text, cite := prepareShareQuote(st, "lov")
	if text != "lov" {
		t.Errorf("sub-word drag shares the fragment verbatim (legacy path): %q", text)
	}
	if cite != "John 3" {
		t.Errorf("sub-word drag cites the chapter only: %q", cite)
	}
}

// The '…' rune is a first-class sentence terminal on both sides of the
// pipeline: a verse ending in an ellipsis character completes with it, and
// addEndOmission treats it as already-complete.
func TestShareEllipsisRuneTerminal(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Test"},
		Verses: map[string]map[int][]Verse{"Test": {1: {
			{BookName: "Test", Book: "Test", Chapter: 1, Verse: 1, Text: "The words trailed off…"},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Test", CurrentChapter: 1}
	if got := completeTrailingSentence(st, "The words trailed off"); got != "The words trailed off…" {
		t.Errorf("'…' must be restorable as a terminal: %q", got)
	}
	if got := addEndOmission("The words trailed off…", '.'); got != "The words trailed off…" {
		t.Errorf("a text ending '…' is complete — no four-dot on top: %q", got)
	}
}

// The poetry-display selection shape: now that the reading pane renders poem
// lines, a drag over Psalm 23 carries breaks INSIDE a verse body — as "\n"
// (Android, and some Apple OS builds) or U+2028 LINE SEPARATOR (the classic
// Apple HTML-importer form) — plus the U+00A0 no-break space after each verse
// marker. All are Unicode whitespace, so normalization must flatten them and
// locate; the text share then restores the authored lines from chapter data.
func TestShareSelectionPoetryDisplayShape(t *testing.T) {
	v1 := "The LORD is my shepherd;\nI shall not want."
	v2 := "He makes me lie down in green pastures;\nHe leads me beside quiet waters."
	bd := &BibleData{
		Books: []string{"Psalms"},
		Verses: map[string]map[int][]Verse{"Psalms": {23: {
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 1, Text: v1},
			{BookName: "Psalms", Book: "Psalms", Chapter: 23, Verse: 2, Text: v2},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 23}

	raw := "The LORD is my shepherd;\u2028I shall not want.\n2\u00a0He makes me lie down in green pastures;\nHe leads me beside quiet waters."
	text, cite := prepareShareQuote(st, raw)
	if strings.ContainsAny(text, "\n\u2028\u00a0") {
		t.Errorf("pipeline text must be flat: %q", text)
	}
	if strings.Contains(text, "2 He") {
		t.Errorf("verse marker across a poem break survived: %q", text)
	}
	if cite != "Psalms 23:1–2" {
		t.Errorf("citation = %q, want Psalms 23:1–2", cite)
	}
	restored := restoreShareLineBreaks(st, text)
	want := "The LORD is my shepherd;\nI shall not want.\nHe makes me lie down in green pastures;\nHe leads me beside quiet waters."
	if restored != want {
		t.Errorf("restored lines:\n got %q\nwant %q", restored, want)
	}
}

// A drag spanning the reading view's line/paragraph breaks arrives with
// newlines; normalization flattens them and everything downstream holds.
func TestShareNewlineSpanningSelection(t *testing.T) {
	st := john3EdgeState()
	raw := "For God so loved the world,\nthat he gave his one and only Son.\n\n17 For God didn’t send his Son"
	text, cite := prepareShareQuote(st, raw)
	if strings.ContainsAny(text, "\n") {
		t.Errorf("newlines must be flattened: %q", text)
	}
	if strings.Contains(text, "17 ") {
		t.Errorf("marker across the break survived: %q", text)
	}
	if cite != "John 3:16–17" {
		t.Errorf("citation = %q, want John 3:16–17", cite)
	}
}

// A final verse with NO terminal punctuation at the very end of the chapter:
// the completion scan runs off the corpus without a terminal and must return
// the text unchanged (no invented punctuation).
func TestShareUnterminatedChapterEnd(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Test"},
		Verses: map[string]map[int][]Verse{"Test": {1: {
			{BookName: "Test", Book: "Test", Chapter: 1, Verse: 1, Text: "The grass withers the flower fades"},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Test", CurrentChapter: 1}
	if got := completeTrailingSentence(st, "The grass withers the flower fades"); got != "The grass withers the flower fades" {
		t.Errorf("no terminal exists to restore — text must be verbatim: %q", got)
	}
}

// Orphan punctuation swept in from neighboring sentences is dropped at both
// ends (found by the ragged sweeps): a leading ". " can never open a
// quotation; a trailing clause colon before an omission is dropped per the
// adjacent-punctuation practice (e.g. Maizebook 5.3(d)).
func TestShareOrphanPunctuationTrimmed(t *testing.T) {
	st := john3EdgeState()
	text, _ := prepareShareQuote(st, ". For God didn’t send his Son")
	if strings.HasPrefix(text, ".") {
		t.Errorf("leading orphan terminal must be dropped: %q", text)
	}
	if !strings.HasPrefix(text, "For God didn’t") {
		t.Errorf("content must survive the orphan trim: %q", text)
	}

	bd := &BibleData{
		Books: []string{"Test"},
		Verses: map[string]map[int][]Verse{"Test": {1: {
			{BookName: "Test", Book: "Test", Chapter: 1, Verse: 1,
				Text: "And the prophets, wrote: Jesus of Nazareth is the one."},
		}}},
	}
	st2 := &AppState{Bible: bd, CurrentBook: "Test", CurrentChapter: 1}
	text2, _ := prepareShareQuote(st2, "And the prophets, wrote:")
	if strings.HasSuffix(text2, ":") {
		t.Errorf("trailing clause colon before an omission must be dropped: %q", text2)
	}
}

// The mixed prose→poetry join must restore identically to how the reading
// pane displays it (buildChapterHTML emits <br> at the same boundary —
// TestBuildChapterHTMLMixedProsePoetryJoin): shared lines == displayed lines.
func TestShareMixedJoinRestoreParity(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Exodus"},
		Verses: map[string]map[int][]Verse{"Exodus": {15: {
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 1,
				Text: "Then Moses and the Israelites sang this song to the LORD:"},
			{BookName: "Exodus", Book: "Exodus", Chapter: 15, Verse: 2,
				Text: "The LORD is my strength and my song,\nand He has become my salvation."},
		}}},
	}
	st := &AppState{Bible: bd, CurrentBook: "Exodus", CurrentChapter: 15}

	raw := "Then Moses and the Israelites sang this song to the LORD: 2 The LORD is my strength and my song, and He has become my salvation."
	text, cite := prepareShareQuote(st, raw)
	if cite != "Exodus 15:1–2" {
		t.Errorf("citation = %q, want Exodus 15:1–2", cite)
	}
	restored := restoreShareLineBreaks(st, text)
	want := "Then Moses and the Israelites sang this song to the LORD:\nThe LORD is my strength and my song,\nand He has become my salvation."
	if restored != want {
		t.Errorf("mixed-join restore:\n got %q\nwant %q", restored, want)
	}
}
