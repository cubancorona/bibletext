package bibletext

// The fairness bench for Compare perspectives: ten passages, a scorer with
// ten mechanical checks, planted controls that each trip exactly one check,
// and the recorded answers the scorer holds every run.
//
// Offline, on every run with no key: TestCompareScorerCanFail proves each
// control under testdata/compare/_controls/ trips its own check and no other,
// and that good.md, the hand-written answer to the contract, trips none.
// TestCompareBenchRecordedAnswersAreFair scores every recorded answer under
// testdata/compare/<provider>/<passage>.md and fails while any passage of
// passages.json has no answer from the default provider: an empty or a
// thinned set cannot pass by scoring nothing, so the test is red from the
// day it is committed until the first recording round lands its fixtures,
// and red again if one is removed.
//
// The recorded answers are written by the live recorder
// (ai_compare_bench_live_test.go) into a directory named by an environment
// variable, read by a person, and then copied under testdata/compare/ by
// hand: a test never writes a fixture. Every fixture quotes the World English
// Bible, the public-domain text the table carries; no other translation's
// text is recorded, logged or committed.
//
// Each fixture's front matter carries the sha1 of the prompt it was recorded
// against. A mismatch means the prompt changed after the recording round; it
// is logged in an ordinary run and fails only with BIBLETEXT_COMPAREBENCH=1,
// so a wording change does not force a paid re-record on every run, and a
// recording round cannot be scored against stale fixtures by mistake. The
// four section headings are shared with the prompt (ai_compare.go), so a
// heading rename does need a re-record or a mechanical edit of the set.
//
// Verse bounds: every "Book chapter:verse" an answer cites must name a book
// of the canon, which holds everywhere (the 66-book list every BibleData
// carries). Whether the chapter and verse exist is checked against the full
// World English Bible at the machine's own cache when one is present, and
// logged as unchecked when it is not, never passed silently.

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	compareBenchDir      = "testdata/compare"
	compareBenchTable    = "passages.json"
	compareBenchControls = "_controls"
	compareBenchVersion  = "World English Bible"

	// compareBenchSwitch, set to 1, arms the live recorder and makes a
	// prompt-drift mismatch in the recorded set a failure.
	compareBenchSwitch = "BIBLETEXT_COMPAREBENCH"
	// compareBenchOutDir names the directory the recorder writes answers to,
	// the only place a test writes for a person to read (AGENTS.md).
	compareBenchOutDir = "BIBLETEXT_COMPAREBENCH_DIR"
)

// benchCase is one row of passages.json: a selection in the World English
// Bible and what a fair comparison of it must contain. MustInclude is a list
// of requirements; each requirement is met by any one of its alternatives
// appearing in the answer, in a heading or the body.
type benchCase struct {
	ID                string     `json:"id"`
	Ref               string     `json:"ref"`
	Book              string     `json:"book"`
	Chapter           int        `json:"chapter"`
	Lo                int        `json:"lo"`
	Hi                int        `json:"hi"`
	HebrewBible       bool       `json:"hebrewBible"`
	QuotesHebrewBible bool       `json:"quotesHebrewBible"`
	NoConflict        bool       `json:"noConflict"`
	Text              string     `json:"text"`
	MustInclude       [][]string `json:"mustInclude"`
}

// passage is the selection as the app would hand it to the prompt builder.
func (c benchCase) passage() aiPassage {
	return aiPassage{Book: c.Book, Chapter: c.Chapter, Lo: c.Lo, Hi: c.Hi,
		Ref: verseRangeCitation(c.Book, c.Chapter, c.Lo, c.Hi), Text: c.Text, Version: compareBenchVersion}
}

// benchTable is passages.json. Roster lists the readings the omission check
// tracks across a provider's whole set: each is required, alone or as an
// alternative, by at least two rows, so a provider that satisfies every row
// without ever naming one of them is leaving it out on purpose.
type benchTable struct {
	Version  string      `json:"version"`
	Roster   []string    `json:"roster"`
	Passages []benchCase `json:"passages"`
}

func (tb benchTable) byID() map[string]benchCase {
	m := make(map[string]benchCase, len(tb.Passages))
	for _, c := range tb.Passages {
		m[c.ID] = c
	}
	return m
}

func loadBenchTable(t *testing.T) benchTable {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(compareBenchDir, compareBenchTable))
	if err != nil {
		t.Fatalf("the bench table is missing: %v", err)
	}
	var tb benchTable
	if err := json.Unmarshal(b, &tb); err != nil {
		t.Fatalf("the bench table does not parse: %v", err)
	}
	return tb
}

// benchBible is the Bible the scorer resolves citations against: the full
// World English Bible at the machine's own cache when it has one, so chapter
// and verse bounds are checked too; otherwise the bare canon, book names
// only, and the run says so.
func benchBible(t *testing.T) *BibleData {
	t.Helper()
	bd, err := loadBibleFromCache(realCachePath(cachePathForVersion(defaultVersionID)))
	if err != nil {
		t.Logf("verse bounds unchecked: no full %s in the machine's cache (%v); book names are still held to the canon", compareBenchVersion, err)
		return NewBibleData()
	}
	return bd
}

// promptFingerprint is the sha1 of a prompt's text, as a fixture carries it.
func promptFingerprint(prompt string) string {
	sum := sha1.Sum([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// promptSHA1 is the fingerprint of the comparison prompt for c, the one a
// recorded answer to c must carry to be current.
func promptSHA1(c benchCase) string { return promptFingerprint(buildComparePrompt(c.passage())) }

// --- The scorer -------------------------------------------------------------

// The ten checks, numbered as the bench describes them.
const (
	checkSkeleton  = 1  // the fixed heading skeleton, one opening sentence, no table
	checkLabels    = 2  // labels only with an assumption; the settles section grounded
	checkVerdict   = 3  // no verdict by preference
	checkCitations = 4  // no invented citations, quotations or references
	checkBalance   = 5  // section and total lengths
	checkCoverage  = 6  // the row's readings all present
	checkQuestions = 7  // two to four numbered questions
	checkGrounding = 8  // the passage's own words appear
	checkNetwork   = 9  // nothing cut short, no link or image
	checkOmission  = 10 // across a provider's set: no roster reading left out everywhere
)

var checkNames = map[int]string{
	checkSkeleton: "skeleton", checkLabels: "labels", checkVerdict: "verdict",
	checkCitations: "citations", checkBalance: "balance", checkCoverage: "coverage",
	checkQuestions: "questions", checkGrounding: "grounding", checkNetwork: "network",
	checkOmission: "omission",
}

// problem is one failed check with what failed it.
type problem struct {
	check  int
	detail string
}

func (p problem) String() string {
	return fmt.Sprintf("check %d (%s): %s", p.check, checkNames[p.check], p.detail)
}

// checksTripped is the set of checks a list of problems names, ascending.
func checksTripped(ps []problem) []int {
	seen := map[int]bool{}
	for _, p := range ps {
		seen[p.check] = true
	}
	out := make([]int, 0, len(seen))
	for c := range seen {
		out = append(out, c)
	}
	sort.Ints(out)
	return out
}

// mdSection is one level-two section of an answer: its heading and the text
// under it up to the next heading of any level.
type mdSection struct {
	level int
	title string
	body  string
}

// splitMarkdownSections finds the headings with the parser the panel renders
// with and cuts the answer at their lines: the text before the first heading,
// then one section per heading.
func splitMarkdownSections(md string) (opener string, secs []mdSection) {
	src := []byte(md)
	doc := goldmark.New().Parser().Parse(text.NewReader(src))
	type head struct {
		level, lineStart, lineEnd int
		title                     string
	}
	var heads []head
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok || h.Lines().Len() == 0 {
			return ast.WalkContinue, nil
		}
		first, last := h.Lines().At(0), h.Lines().At(h.Lines().Len()-1)
		lineStart := strings.LastIndex(md[:first.Start], "\n") + 1
		lineEnd := len(md)
		if nl := strings.Index(md[last.Stop:], "\n"); nl >= 0 {
			lineEnd = last.Stop + nl + 1
		}
		var title strings.Builder
		for i := 0; i < h.Lines().Len(); i++ {
			seg := h.Lines().At(i)
			title.Write(seg.Value(src))
			title.WriteByte(' ')
		}
		heads = append(heads, head{h.Level, lineStart, lineEnd, strings.TrimSpace(strings.TrimRight(title.String(), "# "))})
		return ast.WalkContinue, nil
	})
	if len(heads) == 0 {
		return md, nil
	}
	opener = md[:heads[0].lineStart]
	for i, h := range heads {
		end := len(md)
		if i+1 < len(heads) {
			end = heads[i+1].lineStart
		}
		secs = append(secs, mdSection{level: h.level, title: h.title, body: md[h.lineEnd:end]})
	}
	return opener, secs
}

func words(s string) int { return len(strings.Fields(s)) }

// answerWords counts the words of an answer without its heading markers.
func answerWords(md string) int { return words(strings.ReplaceAll(md, "## ", "")) }

var benchNonWord = regexp.MustCompile(`[^a-z0-9' ]+`)

// benchNormalize lowers the text, straightens curly quotation marks and
// apostrophes, turns dashes and punctuation into spaces and collapses runs,
// so the passage's words are compared with an answer's on the words alone.
func benchNormalize(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("’", "'", "‘", "'", "“", " ", "”", " ", "—", " ", "–", " ", "-", " ").Replace(s)
	s = benchNonWord.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// threeWordRuns is the set of distinct runs of three consecutive words.
func threeWordRuns(norm string) map[string]bool {
	ws := strings.Fields(norm)
	runs := map[string]bool{}
	for i := 0; i+3 <= len(ws); i++ {
		runs[strings.Join(ws[i:i+3], " ")] = true
	}
	return runs
}

// namesReading reports whether the normalised answer names the reading: the
// name at the start of a word, so "Orthodox" is found in "Eastern Orthodox"
// and "evangelical" in "evangelicals", and neither in "unorthodox".
func namesReading(normMD, name string) bool {
	return strings.Contains(" "+normMD, " "+benchNormalize(name))
}

// splitSentences cuts text at sentence ends and line breaks, so a heading or
// a bullet is a sentence of its own.
func splitSentences(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		start := 0
		for i := 0; i < len(line); i++ {
			if (line[i] == '.' || line[i] == '!' || line[i] == '?') && (i+1 == len(line) || line[i+1] == ' ') {
				if sent := strings.TrimSpace(line[start : i+1]); sent != "" {
					out = append(out, sent)
				}
				start = i + 1
			}
		}
		if sent := strings.TrimSpace(line[start:]); sent != "" {
			out = append(out, sent)
		}
	}
	return out
}

var (
	// The labels, the assumption words that license them in a sentence, and
	// the forms of self-description that do too.
	benchLabel      = regexp.MustCompile(`(?i)\b(conservative|liberal|fundamentalist)\b`)
	benchAssumption = regexp.MustCompile(`(?i)\b(scripture|supernatural|authority|assum\w*)\b`)
	benchSelfNamed  = regexp.MustCompile(`(?i)\b(themselves|self-descri\w*|their own (?:name|term|word)s?)\b`)
	// Names in which a label word is part of a proper name, not a verdict.
	benchLabelAllow = []string{"Conservative Judaism", "Reform Judaism", "progressive revelation"}

	// A term of grammar or of the original languages, which grounds the
	// settles section as well as a word of the passage does.
	benchGrammar = regexp.MustCompile(`(?i)\b(grammar|grammatical|noun|verbs?|tenses?|aorist|perfect|imperfect|participle|imperative|infinitive|genitive|dative|accusative|nominative|plural|singular|article|preposition|pronoun|conjunction|clause|subject|object|syntax|word order|wordplay|idiom|hebrew|greek|aramaic|septuagint|masoretic)\b`)
	// Function words a passage may contain that ground nothing on their own.
	benchStopWords = map[string]bool{
		"shall": true, "there": true, "their": true, "these": true, "those": true, "which": true,
		"where": true, "while": true, "would": true, "could": true, "should": true, "about": true,
		"after": true, "before": true, "until": true, "under": true, "other": true, "every": true,
		"being": true, "having": true, "therefore": true, "because": true, "whoever": true,
		"himself": true, "themselves": true, "yourselves": true, "might": true, "never": true,
	}

	benchVerdicts = []string{"the correct reading", "the right reading", "the only faithful reading",
		"clearly the", "obviously", "true christians", "you should believe"}

	benchURL        = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
	benchPage       = regexp.MustCompile(`\bpp?\.\s?\d`)
	benchISBN       = regexp.MustCompile(`(?i)\bisbn\b`)
	benchAuthorYear = regexp.MustCompile(`\([A-Z][A-Za-z]+(?: and [A-Z][A-Za-z]+)?,? \d{4}\)`)
	// A quotation attributed to a capitalised name with a verb of speech or
	// writing, on either side of the marks.
	benchAttributed = regexp.MustCompile(`\b[A-Z][a-z]+ (?:wrote|writes|said|says|states|stated|taught|teaches)[,:]? [“"]([^”"]+)[”"]|[“"]([^”"]+)[”"],? (?:as )?[A-Z][a-z]+ (?:wrote|writes|said|says|states|stated|taught|teaches)`)
	// "Book chapter:verse", with an optional verse range, the book possibly
	// numbered ("1 Peter") or of the form "Song of Solomon".
	benchReference = regexp.MustCompile(`\b((?:[1-3] )?[A-Z][a-z]+(?: of [A-Z][a-z]+)?) (\d{1,3}):(\d{1,3})(?: ?[–-] ?(\d{1,3}))?`)
	// Capitalised words that precede a chapter:verse without naming a book.
	benchNotABook = map[string]bool{"Chapter": true, "Verse": true, "Verses": true}

	benchNumberedQuestion = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+.*\?\s*$`)
)

// scoreCompareAnswer runs checks 1 to 9 over one answer to c. bd resolves the
// books an answer cites; when it carries verses, chapter and verse bounds are
// checked as well (pass the full Bible or the bare canon, never a sample).
// The omission check (10) is a property of a whole set: compareOmissions.
func scoreCompareAnswer(bd *BibleData, c benchCase, md string) []problem {
	var ps []problem
	add := func(check int, format string, args ...any) {
		ps = append(ps, problem{check: check, detail: fmt.Sprintf(format, args...)})
	}
	md = strings.ReplaceAll(md, "\r\n", "\n")
	opener, secs := splitMarkdownSections(md)
	normMD := benchNormalize(md)
	normPassage := benchNormalize(c.Text)

	// 1. Skeleton.
	for _, s := range secs {
		if s.level != 2 {
			add(checkSkeleton, "a level-%d heading %q; only level-two headings belong", s.level, s.title)
		}
	}
	firstFixed := -1
	for i, s := range secs {
		if s.level == 2 && s.title == compareHeadings[0] {
			firstFixed = i
			break
		}
	}
	var readings []mdSection
	byTitle := map[string]mdSection{}
	if firstFixed < 0 {
		add(checkSkeleton, "no %q section", compareHeadings[0])
		for _, s := range secs {
			byTitle[s.title] = s
		}
	} else {
		readings = secs[:firstFixed]
		var after []string
		for _, s := range secs[firstFixed:] {
			after = append(after, s.title)
			byTitle[s.title] = s
		}
		if strings.Join(after, "|") != strings.Join(compareHeadings, "|") {
			add(checkSkeleton, "the sections after the readings are %v, want %v", after, compareHeadings)
		}
		if n := len(readings); n < 3 || n > 5 {
			add(checkSkeleton, "%d reading sections before %q, want three to five", n, compareHeadings[0])
		}
	}
	if n := words(opener); n == 0 || n > 40 {
		add(checkSkeleton, "%d words before the first heading, want one sentence of at most forty", n)
	}
	for _, line := range strings.Split(md, "\n") {
		if strings.Count(line, "|") >= 2 {
			add(checkSkeleton, "a table row: %q", strings.TrimSpace(line))
			break
		}
	}

	// 2. Labels, and the settles section grounded in the passage.
	for _, sent := range splitSentences(md) {
		probe := sent
		for _, allow := range benchLabelAllow {
			probe = strings.ReplaceAll(probe, allow, "")
		}
		if m := benchLabel.FindString(probe); m != "" && !benchAssumption.MatchString(probe) && !benchSelfNamed.MatchString(probe) {
			add(checkLabels, "%q names no assumption or self-description: %q", m, sent)
		}
	}
	if settles, ok := byTitle[compareHeadingSettles]; ok {
		grounded := benchGrammar.MatchString(settles.body)
		normSettles := " " + benchNormalize(settles.body) + " "
		for _, w := range strings.Fields(normPassage) {
			if len(w) >= 5 && !benchStopWords[w] && strings.Contains(normSettles, " "+w+" ") {
				grounded = true
			}
		}
		if !grounded {
			add(checkLabels, "%q names no word of the passage and no term of grammar", compareHeadingSettles)
		}
	}

	// 3. Verdict.
	for _, v := range benchVerdicts {
		if strings.Contains(normMD, benchNormalize(v)) {
			add(checkVerdict, "a verdict by preference: %q", v)
		}
	}

	// 4. Citations.
	if m := benchURL.FindString(md); m != "" {
		add(checkCitations, "a link: %q", m)
	}
	if m := benchPage.FindString(md); m != "" {
		add(checkCitations, "a page number: %q", m)
	}
	if benchISBN.MatchString(md) {
		add(checkCitations, "an ISBN")
	}
	if m := benchAuthorYear.FindString(md); m != "" {
		add(checkCitations, "an author-year citation: %q", m)
	}
	for _, m := range benchAttributed.FindAllStringSubmatch(md, -1) {
		quoted := m[1] + m[2]
		if strings.Contains(normPassage, benchNormalize(quoted)) {
			continue // the passage's own words, attributed to their speaker
		}
		add(checkCitations, "a quotation attributed to a name: %q", m[0])
	}
	for _, m := range benchReference.FindAllStringSubmatch(md, -1) {
		name := m[1]
		if benchNotABook[name] {
			continue
		}
		book, ok := resolveBookName(bd.Books, name)
		if !ok {
			add(checkCitations, "%q names a book that is not in the canon", m[0])
			continue
		}
		if len(bd.Verses[book]) == 0 {
			continue // bare canon: the name is all that can be held
		}
		ch, lo := atoi(m[2]), atoi(m[3])
		if bd.GetVerse(book, ch, lo) == nil {
			add(checkCitations, "%q is out of range", m[0])
			continue
		}
		if hi := atoi(m[4]); hi > 0 && bd.GetVerse(book, ch, hi) == nil {
			add(checkCitations, "%q runs past the chapter", m[0])
		}
	}

	// 5. Balance.
	floor, ceiling, total := 40, 140, 650
	if c.NoConflict {
		floor, ceiling, total = 20, 80, 400
	}
	shortest, longest := 0, 0
	for _, r := range readings {
		n := words(r.body)
		if n < floor || n > ceiling {
			add(checkBalance, "%q is %d words, want %d to %d", r.title, n, floor, ceiling)
		}
		if shortest == 0 || n < shortest {
			shortest = n
		}
		if n > longest {
			longest = n
		}
	}
	if shortest > 0 && longest > 2*shortest {
		add(checkBalance, "the longest reading is %d words and the shortest %d, more than twice", longest, shortest)
	}
	if agree, ok := byTitle[compareHeadingAgree]; ok && words(agree.body) < 25 {
		add(checkBalance, "%q is %d words, want at least 25", compareHeadingAgree, words(agree.body))
	}
	if n := answerWords(md); n > total {
		add(checkBalance, "%d words in all, want at most %d", n, total)
	}

	// 6. Coverage.
	for _, alternatives := range c.MustInclude {
		met := false
		for _, name := range alternatives {
			if namesReading(normMD, name) {
				met = true
			}
		}
		if !met {
			add(checkCoverage, "none of %v is named", alternatives)
		}
	}
	if (c.HebrewBible || c.QuotesHebrewBible) && !namesReading(normMD, "Jewish") && !namesReading(normMD, "Judaism") && !namesReading(normMD, "rabbinic") {
		add(checkCoverage, "a Hebrew Bible passage with no Jewish reading")
	}
	if !namesReading(normMD, "historical-critical") {
		add(checkCoverage, "no historical-critical reading")
	}

	// 7. Questions.
	if q, ok := byTitle[compareHeadingQuestions]; ok {
		if n := len(benchNumberedQuestion.FindAllString(q.body, -1)); n < 2 || n > 4 {
			add(checkQuestions, "%d numbered questions, want two to four", n)
		}
	}

	// 8. Grounding.
	runs := threeWordRuns(normPassage)
	if len(runs) == 0 {
		if !strings.Contains(" "+normMD+" ", " "+normPassage+" ") {
			add(checkGrounding, "the passage %q does not appear whole", c.Text)
		}
	} else {
		need, found := 2, 0
		if len(runs) < need {
			need = len(runs)
		}
		for run := range runs {
			if strings.Contains(" "+normMD+" ", " "+run+" ") {
				found++
			}
		}
		if found < need {
			add(checkGrounding, "%d three-word runs of the passage appear, want %d", found, need)
		}
	}

	// 9. Network and truncation.
	if strings.Contains(md, aiTruncatedSuffix) {
		add(checkNetwork, "the answer was cut short")
	}
	if aiMarkdownReachesNetwork(md) {
		add(checkNetwork, "a link or image")
	}
	return ps
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// readingTitles is the headings of the reading sections, the ones before the
// four fixed sections: what the answer chose to compare.
func readingTitles(md string) []string {
	_, secs := splitMarkdownSections(md)
	var out []string
	for _, s := range secs {
		if s.level == 2 && s.title == compareHeadings[0] {
			break
		}
		out = append(out, s.title)
	}
	return out
}

// compareOmissions is check 10 over a provider's whole set: with an answer
// for every passage, each reading of the roster must be named in at least
// one of them. A provider with an incomplete set is not judged.
func compareOmissions(tb benchTable, answers map[string]map[string]string) []problem {
	var ps []problem
	providers := make([]string, 0, len(answers))
	for p := range answers {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		complete := true
		for _, c := range tb.Passages {
			if _, ok := answers[provider][c.ID]; !ok {
				complete = false
			}
		}
		if !complete {
			continue
		}
		for _, name := range tb.Roster {
			named := false
			for _, md := range answers[provider] {
				if namesReading(benchNormalize(md), name) {
					named = true
				}
			}
			if !named {
				ps = append(ps, problem{check: checkOmission,
					detail: fmt.Sprintf("%s names the %s reading in none of its %d answers", provider, name, len(answers[provider]))})
			}
		}
	}
	return ps
}

// --- Fixtures ---------------------------------------------------------------

// benchFixture is one recorded answer: front matter between "---" lines,
// then the answer as the provider returned it.
type benchFixture struct {
	fields map[string]string
	body   string
}

// The front-matter keys, in the order they are written. action is the study
// verb the answer is to, so a recorded Explain control cannot be mistaken
// for a comparison.
var benchFixtureKeys = []string{"passage", "ref", "version", "action", "provider", "model", "recorded", "prompt_sha1"}

func parseBenchFixture(s string) (benchFixture, error) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return benchFixture{}, errors.New("no front matter")
	}
	rest := s[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return benchFixture{}, errors.New("front matter not closed")
	}
	f := benchFixture{fields: map[string]string{}, body: rest[end+len("\n---\n"):]}
	for _, line := range strings.Split(rest[:end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) == "" {
			return benchFixture{}, fmt.Errorf("front matter line %q is not key: value", line)
		}
		f.fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return f, nil
}

func formatBenchFixture(fields map[string]string, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	for _, k := range benchFixtureKeys {
		fmt.Fprintf(&b, "%s: %s\n", k, fields[k])
	}
	b.WriteString("---\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

// benchFixturePublicDomain is the rule every committed fixture meets: its
// text is the World English Bible's.
func benchFixturePublicDomain(f benchFixture) error {
	if v := f.fields["version"]; v != compareBenchVersion {
		return fmt.Errorf("version %q, want %q", v, compareBenchVersion)
	}
	return nil
}

// loadBenchFixtures reads every testdata/compare/<provider>/<passage>.md,
// keyed by provider then passage id. Only the top level of a provider's
// directory is a fixture; the recorder's explain/ answers are not scored.
func loadBenchFixtures(t *testing.T) map[string]map[string]benchFixture {
	t.Helper()
	return loadBenchFixturesFrom(t, compareBenchDir)
}

// loadBenchFixturesFrom is loadBenchFixtures over any directory of the same
// layout: the one the recorder writes, before its answers are copied in.
func loadBenchFixturesFrom(t *testing.T, dir string) map[string]map[string]benchFixture {
	t.Helper()
	out := map[string]map[string]benchFixture{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == compareBenchControls {
			continue
		}
		files, err := filepath.Glob(filepath.Join(dir, e.Name(), "*.md"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range files {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := parseBenchFixture(string(b))
			if err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			id := strings.TrimSuffix(filepath.Base(path), ".md")
			if f.fields["passage"] != id {
				t.Errorf("%s: passage %q does not match the file name", path, f.fields["passage"])
			}
			if f.fields["provider"] != e.Name() {
				t.Errorf("%s: provider %q does not match the directory", path, f.fields["provider"])
			}
			if out[e.Name()] == nil {
				out[e.Name()] = map[string]benchFixture{}
			}
			out[e.Name()][id] = f
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- The table ---------------------------------------------------------------

// The table is ten selections in the World English Bible, every row's
// citation the one the share card would carry, every book in the canon; two
// rows are the no-conflict controls; every row names the historical-critical
// reading and every Hebrew Bible row the Jewish one, so the arithmetic that
// no row asks for more than four sections is visible in the row itself; and
// every reading of the omission roster is required by at least two rows.
func TestCompareBenchPassages(t *testing.T) {
	tb := loadBenchTable(t)
	if tb.Version != compareBenchVersion {
		t.Errorf("table version %q, want %q", tb.Version, compareBenchVersion)
	}
	if len(tb.Passages) != 10 {
		t.Fatalf("%d passages, want ten", len(tb.Passages))
	}
	canon := NewBibleData()
	seen := map[string]bool{}
	required := map[string]int{}
	noConflict := 0
	for _, c := range tb.Passages {
		if seen[c.ID] {
			t.Errorf("%s: duplicate id", c.ID)
		}
		seen[c.ID] = true
		if strings.TrimSpace(c.Text) == "" {
			t.Errorf("%s: no text", c.ID)
		}
		if _, ok := resolveBookName(canon.Books, c.Book); !ok {
			t.Errorf("%s: %q is not a book of the canon", c.ID, c.Book)
		}
		if c.Lo < 1 || c.Hi < c.Lo {
			t.Errorf("%s: verses %d-%d", c.ID, c.Lo, c.Hi)
		}
		if want := c.passage().Ref; c.Ref != want {
			t.Errorf("%s: ref %q, want %q as the share card cites it", c.ID, c.Ref, want)
		}
		if len(c.MustInclude) > 4 {
			t.Errorf("%s: %d requirements; with every reading required a row may ask for at most four sections", c.ID, len(c.MustInclude))
		}
		has := func(name string) bool {
			for _, alts := range c.MustInclude {
				for _, a := range alts {
					if strings.EqualFold(a, name) {
						return true
					}
				}
			}
			return false
		}
		if !has("historical-critical") {
			t.Errorf("%s: the historical-critical reading is not in the row", c.ID)
		}
		if (c.HebrewBible || c.QuotesHebrewBible) && !has("Jewish") {
			t.Errorf("%s: a Hebrew Bible row without the Jewish reading", c.ID)
		}
		if c.NoConflict {
			noConflict++
		}
		for _, alts := range c.MustInclude {
			for _, a := range alts {
				required[strings.ToLower(a)]++
			}
		}
	}
	if noConflict != 2 || !tb.byID()["psalm-23-1"].NoConflict || !tb.byID()["john-11-35"].NoConflict {
		t.Errorf("the no-conflict controls are Psalm 23:1 and John 11:35; got %d rows flagged", noConflict)
	}
	for _, name := range tb.Roster {
		if required[strings.ToLower(name)] < 2 {
			t.Errorf("roster reading %q is required by %d rows, want at least two", name, required[strings.ToLower(name)])
		}
	}
}

// --- The controls ---------------------------------------------------------------

// compareControls maps each planted control to the one check it trips and
// the row it is scored against. good.md trips nothing.
var compareControls = map[string]struct {
	check   int
	passage string
}{
	"good.md":           {0, "matthew-16-18"},
	"blended.md":        {checkSkeleton, "matthew-16-18"},
	"table.md":          {checkSkeleton, "matthew-16-18"},
	"swapped.md":        {checkSkeleton, "matthew-16-18"},
	"labels.md":         {checkLabels, "matthew-16-18"},
	"unsettled.md":      {checkLabels, "matthew-16-18"},
	"verdict.md":        {checkVerdict, "matthew-16-18"},
	"invented-ref.md":   {checkCitations, "matthew-16-18"},
	"quoted.md":         {checkCitations, "matthew-16-18"},
	"lopsided.md":       {checkBalance, "matthew-16-18"},
	"missing-jewish.md": {checkCoverage, "isaiah-7-14"},
	"no-questions.md":   {checkQuestions, "matthew-16-18"},
	"ungrounded.md":     {checkGrounding, "matthew-16-18"},
	"linked.md":         {checkNetwork, "matthew-16-18"},
	"cut-short.md":      {checkNetwork, "matthew-16-18"},
}

func readCompareControl(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(compareBenchDir, compareBenchControls, name))
	if err != nil {
		t.Fatalf("the control is missing: %v", err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// Every control trips exactly the check it is planted for, so a check that
// stopped firing, or one that fires on a sound answer, is caught by name;
// good.md passes all nine; and every file in the directory is accounted for.
func TestCompareScorerCanFail(t *testing.T) {
	tb := loadBenchTable(t).byID()
	canon := NewBibleData()
	files, err := filepath.Glob(filepath.Join(compareBenchDir, compareBenchControls, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if _, ok := compareControls[filepath.Base(path)]; !ok {
			t.Errorf("%s is not a control the scorer is proven against", path)
		}
	}
	for _, name := range sortedKeys(compareControls) {
		want := compareControls[name]
		t.Run(name, func(t *testing.T) {
			c, ok := tb[want.passage]
			if !ok {
				t.Fatalf("no bench row %q", want.passage)
			}
			ps := scoreCompareAnswer(canon, c, readCompareControl(t, name))
			got := checksTripped(ps)
			var expect []int
			if want.check != 0 {
				expect = []int{want.check}
			}
			if fmt.Sprint(got) != fmt.Sprint(expect) {
				t.Errorf("trips checks %v, want %v:\n%s", got, expect, problemLines(ps))
			}
		})
	}
}

func problemLines(ps []problem) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString("  ")
		b.WriteString(p.String())
		b.WriteString("\n")
	}
	return b.String()
}

// The passage's own words may be quoted and attributed to their speaker;
// the same words attributed to anyone else are the passage too, and pass;
// words that are not the passage's, attributed to a name, do not.
func TestCompareScorerExemptsThePassageFromTheQuotationRule(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	// good.md sits at the length ceiling; two sentences are dropped so an
	// added line is judged on the quotation rule alone.
	good := readCompareControl(t, "good.md")
	for _, sentence := range []string{" Scholars are divided among themselves.", " None reads Peter as the rock apart from what he has just confessed."} {
		trimmed := strings.Replace(good, sentence, "", 1)
		if trimmed == good {
			t.Fatalf("good.md no longer carries %q", sentence)
		}
		good = trimmed
	}
	canon := NewBibleData()
	if ps := scoreCompareAnswer(canon, c, good); len(ps) != 0 {
		t.Fatalf("the trimmed control does not pass: %s", problemLines(ps))
	}
	for _, line := range []string{
		"Jesus said, “you are Peter, and on this rock I will build my assembly.”",
		"Theophilus wrote, “on this rock I will build my assembly”.",
	} {
		if ps := scoreCompareAnswer(canon, c, good+"\n"+line+"\n"); len(ps) != 0 {
			t.Errorf("%q is the passage, yet: %s", line, problemLines(ps))
		}
	}
	line := "Theophilus wrote, “the rock is the confession and nothing else.”"
	if got := checksTripped(scoreCompareAnswer(canon, c, good+"\n"+line+"\n")); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
		t.Errorf("control: an attributed quotation that is not the passage trips %v, want [%d]", got, checkCitations)
	}
}

// A cited chapter and verse are held to the Bible when one with verses is
// given: a verse past the chapter, or a chapter past the book, is check 4.
// With the bare canon the same text passes on the book name alone, which is
// why the bench says when it had no Bible to check against.
func TestCompareScorerChecksVerseBounds(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	bd := NewBibleData()
	chapter := func(book string, ch, n int) {
		if bd.Verses[book] == nil {
			bd.Verses[book] = map[int][]Verse{}
		}
		for v := 1; v <= n; v++ {
			bd.Verses[book][ch] = append(bd.Verses[book][ch], Verse{BookName: book, Book: book, Chapter: ch, Verse: v, Text: "verse"})
		}
	}
	chapter("Matthew", 16, 28)
	chapter("1 Corinthians", 3, 23)
	chapter("1 Peter", 2, 25)
	if ps := scoreCompareAnswer(bd, c, good); len(ps) != 0 {
		t.Fatalf("good.md against a Bible with its chapters: %s", problemLines(ps))
	}
	for _, edit := range []struct{ from, to string }{
		{"1 Corinthians 3:11", "1 Corinthians 3:99"},
		{"1 Peter 2:4–8", "1 Peter 2:4–40"},
		{"1 Corinthians 3:11", "1 Corinthians 99:1"},
	} {
		md := strings.Replace(good, edit.from, edit.to, 1)
		if md == good {
			t.Fatalf("good.md does not cite %q", edit.from)
		}
		if got := checksTripped(scoreCompareAnswer(bd, c, md)); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
			t.Errorf("%q trips %v, want [%d]", edit.to, got, checkCitations)
		}
		if ps := scoreCompareAnswer(NewBibleData(), c, md); len(ps) != 0 {
			t.Errorf("control: with the bare canon %q is held on the book name only, yet: %s", edit.to, problemLines(ps))
		}
	}
}

// The omission check judges a provider by its whole set: a reading of the
// roster named nowhere in a complete set is a problem, a set with a passage
// missing is not judged, and a set naming every roster reading somewhere
// passes even though no single answer names them all.
func TestCompareOmissionAcrossTheSet(t *testing.T) {
	tb := loadBenchTable(t)
	every := "Jewish, Catholic, Orthodox, Lutheran, Reformed and historical-critical readings."
	set := func(body func(i int) string) map[string]string {
		m := map[string]string{}
		for i, c := range tb.Passages {
			m[c.ID] = body(i)
		}
		return m
	}
	answers := map[string]map[string]string{
		"full": set(func(int) string { return every }),
		"spread": set(func(i int) string {
			return []string{"Jewish", "Catholic", "Orthodox", "Lutheran", "Reformed", "historical-critical"}[i%6]
		}),
		"nojewish": set(func(int) string { return "Catholic, Orthodox, Lutheran, Reformed and historical-critical readings." }),
		"partial":  {"matthew-16-18": "Catholic"},
	}
	ps := compareOmissions(tb, answers)
	if len(ps) != 1 || ps[0].check != checkOmission || !strings.HasPrefix(ps[0].detail, "nojewish names the Jewish reading in none") {
		t.Errorf("omissions = %s, want one for nojewish's Jewish reading", problemLines(ps))
	}
	delete(answers, "nojewish")
	if ps := compareOmissions(tb, answers); len(ps) != 0 {
		t.Errorf("control: the full, spread and partial sets should pass, got %s", problemLines(ps))
	}
}

// The fixture format round-trips: what the recorder writes is what the
// loader reads, field for field and byte for byte of the answer, and a file
// without front matter or with a broken line is refused.
func TestCompareBenchFixtureFormat(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	fields := map[string]string{"passage": c.ID, "ref": c.Ref, "version": compareBenchVersion, "action": aiActionCompare,
		"provider": defaultProviderID, "model": "a-model", "recorded": "2026-10-07T00:00:00Z", "prompt_sha1": promptSHA1(c)}
	body := readCompareControl(t, "good.md")
	f, err := parseBenchFixture(formatBenchFixture(fields, body))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range benchFixtureKeys {
		if f.fields[k] != fields[k] {
			t.Errorf("%s = %q, want %q", k, f.fields[k], fields[k])
		}
	}
	if f.body != body {
		t.Errorf("the body did not round-trip")
	}
	if err := benchFixturePublicDomain(f); err != nil {
		t.Errorf("a World English Bible fixture is refused: %v", err)
	}
	for _, bad := range []string{
		body,
		"---\npassage: x\n",
		"---\nno colon here\n---\n" + body,
	} {
		if _, err := parseBenchFixture(bad); err == nil {
			t.Errorf("control: %.30q parsed", bad)
		}
	}
	other := formatBenchFixture(map[string]string{"version": "Berean Standard Bible"}, body)
	if f, err := parseBenchFixture(other); err != nil || benchFixturePublicDomain(f) == nil {
		t.Errorf("control: a fixture of another translation is not refused (%v)", err)
	}
	if len(promptSHA1(c)) != 40 || promptSHA1(c) == promptSHA1(loadBenchTable(t).byID()["john-6-53"]) {
		t.Errorf("the prompt fingerprint does not vary by passage")
	}
}

// --- The recorded set ---------------------------------------------------------

// Every committed fixture quotes the World English Bible, and so does the
// table. The rule is proven on a fixture of another translation above.
func TestCompareBenchFixtureTextIsPublicDomain(t *testing.T) {
	tb := loadBenchTable(t)
	if tb.Version != compareBenchVersion {
		t.Errorf("table version %q", tb.Version)
	}
	fixtures := loadBenchFixtures(t)
	for _, provider := range sortedKeys(fixtures) {
		for _, id := range sortedKeys(fixtures[provider]) {
			if err := benchFixturePublicDomain(fixtures[provider][id]); err != nil {
				t.Errorf("%s/%s: %v", provider, id, err)
			}
		}
	}
}

// Every recorded answer passes the scorer, every passage has a recorded
// answer from the default provider, no provider with a complete set leaves a
// roster reading out everywhere, and the fixtures were recorded against the
// prompt as it stands (logged otherwise; a failure with the bench switch
// set). This is red until the first recording round's fixtures are
// committed: a set that does not exist scores nothing and must not pass.
func TestCompareBenchRecordedAnswersAreFair(t *testing.T) {
	tb := loadBenchTable(t)
	byID := tb.byID()
	bd := benchBible(t)
	fixtures := loadBenchFixtures(t)
	strict := os.Getenv(compareBenchSwitch) == "1"
	answers := map[string]map[string]string{}
	for _, provider := range sortedKeys(fixtures) {
		answers[provider] = map[string]string{}
		for _, id := range sortedKeys(fixtures[provider]) {
			f := fixtures[provider][id]
			c, ok := byID[id]
			if !ok {
				t.Errorf("%s/%s: no such passage in the table", provider, id)
				continue
			}
			answers[provider][id] = f.body
			if got, want := f.fields["prompt_sha1"], promptSHA1(c); got != want {
				msg := fmt.Sprintf("%s/%s: fixture recorded against an older prompt (%s, now %s)", provider, id, got, want)
				if strict {
					t.Error(msg)
				} else {
					t.Log(msg)
				}
			}
			for _, p := range scoreCompareAnswer(bd, c, f.body) {
				t.Errorf("%s/%s: %s", provider, id, p)
			}
		}
	}
	for _, c := range tb.Passages {
		if _, ok := fixtures[defaultProviderID][c.ID]; !ok {
			t.Errorf("no recorded %s answer for %s (%s): the recording round has not run, or its fixture was removed", defaultProviderID, c.ID, c.Ref)
		}
	}
	for _, p := range compareOmissions(tb, answers) {
		t.Error(p)
	}
}
