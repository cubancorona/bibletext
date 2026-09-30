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
//
// The decisions Linux takes — which route may follow a failed one, what the
// portal's answer means, and when the button is offered — are pure functions
// here rather than in the Linux file, so that the suite on every platform
// holds them, not only the Linux job.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
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

// mailtoMaxLen is the longest mailto: link the app builds, in characters.
// The shell passes a link of any length on (CreateProcess takes 32,767), but
// mail handlers on Windows cut or refuse a link past about 2,000 characters
// (the old 2,083-character URL limit), and a multi-verse share passes that
// easily: John 3 whole is some 5,900 characters as a link, Psalm 119 some
// 18,000.
const mailtoMaxLen = 2000

// mailtoURL is the mailto: link for a new message with subject and body, for
// the routes that open one through the shell. Spaces are %20, not +: the
// query of a mailto: URL is not a form, and line breaks are CRLF, %0D%0A, as
// RFC 6068 (section 5) requires of a body. A link that would pass
// mailtoMaxLen carries a shortened body (mailtoFitBody).
func mailtoURL(subject, body string) *url.URL {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	link := func(b string) *url.URL {
		return &url.URL{
			Scheme: "mailto",
			Opaque: "?subject=" + mailtoEscape(subject) + "&body=" + mailtoEscape(strings.ReplaceAll(b, "\n", "\r\n")),
		}
	}
	return link(mailtoFitBody(body, func(b string) bool { return len(link(b).String()) <= mailtoMaxLen }))
}

func mailtoEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// mailtoFitBody shortens a share's body until fits accepts it. The share's
// last paragraph — the citation line, or the citation and the link beneath
// it — is kept whole; the passage or the note before it is cut at a word and
// ends in an ellipsis, inside the closing quotation mark when it is a
// quotation. The whole text is still on the clipboard (the sheet says so), for
// the reader to paste in its place.
func mailtoFitBody(body string, fits func(string) bool) string {
	if fits(body) {
		return body
	}
	head, tail := body, ""
	if i := strings.LastIndex(body, "\n\n"); i >= 0 {
		head, tail = body[:i], body[i:]
	}
	closeQuote := ""
	if strings.HasPrefix(head, "“") && strings.HasSuffix(head, "”") {
		head, closeQuote = strings.TrimSuffix(head, "”"), "”"
	}
	cut := func(n int) string {
		r := []rune(head)
		if n < len(r) {
			// Back to the start of the word the cut fell in.
			for n > 0 && !isMailSpace(r[n]) {
				n--
			}
		}
		kept := strings.TrimRight(string(r[:n]), " \t\n,;:—–-")
		if kept == "" || kept == "“" {
			return "…" + tail
		}
		return kept + "…" + closeQuote + tail
	}
	lo, hi := 0, len([]rune(head))
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fits(cut(mid)) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return cut(lo)
}

func isMailSpace(r rune) bool { return r == ' ' || r == '\n' || r == '\t' }

// mailRouteStop marks a failed route as the end of the compose: the route
// reached the desktop, which may yet show a compose of its own, so no later
// route may open a second one.
type mailRouteStop struct{ err error }

func (e mailRouteStop) Error() string { return e.err.Error() }
func (e mailRouteStop) Unwrap() error { return e.err }

// mailRoute is one way of opening a compose.
type mailRoute func(subject, body, attachment string) error

// composeByRoutes tries routes in order until one succeeds. A route that
// fails before it reached anything — no portal, no tool, the desktop
// answering that it has nothing to compose with — hands on to the next; one
// that fails with mailRouteStop ends the compose there. The errors of every
// route tried are returned when none succeeded.
func composeByRoutes(routes []mailRoute, subject, body, attachment string) error {
	var errs []error
	for _, route := range routes {
		err := route(subject, body, attachment)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
		var stop mailRouteStop
		if errors.As(err, &stop) {
			break
		}
	}
	return errors.Join(errs...)
}

// portalEmailResponse is what the Email portal's Response code means for the
// compose (org.freedesktop.portal.Request): 0, a compose was opened; 2, the
// backend had nothing to open one with — what the GTK backend answers with
// no mailto: handler — so the next route is tried; anything else, 1 the
// reader cancelling among them, ends the compose.
func portalEmailResponse(code uint32) error {
	switch code {
	case 0:
		return nil
	case 2:
		return errors.New("the Email portal answered 2: the desktop has nothing to compose with")
	}
	return mailRouteStop{fmt.Errorf("the Email portal answered %d", code)}
}

// mailHandler is the desktop's mailto: handler as the app can see it.
type mailHandler int

const (
	// mailHandlerUnknown: there was nothing to ask (no xdg-mime), or the
	// answer could not be read.
	mailHandlerUnknown mailHandler = iota
	// mailHandlerNone: the desktop names no handler.
	mailHandlerNone
	// mailHandlerOther: a handler that is not known to be a mail client —
	// a web browser, which is what a stock Ubuntu desktop names, or an entry
	// that could not be found.
	mailHandlerOther
	// mailHandlerMailClient: a desktop entry in the Email category that is
	// not also a web browser.
	mailHandlerMailClient
)

// mailHandlerOf classifies the handler whose desktop file ID the desktop
// named: "" is none; otherwise the first applications/<id> under dataDirs
// (the XDG data directories, the user's first) decides.
func mailHandlerOf(id string, dataDirs []string) mailHandler {
	id = strings.TrimSpace(id)
	if id == "" {
		return mailHandlerNone
	}
	for _, dir := range dataDirs {
		if dir == "" {
			continue
		}
		f, err := os.Open(filepath.Join(dir, "applications", id))
		if err != nil {
			continue
		}
		mail := desktopEntryIsMailClient(f)
		f.Close()
		if mail {
			return mailHandlerMailClient
		}
		return mailHandlerOther
	}
	return mailHandlerOther
}

// desktopEntryIsMailClient reads a .desktop file's [Desktop Entry] group and
// reports whether its Categories name Email and not WebBrowser.
func desktopEntryIsMailClient(entry io.Reader) bool {
	sc := bufio.NewScanner(entry)
	inEntry := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			inEntry = line == "[Desktop Entry]"
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !inEntry || !ok || strings.TrimSpace(key) != "Categories" {
			continue
		}
		email, browser := false, false
		for _, c := range strings.Split(value, ";") {
			switch strings.TrimSpace(c) {
			case "Email":
				email = true
			case "WebBrowser":
				browser = true
			}
		}
		return email && !browser
	}
	return false
}

// linuxMailFacts is what the Linux probe found out about the desktop.
type linuxMailFacts struct {
	// confined: the app runs inside a snap or a Flatpak, where xdg-mime and
	// the desktop entries it can read are the sandbox's own, not the
	// desktop's.
	confined bool
	// schemeKnown: the portal answered OpenURI's SchemeSupported("mailto")
	// (portal 1.19.1 and later), and schemeSupported is its answer.
	schemeKnown, schemeSupported bool
	// portalEmail: the portal has the Email interface.
	portalEmail bool
	// xdgEmail: xdg-email is there to run.
	xdgEmail bool
	// handler: the desktop's mailto: handler, as xdg-mime names it and its
	// desktop entry describes it. Not asked inside a sandbox.
	handler mailHandler
}

// linuxMailOffered decides whether Email… is offered on Linux.
//
// Text: the portal's SchemeSupported is the desktop's own answer and is
// taken wherever there is one. Without it, a sandboxed app cannot see the
// desktop's handler, so Email… is withheld; unsandboxed, xdg-mime's handler
// decides, and with no xdg-mime to ask, Email… is withheld too. A browser
// counts as a handler for text: it opens the link with the subject and body.
//
// Image: offered only unsandboxed, where a route that takes a file exists
// (the portal's Email interface or xdg-email) and the handler is a mail
// client. Both routes end in the handler, and the GTK backend and xdg-email
// hand a browser a mailto: link with the file's path in it: the browser opens
// an empty compose, the picture dropped and the path in the link. A
// sandboxed app cannot tell a mail client from a browser, so there Email… is
// withheld for the image, as it is on Windows.
func linuxMailOffered(withAttachment bool, f linuxMailFacts) bool {
	if withAttachment {
		return !f.confined && (f.portalEmail || f.xdgEmail) && f.handler == mailHandlerMailClient
	}
	if f.schemeKnown {
		return f.schemeSupported
	}
	if f.confined {
		return false
	}
	return f.handler == mailHandlerOther || f.handler == mailHandlerMailClient
}
