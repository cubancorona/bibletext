#!/usr/bin/env bash
# The tar.xz and the AppImage run on any glibc at or above the highest version
# the executable references. Ubuntu 22.04 (supported to May 2027) has 2.35, so
# that is the floor the build must not rise above; a runner image drifting to
# a newer distribution would raise it silently otherwise.
#
#   scripts/check-glibc-floor.sh <executable> [ceiling, default 2.35]
set -euo pipefail
bin="${1:?executable}"
ceiling="${2:-2.35}"
if objdump -T "$bin" | grep -q 'GLIBC_ABI_DT_RELR'; then
  echo "::error::$bin uses DT_RELR relocations, which need glibc 2.36; the linker must not pack relative relocations" >&2
  exit 1
fi
max="$(objdump -T "$bin" | grep -o 'GLIBC_[0-9.]*' | sort -uV | tail -1)"
[ -n "$max" ] || { echo "no GLIBC_ symbols found in $bin; the check itself is broken" >&2; exit 1; }
top="$(printf '%s\nGLIBC_%s\n' "$max" "$ceiling" | sort -V | tail -1)"
[ "$top" = "GLIBC_$ceiling" ] || { echo "::error::$bin needs $max, above the $ceiling floor" >&2; exit 1; }
echo "glibc floor: $max (<= $ceiling)"
