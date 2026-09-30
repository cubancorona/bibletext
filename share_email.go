//go:build !ios && !android

package bibletext

// Email… on the desktop share confirmation (share_sheet_desktop.go): the one
// hand-off the sheet offers beyond the clipboard, to whatever mail client the
// desktop has. The two platform calls behind the seams below are
//
//   - shareEmailAvailable(withAttachment): whether there is a mail client to
//     hand the share to, and, for the image share, one that takes a file;
//   - composeShareEmail(subject, body, attachment): open the client on a new
//     message carrying them.
//
// Linux asks the desktop portal and falls back to xdg-email and then a
// mailto: link (share_email_linux.go); Windows opens a mailto: link through
// the shell, which carries no file (share_email_windows.go); everywhere else
// the fallback bodies compile (the macOS mimic of the desktop path) it is the
// mailto: link too (share_email_other.go). Both calls may block on the bus
// or a subprocess, so the sheet runs them off the UI goroutine through the
// seams, which a test replaces to answer at once and to open nothing.

import (
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
)

// shareEmailProbe asks the platform whether Email… has somewhere to go and
// reports the answer on the UI goroutine. The sheet is never delayed by it:
// the button appears when the answer comes, and not at all when it is no.
var shareEmailProbe = func(withAttachment bool, report func(bool)) {
	go func() {
		ok := shareEmailAvailable(withAttachment)
		fyne.Do(func() { report(ok) })
	}()
}

// shareEmailCompose opens the mail client on the share. Called off the UI
// goroutine.
var shareEmailCompose = composeShareEmail

// mailSubjectMaxRunes is the Email portal's cap on a subject.
const mailSubjectMaxRunes = 200

// mailSubjectLine makes a subject every route accepts: one line, at most
// mailSubjectMaxRunes characters.
func mailSubjectLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > mailSubjectMaxRunes {
		s = strings.TrimSpace(string(r[:mailSubjectMaxRunes]))
	}
	return s
}

// mailtoURL is the mailto: link for a new message with subject and body, for
// the routes that open one through the shell. Spaces are %20, not +: the
// query of a mailto: URL is not a form (RFC 6068).
func mailtoURL(subject, body string) *url.URL {
	return &url.URL{
		Scheme: "mailto",
		Opaque: "?subject=" + mailtoEscape(subject) + "&body=" + mailtoEscape(body),
	}
}

func mailtoEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}
