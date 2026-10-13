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
                                               breaks kept, at most 500 characters;
                                               without it, the tracked
                                               play/metadata/en-GB/whats-new-<v>.txt
                                               for the mobile ledger's Version
  play-publish.py promote <from-track> production --confirm-version <v>
                  [--rollout <fraction>] [--dry-run]
                                               the from-track's one release to
                                               production, its notes and name as
                                               they are; --dry-run discards the edit

WHAT AN UPLOAD IS HELD TO. The bundle Play reports back is checked against
this tree before the commit: its versionCode must be the mobile ledger's
Build, and the sha1 Play computed over the bytes it received must be the
file's, or the edit is discarded with nothing changed. After the commit the
track is read back in a fresh edit and printed, and the release Play now
holds must carry that versionCode with the status asked for; Play names a
release it was given no name for after the bundle's versionName, so the
read-back is also where the bundle's versionName is held to the ledger's
Version. A promotion reads production back the same way.

ONE PLAY STEP AT A TIME. Every command here opens an edit as the one service
account, and Play lets each user hold one open edit: a new edit ends the one
that user already has open, and a commit, or any change in the Play Console,
ends every other edit for the app. An upload whose edit is ended that way
says so (`EDIT_DELETED`). So uploads, promotions, `tracks`,
`scripts/release-status.py` and `play/push-screenshots.py` run one at a time.

A REVIEW IN PROGRESS IS NEVER RESTARTED. An upload and a promotion commit with
`IN_REVIEW_QUERY`, so while anything else is in Play's review, a production
release or a listing change, Play refuses the commit rather than cancel that
review and send everything again. The edit is then discarded with nothing
changed, and the same command is run again once the review clears. For a
promotion the likeliest thing in review is this release's own alpha upload:
Google reviews every track change, and only the Play Console's Publishing
overview shows when that review has cleared.
"""
import base64, copy, hashlib, json, os, re, sys, time, urllib.parse, urllib.request
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
# The release notes, tracked per release as the Microsoft Store's are
# (msstore/metadata/<language>/whats-new-<version>.txt). `upload` reads the
# ledger's file unless --notes names another, and play_whats_new_test.go
# holds the file equal to the blockquote under "Release notes — <version>"
# in docs/PLAY_LISTING.md, so what is sent is what was read.
NOTES_DIR = os.path.join(REPO, "play", "metadata", "en-GB")
PRODUCTION = "production"
NOTES_CAP = 500

# edits.commit's changesInReviewBehavior defaults to CANCEL_IN_REVIEW_AND_SUBMIT:
# a commit made while other changes are in review cancels that review and sends
# everything again. ERROR_IF_IN_REVIEW makes Play refuse the commit instead, so
# neither an upload nor a promotion restarts the review of something else; the
# command waits and is run again once that review clears
# (play/push-screenshots.py commits the same way, and says more).
IN_REVIEW_QUERY = "changesInReviewBehavior=ERROR_IF_IN_REVIEW"

# The status call()'s exit text carries: "Play API <method> <url> -> HTTP <code>".
HTTP_STATUS = re.compile(r"-> HTTP (\d{3})\b")

# Play's refusal of a commit made with IN_REVIEW_QUERY while changes are in
# review: HTTP 400, the reason CHANGES_ALREADY_IN_REVIEW, and a message that
# says in words that changes are in review.
IN_REVIEW_REFUSAL = re.compile(r"(?i)in review|CHANGES_ALREADY_IN_REVIEW")

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

    The whole loses the whitespace at its two ends, which takes the blank
    lines there and the first line's indent, and each line loses its
    trailing spaces; the line breaks between lines stay, and so does the
    indent of every line after the first, so the opening line and its
    bullets reach Play as lines, not as one paragraph with the bullets
    inline. Play caps a language's notes at 500 characters, and every line
    break is one of them.
    """
    try:
        with open(path, encoding="utf-8") as f:
            text = f.read()
    except OSError as error:
        raise SystemExit(f"--notes: cannot read {path} ({error.strerror or error}); "
                         "nothing was changed") from None
    notes = "\n".join(line.rstrip() for line in text.strip().splitlines())
    if not notes:
        raise SystemExit(f"--notes: {path} is empty")
    if len(notes) > NOTES_CAP:
        raise SystemExit(f"--notes: {len(notes)} characters, line breaks included; "
                         f"Play caps release notes at {NOTES_CAP}")
    return notes


def default_notes(version):
    """The tracked release notes file for the ledger's Version."""
    return os.path.join(NOTES_DIR, f"whats-new-{version}.txt")


def shown(path):
    """`path` relative to the repository when it is inside it."""
    if path.startswith(REPO + os.sep):
        return path[len(REPO) + 1:]
    return path


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


def named_version(name):
    """The version a release name Play generated carries, or None for a name
    given by hand.

    A release given no name is named by Play after the bundle's versionName,
    "1.2.21", which is the only form the Developer API has returned; the
    Play Console shows a release as "<versionCode> (<versionName>)", and a
    name in that form is read the same way. Either names the version the
    bundle was built with. Any other name was given by hand and names none.
    """
    if re.fullmatch(r"\d+(\.\d+)+", name):
        return name
    shown = re.fullmatch(r"\d+ \((\d+(\.\d+)+)\)", name)
    return shown.group(1) if shown else None


def promotion_refusal(source, held, production, version, build):
    """Why the source track's release may not go to production now, or None.

    `held` and `production` are the two tracks as Play returned them in this
    edit; `version` and `build` are the mobile ledger's, which
    --confirm-version has already been held to.
    """
    for release in production.get("releases") or []:
        codes = [str(c) for c in release.get("versionCodes") or []]
        if build in codes:
            return (f"production already carries versionCode {build} ({describe(release)}); "
                    "there is nothing to promote. A staged rollout is raised or completed in "
                    "the Play Console.")
        above = [c for c in codes if c.isdigit() and int(c) > int(build)]
        if above:
            return (f"production carries versionCode {above[0]} ({describe(release)}), above "
                    f"the mobile ledger's Build {build}: this tree is behind production, and "
                    "promote would put an older build there")
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
    # Play names a release after its bundle's versionName unless it is given
    # a name; a name that names another version is a bundle built from
    # another ledger.
    carried = named_version(release.get("name") or "")
    if carried is not None and carried != version:
        return (f"the release on '{source}' is named '{release['name']}', for {carried}, "
                f"not {version}")
    if release.get("status") != "completed":
        return (f"the release on '{source}' is {release.get('status')}, not completed: promote "
                "takes a release the track's testers already have")
    if not any((note.get("text") or "").strip() for note in release.get("releaseNotes") or []):
        return (f"the release on '{source}' carries no release notes, so production would show "
                "none; this script writes no notes of its own")
    # The promotion writes production's release list whole, so whatever the
    # track holds gives way to it. A completed release and a halted staged
    # rollout may (a halted rollout is where a fix is promoted from); a
    # rollout still going out, or a draft made in the Play Console, is
    # someone's unfinished step and is settled there first.
    for other in production.get("releases") or []:
        status = other.get("status")
        if status in ("completed", "halted"):
            continue
        if status == "draft":
            return (f"production holds a draft release ({describe(other)}); roll it out or "
                    "discard it in the Play Console first")
        return (f"production holds a release that is {status} ({describe(other)}); complete "
                "or halt its rollout in the Play Console first: promote replaces a completed "
                "or halted release, never one still rolling out")
    return None


def bundle_problem(answer, blob, version, build):
    """Why the bundle Play reports is not this tree's, or None.

    `answer` is Play's description of the bundle it received: the
    versionCode it read from the manifest and the sha1 it computed over the
    bytes. Both are compared before anything is committed, the versionCode
    with the mobile ledger's Build and the sha1 with the file's, so a bundle
    built from another ledger, or bytes Play received short, never reach a
    track.
    """
    code = str(answer.get("versionCode"))
    if code != build:
        return (f"Play holds the bundle as versionCode {code}; the mobile ledger is {version}, "
                f"Build {build}, so it is not the bundle this tree builds")
    ours = hashlib.sha1(blob).hexdigest()
    theirs = str(answer.get("sha1") or "").lower()
    if theirs != ours:
        return (f"Play reports sha1 {theirs or '(none)'} for the bundle it received; the file's "
                f"is {ours}, so the bytes Play holds are not the file's")
    return None


def committed_problem(name, held, version, build, status):
    """Why the track read back after a commit does not hold the release written, or None."""
    releases = [r for r in held.get("releases") or []
                if build in [str(c) for c in (r.get("versionCodes") or [])]]
    if not releases:
        return f"'{name}' read back without versionCode {build}"
    release = releases[0]
    # Play names a release it was given no name for after the bundle's
    # versionName, so this is where the bundle's versionName shows.
    carried = named_version(release.get("name") or "")
    if carried is not None and carried != version:
        return (f"'{name}' read back with versionCode {build} named '{release['name']}', for "
                f"{carried}, not {version}: the bundle's versionName is not the mobile "
                "ledger's Version")
    if release.get("status") != status:
        return (f"'{name}' read back with versionCode {build} {release.get('status')}, not "
                f"{status}")
    return None


def read_track(token, name):
    """The track as committed, read in an edit of its own, which is then deleted."""
    _, edit = call(BASE + "/edits", token, "POST", {})
    eid = edit["id"]
    try:
        _, held = call(f"{BASE}/edits/{eid}/tracks/{name}", token)
    finally:
        try:
            call(f"{BASE}/edits/{eid}", token, "DELETE")
        except (SystemExit, OSError):
            pass   # a discarded edit expires on its own; the read is what matters
    return held


def read_back(token, name, version, build, status):
    """Print the track as Play holds it after the commit, held to the release written.

    The commit has gone through by now, so a disagreement is reported, not
    undone: Play holds what was committed, and the next step waits until the
    track is settled in the Play Console.
    """
    try:
        held = read_track(token, name)
    except (SystemExit, OSError) as error:
        reason = str(error.code if isinstance(error, SystemExit) else error)
        raise SystemExit(
            f"{reason}\n\nThe commit went through, and the read-back of '{name}' did not: run "
            "`scripts/play-publish.py tracks` on its own to see what Play holds.") from None
    print("  read back after the commit:")
    for line in track_lines(name, held):
        print(line)
    problem = committed_problem(name, held, version, build, status)
    if problem:
        raise SystemExit(f"The commit went through, but {problem}. Play holds what was committed "
                         "and nothing here can discard it; settle the track in the Play Console "
                         "before the next step.")


def tracks(token):
    _, edit = call(BASE + "/edits", token, "POST", {})
    _, listed = call(f"{BASE}/edits/{edit['id']}/tracks", token)
    for t in listed.get("tracks", []):
        rel = t.get("releases") or []
        codes = [c for r in rel for c in (r.get("versionCodes") or [])]
        print(f"  {t['track']:<12} {', '.join(codes) if codes else '(no release)'}")
    call(f"{BASE}/edits/{edit['id']}", token, "DELETE")   # leave nothing dangling
    return 0


def upload(token, aab, track, status, notes, version, build, dry):
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
        problem = bundle_problem(b, blob, version, build)
        if problem:
            raise discarded(token, eid, f"{problem}; the bundle is not on '{track}'.")
        print(f"  the ledger's Build {build} and the file's sha1, as Play holds them")
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
    except SystemExit as stop:
        raise lost_edit(stop, "the upload was in flight", ", and the versionCode is still unused") from None
    try:
        _, done = call(f"{BASE}/edits/{eid}:commit?{IN_REVIEW_QUERY}", token, "POST")
    except SystemExit as stop:
        explained = lost_edit(stop, "the upload was in flight", ", and the versionCode is still unused")
        if explained is not stop:
            raise explained from None
        message = str(stop.code)
        status = HTTP_STATUS.search(message)
        if status and 400 <= int(status.group(1)) < 500:
            raise discarded(token, eid, (
                f"{message}\n\nPlay refused the commit, so the bundle is not on '{track}' and "
                f"versionCode {code} is still unused. A refusal over changes in review means "
                "something else, a production release or a listing change, is still in Play's "
                "review, and this upload waits rather than send that review back to the start: "
                "once the Play Console's Publishing overview shows nothing in review, run the "
                "same command again.")) from None
        raise
    print(f"  committed edit {done.get('id', eid)}")
    read_back(token, track, version, build, status)
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
        for other in current.get("releases") or []:
            if other.get("status") == "halted":
                print(f"  in place of production's halted release {describe(other)}")
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
        status = HTTP_STATUS.search(message)
        if status and 400 <= int(status.group(1)) < 500:
            if IN_REVIEW_REFUSAL.search(message):
                why = (f"Play refused the commit over changes in review, so production is "
                       f"unchanged. The likeliest is this release's own '{source}' upload: Google "
                       "reviews every track change, and until the Play Console's Publishing "
                       f"overview shows the {source} release published, a promotion is refused "
                       "rather than cancel that review and start it again. Otherwise an earlier "
                       "production release or a listing change is still in review. Read the "
                       "Publishing overview, and once it shows nothing in review run the same "
                       "command again; the refusal changed nothing.")
            else:
                why = "Play refused the commit, so production is unchanged."
            raise discarded(token, eid, f"{message}\n\n{why}") from None
        raise SystemExit(unknown_outcome(message, build)) from None
    except (Exception, KeyboardInterrupt) as error:
        # Anything else once the commit has been sent -- a lost connection, a
        # truncated or unreadable answer, an interrupt -- leaves Play's side
        # unknown, so every such failure gets the same advice.
        reason = f"{type(error).__name__}: {error}" if str(error) else type(error).__name__
        raise SystemExit(unknown_outcome(reason, build)) from None
    print(f"  committed edit {done.get('id', eid)}: production is {describe(new)}, "
          "in Play's review before readers see it")
    read_back(token, PRODUCTION, version, build, new["status"])
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
    confirm = take(argv, "--confirm-version", "the version")
    rollout_text = take(argv, "--rollout", "a fraction")
    # Every option the script knows is out of argv by now, so anything left
    # that looks like one is mistyped. Read as a track, or dropped, a
    # --dryrun or a --rollout=0.2 would turn a dry run into a commit or a
    # staged rollout into a full one.
    stray = [a for a in argv[1:] if a.startswith("-")]
    if stray:
        raise SystemExit(f"unknown option {' '.join(stray)}; nothing was changed")
    cmd = argv[1] if len(argv) > 1 else "tracks"
    if cmd != "promote" and (confirm is not None or rollout_text is not None):
        raise SystemExit("--confirm-version and --rollout belong to promote")

    if cmd == "tracks":
        return tracks(access_token())

    if cmd == "upload":
        if not 3 <= len(argv) <= 4:
            raise SystemExit("usage: play-publish.py [--status draft|completed] [--notes file] upload <bundle.aab> [track]")
        aab, track = argv[2], (argv[3] if len(argv) > 3 else "internal")
        if track == PRODUCTION:
            raise SystemExit("upload goes to a testing track; production is reached only by "
                             "promote, on the account holder's OK")
        version, build = ledger()
        if notes_path is None:
            notes_path = default_notes(version)
            if not os.path.exists(notes_path):
                raise SystemExit(
                    f"no release notes for {version}: {shown(notes_path)} is missing. upload "
                    "reads the tracked play/metadata/en-GB/whats-new-<version>.txt for the mobile "
                    "ledger's Version, the text of the blockquote under 'Release notes — "
                    "<version>' in docs/PLAY_LISTING.md, unless --notes names another file; "
                    "nothing was changed")
        notes = read_notes(notes_path)
        print(f"  ledger {version}, Build {build}; release notes from {shown(notes_path)}")
        return upload(access_token(), aab, track, status, notes, version, build, dry)

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
