#!/usr/bin/env python3
"""Validate and optionally update App Store Connect text metadata.

The default mode is a read-only remote preflight: local files are validated
first, then current App Store Connect values are resolved and compared. No
PATCH is possible without both ``--write`` and an exact ``--confirm-version``.
Use ``--local-only`` for a validation pass that makes no network request.

The app record carries two platforms. The default is IOS; ``--platform
MAC_OS`` targets the Mac version instead, taking its version string from
cmd/desktop/FyneApp.toml and its description, promotional text, and optional
What's New from ``metadata/en-GB/mac/``. Keywords and the URLs fall back to
the shared en-GB files when no Mac-specific file exists, and the app-level
name/subtitle/privacy-URL fields are skipped — they are one per app, not one
per platform, and belong to the default iOS run.

This tool never selects a build, creates a version, uploads screenshots, or
submits anything for review.
"""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
ASC_DIR = os.path.join(REPO, "build", "appstore")
MD = os.path.join(ASC_DIR, "metadata")
SUPPORT_CONTACT_CHECK = os.path.join(REPO, "scripts", "check-support-contact.py")
HYGIENE_CHECK = os.path.join(REPO, "scripts", "check-repository-hygiene.py")
SUPPORT_CONFIG = os.path.join(REPO, "config", "product.json")
APP = "6784567351"
LOCALE = "en-GB"

# Each platform packages its own version string; resolving both from the
# mobile ledger is how the Mac record went unpreflighted.
PLATFORM_VERSION_CONFIG = {
    "IOS": os.path.join("cmd", "mobile", "FyneApp.toml"),
    "MAC_OS": os.path.join("cmd", "desktop", "FyneApp.toml"),
}


# Deliberately no environment override: a forgotten ASC_VERSION must never
# redirect a write toward a historical version record.
def configured_version(platform):
    config = os.path.join(REPO, PLATFORM_VERSION_CONFIG[platform])
    with open(config, encoding="utf-8") as handle:
        for line in handle:
            if line.strip().startswith("Version"):
                return line.split("=", 1)[1].strip().strip('"')
    raise SystemExit(f"Version is missing from {config}")


def read_file(relative, *, required=True):
    path = os.path.join(MD, relative)
    if not os.path.isfile(path):
        if required:
            raise SystemExit(f"missing metadata file: {path}")
        return None
    with open(path, encoding="utf-8") as handle:
        value = handle.read().rstrip("\n")
    if required and not value.strip():
        raise SystemExit(f"metadata file is empty: {path}")
    return value


def read_platform_file(platform, name, *, required=True, shared_fallback=False):
    """A Mac-specific file when the platform is MAC_OS, else the shared one.

    ``shared_fallback`` lets a genuinely platform-independent value (a URL,
    keywords) come from the shared en-GB file when no Mac copy exists; the
    fallback is announced so it is a choice on the record, not a silent reuse.
    """
    if platform != "MAC_OS":
        return read_file(f"{LOCALE}/{name}", required=required)
    value = read_file(f"{LOCALE}/mac/{name}", required=required and not shared_fallback)
    if value is not None:
        return value
    if not shared_fallback:
        return None
    value = read_file(f"{LOCALE}/{name}", required=required)
    if value is not None:
        print(f"note: {LOCALE}/mac/{name} absent; using the shared {LOCALE}/{name}")
    return value


def validate_url(label, value):
    parsed = urllib.parse.urlparse(value)
    if parsed.scheme != "https" or not parsed.netloc:
        raise SystemExit(f"{label} must be an absolute https URL: {value!r}")


def validate_length(label, value, maximum):
    if len(value) > maximum:
        raise SystemExit(f"{label} is {len(value)} characters; maximum is {maximum}")


def load_and_validate(platform, version_string):
    """Load every prospective field before authentication or network access."""
    # A platform's FIRST version has no What's New field in App Store Connect
    # at all, so for MAC_OS the file is optional and an absent one simply
    # omits the field rather than failing or writing a stale iOS text.
    whats_new = read_platform_file(platform, f"whats-new-{version_string}.txt",
                                   required=platform == "IOS")
    version = {
        "description": read_platform_file(platform, "description.txt"),
        "keywords": read_platform_file(platform, "keywords.txt", shared_fallback=True),
        "promotionalText": read_platform_file(platform, "promotional_text.txt"),
        "supportUrl": read_platform_file(platform, "support_url.txt", shared_fallback=True),
        "marketingUrl": read_platform_file(platform, "marketing_url.txt", shared_fallback=True),
    }
    if whats_new is not None:
        version["whatsNew"] = whats_new
    elif platform == "MAC_OS":
        print(f"note: no {LOCALE}/mac/whats-new-{version_string}.txt; "
              "What's New will not be compared or written (a platform's first "
              "version has no such field)")

    # name/subtitle/privacyPolicyUrl live on the app, not on a platform's
    # version. They are validated and written by the default iOS run only, so
    # a Mac run cannot half-own an app-wide value.
    app_info = None
    if platform == "IOS":
        app_info = {
            "name": read_file(f"{LOCALE}/name.txt"),
            "subtitle": read_file(f"{LOCALE}/subtitle.txt"),
            "privacyPolicyUrl": read_file(f"{LOCALE}/privacy_url.txt"),
        }
    copyright_text = read_file("copyright.txt", required=False)

    limits = [
        ("description", version["description"], 4000),
        ("keywords", version["keywords"], 100),
        ("promotional text", version["promotionalText"], 170),
    ]
    if "whatsNew" in version:
        limits.append(("What's New", version["whatsNew"], 4000))
    if app_info is not None:
        limits.append(("name", app_info["name"], 30))
        limits.append(("subtitle", app_info["subtitle"], 30))
    for label, value, limit in limits:
        validate_length(label, value, limit)
    urls = [
        ("support URL", version["supportUrl"]),
        ("marketing URL", version["marketingUrl"]),
    ]
    if app_info is not None:
        urls.append(("privacy URL", app_info["privacyPolicyUrl"]))
    for label, value in urls:
        validate_url(label, value)
    if copyright_text is not None:
        validate_length("copyright", copyright_text, 200)
    return version, app_info, copyright_text


def validate_private_inputs():
    """Reject local metadata containing any known or generic private value."""
    sys.path.insert(0, os.path.join(REPO, "scripts"))
    spec = importlib.util.spec_from_file_location("repository_hygiene", HYGIENE_CHECK)
    if spec is None or spec.loader is None:
        raise SystemExit("cannot load the repository privacy scanner")
    hygiene = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(hygiene)
    secrets = hygiene.local_secrets()
    import json as _json
    support = _json.loads(Path(SUPPORT_CONFIG).read_bytes())["supportEmail"].strip().lower().encode("ascii")

    failures = []
    for path in sorted(Path(MD).rglob("*")):
        if not path.is_file():
            continue
        data = path.read_bytes()
        problems = hygiene.content_problems("appstore-metadata-input.txt", data, secrets)
        foreign_emails = [match.group(0) for match in hygiene.EMAIL.finditer(data)
                          if match.group(0).lower() != support]
        if problems or foreign_emails:
            failures.append(str(path.relative_to(Path(MD))))
    if failures:
        print("App Store metadata privacy check failed:", file=sys.stderr)
        for path in failures:
            print(f"  - {path}", file=sys.stderr)
        print("Values are intentionally omitted; inspect only the named input.", file=sys.stderr)
        raise SystemExit(1)


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--write", action="store_true",
                      help="PATCH changed fields after all preflight checks")
    mode.add_argument("--local-only", action="store_true",
                      help="validate local files without App Store Connect access")
    parser.add_argument(
        "--confirm-version", metavar="VERSION",
        help="required with --write; must exactly equal the configured version",
    )
    parser.add_argument(
        "--platform", choices=sorted(PLATFORM_VERSION_CONFIG), default="IOS",
        help="App Store platform to target (default: IOS); the version string "
             "comes from that platform's FyneApp.toml",
    )
    args = parser.parse_args()
    version = configured_version(args.platform)
    if args.write and args.confirm_version != version:
        parser.error(
            f"--write requires --confirm-version {version}; no remote request was made"
        )
    if args.confirm_version and not args.write:
        parser.error("--confirm-version is meaningful only with --write")
    return args, version


def show_error(label, status, body):
    print(f"[ERROR {status}] {label}")
    rendered = json.dumps(body) if isinstance(body, (dict, list)) else str(body)
    print("   ", rendered[:800])


def get_data(asc, path, label):
    status, body = asc.request("GET", path)
    if not 200 <= status < 300 or not isinstance(body, dict):
        show_error(label, status, body)
        raise SystemExit(1)
    data = body.get("data")
    if not isinstance(data, list):
        raise SystemExit(f"{label}: response did not contain a data list")
    return data


def one(items, label):
    if len(items) != 1:
        raise SystemExit(f"expected exactly one {label}; found {len(items)}")
    return items[0]


def changed(proposed, current):
    return {key: value for key, value in proposed.items()
            if (current.get(key) or "") != value}


def print_plan(label, changes, current):
    print(f"\n{label}")
    if not changes:
        print("  no changes")
        return
    for key, value in changes.items():
        old = current.get(key) or ""
        print(f"  {key}: {len(old)} -> {len(value)} characters")
        print(f"    current:  {old!r}")
        print(f"    proposed: {value!r}")


def main():
    args, version = parse_args()
    platform = args.platform
    contact = subprocess.run([sys.executable, SUPPORT_CONTACT_CHECK], cwd=REPO)
    if contact.returncode != 0:
        raise SystemExit("public support configuration is not release-safe")
    version_values, app_info_values, copyright_text = load_and_validate(platform, version)
    validate_private_inputs()
    print(f"local metadata preflight: OK ({LOCALE}, {platform} version {version})")
    if args.local_only:
        print("local-only mode: no App Store Connect request made")
        return 0

    # Import only after the entirely local preflight. Importing does not make a
    # request; each call below is visibly GET or, behind --write, PATCH.
    if not os.path.isfile(os.path.join(ASC_DIR, "asc.py")):
        raise SystemExit(
            "missing build/appstore/asc.py (local App Store tooling is ignored; "
            "see docs/APP_STORE_SUBMISSION.md)"
        )
    sys.path.insert(0, ASC_DIR)
    import asc  # pylint: disable=import-outside-toplevel

    query = urllib.parse.urlencode({
        "filter[platform]": platform,
        "filter[versionString]": version,
        "limit": "10",
    })
    version_record = one(
        get_data(asc, f"/v1/apps/{APP}/appStoreVersions?{query}", "resolve version"),
        f"{platform} App Store version {version!r}",
    )
    version_id = version_record["id"]
    version_state = version_record.get("attributes", {}).get("appStoreState", "UNKNOWN")

    localizations = get_data(
        asc, f"/v1/appStoreVersions/{version_id}/appStoreVersionLocalizations?limit=200",
        "resolve version localization",
    )
    localization = one(
        [item for item in localizations
         if item.get("attributes", {}).get("locale") == LOCALE],
        f"{LOCALE} version localization",
    )

    version_changes = changed(version_values, localization.get("attributes", {}))
    record_values = {} if copyright_text is None else {"copyright": copyright_text}
    copyright_changes = changed(record_values, version_record.get("attributes", {}))

    print(f"target: app {APP}, {platform} {version}, {LOCALE}, state {version_state}")
    print_plan("version localization", version_changes, localization.get("attributes", {}))
    print_plan("version record", copyright_changes, version_record.get("attributes", {}))

    plans = [
        ("version localization", f"/v1/appStoreVersionLocalizations/{localization['id']}",
         "appStoreVersionLocalizations", localization["id"], version_changes),
        ("version record", f"/v1/appStoreVersions/{version_id}",
         "appStoreVersions", version_id, copyright_changes),
    ]

    if app_info_values is None:
        print("\napp-info localization: skipped (app-wide fields; the iOS run owns them)")
    else:
        app_info = one(
            get_data(asc, f"/v1/apps/{APP}/appInfos?limit=20", "resolve app info"),
            "appInfo record",
        )
        app_info_localizations = get_data(
            asc, f"/v1/appInfos/{app_info['id']}/appInfoLocalizations?limit=200",
            "resolve app info localization",
        )
        app_info_localization = one(
            [item for item in app_info_localizations
             if item.get("attributes", {}).get("locale") == LOCALE],
            f"{LOCALE} appInfo localization",
        )
        app_info_changes = changed(app_info_values, app_info_localization.get("attributes", {}))
        print_plan("app-info localization", app_info_changes,
                   app_info_localization.get("attributes", {}))
        plans.append(
            ("app-info localization", f"/v1/appInfoLocalizations/{app_info_localization['id']}",
             "appInfoLocalizations", app_info_localization["id"], app_info_changes),
        )

    plans = [plan for plan in plans if plan[4]]

    if not args.write:
        print("\nDRY RUN: no PATCH made. Re-run with --write and "
              f"--confirm-version {version} only after reviewing this plan.")
        return 0
    if version_state not in {
        "PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED"
    }:
        raise SystemExit(
            f"refusing to write version in {version_state}; expected an editable preparation state"
        )
    if not plans:
        print("\nNo changed fields; no PATCH made.")
        return 0

    for label, path, resource_type, resource_id, attributes in plans:
        status, body = asc.request("PATCH", path, {
            "data": {"type": resource_type, "id": resource_id,
                     "attributes": attributes}
        })
        if not 200 <= status < 300:
            show_error(label, status, body)
            raise SystemExit("write stopped; inspect App Store Connect before retrying")
        print(f"[OK] {label}")

    # Read back each changed resource and verify exactly the fields written.
    for label, path, _resource_type, _resource_id, attributes in plans:
        status, body = asc.request("GET", path)
        actual = body.get("data", {}).get("attributes", {}) if isinstance(body, dict) else {}
        mismatch = {key: (value, actual.get(key)) for key, value in attributes.items()
                    if actual.get(key) != value}
        if not 200 <= status < 300 or mismatch:
            show_error(f"read-back {label}", status, body)
            raise SystemExit(f"read-back mismatch: {mismatch}")
        print(f"[OK] read-back {label}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
