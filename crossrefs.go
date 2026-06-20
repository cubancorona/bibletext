package bibletext

// Cross-references: for a selected verse, related passages from the OpenBible.info
// dataset (Treasury of Scripture Knowledge, CC-BY). Same model as the WEB text:
// fetched once, cached locally (the ~2 MB zip), then fully offline. The index is
// built lazily on first use and is translation-independent — target book names
// are resolved against the loaded Bible at lookup time.

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
	EndCh, EndV    int // 0 when it's a single verse
	Votes          int
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
func crossRefsForSelection(state *AppState, text string) []crossRef {
	if crossRefIndex == nil {
		return nil
	}
	seen := map[string]int{} // label -> index into out
	var out []crossRef
	for _, v := range selectionVerses(state, text) {
		for _, c := range crossRefIndex[crossRefKey(v.BookName, v.Chapter, v.Verse)] {
			name, ok := resolveBookName(state.Bible.Books, c.Book)
			if !ok {
				continue
			}
			c.Book = name
			lbl := c.label()
			if i, dup := seen[lbl]; dup {
				if c.Votes > out[i].Votes {
					out[i].Votes = c.Votes
				}
				continue
			}
			seen[lbl] = len(out)
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Votes > out[j].Votes })
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// selectionVerses returns the verses of the current chapter that the selection
// overlaps (matching in either direction so partial selections still resolve).
func selectionVerses(state *AppState, text string) []Verse {
	if state.Bible == nil {
		return nil
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
