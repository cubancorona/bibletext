//go:build windows

package bibletext

// Email… on Windows: a mailto: link through the shell, the route every other
// link the app opens takes there (external_link.go, rundll32's protocol
// handler), with the subject and body in the link (mailtoURL, which keeps it
// to 2,000 characters). A link carries no file, so the image share offers no
// Email… on Windows; the native Share sheet planned for Windows will carry
// it (docs/BACKLOG.md).

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	assocfInitIgnoreUnknown = 0x400 // no fallback to the "Unknown" class: no handler means no answer
	assocfVerify            = 0x40  // the handler's executable must exist on disk
	assocstrExecutable      = 2     // the executable the verb runs
	assocstrDelegateExecute = 18    // the COM handler a packaged app's verb runs instead
)

// shareEmailAvailable reports whether the shell has a mailto: handler to
// open the link with (schemeHandlerRegistered).
func shareEmailAvailable(withAttachment bool) bool {
	if withAttachment {
		return false
	}
	return schemeHandlerRegistered("mailto")
}

// schemeHandlerRegistered asks the shell for the scheme's handler the way
// ShellExecute resolves it: through the association API, which reads the
// reader's own choice (UserChoice) before the class's registration. A
// handler counts when its executable is on disk — a key left behind by an
// uninstalled client does not — or when it is a packaged app's, which opens
// through a DelegateExecute handler rather than a command line.
func schemeHandlerRegistered(scheme string) bool {
	base := uintptr(assocfNoTruncate | assocfIsProtocol | assocfInitIgnoreUnknown)
	return assocQuery(base|assocfVerify, assocstrExecutable, scheme) != "" ||
		assocQuery(base, assocstrDelegateExecute, scheme) != ""
}

// assocQuery is AssocQueryStringW for the "open" verb of assoc, or "" when
// the shell has no answer.
func assocQuery(flags, str uintptr, assoc string) string {
	if procAssocQueryStringW.Find() != nil {
		return ""
	}
	name, err := windows.UTF16PtrFromString(assoc)
	if err != nil {
		return ""
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return ""
	}
	var n uint32
	procAssocQueryStringW.Call(flags, str, uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(verb)), 0, uintptr(unsafe.Pointer(&n)))
	if n == 0 || n > 32768 {
		return ""
	}
	buf := make([]uint16, n)
	r, _, _ := procAssocQueryStringW.Call(flags, str, uintptr(unsafe.Pointer(name)),
		uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
	if r != 0 { // S_OK only
		return ""
	}
	return windows.UTF16ToString(buf)
}

func composeShareEmail(subject, body, attachment string) error {
	if attachment != "" {
		return errors.New("a mailto: link carries no attachment")
	}
	return externalOpener(mailtoURL(mailSubjectLine(subject), body))
}
