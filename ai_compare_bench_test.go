package bibletext

// The fairness bench for Compare perspectives: ten passages, a scorer with
// ten mechanical checks, planted controls that each trip exactly one check,
// and the recorded answers the scorer holds every run.
//
// The prompt names no reading, so the scorer asks for none by name, and the
// bench keeps no list of readers. A row asks only for what the prompt asks
// of such a passage: on one that reports something said or done, a reading
// that takes it as reported and one that takes it as a later composition or
// explains it naturally, told apart; on a Hebrew Bible passage, Scripture to
// Judaism as well as to the churches, a reading within Judaism, which is
// what the prompt's ask for each faith's reading as its own section comes
// to there. The one check judged across a provider's whole set, never on an
// answer, is that same reading on every Hebrew Bible row of the set.
//
// Quotations: a quotation an answer attributes to a speaker is held to the
// World English Bible chapter the passage is from, committed verse by verse
// in chapters.json so every machine reads the same chapter. Its words must
// be there, and its speaker must be named there or be the text's own voice,
// so the passage and its neighbours may be quoted in the mouth of the one
// who speaks them, and a writer, council, document or person the chapter
// does not name may not be quoted at all. The name a book is traditionally
// written under is its voice on that book alone: Paul may quote 1 Timothy
// where the chapter does not name him, and not Matthew.
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
	// compareBenchChapters is the World English Bible text of every chapter
	// the table's passages are drawn from, verse by verse: the chapter a
	// quotation in an answer is held to. It is committed, as the table's
	// passages are, so the scorer gives the same verdict on every machine;
	// where the machine's own cache carries the full World English Bible, the
	// committed chapters are held equal to it (TestCompareBenchPassages).
	compareBenchChapters = "chapters.json"

	// compareBenchSwitch, set to 1, arms the live recorder and makes a
	// prompt-drift mismatch in the recorded set a failure.
	compareBenchSwitch = "BIBLETEXT_COMPAREBENCH"
	// compareBenchOutDir names the directory the recorder writes answers to,
	// the only place a test writes for a person to read (AGENTS.md).
	compareBenchOutDir = "BIBLETEXT_COMPAREBENCH_DIR"
	// compareBenchOnly, set to passage ids separated by commas, narrows a
	// round to those passages, so a later round can re-record the rows a
	// reader held back without paying for the rest; unset, a round records
	// every passage of the table.
	compareBenchOnly = "BIBLETEXT_COMPAREBENCH_ONLY"

	// compareBenchRunawayWords is the one length the scorer holds, on a
	// comparison and on the Explain control alike. The prompt sets no word
	// count: it asks for succinctness and leaves the length to the answer's
	// substance, a number having been taken as a target that cost the
	// readings their breadth, and the scorer follows it, reporting the
	// words of an answer and of each reading rather than ruling on them. An
	// answer over this many words has stopped being succinct whatever its
	// substance, and trips the balance check as a runaway. The figure
	// stands well above the longest answer any recording round has
	// returned, and is not a target.
	compareBenchRunawayWords = 1000
)

// benchCase is one row of passages.json: a selection in the World English
// Bible and what a fair comparison of it must contain. The prompt names no
// reading, so a row names none either; it carries the two the prompt implies
// as properties of the passage. SaidOrDone marks a passage that reports
// something said or done, over which the prompt asks for both a reading that
// takes the report so and one that takes it as a later composition or
// explains it naturally, each told apart. HebrewBible and QuotesHebrewBible
// mark a passage whose first readers are the Jewish people, so a comparison
// of its believers' readings that leaves theirs out is incomplete; that is a
// fact about the passage, not a roster. A no-conflict row carries neither
// floor: the prompt asks such a passage to say in its first sentence that
// the readings largely agree, and to be brief, and the scorer reads the
// first sentence for it. chapter is the text of the chapter the passage is
// from, attached by loadBenchTable from chapters.json and never carried in
// the table itself.
type benchCase struct {
	ID                string `json:"id"`
	Ref               string `json:"ref"`
	Book              string `json:"book"`
	Chapter           int    `json:"chapter"`
	Lo                int    `json:"lo"`
	Hi                int    `json:"hi"`
	HebrewBible       bool   `json:"hebrewBible"`
	QuotesHebrewBible bool   `json:"quotesHebrewBible"`
	NoConflict        bool   `json:"noConflict"`
	SaidOrDone        bool   `json:"saidOrDone"`
	Text              string `json:"text"`

	chapter string
}

// benchChapterKey names the chapter a row is from as chapters.json keys it.
func (c benchCase) benchChapterKey() string { return fmt.Sprintf("%s %d", c.Book, c.Chapter) }

// owesJudaism reports whether the row is a Hebrew Bible passage, or quotes
// one, so that a reading within Judaism is owed its own section.
func (c benchCase) owesJudaism() bool { return c.HebrewBible || c.QuotesHebrewBible }

// passage is the selection as the app would hand it to the prompt builder.
func (c benchCase) passage() aiPassage {
	return aiPassage{Book: c.Book, Chapter: c.Chapter, Lo: c.Lo, Hi: c.Hi,
		Ref: verseRangeCitation(c.Book, c.Chapter, c.Lo, c.Hi), Text: c.Text, Version: compareBenchVersion}
}

// benchTable is passages.json: the version every row quotes, and the rows.
type benchTable struct {
	Version  string      `json:"version"`
	Passages []benchCase `json:"passages"`
}

func (tb benchTable) byID() map[string]benchCase {
	m := make(map[string]benchCase, len(tb.Passages))
	for _, c := range tb.Passages {
		m[c.ID] = c
	}
	return m
}

// loadBenchTable reads the table and gives every row the text of its chapter.
// A row whose chapter chapters.json does not carry is a table the scorer
// cannot hold quotations against, and fails here.
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
	chapters := loadBenchChapters(t)
	for i, c := range tb.Passages {
		vs, ok := chapters[c.benchChapterKey()]
		if !ok {
			t.Fatalf("%s: %s is not in %s", c.ID, c.benchChapterKey(), compareBenchChapters)
		}
		tb.Passages[i].chapter = benchChapterText(vs)
	}
	return tb
}

// benchVerse is one verse of chapters.json.
type benchVerse struct {
	Verse int    `json:"verse"`
	Text  string `json:"text"`
}

// loadBenchChapters reads chapters.json: the chapters by "Book chapter", each
// its verses in order.
func loadBenchChapters(t *testing.T) map[string][]benchVerse {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(compareBenchDir, compareBenchChapters))
	if err != nil {
		t.Fatalf("the bench chapters are missing: %v", err)
	}
	var f struct {
		Version  string                  `json:"version"`
		Chapters map[string][]benchVerse `json:"chapters"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("%s does not parse: %v", compareBenchChapters, err)
	}
	if f.Version != compareBenchVersion {
		t.Fatalf("%s is the %q, want the %s", compareBenchChapters, f.Version, compareBenchVersion)
	}
	return f.Chapters
}

// benchChapterText is a chapter's verses as one text, a line each.
func benchChapterText(vs []benchVerse) string {
	lines := make([]string, 0, len(vs))
	for _, v := range vs {
		lines = append(lines, v.Text)
	}
	return strings.Join(lines, "\n")
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
	checkSkeleton  = 1  // the fixed heading skeleton, at most forty words before it, two to seven readings, no table
	checkLabels    = 2  // labels only with an assumption; the settles section grounded
	checkVerdict   = 3  // no verdict by preference
	checkCitations = 4  // no invented citations, quotations or references
	checkBalance   = 5  // the whole answer's shape: no runaway, Where they agree not empty, the agree sentence first on a no-conflict row
	checkCoverage  = 6  // the readings the prompt implies for the row, told apart
	checkQuestions = 7  // two to four numbered questions, one asking what the reader brought
	checkGrounding = 8  // the passage's own words appear
	checkNetwork   = 9  // nothing cut short, no link or image
	checkOmission  = 10 // across a provider's complete set: the reading within Judaism on every Hebrew Bible row
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

var (
	benchNonWord = regexp.MustCompile(`[^a-z0-9' ]+`)
	// benchApostrophes straightens the curly apostrophes and single
	// quotation marks providers write (God’s, don’t), so a pattern written
	// with a straight apostrophe reads them.
	benchApostrophes = strings.NewReplacer("’", "'", "‘", "'")
)

// benchStraighten is the text with its apostrophes straightened and nothing
// else changed: what the stance patterns, written with straight apostrophes,
// are matched against, since they run on a reading's raw text.
func benchStraighten(s string) string { return benchApostrophes.Replace(s) }

// benchNormalize lowers the text, straightens curly quotation marks and
// apostrophes, turns dashes and punctuation into spaces and collapses runs,
// so the passage's words are compared with an answer's on the words alone.
// An apostrophe inside a word stays (don't, God's); one at a word's edge is
// a single quotation mark (‘Whoever ... divorce,’) or a plural possessive,
// and goes, so a saying quoted without the marks around it is the same words.
func benchNormalize(s string) string {
	s = strings.ToLower(benchStraighten(s))
	s = strings.NewReplacer("“", " ", "”", " ", "—", " ", "–", " ", "-", " ").Replace(s)
	s = benchNonWord.ReplaceAllString(s, " ")
	fields := strings.Fields(s)
	ws := fields[:0]
	for _, w := range fields {
		if w = strings.Trim(w, "'"); w != "" {
			ws = append(ws, w)
		}
	}
	return strings.Join(ws, " ")
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

// namesJudaism reports whether the normalised answer names a reading within
// Judaism, by any of the names answers give it.
func namesJudaism(normMD string) bool {
	return namesReading(normMD, "Jewish") || namesReading(normMD, "Judaism") || namesReading(normMD, "rabbinic")
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
	// the forms of self-description that do too. The assumption words are
	// the three heads the prompt names (Scripture, the supernatural,
	// authority) and the terms its own definitions of the labels use: fully
	// true, a human record, a real miracle or event, explained naturally,
	// the text alone, scholarly reconstruction. A sentence that earns its
	// label in those words without one of the three heads is still a
	// sentence that names the assumption. Tradition, council and confession
	// are left out on purpose: each is as often another thing (a Reformed
	// tradition, Peter's confession) as an authority.
	benchLabel      = regexp.MustCompile(`(?i)\b(conservative|liberal|fundamentalist)\b`)
	benchAssumption = regexp.MustCompile(`(?i)\b(scripture|supernatural|authorit\w*|assum\w*|presuppos\w*|` +
		`inerran\w*|inspir\w*|fully true|full truth|true in faith|human (?:record|editing|author\w*|composition|document|product|work|origin\w*)|` +
		`miracle\w*|miraculous|naturally|naturali[sz]\w*|natural explanation|literal\w*|real(?:ly)? (?:event|happened)|as (?:an )?event|as a sign|as (?:a )?story|` +
		`the text alone|the text as|sola scriptura|scholarly reconstruction)\b`)
	benchSelfNamed = regexp.MustCompile(`(?i)\b(themselves|self-descri\w*|their own (?:name|term|word)s?)\b`)
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

	// A first sentence that says the readings largely agree, in the words
	// answers use: a word of extent and a word of agreement, in either
	// order ("Readers across different traditions largely agree", "the
	// main readings overlap a great deal"), with no negation in front of
	// the agreement ("not all readers agree"); or little disagreement said
	// outright. A sentence with an agreement word and no extent ("Readers
	// agree on little here") has not said that they largely agree.
	benchAgreeExtent = regexp.MustCompile(`(?i)\b(?:largely|broadly|broad|mostly|mainly|essentially|substantially|` +
		`generally|general|widely|wide|overwhelmingly|most|all|every|nearly all|almost all|for the most part|` +
		`on the whole|in the main|a great deal|much)\b`)
	benchAgreeWord = regexp.MustCompile(`(?i)\b(?:agree\w*|concur\w*|overlap\w*|consensus|unanim\w*|common ground|` +
		`in common|alike|of one mind|the same (?:way|reading|meaning|sense|thing)|` +
		`shar(?:e|es|ed|ing) (?:\w+ ){0,2}(?:reading|meaning|sense|understanding|view|ground|conviction))\b`)
	benchAgreeNegated = regexp.MustCompile(`(?i)\b(?:not|no|never|hardly|seldom|rarely|little|few|without)\b(?: \w+){0,3} ` +
		`(?:agree\w*|concur\w*|overlap\w*|consensus|unanim\w*|common ground|in common|alike|of one mind|shar\w+)\b`)
	benchLittleDisagreement = regexp.MustCompile(`(?i)\b(?:little|no|hardly any|scarcely any|without|not much) (?:\w+ ){0,2}` +
		`(?:disagreement|dispute|controversy|division|divergence)\b`)

	benchURL        = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)
	benchPage       = regexp.MustCompile(`\bpp?\.\s?\d`)
	benchISBN       = regexp.MustCompile(`(?i)\bisbn\b`)
	benchAuthorYear = regexp.MustCompile(`\([A-Z][A-Za-z]+(?: and [A-Z][A-Za-z]+)?,? \d{4}\)`)
	// A quotation attributed to a speaker with a verb of speech or writing,
	// on either side of the marks. The speaker is a capitalised name of up to
	// three words ("Jesus", "Simon Peter", "the Council of Trent"), or "the"
	// and one or two plain words ("the bystanders", "the text", and also
	// "the direct" of "the direct commands", which benchNamesASpeaker then
	// reads as no speaker at all). Groups: the speaker and the words, marks
	// first, then the words and the speaker.
	benchSpeaker = `((?:[Tt]he )?[A-Z][a-z]+(?: (?:of )?[A-Z][a-z]+){0,2}|[Tt]he [a-z]+(?: [a-z]+)?)`
	// The verbs of speech and writing that open a quotation, in past and
	// present. Called and named are left out: they label ("the place called
	// ...") as often as they speak.
	benchSpeech = `(?:wrote|writes|said|says|states|stated|taught|teaches|answered|answers|replied|replies|asked|asks|declared|declares|` +
		`told|tells|spoke|speaks|promised|promises|warned|warns|proclaimed|proclaims|cried|cries|commanded|commands|` +
		`prayed|prays|preached|preaches|announced|announces|insisted|insists|affirmed|affirms|confessed|confesses|` +
		`testified|testifies|exclaimed|exclaims|explained|explains|argued|argues|noted|notes|observed|observes|` +
		`added|adds|continued|continues|concluded|concludes|prophesied|prophesies|foretold|foretells|urged|urges|` +
		`charged|charges|instructed|instructs|reminded|reminds|pleaded|pleads|put it|puts it)`
	benchAttributed = regexp.MustCompile(`\b` + benchSpeaker + ` ` + benchSpeech + `[,:]? [“"]([^”"]+)[”"]|[“"]([^”"]+)[”"],? (?:as )?` + benchSpeaker + ` ` + benchSpeech)
	// The text's own voice, which may quote the chapter without being named
	// in it, on any book: the text, the passage, the book and its parts, its
	// words, and the one who speaks for it. A plural is read by its singular
	// (benchVoiceWord).
	benchTextVoice = map[string]bool{
		"text": true, "passage": true, "verse": true, "verses": true, "chapter": true, "book": true, "word": true, "words": true,
		"prophet": true, "apostle": true, "evangelist": true, "narrator": true, "author": true,
		"writer": true, "gospel": true, "letter": true, "epistle": true, "scripture": true, "scriptures": true,
	}
	// benchBookVoice is the text's own voice on one book alone: the name it
	// is traditionally written under, the word for its kind, and for a
	// Gospel the one whose words it reports, written as benchNormalize
	// writes them. Paul may quote 1 Timothy, Moses or the Law Genesis, David
	// or the psalmist a Psalm, and Jesus a Gospel chapter that reports his
	// words without his name (Matthew 5), where the chapter names none of
	// them; and none of them may quote another book. A book that carries
	// its writer's name (Isaiah, 1 Peter) needs no entry for the writer: the
	// book's own name passes already.
	benchBookVoice = map[string]string{
		"Genesis": "moses law torah", "Exodus": "moses law torah", "Leviticus": "moses law torah",
		"Numbers": "moses law torah", "Deuteronomy": "moses law torah",
		"Psalms": "david psalm psalmist", "Proverbs": "solomon", "Ecclesiastes": "solomon",
		"Lamentations": "jeremiah", "Acts": "luke",
		"Matthew": "jesus christ", "Mark": "jesus christ", "Luke": "jesus christ", "John": "jesus christ",
		"Romans": "paul", "1 Corinthians": "paul", "2 Corinthians": "paul", "Galatians": "paul",
		"Ephesians": "paul", "Philippians": "paul", "Colossians": "paul", "1 Thessalonians": "paul",
		"2 Thessalonians": "paul", "1 Timothy": "paul", "2 Timothy": "paul", "Titus": "paul",
		"Philemon": "paul", "1 John": "john", "2 John": "john", "3 John": "john", "Revelation": "john",
	}
	// Words that open a clause before a speaker's name ("As Jesus said") and
	// are not part of it, with "the" and "of".
	benchNotASpeaker = map[string]bool{
		"the": true, "of": true, "and": true, "as": true, "but": true, "for": true, "so": true,
		"then": true, "when": true, "where": true, "while": true, "since": true, "because": true,
		"yet": true, "here": true, "thus": true, "if": true, "once": true, "after": true, "before": true,
	}
	// The plain words for a person, a body or a document that a lower-case
	// speaker phrase may name ("the council", "the psalmist", "the law"); a
	// plural names a body by its ending ("the bystanders"). A phrase whose
	// head word is none of these ("the direct", "the same breath") names no
	// one: it is prose around the quotation marks, and the words in them are
	// held to the chapter on their own (benchNamesASpeaker).
	benchBodyWords = map[string]bool{
		"law": true, "torah": true, "psalmist": true, "council": true, "synod": true, "court": true, "senate": true,
		"tribunal": true, "crowd": true, "mob": true, "multitude": true, "people": true, "nation": true, "church": true,
		"assembly": true, "congregation": true, "community": true, "synagogue": true, "household": true, "family": true,
		"army": true, "clergy": true, "laity": true, "faithful": true, "magisterium": true, "papacy": true,
		"priesthood": true, "catechism": true, "creed": true, "decree": true, "encyclical": true, "commentary": true,
		"talmud": true, "mishnah": true, "midrash": true, "targum": true, "vulgate": true, "septuagint": true,
		"king": true, "queen": true, "emperor": true, "governor": true, "ruler": true, "pope": true, "bishop": true,
		"priest": true, "rabbi": true, "elder": true, "scribe": true, "pharisee": true, "sadducee": true, "lawyer": true,
		"judge": true, "centurion": true, "soldier": true, "servant": true, "slave": true, "man": true, "woman": true,
		"child": true, "boy": true, "girl": true, "father": true, "mother": true, "son": true, "daughter": true,
		"brother": true, "sister": true, "husband": true, "wife": true, "disciple": true, "teacher": true, "master": true,
		"student": true, "scholar": true, "critic": true, "commentator": true, "translator": true, "editor": true,
		"redactor": true, "reformer": true, "theologian": true, "historian": true, "philosopher": true, "saint": true,
		"martyr": true, "angel": true, "devil": true, "serpent": true, "dragon": true, "beast": true, "voice": true,
		"speaker": true, "reader": true, "hearer": true, "believer": true, "mourner": true, "bystander": true,
		"onlooker": true, "witness": true, "friend": true, "stranger": true, "neighbour": true, "neighbor": true, "enemy": true,
	}
	// "Book chapter:verse", with an optional verse range, the book possibly
	// numbered ("1 Peter") or of the form "Song of Solomon".
	benchReference = regexp.MustCompile(`\b((?:[1-3] )?[A-Z][a-z]+(?: of [A-Z][a-z]+)?) (\d{1,3}):(\d{1,3})(?: ?[–-] ?(\d{1,3}))?`)
	// Capitalised English words that precede a bare chapter:verse of the
	// passage's own book ("In 11:33–38", "Read 7:15–16") without naming a
	// book: the opening word of a sentence or a bullet, not a citation. The
	// chapter and verse are then held to the passage's own book. A name
	// that is in neither this list nor the canon is an invented book.
	benchNotABook = map[string]bool{
		"And": true, "At": true, "But": true, "Chapter": true, "Chapters": true, "Compare": true,
		"Does": true, "From": true, "How": true, "In": true, "On": true, "Read": true, "See": true,
		"Then": true, "Verse": true, "Verses": true, "With": true,
	}

	benchNumberedQuestion = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+.*\?\s*$`)
	// A question that asks what the reader brought to the passage: what they
	// expected, already held, or came to it with.
	benchBrought = regexp.MustCompile(`(?i)\b(?:brought|bring|expect\w*|(?:came|come) to (?:it|this|the (?:passage|verse|text))|` +
		`already (?:believ\w*|th(?:ink|ought)|held|assum\w*|read|knew|know)|before (?:you )?(?:read|came|opened|looked)|` +
		`your own (?:reading|view|assumption\w*|tradition|church|background|expectation\w*|upbringing)|` +
		`you (?:already )?(?:assumed|thought|believed|took for granted|were taught|grew up)|` +
		`prior (?:view|reading|assumption\w*|belief\w*)|preconception\w*|presupposition\w*)\b`)

	// The two stances the prompt asks to be told apart on a passage that
	// reports something said or done. A reading takes the report as said or
	// done when it says so in any of the ways answers do: the words as the
	// speaker's own, the act as real, actual or historical, the text at face
	// value, as it stands, inerrant or God's word, a miracle, an act of God;
	// the report as accurate (the Bible accurately records Jesus' words,
	// reported accurately, an accurate, authoritative record, with an
	// adjective or two between the stance word and its noun); the text as
	// divinely or supernaturally inspired; the letter as written by the
	// apostle whose name it carries. A reading that names its holders and
	// the words it rests on and says nothing of the report states no stance.
	benchSaidOrDone = regexp.MustCompile(`(?i)\b(?:` +
		`as (?:actually |really |truly |genuinely |historically )?(?:said|spoken|done|happened|given|reported|recorded|written|stated|uttered|delivered|performed|told|taught|commanded|promised|foretold)|` +
		`(?:actually|really|truly|genuinely|historically|literally) (?:said|spoke|spoken|happened|did|done|took place|occurred|gave|made|created|wept|wrote|written|uttered|delivered|performed|taught|commanded|conceived|born|promised|founded|is|was|were)|` +
		`(?:his|jesus'?s?|christ's|god's|the lord's|paul's|john's|moses'?s?|isaiah's|the prophet's|the apostle's) own (?:words?|sayings?|teaching|voice|acts?|deeds?|hands?|instructions?|commands?|promise|prophecy|vision|work|letter)|` +
		`(?:jesus|christ|god|paul|john|moses|isaiah|the prophet|the apostle) himself|` +
		`(?:real|actual|genuine|true|historical|literal|authentic|reliable|accurate|factual|original|eyewitness)(?:,? (?:and |or )?(?:real|actual|genuine|true|historical|literal|authentic|reliable|accurate|factual|original|eyewitness|authoritative|inspired|divinely inspired|supernaturally inspired|trustworthy|faithful|direct|straightforward|scientific|first-?hand|contemporary|physical)){0,2} (?:events?|acts?|action|sayings?|words?|promise|sign|commands?|instructions?|prophecy|prediction|miracles?|deeds?|history|account|record|report|narrative|speech|statement|teaching|utterance|happening|occurrence|birth|creation|conception|vision|memory)|` +
		`took place|at face value|as (?:it|they) (?:stands?|reads?)|as (?:a )?(?:fact|history)|as (?:a |an )?(?:genuine|authentic|real|actual|historical|true|reliable|accurate|original)\b|` +
		`eyewitness|inerran\w*|infallib\w*|fully true|verbally inspired|god-breathed|(?:the )?(?:very )?words? of god\b|god's (?:own )?words?|` +
		`divinely inspired|supernaturally inspired|inspired by (?:god|the (?:holy )?spirit)|(?:divine|god's|the (?:holy )?spirit's) inspiration|under inspiration|inspired (?:scripture|word|text|prophecy|record|revelation)|` +
		`(?:accurately|faithfully|reliably|truthfully) (?:records?|recorded|reports?|reported|preserves?|preserved|transmits?|transmitted|describes?|described|relates?|related|recounts?|recounted|conveys?|conveyed|quotes?|quoted|hands? down|handed down|sets? down)|` +
		`(?:recorded|reported|preserved|transmitted|described|related|recounted|conveyed|quoted|handed down|set down) (?:accurately|faithfully|reliably|truthfully)|` +
		`(?:paul|the apostle paul|apostle paul|the apostle|moses|isaiah|the prophet) (?:himself )?(?:wrote|authored|penned|dictated)|` +
		`historically (?:reliable|accurate|true|real|genuine|factual|grounded|rooted)|act of (?:god|christ|jesus|the lord)|miracul\w*|a miracle|the miracle|` +
		`divine (?:act|action|events?|intervention|speech|promise|guarantee|revelation|miracle|command))\b`)
	// A reading takes the report as a later composition, or explains it
	// naturally, when it says so: a later hand, community, editor or
	// addition; a pseudonymous or attributed writing; words put in a mouth;
	// a natural cause or an ordinary birth; myth, legend, poem, liturgy or
	// genre; an exilic or priestly source. Neither the name of a school nor
	// the word scholars counts: the stance has to be stated.
	benchComposition = regexp.MustCompile(`(?i)\b(?:` +
		`later (?:compositions?|additions?|insertions?|hands?|editors?|editing|redaction|redactor|church|communit(?:y|ies)|writers?|authors?|date|dating|stage|developments?|reflection|traditions?|expansion|creation|invention|generations?|disciples?|followers?|believers?|christians?|re-?reading|theolog\w*|formulation|layers?|sources?|period|time|origin|product|work|stor(?:y|ies)|era|age)|` +
		`(?:decades|years|generations|a generation|centuries|a century|long|much) later|post-?easter|post-?resurrection|after (?:jesus'?s?|his|the) (?:death|resurrection|lifetime)|` +
		`composed|composition|compiled|compilation|redact\w*|edited|editorial|editors?|inserted|insertion|interpolat\w+|added (?:by|later|to)|an? (?:later )?addition|` +
		`pseudonym\w*|pseudepigraph\w*|deutero-?pauline|disputed (?:letter|epistle|authorship)|not (?:by|from|written by) paul|(?:using|under|in|bearing) (?:paul's|his|the apostle's) name|` +
		`attributed to (?:paul|jesus|him|moses|david|isaiah|john)|put (?:into|in) (?:his|jesus'?s?|the) mouth|placed on (?:his|jesus'?s?) lips|on (?:his|jesus'?s?) lips|` +
		`words? (?:of|from) the (?:early )?(?:church|community)|` +
		`(?:church|community|evangelist|editor|redactor|author|writer|poet|psalmist|followers?|disciples?|school|circle)(?:'s)? (?:own )?(?:reflection|creation|composition|theology|work|hand|addition|wording|shaping|framing|invention|construction|retrojection|voice|stor(?:y|ies)|hope|memory|liturgy|ritual)|` +
		`(?:community|church|evangelist|editor|redactor|author|writer|followers?|disciples?|school|circle) (?:later |then )?(?:created|composed|wrote|added|inserted|shaped|invented|constructed|formulated|produced|developed|put|placed|expanded|reworked|adapted|repurposed)|` +
		`(?:shaped|created|invented|constructed|formulated|developed|coined|crafted|written|composed|produced|generated|adapted|repurposed) (?:by|within|in|for) (?:the|an?|its|their|matthew's|john's|luke's|mark's|paul's|a later|the early|the later) (?:early |later |johannine |matthean |pauline |christian |believing |exilic |post-exilic )?(?:church|communit(?:y|ies)|congregation|circle|school|tradition|followers|disciples|movement|editors?|redactors?|priestly)|` +
		`naturally|natural (?:explanations?|causes?|events?|process(?:es)?|origins?|phenomen\w+|reading|account|terms|means|birth|conception|development|sense)|naturalist\w*|non-?supernatural|` +
		`ordinary (?:birth|woman|child|event|conception|pregnancy|human|grief|tears)|young woman|` +
		`(?:ancient near eastern|near eastern|babylonian|mesopotamian|canaanite|ugaritic|egyptian|ancient|israelite|hebrew) (?:myth\w*|cosmolog\w*|stor(?:y|ies)|poem|poetry|hymn|parallels?|creation (?:stor(?:y|ies)|myth\w*|accounts?)|literature|genre|conventions?|liturgy)|` +
		`myth\w*|legend\w*|saga|folk ?tales?|a?etiolog\w*|liturg\w+|hymn|a poem|poetic (?:composition|creation|text|account|narrative|framework|structure|figure)|` +
		`literary (?:creation|composition|device|form|genre|construct\w*|artistry|framework|structure|artifice|convention|fiction)|genre|apocalyptic (?:literature|genre|convention|imagery|form|writing)|symbolic (?:vision|narrative)|` +
		`priestly (?:source|writer|author|account|tradition|school)|exilic|post-?exilic|second temple)\b`)
)

// statesSaidOrDone and statesComposition read a reading's text for the two
// stances with its apostrophes straightened first: the patterns are written
// with straight apostrophes, and a provider's "God’s word" states the same
// stance as "God's word".
func statesSaidOrDone(text string) bool  { return benchSaidOrDone.MatchString(benchStraighten(text)) }
func statesComposition(text string) bool { return benchComposition.MatchString(benchStraighten(text)) }

// saysLargelyAgree reads a sentence for the statement the prompt asks of a
// passage whose readings largely agree, with its apostrophes straightened
// as the stances are.
func saysLargelyAgree(sentence string) bool {
	s := benchStraighten(sentence)
	if benchLittleDisagreement.MatchString(s) {
		return true
	}
	return benchAgreeExtent.MatchString(s) && benchAgreeWord.MatchString(s) && !benchAgreeNegated.MatchString(s)
}

// firstSentence is an answer's first sentence as splitSentences cuts it: a
// heading line is a sentence of its own, so an answer that opens on a
// heading has that heading as its first sentence and has said nothing
// before it.
func firstSentence(md string) string {
	if ss := splitSentences(md); len(ss) > 0 {
		return ss[0]
	}
	return ""
}

// groundingRunsWanted is how many distinct three-word runs of the passage an
// answer must carry to be grounded in its words: two of a passage of twelve
// words or more, where two runs are a small part of it; one of a shorter
// passage, where a second run would be most of the verse quoted twice (Psalm
// 23:1 is nine words); and none under four words, where the one run is the
// passage itself (John 11:35), which must then appear whole.
func groundingRunsWanted(normPassage string) int {
	switch n := words(normPassage); {
	case n < 4:
		return 0
	case n < 12:
		return 1
	}
	return 2
}

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
		if n := len(readings); n < 2 || n > 7 {
			add(checkSkeleton, "%d reading sections before %q, want two to seven", n, compareHeadings[0])
		}
	}
	if n := words(opener); n > 40 {
		add(checkSkeleton, "%d words before the first heading, want at most forty", n)
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
	// A quotation attributed to a speaker is held to the chapter the passage
	// is from: its words must be the chapter's, and its speaker must be
	// named in the chapter or be the text's own voice. A quotation of one
	// word is a term, not a quotation, and is not held. A phrase before the
	// verb that names no one ("the direct commands", "in the same breath
	// says") is prose, and the chapter's own words beside it are the
	// chapter's, in no one's mouth; words the chapter does not have are held
	// whatever stands before them. A row with no chapter text holds
	// quotations to the passage alone.
	chapter := c.chapter
	if chapter == "" {
		chapter = c.Text
	}
	normChapter := " " + benchNormalize(chapter) + " "
	for _, m := range benchAttributed.FindAllStringSubmatch(md, -1) {
		speaker, quoted := m[1]+m[4], benchNormalize(m[2]+m[3])
		if words(quoted) < 2 {
			continue
		}
		if !strings.Contains(normChapter, " "+quoted+" ") {
			add(checkCitations, "a quotation whose words are not in %s: %q", c.benchChapterKey(), m[0])
			continue
		}
		if benchNamesASpeaker(speaker) && !benchSpeaksInChapter(speaker, normChapter, c) {
			add(checkCitations, "a quotation attributed to %q, named nowhere in %s: %q", speaker, c.benchChapterKey(), m[0])
		}
	}
	for _, m := range benchReference.FindAllStringSubmatch(md, -1) {
		name := m[1]
		if benchNotABook[name] {
			name = c.Book // a bare chapter:verse is the passage's own book
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
			add(checkCitations, "%q is out of range in %s", m[0], book)
			continue
		}
		if hi := atoi(m[4]); hi > 0 && bd.GetVerse(book, ch, hi) == nil {
			add(checkCitations, "%q runs past the chapter in %s", m[0], book)
		}
	}

	// 5. The whole answer's shape. The prompt sets no word count: it asks
	// for succinctness and gives a reading the length its substance needs,
	// so the scorer holds no cap and no band, and the words of the answer
	// and of each reading are reported for a reader to weigh (the
	// recorder's line, readingLengths) rather than ruled on. One guard
	// stands against a runaway: an answer over compareBenchRunawayWords has
	// stopped being succinct whatever its substance. "Where they agree"
	// need only be there. On a passage whose readings largely agree the
	// prompt asks for that in the first sentence, and the scorer reads the
	// first sentence for it: an answer that opens on a heading, or on a
	// sentence that says anything else, has not said so.
	if n := answerWords(md); n > compareBenchRunawayWords {
		add(checkBalance, "not succinct: %d words", n)
	}
	if agree, ok := byTitle[compareHeadingAgree]; ok && words(agree.body) == 0 {
		add(checkBalance, "%q is empty", compareHeadingAgree)
	}
	if c.NoConflict {
		if first := firstSentence(md); !saysLargelyAgree(first) {
			add(checkBalance, "the first sentence does not say the readings largely agree: %q", first)
		}
	}

	// 6. Coverage: the readings the prompt implies for this row. On a passage
	// that reports something said or done, one reading section takes the
	// report so and a different one takes it as a later composition or
	// explains it naturally; a single section carrying both stances has not
	// said which is which. On a Hebrew Bible passage, Scripture to Judaism
	// as well as to the churches, the prompt's ask for each faith's reading
	// as its own section comes to a reading within Judaism, so one is owed;
	// the check reads the whole answer for its name, and an answer naming
	// none has left a faith's reading out.
	if c.SaidOrDone && len(readings) > 0 {
		var said, composed []int
		for i, r := range readings {
			text := r.title + "\n" + r.body
			if statesSaidOrDone(text) {
				said = append(said, i)
			}
			if statesComposition(text) {
				composed = append(composed, i)
			}
		}
		told := false
		for _, i := range said {
			for _, j := range composed {
				if i != j {
					told = true
				}
			}
		}
		switch {
		case told:
		case len(said) == 0 && len(composed) == 0:
			add(checkCoverage, "no reading says it takes what the passage reports as said or done, and none takes it as a later composition or explains it naturally")
		case len(said) == 0:
			add(checkCoverage, "no reading says it takes what the passage reports as said or done")
		case len(composed) == 0:
			add(checkCoverage, "no reading takes what the passage reports as a later composition or explains it naturally")
		default:
			add(checkCoverage, "%q alone carries both stances, so which is which is not said", readings[said[0]].title)
		}
	}
	if c.owesJudaism() && !namesJudaism(normMD) {
		add(checkCoverage, "a Hebrew Bible passage is Scripture to Judaism as well, so a reading within Judaism is owed its own section, and none is named")
	}

	// 7. Questions: two to four numbered, one of them, or a line beside them,
	// asking what the reader brought to the passage.
	if q, ok := byTitle[compareHeadingQuestions]; ok {
		if n := len(benchNumberedQuestion.FindAllString(q.body, -1)); n < 2 || n > 4 {
			add(checkQuestions, "%d numbered questions, want two to four", n)
		}
		if !benchBrought.MatchString(q.body) {
			add(checkQuestions, "no question asks what the reader brought to the passage")
		}
	}

	// 8. Grounding: the passage's own words, in the measure its length
	// allows (groundingRunsWanted).
	if need := groundingRunsWanted(normPassage); need == 0 {
		if !strings.Contains(" "+normMD+" ", " "+normPassage+" ") {
			add(checkGrounding, "the passage %q does not appear whole", c.Text)
		}
	} else {
		found := 0
		for run := range threeWordRuns(normPassage) {
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

// benchSpeaksInChapter reports whether a quotation's speaker is a voice of the
// chapter: every word of the name, past a clause opener, is a word of the
// chapter, of the text's own voice, or of the book's name and the voice of
// that book alone.
func benchSpeaksInChapter(speaker, normChapter string, c benchCase) bool {
	book := " " + benchNormalize(c.Book) + " " + benchBookVoice[c.Book] + " "
	for _, w := range strings.Fields(benchNormalize(speaker)) {
		switch {
		case benchNotASpeaker[w], benchVoiceWord(w):
		case strings.Contains(normChapter, " "+w+" "):
		case strings.Contains(book, " "+w+" "):
		default:
			return false
		}
	}
	return true
}

// benchVoiceWord reports whether w, singular or plural, is the text's own
// voice (benchTextVoice).
func benchVoiceWord(w string) bool {
	return benchTextVoice[w] || benchTextVoice[strings.TrimSuffix(w, "s")]
}

// benchNamesASpeaker reports whether the phrase before a verb of speech names
// someone the quotation can be attributed to: a capitalised name past the
// article ("Jesus", "the Council of Trent"), or a plain phrase whose head
// word is the text's own voice, a word for a person, a body or a document,
// or a plural ("the text", "the council", "the bystanders"). "The direct" of
// "the direct commands" and "the same breath" name no one, so the words
// beside them are held to the chapter on their own and in no mouth.
func benchNamesASpeaker(speaker string) bool {
	ws := strings.Fields(speaker)
	if len(ws) > 0 && strings.EqualFold(ws[0], "the") {
		ws = ws[1:]
	}
	if len(ws) == 0 {
		return false
	}
	for _, w := range ws {
		if w != strings.ToLower(w) {
			return true
		}
	}
	head := benchNormalize(ws[len(ws)-1])
	return benchVoiceWord(head) || benchBodyWords[head] || strings.HasSuffix(head, "s")
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

// readingLengths is readingTitles with each reading's word count beside its
// title, for the recorder's line: the scorer holds no reading to a length,
// so what the answer gave each reading is reported for a reader to weigh.
func readingLengths(md string) []string {
	_, secs := splitMarkdownSections(md)
	var out []string
	for _, s := range secs {
		if s.level == 2 && s.title == compareHeadings[0] {
			break
		}
		out = append(out, fmt.Sprintf("%s (%d)", s.title, words(s.body)))
	}
	return out
}

// compareOmissions is check 10 over a provider's whole set: with an answer
// for every passage, every Hebrew Bible row's answer names a reading within
// Judaism, the one reading the prompt's ask for each faith's own section
// implies on such a row, seen across the set rather than row by row (where
// the coverage check reads it too). A provider with an incomplete set is
// not judged, so the check is a verdict on a set and never on a sample of
// it. The detail opens with the provider's id, which the recorder reads to
// tell the default provider's verdict from the others'.
func compareOmissions(tb benchTable, answers map[string]map[string]string) []problem {
	var ps []problem
	for _, provider := range sortedKeys(answers) {
		complete := true
		for _, c := range tb.Passages {
			if _, ok := answers[provider][c.ID]; !ok {
				complete = false
			}
		}
		if !complete {
			continue
		}
		var hebrew, missing []string
		for _, c := range tb.Passages {
			if !c.owesJudaism() {
				continue
			}
			hebrew = append(hebrew, c.ID)
			if !namesJudaism(benchNormalize(answers[provider][c.ID])) {
				missing = append(missing, c.ID)
			}
		}
		if len(missing) > 0 {
			ps = append(ps, problem{check: checkOmission,
				detail: fmt.Sprintf("%s names a reading within Judaism on %d of its %d Hebrew Bible rows, and none on %s",
					provider, len(hebrew)-len(missing), len(hebrew), strings.Join(missing, ", "))})
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
// rows are the no-conflict controls and carry no said-or-done floor, since a
// floor on a passage whose readings largely agree would force a division;
// no row names a reading, the prompt naming none; at least one row carries
// the floor and at least one is a Hebrew Bible passage, so both halves of
// the coverage check are exercised by the set and the omission check has a
// row to read. Every row's chapter is in chapters.json with its
// verses ascending and the row's verses among them, the row's text is those
// verses word for word, and where the machine's cache carries the full World
// English Bible the committed chapter is that Bible's, verse for verse;
// without the cache the run says the chapters went unchecked against it.
func TestCompareBenchPassages(t *testing.T) {
	tb := loadBenchTable(t)
	chapters := loadBenchChapters(t)
	bd := benchBible(t)
	if tb.Version != compareBenchVersion {
		t.Errorf("table version %q, want %q", tb.Version, compareBenchVersion)
	}
	if len(tb.Passages) != 10 {
		t.Fatalf("%d passages, want ten", len(tb.Passages))
	}
	raw, err := os.ReadFile(filepath.Join(compareBenchDir, compareBenchTable))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"mustInclude", "roster"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("the table carries %q, a list of readings; the prompt names none and the bench keeps no list of readers", key)
		}
	}
	canon := NewBibleData()
	seen := map[string]bool{}
	noConflict, floors, hebrew := 0, 0, 0
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
		vs := chapters[c.benchChapterKey()]
		var selected []string
		for i, v := range vs {
			if i > 0 && v.Verse <= vs[i-1].Verse {
				t.Errorf("%s: verse %d of %s follows verse %d", c.ID, v.Verse, c.benchChapterKey(), vs[i-1].Verse)
			}
			if v.Verse >= c.Lo && v.Verse <= c.Hi {
				selected = append(selected, v.Text)
			}
		}
		if got := strings.Join(selected, " "); got != c.Text {
			t.Errorf("%s: the row's text is not verses %d to %d of %s:\n got: %s\nwant: %s", c.ID, c.Lo, c.Hi, c.benchChapterKey(), c.Text, got)
		}
		if len(bd.Verses[c.Book]) > 0 {
			full := bd.GetChapter(c.Book, c.Chapter)
			if len(full) != len(vs) {
				t.Errorf("%s: %s has %d verses in the machine's %s, %d committed", c.ID, c.benchChapterKey(), len(full), compareBenchVersion, len(vs))
			}
			for i := 0; i < len(full) && i < len(vs); i++ {
				if full[i].Verse != vs[i].Verse || strings.ReplaceAll(full[i].Text, "\r\n", "\n") != vs[i].Text {
					t.Errorf("%s: verse %d of %s differs from the machine's %s", c.ID, vs[i].Verse, c.benchChapterKey(), compareBenchVersion)
				}
			}
		}
		if c.NoConflict {
			noConflict++
			if c.SaidOrDone {
				t.Errorf("%s: a no-conflict row with the said-or-done floor", c.ID)
			}
		}
		if c.SaidOrDone {
			floors++
		}
		if c.owesJudaism() {
			hebrew++
		}
	}
	if noConflict != 2 || !tb.byID()["psalm-23-1"].NoConflict || !tb.byID()["john-11-35"].NoConflict {
		t.Errorf("the no-conflict controls are Psalm 23:1 and John 11:35; got %d rows flagged", noConflict)
	}
	if floors == 0 {
		t.Error("no row carries the said-or-done floor, so that half of the coverage check is never exercised")
	}
	if hebrew == 0 {
		t.Error("no row is a Hebrew Bible passage, so the reading within Judaism is never asked for")
	}
}

// --- The controls ---------------------------------------------------------------

// compareControls maps each planted control to the one check it trips and
// the row it is scored against. good.md trips nothing, and nor does
// neighbour-quote.md, good.md quoting the verse after the passage in the
// mouth of its speaker. Three controls take that quotation through each
// branch of the citation check in turn: misquote-neighbour.md alters its
// words, so it trips before its speaker is read; wrong-speaker.md keeps the
// words and puts them in the mouth of bystanders the chapter never names;
// wrong-writer.md puts them in Paul's mouth, the voice of his letters and
// of no Gospel. Three controls hold the first stance of the coverage check:
// said-by-record.md states it in one reading only as the Bible accurately
// recording Jesus' words, beside a composition reading, and passes;
// said-by-gods-word.md states it in one reading only as the Bible being
// God’s word, with the curly apostrophe providers write, and passes;
// no-said.md names holders and words and takes no stance on the report in
// any reading, beside the same composition reading, and trips the check.
// Two controls hold the grounding check to a short passage: on Psalm 23:1,
// nine words, grounded-once.md quotes "I shall lack nothing" once and
// passes, and ungrounded-short.md names the verse's word shepherd, quotes
// no three-word run of it, and trips the check. Two controls stand for the
// band the scorer no longer holds a reading to: thin.md cuts good.md's last
// reading to one sentence of twenty-five words, lopsided.md lengthens its
// first to a hundred and fifty, and both pass, the prompt giving a reading
// the length its substance needs. overlong.md runs past the runaway guard
// and trips the balance check as not succinct; agree-unsaid.md, a
// two-reading answer to John 11:35, a passage whose readings largely
// agree, opens on a heading instead of saying so, and trips the same check.
var compareControls = map[string]struct {
	check   int
	passage string
}{
	"good.md":               {0, "matthew-16-18"},
	"neighbour-quote.md":    {0, "matthew-16-18"},
	"misquote-neighbour.md": {checkCitations, "matthew-16-18"},
	"wrong-speaker.md":      {checkCitations, "matthew-16-18"},
	"wrong-writer.md":       {checkCitations, "matthew-16-18"},
	"one-reading.md":        {checkSkeleton, "john-11-35"},
	"table.md":              {checkSkeleton, "matthew-16-18"},
	"swapped.md":            {checkSkeleton, "matthew-16-18"},
	"labels.md":             {checkLabels, "matthew-16-18"},
	"unsettled.md":          {checkLabels, "matthew-16-18"},
	"verdict.md":            {checkVerdict, "matthew-16-18"},
	"invented-ref.md":       {checkCitations, "matthew-16-18"},
	"quoted.md":             {checkCitations, "matthew-16-18"},
	"lopsided.md":           {0, "matthew-16-18"},
	"thin.md":               {0, "matthew-16-18"},
	"overlong.md":           {checkBalance, "matthew-16-18"},
	"empty-agree.md":        {checkBalance, "matthew-16-18"},
	"agree-unsaid.md":       {checkBalance, "john-11-35"},
	"all-as-said.md":        {checkCoverage, "matthew-16-18"},
	"said-by-record.md":     {0, "matthew-16-18"},
	"said-by-gods-word.md":  {0, "matthew-16-18"},
	"no-said.md":            {checkCoverage, "matthew-16-18"},
	"missing-jewish.md":     {checkCoverage, "isaiah-7-14"},
	"no-questions.md":       {checkQuestions, "matthew-16-18"},
	"no-brought.md":         {checkQuestions, "matthew-16-18"},
	"ungrounded.md":         {checkGrounding, "matthew-16-18"},
	"grounded-once.md":      {0, "psalm-23-1"},
	"ungrounded-short.md":   {checkGrounding, "psalm-23-1"},
	"linked.md":             {checkNetwork, "matthew-16-18"},
	"cut-short.md":          {checkNetwork, "matthew-16-18"},
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
// the controls planted to pass, good.md first among them, pass all nine;
// and every file in the directory is accounted for.
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

// citationProblems is the citation check's share of a score.
func citationProblems(ps []problem) []string {
	var out []string
	for _, p := range ps {
		if p.check == checkCitations {
			out = append(out, p.detail)
		}
	}
	return out
}

// A quotation attributed to a speaker is held to the chapter the passage is
// from. The passage's own words and the verses around it pass in the mouth
// of one the chapter names, past a clause opener, in either order of words
// and name, and in the text's own voice; a one-word quotation is a term and
// is not held. The same words attributed to a writer, a council or
// bystanders the chapter never names trip the citation check alone, with
// the name in the detail; words the chapter does not have trip it whoever is
// named, with the chapter in the detail; and the chapter is the row's own,
// so Matthew's words in John's chapter are not there. A saying the chapter
// sets in single quotation marks is quoted without them. Every verb of
// speech or writing the pattern reads opens a held quotation, on either side
// of the marks.
func TestCompareScorerHoldsQuotationsToTheChapter(t *testing.T) {
	rows := loadBenchTable(t).byID()
	c := rows["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	for _, line := range []string{
		"Jesus said, “you are Peter, and on this rock I will build my assembly.”",
		"Jesus says, “Blessed are you, Simon Bar Jonah, for flesh and blood has not revealed this to you.”",
		"Simon Peter answered, “You are the Christ, the Son of the living God.”",
		"As Jesus said, “I will give to you the keys of the Kingdom of Heaven.”",
		"“Get behind me, Satan!” Jesus said.",
		"The text says, “the gates of Hades will not prevail against it.”",
		"Matthew writes, “Who do men say that I, the Son of Man, am?”",
		"Jesus warned, “Get behind me, Satan!”",
		"Jesus promised, “I will give to you the keys of the Kingdom of Heaven.”",
		"“You are the Christ, the Son of the living God,” Simon Peter confessed.",
		"The Greek says “petra”.",
	} {
		if ps := scoreCompareAnswer(canon, c, good+"\n"+line+"\n"); len(ps) != 0 {
			t.Errorf("%q is the chapter in its speaker's mouth, yet: %s", line, problemLines(ps))
		}
	}
	for _, tc := range []struct{ line, detail string }{
		{"Theophilus wrote, “on this rock I will build my assembly”.", `attributed to "Theophilus"`},
		{"The Council of Trent stated, “on this rock I will build my assembly.”", `attributed to "The Council of Trent"`},
		{"the bystanders said, “You are the Christ, the Son of the living God.”", `attributed to "the bystanders"`},
		{"Jesus said, “the rock is the confession and nothing else.”", "words are not in Matthew 16"},
		{"Peter said, “You are the Christ, the Son of the one true God.”", "words are not in Matthew 16"},
		{"Augustine wrote, “the rock is the confession and nothing else.”", "words are not in Matthew 16"},
	} {
		ps := scoreCompareAnswer(canon, c, good+"\n"+tc.line+"\n")
		if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
			t.Errorf("control: %q trips %v, want [%d]:\n%s", tc.line, got, checkCitations, problemLines(ps))
		}
		if cs := citationProblems(ps); len(cs) != 1 || !strings.Contains(cs[0], tc.detail) {
			t.Errorf("%q: citation problems %v, want one saying %q", tc.line, cs, tc.detail)
		}
	}

	// Invented words are held in any mouth whatever verb opens them. The
	// verbs are written out here rather than read from the pattern, so a
	// verb dropped from the pattern is still asked for.
	invented := "the rock is the confession and nothing else"
	for _, verb := range []string{
		"wrote", "writes", "said", "says", "states", "stated", "taught", "teaches", "answered", "answers",
		"replied", "replies", "asked", "asks", "declared", "declares", "told", "tells", "spoke", "speaks",
		"promised", "promises", "warned", "warns", "proclaimed", "proclaims", "cried", "cries", "commanded", "commands",
		"prayed", "prays", "preached", "preaches", "announced", "announces", "insisted", "insists", "affirmed", "affirms",
		"confessed", "confesses", "testified", "testifies", "exclaimed", "exclaims", "explained", "explains", "argued", "argues",
		"noted", "notes", "observed", "observes", "added", "adds", "continued", "continues", "concluded", "concludes",
		"prophesied", "prophesies", "foretold", "foretells", "urged", "urges", "charged", "charges", "instructed", "instructs",
		"reminded", "reminds", "pleaded", "pleads", "put it", "puts it",
	} {
		for _, line := range []string{
			"Jesus " + verb + ", “" + invented + ".”",
			"“" + invented + ",” as Jesus " + verb + ".",
		} {
			ps := scoreCompareAnswer(canon, c, good+"\n"+line+"\n")
			if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
				t.Errorf("control: %q trips %v, want [%d]:\n%s", line, got, checkCitations, problemLines(ps))
			}
		}
	}

	john := rows["john-11-35"]
	line := "Jesus said, “you are Peter, and on this rock I will build my assembly.”"
	if cs := citationProblems(scoreCompareAnswer(canon, john, line+"\n")); len(cs) != 1 || !strings.Contains(cs[0], "not in John 11") {
		t.Errorf("Matthew's words on John 11:35: citation problems %v, want one naming John 11", cs)
	}

	divorce := rows["matthew-5-31-32"]
	for _, tc := range []struct {
		line  string
		trips bool
	}{
		{"Jesus said, “Whoever shall put away his wife, let him give her a writing of divorce.”", false},
		{"Jesus said, “Whoever shall put away his wife, let him give her a bill of divorce.”", true},
	} {
		cs := citationProblems(scoreCompareAnswer(canon, divorce, tc.line+"\n"))
		if tc.trips != (len(cs) == 1) {
			t.Errorf("%q on Matthew 5:31: citation problems %v, want %v", tc.line, cs, tc.trips)
		}
	}
}

// The name a book is traditionally written under, the word for its kind,
// and for a Gospel the one whose words it reports, are the text's own voice
// on that book alone. Paul may quote 1 Timothy 2, Moses and the Law Genesis
// 1, David and the psalmist Psalm 23, John Revelation 20 and Jesus Matthew
// 5, none of which names them, and Isaiah his own book, which does; the same
// names over Matthew 16's words, and Jesus over Romans 9's, are a speaker
// the chapter never names, and trip the citation check alone with the name
// in the detail. The directory control is wrong-writer.md, good.md quoting
// Matthew 16:19 in Paul's mouth.
func TestCompareScorerHoldsAWriterToTheirBook(t *testing.T) {
	rows := loadBenchTable(t).byID()
	canon := NewBibleData()
	for _, tc := range []struct{ id, line string }{
		{"1-timothy-2-12", "Paul writes, “But I don’t permit a woman to teach, nor to exercise authority over a man, but to be in quietness.”"},
		{"genesis-1-1-5", "Moses wrote, “In the beginning, God created the heavens and the earth.”"},
		{"genesis-1-1-5", "The law says, “Let there be light,” and light is the first thing made."},
		{"psalm-23-1", "David says, “I shall lack nothing.”"},
		{"psalm-23-1", "The psalmist says, “I shall lack nothing.”"},
		{"revelation-20-1-6", "John writes, “Blessed and holy is he who has part in the first resurrection.”"},
		{"isaiah-7-14", "Isaiah said, “Behold, the virgin will conceive, and bear a son, and shall call his name Immanuel.”"},
		{"matthew-5-31-32", "Jesus says, “Whoever shall put away his wife, let him give her a writing of divorce.”"},
	} {
		if cs := citationProblems(scoreCompareAnswer(canon, rows[tc.id], tc.line+"\n")); len(cs) != 0 {
			t.Errorf("%q on %s is the book in its own voice, yet: %v", tc.line, tc.id, cs)
		}
	}
	line := "Jesus said, “I will have mercy on whom I have mercy, and I will have compassion on whom I have compassion.”"
	if cs := citationProblems(scoreCompareAnswer(canon, rows["romans-9-14-18"], line+"\n")); len(cs) != 1 || !strings.Contains(cs[0], `attributed to "Jesus"`) {
		t.Errorf("control: %q on Romans 9: citation problems %v, want one naming Jesus", line, cs)
	}
	c := rows["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	for _, tc := range []struct{ line, detail string }{
		{"Paul writes, “on this rock I will build my assembly.”", `attributed to "Paul"`},
		{"Luke writes, “on this rock I will build my assembly.”", `attributed to "Luke"`},
		{"Isaiah said, “on this rock I will build my assembly.”", `attributed to "Isaiah"`},
		{"Moses said, “on this rock I will build my assembly.”", `attributed to "Moses"`},
		{"David says, “on this rock I will build my assembly.”", `attributed to "David"`},
		{"The law says, “on this rock I will build my assembly.”", `attributed to "The law"`},
		{"The psalmist says, “on this rock I will build my assembly.”", `attributed to "The psalmist"`},
	} {
		ps := scoreCompareAnswer(canon, c, good+"\n"+tc.line+"\n")
		if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
			t.Errorf("control: %q trips %v, want [%d]:\n%s", tc.line, got, checkCitations, problemLines(ps))
		}
		if cs := citationProblems(ps); len(cs) != 1 || !strings.Contains(cs[0], tc.detail) {
			t.Errorf("%q: citation problems %v, want one saying %q", tc.line, cs, tc.detail)
		}
	}
}

// A phrase before a verb of speech that names no one ("the direct commands",
// "in the same breath says") is prose, and the chapter's own words in
// quotation marks beside it are the chapter's: they trip nothing. A name,
// the text's own voice, a plain word for a person, a body or a document, or
// a plural names a speaker, who is held to the chapter as before; and words
// the chapter does not have trip the citation check whatever stands before
// them. Controls: the chapter's words in the mouth of a council and of
// bystanders the chapter never names, and of a named writer, trip with the
// name in the detail; the same writer with altered words, and the prose
// phrase with altered words, trip on the words.
func TestCompareScorerReadsProseAroundTheChaptersWords(t *testing.T) {
	rows := loadBenchTable(t).byID()
	canon := NewBibleData()
	for _, tc := range []struct{ id, line string }{
		{"1-timothy-2-12", `This reading rests on the direct commands "don't permit," "teach," and "exercise authority."`},
		{"matthew-16-18", `Jesus renames Simon "Peter" (*Petros*, rock) and in the same breath says "on this rock" — the nearest and most natural referent.`},
	} {
		if cs := citationProblems(scoreCompareAnswer(canon, rows[tc.id], tc.line+"\n")); len(cs) != 0 {
			t.Errorf("%q on %s is prose around the chapter's words, yet: %v", tc.line, tc.id, cs)
		}
	}
	for _, tc := range []struct{ id, line, detail string }{
		{"matthew-16-18", `the council stated, “on this rock I will build my assembly.”`, `attributed to "the council"`},
		{"matthew-16-18", `the bystanders said, “on this rock I will build my assembly.”`, `attributed to "the bystanders"`},
		{"matthew-16-18", `Augustine commands, “on this rock I will build my assembly.”`, `attributed to "Augustine"`},
		{"matthew-16-18", `Augustine commands, “on this rock I will build my church.”`, "words are not in Matthew 16"},
		{"1-timothy-2-12", `This reading rests on the direct commands "do not permit," "teach," and "exercise authority."`, "words are not in 1 Timothy 2"},
	} {
		cs := citationProblems(scoreCompareAnswer(canon, rows[tc.id], tc.line+"\n"))
		if len(cs) != 1 || !strings.Contains(cs[0], tc.detail) {
			t.Errorf("control: %q on %s: citation problems %v, want one saying %q", tc.line, tc.id, cs, tc.detail)
		}
	}
	for _, tc := range []struct {
		speaker string
		names   bool
	}{
		{"Jesus", true}, {"the Council of Trent", true}, {"The text", true}, {"the following verses", true},
		{"the council", true}, {"the bystanders", true}, {"The law", true}, {"The psalmist", true},
		{"the direct", false}, {"the same breath", false}, {"the following", false},
	} {
		if got := benchNamesASpeaker(tc.speaker); got != tc.names {
			t.Errorf("benchNamesASpeaker(%q) = %v, want %v", tc.speaker, got, tc.names)
		}
	}
}

// checkProblems is one check's share of a score.
func checkProblems(ps []problem, check int) []string {
	var out []string
	for _, p := range ps {
		if p.check == check {
			out = append(out, p.detail)
		}
	}
	return out
}

// The grounding check scales with the passage: two three-word runs of a
// passage of twelve words or more, one of a shorter passage, and the whole
// passage under four words, at each edge of the count. On Psalm 23:1, nine
// words, one run is wanted: grounded-once.md quotes "I shall lack nothing"
// once and passes every check, and ungrounded-short.md names the verse's
// word shepherd and quotes no run of it, so it trips the grounding check
// alone, asking for one. Control: an answer carrying one run passes on a
// row of eleven words and trips on the same row a word longer, asking for
// two; the whole of a three-word row passes and two of its words do not.
func TestCompareScorerScalesGroundingToThePassage(t *testing.T) {
	for _, tc := range []struct{ words, want int }{{2, 0}, {3, 0}, {4, 1}, {11, 1}, {12, 2}, {26, 2}} {
		passage := strings.TrimSpace(strings.Repeat("word ", tc.words))
		if got := groundingRunsWanted(benchNormalize(passage)); got != tc.want {
			t.Errorf("a passage of %d words wants %d runs, want %d", tc.words, got, tc.want)
		}
	}
	rows := loadBenchTable(t).byID()
	psalm := rows["psalm-23-1"]
	if n := words(benchNormalize(psalm.Text)); n != 9 {
		t.Fatalf("Psalm 23:1 is %d words; the controls are written for nine", n)
	}
	canon := NewBibleData()
	if ps := scoreCompareAnswer(canon, psalm, readCompareControl(t, "grounded-once.md")); len(ps) != 0 {
		t.Errorf("grounded-once.md quotes one run of a nine-word verse, yet: %s", problemLines(ps))
	}
	ps := scoreCompareAnswer(canon, psalm, readCompareControl(t, "ungrounded-short.md"))
	if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkGrounding}) {
		t.Errorf("control: ungrounded-short.md trips %v, want [%d]:\n%s", got, checkGrounding, problemLines(ps))
	}
	if gs := checkProblems(ps, checkGrounding); len(gs) != 1 || !strings.HasSuffix(gs[0], "want 1") {
		t.Errorf("control: ungrounded-short.md's grounding problems %v, want one asking for one run", gs)
	}

	eleven := benchCase{ID: "eleven", Book: "Psalms", Chapter: 23, Lo: 1, Hi: 1,
		Text: "one two three four five six seven eight nine ten eleven"}
	twelve := eleven
	twelve.Text += " twelve"
	oneRun := "The reading rests on \"one two three\" and on nothing else.\n"
	if gs := checkProblems(scoreCompareAnswer(canon, eleven, oneRun), checkGrounding); len(gs) != 0 {
		t.Errorf("one run of an eleven-word passage: %v", gs)
	}
	if gs := checkProblems(scoreCompareAnswer(canon, twelve, oneRun), checkGrounding); len(gs) != 1 || !strings.HasSuffix(gs[0], "want 2") {
		t.Errorf("control: one run of a twelve-word passage: %v, want one asking for two runs", gs)
	}
	three := eleven
	three.Text = "one two three"
	if gs := checkProblems(scoreCompareAnswer(canon, three, oneRun), checkGrounding); len(gs) != 0 {
		t.Errorf("the whole of a three-word passage: %v", gs)
	}
	if gs := checkProblems(scoreCompareAnswer(canon, three, "The reading rests on \"one two\".\n"), checkGrounding); len(gs) != 1 || !strings.Contains(gs[0], "does not appear whole") {
		t.Errorf("control: two words of a three-word passage: %v, want one saying it does not appear whole", gs)
	}
}

// giveChapter gives bd a chapter of n verses in book, so a scorer test can
// hold citations to a Bible of its own instead of the machine's cache.
func giveChapter(bd *BibleData, book string, ch, n int) {
	if bd.Verses[book] == nil {
		bd.Verses[book] = map[int][]Verse{}
	}
	for v := 1; v <= n; v++ {
		bd.Verses[book][ch] = append(bd.Verses[book][ch], Verse{BookName: book, Book: book, Chapter: ch, Verse: v, Text: "verse"})
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
	giveChapter(bd, "Matthew", 16, 28)
	giveChapter(bd, "1 Corinthians", 3, 23)
	giveChapter(bd, "1 Peter", 2, 25)
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

// A label is licensed by the assumption that earns it, stated in the words
// the prompt defines the label with as much as by naming Scripture, the
// supernatural or authority outright; a label hung on a reading's holders
// with no assumption in the sentence is not. Control: each bare sentence
// trips the label check alone.
func TestCompareScorerReadsTheLabelDefinitions(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	for _, line := range []string{
		"By treating the text as a human record and explaining its prophecy naturally, this is a liberal reading.",
		"Because it holds the text as fully true and the miracle as real, this is a conservative reading.",
		"It is a conservative reading because it treats the text as the true, authoritative command of Jesus.",
		"This makes it a liberal reading, as it explains the text's development through human editing rather than as the unchanged speech of Jesus.",
		"This is theologically conservative (inerrancy and a real miracle).",
		"Theological stance: conservative, because it affirms the full truth of the text and literal events.",
	} {
		if ps := scoreCompareAnswer(canon, c, good+"\n"+line+"\n"); len(ps) != 0 {
			t.Errorf("%q names its assumption, yet: %s", line, problemLines(ps))
		}
	}
	for _, line := range []string{
		"It is held by Catholics, Eastern Orthodox, and many conservative Protestants.",
		"It is held by mainline Protestants and liberal scholars across traditions.",
		"Theologically conservative.",
		"Theological stance: conservative.",
	} {
		if got := checksTripped(scoreCompareAnswer(canon, c, good+"\n"+line+"\n")); fmt.Sprint(got) != fmt.Sprint([]int{checkLabels}) {
			t.Errorf("control: %q trips %v, want [%d]", line, got, checkLabels)
		}
	}
}

// A bare chapter:verse opened by an English word ("In 16:18", "Read 16:13")
// refers to the passage's own book and is not a citation of a book that
// does not exist; a capitalised name that is neither such a word nor a book
// of the canon still is. Control: the invented book trips the citation
// check alone.
func TestCompareScorerAllowsABareReferenceToTheBook(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	for _, line := range []string{
		"In 16:18 the promise follows the confession.",
		"Read 16:13–20 as one scene.",
		"But 16:19 gives the keys to Peter alone.",
	} {
		if ps := scoreCompareAnswer(canon, c, good+"\n"+line+"\n"); len(ps) != 0 {
			t.Errorf("%q is a bare reference, yet: %s", line, problemLines(ps))
		}
	}
	line := "Hezekiah 16:18 says the same."
	if got := checksTripped(scoreCompareAnswer(canon, c, good+"\n"+line+"\n")); fmt.Sprint(got) != fmt.Sprint([]int{checkCitations}) {
		t.Errorf("control: %q trips %v, want [%d]", line, got, checkCitations)
	}
}

// A bare chapter:verse is read as the passage's own book, and its chapter
// and verse are held to that book when the Bible carries verses: "In 11:33"
// on John 11 is in range, "In 99:1" on Psalm 23 is a chapter the Bible given
// does not have. The Bible here carries Psalm 23 and John 11 alone, and
// good.md cites neither, so the citation problems are the added line's
// alone. Control: the John line on the Psalm passage trips, read as Psalm
// 11:33 against a Bible with Psalm 23 alone.
func TestCompareScorerHoldsABareReferenceToItsBook(t *testing.T) {
	rows := loadBenchTable(t).byID()
	good := readCompareControl(t, "good.md")
	bd := NewBibleData()
	giveChapter(bd, "Psalms", 23, 6)
	giveChapter(bd, "John", 11, 57)
	citations := citationProblems
	for _, id := range []string{"psalm-23-1", "john-11-35"} {
		if ps := citations(scoreCompareAnswer(bd, rows[id], good)); len(ps) != 0 {
			t.Fatalf("good.md on %s cites out of this Bible before any line is added: %v", id, ps)
		}
	}
	for _, tc := range []struct {
		id, line string
		trips    bool
	}{
		{"john-11-35", "In 11:33 he groaned in the spirit.", false},
		{"john-11-35", "In 11:57 the order to report him is given.", false},
		{"psalm-23-1", "In 23:1 the shepherd is named.", false},
		{"psalm-23-1", "In 99:1 the same shepherd reigns.", true},
		{"psalm-23-1", "In 23:7 the psalm would go on.", true},
		{"psalm-23-1", "In 11:33 he groaned in the spirit.", true},
	} {
		got := citations(scoreCompareAnswer(bd, rows[tc.id], good+"\n"+tc.line+"\n"))
		switch {
		case tc.trips && (len(got) != 1 || !strings.Contains(got[0], benchReference.FindString(tc.line))):
			t.Errorf("%q on %s: citation problems %v, want one naming the reference", tc.line, tc.id, got)
		case !tc.trips && len(got) != 0:
			t.Errorf("%q on %s is in range, yet: %v", tc.line, tc.id, got)
		}
	}
}

// The scorer holds no reading to a band: the prompt gives a reading the
// length its substance needs and no number, so good.md with its first
// reading padded to three hundred words passes, as thin.md, whose last
// reading is one sentence of twenty-five words, is planted to. What each
// reading was given is reported beside its title instead (readingLengths,
// the recorder's line). Control: the same padding carried on until the
// whole answer passes the runaway guard trips the balance check alone, so
// the padding reaches the scorer and only that guard reads it.
func TestCompareScorerHoldsNoReadingBand(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	readingWords := func(md, title string) int {
		_, secs := splitMarkdownSections(md)
		for _, s := range secs {
			if s.title == title {
				return words(s.body)
			}
		}
		t.Fatalf("no %q section", title)
		return 0
	}
	padded := func(to int) string {
		n := readingWords(good, "Catholic")
		if n > to {
			t.Fatalf("good.md's first reading is %d words, more than the %d wanted", n, to)
		}
		heading := "## Catholic\n"
		md := strings.Replace(good, heading, heading+strings.Repeat("again ", to-n)+"\n", 1)
		if got := readingWords(md, "Catholic"); got != to {
			t.Fatalf("the first reading padded to %d words, want %d", got, to)
		}
		return md
	}
	if got := strings.Join(readingLengths(good), "; "); got != "Catholic (101); Eastern Orthodox (96); Reformed (102); Historical-critical (87)" {
		t.Errorf("good.md's readings are reported as %q", got)
	}
	thin := readCompareControl(t, "thin.md")
	if got := readingLengths(thin); got[len(got)-1] != "Historical-critical (25)" {
		t.Errorf("thin.md's last reading is reported as %q", got[len(got)-1])
	}
	if ps := scoreCompareAnswer(canon, c, thin); len(ps) != 0 {
		t.Errorf("a reading of twenty-five words fails: %s", problemLines(ps))
	}
	long := padded(300)
	if ps := scoreCompareAnswer(canon, c, long); len(ps) != 0 {
		t.Errorf("a reading of three hundred words fails: %s", problemLines(ps))
	}
	if got := readingLengths(long)[0]; got != "Catholic (300)" {
		t.Errorf("the padded reading is reported as %q", got)
	}
	runaway := padded(compareBenchRunawayWords)
	n := answerWords(runaway)
	if n <= compareBenchRunawayWords {
		t.Fatalf("the padded answer is %d words, inside the runaway guard", n)
	}
	ps := scoreCompareAnswer(canon, c, runaway)
	if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkBalance}) {
		t.Errorf("control: the padding past the guard trips %v, want [%d]:\n%s", got, checkBalance, problemLines(ps))
	}
	if len(ps) != 1 || ps[0].detail != fmt.Sprintf("not succinct: %d words", n) {
		t.Errorf("control: the padding past the guard reads as %s", problemLines(ps))
	}
}

// The runaway guard is the one length the scorer holds, and it is strict at
// its figure: good.md padded to exactly compareBenchRunawayWords words
// passes, and one word more trips the balance check alone, as not
// succinct, with the count in the detail. The padding goes after the last
// section, where no other check counts words. The directory control is
// overlong.md.
func TestCompareScorerHoldsTheRunawayGuardStrictly(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	padded := func(to int) string {
		n := answerWords(good)
		if n >= to {
			t.Fatalf("good.md is %d words, no room under %d", n, to)
		}
		md := good + "\n" + strings.TrimSpace(strings.Repeat("again ", to-n)) + "\n"
		if got := answerWords(md); got != to {
			t.Fatalf("the padded answer is %d words, want %d", got, to)
		}
		return md
	}
	over := compareBenchRunawayWords + 1
	ps := scoreCompareAnswer(canon, c, padded(over))
	if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkBalance}) {
		t.Errorf("%d words trips %v, want [%d]:\n%s", over, got, checkBalance, problemLines(ps))
	}
	if want := fmt.Sprintf("not succinct: %d words", over); len(ps) != 1 || ps[0].detail != want {
		t.Errorf("%d words reads as %s, want %q", over, problemLines(ps), want)
	}
	if ps := scoreCompareAnswer(canon, c, padded(compareBenchRunawayWords)); len(ps) != 0 {
		t.Errorf("control: %d words is inside the guard, yet: %s", compareBenchRunawayWords, problemLines(ps))
	}
}

// On a passage whose readings largely agree the prompt asks for that in the
// first sentence, and the scorer reads the first sentence for it in the
// words answers use: a word of extent and a word of agreement, in either
// order, with no negation in front of the agreement, or little disagreement
// said outright. Each line below is read as saying so; none of the lines
// after them is, a heading among them, since an answer that opens on a
// heading has said nothing first. A two-reading answer to John 11:35 that
// opens on the sentence passes; the same answer with the sentence second
// trips the balance check alone, naming the sentence that came first; and
// on Matthew 16:18, whose readings part, good.md opens on a heading and
// passes, so the rule is the no-conflict rows' alone. The directory control
// is agree-unsaid.md.
func TestCompareScorerWantsTheAgreeSentenceFirst(t *testing.T) {
	for _, line := range []string{
		"Readers across different traditions largely agree on the meaning of this famous verse.",
		"Readers largely agree this verse shows Jesus experiencing deep human emotion, though they differ on the event's history.",
		"On this verse the main readings largely agree: the speaker calls God his shepherd, and on that ground claims he will lack nothing.",
		"Psalm 23:1 is one of the most widely shared verses in the Bible, and its main readings overlap a great deal.",
		"The readings of this verse are broadly in agreement.",
		"Most readers agree that Jesus wept from grief.",
		"Jews and Christians alike share one reading of this verse, and there is little real disagreement about it.",
		"There is broad consensus on this verse across the churches.",
		"Readers of every tradition agree on what the verse means.",
		"Readers’ views largely concur here.",
	} {
		if !saysLargelyAgree(line) {
			t.Errorf("%q is not read as saying the readings largely agree", line)
		}
	}
	for _, line := range []string{
		"## Jewish Reading",
		"Jesus wept: at the tomb of Lazarus, Jesus grieved as any mourner does, and the verse shows his full humanity.",
		"Readers do not agree on why Jesus wept.",
		"Not all readers agree on the meaning of this verse.",
		"Few readers agree about who the shepherd is.",
		"The readings differ sharply on who the shepherd is.",
		"Readers agree on little here.",
		"There is no agreement on the verse.",
		"The verse is one of the most quoted in the Bible.",
	} {
		if saysLargelyAgree(line) {
			t.Errorf("control: %q is read as saying the readings largely agree", line)
		}
	}

	c := loadBenchTable(t).byID()["john-11-35"]
	if !c.NoConflict {
		t.Fatal("John 11:35 is not a no-conflict row")
	}
	canon := NewBibleData()
	one := readCompareControl(t, "one-reading.md")
	sentence := firstSentence(one)
	if !saysLargelyAgree(sentence) || !strings.HasPrefix(one, sentence+"\n\n## ") {
		t.Fatalf("one-reading.md does not open on the sentence the row asks for: %q", sentence)
	}
	at := strings.Index(one, "## "+compareHeadingAgree)
	second := "## The sign and the tears\nThe tears are read with the raising that follows them: Jesus weeps and then calls Lazarus out, so grief and power stand in one scene, and the weight falls on the whole chapter as much as on the one verb. Held by readers of every church alike.\n\n"
	two := one[:at] + second + one[at:]
	if ps := scoreCompareAnswer(canon, c, two); len(ps) != 0 {
		t.Fatalf("two readings on John 11:35 opening on the sentence should pass, yet: %s", problemLines(ps))
	}
	other := "Jesus wept, and the verse gives no reason for the tears."
	moved := other + " " + two
	if got := firstSentence(moved); got != other {
		t.Fatalf("the moved answer's first sentence is %q", got)
	}
	ps := scoreCompareAnswer(canon, c, moved)
	if got := checksTripped(ps); fmt.Sprint(got) != fmt.Sprint([]int{checkBalance}) {
		t.Errorf("the sentence second trips %v, want [%d]:\n%s", got, checkBalance, problemLines(ps))
	}
	if len(ps) != 1 || !strings.Contains(ps[0].detail, "first sentence") || !strings.Contains(ps[0].detail, other) {
		t.Errorf("the sentence second reads as %s", problemLines(ps))
	}
	matthew := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	if !strings.HasPrefix(good, "## ") {
		t.Fatalf("good.md does not open on a heading: %.40q", good)
	}
	if ps := scoreCompareAnswer(canon, matthew, good); len(ps) != 0 {
		t.Errorf("control: on a passage whose readings part, an answer opening on a heading fails: %s", problemLines(ps))
	}
}

// The prompt asks for no opening sentence, so none is required: good.md
// opens on its first heading and passes, forty words before that heading
// pass, and forty-one trip the skeleton check alone.
func TestCompareScorerAllowsAShortOpener(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	if !strings.HasPrefix(good, "## ") {
		t.Fatalf("good.md does not open on its first heading: %.40q", good)
	}
	for _, tc := range []struct {
		opener int
		trips  bool
	}{{0, false}, {40, false}, {41, true}} {
		md := strings.TrimSpace(strings.Repeat("again ", tc.opener)) + "\n\n" + good
		if tc.opener == 0 {
			md = good
		}
		want := fmt.Sprint([]int{})
		if tc.trips {
			want = fmt.Sprint([]int{checkSkeleton})
		}
		if got := checksTripped(scoreCompareAnswer(canon, c, md)); fmt.Sprint(got) != want {
			t.Errorf("%d words before the first heading trips %v, want %s", tc.opener, got, want)
		}
	}
}

// The reading count is two to seven, both ends inside it: two and seven
// pass, eight trip the skeleton check alone, and one trips it with the
// coverage check, since on a passage with the said-or-done floor a single
// reading cannot tell the two stances apart (one-reading.md, on a passage
// without the floor, trips the skeleton check alone). The answers are built
// from good.md's four fixed sections and readings of forty-odd words each,
// the first taking the saying as Jesus' own and the second as the
// community's, so no other check moves with the count.
func TestCompareScorerHoldsTheReadingCountAtItsEdges(t *testing.T) {
	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	tail := good[strings.Index(good, "## "+compareHeadingAgree):]
	bodies := []string{
		"Peter himself is the rock, and the reading takes the words as Jesus' own words, spoken as the Gospel reports them, and the promise as a real act of Christ; it holds the Bible fully true and reads it with the Church's tradition. Held by the Catholic Church.",
		"The saying is a later composition, Matthew's community reflecting on Peter's standing after his death; the weight falls on the word assembly, which Matthew alone uses. It treats the Gospel as a human record shaped by its community. Held by critical scholars of several churches.",
		"The rock is the confession Peter has just made, and the Church is built on right faith kept by every bishop; the weight falls on the verse before and on the shift from Petros to petra. Held by the Orthodox Church, reading with the Fathers.",
	}
	with := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "## Reading %d\n%s\n\n", i+1, bodies[i%len(bodies)])
		}
		return b.String() + tail
	}
	for _, tc := range []struct {
		readings int
		trips    []int
	}{
		{1, []int{checkSkeleton, checkCoverage}},
		{2, []int{}},
		{7, []int{}},
		{8, []int{checkSkeleton}},
	} {
		md := with(tc.readings)
		if got := len(readingTitles(md)); got != tc.readings {
			t.Fatalf("the built answer has %d readings, want %d", got, tc.readings)
		}
		if got := checksTripped(scoreCompareAnswer(canon, c, md)); fmt.Sprint(got) != fmt.Sprint(tc.trips) {
			t.Errorf("%d readings trips %v, want %v:\n%s", tc.readings, got, tc.trips, problemLines(scoreCompareAnswer(canon, c, md)))
		}
	}
}

// On a passage that reports something said or done, the coverage check
// reads the two stances in the words answers use for them, with a curly
// apostrophe as with a straight one, and wants them in different readings.
// Each line below is read as the stance it states, none of the neutral
// lines as either; and good.md with its two stances folded into one reading
// trips the coverage check alone, since which is which is then not said.
// Control: a reading that states neither stance, added to good.md, changes
// nothing. The directory control for the apostrophe is said-by-gods-word.md.
func TestCompareScorerTellsTheTwoStancesApart(t *testing.T) {
	for _, line := range []string{
		"It takes the words as Jesus' own words, spoken to Peter as the Gospel reports them.",
		"This reading takes the saying as actually spoken by Jesus.",
		"The promise is a real act of Christ.",
		"It assumes the Bible is a historical record of the Church's founding.",
		"Holders take the sign as a genuine prophecy, fulfilled in a virgin birth that was a miracle.",
		"It holds that God really did create in six days.",
		"Paul himself wrote the letter, and the instruction is his own command.",
		"The reading takes the text at face value.",
		"It reads the vision as given to John, and the Bible as inerrant.",
		"This assumes the Bible accurately records Jesus' words.",
		"It assumes the Bible accurately reports Jesus's teachings.",
		"This reading assumes the Bible is the accurate, authoritative record of Jesus' teachings.",
		"It assumes the Bible provides an accurate historical and scientific record.",
		"They assume the birth is reported accurately in the New Testament.",
		"It assumes the Apostle Paul wrote the letter under divine inspiration.",
		"This view assumes the text is divinely inspired and written by Paul.",
		"Traditional readers assume the text is a genuine, supernaturally inspired prophecy given by God to Isaiah.",
		"It assumes the Bible is God’s word, interpreted alongside Church tradition.",
	} {
		if !statesSaidOrDone(line) || statesComposition(line) {
			t.Errorf("%q is not read as taking the report as said or done", line)
		}
	}
	for _, line := range []string{
		"The saying is read as a later composition.",
		"Matthew's community created the verse after Jesus' death.",
		"The exception clause was added by the evangelist.",
		"It explains the sign naturally: a young woman in Ahaz's day.",
		"The letter was written decades later by a follower using Paul's name.",
		"The passage is an ancient Near Eastern poem adapting older creation myths.",
		"On this reading the vision is apocalyptic literature of the first century.",
		"It treats the saying as the evangelist's own creation.",
		"The tears are a later theological story added by the author.",
		"The verse is Matthew's redaction of an earlier saying.",
		"The exception clause is the evangelist’s own addition.",
	} {
		if !statesComposition(line) || statesSaidOrDone(line) {
			t.Errorf("%q is not read as taking the report as a later composition or explaining it naturally", line)
		}
	}
	for _, line := range []string{
		"Peter is the rock, and the Church is built on him.",
		"The weight falls on the wordplay between Peter and rock.",
		"Held by the Catholic Church.",
		"Jesus says, you are Peter, and the crowd hears him.",
		"Critical scholars of several churches hold it.",
		"Matthew alone records this promise, and the Gospel's report of it is read with the Fathers.",
		"It reads the words with the Church's tradition, which is its authority.",
		"Scholars inspired by the Aramaic wordplay look for an early form of the saying.",
		"The letter addresses a church in Ephesus, and its holders read it with Paul's other letters.",
		"It reads the words with the Church’s tradition, which is its authority.",
	} {
		if statesSaidOrDone(line) || statesComposition(line) {
			t.Errorf("control: %q states a stance", line)
		}
	}

	c := loadBenchTable(t).byID()["matthew-16-18"]
	good := readCompareControl(t, "good.md")
	canon := NewBibleData()
	at := strings.Index(good, "## "+compareHeadingAgree)
	both := "## Peter the rock\nPeter himself is the rock, and the reading takes the words as Jesus' own words, spoken as the Gospel reports them; some scholars instead read the saying as a later composition of Matthew's community. The weight falls on the wordplay between Peter and rock. Held by the Catholic Church.\n\n"
	neutral := "## Anglican\nThe rock is Peter in his confession, and the Church is built on both together; the weight falls on the whole scene from verse 13, and on Peter as the first to speak for the Twelve. Held by the Church of England and the churches of the Anglican Communion, reading with the Fathers and the Reformers.\n\n"
	folded := both + neutral + good[at:]
	if got := checksTripped(scoreCompareAnswer(canon, c, folded)); fmt.Sprint(got) != fmt.Sprint([]int{checkCoverage}) {
		t.Errorf("both stances in one reading trips %v, want [%d]:\n%s", got, checkCoverage, problemLines(scoreCompareAnswer(canon, c, folded)))
	}
	if ps := scoreCompareAnswer(canon, c, good[:at]+neutral+good[at:]); len(ps) != 0 {
		t.Errorf("control: a reading stating neither stance, added, yet: %s", problemLines(ps))
	}
}

// The questions check reads a question about what the reader brought to the
// passage in the words such questions use, and not in a question about the
// text. Control: a numbered question about the verses around the passage
// is not one.
func TestCompareScorerReadsTheQuestionAboutWhatTheReaderBrought(t *testing.T) {
	for _, line := range []string{
		"4. What did you bring to this verse: which reading did you expect, and where did it come from?",
		"4. Which reading did you expect before you read these, and why?",
		"4. What did you already believe about this verse, and from whom?",
		"4. Which church or tradition did you come to it from?",
		"Ask what you brought to the passage, and whether it survived the reading.",
	} {
		if !benchBrought.MatchString(line) {
			t.Errorf("%q is not read as asking what the reader brought", line)
		}
	}
	for _, line := range []string{
		"1. Read verses 13 to 20 together: does the promise answer the man or the confession?",
		"2. In your translation, is rock the same word as Peter, and what changes if it is?",
		"3. What does Jesus build, an assembly, and who belongs to it elsewhere in this Gospel?",
	} {
		if benchBrought.MatchString(line) {
			t.Errorf("control: %q is read as asking what the reader brought", line)
		}
	}
}

// The omission check judges a provider by its whole set: a complete set
// whose Hebrew Bible rows all name a reading within Judaism passes, whether
// or not any other row names one; a complete set naming it on none of them,
// or on all but one, is one problem naming the rows that lack it; and a set
// with a passage missing is not judged, however its rows read.
func TestCompareOmissionAcrossTheSet(t *testing.T) {
	tb := loadBenchTable(t)
	var hebrew []string
	for _, c := range tb.Passages {
		if c.owesJudaism() {
			hebrew = append(hebrew, c.ID)
		}
	}
	if len(hebrew) < 2 {
		t.Fatalf("the table has %d Hebrew Bible rows; the test wants two or more", len(hebrew))
	}
	set := func(body func(c benchCase) string) map[string]string {
		m := map[string]string{}
		for _, c := range tb.Passages {
			m[c.ID] = body(c)
		}
		return m
	}
	jewish := "The reading within Judaism takes the verse as Israel's own; the Catholic and Reformed readings take it as the Church's."
	churches := "The Catholic and Reformed readings take it as the Church's."
	answers := map[string]map[string]string{
		"everywhere": set(func(benchCase) string { return jewish }),
		"onhebrew": set(func(c benchCase) string {
			if c.owesJudaism() {
				return jewish
			}
			return churches
		}),
		"nowhere": set(func(benchCase) string { return churches }),
		"butone": set(func(c benchCase) string {
			if c.ID == hebrew[0] {
				return churches
			}
			return jewish
		}),
		"partial": {hebrew[0]: churches},
	}
	ps := compareOmissions(tb, answers)
	if len(ps) != 2 {
		t.Fatalf("omissions = %s, want one for nowhere and one for butone", problemLines(ps))
	}
	for i, want := range []string{
		fmt.Sprintf("butone names a reading within Judaism on %d of its %d Hebrew Bible rows, and none on %s", len(hebrew)-1, len(hebrew), hebrew[0]),
		fmt.Sprintf("nowhere names a reading within Judaism on 0 of its %d Hebrew Bible rows, and none on %s", len(hebrew), strings.Join(hebrew, ", ")),
	} {
		if ps[i].check != checkOmission || ps[i].detail != want {
			t.Errorf("omission %d = %s, want check %d: %s", i, ps[i], checkOmission, want)
		}
	}
	delete(answers, "nowhere")
	delete(answers, "butone")
	if ps := compareOmissions(tb, answers); len(ps) != 0 {
		t.Errorf("control: the everywhere, onhebrew and partial sets should pass, got %s", problemLines(ps))
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
// answer from the default provider, no provider with a complete set leaves
// the reading within Judaism out of a Hebrew Bible row, and the fixtures
// were recorded against the prompt as it stands (logged otherwise; a
// failure with the bench switch set). This is red until the first
// recording round's fixtures are committed: a set that does not exist
// scores nothing and must not pass.
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
