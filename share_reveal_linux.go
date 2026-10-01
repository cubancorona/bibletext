//go:build linux && !android

package bibletext

// Showing a shared picture in the file manager on Linux (revealFile, behind
// revealInFileManager in share_fallback.go).
//
// Outside a sandbox — the tarball and the AppImage — xdg-open opens the
// picture's folder in the desktop's file manager, and its exit status says
// whether it could.
//
// Inside the snap that route opens nothing: xdg-open there is snapd's, which
// hands on web links and not folders. The picture is shown through the
// desktop portal instead: OpenURI.OpenDirectory, handed the file as a
// descriptor, asks the file manager to show it selected
// (org.freedesktop.FileManager1.ShowItems), or failing that opens its
// folder. The portal reads the descriptor's path as the host sees it and
// refuses one the host cannot reach, such as a file in the snap's private
// temp folder, and from a sandboxed app it refuses a descriptor open for
// writing, so the file is opened read-only. Its request's Response says how
// it went, 0 when the file manager took it; that answer, and nothing short
// of it, is what lets the sheet say the picture is shown.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"github.com/godbus/dbus/v5"
)

// revealFile asks the file manager to show path and reports whether it said
// it did. It blocks, on the bus or on xdg-open, for at most
// revealAnswerWait; never on the UI goroutine.
func revealFile(path string) bool {
	if linuxSandboxed() {
		if err := revealViaPortal(path, revealAnswerWait); err != nil {
			fyne.LogError("could not show the shared picture in the file manager", err)
			return false
		}
		return true
	}
	return revealByCommand(exec.Command("xdg-open", filepath.Dir(path)), revealAnswerWait)
}

// revealViaPortal calls org.freedesktop.portal.OpenURI.OpenDirectory with
// path as a read-only descriptor and waits at most wait for its request's
// Response (portalRequest, share_portal_linux.go). Anything but a Response of
// 0 is an error.
func revealViaPortal(path string, wait time.Duration) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	if !conn.SupportsUnixFDs() {
		return errors.New("the session bus does not carry file descriptors")
	}
	code, _, err := portalRequest(conn, portalOpenURIIface+".OpenDirectory", map[string]dbus.Variant{}, wait, "", dbus.UnixFD(f.Fd()))
	if err != nil {
		return fmt.Errorf("the OpenURI portal: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("the OpenURI portal answered %d to OpenDirectory", code)
	}
	return nil
}
