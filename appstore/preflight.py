#!/usr/bin/env python3
"""Pre-submission read-back: what does App Store Connect ACTUALLY hold, and how
much of it was silently inherited from the last release?

WHY THIS EXISTS. App Store Connect copies a new version record from the previous
one. Every per-release field therefore arrives already populated, already
plausible, and already WRONG — there is no empty box to notice. Twice now that
has shipped:

  * 1.2.0 went into review with review notes headed "VERSION 1.1.8 — HOTFIX",
    describing a search-results fix, while its headline feature was shared notes.
    1.1.5, 1.1.6 and 1.1.7 had all carried "NEW IN 1.1.0 — IPAD" before that.
  * 1.2.0's screenshots are byte-identical to 1.1.8's (checked by
    sourceFileChecksum) and their filenames predate even the prepared 1.1.8 set,
    so the store page for the shared-notes release shows no shared note.

Reading the fields one at a time never caught it, because each looked fine on
its own. What catches it is the COMPARISON: this tool diffs the version being
submitted against the one before it and says which per-release fields did not
move. A field that must change every release and did not is the whole bug.

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

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
ASC = os.path.join(REPO, "build", "appstore", "asc.py")
APP_ID = os.environ.get("ASC_APP_ID", "6784567351")

# Fields that MUST be rewritten for every release. Anything here that is
# identical to the previous version was inherited, not written.
PER_RELEASE = {"whatsNew", "review notes", "screenshots"}

# Fields that legitimately stay the same release to release. Listed so the
# report can say "unchanged, and that is fine" rather than staying silent.
STABLE = {"description", "keywords", "marketingUrl", "supportUrl"}


def asc(path):
    if not os.path.exists(ASC):
        sys.exit(f"cannot find {ASC} (build/ is gitignored — see docs/APP_STORE_SUBMISSION.md)")
    out = subprocess.run([sys.executable, ASC, "get", path], capture_output=True, text=True)
    if "{" not in out.stdout:
        sys.exit(f"App Store Connect call failed for {path}:\n{out.stdout}\n{out.stderr}")
    return json.loads(out.stdout[out.stdout.index("{"):])


def version_fields(vid):
    """Every per-release field of one version, in a comparable shape."""
    f = {}
    loc = asc(f"/v1/appStoreVersions/{vid}/appStoreVersionLocalizations")["data"][0]
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


def main():
    versions = asc(f"/v1/apps/{APP_ID}/appStoreVersions?limit=2")["data"]
    if len(versions) < 2:
        sys.exit("need at least two versions to compare against")
    cur, prev = versions[0], versions[1]
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

    if problems:
        print("\nPER-RELEASE FIELDS THAT WERE NOT WRITTEN FOR THIS RELEASE:")
        for p in problems:
            print(f"  - {p}")
        print(
            "\nThese arrived by App Store Connect's copy-forward, not by anyone's decision.\n"
            "That is exactly how 1.2.0 shipped 1.1.8's review notes and 1.1.8's screenshots.\n"
            "Fix them before submitting (docs/APP_STORE_SUBMISSION.md)."
        )
        return 1
    print("\nevery per-release field was written for this release.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
