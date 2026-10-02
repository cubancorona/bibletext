package bibletext

// THE MAC FLOOR IS HELD BY THE COMPILER. The Mac builds compile at
// macMinimumOSVersion (config/product.json, passed as -mmacosx-version-min by
// both release builds), and the package's darwin cgo carries
// -Werror=unguarded-availability-new, so an AppKit or Foundation call newer
// than the floor, made without an @available check, fails those builds
// instead of shipping as an unrecognised selector on the older macOS. A build
// without the floor (the host default, and CI's macOS job) targets the macOS
// it runs on and cannot see such a call, so the flag is held here at the
// source, where every platform's test run can.
//
// The open note card was that call: its outline came from NSBezierPath's
// CGPath property, which exists only from macOS 14, on a Mac app offered from
// macOS 12. It now copies the bezier path element by element
// (btMacCGPathCreate), and no NSBezierPath CGPath read is left in the pane.
//
// Mutation: drop -Werror=unguarded-availability-new from the #cgo CFLAGS of
// reading_macos.go; or set the card's path from btMacNoteBubblePath(w, h).CGPath
// again.

import (
	"regexp"
	"strings"
	"testing"
)

func TestTheMacBuildRefusesAnAPINewerThanItsFloor(t *testing.T) {
	src := readNativeSource(t, "reading_macos.go")
	if !strings.HasPrefix(src, "//go:build darwin && !ios\n") {
		t.Fatal("reading_macos.go no longer builds for every Mac build and only those; its CFLAGS are the package's Mac CFLAGS")
	}
	preamble, _, ok := strings.Cut(src, "\nimport \"C\"")
	if !ok {
		t.Fatal("reading_macos.go has no cgo preamble")
	}
	var cflags []string
	for _, line := range strings.Split(preamble, "\n") {
		if strings.HasPrefix(line, "#cgo CFLAGS:") {
			cflags = append(cflags, strings.Fields(strings.TrimPrefix(line, "#cgo CFLAGS:"))...)
		}
	}
	found := false
	for _, f := range cflags {
		found = found || f == "-Werror=unguarded-availability-new"
	}
	if !found {
		t.Errorf("reading_macos.go's #cgo CFLAGS %q lack -Werror=unguarded-availability-new: "+
			"a call newer than the Mac floor would ship as a warning", cflags)
	}
	if m := regexp.MustCompile(`\.CGPath\b`).FindString(preamble); m != "" {
		t.Errorf("reading_macos.go reads %s, NSBezierPath's macOS 14 property; copy the path with btMacCGPathCreate", m)
	}
	// Matched with its brace: the function is forward-declared above it.
	card := nativeFunctionSource(t, "reading_macos.go", "static void btMacLayoutNote(void) {")
	if !strings.Contains(card, "btMacCGPathCreate(btMacNoteBubblePath(w, h))") {
		t.Error("the note card's outline no longer comes from btMacCGPathCreate(btMacNoteBubblePath(w, h))")
	}
}
