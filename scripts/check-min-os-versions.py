#!/usr/bin/env python3
"""Hold the declared Apple OS floors to reality, and every build to the declaration.

config/product.json carries the two floors the App Store listings display and
the built artifacts enforce: iosMinimumOSVersion and macMinimumOSVersion.
Neither value is a taste question, so both have a hard lower bound here:

  * iOS: App Store Connect refuses uploads whose MinimumOSVersion is below
    15.0 starting in Spring 2027, and has warned on every lower upload since
    2026 (warning 90068). A floor below 15.0 is a build that will one day
    simply stop being uploadable.
  * macOS: Go 1.24 dropped macOS 11 — a pure-Go binary from that toolchain is
    linked with LC_BUILD_VERSION minos 12.0, and macOS 11's loader refuses
    it. Worse, with cgo and no explicit -mmacosx-version-min the external
    linker stamps the BUILD MACHINE's SDK version instead: measured on an
    Xcode 26 Mac, the packaged app carried minos 26.0 while its Info.plist
    advertised 10.11, i.e. an app that claimed to run everywhere and would
    launch nowhere but the newest macOS.

That last failure is why the second half of this checker exists: every script
and workflow that compiles, links, or packages for an Apple platform must
derive its floor from the declaration — a version-min literal in one of those
files is exactly how the packager's 10.11 template default and 2020's iOS
13.0 shipped for years without anyone having chosen them. The release scripts
then read the floor back out of the built artifact (Info.plist and the
binary's minos), so the declaration, the compile, and the upload cannot
disagree silently.

Like every checker in this repository, it self-tests first: each rule is run
against a synthetic violation and must fail, because a checker that cannot
fail proves nothing when it passes.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PRODUCT = ROOT / "config" / "product.json"

IOS_KEY = "iosMinimumOSVersion"
MAC_KEY = "macMinimumOSVersion"

# key -> (hard lower bound, printable bound, why the bound exists)
FLOOR_BOUNDS = {
    IOS_KEY: (
        (15, 0),
        "15.0",
        "App Store Connect refuses uploads below iOS 15.0 from Spring 2027 "
        "(warning 90068 on every earlier upload)",
    ),
    MAC_KEY: (
        (12, 0),
        "12.0",
        "Go 1.24+ does not support macOS 11; its binaries are linked with "
        "minos 12.0 and older loaders refuse them",
    ),
}

# Files that compile, link, or package for an Apple platform. Each must read
# its floor from config/product.json, must not carry a version-min literal,
# and — where it assembles a bundle — must stamp the floor into the plist.
# file -> (config key, required plist stamp or None)
CONSUMERS = {
    "scripts/release-ios.sh": (IOS_KEY, ":MinimumOSVersion $IOS_MIN"),
    "scripts/run-ios-device.sh": (IOS_KEY, ":MinimumOSVersion $IOS_MIN"),
    "scripts/run-ios-sim.sh": (IOS_KEY, None),
    "scripts/check-ios-pane.sh": (IOS_KEY, None),
    ".github/workflows/ci.yml": (IOS_KEY, None),
    "scripts/release-mac-store.sh": (MAC_KEY, ":LSMinimumSystemVersion $MAC_MIN"),
    ".github/workflows/release.yml": (MAC_KEY, ":LSMinimumSystemVersion $MAC_MIN"),
}

VAR_FOR = {IOS_KEY: "IOS_MIN", MAC_KEY: "MAC_MIN"}

VERSION_RE = re.compile(r"^\d+(\.\d+)+$")
# The value handed to any Apple version-min flag, however the flag is spelled.
FLAG_RE = re.compile(r"-m(?:macosx|iphoneos|ios-simulator)-version-min=([^\s\"']+)")
DEPLOY_RE = re.compile(r"--minimum-deployment-target\s+(\S+)")


def parse_version(raw: str) -> tuple[int, ...]:
    return tuple(int(part) for part in raw.split("."))


def declaration_failures(config: dict) -> list[str]:
    failures = []
    for key, (bound, bound_str, why) in FLOOR_BOUNDS.items():
        raw = config.get(key)
        if not isinstance(raw, str) or not VERSION_RE.match(raw):
            failures.append(
                f"config/product.json: {key} must be a dotted version string, got {raw!r}"
            )
            continue
        if parse_version(raw) < bound:
            failures.append(
                f"config/product.json: {key} is {raw}, below {bound_str} — {why}"
            )
    return failures


def delegates_to_consumer(body: str, key: str) -> bool:
    """A consumer may INVOKE another consumer instead of reading the key itself.

    The invariant is that the floor comes from config/product.json and never
    from a private constant — not that every file parses the JSON. A workflow
    that calls a script which does is exactly as safe, and factoring the two
    apart is how the check and the developer loop stop drifting.

    The delegation is only accepted to a file this table ALSO checks for the
    same key. Anything looser turns "delegates" into a hole: a call to an
    unchecked script would satisfy the rule while reading a literal.

    COMMENTS DO NOT COUNT. These files explain themselves by naming their
    siblings — check-ios-pane.sh's own header names run-ios-sim.sh — so a
    mention in prose would let a script satisfy the rule by talking about
    another one while hardcoding its own floor. Only a line that actually runs
    it counts.
    """
    code = "\n".join(
        line.split("#", 1)[0] for line in body.splitlines()
    )
    for other, (other_key, _) in CONSUMERS.items():
        if other_key != key or not other.startswith("scripts/"):
            continue
        if other in code:
            return True
    return False


def consumer_failures(rel: str, body: str, key: str, stamp: str | None) -> list[str]:
    var = VAR_FOR[key]
    failures = []
    if key not in body and not delegates_to_consumer(body, key):
        failures.append(
            f"{rel}: never reads {key} from config/product.json, and invokes no "
            "script that does — its floor is a private constant that will drift "
            "from the declared one"
        )
    for match in FLAG_RE.finditer(body):
        value = match.group(1)
        if value not in (f"${var}", f"${{{var}}}"):
            failures.append(
                f"{rel}: hardcodes {match.group(0)} — the value must be ${var}, "
                f"derived from {key}"
            )
    for match in DEPLOY_RE.finditer(body):
        value = match.group(1).strip("\"'")
        if value not in (f"${var}", f"${{{var}}}"):
            failures.append(
                f"{rel}: hardcodes --minimum-deployment-target {match.group(1)} — "
                f"the value must be ${var}, derived from {key}"
            )
    if stamp is not None and stamp not in body:
        failures.append(
            f"{rel}: never stamps {stamp!r} into the bundle it assembles — the "
            "packaged plist would keep whatever floor the packager's template "
            "carries, which is how LSMinimumSystemVersion 10.11 shipped"
        )
    return failures


def self_test() -> list[str]:
    problems = []
    good_ios_body = (
        'IOS_MIN="$(python3 -c ... iosMinimumOSVersion ...)"\n'
        'CGO_CFLAGS="-isysroot $SDK -miphoneos-version-min=$IOS_MIN"\n'
        'ibtool --minimum-deployment-target "$IOS_MIN"\n'
        'PB "Set :MinimumOSVersion $IOS_MIN"\n'
    )
    checks = [
        ("missing floor", declaration_failures({MAC_KEY: "12.0"}),
         "iosMinimumOSVersion must be"),
        ("malformed floor", declaration_failures(
            {IOS_KEY: "fifteen", MAC_KEY: "12.0"}), "dotted version string"),
        ("iOS floor below the upload deadline", declaration_failures(
            {IOS_KEY: "13.0", MAC_KEY: "12.0"}), "below 15.0"),
        ("mac floor below the toolchain", declaration_failures(
            {IOS_KEY: "15.0", MAC_KEY: "10.11"}), "below 12.0"),
        ("version-min literal", consumer_failures(
            "x.sh", 'iosMinimumOSVersion\n-miphoneos-version-min=13.0\n',
            IOS_KEY, None), "hardcodes -miphoneos-version-min=13.0"),
        ("simulator version-min literal", consumer_failures(
            "x.sh", 'iosMinimumOSVersion\n-mios-simulator-version-min=15.0\n',
            IOS_KEY, None), "hardcodes -mios-simulator-version-min=15.0"),
        ("mac version-min literal", consumer_failures(
            "x.sh", 'macMinimumOSVersion\n-mmacosx-version-min=11.0\n'
            'PlistBuddy "Set :LSMinimumSystemVersion $MAC_MIN"',
            MAC_KEY, ":LSMinimumSystemVersion $MAC_MIN"),
         "hardcodes -mmacosx-version-min=11.0"),
        ("wrong variable", consumer_failures(
            "x.sh", 'macMinimumOSVersion\n-mmacosx-version-min=$IOS_MIN\n'
            'PlistBuddy "Set :LSMinimumSystemVersion $MAC_MIN"',
            MAC_KEY, ":LSMinimumSystemVersion $MAC_MIN"), "must be $MAC_MIN"),
        ("deployment-target literal", consumer_failures(
            "x.sh", 'iosMinimumOSVersion\nibtool --minimum-deployment-target 13.0\n',
            IOS_KEY, None), "hardcodes --minimum-deployment-target 13.0"),
        ("private constant", consumer_failures(
            "x.sh", 'IOS_MIN="15.0"\n-miphoneos-version-min=$IOS_MIN\n',
            IOS_KEY, None), "never reads"),
        ("unstamped bundle", consumer_failures(
            "x.sh", 'iosMinimumOSVersion\n-miphoneos-version-min=$IOS_MIN\n',
            IOS_KEY, ":MinimumOSVersion $IOS_MIN"), "never stamps"),
    ]
    for name, failures, fragment in checks:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: the {name!r} rule cannot fire")
    clean = declaration_failures({IOS_KEY: "15.0", MAC_KEY: "12.0"})
    clean += consumer_failures("x.sh", good_ios_body, IOS_KEY,
                               ":MinimumOSVersion $IOS_MIN")
    if clean:
        problems.append(f"self-test: a correct configuration still fails: {clean}")
    return problems


def main() -> int:
    if problems := self_test():
        print("min OS version checker self-test failed:")
        for p in problems:
            print(f"  - {p}")
        return 1

    failures: list[str] = []
    try:
        config = json.loads(PRODUCT.read_bytes())
    except (OSError, ValueError) as err:
        print(f"min OS version check failed:\n  - config/product.json: {err}")
        return 1
    failures += declaration_failures(config)

    for rel, (key, stamp) in CONSUMERS.items():
        try:
            body = (ROOT / rel).read_text(encoding="utf-8")
        except OSError as err:
            failures.append(f"{rel}: unreadable ({err})")
            continue
        failures += consumer_failures(rel, body, key, stamp)

    if failures:
        print("min OS version check failed:")
        for f in failures:
            print(f"  - {f}")
        return 1
    print(
        f"min OS version check passed "
        f"(iOS {config[IOS_KEY]}, macOS {config[MAC_KEY]})"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
