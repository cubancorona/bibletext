//go:build !race

package bibletext

// EVERY SHEET, ON EVERY PHONE HELD SIDEWAYS, KEEPS CLEAR OF THE SIDE INSETS.
//
// The whole matrix: each sheet a phone opens over the page on its own (the
// names BIBLETEXT_DEV_OPEN takes, which TestTheSideInsetSheetsAreTheDevOpenSheets
// holds this table to) and the notes question that opens over Settings, over
// the full-screen Read tab and over Books, on two iPhones and on an Android
// phone with a cutout or a navigation bar at one side. About three hundred
// openings take a few seconds, and far longer under the race detector, which
// has nothing to find in layout arithmetic on the test's own goroutine; CI's
// run without the detector runs them. The harness is sheet_side_insets_test.go.

import (
	"slices"
	"testing"
)

// insetSheet is one sheet the matrix opens, by the name a development launch
// opens it with.
type insetSheet struct {
	name string
	open func(*testing.T, *AppState)
}

// notInsetSheets are the names BIBLETEXT_DEV_OPEN takes that the matrix opens
// under another name, and why.
var notInsetSheets = map[string]string{
	"votd": "today's passage, on the card votd-one and votd-long open (showVerseOfDayCard)",
}

// insetSheets is every sheet the matrix opens, each as the reader opens it,
// in its tallest and shortest forms where its height is data. The two that
// change the reader's state to open as they do come last.
func insetSheets(t *testing.T) []insetSheet {
	t.Helper()
	votdSynchronousRemeasure(t)
	holdSheetTimers(t)
	holdNoteSheetTimers(t)
	studies := stubAIActionParked(t)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	t.Cleanup(func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad })
	crossRefsLoad = func() error { return nil }
	const quote = "For God so loved the world"
	oneVerse := dayVerse{Book: "John", Chapter: 3, Lo: 16, Hi: 16, Verses: []Verse{{BookName: "John", Chapter: 3, Verse: 16,
		Text: "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish, but have eternal life."}}}
	return []insetSheet{
		{"settings", func(_ *testing.T, s *AppState) { showAISettings(s) }},
		{"goto", func(_ *testing.T, s *AppState) { showGotoPicker(s) }},
		{"chapters", func(_ *testing.T, s *AppState) { showChapterPicker(s) }},
		{"versions", func(_ *testing.T, s *AppState) { showVersionPicker(s) }},
		{"votd-one", func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, oneVerse) }},
		{"votd-long", func(_ *testing.T, s *AppState) { showVerseOfDayCard(s, longDayPassage()) }},
		{"audio", func(_ *testing.T, s *AppState) { showAudioSourceMenu(s) }},
		{"note", func(_ *testing.T, s *AppState) { promptShareNote(s, quote, selSpan{}) }},
		{"ask", func(_ *testing.T, s *AppState) { promptAskQuestion(s, quote) }},
		{"ai-waiting", func(t *testing.T, s *AppState) {
			showAIPanel(s, aiActionExplain, quote, "")
			waitParked(t, studies, "the study request")
		}},
		{"xrefs-waiting", func(_ *testing.T, s *AppState) {
			crossRefsRun = func(func()) {} // the load never lands
			showCrossRefs(s, quote, selSpan{})
		}},
		{"share-image", func(_ *testing.T, s *AppState) { showShareImagePreview(s, quote, "John 3:16", "WEB") }},
		{"note-offer", func(_ *testing.T, s *AppState) {
			offerNoteLinkChoice(s, "https://example.invalid/web/john/3/16",
				ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16})
		}},
		{"link-notice", func(_ *testing.T, s *AppState) {
			showLinkNotice(s, "This passage isn't available", "John 3:16",
				"The link names a translation this reader does not have.")
		}},
		{"version-loading", func(_ *testing.T, s *AppState) { showVersionLoading(s, "Sample Translation") }},
		{"version-error", func(_ *testing.T, s *AppState) { showVersionLoadError(s, "Sample Translation") }},
		// Over Settings, never over the page on its own; the delete-all
		// question is the same card at the same width.
		{"notes-keep-or-delete", func(_ *testing.T, s *AppState) { promptKeepOrDeleteNotes(s, func() {}, func() {}) }},
		{"versions-more", func(t *testing.T, s *AppState) {
			withMoreTranslations(t)
			withEveryNotice(t, s)
			showVersionPicker(s)
		}},
		{"xrefs", func(t *testing.T, s *AppState) {
			crossRefsRun = func(work func()) { work() }
			withBeatitudes(s)
			showCrossRefs(s, "", selSpan{lo: 3, hi: 3})
			if list := findScroll(s.window.Canvas().Overlays().Top()); list == nil || !list.Visible() {
				t.Fatal("control: the panel should be showing its list")
			}
		}},
	}
}

// THE MATRIX OPENS EVERY SHEET A LAUNCH CAN NAME. A sheet added to the
// development table, and so to devOpenNames, without a row here fails, as
// does a row whose name names no sheet a launch can open.
func TestTheSideInsetSheetsAreTheDevOpenSheets(t *testing.T) {
	var names []string
	for _, sh := range insetSheets(t) {
		names = append(names, sh.name)
	}
	for _, n := range devOpenNames {
		if _, elsewhere := notInsetSheets[n]; !elsewhere && !slices.Contains(names, n) {
			t.Errorf("the matrix does not open %q", n)
		}
	}
	for _, n := range names {
		if !slices.Contains(devOpenNames, n) && n != "notes-keep-or-delete" {
			t.Errorf("the matrix opens %q, which no launch names", n)
		}
	}
}

// NO SHEET DRAWS UNDER A SIDE INSET. On each phone held sideways, over the
// full-screen Read tab and over Books, each sheet is opened on a fresh
// window and nothing it draws lies under the left or right safe inset. The
// iPhones are the 17 Pro Max and the 16 Pro with the insets UIKit reports for
// them on the iOS 26.5 simulator; the Android phone is a 360-point-tall
// screen with a 21-point cutout on its left, and the same phone with
// three-button navigation's 42-point bar on its right. Shown to fail on the
// tree before clearOfSideInsets, for the note composer and the Ask sheet on
// every one of these screens and for no other sheet.
func TestEverySheetKeepsClearOfTheSideInsets(t *testing.T) {
	for _, s := range []insetScreen{
		proMaxSideways,
		{"iPhone 16 Pro, landscape", 874, 402, 0, 20, 62, 62},
		{"Android phone, landscape, cutout on the left", 803, 360, 0, 0, 21, 0},
		{"Android phone, landscape, three-button navigation on the right", 803, 360, 21, 0, 0, 42},
	} {
		for _, tab := range []int{0, 1} {
			for _, sh := range insetSheets(t) {
				st, w := insetWindow(t, s, tab)
				sh.open(t, st)
				popup := topPopup(t, w)
				parts := sheetParts(popup)
				if len(parts) == 0 {
					t.Fatalf("control: %s, tab %d, %s: the sheet draws nothing", s.name, tab, sh.name)
				}
				for _, p := range underASideInset(s, parts) {
					t.Errorf("%s, tab %d, %s draws %s, under a side inset (the safe area runs from x %.0f to %.0f)",
						s.name, tab, sh.name, p, s.left, s.w-s.right)
				}
				popup.Hide()
			}
		}
	}
}
