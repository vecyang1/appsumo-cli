#!/usr/bin/env python3
"""
AppSumo Agent-Friendly Product Scraper & Cache Tool

Scrapes, aggregates, saves, and caches all public product specifications,
pricing tiers, terms, founders background, FAQs, public reviews, and questions.

Usage:
    python3 appsumo_scraper.py https://appsumo.com/products/poppy-ai/
    python3 appsumo_scraper.py poppy-ai --out-dir ./data/poppy-ai
"""

import argparse
import json
import os
import re
import subprocess
import sys
from urllib.parse import urlparse

def clean_slug(raw: str) -> str:
    raw = raw.strip()
    if raw.startswith("http://") or raw.startswith("https://"):
        u = urlparse(raw)
        path = u.path.strip("/")
        parts = path.split("/")
        for i, p in enumerate(parts):
            if p == "products" and i + 1 < len(parts):
                return parts[i + 1]
        if parts:
            return parts[-1]
    return raw.strip("/").split("/")[-1]

def run_go_scraper(slug: str, out_dir: str, no_reviews: bool, no_questions: bool, as_json: bool) -> int:
    cmd = ["appsumo", "scrape", slug, "--out-dir", out_dir]
    if no_reviews:
        cmd.append("--no-reviews")
    if no_questions:
        cmd.append("--no-questions")
    if as_json:
        cmd.append("--json")
    
    return subprocess.call(cmd)

def main():
    parser = argparse.ArgumentParser(description="AppSumo Agentic Product Scraper & Cache CLI")
    parser.add_argument("slug_or_url", help="Product slug (e.g. poppy-ai) or full AppSumo URL")
    parser.add_argument("-o", "--out-dir", default="", help="Directory to save scraped cache files")
    parser.add_argument("--no-reviews", action="store_true", help="Skip reviews scraping")
    parser.add_argument("--no-questions", action="store_true", help="Skip questions scraping")
    parser.add_argument("--json", action="store_true", help="Output summary in JSON format")
    
    args = parser.parse_args()
    slug = clean_slug(args.slug_or_url)
    out_dir = args.out_dir or os.path.join(os.getcwd(), "data", slug)

    exit_code = run_go_scraper(slug, out_dir, args.no_reviews, args.no_questions, args.json)
    sys.exit(exit_code)

if __name__ == "__main__":
    main()
