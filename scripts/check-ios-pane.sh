#!/usr/bin/env bash
# Compile the iOS pane on this host, in seconds, without packaging anything.
#
# reading_ios.go is thousands of lines of Objective-C inside a cgo preamble, and
# `go build ./...` never sees one of them: the file is behind //go:build ios, so
# the host build skips it and every ordinary test run is blind to it. Locally,
# the only thing that compiled it was scripts/run-ios-sim.sh — three minutes of
# fyne packaging away, and long enough that a typo in the preamble can survive
# several rounds of "the tests pass".
#
# It compiles twice. First against the toolkit go.mod names, stock Fyne, which
# is what `go build` sees. Then against the PATCHED toolkit the mobile builds
# ship (patches/, applied by setup-fyne-patch.sh): go.mod carries no `replace`,
# so without the second pass nothing but a full packaging build ever compiled a
# patch — a broken edit to the iOS app delegate passed this check. The patched
# tree is regenerated each run, so a stale copy cannot stand in for the patches,
# into the check's own directory (build/ios-check/fyne), never third_party/fyne:
# a packaging build may be compiling from that one. The `replace` goes into a
# scratch copy of go.mod, never the real one.
#
# This is the SAME cross-compile the macOS CI job runs, factored out so the two
# cannot drift and so it can be run in the local loop. Output is thrown away;
# nothing is signed and no simulator is touched.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$PWD"

IOS_MIN="$(python3 -c 'import json;print(json.load(open("config/product.json"))["iosMinimumOSVersion"],end="")')"
SDK="$(xcrun --sdk iphoneos --show-sdk-path)"
CC="$(xcrun --sdk iphoneos -f clang)"

# fyne's gl package is full of OpenGLES, deprecated on every modern SDK; the
# define is the difference between a readable failure and a thousand lines of
# noise around it.
COMMON="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN -DGLES_SILENCE_DEPRECATION"

scratch="$(mktemp -d -t bibletext-ios-typecheck)"
trap 'rm -rf "$scratch"' EXIT

compile() { # <label> [extra go flags...]
  local label="$1"; shift
  local log="$scratch/$label.log"
  if CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC="$CC" \
       CGO_CFLAGS="$COMMON" CGO_LDFLAGS="$COMMON" \
       go build "$@" -o /dev/null ./cmd/mobile >"$log" 2>&1; then
    return 0
  fi
  # Keep the log before anything else can fail: the EXIT trap removes $scratch.
  local kept="${TMPDIR:-/tmp}/bibletext-ios-typecheck-$label.log"
  cp "$log" "$kept"
  echo "The iOS pane does not compile against the $label toolkit:" >&2
  { grep -E "error:|^#" "$log" || tail -20 "$log"; } | head -40 >&2 || true
  echo "  (full log: $kept)" >&2
  exit 1
}

compile stock

PATCHED="build/ios-check/fyne"
BIBLETEXT_FYNE_DEST="$PATCHED" scripts/setup-fyne-patch.sh >"$scratch/setup.log" 2>&1 || {
  echo "Could not regenerate the patched toolkit:" >&2
  tail -20 "$scratch/setup.log" >&2
  exit 1
}
cp go.mod go.sum "$scratch/"
go mod edit -modfile="$scratch/go.mod" -replace "fyne.io/fyne/v2=$ROOT/$PATCHED"
compile patched -modfile="$scratch/go.mod"

echo "OK: the iOS pane compiles against the stock and the patched toolkit."
