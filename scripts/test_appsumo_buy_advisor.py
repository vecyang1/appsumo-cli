#!/usr/bin/env python3
"""
Unit tests for AppSumo Buy Advisor & Asset-Aware Recommender
"""

import unittest
from pathlib import Path
from tempfile import NamedTemporaryFile
import json
import sys

# Ensure scripts dir is on sys.path
sys.path.insert(0, str(Path(__file__).resolve().parent))

from appsumo_buy_advisor import (
    AssetKnowledgeBase,
    DealAuditor,
    COMPETITOR_TO_OWNED_ASSET,
    SATURATED_SUBCATEGORIES,
    build_notion_blocks,
    clean_product_slug,
    explain_deal,
    fetch_candidate_deals,
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
            self.assertEqual(mock_run.call_count, 2)


if __name__ == "__main__":
    unittest.main()


