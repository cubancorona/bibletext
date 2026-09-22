#!/usr/bin/env python3
"""Where every store actually stands, read live, in one command.

    . scripts/asc-env.sh
    . scripts/msstore-env.sh
    scripts/release-status.py

Read-only. It creates nothing, uploads nothing and changes nothing anywhere,
so it is safe to run at any point in a release or outside one.

WHY THIS EXISTS. The version a store is serving is not written down anywhere
that stays true -- the repo's notes go stale between releases, and a release
decision made against a stale note is how a whole preparation gets wasted.
Five different APIs have to be asked, each with its own auth, and doing that
by hand is both slow and the sort of thing that gets skipped under time
pressure. So it is one command, and its answers come from the stores.

WHAT IT DELIBERATELY DOES NOT DO. It does not judge whether a release is
ready, and it does not tell you the next step. It reports state; the runbook
in docs/RELEASING.md holds the order and the decisions.

A store that cannot be reached is reported as UNREACHABLE and the exit code
is non-zero. It is never reported as up to date -- a status tool that turns a
network failure into a green line is worse than no tool, because the whole
point is to be believed.
"""

from __future__ import annotations

import importlib.util
import json
import os
import re
import subprocess
import sys
import urllib.request

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def load(path: str, name: str):
    spec = importlib.util.spec_from_file_location(name, os.path.join(REPO, path))
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)
    except SystemExit:
        pass           # these modules parse argv at import; we only want helpers
    return mod


def ledger_versions() -> dict:
    """The version this tree declares. Everything else is compared against it."""
    out = {}
    for label, path in (("desktop", "cmd/bibletext/FyneApp.toml"),
                        ("mobile", "cmd/mobile/FyneApp.toml")):
        text = open(os.path.join(REPO, path)).read()
        v = re.search(r'Version\s*=\s*"([^"]+)"', text)
        b = re.search(r"Build\s*=\s*(\d+)", text)
        out[label] = {"version": v.group(1) if v else None,
                      "build": int(b.group(1)) if b else None}
    return out


# ------------------------------------------------------------------ stores

def apple(version: str) -> list[str]:
    if not os.environ.get("ASC_KEY_PATH"):
        raise RuntimeError("ASC env not loaded -- run `. scripts/asc-env.sh` (source it, never pipe it)")
    sv = load("appstore/submit-version.py", "sv")
    rows = []
    for platform in ("IOS", "MAC_OS"):
        d = sv.asc("get", f"/v1/apps/{sv.APP_ID}/appStoreVersions?filter[platform]={platform}"
                          f"&fields[appStoreVersions]=versionString,appStoreState,releaseType&limit=20")
        items = [i["attributes"] for i in d.get("data", [])]
        if not items:
            rows.append(f"{platform:10} (no versions)")
            continue
        live = next((i for i in items if i.get("appStoreState") == "READY_FOR_SALE"), None)
        inflight = [i for i in items if i.get("appStoreState") not in ("READY_FOR_SALE", "REPLACED_WITH_NEW_INFO_FROM_DEVELOPER")]
        live_s = live["versionString"] if live else "none"
        if inflight:
            i = inflight[0]
            rows.append(f"{platform:10} live {live_s:8}  in flight {i['versionString']} "
                        f"({i.get('appStoreState')}, releaseType {i.get('releaseType')})")
        else:
            rows.append(f"{platform:10} live {live_s:8}  nothing in flight")
    return rows


def play(version: str) -> list[str]:
    pp = load("scripts/play-publish.py", "pp")
    tok = pp.access_token()
    _st, edit = pp.call(f"{pp.BASE}/edits", tok, method="POST", payload={})
    eid = edit["id"]
    rows = []
    try:
        for track in ("internal", "alpha", "beta", "production"):
            _st, t = pp.call(f"{pp.BASE}/edits/{eid}/tracks/{track}", tok)
            rel = t.get("releases") or []
            if not rel:
                rows.append(f"{track:11} (empty)")
            else:
                rows.append("; ".join(f"{track:11} {r.get('name')} "
                                      f"code {(r.get('versionCodes') or ['?'])[0]} {r.get('status')}"
                                      for r in rel))
    finally:
        try:
            pp.call(f"{pp.BASE}/edits/{eid}", tok, method="DELETE")
        except (Exception, SystemExit):
            # A discarded edit expires on its own. call() raises SystemExit on
            # an HTTP error, and letting that escape from a finally would throw
            # away every row already read above -- the cleanup must never cost
            # the report.
            pass
    return rows


def microsoft(version: str) -> list[str]:
    if not os.environ.get("MSSTORE_CLIENT_ID") and not os.environ.get("AZURE_CLIENT_ID"):
        raise RuntimeError("Microsoft env not loaded -- run `. scripts/msstore-env.sh`")
    sub = load("msstore/submit.py", "mssubmit")
    tok = sub.token()
    app = sub.api("GET", f"/applications/{sub.STORE_ID}", tok)
    rows = []
    pub = (app.get("lastPublishedApplicationSubmission") or {}).get("id")
    if pub:
        s = sub.api("GET", f"/applications/{sub.STORE_ID}/submissions/{pub}", tok)
        pkgs = [f"{p.get('version')} {p.get('architecture')}"
                for p in (s.get("applicationPackages") or []) if p.get("fileStatus") == "Uploaded"]
        rows.append(f"published  {', '.join(pkgs) or '(no packages)'}")
    pend = (app.get("pendingApplicationSubmission") or {}).get("id")
    if pend:
        st = sub.api("GET", f"/applications/{sub.STORE_ID}/submissions/{pend}/status", tok)
        rows.append(f"pending    {pend} -> {st.get('status')}")
        for e in (st.get("statusDetails") or {}).get("errors") or []:
            rows.append(f"    ERROR {e.get('code')}: {e.get('details')}")
    else:
        rows.append("pending    none")
    return rows


def snap(version: str) -> list[str]:
    req = urllib.request.Request(
        "https://api.snapcraft.io/v2/snaps/info/bibletext?fields=version,revision",
        headers={"Snap-Device-Series": "16"})
    with urllib.request.urlopen(req, timeout=60) as r:
        d = json.load(r)
    rows = []
    for c in d.get("channel-map", []):
        ch = c["channel"]
        rows.append(f"{ch['architecture']:6} {ch['risk']:9} {c.get('version'):9} rev {c.get('revision')}")
    return rows or ["(no channels)"]


def github(version: str) -> list[str]:
    out = subprocess.run(["gh", "release", "view", "--json", "tagName,assets,isDraft,publishedAt"],
                         cwd=REPO, capture_output=True, text=True)
    if out.returncode != 0:
        raise RuntimeError(out.stderr.strip()[:200] or "gh release view failed")
    d = json.loads(out.stdout)
    names = [a["name"] for a in d.get("assets", [])]
    rows = [f"latest     {d.get('tagName')}  {len(names)} assets"
            f"{'  DRAFT' if d.get('isDraft') else ''}  {(d.get('publishedAt') or '')[:10]}"]
    missing = [n for n in re.findall(r"releases/latest/download/([A-Za-z0-9._-]+)",
                                     open(os.path.join(REPO, "docs/index.html")).read())
               if n not in names]
    rows.append("page links " + ("all present on this release" if not missing
                                 else f"MISSING from the release: {', '.join(sorted(set(missing)))}"))
    return rows


STORES = [("Apple", apple), ("Google Play", play), ("Microsoft", microsoft),
          ("Snap", snap), ("GitHub", github)]


def main() -> int:
    led = ledger_versions()
    version = led["desktop"]["version"]
    print("this tree declares")
    for k, v in led.items():
        print(f"  {k:8} {v['version']}  build {v['build']}")
    if led["desktop"]["version"] != led["mobile"]["version"]:
        print("  ! the two ledgers disagree; check-release-identity.py would fail")
    print()

    failures = []
    for label, fn in STORES:
        print(f"{label}")
        try:
            for row in fn(version):
                print(f"  {row}")
        except (Exception, SystemExit) as e:
            # Reported, never swallowed: an unreachable store must not read as
            # an up-to-date one.
            #
            # SystemExit is named on purpose. Every store helper this imports
            # -- play-publish.py's call(), msstore/submit.py's http(), the ASC
            # client -- signals an HTTP or credential failure by raising
            # SystemExit, which is a BaseException and sails straight past a
            # bare `except Exception`. Without it one store's 5xx killed the
            # whole report instead of being printed as UNREACHABLE, which is
            # the exact promise this tool makes.
            print(f"  UNREACHABLE: {type(e).__name__}: {str(e)[:200]}")
            failures.append(label)
        print()

    if failures:
        print(f"could not read: {', '.join(failures)} -- this report is INCOMPLETE")
        return 1
    print("every store answered. docs/RELEASING.md holds the order and the decisions.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
