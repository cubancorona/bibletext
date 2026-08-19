//go:build !ios && !android

package bibletext

// The desktop share FALLBACK bodies — clipboard + notice for text, save to
// ~/Downloads + file-manager reveal for images. These are the shipping
// Windows/Linux share verbs (share_other.go's nativeShareText/-Image are
// one-line wrappers over them, byte-identical in behaviour to before the
// extraction), and they are untagged-for-desktop so the darwin platform-mimic
// dev mode (dev_mimic_on.go) can route the macOS share verbs here and show the
// real Windows/Linux share UX on a Mac. On release macOS nothing references
// them (reading_macos.go's mimic branch is dead behind a constant), so the
// linker drops them.
//
// All run on the Fyne UI goroutine (the share flow dispatches from menu taps).

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// fallbackShareText copies the composed quote+citation to the clipboard with a
// brief confirmation notice — ready to paste anywhere.
func fallbackShareText(s string) {
	state := activeAIState
	if state == nil || state.window == nil {
		return
	}
	cb := state.window.Clipboard()
	if cb == nil {
		return
	}
	cb.SetContent(s)
	showShareNotice(state, "Copied to the clipboard")
}

// fallbackShareImage saves the rendered PNG to ~/Downloads (falling back to the
// temp copy) and reveals it in the file manager.
func fallbackShareImage(path string) {
	state := activeAIState
	// The renderer writes to a temp file; move the share into ~/Downloads under
	// a readable name so it outlives temp cleaning and is easy to find.
	dst := path
	notice := "Image ready"
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, "Downloads")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			name := fmt.Sprintf("BibleText verse %s.png", time.Now().Format("2006-01-02 15.04.05"))
			target := filepath.Join(dir, name)
			if copyFileContents(path, target) == nil {
				dst = target
				notice = "Image saved to Downloads"
			}
		}
	}
	revealInFileManager(dst)
	if state != nil {
		showShareNotice(state, notice)
	}
}

// copyFileContents copies src to dst (0644), failing without side effects on a
// read error.
func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// revealInFileManager shows the file to the user: Explorer's select mode on
// Windows, Finder's reveal on macOS (only reachable via the mimic dev mode —
// the flow still needs to COMPLETE there, and the Mac equivalent is the honest
// substitute; the doc's not-mimicked table names the real Explorer/xdg-open
// behaviour as provable only on the target OS), the containing folder via
// xdg-open elsewhere (Linux/BSD). Failures are silent — the notice already says
// where the file went. The Wait goroutine reaps the short-lived helper so each
// share doesn't leave a zombie behind.
func revealInFileManager(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}

// shareNotice is the currently-showing confirmation popup, so back-to-back
// shares replace the notice instead of stacking overlays (each PopUp overlay
// intercepts scroll for its lifetime — one short-lived notice keeps that
// window minimal).
var shareNotice *widget.PopUp

// showShareNotice flashes a small confirmation at the bottom of the window and
// auto-dismisses — the desktop stand-in for the mobile share sheet's feedback.
func showShareNotice(state *AppState, msg string) {
	if state == nil || state.window == nil {
		return
	}
	cnv := state.window.Canvas()
	if cnv == nil {
		return
	}
	if shareNotice != nil {
		shareNotice.Hide()
	}
	pal := state.pal()
	txt := canvas.NewText(msg, pal.Text)
	txt.TextSize = 13
	pop := widget.NewPopUp(surface(container.NewPadded(txt), pal.SurfaceAlt, pal.Border, fyne.Size{}), cnv)
	shareNotice = pop
	sz := pop.MinSize()
	pop.ShowAtPosition(fyne.NewPos((cnv.Size().Width-sz.Width)/2, cnv.Size().Height-sz.Height-28))
	time.AfterFunc(1400*time.Millisecond, func() {
		fyne.Do(func() {
			pop.Hide()
			if shareNotice == pop {
				shareNotice = nil
			}
		})
	})
}
