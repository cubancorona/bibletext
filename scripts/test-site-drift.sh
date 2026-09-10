#!/usr/bin/env bash
# Regression for scripts/site-drift.sh: identical trees report none and return
# 0; a changed, an added and a removed file are each counted and return 1; a
# .git directory in the live tree is ignored. Mutation guarded: a report that
# counted only changed files, or returned 0 on drift, or counted the live
# tree's .git as a removal.
set -euo pipefail
cd "$(dirname "$0")/.."
. scripts/site-drift.sh
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT
mkdir -p "$T/live/web/john/3" "$T/built/web/john/3" "$T/live/.git"
echo same > "$T/live/web/john/3/index.html"; cp "$T/live/web/john/3/index.html" "$T/built/web/john/3/index.html"
echo x > "$T/live/.git/HEAD"

out=$(site_drift "$T/live" "$T/built") && rc=0 || rc=$?
[[ $rc -eq 0 ]] || { echo "FAIL: identical trees returned $rc: $out"; exit 1; }
[[ "$out" == *"none"* ]] || { echo "FAIL: identical trees reported: $out"; exit 1; }

echo changed > "$T/built/web/john/3/index.html"
echo added > "$T/built/added.html"
echo removed > "$T/live/removed.html"
out=$(site_drift "$T/live" "$T/built") && rc=0 || rc=$?
[[ $rc -eq 1 ]] || { echo "FAIL: differing trees returned $rc"; exit 1; }
[[ "$out" == *"1 changed, 1 added, 1 removed"* ]] || { echo "FAIL: counts wrong: $out"; exit 1; }
[[ "$out" == *"Only in live: removed.html"* && "$out" == *"Only in built: added.html"* ]] || { echo "FAIL: examples missing: $out"; exit 1; }
[[ "$out" != *".git"* ]] || { echo "FAIL: the live tree's .git was counted: $out"; exit 1; }
echo "site drift report: OK"
