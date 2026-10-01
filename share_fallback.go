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
// All run on the Fyne UI goroutine (the share flow dispatches from menu
// taps); the image share's save and reveal leave it (shareImageAside).

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
//
// The card is read here, as the tap left it, since the next card is
// rendered over the same file; the save and the reveal then run off the UI
// goroutine (shareImageAside), and the sheet back on it. The save can wait
// on more than the disk: inside the snap, where AppArmor prompting is on,
// the first write into the reader's home waits for the reader to answer a
// permission prompt, and the window must not freeze while it does.
// Each platform's share mechanism is recorded in docs/PLATFORM_MATRIX.md, Sharing.
func fallbackShareImage(path string) {
	state := activeAIState
	// The mail is this share's: a preview opened while the picture is being
	// saved or the file manager is answering sets its own.
	mail := shareImageMail
	card, readErr := os.ReadFile(path)
	place, now := shareImagePlaceNow(), time.Now()
	var saved savedImage
	var shown bool
	shareImageAside(func() {
		if readErr != nil {
			saved = savedImage{file: path, err: readErr}
		} else {
			saved = saveSharedImage(card, path, place, now)
		}
		shown = revealInFileManager(saved.file)
	}, func() {
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

// shareImageAside runs work off the UI goroutine and then done on it: the
// image share's save and reveal, which wait on the disk, a permission prompt
// and the file manager, and then the sheet that says how they went. A
// variable so that a host test can run the two in place, in order, or hold
// them.
var shareImageAside = func(work, done func()) {
	go func() {
		work()
		fyne.Do(done)
	}()
}

// writeFileContents writes data to dst (0644), removing what it wrote when
// the write fails, so that a folder is never left a part of a picture.
func writeFileContents(dst string, data []byte) error {
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
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
// path and reports whether the file manager said it did (revealFile,
// share_reveal_linux.go and share_reveal_other.go). It blocks, so it runs in
// shareImageAside's work, never on the UI goroutine. A variable so a host
// test of the image share opens no window on the machine running it, and
// answers as the test chooses.
var revealInFileManager = revealFile

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
