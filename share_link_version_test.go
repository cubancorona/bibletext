package bibletext

// A shared link opens in the translation it was written against, and the note it
// carries is filed under that translation — not under whatever the reader
// happened to be reading. That filing rule is deliberate.
//
// Verse numbering is NOT interchangeable across translations (see
// versification_test.go: Romans 14/16 genuinely renumber), which is what makes
// this correctness rather than preference — a note filed under the wrong
// translation can point at the wrong text.

import "testing"

func versionLinkState(t *testing.T) *AppState {
	t.Helper()
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	return &AppState{
		Bible:          bd,
		CurrentBook:    "Genesis",
		CurrentChapter: 1,
		CurrentVersion: "web",
		loadPhase:      loadReady,
	}
}

// A link naming the translation already open must not park or switch — it just
// opens, with no spinner and no download.
func TestSameTranslationLinkOpensInline(t *testing.T) {
	st := versionLinkState(t)
	applyShareTarget(st, ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16})

	if st.pendingLink != nil || st.pendingLinkVersion != "" {
		t.Error("a same-translation link was parked; it should apply immediately")
	}
	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("did not navigate: got %s %d", st.CurrentBook, st.CurrentChapter)
	}
}

// An UNKNOWN translation id must degrade to opening in what the reader has,
// rather than parking forever waiting for something that will never load.
func TestUnknownTranslationLinkStillOpens(t *testing.T) {
	st := versionLinkState(t)
	applyShareTarget(st, ShareTarget{VersionID: "nosuchversion", Book: "John", Chapter: 3, VerseLo: 16})

	if st.pendingLink != nil {
		t.Error("parked on a translation that cannot load")
	}
	if st.CurrentBook != "John" {
		t.Errorf("did not open the passage: got %s", st.CurrentBook)
	}
}

// The note is stored under the LINK's translation. Storing it under the reader's
// current one files somebody's remark against wording it was never about.
func TestIncomingNoteIsFiledUnderTheLinksTranslation(t *testing.T) {
	st := versionLinkState(t)
	st.CurrentVersion = "web"
	// Note the deliberate asymmetry: the TRANSLATION comes from the link, but
	// the CHAPTER comes from where the reader actually landed — applyShareTarget
	// clamps a link naming a chapter this canon lacks, and the note belongs on
	// the passage they are looking at. So stand in for that navigation here.
	st.CurrentChapter = 3
	rememberIncomingNote(st, ShareTarget{
		VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16, Note: "this one",
	})

	if _, ok := noteForChapter(appPrefs(), "bsb", "John", 3, nil); !ok {
		t.Error("note was not filed under the link's translation (bsb)")
	}
	// Asked of the STORE, not through the display derive — a reader in the
	// WEB should indeed be shown this note, because notes follow the passage.
	// What must still be true is where it is FILED: under the link's
	// translation, so the note carries the wording it was written about.
	if _, filed := findStoredNote(appPrefs(), "web", "John", 3); filed {
		t.Error("note was filed under the reader's translation instead of the link's")
	}
	deleteAllNotes(appPrefs())
}

// A target parked for one translation must not be applied when a DIFFERENT one
// lands — the reader switched by hand mid-download, or the load fell back
// elsewhere. Applying it would yank them to a passage they no longer asked for.
func TestParkedLinkForAnotherTranslationIsDropped(t *testing.T) {
	st := versionLinkState(t)
	parked := ShareTarget{VersionID: "bsb", Book: "John", Chapter: 3, VerseLo: 16}
	st.pendingLink = &parked
	st.pendingLinkVersion = "bsb"

	// WEBC arrives instead of the BSB we were waiting for.
	other, _ := versionByID("webc")
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	applyLoadedVersion(st, other, bd, modeReal)

	if st.pendingLink != nil || st.pendingLinkVersion != "" {
		t.Error("a stale parked link survived a different translation landing")
	}
	if st.CurrentBook == "John" {
		t.Error("the stale link was applied — the reader was yanked to it")
	}
}

// A shared DEUTEROCANON link must open. ShareLinkURLWithNote forces version=webc
// for those books, so the link names a translation the app can load — but the
// canon guard used to run BEFORE the translation switch, checking Tobit against
// whichever 66-book canon the reader happened to be in and returning silently.
// HandleShareLink still reported success, so the OS did not offer the browser
// either: the tap did nothing at all, with no message.
func TestDeuterocanonLinkIsNotDroppedByTheCanonGuard(t *testing.T) {
	st := versionLinkState(t)

	// Stand in for WEBC being already loaded, so the switch is synchronous.
	webc, ok := versionByID("webc")
	if !ok {
		t.Skip("webc not registered")
	}
	wide := NewBibleData()
	wide.PopulateWithSampleVerses()
	// Give the wider canon a book the reader's current one lacks.
	wide.Books = append(wide.Books, "Tobit")
	if wide.Verses["Tobit"] == nil {
		wide.Verses["Tobit"] = map[int][]Verse{3: {{BookName: "Tobit", Chapter: 3, Verse: 1, Text: "…"}}}
	}
	wide.PrepareSearchIndex()
	st.loadedVersions = map[string]*BibleData{"webc": wide}

	if st.Bible.GetChaptersForBook("Tobit") != 0 {
		t.Fatal("precondition: the starting canon must NOT have Tobit")
	}

	applyShareTarget(st, ShareTarget{VersionID: webc.ID, Book: "Tobit", Chapter: 3, VerseLo: 1})

	if st.CurrentVersion != webc.ID {
		t.Errorf("did not switch to the link's translation: %q", st.CurrentVersion)
	}
	if st.CurrentBook != "Tobit" {
		t.Errorf("the deuterocanon link opened nothing; book = %q", st.CurrentBook)
	}
}

// The same collision from the CONSUMER's side, which is where the guard was
// missing. Two downloads can be in flight at once on a fresh install: the
// four-book seed filling in the reader's own translation, and a translation
// switch that a shared link asked for. Whichever finishes first used to consume
// the park — so an NKJV link could be applied by the WEB download landing, and
// the reader got their own translation's wording with no message at all
// (linkVersionUnavailable says nothing for a translation they can select).
//
// applyShareTarget's park already refused the mirror-image mistake; this pins
// the other half. Found by the state-machine sweep as B_FULLDL_STEALS_PARK, and
// it was introduced when consumePendingLink was wired into triggerFullDownload's
// success tail without the version check applyLoadedVersion has always had.
func TestSeedDownloadDoesNotStealATranslationsParkedLink(t *testing.T) {
	st := versionLinkState(t)
	parked := ShareTarget{VersionID: "nkjv", Book: "John", Chapter: 3, VerseLo: 16}
	st.pendingLink = &parked
	st.pendingLinkVersion = "nkjv" // waiting on a TRANSLATION, not on the seed

	// The production rule itself, not a copy of it — consumeSeedParkedLink is
	// what triggerFullDownload's success tail calls.
	consumeSeedParkedLink(st)

	if st.pendingLink == nil {
		t.Error("the seed download consumed a target that was waiting on another translation")
	}
	if st.pendingLinkVersion != "nkjv" {
		t.Errorf("the park lost the translation it was waiting for: %q", st.pendingLinkVersion)
	}
}

// ...and the case it must NOT break: a link parked because the app was on the
// seed carries no pendingLinkVersion, and the finishing download is exactly what
// it was waiting for. That one has to be honoured, or 62 of the 66 books never
// open from a link on a fresh install.
func TestSeedDownloadHonoursASeedParkedLink(t *testing.T) {
	st := versionLinkState(t)
	parked := ShareTarget{VersionID: "web", Book: "John", Chapter: 3, VerseLo: 16}
	st.pendingLink = &parked
	st.pendingLinkVersion = "" // parked by applyShareTarget's seed branch

	consumeSeedParkedLink(st)

	if st.pendingLink != nil {
		t.Error("a seed-parked link was left parked after the download it was waiting for")
	}
	if st.CurrentBook != "John" || st.CurrentChapter != 3 {
		t.Errorf("the seed-parked link did not open: got %s %d", st.CurrentBook, st.CurrentChapter)
	}
}
