package bibletext

import (
	"sort"
	"strings"
	"testing"
)

// The shorthand reader on the shapes the edition prints, each with the id the
// feed would tag it with. The notes are citations only, so the fixtures are
// citations: no verse is quoted. CI has no canon; the same reader is held
// against every tagged citation of a real one by the apparatus census
// (publisher_xrefs_census_test.go).

// shorthandReading spells what readShorthand made of a note: each piece as
// its printed words, "=" and the id it was read to ("?" for a piece that was
// not read), with "@n" after a piece of group n; and the groups as printed.
func shorthandReading(text string) (pieces, groups []string) {
	runes := []rune(text)
	citations, spans := readShorthand(text)
	for _, c := range citations {
		id := c.ID
		if id == "" {
			id = "?"
		}
		piece := string(runes[c.Start:c.End]) + "=" + id
		if c.Group >= 0 {
			piece += "@" + string(rune('0'+c.Group))
		}
		pieces = append(pieces, piece)
	}
	for _, g := range spans {
		groups = append(groups, string(runes[g.Start:g.End]))
	}
	return pieces, groups
}

func TestTheShorthandReadsEveryShapeTheEditionPrints(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		pieces     []string
		groups     []string
	}{
		{"three direct citations and a group",
			"Rom. 5:8; Eph. 2:4; 2 Thess. 2:16; (1 John 4:9, 10; Rev. 1:5)",
			[]string{"Rom. 5:8=ROM.5.8", "Eph. 2:4=EPH.2.4", "2 Thess. 2:16=2TH.2.16",
				"1 John 4:9=1JN.4.9@0", "10=1JN.4.10@0", "Rev. 1:5=REV.1.5@0"},
			[]string{"(1 John 4:9, 10; Rev. 1:5)"}},
		{"a note that is one group",
			"(Is. 9:6)", []string{"Is. 9:6=ISA.9.6@0"}, []string{"(Is. 9:6)"}},
		{"a group before a direct citation",
			"(Lev. 19:31; 20:6); 1 Sam. 28:7",
			[]string{"Lev. 19:31=LEV.19.31@0", "20:6=LEV.20.6@0", "1 Sam. 28:7=1SA.28.7"},
			[]string{"(Lev. 19:31; 20:6)"}},
		{"three groups among two direct citations",
			"(Matt. 15:11; Mark 7:19); Acts 10:28; (Rom. 14:14); 1 Cor. 10:25; (1 Tim. 4:4; Titus 1:15)",
			[]string{"Matt. 15:11=MAT.15.11@0", "Mark 7:19=MRK.7.19@0", "Acts 10:28=ACT.10.28",
				"Rom. 14:14=ROM.14.14@1", "1 Cor. 10:25=1CO.10.25", "1 Tim. 4:4=1TI.4.4@2", "Titus 1:15=TIT.1.15@2"},
			[]string{"(Matt. 15:11; Mark 7:19)", "(Rom. 14:14)", "(1 Tim. 4:4; Titus 1:15)"}},
		{"the book carried into a group from the citation before it",
			"Matt. 5:3–12; (11:5); Luke 6:20–23; (James 2:5)",
			[]string{"Matt. 5:3–12=MAT.5.3-MAT.5.12", "11:5=MAT.11.5@0", "Luke 6:20–23=LUK.6.20-LUK.6.23", "James 2:5=JAS.2.5@1"},
			[]string{"(11:5)", "(James 2:5)"}},
		{"the book carried out of a group, and into the next",
			"(Dan. 2:44; 7:14; Luke 1:33); 11:20; (17:20, 21; 1 John 3:8)",
			[]string{"Dan. 2:44=DAN.2.44@0", "7:14=DAN.7.14@0", "Luke 1:33=LUK.1.33@0", "11:20=LUK.11.20",
				"17:20=LUK.17.20@1", "21=LUK.17.21@1", "1 John 3:8=1JN.3.8@1"},
			[]string{"(Dan. 2:44; 7:14; Luke 1:33)", "(17:20, 21; 1 John 3:8)"}},
		{"a group that opens with a numbered book hands that book on",
			"2 Kin. 9:11; John 10:20; (1 Cor. 1:23; 2:13, 14; 4:10)",
			[]string{"2 Kin. 9:11=2KI.9.11", "John 10:20=JHN.10.20",
				"1 Cor. 1:23=1CO.1.23@0", "2:13=1CO.2.13@0", "14=1CO.2.14@0", "4:10=1CO.4.10@0"},
			[]string{"(1 Cor. 1:23; 2:13, 14; 4:10)"}},
		{"the chapter carried across a comma, to a verse and to a run of verses",
			"Gen. 9:18, 25–27; 10:15",
			[]string{"Gen. 9:18=GEN.9.18", "25–27=GEN.9.25-GEN.9.27", "10:15=GEN.10.15"}, nil},
		{"a comma after a run of verses",
			"(Deut. 4:6–8, 33–38; Heb. 9:13, 14, 18–23)",
			[]string{"Deut. 4:6–8=DEU.4.6-DEU.4.8@0", "33–38=DEU.4.33-DEU.4.38@0",
				"Heb. 9:13=HEB.9.13@0", "14=HEB.9.14@0", "18–23=HEB.9.18-HEB.9.23@0"},
			[]string{"(Deut. 4:6–8, 33–38; Heb. 9:13, 14, 18–23)"}},
		{"a run across chapters",
			"(Col. 3:18—4:1; Eph. 5:22—6:9)",
			[]string{"Col. 3:18—4:1=COL.3.18-COL.4.1@0", "Eph. 5:22—6:9=EPH.5.22-EPH.6.9@0"},
			[]string{"(Col. 3:18—4:1; Eph. 5:22—6:9)"}},
		{"a run of whole chapters, then a run across chapters of the same book",
			"Ex. 25—27; 35:4—36:38",
			[]string{"Ex. 25—27=EXO.25-EXO.27", "35:4—36:38=EXO.35.4-EXO.36.38"}, nil},
		{"a one-chapter book cited with no colon",
			"(Jude 3, 4, 16–19); 2 John 4; 3 John 3; Obad. 10–14; Philem. 16",
			[]string{"Jude 3=JUD.1.3@0", "4=JUD.1.4@0", "16–19=JUD.1.16-JUD.1.19@0",
				"2 John 4=2JN.1.4", "3 John 3=3JN.1.3", "Obad. 10–14=OBA.1.10-OBA.1.14", "Philem. 16=PHM.1.16"},
			[]string{"(Jude 3, 4, 16–19)"}},
		{"whole chapters, the book carried to the second",
			"(1 Cor. 12; 13; Eph. 2:13)",
			[]string{"1 Cor. 12=1CO.12@0", "13=1CO.13@0", "Eph. 2:13=EPH.2.13@0"},
			[]string{"(1 Cor. 12; 13; Eph. 2:13)"}},
		{"a whole chapter between verses of one book",
			"(Ps. 16:9, 10; 22; 132:11)",
			[]string{"Ps. 16:9=PSA.16.9@0", "10=PSA.16.10@0", "22=PSA.22@0", "132:11=PSA.132.11@0"},
			[]string{"(Ps. 16:9, 10; 22; 132:11)"}},
		{"a title citation: the chapter's span, the suffix left to the resolver",
			"Judg. 16:2; Ps. 59:title",
			[]string{"Judg. 16:2=JDG.16.2", "Ps. 59=PSA.59"}, nil},
		{"two titles, the book carried, and one inside a group",
			"Ps. 38:title; 70:title; (Ps. 51:title; 51:1)",
			[]string{"Ps. 38=PSA.38", "70=PSA.70", "Ps. 51=PSA.51@0", "51:1=PSA.51.1@0"},
			[]string{"(Ps. 51:title; 51:1)"}},
		{"a basis is not a piece, and the citation after it reads on",
			"Deut. 32:43, LXX, DSS; Ps. 97:7; 45:4, 5, LXX",
			[]string{"Deut. 32:43=DEU.32.43", "Ps. 97:7=PSA.97.7", "45:4=PSA.45.4", "5=PSA.45.5"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pieces, groups := shorthandReading(tc.text)
			if joined(pieces) != joined(tc.pieces) {
				t.Errorf("%q read as\n  %q\nwant\n  %q", tc.text, pieces, tc.pieces)
			}
			if joined(groups) != joined(tc.groups) {
				t.Errorf("%q: groups %q, want %q", tc.text, groups, tc.groups)
			}
		})
	}
}

// IT DOES NOT GUESS. A piece that is not one of the shapes the feed's tags
// attest is returned unread, and nothing is carried over it: the citation
// after it takes no book and no chapter the print may not mean. Each case
// names the unread pieces; the control beside it is the same note without the
// defect, read whole.
func TestTheShorthandLeavesWhatItCannotReadUnread(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		pieces     []string
	}{
		{"a word where a book should be, and the citation that would carry from it",
			"(Rom. 5:8; see 6:1; 7:2; Eph. 2:4)",
			[]string{"Rom. 5:8=ROM.5.8@0", "see 6:1=?@0", "7:2=?@0", "Eph. 2:4=EPH.2.4@0"}},
		{"CONTROL: the same group without the word",
			"(Rom. 5:8; 6:1; 7:2; Eph. 2:4)",
			[]string{"Rom. 5:8=ROM.5.8@0", "6:1=ROM.6.1@0", "7:2=ROM.7.2@0", "Eph. 2:4=EPH.2.4@0"}},
		{"no book to carry at the start of a note",
			"5:8; Eph. 2:4", []string{"5:8=?", "Eph. 2:4=EPH.2.4"}},
		{"a verse with a letter, and the comma after it",
			"Rom. 5:8a, 9; 6:1", []string{"Rom. 5:8a=?", "9=?", "6:1=?"}},
		{"a book after a comma",
			"Rom. 5:8, Eph. 2:4; 2:5", []string{"Rom. 5:8=ROM.5.8", "Eph. 2:4=?", "2:5=?"}},
		{"a chapter and verse after a comma",
			"Rom. 5:8, 6:1", []string{"Rom. 5:8=ROM.5.8", "6:1=?"}},
		{"a comma after a whole chapter",
			"Lev. 11, 12", []string{"Lev. 11=LEV.11", "12=?"}},
		{"a comma after a run across chapters",
			"Col. 3:18—4:1, 2", []string{"Col. 3:18—4:1=COL.3.18-COL.4.1", "2=?"}},
		{"a run that ends before it starts",
			"Rom. 5:8–3; (Gen. 9:27–25; Ex. 27—25)", []string{"Rom. 5:8–3=?", "Gen. 9:27–25=?@0", "Ex. 27—25=?@0"}},
		{"the wrong dash for the run",
			"Rom. 5:8–6:2; Col. 3:18—25; Ex. 25–27; Jude 3—5",
			[]string{"Rom. 5:8–6:2=?", "Col. 3:18—25=?", "Ex. 25–27=?", "Jude 3—5=?"}},
		{"a hyphen is not a dash",
			"Rom. 5:8-10", []string{"Rom. 5:8-10=?"}},
		{"a title suffix where there is no whole chapter",
			"Ps. 59:1:title; Jude 6:title; Ps. 38—40:title",
			[]string{"Ps. 59:1=?", "Jude 6=?", "Ps. 38—40=?"}},
		{"a nought, a number of four figures, and a full stop",
			"Rom. 0:1; Rom. 5:08; Ps. 1190:1; Rom. 5:8.",
			[]string{"Rom. 0:1=?", "Rom. 5:08=?", "Ps. 1190:1=?", "Rom. 5:8.=?"}},
		{"a basis with no citation to rest on",
			"LXX; Rom. 5:8", []string{"LXX=?", "Rom. 5:8=ROM.5.8"}},
		{"an abbreviation the table does not hold",
			"Rm. 5:8; Romans 5:8; rom. 5:8", []string{"Rm. 5:8=?", "Romans 5:8=?", "rom. 5:8=?"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if pieces, _ := shorthandReading(tc.text); joined(pieces) != joined(tc.pieces) {
				t.Errorf("%q read as\n  %q\nwant\n  %q", tc.text, pieces, tc.pieces)
			}
		})
	}
}

// A group is a pair of parentheses at the top level of the note. One inside
// another is part of the outer group, and one that never closes is no group:
// its citations stand outside every group, where nothing opens them.
func TestTheShorthandFindsGroupsByTheirParentheses(t *testing.T) {
	for _, tc := range []struct {
		text           string
		pieces, groups []string
	}{
		{"(Rom. 5:8; (Eph. 2:4); Gal. 6:15); John 7:50",
			[]string{"Rom. 5:8=ROM.5.8@0", "Eph. 2:4=EPH.2.4@0", "Gal. 6:15=GAL.6.15@0", "John 7:50=JHN.7.50"},
			[]string{"(Rom. 5:8; (Eph. 2:4); Gal. 6:15)"}},
		{"(Rom. 5:8); (Eph. 2:4; Gal. 6:15",
			[]string{"Rom. 5:8=ROM.5.8@0", "Eph. 2:4=EPH.2.4", "Gal. 6:15=GAL.6.15"},
			[]string{"(Rom. 5:8)"}},
		{"Rom. 5:8); Eph. 2:4",
			[]string{"Rom. 5:8=ROM.5.8", "Eph. 2:4=EPH.2.4"}, nil},
		{"", nil, nil},
		{"()", nil, []string{"()"}},
	} {
		pieces, groups := shorthandReading(tc.text)
		if joined(pieces) != joined(tc.pieces) || joined(groups) != joined(tc.groups) {
			t.Errorf("%q read as %q in groups %q; want %q in %q", tc.text, pieces, groups, tc.pieces, tc.groups)
		}
	}
	// CONTROL: the spans are rune offsets, as the feed's tags are. An en dash
	// is three bytes and one rune.
	citations, groups := readShorthand("Matt. 5:3–12; (11:5)")
	if len(citations) != 2 || citations[0].Start != 0 || citations[0].End != 12 || citations[1].Start != 15 || citations[1].End != 19 {
		t.Errorf("spans = %+v; want 0..12 and 15..19 in runes", citations)
	}
	if len(groups) != 1 || groups[0] != (shorthandGroup{Start: 14, End: 20}) {
		t.Errorf("group spans = %+v; want 14..20 in runes", groups)
	}
}

// The table of abbreviations names every book of the canon once, by an id the
// app knows, and the one-chapter books are five of them.
func TestTheShorthandTableNamesEveryBookOnce(t *testing.T) {
	if len(shorthandBooks) != 66 {
		t.Errorf("the table holds %d abbreviations, want one for each of the 66 books", len(shorthandBooks))
	}
	byID := map[string]string{}
	for token, id := range shorthandBooks {
		name := apiBibleBookName(id)
		if name == "" || !protestantCanonBooks[name] {
			t.Errorf("%q is %q, which names no book of the canon", token, id)
		}
		if other, dup := byID[id]; dup {
			t.Errorf("%q and %q are both %q", other, token, id)
		}
		byID[id] = token
		// The abbreviation abbreviates its own book: its letters open the
		// book's name, the figure before them kept ("1 Cor." and "1
		// Corinthians"), save the four the edition shortens otherwise.
		stem := strings.TrimSuffix(token, ".")
		if exception := map[string]string{"Ps.": "Psalms", "Song": "Song of Solomon", "Philem.": "Philemon", "Phil.": "Philippians"}[token]; exception != "" {
			if name != exception {
				t.Errorf("%q is %q, want %q", token, name, exception)
			}
		} else if !strings.HasPrefix(name, stem) {
			t.Errorf("%q does not abbreviate %q", token, name)
		}
	}
	var single []string
	for id := range oneChapterBooks {
		single = append(single, apiBibleBookName(id))
		if byID[id] == "" {
			t.Errorf("the one-chapter book %q has no abbreviation in the table", id)
		}
	}
	sort.Strings(single)
	if got := joined(single); got != "2 John|3 John|Jude|Obadiah|Philemon" {
		t.Errorf("the one-chapter books are %q", got)
	}
}

// THE SHORTHAND PRINTS WHAT IT READS. shorthandPrint is the reverse of the
// reading of a citation that carries its own book; a note that is one
// citation is drawn as its card alone only where the print can be had back
// from the card this way (publisherCrossRef.alone).
func TestTheShorthandPrintsWhatItReads(t *testing.T) {
	for _, c := range []struct {
		id    string
		title bool
		basis string
		want  string
	}{
		{"ROM.5.8", false, "", "Rom. 5:8"},
		{"MAT.3.1-MAT.3.12", false, "", "Matt. 3:1–12"},
		{"GEN.8.20-GEN.9.17", false, "", "Gen. 8:20—9:17"},
		{"LEV.11", false, "", "Lev. 11"},
		{"ISA.36-ISA.39", false, "", "Is. 36—39"},
		{"PSA.59", true, "", "Ps. 59:title"},
		{"JOB.13.16", false, "LXX", "Job 13:16, LXX"},
		{"DEU.32.43", false, "LXX, DSS", "Deut. 32:43, LXX, DSS"},
		{"JUD.1.20", false, "", "Jude 20"},
		{"OBA.1.1-OBA.1.4", false, "", "Obad. 1–4"},
		{"1SA.2.1-1SA.2.10", false, "", "1 Sam. 2:1–10"},
	} {
		got, ok := shorthandPrint(c.id, c.title, c.basis)
		if !ok || got != c.want {
			t.Errorf("%s (title %v, basis %q) prints as %q, %v; want %q", c.id, c.title, c.basis, got, ok, c.want)
			continue
		}
		// And the print reads back to the id: the two are one mapping.
		citations, _ := readShorthand(got)
		if len(citations) != 1 || citations[0].ID != c.id {
			t.Errorf("%q reads back as %+v, want the one citation %s", got, citations, c.id)
		}
	}
	// What the edition does not print is not printed: an id that is no id,
	// a book the table lacks, a run that ends before it starts or in
	// another book, a whole chapter of a book of one chapter, and a title
	// of anything but a whole chapter.
	for _, c := range []struct {
		id    string
		title bool
	}{
		{"", false}, {"ROM", false}, {"ROM.5.8.1", false}, {"XYZ.1.1", false}, {"ROM.0.1", false},
		{"ROM.5.8-ROM.5.3", false}, {"ROM.5.8-ROM.5.8", false}, {"ROM.5.8-GAL.1.1", false}, {"ROM.5-ROM.6.1", false},
		{"JUD.1", false}, {"JUD.2.3", false}, {"PSA.59.1", true}, {"PSA.59-PSA.60", true},
	} {
		if got, ok := shorthandPrint(c.id, c.title, ""); ok {
			t.Errorf("%q (title %v) prints as %q; want no print", c.id, c.title, got)
		}
	}
}
