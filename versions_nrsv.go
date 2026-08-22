//go:build nrsv

package bibletext

// THE NRSV, BEHIND A BUILD TAG.
//
// It was a registered translation carrying a placeholder source: never
// selectable, shown in the picker as "evaluation in progress", and named in the
// support page as a copyrighted translation awaiting licensing. The owner asked
// for it gone from everything that ships (22 Aug 2026: "remove nrsv … nothing in
// the deployed apps or docs"), while leaving the wiring for anyone who does hold
// a licence.
//
// A BUILD TAG rather than a runtime switch, deliberately. A runtime flag would
// still carry the name, the publisher line and the provider plumbing in every
// shipped binary, where a curious reader with `strings` would find a translation
// the app does not offer. Tagged out, none of it is compiled at all.
//
// To build with it:  go build -tags nrsv ./...   (and --tags nrsv for fyne package)
//
// The LSB is deliberately NOT tagged out alongside it: the owner asked about the
// NRSV alone, and it is not this file's place to widen that.
func init() {
	registeredVersions = append(registeredVersions, BibleVersion{
		ID: "nrsv", Name: "New Revised Standard Version", Abbrev: "NRSV",
		Publisher: "© National Council of the Churches of Christ — license required",
		source:    newLicensedSource("nrsv"),
	})
}
