package holybible

// Bible translations (versions). The reader can switch between translations; the
// active one's text lives in AppState.Bible and is swapped on switch. All
// versions share the canonical 66-book structure (see bible.go), so navigation,
// search and the UI need no per-version special-casing.
//
// Licensing. The World English Bible is public domain and comes from the free
// bible-api.com source. NRSV and LSB are copyrighted and require a distribution
// license (see README → "Bible versions"); until a license + credentials are
// configured, those versions serve a clearly-labeled TESTING placeholder so the
// whole flow (switching, reading, search, AI study) can be exercised. The
// retrieval/cache/UI are fully wired — only the licensed provider's HTTP calls
// remain to be filled in (licensedAPISource.fetch).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultVersionID is the translation shown on first launch (public domain, so
// it always loads with no license). It also acts as the structural "base" used
// to shape testing placeholders for not-yet-licensed versions.
const defaultVersionID = "web"

// BibleVersion describes one selectable translation.
type BibleVersion struct {
	ID        string // stable id; also the per-version cache filename ("web","nrsv","lsb")
	Name      string // full name, e.g. "World English Bible"
	Abbrev    string // short label, e.g. "WEB"
	Publisher string // one-line rights/copyright note, shown in the picker

	// PublicDomain marks freely-distributable text (no license required).
	PublicDomain bool

	// source fetches the real, licensed text. When it is unavailable (no
	// license/credentials configured), the app falls back to a clearly-labeled
	// testing placeholder. nil is treated as "never available" (testing only).
	source bibleSource
}

// isTesting reports whether this version currently serves placeholder text
// rather than real scripture (because its licensed source isn't available yet).
func (v BibleVersion) isTesting() bool { return v.source == nil || !v.source.available() }

// registeredVersions is the ordered list shown in the version picker.
var registeredVersions = []BibleVersion{
	{
		ID: "web", Name: "World English Bible", Abbrev: "WEB",
		Publisher: "Public Domain", PublicDomain: true,
		source: webSource{},
	},
	{
		ID: "nrsv", Name: "New Revised Standard Version", Abbrev: "NRSV",
		Publisher: "© National Council of the Churches of Christ — license required",
		source:    newLicensedSource("nrsv"),
	},
	{
		ID: "lsb", Name: "Legacy Standard Bible", Abbrev: "LSB",
		Publisher: "© The Lockman Foundation — license required",
		source:    newLicensedSource("lsb"),
	},
}

func bibleVersions() []BibleVersion { return registeredVersions }

func versionByID(id string) (BibleVersion, bool) {
	for _, v := range registeredVersions {
		if v.ID == id {
			return v, true
		}
	}
	return BibleVersion{}, false
}

// --- Sources ----------------------------------------------------------------

// bibleSource knows how to obtain the full text of one version.
type bibleSource interface {
	// available reports whether this source can return real, licensed text now.
	available() bool
	// fetch returns the complete BibleData (only meaningful when available()).
	fetch() (*BibleData, error)
}

// webSource serves the public-domain World English Bible from bible-api.com.
type webSource struct{}

func (webSource) available() bool            { return true }
func (webSource) fetch() (*BibleData, error) { return FetchBibleFromAPI() }

// licensedAPISource is the structured path for a copyrighted translation served
// by a licensed Bible API (e.g. scripture.api.bible). It activates only when
// BOTH are true: we hold a distribution license for the translation, and the
// provider credentials are configured. This double gate makes it impossible to
// ship copyrighted text by accident. Configuration is via environment so no
// secrets live in the repo:
//
//	BIBLE_API_KEY                  provider API key (shared across versions)
//	HOLY_BIBLE_LICENSE_<ID>=1      explicit "we are licensed for <ID>" opt-in
//	HOLY_BIBLE_PROVIDER_ID_<ID>    the provider's bible id for this translation
//
// (<ID> is the upper-cased version id, e.g. NRSV, LSB.)
type licensedAPISource struct {
	versionID string
}

func newLicensedSource(versionID string) *licensedAPISource {
	return &licensedAPISource{versionID: versionID}
}

func (s *licensedAPISource) apiKey() string { return strings.TrimSpace(os.Getenv("BIBLE_API_KEY")) }

// licensed is the explicit operator opt-in confirming we hold rights to ship
// this translation's text.
func (s *licensedAPISource) licensed() bool {
	return envTruthy(os.Getenv("HOLY_BIBLE_LICENSE_" + strings.ToUpper(s.versionID)))
}

// providerVersionID is the licensed provider's id for this translation.
func (s *licensedAPISource) providerVersionID() string {
	return strings.TrimSpace(os.Getenv("HOLY_BIBLE_PROVIDER_ID_" + strings.ToUpper(s.versionID)))
}

func (s *licensedAPISource) available() bool {
	return s.apiKey() != "" && s.licensed() && s.providerVersionID() != ""
}

func (s *licensedAPISource) fetch() (*BibleData, error) {
	if !s.available() {
		return nil, fmt.Errorf("version %q: licensed source not configured "+
			"(need a distribution license, BIBLE_API_KEY, HOLY_BIBLE_LICENSE_%s=1 and HOLY_BIBLE_PROVIDER_ID_%s)",
			s.versionID, strings.ToUpper(s.versionID), strings.ToUpper(s.versionID))
	}
	// TODO(license): with rights secured, implement the provider call here.
	// Shape (scripture.api.bible): for each book+chapter,
	//   GET https://api.scripture.api.bible/v1/bibles/<providerVersionID>/chapters/<chapterId>?content-type=text
	//   header: api-key: <apiKey>
	// then map verses into BibleData (mirror decodeChapterResponse). Caching,
	// state, switching and the UI already work for real data via loadVersionData.
	return nil, fmt.Errorf("version %q: licensed provider fetch not yet implemented", s.versionID)
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// --- Loading + placeholders -------------------------------------------------

// dataMode distinguishes real scripture from a testing placeholder.
type dataMode int

const (
	modeReal dataMode = iota
	modeTesting
)

// loadVersionData returns the data for a version. Versions whose source is
// available load real text (per-version cache, else fetch + cache). Versions
// without an available source get a clearly-labeled testing placeholder that
// mirrors base's book/chapter/verse structure so navigation and search behave
// realistically.
func loadVersionData(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
	if v.source != nil && v.source.available() {
		data, _, err := loadBibleData(v.source.fetch, cachePathForVersion(v.ID), currentUTCTime)
		if err != nil {
			return nil, modeReal, err
		}
		return data, modeReal, nil
	}
	if base == nil {
		return nil, modeTesting, fmt.Errorf("cannot build %q placeholder: base text not loaded", v.ID)
	}
	return makePlaceholderBible(v, base), modeTesting, nil
}

// makePlaceholderBible clones base's structure with placeholder text so an
// unlicensed version is navigable/searchable without shipping copyrighted text.
func makePlaceholderBible(v BibleVersion, base *BibleData) *BibleData {
	out := &BibleData{
		Verses: make(map[string]map[int][]Verse, len(base.Verses)),
		Books:  append([]string(nil), base.Books...),
	}
	for _, book := range base.Books {
		chapters := base.Verses[book]
		out.Verses[book] = make(map[int][]Verse, len(chapters))
		for chapter, verses := range chapters {
			placeheld := make([]Verse, len(verses))
			for i, src := range verses {
				placeheld[i] = Verse{
					BookName: src.BookName,
					Book:     src.Book,
					Chapter:  src.Chapter,
					Verse:    src.Verse,
					Text:     placeholderVerseText(v.Abbrev, src.BookName, src.Chapter, src.Verse),
				}
			}
			out.Verses[book][chapter] = placeheld
		}
	}
	out.PrepareSearchIndex()
	return out
}

func placeholderVerseText(abbrev, book string, chapter, verse int) string {
	return fmt.Sprintf("[%s sample — licensed text not available in this testing build] %s %d:%d",
		abbrev, book, chapter, verse)
}

// cachePathForVersion is the on-disk cache for a version. The default (web) stays
// at the legacy path (honoring HOLY_BIBLE_CACHE_PATH) for backwards
// compatibility; other versions live beside it as holy-bible-<id>.json.
func cachePathForVersion(id string) string {
	base := defaultCachePath()
	if id == defaultVersionID {
		return base
	}
	return filepath.Join(filepath.Dir(base), "holy-bible-"+id+".json")
}

// --- Switching --------------------------------------------------------------

// switchVersion loads (or reuses) a translation, swaps it into the reader, and
// rebuilds the window so the header, reading pane and sidebar reflect it. The
// canonical 66-book structure is shared across versions, so the open book and
// chapter stay valid. Cached versions and testing placeholders switch instantly;
// a first real licensed fetch would block here — a loading affordance for that
// case is a future refinement (see README → "Bible versions").
func switchVersion(state *AppState, id string) {
	if state == nil || id == state.CurrentVersion {
		return
	}
	v, ok := versionByID(id)
	if !ok {
		return
	}

	data, cached := state.loadedVersions[id]
	mode := modeReal
	if cached {
		if v.isTesting() {
			mode = modeTesting
		}
	} else {
		d, m, err := loadVersionData(v, state.baseBible())
		if err != nil {
			// Keep the current version rather than blanking the reader.
			fmt.Fprintf(os.Stderr, "Holy Bible: could not load %s: %v\n", v.Name, err)
			return
		}
		data, mode = d, m
		if state.loadedVersions == nil {
			state.loadedVersions = map[string]*BibleData{}
		}
		state.loadedVersions[id] = data
	}

	state.Bible = data
	state.CurrentVersion = id
	state.currentMode = mode
	clampToCurrentVersion(state)
	rebuildWindow(state)
}

// clampToCurrentVersion keeps the open book/chapter valid for the active version
// (all versions share the canonical structure, so this is just belt-and-braces).
func clampToCurrentVersion(state *AppState) {
	if state.Bible.GetChaptersForBook(state.CurrentBook) == 0 {
		state.CurrentBook = defaultStartBook(state.Bible)
	}
	normalizeCurrentChapter(state, state.Bible.GetChapterNumbersForBook(state.CurrentBook))
}
