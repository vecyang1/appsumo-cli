#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"
LOG_DIR="$REPO_ROOT/logs"
mkdir -p "$LOG_DIR"

echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] Starting AppSumo Deal Monitor Cadence ==="

# 1. Sync catalog to local SQLite
./appsumo deals sync

# 2. Diff changes against previous snapshot
./appsumo deals diff || true

# 3. Sync ideal deal recommendations to Notion
./appsumo deals sync-notion --min-rating 4.8 --min-reviews 50 --limit 10

echo "=== [$(date '+%Y-%m-%d %H:%M:%S')] AppSumo Deal Monitor Finished Successfully ==="
