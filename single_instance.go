package bibletext

// One window per reader on Windows and Linux.
//
// Neither OS single-instances an application the way LaunchServices does on
// the Mac: every link launched through the shell, and every second click on
// the icon, starts a NEW BibleText.exe. Two live instances would take turns
// rewriting the same preferences file — the notes and the reading position
// with it — so the second process must hand its link to the first and leave.
//
// The handoff is a loopback TCP connection with a per-run secret. The running
// instance (the primary) listens on 127.0.0.1 at an ephemeral port and
// records port, token and pid in a small file under the user's own cache
// directory, created EXCLUSIVELY: two processes starting together cannot
// both become primaries, because only one O_EXCL create succeeds and the
// other finds the record and forwards. A new process reads the record and
// runs a two-step handshake:
//
//	→ BTLINK/2 <nonce>
//	← HMAC-SHA256(token, nonce)      the primary proves it holds the token
//	→ <token> <url>  (or "-")        the forwarder proves it too
//	← ok | no                        delivered, or refused (not ours, too long)
//
// A squatter on a stale port cannot answer the first step — the token lives
// in a file only the owner can read — so it is told nothing and the record is
// treated as stale: removed, and the new process carries on as the primary.
// Any well-formed reply after a proven handshake means a primary is alive,
// and the forwarder exits whatever the answer; the record is removed only
// when nothing answers (no file, a dead port, a silent peer, a wrong proof).
// Stdlib only: a named pipe would need go-winio and a window message would
// need the toolkit's window handle before the toolkit has made one.
//
// What a same-user process can make the app do through this socket: open a
// passage — the primary re-runs HandleShareLink on a URL that must be on our
// own host — which is exactly what pasting a link into Search does. One
// bounded line at a time, under a deadline, a handful at once. On Windows the
// record's protection is the profile directory's ACL (a POSIX mode means
// nothing there); on Linux it is 0600 under $XDG_RUNTIME_DIR. The gate that
// decides where this is compiled in at all — never a darwin release build,
// whose sandbox forbids a listener — is single_instance_on.go /
// single_instance_off.go.

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	singleInstanceGreeting = "BTLINK/2"
	singleInstanceDeadline = 2 * time.Second
	singleInstanceMaxLine  = 16 << 10
	singleInstanceMaxBusy  = 8 // handshakes served at once; the rest are dropped
)

// errRecordExists is serveShareLinks' answer when another process holds the
// record: the caller forwards to it instead of listening.
var errRecordExists = errors.New("another instance holds the single-instance record")

// instanceRecord is what the primary leaves for a second process to find.
type instanceRecord struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
	PID   int    `json:"pid"`
	Exe   string `json:"exe"`
}

// singleInstanceRecordName keeps the channels apart. The Store package wraps
// the very exe the direct-download zip ships, and MSIX redirects new files
// under AppData to a per-package location while READS fall through to the
// real one — so a Store forwarder could find a zip build's record but never
// the reverse. The channel in the name settles both directions; the mimic
// target and the app id keep a dev profile away from the real one.
func singleInstanceRecordName(appID, mimic, channel string) string {
	name := "single-instance." + appID
	if mimic != "" {
		name += "." + mimic
	}
	return name + "." + channel + ".json"
}

// singleInstanceRecordPath is the record for THIS build on THIS machine.
func singleInstanceRecordPath() (string, error) {
	dir, err := singleInstanceDir()
	if err != nil {
		return "", err
	}
	channel := "direct"
	if packagedWindows() {
		channel = "store"
	}
	return filepath.Join(dir, singleInstanceRecordName(devAppID("bibletext"), devMimicTarget(), channel)), nil
}

// writeInstanceRecord creates the record exclusively; errRecordExists when
// another process got there first.
func writeInstanceRecord(path string, rec instanceRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return errRecordExists
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func proof(token, nonce string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(nonce))
	return hex.EncodeToString(m.Sum(nil))
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// serveShareLinks makes this process the primary: it listens on loopback,
// writes the record, and calls deliver with each forwarded URL ("" for a
// bare "come forward"). stop closes the listener and removes the record.
func serveShareLinks(recordPath string, deliver func(raw string)) (stop func(), err error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	token, err := randomHex(32)
	if err != nil {
		ln.Close()
		return nil, err
	}
	rec := instanceRecord{
		Port:  ln.Addr().(*net.TCPAddr).Port,
		Token: token,
		PID:   os.Getpid(),
		Exe:   os.Args[0],
	}
	if err := writeInstanceRecord(recordPath, rec); err != nil {
		ln.Close()
		return nil, err
	}
	busy := make(chan struct{}, singleInstanceMaxBusy)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				time.Sleep(100 * time.Millisecond) // a transient failure (fd exhaustion): keep serving
				continue
			}
			select {
			case busy <- struct{}{}:
				go func() {
					defer func() { <-busy }()
					serveOneForward(c, token, deliver)
				}()
			default:
				c.Close() // a flood: nothing queues
			}
		}
	}()
	return func() {
		ln.Close()
		os.Remove(recordPath)
	}, nil
}

// readLine reads one line of at most singleInstanceMaxLine bytes; truncated
// reports a line that hit the limit before its newline.
func readLine(r *bufio.Reader) (line string, truncated bool, err error) {
	line, err = r.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && line != "" {
			return "", true, nil
		}
		return "", false, err
	}
	return strings.TrimRight(line, "\r\n"), false, nil
}

// serveOneForward runs the primary's side of the handshake.
func serveOneForward(c net.Conn, token string, deliver func(raw string)) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(singleInstanceDeadline))
	r := bufio.NewReader(io.LimitReader(c, singleInstanceMaxLine))
	hello, truncated, err := readLine(r)
	if err != nil || truncated {
		return
	}
	greeting, nonce, ok := strings.Cut(hello, " ")
	if !ok || greeting != singleInstanceGreeting || len(nonce) != 32 {
		return
	}
	if _, err := io.WriteString(c, proof(token, nonce)+"\n"); err != nil {
		return
	}
	req, truncated, err := readLine(r)
	if err != nil {
		return
	}
	if truncated {
		_, _ = io.WriteString(c, "no\n") // too long for any link of ours; a primary is alive all the same
		deliver("")
		return
	}
	sent, payload, ok := strings.Cut(req, " ")
	if !ok || subtle.ConstantTimeCompare([]byte(sent), []byte(token)) != 1 {
		return // the forwarder never proved itself: no answer
	}
	raw := ""
	if payload != "-" {
		u, ok := normaliseSiteURL(payload)
		if !ok {
			_, _ = io.WriteString(c, "no\n")
			deliver("")
			return
		}
		raw = u
	}
	deliver(raw)
	_, _ = io.WriteString(c, "ok\n")
}

// forwardShareLink hands raw ("" to just raise the window) to the primary the
// record names. grant, when non-nil, is called with the primary's pid once it
// has proved itself and before the URL is sent, so the primary may take the
// foreground (Windows needs the launching process to say so). true means a
// primary is alive and this process should exit; false means there is none —
// the record, if any, was stale and has been removed.
func forwardShareLink(recordPath, raw string, grant func(pid int)) bool {
	data, err := os.ReadFile(recordPath)
	if err != nil {
		return false
	}
	var rec instanceRecord
	if err := json.Unmarshal(data, &rec); err != nil || rec.Port <= 0 || rec.Port > 65535 ||
		rec.Token == "" || rec.PID <= 0 || rec.PID > math.MaxInt32 {
		os.Remove(recordPath)
		return false
	}
	stale := func() bool { os.Remove(recordPath); return false }
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(rec.Port)), singleInstanceDeadline)
	if err != nil {
		return stale()
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(singleInstanceDeadline))
	nonce, err := randomHex(16)
	if err != nil {
		return false
	}
	r := bufio.NewReader(io.LimitReader(c, 256))
	if _, err := fmt.Fprintf(c, "%s %s\n", singleInstanceGreeting, nonce); err != nil {
		return stale()
	}
	answer, truncated, err := readLine(r)
	if err != nil || truncated || subtle.ConstantTimeCompare([]byte(answer), []byte(proof(rec.Token, nonce))) != 1 {
		return stale() // nothing there, or something that is not our primary
	}
	if grant != nil {
		grant(rec.PID)
	}
	payload := raw
	if payload == "" {
		payload = "-"
	}
	if _, err := fmt.Fprintf(c, "%s %s\n", rec.Token, payload); err != nil {
		return stale()
	}
	reply, truncated, err := readLine(r)
	if err != nil || truncated {
		return stale() // it vanished mid-handshake
	}
	switch reply {
	case "ok", "no":
		return true
	}
	return stale()
}

// singleInstanceDeliver is what the primary does with a forwarded URL, on the
// UI goroutine: open it like any tapped link, send a declined one to the
// browser (I2 — an activated app has no OS fallback), drop an echo, and in
// every case bring the window forward.
func singleInstanceDeliver(state *AppState, raw string) {
	if state == nil {
		return
	}
	if raw != "" && !browserEcho.isEcho(raw) {
		if !HandleShareLink(state, raw) {
			openLinkInBrowser(raw)
		}
	}
	raiseWindow(state)
}

// raiseWindow brings the reader's window to the front: un-minimised first
// where the toolkit cannot do that itself (Windows), then focused. It must
// run after Show, on the UI goroutine — a request before the native window
// exists is silently dropped, which is why the listener's callback goes
// through fyne.Do: queued until the loop runs, never lost.
func raiseWindow(state *AppState) {
	if state == nil || state.window == nil {
		return
	}
	restoreNative(state.window)
	state.window.RequestFocus()
}
