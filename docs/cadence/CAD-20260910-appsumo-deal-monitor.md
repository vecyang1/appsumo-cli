# Cadence Record

### `CAD-20260910-appsumo-deal-monitor` - AppSumo Daily Catalog Sync & Ideal Deal Monitor

- `Status`: active
- `Activation Status`: active
- `Activation Mode`: self-scheduled
- `Created`: 2026-09-10
- `Updated`: 2026-09-10
- `Created By`: Vec + Antigravity Agent
- `Planned By`: Antigravity Pair Programmer
- `Project Ref`: `26.05.23-appsumo-cli`
- `Execution Root`: `$PROJECT_DIR`
- `Root Alias`: `{A_CODING}/26.05.23-appsumo-cli`
- `Primary Runtime`: LaunchAgent / local cron
- `Runtime Systems`: launchd, antigravity
- `Runtime Locator`: local cron / launchd `com.appsumo.deal-monitor` (root: `26.05.23-appsumo-cli`)
- `Schedule Frequency`: daily
- `Schedule Expression`: `0 9 * * *` (09:00 AM UTC+7)
- `Timezone`: Asia/Bangkok
- `Preferred Window`: 09:00 - 09:30
- `Credit Policy`: normal
- `Catch Up Policy`: run_latest_only
- `Retry Policy`: exponential_backoff
- `Retry Interval Minutes`: 15
- `Max Catch Up Age Hours`: 24
- `Execution Mode`: incremental
- `State Owner`: SQLite database `appsumo.db` (tables: `deals`, `deal_snapshots`)
- `Output Owner`: Notion Page `https://app.notion.com/p/Appsumo-CLI-appsumo-cli-3d3e1b43239381f1a87dc75cb340e5e1`
- `Side Effect Level`: low (read-only crawl of public catalog, local SQLite writes, Notion append)
- `Requires Human Review`: false
- `Network Required`: true (appsumo.com public API, api.notion.com)
- `Configuration Owner`: `$NOTION_ENV_PATH` / `.env`
- `Checkpoint Path`: `.run/cadence/CAD-20260910-appsumo-deal-monitor/state.json`
- `Success Marker Pattern`: `runs/YYYY-MM-DD.json`
- `Resume Policy`: restart_from_last_successful_sync
- `Stop Condition`: no-op if catalog unchanged or API unreachable after 3 retries

#### Execution Steps

1. Snapshot live public AppSumo deal catalog:
   ```bash
   ./appsumo deals sync
   ```
2. Detect deal movements, price changes, and stock countdowns:
   ```bash
   ./appsumo deals diff
   ```
3. Extract top ideal products:
   ```bash
   ./appsumo deals ideal --min-rating 4.8 --min-reviews 50 --limit 10
   ```
4. Sync recommendations to Notion:
   ```bash
   python3 scripts/sync_notion_recommendations.py
   ```
