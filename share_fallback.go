//go:build !ios && !android

package bibletext

// The desktop share FALLBACK bodies — clipboard + the confirmation sheet for
// text, save to ~/Downloads + file-manager reveal + the same sheet for
// images (share_sheet_desktop.go). These are the shipping Linux share verbs
// (share_other.go's nativeShareText/-Image are one-line wrappers over them),
// and the Windows Share sheet's fallback, wherever it cannot take a share
// (share_windows.go). They are untagged-for-desktop so the darwin
// platform-mimic dev mode (dev_mimic_on.go) can route the macOS share verbs
// here and show the real Linux share UX on a Mac. On release macOS nothing
// references them (reading_macos.go's mimic branch is dead behind a
// constant), so the linker drops them.
//
// All run on the Fyne UI goroutine (the share flow dispatches from menu taps).

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// fallbackShareText copies the composed text to the clipboard and opens the
// confirmation sheet over the page: what was copied, what to do with it,
// Copy again, Email… where there is a mail client, Done.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func fallbackShareText(s string) {
	state := activeAIState
	if state == nil || state.window == nil {
		return
	}
	setShareClipboard(s)
	showShareCopiedSheet(state, shareDoneForText(s))
}

// fallbackShareImage saves the rendered PNG to ~/Downloads (falling back to
// the temp copy), reveals it in the file manager, and opens the confirmation
// sheet saying where it went, with Email… attaching it where the platform
// can (shareImageMail). Where there is no Downloads folder, or the copy
// fails, the sheet says only that the picture is shown in the file manager,
// which then opens on the temp copy.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func fallbackShareImage(path string) {
	state := activeAIState
	// The renderer writes to a temp file; move the share into ~/Downloads under
	// a readable name so it outlives temp cleaning and is easy to find.
	dst := path
	line := shareLineImageTemp
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, "Downloads")
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			target := filepath.Join(dir, shareImageName(time.Now()))
			if copyFileContents(path, target) == nil {
				dst = target
				line = shareLineImage
			}
		}
	}
	revealInFileManager(dst)
	if state != nil {
		showShareCopiedSheet(state, shareDone{line: line, subject: shareImageMail.subject, body: shareImageMail.body, attachment: dst})
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
// xdg-open elsewhere (Linux/BSD). Failures are silent — the sheet already says
// where the file went. The Wait goroutine reaps the short-lived helper so each
// share doesn't leave a zombie behind. A variable so a host test of the image
// share opens no window on the machine running it.
var revealInFileManager = func(path string) {
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
