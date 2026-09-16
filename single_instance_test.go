package bibletext

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
)

type forwardRecorder struct {
	mu   sync.Mutex
	got  []string
	seen chan struct{}
}

func newForwardRecorder() *forwardRecorder { return &forwardRecorder{seen: make(chan struct{}, 16)} }

func (r *forwardRecorder) deliver(raw string) {
	r.mu.Lock()
	r.got = append(r.got, raw)
	r.mu.Unlock()
	r.seen <- struct{}{}
}

func (r *forwardRecorder) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.got...)
}

func (r *forwardRecorder) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.seen:
	case <-time.After(3 * time.Second):
		t.Fatal("the primary never delivered")
	}
}

// quiet proves nothing was delivered: the channel stays empty for a moment.
func (r *forwardRecorder) quiet(t *testing.T) {
	t.Helper()
	select {
	case <-r.seen:
		t.Fatalf("delivered %q, want nothing", r.calls())
	case <-time.After(150 * time.Millisecond):
	}
}

func startPrimary(t *testing.T) (record string, rec *forwardRecorder, stop func()) {
	t.Helper()
	rec = newForwardRecorder()
	record = filepath.Join(t.TempDir(), "single-instance.test.json")
	stop, err := serveShareLinks(record, rec.deliver)
	if err != nil {
		t.Fatal(err)
	}
	return record, rec, stop
}

func recordOf(t *testing.T, path string) instanceRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec instanceRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func recordExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

const forwardLink = "https://bibletext.co.uk/web/john/3/#v16"

func TestForwardReachesThePrimary(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	if !forwardShareLink(record, forwardLink, nil) {
		t.Fatal("the forward was not acknowledged")
	}
	rec.wait(t)
	if got := rec.calls(); len(got) != 1 || got[0] != forwardLink {
		t.Fatalf("delivered %q, want exactly the forwarded link", got)
	}
	fi, err := os.Stat(record)
	if err != nil {
		t.Fatalf("the record is gone after a successful forward: %v", err)
	}
	// A POSIX mode means nothing on NTFS; there the profile's ACL protects it.
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("record mode %v, want 0600", fi.Mode().Perm())
	}
	if r := recordOf(t, record); r.PID != os.Getpid() || r.Port <= 0 {
		t.Errorf("record %+v does not name this process and a port", r)
	}
}

func TestForwardWithoutAURLJustRaises(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	granted := 0
	if !forwardShareLink(record, "", func(pid int) {
		granted++
		if pid != os.Getpid() {
			t.Errorf("grant for pid %d, want this process %d", pid, os.Getpid())
		}
	}) {
		t.Fatal("a bare raise was not acknowledged")
	}
	rec.wait(t)
	if got := rec.calls(); len(got) != 1 || got[0] != "" {
		t.Fatalf("delivered %q, want one empty (raise-only) call", got)
	}
	if granted != 1 {
		t.Errorf("the foreground grant was called %d times, want once", granted)
	}
}

// A record whose token does not match the listener's is not our primary as
// far as the forwarder can tell — it fails the proof like a squatter would —
// so it is stale: removed, no delivery, and this process runs alone.
func TestATamperedRecordIsTreatedAsStale(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(data), `"token":"`, `"token":"0`, 1)
	if err := os.WriteFile(record, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	if forwardShareLink(record, forwardLink, nil) {
		t.Fatal("a forward with the wrong token was acknowledged")
	}
	rec.quiet(t)
	if recordExists(record) {
		t.Error("the unusable record was left behind")
	}
}

// A live primary that refuses a URL still answers, so the forwarder knows a
// primary exists, exits, and leaves the record alone; the primary raises
// its window instead.
func TestPrimaryDeclinesAURLThatIsNotOursAndStaysPrimary(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	if !forwardShareLink(record, "https://example.com/web/john/3/", nil) {
		t.Fatal("a refusal from a live primary was read as no primary at all")
	}
	rec.wait(t)
	if got := rec.calls(); len(got) != 1 || got[0] != "" {
		t.Fatalf("delivered %q, want one bare raise and never the foreign URL", got)
	}
	if !recordExists(record) {
		t.Error("the live primary's record was removed")
	}
}

func TestAnOverLongLinkIsRefusedNotMistakenForADeadPrimary(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	long := forwardLink + "&n=" + strings.Repeat("a", 20<<10)
	if !forwardShareLink(record, long, nil) {
		t.Fatal("an over-long line made the forwarder think the primary was dead")
	}
	rec.wait(t)
	if got := rec.calls(); len(got) != 1 || got[0] != "" {
		t.Fatalf("delivered %q, want one bare raise", got)
	}
	if !recordExists(record) {
		t.Error("the live primary's record was removed")
	}
}

func TestPrimarySurvivesGarbage(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	c, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(recordOf(t, record).Port))
	if err != nil {
		t.Fatal(err)
	}
	// 64 KiB with no newline: the write may fail once the primary closes.
	_, _ = c.Write([]byte(strings.Repeat("x", 64<<10)))
	c.Close()
	rec.quiet(t)
	if !forwardShareLink(record, forwardLink, nil) {
		t.Fatal("the primary stopped serving after garbage")
	}
	rec.wait(t)
	if got := rec.calls(); len(got) != 1 || got[0] != forwardLink {
		t.Fatalf("delivered %q, want exactly the one legitimate link", got)
	}
}

func TestStaleRecordIsRemovedAfterAFailedHandshake(t *testing.T) {
	record, _, stop := startPrimary(t)
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	stop() // the primary is gone; its port is dead
	if err := os.WriteFile(record, data, 0o600); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if forwardShareLink(record, forwardLink, nil) {
		t.Fatal("a forward to a dead primary was acknowledged")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("gave up after %v, want within the dial deadline", d)
	}
	if recordExists(record) {
		t.Error("the stale record is still there")
	}
}

func fakePeer(t *testing.T, answer func(w io.Writer, line string) bool) (port int, lines func() []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var mu sync.Mutex
	var got []string
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					mu.Lock()
					got = append(got, strings.TrimRight(line, "\n"))
					mu.Unlock()
					if !answer(c, line) {
						return
					}
				}
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), got...)
	}
}

func TestForwardGivesUpOnASilentPeer(t *testing.T) {
	port, _ := fakePeer(t, func(io.Writer, string) bool { return true }) // reads, never replies
	record := filepath.Join(t.TempDir(), "single-instance.test.json")
	if err := writeInstanceRecord(record, instanceRecord{Port: port, Token: "abc", PID: 1}); err != nil {
		t.Fatal(err)
	}
	done := make(chan bool, 1)
	go func() { done <- forwardShareLink(record, forwardLink, nil) }()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("a silent peer was taken for a primary")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the forwarder hung on a silent peer")
	}
	if recordExists(record) {
		t.Error("the record naming a silent port was kept")
	}
}

// A process squatting a stale record's port cannot read the token, so it
// cannot answer the challenge — and it never sees the link: the URL is sent
// only after the proof.
func TestASquatterThatCannotProveTheTokenNeverSeesTheLink(t *testing.T) {
	port, lines := fakePeer(t, func(w io.Writer, _ string) bool {
		_, _ = io.WriteString(w, strings.Repeat("0", 64)+"\n") // a guess at the proof
		return true
	})
	record := filepath.Join(t.TempDir(), "single-instance.test.json")
	if err := writeInstanceRecord(record, instanceRecord{Port: port, Token: "real-token", PID: 1}); err != nil {
		t.Fatal(err)
	}
	if forwardShareLink(record, forwardLink, func(int) { t.Error("the foreground was granted to a squatter") }) {
		t.Fatal("a squatter was taken for the primary")
	}
	for _, l := range lines() {
		if strings.Contains(l, "real-token") || strings.Contains(l, forwardLink) {
			t.Fatalf("the squatter received %q", l)
		}
	}
	if got := lines(); len(got) != 1 || !strings.HasPrefix(got[0], singleInstanceGreeting+" ") {
		t.Errorf("the squatter received %q, want only the greeting", got)
	}
	if recordExists(record) {
		t.Error("the squatted record was kept")
	}
}

// Two processes starting together: only one exclusive create succeeds, the
// other is told to forward — never two primaries on one record.
func TestTwoPrimariesCannotShareARecord(t *testing.T) {
	record, rec, stop := startPrimary(t)
	defer stop()
	second := newForwardRecorder()
	if _, err := serveShareLinks(record, second.deliver); err == nil {
		t.Fatal("a second primary took the same record")
	} else if err != errRecordExists {
		t.Fatalf("second serve failed with %v, want errRecordExists", err)
	}
	if !forwardShareLink(record, forwardLink, nil) {
		t.Fatal("the first primary stopped answering")
	}
	rec.wait(t)
	second.quiet(t)
	stop()
	if _, err := serveShareLinks(record, second.deliver); err != nil {
		t.Fatalf("after the first primary stopped, a new one could not start: %v", err)
	}
}

func TestRecordNameSeparatesChannelsAndMimicTargets(t *testing.T) {
	store := singleInstanceRecordName("bibletext", "", "store")
	direct := singleInstanceRecordName("bibletext", "", "direct")
	if store == direct {
		t.Error("the Store and direct-download channels share a record")
	}
	if singleInstanceRecordName("bibletext", "windows", "direct") == singleInstanceRecordName("bibletext", "linux", "direct") {
		t.Error("two mimic targets share a record")
	}
	if singleInstanceRecordName("bibletext", "", "direct") == singleInstanceRecordName("bibletext.devnotes", "", "direct") {
		t.Error("a dev profile shares the real profile's record")
	}
	if !strings.HasPrefix(store, "single-instance.") || !strings.HasSuffix(store, ".json") {
		t.Errorf("record name %q", store)
	}
}

// The whole path a second process takes on Windows or Linux, minus the OS:
// forward → primary → HandleShareLink (or the browser for a decline, or
// nothing for an echo) → raise.
func TestForwardEndToEndThroughHandleShareLink(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	bd := NewBibleData()
	bd.PopulateWithSampleVerses()
	st := &AppState{
		Bible: bd, CurrentBook: "Genesis", CurrentChapter: 1,
		CurrentVersion: "web", loadPhase: loadReady,
	}
	opened := stubBrowser(t)
	guard := &browserEchoGuard{now: time.Now}
	prevGuard := browserEcho
	browserEcho = guard
	defer func() { browserEcho = prevGuard }()

	record := filepath.Join(t.TempDir(), "single-instance.test.json")
	delivered := make(chan struct{}, 8)
	stop, err := serveShareLinks(record, func(raw string) {
		singleInstanceDeliver(st, raw) // the listener's callback, minus fyne.Do
		delivered <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	wait := func() {
		select {
		case <-delivered:
		case <-time.After(3 * time.Second):
			t.Fatal("nothing delivered")
		}
	}

	if !forwardShareLink(record, forwardLink, nil) {
		t.Fatal("forward refused")
	}
	wait()
	if st.CurrentBook != "John" || st.CurrentChapter != 3 || st.hlLo() != 16 {
		t.Fatalf("after the forward: %s %d verse %d", st.CurrentBook, st.CurrentChapter, st.hlLo())
	}

	if !forwardShareLink(record, "https://bibletext.co.uk/web/john/", nil) {
		t.Fatal("a book index (ours, not a passage) was not acknowledged")
	}
	wait()
	if got := *opened; len(got) != 1 || got[0] != "https://bibletext.co.uk/web/john/" {
		t.Fatalf("the declined link was not sent to the browser: %q", got)
	}

	guard.note("https://bibletext.co.uk/web/psalms/23/#v1")
	if !forwardShareLink(record, "https://bibletext.co.uk/web/psalms/23/#v1", nil) {
		t.Fatal("an echo was not acknowledged (it must be, and dropped)")
	}
	wait()
	if st.CurrentBook != "John" {
		t.Errorf("an echoed URL moved the reader to %s", st.CurrentBook)
	}
}

// The listener's build gate is the Mac App Store sandbox's line: it must read
// exactly this, or a release build could start listening.
func TestSingleInstanceWiringIsGatedToTheDesktopsThatNeedIt(t *testing.T) {
	for file, want := range map[string]string{
		"single_instance_on.go":  "//go:build windows || linux || bibletextdev",
		"single_instance_off.go": "//go:build !windows && !linux && !bibletextdev",
	} {
		src := readSourceFile(t, file)
		if first := strings.SplitN(src, "\n", 2)[0]; first != want {
			t.Errorf("%s first line = %q, want %q", file, first, want)
		}
	}
}

// stubBrowser keeps every test away from a real browser: the direct launch
// (Windows) fails so the fallback runs, and the fallback's opener records.
func stubBrowser(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	prevOpener := externalOpener
	externalOpener = func(u *url.URL) error { opened = append(opened, u.String()); return nil }
	prevDirect := directBrowserLaunch
	directBrowserLaunch = func(string, string) error { return errNoDirectBrowser }
	t.Cleanup(func() {
		externalOpener = prevOpener
		directBrowserLaunch = prevDirect
	})
	return &opened
}
