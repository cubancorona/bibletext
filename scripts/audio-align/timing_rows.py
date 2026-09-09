"""Pure helpers for the alignment pipeline, kept torch-free so they can be
unit-tested under the system python3 (test_timing_rows.py, run by CI's hygiene
step). align_chapter.py imports torch at module level, and its rollup is exactly
where a truthiness slip (`if not vn`) would silently drop the Psalm title's
verse-0 row — so the rollup lives here, where a test can reach it."""


def rollup_verses(word_times, word_verse):
    """Roll word spans up to rows: a row's span is [first word start, last word
    end], keyed by the verse each word was tagged with (0 = the Psalm's title).
    Rows come out sorted by verse number, so the title row leads."""
    spans = {}
    for (start, end), vn in zip(word_times, word_verse):
        if vn not in spans:
            spans[vn] = [start, end]
        else:
            spans[vn][1] = end
    return [{"v": vn, "start": st, "end": en} for vn, (st, en) in sorted(spans.items())]


def asset_rows(verses):
    """The compact asset's rows for one chapter: [verse, start, end] at 0.1 s."""
    return [[v["v"], round(v["start"], 1), round(v["end"], 1)] for v in verses]


def title_pace_ok(verses, title_words, min_secs_per_word=0.15):
    """A narrated title takes time. A verse-0 row squeezed under 0.15 s per word
    is a narrator who skipped the title and an aligner that crammed its words
    into a sliver of whatever came first; such a chapter is suspect. A chapter
    without a title row is trivially fine."""
    if not verses or verses[0]["v"] != 0:
        return True
    return (verses[0]["end"] - verses[0]["start"]) >= min_secs_per_word * title_words
