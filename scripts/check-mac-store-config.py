#!/usr/bin/env python3
"""Hold the Mac App Store entitlements and migration manifest to the code.

Both files fail silently when wrong. Entitlements that omit a needed key give
a sandboxed app that launches and does nothing useful; a migration manifest
pointing at the wrong path copies nothing and an existing reader loses their
notes, with no error on either side and no second chance — macOS consults the
manifest only on the launch that creates the container.

So the checks here are not style checks. They assert that:

  * the entitlements request network access, which every download needs, and
    do NOT quietly acquire capabilities this app has no code path for;
  * the migration source path matches the identifier the app actually passes
    to NewWithID, which is NOT the bundle id — Fyne keys its preferences
    directory by that identifier, and a manifest written against the bundle id
    migrates nothing;
  * the migration carries the data with an operation macOS actually honours.
    That operation is Move, and only Move: a Copy entry is accepted by the
    plist and then ignored, which looks exactly like a reader who had no data.
    The cost is real and deliberate — Move takes the preferences, so a
    direct-download build still installed alongside opens as a new reader
    afterwards — and docs/MAC_APP_STORE.md records why that trade was taken.

Self-tests first, as everywhere else here: each rule is run against a
synthetic violation and must fail, because a checker that cannot fail proves
nothing when it passes.
"""

from __future__ import annotations

import plistlib
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ENTITLEMENTS = ROOT / "appstore" / "mac" / "BibleText.entitlements"
MIGRATION = ROOT / "appstore" / "mac" / "Container-Migration.plist"
APP_GO = ROOT / "app.go"

REQUIRED_ENTITLEMENTS = {
    "com.apple.security.app-sandbox": "the Mac App Store accepts sandboxed apps only",
    "com.apple.security.network.client": "every translation, audio and AI request",
}

# Capabilities no code path in this app uses. Acquiring one silently would
# widen the app's reach and invite a review question.
FORBIDDEN_ENTITLEMENT_PREFIXES = (
    "com.apple.security.network.server",
    "com.apple.security.files.user-selected",
    "com.apple.security.files.downloads",
    "com.apple.security.temporary-exception",
    "com.apple.security.device.camera",
    "com.apple.security.device.microphone",
    "com.apple.security.personal-information",
)


def app_unique_id(source: str) -> str | None:
    """The identifier the app passes to NewWithID, which keys the prefs dir."""
    m = re.search(r'app\.NewWithID\(\s*devAppID\(\s*"([^"]+)"\s*\)\s*\)', source)
    return m.group(1) if m else None


def entitlement_failures(data: dict) -> list[str]:
    failures = []
    for key, why in REQUIRED_ENTITLEMENTS.items():
        if data.get(key) is not True:
            failures.append(f"entitlements: {key} must be true — {why}")
    for key in data:
        if key.startswith(FORBIDDEN_ENTITLEMENT_PREFIXES):
            failures.append(
                f"entitlements: {key} is not needed by any code path here; "
                "add it deliberately or remove it"
            )
    return failures


# The real Container-Migration.plist schema, taken from the manifests Safari
# and OneDrive ship on macOS rather than from assumption: the OPERATION is the
# top-level key and holds an array whose elements are either a path string
# (same relative location inside the container) or a two-element [source,
# destination] array. Paths use ${Library} / ${ApplicationSupport}; ${HOME} is
# wrong, because under the sandbox HOME already points at the container.
#
# A manifest in any other shape is ignored in silence — which is exactly what
# an un-migrated reader looks like — so the shape is checked, not assumed.
MIGRATION_OPERATIONS = {"Move", "Copy", "Symlink", "Unlink"}
# Only Move is accepted for carrying the preferences across. Copy is in the
# schema but measured not to work: a sandboxed build whose manifest used Copy
# created its container and wrote a fresh empty preferences file while the
# source sat untouched. Move carried all eleven keys. Symlink and Unlink do
# not carry data at all.
CARRYING_OPERATIONS = {"Move"}


def migration_paths(data: dict) -> list[tuple[str, str]]:
    """[(operation, source path)] for every entry, whatever element shape."""
    out = []
    for op, entries in data.items():
        if not isinstance(entries, list):
            continue
        for entry in entries:
            if isinstance(entry, str):
                out.append((op, entry))
            elif isinstance(entry, list) and entry:
                out.append((op, str(entry[0])))
    return out


def migration_failures(data: dict, unique_id: str | None) -> list[str]:
    failures = []
    if not data:
        return ["migration: empty manifest; an updating reader would lose their notes"]

    unknown = set(data) - MIGRATION_OPERATIONS
    if unknown:
        failures.append(
            f"migration: top-level keys {sorted(unknown)} are not migration "
            f"operations; the schema is one of {sorted(MIGRATION_OPERATIONS)} as the "
            "top-level key holding an array of paths, and macOS ignores any other "
            "shape without reporting anything"
        )
    if unique_id is None:
        failures.append("migration: could not read the app's NewWithID identifier from app.go")

    entries = migration_paths(data)
    want_tail = "/fyne/" + unique_id if unique_id else "/fyne/"
    carriers = [
        (op, p) for op, p in entries
        if op in CARRYING_OPERATIONS and p.rstrip("/").endswith(want_tail)
    ]
    if not carriers:
        listed = [f"{op}:{p}" for op, p in entries] or ["(none)"]
        ops = " or ".join(sorted(CARRYING_OPERATIONS))
        failures.append(
            f"migration: no {ops} entry carries "
            f"{want_tail!r}, the preferences directory holding the notes "
            "scrapbook, reading position, settings and any saved keys. "
            f"Entries present: {listed}. Fyne keys that directory by the "
            "identifier passed to NewWithID, not by the bundle id."
        )
    for op, p in carriers:
        if "${HOME}" in p:
            failures.append(
                f"migration: {op} source {p!r} uses ${{HOME}}, which under the "
                "sandbox already points at the container; use ${Library}"
            )
        elif not p.startswith("${Library}"):
            failures.append(
                f"migration: {op} source {p!r} should start with ${{Library}} so it "
                "resolves against the real home directory"
            )
    return failures


def self_test() -> list[str]:
    problems = []
    checks = [
        ("missing network", entitlement_failures({"com.apple.security.app-sandbox": True}),
         "network.client"),
        ("missing sandbox", entitlement_failures({"com.apple.security.network.client": True}),
         "app-sandbox"),
        ("forbidden key", entitlement_failures(
            {"com.apple.security.app-sandbox": True,
             "com.apple.security.network.client": True,
             "com.apple.security.network.server": True}), "network.server"),
        ("empty manifest", migration_failures({}, "bibletext"), "lose their notes"),
        ("invented schema", migration_failures(
            {"Version": "1", "Migrations": [{"Source": "x", "Operation": "Copy"}]},
            "bibletext"), "not migration operations"),
        ("bundle-id source", migration_failures(
            {"Move": ["${Library}/Preferences/fyne/uk.co.bibletext.desktop"]},
            "bibletext"), "entry carries"),
        ("HOME variable", migration_failures(
            {"Move": ["${HOME}/Library/Preferences/fyne/bibletext"]},
            "bibletext"), "already points at the container"),
        ("symlink only", migration_failures(
            {"Symlink": ["${Library}/Preferences/fyne/bibletext"]},
            "bibletext"), "entry carries"),
        ("copy does not carry", migration_failures(
            {"Copy": ["${Library}/Preferences/fyne/bibletext"]},
            "bibletext"), "entry carries"),
    ]
    for name, failures, fragment in checks:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: the {name!r} rule cannot fire")
    # A correct pair must pass clean.
    clean = entitlement_failures(
        {"com.apple.security.app-sandbox": True, "com.apple.security.network.client": True}
    ) + migration_failures(
        {"Move": ["${Library}/Preferences/fyne/bibletext"]}, "bibletext")
    if clean:
        problems.append(f"self-test: a correct configuration still fails: {clean}")
    return problems


def main() -> int:
    if problems := self_test():
        print("mac store config checker self-test failed:")
        for p in problems:
            print(f"  - {p}")
        return 1

    failures: list[str] = []
    try:
        ents = plistlib.loads(ENTITLEMENTS.read_bytes())
    except (OSError, plistlib.InvalidFileException) as err:
        failures.append(f"entitlements: unreadable or not a plist ({err})")
        ents = {}
    try:
        mig = plistlib.loads(MIGRATION.read_bytes())
    except (OSError, plistlib.InvalidFileException) as err:
        failures.append(f"migration: unreadable or not a plist ({err})")
        mig = {}
    try:
        unique = app_unique_id(APP_GO.read_text(encoding="utf-8"))
    except OSError:
        failures.append("app.go: unreadable")
        unique = None

    failures += entitlement_failures(ents)
    failures += migration_failures(mig, unique)

    if failures:
        print("mac store config check failed:")
        for f in failures:
            print(f"  - {f}")
        return 1
    print(f"mac store config check passed (preferences key: {unique!r})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
