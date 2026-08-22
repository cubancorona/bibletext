//go:build lsb

package bibletext

// THE LSB, BEHIND A BUILD TAG — the same treatment the NRSV got, for the same

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
func init() {
	registeredVersions = append(registeredVersions, BibleVersion{
		ID: "lsb", Name: "Legacy Standard Bible", Abbrev: "LSB",
		Publisher: "© The Lockman Foundation — license required",
		source:    newLicensedSource("lsb"),
	})
}
