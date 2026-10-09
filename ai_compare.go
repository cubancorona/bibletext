package bibletext

// Compare perspectives: the study verb that sets a passage's main readings
// side by side, each with what it assumes in the open, and lets the words of
// the passage weigh for and against them. One prompt, one answer in the
// ordinary study panel (ai_panel.go), no input sheet, no setting and nothing
// stored. The answer is a fixed skeleton of level-two markdown headings the
// panel's RichText already renders — one section per reading, then the five
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
// Ask is available only while its refinement gate is enabled.
var aiStudyVerbs = aiStudyVerbsForAsk(askQuestionEnabled)

func aiStudyVerbsForAsk(enabled bool) []aiStudyVerb {
	verbs := []aiStudyVerb{
		{Action: aiActionExplain, Label: "Explain"},
		{Action: aiActionContext, Label: "Analyze context"},
		{Action: aiActionTranslation, Label: "Analyze translation"},
		{Action: aiActionCompare, Label: "Compare perspectives"},
	}
	if enabled {
		verbs = append(verbs, aiStudyVerb{Action: aiActionAsk, Label: "Ask a question"})
	}
	return verbs
}

// The five sections that follow the readings, in the order the prompt asks
// for them. The prompt names them in one sentence and the answer scorer
// splits on the same strings, so a heading renamed here is renamed in both.
const (
	compareHeadingAgree     = "Where they agree"
	compareHeadingPart      = "Where they part, and why"
	compareHeadingTogether  = "Where readings can be held together"
	compareHeadingSettles   = "What the text settles"
	compareHeadingQuestions = "Questions to take back to the text"
)

// compareHeadings is the five in order.
var compareHeadings = []string{
	compareHeadingAgree, compareHeadingPart, compareHeadingTogether, compareHeadingSettles, compareHeadingQuestions,
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
// guard. Every heading is level two, the readings' and the five fixed ones
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
		"where they differ and why, whether any can be held together, and what the words of the passage settle or leave open. If " +
		"the readings largely agree, say so in your first sentence and be brief. End with two or " +
		"three numbered questions the reader can test against the words of the passage and the " +
		"verses around it, and one asking what they brought to it."

	const headings = "Give each reading a level-two heading, and then, also at level two, the " +
		"headings \"" + compareHeadingAgree + "\", \"" + compareHeadingPart + "\", \"" +
		compareHeadingTogether + "\", \"" + compareHeadingSettles + "\" and \"" + compareHeadingQuestions + "\"."

	const compatibility = "Consider whether different readings can be true together, including where they address " +
		"different aspects, levels of meaning, or kinds of explanation. Distinguish a reading that " +
		"adds another meaning or explanation from one that excludes alternatives. Where readings can " +
		"be held together, explain how, which words support that combination, and what assumptions it " +
		"requires. When combining readings, preserve what each actually affirms; do not replace them " +
		"with a weaker compromise. Consider compatible meanings or explanations within a reading as " +
		"well as between readings. For example, a reading may affirm a real event or presence and " +
		"also symbolic meaning; natural mechanisms may coexist with divine agency or purpose. These " +
		"are examples, not an exhaustive set, and a reading that explicitly excludes the additional " +
		"meaning or explanation must be distinguished. If a combination changes a reading’s original " +
		"claim, identify it as a revised position, not as both original readings being true. Where " +
		"claims contradict one another, state the specific conflicting claims clearly; do not force a " +
		"reconciliation. Different labels, methods, genres, or dates of composition are not by " +
		"themselves contradictions. Literary form alone does not deny that an event occurred; " +
		"identify any actual denial before calling the readings incompatible. Compatibility alone " +
		"does not establish that either reading is true."

	const fairness = "Put each reading as its own thoughtful holders would state it; do not choose " +
		"for the reader, and do not invent quotations or references. Quote only wording supplied in the " +
		"passage; describe surrounding verses without quoting them. A scholarly method is not by itself " +
		"a rejection of supernatural action, and a named group need not be uniform in its readings. " +
		"Distinguish what the passage explicitly states from what a reading infers, what it leaves possible, " +
		"and what requires evidence beyond it. Do not treat the narrative’s claims as independent proof " +
		"of an event or its mechanism. Make the closing questions open: do not assume a disputed reading " +
		"is correct or force an either-or where meanings or explanations could coexist."

	version := fmt.Sprintf("The quoted text is from the %s. Verse numbers may appear inside the quotation.", p.Version)

	return voice + "\n\n" + task + "\n\n" + readings + "\n\n" + compatibility + "\n\n" + headings + "\n\n" +
		fairness + "\n\n" + version + "\n\n" + aiPassageLines(p)
}
