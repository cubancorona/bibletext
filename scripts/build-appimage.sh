#!/usr/bin/env bash
# Wrap the keyed, verified Linux executable in an AppDir and squash it with
# appimagetool on the static type-2 runtime (readers' machines need no
# libfuse2). The desktop entry, icon and MetaInfo are the rendered files under
# linux/ (cmd/linuxmeta); nothing else is bundled: the executable's shared
# libraries (libGL, libasound, libX11, glibc) are the host's on every
# distribution, and the X extension libraries GLFW loads at run time are on
# the same list.
#
#   scripts/build-appimage.sh <keyed desktop executable> [out.AppImage] [arch]
#
# The output name is constant per architecture (BibleText-x86_64.AppImage,
# BibleText-aarch64.AppImage) because the site links /releases/latest/download/
# by name; the version travels inside as X-AppImage-Version. VERSION is
# deliberately unset for appimagetool, which would otherwise put it in the file
# name.
#
# ARCHITECTURE. appimagetool RUNS on the build host, so its own binary must
# match that host, while the runtime it embeds must match the executable being
# wrapped. On a native runner those are the same thing, which is why the
# default is the host's own machine type rather than a fixed value: a wrong
# default here would produce an AppImage that launches on the build machine and
# nowhere else. Both are pinned by sha256, and the update-information string
# carries the architecture too, or every architecture would advertise the same
# zsync file and readers would be offered the wrong one.
set -euo pipefail
cd "$(dirname "$0")/.."

BIN="${1:?keyed desktop executable}"
# Linux-only, and the guard is not pedantry: macOS reports arm64 for uname -m,
# which would otherwise satisfy the host/target check below and then fail
# obscurely several steps later on a Linux-only tool.
[ "$(uname -s)" = "Linux" ] || { echo "AppImages are built on Linux; this is $(uname -s)" >&2; exit 2; }
case "$(uname -m)" in
  x86_64)          HOST_ARCH=x86_64 ;;
  aarch64|arm64)   HOST_ARCH=aarch64 ;;
  *) echo "unsupported build host $(uname -m)" >&2; exit 2 ;;
esac
ARCH="${3:-$HOST_ARCH}"
OUT="${2:-BibleText-$ARCH.AppImage}"
ID=uk.co.bibletext.BibleText
LEDGER_VERSION="$(sed -n 's/^Version = "\(.*\)"/\1/p' cmd/desktop/FyneApp.toml)"
[ -n "$LEDGER_VERSION" ] || { echo "no Version in cmd/desktop/FyneApp.toml" >&2; exit 1; }
APPDIR=build/BibleText.AppDir
TOOLS=build/appimage-tools
# Pinned tool releases, checked once and recorded here (sha256 of the assets).
APPIMAGETOOL_TAG=1.9.1
RUNTIME_TAG=20251108
case "$ARCH" in
  x86_64)
    APPIMAGETOOL_SHA256=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
    RUNTIME_SHA256=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d ;;
  aarch64)
    APPIMAGETOOL_SHA256=f0837e7448a0c1e4e650a93bb3e85802546e60654ef287576f46c71c126a9158
    RUNTIME_SHA256=00cbdfcf917cc6c0ff6d3347d59e0ca1f7f45a6df1a428a0d6d8a78664d87444 ;;
  *) echo "no pinned AppImage tools for $ARCH" >&2; exit 2 ;;
esac
[ "$ARCH" = "$HOST_ARCH" ] || {
  echo "appimagetool runs on this host ($HOST_ARCH) and cannot target $ARCH" >&2; exit 2; }
UPDATE="gh-releases-zsync|cubancorona|bibletext|latest|BibleText-*$ARCH.AppImage.zsync"

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
curl -fsSL -o "$TOOLS/appimagetool" "https://github.com/AppImage/appimagetool/releases/download/$APPIMAGETOOL_TAG/appimagetool-$ARCH.AppImage"
echo "$APPIMAGETOOL_SHA256  $TOOLS/appimagetool" | sha256sum -c -
curl -fsSL -o "$TOOLS/runtime-$ARCH" "https://github.com/AppImage/type2-runtime/releases/download/$RUNTIME_TAG/runtime-$ARCH"
echo "$RUNTIME_SHA256  $TOOLS/runtime-$ARCH" | sha256sum -c -
chmod +x "$TOOLS/appimagetool"

env -u VERSION ARCH="$ARCH" "$TOOLS/appimagetool" --appimage-extract-and-run \
  --runtime-file "$TOOLS/runtime-$ARCH" -u "$UPDATE" "$APPDIR" "$OUT"
[ -s "$OUT.zsync" ] || { echo "no $OUT.zsync was written (is zsync installed?)" >&2; exit 1; }
# The runtime answers this from its own ELF section, with no mount and no
# extraction; putting --appimage-extract-and-run first would hand the
# question to the app instead.
got="$("$OUT" --appimage-updateinformation 2>/dev/null || true)"
[ "$got" = "$UPDATE" ] || { echo "update information mismatch: got '$got'" >&2; exit 1; }
echo "built $OUT ($LEDGER_VERSION, $ARCH)"
