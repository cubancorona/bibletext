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
import unicodedata
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


# A What's New as a release would write it: two lines, typographic
# punctuation and a bullet, all of which the Store must be sent unchanged.
NOTES_TEXT = "Shared notes open where they were written.\n• Search keeps its place — and “quotes” too."
NOTES = {"en-gb": NOTES_TEXT}


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
            "keywords": ["bible"], "releaseNotes": "", "images": [{"fileStatus": "Uploaded", "id": "I1"},
                                                                  {"fileStatus": "Uploaded", "id": "I2"}]},
            "platformOverrides": {}}},
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
        # The release notes are read from a tempdir too, never from the
        # tracked msstore/metadata, so no test depends on the owner's copy.
        self.m.RELEASE_NOTES_DIR = os.path.join(self.tmp, "metadata")
        self.m.token = lambda: "t"
        self.m.time.sleep = lambda s: None
        self.calls: list = []
        self.m.http = self.fail_http

    def fail_http(self, *a, **k):
        self.fail(f"reached http() -- a real request was about to be made: {a[:2]}")

    def write_notes(self, text: str | bytes = NOTES_TEXT + "\n", version: str | None = None,
                    lang: str = "en-gb") -> str:
        version = version or self.m.desktop_version()
        path = os.path.join(self.m.RELEASE_NOTES_DIR, lang, f"whats-new-{version}.txt")
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as f:
            f.write(text if isinstance(text, bytes) else text.encode("utf-8"))
        return path

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


class ReleaseNotes(Stubbed):
    """read_release_notes: each refusal fires on the input built for it, and
    the controls beside them pass, so the refusals are about what they name."""

    def read(self):
        return self.m.read_release_notes(self.m.desktop_version())

    def test_the_file_for_this_version_is_read_as_written(self):
        self.write_notes(NOTES_TEXT + "\n")
        # Less the editor's final line break, and nothing else.
        self.assertEqual(self.read(), {"en-gb": NOTES_TEXT})
        # Nothing else includes the spaces at either end: what is sent is
        # what verify_staged holds the server to, so a trimmed reading here
        # would pass a server that trimmed the same way.
        self.write_notes("  Indented first line.\nA last line ending in a space. \n")
        self.assertEqual(self.read(), {"en-gb": "  Indented first line.\nA last line ending in a space. "})

    def test_a_missing_file_is_refused(self):
        # A file for another version is not this release's.
        self.write_notes(version="0.0.1")
        self.assertIn("is missing", self.refused(self.read))

    def test_an_empty_file_is_refused(self):
        self.write_notes(" \n\n")
        self.assertIn("is empty", self.refused(self.read))

    def test_the_limit_is_1500_counted_in_utf16_code_units(self):
        self.assertEqual(self.m.RELEASE_NOTES_LIMIT, 1500)
        book = "\U0001F4D6"   # outside the BMP: one character, two UTF-16 units
        for label, text, fits in (
            ("1500 letters", "a" * 1500, True),
            ("1501 letters", "a" * 1501, False),
            ("1498 letters and an emoji", "a" * 1498 + book, True),
            ("1499 letters and an emoji", "a" * 1499 + book, False),
        ):
            with self.subTest(label):
                self.write_notes(text)
                if fits:
                    self.assertEqual(self.read()["en-gb"], text)
                else:
                    self.assertIn("at most 1500", self.refused(self.read))

    def test_a_copy_of_another_releases_notes_is_refused(self):
        self.write_notes("Earlier notes.\n\n", version="1.0.0")
        self.write_notes("Earlier notes.")
        self.assertIn("the same text as", self.refused(self.read))
        # Control: another release's file that says something else is no bar.
        self.write_notes("These notes.")
        self.assertEqual(self.read(), {"en-gb": "These notes."})

    def test_characters_the_listing_must_not_carry_are_refused(self):
        for label, text in {
            "a tab": "One\tTwo",
            "a CRLF line end": "One\r\nTwo",
            "a byte-order mark": "\ufeffOne",
            "a zero-width joiner": "One\u200dTwo",
            "a private-use character": "One \ue000",
            "an unassigned code point": "One \u0378",
            "a noncharacter": "One \uffff",
            "a line separator": "One\u2028Two",
            "a drawn small capital": "The name as drawn, L\u1d0f\u0280\u1d05",
        }.items():
            with self.subTest(label):
                self.write_notes(text)
                self.assertIn("U+", self.refused(self.read))
        with self.subTest("not UTF-8"):
            self.write_notes(b"One \xff Two")
            self.assertIn("not UTF-8", self.refused(self.read))
        with self.subTest("an unassigned code point names the Unicode version it is judged by"):
            self.write_notes("One \u0378")
            self.assertIn(f"unassigned in Unicode {unicodedata.unidata_version}", self.refused(self.read))

    def test_visible_text_of_every_kind_passes(self):
        text = "Café — “quoted”, ‘single’, 3 × 4 ≥ 10, a\u00a0non-breaking space, \U0001F4D6, THE LORD\nsecond line"
        self.write_notes(text)
        self.assertEqual(self.read(), {"en-gb": text})

    def test_the_small_capitals_refused_are_the_ones_the_app_draws(self):
        with open(os.path.join(REPO, "small_caps_draw.go"), encoding="utf-8") as f:
            src = f.read()
        table = re.search(r"var smallCapitals = map\[rune\]rune\{(.*?)\n\}", src, re.S)
        self.assertIsNotNone(table, "small_caps_draw.go no longer has the smallCapitals table this reads")
        drawn = set(re.findall(r"'[a-z]':\s*'(.)'", table.group(1)))
        self.assertEqual(len(drawn), 25, "the table's shape changed; this parse no longer reads all of it")
        self.assertEqual(drawn, set(self.m.DRAWN_SMALL_CAPITALS))
        for ch in sorted(drawn):
            with self.subTest(f"U+{ord(ch):04X}"):
                self.assertIsNotNone(self.m.refused_character(f"The {ch}"))

    def test_every_listing_language_needs_its_own_file(self):
        self.m.EXPECTED_LISTINGS = {"en-gb", "fr-fr"}
        self.write_notes()
        message = self.refused(self.read)
        self.assertIn("fr-fr", message)
        self.assertIn("is missing", message)


class Preflight(Stubbed):
    """preflight reads the notes after the packages and before the account,
    so an unfit note is reported before anything is asked of the server."""

    def arrange(self):
        self.pkg_dir = os.path.join(self.tmp, "packages")
        os.makedirs(self.pkg_dir)
        synthetic_msix(self.pkg_dir, "x64")
        synthetic_msix(self.pkg_dir, "arm64")
        self.asked = []
        self.m.token = lambda: self.asked.append("token") or "t"
        self.m.api = lambda method, *a, **k: self.asked.append(method) or {"pendingApplicationSubmission": None}

    def test_an_unfit_note_stops_it_before_the_server_is_asked(self):
        self.arrange()
        self.assertIn("is missing", self.refused(self.m.cmd_preflight, self.pkg_dir))
        self.write_notes("One\tTwo")
        self.assertIn("U+0009", self.refused(self.m.cmd_preflight, self.pkg_dir))
        self.assertEqual(self.asked, [])

    def test_a_fit_note_is_measured_and_the_account_read(self):
        self.arrange()
        self.write_notes()
        _, out = self.quiet(self.m.cmd_preflight, self.pkg_dir)
        self.assertIn(f"{len(NOTES_TEXT)} of 1500 characters", out)
        self.assertEqual(self.asked, ["token", "GET"])


class ClonePreserved(Stubbed):
    def body(self, clone):
        b = copy.deepcopy(clone)
        b.pop("fileUploadUrl", None)
        b["applicationPackages"] = list(clone["applicationPackages"]) + [
            {"fileName": "new.msix", "fileStatus": "PendingUpload"}]
        b["targetPublishMode"] = "Immediate"
        self.m.apply_release_notes(b, NOTES)
        return b

    def test_the_intended_change_passes_and_file_upload_url_is_exempt(self):
        clone = clone_fixture()
        self.m.assert_clone_preserved(clone, self.body(clone), NOTES)   # no raise

    def test_apply_release_notes_sets_that_field_and_nothing_else(self):
        clone = clone_fixture()
        b = copy.deepcopy(clone)
        self.m.apply_release_notes(b, NOTES)
        self.assertEqual(b["listings"]["en-gb"]["baseListing"].pop("releaseNotes"), NOTES_TEXT)
        clone["listings"]["en-gb"]["baseListing"].pop("releaseNotes")
        self.assertEqual(b, clone)

    def test_every_attack_on_the_listing_is_blocked(self):
        clone = clone_fixture()
        base = lambda b: b["listings"]["en-gb"]["baseListing"]
        attacks = {
            "listing dropped": lambda b: b["listings"].pop("en-gb"),
            "description blanked": lambda b: base(b).__setitem__("description", ""),
            "title changed": lambda b: base(b).__setitem__("title", "BibleText Reader"),
            "a feature added": lambda b: base(b)["features"].append("new"),
            "screenshot removed": lambda b: base(b)["images"].pop(),
            "screenshot marked for deletion": lambda b: base(b)["images"][0].__setitem__("fileStatus", "PendingDelete"),
            "a listing key added": lambda b: base(b).__setitem__("devStudio", "someone"),
            "pricing rebuilt": lambda b: b.__setitem__("pricing", {"priceId": "Tier2"}),
            "cert notes replaced": lambda b: b.__setitem__("notesForCertification", "see notes"),
            "phantom en-us locale": lambda b: b["listings"].__setitem__("en-us", {"baseListing": {"images": []}}),
            # The one field that may change must change to the file's text.
            "release notes other than the file's": lambda b: base(b).__setitem__("releaseNotes", "Other."),
            "release notes left as the clone's": lambda b: base(b).__setitem__("releaseNotes", ""),
            "release notes dropped": lambda b: base(b).pop("releaseNotes"),
            # Exactly the file's text: a space is a difference, not a rounding.
            "release notes with a space before them": lambda b: base(b).__setitem__("releaseNotes", " " + NOTES_TEXT),
            "release notes with a space after them": lambda b: base(b).__setitem__("releaseNotes", NOTES_TEXT + " "),
            # And only in baseListing: a platform override is a base listing
            # too, and notes there are a change like any other, as are notes
            # set on the language's entry beside its baseListing.
            "release notes in a platform override": lambda b: b["listings"]["en-gb"]["platformOverrides"].__setitem__(
                "Windows81", {"releaseNotes": NOTES_TEXT}),
            "release notes beside baseListing": lambda b: b["listings"]["en-gb"].__setitem__(
                "releaseNotes", NOTES_TEXT),
        }
        for label, attack in attacks.items():
            b = self.body(clone)
            attack(b)
            with self.subTest(label):
                with self.assertRaises(SystemExit):
                    self.m.assert_clone_preserved(clone, b, NOTES)

    def test_notes_for_a_language_the_listing_lacks_are_refused(self):
        b = copy.deepcopy(clone_fixture())
        self.assertIn("a new locale", self.refused(self.m.apply_release_notes, b, {"en-gb": "x", "fr-fr": "y"}))

    def test_a_listing_with_no_note_read_for_it_is_refused(self):
        # A clone whose baseListing has no releaseNotes key at all, sent on
        # with no note read for its language: the field and the note are both
        # absent, and that must not pass for agreement.
        clone = clone_fixture()
        clone["listings"]["en-gb"]["baseListing"].pop("releaseNotes")
        b = copy.deepcopy(clone)
        b.pop("fileUploadUrl")
        self.assertIn("releaseNotes is not the text", self.refused(self.m.assert_clone_preserved, clone, b, {}))
        # Control: the same clone with the note written into it passes.
        self.m.apply_release_notes(b, NOTES)
        self.m.assert_clone_preserved(clone, b, NOTES)


class VerifyStaged(Stubbed):
    def fresh_from(self, clone, mutate=None):
        """What the server holds after a correct PUT: the clone, the new
        package pending, and the release's notes in the listing."""
        f = copy.deepcopy(clone)
        f["applicationPackages"] = list(clone["applicationPackages"]) + [
            {"fileName": "new.msix", "fileStatus": "PendingUpload", "version": None, "architecture": None}]
        f["listings"]["en-gb"]["baseListing"]["releaseNotes"] = NOTES_TEXT
        if mutate:
            mutate(f)
        self.m.api = lambda *a, **k: f
        return f

    def verify(self, clone):
        return self.m.verify_staged, "S", [{"fileName": "new.msix"}], "Immediate", clone, NOTES

    def test_an_identical_round_trip_passes(self):
        clone = clone_fixture()
        self.fresh_from(clone)
        self.quiet(*self.verify(clone))

    def test_a_server_side_listing_wipe_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["listings"]["en-gb"]["baseListing"].__setitem__("description", ""))
        self.assertIn("'listings' came back", self.refused(*self.verify(clone)))

    def test_release_notes_other_than_the_files_are_refused(self):
        clone = clone_fixture()
        for label, got in (("the clone's blank", ""), ("other text", "Other."), ("absent", None),
                           # Exactly: a difference in whitespace alone is still one.
                           ("a space after", NOTES_TEXT + " "), ("a space before", " " + NOTES_TEXT),
                           ("a line break after", NOTES_TEXT + "\n")):
            with self.subTest(label):
                def mutate(f, got=got):
                    base = f["listings"]["en-gb"]["baseListing"]
                    if got is None:
                        base.pop("releaseNotes")
                    else:
                        base["releaseNotes"] = got
                self.fresh_from(clone, mutate)
                self.assertIn("releaseNotes came back as", self.refused(*self.verify(clone)))

    def test_release_notes_with_rewritten_line_breaks_are_named_as_such(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["listings"]["en-gb"]["baseListing"].__setitem__(
            "releaseNotes", NOTES_TEXT.replace("\n", "\r\n")))
        self.assertIn("CRLF line breaks", self.refused(*self.verify(clone)))

    def test_any_other_listing_change_beside_the_notes_is_still_refused(self):
        clone = clone_fixture()
        for label, mutate in {
            "title": lambda f: f["listings"]["en-gb"]["baseListing"].__setitem__("title", "Other"),
            "keywords": lambda f: f["listings"]["en-gb"]["baseListing"]["keywords"].append("x"),
            "a listing key the clone lacked": lambda f: f["listings"]["en-gb"]["baseListing"].__setitem__("devStudio", "x"),
            "notes in a platform override": lambda f: f["listings"]["en-gb"]["platformOverrides"].__setitem__(
                "Windows81", {"releaseNotes": NOTES_TEXT}),
            "notes beside baseListing": lambda f: f["listings"]["en-gb"].__setitem__("releaseNotes", NOTES_TEXT),
            "a second locale": lambda f: f["listings"].__setitem__("en-us", {"baseListing": {"releaseNotes": NOTES_TEXT}}),
        }.items():
            with self.subTest(label):
                self.fresh_from(clone, mutate)
                self.assertIn("'listings' came back", self.refused(*self.verify(clone)))

    def test_a_changed_cert_note_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f.__setitem__("notesForCertification", "different"))
        self.assertIn("notesForCertification", self.refused(*self.verify(clone)))

    def test_a_duplicate_file_name_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["applicationPackages"].append(
            {"fileName": "new.msix", "fileStatus": "Uploaded"}))
        self.assertIn("appears 2 times", self.refused(*self.verify(clone)))

    def test_losing_the_kept_package_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["applicationPackages"].__delitem__(0))
        # The kept package is also a non-mutable difference, but the specific
        # message must name the rollback, which is the thing that was lost.
        self.assertIn("rollback path is gone", self.refused(*self.verify(clone)))

    def test_the_servers_own_bookkeeping_is_not_a_difference(self):
        # What the server did on 23 September 2026: cleared the label and set
        # the read-only pricing flag. Neither is a change to the listing.
        clone = clone_fixture()
        clone["friendlyName"] = "Submission 3"
        clone["pricing"]["isAdvancedPricingModel"] = True
        def rewrite(f):
            f["friendlyName"] = None
            f["pricing"]["isAdvancedPricingModel"] = False
        self.fresh_from(clone, rewrite)
        self.quiet(*self.verify(clone))

    def test_a_changed_price_is_still_refused(self):
        # The exemption is one sub-key, not all of pricing.
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f["pricing"].__setitem__("priceId", "Tier2"))
        self.assertIn("'pricing'", self.refused(*self.verify(clone)))

    def test_the_wrong_publish_mode_is_refused(self):
        clone = clone_fixture()
        self.fresh_from(clone, lambda f: f.__setitem__("targetPublishMode", "Manual"))
        self.assertIn("targetPublishMode", self.refused(*self.verify(clone)))


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

    def test_an_abort_leaves_no_submission_and_no_verification_recorded(self):
        # The state names no submission once its draft is deleted, so it must
        # not go on saying one was verified.
        self.m.save_state(submissionId="A", committed=False, verified=True)
        deleted = []
        self.m.http = lambda method, url, *a, **k: (deleted.append(method) or (204, {}, b""))
        self.m.api = lambda method, path, tok, payload=None: (
            ({"pendingApplicationSubmission": {"id": "A"}} if not deleted else {})
            if path.endswith(self.m.STORE_ID) else {"status": "PendingCommit"})
        self.quiet(self.m.cmd_abort)
        state = self.m.load_state()
        self.assertIsNone(state["submissionId"])
        self.assertIs(state["verified"], False)


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
        self.clone = clone = clone_fixture(kept_version=kept)
        self.issued = []
        self.put = None
        def api(method, path, tok, payload=None):
            self.issued.append(method)
            if method == "POST":
                return copy.deepcopy(clone)
            if method == "PUT":
                # Stored as sent, the way a PUT that works stores it; the
                # GET below then reads back exactly what create chose to send.
                self.put = copy.deepcopy(payload)
                return copy.deepcopy(payload)
            if method == "GET" and "/submissions/" in path:
                return copy.deepcopy(self.put)
            return {}
        self.m.api = api
        self.m.http = lambda method, url, *a, **k: (self.issued.append("BLOB") or (201, {}, b""))
        self.pkgs = []
        for arch in names:
            p = synthetic_msix(self.tmp, arch, version=new_version)
            self.pkgs.append({"fileName": os.path.basename(p), "path": p, "architecture": arch,
                              "version": new_version, "bytes": os.path.getsize(p), "sha256": "d"})
        self.m.cmd_preflight = lambda d: self.pkgs
        self.write_notes()

    def test_a_run_writes_the_packages_and_the_notes_and_nothing_else(self):
        self.arrange("9.9.9.0")
        self.quiet(self.m.cmd_create, self.tmp, "Immediate")
        self.assertEqual(self.issued, ["POST", "PUT", "BLOB", "GET"])
        sent = copy.deepcopy(self.clone)
        sent.pop("fileUploadUrl")

        def differ(a, b, path=()):
            if isinstance(a, dict) and isinstance(b, dict):
                for k in sorted(set(a) | set(b)):
                    yield from differ(a.get(k), b.get(k), path + (k,))
            elif a != b:
                yield path
        # targetPublishMode and targetPublishDate are set on every run, and
        # the fixture already carries the values set, so they do not show.
        self.assertEqual(sorted(differ(sent, self.put)),
                         [("applicationPackages",), ("listings", "en-gb", "baseListing", "releaseNotes")])
        self.assertEqual(self.put["listings"]["en-gb"]["baseListing"]["releaseNotes"], NOTES_TEXT)
        self.assertEqual(self.m.load_state()["releaseNotes"], {"en-gb": self.m.notes_digest(NOTES_TEXT)})

    def test_unfit_notes_never_reach_the_post(self):
        for label, arrange in {
            "missing": lambda: os.remove(self.write_notes()),
            "empty": lambda: self.write_notes("\n"),
            "over the limit": lambda: self.write_notes("a" * 1501),
            "another release's": lambda: self.write_notes(NOTES_TEXT, version="1.0.0"),
            "a refused character": lambda: self.write_notes("One\tTwo"),
        }.items():
            with self.subTest(label):
                self.m.RELEASE_NOTES_DIR = os.path.join(tempfile.mkdtemp(), "metadata")
                self.arrange("9.9.9.0")
                arrange()
                self.refused(self.m.cmd_create, self.tmp, "Immediate")
                self.assertEqual(self.issued, [], "the server was asked to create a submission "
                                                  "the release notes could not go with")

    def test_a_read_back_that_refuses_leaves_nothing_to_commit(self):
        # The state starts as the last release left it, committed and
        # verified. create's own read-back then finds the notes gone from the
        # server, and commit must not take that old verified for this one.
        for label, drop in (("the notes came back blank", True), ("control: they came back as sent", False)):
            with self.subTest(label):
                self.m.RELEASE_NOTES_DIR = os.path.join(tempfile.mkdtemp(), "metadata")
                self.m.STATE = os.path.join(tempfile.mkdtemp(), "run-state.json")
                self.m.save_state(submissionId="OLD", committed=True, verified=True)
                self.arrange("9.9.9.0")
                inner, reached = self.m.api, []

                def api(method, path, tok, payload=None, inner=inner, drop=drop, reached=reached):
                    reached.append((method, path.rsplit("/", 1)[-1]))
                    r = inner(method, path, tok, payload)
                    if drop and method == "GET" and "/submissions/" in path:
                        r["listings"]["en-gb"]["baseListing"]["releaseNotes"] = ""
                    return r
                self.m.api = api
                if drop:
                    self.assertIn("releaseNotes came back", self.refused(self.m.cmd_create, self.tmp, "Immediate"))
                    self.assertIn("not been verified", self.refused(self.m.cmd_commit))
                    self.assertNotIn(("POST", "commit"), reached)
                else:
                    self.quiet(self.m.cmd_create, self.tmp, "Immediate")
                    self.quiet(self.m.cmd_commit)
                    self.assertIn(("POST", "commit"), reached)

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

    def arrange_verify(self, sent_text: str | None):
        state = {"submissionId": "A", "packages": [{"fileName": "p.msix", "sha256": "aaaa"}],
                 "publishMode": "Immediate"}
        if sent_text is not None:
            state["releaseNotes"] = {"en-gb": self.m.notes_digest(sent_text)}
        self.m.load_state = lambda: state
        self.m.save_state = lambda **kw: kw
        self.m.read_packages = lambda d: [{"fileName": "p.msix", "sha256": "aaaa"}]
        json.dump({}, open(os.path.join(self.tmp, "clone-A.json"), "w"))
        self.checked = []
        self.m.verify_staged = lambda sid, pkgs, mode, clone, notes: self.checked.append(notes)
        self.write_notes()

    def test_verify_refuses_notes_edited_since_create(self):
        self.arrange_verify("What create sent, since edited.")
        self.assertIn("is not the text create sent", self.refused(self.m.main, ["submit.py", "verify", self.tmp]))
        self.assertEqual(self.checked, [])

    def test_verify_refuses_a_submission_create_recorded_no_notes_for(self):
        self.arrange_verify(None)
        self.assertIn("is not the text create sent", self.refused(self.m.main, ["submit.py", "verify", self.tmp]))

    def test_verify_holds_the_server_to_the_notes_create_sent(self):
        self.arrange_verify(NOTES_TEXT)
        self.assertEqual(self.m.main(["submit.py", "verify", self.tmp]), 0)
        self.assertEqual(self.checked, [NOTES])

    def test_commit_follows_the_latest_verify(self):
        # create passed and recorded verified; a verify run after it finds the
        # server changed since. commit must go by the later verdict.
        for label, passes in (("the verify refuses", False), ("control: it passes", True)):
            with self.subTest(label):
                self.m.STATE = os.path.join(tempfile.mkdtemp(), "run-state.json")
                self.m.save_state(submissionId="A", committed=False, verified=True, publishMode="Immediate",
                                  packages=[{"fileName": "p.msix", "sha256": "aaaa"}],
                                  releaseNotes={"en-gb": self.m.notes_digest(NOTES_TEXT)})
                self.m.read_packages = lambda d: [{"fileName": "p.msix", "sha256": "aaaa"}]
                json.dump({}, open(os.path.join(self.tmp, "clone-A.json"), "w"))
                self.write_notes()

                def verify_staged(*a, passes=passes):
                    if not passes:
                        raise SystemExit("REFUSING TO PROCEED:\n  - 'listings' came back from the server "
                                         "different from the clone")
                self.m.verify_staged = verify_staged
                reached = []
                self.m.api = lambda method, path, tok, payload=None, reached=reached: (
                    reached.append((method, path.rsplit("/", 1)[-1])) or {"status": "PendingCommit"})
                if passes:
                    self.assertEqual(self.m.main(["submit.py", "verify", self.tmp]), 0)
                    self.quiet(self.m.cmd_commit)
                    self.assertIn(("POST", "commit"), reached)
                else:
                    self.refused(self.m.main, ["submit.py", "verify", self.tmp])
                    self.assertIn("not been verified", self.refused(self.m.cmd_commit))
                    self.assertEqual(reached, [])


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
