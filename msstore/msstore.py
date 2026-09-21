#!/usr/bin/env python3
"""The Microsoft Store submission API, read side, from the command line.

    . scripts/msstore-env.sh
    msstore/msstore.py apps                 # every product on the account
    msstore/msstore.py app 9NDCCZH9RB9K     # one product, with its submissions
    msstore/msstore.py submission 9NDCCZH9RB9K <submission id>

Standard library only, like appstore/asc.py. The token is a client-credentials
grant against the Entra tenant for the classic Dev Center resource; the
application must be added to the Partner Center account with the Manager role
(docs/WINDOWS_STORE_LISTING.md, "Automation after the first release").

The write side — create a submission, upload the package to its SAS URL,
commit, poll — lives in **msstore/submit.py**, which was written against the
second submission on 21 September 2026 rather than guessed ahead of one. This
file stays read-only on purpose: the two have different blast radii, and a
module that can only ask questions is worth keeping separable from one that
can publish.

Note if you are extending either: `fetch()` below calls `json.load()`
unconditionally, which is correct for the GETs it serves and wrong for
anything with an empty body — a DELETE, or the 201 from a blob upload. It
would raise AFTER the server had already acted. submit.py splits transport
from parsing for exactly that reason; do not carry this one over.
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

API = "https://manage.devcenter.microsoft.com/v1.0/my"
RESOURCE = "https://manage.devcenter.microsoft.com"


def credentials(env: dict[str, str] | None = None) -> tuple[str, str, str]:
    env = os.environ if env is None else env
    try:
        return env["MSSTORE_TENANT_ID"], env["MSSTORE_CLIENT_ID"], env["MSSTORE_CLIENT_SECRET"]
    except KeyError as missing:
        raise SystemExit(f"{missing.args[0]} is not set; source scripts/msstore-env.sh first") from None


def token_request(tenant: str, client_id: str, secret: str) -> urllib.request.Request:
    """The client-credentials request, built separately so a test can see it
    without a network: the token endpoint of the tenant, form-encoded, the
    classic Dev Center resource."""
    body = urllib.parse.urlencode({
        "grant_type": "client_credentials",
        "client_id": client_id,
        "client_secret": secret,
        "resource": RESOURCE,
    }).encode()
    return urllib.request.Request(
        f"https://login.microsoftonline.com/{tenant}/oauth2/token",
        data=body,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )


def api_request(path: str, token: str) -> urllib.request.Request:
    if not path.startswith("/"):
        raise ValueError("path must start with /")
    return urllib.request.Request(
        API + path,
        headers={"Authorization": "Bearer " + token, "Accept": "application/json"},
    )


def fetch(req: urllib.request.Request) -> dict:
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as err:
        detail = err.read().decode("utf-8", "replace")[:800]
        raise SystemExit(f"HTTP {err.code} from {req.full_url}: {detail}") from None


def get_token() -> str:
    tenant, client_id, secret = credentials()
    answer = fetch(token_request(tenant, client_id, secret))
    token = answer.get("access_token")
    if not token:
        raise SystemExit("the token response carried no access_token")
    return token


def summarise_apps(listing: dict) -> list[dict]:
    """The fields a release needs from GET /applications: id, name, and
    whether a submission is pending or published."""
    out = []
    for app in listing.get("value", []):
        pending = app.get("pendingApplicationSubmission") or {}
        published = app.get("lastPublishedApplicationSubmission") or {}
        out.append({
            "id": app.get("id"),
            "name": app.get("primaryName"),
            "pendingSubmission": pending.get("id"),
            "lastPublishedSubmission": published.get("id"),
        })
    return out


def main(argv: list[str]) -> int:
    if len(argv) < 2 or argv[1] not in {"apps", "app", "submission"}:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    token = get_token()
    if argv[1] == "apps":
        for app in summarise_apps(fetch(api_request("/applications?top=50", token))):
            print(json.dumps(app))
        return 0
    if argv[1] == "app":
        if len(argv) != 3:
            raise SystemExit("usage: msstore.py app <store id>")
        print(json.dumps(fetch(api_request(f"/applications/{argv[2]}", token)), indent=2))
        return 0
    if len(argv) != 4:
        raise SystemExit("usage: msstore.py submission <store id> <submission id>")
    sub = fetch(api_request(f"/applications/{argv[2]}/submissions/{argv[3]}", token))
    print(json.dumps({"id": sub.get("id"), "status": sub.get("status"),
                      "statusDetails": sub.get("statusDetails")}, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
