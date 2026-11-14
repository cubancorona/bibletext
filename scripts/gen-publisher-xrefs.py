#!/usr/bin/env python3
"""Build the WEB's, WEB Catholic's and BSB's cross references from their USFM.

    python3 scripts/gen-publisher-xrefs.py \
        --web-usfm   <dir of eng-web USFM>     --web-cache   <path to decoded WEB cache> \
        --webc-usfm  <dir of eng-web-c USFM>   --webc-cache  <path to decoded WEBC cache> \
        --bsb-usfm   <dir of bsb USFM>         --bsb-cache   <path to decoded BSB cache> \
        --out publisher_xrefs_public_data.go

WHY A TABLE AND NOT THE FEED. All three public-domain / CC0 editions carry
publisher cross-references in their published USFM:

  - World English Bible (WEB): 340 in-verse \\x + ... \\x* cross-reference notes
    across 322 verses (Old Testament quotations, allusions and parallels);
  - World English Bible Catholic (WEBC): 343 in-verse \\x + ... \\x* notes across
    325 verses (the same set plus Tobit 2:6, Wisdom 14:27 and Sirach 6:30);
  - Berean Standard Bible (BSB): 1,325 section-heading \\r parallel-passage
    reference lines beneath \\s1/\\s2 headings (Synoptic parallels, Samuel/Kings
    and Chronicles parallels, Psalm historical superscription links, etc.).

bible.helloao.org strips every \\x note from WEB and WEBC and every \\r line
from BSB (save a single leaked "(Joshua–Malachi)" heading node at Hebrews
11:30). As with WEB paragraphing (scripts/gen-web-paragraphs.py) and the
red-letter span tables (scripts/gen-web-redletter.py, scripts/gen-bsb-redletter.py),
this script recovers those publisher notes offline from the editions' own
public-domain / CC0 USFM files and verifies every anchor offset and every
citation target against the decoded runtime caches.

NO NETWORK CALLS. Every citation target is checked against the target edition's
own decoded cache so nothing points at a missing chapter or verse.
"""

import argparse
import glob
import json
import os
import re
import subprocess
import sys

CANON_66 = [
    'GEN', 'EXO', 'LEV', 'NUM', 'DEU', 'JOS', 'JDG', 'RUT', '1SA', '2SA',
    '1KI', '2KI', '1CH', '2CH', 'EZR', 'NEH', 'EST', 'JOB', 'PSA', 'PRO',
    'ECC', 'SNG', 'ISA', 'JER', 'LAM', 'EZK', 'DAN', 'HOS', 'JOL', 'AMO',
    'OBA', 'JON', 'MIC', 'NAM', 'HAB', 'ZEP', 'HAG', 'ZEC', 'MAL',
    'MAT', 'MRK', 'LUK', 'JHN', 'ACT', 'ROM', '1CO', '2CO', 'GAL', 'EPH',
    'PHP', 'COL', '1TH', '2TH', '1TI', '2TI', 'TIT', 'PHM', 'HEB', 'JAS',
    '1PE', '2PE', '1JN', '2JN', '3JN', 'JUD', 'REV',
]

CANON_CATHOLIC = [b for b in CANON_66 if b not in ('EST', 'DAN')] + [
    'ESG', 'DAG', 'TOB', 'JDT', 'WIS', 'SIR', 'BAR', '1MA', '2MA',
]

USFM_TO_BOOK = {
    'GEN': 'Genesis', 'EXO': 'Exodus', 'LEV': 'Leviticus', 'NUM': 'Numbers',
    'DEU': 'Deuteronomy', 'JOS': 'Joshua', 'JDG': 'Judges', 'RUT': 'Ruth',
    '1SA': '1 Samuel', '2SA': '2 Samuel', '1KI': '1 Kings', '2KI': '2 Kings',
    '1CH': '1 Chronicles', '2CH': '2 Chronicles', 'EZR': 'Ezra',
    'NEH': 'Nehemiah', 'EST': 'Esther', 'ESG': 'Esther', 'JOB': 'Job',
    'PSA': 'Psalms', 'PRO': 'Proverbs', 'ECC': 'Ecclesiastes',
    'SNG': 'Song of Solomon', 'ISA': 'Isaiah', 'JER': 'Jeremiah',
    'LAM': 'Lamentations', 'EZK': 'Ezekiel', 'DAN': 'Daniel', 'DAG': 'Daniel',
    'HOS': 'Hosea', 'JOL': 'Joel', 'AMO': 'Amos', 'OBA': 'Obadiah',
    'JON': 'Jonah', 'MIC': 'Micah', 'NAM': 'Nahum', 'HAB': 'Habakkuk',
    'ZEP': 'Zephaniah', 'HAG': 'Haggai', 'ZEC': 'Zechariah', 'MAL': 'Malachi',
    'TOB': 'Tobit', 'JDT': 'Judith', 'WIS': 'Wisdom', 'SIR': 'Sirach',
    'BAR': 'Baruch', '1MA': '1 Maccabees', '2MA': '2 Maccabees',
    'MAT': 'Matthew', 'MRK': 'Mark', 'LUK': 'Luke', 'JHN': 'John',
    'ACT': 'Acts', 'ROM': 'Romans', '1CO': '1 Corinthians',
    '2CO': '2 Corinthians', 'GAL': 'Galatians', 'EPH': 'Ephesians',
    'PHP': 'Philippians', 'COL': 'Colossians', '1TH': '1 Thessalonians',
    '2TH': '2 Thessalonians', '1TI': '1 Timothy', '2TI': '2 Timothy',
    'TIT': 'Titus', 'PHM': 'Philemon', 'HEB': 'Hebrews', 'JAS': 'James',
    '1PE': '1 Peter', '2PE': '2 Peter', '1JN': '1 John', '2JN': '2 John',
    '3JN': '3 John', 'JUD': 'Jude', 'REV': 'Revelation',
}

# Standard 3-letter USFM code emitted in NoteRef.ID (accepted by apiBibleBookName).
BOOK_TO_ID_CODE = {
    name: code for code, name in USFM_TO_BOOK.items()
    if code not in ('ESG', 'DAG')
}

# Book names as printed in WEB, WEBC and BSB cross-references.
PRINTED_BOOK_TO_CODE = {name: BOOK_TO_ID_CODE[name] for name in BOOK_TO_ID_CODE}
PRINTED_BOOK_TO_CODE['Psalm'] = 'PSA'
PRINTED_BOOK_TO_CODE['Song of Songs'] = 'SNG'

ONE_CHAPTER_BOOKS = {'OBA', 'PHM', '2JN', '3JN', 'JUD'}

# Known wording difference between eBible's eng-web USFM ("Yahweh's house") and
# helloao's ENGWEBP ("the LORD's house") in 2 Kings 12:4, and between BSB's
# USFM heading ("Psalm") and helloao's BSB heading ("Psalms") at 1 Chr 16:7.
VERSE_WORDING_RECOVERIES = {
    ('web', '2 Kings 12:4'): ('Yahweh\u2019s', 'the LORD\u2019s'),
}

BSB_HEADING_RECOVERIES = {
    ('1 Chronicles 16:7', 'David\u2019s Psalm of Thanksgiving'): 'David\u2019s Psalms of Thanksgiving',
}

BOOKID_RE = re.compile(r'^\\id\s+([A-Z0-9]{3})\b')
NON_VERSE_PARAGRAPH = re.compile(
    r's\d*|ms\d*|mr|r|sr|sp|qa|cl|cd|d|mt\d*|mte\d*|h\d*|toc\d*|toca\d*|id|ide|usfm|rem|sts'
)
VERSE_PARAGRAPH = re.compile(
    r'p|m|mi|nb|pc|pr|pm|pmo|pmc|pmr|pi\d*|ph\d*|li\d*|lim\d*|lf|lh|q\d*|qr|qc|qm\d*|qd|b'
)
CHAPTER_TOKEN = re.compile(r'\\c\s+(\d+)')
VERSE_TOKEN = re.compile(r'\\v\s+(\d+)')
BASIS_TOKENS = ('LXX', 'DSS')


def fnv1a64(text):
    """Exact match for redLetterTextHash in red_letter_bsb.go (UTF-8 bytes)."""
    h = 14695981039346656037
    prime = 1099511628211
    for b in text.encode('utf-8'):
        h ^= b
        h = (h * prime) & 0xFFFFFFFFFFFFFFFF
    return h


def alnum_only(text):
    return ''.join(ch for ch in text if ch.isalnum())


def load_cache(path):
    raw = json.load(open(path, encoding='utf-8'))
    data = raw.get('data', raw)
    verses = {}
    chapters = {}
    for book, ch_map in (data.get('Verses') or {}).items():
        chapters[book] = {}
        for ch_str, v_list in ch_map.items():
            ch = int(ch_str)
            v_nums = []
            for v_obj in v_list:
                v_num = int(v_obj.get('Verse', v_obj.get('verse', 0)))
                txt = v_obj.get('Text', v_obj.get('text', ''))
                verses[(book, ch, v_num)] = txt
                v_nums.append(v_num)
            if v_nums:
                chapters[book][ch] = sorted(v_nums)
    headings = {}
    raw_headings = data.get('headings') or data.get('Headings') or {}
    for book, ch_map in raw_headings.items():
        headings[book] = {}
        for ch_str, h_list in ch_map.items():
            ch = int(ch_str)
            parsed = []
            for h_obj in h_list:
                bv = h_obj.get('before_verse')
                if bv is None:
                    bv = h_obj.get('BeforeVerse', h_obj.get('beforeVerse', 0))
                parsed.append({
                    'Text': h_obj.get('Text', h_obj.get('text', '')),
                    'Style': h_obj.get('Style', h_obj.get('style', '')),
                    'BeforeVerse': int(bv),
                })
            headings[book][ch] = parsed
    return verses, chapters, headings


def read_canon_usfm(usfm_dir, canon, label, cache_books, allow_partial):
    files = sorted(glob.glob(os.path.join(usfm_dir, '*.usfm')))
    if not files:
        sys.exit(f'{label}: no .usfm files in {usfm_dir}')
    canon_set = set(canon)
    by_code = {}
    for path in files:
        text = open(path, encoding='utf-8').read()
        code = None
        for line in text.splitlines():
            m = BOOKID_RE.match(line.strip())
            if m:
                code = m.group(1)
                break
        if code and code in canon_set:
            if code in by_code:
                sys.exit(f'{label}: duplicate USFM book {code} in {usfm_dir}')
            by_code[code] = (path, text)
    if not by_code:
        sys.exit(f'{label}: no USFM files matched canon in {usfm_dir}')
    if allow_partial:
        want_books = {USFM_TO_BOOK[c] for c in by_code}
        if want_books != (cache_books & {USFM_TO_BOOK[c] for c in canon}):
            sys.exit(
                f'{label}: partial-canon book mismatch between USFM {sorted(want_books)} '
                f'and cache {sorted(cache_books)}'
            )
    else:
        missing = sorted(canon_set - set(by_code))
        if missing:
            sys.exit(f'{label}: missing USFM files for {", ".join(missing)}')
    return [(code, USFM_TO_BOOK[code], by_code[code][1]) for code in canon if code in by_code]


def clean_xref_note_body(raw):
    """Extract clean citation text from \\x + \\xo ... \\xt ...\\x*."""
    body = raw[2:-3].strip()  # strip leading \x and trailing \x*
    if body.startswith('+') or body.startswith('-'):
        body = body[1:].strip()
    body = re.sub(r'^\\xo\s+\S+\s*', '', body)
    body = re.sub(r'\\xt\s*', '', body)
    body = re.sub(r'\\\+?[a-z0-9]+\*?', '', body)
    # If USFM 3 link attributes |... ever appear inside \xt, drop the attribute half.
    body = re.sub(r'\|[^;\\]+', '', body)
    body = re.sub(r'\s+', ' ', body).strip()
    return body


def strip_basis_suffix(piece):
    """Strip trailing ' LXX', ', LXX', ', DSS' basis tokens, returning (core, cut_delta)."""
    cur = piece
    while True:
        matched = False
        for tok in BASIS_TOKENS:
            for sep in (', ' + tok, ' ' + tok):
                if cur.endswith(sep):
                    cur = cur[:-len(sep)].rstrip()
                    matched = True
                    break
            if matched:
                break
        if not matched:
            break
    return cur


FIGURES_RE = re.compile(
    r'^(\d+)(?::(\d+))?(?:([-\u2013\u2014])(\d+)(?::(\d+))?)?$'
)


def parse_citation_pieces(text, chapters_in_cache, context_label):
    """Split note text on ';' and ',' identically to readShorthand and resolve NoteRefs."""
    if '(' in text or ')' in text:
        sys.exit(f'{context_label}: unexpected parentheses in {text!r}')

    # Check whole-note cross-book span first (e.g. BSB Hebrews 11:30 "Joshua–Malachi").
    m_books = re.fullmatch(r'([1-3]?\s*[A-Za-z ]+?)[-\u2013\u2014]([1-3]?\s*[A-Za-z ]+)', text)
    if m_books:
        b1, b2 = m_books.group(1).strip(), m_books.group(2).strip()
        if b1 in PRINTED_BOOK_TO_CODE and b2 in PRINTED_BOOK_TO_CODE:
            return []

    refs = []
    carry_book = None
    carry_chapter = None
    comma = False
    n = len(text)
    i = 0
    while i < n:
        ch = text[i]
        if ch == ' ':
            i += 1
            continue
        if ch == ';':
            comma = False
            i += 1
            continue
        if ch == ',':
            comma = True
            i += 1
            continue

        nxt = i
        while nxt < n and text[nxt] not in ';,()':
            nxt += 1
        end = nxt
        while end > i and text[end - 1] == ' ':
            end -= 1
        raw_piece = text[i:end]
        if comma and carry_book and raw_piece in BASIS_TOKENS:
            i = nxt
            continue

        piece_start = i
        if raw_piece.startswith('See '):
            if comma:
                sys.exit(f'{context_label}: unexpected "See " after comma in {text!r}')
            piece_start += 4
            raw_piece = raw_piece[4:]
            if not raw_piece or raw_piece[0] == ' ':
                sys.exit(f'{context_label}: malformed "See " prefix in {text!r}')

        core = strip_basis_suffix(raw_piece)
        piece_end = piece_start + len(core)

        ref_id, carry_book, carry_chapter = resolve_single_piece(
            core, comma, carry_book, carry_chapter, chapters_in_cache,
            f'{context_label} ({text!r})'
        )
        refs.append((piece_start, piece_end, ref_id))
        i = nxt

    if not refs:
        sys.exit(f'{context_label}: resolved zero citations in {text!r}')
    return refs


def resolve_single_piece(core, comma, carry_book, carry_chapter, chapters_in_cache, ctx):
    # Match longest printed book prefix if present.
    book_code = None
    figures = core
    for name in sorted(PRINTED_BOOK_TO_CODE, key=len, reverse=True):
        if core.startswith(name + ' '):
            cand_fig = core[len(name) + 1:].strip()
            if FIGURES_RE.match(cand_fig):
                book_code = PRINTED_BOOK_TO_CODE[name]
                figures = cand_fig
                break

    m = FIGURES_RE.match(figures)
    if not m:
        sys.exit(f'{ctx}: cannot parse citation piece {core!r}')
    n1 = int(m.group(1))
    n2 = int(m.group(2)) if m.group(2) is not None else None
    dash = m.group(3)
    n3 = int(m.group(4)) if m.group(4) is not None else None
    n4 = int(m.group(5)) if m.group(5) is not None else None

    if comma:
        # After a comma, either bare verse/range in carry_chapter ("66:10,13") or
        # chapter:verse in carry_book if a publisher wrote "14:1, 53:1".
        if book_code is not None:
            book = book_code
        else:
            if not carry_book:
                sys.exit(f'{ctx}: comma continuation {core!r} without carried book')
            book = carry_book
        if n2 is None and n4 is None:
            if not carry_chapter:
                sys.exit(f'{ctx}: comma verse continuation {core!r} without carried chapter')
            ch = carry_chapter
            if dash is None:
                ref_id = f'{book}.{ch}.{n1}'
            else:
                if n3 <= n1:
                    sys.exit(f'{ctx}: non-increasing verse range {core!r}')
                ref_id = f'{book}.{ch}.{n1}-{book}.{ch}.{n3}'
            verify_ref_id(ref_id, chapters_in_cache, ctx)
            return ref_id, book, ch
    else:
        book = book_code or carry_book
        if not book:
            sys.exit(f'{ctx}: citation {core!r} has no book')

    if n2 is not None:
        ch1, v1 = n1, n2
        if dash is None:
            ref_id = f'{book}.{ch1}.{v1}'
            next_ch = ch1
        elif n4 is None:
            if n3 <= v1:
                sys.exit(f'{ctx}: non-increasing verse range {core!r}')
            ref_id = f'{book}.{ch1}.{v1}-{book}.{ch1}.{n3}'
            next_ch = ch1
        else:
            ch2, v2 = n3, n4
            if (ch2, v2) <= (ch1, v1):
                sys.exit(f'{ctx}: non-increasing cross-chapter range {core!r}')
            ref_id = f'{book}.{ch1}.{v1}-{book}.{ch2}.{v2}'
            next_ch = None
    else:
        if book in ONE_CHAPTER_BOOKS:
            if dash is None:
                ref_id = f'{book}.1.{n1}'
            elif n4 is None:
                if n3 <= n1:
                    sys.exit(f'{ctx}: non-increasing one-chapter range {core!r}')
                ref_id = f'{book}.1.{n1}-{book}.1.{n3}'
            else:
                sys.exit(f'{ctx}: unexpected colon in one-chapter range {core!r}')
            next_ch = 1
        else:
            if dash is None:
                ref_id = f'{book}.{n1}'
            elif n4 is None:
                if n3 <= n1:
                    sys.exit(f'{ctx}: non-increasing chapter range {core!r}')
                ref_id = f'{book}.{n1}-{book}.{n3}'
            else:
                sys.exit(f'{ctx}: chapter-to-verse range missing start verse {core!r}')
            next_ch = None

    verify_ref_id(ref_id, chapters_in_cache, ctx)
    return ref_id, book, next_ch


def verify_ref_id(ref_id, chapters_in_cache, ctx):
    parts = ref_id.split('-')
    for part in parts:
        fields = part.split('.')
        book_name = USFM_TO_BOOK.get(fields[0])
        if not book_name or book_name not in chapters_in_cache:
            sys.exit(f'{ctx}: target book {fields[0]} ({book_name}) not in cache for {ref_id}')
        ch = int(fields[1])
        ch_verses = chapters_in_cache[book_name].get(ch)
        if not ch_verses:
            sys.exit(f'{ctx}: target chapter {book_name} {ch} not in cache for {ref_id}')
        if len(fields) == 3:
            v = int(fields[2])
            if v not in ch_verses:
                sys.exit(f'{ctx}: target verse {book_name} {ch}:{v} not in cache for {ref_id}')


def extract_web_family_notes(edition_id, label, usfm_dir, cache_path, canon, allow_partial):
    verses_cache, chapters_cache, _ = load_cache(cache_path)
    books = read_canon_usfm(usfm_dir, canon, label, set(chapters_cache), allow_partial)
    out = {}

    token_re = re.compile(
        r'\\f\s.*?\\f\*|\\x\s.*?\\x\*|\\(?:ca|va|vp)\s.*?\\(?:ca|va|vp)\*'
        r'|\\\+?w\s+([^|\\]*?)(?:\|[^\\]*?)?\\\+?w\*'
        r'|\\c\s+\d+|\\v\s+\d+|\\\+?[a-z0-9]+\*?|[^\\]+',
        flags=re.S,
    )

    for _, book_name, raw_usfm in books:
        ch = 0
        v = 0
        in_non_verse = False
        # Per-verse list of ('text', str) or ('xref', note_text)
        verse_parts = {}

        for m in token_re.finditer(raw_usfm):
            tok = m.group(0)
            if m.group(1) is not None:
                if ch > 0 and v > 0 and not in_non_verse:
                    verse_parts.setdefault((ch, v), []).append(('text', m.group(1)))
                continue
            if tok.startswith('\\f') or tok.startswith('\\ca') or tok.startswith('\\va') or tok.startswith('\\vp'):
                continue
            if tok.startswith('\\x'):
                if ch == 0 or v == 0 or in_non_verse:
                    sys.exit(f'{label}: \\x outside verse text in {book_name} ch={ch} v={v}: {tok!r}')
                note_text = clean_xref_note_body(tok)
                if not note_text:
                    sys.exit(f'{label}: empty \\x note in {book_name} {ch}:{v}')
                verse_parts.setdefault((ch, v), []).append(('xref', note_text))
                continue
            mc = CHAPTER_TOKEN.fullmatch(tok)
            if mc:
                ch = int(mc.group(1))
                v = 0
                in_non_verse = False
                continue
            mv = VERSE_TOKEN.fullmatch(tok)
            if mv:
                v = int(mv.group(1))
                in_non_verse = False
                verse_parts.setdefault((ch, v), [])
                continue
            if tok.startswith('\\'):
                marker = tok[1:].lstrip('+').rstrip('*')
                if NON_VERSE_PARAGRAPH.fullmatch(marker):
                    in_non_verse = True
                elif VERSE_PARAGRAPH.fullmatch(marker):
                    in_non_verse = False
                    # Paragraph/poetry break inside a verse separates words with whitespace.
                    if ch > 0 and v > 0:
                        verse_parts.setdefault((ch, v), []).append(('text', '\n'))
                continue
            if ch > 0 and v > 0 and not in_non_verse:
                verse_parts.setdefault((ch, v), []).append(('text', tok))

        for (ch_num, v_num), raw_parts in sorted(verse_parts.items()):
            if not any(kind == 'xref' for kind, _ in raw_parts):
                continue
            parts = []
            for kind, val in raw_parts:
                if kind == 'text' and parts and parts[-1][0] == 'text':
                    parts[-1] = ('text', parts[-1][1] + val)
                else:
                    parts.append((kind, val))
            key = f'{book_name} {ch_num}:{v_num}'
            if (book_name, ch_num, v_num) not in verses_cache:
                sys.exit(f'{label}: {key} has \\x in USFM but verse is missing from runtime cache')
            rt_text = verses_cache[(book_name, ch_num, v_num)]

            recovery = VERSE_WORDING_RECOVERIES.get((edition_id, key))
            if recovery:
                old_w, new_w = recovery
                replaced = False
                new_parts = []
                for kind, val in parts:
                    if kind == 'text' and old_w in val:
                        val = val.replace(old_w, new_w)
                        replaced = True
                    new_parts.append((kind, val))
                if not replaced:
                    sys.exit(f'{label}: unused wording recovery on {key}')
                parts = new_parts

            full_usfm_text = ''.join(val for kind, val in parts if kind == 'text')
            if alnum_only(full_usfm_text) != alnum_only(rt_text):
                sys.exit(
                    f'{label}: alphanumeric mismatch at {key}:\n'
                    f'  USFM:    {full_usfm_text!r}\n'
                    f'  runtime: {rt_text!r}'
                )

            # Build per-note Anchor and AtEnd against rt_text.
            notes_out = []
            for idx, (kind, val) in enumerate(parts):
                if kind != 'xref':
                    continue
                before_txt = ''.join(p_val for p_kind, p_val in parts[:idx] if p_kind == 'text')
                after_txt = ''.join(p_val for p_kind, p_val in parts[idx + 1:] if p_kind == 'text')
                at_end = (alnum_only(after_txt) == '' and after_txt.strip() == '')
                if at_end:
                    anchor = len(rt_text)
                else:
                    anchor = locate_mid_verse_anchor(before_txt, after_txt, rt_text, f'{label} {key}')
                refs = parse_citation_pieces(val, chapters_cache, f'{label} {key}')
                notes_out.append({
                    'Anchor': anchor,
                    'AtEnd': at_end,
                    'Text': val,
                    'Refs': refs,
                })

            out[key] = {
                'Runes': len(rt_text),
                'Hash': fnv1a64(rt_text),
                'Notes': notes_out,
            }

    return out


def locate_mid_verse_anchor(before_txt, after_txt, rt_text, ctx):
    """Map a mid-verse \\x position onto rt_text by exact alphanumeric count and boundary side."""
    # Does \x attach flush BEFORE the following word (whitespace before \x and
    # non-whitespace immediately after \x*), or flush AFTER the preceding word?
    before_ends_space = (before_txt == '' or before_txt[-1].isspace())
    after_starts_nonspace = (after_txt != '' and not after_txt[0].isspace())
    attaches_before = before_ends_space and after_starts_nonspace

    target_alnum = len(alnum_only(before_txt))
    if target_alnum == 0:
        if not attaches_before:
            sys.exit(f'{ctx}: note at start of verse does not attach before word')
        pos = 0
        while pos < len(rt_text) and rt_text[pos].isspace():
            pos += 1
        return pos

    # Advance to the target_alnum-th alphanumeric character in rt_text.
    seen = 0
    pos = 0
    while pos < len(rt_text):
        if rt_text[pos].isalnum():
            seen += 1
            if seen == target_alnum:
                pos += 1
                break
        pos += 1
    if seen != target_alnum:
        sys.exit(f'{ctx}: failed to count {target_alnum} alnum chars in runtime text')

    # Advance past immediate trailing punctuation attached to that word (before any whitespace).
    # In USFM before_txt, count non-whitespace characters after its last alnum char.
    last_alnum_usfm = len(before_txt) - 1
    while last_alnum_usfm >= 0 and not before_txt[last_alnum_usfm].isalnum():
        last_alnum_usfm -= 1
    usfm_trail = ''
    j = last_alnum_usfm + 1
    while j < len(before_txt) and not before_txt[j].isspace():
        usfm_trail += before_txt[j]
        j += 1

    # Advance past the matching punctuation run in rt_text (stopping at whitespace).
    rt_punct_start = pos
    while pos < len(rt_text) and not rt_text[pos].isspace() and not rt_text[pos].isalnum():
        if attaches_before and len(usfm_trail) == 0:
            break
        pos += 1
        if not attaches_before and (pos - rt_punct_start) >= len(usfm_trail) and pos < len(rt_text) and rt_text[pos].isspace():
            break

    if attaches_before:
        while pos < len(rt_text) and rt_text[pos].isspace():
            pos += 1
        if pos >= len(rt_text) or rt_text[pos].isspace():
            sys.exit(f'{ctx}: attaches_before anchor landed on whitespace at {pos}')
        return pos

    # Flush-after-word anchor must stand at end of word/clause right before whitespace.
    if pos < len(rt_text) and not rt_text[pos].isspace():
        sys.exit(
            f'{ctx}: flush-after anchor {pos} is not followed by whitespace in {rt_text!r} '
            f'(usfm_trail={usfm_trail!r})'
        )
    return pos


REF_TAG_RE = re.compile(r'\\ref\s+([^|\\]+?)(?:\|([^\\]+?))?\\ref\*')
HEADING_LINE_RE = re.compile(r'^\\(s\d*)\b\s*(.*)$')
REF_LINE_RE = re.compile(r'^\\r\b\s*(.*)$')


def verify_bsb_target_attr(first_ref_id, target_attr, ctx):
    """Cross-check BSB's |TARGET attribute against the first parsed NoteRef ID.

    bsb2usfm omits the USFM book code on Song of Solomon references even in
    other books ("\\ref Song of Solomon 1:1–17|1:1-17\\ref*" at Ephesians 5:21),
    so the book code is checked when present and the chapter/verse numbers
    always.
    """
    if not target_attr:
        return
    t = target_attr.strip()
    m = re.fullmatch(r'(?:([A-Z0-9]{3})\s+)?(\d+)(?::(\d+))?(?:[-–—](\d+)(?::(\d+))?)?', t)
    if not m:
        sys.exit(f'{ctx}: unrecognized BSB |target attribute {target_attr!r}')
    if m.group(1):
        code = BOOK_TO_ID_CODE[USFM_TO_BOOK[m.group(1)]]
        if not first_ref_id.startswith(code + '.'):
            sys.exit(f'{ctx}: BSB |target {target_attr!r} disagrees with {first_ref_id}')
    # Verify first chapter/verse number matches.
    first_part = first_ref_id.split('-')[0].split('.')
    if m.group(3) is not None:
        want_nums = (int(m.group(2)), int(m.group(3)))
        got_nums = (int(first_part[1]), int(first_part[2]))
    elif first_part[0] in ONE_CHAPTER_BOOKS:
        want_nums = (1, int(m.group(2)))
        got_nums = (int(first_part[1]), int(first_part[2]))
    else:
        want_nums = (int(m.group(2)),)
        got_nums = (int(first_part[1]),)
    if want_nums != got_nums:
        sys.exit(f'{ctx}: BSB |target {target_attr!r} numbers disagree with {first_ref_id}')


def extract_bsb_heading_notes(usfm_dir, cache_path, allow_partial):
    _, chapters_cache, headings_cache = load_cache(cache_path)
    books = read_canon_usfm(usfm_dir, CANON_66, 'BSB', set(chapters_cache), allow_partial)
    out = {}

    for book_code, book_name, raw_usfm in books:
        if '\\x ' in raw_usfm:
            sys.exit(f'BSB: unexpected \\x cross-reference marker in {book_name}')
        ch = 0
        pending_heads = []  # list of {'text': str, 'refs': [str]} before next \v
        per_chapter = {}

        for raw_line in raw_usfm.splitlines():
            line = raw_line.strip()
            if not line:
                continue
            mc = CHAPTER_TOKEN.match(line)
            if mc:
                if any(ph['refs'] for ph in pending_heads):
                    sys.exit(f'BSB: trailing noted heading at end of {book_name} {ch} without verse')
                pending_heads = []
                ch = int(mc.group(1))
                continue
            mh = HEADING_LINE_RE.match(line)
            if mh:
                h_text = re.sub(r'\\f\s.*?\\f\*', '', mh.group(2))
                h_text = re.sub(r'\\\+?[a-z0-9]+\*?', '', h_text)
                h_text = re.sub(r'\s+', ' ', h_text).strip()
                if h_text:
                    pending_heads.append({'text': h_text, 'refs': []})
                continue
            mr = REF_LINE_RE.match(line)
            if mr:
                if not pending_heads:
                    sys.exit(f'BSB: \\r without preceding heading in {book_name} {ch}: {line!r}')
                pending_heads[-1]['refs'].append(mr.group(1).strip())
                continue
            mv = VERSE_TOKEN.search(line)
            if mv and ch > 0 and pending_heads:
                before_v = int(mv.group(1))
                if any(ph['refs'] for ph in pending_heads):
                    ch_heads = headings_cache.get(book_name, {}).get(ch, [])
                    # Filter runtime headings at before_v, excluding any leaked "(...)" \r duplicate
                    # when real headings stand at the same verse (Hebrews 11:30 "(Joshua–Malachi)").
                    raw_at_v = [h for h in ch_heads if h['BeforeVerse'] == before_v and h['Style'] == 'heading']
                    usfm_texts = {ph['text'] for ph in pending_heads}
                    rt_at_v = [
                        h for h in raw_at_v
                        if not (h['Text'].startswith('(') and h['Text'].endswith(')') and h['Text'] not in usfm_texts and len(raw_at_v) > 1)
                    ]

                    search_from = 0
                    for ph in pending_heads:
                        if not ph['refs']:
                            continue
                        want_text = BSB_HEADING_RECOVERIES.get(
                            (f'{book_name} {ch}:{before_v}', ph['text']), ph['text']
                        )
                        matched_ord = None
                        for idx in range(search_from, len(rt_at_v)):
                            if rt_at_v[idx]['Text'] == want_text:
                                matched_ord = idx
                                search_from = idx + 1
                                break
                        if matched_ord is None:
                            sys.exit(
                                f'BSB: noted heading {ph["text"]!r} at {book_name} {ch}:{before_v} '
                                f'missing from runtime cache headings {[h["Text"] for h in rt_at_v]}'
                            )
                        rt_h = rt_at_v[matched_ord]
                        for r_raw in ph['refs']:
                            ctx = f'BSB {book_name} {ch}:{before_v} ({ph["text"]})'
                            inner = r_raw
                            if inner.startswith('(') and inner.endswith(')'):
                                inner = inner[1:-1].strip()
                            else:
                                sys.exit(f'{ctx}: expected outer parentheses on \\r {r_raw!r}')

                            # Unwrap \ref DISPLAY|TARGET\ref* while recording target attributes.
                            target_checks = []
                            def replace_ref(m_ref):
                                disp = m_ref.group(1).strip()
                                targ = m_ref.group(2)
                                if targ:
                                    target_checks.append((disp, targ.strip()))
                                return disp

                            clean_inner = REF_TAG_RE.sub(replace_ref, inner)
                            if '\\' in clean_inner:
                                sys.exit(f'{ctx}: leftover USFM markup in \\r {clean_inner!r}')
                            clean_inner = re.sub(r'\s+', ' ', clean_inner).strip()

                            refs = parse_citation_pieces(clean_inner, chapters_cache, ctx)
                            for disp, targ in target_checks:
                                disp_refs = parse_citation_pieces(disp, chapters_cache, ctx)
                                if disp_refs:
                                    verify_bsb_target_attr(disp_refs[0][2], targ, ctx)

                            per_chapter.setdefault(ch, []).append({
                                'BeforeVerse': before_v,
                                'HeadingOrd': matched_ord,
                                'HeadingHash': fnv1a64(rt_h['Text']),
                                'Text': clean_inner,
                                'Refs': refs,
                            })
                pending_heads = []

        if any(ph['refs'] for ph in pending_heads):
            sys.exit(f'BSB: trailing noted heading at end of {book_name} {ch} without verse')

        for ch_num, notes in sorted(per_chapter.items()):
            out[f'{book_name} {ch_num}'] = notes

    return out


def go_quote(s):
    return json.dumps(s, ensure_ascii=False)


def format_refs(refs):
    if not refs:
        return 'nil'
    items = ', '.join(f'{{{go_quote(r_id)}, {start}, {end}}}' for start, end, r_id in refs)
    return f'[]NoteRef{{{items}}}'


def emit_verse_map(lines, var_name, label, data):
    note_count = sum(len(v['Notes']) for v in data.values())
    ref_count = sum(len(n['Refs']) for v in data.values() for n in v['Notes'])
    lines.append(
        f'// {var_name} holds the {label} cross-reference notes ({note_count} notes, '
        f'{ref_count} citations across {len(data)} verses).'
    )
    lines.append(f'var {var_name} = map[string]publisherVerseEntry{{')
    for key in sorted(data):
        entry = data[key]
        lines.append(f'\t{go_quote(key)}: {{')
        lines.append(f'\t\tRunes: {entry["Runes"]},')
        lines.append(f'\t\tHash:  0x{entry["Hash"]:016x},')
        lines.append('\t\tNotes: []publisherVerseNote{')
        for n in entry['Notes']:
            at_end = 'true' if n['AtEnd'] else 'false'
            lines.append(
                f'\t\t\t{{Anchor: {n["Anchor"]}, AtEnd: {at_end}, '
                f'Text: {go_quote(n["Text"])}, Refs: {format_refs(n["Refs"])}}},'
            )
        lines.append('\t\t},')
        lines.append('\t},')
    lines.append('}\n')
    return note_count, ref_count


def emit_heading_map(lines, var_name, label, data):
    note_count = sum(len(v) for v in data.values())
    ref_count = sum(len(n['Refs']) for v in data.values() for n in v)
    lines.append(
        f'// {var_name} holds the {label} section-heading reference lines '
        f'({note_count} notes, {ref_count} citations across {len(data)} chapters).'
    )
    lines.append(f'var {var_name} = map[string][]publisherHeadingNote{{')
    for key in sorted(data):
        lines.append(f'\t{go_quote(key)}: {{')
        for n in data[key]:
            lines.append(
                f'\t\t{{BeforeVerse: {n["BeforeVerse"]}, HeadingOrd: {n["HeadingOrd"]}, '
                f'HeadingHash: 0x{n["HeadingHash"]:016x}, Text: {go_quote(n["Text"])}, '
                f'Refs: {format_refs(n["Refs"])}}},'
            )
        lines.append('\t},')
    lines.append('}\n')
    return note_count, ref_count


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--web-usfm', required=True)
    ap.add_argument('--web-cache', required=True)
    ap.add_argument('--webc-usfm', required=True)
    ap.add_argument('--webc-cache', required=True)
    ap.add_argument('--bsb-usfm', required=True)
    ap.add_argument('--bsb-cache', required=True)
    ap.add_argument('--out', required=True)
    ap.add_argument('--allow-partial-canon', action='store_true')
    args = ap.parse_args()

    web = extract_web_family_notes(
        'web', 'WEB', args.web_usfm, args.web_cache, CANON_66, args.allow_partial_canon
    )
    webc = extract_web_family_notes(
        'webc', 'WEB Catholic', args.webc_usfm, args.webc_cache, CANON_CATHOLIC, args.allow_partial_canon
    )
    bsb = extract_bsb_heading_notes(args.bsb_usfm, args.bsb_cache, args.allow_partial_canon)

    lines = [
        'package bibletext\n',
        '// Code generated by scripts/gen-publisher-xrefs.py. DO NOT EDIT BY HAND.',
        '//',
        "// The WEB's, WEB Catholic's and BSB's own cross-references, recovered from",
        '// their published public-domain / CC0 USFM because bible.helloao.org strips',
        '// \\x cross-reference notes and \\r section-heading reference lines.\n',
    ]
    web_n, web_r = emit_verse_map(lines, 'webPublisherCrossRefs', 'WEB', web)
    webc_n, webc_r = emit_verse_map(lines, 'webcPublisherCrossRefs', 'WEB Catholic', webc)
    bsb_n, bsb_r = emit_heading_map(lines, 'bsbPublisherCrossRefs', 'BSB', bsb)

    src = '\n'.join(lines)
    fmt = subprocess.run(['gofmt'], input=src.encode('utf-8'), capture_output=True)
    if fmt.returncode != 0:
        sys.exit(f'gofmt failed:\n{fmt.stderr.decode("utf-8", errors="replace")}')

    os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
    with open(args.out, 'wb') as fh:
        fh.write(fmt.stdout)

    print(
        f'WEB {web_n} notes ({web_r} refs, {len(web)} verses); '
        f'WEBC {webc_n} notes ({webc_r} refs, {len(webc)} verses); '
        f'BSB {bsb_n} notes ({bsb_r} refs, {len(bsb)} chapters) -> {args.out}'
    )


if __name__ == '__main__':
    main()
