package bibletext

// The notes YOU send.
//
// "Share with note" used to keep nothing: the moment the share sheet closed,
// your own words existed only in the messenger thread. They are kept —
// deliberately NOT drawn in the scripture text (by design: stored, and
// visible in the notes list, but the reading page stays the other person's).
//
// They live in the ONE scrapbook store as Kind=mine records (notes_store.go).
// They used to be a separate list precisely because the received store was a
// passage-keyed map that would have let your note overwrite a friend's; the
// scrapbook store has no key to collide on, so the separate list folded in —
// saveMyNote / readMyNotes are the Kind=mine reads and writes of that store
// and live beside it.

// noteByline is who the note is from, for the Fyne surfaces (the banner and
// the browser). The PERSON half routes through senderName (notes_byline.go)
// with every other surface, so the dormant name path is one constant away on
// all of them at once — today it can only say "Friend", because
// senderNamesEnabled is false and there is no name field on the share sheet.
func noteByline(n StoredNote) string {
	if n.Kind == noteKindMine {
		return "From you"
	}
	return "From " + senderName(n)
}
