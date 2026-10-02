package bibletext

// Cross-references: for a selected verse, related passages from the OpenBible.info
// dataset (Treasury of Scripture Knowledge, CC-BY). Same model as the WEB text:
// fetched once, cached locally (the ~2 MB zip), then fully offline. The index is
// built lazily on first use.
//
// THE INDEX IS NUMBERED, NOT TRANSLATION-FREE. This header used to claim the
// index was "translation-independent" because target BOOK NAMES are resolved
// against the loaded Bible. Book names were never the hard part: the dataset is
// keyed by chapter and verse in ONE numbering (the reference, versification.go),
// and translations disagree about verse numbers in a small but real set of
// places. Keying it with whatever numbering happened to be on screen, and
// looking the target up the same way, was wrong on BOTH sides:
//
//   - WEB Catholic's Daniel 3 carries the Song of the Three as 3:24-90, pushing
//     the Hebrew 3:24-30 down to 3:91-97. A row labelled "Daniel 3:25" — "the
//     fourth is like a son of the gods" — previewed and jumped to Azariah's
//     prayer instead, silently.
//   - The Romans doxology sits at 16:25-27 in the BSB and NKJV and at 14:24-26
//     in the WEB and WEB Catholic, so half the readers were told "No
//     cross-references for this selection" on a passage that has many.
//   - Verses some translations omit (Mark 9:44, 11:26, Matthew 17:21 …) drew a
//     row with a blank preview whose tap went nowhere.
//
// So every lookup now goes through the versification tables the notes feature
// already used (MapVerse): the SOURCE verse is mapped into the reference before
// keying, and each TARGET is mapped back out into the translation on screen,
// with anything absent or incommensurable dropped rather than shown blank.
// Where the numbering agrees — which is almost everywhere — both are identities
// and nothing changes.

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	crossRefURL          = "https://a.openbible.info/data/cross-references.zip"
	maxCrossRefsPerVerse = 16
)

// crossRef is one related passage: a verse, or a range of verses.
//
// A range almost always lies within one book. The Treasury has eighteen that
// do not — "Leviticus 27:34-Numbers 1:1", "2 Chronicles 36:22-Ezra 1:3",
// "2 John 1:1-3 John 1:14" — and EndBook names the end's book for those. It is
// empty for every other row, which is how a range within one book is spelled.
type crossRef struct {
	Book           string
	Chapter, Verse int
	EndBook        string // the end's book when it is not Book; "" otherwise
	EndCh, EndV    int    // 0 when it's a single verse; EndCh 0 also means "Chapter"
	Votes          int    // TSK agreement count (0 for parallels)
	Parallel       bool   // true = a Gospel-synopsis parallel (parallels.go), not a TSK cross-ref
	Title          string // synopsis pericope title, for parallels (e.g. "The Beatitudes")
}

// crossBook reports whether the range ends in a different book from the one it
// starts in.
func (c crossRef) crossBook() bool { return c.EndV != 0 && c.EndBook != "" && c.EndBook != c.Book }

func (c crossRef) label() string {
	switch {
	case c.crossBook():
		return fmt.Sprintf("%s %d:%d-%s %d:%d", c.Book, c.Chapter, c.Verse, c.EndBook, c.EndCh, c.EndV)
	case c.EndV == 0 || ((c.EndCh == 0 || c.EndCh == c.Chapter) && c.EndV == c.Verse):
		// A range whose end is its start is one verse, however it was spelled:
		// "Romans 16:25-25" is not a citation anyone writes.
		return fmt.Sprintf("%s %d:%d", c.Book, c.Chapter, c.Verse)
	case c.EndCh == 0 || c.EndCh == c.Chapter:
		return fmt.Sprintf("%s %d:%d-%d", c.Book, c.Chapter, c.Verse, c.EndV)
	default:
		return fmt.Sprintf("%s %d:%d-%d:%d", c.Book, c.Chapter, c.Verse, c.EndCh, c.EndV)
	}
}

var (
	crossRefMu      sync.Mutex
	crossRefIndex   map[string][]crossRef
	crossRefLoaded  bool
	crossRefLoadErr error
)

func crossRefKey(book string, ch, v int) string {
	return book + "|" + strconv.Itoa(ch) + "|" + strconv.Itoa(v)
}

func crossRefCachePath() string {
	base := defaultCachePath()
	return filepath.Join(filepath.Dir(base), "bibletext-crossrefs.zip")
}

// ensureCrossRefs builds the index once (loading the cached zip, or fetching it).
// Safe to call from a background goroutine; returns any load error.
func ensureCrossRefs() error {
	crossRefMu.Lock()
	defer crossRefMu.Unlock()
	if crossRefLoaded {
		return crossRefLoadErr
	}
	crossRefLoaded = true // attempt once; a failure is remembered until restart

	zipBytes, err := readOrFetchCrossRefZip()
	if err != nil {
		crossRefLoadErr = err
		return err
	}
	idx, err := parseCrossRefZip(zipBytes)
	if err != nil {
		crossRefLoadErr = err
		return err
	}
	crossRefIndex = idx
	return nil
}

func readOrFetchCrossRefZip() ([]byte, error) {
	path := crossRefCachePath()
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		return b, nil
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(crossRefURL)
	if err != nil {
		return nil, fmt.Errorf("fetch cross-references: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch cross-references: HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("read cross-references: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err == nil {
		_ = os.Rename(tmp, path) // best-effort cache; ignore failure
	}
	return b, nil
}

func parseCrossRefZip(zipBytes []byte) (map[string][]crossRef, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return nil, fmt.Errorf("open cross-references zip: %w", err)
	}
	var tsv io.ReadCloser
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".txt") {
			tsv, err = f.Open()
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if tsv == nil {
		return nil, fmt.Errorf("cross-references zip has no .txt entry")
	}
	defer tsv.Close()
	return parseCrossRefRows(tsv)
}

// THE DATASET'S OWN NUMBERING. OpenBible's Treasury of Scripture Knowledge is
// not numbered as the KJV is, although its references were compiled against
// it. Measured against every shipped text, its verse SET is the ESV's and the
// BSB's: it reaches the last verse of every chapter the BSB has, it numbers the
// Romans doxology 16:25-27, 3 John runs to verse 15, and not one row starts at
// or points to any of the sixteen Textus Receptus verses (Matthew 17:21, Acts
// 8:37 …). Its CONTENT, though, keeps the KJV's ORDER where two verses stand
// in different orders under the same numbers: its Philippians 1:16 rows are
// about the preachers of selfish ambition and its 1:17 rows about the defence
// of the gospel, as in the KJV and the WEB and the reverse of the BSB.
//
// So no shipped translation's profile describes it, and its differences from
// the reference are written out here, measured from the dataset itself. The
// BSB's profile stood in for it once; it shares the doxology but not 3 John
// 1:15, and it records the BSB's reordered Philippians 1:16-17, which would
// move every one of the dataset's rows there onto the wrong verse.
//
//   - the doxology, 16:25-27 here and 14:24-26 in the reference;
//   - 3 John 1:15, the closing greeting, which every shipped text prints as
//     the end of 1:14. Its one row — the friends greeted "by name", to John
//     10:3 — was keyed to a verse nothing looks up;
//   - Matthew 23:13 as a SOURCE (crossRefDatasetSourceMoves).
var crossRefDatasetMoves = map[verseRef]verseRef{
	{"Romans", 16, 25}: {"Romans", 14, 24},
	{"Romans", 16, 26}: {"Romans", 14, 25},
	{"Romans", 16, 27}: {"Romans", 14, 26},
	{"3 John", 1, 15}:  {"3 John", 1, 14},
}

// crossRefDatasetSourceMoves are the dataset's differences that hold for the
// verse a row comes FROM and not for the verses rows point TO.
//
// The dataset's Matthew 23:13 is the kingdom woe, "you shut up the Kingdom of
// Heaven", as the KJV's and the ESV's 23:13 are: all 23 of its rows fit it,
// led by Luke 11:52 (the key of knowledge taken away). The reference — the WEB
// — has the two woes the other way round, and that woe is its 23:14. As a
// TARGET the dataset's 23:13 also holds the references the Treasury gives the
// KJV's 23:14, the widows' woe, because its verse set has no 23:14 to hold
// them: Mark 12:40, Luke 20:47, Isaiah 10:2 and 1 Timothy 5:3 are among them,
// and they are most of its 70. A row pointing at 23:13 therefore keeps the
// reference's 23:13, the widows' woe, and so does a range that starts there
// ("Matthew 23:13-36", the woes), which begins at the first woe in the WEB's
// order as in the KJV's.
var crossRefDatasetSourceMoves = map[verseRef]verseRef{
	{"Matthew", 23, 13}: {"Matthew", 23, 14},
}

// crossRefDatasetRowSources re-files the rows whose own content contradicts
// the verse the dataset files them under, keyed by the row's two columns.
// There is one: "Do nothing through rivalry or through conceit" (Philippians
// 2:3) is filed under 1:17 in the ESV's order, among rows that follow the
// KJV's, and belongs with the preachers "of selfish ambition" — the dataset's
// and the reference's 1:16.
var crossRefDatasetRowSources = map[[2]string]string{
	{"Phil.1.17", "Phil.2.3"}: "Phil.1.16",
}

// parseCrossRefRows reads the dataset's TSV and returns the index, NORMALISED
// into the reference numbering.
//
// Normalising here rather than at lookup time is what makes the rest of the
// feature correct by construction: crossRefSourceRef maps the reader's verse
// INTO the reference, and crossRefTargetIn maps a stored row OUT of it, so
// both already assume the index speaks reference numbers. It did not — it
// spoke the dataset's — and the doxology, the one passage where the two
// disagree, lost its 92 rows in every translation while rows pointing at it
// rendered a verse number the WEB does not have.
//
// Every verse the dataset names has a counterpart in the reference once the
// moves above are applied, so no row is lost here: a row the reader's
// translation cannot show is dropped later, by crossRefTargetIn, for that
// translation alone.
func parseCrossRefRows(r io.Reader) (map[string][]crossRef, error) {
	idx := make(map[string][]crossRef, 32000)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first { // header row
			first = false
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			continue
		}
		from := cols[0]
		if moved, ok := crossRefDatasetRowSources[[2]string{from, cols[1]}]; ok {
			from = moved
		}
		fromBook, fromCh, fromV, ok := parseOSISStart(from)
		if !ok {
			continue
		}
		ref, ok := parseOSISTarget(cols[1])
		if !ok {
			continue
		}
		ref.Votes, _ = strconv.Atoi(strings.TrimSpace(cols[2]))
		fromCh, fromV = crossRefSourceToReference(fromBook, fromCh, fromV)
		ref = crossRefTargetToReference(ref)
		key := crossRefKey(fromBook, fromCh, fromV)
		idx[key] = append(idx[key], ref)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan cross-references: %w", err)
	}

	// Keep the top-voted few per verse, highest first.
	for key, refs := range idx {
		sort.SliceStable(refs, func(i, j int) bool { return refs[i].Votes > refs[j].Votes })
		if len(refs) > maxCrossRefsPerVerse {
			refs = refs[:maxCrossRefsPerVerse]
		}
		idx[key] = refs
	}
	return idx, nil
}

// parseOSISStart parses the source side ("Gen.1.1"), taking the first verse if a
// range somehow appears.
func parseOSISStart(s string) (book string, ch, v int, ok bool) {
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	return parseOSISRef(s)
}

// parseOSISTarget parses the target side, which may be "Book.C.V" or a range
// "Book.C.V-Book2.C2.V2". The end's book is kept when it differs: eighteen
// ranges run from the end of one book into the next, and reading their end as
// a verse of the START book gave "Leviticus 27:34-1:1" and "2 John 1:1-15".
func parseOSISTarget(s string) (crossRef, bool) {
	startStr, endStr := s, ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		startStr, endStr = s[:i], s[i+1:]
	}
	book, ch, v, ok := parseOSISRef(startStr)
	if !ok {
		return crossRef{}, false
	}
	ref := crossRef{Book: book, Chapter: ch, Verse: v}
	if endStr != "" {
		if eb, ec, ev, ok2 := parseOSISRef(endStr); ok2 {
			ref.EndCh, ref.EndV = ec, ev
			if eb != book {
				ref.EndBook = eb
			}
		}
	}
	return ref, true
}

// parseOSISRef parses "Abbrev.Chapter.Verse" into the app's book name.
func parseOSISRef(s string) (book string, ch, v int, ok bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return "", 0, 0, false
	}
	name, ok := osisBookNames[parts[0]]
	if !ok {
		return "", 0, 0, false
	}
	c, err1 := strconv.Atoi(parts[1])
	vv, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return name, c, vv, true
}

// osisBookNames maps OpenBible's OSIS abbreviations to the app's canonical book
// names (resolved against the loaded translation at lookup time).
var osisBookNames = map[string]string{
	"Gen": "Genesis", "Exod": "Exodus", "Lev": "Leviticus", "Num": "Numbers",
	"Deut": "Deuteronomy", "Josh": "Joshua", "Judg": "Judges", "Ruth": "Ruth",
	"1Sam": "1 Samuel", "2Sam": "2 Samuel", "1Kgs": "1 Kings", "2Kgs": "2 Kings",
	"1Chr": "1 Chronicles", "2Chr": "2 Chronicles", "Ezra": "Ezra", "Neh": "Nehemiah",
	"Esth": "Esther", "Job": "Job", "Ps": "Psalms", "Prov": "Proverbs",
	"Eccl": "Ecclesiastes", "Song": "Song of Solomon", "Isa": "Isaiah", "Jer": "Jeremiah",
	"Lam": "Lamentations", "Ezek": "Ezekiel", "Dan": "Daniel", "Hos": "Hosea",
	"Joel": "Joel", "Amos": "Amos", "Obad": "Obadiah", "Jonah": "Jonah",
	"Mic": "Micah", "Nah": "Nahum", "Hab": "Habakkuk", "Zeph": "Zephaniah",
	"Hag": "Haggai", "Zech": "Zechariah", "Mal": "Malachi", "Matt": "Matthew",
	"Mark": "Mark", "Luke": "Luke", "John": "John", "Acts": "Acts",
	"Rom": "Romans", "1Cor": "1 Corinthians", "2Cor": "2 Corinthians", "Gal": "Galatians",
	"Eph": "Ephesians", "Phil": "Philippians", "Col": "Colossians", "1Thess": "1 Thessalonians",
	"2Thess": "2 Thessalonians", "1Tim": "1 Timothy", "2Tim": "2 Timothy", "Titus": "Titus",
	"Phlm": "Philemon", "Heb": "Hebrews", "Jas": "James", "1Pet": "1 Peter",
	"2Pet": "2 Peter", "1John": "1 John", "2John": "2 John", "3John": "3 John",
	"Jude": "Jude", "Rev": "Revelation",
}

// bookAbbrevByName is the reverse of osisBookNames: canonical book name -> a compact
// OSIS-style abbreviation ("Genesis"->"Gen", "1 Corinthians"->"1Cor",
// "Revelation"->"Rev"). Built once at startup.
var bookAbbrevByName = func() map[string]string {
	m := make(map[string]string, len(osisBookNames))
	for abbr, full := range osisBookNames {
		m[full] = abbr
	}
	return m
}()

// bookAbbrev returns a short label for a canonical book name (for compact UI such
// as the recent-chapters bar), falling back to the full name for anything unknown.
func bookAbbrev(name string) string {
	if a, ok := bookAbbrevByName[name]; ok {
		return a
	}
	return name
}

// crossRefsForSelection aggregates the cross-references for the verse(s) the
// selection spans, resolving target book names against the loaded translation
// and merging duplicates (keeping the highest vote). Highest-voted first.
// crossRefSourceRef maps a verse the reader has selected — numbered in whatever
// translation is on screen — into the numbering the dataset is keyed by.
//
// ok is false when this translation's verse has no counterpart in the reference
// at all: there is nothing to look up, and inventing a neighbouring number would
// answer a question the reader did not ask.
func crossRefSourceRef(versionID string, v Verse) (int, int, bool) {
	ch, vs, res := MapVerse(versionID, versificationReference, v.BookName, v.Chapter, v.Verse)
	if res == verseMapAbsent || res == verseMapIncommensurable {
		return 0, 0, false
	}
	return ch, vs, true
}

// crossRefToReference moves one dataset verse number into the reference
// numbering (crossRefDatasetMoves).
func crossRefToReference(book string, ch, v int) (int, int) {
	if to, ok := crossRefDatasetMoves[verseRef{book, ch, v}]; ok {
		return to.Chapter, to.Verse
	}
	return ch, v
}

// crossRefSourceToReference does the same for the verse a row comes from.
func crossRefSourceToReference(book string, ch, v int) (int, int) {
	if to, ok := crossRefDatasetSourceMoves[verseRef{book, ch, v}]; ok {
		return to.Chapter, to.Verse
	}
	return crossRefToReference(book, ch, v)
}

// crossRefTargetToReference does the same for a target, span end included,
// each end in its own book and its own chapter.
func crossRefTargetToReference(c crossRef) crossRef {
	startCh := c.Chapter
	c.Chapter, c.Verse = crossRefToReference(c.Book, c.Chapter, c.Verse)
	if c.EndV == 0 {
		return c
	}
	endBook, endCh := c.Book, c.EndCh
	if c.EndBook != "" {
		endBook = c.EndBook
	}
	if endCh == 0 {
		endCh = startCh // the END's chapter in the DATASET's numbering
	}
	c.EndCh, c.EndV = crossRefToReference(endBook, endCh, c.EndV)
	return normaliseSpanEnd(c)
}

// normaliseSpanEnd puts a range's end in its canonical form: a same-chapter
// end carries EndCh 0, an end that is the start makes the row one verse, and an
// end BEFORE the start — which no citation means — keeps only the start.
func normaliseSpanEnd(c crossRef) crossRef {
	if c.EndV == 0 {
		c.EndBook, c.EndCh = "", 0
		return c
	}
	if c.crossBook() {
		return c
	}
	c.EndBook = ""
	if c.EndCh == 0 {
		c.EndCh = c.Chapter
	}
	switch {
	case c.EndCh < c.Chapter || (c.EndCh == c.Chapter && c.EndV <= c.Verse):
		c.EndCh, c.EndV = 0, 0
	case c.EndCh == c.Chapter:
		c.EndCh = 0
	}
	return c
}

// crossRefTargetIn rewrites one reference — numbered as the reference
// translation numbers it — into the translation on screen, and reports
// whether it can be shown at all.
//
// This is the half that was producing WRONG TEXT rather than merely missing
// text: the panel previews the target with GetVerse and the row's tap navigates
// there, so an unmapped number in WEB Catholic's Daniel 3 previewed and jumped
// to a different passage under the right-looking label. A row that cannot be
// mapped is dropped — the panel would otherwise render it with a blank preview
// and a tap that goes nowhere.
//
// The END of a span is mapped too, and independently: a span may begin in a
// verse that exists and run past one that does not. When the end cannot be
// mapped the row keeps its start and becomes a single-verse reference, which is
// honest — it points at scripture the reader can actually see.
//
// The end is read in the REFERENCE's chapter. By the time it is mapped the
// start's chapter has been rewritten into the translation's, and taking the
// end's chapter from that put it in the wrong chapter wherever the start moved
// chapter: the doxology's "Romans 14:24-25" became "16:25-25" in the BSB and
// the NKJV instead of 16:25-26, and its 14:24-26 became 16:25-26.
//
// Within one book the span is the smallest that holds every verse the
// reference's span names that this translation has: its ends and any verse
// inside it the table moves. Two verses can stand in the opposite order under
// the same numbers — the BSB's Philippians 1:16-17, the NKJV's Matthew
// 23:13-14 — and mapping the ends alone turned "1:16-17" into "1:17-16" and
// left the moved verse's own text out of a span such as 1:12-17.
func crossRefTargetIn(versionID string, c crossRef) (crossRef, bool) {
	ch, vs, res := MapVerse(versificationReference, versionID, c.Book, c.Chapter, c.Verse)
	if res == verseMapAbsent && c.EndV != 0 && c.EndBook == "" {
		// A range whose FIRST verse this translation lacks begins at the next
		// verse of the range it has. The BSB lacks the WEB's Matthew 23:13 —
		// the widows' woe — and prints the woe after it as its own 23:13, so
		// "Matthew 23:13-36", the woes, is still the BSB's 23:13-36; dropping
		// the row would lose a passage the reader's text holds in full. Absent
		// verses are listed one by one in the table, so the walk is short.
		for steps := len(versificationDeltas[versionID].absent); res == verseMapAbsent && steps >= 0; steps-- {
			c.Verse++
			if (c.EndCh == 0 || c.EndCh == c.Chapter) && c.Verse > c.EndV {
				break
			}
			ch, vs, res = MapVerse(versificationReference, versionID, c.Book, c.Chapter, c.Verse)
		}
	}
	if res == verseMapAbsent || res == verseMapIncommensurable {
		return crossRef{}, false
	}
	out := c
	out.Chapter, out.Verse = ch, vs
	out.EndBook, out.EndCh, out.EndV = "", 0, 0
	if c.EndV == 0 {
		return out, true
	}
	endBook, endCh := c.Book, c.EndCh
	if c.EndBook != "" {
		endBook = c.EndBook // a range that runs on into the next book
	}
	if endCh == 0 {
		endCh = c.Chapter // the reference's chapter, not the rewritten one
	}
	ech, ev, r := MapVerse(versificationReference, versionID, endBook, endCh, c.EndV)
	if r == verseMapAbsent || r == verseMapIncommensurable {
		return out, true
	}
	if endBook != c.Book {
		out.EndBook, out.EndCh, out.EndV = endBook, ech, ev
		return out, true
	}
	lo, hi := verseRef{c.Book, ch, vs}, verseRef{c.Book, ech, ev}
	if verseBefore(hi, lo) {
		lo, hi = hi, lo
	}
	start, end := verseRef{c.Book, c.Chapter, c.Verse}, verseRef{c.Book, endCh, c.EndV}
	for _, m := range versificationDeltas[versionID].moved {
		at := verseRef{m.Book, m.Chapter, m.Verse}
		if m.Book != c.Book || verseBefore(at, start) || verseBefore(end, at) {
			continue
		}
		to := verseRef{m.Book, m.ToChapter, m.ToVerse}
		if verseBefore(to, lo) {
			lo = to
		}
		if verseBefore(hi, to) {
			hi = to
		}
	}
	out.Chapter, out.Verse = lo.Chapter, lo.Verse
	out.EndCh, out.EndV = hi.Chapter, hi.Verse
	return normaliseSpanEnd(out), true
}

// verseBefore orders two verses of one book.
func verseBefore(a, b verseRef) bool {
	return a.Chapter < b.Chapter || (a.Chapter == b.Chapter && a.Verse < b.Verse)
}

func crossRefsForSelection(state *AppState, text string, span selSpan) []crossRef {
	if state == nil || state.Bible == nil {
		return nil
	}
	verses := selectionVerses(state, text, span)
	shown := map[string]bool{} // label -> already emitted

	// resolve names a row's book — and a cross-book range's end book — as the
	// loaded translation does, and rewrites the row into its numbering.
	vid := state.currentVersion().ID
	resolve := func(c crossRef) (crossRef, bool) {
		name, ok := resolveBookName(state.Bible.Books, c.Book)
		if !ok {
			return crossRef{}, false
		}
		c.Book = name
		if c.EndBook != "" {
			if c.EndBook, ok = resolveBookName(state.Bible.Books, c.EndBook); !ok {
				return crossRef{}, false
			}
		}
		return crossRefTargetIn(vid, c)
	}

	// Gospel synopsis parallels first (parallels.go): the same event in the other
	// Gospels, tagged. Embedded, so these appear even when the TSK cross-references
	// failed to load (offline). Kept in synopsis order, not sorted by votes.
	var parallels []crossRef
	for _, v := range verses {
		srcCh, srcV, ok := crossRefSourceRef(vid, v)
		if !ok {
			continue
		}
		for _, c := range gospelParallelsForVerse(v.BookName, srcCh, srcV) {
			c, ok := resolve(c)
			if !ok {
				continue
			}
			lbl := c.label()
			if shown[lbl] {
				continue
			}
			shown[lbl] = true
			parallels = append(parallels, c)
		}
	}

	// Treasury-of-Scripture-Knowledge cross-references, highest-voted first, minus
	// anything already shown as a parallel.
	var tsk []crossRef
	if crossRefIndex != nil {
		seen := map[string]int{} // label -> index into tsk
		for _, v := range verses {
			srcCh, srcV, ok := crossRefSourceRef(vid, v)
			if !ok {
				continue
			}
			for _, c := range crossRefIndex[crossRefKey(v.BookName, srcCh, srcV)] {
				c, ok := resolve(c)
				if !ok {
					continue
				}
				lbl := c.label()
				if shown[lbl] {
					continue
				}
				if i, dup := seen[lbl]; dup {
					if c.Votes > tsk[i].Votes {
						tsk[i].Votes = c.Votes
					}
					continue
				}
				seen[lbl] = len(tsk)
				tsk = append(tsk, c)
			}
		}
		sort.SliceStable(tsk, func(i, j int) bool { return tsk[i].Votes > tsk[j].Votes })
		if len(tsk) > 40 {
			tsk = tsk[:40]
		}
	}

	return append(parallels, tsk...)
}

// selectionVerses returns the verses of the current chapter that the selection
// overlaps. A valid span is answered from POSITION alone — exactly the
// chapter's verses lo..hi, clamped to the verses that exist — because the
// matching below cannot be trusted with scripture's repetitions: Psalm 136's
// refrain opens the same way in all 26 verses, so a probe match cites whichever
// verse compares first, and the 8-rune floor resolves short selections to
// nothing at all. The matching path survives only as the fallback for
// selections that arrive without a position (legacy Entry pane, zero span).
func selectionVerses(state *AppState, text string, span selSpan) []Verse {
	book, chapter := readerChapter(state)
	return selectionVersesIn(state, book, chapter, text, span)
}

// selectionVersesIn is selectionVerses read against a named chapter rather
// than the reader's — the share pipeline's passage route (shareQuoteForPassage)
// runs its legacy citation fallback through here.
func selectionVersesIn(state *AppState, book string, chapter int, text string, span selSpan) []Verse {
	if state.Bible == nil {
		return nil
	}
	if span.valid() {
		lo, hi := span.lo, span.hi
		// ONE RESOLVER FOR EVERY VERB. The share pipeline attributes the same
		// selection positionally through normalizeShareSelection, whose trims
		// drop a verse that contributes no words (a selection swept just past a
		// verse's NUMBER spans lo..N natively, but no word of N is quoted). A
		// span answered verbatim here made the crossref panel cite — and pull
		// references for — a verse the share card rightly refused to name, for
		// one and the same drag. Delegating to the normalize
		// makes the verbs agree by construction; the raw span survives as the
		// answer only where the normalize declines outright (a single partial
		// word), where "the verse the position touches" is the honest reading.
		// A selection that is ONLY a verse number no longer declines: it
		// resolves to the verse the number labels.
		if _, l, h, _, ok := normalizeShareSelectionIn(state, book, chapter, text, span); ok {
			lo, hi = l, h
		}
		var out []Verse
		for _, v := range state.Bible.GetChapter(book, chapter) {
			if v.Verse >= lo && v.Verse <= hi {
				out = append(out, v)
			}
		}
		return out
	}
	norm := collapseSpaces(text)
	selProbe := firstRunes(norm, 24)
	var out []Verse
	for _, v := range state.Bible.GetChapter(book, chapter) {
		vt := collapseSpaces(v.Text)
		vProbe := firstRunes(vt, 24)
		if (len([]rune(vProbe)) >= 8 && strings.Contains(norm, vProbe)) ||
			(len([]rune(selProbe)) >= 8 && strings.Contains(vt, selProbe)) {
			out = append(out, v)
		}
	}
	return out
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
