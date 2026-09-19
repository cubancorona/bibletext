#!/usr/bin/env bash
# Everything a packaged Linux tarball must be true of, for any architecture.
#
# WHY THIS IS A SCRIPT AND NOT INLINE YAML
#   These checks used to live inside the release workflow's amd64 job. Adding an
#   arm64 job meant either duplicating them — two copies that drift, and the
#   arm64 one silently stops checking something — or extracting them once. The
#   desktop-entry assertions in particular exist because a stock packager once
#   shipped `Exec=… %F`, which breaks every "Open in BibleText" link on Linux;
#   that must not be re-broken on one architecture only.
#
#   scripts/check-linux-package.sh <tarball> <glibc-floor>
#
# Reads BIBLE_KEY_LDFLAGS from the environment for the packaged-binary check.
set -euo pipefail
tarball="${1:?the packaged tarball}"
floor="${2:?the glibc floor, e.g. 2.35}"
here="$(cd "$(dirname "$0")/.." && pwd)"

scan_dir="$(mktemp -d)"
trap 'rm -rf "$scan_dir"' EXIT
tar -xJf "$tarball" -C "$scan_dir"

mapfile -t packaged_binaries < <(find "$scan_dir" -type f -path '*/bin/bibletext' -print)
if (( ${#packaged_binaries[@]} != 1 )); then
  echo "::error::Expected one packaged Linux executable, found ${#packaged_binaries[@]}"
  exit 1
fi

# The desktop entry must register the bibletext: scheme and be launched with the
# URL, or the site's "Open in BibleText" link opens nothing on Linux. The stock
# CLI's %F must never ship again.
entry="$(find "$scan_dir" -type f -name 'uk.co.bibletext.desktop' -print -quit)"
[ -n "$entry" ] || { echo "::error::no uk.co.bibletext.desktop in the Linux package"; exit 1; }
# THE EXECUTABLE'S NAME IS USER-VISIBLE, IN THREE PLACES AT ONCE, and the
# packager picks it for us: `fyne package` names the binary after the source
# directory, which is cmd/desktop. So the tarball installed /usr/local/bin/desktop
# -- a generic name in the reader's PATH that any other Fyne app packaged from a
# desktop/ directory overwrites, the command a reader must type to start the app,
# and the client name the sound server shows while narration plays (a Linux
# volume control read "desktop", never "BibleText"). The snap and the AppImage
# each rename the same executable; only the tarball shipped it raw. Asserted by
# name rather than derived, because a check that follows whatever the archive
# happens to contain cannot notice the name going generic again.
grep -q '^Exec=bibletext %u$' "$entry" || { echo "::error::desktop entry Exec line is not 'bibletext %u'"; cat "$entry"; exit 1; }
grep -q '^MimeType=x-scheme-handler/bibletext;' "$entry" || { echo "::error::desktop entry does not register x-scheme-handler/bibletext"; cat "$entry"; exit 1; }
grep -q '^Categories=Education;Spirituality;$' "$entry" || { echo "::error::desktop entry has no Categories"; cat "$entry"; exit 1; }
if grep -q '^Keywords=fyne;' "$entry"; then echo "::error::the packager's fallback Keywords shipped"; exit 1; fi

makefile="$(find "$scan_dir" -type f -name Makefile -print -quit)"
[ -n "$makefile" ] || { echo "::error::no Makefile in the Linux package"; exit 1; }
grep -q 'update-desktop-database' "$makefile" || { echo "::error::the Makefile does not refresh mimeinfo.cache"; exit 1; }

BIBLETEXT_RELEASE_LDFLAGS="${BIBLE_KEY_LDFLAGS:-}" "$here/scripts/verify-release-package.sh" "${packaged_binaries[0]}" "$scan_dir"
"$here/scripts/check-glibc-floor.sh" "${packaged_binaries[0]}" "$floor"

# THE TARBALL IS THE THING A READER ACTUALLY INSTALLS, AND NOTHING EVER RAN ITS
# INSTALLER. Every check above inspects files inside the archive; none of them
# proves `make install` works. DESTDIR keeps this to a temporary tree, so it
# needs no privileges and mutates nothing.
#
# It does NOT currently reach the update-desktop-database line: the icon defect
# below fails the target first, and make stops there. Once that is fixed this
# block will exercise the whole install, which is the point.
pkg_dir="$(dirname "$makefile")"
dest="$(mktemp -d)"
trap 'rm -rf "$scan_dir" "$dest"' EXIT
if ! make -C "$pkg_dir" install DESTDIR="$dest" PREFIX=/usr >"$dest/install.log" 2>&1; then
  echo "::error::the packaged Makefile's install target failed"
  cat "$dest/install.log"
  exit 1
fi

# The icon must land too. It did not until 19 September 2026: the Makefile named
# it without its .png, so install failed on that line AND never reached
# update-desktop-database — which meant a tarball install produced no
# mimeinfo.cache, and a bibletext: link opened nothing.
# Whatever the icon does, these must land, or a reader who installs the tarball
# has no application and no way for a bibletext: link to reach it.
exe="$(basename "${packaged_binaries[0]}")"
[ "$exe" = bibletext ] ||
  { echo "::error::the packaged executable is named '$exe', not 'bibletext' (see the note above)"; exit 1; }
for want in "$dest/usr/bin/$exe" \
             "$dest/usr/share/applications/uk.co.bibletext.desktop" \
             "$dest/usr/share/pixmaps/uk.co.bibletext.png" \
             "$dest/usr/share/applications/mimeinfo.cache"; do
  [ -e "$want" ] || { echo "::error::make install did not produce $want"; find "$dest" -type f | head -20; exit 1; }
done
# The INSTALLED entry — not the one in the archive — is what the desktop reads.
installed_entry="$dest/usr/share/applications/uk.co.bibletext.desktop"
grep -q '^MimeType=x-scheme-handler/bibletext;' "$installed_entry" ||
  { echo "::error::the INSTALLED desktop entry does not register the bibletext: scheme"; exit 1; }
grep -q "^Exec=$exe %u\$" "$installed_entry" ||
  { echo "::error::the INSTALLED desktop entry is not launched with the URL"; cat "$installed_entry"; exit 1; }

echo "linux package ok: $(basename "$tarball") (installs; the installed entry keeps the scheme and %u)"
