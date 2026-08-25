#!/usr/bin/env python3
"""Validate the single public-support mailbox source and every delivery gate."""

from __future__ import annotations

from pathlib import Path
import re
import subprocess
import sys

from support_contact_config import (
    SupportContactConfigurationError,
    grammar_self_test_failures,
    read_support_email,
)


ROOT = Path(__file__).resolve().parents[1]
CONFIG_PATH = "config/support-email.txt"
PATTERN_PATH = "config/support-email-pattern.txt"
DISPLAY_MARKER = b"{{BIBLETEXT_SUPPORT_EMAIL_DISPLAY}}"
HREF_MARKER = b"{{BIBLETEXT_SUPPORT_EMAIL_HREF}}"
LEGACY_MARKER = b"{{BIBLETEXT_SUPPORT_EMAIL}}"
DISPLAY_SLOT = b">" + DISPLAY_MARKER + b"</a>"
HREF_SLOT = b'href="mailto:' + HREF_MARKER + b'"'

SITE_TEMPLATES = {
    "docs/index.html": (0, 0),
    "docs/privacy.html": (1, 1),
    "docs/support.html": (1, 1),
}

# These entry points can put the configured contact into a public page, store
# submission, or release binary. Requiring the checker by name prevents a new
# address from being accepted by one path while another silently keeps a copy.
REQUIRED_GATES = {
    ".github/workflows/ci.yml": 1,
    ".github/workflows/release.yml": 1,
    "appstore/preflight.py": 1,
    "appstore/push-metadata.py": 1,
    "appstore/push-review-notes.py": 1,
    "scripts/build-android.sh": 1,
    "scripts/publish-site.sh": 1,
    "scripts/release-ios.sh": 1,
}

OBSOLETE_LOCAL_APPSTORE_PATHS = (
    "build/appstore/PRE-RELEASE-FINDINGS-1.1.8.md",
    "build/appstore/push_betareview.py",
    "build/appstore/push_pricing.py",
    "build/appstore/push_testflight.py",
    "build/appstore/review_notes.txt",
    "build/appstore/upload_screenshots.py",
)


def repository_paths() -> list[str]:
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    return [
        raw.decode("utf-8", "surrogateescape")
        for raw in result.stdout.split(b"\0")
        if raw
    ]


def read_required(path: str, errors: list[str]) -> bytes | None:
    try:
        return (ROOT / path).read_bytes()
    except OSError:
        errors.append(f"{path}: required file is missing or unreadable")
        return None


def configured_email(errors: list[str]) -> bytes | None:
    try:
        return read_support_email()
    except SupportContactConfigurationError:
        errors.append(
            f"{CONFIG_PATH}: expected exactly one conservative ASCII address "
            f"matching {PATTERN_PATH}"
        )
        return None


def check() -> list[str]:
    errors = grammar_self_test_failures()
    email = configured_email(errors)

    for path, (expected_display, expected_href) in SITE_TEMPLATES.items():
        data = read_required(path, errors)
        if data is None:
            continue
        display_count = data.count(DISPLAY_MARKER)
        if display_count != expected_display:
            errors.append(
                f"{path}: found {display_count} support display markers; "
                f"expected exactly {expected_display}"
            )
        href_count = data.count(HREF_MARKER)
        if href_count != expected_href:
            errors.append(
                f"{path}: found {href_count} support href markers; "
                f"expected exactly {expected_href}"
            )
        display_slots = data.count(DISPLAY_SLOT)
        if display_slots != expected_display:
            errors.append(
                f"{path}: found {display_slots} support display slots; "
                f"expected exactly {expected_display}"
            )
        href_slots = data.count(HREF_SLOT)
        if href_slots != expected_href:
            errors.append(
                f"{path}: found {href_slots} support href slots; "
                f"expected exactly {expected_href}"
            )
        if LEGACY_MARKER in data:
            errors.append(f"{path}: contains the ambiguous legacy support marker")
        if email is not None and email in data:
            errors.append(f"{path}: embeds the support address instead of its marker")

    app = read_required("ai_panel.go", errors)
    if app is not None:
        uses = len(re.findall(rb"\bOpaque:\s*SupportMailtoRecipient\(\)", app))
        if uses != 1:
            errors.append(
                "ai_panel.go: the report-mail recipient must use "
                "SupportMailtoRecipient exactly once"
            )

    contact_source = read_required("support_contact.go", errors)
    if contact_source is not None:
        for embedded_path in (CONFIG_PATH, PATTERN_PATH):
            if contact_source.count(b"//go:embed " + embedded_path.encode()) != 1:
                errors.append(
                    f"support_contact.go: expected one embed of {embedded_path}"
                )
        if contact_source.count(b"func SupportMailtoRecipient() string") != 1:
            errors.append(
                "support_contact.go: expected one public mailto-recipient formatter"
            )

    site_source = read_required("cmd/sitepages/main.go", errors)
    if site_source is not None:
        for accessor in (b"bibletext.SupportEmail()", b"bibletext.SupportMailtoRecipient()"):
            count = site_source.count(accessor)
            if count != 1:
                errors.append(
                    f"cmd/sitepages/main.go: expected one use of {accessor.decode()}"
                )

    checker_name = Path(__file__).name.encode()
    for path, expected in REQUIRED_GATES.items():
        data = read_required(path, errors)
        if data is None:
            continue
        count = data.count(checker_name)
        if count != expected:
            errors.append(
                f"{path}: references the support-contact gate {count} times; "
                f"expected exactly {expected}"
            )

    for path in OBSOLETE_LOCAL_APPSTORE_PATHS:
        if (ROOT / path).exists():
            errors.append(f"{path}: obsolete private store-metadata path must be removed")

    if email is not None:
        copies: list[str] = []
        try:
            paths = repository_paths()
        except (OSError, subprocess.CalledProcessError):
            errors.append("could not enumerate repository files")
            paths = []
        for path in paths:
            if path == CONFIG_PATH:
                continue
            worktree = ROOT / path
            if not worktree.is_file():
                continue
            try:
                data = worktree.read_bytes()
            except OSError:
                errors.append(f"{path}: could not inspect public-contact content")
                continue
            if email in data:
                copies.append(path)
        if copies:
            errors.extend(
                f"{copy}: duplicates the configured support address" for copy in copies
            )

    return errors


def main() -> int:
    errors = check()
    if errors:
        print("public support configuration check failed:", file=sys.stderr)
        for error in errors:
            print(f"  - {error}", file=sys.stderr)
        return 1
    print("public support configuration check passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
