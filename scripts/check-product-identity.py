#!/usr/bin/env python3
"""Hold every identity value outside config/product.json equal to it.

Some files that carry the product's identity are read by external tools that
cannot parse the identity file: the fyne CLI reads the two FyneApp.toml files,
Apple and Google fetch the deep-link association files, and the Objective-C
keychain fallback is compiled before any Go runs. Those values must stay byte-equal to config/product.json, and this
checker is what makes that a property rather than a habit.

Per-publisher records are deliberately NOT checked against the identity file:
the Apple team id inside the Universal Links entry, the Android signing
certificate fingerprint, and the numeric App Store listing id belong to
whoever ships the build, not to the product.

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
PRODUCT_PATH = ROOT / "config" / "product.json"


def load_identity() -> dict:
    return json.loads(PRODUCT_PATH.read_bytes())


def rule_failures(identity: dict, read) -> list[str]:
    """All consistency rules, over an injectable file reader for the self-test."""
    failures: list[str] = []

    def text(rel: str) -> str | None:
        raw = read(rel)
        if raw is None:
            failures.append(f"{rel}: required file is missing")
            return None
        return raw.decode("utf-8", "replace")

    site = identity["siteBase"]
    host = re.sub(r"^https://", "", site)

    # 1. FyneApp.toml: Website, ID and Name must each appear as exactly one
    # non-comment line carrying exactly the identity value. Line-anchored on
    # purpose: a substring match is satisfied by a stale value in a comment or
    # a duplicate line while the fyne CLI reads something else.
    for rel, want_id in (
        ("cmd/mobile/FyneApp.toml", identity["appID"]),
        ("cmd/desktop/FyneApp.toml", identity["desktopAppID"]),
    ):
        if (body := text(rel)) is None:
            continue
        lines = [l.strip() for l in body.splitlines() if not l.strip().startswith("#")]
        for key, want in (("Website", site), ("ID", want_id), ("Name", identity["productName"])):
            carriers = [l for l in lines if l.startswith(f"{key} = ")]
            if carriers != [f'{key} = "{want}"']:
                failures.append(
                    f"{rel}: {key} does not match the identity file "
                    f"as exactly one line ({want})"
                )

    # 2. Universal Links: parse the JSON and require every app id entry to be
    # a ten-character team id prefixing OUR app id. JSON has no comments, so
    # parsing (rather than searching the bytes) is what defeats shadowing.
    if (body := text("docs/apple-app-site-association")) is not None:
        ids: list[str] = []
        try:
            for detail in json.loads(body)["applinks"]["details"]:
                ids += detail.get("appIDs") or ([detail["appID"]] if "appID" in detail else [])
        except (ValueError, KeyError, TypeError):
            failures.append("docs/apple-app-site-association: does not parse")
            ids = None
        if ids is not None:
            pattern = re.compile(r"^[A-Z0-9]{10}\." + re.escape(identity["appID"]) + r"$")
            if not ids or not all(pattern.match(i) for i in ids):
                failures.append(
                    "docs/apple-app-site-association: app id entries are not all "
                    "TEAMID." + identity["appID"]
                )

    # 3. Android app links: every target's package_name must be our app id.
    if (body := text("docs/assetlinks.json")) is not None:
        try:
            packages = [e["target"]["package_name"] for e in json.loads(body)]
        except (ValueError, KeyError, TypeError):
            failures.append("docs/assetlinks.json: does not parse")
            packages = None
        if packages is not None:
            if not packages or any(p != identity["appID"] for p in packages):
                failures.append(
                    "docs/assetlinks.json: package_name entries are not all "
                    + identity["appID"]
                )

    # The site publisher and its generated CNAME are deliberately NOT rules
    # here: publish-site.sh derives DOMAIN from the identity file at run time,
    # so there is no second copy to hold equal.

    # 4. The native keychain fallback, which cannot read the identity file.
    # Non-comment lines only, for the same shadowing reason as rule 1.
    if (body := text("ai_secure_store_darwin.go")) is not None:
        carriers = [
            l.strip() for l in body.splitlines()
            if 'bundleID = @"' in l and not l.strip().startswith("//")
        ]
        if carriers != [f'if (bundleID.length == 0) bundleID = @"{identity["appID"]}";']:
            failures.append(
                "ai_secure_store_darwin.go: keychain fallback bundle id "
                "does not match appID as exactly one line"
            )

    # 5. The release workflow must actually reference the declared secret —
    # a secrets.NAME expression, not merely the name in prose.
    if (body := text(".github/workflows/release.yml")) is not None:
        live = [l for l in body.splitlines() if not l.strip().startswith("#")]
        if not any("secrets." + identity["bundledKeySecretName"] in l for l in live):
            failures.append(
                ".github/workflows/release.yml: no secrets."
                + identity["bundledKeySecretName"] + " reference"
            )

    return failures


def real_reader(rel: str) -> bytes | None:
    p = ROOT / rel
    try:
        return p.read_bytes()
    except OSError:
        return None


def self_test() -> list[str]:
    """Prove every sub-rule fires, then that a consistent tree passes clean.

    Asserting one distinct failure message per SUB-rule matters: a per-file
    assertion is satisfied while a deleted sibling rule (say, the Name check)
    regresses silently behind its neighbours.
    """
    identity = {
        "productName": "SelfTest",
        "siteBase": "https://selftest.invalid",
        "appID": "invalid.selftest",
        "desktopAppID": "invalid.selftest.desktop",
        "supportEmail": "support@selftest.invalid",
        "audioBase": "https://selftest.invalid/audio/",
        "sourceRepo": "https://selftest.invalid/repo",
        "bundledKeySecretName": "SELFTEST_SECRET",
    }
    wrong = {
        # Each value wrong AND shadowed by a comment carrying the right one,
        # so the self-test also proves the rules see through shadowing.
        "cmd/mobile/FyneApp.toml": (
            b'# Website = "https://selftest.invalid"\n'
            b'Website = "https://other.invalid"\n'
            b'# ID = "invalid.selftest"\n'
            b'ID = "wrong.id"\n'
            b'# Name = "SelfTest"\n'
            b'Name = "Other"\n'
        ),
        "cmd/desktop/FyneApp.toml": (
            b'Website = "https://other.invalid"\n'
            b'ID = "wrong.id"\n'
            b'Name = "Other"\n'
        ),
        "docs/apple-app-site-association": b'{"applinks":{"details":[{"appIDs":["ABCDE12345.wrong.id"]}]}}',
        "docs/assetlinks.json": b'[{"target":{"package_name": "wrong.id"}}]',
        "ai_secure_store_darwin.go": (
            b'// if (bundleID.length == 0) bundleID = @"invalid.selftest";\n'
            b'    if (bundleID.length == 0) bundleID = @"wrong.id";\n'
        ),
        ".github/workflows/release.yml": b"# secrets.SELFTEST_SECRET mentioned in prose only\n",
    }
    failures = rule_failures(identity, lambda rel: wrong.get(rel))
    problems = []
    expected_messages = (
        "cmd/mobile/FyneApp.toml: Website does not match",
        "cmd/mobile/FyneApp.toml: ID does not match",
        "cmd/mobile/FyneApp.toml: Name does not match",
        "cmd/desktop/FyneApp.toml: Website does not match",
        "cmd/desktop/FyneApp.toml: ID does not match",
        "cmd/desktop/FyneApp.toml: Name does not match",
        "apple-app-site-association: app id entries",
        "assetlinks.json: package_name entries",
        "keychain fallback bundle id",
        "no secrets.SELFTEST_SECRET reference",
    )
    for fragment in expected_messages:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: no rule produced {fragment!r}")
    right = {
        "cmd/mobile/FyneApp.toml": (
            b'Website = "https://selftest.invalid"\n'
            b'Name = "SelfTest"\n'
            b'ID = "invalid.selftest"\n'
        ),
        "cmd/desktop/FyneApp.toml": (
            b'Website = "https://selftest.invalid"\n'
            b'Name = "SelfTest"\n'
            b'ID = "invalid.selftest.desktop"\n'
        ),
        "docs/apple-app-site-association": b'{"applinks":{"details":[{"appIDs":["ABCDE12345.invalid.selftest"]}]}}',
        "docs/assetlinks.json": b'[{"target":{"package_name": "invalid.selftest"}}]',
        "ai_secure_store_darwin.go": b'    if (bundleID.length == 0) bundleID = @"invalid.selftest";\n',
        ".github/workflows/release.yml": b"env:\n  K: ${{ secrets.SELFTEST_SECRET }}\n",
    }
    if clean := rule_failures(identity, lambda rel: right.get(rel)):
        problems.append(f"self-test: consistent tree still fails: {clean}")
    return problems


def main() -> int:
    if problems := self_test():
        print("product identity checker self-test failed:")
        for p in problems:
            print(f"  - {p}")
        return 1
    failures = rule_failures(load_identity(), real_reader)
    if failures:
        print("product identity check failed:")
        for f in failures:
            print(f"  - {f}")
        return 1
    print("product identity check passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
