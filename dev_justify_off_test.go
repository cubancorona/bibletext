//go:build !bibletextdev

package bibletext

import "testing"

// A release build justifies as the spec says: nothing sets the override.
func TestReleaseBuildsJustifyAsTheSpecSays(t *testing.T) {
	if readingJustifyOverride != nil {
		t.Fatal("a release build sets the justification override")
	}
	if readingJustify() != readingJustifyProse {
		t.Fatal("a release build does not justify as readingJustifyProse says")
	}
}
