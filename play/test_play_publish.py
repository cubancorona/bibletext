"""What scripts/play-publish.py does itself: the release notes an upload
carries, and what it says when Play ends an upload's edit.

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
BUILD = "987"
NOTES = "Opening line.\n• first bullet\n• second bullet"


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

    def __init__(self, module, tracks=None, fail=None, bundle_code=BUILD):
        self.m = module
        self.tracks = copy.deepcopy(tracks or {})
        self.before = copy.deepcopy(self.tracks)
        self.fail = dict(fail or {})
        self.bundle_code = bundle_code
        self.calls = []
        self.commits = 0
        self.opened = 0
        self.edits = {}

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
            return 200, {"versionCode": int(self.bundle_code), "sha1": "0" * 40}
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
            self.commits += 1
            self.tracks.update(self.edits.pop(edit_id))
            return 200, {"id": edit_id}
        del self.edits[edit_id]
        return 204, {}


class Harness(unittest.TestCase):
    def setUp(self):
        self.m = load_module()
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.dir = scratch.name
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
        return self.write("app.aab", b"bundle bytes", "wb")

    def uploaded_notes(self, play):
        puts = [payload for kind, _m, _p, payload in play.calls if kind == "put"]
        self.assertEqual(len(puts), 1)
        return puts[0]["releases"][0]["releaseNotes"]

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

    def test_the_upload_sequence_and_payload_are_unchanged(self):
        play = self.fake()
        notes = self.write("notes.txt", NOTES + "\n")
        self.run_script("--notes", notes, "upload", self.bundle(), "alpha")
        self.assertEqual(play.calls, [
            ("open", "POST", "/edits", {}),
            ("bundle", "POST", "/edits/edit-1/bundles?uploadType=media", None),
            ("put", "PUT", "/edits/edit-1/tracks/alpha", {"track": "alpha", "releases": [
                {"status": "completed", "versionCodes": [BUILD],
                 "releaseNotes": [{"language": "en-GB", "text": NOTES}]}]}),
            ("commit", "POST", "/edits/edit-1:commit", None)])
        self.assertEqual(play.commits, 1)

    def test_the_cap_counts_every_line_break(self):
        # 250 + 2 + 249: 501 characters with both line breaks of the blank
        # line counted, 500 if they were folded into one space.
        notes = self.write("notes.txt", "x" * 250 + "\n\n" + "y" * 249 + "\n")
        play = self.fake()
        message = self.refused("--notes", notes, "upload", self.bundle(), "alpha")
        self.assertIn("501 characters", message)
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

class EditEndedMidUpload(Harness):
    """Play's 'This Edit has been deleted' is explained, call()'s text kept."""

    def setUp(self):
        super().setUp()
        self.aab = self.write("app.aab", b"bundle bytes", "wb")

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


if __name__ == "__main__":
    unittest.main()
