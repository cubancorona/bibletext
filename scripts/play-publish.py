#!/usr/bin/env python3
"""Google Play Developer API client for BibleText releases.

WHY THIS EXISTS AND WHY IT HAS NO DEPENDENCIES. Uploading an app bundle by hand
means a console session per release, and a console session is not reviewable
afterwards. This does the same work from the same tree that produced the
artifact, so a release leaves a command rather than a memory. google-auth is
not installed on this machine and pulling it in for one RS256 signature would
add a dependency to a release path that must keep working years from now, so
the service-account JWT is built with `cryptography`, which is already present
(the App Store tooling does the same for its ES256 token).

CREDENTIAL. A service-account key at ~/.private_keys/, mode 0600, never in the
repository. The account is scoped in Play Console to this app only, with
testing-track releases, tester lists and store presence - deliberately NOT
"release to production", so nothing here can publish to the public.

  play-publish.py tracks                       what tracks exist
  play-publish.py upload <bundle.aab> [track]  upload, assign, commit (default: internal)
  play-publish.py --dry-run upload <b> [track] everything except the commit
"""
import base64, json, os, sys, time, urllib.parse, urllib.request
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

KEY_PATH = os.environ.get("BIBLETEXT_PLAY_KEY",
                          os.path.expanduser("~/.private_keys/bibletext-play-publisher.json"))
PKG = "uk.co.bibletext"
BASE = "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/" + PKG
UPLOAD = "https://androidpublisher.googleapis.com/upload/androidpublisher/v3/applications/" + PKG
b64 = lambda b: base64.urlsafe_b64encode(b).rstrip(b"=")


def access_token():
    with open(KEY_PATH) as f:
        sa = json.load(f)
    key = serialization.load_pem_private_key(sa["private_key"].encode(), password=None)
    now = int(time.time())
    claims = {"iss": sa["client_email"],
              "scope": "https://www.googleapis.com/auth/androidpublisher",
              "aud": "https://oauth2.googleapis.com/token", "iat": now, "exp": now + 3600}
    si = b64(json.dumps({"alg": "RS256", "typ": "JWT"}).encode()) + b"." + b64(json.dumps(claims).encode())
    assertion = (si + b"." + b64(key.sign(si, padding.PKCS1v15(), hashes.SHA256()))).decode()
    body = urllib.parse.urlencode({"grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                                   "assertion": assertion}).encode()
    req = urllib.request.Request("https://oauth2.googleapis.com/token", data=body,
                                 headers={"Content-Type": "application/x-www-form-urlencoded"})
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.load(r)["access_token"]


def call(url, token, method="GET", payload=None, raw=None, content_type="application/json"):
    data = raw if raw is not None else (json.dumps(payload).encode() if payload is not None else None)
    req = urllib.request.Request(url, data=data, method=method,
                                 headers={"Authorization": "Bearer " + token,
                                          "Content-Type": content_type})
    try:
        with urllib.request.urlopen(req, timeout=900) as r:
            body = r.read()
            return r.status, (json.loads(body) if body.strip() else {})
    except urllib.error.HTTPError as e:
        raise SystemExit(f"Play API {method} {url.split('?')[0]} -> HTTP {e.code}\n{e.read().decode()[:600]}")


def main(argv):
    dry = "--dry-run" in argv
    argv = [a for a in argv if a != "--dry-run"]
    cmd = argv[1] if len(argv) > 1 else "tracks"
    token = access_token()

    if cmd == "tracks":
        _, edit = call(BASE + "/edits", token, "POST", {})
        _, tracks = call(f"{BASE}/edits/{edit['id']}/tracks", token)
        for t in tracks.get("tracks", []):
            rel = t.get("releases") or []
            codes = [c for r in rel for c in (r.get("versionCodes") or [])]
            print(f"  {t['track']:<12} {', '.join(codes) if codes else '(no release)'}")
        call(f"{BASE}/edits/{edit['id']}", token, "DELETE")   # leave nothing dangling
        return 0

    if cmd == "upload":
        if len(argv) < 3:
            raise SystemExit("usage: play-publish.py upload <bundle.aab> [track]")
        aab, track = argv[2], (argv[3] if len(argv) > 3 else "internal")
        with open(aab, "rb") as f:
            blob = f.read()
        print(f"  {aab}: {len(blob):,} bytes -> track '{track}'")
        _, edit = call(BASE + "/edits", token, "POST", {})
        eid = edit["id"]
        _, b = call(f"{UPLOAD}/edits/{eid}/bundles?uploadType=media", token, "POST",
                    raw=blob, content_type="application/octet-stream")
        code = b["versionCode"]
        print(f"  uploaded versionCode {code} (sha1 {b.get('sha1')})")
        call(f"{BASE}/edits/{eid}/tracks/{track}", token, "PUT",
             {"track": track, "releases": [{"status": "completed", "versionCodes": [str(code)]}]})
        print(f"  assigned {code} to '{track}'")
        if dry:
            call(f"{BASE}/edits/{eid}", token, "DELETE")
            print("  --dry-run: edit discarded, nothing changed on Play")
            return 0
        _, done = call(f"{BASE}/edits/{eid}:commit", token, "POST")
        print(f"  committed edit {done.get('id', eid)}")
        return 0

    raise SystemExit(f"unknown command: {cmd}")


if __name__ == "__main__":
    sys.exit(main(sys.argv))
