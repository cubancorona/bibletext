#!/usr/bin/env python3
"""Regenerate versification_data.go — how the translations' verse numbers relate.

    python3 scripts/gen-versification.py \
        --web  ~/Library/Caches/bibletext/bibletext-web-v2.json \
        --bsb  ~/Library/Caches/bibletext/bibletext-bsb-v3.json \
        --webc ~/Library/Caches/bibletext/bibletext-webc-v2.json \
        --nkjv /path/to/bibletext-nkjv.json      # optional; licensed, never committed

The inputs are the app's OWN cache files, so the table always describes the text
the app actually ships rather than a published versification standard that may
differ from it in detail. Run this whenever a translation's cache epoch is bumped
(see cacheEpoch) or a translation is added; the test in versification_map_test.go
pins the cases a reader can actually hit, and will fail if a regeneration quietly
changes them.

WEB is the reference: every other translation is stored as a delta against it,
so adding a translation costs one delta rather than one per existing pair.

HOW THE THREE RELATIONS ARE DECIDED — each rule exists because the obvious
version of it was wrong when measured:

  moved   A verse the reference has and the target does not, whose text turns up
          at a verse number the REFERENCE does not have. That last clause is the
          whole discriminator. Matching purely on text similarity claimed Mark
          7:16 had "moved" to Mark 4:23 — both are "if anyone has ears to hear,
          let him hear", a formula that recurs; 4:23 exists in both translations
          and has not moved anywhere. A genuine relocation lands in a NEW slot,
          which is exactly what Romans 16:25-27 is in the BSB and the NKJV.

  absent  Everything else the reference has and the target lacks: the eleven
          textual-critical omissions, plus Romans 16:24.

  incommensurable
          A whole book whose verse numbers do not correspond at all. Only
          decidable when the two are the SAME translation (WEB vs WEB Catholic),
          where differing text at the same number means something real. WEBC's
          Esther is Greek Esther — a different book, not a renumbering: its 1:1
          is Mordecai's dream, the WEB's is Ahasuerus. Daniel by contrast differs
          only in wording ("some of" vs "part of"), so it stays mappable, with
          its tail genuinely moved by the Song of the Three.
"""

import argparse
import json
import re
import sys

REFERENCE = "web"
# Translations that ARE the reference text, so a difference at the same verse
# number is evidence about numbering rather than about translators disagreeing.
SAME_TEXT_AS_REFERENCE = {"webc"}

MOVE_MIN_SIMILARITY = 0.30          # cross-translation wording varies a lot
RETARGET_MIN_SIMILARITY = 0.90      # same translation: near-identical or nothing
DIFFERENT_TEXT_FRACTION = 0.50      # over half the shared verses disagreeing

STOPWORDS = set(
    "the and of to a in that he it his him for is was with as they i you not be but".split()
)


def tokens(text):
    return {w for w in re.findall(r"[a-z']+", text.lower()) if w not in STOPWORDS and len(w) > 2}


def similarity(a, b):
    A, B = tokens(a), tokens(b)
    if not A and not B:
        return 1.0
    return len(A & B) / max(1, len(A | B))


def load(path):
    with open(path) as f:
        blob = json.load(f)
    return (blob.get("data") or blob)["Verses"]


def verses_of(bible, book):
    out = {}
    for chapter, verses in bible.get(book, {}).items():
        for v in verses:
            out[(int(chapter), v["Verse"])] = v["Text"]
    return out


def delta(reference, target, target_id):
    absent, moved, extra, incommensurable = [], [], [], []
    for book in sorted(set(reference) | set(target)):
        ref, tgt = verses_of(reference, book), verses_of(target, book)
        if not ref:
            continue                      # a book only the target has (deuterocanon)
        if not tgt:
            incommensurable.append((book, "book absent from this translation"))
            continue

        only_ref = set(ref) - set(tgt)
        only_tgt = set(tgt) - set(ref)
        shared = set(ref) & set(tgt)

        if target_id in SAME_TEXT_AS_REFERENCE and len(shared) > 5:
            disagreeing = [k for k in shared if similarity(ref[k], tgt[k]) < 0.5]
            if len(disagreeing) > len(shared) * DIFFERENT_TEXT_FRACTION:
                incommensurable.append(
                    (book, "different underlying text; verse numbers do not correspond")
                )
                continue

        for key in sorted(only_ref):
            best, score = None, 0.0
            for candidate in only_tgt:
                s = similarity(ref[key], tgt[candidate])
                if s > score:
                    best, score = candidate, s
            if best and score >= MOVE_MIN_SIMILARITY:
                moved.append((book, key[0], key[1], best[0], best[1]))
                only_tgt.discard(best)
            else:
                absent.append((book, key[0], key[1]))

        # Same number, different text, same translation: the real correspondent
        # may be one of the new slots further down the chapter.
        if target_id in SAME_TEXT_AS_REFERENCE:
            for key in sorted(shared):
                if similarity(ref[key], tgt[key]) >= 0.5:
                    continue
                best, score = None, 0.0
                for candidate in only_tgt:
                    s = similarity(ref[key], tgt[candidate])
                    if s > score:
                        best, score = candidate, s
                if best and score >= RETARGET_MIN_SIMILARITY:
                    moved.append((book, key[0], key[1], best[0], best[1]))
                    only_tgt.discard(best)
                    # The number the reference verse VACATED is now occupied by
                    # something with no counterpart in the reference — WEBC's
                    # Daniel 3:24 is the Song of the Three, where the WEB's 3:24
                    # is Nebuchadnezzar's astonishment (now at 3:91). Without
                    # recording that, mapping WEBC 3:24 back would silently
                    # answer "3:24, exact" and a round trip would not close.
                    extra.append((book, key[0], key[1]))

        extra.extend((book, k[0], k[1]) for k in sorted(only_tgt))
    return absent, sorted(moved), extra, incommensurable


def go_source(deltas):
    out = [
        "package bibletext",
        "",
        "// Code generated by scripts/gen-versification.py. DO NOT EDIT BY HAND.",
        "//",
        "// How each translation's verse numbers relate to the WEB's. Derived from the",
        "// app's own cache files, so it describes the text actually shipped. Regenerate",
        "// when a translation's cache epoch changes or a translation is added.",
        "",
        "var versificationDeltas = map[string]versificationDelta{",
    ]
    for vid in sorted(deltas):
        absent, moved, extra, incommensurable = deltas[vid]
        out.append(f'\t{vid!r}: {{'.replace("'", '"'))
        out.append("\t\tabsent: []verseRef{")
        for b, c, v in absent:
            out.append(f'\t\t\t{{{b!r}, {c}, {v}}},'.replace("'", '"'))
        out.append("\t\t},")
        out.append("\t\tmoved: []verseMove{")
        for b, c, v, tc, tv in moved:
            out.append(f'\t\t\t{{{b!r}, {c}, {v}, {tc}, {tv}}},'.replace("'", '"'))
        out.append("\t\t},")
        out.append("\t\textra: []verseRef{")
        for b, c, v in extra:
            out.append(f'\t\t\t{{{b!r}, {c}, {v}}},'.replace("'", '"'))
        out.append("\t\t},")
        out.append("\t\tincommensurable: map[string]string{")
        for book, why in incommensurable:
            out.append(f'\t\t\t{book!r}: {why!r},'.replace("'", '"'))
        out.append("\t\t},")
        out.append("\t},")
    out.append("}")
    out.append("")
    return "\n".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--web", required=True)
    ap.add_argument("--bsb")
    ap.add_argument("--webc")
    ap.add_argument("--nkjv")
    ap.add_argument("--out", default="versification_data.go")
    args = ap.parse_args()

    reference = load(args.web)
    deltas = {}
    for vid in ("bsb", "webc", "nkjv"):
        path = getattr(args, vid)
        if not path:
            print(f"note: no --{vid}, leaving it out of the table", file=sys.stderr)
            continue
        deltas[vid] = delta(reference, load(path), vid)
        absent, moved, extra, incomm = deltas[vid]
        print(
            f"{vid}: {len(absent)} absent, {len(moved)} moved, {len(extra)} extra, "
            f"{len(incomm)} incommensurable",
            file=sys.stderr,
        )

    with open(args.out, "w") as f:
        f.write(go_source(deltas))
    print(f"wrote {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()
