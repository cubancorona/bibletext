#!/usr/bin/env python3
"""Compact per-chapter alignment output into the app's bundled timing asset.

Reads the per-chapter JSONs a batch_align.py run wrote (verses with full-precision
start/end) and emits one compact file for go:embed:

    {book: {chapter: [[verse, start, end], ...]}}

with times rounded to 0.1s — plenty for verse-level highlighting, and it keeps
the whole Bible around half a megabyte.

Usage:
    python3 make_timings_asset.py --timings timings --out ../../assets/timings/bsb.json --chapters 1189 --titles 116
    python3 make_timings_asset.py --timings timings-web-williams --out ../../assets/timings/web.json --chapters 1189 --titles 116

The two guards exist because the asset is rebuilt WHOLE from the per-chapter
directory: a per-chapter JSON that is missing silently drops its chapter, and a
chapter's presence here is what advertises a recording to the app
(recordingHasChapter); and a batch that stopped short of every titled psalm would
ship a half-titled table with no other sign. Name the counts you expect and the
build refuses anything else.
"""

import argparse
import glob
import json
import os
import sys

from timing_rows import asset_rows

DATA = os.environ.get("BIBLETEXT_AUDIO_DATA", os.path.expanduser("~/Dev/bibletext-audiodata"))


def compact(files, skip_suspect=False):
    """Fold per-chapter alignment JSONs into the asset's {book: {chapter: rows}}.
    Returns (books, n_chapters, n_rows, n_titled_chapters, suspect_labels)."""
    books = {}
    n_ch = n_v = n_t = 0
    suspect = []
    for fp in sorted(files):
        d = json.load(open(fp))
        # batch_align records a per-chapter verdict. It matters more than it looks:
        # the app treats a chapter's PRESENCE in this table as proof the chapter was
        # recorded (recordingHasChapter), so a bad alignment does not merely mistime
        # the highlight, it advertises audio. Always report; drop on request.
        if not d.get("ok", True):
            suspect.append(f"{d['book']} {d['chapter']}")
            if skip_suspect:
                continue
        rows = asset_rows(d["verses"])
        if not rows:
            continue
        books.setdefault(d["book"], {})[str(d["chapter"])] = rows
        n_ch += 1
        n_v += len(rows)
        if rows[0][0] == 0:
            n_t += 1
    return books, n_ch, n_v, n_t, suspect


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--timings", required=True, help="dir under the data dir with <Book>_<ch>.json files")
    ap.add_argument("--out", required=True, help="output path for the compact asset")
    ap.add_argument("--skip-suspect", action="store_true",
                    help="drop chapters batch_align flagged not-ok (non-monotonic, verses "
                         "covering <85%% of the audio, or a title crammed into a sliver) "
                         "instead of shipping them")
    ap.add_argument("--chapters", type=int, help="refuse to build unless exactly this many chapters are present")
    ap.add_argument("--titles", type=int, help="refuse to build unless exactly this many chapters carry a verse-0 row")
    a = ap.parse_args()

    src = os.path.join(DATA, a.timings)
    books, n_ch, n_v, n_t, suspect = compact(glob.glob(os.path.join(src, "*.json")), a.skip_suspect)
    if a.chapters is not None and n_ch != a.chapters:
        sys.exit(f"{src}: {n_ch} chapters, expected {a.chapters}; a per-chapter file is missing or extra")
    if a.titles is not None and n_t != a.titles:
        sys.exit(f"{src}: {n_t} chapters carry a title row, expected {a.titles}; the batch stopped short or the transcript lacks titles")

    os.makedirs(os.path.dirname(a.out), exist_ok=True)
    with open(a.out, "w") as f:
        json.dump(books, f, separators=(",", ":"))
    print(f"{a.out}: {len(books)} books, {n_ch} chapters, {n_v} rows, {n_t} titled, "
          f"{os.path.getsize(a.out) // 1024} KB")
    if suspect:
        verb = "DROPPED" if a.skip_suspect else "INCLUDED (rerun with --skip-suspect to drop)"
        print(f"  {len(suspect)} chapter(s) flagged suspect by the aligner, {verb}: "
              f"{', '.join(suspect[:12])}{' …' if len(suspect) > 12 else ''}")


if __name__ == "__main__":
    main()
