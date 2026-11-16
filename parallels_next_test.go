//go:build next

package bibletext

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// The Gospel parallels of the next major release (docs/NEXT.md): the
// twenty-two Gospel verses that were in no synopsis set placed
// (gospel_parallels_next.json), and the same saying on another occasion
// (gospel_occasions.json), both put in place by parallels_next.go. The
// shipping build's are in parallels_current_test.go.

// gospelSynopsisColumns is how many Gospel passages the synopsis's sets name:
// 469, then three placed in sets that lacked that Gospel (Matthew 4:23, Luke
// 6:17-19 and 6:43-45) and four sets of one Gospel each for verses that were
// in none. Five columns carry two comma-separated spans (481 spans in all).
const (
	gospelSynopsisPericopes    = 271
	gospelSynopsisSections     = 17
	gospelSynopsisSingleGospel = 159
	gospelSynopsisColumns      = 476
	gospelSynopsisSpans        = 481
)

// gospelVersesInNoSet is empty: every verse of the WEB's Gospels belongs to a
// synopsis set.
var gospelVersesInNoSet []string

// The synopsis and the other-occasion data in force are the next release's.
func TestTheNextReleasesParallelsAreInForce(t *testing.T) {
	for _, f := range []struct {
		path string
		data []byte
	}{
		{"assets/parallels/gospel_parallels_next.json", gospelParallelsJSON},
		{"assets/parallels/gospel_occasions.json", gospelOccasionsJSON},
	} {
		want, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(f.data, want) {
			t.Errorf("the data in force is not %s", f.path)
		}
	}
}

// THE TWENTY-TWO VERSES are placed where Stevens and Burton's harmony prints
// them (docs/TEXTUAL-DATA.md §9.4): three in sets that lacked their Gospel,
// the rest in four sets of one Gospel each, which show no rows but place the
// verses.
func TestTheTwentyTwoVersesArePlacedWhereTheHarmonyPrintsThem(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	for _, ref := range []struct {
		book  string
		ch, v int
		title string
	}{
		{"Matthew", 4, 23, "Preaching tour of Galilee"},
		{"Matthew", 4, 25, "Healing the multitudes by the sea"},
		{"Luke", 6, 17, "Healing the multitudes by the sea"},
		{"Luke", 6, 24, "Woes on the rich and the satisfied"},
		{"Luke", 6, 44, "A tree and its fruit"},
		{"Luke", 21, 38, "Teaching daily in the Temple"},
		{"John", 11, 56, "The Passover draws near"},
		{"John", 13, 34, "The new commandment"},
	} {
		in := false
		for _, p := range gospelPericopes {
			if p.title == ref.title && spansContain(p.spans[ref.book], ref.ch, ref.v) {
				in = true
			}
		}
		if !in {
			t.Errorf("%s %d:%d is not in %q", ref.book, ref.ch, ref.v, ref.title)
		}
	}
}

// labelsOf is the panel's rows of one kind, by label.
func labelsOf(refs []crossRef, kind string) []string {
	var out []string
	for _, c := range refs {
		k := "treasury"
		switch {
		case c.Parallel && c.OtherOccasion:
			k = "other"
		case c.Parallel:
			k = "parallel"
		}
		if k == kind {
			out = append(out, c.label())
		}
	}
	return out
}

// THE EMBEDDED FILE LOADS WHOLE. The loader drops what it cannot read rather
// than guess, so a typo in the file would quietly lose a link; here every
// group, passage and pair the file holds must reach the panel's tables, and
// the totals are pinned to the list the file was built from.
func TestOtherOccasionDataLoadsWhole(t *testing.T) {
	var raw rawOccasions
	if err := json.Unmarshal(gospelOccasionsJSON, &raw); err != nil {
		t.Fatalf("gospel_occasions.json does not parse: %v", err)
	}
	occasionOnce.Do(loadGospelOccasions)
	if len(occasionGroups) != len(raw.Groups) {
		t.Fatalf("%d groups loaded of the file's %d", len(occasionGroups), len(raw.Groups))
	}
	gospelOnce.Do(loadGospelParallels)
	longest := 0
	for _, p := range gospelPericopes {
		longest = max(longest, len(p.title))
	}
	ids := map[string]bool{}
	passages, pairs := 0, 0
	for i, rg := range raw.Groups {
		g := occasionGroups[i]
		if len(g.passages) != len(rg.Passages) || len(g.pairs) != len(rg.Pairs) {
			t.Errorf("group %s: %d of %d passages and %d of %d pairs loaded",
				rg.ID, len(g.passages), len(rg.Passages), len(g.pairs), len(rg.Pairs))
		}
		if ids[rg.ID] {
			t.Errorf("group id %s is used twice", rg.ID)
		}
		ids[rg.ID] = true
		// The title stands where a pericope's does, in a line that does not
		// wrap, so it is held to the longest the synopsis already shows.
		if g.title == "" || len(g.title) > longest {
			t.Errorf("group %s: title %q is empty or longer than the synopsis's longest (%d)", rg.ID, g.title, longest)
		}
		inPair := map[int]bool{}
		seen := map[[2]int]bool{}
		for _, pr := range g.pairs {
			key := [2]int{min(pr[0], pr[1]), max(pr[0], pr[1])}
			if seen[key] {
				t.Errorf("group %s: the pair %v is listed twice", rg.ID, rg.Pairs)
			}
			seen[key] = true
			inPair[pr[0]], inPair[pr[1]] = true, true
		}
		for k, ref := range rg.Passages {
			if !inPair[k] {
				t.Errorf("group %s: %s is in no pair, so no row ever shows it", rg.ID, ref)
			}
		}
		passages += len(g.passages)
		pairs += len(g.pairs)
	}
	if len(raw.Groups) != 102 || passages != 297 || pairs != 274 {
		t.Errorf("the file holds %d groups, %d passages and %d pairs; docs/TEXTUAL-DATA.md section 9 records 102, 297 and 274",
			len(raw.Groups), passages, pairs)
	}
}

// THE TWO KINDS CANNOT BE CONFUSED. A pair under the other-occasion label
// claims two occasions; if one synopsis set already held both passages, the
// panel would call one event two. No pair may lie inside a set.
func TestNoOtherOccasionPairIsOneSynopsisSet(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	occasionOnce.Do(loadGospelOccasions)
	within := func(outer, inner []gSpan) bool {
		for _, in := range inner {
			ok := false
			for _, out := range outer {
				if out.contains(in.ch1, in.v1) && out.contains(in.ch2, in.v2) {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		return len(outer) > 0
	}
	checked := 0
	for _, g := range occasionGroups {
		for _, pr := range g.pairs {
			a, b := g.passages[pr[0]], g.passages[pr[1]]
			checked++
			for _, p := range gospelPericopes {
				if within(p.spans[a.book], a.spans) && within(p.spans[b.book], b.spans) {
					t.Errorf("%q pairs %s %v with %s %v, which the synopsis set %q already joins",
						g.title, a.book, a.spans, b.book, b.spans, p.title)
				}
			}
		}
	}
	// CONTROL: the check can fire. The Lord's Prayer's two passages are not
	// one set, but Matthew 13:9 and Mark 4:9 are (the sower).
	sower := []gPassage{{"Matthew", []gSpan{{13, 9, 13, 9}}}, {"Mark", []gSpan{{4, 9, 4, 9}}}}
	fired := false
	for _, p := range gospelPericopes {
		if within(p.spans[sower[0].book], sower[0].spans) && within(p.spans[sower[1].book], sower[1].spans) {
			fired = true
		}
	}
	if !fired || checked == 0 {
		t.Fatalf("control: the check did not fire on a same-occasion pair (fired %v) or checked nothing (%d)", fired, checked)
	}
}

// Source kinds stay in order without suppressing one another's relationships.
// An overlapping Treasury destination keeps its own selected-verse origin.
func TestOtherOccasionRowsStandBetweenTheParallelsAndTheTreasury(t *testing.T) {
	withCrossRefIndex(t, "Matt.12.25\tLuke.11.17\t50\n"+
		"Matt.12.25\tMark.3.24\t40\n"+
		"Matt.12.25\tLuke.11.14-Luke.11.23\t30\n"+
		"Matt.12.25\tDan.5.1\t20\n")
	bd := xrefBible(map[string]map[int]int{"Matthew": {12: 50}, "Mark": {3: 35}, "Luke": {11: 54}, "Daniel": {5: 31}})
	st := &AppState{Bible: bd, CurrentBook: "Matthew", CurrentChapter: 12, CurrentVersion: "web"}
	refs := crossRefsForSelection(st, "", selSpan{lo: 25, hi: 25})
	var got []string
	for _, c := range refs {
		kind := "treasury"
		switch {
		case c.Parallel && c.OtherOccasion:
			kind = "other"
		case c.Parallel:
			kind = "parallel"
		}
		got = append(got, kind+" "+c.label())
	}
	want := []string{
		"parallel Mark 3:20–30",
		"other Luke 11:17–18",
		"treasury Luke 11:17",
		"treasury Mark 3:24",
		"treasury Luke 11:14–23",
		"treasury Daniel 5:1",
	}
	if strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("Matthew 12:25 lists\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
	for _, c := range refs {
		if c.OtherOccasion && (c.Title != "A kingdom divided against itself" || c.SourceGroup == "" || c.SourceLabel != "Matthew 12:25–26") {
			t.Errorf("the other-occasion row lost its source group: %+v", c)
		}
		if !c.Parallel && (c.SourceVerse != 25 || c.SourceLabel != "Matthew 12:25") {
			t.Errorf("Treasury row lost its selected origin: %+v", c)
		}
	}
}

// THE LABEL. A same-occasion row wears the PARALLEL tag; an other-occasion
// row wears otherOccasionLabel instead, never both; a Treasury row neither.
// And the list keeps the kinds in order even when handed them out of it.
func TestOtherOccasionRowsAreLabelled(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := xrefBible(map[string]map[int]int{"Matthew": {6: 34}, "Luke": {11: 54}, "John": {7: 53}})
	st := &AppState{Bible: bd, CurrentBook: "Luke", CurrentChapter: 11, CurrentVersion: "web"}
	other := crossRef{Book: "Matthew", Chapter: 6, Verse: 9, EndV: 13, Parallel: true, OtherOccasion: true, Title: "A synthetic saying"}
	same := crossRef{Book: "Matthew", Chapter: 6, Verse: 1, Parallel: true, Title: "A synthetic pericope"}
	tsk := crossRef{Book: "John", Chapter: 7, Verse: 50, Votes: 4}
	lst := buildCrossRefList(st, nil, []crossRef{other, tsk, same}, nil, lightPalette, func(crossRef) {})
	if lst.Parallels != 1 || lst.OtherOccasions != 1 || lst.TSKRows != 1 {
		t.Fatalf("wrong source counts: %+v", lst)
	}
	var lines []string
	for _, o := range lst.Objects {
		lines = append(lines, textsIn(o)...)
	}
	text := strings.Join(lines, "\n")
	for _, want := range []string{parallelsLabel, treasuryLabel, "Matthew 6:1", "Matthew 6:9–13", "John 7:50", "A synthetic pericope", "A synthetic saying", otherOccasionLabel} {
		if !strings.Contains(text, want) {
			t.Errorf("missing source label or destination %q", want)
		}
	}
	if strings.Count(text, otherOccasionLabel) != 1 {
		t.Fatal("other-occasion label must appear only on its episode")
	}
}

// MAPPED THROUGH VERSIFICATION, AS THE SYNOPSIS IS. The kingdom woe is the
// WEB's Matthew 23:14 and the BSB's and NKJV's 23:13 (the WEB's 23:13, the
// widows' woe, is absent from the BSB and stands at 23:14 in the NKJV), and
// Luke 11:52 is its other occasion. In every translation each side must find
// the other at its own number, and the widows' woe must find nothing. A
// range with a verse the BSB omits (Mark 9:43-47, without 9:44 and 9:46)
// keeps its range.
func TestOtherOccasionRowsFollowTheTranslationsNumbering(t *testing.T) {
	withCrossRefIndex(t, "")
	bd := xrefBible(map[string]map[int]int{"Matthew": {5: 48, 18: 35, 23: 39}, "Mark": {9: 50}, "Luke": {11: 54}})
	for _, tc := range []struct {
		vid, book string
		ch, v     int
		want      []string
	}{
		{"web", "Luke", 11, 52, []string{"Matthew 23:14"}},
		{"webc", "Luke", 11, 52, []string{"Matthew 23:14"}},
		{"bsb", "Luke", 11, 52, []string{"Matthew 23:13"}},
		{"nkjv", "Luke", 11, 52, []string{"Matthew 23:13"}},
		{"web", "Matthew", 23, 14, []string{"Luke 11:52"}},
		{"bsb", "Matthew", 23, 13, []string{"Luke 11:52"}},
		{"nkjv", "Matthew", 23, 13, []string{"Luke 11:52"}},
		{"web", "Matthew", 23, 13, nil},
		{"nkjv", "Matthew", 23, 14, nil},
		{"web", "Matthew", 5, 29, []string{"Matthew 18:8–9", "Mark 9:43–47"}},
		{"bsb", "Matthew", 5, 29, []string{"Matthew 18:8–9", "Mark 9:43–47"}},
	} {
		st := &AppState{Bible: bd, CurrentBook: tc.book, CurrentChapter: tc.ch, CurrentVersion: tc.vid}
		got := labelsOf(crossRefsForSelection(st, "", selSpan{lo: tc.v, hi: tc.v}), "other")
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s %s %d:%d lists %q on another occasion, want %q", tc.vid, tc.book, tc.ch, tc.v, got, tc.want)
		}
	}
}

// EVERY PASSAGE IN EVERY TRANSLATION. Each passage of the file opens in each
// translation, as one row, and a range keeps its range wherever the
// translation has its last verse: a range that silently became its first
// verse would show the reader a fragment of the saying.
func TestEveryOtherOccasionPassageMapsWhole(t *testing.T) {
	occasionOnce.Do(loadGospelOccasions)
	for _, vid := range []string{"web", "webc", "bsb", "nkjv"} {
		for _, g := range occasionGroups {
			for _, p := range g.passages {
				for _, s := range p.spans {
					c := spanToCrossRef(p.book, s, g.title)
					got := crossRefTargetIn(vid, c)
					if len(got) != 1 {
						t.Errorf("%s: %s (%q) is shown as %d rows", vid, c.label(), g.title, len(got))
						continue
					}
					if c.EndV != 0 && got[0].EndV == 0 && VerseExistsIn(vid, p.book, s.ch2, s.v2) {
						t.Errorf("%s: the range %s (%q) became %s", vid, c.label(), g.title, got[0].label())
					}
				}
			}
		}
	}
}

// Distinct sayings keep their own target ranges, even where one covers
// another. Selecting two verses of the same saying must not duplicate it.
func TestOtherOccasionRowsKeepEachGroupsExactRanges(t *testing.T) {
	withCrossRefIndex(t, "")
	bd := xrefBible(map[string]map[int]int{"Luke": {9: 62, 22: 71}, "Matthew": {20: 34, 23: 39}, "Mark": {9: 50, 10: 52}})
	st := &AppState{Bible: bd, CurrentBook: "Luke", CurrentChapter: 22, CurrentVersion: "web"}
	want := map[string]bool{
		"Luke 22:26 -> Matthew 20:26–27":    true,
		"Luke 22:26 -> Matthew 23:11":       true,
		"Luke 22:26 -> Mark 9:35":           true,
		"Luke 22:26 -> Mark 10:43–44":       true,
		"Luke 22:26 -> Luke 9:48":           true,
		"Luke 22:25–26 -> Matthew 20:25–27": true,
		"Luke 22:25–26 -> Mark 10:42–44":    true,
	}
	for _, span := range []selSpan{{lo: 26, hi: 26}, {lo: 25, hi: 26}} {
		seen := map[string]bool{}
		groups := map[string]string{}
		for _, c := range crossRefsForSelection(st, "", span) {
			if !c.OtherOccasion {
				continue
			}
			key := c.SourceLabel + " -> " + c.label()
			if seen[key] || !want[key] {
				t.Errorf("selection %d–%d emitted repeated or unexpected relation %q", span.lo, span.hi, key)
			}
			seen[key] = true
			if c.SourceGroup == "" {
				t.Errorf("relation %q has no source identity", key)
			}
			if old, exists := groups[c.SourceGroup]; exists && old != c.SourceLabel {
				t.Errorf("source identity %q combines %q and %q", c.SourceGroup, old, c.SourceLabel)
			}
			groups[c.SourceGroup] = c.SourceLabel
		}
		for key := range want {
			if !seen[key] {
				t.Errorf("selection %d–%d lost exact relation %q", span.lo, span.hi, key)
			}
		}
		if len(groups) != 2 {
			t.Errorf("selection %d–%d has %d source groups, want the two distinct sayings", span.lo, span.hi, len(groups))
		}
	}
}

// THE SIX LUKAN DOUBLETS. The synopsis keeps six of Luke's sets apart from
// Matthew's telling of the same words, because the harmonies place them on
// another occasion. Each Luke verse of those sets now reaches Matthew through
// an other-occasion row — Matthew's set, or for a saying Matthew tells
// elsewhere (the lamp at 5:15, treasure in heaven at 6:19-21) the place he
// tells it — and Matthew's set reaches Luke's. Two Luke verses pair with
// nothing in either source: the request for a sign (11:16), answered at
// 11:29, and "Don't be afraid, little flock" (12:32).
func TestTheSixLukanDoubletsAreLinked(t *testing.T) {
	gospelOnce.Do(loadGospelParallels)
	occasionOnce.Do(loadGospelOccasions)
	byTitle := map[string]gPericope{}
	for _, p := range gospelPericopes {
		byTitle[p.title] = p
	}
	unpaired := map[verseRef]bool{{"Luke", 11, 16}: true, {"Luke", 12, 32}: true}
	for _, d := range []struct{ luke, matthew string }{
		{"The Beelzebul controversy (Lukan)", "The Beelzebul controversy"},
		{"The return of the unclean spirit (Lukan)", "The return of the unclean spirit"},
		{"The sign of Jonah (Lukan)", "The sign of Jonah"},
		{"The light of the body (Lukan)", "The light of the body"},
		{"On anxiety (Lukan)", "On anxiety"},
		{"Mustard seed and leaven (Lukan)", "The mustard seed"},
	} {
		luke, ok1 := byTitle[d.luke]
		matt, ok2 := byTitle[d.matthew]
		if !ok1 || !ok2 || len(luke.spans["Luke"]) != 1 || len(matt.spans["Matthew"]) != 1 {
			t.Fatalf("the synopsis no longer holds the sets %q and %q", d.luke, d.matthew)
		}
		if len(luke.spans) != 1 {
			t.Errorf("%q is no longer Luke's alone: %v", d.luke, luke.spans)
		}
		ls := luke.spans["Luke"][0]
		mt := matt.spans["Matthew"][0]
		reachedMatthew := false
		for v := ls.v1; v <= ls.v2; v++ {
			found := false
			for _, c := range gospelOccasionsForVerse("Luke", ls.ch1, v) {
				if c.Book == "Matthew" {
					found = true
				}
			}
			if unpaired[verseRef{"Luke", ls.ch1, v}] {
				if found {
					t.Errorf("Luke %d:%d is now paired; update the test's note", ls.ch1, v)
				}
				continue
			}
			if !found {
				t.Errorf("Luke %d:%d (%s) reaches nothing in Matthew", ls.ch1, v, d.luke)
			}
		}
		for ch := mt.ch1; ch <= mt.ch2; ch++ {
			for v := 1; v <= webGospelChapterEnds["Matthew"][ch-1]; v++ {
				if !mt.contains(ch, v) {
					continue
				}
				for _, c := range gospelOccasionsForVerse("Matthew", ch, v) {
					if c.Book == "Luke" && ls.contains(c.Chapter, c.Verse) {
						reachedMatthew = true
					}
				}
			}
		}
		if !reachedMatthew {
			t.Errorf("no verse of Matthew's %q reaches Luke's %q", d.matthew, d.luke)
		}
	}
	// The leaven is a set of its own in Matthew.
	found := false
	for _, c := range gospelOccasionsForVerse("Luke", 13, 20) {
		if c.label() == "Matthew 13:33" {
			found = true
		}
	}
	if !found {
		t.Error("Luke 13:20 does not reach the leaven, Matthew 13:33")
	}
}

// THE NAMED PAIRS. The sayings the second kind was made for, each linked
// both ways.
func TestTheNamedOtherOccasionPairs(t *testing.T) {
	for _, p := range []struct {
		book  string
		ch, v int
		want  string
	}{
		{"Luke", 11, 2, "Matthew 6:9–13"}, // the Lord's Prayer
		{"Matthew", 6, 9, "Luke 11:2–4"},
		{"Luke", 11, 9, "Matthew 7:7–11"},   // ask, seek, knock
		{"Luke", 15, 4, "Matthew 18:12–14"}, // the lost sheep
		{"Matthew", 18, 12, "Luke 15:4–7"},
		{"Luke", 12, 42, "Matthew 24:45–51"}, // the faithful servant
		{"Luke", 13, 34, "Matthew 23:37–39"}, // the lament over Jerusalem
		{"Matthew", 23, 37, "Luke 13:34–35"},
		{"Luke", 8, 16, "Luke 11:33"}, // the lamp, twice in Luke
		{"Luke", 11, 33, "Luke 8:16"},
	} {
		var got []string
		for _, c := range gospelOccasionsForVerse(p.book, p.ch, p.v) {
			got = append(got, c.label())
		}
		if !strings.Contains(strings.Join(got, "; ")+";", p.want+";") {
			t.Errorf("%s %d:%d lists %q on another occasion, want %s among them", p.book, p.ch, p.v, got, p.want)
		}
	}
	// A verse is never its own other occasion.
	eachWEBGospelVerse(func(book string, ch, v int) {
		for _, c := range gospelOccasionsForVerse(book, ch, v) {
			if c.Book == book && c.covers(crossRef{Book: book, Chapter: ch, Verse: v}) {
				t.Errorf("%s %d:%d lists its own passage %s", book, ch, v, c.label())
			}
		}
	})
}

// Every selection of up to six verses, and every whole Gospel chapter,
// retains the union of its verses' source relationships. Only repeated hits
// within the same source group and exact destination collapse; overlapping
// ranges and distinct source identities remain separate.
func TestGospelSelectionsRetainEachSourceGroupsExactDestinations(t *testing.T) {
	withCrossRefIndex(t, "")
	selections, relations := 0, 0
	for _, id := range []string{"web", "bsb"} {
		bd := gospelEditionBible(id)
		for _, g := range gospelColumns {
			for _, ch := range bd.GetChapterNumbersForBook(g.book) {
				verses := bd.GetChapter(g.book, ch)
				if len(verses) == 0 {
					t.Fatalf("control: %s %s %d has no verses", id, g.book, ch)
				}
				st := &AppState{Bible: bd, CurrentBook: g.book, CurrentChapter: ch, CurrentVersion: id}
				own := make([]map[string]crossRef, len(verses))
				for i, v := range verses {
					own[i] = map[string]crossRef{}
					for _, c := range crossRefsForSelection(st, "", selSpan{lo: v.Verse, hi: v.Verse}) {
						if !c.Parallel || c.SourceGroup == "" || c.SourceLabel == "" {
							t.Fatalf("%s %s %d:%d lost source metadata: %+v", id, g.book, ch, v.Verse, c)
						}
						key := c.SourceGroup + "\x00" + c.label()
						if _, duplicate := own[i][key]; duplicate {
							t.Fatalf("%s %s %d:%d repeated relation %q", id, g.book, ch, v.Verse, key)
						}
						own[i][key] = c
						relations++
					}
				}
				check := func(lo, hi int) {
					selections++
					where := fmt.Sprintf("%s %s %d:%d–%d", id, g.book, ch, verses[lo].Verse, verses[hi].Verse)
					want := map[string]crossRef{}
					for i := lo; i <= hi; i++ {
						for key, c := range own[i] {
							want[key] = c
						}
					}
					seen := map[string]bool{}
					for _, c := range crossRefsForSelection(st, "", selSpan{lo: verses[lo].Verse, hi: verses[hi].Verse}) {
						key := c.SourceGroup + "\x00" + c.label()
						if seen[key] {
							t.Fatalf("%s duplicated relation %q", where, key)
						}
						seen[key] = true
						if w, ok := want[key]; !ok || c != w {
							t.Fatalf("%s added or changed relation %+v; expected %+v, present=%v", where, c, w, ok)
						}
					}
					if len(seen) != len(want) {
						for key := range want {
							if !seen[key] {
								t.Errorf("%s lost exact source relation %q", where, key)
							}
						}
					}
				}
				for lo := range verses {
					for hi := lo; hi < len(verses) && hi < lo+6; hi++ {
						check(lo, hi)
					}
				}
				check(0, len(verses)-1)
			}
		}
	}
	if selections < 40000 || relations < 1000 {
		t.Errorf("control: checked %d selections and %d single-verse relations", selections, relations)
	}
}
