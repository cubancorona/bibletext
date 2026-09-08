"""Screenshot helpers for the Play capture run.

The layout check exists because the earlier run produced images that LOOKED
fine to a naive check: the app came back from a theme change with its pane
occupying the top ~45% of the window, and a "does content reach the bottom"
test passed anyway because the page background is not the same colour as the
system background. So the test here is specific - is there real content in the
bottom band, where the tab bar must be - and it is run before every capture.
"""
import subprocess, sys
from PIL import Image

E = "emulator-5554"

def sh(*args):
    return subprocess.run(["adb", "-s", E, *args], capture_output=True, text=True).stdout

def grab(path):
    with open(path, "wb") as f:
        f.write(subprocess.run(["adb", "-s", E, "exec-out", "screencap", "-p"],
                               capture_output=True).stdout)
    return path

def laid_out(path):
    """True when the tab bar is where it belongs: real content in the bottom band."""
    im = Image.open(path).convert("RGB")
    w, h = im.size
    px = im.load()
    bg = px[w // 2, h - 12]                      # system background under the app
    band = range(int(h * 0.90), int(h * 0.97))   # where Read/Books/Search sit
    ink = sum(1 for y in band for x in range(0, w, 12)
              if sum(abs(a - b) for a, b in zip(px[x, y], bg)) > 45)
    return ink > 40, ink

if __name__ == "__main__":
    p = grab(sys.argv[1] if len(sys.argv) > 1 else "/tmp/probe.png")
    ok, ink = laid_out(p)
    print(f"  laid out: {ok} (ink in the tab-bar band: {ink})")
