#!/usr/bin/env bash
# Refuse a release artifact built with the next-release switch (docs/NEXT.md).
#
#   scripts/verify-not-next.sh ARTIFACT...
#
# Each ARTIFACT is a Go executable or shared library, a universal macOS binary
# each of whose architectures is checked, or an .apk or .aab whose native
# libraries are each checked. Go records the build tags in every binary it
# links (`go version -m`), however they arrived: on the command line, from
# GOFLAGS in the environment, or from GOFLAGS saved with `go env -w`. The last
# two leave no trace in any script, which is why the release paths check what
# they built as well as next_release_guard_test.go checking their text.
#
# A universal binary is one executable per architecture, built separately and
# joined by lipo, so one slice can carry the tag while another does not; and
# `go version -m` reads only the first. Each slice is therefore cut out and
# read on its own. That is done from the universal header here rather than
# with lipo, which only macOS has, so the guard test can prove it on any CI
# runner.
#
# It fails closed. An artifact with no readable Go build information is
# refused, and so is a package with no native library in it, or a universal
# header that cannot be read whole: a check that cannot see the tags has not
# shown they are absent.
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
ensure_scratch() {
  if [ -z "$scratch" ]; then
    scratch="$(mktemp -d "${TMPDIR:-/tmp}/bibletext-not-next.XXXXXX")"
  fi
}

# be_uint <file> <offset> <bytes>: the big-endian unsigned integer stored
# there. Fails when the file ends first.
be_uint() {
  od -An -tu1 -j "$2" -N "$3" "$1" | awk -v want="$3" '
    { for (i = 1; i <= NF; i++) { v = v * 256 + $i; n++ } }
    END { if (n != want) exit 1; printf "%.0f\n", v }'
}

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

# check_universal <file> <label> <magic>: each architecture of a universal
# (fat) Mach-O binary. The header is big-endian: magic, a count, then per
# architecture cputype, cpusubtype, offset, size and align, with 32-bit offsets
# and sizes under cafebabe and 64-bit ones (and a reserved word) under cafebabf.
check_universal() {
  local file="$1" label="$2" magic="$3"
  local count entry width length i at cpu arch offset size
  count="$(be_uint "$file" 4 4)" ||
    fail "$label: its universal header cannot be read, so its build tags are unknown"
  if [ "$count" -lt 1 ] || [ "$count" -gt 32 ]; then
    fail "$label: a universal header naming $count architectures cannot be read, so its build tags are unknown"
  fi
  if [ "$magic" = cafebabf ]; then
    entry=32 width=8
  else
    entry=20 width=4
  fi
  length="$(wc -c <"$file" | tr -d '[:space:]')"
  ensure_scratch
  for ((i = 0; i < count; i++)); do
    at=$((8 + i * entry))
    cpu="$(be_uint "$file" "$at" 4)" &&
      offset="$(be_uint "$file" $((at + 8)) "$width")" &&
      size="$(be_uint "$file" $((at + 8 + width)) "$width")" ||
      fail "$label: its universal header cannot be read, so its build tags are unknown"
    case "$cpu" in
      16777223) arch=x86_64 ;;
      16777228) arch=arm64 ;;
      *) arch="cputype $cpu" ;;
    esac
    if [ "$size" -lt 1 ] || [ $((offset + size)) -gt "$length" ]; then
      fail "$label: its $arch slice lies outside the file, so its build tags are unknown"
    fi
    # head stops reading at the slice's end, so tail, still writing the slices
    # after it, may be stopped by SIGPIPE; what was cut is measured instead.
    { tail -c "+$((offset + 1))" "$file" 2>/dev/null || true; } | head -c "$size" >"$scratch/slice"
    if [ "$(wc -c <"$scratch/slice" | tr -d '[:space:]')" != "$size" ]; then
      fail "$label: its $arch slice cannot be read whole, so its build tags are unknown"
    fi
    check_binary "$scratch/slice" "$label ($arch slice)"
  done
}

# check_artifact <file> <label>: a universal binary slice by slice, anything
# else as one binary.
check_artifact() {
  local file="$1" label="$2" magic
  magic="$(od -An -tx1 -N 4 "$file" | tr -d '[:space:]')"
  case "$magic" in
    cafebabe|cafebabf) check_universal "$file" "$label" "$magic" ;;
    *) check_binary "$file" "$label" ;;
  esac
}

for artifact in "$@"; do
  [ -f "$artifact" ] || fail "$artifact: no such file"
  case "$artifact" in
    *.apk|*.aab)
      ensure_scratch
      libraries="$(unzip -Z1 "$artifact" | grep -E '^(base/)?lib/.*\.so$' || true)"
      [ -n "$libraries" ] || fail "$artifact: no native library to check"
      while IFS= read -r library; do
        unzip -p "$artifact" "$library" >"$scratch/library.so"
        check_artifact "$scratch/library.so" "$artifact!$library"
      done <<<"$libraries"
      ;;
    *)
      check_artifact "$artifact" "$artifact"
      ;;
  esac
done
echo "Not a next build: ${*}"
