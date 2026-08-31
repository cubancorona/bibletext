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
# This is the SAME cross-compile the macOS CI job runs, factored out so the two
# cannot drift and so it can be run in the local loop. Output is thrown away;
# nothing is signed and no simulator is touched.
set -euo pipefail

cd "$(dirname "$0")/.."

IOS_MIN="$(python3 -c 'import json;print(json.load(open("config/product.json"))["iosMinimumOSVersion"],end="")')"
SDK="$(xcrun --sdk iphoneos --show-sdk-path)"
CC="$(xcrun --sdk iphoneos -f clang)"

# fyne's gl package is full of OpenGLES, deprecated on every modern SDK; the
# define is the difference between a readable failure and a thousand lines of
# noise around it.
COMMON="-isysroot $SDK -arch arm64 -miphoneos-version-min=$IOS_MIN -DGLES_SILENCE_DEPRECATION"

log="$(mktemp -t bibletext-ios-typecheck).log"
if CGO_ENABLED=1 GOOS=ios GOARCH=arm64 CC="$CC" \
     CGO_CFLAGS="$COMMON" CGO_LDFLAGS="$COMMON" \
     go build -o /dev/null ./cmd/mobile >"$log" 2>&1; then
  echo "OK: the iOS pane compiles."
  exit 0
fi

echo "The iOS pane does not compile:" >&2
grep -E "error:|^#" "$log" | head -40 >&2
echo "  (full log: $log)" >&2
exit 1
