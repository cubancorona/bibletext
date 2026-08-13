#!/usr/bin/env python3
"""Check our editions' numbering against the UBS/Paratext versification schemes.

    python3 scripts/check-versification-refs.py \
        --cache ~/Library/Caches/bibletext --scheme eng

This is the external half of validating versification_data.go. The generator
(scripts/gen-versification.py) derives how our editions differ FROM EACH OTHER;
this says how they differ from a published standard, which is a different and
also useful question — a reference pasted in from outside the app is numbered
according to a tradition, not according to us.

The schemes are the .vrs files shipped by SIL under the MIT licence:

    https://raw.githubusercontent.com/sillsdev/libpalaso/master/SIL.Scripture/Resources/eng.vrs.txt
                                                                …/org.vrs.txt (Hebrew/Greek original)
                                                                …/vul.vrs.txt (Vulgate)
                                                                …/lxx.vrs.txt (Septuagint, with mappings)

Mind the `.txt` suffix; without it those URLs 404. The files are downloaded to a
cache directory on first use and not committed — they are somebody else's data
and they may be updated.

WHAT A DIFFERENCE MEANS. Not necessarily an error. As of 2026-08-13 every edition
we ship differs from `eng` at 3 John 1 (we have 14 verses, the scheme 15) and
Revelation 12 (17 versus 18), because editions genuinely disagree about where
those verse boundaries fall. Since ALL our editions agree with each other there,
it produces no entry in our table — correctly. The WEB additionally differs at
Romans 14 and 16, which is the doxology, and which our table does map. A NEW
difference appearing here is the signal worth chasing: it means either a
translation's text changed underneath us or our reading of it is wrong.

See docs/TEXTUAL-DATA.md for the whole picture.
"""

import argparse
import json
import os
import sys
import urllib.request

SCHEME_URL = (
    "https://raw.githubusercontent.com/sillsdev/libpalaso/master/"
    "SIL.Scripture/Resources/{scheme}.vrs.txt"
)

# USFM book code -> the name our data uses.
BOOKS = {
    "GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers",
    "DEU": "Deuteronomy", "JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth",
    "1SA": "1 Samuel", "2SA": "2 Samuel", "1KI": "1 Kings", "2KI": "2 Kings",
    "1CH": "1 Chronicles", "2CH": "2 Chronicles", "EZR": "Ezra", "NEH": "Nehemiah",
    "EST": "Esther", "JOB": "Job", "PSA": "Psalms", "PRO": "Proverbs",
    "ECC": "Ecclesiastes", "SNG": "Song of Solomon", "ISA": "Isaiah",
    "JER": "Jeremiah", "LAM": "Lamentations", "EZK": "Ezekiel", "DAN": "Daniel",
    "HOS": "Hosea", "JOL": "Joel", "AMO": "Amos", "OBA": "Obadiah", "JON": "Jonah",
    "MIC": "Micah", "NAM": "Nahum", "HAB": "Habakkuk", "ZEP": "Zephaniah",
    "HAG": "Haggai", "ZEC": "Zechariah", "MAL": "Malachi", "MAT": "Matthew",
    "MRK": "Mark", "LUK": "Luke", "JHN": "John", "ACT": "Acts", "ROM": "Romans",
    "1CO": "1 Corinthians", "2CO": "2 Corinthians", "GAL": "Galatians",
    "EPH": "Ephesians", "PHP": "Philippians", "COL": "Colossians",
    "1TH": "1 Thessalonians", "2TH": "2 Thessalonians", "1TI": "1 Timothy",
    "2TI": "2 Timothy", "TIT": "Titus", "PHM": "Philemon", "HEB": "Hebrews",
    "JAS": "James", "1PE": "1 Peter", "2PE": "2 Peter", "1JN": "1 John",
    "2JN": "2 John", "3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
}

# Differences we have already looked at and understood, so a run stays quiet
# until something NEW happens. Removing an entry here should make it reappear.
KNOWN = {
    ("3 John", 1): "editions differ on whether the closing greeting is one verse or two",
    ("Revelation", 12): "editions differ on whether the last sentence closes ch.12 or opens ch.13",
    ("Romans", 14): "the doxology: the WEB has 14:24-26 where the standard ends the chapter at 23",
    ("Romans", 16): "the doxology: the standard has 16:25-27 where the WEB ends at 24",
    # The Catholic edition carries Greek Daniel and Greek Esther (USFM codes DAG
    # and ESG), so 'eng' is simply the wrong yardstick for it — those books are
    # longer by their additions. Compare it against 'vul' or a Catholic scheme
    # if you want a meaningful number; lxx.vrs even states the Daniel mapping
    # outright ("DAG 3:91-97 = DAN 3:24-30"), which is the one our table derived.
    ("Daniel", 3): "WEBC has Greek Daniel: the Song of the Three occupies 3:24-90",
    ("Esther", 4): "WEBC has Greek Esther: Addition B falls inside chapter 4",
    ("Esther", 10): "WEBC has Greek Esther: Addition F extends chapter 10",
}


def fetch_scheme(scheme, cache_dir):
    path = os.path.join(cache_dir, f"{scheme}.vrs")
    if not os.path.exists(path):
        os.makedirs(cache_dir, exist_ok=True)
        url = SCHEME_URL.format(scheme=scheme)
        print(f"downloading {url}", file=sys.stderr)
        with urllib.request.urlopen(url, timeout=60) as r, open(path, "wb") as f:
            f.write(r.read())
    out = {}
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" in line:
                continue                      # comments and mapping lines
            parts = line.split()
            if len(parts) < 2 or parts[0] not in BOOKS:
                continue
            chapters = {}
            for p in parts[1:]:
                if ":" in p:
                    c, v = p.split(":")
                    chapters[int(c)] = int(v)
            out[BOOKS[parts[0]]] = chapters
    return out


def load_edition(path):
    with open(path) as f:
        blob = json.load(f)
    return (blob.get("data") or blob)["Verses"]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cache", default=os.path.expanduser("~/Library/Caches/bibletext"),
                    help="directory holding the app's bibletext-*.json cache files")
    ap.add_argument("--scheme", default="eng", help="eng | org | vul | lxx")
    ap.add_argument("--refs", default=os.path.expanduser("~/.cache/bibletext-vrs"),
                    help="where to keep the downloaded .vrs files")
    ap.add_argument("--all", action="store_true", help="report differences already understood too")
    args = ap.parse_args()

    scheme = fetch_scheme(args.scheme, args.refs)
    editions = sorted(
        f for f in os.listdir(args.cache)
        if f.startswith("bibletext-") and f.endswith(".json") and "crossref" not in f
    )
    if not editions:
        print(f"no cache files in {args.cache}", file=sys.stderr)
        return 1

    news = 0
    for name in editions:
        bible = load_edition(os.path.join(args.cache, name))
        diffs = []
        for book, chapters in scheme.items():
            for c, last in chapters.items():
                got = bible.get(book, {}).get(str(c))
                if got is None:
                    continue                  # the edition simply lacks the book
                ours = max(v["Verse"] for v in got)
                if ours != last:
                    diffs.append((book, c, ours, last))
        fresh = [d for d in diffs if (d[0], d[1]) not in KNOWN]
        news += len(fresh)
        print(f"{name}: {len(diffs)} chapter(s) differ from '{args.scheme}', {len(fresh)} not yet understood")
        for book, c, ours, last in diffs:
            known = KNOWN.get((book, c))
            if known and not args.all:
                continue
            mark = "known" if known else "NEW  "
            print(f"    [{mark}] {book} {c}: ours ends at {ours}, {args.scheme} says {last}"
                  + (f"  — {known}" if known else ""))
    return 1 if news else 0


if __name__ == "__main__":
    sys.exit(main())
