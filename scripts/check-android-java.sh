#!/usr/bin/env bash
# Compile android/*.java on this host, in seconds, without packaging anything.
#
# This is the JAVA twin of scripts/check-android-pane.sh, and it exists for the
# same reason one architecture up. That script closed the gap for the
# android-tagged GO sources. The Java on the other side of the JNI boundary —
# BtBridge and BtAudio, around eighty classes carrying the native selectable
# TextView, the selection menu, the media session and the foreground audio
# service — had no gate at all: `go build ./...`, `go vet`, the whole test
# suite and both mobile pane checks are blind to it, and the ONLY thing that
# compiled it was a full scripts/build-android.sh run on one machine with the
# SDK installed. A mistake there could survive every green CI run until a
# release build.
#
# It was not hypothetical. The silent-failure defect in shareImage was found by
# reading; the API floor it turns on (checkSelfPermission exists from 23, the
# app's minSdk is 21) was caught only by compiling and thinking, which is
# exactly what nothing was doing.
#
#   scripts/check-android-java.sh
#
# Output is thrown away; nothing is dexed, packaged or signed.
set -euo pipefail
cd "$(dirname "$0")/.."

# The same recipe scripts/build-android.sh uses at line ~194, so this gate
# accepts exactly what the shipped build accepts — a different --release or a
# different android.jar would let through code the real build rejects, or
# reject code it accepts.
TARGET_API=36
RELEASE=8

find_android_jar() {
  # Prefer the API the real build compiles against; fall back to the newest
  # available, saying which, because any platform jar catches a typo.
  for home in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}" "$HOME/Library/Android/sdk" "$HOME/android-sdk" /usr/local/lib/android/sdk; do
    [ -n "$home" ] || continue
    if [ -f "$home/platforms/android-$TARGET_API/android.jar" ]; then
      echo "$home/platforms/android-$TARGET_API/android.jar"; return
    fi
  done
  for home in "${ANDROID_HOME:-}" "${ANDROID_SDK_ROOT:-}" "$HOME/Library/Android/sdk" "$HOME/android-sdk" /usr/local/lib/android/sdk; do
    [ -n "$home" ] || continue
    latest="$(ls -1d "$home"/platforms/android-* 2>/dev/null | sort -V | tail -1 || true)"
    if [ -n "$latest" ] && [ -f "$latest/android.jar" ]; then echo "$latest/android.jar"; return; fi
  done
}

JAR="$(find_android_jar)"
if [ -z "$JAR" ]; then
  # A check that quietly skips is worse than no check, because it reads as a
  # pass. On a runner this is a hard failure; locally it says what to install.
  if [ -n "${CI:-}" ]; then
    echo "::error::no Android platform jar found; android/*.java would go uncompiled and this gate would prove nothing" >&2
    exit 1
  fi
  echo "no Android platform jar found (looked for platforms/android-$TARGET_API under ANDROID_HOME and the usual paths)." >&2
  echo "install it with: sdkmanager 'platforms;android-$TARGET_API'" >&2
  exit 2
fi

command -v javac >/dev/null || { echo "javac not on PATH" >&2; exit 2; }

out="$(mktemp -d)"
log="$(mktemp)"
trap 'rm -rf "$out" "$log"' EXIT

if javac --release "$RELEASE" -Xlint:-options -cp "$JAR" -d "$out" android/*.java >"$log" 2>&1; then
  classes="$(find "$out" -name '*.class' | wc -l | tr -d ' ')"
  # A compile that produced nothing is not a pass. The sources really do define
  # dozens of classes, so a near-zero count means the glob or the output
  # directory moved and this gate stopped looking at anything.
  if [ "$classes" -lt 10 ]; then
    echo "only $classes classes came out of android/*.java; this gate is no longer compiling what it thinks it is" >&2
    exit 1
  fi
  echo "android/*.java compiles ($classes classes, --release $RELEASE against $(basename "$(dirname "$JAR")"))"
else
  echo "android/*.java does NOT compile:" >&2
  cat "$log" >&2
  exit 1
fi
