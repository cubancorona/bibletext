//go:build windows

package bibletext

import (
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"golang.org/x/sys/windows"
)

var (
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procSendMessageW          = user32.NewProc("SendMessageW")
)

const (
	// dwmwaUseImmersiveDarkMode is DWMWA_USE_IMMERSIVE_DARK_MODE as Windows 10
	// 20H1 and later, and Windows 11, number it — the number Fyne sends when
	// it creates the window.
	dwmwaUseImmersiveDarkMode = 20

	wmNCActivate   = 0x0086
	dwmEInvalidArg = 0x80070057 // E_INVALIDARG: a build that has no such attribute
)

// syncNativeTitleBar sets the window's immersive dark mode to the variant the
// content was just built in, then has Windows repaint the caption.
//
// Through the window's own handle, by the seam restoreNative
// (single_instance_windows.go) already uses: RunNative hands over the HWND of
// the GLFW window Fyne made. The attribute is the one Fyne sets at creation,
// under the same number and with the same BOOL, so a switch leaves the frame
// exactly as a launch into the new variant would have made it.
//
// Setting the attribute is not enough on Windows 10. There the attribute is
// stored, but a visible window's caption keeps the old mode until its
// activation changes or the window is resized: until the reader clicks away
// and back — the case of an automatic dark-mode schedule firing while the
// reader is in the app. A frame-changed SetWindowPos does not repaint it, nor
// does RedrawWindow; the activation message does, so a WM_NCACTIVATE pair is
// sent: the opposite of the caption's real state, then the real state
// (titleBarRepaint), which leaves it drawn as it was, now in the new mode.
// Windows 11 applies the attribute at once and the pair only repaints what is
// already right, so it is sent everywhere rather than behind a version check
// that could guess wrong. GLFW passes the message to DefWindowProc for a
// decorated window, and it activates nothing — focus and the app's own focus
// callbacks come from other messages.
//
// "Drawn active" is asked as "is this the foreground window", which any
// thread may ask; the thread-local active window would read "inactive" from
// the wrong thread and leave an active window with a grey caption.
//
// Only attribute 20. Windows 10 before 20H1 knows the same switch as 19, but
// Fyne never sends 19, so on those builds the frame has never followed the
// variant at launch; sending 19 here would make a switch the only way to get
// a dark frame there. Such a build answers E_INVALIDARG, which, as in Fyne, is
// not an error worth a log line.
func syncNativeTitleBar(w fyne.Window, v fyne.ThemeVariant) {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return
	}
	if procDwmSetWindowAttribute.Find() != nil {
		return // no DWM at all: nothing to follow
	}
	nw.RunNative(func(ctx any) {
		c, ok := ctx.(driver.WindowsWindowContext)
		if !ok || c.HWND == 0 {
			return // not created yet: Fyne's own creation reads the variant then
		}
		on := titleBarDarkMode(v)
		r, _, err := procDwmSetWindowAttribute.Call(c.HWND, dwmwaUseImmersiveDarkMode,
			uintptr(unsafe.Pointer(&on)), unsafe.Sizeof(on))
		if r != 0 {
			if uint32(r) != dwmEInvalidArg {
				fyne.LogError("Failed to set the title bar's dark mode", err)
			}
			return
		}
		if procGetForegroundWindow.Find() != nil || procSendMessageW.Find() != nil {
			return
		}
		fg, _, _ := procGetForegroundWindow.Call()
		for _, wParam := range titleBarRepaint(fg == c.HWND) {
			procSendMessageW.Call(c.HWND, wmNCActivate, wParam, 0)
		}
	})
}
