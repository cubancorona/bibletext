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

// crossRef is one related passage (a verse or a verse range within one book).
type crossRef struct {
	Book           string
	Chapter, Verse int
	EndCh, EndV    int    // 0 when it's a single verse
	Votes          int    // TSK agreement count (0 for parallels)
	Parallel       bool   // true = a Gospel-synopsis parallel (parallels.go), not a TSK cross-ref
	Title          string // synopsis pericope title, for parallels (e.g. "The Beatitudes")
}

func (c crossRef) label() string {
	switch {
	case c.EndV == 0 || (c.EndCh == c.Chapter && c.EndV == c.Verse):
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

	idx := make(map[string][]crossRef, 32000)
	sc := bufio.NewScanner(tsv)
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
		fromBook, fromCh, fromV, ok := parseOSISStart(cols[0])
		if !ok {
			continue
		}
		ref, ok := parseOSISTarget(cols[1])
		if !ok {
			continue
		}
		ref.Votes, _ = strconv.Atoi(strings.TrimSpace(cols[2]))
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
// "Book.C.V-Book.C2.V2".
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
		if _, ec, ev, ok2 := parseOSISRef(endStr); ok2 {
			ref.EndCh, ref.EndV = ec, ev
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

// crossRefTargetIn rewrites one dataset reference into the translation on
// screen, and reports whether it can be shown at all.
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
func crossRefTargetIn(versionID string, c crossRef) (crossRef, bool) {
	ch, vs, res := MapVerse(versificationReference, versionID, c.Book, c.Chapter, c.Verse)
	if res == verseMapAbsent || res == verseMapIncommensurable {
		return crossRef{}, false
	}
	c.Chapter, c.Verse = ch, vs
	if c.EndV != 0 {
		endCh := c.EndCh
		if endCh == 0 {
			endCh = c.Chapter
		}
		if ech, ev, r := MapVerse(versificationReference, versionID, c.Book, endCh, c.EndV); r != verseMapAbsent && r != verseMapIncommensurable {
			c.EndCh, c.EndV = ech, ev
			if c.EndCh == c.Chapter {
				c.EndCh = 0
			}
		} else {
			c.EndCh, c.EndV = 0, 0
		}
	}
	return c, true
}

func crossRefsForSelection(state *AppState, text string, span selSpan) []crossRef {
	if state == nil || state.Bible == nil {
		return nil
	}
	verses := selectionVerses(state, text, span)
	shown := map[string]bool{} // label -> already emitted

	// Gospel synopsis parallels first (parallels.go): the same event in the other
	// Gospels, tagged. Embedded, so these appear even when the TSK cross-references
	// failed to load (offline). Kept in synopsis order, not sorted by votes.
	var parallels []crossRef
	vid := state.currentVersion().ID
	for _, v := range verses {
		srcCh, srcV, ok := crossRefSourceRef(vid, v)
		if !ok {
			continue
		}
		for _, c := range gospelParallelsForVerse(v.BookName, srcCh, srcV) {
			name, ok := resolveBookName(state.Bible.Books, c.Book)
			if !ok {
				continue
			}
			c.Book = name
			c, ok = crossRefTargetIn(vid, c)
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
				name, ok := resolveBookName(state.Bible.Books, c.Book)
				if !ok {
					continue
				}
				c.Book = name
				c, ok = crossRefTargetIn(vid, c)
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
		// one and the same drag (verification finding). Delegating to the normalize
		// makes the verbs agree by construction; the raw span survives as the
		// answer only where the normalize declines outright (a selection that
		// is ONLY a verse number, a single partial word), where "the verse the
		// position touches" is the honest reading.
		if _, l, h, _, ok := normalizeShareSelection(state, text, span); ok {
			lo, hi = l, h
		}
		var out []Verse
		for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
			if v.Verse >= lo && v.Verse <= hi {
				out = append(out, v)
			}
		}
		return out
	}
	norm := collapseSpaces(text)
	selProbe := firstRunes(norm, 24)
	var out []Verse
	for _, v := range state.Bible.GetChapter(state.CurrentBook, state.CurrentChapter) {
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
