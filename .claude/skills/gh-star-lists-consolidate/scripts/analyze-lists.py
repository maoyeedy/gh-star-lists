#!/usr/bin/env python3
"""Analyze gh star-lists --json output.

Usage:
  gh star-lists --json | python scripts/analyze-lists.py [--min-prefix-group N]

Outputs a JSON analysis: list details, prefix groupings, empty lists, and
semantic word overlap hints. Does NOT recommend merges — the AI uses this
data to decide.
"""

import json, sys, os
from collections import defaultdict, Counter

data = json.load(sys.stdin)

# Prefix grouping: take the first dash-separated segment
def prefix(name: str) -> str:
    return name.split("-")[0].lower()

# Word-level grouping (for semantic overlap)
def words(name: str) -> list[str]:
    return [w.lower() for w in name.replace("-", " ").replace("&", "").replace("/", " ").split() if len(w) > 1]

prefix_groups = defaultdict(list)
for lst in data:
    prefix_groups[prefix(lst["name"])].append(lst)

# Count total
total_repos = sum(lst["repoCount"] for lst in data)

# Word frequency across all list names (to spot patterns/overlaps)
all_words = []
for lst in data:
    all_words.extend(words(lst["name"]))
word_freq = Counter(all_words)

# Shared-word clusters: lists that share >= 2 significant words
significant_words = {w for w, c in word_freq.items() if c >= 2}
word_clusters = defaultdict(list)
for lst in data:
    lst_words = [w for w in words(lst["name"]) if w in significant_words]
    key = tuple(sorted(lst_words))
    if len(key) >= 2:
        word_clusters[key].append(lst["name"])

output = {
    "total_lists": len(data),
    "total_repos": total_repos,
    "lists": sorted([
        {"name": lst["name"], "id": lst["id"], "repoCount": lst["repoCount"], "prefix": prefix(lst["name"])}
        for lst in data
    ], key=lambda x: x["repoCount"]),

    "prefix_groups": {
        p: [{"name": l["name"], "repoCount": l["repoCount"], "id": l["id"]} for l in group]
        for p, group in prefix_groups.items() if len(group) > 1
    },

    "empty_lists": [{"name": lst["name"], "id": lst["id"]} for lst in data if lst["repoCount"] == 0],

    "word_clusters": [
        {"shared_words": list(k), "lists": v}
        for k, v in word_clusters.items()
    ],

    "prefix_groups_to_merge": [],

    "summary_stats": {
        "counts_0": sum(1 for l in data if l["repoCount"] == 0),
        "counts_1": sum(1 for l in data if l["repoCount"] == 1),
        "counts_2": sum(1 for l in data if l["repoCount"] == 2),
        "counts_3_5": sum(1 for l in data if 3 <= l["repoCount"] <= 5),
        "counts_6_plus": sum(1 for l in data if l["repoCount"] >= 6),
    }
}

print(json.dumps(output, indent=2))
