#!/usr/bin/env python3
"""Read-only App Store Connect state and metadata-consistency preflight.

App Store Connect initializes a new version with populated fields copied from
the previous version — and it seeds a platform's FIRST version from the app's
existing metadata on the other platform. A plausible value is therefore not
proof that a release-specific field was reviewed for the current version. This
tool compares the configured version against its predecessors and reports
release-specific fields that did not change.

The app record carries two platforms. The default is IOS; pass
``--platform MAC_OS`` to preflight the Mac version, whose version string comes
from cmd/desktop/FyneApp.toml rather than cmd/mobile/FyneApp.toml.

Screenshots get two real checks beyond the copy-forward diff:

- every uploaded image's assetDeliveryState must be COMPLETE — a wrong-sized
  upload is accepted by the API and then sits at FAILED with no error at
  upload time; and
- the local upload-ready set for this version, when present, must use pixel
  sizes Apple accepts for the platform and must carry no alpha channel
  (App Store Connect refuses a PNG with one).

READ-ONLY — it makes no PATCH, POST or DELETE. Safe to run at any time,
including while a version is in review.

    ASC_KEY_PATH=~/.private_keys/AuthKey_XXXX.p8 \\
    ASC_KEY_ID=XXXX ASC_ISSUER_ID=... \\
    python3 appstore/preflight.py [--platform MAC_OS]

Exit status is 1 if anything that must be release-specific was inherited, so it
can gate a submission step.
"""
import argparse
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

# Each platform packages its version independently; preflighting one against
# the other's FyneApp.toml is how the Mac record went unexamined for so long.
PLATFORM_VERSION_CONFIG = {
    "IOS": os.path.join("cmd", "mobile", "FyneApp.toml"),
    "MAC_OS": os.path.join("cmd", "desktop", "FyneApp.toml"),
}


def configured_version(platform):
    config = os.path.join(REPO, PLATFORM_VERSION_CONFIG[platform])
    with open(config, encoding="utf-8") as handle:
        for line in handle:
            if line.strip().startswith("Version"):
                return line.split("=", 1)[1].strip().strip('"')
    raise SystemExit(f"Version is missing from {config}")


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

# The pixel sizes App Store Connect accepts for a screenshot set, per platform.
# An off-list upload does not error — it reaches assetDeliveryState FAILED
# later, silently — so the local set is checked before it ever goes up.
# iOS sizes are the 6.9" iPhone and 13" iPad slots (everything smaller
# auto-scales from those) in both orientations; Mac sizes are landscape only.
_IOS_PORTRAIT_SIZES = {
    (1320, 2868), (1290, 2796), (1260, 2736),  # iPhone 6.9"
    (2064, 2752), (2048, 2732),                # iPad 13"
}
ACCEPTED_SCREENSHOT_SIZES = {
    "IOS": _IOS_PORTRAIT_SIZES | {(h, w) for (w, h) in _IOS_PORTRAIT_SIZES},
    "MAC_OS": {(1280, 800), (1440, 900), (2560, 1600), (2880, 1800)},
}


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


def png_inspect(path):
    """(width, height, has_alpha) for a PNG, or None if the file is not one.

    Pure chunk walk — no image library. Alpha means an RGBA/greyscale-alpha
    color type or a tRNS transparency chunk; either one makes App Store
    Connect refuse the upload.
    """
    with open(path, "rb") as handle:
        data = handle.read()
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        return None
    width = int.from_bytes(data[16:20], "big")
    height = int.from_bytes(data[20:24], "big")
    color_type = data[25]
    has_alpha = color_type in (4, 6)
    offset = 8
    while not has_alpha and offset + 8 <= len(data):
        length = int.from_bytes(data[offset:offset + 4], "big")
        if data[offset + 4:offset + 8] == b"tRNS":
            has_alpha = True
        offset += 12 + length
    return width, height, has_alpha


def check_local_screenshots(platform, version):
    """Validate the upload-ready set on disk before it can fail at Apple.

    Returns problem strings; an absent set is advisory only, because the live
    listing may already hold images uploaded from elsewhere — the remote
    delivery-state check covers those.
    """
    base = os.path.join(REPO, "build", "appstore",
                        f"screenshots-ready-{version}", "en-GB")
    if platform == "MAC_OS":
        directories = [os.path.join(base, "mac")]
    else:
        directories = [base, os.path.join(base, "ipad13")]

    accepted = ACCEPTED_SCREENSHOT_SIZES[platform]
    problems, checked = [], 0
    print(f"local upload-ready screenshots ({platform})")
    for directory in directories:
        if not os.path.isdir(directory):
            print(f"   {os.path.relpath(directory, REPO)}: absent — nothing local to validate")
            continue
        for name in sorted(os.listdir(directory)):
            path = os.path.join(directory, name)
            if not os.path.isfile(path):
                continue
            info = png_inspect(path)
            rel = os.path.relpath(path, REPO)
            if info is None:
                print(f"   {rel}: not a PNG — dimensions and alpha NOT validated here")
                continue
            checked += 1
            width, height, has_alpha = info
            if (width, height) not in accepted:
                sizes = ", ".join(f"{w}x{h}" for w, h in sorted(accepted))
                problems.append(
                    f"{rel} is {width}x{height}; {platform} accepts only {sizes}. "
                    "The upload would sit at assetDeliveryState FAILED."
                )
            if has_alpha:
                problems.append(
                    f"{rel} carries an alpha channel; App Store Connect refuses it. "
                    "Redraw opaque (the -ready copies exist for exactly this)."
                )
    if checked and not problems:
        print(f"   {checked} PNGs validated: accepted {platform} sizes, no alpha channel.")
    for problem in problems:
        print(f" ! {problem}")
    print()
    return problems


def version_fields(vid):
    """One version's per-release fields, plus any failed screenshot uploads.

    Returns (fields, delivery_problems). The delivery list matters only for
    the version under preparation; callers ignore it for baselines.
    """
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

    shots, delivery = {}, []
    for s in asc(f"/v1/appStoreVersionLocalizations/{loc['id']}/appScreenshotSets")["data"]:
        display = s["attributes"]["screenshotDisplayType"]
        imgs = asc(f"/v1/appScreenshotSets/{s['id']}/appScreenshots")["data"]
        shots[display] = [
            (i["attributes"].get("fileName"), i["attributes"].get("sourceFileChecksum")) for i in imgs
        ]
        for i in imgs:
            state = ((i["attributes"].get("assetDeliveryState") or {}).get("state")) or "UNKNOWN"
            if state != "COMPLETE":
                delivery.append(
                    f"{display} {i['attributes'].get('fileName')}: assetDeliveryState {state}"
                )
    f["screenshots"] = json.dumps(shots, sort_keys=True)
    return f, delivery


# States that mean Apple currently owns the version: a second version cannot be
# taken into review while one of these is outstanding, so a release plan built
# without checking is a plan that cannot be executed.
IN_REVIEW_STATES = {
    "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_APPLE_RELEASE",
    "PENDING_DEVELOPER_RELEASE", "PROCESSING_FOR_APP_STORE",
}


def report_live_states(data, platform):
    """Print every version's live state and flag anything blocking a submission.

    App Store version states change asynchronously and can invalidate a release
    plan between local preparation and submission. Querying the live state first
    prevents later steps from relying on an earlier observation.

    Field comparison does not establish whether another version is already in
    review, so the live state is reported independently. Review pipelines are
    per platform: an iOS version with Apple does not block a Mac submission.
    """
    print(f"live version states ({platform})")
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
        print("   App Store Connect takes one version per platform into review at a")
        print(f"   time, so a new {platform} version cannot be submitted until this")
        print("   clears or is removed from review.")
    else:
        print(f"\n   nothing is with Apple on {platform}; a new version can be submitted.")
    print()
    return blocking


def version_tuple(value):
    try:
        parts = tuple(int(part) for part in value.split("."))
    except (AttributeError, ValueError):
        raise SystemExit(f"invalid App Store version string: {value!r}")
    if not 1 <= len(parts) <= 3:
        raise SystemExit(f"invalid App Store version string: {value!r}")
    # Apple's own history is not uniformly three-part (the first iOS release
    # is "1.0"), so short versions order as if zero-padded.
    return parts + (0,) * (3 - len(parts))


def resolve_current(data, platform, version):
    current = [item for item in data
               if item.get("attributes", {}).get("versionString") == version]
    if len(current) != 1:
        raise SystemExit(
            f"expected exactly one {platform} App Store version {version}; found {len(current)}"
        )
    return current[0]


def resolve_baselines(data, platform, version):
    """The versions the configured record must differ from, as (label, id).

    Normally that is the nearest earlier version on the same platform. A
    platform's FIRST version has no such predecessor — App Store Connect seeds
    it from the app's OTHER platform instead (the Mac 1.2.3 record arrived
    carrying the live iOS version's review notes verbatim), so the recent
    versions over there become the baselines and a matching field is
    cross-platform inheritance, not authorship.
    """
    target = version_tuple(version)
    earlier = [
        item for item in data
        if version_tuple(item.get("attributes", {}).get("versionString", "")) < target
    ]
    if earlier:
        previous = max(
            earlier,
            key=lambda item: version_tuple(item["attributes"]["versionString"]),
        )
        return [(previous["attributes"]["versionString"], previous["id"])], False

    other = "IOS" if platform == "MAC_OS" else "MAC_OS"
    query = urllib.parse.urlencode({"filter[platform]": other, "limit": "200"})
    candidates = asc(f"/v1/apps/{APP_ID}/appStoreVersions?{query}")["data"]
    candidates.sort(
        key=lambda item: version_tuple(item["attributes"]["versionString"]),
        reverse=True,
    )
    if not candidates:
        raise SystemExit(
            f"no earlier {platform} version and no {other} version exists; "
            f"nothing to compare {version} against"
        )
    print(f"no earlier {platform} version exists. App Store Connect seeds a platform's")
    print(f"first version from the app's {other} metadata, so the newest {other} versions")
    print("serve as the baselines: a matching field was inherited across platforms.\n")
    return [(f"{other} {item['attributes']['versionString']}", item["id"])
            for item in candidates[:3]], True


def parse_args():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--platform", choices=sorted(PLATFORM_VERSION_CONFIG), default="IOS",
        help="App Store platform to preflight (default: IOS); the version "
             "string comes from that platform's FyneApp.toml",
    )
    parser.add_argument(
        "--carried-forward", metavar="FIELD=REASON", action="append", default=[],
        help="Record that FIELD is deliberately unchanged from the previous "
             "release, with the reason it is still correct — e.g. "
             "--carried-forward 'screenshots=1.2.4 changes nothing they show'. "
             "The field still has to be one this run actually flagged, and the "
             "reason is printed and stored in the submission log. A corrective "
             "release that alters no visible surface is the honest case for "
             "this; wanting the gate to go green is not.",
    )
    return parser.parse_args()


def main():
    args = parse_args()
    platform = args.platform
    version = configured_version(platform)
    verify_support_contact()
    query = urllib.parse.urlencode({"filter[platform]": platform, "limit": "200"})
    versions = asc(f"/v1/apps/{APP_ID}/appStoreVersions?{query}")["data"]
    blocking = report_live_states(versions, platform)
    cur = resolve_current(versions, platform, version)
    baselines, first_on_platform = resolve_baselines(versions, platform, version)
    cv = cur["attributes"].get("versionString")
    against = ", ".join(label for label, _ in baselines)
    print(f"comparing {platform} {cv} ({cur['attributes'].get('appStoreState')}) against {against}\n")

    now, delivery = version_fields(cur["id"])
    befores = [(label, version_fields(vid)[0]) for label, vid in baselines]

    problems = []
    keys = set(now)
    for _, before in befores:
        keys |= set(before)
    for key in sorted(keys):
        a = now.get(key, "")
        matched = next((label for label, before in befores
                        if a and before.get(key, "") == a), None)
        if key == "whatsNew" and first_on_platform and not a:
            # A platform's FIRST version has no What's New field in App Store
            # Connect at all; an empty read-back is the correct state, not an
            # unwritten one.
            print(f"   {key:16} not applicable (a platform's first version has no What's New)")
            continue
        if not a:
            status = "EMPTY"
        elif matched:
            status = "INHERITED from " + matched
        else:
            status = "written for " + cv
        flag = " "
        if key in PER_RELEASE and (matched or not a):
            flag = "!"
            problems.append(key)
        note = ""
        if key in STABLE and matched:
            note = "  (fine — this one is meant to be stable)"
        print(f" {flag} {key:16} {status}{note}")
    print()

    if delivery:
        print("SCREENSHOT UPLOADS THAT NEVER ARRIVED:")
        for item in delivery:
            print(f" ! {item}")
        print(
            "  An upload App Store Connect cannot use (wrong pixel size, alpha channel)\n"
            "  is accepted by the API and then sits in this state with no error at\n"
            "  upload time. COMPLETE is the only good answer; replace these.\n"
        )

    local_problems = check_local_screenshots(platform, version)

    # The description check is advisory, never fatal: this can identify text
    # that looks absent but cannot decide the intended marketing copy.
    desc = now.get("description", "")
    missing_t = [t for t, needles, _ in DESCRIBED_TRANSLATIONS
                 if not any(n in desc.lower() for n in needles)]
    missing_f = [name for name, needles in DESCRIBED_FEATURES
                 if not any(n in desc.lower() for n in needles)]
    if missing_t or missing_f:
        print("THE DESCRIPTION MAY HAVE FALLEN BEHIND THE APP:")
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

    # An explicit, reasoned acknowledgement can clear a copy-forward finding —
    # and nothing else. It cannot touch a failed asset delivery, a bad local
    # image or any other blocking check, because those are defects rather than
    # decisions, and a release where the visible surface genuinely did not
    # change is the only thing this is for.
    acknowledged = {}
    for entry in args.carried_forward:
        field, _, reason = entry.partition("=")
        field, reason = field.strip(), reason.strip()
        if not reason:
            sys.exit(f"--carried-forward {entry!r} needs FIELD=REASON; the "
                     "reason is the point of recording it")
        if field not in problems:
            sys.exit(f"--carried-forward names {field!r}, which this run did not "
                     f"flag as carried forward. Flagged: {sorted(problems) or 'nothing'}. "
                     "Acknowledging something that was never in question hides "
                     "the next real finding.")
        acknowledged[field] = reason

    remaining = [p for p in problems if p not in acknowledged]

    if acknowledged:
        print("\nCARRIED FORWARD DELIBERATELY:")
        for field, reason in sorted(acknowledged.items()):
            print(f"  - {field}: {reason}")

    if remaining:
        print("\nPER-RELEASE FIELDS THAT WERE NOT WRITTEN FOR THIS RELEASE:")
        for p in remaining:
            print(f"  - {p}")
        print(
            "\nThese values match a predecessor and therefore require explicit review.\n"
            "Fix them before submitting (docs/APP_STORE_SUBMISSION.md)."
        )
    if blocking or remaining or delivery or local_problems:
        return 1
    if acknowledged:
        # Not "every field was written" — one was not, deliberately, and saying
        # otherwise would make the sign-off line the least accurate in the run.
        print("\nevery per-release field was either written for this release "
              "or carried forward with a stated reason.")
    else:
        print("\nevery per-release field was written for this release.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
