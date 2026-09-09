"""Unit tests for the torch-free half of the alignment pipeline. Run by CI's
repository-hygiene step under the runner's python3:

    python3 -m unittest discover -s scripts/audio-align -p 'test_*.py'

Only extract_transcript, timing_rows and make_timings_asset are imported here;
align_chapter and batch_align import torch and stay out. Every check carries a
control that proves it can fail."""
import json
import os
import tempfile
import unittest

import extract_transcript
import make_timings_asset
import timing_rows


def _doc(subtitle):
    """One chapter of a helloao complete.json shape, with an optional subtitle."""
    content = [{"type": "heading", "content": ["x"]}]
    if subtitle is not None:
        content.append({"type": "hebrew_subtitle", "content": subtitle})
    content += [
        {"type": "line_break"},
        {"type": "verse", "number": 1, "content": [{"text": "LORD, how"}]},
        {"type": "verse", "number": 2, "content": ["Many", {"noteId": 1}, "say."]},
    ]
    return {"books": [{"order": 19, "chapters": [{"chapter": {"number": 3, "content": content}}]}]}


class ExtractTests(unittest.TestCase):
    def test_extract_emits_title_as_verse_zero(self):
        got, n_ch, n_v, n_t = extract_transcript.extract(_doc(["A Psalm of David,", {"noteId": 1}, "when he fled."]))
        self.assertEqual(got, {"Psalms": {"3": [
            {"v": 0, "text": "A Psalm of David, when he fled."},
            {"v": 1, "text": "LORD, how"},
            {"v": 2, "text": "Many say."},
        ]}})
        self.assertEqual((n_ch, n_v, n_t), (1, 3, 1))

    def test_acrostic_letter_is_not_a_title(self):
        # CONTROL: the same chapter under ALEPH has no verse-0 row anywhere.
        got, _, _, n_t = extract_transcript.extract(_doc(["ALEPH"]))
        self.assertEqual([r["v"] for r in got["Psalms"]["3"]], [1, 2])
        self.assertEqual(n_t, 0)
        got, _, _, n_t = extract_transcript.extract(_doc(None))
        self.assertEqual([r["v"] for r in got["Psalms"]["3"]], [1, 2])
        self.assertEqual(n_t, 0)

    def test_acrostic_rule_is_exact(self):
        for text, want in [("ALEPH", True), ("  BETH ", True), ("Aleph", False), ("ALEPH.", False),
                           ("", False), ("   ", False), ("A Psalm of David.", False), ("A PSALM", False)]:
            self.assertEqual(extract_transcript.acrostic_letter_label(text), want, text)


class RollupTests(unittest.TestCase):
    def test_rollup_keeps_verse_zero_first(self):
        rows = timing_rows.rollup_verses([(3.4, 3.9), (4.0, 6.9), (7.2, 8.0)], [0, 0, 1])
        # len must be 2: a truthiness slip on the verse number yields 1.
        self.assertEqual(rows, [{"v": 0, "start": 3.4, "end": 6.9}, {"v": 1, "start": 7.2, "end": 8.0}])

    def test_rollup_without_a_title(self):
        # CONTROL: no verse-0 tag, no verse-0 row, verse 1 first.
        rows = timing_rows.rollup_verses([(3.4, 3.9), (4.0, 6.9), (7.2, 8.0)], [1, 1, 2])
        self.assertEqual([r["v"] for r in rows], [1, 2])
        self.assertEqual(rows[0], {"v": 1, "start": 3.4, "end": 6.9})

    def test_asset_rows_round_to_a_tenth(self):
        self.assertEqual(timing_rows.asset_rows([{"v": 0, "start": 3.361, "end": 6.94}]), [[0, 3.4, 6.9]])

    def test_title_pace(self):
        titled = [{"v": 0, "start": 1.0, "end": 3.0}, {"v": 1, "start": 3.5, "end": 9.0}]
        self.assertTrue(timing_rows.title_pace_ok(titled, 8))    # 2.0 s for 8 words: 0.25 s/word
        self.assertFalse(timing_rows.title_pace_ok(titled, 20))  # CONTROL: 0.10 s/word is a sliver
        self.assertTrue(timing_rows.title_pace_ok(titled[1:], 20))  # no title row: nothing to judge
        self.assertTrue(timing_rows.title_pace_ok([], 20))


class CompactTests(unittest.TestCase):
    def test_compact_counts_titled_chapters(self):
        with tempfile.TemporaryDirectory() as d:
            files = []
            for ch, verses in [(1, [{"v": 1, "start": 0.51, "end": 2.0}]),
                               (3, [{"v": 0, "start": 1.26, "end": 6.28}, {"v": 1, "start": 8.52, "end": 13.9}])]:
                fp = os.path.join(d, f"Psalms_{ch}.json")
                json.dump({"book": "Psalms", "chapter": ch, "verses": verses, "ok": True}, open(fp, "w"))
                files.append(fp)
            books, n_ch, n_v, n_t, suspect = make_timings_asset.compact(files)
            self.assertEqual((n_ch, n_v, n_t, suspect), (2, 3, 1, []))
            self.assertEqual(books["Psalms"]["3"], [[0, 1.3, 6.3], [1, 8.5, 13.9]])
            self.assertEqual(books["Psalms"]["1"], [[1, 0.5, 2.0]])
            # CONTROL: a suspect chapter is reported, and dropped only on request.
            json.dump({"book": "Psalms", "chapter": 1, "verses": [{"v": 1, "start": 0, "end": 1}], "ok": False},
                      open(files[0], "w"))
            _, n_ch, _, _, suspect = make_timings_asset.compact(files)
            self.assertEqual((n_ch, suspect), (2, ["Psalms 1"]))
            _, n_ch, _, _, _ = make_timings_asset.compact(files, skip_suspect=True)
            self.assertEqual(n_ch, 1)


if __name__ == "__main__":
    unittest.main()
