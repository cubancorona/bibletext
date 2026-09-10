package bibletext

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// A fixture that carries every rotation entry as a real verse (text "x"), so
// the whole sequence resolves; with holes, a list of entries to leave out.
func rotationFixture(holes ...dayPassage) *BibleData {
	bd := NewBibleData()
	seen := map[string]bool{}
	skip := map[dayPassage]bool{}
	for _, h := range holes {
		skip[h] = true
	}
	for _, p := range verseOfDayRefs {
		if !seen[p.Book] {
			seen[p.Book] = true
			bd.Books = append(bd.Books, p.Book)
			bd.Verses[p.Book] = map[int][]Verse{}
		}
		if skip[p] {
			continue
		}
		for n := p.Lo; n <= p.hi(); n++ {
			bd.Verses[p.Book][p.Chapter] = append(bd.Verses[p.Book][p.Chapter],
				Verse{BookName: p.Book, Chapter: p.Chapter, Verse: n, Text: "x"})
		}
	}
	return bd
}

func TestVerseOfDayRefsAreWellFormed(t *testing.T) {
	if len(verseOfDayRefs) == 0 {
		t.Fatal("verseOfDayRefs is empty")
	}
	for _, r := range verseOfDayRefs {
		if r.Book == "" || r.Chapter < 1 || r.Lo < 1 || (r.Hi != 0 && r.Hi <= r.Lo) {
			t.Errorf("malformed entry %+v", r)
		}
		if r.hi()-r.Lo > 4 {
			t.Errorf("%s runs to %d verses; the card is for a passage, not a reading", r.key(), r.hi()-r.Lo+1)
		}
	}
	entries := map[dayPassage]bool{}
	for _, r := range verseOfDayRefs {
		entries[r] = true
	}
	for from, to := range verseOfDayAlternates {
		if !entries[from] {
			t.Errorf("alternate for %s, which is not an entry (write the key exactly as the list does)", from.key())
		}
		if entries[to] {
			t.Errorf("the alternate %s is itself an entry, so it would come round twice", to.key())
		}
	}
}

func TestVerseOfDayRefsAreUniqueAndAmple(t *testing.T) {
	seen := map[dayPassage]bool{}
	verses := map[string]string{}
	for _, r := range verseOfDayRefs {
		if seen[r] {
			t.Errorf("duplicate entry %s — a repeat wastes a slot in the daily rotation", r.key())
		}
		seen[r] = true
		for n := r.Lo; n <= r.hi(); n++ {
			at := dayPassage{r.Book, r.Chapter, n, 0}.key()
			if other, dup := verses[at]; dup {
				t.Errorf("%s is inside both %s and %s", at, other, r.key())
			}
			verses[at] = r.key()
		}
	}
	// A year's worth is the whole point; guard against an accidental truncation.
	if n := len(verseOfDayRefs); n < 300 {
		t.Errorf("verseOfDayRefs shrank to %d; expected a full-year rotation", n)
	}
}

// THE DAY KEY ACROSS THE CLOCK CHANGE. Two consecutive civil dates must always
// be one day apart. The control at the end re-computes the formula this
// replaced — the Unix day of local midnight — and shows it repeating a day
// after the spring change and skipping one after the autumn change in
// London; the mutation "time.UTC -> now.Location()" in verseOfDayNumber is
// exactly that formula, and this test fails with +0 and +2 under it.
func TestVerseOfDayNumberAdvancesByOneAcrossDST(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Skip("no zoneinfo on this host: " + err.Error())
	}
	spans := [][2]time.Time{
		{time.Date(2026, 3, 28, 9, 0, 0, 0, london), time.Date(2026, 3, 31, 9, 0, 0, 0, london)},
		{time.Date(2026, 10, 24, 9, 0, 0, 0, london), time.Date(2026, 10, 27, 9, 0, 0, 0, london)},
	}
	old := func(now time.Time) int64 {
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()).Unix() / 86400
	}
	oldBroke := false
	for _, sp := range spans {
		prev, prevOld := verseOfDayNumber(sp[0]), old(sp[0])
		for day := sp[0].AddDate(0, 0, 1); !day.After(sp[1]); day = day.AddDate(0, 0, 1) {
			if n := verseOfDayNumber(day); n-prev != 1 {
				t.Errorf("%s: day number moved by %d, not 1", day.Format("Mon 2 Jan 2006"), n-prev)
			}
			prev = verseOfDayNumber(day)
			if o := old(day); o-prevOld != 1 {
				oldBroke = true
			}
			prevOld = old(day)
		}
	}
	if !oldBroke {
		t.Error("the formula this replaced advances by one on these dates too, so this test " +
			"cannot tell the two apart — pick dates that straddle a clock change")
	}
}

// THE ORDER IS FROZEN. Changing the seed, the hash, the finalizer or the key
// format re-deals every reader's year. If that is intended, update these
// slots in the same change and say so in the message; if it is not, this is
// the test that caught it.
func TestVerseOfDayOrderIsFrozen(t *testing.T) {
	golden := []string{
		"Psalms 103:2-5", "1 John 5:4-5", "Wisdom 11:24-26", "Colossians 3:12-13", "Psalms 16:11",
		"Psalms 19:14", "John 4:13-14", "Matthew 7:7", "1 Corinthians 2:9", "John 10:10",
		"2 Samuel 22:31", "2 Corinthians 3:18", "Romans 12:1-2", "Isaiah 6:8", "Nehemiah 8:10",
		"Ephesians 4:32", "1 Corinthians 13:13", "Galatians 2:20", "1 Timothy 6:12", "Micah 7:7",
	}
	if n := len(verseOfDaySequence); n != 329 {
		t.Errorf("the rotation has %d entries, not 329: every reader's day moves once. If that is "+
			"intended, update this number and regenerate the snapshot", n)
	}
	for i, want := range golden {
		if i >= len(verseOfDaySequence) {
			break
		}
		if got := verseOfDaySequence[i].key(); got != want {
			t.Errorf("slot %d is %s, golden says %s — the order has been re-dealt", i, got, want)
		}
	}
}

// THE ORDER SPREADS THE BOOKS. The control measures the canonical order the
// file is written in and shows it failing the same bars, so a sequence that
// silently reverted to source order could not pass.
func TestVerseOfDayOrderSpreadsBooks(t *testing.T) {
	stats := func(seq []dayPassage) (sameBook, adjacent, longest int) {
		n := len(seq)
		run := 1
		longest = 1
		for i := range seq {
			a, b := seq[i], seq[(i+1)%n]
			if a.Book != b.Book {
				run = 1
				continue
			}
			sameBook++
			run++
			if run > longest {
				longest = run
			}
			if a.Chapter == b.Chapter && (b.Lo == a.hi()+1 || a.Lo == b.hi()+1) {
				adjacent++
			}
		}
		return
	}
	sameBook, adjacent, longest := stats(verseOfDaySequence)
	n := len(verseOfDaySequence)
	if 10*sameBook > n {
		t.Errorf("%d of %d consecutive days stay in one book (limit: one in ten)", sameBook, n)
	}
	if adjacent != 0 {
		t.Errorf("%d consecutive days are adjacent verses of one chapter — the split-passage "+
			"problem the order exists to remove", adjacent)
	}
	if longest > 4 {
		t.Errorf("the longest run in one book is %d days", longest)
	}
	if cb, ca, _ := stats(verseOfDayRefs); 10*cb <= n || ca == 0 {
		t.Errorf("the canonical order passes these bars too (%d same-book, %d adjacent), so they "+
			"prove nothing about the sequence", cb, ca)
	}
}

// ONE MISSING ENTRY MOVES ONE DAY, NOT THE YEAR. The index runs over the whole
// list; only the day whose entry is missing falls back. Under the compacted
// index this replaced, an edition lacking a single entry disagreed with every
// other edition on almost every day — the control at the end shows that
// count for the same fixture.
func TestOneMissingEntryDoesNotRetimeTheRotation(t *testing.T) {
	hole := verseOfDaySequence[1]
	full := &AppState{Bible: rotationFixture()}
	holed := &AppState{Bible: rotationFixture(hole)}
	agree, days := 0, int64(400)
	for day := int64(0); day < days; day++ {
		a, okA := verseOfTheDayAt(full, day)
		b, okB := verseOfTheDayAt(holed, day)
		if !okA || !okB {
			t.Fatalf("day %d: no pick (%v, %v)", day, okA, okB)
		}
		if a.reference() == b.reference() {
			agree++
		} else if a.Book != hole.Book || a.Chapter != hole.Chapter || a.Lo != hole.Lo {
			t.Errorf("day %d: the editions differ (%s vs %s) on a day that is not the hole's",
				day, a.reference(), b.reference())
		}
	}
	if days-int64(agree) > 2 {
		t.Errorf("the editions disagree on %d of %d days; a single hole should cost at most its own days", days-int64(agree), days)
	}
	// The control: the compacted index over the same two fixtures.
	fullValid, holedValid := resolvedVerseOfDay(full), resolvedVerseOfDay(holed)
	compactAgree := 0
	for day := int64(0); day < days; day++ {
		if fullValid[dayIndex(day, len(fullValid))].reference() == holedValid[dayIndex(day, len(holedValid))].reference() {
			compactAgree++
		}
	}
	if compactAgree > int(days)/2 {
		t.Errorf("the compacted index agrees on %d of %d days for this fixture, so the check above "+
			"could not tell the two indexes apart", compactAgree, days)
	}
}

// THE SEED KEEPS EVERY PASSAGE IN ROTATION. The embedded Gospels resolve only
// the Gospel entries; a day whose slot is one of them shows it, and every other
// day falls back to the seed's own compacted list. The split between the two
// paths makes the frequencies uneven, but bounded: every passage comes round,
// and none more than twice its share. Under the alternative — walk forward
// from today's slot to the next entry that resolves — a passage is shown once
// for every day in the gap before it, a dozen days running for one and a
// single day for the next, which breaks the bound.
func TestSeedGospelsKeepEveryPassageInRotation(t *testing.T) {
	bd, err := loadSeedGospels()
	if err != nil {
		t.Fatal(err)
	}
	st := &AppState{Bible: bd}
	valid := len(resolvedVerseOfDay(st))
	if valid < 40 {
		t.Fatalf("only %d rotation entries resolve on the seed; the fixture is too thin", valid)
	}
	gospels := map[string]bool{"Matthew": true, "Mark": true, "Luke": true, "John": true}
	const cycles = 3
	shown := map[string]int{}
	for day := int64(20000); day < 20000+int64(cycles*valid); day++ {
		d, ok := verseOfTheDayAt(st, day)
		if !ok {
			t.Fatalf("day %d: nothing on the seed", day)
		}
		if !gospels[d.Book] {
			t.Errorf("day %d: %s is not in the seed", day, d.reference())
		}
		shown[d.reference()]++
	}
	if len(shown) != valid {
		t.Errorf("%d distinct passages shown of the %d the seed can show", len(shown), valid)
	}
	for ref, n := range shown {
		if n > 2*cycles {
			t.Errorf("%s came round %d times in %d cycles — more than twice its share", ref, n, cycles)
		}
	}
}

// A BOOK THE EDITION LACKS shows its alternate on the same slot; a VERSE the
// edition lacks does not — that is the edition's decision about the passage
// and the fallback pick covers the day.
func TestDeuterocanonShowsItselfOrItsAlternate(t *testing.T) {
	wisdom := dayPassage{"Wisdom", 3, 1, 3}
	alt, ok := verseOfDayAlternates[wisdom]
	if !ok {
		t.Fatal("fixture: Wisdom 3:1-3 has no alternate")
	}
	with := rotationFixture()
	if d, ok := resolveDayEntry(with, wisdom); !ok || d.Book != "Wisdom" || len(d.Verses) != 3 {
		t.Errorf("with the book present, resolved %+v %v — expected Wisdom 3:1-3 itself", d, ok)
	}
	without := rotationFixture()
	delete(without.Verses, "Wisdom")
	without.Books = removeBook(without.Books, "Wisdom")
	without.Verses[alt.Book][alt.Chapter] = append(without.Verses[alt.Book][alt.Chapter],
		Verse{BookName: alt.Book, Chapter: alt.Chapter, Verse: alt.Lo, Text: "x"})
	if d, ok := resolveDayEntry(without, wisdom); !ok || d.Book != alt.Book || d.Lo != alt.Lo {
		t.Errorf("with the book absent, resolved %+v %v — expected the alternate %s", d, ok, alt.key())
	}
	holed := rotationFixture(wisdom)
	holed.Verses["Wisdom"][3] = []Verse{{BookName: "Wisdom", Chapter: 3, Verse: 1, Text: "x"}}
	// The alternate is present, so a resolver that reached for it on a
	// missing verse would find it — that is the mutation this case is for.
	holed.Verses[alt.Book][alt.Chapter] = append(holed.Verses[alt.Book][alt.Chapter],
		Verse{BookName: alt.Book, Chapter: alt.Chapter, Verse: alt.Lo, Text: "x"})
	if d, ok := resolveDayEntry(holed, wisdom); ok {
		t.Errorf("with the book present but the run incomplete, resolved %s — a missing verse must "+
			"not summon the alternate", d.reference())
	}
}

func removeBook(books []string, name string) []string {
	out := books[:0:0]
	for _, b := range books {
		if b != name {
			out = append(out, b)
		}
	}
	return out
}

func TestSpanNeedsEveryVerse(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{3: {
		{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world."},
	}}
	span := dayPassage{"John", 3, 16, 17}
	if _, ok := resolveDayPassage(bd, span); ok {
		t.Error("John 3:16-17 resolved with verse 17 absent")
	}
	bd.Verses["John"][3] = append(bd.Verses["John"][3],
		Verse{BookName: "John", Chapter: 3, Verse: 17, Text: "For God didn't send his Son to judge."})
	d, ok := resolveDayPassage(bd, span)
	if !ok {
		t.Fatal("John 3:16-17 did not resolve with both verses present")
	}
	if d.reference() != "John 3:16–17" {
		t.Errorf("reference %q", d.reference())
	}
	if want := "For God so loved the world.\nFor God didn't send his Son to judge."; d.text() != want {
		t.Errorf("text %q, want the verses on their own lines", d.text())
	}
}

func TestFragmentFrame(t *testing.T) {
	cases := []struct{ in, want string }{
		{"for all have sinned, and fall short of the glory of God;", "… for all have sinned, and fall short of the glory of God;…"},
		{"‘The LORD bless you, and keep you.", "‘The LORD bless you, and keep you."},
		{"Love is patient and is kind", "Love is patient and is kind…"},
		{"to give you hope and a future.”", "… to give you hope and a future.”"},
		{"says the LORD,”", "… says the LORD,…”"},
		{"Always rejoice.", "Always rejoice."},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := fragmentFrame(c.in); got != c.want {
			t.Errorf("fragmentFrame(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A run must never cross a verse an edition omits: it would read as one
// sentence with a piece missing. The omissions are the shipped table.
func TestNoSpanCrossesAnOmittedVerse(t *testing.T) {
	checked := 0
	for versionID := range omittedVerses {
		for _, p := range verseOfDayRefs {
			for n := p.Lo; n <= p.hi(); n++ {
				checked++
				if omitsVerse(versionID, p.Book, p.Chapter, n) {
					t.Errorf("%s: the %s omits verse %d", p.key(), versionID, n)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no omitted-verse table was consulted; the check did nothing")
	}
}

func TestVerseOfTheDayNilOrEmpty(t *testing.T) {
	if _, ok := verseOfTheDay(nil); ok {
		t.Error("nil state should yield no verse")
	}
	if _, ok := verseOfTheDay(&AppState{Bible: NewBibleData()}); ok {
		t.Error("empty bible should yield no verse")
	}
}

func TestVerseOfTheDayResolvesAndIsStable(t *testing.T) {
	bd := NewBibleData()
	bd.Books = []string{"John"}
	bd.Verses["John"] = map[int][]Verse{
		14: {{BookName: "John", Chapter: 14, Verse: 6, Text: "Jesus said to him, “I am the way, the truth, and the life.”"}},
	}
	state := &AppState{Bible: bd}

	v, ok := verseOfTheDay(state)
	if !ok {
		t.Fatal("expected a resolvable verse")
	}
	if v.reference() != "John 14:6" {
		t.Errorf("unexpected pick %s", v.reference())
	}
	if v2, _ := verseOfTheDay(state); v2.reference() != v.reference() || v2.text() != v.text() {
		t.Error("verse of the day should be stable within a day")
	}
}

// THE CARD SETS THE PASSAGE IN THE READING FACE at the reader's size. It used
// to be the toolkit's rich text: the chrome face at the chrome size whatever
// the reader chose. Mutation: build the body with widget.NewRichTextWithText.
func TestTheCardSetsThePassageInTheReadingFace(t *testing.T) {
	st, win := smallPhone(t)
	votdSynchronousRemeasure(t)
	showVerseOfDay(st)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	body := findReadingParagraph(p.Content)
	if body == nil {
		t.Fatal("the card sets no reading paragraph")
	}
	if body.face == nil || body.face != styledPaneFont() {
		t.Error("the passage is not set in the reading face the pane uses")
	}
	if fonts := loadReadingFonts(); fonts == nil || body.face.Name() != fonts.regular.Name() {
		t.Errorf("the card's face is %v, not the reading family", body.face)
	}
	if want := float32(readingGlyphPx()); body.size != want {
		t.Errorf("the passage is set at %vpt; the reading panes set their body at %vpt", body.size, want)
	}
	if strings.TrimSpace(body.text) == "" {
		t.Error("the paragraph carries no text")
	}
}

// THE CARD SHARES THE PASSAGE IN ITS OWN CHAPTER. The selection route reads
// the chapter the reader is on at every stage, so a card over Matthew 5 would
// cite Psalm 23 as "Matthew 5". Mutation: route the card's Share through
// shareVerse (the selection route) — the citation names the reader's chapter.
func TestTheCardSharesThePassageNotTheReadersChapter(t *testing.T) {
	st, win := smallPhone(t)
	votdSynchronousRemeasure(t)
	st.Bible = &BibleData{
		Books: []string{"Matthew", "John"},
		Verses: map[string]map[int][]Verse{
			"Matthew": {5: {{BookName: "Matthew", Chapter: 5, Verse: 3, Text: "Blessed are the poor in spirit."}}},
			"John": {3: {
				{BookName: "John", Chapter: 3, Verse: 16, Text: "For God so loved the world, that he gave his only born Son."},
				{BookName: "John", Chapter: 3, Verse: 17, Text: "For God didn't send his Son into the world to judge the world."},
			}},
		},
	}
	// The reader is on Matthew 5; the rotation entry the fixture can show is
	// John 3:16-17 (Matthew 5:3 is one too — both resolve, and the pick is
	// whichever the day lands on; either way the card's chapter is not
	// guaranteed to be the reader's, which is what the assertion needs).
	st.CurrentBook, st.CurrentChapter = "Matthew", 5
	var got string
	prevOut := shareTextOut
	shareTextOut = func(s string) { got = s }
	t.Cleanup(func() { shareTextOut = prevOut })
	showVerseOfDay(st)
	p := topPopup(t, win)
	test.WidgetRenderer(p).Layout(p.Size())

	d, ok := verseOfTheDay(st)
	if !ok {
		t.Fatal("fixture: no verse of the day")
	}
	if d.Book == st.CurrentBook && d.Chapter == st.CurrentChapter {
		// Make the reader's chapter differ from the card's so the two routes
		// cannot agree by coincidence.
		st.CurrentBook, st.CurrentChapter = "John", 3
		if d.Book == "John" {
			st.CurrentBook, st.CurrentChapter = "Matthew", 5
		}
	}
	share := findIconTapButton(p.Content, theme.MailSendIcon().Name())
	if share == nil {
		t.Fatal("the card has no share control")
	}
	share.Tapped(&fyne.PointEvent{})

	want, ok := sharePassageMessage(st, d.Book, d.Chapter, d.Lo, d.Hi)
	if !ok {
		t.Fatal("fixture: the passage route produced nothing")
	}
	if got != want {
		t.Errorf("the card shared:\n%s\nthe passage route says:\n%s", got, want)
	}
	if !strings.Contains(got, d.reference()) {
		t.Errorf("the share does not cite the card's passage %s:\n%s", d.reference(), got)
	}
	if strings.Contains(got, fmt.Sprintf("%s %d", st.CurrentBook, st.CurrentChapter)) {
		t.Errorf("the share cites the reader's chapter %s %d:\n%s", st.CurrentBook, st.CurrentChapter, got)
	}
}

func findIconTapButton(o fyne.CanvasObject, iconName string) *iconTapButton {
	switch v := o.(type) {
	case *iconTapButton:
		if v.icon != nil && v.icon.Name() == iconName {
			return v
		}
	case *fyne.Container:
		for _, c := range v.Objects {
			if b := findIconTapButton(c, iconName); b != nil {
				return b
			}
		}
	case *container.Scroll:
		return findIconTapButton(v.Content, iconName)
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(v).Objects() {
			if b := findIconTapButton(c, iconName); b != nil {
				return b
			}
		}
	}
	return nil
}
