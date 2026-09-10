#!/usr/bin/env python3
"""
Sync AppSumo ideal product recommendations and capability guide to Notion.
Page: Appsumo CLI (appsumo-cli) - 3d3e1b43-2393-81f1-a87d-c75cb340e5e1
"""

import json
import os
import subprocess
import sys
import urllib.error
import urllib.request

NOTION_PAGE_ID = "3d3e1b43-2393-81f1-a87d-c75cb340e5e1"
ENV_PATH = "/Users/vecsatfoxmailcom/.gemini/antigravity/skills/notion-mcp-connector/.env"

def get_notion_token():
    token = os.getenv("NOTION_TOKEN") or os.getenv("NOTION_API_KEY")
    if token:
        return token
    if os.path.exists(ENV_PATH):
        with open(ENV_PATH) as f:
            for line in f:
                if line.startswith("NOTION_TOKEN=") or line.startswith("NOTION_API_KEY="):
                    return line.split("=", 1)[1].strip().strip("\"'")
    raise ValueError("Notion token not found in environment or .env file")

def fetch_ideal_deals():
    # Run the local appsumo CLI to get verified live deals
    cmd = ["./appsumo", "deals", "ideal", "--min-rating", "4.8", "--min-reviews", "50", "--limit", "10", "--json"]
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
    token = get_notion_token()
    deals = fetch_ideal_deals()
    print(f"Fetched {len(deals)} ideal deals from AppSumo CLI")

    blocks = []

    # Overview Callout
    blocks.append(make_callout(
        "AppSumo CLI 原生工具链已全面升级：支持 Elasticsearch 实时搜索 (query 关键词)、按真实评价排序 (sort=rating)、优质 Deal 自动化探测 (ideal 筛选评分>=4.8且评论>=50)、以及 SQLite 本地全量快照离线检索与双向比对。",
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
        "# 1. 探测优质 Deal (评分 >= 4.8 且评论 >= 50)\n"
        "appsumo deals ideal --min-rating 4.8 --min-reviews 50 --limit 10\n\n"
        "# 2. 实时按关键词检索（例如 SEO 工具）\n"
        "appsumo deals search seo\n\n"
        "# 3. 全量快照同步与本地检索\n"
        "appsumo deals sync\n"
        "appsumo deals search video --local\n"
        "appsumo search seo --deals\n\n"
        "# 4. 监测目录变化（价格调整、库存移动、新上线产品）\n"
        "appsumo deals diff"
    ))

    # Ideal Deals Recommendations
    blocks.append(make_h2("三、精选 AppSumo 优质产品推荐榜单 (Ideal Deals Top 10)"))
    blocks.append(make_paragraph([
        text_rt("筛选条件：", bold=True),
        text_rt("平均评分 >= 4.80 星，买家真实评价数 >= 50 条，具备明确终身授权 (Lifetime Deal) 与极高性价比。")
    ]))
    blocks.append(make_divider())

    for idx, d in enumerate(deals, start=1):
        name = d.get("name", "Unknown")
        slug = d.get("slug", "")
        price = d.get("price", 0)
        orig_price = d.get("original_price", 0)
        rating = d.get("average_rating", 0)
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
            text_rt(f" (官方原价 ${orig_price:.2f}，立省 {discount_pct}%) | "),
            text_rt("📂 分类: ", bold=True),
            text_rt(category),
            text_rt(" | "),
            text_rt("🔗 直达链接: ", bold=True),
            text_rt("点击前往 AppSumo", link=deal_url)
        ]
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

    # Push to Notion in chunks of 50 blocks (Notion API limit per request is 100)
    chunk_size = 40
    for i in range(0, len(blocks), chunk_size):
        chunk = blocks[i:i + chunk_size]
        print(f"Appending chunk {i // chunk_size + 1} ({len(chunk)} blocks)...")
        append_blocks_chunk(token, NOTION_PAGE_ID, chunk)

    print("Successfully synced all recommendations and guide to Notion!")

if __name__ == "__main__":
    main()
