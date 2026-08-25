#!/usr/bin/env bash
# Enforce trimpath and inject BibleText's release-only linker value into Fyne's
# mobile `go build` command. Fyne otherwise replaces GOFLAGS' flags, and its
# debug Android packager omits both -trimpath and -ldflags.
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
trimpath_state=0
for arg in "${args[@]}"; do
  if [ "$arg" = "-buildmode=c-shared" ]; then
    is_shared=1
  fi
  case "$arg" in
    -trimpath|-trimpath=true) trimpath_state=1 ;;
    -trimpath=false) trimpath_state=-1 ;;
  esac
done

if [ "$is_shared" = 1 ]; then
  if [ "$trimpath_state" = -1 ]; then
    echo "ERROR: keyed Android shared-library builds forbid -trimpath=false." >&2
    exit 2
  fi
  if [ "$trimpath_state" = 0 ]; then
    if [ "${args[0]:-}" != "build" ]; then
      echo "ERROR: cannot add -trimpath to an unrecognized shared-library command." >&2
      exit 2
    fi
    args=("${args[0]}" -trimpath "${args[@]:1}")
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
    if [ "${args[0]:-}" != "build" ]; then
      echo "ERROR: cannot add linker flags to an unrecognized shared-library command." >&2
      exit 2
    fi
    args=("${args[0]}" -ldflags "$release_flags" "${args[@]:1}")
  fi
fi

exec "$real_go" "${args[@]}"
