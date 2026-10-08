package bibletext

// Compare perspectives: the study verb that sets a passage's main readings
// side by side, each with what it assumes in the open, and lets the words of
// the passage weigh for and against them. One prompt, one answer in the
// ordinary study panel (ai_panel.go), no input sheet, no setting and nothing
// stored. The answer is a fixed skeleton of level-two markdown headings the
// panel's RichText already renders — one section per reading, then the four
// sections named below — so the builder is its own: the prose verbs' shared
// preamble (buildAIPrompt) forbids headings and lists on purpose, and this
// one asks for them.

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
// for them. The prompt names them in one sentence and the answer scorer
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
// last, as every study prompt is assembled (aiPassageLines). It is a direct
// instruction and carries no list of churches, schools or groups: the model
// chooses the main readings of this passage and names each group's holders
// as they name themselves. Each reading states what it takes the passage to
// mean, the words it rests on, what it assumes about the Bible and about
// anything supernatural in the passage, and who holds it; where the passage
// is Scripture to more than one faith, each faith's reading is a section of
// its own; where a passage reports something said or done, a reading that
// takes it as reported and one that takes it as a later composition or
// explains it naturally are both set out and told apart; the passage, not
// the model, does the judging, in the section named by
// compareHeadingSettles; and a passage whose readings largely agree says so
// in its first sentence and is brief. The prompt sets no word count: it
// asks for succinctness, as the Explain prompt asks for concision, and
// leaves the length to the answer's substance, since a number was taken as
// a target and cost the readings their breadth. The fairness scorer
// (ai_compare_bench_test.go) holds no length either, beyond a runaway
// guard. Every heading is level two, the readings' and the four fixed ones
// alike, so the panel renders one skeleton. README.md carries this prompt
// in full, and a test holds the two equal.
func buildComparePrompt(p aiPassage) string {
	const voice = "You are a knowledgeable, even-handed Bible study assistant. Write in clear, " +
		"plain language for a general reader, and be succinct."

	const task = "Help the reader understand the passage below honestly: how different believers " +
		"and scholars understand it, why they differ, and what the words of the passage " +
		"themselves support."

	const readings = "Set out the main readings side by side. For each one say what it takes the " +
		"passage to mean, which words it rests on, what it assumes about the Bible and about " +
		"anything supernatural in the passage, and who holds it, calling each group by the name " +
		"it uses for itself. Where the passage is Scripture to more than one faith, give each " +
		"faith's reading its own section. Where the passage reports something said or done, and " +
		"some readers take it as reported while others take it as a later composition or explain " +
		"it naturally, include both and say which is which. Then say where the readings agree, " +
		"where they differ and why, and what the words of the passage settle or leave open. If " +
		"the readings largely agree, say so in your first sentence and be brief. End with two or " +
		"three numbered questions the reader can test against the words of the passage and the " +
		"verses around it, and one asking what they brought to it."

	const headings = "Give each reading a level-two heading, and then, also at level two, the " +
		"headings \"" + compareHeadingAgree + "\", \"" + compareHeadingPart + "\", \"" +
		compareHeadingSettles + "\" and \"" + compareHeadingQuestions + "\"."

	const fairness = "Put each reading as its own thoughtful holders would state it; do not choose " +
		"for the reader, and do not invent quotations or references."

	version := fmt.Sprintf("The quoted text is from the %s. Verse numbers may appear inside the quotation.", p.Version)

	return voice + "\n\n" + task + "\n\n" + readings + "\n\n" + headings + "\n\n" +
		fairness + "\n\n" + version + "\n\n" + aiPassageLines(p)
}
