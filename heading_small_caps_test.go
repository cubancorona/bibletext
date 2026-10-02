package bibletext

// THE DIVINE NAME IN A HEADING OR A PSALM'S TITLE IS SET IN SMALL CAPITALS, as
// the edition prints it. The decoder used to bracket the feed's nd/sc spans
// only inside verses, so a heading such as "The LORD Is My Shepherd" and the
// titles that name Him reached every surface as the stored "Lord", in ordinary
// case, under a verse that drew the same word in small capitals.
//
// Every fixture here is SYNTHETIC: API-shaped markup with invented wording. No
// licensed text belongs in a test.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
)

// A titled chapter as the API.Bible chapter endpoint shapes it: a "d" title
// naming the divine name in an nd span, a section heading doing the same, a
// heading whose capitals-only span is the shape the feed sends for "GOD", an
// acrostic letter, and one whose name the publisher sets in small capitals
// with an sc span (the NKJV marks each of Psalm 119's stanza names so), a
// heading carrying a supplied-word span (which a heading keeps as plain
// words), and a verse with its own nd span as the control.
const smallCapsHeadedChapter = `[
  {"name":"para","type":"tag","attrs":{"style":"d"},"items":[
    {"type":"text","text":"A fixture song, when the ","attrs":{"verseId":"PSA.150.1"}},
    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord","attrs":{"verseId":"PSA.150.1"}}]},
    {"type":"text","text":" answered.","attrs":{"verseId":"PSA.150.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"s1"},"items":[
    {"type":"text","text":"The "},
    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord"}]},
    {"type":"text","text":" Keeps the Fixture"}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"1","sid":"PSA 150:1"},"items":[{"type":"text","text":"1"}]},
    {"type":"text","text":"Fixture praise to the ","attrs":{"verseId":"PSA.150.1"}},
    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord","attrs":{"verseId":"PSA.150.1"}}]},
    {"type":"text","text":".","attrs":{"verseId":"PSA.150.1"}}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"s1"},"items":[
    {"type":"text","text":"The Lord G"},
    {"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"OD"}]},
    {"type":"text","text":" Is Fixed"}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"qa"},"items":[{"type":"text","text":"א Aleph"}]},
  {"name":"para","type":"tag","attrs":{"style":"qa"},"items":[{"type":"text","text":"ב "},
    {"name":"char","type":"tag","attrs":{"style":"sc"},"items":[{"type":"text","text":"Beth"}]}]},
  {"name":"para","type":"tag","attrs":{"style":"s2"},"items":[
    {"type":"text","text":"A Heading "},
    {"name":"char","type":"tag","attrs":{"style":"it"},"items":[{"type":"text","text":"with"}]},
    {"type":"text","text":" Supplied Words"}
  ]},
  {"name":"para","type":"tag","attrs":{"style":"q1"},"items":[
    {"name":"verse","type":"tag","attrs":{"style":"v","number":"2","sid":"PSA 150:2"},"items":[{"type":"text","text":"2"}]},
    {"type":"text","text":"A second fixture line.","attrs":{"verseId":"PSA.150.2"}}
  ]}
]`

func noSentinels(t *testing.T, what, s string) {
	t.Helper()
	if withoutSentinels(s) != s {
		t.Errorf("%s carries a decoder sentinel: %q", what, s)
	}
}

func TestDecodeAPIBibleKeepsTheSmallCapitalsInHeadingsAndTitles(t *testing.T) {
	vs, _, sup, heads, err := decodeAPIBibleChapter(json.RawMessage(smallCapsHeadedChapter), "Psalms", 150)
	if err != nil {
		t.Fatal(err)
	}

	// The title: the publisher's letters kept, the span recorded, the
	// small capitals drawn.
	if want := "A fixture song, when the Lord answered."; sup.Text != want {
		t.Errorf("title = %q, want %q", sup.Text, want)
	}
	noSentinels(t, "the title", sup.Text)
	at := len([]rune("A fixture song, when the "))
	if want := []TextSpan{{Start: at, End: at + 4}}; !reflect.DeepEqual(sup.SmallCaps, want) {
		t.Errorf("title small capitals = %+v, want %+v", sup.SmallCaps, want)
	}
	if want := "A fixture song, when the Lᴏʀᴅ answered."; sup.DrawnText() != want {
		t.Errorf("title drawn = %q, want %q", sup.DrawnText(), want)
	}

	if len(heads) != 5 {
		t.Fatalf("got %d headings, want 5: %+v", len(heads), heads)
	}
	for _, h := range heads {
		noSentinels(t, "heading "+h.Style, h.Text)
	}
	cases := []struct {
		text, drawn string
		caps        []TextSpan
	}{
		{"The Lord Keeps the Fixture", "The Lᴏʀᴅ Keeps the Fixture", []TextSpan{{Start: 4, End: 8}}},
		// A span of capitals alone has no lower case to work on, so its
		// capitals are the letters that shrink, as in a verse.
		{"The Lord GOD Is Fixed", "The Lord Gᴏᴅ Is Fixed", []TextSpan{{Start: 10, End: 12}}},
		// Nothing marked: the letter and its name, unchanged.
		{"א Aleph", "א Aleph", nil},
		// The publisher's own small capitals on a letter's name.
		{"ב Beth", "ב Bᴇᴛʜ", []TextSpan{{Start: 2, End: 6}}},
		// A supplied-word span in a heading is read as plain words, as before.
		{"A Heading with Supplied Words", "A Heading with Supplied Words", nil},
	}
	for i, c := range cases {
		h := heads[i]
		if h.Text != c.text {
			t.Errorf("heading %d text = %q, want %q", i, h.Text, c.text)
		}
		if !reflect.DeepEqual(h.SmallCaps, c.caps) {
			t.Errorf("heading %q small capitals = %+v, want %+v", c.text, h.SmallCaps, c.caps)
		}
		if h.DrawnText() != c.drawn {
			t.Errorf("heading %q drawn = %q, want %q", c.text, h.DrawnText(), c.drawn)
		}
	}
	if heads[0].BeforeVerse != 1 || heads[1].BeforeVerse != 2 {
		t.Errorf("headings stand above verses %d and %d, want 1 and 2", heads[0].BeforeVerse, heads[1].BeforeVerse)
	}

	// The control: the verse's own span is what it always was, and no
	// heading or title word reached a verse.
	if len(vs) != 2 {
		t.Fatalf("got %d verses, want 2", len(vs))
	}
	if want := "Fixture praise to the Lord."; vs[0].Text != want {
		t.Errorf("verse 1 = %q, want %q", vs[0].Text, want)
	}
	if want := []TextSpan{{Start: 22, End: 26}}; !reflect.DeepEqual(vs[0].SmallCaps, want) {
		t.Errorf("verse 1 small capitals = %+v, want %+v", vs[0].SmallCaps, want)
	}
}

// On the passages endpoint a heading and a title are read while the decoder is
// still in the previous chapter; their small capitals must travel with them.
func TestDecodeAPIBiblePassageCarriesHeadingSmallCapitalsAcrossChapters(t *testing.T) {
	raw := passageOf(psalm3LastVersePara,
		`{"name":"para","type":"tag","attrs":{"style":"d"},"items":[{"type":"text","text":"A fixture title for the "},`+
			`{"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord"}]}]}`,
		`{"name":"para","type":"tag","attrs":{"style":"s1"},"items":[{"name":"char","type":"tag","attrs":{"style":"nd"},"items":[{"type":"text","text":"Lord"}]},{"type":"text","text":", Hear the Fixture"}]}`,
		psalm4FirstVersePara)
	_, _, sups, heads, err := decodeAPIBiblePassage(raw, "Psalms", 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := sups[4].DrawnText(); got != "A fixture title for the Lᴏʀᴅ" {
		t.Errorf("Psalm 4's title drawn = %q", got)
	}
	if len(heads[4]) != 1 || heads[4][0].DrawnText() != "Lᴏʀᴅ, Hear the Fixture" || heads[4][0].Text != "Lord, Hear the Fixture" {
		t.Errorf("Psalm 4's headings = %+v", heads[4])
	}
	if len(heads[3]) != 0 {
		t.Errorf("Psalm 3 took a heading: %+v", heads[3])
	}
}

// A bracket that stands apart from its words would leave a stray space where it
// stood. The words are kept as the publisher spaced them and the span let go,
// rather than a span that marks the wrong letters.
func TestSettleMarkedTextLetsGoOfAStrayBracket(t *testing.T) {
	stray := "The " + string(smallCapsOpen) + " Lord" + string(smallCapsClose) + " Reigns"
	text, anchors, caps := settleMarkedText(stray)
	if text != "The Lord Reigns" || caps != nil || anchors != nil {
		t.Errorf("stray bracket: text %q, caps %+v, anchors %v", text, caps, anchors)
	}
	tight := "The " + string(smallCapsOpen) + "Lord" + string(smallCapsClose) + " Reigns"
	text, _, caps = settleMarkedText(tight)
	if text != "The Lord Reigns" || !reflect.DeepEqual(caps, []TextSpan{{Start: 4, End: 8}}) {
		t.Errorf("tight bracket: text %q, caps %+v", text, caps)
	}
	// A note in a title keeps its anchor beside the small capitals.
	noted := "A title" + string(footnoteSentinel) + " for the " + string(smallCapsOpen) + "Lord" + string(smallCapsClose)
	text, anchors, caps = settleMarkedText(noted)
	if text != "A title for the Lord" || !reflect.DeepEqual(anchors, []int{7}) ||
		!reflect.DeepEqual(caps, []TextSpan{{Start: 16, End: 20}}) {
		t.Errorf("noted title: text %q, anchors %v, caps %+v", text, anchors, caps)
	}
}

// smallCapsHeadedState is a chapter whose heading and title carry the divine
// name in small capitals, as the decoder now records them.
func smallCapsHeadedState() *AppState {
	bd := NewBibleData()
	bd.Books = []string{"Psalms"}
	bd.Verses["Psalms"] = map[int][]Verse{150: {
		{BookName: "Psalms", Chapter: 150, Verse: 1, Text: "Fixture praise to the Lord.", ParaStart: true,
			SmallCaps: []TextSpan{{Start: 22, End: 26}}},
		{BookName: "Psalms", Chapter: 150, Verse: 2, Text: "A second fixture line."},
	}}
	bd.Headings = map[string]map[int][]Heading{"Psalms": {150: {
		{Text: "The Lord Keeps the Fixture", Style: "s1", BeforeVerse: 2, SmallCaps: []TextSpan{{Start: 4, End: 8}}},
	}}}
	bd.Superscriptions = map[string]map[int]Superscription{"Psalms": {150: {
		Text: "A fixture song, when the Lord answered.", SmallCaps: []TextSpan{{Start: 25, End: 29}},
	}}}
	return &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 150}
}

// Every surface draws the heading and the title through DrawnText: the Apple
// panes' HTML, Android's, the Windows and Linux pane, and the blocks the web
// reader renders. The control is the same chapter with the spans removed,
// which every surface must draw exactly as the publisher's letters.
func TestEverySurfaceDrawsAHeadingsSmallCapitals(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	const heading, title = "The Lᴏʀᴅ Keeps the Fixture", "A fixture song, when the Lᴏʀᴅ answered."
	for _, marked := range []bool{true, false} {
		st := smallCapsHeadedState()
		wantHeading, wantTitle := heading, title
		if !marked {
			st.Bible.Headings["Psalms"][150][0].SmallCaps = nil
			sup := st.Bible.Superscriptions["Psalms"][150]
			sup.SmallCaps = nil
			st.Bible.Superscriptions["Psalms"][150] = sup
			wantHeading, wantTitle = "The Lord Keeps the Fixture", "A fixture song, when the Lord answered."
		}
		verses := st.Bible.GetChapter("Psalms", 150)

		apple := buildChapterHTML(st, verses)
		if !strings.Contains(apple, `<p class="sec">`+wantHeading+`</p>`) {
			t.Errorf("marked=%v: the Apple pane's heading is not %q", marked, wantHeading)
		}
		if !strings.Contains(apple, `">`+wantTitle+`</p>`) {
			t.Errorf("marked=%v: the Apple pane's title is not %q", marked, wantTitle)
		}
		android := buildChapterHTMLAndroid(st, verses)
		if !strings.Contains(android, "<p><b>"+wantHeading+"</b></p>") {
			t.Errorf("marked=%v: Android's heading is not %q", marked, wantHeading)
		}
		if !strings.Contains(android, "<i>"+wantTitle+"</i>") {
			t.Errorf("marked=%v: Android's title is not %q", marked, wantTitle)
		}
		lay := layoutChapter(st, verses, testLayoutParams, fixedMeasure)
		var drawn []string
		for _, ln := range lay.Lines {
			if ln.Heading != "" {
				drawn = append(drawn, ln.Heading)
			}
		}
		if got := strings.Join(drawn, " "); got != wantHeading {
			t.Errorf("marked=%v: the Windows and Linux pane lays the heading out as %q, want %q", marked, got, wantHeading)
		}
		if got := newStyledReadingPane(st, verses).superText; got != wantTitle {
			t.Errorf("marked=%v: the Windows and Linux pane's title is %q, want %q", marked, got, wantTitle)
		}
		var blocks []string
		for _, b := range ChapterBlocks(st.Bible, "Psalms", 150, verses) {
			if b.HeadingText != "" {
				blocks = append(blocks, b.HeadingText)
			}
		}
		if len(blocks) != 1 || blocks[0] != wantHeading {
			t.Errorf("marked=%v: the web reader's heading blocks are %q, want [%q]", marked, blocks, wantHeading)
		}
	}
}

// A selection is taken from the DRAWN page, where the heading's divine name is
// set in small capitals, so the heading repair must look for the heading as
// drawn. Looking for the stored letters, it found nothing, and a heading-led
// selection reached back to the verse above.
func TestTheHeadingRepairFindsAHeadingAsItIsDrawn(t *testing.T) {
	st := smallCapsHeadedState()
	under := "2 A second fixture line."

	got := selectionVersesIn(st, "Psalms", 150, "The Lᴏʀᴅ Keeps the Fixture "+under, selSpanFromNative(1, 2))
	if len(got) != 1 || got[0].Verse != 2 {
		t.Errorf("a selection led by the drawn heading resolved to %s, want just 2", verseRunString(got))
	}
	alone := selectionVersesIn(st, "Psalms", 150, "The Lᴏʀᴅ Keeps the Fixture", selSpanFromNative(1, 1))
	if len(alone) != 1 || alone[0].Verse != 2 {
		t.Errorf("the drawn heading selected alone resolved to %s, want just 2", verseRunString(alone))
	}
	// The control: the same heading stored without its span draws as plain
	// letters, and the plain selection is repaired exactly as before.
	st.Bible.Headings["Psalms"][150][0].SmallCaps = nil
	plain := selectionVersesIn(st, "Psalms", 150, "The Lord Keeps the Fixture "+under, selSpanFromNative(1, 2))
	if len(plain) != 1 || plain[0].Verse != 2 {
		t.Errorf("control: a plain heading-led selection resolved to %s, want just 2", verseRunString(plain))
	}
}
