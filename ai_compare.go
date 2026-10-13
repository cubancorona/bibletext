package bibletext

// Compare perspectives compares a passage's main readings in the ordinary
// study panel. It uses one selected-passage request and a concise prose prompt
// that permits simple Markdown without prescribing a section structure.

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
		verbs = append(verbs, aiStudyVerb{Action: aiActionAsk, Label: "Ask a question…"})
	}
	return verbs
}

// buildComparePrompt uses the prose study instructions, with optional Markdown
// and a comparison task. The model chooses the relevant readings; the prompt
// supplies no named groups, examples or fixed section structure. README.md
// mirrors the complete prompt.
func buildComparePrompt(p aiPassage) string {
	const preamble = "You are a knowledgeable, even-handed Bible study assistant. " +
		"Write in clear, plain language for a general reader and keep it concise — a " +
		"few short paragraphs at most. " + aiReadingsSentence +
		" Use Markdown for any formatting. Avoid tables and HTML."

	const task = "Compare the main readings of the passage below: what each means, why readers " +
		"differ, and whether any can be held together. Represent each fairly without choosing " +
		"for the reader or forcing agreement. Where readings can be held together, explain how " +
		"without changing what each reading actually claims. If the main readings largely agree, say so."

	return fmt.Sprintf("%s\n\n%s\n\nThe quoted text is from the %s.\n\n%s",
		preamble, task, p.Version, aiPassageLines(p))
}
