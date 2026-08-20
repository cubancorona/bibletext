package bibletext

// Where notes live once they belong to the reader — the scrapbook store
// (S5, [redacted-retired-private-reference]).
//
// ONE STORE, LINE-FRAMED, APPEND-FRIENDLY. The value under notes.store is one
// JSON object per line (JSONL). Per-record framing is spec rule 2 of the
// long-term foundation: one corrupt line costs ONE record, not the scrapbook.
// Unparseable lines are QUARANTINED VERBATIM — kept in memory, re-emitted
// byte-for-byte on every write, never deleted, never counted as notes. The app
// never deletes what it cannot parse. (The store this replaced filtered junk
// on read and the next unrelated write made that permanent.)
//
// NO KEY, NO CAP, NO EVICTION. The old store was a map keyed
// version|book|chapter — so a second note on a passage silently destroyed the
// first — and it evicted past 200 entries by ALPHABETICAL ORDER of the storage
// key, which discarded every "bsb|…" note before any "web|…" one and could
// evict an arriving note by the very write that stored it (X3). Eviction is a
// data-loss event; this store keeps what it is given. (The browser's capacity
// notice is S11.)
//
// WRITERS STAND DOWN when the store is unreadable AS A WHOLE (a value that is
// present but yields nothing recognisable): every mutation is a
// read-modify-write, so a failed read that answered "no notes" would serialise
// emptiness over the reader's collection and their next action would be the
// thing that destroyed it. Per-line damage is quarantined, not a stand-down.
//
// AN EMPTY VALUE MEANS "NEW READER", never "wiped": deleteAllNotes
// (notes_setting.go) writes the one-line header sentinel {"wiped":true}
// instead of "", so a deliberate wipe is distinguishable from a value-level
// loss. (What that sentinel cannot catch is whole-file truncation — the
// sentinel lives in the same preferences.json — but shipped builds apply
// patches/fyne-2.7.4-atomic-prefs.patch, so a torn preferences write cannot
// publish a truncated file in a shipped build.)

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// prefNotesStore is the JSONL scrapbook store.
	prefNotesStore = "notes.store"
	// prefNotesNextID is the ID counter: the LAST allocated id, decimal. Its
	// own key, deliberately: deletion never touches it, so an ID is never
	// reused however much of the store is deleted.
	prefNotesNextID = "notes.store.nextid"

	// The pre-S5 stores, read once by migration and then cleared.
	prefLegacySharedNotes = "shared.notes"
	prefLegacyMyNotes     = "notes.mine"

	// notesWipedSentinel is line 1 of a deliberately emptied store.
	notesWipedSentinel = `{"wiped":true}`
)

// noteStore is the parsed store: every record, every quarantined line, and
// whether the value as a whole could be read.
type noteStore struct {
	notes      []StoredNote // ID ascending — arrival order, byte-stable writes
	quarantine []string     // lines that did not parse, verbatim, re-emitted at the tail
	wiped      bool         // the header sentinel was present
	ok         bool         // false: value present but nothing recognisable — writers stand down
}

// readNoteStoreRaw parses the store value without running migration.
func readNoteStoreRaw(p prefStore) *noteStore {
	s := &noteStore{ok: true}
	if p == nil {
		return s // no app: nothing to read, nothing to overwrite
	}
	raw := p.String(prefNotesStore)
	if raw == "" {
		return s // a new reader — genuinely empty, safe to write to
	}
	recognised := false
	for lineNo, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			s.quarantine = appendQuarantine(s.quarantine, line)
			continue
		}
		// The wiped sentinel is recognised ONLY as the FIRST line, and only
		// when the line carries no record id. "Any line with a wiped key" was
		// the first rule here, and it let a FUTURE build's record-level field
		// named "wiped" destroy the record it rode on and falsely mark the
		// whole store wiped — the reviewer spliced "wiped":false into a live
		// record and watched it vanish without reaching quarantine. A field
		// name must never be a trapdoor; position and shape make the sentinel
		// unforgeable by a record.
		_, hasWiped := probe["wiped"]
		_, hasID := probe["id"]
		if lineNo == 0 && hasWiped && !hasID {
			s.wiped = true
			recognised = true
			continue
		}
		var n StoredNote
		if err := json.Unmarshal([]byte(line), &n); err != nil || n.ID == 0 {
			// Parseable JSON that is not a record this build can address.
			// Quarantined, not judged: a future build may know what it is.
			s.quarantine = appendQuarantine(s.quarantine, line)
			continue
		}
		s.notes = append(s.notes, n)
		recognised = true
	}
	if !recognised {
		// A present value in which nothing was recognisable at all. This is
		// readNotesChecked's old ok=false contract, preserved and for the same
		// reason: never overwrite what you cannot read.
		s.ok = false
		return s
	}
	sort.SliceStable(s.notes, func(i, j int) bool { return s.notes[i].ID < s.notes[j].ID })
	return s
}

// noteStoreCache is the parsed store, valid while the raw preferences value is
// byte-identical to cacheRaw.
//
// The store is read on EVERY navigation (applyNoteForCurrentChapter), and
// parsing is three json.Unmarshal passes per record — measured at 3.9ms for
// 500 notes and 15ms for 2,000, per navigation, against ~0.3ms for the old
// map. That is the iOS scroll budget spent on re-parsing bytes that have not
// changed. The raw string is the cache key because it is the whole truth: only
// this process writes the value, and even an external change (a synced
// preferences file) changes the bytes and misses the cache. The UI is
// single-goroutine (fyne.Do), matching every other package-level cache here.
var (
	noteStoreCacheRaw string
	noteStoreCache    *noteStore
)

// readNoteStore reads the store and, when the pre-S5 blobs still hold
// anything, folds them in first (once — the legacy keys are cleared after a
// verified write). Every reader and every writer goes through here.
func readNoteStore(p prefStore) *noteStore {
	if p != nil {
		if raw := p.String(prefNotesStore); raw != "" && raw == noteStoreCacheRaw && noteStoreCache != nil {
			// Migration is not consulted on a cache hit on purpose: a hit means
			// the store bytes exist and are unchanged, and the legacy fold-in
			// happened on the miss that populated the cache.
			return noteStoreCache
		}
	}
	s := readNoteStoreRaw(p)
	migrateLegacyNotes(p, s)
	if p != nil && s.ok {
		noteStoreCacheRaw = p.String(prefNotesStore)
		noteStoreCache = s
	}
	return s
}

// serializeNoteStore renders the store to the exact bytes writeNoteStore
// stores: header sentinel if wiped, then records by ID ascending, then the
// quarantine verbatim. No map is ranged anywhere, so an unchanged store
// serialises to identical bytes and Fyne's set() short-circuits with no file
// write at all.
func serializeNoteStore(s *noteStore) string {
	var lines []string
	if s.wiped {
		lines = append(lines, notesWipedSentinel)
	}
	for _, n := range s.notes {
		b, err := json.Marshal(n)
		if err != nil {
			continue // unencodable record: skip the line, never the store
		}
		lines = append(lines, string(b))
	}
	lines = append(lines, s.quarantine...)
	return strings.Join(lines, "\n")
}

func writeNoteStore(p prefStore, s *noteStore) {
	if p == nil || !s.ok {
		return
	}
	raw := serializeNoteStore(s)
	p.SetString(prefNotesStore, raw)
	// The writer's own result primes the cache: the next navigation reads what
	// was just written without re-parsing it.
	noteStoreCacheRaw = raw
	noteStoreCache = s
}

// nextNoteID allocates a fresh, never-reused id and persists the counter in
// the same mutation. The counter is the authority — NOT max(existing)+1,
// because deleting the newest note must not free its id. The max() below is
// recovery only: if the counter key itself is lost or damaged, restarting at 1
// would re-issue ids that live notes still hold, so the store's own high-water
// mark floors it. (Ids of notes deleted before such a loss can no longer be
// protected; that is the honest limit of a counter that lives in preferences.)

// appendQuarantine keeps a line the store cannot parse, EXACTLY ONCE.
//
// Byte-identical quarantine entries are collapsed because the two paths that
// feed this both replay: a crash inside migrateLegacyNotes' write-then-clear
// window re-quarantines the same corrupt legacy blob on the next launch, and a
// backup restore reintroduces the legacy key wholesale. Without the dedup each
// replay grew the store by another copy of the same unparseable bytes,
// forever. Deduping by bytes is safe precisely because quarantine is defined
// as byte-preservation: two identical lines carry identical information.
func appendQuarantine(q []string, line string) []string {
	for _, have := range q {
		if have == line {
			return q
		}
	}
	return append(q, line)
}

func nextNoteID(p prefStore, s *noteStore) uint64 {
	var last uint64
	if p != nil {
		if v, err := strconv.ParseUint(strings.TrimSpace(p.String(prefNotesNextID)), 10, 64); err == nil {
			last = v
		}
	}
	for _, n := range s.notes {
		if n.ID > last {
			last = n.ID
		}
	}
	// Quarantined lines can carry ids too — a record damaged in one FIELD is
	// quarantined whole, identity and all, and a future build may repair and
	// re-admit it. Recovery that ignored them could re-mint a quarantined id
	// for a brand-new note, and an id that means two notes is the one thing an
	// identity must never do. Best-effort by design: a line whose very id is
	// unreadable cannot collide with anything, because re-admission would fail
	// on it too.
	for _, line := range s.quarantine {
		var probe struct {
			ID uint64 `json:"id"`
		}
		if json.Unmarshal([]byte(line), &probe) == nil && probe.ID > last {
			last = probe.ID
		}
	}
	last++
	if p != nil {
		p.SetString(prefNotesNextID, strconv.FormatUint(last, 10))
	}
	return last
}

// noteNow is the clock, indirected so tests can pin it.
var noteNow = func() int64 { return time.Now().Unix() }

// addNote appends a note to the store and returns the stored record — which
// is the EXISTING record when the same note is already there.
//
// DEDUP is sameNoteContent (the content tuple, never payload bytes) plus the
// kind: same tuple + same Kind = same note. On a duplicate the stored record
// is preserved untouched — Received and Minimized are the reader's history
// with the note, and a re-opened link must not rewrite it.
//
// ok=false means the note was NOT stored: the store is unreadable and every
// writer stands down (see the file header).
func addNote(p prefStore, n StoredNote) (StoredNote, bool) {
	if strings.TrimSpace(n.Text) == "" || n.Book == "" || n.Chapter < 1 {
		return StoredNote{}, false
	}
	if n.Kind == "" {
		n.Kind = noteKindReceived
	}
	s := readNoteStore(p)
	if !s.ok {
		return StoredNote{}, false
	}
	for i, e := range s.notes {
		if e.Kind == n.Kind && sameNoteContent(e, n) {
			// The same note, re-opened — but this arrival's payload may carry
			// unknown wire records the stored copy lacks (an older link opened
			// first, a newer re-share second). Spec rule 3 preserves unknown
			// records; discarding the fresher ones on a dedup hit quietly
			// destroyed them. First-wins where BOTH have leftovers, adopt
			// where the stored copy has none.
			changed := false
			if len(e.WireSkipped) == 0 && len(n.WireSkipped) > 0 {
				s.notes[i].WireSkipped = n.WireSkipped
				changed = true
			}
			if len(e.WireOpaque) == 0 && len(n.WireOpaque) > 0 {
				s.notes[i].WireOpaque = n.WireOpaque
				changed = true
			}
			if len(e.AnchorRuns) == 0 && len(n.AnchorRuns) > 0 {
				// Same rule for the run set: a copy stored by a build that read
				// only the first run gains the full anchor when the same link
				// is re-opened by this one.
				s.notes[i].AnchorRuns = n.AnchorRuns
				changed = true
			}
			if changed {
				writeNoteStore(p, s)
			}
			return s.notes[i], true
		}
	}
	if n.Received == 0 {
		n.Received = noteNow()
	}
	n.ID = nextNoteID(p, s)
	s.notes = append(s.notes, n)
	writeNoteStore(p, s)
	return n, true
}

// deleteNoteByID removes one record for good. The id is handed to the caller
// by the surface that drew the note — never rebuilt from where the reader is
// standing, which is what let Delete miss a cross-chapter note (X13) and made
// Hide and Show address different objects (X5).
func deleteNoteByID(p prefStore, id uint64) {
	if id == 0 {
		return
	}
	s := readNoteStore(p)
	if !s.ok {
		return
	}
	for i, n := range s.notes {
		if n.ID == id {
			s.notes = append(s.notes[:i], s.notes[i+1:]...)
			writeNoteStore(p, s)
			return
		}
	}
}

// setNoteMinimizedByID collapses or restores one record.
func setNoteMinimizedByID(p prefStore, id uint64, min bool) {
	if id == 0 {
		return
	}
	s := readNoteStore(p)
	if !s.ok {
		return
	}
	for i := range s.notes {
		if s.notes[i].ID == id {
			if s.notes[i].Minimized == min {
				return
			}
			s.notes[i].Minimized = min
			writeNoteStore(p, s)
			return
		}
	}
}

// displayableNote reports whether a record is a note this build can show at
// all. Records that fail this are still carried through every rewrite — they
// may be a future build's — they just are not drawn or counted.
func displayableNote(n StoredNote) bool {
	if n.Kind != noteKindReceived && n.Kind != noteKindMine {
		return false
	}
	return n.Book != "" && n.Chapter >= 1 && strings.TrimSpace(n.Text) != ""
}

// allNotesForBrowsing returns every note the app holds — received and sent —
// in arrival order, for the notes list. One list, mixed, because a scrapbook
// records an EXCHANGE; the browser owns the ordering (sortedNotes).
func allNotesForBrowsing(p prefStore) []StoredNote {
	var out []StoredNote
	for _, n := range readNoteStore(p).notes {
		if displayableNote(n) {
			out = append(out, n)
		}
	}
	return out
}

// storedNoteCount is how many notes "Delete all notes" would delete — both
// kinds, because it is one store and one control.
func storedNoteCount(p prefStore) int {
	return len(allNotesForBrowsing(p))
}

// readMyNotes returns the notes the reader has sent, oldest first, and whether
// the store could be read at all — Kind=mine reads of the one store.
func readMyNotes(p prefStore) ([]StoredNote, bool) {
	s := readNoteStore(p)
	if !s.ok {
		return nil, false
	}
	var out []StoredNote
	for _, n := range s.notes {
		if n.Kind == noteKindMine && displayableNote(n) {
			out = append(out, n)
		}
	}
	return out, true
}

// isOwnLiveNote reports whether the note the verbs would act on is one the
// reader WROTE, rather than one they were sent. The mirror carries the
// identity; the store says whose it is.
func isOwnLiveNote(state *AppState) bool {
	if state == nil || state.NoteID == 0 {
		return false
	}
	n, ok := findNoteByID(appPrefs(), state.NoteID)
	return ok && n.Kind == noteKindMine
}

// findNoteByID answers "whose note is this, really" for a note the mirror is
// carrying but the chapter plan does not hold — the clamped-chapter arrival.
func findNoteByID(p prefStore, id uint64) (StoredNote, bool) {
	if id == 0 {
		return StoredNote{}, false
	}
	s := readNoteStore(p)
	if !s.ok {
		return StoredNote{}, false
	}
	for _, n := range s.notes {
		if n.ID == id {
			return n, true
		}
	}
	return StoredNote{}, false
}

// findNoteByNonce looks for a note YOU shared, by the identity minted when you
// shared it. Kind=mine only: a received note's nonce is the SENDER's, and two
// readers who were both sent the same link legitimately hold the same value —
// folding on that would merge one friend's note into another's.
func findNoteByNonce(p prefStore, nonce []byte) (StoredNote, bool) {
	if len(nonce) != noteNonceLen {
		return StoredNote{}, false
	}
	s := readNoteStore(p)
	if !s.ok {
		return StoredNote{}, false
	}
	for _, n := range s.notes {
		if n.Kind == noteKindMine && bytes.Equal(n.Nonce, nonce) {
			return n, true
		}
	}
	return StoredNote{}, false
}

// saveMyNote appends a note the reader just sent — a Kind=mine write of the
// one store. Two of your own notes on one passage are two notes; the same
// words re-shared are one (sameNoteContent, owner).
func saveMyNote(p prefStore, n StoredNote) {
	n.Kind = noteKindMine
	// Store what actually TRAVELLED, not what was typed. The wire runs
	// normalizeNote on encode (share_note.go) — it collapses blank-line runs and
	// truncates at the rune cap, and the compose sheet deliberately lets a
	// reader write past that cap and tells them it will be shortened. Storing
	// the raw string left the sent record and the shared link holding different
	// text for the same note, so the two could never be recognised as one.
	n.Text = normalizeNote(n.Text)
	// And the store's single spelling for a one-verse note (singleVerseHi).
	if n.VerseHi <= n.VerseLo {
		n.VerseHi = 0
	}
	addNote(p, n)
}

// --- migration from the pre-S5 stores ---------------------------------------

// legacySharedNote is the old SharedNote wire shape, kept only so migration
// can read the two pre-S5 blobs.
type legacySharedNote struct {
	VersionID  string `json:"v"`
	Book       string `json:"b"`
	Chapter    int    `json:"c"`
	VerseLo    int    `json:"lo"`
	VerseHi    int    `json:"hi"`
	Text       string `json:"t"`
	Minimized  bool   `json:"m"`
	Received   int64  `json:"ts"`
	Mine       bool   `json:"me"`
	SenderName string `json:"sn"`
	SenderID   string `json:"sid"`
}

// migrateLegacyNotes folds the old shared.notes map and notes.mine list into

// only migration and a failed one is accepted): a legacy blob that will not
// parse is quarantined into the new store VERBATIM rather than dropped, and
// the old keys are cleared ONLY after the new store's write is verified by
// reading it back.
func migrateLegacyNotes(p prefStore, s *noteStore) {
	if p == nil || !s.ok {
		return
	}
	rawShared := p.String(prefLegacySharedNotes)
	rawMine := p.String(prefLegacyMyNotes)
	if strings.TrimSpace(rawShared) == "" && strings.TrimSpace(rawMine) == "" {
		return
	}

	convert := func(raw string, kind NoteKind) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		var list []legacySharedNote
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			// Unreadable old blob: its raw bytes go to quarantine, where every
			// rewrite carries them, rather than being dropped.
			s.quarantine = appendQuarantine(s.quarantine, raw)
			return
		}
		// Oldest first, so ID order matches arrival order.
		sort.SliceStable(list, func(i, j int) bool { return list[i].Received < list[j].Received })
		for _, l := range list {
			k := kind
			if l.Mine {
				k = noteKindMine
			}
			n := StoredNote{
				Kind: k, VersionID: l.VersionID, Book: l.Book, Chapter: l.Chapter,
				VerseLo: l.VerseLo, VerseHi: l.VerseHi, Text: l.Text,
				Minimized: l.Minimized, Received: l.Received,
				SenderName: l.SenderName, SenderID: l.SenderID,
			}
			if l.Book == "" || l.Chapter < 1 || strings.TrimSpace(l.Text) == "" {
				// Parseable but not a note this build recognises. Quarantine its
				// own re-encoding rather than judging it away.
				if b, err := json.Marshal(l); err == nil {
					s.quarantine = appendQuarantine(s.quarantine, string(b))
				}
				continue
			}
			dup := false
			for _, e := range s.notes {
				if e.Kind == n.Kind && sameNoteContent(e, n) {
					dup = true
					break
				}
			}
			if dup {
				continue // a restored backup re-importing what already migrated
			}
			n.ID = nextNoteID(p, s)
			s.notes = append(s.notes, n)
		}
	}
	convert(rawShared, noteKindReceived)
	convert(rawMine, noteKindMine)

	// The old keys are cleared ONLY after a successful write of the new store,
	// verified by reading the value back. If the write did not land (a fake
	// store in tests, or anything stranger), the legacy blobs stay put and
	// migration simply runs again next read.
	writeNoteStore(p, s)
	if p.String(prefNotesStore) == serializeNoteStore(s) {
		p.SetString(prefLegacySharedNotes, "")
		p.SetString(prefLegacyMyNotes, "")
	}
}

// --- the display derive: what the reading pane shows ------------------------

// noteForChapter picks the ONE note the arity-1 display draws for a passage.
//
// SINCE S7 THIS IS THE DEFAULT SELECTION RULE ONLY: the derive is
// buildChapterPlan (notes_plan.go), which answers with the whole SET and calls
// this for the shipped display default so S7 renders byte-identically to
// before it. Its selection — native first, newest first, a collapsed native
// note masking an expanded followed one — is the enumerated X7/COLLAPSED_MASK
// debt, kept deliberately until S8 draws the set; S8 deletes this function.
//
// Placement comes from resolveNoteAnchor (notes_anchor.go), not from an inline
// MapVerse probe: the anchor resolves to a SET of runs plus a REASON, so a
// note is shown exactly when a resolved run lands on this chapter, and the
// unplaced arms (no such book here, incommensurable numbering, verses absent)
// decline honestly instead of trusting a table that answers "the numbering
// agrees" for books it has never heard of.
//
// Notes filed under the translation being read come first — home, byte-exact —
// then followed notes from other translations, renumbered. Deterministic order
// throughout: newest Received first, version id then id breaking ties, so the
// same store always shows the same note. bible is the reading translation's
// loaded data, consulted for book existence (nil skips that test).
func noteForChapter(p prefStore, versionID, book string, chapter int, bible *BibleData) (StoredNote, bool) {
	s := readNoteStore(p)
	var candidates []StoredNote
	for _, n := range s.notes {

		// directive); only received notes reach the reading page.
		if n.Kind != noteKindReceived || !displayableNote(n) {
			continue
		}
		candidates = append(candidates, n)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Received != b.Received {
			return a.Received > b.Received // newest first
		}
		if a.VersionID != b.VersionID {
			return a.VersionID < b.VersionID
		}
		return a.ID > b.ID // newest arrival breaks the remaining tie
	})

	// Native: filed under the translation being read. Returned byte-exact —
	// no mapping ran, so the note comes home even where the delta tables are
	// wrong about its translation. Selection deliberately ignores Minimized —
	// a collapsed note is still the chapter's note, shown as a chip (the
	// shipped semantics).
	for _, n := range candidates {
		if !strings.EqualFold(n.VersionID, versionID) || n.Book != book {
			continue
		}
		if _, on := placementRunOn(resolveNoteAnchor(n, versionID, bible), chapter); on {
			return n, true
		}
	}

	// Followed: the passage is the same passage in every translation, so the
	// note goes with it — mapped, not copied, because chapter and verse
	// numbers do not mean the same thing across translations (the Romans
	// doxology moves). The unplaced arms are the cases where the honest thing
	// is to show nothing: Greek Esther is a different book, not a renumbering,
	// and a book this translation does not contain has nowhere to show a note
	// however confidently the table maps it.
	for _, n := range candidates {
		if strings.EqualFold(n.VersionID, versionID) || n.Book != book {
			continue
		}
		run, on := placementRunOn(resolveNoteAnchor(n, versionID, bible), chapter)
		if !on {
			continue
		}
		out := n
		// VersionID is deliberately NOT rewritten: it says where the note is
		// FILED. Only the location is renumbered — the covering run's first
		// Lo/Hi becomes the mirror anchor (the arity-1 display; the plural
		// plan is S7) — and the ID rides along, so the verbs address the
		// record wherever its passage displays (X13's fix: nothing rebuilds a
		// key from where the reader is standing).
		out.Chapter = chapter
		if n.VerseLo > 0 {
			out.VerseLo = run.Lo
			out.VerseHi = run.Hi
		}
		return out, true
	}
	return StoredNote{}, false
}

// --- the live view of all this, for the panes -------------------------------

// applyNoteForCurrentChapter derives the chapter PLAN (notes_plan.go) and
// projects its display note into the AppState mirror — text, minimized, verse,
// and the ID the verbs address. Called on every navigation.
//
// THE MIRROR IS A PROJECTION NOW (S7). The model is plural — buildChapterPlan
// answers with every note on the passage — and this function truncates it to
// the one note the shipped surfaces draw, byte-identically to the pre-plan
// derive. S8 points the surfaces at the plan itself and retires the mirror.
func applyNoteForCurrentChapter(state *AppState) {
	if state == nil {
		return
	}
	// Off means off, but it does NOT mean gone: the stored notes stay where
	// they are unless the reader asked for them to be deleted, so switching
	// back on brings them all back. The note's own mark goes with the note it
	// explained — with the feature off no surface draws a note, so a mark a
	// note placed would be a tinted verse with nothing to say why (X4, defect
	// 1 through the off switch). Ownership is RECORDED (mark.go), so this
	// clears ONLY a mark hlNote placed: a search result or a link span on the
	// same chapter is not the notes feature's to take down. turnNotesOff
	// (notes_setting.go) clears at the moment of the switch; this is the same
	// equality on every re-derive route, so no path to "off" can strand one.
	if !notesFeatureOn(state) {
		state.clearMarkFromNote()
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		state.NoteID = 0
		return
	}
	plan := buildChapterPlan(state, appPrefs(), state.Bible)
	// YOUR OWN NOTE, the requested behavior to see it. It is not in plan.Notes and so
	// has no display index — it is the plan's own slot, filled only while
	// noteFocus names it (buildChapterPlan). Projected exactly like a received
	// note so every surface draws it through the one path it already has, and
	// so the mark below is raised from the note's own span; the byline
	// (senderByline) is what says whose words these are.
	if plan.HasOwn {
		own := displayCopy(plan.Own, state.currentVersion().ID, state.CurrentChapter)
		state.ActiveNote = own.Text
		state.NoteMinimized = false // ephemeral: never the stored Minimized bit
		state.NoteVerseLo = own.VerseLo
		state.NoteID = own.ID
		if !notesSuppressed(state) && own.VerseLo > 0 {
			state.setMark(hlNote, VerseSpan{
				VersionID: state.currentVersion().ID,
				Book:      state.CurrentBook,
				Chapter:   state.CurrentChapter,
				Lo:        own.VerseLo,
				Hi:        own.VerseHi,
			})
		}
		return
	}
	if plan.display < 0 {
		// Nothing to draw. The note's own highlight goes with it — ownership
		// is RECORDED (mark.go), so this is an equality, and a highlight that
		// arrived for any OTHER reason is not the note's to clear.
		state.clearMarkFromNote()
		state.ActiveNote = ""
		state.NoteMinimized = false
		state.NoteVerseLo = 0
		state.NoteID = 0
		return
	}
	// The display note, renumbered into the reading translation exactly as
	// the pre-plan derive renumbered it.
	n := displayCopy(plan.Notes[plan.display], state.currentVersion().ID, state.CurrentChapter)
	state.ActiveNote = n.Text
	// THE MIRROR HONOURS THE PLAN'S Open, not the stored bit alone.
	//
	// Three things keep a note closed and only ONE of them is durable:
	// Minimized (the reader pressed −), focus NONE (the reader closed the
	// session's open note), and suppression (a foreign mark owns the page).
	// The plan folds all three into Open; the mirror used to read Minimized and
	// so could not see the other two.
	//
	// What that cost, measured: with a friend's note also on the passage,
	// putting YOUR OWN note away opened THEIRS in its place — fully expanded,
	// with the wash jumping onto their verse — because focus fell to none, the
	// display index moved to their note, and the mirror asked only whether they
	// had pressed − on it. That is exactly what N3 forbids: "nothing may take
	// the closed one's place under the reader's eyes." The plan was right
	// throughout (openNote() reported false); only the projection was wrong.
	state.NoteMinimized = !plan.Notes[plan.display].Open
	state.NoteVerseLo = n.VerseLo
	state.NoteID = n.ID // the identity every verb addresses, carried whole
	if state.NoteMinimized {
		return
	}
	// Never clobber a highlight that is on the page for another reason —
	// arriving by a search result, say. That highlight is what the reader
	// just asked for; the note's is only a default. This is the SAME
	// predicate as the plan's derived suppression (notesSuppressed), on
	// purpose: it used to ask markHere — a foreign mark on THIS chapter —
	// which was the same thing until renumberMarkForVersion could carry a
	// mark cleanly onto another chapter (the doxology under a version
	// switch); a chapter-scoped guard then walked over the mark the reader
	// was holding while the plan stood the notes down for it. One predicate,
	// two consumers, no seam.
	if notesSuppressed(state) {
		return
	}
	if n.VerseLo > 0 {
		// The span's numbers are in the READING translation's numbering —
		// displayCopy just renumbered them — so the span carries the reading
		// frame, not the note's filing. The filing rides on the note
		// (StoredNote.VersionID says where it LIVES); the frame rides on the
		// span (VerseSpan.VersionID says what numbers these ARE), and a
		// version switch consults the frame (renumberMarkForVersion). Handing
		// the filing to the frame is how a followed note's mark would claim
		// numbers it does not have.
		sp := n.span()
		sp.VersionID = state.currentVersion().ID
		state.setMark(hlNote, sp)
	}
}

// rememberIncomingNote stores a note that just arrived on a link and returns
// the stored record — the caller mirrors its ID into AppState so the verbs
// can reach it. ok=false when nothing was stored (no note, or the store is
// unreadable and stood the write down).
//
// EVERYTHING FILED COMES FROM THE TARGET, NOTHING FROM AppState. This used to
// read Chapter: state.CurrentChapter — the chapter the READER was standing on,
// not the one the link named — and was only ever right because applyShareTarget
// happened to navigate first (and wrong the moment navigation clamped the
// chapter). A record that can see where the reader is standing will eventually
// record where the reader is standing; the note is filed under the LINK's
// anchor ([redacted-retired-private-reference], hard case 11).
func rememberIncomingNote(state *AppState, t ShareTarget) (StoredNote, bool) {
	if state == nil || strings.TrimSpace(t.Note) == "" {
		return StoredNote{}, false
	}
	// YOUR OWN NOTE, COMING HOME. If this link carries a nonce that matches a
	// note YOU shared, it is not an arrival from anyone — it is your own words
	// returning, and storing a second copy is what made the app show them back
	// to you under "Note from Friend". So: store nothing, and answer with the
	// note you already have, so the caller focuses THAT and the passage draws
	// it as yours.
	//
	// THE NONCE, NOT THE WORDS. Matching on content would fold a friend's
	// "Amen" on the verse you already wrote "Amen" on into your record, and
	// their message would be gone with nothing to show it ever arrived. The
	// scrapbook keeps what it is given; identity is the only safe key, and the
	// wire carries none except this one (share_note.go, noteTagNonce).
	//
	// A link from before the nonce existed, or one whose note this build could
	// not read, has none — it stores as it always did. That is the honest
	// fallback: a duplicate is a nuisance, a swallowed message is not.
	if t.NoteNonce != ([noteNonceLen]byte{}) {
		if mine, ok := findNoteByNonce(appPrefs(), t.NoteNonce[:]); ok {
			return mine, true
		}
	}
	return addNote(appPrefs(), StoredNote{
		Kind: noteKindReceived,
		// The LINK's anchor, not whatever the reader happens to be reading. A
		// note is a remark on particular wording, so it belongs to the
		// translation it was written against — and the payload's own
		// v/b/c/a records outrank the lossy path (noteStorageTarget).
		VersionID:  t.VersionID,
		Book:       t.Book,
		Chapter:    t.Chapter,
		VerseLo:    t.VerseLo,
		VerseHi:    t.VerseHi,
		AnchorRuns: storableNoteRuns(t),
		Text:       t.Note,
		// What the payload carried that this build could not use, preserved so
		// a future forward/re-share can re-emit it (spec rule 3).
		WireSkipped: []byte(t.NoteSkipped),
		WireOpaque:  []byte(t.NoteOpaque),
	})
}

// maxStoredAnchorRuns bounds the run set one note may file. A genuine 'a'
// record holds one selection, split at most where a projection genuinely
// splits (the doxology: two runs); the wire grammar, though, admits ~2,000
// runs inside its byte cap, and a declared count must never buy unbounded
// resolution work on every navigation. Far above any honest anchor, far below
// abuse.
const maxStoredAnchorRuns = 32

// storableNoteRuns is the wire's full 'a' run set as the store files it: every
// run, in the storage target's own chapter (the wire carries one 'c' for the
// whole set), single verses in the store's Hi==0 spelling.
func storableNoteRuns(t ShareTarget) []anchorRun {
	wire := noteRunsFromSpelling(t.NoteRuns)
	if len(wire) == 0 {
		return nil
	}
	if len(wire) > maxStoredAnchorRuns {
		wire = wire[:maxStoredAnchorRuns]
	}
	runs := make([]anchorRun, 0, len(wire))
	for _, r := range wire {
		run := anchorRun{Chapter: t.Chapter, Lo: r.Lo}
		if r.Hi > r.Lo {
			run.Hi = r.Hi
		}
		runs = append(runs, run)
	}
	return runs
}

// hideCurrentNote / restoreCurrentNote / dropCurrentNote are the verbs. Each
// addresses AppState.NoteID — the identity of the note whose text is on
// screen, handed to the mirror by the derive — and NEVER a key rebuilt from
// state.CurrentBook / CurrentChapter / currentVersion(). Rebuilding the key
// from where the reader is standing is what deleted the wrong note (X1), made
// Hide and Show address different objects (X5), and left a cross-chapter note
// unreachable by any verb (X13).
func hideCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	// YOUR OWN NOTE IS PUT AWAY, NOT MINIMIZED. It is on the passage only
	// the requested behavior to see it, and only until you navigate away, so a
	// durable Minimized bit would record something that is never true of it —
	// and the notes browser, whose only reader of that bit is a sentence about
	// being "minimized in the chapter", would then say something false. Putting
	// it away is exactly focus falling to none: the same thing navigating away
	// does, asked for a moment earlier.
	if isOwnLiveNote(state) {
		state.focusNone()
		state.clearMarkFromNote()
		applyNoteForCurrentChapter(state)
		return
	}
	state.NoteMinimized = true
	// The ONLY store write any focus change may make: an explicit Hide is the
	// one durable collapse, and Minimized means exactly "this reader pressed
	// minimize on this note". Focus falls to NONE, never to another note —
	// nothing may take the closed note's place under the reader's eyes (N3).
	setNoteMinimizedByID(appPrefs(), state.NoteID, true)
	state.focusNone()
	// Only the note's own mark. Hiding a note used to put out whatever was
	// lit, so a reader who arrived on a search result and then collapsed a
	// note on the same chapter lost the result they had come for (X10).
	state.clearMarkFromNote()
}

func restoreCurrentNote(state *AppState) {
	if state == nil || state.ActiveNote == "" {
		return
	}
	// Pressing the pill is the reader choosing this note as the page's reason,
	// exactly like tapping its chip in the Fyne banner (the identity table:
	// "taps a note chip instead → that is the new choice") — so a foreign mark
	// stands aside and the suppression it caused releases. Without this, the
	// press did nothing visible while a search result was lit: the restore
	// re-derived into a still-suppressed plan and the sticker stayed a pill.
	if state.mark.live() && !state.mark.fromNote() {
		state.clearMark()
	}
	state.NoteMinimized = false
	setNoteMinimizedByID(appPrefs(), state.NoteID, false)
	if state.NoteID != 0 {
		// Show is the session act of opening THIS note: focus names it, and
		// the re-derive refreshes the mirror from the plan and re-raises the
		// note's own mark (without clobbering a foreign one).
		state.focusNote(state.NoteID)
		applyNoteForCurrentChapter(state)
		return
	}
	// A note that never reached the store (it arrived while the store was
	// unreadable): the mirror is all there is, so re-raise its mark from it.
	if state.NoteVerseLo > 0 {
		state.setMark(hlNote, VerseSpan{
			VersionID: state.currentVersion().ID,
			Book:      state.CurrentBook,
			Chapter:   state.CurrentChapter,
			Lo:        state.NoteVerseLo,
			Hi:        state.NoteVerseLo,
		})
	}
}

func dropCurrentNote(state *AppState) {
	if state == nil {
		return
	}
	// ✕ ON YOUR OWN NOTE DISMISSES IT; IT DOES NOT DESTROY IT.
	//
	// On a received note ✕ deletes, and that is right: it is someone else's
	// message, you have read it, and the store is yours to prune. On your OWN
	// note the same press would delete the only record of something you wrote —
	// unconfirmed, in one tap, from a card that appeared the requested behavior to
	// look at it and that is about to disappear on its own when you navigate
	// away. That is not a trade any reader would knowingly make.
	//
	// So here it means "put it away", like −. Deleting your own note stays an
	// explicit act in the notes browser, where you are looking at a list of
	// your own notes and the row you press is unambiguous.
	if isOwnLiveNote(state) {
		state.focusNone()
		state.clearMarkFromNote()
		applyNoteForCurrentChapter(state)
		return
	}
	deleteNoteByID(appPrefs(), state.NoteID)
	state.ActiveNote = ""
	state.NoteMinimized = false
	state.NoteVerseLo = 0
	state.NoteID = 0
	// FOCUS RETURNS TO THE DEFAULT RULE, and this is not the same as the reader
	// closing a note. A deleted note is GONE, not put away, so nothing is
	// "taking its place under the reader's eyes" (N3) when the rest of the set

	// requirement, from the report that deleting one note made every pill on
	// the passage vanish.
	//
	// It said focusNone here, and said so in a comment ("deleting is closing,
	// not choosing a neighbour") that the code did not actually implement: the
	// mirror read the stored Minimized bit and never looked at focus, so the
	// none had no effect on what was drawn and the two disagreed in silence.
	// Making the mirror honour the plan's Open turned that dormant conflation
	// into a real regression — the survivor pilled instead of surfacing — which
	// is how it was found.
	state.resetNoteFocus()
	// As in hideCurrentNote: the note's mark goes, a search result's or a
	// shared link's stays.
	state.clearMarkFromNote()
	// And the REST OF THE SET comes back. This line is the retirement of a
	// debt the old comment here recorded: pre-S8 the pane stayed blank until
	// the next navigation re-derived, which at arity 1 merely looked calm —
	// and at arity 3 looked like the delete took EVERY note with it (field
	// report: "all the note pills disappear... until I navigate away and
	// come back"). The projection re-derives from the store the delete just
	// wrote: the plan's remaining notes surface with their honest count, or
	// nothing where nothing remains — the same one source of truth every
	// other verb ends on.
	applyNoteForCurrentChapter(state)
}

// applyNoteOnResume surfaces a stored note for the chapter the app is
// REOPENING into.
//
// It exists because reopening never went through addRecentChapter. The restore
// path sets book and chapter directly (reading_state.go), so nothing called
// applyNoteForCurrentChapter and the chapter the reader last had open came
// back bare. The note is restored WHOLE — bubble and highlight — and the
// scroll is settled in the reading panes, where a pending restore outranks the
// highlight (see AppState.restore and openSearchResultRange).
func applyNoteOnResume(state *AppState) {
	if state == nil {
		return
	}
	applyNoteForCurrentChapter(state)
}
