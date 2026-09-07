//go:build bibletextdev

package bibletext

// Tests for the platform-mimic dev mode (dev_mimic_on.go). Dev builds only —
// the release-shaped twin of these assertions is dev_mimic_guard_test.go,
// which proves the switch is ABSENT without the tag.

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// saveMimicSeams snapshots every seam devApplyMimicSeams touches and restores
// them on cleanup, so a mimic test can never leak a flipped seam into the rest
// of the suite.
func saveMimicSeams(t *testing.T) {
	t.Helper()
	origStyled := useStyledPane
	origSheet := sheetConsumeClosure
	origSticker := nativeNoteSticker
	origReporter := reporterLayout
	origTTS := ttsSupported
	origTarget := devMimicTargetValue
	t.Cleanup(func() {
		useStyledPane = origStyled
		sheetConsumeClosure = origSheet
		nativeNoteSticker = origSticker
		reporterLayout = origReporter
		ttsSupported = origTTS
		devMimicTargetValue = origTarget
	})
}

// The linux target flips every platform seam to the Windows/Linux answer —
// including hiding TTS (no read-aloud row, no audio button on recording-less
// chapters) and dropping the Georgia font candidates.
func TestMimicLinuxFlipsSeams(t *testing.T) {
	saveMimicSeams(t)
	t.Setenv("BIBLETEXT_MIMIC", "linux")
	devApplyMimic()

	if !useStyledPane() {
		t.Error("useStyledPane is false under mimic — the styled reading pane would not build")
	}
	if !sheetConsumeClosure() {
		t.Error("sheetConsumeClosure is false under mimic — a deferred rebuild under an open sheet would leak")
	}
	// INVERTED 19 AUG, with the surface it names. The styled pane now draws the
	// note IN THE TEXT (reading_styled_note.go), so the Windows/Linux answer to
	// "does the pane draw the note itself?" is yes — and mimic must agree, or
	// the mode would show the retired banner and hide the very surface mimic
	// exists to preview. Nothing in dev_mimic_on.go pins this any more: it
	// follows useStyledPane, which the same function just set.
	if !nativeNoteSticker() {
		t.Error("nativeNoteSticker is false under mimic — mimic would draw the retired banner instead of the styled pane's own in-text sticker")
	}
	if reporterLayout() {
		t.Error("reporterLayout is true under mimic — the styled pane must own its own width-gated reporter typesetting, as on the target")
	}
	if ttsSupported() {
		t.Error("ttsSupported is true under mimic — the Windows/Linux audio surface hides read-aloud entirely")
	}
	if got := devMimicLabel(); got != "MIMIC: Linux" {
		t.Errorf("devMimicLabel() = %q, want %q — the badge is what keeps mimic screenshots honest", got, "MIMIC: Linux")
	}
	// There is no scripture-face seam to check any more. The reading face is
	// embedded, so a Mac, a Windows machine and a Linux machine all draw the
	// same Spectral and there is nothing for a mimic to approximate.
	if f := styledPaneFont(); f == nil || !strings.HasPrefix(f.Name(), "Spectral") {
		t.Errorf("styledPaneFont() = %v under mimic, want the shipped Spectral on every target", f)
	}
}

// The windows target flips the same behavioural seams. It no longer has
// anything to say about the scripture face: that is shipped, not borrowed.
func TestMimicWindowsFlipsTheBehaviouralSeams(t *testing.T) {
	saveMimicSeams(t)
	t.Setenv("BIBLETEXT_MIMIC", "windows")
	devApplyMimic()

	if !useStyledPane() || ttsSupported() {
		t.Error("mimic=windows did not flip the behavioural seams")
	}
	if got := devMimicLabel(); got != "MIMIC: Windows" {
		t.Errorf("devMimicLabel() = %q, want %q", got, "MIMIC: Windows")
	}

}

// An unknown target leaves the mode off and every seam at its platform answer.
func TestMimicIgnoresUnknownTarget(t *testing.T) {
	saveMimicSeams(t)
	t.Setenv("BIBLETEXT_MIMIC", "solaris")
	devApplyMimic()

	if devMimicTarget() != "" || devMimicLabel() != "" {
		t.Errorf("unknown target activated the mode: target=%q label=%q", devMimicTarget(), devMimicLabel())
	}
	if useStyledPane() != styledPaneEnabledOnPlatform {
		t.Error("unknown target moved the useStyledPane seam")
	}
	if ttsSupported() != ttsSupportedOnPlatform {
		t.Error("unknown target moved the ttsSupported seam")
	}
}

// Under mimic the reading view on this darwin host builds the REAL styled pane
// — the surface Windows and Linux ship — and the reader can see the verses in
// the Fyne tree (impossible with the native NSTextView overlay, which draws
// above the canvas).
func TestMimicBuildsStyledPaneOnHost(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	saveMimicSeams(t)
	t.Setenv("BIBLETEXT_MIMIC", "windows")
	devApplyMimic()

	st := psalm23State()
	got := seenText(t, buildReadingView(st), fyne.NewSize(900, 700))
	for _, want := range []string{"shepherd", "green pastures"} {
		if !strings.Contains(got, want) {
			t.Errorf("under mimic the reader cannot see %q on the reading screen — the styled pane did not build.\nseen:\n%s", want, got)
		}
	}
}
