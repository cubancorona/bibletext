#!/usr/bin/env bash
# Builds cmd/bibletext/BibleText.exe for the Microsoft Store package exactly the
# way .github/workflows/release.yml's Windows job builds the release zip: the
# bundled NKJV key comes from BIBLETEXT_BUNDLED_KEY_ENC, the binary is
# trimmed and stripped, `fyne package` adds the icon and version resources,
# and the release-package verifier checks the result. The command lines here
# are held identical to the release job's by scripts/test-release-key-flow.sh,
# so the two channels cannot drift apart.
#
# THE TAG APPEARS TWICE ON PURPOSE. `gles` selects the toolkit's OpenGL ES
# path, which renders through Direct3D by way of ANGLE (scripts/fetch-angle.ps1)
# and therefore runs on a machine with no graphics driver. The packager
# REBUILDS the executable to add its icon and version resources, so a tag on
# the first line alone would be discarded by the second and the shipped
# binary would quietly revert to desktop OpenGL. Runs on the windows-latest runner
# (bash, Go, the fyne CLI under GOPATH/bin).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

unset BIBLE_API_KEY ANTHROPIC_API_KEY OPENAI_API_KEY GEMINI_API_KEY XAI_API_KEY
# shellcheck source=scripts/release-bible-key.sh
source scripts/release-bible-key.sh
load_encoded_release_bible_key
trap clear_release_bible_key EXIT

cd cmd/bibletext
# EXPORTED, not a command prefix: `fyne package` below rebuilds the
# executable to add its icon and version resources, and a prefix assignment
# would not reach it — the rebuild would silently target the host.
export GOARCH="${WIN_GOARCH:-amd64}"
CGO_ENABLED=1 go build -tags gles -trimpath -ldflags="$BIBLE_KEY_LDFLAGS -s -w" -o BibleText.exe .
# Fyne's Windows packager rebuilds the target to add icon/version
# resources. GOFLAGS keeps that metadata pass trimmed and stripped.
GOFLAGS="-trimpath -ldflags=-s -ldflags=-w -ldflags=$BIBLE_KEY_LDFLAGS" "$(go env GOPATH)/bin/fyne" package -os windows --tags gles --app-id uk.co.bibletext --executable BibleText.exe
BIBLETEXT_RELEASE_LDFLAGS="$BIBLE_KEY_LDFLAGS" ../../scripts/verify-release-package.sh BibleText.exe BibleText.exe
