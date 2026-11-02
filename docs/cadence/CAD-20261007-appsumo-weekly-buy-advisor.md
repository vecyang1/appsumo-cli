# Cadence Record

### `CAD-20261007-appsumo-weekly-buy-advisor` - AppSumo Weekly Buy Advisor & Asset Deduplication

| Field | Value |
|---|---|
| Status | `active` |
| Activation Status | `active` |
| Activation Mode | `agent_self_schedule` |
| Created | `2026-10-07` |
| Updated | `2026-11-02` |
| Created By | `Vec + Antigravity Agent` |
| Planned By | `Vec + Antigravity Agent` |
| Project Ref | `Weekly AppSumo deal scanning and recommendation engine cross-referenced with 67+ purchased SaaS assets and local developer skills; co-maintains cloud Notion hub (reads human wishlist/blacklist/new assets without erasing notes); strictly filters duplicate wheels across full live catalog (461+ deals); recommends nothing if no capability gaps exist; syncs to Notion` |
| Execution Root | `{A_CODING_ROOT}/26.05.23-appsumo-cli` |
| Root Alias | `A_CODING_ROOT` |
| Runtime Project Label | `appsumo-weekly-buy-advisor` |
| Primary Runtime | `antigravity` |
| Runtime Systems | `Antigravity sidecar appsumo-weekly-buy-advisor; scripts/run_appsumo_buy_advisor.sh; scripts/appsumo_buy_advisor.py; appsumo CLI; Memory Center` |
| Runtime Locator | `Antigravity → Scheduled Tasks → AppSumo Weekly Buy Advisor` (`appsumo-weekly-buy-advisor`) |
| Canonical Path | `config/sidecars/appsumo-weekly-buy-advisor/sidecar.json` |
| Schedule Frequency | `weekly` |
| Schedule Expression | `0 9 * * 1` (09:00 Asia/Bangkok weekly Monday) |
| Timezone | `Asia/Bangkok` |
| Preferred Window | `morning_review` |
| Credit Policy | `prefer_midnight_5h_cap` |
| Catch Up Policy | `catch_up_when_awake` |
| Retry Policy | `retry_3_times` |
| Retry Interval Minutes | `30` |
| Max Catch Up Age Hours | `48` |
| Execution Mode | `incremental` |
| State Owner | `project_local` |
| Checkpoint Path | `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/state.json` |
| Success Marker Pattern | `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/YYYY-WW.success` |
| Output Owner | `Notion Cloud Hub 3f2e1b43-2393-815a-97e2-e06c9efcbfdc; local SQLite deals snapshot; .run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/receipts/YYYY-MM-DD.md` |
| Configuration Owner | `this card; scripts/appsumo_buy_advisor.py` |
| Receipt Pattern | `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/receipts/YYYY-MM-DD.md` |
| Evidence / Dedupe Rule | `Aggressive weekly scan of full AppSumo deal catalog matching 67+ owned assets from appsumo_purchased_assets.md and project-capabilities.md; co-maintains cloud Notion hub by reading Zone 1 Human Directives (wishlist/blacklist/new assets) and non-destructively updating Zone 2 Latest Verdict & Zone 3 Archive; strictly eliminates duplicate wheels and commodities; recommends nothing if no gap exists` |
| Activation Gate | `tested 2026-11-02: aggressive full-catalog crawl audited 461 candidate deals with complete pagination verification (461/461 unique deals); co-maintains cloud doc 3f2e1b43-2393-815a-97e2-e06c9efcbfdc with bi-directional human feedback ingestion (wishlist boost + blacklist veto verified live); 38/38 unit tests pass; exit 0` |
| Failure Receipt Owner | `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/receipts/YYYY-MM-DD.md` |
| Resume Policy | `incremental skip via weekly success marker; if forced, re-scan and merge` |
| Prior Track | `CAD-20260910-appsumo-deal-monitor` |
| Stop Condition | `weekly deal scan and asset deduplication finishes, decisions recorded in durable receipts, Notion cloud doc co-maintained, and weekly success marker touched` |
| Stale After Hours | `168` |
| Max Runtime Minutes | `10` |
| Side Effect Level | `writes_local` |
| Requires Human Review | `false` |
| Network Required | `true` |
| Manual Paste Required | `false` |
| Manual Paste Payload | `not_required_sidecar_active` |
| Source Refs | `config/sidecars/appsumo-weekly-buy-advisor/sidecar.json; scripts/appsumo_buy_advisor.py` |
| Review Judgment | `Active weekly AppSumo purchase advisor: prevents impulse buying and redundant SaaS accumulation by cross-checking all candidate lifetime deals against Vec's 66+ existing tools and skills across full catalog (338+ deals). Zero recommendation when no gaps exist; high-signal recommendation when genuine gaps appear.` |

#### Prompt

```text
AppSumo 每周资产对账与采购顾问 / AppSumo Weekly Buy Advisor & Asset Deduplication
Antigravity project label: appsumo-weekly-buy-advisor
Working directory: .
Cadence card: CAD-20261007-appsumo-weekly-buy-advisor

Language requirement: All conversation replies, findings, and summary messages to the user must be in Chinese (全程使用中文回复与输出汇报).

Before doing work:
# working directory is repository root

执行 AppSumo 每周资产对账与采购决策体系：

1. 增量防空转门禁与状态核验：
   - 检查本周原子标记：`.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/$(date +%Y-W%W).success`。
   - 若本周任务已完成且无需强刷，输出简短 3 行以内的 `no_change` 凭据后直接优雅退出。

2. 读取云端协同文档与意向注入 (Cloud Co-maintenance Intake)：
   - 从 Notion 协同中枢（https://app.notion.com/p/cadence-AppSumo-3f2e1b432393815a97e2e06c9efcbfdc）读取【人工采购意向与协同反馈区】。
   - 解析用户填写的【关注意向清单 (Wishlist)】（对符合品类与场景赋予 +50.0 分加权）、【黑名单禁令 (Blacklist)】（绝对拦截不买品类）与【新购资产录入 (Newly Acquired)】（作为已购资产参与防重），将其作为本周运行最高优先级输入。

3. 执行核心对账与采购决策流水线：
   - 运行资产感知推荐引擎：
     bash scripts/run_appsumo_buy_advisor.sh
   - 核心防重与空白能力对账策略：
     - 资产比对：读取 `05 - Memory Center/tools/appsumo_purchased_assets.md` 中的 67+ 款已购 SaaS 资产底册及云端新录入资产。
     - 轮子去重：对齐 Zapier/Make（已有 Albato+n8n）、Trustpilot（已有 TrustMate）、OptinMonster（已有 Taku）、Buffer（已有 Zernio）、ThriveCart（已有 Zylvie）等竞品标的，坚决拦截重复轮子。
     - 质量门禁：过滤低杠杆 Canva 模板包、电子书、商业课程及开发机已有原生替代品。
     - 采购准则：若本周无真正填补空白能力的优质软件，坚决不推销任何 Deal（输出 0 推荐结论，保护注意力与资金）。

4. 云端人机协同非破坏性更新与留痕 (Bi-directional Sync & Archive)：
   - 严格保护【人工采购意向与协同反馈区】，绝不抹除用户手工编辑的内容。
   - 自动将上期推荐结果归档沉淀至【往期对账与决策留痕 (Historical Archive)】折叠块。
   - 刷新更新【本周全量资产对账与推荐巡检】板块，并原子更新任务状态为 Done。
   - 生成带日期的 Markdown 决策凭据至 `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/receipts/YYYY-MM-DD.md`。
   - 更新持久化游标 `.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/state.json`。
   - 写入本周原子成功标记：`.run/cadence/CAD-20261007-appsumo-weekly-buy-advisor/$(date +%Y-W%W).success`。

5. 极简中文汇报：
   - 无采购需求：输出 3 行以内结论（“已对账全量 460+ Deals，全部与既有轮子功能重叠，本周建议 0 采购”）。
   - 发现真实空白机会：列出推荐产品、价格、核心功能、为何既有资产无法覆盖、预期商业价值与购买链接。

闭环自愈与演进要求：
- CI/CD 巡检：运行 `gh run list --limit 3` 检查远程 GitHub Actions 构建状态；若最新 run 失败，执行 `gh run view --log-failed` 定位原因并就地修复推送，确保远端流水线持续绿灯。
- 重试成功脆弱点复盘与默认固化演进：凡遇执行失败后经重试成功（Retry & Succeed）或就地自愈修复的情况，严禁仅做临时修补！必须自动启动“核心脆弱点复盘”：深入剖析为何首次会发生事故（如超时预算过紧、并发锁竞争、网络抖动无退避、凭据衰减、未做前置探活等），并在执行凭据（reports/runs/）与运行日志中输出明确的【核心脆弱点复盘】与【建议加固方案】；更必须默认执行固化演进（Evolve by Default），就地加固底层脚本（增加超时缓冲/指数退避/锁隔离）或更新 Cadence Card 契约，杜绝同类隐形事故在无人值守时再次发生。
- 视觉巡检与真实端到端演练（ego-browser）：涉及 BuildFast 博客页面生成、SEO 文章发布或前端/结算交互流程时，必须在页面生成/构建或发布后通过 `ego-browser` 进行真实视觉巡检，并在 `reports/screenshots/` 落盘带日期的真实全屏截图与渲染凭据，持续测试调优直至界面排版、封面图片与转化 CTA 完全正常（E2E tested till good）；已获用户永久全权预授权自主完成登录、OAuth 授权与免密放行，严禁擅自挂起；操作强制采用智能体编程式方案优先（Agentic Programmatic Way：API/CLI/脚本优先，浏览器内采用 Tier-1 page.evaluate() 原子化 DOM 批处理，消灭逐点低效鼠标模拟以保障极致速度与确定性高精度）。
- 真实对账与两端验证：写完从真实环境对答案，必须生成真实落地凭据。
- 极简汇报与人类注意力保护：默认极简汇报或优雅跳过（Protect Human Attention）。
- 自治目标与抗抖闭环：以 `/goal` 模式闭环执行。
- 系统演进与防腐化写回：将能力收敛至原生工具链（Extend than fork）；更新 CHANGELOG 并同步写回底册（SSOT）。
```
