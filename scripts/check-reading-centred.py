#!/usr/bin/env python3
"""Assert that a screenshot's reading column sits centred in its pane.

The defect this exists for: the app asks for a 1280x860 window. On a desktop
that cannot give one, the window comes back smaller while the content keeps
the layout it was given for the size that was asked for. The column is then
centred for a canvas nobody has, so it sits far to the right of where it
belongs -- a wide dead gutter down the left and the text crowding the right
edge. It stays that way until some resize forces a fresh layout pass.

docs/windows-store-smoke-john3.jpg is that failure, captured on a 1024x768
runner and committed without anyone noticing. Every check that looked at that
image only asked whether the app had DRAWN, and it had. Nothing asked whether
what it drew was in the right place.

    scripts/check-reading-centred.py <image> [<image> ...]
    scripts/check-reading-centred.py --expect-fail docs/windows-store-smoke-john3.jpg

The rule is deliberately about geometry and not about the app's internals, so
it works on any screenshot from any platform: find the block of text, find the
pane it sits in, and require its left and right margins to be roughly equal.

--expect-fail inverts the verdict, which is how the known-bad artifact above is
used as a fixture: if this script ever reports that image as centred, the rule
has stopped working and a passing run on a real screenshot would mean nothing.

Every run self-tests first, against synthetic images with a known answer,
because a checker that cannot fail proves nothing when it passes.
"""

from __future__ import annotations

import sys

try:
    from PIL import Image
except ImportError:  # pragma: no cover - the message is the point
    print("ERROR: this needs Pillow (python3 -m pip install Pillow)", file=sys.stderr)
    raise SystemExit(2)

# A column counts as carrying text when this share of the sampled rows have ink
# in it. Low on purpose: it is a noise floor, not a plateau detector. How much
# ink a column of body text carries varies a great deal with the capture -- a
# downscaled screenshot smears ink across columns and reads around 30%, a sharp
# one at device scale reads 8%, and the same app in a wider window reads lower
# again because the lines are longer and thinner. A threshold set near any of
# those numbers works on that capture and silently finds noise on the others.
# Measured across three real captures (a device-scale 800x600 guest, the same
# guest maximised, and the committed downscaled failure) every value from 0.02
# to 0.08 classifies all three correctly; 0.10 does not. This sits in the
# middle of that band.
INK_COLUMN_SHARE = 0.04

# How far a pixel must sit from the background to count as ink, summed over RGB.
# Generous, because these images are JPEG and the paper colour is not flat.
INK_DISTANCE = 48

# The share of the pane's width by which the two margins may differ. A centred
# column differs by nothing; the committed failure differs by about a quarter.
MAX_ASYMMETRY = 0.15

# Gaps between words, as a share of the image width, to close before looking
# for the text block. Wide enough to bridge the space between words at any
# capture scale, far narrower than the gutter this check is looking for.
GAP_CLOSE_SHARE = 0.02

# Rows outside this band are chrome: the title bar and header above, the
# taskbar below. The reading pane is what is left.
BAND_TOP, BAND_BOTTOM = 0.25, 0.92


def measure(path):
    """Return (pane_left, text_left, text_right, pane_right) in pixels, or None.

    None means the image has no text block to judge -- an app that drew nothing,
    or a crop with no reading pane in it. That is not a pass.
    """
    im = Image.open(path).convert("RGB")
    w, h = im.size
    px = im.load()
    top, bottom = int(h * BAND_TOP), int(h * BAND_BOTTOM)
    rows = list(range(top, bottom, 2))
    if not rows or w < 64:
        return None

    counts = {}
    for y in rows[::2]:
        for x in range(0, w, 2):
            c = px[x, y]
            counts[c] = counts.get(c, 0) + 1
    bg = max(counts, key=counts.get)

    def ink(c):
        return abs(c[0] - bg[0]) + abs(c[1] - bg[1]) + abs(c[2] - bg[2]) > INK_DISTANCE

    need = INK_COLUMN_SHARE * len(rows)
    dense = [sum(1 for y in rows if ink(px[x, y])) >= need for x in range(w)]

    # Close the small gaps first. Inside a block of text there are columns that
    # happen to fall in the space between words on every line, and they drop
    # below the threshold -- so a run of "dense" columns is not contiguous and
    # the widest one can be a fragment a few letters wide. How bad this is
    # depends on the capture: a downscaled screenshot smears ink across columns
    # and hides it, while a sharp one at device scale shows it badly. The rule
    # has to behave the same on both, so gaps narrower than a word are filled
    # in before any run is measured. The dead gutter this check exists to find
    # is hundreds of pixels wide and is never closed by this.
    gap = max(8, int(w * GAP_CLOSE_SHARE))
    closed = list(dense)
    x = 0
    while x < w:
        if dense[x]:
            x += 1
            continue
        end = x
        while end < w and not dense[end]:
            end += 1
        if x > 0 and end < w and (end - x) <= gap:
            for i in range(x, end):
                closed[i] = True
        x = end
    dense = closed

    # The text block is the widest run of dense columns. The icon rail is a
    # narrow run and loses; a correctly centred column and a displaced one are
    # both the widest thing on the page, which is what makes this comparable
    # between the good and bad cases.
    best = run = None
    for x in range(w):
        if dense[x]:
            run = (run[0], x) if run else (x, x)
            if best is None or (run[1] - run[0]) > (best[1] - best[0]):
                best = run
        else:
            run = None
    if best is None or (best[1] - best[0]) < w * 0.15:
        return None
    text_left, text_right = best

    # The pane's left edge: walk back from the text through the dead gutter and
    # stop at whatever is drawn to its left (the rail, or the window edge).
    pane_left = 0
    for x in range(text_left - 1, -1, -1):
        if dense[x]:
            pane_left = x + 1
            break
    # The pane's right edge is the image edge; a scrollbar is a few pixels and
    # is absorbed by MAX_ASYMMETRY.
    pane_right = w - 1
    return pane_left, text_left, text_right, pane_right


def verdict(path):
    """(centred: bool, note: str)."""
    m = measure(path)
    if m is None:
        return False, "no text block found to judge (did the app draw a reading pane?)"
    pane_left, text_left, text_right, pane_right = m
    left = text_left - pane_left
    right = pane_right - text_right
    pane = pane_right - pane_left
    if pane <= 0:
        return False, "the pane measured as zero-width"
    asym = abs(left - right) / pane
    note = (
        f"pane x={pane_left}..{pane_right} ({pane}px); text x={text_left}..{text_right}; "
        f"margins left {left}px right {right}px; asymmetry {asym * 100:.1f}% "
        f"(limit {MAX_ASYMMETRY * 100:.0f}%)"
    )
    return asym <= MAX_ASYMMETRY, note


def _synthetic(tmpdir, left_margin):
    """A fake reading pane: cream paper, a rail, and a text block at a margin."""
    w, h = 1024, 768
    im = Image.new("RGB", (w, h), (237, 233, 224))
    px = im.load()
    for x in range(0, 70):  # the icon rail
        for y in range(300, 500, 4):
            px[x, y] = (40, 40, 40)
    block = 500
    for y in range(200, 700, 4):  # text lines
        for x in range(left_margin, min(left_margin + block, w)):
            px[x, y] = (30, 25, 20)
    p = f"{tmpdir}/synthetic-{left_margin}.png"
    im.save(p)
    return p


def self_test():
    """Run the rule against images whose answer is known before trusting it."""
    import tempfile

    with tempfile.TemporaryDirectory() as tmp:
        # Centred: pane runs 70..1023 (954px), a 500px block centred leaves 227
        # either side. Deliberately offset by 20px rather than placed exactly,
        # because a real render never lands on the pixel and a fixture that is
        # mathematically perfect cannot detect a limit tightened to zero -- it
        # would score 0% asymmetry and pass any limit at all. At 20px it scores
        # about 4%: comfortably centred, and close enough to the edge of the
        # band that an over-strict limit fails here instead of in the wild.
        centred = _synthetic(tmp, 70 + (954 - 500) // 2 + 20)
        ok, note = verdict(centred)
        if not ok:
            print(f"SELF-TEST FAILED: a centred column was called off-centre\n  {note}", file=sys.stderr)
            return False
        # Displaced by the amount the real defect displaces it.
        off = _synthetic(tmp, 70 + 340)
        ok, note = verdict(off)
        if ok:
            print(f"SELF-TEST FAILED: a displaced column was called centred\n  {note}", file=sys.stderr)
            return False
    return True


def main(argv):
    expect_fail = False
    args = []
    for a in argv:
        if a == "--expect-fail":
            expect_fail = True
        else:
            args.append(a)
    if not args:
        print(__doc__.strip().splitlines()[0], file=sys.stderr)
        print("usage: check-reading-centred.py [--expect-fail] <image> ...", file=sys.stderr)
        return 2

    if not self_test():
        print("the rule does not behave on images with a known answer; judging nothing", file=sys.stderr)
        return 2

    bad = 0
    for path in args:
        centred, note = verdict(path)
        if expect_fail:
            if centred:
                print(f"FAIL {path}: expected an OFF-CENTRE column and measured a centred one.\n     {note}\n"
                      f"     This fixture exists to prove the rule can fail. It no longer can, so a\n"
                      f"     passing run on a real screenshot proves nothing.", file=sys.stderr)
                bad += 1
            else:
                print(f"ok   {path}: off-centre, as this fixture requires\n     {note}")
        else:
            if centred:
                print(f"ok   {path}: the reading column is centred\n     {note}")
            else:
                print(f"FAIL {path}: the reading column is not centred in its pane.\n     {note}\n"
                      f"     This is the shape of a window laid out for a size it was not given.", file=sys.stderr)
                bad += 1
    return 1 if bad else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
