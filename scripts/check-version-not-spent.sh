#!/usr/bin/env bash
# Refuse to build a release for a version whose tag already shipped other code.
#
# THE RULE (docs/VERSIONING.md): a version number is SPENT the moment anything
# ships under it. Shipping different code under the same number afterwards
# forces the alternative this script exists to prevent — moving a published
# tag, which git fetch deliberately never propagates, so every clone that ever
# fetched it keeps the old one and the number stops meaning one thing.
#
# Mechanically: if origin has v<VERSION> and it does not point at HEAD, the
# number has shipped code that is not this code — bump the version instead.
# Three situations pass:
#   - no v<VERSION> tag anywhere: the number is unspent (pre-release builds,
#     App Store build-number iterations before the GitHub release is cut);
#   - the tag points at HEAD: rebuilding exactly what shipped;
#   - BIBLETEXT_ALLOW_SPENT_VERSION=1: the documented emergency override,
#     for the case where moving the tag was decided deliberately and recorded.
#
# Fails CLOSED when origin's tags cannot be fetched: a release needs the
# network anyway, and "could not check" must never read as "checked".
#
# Usage: check-version-not-spent.sh <version>   (e.g. 1.2.3)
set -euo pipefail

VERSION="${1:?usage: check-version-not-spent.sh <version>}"
TAG="v$VERSION"

if [ "${BIBLETEXT_ALLOW_SPENT_VERSION:-}" = "1" ]; then
  echo "version-spent check OVERRIDDEN for $TAG (BIBLETEXT_ALLOW_SPENT_VERSION=1)" >&2
  exit 0
fi

REMOTE_SHA=$(git ls-remote --tags origin "refs/tags/$TAG" 2>/dev/null | awk '{print $1}') || {
  echo "ERROR: could not read origin's tags to check whether $TAG is spent." >&2
  echo "A release needs the network anyway; refusing to guess." >&2
  exit 1
}

if [ -z "$REMOTE_SHA" ]; then
  echo "version-spent check passed: origin has no $TAG yet"
  exit 0
fi

HEAD_SHA=$(git rev-parse HEAD)
# An annotated tag's ls-remote sha is the tag object; peel it locally if we
# have it, otherwise compare against the ^{} peeled line ls-remote also lists.
PEELED=$(git ls-remote --tags origin "refs/tags/$TAG^{}" 2>/dev/null | awk '{print $1}')
TARGET="${PEELED:-$REMOTE_SHA}"

if [ "$TARGET" = "$HEAD_SHA" ]; then
  echo "version-spent check passed: $TAG already points at HEAD (rebuilding what shipped)"
  exit 0
fi

echo "ERROR: version $VERSION is SPENT — origin's $TAG points at ${TARGET:0:12}," >&2
echo "which is not HEAD (${HEAD_SHA:0:12}). Shipping different code under a" >&2
echo "released number means moving a published tag later. Bump the version" >&2
echo "instead (docs/VERSIONING.md)." >&2
exit 1
