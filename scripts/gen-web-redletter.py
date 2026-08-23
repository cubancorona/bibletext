#!/usr/bin/env python3
"""Build the WEB's and WEB Catholic's red-letter spans from their own USFM.

    python3 scripts/gen-web-redletter.py \
        --web-usfm  <dir of eng-web USFM>   --web-cache  ~/Library/Caches/bibletext/bibletext-web-v2.json \
        --webc-usfm <dir of eng-web-c USFM> --webc-cache ~/Library/Caches/bibletext/bibletext-webc-v2.json

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


def clean(t):
    t = re.sub(r'\\f\s.*?\\f\*', '', t, flags=re.S)     # footnotes are not verse text
    t = re.sub(r'\\x\s.*?\\x\*', '', t, flags=re.S)     # nor cross references
    # \w word|strong="G1"\w* -> word. Strip these BEFORE splitting on backslashes:
    # doing it after leaves a bare "w" that a naive regex then eats out of every
    # word containing one ("which" -> "hich"), which is how this went wrong first.
    return re.sub(r'\\\+?w\s+([^|\\]*?)(?:\|[^\\]*?)?\\\+?w\*', r'\1', t)


def wj_texts(usfm_dir):
    """{book: {(chapter, verse): [span text, …]}} — each \\wj span's own words."""
    out = {}
    for f in glob.glob(os.path.join(usfm_dir, '*.usfm')):
        code = re.search(r'-([A-Z0-9]{3})eng', os.path.basename(f))
        if not code or code.group(1) not in BOOK:
            continue
        t = clean(open(f, encoding='utf-8').read())
        ch = v = 0
        inwj = False
        cur, d = [], {}
        def flush():
            if inwj and cur:
                d.setdefault((ch, v), []).append(re.sub(r'\s+', ' ', ''.join(cur)).strip())
        for tok in re.findall(r'\\wj\*|\\wj\b|\\c\s+\d+|\\v\s+\d+|\\[a-z0-9]+\*?|[^\\]+', t):
            if tok.startswith('\\wj*'):
                flush(); cur = []; inwj = False
            elif tok.startswith('\\wj'):
                inwj = True; cur = []
            elif tok.startswith('\\c'):
                flush(); cur = []; ch = int(tok.split()[1]); v = 0
            elif tok.startswith('\\v'):
                flush(); cur = []; v = int(tok.split()[1])
            elif tok.startswith('\\'):
                pass
            elif inwj and ch and v:
                cur.append(tok)
        flush()
        out[BOOK[code.group(1)]] = d
    return out


def locate(haystack, needle, frm):
    """Find needle in haystack from frm, tolerating differing whitespace."""
    if not needle:
        return None
    m = re.compile(re.escape(needle).replace(r'\ ', r'\s+')).search(haystack, frm)
    return (m.start(), m.end()) if m else None


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


def build(edition, usfm_dir, cache_path):
    blob = json.load(open(cache_path))
    app = (blob.get('data') or blob)['Verses']
    spans, runes, hashes, marked = {}, {}, {}, set()
    exact = recovered = 0
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
            found, at, bad = [], 0, False
            for w in texts:
                loc = locate(text, w, at)
                if loc:
                    found.append(loc); at = loc[1]
                else:
                    bad = True
            if not found or bad:
                found = recovered_spans(edition, key, text)
                if not found:
                    failures.append(f'{key} (unlocated publisher run)')
                    continue
                recovered += 1
            else:
                exact += 1
            spans[key], runes[key], hashes[key] = found, len(text), fnv1a64(text)
    if failures:
        raise RuntimeError(f"{edition}: incomplete mapping: " + ', '.join(failures))
    return spans, runes, hashes, marked, (exact, recovered)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--web-usfm', required=True)
    ap.add_argument('--web-cache', required=True)
    ap.add_argument('--webc-usfm', required=True)
    ap.add_argument('--webc-cache', required=True)
    ap.add_argument('--out', default='red_letter_web_data.go')
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
        spans, runes, hashes, marked, (exact, recovered) = build(name, usfm, cache)
        run_count = sum(len(value) for value in spans.values())
        print(f"{name}: {len(spans)} span verses, {run_count} runs "
              f"({exact} exact, {recovered} boundary-recovered)", file=sys.stderr)
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
