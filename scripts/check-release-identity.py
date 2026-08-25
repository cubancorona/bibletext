#!/usr/bin/env python3
"""Fail when tracked release identities disagree with each other or the tag."""

from __future__ import annotations

from pathlib import Path
import re
import sys


ROOT = Path(__file__).resolve().parent.parent
LEDGERS = (
    ROOT / "cmd/mobile/FyneApp.toml",
    ROOT / "cmd/desktop/FyneApp.toml",
)


def fail(message: str) -> None:
    raise SystemExit("release identity check failed: " + message)


def ledger(path: Path) -> tuple[str, int]:
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as error:
        fail(f"cannot read {path.relative_to(ROOT)}: {error}")
    version_match = re.search(r'^Version = "([0-9]+\.[0-9]+\.[0-9]+)"$', text, re.M)
    build_match = re.search(r"^Build = ([0-9]+)$", text, re.M)
    if version_match is None or build_match is None:
        fail(f"{path.relative_to(ROOT)} has an invalid Version or Build")
    build = int(build_match.group(1))
    if build <= 0:
        fail(f"{path.relative_to(ROOT)} has a non-positive Build")
    return version_match.group(1), build


mobile_version, mobile_build = ledger(LEDGERS[0])
desktop_version, desktop_build = ledger(LEDGERS[1])
if mobile_version != desktop_version:
    fail(
        "mobile and desktop versions differ "
        f"({mobile_version} versus {desktop_version})"
    )

review_notes = (ROOT / "appstore/review-notes.txt").read_text(encoding="utf-8")
if not review_notes.startswith(f"VERSION {mobile_version}\n"):
    fail("App Store review notes do not match the tracked application version")

if len(sys.argv) > 2:
    fail("usage: check-release-identity.py [vMAJOR.MINOR.PATCH]")
if len(sys.argv) == 2:
    expected_tag = "v" + mobile_version
    if sys.argv[1] != expected_tag:
        fail(f"tag {sys.argv[1]!r} does not match {expected_tag!r}")

print(
    "release identity check passed "
    f"(version {mobile_version}; mobile build {mobile_build}; "
    f"desktop build {desktop_build})"
)
