#!/usr/bin/env python3
"""Build the BSB's red-letter spans — which words in the Berean Standard Bible
are Christ's — and emit red_letter_bsb_data.go.

    python3 scripts/gen-bsb-redletter.py \
        --usfm  <dir of eng-web USFM files>            \
        --bsb   ~/Library/Caches/bibletext/bibletext-bsb-v3.json \
        --adjudications scripts/data/bsb-redletter-adjudications.json \
        [--nkjv-wj scripts/data/nkjv-wj-verses.json]

WHY THIS EXISTS. The BSB is not a red-letter edition: its published USFM
(ebible.org, engbsb) contains zero \\wj markers. So unlike the WEB and the NKJV,
there is no publisher's judgement to copy, and the app previously reddened whole
verses using the WEB's verse-level marks — which put narration and, in about 79
verses, ANOTHER SPEAKER'S WORDS in red ("We are able", "No one, Lord",
"Caesar's").

This builds spans instead, by combining four independent signals the way a
person would:

  1. THE WEB'S \\wj SPANS. The same underlying Greek, so a verse the WEB marks is
     a verse where Christ speaks. Gives the candidate verses and the content of
     what He says.
  2. THE BSB'S OWN PUNCTUATION. Quotation marks delimit the speech. A verse is
     parsed into quoted regions, including a leading region for speech carried
     over from the previous verse (a closing quote with no opener) and
     single-quoted regions for the Lord quoted inside someone else's speech
     (Acts 11:7).
  3. THE ATTRIBUTION. '…,” Jesus replied' versus '…,” they answered'. This
     outranks similarity: in Matthew 22:21 the Pharisees' "Caesar\u2019s" is
     textually contained in Christ's reply and scores 1.00 on similarity, but the
     attribution settles it. Bare pronouns are treated as UNKNOWN, never as
     Jesus — Luke 7:40's '"Tell me, Teacher," he said' is Simon.
  4. THE NKJV'S OWN \\wj, fetched from API.Bible, as an independent editorial
     witness for the hard cases. Used to decide verses, not spans.

Anything those four leave ambiguous was read and adjudicated by hand; those
decisions, with a reason for each, live in the adjudications file and are applied
here verbatim. See docs/TEXTUAL-DATA.md for the full account.

RULES THAT EXIST BECAUSE THE OBVIOUS VERSION WAS WRONG:

  - A quotation that carries its OWN attribution is a new speech act, not a
    continuation of the previous one, even when nobody else is credited between
    them (Luke 7:40).
  - Similarity must never outrank a named attribution (Matthew 22:21).
  - A short quotation cannot be matched on similarity alone; the containment
    metric saturates on two or three words.
"""
import argparse

import re, json, glob, os, sys

BOOK={'MAT':'Matthew','MRK':'Mark','LUK':'Luke','JHN':'John','ACT':'Acts',
      '1CO':'1 Corinthians','2CO':'2 Corinthians','1TI':'1 Timothy','REV':'Revelation'}
STOP=set("the and of to a in that he it his him for is was with as they i you not be but me my "
         "them their we us this these those there then so had have has will would shall".split())

def clean(t):
    t=re.sub(r'\\f\s.*?\\f\*','',t,flags=re.S); t=re.sub(r'\\x\s.*?\\x\*','',t,flags=re.S)
    return re.sub(r'\\\+?w\s+([^|\\]*?)(?:\|[^\\]*?)?\\\+?w\*', r'\1', t)

def web_wj(usfm_dir='usfm'):
    """{book: {(ch,v): (wj_text, other_text)}} from the WEB's own \\wj markers."""
    out={}
    for f in glob.glob(os.path.join(usfm_dir,'*.usfm')):
        code=re.search(r'-([A-Z0-9]{3})eng',os.path.basename(f))
        if not code or code.group(1) not in BOOK: continue
        t=clean(open(f,encoding='utf-8').read())
        ch=v=0; inwj=False; wj={}; oth={}
        for tok in re.findall(r'\\wj\*|\\wj\b|\\c\s+\d+|\\v\s+\d+|\\[a-z0-9]+\*?|[^\\]+', t):
            if tok.startswith('\\wj*'): inwj=False
            elif tok.startswith('\\wj'): inwj=True
            elif tok.startswith('\\c'): ch,v=int(tok.split()[1]),0
            elif tok.startswith('\\v'): v=int(tok.split()[1])
            elif tok.startswith('\\'): pass
            elif ch and v: (wj if inwj else oth).setdefault((ch,v),[]).append(tok)
        d={}
        for k in set(wj)|set(oth):
            d[k]=(re.sub(r'\s+',' ',''.join(wj.get(k,[]))).strip(),
                  re.sub(r'\s+',' ',''.join(oth.get(k,[]))).strip())
        out[BOOK[code.group(1)]]=d
    return out

def toks(t, keep_short=False):
    ws = re.findall(r"[a-z']+", t.lower())
    if keep_short:
        # A short utterance is mostly stopwords and short words — "Go!", "Come
        # out of him!", "Call him." Filtering those away leaves an empty set and
        # scores 0.0, which silently dropped several of Christ's shortest
        # commands. For a short span, keep everything.
        return set(ws)
    return {w for w in ws if w not in STOP and len(w) > 2}

def sim(a, b):
    short = len(re.findall(r"[A-Za-z']+", a)) <= 5
    A, B = toks(a, short), toks(b, short)
    if not A or not B: return 0.0
    return len(A&B)/min(len(A),len(B))          # containment, not Jaccard: a BSB
                                                # segment is a PART of the WEB span

OPEN,CLOSE='\u201c','\u201d'
def regions(text):
    """Double-quoted regions as (start,end) rune offsets. An unclosed region runs
    to the end of the verse — BSB discourse opens a quote per paragraph and only
    closes it when the speech ends, so mid-discourse verses are legitimately
    unbalanced."""
    out=[]; start=None
    for i,c in enumerate(text):
        if c==OPEN:
            if start is None: start=i
        elif c==CLOSE and start is not None:
            out.append((start,i+1)); start=None
    if start is not None: out.append((start,len(text)))
    return out

SPEECH_VERB=r"(?:said|says|answered|replied|declared|told|asked|responded|continued|" \
            r"added|explained|exclaimed|shouted|cried|called|urged|warned|instructed|" \
            r"commanded|promised|prayed|began|spoke|testified|rebuked|proclaimed)"
# NAMED subjects only. A bare pronoun is not evidence: Luke 7:40 ends
# '"Tell me, Teacher," he said' — and that "he" is Simon, not Jesus. Treating
# pronouns as a Jesus attribution reddened the Pharisee's line.
JESUS=re.compile(r"\b(Jesus|the Lord|Christ|the Son of Man)\b")
PRONOUN=re.compile(r"^(He|he|She|she|They|they|it|It)$")
NOT_JESUS=re.compile(r"\b(Peter|Simon|Judas|Thomas|Philip|Andrew|Martha|Mary|Pilate|Herod|"
                     r"John|Paul|the (?:disciples|brothers|crowd|Jews|Pharisees|scribes|"
                     r"chief priests|people|men|women|others|servants|soldiers)|they|she|"
                     r"the (?:man|woman|father|mother|angel|voice|devil|tempter))\b")

def attribution(text, s, e):
    """Who is credited with the quotation at [s:e]? Returns 'jesus' | 'other' | ''.

    Looks at the narration immediately AFTER the closing quote first, because
    English puts the attribution there most often ('...,” Jesus replied'), then
    before the opening quote ('So Jesus told them, “...')."""
    after = text[e:e+60]
    m = re.match(r"[\s,]*([A-Za-z’' ]{0,30}?)\s*" + SPEECH_VERB, after)
    if m:
        who = m.group(1).strip()
        if who:
            if NOT_JESUS.search(who): return 'other'
            if JESUS.search(who): return 'jesus'
            if PRONOUN.match(who): return ''          # ambiguous, resolve elsewhere
    before = text[max(0,s-70):s]
    m = re.search(r"([A-Za-z’' ]{0,30}?)\s*" + SPEECH_VERB + r"[^.!?]{0,25}$", before)
    if m:
        who = m.group(1).strip()
        if who:
            if NOT_JESUS.search(who): return 'other'
            if JESUS.search(who): return 'jesus'
            if PRONOUN.match(who): return ''
    return ''

def has_own_attribution(text, s, e):
    """Does this quotation carry its own '…,” X said' clause?

    A quotation with an attribution of its own is a NEW speech act, not a
    continuation of the previous one — even when the attribution is only a
    pronoun. Luke 7:40 is the case that forced this: '“Simon, I have something
    to tell you.” / “Tell me, Teacher,” he said.' Nobody else is credited
    BETWEEN the two quotations, so a naive continuation rule reddened Simon's
    reply; but the reply has its own 'he said', and that is the signal."""
    return re.match(r"[\s,]*[A-Za-z’' ]{0,30}?\s*" + SPEECH_VERB, text[e:e+60]) is not None


OPEN, CLOSE = '“', '”'
SOPEN, SCLOSE = '‘', '’'


def regions2(text):
    """Quoted regions, plus a LEADING one when the verse opens with a closing
    quote it never opened — the BSB carries a speech across a verse boundary and
    only re-opens the quotation at a paragraph."""
    out, start, lead = [], None, None
    for i, c in enumerate(text):
        if c == OPEN and start is None:
            start = i
        elif c == CLOSE:
            if start is not None:
                out.append((start, i + 1)); start = None
            elif lead is None and not out:
                lead = (0, i + 1)
    if start is not None:
        out.append((start, len(text)))
    return lead, out


def single_regions(text):
    out, start = [], None
    for i, c in enumerate(text):
        if c == SOPEN and start is None:
            start = i
        elif c == SCLOSE and start is not None:
            out.append((start, i + 1)); start = None
    return out


def build(usfm_dir, bsb, adjudications, nkjv_wj):
    web = web_wj(usfm_dir)
    def verse(book, c, v):
        for x in bsb.get(book, {}).get(str(c), []):
            if x['Verse'] == v:
                return x['Text']
    def nkjv_marks(book, c, v):
        return v in nkjv_wj.get(f"{book} {c}", [])

    spans, stats = {}, {'whole': 0, 'partial': 0, 'none': 0, 'absent': 0, 'hand': 0}
    for book, d in web.items():
        for (c, v), (wj, other) in sorted(d.items()):
            if not wj:
                continue
            t = verse(book, c, v)
            if t is None:
                stats['absent'] += 1        # a verse the BSB omits entirely
                continue
            key = f"{book} {c}:{v}"
            hand = adjudications.get(key)
            lead, rs = regions2(t)
            picked = []
            if hand and hand.get('single_only_matching'):
                # Several single-quoted speeches in one verse, only some His:
                # keep the ones whose words are what the WEB marks.
                def content_sim(seg):
                    A, B = toks(seg), toks(wj)
                    return len(A & B) / min(len(A), len(B)) if A and B else 0.0
                picked = [x for x in single_regions(t) if content_sim(t[x[0]:x[1]]) >= 0.3]
                stats['hand'] += 1
            elif hand and hand.get('whole'):
                picked = [(0, len(t))]; stats['hand'] += 1
            elif hand and hand.get('single'):
                picked = single_regions(t); stats['hand'] += 1
            elif hand:
                stats['hand'] += 1
                if hand.get('lead') and lead:
                    picked.append(lead)
                for i, verdict in enumerate(hand.get('regions', [])):
                    if verdict == 'yes' and i < len(rs):
                        picked.append(rs[i])
            elif len(re.sub(r'[^A-Za-z]', '', other)) <= 2:
                picked = [(0, len(t))]      # the WEB marks the whole verse
            elif not rs and not lead:
                # NO QUOTATION MARKS AT ALL. Two quite different situations.
                #
                # Mid-discourse, the BSB opens a quotation at the start of the
                # speech and only closes it at the end, so an interior verse
                # carries none of its own (Luke 10:22, Mark 13:14, Matthew
                # 28:20). Those are wholly His.
                #
                # But the Lord's words are also written with SINGLE quotes when
                # someone recounts them inside their own speech — Paul before
                # the crowd in Acts 22, Peter in Acts 11. There the red belongs
                # to the single-quoted part only.
                singles = single_regions(t)
                if singles and sum(e - s for s, e in singles) < len(t):
                    picked = [x for x in singles if sim(t[x[0]:x[1]], wj) >= 0.25]
                elif not NOT_JESUS.search(t) and len(wj) >= 0.6 * len(t):
                    # Only when the WEB's marked words account for most of the
                    # verse. Otherwise the verse is somebody else's sentence with
                    # His words inside it — Acts 20:35 is Paul preaching, and
                    # only "It is more blessed to give than to receive" is the
                    # Lord's.
                    picked = [(0, len(t))]
            else:
                labels = []
                for (s, e) in rs:
                    at, sm = attribution(t, s, e), sim(t[s:e], wj)
                    labels.append('yes' if at == 'jesus' else 'no' if at == 'other'
                                  else 'yes' if (sm >= 0.9 and at != 'other')
                                  else 'yes' if (sm >= 0.75 and len(toks(t[s:e])) >= 4)
                                  else 'no' if sm <= 0.10 else '?')
                if len(rs) == 1 and nkjv_marks(book, c, v):
                    labels = ['yes']
                for i in range(1, len(labels)):
                    if (labels[i] == '?' and labels[i - 1] == 'yes'
                            and not NOT_JESUS.search(t[rs[i - 1][1]:rs[i][0]])
                            and not has_own_attribution(t, *rs[i])):
                        labels[i] = 'yes'
                if rs and nkjv_marks(book, c, v) and 'no' not in labels and not NOT_JESUS.search(t):
                    labels = ['yes'] * len(labels)
                picked = [rs[i] for i, l in enumerate(labels) if l == 'yes']
                # Speech carried in from the previous verse and closing here:
                # 'But so that you may know ... authority to forgive sins…" He
                # said to the paralytic' (Mark 2:10). The closing quote has no
                # opener in this verse, so it forms no region and was being
                # dropped entirely.
                if lead and sim(t[lead[0]:lead[1]], wj) >= 0.25:
                    picked.insert(0, lead)
                # The Lord quoted inside somebody else's speech, single-quoted.
                for sp in single_regions(t):
                    if any(sp[0] >= s0 and sp[1] <= e0 for s0, e0 in picked):
                        continue            # already inside a chosen region
                    if sim(t[sp[0]:sp[1]], wj) >= 0.35:
                        picked.append(sp)
            if picked:
                picked = sorted(set(picked))
                spans[key] = picked
                if len(picked) == 1 and picked[0] == (0, len(t)):
                    stats['whole'] += 1
                else:
                    stats['partial'] += 1
            else:
                stats['none'] += 1
    return spans, stats


def emit_go(spans, bsb):
    def verse_len(book, c, v):
        for x in bsb.get(book, {}).get(str(c), []):
            if x['Verse'] == v:
                return len(x['Text'])
        return 0
    out = ["package bibletext", "",
           "// Code generated by scripts/gen-bsb-redletter.py. DO NOT EDIT BY HAND.",
           "//",
           "// Which words of each BSB verse are Christ's, as rune offsets into the verse",
           "// text. The BSB ships no words-of-Jesus markup of its own, so these were",
           "// derived — see the generator and docs/TEXTUAL-DATA.md.",
           "//",
           "// runes is the length of the verse text these offsets were computed against.",
           "// If the supplied text ever changes, the offsets are meaningless, and a caller",
           "// that finds a length mismatch must fall back rather than paint the wrong words.",
           "", "var bsbRedLetterSpans = map[string][]redLetterSpan{"]
    for key in sorted(spans):
        book, cv = key.rsplit(' ', 1)
        c, v = cv.split(':')
        n = verse_len(book, int(c), int(v))
        pairs = ", ".join(f"{{{s}, {e}}}" for s, e in spans[key])
        out.append(f'\t"{key}": {{{pairs}}},')
    out.append("}")
    out.append("")
    out.append("// bsbRedLetterRunes is the length of the verse text each span set was computed")
    out.append("// against, so a caller can detect that the supplied text has changed.")
    out.append("var bsbRedLetterRunes = map[string]int{")
    for key in sorted(spans):
        book, cv = key.rsplit(' ', 1)
        c, v = cv.split(':')
        out.append(f'\t"{key}": {verse_len(book, int(c), int(v))},')
    out.append("}")
    out.append("")
    return "\n".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--usfm', required=True, help='directory of eng-web USFM files')
    ap.add_argument('--bsb', required=True, help="the app's BSB cache JSON")
    ap.add_argument('--adjudications', default='scripts/data/bsb-redletter-adjudications.json')
    ap.add_argument('--nkjv-wj', default='scripts/data/nkjv-wj-verses.json')
    ap.add_argument('--out', default='red_letter_bsb_data.go')
    a = ap.parse_args()

    blob = json.load(open(a.bsb))
    bsb = (blob.get('data') or blob)['Verses']
    adj = json.load(open(a.adjudications))
    nkjv = json.load(open(a.nkjv_wj)) if a.nkjv_wj else {}
    spans, stats = build(a.usfm, bsb, adj, nkjv)
    open(a.out, 'w').write(emit_go(spans, bsb))
    print(f"{a.out}: {len(spans)} verses  "
          f"({stats['whole']} whole, {stats['partial']} partial, "
          f"{stats['none']} with no red, {stats['absent']} absent from the BSB, "
          f"{stats['hand']} hand-adjudicated)", file=sys.stderr)


if __name__ == '__main__':
    main()
