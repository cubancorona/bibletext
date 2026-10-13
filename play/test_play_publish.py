"""What scripts/play-publish.py does itself: the release notes an upload
carries and where it reads them, the bundle it holds to the ledger and to the
file before the commit, the read-back after a commit, what it says when Play
ends an upload's edit, an upload or a promotion that meets a review in
progress, and promote.

No store is contacted. The script reaches Play only through its own
access_token() and call(); every test replaces both on the loaded module.
The stand-in for call() keeps each track and each open edit in memory and
records every request in order, and urlopen is replaced so that a request
that reaches the network fails the test. ci.yml's ``unittest discover -s
play -p 'test_*.py'`` runs this file unchanged.
"""

from __future__ import annotations

# Imported before the script is loaded, so that loading it under stand-in
# cryptography modules adds nothing else to sys.modules for the patch to
# take away again.
import base64  # noqa: F401
import contextlib
import copy
import hashlib
import http.client
import importlib.util
import io
import json
import os
import re
import sys
import tempfile
import time  # noqa: F401
import types
import unittest
import urllib.error
import urllib.parse  # noqa: F401
import urllib.request
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
SCRIPT = os.path.join(REPO, "scripts", "play-publish.py")
TOKEN = "token-that-must-never-be-printed"
VERSION, BUILD = "9.8.7", "987"
NOTES = "Opening line.\n• first bullet\n• second bullet"
# The tracked notes the harness writes for VERSION, read when --notes is absent.
TRACKED_NOTES = "Tracked opening line.\n• tracked bullet"
BUNDLE = b"bundle bytes"
REVIEW = "?changesInReviewBehavior=ERROR_IF_IN_REVIEW"


def read_back_calls(edit_id, name):
    """The three requests a read-back after a commit makes."""
    return [("open", "POST", "/edits", {}),
            ("get", "GET", f"/edits/{edit_id}/tracks/{name}", None),
            ("delete", "DELETE", f"/edits/{edit_id}", None)]


def load_module():
    """scripts/play-publish.py, loaded under stand-ins for its cryptography.

    No test signs a token, and the python CI runs need not have
    cryptography installed, so the three names the script imports from it
    come from empty modules for the load.
    """
    stubs = {name: types.ModuleType(name) for name in (
        "cryptography", "cryptography.hazmat", "cryptography.hazmat.primitives",
        "cryptography.hazmat.primitives.asymmetric")}
    stubs["cryptography.hazmat.primitives"].hashes = types.ModuleType("hashes")
    stubs["cryptography.hazmat.primitives"].serialization = types.ModuleType("serialization")
    stubs["cryptography.hazmat.primitives.asymmetric"].padding = types.ModuleType("padding")
    spec = importlib.util.spec_from_file_location("play_publish_under_test", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    with mock.patch.dict(sys.modules, stubs):
        spec.loader.exec_module(module)
    return module


def release(code=BUILD, *, name=None, status="completed", notes=NOTES, codes=None):
    """A track release as Play returns one; named the way Play names it."""
    found = {"name": f"{code} ({VERSION})" if name is None else name,
             "versionCodes": list(codes) if codes is not None else [code],
             "status": status}
    if notes is not None:
        found["releaseNotes"] = [{"language": "en-GB", "text": notes}]
    return found


def track(name, *releases):
    return {"track": name, "releases": list(releases)}


LIVE = release("986", name="986 (9.8.6)", notes="Last time.")


def in_review_answer(url):
    """call()'s exit text for Play refusing a commit over a review in progress."""
    body = json.dumps({"error": {"code": 400, "message": "Changes are currently in review.",
                                 "status": "FAILED_PRECONDITION"}})
    return f"Play API POST {url} -> HTTP 400\n{body}"


def deleted_answer(method, url):
    """call()'s exit text for Play's answer on an edit it has ended."""
    body = json.dumps({"error": {"code": 400, "message": "This Edit has been deleted.",
                                 "status": "FAILED_PRECONDITION"}})
    return f"Play API {method} {url} -> HTTP 400\n{body}"


class FakePlay:
    """Play as the script sees it through call().

    ``tracks`` is the committed state; each open edit starts from it, and a
    PUT changes only the edit until a commit. ``fail`` maps a request kind
    to the text call() would exit with, or to an exception to raise.

    ``in_review`` is something else already in Play's review, a production
    release or a listing change. A commit asking for ERROR_IF_IN_REVIEW is
    then refused and its edit left open; any other commit is taken with
    Play's default, CANCEL_IN_REVIEW_AND_SUBMIT, which sends that review
    back to the start, and is recorded in ``restarted``.

    A bundle is answered with the versionCode ``bundle_code`` and the sha1
    of the bytes received, or ``bundle_sha1`` when given. A commit names
    each release it was given no name for the way Play does,
    "<versionCode> (<versionName>)", with ``bundle_version`` as the
    versionName; ``tamper``, when set, is called with the committed tracks
    after each commit, standing for a change made beside the script.
    """

    KINDS = (
        ("POST", r"/edits", "open"),
        ("POST", r"/edits/[^/:?]+/bundles\?uploadType=media", "bundle"),
        ("GET", r"/edits/[^/:?]+/tracks", "list"),
        ("GET", r"/edits/[^/:?]+/tracks/[^/:?]+", "get"),
        ("PUT", r"/edits/[^/:?]+/tracks/[^/:?]+", "put"),
        ("POST", r"/edits/[^/:?]+:validate", "validate"),
        ("POST", r"/edits/[^/:?]+:commit(\?.*)?", "commit"),
        ("DELETE", r"/edits/[^/:?]+", "delete"),
    )

    def __init__(self, module, tracks=None, fail=None, bundle_code=BUILD, bundle_version=VERSION,
                 bundle_sha1=None, in_review=False):
        self.m = module
        self.tracks = copy.deepcopy(tracks or {})
        self.before = copy.deepcopy(self.tracks)
        self.fail = dict(fail or {})
        self.bundle_code = bundle_code
        self.bundle_version = bundle_version
        self.bundle_sha1 = bundle_sha1
        self.calls = []
        self.commits = 0
        self.opened = 0
        self.edits = {}
        self.in_review = in_review
        self.restarted = []
        self.tamper = None

    def kinds(self):
        return [kind for kind, _method, _path, _payload in self.calls]

    def call(self, url, token, method="GET", payload=None, raw=None,
             content_type="application/json"):
        if token != TOKEN:
            raise AssertionError(f"request without the script's token: {method} {url}")
        for base in (self.m.UPLOAD, self.m.BASE):
            if url.startswith(base):
                path = url[len(base):]
                break
        else:
            raise AssertionError(f"request outside the app: {url}")
        kind = next((k for m, pattern, k in self.KINDS
                     if m == method and re.fullmatch(pattern, path)), None)
        if kind is None:
            raise AssertionError(f"request Play has no answer for: {method} {path}")
        self.calls.append((kind, method, path, copy.deepcopy(payload)))
        failure = self.fail.get(kind)
        if isinstance(failure, BaseException):
            raise failure
        if failure is not None:
            raise SystemExit(failure)
        if kind == "open":
            self.opened += 1
            edit_id = f"edit-{self.opened}"
            self.edits[edit_id] = {}
            return 200, {"id": edit_id}
        edit_id = re.match(r"/edits/([^/:?]+)", path).group(1)
        if edit_id not in self.edits:
            raise SystemExit(deleted_answer(method, url.split("?")[0]))
        if kind == "bundle":
            sha1 = self.bundle_sha1 or hashlib.sha1(raw or b"").hexdigest()
            return 200, {"versionCode": int(self.bundle_code), "sha1": sha1}
        if kind == "list":
            return 200, {"tracks": list(self.tracks.values())}
        if kind == "get":
            name = path.rsplit("/", 1)[1]
            held = self.edits[edit_id].get(name) or self.tracks.get(name) or {"track": name}
            return 200, copy.deepcopy(held)
        if kind == "put":
            self.edits[edit_id][path.rsplit("/", 1)[1]] = copy.deepcopy(payload)
            return 200, copy.deepcopy(payload)
        if kind == "validate":
            return 200, {"id": edit_id}
        if kind == "commit":
            if self.in_review:
                if "changesInReviewBehavior=ERROR_IF_IN_REVIEW" in path:
                    raise SystemExit(in_review_answer(url.split("?")[0]))
                self.restarted.append(edit_id)
            self.commits += 1
            committed = self.edits.pop(edit_id)
            for held in committed.values():
                for found in held.get("releases") or []:
                    if not found.get("name"):
                        code = (found.get("versionCodes") or ["?"])[0]
                        found["name"] = f"{code} ({self.bundle_version})"
            self.tracks.update(committed)
            if self.tamper is not None:
                self.tamper(self.tracks)
            return 200, {"id": edit_id}
        del self.edits[edit_id]
        return 204, {}


class Harness(unittest.TestCase):
    def setUp(self):
        self.m = load_module()
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.dir = scratch.name
        self.m.LEDGER = self.write("FyneApp.toml",
                                   f'[Details]\nID = "x"\nVersion = "{VERSION}"\nBuild = {BUILD}\n')
        # The tracked notes directory, holding the ledger's file.
        self.m.NOTES_DIR = self.dir
        self.tracked = self.write(f"whats-new-{VERSION}.txt", TRACKED_NOTES + "\n")
        self.tokens = 0

        def access_token():
            self.tokens += 1
            return TOKEN
        self.m.access_token = access_token
        network = mock.patch.object(urllib.request, "urlopen",
                                    side_effect=AssertionError("a request reached the network"))
        network.start()
        self.addCleanup(network.stop)
        self.printed = ""

    def write(self, name, text, mode="w"):
        path = os.path.join(self.dir, name)
        with open(path, mode, **({} if "b" in mode else {"encoding": "utf-8", "newline": ""})) as f:
            f.write(text)
        return path

    def fake(self, **kwargs):
        play = FakePlay(self.m, **kwargs)
        self.m.call = play.call
        return play

    def run_script(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = self.m.main(["play-publish.py", *args])
        self.printed = out.getvalue()
        self.assertNotIn(TOKEN, self.printed)
        return code

    def refused(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), self.assertRaises(SystemExit) as caught:
            self.m.main(["play-publish.py", *args])
        self.printed = out.getvalue()
        message = str(caught.exception.code)
        self.assertNotIn(TOKEN, message + self.printed)
        return message


class UploadNotes(Harness):
    """--notes reaches the track release with its line breaks."""

    def bundle(self):
        return self.write("app.aab", BUNDLE, "wb")

    def uploaded_notes(self, play):
        puts = [payload for kind, _m, _p, payload in play.calls if kind == "put"]
        self.assertEqual(len(puts), 1)
        return puts[0]["releases"][0]["releaseNotes"]

    def test_without_notes_the_tracked_file_for_the_ledger_version_is_sent(self):
        play = self.fake()
        self.assertEqual(self.run_script("upload", self.bundle(), "alpha"), 0)
        self.assertEqual(self.uploaded_notes(play), [{"language": "en-GB", "text": TRACKED_NOTES}])
        self.assertIn(f"release notes from {self.tracked}", self.printed)

    def test_notes_names_another_file_over_the_tracked_one(self):
        play = self.fake()
        notes = self.write("notes.txt", NOTES + "\n")
        self.assertEqual(self.run_script("--notes", notes, "upload", self.bundle(), "alpha"), 0)
        self.assertEqual(self.uploaded_notes(play), [{"language": "en-GB", "text": NOTES}])
        self.assertIn(f"release notes from {notes}", self.printed)

    def test_a_missing_tracked_file_is_refused_before_anything_is_sent(self):
        # CONTROL for the default: the same upload with the file goes up.
        play = self.fake()
        os.remove(self.tracked)
        message = self.refused("upload", self.bundle(), "alpha")
        self.assertIn(f"no release notes for {VERSION}", message)
        self.assertIn(f"whats-new-{VERSION}.txt is missing", message)
        self.assertIn("docs/PLAY_LISTING.md", message)
        self.assertIn("nothing was changed", message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0)
        self.write(f"whats-new-{VERSION}.txt", TRACKED_NOTES + "\n")
        self.assertEqual(self.run_script("upload", self.bundle(), "alpha"), 0)
        self.assertEqual(play.commits, 1)

    def test_the_tracked_file_is_under_the_repository(self):
        module = load_module()
        self.assertEqual(module.NOTES_DIR, os.path.join(REPO, "play", "metadata", "en-GB"))
        self.assertEqual(module.default_notes("1.2.3"),
                         os.path.join(REPO, "play", "metadata", "en-GB", "whats-new-1.2.3.txt"))
        self.assertEqual(module.shown(module.default_notes("1.2.3")),
                         os.path.join("play", "metadata", "en-GB", "whats-new-1.2.3.txt"))

    def test_a_notes_file_of_several_lines_keeps_its_line_breaks(self):
        for label, text in (
                ("trailing spaces and blank ends", "\n  \nOpening line.   \n• first bullet\t\n"
                                                    "• second bullet\n\n"),
                ("CRLF line ends", "Opening line.\r\n• first bullet\r\n• second bullet\r\n")):
            with self.subTest(label):
                play = self.fake()
                notes = self.write("notes.txt", text)
                self.assertEqual(self.run_script("--notes", notes, "upload", self.bundle(), "alpha"), 0)
                self.assertEqual(self.uploaded_notes(play), [{"language": "en-GB", "text": NOTES}])
                self.assertIn(f"notes {len(NOTES)} chars", self.printed)

    def test_the_upload_sequence_and_payload(self):
        # The commit asks Play to refuse rather than restart a review in
        # progress, as promote's does; the track is then read back in an
        # edit of its own.
        play = self.fake()
        notes = self.write("notes.txt", NOTES + "\n")
        self.run_script("--notes", notes, "upload", self.bundle(), "alpha")
        self.assertEqual(play.calls, [
            ("open", "POST", "/edits", {}),
            ("bundle", "POST", "/edits/edit-1/bundles?uploadType=media", None),
            ("put", "PUT", "/edits/edit-1/tracks/alpha", {"track": "alpha", "releases": [
                {"status": "completed", "versionCodes": [BUILD],
                 "releaseNotes": [{"language": "en-GB", "text": NOTES}]}]}),
            ("commit", "POST", "/edits/edit-1:commit" + REVIEW, None),
            *read_back_calls("edit-2", "alpha")])
        self.assertEqual(play.commits, 1)
        self.assertEqual(play.edits, {}, "the read-back left its edit open")

    def test_the_cap_counts_every_line_break(self):
        # 250 + 2 + 249: 501 characters with both line breaks of the blank
        # line counted, 500 if they were folded into one space.
        notes = self.write("notes.txt", "x" * 250 + "\n\n" + "y" * 249 + "\n")
        play = self.fake()
        message = self.refused("--notes", notes, "upload", self.bundle(), "alpha")
        self.assertIn("501 characters", message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0)

    def test_the_indent_of_every_line_after_the_first_is_kept(self):
        # Only the ends of the whole lose whitespace: the 1.2.18 copy did the
        # same, and an indented bullet stays indented on Play.
        play = self.fake()
        notes = self.write("notes.txt", "  Opening line.\n  • indented bullet\n\t• tabbed bullet\n")
        self.run_script("--notes", notes, "upload", self.bundle(), "alpha")
        self.assertEqual(self.uploaded_notes(play), [
            {"language": "en-GB", "text": "Opening line.\n  • indented bullet\n\t• tabbed bullet"}])

    def test_a_notes_file_that_cannot_be_read_is_refused_before_anything_is_sent(self):
        play = self.fake()
        message = self.refused("--notes", os.path.join(self.dir, "missing.txt"), "upload",
                               self.bundle(), "alpha")
        self.assertIn("--notes: cannot read", message)
        self.assertIn("nothing was changed", message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0)

    def test_notes_of_exactly_500_characters_with_line_breaks_go_up(self):
        text = "x" * 249 + "\n\n" + "y" * 249
        notes = self.write("notes.txt", text + "\n")
        play = self.fake()
        self.assertEqual(self.run_script("--notes", notes, "upload", self.bundle(), "alpha"), 0)
        self.assertEqual(self.uploaded_notes(play), [{"language": "en-GB", "text": text}])

    def test_an_empty_notes_file_is_refused(self):
        notes = self.write("notes.txt", " \n\n\t\n")
        self.fake()
        self.assertIn("is empty", self.refused("--notes", notes, "upload", self.bundle(), "alpha"))

    def test_upload_refuses_the_production_track(self):
        play = self.fake()
        message = self.refused("upload", self.bundle(), "production")
        self.assertIn("promote", message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0)

    def test_confirm_version_and_rollout_belong_to_promote(self):
        play = self.fake()
        for extra in (("--confirm-version", VERSION), ("--rollout", "0.5")):
            with self.subTest(extra[0]):
                message = self.refused(*extra, "upload", self.bundle(), "alpha")
                self.assertIn("belong to promote", message)
        self.assertEqual(play.calls, [])

    def test_a_mistyped_dry_run_never_uploads(self):
        # In the track's place, "--dryrun" would be taken for a track name;
        # only the unknown-option check stops it.
        for args in (("upload", self.bundle(), "--dryrun"), ("upload", self.bundle(), "--dry_run"),
                     ("--dryrun", "upload", self.bundle()), ("upload", self.bundle(), "-n")):
            with self.subTest(args=args):
                play = self.fake()
                message = self.refused(*args)
                self.assertIn("unknown option", message)
                self.assertEqual(play.calls, [])
                self.assertEqual(self.tokens, 0)

    def test_an_argument_after_the_track_is_refused(self):
        play = self.fake()
        message = self.refused("upload", self.bundle(), "alpha", "beta")
        self.assertIn("usage:", message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0)


class UploadHeldToTheTree(Harness):
    """The bundle Play reports is this tree's, or nothing is committed; and
    after the commit the track is read back."""

    def setUp(self):
        super().setUp()
        self.aab = self.write("app.aab", BUNDLE, "wb")

    def assert_refused_before_the_commit(self, play, message, *wanted):
        for text in wanted:
            self.assertIn(text, message)
        self.assertIn("the bundle is not on 'alpha'", message)
        self.assertIn("Nothing changed on Play; the edit is discarded.", message)
        self.assertEqual(play.kinds(), ["open", "bundle", "delete"])
        self.assertEqual(play.commits, 0)
        self.assertEqual(play.edits, {})
        self.assertEqual(play.tracks, play.before)

    def test_a_bundle_of_another_versioncode_is_refused_and_the_edit_discarded(self):
        play = self.fake(bundle_code="986")
        message = self.refused("upload", self.aab, "alpha")
        self.assert_refused_before_the_commit(
            play, message, "Play holds the bundle as versionCode 986",
            f"the mobile ledger is {VERSION}, Build {BUILD}")

    def test_a_bundle_whose_sha1_is_not_the_files_is_refused_and_the_edit_discarded(self):
        theirs = "f" * 40
        play = self.fake(bundle_sha1=theirs)
        message = self.refused("upload", self.aab, "alpha")
        self.assert_refused_before_the_commit(
            play, message, f"Play reports sha1 {theirs}",
            f"the file's is {hashlib.sha1(BUNDLE).hexdigest()}")

    def test_the_bundle_this_tree_built_goes_up(self):
        # CONTROL: the ledger's versionCode and the file's own sha1 pass,
        # with the sha1 compared whatever its case.
        for sha1 in (None, hashlib.sha1(BUNDLE).hexdigest().upper()):
            with self.subTest(sha1=sha1):
                play = self.fake(bundle_sha1=sha1)
                self.assertEqual(self.run_script("upload", self.aab, "alpha"), 0)
                self.assertEqual(play.commits, 1)
                self.assertIn(f"the ledger's Build {BUILD} and the file's sha1", self.printed)

    def test_the_track_is_read_back_after_the_commit_and_printed(self):
        play = self.fake()
        self.assertEqual(self.run_script("upload", self.aab, "alpha"), 0)
        self.assertEqual(play.calls[-3:], read_back_calls("edit-2", "alpha"))
        self.assertIn("committed edit edit-1\n  read back after the commit:\n"
                      f"  alpha: '{BUILD} ({VERSION})', versionCode {BUILD}, completed\n",
                      self.printed)
        self.assertEqual(play.edits, {})

    def test_a_read_back_that_disagrees_is_reported_after_the_commit(self):
        # The commit stands; the disagreement is said, not undone.
        def gone(tracks):
            tracks["alpha"]["releases"] = []

        def draft(tracks):
            tracks["alpha"]["releases"][0]["status"] = "draft"
        for label, kwargs, hook, wanted in (
                ("a versionName that is not the ledger's Version", {"bundle_version": "9.8.6"},
                 None, f"named '{BUILD} (9.8.6)', for 9.8.6, not {VERSION}"),
                ("the release gone", {}, gone, f"'alpha' read back without versionCode {BUILD}"),
                ("another status", {}, draft, f"versionCode {BUILD} draft, not completed")):
            with self.subTest(label):
                play = self.fake(**kwargs)
                play.tamper = hook
                message = self.refused("upload", self.aab, "alpha")
                self.assertTrue(message.startswith("The commit went through, but "), message)
                self.assertIn(wanted, message)
                self.assertIn("settle the track in the Play Console", message)
                self.assertEqual(play.commits, 1)
                self.assertIn("committed edit edit-1", self.printed)
                self.assertEqual(play.calls[-3:], read_back_calls("edit-2", "alpha"))

    def test_a_read_back_that_cannot_be_made_still_says_the_commit_went_through(self):
        # Nothing before the commit reads a track, so the failure is the
        # read-back's own.
        answer = f"Play API GET {self.m.BASE}/edits/edit-2/tracks/alpha -> HTTP 503\nbackend error"
        play = self.fake(fail={"get": answer})
        message = self.refused("upload", self.aab, "alpha")
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("The commit went through, and the read-back of 'alpha' did not", message)
        self.assertIn("scripts/play-publish.py tracks", message)
        self.assertEqual(play.commits, 1)
        self.assertEqual(play.kinds()[-3:], ["open", "get", "delete"], "the read-back's edit was left open")

    def test_a_dry_run_is_held_to_the_tree_too(self):
        play = self.fake(bundle_code="986")
        message = self.refused("--dry-run", "upload", self.aab, "alpha")
        self.assert_refused_before_the_commit(play, message, "versionCode 986")


class EditEndedMidUpload(Harness):
    """Play's 'This Edit has been deleted' is explained, call()'s text kept."""

    def setUp(self):
        super().setUp()
        self.aab = self.write("app.aab", BUNDLE, "wb")

    def test_an_edit_ended_during_the_bundle_post_is_explained(self):
        answer = deleted_answer("POST", self.m.UPLOAD + "/edits/edit-1/bundles")
        play = self.fake(fail={"bundle": answer})
        message = self.refused("upload", self.aab, "alpha")
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("-> HTTP 400", message)
        self.assertIn("Play ended this edit while the upload was in flight", message)
        self.assertIn("scripts/release-status.py", message)
        self.assertIn("the versionCode is still unused", message)
        self.assertIn("Run the same command again once nothing else is touching Play", message)
        self.assertEqual(play.commits, 0)

    def test_an_edit_ended_before_the_commit_is_explained(self):
        answer = deleted_answer("POST", self.m.BASE + "/edits/edit-1:commit")
        self.fake(fail={"commit": answer})
        message = self.refused("upload", self.aab, "alpha")
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("Play ended this edit while the upload was in flight", message)

    def test_any_other_refusal_is_left_exactly_as_call_gave_it(self):
        answer = ("Play API POST " + self.m.UPLOAD + "/edits/edit-1/bundles -> HTTP 403\n"
                  '{"error": {"message": "The caller does not have permission"}}')
        self.fake(fail={"bundle": answer})
        self.assertEqual(self.refused("upload", self.aab, "alpha"), answer)

    def test_call_keeps_its_shared_http_message(self):
        # play/push-screenshots.py reads the status back out of this text.
        refusal = urllib.error.HTTPError("https://play.invalid/edits?uploadType=media", 400,
                                         "Bad Request", {}, io.BytesIO(b'{"error": "no"}'))
        with mock.patch.object(urllib.request, "urlopen", side_effect=refusal):
            with self.assertRaises(SystemExit) as caught:
                self.m.call("https://play.invalid/edits?uploadType=media", TOKEN, "POST", {})
        self.assertEqual(str(caught.exception.code),
                         'Play API POST https://play.invalid/edits -> HTTP 400\n{"error": "no"}')


class UploadWhileInReview(Harness):
    """An upload waits for a review in progress instead of restarting it."""

    def setUp(self):
        super().setUp()
        self.aab = self.write("app.aab", BUNDLE, "wb")
        self.notes = self.write("notes.txt", NOTES + "\n")

    def upload(self):
        return ("--notes", self.notes, "upload", self.aab, "alpha")

    def test_an_upload_never_restarts_a_review_in_progress(self):
        play = self.fake(tracks={"alpha": track("alpha", LIVE)}, in_review=True)
        message = self.refused(*self.upload())
        self.assertTrue(message.startswith(in_review_answer(self.m.BASE + "/edits/edit-1:commit")),
                        message)
        self.assertEqual(play.restarted, [], "the upload's commit sent the review back to the start")
        self.assertEqual(play.commits, 0)
        self.assertIn(f"Play refused the commit, so the bundle is not on 'alpha' and versionCode "
                      f"{BUILD} is still unused", message)
        self.assertIn("still in Play's review", message)
        self.assertIn("run the same command again", message)
        self.assertIn("Nothing changed on Play; the edit is discarded.", message)
        self.assertEqual(play.kinds(), ["open", "bundle", "put", "commit", "delete"])
        self.assertEqual(play.edits, {})
        self.assertEqual(play.tracks, play.before)

    def test_the_same_command_goes_up_once_the_review_clears(self):
        play = self.fake(tracks={"alpha": track("alpha", LIVE)}, in_review=True)
        self.refused(*self.upload())
        play.in_review = False
        self.assertEqual(self.run_script(*self.upload()), 0)
        self.assertEqual(play.commits, 1)
        self.assertEqual(play.tracks["alpha"], {"track": "alpha", "releases": [
            {"status": "completed", "versionCodes": [BUILD], "name": f"{BUILD} ({VERSION})",
             "releaseNotes": [{"language": "en-GB", "text": NOTES}]}]})
        self.assertIn("committed edit edit-2", self.printed)

    def test_any_refused_commit_is_explained_and_its_edit_discarded(self):
        answer = (f"Play API POST {self.m.BASE}/edits/edit-1:commit -> HTTP 403\n"
                  '{"error": {"message": "The caller does not have permission"}}')
        play = self.fake(fail={"commit": answer})
        message = self.refused(*self.upload())
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("Play refused the commit, so the bundle is not on 'alpha'", message)
        self.assertEqual(play.kinds()[-2:], ["commit", "delete"])
        self.assertEqual(play.tracks, play.before)

    def test_a_commit_play_did_not_answer_is_not_called_a_refusal(self):
        # A 5xx may follow a commit Play took, so nothing is said to be unused
        # and the edit is not touched.
        answer = f"Play API POST {self.m.BASE}/edits/edit-1:commit -> HTTP 503\nbackend error"
        play = self.fake(fail={"commit": answer})
        self.assertEqual(self.refused(*self.upload()), answer)
        self.assertEqual(play.kinds()[-1], "commit")

    def test_an_edit_ended_at_the_commit_is_still_explained_as_ended(self):
        answer = deleted_answer("POST", self.m.BASE + "/edits/edit-1:commit")
        play = self.fake(fail={"commit": answer})
        message = self.refused(*self.upload())
        self.assertIn("Play ended this edit while the upload was in flight", message)
        self.assertNotIn("Play refused the commit", message)
        self.assertNotIn("delete", play.kinds())


class Promote(Harness):
    """promote: the one release on a testing track to production."""

    def promote(self, *extra, source="alpha"):
        return ("promote", source, "production", "--confirm-version", VERSION, *extra)

    def play(self, held=None, production=None, **kwargs):
        return self.fake(tracks={
            "alpha": held if held is not None else track("alpha", release()),
            "production": production if production is not None else track("production", LIVE),
        }, **kwargs)

    def expected(self, **changes):
        found = {"name": f"{BUILD} ({VERSION})", "versionCodes": [BUILD],
                 "releaseNotes": [{"language": "en-GB", "text": NOTES}], "status": "completed"}
        found.update(changes)
        return {"track": "production", "releases": [{k: v for k, v in found.items() if v is not None}]}

    def assert_refused_and_discarded(self, play, message, *wanted):
        for text in wanted:
            self.assertIn(text, message)
        self.assertEqual(play.commits, 0)
        self.assertNotIn("put", play.kinds())
        self.assertEqual(play.kinds()[-1], "delete", "the edit opened for the read was left open")
        self.assertEqual(play.edits, {})
        self.assertEqual(play.tracks, play.before)
        self.assertIn("Nothing changed on Play; the edit is discarded.", message)

    def assert_refused_locally(self, play, message, *wanted):
        for text in wanted:
            self.assertIn(text, message)
        self.assertEqual(play.calls, [])
        self.assertEqual(self.tokens, 0, "the key was read before a local refusal")

    # A real run, and a dry run.

    def test_a_real_run_commits_once_with_the_exact_payload(self):
        for extra in ((), ("--rollout", "1")):
            with self.subTest(extra=extra):
                play = self.play()
                self.assertEqual(self.run_script(*self.promote(*extra)), 0)
                self.assertEqual(play.calls, [
                    ("open", "POST", "/edits", {}),
                    ("get", "GET", "/edits/edit-1/tracks/alpha", None),
                    ("get", "GET", "/edits/edit-1/tracks/production", None),
                    ("put", "PUT", "/edits/edit-1/tracks/production", self.expected()),
                    ("validate", "POST", "/edits/edit-1:validate", None),
                    ("commit", "POST", "/edits/edit-1:commit" + REVIEW, None),
                    *read_back_calls("edit-2", "production")])
                self.assertEqual(play.commits, 1)
                self.assertEqual(play.tracks["production"], self.expected())
                self.assertEqual(play.tracks["alpha"], play.before["alpha"])
                self.assertIn("committed edit edit-1", self.printed)
                self.assertIn("  read back after the commit:\n"
                              f"  production: '{BUILD} ({VERSION})', versionCode {BUILD}, completed\n",
                              self.printed)
                self.assertEqual(play.edits, {}, "the read-back left its edit open")

    def test_a_read_back_that_disagrees_is_reported_after_the_commit(self):
        # The commit stands; the disagreement is said, not undone.
        def replaced(tracks):
            tracks["production"]["releases"] = [copy.deepcopy(LIVE)]

        def halted(tracks):
            tracks["production"]["releases"][0]["status"] = "halted"
        for label, extra, hook, wanted in (
                ("the release gone", (), replaced, f"'production' read back without versionCode {BUILD}"),
                ("a rollout halted beside the commit", ("--rollout", "0.2"), halted,
                 f"versionCode {BUILD} halted, not inProgress")):
            with self.subTest(label):
                play = self.play()
                play.tamper = hook
                message = self.refused(*self.promote(*extra))
                self.assertTrue(message.startswith("The commit went through, but "), message)
                self.assertIn(wanted, message)
                self.assertIn("settle the track in the Play Console", message)
                self.assertEqual(play.commits, 1)
                self.assertEqual(play.calls[-3:], read_back_calls("edit-2", "production"))

    def test_a_read_back_that_cannot_be_made_still_says_the_commit_went_through(self):
        play = self.play()

        # The promotion's own reads come first; only the read-back's GET fails.
        def fail_the_read_back(tracks):
            play.fail["get"] = (f"Play API GET {self.m.BASE}/edits/edit-2/tracks/production "
                                "-> HTTP 503\nbackend error")
        play.tamper = fail_the_read_back
        message = self.refused(*self.promote())
        self.assertIn("The commit went through, and the read-back of 'production' did not", message)
        self.assertIn("scripts/play-publish.py tracks", message)
        self.assertEqual(play.commits, 1)
        self.assertEqual(play.kinds()[-3:], ["open", "get", "delete"])

    def test_a_staged_rollout_is_in_progress_with_its_fraction(self):
        play = self.play()
        self.run_script(*self.promote("--rollout", "0.2"))
        puts = [payload for kind, _m, _p, payload in play.calls if kind == "put"]
        self.assertEqual(puts, [self.expected(status="inProgress", userFraction=0.2)])
        self.assertEqual(play.commits, 1)
        self.assertIn("inProgress to 20% of users", self.printed)

    def test_a_dry_run_assigns_validates_and_deletes_the_edit_without_committing(self):
        play = self.play()
        self.assertEqual(self.run_script(*self.promote(), "--dry-run"), 0)
        self.assertEqual(play.kinds(), ["open", "get", "get", "put", "validate", "delete"])
        self.assertEqual(play.calls[3][3], self.expected())
        self.assertEqual(play.commits, 0)
        self.assertEqual(play.edits, {})
        self.assertEqual(play.tracks, play.before)
        self.assertIn("--dry-run: edit discarded, nothing changed on Play", self.printed)

    def test_the_dry_run_flag_counts_wherever_it_stands(self):
        play = self.play()
        self.run_script("--dry-run", *self.promote())
        self.assertEqual(play.commits, 0)
        self.assertEqual(play.kinds()[-1], "delete")

    def test_the_plan_shows_both_tracks_and_the_notes_line_by_line(self):
        self.play()
        self.run_script(*self.promote(), "--dry-run")
        self.assertIn(f"  alpha: '{BUILD} ({VERSION})', versionCode {BUILD}, completed", self.printed)
        self.assertIn("  production: '986 (9.8.6)', versionCode 986, completed", self.printed)
        for line in NOTES.split("\n"):
            self.assertIn(f"    | {line}\n", self.printed)

    def test_every_language_of_the_notes_is_carried_exactly(self):
        notes = [{"language": "en-GB", "text": "  Opening line.\n  • indented bullet \n"},
                 {"language": "en-US", "text": "Other words.\n\n"}]
        held = release()
        held["releaseNotes"] = copy.deepcopy(notes)
        play = self.play(held=track("alpha", held))
        self.assertEqual(self.run_script(*self.promote()), 0)
        self.assertEqual(play.tracks["production"], self.expected(releaseNotes=notes))

    def test_the_name_is_carried_only_when_the_release_has_one(self):
        for name, carried in (("Autumn release", "Autumn release"), ("", None)):
            with self.subTest(name=name):
                play = self.play(held=track("alpha", release(name=name)))
                self.run_script(*self.promote())
                puts = [payload for kind, _m, _p, payload in play.calls if kind == "put"]
                self.assertEqual(puts, [self.expected(name=carried)])
                self.assertEqual(play.commits, 1)

    # Refused before anything is read from Play.

    def test_the_version_must_be_confirmed_as_the_ledger_has_it(self):
        for extra in ((), ("--confirm-version", "9.8.6"), ("--confirm-version", "9.8"),
                      ("--confirm-version", "9.8.70")):
            with self.subTest(extra=extra):
                play = self.play()
                message = self.refused("promote", "alpha", "production", *extra)
                self.assert_refused_locally(play, message, f"--confirm-version {VERSION}")

    def test_a_bad_rollout_is_refused(self):
        for value in ("0", "-0.5", "1.01", "2", "nan", "inf", "half", "20%"):
            with self.subTest(rollout=value):
                play = self.play()
                message = self.refused(*self.promote("--rollout", value))
                self.assert_refused_locally(play, message, "--rollout must be a fraction")

    def test_promote_goes_from_a_testing_track_to_production_only(self):
        for args in (("promote", "alpha", "beta"), ("promote", "production", "production"),
                     ("promote", "alpha", "internal")):
            with self.subTest(args=args):
                play = self.play()
                message = self.refused(*args, "--confirm-version", VERSION)
                self.assert_refused_locally(play, message, "from a testing track to production")

    def test_promote_writes_no_notes_or_status_of_its_own(self):
        notes = self.write("notes.txt", "Other words.\n")
        missing = os.path.join(self.dir, "missing.txt")
        for extra in (("--notes", notes), ("--notes", missing), ("--status", "completed")):
            with self.subTest(extra=extra):
                play = self.play()
                message = self.refused(*self.promote(*extra))
                self.assert_refused_locally(play, message, "--notes and --status belong to upload")

    def test_a_mistyped_option_is_refused_before_anything_is_read(self):
        # Without the refusal, --dryrun would leave a real commit and
        # --rollout=0.2 a release to every reader.
        for extra in (("--dryrun",), ("--dry_run",), ("--rollout=0.2",), ("-n",)):
            with self.subTest(extra=extra):
                play = self.play()
                message = self.refused(*self.promote(*extra))
                self.assert_refused_locally(play, message, f"unknown option {extra[0]}",
                                            "nothing was changed")

    def test_promote_takes_exactly_two_tracks(self):
        for args in (("promote", "alpha", "production", "now"), ("promote", "alpha")):
            with self.subTest(args=args):
                play = self.play()
                message = self.refused(*args, "--confirm-version", VERSION)
                self.assert_refused_locally(play, message, "usage: play-publish.py promote")

    # Refused after reading the two tracks; the edit is discarded.

    def test_a_release_already_in_production_is_refused_with_its_state(self):
        for status, fraction in (("completed", None), ("inProgress", 0.2), ("halted", 0.2)):
            with self.subTest(status=status):
                live = release(status=status)
                if fraction is not None:
                    live["userFraction"] = fraction
                play = self.play(production=track("production", live, LIVE))
                message = self.refused(*self.promote())
                self.assert_refused_and_discarded(
                    play, message, f"production already carries versionCode {BUILD}", status)
                self.assertIn(f"  production: '{BUILD} ({VERSION})', versionCode {BUILD}, {status}",
                              self.printed)

    def test_another_versioncode_on_the_track_is_refused(self):
        play = self.play(held=track("alpha", release("986", name="986 (9.8.6)")))
        message = self.refused(*self.promote())
        self.assert_refused_and_discarded(play, message, "'alpha' holds versionCode 986",
                                          f"Build {BUILD}")

    def test_two_versioncodes_are_refused(self):
        play = self.play(held=track("alpha", release(codes=[BUILD, "988"])))
        message = self.refused(*self.promote())
        self.assert_refused_and_discarded(play, message, "holds 2 versionCodes")

    def test_a_track_without_exactly_one_release_is_refused(self):
        for held, count in ((track("alpha", release(), release("986")), 2),
                            ({"track": "alpha"}, 0)):
            with self.subTest(count=count):
                play = self.play(held=held)
                message = self.refused(*self.promote())
                self.assert_refused_and_discarded(play, message, f"'alpha' holds {count} releases")

    def test_a_release_named_for_another_version_is_refused(self):
        play = self.play(held=track("alpha", release(name=f"{BUILD} (9.8.6)")))
        message = self.refused(*self.promote())
        self.assert_refused_and_discarded(play, message, "for 9.8.6, not 9.8.7")

    def test_a_release_testers_do_not_yet_have_is_refused(self):
        for status in ("draft", "inProgress", "halted"):
            with self.subTest(status=status):
                play = self.play(held=track("alpha", release(status=status)))
                message = self.refused(*self.promote())
                self.assert_refused_and_discarded(play, message, f"is {status}, not completed")

    def test_a_release_without_notes_is_refused(self):
        for notes in (None, "  \n "):
            with self.subTest(notes=notes):
                play = self.play(held=track("alpha", release(notes=notes)))
                message = self.refused(*self.promote())
                self.assert_refused_and_discarded(play, message, "carries no release notes")

    def test_a_production_rollout_still_going_out_is_refused(self):
        staged = release("986", name="986 (9.8.6)", status="inProgress")
        staged["userFraction"] = 0.1
        play = self.play(production=track("production", release("985", name="985 (9.8.5)"), staged))
        message = self.refused(*self.promote())
        self.assert_refused_and_discarded(play, message, "production holds a release that is inProgress",
                                          "complete or halt its rollout in the Play Console first")

    def test_a_production_draft_is_refused(self):
        draft = release("986", name="986 (9.8.6)", status="draft")
        play = self.play(production=track("production", release("985", name="985 (9.8.5)"), draft))
        message = self.refused(*self.promote())
        self.assert_refused_and_discarded(play, message, "production holds a draft release",
                                          "roll it out or discard it in the Play Console first")

    def test_a_halted_production_rollout_gives_way_to_the_promotion(self):
        # A staged rollout halted over a defect is where a fix is promoted
        # from; the promotion writes production's releases whole.
        halted = release("986", name="986 (9.8.6)", status="halted")
        halted["userFraction"] = 0.2
        for extra, status in (((), {}), (("--rollout", "0.1"), {"status": "inProgress",
                                                                 "userFraction": 0.1})):
            with self.subTest(extra=extra):
                play = self.play(production=track("production",
                                                  release("985", name="985 (9.8.5)"), halted))
                self.assertEqual(self.run_script(*self.promote(*extra)), 0)
                self.assertEqual(play.commits, 1)
                self.assertEqual(play.tracks["production"], self.expected(**status))
                self.assertIn("in place of production's halted release '986 (9.8.6)', "
                              "versionCode 986, halted to 20% of users", self.printed)

    def test_production_ahead_of_the_ledger_is_refused(self):
        for status in ("completed", "halted"):
            with self.subTest(status=status):
                ahead = release("990", name="990 (9.9.0)", status=status)
                play = self.play(production=track("production", ahead))
                message = self.refused(*self.promote())
                self.assert_refused_and_discarded(
                    play, message, "production carries versionCode 990",
                    f"above the mobile ledger's Build {BUILD}")

    # The commit, and an edit Play ends.

    def test_a_refused_commit_says_production_is_unchanged_and_discards_the_edit(self):
        # A refusal over changes in review names this release's own alpha
        # upload as the likeliest cause; any other refusal does not.
        own = "The likeliest is this release's own 'alpha' upload"
        for code, body, in_review in (
                ("400", '{"error": {"code": 400, "message": "Changes are currently in review.", '
                        '"status": "FAILED_PRECONDITION"}}', True),
                ("400", '{"error": {"code": 400, "message": "You already have changes in review.", '
                        '"status": "FAILED_PRECONDITION"}}', True),
                ("400", '{"error": {"code": 400, "message": "Precondition check failed.", '
                        '"status": "FAILED_PRECONDITION", "details": [{"reason": '
                        '"CHANGES_ALREADY_IN_REVIEW"}]}}', True),
                ("403", '{"error": {"message": "The caller does not have permission"}}', False),
                ("400", '{"error": {"message": "Invalid request"}}', False)):
            with self.subTest(code=code, body=body):
                answer = f"Play API POST {self.m.BASE}/edits/edit-1:commit -> HTTP {code}\n{body}"
                play = self.play(fail={"commit": answer})
                message = self.refused(*self.promote())
                self.assertTrue(message.startswith(answer), message)
                self.assertIn("so production is unchanged", message)
                self.assertIn("Nothing changed on Play; the edit is discarded.", message)
                if in_review:
                    self.assertIn(own, message)
                    self.assertIn("Publishing overview shows the alpha release published", message)
                    self.assertIn("run the same command again", message)
                else:
                    self.assertNotIn(own, message)
                    self.assertNotIn("in review", message)
                self.assertEqual(play.kinds()[-2:], ["commit", "delete"])
                self.assertEqual(play.tracks, play.before)

    def test_an_edit_that_cannot_be_deleted_is_still_reported_unchanged(self):
        for failure in ("Play API DELETE https://x/edits/edit-1 -> HTTP 500\nbackend error",
                        ConnectionResetError("connection reset")):
            with self.subTest(failure=str(failure).splitlines()[0]):
                play = self.play(production=track("production", release(), LIVE),
                                 fail={"delete": failure})
                message = self.refused(*self.promote())
                self.assertIn(f"production already carries versionCode {BUILD}", message)
                self.assertIn("Nothing changed on Play. Edit edit-1 could not be discarded ("
                              + str(failure).splitlines()[0] + "); it ends when the next edit opens.",
                              message)
                self.assertEqual(play.kinds(), ["open", "get", "get", "delete"])
                self.assertEqual(play.commits, 0)

    def test_a_commit_whose_answer_is_lost_is_an_unknown_outcome(self):
        for failure in ("Play API POST https://x/edits/edit-1:commit -> HTTP 503\nbackend error",
                        TimeoutError("timed out"), KeyboardInterrupt(),
                        http.client.IncompleteRead(b"{\"id\""),
                        json.JSONDecodeError("Expecting value", "<html>", 0)):
            with self.subTest(failure=repr(failure).splitlines()[0]):
                play = self.play(fail={"commit": failure})
                try:
                    message = self.refused(*self.promote())
                except KeyboardInterrupt:
                    # Failed here, so an escaping interrupt cannot stop the run.
                    self.fail("an interrupt during the commit escaped without the advice")
                self.assertIn("the commit's outcome is unknown", message)
                self.assertIn("scripts/play-publish.py tracks", message)
                self.assertIn(f"refuses once production carries versionCode {BUILD}", message)
                self.assertEqual(play.kinds()[-1], "commit")

    def test_an_edit_ended_during_the_read_is_explained(self):
        answer = deleted_answer("GET", self.m.BASE + "/edits/edit-1/tracks/alpha")
        play = self.play(fail={"get": answer})
        message = self.refused(*self.promote())
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("Play ended this edit while the promotion was being prepared", message)
        self.assertNotIn("delete", play.kinds())

    def test_an_edit_ended_before_the_promotion_commit_is_explained(self):
        answer = deleted_answer("POST", self.m.BASE + "/edits/edit-1:commit")
        play = self.play(fail={"commit": answer})
        message = self.refused(*self.promote())
        self.assertTrue(message.startswith(answer), message)
        self.assertIn("Play ended this edit while the promotion was being committed", message)
        self.assertIn("production is unchanged", message)
        self.assertEqual(play.commits, 0)

    def test_a_promotion_never_restarts_a_review_in_progress(self):
        play = self.play(in_review=True)
        message = self.refused(*self.promote())
        self.assertIn("Play refused the commit over changes in review, so production is unchanged",
                      message)
        self.assertIn("The likeliest is this release's own 'alpha' upload", message)
        self.assertIn("the refusal changed nothing", message)
        self.assertEqual(play.restarted, [])
        self.assertEqual(play.commits, 0)
        self.assertEqual(play.tracks, play.before)
        # CONTROL: once the review clears, the same command goes up.
        play.in_review = False
        self.assertEqual(self.run_script(*self.promote()), 0)
        self.assertEqual(play.commits, 1)
        self.assertEqual(play.tracks["production"], self.expected())

    def test_the_promotion_names_the_track_it_was_given(self):
        play = self.fake(tracks={"beta": track("beta", release()),
                                 "production": track("production", LIVE)}, in_review=True)
        message = self.refused(*self.promote(source="beta"))
        self.assertIn("this release's own 'beta' upload", message)
        self.assertIn("shows the beta release published", message)
        self.assertEqual(play.commits, 0)

    def test_a_re_run_after_the_commit_refuses(self):
        play = self.play()
        self.run_script(*self.promote())
        message = self.refused(*self.promote())
        self.assertIn(f"production already carries versionCode {BUILD}", message)
        self.assertEqual(play.commits, 1)


class Ledger(unittest.TestCase):
    def test_the_ledger_is_the_mobile_one_and_reads(self):
        module = load_module()
        self.assertEqual(module.LEDGER, os.path.join(REPO, "cmd", "mobile", "FyneApp.toml"))
        version, build = module.ledger()
        self.assertRegex(version, r"^\d+(\.\d+)+$")
        self.assertRegex(build, r"^\d+$")


if __name__ == "__main__":
    unittest.main()
