package bibletext

// Test-only reach into the mark.
//
// The highlight used to be five exported fields on AppState, so tests set and
// read it by assignment. It is one unexported value now (mark.go), which is the
// point — five fields written by five callers with nothing recording which is
// exactly how a highlight came to outlive the note that placed it. These
// helpers keep the tests reading the way they read before, without reopening
// the fields to production code.
//
// setHL takes an ORIGIN because in the new model there is no such thing as a
// highlight with no reason. A test that does not care which reason should say
// hlSearch: "the reader navigated here", the most neutral of the five.

func (s *AppState) setHL(origin hlOrigin, book string, chapter, lo, hi int) {
	s.setMark(origin, VerseSpan{
		VersionID: s.CurrentVersion,
		Book:      book,
		Chapter:   chapter,
		Lo:        lo,
		Hi:        hi,
	})
}

func (s *AppState) hlOn() bool { return s.hasMark() }

func (s *AppState) hlBook() string {
	sp, _ := s.markSpan()
	return sp.Book
}

func (s *AppState) hlChapter() int {
	sp, _ := s.markSpan()
	return sp.Chapter
}

func (s *AppState) hlLo() int {
	sp, _ := s.markSpan()
	return sp.Lo
}

func (s *AppState) hlHi() int {
	sp, _ := s.markSpan()
	return sp.Hi
}

func (s *AppState) hlOrigin() hlOrigin { return s.mark.Origin }
