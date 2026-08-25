#!/usr/bin/env python3
"""Read-only App Store Connect state and metadata-consistency preflight.

App Store Connect initializes a new version with populated fields copied from
the previous version. A plausible value is therefore not proof that a
release-specific field was reviewed for the current version. This tool compares
adjacent versions and reports release-specific fields that did not change.

READ-ONLY — it makes no PATCH, POST or DELETE. Safe to run at any time,
including while a version is in review.

    ASC_KEY_PATH=~/.private_keys/AuthKey_XXXX.p8 \\
    ASC_KEY_ID=XXXX ASC_ISSUER_ID=... \\
    python3 appstore/preflight.py

Exit status is 1 if anything that must be release-specific was inherited, so it
can gate a submission step.
"""
import json
import os
import subprocess
import sys
import urllib.parse

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
ASC = os.path.join(REPO, "build", "appstore", "asc.py")
SUPPORT_CONTACT_CHECK = os.path.join(REPO, "scripts", "check-support-contact.py")
APP_ID = "6784567351"
LOCALE = "en-GB"


def configured_version():
    config = os.path.join(REPO, "cmd", "mobile", "FyneApp.toml")
    with open(config, encoding="utf-8") as handle:
        for line in handle:
            if line.strip().startswith("Version"):
                return line.split("=", 1)[1].strip().strip('"')
    raise SystemExit(f"Version is missing from {config}")


VERSION = configured_version()

# Fields that MUST be rewritten for every release. Anything here that is
# identical to the previous version was inherited, not written.
PER_RELEASE = {"whatsNew", "review notes", "screenshots"}

# Fields that legitimately stay the same release to release. Listed so the
# report can say "unchanged, and that is fine" rather than staying silent.
STABLE = {"description", "keywords", "marketingUrl", "supportUrl"}

# A stable description needs a content check rather than a change check. Every
# translation a reader can open should be named on the store page. These are the
# translations registeredVersions (versions.go) serves today.
# Each entry: (name-for-the-report, tuple of phrasings that count as naming it).
# Alternate phrasings keep the advisory focused on content, not exact wording.
DESCRIBED_TRANSLATIONS = [
    ("World English Bible", ("world english bible",), "public domain, always available"),
    ("Berean Standard Bible", ("berean standard bible",), "public domain, always available"),
    ("World English Bible (Catholic)", ("world english bible (catholic)", "catholic edition"),
     "public domain, the 73-book canon"),
    ("New King James Version", ("new king james version",), "licensed, works on install"),
]

# Features big enough that a reader deciding whether to install would want them
# on the page. Kept short on purpose: this is not a changelog.
# The needles are deliberately specific so unrelated uses of "notes" do not
# satisfy the shared-notes check.
DESCRIBED_FEATURES = [
    ("shared notes", ("shared note", "note attached", "verse with a note", "send someone a verse",
                      "notes from friends", "note of your own")),
    ("audio / read-along narration", ("audio", "narration", "listen", "read-along", "read along")),
]


def asc(path):
    if not os.path.exists(ASC):
        sys.exit(f"cannot find {ASC} (build/ is gitignored — see docs/APP_STORE_SUBMISSION.md)")
    out = subprocess.run([sys.executable, ASC, "get", path], capture_output=True, text=True)
    if "{" not in out.stdout:
        sys.exit(f"App Store Connect call failed for {path}:\n{out.stdout}\n{out.stderr}")
    return json.loads(out.stdout[out.stdout.index("{"):])


def verify_support_contact():
    """Stop before store access if public contact consumers have diverged."""
    result = subprocess.run([sys.executable, SUPPORT_CONTACT_CHECK], cwd=REPO)
    if result.returncode != 0:
        sys.exit("public support configuration is not release-safe")


def version_fields(vid):
    """Every per-release field of one version, in a comparable shape."""
    f = {}
    localizations = asc(
        f"/v1/appStoreVersions/{vid}/appStoreVersionLocalizations?limit=200"
    )["data"]
    matches = [item for item in localizations
               if item.get("attributes", {}).get("locale") == LOCALE]
    if len(matches) != 1:
        sys.exit(f"expected exactly one {LOCALE} localization; found {len(matches)}")
    loc = matches[0]
    a = loc["attributes"]
    for k in ("whatsNew", "description", "keywords", "promotionalText", "marketingUrl", "supportUrl"):
        f[k] = (a.get(k) or "").strip()

    detail = asc(f"/v1/appStoreVersions/{vid}/appStoreReviewDetail").get("data")
    f["review notes"] = ((detail or {}).get("attributes", {}).get("notes") or "").strip()

    shots = {}
    for s in asc(f"/v1/appStoreVersionLocalizations/{loc['id']}/appScreenshotSets")["data"]:
        imgs = asc(f"/v1/appScreenshotSets/{s['id']}/appScreenshots")["data"]
        shots[s["attributes"]["screenshotDisplayType"]] = [
            (i["attributes"].get("fileName"), i["attributes"].get("sourceFileChecksum")) for i in imgs
        ]
    f["screenshots"] = json.dumps(shots, sort_keys=True)
    return f


# States that mean Apple currently owns the version: a second version cannot be
# taken into review while one of these is outstanding, so a release plan built
# without checking is a plan that cannot be executed.
IN_REVIEW_STATES = {
    "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_APPLE_RELEASE",
    "PENDING_DEVELOPER_RELEASE", "PROCESSING_FOR_APP_STORE",
}


def report_live_states(data):
    """Print every version's live state and flag anything blocking a submission.

    App Store version states change asynchronously and can invalidate a release
    plan between local preparation and submission. Querying the live state first
    prevents later steps from relying on an earlier observation.

    Field comparison does not establish whether another version is already in
    review, so the live state is reported independently.
    """
    print("live version states")
    blocking = []
    for v in data:
        a = v["attributes"]
        state = a.get("appStoreState", "?")
        mark = "  <- Apple has this one" if state in IN_REVIEW_STATES else ""
        print(f"   {a.get('versionString',''):8} {state}{mark}")
        if state in IN_REVIEW_STATES:
            blocking.append((a.get("versionString", ""), state))
    if blocking:
        names = ", ".join(f"{v} ({s})" for v, s in blocking)
        print(f"\n   BLOCKING: {names}.")
        print("   App Store Connect takes one version into review at a time, so a new")
        print("   version cannot be submitted until this clears or is removed from review.")
    else:
        print("\n   nothing is with Apple; a new version can be submitted.")
    print()
    return blocking


def version_tuple(value):
    try:
        parts = tuple(int(part) for part in value.split("."))
    except (AttributeError, ValueError):
        raise SystemExit(f"invalid App Store version string: {value!r}")
    if len(parts) != 3:
        raise SystemExit(f"invalid App Store version string: {value!r}")
    return parts


def resolve_versions(data):
    """Resolve the configured iOS record and its nearest earlier version."""
    current = [item for item in data
               if item.get("attributes", {}).get("versionString") == VERSION]
    if len(current) != 1:
        raise SystemExit(
            f"expected exactly one iOS App Store version {VERSION}; found {len(current)}"
        )
    target_tuple = version_tuple(VERSION)
    earlier = [
        item for item in data
        if version_tuple(item.get("attributes", {}).get("versionString", "")) < target_tuple
    ]
    if not earlier:
        raise SystemExit(f"no earlier iOS version exists for comparison with {VERSION}")
    previous = max(
        earlier,
        key=lambda item: version_tuple(item["attributes"]["versionString"]),
    )
    return current[0], previous


def main():
    verify_support_contact()
    query = urllib.parse.urlencode({"filter[platform]": "IOS", "limit": "200"})
    versions = asc(f"/v1/apps/{APP_ID}/appStoreVersions?{query}")["data"]
    blocking = report_live_states(versions)
    cur, prev = resolve_versions(versions)
    cv = cur["attributes"].get("versionString")
    pv = prev["attributes"].get("versionString")
    print(f"comparing {cv} ({cur['attributes'].get('appStoreState')}) against {pv}\n")

    now, before = version_fields(cur["id"]), version_fields(prev["id"])

    problems = []
    for key in sorted(set(now) | set(before)):
        a, b = now.get(key, ""), before.get(key, "")
        if not a:
            status = "EMPTY"
        elif a == b:
            status = "INHERITED from " + pv
        else:
            status = "written for " + cv
        flag = " "
        if key in PER_RELEASE and a == b:
            flag = "!"
            problems.append(key)
        elif key in PER_RELEASE and not a:
            flag = "!"
            problems.append(key)
        note = ""
        if key in STABLE and a == b:
            note = "  (fine — this one is meant to be stable)"
        print(f" {flag} {key:16} {status}{note}")

    # The description check is advisory, never fatal: this can identify text
    # that looks absent but cannot decide the intended marketing copy.
    desc = now.get("description", "")
    missing_t = [t for t, needles, _ in DESCRIBED_TRANSLATIONS
                 if not any(n in desc.lower() for n in needles)]
    missing_f = [name for name, needles in DESCRIBED_FEATURES
                 if not any(n in desc.lower() for n in needles)]
    if missing_t or missing_f:
        print("\nTHE DESCRIPTION MAY HAVE FALLEN BEHIND THE APP:")
        whys = {t: why for t, _, why in DESCRIBED_TRANSLATIONS}
        for t in missing_t:
            print(f"  - does not name {t} ({whys[t]})")
        for f in missing_f:
            print(f"  - says nothing about {f}")
        print(
            "  The description is the one field that SHOULD stay still between\n"
            "  releases, so the inherited/written check above can never catch this.\n"
            "  Not fatal — the copy is yours to write — but worth a look."
        )

    if problems:
        print("\nPER-RELEASE FIELDS THAT WERE NOT WRITTEN FOR THIS RELEASE:")
        for p in problems:
            print(f"  - {p}")
        print(
            "\nThese values match the previous version and therefore require explicit review.\n"
            "Fix them before submitting (docs/APP_STORE_SUBMISSION.md)."
        )
    if blocking:
        return 1
    if problems:
        return 1
    print("\nevery per-release field was written for this release.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
