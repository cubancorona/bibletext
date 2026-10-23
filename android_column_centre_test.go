package bibletext

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Where Android's book page stands across the reading overlay. The bridge
// centred the column on the window, which suits a phone held sideways: the
// toolkit keeps the pane out of the camera cutout, so the pane stands off the
// screen's centre by half the cutout's inset, and a column centred on the
// window clears the cutout evenly, as the iPhone's does. Beside a tablet's
// rail the same rule set the column about half the rail's width left of the
// pane's centre, where the iPad centres its column on its pane
// (btIOSApplyInsets). The column now takes the pane's centre unless the pane
// stands evenly in the window less the system insets at its sides
// (BtReadingColumn.sides).
//
// The arithmetic is android/BtReadingColumn.java, which uses no platform
// types, so this test compiles that file with the JDK and runs it as it
// ships. Without a JDK it skips, except on CI's Linux job, which compiles
// android/*.java with javac a step before the tests run
// (scripts/check-android-java.sh), so a JDK is there.
//
// The first fixtures are geometry the bridge logged on the emulators, with the
// padding it set there: the 10-inch tablet at density 2, and phones on API 35,
// 33 and 30 held sideways, each with a cutout on the side it is turned to and
// the API 30 phone with a navigation bar opposite it. Beside the tablet's rail
// the frame stands 136 px from the window's left and 28 px from its right;
// held upright, 14 and 28. The right margin carries the reading body's own
// theme pad, which buildReadingViewMobile sets on the right only, so the
// upright pane is uneven in the window by that pad and its column takes the
// pane's centre, 7 px left of the window's, as the iPad's does in its text
// view; the rule before took the window's, 6.5 px right of the pane's. A
// phone's pane held sideways has margins of 42 or 28 px, the same both sides.
// The measure is the one the bridge logged, the book page's at the reference
// size.
//
// The rest are built from those: the cutout on the other side, side insets
// on the tablet, the reader's own full screen (container.NewPadded, even on
// both sides), a measure wider than the pane, a window whose width is not
// read, a phone's rail, and panes one, two and three pixels uneven, which hold
// the slack the frame's rounding is allowed (BtReadingColumn.EVEN_SLACK_PX)
// to 2 px.
//
// The rule before, centred on the window for every pane
// (columnSidesBefore), is the control: it must give every phone fixture the
// same padding as the bridge does now, and must leave every fixture marked
// moved with its column off the pane's centre, or the fixtures no longer show
// what the change does.
func TestAndroidCentresTheBookPageOnItsPaneBesideARail(t *testing.T) {
	const measure = 577.5
	cases := []columnFixture{
		{name: "tablet sideways, beside the rail", density: 2, windowW: 2560, frameX: 136, frameW: 2396, measureDp: measure, centre: onPane, moved: true, logged: []int{620, 620}},
		{name: "tablet upright, the bar below", density: 2, windowW: 1600, frameX: 14, frameW: 1558, measureDp: measure, centre: onPane, moved: true, logged: []int{201, 201}},
		{name: "API 35 phone sideways, cutout on the left", density: 2.625, windowW: 2400, insetL: 136, frameX: 42, frameW: 2180, measureDp: measure, centre: onWindow, phone: true, logged: []int{264, 400}},
		{name: "API 33 phone sideways, cutout on the left", density: 1.6875, windowW: 1600, insetL: 136, frameX: 28, frameW: 1408, measureDp: measure, centre: onWindow, phone: true, logged: []int{148, 285}},
		{name: "API 30 phone sideways, cutout left, navigation bar right", density: 2.75, windowW: 2340, insetL: 136, insetR: 132, frameX: 42, frameW: 1988, measureDp: measure, centre: onWindow, phone: true, logged: []int{198, 202}},
		{name: "API 30 phone sideways, navigation bar left, cutout right", density: 2.75, windowW: 2340, insetL: 132, insetR: 136, frameX: 42, frameW: 1988, measureDp: measure, centre: onWindow, phone: true, logged: []int{202, 198}},
		{name: "API 35 phone upright, the phone page", density: 2.625, windowW: 1080, frameX: 21, frameW: 1017, centre: onPadding, phone: true},

		{name: "tablet sideways, beside the rail, an inset on the rail's side", density: 2, windowW: 2560, insetL: 64, frameX: 136, frameW: 2332, measureDp: measure, centre: onPane, moved: true},
		{name: "tablet sideways, beside the rail, an inset on the far side", density: 2, windowW: 2560, insetR: 64, frameX: 136, frameW: 2332, measureDp: measure, centre: onPane, moved: true},
		{name: "tablet sideways, the reader's own full screen", density: 2, windowW: 2560, frameX: 14, frameW: 2532, measureDp: measure, centre: onWindow},
		{name: "tablet upright, the reader's own full screen", density: 2, windowW: 1600, frameX: 14, frameW: 1572, measureDp: measure, centre: onWindow},
		{name: "tablet upright, a measure wider than the pane", density: 2, windowW: 1600, frameX: 14, frameW: 1558, measureDp: 820, centre: onPadding},
		{name: "API 35 phone sideways, cutout on the right", density: 2.625, windowW: 2400, insetR: 136, frameX: 42, frameW: 2180, measureDp: measure, centre: onWindow, phone: true},
		{name: "phone sideways, the window's width not read", density: 2.625, windowW: 0, insetL: 136, frameX: 42, frameW: 2180, measureDp: measure, centre: onPane, phone: true},
		// The Read tab shows a rail on a phone held sideways only with the
		// landscape presentation switched off, which a development build
		// alone can do. Its column follows the rail's rule.
		{name: "phone sideways, presentation off, rail", density: 2.625, windowW: 2400, insetL: 136, frameX: 150, frameW: 2102, measureDp: measure, centre: onPane, moved: true},
		// The frame's place and width are each rounded to a pixel, so a
		// phone's pane can stand a pixel or two uneven and still takes the
		// window's centre; three pixels is past that, and takes the pane's.
		{name: "API 35 phone sideways, the frame 1 px uneven", density: 2.625, windowW: 2400, insetL: 136, frameX: 42, frameW: 2179, measureDp: measure, centre: onWindow, phone: true},
		{name: "API 35 phone sideways, the frame 2 px uneven", density: 2.625, windowW: 2400, insetL: 136, frameX: 42, frameW: 2178, measureDp: measure, centre: onWindow, phone: true},
		{name: "a pane 3 px uneven in the window", density: 2.625, windowW: 2400, insetL: 136, frameX: 42, frameW: 2177, measureDp: measure, centre: onPane, moved: true},
	}

	got := runReadingColumn(t, cases)
	for i, c := range cases {
		sides := got[i]
		before := columnSidesBefore(c)
		if c.logged != nil && !equalSides(sides, c.logged) {
			t.Errorf("%s: the bridge pads %v; on the emulator it padded %v", c.name, sides, c.logged)
		}
		switch {
		case c.centre == onPadding:
			if sides != nil {
				t.Errorf("%s: the bridge pads %v; the pushed padding should stand", c.name, sides)
			}
		case sides == nil:
			t.Errorf("%s: the bridge keeps the pushed padding; the column should be centred", c.name)
		default:
			col := c.columnCentre2(sides)
			want, where := c.paneCentre2(), "pane"
			if c.centre == onWindow {
				want, where = c.windowW, "window"
			}
			if d := col - want; d < -2 || d > 2 {
				t.Errorf("%s: the column's centre is %.1f px, the %s's %.1f: %.1f px off",
					c.name, float64(col)/2, where, float64(want)/2, float64(d)/2)
			}
			m, side := c.measurePx(), c.sideMinPx()
			if w := c.frameW - sides[0] - sides[1]; w < m || w > m+1 {
				t.Errorf("%s: the column is %d px wide, the measure %d", c.name, w, m)
			}
			if sides[0] < side || sides[1] < side {
				t.Errorf("%s: padding %v is inside the page's margin of %d px", c.name, sides, side)
			}
		}
		if c.phone && !equalSides(sides, before) {
			t.Errorf("%s: a phone's padding moved from %v to %v", c.name, before, sides)
		}
		if c.moved {
			if before == nil {
				t.Errorf("%s: the rule before keeps the pushed padding; the fixture shows nothing", c.name)
				continue
			}
			off := float64(c.columnCentre2(before)-c.paneCentre2()) / 2
			if math.Abs(off) <= 1 {
				t.Errorf("%s: the rule before already centres the column on the pane; the fixture shows nothing", c.name)
			}
			t.Logf("%s: the rule before set the column %.1f px from the pane's centre; now %v", c.name, off, sides)
		}
	}
}

// The bridge takes the column's place from BtReadingColumn with the live
// window, and no longer works it out itself.
func TestTheBridgeTakesTheColumnFromBtReadingColumn(t *testing.T) {
	java := readNativeSource(t, "android/BtBridge.java")
	body := squashSpace(javaMethodBody(t, java, "private static boolean applyReadingPadding("))
	for _, want := range []string{
		"int[] sides = BtReadingColumn.sides(w, measurePx, padL, windowW, insetL, insetR, frameX, frameW);",
		"insetL = wi.getSystemWindowInsetLeft();",
		"insetR = wi.getSystemWindowInsetRight();",
		"windowW = decor.getWidth();",
		"padL = sides[0];",
		"padR = sides[1];",
		"text.setPadding(padL, padT, padR, padB);",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("applyReadingPadding lacks %q", want)
		}
	}
	for _, gone := range []string{"- measurePx) / 2", "wantLeft", "viewLeft"} {
		if strings.Contains(body, gone) {
			t.Errorf("applyReadingPadding still works out the column itself (%q); BtReadingColumn.sides is the one place", gone)
		}
	}
	col := readNativeSource(t, "android/BtReadingColumn.java")
	if regexp.MustCompile(`(?m)^import `).MatchString(col) || strings.Contains(col, "android.") {
		t.Error("android/BtReadingColumn.java names a platform type; the host test can no longer run it as it ships")
	}
}

type columnCentreKind int

const (
	onPadding columnCentreKind = iota // the pushed padding stands
	onPane
	onWindow
)

// columnFixture is one overlay in one window, in the bridge's pixels: the
// window's width and side insets, and the frame Go pushed, from the left edge
// of the toolkit's content area. The text view is as wide as the frame
// (TestTheReadingContentIsAsWideAsTheOverlaysFrame).
type columnFixture struct {
	name                    string
	density                 float32
	windowW, insetL, insetR int
	frameX, frameW          int
	measureDp               float64 // 0 is the phone page
	centre                  columnCentreKind
	// phone: the padding must be the rule before's. moved: the rule before
	// set the column off the pane's centre.
	phone, moved bool
	// logged is the padding the bridge set for this geometry on the
	// emulator; nil for a fixture built rather than read back.
	logged []int
}

// javaRound is Math.round over a float product, as the bridge takes its dp to
// pixels.
func javaRound(dp float64, density float32) int {
	return int(math.Floor(float64(float32(dp)*density) + 0.5))
}

func (c columnFixture) measurePx() int {
	if c.measureDp <= 0 {
		return 0
	}
	return javaRound(c.measureDp, c.density)
}

func (c columnFixture) sideMinPx() int { return javaRound(readingPageSideMin, c.density) }

// The centres, doubled to stay in whole pixels, in window coordinates.
func (c columnFixture) columnCentre2(sides []int) int {
	return 2*(c.insetL+c.frameX) + c.frameW + sides[0] - sides[1]
}

func (c columnFixture) paneCentre2() int { return 2*(c.insetL+c.frameX) + c.frameW }

// columnSidesBefore is the bridge's rule before the change: centred on the
// window for every pane that fits it with the page's margin on both sides,
// on the pane otherwise, and the pushed padding when neither fits.
func columnSidesBefore(c columnFixture) []int {
	m, base, w := c.measurePx(), c.sideMinPx(), c.frameW
	if m <= 0 {
		return nil
	}
	if c.windowW > 0 {
		left := (c.windowW-m)/2 - (c.frameX + c.insetL)
		right := w - left - m
		if left >= base && right >= base {
			return []int{left, right}
		}
	}
	if side := (w - m) / 2; side > base {
		return []int{side, side}
	}
	return nil
}

func equalSides(a, b []int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a[0] == b[0] && a[1] == b[1]
}

func squashSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// columnProbeJava feeds BtReadingColumn.sides one line of eight integers at a
// time and prints its answer: the two paddings, or "keep".
const columnProbeJava = `package org.bibletext;

public final class ColumnProbe {
    public static void main(String[] args) throws Exception {
        java.io.BufferedReader in = new java.io.BufferedReader(
                new java.io.InputStreamReader(System.in, "UTF-8"));
        String line;
        while ((line = in.readLine()) != null) {
            String[] f = line.trim().split(" ");
            if (f.length != 8) continue;
            int[] v = new int[8];
            for (int i = 0; i < 8; i++) v[i] = Integer.parseInt(f[i]);
            int[] s = BtReadingColumn.sides(v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7]);
            System.out.println(s == null ? "keep" : s[0] + " " + s[1]);
        }
    }
}
`

// runReadingColumn compiles android/BtReadingColumn.java beside the probe and
// asks it for every fixture's paddings, nil where it keeps the pushed ones.
func runReadingColumn(t *testing.T, cases []columnFixture) [][]int {
	t.Helper()
	javac := jdkTool(t)
	dir := t.TempDir()
	probe := filepath.Join(dir, "ColumnProbe.java")
	if err := os.WriteFile(probe, []byte(columnProbeJava), 0o644); err != nil {
		t.Fatal(err)
	}
	compile := exec.Command(javac, "-encoding", "UTF-8", "-d", dir,
		filepath.Join("android", "BtReadingColumn.java"), probe)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}

	var in strings.Builder
	for _, c := range cases {
		fmt.Fprintf(&in, "%d %d %d %d %d %d %d %d\n", c.frameW, c.measurePx(), c.sideMinPx(),
			c.windowW, c.insetL, c.insetR, c.frameX, c.frameW)
	}
	java := filepath.Join(filepath.Dir(javac), "java"+filepath.Ext(javac))
	run := exec.Command(java, "-cp", dir, "org.bibletext.ColumnProbe")
	run.Stdin = strings.NewReader(in.String())
	var stderr bytes.Buffer
	run.Stderr = &stderr
	out, err := run.Output()
	if err != nil {
		t.Fatalf("java: %v\n%s", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("the probe answered %d fixtures of %d:\n%s", len(lines), len(cases), out)
	}
	got := make([][]int, len(cases))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "keep" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			t.Fatalf("the probe answered %q for %s", line, cases[i].name)
		}
		l, err1 := strconv.Atoi(f[0])
		r, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			t.Fatalf("the probe answered %q for %s", line, cases[i].name)
		}
		got[i] = []int{l, r}
	}
	return got
}

// jdkTool finds a javac that runs, and the java beside it is taken with it.
func jdkTool(t *testing.T) string {
	t.Helper()
	var found []string
	if p, err := exec.LookPath("javac"); err == nil {
		found = append(found, p)
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		exe := ""
		if runtime.GOOS == "windows" {
			exe = ".exe"
		}
		found = append(found, filepath.Join(home, "bin", "javac"+exe))
	}
	for _, p := range found {
		// A javac that cannot report its version is a stand-in for a JDK
		// that is not installed, as macOS's is.
		if err := exec.Command(p, "-version").Run(); err == nil {
			return p
		}
	}
	if os.Getenv("CI") != "" && runtime.GOOS == "linux" {
		t.Fatal("no JDK on CI's Linux job, which compiles android/*.java with javac; BtReadingColumn would go unrun")
	}
	t.Skip("no JDK found (javac on PATH or under JAVA_HOME); BtReadingColumn is not run")
	return ""
}
