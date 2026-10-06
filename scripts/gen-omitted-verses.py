#!/usr/bin/env python3
"""Build the per-edition table of verses each translation OMITS.

    scripts/gen-omitted-verses.py \
        --bsb  build/biblecache/bsb.json \
        --web  build/biblecache/web.json \
        --webc build/biblecache/webc.json \
        --out  omitted_verses_data.go

    scripts/gen-omitted-verses.py --next ... --out omitted_verses_data_next.go

With --next it writes the next major release's table (docs/NEXT.md), built only
with the next tag, and checks it against versification_data_next.go instead of
versification_data.go. Run both whenever either is run.

WHY A TABLE AND NOT A RUNTIME RULE. An omitted verse leaves a hole in the
numbering — Luke 17:35 is followed by 17:37 — and a reader is given no reason
for it unless the chapter's notes happen to explain it. Marking the hole means
knowing it IS a hole, and the app cannot learn that at runtime by looking at
the numbering:

  * a merged verse is keyed by its first number ("17-18" files under 17), so a
    merged range is indistinguishable from an omission by number alone;
  * a partial or interrupted fetch leaves gaps that are decode faults, not
    translation decisions, and marking those would have the app assert an
    omission the publisher never made;
  * and some gaps are neither — Greek Esther's numbering does not correspond to
    anything, which versification.go already records as incommensurable.

Which books those are is read from the versification table the output is
checked against: WEB Catholic's Greek Esther in the shipping one. The next
release's maps the Greek Esther verse for verse, so its three gaps (4:6, 9:5,
9:30) are holes like any other there: the Greek text keeps the Hebrew book's
verse numbers and has nothing at those three, which versification_data_next.go
records as absent.

So the holes are found once, offline, from the publisher's own complete feed,
and checked before they are written down. This is the same shape as the
red-letter and paragraph tables: the publisher's material, resolved offline,
shipped as data the runtime only reads.

WHAT COUNTS AS A HOLE. A verse number strictly between the lowest and highest
this translation actually prints in a chapter, which the translation does not
print. A gap at the START or END of a chapter is not a hole: that is the chapter
being shorter, and nothing can tell from the numbering whether a verse is
missing or was never there.

CROSS-CHECK. versification_data.go independently records, for each translation,
the verses the REFERENCE has and it does not (`absent`). Every such verse must
appear as a hole here, or one of the two derivations is wrong. It is a partial
check by nature — the reference is the WEB, so a verse the WEB also omits
appears in no `absent` list — which is exactly why this table exists alongside
it rather than being replaced by it.

NO API CALLS. Point it at the cached complete.json bodies (build/biblecache) or
at freshly downloaded ones; the feeds are public and key-less.
"""
import argparse
import json
import re
import sys

# USFM id -> the app's book name, for the 66-book editions. The Catholic
# edition maps through the same names plus the deuterocanon.
USFM = {
    "GEN": "Genesis", "EXO": "Exodus", "LEV": "Leviticus", "NUM": "Numbers",
    "DEU": "Deuteronomy", "JOS": "Joshua", "JDG": "Judges", "RUT": "Ruth",
    "1SA": "1 Samuel", "2SA": "2 Samuel", "1KI": "1 Kings", "2KI": "2 Kings",
    "1CH": "1 Chronicles", "2CH": "2 Chronicles", "EZR": "Ezra", "NEH": "Nehemiah",
    "EST": "Esther", "ESG": "Esther", "JOB": "Job", "PSA": "Psalms",
    "PRO": "Proverbs", "ECC": "Ecclesiastes", "SNG": "Song of Solomon",
    "ISA": "Isaiah", "JER": "Jeremiah", "LAM": "Lamentations", "EZK": "Ezekiel",
    "DAN": "Daniel", "DAG": "Daniel", "HOS": "Hosea", "JOL": "Joel", "AMO": "Amos",
    "OBA": "Obadiah", "JON": "Jonah", "MIC": "Micah", "NAM": "Nahum",
    "HAB": "Habakkuk", "ZEP": "Zephaniah", "HAG": "Haggai", "ZEC": "Zechariah",
    "MAL": "Malachi", "MAT": "Matthew", "MRK": "Mark", "LUK": "Luke",
    "JHN": "John", "ACT": "Acts", "ROM": "Romans", "1CO": "1 Corinthians",
    "2CO": "2 Corinthians", "GAL": "Galatians", "EPH": "Ephesians",
    "PHP": "Philippians", "COL": "Colossians", "1TH": "1 Thessalonians",
    "2TH": "2 Thessalonians", "1TI": "1 Timothy", "2TI": "2 Timothy",
    "TIT": "Titus", "PHM": "Philemon", "HEB": "Hebrews", "JAS": "James",
    "1PE": "1 Peter", "2PE": "2 Peter", "1JN": "1 John", "2JN": "2 John",
    "3JN": "3 John", "JUD": "Jude", "REV": "Revelation",
    "TOB": "Tobit", "JDT": "Judith", "1MA": "1 Maccabees", "2MA": "2 Maccabees",
    "WIS": "Wisdom", "SIR": "Sirach", "BAR": "Baruch",
}


def verse_text(content):
    out = []
    for it in content or []:
        if isinstance(it, str):
            out.append(it)
        elif isinstance(it, dict) and it.get("text"):
            out.append(it["text"])
    return "".join(out).strip()


def holes(path, incommensurable):
    """Every interior hole in one edition, and the notes that explain them."""
    doc = json.load(open(path, encoding="utf-8"))
    found, noted, skipped = [], 0, []
    for b in doc["books"]:
        name = USFM.get(b["id"])
        if name is None:
            print(f"  ! unmapped USFM id {b['id']}", file=sys.stderr)
            continue
        if name in incommensurable:
            skipped.append(name)
            continue
        for wrap in b["chapters"]:
            ch = wrap["chapter"]
            printed, wordless = [], set()
            for node in ch.get("content") or []:
                if not isinstance(node, dict) or node.get("type") != "verse":
                    continue
                n = node.get("number", 0)
                if verse_text(node.get("content")):
                    printed.append(n)
                elif n > 0:
                    # The feed sent the verse node with no words: the decoder
                    # turns this into an orphan note. It is a hole either way.
                    wordless.add(n)
            if not printed:
                continue
            have = set(printed)
            for v in range(min(printed), max(printed) + 1):
                if v in have:
                    continue
                found.append((name, ch["number"], v))
                if v in wordless:
                    noted += 1
    return found, noted, sorted(set(skipped))


def incommensurable_in_versification(table, edition):
    """The books the versification table records as incommensurable for this
    edition, whose gaps are not omissions."""
    src = open(table, encoding="utf-8").read()
    m = re.search(r'"%s": \{.*?incommensurable: map\[string\]string\{(.*?)\}' % edition, src, re.S)
    if not m:
        return set()
    return set(re.findall(r'"([^"]+)":', m.group(1)))


def absent_from_versification(edition, table="versification_data.go"):
    """The holes versification_data.go says this edition has, in its OWN numbering.

    The table records a verse the edition lacks by the REFERENCE's number. That
    is the edition's hole only while nothing moved into the number: the BSB
    lacks the WEB's Matthew 23:13 and prints the WEB's 23:14 as its 23:13, so
    its hole is 23:14, the number the moved verse vacated. Follow the moves.
    """
    src = open(table, encoding="utf-8").read()
    m = re.search(r'"%s": \{\s*absent: \[\]verseRef\{(.*?)\},\s*moved: \[\]verseMove\{(.*?)\},\s*extra' % edition,
                  src, re.S)
    if not m:
        return []
    moved = {(b, int(tc), int(tv)): (int(c), int(v)) for b, c, v, tc, tv in
             re.findall(r'\{"([^"]+)", (\d+), (\d+), (\d+), (\d+)\}', m.group(2))}
    holes = []
    for b, c, v in re.findall(r'\{"([^"]+)", (\d+), (\d+)\}', m.group(1)):
        hole = (int(c), int(v))
        for _ in range(len(moved)):
            came_from = moved.get((b,) + hole)
            if came_from is None or came_from == hole:
                break
            hole = came_from
        holes.append((b,) + hole)
    return holes


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--bsb", required=True)
    ap.add_argument("--web", required=True)
    ap.add_argument("--webc", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--next", action="store_true",
                    help="write the next major release's table (docs/NEXT.md)")
    args = ap.parse_args()
    table = "versification_data_next.go" if args.next else "versification_data.go"

    # A book whose numbering corresponds to nothing has gaps that are not
    # omissions, and they must not be marked as any. The versification table
    # says which: the Greek Esther in the shipping one, none in the next
    # release's.
    incommensurable = {e: incommensurable_in_versification(table, e) for e in ("bsb", "web", "webc")}

    tables, total = {}, 0
    for edition, path in (("bsb", args.bsb), ("web", args.web), ("webc", args.webc)):
        found, noted, skipped = holes(path, incommensurable.get(edition, set()))
        tables[edition] = found
        total += len(found)
        print(f"{edition}: {len(found)} omitted verses ({noted} carry a note)"
              + (f"; skipped incommensurable {', '.join(skipped)}" if skipped else ""))

        # Every verse the reference has and this edition does not must leave a
        # hole here.
        missing = [r for r in absent_from_versification(edition, table) if r not in found]
        if missing:
            sys.exit(f"ERROR: {edition}: {table} records {missing} as absent, "
                     f"but the feed shows no hole there — the two derivations disagree")

    if total == 0:
        sys.exit("ERROR: no omitted verses found in any edition; the feeds or the "
                 "hole rule have changed and this table would silently empty")

    with open(args.out, "w", encoding="utf-8") as f:
        if not args.next:
            f.write("package bibletext\n\n")
            f.write("// Code generated by scripts/gen-omitted-verses.py. DO NOT EDIT BY HAND.\n//\n")
        else:
            f.write("//go:build next\n\npackage bibletext\n\n")
            f.write("// Code generated by scripts/gen-omitted-verses.py --next. DO NOT EDIT BY HAND.\n//\n")
            f.write("// The next major release's table (docs/NEXT.md). Its init installs it in\n")
            f.write("// place of omitted_verses_data.go's before main or any test runs, so every\n")
            f.write("// reader of omittedVerses reads it. Without the next tag this file is not\n")
            f.write("// compiled at all.\n//\n")
        f.write("// The verses each translation OMITS: a number its own chapter skips,\n")
        f.write("// between verses it does print. Resolved offline from the publishers' own\n")
        f.write("// complete feeds, because the runtime cannot tell a hole from a merged\n")
        f.write("// verse or an interrupted fetch by looking at the numbering.\n//\n")
        f.write("// Cross-checked against %s: every verse recorded there\n" % table)
        if not args.next:
            f.write("// as absent from an edition appears here as a hole.\n//\n")
            f.write("// Books whose numbering corresponds to nothing are excluded, so a gap in\n")
            f.write("// Greek Esther is not reported as an omission.\n\n")
            f.write("var omittedVerses = map[string]map[string]map[int][]int{\n")
        else:
            excluded = sorted({b for books in incommensurable.values() for b in books})
            f.write("// as absent from an edition appears here as a hole.\n//\n")
            if excluded:
                f.write("// Books whose numbering corresponds to nothing there are excluded: %s.\n\n"
                        % ", ".join(excluded))
            else:
                f.write("// No book's numbering fails to correspond there, so none is excluded: the\n")
                f.write("// Greek Esther's three gaps are holes like any other.\n\n")
            f.write("func init() { omittedVerses = nextOmittedVerses }\n\n")
            f.write("var nextOmittedVerses = map[string]map[string]map[int][]int{\n")
        for edition in ("bsb", "web", "webc"):
            f.write('\t"%s": {\n' % edition)
            by_book = {}
            for book, ch, v in tables[edition]:
                by_book.setdefault(book, {}).setdefault(ch, []).append(v)
            for book in sorted(by_book):
                f.write('\t\t"%s": {' % book)
                parts = []
                for ch in sorted(by_book[book]):
                    parts.append("%d: {%s}" % (ch, ", ".join(str(v) for v in sorted(by_book[book][ch]))))
                f.write(", ".join(parts))
                f.write("},\n")
            f.write("\t},\n")
        f.write("}\n")
    print(f"wrote {args.out}: {total} omitted verses across three editions")


if __name__ == "__main__":
    main()
