#!/usr/bin/env bash
# Refuse a release artifact built with the next-release switch (docs/NEXT.md).
#
#   scripts/verify-not-next.sh ARTIFACT...
#
# Each ARTIFACT is a Go executable or shared library, or an .apk or .aab whose
# native libraries are each checked. Go records the build tags in every binary
# it links (`go version -m`), however they arrived: on the command line, from
# GOFLAGS in the environment, or from GOFLAGS saved with `go env -w`. The last
# two leave no trace in any script, which is why the release paths check what
# they built as well as next_release_guard_test.go checking their text.
#
# It fails closed. An artifact with no readable Go build information is
# refused, and so is a package with no native library in it: a check that
# cannot see the tags has not shown they are absent.
#
# BIBLETEXT_REAL_GO names the Go toolchain when `go` on PATH is not it (the
# Android build puts its linker wrapper there).
set -euo pipefail

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[ "$#" -gt 0 ] || fail "usage: verify-not-next.sh ARTIFACT..."

go_bin="${BIBLETEXT_REAL_GO:-go}"
scratch=""
cleanup() {
  if [ -n "$scratch" ]; then
    rm -rf "$scratch"
  fi
}
trap cleanup EXIT

# check_binary <file> <label>
check_binary() {
  local file="$1" label="$2" info tags
  if ! info="$("$go_bin" version -m "$file" 2>/dev/null)"; then
    fail "$label: no Go build information could be read, so its build tags are unknown"
  fi
  # Every Go binary records its target; a listing without it was not read.
  case "$info" in
    *$'\tbuild\tGOOS='*) ;;
    *) fail "$label: the Go build settings are missing, so its build tags are unknown" ;;
  esac
  # The build-settings line reads: <tab>build<tab>-tags=one,two
  tags="$(printf '%s\n' "$info" | sed -n 's/^[[:space:]]*build[[:space:]]*-tags=//p')"
  tags="${tags//$'\r'/}"
  case ",$tags," in
    *,next,*)
      fail "$label was built with the next tag (-tags=$tags). The next major release must never ship before its day; see docs/NEXT.md."
      ;;
  esac
}

for artifact in "$@"; do
  [ -f "$artifact" ] || fail "$artifact: no such file"
  case "$artifact" in
    *.apk|*.aab)
      if [ -z "$scratch" ]; then
        scratch="$(mktemp -d "${TMPDIR:-/tmp}/bibletext-not-next.XXXXXX")"
      fi
      libraries="$(unzip -Z1 "$artifact" | grep -E '^(base/)?lib/.*\.so$' || true)"
      [ -n "$libraries" ] || fail "$artifact: no native library to check"
      while IFS= read -r library; do
        unzip -p "$artifact" "$library" >"$scratch/library.so"
        check_binary "$scratch/library.so" "$artifact!$library"
      done <<<"$libraries"
      ;;
    *)
      check_binary "$artifact" "$artifact"
      ;;
  esac
done
echo "Not a next build: ${*}"
