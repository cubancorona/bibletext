#!/usr/bin/env python3
"""Regenerate the embedded offline seed from a decoded WEB cache.

    scripts/gen-seed-gospels.py --cache ~/Library/Caches/bibletext/bibletext-web-v6.json \
                                --out assets/seed/web-gospels.json

WHY IT MUST BE REGENERATED. The seed is the four Gospels shown when a first
launch can neither read a cache nor reach the network — the first thing a new
reader ever sees. It is a snapshot of the decoder's OUTPUT, so every decoder
change leaves it a little further behind: the version replaced here had no
poem line breaks, no footnotes and no Psalm titles, so its poetry read as
prose. Regenerate it whenever the decoder's output changes.

Take the cache from a real fetch rather than the raw feed, so the seed is
exactly what the app itself would have decoded.
"""

import argparse
import json
import sys

GOSPELS = ["Matthew", "Mark", "Luke", "John"]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cache", required=True, help="a decoded WEB cache (the app's own file)")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    raw = json.load(open(args.cache, encoding="utf-8"))
    data = raw.get("data", raw)
    if "Verses" not in data:
        sys.exit("%s does not look like a decoded BibleData cache" % args.cache)

    verses = {}
    total = breaks = notes = starts = 0
    for book in GOSPELS:
        chapters = data["Verses"].get(book)
        if not chapters:
            sys.exit("the cache has no %s" % book)
        verses[book] = chapters
        for chapter in chapters.values():
            for v in chapter:
                total += 1
                if "\n" in (v.get("Text") or v.get("text") or ""):
                    breaks += 1
                if v.get("footnotes"):
                    notes += 1
                if v.get("para_start"):
                    starts += 1

    out = {"Verses": verses, "Books": GOSPELS}
    # Superscriptions and orphan footnotes belong to books the seed does not
    # carry (the Psalms, and the omitted verses of Luke and Acts — Luke 17:36
    # is in range, so keep any that fall inside the Gospels).
    orphans = {b: m for b, m in (data.get("orphan_footnotes") or {}).items() if b in GOSPELS}
    if orphans:
        out["orphan_footnotes"] = orphans

    with open(args.out, "w", encoding="utf-8") as fh:
        json.dump(out, fh, ensure_ascii=False, separators=(",", ":"))
    print("%s: %d verses, %d with poem lines, %d with notes, %d opening a paragraph, %d orphan books"
          % (args.out, total, breaks, notes, starts, len(orphans)))


if __name__ == "__main__":
    main()
