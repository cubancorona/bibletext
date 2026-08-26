package bibletext

// Bible translations (versions). The reader can switch between translations; the
// active one's text lives in AppState.Bible and is swapped on switch. Most versions
// are the 66-book Protestant canon; the World English Bible (Catholic) adds the
// 73-book deuterocanon (see catholic.go). Navigation, search and the UI are data-driven
// off BibleData.Books, so they need no per-version special-casing either way.
//
// Licensing. The World English Bible and Berean Standard Bible are public domain
// and come from the free, key-less bible.helloao.org (one request each). A
// copyrighted translation requires a distribution
// license (see README → "Bible versions"). Until a license + credentials are
// configured it is NOT user-selectable: the picker shows it as "evaluation
// in progress" and tapping is disabled, so a shipped build never exposes
// placeholder text to end users. The full testing/placeholder path stays in the
// code and can be exercised for internal QA by setting BIBLETEXT_ENABLE_TESTING=1
// (see canSelect + testingVersionsEnabled). The retrieval/cache/UI are fully
// wired — only the licensed provider's HTTP calls remain to be filled in
// (licensedAPISource.fetch), at which point the version becomes selectable
// automatically with real text.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultVersionID is the translation shown on first launch (public domain, so
// it always loads with no license). It also acts as the structural "base" used
// to shape testing placeholders for not-yet-licensed versions.
const defaultVersionID = "web"

// BibleVersion describes one selectable translation.
type BibleVersion struct {
	ID        string // stable id; also the per-version cache filename ("web","lsb","nkjv")
	Name      string // full name, e.g. "World English Bible"
	Abbrev    string // short label, e.g. "WEB"
	Publisher string // one-line rights/copyright note, shown in the picker

	// LicenseNotice is the attribution the rights holder requires displayed
	// with the text — shown in the picker once the version is actually
	// licensed and serving real text (versionRow). Empty for public-domain
	// versions, whose Publisher line already says everything.
	LicenseNotice string

	// PublicDomain marks freely-distributable text (no license required).
	PublicDomain bool

	// cacheEpoch invalidates this version's on-disk cache when its DECODING
	// changes (not the cache file format — that is cacheSchemaVersion). The cache
	// stores already-decoded text, so a decoder fix would otherwise stay masked by
	// a stale cache. Bumping the epoch versions the cache path
	// (bibletext-<id>-v<epoch>.json), so existing installs re-fetch and re-decode
	// only THIS version; others keep their caches. 0 = legacy unversioned path.
	cacheEpoch int

	// source fetches the real, licensed text. When it is unavailable (no
	// license/credentials configured), the app falls back to a clearly-labeled
	// testing placeholder. nil is treated as "never available" (testing only).
	source bibleSource
}

// isTesting reports whether this version currently serves placeholder text
// rather than real scripture (because its licensed source isn't available yet).
func (v BibleVersion) isTesting() bool { return v.source == nil || !v.source.available() }

// canSelect reports whether a user may switch to this version. It is true only
// when real text is available — public domain, or licensed *and* configured.
// Versions still in placeholder mode are deliberately NOT selectable in a normal
// build (the picker shows them as "evaluation in progress"), so no copyrighted
// placeholder text is ever exposed to end users. Setting BIBLETEXT_ENABLE_TESTING=1
// unlocks them for internal QA of the placeholder flow.
func (v BibleVersion) canSelect() bool {
	return !v.isTesting() || testingVersionsEnabled()
}

// registeredVersions is the ordered list shown in the version picker.
var registeredVersions = []BibleVersion{
	{
		ID: "web", Name: "World English Bible", Abbrev: "WEB",
		Publisher: "Public Domain", PublicDomain: true,
		// epoch 3: footnote capture (side-band, text unchanged). Epoch 2 was
		// poem-clause line breaks (epoch 1, unreleased, folded in).
		cacheEpoch: 3,
		source:     webSource{},
	},
	{
		ID: "bsb", Name: "Berean Standard Bible", Abbrev: "BSB",
		Publisher: "Public Domain (CC0)", PublicDomain: true,
		// epoch 4: footnote capture (side-band, text unchanged). Epoch 3 was
		// poem-clause line breaks; epoch 1 punctuation spacing.
		cacheEpoch: 4,
		source:     bsbSource{},
	},
	{
		ID: "webc", Name: "World English Bible (Catholic)", Abbrev: "WEBC",
		Publisher: "Public Domain", PublicDomain: true,
		// 73-book Catholic canon (deuterocanon) from bible.helloao.org, decoded by
		// USFM id into traditional Catholic order — see catholic.go.
		cacheEpoch: 3, // epoch 3: footnote capture; epoch 2 poem-clause line breaks
		source:     webCatholicSource{},
	},
	{
		ID: "nkjv", Name: "New King James Version", Abbrev: "NKJV",
		Publisher: "© Thomas Nelson (HarperCollins Christian) — license required",
		// The notice Thomas Nelson requires with NKJV text, plus the visible
		// API.Bible citation the Starter plan requires. Rendered by the picker
		// only while the version is licensed and serving real text.
		LicenseNotice: "Scripture taken from the New King James Version®. Copyright © 1982 " +
			"by Thomas Nelson. Used by permission. All rights reserved. Text provided via API.Bible (api.bible).",
		// epoch 1: cross-reference note capture (include-notes=true; the feed
		// carries no translator footnotes — probed live 2026-08-26).
		cacheEpoch: 1,
		source:     newBYOKLicensedSource("nkjv", nkjvProviderBibleID),
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

// webSource serves the public-domain World English Bible from bible.helloao.org in a
// SINGLE request (decoded by the same helloao path as the BSB — see fetchWEBFromHelloAO
// in bsb.go). It replaced the original bible-api.com per-chapter fetch (~1189 rate-limited
// requests, FetchBibleFromAPI), which often never completed and left first-run readers
// stuck on the embedded Gospels seed.
type webSource struct{}

func (webSource) available() bool            { return true }
func (webSource) fetch() (*BibleData, error) { return fetchWEBFromHelloAO() }

// licensedAPISource is the structured path for a copyrighted translation served
// by a licensed Bible API (e.g. scripture.api.bible). It activates only when
// BOTH are true: we hold a distribution license for the translation, and the
// provider credentials are configured. This double gate makes it impossible to
// ship copyrighted text by accident. Configuration is via environment so no
// secrets live in the repo:
//
//	BIBLE_API_KEY                  provider API key (shared across versions)
//	BIBLETEXT_LICENSE_<ID>=1      explicit "we are licensed for <ID>" opt-in
//	BIBLETEXT_PROVIDER_ID_<ID>    the provider's bible id for this translation
//
// (<ID> is the upper-cased version id, e.g. NKJV.)
type licensedAPISource struct {
	versionID string
	// defaultProviderBibleID makes the translation BYOK-capable: a reader who
	// stores their OWN free API.Bible key in Settings unlocks it with no env
	// configuration — their API.Bible account carries the licence for their
	// use. Blank = env-only (translations whose provider id we have not
	// verified stay operator-gated).
	defaultProviderBibleID string
}

// nkjvProviderBibleID is the NKJV's id in API.Bible's catalogue (HarperCollins,
// "Standard License"), verified live 2026-08-11.
const nkjvProviderBibleID = "63097d2a0a2f7db3-01"

func newLicensedSource(versionID string) *licensedAPISource {
	return &licensedAPISource{versionID: versionID}
}

func newBYOKLicensedSource(versionID, providerBibleID string) *licensedAPISource {
	return &licensedAPISource{versionID: versionID, defaultProviderBibleID: providerBibleID}
}

// byokCapable reports whether v can be unlocked by the reader's own
// API.Bible key (as opposed to operator/env licensing only).
func byokCapable(v BibleVersion) bool {
	s, ok := v.source.(*licensedAPISource)
	return ok && s.defaultProviderBibleID != ""
}

// isLicensedSource reports whether v's text comes from a licensed provider —
// content the app holds under terms, not in perpetuity. Licensed caches get
// the recency/expiry treatment below; public-domain caches never do.
func isLicensedSource(v BibleVersion) bool {
	_, ok := v.source.(*licensedAPISource)
	return ok
}

// licensedRecencyWindow is how long a licensed translation's on-device copy
// may serve before it must be revalidated against the provider. API.Bible's
// Terms (§11 Content Recency Requirements, 3 Aug 2026) require stored content
// to be checked for updates at least every 30 days; the app enforces exactly
// that window rather than treating the cache as permanent the way it does for
// public-domain text.
const licensedRecencyWindow = 30 * 24 * time.Hour

// licensedCacheStale reports whether the cache at path is past the recency
// window. A missing or unreadable cache is NOT stale — it is simply absent,
// and the normal load path handles that.
func licensedCacheStale(path string) bool {
	savedAt, err := cacheSavedAt(path)
	if err != nil {
		return false
	}
	return currentUTCTime().Sub(savedAt) > licensedRecencyWindow
}

// purgeUnavailableLicensedCaches removes on-device copies of licensed
// translations whose licence configuration is GONE — the removal obligation
// that comes with holding content under terms (API.Bible §10: content must be
// removed on termination/deactivation). Public-domain caches are never
// touched. Called once at startup, off the UI goroutine.
func purgeUnavailableLicensedCaches() {
	for _, v := range registeredVersions {
		if !isLicensedSource(v) || v.source.available() {
			continue
		}
		_ = os.Remove(cachePathForVersion(v.ID))
		for _, path := range supersededCachePaths(v) {
			_ = os.Remove(path)
		}
	}
}

// apiKey resolves the provider credential: the operator's environment first
// (development), then the reader's key from Settings, then the compiled Store
// release fallback.
func (s *licensedAPISource) apiKey() string {
	if k := strings.TrimSpace(os.Getenv("BIBLE_API_KEY")); k != "" {
		return k
	}
	return sharedKeys().bibleAPIKey()
}

// licensed reports whether rights to this translation's text are in place:
// the explicit operator env opt-in, or — for default-provider translations —
// an effective API.Bible key from Settings or the compiled Store fallback. A
// key not entitled to the translation fails with the provider's rejection.
func (s *licensedAPISource) licensed() bool {
	if envTruthy(os.Getenv("BIBLETEXT_LICENSE_" + strings.ToUpper(s.versionID))) {
		return true
	}
	return s.defaultProviderBibleID != "" && sharedKeys().bibleAPIKey() != ""
}

// providerVersionID is the licensed provider's id for this translation — the
// env override wins, else the built-in id of a BYOK-capable translation.
func (s *licensedAPISource) providerVersionID() string {
	if id := strings.TrimSpace(os.Getenv("BIBLETEXT_PROVIDER_ID_" + strings.ToUpper(s.versionID))); id != "" {
		return id
	}
	return s.defaultProviderBibleID
}

func (s *licensedAPISource) available() bool {
	return s.apiKey() != "" && s.licensed() && s.providerVersionID() != ""
}

func (s *licensedAPISource) fetch() (*BibleData, error) {
	if !s.available() {
		return nil, fmt.Errorf("version %q: licensed source not configured "+
			"(need a distribution license, BIBLE_API_KEY, BIBLETEXT_LICENSE_%s=1 and BIBLETEXT_PROVIDER_ID_%s)",
			s.versionID, strings.ToUpper(s.versionID), strings.ToUpper(s.versionID))
	}
	return fetchAPIBible(strings.ToUpper(s.versionID), s.providerVersionID(), s.apiKey())
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// testingVersionsEnabled unlocks the not-yet-licensed versions for internal QA,
// making them selectable so the placeholder flow can be exercised end to end.
// It is off by default, so shipped builds never expose placeholder text to users
// (they see the versions as "evaluation in progress", not selectable).
func testingVersionsEnabled() bool {
	return envTruthy(os.Getenv("BIBLETEXT_ENABLE_TESTING"))
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
// loadVersionFromCacheOnly returns a version's data from the on-disk cache WITHOUT any
// network fetch — used for the instant first paint before deciding whether to seed the
// Gospels. It returns an error (never a fetch) on a cache miss.
func loadVersionFromCacheOnly(v BibleVersion) (*BibleData, dataMode, error) {
	if v.source == nil || !v.source.available() {
		return nil, modeTesting, errCacheNotFound
	}
	// A LICENSED cache past its recency window must not be served from the
	// fast path: report a miss so startup takes the full load path, which
	// revalidates (refetches) it. Public-domain versions keep their
	// serve-forever behaviour — this gate exists because API.Bible's terms
	// require it, not because the text goes stale.
	if isLicensedSource(v) && licensedCacheStale(cachePathForVersion(v.ID)) {
		return nil, modeReal, errCacheNotFound
	}
	noFetch := func() (*BibleData, error) { return nil, errCacheNotFound }
	data, _, err := loadBibleData(noFetch, cachePathForVersion(v.ID), currentUTCTime)
	if err == nil {
		return data, modeReal, nil
	}
	// The superseded-epoch fallback below is a stale-serve by design — right
	// for public-domain texts, wrong for licensed ones, which never serve
	// stale copies.
	if isLicensedSource(v) {
		return nil, modeReal, errCacheNotFound
	}
	// EPOCH-MIGRATION FALLBACK (incident-hardening): a cacheEpoch bump moves the
	// cache to a new filename, so the first post-update launch misses here even
	// though a complete, valid old-epoch cache sits on disk. Read the superseded
	// paths rather than pretending the reader has no Bible — an offline upgrader
	// keeps their full canon (old decoder output) and their reading history,
	// and the background refetch upgrades the text when the network returns.
	// This is the data-layer root of the history-erasure incident: the miss
	// used to drop startup to the Gospels seed, whose 4-book canon made every
	// saved non-Gospel position look invalid.
	for _, path := range supersededCachePaths(v) {
		if data, _, err := loadBibleData(noFetch, path, currentUTCTime); err == nil {
			return data, modeReal, nil
		}
	}
	return nil, modeReal, errCacheNotFound
}

func loadVersionData(v BibleVersion, base *BibleData) (*BibleData, dataMode, error) {
	if v.source != nil && v.source.available() {
		// Licensed content past the recency window is revalidated, not served:
		// dropping the stale cache first makes loadBibleData refetch, and a
		// failed refetch is an ERROR rather than a quiet fall-back to the old
		// copy. This is the §11 compliance line the API.Bible enquiry offers —
		// enforce it from day one so a granted licence never inherits a laxer
		// behaviour we then have to walk back.
		if isLicensedSource(v) {
			path := cachePathForVersion(v.ID)
			if licensedCacheStale(path) {
				_ = os.Remove(path)
			}
		}
		data, _, err := loadBibleData(v.source.fetch, cachePathForVersion(v.ID), currentUTCTime)
		if err != nil {
			return nil, modeReal, err
		}
		// Purge pre-epoch cache files only AFTER the current-epoch cache exists
		// (incident-hardening): purging first destroyed the reader's only local
		// copy of the translation and only then discovered the network was down.
		purgeSupersededCaches(v)
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

// cachePathForVersion is the on-disk cache for a version. An unversioned default
// (web) uses the legacy path (honoring BIBLETEXT_CACHE_PATH); other unversioned
// translations live beside it as bibletext-<id>.json. A version with a non-zero
// cacheEpoch gets bibletext-<id>-v<epoch>.json, including the default translation,
// so a stale cache produced by an older decoder is bypassed.
func cachePathForVersion(id string) string {
	base := defaultCachePath()
	v, known := versionByID(id)
	if id == defaultVersionID && (!known || v.cacheEpoch == 0) {
		return base
	}
	name := "bibletext-" + id
	if known && v.cacheEpoch > 0 {
		name += fmt.Sprintf("-v%d", v.cacheEpoch)
	}
	return filepath.Join(filepath.Dir(base), name+".json")
}

// versionCacheIsCurrent reports whether v's CURRENT-epoch cache file exists on
// disk. False right after a cacheEpoch bump, when startup was served by the
// superseded-cache fallback (loadVersionFromCacheOnly) — the caller then
// schedules the background refetch that upgrades the stored text to the
// current decoder (triggerFullDownload). Every other load path goes through
// loadVersionData, which is cache-current-or-fetch and self-heals.
func versionCacheIsCurrent(v BibleVersion) bool {
	_, err := os.Stat(cachePathForVersion(v.ID))
	return err == nil
}

// purgeSupersededCaches best-effort removes cache files written by older
// cacheEpochs of v, so a bumped decoder doesn't strand a stale (multi-MB) cache.
// It only ever targets THIS version's own earlier epochs — never the current
// epoch's file, never another version's — so it cannot drop live data. iOS may
// evict Library/Caches on its own; this just keeps the directory tidy.
func purgeSupersededCaches(v BibleVersion) {
	for _, path := range supersededCachePaths(v) {
		_ = os.Remove(path)
	}
}

// supersededCachePaths lists v's earlier-epoch cache files, NEWEST first (the
// migration fallback prefers the most recent decode; the purge removes all).
// The default translation's epoch-0 file is the legacy defaultCachePath()
// itself — included so an epoch bump can both read it on migration and clean
// it up afterwards (it was previously never purged: ~6 MB stranded per user).
func supersededCachePaths(v BibleVersion) []string {
	if v.cacheEpoch <= 0 {
		return nil
	}
	dir := filepath.Dir(defaultCachePath())
	var paths []string
	for k := v.cacheEpoch - 1; k >= 0; k-- {
		if k == 0 && v.ID == defaultVersionID {
			paths = append(paths, defaultCachePath())
			continue
		}
		name := "bibletext-" + v.ID
		if k > 0 {
			name += fmt.Sprintf("-v%d", k)
		}
		paths = append(paths, filepath.Join(dir, name+".json"))
	}
	return paths
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
	if !ok || !v.canSelect() {
		// Unknown id, or a not-yet-licensed version while internal testing mode is
		// off: refuse the switch so placeholder text is never shown to users. The
		// picker already renders these as non-tappable "evaluation in progress"
		// rows; this is the matching backstop.
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
			fmt.Fprintf(os.Stderr, "BibleText: could not load %s: %v\n", v.Name, err)
			return
		}
		data, mode = d, m
	}

	applyLoadedVersion(state, v, data, mode)
}

// applyLoadedVersion swaps an already-loaded translation into the reader: it
// caches the data in memory, points AppState.Bible at it, records the data mode,
// keeps the open book/chapter valid, persists the choice, and rebuilds the
// window. Shared by switchVersion (synchronous) and the picker's async path
// (switchVersionInteractive), so both apply identically once the data is in hand.
func applyLoadedVersion(state *AppState, v BibleVersion, data *BibleData, mode dataMode) {
	// A new translation's text (and recordings) no longer match what's playing, and
	// a version switch doesn't route through addRecentChapter, so stop here.
	gAudio.stop()
	if state.loadedVersions == nil {
		state.loadedVersions = map[string]*BibleData{}
	}
	state.loadedVersions[v.ID] = data

	state.Bible = data
	state.CurrentVersion = v.ID
	state.currentMode = mode
	// A translation actually loaded, so any remembered "we had to fall back"
	// preference is spent: this is now the reader's translation, whether they
	// picked it or the licensed one finally came back.
	state.preferredVersion = ""

	clampToCurrentVersion(state)
	// One ruler (N7): the highlight's span is numbered in the translation the
	// reader just left — the frame VerseSpan.VersionID records. Renumber it
	// into the new translation through the notes' own anchor machinery, or
	// take it down where the mapping is not clean, so the switch can never
	// leave the wrong verse lit (X11/HL_FRAME). A mark the live note owns is
	// left alone here: applyNoteForCurrentChapter below re-derives the note
	// and re-places or clears its mark from the note itself. A consumed parked
	// link (next block) overwrites the mark with its own fresh span either way.
	renumberMarkForVersion(state, v.ID, data)
	// A shared link that asked for THIS translation was parked waiting for it —
	// apply it now, BEFORE the rebuild below, so the rebuild paints the shared
	// passage directly instead of flashing the old chapter first (the same
	// ordering the startup path uses). A target waiting on some OTHER translation
	// is stale: drop it rather than yank the reader somewhere they no longer
	// asked for.
	consumed := false
	if state.pendingLinkVersion != "" {
		if state.pendingLinkVersion == v.ID {
			state.pendingLinkVersion = ""
			consumePendingLink(state)
			consumed = true
		} else {
			state.pendingLinkVersion = ""
			state.pendingLink = nil
			state.pendingLinkRaw = ""
			state.pendingNoteOpenID = 0 // the Show intent dies with its park
		}
	}
	// Notes are keyed version|book|chapter, so the live mirror
	// (ActiveNote/NoteMinimized/NoteVerseLo) belongs to the translation we just
	// left. Re-derive it for the new one, or the old translation's note keeps
	// rendering over different text — anchored to a verse number that need not
	// even mean the same passage (the Romans 14/16 divergence applyShareTarget
	// documents). The inverse failed too: a note stored under the translation
	// being switched TO stayed invisible until the reader navigated.
	//
	// Skipped when a parked link was just consumed — that path went through
	// addRecentChapter, which has already done this, and the arriving note is
	// the one that should win.
	if !consumed {
		applyNoteForCurrentChapter(state)
	}
	// Remember the chosen translation (and current location) across launches.
	persistReadingPosition(state)
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
