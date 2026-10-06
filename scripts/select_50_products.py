#!/usr/bin/env python3
"""
Select 50 diverse AppSumo products for stress-testing and reconciliation.
Stratifies across review counts, categories, pricing, and edge cases.
"""

import json
import subprocess
import sys
from typing import List, Dict, Any

def get_deal_catalog() -> List[Dict[str, Any]]:
    # Call appsumo deals list --limit 250 --json
    cmd = ["appsumo", "deals", "list", "--limit", "250", "--json"]
    proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if proc.returncode != 0:
        print("Error fetching deals:", proc.stderr, file=sys.stderr)
        sys.exit(1)
    
    # Strip any warning lines if present
    stdout = proc.stdout.strip()
    idx = stdout.find("{")
    if idx != -1:
        stdout = stdout[idx:]
    data = json.loads(stdout)
    return data.get("deals", [])

def select_50_deals() -> List[str]:
    deals = get_deal_catalog()
    print(f"Total catalog deals retrieved: {len(deals)}")

    # Always include poppy-ai
    selected_slugs = ["poppy-ai"]
    selected_deals = []

    # Buckets:
    high_reviews = []    # >= 100
    med_reviews = []     # 20 - 99
    low_reviews = []     # 1 - 19
    zero_reviews = []    # 0
    free_deals = []      # price == 0 or is_free == True

    for d in deals:
        slug = d.get("slug")
        if not slug or slug in selected_slugs:
            continue
        price = d.get("price", 0)
        is_free = d.get("is_free", False)
        rc = d.get("review_count", 0) or 0

        if is_free or price == 0:
            free_deals.append(d)
        elif rc >= 100:
            high_reviews.append(d)
        elif rc >= 20:
            med_reviews.append(d)
        elif rc >= 1:
            low_reviews.append(d)
        else:
            zero_reviews.append(d)

    print(f"Buckets: Free={len(free_deals)}, High={len(high_reviews)}, Med={len(med_reviews)}, Low={len(low_reviews)}, Zero={len(zero_reviews)}")

    # Add free deals (up to 5)
    for d in free_deals[:5]:
        selected_slugs.append(d["slug"])

    # Add high reviews (up to 10)
    for d in high_reviews[:10]:
        if d["slug"] not in selected_slugs:
            selected_slugs.append(d["slug"])

    # Add med reviews (up to 15)
    for d in med_reviews[:15]:
        if d["slug"] not in selected_slugs:
            selected_slugs.append(d["slug"])

    # Add low reviews (up to 12)
    for d in low_reviews[:12]:
        if d["slug"] not in selected_slugs:
            selected_slugs.append(d["slug"])

    # Add zero reviews (up to 8)
    for d in zero_reviews[:8]:
        if d["slug"] not in selected_slugs:
            selected_slugs.append(d["slug"])

    # Fill remaining from anywhere in deals until exactly 50
    for d in deals:
        if len(selected_slugs) >= 50:
            break
        slug = d.get("slug")
        if slug and slug not in selected_slugs:
            selected_slugs.append(slug)

    print(f"Selected {len(selected_slugs)} diverse products.")
    return selected_slugs[:50]

if __name__ == "__main__":
    slugs = select_50_deals()
    with open("scripts/selected_50_slugs.json", "w", encoding="utf-8") as f:
        json.dump(slugs, f, indent=2)
    print("Saved to scripts/selected_50_slugs.json")
    for idx, s in enumerate(slugs, 1):
        print(f"{idx}. {s}")
