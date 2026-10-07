package bibletext

// The comparison verb's contract: its prompt carries the roster, the floors,
// the four section headings in order and the passage last; the four prose
// prompts name differing readings in one shared sentence and still forbid
// headings; README.md mirrors every prompt word for word; the action string
// is dispatched by an allow-list; the request is filed under its own cache
// scope; the panel titles it "Perspectives" and renders its headings; and a
// provider's refusal reaches the reader as a declined passage with Try again.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// compareControlAnswer is the hand-written answer to the contract over
// Matthew 16:18 (peterPassage) that the fairness scorer must pass, read from
// testdata/compare/_controls/good.md: the comparison a development launch
// shows (dev_open_sheets_on.go embeds the same file) and the control the
// scorer's own checks are proven against. Line endings are normalised, as
// readNativeSource normalises them, so a checkout's style is not asserted on.
func compareControlAnswer(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "compare", "_controls", "good.md"))
	if err != nil {
		t.Fatalf("the control comparison is missing: %v", err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// The control answer is the skeleton the prompt asks for, rendered: one
// sentence of at most forty words, then four readings and the four sections
// in order as headings, the questions a numbered list; it quotes the
// passage's own words; and it keeps under the length the prompt sets.
// Control: the same text with its last heading renamed is not the skeleton.
func TestCompareControlAnswerRendersTheSkeleton(t *testing.T) {
	md := compareControlAnswer(t)
	headings := func(md string) []string {
		w := widget.NewRichTextFromMarkdown("")
		setAIAnswerText(w, md)
		return subHeadings(w.Segments)
	}
	got := headings(md)
	want := append([]string{"Catholic", "Eastern Orthodox", "Reformed", "Historical-critical"}, comparePromptHeadings...)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("the control answer's headings are %v, want %v", got, want)
	}
	opener := strings.SplitN(md, "\n## ", 2)[0]
	if n := len(strings.Fields(opener)); n == 0 || n > 40 || strings.Count(opener, ". ") > 0 {
		t.Errorf("the opener is not one sentence of at most forty words: %q", opener)
	}
	for _, run := range []string{"gates of Hades", "will not prevail"} {
		if !strings.Contains(md, run) {
			t.Errorf("the control answer does not quote %q from the passage", run)
		}
	}
	if n := len(strings.Fields(strings.ReplaceAll(md, "## ", ""))); n > 700 {
		t.Errorf("the control answer is %d words", n)
	}
	questions := md[strings.LastIndex(md, "## "):]
	if n := len(regexp.MustCompile(`(?m)^\d+\. .*\?$`).FindAllString(questions, -1)); n < 2 || n > 4 {
		t.Errorf("the questions section has %d numbered questions", n)
	}

	renamed := strings.Replace(md, "## "+compareHeadingQuestions, "## Questions", 1)
	if renamed == md || strings.Join(headings(renamed), "|") == strings.Join(want, "|") {
		t.Error("control: a renamed heading still reads as the skeleton")
	}
}

// peterPassage is Matthew 16:18 in the World English Bible, the worked
// example of the comparison's design.
func peterPassage() aiPassage {
	return aiPassage{Book: "Matthew", Chapter: 16, Lo: 18, Hi: 18, Ref: "Matthew 16:18",
		Text: "I also tell you that you are Peter, and on this rock I will build my assembly, " +
			"and the gates of Hades will not prevail against it.",
		Version: "World English Bible"}
}

// The four headings, the test's own copy: a renamed constant is caught here
// and not silently carried into the prompt and the scorer together.
var comparePromptHeadings = []string{
	"Where they agree",
	"Where they part, and why",
	"What the text settles",
	"Questions to take back to the text",
}

func TestBuildComparePrompt(t *testing.T) {
	psg := peterPassage()
	got := buildComparePrompt(psg)
	for _, want := range []string{
		// The roster: churches, the Jewish reading, schools, and groups for whom
		// a passage is a key text.
		"Catholic", "Eastern Orthodox", "Lutheran", "Reformed", "Anglican", "Wesleyan",
		"Baptist", "Anabaptist", "Pentecostal", "the Jewish reading of the Hebrew text",
		"young-earth creationist", "old-earth", "framework", "dispensational", "covenantal",
		"historical-critical", "Latter-day Saints", "Jehovah's Witnesses",
		"Name the holders precisely.",
		// The floors.
		"include both a reading that takes the claim as true and one that explains it naturally",
		"Do not present any reading, including the academic historical-critical one, as the neutral or default view",
		"If the readings largely agree, say so in one sentence and keep every section short.",
		// Order, the opening sentence, length.
		"oldest reading first", "exactly one sentence before the first heading", "under 700 words",
		"Give each reading the length its substance needs",
		// The labels, defined and tied to an assumption.
		"Where those assumptions make a reading conservative or liberal in the theological sense, say so and say which assumption makes it so.",
		`Here "conservative" means holding Scripture to be fully true and its supernatural events real; "liberal" means treating Scripture as a human record and explaining the supernatural naturally.`,
		// Each reading at its strongest; the text judges; no invented citations.
		"describe each reading as its own thoughtful holders would state it",
		"a weak version of a reading is a false report",
		`the text does the judging, in "What the text settles"`,
		"Quote the passage freely, but quote no writer, document or council, and invent no page numbers, links or tables.",
		"Refer to other Scripture only by book, chapter and verse, and only where you are sure of it.",
		// The passage.
		"The quoted text is from the World English Bible. Verse numbers may appear inside the quotation.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the comparison prompt lacks %q", want)
		}
	}

	// The skeleton: one section per reading, then the four headings, in order,
	// each on its own line as a level-two markdown heading.
	at := strings.Index(got, "\n## <Reading>\n")
	if at < 0 {
		t.Fatalf("the prompt has no per-reading section:\n%s", got)
	}
	for i, h := range comparePromptHeadings {
		if compareHeadings[i] != h {
			t.Errorf("compareHeadings[%d] = %q, want %q", i, compareHeadings[i], h)
		}
		next := strings.Index(got, "\n## "+h+"\n")
		if next < 0 || next < at {
			t.Errorf("heading %q is missing or out of order (at %d, after %d)", h, next, at)
			continue
		}
		at = next
	}
	if strings.Count(got, "\n## ") != 1+len(comparePromptHeadings) {
		t.Errorf("the prompt asks for %d headings, want %d", strings.Count(got, "\n## "), 1+len(comparePromptHeadings))
	}

	// Instruction first, passage last.
	if !strings.HasPrefix(got, "You are a knowledgeable Bible study assistant") {
		t.Errorf("the prompt does not open on the instruction:\n%s", got[:80])
	}
	if tail := "Passage (Matthew 16:18):\n“" + psg.Text + "”"; !strings.HasSuffix(got, tail) {
		t.Errorf("the prompt does not end on the passage lines %q:\n%s", tail, got)
	}

	// Controls: it is its own builder, and the prose prompt has no skeleton.
	explain := buildAIPrompt(aiActionExplain, psg)
	if got == explain {
		t.Fatal("the comparison prompt is the Explain prompt")
	}
	if strings.Contains(explain, "## Where they agree") {
		t.Error("control: the Explain prompt carries the comparison's heading")
	}
	if strings.Contains(got, "Do not use markdown headings") {
		t.Error("the comparison prompt forbids the headings it asks for")
	}
}

// The sentence the four prose verbs share about a passage read more than one
// way, the test's own copy.
const readingsSentence = "Where this passage is read differently, name the main readings, who holds " +
	"them and the assumption each rests on, and say what the wording of the passage counts for " +
	"or against; where a point is uncertain, say so briefly. Do not quote anyone, and name a " +
	"document or writer only when you are sure of it."

// Explain, Context, Translation and Ask all carry the sentence and still
// forbid headings and lists; the comparison carries neither. Control: the
// older "scholars disagree" sentence is gone from all four.
func TestStudyPromptsNameDifferingReadings(t *testing.T) {
	psg := john3Passage()
	prose := map[string]string{
		aiActionExplain:     buildAIPrompt(aiActionExplain, psg),
		aiActionContext:     buildAIPrompt(aiActionContext, psg),
		aiActionTranslation: buildAIPrompt(aiActionTranslation, psg),
		aiActionAsk:         buildAskPrompt(psg, "Who is the world here?"),
	}
	for action, p := range prose {
		if !strings.Contains(p, readingsSentence) {
			t.Errorf("%s: the prompt does not name differing readings:\n%s", action, p)
		}
		if !strings.Contains(p, "Do not use markdown headings or bullet lists.") {
			t.Errorf("%s: the prompt no longer forbids headings and lists:\n%s", action, p)
		}
		if strings.Contains(p, "Where scholars disagree") || strings.Contains(p, "rather than overstating") {
			t.Errorf("%s: the superseded sentence is still in the prompt:\n%s", action, p)
		}
	}
	compare := buildComparePrompt(psg)
	if strings.Contains(compare, readingsSentence) {
		t.Error("the comparison prompt carries the prose verbs' sentence")
	}
	if strings.Contains(compare, "Do not use markdown headings") {
		t.Error("the comparison prompt forbids headings")
	}
}

// The verbs every menu offers: four, in this order, the comparison last under
// its label, every one with a prompt; Ask has a prompt but no menu entry.
func TestAIStudyVerbs(t *testing.T) {
	want := []aiStudyVerb{
		{aiActionExplain, "Explain"},
		{aiActionContext, "Analyze context"},
		{aiActionTranslation, "Analyze translation"},
		{aiActionCompare, "Compare perspectives"},
	}
	if len(aiStudyVerbs) != len(want) {
		t.Fatalf("aiStudyVerbs = %v, want %v", aiStudyVerbs, want)
	}
	for i, v := range want {
		if aiStudyVerbs[i] != v {
			t.Errorf("aiStudyVerbs[%d] = %v, want %v", i, aiStudyVerbs[i], v)
		}
		if !aiStudyActionKnown(v.Action) {
			t.Errorf("%q has no prompt", v.Action)
		}
	}
	if !aiStudyActionKnown(aiActionAsk) {
		t.Error("Ask has no prompt")
	}
	for _, bad := range []string{"", "compar", "Explain", "perspectives", "search"} {
		if aiStudyActionKnown(bad) {
			t.Errorf("%q counts as a study action", bad)
		}
	}
}

// fencedBlock is one ``` block of README.md.
var fencedBlock = regexp.MustCompile("(?s)```\n(.*?)\n```")

// readmeWhatGetsSent is the README section that reproduces the prompts: from
// its heading to the next level-two heading outside a fenced block (the
// comparison prompt carries "## " lines of its own inside one).
func readmeWhatGetsSent(t *testing.T) string {
	t.Helper()
	readme := readRepoFile(t, "README.md")
	start := strings.Index(readme, "### What gets sent")
	if start < 0 {
		t.Fatal("README.md has no \"What gets sent\" section")
	}
	var b strings.Builder
	fenced := false
	for _, line := range strings.SplitAfter(readme[start:], "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		} else if !fenced && strings.HasPrefix(line, "## ") {
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

// README.md reproduces what leaves the device: the shared prose prompt with
// its {task} slot, the three tasks, and the comparison prompt. Each is held
// to the builder's output word for word — the README wraps lines at eighty
// columns, so the comparison is on the words — with placeholders standing
// for the passage. A one-word edit to either side fails here.
func TestREADMEMirrorsPrompts(t *testing.T) {
	section := readmeWhatGetsSent(t)
	blocks := fencedBlock.FindAllStringSubmatch(section, -1)
	if len(blocks) != 2 {
		t.Fatalf("the section has %d fenced blocks, want the shared prompt and the comparison", len(blocks))
	}
	shared, compare := blocks[0][1], blocks[1][1]
	if strings.Count(shared, "{task}") != 1 {
		t.Fatalf("the shared block does not carry one {task} slot:\n%s", shared)
	}
	placeholders := aiPassage{Book: "{Book}", Ref: "{Book} {Chapter}:{Verses}",
		Text: "{selected text}", Version: "{Version}"}
	words := func(s string) string { return strings.Join(strings.Fields(s), " ") }

	for _, c := range []struct{ label, action string }{
		{"Explain", aiActionExplain},
		{"Analyze context", aiActionContext},
		{"Analyze translation", aiActionTranslation},
	} {
		m := regexp.MustCompile(`- \*\*` + regexp.QuoteMeta(c.label) + `\*\* — "([^"]*)"`).FindStringSubmatch(section)
		if m == nil {
			t.Errorf("README.md has no %s task", c.label)
			continue
		}
		task := strings.ReplaceAll(m[1], "`", "")
		want := words(strings.Replace(shared, "{task}", task, 1))
		if got := words(buildAIPrompt(c.action, placeholders)); got != want {
			t.Errorf("%s: README.md and buildAIPrompt differ\n got: %s\nwant: %s", c.label, got, want)
		}
	}
	if got, want := words(buildComparePrompt(placeholders)), words(compare); got != want {
		t.Errorf("README.md and buildComparePrompt differ\n got: %s\nwant: %s", got, want)
	}
}

// An action string outside the allow-list is an error before the cache or the
// key is consulted: an entry filed under that string is not served, and no
// key error is raised for it. Control: the comparison, on the same state,
// reaches the key check.
func TestRunAIActionRejectsUnknownAction(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	st := &AppState{CurrentBook: "Acts", CurrentChapter: 2}
	key := aiCacheKey(providerGemini+"|"+geminiModel+"|compar", "Acts", 2, "they were all together")
	aiCacheSet(key, "filed under a misspelt verb")
	t.Cleanup(func() {
		aiCacheMu.Lock()
		delete(aiCache, key)
		aiCacheMu.Unlock()
	})
	got, err := runAIAction(context.Background(), st, "compar", "they were all together", "", selSpan{})
	if err == nil || got != "" {
		t.Fatalf("an unknown action answered (%q, %v)", got, err)
	}
	if isNoKeyError(err) || !strings.Contains(err.Error(), `"compar"`) {
		t.Errorf("an unknown action should fail as unknown, got %v", err)
	}
	if _, err := runAIAction(context.Background(), st, aiActionCompare, "they were all together", "", selSpan{}); !isNoKeyError(err) {
		t.Errorf("control: the comparison should reach the key check, got %v", err)
	}
}

// The comparison is filed under its own scope, with the verse range: an
// Explain answer for the same verse is not served for it, nor its own
// answer for the refrain at another verse.
func TestRunAIActionCompareIsItsOwnScope(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	st := refrainChapterState()
	seed := func(scope, answer string) {
		key := aiCacheKey(scope, "Psalms", 136, refrain)
		aiCacheSet(key, answer)
		t.Cleanup(func() {
			aiCacheMu.Lock()
			delete(aiCache, key)
			aiCacheMu.Unlock()
		})
	}
	seed(providerGemini+"|"+geminiModel+"|"+aiActionExplain+"|2-2", "explained at verse two")
	seed(providerGemini+"|"+geminiModel+"|"+aiActionCompare+"|2-2", "compared at verse two")
	got, err := runAIAction(context.Background(), st, aiActionCompare, refrain, "", selSpanFromNative(2, 2))
	if err != nil || got != "compared at verse two" {
		t.Fatalf("compare at verse 2: got (%q, %v)", got, err)
	}
	if got, err := runAIAction(context.Background(), st, aiActionCompare, refrain, "", selSpanFromNative(3, 3)); !isNoKeyError(err) {
		t.Errorf("verse 3 was served verse 2's comparison: (%q, %v)", got, err)
	}
	if got, err := runAIAction(context.Background(), st, aiActionExplain, refrain, "", selSpanFromNative(2, 2)); err != nil || got != "explained at verse two" {
		t.Errorf("control: Explain at verse 2: got (%q, %v)", got, err)
	}
}

// compareFixture is a hand-written answer to the contract, short, over the
// World English Bible's Matthew 16:18.
const compareFixture = "The readings that differ most on this verse: Catholic, Reformed, and the historical-critical reading.\n\n" +
	"## Catholic\n" +
	"Peter himself is the rock: Jesus gives Simon a new name and builds on him, so the assembly rests on Peter's person and office. " +
	"The weight falls on the wordplay between Peter and rock read as one and the same. It assumes Scripture fully true, the promise a real " +
	"act of Christ, and tradition read with the text. Held by the Catholic Church; conservative on Scripture and the supernatural.\n\n" +
	"## Reformed\n" +
	"The rock is Christ confessed by Peter, who speaks for all the apostles; no single office is founded here. The weight falls on the " +
	"change of word and on Scripture read with Scripture. It assumes Scripture fully true and the text alone as authority. Held by " +
	"Reformed and Presbyterian churches; conservative on the same grounds.\n\n" +
	"## Historical-critical\n" +
	"The saying is read as Matthew's community reflecting on Peter's standing. The weight falls on Matthew alone using the word " +
	"assembly. It assumes Scripture a human record, the promise a story, and scholarly reconstruction as authority; liberal on both " +
	"counts, and scholars are divided among themselves.\n\n" +
	"## Where they agree\n" +
	"All three read Peter as spokesman for the confession, the assembly as the one people Jesus gathers, and the gates of Hades as " +
	"the power of death.\n\n" +
	"## Where they part, and why\n" +
	"- Whether Peter and rock name the same thing.\n" +
	"- Whether what is founded is an office that passes on, or the confession as such.\n" +
	"- Whether the words are Jesus' own or the community's: an assumption about Scripture.\n\n" +
	"## What the text settles\n" +
	"The wordplay is in the text, and the verse before it is the confession; the passage says nothing of succession either way.\n\n" +
	"## Questions to take back to the text\n" +
	"1. Read verses 13 to 20 together: does the promise answer the man or the confession?\n" +
	"2. In your translation, is rock the same word as Peter?\n" +
	"3. Which reading did you expect, and why?\n"

// The panel titles the comparison and renders its headings as headings.
func TestComparePanelTitlesAndRendersTheSections(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	reqs := stubStudyRequests(t)
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	showAIPanel(st, aiActionCompare, peterPassage().Text, "", selSpan{})
	r := nextStudyRequest(t, reqs, "the comparison")
	p := studyOverlay(t, w)
	if !treeHasText(p, "Perspectives") {
		t.Errorf("the panel is not titled for the comparison; texts %v", treeTexts(p))
	}
	r.answer <- compareFixture
	settled(t, r)
	var answer *widget.RichText
	walkTree(p, func(o fyne.CanvasObject) {
		if rt, ok := o.(*widget.RichText); ok && strings.Contains(segmentText(rt.Segments), "Catholic") {
			answer = rt
		}
	})
	if answer == nil {
		t.Fatalf("the answer did not land in the panel; texts %v", treeTexts(p))
	}
	if got := subHeadings(answer.Segments); strings.Join(got, "|") !=
		"Catholic|Reformed|Historical-critical|"+strings.Join(comparePromptHeadings, "|") {
		t.Errorf("the panel's headings are %v", got)
	}
}

// A declined passage reaches the reader as a declined passage, with Try again
// and not the settings button; Try again asks again. Control: a missing key
// on the re-ask shows the settings button and no Try again.
func TestDeclinedPassageOffersTryAgain(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	errs := make(chan error, 2)
	ctxs := make(chan context.Context, 2)
	prev := aiActionRun
	aiActionRun = func(ctx context.Context, _ *AppState, _, _, _ string, _ selSpan) (string, error) {
		ctxs <- ctx
		return "", <-errs
	}
	t.Cleanup(func() { aiActionRun = prev })
	wait := func(ctx context.Context) {
		t.Helper()
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("the request never settled")
		}
	}
	w := test.NewWindow(widget.NewLabel("reading"))
	defer w.Close()
	st := studyPanelState(t, w)

	errs <- &aiDeclinedError{reason: "SAFETY"}
	showAIPanel(st, aiActionCompare, peterPassage().Text, "", selSpan{})
	wait(nextCtx(t, ctxs))
	p := studyOverlay(t, w)
	if !treeHasText(p, declinedLine) {
		t.Errorf("the panel does not say the passage was declined; texts %v", treeTexts(p))
	}
	if treeHasText(p, "Couldn't reach the AI service. Check your connection and try again.") {
		t.Error("a declined passage reads as a dropped connection")
	}
	again := findTreeButton(p, "Try again")
	if again == nil || findTreeButton(p, "Open AI settings") != nil {
		t.Fatalf("a declined passage should offer Try again alone; texts %v", treeTexts(p))
	}

	info, _ := providerByID(defaultProviderID)
	errs <- &noKeyError{provider: info}
	test.Tap(again)
	wait(nextCtx(t, ctxs))
	if findTreeButton(p, "Open AI settings") == nil || findTreeButton(p, "Try again") != nil {
		t.Errorf("control: a missing key should offer the settings button alone; texts %v", treeTexts(p))
	}
}
