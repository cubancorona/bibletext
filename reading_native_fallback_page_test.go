package bibletext

// THE PLAIN-TEXT FALLBACK'S PAGE, run from the source. When a chapter's HTML
// will not import, the Mac and the iPhone and iPad show that HTML with its
// tags stripped (bibleTextMacPlainFromHTML, bibleTextPlainFromHTML), and a
// share from that page reaches Go with no span: the selected text, and
// nothing to say where it stood. Whatever the strip makes of the page is
// therefore all the share code has. The host cannot run either cgo
// preamble, so these tests run each converter's own steps, read from its
// source, on the HTML buildChapterHTML writes.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// appleFallbackConverters are the two converters, by file and signature.
var appleFallbackConverters = []struct{ path, signature string }{
	{"reading_macos.go", "static NSString *bibleTextMacPlainFromHTML(NSString *html) {"},
	{"reading_ios.go", "static NSString *bibleTextPlainFromHTML(NSString *html) {"},
}

const objcLiteral = `@"((?:[^"\\]|\\.)*)"`

var (
	objcReplaceLiteral = regexp.MustCompile(`^(\w+) = \[(\w+) stringByReplacingOccurrencesOfString:` + objcLiteral + ` withString:` + objcLiteral + ` options:NSLiteralSearch range:NSMakeRange\(0, (\w+)\.length\)\]$`)
	objcRegex          = regexp.MustCompile(`^NSRegularExpression \*(\w+) = \[NSRegularExpression regularExpressionWithPattern:` + objcLiteral + ` options:0 error:nil\]$`)
	objcReplaceRegex   = regexp.MustCompile(`^(?:NSString \*)?(\w+) = \[(\w+) stringByReplacingMatchesInString:(\w+) options:0 range:NSMakeRange\(0, (\w+)\.length\) withTemplate:` + objcLiteral + `\]$`)
	objcReturn         = regexp.MustCompile(`^return (\w+)$`)
)

// objcStatements splits a function's source into its statements, with the
// comments and the line breaks taken out, and string literals kept whole: an
// entity such as "&nbsp;" carries the semicolon that ends a statement.
func objcStatements(t *testing.T, src string) []string {
	t.Helper()
	open := strings.IndexByte(src, '{')
	if open < 0 {
		t.Fatal("no function body")
	}
	src = src[open+1:]
	var out []string
	var b strings.Builder
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inString:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(src) {
				i++
				b.WriteByte(src[i])
			} else if c == '"' {
				inString = false
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
		case c == '"':
			inString = true
			b.WriteByte(c)
		case c == ';':
			out = append(out, strings.Join(strings.Fields(b.String()), " "))
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" && rest != "}" {
		t.Fatalf("the function ends in something that is not a statement: %q", rest)
	}
	return out
}

// objcString is the value of a string literal's contents as the compiler
// reads them.
func objcString(t *testing.T, lit string) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < len(lit); i++ {
		if lit[i] != '\\' {
			b.WriteByte(lit[i])
			continue
		}
		i++
		switch lit[i] {
		case '\\', '"':
			b.WriteByte(lit[i])
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		default:
			t.Fatalf("an escape this test does not read: \\%c", lit[i])
		}
	}
	return b.String()
}

// runAppleFallbackConverter runs a converter's statements on html. It reads
// the three kinds the converters are written in, a literal replacement, a
// regular expression and its replacement, and the return, and fails on any
// other, so a step added in another form cannot be passed over unseen. A
// replacement is read only with NSLiteralSearch over the whole string, which
// compares code units as strings.ReplaceAll compares bytes; without it the
// compare matches only whole composed characters, and an entity followed by
// a combining mark is left standing where ReplaceAll would replace it. The
// expressions are ones ICU and Go read alike.
func runAppleFallbackConverter(t *testing.T, body, html string) string {
	t.Helper()
	vars := map[string]string{"html": html}
	res := map[string]*regexp.Regexp{}
	for _, st := range objcStatements(t, body) {
		if m := objcReplaceLiteral.FindStringSubmatch(st); m != nil {
			v, ok := vars[m[2]]
			if !ok || m[1] != m[2] || m[5] != m[2] {
				t.Fatalf("a replacement this test does not read: %s", st)
			}
			vars[m[1]] = strings.ReplaceAll(v, objcString(t, m[3]), objcString(t, m[4]))
			continue
		}
		if m := objcRegex.FindStringSubmatch(st); m != nil {
			res[m[1]] = regexp.MustCompile(objcString(t, m[2]))
			continue
		}
		if m := objcReplaceRegex.FindStringSubmatch(st); m != nil {
			re, ok := res[m[2]]
			v, have := vars[m[3]]
			if !ok || !have || m[3] != m[4] || strings.Contains(m[5], "$") {
				t.Fatalf("a replacement this test does not read: %s", st)
			}
			vars[m[1]] = re.ReplaceAllLiteralString(v, objcString(t, m[5]))
			continue
		}
		if m := objcReturn.FindStringSubmatch(st); m != nil {
			v, ok := vars[m[1]]
			if !ok {
				t.Fatalf("returns what it never set: %s", st)
			}
			return v
		}
		t.Fatalf("a step this test cannot run: %s", st)
	}
	t.Fatal("the converter returns nothing")
	return ""
}

// fallbackCut is where both panes cut the chapter's HTML before the strip:
// the opening of the translators' notes.
const fallbackCut = `<p class="fnsep">`

// fallbackPageHTML is the HTML the fallback strips: the chapter's page, cut
// at the translators' notes, as both panes cut it before the strip
// (TestTheFallbackSetsTheConvertersPageOfTheCutHTML holds the panes to it).
func fallbackPageHTML(st *AppState) string {
	html := buildChapterHTML(st, st.Bible.GetChapter(st.CurrentBook, st.CurrentChapter))
	if i := strings.Index(html, fallbackCut); i >= 0 {
		html = html[:i]
	}
	return html
}

// appleFallbackCallSites are the two panes' fallbacks, by file, function and
// the statement that sets the page.
var appleFallbackCallSites = []struct{ path, fn, assign string }{
	{"reading_macos.go", "void bibleTextMacTVSetHTML(const char *html) {", "[gTextView setString:bibleTextMacPlainFromHTML(plainSrc)]"},
	{"reading_ios.go", "void bibleTextTVSetHTML(const char *html) {", "gReadingTV.text = bibleTextPlainFromHTML(plainSrc)"},
}

// objcTextWrite matches a statement that writes a text view's characters.
var objcTextWrite = regexp.MustCompile(`\.text\s*=[^=]|\.attributedText\s*=[^=]|\bset(?:String|AttributedString|Text|AttributedText):|\b(?:replace|delete)CharactersInRange:|\b(?:insert|append)AttributedString:`)

// objcBlockEnd is the offset of the brace that closes the block holding
// offset from in src, or -1. String literals and line comments are passed
// over.
func objcBlockEnd(src string, from int) int {
	depth := 0
	for i := from; i < len(src); i++ {
		switch c := src[i]; {
		case c == '"':
			for i++; i < len(src) && src[i] != '"'; i++ {
				if src[i] == '\\' {
					i++
				}
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '{':
			depth++
		case c == '}':
			if depth == 0 {
				return i
			}
			depth--
		}
	}
	return -1
}

// fallbackCallSiteError checks a pane's fallback block, from the copy of
// the HTML it cuts to the end of the block: it opens with exactly the cut
// fallbackPageHTML models and the statement that sets the page to the
// converter's output, and nothing else in it writes the text view's text.
func fallbackCallSiteError(t *testing.T, body, assign string) error {
	t.Helper()
	const open = "NSString *plainSrc = s;"
	if strings.Count(body, open) != 1 {
		return fmt.Errorf("%q is not in the function once", open)
	}
	i := strings.Index(body, open)
	end := objcBlockEnd(body, i)
	if end < 0 {
		return fmt.Errorf("the block holding the fallback does not close")
	}
	sts := objcStatements(t, "{"+body[i:end])
	want := []string{
		"NSString *plainSrc = s",
		`NSRange fnsep = [plainSrc rangeOfString:@"` + strings.ReplaceAll(fallbackCut, `"`, `\"`) + `"]`,
		"if (fnsep.location != NSNotFound) plainSrc = [plainSrc substringToIndex:fnsep.location]",
		assign,
	}
	if len(sts) < len(want) {
		return fmt.Errorf("the fallback block has %d statements, want at least %d", len(sts), len(want))
	}
	for k, w := range want {
		if sts[k] != w {
			return fmt.Errorf("statement %d of the fallback is %q, want %q", k+1, sts[k], w)
		}
	}
	writes := 0
	for _, st := range sts {
		if objcTextWrite.MatchString(st) {
			writes++
		}
	}
	if writes != 1 {
		return fmt.Errorf("the fallback block writes the text %d times, want once", writes)
	}
	return nil
}

// fallbackPsalmTitlePage is psalmTitleState's chapter as a converter writes
// it, with where each verse's number and text stand, found and checked
// against the chapter's prose.
func fallbackPsalmTitlePage(t *testing.T, st *AppState, body string, withHeading bool) psalmTitlePage {
	t.Helper()
	page := runAppleFallbackConverter(t, body, fallbackPageHTML(st))
	title := collapseSpaces(st.Bible.SuperscriptionFor(st.CurrentBook, st.CurrentChapter).DrawnText())
	if !strings.HasPrefix(page, title+"\n") {
		t.Fatalf("the page does not open with the title on a line of its own: %q", page[:min(len(page), 120)])
	}
	pos := len(title) + 1
	if withHeading {
		if !strings.HasPrefix(page[pos:], psalmHeading+"\n") {
			t.Fatalf("the heading is not on a line of its own under the title: %q", page[pos:min(len(page), pos+80)])
		}
		pos += len(psalmHeading) + 1
	}
	corpus, spans := chapterProse(st)
	p := psalmTitlePage{page: page, titleEnd: len(title), number: make([]int, len(spans)), text: make([]int, len(spans))}
	for i, sp := range spans {
		num := strconv.Itoa(sp.verse) + " "
		if !strings.HasPrefix(page[pos:], num) {
			t.Fatalf("verse %d's number is not where its text begins: %q", sp.verse, page[pos:min(len(page), pos+40)])
		}
		p.number[i], p.text[i] = pos, pos+len(num)
		end := p.text[i] + sp.end - sp.start
		if end > len(page) || strings.ReplaceAll(page[p.text[i]:end], "\n", " ") != corpus[sp.start:sp.end] {
			t.Fatalf("the page does not hold verse %d's text after its number", sp.verse)
		}
		pos = end
		if pos < len(page) && (page[pos] == ' ' || page[pos] == '\n') {
			pos++
		}
	}
	return p
}

// The fallback's page is the chapter's text and nothing else, a paragraph to
// a line: the title, a heading, and the verses each stand apart, so verse
// 1's number arrives as a word of its own, and no rule of the stylesheet or
// entity of the markup is left in it, while an entity the text itself spells
// out stands as spelled. It began with the stylesheet, and ran the title, the
// heading and verse 1's number together ("his son.A Morning Prayer1
// Yahweh"); with &amp; read before &lt; and &gt;, the text's own "&lt;b&gt;"
// became "<b>". The controls take the converter's steps out, or move one, and
// show each is what the check sees.
func TestTheFallbackPageIsTheChaptersTextAParagraphToALine(t *testing.T) {
	const (
		spelled  = `Its marks are spelled &quot;, &lt;b&gt;, &nbsp; and &amp;.`
		paraStep = `html = [html stringByReplacingOccurrencesOfString:@"</p>" withString:@"</p>\n" options:NSLiteralSearch range:NSMakeRange(0, html.length)];`
		quotStep = `t = [t stringByReplacingOccurrencesOfString:@"&quot;" withString:@"\"" options:NSLiteralSearch range:NSMakeRange(0, t.length)];`
		nbspStep = `t = [t stringByReplacingOccurrencesOfString:@"&nbsp;" withString:@" " options:NSLiteralSearch range:NSMakeRange(0, t.length)];`
		ampStep  = `t = [t stringByReplacingOccurrencesOfString:@"&amp;" withString:@"&" options:NSLiteralSearch range:NSMakeRange(0, t.length)];`
	)
	st := psalmTitleState(true)
	st.Bible.Verses["Psalms"][3][2].Text = `But you, Yahweh, are a "shield" around me, my glory & my <help>. ` + spelled
	_, spans := chapterProse(st)
	for _, c := range appleFallbackConverters {
		body := nativeFunctionSource(t, c.path, c.signature)
		page := runAppleFallbackConverter(t, body, fallbackPageHTML(st))
		title := collapseSpaces(st.Bible.SuperscriptionFor("Psalms", 3).DrawnText())
		want := title + "\n" + psalmHeading + "\n1 "
		if !strings.HasPrefix(page, want) {
			t.Errorf("%s: the page opens %q, want %q", c.path, page[:min(len(page), len(want)+20)], want)
		}
		if !strings.Contains(page, `my glory & my <help>.`) || !strings.Contains(page, `"shield"`) || strings.ContainsAny(page, "{}") {
			t.Errorf("%s: the page carries markup or the stylesheet: %q", c.path, page)
		}
		if !strings.Contains(page, spelled) {
			t.Errorf("%s: the text's own spelling of an entity is not on the page as spelled: %q", c.path, page)
		}
		if err := fallbackVerseNumbers(page, len(spans)); err != nil {
			t.Errorf("%s: %v", c.path, err)
		}

		// CONTROL: without the paragraph step the title runs into the
		// heading; without the head's removal the stylesheet opens the page;
		// without the quotation mark's entity the mark is not on the page;
		// with &amp; read first the text's own spelling is read twice.
		for _, step := range []struct{ line, what string }{
			{paraStep, "the paragraph step"},
			{`html = [head stringByReplacingMatchesInString:html options:0`, "the head's removal"},
			{quotStep, "the quotation mark's entity"},
			{nbspStep, "the first entity"},
			{ampStep, "the ampersand's entity"},
		} {
			if !strings.Contains(body, step.line) {
				t.Fatalf("%s: %s is not in the converter as this test knows it", c.path, step.what)
			}
		}
		without := strings.Replace(body, paraStep, "", 1)
		if p := runAppleFallbackConverter(t, without, fallbackPageHTML(st)); strings.HasPrefix(p, title+"\n") {
			t.Fatalf("%s: the title stands on its own line with the paragraph step taken out; the check proves nothing", c.path)
		}
		i := strings.Index(body, `NSRegularExpression *head =`)
		j := strings.Index(body, `NSRegularExpression *re =`)
		if i < 0 || j < i {
			t.Fatalf("%s: the head's removal is not where this test knows it", c.path)
		}
		if p := runAppleFallbackConverter(t, body[:i]+body[j:], fallbackPageHTML(st)); strings.HasPrefix(p, title) || !strings.ContainsAny(p, "{}") {
			t.Fatalf("%s: the page opens with the title with the head's removal taken out; the check proves nothing", c.path)
		}
		without = strings.Replace(body, quotStep, "", 1)
		if p := runAppleFallbackConverter(t, without, fallbackPageHTML(st)); strings.Contains(p, `"shield"`) {
			t.Fatalf("%s: the quotation marks are on the page with their entity left alone; the check proves nothing", c.path)
		}
		ampFirst := strings.Replace(strings.Replace(body, ampStep, "", 1), nbspStep, ampStep+"\n"+nbspStep, 1)
		if p := runAppleFallbackConverter(t, ampFirst, fallbackPageHTML(st)); strings.Contains(p, spelled) {
			t.Fatalf("%s: the text's own spelling stands with &amp; read first; the check proves nothing", c.path)
		}
	}
}

// fallbackVerseNumbers checks that each of the first n verse numbers stands
// on the page as a word of its own, after a space or a line break, and says
// which does not.
func fallbackVerseNumbers(page string, n int) error {
	pos := 0
	for v := 1; v <= n; v++ {
		num := strconv.Itoa(v) + " "
		i := strings.Index(page[pos:], num)
		if i < 0 {
			return fmt.Errorf("verse %d's number is not on the page as a word", v)
		}
		i += pos
		if i > 0 && page[i-1] != ' ' && page[i-1] != '\n' {
			return fmt.Errorf("verse %d's number is joined to what stands before it: %q", v, page[max(0, i-20):i+len(num)])
		}
		pos = i + len(num)
	}
	return nil
}

// A drag from a psalm's title into the psalm on the fallback's page, where
// nothing has a span, is answered as TestPsalmTitleDragSweep answers it on
// the page the panes draw: the psalm's words alone, cited by their verses,
// and the title never quoted. The page ran the title into verse 1's number,
// which could then not confirm the title, so every such drag was shared
// whole, the title's words as Scripture under the chapter.
func TestTheFallbackPageAnswersADragFromATitleAsThePanesWould(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range appleFallbackConverters {
		body := nativeFunctionSource(t, c.path, c.signature)
		for _, withHeading := range []bool{false, true} {
			st := psalmTitleState(withHeading)
			p := fallbackPsalmTitlePage(t, st, body, withHeading)
			if seen[p.page] {
				continue // the other pane's converter wrote the same page
			}
			seen[p.page] = true
			name := fmt.Sprintf("%s, heading=%v", c.path, withHeading)
			drags, copies := sweepPsalmTitleDrags(t, st, p, func(int) selSpan { return selSpan{} }, name)
			t.Logf("%s: %d drags, %d of them inside the title and quoted from the psalm", name, drags, copies)
		}
	}
}

// The page the converter tests run on is the page the panes set: each pane's
// fallback cuts the chapter's HTML where fallbackPageHTML does, sets the text
// view to the converter's output for what is left, and writes the text
// nowhere else in the block. The converter tests follow the converter alone,
// so a change at the call site could otherwise change the page unseen. The
// controls invert the cut and write the text a second time.
func TestTheFallbackSetsTheConvertersPageOfTheCutHTML(t *testing.T) {
	for _, c := range appleFallbackCallSites {
		body := nativeFunctionSource(t, c.path, c.fn)
		if err := fallbackCallSiteError(t, body, c.assign); err != nil {
			t.Errorf("%s: %v", c.path, err)
		}
		inverted := strings.Replace(body, "fnsep.location != NSNotFound", "fnsep.location == NSNotFound", 1)
		if inverted == body || fallbackCallSiteError(t, inverted, c.assign) == nil {
			t.Fatalf("%s: the check passes with the cut inverted; it proves nothing", c.path)
		}
		twice := strings.Replace(body, c.assign+";", c.assign+"; "+c.assign+";", 1)
		if twice == body || fallbackCallSiteError(t, twice, c.assign) == nil {
			t.Fatalf("%s: the check passes with the text written twice; it proves nothing", c.path)
		}
	}
}
