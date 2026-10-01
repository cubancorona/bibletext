//go:build linux && !android

package bibletext

// THE DESKTOP PORTAL, AS THE APP ASKS IT AND HEARS IT (portalRequest,
// revealViaPortal, composeEmailViaPortal).
//
// Inside the snap the image share's picture is shown in the file manager
// through the portal's OpenURI.OpenDirectory, and the sheet says it is shown
// only when the portal's answer is 0; Email… composes through its Email
// interface. These hold both against a portal of the test's own, on a
// session bus of the test's own (a dbus-daemon started for it, with nothing
// else on it), which answers each call as the test chooses: 0, 1 or 2, an
// error in place of a request, an answer with no code, no answer, an answer
// before the call's own reply, and an answer on another request first. Like
// the real portal from a sandboxed app, it refuses, with 2, a descriptor
// open for writing. Each answer that is not 0 is a check that the success,
// the control, does not pass; and a writable descriptor handed to the same
// portal shows that it can tell one.
//
// They need dbus-daemon. Without it they skip, except in CI, where the
// Linux job installs it and its absence fails them.

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// privateSessionBus starts a dbus-daemon of the test's own, which starts no
// services, and points DBUS_SESSION_BUS_ADDRESS at it, so that the app's
// portal calls reach the test's portal and nothing of the desktop's.
func privateSessionBus(t *testing.T) string {
	t.Helper()
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("no dbus-daemon in CI, so the portal is not held: %v", err)
		}
		t.Skipf("no dbus-daemon to hold the portal against: %v", err)
	}
	// A socket's path has a length limit that a test's temp directory can
	// pass, so the bus gets a short one.
	dir, err := os.MkdirTemp("", "btbus")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	conf := filepath.Join(dir, "bus.conf")
	if err := os.WriteFile(conf, []byte(`<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:path=`+filepath.Join(dir, "bus")+`</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*"/>
  </policy>
</busconfig>
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(daemon, "--config-file="+conf, "--nofork", "--print-address")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})
	line, err := bufio.NewReader(out).ReadString('\n')
	addr := strings.TrimSpace(line)
	if err != nil || addr == "" {
		t.Fatalf("the test's dbus-daemon gave no address (%q, %v)", line, err)
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", addr)
	return addr
}

// fakeAnswer is how the test's portal answers a call.
type fakeAnswer struct {
	refuse bool   // an error in place of a request, as the portal's lockdown answers
	code   uint32 // the Response's code
	noCode bool   // a Response with nothing in it
	silent bool   // no Response at all
	early  bool   // the Response before the call's own reply
	decoy  bool   // first a Response of 0 on another request's path
}

// fakeCall is a call the test's portal took.
type fakeCall struct {
	method   string
	files    []string // each descriptor's file, as the portal's process sees it
	readOnly []bool   // whether each descriptor was open for reading only
	options  map[string]dbus.Variant
}

// fakePortal stands in for org.freedesktop.portal.Desktop.
type fakePortal struct {
	conn   *dbus.Conn
	mu     sync.Mutex
	answer fakeAnswer
	calls  []fakeCall
}

type fakeOpenURI struct{ p *fakePortal }
type fakeEmail struct{ p *fakePortal }

func (o fakeOpenURI) OpenDirectory(sender dbus.Sender, parent string, fd dbus.UnixFD, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	return o.p.request(sender, "OpenDirectory", []dbus.UnixFD{fd}, options)
}

func (e fakeEmail) ComposeEmail(sender dbus.Sender, parent string, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	var fds []dbus.UnixFD
	if v, ok := options["attachment_fds"]; ok {
		fds, _ = v.Value().([]dbus.UnixFD)
	}
	return e.p.request(sender, "ComposeEmail", fds, options)
}

// startFakePortal puts the test's portal on the private bus at addr, under
// the portal's own name and path.
func startFakePortal(t *testing.T, addr string) *fakePortal {
	t.Helper()
	conn, err := dbus.Connect(addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	p := &fakePortal{conn: conn}
	if err := conn.Export(fakeOpenURI{p}, portalPath, portalOpenURIIface); err != nil {
		t.Fatal(err)
	}
	if err := conn.Export(fakeEmail{p}, portalPath, portalEmailIface); err != nil {
		t.Fatal(err)
	}
	reply, err := conn.RequestName(portalDest, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("the test's portal could not take %s (%v, %v)", portalDest, reply, err)
	}
	return p
}

func (p *fakePortal) set(a fakeAnswer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answer, p.calls = a, nil
}

func (p *fakePortal) taken() []fakeCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]fakeCall(nil), p.calls...)
}

// request takes a call as the portal does: it records what came, makes the
// request's object path from the caller's unique name and the handle token,
// and answers on it as set. A descriptor open for writing is answered 2.
func (p *fakePortal) request(sender dbus.Sender, method string, fds []dbus.UnixFD, options map[string]dbus.Variant) (dbus.ObjectPath, *dbus.Error) {
	call := fakeCall{method: method, options: options}
	writable := false
	for _, fd := range fds {
		file, _ := os.Readlink("/proc/self/fd/" + strconv.Itoa(int(fd)))
		flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFL, 0)
		ro := errno == 0 && int(flags)&syscall.O_ACCMODE == syscall.O_RDONLY
		writable = writable || !ro
		call.files = append(call.files, file)
		call.readOnly = append(call.readOnly, ro)
		syscall.Close(int(fd))
	}
	p.mu.Lock()
	a := p.answer
	p.calls = append(p.calls, call)
	p.mu.Unlock()
	if a.refuse {
		return "", dbus.NewError("org.freedesktop.portal.Error.NotAllowed", []interface{}{"refused by the test"})
	}
	token, _ := options["handle_token"].Value().(string)
	base := "/org/freedesktop/portal/desktop/request/" + strings.ReplaceAll(strings.TrimPrefix(string(sender), ":"), ".", "_") + "/"
	request := dbus.ObjectPath(base + token)
	code := a.code
	if writable {
		code = 2
	}
	respond := func() {
		if a.decoy {
			p.conn.Emit(dbus.ObjectPath(base+"another"), portalRequestIface+".Response", uint32(0), map[string]dbus.Variant{})
		}
		switch {
		case a.silent:
		case a.noCode:
			p.conn.Emit(request, portalRequestIface+".Response")
		default:
			p.conn.Emit(request, portalRequestIface+".Response", code, map[string]dbus.Variant{})
		}
	}
	if a.early {
		respond()
	} else {
		go func() {
			time.Sleep(20 * time.Millisecond)
			respond()
		}()
	}
	return request, nil
}

// portalPicture is a file to hand the portal.
func portalPicture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "BibleText verse.png")
	if err := os.WriteFile(path, []byte("not a real png"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// THE FILE MANAGER IS SAID TO SHOW THE PICTURE ONLY ON THE PORTAL'S 0, to a
// request that is this one, with the picture handed as a descriptor open for
// reading only. The 0 is the control for every other answer.
func TestThePortalRevealBelievesOnlyAZeroAnswer(t *testing.T) {
	portal := startFakePortal(t, privateSessionBus(t))
	pic := portalPicture(t)
	const wait = 2 * time.Second

	portal.set(fakeAnswer{code: 0})
	if err := revealViaPortal(pic, wait); err != nil {
		t.Fatalf("control: the portal answered 0 and the reveal failed: %v", err)
	}
	calls := portal.taken()
	if len(calls) != 1 || calls[0].method != "OpenDirectory" || len(calls[0].files) != 1 {
		t.Fatalf("the portal took %+v, want one OpenDirectory with one descriptor", calls)
	}
	if calls[0].files[0] != pic {
		t.Errorf("the descriptor is %q, want the picture %q", calls[0].files[0], pic)
	}
	if !calls[0].readOnly[0] {
		t.Error("the picture was handed open for writing, which the portal refuses from a sandboxed app")
	}

	for _, c := range []struct {
		name string
		a    fakeAnswer
	}{
		{"1, cancelled", fakeAnswer{code: 1}},
		{"2, failed", fakeAnswer{code: 2}},
		{"an error in place of a request", fakeAnswer{refuse: true}},
		{"an answer with no code", fakeAnswer{noCode: true}},
		{"a 0 to another request, then 2 to this one", fakeAnswer{decoy: true, code: 2}},
	} {
		portal.set(c.a)
		if err := revealViaPortal(pic, wait); err == nil {
			t.Errorf("%s: the reveal was believed", c.name)
		}
	}

	portal.set(fakeAnswer{silent: true})
	start := time.Now()
	if err := revealViaPortal(pic, 300*time.Millisecond); err == nil {
		t.Error("no answer: the reveal was believed")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Errorf("no answer: the reveal waited %v, past its bound", waited)
	}

	portal.set(fakeAnswer{early: true, code: 0})
	if err := revealViaPortal(pic, wait); err != nil {
		t.Errorf("an answer before the call's own reply was missed: %v", err)
	}
}

// THE TEST'S PORTAL CAN TELL A WRITABLE DESCRIPTOR, so that the reveal's
// success above shows the picture went read-only: the same portal, handed
// the picture open for writing, answers 2.
func TestThePortalTellsAWritableDescriptor(t *testing.T) {
	portal := startFakePortal(t, privateSessionBus(t))
	pic := portalPicture(t)
	f, err := os.OpenFile(pic, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	portal.set(fakeAnswer{code: 0})
	code, taken, err := portalRequest(conn, portalOpenURIIface+".OpenDirectory", map[string]dbus.Variant{}, 2*time.Second, "", dbus.UnixFD(f.Fd()))
	if err != nil || !taken {
		t.Fatalf("the request failed: taken %v, %v", taken, err)
	}
	if code != 2 {
		t.Errorf("a writable descriptor was answered %d, want 2", code)
	}
	if calls := portal.taken(); len(calls) != 1 || len(calls[0].readOnly) != 1 || calls[0].readOnly[0] {
		t.Errorf("the portal saw %+v, want one descriptor open for writing", calls)
	}
}

// EMAIL… THROUGH THE PORTAL HEARS THE SAME ANSWER: 0 is a compose, carrying
// the subject and the body; 2 and an error in place of a request hand on to
// the next route; 1, and an answer with no code once the portal has taken
// the call, end the compose there (mailRouteStop). 0 is the control. The
// picture a mail attaches is not handed to the test's portal: it goes as an
// array of descriptors inside the options, and godbus, which the test's
// portal is built on, cuts the message short where it reads such an array
// (the desktop's portal is GLib's, which reads it). The descriptor itself is
// held by the reveal above, which hands one the same way.
func TestTheEmailPortalIsHeardAsItAnswers(t *testing.T) {
	portal := startFakePortal(t, privateSessionBus(t))

	portal.set(fakeAnswer{code: 0})
	if err := composeEmailViaPortal("Psalm 23:1 (Sample)", "The Lord is my shepherd", ""); err != nil {
		t.Fatalf("control: the portal answered 0 and the compose failed: %v", err)
	}
	calls := portal.taken()
	if len(calls) != 1 || calls[0].method != "ComposeEmail" {
		t.Fatalf("the portal took %+v, want one ComposeEmail", calls)
	}
	if s, _ := calls[0].options["subject"].Value().(string); s != "Psalm 23:1 (Sample)" {
		t.Errorf("the subject was %q", s)
	}
	if b, _ := calls[0].options["body"].Value().(string); b != "The Lord is my shepherd" {
		t.Errorf("the body was %q", b)
	}

	for _, c := range []struct {
		name string
		a    fakeAnswer
		stop bool
	}{
		{"2, nothing to compose with", fakeAnswer{code: 2}, false},
		{"an error in place of a request", fakeAnswer{refuse: true}, false},
		{"1, cancelled", fakeAnswer{code: 1}, true},
		{"an answer with no code", fakeAnswer{noCode: true}, true},
		{"a 0 to another request, then 2 to this one", fakeAnswer{decoy: true, code: 2}, false},
	} {
		portal.set(c.a)
		err := composeEmailViaPortal("Psalm 23:1 (Sample)", "The Lord is my shepherd", "")
		if err == nil {
			t.Errorf("%s: the compose was believed", c.name)
			continue
		}
		var stop mailRouteStop
		if got := errors.As(err, &stop); got != c.stop {
			t.Errorf("%s: ends the compose %v, want %v (%v)", c.name, got, c.stop, err)
		}
	}
}
