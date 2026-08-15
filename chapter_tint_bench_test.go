package bibletext

// The allocation budget for the tint seam.
//
// chapterTint runs once per chapter render, and a chapter render is the iOS
// scroll-lag path: chapterRenderFingerprint exists precisely so this work can be
// skipped, and reading_ios.go pays a 20-36 ms NSAttributedString re-import when
// it cannot be. So the seam is measured, not assumed.
//
// Deliberately self-contained — its own fixture, no helper from any other test
// file — so the identical file can be dropped into a checkout of the commit
// BEFORE the seam existed and benchmarked there. That is the only way "no
// allocation regression" is a fact rather than a claim.
//
//	go test -run '^$' -bench ChapterTint -benchmem .

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2/test"
)

// benchChapter is Psalm-119-scale: 176 verses, the longest chapter in the
// canon and the one reading_ios.go's own timings are quoted against.
func benchChapter() []Verse {
	verses := make([]Verse, 0, 176)
	for i := 1; i <= 176; i++ {
		verses = append(verses, Verse{
			BookName: "Psalms", Chapter: 119, Verse: i,
			Text: fmt.Sprintf("Blessed are those whose way is blameless, who walk in the law of the LORD, verse %d.", i),
		})
	}
	return verses
}

func benchState(marked bool) (*AppState, []Verse) {
	verses := benchChapter()
	bd := NewBibleData()
	bd.Verses = map[string]map[int][]Verse{"Psalms": {119: verses}}
	bd.Books = []string{"Psalms"}
	bd.PrepareSearchIndex()
	st := &AppState{Bible: bd, CurrentBook: "Psalms", CurrentChapter: 119, CurrentVersion: "web"}
	if marked {
		st.setHL(hlSearch, "Psalms", 119, 40, 60)
	}
	return st, verses
}

func benchBothMarks(b *testing.B, name string, fn func(*AppState, []Verse)) {
	for _, marked := range []bool{false, true} {
		label := "unmarked"
		if marked {
			label = "marked"
		}
		st, verses := benchState(marked)
		b.Run(name+"/"+label, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				fn(st, verses)
			}
		})
	}
}

// BenchmarkChapterTintApplePane is buildChapterHTML — the iOS/macOS overlay's
// whole render.
func BenchmarkChapterTintApplePane(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	benchBothMarks(b, "apple-html", func(s *AppState, v []Verse) {
		sinkString = buildChapterHTML(s, v)
	})
}

func BenchmarkChapterTintAndroidPane(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	benchBothMarks(b, "android-html", func(s *AppState, v []Verse) {
		sinkString = buildChapterHTMLAndroid(s, v)
	})
}

func BenchmarkChapterTintStyledLayout(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	measure := func(text string, kind runKind) float32 { return float32(len(text)) * 7 }
	benchBothMarks(b, "styled-layout", func(s *AppState, v []Verse) {
		sinkLayout = layoutChapter(s, v, styledLayoutParams{
			Width: 700, LineHeight: 24, ParaGap: 12, SpaceW: 4,
		}, measure)
	})
}

func BenchmarkChapterTintRichText(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	benchBothMarks(b, "richtext", func(s *AppState, v []Verse) {
		sinkSegs = len(mobileParagraphSegments(s, v))
	})
}

// BenchmarkChapterTintFingerprint is the gate itself — the cheapest and most
// frequent of the lot, and the one a per-render allocation would show up in
// first, since it runs on every tab-return-to-Read whether or not anything
// changed.
func BenchmarkChapterTintFingerprint(b *testing.B) {
	app := test.NewApp()
	defer app.Quit()
	benchBothMarks(b, "fingerprint", func(s *AppState, _ []Verse) {
		sinkString = chapterRenderFingerprint(s)
	})
}

var (
	sinkString string
	sinkLayout *chapterLayout
	sinkSegs   int
)
