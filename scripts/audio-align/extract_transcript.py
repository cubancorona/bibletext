#!/usr/bin/env python3
"""Extract per-chapter, per-verse text from a helloao complete.json (BSB/WEB),
matching the app's decode (bsb.go: verse nodes only; footnotes / line breaks /
headings dropped). Output: transcript.json = {book: {chapter: [{"v":n,"text":..}]}}.

A Psalm's TITLE (the hebrew_subtitle node — "A Psalm of David, when he fled from
Absalom his son") is emitted as a verse-0 row ahead of verse 1. The narrators read
the titles: across the Berean tables, titled psalms carry ~2.4s more audio before
verse 1 than untitled ones, and without a row to pin it to that audio was unaligned
"intro" during which nothing highlighted. Verse 0 is the key the app already uses
for a title everywhere else (its footnotes, its search hit). Psalm 119's ALEPH is
not a title but an acrostic letter — a single all-capital word, the only subtitle
of 350 that is — and is skipped exactly as the app's acrosticLetterLabel skips it.

The verse boundaries are what the forced aligner needs to roll word timings up to
verse-level timings. Usage:
    extract_transcript.py <complete.json> <out.json>
"""
import json
import re
import sys

# Canonical 66-book order == helloao `order` (1..66). Labels only; alignment uses text.
APP_BOOKS = [
    "Genesis", "Exodus", "Leviticus", "Numbers", "Deuteronomy", "Joshua", "Judges",
    "Ruth", "1 Samuel", "2 Samuel", "1 Kings", "2 Kings", "1 Chronicles", "2 Chronicles",
    "Ezra", "Nehemiah", "Esther", "Job", "Psalms", "Proverbs", "Ecclesiastes",
    "Song of Solomon", "Isaiah", "Jeremiah", "Lamentations", "Ezekiel", "Daniel", "Hosea",
    "Joel", "Amos", "Obadiah", "Jonah", "Micah", "Nahum", "Habakkuk", "Zephaniah", "Haggai",
    "Zechariah", "Malachi", "Matthew", "Mark", "Luke", "John", "Acts", "Romans",
    "1 Corinthians", "2 Corinthians", "Galatians", "Ephesians", "Philippians", "Colossians",
    "1 Thessalonians", "2 Thessalonians", "1 Timothy", "2 Timothy", "Titus", "Philemon",
    "Hebrews", "James", "1 Peter", "2 Peter", "1 John", "2 John", "3 John", "Jude", "Revelation",
]


def flatten_verse(content):
    """A verse's `content` is a flat array of strings and objects; only strings and
    objects with a "text" field are reader text (footnote {noteId} / {lineBreak} drop)."""
    pieces = []
    for node in content:
        if isinstance(node, str):
            if node:
                pieces.append(node)
        elif isinstance(node, dict) and isinstance(node.get("text"), str):
            if node["text"]:
                pieces.append(node["text"])
    s = " ".join(pieces)
    # tidy: no space before closing punctuation, none after opening (matches bsbTidySpacing intent)
    s = re.sub(r"\s+([,.;:!?”’)\]])", r"\1", s)
    s = re.sub(r"([(\[“‘])\s+", r"\1", s)
    return re.sub(r"\s{2,}", " ", s).strip()


def acrostic_letter_label(text):
    """The app's own rule (bsb.go acrosticLetterLabel): a non-empty run of
    capital letters and nothing else is an acrostic letter — Psalm 119's ALEPH —
    not a title."""
    text = text.strip()
    return bool(text) and all("A" <= c <= "Z" for c in text)


def extract(doc):
    """The transcript for one edition: {book: {chapter: [{"v": n, "text": ...}]}}
    with a titled psalm's title first as v 0. Returns (transcript, n_chapters,
    n_rows, n_titles); the title count is the run's own control — 116 per
    edition, and 117 would mean ALEPH leaked through."""
    result = {}
    n_ch = n_v = n_t = 0
    for b in doc["books"]:
        order = b.get("order", 0)
        if not (1 <= order <= len(APP_BOOKS)):
            continue
        book = APP_BOOKS[order - 1]
        chapters = {}
        for cj in b.get("chapters", []):
            ch = cj["chapter"]["number"]
            verses = []
            for node in cj["chapter"]["content"]:
                if not isinstance(node, dict):
                    continue
                if node.get("type") == "hebrew_subtitle":
                    title = flatten_verse(node.get("content", []))
                    if title and not acrostic_letter_label(title):
                        verses.append({"v": 0, "text": title})
                        n_t += 1
                    continue
                if node.get("type") == "verse":
                    text = flatten_verse(node.get("content", []))
                    if text:
                        verses.append({"v": node.get("number"), "text": text})
            if verses:
                chapters[str(ch)] = verses
                n_ch += 1
                n_v += len(verses)
        if chapters:
            result[book] = chapters
    return result, n_ch, n_v, n_t


def main():
    src, out = sys.argv[1], sys.argv[2]
    result, n_ch, n_v, n_t = extract(json.load(open(src)))
    json.dump(result, open(out, "w"), ensure_ascii=False)
    print(f"{len(result)} books, {n_ch} chapters, {n_v} rows, {n_t} titles -> {out}")


if __name__ == "__main__":
    main()
