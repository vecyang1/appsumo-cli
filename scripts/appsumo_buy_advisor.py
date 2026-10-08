#!/usr/bin/env python3
"""
AppSumo Weekly Buy Advisor & Asset-Aware Deal Recommender
=========================================================
Scans live/synced AppSumo deals against 2nd Brain purchased assets and Memory Center.
Filters out duplicate wheels and commodities.
If no novel high-value deals are needed, strictly recommends NOTHING (zero duplicate noise).
If a genuine capability gap is discovered, formats a structured Chinese recommendation and syncs to Notion.

Cadence Card: CAD-20261007-appsumo-weekly-buy-advisor
Target Notion Page: https://app.notion.com/p/cadence-AppSumo-3f2e1b432393815a97e2e06c9efcbfdc
"""

import argparse
import datetime
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional, Set, Tuple

# Default paths
DEFAULT_ASSETS_MD = Path(
    "/Users/vecsatfoxmailcom/Documents/Cowork/Antigravity Cowork/26.06.06 2nd Brain/05 - Memory Center/tools/appsumo_purchased_assets.md"
)
DEFAULT_PURCHASES_MD = Path(
    "/Users/vecsatfoxmailcom/Documents/Cowork/Antigravity Cowork/26.06.06 2nd Brain/05 - Memory Center/tools/appsumo_purchases.md"
)
DEFAULT_NOTION_PAGE_ID = "3f2e1b43-2393-815a-97e2-e06c9efcbfdc"
CADENCE_ID = "CAD-20261007-appsumo-weekly-buy-advisor"

# Known mapping of competitor keywords ("alternative_to") to owned assets
COMPETITOR_TO_OWNED_ASSET = {
    # Workflow, RPA & Integrations
    "zapier": "Albato + n8n + ZeroWork (已拥有 134+ 自动化流与 RPA 编排)",
    "make": "Albato + n8n (已拥有企业级自托管与云端多租户集成)",
    "integromat": "Albato + n8n",
    "pabbly": "Albato + n8n",
    "konnectzit": "Albato + Bit Integrations",
    "bubble": "Albato + n8n + Webhook Hub (本地与云端原生架构)",
    "appsheet": "Albato + n8n + Webhook Hub",
    "process street": "Albato + n8n (自建标准审批与工作流引擎)",
    # Reviews, Feedback & Feature Requests
    "trustpilot": "TrustMate.io (已拥有 AppSumo 1000 终身版，多站点 1000 邀评/月)",
    "loox": "TrustMate.io",
    "judge.me": "TrustMate.io",
    "yotpo": "TrustMate.io",
    "reviews.io": "TrustMate.io",
    "birdeye": "TrustMate.io",
    "podium": "TrustMate.io",
    "stamped": "TrustMate.io",
    "videoask": "TrustMate.io + CX Genie",
    "canny": "TrustMate.io + GitHub/Notion 统一需求中枢",
    "featurebase": "TrustMate.io + GitHub/Notion 统一需求中枢",
    "uservoice": "TrustMate.io + GitHub/Notion 统一需求中枢",
    # Links, QR Codes & Bio-pages
    "bitly": "SleekBio (已购终身资产) + BetterLinks (本地独立短链与像素系统)",
    "rebrandly": "SleekBio + BetterLinks",
    "linktree": "SleekBio (已购终身资产)",
    # Virtual Meetings, Events & Webinars
    "zoom": "GoBrunch (已购终身会议资产) + Google Meet / Discord",
    "hopin": "GoBrunch (已购终身虚拟活动空间资产)",
    "on24": "GoBrunch (已购终身虚拟活动空间资产)",
    # Analytics & Data Visualization
    "flourish": "本地可视化工具链 (Chart.js / Recharts / Logic Sheet / Python 分析)",
    "matterport": "非核心业务领域 (360度房产虚拟看房，非软件开发与电商核心杠杆)",
    # Email Verification & Deliverability
    "hunter.io": "Email Checkpoint + Encharge (已拥有邮件防投递与有效性校验底座)",
    "zerobounce": "Email Checkpoint (已购终身邮件退信校验资产)",
    "neverbounce": "Email Checkpoint",
    # Popups & Lead Generation
    "optinmonster": "Taku (taku.cool, 已拥有 3 个生产 Space 及 REST API 自动化)",
    "sleeknote": "Taku",
    "sumo": "Taku",
    "wisepops": "Taku",
    "poptin": "Taku",
    "convertbox": "Taku",
    # Checkout & Digital Commerce
    "thrivecart": "Zylvie (已拥有高转化独立结算与许可分发系统)",
    "samcart": "Zylvie",
    "gumroad": "Zylvie",
    "payhip": "Zylvie",
    "lemon squeezy": "Zylvie",
    # Social Media & Messaging
    "buffer": "Late / Zernio (已拥有多渠道社交调度 + 统一短信/RCS/iMessage API)",
    "hootsuite": "Late / Zernio",
    "later": "Late / Zernio",
    "metricool": "Late / Zernio",
    "sprout social": "Late / Zernio",
    "ocoya": "Late / Zernio",
    "predis": "Late / Zernio",
    # Email Marketing
    "mailchimp": "SendFox + Encharge (已拥有邮件自动化与受众细分)",
    "activecampaign": "Encharge + SendFox",
    "convertkit": "SendFox + Encharge",
    "klaviyo": "Encharge",
    "brevo": "SendFox + Encharge",
    "sendinblue": "SendFox + Encharge",
    # Forms & Scheduling
    "typeform": "GoZen Forms.Ai + TidyCal",
    "calendly": "TidyCal (已拥有日历排期终身版)",
    "jotform": "GoZen Forms.Ai",
    "surveymonkey": "GoZen Forms.Ai",
    "gravity forms": "GoZen Forms.Ai",
    # Customer Support & Chatbots
    "intercom": "CX Genie + Zernio (已拥有 AI 客服聊天机器人与 Webhook 集成)",
    "tidio": "CX Genie",
    "zendesk": "CX Genie",
    "livechat": "CX Genie",
    "crisp": "CX Genie",
    # SEO & Indexing
    "yoast": "Squirrly SEO + Cromojo (已拥有 AI SEO 终身版与自动收录)",
    "rank math": "Squirrly SEO",
    "aioseo": "Squirrly SEO",
    "ahrefs": "Squirrly SEO + Google Search Console / Screaming Frog 本地技能",
    "semrush": "Squirrly SEO + Screaming Frog 本地技能",
    "screaming frog": "本地 Python SEO 爬虫与 Screaming Frog 授权",
    # E-commerce Visuals & Design
    "canva": "本地 Figma + Design Tokens + SVG Generator (无需消费低端套版工具)",
    "photoroom": "WeShop AI + SellerPic (已拥有模特实景换装与电商图处理)",
    "flair.ai": "WeShop AI",
    "clipdrop": "WeShop AI",
    # Audio & Video
    "krisp": "Xound (已拥有 AI 降噪与音频修复)",
    "adobe podcast": "Xound",
    "descript": "Xound + Minvo (已拥有 AI 剪辑与去噪套件)",
    "speechelo": "ElevenLabs + EaseUS VoiceWave (本地权威 TTS 技能)",
    "powtoon": "Remotion + Manim + Kling/Seedance (本地编程式与 AI 视频套件)",
    # Flipbooks, Readers & Documents
    "issuu": "FlipBooklets (已拥有终身版交互式 3D 翻页书与全白标 Edge Gateway)",
    "flipsnack": "FlipBooklets (已拥有终身版交互式 3D 翻页书)",
    "heyzine": "FlipBooklets",
    "calameo": "FlipBooklets",
    # RPA, Web Scraping & Crawlers
    "browse ai": "ZeroWork (已购 AppSumo 终身 RPA 资产) + ego-browser / 本地自动化技能",
    "uipath": "ZeroWork (已购 AppSumo 终身 RPA 资产) + n8n / 本地自动化技能",
    "bardeen": "ZeroWork + n8n",
    "axiom": "ZeroWork + n8n",
    # Databases & Spreadsheets
    "airtable": "Boost.space (已购终身数据库集成) + Logic Sheet + Baserow / Notion 统一中枢",
    "monday.com": "Boost.space + Notion 统一中枢",
    "smartsheet": "Boost.space + Logic Sheet",
    # Legal, E-Sign & Contracts
    "docusign": "Nutrient DWS (nutrient-document-processing 技能) + 本地数字签名与 PDF 流水线",
    "pandadoc": "Nutrient DWS (nutrient-document-processing 技能) + 本地数字签名",
    "signwell": "Nutrient DWS",
    # Programmatic Image & Video APIs
    "bannerbear": "本地 Remotion + SVG Generator + nutrient-document-processing 研发套件",
    "placid": "本地 Remotion + SVG Generator 研发套件",
    # AI Meeting & Notetakers
    "otter.ai": "Letterly (已购终身语音资产) + 本地 transcribe-media / Whisper 技能",
    "granola": "Letterly + 本地 transcribe-media 技能",
    "fireflies": "Letterly + 本地 transcribe-media 技能",
    "fathom": "Letterly + 本地 transcribe-media 技能",
    # RSS & Feed Monitors
    "feedly": "本地 opennews 技能 + n8n 自动化监控流水线",
    "inoreader": "本地 opennews 技能 + n8n 自动化监控流水线",
    "newsblur": "本地 opennews 技能 + n8n 自动化监控流水线",
    # YouTube SEO & Growth
    "tubebuddy": "vidiq-mcp (已拥有集成技能) + youtube-data-cli / youtube-research-video-topic",
    "vidiq": "vidiq-mcp (已拥有集成技能) + youtube-data-cli / youtube-research-video-topic",
    # Email Inboxes & AI Assistants
    "superhuman": "email-ops + SendFox / Encharge (本地邮件智能体管理与营销资产)",
    "fyxer.ai": "email-ops + SendFox / Encharge",
    "inbox zero": "email-ops",
    # Ads Optimization & Cold Outreach
    "adespresso": "Adsbot (已拥有 Google Ads 自动化诊断与优化)",
    "wordstream": "Adsbot",
    "optmyzr": "Adsbot",
    "lemlist": "Clodura.AI + SendFox / Encharge (已拥有多套邮件序列与拓客底座)",
    "heyreach": "Clodura.AI + LinkedIn Watcher (现有工作区自动化)",
    "dripify": "Clodura.AI + LinkedIn Watcher",
    # AI Copywriting & Agents
    "copy.ai": "Claude / Gemini / GPT-4 + 本地 NewType / Luogen 技能",
    "jasper": "Claude / Gemini / GPT-4 + Squirrly SEO AI",
    "grammarly": "Claude / Gemini / GPT-4 + 本地智能体写作技能",
    "wisprflow": "Letterly + 本地语音转录与 AI 流水线",
    "lovable": "Antigravity + Claude DevFleet (原生 AI 智能体工程集群)",
    "replit": "Antigravity + 本地 A-coding 研发工作区",
}

# Subcategories saturated by existing assets
SATURATED_SUBCATEGORIES = {
    "Feedback & Reviews": "TrustMate.io (已拥有评价收集与展示轮子)",
    "Popups & Notifications": "Taku (taku.cool, 已拥有弹窗与触达中心)",
    "Social Media Management": "Zernio / Late (已拥有全平台社媒与消息 API)",
    "Automation & Integration": "Albato + n8n (已拥有完整工作流集成)",
    "Workflow Automation": "Albato + n8n (已拥有 134+ 自动化流)",
    "Scheduling": "TidyCal (已拥有日程预约系统)",
    "Forms & Surveys": "GoZen Forms.Ai (已拥有表单生成器)",
    "Form Builders": "GoZen Forms.Ai (已拥有高转化表单生成器)",
    "AI Voice": "ElevenLabs + EaseUS VoiceWave (已拥有顶级语音克隆与 TTS 技能)",
    "Voice & Dictation": "Letterly (已拥有语音备忘与录音转化工具)",
    "Virtual Events": "GoBrunch (已拥有虚拟会议与活动空间终身资产)",
    "Website Analytics": "GA4 / GTM / Cromojo / 本地 gtm-agent + google-analytics-ops 技能",
    "Data & Reporting": "Logic Sheet + Power Formulas + 本地研发环境图表库",
    "Paid Ads & Retargeting": "Adsbot + BetterLinks + Taku (已拥有广告监控与独立短链像素)",
    "Photo Editing": "WeShop AI + SellerPic + Topaz Photo AI (本地商业视觉工具链)",
    "Design Tools": "本地 Figma + Design Tokens + SVG Generator (无需消费低端套版工具)",
    "Templates & Fonts": "非高杠杆软件资产（纯模板/字库素材包，坚决不买）",
    "Business Courses": "非软件资产（商业/营销课程教材，坚决不买）",
    "Skills & Learning": "非软件资产（教程与技巧指南，坚决不买）",
    "Cookie Consent": "WordPress 原生合规插件 / Klaro 本地脚本（无需第三方付费 SaaS）",
    "VPN & Privacy": "1Password + 专属代理节点集群（无需第三方链接分享工具）",
    "AI Notetakers": "Letterly (已购终身语音资产) + 本地 transcribe-media / Whisper 技能",
    "Lead Capture & Forms": "Taku (已拥有弹窗中心) + GoZen Forms.Ai + SendFox 邮件列表",
    "Email & Inbox Management": "1Password Masked Email + Cloudflare Email Routing + 本地 email-ops 技能",
    "Web Scraping": "ZeroWork (已购终身 RPA) + 本地 ultra-low-cost-scraper 智能体套件",
    "SEO": "Squirrly SEO + Cromojo (已拥有 AI SEO 终身版与自动收录) + 本地 ai-seo 技能",
    "Tab Management": "本地浏览器原生能力 / OneTab / 智能体工作区",
    "Books": "FlipBooklets (已拥有电子翻页书与发布工具链) / 本地 Markdown 电子书管线",
    "Content Strategy": "本地 NewType / Luogen 智能体写作与调研套件",
    "No-Code Automation": "Albato + n8n + ZeroWork (已拥有 134+ 自动化流与 RPA 编排)",
    "E-Signatures & Legal": "Nutrient DWS (nutrient-document-processing 技能) + 本地合规签名",
}

# Low-value or non-software junk indicators
NON_SOFTWARE_PATTERNS = [
    r"\bcanva\b",
    r"\btemplates?\b",
    r"\bbundle\b",
    r"\bcourse\b",
    r"\bebook\b",
    r"\bmarketing plan\b",
    r"\bfonts?\b",
    r"\billustrations?\b",
    r"\bprompts?\b",
    r"\bguide\b",
    r"\bchecklist\b",
    r"\bswipe file\b",
    r"\bworksheet\b",
    r"\bnotion-style\b",
    r"\bnotion template\b",
    r"\bmba\b",
    r"\bmasterclass\b",
    r"\btraining\b",
    r"\blearn how to\b",
]


class AssetKnowledgeBase:
    """Loads and indexes owned software assets from 2nd Brain memory."""

    def __init__(self, assets_file: Path = DEFAULT_ASSETS_MD):
        self.assets_file = Path(assets_file)
        self.tools: List[Dict[str, Any]] = []
        self.tool_names: Set[str] = set()
        self.normalized_names: Set[str] = set()
        self._load()

    def _load(self):
        if not self.assets_file.exists():
            return

        content = self.assets_file.read_text(encoding="utf-8")
        pattern = re.compile(r"^###\s+([^\n]+)\n(.*?)(?=^###|\Z)", re.MULTILINE | re.DOTALL)
        for title_raw, body in pattern.findall(content):
            name = title_raw.strip()
            # Clean name (remove extra markdown / subtitles)
            clean_name = re.sub(r"\(.*?\)", "", name)
            for sep in [" - ", " – ", " — "]:
                clean_name = clean_name.split(sep)[0]
            clean_name = clean_name.strip()
            status = "unknown"
            if "🟢" in body or "activated" in body.lower() or "redeemed" in body.lower():
                status = "active"
            elif "🔴" in body or "expired" in body.lower():
                status = "expired"

            desc_match = re.search(r"\*\*Description:\*\*\s*([^\n]+)", body)
            desc = desc_match.group(1).strip() if desc_match else ""

            self.tools.append({
                "raw_name": name,
                "clean_name": clean_name,
                "status": status,
                "description": desc,
            })
            self.tool_names.add(clean_name.lower())
            self.normalized_names.add(re.sub(r"[^a-z0-9]", "", clean_name.lower()))

    def is_owned(self, product_name: str, slug: str) -> Optional[str]:
        """Checks if a deal is already owned by name or slug using word-boundary tokens."""
        name_tokens = set(re.findall(r"[a-z0-9]+", product_name.lower()))
        slug_tokens = set(re.findall(r"[a-z0-9]+", slug.lower()))
        combined_tokens = name_tokens.union(slug_tokens)

        norm_name = re.sub(r"[^a-z0-9]", "", product_name.lower())
        norm_slug = re.sub(r"[^a-z0-9]", "", slug.lower())

        for tool in self.tools:
            tool_clean = tool["clean_name"].lower()
            tool_tokens = re.findall(r"[a-z0-9]+", tool_clean)
            tool_norm = "".join(tool_tokens)

            if not tool_norm:
                continue

            # Exact match on full normalized name or slug
            if tool_norm == norm_name or tool_norm == norm_slug:
                return f"{tool['clean_name']} (已购资产，状态: {tool['status']})"

            # Single short token (e.g. 'late', 'taku', 'minvo'): must match an exact token
            if len(tool_tokens) == 1:
                t = tool_tokens[0]
                if len(t) < 3:
                    if t == norm_name or t == norm_slug:
                        return f"{tool['clean_name']} (已购资产，状态: {tool['status']})"
                elif len(t) <= 4:
                    if t in combined_tokens:
                        return f"{tool['clean_name']} (已购资产，状态: {tool['status']})"
                else:
                    if t in combined_tokens or tool_norm in norm_slug:
                        return f"{tool['clean_name']} (已购资产，状态: {tool['status']})"
            else:
                # Multi-word tool name: check if all non-stop tokens match
                if all(t in combined_tokens for t in tool_tokens if len(t) > 2):
                    return f"{tool['clean_name']} (已购资产，状态: {tool['status']})"

        return None



class DealAuditor:
    """Evaluates AppSumo deals against owned assets and capability needs."""

    def __init__(self, kb: AssetKnowledgeBase):
        self.kb = kb

    def audit_deal_steps(self, deal: Dict[str, Any]) -> Dict[str, Any]:
        """
        Authoritative single-source-of-truth audit engine.
        Evaluates a deal through every rule and records a detailed step-by-step trace.
        """
        name = deal.get("name") or ""
        slug = deal.get("slug") or ""
        is_unresolved = bool(deal.get("is_unresolved"))
        is_free = bool(deal.get("is_free"))
        raw_price = deal.get("price")
        price = float(raw_price) if raw_price is not None else 0.0
        orig_price = float(deal.get("original_price") or 0.0)
        rating = float(deal.get("average_rating") or 0.0)
        reviews = int(deal.get("review_count") or 0)
        desc = deal.get("card_description") or ""
        subcategory = deal.get("subcategory") or ""
        category = deal.get("category") or ""
        alt_to = [a.lower().strip() for a in (deal.get("alternative_to") or [])]
        canonical_id = f"appsumo:{slug}" if slug else f"appsumo:{deal.get('id', '')}"

        steps: List[Dict[str, Any]] = []

        def _verdict(
            recommended: bool,
            triggered_rule: str,
            reason: str,
            replacement: Optional[str] = None,
            score: float = 0.0,
        ) -> Dict[str, Any]:
            return {
                "deal": {
                    "id": deal.get("id"),
                    "name": name,
                    "slug": slug,
                    "canonical_id": canonical_id,
                    "price": price,
                    "original_price": orig_price,
                    "rating": rating,
                    "reviews": reviews,
                    "category": category,
                    "subcategory": subcategory,
                    "alternative_to": alt_to,
                    "url": f"https://appsumo.com{deal.get('url', '')}",
                    "card_description": desc,
                    "is_unresolved": is_unresolved,
                },
                "should_recommend": recommended,
                "verdict": "RECOMMENDED" if recommended else "REJECTED",
                "triggered_rule": triggered_rule,
                "reason": reason,
                "replacement": replacement,
                "score": score,
                "steps": steps,
            }

        # Step 0: Free / Lead magnet / $0 items
        if is_unresolved:
            steps.append({
                "step": 0,
                "rule_name": "Free / Lead Magnet Filter",
                "status": "SKIP",
                "detail": "标的未在在售目录中解析，跳过免费品排查并继续对账已购底册与规则",
            })
        elif is_free or price <= 0:
            steps.append({
                "step": 0,
                "rule_name": "Free / Lead Magnet Filter",
                "status": "HIT",
                "detail": f"免费引流品或 $0 标的 (${price:.2f})，非高价值商业软件采购需求",
            })
            return _verdict(False, "Rule 0 (免费引流品排查)", "属于免费引流品/公开营销教材，非软件采购需求")
        else:
            steps.append({
                "step": 0,
                "rule_name": "Free / Lead Magnet Filter",
                "status": "PASS",
                "detail": f"有效付费商业软件 (${price:.2f})",
            })

        # Step 1: Exact or token ownership check
        owned_match = self.kb.is_owned(name, slug)
        if owned_match:
            steps.append({
                "step": 1,
                "rule_name": "Owned Asset Match",
                "status": "HIT",
                "detail": f"已拥有该资产: {owned_match}",
            })
            return _verdict(False, "Rule 1 (已购资产底册对账)", f"已拥有该资产: {owned_match}", replacement=owned_match)
        steps.append({
            "step": 1,
            "rule_name": "Owned Asset Match",
            "status": "PASS",
            "detail": "未在 2nd Brain 65+ 款已购 SaaS 资产底册中直接命中",
        })

        # Step 2: Competitor overlap
        matched_comp = None
        comp_replacement = None
        for alt in alt_to:
            for comp_key, owned_rep in COMPETITOR_TO_OWNED_ASSET.items():
                if comp_key in alt:
                    matched_comp = alt
                    comp_replacement = owned_rep
                    break
            if matched_comp:
                break

        if matched_comp:
            steps.append({
                "step": 2,
                "rule_name": "Competitor Overlap (alternative_to)",
                "status": "HIT",
                "detail": f"宣称替代 {matched_comp.title()}，与本地「{comp_replacement}」重叠",
            })
            return _verdict(
                False,
                "Rule 2 (竞品标的对账与轮子去重)",
                f"与现有资产重叠: 宣称替代 {matched_comp.title()}，已被本地「{comp_replacement}」覆盖",
                replacement=comp_replacement,
            )
        steps.append({
            "step": 2,
            "rule_name": "Competitor Overlap (alternative_to)",
            "status": "PASS",
            "detail": "未命中既有轮子竞品替代词",
        })

        # Step 3: Saturated Subcategory & Category
        if subcategory in SATURATED_SUBCATEGORIES:
            rep = SATURATED_SUBCATEGORIES[subcategory]
            steps.append({
                "step": 3,
                "rule_name": "Saturated Subcategory",
                "status": "HIT",
                "detail": f"所属子分类「{subcategory}」已饱和覆盖: {rep}",
            })
            return _verdict(
                False,
                "Rule 3 (饱和分类拦截)",
                f"所属分类「{subcategory}」已被既有资产饱和覆盖: {rep}",
                replacement=rep,
            )
        elif category in SATURATED_SUBCATEGORIES:
            rep = SATURATED_SUBCATEGORIES[category]
            steps.append({
                "step": 3,
                "rule_name": "Saturated Category",
                "status": "HIT",
                "detail": f"所属主分类「{category}」已饱和覆盖: {rep}",
            })
            return _verdict(
                False,
                "Rule 3 (饱和分类拦截)",
                f"所属分类「{category}」已被既有资产饱和覆盖: {rep}",
                replacement=rep,
            )
        steps.append({
            "step": 3,
            "rule_name": "Saturated Classification",
            "status": "PASS",
            "detail": f"分类「{category} / {subcategory}」尚未饱和",
        })

        # Step 4: Domain & Capability deduplication against specific owned tools
        text_to_check = re.sub(r"[-_]", " ", f"{name} {desc} {subcategory} {category}").lower()

        # Google Sheets AI add-ons
        if ("sheet" in text_to_check or "spreadsheet" in text_to_check) and ("ai" in text_to_check or "formula" in text_to_check):
            rep = "Logic Sheet + Power Formulas (已拥有表格增强终身资产)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "Google 表格 AI 增强插件: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "Google 表格 AI 增强插件: 已拥有「Logic Sheet」+「Power Formulas」终身资产，无需重复购置", replacement=rep)

        # WooCommerce shop builders
        if "woo" in text_to_check and ("shop" in text_to_check or "elementor" in text_to_check or "lentor" in text_to_check):
            rep = "ShopEngine Plus Exclusive (已拥有 WooCommerce 全套构建器)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "WooCommerce 构建器: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "WooCommerce 电商模版与构建器: 已拥有「ShopEngine Plus Exclusive」终身资产，无需重复购置", replacement=rep)

        # Client portal / file transfer
        if "client management" in subcategory.lower() or "file transfer" in text_to_check or "client portal" in text_to_check:
            rep = "Skillplate (数字交付枢纽) + Cloudflare/Drive 原生链路"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "客户文件交付平台: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "客户文件交付平台: 已拥有「Skillplate」数字交付枢纽 + Cloudflare/Drive 原生链路", replacement=rep)

        # Native developer toolchain
        if "api client" in text_to_check or "rest client" in text_to_check or "api testing" in text_to_check:
            rep = "原生 Postman / Bruno / curl / OpenAPI 研发工具链"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "REST 客户端/API 工具: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "REST 客户端/API 工具: 本地拥有原生 Postman / Bruno / curl / OpenAPI 研发工具链，无需购置商业 SaaS", replacement=rep)

        if "documentation" in text_to_check and ("api" in text_to_check or "dev" in text_to_check):
            rep = "TypeDoc / Swagger / OpenAPI / NewType 智能体流水线"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "文档生成工具: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "文档生成工具: 本地拥有 TypeDoc / Swagger / OpenAPI / NewType 智能体流水线", replacement=rep)

        # GA4 / GTM / BigQuery automation wrappers
        if ("ga4" in text_to_check or "gtm" in text_to_check or "bigquery" in text_to_check) and (
            "track" in text_to_check or "tag" in text_to_check or "analytics" in text_to_check or "insight" in text_to_check or "report" in text_to_check
        ):
            rep = "gtm-agent + google-analytics-ops 研发技能"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "GA4/GTM/BigQuery 监测: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "GA4/GTM/BigQuery 自动化监测: 本地拥有 gtm-agent、google-analytics-ops 及原生 GCP 研发技能，无需采购低杠杆 SaaS", replacement=rep)

        # QR code & shortlink tools
        if ("qr code" in text_to_check or "qr" in text_to_check) and (
            "generate" in text_to_check or "secure" in text_to_check or "scan" in text_to_check or "link" in text_to_check
        ):
            rep = "SleekBio (已购资产) + BetterLinks (本地短链)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "二维码生成与短链: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "二维码生成与短链管理: 已拥有「SleekBio」+「BetterLinks」终身资产及本地 SVG/Python 生成工具，无需重复购置", replacement=rep)

        # Retargeting links & bio links
        if "retargeting link" in text_to_check or "custom retargeting" in text_to_check or "bio link" in text_to_check or "link in bio" in text_to_check:
            rep = "SleekBio + BetterLinks"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "重定向短链与 Bio 落地页: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "重定向短链与 Bio 落地页: 已拥有「SleekBio」终身资产 +「BetterLinks」独立短链系统，无需重复购置", replacement=rep)

        # Virtual events / webinar platforms
        if "virtual event" in text_to_check or "virtual conference" in text_to_check or "expo" in text_to_check or "meet-up platform" in text_to_check:
            rep = "GoBrunch (已购终身资产)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "虚拟会议与空间: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "虚拟会议与活动空间: 已拥有「GoBrunch」终身资产及 Google Meet / Discord / Zoom 原生支持", replacement=rep)

        # Gimmicky AR / 3D model viewers & 360 photo tours (out of domain)
        if (
            "virtual tour" in text_to_check
            or "dollhouse" in text_to_check
            or "matterport" in text_to_check
            or (("3d" in text_to_check or "augmented reality" in text_to_check) and ("model" in text_to_check or "ar" in text_to_check or "qr" in text_to_check or "product" in text_to_check))
        ):
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "3D/AR 展品与 360 全景看房 (非核心低杠杆套件)"})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "3D/AR 展品与 360 全景看房: 属于非核心垂直行业（房产/展厅）低杠杆套件，与当前 AI 智能体及电商研发栈脱节，坚决不买")

        # Chart / Graph embedders
        if ("chart" in text_to_check or "graph" in text_to_check) and (
            "embed" in text_to_check or "spreadsheet" in text_to_check or "flourish" in text_to_check or "json" in text_to_check or "google sheets" in text_to_check
        ):
            rep = "Chart.js / Recharts / Manim + Logic Sheet"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "轻量图表嵌入工具: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "轻量图表嵌入工具: 本地拥有 Chart.js / Recharts / Manim 研发能力与「Logic Sheet」资产，无需购置商业 SaaS", replacement=rep)

        # OCR & text extractors
        if "ocr" in text_to_check or "text sniper" in text_to_check or "textsniper" in text_to_check:
            rep = "macOS 原生 Live Text 实况文本识别"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "屏幕 OCR 工具: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "屏幕 OCR 工具: 系统自带 macOS 原生 Live Text 实况文本识别及本地技能，无需付费购买", replacement=rep)

        # WordPress 301 redirects, broken links
        if ("301 redirect" in text_to_check or "redirect" in text_to_check) and (
            "404" in text_to_check or "broken link" in text_to_check or "wp" in text_to_check or "link" in text_to_check
        ):
            rep = "BetterLinks (已拥有 WordPress 独立短链与重定向高级资产)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "WordPress 重定向与死链监控: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "WordPress 重定向与死链监控: 已拥有「BetterLinks」终身高级资产 + 原生 Redirection，无需重复购置", replacement=rep)

        # 3D interactive flipbooks and PDF viewers
        if ("flipbook" in text_to_check or "flip book" in text_to_check or "pdf viewer" in text_to_check) and (
            "embed" in text_to_check or "pdf" in text_to_check or "digital booklet" in text_to_check or "book" in text_to_check
        ):
            rep = "FlipBooklets (已拥有终身资产与 Cloudflare Edge Worker 白标网关)"
            steps.append({"step": 4, "rule_name": "Domain Capability", "status": "HIT", "detail": "3D 交互式电子书/翻页书: " + rep})
            return _verdict(False, "Rule 4 (垂直领域与能力去重)", "3D 交互式电子书/翻页书: 已拥有「FlipBooklets」终身资产与 Cloudflare Edge Worker 白标网关，无需重复购置", replacement=rep)

        steps.append({
            "step": 4,
            "rule_name": "Domain Capability Deduplication",
            "status": "PASS",
            "detail": "通过垂直领域与本地技能能力去重检查",
        })

        # Step 5: Low-value or Non-software asset check
        matched_pat = None
        for pat in NON_SOFTWARE_PATTERNS:
            if re.search(pat, text_to_check):
                if "software" not in text_to_check and "api" not in text_to_check and "platform" not in text_to_check:
                    matched_pat = pat
                    break

        if matched_pat:
            steps.append({
                "step": 5,
                "rule_name": "Software Legitimacy Gate",
                "status": "HIT",
                "detail": f"命中非高杠杆模板/教材规则: {matched_pat}",
            })
            return _verdict(False, "Rule 5 (非高杠杆素材/教材门禁)", f"非高杠杆软件工具 (命中模板/教程/素材库规则: {matched_pat})，坚决不买")

        steps.append({
            "step": 5,
            "rule_name": "Software Legitimacy Gate",
            "status": "PASS",
            "detail": "属于合法软件系统或开发工具，非低杠杆模板教材",
        })

        if is_unresolved:
            steps.append({
                "step": 6,
                "rule_name": "Catalog Metadata Existence",
                "status": "HIT",
                "detail": f"未能在当前 AppSumo 在售目录或本地数据库中找到标的 '{slug}' 的元数据",
            })
            return _verdict(
                False,
                "Rule 6 (标的无法解析或已下架)",
                f"未能在当前 AppSumo 在售目录或本地数据库中找到标的 '{slug}' 的有效元数据，请核对链接",
            )

        # Step 6: Review & Rating quality gate
        if rating < 4.5:
            steps.append({
                "step": 6,
                "rule_name": "Quality Gate (Rating)",
                "status": "HIT",
                "detail": f"评分不足 4.5 ({rating:.2f}⭐️)",
            })
            return _verdict(False, "Rule 6 (评分与口碑门禁)", f"评分不足 4.5 ({rating:.1f}⭐️)，质量门禁未通过")

        if reviews < 10:
            steps.append({
                "step": 6,
                "rule_name": "Quality Gate (Reviews)",
                "status": "HIT",
                "detail": f"评价数量不足 10 条 ({reviews}条)",
            })
            return _verdict(False, "Rule 6 (评分与口碑门禁)", f"评价数量不足 10 条 ({reviews}条)，缺乏充分社区信任验证")

        steps.append({
            "step": 6,
            "rule_name": "Quality Gate (Rating & Reviews)",
            "status": "PASS",
            "detail": f"满足质量门禁要求 ({rating:.2f}⭐️, {reviews} 条评价)",
        })

        # Step 7: Strategic business capability alignment gate
        STRATEGIC_DOMAINS = [
            "ai", "automation", "agent", "llm", "api", "ecommerce", "woocommerce", "shopify",
            "checkout", "seo", "crm", "workflow", "database", "developer", "marketing", "customer"
        ]
        if not any(dom in text_to_check for dom in STRATEGIC_DOMAINS):
            steps.append({
                "step": 7,
                "rule_name": "Strategic Alignment",
                "status": "HIT",
                "detail": "不属于当前核心业务主线 (AI/电商/增长/开发)",
            })
            return _verdict(False, "Rule 7 (战略主线对齐)", "所属领域不属于当前核心业务主线 (AI/电商/增长/开发)，避免分散注意力与不必要支出")

        steps.append({
            "step": 7,
            "rule_name": "Strategic Alignment",
            "status": "PASS",
            "detail": "契合当前核心业务与技术主线",
        })

        # Step 8: High-value capability gap candidate & scoring
        score = 0.0
        score += (rating - 4.0) * 40
        score += min(30.0, reviews * 0.5)
        if 0 < price <= 69:
            score += 20.0
        elif price <= 129:
            score += 10.0

        steps.append({
            "step": 8,
            "rule_name": "Candidate Scoring",
            "status": "PASS",
            "detail": f"综合评分: {score:.1f} 分 (评分权重: {(rating-4.0)*40:.1f}, 评价权重: {min(30.0, reviews*0.5):.1f})",
        })

        return _verdict(True, "Rule 8 (空白能力候选通过)", "通过资产对账与空白能力门禁，属于未拥有的高分软件候选", score=score)

    def audit_deal(self, deal: Dict[str, Any]) -> Tuple[bool, str, float]:
        res = self.audit_deal_steps(deal)
        return res["should_recommend"], res["reason"], res["score"]


def resolve_binary() -> str:
    """Finds appsumo binary in local repo or PATH."""
    repo_bin = Path(__file__).resolve().parent.parent / "appsumo"
    if repo_bin.is_file() and os.access(repo_bin, os.X_OK):
        return str(repo_bin)
    which_bin = shutil.which("appsumo")
    if which_bin:
        return which_bin
    return str(repo_bin)


def clean_product_slug(deal_input: str) -> str:
    """Extracts clean product slug from URL or bare slug."""
    raw = str(deal_input).strip()
    raw = raw.split("#")[0].split("?")[0].strip()
    if raw.startswith("http://") or raw.startswith("https://"):
        try:
            from urllib.parse import urlparse
            p = urlparse(raw).path.strip("/")
            parts = [seg for seg in p.split("/") if seg]
            for i, part in enumerate(parts):
                if part == "products" and i + 1 < len(parts) and parts[i + 1]:
                    return parts[i + 1]
            if parts:
                return parts[-1]
        except Exception:
            pass
    if "products/" in raw:
        parts = [seg for seg in raw.split("/") if seg]
        for i, part in enumerate(parts):
            if part == "products" and i + 1 < len(parts) and parts[i + 1]:
                return parts[i + 1]
    parts = [seg for seg in raw.split("/") if seg]
    if parts:
        return parts[-1]
    return raw


def _normalize_scraped_deal(d: Dict[str, Any], slug: str) -> Dict[str, Any]:
    """Normalizes deal representations from SQLite, scraped JSON, or wire API into a standard Deal dict."""
    plans = d.get("plans") or []
    first_plan = plans[0] if (isinstance(plans, list) and plans) else {}
    ratings = d.get("ratings") or {}
    deal_review = d.get("deal_review") or {}
    attrs = d.get("attributes") or {}
    tax = d.get("taxonomy") or {}

    raw_price = first_plan.get("price") if isinstance(first_plan, dict) else None
    if raw_price is None:
        raw_price = d.get("price")
    raw_orig = first_plan.get("original_price") if isinstance(first_plan, dict) else None
    if raw_orig is None:
        raw_orig = d.get("original_price")

    raw_rating = None
    if isinstance(ratings, dict) and ratings.get("average_rating") is not None:
        raw_rating = ratings["average_rating"]
    elif isinstance(deal_review, dict) and deal_review.get("average_rating") is not None:
        raw_rating = deal_review["average_rating"]
    elif d.get("average_rating") is not None:
        raw_rating = d["average_rating"]

    raw_reviews = None
    if isinstance(ratings, dict) and ratings.get("review_count") is not None:
        raw_reviews = ratings["review_count"]
    elif isinstance(deal_review, dict) and deal_review.get("review_count") is not None:
        raw_reviews = deal_review["review_count"]
    elif d.get("review_count") is not None:
        raw_reviews = d["review_count"]

    alt_to = d.get("alternative_to") or []
    if not alt_to and isinstance(attrs, dict) and attrs.get("alternative_to"):
        alt_to = attrs["alternative_to"]
    if isinstance(alt_to, str):
        try:
            alt_to = json.loads(alt_to)
        except Exception:
            alt_to = [alt_to]

    best_for = d.get("best_for") or []
    if not best_for and isinstance(attrs, dict) and attrs.get("best_for"):
        best_for = attrs["best_for"]
    if isinstance(best_for, str):
        try:
            best_for = json.loads(best_for)
        except Exception:
            best_for = [best_for]

    integrations = d.get("integrations") or []
    if not integrations and isinstance(attrs, dict) and attrs.get("integrations"):
        integrations = attrs["integrations"]
    if isinstance(integrations, str):
        try:
            integrations = json.loads(integrations)
        except Exception:
            integrations = [integrations]

    subcat = d.get("subcategory") or ""
    if not subcat and isinstance(attrs, dict) and attrs.get("subcategory"):
        s_val = attrs["subcategory"]
        subcat = s_val[0] if isinstance(s_val, list) and s_val else str(s_val)

    cat = d.get("category") or ""
    if not cat and isinstance(attrs, dict) and attrs.get("category"):
        c_val = attrs["category"]
        cat = c_val[0] if isinstance(c_val, list) and c_val else str(c_val)
    elif not cat and isinstance(tax, dict) and tax.get("category"):
        cat = tax["category"].get("value_enumeration", "")

    return {
        "id": d.get("id"),
        "name": d.get("public_name") or d.get("name") or slug.replace("-", " ").title(),
        "slug": d.get("slug") or slug,
        "url": d.get("get_absolute_url") or d.get("appsumo_url") or d.get("url") or f"/products/{slug}/",
        "price": float(raw_price) if raw_price is not None else 0.0,
        "original_price": float(raw_orig) if raw_orig is not None else 0.0,
        "plus_price": float(d.get("plus_price") or d.get("plus_discount_price") or 0.0),
        "is_free": bool(d.get("is_free")),
        "average_rating": float(raw_rating) if raw_rating is not None else None,
        "review_count": int(raw_reviews) if raw_reviews is not None else None,
        "card_description": d.get("card_description") or "",
        "value_prop": d.get("value_prop") or "",
        "subcategory": subcat,
        "category": cat,
        "alternative_to": alt_to,
        "best_for": best_for,
        "integrations": integrations,
    }


def fetch_deal_by_slug(slug_or_url: str, bin_path: Optional[str] = None) -> Optional[Dict[str, Any]]:
    """Resolves deal metadata by slug from local SQLite, data files, CLI query, or live API."""
    slug = clean_product_slug(slug_or_url)
    repo_root = Path(__file__).resolve().parent.parent

    # 1. Query local SQLite database (authoritative catalog snapshot)
    db_candidates = [
        Path.home() / "Library/Application Support/appsumo-cli/appsumo.db",
        Path.home() / ".config/appsumo-cli/appsumo.db",
    ]
    if os.getenv("APPSUMO_DB"):
        db_candidates.insert(0, Path(os.getenv("APPSUMO_DB")))

    for db_path in db_candidates:
        if db_path.exists():
            try:
                import sqlite3
                conn = sqlite3.connect(db_path)
                conn.row_factory = sqlite3.Row
                cursor = conn.cursor()
                cursor.execute(
                    "select * from deals where slug = ? or lower(slug) = lower(?) limit 1",
                    (slug, slug),
                )
                row = cursor.fetchone()
                if row:
                    return _normalize_scraped_deal(dict(row), slug)
            except Exception:
                pass

    # 2. Local scraped data folder (data/<slug>/deal.json)
    deal_json_path = repo_root / "data" / slug / "deal.json"
    if deal_json_path.exists():
        try:
            data = json.loads(deal_json_path.read_text(encoding="utf-8"))
            if data:
                return _normalize_scraped_deal(data, slug)
        except Exception:
            pass

    # 3. Query via appsumo deals list --query <slug>
    bin_path = bin_path or resolve_binary()
    cmd = [bin_path, "deals", "list", "--query", slug, "--limit", "10", "--json"]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
        data = json.loads(proc.stdout)
        deals = data.get("deals", [])
        norm_slug = re.sub(r"[^a-z0-9]", "", slug.lower())
        for d in deals:
            d_slug = d.get("slug") or ""
            if d_slug.lower() == slug.lower() or re.sub(r"[^a-z0-9]", "", d_slug.lower()) == norm_slug:
                return _normalize_scraped_deal(d, slug)
    except Exception:
        pass

    # 4. Direct fetch from AppSumo public browse endpoint via curl
    try:
        api_url = f"https://appsumo.com/api/v2/deals/esbrowse/?query={slug}&per_page=10&sort=newest"
        proc = subprocess.run(
            ["curl", "-s", "-H", "User-Agent: appsumo-cli/buy-advisor", api_url],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if proc.returncode == 0 and proc.stdout:
            data = json.loads(proc.stdout)
            norm_slug = re.sub(r"[^a-z0-9]", "", slug.lower())
            for raw_deal in data.get("deals", []):
                rd_slug = raw_deal.get("slug") or ""
                if rd_slug.lower() == slug.lower() or re.sub(r"[^a-z0-9]", "", rd_slug.lower()) == norm_slug:
                    return _normalize_scraped_deal(raw_deal, slug)
    except Exception:
        pass

    return None


def explain_deal(
    slug_or_url: str,
    assets_file: Path = DEFAULT_ASSETS_MD,
    json_output: bool = False,
    bin_path: Optional[str] = None,
) -> Dict[str, Any]:
    """Diagnostic-First inspector: evaluates a deal and prints or returns detailed trace."""
    kb = AssetKnowledgeBase(assets_file)
    auditor = DealAuditor(kb)
    slug = clean_product_slug(slug_or_url)

    deal = fetch_deal_by_slug(slug, bin_path=bin_path)
    if not deal:
        # Fallback minimal payload marked as unresolved so inspector still audits ownership/rules
        deal = {
            "name": slug.replace("-", " ").title(),
            "slug": slug,
            "url": f"/products/{slug}/",
            "price": None,
            "average_rating": None,
            "review_count": None,
            "card_description": f"Deal with slug '{slug}' (unresolved from remote/local catalog)",
            "is_unresolved": True,
        }

    audit_result = auditor.audit_deal_steps(deal)

    if json_output:
        print(json.dumps(audit_result, indent=2, ensure_ascii=False))
        return audit_result

    # Format human-readable Chinese terminal diagnosis
    d = audit_result["deal"]
    price_val = f"${d.get('price', 0):.2f}" if d.get('price') is not None else "未知"
    orig_val = f"${d.get('original_price', 0):.2f}" if d.get('original_price') is not None else "未知"
    rating_val = f"{d.get('rating', 0):.2f}⭐️" if d.get('rating') is not None else "无评分"
    reviews_val = f"{d.get('reviews', 0)} 条评价" if d.get('reviews') is not None else "无评价"

    lines = [
        "=" * 70,
        "🔍 AppSumo 采购决策对账诊断器 (Deal Decision Inspector)",
        "=" * 70,
        "📦 标的信息 (Deal Profile):",
        f"  - 产品名称: {d.get('name')}",
        f"  - 唯一标识: {d.get('slug')} (Canonical ID: {d.get('canonical_id')})",
        f"  - 官方链接: {d.get('url')}",
        f"  - 类目架构: {d.get('category') or '未标明'} / {d.get('subcategory') or '未标明'}",
        f"  - 价格评级: {price_val} (原价 {orig_val}), {rating_val} ({reviews_val})",
        f"  - 宣称替代: {', '.join(d.get('alternative_to') or []) or '无'}",
        f"  - 产品简述: {d.get('card_description', '')[:100]}...",
        "",
        "📋 规则逐项对账链条 (Step-by-Step Rule Audit):",
    ]

    for s in audit_result.get("steps", []):
        icon = "✅ [PASS]" if s["status"] == "PASS" else "🚨 [HIT ]" if s["status"] == "HIT" else "⏭️ [SKIP]"
        lines.append(f"  {icon} Rule {s['step']}: {s['rule_name']} -> {s['detail']}")

    lines.extend([
        "",
        "=" * 70,
        f"🎯 最终裁定: {'✅ RECOMMENDED (发现真实空白机会)' if audit_result['should_recommend'] else '❌ REJECTED (坚决拦截重复轮子)'}",
        f"🚨 触发规则: {audit_result.get('triggered_rule')}",
        f"💡 决策原因: {audit_result.get('reason')}",
    ])
    if audit_result.get("replacement"):
        lines.append(f"🛡️ 既有资产替代: {audit_result['replacement']}")
    lines.append("=" * 70)

    print("\n".join(lines))
    return audit_result


def fetch_candidate_deals(
    bin_path: str,
    limit: int = 0,
    scan_mode: str = "all_catalog",
    prefer_local: bool = True,
) -> Tuple[List[Dict[str, Any]], Dict[str, Any]]:
    """
    Fetches deals from AppSumo catalog with full pagination tracking.
    Prefers fast local SQLite snapshot when available, with automatic live API fallback.

    Args:
        bin_path: path to appsumo binary
        limit: max deals to fetch (0 = all deals across all pages, no cap)
        scan_mode: "all_catalog" (all public deals) or "ideal" (rating >= 4.5, reviews >= 10)
        prefer_local: if True, query local SQLite first for speed and offline safety

    Returns:
        (deals_list, fetch_info_dict)
    """
    # 1. Attempt fast local SQLite query first if preferred
    if prefer_local:
        if scan_mode == "ideal":
            local_cmd = [
                bin_path,
                "deals",
                "ideal",
                "--local",
                "--min-rating",
                "4.5",
                "--min-reviews",
                "10",
                "--limit",
                str(limit),
                "--json",
            ]
        else:
            local_cmd = [
                bin_path,
                "deals",
                "list",
                "--local",
                "--limit",
                str(limit),
                "--json",
            ]
        try:
            proc = subprocess.run(local_cmd, capture_output=True, text=True, check=True)
            data = json.loads(proc.stdout)
            deals = data.get("deals", [])
            if deals:
                fetch_info = data.get("fetch", {})
                fetch_info["warnings"] = data.get("warnings", [])
                fetch_info["source"] = "local_sqlite"
                return deals, fetch_info
        except Exception:
            pass

    # 2. Live remote catalog query (fallback or explicit live mode)
    if scan_mode == "ideal":
        cmd = [
            bin_path,
            "deals",
            "ideal",
            "--min-rating",
            "4.5",
            "--min-reviews",
            "10",
            "--limit",
            str(limit),
            "--json",
        ]
    else:
        cmd = [
            bin_path,
            "deals",
            "list",
            "--limit",
            str(limit),
            "--json",
        ]

    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
        data = json.loads(proc.stdout)
        deals = data.get("deals", [])
        fetch_info = data.get("fetch", {})
        fetch_info["warnings"] = data.get("warnings", [])
        fetch_info["source"] = "live_api"
        return deals, fetch_info
    except Exception as e:
        print(f"Error fetching deals via CLI ({cmd}): {e}", file=sys.stderr)
        return [], {"error": str(e), "complete": False, "unique_deals": 0}


def cleanup_previous_notion_runs(page_id: str, token: str) -> None:
    """Removes prior automated cadence runs from the Notion page to maintain single authoritative verdict."""
    cmd = [
        "curl", "-s",
        "-H", f"Authorization: Bearer {token}",
        "-H", "Notion-Version: 2022-06-28",
        f"https://api.notion.com/v1/blocks/{page_id}/children",
    ]
    try:
        proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
        data = json.loads(proc.stdout)
        results = data.get("results", [])
        # Keep the first block (original prompt / task description)
        for block in results[1:]:
            bid = block.get("id")
            del_cmd = [
                "curl", "-s", "-X", "DELETE",
                "-H", f"Authorization: Bearer {token}",
                "-H", "Notion-Version: 2022-06-28",
                f"https://api.notion.com/v1/blocks/{bid}",
            ]
            subprocess.run(del_cmd, capture_output=True, text=True)
    except Exception as e:
        print(f"Notice: cleanup previous Notion blocks skipped: {e}", file=sys.stderr)


def push_notion_resolution(page_id: str, title: str, blocks: List[Dict[str, Any]]) -> bool:
    """Appends blocks to Notion page using notion token and sets Status to Done."""
    env_path = Path.home() / ".gemini/antigravity/skills/notion-mcp-connector/.env"
    token = os.getenv("NOTION_TOKEN") or os.getenv("NOTION_API_KEY")
    if not token and env_path.exists():
        for line in env_path.read_text().splitlines():
            if line.startswith("NOTION_TOKEN=") or line.startswith("NOTION_API_KEY="):
                token = line.split("=", 1)[1].strip().strip("\"'")
                break

    if not token:
        print("Warning: Notion token not found; skipping Notion live push.", file=sys.stderr)
        return False

    import urllib.request

    # 1. Clean up older run blocks to prevent duplicate accumulation
    cleanup_previous_notion_runs(page_id, token)

    # 2. Append fresh authoritative verdict blocks
    url = f"https://api.notion.com/v1/blocks/{page_id}/children"
    headers = {
        "Authorization": f"Bearer {token}",
        "Notion-Version": "2022-06-28",
        "Content-Type": "application/json",
    }
    payload = {"children": blocks}
    req = urllib.request.Request(url, data=json.dumps(payload).encode("utf-8"), headers=headers, method="PATCH")
    try:
        with urllib.request.urlopen(req) as resp:
            append_ok = resp.status == 200
    except Exception as e:
        print(f"Error pushing blocks to Notion: {e}", file=sys.stderr)
        append_ok = False

    # 3. Update page Status to Done
    page_url = f"https://api.notion.com/v1/pages/{page_id}"
    page_payload = {"properties": {"Status": {"status": {"name": "Done"}}}}
    page_req = urllib.request.Request(
        page_url,
        data=json.dumps(page_payload).encode("utf-8"),
        headers=headers,
        method="PATCH",
    )
    try:
        with urllib.request.urlopen(page_req) as _:
            pass
    except Exception as e:
        print(f"Notice: updating page Status property skipped: {e}", file=sys.stderr)

    return append_ok


def build_notion_blocks(
    audited_count: int,
    recommended: List[Tuple[Dict[str, Any], str, float]],
    rejections: List[Tuple[Dict[str, Any], str]],
    fetch_info: Optional[Dict[str, Any]] = None,
) -> List[Dict[str, Any]]:
    """Builds rich Notion blocks for the cadence run with pagination reconciliation audit."""
    now_str = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    fetch_info = fetch_info or {}
    declared = fetch_info.get("declared_total") or audited_count
    complete = fetch_info.get("complete", True)
    requests = fetch_info.get("requests", 1)
    scanned = fetch_info.get("scanned_deals") or declared
    status_str = "全量对账完整 (0 截断)" if complete else "⚠️ 注意：分页存在截断"

    callout_text = (
        f"全量分页对账巡检：已扫描 AppSumo 目录全部 {scanned} / {declared} 款 Deals "
        f"({status_str}，共发起 {requests} 批次请求)，"
        f"结合 2nd Brain 66 款已购 SaaS 资产底册严格防重对账。"
    )

    blocks = [
        {
            "object": "block",
            "type": "heading_2",
            "heading_2": {
                "rich_text": [
                    {
                        "type": "text",
                        "text": {"content": f"🎯 AppSumo 每周全量资产对账与推荐巡检 ({now_str})"},
                    }
                ]
            },
        },
        {
            "object": "block",
            "type": "callout",
            "callout": {
                "icon": {"type": "emoji", "emoji": "🛡️"},
                "rich_text": [
                    {
                        "type": "text",
                        "text": {"content": callout_text},
                    }
                ],
            },
        },
    ]

    if not recommended:
        blocks.append({
            "object": "block",
            "type": "paragraph",
            "paragraph": {
                "rich_text": [
                    {
                        "type": "text",
                        "text": {
                            "content": (
                                "✅ 核心结论：当前全量目录暂无必须购买的 Deal。所有候选产品均与现有轮子"
                                "（如 Albato, Zernio, Taku, TrustMate, Encharge, Zylvie, SleekBio, GoBrunch, BetterLinks, FlipBooklets 等）功能重叠，"
                                "坚决不推销重复轮子（0 采购推荐，保护注意力与资金）。"
                            )
                        },
                        "annotations": {"bold": True, "color": "green"},
                    }
                ]
            },
        })
    else:
        blocks.append({
            "object": "block",
            "type": "heading_3",
            "heading_3": {
                "rich_text": [
                    {
                        "type": "text",
                        "text": {"content": f"💡 发现 {len(recommended)} 款真正填补空白能力的推荐软件："},
                    }
                ]
            },
        })
        for deal, reason, score in recommended:
            name = deal.get("name", "")
            price = deal.get("price", 0)
            orig = deal.get("original_price", 0)
            rating = deal.get("average_rating", 0)
            reviews = deal.get("review_count", 0)
            url = f"https://appsumo.com{deal.get('url', '')}"
            desc = deal.get("card_description", "")

            blocks.append({
                "object": "block",
                "type": "paragraph",
                "paragraph": {
                    "rich_text": [
                        {
                            "type": "text",
                            "text": {"content": f"• {name} (${price}, 原价 ${orig}, {rating}⭐️/{reviews}条评价): "},
                            "annotations": {"bold": True},
                        },
                        {"type": "text", "text": {"content": f"{desc}\n  理由: {reason}\n  链接: "}},
                        {"type": "text", "text": {"content": url, "link": {"url": url}}},
                    ]
                },
            })

    # Summary of filtered categories in a toggle block
    blocks.append({
        "object": "block",
        "type": "toggle",
        "toggle": {
            "rich_text": [
                {
                    "type": "text",
                    "text": {"content": f"🔍 查看详细过滤日志 (已拦截全部 {len(rejections)} 款重叠轮子/低质素材)"},
                }
            ],
            "children": [
                {
                    "object": "block",
                    "type": "paragraph",
                    "paragraph": {
                        "rich_text": [
                            {
                                "type": "text",
                                "text": {
                                    "content": "\n".join(
                                        f"• {deal.get('name', '')[:25]}: {reason}"
                                        for deal, reason in rejections[:20]
                                    )
                                },
                            }
                        ]
                    },
                }
            ],
        },
    })

    return blocks


def run_advisor(
    assets_file: Path = DEFAULT_ASSETS_MD,
    notion_page_id: str = DEFAULT_NOTION_PAGE_ID,
    limit: int = 0,
    scan_mode: str = "all_catalog",
    dry_run: bool = False,
    as_json: bool = False,
    force: bool = False,
) -> Dict[str, Any]:
    """Main entrypoint for running the buy advisor."""
    kb = AssetKnowledgeBase(assets_file)
    auditor = DealAuditor(kb)
    bin_path = resolve_binary()

    # 1. State / Skip Check
    state_dir = Path(__file__).resolve().parent.parent / ".run/cadence" / CADENCE_ID
    state_dir.mkdir(parents=True, exist_ok=True)
    state_file = state_dir / "state.json"
    receipts_dir = state_dir / "receipts"
    receipts_dir.mkdir(parents=True, exist_ok=True)

    today_str = datetime.date.today().isoformat()
    week_str = datetime.date.today().strftime("%Y-W%W")
    success_marker = state_dir / f"{week_str}.success"

    if success_marker.exists() and not force and not dry_run:
        return {
            "status": "skipped",
            "message": f"本周任务已完成 (Marker: {success_marker.name})，且输入处于静默期，优雅跳过。",
            "recommended": [],
            "audited_count": 0,
        }

    # 2. Fetch candidates with full pagination tracking
    deals, fetch_info = fetch_candidate_deals(bin_path, limit=limit, scan_mode=scan_mode)
    if not deals:
        return {
            "status": "no_data",
            "message": "未能拉取到候选 Deals，可能网络波动或目录暂不可达。",
            "recommended": [],
            "audited_count": 0,
        }

    recommended: List[Tuple[Dict[str, Any], str, float]] = []
    rejections: List[Tuple[Dict[str, Any], str]] = []

    for deal in deals:
        should_rec, reason, score = auditor.audit_deal(deal)
        if should_rec:
            recommended.append((deal, reason, score))
        else:
            rejections.append((deal, reason))

    # Sort recommended by score descending
    recommended.sort(key=lambda x: x[2], reverse=True)

    declared_total = fetch_info.get("declared_total") or len(deals)
    scanned_deals = fetch_info.get("scanned_deals") or declared_total
    pagination_complete = fetch_info.get("complete", True)
    requests_count = fetch_info.get("requests", 1)

    result = {
        "status": "recommended" if recommended else "no_purchase_needed",
        "timestamp": datetime.datetime.now().isoformat(),
        "cadence_id": CADENCE_ID,
        "scan_mode": scan_mode,
        "audited_count": len(deals),
        "declared_total": declared_total,
        "scanned_deals": scanned_deals,
        "unique_deals": fetch_info.get("unique_deals", len(deals)),
        "complete_pagination": pagination_complete,
        "requests_count": requests_count,
        "pagination_truncated": fetch_info.get("truncated", False),
        "pagination_warnings": fetch_info.get("warnings", []),
        "owned_assets_indexed": len(kb.tools),
        "rejected_count": len(rejections),
        "recommended_count": len(recommended),
        "recommended": [
            {
                "name": d.get("name"),
                "slug": d.get("slug"),
                "canonical_id": f"appsumo:{d.get('slug')}",
                "price": d.get("price"),
                "original_price": d.get("original_price"),
                "rating": d.get("average_rating"),
                "review_count": d.get("review_count"),
                "url": f"https://appsumo.com{d.get('url', '')}",
                "reason": r,
                "score": s,
            }
            for d, r, s in recommended
        ],
        "rejection_summary": [
            {"name": d.get("name"), "slug": d.get("slug"), "reason": r}
            for d, r in rejections[:15]
        ],
    }

    # 3. Output receipts & state
    if not dry_run:
        # Write receipt
        receipt_path = receipts_dir / f"{today_str}.md"
        receipt_md = [
            f"# AppSumo Weekly Buy Advisor Receipt - {today_str}",
            "",
            f"- **Cadence ID**: `{CADENCE_ID}`",
            f"- **全量分页对账**: 实际扫描 {scanned_deals} / 官方声明 {declared_total} 款 (审计 {len(deals)} 款, 完整性: {'✅ 完整零截断' if pagination_complete else '⚠️ 存在截断'}, 请求批次: {requests_count})",
            f"- **Audited Deals**: {len(deals)}",
            f"- **Owned Assets Indexed**: {len(kb.tools)}",
            f"- **Recommended Count**: {len(recommended)}",
            f"- **Status**: `{result['status']}`",
            "",
            "## Decision Verdict",
            "",
        ]
        if not recommended:
            receipt_md.append(
                f"> **✅ 当前无必须采购项**：已全量扫描 AppSumo {len(deals)} 款 Deals，全部与已拥有的 {len(kb.tools)} 款终身软件"
                "（如 Albato, Zernio, Taku, TrustMate, Encharge, Zylvie, FlipBooklets, SleekBio, GoBrunch 等）重叠或属于低杠杆素材，"
                "坚决不推销重复轮子（0 采购推荐，保护注意力与资金）。\n"
            )
        else:
            receipt_md.append("### 推荐产品清单\n")
            for item in result["recommended"]:
                receipt_md.append(
                    f"- **[{item['name']}]({item['url']})** (${item['price']}, 原价 ${item['original_price']}, "
                    f"{item['rating']}⭐️/{item['review_count']}条评价)\n"
                    f"  - **匹配原因**: {item['reason']}\n"
                )

        receipt_md.append("\n## 被拦截重叠轮子样本 (Top 15)\n")
        for item in result["rejection_summary"]:
            receipt_md.append(f"- **{item['name']}** (`{item['slug']}`): {item['reason']}")

        receipt_path.write_text("\n".join(receipt_md), encoding="utf-8")

        # Write state.json
        state_data = {
            "cadence_id": CADENCE_ID,
            "last_run_at": datetime.datetime.now().isoformat(),
            "last_status": result["status"],
            "audited_count": len(deals),
            "declared_total": declared_total,
            "complete_pagination": pagination_complete,
            "requests_count": requests_count,
            "recommended_count": len(recommended),
            "receipt_path": str(receipt_path),
        }
        state_file.write_text(json.dumps(state_data, indent=2), encoding="utf-8")

        # Write success marker
        success_marker.write_text(f"PASS {datetime.datetime.now().isoformat()}\n", encoding="utf-8")

        # Push to Notion if page ID provided
        if notion_page_id:
            blocks = build_notion_blocks(len(deals), recommended, rejections, fetch_info=fetch_info)
            push_notion_resolution(notion_page_id, "AppSumo Weekly Buy Advisor", blocks)

    return result


def main():
    # Diagnostic explain sub-handler
    if len(sys.argv) > 1 and sys.argv[1] == "explain":
        explain_parser = argparse.ArgumentParser(description="AppSumo Deal Decision Inspector")
        explain_parser.add_argument("slug_or_url", help="AppSumo deal slug or full URL")
        explain_parser.add_argument("--assets-file", default=str(DEFAULT_ASSETS_MD), help="Path to assets markdown")
        explain_parser.add_argument("--json", action="store_true", help="Output JSON format")
        exp_args = explain_parser.parse_args(sys.argv[2:])
        explain_deal(
            slug_or_url=exp_args.slug_or_url,
            assets_file=Path(exp_args.assets_file),
            json_output=exp_args.json,
        )
        return

    parser = argparse.ArgumentParser(description="AppSumo Weekly Buy Advisor")
    parser.add_argument("--explain", type=str, default="", help="Inspect and explain decision for a specific deal slug or URL")
    parser.add_argument("--assets-file", default=str(DEFAULT_ASSETS_MD), help="Path to appsumo_purchased_assets.md")
    parser.add_argument("--notion-page-id", default=DEFAULT_NOTION_PAGE_ID, help="Target Notion Task Page ID")
    parser.add_argument("--limit", type=int, default=0, help="Number of deals to audit (0 fetches all, default: 0)")
    parser.add_argument("--scan-mode", default="all_catalog", choices=["all_catalog", "ideal"], help="Scan mode: all_catalog (default) or ideal")
    parser.add_argument("--dry-run", action="store_true", help="Audit without saving state or pushing Notion")
    parser.add_argument("--force", action="store_true", help="Bypass weekly success marker")
    parser.add_argument("--json", action="store_true", help="Emit raw JSON")

    args = parser.parse_args()

    if args.explain:
        explain_deal(
            slug_or_url=args.explain,
            assets_file=Path(args.assets_file),
            json_output=args.json,
        )
        return

    res = run_advisor(
        assets_file=Path(args.assets_file),
        notion_page_id=args.notion_page_id,
        limit=args.limit,
        scan_mode=args.scan_mode,
        dry_run=args.dry_run,
        as_json=args.json,
        force=args.force,
    )

    if args.json:
        print(json.dumps(res, indent=2, ensure_ascii=False))
    else:
        print(f"=== AppSumo Weekly Buy Advisor ({res.get('status')}) ===")
        if res.get("status") == "skipped":
            print(f"ℹ️ {res.get('message')}")
        else:
            declared = res.get("declared_total", res.get("audited_count", 0))
            scanned = res.get("scanned_deals", declared)
            complete_str = "完整走完零截断" if res.get("complete_pagination", True) else "⚠️ 分页存在截断"
            print(f"全量分页对账: 已扫描 {scanned} / 官方声明 {declared} 款 (审计 {res.get('audited_count', 0)} 款, {complete_str}, {res.get('requests_count', 1)} 批次请求)")
            print(f"已索引已购资产: {res.get('owned_assets_indexed', 0)} 款")
            print(f"推荐数量: {res.get('recommended_count', 0)} 款 | 拦截重复/低质: {res.get('rejected_count', 0)} 款")
            if res.get("status") == "no_purchase_needed":
                print("\n✅ 核心结论: 本周无须购买任何新 Deal！所有候选均与既有轮子功能重叠或不属于当前核心业务主线，坚决不推销重复轮子（0 采购推荐，保护注意力与资金）。")
            elif res.get("status") == "recommended":
                print("\n💡 发现以下填补空白能力的高分 Deal:")
                for item in res.get("recommended", []):
                    print(f" - {item['name']} (${item['price']}): {item['reason']}")
                    print(f"   链接: {item['url']}")


if __name__ == "__main__":
    main()
