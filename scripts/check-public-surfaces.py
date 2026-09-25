#!/usr/bin/env python3
"""Hold the public pages to what the repository actually ships.

Four kinds of drift have each reached a published page, and none of them is
the sort a reader of the diff notices, because in every case the stale text
was true when it was written:

  * a release gained an artifact and nothing linked to it. The AppImage
    shipped in 1.2.10 and neither the download page nor the README named it;
    the release workflow's own comment says the download page links assets by
    name, which is exactly what makes a missing name invisible.
  * two copies of the same instruction drifted apart. The Linux build
    dependencies are written out in the README, in CONTRIBUTING and in the CI
    workflow, and CONTRIBUTING lost the ALSA headers, so a contributor's
    first build failed on audio while CI stayed green.
  * prose counted something the tree can count itself. The README said four
    programs live under cmd/ when there were six.
  * an instruction outlived the system it described. The release notes, the
    download page, the README and the Mac App Store notes all told a Mac
    reader to right-click the unsigned download and choose Open, an override
    macOS 15 removed. The rule requires the route that replaced it and
    rejects the retired wording, since text pasted from an old release would
    otherwise sit beside the new steps and pass.

Each rule below is mechanical: it compares a public page against the thing it
claims to describe. Nothing here judges wording, and none of it can tell that
a sentence is merely old — that is what review is for.

Every rule also refuses to pass when it cannot see its own inputs. A checker
that quietly reads nothing is worse than no checker, because the green tick
is then evidence of nothing; each rule below fails loudly if the shape it
parses disappears.

Like every checker in this repository it self-tests first: each rule, and each
blindness guard, is run against a synthetic violation and must fail, because a
checker that cannot fail proves nothing when it passes.
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

RELEASE_WORKFLOW = ".github/workflows/release.yml"
STORE_IDENTITY = "msstore/identity.json"
DOWNLOAD_PAGE = "docs/index.html"
READ_ME = "README.md"
CONTRIBUTING = "CONTRIBUTING.md"
CI_WORKFLOW = ".github/workflows/ci.yml"
PRODUCT_CONFIG = "config/product.json"
MAC_STORE_GUIDE = "docs/MAC_APP_STORE.md"

# The four places that tell a Mac reader how to open the unsigned direct
# download the first time. The release notes are a printf in the workflow, so
# no copy can include another, and the wording is free to differ; the routes
# are not.
MAC_OPEN_SURFACES = (RELEASE_WORKFLOW, DOWNLOAD_PAGE, READ_ME, MAC_STORE_GUIDE)

# The release notes are the argument of the printf that fills NOTES, and only
# that argument reaches a reader. The rest of the workflow is YAML and shell
# comments, and a comment explaining the macOS note names both routes, so
# searching the whole file would pass on the comment whatever the notes said.
# A shell single-quoted string cannot contain a quote, so the argument ends at
# the first one.
RELEASE_NOTES = re.compile(r"""NOTES="\$\(printf '([^']*)'""")

# Markup comments on the pages are not shown to a reader either, and would
# satisfy the rule the same way a workflow comment did.
MARKUP_COMMENT = re.compile(r"<!--.*?-->", re.DOTALL)

# macOS asks before it first opens an app it cannot verify, and has had two
# ways past the question. The Open Anyway button in the security settings
# works on every macOS the app supports, and is the only way from macOS 15,
# which removed the Finder's Control-click > Open override. The Finder route is
# owed while the floor is below 15 because it reads the same on 12, 13 and 14,
# where the settings route does not: macOS 12 keeps the button in System
# Preferences under Security & Privacy, not in System Settings under Privacy &
# Security, where the pages send a reader on 15 and later.
MAC_FLOOR_KEY = "macMinimumOSVersion"
SETTINGS_ROUTE = "Open Anyway"
FINDER_ROUTE = "Control-click"
FINDER_ROUTE_GONE = 15

# The retired instruction: right-click, then Open, in one sentence. Requiring
# the new routes does not remove it, and a reader on macOS 15 who tries it
# first meets the same refusal. The Finder route is written as Control-click,
# as Apple's guide words it, so this spelling is free to mean only the old
# advice. Open must be the capitalised menu item and not Open Anyway, and the
# sentence may not end in between, so the app's own right-click menu, which
# has no Open item, is never mistaken for it.
STALE_FINDER_ROUTE = re.compile(r"(?i:right[- ]?click)\w*[^.!?]{0,80}?\bOpen\b(?!\s+Anyway)")

# The three places the Linux build dependencies are written out for a reader
# building this project. They are prose in two of them and a workflow step in
# the third, so no file can import the list from another; equality is the only
# thing holding them together. Other apt lines in the repository install
# packaging tools rather than build dependencies, so the rule matches only
# lines carrying this anchor package.
DEPENDENCY_SOURCES = (READ_ME, CONTRIBUTING, CI_WORKFLOW)
DEPENDENCY_ANCHOR = "libgl1-mesa-dev"

# Directory names under cmd/ that are not programs. Go puts both of these
# beside real commands, and neither belongs in a count of programs.
NOT_A_PROGRAM = {"testdata", "internal"}

COUNT_WORDS = {
    "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
    "seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
}

# An asset name may contain dots and dashes but cannot end with one, so a bare
# URL that ends a sentence does not swallow the full stop.
ASSET = r"BibleText-[A-Za-z0-9._+-]*[A-Za-z0-9]"

# Assets a reader is never given a link to. Sidecars are matched by suffix
# rather than by filename because their names are derived from the artifact
# they accompany (scripts/build-appimage.sh builds "$OUT.zsync"), so an
# architecture or a rename must not silently create an unlinkable asset.
SIDECAR_SUFFIXES = {
    ".zsync": (
        "an AppImage update sidecar: update tools fetch it by the URL embedded "
        "in the AppImage, and a reader never downloads it"
    ),
}


# Every public page that points a Windows reader at the Store must use the URL
# the packaging identity owns. A store link is not an asset, so the release
# rules above cannot see it: nothing recomputes it, and a listing that moved or
# an id typed by hand would sit there looking right. This is the cheap half of
# that problem — it cannot know a channel went live and should be linked, only
# that a link which exists names the right product.
STORE_LINK = r"https://apps\.microsoft\.com/detail/[A-Za-z0-9]+"


def store_links(text: str) -> set[str]:
    return set(re.findall(STORE_LINK, text))


def is_sidecar(asset: str) -> bool:
    return any(asset.endswith(suffix) for suffix in SIDECAR_SUFFIXES)


def fold_continuations(text: str) -> str:
    """Join shell line-continuations so a wrapped command reads as one line.

    A step that grows past one long line is normally wrapped, which would
    otherwise hide everything after the first line from a line-oriented scan.
    """
    return re.sub(r"\\\n\s*", " ", text)


def released_assets(text: str) -> tuple[set[str], list[str]]:
    """Every asset name the release attaches to a tag, and any unreadable step.

    Both the automated `gh release upload` steps and the commented sideload
    command count: the APK is uploaded by hand, and a page that links it makes
    the same promise as one that links the tarball.
    """
    names: set[str] = set()
    unreadable: list[str] = []
    # A job that ships one artefact per architecture names it through its
    # matrix, so the literal text carries no asset name at all. Reading the line
    # verbatim would take `BibleText-Windows-${{ matrix.goarch }}.zip` for an
    # asset called "BibleText-Windows" and then declare the real, correct links
    # dead. Expansion is scoped to the job that declares the values, so two jobs
    # using the same key name for different things cannot contaminate each other.
    for block in split_jobs(fold_continuations(text)):
        values = matrix_values(block)
        for line in block.splitlines():
            if "gh release upload" not in line:
                continue
            for candidate in expand_matrix(line, values):
                found = set(re.findall(ASSET, candidate))
                if not found:
                    unreadable.append(line.strip())
                names.update(found)
    return names, unreadable


def split_jobs(text: str) -> list[str]:
    """The workflow's job blocks, so matrix values stay with their own job."""
    return re.split(r"\n  (?=[a-z][a-z0-9-]*:\n)", text)


def matrix_values(block: str) -> dict[str, list[str]]:
    """Every value each matrix key takes in this job."""
    values: dict[str, list[str]] = {}
    in_matrix = False
    for line in block.splitlines():
        if re.match(r"\s*matrix:\s*$", line):
            in_matrix = True
            continue
        if in_matrix:
            # The matrix ends at the next key indented no further than `matrix:`.
            if line.strip() and not line.startswith("        "):
                in_matrix = False
                continue
            m = re.match(r"\s*-?\s*([a-z][a-z0-9_-]*):\s*(\S+)\s*$", line)
            if m:
                values.setdefault(m.group(1), []).append(m.group(2))
    return values


def expand_matrix(line: str, values: dict[str, list[str]]) -> list[str]:
    """One line per combination of the matrix keys it mentions."""
    used = re.findall(r"\$\{\{\s*matrix\.([a-z][a-z0-9_-]*)\s*\}\}", line)
    if not used:
        return [line]
    out = [line]
    for key in dict.fromkeys(used):
        options = values.get(key)
        if not options:
            return [line]  # unknown key: leave it, the caller will report it
        expanded = []
        for candidate in out:
            for value in options:
                expanded.append(re.sub(r"\$\{\{\s*matrix\." + key + r"\s*\}\}", value, candidate))
        out = expanded
    return out


def linked_assets(text: str) -> set[str]:
    return set(re.findall(rf"releases/latest/download/({ASSET})", text))


def apt_packages(text: str) -> set[str]:
    """The build dependencies an apt line asks for, flags and comments off.

    Only lines carrying the anchor package count, so an unrelated apt step in
    the same file cannot change what the rule believes the list to be.
    """
    packages: set[str] = set()
    for match in re.finditer(r"apt(?:-get)? install([^\n#]*)", fold_continuations(text)):
        asked = [token for token in match.group(1).split() if not token.startswith("-")]
        if DEPENDENCY_ANCHOR in asked:
            packages.update(asked)
    return packages


def mac_floor(config: str) -> int | None:
    """The major version of the macOS floor, or None when it cannot be read."""
    try:
        declared = json.loads(config).get(MAC_FLOOR_KEY, "")
    except (json.JSONDecodeError, AttributeError):
        return None
    match = re.fullmatch(r"(\d+)(?:\.\d+)*", str(declared))
    return int(match.group(1)) if match else None


def mac_open_text(surface: str, body: str) -> str | None:
    """What a reader of this surface sees, or None when the notes are not found.

    For the release workflow that is the release notes alone; for a page it is
    the page less its markup comments.
    """
    if surface == RELEASE_WORKFLOW:
        match = RELEASE_NOTES.search(body)
        return match.group(1).replace("\\n", "\n") if match else None
    return MARKUP_COMMENT.sub("", body)


def readme_cmd_tree(text: str) -> tuple[set[str], int | None, bool]:
    """The programs the README's tree lists under cmd/, and the count it states.

    Returns the entries, the stated count, and whether a cmd/ line was found at
    all — an unparsed tree must be distinguishable from a satisfied rule. Only
    a cmd/ line inside a fenced block counts, so a sentence mentioning cmd/
    cannot short-circuit the search.
    """
    lines = text.splitlines()
    fenced = False
    for index, line in enumerate(lines):
        if line.startswith("```"):
            fenced = not fenced
            continue
        if not fenced or not re.search(r"^\S*\s*cmd/(\s|$)", line):
            continue
        stated = None
        if claim := re.search(r"#\s*(\S+)\s+programs", line):
            stated = COUNT_WORDS.get(claim.group(1).lower())
            if stated is None and claim.group(1).isdigit():
                stated = int(claim.group(1))
            if stated is None:
                # A claim was made in a form this checker cannot read. Saying
                # "no claim" here would silently stop checking the count.
                return set(), -1, True
        listed: set[str] = set()
        indent = None
        for following in lines[index + 1:]:
            if following.startswith("```"):
                break
            entry = re.match(r"(\s+)[├└]──\s+([A-Za-z0-9._-]+)/", following)
            if entry:
                if indent is None:
                    indent = len(entry.group(1))
                if len(entry.group(1)) == indent:
                    listed.add(entry.group(2))
            elif re.match(r"^[├└]──", following):
                break
        return listed, stated, True
    return set(), None, False


def rule_failures(read, list_cmd) -> list[str]:
    """All rules, over injectable readers so the self-test can violate them."""
    failures: list[str] = []

    def text(rel: str) -> str | None:
        raw = read(rel)
        if raw is None:
            failures.append(f"{rel}: required file is missing")
            return None
        return raw.decode("utf-8", "replace")

    # 1. Every artifact a release ships is offered somewhere a reader can see.
    workflow = text(RELEASE_WORKFLOW)
    page = text(DOWNLOAD_PAGE)
    readme = text(READ_ME)
    if workflow is not None and page is not None and readme is not None:
        shipped, unreadable = released_assets(workflow)
        for step in unreadable:
            failures.append(
                f"{RELEASE_WORKFLOW}: an upload step names no asset this checker can read "
                f"({step!r}); its shape changed and the check is now partly blind"
            )
        if not shipped:
            failures.append(
                f"{RELEASE_WORKFLOW}: no release assets found at all; the upload step's shape "
                f"changed and this checker is now blind"
            )
        offered = linked_assets(page)
        for asset in sorted(shipped):
            if asset in offered:
                continue
            if is_sidecar(asset):
                continue
            failures.append(
                f"{DOWNLOAD_PAGE}: the release ships {asset} and nothing on the download page "
                f"links it. Add a download button, or give it a sidecar suffix in this checker "
                f"with the reason it is never offered"
            )
        for surface, names in ((DOWNLOAD_PAGE, offered), (READ_ME, linked_assets(readme))):
            for asset in sorted(names - shipped):
                failures.append(
                    f"{surface}: links {asset}, which no release step uploads — the link is dead"
                )

    # 1b. A Microsoft Store link names the product the packaging identity does.
    identity = text(STORE_IDENTITY)
    if identity is not None and page is not None and readme is not None:
        try:
            want = json.loads(identity).get("storeUrl", "")
        except json.JSONDecodeError as broken:
            failures.append(f"{STORE_IDENTITY}: is not valid JSON ({broken})")
            want = ""
        if not want:
            failures.append(
                f"{STORE_IDENTITY}: has no storeUrl, so this checker cannot tell whether a "
                f"Store link on a public page is the right one"
            )
        else:
            for surface, body in ((DOWNLOAD_PAGE, page), (READ_ME, readme)):
                for link in sorted(store_links(body)):
                    if link != want:
                        failures.append(
                            f"{surface}: links {link}, but {STORE_IDENTITY} says the product is "
                            f"at {want} — one of the two is stale"
                        )

    # 2. One instruction, written out three times, stays one instruction.
    listings = {}
    for rel in DEPENDENCY_SOURCES:
        body = text(rel)
        if body is None:
            continue
        listings[rel] = apt_packages(body)
    if len(listings) == len(DEPENDENCY_SOURCES):
        blind = sorted(rel for rel, packages in listings.items() if not packages)
        if blind:
            failures.append(
                f"the Linux build dependencies could not be read from {', '.join(blind)} — "
                f"the apt line's shape changed, or it no longer installs {DEPENDENCY_ANCHOR}, "
                f"and this checker is now blind"
            )
        elif len({frozenset(packages) for packages in listings.values()}) != 1:
            for rel, packages in sorted(listings.items()):
                failures.append(f"{rel}: Linux build dependencies are {sorted(packages)}")
            failures.append(
                "the Linux build dependencies differ between the README, CONTRIBUTING and CI; "
                "a contributor following the odd one out gets a build this repository never tests"
            )

    # 2b. The first launch of the unsigned Mac download, explained four times,
    # names the route every supported macOS takes and not the one 15 removed.
    config = text(PRODUCT_CONFIG)
    floor = mac_floor(config) if config is not None else None
    if config is not None and floor is None:
        failures.append(
            f"{PRODUCT_CONFIG}: {MAC_FLOOR_KEY} is missing or unreadable, so this checker "
            f"cannot tell which macOS versions the first-launch steps must cover"
        )
    guide = text(MAC_STORE_GUIDE)
    bodies = {
        RELEASE_WORKFLOW: workflow, DOWNLOAD_PAGE: page, READ_ME: readme, MAC_STORE_GUIDE: guide,
    }
    for surface in MAC_OPEN_SURFACES:
        if bodies[surface] is None:
            continue
        body = mac_open_text(surface, bodies[surface])
        if body is None:
            failures.append(
                f"{surface}: no NOTES=\"$(printf '...')\" line was found, so this checker cannot "
                f"read the release notes' macOS steps; its shape changed and the check is blind"
            )
            continue
        for stale in STALE_FINDER_ROUTE.finditer(body):
            failures.append(
                f"{surface}: tells a Mac reader to {' '.join(stale.group(0).split())!r}, the "
                f"override macOS 15 removed; write the Finder route for macOS 12 to 14 as "
                f"{FINDER_ROUTE}, beside {SETTINGS_ROUTE} for 15 and later"
            )
        if SETTINGS_ROUTE not in body:
            failures.append(
                f"{surface}: never names {SETTINGS_ROUTE}; from macOS 15, Privacy & Security's "
                f"{SETTINGS_ROUTE} is the only way to open the unsigned download"
            )
        if floor is not None and floor < FINDER_ROUTE_GONE and FINDER_ROUTE not in body:
            failures.append(
                f"{surface}: the app runs from macOS {floor}, and nothing tells a reader on "
                f"{floor} to {FINDER_ROUTE.lower()} the app in the Finder and choose Open"
            )

    # 3. Prose that counts the tree is checked against the tree.
    if readme is not None:
        listed, stated, found = readme_cmd_tree(readme)
        actual = list_cmd()
        if actual is None:
            failures.append("cmd/: directory is missing")
        elif not found:
            failures.append(
                f"{READ_ME}: no cmd/ line was found in any fenced block; the repository-layout "
                f"tree's shape changed and this checker is now blind"
            )
        elif stated == -1:
            failures.append(
                f"{READ_ME}: the cmd/ line states a count this checker cannot read; write it as "
                f"a word or a numeral so the claim stays checkable"
            )
        elif not listed:
            failures.append(
                f"{READ_ME}: no cmd/ entries could be read under the tree's cmd/ line; its shape "
                f"changed and this checker is now blind"
            )
        else:
            for name in sorted(actual - listed):
                failures.append(f"{READ_ME}: cmd/{name} exists and the tree does not list it")
            for name in sorted(listed - actual):
                failures.append(f"{READ_ME}: the tree lists cmd/{name}, which does not exist")
            if stated is not None and stated != len(actual):
                failures.append(
                    f"{READ_ME}: the tree says {stated} programs under cmd/; there are {len(actual)}"
                )
    return failures


def real_reader(rel: str) -> bytes | None:
    path = ROOT / rel
    return path.read_bytes() if path.is_file() else None


def real_cmd_listing() -> set[str] | None:
    path = ROOT / "cmd"
    if not path.is_dir():
        return None
    return {
        child.name
        for child in path.iterdir()
        if child.is_dir() and child.name not in NOT_A_PROGRAM
    }


def self_test() -> list[str]:
    """Every rule and every blindness guard, each violated exactly once."""
    problems: list[str] = []
    deps = b"sudo apt-get install gcc libgl1-mesa-dev libasound2-dev\n"
    routes = b"From macOS 15, click Open Anyway. On 12 to 14, Control-click it and choose Open.\n"
    settings_only = b"Click Open Anyway in Privacy & Security.\n"
    finder_only = b"Control-click the app and choose Open.\n"

    def notes(body: bytes) -> bytes:
        """A release-notes step whose printf carries body, shaped as release.yml's."""
        opening = b"""          NOTES="$(printf 'Desktop builds.\\n\\nmacOS note: """
        return opening + body.strip() + b"""')"\n"""

    clean = {
        RELEASE_WORKFLOW: (
            b'        run: gh release upload "$TAG" BibleText-Linux-amd64.tar.xz '
            b"BibleText-x86_64.AppImage BibleText-x86_64.AppImage.zsync --clobber\n" + notes(routes)
        ),
        STORE_IDENTITY: b'{"storeId": "TESTID", "storeUrl": "https://apps.microsoft.com/detail/TESTID"}\n',
        DOWNLOAD_PAGE: (
            b'<a href="https://example.invalid/releases/latest/download/'
            b'BibleText-Linux-amd64.tar.xz">Linux</a>\n'
            b'<a href="https://example.invalid/releases/latest/download/'
            b'BibleText-x86_64.AppImage">AppImage</a>\n'
            b'<a href="https://apps.microsoft.com/detail/TESTID">Store</a>\n' + routes
        ),
        READ_ME: (
            b"```\n"
            b"\xe2\x94\x94\xe2\x94\x80\xe2\x94\x80 cmd/   # two programs\n"
            b"    \xe2\x94\x9c\xe2\x94\x80\xe2\x94\x80 desktop/\n"
            b"    \xe2\x94\x94\xe2\x94\x80\xe2\x94\x80 mobile/\n"
            b"```\n" + deps + routes
        ),
        CONTRIBUTING: deps,
        CI_WORKFLOW: deps,
        PRODUCT_CONFIG: b'{"macMinimumOSVersion": "12.0"}\n',
        MAC_STORE_GUIDE: b"| Signing | none (" + routes.strip() + b") |\n",
    }
    pair = lambda: {"desktop", "mobile"}  # noqa: E731 - a stub, not a policy

    def run(tree, cmd_listing=pair) -> list[str]:
        return rule_failures(lambda rel: tree.get(rel), cmd_listing)

    if clean_failures := run(clean):
        problems.append(f"a consistent tree still fails: {clean_failures}")

    # A wrapped upload step must read the same as an unwrapped one.
    wrapped = dict(clean)
    wrapped[RELEASE_WORKFLOW] = (
        b'        run: |\n          gh release upload "$TAG" \\\n'
        b"            BibleText-Linux-amd64.tar.xz \\\n"
        b"            BibleText-x86_64.AppImage \\\n"
        b"            BibleText-x86_64.AppImage.zsync --clobber\n" + notes(routes)
    )
    if wrapped_failures := run(wrapped):
        problems.append(f"a wrapped upload step is misread: {wrapped_failures}")

    # So must a wrapped dependency line.
    wrapped_deps = dict(clean)
    folded = b"sudo apt-get install gcc \\\n  libgl1-mesa-dev libasound2-dev\n"
    wrapped_deps[CONTRIBUTING] = folded
    if wrapped_dep_failures := run(wrapped_deps):
        problems.append(f"a wrapped dependency line is misread: {wrapped_dep_failures}")

    # A matrixed upload names its asset through the matrix, so the literal text
    # carries no asset name at all. Read verbatim, the name below would be taken
    # for an asset called "BibleText-Windows", and every correct link on the page
    # would then be reported as dead.
    matrixed = dict(clean)
    matrixed[RELEASE_WORKFLOW] = (
        b"  windows:\n"
        b"    strategy:\n"
        b"      matrix:\n"
        b"        include:\n"
        b"          - goarch: amd64\n"
        b"          - goarch: arm64\n"
        b"    steps:\n"
        b'      - run: gh release upload "$TAG" BibleText-Windows-${{ matrix.goarch }}.zip --clobber\n'
        + notes(routes)
    )
    matrixed[DOWNLOAD_PAGE] = (
        b'<a href="https://example.invalid/releases/latest/download/'
        b'BibleText-Windows-amd64.zip">Windows</a>\n'
        b'<a href="https://example.invalid/releases/latest/download/'
        b'BibleText-Windows-arm64.zip">Windows ARM</a>\n'
        b'<a href="https://apps.microsoft.com/detail/TESTID">Store</a>\n' + routes
    )
    if matrix_failures := run(matrixed):
        problems.append(f"a matrixed upload step is misread: {matrix_failures}")

    # Once the floor reaches macOS 15 no supported reader has the Finder route,
    # so the rule must stop owing it rather than demand it for ever.
    risen = dict(clean)
    risen[PRODUCT_CONFIG] = b'{"macMinimumOSVersion": "15.0"}\n'
    for rel in MAC_OPEN_SURFACES:
        risen[rel] = clean[rel].replace(routes.strip(), settings_only.strip())
    if risen_failures := run(risen):
        problems.append(f"a macOS 15 floor still owes the Finder route: {risen_failures}")

    # Right-click sentences that are not the retired advice must pass: the
    # app's own selection menu, with an Open in the next sentence and an open
    # that is not a menu item, and a sentence that names right-clicking only to
    # send the reader to Open Anyway.
    bystanding = dict(clean)
    for rel in (DOWNLOAD_PAGE, READ_ME):
        bystanding[rel] = clean[rel] + (
            b"Select a passage and right-click: the menu offers Copy, Look Up and Share. "
            b"Open a chapter to begin.\n"
            b"Right-click a verse to open the study menu.\n"
            b"On macOS 15 right-clicking no longer helps; click Open Anyway instead.\n"
        )
    if bystander_failures := run(bystanding):
        problems.append(f"a right-click that is not the retired advice fails: {bystander_failures}")

    violations: list[tuple[str, dict, object]] = []

    # ...and the expansion must still DEMAND each leg. Without this case, matrix
    # support could satisfy the clean fixture by switching the rule off.
    matrix_unlinked = dict(matrixed)
    matrix_unlinked[DOWNLOAD_PAGE] = matrixed[DOWNLOAD_PAGE].replace(
        b'<a href="https://example.invalid/releases/latest/download/'
        b'BibleText-Windows-arm64.zip">Windows ARM</a>\n',
        b"",
    )
    violations.append(("one leg of a matrixed asset left unlinked", matrix_unlinked, pair))

    unlinked = dict(clean)
    unlinked[DOWNLOAD_PAGE] = clean[DOWNLOAD_PAGE].replace(
        b'<a href="https://example.invalid/releases/latest/download/'
        b'BibleText-x86_64.AppImage">AppImage</a>\n',
        b"",
    )
    violations.append(("an unlinked release asset", unlinked, pair))

    dead = dict(clean)
    dead[DOWNLOAD_PAGE] = clean[DOWNLOAD_PAGE] + (
        b'<a href="https://example.invalid/releases/latest/download/'
        b'BibleText-Gone.zip">Gone</a>\n'
    )
    violations.append(("a dead download link", dead, pair))

    blind_upload = dict(clean)
    blind_upload[RELEASE_WORKFLOW] = b"        run: echo nothing is uploaded here\n" + notes(routes)
    blind_upload[DOWNLOAD_PAGE] = b"<p>no downloads yet</p>\n" + routes
    blind_upload[READ_ME] = clean[READ_ME]
    violations.append(("a release that uploads nothing", blind_upload, pair))

    unreadable_upload = dict(clean)
    unreadable_upload[RELEASE_WORKFLOW] = (
        clean[RELEASE_WORKFLOW] + b'        run: gh release upload "$TAG" $ASSETS --clobber\n'
    )
    violations.append(("an upload step naming no asset", unreadable_upload, pair))

    stale_store = dict(clean)
    stale_store[DOWNLOAD_PAGE] = clean[DOWNLOAD_PAGE].replace(
        b"detail/TESTID", b"detail/OLDID")
    violations.append(("a stale Microsoft Store link", stale_store, pair))

    stale_store_readme = dict(clean)
    stale_store_readme[READ_ME] = clean[READ_ME] + b"https://apps.microsoft.com/detail/OLDID\n"
    violations.append(("a stale Store link in the README", stale_store_readme, pair))

    no_store_url = dict(clean)
    no_store_url[STORE_IDENTITY] = b'{"storeId": "TESTID"}\n'
    violations.append(("an identity with no storeUrl", no_store_url, pair))

    broken_identity = dict(clean)
    broken_identity[STORE_IDENTITY] = b"{ this is not json\n"
    violations.append(("an unreadable store identity", broken_identity, pair))

    drifted = dict(clean)
    drifted[CONTRIBUTING] = b"sudo apt-get install gcc libgl1-mesa-dev\n"
    violations.append(("drifted build dependencies", drifted, pair))

    blind_deps = dict(clean)
    for rel in DEPENDENCY_SOURCES:
        blind_deps[rel] = clean[rel].replace(deps, b"nix-shell -p gcc libGL alsa-lib\n")
    violations.append(("dependency lists that cannot be read", blind_deps, pair))

    # Count-preserving, so the count rule cannot mask this: only the
    # "listed but absent" direction fires.
    gone = dict(clean)
    gone[READ_ME] = clean[READ_ME].replace(b"   # two programs", b"")
    violations.append(("a listed cmd/ program that is gone", gone, lambda: {"desktop"}))

    # No stated count, and a program the tree omits: isolates the other
    # direction, "exists but unlisted" — the drift this rule exists for.
    uncounted = dict(clean)
    uncounted[READ_ME] = clean[READ_ME].replace(b"   # two programs", b"")
    violations.append(
        ("an unlisted cmd/ program", uncounted, lambda: {"desktop", "mobile", "extra"})
    )

    miscounted = dict(clean)
    miscounted[READ_ME] = clean[READ_ME].replace(b"two programs", b"three programs")
    violations.append(("a stated count that is wrong", miscounted, pair))

    unreadable_count = dict(clean)
    unreadable_count[READ_ME] = clean[READ_ME].replace(b"two programs", b"several programs")
    violations.append(("a count written unreadably", unreadable_count, pair))

    reindented = dict(clean)
    reindented[READ_ME] = clean[READ_ME].replace(b"   # two programs", b"")
    reindented[READ_ME] = reindented[READ_ME].replace(
        b"    \xe2\x94\x9c\xe2\x94\x80\xe2\x94\x80 desktop/\n"
        b"    \xe2\x94\x94\xe2\x94\x80\xe2\x94\x80 mobile/\n",
        b"",
    )
    violations.append(("a tree with no readable cmd/ entries", reindented, pair))

    unfenced = dict(clean)
    unfenced[READ_ME] = clean[READ_ME].replace(b"```\n", b"", 1)
    violations.append(("a cmd/ line outside any fenced block", unfenced, pair))

    # Each copy is held on its own: the old instruction in any one of them
    # strands a reader on macOS 15, and the settings route alone sends a reader
    # on macOS 12 to a System Settings pane that release does not have. The four
    # are named here rather than read from MAC_OPEN_SURFACES, so a copy dropped
    # from that tuple is a copy this self-test still expects to be held.
    #
    # Text pasted from an old release strands the same reader even beside the
    # right routes, so each copy also gets the retired advice added, in the
    # wording and markup it carried before macOS 15. Both routes are still
    # named, so only the rule that rejects the old advice can catch it.
    retired = {
        RELEASE_WORKFLOW: b"On first launch, right-click the app and choose Open.",
        DOWNLOAD_PAGE: b"Or right-click the app and choose <em>Open</em> the\n  first time.",
        READ_ME: b"Or right-click \xe2\x86\x92 **Open** the first time.",
        MAC_STORE_GUIDE: b"Or readers right-click \xe2\x86\x92 Open.",
    }
    for rel in (RELEASE_WORKFLOW, DOWNLOAD_PAGE, READ_ME, MAC_STORE_GUIDE):
        stale = dict(clean)
        stale[rel] = clean[rel].replace(routes.strip(), finder_only.strip())
        violations.append((f"only the Finder route in {rel}", stale, pair))
        settings_alone = dict(clean)
        settings_alone[rel] = clean[rel].replace(routes.strip(), settings_only.strip())
        violations.append((f"no Finder route in {rel} on a macOS 12 floor", settings_alone, pair))
        pasted = dict(clean)
        pasted[rel] = clean[rel].replace(routes.strip(), routes.strip() + b" " + retired[rel])
        violations.append((f"the retired right-click advice beside the routes in {rel}", pasted, pair))

    # The retired advice in another spelling is still the retired advice.
    respelled = dict(clean)
    respelled[READ_ME] = clean[READ_ME].replace(
        routes.strip(), routes.strip() + b" Right click the app and choose Open."
    )
    violations.append(("the retired advice spelled Right click", respelled, pair))

    # The cut-off at 15 is pinned from both sides: the 15 floor above owes no
    # Finder route, and 14, the last release that has it, still does.
    fourteen = dict(clean)
    fourteen[PRODUCT_CONFIG] = b'{"macMinimumOSVersion": "14.0"}\n'
    for rel in MAC_OPEN_SURFACES:
        fourteen[rel] = clean[rel].replace(routes.strip(), settings_only.strip())
    violations.append(("no Finder route on a macOS 14 floor", fourteen, pair))

    # A comment beside the release notes that names both routes must not stand
    # in for the notes. The notes here carry no macOS steps and no retired
    # advice, so only reading the printf alone can fail them.
    commented = dict(clean)
    commented[RELEASE_WORKFLOW] = clean[RELEASE_WORKFLOW].replace(
        notes(routes), notes(b"The app is unsigned.")
    ) + b"          # macOS 15 removed Control-click > Open; Open Anyway is the only way.\n"
    violations.append(("release notes vouched for by the comment beside them", commented, pair))

    blind_notes = dict(clean)
    blind_notes[RELEASE_WORKFLOW] = clean[RELEASE_WORKFLOW].replace(
        notes(routes), b"          # " + routes
    )
    violations.append(("release notes no NOTES printf carries", blind_notes, pair))

    commented_page = dict(clean)
    commented_page[DOWNLOAD_PAGE] = clean[DOWNLOAD_PAGE].replace(
        routes, b"The downloads are unsigned.\n<!-- " + routes.strip() + b" -->\n"
    )
    violations.append(("macOS steps only inside a markup comment", commented_page, pair))

    blind_floor = dict(clean)
    blind_floor[PRODUCT_CONFIG] = b'{"iosMinimumOSVersion": "15.0"}\n'
    violations.append(("an unreadable macOS floor", blind_floor, pair))

    absent = dict(clean)
    del absent[CONTRIBUTING]
    violations.append(("a missing file", absent, pair))

    for name, tree, cmd_listing in violations:
        if not run(tree, cmd_listing):
            problems.append(f"{name} was not caught")
    return problems


def main() -> int:
    if problems := self_test():
        print("public surfaces checker self-test failed:")
        for problem in problems:
            print(f"  - {problem}")
        return 1
    failures = rule_failures(real_reader, real_cmd_listing)
    if failures:
        print("public surfaces check failed:")
        for failure in failures:
            print(f"  - {failure}")
        return 1
    print("public surfaces check passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
