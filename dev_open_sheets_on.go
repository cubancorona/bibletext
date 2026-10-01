//go:build bibletextdev

package bibletext

// The sheets BIBLETEXT_DEV_OPEN can open at launch. DEVELOPMENT BUILDS ONLY:
// the release build has no table and no names (dev_autoopen_off.go).
//
// Every sheet a phone or tablet opens over the page on its own is here (the
// notes questions and the model list open only from Settings, over it), in
// its tallest and shortest forms where its height is data and in the states
// it waits in, so a simulator can show each one without a tap, on a real
// screen's safe area, Dynamic Island and home indicator and in the system's
// own fonts. Each is opened by the function the app opens it with. A state
// the app reaches only through a download or a provider is held by a seam
// the host tests hold it with too, and never reaches the network:
//
//	ai-waiting     the study request is answered by devParkedStudy, which
//	               waits for the panel to abandon it (Close, Cancel, or the
//	               request budget) and sends nothing. It stays in place for
//	               the rest of the launch, so no study request of this run
//	               reaches a provider.
//	xrefs-waiting  the panel's load is never run (crossRefsRun), for this
//	               one opening only.
//	xrefs          the load is run where it stands and answered at once
//	               (crossRefsRun, crossRefsLoad), for this one opening only,
//	               on Matthew 5:3, whose Gospel parallels are embedded: the
//	               Treasury is not fetched, and the list stands at its cap
//	               whatever its length (fitList).
//	versions-more  eight synthetic translations under evaluation are
//	               registered, and every fact the picker's notice can state is
//	               set, while the picker is built, and taken away again before
//	               it returns: nothing is downloaded, and no row can switch
//	               to a translation that is not there.
//
// The long verse-of-the-day passage and the synthetic translations are the
// sheet tests' own fixtures (longDayPassage and withMoreTranslations, in
// sheet_header_clearance_test.go), held equal to them by
// TestDevOpenFixturesAreTheTestsOwn, so the simulator shows the forms the
// host tests measure.

import (
	"context"
	"fmt"
	"strings"
)

// devSheet is one sheet BIBLETEXT_DEV_OPEN names: the name, what it opens,
// and how.
type devSheet struct {
	name, opens string
	open        func(*AppState)
}

// devSheets is every sheet a launch can open, in the order of devOpenNames,
// with today's verse of the day beside the two fixed passages. The two that
// set state up to open as they do, every notice at once and Matthew 5, come
// last.
func devSheets() []devSheet {
	return []devSheet{
		{"settings", "Settings", showAISettings},
		{"goto", "the Go to picker", showGotoPicker},
		{"chapters", "the chapter picker", showChapterPicker},
		{"versions", "the translation picker", showVersionPicker},
		{"votd", "the verse of the day, today's", showVerseOfDay},
		{"votd-one", "the verse of the day card, one verse (John 3:16)", func(s *AppState) {
			showVerseOfDayCard(s, devOneDayVerse())
		}},
		{"votd-long", "the verse of the day card, sixteen verses (Psalm 119:1-16)", func(s *AppState) {
			showVerseOfDayCard(s, devLongDayPassage())
		}},
		{"audio", "the audio source menu", showAudioSourceMenu},
		{"note", "the note composer", func(s *AppState) { promptShareNote(s, devQuote, selSpan{}) }},
		{"ask", "the Ask sheet", func(s *AppState) { promptAskQuestion(s, devQuote) }},
		{"ai-waiting", "the AI answer panel, waiting for its answer", devOpenAIWaiting},
		{"xrefs-waiting", "the cross-references panel, waiting for its list", devOpenCrossRefsWaiting},
		{"share-image", "the share image preview", func(s *AppState) {
			showShareImagePreview(s, devQuote, "John 3:16", s.currentVersion().Name)
		}},
		{"note-offer", "the offer for a link carrying a note", devOpenNoteOffer},
		{"link-notice", "the notice for a link in a translation this reader lacks", func(s *AppState) {
			showLinkVersionUnavailable(s, "New King James Version")
		}},
		{"version-loading", "the translation download spinner", func(s *AppState) {
			showVersionLoading(s, devVersionName())
		}},
		{"version-error", "the translation download failure", func(s *AppState) {
			showVersionLoadError(s, devVersionName())
		}},
		{"versions-more", "the translation picker with eight more translations and every notice", devOpenVersionsMore},
		{"xrefs", "the cross-references panel, listing Matthew 5:3", devOpenCrossRefsListed},
	}
}

// devOpenSheet opens the sheet name names, and reports whether it names one.
func devOpenSheet(state *AppState, name string) bool {
	for _, sh := range devSheets() {
		if sh.name == name {
			sh.open(state)
			return true
		}
	}
	return false
}

// devOpenSheetNames lists the names, for the message a launch that names no
// sheet prints.
func devOpenSheetNames() string {
	var names []string
	for _, sh := range devSheets() {
		names = append(names, sh.name)
	}
	return strings.Join(names, ", ")
}

// devQuote is the selection the sheets that quote one are opened on: John
// 3:16 in the World English Bible, which is in the public domain.
const devQuote = "For God so loved the world, that he gave his one and only Son, that whoever believes in him should not perish, but have eternal life."

// devOneDayVerse is the verse of the day at its shortest ordinary form: one
// verse, John 3:16.
func devOneDayVerse() dayVerse {
	return dayVerse{Book: "John", Chapter: 3, Lo: 16, Hi: 16, Verses: []Verse{{BookName: "John", Chapter: 3, Verse: 16,
		Text: devQuote}}}
}

// devLongDayPassage is a verse of the day long enough to stand at its cap on
// every screen but a tall one (longDayPassage).
func devLongDayPassage() dayVerse {
	d := dayVerse{Book: "Psalms", Chapter: 119, Lo: 1, Hi: 16}
	for v := d.Lo; v <= d.Hi; v++ {
		d.Verses = append(d.Verses, Verse{BookName: "Psalms", Chapter: 119, Verse: v,
			Text: "Blessed are those whose ways are blameless, who walk according to Yahweh’s law."})
	}
	return d
}

// devVersionName is the translation the download sheets name: the default's.
func devVersionName() string {
	if v, ok := versionByID(defaultVersionID); ok {
		return v.Name
	}
	return defaultVersionID
}

// devStudyWait is how devParkedStudy waits: until the request's context ends.
// A variable so the host test can park the request for good, as the suite's
// study stubs do, and no completion runs while it reads the panel.
var devStudyWait = func(ctx context.Context) { <-ctx.Done() }

// devParkedStudy answers a study request without asking anyone: it waits
// until the panel abandons the request and returns why.
func devParkedStudy(ctx context.Context, _ *AppState, _, _, _ string) (string, error) {
	devStudyWait(ctx)
	return "", ctx.Err()
}

// devOpenAIWaiting opens the AI answer panel on an Explain request that is
// never sent, so the panel holds its waiting state.
func devOpenAIWaiting(state *AppState) {
	aiActionRun = devParkedStudy
	showAIPanel(state, aiActionExplain, devQuote, "")
}

// devOpenCrossRefsWaiting opens the cross-references panel with its load never
// run, so the panel holds its waiting state.
func devOpenCrossRefsWaiting(state *AppState) {
	prev := crossRefsRun
	crossRefsRun = func(func()) {}
	defer func() { crossRefsRun = prev }()
	showCrossRefs(state, devQuote, selSpan{})
}

// devOpenCrossRefsListed opens the cross-references panel on Matthew 5:3 with
// its list in place: the load runs where it stands and loads nothing, so the
// list is the embedded Gospel parallels (and the Treasury only if it was
// already loaded).
func devOpenCrossRefsListed(state *AppState) {
	navigateToReference(state, "Matthew", 5)
	prevRun, prevLoad := crossRefsRun, crossRefsLoad
	crossRefsRun = func(work func()) { work() }
	crossRefsLoad = func() error { return nil }
	defer func() { crossRefsRun, crossRefsLoad = prevRun, prevLoad }()
	showCrossRefs(state, "", selSpan{lo: 3, hi: 3})
}

// devOpenNoteOffer opens the offer a link carrying a note makes where notes
// are turned off, for a link to John 3:16 in the translation on screen.
func devOpenNoteOffer(state *AppState) {
	raw := ShareLinkURLWithNote(state.currentVersion().ID, "John", 3, 16, 16, "Fixture note carried by a shared link.")
	t, ok := ParseShareLink(raw)
	if !ok {
		t = ShareTarget{VersionID: state.currentVersion().ID, Book: "John", Chapter: 3, VerseLo: 16, VerseHi: 16}
	}
	offerNoteLinkChoice(state, raw, t)
}

// devMoreTranslations is how many synthetic translations versions-more adds
// (moreTranslations).
const devMoreTranslations = 8

// devSampleVersions are the synthetic translations under evaluation that
// versions-more registers (withMoreTranslations).
func devSampleVersions() []BibleVersion {
	var vs []BibleVersion
	for i := 0; i < devMoreTranslations; i++ {
		id := fmt.Sprintf("sample-%c", 'a'+i)
		vs = append(vs, BibleVersion{
			ID: id, Name: fmt.Sprintf("Sample Translation %c", 'A'+i), Abbrev: strings.ToUpper(id),
			Publisher: "Sample Publisher — license required",
			source:    newLicensedSource(id),
		})
	}
	return vs
}

// devEveryNotice is the picker's notice with every fact it can state at once,
// one per line (withEveryNotice): the reader's choice could not be opened,
// the default translation is updating, and every registered translation is
// on a previous edition. It is read from state set for the purpose and put
// back before it returns.
func devEveryNotice(state *AppState) string {
	cur, pref := state.CurrentVersion, state.preferredVersion
	pending, downloading, seed, stale := state.fullPending, state.fullDownloading, state.seedOnly, state.staleVersions
	defer func() {
		state.CurrentVersion, state.preferredVersion = cur, pref
		state.fullPending, state.fullDownloading, state.seedOnly, state.staleVersions = pending, downloading, seed, stale
	}()
	vs := bibleVersions()
	state.CurrentVersion = defaultVersionID
	state.preferredVersion = vs[len(vs)-1].ID
	state.fullPending, state.fullDownloading, state.seedOnly = true, true, false
	state.staleVersions = map[string]bool{}
	for _, v := range vs {
		state.staleVersions[v.ID] = true
	}
	return fullPendingNotice(state)
}

// devOpenVersionsMore opens the translation picker at its tallest: eight more
// translations under evaluation and the three-line notice. The picker is
// built from the registry and the notice as they stand when it opens, and
// both are put back before this returns. It skips showVersionPicker's retry
// of a pending download, which is the reader's opening asking, not a sheet.
func devOpenVersionsMore(state *AppState) {
	prev := registeredVersions
	registeredVersions = append(append([]BibleVersion(nil), prev...), devSampleVersions()...)
	defer func() { registeredVersions = prev }()
	showVersionPickerWith(state, devEveryNotice(state))
}
