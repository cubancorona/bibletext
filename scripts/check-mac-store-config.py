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
  * the migration copies rather than moves, so the direct-download build the
    reader may still be using keeps its data.

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


def migration_failures(data: dict, unique_id: str | None) -> list[str]:
    failures = []
    migrations = data.get("Migrations")
    if not isinstance(migrations, list) or not migrations:
        return ["migration: no Migrations entries; an updating reader would lose their notes"]
    if unique_id is None:
        failures.append("migration: could not read the app's NewWithID identifier from app.go")
    prefs_rules = [
        m for m in migrations
        if isinstance(m, dict) and "Library/Preferences/fyne" in str(m.get("Source", ""))
    ]
    if not prefs_rules:
        failures.append(
            "migration: no rule copies the Fyne preferences directory, which holds "
            "the notes scrapbook, reading position, settings and any saved keys"
        )
    for rule in prefs_rules:
        source = str(rule.get("Source", ""))
        if unique_id and not source.rstrip("/").endswith("/" + unique_id):
            failures.append(
                f"migration: source {source!r} does not end in the app's own "
                f"identifier {unique_id!r} — Fyne keys the preferences directory by "
                "that identifier, not the bundle id, so this would migrate nothing"
            )
        if rule.get("Operation") != "Copy":
            failures.append(
                f"migration: Operation is {rule.get('Operation')!r}; use Copy so the "
                "direct-download build the reader may still run keeps its data"
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
        ("no migrations", migration_failures({}, "bibletext"), "lose their notes"),
        ("bundle-id source", migration_failures(
            {"Migrations": [{"Source": "${HOME}/Library/Preferences/fyne/uk.co.bibletext.desktop",
                             "Operation": "Copy"}]}, "bibletext"), "migrate nothing"),
        ("move not copy", migration_failures(
            {"Migrations": [{"Source": "${HOME}/Library/Preferences/fyne/bibletext",
                             "Operation": "Move"}]}, "bibletext"), "use Copy"),
    ]
    for name, failures, fragment in checks:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: the {name!r} rule cannot fire")
    # A correct pair must pass clean.
    clean = entitlement_failures(
        {"com.apple.security.app-sandbox": True, "com.apple.security.network.client": True}
    ) + migration_failures(
        {"Migrations": [{"Source": "${HOME}/Library/Preferences/fyne/bibletext",
                         "Operation": "Copy"}]}, "bibletext")
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
