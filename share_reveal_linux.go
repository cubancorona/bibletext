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
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// path as a read-only descriptor and waits for its request's Response, which
// is matched before the call is made (the request's object path is known
// from the handle token), so an answer that comes at once is not missed.
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

	token := fmt.Sprintf("bibletext%d", time.Now().UnixNano())
	names := conn.Names()
	if len(names) == 0 {
		return errors.New("the session bus gave no unique name")
	}
	sender := strings.ReplaceAll(strings.TrimPrefix(names[0], ":"), ".", "_")
	request := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalRequestIface),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return err
	}
	responses := make(chan *dbus.Signal, 8)
	conn.Signal(responses)
	defer conn.RemoveSignal(responses)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token)}
	var handle dbus.ObjectPath
	if err := conn.Object(portalDest, portalPath).
		CallWithContext(ctx, portalOpenURIIface+".OpenDirectory", 0, "", dbus.UnixFD(f.Fd()), options).
		Store(&handle); err != nil {
		return err
	}
	for {
		select {
		case sig := <-responses:
			if sig == nil || (sig.Path != handle && sig.Path != request) {
				continue
			}
			code, ok := uint32(0), len(sig.Body) > 0
			if ok {
				code, ok = sig.Body[0].(uint32)
			}
			if !ok {
				return errors.New("the OpenURI portal answered with no response code")
			}
			if code != 0 {
				return fmt.Errorf("the OpenURI portal answered %d to OpenDirectory", code)
			}
			return nil
		case <-ctx.Done():
			return errors.New("no answer from the OpenURI portal")
		}
	}
}
