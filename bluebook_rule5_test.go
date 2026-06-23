package bibletext

import (
	"strings"
	"testing"
)

// TestBluebookRule5PreservesEditorialMarks is an INDEPENDENT property check (not a
// restatement of the formatting rule): because a share quotes a contiguous, verbatim
// selection, the app must never FABRICATE an *interior* omission or alteration — and
// must never delete one already present. (It DOES add a Rule 5.3 end-of-quote omission
// mark when the selection is cut off mid-sentence; that is tested in bluebook_test.go.)
// Inputs are representative of the harvested corpus.
func TestBluebookRule5PreservesEditorialMarks(t *testing.T) {
	inputs := []string{
		"Standard poodles generally look great in . . . sweaters, and can rock a winter coat.",
		"[T]his presumptive privilege must be considered in light of our historic commitment.",
		"Plaintiff appeal[ed] the trial court's order granting summary judgment.",
		"This list of statutes are [sic] necessarily incomplete.",
		"[The two heirs] never bother about me in life.",
	}
	for _, in := range inputs {
		out := formatBibleQuote(in)
		for _, mark := range []string{"[", "]", "sic"} {
			if strings.Count(out, mark) != strings.Count(in, mark) {
				t.Errorf("the app must not insert or delete %q (Rule 5.2):\n in:  %q\n out: %q", mark, in, out)
			}
		}
	}
}

// Bluebook Rule 5 quotation conformance — a large corpus of REAL, sourced examples,
// harvested from law-library guides and quizzes, then adversarially verified. The
// expected values apply the app's quote rule (balancing, end-omission per Rule 5.3,
// the 50-word block threshold, and double-mark wrapping). Source domain per row.
func TestBluebookRule5Corpus(t *testing.T) {
	cases := []struct{ rule, src, in, want string }{
		// --- block quotations (Rule 5.1 — 50-word threshold) ---
		{"block-quote-threshold", "mitchellhamline.edu", "[T]his presumptive privilege must be considered in light of our historic commitment to the rule of law. This is nowhere more profoundly manifest than in our view that \"the twofold aim [of criminal justice] is that guilt shall not escape or innocence suffer.\" We have elected to employ an adversary system of criminal justice in which the parties contest all issues before a court of law. . . . To ensure that justice is done, it is imperative to the function of courts that compulsory process be available for the production of evidence needed either by the prosecution or by the defense.", "[T]his presumptive privilege must be considered in light of our historic commitment to the rule of law. This is nowhere more profoundly manifest than in our view that \"the twofold aim [of criminal justice] is that guilt shall not escape or innocence suffer.\" We have elected to employ an adversary system of criminal justice in which the parties contest all issues before a court of law. . . . To ensure that justice is done, it is imperative to the function of courts that compulsory process be available for the production of evidence needed either by the prosecution or by the defense."},
		{"block-quote-threshold", "mitchellhamline.edu", "A quotation of fifty or more words should be single spaced, indented on both sides, justified, and without quotation marks. This is known as a block quotation. Quotation marks within a block quotation should appear as they do in the quoted material. The citation following a block quotation should not be indented but should begin at the left margin on the line following the quotation", "A quotation of fifty or more words should be single spaced, indented on both sides, justified, and without quotation marks. This is known as a block quotation. Quotation marks within a block quotation should appear as they do in the quoted material. The citation following a block quotation should not be indented but should begin at the left margin on the line following the quotation . . ."},
		{"block-quote-threshold", "ubalt.edu", "According to the Bluebook rules, any quotation that is 50 words or longer must be formatted as a block quote.", "“According to the Bluebook rules, any quotation that is 50 words or longer must be formatted as a block quote.”"},
		{"block-quote-threshold", "sfbar.org", "[T]his must be pronounced a case of gross and wanton outrage, without any just provocation or excuse …. [The owners] are bound to repair all the real injuries and personal wrongs sustained by the libellants, but they are not bound to the extent of vindictive damages.", "“[T]his must be pronounced a case of gross and wanton outrage, without any just provocation or excuse …. [The owners] are bound to repair all the real injuries and personal wrongs sustained by the libellants, but they are not bound to the extent of vindictive damages.”"},
		{"block-quote-threshold", "sfbar.org", "Occasionally, you will need to quote four or more lines (50 or more words) of text from a key source.", "“Occasionally, you will need to quote four or more lines (50 or more words) of text from a key source.”"},
		{"block-quote-threshold", "blog.legaleasecitations.com", "Melanie's Bluebook Wednesday post today was about block quotes. It was helpful because I've been confused about them. Quoting Rule 5.2, she wrote: \"[Q]uotation marks within a block quotation should appear as they do in the original.\"", "Melanie's Bluebook Wednesday post today was about block quotes. It was helpful because I've been confused about them. Quoting Rule 5.2, she wrote: \"[Q]uotation marks within a block quotation should appear as they do in the original.\""},
		// --- double quotation marks (short, run-in) ---
		{"double-marks", "mitchellhamline.edu", "The question is whether Aranda realized \"the full extent of the risk involved.\"", "The question is whether Aranda realized \"the full extent of the risk involved.\""},
		{"double-marks", "mitchellhamline.edu", "For God so loved the world, that he gave his only begotten Son.", "“For God so loved the world, that he gave his only begotten Son.”"},
		{"double-marks", "lawprose.org", "The sign said, \"No motor vehicles.\"", "The sign said, \"No motor vehicles.\""},
		{"double-marks", "grammarbook.com", "She said, \"Hurry up.\"", "She said, \"Hurry up.\""},
		{"double-marks", "grammarbook.com", "Texas, with a history of rugged individualism, was part of the \"Sagebrush rebellion.\"", "Texas, with a history of rugged individualism, was part of the \"Sagebrush rebellion.\""},
		{"double-marks", "grammarbook.com", "In the Username field, type \"Guest.\"", "In the Username field, type \"Guest.\""},
		{"double-marks", "law.georgetown.edu", "Commas and periods always go inside of the closing quotation mark.", "“Commas and periods always go inside of the closing quotation mark.”"},
		// --- punctuation placement with quotation marks ---
		{"punctuation-inside", "lawprose.org", "Only positive adjectives, such as \"reliable,\" \"kind,\" and \"trustworthy,\" were used to describe the plaintiff.", "Only positive adjectives, such as \"reliable,\" \"kind,\" and \"trustworthy,\" were used to describe the plaintiff."},
		{"punctuation-inside", "grammarbook.com", "The sign changed from \"Walk,\" to \"Don't Walk,\" to \"Walk\" again within 30 seconds.", "The sign changed from \"Walk,\" to \"Don't Walk,\" to \"Walk\" again within 30 seconds."},
		{"punctuation-inside", "guides.library.lls.edu", "I like Aretha's songs \"Respect,\" \"Do Right Woman,\" and \"I Never Loved a Man.\"", "I like Aretha's songs \"Respect,\" \"Do Right Woman,\" and \"I Never Loved a Man.\""},
		{"punctuation-inside", "guides.library.lls.edu", "I don't like \"Respect\"; it's too monotonous.", "I don't like \"Respect\"; it's too monotonous."},
		{"punctuation-inside", "guides.library.lls.edu", "I like \"I Never Loved a Man\": it's a blues ballad in gospel style.", "I like \"I Never Loved a Man\": it's a blues ballad in gospel style."},
		{"punctuation-inside", "lawprose.org", "The attorney asked the witness, \"Did you see the defendant hit her?\"", "The attorney asked the witness, \"Did you see the defendant hit her?\""},
		{"punctuation-inside", "lawprose.org", "Did the victim \"suffer an economic loss as a result of a crime\"?", "Did the victim \"suffer an economic loss as a result of a crime\"?"},
		{"punctuation-inside", "lawprose.org", "She yelled, \"Fire!\"", "She yelled, \"Fire!\""},
		{"punctuation-inside", "lawprose.org", "It's snowing, but the meteorologist this morning forecast \"a warm and sunny day\"!", "It's snowing, but the meteorologist this morning forecast \"a warm and sunny day\"!"},
		{"punctuation-inside", "syntaxis.com", "Don't ever call me a \"crotchety curmudgeon\"!", "Don't ever call me a \"crotchety curmudgeon\"!"},
		{"punctuation-inside", "syntaxis.com", "He yelled, \"You have no right to delete my Oxford commas!\"", "He yelled, \"You have no right to delete my Oxford commas!\""},
		// --- nested quotes (double outside, single inside) ---
		{"nested-quotes", "lawprose.org", "The officer said, \"My partner told Mr. Taylor, 'Please get in the car.'\"", "The officer said, \"My partner told Mr. Taylor, 'Please get in the car.'\""},
		{"nested-quotes", "chicagomanualofstyle.org", "He said, \"Danea said, 'Do not treat me that way.'\"", "He said, \"Danea said, 'Do not treat me that way.'\""},
		{"nested-quotes", "guides.library.lls.edu", "The reviewer said, \"When I asked her where she got such vocal power, she said, 'it was a gift from God.'\"", "The reviewer said, \"When I asked her where she got such vocal power, she said, 'it was a gift from God.'\""},
		{"nested-quotes", "grammarly.com", "Anna wrote: \"In his essay, Steven wrote, 'As Plato once said, \"Be kind, for everyone you meet is fighting a harder battle.\" This is how I feel.' I agree.\"", "Anna wrote: \"In his essay, Steven wrote, 'As Plato once said, \"Be kind, for everyone you meet is fighting a harder battle.\" This is how I feel.' I agree.\""},
		{"nested-quotes", "apastyle.apa.org", "Bliese et al. (2017) noted that \"mobile devices enabled employees in many jobs to work 'anywhere, anytime' and stay electronically tethered to work outside formal working hours\" (p. 391).", "Bliese et al. (2017) noted that \"mobile devices enabled employees in many jobs to work 'anywhere, anytime' and stay electronically tethered to work outside formal working hours\" (p. 391)."},
		{"nested-quotes", "lwionline.org", "The First Circuit has held that \"[p]ersecution normally involves 'severe mistreatment at the hands of [a petitioner's] own government,' but it may also arise where 'non-governmental actors . . . are in league with the government or are not controllable by the government.'\"", "The First Circuit has held that \"[p]ersecution normally involves 'severe mistreatment at the hands of [a petitioner's] own government,' but it may also arise where 'non-governmental actors . . . are in league with the government or are not controllable by the government.'\""},
		{"nested-quotes", "studyguidezone.com", "Robert replied, \"I asked Sally and she said, 'I will not help your group.'\"", "Robert replied, \"I asked Sally and she said, 'I will not help your group.'\""},
		// --- verses/text carrying their own internal marks ---
		{"internal-marks", "mitchellhamline.edu", "When, as here, the plaintiff is a public figure, he cannot recover unless he proves by clear and convincing evidence that the defendant published the defamatory statement with actual malice, i.e., with 'knowledge that it was false or with reckless disregard of whether it was false or not.'", "“When, as here, the plaintiff is a public figure, he cannot recover unless he proves by clear and convincing evidence that the defendant published the defamatory statement with actual malice, i.e., with 'knowledge that it was false or with reckless disregard of whether it was false or not.'”"},
		{"internal-marks", "mitchellhamline.edu", "We refused to permit recovery for choice of language which, though perhaps reflecting a misconception, represented 'the sort of inaccuracy that is commonplace in the forum of robust debate to which the New York Times rule applies.'", "“We refused to permit recovery for choice of language which, though perhaps reflecting a misconception, represented 'the sort of inaccuracy that is commonplace in the forum of robust debate to which the New York Times rule applies.'”"},
		{"internal-marks", "ubalt.edu", "In Arizona v. Evans, this Court reaffirmed the presumption that state court decisions are on the merits rather than on [other] state law grounds, \"to obviate the 'unsatisfactory and intrusive practice of requiring state courts to clarify their decisions to the satisfaction of this Court.'\"", "In Arizona v. Evans, this Court reaffirmed the presumption that state court decisions are on the merits rather than on [other] state law grounds, \"to obviate the 'unsatisfactory and intrusive practice of requiring state courts to clarify their decisions to the satisfaction of this Court.'\""},
		{"internal-marks", "syntaxis.com", "My mother told me, \"In the middle of lunch, Richard looked up 'garrulous' in the dictionary.\"", "My mother told me, \"In the middle of lunch, Richard looked up 'garrulous' in the dictionary.\""},
		{"internal-marks", "grammarist.com", "We don't all have the same 'privilege' as you, Karen exclaimed.", "“We don't all have the same 'privilege' as you, Karen exclaimed.”"},
		// --- quote-mark balancing (dangling open/close) ---
		{"balancing", "legalbluebook.com", "For God so loved the world, that he gave his only begotten Son.”", "“For God so loved the world, that he gave his only begotten Son.”"},
		{"balancing", "legalbluebook.com", "“For God so loved the world, that he gave his only begotten Son.", "“For God so loved the world, that he gave his only begotten Son.”"},
		{"balancing", "lawprose.org", "“Reliable,” “kind,” and “trustworthy”", "“Reliable,” “kind,” and “trustworthy . . .”"},
		{"balancing", "legalbluebook.com", "   For God so loved the world.   ", "“For God so loved the world.”"},
		// --- omissions / ellipses (Rule 5.3: mid-sentence cut adds " . . .") ---
		{"ellipsis-omission", "legalbluebook.com", "An omission of a word or words is generally indicated by the insertion of an ellipsis, three periods separated by spaces and set off by a space before the first and after the last period ('. . .').", "“An omission of a word or words is generally indicated by the insertion of an ellipsis, three periods separated by spaces and set off by a space before the first and after the last period ('. . .').”"},
		{"ellipsis-omission", "cmlawlibraryblog.classcaster.net", "Standard poodles generally look great in . . . sweaters, and can rock the booties, too.", "“Standard poodles generally look great in . . . sweaters, and can rock the booties, too.”"},
		{"ellipsis-omission", "legalbluebook.com", "National borders are less of a barrier . . . now than at almost any other time in history.", "“National borders are less of a barrier . . . now than at almost any other time in history.”"},
		{"ellipsis-omission", "cmlawlibraryblog.classcaster.net", "Standard poodles generally look great in chunky winter sweaters . . . .", "“Standard poodles generally look great in chunky winter sweaters . . . .”"},
		{"ellipsis-omission", "baronofthebluebook.wordpress.com", "[B]orders are less of a barrier to economic exchange now . . .", "“[B]orders are less of a barrier to economic exchange now . . .”"},
		{"ellipsis-omission", "templelawreview.org", "staffers on the Temple Law Review are pleasant . . . . They . . . get along", "“staffers on the Temple Law Review are pleasant . . . . They . . . get along . . .”"},
		{"ellipsis-omission", "ww1.up.edu", "Othello is characterized by '…first, a sense of violent energies and passions … and secondly, a single-mindedness of intention and desire.'", "“Othello is characterized by '…first, a sense of violent energies and passions … and secondly, a single-mindedness of intention and desire.'”"},
		{"ellipsis-omission", "ubalt.edu", "Mr. Moore has requested that Volant extend an offer of employment to him and Volant has agreed to do so, but only if said offer of employment does not violate any non-compete or other restrictive covenants existing between Mr. Moore and CAI.", "“Mr. Moore has requested that Volant extend an offer of employment to him and Volant has agreed to do so, but only if said offer of employment does not violate any non-compete or other restrictive covenants existing between Mr. Moore and CAI.”"},
		// --- alterations / brackets / [sic] (preserved, never auto-inserted) ---
		{"bracket-alteration", "cmlawlibraryblog.classcaster.net", "[P]oodles generally look great in chunky winter sweaters, and can rock the booties, too.", "“[P]oodles generally look great in chunky winter sweaters, and can rock the booties, too.”"},
		{"bracket-alteration", "monmouth.edu", "[T]his presumptive privilege must be considered in light of our historic commitment to the rule of law.", "“[T]his presumptive privilege must be considered in light of our historic commitment to the rule of law.”"},
		{"bracket-alteration", "ubalt.edu", "the drug-addled, intoxicated, 5'10\", 155 pound Johnson [c]ould have [performed] an athletic feat of nearly Olympic proportions [by moving] Klein from the bedroom doorway to the couch", "the drug-addled, intoxicated, 5'10\", 155 pound Johnson [c]ould have [performed] an athletic feat of nearly Olympic proportions [by moving] Klein from the bedroom doorway to the couch . . ."},
		{"bracket-alteration", "michiganlawreview.org", "[The two heirs] never bother about me in life.", "“[The two heirs] never bother about me in life.”"},
		{"bracket-alteration", "michiganlawreview.org", "They [two heirs] never bother about me in life.", "“They [two heirs] never bother about me in life.”"},
		{"bracket-alteration", "blog.legaleasecitations.com", "Plaintiff appeal[ed] the trial court's order granting summary judgment for Defendants.", "“Plaintiff appeal[ed] the trial court's order granting summary judgment for Defendants.”"},
		{"bracket-alteration", "michiganlawreview.org", "The statute require[d] 'best effort[s].'", "“The statute require[d] 'best effort[s].'”"},
		{"bracket-alteration", "michiganlawreview.org", "[T]eachers hated[ ]inconsistent spelling.", "“[T]eachers hated[ ]inconsistent spelling.”"},
		{"bracket-alteration", "michiganlawreview.org", "a special rule[] allowing students to run in the halls.", "“a special rule[] allowing students to run in the halls.”"},
		{"bracket-alteration", "libguides.law.gsu.edu", "This list of statutes are [sic] necessarily incomplete.", "“This list of statutes are [sic] necessarily incomplete.”"},
		{"bracket-alteration", "ww1.up.edu", "…In 1592 [sic] Columbus crossed the Atlantic…", "“…In 1592 [sic] Columbus crossed the Atlantic…”"},
		{"bracket-alteration", "michiganlawreview.org", "Many people love watching football but hate watching soccer.", "“Many people love watching football but hate watching soccer.”"},
		{"bracket-alteration", "michiganlawreview.org", "certain fundamental rights are \"deeply rooted in this Nation's history and tradition\"", "certain fundamental rights are \"deeply rooted in this Nation's history and tradition . . .\""},
		// --- quotation quizzes / practice questions ---
		{"quiz", "syntaxis.com", "Did you hear that Ruth called Robby a 'perpetual plagiarizer'?", "“Did you hear that Ruth called Robby a 'perpetual plagiarizer'?”"},
		{"quiz", "grammarbook.com", "Is it almost over? he asked.", "“Is it almost over? he asked.”"},
		{"quiz", "grammarbook.com", "She screamed, \"I've had it up to here!\"", "She screamed, \"I've had it up to here!\""},
		{"quiz", "syntaxis.com", "Don't ever call me a \"crotchety curmudgeon\"! vs Don't ever call me a \"crotchety curmudgeon!\"", "Don't ever call me a \"crotchety curmudgeon\"! vs Don't ever call me a \"crotchety curmudgeon!\""},
		{"quiz", "syntaxis.com", "My boss called his boss a \"sycophantic opportunist.\"", "My boss called his boss a \"sycophantic opportunist.\""},
		{"quiz", "syntaxis.com", "\"Where are the motorcycle keys?\" he asked.", "\"Where are the motorcycle keys?\" he asked."},
		{"quiz", "studyguidezone.com", "Did he ask you first, \"May I have permission to leave now?\"", "Did he ask you first, \"May I have permission to leave now?\""},
		{"quiz", "studyguidezone.com", "Haven't you heard the old saying, \"Neither a borrower nor a lender be\"?", "Haven't you heard the old saying, \"Neither a borrower nor a lender be\"?"},
		{"quiz", "studyguidezone.com", "What, he asked, do you expect me to do?", "“What, he asked, do you expect me to do?”"},
		{"quiz", "studyguidezone.com", "She asked what time you would be arriving.", "“She asked what time you would be arriving.”"},
		{"quiz", "owl.purdue.edu", "Mary is trying hard in school this semester, her father said.", "“Mary is trying hard in school this semester, her father said.”"},
		{"quiz", "testprepreview.com", "temps, or temporary workers.", "“temps, or temporary workers.”"},
		{"quiz", "owl.purdue.edu", "A Perfect Day for Bananafish is, I believe, J. D. Salinger's best short story.", "“A Perfect Day for Bananafish is, I believe, J. D. Salinger's best short story.”"},
	}
	for _, c := range cases {
		if got := formatBibleQuote(c.in); got != c.want {
			t.Errorf("[%s · %s]\n in:   %q\n got:  %q\n want: %q", c.rule, c.src, c.in, got, c.want)
		}
	}
}
