#!/usr/bin/env python3
"""Build the WEB's and WEB Catholic's red-letter spans from their own USFM.

    python3 scripts/gen-web-redletter.py \
        --web-usfm  <dir of eng-web USFM>   --web-cache  ~/Library/Caches/bibletext/bibletext-web-v9.json \
        --webc-usfm <dir of eng-web-c USFM> --webc-cache ~/Library/Caches/bibletext/bibletext-webc-v9.json

Both editions mark the words of Jesus themselves, with \\wj … \\wj* in the USFM
eBible publishes, so nothing here is our editorial judgement — it is the
translators'.

NO API CALLS. Fetch the USFM once from eBible and point this at the unpacked
directories:

    https://ebible.org/Scriptures/eng-web_usfm.zip
    https://ebible.org/Scriptures/eng-web-c_usfm.zip

WHY THE SPANS ARE LOCATED BY TEXT rather than taken as USFM character positions:
the runtime supplier's copy of the text is NOT byte-identical to eBible's
revision. Matching each span's own text inside the app's verse text, tolerating
whitespace differences, produces exact spans where every marked run locates.
The match is made at the run's own place: its offset in the verse, counted in
letters and digits while walking the USFM, so the same words earlier in the
verse (another speaker's "I am he.", or "Go" inside "Gospel") cannot take it.
The walk carries the letters before the run as well, and the runtime verse
must have the same ones before it. When it does not, or the run's letters are
not at that offset, the run is unplaced; with --allow-relocated it is placed
instead where its letters occur once in the whole verse, after the run before
it, and listed. A run is never placed with an end inside a word.
Seven known revision differences use small, shape-checked boundary recoveries;
generation fails on every other mismatch. A separate publisher-marked verse set
provides a safe edition-local fallback if the runtime text later changes.

See docs/TEXTUAL-DATA.md §3 for the whole picture.
"""
import argparse, glob, json, os, re, subprocess, sys

BOOK = {'MAT': 'Matthew', 'MRK': 'Mark', 'LUK': 'Luke', 'JHN': 'John', 'ACT': 'Acts',
        '1CO': '1 Corinthians', '2CO': '2 Corinthians', '1TI': '1 Timothy', 'REV': 'Revelation'}

# The runtime supplier's WEB-family revision differs from the current eBible
# USFM in these verses. These are boundary recoveries, not red-letter
# judgements: each key is already publisher-marked, and the recovery merely
# maps the publisher's \wj boundaries onto the older runtime wording.
#
# "whole" is used only where \wj covers the complete source verse. "quoted-tail"
# maps a black narrative lead-in followed by one marked quotation. "two-speeches"
# maps marked speech, black narration, then marked speech. The WEBC Mark 9:47
# mismatch is a runtime "oF" typo inside an otherwise wholly marked verse.
RECOVERY = {
    ('web', 'John 3:7'): 'whole',
    ('web', 'John 4:48'): 'quoted-tail',
    ('web', 'Luke 8:6'): 'whole',
    ('web', 'Luke 8:7'): 'whole',
    ('web', 'Luke 8:8'): 'two-speeches',
    ('web', 'Luke 12:51'): 'whole',
    ('webc', 'John 3:7'): 'whole',
    ('webc', 'John 4:48'): 'quoted-tail',
    ('webc', 'Luke 8:6'): 'whole',
    ('webc', 'Luke 8:7'): 'whole',
    ('webc', 'Luke 8:8'): 'two-speeches',
    ('webc', 'Luke 12:51'): 'whole',
    ('webc', 'Mark 9:47'): 'whole',
}


# Paragraph markers whose lines are not verse text (headings, the references
# under them, titles, identification lines): not counted in a run's offset.
NON_VERSE_PARAGRAPH = re.compile(
    r's\d*|ms\d*|mr|r|sr|sp|qa|cl|cd|mt\d*|mte\d*|h\d*|toc\d*|toca\d*|id|ide|usfm|rem|sts')
# Paragraph markers that resume verse text after one of those.
VERSE_PARAGRAPH = re.compile(r'p|m|mi|nb|pc|pr|pm|pmo|pmc|pmr|pi\d*|ph\d*|li\d*|q\d*|qr|qc|qm\d*|qd|b')
# A chapter or verse number token, exactly: \cl, \cd, \ca, \va and \vp begin
# with the same letters and are other markers.
CHAPTER_TOKEN = re.compile(r'\\c\s+(\d+)')
VERSE_TOKEN = re.compile(r'\\v\s+(\d+)')


def clean(t):
    t = re.sub(r'\\f\s.*?\\f\*', '', t, flags=re.S)     # footnotes are not verse text
    t = re.sub(r'\\x\s.*?\\x\*', '', t, flags=re.S)     # nor cross references
    t = re.sub(r'\\(ca|va|vp)\s.*?\\\1\*', '', t, flags=re.S)  # nor alternate or published numbers
    # \w word|strong="G1"\w* -> word. Strip these BEFORE splitting on backslashes:
    # doing it after leaves a bare "w" that a naive regex then eats out of every
    # word containing one ("which" -> "hich"), which is how this went wrong first.
    return re.sub(r'\\\+?w\s+([^|\\]*?)(?:\|[^\\]*?)?\\\+?w\*', r'\1', t)


def wj_texts(usfm_dir):
    """{book: {(chapter, verse): [(before, span text), …]}} — each \\wj span's own
    words, and the letters and digits (alphanumeric_map) of the verse's own
    text, without notes or heading lines, that come before it; their count is
    where it begins in its verse. A span carried on into the next verse has
    nothing before it there."""
    out = {}
    for f in glob.glob(os.path.join(usfm_dir, '*.usfm')):
        code = re.search(r'-([A-Z0-9]{3})eng', os.path.basename(f))
        if not code or code.group(1) not in BOOK:
            continue
        t = clean(open(f, encoding='utf-8').read())
        ch = v = 0
        inwj = heading = False
        cur, d = [], {}
        letters, before = [], ''
        def flush():
            if inwj and cur:
                d.setdefault((ch, v), []).append((before, re.sub(r'\s+', ' ', ''.join(cur)).strip()))
        for tok in re.findall(r'\\wj\*|\\wj\b|\\c\s+\d+|\\v\s+\d+|\\[a-z0-9]+\*?|[^\\]+', t):
            chapter, verse = CHAPTER_TOKEN.fullmatch(tok), VERSE_TOKEN.fullmatch(tok)
            if tok.startswith('\\wj*'):
                flush(); cur = []; inwj = False
            elif tok.startswith('\\wj'):
                inwj = True; cur = []; before = ''.join(letters)
            elif chapter:
                flush(); cur = []; ch = int(chapter.group(1)); v = 0; letters = []; heading = False
            elif verse:
                flush(); cur = []; v = int(verse.group(1)); letters, before = [], ''; heading = False
            elif tok.startswith('\\'):
                marker = tok[1:].rstrip('*')
                if NON_VERSE_PARAGRAPH.fullmatch(marker):
                    heading = True
                elif VERSE_PARAGRAPH.fullmatch(marker):
                    heading = False
            else:
                if inwj and ch and v:
                    cur.append(tok)
                if not heading:
                    letters.append(alphanumeric_map(tok)[0])
        flush()
        out[BOOK[code.group(1)]] = d
    return out


def alphanumeric_map(text):
    """Casefolded letters and digits of text, and the rune each came from."""
    letters, positions = [], []
    for index, char in enumerate(text):
        if char.isalnum():
            for folded in char.casefold():
                letters.append(folded)
                positions.append(index)
    return ''.join(letters), positions


def locate(haystack, needle, offset, frm):
    """Find needle in haystack at its offset-th letter, at or after rune frm,
    tolerating differing whitespace: from a rune between the letter before the
    offset and the one at it. None when it is not there."""
    if not needle:
        return None
    letters, positions = alphanumeric_map(haystack)
    wanted, _ = alphanumeric_map(needle)
    if offset > len(letters) or (wanted and not letters.startswith(wanted, offset)):
        return None
    lowest = max(frm, positions[offset - 1] + 1 if offset else 0)
    pattern = re.compile(re.escape(needle).replace(r'\ ', r'\s+'))
    if not wanted:
        # Punctuation alone, such as a closing quotation mark a note parted
        # from its words: it lies between the letter before its offset and
        # the letter at it.
        m = pattern.search(haystack, lowest, positions[offset] if offset < len(positions) else len(haystack))
        return (m.start(), m.end()) if m else None
    for begin in range(lowest, positions[offset] + 1):
        m = pattern.match(haystack, begin)
        if m:
            return (m.start(), m.end())
    return None


def inside_word(text, index):
    """True when index falls between two letters or digits."""
    return 0 < index < len(text) and text[index - 1].isalnum() and text[index].isalnum()


def place(haystack, needle, before, frm, allow_relocated=False):
    """(span, relocated, None) for one publisher-marked run, or (None, False, why).

    before is the verse's letters ahead of the run in the publisher's text
    (wj_texts); its length is the run's offset. The run goes at that offset
    when the runtime verse has the same letters before it and the run's text
    there. Otherwise the runtime revision differs before it, and it is
    unplaced, unless allow_relocated is set and its letters occur once in the
    whole verse, after the run before it: with no copy, or two or more,
    nothing says which copy the publisher marked. A span with an end inside a
    word is refused."""
    offset = len(before)
    letters, _ = alphanumeric_map(haystack)
    loc = locate(haystack, needle, offset, frm) if letters[:offset] == before else None
    relocated = False
    if loc is None:
        why = f"the text before it differs from the publisher's, or its text is not at its offset {offset}"
        wanted, _ = alphanumeric_map(needle)
        copies = [i for i in range(len(letters)) if wanted and letters.startswith(wanted, i)]
        if len(copies) != 1:
            return None, False, f"{why}, and the verse holds {len(copies)} copies of its letters"
        loc, relocated = locate(haystack, needle, copies[0], frm), True
        if loc is None:
            return None, False, f"{why}, nor at the one copy of its letters after the run before it"
        if not allow_relocated:
            return None, False, f"{why}; the one copy of its letters is at {loc}, taken only with --allow-relocated"
    if inside_word(haystack, loc[0]) or inside_word(haystack, loc[1]):
        return None, False, f"it would begin or end inside a word at {loc}"
    return loc, relocated, None


def fnv1a64(text):
    value = 0xCBF29CE484222325
    for byte in text.encode('utf-8'):
        value ^= byte
        value = (value * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return value


def recovered_spans(edition, key, text):
    """Map a known source/runtime revision difference to checked rune spans."""
    kind = RECOVERY.get((edition, key))
    if kind == 'whole':
        return [(0, len(text))] if text else None
    if kind == 'quoted-tail':
        start = text.find('“')
        end = text.rfind('”') + 1
        if 0 < start < end == len(text):
            return [(start, end)]
        return None
    if kind == 'two-speeches':
        first_end = text.find('”') + 1
        second_start = text.find('“', first_end)
        if 0 < first_end < second_start < len(text) and text.endswith('”'):
            return [(0, first_end), (second_start, len(text))]
        return None
    return None


def build(edition, usfm_dir, cache_path, allow_relocated=False):
    blob = json.load(open(cache_path))
    app = (blob.get('data') or blob)['Verses']
    spans, runes, hashes, marked = {}, {}, {}, set()
    exact = recovered = 0
    relocated = []
    failures = []
    for book, d in wj_texts(usfm_dir).items():
        for (c, v), texts in sorted(d.items()):
            key = f"{book} {c}:{v}"
            marked.add(key)
            verses = {x['Verse']: x['Text'] for x in app.get(book, {}).get(str(c), [])}
            text = verses.get(v)
            if text is None:
                failures.append(f'{key} (missing runtime verse)')
                continue
            found, at, bad, moved = [], 0, None, 0
            for before, w in texts:
                loc, by_copy, why = place(text, w, before, at, allow_relocated)
                if loc:
                    found.append(loc); at = loc[1]; moved += by_copy
                else:
                    bad = bad or why
            if not found or bad:
                found = recovered_spans(edition, key, text)
                if not found:
                    failures.append(f'{key} (unplaced publisher run: {bad or "no run"})')
                    continue
                recovered += 1
            else:
                exact += 1
                if moved:
                    relocated.append(key)
            spans[key], runes[key], hashes[key] = found, len(text), fnv1a64(text)
    if failures:
        raise RuntimeError(f"{edition}: incomplete mapping: " + ', '.join(failures))
    return spans, runes, hashes, marked, (exact, recovered), relocated


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--web-usfm', required=True)
    ap.add_argument('--web-cache', required=True)
    ap.add_argument('--webc-usfm', required=True)
    ap.add_argument('--webc-cache', required=True)
    ap.add_argument('--out', default='red_letter_web_data.go')
    ap.add_argument('--allow-relocated', action='store_true',
                    help="place a run whose verse differs before it by the one copy of its letters, and list it")
    a = ap.parse_args()

    out = ["package bibletext", "",
           "// Code generated by scripts/gen-web-redletter.py. DO NOT EDIT BY HAND.",
           "//",
           "// The WEB's and WEB Catholic's own words-of-Jesus spans, as rune offsets into",
           "// each verse's text. Both editions mark them in their published USFM (\\wj), so",
           "// nothing here is our editorial judgement — it is the translators'.",
           "//",
           "// Located by matching each span's TEXT inside the app's verse text rather than",
           "// by trusting USFM character positions: the runtime supplier's copy is not",
           "// byte-identical to eBible's revision. Known differences use shape-checked",
           "// boundary recoveries; generation fails on any unaccounted mismatch. The marked",
           "// set preserves edition-local fallback if runtime text later changes.",
           ""]
    for name, usfm, cache in (('web', a.web_usfm, a.web_cache), ('webc', a.webc_usfm, a.webc_cache)):
        spans, runes, hashes, marked, (exact, recovered), relocated = build(name, usfm, cache, a.allow_relocated)
        run_count = sum(len(value) for value in spans.values())
        print(f"{name}: {len(spans)} span verses, {run_count} runs "
              f"({exact} exact, {recovered} boundary-recovered; "
              f"{len(relocated)} with a run placed by its only copy, not its offset)", file=sys.stderr)
        for key in relocated:
            print(f"  placed by its only copy: {key}", file=sys.stderr)
        out.append(f"var {name}RedLetterSpans = map[string][]redLetterSpan{{")
        for k in sorted(spans):
            out.append('\t"%s": {%s},' % (k, ", ".join("{%d, %d}" % (s, e) for s, e in spans[k])))
        out += ["}", "", f"var {name}RedLetterRunes = map[string]int{{"]
        for k in sorted(runes):
            out.append('\t"%s": %d,' % (k, runes[k]))
        out += ["}", "", f"var {name}RedLetterHashes = map[string]uint64{{"]
        for k in sorted(hashes):
            out.append('\t"%s": 0x%016x,' % (k, hashes[k]))
        out += ["}", "", f"var {name}RedLetterMarked = map[string]struct{{}}{{"]
        for k in sorted(marked):
            out.append('\t"%s": {},' % k)
        out += ["}", ""]
    open(a.out, 'w').write("\n".join(out))
    subprocess.run(['gofmt', '-w', a.out], check=True)
    print(f"wrote {a.out}", file=sys.stderr)


if __name__ == '__main__':
    main()
