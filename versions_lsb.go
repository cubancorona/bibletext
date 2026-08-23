//go:build lsb

package bibletext

// THE LSB, BEHIND A BUILD TAG — the same treatment the NRSV got, for the same
// reason: it is gated out of the default build rather than removed.
//
// See versions_nrsv.go for why this is a build tag and not a runtime flag: a
// runtime switch would still carry the name, the publisher line and the
// provider plumbing in every shipped binary.
//
// To build with it:      go build -tags lsb ./...
// With both evaluations: go build -tags nrsv,lsb ./...
// (and the same --tags for `fyne package`.)
//
// WITH BOTH GATED OUT THE DEFAULT BUILD HAS NO UNLICENSED VERSION AT ALL, which
// is worth knowing before reaching for one: the picker's "evaluation in
// progress" footer never renders, and the placeholder/testing path
// (BIBLETEXT_ENABLE_TESTING) has no registered version to exercise. Tests that
// need that shape now build a BibleVersion value directly rather than looking
// one up in the shipping catalogue — which is the better test anyway, since it
// stops "does the app offer this translation" and "does an unconfigured
// licensed source behave" from being the same question.
//
// IF IT IS EVER RESTORED, a one-line description for the picker was drafted and
// not used (see the note above versionRow in versions_ui.go):
//
//	A strictly literal revision in the NASB line, distinctive for rendering
//	the divine name as "Yahweh" rather than "the LORD".
//
// That marker is worth keeping in the sentence: no translation this app ships
// renders the divine name that way — the WEB here reads "The LORD is my
// shepherd" — so it is the difference a reader sees within seconds of
// switching, not a fact about lineage they have to take on trust.
func init() {
	registeredVersions = append(registeredVersions, BibleVersion{
		ID: "lsb", Name: "Legacy Standard Bible", Abbrev: "LSB",
		Publisher: "© The Lockman Foundation — license required",
		source:    newLicensedSource("lsb"),
	})
}
