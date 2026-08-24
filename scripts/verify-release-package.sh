#!/usr/bin/env bash

# Reject desktop release packages that retain a GitHub runner workspace path.
# The root is derived at runtime so the repository does not publish a runner's
# home-directory layout as part of the check itself.
set -euo pipefail

fail() {
  echo "release package verification failed: $1" >&2
  exit 1
}

[[ "$#" -eq 2 ]] || fail "expected an executable and its package"

binary_path="$1"
package_path="$2"
workspace_raw="${GITHUB_WORKSPACE:-}"

[[ -f "$binary_path" ]] || fail "packaged executable is missing"
[[ -e "$package_path" ]] || fail "package is missing"
[[ -n "$workspace_raw" ]] || fail "GITHUB_WORKSPACE is missing"

if ! build_info="$(go version -m "$binary_path" 2>/dev/null)"; then
  fail "could not inspect the packaged executable"
fi
[[ "$build_info" == *$'build\t-trimpath=true'* ]] || \
  fail "packaged executable is not a trimpath build"

# Normalize GitHub's Windows form before removing the checkout's two
# repository-specific components. Both separator forms are scanned because Go
# and native tools can record Windows paths differently.
workspace_slash="${workspace_raw//\\//}"
workspace_slash="${workspace_slash%/}"
workspace_parent="${workspace_slash%/*}"
runner_root_slash="${workspace_parent%/*}"

[[ "$workspace_parent" != "$workspace_slash" ]] || \
  fail "could not derive the runner workspace root"
[[ -n "$runner_root_slash" && "$runner_root_slash" != "$workspace_parent" && "$runner_root_slash" != "/" ]] || \
  fail "could not derive the runner workspace root"

runner_root_slash="${runner_root_slash}/"
runner_root_backslash="${runner_root_slash//\//\\}"

scan_root() {
  local runner_root="$1"
  local scan_status

  if [[ -d "$package_path" ]]; then
    if LC_ALL=C grep -aRFq -- "$runner_root" "$package_path"; then
      fail "package contains a runner workspace path"
    else
      scan_status=$?
    fi
  else
    if LC_ALL=C grep -aFq -- "$runner_root" "$package_path"; then
      fail "package contains a runner workspace path"
    else
      scan_status=$?
    fi
  fi

  [[ "$scan_status" -eq 1 ]] || fail "package scan did not complete"
}

scan_root "$runner_root_slash"
scan_root "$runner_root_backslash"
