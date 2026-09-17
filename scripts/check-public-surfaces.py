#!/usr/bin/env python3
"""Hold the public pages to what the repository actually ships.

Three kinds of drift have each reached a published page, and none of them is
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

import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

RELEASE_WORKFLOW = ".github/workflows/release.yml"
DOWNLOAD_PAGE = "docs/index.html"
READ_ME = "README.md"
CONTRIBUTING = "CONTRIBUTING.md"
CI_WORKFLOW = ".github/workflows/ci.yml"

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
    for line in fold_continuations(text).splitlines():
        if "gh release upload" not in line:
            continue
        found = set(re.findall(ASSET, line))
        if not found:
            unreadable.append(line.strip())
        names.update(found)
    return names, unreadable


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
    clean = {
        RELEASE_WORKFLOW: (
            b'        run: gh release upload "$TAG" BibleText-Linux-amd64.tar.xz '
            b"BibleText-x86_64.AppImage BibleText-x86_64.AppImage.zsync --clobber\n"
        ),
        DOWNLOAD_PAGE: (
            b'<a href="https://example.invalid/releases/latest/download/'
            b'BibleText-Linux-amd64.tar.xz">Linux</a>\n'
            b'<a href="https://example.invalid/releases/latest/download/'
            b'BibleText-x86_64.AppImage">AppImage</a>\n'
        ),
        READ_ME: (
            b"```\n"
            b"\xe2\x94\x94\xe2\x94\x80\xe2\x94\x80 cmd/   # two programs\n"
            b"    \xe2\x94\x9c\xe2\x94\x80\xe2\x94\x80 desktop/\n"
            b"    \xe2\x94\x94\xe2\x94\x80\xe2\x94\x80 mobile/\n"
            b"```\n" + deps
        ),
        CONTRIBUTING: deps,
        CI_WORKFLOW: deps,
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
        b"            BibleText-x86_64.AppImage.zsync --clobber\n"
    )
    if wrapped_failures := run(wrapped):
        problems.append(f"a wrapped upload step is misread: {wrapped_failures}")

    # So must a wrapped dependency line.
    wrapped_deps = dict(clean)
    folded = b"sudo apt-get install gcc \\\n  libgl1-mesa-dev libasound2-dev\n"
    wrapped_deps[CONTRIBUTING] = folded
    if wrapped_dep_failures := run(wrapped_deps):
        problems.append(f"a wrapped dependency line is misread: {wrapped_dep_failures}")

    violations: list[tuple[str, dict, object]] = []

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
    blind_upload[RELEASE_WORKFLOW] = b"        run: echo nothing is uploaded here\n"
    blind_upload[DOWNLOAD_PAGE] = b"<p>no downloads yet</p>\n"
    blind_upload[READ_ME] = clean[READ_ME]
    violations.append(("a release that uploads nothing", blind_upload, pair))

    unreadable_upload = dict(clean)
    unreadable_upload[RELEASE_WORKFLOW] = (
        clean[RELEASE_WORKFLOW] + b'        run: gh release upload "$TAG" $ASSETS --clobber\n'
    )
    violations.append(("an upload step naming no asset", unreadable_upload, pair))

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
