package bibletext

// Recorded audio for the Berean Standard Bible. The BSB has a COMPLETE
// public-domain (CC0) narration by Barry Hays, streamed per-chapter from
// openbible.com with HTTP range support (so the ±15s skip works). Unlike the WEB
// eBible set (partial coverage), this covers all 66 books, so BSB chapters play a
// real recording instead of falling back to text-to-speech.
//
// File scheme (confirmed from the openbible.com/audio/hays/ directory):
//   BSB_{book:02}_{Abbr3}_{chapter:03}_H.mp3
// e.g. BSB_43_Jhn_003_H.mp3 (John 3), BSB_19_Psa_023_H.mp3 (Psalm 23).
//
// The 3-letter abbreviations are openbible's own and a few are non-obvious
// (Titus=Tts, Song=Sng, Ezekiel=Ezk, Mark=Mrk, John=Jhn, Joel=Jol, Nahum=Nam),
// so the map below was taken verbatim from the live directory listing.

import "fmt"

const bsbAudioBase = "https://openbible.com/audio/hays/"

// bsbAudioBook is a book's BSB-audio identity: its 1–66 number and openbible's
// 3-letter abbreviation.
type bsbAudioBook struct {
	num  int
	abbr string
}

// bsbAudioBooks maps the app's canonical book names to the BSB recording's file
// identity. All 66 books are present (complete coverage).
var bsbAudioBooks = map[string]bsbAudioBook{
	"Genesis":         {1, "Gen"},
	"Exodus":          {2, "Exo"},
	"Leviticus":       {3, "Lev"},
	"Numbers":         {4, "Num"},
	"Deuteronomy":     {5, "Deu"},
	"Joshua":          {6, "Jos"},
	"Judges":          {7, "Jdg"},
	"Ruth":            {8, "Rut"},
	"1 Samuel":        {9, "1Sa"},
	"2 Samuel":        {10, "2Sa"},
	"1 Kings":         {11, "1Ki"},
	"2 Kings":         {12, "2Ki"},
	"1 Chronicles":    {13, "1Ch"},
	"2 Chronicles":    {14, "2Ch"},
	"Ezra":            {15, "Ezr"},
	"Nehemiah":        {16, "Neh"},
	"Esther":          {17, "Est"},
	"Job":             {18, "Job"},
	"Psalms":          {19, "Psa"},
	"Proverbs":        {20, "Pro"},
	"Ecclesiastes":    {21, "Ecc"},
	"Song of Solomon": {22, "Sng"},
	"Isaiah":          {23, "Isa"},
	"Jeremiah":        {24, "Jer"},
	"Lamentations":    {25, "Lam"},
	"Ezekiel":         {26, "Ezk"},
	"Daniel":          {27, "Dan"},
	"Hosea":           {28, "Hos"},
	"Joel":            {29, "Jol"},
	"Amos":            {30, "Amo"},
	"Obadiah":         {31, "Oba"},
	"Jonah":           {32, "Jon"},
	"Micah":           {33, "Mic"},
	"Nahum":           {34, "Nam"},
	"Habakkuk":        {35, "Hab"},
	"Zephaniah":       {36, "Zep"},
	"Haggai":          {37, "Hag"},
	"Zechariah":       {38, "Zec"},
	"Malachi":         {39, "Mal"},
	"Matthew":         {40, "Mat"},
	"Mark":            {41, "Mrk"},
	"Luke":            {42, "Luk"},
	"John":            {43, "Jhn"},
	"Acts":            {44, "Act"},
	"Romans":          {45, "Rom"},
	"1 Corinthians":   {46, "1Co"},
	"2 Corinthians":   {47, "2Co"},
	"Galatians":       {48, "Gal"},
	"Ephesians":       {49, "Eph"},
	"Philippians":     {50, "Php"},
	"Colossians":      {51, "Col"},
	"1 Thessalonians": {52, "1Th"},
	"2 Thessalonians": {53, "2Th"},
	"1 Timothy":       {54, "1Ti"},
	"2 Timothy":       {55, "2Ti"},
	"Titus":           {56, "Tts"},
	"Philemon":        {57, "Phm"},
	"Hebrews":         {58, "Heb"},
	"James":           {59, "Jas"},
	"1 Peter":         {60, "1Pe"},
	"2 Peter":         {61, "2Pe"},
	"1 John":          {62, "1Jn"},
	"2 John":          {63, "2Jn"},
	"3 John":          {64, "3Jn"},
	"Jude":            {65, "Jud"},
	"Revelation":      {66, "Rev"},
}

// bsbAudioURL returns the BSB recorded-narration MP3 URL for a book + chapter and
// whether one is mapped (all 66 canonical books are).
func bsbAudioURL(book string, chapter int) (string, bool) {
	b, ok := bsbAudioBooks[book]
	if !ok || chapter < 1 {
		return "", false
	}
	return fmt.Sprintf("%sBSB_%02d_%s_%03d_H.mp3", bsbAudioBase, b.num, b.abbr, chapter), true
}
