//go:build nrsv

package bibletext

// THE NRSV, BEHIND A BUILD TAG.
//
// It was a registered translation carrying a placeholder source: never
// selectable, shown in the picker as "evaluation in progress", and named in the
// support page as a copyrighted translation awaiting licensing. It is now out of
// everything that ships — nothing in the deployed apps or the docs — while the
// wiring stays for anyone who does hold a licence.
//
// A BUILD TAG rather than a runtime switch, deliberately. A runtime flag would
// still carry the name, the publisher line and the provider plumbing in every
// shipped binary, where a curious reader with `strings` would find a translation
// the app does not offer. Tagged out, none of it is compiled at all.
//
// To build with it:  go build -tags nrsv ./...   (and --tags nrsv for fyne package)
//
// The LSB followed the same day, behind its own `lsb` tag (versions_lsb.go).
//
// IF IT IS EVER RESTORED, a one-line description for the picker was drafted and
// not used (see the note above versionRow in versions_ui.go for why none of the
// rows carry one yet):
//
//	A careful, ecumenical revision widely used in universities and mainline
//	churches, updating the 1989 NRSV with newer manuscript scholarship.
//
// A variant naming its most-discussed feature outright, if that is wanted:
// "… with gender-inclusive wording where the translators judge the original
// means both."
func init() {
	registeredVersions = append(registeredVersions, BibleVersion{
		ID: "nrsv", Name: "New Revised Standard Version", Abbrev: "NRSV",
		Publisher: "© National Council of the Churches of Christ — license required",
		source:    newLicensedSource("nrsv"),
	})
}
