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

# THE DESCRIPTION NEEDS A DIFFERENT KIND OF CHECK, and this is the reasoning.
# Every other per-release field is guarded by "did it move?". The description is
# the one field that SHOULD sit still for years, so that test can never apply to
# it — and sitting still is exactly how it goes wrong: the app grows and the
# page describing it does not. On 19 Aug 2026 the live copy still offered "the
# World English Bible and the Berean Standard Bible" while the app shipped four
# readable translations, and mentioned neither shared notes nor audio.
#
# What CAN be checked is the claim against the code: every translation a reader
# can actually open should be named on the page selling the app. These are the
# ones registeredVersions (versions.go) serves today.
# Each entry: (name-for-the-report, tuple of phrasings that count as naming it).
# The alternates exist because the 1.2.1 copy says "including a Catholic
# edition" and the checker's exact-name grep called that absent — a false
# advisory, and a checker that cries wolf trains its reader to skip the report.
DESCRIBED_TRANSLATIONS = [
    ("World English Bible", ("world english bible",), "public domain, always available"),
    ("Berean Standard Bible", ("berean standard bible",), "public domain, always available"),
    ("World English Bible (Catholic)", ("world english bible (catholic)", "catholic edition"),
     "public domain, the 73-book canon"),
    ("New King James Version", ("new king james version",), "licensed, works on install"),
]

# Features big enough that a reader deciding whether to install would want them
# on the page. Kept short on purpose: this is not a changelog.
# The needles are deliberately SPECIFIC. "notes" alone matched the iPad line's
# "Split View alongside your notes" and passed the check while the description
# said nothing whatever about the shared-notes feature — a false pass on the one
# feature that most needs to be on the page.
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


# States that mean Apple currently owns the version: a second version cannot be
# taken into review while one of these is outstanding, so a release plan built
# without checking is a plan that cannot be executed.
IN_REVIEW_STATES = {
    "WAITING_FOR_REVIEW", "IN_REVIEW", "PENDING_APPLE_RELEASE",
    "PENDING_DEVELOPER_RELEASE", "PROCESSING_FOR_APP_STORE",
}


def report_live_states():
    """Print every version's live state and flag anything blocking a submission.

    WHY THIS IS THE FIRST THING PRINTED. Twice in two days a release plan was
    made against a remembered state that had already moved: 1.2.1 was recorded
    as awaiting review when it had gone live, and 1.2.2 was assumed reviewable
    when it was still queued and therefore blocking everything behind it. Both
    were found by hand-querying, which is precisely the step that gets skipped.

    The field comparison below answers "was this version's copy written for it".
    It cannot answer "may I submit at all", and that is the question a release
    starts with.
    """
    data = asc(f"/v1/apps/{APP_ID}/appStoreVersions?limit=10")["data"]
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


def main():
    report_live_states()

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

    # The description's own check — reported, never fatal: marketing copy is the
    # owner's to write, and this can only say what looks absent.
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
            "\nThese arrived by App Store Connect's copy-forward, not by anyone's decision.\n"
            "That is exactly how 1.2.0 shipped 1.1.8's review notes and 1.1.8's screenshots.\n"
            "Fix them before submitting (docs/APP_STORE_SUBMISSION.md)."
        )
        return 1
    print("\nevery per-release field was written for this release.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
