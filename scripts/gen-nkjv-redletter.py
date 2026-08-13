#!/usr/bin/env python3
"""Build the NKJV's red-letter spans from the publisher's own markup.

    python3 scripts/gen-nkjv-redletter.py          # fetch (cached), then derive

Unlike the BSB — which publishes no words-of-Jesus markup at all, so its table
had to be derived (see scripts/gen-bsb-redletter.py) — the NKJV's own spans are
served by API.Bible as <span class="wj">. This is the publisher's editorial
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
"""
CACHE=os.path.expanduser('~/.cache/bibletext-nkjv-html')
KEY=PID=None
for line in open(os.path.expanduser('~/Dev/bibletext/.env.local')):
    if line.startswith('BIBLE_API_KEY='): KEY=line.split('=',1)[1].strip().strip('"\'')
    if line.startswith('BIBLETEXT_PROVIDER_ID_NKJV='): PID=line.split('=',1)[1].strip().strip('"\'')
BOOKS={'Matthew':('MAT',28),'Mark':('MRK',16),'Luke':('LUK',24),'John':('JHN',21),'Acts':('ACT',28),
       '1 Corinthians':('1CO',16),'2 Corinthians':('2CO',13),'1 Timothy':('1TI',6),'Revelation':('REV',22)}
calls=0
for book,(code,n) in BOOKS.items():
    for ch in range(1,n+1):
        path=os.path.join(CACHE,f"{code}.{ch}.json")
        if os.path.exists(path) and os.path.getsize(path)>200: continue   # never re-fetch
        url=(f"https://api.scripture.api.bible/v1/bibles/{PID}/chapters/{code}.{ch}"
             "?content-type=html&include-notes=false&include-verse-spans=true")
        try:
            with urllib.request.urlopen(urllib.request.Request(url,headers={'api-key':KEY}),timeout=45) as r:
                raw=r.read()
            open(path,'wb').write(raw); calls+=1
        except Exception as e:
            print(f"  {code}.{ch}: {type(e).__name__} {e}",file=sys.stderr)
            time.sleep(2)
print(f"API calls made this run: {calls}; cached chapters: {len(os.listdir(CACHE))}",file=sys.stderr)
