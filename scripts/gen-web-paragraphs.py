#!/usr/bin/env python3
"""Build the WEB's and WEB Catholic's paragraph starts from their own USFM.

    scripts/gen-web-paragraphs.py \
        --web-usfm  <dir of eng-web USFM> \
        --webc-usfm <dir of eng-web-c USFM> \
        --out paragraph_web_data.go

WHY A TABLE AND NOT THE FEED. Paragraphing is the translators' work: it says
where a speaker changes or an argument turns, and the app draws paragraphs
wherever the publisher set them (Verse.ParaStart). The BSB's and the NKJV's
feeds carry that structure and their decoders read it straight through. The
WEB's runtime feed does not: bible.helloao.org sends 742 break nodes for the
whole Bible, and none at all in Genesis 1, John 3 or Romans 8, while the
edition's own USFM carries 9,254 paragraph markers and breaks Genesis 1 at
the days of creation. About 92% of the paragraphing is lost in the supply.

So it is recovered here instead, from the publisher's files, in the same
shape as the red-letter tables and for the same reason (see
scripts/gen-web-redletter.py): fetch the USFM once from eBible, generate
offline, ship references only.

    https://ebible.org/Scriptures/eng-web_usfm.zip
    https://ebible.org/Scriptures/eng-web-c_usfm.zip

WHAT COUNTS AS A PARAGRAPH START. The prose paragraph markers, and the verse
that follows one. Poetry lines (\\q…) are LINES INSIDE a paragraph, not
paragraphs: marking them would make every line of a psalm its own paragraph,
which is the same mistake the API.Bible decoder avoids by skipping q blocks.
Headings, titles and references are not paragraphs either — they are not
Scripture and the app does not render them.

NO API CALLS, and no text: the output is a set of verse references.
"""

import argparse
import glob
import os
import re
import sys

# Prose paragraph markers. \q* (poetry lines), \b (stanza break), \s* / \r /
# \d / \ms (headings and titles) are deliberately absent.
PARA = re.compile(r'^\\(p|m|mi|nb|pc|po|pr|ph\d?|pi\d?|pm|pmo|pmc|pmr|li\d?|lim\d?|lf|lh)\b')
CHAP = re.compile(r'^\\c\s+(\d+)')
VERSE = re.compile(r'^\\v\s+(\d+)')
BOOKID = re.compile(r'^\\id\s+([A-Z0-9]{3})')


# The 66-book canon, by USFM id. The eBible zips also carry the deuterocanon
# and the GREEK Daniel and Esther (DAG, ESG), which the app maps to the same
# book names as the Hebrew ones — so a reference from Greek Daniel 3, where
# the Song of the Three occupies verses 24-90, would land on a different verse
# of the Daniel this edition actually has. Each edition therefore takes only
# the ids of its own canon.
CANON_66 = [
    'GEN', 'EXO', 'LEV', 'NUM', 'DEU', 'JOS', 'JDG', 'RUT', '1SA', '2SA',
    '1KI', '2KI', '1CH', '2CH', 'EZR', 'NEH', 'EST', 'JOB', 'PSA', 'PRO',
    'ECC', 'SNG', 'ISA', 'JER', 'LAM', 'EZK', 'DAN', 'HOS', 'JOL', 'AMO',
    'OBA', 'JON', 'MIC', 'NAM', 'HAB', 'ZEP', 'HAG', 'ZEC', 'MAL',
    'MAT', 'MRK', 'LUK', 'JHN', 'ACT', 'ROM', '1CO', '2CO', 'GAL', 'EPH',
    'PHP', 'COL', '1TH', '2TH', '1TI', '2TI', 'TIT', 'PHM', 'HEB', 'JAS',
    '1PE', '2PE', '1JN', '2JN', '3JN', 'JUD', 'REV',
]
# The Catholic canon: the same books with the Greek Daniel and Esther in place
# of the Hebrew, plus the seven deuterocanonical books.
CANON_CATHOLIC = [b for b in CANON_66 if b not in ('EST', 'DAN')] + [
    'ESG', 'DAG', 'TOB', 'JDT', 'WIS', 'SIR', 'BAR', '1MA', '2MA',
]


def starts_for(usfm_dir, canon):
    """usfm id -> {chapter: [verse, …]} for verses that open a paragraph.

    Also returns the set of canon ids whose file was present, so a missing
    BOOK can be told apart from a book that simply has no prose paragraphs —
    Wisdom is 1,105 poetry lines and not one \\p, and is correctly empty.
    """
    out = {}
    present = set()
    files = sorted(glob.glob(os.path.join(usfm_dir, '*.usfm')))
    if not files:
        sys.exit('no .usfm files in ' + usfm_dir)
    for path in files:
        book = None
        chapter = 0
        pending = False
        seen = {}
        with open(path, encoding='utf-8', errors='replace') as fh:
            for line in fh:
                line = line.strip()
                if book is None:
                    m = BOOKID.match(line)
                    if m:
                        book = m.group(1)
                    continue
                m = CHAP.match(line)
                if m:
                    chapter = int(m.group(1))
                    # A chapter always opens a paragraph; the app opens one at
                    # the first verse anyway, so nothing is marked here.
                    pending = False
                    continue
                if PARA.match(line):
                    pending = True
                # A marker line can carry its own \v, so test the verse after.
                m = VERSE.search(line)
                if m and chapter:
                    if pending:
                        seen.setdefault(chapter, []).append(int(m.group(1)))
                        pending = False
        if book and book in canon:
            present.add(book)
            if seen:
                out[book] = {c: sorted(set(v)) for c, v in sorted(seen.items())}
    return out, present


def emit(fh, name, data):
    total = sum(len(v) for ch in data.values() for v in ch.values())
    fh.write('\n// %s holds the verses that OPEN a paragraph, keyed "<usfm id> <chapter>".\n' % name)
    fh.write('// %d references across %d books, from the publisher\'s own USFM.\n' % (total, len(data)))
    fh.write('var %s = map[string][]int{\n' % name)
    for book in sorted(data):
        for chapter in sorted(data[book]):
            verses = data[book][chapter]
            fh.write('\t"%s %d": {%s},\n' % (book, chapter, ', '.join(str(v) for v in verses)))
    fh.write('}\n')
    return total


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--web-usfm', required=True)
    ap.add_argument('--webc-usfm', required=True)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()

    web, webSeen = starts_for(args.web_usfm, set(CANON_66))
    webc, webcSeen = starts_for(args.webc_usfm, set(CANON_CATHOLIC))
    for label, canon, seen, data in (('WEB', CANON_66, webSeen, web),
                                     ('WEB Catholic', CANON_CATHOLIC, webcSeen, webc)):
        missing = sorted(set(canon) - seen)
        if missing:
            sys.exit('%s: no USFM file for %s' % (label, ', '.join(missing)))
        allPoetry = sorted(seen - set(data))
        if allPoetry:
            print('%s: no prose paragraphs in %s (poetry throughout)' % (label, ', '.join(allPoetry)))

    with open(args.out, 'w', encoding='utf-8') as fh:
        fh.write('package bibletext\n\n')
        fh.write('// Code generated by scripts/gen-web-paragraphs.py. DO NOT EDIT BY HAND.\n')
        fh.write('//\n')
        fh.write("// The WEB's and WEB Catholic's own paragraph starts, as verse references.\n")
        fh.write('// Both editions set them in their published USFM; the runtime feed drops\n')
        fh.write('// about 92%% of them, so they are recovered from the publisher instead. No\n')
        fh.write('// text is stored here — only which verse opens a paragraph.\n')
        a = emit(fh, 'webParagraphStarts', web)
        b = emit(fh, 'webcParagraphStarts', webc)
    print('WEB %d references / %d books; WEB Catholic %d / %d -> %s' % (a, len(web), b, len(webc), args.out))


if __name__ == '__main__':
    main()
