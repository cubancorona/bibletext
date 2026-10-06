#!/usr/bin/env python3
"""Check the device families an iOS app bundle declares.

    scripts/check-ios-device-family.py <App.app or Info.plist> [--iphone-only]

UIDeviceFamily is the one key that makes the app universal: 1 is iPhone and 2
is iPad. The App Store build has shipped for both since 1.1.0, App Review does
not let an update drop a device family it shipped, and nothing else in the
build says when the key is lost: Fyne's packager writes its own Info.plist,
and release-ios.sh sets the key afterwards. So the archived and exported
bundles are read back here, and must declare exactly iPhone and iPad, or
iPhone alone with --iphone-only (BIBLETEXT_IPAD=0, a local experiment).

Read with plistlib, so it reads the XML and binary forms alike and runs on any
host, not only where plutil and PlistBuddy are.
"""

from __future__ import annotations

import os
import plistlib
import sys

NAMES = {1: "iPhone", 2: "iPad", 3: "Apple TV", 4: "Apple Watch", 6: "Mac", 7: "Apple Vision"}


def declared(path: str):
    plist = os.path.join(path, "Info.plist") if os.path.isdir(path) else path
    with open(plist, "rb") as f:
        info = plistlib.load(f)
    return plist, info.get("UIDeviceFamily")


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--iphone-only"]
    if len(args) != 1:
        print("usage: check-ios-device-family.py <App.app or Info.plist> [--iphone-only]",
              file=sys.stderr)
        return 2
    want = [1] if "--iphone-only" in argv[1:] else [1, 2]
    try:
        plist, families = declared(args[0])
    except (OSError, plistlib.InvalidFileException, ValueError) as error:
        print(f"ERROR: cannot read the bundle's Info.plist: {error}", file=sys.stderr)
        return 1
    named = lambda fs: " and ".join(NAMES.get(f, str(f)) for f in fs)  # noqa: E731
    if (not isinstance(families, list) or not families
            or not all(isinstance(f, int) and not isinstance(f, bool) for f in families)):
        print(f"ERROR: {plist} declares no UIDeviceFamily the App Store can read "
              f"({families!r}); want {named(want)}", file=sys.stderr)
        return 1
    if sorted(set(families)) != want or len(families) != len(set(families)):
        print(f"ERROR: {plist} declares UIDeviceFamily {families} ({named(families)}); "
              f"want {want} ({named(want)})", file=sys.stderr)
        return 1
    print(f"device families: {named(families)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
