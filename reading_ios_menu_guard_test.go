package bibletext

// THE iOS SELECTION MENU'S CONTRACT, held by parsing the source.
//
// The menu is built in Objective-C inside a cgo preamble (reading_ios.go,
// editMenuForTextInRange:suggestedActions:), so it cannot be exercised from a
// Go test on this machine — the same problem notes_spacing_spec_test.go solves
// the same way, and for the same reason: a rule nothing can check is a rule
// that quietly stops being true.
//
// WHAT IS BEING GUARDED, and why it is worth a test rather than a comment:
//
//  1. THE SYSTEM'S SHARE IS DROPPED. It offered a second thing called "Share"
//     a few rows under ours, and it shared the raw selected string — scripture
//     with no reference and no translation named, from an app that names the
//     version everywhere else. Ours sends the quote WITH its citation and
//     version (composeShareText, share.go). Verified on the simulator when this
//     landed: the menu reads Copy · Study with AI · Share · Cross-references ·
//     Look Up · Translate · Search Web, and no system Share.
//
//  2. IT FAILS OPEN. The filter matches UIMenuShare — UIKit's own identifier,
//     read out of the SDK headers rather than recalled — and everything it does
//     not recognise is KEPT. If a later iOS renames or restructures that item,
//     the duplicate simply returns, which is merely the old behaviour. The
//     opposite failure, a broad match that eats Look Up or Translate, is one a
//     reader could not diagnose and we would not hear about.
//
//  3. COPY SURVIVES. It is what makes dropping the system share cost nothing:
//     a reader who wants the words alone still has one press. The standard-edit
//     group is added FIRST and unfiltered, and this asserts that is still so.

import (
	"strings"
	"testing"
)

func TestIOSSelectionMenuDropsOnlyTheSystemShare(t *testing.T) {
	src := readNativeSource(t, "reading_ios.go")

	// 1. The drop, by UIKit's own identifier.
	if !strings.Contains(src, "isEqualToString:UIMenuShare") {
		t.Error("the system Share is no longer filtered out of the selection menu.\n" +
			"It shares the raw selection — scripture with no reference and no " +
			"translation named — beside our own Share, which sends the citation too.")
	}

	// 2. Fail open: the standard-edit group is still taken by identifier, and
	//    unrecognised elements still reach systemRest rather than being dropped.
	for frag, why := range map[string]string{
		"isEqualToString:UIMenuStandardEdit": "Copy/Cut/Paste must still be identified and kept first — " +
			"Copy is what makes dropping the system Share cost the reader nothing",
		"[systemRest addObject:el]": "unrecognised system actions must still be KEPT. A filter that " +
			"dropped what it could not identify would eat Look Up or Translate on some future iOS, " +
			"silently and undiagnosably",
	} {
		if !strings.Contains(src, frag) {
			t.Errorf("reading_ios.go no longer contains %q — %s", frag, why)
		}
	}

	// 3. And the filter stays NARROW: matching on a substring of the title, or
	//    on anything but the identifier, is how this turns into a broad rule
	//    that removes the wrong thing in another language or another release.
	for _, banned := range []string{
		`localizedTitle containsString:@"Share"`,
		`title isEqualToString:@"Share"`,
	} {
		if strings.Contains(src, banned) {
			t.Errorf("the selection menu filters on %q — a title match is locale-dependent "+
				"and would remove the wrong item for a reader in another language. Match the "+
				"UIMenu identifier instead.", banned)
		}
	}
}
