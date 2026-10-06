"""What appstore/push-review-notes.py refuses in the notes it would send.

No store is contacted: these call ``load_notes()`` on files written into a
temporary directory, with the pinned version replaced on the loaded module.
ci.yml's ``unittest discover -s appstore -p 'test_*.py'`` runs this file
unchanged.
"""

from __future__ import annotations

import importlib.util
import os
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
TARGET = "9.9.9"


def load_module():
    spec = importlib.util.spec_from_file_location(
        "push_review_notes_under_test", os.path.join(HERE, "push-review-notes.py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.TARGET_VERSION = TARGET
    return module


class LoadNotes(unittest.TestCase):
    def setUp(self):
        self.m = load_module()
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.dir = scratch.name

    def notes(self, text):
        path = os.path.join(self.dir, "review-notes.txt")
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def refused(self, text):
        with self.assertRaises(SystemExit) as caught:
            self.m.load_notes(self.notes(text))
        return str(caught.exception.code)

    def test_the_last_release_body_under_a_bumped_heading_is_refused(self):
        # The heading passes on its own; only the body names the old release.
        message = self.refused("VERSION 9.9.9 (macOS)\n\nWHAT IS NEW IN 9.9.8\nA change.\n\n"
                               "HOW TO EXERCISE 9.9.8\nOpen it.\n")
        self.assertIn("names 9.9.8;", message)
        self.assertIn("name no release but 9.9.9", message)

    def test_a_comparison_with_an_earlier_release_is_refused(self):
        message = self.refused("VERSION 9.9.9\n\nWHAT IS NEW IN 9.9.9\n"
                               "The list includes \"A 1:2\" (9.9.8: \"A 1:1\") and \"B 2:3\" "
                               "(9.9.7: \"B 2:2\").\n")
        self.assertIn("names 9.9.8, 9.9.7;", message)

    def test_any_other_x_y_z_is_refused_with_no_allow_list(self):
        self.assertIn("names 14.0.1;", self.refused("VERSION 9.9.9\n\nOn macOS 14.0.1, open it.\n"))

    def test_notes_that_name_only_their_release_are_returned(self):
        text = ("VERSION 9.9.9\n\nWHAT IS NEW IN 9.9.9\nGenesis 1:1 and Psalm 119:1 open in "
                "2.5 seconds.\n")
        self.assertEqual(self.m.load_notes(self.notes(text + "\n")), text.rstrip("\n"))

    def test_the_heading_must_still_name_the_release(self):
        self.assertIn("must open with VERSION 9.9.9", self.refused("VERSION 9.9.8\n\nA change.\n"))

    def test_the_pattern_is_the_go_tests(self):
        # appstore_review_notes_test.go holds the tracked notes to the same
        # x.y.z, so the two cannot disagree about what a version is.
        with open(os.path.join(os.path.dirname(HERE), "appstore_review_notes_test.go"),
                  encoding="utf-8") as f:
            go = f.read()
        self.assertIn("regexp.MustCompile(`" + self.m.VERSION_PATTERN.pattern + "`)", go)


if __name__ == "__main__":
    unittest.main()
