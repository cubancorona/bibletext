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
RELEASE_SCRIPT = ROOT / "scripts" / "release-mac-store.sh"

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


def patch_step_failures(script: str) -> list[str]:
    """The Mac store build must link the PATCHED Fyne, like the other platforms.

    go.mod ships stock, and each release script applies the local patches for
    its own build. This script did not, so Mac App Store builds linked stock
    Fyne — whose preferences writer truncates the file in place, meaning a
    death mid-write leaves an empty store that reads as a brand-new reader
    and loses every note. Nothing about the resulting build looks wrong, so
    the step is asserted here rather than left to memory.
    """
    failures = []
    if "setup-fyne-patch.sh" not in script:
        failures.append(
            "release script: the Fyne patches are never applied — a Mac store "
            "build would link the stock preferences writer, which truncates in "
            "place and can lose a reader's whole notes store on a torn write"
        )
    if "go mod edit -replace fyne.io/fyne/v2=./third_party/fyne" not in script:
        failures.append(
            "release script: go.mod is never pointed at third_party/fyne, so "
            "regenerating the patched tree would have no effect on the build"
        )
    if "Preferences save not published" not in script:
        failures.append(
            "release script: nothing verifies the built binary carries the "
            "atomic preferences writer; the patch step could silently no-op"
        )
    return failures


# The gates every release entry point runs, and the credentials none of them
# should hand to a subprocess. release-mac-store.sh had neither: it could ship
# from a tree CI would have refused, and it ran go, fyne, codesign and
# productbuild with whatever the operator had exported — which, in this
# project's own AI-testing workflow, is four provider keys.
REQUIRED_GATES = (
    ("check-support-contact.py", "the public support configuration"),
    ("check-repository-hygiene.py", "repository hygiene"),
)
SCRUBBED_ENV = ("ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY")


def supply_chain_failures(script: str) -> list[str]:
    failures = []
    for gate, what in REQUIRED_GATES:
        if gate not in script:
            failures.append(
                f"release script: {gate} is never run, so a store build could "
                f"ship from a tree whose {what} CI would have refused"
            )
    for var in SCRUBBED_ENV:
        if var not in script:
            failures.append(
                f"release script: {var} is never unset, so every build "
                "subprocess inherits it"
            )
    return failures


def release_script_failures(script: str) -> list[str]:
    """The Universal Links claim must be injected at signing time.

    com.apple.developer.associated-domains is a RESTRICTED entitlement: a
    build may only launch with it when a provisioning profile authorises it,
    and the dev-signed sandbox rehearsal embeds no profile. So the claim
    lives in release-mac-store.sh's signing-time entitlements rather than the
    tracked file — which means nothing plist-shaped guards it, and dropping
    the two lines would ship a store build whose clicked links silently open
    the browser again. This holds the script to it instead.
    """
    failures = []
    if "com.apple.developer.associated-domains" not in script:
        failures.append(
            "release script: com.apple.developer.associated-domains is never "
            "injected — the store build would carry no Universal Links claim "
            "and clicked share links would open the browser, not the app"
        )
    if "applinks:$SITE_HOST" not in script:
        failures.append(
            "release script: the applinks value must be applinks:$SITE_HOST, "
            "derived from config/product.json's siteBase — a hand-written "
            "domain drifts when the identity file changes"
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
        ("missing applinks injection", release_script_failures(
            "codesign -f -s CERT --entitlements ents.plist APP"),
         "never injected"),
        ("missing hygiene gate", supply_chain_failures(
            "python3 scripts/check-support-contact.py\n"
            + "\n".join(SCRUBBED_ENV)),
         "check-repository-hygiene.py is never run"),
        ("unscrubbed provider key", supply_chain_failures(
            "".join(g for g, _ in REQUIRED_GATES)),
         "ANTHROPIC_API_KEY is never unset"),
        ("patch step absent", patch_step_failures(
            "codesign -f -s CERT --entitlements ents.plist APP"),
         "never applied"),
        ("replace directive absent", patch_step_failures(
            'scripts/setup-fyne-patch.sh\ngo build ./cmd/desktop'),
         "never pointed at"),
        ("patch verification absent", patch_step_failures(
            'scripts/setup-fyne-patch.sh\n'
            'go mod edit -replace fyne.io/fyne/v2=./third_party/fyne\n'),
         "nothing verifies"),
        ("hand-written domain", release_script_failures(
            'PlistBuddy -c "Add :com.apple.developer.associated-domains:0 '
            'string applinks:example.com"'),
         "applinks:$SITE_HOST"),
    ]
    for name, failures, fragment in checks:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: the {name!r} rule cannot fire")
    # A correct pair must pass clean.
    clean = entitlement_failures(
        {"com.apple.security.app-sandbox": True, "com.apple.security.network.client": True}
    ) + migration_failures(
        {"Move": ["${Library}/Preferences/fyne/bibletext"]}, "bibletext"
    ) + supply_chain_failures(
        "".join(g for g, _ in REQUIRED_GATES) + "\n" + "\n".join(SCRUBBED_ENV)
    ) + patch_step_failures(
        'scripts/setup-fyne-patch.sh\n'
        'go mod edit -replace fyne.io/fyne/v2=./third_party/fyne\n'
        'grep -qF "Preferences save not published"\n'
    ) + release_script_failures(
        'PlistBuddy -c "Add :com.apple.developer.associated-domains array"\n'
        'PlistBuddy -c "Add ... string applinks:$SITE_HOST"')
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
    try:
        script_text = RELEASE_SCRIPT.read_text(encoding="utf-8")
        failures += release_script_failures(script_text)
        failures += patch_step_failures(script_text)
        failures += supply_chain_failures(script_text)
    except OSError:
        failures.append("release script: scripts/release-mac-store.sh is unreadable")

    if failures:
        print("mac store config check failed:")
        for f in failures:
            print(f"  - {f}")
        return 1
    print(f"mac store config check passed (preferences key: {unique!r})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
