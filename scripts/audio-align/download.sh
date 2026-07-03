#!/usr/bin/env bash
# Download all recorded Bible narration (BSB complete Hays set + WEB eBible subset)
# for the read-along forced-alignment pipeline. Idempotent + resumable: files already
# present are skipped, so re-running fills only the gaps. The audio lives OUTSIDE the
# git repo (BIBLETEXT_AUDIO_DATA); nothing downloaded is committed.
#
#   BIBLETEXT_AUDIO_DATA/manifest.tsv   ver<TAB>book<TAB>chapter<TAB>url   (generated)
#   BIBLETEXT_AUDIO_DATA/{bsb,web}/*.mp3
set -uo pipefail
DATA="${BIBLETEXT_AUDIO_DATA:-$HOME/Dev/bibletext-audiodata}"
MAN="$DATA/manifest.tsv"
JOBS="${JOBS:-4}"
[ -f "$MAN" ] || { echo "manifest not found: $MAN" >&2; exit 1; }
mkdir -p "$DATA/bsb" "$DATA/web"

fetch_url() {
  local url="$1" fn out ver
  case "$url" in
    *//openbible.com/*) ver=bsb ;;
    *//ebible.org/*)    ver=web ;;
    *)                  ver=other ;;
  esac
  fn="${url##*/}"; out="$DATA/$ver/$fn"
  if [ -s "$out" ]; then printf 'skip %s\n' "$fn"; return 0; fi
  if curl -fsS --retry 4 --retry-delay 2 --max-time 180 -o "$out.part" "$url"; then
    mv "$out.part" "$out"; printf 'ok   %s\n' "$fn"
  else
    rm -f "$out.part"; printf 'FAIL %s\n' "$url"; printf '%s\n' "$url" >>"$DATA/failures.log"
  fi
}
export -f fetch_url; export DATA

: > "$DATA/failures.log"
awk -F'\t' '{print $4}' "$MAN" | xargs -P "$JOBS" -I{} bash -c 'fetch_url "$1"' _ {}
echo "=== done: $(ls "$DATA/bsb" | wc -l | tr -d ' ') bsb + $(ls "$DATA/web" 2>/dev/null | wc -l | tr -d ' ') web files; failures: $(wc -l < "$DATA/failures.log" | tr -d ' ') ==="
