#!/usr/bin/env bash
# Wrap the keyed, verified Linux executable in an AppDir and squash it with
# appimagetool on the static type-2 runtime (readers' machines need no
# libfuse2). The desktop entry, icon and MetaInfo are the rendered files under
# linux/ (cmd/linuxmeta); nothing else is bundled: the executable's shared
# libraries (libGL, libasound, libX11, glibc) are the host's on every
# distribution, and the X extension libraries GLFW loads at run time are on
# the same list.
#
#   scripts/build-appimage.sh <keyed desktop executable> [out.AppImage]
#
# The output name is constant (BibleText-x86_64.AppImage) because the site
# links /releases/latest/download/ by name; the version travels inside as
# X-AppImage-Version. VERSION is deliberately unset for appimagetool, which
# would otherwise put it in the file name.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN="${1:?keyed desktop executable}"
OUT="${2:-BibleText-x86_64.AppImage}"
ID=uk.co.bibletext.BibleText
LEDGER_VERSION="$(sed -n 's/^Version = "\(.*\)"/\1/p' cmd/desktop/FyneApp.toml)"
[ -n "$LEDGER_VERSION" ] || { echo "no Version in cmd/desktop/FyneApp.toml" >&2; exit 1; }
APPDIR=build/BibleText.AppDir
TOOLS=build/appimage-tools
# Pinned tool releases, checked once and recorded here (sha256 of the assets).
APPIMAGETOOL_TAG=1.9.1
APPIMAGETOOL_SHA256=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
RUNTIME_TAG=20251108
RUNTIME_SHA256=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d
UPDATE='gh-releases-zsync|cubancorona|bibletext|latest|BibleText-*x86_64.AppImage.zsync'

case "$OUT" in */*) ;; *) OUT="./$OUT" ;; esac

rm -rf "$APPDIR"
install -Dm755 "$BIN" "$APPDIR/usr/bin/bibletext"
# AppRun as a script that execs the real binary, so the process is named
# bibletext (an AppRun symlink would leave it named AppRun).
cat > "$APPDIR/AppRun" <<'RUN'
#!/bin/sh
here="$(dirname "$(readlink -f "$0")")"
exec "$here/usr/bin/bibletext" "$@"
RUN
chmod 755 "$APPDIR/AppRun"
{ cat "linux/$ID.desktop"; echo "X-AppImage-Version=$LEDGER_VERSION"; } > "$APPDIR/$ID.desktop"
install -Dm644 "$APPDIR/$ID.desktop" "$APPDIR/usr/share/applications/$ID.desktop"
install -Dm644 "linux/icons/hicolor/256x256/apps/$ID.png" "$APPDIR/$ID.png"
ln -s "$ID.png" "$APPDIR/.DirIcon"
for s in 256x256 512x512; do
  install -Dm644 "linux/icons/hicolor/$s/apps/$ID.png" "$APPDIR/usr/share/icons/hicolor/$s/apps/$ID.png"
done
# appimagetool looks for the metainfo under this older name only.
install -Dm644 "linux/$ID.metainfo.xml" "$APPDIR/usr/share/metainfo/$ID.appdata.xml"
install -Dm644 LICENSE "$APPDIR/usr/share/doc/bibletext/LICENSE"
install -Dm644 NOTICE "$APPDIR/usr/share/doc/bibletext/NOTICE"
install -Dm644 patches/NotoColorEmoji-LICENSE-OFL.txt "$APPDIR/usr/share/doc/bibletext/OFL-NotoColorEmoji.txt"
for f in assets/fonts/atkinson/OFL.txt assets/fonts/reading/Junicode-OFL.txt assets/fonts/reading/EzraSIL-Licenses.txt assets/fonts/share/OFL-LICENSES.txt; do
  install -Dm644 "$f" "$APPDIR/usr/share/doc/bibletext/$(basename "$f")"
done

desktop-file-validate "$APPDIR/$ID.desktop"
appstreamcli validate --no-net "$APPDIR/usr/share/metainfo/$ID.appdata.xml"

mkdir -p "$TOOLS"
curl -fsSL -o "$TOOLS/appimagetool" "https://github.com/AppImage/appimagetool/releases/download/$APPIMAGETOOL_TAG/appimagetool-x86_64.AppImage"
echo "$APPIMAGETOOL_SHA256  $TOOLS/appimagetool" | sha256sum -c -
curl -fsSL -o "$TOOLS/runtime-x86_64" "https://github.com/AppImage/type2-runtime/releases/download/$RUNTIME_TAG/runtime-x86_64"
echo "$RUNTIME_SHA256  $TOOLS/runtime-x86_64" | sha256sum -c -
chmod +x "$TOOLS/appimagetool"

env -u VERSION ARCH=x86_64 "$TOOLS/appimagetool" --appimage-extract-and-run \
  --runtime-file "$TOOLS/runtime-x86_64" -u "$UPDATE" "$APPDIR" "$OUT"
[ -s "$OUT.zsync" ] || { echo "no $OUT.zsync was written (is zsync installed?)" >&2; exit 1; }
# The runtime answers this from its own ELF section, with no mount and no
# extraction; putting --appimage-extract-and-run first would hand the
# question to the app instead.
got="$("$OUT" --appimage-updateinformation 2>/dev/null || true)"
[ "$got" = "$UPDATE" ] || { echo "update information mismatch: got '$got'" >&2; exit 1; }
echo "built $OUT ($LEDGER_VERSION)"
