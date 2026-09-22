#!/usr/bin/env python3
"""Fresh census of the NKJV's run-together class, from API.Bible's raw JSON.

    BIBLE_API_KEY="$(security find-generic-password -a release -s uk.co.bibletext.apibible-release -w)" \
        scripts/nkjv-upstream-joins.py /path/outside/the/repo/joins.json

Writes every candidate to the file named (licensed fragments -- keep it OUT of
the repository) and prints only counts. docs/SOURCE_FIELDS.md, "A defect in the
licensed source", records what the counts mean and what is excluded: a note
before punctuation, a note after an opening mark, the NKJV's closed em-dash and
the known word-internal joins are all correct typography, not missing spaces.

A candidate is a note element whose nearest visible text on BOTH sides (not
crossing a paragraph or a verse boundary) ends / begins with a non-space
character. That is the shape of every one of the 259 recorded on 7 September,
and also of the handful of legitimate word-internal joins (young|est, G|OD),
which the response cannot tell apart -- so this counts candidates, and the
split is reported against the known legitimate ones separately.
"""
import json, os, sys, time, urllib.request, collections
KEY = os.environ["BIBLE_API_KEY"]
BIB = "63097d2a0a2f7db3-01"
OUT = sys.argv[1]

def get(path):
    for attempt in range(5):
        try:
            req = urllib.request.Request("https://rest.api.bible/v1" + path, headers={"api-key": KEY})
            with urllib.request.urlopen(req, timeout=60) as r:
                return json.load(r)
        except Exception as e:
            if attempt == 4: raise
            time.sleep(2 * (attempt + 1))

books = get(f"/bibles/{BIB}/books?include-chapters=true")["data"]
chapters = [c["id"] for b in books for c in b.get("chapters", []) if c["number"] != "intro"]
print(f"{len(chapters)} chapters", file=sys.stderr)

def events(node, out, in_note=False, verse=None):
    if isinstance(node, list):
        for n in node: verse = events(n, out, in_note, verse)
        return verse
    if not isinstance(node, dict): return verse
    a = node.get("attrs") or {}
    if node.get("type") == "text":
        if not in_note and node.get("text"):
            out.append(("text", node["text"], a.get("verseId") or verse))
        return verse
    name = node.get("name"); style = a.get("style")
    if name == "verse" or style == "v":
        verse = a.get("sid") or a.get("number") or verse
        out.append(("verse", verse, verse))
        return verse
    if name == "note":
        out.append(("note", style, verse))
        events(node.get("items", []), [], True, verse)   # note body is not visible text
        return verse
    if name == "para":
        out.append(("para", style, verse))
        verse = events(node.get("items", []), out, in_note, verse)
        out.append(("para", style, verse))
        return verse
    return events(node.get("items", []), out, in_note, verse)

rows = []
for i, ch in enumerate(chapters):
    d = get(f"/bibles/{BIB}/chapters/{ch}?content-type=json&include-notes=true&include-titles=true&include-verse-numbers=true&include-verse-spans=false")
    ev = []
    events(d["data"]["content"], ev)
    for k, (kind, val, verse) in enumerate(ev):
        if kind != "note": continue
        # previous visible text, not crossing para/verse
        prev = None
        for j in range(k - 1, -1, -1):
            if ev[j][0] in ("para", "verse"): break
            if ev[j][0] == "text" and ev[j][1] != "":
                prev = ev[j][1]; break
        nxt = None
        for j in range(k + 1, len(ev)):
            if ev[j][0] in ("para", "verse"): break
            if ev[j][0] == "text" and ev[j][1] != "":
                nxt = ev[j][1]; break
        if prev is None or nxt is None: continue
        if not prev[-1].isspace() and not nxt[0].isspace():
            left = prev.split()[-1] if prev.split() else prev
            right = nxt.split()[0] if nxt.split() else nxt
            rows.append({"chapter": ch, "verse": verse, "style": val, "left": left, "right": right})
    if (i + 1) % 100 == 0: print(f"  {i+1}/{len(chapters)}", file=sys.stderr)

json.dump(rows, open(OUT, "w"), indent=1)
by = collections.Counter(r["chapter"].split(".")[0] for r in rows)
verses = {(r["chapter"], r["verse"]) for r in rows}
print(f"CANDIDATES: {len(rows)} joins in {len(verses)} verses across {len(by)} books")
for b, n in by.most_common(10): print(f"  {b:4} {n}")
