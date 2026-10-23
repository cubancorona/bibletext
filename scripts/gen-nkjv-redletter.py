#!/usr/bin/env python3
"""Build the NKJV's red-letter spans from the publisher's own markup.

    BIBLETEXT_PROVIDER_ID_NKJV=<provider-id> \
        python3 scripts/gen-nkjv-redletter.py      # fetch (cached), then derive

    python3 scripts/gen-nkjv-redletter.py --no-fetch \
        [--html-cache <dir>] [--app-cache <bibletext-nkjv-v7.json>] [--out <file>]
                                                   # derive only: no key, no call

The NKJV's own spans are served by API.Bible as <span class="wj">. This is the publisher's editorial
judgement, not ours, which is the right basis for a licensed edition.

QUOTA. Each chapter is one API call and the account's allowance is small, so
every raw response is written to ~/.cache/bibletext-nkjv-html and NEVER
re-fetched: a run with a warm cache makes zero calls. The nine books with any
red letters are 174 chapters, i.e. 174 calls once, ever. Do not point this at a
cold cache casually.

LICENSING. The cache lives outside the repository and the generated Go file
holds OFFSETS ONLY — no NKJV text is stored anywhere in this repo. An offset is
a fact about the text rather than the text itself.

The offsets are located by matching each marked span's TEXT inside the app's own
verse text, tolerating whitespace differences: API.Bible's HTML runs verses
together without the space the app's copy has, so raw offsets from the HTML do
not transfer. All 2,054 marked verses located cleanly at the time of writing.

Each run is matched at its own place in the verse: the parser keeps the
verse's non-space characters as it reads them, leaving out what the app's
decoder leaves out of the verse (verse numbers, headings), and the run's
characters must be found at their count, with the same characters before
them in the app's verse. The same words earlier in the verse, in narration or
inside a longer word ("Go" in "Gospel"), cannot take its place. Where the
app's text before the run differs from the markup's, or the run's characters
are not at its count, generation fails, naming the verse and the run's number.
With --allow-relocated such a run is placed instead where its characters
occur once in the whole verse, after the run before it, and listed; with no
copy there, or two or more, it fails all the same. An end inside a word fails
generation too, as does a chapter missing from the cache. Nothing is searched
for again from the verse's start.

A place where the app's verse anchors a note is the edge of a word. At some
of the NKJV's notes the publisher's text has no space on either side of the
note, so two words run together in the markup fetched without notes and in
the app's verse alike, and a run that ends or begins there, or two runs that
touch there, would otherwise read as an end inside one word.

The characters before each run are held in memory only, to be compared, and
never printed: they are licensed text.
"""
import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

CACHE=os.path.expanduser('~/.cache/bibletext-nkjv-html')
PID=os.environ.pop('BIBLETEXT_PROVIDER_ID_NKJV', '').strip()
for personal_key_var in ('ANTHROPIC_API_KEY', 'OPENAI_API_KEY', 'GEMINI_API_KEY', 'XAI_API_KEY'):
    os.environ.pop(personal_key_var, None)
BOOKS={'Matthew':('MAT',28),'Mark':('MRK',16),'Luke':('LUK',24),'John':('JHN',21),'Acts':('ACT',28),
       '1 Corinthians':('1CO',16),'2 Corinthians':('2CO',13),'1 Timothy':('1TI',6),'Revelation':('REV',22)}


class SameOriginRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        old = urllib.parse.urlsplit(req.full_url)
        new = urllib.parse.urlsplit(newurl)
        if (old.scheme.lower(), old.netloc.lower()) != (new.scheme.lower(), new.netloc.lower()):
            raise urllib.error.HTTPError(
                newurl, code, 'cross-origin redirect refused', headers, fp)
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def fetch(cache):
    """Fetch every chapter not yet in the cache. The only part that reads the
    key or calls API.Bible; a derive-only run (--no-fetch) never reaches it."""
    key = os.environ.pop('BIBLE_API_KEY', '').strip()
    # This generator never opens the repository's general-purpose .env.local,
    # which may contain unrelated personal provider credentials. On macOS it may
    # use the same dedicated API.Bible Keychain item as the release pipeline.
    if not key and sys.platform == 'darwin':
        result = subprocess.run(
            ['security', 'find-generic-password', '-a', 'release',
             '-s', 'uk.co.bibletext.apibible-release', '-w'],
            check=False, capture_output=True, text=True,
        )
        if result.returncode == 0:
            key = result.stdout.strip()
        result.stdout = ''
        result.stderr = ''
        del result

    if not key or not PID:
        missing = []
        if not key:
            missing.append('BIBLE_API_KEY (or the dedicated macOS Keychain item)')
        if not PID:
            missing.append('BIBLETEXT_PROVIDER_ID_NKJV')
        raise SystemExit('Missing ' + ' and '.join(missing))
    calls=0
    opener = urllib.request.build_opener(SameOriginRedirectHandler())
    for book,(code,n) in BOOKS.items():
        for ch in range(1,n+1):
            path=os.path.join(cache,f"{code}.{ch}.json")
            if os.path.exists(path) and os.path.getsize(path)>200: continue   # never re-fetch
            url=(f"https://api.scripture.api.bible/v1/bibles/{PID}/chapters/{code}.{ch}"
                 "?content-type=html&include-notes=false&include-verse-spans=true")
            try:
                with opener.open(urllib.request.Request(url,headers={'api-key':key}),timeout=45) as r:
                    raw=r.read()
                open(path,'wb').write(raw); calls+=1
            except Exception as e:
                print(f"  {code}.{ch}: {type(e).__name__} {e}",file=sys.stderr)
                time.sleep(2)
    print(f"API calls made this run: {calls}; cached chapters: {len(os.listdir(cache))}",file=sys.stderr)

# ---------------------------------------------------------------- derive
# THE HALF THAT WAS MISSING. Until 20 Aug 2026 this script ended at the fetch,
# while red_letter_nkjv_data.go claimed to be generated by it — the extraction
# that actually produced the table was never committed, and it carried a bug
# this rewrite exists to fix: the NKJV italicises supplied words, API.Bible
# nests those as <span class="it"> INSIDE <span class="wj">, and the lost
# extractor stopped at the first nested element. Every quotation containing an
# italicised word was truncated at that word: Matthew 11:30 kept "For My yoke"
# of "For My yoke *is* easy and My burden is light."; Matthew 27:11 kept one
# rune of a 144-rune verse. Measured against the shipped table: 411 of 2,054
# verses had red covering under half the verse.
#
# THE RULE, stated once: every character of TEXT that lies inside a <span
# class="wj"> belongs to Christ's words, whatever elements it is nested in —
# except a verse NUMBER (class "v"), which is display chrome the app's verse
# text does not contain. Contiguity in document order defines a span; a change
# of verse or a new wj block starts a new one.

from html.parser import HTMLParser

# Paragraph classes whose text the app's decoder keeps out of every verse
# (apiBibleSkipPara, and "d", the psalm title): not counted in a run's place.
NON_VERSE_BLOCK = {"qa", "cl", "cd", "mr", "sr", "r", "sp", "s", "s1", "s2", "s3", "s4",
                   "ms", "ms1", "ms2", "ms3", "d"}

class WJExtract(HTMLParser):
    """Per verse, the contiguous red runs, as text fragments in document order,
    and what comes before each: the verse's non-space characters ahead of it,
    lowercased as rune_map gives them. Their count is where the run begins."""
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.stack = []          # class-list per open span, innermost last
        self.blocks = []         # class-list per open element of any kind, in step
        self.verse = None        # current verse id, e.g. "MAT.11.30"
        self.runs = {}           # verse id -> [ [fragments...], ... ]
        self.befores = {}        # verse id -> [ the characters before each run, ... ]
        self.read = {}           # verse id -> [ non-space characters read so far, ... ]
        self._open = {}          # verse id -> True while its current run continues

    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        cls = set((a.get("class") or "").split())
        self.blocks.append(cls)
        if tag != "span":
            self.stack.append(set()); return
        self.stack.append(cls)
        if "verse-span" in cls and a.get("data-verse-id"):
            self.verse = a["data-verse-id"]

    def handle_endtag(self, tag):
        if self.blocks:
            self.blocks.pop()
        if self.stack:
            cls = self.stack.pop()
            # closing a wj ends every verse's current run: red text after this
            # (a later wj) is a NEW quotation, not a continuation.
            if "wj" in cls:
                self._open.clear()

    def handle_data(self, data):
        in_wj = any("wj" in c for c in self.stack)
        in_num = any("v" in c for c in self.stack)
        in_head = any("s" in c for c in self.stack)
        before = "".join(self.read.get(self.verse, ()))
        if (self.verse is not None and not in_num and not in_head
                and not any(c & NON_VERSE_BLOCK for c in self.blocks)):
            self.read.setdefault(self.verse, []).append(
                "".join(ch.lower() for ch in data if not ch.isspace()))
        if not in_wj or in_num or in_head or self.verse is None:
            return
        if not data.strip():
            # bare whitespace joins an open run but never opens one
            if self._open.get(self.verse):
                self.runs[self.verse][-1].append(data)
            return
        if not self._open.get(self.verse):
            self.runs.setdefault(self.verse, []).append([])
            self.befores.setdefault(self.verse, []).append(before)
            # a new run for THIS verse closes every other verse's run: the
            # quotation moved on, and fragments of another verse can no longer
            # be contiguous with their old run.
            self._open = {self.verse: True}
        self.runs[self.verse][-1].append(data)

def rune_map(s):
    """chars-with-whitespace-stripped, casefolded -> rune index in the original.

    Case-insensitive because the app's decoder renders the NKJV's small-caps
    divine name as "LORD" while the HTML carries "Lord" inside <span class="sc">
    — same characters, different case, and it cost 14 verses on the first run.
    Lowercasing is done per character so lengths cannot drift and every stripped
    character keeps its original rune index.
    """
    stripped, idx = [], []
    for i, ch in enumerate(s):
        if not ch.isspace():
            stripped.append(ch.lower()); idx.append(i)
    return "".join(stripped), idx

def inside_word(text, index, edges=()):
    """True when index falls between two letters or digits, unless it is one
    of edges: the places where the app's verse anchors a note, each the edge
    of a word even where the source runs two words together there."""
    return (index not in edges and 0 < index < len(text)
            and text[index - 1].isalnum() and text[index].isalnum())

def place(run_text, verse_text, before, cursor_nospace, allow_relocated=False, edges=()):
    """Place run_text in verse_text, ignoring ALL whitespace differences, after
    before: the verse's non-space characters ahead of the run in the markup
    (WJExtract), whose count is the run's place. edges are the rune offsets
    where the app's verse anchors a note. Returns ((start_rune, end_rune,
    new_cursor, relocated), None), or (None, why); why names counts only,
    never the text.

    When the app's verse has other characters before the run, or the run's
    characters are not at its place, at or after the cursor, the app's text
    differs from the HTML before the run, and the run is refused, unless
    allow_relocated is set and its characters occur once in the whole verse,
    after the run before it: with no copy, or two or more, nothing says which
    the publisher marked. A placement with either end inside a word, at a
    place that is not one of edges, is refused."""
    needle = "".join(ch.lower() for ch in run_text if not ch.isspace())
    if not needle:
        return None, "it has no characters"
    hay, idx = rune_map(verse_text)
    at, relocated = len(before), False
    if at < cursor_nospace or hay[:at] != before or not hay.startswith(needle, at):
        why = f"the app's text before it differs from the markup's, or its characters are not at its place ({at})"
        relocated = True
        copies = [i for i in range(len(hay)) if hay.startswith(needle, i)]
        if len(copies) != 1:
            return None, f"{why}, and the verse holds {len(copies)} copies of them"
        if copies[0] < cursor_nospace:
            return None, f"{why}, and their one copy lies before the run before it"
        if not allow_relocated:
            return None, f"{why}; their one copy is at {copies[0]}, taken only with --allow-relocated"
        at = copies[0]
    a, b = idx[at], idx[at + len(needle) - 1] + 1
    if inside_word(verse_text, a, edges) or inside_word(verse_text, b, edges):
        return None, f"it would begin or end inside a word at ({a}, {b})"
    return (a, b, at + len(needle), relocated), None

def fnv1a64(text):
    value = 0xCBF29CE484222325
    for byte in text.encode('utf-8'):
        value ^= byte
        value = (value * 0x100000001B3) & 0xFFFFFFFFFFFFFFFF
    return value

def derive(cache, app_cache=None, out_path=None, allow_relocated=False):
    import glob
    # The cache name carries the decoder's EPOCH — bibletext-nkjv-v7.json — so
    # a table can only ever be built against a text some decoder really
    # produced. Globbing the bare name found nothing once epochs arrived, and a
    # generator that silently finds no text is worse than one that stops.
    patterns = () if app_cache else (
        os.path.expanduser("~/Library/Caches/bibletext/bibletext-nkjv*.json"),
        os.path.expanduser("~/Library/Developer/CoreSimulator/Devices/*/data/Containers/Data/Application/*/Library/Caches/bibletext/bibletext-nkjv*.json"),
    )
    for pat in patterns:
        hits = sorted(glob.glob(pat), key=os.path.getmtime, reverse=True)
        if hits:
            app_cache = hits[0]; break
    if not app_cache:
        sys.exit("no bibletext-nkjv.json found — open the NKJV in the app or a simulator once")
    print(f"app text: {app_cache}", file=sys.stderr)

    # Every chapter must be there. One the fetch could not get would otherwise
    # leave its verses out of the table, which reads as black letter, and the
    # run would still report success.
    missing = [f"{code}.{ch}" for code, n in BOOKS.values() for ch in range(1, n + 1)
               if not os.path.exists(os.path.join(cache, f"{code}.{ch}.json"))]
    if missing:
        print(f"FAILED: {len(missing)} chapters are not in {cache}: {', '.join(missing[:20])}",
              file=sys.stderr)
        sys.exit(1)

    data = json.load(open(app_cache))["data"]["Verses"]
    def app_verse(book, ch, v):
        """The app's verse text and the rune offsets where it anchors a note
        (Footnote.Anchor), or (None, None)."""
        for rec in data.get(book, {}).get(str(ch), []):
            if rec["Verse"] == v:
                return rec["Text"], frozenset(f["anchor"] for f in rec.get("footnotes") or ())
        return None, None

    code_to_book = {code: book for book, (code, _) in BOOKS.items()}
    spans, lengths, hashes, failed, relocated = {}, {}, {}, [], []
    for book, (code, n) in BOOKS.items():
        for ch in range(1, n + 1):
            path = os.path.join(cache, f"{code}.{ch}.json")
            html = json.load(open(path))["data"]["content"]
            p = WJExtract(); p.feed(html)
            for vid, runs in p.runs.items():
                vcode, vch, vv = vid.split(".")
                vbook = code_to_book[vcode]
                text, edges = app_verse(vbook, int(vch), int(vv))
                key = f"{vbook} {int(vch)}:{int(vv)}"
                if text is None:
                    failed.append((key, "no app text")); continue
                cursor, out = 0, []
                for number, (frags, before) in enumerate(zip(runs, p.befores[vid]), 1):
                    # The reason names the run by number and place, never by
                    # its words: they are licensed text.
                    got, why = place("".join(frags), text, before, cursor, allow_relocated, edges)
                    if got is None:
                        failed.append((key, f"run {number}: {why}")); out = None; break
                    a, b, cursor, moved = got
                    if moved:
                        relocated.append(f"{key} run {number}")
                    # merge with the previous span when touching
                    if out and a <= out[-1][1]:
                        out[-1] = (out[-1][0], max(out[-1][1], b))
                    else:
                        out.append((a, b))
                if out:
                    spans[key] = out
                    lengths[key] = len(text)
                    hashes[key] = fnv1a64(text)

    if failed:
        print(f"FAILED to place {len(failed)} runs:", file=sys.stderr)
        for k, why in failed[:20]:
            print(f"  {k}: {why}", file=sys.stderr)
        sys.exit(1)
    print(f"{len(relocated)} runs placed by their only copy, not their place: {', '.join(relocated[:20])}",
          file=sys.stderr)

    if not out_path:
        out_path = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "red_letter_nkjv_data.go")
    with open(out_path, "w") as f:
        f.write("""package bibletext

// Code generated by scripts/gen-nkjv-redletter.py. DO NOT EDIT BY HAND.
//
// Which words of each NKJV verse are Christ's, as rune offsets into the verse
// text. These are the NKJV's OWN words-of-Jesus spans, served by
// API.Bible as <span class="wj"> and located inside the app's verse text.
//
// OFFSETS ONLY. The NKJV is licensed (© 1982 Thomas Nelson) and none of its text
// is stored here or anywhere else in this repository; an offset is a fact about
// the text, not the text.
//
// REGENERATED 2026-08-20: the first table was cut by a lost extractor that
// stopped at the NKJV's italicised supplied words (<span class="it"> nested
// inside the wj markup) — 411 of 2,054 verses had most of their red missing,
// visibly on Matthew 11:28-30. Text inside a wj now counts wherever it is
// nested; only verse-number chrome is excluded.

var nkjvRedLetterSpans = map[string][]redLetterSpan{
""")
        for key in sorted(spans):
            body = ", ".join("{%d, %d}" % s for s in spans[key])
            f.write('\t"%s": {%s},\n' % (key, body))
        f.write("}\n\n// nkjvRedLetterRunes is the rune length of each verse the spans were computed\n// against. It is checked together with the content fingerprint below.\nvar nkjvRedLetterRunes = map[string]int{\n")
        for key in sorted(lengths):
            f.write('\t"%s": %d,\n' % (key, lengths[key]))
        f.write("}\n\n// nkjvRedLetterHashes binds the offsets to the exact runtime text revision.\nvar nkjvRedLetterHashes = map[string]uint64{\n")
        for key in sorted(hashes):
            f.write('\t"%s": 0x%016x,\n' % (key, hashes[key]))
        f.write("}\n")
    subprocess.run(["gofmt", "-w", out_path], check=True)
    print(f"wrote {out_path}: {len(spans)} verses", file=sys.stderr)

def main():
    ap = argparse.ArgumentParser(description="Build red_letter_nkjv_data.go from API.Bible's NKJV markup.")
    ap.add_argument("--no-fetch", action="store_true",
                    help="derive from the cached chapters alone: reads no key and makes no API call")
    ap.add_argument("--html-cache", default=CACHE, help="the directory of cached chapter responses")
    ap.add_argument("--app-cache", help="the app's NKJV cache file (default: the newest one found)")
    ap.add_argument("--out", help="where to write the table (default: red_letter_nkjv_data.go in the repository)")
    ap.add_argument("--allow-relocated", action="store_true",
                    help="place a run whose verse differs before it by the one copy of its characters, and list it")
    a = ap.parse_args()
    if a.no_fetch:
        # Not read: taken out of the environment so that nothing this starts
        # inherits it.
        os.environ.pop('BIBLE_API_KEY', None)
    else:
        fetch(a.html_cache)
    derive(a.html_cache, a.app_cache, a.out, a.allow_relocated)

if __name__ == "__main__":
    main()
