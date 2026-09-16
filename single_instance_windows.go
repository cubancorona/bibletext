//go:build windows

package bibletext

import (
	"os"
	"path/filepath"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var (
	kernel32                      = windows.NewLazySystemDLL("kernel32.dll")
	user32                        = windows.NewLazySystemDLL("user32.dll")
	procGetCurrentPackageFullName = kernel32.NewProc("GetCurrentPackageFullName")
	procAllowSetForegroundWindow  = user32.NewProc("AllowSetForegroundWindow")
	procIsIconic                  = user32.NewProc("IsIconic")
	procShowWindow                = user32.NewProc("ShowWindow")
)

const (
	appModelErrorNoPackage = 15700 // "The process has no package identity."
	swRestore              = 9
)

// packagedWindows reports whether this process runs with package identity —
// the Store (MSIX) build — as opposed to the direct-download exe. Asked with
// no buffer: a packaged process answers ERROR_INSUFFICIENT_BUFFER with the
// length it would need; an unpackaged one answers APPMODEL_ERROR_NO_PACKAGE.
func packagedWindows() bool {
	if procGetCurrentPackageFullName.Find() != nil {
		return false
	}
	var n uint32
	r, _, _ := procGetCurrentPackageFullName.Call(uintptr(unsafe.Pointer(&n)), 0)
	return uint32(r) != appModelErrorNoPackage
}

// allowSetForeground lets the primary take the foreground. Windows grants the
// foreground only to the process the user launched — the forwarder, which
// is about to exit — unless that process passes the right on; otherwise the
// primary's taskbar button merely flashes.
func allowSetForeground(pid int) {
	if procAllowSetForegroundWindow.Find() != nil {
		return
	}
	procAllowSetForegroundWindow.Call(uintptr(uint32(pid)))
}

// restoreNative un-minimises the window: the toolkit's RequestFocus does not,
// and a link opened into a minimised reader would be invisible.
func restoreNative(w fyne.Window) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	nw.RunNative(func(ctx any) {
		c, ok := ctx.(driver.WindowsWindowContext)
		if !ok || c.HWND == 0 {
			return
		}
		if r, _, _ := procIsIconic.Call(c.HWND); r != 0 {
			procShowWindow.Call(c.HWND, swRestore)
		}
	})
}

// singleInstanceDir is %LocalAppData%\bibletext: per user, not roaming, and
// beside the Bible cache. Under MSIX the file is redirected into the
// package's LocalCache, which is why the channel is in its name.
func singleInstanceDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bibletext"), nil
}
