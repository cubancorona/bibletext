#!/usr/bin/env bash
# The headless launch every Linux package gets on the runner: start the app
# under Xvfb with software OpenGL, screenshot it after 30 s, then launch it
# again with a share link — the second process must hand the link to the
# first and exit, leaving exactly one instance. The launch command is the
# rest of the arguments, so one script serves the AppImage, the snap and the
# Flatpak.
#
#   scripts/smoke-linux-launch.sh <name> <command> [args…]
set -euo pipefail
name="${1:?a name for the screenshots}"
shift
[ $# -ge 1 ] || { echo "usage: smoke-linux-launch.sh <name> <command> [args…]" >&2; exit 2; }
link='bibletext://bibletext.co.uk/bsb/john/3'
export LIBGL_ALWAYS_SOFTWARE=1
xvfb-run -a -s "-screen 0 1280x800x24" bash -c '
  set -euo pipefail
  name="$1"; link="$2"; shift 2
  "$@" &
  first=$!
  sleep 30
  import -window root "smoke-$name-launch.png"
  kill -0 "$first" || { echo "::error::$name: the app exited within 30 s"; exit 1; }
  if ! timeout 20 "$@" "$link"; then
    echo "::error::$name: the second launch did not exit cleanly (it should hand the link off and leave)"; exit 1
  fi
  sleep 3
  import -window root "smoke-$name-link.png"
  n="$(pgrep -cx bibletext || true)"
  [ "$n" = 1 ] || { echo "::error::$name: $n bibletext processes after the second launch, want 1"; exit 1; }
  kill "$first"
' smoke "$name" "$link" "$@"
echo "$name: launched, handed off, one instance"
