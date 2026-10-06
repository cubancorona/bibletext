//go:build next

package bibletext

// The Gospel parallels of the next major release (docs/NEXT.md), and the
// files that hold them:
//
//   - gospel_parallels_next.json is the synopsis with the twenty-two Gospel
//     verses that are in no set placed (docs/TEXTUAL-DATA.md §9.4). It is
//     gospel_parallels.json with three sets given a passage they lacked and
//     four sets of one Gospel each added, and nothing else changed
//     (parallels_states_test.go).
//   - gospel_occasions.json is the same saying on another occasion, which the
//     shipping build does not have at all.
//
// The init puts both in place before main or any test runs, so every reader
// of the synopsis reads this one; both are parsed on first use, after it.
// Without the next tag this file is not compiled, and neither file is in the
// binary.

import _ "embed"

//go:embed assets/parallels/gospel_parallels_next.json
var nextGospelParallelsJSON []byte

//go:embed assets/parallels/gospel_occasions.json
var nextGospelOccasionsJSON []byte

func init() {
	gospelParallelsJSON = nextGospelParallelsJSON
	gospelOccasionsJSON = nextGospelOccasionsJSON
}
