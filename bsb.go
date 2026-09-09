package bibletext

// Berean Standard Bible (BSB) source.
//
// The BSB is a modern translation dedicated to the PUBLIC DOMAIN (CC0) by BSB
// Publishing on 2023-04-30, so no license is required to ship its text — it is a
// real, selectable version like the WEB, not one of the licensed/evaluation
// entries. Like the WEB today, it comes from the free, key-less "Free Use Bible
// API" at bible.helloao.org (the retired bible-api.com path never carried the
// BSB), which serves the whole translation as a single JSON document. That one ~7 MB
// fetch is decoded into BibleData and cached like any other version (see
// loadVersionData / cachePathForVersion). The cache filename carries the version's
// cacheEpoch so decoder fixes re-decode existing installs instead of being masked
// by stale, already-flattened caches.

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"unicode/utf8"
)

// bsbCompleteURL is helloao's whole-translation endpoint for the BSB: one request
// for all 66 books / 1189 chapters (vs. the WEB's per-chapter fetch loop).
const bsbCompleteURL = "https://bible.helloao.org/api/BSB/complete.json"

// bsbSource serves the public-domain Berean Standard Bible from bible.helloao.org.
type bsbSource struct{}

func (bsbSource) available() bool { return true }

func (bsbSource) fetch() (*BibleData, error) {
	// The whole translation is one large body, so the client's deadline is a
	// STALL watchdog, not a wall clock — a slow connection may take as long
	// as it keeps moving (fetch_stall.go).
	return fetchHelloAOComplete("BSB", bsbCompleteURL, newCorpusClient(), decodeCanonical66)
}

// webCompleteURL is helloao's whole-translation endpoint for the 66-book World English
// Bible (Protestant — the "P"). One request for the entire Bible, vs. bible-api.com's
// ~1189 chapter-by-chapter, rate-limited requests that often never finished, leaving a
// first-run reader stuck on the embedded Gospels seed. The WEB is public domain.
const webCompleteURL = "https://bible.helloao.org/api/ENGWEBP/complete.json"

// fetchWEBFromHelloAO downloads the complete World English Bible from helloao in ONE
// request, decoded by the same path as the BSB (decodeBSBComplete maps a 66-book helloao
// complete.json by canonical book order). It backs webSource (versions.go).
func fetchWEBFromHelloAO() (*BibleData, error) {
	return fetchHelloAOComplete("WEB", webCompleteURL, newCorpusClient(), decodeWEB)
}

// decodeWEB is decodeCanonical66 plus the edition's own paragraphing, which
// its runtime feed drops (paragraph_web.go). The BSB keeps decodeCanonical66
// unchanged: its feed carries the breaks itself.
func decodeWEB(body []byte) (*BibleData, error) {
	bd, err := decodeCanonical66(body)
	if err != nil {
		return nil, err
	}
	applyPublisherParagraphs(bd, webParagraphStarts)
	return bd, nil
}

// decodeCanonical66 decodes a 66-book helloao complete.json (BSB, WEB) by canonical book
// order. The Catholic edition has 73 books in a different arrangement, so it uses
// decodeHelloAOCatholic (id-based) — see catholic.go.
func decodeCanonical66(body []byte) (*BibleData, error) {
	return decodeBSBComplete(body, NewBibleData().Books)
}

// fetchHelloAOComplete fetches one of helloao's whole-translation complete.json bodies
// and decodes it with the given decoder. Shared by the BSB, WEB and WEB-Catholic
// sources; label only flavours the error messages. validateBibleData checks against the
// DECODED book list, so it adapts to each edition's canon (66 vs. 73).
func fetchHelloAOComplete(label, url string, client httpClient, decode func([]byte) (*BibleData, error)) (*BibleData, error) {
	body, err := fetchWithRetry(client, url, maxRetries)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", label, err)
	}
	bd, err := decode(body)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}
	// Guard against a truncated/partial parse silently caching an incomplete Bible:
	// every book in the decoded list must have come through with chapters and verses.
	if err := validateBibleData(bd); err != nil {
		return nil, fmt.Errorf("%s data incomplete: %w", label, err)
	}
	return bd, nil
}

// helloAOBook mirrors one book in a bible.helloao.org complete.json. Shared by the
// order-based 66-book decoder (decodeBSBComplete) and the id-based Catholic decoder
// (decodeHelloAOCatholic, catholic.go).
type helloAOBook struct {
	ID    string `json:"id"`
	Order int    `json:"order"`
	// TotalNumberOfVerses is the feed's own count of this book's verse NODES,
	// omitted verses included. It is the only independent witness we get that a
	// book arrived whole, which is the failure a cached Bible hides best: a
	// truncated book reads as a shorter book, not as an error. Reconciled in
	// verse_count.go; never used to decide anything about the text.
	TotalNumberOfVerses int                   `json:"totalNumberOfVerses"`
	Chapters            []helloAOChapterEntry `json:"chapters"`
}

// helloAOChapterEntry is one element of a book's `chapters` array: the chapter
// itself, plus the feed's own count of its verses.
//
// Named rather than left anonymous because tests build these directly, and an
// anonymous struct makes them restate its entire shape — so adding one field
// here broke two unrelated tests. A name means the next field costs nothing.
type helloAOChapterEntry struct {
	// NumberOfVerses is the feed's verse count for THIS chapter, and it sits
	// here, beside `chapter` rather than inside it — worth stating because the
	// obvious guess puts it one level deeper. It localises a shortfall to the
	// chapter instead of leaving it somewhere in a book of 1,533 verses.
	NumberOfVerses int `json:"numberOfVerses"`
	Chapter        struct {
		Number  int               `json:"number"`
		Content []json.RawMessage `json:"content"`
		// Footnotes are the chapter's note BODIES; the in-verse
		// {"noteId":N} markers point into this list. A marker inside a
		// Psalm superscription is captured with the title
		// (Superscription.Footnotes); a marker in any other non-verse
		// node has nowhere to belong and its body is dropped. See
		// docs/FOOTNOTES.md and docs/SOURCE_FIELDS.md.
		Footnotes []struct {
			NoteID    int    `json:"noteId"`
			Caller    string `json:"caller"`
			Text      string `json:"text"`
			Reference struct {
				Chapter int `json:"chapter"`
				Verse   int `json:"verse"`
			} `json:"reference"`
		} `json:"footnotes"`
	} `json:"chapter"`
}

// decodeHelloAOChapters turns one helloao book's chapters into the app's chapter→[]Verse
// map under book. Each chapter's `content` is a flat array of typed nodes; only `verse`
// nodes carry reader text (bsbVerseText retains their internal line breaks). Chapter-level
// line breaks and headings are editorial nodes outside verse text and are skipped on
// purpose (docs/SOURCE_FIELDS.md); Hebrew subtitles (Psalm superscriptions like "A Psalm
// of David") become the chapter's Superscription. Shared by both decoders.
func decodeHelloAOChapters(book string, b helloAOBook, ck *helloAOChecks) (map[int][]Verse, map[int][]OrphanFootnote, map[int]Superscription, map[int][]Heading) {
	chapters := make(map[int][]Verse, len(b.Chapters))
	var orphans map[int][]OrphanFootnote
	var supers map[int]Superscription
	var headings map[int][]Heading
	for _, cj := range b.Chapters {
		num := cj.Chapter.Number
		// noteId → body, for joining the in-verse markers to their text.
		// noteIds number continuously across a BOOK, so per-chapter lookup is
		// simply a subset view; collisions cannot occur.
		bodies := make(map[int]helloAOFootnoteBody, len(cj.Chapter.Footnotes))
		for _, fn := range cj.Chapter.Footnotes {
			bodies[fn.NoteID] = helloAOFootnoteBody{
				text: fn.Text, caller: fn.Caller,
				refChapter: fn.Reference.Chapter, refVerse: fn.Reference.Verse,
			}
		}
		var verses []Verse
		var heads []Heading
		// A chapter-level line_break is the publisher's paragraph boundary:
		// the NEXT verse opens a paragraph. Carried on the verse
		// (Verse.ParaStart) rather than dropped, so every surface paragraphs
		// where the translators did. A break with no verse after it, and a
		// run of them, both collapse to the one flag.
		paraStart := false
		for _, node := range cj.Chapter.Content {
			var head struct {
				Type    string            `json:"type"`
				Number  int               `json:"number"`
				Content []json.RawMessage `json:"content"`
			}
			if err := json.Unmarshal(node, &head); err != nil {
				ck.census().badNode()
				continue
			}
			ck.census().node(head.Type)
			if head.Type == "hebrew_subtitle" {
				// The Psalm title, assembled by the SAME marked-text path
				// verse text uses — identical spacing rules, and the title's
				// note markers resolve to anchors into the title exactly as
				// verse markers do. Titles are text (the Masoretic tradition
				// numbers them as verse 1), rendered as an italic unnumbered
				// line above verse 1; their notes join the chapter-bottom
				// section keyed "Title".
				text, marks, _ := bsbVerseTextMarkedChecked(head.Content, ck, "")
				if text == "" {
					continue
				}
				var notes []Footnote
				for _, m := range marks {
					body, ok := bodies[m.noteID]
					if !ok || strings.TrimSpace(body.text) == "" {
						continue
					}
					// A title's note belongs to verse 0 by the feed's own
					// convention, which is what makes it distinguishable from
					// a note on verse 1.
					ck.noteRefs().note(book, num, 0, body.refChapter, body.refVerse)
					notes = append(notes, Footnote{
						Anchor: m.anchor,
						Text:   strings.TrimSpace(body.text),
						Caller: body.caller,
					})
				}
				// Psalm 119's ALEPH arrives as the psalm's SUBTITLE rather
				// than as a descriptive run, so it would be drawn as the
				// psalm's title — which is why this edition has 117 titles to
				// the Berean's 116. It is not a title; it is the first of the
				// twenty-two acrostic letters, and the other twenty-one arrive
				// as trailing descriptive runs. A title that is a single
				// all-capital word is that and nothing else: across all three
				// editions' 350 subtitles, ALEPH is the only one, and a real
				// title is a sentence ("A Psalm of David, when he fled...").
				if acrosticLetterLabel(text) {
					heads = append(heads, Heading{Text: text, Style: "acrostic"})
					continue
				}
				if supers == nil {
					supers = make(map[int]Superscription)
				}
				supers[num] = Superscription{Text: text, Footnotes: notes}
				continue
			}
			if head.Type == "line_break" {
				paraStart = true
				continue
			}
			if head.Type == "heading" {
				// The publisher's section heading. Captured with the verse it
				// stands above, filled in when that verse arrives; a heading
				// also opens a paragraph, because in print it always does.
				if text := strings.TrimSpace(bsbVerseText(head.Content)); text != "" {
					heads = append(heads, Heading{Text: text, Style: "heading"})
					paraStart = true
				}
				continue
			}
			if head.Type != "verse" {
				continue
			}
			text, marks, levels, label := bsbVerseTextMarkedLevelsChecked(head.Content, ck,
				book+" "+itoa(num)+":"+itoa(head.Number))
			if text == "" {
				// A verse node with a marker but NO text is a critical-text
				// omission (Luke 17:36 and kin): the verse number exists in
				// the versification, the translation omits its words, and
				// the note explains the omission. Capture it as an orphan —
				// keyed by the verse it belongs to — instead of dropping it
				// with the verse. Bodies whose markers sit in a Psalm
				// superscription are captured with the title (the
				// hebrew_subtitle branch above); a marker in any other
				// non-verse node is never scanned and its body is dropped.
				if head.Number > 0 {
					for _, m := range marks {
						body, ok := bodies[m.noteID]
						if !ok || strings.TrimSpace(body.text) == "" {
							continue
						}
						ck.noteRefs().note(book, num, head.Number, body.refChapter, body.refVerse)
						if orphans == nil {
							orphans = make(map[int][]OrphanFootnote)
						}
						orphans[num] = append(orphans[num], OrphanFootnote{
							Verse:  head.Number,
							Text:   strings.TrimSpace(body.text),
							Caller: body.caller,
						})
					}
				}
				// The pending paragraph start is NOT consumed: an omitted
				// verse prints nothing, so the paragraph opens at the next
				// verse that does.
				continue
			}
			var notes []Footnote
			for _, m := range marks {
				body, ok := bodies[m.noteID]
				if !ok || strings.TrimSpace(body.text) == "" {
					continue // a marker with no body is nothing to show
				}
				ck.noteRefs().note(book, num, head.Number, body.refChapter, body.refVerse)
				notes = append(notes, Footnote{
					Anchor: m.anchor,
					Text:   strings.TrimSpace(body.text),
					Caller: body.caller,
				})
			}
			for i := range heads {
				if heads[i].BeforeVerse == 0 {
					heads[i].BeforeVerse = head.Number
				}
			}
			verses = append(verses, Verse{
				BookName:   book,
				Book:       book,
				Chapter:    num,
				Verse:      head.Number,
				Text:       text,
				Footnotes:  notes,
				ParaStart:  paraStart,
				PoemLevels: levels,
			})
			// A trailing descriptive run labels the stanza that FOLLOWS, so it
			// is appended AFTER this verse — the pending-heading sweep above
			// has already run, and the NEXT verse will claim it.
			if label != "" {
				heads = append(heads, Heading{Text: label, Style: "acrostic"})
			}
			// An acrostic letter or an oracle's title heads what comes NEXT.
			paraStart = bsbVerseHasDescriptive(head.Content)
		}
		if len(verses) > 0 {
			chapters[num] = verses
		}
		if len(heads) > 0 {
			if headings == nil {
				headings = make(map[int][]Heading)
			}
			headings[num] = heads
		}
	}
	return chapters, orphans, supers, headings
}

// helloAOFootnoteBody is one chapter-level note body awaiting its in-verse marker.
type helloAOFootnoteBody struct {
	text   string
	caller string
	// The chapter and verse the feed itself says this note belongs to. Read
	// only by the note-reference audit (helloao_checks.go): where a note is
	// ATTACHED is decided by the marker's position, exactly as before.
	refChapter int
	refVerse   int
}

// decodeBSBComplete maps a 66-book bible.helloao.org complete.json into a BibleData
// using the app's canonical book NAMES. helloao identifies books by a USFM code and a
// canonical `order` (1=Genesis … 66=Revelation); appBooks is that same canonical
// sequence, so books are matched by order — the decoded data therefore carries the app's
// own book names, keeping navigation, search, caching and reading-state aligned. Backs
// the BSB and (Protestant) WEB; the Catholic edition maps by id (decodeHelloAOCatholic).
func decodeBSBComplete(body []byte, appBooks []string) (*BibleData, error) {
	var doc struct {
		// The feed names itself, which is the only reason a decoder shared by
		// two editions can say WHICH one failed to add up. Nothing routes on it.
		Translation struct {
			ShortName string `json:"shortName"`
		} `json:"translation"`
		Books []helloAOBook `json:"books"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(doc.Books) == 0 {
		return nil, fmt.Errorf("no books in response")
	}

	audit := newVerseCountAudit(doc.Translation.ShortName)
	defer audit.report()
	redTable := redLetterTableFor(doc.Translation.ShortName)
	checks := &helloAOChecks{
		Census:    newHelloAOCensus(doc.Translation.ShortName, log.Printf),
		NoteRefs:  newNoteRefAudit(doc.Translation.ShortName, log.Printf),
		RedLetter: newRedLetterWitness(doc.Translation.ShortName, log.Printf, redTable != nil),
	}
	defer checks.report(redTable)

	bd := &BibleData{
		Verses: make(map[string]map[int][]Verse, len(appBooks)),
		Books:  append([]string(nil), appBooks...),
	}
	for _, b := range doc.Books {
		if b.Order < 1 || b.Order > len(appBooks) {
			continue // outside the canonical 66 (not expected for the BSB/WEB)
		}
		book := appBooks[b.Order-1]
		chapters, orphans, supers, heads := decodeHelloAOChapters(book, b, checks)
		audit.book(book, b.TotalNumberOfVerses, chapters, orphans)
		if len(chapters) > 0 {
			bd.Verses[book] = chapters
		}
		if len(orphans) > 0 {
			if bd.OrphanFootnotes == nil {
				bd.OrphanFootnotes = make(map[string]map[int][]OrphanFootnote)
			}
			bd.OrphanFootnotes[book] = orphans
		}
		if len(supers) > 0 {
			if bd.Superscriptions == nil {
				bd.Superscriptions = make(map[string]map[int]Superscription)
			}
			bd.Superscriptions[book] = supers
		}
		if len(heads) > 0 {
			if bd.Headings == nil {
				bd.Headings = make(map[string]map[int][]Heading)
			}
			bd.Headings[book] = heads
		}
	}
	// Note: PrepareSearchIndex is left to the caller (loadBibleData), matching
	// FetchBibleFromAPI — the index is built once after caching.
	return bd, nil
}

// bsbVerseText turns one verse's content array into reader text. Elements are
// either plain strings or objects; objects with a "text" field (poetry lines,
// descriptive text) contribute that text, footnote markers ({"noteId":N})
// contribute nothing, and source-authored line breaks ({"lineBreak":true}) are
// retained. Reading surfaces may lay the verse out as prose, but the share-text
// formatter can therefore recover poetry lines without mistaking visual wrapping
// for source structure.
//
// helloao TRIMS the whitespace around every boundary it introduces — both the
// dropped footnote nodes and the poetry/descriptive clauses carry no surrounding
// spaces. So prose runs on either side of a footnote abut with nothing between
// them: "...Eve," + {noteId} + "because..." would render as "Eve,because" if
// concatenated verbatim. Every such boundary sits at a word boundary (verified
// across the whole translation: there are no mid-word footnote splits), so the
// right join is a single space between contributing pieces. An explicit line-break
// node instead becomes a newline. The only exception is when the next piece begins
// with closing
// punctuation or a quote ("...egg" + {noteId} + "?" → "egg?", "...heel." +
// {noteId} + "”" → "heel.”"); bsbTidySpacing strips those — and any space that
// lands just after an opening bracket/quote — after the fact, which is always safe
// because English never spaces before closing or after opening punctuation.
// bsbVerseHasDescriptive reports whether a verse carries a `descriptive` run —
// the USFM \d class, which these feeds use for two different things. In the
// Psalms it is the acrostic letter that heads a stanza: Psalm 119's twenty-two
// of them arrive at the END of the last verse of the stanza before, so the
// verse that FOLLOWS one opens a stanza. In the Berean it also carries an
// oracle's title (Zechariah 12:1), where the same rule holds and the feed
// already marks the break itself.
//
// Only the position is read here. The run's TEXT is still part of the verse,
// which for the acrostic letters is a defect of its own, tracked separately.
// poemLevel reads the indent depth out of a clause's `poem` value, which the
// feeds send as a bare number. A level the app cannot read is reported as 1,
// the shallowest real depth, rather than as no poetry at all.
func poemLevel(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil || n < 1 {
		return 1
	}
	return n
}

func bsbVerseHasDescriptive(content []json.RawMessage) bool {
	for _, node := range content {
		var obj struct {
			Descriptive bool `json:"descriptive"`
		}
		if json.Unmarshal(node, &obj) == nil && obj.Descriptive {
			return true
		}
	}
	return false
}

func bsbVerseText(content []json.RawMessage) string {
	text, _ := bsbVerseTextMarked(content)
	return text
}

// bsbMark is one in-verse footnote marker: which body it points at, and the
// rune offset in the FINAL verse text where the source placed it.
type bsbMark struct {
	noteID int
	anchor int
}

// bsbVerseTextMarked is bsbVerseText plus the footnote-marker positions. The
// TEXT path is byte-for-byte the historical one — the same pieces, the same
// synthesized-space join, the same bsbTidySpacing — so capturing markers can
// never change a verse. Each marker's anchor is computed by tidying the
// PREFIX of pieces before it: every transformation bsbTidySpacing performs is
// local (collapse runs, strip a space beside punctuation), so the tidied
// prefix is exactly the final text up to the marker's word boundary — the
// property the anchor needs, and the reason no sentinel character ever enters
// this pipeline. Anchors are rune offsets. A marker between poem lines lands
// at the end of the earlier line (the "\n" is only synthesized when the NEXT
// poem clause arrives, after the marker).
func bsbVerseTextMarked(content []json.RawMessage) (string, []bsbMark) {
	text, marks, _ := bsbVerseTextMarkedLevels(content)
	return text, marks
}

// The census-carrying forms. The plain names above stay for callers that have
// no census to give — the share and seed paths, and the tests that predate it.
func bsbVerseTextMarkedChecked(content []json.RawMessage, ck *helloAOChecks, ref string) (string, []bsbMark, string) {
	text, marks, _, label := bsbVerseTextMarkedLevelsChecked(content, ck, ref)
	return text, marks, label
}

func bsbVerseTextMarkedLevels(content []json.RawMessage) (string, []bsbMark, []int) {
	text, marks, levels, _ := bsbVerseTextMarkedLevelsChecked(content, nil, "")
	return text, marks, levels
}

// bsbVerseTextMarkedLevels is bsbVerseTextMarked plus the INDENT DEPTH of each
// line. The feeds give every poetry clause a level — 1 for the opening half of
// a Hebrew couplet, 2 for the answering half, and a third exists in the
// Catholic edition — and the app read only whether a level was present at all,
// so every line drew flush left and the pairing print shows was invisible.
//
// One entry per line of the finished text, zero where a line is not poetry.
func bsbVerseTextMarkedLevelsChecked(content []json.RawMessage, ck *helloAOChecks, ref string) (string, []bsbMark, []int, string) {
	var pieces []string
	var marks []bsbMark // anchor holds the piece INDEX until resolved below
	levels := []int{0}  // the depth of each line, the first line included
	var trailingLabel string
	for idx, node := range content {
		last := idx == len(content)-1
		var s string
		if err := json.Unmarshal(node, &s); err == nil {
			if s != "" {
				pieces = append(pieces, s)
			}
			continue
		}
		var obj struct {
			Text      *string         `json:"text"`
			LineBreak bool            `json:"lineBreak"`
			Poem      json.RawMessage `json:"poem"`
			NoteID    *int            `json:"noteId"`
			// The feed's own words-of-Jesus mark. Read as a WITNESS only: the
			// red a reader sees comes from the generated table, which carries
			// rune offsets this flag cannot. See redLetterWitness.
			WordsOfJesus bool `json:"wordsOfJesus"`
			// Descriptive marks a run the publisher sets as a label rather than
			// as the verse's own words — in these feeds, the acrostic letters
			// of Psalm 119 and one oracle title in Zechariah.
			Descriptive bool `json:"descriptive"`
		}
		if err := json.Unmarshal(node, &obj); err == nil {
			switch {
			case obj.Text != nil && *obj.Text != "":
				// Real helloao poetry carries NO lineBreak nodes — each
				// {"text","poem":N} clause IS one source line (verified against
				// live captures: Gen 1:27 = three clauses/three lines, Ps 23:2 =
				// two, all without a single lineBreak). So a poem clause after
				// prior content starts a new line — EXCEPT a clause that begins
				// with closing punctuation (Job 6:6 ends with a bare "?" clause),
				// which belongs to the previous line and must abut. An explicit
				// lineBreak (prose lists like Gen 10:2) already broke the line,
				// so no second break is added after one.
				isPoem := len(obj.Poem) > 0 && string(obj.Poem) != "null"
				prevBreak := len(pieces) > 0 && pieces[len(pieces)-1] == "\n"
				if isPoem && len(pieces) > 0 && !prevBreak && !startsWithClosingPunct(*obj.Text) {
					pieces = append(pieces, "\n")
					levels = append(levels, 0)
				}
				if isPoem {
					// The clause's own depth, which belongs to the line it
					// opens. A line built from more than one clause keeps the
					// first depth it was given.
					if n := poemLevel(obj.Poem); n > 0 && levels[len(levels)-1] == 0 {
						levels[len(levels)-1] = n
					}
				}
				// A descriptive run in the LAST position labels what FOLLOWS,
				// not the verse it sits in: these are Psalm 119's acrostic
				// letters, so verse 8 ended "Don't utterly forsake me. BETH"
				// and BETH is the head of the next stanza. Lift it out, and
				// let the next verse claim it as a heading.
				//
				// Position is the whole rule, and it is the publisher's own
				// structure rather than a guess about the words: a descriptive
				// run anywhere ELSE is a title for the verse it opens (the
				// Berean's Zechariah 12:1) and stays exactly where it is.
				if obj.Descriptive && last && trailingLabel == "" {
					trailingLabel = strings.TrimSpace(*obj.Text)
					continue
				}
				pieces = append(pieces, *obj.Text)
				if obj.WordsOfJesus {
					ck.redLetter().verse(ref)
				}
			case obj.NoteID != nil:
				// The marker contributes nothing to the text (historical
				// behaviour); it records where in the piece stream it stood.
				marks = append(marks, bsbMark{noteID: *obj.NoteID, anchor: len(pieces)})
			case obj.LineBreak:
				pieces = append(pieces, "\n")
				levels = append(levels, 0)
			default:
				// THE SILENT DROP. An item that unmarshals cleanly and matches
				// nothing contributes no text, no marker and no break, and
				// before the census left no trace whatsoever. A feed that grew
				// a new item shape would simply lose those words. No current
				// edition reaches this line, which is exactly why it needs to
				// announce itself when one does.
				ck.census().unmatchedItem(node)
			}
			continue
		}
		// The item is neither a bare string nor an object this decoder can
		// read. Same silence, same reason to break it.
		ck.census().unmatchedItem(node)
	}
	text := bsbTidySpacing(strings.Join(pieces, " "))
	for i, m := range marks {
		marks[i].anchor = utf8.RuneCountInString(bsbTidySpacing(strings.Join(pieces[:m.anchor], " ")))
	}
	// Only report depths that describe the text as it actually came out. The
	// tidier never adds or removes a line, but saying so is cheaper than
	// trusting it, and a verse with no poetry reports nothing at all.
	if lines := strings.Count(text, "\n") + 1; lines != len(levels) {
		return text, marks, nil, trailingLabel
	}
	for _, n := range levels {
		if n > 0 {
			return text, marks, levels, trailingLabel
		}
	}
	return text, marks, nil, trailingLabel
}

// Spacing artifacts that survive the synthesized-space join: a space before
// closing punctuation/quotes, or after an opening bracket/quote. Removing them is
// always safe — neither ever takes an adjacent space in English prose.
var (
	bsbSpaceBeforeClose = regexp.MustCompile(`\s+([,.;:!?)\]}’”])`)
	bsbSpaceAfterOpen   = regexp.MustCompile(`([(\[{“‘])\s+`)
)

// startsWithClosingPunct reports a piece that opens with punctuation belonging
// to the PREVIOUS clause ("?" / "”" / ")." …) — it must join that line, never
// begin a new one; bsbTidySpacing then merges the space away.
func startsWithClosingPunct(s string) bool {
	for _, r := range s {
		switch r {
		case ')', ']', '}', '”', '’', '!', '?', ';', ':', ',', '.':
			return true
		}
		return false
	}
	return false
}

// bsbTidySpacing collapses redundant whitespace WITHIN each source line and
// removes spaces that the per-piece join wrongly placed adjacent to punctuation.
// Newlines themselves survive for the share-text formatter.
func bsbTidySpacing(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		line = bsbSpaceBeforeClose.ReplaceAllString(line, "$1")
		lines[i] = bsbSpaceAfterOpen.ReplaceAllString(line, "$1")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// acrosticLetterLabel reports whether a Psalm subtitle is really one of the
// twenty-two acrostic letters rather than a title.
//
// Psalm 119's ALEPH arrives as a hebrew_subtitle while its other twenty-one
// letters arrive as trailing descriptive runs, so without this the psalm gains
// a title no translator wrote and the World English editions carry 117
// subtitles to the Berean's 116. A single all-capital word is the whole test,
// and it is exact: across the 350 subtitles the three editions send, ALEPH is
// the only one that matches, and every genuine title is a sentence.
func acrosticLetterLabel(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
