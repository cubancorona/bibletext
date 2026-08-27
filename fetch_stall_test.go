package bibletext

// The stall-watchdog corpus client (fetch_stall.go): a download may take any
// TOTAL time as long as it keeps moving; only inactivity aborts — connect,
// headers, and every body read under one window. The first test's elapsed-
// time control is what proves the old behaviour is gone: the transfer runs
// several windows long and still succeeds, which a flat deadline of the same
// length could not do.

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStallClientSurvivesSlowButMovingDownload(t *testing.T) {
	const chunks = 30
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl := w.(http.Flusher)
		for i := 0; i < chunks; i++ {
			if _, err := w.Write([]byte("x")); err != nil {
				return
			}
			fl.Flush()
			time.Sleep(30 * time.Millisecond)
		}
	}))
	defer srv.Close()

	c := stallGuardClient{inner: &http.Client{}, stall: 150 * time.Millisecond}
	start := time.Now()
	body, err := fetchWithRetry(c, srv.URL, 1)
	if err != nil {
		t.Fatalf("slow-but-moving download must succeed: %v", err)
	}
	if len(body) != chunks {
		t.Fatalf("body = %d bytes, want %d", len(body), chunks)
	}
	// The control that can fail: the transfer ran MANY stall windows long. A
	// flat 150ms deadline would have killed it; only an inactivity deadline
	// lets it finish.
	if el := time.Since(start); el < 4*150*time.Millisecond {
		t.Fatalf("transfer finished in %v — too fast to prove the watchdog resets", el)
	}
}

func TestStallClientAbortsOnMidBodyStall(t *testing.T) {
	stall := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("start"))
		w.(http.Flusher).Flush()
		<-stall // never send another byte
	}))
	defer srv.Close()
	defer close(stall) // LIFO: release the handler before srv.Close waits on it

	c := stallGuardClient{inner: &http.Client{}, stall: 120 * time.Millisecond}
	start := time.Now()
	if _, err := fetchWithRetry(c, srv.URL, 1); err == nil {
		t.Fatal("a mid-body stall must abort with an error")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("stall abort took %v — the watchdog is not bounding the wait", el)
	}
}

func TestStallClientAbortsWhenHeadersNeverArrive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	conns := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns <- c // hold the connection open, answer nothing
		}
	}()
	defer func() {
		for {
			select {
			case c := <-conns:
				c.Close()
			default:
				return
			}
		}
	}()

	c := stallGuardClient{inner: &http.Client{}, stall: 120 * time.Millisecond}
	start := time.Now()
	if _, err := fetchWithRetry(c, "http://"+ln.Addr().String(), 1); err == nil {
		t.Fatal("a server that never answers must abort with an error")
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("headers-never-arrive abort took %v", el)
	}
}

// The picker's not-silent notice for a pending text update.
func TestFullPendingNotice(t *testing.T) {
	if got := fullPendingNotice(nil); got != "" {
		t.Errorf("nil state: %q", got)
	}
	if got := fullPendingNotice(&AppState{}); got != "" {
		t.Errorf("nothing pending: %q", got)
	}
	dl := &AppState{fullPending: true, fullDownloading: true}
	if got := fullPendingNotice(dl); got == "" || !containsAll(got, "updating", "previous edition") {
		t.Errorf("downloading notice wrong: %q", got)
	}
	wait := &AppState{fullPending: true, fullRetryDelay: 20 * time.Second}
	if got := fullPendingNotice(wait); got == "" || !containsAll(got, "waiting for a connection", "retries automatically") {
		t.Errorf("retry-wait notice wrong: %q", got)
	}
	seed := &AppState{fullPending: true, seedOnly: true}
	if got := fullPendingNotice(seed); got == "" || !containsAll(got, "starter portion") {
		t.Errorf("seed notice wrong: %q", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
			return false
		}
	}
	return true
}
