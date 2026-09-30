#!/usr/bin/env python3
"""Validate and optionally upload the Google Play screenshot set.

Two image types on the en-GB store listing are handled: phoneScreenshots,
from <set>/phone/, and tenInchScreenshots, from <set>/tablet10/. The feature
graphic, the icon, the seven-inch, TV and Wear types, and every other
language are never touched. The set on disk is build/play/screenshots-<version>/,
or ``--set-dir``. Files go up in sorted name order, which is what their
numbers are for.

Every mode checks the files first and makes no network request when any file
fails: 1 to 8 files per type, each a PNG in 8-bit RGB (colour type 2) with no
tRNS chunk, at most 8 MB, no two alike, the phone at exactly 1080x2160 and
the tablet at exactly 2560x1600. A dotfile such as .DS_Store is ignored; any
other file that is not a PNG is refused rather than skipped.

  (no mode flag)   READ-ONLY. Open an edit, list the listing's languages and
                   the images each handled type holds, by sha256, print the
                   plan, then delete the edit.
  --local-only     the file checks alone; no network request.
  --rehearse       everything --write does except the commit: in one edit,
                   delete every image of each type that changes, upload each
                   file in order, read the type back (count, order, and each
                   image's sha256 against its file's), validate the edit, then
                   delete it. Nothing reaches the listing.
  --write --confirm-version <v>
                   the rehearsal, then commit the edit, then open a fresh
                   edit and read the committed listing back the same way.
                   A commit sends the change to Play's review; the store
                   shows the new images only after that.

Every mode but --local-only opens an edit, and Play keeps one edit open per
user: opening one invalidates any other edit the same service account has
open, scripts/play-publish.py's included. So a read-only run leaves the
listing alone but not another run's edit; run nothing else against the Play
account while --rehearse or --write runs.

Any failure after an edit is opened deletes that edit; only a committed edit
is left, because a commit consumes it. A commit that gets no definite answer
(no response, a server error, an interrupt) may still have gone through: the
tool says the outcome is unknown, and a read-only run shows what the listing
holds. The token comes from scripts/play-publish.py's access_token(), which
reads the service-account key it names; nothing here prints the token or
the key.
"""

from __future__ import annotations

import argparse
import collections
import hashlib
import importlib.util
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
PLAY_PUBLISH = os.path.join(REPO, "scripts", "play-publish.py")
LANGUAGE = "en-GB"

# (directory under the set, Play's AppImageType, the one size taken). The
# phone is exactly 2:1, the longest a Play screenshot may be; the tablet is
# the Pixel Tablet profile's own display (docs/SCREENSHOT_PLAYBOOK.md, §3).
IMAGE_TYPES = (
    ("phone", "phoneScreenshots", (1080, 2160)),
    ("tablet10", "tenInchScreenshots", (2560, 1600)),
)

# Play shows at most eight screenshots per device type and takes a file of
# at most 8 MB.
MAX_IMAGES = 8
MAX_BYTES = 8 * 1024 * 1024

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"

# The commit's review parameters (edits.commit, Google Play Developer API v3:
# https://developers.google.com/android-publisher/api-ref/rest/v3/edits/commit).
#
# changesInReviewBehavior defaults to CANCEL_IN_REVIEW_AND_SUBMIT: a commit
# made while other changes are in review cancels that review and sends
# everything again. ERROR_IF_IN_REVIEW makes Play refuse the commit instead,
# without invalidating the edit, so a screenshot change never restarts the
# review of something else.
#
# changesNotSentForReview=true is for the app whose changes Play will not
# send for review automatically; it answers such a commit with HTTP 400,
# "Changes cannot be sent for review automatically. Please set the query
# parameter changesNotSentForReview to true." With it, the changes in the
# edit "won't be reviewed until they are explicitly sent for review from
# within the Google Play Console UI" and join any other changes not yet sent.
# It is passed only when --changes-not-sent-for-review asks for it, and then
# alone: nothing is sent for review, so nothing in review is touched.
IN_REVIEW_QUERY = "changesInReviewBehavior=ERROR_IF_IN_REVIEW"
NOT_SENT_QUERY = "changesNotSentForReview=true"
NOT_SENT_MARKER = "changesNotSentForReview"

# play-publish.py's call() ends the process with "Play API <method> <url> ->
# HTTP <code>" and the body when Play answers with an error.
HTTP_STATUS = re.compile(r"-> HTTP (\d{3})\b")

LocalImage = collections.namedtuple("LocalImage", "name path size sha256 width height")
LocalSet = collections.namedtuple("LocalSet", "subdirectory image_type directory images")


class PlayError(Exception):
    """A request the Play Developer API refused or that did not complete.

    ``status`` is the HTTP status of Play's answer, or None when no answer
    came back.
    """

    def __init__(self, message, status=None):
        super().__init__(message)
        self.status = status


class NotSentForReview(Exception):
    """Play refused the commit because it cannot send the change for review."""


class CommitOutcomeUnknown(Exception):
    """The commit was sent and no definite answer came back."""


def png_header(data):
    """(width, height, bit depth, colour type, has tRNS), or None if not a PNG.

    A chunk walk with no image library: the signature, then IHDR first as the
    format requires, then every chunk header up to IEND.
    """
    if data[:8] != PNG_SIGNATURE or data[12:16] != b"IHDR" or len(data) < 33:
        return None
    width = int.from_bytes(data[16:20], "big")
    height = int.from_bytes(data[20:24], "big")
    bit_depth, colour_type = data[24], data[25]
    has_trns, offset = False, 8
    while offset + 8 <= len(data):
        length = int.from_bytes(data[offset:offset + 4], "big")
        kind = data[offset + 4:offset + 8]
        if kind == b"tRNS":
            has_trns = True
        if kind == b"IEND":
            break
        offset += 12 + length
    return width, height, bit_depth, colour_type, has_trns


def sha256_hex(data):
    return hashlib.sha256(data).hexdigest()


def read_local_set(set_dir):
    """Every handled type's files, checked, in upload order.

    Every problem is collected and reported together, before any network
    request.
    """
    sets, problems = [], []
    for subdirectory, image_type, (want_w, want_h) in IMAGE_TYPES:
        directory = os.path.join(set_dir, subdirectory)
        if not os.path.isdir(directory):
            problems.append(f"{image_type}: {directory} is missing")
            continue
        names = sorted(
            name for name in os.listdir(directory)
            if not name.startswith(".") and os.path.isfile(os.path.join(directory, name))
        )
        if not names:
            problems.append(f"{image_type}: {directory} holds no PNG")
            continue
        if len(names) > MAX_IMAGES:
            problems.append(f"{image_type}: {directory} holds {len(names)} files; "
                            f"Play takes at most {MAX_IMAGES}")
        images, seen = [], {}
        for name in names:
            path = os.path.join(directory, name)
            with open(path, "rb") as handle:
                data = handle.read()
            header = png_header(data) if name.lower().endswith(".png") else None
            if header is None:
                problems.append(f"{image_type}: {name} is not a PNG")
                continue
            width, height, bit_depth, colour_type, has_trns = header
            if colour_type in (4, 6) or has_trns:
                problems.append(f"{image_type}: {name} carries an alpha channel; "
                                "Play takes 24-bit PNG without alpha")
            elif colour_type != 2 or bit_depth != 8:
                problems.append(f"{image_type}: {name} is colour type {colour_type} at "
                                f"bit depth {bit_depth}; Play takes 24-bit RGB PNG "
                                "(colour type 2, bit depth 8)")
            if (width, height) != (want_w, want_h):
                problems.append(f"{image_type}: {name} is {width}x{height}; "
                                f"this type takes exactly {want_w}x{want_h}")
            if len(data) > MAX_BYTES:
                problems.append(f"{image_type}: {name} is {len(data)} bytes; "
                                f"Play takes at most {MAX_BYTES}")
            digest = sha256_hex(data)
            if digest in seen:
                problems.append(f"{image_type}: {name} is the same file as {seen[digest]}")
            seen[digest] = name
            images.append(LocalImage(name, path, len(data), digest, width, height))
        sets.append(LocalSet(subdirectory, image_type, directory, images))
    if problems:
        raise SystemExit("local screenshot set is not upload-ready:\n"
                         + "\n".join(f"  - {problem}" for problem in problems))
    return sets


class Client:
    """Every remote call, through play-publish.py's call() and its token.

    ``api`` reaches .../applications/<package>/<path>; ``upload`` reaches the
    media upload host with the PNG's bytes. call() ends the process on an HTTP
    error; here that becomes a PlayError, so the caller's cleanup still runs
    and the commit can tell a refusal it knows from one it does not. The token
    is held here and nowhere else.
    """

    def __init__(self, play_publish, token):
        self._pp = play_publish
        self._token = token

    def _call(self, url, method, **kwargs):
        try:
            _status, body = self._pp.call(url, self._token, method, **kwargs)
        except SystemExit as stop:
            message = str(stop.code)
            status = HTTP_STATUS.search(message)
            raise PlayError(message, int(status.group(1)) if status else None) from None
        except OSError as error:
            # A connection that failed or timed out, never an answer; the
            # URL is printed without its query and the token is not in it.
            raise PlayError(f"{method} {url.split('?')[0]}: {error}") from None
        return body if isinstance(body, dict) else {}

    def api(self, method, path, payload=None):
        return self._call(self._pp.BASE + path, method, payload=payload)

    def upload(self, path, data):
        return self._call(self._pp.UPLOAD + path + "?uploadType=media", "POST",
                          raw=data, content_type="image/png")


def load_play_publish():
    spec = importlib.util.spec_from_file_location("play_publish", PLAY_PUBLISH)
    if spec is None or spec.loader is None:
        raise SystemExit(f"cannot load {PLAY_PUBLISH}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def connect():
    """The real client, loaded only after the entirely local checks."""
    play_publish = load_play_publish()
    try:
        token = play_publish.access_token()
    except FileNotFoundError:
        raise SystemExit("no Play service-account key: set BIBLETEXT_PLAY_KEY or place it "
                         "where scripts/play-publish.py looks") from None
    return Client(play_publish, token)


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--version", required=True,
                        help="the release the set was captured from; names the default set "
                             "directory and is what --confirm-version must repeat")
    parser.add_argument("--set-dir", metavar="DIR",
                        help="the set (default: build/play/screenshots-<version>)")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--local-only", action="store_true",
                      help="check the files without any network request")
    mode.add_argument("--rehearse", action="store_true",
                      help="upload into an edit, read it back, validate it, delete it")
    mode.add_argument("--write", action="store_true",
                      help="the rehearsal, then commit, then read the committed listing back")
    parser.add_argument("--confirm-version", metavar="VERSION",
                        help="required with --write; must equal --version exactly")
    parser.add_argument("--changes-not-sent-for-review", action="store_true",
                        help="with --write: commit with changesNotSentForReview=true, "
                             "leaving the change to be sent for review from the console")
    args = parser.parse_args(argv)
    if not re.fullmatch(r"[0-9]+(\.[0-9]+)*", args.version):
        parser.error(f"--version {args.version!r} is not a version number")
    if args.write and args.confirm_version != args.version:
        parser.error(f"--write requires --confirm-version {args.version}; "
                     "no network request was made")
    if args.confirm_version is not None and not args.write:
        parser.error("--confirm-version is meaningful only with --write")
    if args.changes_not_sent_for_review and not args.write:
        parser.error("--changes-not-sent-for-review is meaningful only with --write")
    return args


def short(digest):
    return (digest or "none")[:16]


def list_images(client, edit_id, image_type):
    body = client.api("GET", f"/edits/{edit_id}/listings/{LANGUAGE}/{image_type}")
    images = body.get("images") or []
    if not isinstance(images, list):
        raise PlayError(f"list {image_type}: the response carried no image list")
    return images


def open_edit(client):
    edit_id = client.api("POST", "/edits", {}).get("id")
    if not edit_id:
        raise PlayError("open edit: the response carried no id")
    print(f"opened edit {edit_id}")
    return edit_id


def discard_edit(client, edit_id):
    """Delete an edit; report, never raise, so a failure in flight survives."""
    try:
        client.api("DELETE", f"/edits/{edit_id}")
    except Exception as error:  # pylint: disable=broad-except
        print(f"[ERROR] could not delete edit {edit_id}: {str(error)[:300]}")
        return False
    print(f"deleted edit {edit_id}")
    return True


def check_languages(client, edit_id):
    listings = client.api("GET", f"/edits/{edit_id}/listings").get("listings") or []
    languages = sorted(item.get("language") for item in listings if item.get("language"))
    print(f"listing languages: {', '.join(languages) or 'none'}")
    others = [language for language in languages if language != LANGUAGE]
    if others:
        print(f"  images are per language: {', '.join(others)} keep their own images; "
              f"only {LANGUAGE} is touched")
    if LANGUAGE not in languages:
        raise PlayError(f"the app has no {LANGUAGE} listing to hold the images")


def plan(client, edit_id, local_sets):
    """Print what each type holds and what would replace it.

    Returns the sets whose images differ from the files, by sha256 and order,
    and each type's sha256 list as the edit held it.
    """
    changing, held_by_type = [], {}
    for local in local_sets:
        current = list_images(client, edit_id, local.image_type)
        print(f"\n{local.image_type} ({LANGUAGE})  <-  {local.directory}")
        print(f"  now {len(current)} image(s):")
        for number, image in enumerate(current, 1):
            print(f"    {number}. sha256 {short(image.get('sha256'))}  id {image.get('id')}")
        print(f"  new {len(local.images)} file(s), in order:")
        for number, image in enumerate(local.images, 1):
            print(f"    {number}. sha256 {short(image.sha256)}  {image.name}  "
                  f"{image.width}x{image.height}  {image.size} B")
        held = [(image.get("sha256") or "").lower() for image in current]
        held_by_type[local.image_type] = held
        if held == [image.sha256 for image in local.images]:
            print("  already holds these files in this order; left alone")
            continue
        print(f"  would delete all {len(current)} and upload {len(local.images)}")
        changing.append(local)
    return changing, held_by_type


def compare(image_type, listed, local, ids=None):
    """Problems between a listing and the files: count, order, sha256."""
    problems = []
    if len(listed) != len(local.images):
        problems.append(f"{image_type} holds {len(listed)} image(s), expected {len(local.images)}")
    for number, (image, want) in enumerate(zip(listed, local.images), 1):
        got = (image.get("sha256") or "").lower()
        if got != want.sha256:
            problems.append(f"{image_type} position {number}: sha256 {got or 'none'}, "
                            f"expected {want.sha256} ({want.name})")
        if ids is not None and image.get("id") != ids[number - 1]:
            problems.append(f"{image_type} position {number}: id {image.get('id')}, "
                            f"expected {ids[number - 1]} ({want.name})")
    return problems


def replace(client, edit_id, local):
    """deleteall, then one upload per file in order, then the read-back."""
    print(f"\n{local.image_type}")
    client.api("DELETE", f"/edits/{edit_id}/listings/{LANGUAGE}/{local.image_type}")
    left = list_images(client, edit_id, local.image_type)
    if left:
        raise PlayError(f"{local.image_type} still holds {len(left)} image(s) after deleteall")
    print(f"  deleted every {local.image_type} image in the edit")
    ids = []
    for number, image in enumerate(local.images, 1):
        with open(image.path, "rb") as handle:
            data = handle.read()
        if sha256_hex(data) != image.sha256:
            raise SystemExit(f"{image.name} changed on disk since it was checked")
        uploaded = client.upload(
            f"/edits/{edit_id}/listings/{LANGUAGE}/{local.image_type}", data).get("image") or {}
        reported = (uploaded.get("sha256") or "").lower()
        if reported and reported != image.sha256:
            raise PlayError(f"{image.name}: Play reports sha256 {reported}, "
                            f"the file is {image.sha256}")
        if not uploaded.get("id"):
            raise PlayError(f"upload {image.name}: the response carried no image id")
        ids.append(uploaded["id"])
        print(f"  uploaded {number}. {image.name}  sha256 {short(image.sha256)}")
    problems = compare(local.image_type, list_images(client, edit_id, local.image_type),
                       local, ids)
    if problems:
        raise SystemExit("read-back mismatch:\n"
                         + "\n".join(f"  - {problem}" for problem in problems))
    print(f"  read back {len(ids)}: count, order and sha256 agree")


def commit(client, edit_id, not_sent):
    """POST :commit, and tell a refusal from an outcome nobody knows.

    An HTTP 4xx is Play refusing the commit, so nothing was committed; it is
    raised as it came, or as NotSentForReview for the refusal that flag
    answers. Anything else (no answer, a 5xx, an answer that cannot be read,
    an interrupt) may have reached Play after the commit took effect, so it
    is CommitOutcomeUnknown and never reported as a plain stop.
    """
    query = NOT_SENT_QUERY if not_sent else IN_REVIEW_QUERY
    try:
        client.api("POST", f"/edits/{edit_id}:commit?{query}")
    except PlayError as error:
        if error.status is not None and 400 <= error.status < 500:
            if NOT_SENT_MARKER in str(error) and not not_sent:
                raise NotSentForReview(str(error)) from None
            raise
        raise CommitOutcomeUnknown(str(error)) from None
    except (Exception, KeyboardInterrupt) as error:  # pylint: disable=broad-except
        raise CommitOutcomeUnknown(str(error) or type(error).__name__) from None
    if not_sent:
        print(f"committed edit {edit_id}, NOT sent for review")
    else:
        print(f"committed edit {edit_id}; Play reviews it before the store shows it")


def not_sent_message(edit_id, deleted):
    return ("Play refused the commit: these changes cannot be sent for review "
            "automatically. Nothing was committed; "
            + (f"edit {edit_id} is deleted.\n" if deleted else
               f"edit {edit_id} was NOT deleted (the error is above); "
               "delete it before another run.\n")
            + "--changes-not-sent-for-review commits them without sending them; "
            "they then wait in the Play Console until sent for review there.")


def unknown_message(edit_id, reason, deleted):
    return (f"the commit's outcome is unknown: {reason}\n"
            "Play may have taken the commit before the answer was lost, so the "
            "listing may already hold the new images.\n"
            + (f"edit {edit_id} was deleted after the commit was sent.\n" if deleted else
               f"edit {edit_id} could not be deleted after the commit was sent (the error "
               "is above); a commit that went through ends its edit.\n")
            + "run the tool read-only to see what the listing holds before any retry.")


def verify_committed(client, local_sets, before, not_sent):
    """A fresh edit shows the listing as committed; hold it to the files.

    This is the committed state, not what the store shows: the store shows a
    change only after Play's review, and one committed with
    changesNotSentForReview waits in the console until it is sent. Google's
    edits overview says a new edit "is a copy of the current deployed state
    of the app"; if that state lags a commit in review, a type still holds
    what it held before the commit, and the message says so.
    """
    print("\nreading the committed listing back in a fresh edit")
    edit_id = open_edit(client)
    problems, unchanged, deleted = [], [], False
    try:
        for local in local_sets:
            listed = list_images(client, edit_id, local.image_type)
            found = compare(local.image_type, listed, local)
            held = [(image.get("sha256") or "").lower() for image in listed]
            if found and held == before.get(local.image_type):
                unchanged.append(local.image_type)
            problems += found
            print(f"  {local.image_type}: {len(listed)} image(s)")
    finally:
        deleted = discard_edit(client, edit_id)
    if problems:
        message = ("the committed listing does not match the files:\n"
                   + "\n".join(f"  - {problem}" for problem in problems))
        if unchanged:
            message += (f"\n{', '.join(unchanged)} still hold the images from before the "
                        "commit; a new edit copies the app's deployed state, which may not "
                        "show a change in review. Look at the Play Console before any re-run.")
        if not deleted:
            message += f"\nedit {edit_id} was not deleted; delete it before another run"
        raise SystemExit(message)
    if not deleted:
        raise SystemExit(f"edit {edit_id} was not deleted; delete it before another run")
    print("committed listing matches the files: count, order and sha256")
    if not_sent:
        print("committed but NOT sent for review: send it for review from the Play "
              "Console; the store shows it only after that review.")
    else:
        print("the change is in Play's review, and the store shows it only after that. "
              "Record it as in review until the Play Console shows it published.")


def run_remote(client, args, local_sets):
    edit_id = open_edit(client)
    committed = deleted = False
    before = {}
    try:
        try:
            check_languages(client, edit_id)
            changing, before = plan(client, edit_id, local_sets)
            if not (args.rehearse or args.write):
                print("\nREAD-ONLY: nothing deleted or uploaded.")
            elif not changing:
                print("\nevery type already holds its files; nothing to upload")
            else:
                for local in changing:
                    replace(client, edit_id, local)
                client.api("POST", f"/edits/{edit_id}:validate")
                print("\nPlay validated the edit")
                if args.write:
                    commit(client, edit_id, args.changes_not_sent_for_review)
                    committed = True
                else:
                    print("REHEARSAL: not committed.")
        finally:
            # A committed edit is spent. One whose commit went unanswered is
            # deleted all the same: open, it would linger; spent, the delete
            # fails and the message below says what that means.
            if not committed:
                deleted = discard_edit(client, edit_id)
    except NotSentForReview:
        raise SystemExit(not_sent_message(edit_id, deleted)) from None
    except CommitOutcomeUnknown as error:
        raise SystemExit(unknown_message(edit_id, error, deleted)) from None
    if not committed:
        if not deleted:
            raise SystemExit(f"edit {edit_id} was not deleted; delete it before another run")
        print("nothing changed on Play")
        return 0
    verify_committed(client, local_sets, before, args.changes_not_sent_for_review)
    return 0


def main(argv=None):
    args = parse_args(argv)
    set_dir = (os.path.abspath(args.set_dir) if args.set_dir
               else os.path.join(REPO, "build", "play", f"screenshots-{args.version}"))
    local_sets = read_local_set(set_dir)
    total = sum(len(local.images) for local in local_sets)
    print(f"local set: OK ({LANGUAGE}, {args.version}, {total} PNGs under {set_dir})")
    for local in local_sets:
        print(f"  {local.image_type}: {len(local.images)} from {local.subdirectory}/")
    if args.local_only:
        print("local-only: no network request made")
        return 0
    client = connect()
    try:
        return run_remote(client, args, local_sets)
    except PlayError as error:
        raise SystemExit(f"stopped: {error}") from None


if __name__ == "__main__":
    raise SystemExit(main())
