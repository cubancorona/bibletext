#!/usr/bin/env python3
"""Extract per-chapter, per-verse text from a helloao complete.json (BSB/WEB),
matching the app's decode (bsb.go: verse nodes only; footnotes / line breaks /
headings dropped). Output: transcript.json = {book: {chapter: [{"v":n,"text":..}]}}.

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


def main():
    src, out = sys.argv[1], sys.argv[2]
    doc = json.load(open(src))
    result = {}
    n_ch = n_v = 0
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
                if isinstance(node, dict) and node.get("type") == "verse":
                    text = flatten_verse(node.get("content", []))
                    if text:
                        verses.append({"v": node.get("number"), "text": text})
            if verses:
                chapters[str(ch)] = verses
                n_ch += 1
                n_v += len(verses)
        if chapters:
            result[book] = chapters
    json.dump(result, open(out, "w"), ensure_ascii=False)
    print(f"{len(result)} books, {n_ch} chapters, {n_v} verses -> {out}")


if __name__ == "__main__":
    main()
