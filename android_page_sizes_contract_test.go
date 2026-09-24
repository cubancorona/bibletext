package bibletext

// The Android bridge cannot import Go, so the sizes inside the page
// (reading_page.go) reach it as copies, and the dialect carries them in tags of
// its own that the bridge's tag handler turns into spans. This holds the copies
// to the spec, and holds the tags the dialect writes to the tags the handler
// knows: a tag the handler does not know imports as plain text at body size,
// and nothing on a device says so.

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestAndroidPageSizesEqualTheSpec(t *testing.T) {
	src, err := os.ReadFile("android/BtBridge.java")
	if err != nil {
		t.Fatalf("cannot read the bridge: %v", err)
	}
	re := regexp.MustCompile(`NUM_EM = ([0-9.]+)f, NUM_LIFT_EM = ([0-9.]+)f, GAP_MARK_EM = ([0-9.]+)f, FN_EM = ([0-9.]+)f, FN_RULE_GAP_EM = ([0-9.]+)f, FN_ENTRY_GAP_EM = ([0-9.]+)f`)
	m := re.FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("the bridge's page sizes are no longer declared in the shape this test reads; " +
			"re-point the test rather than dropping it")
	}
	for i, want := range []struct {
		name string
		v    float64
	}{
		{"NUM_EM", readingNumeralEm},
		{"NUM_LIFT_EM", readingNumeralLiftEm},
		{"GAP_MARK_EM", readingGapMarkEm},
		{"FN_EM", readingFootnoteEm},
		{"FN_RULE_GAP_EM", readingFootnoteRuleGapEm},
		{"FN_ENTRY_GAP_EM", readingFootnoteEntryGapEm},
	} {
		got, err := strconv.ParseFloat(m[i+1], 64)
		// A float literal holds a third to five places; a hundredth of a pixel
		// at the largest body the app sets.
		if err != nil || math.Abs(got-want.v) > 1e-4 {
			t.Errorf("android/BtBridge.java %s = %s; reading_page.go says %g", want.name, m[i+1], want.v)
		}
	}
}

// Every size tag the dialect writes is one the handler turns into a span, the
// importer's own <small> is gone, and both imports hand the handler over.
func TestAndroidDialectSizeTagsAreTheHandlers(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)

	// A chapter with numbers, an omitted verse's mark and a footnote section.
	verses := footnoteFixtureVerses()
	st := footnoteFixtureState(verses)
	html := buildChapterHTMLAndroid(st, verses)
	gapState, gapVerses := gappedState()
	html += buildChapterHTMLAndroid(gapState, gapVerses)

	tags := map[string]bool{}
	for _, m := range regexp.MustCompile(`<(bt[a-z]+)>`).FindAllStringSubmatch(html, -1) {
		tags[m[1]] = true
	}
	for _, want := range []string{"btnum", "btgap", "btfn"} {
		if !tags[want] {
			t.Errorf("the fixture's markup carries no <%s> — the case does not exercise it:\n%s", want, html)
		}
	}
	java := readNativeSource(t, "android/BtBridge.java")
	handler := java[strings.Index(java, "private static final Html.TagHandler READING_SIZES"):]
	handler = handler[:strings.Index(handler, "\n    };\n")]
	for tag := range tags {
		if strings.Count(handler, `"`+tag+`".equals(tag)`) < 2 {
			t.Errorf("the dialect writes <%s> and READING_SIZES does not both accept it and size it", tag)
		}
	}
	if strings.Contains(html, "<small>") {
		t.Errorf("the dialect still writes the importer's <small> (0.8):\n%s", html)
	}
	if got := strings.Count(java, ", null, READING_SIZES)"); got != 2 {
		t.Errorf("%d of the two chapter imports hand READING_SIZES over", got)
	}
	// The numeral's lift is the spec's, from the body as SET, and the plain
	// superscript the <sup> made is dropped before the chapter is laid out.
	if !strings.Contains(handler, "new NumeralSpan(Math.round(NUM_LIFT_EM * lastTextPx))") {
		t.Error("the numeral is not lifted by NUM_LIFT_EM of the body as set")
	}
	if !strings.Contains(java, "dropPlainSuperscripts((Spannable) s);\n                    footnotesTakeTheirNewlines((Spannable) s);") {
		t.Error("setHtml does not drop the importer's own superscripts and give the footnotes their newlines")
	}
}

// The compact page (the book page) carries its heading and footnote air on
// spans. Which page a chapter is on comes from the import: it used to be
// inferred from blank lines, and the compact page has two — the empty
// "paragraph" after the closing newline every chapter ends with, and a psalm
// title's own gap — so the Android book page drew no air above or below a
// heading.
func TestAndroidCompactPageKeepsItsAir(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	i := strings.Index(java, "private static void applyParagraphAir(")
	if i < 0 {
		t.Fatal("BtBridge.java has no applyParagraphAir")
	}
	body := java[i:]
	body = body[:strings.Index(body, "\n    }\n")]
	if !strings.Contains(body, "if (!compact) continue;") || strings.Contains(body, "anyBlank") {
		t.Error("applyParagraphAir infers the page from blank lines rather than from the import")
	}
	if !strings.Contains(java, "applyParagraphAir((android.text.SpannableStringBuilder) s, text, lastMeasureDp > 0f);") {
		t.Error("setHtml does not tell applyParagraphAir which page it imported")
	}
	for _, want := range []string{"fn[k] == 2 ? fnRule : fnEntry", "h = fn[k - 1] == 1 ? fnRule : fnEntry;"} {
		if !strings.Contains(body, want) {
			t.Errorf("applyParagraphAir does not give the footnote section its air (%q)", want)
		}
	}
}
