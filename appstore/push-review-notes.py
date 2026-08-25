#!/usr/bin/env python3
"""Preview or update App Review notes for the pinned iOS release.

The tracked review-notes file is the source of truth. Remote access is
read-only by default. A write requires both ``--write`` and the exact pinned
version confirmation, and is allowed only while that version remains editable.

Examples:

    python3 appstore/push-review-notes.py --local-only
    python3 appstore/push-review-notes.py
    python3 appstore/push-review-notes.py --write --confirm-version 1.2.3
"""

import argparse
import json
import os
import re
import subprocess
import sys
import urllib.parse


HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
NOTES = os.path.join(HERE, "review-notes.txt")
VERSION_CONFIG = os.path.join(REPO, "cmd", "mobile", "FyneApp.toml")
ASC = os.path.join(REPO, "build", "appstore", "asc.py")
SUPPORT_CONTACT_CHECK = os.path.join(REPO, "scripts", "check-support-contact.py")
REPOSITORY_HYGIENE_CHECK = os.path.join(REPO, "scripts", "check-repository-hygiene.py")

# This helper intentionally has no app-ID or version override. Changing either
# target requires a tracked, reviewable source change.
APP_ID = "6784567351"
TARGET_VERSION = "1.2.3"

# Review notes may be changed only while App Store Connect considers the
# version editable.
EDITABLE_STATES = {
    "PREPARE_FOR_SUBMISSION",
    "DEVELOPER_REJECTED",
    "REJECTED",
    "METADATA_REJECTED",
    "INVALID_BINARY",
}


def fail(message):
    raise SystemExit(message)


def verify_tracked_version():
    try:
        with open(VERSION_CONFIG, encoding="utf-8") as source:
            config = source.read()
    except OSError as exc:
        fail(f"cannot read {VERSION_CONFIG}: {exc}")

    match = re.search(
        r'^\s*Version\s*=\s*"([0-9]+(?:\.[0-9]+){2})"\s*$',
        config,
        re.MULTILINE,
    )
    if not match:
        fail(f"cannot determine the packaged version from {VERSION_CONFIG}")
    configured = match.group(1)
    if configured != TARGET_VERSION:
        fail(
            f"writer target is {TARGET_VERSION}, but {VERSION_CONFIG} packages "
            f"{configured}; update and review the pinned target before remote access"
        )


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument(
        "--write",
        action="store_true",
        help="write the tracked notes after all safety checks pass",
    )
    mode.add_argument(
        "--local-only",
        action="store_true",
        help="validate tracked inputs and gates without authenticating",
    )
    parser.add_argument(
        "--confirm-version",
        metavar="VERSION",
        help="required exact target confirmation for --write",
    )
    args = parser.parse_args(argv)

    if args.write and args.confirm_version != TARGET_VERSION:
        parser.error(
            f"--write requires --confirm-version {TARGET_VERSION}; "
            "no remote request was made"
        )
    if args.confirm_version is not None and not args.write:
        parser.error("--confirm-version is valid only with --write")
    return args


def load_notes():
    try:
        with open(NOTES, encoding="utf-8") as source:
            notes = source.read().rstrip("\n")
    except OSError as exc:
        fail(f"cannot read {NOTES}: {exc}")

    if not notes.strip():
        fail(f"{NOTES} is empty; refusing to blank the review notes")
    if len(notes) > 4000:
        fail(
            f"{NOTES} is {len(notes)} characters; App Store Connect caps "
            f"review notes at 4,000. Remove {len(notes) - 4000} characters."
        )
    heading = notes.splitlines()[0].strip()
    if not re.match(rf"^VERSION {re.escape(TARGET_VERSION)}(?:\s|$)", heading):
        fail(
            f"{NOTES} must open with VERSION {TARGET_VERSION}; "
            f"found {heading!r}"
        )
    return notes


def run_gate(path, failure_message):
    result = subprocess.run([sys.executable, path], cwd=REPO)
    if result.returncode != 0:
        fail(failure_message)


def run_repository_gates():
    # These run before asc.py, so a failing local invariant cannot authenticate
    # or reach App Store Connect.
    run_gate(SUPPORT_CONTACT_CHECK, "public support configuration is not release-safe")
    run_gate(REPOSITORY_HYGIENE_CHECK, "repository hygiene checks failed")


def api_error(status, body):
    if isinstance(body, dict) and body.get("errors"):
        first = body["errors"][0]
        title = first.get("title") or "request rejected"
        detail = first.get("detail") or "no detail supplied"
        code = first.get("code") or "unknown code"
        fail(f"App Store Connect returned HTTP {status}: {title}: {detail} ({code})")
    fail(f"App Store Connect returned HTTP {status}")


def api_request(method, path, payload=None):
    command = [sys.executable, ASC, method.lower(), path]
    if payload is not None:
        command.append(json.dumps(payload, separators=(",", ":")))
    result = subprocess.run(command, capture_output=True, text=True)
    if result.returncode != 0:
        fail(f"App Store Connect client failed: {result.stderr.strip() or 'unknown error'}")

    lines = result.stdout.splitlines()
    if not lines or not re.fullmatch(r"[0-9]{3}", lines[0].strip()):
        fail("App Store Connect client returned an unreadable response")
    status = int(lines[0])
    encoded = "\n".join(lines[1:]).strip()
    try:
        body = json.loads(encoded) if encoded else None
    except json.JSONDecodeError:
        fail("App Store Connect client returned a non-JSON response")
    if not 200 <= status < 300:
        api_error(status, body)
    if isinstance(body, dict) and body.get("errors"):
        api_error(status, body)
    return body


def document_data(body, description):
    if not isinstance(body, dict) or not isinstance(body.get("data"), dict):
        fail(f"App Store Connect returned no {description}")
    return body["data"]


def exact_version():
    query = urllib.parse.urlencode(
        {
            "filter[platform]": "IOS",
            "filter[versionString]": TARGET_VERSION,
            "limit": "10",
        }
    )
    body = api_request("GET", f"/v1/apps/{APP_ID}/appStoreVersions?{query}")
    data = body.get("data") if isinstance(body, dict) else None
    if not isinstance(data, list):
        fail("App Store Connect returned no version list")
    matches = [
        version
        for version in data
        if version.get("attributes", {}).get("versionString") == TARGET_VERSION
        and version.get("attributes", {}).get("platform") == "IOS"
    ]
    if len(matches) != 1:
        fail(
            f"expected exactly one iOS {TARGET_VERSION} version record; "
            f"found {len(matches)}"
        )
    return matches[0]


def main(argv=None):
    args = parse_args(argv)
    verify_tracked_version()
    notes = load_notes()
    run_repository_gates()

    if args.local_only:
        print(f"local review-notes preflight: OK (version {TARGET_VERSION})")
        print("local-only mode: no App Store Connect request made")
        return 0

    if not os.path.isfile(ASC):
        fail(
            f"cannot find {ASC}; the local App Store Connect client is described "
            "in docs/APP_STORE_SUBMISSION.md"
        )

    version = exact_version()
    attributes = version.get("attributes", {})
    state = attributes.get("appStoreState") or "UNKNOWN"
    print(f"target version: {TARGET_VERSION}  platform: IOS  state: {state}")

    detail = document_data(
        api_request(
            "GET",
            f"/v1/appStoreVersions/{version['id']}/appStoreReviewDetail",
        ),
        "App Review detail",
    )
    current = detail.get("attributes", {}).get("notes") or ""
    print("\n--- notes currently in App Store Connect ---")
    print(current if current.strip() else "(EMPTY)")
    print("--- end ---\n")

    same = current.rstrip("\n") == notes
    if not args.write:
        if same:
            print("in sync with appstore/review-notes.txt; preview only, no PATCH made")
        else:
            print("DIFFERENT from appstore/review-notes.txt; preview only, no PATCH made")
            print(
                "To write, re-run with "
                f"--write --confirm-version {TARGET_VERSION}."
            )
        return 0

    if state not in EDITABLE_STATES:
        fail(
            f"refusing to write: iOS {TARGET_VERSION} is {state}, which is not "
            "an editable App Store state"
        )
    if same:
        print("already in sync; no PATCH needed")
        return 0

    detail_id = detail.get("id")
    if not detail_id:
        fail("App Review detail has no resource ID; refusing to write")
    payload = {
        "data": {
            "type": "appStoreReviewDetails",
            "id": detail_id,
            "attributes": {"notes": notes},
        }
    }
    api_request("PATCH", f"/v1/appStoreReviewDetails/{detail_id}", payload)

    read_back = document_data(
        api_request("GET", f"/v1/appStoreReviewDetails/{detail_id}"),
        "App Review detail read-back",
    )
    written = read_back.get("attributes", {}).get("notes") or ""
    if written.rstrip("\n") != notes:
        fail("read-back mismatch: App Store Connect does not hold the tracked notes")
    print("written and read back identical")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
