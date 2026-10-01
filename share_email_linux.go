//go:build linux && !android

package bibletext

// Email… on Linux: the xdg-desktop-portal Email interface first, then
// xdg-email, then a mailto: link. The portal is the desktop's own route — it
// fills in the subject, the body and an attachment, works from inside the
// snap's confinement, and is what GTK itself uses.
//
// Only the GTK and KDE backends provide the interface. With no mail client
// set up the GTK backend shows nothing and answers 2 on the request, and with
// one it hands the desktop's mailto: handler a mailto: link, the attachment
// as a path in it, which only a mail client reads: a browser opens an empty
// compose. So the button is offered only as linuxMailOffered
// (share_email.go) decides from what shareEmailAvailable finds out here, and
// a compose the desktop answers 2 to falls through to the next route, which
// then does as little. Every call here blocks on the bus or a process and
// runs off the UI goroutine (share_email.go).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalDest         = "org.freedesktop.portal.Desktop"
	portalPath         = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalEmailIface   = "org.freedesktop.portal.Email"
	portalOpenURIIface = "org.freedesktop.portal.OpenURI"
	portalRequestIface = "org.freedesktop.portal.Request"
	// portalAnswerWait is how long a call to the portal may take before it
	// is given up on, and how long the compose waits for its request's
	// Response: the Email portal answers as soon as the mail client is
	// launched, not when the mail is sent.
	portalAnswerWait = 15 * time.Second
	portalProbeWait  = 3 * time.Second
)

// shareEmailAvailable reports whether Email… has somewhere to go
// (linuxMailOffered).
func shareEmailAvailable(withAttachment bool) bool {
	return linuxMailOffered(withAttachment, linuxMailFactsNow())
}

// linuxMailFactsNow asks the desktop what linuxMailOffered needs: the
// portal, over the session bus, whether anything handles mailto:
// (OpenURI.SchemeSupported, portal 1.19.1 and later) and whether it has the
// Email interface; and, outside a sandbox, xdg-email's presence and
// xdg-mime's mailto: handler, the way the GTK backend finds it (the default
// application for the scheme), with its desktop entry read for what it is.
func linuxMailFactsNow() linuxMailFacts {
	f := linuxMailFacts{confined: linuxSandboxed(), handler: mailHandlerUnknown}
	ctx, cancel := context.WithTimeout(context.Background(), portalProbeWait)
	defer cancel()
	if conn, err := dbus.ConnectSessionBus(); err == nil {
		defer conn.Close()
		obj := conn.Object(portalDest, portalPath)
		call := obj.CallWithContext(ctx, portalOpenURIIface+".SchemeSupported", 0, "mailto", map[string]dbus.Variant{})
		f.schemeKnown = call.Err == nil && call.Store(&f.schemeSupported) == nil
		var xml string
		call = obj.CallWithContext(ctx, "org.freedesktop.DBus.Introspectable.Introspect", 0)
		f.portalEmail = call.Err == nil && call.Store(&xml) == nil && strings.Contains(xml, `name="`+portalEmailIface+`"`)
	}
	if f.confined {
		return f
	}
	_, err := exec.LookPath("xdg-email")
	f.xdgEmail = err == nil
	if xdgMime, err := exec.LookPath("xdg-mime"); err == nil {
		if out, err := exec.Command(xdgMime, "query", "default", "x-scheme-handler/mailto").Output(); err == nil {
			f.handler = mailHandlerOf(string(out), xdgDataDirs())
		}
	}
	return f
}

// linuxSandboxed reports whether the app runs inside a snap or a Flatpak.
func linuxSandboxed() bool {
	if os.Getenv("SNAP") != "" {
		return true
	}
	_, err := os.Stat("/.flatpak-info")
	return err == nil
}

// xdgDataDirs is where desktop entries are looked for, the user's own first:
// $XDG_DATA_HOME, then $XDG_DATA_DIRS, with the specification's defaults.
func xdgDataDirs() []string {
	home := os.Getenv("XDG_DATA_HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(h, ".local", "share")
		}
	}
	dirs := os.Getenv("XDG_DATA_DIRS")
	if dirs == "" {
		dirs = "/usr/local/share:/usr/share"
	}
	return append([]string{home}, strings.Split(dirs, ":")...)
}

// composeShareEmail opens a new message carrying subject, body and, when
// given, attachment, by the first route that succeeds: the Email portal,
// xdg-email, a mailto: link (composeByRoutes).
func composeShareEmail(subject, body, attachment string) error {
	return composeByRoutes([]mailRoute{
		composeEmailViaPortal,
		composeEmailViaXDGEmail,
		composeEmailViaMailto,
	}, mailSubjectLine(subject), body, attachment)
}

// composeEmailViaPortal calls org.freedesktop.portal.Email.ComposeEmail on
// the session bus and waits for its request's Response (portalRequest,
// share_portal_linux.go). The answer means what portalEmailResponse says.
// Once the portal has taken the call, no answer in time ends the compose
// too: a slow portal may yet open one, and a second from the next route
// would make two.
func composeEmailViaPortal(subject, body, attachment string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

	options := map[string]dbus.Variant{
		"subject": dbus.MakeVariant(subject),
		"body":    dbus.MakeVariant(body),
	}
	if attachment != "" {
		if !conn.SupportsUnixFDs() {
			return errors.New("the session bus does not carry file descriptors")
		}
		f, err := os.Open(attachment)
		if err != nil {
			return err
		}
		defer f.Close()
		options["attachment_fds"] = dbus.MakeVariant([]dbus.UnixFD{dbus.UnixFD(f.Fd())})
	}
	code, taken, err := portalRequest(conn, portalEmailIface+".ComposeEmail", options, portalAnswerWait, "")
	switch {
	case err != nil && taken:
		return mailRouteStop{fmt.Errorf("the Email portal: %w", err)}
	case err != nil:
		return err
	}
	return portalEmailResponse(code)
}

// composeEmailViaXDGEmail runs xdg-email, which finds the desktop's mail
// client itself and passes an attachment to the clients it knows. It exits
// non-zero, without opening anything, when there is no client.
func composeEmailViaXDGEmail(subject, body, attachment string) error {
	path, err := exec.LookPath("xdg-email")
	if err != nil {
		return err
	}
	args := []string{"--subject", subject, "--body", body}
	if attachment != "" {
		args = append(args, "--attach", attachment)
	}
	return exec.Command(path, args...).Run()
}

// composeEmailViaMailto opens a mailto: link the way every other link the
// app opens is opened (external_link.go): xdg-open here. A link carries no
// file, so the image share does not take this route.
func composeEmailViaMailto(subject, body, attachment string) error {
	if attachment != "" {
		return errors.New("a mailto: link carries no attachment")
	}
	return externalOpener(mailtoURL(subject, body))
}
