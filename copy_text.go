package bibletext

// What "copy" puts on the clipboard, for the paths that copy less than a whole
// chapter — docs/SCRIPTURE_WORKLIST.md, S13.
//
// Untagged on purpose. The callers live in the Android fallback pane
// (reading_mobile.go, //go:build android), which no host test can compile, and
// the decision these functions carry — whether the translators' poem lines
// survive a copy — is far too easy to get wrong to leave where nothing can
// check it.

import "strings"

// paragraphCopyText is the clean text a reader expects from "Copy paragraph":
// no verse numbers, and the authored poem lines intact.
//
// Copying a chapter has always kept those lines (chapterCopyText) and so has
// the cited-text share, on the stated principle that poetry copies as poetry.
// Copying a verse or a paragraph flattened them, and no reason was ever given.
//
// Prose verses join with a space, as they always have. A paragraph carrying ANY
// authored break is poetry, and there the verses join with a newline instead: a
// poem line is a unit, so running the next verse onto the end of the last one
// with a space would be the same flattening by another route.
func paragraphCopyText(verses []Verse) string {
	poetry := false
	for _, v := range verses {
		if strings.Contains(v.Text, "\n") {
			poetry = true
			break
		}
	}
	sep := " "
	if poetry {
		sep = "\n"
	}
	parts := make([]string, 0, len(verses))
	for _, v := range verses {
		if t := strings.TrimSpace(v.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, sep)
}

// citedCopy sets a quotation off from its reference the way the share text does
// (composeShareText): a BLANK line, not a dash on the same line. The blank line
// is load-bearing for poetry — a citation on a bare next line would read as one
// more poem line — and one shape for both keeps the two from drifting apart.
func citedCopy(body, ref string) string {
	return body + "\n\n— " + ref
}
