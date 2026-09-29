#!/usr/bin/env python3
"""Validate and optionally upload an App Store screenshot set.

The default mode is a read-only remote preflight: the local upload-ready set
is validated first, then the version record, its en-GB localization and the
screenshot sets it holds are resolved, and a plan is printed per display type.
No POST, PATCH, DELETE or upload is possible without both ``--write`` and an
exact ``--confirm-version``. Use ``--local-only`` for a validation pass that
makes no network request.

The set on disk is build/appstore/screenshots-ready-<version>/en-GB, or
``--set-dir``. The IOS platform (the default) takes the PNGs in that directory
as the 6.9-inch iPhone set and those in its ipad13/ directory as the 13-inch
iPad set; ``--platform MAC_OS`` takes mac/ as the Mac set. Files go up in
sorted name order, which is what their numbers are for. Each platform's
version string comes from its own ledger, as push-metadata reads it.

A write replaces each set: every image the set holds is deleted, each local
file is reserved, uploaded in the byte ranges App Store Connect names,
committed with its MD5, the set is ordered, and every image is polled until
assetDeliveryState is COMPLETE, the only state that means the image will
show (an off-size or alpha-carrying upload is accepted and then sits at
FAILED). The sets are then read back and compared with the files, and the run
succeeds only when count, order and checksums all agree. ``--keep-existing``
appends after the images a set already holds instead, and refuses a set that
would exceed the ten images a set may carry.

This tool never selects a build, creates a version, writes text metadata or
submits anything for review. The version record must already exist;
submit-version.py --write creates it.
"""

import argparse
import collections
import hashlib
import importlib.util
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
ASC_DIR = os.path.join(REPO, "build", "appstore")
PREFLIGHT = os.path.join(HERE, "preflight.py")
APP = "6784567351"
LOCALE = "en-GB"

# Each platform packages its own version string; the Mac record is resolved
# from the Mac ledger, never from the mobile one.
PLATFORM_VERSION_CONFIG = {
    "IOS": os.path.join("cmd", "mobile", "FyneApp.toml"),
    "MAC_OS": os.path.join("cmd", "bibletext", "FyneApp.toml"),
}

# Which directory of the set feeds which display type. The identifiers are
# App Store Connect's ScreenshotDisplayType cases for the 6.9-inch iPhone, the
# 13-inch iPad and the Mac; the live sets report the same three. Smaller
# devices scale down from these, so no other set is uploaded.
DISPLAY_TYPES = {
    "IOS": (("", "APP_IPHONE_67"), ("ipad13", "APP_IPAD_PRO_3GEN_129")),
    "MAC_OS": (("mac", "APP_DESKTOP"),),
}

# App Store Connect holds at most this many screenshots in one set.
SET_CAPACITY = 10

# Screenshots may be changed only while App Store Connect considers the
# version editable: while it is being prepared, and after a rejection or an
# invalid binary hands it back. READY_FOR_REVIEW is a version already added
# to a review submission that has not been sent; App Store Connect still
# takes text there but no longer takes images or previews, and everything
# from WAITING_FOR_REVIEW on belongs to Apple or to the store.
EDITABLE_STATES = {
    "PREPARE_FOR_SUBMISSION",
    "DEVELOPER_REJECTED",
    "REJECTED",
    "METADATA_REJECTED",
    "INVALID_BINARY",
}

# Processing an upload takes seconds to a few minutes. The poll is bounded so
# an image that never arrives ends the run with a message rather than a hang.
POLL_INTERVAL = 5.0
POLL_LIMIT = 600.0

# Replaced by the tests so a poll runs without waiting.
sleep = time.sleep
monotonic = time.monotonic

LocalImage = collections.namedtuple(
    "LocalImage", "name path size checksum width height")
LocalSet = collections.namedtuple("LocalSet", "display_type directory images")
SetPlan = collections.namedtuple(
    "SetPlan", "display_type directory set_id existing deletions kept images unchanged")


# Deliberately no environment override: a forgotten ASC_VERSION must never
# redirect a write toward a historical version record.
def configured_version(platform):
    config = os.path.join(REPO, PLATFORM_VERSION_CONFIG[platform])
    with open(config, encoding="utf-8") as handle:
        for line in handle:
            if line.strip().startswith("Version"):
                return line.split("=", 1)[1].strip().strip('"')
    raise SystemExit(f"Version is missing from {config}")


def load_preflight():
    """preflight.py's PNG inspector and accepted sizes.

    Shared rather than copied, so the preflight that gates a submission and
    the uploader that feeds it cannot disagree about what Apple accepts.
    """
    spec = importlib.util.spec_from_file_location("appstore_preflight", PREFLIGHT)
    if spec is None or spec.loader is None:
        raise SystemExit(f"cannot load {PREFLIGHT}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def md5_hex(data):
    """App Store Connect's sourceFileChecksum is the file's MD5, hex-encoded."""
    try:
        digest = hashlib.md5(usedforsecurity=False)
    except TypeError:
        digest = hashlib.md5()
    digest.update(data)
    return digest.hexdigest()


def read_local_set(platform, set_dir, preflight):
    """Every display type's files, validated, in upload order.

    Any problem is fatal here, before authentication or network access: a
    directory that is missing or empty, a file that is not a PNG, a size Apple
    does not accept, an alpha channel, or more files than a set can hold.
    """
    accepted = preflight.ACCEPTED_SCREENSHOT_SIZES[platform]
    sizes = ", ".join(f"{w}x{h}" for w, h in sorted(accepted))
    sets, problems = [], []
    for subdirectory, display_type in DISPLAY_TYPES[platform]:
        directory = os.path.join(set_dir, subdirectory) if subdirectory else set_dir
        if not os.path.isdir(directory):
            problems.append(f"{display_type}: {directory} is missing")
            continue
        names = sorted(
            name for name in os.listdir(directory)
            if name.lower().endswith(".png") and os.path.isfile(os.path.join(directory, name))
        )
        if not names:
            problems.append(f"{display_type}: {directory} holds no PNG")
            continue
        if len(names) > SET_CAPACITY:
            problems.append(
                f"{display_type}: {directory} holds {len(names)} PNGs; "
                f"a set may hold at most {SET_CAPACITY}"
            )
        images = []
        for name in names:
            path = os.path.join(directory, name)
            with open(path, "rb") as handle:
                data = handle.read()
            info = preflight.png_inspect(path)
            if info is None:
                problems.append(f"{display_type}: {name} is not a PNG")
                continue
            width, height, has_alpha = info
            if (width, height) not in accepted:
                problems.append(
                    f"{display_type}: {name} is {width}x{height}; {platform} accepts "
                    f"only {sizes}. The upload would sit at assetDeliveryState FAILED."
                )
            if has_alpha:
                problems.append(
                    f"{display_type}: {name} carries an alpha channel; "
                    "App Store Connect refuses it"
                )
            images.append(LocalImage(name, path, len(data), md5_hex(data), width, height))
        sets.append(LocalSet(display_type, directory, images))
    if problems:
        raise SystemExit(
            "local screenshot set is not upload-ready:\n"
            + "\n".join(f"  - {problem}" for problem in problems)
        )
    return sets


class Client:
    """Every remote call, behind two methods.

    ``api`` is an App Store Connect request carrying asc.py's token.
    ``upload`` is the one call that is not: the PUT of a byte range to the
    pre-signed URL an upload operation names, with exactly the headers it
    lists and no Authorization header. The URL is its own credential and the
    upload host is not the API.
    """

    def __init__(self, asc_module):
        self._asc = asc_module

    def api(self, method, path, body=None):
        return self._asc.request(method, path, body)

    def upload(self, operation, chunk):
        headers = {
            header["name"]: header["value"]
            for header in operation.get("requestHeaders") or []
        }
        request = urllib.request.Request(
            operation["url"], data=chunk, method=operation.get("method") or "PUT",
            headers=headers,
        )
        try:
            with urllib.request.urlopen(request, timeout=300) as response:
                return response.status, response.read()
        except urllib.error.HTTPError as error:
            return error.code, error.read()


def connect():
    """The real client, imported only after the entirely local preflight.

    Importing asc.py makes no request; every call after this is visibly GET
    or, behind --write, POST, PUT, PATCH or DELETE.
    """
    if not os.path.isfile(os.path.join(ASC_DIR, "asc.py")):
        raise SystemExit(
            "missing build/appstore/asc.py (local App Store tooling is ignored; "
            "see docs/APP_STORE_SUBMISSION.md)"
        )
    sys.path.insert(0, ASC_DIR)
    import asc  # pylint: disable=import-outside-toplevel
    return Client(asc)


def parse_args(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--write", action="store_true",
                      help="replace (or, with --keep-existing, extend) the sets "
                           "after all preflight checks")
    mode.add_argument("--local-only", action="store_true",
                      help="validate the local files without App Store Connect access")
    parser.add_argument(
        "--confirm-version", metavar="VERSION",
        help="required with --write; must exactly equal the configured version",
    )
    parser.add_argument(
        "--platform", choices=sorted(PLATFORM_VERSION_CONFIG), default="IOS",
        help="App Store platform to target (default: IOS); the version string "
             "comes from that platform's FyneApp.toml",
    )
    parser.add_argument(
        "--set-dir", metavar="DIR",
        help="the upload-ready set (default: build/appstore/screenshots-ready-"
             "<version>/en-GB)",
    )
    parser.add_argument(
        "--keep-existing", action="store_true",
        help="append after the images each set already holds instead of "
             f"replacing them; refused when a set would exceed {SET_CAPACITY}",
    )
    args = parser.parse_args(argv)
    version = configured_version(args.platform)
    if args.write and args.confirm_version != version:
        parser.error(
            f"--write requires --confirm-version {version}; no remote request was made"
        )
    if args.confirm_version and not args.write:
        parser.error("--confirm-version is meaningful only with --write")
    return args, version


def show_error(label, status, body):
    print(f"[ERROR {status}] {label}")
    rendered = json.dumps(body) if isinstance(body, (dict, list)) else str(body)
    print("   ", rendered[:800])


def get_data(client, path, label):
    status, body = client.api("GET", path)
    if not 200 <= status < 300 or not isinstance(body, dict):
        show_error(label, status, body)
        raise SystemExit(1)
    data = body.get("data")
    if not isinstance(data, list):
        raise SystemExit(f"{label}: response did not contain a data list")
    return data


def get_record(client, path, label):
    status, body = client.api("GET", path)
    if not 200 <= status < 300 or not isinstance(body, dict):
        show_error(label, status, body)
        raise SystemExit(1)
    data = body.get("data")
    if not isinstance(data, dict):
        raise SystemExit(f"{label}: response did not contain a data record")
    return data


def written(client, method, path, body, label):
    """A write that must succeed; anything else stops the run where it is."""
    status, response = client.api(method, path, body)
    if not 200 <= status < 300:
        show_error(label, status, response)
        raise SystemExit("write stopped; inspect App Store Connect before retrying")
    return response


def resolve_version(client, platform, version):
    query = urllib.parse.urlencode({
        "filter[platform]": platform,
        "filter[versionString]": version,
        "limit": "10",
    })
    records = [
        item for item in get_data(client, f"/v1/apps/{APP}/appStoreVersions?{query}",
                                  "resolve version")
        if item.get("attributes", {}).get("versionString") == version
    ]
    if not records:
        raise SystemExit(
            f"no {platform} App Store version {version} exists, so there is nothing "
            f"to upload to. submit-version.py --platform {platform} --write "
            f"--confirm-version {version} creates the record (and attaches the "
            "build); run this tool after it."
        )
    if len(records) != 1:
        raise SystemExit(
            f"expected exactly one {platform} App Store version {version!r}; "
            f"found {len(records)}"
        )
    return records[0]


def version_state(record):
    """The version's state, read from appVersionState before appStoreState.

    appStoreState is the older attribute, deprecated in favour of
    appVersionState. Records carry both today, under different names for the
    released states (READY_FOR_SALE beside READY_FOR_DISTRIBUTION) and the
    same names for every editable one. A record with neither reads as
    UNKNOWN, which no write accepts.
    """
    attributes = record.get("attributes", {})
    return attributes.get("appVersionState") or attributes.get("appStoreState") or "UNKNOWN"


def resolve_localization(client, version_id):
    localizations = get_data(
        client, f"/v1/appStoreVersions/{version_id}/appStoreVersionLocalizations?limit=200",
        "resolve version localization",
    )
    matches = [item for item in localizations
               if item.get("attributes", {}).get("locale") == LOCALE]
    if len(matches) != 1:
        raise SystemExit(
            f"expected exactly one {LOCALE} version localization; found {len(matches)}"
        )
    return matches[0]


def set_screenshots(client, set_id):
    return get_data(client, f"/v1/appScreenshotSets/{set_id}/appScreenshots?limit=200",
                    "list screenshots")


def remote_sets(client, localization_id):
    """{display type: (set id, its screenshot records in order)}."""
    result = {}
    sets = get_data(
        client, f"/v1/appStoreVersionLocalizations/{localization_id}/appScreenshotSets?limit=200",
        "list screenshot sets",
    )
    for item in sets:
        display_type = item.get("attributes", {}).get("screenshotDisplayType")
        result[display_type] = (item["id"], set_screenshots(client, item["id"]))
    return result


def summary(record):
    """(fileName, fileSize, sourceFileChecksum, delivery state) of a record."""
    attributes = record.get("attributes", {})
    state = (attributes.get("assetDeliveryState") or {}).get("state") or "UNKNOWN"
    return (attributes.get("fileName"), attributes.get("fileSize"),
            attributes.get("sourceFileChecksum"), state)


def build_plan(local_sets, remote, keep_existing):
    """Per display type: what exists, what would be deleted, what goes up."""
    plan = []
    for local in local_sets:
        set_id, existing = remote.get(local.display_type, (None, []))
        if keep_existing:
            total = len(existing) + len(local.images)
            if total > SET_CAPACITY:
                raise SystemExit(
                    f"--keep-existing would leave {local.display_type} with {total} "
                    f"images; a set may hold at most {SET_CAPACITY}"
                )
            deletions, kept = [], list(existing)
        else:
            deletions, kept = list(existing), []
        # A set that already holds exactly these files, all delivered, has
        # nothing to gain from being deleted and uploaded again.
        unchanged = (
            not keep_existing
            and [summary(record)[::2] for record in existing]
            == [(image.name, image.checksum) for image in local.images]
            and all(summary(record)[3] == "COMPLETE" for record in existing)
        )
        plan.append(SetPlan(local.display_type, local.directory, set_id, existing,
                            deletions, kept, local.images, unchanged))
    return plan


def print_plan(plan):
    for item in plan:
        print(f"\n{item.display_type}  <-  {item.directory}")
        if item.set_id is None:
            print("  remote set: none; it would be created")
        else:
            print(f"  remote set {item.set_id}: {len(item.existing)} images")
            for number, record in enumerate(item.existing, 1):
                name, size, checksum, state = summary(record)
                print(f"    {number:2}. {name}  {size} B  md5 {checksum}  {state}")
        if item.unchanged:
            print("  already holds these files in this order, all COMPLETE; nothing to do")
            continue
        if item.deletions:
            print(f"  would delete all {len(item.deletions)} existing images")
        elif item.kept:
            print(f"  would keep the {len(item.kept)} existing images (--keep-existing)")
        print("  would upload, in order:")
        for number, image in enumerate(item.images, len(item.kept) + 1):
            print(f"    {number:2}. {image.name}  {image.size} B  "
                  f"{image.width}x{image.height}  md5 {image.checksum}")


def create_set(client, localization_id, display_type):
    response = written(client, "POST", "/v1/appScreenshotSets", {
        "data": {
            "type": "appScreenshotSets",
            "attributes": {"screenshotDisplayType": display_type},
            "relationships": {"appStoreVersionLocalization": {
                "data": {"type": "appStoreVersionLocalizations", "id": localization_id}}},
        }
    }, f"create {display_type} set")
    set_id = (response.get("data") or {}).get("id") if isinstance(response, dict) else None
    if not set_id:
        raise SystemExit(f"create {display_type} set: response carried no id")
    print(f"[OK] created {display_type} set {set_id}")
    return set_id


def upload_image(client, set_id, image):
    """Reserve, upload every part, commit. Returns the new screenshot's id."""
    with open(image.path, "rb") as handle:
        data = handle.read()
    if len(data) != image.size or md5_hex(data) != image.checksum:
        raise SystemExit(f"{image.name} changed on disk since it was validated")
    response = written(client, "POST", "/v1/appScreenshots", {
        "data": {
            "type": "appScreenshots",
            "attributes": {"fileName": image.name, "fileSize": image.size},
            "relationships": {"appScreenshotSet": {
                "data": {"type": "appScreenshotSets", "id": set_id}}},
        }
    }, f"reserve {image.name}")
    record = (response.get("data") or {}) if isinstance(response, dict) else {}
    screenshot_id = record.get("id")
    operations = record.get("attributes", {}).get("uploadOperations") or []
    if not screenshot_id or not operations:
        raise SystemExit(f"reserve {image.name}: response carried no id or no upload operations")
    for operation in operations:
        offset = int(operation.get("offset") or 0)
        length = int(operation.get("length") or 0)
        chunk = data[offset:offset + length]
        if len(chunk) != length:
            raise SystemExit(
                f"{image.name}: upload operation wants bytes {offset}..{offset + length} "
                f"of a {len(data)}-byte file"
            )
        status, body = client.upload(operation, chunk)
        if not 200 <= status < 300:
            show_error(f"upload {image.name} bytes {offset}..{offset + length}", status, body)
            raise SystemExit("write stopped; inspect App Store Connect before retrying")
    written(client, "PATCH", f"/v1/appScreenshots/{screenshot_id}", {
        "data": {
            "type": "appScreenshots",
            "id": screenshot_id,
            "attributes": {"uploaded": True, "sourceFileChecksum": image.checksum},
        }
    }, f"commit {image.name}")
    print(f"[OK] uploaded {image.name} in {len(operations)} part(s), md5 {image.checksum}")
    return screenshot_id


def wait_for_delivery(client, display_type, screenshot_ids):
    """Poll until every id is COMPLETE; FAILED or the time limit ends the run."""
    pending = list(screenshot_ids)
    deadline = monotonic() + POLL_LIMIT
    while pending:
        remaining = []
        for screenshot_id in pending:
            record = get_record(client, f"/v1/appScreenshots/{screenshot_id}",
                                f"poll {screenshot_id}")
            name, _size, _checksum, state = summary(record)
            if state == "COMPLETE":
                print(f"[OK] {name} COMPLETE")
                continue
            if state == "FAILED":
                delivery = record.get("attributes", {}).get("assetDeliveryState") or {}
                errors = "; ".join(
                    f"{error.get('code')}: {error.get('description')}"
                    for error in delivery.get("errors") or []
                ) or "no error detail"
                raise SystemExit(
                    f"{display_type} {name}: assetDeliveryState FAILED ({errors}). "
                    "The set is now partial; inspect App Store Connect before retrying."
                )
            remaining.append(screenshot_id)
        pending = remaining
        if pending:
            if monotonic() >= deadline:
                raise SystemExit(
                    f"{display_type}: {len(pending)} image(s) had not reached COMPLETE "
                    f"after {POLL_LIMIT:.0f} s; giving up. Re-run the read-only "
                    "preflight later to see where they got to."
                )
            sleep(POLL_INTERVAL)


def write_set(client, localization_id, item):
    """Replace or extend one set. Returns (set id, expected ids in order)."""
    set_id = item.set_id or create_set(client, localization_id, item.display_type)
    for record in item.deletions:
        written(client, "DELETE", f"/v1/appScreenshots/{record['id']}", None,
                f"delete {summary(record)[0]}")
        print(f"[OK] deleted {summary(record)[0]}")
    created = [upload_image(client, set_id, image) for image in item.images]
    order = [record["id"] for record in item.kept] + created
    written(client, "PATCH", f"/v1/appScreenshotSets/{set_id}/relationships/appScreenshots",
            {"data": [{"type": "appScreenshots", "id": screenshot_id}
                      for screenshot_id in order]},
            f"order {item.display_type}")
    print(f"[OK] ordered {item.display_type}: {len(order)} images")
    wait_for_delivery(client, item.display_type, created)
    return set_id, order


def verify_set(client, item, set_id, order):
    """Read the set back and hold it to the files: count, order, checksums."""
    expected = [(record["id"],) + summary(record)[::2] for record in item.kept]
    expected += [(screenshot_id, image.name, image.checksum)
                 for screenshot_id, image in zip(order[len(item.kept):], item.images)]
    records = set_screenshots(client, set_id)
    print(f"\n{item.display_type} read back: {len(records)} images")
    problems = []
    if len(records) != len(expected):
        problems.append(f"holds {len(records)} images, expected {len(expected)}")
    for number, (want, record) in enumerate(zip(expected, records), 1):
        name, _size, checksum, state = summary(record)
        print(f"    {number:2}. {name}  md5 {checksum}  {state}")
        if record.get("id") != want[0] or name != want[1]:
            problems.append(f"position {number} is {name}, expected {want[1]}")
        if checksum != want[2]:
            problems.append(f"{name}: checksum {checksum}, expected {want[2]}")
        if state != "COMPLETE":
            problems.append(f"{name}: assetDeliveryState {state}")
    if problems:
        raise SystemExit(
            f"read-back mismatch for {item.display_type}:\n"
            + "\n".join(f"  - {problem}" for problem in problems)
        )
    print(f"[OK] read-back {item.display_type}")


def main(argv=None):
    args, version = parse_args(argv)
    platform = args.platform
    set_dir = os.path.abspath(args.set_dir) if args.set_dir else os.path.join(
        ASC_DIR, f"screenshots-ready-{version}", LOCALE)
    local_sets = read_local_set(platform, set_dir, load_preflight())
    total = sum(len(local.images) for local in local_sets)
    print(f"local screenshot set: OK ({LOCALE}, {platform} version {version}, "
          f"{total} PNGs under {set_dir})")
    for local in local_sets:
        print(f"  {local.display_type}: {len(local.images)} from {local.directory}")
    if args.local_only:
        print("local-only mode: no App Store Connect request made")
        return 0

    client = connect()
    version_record = resolve_version(client, platform, version)
    version_id = version_record["id"]
    state = version_state(version_record)
    localization = resolve_localization(client, version_id)
    remote = remote_sets(client, localization["id"])
    print(f"target: app {APP}, {platform} {version}, {LOCALE}, state {state}")
    plan = build_plan(local_sets, remote, args.keep_existing)
    print_plan(plan)
    pending = [item for item in plan if not item.unchanged]

    if not args.write:
        print("\nDRY RUN: nothing deleted, uploaded or reordered. Re-run with --write and "
              f"--confirm-version {version} only after reviewing this plan.")
        return 0
    if state not in EDITABLE_STATES:
        raise SystemExit(
            f"refusing to write version in {state}; expected an editable "
            f"preparation state ({', '.join(sorted(EDITABLE_STATES))})"
        )
    if not pending:
        print("\nEvery set already holds its files; nothing written.")
        return 0

    results = []
    for item in pending:
        print(f"\n{item.display_type}")
        set_id, order = write_set(client, localization["id"], item)
        results.append((item, set_id, order))
    for item, set_id, order in results:
        verify_set(client, item, set_id, order)
    print("\nevery set read back as uploaded: count, order and checksums agree, all COMPLETE.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
