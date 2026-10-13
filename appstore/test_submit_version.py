"""What appstore/submit-version.py holds the version to before the review
submission: its releaseType, read back immediately before the submission.

No store is contacted. The script reaches App Store Connect through one
seam, ``asc()``, and runs the metadata writers and the preflight through
``run()`` and ``unwritten_per_release()``; every test replaces the three on
the loaded module with stand-ins that record each call, the ledger read
with a fixed version, and the ASC environment with placeholders. ci.yml's
``unittest discover -s appstore -p 'test_*.py'`` runs this file unchanged.
"""

from __future__ import annotations

import contextlib
import importlib.util
import io
import os
import sys
import unittest
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
VERSION = "9.9.9"
VERSION_ID = "v-0001"
BUILD = 49
BUILD_ID = "6df65fa3-0000-4000-8000-000000000049"
SUBMISSION_ID = "sub-0001"
AFTER_APPROVAL = "AFTER_APPROVAL"


def load_module():
    spec = importlib.util.spec_from_file_location(
        "submit_version_under_test", os.path.join(HERE, "submit-version.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class FakeConnect:
    """App Store Connect behind the script's one seam, holding one version.

    ``release_type`` is what the version record answers; ``later`` replaces
    it from the second read of the version on, standing for a change made
    on the version's page after the script's first look. A review submission
    is recorded in ``submitted``; any request the stand-in has no answer for
    fails the test.
    """

    def __init__(self, module, *, release_type=AFTER_APPROVAL, later=None,
                 state="PREPARE_FOR_SUBMISSION"):
        self.m = module
        self.release_type = release_type
        self.later = later
        self.state = state
        self.calls = []
        self.version_reads = 0
        self.submitted = False

    def asc(self, method, path, body=None):
        self.calls.append((method, path, body))
        if method == "get" and path.startswith("/v1/builds?"):
            return {"data": [{"id": BUILD_ID, "attributes": {"version": str(BUILD),
                                                             "processingState": "VALID"}}]}
        if method == "get" and path.startswith(f"/v1/apps/{self.m.APP_ID}/appStoreVersions?"):
            self.version_reads += 1
            if self.version_reads > 1 and self.later is not None:
                self.release_type = self.later
            return {"data": [{"id": VERSION_ID, "attributes": {
                "versionString": VERSION, "appStoreState": self.state,
                "releaseType": self.release_type}}]}
        if method == "get" and path.startswith(f"/v1/appStoreVersions/{VERSION_ID}/build"):
            return {"data": {"id": BUILD_ID, "attributes": {"version": str(BUILD)}}}
        if method == "post" and path == "/v1/reviewSubmissions":
            return {"data": {"id": SUBMISSION_ID}}
        if method == "post" and path == "/v1/reviewSubmissionItems":
            return {"data": {"id": "item-0001"}}
        if method == "patch" and path == f"/v1/reviewSubmissions/{SUBMISSION_ID}":
            self.submitted = True
            self.state = "WAITING_FOR_REVIEW"
            return {"data": {"attributes": {"state": "WAITING_FOR_REVIEW"}}}
        raise AssertionError(f"request the stand-in has no answer for: {method} {path}")

    def submissions(self):
        return [(m, p) for m, p, _b in self.calls if p.startswith("/v1/reviewSubmission")]


class Harness(unittest.TestCase):
    def setUp(self):
        self.m = load_module()
        self.m.ledger_version = lambda platform: VERSION
        self.ran = []
        self.m.run = lambda label, argv: self.ran.append(label)
        self.m.unwritten_per_release = lambda platform: []
        env = mock.patch.dict(os.environ, {"ASC_ISSUER_ID": "set", "ASC_KEY_ID": "set",
                                           "ASC_KEY_PATH": "set"})
        env.start()
        self.addCleanup(env.stop)
        self.printed = ""

    def fake(self, **kwargs):
        store = FakeConnect(self.m, **kwargs)
        self.m.asc = store.asc
        return store

    def run_script(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), mock.patch.object(sys, "argv", ["submit-version.py", *args]):
            code = self.m.main()
        self.printed = out.getvalue()
        return code

    def refused(self, *args):
        out = io.StringIO()
        with contextlib.redirect_stdout(out), mock.patch.object(sys, "argv", ["submit-version.py", *args]):
            with self.assertRaises(SystemExit) as caught:
                self.m.main()
        self.printed = out.getvalue()
        return str(caught.exception.code)


class SubmitHoldsTheReleaseType(Harness):
    """--submit sends the version only while approval would publish it."""

    def submit(self, *extra):
        return ("--platform", "IOS", "--build", str(BUILD), "--delivery-uuid", BUILD_ID,
                "--write", "--confirm-version", VERSION, "--submit", *extra)

    def test_a_version_that_approval_would_not_publish_is_not_submitted(self):
        for release_type in ("MANUAL", "SCHEDULED"):
            with self.subTest(release_type=release_type):
                store = self.fake(release_type=release_type)
                message = self.refused(*self.submit())
                self.assertTrue(message.startswith("NOT SUBMITTED: "), message)
                self.assertIn(f"releaseType {release_type}, not {AFTER_APPROVAL}", message)
                self.assertIn("nothing was submitted", message)
                self.assertFalse(store.submitted)
                self.assertEqual(store.submissions(), [])
                # The writes before it ran; only the submission did not.
                self.assertEqual(self.ran, ["push-metadata", "push-review-notes"])
                self.ran.clear()

    def test_the_release_type_is_read_back_before_the_submission_not_remembered(self):
        # AFTER_APPROVAL at the first look, changed on the version's page
        # before the submission: the read-back is what stops it.
        store = self.fake(release_type=AFTER_APPROVAL, later="MANUAL")
        message = self.refused(*self.submit())
        self.assertIn("releaseType MANUAL", message)
        self.assertFalse(store.submitted)
        self.assertIn(f"releaseType {AFTER_APPROVAL}", self.printed, "the first look was not printed")

    def test_a_version_releasing_after_approval_is_submitted(self):
        # CONTROL: the read-back is the request immediately before the
        # submission, and it asks for the releaseType.
        store = self.fake()
        self.assertEqual(self.run_script(*self.submit()), 0)
        self.assertTrue(store.submitted)
        self.assertEqual(store.submissions(), [
            ("post", "/v1/reviewSubmissions"), ("post", "/v1/reviewSubmissionItems"),
            ("patch", f"/v1/reviewSubmissions/{SUBMISSION_ID}")])
        first = next(i for i, (m, p, _b) in enumerate(store.calls) if p == "/v1/reviewSubmissions")
        method, path, _body = store.calls[first - 1]
        self.assertEqual(method, "get")
        self.assertIn("appStoreVersions?", path)
        self.assertIn("releaseType", path)
        self.assertIn(f"is releaseType {AFTER_APPROVAL}: approval publishes it", self.printed)
        self.assertIn("submitted: submission sub-0001 is WAITING_FOR_REVIEW", self.printed)

    def test_every_look_at_the_version_asks_for_its_release_type(self):
        store = self.fake(release_type="MANUAL")
        self.assertEqual(self.run_script("--platform", "IOS", "--build", str(BUILD)), 0)
        reads = [p for m, p, _b in store.calls if m == "get" and "appStoreVersions?" in p]
        self.assertEqual(len(reads), 1)
        self.assertIn("fields[appStoreVersions]=versionString,appStoreState,releaseType", reads[0])
        self.assertIn("releaseType MANUAL", self.printed)
        self.assertIn("read-only: nothing changed", self.printed)
        self.assertFalse(store.submitted)

    def test_without_submit_the_release_type_stops_nothing(self):
        # Preparing a version is not submitting it; the refusal belongs to
        # the submission alone.
        store = self.fake(release_type="MANUAL")
        self.assertEqual(self.run_script(*self.submit()[:-1]), 0)
        self.assertIn("prepared; add --submit to send it for review", self.printed)
        self.assertFalse(store.submitted)
        self.assertEqual(self.ran, ["push-metadata", "push-review-notes"])

    def test_the_refusal_names_each_release_type_it_meets(self):
        for release_type, refused in ((AFTER_APPROVAL, False), ("MANUAL", True),
                                      ("SCHEDULED", True), (None, True)):
            with self.subTest(release_type=release_type):
                problem = self.m.release_type_refusal("MAC_OS", VERSION, release_type)
                if not refused:
                    self.assertIsNone(problem)
                    continue
                self.assertIn(f"version {VERSION} on MAC_OS is releaseType {release_type}", problem)
                self.assertIn(f"not {AFTER_APPROVAL}", problem)


if __name__ == "__main__":
    unittest.main()
