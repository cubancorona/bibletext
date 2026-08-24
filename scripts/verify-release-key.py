#!/usr/bin/env python3
"""Prove that every packaged native library contains the release linker value."""

from pathlib import Path
import os
import plistlib
import sys
import zipfile


PREFIX = "-X=bibletext.bundledBibleKeyEnc="


def fail(message: str) -> None:
    raise SystemExit("ERROR: " + message)


if len(sys.argv) != 2:
    fail("usage: verify-release-key.py <apk-or-aab>")

flags = os.environ.get("BIBLETEXT_RELEASE_LDFLAGS", "")
if not flags.startswith(PREFIX):
    fail("release linker value is unavailable")
marker = flags[len(PREFIX):].encode("ascii", "strict")
if len(marker) < 16:
    fail("release linker value is malformed")

artifact = Path(sys.argv[1])
if not artifact.is_file():
    fail("release artifact does not exist")

payloads: list[tuple[str, bytes]] = []
if zipfile.is_zipfile(artifact):
    with zipfile.ZipFile(artifact) as archive:
        libraries = [
            name for name in archive.namelist()
            if name.endswith(".so") and (name.startswith("base/lib/") or name.startswith("lib/"))
        ]
        payloads.extend((name, archive.read(name)) for name in libraries)
        if not payloads:
            plists = [
                name for name in archive.namelist()
                if name.startswith("Payload/") and name.endswith(".app/Info.plist")
            ]
            for info_name in plists:
                info = plistlib.loads(archive.read(info_name))
                executable = info.get("CFBundleExecutable")
                if isinstance(executable, str) and executable:
                    binary_name = info_name.rsplit("/", 1)[0] + "/" + executable
                    if binary_name in archive.namelist():
                        payloads.append((binary_name, archive.read(binary_name)))
else:
    payloads.append((artifact.name, artifact.read_bytes()))

if not payloads:
    fail("release artifact contains no native payload")

missing = [name for name, data in payloads if marker not in data]

if missing:
    fail(f"release linker value is absent from {len(missing)} of {len(payloads)} native payloads")

print(f"Verified keyed release value in {len(payloads)} native payloads.")
