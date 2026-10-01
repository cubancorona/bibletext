//go:build !ios && !android

package bibletext

// The desktop share FALLBACK bodies — clipboard + the confirmation sheet for
// text, save to Downloads + file-manager reveal + the same sheet for images
// (share_sheet_desktop.go). These are the shipping Linux share verbs
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
	"time"

	"fyne.io/fyne/v2"
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

// fallbackShareImage saves the rendered PNG to the reader's Downloads folder
// (saveSharedImage, share_image_folder.go), asks the file manager to show
// it, and, once the file manager has answered, opens the confirmation sheet
// saying what happened, with Email… attaching the picture where the platform
// can (shareImageMail): saved in Downloads, or elsewhere, and shown in the
// file manager only when the file manager said it was (savedImage.line).
// The sheet waits for that answer, at most revealAnswerWait, so that it
// never says the picture is shown and then takes it back. A picture that no
// folder took and the file manager did not show has nothing true to be said
// of it, and no sheet opens; why is logged.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func fallbackShareImage(path string) {
	state := activeAIState
	// The mail is this share's: a preview opened while the file manager is
	// answering sets its own.
	mail := shareImageMail
	saved := saveSharedImage(path, shareImagePlaceNow(), time.Now())
	revealInFileManager(saved.file, func(shown bool) {
		line := saved.line(shown)
		if line == "" {
			fyne.LogError("the shared picture was saved nowhere the reader can find it", saved.err)
			return
		}
		if state != nil {
			showShareCopiedSheet(state, shareDone{line: line, subject: mail.subject, body: mail.body, attachment: saved.file})
		}
	})
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

// revealInFileManager asks the desktop's file manager to show the file at
// path, off the UI goroutine, and reports on the UI goroutine whether the
// file manager said it did (revealFile, share_reveal_linux.go and
// share_reveal_other.go). A variable so a host test of the image share opens
// no window on the machine running it, and answers as the test chooses.
var revealInFileManager = func(path string, report func(shown bool)) {
	go func() {
		shown := revealFile(path)
		fyne.Do(func() { report(shown) })
	}()
}

// revealAnswerWait is how long the sheet waits for the file manager's answer
// before it opens saying nothing of it. The portal's answer comes once the
// file manager has the file, which on a desktop just signed in to can mean
// starting it.
const revealAnswerWait = 5 * time.Second

// revealByCommand starts a command that hands a file or a folder to the file
// manager and reports whether it said it did: an exit status of 0 within
// wait. One that cannot start, fails, or is still running at wait has not
// said so; it is reaped whenever it exits.
func revealByCommand(cmd *exec.Cmd, wait time.Duration) bool {
	if err := cmd.Start(); err != nil {
		return false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case err := <-done:
		return err == nil
	case <-timer.C:
		return false
	}
}
