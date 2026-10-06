//go:build next

package bibletext

// The next major release, switched on. Development and CI builds only: see
// next_off.go for what the switch is, and docs/NEXT.md for what is behind it.
//
//	go test -tags next ./...
//	go run -tags next ./cmd/bibletext
//	scripts/run-ios-device.sh --next   (or run-ios-sim.sh --next)
//	BT_ANDROID_TAGS=next scripts/build-android.sh
//
// Nothing that ships passes the tag; a release artifact built with it is
// refused by scripts/verify-not-next.sh.

const nextRelease = true
