#!/usr/bin/env python3
"""Write appstore/review-notes.txt into App Store Connect's APP REVIEW notes.

WHY THIS EXISTS. Nothing in this repo ever wrote that field. push_metadata.py
writes the customer-facing text (description, keywords, whatsNew, copyright);
push_betareview.py and push_testflight.py write betaAppReviewDetail, which is
TESTFLIGHT review and a different field entirely. So App Review's notes were
whatever App Store Connect had copied forward from the previous version — which
is how 1.2.0 went to review describing 1.1.8's search-results hotfix, and how
1.1.5, 1.1.6 and 1.1.7 all shipped notes headed "NEW IN 1.1.0 — IPAD".

The notes themselves are tracked (appstore/review-notes.txt) and guarded by
appstore_review_notes_test.go, which fails when they still describe an older
version than cmd/mobile/FyneApp.toml ships.

USAGE — read first, write only when asked:

    ASC_KEY_PATH=~/.private_keys/AuthKey_XXXX.p8 \\
    ASC_KEY_ID=XXXX ASC_ISSUER_ID=... \\
    python3 appstore/push-review-notes.py            # SHOW what is there now
    python3 appstore/push-review-notes.py --write    # replace it

It refuses to write to a version that is not editable: once a version is
IN_REVIEW or READY_FOR_SALE, changing what the reviewer was told is not
something a script should do behind anyone's back.
"""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
NOTES = os.path.join(HERE, "review-notes.txt")
ASC = os.path.join(REPO, "build", "appstore", "asc.py")

# States whose review notes may still be rewritten. Anything else is either
# already in front of a reviewer or already shipped.
EDITABLE = {
    "PREPARE_FOR_SUBMISSION",
    "DEVELOPER_REJECTED",
    "REJECTED",
    "METADATA_REJECTED",
    "INVALID_BINARY",
}

APP_ID = os.environ.get("ASC_APP_ID", "6784567351")


def asc(*args):
    if not os.path.exists(ASC):
        sys.exit(
            f"cannot find {ASC}.\n"
            "The ASC client lives under build/, which is gitignored, so a fresh\n"
            "clone does not have it. See docs/APP_STORE_SUBMISSION.md."
        )
    out = subprocess.run([sys.executable, ASC, *args], capture_output=True, text=True)
    body = out.stdout
    if "{" not in body:
        sys.exit(f"App Store Connect call failed:\n{out.stdout}\n{out.stderr}")
    return json.loads(body[body.index("{"):])


def main():
    write = "--write" in sys.argv

    with open(NOTES, encoding="utf-8") as f:
        notes = f.read().rstrip("\n")
    if not notes.strip():
        sys.exit(f"{NOTES} is empty — refusing to blank the review notes")

    versions = asc("get", f"/v1/apps/{APP_ID}/appStoreVersions?limit=5")["data"]
    if not versions:
        sys.exit("no app store versions found")
    v = versions[0]
    state = v["attributes"].get("appStoreState")
    print(f"latest version: {v['attributes'].get('versionString')}  state: {state}")

    detail = asc("get", f"/v1/appStoreVersions/{v['id']}/appStoreReviewDetail").get("data")
    if not detail:
        sys.exit("this version has no appStoreReviewDetail yet — create it in the ASC UI first")

    current = detail["attributes"].get("notes") or ""
    print("\n--- notes currently in App Store Connect ---")
    print(current if current.strip() else "(EMPTY)")
    print("--- end ---\n")

    if not write:
        same = current.rstrip("\n") == notes
        print("in sync with appstore/review-notes.txt" if same
              else "DIFFERENT from appstore/review-notes.txt — re-run with --write to replace")
        return

    if state not in EDITABLE:
        sys.exit(
            f"refusing to write: the version is {state}.\n"
            "Review notes are only rewritten while a version is still editable — once it\n"
            "is with a reviewer, changing what they were told is the owner's call to make\n"
            "in the App Store Connect UI, deliberately, not a script's to make silently."
        )

    payload = json.dumps({"data": {
        "type": "appStoreReviewDetails", "id": detail["id"],
        "attributes": {"notes": notes},
    }})
    asc("patch", f"/v1/appStoreReviewDetails/{detail['id']}", payload)

    back = asc("get", f"/v1/appStoreVersions/{v['id']}/appStoreReviewDetail")
    written = back["data"]["attributes"].get("notes") or ""
    if written.rstrip("\n") != notes:
        sys.exit("READ-BACK MISMATCH: what App Store Connect holds is not what was sent")
    print("written and read back identical.")


if __name__ == "__main__":
    main()
