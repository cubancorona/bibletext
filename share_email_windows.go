//go:build windows

package bibletext

// Email… on Windows: a mailto: link through the shell, the route every other
// link the app opens takes there (external_link.go, rundll32's protocol
// handler), with the subject and body in the link. A link carries no file,
// so the image share offers no Email… on Windows; the native Share sheet
// planned for Windows will carry it (docs/BACKLOG.md).

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// shareEmailAvailable reports whether a mailto: handler is registered: the
// shell opens the link through the mailto class's open command, and a
// Windows with no mail client registered has no such key.
func shareEmailAvailable(withAttachment bool) bool {
	if withAttachment {
		return false
	}
	k, err := registry.OpenKey(registry.CLASSES_ROOT, `mailto\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

func composeShareEmail(subject, body, attachment string) error {
	if attachment != "" {
		return errors.New("a mailto: link carries no attachment")
	}
	return externalOpener(mailtoURL(mailSubjectLine(subject), body))
}
