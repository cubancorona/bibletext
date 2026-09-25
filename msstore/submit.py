#!/usr/bin/env python3
"""The Microsoft Store submission API, write side.

    . scripts/msstore-env.sh
    msstore/submit.py preflight <dir>       # read-only: prove the packages and the notes before anything is created
    msstore/submit.py verify <dir>          # read-only: re-check a staged submission from the server
    msstore/submit.py create <dir>          # POST + PUT + upload, then STOP before commit
    msstore/submit.py commit                # the point of no return
    msstore/submit.py poll                  # watch the commit through
    msstore/submit.py abort                 # delete the submission this run created

Standard library only, like msstore.py and appstore/asc.py. The read side lives
in msstore.py and is unchanged; this file imports its token helper rather than
minting a second one.

WHY THIS IS SPLIT INTO STAGES. Everything up to `commit` is reversible: the new
submission is a copy, and the live listing is untouched by the create, the
update or the upload. `commit` is the point of no return, and under
targetPublishMode Immediate there is no second checkpoint after it -- the
release goes public the moment certification passes. So the stages exist to put
a human between the reversible part and the irreversible one.

PUBLISH MODE. Immediate, deliberately, to match the App Store, where
appstore/submit-version.py sets releaseType AFTER_APPROVAL on every version.
One release policy across the stores. The value is SET explicitly on every run
and then read back from the server, never inherited from the clone: a clone
carries whatever the last published submission carried, and what that is has
been wrong in our own notes before.

WHAT A RELEASE CHANGES IN THE LISTING. One field: each listing's What's New,
releaseNotes in the API, from the release's own file under msstore/metadata/
(see RELEASE_NOTES_DIR). Every other listing field goes back exactly as the
clone brought it, and both the body sent and the submission read back are held
to that.

WHAT THIS DELIBERATELY DOES NOT DO. It does not mark the previously published
package PendingDelete. That costs nothing -- the Store serves the highest
applicable version, so 1.2.13.0 wins over 1.2.10.0 -- and it keeps a
known-good, already-certified package inside the published submission. Rolling
a bad release back then costs one PUT rather than a rebuild against a version
number that is already spent.
"""

from __future__ import annotations

import copy
import glob
import hashlib
import importlib.util
import json
import os
import re
import struct
import sys
import time
import unicodedata
import urllib.error
import urllib.parse
import urllib.request
import zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)

_spec = importlib.util.spec_from_file_location("msstore", os.path.join(HERE, "msstore.py"))
_ms = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_ms)

API = _ms.API
with open(os.path.join(HERE, "identity.json")) as _f:
    IDENTITY = json.load(_f)
STORE_ID = IDENTITY["storeId"]

STATE_DIR = os.path.join(REPO, "build", "msstore")
STATE = os.path.join(STATE_DIR, "run-state.json")

# The one listing language this product has. Asserted rather than assumed: a
# literal "en-us" would either KeyError mid-run, leaving a pending submission
# behind, or publish a blank second locale to the largest market.
EXPECTED_LISTINGS = {"en-gb"}

# Only these keys may differ between the clone and what we send, and inside
# `listings` only each language's baseListing.releaseNotes (see
# RELEASE_NOTES_DIR). Everything else must survive byte-identical: the en-GB
# listing was typed in by hand -- description, product features, captioned
# screenshots, box art, tile, keywords -- and a whole-object PUT that drops an
# unmodelled key wipes it. So `listings` stays out of this set, and the
# comparisons leave out that one field by name rather than the listing.
MUTABLE = {"applicationPackages", "targetPublishMode", "targetPublishDate"}

# Server-issued, not part of the submission payload. fileUploadUrl comes back
# from the POST and is not sent to the PUT, so it is neither mutable nor
# preserved -- it is simply not ours to compare.
NOT_PAYLOAD = {"fileUploadUrl"}

# What's New, the one listing field a release writes. Partner Center calls it
# "What's new in this version" (formerly "Release notes"); the API calls it
# listings -> <language> -> baseListing -> releaseNotes, a string. Until it was
# written here, each submission this tool made sent the clone's value, which
# was empty, so the Store never told a Windows reader what a release changed.
#
# The text is written for each release in a version-named file per listing
# language, msstore/metadata/<language>/whats-new-<version>.txt, the shape of
# the App Store's build/appstore/metadata/en-GB/whats-new-<version>.txt with
# the directory named by the API's listing key, and sent as written. The files
# are tracked, unlike the App Store's under the gitignored build/: the text is
# published verbatim on the Store in any case, the text a version was sent is
# then in the tree its tag names, and a tracked file lets CI hold every release
# to it (TestWindowsWhatsNewIsNamedForThisRelease), where the Apple files' test
# skips on any machine without build/ and so CI cannot see a missing one
# (docs/RELEASING.md, stage 0).
RELEASE_NOTES_DIR = os.path.join(HERE, "metadata")

# Partner Center's documented limit for the field ("Add and edit Store listing
# info for MSIX app" on Microsoft Learn, which also asks for the field to be
# blank on an app's first submission, the one this tool never makes). Counted
# in UTF-16 code units, which is how .NET measures a string and never fewer
# than the characters a reader sees, so a note inside it here is inside it by
# either count. The API has been reported enforcing a lower limit than the
# console for another listing field (shortDescription: "must be 500 or less"
# through the API, 1000 in the form, on Microsoft Q&A); if it does for this
# one, the PUT is refused with the limit in the message, nothing public has
# changed, and `abort` removes the draft.
RELEASE_NOTES_LIMIT = 1500

# The small capitals the app draws the divine name in (smallCapitals in
# small_caps_draw.go; test_submit.py holds the two equal). App Store Connect
# refused two of them in its own What's New, with INVALID_CHARACTERS, on
# 1.2.14. How the Store treats them is untested, and a Windows note is often the
# Apple one adapted; describing the name ("in small capitals") costs nothing,
# so a release is not where to find out.
DRAWN_SMALL_CAPITALS = frozenset("ᴀʙᴄᴅᴇꜰɢʜɪᴊᴋʟᴍɴᴏᴘꞯʀꜱᴛᴜᴠᴡʏᴢ")


def redact(url: str) -> str:
    """A fileUploadUrl carries sig=, sp=rwl and se=: a time-limited write
    credential for the ingestion blob. Never let it reach a log, a transcript
    or a file on disk."""
    return url.split("?")[0] + "?<sas-redacted>" if "?" in url else url


def redact_text(s: str) -> str:
    """The same job for free text: strip the query string off any blob URL
    embedded in a message, so no log line, error or saved file can carry a
    live write credential."""
    return re.sub(r"(https://[^\s\"]*\.blob\.core\.windows\.net/[^\s\"?]*)\?[^\s\"]*",
                  r"\1?<sas-redacted>", s)


def http(method: str, url: str, token: str | None = None, body: bytes | None = None,
         content_type: str = "application/json", extra: dict | None = None,
         timeout: int = 900):
    """Transport only. Returns (status, headers, raw bytes) and parses nothing.

    msstore.py's fetch() calls json.load() unconditionally, which raises on the
    empty body of a DELETE and on the empty 201 of the blob PUT -- AFTER the
    server has already acted. A caller that reads that as failure and retries
    re-deletes, or creates a second submission, or re-uploads to a consumed
    SAS. So success is decided on the status code, before any parsing.
    """
    headers = {}
    if token:
        headers["Authorization"] = "Bearer " + token
    if body is not None:
        headers["Content-Type"] = content_type
    if extra:
        headers.update(extra)
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, dict(r.headers), r.read()
    except urllib.error.HTTPError as e:
        raw = e.read()
        # MS-CorrelationId is what Microsoft support asks for first.
        cid = e.headers.get("MS-CorrelationId", "?")
        # The body is server-supplied free text and is the one place a URL we
        # did not write could leave the process -- so it goes through the same
        # redaction as everything else, and BEFORE the truncation, so a cut
        # cannot land inside the query string and leave half a signature.
        raise SystemExit(
            f"{method} {redact(url).split('?')[0]} -> HTTP {e.code} (MS-CorrelationId {cid})\n"
            f"{redact_text(raw.decode('utf-8', 'replace'))[:900]}"
        )


def api(method: str, path: str, token: str, payload=None):
    body = json.dumps(payload).encode() if payload is not None else None
    status, headers, raw = http(method, f"{API}{path}", token, body)
    # 201 is what POST /submissions returns; 204 is the empty body of a DELETE.
    # Any 2xx is success -- the parse happens after that verdict, never as part
    # of it, so a successful call with an empty body is not read as a failure.
    if not 200 <= status < 300:
        raise SystemExit(f"{method} {path} -> unexpected status {status}")
    return json.loads(raw) if raw.strip() else {}


def token() -> str:
    """Minted fresh at every stage. The Entra token lives 60 minutes and a full
    run is a create plus a multi-tens-of-MB upload plus polling."""
    return _ms.get_token()


def save_state(**kw):
    os.makedirs(STATE_DIR, exist_ok=True)
    s = load_state()
    s.update(kw)
    with open(STATE, "w") as f:
        json.dump(s, f, indent=2)
    return s


def load_state() -> dict:
    try:
        with open(STATE) as f:
            return json.load(f)
    except FileNotFoundError:
        return {}


# ------------------------------------------------------------ release notes

def desktop_version() -> str:
    """The desktop ledger's Version, which names both the packages and the
    release notes."""
    with open(os.path.join(REPO, "cmd", "bibletext", "FyneApp.toml")) as f:
        text = f.read()
    m = re.search(r'Version\s*=\s*"([0-9.]+)"', text)
    if not m:
        raise SystemExit("could not read Version from cmd/bibletext/FyneApp.toml")
    return m.group(1)


def utf16_length(text: str) -> int:
    return len(text.encode("utf-16-le")) // 2


def refused_character(text: str):
    """The first character a release note must not carry, as (line, column,
    character, reason), or None.

    Microsoft documents no character rule for the field, so what is refused is
    what cannot be what the writer meant a reader to see: anything but a
    letter, mark, number, punctuation, symbol, space or line break. That is a
    control character (a tab, the carriage return of a file saved with CRLF
    line ends), a format character (the byte-order mark some Windows editors
    write at the start of a file, a zero-width joiner, which also rules out an
    emoji composed of several), and a private-use or unassigned code point.
    And the drawn small capitals, for the reason at DRAWN_SMALL_CAPITALS.

    Unassigned is as of the Unicode version this Python knows, which can be
    older than the Go test's, so the reason names it: a character newer than
    that is refused here though CI passed it.
    """
    for n, line in enumerate(text.split("\n"), 1):
        for c, ch in enumerate(line, 1):
            if ch in DRAWN_SMALL_CAPITALS:
                return n, c, ch, "a drawn small capital; describe the divine name instead of showing it"
            cat = unicodedata.category(ch)
            if cat == "Cn":
                return n, c, ch, (f"not a visible character (unassigned in Unicode "
                                  f"{unicodedata.unidata_version}, the version this Python knows)")
            if cat[0] not in "LMNPS" and cat != "Zs":
                return n, c, ch, f"not a visible character (Unicode category {cat})"
    return None


def release_notes_path(lang: str, version: str) -> str:
    return os.path.join(RELEASE_NOTES_DIR, lang, f"whats-new-{version}.txt")


def read_release_notes(version: str) -> dict[str, str]:
    """Each listing language's What's New for `version`, as it will be sent,
    or SystemExit naming what is wrong with it.

    Read in full before anything is created, so an unfit note stops the run
    while the server holds nothing. The text is the file as written, less the
    line break an editor leaves at the end, the same reading
    appstore/push-metadata.py gives the App Store's.
    """
    notes = {}
    for lang in sorted(EXPECTED_LISTINGS):
        path = release_notes_path(lang, version)
        rel = os.path.relpath(path, REPO)
        try:
            with open(path, "rb") as f:
                raw = f.read()
        except FileNotFoundError:
            raise SystemExit(f"{rel} is missing. The {lang} listing's What's New for {version} is written "
                             f"for each release, like the App Store's, and create will not send a blank "
                             f"one or an earlier release's.") from None
        try:
            text = raw.decode("utf-8").rstrip("\n")
        except UnicodeDecodeError as e:
            raise SystemExit(f"{rel} is not UTF-8 (byte {e.start}); save it as UTF-8") from None
        if not text.strip():
            raise SystemExit(f"{rel} is empty")
        bad = refused_character(text)
        if bad:
            n, c, ch, why = bad
            raise SystemExit(f"{rel}: line {n}, column {c} is U+{ord(ch):04X}, {why}")
        size = utf16_length(text)
        if size > RELEASE_NOTES_LIMIT:
            raise SystemExit(f"{rel} is {size} characters; the Store takes at most {RELEASE_NOTES_LIMIT}. "
                             f"Remove {size - RELEASE_NOTES_LIMIT}.")
        for other in sorted(glob.glob(os.path.join(os.path.dirname(path), "whats-new-*.txt"))):
            if os.path.basename(other) == os.path.basename(path):
                continue
            with open(other, encoding="utf-8", errors="replace") as f:
                if f.read().strip() == text.strip():
                    raise SystemExit(f"{rel} is the same text as {os.path.relpath(other, REPO)}; "
                                     f"the notes for {version} would describe a different release")
        notes[lang] = text
    return notes


def notes_digest(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def apply_release_notes(body: dict, notes: dict[str, str]):
    """Write each listing's releaseNotes into the body, and nothing else. A
    platformOverrides entry is a base listing too and could carry notes of its
    own; it is left as the clone had it."""
    listings = body.get("listings") or {}
    if {k.lower() for k in listings} != set(notes):
        raise SystemExit(f"listing languages are {sorted(listings)}, release notes are for {sorted(notes)}; "
                         f"a new locale must be handled deliberately, not by a default")
    for lang, val in listings.items():
        base = (val or {}).get("baseListing")
        if not isinstance(base, dict):
            raise SystemExit(f"{lang}: the clone has no baseListing to carry the release notes")
        base["releaseNotes"] = notes[lang.lower()]


def _listings_but_release_notes(listings):
    """The listings with each baseListing's releaseNotes left out: everything
    the comparisons still hold byte-identical. Only that one field, and only
    in baseListing -- a releaseNotes anywhere else is a change like any
    other."""
    if not isinstance(listings, dict):
        return listings
    out = copy.deepcopy(listings)
    for val in out.values():
        base = val.get("baseListing") if isinstance(val, dict) else None
        if isinstance(base, dict):
            base.pop("releaseNotes", None)
    return out


def _release_notes_in(listings, lang: str):
    for k, val in (listings or {}).items():
        if k.lower() == lang:
            return ((val or {}).get("baseListing") or {}).get("releaseNotes")
    return None


# ---------------------------------------------------------------- preflight

def read_packages(d: str) -> list[dict]:
    """Prove each MSIX is what it claims before anything is created.

    Every assertion here can fail, and each one stands for a way a package has
    actually gone wrong somewhere: a mislabelled architecture (the arm64 leg
    silently falling back to the runner's x86_64 gcc), a missing ANGLE (the app
    creates no window on a driverless certification VM), an identity that does
    not match the reservation (rejected at ingestion), a fourth version field
    that is not 0 (reserved for Store use).
    """
    MACHINE = {"x64": 0x8664, "arm64": 0xAA64}
    expect_version = desktop_version() + ".0"

    out = []
    for name in sorted(os.listdir(d)):
        if not name.endswith(".msix"):
            continue
        path = os.path.join(d, name)
        if not name.isascii() or " " in name:
            raise SystemExit(f"{name}: package filenames must be plain ASCII with no spaces")
        with zipfile.ZipFile(path) as z:
            manifest = z.read("AppxManifest.xml").decode("utf-8", "replace")
            ident = re.search(r"<Identity\b[^>]*>", manifest, re.S).group(0)

            def attr(k):
                mm = re.search(rf'{k}="([^"]+)"', ident)
                return mm.group(1) if mm else None

            got = {k: attr(k) for k in ("Name", "Publisher", "Version", "ProcessorArchitecture")}
            if got["Name"] != IDENTITY["identityName"]:
                raise SystemExit(f"{name}: Identity Name {got['Name']!r} != {IDENTITY['identityName']!r}")
            if got["Publisher"] != IDENTITY["identityPublisher"]:
                raise SystemExit(f"{name}: Identity Publisher {got['Publisher']!r} != reservation")
            if got["Version"] != expect_version:
                raise SystemExit(f"{name}: Version {got['Version']!r} != {expect_version!r} (FyneApp.toml + '.0')")
            if not got["Version"].endswith(".0"):
                raise SystemExit(f"{name}: the fourth version field is reserved for Store use and must be 0")

            arch = got["ProcessorArchitecture"]
            if arch not in MACHINE:
                raise SystemExit(f"{name}: unexpected architecture {arch!r}")

            names = z.namelist()
            for dll in ("libEGL.dll", "libGLESv2.dll", "d3dcompiler_47.dll"):
                if not any(n.lower().endswith(dll.lower()) for n in names):
                    raise SystemExit(f"{name}: {dll} missing -- without ANGLE the app creates no window "
                                     f"on a driverless certification VM")

            exe = [n for n in names if n.lower().endswith("bibletext.exe")]
            if not exe:
                raise SystemExit(f"{name}: no BibleText.exe inside the package")
            head = z.read(exe[0])[:1024]
            e_lfanew = struct.unpack_from("<I", head, 0x3C)[0]
            machine = struct.unpack_from("<H", head, e_lfanew + 4)[0]
            if machine != MACHINE[arch]:
                raise SystemExit(f"{name}: manifest says {arch} but the PE header says {machine:#06x} "
                                 f"-- the binary does not match its label")

        digest = hashlib.sha256(open(path, "rb").read()).hexdigest()
        out.append({"fileName": name, "path": path, "architecture": arch,
                    "version": got["Version"], "bytes": os.path.getsize(path), "sha256": digest})

    archs = sorted(p["architecture"] for p in out)
    if archs != ["arm64", "x64"]:
        raise SystemExit(f"expected exactly one x64 and one arm64 package, got {archs}")
    return out


def cmd_preflight(d: str):
    pkgs = read_packages(d)
    print(f"packages in {d}:")
    for p in pkgs:
        print(f"  {p['fileName']:34} {p['architecture']:6} {p['version']:10} "
              f"{p['bytes']:>10,} bytes  sha256={p['sha256'][:16]}")
    total = sum(p["bytes"] for p in pkgs)
    print(f"\n  zip payload ~{total/1048576:.1f} MiB (MSIX is already compressed, expect near-zero deflation)")
    if total > 60 * 1048576:
        print("  ! close to the 64 MiB single-shot Put Blob ceiling of the older SAS versions")

    version = desktop_version()
    notes = read_release_notes(version)
    print(f"\nrelease notes (What's New) for {version}:")
    for lang, text in sorted(notes.items()):
        print(f"  {lang}  {os.path.relpath(release_notes_path(lang, version), REPO)}  "
              f"{utf16_length(text)} of {RELEASE_NOTES_LIMIT} characters")

    tok = token()
    app = api("GET", f"/applications/{STORE_ID}", tok)
    pending = app.get("pendingApplicationSubmission")
    published = (app.get("lastPublishedApplicationSubmission") or {}).get("id")
    print(f"\naccount:")
    print(f"  firstPublishedDate  {app.get('firstPublishedDate')}")
    print(f"  lastPublished       {published}")
    print(f"  pending             {pending.get('id') if pending else 'none'}")
    if pending:
        mine = load_state().get("submissionId")
        if pending.get("id") != mine:
            raise SystemExit(
                f"\nSTOP: a pending submission {pending.get('id')} exists that this automation did not create.\n"
                f"It may be a draft started in Partner Center. Deleting it would destroy that work with no undo.\n"
                f"Clear or finish it in the console, then re-run.")
    return pkgs


# ------------------------------------------------------------------- create

def assert_clone_preserved(clone: dict, body: dict, notes: dict[str, str]):
    """Everything outside MUTABLE must be byte-identical to what the server
    handed us, and so must every listing field but releaseNotes, which must be
    the release's own text. This is the guard against the whole-object PUT
    quietly dropping the hand-entered listing."""
    for key in set(clone) | set(body):
        if key in MUTABLE or key in NOT_PAYLOAD:
            continue
        a_val, b_val = clone.get(key), body.get(key)
        if key == "listings":
            a_val, b_val = _listings_but_release_notes(a_val), _listings_but_release_notes(b_val)
        a = json.dumps(a_val, sort_keys=True)
        b = json.dumps(b_val, sort_keys=True)
        if a != b:
            # The values are echoed to say WHAT differs, so they go through the
            # same redaction as every other log line: a submission can carry a
            # SAS, and an abort path is no excuse for printing one.
            raise SystemExit(f"refusing to PUT: {key!r} would change and is not in the mutable set\n"
                             f"  clone: {redact_text(a)[:200]}\n  body : {redact_text(b)[:200]}")

    listings = body.get("listings") or {}
    if {k.lower() for k in listings} != EXPECTED_LISTINGS:
        raise SystemExit(f"listing languages are {sorted(listings)}, expected {sorted(EXPECTED_LISTINGS)}; "
                         f"a new locale must be handled deliberately, not by a default")
    for lang, val in listings.items():
        base = (val or {}).get("baseListing") or {}
        # A language with no note read for it is a refusal of its own, not a
        # comparison: an absent note and an absent field are both None, and
        # would agree.
        text = notes.get(lang.lower())
        if not isinstance(text, str) or base.get("releaseNotes") != text:
            raise SystemExit(f"refusing to PUT: {lang} releaseNotes is not the text read from "
                             f"that language's whats-new file")
        imgs = base.get("images") or []
        cimgs = ((clone["listings"][lang] or {}).get("baseListing") or {}).get("images") or []
        if len(imgs) != len(cimgs):
            raise SystemExit(f"{lang}: image count changed {len(cimgs)} -> {len(imgs)}")
        for im in imgs:
            if im.get("fileStatus") != "Uploaded":
                raise SystemExit(f"{lang}: an image has fileStatus {im.get('fileStatus')!r}; "
                                 f"anything but 'Uploaded' either fails the commit or deletes the screenshot")


def cmd_create(d: str, publish_mode: str):
    pkgs = cmd_preflight(d)
    # Read here as well as in preflight, and before the POST: this is the read
    # whose text is sent, and a note that is missing or unfit has to stop the
    # run while there is still nothing on the server to abort.
    notes = read_release_notes(desktop_version())
    tok = token()

    print("\n==> creating the submission (a copy of the last published one)")
    sub = api("POST", f"/applications/{STORE_ID}/submissions", tok)
    # The id goes to disk the instant it is known and BEFORE anything is
    # checked about the response. From this line on the server holds a live
    # pending submission, and `abort` can only reach it by that id: an exit
    # between the POST and the write would leave an orphan that preflight then
    # reports as someone else's work and steers the operator away from
    # deleting. Record first; judge second.
    #
    # verified is cleared in the same write. save_state merges into the last
    # run's state, and the last release left verified true behind it; kept,
    # it would stand for this submission, so a read-back that refuses below
    # would still leave commit open.
    sid = sub.get("id")
    if sid:
        save_state(submissionId=sid, created=time.time(), committed=False, verified=False,
                   publishMode=publish_mode,
                   packages=[{k: p[k] for k in ("fileName", "sha256", "bytes", "architecture")} for p in pkgs],
                   releaseNotes={lang: notes_digest(text) for lang, text in notes.items()})
    else:
        raise SystemExit("the created submission carries no id; check the account in Partner Center "
                         "for a pending submission before creating another")
    upload_url = sub.get("fileUploadUrl")
    if not upload_url:
        raise SystemExit(f"the created submission {sid} carries no fileUploadUrl; "
                         f"delete it with: msstore/submit.py abort")
    print(f"    id {sid}  status {sub.get('status')}")
    print(f"    upload {redact(upload_url)}")

    # Keep the exact pre-change state, SAS stripped. It is the reference a
    # recovery PUT diffs against, and build/ is gitignored.
    os.makedirs(STATE_DIR, exist_ok=True)
    safe = copy.deepcopy(sub)
    safe["fileUploadUrl"] = redact(upload_url)
    with open(os.path.join(STATE_DIR, f"clone-{sid}.json"), "w") as f:
        json.dump(safe, f, indent=2)

    try:
        cloned_pkgs = sub.get("applicationPackages") or []
        print(f"\n    cloned packages ({len(cloned_pkgs)}):")
        for p in cloned_pkgs:
            print(f"      {p.get('fileName')}  {p.get('version')}  {p.get('architecture')}  {p.get('fileStatus')}")

        # The rollback story rests on "the Store serves the highest applicable
        # version, so the new package wins over the kept one". That is only
        # true if the new one IS higher, and both sides are in hand here, so
        # it is asserted rather than assumed -- per architecture, since the
        # kept x64 package says nothing about an arm64 one. And a new package
        # must not share a fileName with a kept entry: verify_staged keys the
        # server's list by fileName, and two entries under one name collapse
        # to whichever the server listed last.
        def vtuple(v):
            return tuple(int(x) for x in (v or "0").split("."))
        kept_names = {p.get("fileName") for p in cloned_pkgs}
        for p in pkgs:
            if p["fileName"] in kept_names:
                raise SystemExit(f"{p['fileName']} is already a package in the published submission; "
                                 f"a new package needs a new name")
            for k in cloned_pkgs:
                if k.get("architecture") == p["architecture"] and k.get("fileStatus") == "Uploaded" \
                        and vtuple(p["version"]) <= vtuple(k.get("version")):
                    raise SystemExit(f"{p['fileName']} is {p['version']}, not above the kept "
                                     f"{k.get('version')} {k.get('architecture')} package; the Store "
                                     f"would keep serving the old one")

        body = copy.deepcopy(sub)
        body.pop("fileUploadUrl", None)

        # The cloned entries are round-tripped exactly as returned -- their
        # fileName is whatever Partner Center recorded by hand, and is NOT
        # reconstructed. They stay 'Uploaded' on purpose (see the module note).
        new_entries = [{"fileName": p["fileName"], "fileStatus": "PendingUpload",
                        "minimumDirectXVersion": "None", "minimumSystemRam": "None"} for p in pkgs]
        body["applicationPackages"] = list(cloned_pkgs) + new_entries
        body["targetPublishMode"] = publish_mode
        body["targetPublishDate"] = "1601-01-01T00:00:00Z"
        apply_release_notes(body, notes)

        assert_clone_preserved(sub, body, notes)
        print(f"\n==> updating it (publish mode {publish_mode}, "
              f"{len(cloned_pkgs)} kept + {len(new_entries)} new, What's New for "
              f"{', '.join(sorted(notes))})")
        api("PUT", f"/applications/{STORE_ID}/submissions/{sid}", tok, body)

        zip_path = os.path.join(STATE_DIR, f"upload-{sid}.zip")
        with zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED) as z:
            for p in pkgs:
                z.write(p["path"], arcname=p["fileName"])   # zip ROOT, no prefix
        with zipfile.ZipFile(zip_path) as z:
            entries = z.namelist()
        want = {p["fileName"] for p in pkgs}
        if set(entries) != want:
            raise SystemExit(f"zip holds {entries}, expected exactly {sorted(want)}")
        print(f"\n==> uploading {os.path.getsize(zip_path)/1048576:.1f} MiB -> {redact(upload_url)}")

        sv = urllib.parse.parse_qs(urllib.parse.urlparse(upload_url).query).get("sv", ["?"])[0]
        url = upload_url
        if sv < "2019-12-12":
            # Older SAS versions cap a single-shot Put Blob at 64 MiB. Ask for a
            # newer protocol version explicitly rather than hope.
            url += ("&" if "?" in url else "?") + "api-version=2019-12-12"
            print(f"    SAS sv={sv} -> pinning api-version=2019-12-12 for the 5000 MiB ceiling")

        data = open(zip_path, "rb").read()
        # No Authorization header: the SAS query string IS the authorization,
        # and the manage.devcenter token must never leave that host.
        # A single deadline for the whole body: 54 MB on a slow uplink is a
        # correct run that 900 s would cut off, so this one call gets an hour.
        status, _h, raw = http("PUT", url, None, data, "application/zip",
                               {"x-ms-blob-type": "BlockBlob"}, timeout=3600)
        if status != 201:
            raise SystemExit(f"blob upload returned {status}, expected 201")
        print(f"    201 Created")

        verify_staged(sid, pkgs, publish_mode, sub, notes)
        save_state(verified=True)
        print(f"\ncreated and staged, NOT committed. Nothing public has changed.")
        print(f"  commit:  msstore/submit.py commit")
        print(f"  abort :  msstore/submit.py abort")
    except BaseException:
        print(f"\n!! failed after creating submission {sid}. It is still pending and blocks the next one.")
        print(f"   delete it with: msstore/submit.py abort")
        raise


# Keys the server owns and rewrites on its own: never ours to compare.
#
# friendlyName is Partner Center's display label for the submission ("Submission
# 3"); the server clears it when the submission is updated. It is not part of
# the listing and no reader sees it. Found on the first release this comparison
# ran against, 23 September 2026.
SERVER_OWNED = {"status", "statusDetails", "friendlyName"}

# Sub-keys the server owns inside a key that is otherwise compared. Only the
# named sub-key is exempt: the rest of `pricing` -- the price itself, the market
# availability, the trial -- must still come back exactly as it went, because a
# silently changed price is the failure this comparison most exists to catch.
# isAdvancedPricingModel is documented read-only and the server set it from
# true to false on that same release.
SERVER_OWNED_WITHIN = {"pricing": {"isAdvancedPricingModel"}}


def _comparable(key: str, value):
    if key == "listings":
        # releaseNotes is checked on its own, against the file, in verify_staged.
        return _listings_but_release_notes(value)
    exempt = SERVER_OWNED_WITHIN.get(key)
    if exempt and isinstance(value, dict):
        return {k: v for k, v in value.items() if k not in exempt}
    return value


def verify_staged(sid: str, pkgs: list[dict], publish_mode: str, clone: dict, notes: dict[str, str]):
    """Read the submission back FROM THE SERVER and refuse to go on unless it
    is exactly what we meant. Deliberately a fresh GET rather than the PUT's
    own response: what matters is what Partner Center stored, not what it
    echoed.

    `clone` is the submission as the POST returned it -- the state before we
    touched anything. assert_clone_preserved proved the body we SENT matched
    it outside the mutable set; this proves the round trip did, because a PUT
    that returns 200 and quietly drops the hand-entered listing is the failure
    that matters most, and a printed image count is not a guard against it.
    `notes` is the What's New that was sent, per language, and each listing's
    releaseNotes must have come back as exactly that text.
    """
    fresh = api("GET", f"/applications/{STORE_ID}/submissions/{sid}", token())
    problems = []
    if fresh.get("status") != "PendingCommit":
        problems.append(f"status is {fresh.get('status')!r}, expected 'PendingCommit'")
    if fresh.get("targetPublishMode") != publish_mode:
        problems.append(f"targetPublishMode is {fresh.get('targetPublishMode')!r}, expected {publish_mode!r}")
    ro = ((fresh.get("packageDeliveryOptions") or {}).get("packageRollout") or {})
    if ro.get("isPackageRollout"):
        problems.append("a package rollout is enabled")
    # A new entry is identified by fileName and fileStatus ONLY. Its
    # version and architecture come back null until the commit, because
    # Partner Center reads them out of the MSIX manifest rather than
    # taking them from us -- which is also why they are not sent. Asserting
    # on them here fails a submission that is in fact correct.
    names = [p.get("fileName") for p in (fresh.get("applicationPackages") or [])]
    for n in sorted({n for n in names if names.count(n) > 1}):
        # Keyed by fileName below, so two entries under one name would collapse
        # to whichever the server listed last and the check would inspect the
        # wrong one.
        problems.append(f"{n!r} appears {names.count(n)} times in the stored package list")
    stored = {p.get("fileName"): p.get("fileStatus")
              for p in (fresh.get("applicationPackages") or [])}
    for p in pkgs:
        if p["fileName"] not in stored:
            problems.append(f"{p['fileName']} is not in the stored package list")
        elif stored[p["fileName"]] != "PendingUpload":
            problems.append(f"{p['fileName']} is {stored[p['fileName']]!r}, expected 'PendingUpload'")
    # The previously published package must still be there, untouched:
    # it is the cheap rollback, and losing it would leave no certified
    # x64 package behind if 1.2.13 turns out bad.
    kept = [f for f, st in stored.items() if st == "Uploaded"]
    if not kept:
        problems.append("no previously published package survived; the rollback path is gone")
    for lang, val in (fresh.get("listings") or {}).items():
        imgs = ((val or {}).get("baseListing") or {}).get("images") or []
        bad = [i for i in imgs if i.get("fileStatus") != "Uploaded"]
        if bad:
            problems.append(f"{lang}: {len(bad)} image(s) not 'Uploaded'")
    for lang, text in sorted(notes.items()):
        got = _release_notes_in(fresh.get("listings"), lang)
        if got == text:
            continue
        if not isinstance(got, str):
            problems.append(f"{lang}: releaseNotes came back as "
                            f"{'nothing' if got is None else type(got).__name__}, "
                            f"not the text of the release's whats-new file")
        elif got.replace("\r\n", "\n") == text:
            # Named apart because it is the one rewrite a server could
            # plausibly make on its own. If Partner Center makes it, the
            # refusal says so at once, and the fix is to record it here with
            # the date it was seen, as SERVER_OWNED records its keys, rather
            # than to loosen the comparison.
            problems.append(f"{lang}: releaseNotes came back with CRLF line breaks where the file has LF")
        else:
            problems.append(f"{lang}: releaseNotes came back as other text than the release's whats-new "
                            f"file ({utf16_length(got)} characters against its {utf16_length(text)})")
    # Everything we did not mean to change must have come back as it went.
    # Strict on purpose: a spurious refusal here costs an investigation with
    # nothing public changed, and the opposite mistake costs the listing.
    # If a release ever trips on a key the server rewrites benignly, name it
    # in SERVER_OWNED with the reason, rather than loosening the comparison.
    for k in sorted((set(clone) | set(fresh)) - MUTABLE - NOT_PAYLOAD - SERVER_OWNED):
        if json.dumps(_comparable(k, clone.get(k)), sort_keys=True) != \
                json.dumps(_comparable(k, fresh.get(k)), sort_keys=True):
            problems.append(f"{k!r} came back from the server different from the clone")
    if problems:
        raise SystemExit("REFUSING TO PROCEED:\n  - " + "\n  - ".join(problems))

    for p in (fresh.get("applicationPackages") or []):
        print(f"    {str(p.get('fileName')):32} {str(p.get('fileStatus')):14} "
              f"{p.get('version') or '(set at commit)'}")
    li = (fresh.get("listings") or {}).get("en-gb", {}).get("baseListing", {})
    print(f"    listing: {len(li.get('images') or [])} images, "
          f"{len(li.get('description') or '')} chars of description, identical to the clone "
          f"but for What's New, which is the file's {utf16_length(li.get('releaseNotes') or '')} characters")
    print(f"    targetPublishMode {fresh.get('targetPublishMode')}   status {fresh.get('status')}")


# --------------------------------------------------------- commit and abort

def cmd_commit():
    s = load_state()
    sid = s.get("submissionId")
    if not sid:
        raise SystemExit("no submission recorded; run create first")
    if s.get("committed"):
        raise SystemExit(f"{sid} was already committed")
    # Commit is the irreversible step, and it takes the server's word that the
    # staged submission is what we meant only through verify_staged. create
    # runs that; so does `verify <dir>`, which also proves the bytes on disk are
    # the bytes that were uploaded. Neither having run is not a state to commit
    # from, and nor is the last of them having refused: each clears the flag
    # before it checks and sets it only on a pass.
    if not s.get("verified"):
        raise SystemExit(f"{sid} has not been verified against the server since it was staged; "
                         f"run: msstore/submit.py verify <dir>")
    tok = token()
    fresh = api("GET", f"/applications/{STORE_ID}/submissions/{sid}", tok)
    if fresh.get("status") != "PendingCommit":
        raise SystemExit(f"{sid} is {fresh.get('status')!r}, not 'PendingCommit'")
    mode = fresh.get("targetPublishMode")
    print(f"committing {sid} with targetPublishMode {mode}")
    if mode == "Immediate":
        print("  -> this publishes to the public automatically when certification passes.")
    r = api("POST", f"/applications/{STORE_ID}/submissions/{sid}/commit", tok)
    save_state(committed=True, commitStatus=r.get("status"))
    print(f"  {r.get('status')}")


def cmd_poll():
    s = load_state()
    sid = s.get("submissionId")
    if not sid:
        raise SystemExit("no submission recorded")
    # Polls until the commit has been TAKEN -- until the status leaves
    # CommitStarted -- and then says where it landed. It does not sit through
    # certification: that is up to three business days, and release-status.py
    # answers "where is it now" at any point. What it must never do is exit 0
    # on a failure. CommitFailed is a terminal state, and the earlier version
    # printed it and returned as if it were progress.
    tok, n = token(), 0
    while True:
        r = api("GET", f"/applications/{STORE_ID}/submissions/{sid}/status", tok)
        st = r.get("status") or ""
        print(f"  {time.strftime('%H:%M:%S')}  {st}")
        det = r.get("statusDetails") or {}
        for e in det.get("errors") or []:
            print(f"    ERROR {e.get('code')}: {redact_text(str(e.get('details')))}")
        for w in det.get("warnings") or []:
            print(f"    warning {w.get('code')}: {redact_text(str(w.get('details')))}")
        if st != "CommitStarted":
            save_state(commitStatus=st)
            if st.endswith("Failed") or st == "Canceled":
                print(f"  the commit did not go through: {st}")
                return 1
            if st in ("PreProcessing", "Certification", "Release", "Publishing",
                      "PendingPublication", "Published"):
                print(f"  the commit was taken; it is now {st}. Certification takes up to three "
                      f"business days -- scripts/release-status.py shows where it is.")
                return 0
            print(f"  unexpected status {st!r}; treating it as not proven")
            return 1
        n += 1
        if n % 10 == 0:
            tok = token()
        time.sleep(60)


def cmd_abort():
    s = load_state()
    sid = s.get("submissionId")
    if not sid:
        raise SystemExit("no submission recorded")
    # Abort is for a DRAFT. Once commit has run, the submission is in
    # certification and this file's own docstring calls that the point of no
    # return; honouring it locally is cheaper than finding out whether the
    # server would. A committed submission stays as pendingApplicationSubmission
    # until it publishes, so the ownership check below would let the DELETE
    # through on its own -- and the runbook's own recovery for a 409 on
    # `create` sends the operator straight here. Withdrawing an in-flight
    # release is a Partner Center act, taken with the reason in front of you.
    if s.get("committed"):
        raise SystemExit(f"{sid} was committed; a committed submission is not a draft. "
                         f"If it really must be withdrawn, cancel it in Partner Center.")
    tok = token()
    app = api("GET", f"/applications/{STORE_ID}", tok)
    pending = (app.get("pendingApplicationSubmission") or {}).get("id")
    if pending != sid:
        raise SystemExit(f"the pending submission is {pending}, not the {sid} this run created; "
                         f"refusing to delete someone else's work")
    # And ask the server what state it is really in: the local flag is written
    # after the commit call returns, so a crash between the two leaves it
    # false for a submission that is already in flight.
    fresh = api("GET", f"/applications/{STORE_ID}/submissions/{sid}", tok)
    st = fresh.get("status") or ""
    if not (st == "PendingCommit" or st.endswith("Failed")):
        raise SystemExit(f"{sid} is {st!r} on the server, not a draft; refusing to delete it")
    status, _h, raw = http("DELETE", f"{API}/applications/{STORE_ID}/submissions/{sid}", tok)
    if status not in (200, 204):
        raise SystemExit(f"delete returned {status}")
    after = api("GET", f"/applications/{STORE_ID}", token())
    if after.get("pendingApplicationSubmission"):
        raise SystemExit("delete reported success but a pending submission is still there")
    print(f"deleted {sid}; lastPublished is still "
          f"{(after.get('lastPublishedApplicationSubmission') or {}).get('id')}")
    save_state(submissionId=None, committed=False, verified=False)


def main(argv):
    if len(argv) < 2:
        print(__doc__.strip(), file=sys.stderr)
        return 2
    cmd = argv[1]
    mode = "Immediate"
    if "--manual" in argv:
        mode = "Manual"
        argv = [a for a in argv if a != "--manual"]
    if cmd == "preflight":
        cmd_preflight(argv[2]); return 0
    if cmd == "verify":
        s = load_state()
        if not s.get("submissionId"):
            raise SystemExit("no submission recorded; run create first")
        # Cleared before anything is checked, so commit follows the latest
        # verdict: a verify that refuses must not leave the verified that a
        # passing create wrote before it.
        save_state(verified=False)
        # Compare against the clone create saved, and hold the submission to
        # the publish mode create used -- not whatever this invocation's argv
        # happens to say, which is how a verify could bless the wrong mode.
        clone_path = os.path.join(STATE_DIR, f"clone-{s['submissionId']}.json")
        try:
            with open(clone_path) as f:
                clone = json.load(f)
        except FileNotFoundError:
            raise SystemExit(f"{clone_path} is missing; verify has nothing to compare the server against")
        pkgs = read_packages(argv[2])
        # The bytes on disk must be the bytes create uploaded. The digests were
        # recorded for exactly this, and a directory rebuilt since create would
        # otherwise pass verify unchanged -- the server has the old bytes and
        # the operator is looking at new ones.
        recorded = {p["fileName"]: p["sha256"] for p in (s.get("packages") or [])}
        for p in pkgs:
            if recorded.get(p["fileName"]) != p["sha256"]:
                raise SystemExit(f"{p['fileName']} on disk (sha256 {p['sha256'][:16]}) is not the file "
                                 f"create uploaded ({(recorded.get(p['fileName']) or '?')[:16]}); "
                                 f"the server holds different bytes from the ones you are looking at")
        # The same for the release notes: the server is held to the text create
        # sent, and a file edited since then must not be what it is compared
        # against, or a verify would pass text nobody is looking at.
        notes = read_release_notes(desktop_version())
        sent = s.get("releaseNotes") or {}
        for lang, text in sorted(notes.items()):
            if sent.get(lang) != notes_digest(text):
                raise SystemExit(f"{os.path.relpath(release_notes_path(lang, desktop_version()), REPO)} "
                                 f"is not the text create sent for {lang}; the server holds different "
                                 f"release notes from the ones you are looking at")
        verify_staged(s["submissionId"], pkgs, s.get("publishMode") or mode, clone, notes)
        save_state(verified=True)
        return 0
    if cmd == "create":
        cmd_create(argv[2], mode); return 0
    if cmd == "commit":
        cmd_commit(); return 0
    if cmd == "poll":
        return cmd_poll()
    if cmd == "abort":
        cmd_abort(); return 0
    print(__doc__.strip(), file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
