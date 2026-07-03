#!/usr/bin/env python3
"""Forced-align one chapter's recorded narration to its known verse text, producing
verse-level (and word-level) timings. Uses torchaudio's MMS_FA aligner: we already
KNOW every word, so this is alignment (find where each known word is in time), not
transcription.

    align_chapter.py <Book> <chapter> [--json out.json] [--srt out.srt] [--words]

Env: BIBLETEXT_AUDIO_DATA (default ~/Dev/bibletext-audiodata) must hold
     manifest.tsv, BSB_transcript.json, and bsb/*.mp3.
"""
import argparse
import json
import os
import sys

import torch
import torchaudio

DATA = os.environ.get("BIBLETEXT_AUDIO_DATA", os.path.expanduser("~/Dev/bibletext-audiodata"))
# Model forward runs on this device (MPS gives a nice speedup on Apple Silicon);
# the CTC forced_align op itself is CPU-only, so emission is moved to CPU before it.
DEVICE = torch.device(os.environ.get("BIBLETEXT_ALIGN_DEVICE", "cpu"))
# Cap intra-op threads so a batch can't oversubscribe every core and starve the UI.
torch.set_num_threads(int(os.environ.get("BIBLETEXT_ALIGN_THREADS", "4")))


def cleanup():
    """Release per-chapter memory between iterations so a long batch stays flat —
    critical on MPS, where freed tensors otherwise linger as wired unified memory
    (the cause of the batch freeze); harmless and cheap on CPU."""
    import gc
    gc.collect()
    if DEVICE.type == "mps":
        torch.mps.empty_cache()

_bundle = torchaudio.pipelines.MMS_FA
_model = None
_tokenizer = None
_aligner = None
_keep = None
SR = _bundle.sample_rate


def _lazy_model():
    global _model, _tokenizer, _aligner, _keep
    if _model is None:
        _model = _bundle.get_model().to(DEVICE).eval()
        _tokenizer = _bundle.get_tokenizer()
        _aligner = _bundle.get_aligner()
        d = _bundle.get_dict()
        _keep = {c for c in d if len(c) == 1 and c.isalpha()}
        if "'" in d:
            _keep.add("'")
    return _model, _tokenizer, _aligner


def normalize(word):
    return "".join(c for c in word.lower() if c in _keep)


def load_manifest():
    m = {}
    for line in open(os.path.join(DATA, "manifest.tsv")):
        ver, book, ch, url = line.rstrip("\n").split("\t")
        fn = url.rsplit("/", 1)[-1]
        m[(ver, book, int(ch))] = os.path.join(DATA, ver, fn)
    return m


def load_audio(path):
    wav, sr = torchaudio.load(path)
    if wav.shape[0] > 1:
        wav = wav.mean(0, keepdim=True)
    if sr != SR:
        wav = torchaudio.functional.resample(wav, sr, SR)
    return wav


def align_chapter(book, chapter, transcript, manifest, ver="bsb"):
    model, tokenizer, aligner = _lazy_model()
    verses = transcript[book][str(chapter)]

    words, word_verse = [], []
    for v in verses:
        for w in v["text"].split():
            nw = normalize(w)
            if nw:  # NOTE: words that normalize empty (bare numerals, em-dashes) are dropped;
                words.append(nw)  # rare in narrative, but numbers need spelling-out before batch
                word_verse.append(v["v"])
    if not words:
        raise ValueError(f"{book} {chapter}: no alignable words")

    path = manifest[(ver, book, chapter)]
    wav = load_audio(path)
    total_s = wav.shape[1] / SR

    with torch.inference_mode():
        emission, _ = model(wav.to(DEVICE))
        # forced_align (inside the aligner) has no MPS/CUDA kernel — run it on CPU.
        token_spans = aligner(emission[0].to("cpu"), tokenizer(words))

    num_frames = emission.size(1)
    ratio = wav.shape[1] / num_frames  # samples per emission frame

    def secs(i):
        return round(ratio * i / SR, 3)

    word_times = []
    for spans in token_spans:
        word_times.append((secs(spans[0].start), secs(spans[-1].end)))

    # roll up to verses: verse span = [first word start, last word end]
    verse_times = {}
    for (s, e), vn in zip(word_times, word_verse):
        if vn not in verse_times:
            verse_times[vn] = [s, e]
        else:
            verse_times[vn][1] = e

    return {
        "book": book, "chapter": chapter, "version": ver,
        "audio": os.path.basename(path), "duration": round(total_s, 2),
        "n_words": len(words),
        "verses": [{"v": vn, "start": st, "end": en} for vn, (st, en) in sorted(verse_times.items())],
        "_word_times": word_times, "_word_verse": word_verse,
    }


def diagnostics(res):
    vs = res["verses"]
    starts = [v["start"] for v in vs]
    monotonic = all(starts[i] <= starts[i + 1] for i in range(len(starts) - 1))
    covers = vs[-1]["end"] / res["duration"] if vs else 0
    gap0 = vs[0]["start"] if vs else 0
    print(f"  {res['book']} {res['chapter']}: {len(vs)} verses, {res['n_words']} words, "
          f"audio {res['duration']}s")
    print(f"  monotonic={monotonic}  first_verse_starts@{gap0}s  last_verse_ends@{vs[-1]['end']}s "
          f"({covers*100:.0f}% of audio)")
    # show a few verses
    for v in vs[:3] + (vs[-2:] if len(vs) > 5 else []):
        print(f"    v{v['v']:>3}  {v['start']:>7.2f} - {v['end']:>7.2f}")


def to_srt(res):
    def ts(x):
        h = int(x // 3600); m = int((x % 3600) // 60); s = x % 60
        return f"{h:02d}:{m:02d}:{s:06.3f}".replace(".", ",")
    out = []
    for i, v in enumerate(res["verses"], 1):
        out.append(f"{i}\n{ts(v['start'])} --> {ts(v['end'])}\nv{v['v']}\n")
    return "\n".join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book")
    ap.add_argument("chapter", type=int)
    ap.add_argument("--json")
    ap.add_argument("--srt")
    ap.add_argument("--words", action="store_true")
    a = ap.parse_args()

    transcript = json.load(open(os.path.join(DATA, "BSB_transcript.json")))
    manifest = load_manifest()
    res = align_chapter(a.book, a.chapter, transcript, manifest)
    diagnostics(res)
    if a.json:
        slim = {k: v for k, v in res.items() if not k.startswith("_")}
        if a.words:
            slim["words"] = [{"start": s, "end": e, "v": vn}
                             for (s, e), vn in zip(res["_word_times"], res["_word_verse"])]
        json.dump(slim, open(a.json, "w"))
        print(f"  wrote {a.json}")
    if a.srt:
        open(a.srt, "w").write(to_srt(res))
        print(f"  wrote {a.srt}")


if __name__ == "__main__":
    main()
