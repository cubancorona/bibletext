//go:build spacingaudit

package bibletext

// SPACING AUDIT — fetch every edition fresh through the app's own decoders and
// hold every surface that assembles verse text to the words the edition sent.
//
// Tagged so it never runs in CI: it needs the network, and the NKJV half needs
// the licensed key. Snapshots are written OUTSIDE the repository, because the
// NKJV is a licensed text and must not be committed in any form.
//
//	BIBLE_API_KEY="$(security find-generic-password -a release -s uk.co.bibletext.apibible-release -w)" \
//	SPACING_AUDIT_DIR=/path/outside/the/repo \
//	go test -tags spacingaudit -run TestSpacingAudit -v -timeout 60m .
//
// SPACING_AUDIT_REFETCH=1 discards existing snapshots and downloads again.

import (
	"encoding/json"
	"fmt"
	"html"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func spacingAuditDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("SPACING_AUDIT_DIR")
	if dir == "" {
		t.Skip("SPACING_AUDIT_DIR is not set")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Refuse a directory inside the repository: the NKJV snapshot is licensed
	// text, and a stray `git add -A` must not be able to reach it.
	if root, err := filepath.Abs(repoRoot(t)); err == nil {
		if rel, err := filepath.Rel(root, abs); err == nil && !filepath.IsAbs(rel) && rel != ".." &&
			len(rel) >= 2 && rel[:2] != ".." {
			t.Fatalf("SPACING_AUDIT_DIR %s is inside the repository; the NKJV must not be written there", abs)
		}
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		t.Fatal(err)
	}
	return abs
}

// spacingAuditEditions is every edition the app ships text for.
var spacingAuditEditions = []string{"web", "bsb", "webc", "nkjv"}

func spacingAuditFetch(t *testing.T, id string) (*BibleData, error) {
	t.Helper()
	if id == "nkjv" {
		key := os.Getenv("BIBLE_API_KEY")
		if key == "" {
			t.Skip("BIBLE_API_KEY is not set; the NKJV cannot be fetched")
		}
		return fetchAPIBible("NKJV", nkjvProviderBibleID, key)
	}
	v, ok := versionByID(id)
	if !ok {
		t.Fatalf("unknown edition %q", id)
	}
	return v.source.fetch()
}

// loadSpacingAuditEdition returns the snapshot for id, fetching it fresh when
// it is missing or SPACING_AUDIT_REFETCH is set.
func loadSpacingAuditEdition(t *testing.T, dir, id string) *BibleData {
	t.Helper()
	path := filepath.Join(dir, id+".json")
	if os.Getenv("SPACING_AUDIT_REFETCH") == "" {
		if raw, err := os.ReadFile(path); err == nil {
			var bd BibleData
			if err := json.Unmarshal(raw, &bd); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			return &bd
		}
	}
	start := time.Now()
	bd, err := spacingAuditFetch(t, id)
	if err != nil {
		t.Fatalf("fetch %s: %v", id, err)
	}
	raw, err := json.Marshal(bd)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, chs := range bd.Verses {
		for _, vs := range chs {
			n += len(vs)
		}
	}
	t.Logf("%s: fetched fresh in %s — %d books, %d verses", id, time.Since(start).Round(time.Second), len(bd.Books), n)
	return bd
}

func TestSpacingAuditFetch(t *testing.T) {
	dir := spacingAuditDir(t)
	for _, id := range spacingAuditEditions {
		loadSpacingAuditEdition(t, dir, id)
	}
}

// ---------------------------------------------------------------- the lenses
//
// A lost space shows up as a token that is two (or three) adjacent source
// tokens run together — "Behold,I", "16For". So each chapter's source tokens
// are indexed along with every concatenation of 2 and 3 consecutive ones
// (across verse boundaries too, and with each verse number glued to its first
// word), and a surface's output is split on whitespace: any token that is not
// a source token but IS such a concatenation is a space that surface lost.
//
// Comparison is on a normal form — outboundText (small capitals to capitals,
// superscripts to digits, gap marks dropped) then lower case — so the drawn
// form and the stored form of the same word compare equal. A join that is
// already in the source (the upstream NKJV defects) is a source token and is
// deliberately NOT reported here; that is a different lens.

type spacingIndex struct {
	single map[string]bool
	joins  map[string]string // normalised concatenation -> "verse N" where it would occur
}

func spacingNorm(s string) string { return strings.ToLower(outboundText(s)) }

// spacingCore is a token with its surrounding punctuation and quotation marks
// removed. A join that is ALREADY in the source — the upstream NKJV defects,
// "“Theyhave" — must never be attributed to a surface, including when the
// surface presents it without the quotation mark the source token carried;
// the first version of this detector did exactly that.
func spacingCore(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) })
}

func buildSpacingIndex(verses []Verse) spacingIndex {
	idx := spacingIndex{single: map[string]bool{}, joins: map[string]string{}}
	type tok struct {
		t string
		v int
	}
	var seq []tok
	for _, v := range verses {
		fields := strings.Fields(v.Text)
		if len(fields) > 0 {
			idx.joins[spacingNorm(strconv.Itoa(v.Verse)+fields[0])] = fmt.Sprintf("verse %d (number glued)", v.Verse)
		}
		for _, f := range fields {
			seq = append(seq, tok{spacingNorm(f), v.Verse})
		}
	}
	for _, t := range seq {
		idx.single[t.t] = true
		idx.single[spacingCore(t.t)] = true
	}
	for i := range seq {
		if i+1 < len(seq) {
			j := seq[i].t + seq[i+1].t
			idx.joins[j] = fmt.Sprintf("verse %d", seq[i].v)
			idx.joins[spacingCore(j)] = fmt.Sprintf("verse %d", seq[i].v)
		}
		if i+2 < len(seq) {
			j := seq[i].t + seq[i+1].t + seq[i+2].t
			idx.joins[j] = fmt.Sprintf("verse %d", seq[i].v)
			idx.joins[spacingCore(j)] = fmt.Sprintf("verse %d", seq[i].v)
		}
	}
	return idx
}

// lostSpaces returns every token of out that is a join of source tokens.
func (idx spacingIndex) lostSpaces(out string) (found []string) {
	for _, f := range strings.Fields(out) {
		n := spacingNorm(f)
		if idx.single[n] || idx.single[spacingCore(n)] {
			continue
		}
		where, ok := idx.joins[n]
		if !ok {
			where, ok = idx.joins[spacingCore(n)]
		}
		if ok {
			found = append(found, where+": "+f)
		}
	}
	return found
}

// splitWords is the opposite defect: a source word drawn as two pieces with a
// break between them — Fyne's RichText, for one, wraps a segment's first
// word mid-word when it does not fit the rest of the line ("Bein" / "g
// therefore"). Two neighbouring output tokens whose join is a source word,
// and at least one of which is not a word of the source on its own.
func (idx spacingIndex) splitWords(out string) (found []string) {
	f := strings.Fields(out)
	for i := 0; i+1 < len(f); i++ {
		a, b := spacingNorm(f[i]), spacingNorm(f[i+1])
		joined := a + b
		if !idx.single[joined] && !idx.single[spacingCore(joined)] {
			continue
		}
		if idx.single[a] && idx.single[b] {
			continue
		}
		// Both halves must carry a letter: an ellipsis or a lone quotation
		// mark beside a word is framing, not half of a broken word.
		if spacingCore(a) == "" || spacingCore(b) == "" {
			continue
		}
		found = append(found, f[i]+" | "+f[i+1])
	}
	return found
}

// htmlToDrawnText renders builder HTML to the text a reader sees: block tags
// and <br> are line breaks, every inline tag is REMOVED WITH NOTHING — a tag
// boundary is not a space, so <span>but</span><span>you</span> reads as one
// word — superscript verse numbers are dropped, and entities are decoded.
var (
	reSup   = regexp.MustCompile(`(?is)<sup\b[^>]*>.*?</sup>`)
	reStyle = regexp.MustCompile(`(?is)<(style|script|head)\b[^>]*>.*?</(style|script|head)>`)
	reBlock = regexp.MustCompile(`(?i)<(br|/?p|/?div|/?h[1-6]|/?li|/?tr)\b[^>]*>`)
	reTag   = regexp.MustCompile(`<[^>]+>`)
)

func htmlToDrawnText(h string) string {
	h = reStyle.ReplaceAllString(h, "")
	h = reSup.ReplaceAllString(h, "")
	h = reBlock.ReplaceAllString(h, "\n")
	h = reTag.ReplaceAllString(h, "")
	return html.UnescapeString(h)
}

// renderedRichText lays a RichText out at width and reads back what it DRAWS:
// its canvas.Text objects grouped into rows, in x order, with a space wherever
// two neighbours are drawn apart and nothing where they touch. A segment
// boundary the widget swallows a space at shows up here and nowhere else.
func renderedRichText(segs []widget.RichTextSegment, width float32) string {
	rt := widget.NewRichText(segs...)
	rt.Wrapping = fyne.TextWrapWord
	rt.Resize(fyne.NewSize(width, 10000))
	r := test.WidgetRenderer(rt)
	r.Layout(fyne.NewSize(width, 10000))
	type obj struct {
		t    *canvas.Text
		x, y float32
	}
	var objs []obj
	var walk func(o fyne.CanvasObject, ox, oy float32)
	walk = func(o fyne.CanvasObject, ox, oy float32) {
		p := o.Position()
		switch v := o.(type) {
		case *canvas.Text:
			objs = append(objs, obj{v, ox + p.X, oy + p.Y})
		case *fyne.Container:
			for _, c := range v.Objects {
				walk(c, ox+p.X, oy+p.Y)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				walk(c, ox+p.X, oy+p.Y)
			}
		}
	}
	for _, o := range r.Objects() {
		walk(o, 0, 0)
	}
	// Rows are clustered with a tolerance rather than by exact y: a raised
	// superscript verse number sits a few pixels above its row, and grouping
	// by exact y split every numbered line around it -- which read as a lost
	// space in the first run of this audit.
	sort.SliceStable(objs, func(i, j int) bool { return objs[i].y < objs[j].y })
	row := make([]int, len(objs))
	for i := range objs {
		if i > 0 && objs[i].y-objs[i-1].y < objs[i].t.TextSize*0.6 {
			row[i] = row[i-1]
		} else if i > 0 {
			row[i] = row[i-1] + 1
		}
	}
	idx := make([]int, len(objs))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if row[idx[a]] != row[idx[b]] {
			return row[idx[a]] < row[idx[b]]
		}
		return objs[idx[a]].x < objs[idx[b]].x
	})
	sorted := make([]obj, len(objs))
	rows := make([]int, len(objs))
	for k, i := range idx {
		sorted[k], rows[k] = objs[i], row[i]
	}
	objs = sorted
	var b strings.Builder
	for i, o := range objs {
		if i > 0 {
			prev := objs[i-1]
			switch {
			case rows[i] != rows[i-1]:
				b.WriteByte('\n')
			default:
				w := fyne.MeasureText(prev.t.Text, prev.t.TextSize, prev.t.TextStyle).Width
				if o.x-(prev.x+w) > 1 {
					b.WriteByte(' ')
				}
			}
		}
		b.WriteString(o.t.Text)
	}
	return b.String()
}

// votdDrawnText is the verse-of-the-day card's paragraph as it lays out: the
// fragments it measures and places, a space where it places a gap.
func votdDrawnText(runs []cardRun) string {
	p := newReadingParagraph(runs, 16, color.Black, color.Black, nil, nil)
	var b strings.Builder
	for i, f := range p.fragments() {
		switch {
		case f.breakBefore && i > 0:
			b.WriteByte('\n')
		case f.spaceBefore && i > 0:
			b.WriteByte(' ')
		}
		b.WriteString(f.text)
	}
	return b.String()
}

func TestSpacingAuditSurfaces(t *testing.T) {
	dir := spacingAuditDir(t)
	app := test.NewApp()
	defer app.Quit()

	// The detector must be able to fail before a clean sweep means anything.
	ctrl := buildSpacingIndex([]Verse{{Verse: 20, Text: "Behold, I stand at the door and knock."}})
	if got := ctrl.lostSpaces("Behold,I stand at the door and knock."); len(got) != 1 {
		t.Fatalf("control: the detector did not see a lost space in %q: %v", "Behold,I …", got)
	}
	up := buildSpacingIndex([]Verse{{Verse: 5, Text: "“Theyhave corrupted themselves;"}, {Verse: 21, Text: "They have provoked Me"}})
	if got := up.lostSpaces("Theyhave corrupted themselves;"); len(got) != 0 {
		t.Fatalf("control: an upstream join shown without its quotation mark was blamed on the surface: %v", got)
	}
	sp := buildSpacingIndex([]Verse{{Verse: 33, Text: "Being therefore exalted"}})
	if got := sp.splitWords("Bein\ng therefore exalted"); len(got) != 1 {
		t.Fatalf("control: the detector did not see a word split across a line: %v", got)
	}
	if got := sp.splitWords("Being therefore exalted"); len(got) != 0 {
		t.Fatalf("control: an intact line was reported as split: %v", got)
	}
	// And a verse number glued to its first word is a lost space too.
	if got := ctrl.lostSpaces("20Behold, I stand"); len(got) != 1 {
		t.Fatalf("control: the detector did not see a number glued to its verse: %v", got)
	}

	surfaces := []string{"votd-red", "votd-black", "html-apple", "html-android", "mobile-segments",
		"mobile-segments-drawn", "search-card-drawn", "web-runs", "share-passage", "speech", "styled-tokens"}
	for _, id := range spacingAuditEditions {
		bd := loadSpacingAuditEdition(t, dir, id)
		counts := map[string]int{}
		examples := map[string][]string{}
		note := func(surface, where string) {
			counts[surface]++
			if len(examples[surface]) < 12 {
				examples[surface] = append(examples[surface], where)
			}
		}
		chapters, verses := 0, 0
		for _, book := range bd.Books {
			chs := make([]int, 0, len(bd.Verses[book]))
			for c := range bd.Verses[book] {
				chs = append(chs, c)
			}
			sort.Ints(chs)
			for _, ch := range chs {
				vs := bd.GetChapter(book, ch)
				if len(vs) == 0 {
					continue
				}
				chapters++
				verses += len(vs)
				idx := buildSpacingIndex(vs)
				st := &AppState{Bible: bd, CurrentBook: book, CurrentChapter: ch, CurrentVersion: id}
				at := func(surface string, out string) {
					for _, w := range idx.lostSpaces(out) {
						note(surface, fmt.Sprintf("%s %d %s", book, ch, w))
					}
					for _, w := range idx.splitWords(out) {
						note(surface+" SPLIT", fmt.Sprintf("%s %d %s", book, ch, w))
					}
				}

				at("html-apple", htmlToDrawnText(buildChapterHTML(st, vs)))
				at("html-android", htmlToDrawnText(buildChapterHTMLAndroid(st, vs)))
				segs := mobileParagraphSegments(st, vs)
				var concat strings.Builder
				for _, s := range segs {
					if ts, ok := s.(*widget.TextSegment); ok {
						concat.WriteString(outboundText(ts.Text))
					}
				}
				at("mobile-segments", concat.String())
				at("mobile-segments-drawn", outboundText(renderedRichText(segs, 360)))
				if q, _, ok := shareQuoteForPassage(st, book, ch, vs[0].Verse, vs[len(vs)-1].Verse); ok {
					at("share-passage", q)
				}
				at("speech", chapterSpeechText(st))

				for _, v := range vs {
					one := dayVerse{Book: book, Chapter: ch, Lo: v.Verse, Hi: v.Verse, Verses: []Verse{v}}
					at("votd-red", votdDrawnText(frameAndBalance(one.runs(id, true))))
					at("votd-black", votdDrawnText(frameAndBalance(one.runs(id, false))))
					var web strings.Builder
					for _, r := range RedLetterRuns(id, v) {
						web.WriteString(r.Text)
					}
					at("web-runs", web.String())
					at("styled-tokens", strings.Join(verseTokens(v), " "))

					// A search card highlights a term mid-verse, which puts
					// two segment boundaries inside the text.
					card := strings.Join(strings.Fields(v.Text), " ")
					if f := strings.Fields(card); len(f) >= 3 {
						term := strings.ToLower(strings.Trim(f[1], ".,;:!?‘’“”'\"()"))
						if term != "" {
							segs := termHighlightSegments(card, []string{term}, colorNameVerseText, colorNameHighlightHi)
							at("search-card-drawn", outboundText(renderedRichText(segs, 340)))
						}
					}
				}
			}
		}
		t.Logf("=== %s: %d chapters, %d verses", id, chapters, verses)
		for _, s := range surfaces {
			t.Logf("   %-22s %5d lost space(s)   %5d split word(s)", s, counts[s], counts[s+" SPLIT"])
			for _, e := range examples[s] {
				t.Logf("        %s", e)
			}
			for _, e := range examples[s+" SPLIT"] {
				t.Logf("        split: %s", e)
			}
		}
	}
}
