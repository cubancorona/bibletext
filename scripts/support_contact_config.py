"""Shared parsing for the tracked public-support mailbox configuration."""

from __future__ import annotations

from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[1]
CONFIG_PATH = ROOT / "config" / "support-email.txt"
PATTERN_PATH = ROOT / "config" / "support-email-pattern.txt"

VALID_EXAMPLES = (
    b"support@example.invalid",
    b"support+site@example.invalid",
    b"support.team+site@sub-domain.example",
)

INVALID_EXAMPLES = (
    b".support@example.invalid",
    b"support.@example.invalid",
    b"support..team@example.invalid",
    b"support@.example.invalid",
    b"support@example..invalid",
    b"support@example.invalid.",
    b"support@-example.invalid",
    b"support@example-.invalid",
    b"support?tag@example.invalid",
    b"support#tag@example.invalid",
    b"support&tag@example.invalid",
    b"support/tag@example.invalid",
    b"support%tag@example.invalid",
    b"support=tag@example.invalid",
    b"support:tag@example.invalid",
    b"support;tag@example.invalid",
    b"support,tag@example.invalid",
)


class SupportContactConfigurationError(ValueError):
    """The tracked mailbox grammar or value is not release-safe."""


def support_email_pattern() -> re.Pattern[bytes]:
    try:
        raw = PATTERN_PATH.read_bytes()
    except OSError as error:
        raise SupportContactConfigurationError(
            "support mailbox grammar is missing or unreadable"
        ) from error
    pattern = raw.rstrip(b"\r\n")
    if not pattern or raw not in {pattern, pattern + b"\n", pattern + b"\r\n"}:
        raise SupportContactConfigurationError(
            "support mailbox grammar must be exactly one non-empty line"
        )
    try:
        return re.compile(rb"(?:" + pattern + rb")\Z")
    except re.error as error:
        raise SupportContactConfigurationError(
            "support mailbox grammar is invalid"
        ) from error


def parse_support_email(raw: bytes) -> bytes:
    value = raw.rstrip(b"\r\n")
    if (
        not value
        or raw not in {value, value + b"\n", value + b"\r\n"}
        or support_email_pattern().fullmatch(value) is None
    ):
        raise SupportContactConfigurationError(
            "support mailbox must contain exactly one conservative ASCII address"
        )
    return value


def read_support_email() -> bytes:
    try:
        raw = CONFIG_PATH.read_bytes()
    except OSError as error:
        raise SupportContactConfigurationError(
            "support mailbox configuration is missing or unreadable"
        ) from error
    return parse_support_email(raw)


def grammar_self_test_failures() -> list[str]:
    failures: list[str] = []
    for index, value in enumerate(VALID_EXAMPLES, 1):
        try:
            parsed = parse_support_email(value + b"\n")
        except SupportContactConfigurationError:
            failures.append(f"valid support-mailbox example {index} was rejected")
            continue
        if parsed != value:
            failures.append(f"valid support-mailbox example {index} changed while parsing")
    for index, value in enumerate(INVALID_EXAMPLES, 1):
        try:
            parse_support_email(value + b"\n")
        except SupportContactConfigurationError:
            continue
        failures.append(f"invalid support-mailbox example {index} was accepted")
    return failures
