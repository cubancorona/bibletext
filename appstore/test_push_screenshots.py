"""The guards and the write sequence of appstore/push-screenshots.py.

No store is contacted: the tool reaches App Store Connect through one seam,
``connect()``, and every test replaces it with a scripted stand-in that
records each call in order, answers reads from a small in-memory model and
fails the test outright on any write made without --write. ci.yml's
``unittest discover -s appstore -p 'test_*.py'`` runs this file unchanged.
"""

from __future__ import annotations

import contextlib
import hashlib
import importlib.util
import io
import itertools
import os
import re
import struct
import tempfile
import unittest
import urllib.error
import urllib.request
import zlib
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)


def load_module():
    spec = importlib.util.spec_from_file_location(
        "push_screenshots_under_test", os.path.join(HERE, "push-screenshots.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def ledger_version(platform: str) -> str:
    ledger = {"IOS": "cmd/mobile/FyneApp.toml", "MAC_OS": "cmd/bibletext/FyneApp.toml"}[platform]
    text = open(os.path.join(REPO, ledger), encoding="utf-8").read()
    return re.search(r'Version\s*=\s*"([0-9.]+)"', text).group(1)


def png(width: int, height: int, *, alpha: bool = False, trns: bool = False,
        seed: int = 0) -> bytes:
    """A PNG whose header claims the given size and colour type.

    The tool reads the header and hashes the bytes; nothing decodes the image,
    so one pixel of data is enough. ``seed`` keeps every file's checksum
    distinct. ``trns`` adds the transparency chunk that also counts as alpha.
    """
    def chunk(kind: bytes, payload: bytes) -> bytes:
        return (struct.pack(">I", len(payload)) + kind + payload
                + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF))

    colour_type = 6 if alpha else 2
    header = struct.pack(">IIBBBBB", width, height, 8, colour_type, 0, 0, 0)
    pixel = b"\0" + (b"\0\0\0\0" if alpha else b"\0\0\0")
    parts = [b"\x89PNG\r\n\x1a\n", chunk(b"IHDR", header)]
    if trns:
        parts.append(chunk(b"tRNS", b"\0\0\0\0\0\0"))
    parts += [chunk(b"IDAT", zlib.compress(pixel + bytes([seed]))), chunk(b"IEND", b"")]
    return b"".join(parts)


SIZES = {"APP_IPHONE_67": (1320, 2868), "APP_IPAD_PRO_3GEN_129": (2064, 2752),
         "APP_DESKTOP": (2560, 1600)}
SUBDIRS = {"APP_IPHONE_67": "", "APP_IPAD_PRO_3GEN_129": "ipad13", "APP_DESKTOP": "mac"}
VERSION_ID = "v-0001"
LOCALIZATION_ID = "loc-0001"
# More polls of one image, and more listings of one set, than any scripted
# delivery or read-back needs: a loop that has lost its exit fails the test
# here rather than hanging the suite.
POLL_CAP = 25
LISTING_CAP = 25


class FakeStore:
    """A scripted App Store Connect behind the tool's one seam.

    Reads answer from ``self.sets``; writes mutate it the way the real service
    would (a reservation appears AWAITING_UPLOAD, a commit moves it along the
    scripted ``delivery`` states on each poll, the relationship PATCH reorders)
    and are refused by the test unless ``writable`` is set, which only a test
    that passed --write does.
    """

    def __init__(self, test: unittest.TestCase, *, version: str | None,
                 state: str = "PREPARE_FOR_SUBMISSION", locales=("en-GB",),
                 sets: dict | None = None, writable: bool = False,
                 delivery=("UPLOAD_COMPLETE", "COMPLETE"), failure=None,
                 version_attributes: dict | None = None):
        self.test = test
        self.version = version
        self.state = state
        # The version record carries ``state`` under both names App Store
        # Connect reports today unless a test gives the attributes itself.
        self.version_attributes = version_attributes
        self.locales = list(locales)
        self.sets = {display: {"id": f"set-{display}", "screenshots": list(records)}
                     for display, records in (sets or {}).items()}
        self.writable = writable
        self.delivery = list(delivery)
        self.failure = failure
        self.calls: list = []
        self.polls: dict = {}
        self.listings: dict = {}
        self.uploads: dict = {}
        self.tamper = None
        self.connected = 0
        self.reserved = 0

    # -- what a set holds, as the API reports it ---------------------------
    @staticmethod
    def record(screenshot_id: str, name: str, size: int, checksum: str | None,
               state: str = "COMPLETE", errors=None) -> dict:
        delivery = {"state": state, "errors": errors or [], "warnings": []}
        return {"type": "appScreenshots", "id": screenshot_id,
                "attributes": {"fileName": name, "fileSize": size,
                               "sourceFileChecksum": checksum,
                               "assetDeliveryState": delivery, "uploadOperations": None}}

    def find(self, screenshot_id: str):
        for entry in self.sets.values():
            for record in entry["screenshots"]:
                if record["id"] == screenshot_id:
                    return entry, record
        return None, None

    def connect(self):
        self.connected += 1
        return self

    # -- the seam -----------------------------------------------------------
    def api(self, method, path, body=None):
        self.calls.append((method, path, body))
        if method == "GET":
            return self.get(path)
        if not self.writable:
            self.test.fail(f"{method} {path} reached App Store Connect without --write")
        return self.write(method, path, body)

    def upload(self, operation, chunk):
        headers = {h["name"]: h["value"] for h in operation["requestHeaders"]}
        self.calls.append(("PUT", operation["url"], chunk, headers))
        if not self.writable:
            self.test.fail(f"PUT {operation['url']} without --write")
        self.uploads.setdefault(operation["url"].rsplit("/", 2)[1], []).append(chunk)
        return 200, b""

    def get(self, path):
        if path.startswith("/v1/apps/") and "appStoreVersions?" in path:
            if self.version is None:
                return 200, {"data": []}
            attributes = {"appVersionState": self.state, "appStoreState": self.state}
            if self.version_attributes is not None:
                attributes = dict(self.version_attributes)
            attributes["versionString"] = self.version
            return 200, {"data": [{"type": "appStoreVersions", "id": VERSION_ID,
                                   "attributes": attributes}]}
        if path.startswith(f"/v1/appStoreVersions/{VERSION_ID}/appStoreVersionLocalizations"):
            return 200, {"data": [{"type": "appStoreVersionLocalizations",
                                   "id": f"{LOCALIZATION_ID}-{n}" if n else LOCALIZATION_ID,
                                   "attributes": {"locale": locale}}
                                  for n, locale in enumerate(self.locales)]}
        if path.startswith(f"/v1/appStoreVersionLocalizations/{LOCALIZATION_ID}/appScreenshotSets"):
            return 200, {"data": [{"type": "appScreenshotSets", "id": entry["id"],
                                   "attributes": {"screenshotDisplayType": display}}
                                  for display, entry in self.sets.items()]}
        match = re.fullmatch(r"/v1/appScreenshotSets/(set-[A-Z0-9_]+)/appScreenshots\?limit=200", path)
        if match:
            for entry in self.sets.values():
                if entry["id"] == match.group(1):
                    n = self.listings.get(entry["id"], 0)
                    self.listings[entry["id"]] = n + 1
                    if n >= LISTING_CAP:
                        self.test.fail(f"{path} listed more than {LISTING_CAP} times")
                    records = [dict(r, attributes=dict(r["attributes"])) for r in entry["screenshots"]]
                    if self.tamper:
                        self.tamper(records)
                    return 200, {"data": records}
            return 404, {"errors": [{"title": "no such set"}]}
        match = re.fullmatch(r"/v1/appScreenshots/(new-\d+)", path)
        if match:
            _entry, record = self.find(match.group(1))
            self.test.assertIsNotNone(record, f"polled {path}, which was never reserved")
            self.test.assertTrue(record["attributes"].get("uploaded"),
                                 f"polled {path} before its commit")
            n = self.polls.get(record["id"], 0)
            self.polls[record["id"]] = n + 1
            if n >= POLL_CAP:
                self.test.fail(f"{path} polled more than {POLL_CAP} times")
            state = self.delivery[min(n, len(self.delivery) - 1)]
            record["attributes"]["assetDeliveryState"] = {
                "state": state, "errors": list(self.failure or []) if state == "FAILED" else [],
                "warnings": []}
            return 200, {"data": record}
        self.test.fail(f"unexpected GET {path}")

    def write(self, method, path, body):
        if method == "POST" and path == "/v1/appScreenshotSets":
            display = body["data"]["attributes"]["screenshotDisplayType"]
            self.test.assertEqual(body["data"]["type"], "appScreenshotSets")
            self.test.assertEqual(body["data"]["relationships"]["appStoreVersionLocalization"]["data"],
                                  {"type": "appStoreVersionLocalizations", "id": LOCALIZATION_ID})
            self.sets[display] = {"id": f"set-{display}", "screenshots": []}
            return 201, {"data": {"type": "appScreenshotSets", "id": f"set-{display}"}}
        if method == "POST" and path == "/v1/appScreenshots":
            attributes = body["data"]["attributes"]
            set_id = body["data"]["relationships"]["appScreenshotSet"]["data"]["id"]
            entry = next(e for e in self.sets.values() if e["id"] == set_id)
            new_id = f"new-{self.reserved}"
            self.reserved += 1
            size = attributes["fileSize"]
            half = size // 2
            operations = [
                {"method": "PUT", "url": f"https://upload.invalid/{new_id}/1",
                 "offset": 0, "length": half,
                 "requestHeaders": [{"name": "Content-Type", "value": "image/png"},
                                    {"name": "X-Part", "value": "1"}]},
                {"method": "PUT", "url": f"https://upload.invalid/{new_id}/2",
                 "offset": half, "length": size - half,
                 "requestHeaders": [{"name": "Content-Type", "value": "image/png"},
                                    {"name": "X-Part", "value": "2"}]},
            ]
            record = self.record(new_id, attributes["fileName"], size, None, "AWAITING_UPLOAD")
            record["attributes"]["uploadOperations"] = operations
            entry["screenshots"].append(record)
            return 201, {"data": record}
        if method == "PATCH" and path.startswith("/v1/appScreenshots/"):
            _entry, record = self.find(path.rsplit("/", 1)[1])
            self.test.assertIsNotNone(record)
            self.test.assertEqual(body["data"]["type"], "appScreenshots")
            self.test.assertEqual(body["data"]["id"], record["id"])
            record["attributes"]["uploaded"] = body["data"]["attributes"]["uploaded"]
            record["attributes"]["sourceFileChecksum"] = body["data"]["attributes"]["sourceFileChecksum"]
            return 200, {"data": record}
        match = re.fullmatch(r"/v1/appScreenshotSets/(set-[A-Z0-9_]+)/relationships/appScreenshots", path)
        if method == "PATCH" and match:
            entry = next(e for e in self.sets.values() if e["id"] == match.group(1))
            by_id = {r["id"]: r for r in entry["screenshots"]}
            self.test.assertEqual({item["type"] for item in body["data"]}, {"appScreenshots"},
                                  "each reorder item names its resource type")
            self.test.assertEqual(sorted(by_id), sorted(item["id"] for item in body["data"]),
                                  "the reorder must name exactly the set's screenshots")
            entry["screenshots"] = [by_id[item["id"]] for item in body["data"]]
            return 204, b""
        if method == "DELETE" and path.startswith("/v1/appScreenshots/"):
            entry, record = self.find(path.rsplit("/", 1)[1])
            self.test.assertIsNotNone(record)
            entry["screenshots"].remove(record)
            return 204, b""
        self.test.fail(f"unexpected {method} {path}")


class Harness(unittest.TestCase):
    """A module under test, a set directory in a tempdir and a fake store."""

    platform = "IOS"

    def setUp(self):
        self.m = load_module()
        self.sleeps: list = []
        self.m.sleep = self.sleeps.append
        # The clock advances 100 s per read, so a poll that never completes
        # reaches the tool's limit within a few rounds rather than spinning
        # through POLL_LIMIT real seconds with sleep stubbed out.
        clock = itertools.count(0, 100)
        self.m.monotonic = lambda: float(next(clock))
        self.version = ledger_version(self.platform)
        self.tmp = tempfile.mkdtemp()
        self.files: dict = {}

    def write_set(self, display: str, count: int = 3, *, size=None, **kwargs) -> list:
        directory = os.path.join(self.tmp, SUBDIRS[display])
        os.makedirs(directory, exist_ok=True)
        width, height = size or SIZES[display]
        names = []
        for n in range(1, count + 1):
            name = f"{n:02d}-shot.png"
            data = png(width, height, seed=n + len(self.files), **kwargs)
            with open(os.path.join(directory, name), "wb") as handle:
                handle.write(data)
            self.files[(display, name)] = data
            names.append(name)
        return names

    def write_full_set(self, count: int = 3):
        for _sub, display in self.m.DISPLAY_TYPES[self.platform]:
            self.write_set(display, count)

    def existing(self, display: str, count: int = 2, state: str = "COMPLETE") -> list:
        return [FakeStore.record(f"old-{display}-{n}", f"old-{n:02d}.png", 100 + n,
                                 f"{n:032x}", state) for n in range(1, count + 1)]

    def store(self, **kwargs) -> FakeStore:
        kwargs.setdefault("version", self.version)
        fake = FakeStore(self, **kwargs)
        self.m.connect = fake.connect
        return fake

    def run_tool(self, *extra: str):
        argv = ["--platform", self.platform, "--set-dir", self.tmp, *extra]
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = self.m.main(argv)
        return code, out.getvalue()

    def refused(self, *extra: str):
        out, err = io.StringIO(), io.StringIO()
        with self.assertRaises(SystemExit) as stop, contextlib.redirect_stdout(out), \
                contextlib.redirect_stderr(err):
            self.m.main(["--platform", self.platform, "--set-dir", self.tmp, *extra])
        code = stop.exception.code
        self.assertTrue(code not in (0, None), f"exit {code!r} is not a refusal")
        return f"{code}\n{out.getvalue()}\n{err.getvalue()}"

    def md5(self, display: str, name: str) -> str:
        """hashlib's own MD5 of the file, so a wrong checksum in the tool
        cannot be held to itself here."""
        return hashlib.md5(self.files[(display, name)]).hexdigest()


class LocalValidation(Harness):
    def test_a_ready_set_passes_local_only_without_connecting(self):
        self.write_full_set()
        fake = self.store()
        code, out = self.run_tool("--local-only")
        self.assertEqual(code, 0)
        self.assertIn("local-only mode: no App Store Connect request made", out)
        self.assertEqual(fake.connected, 0)
        self.assertEqual(fake.calls, [])

    def test_the_sizes_and_alpha_rule_are_preflights_own(self):
        preflight = self.m.load_preflight()
        self.assertIn((1320, 2868), preflight.ACCEPTED_SCREENSHOT_SIZES["IOS"])
        self.assertEqual(preflight.png_inspect.__module__, "appstore_preflight")
        self.assertFalse(hasattr(self.m, "ACCEPTED_SCREENSHOT_SIZES"))
        self.assertFalse(hasattr(self.m, "png_inspect"))

    def test_an_off_size_png_is_refused(self):
        self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2, size=(2000, 2000))
        self.store()
        message = self.refused("--local-only")
        self.assertIn("2000x2000", message)
        self.assertIn("APP_IPAD_PRO_3GEN_129", message)

    def test_an_alpha_png_is_refused(self):
        self.write_set("APP_IPHONE_67", 2, alpha=True)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2)
        self.store()
        self.assertIn("alpha channel", self.refused("--local-only"))

    def test_a_transparency_chunk_counts_as_alpha(self):
        self.write_set("APP_IPHONE_67", 2, trns=True)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2)
        self.store()
        self.assertIn("alpha channel", self.refused("--local-only"))

    def test_an_empty_directory_is_refused(self):
        self.write_set("APP_IPHONE_67", 2)
        os.makedirs(os.path.join(self.tmp, "ipad13"))
        self.store()
        self.assertIn("holds no PNG", self.refused("--local-only"))

    def test_a_missing_directory_is_refused(self):
        self.write_set("APP_IPHONE_67", 2)
        self.store()
        message = self.refused("--local-only")
        self.assertIn("is missing", message)
        self.assertIn("ipad13", message)

    def test_more_than_a_set_holds_is_refused(self):
        self.write_set("APP_IPHONE_67", 11)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2)
        self.store()
        self.assertIn("at most 10", self.refused("--local-only"))

    def test_a_set_of_exactly_ten_is_accepted(self):
        self.write_set("APP_IPHONE_67", 10)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        self.store()
        code, out = self.run_tool("--local-only")
        self.assertEqual(code, 0)
        self.assertIn("APP_IPHONE_67: 10 from", out)

    def test_a_file_that_is_not_a_png_is_refused(self):
        self.write_full_set(2)
        with open(os.path.join(self.tmp, "03-shot.png"), "wb") as handle:
            handle.write(b"not a png at all")
        self.store()
        self.assertIn("is not a PNG", self.refused("--local-only"))

    def test_every_local_problem_stops_before_the_network(self):
        self.write_set("APP_IPHONE_67", 2, size=(1, 1))
        fake = self.store()
        self.refused()
        self.assertEqual(fake.connected, 0)


class MacPlatform(Harness):
    platform = "MAC_OS"

    def test_the_mac_set_is_the_desktop_display_type(self):
        self.write_set("APP_DESKTOP", 2)
        fake = self.store()
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertIn("APP_DESKTOP", out)
        self.assertIn("remote set: none; it would be created", out)
        self.assertIn("filter%5Bplatform%5D=MAC_OS", fake.calls[0][1])

    def test_an_iphone_size_is_off_size_for_the_mac(self):
        self.write_set("APP_DESKTOP", 2, size=(1320, 2868))
        self.store()
        self.assertIn("1320x2868", self.refused("--local-only"))

    def test_the_mac_version_comes_from_the_mac_ledger(self):
        # Two ledgers that disagree, so reading the mobile one for the Mac
        # is visible; the repository's own agree on every release.
        for ledger, version in (("cmd/mobile/FyneApp.toml", "9.9.9"),
                                ("cmd/bibletext/FyneApp.toml", "8.8.8")):
            os.makedirs(os.path.join(self.tmp, os.path.dirname(ledger)))
            with open(os.path.join(self.tmp, ledger), "w", encoding="utf-8") as handle:
                handle.write(f'Version = "{version}"\n')
        self.m.REPO = self.tmp
        self.write_set("APP_DESKTOP", 2)
        fake = self.store(writable=True)
        message = self.refused("--write", "--confirm-version", "9.9.9")
        self.assertIn("--confirm-version 8.8.8", message)
        self.assertEqual(fake.connected, 0)


class Arguments(Harness):
    def test_write_needs_the_exact_ledger_version(self):
        self.write_full_set()
        fake = self.store(writable=True)
        message = self.refused("--write", "--confirm-version", "0.0.1")
        self.assertIn(f"--confirm-version {self.version}", message)
        self.assertEqual(fake.connected, 0)

    def test_write_needs_a_confirmation_at_all(self):
        self.write_full_set()
        fake = self.store(writable=True)
        self.refused("--write")
        self.assertEqual(fake.connected, 0)

    def test_confirm_version_alone_is_refused(self):
        self.write_full_set()
        fake = self.store()
        self.assertIn("only with --write", self.refused("--confirm-version", self.version))
        self.assertEqual(fake.connected, 0)


class ReadOnlyPlan(Harness):
    def test_a_missing_version_record_names_the_tool_that_creates_it(self):
        self.write_full_set()
        fake = self.store(version=None)
        message = self.refused()
        self.assertIn(f"no IOS App Store version {self.version} exists", message)
        self.assertIn("submit-version.py", message)
        self.assertTrue(all(call[0] == "GET" for call in fake.calls))

    def test_the_localization_must_resolve(self):
        self.write_full_set()
        self.store(locales=("fr-FR",))
        self.assertIn("exactly one en-GB version localization", self.refused())

    def test_the_plan_names_what_exists_what_goes_and_what_comes(self):
        self.write_full_set()
        fake = self.store(sets={"APP_IPHONE_67": self.existing("APP_IPHONE_67", 2, "FAILED"),
                                "APP_IPAD_PRO_3GEN_129": self.existing("APP_IPAD_PRO_3GEN_129", 1)})
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertIn(f"target: app {self.m.APP}, IOS {self.version}, en-GB, state PREPARE_FOR_SUBMISSION", out)
        self.assertIn("remote set set-APP_IPHONE_67: 2 images", out)
        self.assertIn(f"old-01.png  101 B  md5 {1:032x}  FAILED", out)
        self.assertIn("would delete all 2 existing images", out)
        self.assertIn("remote set set-APP_IPAD_PRO_3GEN_129: 1 images", out)
        self.assertIn("would delete all 1 existing images", out)
        uploads = re.findall(r"^\s+(\d+)\. (\d\d-shot\.png)  \d+ B  (\d+x\d+)  md5 ([0-9a-f]{32})$",
                             out, re.M)
        self.assertEqual([(n, name, size) for n, name, size, _ in uploads],
                         [("1", "01-shot.png", "1320x2868"), ("2", "02-shot.png", "1320x2868"),
                          ("3", "03-shot.png", "1320x2868"), ("1", "01-shot.png", "2064x2752"),
                          ("2", "02-shot.png", "2064x2752"), ("3", "03-shot.png", "2064x2752")])
        self.assertEqual(uploads[0][3], self.md5("APP_IPHONE_67", "01-shot.png"))
        self.assertEqual(uploads[5][3], self.md5("APP_IPAD_PRO_3GEN_129", "03-shot.png"))
        self.assertIn("DRY RUN: nothing deleted, uploaded or reordered", out)
        self.assertTrue(all(call[0] == "GET" for call in fake.calls), fake.calls)

    def test_no_write_is_made_without_the_flag_even_in_an_editable_state(self):
        self.write_full_set()
        fake = self.store(sets={"APP_IPHONE_67": self.existing("APP_IPHONE_67", 8)})
        code, _out = self.run_tool()
        self.assertEqual(code, 0)
        methods = {call[0] for call in fake.calls}
        self.assertEqual(methods, {"GET"})
        self.assertEqual(fake.uploads, {})
        self.assertEqual([r["id"] for r in fake.sets["APP_IPHONE_67"]["screenshots"]],
                         [f"old-APP_IPHONE_67-{n}" for n in range(1, 9)])

    def test_keep_existing_that_would_overflow_the_set_is_refused_in_the_plan(self):
        self.write_full_set(3)
        fake = self.store(sets={"APP_IPHONE_67": self.existing("APP_IPHONE_67", 8)}, writable=True)
        message = self.refused("--keep-existing", "--write", "--confirm-version", self.version)
        self.assertIn("11 images", message)
        self.assertIn("at most 10", message)
        self.assertTrue(all(call[0] == "GET" for call in fake.calls))

    def test_a_set_that_already_holds_the_files_has_nothing_to_do(self):
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name))
                for n, name in enumerate(names)]
        self.store(sets={"APP_IPHONE_67": held})
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertIn("already holds these files in this order, all COMPLETE", out)
        self.assertNotIn("would delete all 2", out)

    def test_a_set_holding_the_files_in_another_order_would_only_be_reordered(self):
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 2)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name))
                for n, name in enumerate(names)]
        held.reverse()
        fake = self.store(sets={"APP_IPHONE_67": held})
        code, out = self.run_tool()
        self.assertEqual(code, 0)
        self.assertIn("in another order; would only reorder them:\n"
                      "     1. 01-shot.png\n     2. 02-shot.png\n", out)
        self.assertNotIn("would delete all 2", out)
        self.assertTrue(all(call[0] == "GET" for call in fake.calls))


class Writing(Harness):
    def arrange(self, **kwargs):
        self.iphone = self.write_set("APP_IPHONE_67", 2)
        self.ipad = self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        kwargs.setdefault("sets", {"APP_IPHONE_67": self.existing("APP_IPHONE_67", 2)})
        kwargs.setdefault("writable", True)
        return self.store(**kwargs)

    def write(self, *extra):
        return self.run_tool("--write", "--confirm-version", self.version, *extra)

    def kinds(self, calls):
        """The write sequence as (method, what) pairs a reader can check."""
        out = []
        for call in calls:
            method, target = call[0], call[1]
            if method == "GET":
                continue
            if method == "PUT":
                out.append(("PUT", target.split("upload.invalid/")[1]))
            elif method == "DELETE":
                out.append(("DELETE", target.rsplit("/", 1)[1]))
            elif method == "POST":
                out.append(("POST", target))
            elif "/relationships/" in target:
                out.append(("PATCH", "order " + target.split("/")[3]))
            else:
                out.append(("PATCH", target.rsplit("/", 1)[1]))
        return out

    def readbacks(self, fake, display: str = "APP_IPHONE_67") -> int:
        """How many times the set was listed after its reorder: its read-backs."""
        ordered = [n for n, call in enumerate(fake.calls) if call[0] == "PATCH"
                   and call[1].endswith(f"set-{display}/relationships/appScreenshots")]
        return sum(1 for call in fake.calls[ordered[0]:] if call[0] == "GET"
                   and call[1] == f"/v1/appScreenshotSets/set-{display}/appScreenshots?limit=200")

    def test_the_sequence_delete_reserve_put_commit_order_poll_readback(self):
        fake = self.arrange()
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertEqual(self.kinds(fake.calls), [
            ("DELETE", "old-APP_IPHONE_67-1"), ("DELETE", "old-APP_IPHONE_67-2"),
            ("POST", "/v1/appScreenshots"), ("PUT", "new-0/1"), ("PUT", "new-0/2"), ("PATCH", "new-0"),
            ("POST", "/v1/appScreenshots"), ("PUT", "new-1/1"), ("PUT", "new-1/2"), ("PATCH", "new-1"),
            ("PATCH", "order set-APP_IPHONE_67"),
            ("POST", "/v1/appScreenshotSets"),
            ("POST", "/v1/appScreenshots"), ("PUT", "new-2/1"), ("PUT", "new-2/2"), ("PATCH", "new-2"),
            ("PATCH", "order set-APP_IPAD_PRO_3GEN_129"),
        ])
        gets = [call[1] for call in fake.calls if call[0] == "GET"]
        order_at = [n for n, call in enumerate(fake.calls) if call[0] == "PATCH" and "/relationships/" in call[1]]

        def polls_of(pattern):
            return [n for n, call in enumerate(fake.calls)
                    if call[0] == "GET" and re.fullmatch(pattern, call[1])]
        iphone_polls, ipad_polls = polls_of(r"/v1/appScreenshots/new-[01]"), polls_of(r"/v1/appScreenshots/new-2")
        readbacks = [n for n, call in enumerate(fake.calls)
                     if call[0] == "GET" and call[1].endswith("/appScreenshots?limit=200")
                     and n > order_at[0]]
        self.assertTrue(iphone_polls and max(iphone_polls) < order_at[0],
                        "the iPhone set is ordered only after its last image is COMPLETE")
        self.assertTrue(ipad_polls and order_at[0] < min(ipad_polls) and max(ipad_polls) < order_at[1],
                        "the iPad set is ordered only after its last image is COMPLETE")
        self.assertEqual(len(readbacks), 2, "each set is read back once")
        self.assertTrue(min(readbacks) > order_at[1], "read-back follows the last reorder")
        self.assertEqual(fake.polls, {"new-0": 2, "new-1": 2, "new-2": 2},
                         "each image is polled until COMPLETE")
        self.assertIn("every set read back as uploaded", out)
        self.assertEqual(len([g for g in gets if g.endswith("/appScreenshots?limit=200")]), 3)

    def test_each_upload_operation_gets_its_exact_bytes_and_headers(self):
        fake = self.arrange()
        self.write()
        data = self.files[("APP_IPHONE_67", "01-shot.png")]
        half = len(data) // 2
        puts = [call for call in fake.calls if call[0] == "PUT" and "new-0/" in call[1]]
        self.assertEqual([(c[1], c[2], c[3]) for c in puts], [
            ("https://upload.invalid/new-0/1", data[:half],
             {"Content-Type": "image/png", "X-Part": "1"}),
            ("https://upload.invalid/new-0/2", data[half:],
             {"Content-Type": "image/png", "X-Part": "2"}),
        ])
        self.assertNotIn("Authorization", puts[0][3])

    def test_the_reservation_carries_name_size_and_set(self):
        fake = self.arrange()
        self.write()
        reserve = [c[2] for c in fake.calls if c[0] == "POST" and c[1] == "/v1/appScreenshots"][0]
        self.assertEqual(reserve, {"data": {
            "type": "appScreenshots",
            "attributes": {"fileName": "01-shot.png",
                           "fileSize": len(self.files[("APP_IPHONE_67", "01-shot.png")])},
            "relationships": {"appScreenshotSet": {"data": {"type": "appScreenshotSets",
                                                            "id": "set-APP_IPHONE_67"}}}}})

    def test_the_commit_carries_uploaded_and_the_md5(self):
        fake = self.arrange()
        self.write()
        commits = [c[2] for c in fake.calls if c[0] == "PATCH" and c[1] == "/v1/appScreenshots/new-0"]
        self.assertEqual(commits, [{"data": {
            "type": "appScreenshots", "id": "new-0",
            "attributes": {"uploaded": True,
                           "sourceFileChecksum": self.md5("APP_IPHONE_67", "01-shot.png")}}}])

    def test_the_reorder_lists_the_new_ids_in_file_order(self):
        fake = self.arrange()
        self.write()
        orders = [c[2] for c in fake.calls if c[0] == "PATCH" and "/relationships/" in c[1]]
        self.assertEqual(orders[0], {"data": [{"type": "appScreenshots", "id": "new-0"},
                                              {"type": "appScreenshots", "id": "new-1"}]})
        self.assertEqual([r["attributes"]["fileName"] for r in fake.sets["APP_IPHONE_67"]["screenshots"]],
                         ["01-shot.png", "02-shot.png"])

    def test_keep_existing_appends_and_deletes_nothing(self):
        fake = self.arrange()
        code, out = self.write("--keep-existing")
        self.assertEqual(code, 0)
        self.assertNotIn("DELETE", [c[0] for c in fake.calls])
        orders = [c[2] for c in fake.calls if c[0] == "PATCH" and "/relationships/" in c[1]]
        self.assertEqual([item["id"] for item in orders[0]["data"]],
                         ["old-APP_IPHONE_67-1", "old-APP_IPHONE_67-2", "new-0", "new-1"])
        self.assertIn("APP_IPHONE_67 read back: 4 images", out)

    def test_keep_existing_holds_a_kept_image_listed_without_a_checksum_to_its_place_and_state(self):
        # A kept image is held to what the listing gave. One listed without
        # a checksum has none to be held to: the read-back checks its
        # position and state on the first read and does not wait for a
        # checksum that no upload of this run will bring.
        held = [FakeStore.record("held-0", "old-01.png", 100, None)]
        fake = self.arrange(sets={"APP_IPHONE_67": held})
        code, out = self.write("--keep-existing")
        self.assertEqual(code, 0)
        self.assertIn("APP_IPHONE_67 read back: 3 images", out)
        self.assertIn("[OK] read-back APP_IPHONE_67", out)
        self.assertIn("every set read back as uploaded", out)
        self.assertEqual(self.readbacks(fake), 1, "nothing to read again for")
        self.assertNotIn(self.m.READBACK_INTERVAL, self.sleeps)

    def test_keep_existing_still_holds_a_kept_image_to_the_checksum_the_listing_gave(self):
        # Only a kept image listed without a checksum goes unheld; one listed
        # with a checksum that reads back as another is a mismatch at once.
        fake = self.arrange()

        def change_a_kept_checksum(records):
            if records[-1]["id"].startswith("new-"):
                records[0]["attributes"]["sourceFileChecksum"] = "0" * 32
        fake.tamper = change_a_kept_checksum
        message = self.refused("--write", "--confirm-version", self.version, "--keep-existing")
        self.assertIn("read-back mismatch for APP_IPHONE_67", message)
        self.assertIn(f"old-01.png: checksum {'0' * 32}, expected {1:032x}", message)
        self.assertEqual(self.readbacks(fake), 1)
        self.assertNotIn(self.m.READBACK_INTERVAL, self.sleeps)

    def test_a_failed_delivery_is_a_non_zero_exit_naming_the_error(self):
        fake = self.arrange(delivery=("UPLOAD_COMPLETE", "FAILED"),
                            failure=[{"code": "IMAGE_TOO_SMALL", "description": "the image is 1x1"}])
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("APP_IPHONE_67 01-shot.png: assetDeliveryState FAILED", message)
        self.assertIn("IMAGE_TOO_SMALL: the image is 1x1", message)
        self.assertIn("The set is now partial", message)
        # The poll ends the run: the set is neither reordered nor read back.
        self.assertNotIn(("PATCH", "order set-APP_IPHONE_67"), self.kinds(fake.calls))
        polls = [n for n, call in enumerate(fake.calls)
                 if call[0] == "GET" and re.fullmatch(r"/v1/appScreenshots/new-\d+", call[1])]
        self.assertEqual([call for call in fake.calls[max(polls) + 1:]], [])

    def test_a_set_holding_the_files_in_another_order_is_only_reordered(self):
        names = self.write_set("APP_IPHONE_67", 3)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name))
                for n, name in enumerate(names)]
        held.reverse()
        fake = self.store(sets={"APP_IPHONE_67": held}, writable=True)
        code, out = self.write()
        self.assertEqual(code, 0)
        kinds = self.kinds(fake.calls)
        self.assertEqual(kinds[0], ("PATCH", "order set-APP_IPHONE_67"))
        self.assertNotIn("DELETE", [kind[0] for kind in kinds])
        self.assertEqual(kinds.count(("POST", "/v1/appScreenshots")), 1, "only the iPad's image goes up")
        orders = [c[2] for c in fake.calls if c[0] == "PATCH" and "set-APP_IPHONE_67/relationships" in c[1]]
        self.assertEqual([item["id"] for item in orders[0]["data"]], ["held-0", "held-1", "held-2"])
        self.assertEqual([r["attributes"]["fileName"] for r in fake.sets["APP_IPHONE_67"]["screenshots"]],
                         names)
        self.assertIn("APP_IPHONE_67 read back: 3 images", out)

    def test_a_set_holding_the_files_in_another_order_but_not_delivered_is_replaced(self):
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name), "FAILED")
                for n, name in enumerate(names)]
        held.reverse()
        fake = self.store(sets={"APP_IPHONE_67": held}, writable=True)
        code, _out = self.write()
        self.assertEqual(code, 0)
        self.assertIn(("DELETE", "held-0"), self.kinds(fake.calls))
        self.assertIn(("DELETE", "held-1"), self.kinds(fake.calls))

    def test_a_delivery_that_never_completes_gives_up_and_says_so(self):
        fake = self.arrange(delivery=("UPLOAD_COMPLETE",))
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("had not reached COMPLETE", message)
        self.assertIn("giving up", message)
        self.assertLess(fake.polls["new-0"], 20)
        self.assertNotIn(("PATCH", "order set-APP_IPHONE_67"), self.kinds(fake.calls))

    def test_a_read_back_checksum_that_is_wrong_is_a_mismatch_at_once(self):
        fake = self.arrange()

        def tamper(records):
            for record in records:
                if record["attributes"]["fileName"] == "02-shot.png":
                    record["attributes"]["sourceFileChecksum"] = "0" * 32
        fake.tamper = tamper
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("read-back mismatch for APP_IPHONE_67", message)
        self.assertIn("02-shot.png: checksum 000", message)
        self.assertEqual(self.readbacks(fake), 1, "a checksum that is present and wrong is not read again")
        self.assertNotIn(self.m.READBACK_INTERVAL, self.sleeps)

    def test_a_checksum_not_yet_reported_on_a_complete_image_is_read_again(self):
        # App Store Connect reports an image COMPLETE a moment before it
        # reports the image's checksum; the read-back waits for it rather
        # than calling a delivered image a mismatch.
        fake = self.arrange()
        reads = []

        def report_the_checksum_late(records):
            if not records[-1]["id"].startswith("new-"):
                return
            reads.append(len(reads))
            if len(reads) <= 2:
                for record in records:
                    if record["attributes"]["fileName"] == "02-shot.png":
                        record["attributes"]["sourceFileChecksum"] = None
        fake.tamper = report_the_checksum_late
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertEqual(self.readbacks(fake), 3, "read until the checksum is there")
        self.assertEqual(self.readbacks(fake, "APP_IPAD_PRO_3GEN_129"), 1)
        self.assertIn("no checksum yet on 02-shot.png; reading again in 10 s (read 1 of 6)", out)
        self.assertIn("[OK] read-back APP_IPHONE_67", out)
        self.assertIn("every set read back as uploaded", out)
        # Every wait — the two polls' and the two re-reads' — went through
        # the seam; nothing slept for real.
        self.assertEqual(self.sleeps, [self.m.POLL_INTERVAL, self.m.POLL_INTERVAL,
                                       self.m.READBACK_INTERVAL, self.m.READBACK_INTERVAL])

    def test_a_checksum_never_reported_is_a_mismatch_after_the_bound(self):
        fake = self.arrange()

        def never_report_a_checksum(records):
            for record in records:
                if record["id"].startswith("new-"):
                    record["attributes"]["sourceFileChecksum"] = None
        fake.tamper = never_report_a_checksum
        self.assertEqual((self.m.READBACK_ATTEMPTS, self.m.READBACK_INTERVAL), (6, 10.0))
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("read-back mismatch for APP_IPHONE_67", message)
        self.assertIn("01-shot.png: no checksum after 6 reads 10 s apart, expected "
                      + self.md5("APP_IPHONE_67", "01-shot.png"), message)
        self.assertIn("02-shot.png: no checksum after 6 reads 10 s apart", message)
        self.assertEqual(self.readbacks(fake), 6)
        self.assertEqual(self.sleeps.count(self.m.READBACK_INTERVAL), 5)
        # The first set's mismatch ends the run before the next is read back.
        self.assertEqual(self.readbacks(fake, "APP_IPAD_PRO_3GEN_129"), 0)

    def test_a_null_checksum_on_an_image_that_is_not_complete_is_a_mismatch_at_once(self):
        # Only a delivered image is waited for; a null checksum beside any
        # other disagreement is reported with it on the first read.
        fake = self.arrange()

        def tamper(records):
            for record in records:
                if record["id"].startswith("new-") and record["attributes"]["fileName"] == "02-shot.png":
                    record["attributes"]["sourceFileChecksum"] = None
                    record["attributes"]["assetDeliveryState"] = {"state": "UPLOAD_COMPLETE",
                                                                  "errors": []}
        fake.tamper = tamper
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("02-shot.png: assetDeliveryState UPLOAD_COMPLETE", message)
        self.assertIn("02-shot.png: checksum None, expected " + self.md5("APP_IPHONE_67", "02-shot.png"),
                      message)
        self.assertEqual(self.readbacks(fake), 1)
        self.assertNotIn(self.m.READBACK_INTERVAL, self.sleeps)

    def test_a_read_back_with_a_missing_image_is_a_non_zero_exit(self):
        fake = self.arrange()
        # Only a read-back of the uploaded set, never the listing the plan reads.
        fake.tamper = lambda records: records.pop() if records[-1]["id"].startswith("new-") else None
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("holds 1 images, expected 2", message)

    def test_a_read_back_out_of_order_is_a_non_zero_exit(self):
        fake = self.arrange()
        fake.tamper = lambda records: records.reverse() if records[-1]["id"].startswith("new-") else None
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("position 1 is 02-shot.png, expected 01-shot.png", message)

    def test_a_read_back_that_is_not_complete_is_a_non_zero_exit(self):
        fake = self.arrange()

        def tamper(records):
            for record in records:
                if record["id"].startswith("new-"):
                    record["attributes"]["assetDeliveryState"] = {"state": "UPLOAD_COMPLETE",
                                                                  "errors": []}
        fake.tamper = tamper
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("read-back mismatch for APP_IPHONE_67", message)
        self.assertIn("01-shot.png: assetDeliveryState UPLOAD_COMPLETE", message)

    def test_a_version_apple_holds_is_refused_before_any_write(self):
        fake = self.arrange(state="WAITING_FOR_REVIEW")
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("refusing to write version in WAITING_FOR_REVIEW", message)
        self.assertEqual({call[0] for call in fake.calls}, {"GET"})

    def test_a_version_in_an_unsent_submission_is_refused_before_any_write(self):
        # READY_FOR_REVIEW is a version added to a review submission that has
        # not been sent: text still changes there, images and previews do not.
        fake = self.arrange(state="READY_FOR_REVIEW")
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("refusing to write version in READY_FOR_REVIEW", message)
        self.assertEqual({call[0] for call in fake.calls}, {"GET"})

    # Every state in which App Store Connect no longer takes image edits, in
    # the appVersionState vocabulary and the older appStoreState one, with the
    # state the released records sit in and the one an unreadable record gets.
    HELD_STATES = (
        "READY_FOR_REVIEW", "WAITING_FOR_REVIEW", "IN_REVIEW",
        "PENDING_DEVELOPER_RELEASE", "PENDING_APPLE_RELEASE",
        "PROCESSING_FOR_DISTRIBUTION", "PROCESSING_FOR_APP_STORE",
        "READY_FOR_DISTRIBUTION", "READY_FOR_SALE", "ACCEPTED",
        "REPLACED_WITH_NEW_VERSION", "WAITING_FOR_EXPORT_COMPLIANCE",
        "REMOVED_FROM_SALE", "DEVELOPER_REMOVED_FROM_SALE", "UNKNOWN",
    )

    def test_every_state_apple_or_the_store_holds_is_refused_before_any_write(self):
        for state in self.HELD_STATES:
            with self.subTest(state=state):
                fake = self.arrange(state=state)
                message = self.refused("--write", "--confirm-version", self.version)
                self.assertIn(f"refusing to write version in {state}", message)
                self.assertEqual({call[0] for call in fake.calls}, {"GET"})

    def test_the_editable_states_are_preparation_and_the_states_that_hand_a_version_back(self):
        self.assertEqual(self.m.EDITABLE_STATES, {
            "PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED",
            "METADATA_REJECTED", "INVALID_BINARY"})
        for state in sorted(self.m.EDITABLE_STATES):
            with self.subTest(state=state):
                fake = self.arrange(state=state)
                code, _out = self.write()
                self.assertEqual(code, 0)
                self.assertIn(("DELETE", "old-APP_IPHONE_67-1"), self.kinds(fake.calls))

    def test_the_state_is_read_from_appVersionState_before_the_deprecated_attribute(self):
        # A released record today: READY_FOR_SALE in the deprecated attribute,
        # READY_FOR_DISTRIBUTION in the one that replaces it.
        self.arrange(version_attributes={"appStoreState": "READY_FOR_SALE",
                                         "appVersionState": "READY_FOR_DISTRIBUTION"})
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("state READY_FOR_DISTRIBUTION", message)
        self.assertIn("refusing to write version in READY_FOR_DISTRIBUTION", message)

    def test_a_record_without_the_deprecated_attribute_still_resolves(self):
        self.arrange(version_attributes={"appVersionState": "PREPARE_FOR_SUBMISSION"})
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertIn("state PREPARE_FOR_SUBMISSION", out)

    def test_a_record_with_only_the_deprecated_attribute_still_resolves(self):
        self.arrange(version_attributes={"appStoreState": "PREPARE_FOR_SUBMISSION"})
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertIn("state PREPARE_FOR_SUBMISSION", out)

    def test_a_record_with_neither_state_attribute_is_refused(self):
        fake = self.arrange(version_attributes={})
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("refusing to write version in UNKNOWN", message)
        self.assertEqual({call[0] for call in fake.calls}, {"GET"})

    def test_a_missing_version_record_is_refused_with_write_too(self):
        fake = self.arrange(version=None)
        self.assertIn("submit-version.py", self.refused("--write", "--confirm-version", self.version))
        self.assertEqual({call[0] for call in fake.calls}, {"GET"})

    def test_a_set_already_holding_the_files_is_left_alone(self):
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name))
                for n, name in enumerate(names)]
        fake = self.store(sets={"APP_IPHONE_67": held}, writable=True)
        code, _out = self.write()
        self.assertEqual(code, 0)
        self.assertNotIn(("DELETE", "held-0"), self.kinds(fake.calls))
        self.assertEqual([r["id"] for r in fake.sets["APP_IPHONE_67"]["screenshots"]],
                         ["held-0", "held-1"])
        self.assertIn(("POST", "/v1/appScreenshotSets"), self.kinds(fake.calls))

    def test_a_set_holding_the_same_names_with_other_content_is_replaced(self):
        # A retaken set keeps its file names from release to release; only
        # the checksums say the images changed.
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        held = [FakeStore.record(f"held-{n}", name, 10, f"{n + 1:032x}")
                for n, name in enumerate(names)]
        fake = self.store(sets={"APP_IPHONE_67": held}, writable=True)
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertNotIn("nothing to do", out)
        self.assertNotIn("would only reorder", out)
        self.assertIn(("DELETE", "held-0"), self.kinds(fake.calls))
        self.assertIn(("DELETE", "held-1"), self.kinds(fake.calls))
        self.assertEqual([r["attributes"]["sourceFileChecksum"]
                          for r in fake.sets["APP_IPHONE_67"]["screenshots"]],
                         [self.md5("APP_IPHONE_67", name) for name in names])

    def test_a_set_holding_the_same_files_at_failed_is_replaced(self):
        names = self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        held = [FakeStore.record(f"held-{n}", name, 10, self.md5("APP_IPHONE_67", name), "FAILED")
                for n, name in enumerate(names)]
        fake = self.store(sets={"APP_IPHONE_67": held}, writable=True)
        code, out = self.write()
        self.assertEqual(code, 0)
        self.assertNotIn("nothing to do", out)
        self.assertIn(("DELETE", "held-0"), self.kinds(fake.calls))
        self.assertIn(("DELETE", "held-1"), self.kinds(fake.calls))

    def test_keep_existing_up_to_exactly_ten_is_accepted(self):
        self.write_set("APP_IPHONE_67", 2)
        self.write_set("APP_IPAD_PRO_3GEN_129", 1)
        self.store(sets={"APP_IPHONE_67": self.existing("APP_IPHONE_67", 8)}, writable=True)
        code, out = self.write("--keep-existing")
        self.assertEqual(code, 0)
        self.assertIn("APP_IPHONE_67 read back: 10 images", out)

    def test_a_file_changed_after_validation_is_refused_before_upload(self):
        fake = self.arrange()
        path = os.path.join(self.tmp, "01-shot.png")

        def connect_after_a_rewrite():
            with open(path, "ab") as handle:
                handle.write(b"tail")
            return fake.connect()
        self.m.connect = connect_after_a_rewrite
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("01-shot.png changed on disk", message)
        self.assertNotIn("PUT", [call[0] for call in fake.calls])
        self.assertNotIn("PATCH", [call[0] for call in fake.calls])

    def test_a_rejected_upload_part_stops_the_run_before_the_commit(self):
        fake = self.arrange()
        accept = fake.upload

        def reject_the_second_part(operation, chunk):
            if operation["url"].endswith("/2"):
                fake.calls.append(("PUT", operation["url"], chunk, {}))
                return 403, b"<Error><Code>AccessDenied</Code></Error>"
            return accept(operation, chunk)
        fake.upload = reject_the_second_part
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("[ERROR 403] upload 01-shot.png bytes", message)
        self.assertIn("AccessDenied", message)
        self.assertIn("write stopped", message)
        self.assertNotIn("PATCH", [call[0] for call in fake.calls])
        self.assertEqual(self.kinds(fake.calls)[-1], ("PUT", "new-0/2"))

    def test_a_refused_write_stops_the_run(self):
        fake = self.arrange()
        real_write = fake.write

        def refuse_second_delete(method, path, body):
            if method == "DELETE" and path.endswith("-2"):
                return 409, {"errors": [{"title": "conflict"}]}
            return real_write(method, path, body)
        fake.write = refuse_second_delete
        message = self.refused("--write", "--confirm-version", self.version)
        self.assertIn("write stopped", message)
        self.assertNotIn("POST", [c[0] for c in fake.calls])


class UploadClient(Harness):
    """Client.upload is the one HTTP request the fake store does not stand in
    for: the PUT of a byte range to the pre-signed URL an operation names."""

    OPERATION = {"method": "PUT", "url": "https://upload.invalid/x/1", "offset": 0, "length": 3,
                 "requestHeaders": [{"name": "Content-Type", "value": "image/png"},
                                    {"name": "X-Part", "value": "1"}]}

    class Response(io.BytesIO):
        status = 200

        def __enter__(self):
            return self

        def __exit__(self, *exc):
            return False

    def test_the_put_carries_the_operations_headers_and_bytes_and_no_token(self):
        seen = []

        def urlopen(request, timeout=None):
            seen.append(request)
            return self.Response(b"")
        with mock.patch.object(urllib.request, "urlopen", urlopen):
            status, body = self.m.Client(None).upload(self.OPERATION, b"abc")
        self.assertEqual((status, body), (200, b""))
        request, = seen
        self.assertEqual(request.get_method(), "PUT")
        self.assertEqual(request.full_url, self.OPERATION["url"])
        self.assertEqual(request.data, b"abc")
        self.assertEqual({name.lower(): value for name, value in request.header_items()},
                         {"content-type": "image/png", "x-part": "1"})

    def test_a_refused_put_is_reported_as_its_status_and_body(self):
        def urlopen(request, timeout=None):
            raise urllib.error.HTTPError(request.full_url, 403, "Forbidden", {},
                                         io.BytesIO(b"<Error>AccessDenied</Error>"))
        with mock.patch.object(urllib.request, "urlopen", urlopen):
            status, body = self.m.Client(None).upload(self.OPERATION, b"abc")
        self.assertEqual((status, body), (403, b"<Error>AccessDenied</Error>"))


if __name__ == "__main__":
    unittest.main()
