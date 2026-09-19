#!/usr/bin/env bash
# Compile the Android pane on this host, in seconds, without packaging anything.
#
# This is the twin of scripts/check-ios-pane.sh, and it exists because the
# Android side had no compile gate ANYWHERE — not in CI, not locally. The
# android-tagged sources (the JNI bridge, the audio service glue, the native
# reading pane) are behind //go:build android, so `go build ./...`, `go vet` and
# every ordinary test run skip them entirely. The only thing that compiled them
# was a full scripts/build-android.sh run on one particular machine, which meant
# a typo in a cgo preamble could survive indefinitely on every other machine and
# every CI run.
#
#   scripts/check-android-pane.sh
#
# Output is thrown away; nothing is signed, packaged or installed.
set -euo pipefail
cd "$(dirname "$0")/.."

# minSdkVersion 21 is asserted twice in scripts/build-android.sh against the
# built package; the compile must target the same floor or it can accept code
# the shipped package cannot run.
API=21
PINNED_NDK=27.2.12479018

find_ndk() {
  if [ -n "${ANDROID_NDK_HOME:-}" ] && [ -d "${ANDROID_NDK_HOME}" ]; then
    echo "$ANDROID_NDK_HOME"; return
  fi
  for home in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}" "$HOME/Library/Android/sdk" "$HOME/android-sdk" /usr/local/lib/android/sdk; do
    [ -n "$home" ] || continue
    if [ -d "$home/ndk/$PINNED_NDK" ]; then echo "$home/ndk/$PINNED_NDK"; return; fi
    # Any NDK is better than none for a typecheck, but say which one was used.
    latest="$(ls -1d "$home"/ndk/* 2>/dev/null | sort -V | tail -1 || true)"
    if [ -n "$latest" ]; then echo "$latest"; return; fi
  done
}

NDK="$(find_ndk)"
if [ -z "$NDK" ]; then
  # A check that quietly skips is worse than no check, because it reads as a
  # pass. On a runner this is a hard failure; locally it tells you what to do.
  if [ -n "${CI:-}" ]; then
    echo "::error::no Android NDK found; the android sources would go uncompiled and this gate would prove nothing" >&2
    exit 1
  fi
  echo "no Android NDK found (looked for ndk/$PINNED_NDK under ANDROID_HOME and the usual paths)." >&2
  echo "install it with: sdkmanager 'ndk;$PINNED_NDK'" >&2
  exit 2
fi

case "$(uname -s)" in
  Darwin) HOST_TAG=darwin-x86_64 ;;
  Linux)  HOST_TAG=linux-x86_64 ;;
  *) echo "unsupported host $(uname -s)" >&2; exit 2 ;;
esac

CC="$NDK/toolchains/llvm/prebuilt/$HOST_TAG/bin/aarch64-linux-android$API-clang"
[ -x "$CC" ] || { echo "no compiler at $CC" >&2; exit 2; }

# Portable: `mktemp -t PREFIX` is BSD-only and fails on the Linux runner.
log="$(mktemp)"
if CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC="$CC" \
     go build -o /dev/null ./cmd/mobile >"$log" 2>&1; then
  echo "android/arm64 pane compiles (NDK $(basename "$NDK"), API $API)"
  rm -f "$log"
else
  echo "the android/arm64 pane does NOT compile:" >&2
  cat "$log" >&2
  rm -f "$log"
  exit 1
fi
