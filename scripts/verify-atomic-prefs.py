#!/usr/bin/env python3
"""Refuse a release artifact built without the atomic preferences writer.

    scripts/verify-atomic-prefs.py ARTIFACT...

Stock Fyne writes preferences.json by truncating it in place and then
encoding into it, so a death in between (a crash, a force quit, the kill a
store update performs) leaves the file empty, and an empty store reads as a
new reader: every note gone, including notes other people shared, which exist
nowhere else. patches/fyne-2.7.4-atomic-prefs.patch publishes by rename
instead. It changes Fyne's app/preferences_nonweb.go, which every platform
but the web builds, and it reaches a build only when that build applies the
patch (scripts/setup-fyne-patch.sh and the go.mod replace). This checks what
was built rather than trusting that the step ran: each binary must carry the
patched writer's log message.

A control string every build carries proves the search can find anything at
all, so a missing marker means a stock writer rather than an unreadable file.

Each ARTIFACT is an executable or shared library, a universal macOS binary
each of whose architectures is checked on its own, or an .apk or .aab each of
whose native libraries is checked. It fails closed: a package with no native
library, or a universal header that cannot be read whole, is refused.
"""

from __future__ import annotations

import struct
import sys
import zipfile

# The patched writer's fyne.LogError text when a save is not published.
MARKER = b"Preferences save not published"
# A string every build of the app carries (the default translation's name).
CONTROL = b"World English Bible"

FAT_MAGIC = 0xCAFEBABE
FAT_MAGIC_64 = 0xCAFEBABF
# More slices than any universal binary has; a Java class file shares the
# magic, and its version fields read as a slice count far above this.
MAX_SLICES = 16


class Refused(Exception):
    pass


def slices(data: bytes, label: str):
    """(label, bytes) for each architecture of a universal binary, or the whole."""
    if len(data) < 8:
        return [(label, data)]
    magic, count = struct.unpack(">II", data[:8])
    if magic not in (FAT_MAGIC, FAT_MAGIC_64):
        return [(label, data)]
    if not 0 < count <= MAX_SLICES:
        raise Refused(f"{label}: a universal header naming {count} slices cannot be read")
    entry = 32 if magic == FAT_MAGIC_64 else 20
    out = []
    for i in range(count):
        at = 8 + i * entry
        if at + entry > len(data):
            raise Refused(f"{label}: the universal header ends before slice {i + 1}")
        if magic == FAT_MAGIC_64:
            cpu, _sub, offset, size = struct.unpack(">iiQQ", data[at:at + 24])
        else:
            cpu, _sub, offset, size = struct.unpack(">iiII", data[at:at + 16])
        if offset + size > len(data) or size == 0:
            raise Refused(f"{label}: slice {i + 1} lies outside the file")
        out.append((f"{label} (cputype {cpu})", data[offset:offset + size]))
    return out


def binaries(path: str):
    """(label, bytes) for every binary the artifact holds."""
    if path.endswith((".apk", ".aab")):
        with zipfile.ZipFile(path) as archive:
            names = [n for n in archive.namelist()
                     if n.endswith(".so") and (n.startswith("lib/") or n.startswith("base/lib/"))]
            if not names:
                raise Refused(f"{path}: no native library to check")
            found = []
            for name in names:
                found.extend(slices(archive.read(name), f"{path}!{name}"))
            return found
    with open(path, "rb") as f:
        return slices(f.read(), path)


def check(path: str) -> list[str]:
    """The labels of the binaries checked; Refused at the first that fails."""
    checked = []
    for label, data in binaries(path):
        if CONTROL not in data:
            raise Refused(f"{label}: carries no {CONTROL.decode()!r}, a string every build has; "
                          "the marker search cannot be trusted")
        if MARKER not in data:
            raise Refused(f"{label}: lacks the atomic preferences writer; the Fyne patch "
                          "(patches/fyne-2.7.4-atomic-prefs.patch) did not reach this build")
        checked.append(label)
    return checked


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("usage: verify-atomic-prefs.py ARTIFACT...", file=sys.stderr)
        return 2
    try:
        checked = []
        for path in argv[1:]:
            checked.extend(check(path))
    except Refused as refusal:
        print(f"ERROR: {refusal}", file=sys.stderr)
        return 1
    except (OSError, zipfile.BadZipFile) as error:
        print(f"ERROR: cannot read {error}", file=sys.stderr)
        return 1
    print(f"atomic preferences writer present: {', '.join(checked)}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
