#!/usr/bin/env bash
# Verify the artifact a user installs, from outside the source tree.
#
# `go build -o ./appsumo` plus `./appsumo ...` is how contributors run this CLI,
# and it passes for reasons a user does not have: the binary is simply sitting in
# the working directory. This script installs the way `README.md` tells a user to,
# leaves the checkout, and runs the command by name.
#
# It grades every top-level command rather than one, and prints the count. A
# selector that silently narrows later — a command renamed, a loop that stops
# early — then shows up as a number that dropped rather than as continued green.
#
# What this gate CANNOT reach: the user's own PATH. `go install` writes to
# $(go env GOPATH)/bin, and whether that directory is on PATH is a property of the
# machine, not the artifact. That remedy lives in README.md's Build section, which
# is where the `command not found` is actually read.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
staging="$(mktemp -d)"
elsewhere="$(mktemp -d)"
trap 'rm -rf "$staging" "$elsewhere"' EXIT INT TERM

# Any command that opens SQLite must land in the throwaway directory. Without
# this, running this script on a developer machine writes into the real account
# database at $(go env os.UserConfigDir)/appsumo-cli/appsumo.db.
export APPSUMO_DB_PATH="$elsewhere/appsumo.db"

echo "==> installing into $staging"
GOBIN="$staging" go install "$repo_root/cmd/appsumo"

binary="$staging/appsumo"
if [ ! -x "$binary" ]; then
  echo "FAIL: go install produced no executable named 'appsumo' in $staging" >&2
  ls -la "$staging" >&2
  exit 1
fi

cd "$elsewhere"
echo "==> running from $elsewhere (outside the checkout)"

if ! "$binary" --help >/dev/null; then
  echo "FAIL: installed binary could not print help from outside the source tree" >&2
  exit 1
fi

# Every command the CLI advertises must at least resolve and print its help. A
# command registered but never reachable is exactly the orphaned-feature failure
# that unit tests do not see.
commands=(
  "auth status"
  "portfolio"
  "products list"
  "products search"
  "products export"
  "deals list"
  "deals sync"
  "deals diff"
  "reviews"
  "questions"
  "sync"
  "search"
  "sql"
)
graded=0
for command in "${commands[@]}"; do
  # shellcheck disable=SC2086
  if ! "$binary" $command --help >/dev/null 2>&1; then
    echo "FAIL: 'appsumo $command --help' did not resolve" >&2
    exit 1
  fi
  graded=$((graded + 1))
done

# A packaging fault often first shows up on a failure path, as an unhandled
# import/link error that reads like an application bug. Exercise the commands
# whose required argument is missing; each must refuse, not crash and not succeed.
needs_argument=("reviews" "questions" "search" "sql")
for command in "${needs_argument[@]}"; do
  set +e
  "$binary" "$command" >/dev/null 2>&1
  status=$?
  set -e
  if [ "$status" -eq 0 ]; then
    echo "FAIL: 'appsumo $command' with no argument exited 0; argument validation is not wired" >&2
    exit 1
  fi
  if [ "$status" -gt 2 ]; then
    echo "FAIL: 'appsumo $command' with no argument exited $status, which looks like a crash rather than a refusal" >&2
    exit 1
  fi
  graded=$((graded + 1))
done

echo "==> install smoke passed (graded $graded checks across ${#commands[@]} commands)"
