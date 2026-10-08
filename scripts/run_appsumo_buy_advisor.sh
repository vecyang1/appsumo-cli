#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"
LOG_DIR="$REPO_ROOT/logs"
mkdir -p "$LOG_DIR"

echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] Starting AppSumo Weekly Buy Advisor Cadence ==="

# 1. Incremental Skip Gate: check if weekly task already completed
WEEK_MARKER="$REPO_ROOT/.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/$(date +%Y-W%W).success"
FORCE=false
DRY_RUN=false
for arg in "$@"; do
  if [ "$arg" = "--force" ]; then
    FORCE=true
  elif [ "$arg" = "--dry-run" ]; then
    DRY_RUN=true
  fi
done

if [ -f "$WEEK_MARKER" ] && [ "$FORCE" = "false" ] && [ "$DRY_RUN" = "false" ]; then
  echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] Weekly success marker $(basename "$WEEK_MARKER") present: skipping redundant network sync ==="
  python3 "$REPO_ROOT/scripts/appsumo_buy_advisor.py" "$@"
  echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] AppSumo Weekly Buy Advisor Cadence Gracefully Skipped ==="
  exit 0
fi

# 2. Sync live deals catalog to local SQLite (only when active run is needed)
"$REPO_ROOT/appsumo" deals sync || true

# 3. Run asset-aware Buy Advisor & Deduplication Engine
python3 "$REPO_ROOT/scripts/appsumo_buy_advisor.py" "$@"

echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] AppSumo Weekly Buy Advisor Cadence Completed Successfully ==="
