package bibletext

// The comparison verb's contract: its prompt is a direct instruction with no
// list of churches, schools or groups, carries the four section headings in
// order and the passage last; the four prose prompts name differing readings
// in one shared sentence and still forbid headings; README.md mirrors every
// prompt word for word; the action string is dispatched by an allow-list;
// the request is filed under its own cache scope; the panel titles it
// "Perspectives" and renders its headings; and a provider's refusal reaches
// the reader as a declined passage with Try again.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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

// The control answer is the skeleton the prompt asks for, rendered: four
// readings followed by the four sections in order as headings; it quotes the
// passage's own words and hangs no conservative or liberal label on a
// reading, which the prompt no longer asks for; and it is well inside the
// fairness scorer's runaway guard
// (ai_compare_bench_test.go), the one length the scorer holds, the prompt
// setting none. Control: the same text with its last heading renamed is not
// the skeleton.
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
	for _, run := range []string{"gates of Hades", "will not prevail"} {
		if !strings.Contains(md, run) {
			t.Errorf("the control answer does not quote %q from the passage", run)
		}
	}
	if m := regexp.MustCompile(`(?i)\b(conservative|liberal)\b`).FindString(md); m != "" {
		t.Errorf("the control answer labels a reading %q", m)
	}
	if n := answerWords(md); n > compareBenchRunawayWords {
		t.Errorf("the control answer is %d words, over the runaway guard of %d", n, compareBenchRunawayWords)
	}
	renamed := strings.Replace(md, "## "+compareHeadingSettles, "## Conclusion", 1)
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
	"Where readings can be held together",
	"What the text settles",
}

// The prompt is a direct instruction: it names no church, school or group,
// none of the readings answers have named, and no theological label, for the
// model to fill a slot with; it asks for the readings with their
// assumptions, for each faith's reading as its own section where the
// passage is Scripture to more than one faith (and so names no faith, not
// Judaism on a Hebrew Bible passage), for both a reading that takes what the
// passage reports as said or done as reported and one that takes it as a
// later composition, for the agreement, the differences and what the words
// settle, for every heading at level two and the four fixed ones by name,
// and for honesty, fairness and no invented quotation; succinct, and brief
// where the readings largely agree, said in
// the first sentence, with no word count anywhere: a number was taken as a
// target and cost the readings their breadth. Control: the Explain prompt
// is a different prompt and carries no heading.
func TestBuildComparePrompt(t *testing.T) {
	psg := peterPassage()
	got := buildComparePrompt(psg)
	for _, absent := range []string{
		"Catholic", "Eastern Orthodox", "Orthodox", "Lutheran", "Reformed", "Jewish",
		"Jehovah's Witnesses", "historical-critical", "conservative", "liberal",
		"within Judaism", "Questions to take back to the text", "numbered questions", "closing questions",
	} {
		if strings.Contains(strings.ToLower(got), strings.ToLower(absent)) {
			t.Errorf("the comparison prompt contains %q", absent)
		}
	}
	for _, want := range []string{
		"Help the reader understand the passage below honestly: how different believers and scholars understand it, why they differ, and what the words of the passage themselves support.",
		"what it assumes about the Bible and about anything supernatural in the passage",
		"calling each group by the name it uses for itself",
		"Where the passage is Scripture to more than one faith, give each faith's reading its own section.",
		"Where the passage reports something said or done, and some readers take it as reported while others take it as a later composition or explain it naturally, include both and say which is which.",
		"Then say where the readings agree, where they differ and why, whether any can be held together, and what the words of the passage settle or leave open.",
		"If the readings largely agree, say so in your first sentence and be brief.",
		"Write in clear, plain language for a general reader, and be succinct.",
		"Put each reading as its own thoughtful holders would state it; do not choose for the reader, and do not invent quotations or references.",

		"Consider whether different readings can be true together, including where they address different aspects, levels of meaning, or kinds of explanation.",
		"Distinguish a reading that adds another meaning or explanation from one that excludes alternatives.",
		"Where readings can be held together, explain how, which words support that combination, and what assumptions it requires.",
		"When combining readings, preserve what each actually affirms; do not replace them with a weaker compromise.",
		"Consider compatible meanings or explanations within a reading as well as between readings. For example, a reading may affirm a real event or presence and also symbolic meaning; natural mechanisms may coexist with divine agency or purpose. These are examples, not an exhaustive set, and a reading that explicitly excludes the additional meaning or explanation must be distinguished. If a combination changes a reading’s original claim, identify it as a revised position, not as both original readings being true. Where claims contradict one another, state the specific conflicting claims clearly; do not force a reconciliation. Different labels, methods, genres, or dates of composition are not by themselves contradictions. Literary form alone does not deny that an event occurred; identify any actual denial before calling the readings incompatible.",
		"For example, a reading may affirm a real event or presence and also symbolic meaning; natural mechanisms may coexist with divine agency or purpose.",
		"These are examples, not an exhaustive set, and a reading that explicitly excludes the additional meaning or explanation must be distinguished.",
		"Compatibility alone does not establish that either reading is true.",
		"Quote only wording supplied in the passage; describe surrounding verses without quoting them.",
		"A scholarly method is not by itself a rejection of supernatural action, and a named group need not be uniform in its readings.",
		"Distinguish what the passage explicitly states from what a reading infers, what it leaves possible, and what requires evidence beyond it.",
		"Do not treat the narrative’s claims as independent proof of an event or its mechanism.",
		"The quoted text is from the World English Bible. Verse numbers may appear inside the quotation.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the comparison prompt lacks %q", want)
		}
	}
	for _, limit := range []string{"under 600", "under 250", "under 700", "under 300"} {
		if strings.Contains(got, limit) {
			t.Errorf("the comparison prompt sets a word count: %q", limit)
		}
	}
	if m := regexp.MustCompile(`(?i)\b(?:under|within|at most|no more than|up to) \d+\b|\b\d+ words\b`).FindString(got); m != "" {
		t.Errorf("the comparison prompt sets a word count: %q", m)
	}

	// The skeleton: a level-two heading per reading, then the four headings
	// named in one sentence, in order, at level two as well, from the same
	// constants the scorer splits on. The prompt writes no heading line of
	// its own.
	if len(compareHeadings) != len(comparePromptHeadings) {
		t.Fatalf("the comparison has %d fixed headings, want %d", len(compareHeadings), len(comparePromptHeadings))
	}
	for i, h := range comparePromptHeadings {
		if compareHeadings[i] != h {
			t.Errorf("compareHeadings[%d] = %q, want %q", i, compareHeadings[i], h)
		}
	}
	sentence := `Give each reading a level-two heading, and then, also at level two, the headings "` +
		strings.Join(comparePromptHeadings[:3], `", "`) + `" and "` + comparePromptHeadings[3] + `".`
	if strings.Count(got, sentence) != 1 {
		t.Errorf("the prompt does not name the four headings once in order, at level two:\n%s", got)
	}
	if strings.Count(got, "level-two") != 1 || strings.Count(got, "level two") != 1 {
		t.Errorf("the prompt should set level two once for the readings and once for the fixed headings:\n%s", got)
	}
	if strings.Contains(got, "\n## ") {
		t.Errorf("the prompt writes a heading line of its own:\n%s", got)
	}

	// Instruction first, passage last.
	if !strings.HasPrefix(got, "You are a knowledgeable, even-handed Bible study assistant") {
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
	if strings.Contains(explain, "Where they agree") {
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

// The fixed study verbs retain their order; Ask is appended only with its
// refinement gate. Every offered action has a prompt.
func TestAIStudyVerbs(t *testing.T) {
	want := []aiStudyVerb{
		{aiActionExplain, "Explain"},
		{aiActionContext, "Analyze context"},
		{aiActionTranslation, "Analyze translation"},
		{aiActionCompare, "Compare perspectives"},
	}
	if askQuestionEnabled {
		want = append(want, aiStudyVerb{aiActionAsk, "Ask a question…"})
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
const compareFixture = "## Catholic\n" +
	"Peter himself is the rock: Jesus gives Simon a new name and builds on him, so the assembly rests on Peter's person and office. " +
	"The weight falls on the wordplay between Peter and rock read as one and the same. It takes the words as Jesus' own words and the " +
	"promise as a real act of Christ, holds the Bible fully true, and reads it with tradition. Held by the Catholic Church.\n\n" +
	"## Reformed\n" +
	"The rock is Christ confessed by Peter, who speaks for all the apostles; no single office is founded here. The weight falls on the " +
	"change of word and on Scripture read with Scripture. It takes the words as Jesus' own words, holds the Bible fully true and the " +
	"text alone as its authority. Held by Reformed and Presbyterian churches.\n\n" +
	"## Historical-critical\n" +
	"The saying is read as a later composition, Matthew's community reflecting on Peter's standing. The weight falls on Matthew alone " +
	"using the word assembly. It treats the Gospel as a human record and the promise as the community's hope, with scholarly " +
	"reconstruction as its authority; scholars are divided among themselves.\n\n" +
	"## Where they agree\n" +
	"All three read Peter as spokesman for the confession, the assembly as the one people Jesus gathers, and the gates of Hades as " +
	"the power of death.\n\n" +
	"## Where they part, and why\n" +
	"- Whether Peter and rock name the same thing.\n" +
	"- Whether what is founded is an office that passes on, or the confession as such.\n" +
	"- Whether the words are Jesus' own or the community's: an assumption about the Gospel itself.\n\n" +
	"## Where readings can be held together\n" +
	"Peter and his confession can both be foundational if the promise addresses him as the confessing apostle; this does not settle succession.\n\n" +
	"## What the text settles\n" +
	"The wordplay is in the text, and the verse before it is the confession; the passage says nothing of succession either way.\n"

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
		// The callback must finish before the test closes its app. The
		// test run's timeout bounds a genuinely blocked completion.
		<-ctx.Done()
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
