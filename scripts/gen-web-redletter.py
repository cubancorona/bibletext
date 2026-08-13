#!/usr/bin/env python3
"""Build the WEB's and WEB Catholic's red-letter spans from their own USFM.

    python3 scripts/gen-web-redletter.py \
        --web-usfm  <dir of eng-web USFM>   --web-cache  ~/Library/Caches/bibletext/bibletext-web-v2.json \
        --webc-usfm <dir of eng-web-c USFM> --webc-cache ~/Library/Caches/bibletext/bibletext-webc-v2.json

Both editions mark the words of Jesus themselves, with \\wj … \\wj* in the USFM
eBible publishes, so nothing here is our editorial judgement — it is the
translators'. That is the opposite of the BSB, which publishes no such markup at
all and whose table had to be derived (scripts/gen-bsb-redletter.py).

NO API CALLS. Fetch the USFM once from eBible and point this at the unpacked
directories:

    https://ebible.org/Scriptures/eng-web_usfm.zip
    https://ebible.org/Scriptures/eng-web-c_usfm.zip

WHY THE SPANS ARE LOCATED BY TEXT rather than taken as USFM character positions:
the runtime supplier's copy of the text is NOT byte-identical to eBible's
revision. Matching each span's own text inside the app's verse text, tolerating
whitespace differences, located 2,053 of 2,054 verses for the WEB and 2,052 for
WEBC at the time of writing. Five verses per edition differ outright ("Other
seed" where eBible has "Some seed"), get no entry, and fall back to the
whole-verse answer — which is what the app did before spans existed, so a miss
degrades rather than breaks.

See docs/TEXTUAL-DATA.md §3 for the whole picture.
"""
import argparse, glob, json, os, re, sys

BOOK = {'MAT': 'Matthew', 'MRK': 'Mark', 'LUK': 'Luke', 'JHN': 'John', 'ACT': 'Acts',
        '1CO': '1 Corinthians', '2CO': '2 Corinthians', '1TI': '1 Timothy', 'REV': 'Revelation'}


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


def build(usfm_dir, cache_path):
    blob = json.load(open(cache_path))
    app = (blob.get('data') or blob)['Verses']
    spans, runes = {}, {}
    full = partial = miss = 0
    for book, d in wj_texts(usfm_dir).items():
        for (c, v), texts in sorted(d.items()):
            verses = {x['Verse']: x['Text'] for x in app.get(book, {}).get(str(c), [])}
            text = verses.get(v)
            if text is None:
                miss += 1
                continue
            found, at, bad = [], 0, False
            for w in texts:
                loc = locate(text, w, at)
                if loc:
                    found.append(loc); at = loc[1]
                else:
                    bad = True
            if not found:
                miss += 1
                continue
            key = f"{book} {c}:{v}"
            spans[key], runes[key] = found, len(text)
            partial += 1 if bad else 0
            full += 0 if bad else 1
    return spans, runes, (full, partial, miss)


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
           "// unlike the BSB nothing here is our editorial judgement — it is the",
           "// translators'.",
           "//",
           "// Located by matching each span's TEXT inside the app's verse text rather than",
           "// by trusting USFM character positions: the runtime supplier's copy is not",
           "// byte-identical to eBible's revision. A few verses per edition do not match at",
           "// all and are simply absent here, so they fall back to the whole-verse answer.",
           ""]
    for name, usfm, cache in (('web', a.web_usfm, a.web_cache), ('webc', a.webc_usfm, a.webc_cache)):
        spans, runes, (full, partial, miss) = build(usfm, cache)
        print(f"{name}: {len(spans)} verses (all spans located {full}, some {partial}, none {miss})",
              file=sys.stderr)
        out.append(f"var {name}RedLetterSpans = map[string][]redLetterSpan{{")
        for k in sorted(spans):
            out.append('\t"%s": {%s},' % (k, ", ".join("{%d, %d}" % (s, e) for s, e in spans[k])))
        out += ["}", "", f"var {name}RedLetterRunes = map[string]int{{"]
        for k in sorted(runes):
            out.append('\t"%s": %d,' % (k, runes[k]))
        out += ["}", ""]
    open(a.out, 'w').write("\n".join(out))
    print(f"wrote {a.out}", file=sys.stderr)


if __name__ == '__main__':
    main()
