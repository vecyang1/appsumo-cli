#!/usr/bin/env python3
"""
Unit tests for AppSumo Buy Advisor & Asset-Aware Recommender
"""

import unittest
from unittest import mock
from pathlib import Path
from tempfile import NamedTemporaryFile
import datetime
import json
import os
import shutil
import sys
import tempfile
import urllib.error

# Ensure scripts dir is on sys.path
sys.path.insert(0, str(Path(__file__).resolve().parent))

from appsumo_buy_advisor import (
    AssetKnowledgeBase,
    DealAuditor,
    DeliveryError,
    COMPETITOR_TO_OWNED_ASSET,
    SATURATED_SUBCATEGORIES,
    build_notion_blocks,
    clean_product_slug,
    explain_deal,
    fetch_candidate_deals,
    notify_slack_summary,
    redeliver_notifications,
    run_advisor,
    write_state_atomic,
)

SAMPLE_ASSETS_MD = """---
type: memory
status: active
---

# AppSumo Purchased Assets Portfolio

## Automation & Integrations
### Albato
**Description:** Zapier competitor for workflow automation.
**Global Status:** 🟢 Activated

### Boost.space
**Description:** Centralized database and automated cloud data synchronization.
**Global Status:** 🟢 Activated

## E-commerce & Checkout
### Zylvie - High-Converting Checkout Software
**Description:** High-converting online checkout software for digital creators.
**Global Status:** 🟢 Activated

### TrustMate.io
**Description:** Customer review and feedback collection, star rating showcase widgets.
**Global Status:** 🟢 Redeemed & Active

### Taku (taku.cool)
**Description:** High-converting website popup, modal notification center.
**Global Status:** 🟢 Redeemed & Active

## Marketing & Social Media
### Late (Zernio)
**Description:** Social media scheduling and multi-channel API platform.
**Global Status:** 🟢 Activated

### SendFox - Plus exclusive
**Description:** Email newsletter marketing platform.
**Global Status:** 🟢 Activated

## Publishing & Documents
### FlipBooklets
**Description:** Interactive 3D HTML5 flipbook generator and white-label document publisher with edge gateway.
**Global Status:** 🟢 Activated
"""

class TestAppSumoBuyAdvisor(unittest.TestCase):
    def setUp(self):
        self.temp_file = NamedTemporaryFile(mode="w+", delete=False, suffix=".md", encoding="utf-8")
        self.temp_file.write(SAMPLE_ASSETS_MD)
        self.temp_file.flush()
        self.temp_file.close()

        self.kb = AssetKnowledgeBase(Path(self.temp_file.name))
        self.auditor = DealAuditor(self.kb)

    def tearDown(self):
        Path(self.temp_file.name).unlink(missing_ok=True)

    def test_kb_loaded_active_tools(self):
        self.assertGreaterEqual(len(self.kb.tools), 6)
        self.assertIn("albato", self.kb.tool_names)
        self.assertIn("zylvie", self.kb.tool_names)
        self.assertIn("trustmate.io", self.kb.tool_names)
        self.assertIn("taku", self.kb.tool_names)

    def test_exact_name_match_rejected(self):
        deal = {
            "name": "Albato Pro Lifetime",
            "slug": "albato-pro",
            "average_rating": 4.9,
            "review_count": 100,
            "price": 69,
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("已拥有该资产", reason)

    def test_competitor_overlap_rejected(self):
        # A deal claiming to be an alternative to Zapier or Make
        deal = {
            "name": "FlowCraft Connect",
            "slug": "flowcraft",
            "average_rating": 4.8,
            "review_count": 25,
            "price": 49,
            "alternative_to": ["Zapier", "Make"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("与现有资产重叠", reason)
        self.assertIn("Albato", reason)

    def test_reviews_widget_competitor_rejected(self):
        deal = {
            "name": "StarProof Reviews",
            "slug": "starproof",
            "average_rating": 5.0,
            "review_count": 40,
            "price": 39,
            "alternative_to": ["Trustpilot", "Loox"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("TrustMate.io", reason)

    def test_saturated_subcategory_rejected(self):
        deal = {
            "name": "FeedbackCollector Widget",
            "slug": "feedback-collector",
            "average_rating": 4.9,
            "review_count": 15,
            "price": 29,
            "subcategory": "Feedback & Reviews",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("Feedback & Reviews", reason)
        self.assertIn("TrustMate.io", reason)

    def test_non_software_templates_rejected(self):
        deal = {
            "name": "500+ Canva Social Media Template Bundle",
            "slug": "canva-templates-bundle",
            "average_rating": 5.0,
            "review_count": 50,
            "price": 39,
            "subcategory": "Templates & Fonts",
            "card_description": "Canva templates to create high quality posts",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("非高杠杆软件" in reason or "模板" in reason)

    def test_low_rating_rejected(self):
        deal = {
            "name": "Some Obscure Tool",
            "slug": "obscure-tool",
            "average_rating": 3.8,
            "review_count": 20,
            "price": 29,
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("评分不足", reason)

    def test_free_item_rejected(self):
        deal = {
            "name": "Free Marketing Plan",
            "slug": "free-marketing-plan",
            "is_free": True,
            "price": 0,
            "average_rating": 5.0,
            "review_count": 20,
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("免费引流品", reason)

    def test_google_sheets_ai_rejected(self):
        deal = {
            "name": "SheetXAI Formula Generator",
            "slug": "sheetxai",
            "is_free": False,
            "price": 49,
            "average_rating": 4.9,
            "review_count": 20,
            "card_description": "AI formulas for Google Sheets",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("Logic Sheet", reason)

    def test_woocommerce_shopbuilder_rejected(self):
        deal = {
            "name": "ShopLentor WooCommerce Builder",
            "slug": "shoplentor",
            "is_free": False,
            "price": 69,
            "average_rating": 4.8,
            "review_count": 15,
            "card_description": "WooCommerce shop elementor widgets",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("ShopEngine", reason)

    def test_genuine_gap_recommended(self):

        deal = {
            "name": "QuantumVector DB Engine",
            "slug": "quantum-vector",
            "average_rating": 4.9,
            "review_count": 45,
            "price": 59,
            "original_price": 499,
            "category": "developer-tools",
            "subcategory": "Database & Storage",
            "card_description": "High performance vector database for local AI search and RAG",
            "alternative_to": ["Pinecone"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertTrue(rec)
        self.assertIn("通过资产对账与空白能力门禁", reason)
        self.assertGreater(score, 50.0)

    def test_link_shortener_competitor_rejected(self):
        deal = {
            "name": "Switchy",
            "slug": "switchy",
            "average_rating": 4.87,
            "review_count": 275,
            "price": 39,
            "alternative_to": ["Bitly", "Linktree", "Rebrandly"],
            "subcategory": "Paid Ads & Retargeting",
            "card_description": "Boost engagement and conversions with custom retargeting links",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("SleekBio" in reason or "BetterLinks" in reason or "Paid Ads" in reason)

    def test_qr_code_generator_rejected(self):
        deal = {
            "name": "ElkQR",
            "slug": "elkqr",
            "average_rating": 4.88,
            "review_count": 240,
            "price": 39,
            "alternative_to": ["Bitly", "Rebrandly"],
            "subcategory": "Website Analytics",
            "card_description": "The ultimate tool for generating and managing secure QR codes",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("二维码生成与短链管理" in reason or "Website Analytics" in reason or "SleekBio" in reason)

    def test_virtual_events_competitor_rejected(self):
        deal = {
            "name": "BeHuman.Online",
            "slug": "behumanonline",
            "average_rating": 4.87,
            "review_count": 30,
            "price": 49,
            "alternative_to": ["Hopin", "ON24", "Zoom"],
            "subcategory": "Virtual Events",
            "card_description": "Virtual conference and meetup platform",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("GoBrunch" in reason or "Virtual Events" in reason)

    def test_chart_embedder_rejected(self):
        deal = {
            "name": "InstaCharts",
            "slug": "instacharts",
            "average_rating": 4.92,
            "review_count": 13,
            "price": 29,
            "alternative_to": ["Flourish"],
            "subcategory": "Data & Reporting",
            "card_description": "Instantly create, share & embed charts from spreadsheet files",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("图表" in reason or "Data & Reporting" in reason or "Logic Sheet" in reason)

    def test_ga4_gtm_analytics_wrapper_rejected(self):
        deal = {
            "name": "Measuremate",
            "slug": "measuremate",
            "average_rating": 4.92,
            "review_count": 12,
            "price": 69,
            "subcategory": "Website Analytics",
            "card_description": "Automate GA4, GTM & BigQuery - track, tag, validate, report & insights w/o coding",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("GA4" in reason or "gtm-agent" in reason or "Website Analytics" in reason)

    def test_out_of_domain_rejected(self):
        deal = {
            "name": "TeliportMe Virtual Tours",
            "slug": "teliportme-virtual-tours",
            "average_rating": 4.92,
            "review_count": 100,
            "price": 89,
            "alternative_to": ["Matterport"],
            "subcategory": "Photo Editing",
            "card_description": "Create interactive 360 virtual tours with hotspots, floorplans, and 3D dollhouse",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("3D" in reason or "全景看房" in reason or "非核心业务" in reason or "Photo Editing" in reason)

    def test_wordpress_internal_linking_plugin_rejected(self):
        deal = {
            "name": "Internal Linking of Related Contents",
            "slug": "marketplace-internal-linking-related-contents",
            "average_rating": 4.84,
            "review_count": 25,
            "price": 59,
            "category": "website-builders-wordpress",
            "subcategory": "wordpress",
            "card_description": "Automatically link related posts to boost SEO and engage users with relevant content in WordPress Gutenberg and WooCommerce.",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("BuildFast" in reason or "架构不匹配" in reason or "Notion" in reason)

    def test_build_notion_blocks_no_recommendations(self):
        blocks = build_notion_blocks(audited_count=35, recommended=[], rejections=[])
        self.assertGreater(len(blocks), 1)
        # Verify clean conclusion
        text_content = json.dumps(blocks, ensure_ascii=False)
        self.assertIn("暂无必须购买的 Deal", text_content)
        self.assertIn("坚决不推销重复轮子", text_content)

    def test_build_notion_blocks_with_recommendations(self):
        deal = {
            "name": "QuantumVector DB",
            "price": 59,
            "original_price": 499,
            "average_rating": 4.9,
            "review_count": 45,
            "url": "/products/quantum-vector/",
            "card_description": "Fast vector search",
        }
        blocks = build_notion_blocks(
            audited_count=35,
            recommended=[(deal, "填补向量数据库能力空白", 85.0)],
            rejections=[]
        )
        text_content = json.dumps(blocks, ensure_ascii=False)
        self.assertIn("QuantumVector DB", text_content)
        self.assertIn("填补向量数据库能力空白", text_content)

    def test_clean_product_slug(self):
        self.assertEqual(clean_product_slug("flipbooklets"), "flipbooklets")
        self.assertEqual(clean_product_slug("https://appsumo.com/products/flipbooklets/"), "flipbooklets")
        self.assertEqual(clean_product_slug("https://appsumo.com/products/poppy-ai"), "poppy-ai")
        self.assertEqual(clean_product_slug("products/stackby/"), "stackby")

    def test_audit_deal_steps_matches_audit_deal(self):
        # SSOT verification: audit_deal must be an exact projection of audit_deal_steps
        deal = {
            "name": "Stackby Database",
            "slug": "stackby",
            "average_rating": 4.64,
            "review_count": 146,
            "price": 109,
            "alternative_to": ["Airtable", "Monday.com"],
        }
        res = self.auditor.audit_deal_steps(deal)
        rec, reason, score = self.auditor.audit_deal(deal)

        self.assertEqual(rec, res["should_recommend"])
        self.assertEqual(reason, res["reason"])
        self.assertEqual(score, res["score"])
        self.assertFalse(rec)
        self.assertEqual(res["verdict"], "REJECTED")
        self.assertIn("Airtable", reason)
        self.assertIn("Boost.space", res["replacement"])
        self.assertGreater(len(res["steps"]), 0)

    def test_flipbooklets_exact_ownership_rejected(self):
        deal = {
            "name": "FlipBooklets",
            "slug": "flipbooklets",
            "average_rating": 4.80,
            "review_count": 149,
            "price": 59,
            "alternative_to": ["Issuu"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("已拥有该资产", reason)
        self.assertIn("FlipBooklets", reason)

    def test_fliplink_issuu_alternative_rejected(self):
        deal = {
            "name": "FlipLink.me",
            "slug": "fliplinkme",
            "average_rating": 4.85,
            "review_count": 291,
            "price": 129,
            "alternative_to": ["Issuu"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("Issuu", reason)
        self.assertIn("FlipBooklets", reason)

    def test_rtila_rpa_alternative_rejected(self):
        deal = {
            "name": "RTILA X",
            "slug": "marketplace-rtila-growth-hacking-marketing-automation-software",
            "average_rating": 4.69,
            "review_count": 116,
            "price": 109,
            "alternative_to": ["Browse AI", "UiPath"],
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertIn("Browse Ai", reason)

    def test_no_code_mba_course_rejected(self):
        deal = {
            "name": "No Code MBA",
            "slug": "no-code-mba-deal",
            "average_rating": 4.85,
            "review_count": 54,
            "price": 159,
            "subcategory": "No-Code Automation",
            "card_description": "Learn how to build AI apps, marketplaces, and automations.",
        }
        rec, reason, score = self.auditor.audit_deal(deal)
        self.assertFalse(rec)
        self.assertTrue("No-Code Automation" in reason or "非高杠杆" in reason or "教程" in reason)

    def test_null_field_resilience(self):
        deal_with_nulls = {
            "name": "Sparse Metadata Deal",
            "slug": "sparse-deal",
            "average_rating": None,
            "review_count": None,
            "price": None,
            "is_free": False,
        }
        # Must audit cleanly without throwing TypeError: '<' not supported between NoneType and float
        rec, reason, score = self.auditor.audit_deal(deal_with_nulls)
        self.assertFalse(rec)

    def test_clean_product_slug_extended_vectors(self):
        self.assertEqual(clean_product_slug("appsumo.com/products/flipbooklets/?query=test#pricing"), "flipbooklets")
        self.assertEqual(clean_product_slug("products/flipbooklets?utm_source=email"), "flipbooklets")
        self.assertEqual(clean_product_slug("flipbooklets/?foo=bar#section"), "flipbooklets")
        self.assertEqual(clean_product_slug("flipbooklets#overview"), "flipbooklets")
        self.assertEqual(clean_product_slug("https://appsumo.com/products/poppy-ai/reviews/"), "poppy-ai")
        self.assertEqual(clean_product_slug("/poppy-ai/"), "poppy-ai")

    def test_uxer_asset_name_preservation_and_token_safety(self):
        uxer_md = """---
type: memory
status: active
---
# Portfolio
### U-xer
**Description:** User testing and UX research recording platform.
**Global Status:** 🟢 Activated
"""
        with NamedTemporaryFile(mode="w+", delete=False, suffix=".md", encoding="utf-8") as tf:
            tf.write(uxer_md)
            tf_path = Path(tf.name)

        try:
            uxer_kb = AssetKnowledgeBase(tf_path)
            self.assertIn("u-xer", uxer_kb.tool_names)
            self.assertNotIn("u", uxer_kb.tool_names)

            # Exact match on U-xer
            self.assertIsNotNone(uxer_kb.is_owned("U-xer", "u-xer"))
            self.assertIsNotNone(uxer_kb.is_owned("uxer", "uxer"))

            # Short tokens should NOT falsely trigger ownership
            self.assertIsNone(uxer_kb.is_owned("U", "u"))
            self.assertIsNone(uxer_kb.is_owned("Up", "up"))
            self.assertIsNone(uxer_kb.is_owned("Xerox Copy", "xerox-copy"))
        finally:
            tf_path.unlink(missing_ok=True)

    def test_unresolved_deal_rejection_and_rule6(self):
        deal = {
            "name": "non-existent-tool",
            "slug": "non-existent-tool",
            "is_unresolved": True,
            "price": 0.0,
        }
        res = self.auditor.audit_deal_steps(deal)
        self.assertFalse(res["should_recommend"])
        self.assertIn("Rule 6", res["triggered_rule"])
        # Step 0 should be skipped, not falsely hitting Rule 0
        step0 = next(s for s in res["steps"] if s["step"] == 0)
        self.assertEqual(step0["status"], "SKIP")
        # Step 6 should hit Catalog Metadata Existence
        step6 = next(s for s in res["steps"] if s["step"] == 6)
        self.assertEqual(step6["status"], "HIT")

    def test_unresolved_deal_hits_ownership_before_rule6(self):
        deal = {
            "name": "Albato",
            "slug": "albato",
            "is_unresolved": True,
            "price": 0.0,
        }
        res = self.auditor.audit_deal_steps(deal)
        self.assertFalse(res["should_recommend"])
        self.assertIn("Rule 1", res["triggered_rule"])
        self.assertIn("已拥有该资产", res["reason"])

    def test_pagination_tracking_in_notion_blocks(self):
        fetch_info = {
            "unique_deals": 338,
            "declared_total": 338,
            "complete": True,
            "requests": 4,
            "truncated": False,
        }
        blocks = build_notion_blocks(audited_count=338, recommended=[], rejections=[], fetch_info=fetch_info)
        text_content = json.dumps(blocks, ensure_ascii=False)
        self.assertIn("338 / 338", text_content)
        self.assertIn("全量对账完整", text_content)
        self.assertIn("4 批次请求", text_content)

    def test_fetch_candidate_deals_prefers_local(self):
        from unittest.mock import MagicMock, patch
        import subprocess

        fake_stdout = json.dumps({
            "deals": [{"slug": "local-deal-1", "name": "Local Deal"}],
            "fetch": {"unique_deals": 1, "declared_total": 1, "complete": True, "requests": 1},
            "warnings": [],
        })
        mock_proc = MagicMock(stdout=fake_stdout, returncode=0)

        with patch("subprocess.run", return_value=mock_proc) as mock_run:
            deals, fetch_info = fetch_candidate_deals("appsumo", limit=10, scan_mode="all_catalog", prefer_local=True)
            self.assertEqual(len(deals), 1)
            self.assertEqual(deals[0]["slug"], "local-deal-1")
            self.assertEqual(fetch_info.get("source"), "local_sqlite")
            # Verify --local flag was included in command
            called_cmd = mock_run.call_args[0][0]
            self.assertIn("--local", called_cmd)

    def test_fetch_candidate_deals_fallback_to_live_on_empty_local(self):
        from unittest.mock import MagicMock, patch

        empty_local_stdout = json.dumps({"deals": [], "fetch": {}, "warnings": []})
        live_stdout = json.dumps({
            "deals": [{"slug": "live-deal-1", "name": "Live Deal"}],
            "fetch": {"unique_deals": 1, "declared_total": 1, "complete": True, "requests": 1},
            "warnings": [],
        })

        mock_local = MagicMock(stdout=empty_local_stdout, returncode=0)
        mock_live = MagicMock(stdout=live_stdout, returncode=0)

        with patch("subprocess.run", side_effect=[mock_local, mock_live]) as mock_run:
            deals, fetch_info = fetch_candidate_deals("appsumo", limit=10, scan_mode="all_catalog", prefer_local=True)
            self.assertEqual(len(deals), 1)
            self.assertEqual(deals[0]["slug"], "live-deal-1")
            self.assertEqual(fetch_info.get("source"), "live_api")
    def test_crm_competitor_and_category_rejection(self):
        hubspot_deal = {
            "name": "Fluid CRM",
            "slug": "fluid-crm",
            "price": 59.0,
            "original_price": 192.0,
            "average_rating": 5.0,
            "review_count": 29,
            "category": "crm",
            "subcategory": "CRM",
            "alternative_to": ["HubSpot", "Pipedrive"],
            "card_description": "Organize leads, manage follow-ups, and move deals forward with a calm visual CRM",
        }
        rec, reason, score = self.auditor.audit_deal(hubspot_deal)
        self.assertFalse(rec)
        self.assertIn("FluentCRM", reason)

    def test_influencer_discovery_rejection(self):
        modash_deal = {
            "name": "Creator Lookup",
            "slug": "creator-lookup",
            "price": 59.0,
            "original_price": 180.0,
            "average_rating": 4.7,
            "review_count": 10,
            "category": "social-media",
            "subcategory": "Influencer & UGC",
            "alternative_to": ["Modash", "Upfluence"],
            "card_description": "Search an AI-powered influencer database of 400M+ creators",
        }
        rec, reason, score = self.auditor.audit_deal(modash_deal)
        self.assertFalse(rec)
        self.assertIn("EezyCollab", reason)

    def test_security_scanning_and_dependency_graph_rejection(self):
        rafter_deal = {
            "name": "Rafter",
            "slug": "rafter",
            "price": 39.0,
            "original_price": 180.0,
            "average_rating": 4.82,
            "review_count": 22,
            "category": "developer-tools",
            "subcategory": "Dev Utilities",
            "card_description": "Scan GitHub repos for vulnerabilities, secrets, and security issues with AI-ready fixes",
        }
        rec, reason, score = self.auditor.audit_deal(rafter_deal)
        self.assertFalse(rec)
        self.assertIn("security-scan", reason)

        vexp_deal = {
            "name": "Vexp",
            "slug": "vexp",
            "price": 49.0,
            "original_price": 199.0,
            "average_rating": 4.57,
            "review_count": 30,
            "category": "developer-tools",
            "subcategory": "Dev Utilities",
            "card_description": "Build a local dependency graph and serve precise context to AI coding agents across VS Code and CLI",
        }
        rec, reason, score = self.auditor.audit_deal(vexp_deal)
        self.assertFalse(rec)
        self.assertIn("Graphify", reason)

    def test_multilingual_and_trading_rejection(self):
        multilipi_deal = {
            "name": "MultiLipi",
            "slug": "multilipi",
            "price": 79.0,
            "original_price": 360.0,
            "average_rating": 4.69,
            "review_count": 54,
            "category": "seo-analytics",
            "card_description": "Translate your website into 120+ languages to rank high on Google and get cited by ChatGPT or Gemini",
        }
        rec, reason, score = self.auditor.audit_deal(multilipi_deal)
        self.assertFalse(rec)
        self.assertTrue("WordPress 原生多语言架构" in reason or "Squirrly SEO" in reason)

        probors_deal = {
            "name": "ProBors",
            "slug": "probors",
            "price": 49.0,
            "original_price": 240.0,
            "average_rating": 4.75,
            "review_count": 18,
            "category": "finance",
            "card_description": "Follow insider, politician, and whale trades with real-time filings, signal scoring, and AI research",
        }
        rec, reason, score = self.auditor.audit_deal(probors_deal)
        self.assertFalse(rec)
        self.assertIn("核心主线", reason)

    def test_cloud_doc_directives_integration(self):
        from appsumo_buy_advisor import (
            extract_notion_block_text,
            build_human_directives_blocks,
            DealAuditor,
        )

        # 1. Test block text extraction
        sample_b = {
            "type": "heading_2",
            "heading_2": {
                "rich_text": [
                    {"type": "text", "plain_text": "Hello "},
                    {"type": "text", "plain_text": "Notion Cloud"},
                ]
            }
        }
        self.assertEqual(extract_notion_block_text(sample_b), "Hello Notion Cloud")

        # 2. Test build human directives template
        blocks = build_human_directives_blocks()
        self.assertTrue(len(blocks) >= 5)
        self.assertEqual(blocks[0]["type"], "callout")
        self.assertEqual(blocks[1]["type"], "toggle")

        # 3. Test Auditor with user directives (Blacklist, New Assets, Wishlist)
        directives = {
            "blacklist": ["bad-crm", "unwanted-tool"],
            "new_assets": ["MyCustomTool"],
            "wishlist": ["video-dubbing"],
            "feedback": ["prior run was great"],
            "raw_directives_found": True,
        }
        custom_auditor = DealAuditor(self.kb, user_directives=directives)

        # 3A: Blacklist Veto test
        blacklisted_deal = {
            "name": "Bad CRM Pro",
            "slug": "bad-crm",
            "price": 49.0,
            "original_price": 200.0,
            "average_rating": 4.9,
            "review_count": 50,
            "category": "sales",
            "card_description": "A very bad crm",
        }
        rec, reason, score = custom_auditor.audit_deal(blacklisted_deal)
        self.assertFalse(rec)
        self.assertIn("人工黑名单禁令", reason)

        # 3B: Declared Owned Asset test
        owned_custom_deal = {
            "name": "MyCustomTool Suite",
            "slug": "mycustomtool-suite",
            "price": 39.0,
            "original_price": 100.0,
            "average_rating": 4.8,
            "review_count": 40,
            "category": "developer-tools",
            "card_description": "My custom tool in action",
        }
        rec, reason, score = custom_auditor.audit_deal(owned_custom_deal)
        self.assertFalse(rec)
        self.assertIn("已拥有资产", reason)

        # 3C: Wishlist Boost test
        wishlist_deal = {
            "name": "OmniDub AI",
            "slug": "omnidub-ai",
            "price": 49.0,
            "original_price": 199.0,
            "average_rating": 4.85,
            "review_count": 60,
            "category": "video",
            "subcategory": "Video AI",
            "card_description": "Native video dubbing and voice translation for international creators",
        }
        rec, reason, score = custom_auditor.audit_deal(wishlist_deal)
        self.assertTrue(rec)
        self.assertIn("关注意向", reason)
        self.assertTrue(score > 70.0)

    def test_manychat_primary_capability_gap_exemption(self):
        # A pure social scheduler (SocialBu) with alternative_to=["Hootsuite", "Buffer"] must still be REJECTED
        scheduler_deal = {
            "name": "SocialBu",
            "slug": "socialbu",
            "price": 49.0,
            "original_price": 400.0,
            "average_rating": 4.64,
            "review_count": 50,
            "category": "social-media",
            "subcategory": "Social Media Management",
            "alternative_to": ["Buffer", "Hootsuite"],
            "card_description": "Schedule, publish, analyze, and grow social media presence across 12+ platforms",
        }
        rec_sched, reason_sched, _ = self.auditor.audit_deal(scheduler_deal)
        self.assertFalse(rec_sched)
        self.assertIn("Zernio", reason_sched)

        # An Instagram Comment-to-DM / ManyChat alternative (Inrō) is covered by Zernio Usage-Based /v1/comment-automations and must be REJECTED
        inro_deal = {
            "name": "Inrō",
            "slug": "inro",
            "price": 59.0,
            "original_price": 708.0,
            "average_rating": 5.0,
            "review_count": 36,
            "category": "social-media",
            "subcategory": "Social Media Management",
            "alternative_to": ["Hootsuite", "ManyChat"],
            "card_description": "Automate Instagram DMs and comment replies with an AI agent that sounds like you and drives sales",
        }
        res_inro = self.auditor.audit_deal_steps(inro_deal)
        self.assertFalse(res_inro["should_recommend"])
        self.assertEqual(res_inro["verdict"], "REJECTED")
        self.assertIn("Zernio", res_inro["reason"])

    def test_ai_video_competitor_rejected(self):
        # MagicCanvas claiming alternative to Higgsfield, Kapwing, Freepik
        magiccanvas_deal = {
            "name": "MagicCanvas",
            "slug": "magiccanvas",
            "price": 59.0,
            "original_price": 199.0,
            "average_rating": 4.7,
            "review_count": 10,
            "category": "video",
            "subcategory": "AI Video",
            "alternative_to": ["Higgsfield", "Kapwing", "Freepik"],
            "card_description": "Tap into Google Veo, Seedance, Wan, and more with this all-in-one AI video and image generator",
        }
        res = self.auditor.audit_deal_steps(magiccanvas_deal)
        self.assertFalse(res["should_recommend"])
        self.assertEqual(res["verdict"], "REJECTED")
        self.assertIn("seedance-video-generator", res["reason"])


class TestSlackNotification(unittest.TestCase):
    def test_missing_token_returns_skipped_hermetically(self):
        result = {
            "audited_count": 540,
            "owned_assets_indexed": 69,
            "recommended": [],
        }
        with mock.patch.dict(os.environ, {"SLACK_BOT_TOKEN": "", "SLACK_TOKEN": ""}, clear=True):
            with mock.patch("pathlib.Path.exists", return_value=False):
                res = notify_slack_summary(
                    result,
                    notion_page_id="3f2e1b43-2393-815a-97e2-e06c9efcbfdc",
                    channel="fake_channel",
                    token="",
                )
                self.assertEqual(res["status"], "skipped")
                self.assertIsNone(res["ts"])

    @mock.patch("urllib.request.urlopen")
    def test_successful_slack_notification(self, mock_urlopen):
        class FakeResponse:
            def read(self):
                return b'{"ok": true, "ts": "1234567890.123456"}'
            def __enter__(self):
                return self
            def __exit__(self, *args):
                pass

        mock_urlopen.return_value = FakeResponse()
        result = {
            "audited_count": 540,
            "owned_assets_indexed": 69,
            "recommended": [
                {
                    "name": "WP Reset Pro",
                    "price": 49.0,
                    "url": "https://appsumo.com/products/wp-reset/",
                    "reason": "WordPress 应急与恢复工具",
                }
            ],
        }
        res = notify_slack_summary(
            result,
            notion_page_id="3f2e1b43-2393-815a-97e2-e06c9efcbfdc",
            channel="C091GLVKLJU",
            token="xoxp-fake-token",
        )
        self.assertEqual(res["status"], "posted")
        self.assertEqual(res["ts"], "1234567890.123456")
        self.assertTrue(mock_urlopen.called)
        req = mock_urlopen.call_args[0][0]
        self.assertIn("chat.postMessage", req.full_url)

    @mock.patch("urllib.request.urlopen")
    def test_slack_notification_updates_existing_message_with_thread(self, mock_urlopen):
        call_count = 0
        def fake_urlopen(req, timeout=10):
            nonlocal call_count
            call_count += 1
            class FakeResp:
                def __init__(self, count):
                    self.count = count
                def read(self):
                    if self.count == 1:
                        return b'{"ok": true, "ts": "new_updated_ts_9999"}'
                    return b'{"ok": true, "ts": "thread_reply_ts_8888"}'
                def __enter__(self):
                    return self
                def __exit__(self, *args):
                    pass
            return FakeResp(call_count)

        mock_urlopen.side_effect = fake_urlopen
        result = {
            "audited_count": 550,
            "owned_assets_indexed": 69,
            "recommended": [],
        }
        res = notify_slack_summary(
            result,
            notion_page_id="3f2e1b43-2393-815a-97e2-e06c9efcbfdc",
            channel="C091GLVKLJU",
            token="xoxp-fake-token",
            existing_ts="1111111111.222222",
        )
        self.assertEqual(res["status"], "updated")
        self.assertEqual(res["ts"], "new_updated_ts_9999")
        self.assertEqual(call_count, 2)
        # Call 1 was chat.update
        req1 = mock_urlopen.call_args_list[0][0][0]
        self.assertIn("chat.update", req1.full_url)
        body1 = json.loads(req1.data.decode("utf-8"))
        self.assertEqual(body1.get("ts"), "1111111111.222222")
        # Call 2 was threaded reply on existing_ts
        req2 = mock_urlopen.call_args_list[1][0][0]
        self.assertIn("chat.postMessage", req2.full_url)
        body2 = json.loads(req2.data.decode("utf-8"))
        self.assertEqual(body2.get("thread_ts"), "1111111111.222222")

    @mock.patch("urllib.request.urlopen")
    def test_slack_notification_fallback_if_message_deleted(self, mock_urlopen):
        call_count = 0
        def fake_urlopen(req, timeout=10):
            nonlocal call_count
            call_count += 1
            class FakeResp:
                def __init__(self, count):
                    self.count = count
                def read(self):
                    if self.count == 1:
                        return b'{"ok": false, "error": "message_not_found"}'
                    return b'{"ok": true, "ts": "fallback_post_ts_7777"}'
                def __enter__(self):
                    return self
                def __exit__(self, *args):
                    pass
            return FakeResp(call_count)

        mock_urlopen.side_effect = fake_urlopen
        result = {
            "audited_count": 550,
            "owned_assets_indexed": 69,
            "recommended": [],
        }
        res = notify_slack_summary(
            result,
            notion_page_id="3f2e1b43-2393-815a-97e2-e06c9efcbfdc",
            channel="C091GLVKLJU",
            token="xoxp-fake-token",
            existing_ts="deleted_ts_123",
        )
        self.assertEqual(res["status"], "posted")
        self.assertEqual(res["ts"], "fallback_post_ts_7777")
        self.assertEqual(call_count, 2)
        # Call 2 must be chat.postMessage and must NOT have "ts"
        req2 = mock_urlopen.call_args_list[1][0][0]
        self.assertIn("chat.postMessage", req2.full_url)
        body2 = json.loads(req2.data.decode("utf-8"))
        self.assertNotIn("ts", body2)

    @mock.patch("time.sleep", return_value=None)
    @mock.patch("urllib.request.urlopen")
    def test_slack_rate_limit_backoff_and_retry(self, mock_urlopen, mock_sleep):
        call_count = 0
        def fake_urlopen(req, timeout=10):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                fp = mock.MagicMock()
                fp.read.return_value = b'{"ok": false, "error": "ratelimited"}'
                headers = {"Retry-After": "2"}
                raise urllib.error.HTTPError("https://slack.com", 429, "Too Many Requests", headers, fp)
            class FakeResp:
                def read(self):
                    return b'{"ok": true, "ts": "retry_success_ts"}'
                def __enter__(self):
                    return self
                def __exit__(self, *args):
                    pass
            return FakeResp()

        mock_urlopen.side_effect = fake_urlopen
        result = {"audited_count": 100, "owned_assets_indexed": 69, "recommended": []}
        res = notify_slack_summary(result, "fake-notion-id", channel="C091GLVKLJU", token="xoxp-fake")
        self.assertEqual(res["status"], "posted")
        self.assertEqual(res["ts"], "retry_success_ts")
        self.assertEqual(call_count, 2)
        self.assertTrue(mock_sleep.called)

    @mock.patch("urllib.request.urlopen")
    def test_block_kit_overflow_budget_protection(self, mock_urlopen):
        class FakeResponse:
            def read(self):
                return b'{"ok": true, "ts": "overflow_test_ts"}'
            def __enter__(self):
                return self
            def __exit__(self, *args):
                pass

        mock_urlopen.return_value = FakeResponse()
        twenty_deals = [
            {"name": f"Deal {i}", "price": 49.0, "url": f"https://appsumo.com/deal-{i}", "reason": f"Reason {i}"}
            for i in range(20)
        ]
        result = {"audited_count": 500, "owned_assets_indexed": 69, "recommended": twenty_deals}
        res = notify_slack_summary(result, "fake-notion-id", channel="C091GLVKLJU", token="xoxp-fake")
        self.assertEqual(res["status"], "posted")
        req = mock_urlopen.call_args[0][0]
        body = json.loads(req.data.decode("utf-8"))
        blocks = body.get("blocks", [])
        self.assertLessEqual(len(blocks), 40)
        overflow_block = any("另有" in str(b) and "Notion" in str(b) for b in blocks)
        self.assertTrue(overflow_block)


class TestDeliveryPipeline(unittest.TestCase):
    def test_write_state_atomic(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            target = Path(tmpdir) / "state.json"
            write_state_atomic(target, {"hello": "world", "status": "ok"})
            self.assertTrue(target.exists())
            data = json.loads(target.read_text(encoding="utf-8"))
            self.assertEqual(data["hello"], "world")
            self.assertEqual(data["status"], "ok")

    @mock.patch("appsumo_buy_advisor.fetch_cloud_doc_directives", return_value={})
    @mock.patch("appsumo_buy_advisor.sync_co_maintained_notion_doc")
    @mock.patch("appsumo_buy_advisor.fetch_candidate_deals")
    @mock.patch("appsumo_buy_advisor.notify_slack_summary")
    def test_run_advisor_delivery_failure_withholds_success_marker(self, mock_slack, mock_deals, mock_notion_sync, mock_directives):
        mock_deals.return_value = ([
            {"name": "Valid Deal", "slug": "valid-deal", "price": 49.0, "average_rating": 4.9, "review_count": 100}
        ], {"complete": True, "requests": 1, "declared_total": 1})
        mock_slack.return_value = {"status": "failed", "ts": None, "error": "slack_timeout"}

        with tempfile.TemporaryDirectory() as tmpdir:
            fake_assets = Path(tmpdir) / "assets.md"
            fake_assets.write_text("# Assets\n", encoding="utf-8")
            with mock.patch("appsumo_buy_advisor.CADENCE_ID", "TEST-CADENCE"):
                state_dir = Path(__file__).resolve().parent.parent / ".run/cadence" / "TEST-CADENCE"
                state_dir.mkdir(parents=True, exist_ok=True)
                iso_year, iso_week, _ = datetime.datetime.now(datetime.timezone.utc).isocalendar()
                marker = state_dir / f"{iso_year}-W{iso_week:02d}.success"
                marker.unlink(missing_ok=True)

                with self.assertRaises(DeliveryError):
                    run_advisor(
                        assets_file=fake_assets,
                        notion_page_id="fake-page",
                        limit=1,
                        force=True,
                        notify_slack=True,
                    )
                # Success marker must NOT exist because delivery failed
                self.assertFalse(marker.exists())
                # State file must exist and record failure
                state_file = state_dir / "state.json"
                self.assertTrue(state_file.exists())
                state = json.loads(state_file.read_text(encoding="utf-8"))
                self.assertEqual(state["slack"]["status"], "failed")
                self.assertFalse(state["slack_delivered"])
                # Clean up test state dir
                shutil.rmtree(state_dir, ignore_errors=True)

    @mock.patch("appsumo_buy_advisor.fetch_cloud_doc_directives", return_value={})
    @mock.patch("appsumo_buy_advisor.sync_co_maintained_notion_doc")
    @mock.patch("appsumo_buy_advisor.fetch_candidate_deals")
    @mock.patch("appsumo_buy_advisor.notify_slack_summary")
    def test_run_advisor_delivery_success_writes_marker(self, mock_slack, mock_deals, mock_notion_sync, mock_directives):
        mock_deals.return_value = ([
            {"name": "Valid Deal", "slug": "valid-deal", "price": 49.0, "average_rating": 4.9, "review_count": 100}
        ], {"complete": True, "requests": 1, "declared_total": 1})
        mock_slack.return_value = {"status": "posted", "ts": "success_ts_1234", "error": None}

        with tempfile.TemporaryDirectory() as tmpdir:
            fake_assets = Path(tmpdir) / "assets.md"
            fake_assets.write_text("# Assets\n", encoding="utf-8")
            with mock.patch("appsumo_buy_advisor.CADENCE_ID", "TEST-CADENCE"):
                state_dir = Path(__file__).resolve().parent.parent / ".run/cadence" / "TEST-CADENCE"
                state_dir.mkdir(parents=True, exist_ok=True)
                iso_year, iso_week, _ = datetime.datetime.now(datetime.timezone.utc).isocalendar()
                marker = state_dir / f"{iso_year}-W{iso_week:02d}.success"
                marker.unlink(missing_ok=True)

                res = run_advisor(
                    assets_file=fake_assets,
                    notion_page_id="fake-page",
                    limit=1,
                    force=True,
                    notify_slack=True,
                )
                self.assertIn(res["status"], ("recommended", "no_purchase_needed"))
                self.assertTrue(marker.exists())
                state_file = state_dir / "state.json"
                self.assertTrue(state_file.exists())
                state = json.loads(state_file.read_text(encoding="utf-8"))
                self.assertEqual(state["slack"]["status"], "posted")
                self.assertTrue(state["slack_delivered"])
                self.assertEqual(state["slack_ts"], "success_ts_1234")
                shutil.rmtree(state_dir, ignore_errors=True)

    @mock.patch("appsumo_buy_advisor.notify_slack_summary")
    def test_redeliver_notifications(self, mock_slack):
        mock_slack.return_value = {"status": "posted", "ts": "redelivered_ts_999", "error": None}
        with tempfile.TemporaryDirectory() as tmpdir:
            state_dir = Path(tmpdir)
            receipt_file = state_dir / "2026-11-10.md"
            receipt_file.write_text("### 推荐产品清单\n- **[Test Tool](https://appsumo.com)** ($49, 原价 $100, 5.0⭐️/10条评价):\n  - **匹配原因**: 测试匹配\n", encoding="utf-8")
            state_file = state_dir / "state.json"
            state_file.write_text(json.dumps({
                "cadence_id": "TEST",
                "week_key": "2026-W46",
                "last_status": "recommended",
                "audited_count": 10,
                "receipt_path": str(receipt_file),
                "slack_ts": None,
            }), encoding="utf-8")

            res = redeliver_notifications(state_dir, notion_page_id="fake-page", notify_slack=True)
            self.assertEqual(res["status"], "redelivered")
            self.assertEqual(res["slack"]["status"], "posted")
            self.assertEqual(res["slack"]["ts"], "redelivered_ts_999")
            # Marker must be created
            marker = state_dir / "2026-W46.success"
            self.assertTrue(marker.exists())


if __name__ == "__main__":
    unittest.main()


