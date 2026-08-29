# Versioning

Two rules, both learned the expensive way:

> **A version number is spent the moment anything ships under it.**
>
> **A version number names exactly one tree, on every channel.**

The second is the first applied across channels rather than across time. The
App Store build, the iOS build, the GitHub desktop assets, the sideload APK
and the git tag must all come from the SAME commit. A version that means
different code in different places is unfalsifiable: a bug report naming
"1.2.4" no longer identifies what the reporter ran, and no later fix can
repair the ambiguity.

In practice this bites when a tag is cut after work has continued. The commit
to tag is the one the STORE BUILDS were made from, not HEAD — anything merged
since ships under the next number. And a channel that cannot be rebuilt from
the tagged commit does not ride along on an older artifact: rebuild it, or
hold the release. The release workflow rebuilds desktop assets only, so the
APK is the usual one to get wrong.

"Ships" means any of: a GitHub release published, an App Store or Play
submission sent, a tag pushed. After that, changed code takes a NEW number —
never the old number with a moved tag.

## Why the rule is mechanical, not stylistic

`git fetch` deliberately does not update a tag that changed on the remote, so
every clone that ever fetched `v1.2.3` keeps its old one silently. Once a
published tag moves, the name stops meaning one thing, permanently, in ways no
later fix can reach. Package ecosystems harden the same rule into law (the Go
module sum database records the first checksum forever; npm and crates.io
refuse republished versions). This repository moved a published tag three
times in August 2026 — each deliberate, recorded, and survivable only because
nothing `go get`s this module and the release assets are the real product —
and the third time was declared the last.

## What enforces it

`scripts/check-version-not-spent.sh` runs at the top of every release build
(`release-ios.sh`, `release-mac-store.sh`, `build-android.sh --release`). If
origin already has the tag for the version being built and it does not point
at HEAD, the build refuses with instructions to bump the version.

What still passes: pre-release iteration (no tag exists yet — App Store build
numbers 41, 42, 43 under one unreleased version are normal), and rebuilding
exactly the commit that shipped. `BIBLETEXT_ALLOW_SPENT_VERSION=1` is the
emergency override; using it means writing down why, because it recreates the
exact situation this file exists to prevent.

## The cadence that avoids ever wanting a moved tag

1. Version numbers are cheap. A one-line fix after a release is a new patch
   version everywhere, not a quiet re-cut.
2. Cut the GitHub tag LAST, after the App Store submissions are in — Apple's
   records can iterate build numbers under an unreleased version; a published
   tag cannot iterate at all.
3. Every channel of a version builds from the tagged commit, so "v1.2.3" is
   one tree on the stores, the direct downloads, and the sideload APK alike.
4. Tags are consistent in kind as well as in content. The repo is currently
   mixed — v1.2.1 and v1.2.3 are lightweight, v1.2.2 is annotated — which is
   worth settling on the next cut, because an annotated tag is an object
   wrapping a commit and a peel (`ls-remote 'refs/tags/X^{}'`) is the only
   safe way to compare one against a commit.
