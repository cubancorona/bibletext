package bibletext

// THE iOS CHAPTER WASH IS A VIEW BENEATH THE TEXT, held by parsing the source.
//
// UIKit (17+) draws the selection highlight in a view BELOW the text content,
// while TextKit fills NSBackgroundColorAttributeName inside the content view
// above it. A chapter wash written as an opaque attribute therefore hid the
// selection completely: a reader long-pressing a word inside a search hit saw
// the handles and the edit menu come up over a band that stayed flat amber.
// reading_ios.go now draws the wash as BTWashView at subview index 0 and lifts
// the imported .hl fill off the string on import, and only the narration
// writes the attribute — a translucent colour the wash shows through.
//
// The pane is Objective-C inside a cgo preamble, so like
// reading_ios_menu_guard_test.go this holds the contract by parsing the
// source: the wash can never quietly return to the attribute, the view can
// never quietly climb above the highlight, and the premise the import-time
// strip rests on — that the Apple stylesheet emits no background but the tint
// table's — is checked against the stylesheet actually emitted.

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestIOSChapterWashIsAnUnderlayBeneathTheSelection(t *testing.T) {
	src := readNativeSource(t, "reading_ios.go")

	// 1. The view sits beneath UIKit's selection container, cannot take a
	//    touch, and never animates a wash change.
	for frag, why := range map[string]string{
		"insertSubview:gWashView atIndex:0": "the wash view must be inserted at index 0 — beneath the view " +
			"UIKit draws the selection highlight in; anywhere above it the wash hides the selection again",
		"gWashView.userInteractionEnabled = NO": "a full-content view that takes touches would sit in front " +
			"of nothing but still swallow the pane's taps and selection gestures",
		"[CATransaction setDisableActions:YES]": "a CAShapeLayer path or fill change implicitly animates for " +
			"0.25s; without this every wash arrival cross-fades and 'Clear highlight' fades out",
		"[mas removeAttribute:NSBackgroundColorAttributeName range:NSMakeRange(0, mas.length)]": "the " +
			"imported .hl fill must be lifted off the string before assignment, or the opaque attribute is " +
			"back above the selection highlight on every full render",
	} {
		if !strings.Contains(src, frag) {
			t.Errorf("reading_ios.go no longer contains %q — %s", frag, why)
		}
	}

	// 2. The z-order is DEFENDED, in layoutSubviews, not merely established
	//    once: UIKit installs its selection container lazily.
	i := strings.Index(src, "- (void)layoutSubviews {")
	if i < 0 {
		t.Fatal("HBReadingTextView no longer overrides layoutSubviews — the wash view's z-order is undefended")
	}
	body := src[i:]
	if j := strings.Index(body, "\n}\n"); j >= 0 {
		body = body[:j]
	}
	if !strings.Contains(body, "sendSubviewToBack:gWashView") {
		t.Error("layoutSubviews no longer sends the wash view to the back — a future iOS that inserts a " +
			"sibling at index 0 would put the wash above the selection highlight again")
	}

	// 3. EXACTLY ONE writer of the background attribute, and it writes the
	//    narration colour. The chapter wash reaching the attribute by any route
	//    is the defect returning.
	writers := 0
	for _, line := range strings.Split(src, "\n") {
		if !strings.Contains(line, "NSBackgroundColorAttributeName") {
			continue
		}
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "//") || strings.Contains(code, "removeAttribute:") {
			continue // a comment, or a removal
		}
		// Every spelling that writes an attribute: addAttribute:, and the
		// dictionary forms (addAttributes: / setAttributes: with the name as
		// a key) that the narrower check used to let through.
		if !strings.Contains(code, "addAttribute:") && !strings.Contains(code, "NSBackgroundColorAttributeName:") {
			t.Errorf("NSBackgroundColorAttributeName is referenced in a way this guard does not classify:\n  %s\n"+
				"teach the guard the new spelling before shipping it", code)
			continue
		}
		writers++
		if !strings.Contains(line, "btIOSReadAlongColor()") {
			t.Errorf("a background attribute is written with something other than the narration colour:\n  %s\n"+
				"the chapter wash must stay in BTWashView, beneath the selection highlight", strings.TrimSpace(line))
		}
	}
	if writers != 1 {
		t.Errorf("reading_ios.go writes NSBackgroundColorAttributeName at %d sites, want exactly 1 (the narration)", writers)
	}

	// 4. The attribute-route machinery stays gone.
	for _, banned := range []string{
		"btIOSOverlayColor",
		"btIOSRepaintChapterWashFromModel",
		"btIOSPaintRunWash",
	} {
		if strings.Contains(src, banned) {
			t.Errorf("reading_ios.go contains %s — the arithmetic-composite / attribute-repaint route is back; "+
				"the compositor does that source-over now and the wash must not return to the storage", banned)
		}
	}
}

// The import-time strip removes EVERY background attribute from the imported
// string. That is only right while the Apple stylesheet emits no background
// but the tint table's (.hl, .hlm — appleTintHTML, tint.go): the day someone
// styles the footnote section or the superscription with a background, iOS
// would drop it silently. So the emitted stylesheet is checked, with the
// footnote section and the Psalm title both rendering, in both layouts.
func TestAppleStylesheetEmitsBackgroundsOnlyFromTheTintTable(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	verses := footnoteFixtureVerses()
	st := superFixtureState(verses)
	setFootnotesEnabled(true)
	defer setFootnotesEnabled(false)
	st.setHL(hlSearch, st.CurrentBook, st.CurrentChapter, 16, 16)

	for _, reporter := range []bool{false, true} {
		var html string
		withReporterLayout(reporter, func() { html = buildChapterHTML(st, verses) })
		// The premise needs both sections on the page, or the check is vacuous.
		if !strings.Contains(html, `class="fnsep"`) || !strings.Contains(html, `class="pst"`) {
			t.Fatalf("reporter=%v: the fixture no longer renders the footnote section and the superscription:\n%s",
				reporter, html)
		}
		css := appleStylesheetOf(t, html)
		bad, ok := backgroundsOutsideTheTintTable(css, st.pal())
		if ok == 0 {
			t.Errorf("reporter=%v: the stylesheet carries no tint-table background rule at all — the wash is not being emitted",
				reporter)
		}
		for _, rule := range bad {
			t.Errorf("reporter=%v: the Apple stylesheet emits a background outside the tint table:\n  %s\n"+
				"bibleTextApplyHTML strips every background attribute on import, so iOS would drop this one silently",
				reporter, rule)
		}
		// And the check can fail: a bare background on the footnote rule is caught.
		if bad, _ := backgroundsOutsideTheTintTable(css+`p.fn { background-color: #ffffff; }`, st.pal()); len(bad) != 1 {
			t.Fatalf("reporter=%v: the control background was not caught (%d flagged) — the check cannot fail",
				reporter, len(bad))
		}
	}
}

func appleStylesheetOf(t *testing.T, html string) string {
	t.Helper()
	i, j := strings.Index(html, "<style>"), strings.Index(html, "</style>")
	if i < 0 || j < i {
		t.Fatalf("no <style> block in the chapter HTML:\n%s", html)
	}
	return html[i+len("<style>") : j]
}

// backgroundsOutsideTheTintTable returns every rule in css that mentions a
// background and is not, verbatim, one of appleTintHTML's rules formatted with
// its wash — plus how many of the table's rules it found.
func backgroundsOutsideTheTintTable(css string, pal palette) (bad []string, ok int) {
	allowed := map[string]bool{}
	for tint := verseTint(0); tint < tintCount; tint++ {
		rule := appleTintHTML[tint].CSS
		c, washed := tint.wash(pal)
		if rule == "" || !washed {
			continue
		}
		allowed[strings.TrimSpace(fmt.Sprintf(rule, nrgbaToHex(c)))] = true
	}
	rest := css
	for {
		k := strings.Index(rest, "background")
		if k < 0 {
			return bad, ok
		}
		start := strings.LastIndex(rest[:k], "}") + 1
		end := strings.Index(rest[k:], "}")
		if end < 0 {
			end = len(rest) - k
		}
		rule := strings.TrimSpace(rest[start : k+end+1])
		if allowed[rule] {
			ok++
		} else {
			bad = append(bad, rule)
		}
		rest = rest[k+end:]
		if rest == "" {
			return bad, ok
		}
		rest = rest[1:]
	}
}
