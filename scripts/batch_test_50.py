#!/usr/bin/env python3
"""
AppSumo 50-Product Diagnostic & Reconciliation Harness
Runs `appsumo scrape <slug> --json` across 50 diverse products.
Validates end-to-end data integrity, file artifacts, and generates a reconciliation table.
"""

import json
import os
import subprocess
import sys
import time
from typing import Dict, Any, List

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
DATA_DIR = os.path.join(ROOT_DIR, "data")
APP_SUMO_CLI = os.path.expanduser("~/.local/bin/appsumo")

EXPECTED_FILES = [
    "deal.json",
    "reviews.json",
    "questions.json",
    "faqs.json",
    "founders.json",
    "full_archive.json",
    "DOSSIER.md",
    "SUMMARY.md",
]

def run_scrape(slug: str) -> Dict[str, Any]:
    out_dir = os.path.join(DATA_DIR, slug)
    start_time = time.time()
    cmd = [APP_SUMO_CLI, "scrape", slug, "--out-dir", out_dir, "--json"]
    try:
        proc = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=180,
        )
        elapsed = round(time.time() - start_time, 2)
        if proc.returncode != 0:
            return {
                "slug": slug,
                "success": False,
                "elapsed": elapsed,
                "error": proc.stderr.strip() or f"exit code {proc.returncode}",
            }
        
        # Parse JSON output from stdout
        raw_stdout = proc.stdout.strip()
        idx = raw_stdout.find("{")
        if idx != -1:
            raw_stdout = raw_stdout[idx:]
        data = json.loads(raw_stdout)
        
        # Check files on disk
        missing_files = []
        file_sizes = {}
        for fname in EXPECTED_FILES:
            fpath = os.path.join(out_dir, fname)
            if not os.path.exists(fpath) or os.path.getsize(fpath) == 0:
                missing_files.append(fname)
            else:
                file_sizes[fname] = os.path.getsize(fpath)
                
        return {
            "slug": slug,
            "success": len(missing_files) == 0,
            "elapsed": elapsed,
            "data": data,
            "missing_files": missing_files,
            "file_sizes": file_sizes,
        }
    except Exception as e:
        elapsed = round(time.time() - start_time, 2)
        return {
            "slug": slug,
            "success": False,
            "elapsed": elapsed,
            "error": str(e),
        }

def generate_reconciliation_report(results: List[Dict[str, Any]], out_path: str):
    total = len(results)
    passed = sum(1 for r in results if r.get("success"))
    failed = total - passed
    total_reviews = sum(r.get("data", {}).get("reviews_scraped", 0) for r in results if r.get("success"))
    total_questions = sum(r.get("data", {}).get("questions_scraped", 0) for r in results if r.get("success"))
    total_faqs = sum(r.get("data", {}).get("faqs_count", 0) for r in results if r.get("success"))
    total_founders = sum(r.get("data", {}).get("founder_posts_count", 0) for r in results if r.get("success"))
    avg_latency = round(sum(r.get("elapsed", 0) for r in results) / max(1, total), 2)
    
    total_bytes = 0
    for r in results:
        for size in r.get("file_sizes", {}).values():
            total_bytes += size
    total_mb = round(total_bytes / (1024 * 1024), 2)

    lines = []
    lines.append("# AppSumo 50-Product Diagnostic & Reconciliation Report")
    lines.append("")
    lines.append(f"- **Execution Date**: {time.strftime('%Y-%m-%d %H:%M:%S UTC', time.gmtime())}")
    lines.append(f"- **Total Products Tested**: `{total}`")
    lines.append(f"- **Passed**: `{passed}/{total}` (`{round(passed/max(1, total)*100, 1)}%`)")
    lines.append(f"- **Failed**: `{failed}`")
    lines.append(f"- **Total Reviews Scraped**: `{total_reviews:,}`")
    lines.append(f"- **Total Questions Scraped**: `{total_questions:,}`")
    lines.append(f"- **Total FAQs Extracted**: `{total_faqs:,}`")
    lines.append(f"- **Total Founder Posts Extracted**: `{total_founders:,}`")
    lines.append(f"- **Total Disk Cache Generated**: `{total_mb} MB` (across 8 artifacts per product)")
    lines.append(f"- **Average Scrape Latency**: `{avg_latency}s`")
    lines.append("")
    lines.append("## Detailed Reconciliation Table (对账明细)")
    lines.append("")
    lines.append("| # | Product Name | Slug | Tiers | Declared Reviews | Scraped Reviews | Scraped Q&A | FAQs | Founders | Files Status | Latency | Result |")
    lines.append("|---|---|---|---|---|---|---|---|---|---|---|---|")

    for idx, r in enumerate(results, 1):
        slug = r.get("slug", "unknown")
        success = r.get("success", False)
        elapsed = r.get("elapsed", 0)
        data = r.get("data", {})
        name = data.get("name", slug)
        tiers = data.get("tiers_count", "N/A")
        decl_rev = data.get("total_review_count", "N/A")
        scraped_rev = data.get("reviews_scraped", 0) if success else 0
        scraped_q = data.get("questions_scraped", 0) if success else 0
        faqs = data.get("faqs_count", 0) if success else 0
        founders = data.get("founder_posts_count", 0) if success else 0
        
        missing = r.get("missing_files", [])
        if success:
            files_status = "8/8 OK"
            result_badge = "✅ PASS"
        else:
            files_status = f"Missing: {','.join(missing)}" if missing else f"Err: {r.get('error', '')[:30]}"
            result_badge = "❌ FAIL"

        lines.append(f"| {idx} | **{name}** | `{slug}` | {tiers} | {decl_rev} | {scraped_rev} | {scraped_q} | {faqs} | {founders} | {files_status} | {elapsed}s | {result_badge} |")

    lines.append("")
    lines.append("## Artifact Invariants Check")
    lines.append("Every successfully passed product generated all 8 required artifacts:")
    lines.append("1. `deal.json`: Core product metadata, specifications, licensing tiers, pricing, founders, tags.")
    lines.append("2. `reviews.json`: Public buyer reviews, tacos ratings, badges, verified purchase marks.")
    lines.append("3. `questions.json`: Q&A threads, buyer inquiries, founder answers, pinned status.")
    lines.append("4. `faqs.json`: Extracted structured product FAQs.")
    lines.append("5. `founders.json`: Founder updates, welcome letters, roadmap notes.")
    lines.append("6. `full_archive.json`: Consolidated raw JSON payload for agentic digestion.")
    lines.append("7. `DOSSIER.md`: Comprehensive product dossier formatted for human analysis.")
    lines.append("8. `SUMMARY.md`: Executive summary of key findings, strengths, and friction points.")
    lines.append("")

    content = "\n".join(lines)
    os.makedirs(os.path.dirname(out_path), exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"Reconciliation report saved to {out_path}")

if __name__ == "__main__":
    slugs_file = os.path.join(os.path.dirname(__file__), "selected_50_slugs.json")
    if len(sys.argv) > 1:
        slugs = sys.argv[1:]
    elif os.path.exists(slugs_file):
        with open(slugs_file, "r", encoding="utf-8") as f:
            slugs = json.load(f)
    else:
        print("No slugs provided and selected_50_slugs.json not found.", file=sys.stderr)
        sys.exit(1)

    print(f"Starting batch diagnostic run for {len(slugs)} products...")
    results = []
    for idx, slug in enumerate(slugs, 1):
        print(f"[{idx}/{len(slugs)}] Processing '{slug}'...", flush=True)
        res = run_scrape(slug)
        status_str = "PASS" if res.get("success") else "FAIL"
        print(f"    -> [{status_str}] in {res.get('elapsed')}s", flush=True)
        if not res.get("success"):
            print(f"       Error: {res.get('error')}", flush=True)
        results.append(res)

    results_json = os.path.join(ROOT_DIR, "data", "batch_50_results.json")
    with open(results_json, "w", encoding="utf-8") as f:
        json.dump(results, f, ensure_ascii=False, indent=2)
    print(f"Saved raw batch results to {results_json}")

    report_md = os.path.join(ROOT_DIR, "docs", "RECONCILIATION_REPORT_50.md")
    generate_reconciliation_report(results, report_md)
