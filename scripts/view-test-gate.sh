#!/usr/bin/env bash
# The acceptance gate for the view tests.
#
# WHY THIS EXISTS, AND WHY IT IS A SCRIPT RATHER THAN A PARAGRAPH.
#
# A view-test harness was built once already: 1,310 enumerated screens across
# seven platform profiles. It did not catch the bug it was written for. Neutering
# the one-line fix for the Read-tab regression left the whole enumeration green,
# removing the notes browser's only exit control was invisible to every cell, and
# the "hidden but present" case — the entire point — was demonstrated only on the
# one platform where it was never hard. It was rejected and is on the branch
# view-harness-attempt-1.
#
# The failure was not the harness. It was that "does it catch anything?" was
# asked AFTER the harness existed, when the answer was expensive to act on. So
# the question is asked here, first, mechanically: each mutation below breaks
# something a reader would SEE, and the suite must go red for it. A mutation that
# survives names a hole, and the harness is not finished while any survive.
#
# Usage:  scripts/view-test-gate.sh            # every mutation
#         scripts/view-test-gate.sh M3         # just one
#
# It never leaves the tree dirty: every mutation is applied to a COPY.

set -uo pipefail
cd "$(dirname "$0")/.."
REPO="$PWD"
WORK="${TMPDIR:-/tmp}/bibletext-view-gate.$$"
ONLY="${1:-}"
PASS=0; FAIL=0

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

bold()  { printf '\033[1m%s\033[0m\n' "$1"; }
green() { printf '  \033[32m%s\033[0m\n' "$1"; }
red()   { printf '  \033[31m%s\033[0m\n' "$1"; }

# fresh_copy makes a clean tree to mutate.
fresh_copy() {
  rm -rf "$WORK"; mkdir -p "$WORK"
  rsync -a --exclude='.git' --exclude='build' --exclude='third_party' \
        --exclude='zz_*' "$REPO/" "$WORK/"
}

# run_mutation <id> <description> <what the reader would see> <python mutation>
#
# The mutation is a python3 program run inside the copy. It must CHANGE
# something; if it cannot find its target it should exit non-zero, so a mutation
# that silently stops applying (because the code moved) is reported rather than
# counted as a pass.
run_mutation() {
  local id="$1" desc="$2" sees="$3" prog="$4"
  [ -n "$ONLY" ] && [ "$ONLY" != "$id" ] && return 0

  bold "$id — $desc"
  printf '     reader sees: %s\n' "$sees"
  fresh_copy
  if ! (cd "$WORK" && python3 -c "$prog"); then
    red "MUTATION DID NOT APPLY — its target moved. Fix the gate, not the harness."
    FAIL=$((FAIL+1)); return 0
  fi
  if (cd "$WORK" && go test ./ >/dev/null 2>&1); then
    red "SURVIVED — the suite is green with this defect in place."
    FAIL=$((FAIL+1))
  else
    local which
    which=$(cd "$WORK" && go test ./ 2>&1 | grep -E '^\s*--- FAIL' | head -3 | sed 's/^ *//')
    green "caught"
    [ -n "$which" ] && printf '%s\n' "$which" | sed 's/^/       /'
    PASS=$((PASS+1))
  fi
}

sub_py() {
  # helper: emit a python program that replaces OLD with NEW in FILE, or exits 1
  cat <<PY
import sys
p = "$1"
s = open(p).read()
old = """$2"""
new = """$3"""
if old not in s:
    sys.stderr.write("target not found in " + p + "\n"); sys.exit(1)
open(p, "w").write(s.replace(old, new, 1))
PY
}

bold "View-test acceptance gate"
echo

# ── M1 ────────────────────────────────────────────────────────────────────────
# The bug that shipped in 1.1.6 and 1.1.7. If the harness misses this, it has no
# claim to exist: it is the reason the harness was commissioned.
run_mutation M1 \
  "the Read tab keeps believing results occupy the reading pane" \
  "a Read tab holding search results, no verses, no search field" \
  "$(sub_py layout.go '	state.IsSearching = false
}' '	_ = tab
}')"

# ── M2 ────────────────────────────────────────────────────────────────────────
# The the implementation requirement: assert what is SEEN, not what is built. Every
# object stays in the tree; one of them is simply invisible. A test that walks
# the widget tree passes this mutation, which is exactly why walking the tree is
# not enough.
run_mutation M2 \
  "the reading pane is hidden but left in the tree" \
  "a blank reading area, with every widget still present underneath" \
  "$(sub_py reading.go '	paper := readingScrollArea(state, verses, state.pal())' '	paper := readingScrollArea(state, verses, state.pal())
	paper.Hide() // MUTATION: present in the tree, invisible on screen')"

# ── M3 ────────────────────────────────────────────────────────────────────────
# V3, the way out. The macOS notes view shipped without one and the owner had to
# report it. Removing an exit control must not be invisible.
run_mutation M3 \
  "the notes browser loses its only exit control" \
  "a full-screen notes list with no way back to reading" \
  "$(sub_py notes_browse.go '		done := widget.NewButtonWithIcon("Done", theme.NavigateBackIcon(), func() {' '		done := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
			_ = 0')"

# ── M4 ────────────────────────────────────────────────────────────────────────
# Occlusion. Something opaque drawn over the verses. Nothing is hidden and
# nothing is missing — the pixels are simply somebody else's.
run_mutation M4 \
  "an opaque panel is drawn over the reading text" \
  "verses covered by a blank rectangle" \
  "$(sub_py reading_styled_pane.go 'func (r *styledPaneRenderer) Objects() []fyne.CanvasObject { return r.objects }' 'func (r *styledPaneRenderer) Objects() []fyne.CanvasObject {
	cover := canvas.NewRectangle(r.pane.pal.Surface)
	cover.Resize(fyne.NewSize(9999, 9999))
	return append(append([]fyne.CanvasObject{}, r.objects...), cover)
}
func (r *styledPaneRenderer) objectsUnused() []fyne.CanvasObject { return r.objects }')"

# ── M5 ────────────────────────────────────────────────────────────────────────
# The wrong screen, coherently rendered. Nothing is blank, nothing is hidden —
# it is simply not the view the reader asked for.
run_mutation M5 \
  "the reading pane renders the search results instead" \
  "tapping Read shows results even with no search running" \
  "$(sub_py reading.go 'func buildReadingView(state *AppState) fyne.CanvasObject {' 'func buildReadingView(state *AppState) fyne.CanvasObject {
	if state != nil && state.Bible != nil {
		return buildSearchResultsView(state)
	}')"

# ── M6 ────────────────────────────────────────────────────────────────────────
# Nothing at all, from a builder that still returns a valid object.
run_mutation M6 \
  "a view builder returns an empty container" \
  "a blank screen where the notes list should be" \
  "$(sub_py notes_browse.go 'func buildNotesBrowseView(state *AppState) fyne.CanvasObject {' 'func buildNotesBrowseView(state *AppState) fyne.CanvasObject {
	if state != nil {
		return container.NewVBox()
	}')"

echo
bold "$PASS caught, $FAIL survived"
if [ "$FAIL" -ne 0 ]; then
  echo
  echo "The harness is NOT finished. Each survivor is a defect a reader would see"
  echo "that the tests cannot feel. Close them before adding breadth: a cell that"
  echo "catches nothing is worse than no cell, because it reads as coverage."
  exit 1
fi
echo "Gate passed."
