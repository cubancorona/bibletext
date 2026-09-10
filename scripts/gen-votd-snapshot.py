#!/usr/bin/env python3
"""Regenerate testdata/verse_of_day_snapshot.json from the decoded caches.

The verse-of-the-day tests must be able to prove, OFFLINE and in CI, that every
rotation entry resolves in every shipped public-domain edition and how each
passage begins and ends — the caches themselves are downloaded, not committed.
This snapshot records, per edition and per entry, whether the passage is
present, the first and last rune of its text, and (for a book the edition lacks)
which alternate stood in. Run it whenever verse_of_day.go's list changes:

    scripts/gen-votd-snapshot.py

It reads the caches the app writes (~/Library/Caches/bibletext on macOS) and
refuses to write a snapshot from a cache that is missing.
"""
import json, os, re, sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CACHE = os.path.expanduser("~/Library/Caches/bibletext")
EDITIONS = {"web": "bibletext-web-v8.json", "bsb": "bibletext-bsb-v6.json", "webc": "bibletext-webc-v5.json"}

src = open(os.path.join(ROOT, "verse_of_day.go")).read()
body = src.split("var verseOfDayRefs = []dayPassage{", 1)[1].split("\n}\n", 1)[0]
refs = [(b, int(c), int(lo), int(hi)) for b, c, lo, hi in re.findall(r'\{"([^"]+)", (\d+), (\d+), (\d+)\}', body)]
alts_src = src.split("var verseOfDayAlternates = map[dayPassage]dayPassage{", 1)[1].split("\n}\n", 1)[0]
alts = {}
for a, b, c, d, e, f, g, h in re.findall(r'\{"([^"]+)", (\d+), (\d+), (\d+)\}:\s*\{"([^"]+)", (\d+), (\d+), (\d+)\}', alts_src):
    alts[(a, int(b), int(c), int(d))] = (e, int(f), int(g), int(h))

def key(p):
    b, c, lo, hi = p
    return f"{b} {c}:{lo}" if hi in (0, lo) else f"{b} {c}:{lo}-{hi}"

def load(name):
    path = os.path.join(CACHE, name)
    if not os.path.exists(path):
        sys.exit(f"missing cache {path}: open the app and let it download that edition first")
    return json.load(open(path))["data"]

def passage(d, p):
    b, c, lo, hi = p
    hi = hi or lo
    if b not in d["Verses"]:
        return None, "book"
    ch = d["Verses"][b].get(str(c)) or []
    texts = []
    for n in range(lo, hi + 1):
        row = next((x for x in ch if x["Verse"] == n), None)
        if row is None:
            return None, "verse"
        texts.append(row["Text"].strip())
    return "\n".join(texts), None

out = {"editions": {}}
for ed, fname in EDITIONS.items():
    d = load(fname)
    table = {}
    for p in refs:
        text, why = passage(d, p)
        entry = {}
        if text is None and why == "book" and p in alts:
            text, why = passage(d, alts[p])
            entry["via"] = key(alts[p])
        if text is None:
            entry["missing"] = why
        else:
            entry["first"] = text[0]
            entry["last"] = text[-1]
        table[key(p)] = entry
    out["editions"][ed] = table
path = os.path.join(ROOT, "testdata", "verse_of_day_snapshot.json")
json.dump(out, open(path, "w"), ensure_ascii=False, indent=1, sort_keys=True)
missing = [(ed, k) for ed, t in out["editions"].items() for k, e in t.items() if "missing" in e]
print(f"wrote {path}: {len(refs)} entries x {len(EDITIONS)} editions; missing={missing if missing else 'none'}")
