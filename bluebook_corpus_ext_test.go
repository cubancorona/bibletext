package bibletext

import "testing"

// TestBluebookRule5CorpusExpanded is a SECOND, larger batch of real, sourced Bluebook
// Rule 5 examples (harvested from law-library guides, Bluebook quick-references, and
// legal-writing blogs, then adversarially verified) that exercise every quotation
// mechanic the share path must handle:
//   - Rule 5.2 ALTERATIONS — bracketed letters/words and [sic] are PRESERVED verbatim;
//     the app never fabricates or deletes an editorial mark.
//   - Rule 5.3 OMISSIONS — at the beginning (a bracketed capital, never a leading
//     ellipsis), middle, and end (trailing/four-dot ellipses), preserved or added per the
//     mid-sentence-cut rule.
//   - Rule 5.1(b) internal/NESTED quotations.
//   - Rule 5.1(a) the 50-word block threshold (50+ words gets no surrounding marks).
//
// As in the base corpus (bluebook_rule5_test.go), each "in" is a verbatim selection and
// "want" is what formatBibleQuote produces; examples written with STRAIGHT quotes (a
// source artifact, out-of-domain for curly scripture) are left verbatim.
func TestBluebookRule5CorpusExpanded(t *testing.T) {
	cases := []struct{ rule, src, in, want string }{
		// --- alterations: brackets preserved, never fabricated (Rule 5.2) ---
		{"alter", "Temple Law Review  Rul", "[f]ederal courts are courts of limited jurisdiction.", "“[f]ederal courts are courts of limited jurisdiction.”"},
		{"alter", "LegalEase Citations Bl", "[t]he Motion [that] was filed on September 26, 2021.", "“[t]he Motion [that] was filed on September 26, 2021.”"},
		{"alter", "Legal Writing Launch  ", "[T]he court emphasized the importance of precedent.", "“[T]he court emphasized the importance of precedent.”"},
		{"alter", "Legal Writing Launch  ", "[T]he defendant was present at the scene.", "“[T]he defendant was present at the scene.”"},
		{"alter", "The Bluebook Rule  can", "[P]ublic confidence in the [adversary] system depend[s upon] full disclosure of all the facts, within the framework of the rules of evidence.", "“[P]ublic confidence in the [adversary] system depend[s upon] full disclosure of all the facts, within the framework of the rules of evidence.”"},
		{"alter", "Bluebook Citation Help", "it has not [been] shown . . . that the requirement", "“it has not [been] shown . . . that the requirement . . .”"},
		{"alter", "The Blue Book of Gramm", "In my opinion [he] has sufficient vision to drive.", "“In my opinion [he] has sufficient vision to drive.”"},
		{"alter", "Legal Writing Launch  ", "[The defendant] argued that his actions were justified.", "“[The defendant] argued that his actions were justified.”"},
		{"alter", "Legal Writing Launch  ", "He [the defendant] argued that the contract was void.", "“He [the defendant] argued that the contract was void.”"},
		{"alter", "Legal Writing Launch  ", "[The plaintiff] claimed damages for the breach of contract.", "“[The plaintiff] claimed damages for the breach of contract.”"},
		{"alter", "Legal Writing Launch  ", "He [the defendant] was at the scene of the crime.", "“He [the defendant] was at the scene of the crime.”"},
		{"alter", "Legal Writing Launch  ", "The judge stated that [the defendant's] testimony was not credible.", "“The judge stated that [the defendant's] testimony was not credible.”"},
		{"alter", "Legal Writing Launch  ", "The court held that [the defendant's actions] constituted a breach of contract.", "“The court held that [the defendant's actions] constituted a breach of contract.”"},
		{"alter", "Legal Writing Launch  ", "The jurisdiction [of the Supreme Court] extends to all cases . . .", "“The jurisdiction [of the Supreme Court] extends to all cases . . .”"},
		{"alter", "Legal Writing Launch  ", "individual [who] enters the premises without permission.", "“individual [who] enters the premises without permission.”"},
		{"alter", "Legal Writing Launch  ", "The court is satisfied that [the] evidence is sufficient.", "“The court is satisfied that [the] evidence is sufficient.”"},
		{"alter", "Legal Writing Launch  ", "I observed the accused at the scene of the crime . . . holding a weapon and [appearing] agitated.", "“I observed the accused at the scene of the crime . . . holding a weapon and [appearing] agitated.”"},
		{"alter", "Citing and Accessing U", "factors which are peculiarly within [agency] expertise", "“factors which are peculiarly within [agency] expertise . . .”"},
		{"alter", "Citing and Accessing U", "consciously and expressly adopt[] a general policy", "“consciously and expressly adopt[] a general policy . . .”"},
		{"alter", "The Bluebook std ed Ru", "the judgment[] of the lower courts", "“the judgment[] of the lower courts . . .”"},
		{"alter", "The Blue Book of Gramm", "I found their [IT] services invaluable.", "“I found their [IT] services invaluable.”"},
		{"alter", "North Carolina Bar Ass", "[R]ead the original source several times to make sure you understand it. Then use your own words to 'reconstruct' the material without looking at it.", "“[R]ead the original source several times to make sure you understand it. Then use your own words to 'reconstruct' the material without looking at it.”"},
		{"alter", "Mitchell Hamline Law R", "[C]ontracts require consideration . . . .", "“[C]ontracts require consideration . . . .”"},
		{"alter", "LawProse Bryan A Garne", "[Y]ou should at least conclude with a graceful ending . . . .", "“[Y]ou should at least conclude with a graceful ending . . . .”"},
		// --- [sic] preserved verbatim (Rule 5.2) ---
		{"sic", "Legal Writing Launch  ", "principle [sic] objective.", "“principle [sic] objective.”"},
		{"sic", "Legal Writing Launch  ", "He don't [sic] remember what happened that night.", "“He don't [sic] remember what happened that night.”"},
		{"sic", "The Blue Book of Gramm", "They made there [sic] beds.", "“They made there [sic] beds.”"},
		{"sic", "The Blue Book of Gramm", "I can lend you no more then [sic] ten dollars.", "“I can lend you no more then [sic] ten dollars.”"},
		{"sic", "The Blue Book of Gramm", "Who's [sic] turn is it to speak?", "“Who's [sic] turn is it to speak?”"},
		{"sic", "Bar Association of San", "If you'd like to marry me, than [sic] get a job first.", "“If you'd like to marry me, than [sic] get a job first.”"},
		{"sic", "The Maroonbook  The Un", "promote the possibility of lasting peach [sic].", "“promote the possibility of lasting peach [sic].”"},
		// --- beginning omission: bracketed capital, NO leading ellipsis (Rule 5.2/5.3) ---
		{"omit-begin", "Mitchell Hamline  Blue", "[T]here are more mayors of Rockville, Maryland, than there are mayors of Detroit.", "“[T]here are more mayors of Rockville, Maryland, than there are mayors of Detroit.”"},
		{"omit-begin", "Georgia State Universi", "[P]ublic policy forbids such disclosure.", "“[P]ublic policy forbids such disclosure.”"},
		{"omit-begin", "Legal Writing Launch  ", "[T]he evidence was insufficient to merit a conviction.", "“[T]he evidence was insufficient to merit a conviction.”"},
		{"omit-begin", "North Carolina Bar Ass", "[S]tudents do not shed their free speech rights at the schoolhouse door.", "“[S]tudents do not shed their free speech rights at the schoolhouse door.”"},
		// --- middle omission: interior ellipsis preserved (Rule 5.3) ---
		{"omit-middle", "Temple Law Review  D U", "are pleasant . . . and are prompt with their assignments.", "“are pleasant . . . and are prompt with their assignments.”"},
		{"omit-middle", "University of Baltimor", "If the court finds from the pleadings and record that all of the petitioner's claims are frivolous and that it would not be beneficial to continue the proceedings, it may dismiss the petition. . . . However, if the court finds any colorable claim, it is required by Townsend v. Sain, [372 U.S. 293 (1963)], to make a full factual determination before deciding it on its merits.", "If the court finds from the pleadings and record that all of the petitioner's claims are frivolous and that it would not be beneficial to continue the proceedings, it may dismiss the petition. . . . However, if the court finds any colorable claim, it is required by Townsend v. Sain, [372 U.S. 293 (1963)], to make a full factual determination before deciding it on its merits."},
		{"omit-middle", "The Indigo Book  beta ", "The difference between actual and red flag knowledge is . . . between a subjective and an objective standard.", "“The difference between actual and red flag knowledge is . . . between a subjective and an objective standard.”"},
		{"omit-middle", "The Bluebook Online  R", "Omission of a word or words is generally indicated by the insertion of an ellipsis . . . to take the place of the word or words omitted.", "“Omission of a word or words is generally indicated by the insertion of an ellipsis . . . to take the place of the word or words omitted.”"},
		{"omit-middle", "The Blue Book of Gramm", "Four score and seven years ago our fathers brought forth . . . a new nation, conceived in liberty.", "“Four score and seven years ago our fathers brought forth . . . a new nation, conceived in liberty.”"},
		{"omit-middle", "The Blue Book of Gramm", "The happiness of your life depends upon . . . your thoughts.", "“The happiness of your life depends upon . . . your thoughts.”"},
		{"omit-middle", "LegalEase Citations Bl", "Bluebook says that you indicate an omission . . . to take the place of the word or words omitted.", "“Bluebook says that you indicate an omission . . . to take the place of the word or words omitted.”"},
		{"omit-middle", "The Maroonbook  The Un", "The creation of a corporation . . . appertains to sovereignty.", "“The creation of a corporation . . . appertains to sovereignty.”"},
		// --- end omission: trailing/4-dot ellipsis preserved (Rule 5.3) ---
		{"omit-end", "Temple Law Review  D U", "Staffers on the Temple Law Review are pleasant . . . . They . . . get along with one another.", "“Staffers on the Temple Law Review are pleasant . . . . They . . . get along with one another.”"},
		{"omit-end", "Temple Law Review  Red", "[S]taffers on the Temple Law Review are pleasant . . . . They . . . get along with one another.", "“[S]taffers on the Temple Law Review are pleasant . . . . They . . . get along with one another.”"},
		{"omit-end", "LawProse Bryan A Garne", "Rarely will judges relent to those unsubtle suggestions . . . .", "“Rarely will judges relent to those unsubtle suggestions . . . .”"},
		{"omit-end", "LawProse Bryan A Garne", "When the red light comes on, or when you are otherwise informed that your time is up, take at most five or ten seconds to finish your sentence . . . . Don't look yearningly at the presiding judge as if requesting more time.", "“When the red light comes on, or when you are otherwise informed that your time is up, take at most five or ten seconds to finish your sentence . . . . Don't look yearningly at the presiding judge as if requesting more time.”"},
		{"omit-end", "LegalEase Citations Bl", "[B]y the insertion of an ellipsis . . . .", "“[B]y the insertion of an ellipsis . . . .”"},
		{"omit-end", "North Carolina Bar Ass", "The Supreme Court has given great deference to school boards, as in Fraser. . . .", "“The Supreme Court has given great deference to school boards, as in Fraser. . . .”"},
		{"omit-end", "The Indigo Book   publ", "The difference between actual and red flag knowledge is thus not between specific and generalized knowledge . . . .", "“The difference between actual and red flag knowledge is thus not between specific and generalized knowledge . . . .”"},
		{"omit-end", "Columbia Law School Wr", "duty of the judicial department . . . .", "“duty of the judicial department . . . .”"},
		{"omit-end", "Mitchell Hamline Law R", "Every American schoolboy knows that the savage tribes of this continent were deprived of their ancestral ranges by force . . . .", "“Every American schoolboy knows that the savage tribes of this continent were deprived of their ancestral ranges by force . . . .”"},
		{"omit-end", "Mitchell Hamline Law R", "The Court upheld Roe v. Wade . . . .", "“The Court upheld Roe v. Wade . . . .”"},
		{"omit-end", "The Maroonbook  The Un", "I can't remember if I cried . . . . But something touched me deep inside, the day the music died.", "“I can't remember if I cried . . . . But something touched me deep inside, the day the music died.”"},
		// --- sentence/paragraph omission (Rule 5.3); 50+ words stays a block ---
		{"omit-sentence", "Georgetown Law Writing", "Our opinions, like our building, have recognized the role the Decalogue plays in America's heritage. The Executive and Legislative Branches have also acknowledged the historical role of the Ten Commandments. These displays and recognitions of the Ten Commandments bespeak the rich American tradition of religious acknowledgments. . . . . There are, of course, limits to the display of religious messages or symbols. For example, we held unconstitutional a Kentucky statute requiring the posting of the Ten Commandments in every public schoolroom.", "Our opinions, like our building, have recognized the role the Decalogue plays in America's heritage. The Executive and Legislative Branches have also acknowledged the historical role of the Ten Commandments. These displays and recognitions of the Ten Commandments bespeak the rich American tradition of religious acknowledgments. . . . . There are, of course, limits to the display of religious messages or symbols. For example, we held unconstitutional a Kentucky statute requiring the posting of the Ten Commandments in every public schoolroom."},
		{"omit-sentence", "Georgetown Law Writing", "The Plaintiff met her burden. . . . [J]udgment is granted for the Plaintiff.", "“The Plaintiff met her burden. . . . [J]udgment is granted for the Plaintiff.”"},
		{"omit-sentence", "University of Baltimor", "If the court finds from the pleadings and record that all of the petitioner’s claims are frivolous and that it would not be beneficial to continue the proceedings, it may dismiss the petition. . . . However, if the court finds any colorable claim, it is required by Townsend v. Sain, [372 U.S. 293 (1963)], to make a full factual determination before deciding it on its merits.", "If the court finds from the pleadings and record that all of the petitioner’s claims are frivolous and that it would not be beneficial to continue the proceedings, it may dismiss the petition. . . . However, if the court finds any colorable claim, it is required by Townsend v. Sain, [372 U.S. 293 (1963)], to make a full factual determination before deciding it on its merits."},
		{"omit-sentence", "North Carolina Bar Ass", "It is incumbent upon the school, the parents, the students, and the community . . . to work together so that divergent viewpoints . . . may be expressed in a civilized and respectful manner.", "“It is incumbent upon the school, the parents, the students, and the community . . . to work together so that divergent viewpoints . . . may be expressed in a civilized and respectful manner.”"},
		{"omit-sentence", "North Carolina Bar Ass", "The Supreme Court has given great deference to school boards, as in Fraser . . . .", "“The Supreme Court has given great deference to school boards, as in Fraser . . . .”"},
		{"omit-sentence", "Georgetown Law Writing", "Congress shall make no law . . . abridging the freedom of speech . . . .", "“Congress shall make no law . . . abridging the freedom of speech . . . .”"},
		// --- internal/nested quotations (Rule 5.1(b)) ---
		{"nested", "Mitchell Hamline  Blue", "We are a legalized culture. If law is where racism is, then law is where we must confront it . . . . [L]et us present a competing ideology . . . .", "“We are a legalized culture. If law is where racism is, then law is where we must confront it . . . . [L]et us present a competing ideology . . . .”"},
		{"nested", "LegalEase Citations Bl", "that Florida’s sentencing scheme in death penalty cases is unconstitutional because ‘[t]he Sixth Amendment requires a jury, not a judge, to find each fact necessary to impose a sentence of death.’", "“that Florida’s sentencing scheme in death penalty cases is unconstitutional because ‘[t]he Sixth Amendment requires a jury, not a judge, to find each fact necessary to impose a sentence of death.’”"},
		{"nested", "LegalEase Citations Bl", "jury's mere recommendation is not enough.", "“jury's mere recommendation is not enough.”"},
		{"nested", "Citing  Accessing US L", "Past wrongs were evidence bearing on whether there is a real and immediate threat of repeated injury", "“Past wrongs were evidence bearing on whether there is a real and immediate threat of repeated injury . . .”"},
		{"nested", "The Blue Book of Gramm", "Why do you keep saying, 'This doesn't make sense'?", "“Why do you keep saying, 'This doesn't make sense'?”"},
		{"nested", "LegalEase Citations Bl", "[T]he U.S. Supreme Court held 'that Florida's sentencing scheme in death penalty cases is unconstitutional because '[t]he Sixth Amendment requires a jury, not a judge, to find each fact necessary to impose a sentence of death.'", "“[T]he U.S. Supreme Court held 'that Florida's sentencing scheme in death penalty cases is unconstitutional because '[t]he Sixth Amendment requires a jury, not a judge, to find each fact necessary to impose a sentence of death.'”"},
		{"nested", "Citing and Accessing U", "take Care that the Laws be faithfully executed", "“take Care that the Laws be faithfully executed . . .”"},
		{"nested", "The Maroonbook  The Un", "Tribe's analysis of Holmes's language in Schenck, 'The issue is whether Schenck's conduct posed a \"clear and present danger\" of imminent lawless action,' severely misrepresents the doctrine.", "Tribe's analysis of Holmes's language in Schenck, 'The issue is whether Schenck's conduct posed a \"clear and present danger\" of imminent lawless action,' severely misrepresents the doctrine."},
		// --- emphasis & misc. (marks preserved, wrapped) ---
		{"emphasis-paren", "State v Dumlao  Pd   H", "[W]here the language is ambiguous, we are not limited to the words of the statute, but we may look to other aids to statutory construction to assist us in determining legislative intent.", "“[W]here the language is ambiguous, we are not limited to the words of the statute, but we may look to other aids to statutory construction to assist us in determining legislative intent.”"},
		{"emphasis-paren", "Virginia Law Review Sl", "This right of privacy . . . is broad enough to encompass a woman’s decision.", "“This right of privacy . . . is broad enough to encompass a woman’s decision.”"},
		{"emphasis-paren", "Proof That Blog  Empha", "a very short extension of the deadline, but only under the most extraordinary circumstances", "“a very short extension of the deadline, but only under the most extraordinary circumstances . . .”"},
		{"emphasis-paren", "North Carolina Bar Ass", "is manifestly unsupported by reason, or so arbitrary that it could not have been the result of a reasoned decision.", "“is manifestly unsupported by reason, or so arbitrary that it could not have been the result of a reasoned decision.”"},
		// --- inline wrapping (Rule 5.1(a)) ---
		{"block-inline", "Temple Law Review  TLR", "You were born to be hockey players, every one of you, and you were meant to be here tonight.", "“You were born to be hockey players, every one of you, and you were meant to be here tonight.”"},
		{"block-inline", "LawProse Bryan A Garne", "the bane of many a brief and the affliction of many an appellate judge.", "“the bane of many a brief and the affliction of many an appellate judge.”"},
	}
	for _, c := range cases {
		if got := formatBibleQuote(c.in); got != c.want {
			t.Errorf("[%s · %s]\n in:   %q\n got:  %q\n want: %q", c.rule, c.src, c.in, got, c.want)
		}
	}
}
