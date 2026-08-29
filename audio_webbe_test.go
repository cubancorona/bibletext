package bibletext

import (
	"strconv"
	"strings"
	"testing"
)

// TestWEBBEAudioURL pins the file scheme and, more importantly, the boundary
// between the two WEB-Catholic recordings: the synthetic WEBBE narration answers
// for the Greek books, and stays out of every chapter David Williams actually read.
func TestWEBBEAudioURL(t *testing.T) {
	const base = testAudioHost + "webbe-synthetic-v1/"
	for _, c := range []struct {
		book    string
		chapter int
		want    string
		ok      bool
	}{
		{"Tobit", 1, base + "eng-webbe_041_TOB_01.mp3", true},
		{"Judith", 16, base + "eng-webbe_042_JDT_16.mp3", true},
		{"Esther", 1, base + "eng-webbe_043_ESG_01.mp3", true},  // the GREEK Esther
		{"Esther", 10, base + "eng-webbe_043_ESG_10.mp3", true}, // 14 verses, not the Hebrew's 3
		{"Sirach", 51, base + "eng-webbe_046_SIR_51.mp3", true},
		{"Baruch", 6, base + "eng-webbe_047_BAR_06.mp3", true},
		{"2 Maccabees", 15, base + "eng-webbe_053_2MA_15.mp3", true},
		{"Daniel", 3, base + "eng-webbe_066_DAG_03.mp3", true},  // Azariah + Song of the Three
		{"Daniel", 13, base + "eng-webbe_066_DAG_13.mp3", true}, // Susanna
		{"Daniel", 14, base + "eng-webbe_066_DAG_14.mp3", true}, // Bel and the Dragon
		{"Daniel", 1, "", false},                                // Williams reads these
		{"Daniel", 12, "", false},
		{"John", 3, "", false},   // not a Greek book at all
		{"Tobit", 15, "", false}, // past the end of the book
		{"Tobit", 0, "", false},
	} {
		got, ok := webbeAudioURL(c.book, c.chapter)
		if got != c.want || ok != c.ok {
			t.Errorf("webbeAudioURL(%q,%d) = (%q,%v), want (%q,%v)", c.book, c.chapter, got, ok, c.want, c.ok)
		}
	}
}

// TestWEBCRecordingsNeverOverlap is the invariant that keeps the WEB-Catholic's two
// recordings honest: every chapter must be claimed by AT MOST one of them, so the
// reader is never offered a choice between two narrations of the same words and the
// preference order can route without ambiguity. Driven by the bundled timing table
// rather than a hand-written list, so adding chapters to the recording re-checks it.
func TestWEBCRecordingsNeverOverlap(t *testing.T) {
	loadTimings()
	tbl := allTimings[webbeRecordingID]
	if len(tbl) == 0 {
		t.Fatal("no bundled webbe timing table — the asset is missing or empty")
	}
	n := 0
	for book, chs := range tbl {
		for chStr := range chs {
			ch, err := strconv.Atoi(chStr)
			if err != nil {
				t.Fatalf("non-numeric chapter %q in the webbe table for %s", chStr, book)
			}
			n++
			if _, ok := webbeAudioURL(book, ch); !ok {
				t.Errorf("%s %d is in the webbe timing table but webbeAudioURL declines it", book, ch)
			}
			if _, ok := webcAudioURL(book, ch); ok {
				t.Errorf("%s %d is claimed by BOTH the Williams and the WEBBE recording", book, ch)
			}
		}
	}
	if n != 150 {
		t.Errorf("webbe timing table covers %d chapters, want 150", n)
	}
	if len(tbl) != 9 {
		t.Errorf("webbe timing table covers %d books, want 9", len(tbl))
	}
}

// TestWEBCUsesSyntheticVoiceForGreekBooks checks the whole resolution path, not just
// the URL builder: a Greek-book chapter must resolve to the synthetic recording and
// credit it as such, while the protocanon still gets the human narrator and the
// plain WEB is untouched by any of it.
func TestWEBCUsesSyntheticVoiceForGreekBooks(t *testing.T) {
	bd := &BibleData{
		Books: []string{"Tobit", "Daniel", "Esther", "John"},
		Verses: map[string]map[int][]Verse{
			"Tobit":  {1: {{Text: "The book of the words of Tobit"}}},
			"Daniel": {3: {{Text: "Nebuchadnezzar the king made an image"}}, 12: {{Text: "At that time Michael shall stand up"}}},
			"Esther": {1: {{Text: "In the second year of the reign of Ahasuerus"}}},
			"John":   {3: {{Text: "There was a man of the Pharisees"}}},
		},
	}
	for _, c := range []struct {
		book     string
		chapter  int
		wantKind audioKind
		wantRec  string
		wantSub  string
	}{
		{"Tobit", 1, audioRecorded, webbeRecordingID, "World English Bible (Catholic) · Synthetic voice"},
		{"Esther", 1, audioRecorded, webbeRecordingID, "World English Bible (Catholic) · Synthetic voice"},
		{"Daniel", 3, audioRecorded, webbeRecordingID, "World English Bible (Catholic) · Synthetic voice"},
		{"Daniel", 12, audioRecorded, "web-williams", "World English Bible (Catholic) · David Williams"},
		{"John", 3, audioRecorded, "web-williams", "World English Bible (Catholic) · David Williams"},
	} {
		a := audioForChapter(&AppState{CurrentVersion: "webc", CurrentBook: c.book, CurrentChapter: c.chapter, Bible: bd})
		if a.Kind != c.wantKind || a.RecordingID != c.wantRec {
			t.Errorf("webc %s %d: kind=%d rec=%q, want kind=%d rec=%q", c.book, c.chapter, a.Kind, a.RecordingID, c.wantKind, c.wantRec)
		}
		if a.Subtitle != c.wantSub {
			t.Errorf("webc %s %d: subtitle=%q, want %q", c.book, c.chapter, a.Subtitle, c.wantSub)
		}
	}
	// The plain WEB gains nothing from any of this — it has no Greek books.
	if recs := recordingsFor("web"); len(recs) != 1 || recs[0].id != "web-williams" {
		t.Errorf("recordingsFor(web) = %+v, want only web-williams", recs)
	}
	// ...and the WEB-Catholic lists Williams FIRST, so he wins any chapter he covers.
	recs := recordingsFor("webc")
	if len(recs) != 2 || recs[0].id != "web-williams" || recs[1].id != webbeRecordingID {
		t.Errorf("recordingsFor(webc) = %+v, want [web-williams, %s]", recs, webbeRecordingID)
	}
}

// TestRecordedURLsComeFromTheConfiguredHost guards the fork contract in product.go:
// a fork changes config/product.json "and nothing else in Go". product_test.go only
// checks that audioHostBase equals product.AudioBase — nothing checked that the URL
// builders actually USE it, so a builder with the host pasted in as a literal would
// pass every existing test and only fail on a fork's first play. Every recording is
// exercised through its own timing table, so a new one is covered automatically.
func TestRecordedURLsComeFromTheConfiguredHost(t *testing.T) {
	loadTimings()
	seen := 0
	for _, version := range []string{"bsb", "web", "webc"} {
		for _, r := range recordingsFor(version) {
			tbl := allTimings[r.id]
			if len(tbl) == 0 {
				t.Errorf("%s: recording %q has no bundled timing table", version, r.id)
				continue
			}
			for book, chs := range tbl {
				for chStr := range chs {
					ch, err := strconv.Atoi(chStr)
					if err != nil {
						continue
					}
					url, ok := r.urlFor(book, ch)
					if !ok {
						continue // another recording on this version owns the chapter
					}
					if !strings.HasPrefix(url, audioHostBase) {
						t.Errorf("%s/%s %s %d: URL %q does not start with the configured host %q",
							version, r.id, book, ch, url, audioHostBase)
					}
					seen++
					break // one covered chapter per book is enough
				}
			}
		}
	}
	if seen == 0 {
		t.Fatal("no recorded URLs were checked — the guard proved nothing")
	}
}
