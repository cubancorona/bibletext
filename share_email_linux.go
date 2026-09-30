//go:build linux && !android

package bibletext

// Email… on Linux: the xdg-desktop-portal Email interface first, then
// xdg-email, then a mailto: link. The portal is the desktop's own route — it
// fills in the subject, the body and an attachment, works from inside the
// snap's confinement, and is what GTK itself uses — and it is the only one
// of the three that can attach the image share's PNG through every backend.
//
// Only the GTK and KDE backends provide the interface, and with no mail
// client set up the GTK backend shows nothing and answers 2 on the request:
// so the button is offered only when the desktop says a mailto: handler
// exists, and a compose that comes back empty-handed falls through to the
// next route, which then does as little, so the reader is never shown a
// route that goes nowhere. Every call here blocks on the bus or a process
// and runs off the UI goroutine (share_email.go).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// shareEmailAvailable reports whether Email… has somewhere to go. The
// portal's own answer is taken when it can give one: OpenURI's
// SchemeSupported("mailto") (portal 1.19.1 and later) asks the desktop
// whether anything handles mailto:. On an older portal the Email interface
// is looked for, and the desktop is asked for its mailto: handler the way
// the GTK backend asks (the default application for the scheme), since the
// interface being present says nothing about a client being installed. With
// no portal at all, xdg-email needs that handler too. Only a mailto: link
// carries no file, so an attachment needs the portal or xdg-email.
func shareEmailAvailable(withAttachment bool) bool {
	ctx, cancel := context.WithTimeout(context.Background(), portalProbeWait)
	defer cancel()
	if conn, err := dbus.ConnectSessionBus(); err == nil {
		defer conn.Close()
		obj := conn.Object(portalDest, portalPath)
		var supported bool
		call := obj.CallWithContext(ctx, portalOpenURIIface+".SchemeSupported", 0, "mailto", map[string]dbus.Variant{})
		if call.Err == nil && call.Store(&supported) == nil {
			return supported
		}
		var xml string
		call = obj.CallWithContext(ctx, "org.freedesktop.DBus.Introspectable.Introspect", 0)
		if call.Err == nil && call.Store(&xml) == nil && strings.Contains(xml, `name="`+portalEmailIface+`"`) {
			return mailtoHandlerRegistered()
		}
	}
	if _, err := exec.LookPath("xdg-email"); err == nil {
		return mailtoHandlerRegistered()
	}
	return !withAttachment && mailtoHandlerRegistered()
}

// mailtoHandlerRegistered asks the desktop for its mailto: handler. Without
// xdg-mime to ask, it answers yes: the portal or xdg-email will say no
// themselves when pressed, and nothing then happens, which is the graceful
// outcome; a button withheld for want of a tool would hide a client that is
// there.
func mailtoHandlerRegistered() bool {
	xdgMime, err := exec.LookPath("xdg-mime")
	if err != nil {
		return true
	}
	out, err := exec.Command(xdgMime, "query", "default", "x-scheme-handler/mailto").Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// composeShareEmail opens a new message carrying subject, body and, when
// given, attachment, by the first route that succeeds: the Email portal,
// xdg-email, a mailto: link. The error of every route is returned when none
// does.
func composeShareEmail(subject, body, attachment string) error {
	subject = mailSubjectLine(subject)
	var errs []error
	for _, route := range []func(string, string, string) error{
		composeEmailViaPortal,
		composeEmailViaXDGEmail,
		composeEmailViaMailto,
	} {
		err := route(subject, body, attachment)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// composeEmailViaPortal calls org.freedesktop.portal.Email.ComposeEmail on
// the session bus and waits for its request's Response. The request's
// object path is known before the call from the handle token, and the
// Response signal is matched before the call is made, so an answer that
// arrives at once is not missed. Response 0 is success; 1 is the reader
// cancelling; 2 is the backend having nothing to show, which is what the
// GTK backend answers with no mail client, and is an error here so the next
// route is tried.
func composeEmailViaPortal(subject, body, attachment string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()

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

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token),
		"subject":      dbus.MakeVariant(subject),
		"body":         dbus.MakeVariant(body),
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

	ctx, cancel := context.WithTimeout(context.Background(), portalAnswerWait)
	defer cancel()
	var handle dbus.ObjectPath
	if err := conn.Object(portalDest, portalPath).
		CallWithContext(ctx, portalEmailIface+".ComposeEmail", 0, "", options).
		Store(&handle); err != nil {
		return err
	}
	for {
		select {
		case sig := <-responses:
			if sig == nil || (sig.Path != handle && sig.Path != request) {
				continue
			}
			if len(sig.Body) == 0 {
				return errors.New("the Email portal answered with no response code")
			}
			code, _ := sig.Body[0].(uint32)
			if code == 0 {
				return nil
			}
			return fmt.Errorf("the Email portal answered %d", code)
		case <-ctx.Done():
			return errors.New("no answer from the Email portal")
		}
	}
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
