package bibletext

// Recorded audio for the WEB-Catholic's Greek books. The WEB-Catholic shares the
// WEB text — and David Williams's narration of it — for its protocanon, but three
// kinds of chapter are left over: the nine deuterocanonical books the WEB has no
// recording for at all, the GREEK Esther (a different book from the Hebrew Esther
// Williams read), and the Greek Daniel's chapters 3, 13 and 14 (the Prayer of
// Azariah and the Song of the Three inside chapter 3, plus Susanna and Bel and the
// Dragon). Those 150 chapters are served by eBible.org's WEBBE narration, which is
// SYNTHETIC — a computer voice, not a person, which is why it is credited as one.
// It is public domain ("Copy freely") with no separate recording copyright, and its
// text was verified to match the WEB-Catholic's verse for verse before adoption.
//
// File scheme (eBible's original names, kept verbatim on our mirror, as the BSB set
// keeps openbible's):
//   eng-webbe_{order:03}_{CODE}_{chapter:02}.mp3
// e.g. eng-webbe_046_SIR_07.mp3 (Sirach 7), eng-webbe_066_DAG_13.mp3 (Susanna).

import "fmt"

// webbeRecordingID keys the bundled timing table and names the recording in the
// source menu and in a remembered narrator preference.
const webbeRecordingID = "webbe-synthetic"

// webbeReleaseTag is the single release holding this recording. It deliberately
// does NOT go through audioReleaseTag: that splits a corpus by canonical book
// number (1–39 Old Testament, 40–66 New) to stay under GitHub's 1000-asset cap per
// release, and deuterocanonical books have no such number. At 150 assets this
// recording fits in one release with room to spare.
const webbeReleaseTag = "webbe-synthetic-v1"

// webbeAudioBook is a book's WEBBE-audio identity: eBible's file order number and
// its three-letter book code. ESG is the Greek Esther and DAG the Greek Daniel —
// distinct from the Hebrew EST/DAN, which the Williams recording covers.
type webbeAudioBook struct {
	order int
	code  string
}

// webbeAudioBooks maps the app's book names to the WEBBE recording's file identity.
// Esther and Daniel appear here as their GREEK editions; which of their chapters
// this recording actually serves is decided by the timing table, not this map, so
// Daniel resolves only for 3, 13 and 14 and leaves 1–2 and 4–12 to Williams.
var webbeAudioBooks = map[string]webbeAudioBook{
	"Tobit":       {41, "TOB"},
	"Judith":      {42, "JDT"},
	"Esther":      {43, "ESG"},
	"Wisdom":      {45, "WIS"},
	"Sirach":      {46, "SIR"},
	"Baruch":      {47, "BAR"},
	"1 Maccabees": {52, "1MA"},
	"2 Maccabees": {53, "2MA"},
	"Daniel":      {66, "DAG"},
}

// webbeAudioURL returns the WEBBE synthetic-narration MP3 URL for a book + chapter
// and whether one exists. As with the other recordings the chapter bound comes from
// this recording's own timing table (recordingHasChapter), which holds exactly the
// 150 chapters mirrored — so this reports false for every chapter the Williams
// narration already covers, and the two never compete for the same chapter.
func webbeAudioURL(book string, chapter int) (string, bool) {
	b, ok := webbeAudioBooks[book]
	if !ok || !recordingHasChapter(webbeRecordingID, book, chapter) {
		return "", false
	}
	return fmt.Sprintf("%s%s/eng-webbe_%03d_%s_%02d.mp3", audioHostBase, webbeReleaseTag, b.order, b.code, chapter), true
}
