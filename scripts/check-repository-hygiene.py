#!/usr/bin/env python3
"""Reject public-repository content that can expose credentials or private provenance."""

from __future__ import annotations

import argparse
import base64
import binascii
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys

from support_contact_config import (
    SupportContactConfigurationError,
    parse_support_email,
)


ROOT = Path(__file__).resolve().parents[1]
SELF = "scripts/check-repository-hygiene.py"
PROCESS_GUIDE = "docs/COMMIT_AND_CODE_PROTOCOL.md"
PUBLIC_SUPPORT_CONFIG = "config/support-email.txt"
MAX_SEMANTIC_BYTES = 4 * 1024 * 1024

FORBIDDEN_BASENAMES = {
    "CLAUDE.md",
    "COMMIT_EDITMSG",
    "FYNE_28_SANDBOX.md",
    "FYNE_FORK_PLAN.md",
    "HANDOFF.md",
    "NOTES_SCRAPBOOK.md",
    "SESSION_STATUS.md",
    "bundled_key_gen.go",
    "embed-bible-key.sh",
}
FORBIDDEN_SUFFIXES = {
    ".jks",
    ".keystore",
    ".mobileprovision",
    ".p12",
    ".p8",
}

# Keep literals that the checker itself rejects out of its own tracked bytes.
PROCESS_PATTERNS = (
    ("authorship trailer", re.compile(rb"(?im)^\s*Co-" + rb"Authored-By\s*:")),
    ("AI generated-by credit", re.compile(
        rb"(?i)generated\s+(?:with|by)\s+(?:Claude|Claude\s+Code|ChatGPT|Codex)"
    )),
    ("Claude Code promotion", re.compile(rb"(?i)anthropic\.claude-code|claude-code")),
    ("removed private/process-artifact reference", re.compile(
        rb"(?i)\b(?:CLAUDE\.md|SESSION_STATUS\.md|NOTES_SCRAPBOOK\.md|"
        rb"FYNE_FORK_PLAN\.md|FYNE_28_SANDBOX\.md|HANDOFF\.md|"
        rb"embed-bible-key\.sh)\b"
    )),
    ("signing process-record reference", re.compile(
        rb"(?i)\b(?:sha-?256|fingerprint|cert(?:ificate)?)\b"
        rb"(?:[^\r\n]*\r?\n)?[^\r\n]{0,120}\bsession\s+notes?\b"
    )),
    ("local home-directory path", re.compile(rb"/Users/(?!<)[^/\s]+/")),
    ("camera-roll identifier", re.compile(rb"(?i)\bIMG[_-]\d{3,}\b")),
    ("conversation provenance", re.compile(
        rb"(?i)\b(?:the\s+)?(?:owner|user)\s+(?:(?:had\s+to\s+)?"
        rb"(?:ask|say|report|write|request)|asked|said|reported|wrote|requested)\b"
        rb"|\b(?:as|per)\s+(?:the\s+)?(?:owner'?s|user'?s|your)\s+(?:request|instruction)\b"
        rb"|\b(?:owner|user)'?s\s+(?:central\s+)?(?:requirement|dev(?:elopment)?\s+links?)\b"
        rb"|\bowner\s+(?:approval|decision|sign[- ]?off)\b|\baudit\s+finding\b"
        rb"|\b(?:prompt|conversation)\s+transcript\b|\bverbatim\s+(?:report|prompt|quote)\b"
    )),
    ("internal task provenance", re.compile(
        rb"(?i)\b(?:owner|user)\s+task\s*#?\d+\b|\btask\s*#\d+\b"
        rb"|\btask\s+\d+'?s\s+(?:ABI\s+)?(?:shape|requirement|request|finding)\b"
    )),
    ("review-process narration", re.compile(
        rb"(?i)\b(?:review|audit)\s+(?:finding|found|defeated|verdict)\b"
        rb"|\bfound\s+by\s+(?:(?:an?|the)\s+)?(?:adversarial\s+)?(?:review|audit)\b"
        rb"|\badversarial\s+(?:review|audit)\b"
        rb"|\breview(?:[- ]driven|\s+(?:mutation|measured))\b"
        rb"|\baudit\s+(?:offered|confirmed)\b"
        rb"|\bfootgun\s*\(\s*(?:review|audit)\s*\)"
    )),
    ("commit-process narration", re.compile(
        rb"(?i)\b(?:the\s+)?(?:S\d+\s+)?commit\s+(?:said|claimed|promised)\b"
    )),
    ("test-discovery narration", re.compile(
        rb"(?i)\b(?:emulator|simulator|device|screenshot)-(?:caught|found|discovered)\b"
    )),
    ("named-person process attribution", re.compile(
        rb"\b[A-Z][a-z]{2,}\s+\((?i:(?:upstream\s+)?(?:maintainer|contributor|reviewer))\)"
        rb"\s+(?i:said|wrote|reported|declined|pointed)\b"
    )),
    ("specific local-machine narration", re.compile(
        rb"(?i)\bthis\s+(?:mac|machine|computer|laptop)\s+"
        rb"(?:has|is|was|swings|runs|cannot|can't|does|did)\b"
        rb"|\bon\s+this\s+mac\s+(?:before|currently|today)\b"
        rb"|\b(?:dev|development)\s+machine\b"
    )),
    ("conversational request framing", re.compile(
        rb"(?i)\b(?:because|when)\s+you\s+(?:asked|requested)\b"
        rb"|\byou\s+(?:asked|requested)\s+to\b"
        rb"|\bthe\s+one\s+you\s+(?:asked|requested)\b"
        rb"|\byou\s+have\s+explicitly\s+(?:asked|requested)\b"
        rb"|\bscenario\s*:\s*you\b"
    )),
    ("revision-process narration", re.compile(
        rb"(?m)^\s*(?:(?://|#|\*)\s*)?AMENDED[.:]"
    )),
    ("agent-process narration", re.compile(
        rb"(?i)\b(?:\d+|two|three|four|five|six|seven|eight|nine|ten)[ -]agent\b"
        rb"|\bagent\s+(?:panel|session|judge|audit)\b|\bpanel\s+of\s+agents\b"
        rb"|\baudit\s+verdict\b|\brefuter\b|\bfield[- ]report(?:ed)?\b"
    )),
    ("App Store Connect private-key filename", re.compile(rb"\bAuthKey_[A-Z0-9]{10}\.p8\b")),
    ("realistic private-message fixture", re.compile(
        rb"(?i)\b(?:thinking|thought)\s+of\s+you\b|\bpraying\s+for\s+you\b"
        rb"|\bsent\s+to\s+(?:mom|mum|dad|wife|husband|daughter|son)\b"
        rb"|\bat\s+the\s+hospital\b|\bwhat\s+we\s+talked\s+about\s+on\s+the\s+phone\b"
        rb"|\b(?:call|love)\s+you\b"
    )),
)

SECRET_PATTERNS = (
    ("private-key material", re.compile(rb"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----")),
    ("OpenAI-style secret", re.compile(rb"\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{20,}\b")),
    ("Anthropic secret", re.compile(rb"\bsk-ant-[A-Za-z0-9_-]{20,}\b")),
    ("Google API secret", re.compile(rb"\bAIza[0-9A-Za-z_-]{35}\b")),
    ("xAI secret", re.compile(rb"\bxai-[A-Za-z0-9_-]{20,}\b")),
    ("GitHub token", re.compile(rb"\bgh[opusr]_[A-Za-z0-9]{30,}\b")),
    ("AWS access key", re.compile(rb"\b(?:AKIA|ASIA)[A-Z0-9]{16}\b")),
    ("Slack token", re.compile(rb"\bxox[baprs]-[A-Za-z0-9-]{20,}\b")),
)

BASE64_CANDIDATE = re.compile(rb"(?<![A-Za-z0-9_-])[A-Za-z0-9_-]{24,}={0,2}(?![A-Za-z0-9_-])")
DECODED_HIGH_RISK = (
    b"co-authored-by",
    b"generated with claude",
    b"/users/",
)
PRIVATE_CONTEXT = (
    b"hospital",
    b"wife",
    b"husband",
    b"mom",
    b"mum",
    b"dad",
    b"daughter",
    b"son",
    b"call me",
    b"love you",
    b"prayer",
)
EMAIL = re.compile(rb"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}")

PROCESS_POSITIVE_CASES = (
    ("conversation provenance", b"The owner had to report a missing control."),
    ("conversation provenance", b"The user said to preserve the layout."),
    ("conversation provenance", b"The owner's central requirement was visibility."),
    ("internal task provenance", b"Implemented as task #42."),
    ("review-process narration", b"Found by adversarial review."),
    ("review-process narration", b"Review mutation M2 removed the guard."),
    ("commit-process narration", b"The commit said the change was invisible."),
    ("test-discovery narration", b"Emulator-caught: the view jumped."),
    ("named-person process attribution", b"Alex (maintainer) wrote that the loop differs."),
    ("specific local-machine narration", b"This machine swings between benchmark runs."),
    ("specific local-machine narration", b"The test fails on the dev machine."),
    ("conversational request framing", b"The card appears because you asked to see it."),
    ("conversational request framing", b"It appears on the passage you asked to see."),
    ("revision-process narration", b"// AMENDED. The previous assertion differed."),
    ("removed private/process-artifact reference", b"See SESSION_STATUS.md."),
    ("signing process-record reference", b"The fingerprint is in the session notes."),
)
PROCESS_NEGATIVE_CASES = (
    b"Run this command on the macOS build host.",
    b"GitHub issue #42 tracks the defect.",
    b"Review the invariant before merging.",
    b"The emulator reproduces the rendering behavior.",
    b"No available iPhone simulator found.",
    b"Anthropic Claude is a supported provider.",
    b"An own note appears only while explicitly focused.",
    b"The UI says Note from you.",
)

CANONICAL_SCRIPTURE_PREFIXES = (
    "assets/parallels/",
    "assets/seed/",
    "bsb/",
    "web/",
    "webc/",
)


def git(*args: str, input_bytes: bytes | None = None) -> bytes:
    return subprocess.run(
        ["git", *args],
        cwd=ROOT,
        input=input_bytes,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    ).stdout


def tracked_paths() -> list[str]:
    raw = git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
    return [p.decode("utf-8", "surrogateescape") for p in raw.split(b"\0") if p]


def process_labels(data: bytes) -> list[str]:
    return [label for label, pattern in PROCESS_PATTERNS if pattern.search(data)]


def process_pattern_test_failures() -> list[str]:
    failures: list[str] = []
    for index, (expected, sample) in enumerate(PROCESS_POSITIVE_CASES, 1):
        if expected not in process_labels(sample):
            failures.append(f"positive process-pattern case {index} missed {expected}")
    for index, sample in enumerate(PROCESS_NEGATIVE_CASES, 1):
        labels = process_labels(sample)
        if labels:
            failures.append(
                f"negative process-pattern case {index} matched {', '.join(labels)}"
            )
    return failures


def canonical_scripture_path(path: str) -> bool:
    return path.startswith(CANONICAL_SCRIPTURE_PREFIXES) or path.startswith(
        "red_letter_"
    )


def path_problem(path: str) -> str | None:
    p = PurePosixPath(path)
    lower_parts = tuple(part.lower() for part in p.parts)
    if p.name in FORBIDDEN_BASENAMES:
        return f"forbidden tracked filename {p.name}"
    if p.suffix.lower() in FORBIDDEN_SUFFIXES:
        return f"forbidden credential/signing file type {p.suffix}"
    if any(part == ".claude" for part in lower_parts):
        return "tool-specific private configuration is tracked"
    if p.name == ".env" or p.name.startswith(".env."):
        return "local environment file is tracked"
    return None


def local_secrets() -> list[tuple[str, bytes]]:
    found: list[tuple[str, bytes]] = []
    for name in (
        "BIBLE_API_KEY",
        "ANTHROPIC_API_KEY",
        "OPENAI_API_KEY",
        "GEMINI_API_KEY",
        "XAI_API_KEY",
    ):
        value = os.environ.get(name, "").strip().encode()
        if len(value) >= 8:
            found.append((f"{name} from the process environment", value))

    env_path = ROOT / ".env.local"
    if env_path.is_file():
        for raw_line in env_path.read_bytes().splitlines():
            line = raw_line.strip()
            if not line or line.startswith(b"#") or b"=" not in line:
                continue
            raw_name, value = line.split(b"=", 1)
            name = raw_name.decode("ascii", "ignore").strip()
            if not re.search(r"(?:KEY|TOKEN|SECRET|PASSWORD)", name, re.I):
                continue
            value = value.strip().strip(b"\"'")
            if len(value) >= 8:
                found.append((f"{name} from .env.local", value))

    if sys.platform == "darwin":
        try:
            result = subprocess.run(
                ["security", "find-generic-password", "-a", "release", "-s",
                 "uk.co.bibletext.apibible-release", "-w"],
                check=False,
                stdout=subprocess.PIPE,
                stderr=subprocess.DEVNULL,
                timeout=3,
            )
        except (OSError, subprocess.TimeoutExpired):
            result = None
        if result is not None and result.returncode == 0:
            value = result.stdout.strip()
            if len(value) >= 8:
                found.append(("BIBLE_API_KEY from the dedicated Keychain item", value))
    return found


def secret_forms(secret: bytes, include_release_form: bool) -> tuple[tuple[str, bytes], ...]:
    forms = [
        ("plaintext", secret),
        ("base64", base64.b64encode(secret)),
        ("unpadded base64", base64.b64encode(secret).rstrip(b"=")),
        ("URL-safe base64", base64.urlsafe_b64encode(secret).rstrip(b"=")),
        ("base32", base64.b32encode(secret)),
        ("hex", binascii.hexlify(secret)),
        ("uppercase hex", binascii.hexlify(secret).upper()),
        ("percent encoding", b"".join(f"%{byte:02X}".encode() for byte in secret)),
        ("UTF-16 little-endian", secret.decode("utf-8", "ignore").encode("utf-16-le")),
        ("UTF-16 big-endian", secret.decode("utf-8", "ignore").encode("utf-16-be")),
        ("reversed plaintext", secret[::-1]),
    ]
    if include_release_form:
        mask = b"bibletext-nkjv"
        transformed = bytes(b ^ mask[i % len(mask)] for i, b in enumerate(secret))
        forms.append(("release-linker encoding", base64.b64encode(transformed)))
    return tuple(forms)


def decoded_payload_problem(data: bytes) -> str | None:
    if len(data) > MAX_SEMANTIC_BYTES or b"\0" in data[:8192]:
        return None
    for match in BASE64_CANDIDATE.finditer(data):
        token = match.group(0)
        padded = token + b"=" * (-len(token) % 4)
        for altchars in (None, b"-_"):
            try:
                decoded = base64.b64decode(padded, altchars=altchars, validate=True)
            except (binascii.Error, ValueError):
                continue
            if len(decoded) < 12:
                continue
            printable = sum(c in b"\t\n\r" or 32 <= c < 127 for c in decoded)
            if printable / len(decoded) < 0.85:
                continue
            low = decoded.lower()
            if any(marker in low for marker in DECODED_HIGH_RISK):
                return "encoded payload contains private/process provenance"
            if EMAIL.search(decoded):
                return "encoded payload contains an email address"
            if sum(marker in low for marker in PRIVATE_CONTEXT) >= 2:
                return "encoded payload contains realistic private-message context"
    return None


def content_problems(path: str, data: bytes, secrets: list[tuple[str, bytes]]) -> list[str]:
    problems: list[str] = []
    for label, pattern in SECRET_PATTERNS:
        if pattern.search(data):
            problems.append(label)
    if path not in {SELF, PROCESS_GUIDE} and not canonical_scripture_path(path):
        for label in process_labels(data):
            problems.append(label)
    for source, secret in secrets:
        include_release_form = source.startswith("BIBLE_API_KEY")
        for form_name, form in secret_forms(secret, include_release_form):
            if form and form in data:
                problems.append(f"{source} appears in {form_name} form")
    encoded = decoded_payload_problem(data)
    if encoded:
        problems.append(encoded)
    return problems


def scan_current(secrets: list[tuple[str, bytes]]) -> list[str]:
    failures: list[str] = []
    try:
        support_raw = (ROOT / PUBLIC_SUPPORT_CONFIG).read_bytes()
    except OSError:
        failures.append(f"{PUBLIC_SUPPORT_CONFIG}: required configuration is missing")
        support_raw = b""
    try:
        support_email = parse_support_email(support_raw)
        support_valid = True
    except SupportContactConfigurationError:
        support_email = b""
        support_valid = False
    if support_raw and not support_valid:
        failures.append(
            f"{PUBLIC_SUPPORT_CONFIG}: must contain exactly one conservative "
            "ASCII email address"
        )
    for path in tracked_paths():
        worktree_path = ROOT / path
        if problem := path_problem(path):
            failures.append(f"{path}: {problem}")
            continue
        candidates: list[tuple[str, bytes]] = []
        worktree_data: bytes | None = None
        if worktree_path.is_file():
            worktree_data = worktree_path.read_bytes()
            candidates.append(("working tree", worktree_data))

        index_result = subprocess.run(
            ["git", "show", f":{path}"], cwd=ROOT, check=False,
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        )
        if index_result.returncode == 0 and index_result.stdout != worktree_data:
            candidates.append(("index", index_result.stdout))

        for source, data in candidates:
            for problem in content_problems(path, data, secrets):
                failures.append(f"{path} [{source}]: {problem}")
            if (
                support_valid
                and source == "working tree"
                and path != PUBLIC_SUPPORT_CONFIG
                and support_email in data
            ):
                failures.append(
                    f"{path} [{source}]: duplicates the public support address; "
                    f"use {PUBLIC_SUPPORT_CONFIG} or a generated marker"
                )
    return failures


def changed_paths(commit: str) -> list[str]:
    raw = git("diff-tree", "--root", "-m", "--no-commit-id", "--name-only", "-r", "-z", commit)
    return [p.decode("utf-8", "surrogateescape") for p in raw.split(b"\0") if p]


def scan_history(revision_range: str, secrets: list[tuple[str, bytes]]) -> list[str]:
    failures: list[str] = []
    commits = [c.decode() for c in git("rev-list", "--reverse", revision_range).splitlines()]
    seen_states: set[tuple[str, str]] = set()
    for commit in commits:
        message = git("show", "-s", "--format=%B", commit)
        for label in process_labels(message):
            failures.append(f"commit {commit[:12]}: {label}")
        for path in changed_paths(commit):
            state = (commit, path)
            if state in seen_states:
                continue
            seen_states.add(state)
            if path_problem(path):
                failures.append(f"commit {commit[:12]} {path}: forbidden path")
                continue
            result = subprocess.run(
                ["git", "show", f"{commit}:{path}"], cwd=ROOT,
                check=False, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            )
            if result.returncode != 0:  # deleted path or non-file tree entry
                continue
            for problem in content_problems(path, result.stdout, secrets):
                failures.append(f"commit {commit[:12]} {path}: {problem}")
    return failures


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--history-range",
        metavar="REVISION_RANGE",
        help="also scan every changed file state and commit message in this range",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="run synthetic positive and negative tests for process-provenance patterns",
    )
    args = parser.parse_args()

    pattern_failures = process_pattern_test_failures()
    if args.self_test:
        if pattern_failures:
            print("Repository hygiene self-test failed:", file=sys.stderr)
            for failure in pattern_failures:
                print(f"  - {failure}", file=sys.stderr)
            return 1
        print(
            "Repository hygiene self-test passed "
            f"({len(PROCESS_POSITIVE_CASES)} positive, "
            f"{len(PROCESS_NEGATIVE_CASES)} negative cases)."
        )
        return 0

    if pattern_failures:
        print("Repository hygiene pattern tests failed:", file=sys.stderr)
        for failure in pattern_failures:
            print(f"  - {failure}", file=sys.stderr)
        return 1

    secrets = local_secrets()
    failures = scan_current(secrets)
    if args.history_range:
        failures.extend(scan_history(args.history_range, secrets))

    if failures:
        print("Repository hygiene check failed:", file=sys.stderr)
        for failure in sorted(set(failures)):
            print(f"  - {failure}", file=sys.stderr)
        print("Values are intentionally omitted; inspect only the named location.", file=sys.stderr)
        return 1
    print(f"Repository hygiene check passed ({len(tracked_paths())} public-candidate files).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
