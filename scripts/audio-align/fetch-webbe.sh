#!/usr/bin/env bash
# Fetch the WEBBE chapter MP3s the WEB-Catholic needs, into $DATA/webbe/mp3 where
# batch_align.py --audio webbe expects them.
#
# These are the chapters the David Williams WEB narration cannot serve: the seven
# deuterocanonical books, the GREEK Esther (ESG — a different book from the Hebrew
# Esther he read), and the Greek Daniel's chapters 3, 13 and 14 (Daniel 1-2 and
# 4-12 are the same text Williams read and keep his human narration). 150 files,
# ~150 MB, public domain, from eBible.org's World English Bible British Edition
# with Deuterocanon. Idempotent: an existing non-empty file is left alone.
#
# The numeric prefixes and three-letter codes are eBible's own, and its audio
# filenames call the Greek Daniel DAG (its verse-per-line text export calls the
# same book DNG — do not "fix" one to match the other).
set -uo pipefail
DATA="${BIBLETEXT_AUDIO_DATA:-$HOME/Dev/bibletext-audiodata}"
BASE="https://ebible.org/eng-webbe/mp3"
DEST="$DATA/webbe/mp3"
mkdir -p "$DEST"

urls() {
  emit() { # CODE order chapters...
    local code="$1" pre="$2"; shift 2
    for ch in "$@"; do printf '%s/eng-webbe_%s_%s_%02d.mp3\n' "$BASE" "$pre" "$code" "$ch"; done
  }
  emit TOB 041 $(seq 1 14)
  emit JDT 042 $(seq 1 16)
  emit ESG 043 $(seq 1 10)
  emit WIS 045 $(seq 1 19)
  emit SIR 046 $(seq 1 51)
  emit BAR 047 $(seq 1 6)
  emit 1MA 052 $(seq 1 16)
  emit 2MA 053 $(seq 1 15)
  emit DAG 066 3 13 14
}

urls > "$DATA/webbe/urls.txt"
# -f so a 404 is a hard failure: a missing chapter must never masquerade as a
# fetched one, because the timing table built from these files is what tells the
# app a recording exists.
xargs -P 6 -n 1 -I{} sh -c 'u="$1"; f="'"$DEST"'/$(basename "$u")"; [ -s "$f" ] || curl -fsS --retry 3 --retry-delay 2 -o "$f" "$u" || echo "FAILED $u"' _ {} < "$DATA/webbe/urls.txt"

want=$(wc -l < "$DATA/webbe/urls.txt")
got=$(ls -1 "$DEST"/*.mp3 2>/dev/null | wc -l)
echo "webbe: $got of $want files in $DEST"
[ "$got" -eq "$want" ] || { echo "INCOMPLETE — rerun; alignment must not start short" >&2; exit 1; }
