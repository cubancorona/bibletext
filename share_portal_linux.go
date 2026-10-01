//go:build linux && !android

package bibletext

// One request to the desktop portal (org.freedesktop.portal.Request), the
// way Email… composes a mail (composeEmailViaPortal, share_email_linux.go)
// and the image share shows its picture in the file manager
// (revealViaPortal, share_reveal_linux.go). A portal method that asks the
// desktop to do something answers at once with a request's object path, and
// says how it went later, in that request's Response signal: 0 done, 1
// cancelled by the reader, 2 refused or failed. share_portal_linux_test.go
// holds it against a portal of its own on a private session bus.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// portalRequest calls method on the portal with args and then options, a
// handle token added to them, and waits at most wait for the Response of the
// request the call makes, returning its code. The Response is matched before
// the call is made — the request's object path is known from the
// connection's unique name and the token — so an answer that comes at once
// is not missed, and a Response on any other request's path is passed over;
// the handle the call returns is matched too, for a portal that names the
// request otherwise. taken reports whether the portal took the call: an
// error after it did is a missing or unreadable answer, and the portal may
// yet do what it was asked.
func portalRequest(conn *dbus.Conn, method string, options map[string]dbus.Variant, wait time.Duration, args ...interface{}) (code uint32, taken bool, err error) {
	token := fmt.Sprintf("bibletext%d", time.Now().UnixNano())
	names := conn.Names()
	if len(names) == 0 {
		return 0, false, errors.New("the session bus gave no unique name")
	}
	sender := strings.ReplaceAll(strings.TrimPrefix(names[0], ":"), ".", "_")
	request := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface(portalRequestIface),
		dbus.WithMatchMember("Response"),
	); err != nil {
		return 0, false, err
	}
	responses := make(chan *dbus.Signal, 8)
	conn.Signal(responses)
	defer conn.RemoveSignal(responses)

	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	options["handle_token"] = dbus.MakeVariant(token)
	var handle dbus.ObjectPath
	if err := conn.Object(portalDest, portalPath).
		CallWithContext(ctx, method, 0, append(args, options)...).
		Store(&handle); err != nil {
		return 0, false, err
	}
	for {
		select {
		case sig := <-responses:
			if sig == nil || (sig.Path != handle && sig.Path != request) {
				continue
			}
			c, ok := uint32(0), len(sig.Body) > 0
			if ok {
				c, ok = sig.Body[0].(uint32)
			}
			if !ok {
				return 0, true, errors.New("the portal answered with no response code")
			}
			return c, true, nil
		case <-ctx.Done():
			return 0, true, errors.New("no answer from the portal")
		}
	}
}
