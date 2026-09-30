"""The guards and the edit sequence of play/push-screenshots.py.

No store is contacted: the tool reaches Google Play through one seam,
``connect()``, and every test replaces it with a scripted stand-in that
records each call in order, keeps the live listing and each open edit in
memory, and fails the test outright on a write the mode does not allow.
ci.yml's ``unittest discover -s play -p 'test_*.py'`` runs this file
unchanged.
"""

from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import io
import os
import re
import struct
import tempfile
import types
import unittest
import zlib

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
VERSION = "9.8.7"
HANDLED = ("phoneScreenshots", "tenInchScreenshots")
SIZES = {"phone": (1080, 2160), "tablet10": (2560, 1600)}
TYPES = {"phone": "phoneScreenshots", "tablet10": "tenInchScreenshots"}


def load_module():
    spec = importlib.util.spec_from_file_location(
        "play_push_screenshots_under_test", os.path.join(HERE, "push-screenshots.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def png(width: int, height: int, *, colour_type: int = 2, bit_depth: int = 8,
        trns: bool = False, seed: int = 0, pad: int = 0) -> bytes:
    """A PNG whose header claims the given size and colour type.

    The tool reads the header and hashes the bytes; nothing decodes the image,
    so one pixel of data is enough. ``seed`` keeps every file's hash distinct;
    ``pad`` adds an ancillary chunk of that many bytes to reach a file size.
    """
    def chunk(kind: bytes, payload: bytes) -> bytes:
        return (struct.pack(">I", len(payload)) + kind + payload
                + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF))

    header = struct.pack(">IIBBBBB", width, height, bit_depth, colour_type, 0, 0, 0)
    parts = [b"\x89PNG\r\n\x1a\n", chunk(b"IHDR", header)]
    if trns:
        parts.append(chunk(b"tRNS", b"\0\0\0\0\0\0"))
    if pad:
        parts.append(chunk(b"teXt", b"x" * pad))
    parts += [chunk(b"IDAT", zlib.compress(b"\0\0\0\0" + bytes([seed % 256, seed // 256]))),
              chunk(b"IEND", b"")]
    return b"".join(parts)


def sha(data: bytes) -> str:
    """hashlib's own digest, so a wrong hash in the tool cannot be held to itself."""
    return hashlib.sha256(data).hexdigest()


class FakePlay:
    """A scripted Play Developer API behind the tool's one seam.

    ``live`` is what the store listing holds, per (language, type). Opening an
    edit copies it; reads and writes go to that copy; a commit copies the edit
    back to ``live`` and ends the edit, as Play's does. ``allow`` names the
    write kinds the test permits ("deleteall", "upload", "validate",
    "commit"); any other write fails the test. ``fail`` maps a call kind to
    the error message Play answers it with; ``tamper`` rewrites what a list
    of an edit returns.
    """

    def __init__(self, test: unittest.TestCase, *, languages=("en-GB",), live=None,
                 allow=(), fail=None, tamper=None, fail_delete_edit=False):
        self.test = test
        self.languages = list(languages)
        self.live = {key: [dict(image) for image in images]
                     for key, images in (live or {}).items()}
        self.allow = set(allow)
        self.fail = dict(fail or {})
        self.tamper = tamper
        self.fail_delete_edit = fail_delete_edit
        self.edits: dict = {}
        self.opened: list = []
        self.deleted: list = []
        self.committed: list = []
        self.calls: list = []
        self.uploads: list = []
        self.connected = 0
        self.serial = 0

    def connect(self):
        self.connected += 1
        return self

    def image(self, data: bytes) -> dict:
        self.serial += 1
        return {"id": f"img-{self.serial}", "url": f"https://play.invalid/{self.serial}",
                "sha1": hashlib.sha1(data).hexdigest(), "sha256": sha(data)}

    def refuse(self, kind: str):
        if kind in self.fail:
            raise self.test.m.PlayError(self.fail[kind])

    def permit(self, kind: str, path: str):
        if kind not in self.allow:
            self.test.fail(f"{kind} {path} reached Play in a mode that does not allow it")

    def edit(self, edit_id: str) -> dict:
        self.test.assertIn(edit_id, self.edits, f"{edit_id} is not an open edit")
        return self.edits[edit_id]

    # -- the seam ------------------------------------------------------------
    def api(self, method, path, payload=None):
        self.calls.append((method, path))
        if method == "POST" and path == "/edits":
            self.refuse("open")
            self.test.assertEqual(payload, {})
            edit_id = f"edit-{len(self.opened) + 1}"
            self.opened.append(edit_id)
            self.edits[edit_id] = {key: list(images) for key, images in self.live.items()}
            return {"id": edit_id, "expiryTimeSeconds": "0"}
        match = re.fullmatch(r"/edits/([^/:?]+)(.*)", path)
        self.test.assertIsNotNone(match, f"unexpected {method} {path}")
        edit_id, rest = match.groups()
        if method == "DELETE" and rest == "":
            self.edit(edit_id)
            if self.fail_delete_edit:
                raise self.test.m.PlayError("Play API DELETE -> HTTP 500\nbackend error")
            del self.edits[edit_id]
            self.deleted.append(edit_id)
            return {}
        edit = self.edit(edit_id)
        if method == "GET" and rest == "/listings":
            self.refuse("listings")
            return {"kind": "androidpublisher#listingsListResponse",
                    "listings": [{"language": language, "title": "BibleText"}
                                 for language in self.languages]}
        match = re.fullmatch(r"/listings/([A-Za-z-]+)/([A-Za-z]+)", rest)
        if match:
            key = match.groups()
            self.test.assertEqual(key[0], "en-GB", "only the en-GB listing is touched")
            self.test.assertIn(key[1], HANDLED, f"{key[1]} is never touched")
            if method == "GET":
                self.refuse("list")
                images = [dict(image) for image in edit.get(key, [])]
                if self.tamper and not self.committed:
                    self.tamper(key[1], images)
                return {"images": images} if images else {}
            if method == "DELETE":
                self.permit("deleteall", path)
                self.refuse("deleteall")
                gone = edit.get(key, [])
                edit[key] = []
                return {"deleted": gone} if gone else {}
        if method == "POST" and rest == ":validate":
            self.permit("validate", path)
            self.refuse("validate")
            return {"id": edit_id}
        match = re.fullmatch(r":commit\?(.*)", rest) or re.fullmatch(r":commit()", rest)
        if method == "POST" and match:
            self.permit("commit", path)
            self.commit_query = match.group(1)
            self.refuse("commit")
            self.live = {key: list(images) for key, images in edit.items()}
            self.committed.append(edit_id)
            del self.edits[edit_id]
            return {"id": edit_id}
        self.test.fail(f"unexpected {method} {path}")

    def upload(self, path, data):
        self.calls.append(("UPLOAD", path))
        self.permit("upload", path)
        match = re.fullmatch(r"/edits/([^/]+)/listings/(en-GB)/([A-Za-z]+)", path)
        self.test.assertIsNotNone(match, f"unexpected upload path {path}")
        edit_id, language, image_type = match.groups()
        self.test.assertIn(image_type, HANDLED, f"{image_type} is never touched")
        self.refuse("upload")
        image = self.image(data)
        self.edit(edit_id).setdefault((language, image_type), []).append(image)
        self.uploads.append((image_type, data))
        return {"image": dict(image)}

    # -- what the tests read -------------------------------------------------
    def writes(self):
        return [call for call in self.calls
                if call[0] == "UPLOAD" or (call[0] == "DELETE" and "/listings/" in call[1])
                or ":commit" in call[1] or ":validate" in call[1]]


class Harness(unittest.TestCase):
    """A module under test, a set directory in a tempdir and a fake Play."""

    def setUp(self):
        self.m = load_module()
        self.tmp = tempfile.mkdtemp()
        self.files: dict = {}
        self.seed = 0

    def write_type(self, sub: str, count: int = 3, *, names=None, size=None, **kwargs) -> list:
        directory = os.path.join(self.tmp, sub)
        os.makedirs(directory, exist_ok=True)
        width, height = size or SIZES[sub]
        names = names or [f"play-{sub}-{n:02d}.png" for n in range(1, count + 1)]
        for name in names:
            self.seed += 1
            data = png(width, height, seed=self.seed, **kwargs)
            with open(os.path.join(directory, name), "wb") as handle:
                handle.write(data)
            self.files[(sub, name)] = data
        return names

    def write_set(self, count: int = 3):
        self.write_type("phone", count)
        self.write_type("tablet10", count)

    def expected(self, sub: str) -> list:
        """The files of one type as sha256s, in sorted name order."""
        return [sha(self.files[key]) for key in sorted(k for k in self.files if k[0] == sub)]

    def old_live(self, count: int = 2) -> dict:
        return {("en-GB", "phoneScreenshots"): [
            {"id": f"old-{n}", "url": "https://play.invalid/old", "sha1": "0" * 40,
             "sha256": sha(f"old-{n}".encode())} for n in range(1, count + 1)]}

    def fake(self, **kwargs) -> FakePlay:
        play = FakePlay(self, **kwargs)
        self.m.connect = play.connect
        return play

    def run_tool(self, *extra: str):
        argv = ["--version", VERSION, "--set-dir", self.tmp, *extra]
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = self.m.main(argv)
        return code, out.getvalue()

    def refused(self, *extra: str, version: str = VERSION):
        out, err = io.StringIO(), io.StringIO()
        with self.assertRaises(SystemExit) as stop, contextlib.redirect_stdout(out), \
                contextlib.redirect_stderr(err):
            self.m.main(["--version", version, "--set-dir", self.tmp, *extra])
        code = stop.exception.code
        self.assertTrue(code not in (0, None), f"exit {code!r} is not a refusal")
        return f"{code}\n{out.getvalue()}\n{err.getvalue()}"

    def assert_every_edit_deleted(self, play: FakePlay):
        self.assertEqual(play.edits, {}, "an edit was left open")
        self.assertEqual(sorted(play.deleted + play.committed), sorted(play.opened))


class LocalChecks(Harness):
    def test_a_ready_set_passes_local_only_without_connecting(self):
        self.write_set()
        play = self.fake()
        code, out = self.run_tool("--local-only")
        self.assertEqual(code, 0)
        self.assertIn("local-only: no network request made", out)
        self.assertEqual(play.connected, 0)

    def test_an_empty_directory_is_refused(self):
        self.write_type("phone", 2)
        os.makedirs(os.path.join(self.tmp, "tablet10"))
        self.fake()
        message = self.refused("--local-only")
        self.assertIn("tenInchScreenshots", message)
        self.assertIn("holds no PNG", message)

    def test_a_missing_directory_is_refused(self):
        self.write_type("tablet10", 2)
        self.fake()
        message = self.refused("--local-only")
        self.assertIn("phoneScreenshots", message)
        self.assertIn("is missing", message)

    def test_nine_files_are_refused(self):
        self.write_type("phone", 9)
        self.write_type("tablet10", 1)
        self.fake()
        self.assertIn("holds 9 files; Play takes at most 8", self.refused("--local-only"))

    def test_eight_files_are_accepted(self):
        self.write_type("phone", 8)
        self.write_type("tablet10", 8)
        self.fake()
        code, out = self.run_tool("--local-only")
        self.assertEqual(code, 0)
        self.assertIn("phoneScreenshots: 8 from phone/", out)
        self.assertEqual(self.m.MAX_IMAGES, 8)

    def test_an_alpha_png_is_refused(self):
        self.write_type("phone", 2, colour_type=6)
        self.write_type("tablet10", 2)
        self.fake()
        self.assertIn("alpha channel", self.refused("--local-only"))

    def test_a_grey_alpha_png_is_refused(self):
        self.write_type("phone", 2)
        self.write_type("tablet10", 2, colour_type=4)
        self.fake()
        self.assertIn("alpha channel", self.refused("--local-only"))

    def test_a_transparency_chunk_counts_as_alpha(self):
        self.write_type("phone", 2, trns=True)
        self.write_type("tablet10", 2)
        self.fake()
        self.assertIn("alpha channel", self.refused("--local-only"))

    def test_a_palette_grey_or_16_bit_png_is_refused(self):
        for kwargs in ({"colour_type": 3}, {"colour_type": 0}, {"bit_depth": 16}):
            with self.subTest(**kwargs):
                self.tmp = tempfile.mkdtemp()
                self.write_type("phone", 1, **kwargs)
                self.write_type("tablet10", 1)
                self.fake()
                self.assertIn("24-bit RGB PNG", self.refused("--local-only"))

    def test_a_wrong_size_is_refused(self):
        self.write_type("phone", 2, size=(1080, 2400))
        self.write_type("tablet10", 2, size=(1600, 2560))
        self.fake()
        message = self.refused("--local-only")
        self.assertIn("1080x2400; this type takes exactly 1080x2160", message)
        self.assertIn("1600x2560; this type takes exactly 2560x1600", message)

    def test_a_png_named_file_that_is_not_a_png_is_refused(self):
        self.write_set(2)
        with open(os.path.join(self.tmp, "phone", "play-phone-03.png"), "wb") as handle:
            handle.write(b"not a png at all")
        self.fake()
        self.assertIn("play-phone-03.png is not a PNG", self.refused("--local-only"))

    def test_a_file_of_another_kind_is_refused_not_skipped(self):
        self.write_set(2)
        with open(os.path.join(self.tmp, "tablet10", "play-tablet10-03.jpg"), "wb") as handle:
            handle.write(b"\xff\xd8\xff\xe0 a jpeg header")
        self.fake()
        self.assertIn("play-tablet10-03.jpg is not a PNG", self.refused("--local-only"))

    def test_a_png_with_the_signature_but_no_ihdr_is_refused(self):
        self.write_set(1)
        with open(os.path.join(self.tmp, "phone", "play-phone-02.png"), "wb") as handle:
            handle.write(b"\x89PNG\r\n\x1a\n" + b"\0" * 40)
        self.fake()
        self.assertIn("play-phone-02.png is not a PNG", self.refused("--local-only"))

    def test_a_file_over_eight_megabytes_is_refused(self):
        self.assertEqual(self.m.MAX_BYTES, 8 * 1024 * 1024)
        self.write_type("phone", 1)
        self.write_type("phone", names=["play-phone-02.png"], pad=8 * 1024 * 1024)
        self.write_type("tablet10", 1)
        self.fake()
        self.assertIn("Play takes at most 8388608", self.refused("--local-only"))

    def test_two_identical_files_are_refused(self):
        self.write_set(2)
        data = self.files[("phone", "play-phone-01.png")]
        with open(os.path.join(self.tmp, "phone", "play-phone-03.png"), "wb") as handle:
            handle.write(data)
        self.fake()
        self.assertIn("play-phone-03.png is the same file as play-phone-01.png",
                      self.refused("--local-only"))

    def test_a_dotfile_is_ignored(self):
        self.write_set(2)
        with open(os.path.join(self.tmp, "phone", ".DS_Store"), "wb") as handle:
            handle.write(b"\0\0\0\1Bud1")
        self.fake()
        code, out = self.run_tool("--local-only")
        self.assertEqual(code, 0)
        self.assertIn("phoneScreenshots: 2 from phone/", out)

    def test_every_local_problem_stops_before_the_network(self):
        self.write_type("phone", 2, size=(1, 1))
        self.write_type("tablet10", 2)
        for mode in ((), ("--rehearse",), ("--write", "--confirm-version", VERSION)):
            with self.subTest(mode=mode):
                play = self.fake(allow=("deleteall", "upload", "validate", "commit"))
                self.refused(*mode)
                self.assertEqual(play.connected, 0)


class Arguments(Harness):
    def setUp(self):
        super().setUp()
        self.write_set()

    def test_the_version_is_required(self):
        play = self.fake()
        with self.assertRaises(SystemExit) as stop, contextlib.redirect_stderr(io.StringIO()):
            self.m.main(["--set-dir", self.tmp])
        self.assertNotIn(stop.exception.code, (0, None))
        self.assertEqual(play.connected, 0)

    def test_a_version_that_is_not_a_number_is_refused(self):
        play = self.fake()
        self.assertIn("not a version number", self.refused(version="../1.2"))
        self.assertEqual(play.connected, 0)

    def test_write_refuses_a_wrong_confirmation(self):
        play = self.fake(allow=("deleteall", "upload", "validate", "commit"))
        message = self.refused("--write", "--confirm-version", "9.8.6")
        self.assertIn(f"--write requires --confirm-version {VERSION}", message)
        self.assertEqual(play.connected, 0)

    def test_write_refuses_a_confirmation_that_is_only_a_prefix(self):
        play = self.fake(allow=("deleteall", "upload", "validate", "commit"))
        self.refused("--write", "--confirm-version", VERSION + ".0")
        self.assertEqual(play.connected, 0)

    def test_write_needs_a_confirmation_at_all(self):
        play = self.fake(allow=("deleteall", "upload", "validate", "commit"))
        self.refused("--write")
        self.assertEqual(play.connected, 0)

    def test_confirm_version_alone_is_refused(self):
        play = self.fake()
        self.assertIn("only with --write", self.refused("--confirm-version", VERSION))
        self.assertEqual(play.connected, 0)

    def test_not_sent_for_review_is_only_for_write(self):
        play = self.fake()
        self.assertIn("only with --write",
                      self.refused("--rehearse", "--changes-not-sent-for-review"))
        self.assertEqual(play.connected, 0)

    def test_the_modes_exclude_each_other(self):
        play = self.fake()
        self.refused("--rehearse", "--write", "--confirm-version", VERSION)
        self.refused("--local-only", "--rehearse")
        self.assertEqual(play.connected, 0)

    def test_the_default_set_is_build_play_screenshots_version(self):
        base = os.path.join(self.tmp, "repo")
        os.makedirs(os.path.join(base, "build", "play"))
        target = os.path.join(base, "build", "play", f"screenshots-{VERSION}")
        os.makedirs(target)
        for sub in ("phone", "tablet10"):
            os.rename(os.path.join(self.tmp, sub), os.path.join(target, sub))
        self.m.REPO = base
        self.fake()
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = self.m.main(["--version", VERSION, "--local-only"])
        self.assertEqual(code, 0)
        self.assertIn(f"PNGs under {target}", out.getvalue())


class ReadOnly(Harness):
    def test_read_only_never_writes_and_deletes_its_edit(self):
        self.write_set()
        play = self.fake(live=self.old_live())
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertEqual(play.writes(), [])
        self.assertEqual(play.opened, ["edit-1"])
        self.assertEqual(play.committed, [])
        self.assert_every_edit_deleted(play)
        self.assertIn("READ-ONLY: nothing deleted or uploaded.", out)
        self.assertIn("deleted edit edit-1", out)
        self.assertIn("nothing changed on Play", out)

    def test_read_only_lists_languages_then_each_type(self):
        self.write_set()
        play = self.fake()
        self.run_tool()
        self.assertEqual(play.calls, [
            ("POST", "/edits"),
            ("GET", "/edits/edit-1/listings"),
            ("GET", "/edits/edit-1/listings/en-GB/phoneScreenshots"),
            ("GET", "/edits/edit-1/listings/en-GB/tenInchScreenshots"),
            ("DELETE", "/edits/edit-1"),
        ])

    def test_read_only_names_every_other_language(self):
        self.write_set()
        self.fake(languages=("de-DE", "en-GB", "en-US"))
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertIn("listing languages: de-DE, en-GB, en-US", out)
        self.assertIn("de-DE, en-US keep their own images; only en-GB is touched", out)

    def test_the_plan_shows_current_and_new_by_sha256_in_order(self):
        self.write_set()
        self.fake(live=self.old_live(2))
        _code, out = self.run_tool()
        phone = out[out.index("phoneScreenshots (en-GB)"):out.index("tenInchScreenshots (en-GB)")]
        self.assertIn("now 2 image(s):", phone)
        self.assertIn(f"1. sha256 {sha(b'old-1')[:16]}  id old-1", phone)
        self.assertIn(f"2. sha256 {sha(b'old-2')[:16]}  id old-2", phone)
        for number, digest in enumerate(self.expected("phone"), 1):
            self.assertIn(f"{number}. sha256 {digest[:16]}  play-phone-{number:02d}.png", phone)
        self.assertIn("would delete all 2 and upload 3", phone)
        tablet = out[out.index("tenInchScreenshots (en-GB)"):]
        self.assertIn("now 0 image(s):", tablet)
        self.assertIn("would delete all 0 and upload 3", tablet)

    def test_a_type_that_already_holds_the_files_is_left_alone(self):
        self.write_set()
        live = {("en-GB", "phoneScreenshots"): [
            {"id": f"cur-{n}", "sha256": digest}
            for n, digest in enumerate(self.expected("phone"), 1)]}
        self.fake(live=live)
        _code, out = self.run_tool()
        phone = out[out.index("phoneScreenshots (en-GB)"):out.index("tenInchScreenshots (en-GB)")]
        self.assertIn("already holds these files in this order; left alone", phone)

    def test_the_same_files_in_another_order_are_a_change(self):
        self.write_set()
        live = {("en-GB", "phoneScreenshots"): [
            {"id": f"cur-{n}", "sha256": digest}
            for n, digest in enumerate(reversed(self.expected("phone")), 1)]}
        self.fake(live=live)
        _code, out = self.run_tool()
        phone = out[out.index("phoneScreenshots (en-GB)"):out.index("tenInchScreenshots (en-GB)")]
        self.assertIn("would delete all 3 and upload 3", phone)

    def test_a_missing_en_gb_listing_is_refused_and_the_edit_deleted(self):
        self.write_set()
        play = self.fake(languages=("en-US",))
        message = self.refused()
        self.assertIn("no en-GB listing", message)
        self.assert_every_edit_deleted(play)

    def test_a_failure_after_the_edit_opens_deletes_the_edit(self):
        self.write_set()
        play = self.fake(fail={"list": "Play API GET -> HTTP 503\nbackend unavailable"})
        message = self.refused()
        self.assertIn("HTTP 503", message)
        self.assert_every_edit_deleted(play)

    def test_an_edit_that_cannot_be_deleted_is_a_non_zero_exit(self):
        self.write_set()
        play = self.fake(fail_delete_edit=True)
        message = self.refused()
        self.assertIn("could not delete edit edit-1", message)
        self.assertIn("edit edit-1 was not deleted", message)
        self.assertEqual(play.committed, [])

    def test_a_failed_open_leaves_nothing_to_delete(self):
        self.write_set()
        play = self.fake(fail={"open": "Play API POST -> HTTP 403\nno permission"})
        self.assertIn("HTTP 403", self.refused())
        self.assertEqual(play.opened, [])
        self.assertEqual(play.deleted, [])


class Rehearsal(Harness):
    ALLOW = ("deleteall", "upload", "validate")

    def test_the_sequence_never_commits_and_deletes_the_edit(self):
        self.write_set(2)
        play = self.fake(live=self.old_live(), allow=self.ALLOW)
        code, out = self.run_tool("--rehearse")
        self.assertEqual(code, 0)
        e = "/edits/edit-1"
        self.assertEqual(play.calls, [
            ("POST", "/edits"),
            ("GET", f"{e}/listings"),
            ("GET", f"{e}/listings/en-GB/phoneScreenshots"),
            ("GET", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("DELETE", f"{e}/listings/en-GB/phoneScreenshots"),
            ("GET", f"{e}/listings/en-GB/phoneScreenshots"),
            ("UPLOAD", f"{e}/listings/en-GB/phoneScreenshots"),
            ("UPLOAD", f"{e}/listings/en-GB/phoneScreenshots"),
            ("GET", f"{e}/listings/en-GB/phoneScreenshots"),
            ("DELETE", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("GET", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("UPLOAD", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("UPLOAD", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("GET", f"{e}/listings/en-GB/tenInchScreenshots"),
            ("POST", f"{e}:validate"),
            ("DELETE", e),
        ])
        self.assertEqual(play.committed, [])
        self.assert_every_edit_deleted(play)
        self.assertEqual(play.live, self.old_live(), "the rehearsal changed the live listing")
        self.assertIn("Play validated the edit", out)
        self.assertIn("REHEARSAL: not committed.", out)
        self.assertIn("read back 2: count, order and sha256 agree", out)

    def test_uploads_go_in_sorted_file_name_order(self):
        # Written out of order, so directory order and creation order are not
        # the sorted order by accident.
        self.write_type("phone", names=["play-phone-03.png", "play-phone-01.png",
                                        "play-phone-02.png"])
        self.write_type("tablet10", names=["b.png", "a.png"])
        play = self.fake(allow=self.ALLOW)
        self.run_tool("--rehearse")
        self.assertEqual([sha(data) for kind, data in play.uploads if kind == "phoneScreenshots"],
                         [sha(self.files[("phone", f"play-phone-0{n}.png")]) for n in (1, 2, 3)])
        self.assertEqual([sha(data) for kind, data in play.uploads if kind == "tenInchScreenshots"],
                         [sha(self.files[("tablet10", n)]) for n in ("a.png", "b.png")])

    def test_each_upload_carries_the_files_exact_bytes(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW)
        self.run_tool("--rehearse")
        self.assertEqual(sorted(play.uploads, key=lambda u: sha(u[1])),
                         sorted(((TYPES[sub], data) for (sub, _), data in self.files.items()),
                                key=lambda u: sha(u[1])))

    def test_only_the_two_handled_types_on_en_gb_are_touched(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW)
        self.run_tool("--rehearse")
        touched = {call[1].split("/listings/", 1)[1] for call in play.calls
                   if "/listings/" in call[1]}
        self.assertEqual(touched, {"en-GB/phoneScreenshots", "en-GB/tenInchScreenshots"})

    def test_a_read_back_sha256_mismatch_aborts_and_deletes_the_edit(self):
        self.write_set(2)

        def tamper(image_type, images):
            if image_type == "phoneScreenshots" and len(images) == 2:
                images[1]["sha256"] = "f" * 64
        play = self.fake(allow=self.ALLOW, tamper=tamper)
        message = self.refused("--rehearse")
        self.assertIn("read-back mismatch", message)
        self.assertIn("phoneScreenshots position 2: sha256 " + "f" * 64, message)
        self.assertNotIn(("POST", "/edits/edit-1:validate"), play.calls)
        self.assert_every_edit_deleted(play)

    def test_a_read_back_out_of_order_aborts(self):
        self.write_set(2)

        def tamper(image_type, images):
            if image_type == "tenInchScreenshots" and len(images) == 2:
                images.reverse()
        play = self.fake(allow=self.ALLOW, tamper=tamper)
        message = self.refused("--rehearse")
        self.assertIn("tenInchScreenshots position 1", message)
        self.assert_every_edit_deleted(play)

    def test_a_read_back_missing_an_image_aborts(self):
        self.write_set(3)

        def tamper(image_type, images):
            if image_type == "phoneScreenshots" and len(images) == 3:
                del images[2]
        play = self.fake(allow=self.ALLOW, tamper=tamper)
        self.assertIn("phoneScreenshots holds 2 image(s), expected 3", self.refused("--rehearse"))
        self.assert_every_edit_deleted(play)

    def test_a_read_back_without_a_sha256_aborts(self):
        self.write_set(1)

        def tamper(image_type, images):
            for image in images:
                image.pop("sha256", None)
        play = self.fake(allow=self.ALLOW, tamper=tamper)
        self.assertIn("sha256 none", self.refused("--rehearse"))
        self.assert_every_edit_deleted(play)

    def test_a_type_not_empty_after_deleteall_aborts(self):
        self.write_set(1)
        play = self.fake(live=self.old_live(), allow=self.ALLOW)
        original = play.api

        def api(method, path, payload=None):
            if method == "DELETE" and path.endswith("/phoneScreenshots"):
                play.calls.append((method, path))
                return {}
            return original(method, path, payload)
        play.api = api
        self.assertIn("still holds 2 image(s) after deleteall", self.refused("--rehearse"))
        self.assertEqual(play.uploads, [])
        self.assert_every_edit_deleted(play)

    def test_an_upload_refused_midway_deletes_the_edit(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW, fail={"upload": "Play API POST -> HTTP 400\nbad image"})
        self.assertIn("bad image", self.refused("--rehearse"))
        self.assert_every_edit_deleted(play)

    def test_a_validation_failure_deletes_the_edit(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW, fail={"validate": "Play API POST -> HTTP 400\ninvalid"})
        self.assertIn("invalid", self.refused("--rehearse"))
        self.assertEqual(play.committed, [])
        self.assert_every_edit_deleted(play)

    def test_a_file_changed_after_the_check_is_refused_before_its_upload(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW)
        real = self.m.check_languages

        def check_then_change(client, edit_id):
            real(client, edit_id)
            with open(os.path.join(self.tmp, "phone", "play-phone-01.png"), "ab") as handle:
                handle.write(b"changed")
        self.m.check_languages = check_then_change
        self.assertIn("play-phone-01.png changed on disk", self.refused("--rehearse"))
        self.assertEqual(play.uploads, [])
        self.assert_every_edit_deleted(play)

    def test_nothing_is_uploaded_when_every_type_already_holds_its_files(self):
        self.write_set(2)
        live = {("en-GB", TYPES[sub]): [{"id": f"{sub}-{n}", "sha256": digest}
                                        for n, digest in enumerate(self.expected(sub), 1)]
                for sub in ("phone", "tablet10")}
        play = self.fake(live=live, allow=self.ALLOW)
        code, out = self.run_tool("--rehearse")
        self.assertEqual(code, 0)
        self.assertEqual(play.writes(), [])
        self.assertIn("every type already holds its files; nothing to upload", out)
        self.assert_every_edit_deleted(play)

    def test_a_keyboard_interrupt_mid_run_still_deletes_the_edit(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW)
        original = play.upload

        def upload(path, data):
            if play.uploads:
                raise KeyboardInterrupt
            return original(path, data)
        play.upload = upload
        with self.assertRaises(KeyboardInterrupt), contextlib.redirect_stdout(io.StringIO()):
            self.m.main(["--version", VERSION, "--set-dir", self.tmp, "--rehearse"])
        self.assert_every_edit_deleted(play)


class Writing(Harness):
    ALLOW = ("deleteall", "upload", "validate", "commit")

    def write_args(self, *extra):
        return ("--write", "--confirm-version", VERSION, *extra)

    def test_write_commits_then_reads_the_live_listing_in_a_fresh_edit(self):
        self.write_set(2)
        play = self.fake(live=self.old_live(), allow=self.ALLOW)
        code, out = self.run_tool(*self.write_args())
        self.assertEqual(code, 0)
        self.assertEqual(play.opened, ["edit-1", "edit-2"])
        self.assertEqual(play.committed, ["edit-1"])
        self.assertEqual(play.deleted, ["edit-2"])
        self.assert_every_edit_deleted(play)
        e2 = "/edits/edit-2"
        self.assertEqual(play.calls[-4:], [
            ("POST", "/edits"),
            ("GET", f"{e2}/listings/en-GB/phoneScreenshots"),
            ("GET", f"{e2}/listings/en-GB/tenInchScreenshots"),
            ("DELETE", e2),
        ])
        for sub in ("phone", "tablet10"):
            self.assertEqual([image["sha256"] for image in play.live[("en-GB", TYPES[sub])]],
                             self.expected(sub))
        self.assertIn("committed edit edit-1", out)
        self.assertIn("live listing matches the files: count, order and sha256", out)

    def test_validate_comes_before_the_commit(self):
        self.write_set(1)
        play = self.fake(allow=self.ALLOW)
        self.run_tool(*self.write_args())
        posts = [path for method, path in play.calls if method == "POST"]
        self.assertLess(posts.index("/edits/edit-1:validate"),
                        next(i for i, p in enumerate(posts) if ":commit" in p))

    def test_the_default_commit_refuses_rather_than_cancels_a_review(self):
        self.write_set(1)
        play = self.fake(allow=self.ALLOW)
        self.run_tool(*self.write_args())
        self.assertEqual(play.commit_query, "changesInReviewBehavior=ERROR_IF_IN_REVIEW")

    def test_the_not_sent_for_review_flag_is_the_commits_only_parameter(self):
        self.write_set(1)
        play = self.fake(allow=self.ALLOW)
        code, out = self.run_tool(*self.write_args("--changes-not-sent-for-review"))
        self.assertEqual(code, 0)
        self.assertEqual(play.commit_query, "changesNotSentForReview=true")
        self.assertIn("not sent for review", out)

    def test_a_commit_play_cannot_send_for_review_says_so_and_deletes_the_edit(self):
        self.write_set(1)
        refusal = ("Play API POST https://example.invalid/edits/edit-1:commit -> HTTP 400\n"
                   '{"error": {"code": 400, "message": "Changes cannot be sent for review '
                   "automatically. Please set the query parameter changesNotSentForReview "
                   'to true."}}')
        play = self.fake(live=self.old_live(), allow=self.ALLOW, fail={"commit": refusal})
        message = self.refused(*self.write_args())
        self.assertIn("cannot be sent for review automatically", message)
        self.assertIn("--changes-not-sent-for-review", message)
        self.assertEqual(play.committed, [])
        self.assert_every_edit_deleted(play)
        self.assertEqual(play.live, self.old_live())

    def test_any_other_refused_commit_deletes_the_edit(self):
        self.write_set(1)
        play = self.fake(allow=self.ALLOW,
                         fail={"commit": "Play API POST -> HTTP 400\nchanges are in review"})
        self.assertIn("changes are in review", self.refused(*self.write_args()))
        self.assert_every_edit_deleted(play)

    def test_a_read_back_mismatch_aborts_before_the_commit_and_deletes_the_edit(self):
        self.write_set(2)

        def tamper(image_type, images):
            if image_type == "tenInchScreenshots" and len(images) == 2:
                images[0]["sha256"] = "0" * 64
        play = self.fake(live=self.old_live(), allow=self.ALLOW, tamper=tamper)
        message = self.refused(*self.write_args())
        self.assertIn("read-back mismatch", message)
        self.assertFalse(any(":commit" in path for _method, path in play.calls))
        self.assertEqual(play.committed, [])
        self.assert_every_edit_deleted(play)
        self.assertEqual(play.live, self.old_live())

    def test_a_live_listing_that_differs_after_the_commit_is_a_non_zero_exit(self):
        self.write_set(2)
        play = self.fake(allow=self.ALLOW)
        original = play.api

        def api(method, path, payload=None):
            result = original(method, path, payload)
            if ":commit" in path:
                play.live[("en-GB", "phoneScreenshots")].reverse()
            return result
        play.api = api
        message = self.refused(*self.write_args())
        self.assertIn("the live listing does not match the files", message)
        self.assertEqual(play.committed, ["edit-1"])
        self.assert_every_edit_deleted(play)

    def test_the_committed_edit_is_not_deleted(self):
        self.write_set(1)
        play = self.fake(allow=self.ALLOW)
        self.run_tool(*self.write_args())
        self.assertNotIn(("DELETE", "/edits/edit-1"), play.calls)


class PlayClient(Harness):
    """The real Client over a stand-in for play-publish.py's call()."""

    TOKEN = "token-that-must-never-be-printed"

    def stand_in(self, error=None):
        seen = []

        def call(url, token, method="GET", payload=None, raw=None,
                 content_type="application/json"):
            seen.append((url, token, method, payload, raw, content_type))
            if isinstance(error, str):
                raise SystemExit(error)
            if error is not None:
                raise error
            return 200, {"ok": True}
        module = types.SimpleNamespace(
            BASE="https://api.invalid/applications/pkg",
            UPLOAD="https://upload.invalid/applications/pkg", call=call)
        return self.m.Client(module, self.TOKEN), seen

    def test_api_reaches_the_base_with_the_token(self):
        client, seen = self.stand_in()
        self.assertEqual(client.api("GET", "/edits/e/listings"), {"ok": True})
        self.assertEqual(seen, [("https://api.invalid/applications/pkg/edits/e/listings",
                                 self.TOKEN, "GET", None, None, "application/json")])

    def test_upload_sends_png_bytes_to_the_media_endpoint(self):
        client, seen = self.stand_in()
        client.upload("/edits/e/listings/en-GB/phoneScreenshots", b"\x89PNG")
        self.assertEqual(seen, [(
            "https://upload.invalid/applications/pkg/edits/e/listings/en-GB/phoneScreenshots"
            "?uploadType=media", self.TOKEN, "POST", None, b"\x89PNG", "image/png")])

    def test_an_http_refusal_becomes_a_play_error_without_the_token(self):
        client, _seen = self.stand_in("Play API POST https://x -> HTTP 400\nbad")
        with self.assertRaises(self.m.PlayError) as caught:
            client.api("POST", "/edits/e:commit")
        self.assertIn("HTTP 400", str(caught.exception))
        self.assertNotIn(self.TOKEN, str(caught.exception))

    def test_a_connection_failure_becomes_a_play_error_without_the_query(self):
        client, _seen = self.stand_in(OSError("timed out"))
        with self.assertRaises(self.m.PlayError) as caught:
            client.upload("/edits/e/listings/en-GB/phoneScreenshots", b"x")
        self.assertIn("timed out", str(caught.exception))
        self.assertNotIn("uploadType", str(caught.exception))

    def test_the_token_and_the_call_are_play_publishs_own(self):
        self.assertEqual(self.m.PLAY_PUBLISH, os.path.join(REPO, "scripts", "play-publish.py"))
        with open(self.m.PLAY_PUBLISH, encoding="utf-8") as handle:
            source = handle.read()
        self.assertIn("def access_token():", source)
        self.assertIn("def call(url, token, method=\"GET\", payload=None, raw=None, "
                      "content_type=\"application/json\"):", source)
        self.assertFalse(hasattr(self.m, "access_token"))
        with open(os.path.join(HERE, "push-screenshots.py"), encoding="utf-8") as handle:
            self.assertNotIn("private_key", handle.read())


if __name__ == "__main__":
    unittest.main()
