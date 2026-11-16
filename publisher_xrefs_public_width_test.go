//go:build !race

package bibletext

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// EVERY NOTED SELECTION IN WEB, WEBC AND BSB FITS THE NARROWEST PHONE.
// Measures all 322 WEB noted verses, all 325 WEBC noted verses (including the
// three deuterocanonical notes) and all 1,325 BSB noted section headings across
// Normal, Large and Extra large against the 283pt narrowest-phone body.
func TestEveryPublicPublisherCrossRefNoteFitsTheNarrowestPhone(t *testing.T) {
	th := appThemeForTest(t)
	limit := narrowestPhoneBody(t)
	t.Cleanup(func() { setReadingTextSizeID("normal") })

	webBD := publicXRefSkeleton(false)
	applyPublisherCrossRefs("web", webBD)
	webcBD := publicXRefSkeleton(true)
	applyPublisherCrossRefs("webc", webcBD)
	bsbBD := publicXRefSkeleton(false)
	applyPublisherCrossRefs("bsb", bsbBD)

	// When BIBLETEXT_XREF_REALDATA=1 and real caches exist, measure over the
	// real downloaded translations so actual verse previews and headings are
	// exercised at every text size as well.
	if os.Getenv("BIBLETEXT_XREF_REALDATA") != "" {
		if bd, err := loadBibleFromCache(realCachePath(cachePathForVersion("web"))); err == nil {
			applyPublisherCrossRefs("web", bd)
			webBD = bd
		}
		if bd, err := loadBibleFromCache(realCachePath(cachePathForVersion("webc"))); err == nil {
			applyPublisherCrossRefs("webc", bd)
			webcBD = bd
		}
		if bd, err := loadBibleFromCache(realCachePath(cachePathForVersion("bsb"))); err == nil {
			applyPublisherCrossRefs("bsb", bd)
			bsbBD = bd
		}
	}

	nop := func(crossRef) {}
	for _, sizeOpt := range textSizeOptions {
		setReadingTextSizeID(sizeOpt.ID)
		checked := 0

		for _, edition := range []struct {
			id    string
			bd    *BibleData
			table map[string]publisherVerseEntry
		}{
			{"web", webBD, webPublisherCrossRefs},
			{"webc", webcBD, webcPublisherCrossRefs},
		} {
			st := &AppState{Bible: edition.bd, CurrentVersion: edition.id, theme: th}
			for key := range edition.table {
				book, ch, vNum, ok := splitPublisherVerseKey(key)
				if !ok {
					continue
				}
				v := edition.bd.GetVerse(book, ch, vNum)
				if v == nil {
					t.Fatalf("%s missing %s", edition.id, key)
				}
				st.CurrentBook, st.CurrentChapter = book, ch
				lst := buildCrossRefList(st, []Verse{*v}, nil, nil, st.pal(), nop)
				if len(lst.Objects) == 0 || !lst.Sections.Publisher {
					t.Fatalf("size %s %s %s did not mount publisher section (%+v)", sizeOpt.ID, edition.id, key, lst.Sections)
				}
				checked++
				if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
					t.Errorf("size %s %s %s: %s", sizeOpt.ID, edition.id, key, strings.Join(problems, "; "))
				}
			}
		}

		bsbSt := &AppState{Bible: bsbBD, CurrentVersion: "bsb", theme: th}
		seenBSB := map[string]bool{}
		for key, list := range bsbPublisherCrossRefs {
			book, ch, ok := splitPublisherChapterKey(key)
			if !ok {
				continue
			}
			for _, hn := range list {
				selKey := key + ":" + strconv.Itoa(hn.BeforeVerse)
				if seenBSB[selKey] {
					continue
				}
				seenBSB[selKey] = true
				v := bsbBD.GetVerse(book, ch, hn.BeforeVerse)
				if v == nil {
					t.Fatalf("bsb missing %s:%d", key, hn.BeforeVerse)
				}
				bsbSt.CurrentBook, bsbSt.CurrentChapter = book, ch
				lst := buildCrossRefList(bsbSt, []Verse{*v}, nil, nil, bsbSt.pal(), nop)
				if len(lst.Objects) == 0 || !lst.Sections.Publisher {
					t.Fatalf("size %s bsb %s:%d did not mount publisher section (%+v)", sizeOpt.ID, key, hn.BeforeVerse, lst.Sections)
				}
				if lst.Passage {
					t.Fatalf("size %s bsb %s:%d repeated the selected passage despite heading-only origins", sizeOpt.ID, key, hn.BeforeVerse)
				}
				for _, note := range publisherCrossRefsFor(bsbBD, book, ch, []Verse{*v}) {
					if note.HeadingText == "" || !strings.Contains(joinedTexts(lst.Objects), note.HeadingText) || !strings.Contains(joinedTexts(lst.Objects), "Heading before verse "+strconv.Itoa(note.Verse)) {
						t.Fatalf("size %s bsb %s:%d lost its actual heading or placement", sizeOpt.ID, key, hn.BeforeVerse)
					}
				}
				checked++
				if problems := crossRefListMisfits(lst.Objects, limit); len(problems) != 0 {
					t.Errorf("size %s bsb %s:%d: %s", sizeOpt.ID, key, hn.BeforeVerse, strings.Join(problems, "; "))
				}
			}
		}

		if checked < 1970 {
			t.Fatalf("size %s checked only %d noted selections; expected >= 1972", sizeOpt.ID, checked)
		}
	}
}

// Live panel behaviour for sparse public-domain publisher cross-references:
//   - On a noted WEB verse (Matthew 5:43), showCrossRefs renders WEB REFERENCES
//     and GOSPEL PARALLELS immediately with the inline spinner under TREASURY
//     OF SCRIPTURE KNOWLEDGE while Treasury downloads.
//   - On an unnoted WEB Gospel verse (Matthew 5:42), the local Gospel
//     parallels still appear immediately, with Treasury loading beneath and
//     no empty publisher section.
func TestPublicPublisherLivePanelImmediateVsSpinner(t *testing.T) {
	app := themedTestApp()
	t.Cleanup(app.Quit)
	withCrossRefIndex(t, "")

	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsRun = func(func()) {} // hold in initial loading state
	crossRefsLoad = func() error { return nil }

	seed, err := loadSeedGospels()
	if err != nil {
		t.Fatal(err)
	}

	// 1. Noted verse (Matthew 5:43) mounts local WEB REFERENCES immediately.
	w1 := test.NewWindow(nil)
	t.Cleanup(w1.Close)
	w1.Resize(fyne.NewSize(390, 844))
	st1 := &AppState{Bible: seed, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web", window: w1}
	showCrossRefs(st1, "", selSpan{lo: 43, hi: 43})
	top1 := w1.Canvas().Overlays().Top()
	if top1 == nil {
		t.Fatal("panel did not open on Matthew 5:43")
	}
	texts1 := strings.Join(treeTexts(top1), "\n")
	for _, want := range []string{"WEB REFERENCES", "Leviticus 19:18", "TREASURY OF SCRIPTURE KNOWLEDGE", "Finding related passages…"} {
		if !strings.Contains(texts1, want) {
			t.Errorf("noted Matthew 5:43 loading state missing %q:\n%s", want, texts1)
		}
	}

	// 2. Unnoted Gospel verse (Matthew 5:42) still shows local Gospel content.
	w2 := test.NewWindow(nil)
	t.Cleanup(w2.Close)
	w2.Resize(fyne.NewSize(390, 844))
	st2 := &AppState{Bible: seed, CurrentBook: "Matthew", CurrentChapter: 5, CurrentVersion: "web", window: w2}
	showCrossRefs(st2, "", selSpan{lo: 42, hi: 42})
	top2 := w2.Canvas().Overlays().Top()
	if top2 == nil {
		t.Fatal("panel did not open on Matthew 5:42")
	}
	texts2 := strings.Join(treeTexts(top2), "\n")
	if strings.Contains(texts2, "WEB REFERENCES") || strings.Contains(texts2, treasuryCredit) {
		t.Errorf("unnoted Matthew 5:42 loading state leaked an absent publisher or Treasury credit: %q", texts2)
	}
	for _, want := range []string{parallelsLabel, treasuryLabel, "Finding related passages…", parallelsCredit} {
		if !strings.Contains(texts2, want) {
			t.Errorf("unnoted Matthew 5:42 loading state missing local source or inline progress %q: %q", want, texts2)
		}
	}
	listScroll, listBox, bar := panelListParts(top2)
	if listScroll == nil || listBox == nil || bar == nil || !bar.Running() {
		t.Fatal("local Gospel content did not mount a scrollable list with inline Treasury progress")
	}
	if len(cardLabelsIn(listBox.Objects)) == 0 {
		t.Fatal("unnoted Gospel selection has no immediately readable destination")
	}
}
