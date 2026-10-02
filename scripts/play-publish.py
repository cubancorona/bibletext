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
testing-track releases, tester lists and store presence, and, since 2 October
2026, "release to production" for this app only. Production is reached by
`promote` and by nothing else: `upload` refuses the production track, and
`promote` is its own command, run on the account holder's OK, the way Apple's
`--submit` and the Microsoft Store's `commit` are.

  play-publish.py tracks                       what tracks exist
  play-publish.py upload <bundle.aab> [track]  upload, assign, commit (default: internal)
  play-publish.py --dry-run upload <b> [track] everything except the commit
  --status draft|completed                     draft is REQUIRED until the app
                                               has been published once
  --notes <file>                               the en-GB release notes, line
                                               breaks kept, at most 500 characters
  play-publish.py promote <from-track> production --confirm-version <v>
                  [--rollout <fraction>] [--dry-run]
                                               the from-track's one release to
                                               production, its notes and name as
                                               they are; --dry-run discards the edit

ONE PLAY STEP AT A TIME. Every command here opens an edit as the one service
account, and Play lets each user hold one open edit: a new edit ends the one
that user already has open, and a commit, or any change in the Play Console,
ends every other edit for the app. An upload whose edit is ended that way
says so (`EDIT_DELETED`). So uploads, promotions, `tracks`,
`scripts/release-status.py` and `play/push-screenshots.py` run one at a time.
"""
import base64, copy, json, os, re, sys, time, urllib.parse, urllib.request
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

KEY_PATH = os.environ.get("BIBLETEXT_PLAY_KEY",
                          os.path.expanduser("~/.private_keys/bibletext-play-publisher.json"))
PKG = "uk.co.bibletext"
BASE = "https://androidpublisher.googleapis.com/androidpublisher/v3/applications/" + PKG
UPLOAD = "https://androidpublisher.googleapis.com/upload/androidpublisher/v3/applications/" + PKG
b64 = lambda b: base64.urlsafe_b64encode(b).rstrip(b"=")

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
# The mobile ledger. scripts/build-android.sh builds every Android artefact
# from its Version and Build, so the versionCode Play holds for a release is
# the ledger's Build, and the ledger is what a promotion is checked against.
LEDGER = os.path.join(REPO, "cmd", "mobile", "FyneApp.toml")
PRODUCTION = "production"
NOTES_CAP = 500

# edits.commit's changesInReviewBehavior defaults to CANCEL_IN_REVIEW_AND_SUBMIT:
# a commit made while other changes are in review cancels that review and sends
# everything again. ERROR_IF_IN_REVIEW makes Play refuse the commit instead, so
# a promotion never restarts the review of something else
# (play/push-screenshots.py commits the same way, and says more).
IN_REVIEW_QUERY = "changesInReviewBehavior=ERROR_IF_IN_REVIEW"

# Play's answer, HTTP 400 FAILED_PRECONDITION, to a request on an edit that
# another edit, a commit or a Play Console change has ended. On 2 October 2026
# a status read opened beside an upload ended the upload's edit this way.
EDIT_DELETED = "this edit has been deleted"


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


def take(argv, flag, needs):
    """Remove `flag <value>` from argv and return the value, or None."""
    if flag not in argv:
        return None
    i = argv.index(flag)
    if i + 1 >= len(argv):
        raise SystemExit(f"{flag} needs {needs}")
    value = argv[i + 1]
    del argv[i:i + 2]
    return value


def read_notes(path):
    """The release notes file as Play is to show it.

    Each line loses its trailing spaces and the whole loses blank lines at
    its ends; the line breaks between them stay, so the opening line and its
    bullets reach Play as lines, not as one paragraph with the bullets
    inline. Play caps a language's notes at 500 characters, and every line
    break is one of them.
    """
    with open(path, encoding="utf-8") as f:
        notes = "\n".join(line.rstrip() for line in f.read().strip().splitlines())
    if not notes:
        raise SystemExit(f"--notes: {path} is empty")
    if len(notes) > NOTES_CAP:
        raise SystemExit(f"--notes: {len(notes)} characters, line breaks included; "
                         f"Play caps release notes at {NOTES_CAP}")
    return notes


def ledger():
    """(Version, Build) from the mobile ledger, both as text."""
    with open(LEDGER, encoding="utf-8") as f:
        text = f.read()
    version = re.search(r'^Version = "([^"]+)"$', text, re.M)
    build = re.search(r"^Build = (\d+)$", text, re.M)
    if not version or not build:
        raise SystemExit(f"{LEDGER} has no Version or no Build line")
    return version.group(1), build.group(1)


def parse_rollout(text):
    try:
        fraction = float(text)
    except ValueError:
        fraction = None
    # NaN fails every comparison, so it is refused here too.
    if fraction is None or not 0 < fraction <= 1:
        raise SystemExit(f"--rollout must be a fraction above 0 and at most 1, not {text!r}; "
                         "nothing was changed")
    return fraction


def lost_edit(stop, doing, unused=""):
    """`stop` as call() raised it, explained when Play had ended the edit.

    call()'s message is kept whole and only added to: it is shared, and
    play/push-screenshots.py reads Play's status back out of its
    "-> HTTP <code>" text.
    """
    message = str(stop.code)
    if EDIT_DELETED not in message.lower():
        return stop
    return SystemExit(
        f"{message}\n\n"
        f"Play ended this edit while {doing}. Play lets one account hold one open edit, so an "
        "edit opened as the same service account (scripts/release-status.py, play-publish.py "
        "tracks, play/push-screenshots.py, another upload or promote) takes its place, and a "
        "commit or a change in the Play Console ends every other edit. Nothing in this edit "
        f"reached Play{unused}. Run the same command again once nothing else is touching "
        "Play: Play steps go one at a time.")


def discarded(token, eid, message):
    """Delete the edit and stop with `message`: nothing in it was committed."""
    try:
        call(f"{BASE}/edits/{eid}", token, "DELETE")
        tail = "Nothing changed on Play; the edit is discarded."
    except (SystemExit, OSError) as error:
        reason = str(error.code if isinstance(error, SystemExit) else error).splitlines()[0]
        tail = (f"Nothing changed on Play. Edit {eid} could not be discarded ({reason}); "
                "it ends when the next edit opens.")
    return SystemExit(f"{message}\n{tail}")


def describe(release):
    codes = ", ".join(str(c) for c in release.get("versionCodes") or []) or "no versionCode"
    line = f"'{release.get('name') or '(unnamed)'}', versionCode {codes}, {release.get('status')}"
    if release.get("userFraction") is not None:
        line += f" to {float(release['userFraction']) * 100:g}% of users"
    return line


def track_lines(name, track):
    releases = track.get("releases") or []
    if not releases:
        return [f"  {name}: (no release)"]
    return [f"  {name}: {describe(r)}" for r in releases]


def promotion_refusal(source, held, production, version, build):
    """Why the source track's release may not go to production now, or None.

    `held` and `production` are the two tracks as Play returned them in this
    edit; `version` and `build` are the mobile ledger's, which
    --confirm-version has already been held to.
    """
    for release in production.get("releases") or []:
        if build in [str(c) for c in release.get("versionCodes") or []]:
            return (f"production already carries versionCode {build} ({describe(release)}); "
                    "there is nothing to promote. A staged rollout is raised or completed in "
                    "the Play Console.")
    releases = held.get("releases") or []
    if len(releases) != 1:
        return (f"'{source}' holds {len(releases)} releases; promote takes a track holding "
                "exactly one")
    release = releases[0]
    codes = [str(c) for c in release.get("versionCodes") or []]
    if len(codes) != 1:
        return (f"the release on '{source}' holds {len(codes)} versionCodes; promote takes "
                "exactly one")
    if codes[0] != build:
        return (f"'{source}' holds versionCode {codes[0]}; the mobile ledger is {version}, "
                f"Build {build}, so it is not this release")
    # Play names a release "<versionCode> (<versionName>)" unless it is given
    # a name; a name in that form that names another version is a bundle
    # built from another ledger.
    named = re.fullmatch(r"\d+ \((.+)\)", release.get("name") or "")
    if named and named.group(1) != version:
        return (f"the release on '{source}' is named '{release['name']}', for {named.group(1)}, "
                f"not {version}")
    if release.get("status") != "completed":
        return (f"the release on '{source}' is {release.get('status')}, not completed: promote "
                "takes a release the track's testers already have")
    if not any((note.get("text") or "").strip() for note in release.get("releaseNotes") or []):
        return (f"the release on '{source}' carries no release notes, so production would show "
                "none; this script writes no notes of its own")
    for other in production.get("releases") or []:
        if other.get("status") != "completed":
            return (f"production holds a release that is {other.get('status')} "
                    f"({describe(other)}); finish or halt it in the Play Console first, so a "
                    "promotion replaces nothing but the completed release")
    return None


def tracks(token):
    _, edit = call(BASE + "/edits", token, "POST", {})
    _, listed = call(f"{BASE}/edits/{edit['id']}/tracks", token)
    for t in listed.get("tracks", []):
        rel = t.get("releases") or []
        codes = [c for r in rel for c in (r.get("versionCodes") or [])]
        print(f"  {t['track']:<12} {', '.join(codes) if codes else '(no release)'}")
    call(f"{BASE}/edits/{edit['id']}", token, "DELETE")   # leave nothing dangling
    return 0


def upload(token, aab, track, status, notes, dry):
    with open(aab, "rb") as f:
        blob = f.read()
    print(f"  {aab}: {len(blob):,} bytes -> track '{track}'")
    _, edit = call(BASE + "/edits", token, "POST", {})
    eid = edit["id"]
    try:
        _, b = call(f"{UPLOAD}/edits/{eid}/bundles?uploadType=media", token, "POST",
                    raw=blob, content_type="application/octet-stream")
        code = b["versionCode"]
        print(f"  uploaded versionCode {code} (sha1 {b.get('sha1')})")
        # An app that has never been published is a DRAFT app, and Play refuses
        # any release on it whose status is not "draft" ("Only releases with
        # status draft may be created on draft app"). The first release of a new
        # app therefore has to be created as a draft here and sent for review
        # from the console; every release after the app is live is "completed".
        release = {"status": status, "versionCodes": [str(code)]}
        if notes:
            release["releaseNotes"] = [{"language": "en-GB", "text": notes}]
        call(f"{BASE}/edits/{eid}/tracks/{track}", token, "PUT",
             {"track": track, "releases": [release]})
        print(f"  assigned {code} to '{track}' with status '{status}'"
              + (f", notes {len(notes)} chars" if notes else ", no release notes"))
        if dry:
            call(f"{BASE}/edits/{eid}", token, "DELETE")
            print("  --dry-run: edit discarded, nothing changed on Play")
            return 0
        _, done = call(f"{BASE}/edits/{eid}:commit", token, "POST")
    except SystemExit as stop:
        raise lost_edit(stop, "the upload was in flight", ", and the versionCode is still unused") from None
    print(f"  committed edit {done.get('id', eid)}")
    return 0


def promote(token, source, version, build, rollout, dry):
    """The source track's one release to production, in one edit.

    Reads both tracks in a fresh edit, refuses unless the source holds this
    ledger's release and production does not yet, then writes production
    alone: the release's versionCode, name and notes as the source holds
    them, "completed", or "inProgress" with the rollout fraction. Play
    validates the edit; --dry-run then deletes it, and a real run commits it.
    """
    _, edit = call(BASE + "/edits", token, "POST", {})
    eid = edit["id"]
    try:
        _, held = call(f"{BASE}/edits/{eid}/tracks/{source}", token)
        _, current = call(f"{BASE}/edits/{eid}/tracks/{PRODUCTION}", token)
        for line in track_lines(source, held) + track_lines(PRODUCTION, current):
            print(line)
        problem = promotion_refusal(source, held, current, version, build)
        if problem:
            raise SystemExit(problem)
        release = held["releases"][0]
        new = {"versionCodes": [build]}
        if release.get("name"):
            new["name"] = release["name"]
        new["releaseNotes"] = copy.deepcopy(release["releaseNotes"])
        if rollout < 1:
            new["status"], new["userFraction"] = "inProgress", rollout
        else:
            new["status"] = "completed"
        for note in new["releaseNotes"]:
            print(f"  notes {note.get('language')} ({len(note.get('text') or '')} chars), carried as they are:")
            for line in (note.get("text") or "").split("\n"):
                print(f"    | {line}")
        call(f"{BASE}/edits/{eid}/tracks/{PRODUCTION}", token, "PUT",
             {"track": PRODUCTION, "releases": [new]})
        print(f"  assigned to production: {describe(new)}")
        call(f"{BASE}/edits/{eid}:validate", token, "POST")
        print("  Play validated the edit")
        if dry:
            call(f"{BASE}/edits/{eid}", token, "DELETE")
            print("  --dry-run: edit discarded, nothing changed on Play")
            return 0
    except SystemExit as stop:
        explained = lost_edit(stop, "the promotion was being prepared")
        if explained is not stop:
            raise explained from None
        raise discarded(token, eid, str(stop.code)) from None
    try:
        _, done = call(f"{BASE}/edits/{eid}:commit?{IN_REVIEW_QUERY}", token, "POST")
    except SystemExit as stop:
        message = str(stop.code)
        explained = lost_edit(stop, "the promotion was being committed", ", and production is unchanged")
        if explained is not stop:
            raise explained from None
        status = re.search(r"-> HTTP (\d{3})\b", message)
        if status and 400 <= int(status.group(1)) < 500:
            raise discarded(token, eid, (
                f"{message}\n\nPlay refused the commit, so production is unchanged. A refusal "
                "over changes in review means something else, an earlier upload or a listing "
                "change, is still in Play's review: wait for it to clear in the Play Console, "
                "then run the same command again.")) from None
        raise SystemExit(unknown_outcome(message, build)) from None
    except (OSError, KeyboardInterrupt) as error:
        raise SystemExit(unknown_outcome(str(error) or type(error).__name__, build)) from None
    print(f"  committed edit {done.get('id', eid)}: production is {describe(new)}, "
          "in Play's review before readers see it")
    return 0


def unknown_outcome(reason, build):
    return (f"the commit's outcome is unknown: {reason}\n"
            "Play may have taken the commit before the answer was lost. Run "
            "`scripts/play-publish.py tracks` on its own before anything else; a re-run of "
            f"this promote refuses once production carries versionCode {build}.")


def main(argv):
    argv = list(argv)
    dry = "--dry-run" in argv
    argv = [a for a in argv if a != "--dry-run"]
    status = take(argv, "--status", "a value: draft or completed")
    status_given = status is not None
    if status is None:
        status = "completed"
    if status not in ("draft", "completed"):
        raise SystemExit(f"--status must be draft or completed, not {status!r}")
    # --notes <file>: the release notes for the en-GB listing, carried on the
    # track release itself. Play caps them at 500 characters per language and
    # accepts a release without any, so a release sent from here would
    # otherwise reach review with the field empty.
    notes_path = take(argv, "--notes", "a file path")
    notes = read_notes(notes_path) if notes_path is not None else None
    confirm = take(argv, "--confirm-version", "the version")
    rollout_text = take(argv, "--rollout", "a fraction")
    cmd = argv[1] if len(argv) > 1 else "tracks"
    if cmd != "promote" and (confirm is not None or rollout_text is not None):
        raise SystemExit("--confirm-version and --rollout belong to promote")

    if cmd == "tracks":
        return tracks(access_token())

    if cmd == "upload":
        if len(argv) < 3:
            raise SystemExit("usage: play-publish.py [--status draft|completed] [--notes file] upload <bundle.aab> [track]")
        aab, track = argv[2], (argv[3] if len(argv) > 3 else "internal")
        if track == PRODUCTION:
            raise SystemExit("upload goes to a testing track; production is reached only by "
                             "promote, on the account holder's OK")
        return upload(access_token(), aab, track, status, notes, dry)

    if cmd == "promote":
        if len(argv) != 4:
            raise SystemExit("usage: play-publish.py promote <from-track> production "
                             "--confirm-version <v> [--rollout <fraction>] [--dry-run]")
        source, target = argv[2], argv[3]
        if target != PRODUCTION or source in ("", PRODUCTION):
            raise SystemExit(f"promote goes from a testing track to production, not "
                             f"'{source}' to '{target}'")
        if notes_path is not None or status_given:
            raise SystemExit("promote carries the release's own notes and status from the "
                             f"'{source}' track; --notes and --status belong to upload")
        version, build = ledger()
        if confirm != version:
            raise SystemExit(f"promote requires --confirm-version {version}, the mobile ledger's "
                             "Version; nothing was changed")
        rollout = parse_rollout(rollout_text) if rollout_text is not None else 1.0
        print(f"  ledger {version}, Build {build}: '{source}' -> production"
              + (", --dry-run" if dry else ""))
        return promote(access_token(), source, version, build, rollout, dry)

    raise SystemExit(f"unknown command: {cmd}")


if __name__ == "__main__":
    sys.exit(main(sys.argv))
