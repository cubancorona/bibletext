package bibletext

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// publicXRefSkeleton builds a lightweight canon carrying every chapter and the
// verses needed to resolve every citation in WEB, WEBC and BSB without needing
// a machine cache in CI.
func publicXRefSkeleton(catholic bool) *BibleData {
	books := NewBibleData().Books
	if catholic {
		books = append([]string(nil), catholicBooks...)
	}
	bd := &BibleData{
		Books:    books,
		Verses:   make(map[string]map[int][]Verse, len(books)),
		Headings: make(map[string]map[int][]Heading),
	}
	// Populate 150 chapters of 180 synthetic verses per book? Too heavy: instead
	// populate chapter 1 verse 1 for every book, plus every chapter/verse cited
	// or hosting a note in any of the three public tables.
	ensureVerse := func(book string, ch, v int, text string) {
		if ch < 1 || v < 1 {
			return
		}
		if bd.Verses[book] == nil {
			bd.Verses[book] = map[int][]Verse{}
		}
		vs := bd.Verses[book][ch]
		for i := range vs {
			if vs[i].Verse == v {
				if text != "" {
					vs[i].Text = text
				}
				return
			}
		}
		if text == "" {
			text = "A synthetic verse for cross-reference resolution."
		}
		bd.Verses[book][ch] = append(vs, Verse{
			BookName: book, Book: book, Chapter: ch, Verse: v, Text: text,
		})
	}
	for _, b := range books {
		ensureVerse(b, 1, 1, "")
	}
	ensureRef := func(id string) {
		if r, ok := parseUSFMRefID(id); ok {
			ensureVerse(r.Book, r.Chapter, r.Verse, "")
			if r.EndV > 0 {
				ec := r.EndCh
				if ec == 0 {
					ec = r.Chapter
				}
				for v := r.Verse; v <= r.EndV && ec == r.Chapter; v++ {
					ensureVerse(r.Book, r.Chapter, v, "")
				}
				ensureVerse(r.Book, ec, r.EndV, "")
			}
			return
		}
		if b, from, to, ok := parseUSFMChapterID(id); ok {
			for c := from; c <= to; c++ {
				ensureVerse(b, c, 1, "")
				ensureVerse(b, c, 5, "")
			}
		}
	}
	for _, table := range []map[string]publisherVerseEntry{webPublisherCrossRefs, webcPublisherCrossRefs} {
		for key, entry := range table {
			b, c, v, ok := splitPublisherVerseKey(key)
			if !ok {
				continue
			}
			if !catholic && !protestantCanonBooks[b] {
				continue
			}
			ensureVerse(b, c, v, strings.Repeat("w", entry.Runes))
			for _, n := range entry.Notes {
				for _, r := range n.Refs {
					if catholic || protestantCanonBooks[apiBibleBookName(strings.SplitN(r.ID, ".", 2)[0])] {
						ensureRef(r.ID)
					}
				}
			}
		}
	}
	for key, list := range bsbPublisherCrossRefs {
		b, c, ok := splitPublisherChapterKey(key)
		if !ok {
			continue
		}
		if bd.Headings[b] == nil {
			bd.Headings[b] = map[int][]Heading{}
		}
		for _, hn := range list {
			ensureVerse(b, c, hn.BeforeVerse, "")
			var atV int
			for _, h := range bd.Headings[b][c] {
				if h.BeforeVerse == hn.BeforeVerse && h.Style == "heading" {
					atV++
				}
			}
			for atV <= hn.HeadingOrd {
				bd.Headings[b][c] = append(bd.Headings[b][c], Heading{
					Text: "Section heading", Style: "heading", BeforeVerse: hn.BeforeVerse,
				})
				atV++
			}
			for _, r := range hn.Refs {
				ensureRef(r.ID)
			}
		}
	}
	return bd
}

func TestShippedPublicPublisherCrossRefTablesAreCompleteAndSound(t *testing.T) {
	checkVerseTable := func(name string, table map[string]publisherVerseEntry, catholic bool, wantVerses, wantNotes, wantRefs, wantAlone int) {
		t.Helper()
		if len(table) != wantVerses {
			t.Errorf("%s: %d verses in table, want %d", name, len(table), wantVerses)
		}
		bd := publicXRefSkeleton(catholic)
		notes, refs, aloneCount, seeCount := 0, 0, 0, 0
		for key, entry := range table {
			b, ch, vNum, ok := splitPublisherVerseKey(key)
			if !ok {
				t.Fatalf("%s: malformed key %q", name, key)
			}
			if entry.Runes <= 0 || len(entry.Notes) == 0 {
				t.Errorf("%s %s: invalid entry %+v", name, key, entry)
			}
			var fns []Footnote
			for _, n := range entry.Notes {
				notes++
				refs += len(n.Refs)
				runes := []rune(n.Text)
				if n.Anchor < 0 || n.Anchor > entry.Runes {
					t.Errorf("%s %s: anchor %d outside [0,%d]", name, key, n.Anchor, entry.Runes)
				}
				if n.AtEnd && n.Anchor != entry.Runes {
					t.Errorf("%s %s: AtEnd note has anchor %d != Runes %d", name, key, n.Anchor, entry.Runes)
				}
				if len(n.Refs) == 0 {
					t.Errorf("%s %s: note %q resolved zero refs", name, key, n.Text)
				}
				for _, r := range n.Refs {
					if r.Start < 0 || r.End > len(runes) || r.Start >= r.End {
						t.Errorf("%s %s: bad span %+v in %q", name, key, r, n.Text)
					}
				}
				fns = append(fns, Footnote{
					Anchor: n.Anchor, Text: n.Text, Kind: footnoteKindCrossref, Caller: "+", Refs: n.Refs,
				})
			}
			vs := bd.Verses[b][ch]
			var sel []Verse
			for i := range vs {
				if vs[i].Verse == vNum {
					vs[i].Footnotes = fns
					sel = []Verse{vs[i]}
					break
				}
			}
			resolved := publisherCrossRefsFor(bd, b, ch, sel)
			if len(resolved) != len(entry.Notes) {
				t.Fatalf("%s %s: resolved %d notes, want %d", name, key, len(resolved), len(entry.Notes))
			}
			for _, rn := range resolved {
				if len(rn.Targets) != len(entry.Notes[rn.Index].Refs) {
					t.Errorf("%s %s: resolved %d targets, want %d for %q",
						name, key, len(rn.Targets), len(entry.Notes[rn.Index].Refs), rn.Text)
				}
				for _, p := range rn.Pieces {
					if p.Target == nil {
						t.Errorf("%s %s: unresolved piece %+v in %q", name, key, p, rn.Text)
					}
				}
				if _, ok := rn.alone(); ok {
					aloneCount++
					if strings.Contains(rn.Text, "See ") {
						t.Errorf("%s %s: 'See ' note %q collapsed to alone()", name, key, rn.Text)
					}
				}
				if strings.Contains(rn.Text, "See ") {
					seeCount++
					if strings.HasPrefix(rn.Text, "See ") && (len(rn.Pieces) == 0 || rn.Pieces[0].Start != 4) {
						t.Errorf("%s %s: 'See ' note %q swallowed 'See ' into piece %+v", name, key, rn.Text, rn.Pieces)
					}
				}
			}
		}
		if notes != wantNotes || refs != wantRefs || aloneCount != wantAlone || seeCount != 7 {
			t.Errorf("%s: notes=%d (want %d), refs=%d (want %d), alone=%d (want %d), see=%d (want 7)",
				name, notes, wantNotes, refs, wantRefs, aloneCount, wantAlone, seeCount)
		}
	}

	checkVerseTable("WEB", webPublisherCrossRefs, false, 322, 340, 424, 276)
	checkVerseTable("WEBC", webcPublisherCrossRefs, true, 325, 343, 430, 278)

	// WEBC adds the three deuterocanonical cross-reference notes.
	for _, dcKey := range []string{"Tobit 2:6", "Wisdom 14:27", "Sirach 6:30"} {
		if _, ok := webcPublisherCrossRefs[dcKey]; !ok {
			t.Errorf("WEBC missing deuterocanonical cross-reference at %s", dcKey)
		}
		if _, ok := webPublisherCrossRefs[dcKey]; ok {
			t.Errorf("WEB unexpectedly carries deuterocanonical cross-reference at %s", dcKey)
		}
	}

	// BSB: 1,325 section-heading notes across 634 chapters, 2,032 resolved refs
	// plus 1 words-only cross-book span ("Joshua–Malachi" at Hebrews 11:30).
	if len(bsbPublisherCrossRefs) != 634 {
		t.Errorf("BSB: %d chapters in table, want 634", len(bsbPublisherCrossRefs))
	}
	bd := publicXRefSkeleton(false)
	bsbNotes, bsbRefs, bsbAlone, bsbWordsAlone := 0, 0, 0, 0
	for key, list := range bsbPublisherCrossRefs {
		b, ch, ok := splitPublisherChapterKey(key)
		if !ok {
			t.Fatalf("BSB: malformed chapter key %q", key)
		}
		for _, hn := range list {
			bsbNotes++
			bsbRefs += len(hn.Refs)
			bd.Headings[b][ch] = []Heading{{
				Text: "Heading", Style: "heading", BeforeVerse: hn.BeforeVerse,
				Footnotes: []Footnote{{
					Anchor: 0, Text: hn.Text, Kind: footnoteKindCrossref, Caller: "-", Refs: hn.Refs,
				}},
			}}
			var sel []Verse
			for _, v := range bd.Verses[b][ch] {
				if v.Verse == hn.BeforeVerse {
					sel = []Verse{v}
					break
				}
			}
			resolved := publisherCrossRefsFor(bd, b, ch, sel)
			if len(resolved) != 1 || !resolved[0].Heading {
				t.Fatalf("BSB %s:%d: resolved %+v", key, hn.BeforeVerse, resolved)
			}
			rn := resolved[0]
			if hn.Text == "Joshua–Malachi" {
				if w, ok := rn.wordsAlone(); !ok || w != "Joshua–Malachi" {
					t.Errorf("BSB Hebrews 11:30 expected wordsAlone 'Joshua–Malachi', got %+v", rn)
				}
				bsbWordsAlone++
				continue
			}
			if len(rn.Targets) != len(hn.Refs) || len(rn.Targets) == 0 {
				t.Errorf("BSB %s:%d (%q): targets=%d, refs=%d", key, hn.BeforeVerse, hn.Text, len(rn.Targets), len(hn.Refs))
			}
			for _, p := range rn.Pieces {
				if p.Target == nil {
					t.Errorf("BSB %s:%d: unresolved piece %+v in %q", key, hn.BeforeVerse, p, hn.Text)
				}
			}
			if _, ok := rn.alone(); ok {
				bsbAlone++
			}
		}
	}
	if bsbNotes != 1325 || bsbRefs != 2032 || bsbAlone != 796 || bsbWordsAlone != 1 {
		t.Errorf("BSB: notes=%d (want 1325), refs=%d (want 2032), alone=%d (want 796), wordsAlone=%d (want 1)",
			bsbNotes, bsbRefs, bsbAlone, bsbWordsAlone)
	}
}

func TestPublicPublisherCrossRefAttachmentMarksGuardsAndIdempotency(t *testing.T) {
	// 1. WEB embedded Gospels seed receives cross-reference notes automatically
	// and never leaks them into chapter-bottom translator footnotes.
	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatalf("loadSeedGospels: %v", err)
	}
	matt5 := seed.GetVerse("Matthew", 5, 43)
	if matt5 == nil {
		t.Fatal("missing Matthew 5:43 in seed")
	}
	notes := publisherCrossRefsFor(seed, "Matthew", 5, []Verse{*matt5})
	if len(notes) != 1 || notes[0].Text != "Leviticus 19:18" {
		t.Fatalf("seed Matthew 5:43 cross-ref notes = %+v", notes)
	}
	// Caller "+" keeps Mark flush after "neighbor" (not pushed across the space to "and").
	wantMark := len([]rune("“You have heard that it was said, ‘You shall love your neighbor"))
	if notes[0].Mark != wantMark || notes[0].Unkeyed {
		t.Errorf("Matthew 5:43 Mark = %d (Unkeyed=%v), want %d flush after 'neighbor'", notes[0].Mark, notes[0].Unkeyed, wantMark)
	}
	// Translator footnotes exclude crossref notes.
	for _, e := range chapterFootnoteEntries(seed.Verses["Matthew"][5], nil, nil, nil) {
		if strings.Contains(e.Text, "Leviticus 19:18") {
			t.Errorf("cross-reference note leaked into translator footnotes: %+v", e)
		}
	}
	// Idempotency: applying again adds nothing.
	beforeCount := len(seed.GetVerse("Matthew", 5, 43).Footnotes)
	applyPublisherCrossRefs("web", seed)
	if got := len(seed.GetVerse("Matthew", 5, 43).Footnotes); got != beforeCount {
		t.Errorf("applyPublisherCrossRefs not idempotent on WEB: %d -> %d", beforeCount, got)
	}

	// 2. Stale-wording guard: when a verse's text changes in a future feed,
	// mid-verse notes become Unkeyed (Anchor = -1) so no wrong word gets a
	// letter, whereas AtEnd notes still anchor cleanly at the end of the verse.
	staleBD := publicXRefSkeleton(false)
	staleBD.Verses["Hebrews"][11] = []Verse{{
		BookName: "Hebrews", Book: "Hebrews", Chapter: 11, Verse: 34,
		Text: "Revised future wording where the mid-verse offsets no longer match.",
	}}
	staleBD.Verses["Hebrews"][1] = []Verse{{
		BookName: "Hebrews", Book: "Hebrews", Chapter: 1, Verse: 6,
		Text: "Revised future wording with note at the end.",
	}}
	applyPublisherCrossRefs("web", staleBD)

	mid := publisherCrossRefsFor(staleBD, "Hebrews", 11, staleBD.Verses["Hebrews"][11])
	if len(mid) != 2 || !mid[0].Unkeyed || !mid[1].Unkeyed || mid[0].Index != -1 || len(mid[0].Targets) != 1 || len(mid[1].Targets) != 2 {
		t.Errorf("stale mid-verse notes should be Unkeyed with resolved targets, got %+v", mid)
	}
	end := publisherCrossRefsFor(staleBD, "Hebrews", 1, staleBD.Verses["Hebrews"][1])
	wantEnd := len([]rune(staleBD.Verses["Hebrews"][1][0].Text))
	if len(end) != 1 || end[0].Unkeyed || end[0].Mark != wantEnd || end[0].Targets[0].Basis != "LXX" {
		t.Errorf("stale AtEnd note should stay keyed at %d with LXX basis, got %+v", wantEnd, end)
	}

	// 3. BSB: cleans up helloao's leaked "(Joshua–Malachi)" duplicate heading at
	// Hebrews 11:30, attaches "Joshua–Malachi" to "The Faith of Many", and at
	// Proverbs 23:1 attaches to "True Riches" (ord 0) rather than "Saying 7" (ord 1).
	bsbBD := publicXRefSkeleton(false)
	bsbBD.Headings["Hebrews"][11] = []Heading{
		{Text: "The Faith of Many", Style: "heading", BeforeVerse: 30},
		{Text: "(Joshua–Malachi)", Style: "heading", BeforeVerse: 30},
	}
	bsbBD.Headings["Proverbs"][23] = []Heading{
		{Text: "True Riches", Style: "heading", BeforeVerse: 1},
		{Text: "Saying 7", Style: "heading", BeforeVerse: 1},
	}
	applyPublisherCrossRefs("bsb", bsbBD)
	applyPublisherCrossRefs("bsb", bsbBD) // idempotent second call
	hebHeads := bsbBD.Headings["Hebrews"][11]
	if len(hebHeads) != 1 || hebHeads[0].Text != "The Faith of Many" || len(hebHeads[0].Footnotes) != 1 || hebHeads[0].Footnotes[0].Text != "Joshua–Malachi" {
		t.Errorf("BSB Hebrews 11:30 headings after cleanup = %+v", hebHeads)
	}
	provHeads := bsbBD.Headings["Proverbs"][23]
	if len(provHeads[0].Footnotes) != 1 || len(provHeads[1].Footnotes) != 0 {
		t.Errorf("BSB Proverbs 23:1 attached to wrong heading: %+v", provHeads)
	}

	// HeadingHash takes precedence when an unnoted heading precedes the noted one,
	// while HeadingOrd falls back cleanly when feed wording differs (1 Chronicles 16:7).
	bsbGuardBD := publicXRefSkeleton(false)
	bsbGuardBD.Headings["Proverbs"][23] = []Heading{
		{Text: "Saying 7", Style: "heading", BeforeVerse: 1},
		{Text: "True Riches", Style: "heading", BeforeVerse: 1},
	}
	bsbGuardBD.Headings["1 Chronicles"][16] = []Heading{
		{Text: "David’s Song of Thanksgiving (Revised Wording)", Style: "heading", BeforeVerse: 7},
	}
	applyPublisherCrossRefs("bsb", bsbGuardBD)
	if len(bsbGuardBD.Headings["Proverbs"][23][0].Footnotes) != 0 || len(bsbGuardBD.Headings["Proverbs"][23][1].Footnotes) != 1 {
		t.Errorf("HeadingHash did not disambiguate reordered headings: %+v", bsbGuardBD.Headings["Proverbs"][23])
	}
	if len(bsbGuardBD.Headings["1 Chronicles"][16][0].Footnotes) != 1 {
		t.Errorf("HeadingOrd fallback did not attach 1 Chronicles 16:7 note: %+v", bsbGuardBD.Headings["1 Chronicles"][16])
	}
}

func TestPublicPublisherCrossRefCitationShapesAndPanelComposition(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	// WEBC deuterocanonical cross-reference note (Wisdom 14:27) opens WEBC REFERENCES
	// with live cards (including deuterocanonical target Wisdom 14:21!) instead of
	// refusing the selection as uncovered deuterocanon.
	webcBD := publicXRefSkeleton(true)
	webcBD.Verses["Wisdom"][14] = []Verse{
		{BookName: "Wisdom", Book: "Wisdom", Chapter: 14, Verse: 21, Text: "And this became a hidden trap for mankind."},
		{BookName: "Wisdom", Book: "Wisdom", Chapter: 14, Verse: 26, Text: "Confusion of what is good, forgetfulness of favors."},
		{BookName: "Wisdom", Book: "Wisdom", Chapter: 14, Verse: 27, Text: "For the worship of idols that may not be named\nis a beginning and cause and end of every evil."},
	}
	applyPublisherCrossRefs("webc", webcBD)
	st := &AppState{Bible: webcBD, CurrentBook: "Wisdom", CurrentChapter: 14, CurrentVersion: "webc"}

	// Unnoted deuterocanonical verse still reports uncovered before loading.
	if note, none := panelCoverage(st, "Wisdom", webcBD.Verses["Wisdom"][14][1:2]); !none || note != crossRefDeuterocanonLine {
		t.Errorf("unnoted Wisdom 14:26 coverage = (%q, %v), want deuterocanon refusal", note, none)
	}
	// Noted deuterocanonical verse is covered by WEBC's own note!
	notedSel := webcBD.Verses["Wisdom"][14][2:3]
	if note, none := panelCoverage(st, "Wisdom", notedSel); none || note != "" {
		t.Errorf("noted Wisdom 14:27 coverage = (%q, %v), want covered", note, none)
	}
	lst := buildCrossRefList(st, notedSel, nil, nil, lightPalette, func(crossRef) {})
	if !lst.Passage || !lst.Sections.Publisher || lst.PublisherNotes != 1 || lst.PublisherCards != 4 {
		t.Fatalf("Wisdom 14:27 list = %+v", lst)
	}
	joined := joinedTexts(lst.Objects)
	for _, want := range []string{"WEBC REFERENCES", "Exodus 23:13", "Psalms 16:4", "Hosea 2:17", "Wisdom 14:21"} {
		if !strings.Contains(joined, want) {
			t.Errorf("Wisdom 14:27 panel missing %q:\n%s", want, joined)
		}
	}

	// Sparse edition behaviour on WEB:
	// - When selection HAS a publisher note, shows WEB REFERENCES + GOSPEL PARALLELS
	//   (unbadged) + TREASURY OF SCRIPTURE KNOWLEDGE (deduped, treasuryCredit).
	// - When selection has NO publisher note, keeps the plain unlabelled list
	//   (badged Parallel, tskCredit, no "No WEB cross-references" empty banner).
	webBD := publicXRefSkeleton(false)
	webBD.Verses["Matthew"][5] = []Verse{
		{BookName: "Matthew", Book: "Matthew", Chapter: 5, Verse: 42, Text: "Give to him who asks you."},
		{BookName: "Matthew", Book: "Matthew", Chapter: 5, Verse: 43, Text: "“You have heard that it was said, ‘You shall love your neighbor and hate your enemy.’"},
	}
	applyPublisherCrossRefs("web", webBD)
	webSt := &AppState{Bible: webBD, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web"}
	refs := []crossRef{
		{Book: "Luke", Chapter: 6, Verse: 27, Parallel: true},
		{Book: "Leviticus", Chapter: 19, Verse: 18}, // duplicates WEB's own card -> dropped from Treasury!
		{Book: "Romans", Chapter: 13, Verse: 9},
	}

	withNote := buildCrossRefList(webSt, webBD.Verses["Matthew"][5][1:2], refs, nil, lightPalette, func(crossRef) {})
	if !withNote.Passage || !withNote.Sections.Publisher || withNote.PublisherCards != 1 || withNote.TSKRows != 1 {
		t.Fatalf("WEB noted Matthew 5:43 list = %+v", withNote)
	}
	withNoteText := joinedTexts(withNote.Objects)
	for _, want := range []string{"WEB REFERENCES", "Leviticus 19:18", "GOSPEL PARALLELS", "Luke 6:27", "TREASURY OF SCRIPTURE KNOWLEDGE", "Romans 13:9"} {
		if !strings.Contains(withNoteText, want) {
			t.Errorf("WEB noted Matthew 5:43 missing %q:\n%s", want, withNoteText)
		}
	}
	if src, notice := crossRefFooterCredit(webSt, withNote.Sections); src != treasuryCredit+"\n"+parallelsCredit || notice != "" {
		t.Errorf("WEB noted footer = (%q, %q)", src, notice)
	}

	withoutNote := buildCrossRefList(webSt, webBD.Verses["Matthew"][5][0:1], refs, nil, lightPalette, func(crossRef) {})
	if withoutNote.Passage || withoutNote.Sections.Publisher || withoutNote.TSKRows != 2 {
		t.Fatalf("WEB unnoted Matthew 5:42 list = %+v", withoutNote)
	}
	withoutNoteText := joinedTexts(withoutNote.Objects)
	if strings.Contains(withoutNoteText, "WEB REFERENCES") || strings.Contains(withoutNoteText, "No WEB cross-references") {
		t.Errorf("WEB unnoted verse grew an empty publisher section:\n%s", withoutNoteText)
	}
	if src, _ := crossRefFooterCredit(webSt, withoutNote.Sections); src != tskCredit+"\n"+parallelsCredit {
		t.Errorf("WEB unnoted footer = %q, want tskCredit + parallelsCredit", src)
	}

	// Offline error on a noted WEB verse stays scoped under TREASURY OF SCRIPTURE KNOWLEDGE.
	offline := buildCrossRefList(webSt, webBD.Verses["Matthew"][5][1:2], refs[:1], errors.New("offline"), lightPalette, func(crossRef) {})
	offlineText := joinedTexts(offline.Objects)
	if !strings.Contains(offlineText, "WEB REFERENCES") || !strings.Contains(offlineText, crossRefOfflineLine) {
		t.Errorf("WEB noted offline list = %q", offlineText)
	}

	// BSB noted section heading mounts BSB REFERENCES on the verse the heading
	// precedes, while the following verse stays in the plain unlabelled list.
	bsbBD := publicXRefSkeleton(false)
	bsbBD.Headings["Psalms"][3] = []Heading{
		{Text: "Deliver Me, O LORD!", Style: "heading", BeforeVerse: 1},
	}
	bsbBD.Verses["Psalms"][3] = []Verse{
		{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 1, Text: "O LORD, how my foes have increased!"},
		{BookName: "Psalms", Book: "Psalms", Chapter: 3, Verse: 2, Text: "Many say of my soul, “There is no deliverance for him in God.”"},
	}
	applyPublisherCrossRefs("bsb", bsbBD)
	bsbSt := &AppState{Bible: bsbBD, CurrentBook: "Psalms", CurrentChapter: 3, CurrentVersion: "bsb"}
	bsbWith := buildCrossRefList(bsbSt, bsbBD.Verses["Psalms"][3][0:1], nil, nil, lightPalette, func(crossRef) {})
	if !bsbWith.Sections.Publisher || !strings.Contains(joinedTexts(bsbWith.Objects), "BSB REFERENCES") {
		t.Errorf("BSB Psalms 3:1 did not mount BSB REFERENCES: %+v", bsbWith)
	}
	bsbWithout := buildCrossRefList(bsbSt, bsbBD.Verses["Psalms"][3][1:2], []crossRef{{Book: "Genesis", Chapter: 1, Verse: 1}}, nil, lightPalette, func(crossRef) {})
	if bsbWithout.Sections.Publisher || strings.Contains(joinedTexts(bsbWithout.Objects), "BSB REFERENCES") {
		t.Errorf("BSB Psalms 3:2 mounted BSB REFERENCES on unnoted verse: %+v", bsbWithout)
	}
}

func TestThePublisherCrossRefGeneratorBuildsTablesFromInventedUSFM(t *testing.T) {
	dir := t.TempDir()
	webUSFM := `\id MAT
\c 1
\p
\v 1 Spoken words,\x + \xo 1:1 \xt See Genesis 1:1; Exodus 1:1,2\x* then more words.
\v 2 Final words.\x + \xo 1:2 \xt Genesis 1:1 LXX\x*
\c 2
\q1
\v 1 \x + \xo 2:1 \xt Genesis 1:2\x*“Quoted start.”
`
	webcUSFM := webUSFM
	bsbUSFM := `\id MAT
\c 1
\s1 First Section
\r (\ref Genesis 1:1–2|GEN 1:1-2\ref*; \ref Exodus 1|EXO 1\ref*)
\s2 Subheading Without Ref
\p
\v 1 Spoken words, then more words.
\s1 Span Across Books
\r (\ref Genesis–Exodus\ref*)
\p
\v 2 Final words.
`
	writeRedLetterGenFile(t, filepath.Join(dir, "web", "MAT.usfm"), webUSFM)
	writeRedLetterGenFile(t, filepath.Join(dir, "webc", "MAT.usfm"), webcUSFM)
	writeRedLetterGenFile(t, filepath.Join(dir, "bsb", "MAT.usfm"), bsbUSFM)

	cacheObj := map[string]any{
		"Verses": map[string]any{
			"Genesis": map[string]any{
				"1": []map[string]any{{"Verse": 1, "Text": "Gen 1:1."}, {"Verse": 2, "Text": "Gen 1:2."}},
			},
			"Exodus": map[string]any{
				"1": []map[string]any{{"Verse": 1, "Text": "Exo 1:1."}, {"Verse": 2, "Text": "Exo 1:2."}},
			},
			"Matthew": map[string]any{
				"1": []map[string]any{
					{"Verse": 1, "Text": "Spoken words, then more words."},
					{"Verse": 2, "Text": "Final words."},
				},
				"2": []map[string]any{
					{"Verse": 1, "Text": "“Quoted start.”"},
				},
			},
		},
		"headings": map[string]any{
			"Matthew": map[string]any{
				"1": []map[string]any{
					{"text": "First Section", "style": "heading", "before_verse": 1},
					{"text": "Subheading Without Ref", "style": "heading", "before_verse": 1},
					{"text": "Span Across Books", "style": "heading", "before_verse": 2},
					{"text": "(Genesis–Exodus)", "style": "heading", "before_verse": 2},
				},
			},
		},
	}
	// Add GEN and EXO USFM files so partial-canon book sets match exactly.
	genUSFM := "\\id GEN\n\\c 1\n\\p\n\\v 1 Gen 1:1.\n\\v 2 Gen 1:2.\n"
	exoUSFM := "\\id EXO\n\\c 1\n\\p\n\\v 1 Exo 1:1.\n\\v 2 Exo 1:2.\n"
	for _, sub := range []string{"web", "webc", "bsb"} {
		writeRedLetterGenFile(t, filepath.Join(dir, sub, "GEN.usfm"), genUSFM)
		writeRedLetterGenFile(t, filepath.Join(dir, sub, "EXO.usfm"), exoUSFM)
	}
	blob, err := json.Marshal(cacheObj)
	if err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(dir, "cache.json")
	if err := os.WriteFile(cachePath, blob, 0o644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(dir, "out.go")
	src, logOut, err := runRedLetterGen(t, outPath,
		filepath.Join("scripts", "gen-publisher-xrefs.py"),
		"--allow-partial-canon",
		"--web-usfm", filepath.Join(dir, "web"), "--web-cache", cachePath,
		"--webc-usfm", filepath.Join(dir, "webc"), "--webc-cache", cachePath,
		"--bsb-usfm", filepath.Join(dir, "bsb"), "--bsb-cache", cachePath,
	)
	if err != nil {
		t.Fatalf("gen-publisher-xrefs.py failed: %v\n%s", err, logOut)
	}
	for _, want := range []string{
		`Anchor: 13, AtEnd: false, Text: "See Genesis 1:1; Exodus 1:1,2"`,
		`{"GEN.1.1", 4, 15}`,
		`{"EXO.1.1", 17, 27}`,
		`{"EXO.1.2", 28, 29}`,
		`Anchor: 12, AtEnd: true, Text: "Genesis 1:1 LXX", Refs: []NoteRef{{"GEN.1.1", 0, 11}}`,
		`Anchor: 0, AtEnd: false, Text: "Genesis 1:2"`,
		`BeforeVerse: 1, HeadingOrd: 0`,
		`Text: "Genesis 1:1–2; Exodus 1"`,
		`Text: "Genesis–Exodus", Refs: nil`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated table missing %q:\n%s", want, src)
		}
	}

	// CONTROL: fail closed when a target verse does not exist in the cache.
	badUSFM := strings.Replace(webUSFM, "Genesis 1:2", "Genesis 1:99", 1)
	writeRedLetterGenFile(t, filepath.Join(dir, "web", "MAT.usfm"), badUSFM)
	if _, _, err := runRedLetterGen(t, filepath.Join(dir, "bad.go"),
		filepath.Join("scripts", "gen-publisher-xrefs.py"),
		"--allow-partial-canon",
		"--web-usfm", filepath.Join(dir, "web"), "--web-cache", cachePath,
		"--webc-usfm", filepath.Join(dir, "webc"), "--webc-cache", cachePath,
		"--bsb-usfm", filepath.Join(dir, "bsb"), "--bsb-cache", cachePath,
	); err == nil {
		t.Fatal("generator succeeded on missing target verse Genesis 1:99; want failure")
	}
}

func TestThePublisherCrossRefGeneratorReproducesTheShippedTables_LocalOnly(t *testing.T) {
	cacheRoot := filepath.Dir(realCachePath(defaultCachePath()))
	sourcesRoot := filepath.Join(filepath.Dir(cacheRoot), "bibletext-sources")
	webUSFM := os.Getenv("BIBLETEXT_WEB_USFM")
	if webUSFM == "" {
		webUSFM = filepath.Join(sourcesRoot, "web", "unpacked")
	}
	webcUSFM := os.Getenv("BIBLETEXT_WEBC_USFM")
	if webcUSFM == "" {
		webcUSFM = filepath.Join(sourcesRoot, "webc", "unpacked")
	}
	bsbUSFM := os.Getenv("BIBLETEXT_BSB_USFM")
	if bsbUSFM == "" {
		bsbUSFM = filepath.Join(sourcesRoot, "bsb", "unpacked", "bsb_usfm")
	}
	webCache := realCachePath(cachePathForVersion("web"))
	webcCache := realCachePath(cachePathForVersion("webc"))
	bsbCache := realCachePath(cachePathForVersion("bsb"))

	for _, p := range []string{webUSFM, webcUSFM, bsbUSFM, webCache, webcCache, bsbCache} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("local USFM or cache not present (%v); skipping local reproduction check", err)
		}
	}

	outPath := filepath.Join(t.TempDir(), "publisher_xrefs_public_data.go")
	got, logOut, err := runRedLetterGen(t, outPath,
		filepath.Join("scripts", "gen-publisher-xrefs.py"),
		"--web-usfm", webUSFM, "--web-cache", webCache,
		"--webc-usfm", webcUSFM, "--webc-cache", webcCache,
		"--bsb-usfm", bsbUSFM, "--bsb-cache", bsbCache,
	)
	if err != nil {
		t.Fatalf("gen-publisher-xrefs.py failed against local caches: %v\n%s", err, logOut)
	}
	shipped, err := os.ReadFile("publisher_xrefs_public_data.go")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(shipped) {
		t.Fatal("publisher_xrefs_public_data.go differs from generator output; re-run scripts/gen-publisher-xrefs.py")
	}
}

func TestPublicPublisherHeadingReferencesRejectAmbiguousRevisions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		headings []Heading
	}{
		{"renamed and reordered", []Heading{
			{Text: "Numbered saying revised", Style: "heading", BeforeVerse: 1},
			{Text: "Riches revised", Style: "heading", BeforeVerse: 1},
		}},
		{"duplicate matching titles", []Heading{
			{Text: "Original section", Style: "heading", BeforeVerse: 1},
			{Text: "Original section", Style: "heading", BeforeVerse: 1},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bd := &BibleData{Headings: map[string]map[int][]Heading{
				"Proverbs": {23: tc.headings},
			}}
			table := map[string][]publisherHeadingNote{
				"Proverbs 23": {{
					BeforeVerse: 1, HeadingOrd: 0,
					HeadingHash: redLetterTextHash("Original section"),
					Text:        "Luke 12:13–21",
				}},
			}
			applyPublisherHeadingCrossRefs(bd, table)
			for i, h := range bd.Headings["Proverbs"][23] {
				if len(h.Footnotes) != 0 {
					t.Errorf("ambiguous heading %d received a reference", i)
				}
			}
		})
	}
}
