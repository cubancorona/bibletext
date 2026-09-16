//go:build windows

package bibletext

// openLinkInBrowser on Windows: start the default browser DIRECTLY.
//
// The toolkit's route is ShellExecute (rundll32 url.dll,FileProtocolHandler),
// and on the Store build ShellExecute of a bibletext.co.uk link is exactly
// what the web-to-app handler intercepts — the app would launch itself
// instead of the browser. The shell answers "which browser?" through the
// association API, and asked WITHOUT the app-to-app flag it answers with the
// browser's own command line, never with an apps-for-websites handler. That
// command is run as it stands; only when the query fails does the toolkit's
// route serve, where the echo guard (share_link_echo.go) ends the one hop the
// interception can cause. The wiring itself is openInBrowserWithFallback
// (share_link_browser_command.go), tested on every platform.

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shlwapi               = windows.NewLazySystemDLL("shlwapi.dll")
	procAssocQueryStringW = shlwapi.NewProc("AssocQueryStringW")
)

const (
	assocfNoTruncate = 0x20   // fail rather than truncate the answer
	assocfIsProtocol = 0x1000 // "https" is a protocol, mapped through the user's defaults
	assocstrCommand  = 1      // the command string for the verb
)

func init() { directBrowserLaunch = startBrowserProcess }

func openLinkInBrowser(rawURL string) { openInBrowserWithFallback(rawURL, defaultBrowserCommand) }

// startBrowserProcess runs the browser's command line as the shell recorded
// it, verbatim (Edge's --single-argument needs the exact string).
func startBrowserProcess(cmdline, exe string) error {
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: cmdline}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// defaultBrowserCommand asks the shell for the https "open" command of the
// user's default browser, app-to-app handlers excluded, and fills the URL in.
func defaultBrowserCommand(rawURL string) (cmdline, exe string, ok bool) {
	if procAssocQueryStringW.Find() != nil {
		return "", "", false
	}
	proto, err := windows.UTF16PtrFromString("https")
	if err != nil {
		return "", "", false
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return "", "", false
	}
	flags := uintptr(assocfNoTruncate | assocfIsProtocol)
	var n uint32
	procAssocQueryStringW.Call(flags, assocstrCommand, uintptr(unsafe.Pointer(proto)),
		uintptr(unsafe.Pointer(verb)), 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 || n > 32768 {
		return "", "", false
	}
	buf := make([]uint16, n)
	r, _, _ := procAssocQueryStringW.Call(flags, assocstrCommand, uintptr(unsafe.Pointer(proto)),
		uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 { // S_OK only
		return "", "", false
	}
	return browserCommandLine(windows.UTF16ToString(buf), rawURL)
}
