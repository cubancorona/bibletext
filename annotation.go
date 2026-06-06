package bibletext

import (
	"fmt"
	"sort"
	"time"
)

// VerseRef identifies a single verse. It is the anchor for annotations and the
// unit used when citing or copying for research.
type VerseRef struct {
	Book    string
	Chapter int
	Verse   int
}

func (r VerseRef) String() string {
	return fmt.Sprintf("%s %d:%d", r.Book, r.Chapter, r.Verse)
}

func refOf(v Verse) VerseRef {
	return VerseRef{Book: v.BookName, Chapter: v.Chapter, Verse: v.Verse}
}

// Annotation is a user note and/or highlight attached to a verse. This is the
// data foundation for upcoming annotation/research features; the UI and
// persistence are intentionally not wired yet.
type Annotation struct {
	Ref     VerseRef
	Note    string
	Color   string // optional highlight colour key; "" means a plain note
	Created time.Time
	Updated time.Time
}

// AnnotationStore holds annotations keyed by verse reference. It is in-memory for
// now; persistence can follow the versioned, atomic JSON pattern in cache.go.
type AnnotationStore struct {
	byRef map[string][]Annotation
}

func NewAnnotationStore() *AnnotationStore {
	return &AnnotationStore{byRef: make(map[string][]Annotation)}
}

// Add stores an annotation, stamping timestamps when absent.
func (s *AnnotationStore) Add(a Annotation) {
	now := time.Now().UTC()
	if a.Created.IsZero() {
		a.Created = now
	}
	a.Updated = now
	key := a.Ref.String()
	s.byRef[key] = append(s.byRef[key], a)
}

// ForVerse returns the annotations attached to a verse, in insertion order.
func (s *AnnotationStore) ForVerse(r VerseRef) []Annotation {
	return s.byRef[r.String()]
}

// HasAny reports whether a verse carries any annotation (for gutter markers etc.).
func (s *AnnotationStore) HasAny(r VerseRef) bool {
	return len(s.byRef[r.String()]) > 0
}

// Count returns the total number of annotations stored.
func (s *AnnotationStore) Count() int {
	n := 0
	for _, list := range s.byRef {
		n += len(list)
	}
	return n
}

// Refs returns every annotated verse reference, sorted canonically by book order.
func (s *AnnotationStore) Refs(bookOrder []string) []VerseRef {
	rank := make(map[string]int, len(bookOrder))
	for i, b := range bookOrder {
		rank[b] = i
	}
	refs := make([]VerseRef, 0, len(s.byRef))
	for _, list := range s.byRef {
		if len(list) > 0 {
			refs = append(refs, list[0].Ref)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if rank[refs[i].Book] != rank[refs[j].Book] {
			return rank[refs[i].Book] < rank[refs[j].Book]
		}
		if refs[i].Chapter != refs[j].Chapter {
			return refs[i].Chapter < refs[j].Chapter
		}
		return refs[i].Verse < refs[j].Verse
	})
	return refs
}
