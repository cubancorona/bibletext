#!/usr/bin/env bash
# Merge BibleText's release-only linker value into Fyne's explicit mobile
# `go build -ldflags` argument. Fyne otherwise replaces GOFLAGS' ldflags.
set -euo pipefail

case "$-" in
  *x*)
    echo "ERROR: disable shell tracing before the keyed Go build." >&2
    exit 2
    ;;
esac

real_go="${BIBLETEXT_REAL_GO:?missing BIBLETEXT_REAL_GO}"
release_flags="${BIBLETEXT_RELEASE_LDFLAGS:-}"
unset BIBLETEXT_RELEASE_LDFLAGS

if [ -z "$release_flags" ]; then
  exec "$real_go" "$@"
fi

args=("$@")
is_shared=0
has_trimpath=0
for arg in "${args[@]}"; do
  if [ "$arg" = "-buildmode=c-shared" ]; then
    is_shared=1
  fi
  case "$arg" in
    -trimpath|-trimpath=true) has_trimpath=1 ;;
    -trimpath=false) has_trimpath=0 ;;
  esac
done

if [ "$is_shared" = 1 ]; then
  if [ "$has_trimpath" != 1 ]; then
    echo "ERROR: keyed Android shared-library builds require -trimpath." >&2
    exit 2
  fi
  merged=0
  for ((i = 0; i < ${#args[@]}; i++)); do
    case "${args[$i]}" in
      -ldflags)
        if ((i + 1 >= ${#args[@]})); then
          echo "ERROR: go received -ldflags without a value." >&2
          exit 2
        fi
        args[$((i + 1))]="${args[$((i + 1))]} $release_flags"
        merged=1
        break
        ;;
      -ldflags=*)
        args[$i]="${args[$i]} $release_flags"
        merged=1
        break
        ;;
    esac
  done
  if [ "$merged" != 1 ]; then
    echo "ERROR: Fyne's Android shared-library build supplied no linker flags." >&2
    exit 2
  fi
fi

exec "$real_go" "${args[@]}"
