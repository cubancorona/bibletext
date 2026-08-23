#!/usr/bin/env python3
"""Build the BSB red-letter table from the publisher's own USFM markup.

    python3 scripts/gen-bsb-redletter.py \
        --usfm <unpacked bsb_usfm.zip>/bsb_usfm \
        --bsb ~/Library/Caches/bibletext/bibletext-bsb-v3.json

The official Berean Bible download contains ``\\wj ... \\wj*`` markers. Those
markers are the authority: this generator extracts their text, locates it in the
app's runtime BSB text, and writes rune offsets plus a content fingerprint.

The publisher USFM and runtime supplier are allowed to differ in whitespace and
typography. Exact punctuation-preserving matching is attempted first. A
case-insensitive alphanumeric match is used only when typography differs; its
boundaries are expanded to include adjacent quotation punctuation. Every
publisher-marked run must locate. A source/runtime revision mismatch therefore
fails generation instead of silently substituting another edition's judgement.

Official source: https://berean.bible/downloads.htm
Direct archive:  https://bereanbible.com/bsb_usfm.zip
"""

import argparse
import bisect
import glob
import json
import os
import re
import subprocess
import sys


BOOK = {
    "MAT": "Matthew",
    "MRK": "Mark",
    "LUK": "Luke",
    "JHN": "John",
    "ACT": "Acts",
    "1CO": "1 Corinthians",
    "2CO": "2 Corinthians",
    "1TI": "1 Timothy",
    "REV": "Revelation",
}


def clean_usfm(text):
    """Remove non-verse payloads while retaining character-marker contents."""
    text = re.sub(r"\\f\s.*?\\f\*", "", text, flags=re.S)
    text = re.sub(r"\\x\s.*?\\x\*", "", text, flags=re.S)
    # The third-printing files use this literal placeholder at a handful of
    # paragraph-spanning quotation ends; it is editorial metadata, not text.
    text = text.replace("[’’]", "")
    return re.sub(r"\\\+?w\s+([^|\\]*?)(?:\|[^\\]*?)?\\\+?w\*", r"\1", text)


def book_code(path):
    name = os.path.basename(path)
    direct = re.fullmatch(r"([A-Z0-9]{3})\.usfm", name)
    if direct:
        return direct.group(1)
    ebible = re.search(r"-([A-Z0-9]{3})eng", name)
    return ebible.group(1) if ebible else None


def wj_texts(usfm_dir):
    """Return {book: {(chapter, verse): [publisher-marked text, ...]}}."""
    out = {}
    for path in glob.glob(os.path.join(usfm_dir, "*.usfm")):
        code = book_code(path)
        if code not in BOOK:
            continue
        text = clean_usfm(open(path, encoding="utf-8").read())
        chapter = verse = 0
        in_wj = False
        current = []
        marked = {}

        def flush():
            if in_wj and current and chapter and verse:
                value = re.sub(r"\s+", " ", "".join(current)).strip()
                if value:
                    marked.setdefault((chapter, verse), []).append(value)

        token_pattern = r"\\wj\*|\\wj\b|\\c\s+\d+|\\v\s+\d+|\\[+a-z0-9-]+\*?|[^\\]+"
        for token in re.findall(token_pattern, text):
            if token.startswith("\\wj*"):
                flush()
                current = []
                in_wj = False
            elif token.startswith("\\wj"):
                flush()
                current = []
                in_wj = True
            elif token.startswith("\\c"):
                flush()
                current = []
                chapter = int(token.split()[1])
                verse = 0
            elif token.startswith("\\v"):
                flush()
                current = []
                verse = int(token.split()[1])
            elif token.startswith("\\"):
                continue
            elif in_wj and chapter and verse:
                current.append(token)
        flush()
        out[BOOK[code]] = marked
    return out


def exact_locate(haystack, needle, start):
    pattern = re.escape(needle).replace(r"\ ", r"\s+")
    match = re.compile(pattern).search(haystack, start)
    return (match.start(), match.end()) if match else None


def alphanumeric_map(text):
    """Return normalized alphanumerics and their original rune positions."""
    normalized = []
    positions = []
    for index, char in enumerate(text):
        if not char.isalnum():
            continue
        for folded in char.casefold():
            normalized.append(folded)
            positions.append(index)
    return "".join(normalized), positions


def normalized_locate(haystack, needle, start):
    normalized_haystack, positions = alphanumeric_map(haystack)
    normalized_needle, _ = alphanumeric_map(needle)
    if not normalized_needle:
        return None
    normalized_start = bisect.bisect_left(positions, start)
    found = normalized_haystack.find(normalized_needle, normalized_start)
    if found < 0:
        return None

    begin = positions[found]
    end = positions[found + len(normalized_needle) - 1] + 1

    # Preserve the publisher's boundary punctuation without swallowing adjacent
    # punctuation outside the marker. John 6:42, for example, ends the marked
    # inner quotation before the surrounding speaker's closing quote.
    first_alnum = next((index for index, char in enumerate(needle) if char.isalnum()), 0)
    last_alnum = next((index for index in range(len(needle) - 1, -1, -1) if needle[index].isalnum()), len(needle) - 1)
    leading = needle[:first_alnum]
    trailing = needle[last_alnum + 1:]
    if leading:
        boundary = begin
        while boundary > start and not haystack[boundary - 1].isalnum() and not haystack[boundary - 1].isspace():
            boundary -= 1
        wanted = leading[0]
        found_boundary = haystack.find(wanted, boundary, begin)
        if found_boundary >= 0:
            begin = found_boundary
    if trailing:
        boundary = end
        while boundary < len(haystack) and not haystack[boundary].isalnum() and not haystack[boundary].isspace():
            boundary += 1
        wanted = trailing[-1]
        found_boundary = haystack.rfind(wanted, end, boundary)
        if found_boundary >= 0:
            end = found_boundary + 1
    return begin, end


def locate(haystack, needle, start):
    exact = exact_locate(haystack, needle, start)
    if exact:
        return exact, "exact"
    normalized = normalized_locate(haystack, needle, start)
    return (normalized, "normalized") if normalized else (None, "missing")


def fnv1a64(text):
    value = 0xCBF29CE484222325
    for byte in text.encode("utf-8"):
        value ^= byte
        value = (value * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return value


def load_runtime_bsb(cache_path):
    blob = json.load(open(cache_path, encoding="utf-8"))
    return (blob.get("data") or blob)["Verses"]


def build(usfm_dir, cache_path):
    runtime = load_runtime_bsb(cache_path)
    spans = {}
    lengths = {}
    hashes = {}
    failures = []
    normalized_matches = []
    exact = normalized = 0

    for book, marked in wj_texts(usfm_dir).items():
        for (chapter, verse), source_runs in sorted(marked.items()):
            runtime_chapter = runtime.get(book, {}).get(str(chapter), [])
            runtime_text = next((item["Text"] for item in runtime_chapter if item["Verse"] == verse), None)
            key = f"{book} {chapter}:{verse}"
            if runtime_text is None:
                failures.append((key, "verse absent from runtime text"))
                continue

            found_spans = []
            cursor = 0
            failed = False
            for source_text in source_runs:
                found, mode = locate(runtime_text, source_text, cursor)
                if found is None:
                    failures.append((key, repr(source_text[:100])))
                    failed = True
                    break
                start, end = found
                if found_spans and start <= found_spans[-1][1]:
                    found_spans[-1] = (found_spans[-1][0], max(found_spans[-1][1], end))
                else:
                    found_spans.append((start, end))
                cursor = end
                exact += mode == "exact"
                normalized += mode == "normalized"
                if mode == "normalized":
                    normalized_matches.append((key, source_text, runtime_text[start:end]))

            if failed:
                continue
            spans[key] = found_spans
            lengths[key] = len(runtime_text)
            hashes[key] = fnv1a64(runtime_text)

    if failures:
        print(f"FAILED to locate {len(failures)} publisher-marked verses/runs:", file=sys.stderr)
        for key, why in failures[:40]:
            print(f"  {key}: {why}", file=sys.stderr)
        raise SystemExit(1)
    return spans, lengths, hashes, exact, normalized, normalized_matches


def emit_go(spans, lengths, hashes):
    out = [
        "package bibletext",
        "",
        "// Code generated by scripts/gen-bsb-redletter.py. DO NOT EDIT BY HAND.",
        "//",
        "// The BSB publisher's own words-of-Jesus markers, as rune offsets into",
        "// the app's runtime verse text. Source: the official third-printing USFM",
        "// distributed at https://berean.bible/downloads.htm.",
        "//",
        "// Length and FNV-1a fingerprint maps bind every offset set to the exact",
        "// runtime verse revision from which it was generated.",
        "",
        "var bsbRedLetterSpans = map[string][]redLetterSpan{",
    ]
    for key in sorted(spans):
        pairs = ", ".join(f"{{{start}, {end}}}" for start, end in spans[key])
        out.append(f'\t"{key}": {{{pairs}}},')
    out.extend(["}", "", "var bsbRedLetterRunes = map[string]int{"])
    for key in sorted(lengths):
        out.append(f'\t"{key}": {lengths[key]},')
    out.extend(["}", "", "var bsbRedLetterHashes = map[string]uint64{"])
    for key in sorted(hashes):
        out.append(f'\t"{key}": 0x{hashes[key]:016x},')
    out.extend(["}", ""])
    return "\n".join(out)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--usfm", required=True, help="directory from the official bsb_usfm.zip")
    parser.add_argument("--bsb", required=True, help="the app's BSB cache JSON")
    parser.add_argument("--out", default="red_letter_bsb_data.go")
    parser.add_argument("--verbose", action="store_true", help="show normalized source/runtime matches")
    args = parser.parse_args()

    spans, lengths, hashes, exact, normalized, normalized_matches = build(args.usfm, args.bsb)
    with open(args.out, "w", encoding="utf-8") as output:
        output.write(emit_go(spans, lengths, hashes))
    subprocess.run(["gofmt", "-w", args.out], check=True)
    run_count = sum(len(value) for value in spans.values())
    print(
        f"wrote {args.out}: {len(spans)} verses, {run_count} runs "
        f"({exact} exact source runs, {normalized} typography-normalized)",
        file=sys.stderr,
    )
    if args.verbose:
        for key, source, runtime in normalized_matches:
            print(f"  {key}\n    source:  {source!r}\n    runtime: {runtime!r}", file=sys.stderr)


if __name__ == "__main__":
    main()
