package bibletext

// Compare perspectives: the study verb that sets a passage's differing
// readings side by side, each with its assumptions in the open, and lets the
// wording of the passage weigh for and against them. One prompt, one answer
// in the ordinary study panel (ai_panel.go), no input sheet, no setting and
// nothing stored. The answer is a fixed skeleton of level-two markdown
// headings the panel's RichText already renders — one section per reading,
// then the four sections named below — so the builder is its own: the prose
// verbs' shared preamble (buildAIPrompt) forbids headings and lists on
// purpose, and this one asks for them.

import "fmt"

// aiStudyVerb is one entry of the Study with AI submenu: the action string
// the shared tail dispatches on and the label the menu shows for it.
type aiStudyVerb struct {
	Action string
	Label  string
}

// aiStudyVerbs is the ordered list every selection menu offers — the Fyne
// pane's submenu, the iPhone and iPad text view, the Mac text view and the
// Android popup — and every menu is held to it, so the surfaces cannot drift.
// Ask is not here: it exists in the code but no menu surfaces it.
var aiStudyVerbs = []aiStudyVerb{
	{Action: aiActionExplain, Label: "Explain"},
	{Action: aiActionContext, Label: "Analyze context"},
	{Action: aiActionTranslation, Label: "Analyze translation"},
	{Action: aiActionCompare, Label: "Compare perspectives"},
}

// The four sections that follow the readings, in the order the prompt asks
// for them. The prompt writes them as "## " headings and the answer scorer
// splits on the same strings, so a heading renamed here is renamed in both.
const (
	compareHeadingAgree     = "Where they agree"
	compareHeadingPart      = "Where they part, and why"
	compareHeadingSettles   = "What the text settles"
	compareHeadingQuestions = "Questions to take back to the text"
)

// compareHeadings is the four in order.
var compareHeadings = []string{
	compareHeadingAgree, compareHeadingPart, compareHeadingSettles, compareHeadingQuestions,
}

// buildComparePrompt is the comparison's prompt: instruction first, passage
// last, as every study prompt is assembled (aiPassageLines). The readings are
// chosen per passage — a church, the Jewish reading, a school of
// interpretation, or a group for whom the passage is a key text — with their
// holders named precisely; each states what it assumes about Scripture, the
// supernatural and authority, with "conservative" and "liberal" defined and
// tied to the assumption that earns the word; no reading is the default, not
// the historical-critical one either; each is put at its strongest, oldest
// first; the passage, not the model, does the judging, in the section named
// by compareHeadingSettles. README.md carries this prompt in full, and a
// test holds the two equal.
func buildComparePrompt(p aiPassage) string {
	const opening = "You are a knowledgeable Bible study assistant helping a reader find out what a " +
		"passage means. The reader has selected a passage and wants to see how it is read " +
		"differently, set side by side, with each reading's assumptions out in the open, so they " +
		"can test the readings against the text for themselves. Write in clear, plain language " +
		"for a general reader."

	const readings = "Choose the three or four readings that differ most on THIS passage (five " +
		"only if it genuinely divides five ways). A reading is a distinct understanding of the " +
		"passage, whoever holds it: a church (Catholic, Eastern Orthodox, Lutheran, Reformed, " +
		"Anglican, Wesleyan, Baptist, Anabaptist, Pentecostal); the Jewish reading of the Hebrew " +
		"text; a school of interpretation (young-earth creationist, old-earth, framework, " +
		"dispensational, covenantal, historical-critical); or a group such as Latter-day Saints " +
		"or Jehovah's Witnesses where the passage is a key text for them. Name the holders " +
		"precisely. Where the passage concerns a miracle, prophecy, authorship or a historical " +
		"claim, include both a reading that takes the claim as true and one that explains it " +
		"naturally, and say which is which. Do not present any reading, including the academic " +
		"historical-critical one, as the neutral or default view: every reading rests on " +
		"assumptions, and the reader needs to see them. If the readings largely agree, say so in " +
		"one sentence and keep every section short."

	const structure = "Use exactly this structure, with these level-two markdown headings, " +
		"oldest reading first, and exactly one sentence before the first heading naming the " +
		"readings you chose:"

	const perReading = "## <Reading>\n" +
		"One section per reading, in this order: what the passage means on this reading; which " +
		"words, grammar or context carry the weight; what it assumes about Scripture (fully true, " +
		"true in faith and morals, or a human record), about the supernatural in the passage " +
		"(event, sign, or story), and about authority (the text alone, tradition, a council or " +
		"confession, or scholarly reconstruction); and who holds it. Where those assumptions make " +
		"a reading conservative or liberal in the theological sense, say so and say which " +
		"assumption makes it so. Here \"conservative\" means holding Scripture to be fully true " +
		"and its supernatural events real; \"liberal\" means treating Scripture as a human record " +
		"and explaining the supernatural naturally. Give each reading the length its substance " +
		"needs, and say plainly when a reading rests on little in the passage itself."

	const agree = "## " + compareHeadingAgree + "\n" +
		"The shared ground."

	const part = "## " + compareHeadingPart + "\n" +
		"The root of each difference, not only the conclusions: a different sense of a word, a " +
		"different grammar, a different assumption about Scripture or the supernatural, or a " +
		"different authority. Short bullets are fine here."

	const settles = "## " + compareHeadingSettles + "\n" +
		"State plainly where the wording, grammar or context of the passage, in the original " +
		"language where it matters, counts for or against a reading, and where it leaves the " +
		"question open. Keep to what the passage itself supports; this is not the place for a " +
		"view of the doctrine."

	const questions = "## " + compareHeadingQuestions + "\n" +
		"Two or three numbered questions the reader can test against the words of the passage " +
		"and the verses around it, and one asking what the reader brought to the passage: which " +
		"reading they expected, and why."

	const accuracy = "Accuracy: describe each reading as its own thoughtful holders would state " +
		"it, in their vocabulary, so that each is presented at its strongest; a weak version of a " +
		"reading is a false report. Do not blend the readings into one view, and do not choose " +
		"between them by preference: the text does the judging, in \"" + compareHeadingSettles +
		"\". Name a confession, catechism, council or writer only when you are sure it speaks to " +
		"this passage. Quote the passage freely, but quote no writer, document or council, and " +
		"invent no page numbers, links or tables. Refer to other Scripture only by book, chapter " +
		"and verse, and only where you are sure of it. Keep the whole answer under 700 words."

	version := fmt.Sprintf("The quoted text is from the %s. Verse numbers may appear inside the quotation.", p.Version)

	return opening + "\n\n" + readings + "\n\n" + structure + "\n\n" +
		perReading + "\n\n" + agree + "\n\n" + part + "\n\n" + settles + "\n\n" + questions + "\n\n" +
		accuracy + "\n\n" + version + "\n\n" + aiPassageLines(p)
}
