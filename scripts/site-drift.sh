#!/usr/bin/env bash
# site_drift <live-tree> <built-tree> — how the site about to be published
# differs from the one that is live. Sourced by publish-site.sh; tested by
# test-site-drift.sh.
#
# The web reader is generated from the app's own decoder and data, so it moves
# whenever the app's rendering does — and nothing else in the release sequence
# asked whether the web had kept up. On 10 Sep 2026 it was two days and 5,463
# chapter pages behind the app (Psalm titles and the notes apparatus), and
# nobody knew until someone asked. This makes the answer one line.
#
# Prints a one-line summary and up to five examples; returns 0 when the trees
# are identical and 1 when they differ. A .git directory at the root of the
# live tree is ignored (a worktree carries one; an archive does not).
site_drift() {
  local live="$1" built="$2" out changed added removed
  [[ -d "$live" && -d "$built" ]] || { echo "site drift: cannot compare ($live / $built)"; return 2; }
  out=$(diff -rq "$live" "$built" 2>/dev/null | grep -v "^Only in $live: \.git$" || true)
  if [[ -z "$out" ]]; then
    echo "site drift: none — the tree to publish is identical to the live site"
    return 0
  fi
  changed=$(printf '%s\n' "$out" | grep -c '^Files ' || true)
  added=$(printf '%s\n' "$out" | grep -c "^Only in $built" || true)
  removed=$(printf '%s\n' "$out" | grep -c "^Only in $live" || true)
  echo "site drift: $changed changed, $added added, $removed removed (tree to publish vs live)"
  printf '%s\n' "$out" | head -5 | sed "s|$live|live|g; s|$built|built|g; s/^/    /"
  return 1
}
