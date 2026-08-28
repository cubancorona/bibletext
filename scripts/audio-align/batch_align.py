#!/usr/bin/env python3
"""Batch forced-alignment → one compact verse-timing JSON per chapter. Resumable,
logs failures, never lets one bad chapter abort the run. Model forward on
BIBLETEXT_ALIGN_DEVICE (use cpu — MPS leaks unified memory and freezes the machine).

Three corpora:
  BSB (openbible/Hays), streamed by the app:
    batch_align.py                       # defaults: BSB_transcript.json, bsb, timings/
  WEB (AudioTreasure/Williams), which we'd HOST (kept separate):
    batch_align.py --audio williams --transcript WEB_transcript.json \
                   --version web --out-dir timings-web-williams
  WEBBE (eBible synthetic), the WEB-Catholic's Greek books only — fetch with
  fetch-webbe.sh first, and build the transcript from the app's own WEBC text so
  the timings index what the reader actually sees:
    batch_align.py --audio webbe --transcript WEBBE_transcript.json \
                   --version webc --out-dir timings-webbe

    [--shard i/N] [--limit N] [--book "John"]
"""
import argparse
import json
import os
import re
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import align_chapter as A  # noqa: E402
from extract_transcript import APP_BOOKS  # noqa: E402

DATA = A.DATA
FAIL_LOG = os.path.join(DATA, "align_failures.log")


def williams_manifest():
    """WEB (David Williams / AudioTreasure) per-chapter files → {(ver,book,ch): path}.
    Filenames: <booknum>_<Abbr>[_<ch>|<ch>].mp3, or <booknum>_<Abbr>.mp3 for one-chapter
    books. Rule: leading digits = 1..66 canonical book; trailing digits of the rest =
    chapter (default 1 if none, e.g. 57_Philemon.mp3)."""
    man = {}
    for p in __import__("glob").glob(os.path.join(DATA, "web-williams", "*.mp3")):
        b = os.path.basename(p)
        mbn = re.match(r"(\d+)_", b)
        if not mbn:
            continue
        bn = int(mbn.group(1))
        if not (1 <= bn <= 66):
            continue
        rest = b[mbn.end():].rsplit(".", 1)[0]
        mch = re.search(r"(\d+)$", rest)
        ch = int(mch.group(1)) if mch else 1
        man[("web", APP_BOOKS[bn - 1], ch)] = p
    return man


# eBible's book codes for the WEBBE deuterocanon → the app's own book names. The
# Greek Daniel is DAG in the audio filenames (the same text eBible's verse-per-line
# export calls DNG), and the Greek Esther is ESG — a different book from the Hebrew
# EST, which is why the WEB-Catholic cannot use the Williams Esther recording.
WEBBE_BOOKS = {
    "TOB": "Tobit", "JDT": "Judith", "ESG": "Esther", "WIS": "Wisdom",
    "SIR": "Sirach", "BAR": "Baruch", "1MA": "1 Maccabees", "2MA": "2 Maccabees",
    "DAG": "Daniel",
}


def webbe_manifest():
    """WEBBE (eBible synthetic narration) per-chapter files → {(ver,book,ch): path}.
    Filenames keep eBible's own scheme: eng-webbe_<order>_<CODE>_<chapter>.mp3, e.g.
    eng-webbe_046_SIR_07.mp3. Only the books in WEBBE_BOOKS are mirrored — the
    WEB-Catholic's protocanon keeps the human Williams narration."""
    man = {}
    for p in __import__("glob").glob(os.path.join(DATA, "webbe", "mp3", "*.mp3")):
        m = re.match(r"eng-webbe_\d+_([A-Z0-9]{3})_(\d+)\.mp3$", os.path.basename(p))
        if not m:
            continue
        book = WEBBE_BOOKS.get(m.group(1))
        if book:
            man[("webc", book, int(m.group(2)))] = p
    return man


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--book")
    ap.add_argument("--shard", help="i/N — disjoint parallel worker")
    ap.add_argument("--transcript", default="BSB_transcript.json")
    ap.add_argument("--out-dir", default="timings")
    ap.add_argument("--version", default="bsb")
    ap.add_argument("--audio", default="manifest", choices=["manifest", "williams", "webbe"])
    a = ap.parse_args()

    out = os.path.join(DATA, a.out_dir)
    os.makedirs(out, exist_ok=True)
    outp = lambda book, ch: os.path.join(out, f"{book}_{ch}.json")  # noqa: E731

    transcript = json.load(open(os.path.join(DATA, a.transcript)))
    manifest = {"manifest": A.load_manifest, "williams": williams_manifest, "webbe": webbe_manifest}[a.audio]()
    ver = a.version

    jobs = []
    for book, chapters in transcript.items():
        if a.book and book != a.book:
            continue
        for ch in sorted(map(int, chapters.keys())):
            p = manifest.get((ver, book, ch))
            if p and os.path.exists(p):
                jobs.append((book, ch))

    tag = ""
    if a.shard:
        si, sn = (int(x) for x in a.shard.split("/"))
        jobs = [j for idx, j in enumerate(jobs) if idx % sn == si]
        tag = f"[{ver} shard {si}/{sn}] "
    todo = [(b, c) for (b, c) in jobs if not os.path.exists(outp(b, c))]
    print(f"{tag}device={A.DEVICE}  my_chapters={len(jobs)}  done={len(jobs)-len(todo)}  to_align={len(todo)}", flush=True)

    A._lazy_model()
    done = fails = 0
    t0 = time.time()
    for book, ch in todo:
        if a.limit and done >= a.limit:
            break
        try:
            res = A.align_chapter(book, ch, transcript, manifest, ver=ver)
            slim = {k: v for k, v in res.items() if not k.startswith("_")}
            vs = slim["verses"]
            mono = all(vs[j]["start"] <= vs[j + 1]["start"] for j in range(len(vs) - 1))
            cover = vs[-1]["end"] / slim["duration"] if vs else 0
            slim["ok"] = bool(mono and cover > 0.85)
            json.dump(slim, open(outp(book, ch), "w"))
            if not slim["ok"]:
                open(FAIL_LOG, "a").write(f"SUSPECT {ver} {book} {ch}: monotonic={mono} coverage={cover:.2f}\n")
        except Exception as e:  # noqa: BLE001
            fails += 1
            open(FAIL_LOG, "a").write(f"ERROR {ver} {book} {ch}: {e!r}\n")
            print(f"  ERROR {book} {ch}: {e!r}", flush=True)
            continue
        done += 1
        A.cleanup()
        if done % 25 == 0 or done == len(todo):
            el = time.time() - t0
            print(f"  {tag}{done}/{len(todo)}  {el/done:.1f}s/ch  elapsed {el/60:.1f}m  eta {(el/done)*(len(todo)-done)/60:.1f}m  fails={fails}", flush=True)

    print(f"DONE {tag}: aligned {done}, failures {fails}, timings in {out}", flush=True)


if __name__ == "__main__":
    main()
