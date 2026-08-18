package bibletext

// The notes YOU send.
//
// "Share with note" used to keep nothing: the moment the share sheet closed,
// your own words existed only in the messenger thread. They are kept —

// visible in the notes list, but the reading page stays the other person's).
//
// They live in the ONE scrapbook store as Kind=mine records (notes_store.go).
// They used to be a separate list precisely because the received store was a
// passage-keyed map that would have let your note overwrite a friend's; the
// scrapbook store has no key to collide on, so the separate list folded in —
// saveMyNote / readMyNotes are the Kind=mine reads and writes of that store
// and live beside it.

// noteByline is who the note is from, for display. Untrusted names do not
// appear here yet: there is no name field on the share sheet, so a received
// note can only say "Friend". SenderName is carried and stored and simply not
// read — see [redacted-retired-private-reference], "Identity".
func noteByline(n StoredNote) string {
	if n.Kind == noteKindMine {
		return "From you"
	}
	return "From Friend"
}
