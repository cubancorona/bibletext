#!/usr/bin/env bash
# Regression coverage for the Linux scheme-handler patch to the vendored Fyne
# CLI (patches/fyne-tools-1.7.2-linux-scheme-handler.patch): regenerate the
# patched copy, run the patch's own tests, and hold cmd/bibletext/FyneApp.toml
# to the table that makes the Linux desktop entry register bibletext:.
set -euo pipefail

cd "$(dirname "$0")/.."

scripts/setup-fyne-tools-patch.sh >/dev/null

(cd third_party/fyne-tools && go test ./cmd/fyne/internal/commands -run '^TestSchemeHandler' -count=1)

if ! grep -q '^\[CanOpen\]$' cmd/bibletext/FyneApp.toml \
   || ! grep -q '^MimeTypes = "x-scheme-handler/bibletext;"$' cmd/bibletext/FyneApp.toml; then
  echo "ERROR: cmd/bibletext/FyneApp.toml no longer registers x-scheme-handler/bibletext under [CanOpen]" >&2
  exit 1
fi

echo "Linux scheme-handler regression tests passed."
