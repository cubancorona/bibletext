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
# The licensed edition is included: its cache is written by the app once the
# owner's key has fetched it (see bibletext-nkjv-key-permission in the owner's
# notes). The snapshot carries only presence and first/last characters — never
# the text.
EDITIONS = ["web", "bsb", "webc", "nkjv"]

def cache_file(edition):
    """The edition's newest cache: bibletext-<id>-v<epoch>.json with the highest
    epoch, else the unversioned bibletext-<id>.json. The app bumps the epoch
    whenever its decoder changes and migrates the file, so a fixed name here
    would go stale the first time it did."""
    best, best_epoch = None, -1
    for name in os.listdir(CACHE) if os.path.isdir(CACHE) else []:
        m = re.fullmatch(rf"bibletext-{re.escape(edition)}(?:-v(\d+))?\.json", name)
        if not m:
            continue
        epoch = int(m.group(1) or 0)
        if epoch > best_epoch:
            best, best_epoch = name, epoch
    return best

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

def load(edition):
    name = cache_file(edition)
    if name is None:
        sys.exit(f"no cache for {edition} in {CACHE}: open the app and let it load that edition first")
    return json.load(open(os.path.join(CACHE, name)))["data"], name

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

out = {"editions": {}, "sources": {}}
sources = {}
for ed in EDITIONS:
    d, sources[ed] = load(ed)
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
out["sources"] = sources
path = os.path.join(ROOT, "testdata", "verse_of_day_snapshot.json")
json.dump(out, open(path, "w"), ensure_ascii=False, indent=1, sort_keys=True)
missing = [(ed, k) for ed, t in out["editions"].items() for k, e in t.items() if "missing" in e]
print(f"wrote {path}: {len(refs)} entries x {len(EDITIONS)} editions; missing={missing if missing else 'none'}")
