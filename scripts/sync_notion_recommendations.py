#!/usr/bin/env python3
"""
Sync AppSumo ideal product recommendations and capability guide to Notion.
Default Page: Appsumo CLI (appsumo-cli) - 3d3e1b43-2393-81f1-a87d-c75cb340e5e1
"""

import argparse
import datetime
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path

DEFAULT_PAGE_ID = "3d3e1b43-2393-81f1-a87d-c75cb340e5e1"
ENV_PATH = Path.home() / ".gemini/antigravity/skills/notion-mcp-connector/.env"

def get_appsumo_binary(explicit_bin=None):
    if explicit_bin and os.path.isfile(explicit_bin) and os.access(explicit_bin, os.X_OK):
        return explicit_bin
    if os.getenv("APPSUMO_BIN"):
        return os.getenv("APPSUMO_BIN")
    # Resolve relative to script location (26.05.23-appsumo-cli/appsumo)
    repo_bin = Path(__file__).resolve().parent.parent / "appsumo"
    if repo_bin.is_file() and os.access(repo_bin, os.X_OK):
        return str(repo_bin)
    # Check PATH
    which_bin = shutil.which("appsumo")
    if which_bin:
        return which_bin
    return str(repo_bin)

def get_notion_token(explicit_token=None):
    if explicit_token:
        return explicit_token
    token = os.getenv("NOTION_TOKEN") or os.getenv("NOTION_API_KEY")
    if token:
        return token
    if ENV_PATH.exists():
        with open(ENV_PATH) as f:
            for line in f:
                if line.startswith("NOTION_TOKEN=") or line.startswith("NOTION_API_KEY="):
                    return line.split("=", 1)[1].strip().strip("\"'")
    raise ValueError("Notion token not found in arguments, environment, or .env file")

def fetch_ideal_deals(bin_path, min_rating=4.8, min_reviews=50, limit=10, local=False):
    cmd = [
        bin_path, "deals", "ideal",
        "--min-rating", str(min_rating),
        "--min-reviews", str(min_reviews),
        "--limit", str(limit),
        "--json"
    ]
    if local:
        cmd.append("--local")
    proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
    data = json.loads(proc.stdout)
    return data.get("deals", [])

def text_rt(content, bold=False, italic=False, code=False, color="default", link=None):
    obj = {
        "type": "text",
        "text": {"content": content, "link": {"url": link} if link else None},
        "annotations": {
            "bold": bold,
            "italic": italic,
            "strikethrough": False,
            "underline": False,
            "code": code,
            "color": color
        }
    }
    return obj

def make_callout(text, icon="💡"):
    return {
        "object": "block",
        "type": "callout",
        "callout": {
            "icon": {"type": "emoji", "emoji": icon},
            "rich_text": [text_rt(text)]
        }
    }

def make_h2(text):
    return {
        "object": "block",
        "type": "heading_2",
        "heading_2": {
            "rich_text": [text_rt(text, bold=True)]
        }
    }

def make_h3(text):
    return {
        "object": "block",
        "type": "heading_3",
        "heading_3": {
            "rich_text": [text_rt(text, bold=True)]
        }
    }

def make_paragraph(parts):
    return {
        "object": "block",
        "type": "paragraph",
        "paragraph": {
            "rich_text": parts
        }
    }

def make_bullet(parts):
    return {
        "object": "block",
        "type": "bulleted_list_item",
        "bulleted_list_item": {
            "rich_text": parts
        }
    }

def make_divider():
    return {
        "object": "block",
        "type": "divider",
        "divider": {}
    }

def make_code(content, language="bash"):
    return {
        "object": "block",
        "type": "code",
        "code": {
            "language": language,
            "rich_text": [text_rt(content)]
        }
    }

def append_blocks_chunk(token, page_id, blocks):
    url = f"https://api.notion.com/v1/blocks/{page_id}/children"
    req = urllib.request.Request(
        url,
        data=json.dumps({"children": blocks}).encode("utf-8"),
        method="PATCH",
        headers={
            "Authorization": f"Bearer {token}",
            "Notion-Version": "2022-06-28",
            "Content-Type": "application/json"
        }
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))

def main():
    parser = argparse.ArgumentParser(description="Sync AppSumo ideal product recommendations to Notion")
    parser.add_argument("--page-id", default=os.getenv("NOTION_PAGE_ID", DEFAULT_PAGE_ID), help="Notion page ID")
    parser.add_argument("--token", default=None, help="Notion API Token")
    parser.add_argument("--min-rating", type=float, default=4.8, help="Minimum average rating")
    parser.add_argument("--min-reviews", type=int, default=50, help="Minimum review count")
    parser.add_argument("--limit", type=int, default=10, help="Number of deals to recommend")
    parser.add_argument("--local", action="store_true", help="Query local SQLite DB instead of live API")
    parser.add_argument("--bin", default=None, help="Path to appsumo CLI binary")
    args = parser.parse_args()

    token = get_notion_token(args.token)
    appsumo_bin = get_appsumo_binary(args.bin)
    
    print(f"Using AppSumo binary: {appsumo_bin}")
    deals = fetch_ideal_deals(
        appsumo_bin,
        min_rating=args.min_rating,
        min_reviews=args.min_reviews,
        limit=args.limit,
        local=args.local
    )
    print(f"Fetched {len(deals)} ideal deals from AppSumo CLI")

    now_str = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    blocks = []

    # Overview Callout
    blocks.append(make_callout(
        f"AppSumo 优质 Deal 实时推荐榜单（更新时间：{now_str}）\n"
        f"基于 AppSumo CLI 原生工具链自动探测与筛选，本期共精选 {len(deals)} 款评分≥{args.min_rating}且评价数≥{args.min_reviews}条的顶级终身授权软件。",
        icon="🚀"
    ))

    # Architecture & Contract Discovery
    blocks.append(make_h2("一、契约发现与原生工具链设计 (Architecture & Contract)"))
    blocks.append(make_bullet([
        text_rt("单一事实来源 (SSOT)：", bold=True),
        text_rt(" 公开 Deal 浏览与搜索端点无需买家 Cookie，结构性走公共路由避免泄露买家身份。")
    ]))
    blocks.append(make_bullet([
        text_rt("API 真实参数验证：", bold=True),
        text_rt(" AppSumo Elasticsearch browse 接口 "),
        text_rt("/api/v2/deals/esbrowse/", code=True),
        text_rt(" 严格接受 "),
        text_rt("query=<keyword>", code=True),
        text_rt("（忽略 q 参数）；支持 "),
        text_rt("sort=rating", code=True),
        text_rt(" 返回经社群充分验证的高分工具。")
    ]))
    blocks.append(make_bullet([
        text_rt("本地 SQLite 快照：", bold=True),
        text_rt(" 存储包含简介、核心价值、对标竞品、适用场景的完整结构，支持毫秒级离线快速检索。")
    ]))

    # Commands
    blocks.append(make_h2("二、CLI 统一命令入口 (Command Reference)"))
    blocks.append(make_code(
        "# 1. 探测优质 Deal (评分 >= 4.8 且评论 >= 50，支持中文与卡片排版)\n"
        "appsumo deals ideal --min-rating 4.8 --min-reviews 50 --limit 10 --chinese\n\n"
        "# 2. 实时按关键词检索（例如 SEO 工具）\n"
        "appsumo deals search seo\n\n"
        "# 3. 全量快照同步与本地检索\n"
        "appsumo deals sync\n"
        "appsumo deals search video --local\n"
        "appsumo search seo --deals\n\n"
        "# 4. 监测目录变化（价格调整、库存移动、新上线产品）\n"
        "appsumo deals diff\n\n"
        "# 5. 一键同步推荐至 Notion\n"
        "appsumo deals sync-notion --limit 10"
    ))

    # Ideal Deals Recommendations
    blocks.append(make_h2(f"三、精选 AppSumo 优质产品推荐榜单 (Ideal Deals Top {len(deals)})"))
    blocks.append(make_paragraph([
        text_rt("筛选条件：", bold=True),
        text_rt(f"平均评分 >= {args.min_rating:.2f} 星，买家真实评价数 >= {args.min_reviews} 条，具备明确终身授权 (Lifetime Deal) 与极高性价比。")
    ]))
    blocks.append(make_divider())

    for idx, d in enumerate(deals, start=1):
        name = d.get("name") or d.get("slug", "Unknown")
        slug = d.get("slug", "")
        price = d.get("price", 0.0)
        orig_price = d.get("original_price", 0.0)
        rating = d.get("average_rating", 0.0)
        reviews = d.get("review_count", 0)
        category = d.get("category", "")
        desc = d.get("card_description", "")
        value_prop = d.get("value_prop", "")
        best_for = ", ".join(d.get("best_for", []))
        alt_to = ", ".join(d.get("alternative_to", []))
        deal_url = f"https://appsumo.com/products/{slug}/"
        
        discount_pct = 0
        if orig_price > price and orig_price > 0:
            discount_pct = round(((orig_price - price) / orig_price) * 100)

        # Header for each product
        blocks.append(make_h3(f"{idx}. {name} — ★ {rating:.2f} ({reviews} 条评价)"))
        
        # Details
        meta_parts = [
            text_rt("💰 终身价格: ", bold=True),
            text_rt(f"${price:.2f}", bold=True, color="green"),
        ]
        if discount_pct > 0:
            meta_parts.append(text_rt(f" (官方原价 ${orig_price:.2f}，立省 {discount_pct}%) | "))
        else:
            meta_parts.append(text_rt(" | "))
        if category:
            meta_parts.extend([
                text_rt("📂 分类: ", bold=True),
                text_rt(f"{category} | ")
            ])
        meta_parts.extend([
            text_rt("🔗 直达链接: ", bold=True),
            text_rt("点击前往 AppSumo", link=deal_url)
        ])
        blocks.append(make_paragraph(meta_parts))

        if value_prop or desc:
            blocks.append(make_bullet([
                text_rt("核心亮点: ", bold=True),
                text_rt(value_prop or desc)
            ]))
        if desc and value_prop and desc != value_prop:
            blocks.append(make_bullet([
                text_rt("产品简介: ", bold=True),
                text_rt(desc)
            ]))
        if best_for:
            blocks.append(make_bullet([
                text_rt("最适合人群: ", bold=True),
                text_rt(best_for)
            ]))
        if alt_to:
            blocks.append(make_bullet([
                text_rt("对标知名竞品: ", bold=True),
                text_rt(alt_to, italic=True)
            ]))
        
        blocks.append(make_divider())

    # Cadence card reference
    blocks.append(make_h2("四、自动化定时监控体系 (Scheduled Cadence)"))
    blocks.append(make_paragraph([
        text_rt("通过 "),
        text_rt("scheduled-task-rescheduler", code=True),
        text_rt(" 保持数据新鲜度：每日自动执行一次 "),
        text_rt("appsumo deals sync", code=True),
        text_rt(" 存储快照，触发 "),
        text_rt("appsumo deals diff", code=True),
        text_rt(" 自动检测优质 Deal 的库存告急与价格变动，并实时推送更新。")
    ]))

    chunk_size = 40
    for i in range(0, len(blocks), chunk_size):
        chunk = blocks[i:i + chunk_size]
        print(f"Appending chunk {i // chunk_size + 1} ({len(chunk)} blocks)...")
        append_blocks_chunk(token, args.page_id, chunk)

    print(f"Successfully synced all {len(deals)} recommendations and guide to Notion page {args.page_id}!")

if __name__ == "__main__":
    main()
