"""The guards in msstore/submit.py, each proved against an input built to trip it.

These started life as probe scripts run by hand while the write side was being
built, and the commit that added it said the guards were "mutation-proved".
A proof that lives in memory is not enforced; this file is where it lives now,
and ci.yml's `unittest discover -s msstore -p 'test_*.py'` runs it unchanged.

No store is contacted: every network-facing helper on the module is replaced
per test, and a test that reaches the real http() by mistake fails on name
resolution rather than touching anything.
"""

from __future__ import annotations

import contextlib
import copy
import importlib.util
import io
import json
import os
import re
import struct
import tempfile
import unittest
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)


def load_module():
    spec = importlib.util.spec_from_file_location("submit_under_test", os.path.join(HERE, "submit.py"))
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def ledger_version() -> str:
    text = open(os.path.join(REPO, "cmd", "bibletext", "FyneApp.toml")).read()
    return re.search(r'Version\s*=\s*"([0-9.]+)"', text).group(1) + ".0"


def pe_bytes(machine: int) -> bytes:
    """The smallest thing the PE probe accepts: e_lfanew at 0x3C, "PE\\0\\0",
    then the COFF machine word."""
    h = bytearray(1024)
    struct.pack_into("<I", h, 0x3C, 0x80)
    h[0x80:0x84] = b"PE\0\0"
    struct.pack_into("<H", h, 0x84, machine)
    return bytes(h)


MACHINE = {"x64": 0x8664, "arm64": 0xAA64}


def synthetic_msix(directory: str, arch: str, *, version: str | None = None, name: str | None = None,
                   identity: dict | None = None, publisher: str | None = None,
                   drop: str | None = None, machine: int | None = None) -> str:
    ident = identity or json.load(open(os.path.join(HERE, "identity.json")))
    version = version or ledger_version()
    name = name or f"BibleText-{version}-{arch}.msix"
    path = os.path.join(directory, name)
    manifest = (
        '<?xml version="1.0"?><Package>'
        f'<Identity Name="{ident["identityName"]}" Publisher="{publisher or ident["identityPublisher"]}" '
        f'Version="{version}" ProcessorArchitecture="{arch}" /></Package>'
    )
    with zipfile.ZipFile(path, "w") as z:
        z.writestr("AppxManifest.xml", manifest)
        z.writestr("BibleText.exe", pe_bytes(MACHINE[arch] if machine is None else machine))
        for dll in ("libEGL.dll", "libGLESv2.dll", "d3dcompiler_47.dll"):
            if dll != drop:
                z.writestr(dll, b"\0" * 16)
    return path


def clone_fixture(kept_version: str = "1.2.10.0") -> dict:
    """The shape POST /submissions returns: one published x64 package and a
    hand-entered listing worth protecting."""
    return {
        "id": "S", "status": "PendingCommit", "statusDetails": {"errors": [], "warnings": []},
        "fileUploadUrl": "https://x.blob.core.windows.net/ingestion/1?sv=2019-12-12&sig=SECRET&sp=rwl",
        "targetPublishMode": "Immediate", "targetPublishDate": "1601-01-01T00:00:00Z",
        "applicationPackages": [{"fileName": f"BibleText-{kept_version}-x64.msix", "fileStatus": "Uploaded",
                                 "version": kept_version, "architecture": "x64", "id": "P1"}],
        "listings": {"en-gb": {"baseListing": {
            "title": "BibleText", "description": "A clean reader.", "features": ["read", "share"],
            "keywords": ["bible"], "images": [{"fileStatus": "Uploaded", "id": "I1"},
                                              {"fileStatus": "Uploaded", "id": "I2"}]}}},
        "pricing": {"priceId": "Free"}, "notesForCertification": "runFullTrust: reads its own files",
        "packageDeliveryOptions": {"packageRollout": {"isPackageRollout": False}},
    }


class Stubbed(unittest.TestCase):
    """A module with every network-facing helper replaced and state in a tempdir."""

    def setUp(self):
        self.m = load_module()
        self.tmp = tempfile.mkdtemp()
        self.m.STATE_DIR = self.tmp
        self.m.STATE = os.path.join(self.tmp, "run-state.json")
        self.m.token = lambda: "t"
        self.m.time.sleep = lambda s: None
        self.calls: list = []
        self.m.http = self.fail_http

    def fail_http(self, *a, **k):
        self.fail(f"reached http() -- a real request was about to be made: {a[:2]}")

    def quiet(self, fn, *a):
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            return fn(*a), buf.getvalue()

    def refused(self, fn, *a) -> str:
        with self.assertRaises(SystemExit) as cm:
            self.quiet(fn, *a)
        return str(cm.exception)


class ReadPackages(Stubbed):
    def test_a_matching_pair_passes(self):
        synthetic_msix(self.tmp, "x64")
        synthetic_msix(self.tmp, "arm64")
        pkgs = self.m.read_packages(self.tmp)
        self.assertEqual(sorted(p["architecture"] for p in pkgs), ["arm64", "x64"])

    def test_one_architecture_is_refused(self):
        synthetic_msix(self.tmp, "x64")
        self.assertIn("exactly one x64 and one arm64", self.refused(self.m.read_packages, self.tmp))

    def test_a_space_in_the_name_is_refused(self):
        synthetic_msix(self.tmp, "x64", name="BibleText 1.msix")
        synthetic_msix(self.tmp, "arm64")
        self.assertIn("plain ASCII", self.refused(self.m.read_packages, self.tmp))

    def test_missing_angle_is_refused(self):
        synthetic_msix(self.tmp, "x64", drop="libEGL.dll")
        synthetic_msix(self.tmp, "arm64")
        self.assertIn("libEGL.dll missing", self.refused(self.m.read_packages, self.tmp))

    def test_an_x64_binary_inside_an_arm64_package_is_refused(self):
        synthetic_msix(self.tmp, "x64")
        synthetic_msix(self.tmp, "arm64", machine=MACHINE["x64"])
        self.assertIn("does not match its label", self.refused(self.m.read_packages, self.tmp))

    def test_wrong_publisher_is_refused(self):
        synthetic_msix(self.tmp, "x64", publisher="CN=00000000-0000-0000-0000-000000000000")
        synthetic_msix(self.tmp, "arm64")
        self.assertIn("Publisher", self.refused(self.m.read_packages, self.tmp))

    def test_wrong_version_is_refused(self):
        synthetic_msix(self.tmp, "x64", version="0.0.1.0")
        synthetic_msix(self.tmp, "arm64", version="0.0.1.0")
        self.assertIn("FyneApp.toml", self.refused(self.m.read_packages, self.tmp))


class ClonePreserved(Stubbed):
    def body(self, clone):
        b = copy.deepcopy(clone)
        b.pop("fileUploadUrl", None)
        b["applicationPackages"] = list(clone["applicationPackages"]) + [
            {"fileName": "new.msix", "fileStatus": "PendingUpload"}]
        b["targetPublishMode"] = "Immediate"
        return b

    def test_the_intended_change_passes_and_file_upload_url_is_exempt(self):
        clone = clone_fixture()
        self.m.assert_clone_preserved(clone, self.body(clone))   # no raise

    def test_every_attack_on_the_listing_is_blocked(self):
        clone = clone_fixture()
        attacks = {
            "listing dropped": lambda b: b["listings"].pop("en-gb"),
            "description blanked": lambda b: b["listings"]["en-gb"]["baseListing"].__setitem__("description", ""),
            "screenshot removed": lambda b: b["listings"]["en-gb"]["baseListing"]["images"].pop(),
            "screenshot marked for deletion": lambda b: b["listings"]["en-gb"]["baseListing"]["images"][0].__setitem__("fileStatus", "PendingDelete"),
            "pricing rebuilt": lambda b: b.__setitem__("pricing", {"priceId": "Tier2"}),
            "cert notes replaced": lambda b: b.__setitem__("notesForCertification", "see notes"),
            "phantom en-us locale": lambda b: b["listings"].__setitem__("en-us", {"baseListing": {"images": []}}),
        }
        for label, attack in attacks.items():
            b = self.body(clone)
            attack(b)
            with self.subTest(label):
                with self.assertRaises(SystemExit):
                    self.m.assert_clone_preserved(clone, b)


class VerifyStaged(Stubbed):
    def fresh_from(self, clone, mutate=None):
        f = copy.deepcopy(clone)
        f["applicationPackages"] = list(clone["applicationPackages"]) + [
            {"fileName": "new.msix", "fileStatus": "PendingUpload", "version": None, "architecture": None}]
        if mutate:
            mutate(f)
        self.m.api = lambda *a, **k: f
        return f

    def test_an_identical_round_trip_passes(self):
        clone = clone_fixture()
        self.fresh_from(clone)
        self.quiet(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone)

    def test_a_server_side_listing_wipe_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["listings"]["en-gb"]["baseListing"].__setitem__("description", ""))
        self.assertIn("'listings' came back", self.refused(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone))

    def test_a_changed_cert_note_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f.__setitem__("notesForCertification", "different"))
        self.assertIn("notesForCertification", self.refused(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone))

    def test_a_duplicate_file_name_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["applicationPackages"].append(
            {"fileName": "new.msix", "fileStatus": "Uploaded"}))
        self.assertIn("appears 2 times", self.refused(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone))

    def test_losing_the_kept_package_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["applicationPackages"].__delitem__(0))
        # The kept package is also a non-mutable difference, but the specific
        # message must name the rollback, which is the thing that was lost.
        self.assertIn("rollback path is gone", self.refused(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone))

    def test_the_wrong_publish_mode_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f.__setitem__("targetPublishMode", "Manual"))
        self.assertIn("targetPublishMode", self.refused(self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone))


class Abort(Stubbed):
    def test_a_committed_submission_is_refused_without_touching_the_network(self):
        self.m.load_state = lambda: {"submissionId": "A", "committed": True}
        self.m.api = lambda *a, **k: self.fail("abort asked the server about a submission it already knew was committed")
        self.assertIn("not a draft", self.refused(self.m.cmd_abort))

    def test_a_submission_in_certification_is_refused_even_if_the_flag_is_stale(self):
        self.m.load_state = lambda: {"submissionId": "A", "committed": False}
        self.m.api = lambda method, path, tok, payload=None: (
            {"pendingApplicationSubmission": {"id": "A"}} if path.endswith(self.m.STORE_ID)
            else {"status": "Certification"})
        self.assertIn("'Certification'", self.refused(self.m.cmd_abort))

    def test_someone_elses_pending_submission_is_refused(self):
        self.m.load_state = lambda: {"submissionId": "A", "committed": False}
        self.m.api = lambda method, path, tok, payload=None: {"pendingApplicationSubmission": {"id": "B"}}
        self.assertIn("someone else's work", self.refused(self.m.cmd_abort))

    def test_a_pending_commit_draft_is_deleted(self):
        self.m.load_state = lambda: {"submissionId": "A", "committed": False}
        deleted = []
        self.m.http = lambda method, url, *a, **k: (deleted.append(method) or (204, {}, b""))
        self.m.api = lambda method, path, tok, payload=None: (
            ({"pendingApplicationSubmission": {"id": "A"}} if not deleted else {"lastPublishedApplicationSubmission": {"id": "P"}})
            if path.endswith(self.m.STORE_ID) else {"status": "PendingCommit"})
        self.quiet(self.m.cmd_abort)
        self.assertEqual(deleted, ["DELETE"])


class Poll(Stubbed):
    def poll(self, statuses):
        it = iter(statuses)
        self.m.load_state = lambda: {"submissionId": "A"}
        self.m.api = lambda *a, **k: {"status": next(it), "statusDetails": {}}
        rc, _ = self.quiet(self.m.cmd_poll)
        return rc

    def test_a_failed_commit_is_a_non_zero_exit(self):
        self.assertEqual(self.poll(["CommitStarted", "CommitFailed"]), 1)
        self.assertEqual(self.poll(["Canceled"]), 1)

    def test_a_taken_commit_is_zero(self):
        self.assertEqual(self.poll(["CommitStarted", "CommitStarted", "PreProcessing"]), 0)
        self.assertEqual(self.poll(["Certification"]), 0)


class Create(Stubbed):
    """create up to the PUT, with the clone's kept package at the ledger's
    own version so 'higher' and 'not higher' are both constructible."""

    def arrange(self, new_version: str, names=("x64", "arm64")):
        kept = ledger_version()
        clone = clone_fixture(kept_version=kept)
        self.issued = []
        def api(method, path, tok, payload=None):
            self.issued.append(method)
            if method == "POST":
                return copy.deepcopy(clone)
            if method == "GET" and "/submissions/" in path:
                f = copy.deepcopy(clone)
                f["applicationPackages"] += [{"fileName": p["fileName"], "fileStatus": "PendingUpload"} for p in self.pkgs]
                return f
            return {}
        self.m.api = api
        self.m.http = lambda method, url, *a, **k: (self.issued.append("BLOB") or (201, {}, b""))
        self.pkgs = []
        for arch in names:
            p = synthetic_msix(self.tmp, arch, version=new_version)
            self.pkgs.append({"fileName": os.path.basename(p), "path": p, "architecture": arch,
                              "version": new_version, "bytes": os.path.getsize(p), "sha256": "d"})
        self.m.cmd_preflight = lambda d: self.pkgs

    def test_a_version_not_above_the_kept_one_never_reaches_the_put(self):
        self.arrange(ledger_version())
        # A distinct fileName, so the version guard and not the name guard is
        # what fires.
        for p in self.pkgs:
            p["fileName"] = "rebuilt-" + p["fileName"]
        self.assertIn("not above the kept", self.refused(self.m.cmd_create, self.tmp, "Immediate"))
        self.assertNotIn("PUT", self.issued)

    def test_a_reused_file_name_never_reaches_the_put(self):
        self.arrange(ledger_version())
        self.assertIn("already a package", self.refused(self.m.cmd_create, self.tmp, "Immediate"))
        self.assertNotIn("PUT", self.issued)

    def test_the_id_is_saved_before_the_response_is_judged(self):
        self.arrange("9.9.9.0")
        saved = []
        self.m.save_state = lambda **kw: saved.append(kw) or kw
        clone = clone_fixture()
        clone.pop("fileUploadUrl")
        self.m.api = lambda method, path, tok, payload=None: copy.deepcopy(clone)
        self.assertIn("no fileUploadUrl", self.refused(self.m.cmd_create, self.tmp, "Immediate"))
        self.assertEqual(saved[0]["submissionId"], "S", "the id must be on disk before the response is judged")


class CommitGate(Stubbed):
    def test_commit_refuses_an_unverified_submission(self):
        self.m.load_state = lambda: {"submissionId": "A", "committed": False}
        self.m.api = lambda *a, **k: self.fail("commit asked the server before checking it had been verified")
        self.assertIn("not been verified", self.refused(self.m.cmd_commit))

    def test_verify_refuses_bytes_that_differ_from_the_upload(self):
        self.m.load_state = lambda: {"submissionId": "A", "packages": [{"fileName": "p.msix", "sha256": "aaaa"}],
                                     "publishMode": "Immediate"}
        self.m.read_packages = lambda d: [{"fileName": "p.msix", "sha256": "bbbb"}]
        json.dump({}, open(os.path.join(self.tmp, "clone-A.json"), "w"))
        self.assertIn("not the file create uploaded", self.refused(self.m.main, ["submit.py", "verify", self.tmp]))


class Transport(Stubbed):
    def test_a_201_is_success(self):
        self.m.http = lambda *a, **k: (201, {}, b'{"id": "N"}')
        self.assertEqual(self.m.api("POST", "/x", "t"), {"id": "N"})

    def test_an_empty_204_is_success(self):
        self.m.http = lambda *a, **k: (204, {}, b"")
        self.assertEqual(self.m.api("DELETE", "/x", "t"), {})

    def test_the_sas_never_survives_redaction(self):
        sas = "https://x.blob.core.windows.net/ingestion/1?sv=2019-12-12&sig=SECRET&sp=rwl"
        for out in (self.m.redact(sas), self.m.redact_text(f'body: "{sas}" more')):
            self.assertNotIn("SECRET", out)
            self.assertNotIn("sig=", out)
            self.assertIn("<sas-redacted>", out)


if __name__ == "__main__":
    unittest.main()
