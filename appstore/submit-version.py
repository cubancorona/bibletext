#!/usr/bin/env python3
"""Create, attach, write and submit one App Store version — in that order, and
never the last step until the earlier ones are proven.

Read-only by default: it looks the build and version up and says what it would
do. ``--write`` with an exact ``--confirm-version`` creates the version record
if it is missing, attaches the build, and runs the two metadata writers.
``--submit`` is a separate, explicit flag: only then is a review submission
created, and only after ``preflight.py`` reports every per-release field
written. Screenshots are the one field a release may knowingly inherit; say so
with ``--accept-inherited-screenshots`` or the submission is refused.

WHY THE ORDER IS THE POINT. 1.2.8's Mac submission was attempted with the
version's What's New empty: App Store Connect accepted the review submission
record and then refused the item that names the version, leaving an empty
submission behind and the release not submitted. Checking the fields BEFORE
creating the submission is the whole job of this file. The build is also
checked against the upload's delivery UUID when one is given (an App Store
Connect build's id IS that UUID), so what gets attached is provably what was
sent.

    python3 appstore/submit-version.py --platform MAC_OS --build 49 \\
        --delivery-uuid 6df65fa3-...                      # look, do nothing
    python3 appstore/submit-version.py --platform MAC_OS --build 49 \\
        --delivery-uuid 6df65fa3-... --write --confirm-version 1.2.8 --submit
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASC = os.path.join(ROOT, "build", "appstore", "asc.py")
APP_ID = "6784567351"
LEDGERS = {"IOS": "cmd/mobile/FyneApp.toml", "MAC_OS": "cmd/desktop/FyneApp.toml"}


def ledger_version(platform):
    text = open(os.path.join(ROOT, LEDGERS[platform]), encoding="utf-8").read()
    m = re.search(r'^Version = "([^"]+)"$', text, re.M)
    if not m:
        sys.exit(f"{LEDGERS[platform]} has no Version line")
    return m.group(1)


def asc(method, path, body=None):
    if not os.path.exists(ASC):
        sys.exit(f"cannot find {ASC} (build/ is gitignored — see docs/APP_STORE_SUBMISSION.md)")
    cmd = [sys.executable, ASC, method, path] + ([json.dumps(body)] if body is not None else [])
    out = subprocess.run(cmd, capture_output=True, text=True)
    text = out.stdout
    status = text.split("\n", 1)[0].strip() if text[:3].isdigit() else ""
    if "{" not in text:
        if method == "patch" and status == "204":
            return {}
        sys.exit(f"App Store Connect {method.upper()} {path} failed:\n{text}\n{out.stderr}")
    data = json.loads(text[text.index("{"):])
    if "errors" in data:
        sys.exit(f"App Store Connect {method.upper()} {path} refused:\n{json.dumps(data['errors'], indent=2)}")
    return data


def find_build(platform, number):
    d = asc("get", f"/v1/builds?filter[app]={APP_ID}&filter[version]={number}"
                   f"&filter[preReleaseVersion.platform]={platform}&fields[builds]=version,processingState&limit=5")
    items = d.get("data", [])
    if len(items) != 1:
        sys.exit(f"{len(items)} builds numbered {number} on {platform}; expected exactly one")
    return items[0]["id"], items[0]["attributes"]["processingState"]


def find_version(platform, version):
    d = asc("get", f"/v1/apps/{APP_ID}/appStoreVersions?filter[platform]={platform}"
                   f"&filter[versionString]={version}&fields[appStoreVersions]=versionString,appStoreState&limit=5")
    items = d.get("data", [])
    if not items:
        return None, None
    return items[0]["id"], items[0]["attributes"]["appStoreState"]


def attached_build(version_id):
    d = asc("get", f"/v1/appStoreVersions/{version_id}/build?fields[builds]=version")
    data = d.get("data")
    return (data["id"], data["attributes"]["version"]) if data else (None, None)


def run(label, argv):
    print(f"--- {label}")
    out = subprocess.run(argv, cwd=ROOT, capture_output=True, text=True)
    sys.stdout.write(out.stdout)
    if out.returncode != 0:
        sys.stderr.write(out.stderr)
        sys.exit(f"{label} failed (exit {out.returncode})")
    return out.stdout


def unwritten_per_release(platform):
    """The fields preflight reports as inherited for this release."""
    out = subprocess.run([sys.executable, os.path.join(ROOT, "appstore", "preflight.py"),
                          "--platform", platform], cwd=ROOT, capture_output=True, text=True)
    sys.stdout.write(out.stdout)
    fields = []
    if "PER-RELEASE FIELDS THAT WERE NOT WRITTEN" in out.stdout:
        tail = out.stdout.split("PER-RELEASE FIELDS THAT WERE NOT WRITTEN", 1)[1]
        fields = [ln.strip()[2:].strip() for ln in tail.splitlines() if ln.strip().startswith("- ")]
    return fields


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--platform", choices=("IOS", "MAC_OS"), required=True)
    p.add_argument("--build", type=int, required=True, help="the build number uploaded for this version")
    p.add_argument("--delivery-uuid", help="altool's Delivery UUID for that upload; must equal the build's id")
    p.add_argument("--write", action="store_true", help="create/attach/write; otherwise only report")
    p.add_argument("--confirm-version", metavar="VERSION", help="required with --write; must equal the ledger")
    p.add_argument("--submit", action="store_true", help="after --write: create and send the review submission")
    p.add_argument("--accept-inherited-screenshots", action="store_true",
                   help="submit even though the screenshots are inherited from the previous release")
    p.add_argument("--first-on-platform", action="store_true",
                   help="passed through to push-metadata: the platform's first version has no What's New")
    a = p.parse_args()

    for var in ("ASC_ISSUER_ID", "ASC_KEY_ID", "ASC_KEY_PATH"):
        if not os.environ.get(var):
            sys.exit(f"{var} is not set: run `. scripts/asc-env.sh` first (it reads the Keychain)")
    version = ledger_version(a.platform)
    if a.write and a.confirm_version != version:
        sys.exit(f"--write requires --confirm-version {version}; nothing was changed")
    if a.submit and not a.write:
        sys.exit("--submit is meaningful only with --write")

    build_id, state = find_build(a.platform, a.build)
    print(f"build {a.build} on {a.platform}: {build_id} {state}")
    if state != "VALID":
        sys.exit(f"build {a.build} is {state}, not VALID — wait for processing")
    if a.delivery_uuid and a.delivery_uuid.lower() != build_id.lower():
        sys.exit(f"build id {build_id} is not the delivery UUID {a.delivery_uuid}: this is not the artifact that was uploaded")

    version_id, vstate = find_version(a.platform, version)
    print(f"version {version} on {a.platform}: {version_id or 'absent'} {vstate or ''}")
    if not a.write:
        print("read-only: nothing changed (add --write --confirm-version to create/attach/write, --submit to send)")
        return 0

    if version_id is None:
        d = asc("post", "/v1/appStoreVersions", {"data": {
            "type": "appStoreVersions",
            "attributes": {"platform": a.platform, "versionString": version, "releaseType": "AFTER_APPROVAL"},
            "relationships": {"app": {"data": {"type": "apps", "id": APP_ID}}}}})
        version_id, vstate = d["data"]["id"], d["data"]["attributes"]["appStoreState"]
        print(f"created version {version}: {version_id} {vstate}")
    elif vstate not in ("PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED"):
        sys.exit(f"version {version} is {vstate}; it cannot be prepared from here")

    cur_id, cur_num = attached_build(version_id)
    if cur_id != build_id:
        asc("patch", f"/v1/appStoreVersions/{version_id}/relationships/build",
            {"data": {"type": "builds", "id": build_id}})
        cur_id, cur_num = attached_build(version_id)
        if cur_id != build_id:
            sys.exit("the build did not attach")
    print(f"attached build {cur_num} ({cur_id})")

    meta = [sys.executable, "appstore/push-metadata.py", "--write", "--confirm-version", version, "--platform", a.platform]
    if a.first_on_platform:
        meta.append("--first-on-platform")
    run("push-metadata", meta)
    run("push-review-notes", [sys.executable, "appstore/push-review-notes.py", "--write",
                              "--confirm-version", version, "--platform", a.platform])

    unwritten = unwritten_per_release(a.platform)
    blocking = [f for f in unwritten if not (f == "screenshots" and a.accept_inherited_screenshots)]
    if blocking:
        sys.exit("NOT SUBMITTED: preflight reports per-release fields not written for this release: "
                 + ", ".join(blocking) + "\n(screenshots may be accepted with --accept-inherited-screenshots)")
    if not a.submit:
        print("prepared; add --submit to send it for review")
        return 0

    sub = asc("post", "/v1/reviewSubmissions", {"data": {
        "type": "reviewSubmissions", "attributes": {"platform": a.platform},
        "relationships": {"app": {"data": {"type": "apps", "id": APP_ID}}}}})["data"]["id"]
    asc("post", "/v1/reviewSubmissionItems", {"data": {
        "type": "reviewSubmissionItems",
        "relationships": {"reviewSubmission": {"data": {"type": "reviewSubmissions", "id": sub}},
                          "appStoreVersion": {"data": {"type": "appStoreVersions", "id": version_id}}}}})
    d = asc("patch", f"/v1/reviewSubmissions/{sub}", {"data": {
        "type": "reviewSubmissions", "id": sub, "attributes": {"submitted": True}}})
    print(f"submitted: submission {sub} is {d['data']['attributes']['state']}")
    _, vstate = find_version(a.platform, version)
    print(f"version {version} on {a.platform}: {vstate}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
