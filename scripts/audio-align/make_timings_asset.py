#!/usr/bin/env python3
"""Compact per-chapter alignment output into the app's bundled timing asset.

Reads the per-chapter JSONs a batch_align.py run wrote (verses with full-precision
start/end) and emits one compact file for go:embed:

    {book: {chapter: [[verse, start, end], ...]}}

with times rounded to 0.1s — plenty for verse-level highlighting, and it keeps
the whole Bible around half a megabyte.

Usage:
    python3 make_timings_asset.py --timings timings --out ../../assets/timings/bsb.json
    python3 make_timings_asset.py --timings timings-web-williams --out ../../assets/timings/web.json
"""

import argparse
import glob
import json
import os

DATA = os.environ.get("BIBLETEXT_AUDIO_DATA", os.path.expanduser("~/Dev/bibletext-audiodata"))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--timings", required=True, help="dir under the data dir with <Book>_<ch>.json files")
    ap.add_argument("--out", required=True, help="output path for the compact asset")
    a = ap.parse_args()

    src = os.path.join(DATA, a.timings)
    books = {}
    n_ch = n_v = 0
    for fp in sorted(glob.glob(os.path.join(src, "*.json"))):
        d = json.load(open(fp))
        rows = [[v["v"], round(v["start"], 1), round(v["end"], 1)] for v in d["verses"]]
        if not rows:
            continue
        books.setdefault(d["book"], {})[str(d["chapter"])] = rows
        n_ch += 1
        n_v += len(rows)

    os.makedirs(os.path.dirname(a.out), exist_ok=True)
    with open(a.out, "w") as f:
        json.dump(books, f, separators=(",", ":"))
    print(f"{a.out}: {len(books)} books, {n_ch} chapters, {n_v} verses, "
          f"{os.path.getsize(a.out) // 1024} KB")


if __name__ == "__main__":
    main()
