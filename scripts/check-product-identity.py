#!/usr/bin/env python3
"""Hold every identity value outside config/product.json equal to it.

Some files that carry the product's identity are read by external tools that
cannot parse the identity file: the fyne CLI reads the two FyneApp.toml files,
Apple and Google fetch the deep-link association files, the site publisher
stamps the Pages domain, and the Objective-C keychain fallback is compiled
before any Go runs. Those values must stay byte-equal to config/product.json, and this
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

    # 1. FyneApp.toml: Website and ID must match the identity file.
    for rel, want_id in (
        ("cmd/mobile/FyneApp.toml", identity["appID"]),
        ("cmd/desktop/FyneApp.toml", identity["desktopAppID"]),
    ):
        if (body := text(rel)) is None:
            continue
        if f'Website = "{site}"' not in body:
            failures.append(f"{rel}: Website does not match siteBase {site}")
        if f'ID = "{want_id}"' not in body:
            failures.append(f"{rel}: ID does not match the identity file ({want_id})")
        if f'Name = "{identity["productName"]}"' not in body:
            failures.append(f"{rel}: Name does not match productName")

    # 2. Universal Links: the app id after the team-id prefix must match.
    if (body := text("docs/apple-app-site-association")) is not None:
        if not re.search(r'"[A-Z0-9]{10}\.' + re.escape(identity["appID"]) + '"', body):
            failures.append(
                "docs/apple-app-site-association: no TEAMID."
                + identity["appID"] + " entry"
            )

    # 3. Android app links: package_name must match.
    if (body := text("docs/assetlinks.json")) is not None:
        if f'"package_name": "{identity["appID"]}"' not in body:
            failures.append("docs/assetlinks.json: package_name does not match appID")

    # 4. The site publisher's DOMAIN variable. The published CNAME file is
    # GENERATED from this value at publish time, so checking DOMAIN covers the
    # custom domain transitively; no CNAME is tracked on this branch.
    if (body := text("scripts/publish-site.sh")) is not None:
        if f"DOMAIN={host}" not in body:
            failures.append(f"scripts/publish-site.sh: DOMAIN is not {host}")

    # 5. The native keychain fallback, which cannot read the identity file.
    if (body := text("ai_secure_store_darwin.go")) is not None:
        if f'bundleID = @"{identity["appID"]}"' not in body:
            failures.append(
                "ai_secure_store_darwin.go: keychain fallback bundle id "
                "does not match appID"
            )

    # 6. The release workflow references the declared secret name.
    if (body := text(".github/workflows/release.yml")) is not None:
        if identity["bundledKeySecretName"] not in body:
            failures.append(
                ".github/workflows/release.yml: does not reference "
                + identity["bundledKeySecretName"]
            )

    return failures


def real_reader(rel: str) -> bytes | None:
    p = ROOT / rel
    try:
        return p.read_bytes()
    except OSError:
        return None


def self_test() -> list[str]:
    """Prove every rule can fire by feeding it a violating synthetic tree."""
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
        "cmd/mobile/FyneApp.toml": b'Website = "https://other.invalid"\nID = "wrong.id"\nName = "Other"\n',
        "cmd/desktop/FyneApp.toml": b'Website = "https://other.invalid"\nID = "wrong.id"\nName = "Other"\n',
        "docs/apple-app-site-association": b'{"applinks":{"details":[{"appIDs":["ABCDE12345.wrong.id"]}]}}',
        "docs/assetlinks.json": b'[{"target":{"package_name": "wrong.id"}}]',
        "scripts/publish-site.sh": b"DOMAIN=other.invalid\n",
        "ai_secure_store_darwin.go": b'bundleID = @"wrong.id";\n',
        ".github/workflows/release.yml": b"env:\n  OTHER: x\n",
    }
    failures = rule_failures(identity, lambda rel: wrong.get(rel))
    problems = []
    # Every rule must have fired at least once against the violating tree.
    expected_fragments = (
        "cmd/mobile/FyneApp.toml", "cmd/desktop/FyneApp.toml",
        "apple-app-site-association", "assetlinks.json",
        "publish-site.sh", "ai_secure_store_darwin.go", "release.yml",
    )
    for fragment in expected_fragments:
        if not any(fragment in f for f in failures):
            problems.append(f"self-test: the {fragment} rule cannot fire")
    # And a fully consistent synthetic tree must pass clean.
    right = {
        "cmd/mobile/FyneApp.toml": b'Website = "https://selftest.invalid"\nName = "SelfTest"\nID = "invalid.selftest"\n',
        "cmd/desktop/FyneApp.toml": b'Website = "https://selftest.invalid"\nName = "SelfTest"\nID = "invalid.selftest.desktop"\n',
        "docs/apple-app-site-association": b'{"applinks":{"details":[{"appIDs":["ABCDE12345.invalid.selftest"]}]}}',
        "docs/assetlinks.json": b'[{"target":{"package_name": "invalid.selftest"}}]',
        "scripts/publish-site.sh": b"DOMAIN=selftest.invalid\n",
        "ai_secure_store_darwin.go": b'bundleID = @"invalid.selftest";\n',
        ".github/workflows/release.yml": b"env:\n  SELFTEST_SECRET: x\n",
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
