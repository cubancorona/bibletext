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
// The copy's switch is the next such name. AppKit marks two of the element
// kinds it handles macOS 14 (NSBezierPathElementCubicCurveTo,
// NSBezierPathElementQuadraticCurveTo); today's compiler lets a case label
// name them at the floor, and one that stops doing so would fail the Mac
// release on its day. So the two curve kinds are matched by their values, 2
// and 4, which the SDK fixes, and the cubic's deprecated older name, which
// warns in every build that targets macOS 14 or later, is not used either.
//
// Mutation: drop -Werror=unguarded-availability-new from the #cgo CFLAGS of
// reading_macos.go; or set the card's path from btMacNoteBubblePath(w, h).CGPath
// again; or name either macOS 14 element kind, or NSBezierPathElementCurveTo,
// in btMacCGPathCreate.

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

	walk := nativeFunctionSource(t, "reading_macos.go", "static CGPathRef btMacCGPathCreate(NSBezierPath *p) {")
	for _, name := range []string{"NSBezierPathElementCubicCurveTo", "NSBezierPathElementQuadraticCurveTo",
		"NSBezierPathElementCurveTo"} {
		if strings.Contains(walk, name) {
			t.Errorf("btMacCGPathCreate names %s: the two macOS 14 element kinds are matched by value, "+
				"and the cubic's older name is deprecated from macOS 14", name)
		}
	}
	if !regexp.MustCompile(`enum \{ btMacPathCubic = 2, btMacPathQuadratic = 4 \};`).MatchString(preamble) {
		t.Error("reading_macos.go no longer gives the cubic and quadratic element kinds their SDK values, 2 and 4")
	}
	// Every kind of element is still copied: the outline would lose its
	// curves, or its close, without a word.
	for _, kind := range []string{"case NSBezierPathElementMoveTo:", "case NSBezierPathElementLineTo:",
		"case btMacPathCubic:", "case btMacPathQuadratic:", "case NSBezierPathElementClosePath:"} {
		if !strings.Contains(walk, kind) {
			t.Errorf("btMacCGPathCreate no longer handles %q", kind)
		}
	}
}
