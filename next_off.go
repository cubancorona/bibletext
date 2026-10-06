//go:build !next

package bibletext

// The next major release, switched off: this is the shipping build.
//
// Work meant for the next major version lands on main behind the `next` build
// tag, so a minor release can be cut from main at any time and carry none of
// it. Every gated behaviour reads nextRelease, or lives in a pair of files
// split on the same tag, and without the tag it is compiled out entirely: the
// binary a reader receives holds no trace of it, which a runtime setting could
// not promise.
//
// Without the tag the app must behave exactly as the current release does.
// docs/NEXT.md is the plan: what is behind the switch, how to build and test
// both states, and how the work behind it is made unconditional on the day
// the major version ships.
//
// No release path passes the tag. next_release_guard_test.go holds each one's
// text to that, and scripts/verify-not-next.sh refuses any built artifact
// whose recorded build tags include it.

const nextRelease = false

// NextRelease is the same switch for the commands built beside this package
// (cmd/websitegen), which cannot read nextRelease.
const NextRelease = nextRelease
