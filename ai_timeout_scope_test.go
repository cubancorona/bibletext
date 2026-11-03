package bibletext

// What counts as the provider taking too long. A request written whole and
// not answered before its deadline, or within the transport's wait for the
// response headers, is reported once, as the provider's timeout. A
// connection never made, whatever stopped it, and one lost once made keep
// the retries and the connection line they had in 1.2.24. These run
// doAIRequest on real transports against listeners and servers in the
// process, over HTTP/1.1 and HTTP/2, so each case is judged on the error the
// transport itself returns: a deadline that expires during the TLS handshake
// and one that expires after the request was written reach the caller as the
// same error, and a dial timeout satisfies
// errors.Is(err, context.DeadlineExceeded) as a deadline does. They use only
// doAIRequest, friendlyAIError and aiRetrySleep, which 1.2.24 has too, so the
// same file runs against that tree.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// aiConnectionLine is the line 1.2.24 shows for a request that never reached
// the provider.
const aiConnectionLine = "Couldn't reach the AI service. Check your connection and try again."

// aiNoRetryWait takes the wait out of doAIRequest's retries.
func aiNoRetryWait(t *testing.T) {
	t.Helper()
	previous := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { aiRetrySleep = previous })
}

// aiScopeRequest is a request with a body, under ctx, as a provider client
// builds one.
func aiScopeRequest(t *testing.T, ctx context.Context, address string) *http.Request {
	t.Helper()
	return aiScopeRequestOf(t, ctx, address, []byte(`{"synthetic":"question"}`))
}

// aiScopeRequestOf is the same request with body as its body.
func aiScopeRequestOf(t *testing.T, ctx context.Context, address string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// aiMuteConn fails every write once mute is set, with nothing sent, as a
// pooled connection whose far side has reset it does.
type aiMuteConn struct {
	net.Conn
	mute *atomic.Bool
}

func (c aiMuteConn) Write(b []byte) (int, error) {
	if c.mute.Load() {
		return 0, &net.OpError{Op: "write", Net: "tcp", Source: c.LocalAddr(), Addr: c.RemoteAddr(),
			Err: os.NewSyscallError("write", syscall.EPIPE)}
	}
	return c.Conn.Write(b)
}

// aiScopeListener accepts connections on the loopback address and hands each
// to serve, counting them.
func aiScopeListener(t *testing.T, serve func(net.Conn)) (address string, accepted *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted = new(atomic.Int32)
	done := make(chan struct{})
	var mu sync.Mutex
	var open []net.Conn
	go func() {
		defer close(done)
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			mu.Lock()
			open = append(open, c)
			mu.Unlock()
			go serve(c)
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		<-done
		mu.Lock()
		defer mu.Unlock()
		for _, c := range open {
			c.Close()
		}
	})
	return ln.Addr().String(), accepted
}

// aiDeadline is a request's context whose deadline passes when pass is
// called, at a chosen point in the request rather than after a wait the
// machine's load could outrun: the transport sees a deadline pass there.
type aiDeadline struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func newAIDeadline() *aiDeadline {
	return &aiDeadline{Context: context.Background(), done: make(chan struct{})}
}

func (d *aiDeadline) pass()                 { d.once.Do(func() { close(d.done) }) }
func (d *aiDeadline) Done() <-chan struct{} { return d.done }

func (d *aiDeadline) Err() error {
	select {
	case <-d.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

// aiAfterHeaders runs then once a response's headers are in, so what it
// does, letting the deadline pass or losing the connection, happens inside
// the answer.
type aiAfterHeaders struct {
	inner http.RoundTripper
	then  func()
}

func (a aiAfterHeaders) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := a.inner.RoundTrip(r)
	if err == nil {
		a.then()
	}
	return resp, err
}

func (a aiAfterHeaders) CloseIdleConnections() {
	if inner, ok := a.inner.(interface{ CloseIdleConnections() }); ok {
		inner.CloseIdleConnections()
	}
}

// aiLossyNet dials real connections whose reads, once the network is lost,
// fail as read(2) does when TCP gives up on its peer: with errno, in the
// shape the net package gives its own read errors. lose loses every
// connection open at the time; one dialled later starts sound, so every
// attempt reaches the server.
type aiLossyNet struct {
	errno syscall.Errno
	mu    sync.Mutex
	conns []*aiLossyConn
}

type aiLossyConn struct {
	net.Conn
	errno syscall.Errno
	lost  atomic.Bool
}

func (n *aiLossyNet) dial(ctx context.Context, network, address string) (net.Conn, error) {
	c, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	lossy := &aiLossyConn{Conn: c, errno: n.errno}
	n.mu.Lock()
	n.conns = append(n.conns, lossy)
	n.mu.Unlock()
	return lossy, nil
}

func (n *aiLossyNet) lose() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, c := range n.conns {
		c.lost.Store(true)
		_ = c.Conn.SetReadDeadline(time.Now()) // wakes a read under way
	}
}

func (c *aiLossyConn) Read(b []byte) (int, error) {
	if !c.lost.Load() {
		n, err := c.Conn.Read(b)
		if err == nil || !c.lost.Load() {
			return n, err
		}
	}
	return 0, &net.OpError{Op: "read", Net: "tcp", Source: c.LocalAddr(), Addr: c.RemoteAddr(),
		Err: os.NewSyscallError("read", c.errno)}
}

// aiCountedDial counts the connections a transport asks dial for.
func aiCountedDial(dial func(context.Context, string, string) (net.Conn, error)) (func(context.Context, string, string) (net.Conn, error), *atomic.Int32) {
	dials := new(atomic.Int32)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		return dial(ctx, network, address)
	}, dials
}

// A connection never made keeps 1.2.24's behaviour: three attempts while
// the budget lasts, one when the budget runs out first, and the connection
// line. The dial timeout, the TLS handshake timeout, the DNS timeout and
// both expiries of the request's deadline before a connection were the
// provider's timeout in the code before, sent once and told as taking too
// long to answer. So, later, was a dial timeout after a pooled connection
// that sent nothing: the transport takes the request into its buffer,
// reports it written, finds the connection dead with nothing sent and dials
// again by itself, and the write it reported stood for the dial that never
// connected. A deadline that passes while the request is being written,
// the transport reporting the write failed, is no write either.
func TestAIConnectionsNeverMadeKeepTheirRetriesAndLine(t *testing.T) {
	aiNoRetryWait(t)
	silent := func(c net.Conn) { _, _ = io.Copy(io.Discard, c) }
	reset := func(c net.Conn) {
		buf := make([]byte, 4096)
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = c.Read(buf) // the request reaches the server, unanswered
		if tcp, ok := c.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
		c.Close()
	}
	failingDial := func(err error) func(context.Context, string, string) (net.Conn, error) {
		return func(context.Context, string, string) (net.Conn, error) { return nil, err }
	}
	for _, tc := range []struct {
		name     string
		setup    func(t *testing.T, deadline *aiDeadline) (address string, tr *http.Transport, attempts *atomic.Int32)
		deadline bool // the request's deadline passes before the request is written
		attempts int32
		isExpiry bool // control: the error satisfies errors.Is(err, context.DeadlineExceeded)
		body     int  // the size of a body in place of the question, when one is needed
	}{
		{name: "a dial timeout", attempts: 3, isExpiry: true,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				dial, dials := aiCountedDial((&net.Dialer{Timeout: time.Nanosecond}).DialContext)
				return "http://127.0.0.1:9/v1/synthetic", &http.Transport{DialContext: dial}, dials
			}},
		{name: "a connection refused", attempts: 3,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				address := ln.Addr().String()
				ln.Close()
				dial, dials := aiCountedDial((&net.Dialer{}).DialContext)
				return "http://" + address + "/v1/synthetic", &http.Transport{DialContext: dial}, dials
			}},
		{name: "a TLS handshake timeout", attempts: 3,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				address, accepted := aiScopeListener(t, silent)
				return "https://" + address + "/v1/synthetic", &http.Transport{TLSHandshakeTimeout: 50 * time.Millisecond}, accepted
			}},
		{name: "a reset before any response byte", attempts: 3,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				address, accepted := aiScopeListener(t, reset)
				return "http://" + address + "/v1/synthetic", &http.Transport{}, accepted
			}},
		{name: "a host name not found", attempts: 3,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				dial, dials := aiCountedDial(failingDial(&net.OpError{Op: "dial", Net: "tcp",
					Err: &net.DNSError{Err: "no such host", Name: "synthetic.invalid", IsNotFound: true}}))
				return "http://synthetic.invalid/v1/synthetic", &http.Transport{DialContext: dial}, dials
			}},
		{name: "a DNS lookup timeout", attempts: 3,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				dial, dials := aiCountedDial(failingDial(&net.OpError{Op: "dial", Net: "tcp",
					Err: &net.DNSError{Err: "i/o timeout", Name: "synthetic.invalid", IsTimeout: true}}))
				return "http://synthetic.invalid/v1/synthetic", &http.Transport{DialContext: dial}, dials
			}},
		{name: "the request's deadline while connecting", deadline: true, attempts: 1, isExpiry: true,
			setup: func(t *testing.T, deadline *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				dial, dials := aiCountedDial(func(ctx context.Context, _, _ string) (net.Conn, error) {
					deadline.pass() // the dial is under way, and never connects
					<-ctx.Done()
					return nil, ctx.Err()
				})
				return "http://127.0.0.1:9/v1/synthetic", &http.Transport{DialContext: dial}, dials
			}},
		{name: "the request's deadline during the TLS handshake", deadline: true, attempts: 1, isExpiry: true,
			setup: func(t *testing.T, deadline *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				address, accepted := aiScopeListener(t, func(c net.Conn) {
					deadline.pass() // connected, and the handshake never completes
					silent(c)
				})
				return "https://" + address + "/v1/synthetic", &http.Transport{}, accepted
			}},
		{name: "the request's deadline while the request is written", deadline: true, attempts: 1, isExpiry: true, body: 32 << 20,
			setup: func(t *testing.T, deadline *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				stop := make(chan struct{})
				t.Cleanup(func() { close(stop) })
				address, accepted := aiScopeListener(t, func(c net.Conn) {
					// The request begins to arrive; nothing more is read, so
					// a body larger than the connection's buffers is never
					// written whole.
					_, _ = io.ReadFull(c, make([]byte, 512))
					deadline.pass()
					<-stop
				})
				return "http://" + address + "/v1/synthetic", &http.Transport{}, accepted
			}},
		{name: "a pooled connection that sends nothing, then a dial timeout", attempts: 3, isExpiry: true,
			setup: func(t *testing.T, _ *aiDeadline) (string, *http.Transport, *atomic.Int32) {
				served := new(atomic.Int32)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					served.Add(1)
					_, _ = io.ReadAll(r.Body)
					_, _ = w.Write([]byte(`{}`))
				}))
				t.Cleanup(srv.Close)
				mute := new(atomic.Bool)
				redials := new(atomic.Int32)
				tr := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
					if !mute.Load() {
						c, err := (&net.Dialer{}).DialContext(ctx, network, address)
						if err != nil {
							return nil, err
						}
						return aiMuteConn{Conn: c, mute: mute}, nil
					}
					redials.Add(1)
					return (&net.Dialer{Timeout: time.Nanosecond}).DialContext(ctx, network, address)
				}}
				// The first request is answered, and its connection waits in
				// the pool for the next.
				address := srv.URL + "/v1/synthetic"
				if _, err := doAIRequest(&http.Client{Transport: tr}, aiScopeRequest(t, context.Background(), address)); err != nil {
					t.Fatalf("control: the first request failed: %v", err)
				}
				mute.Store(true)
				t.Cleanup(func() {
					if got := served.Load(); got != 1 {
						t.Errorf("control: the server saw %d requests, not the first alone", got)
					}
				})
				return address, tr, redials
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deadline := newAIDeadline()
			address, tr, attempts := tc.setup(t, deadline)
			t.Cleanup(tr.CloseIdleConnections)
			var ctx context.Context = context.Background()
			if tc.deadline {
				ctx = deadline
			}
			req := aiScopeRequest(t, ctx, address)
			if tc.body > 0 {
				req = aiScopeRequestOf(t, ctx, address, make([]byte, tc.body))
			}
			_, err := doAIRequest(&http.Client{Transport: tr}, req)
			if err == nil {
				t.Fatal("control: the request succeeded")
			}
			if tc.isExpiry && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("control: %v does not satisfy errors.Is(err, context.DeadlineExceeded)", err)
			}
			if got := friendlyAIError(err); got != aiConnectionLine {
				t.Errorf("the reader is told %q, not the connection line", got)
			}
			if got := attempts.Load(); got != tc.attempts {
				t.Errorf("the request was tried %d times, not %d", got, tc.attempts)
			}
		})
	}
}

// A connection lost once the provider has the whole request keeps 1.2.24's
// handling too, its attempts and the connection line, as a reset there
// always did. TCP that gives up on a peer which stopped answering, its
// keep-alive probes or retransmissions unanswered, reports ETIMEDOUT, a
// net.Error whose Timeout is true; the code before told that, after the
// write, as the provider taking too long, after one attempt, beside the
// faster model. Over HTTP/1.1 and HTTP/2, before any response and inside an
// answer that began; the same loss reported as a reset is the control.
func TestAIConnectionsLostAfterTheWriteKeepTheirRetriesAndLine(t *testing.T) {
	aiNoRetryWait(t)
	for _, tc := range []struct {
		name   string
		errno  syscall.Errno
		h2     bool
		inBody bool
	}{
		{name: "timed out before any response, HTTP/1.1", errno: syscall.ETIMEDOUT},
		{name: "timed out before any response, HTTP/2", errno: syscall.ETIMEDOUT, h2: true},
		{name: "timed out inside the answer, HTTP/1.1", errno: syscall.ETIMEDOUT, inBody: true},
		{name: "timed out inside the answer, HTTP/2", errno: syscall.ETIMEDOUT, h2: true, inBody: true},
		{name: "reset before any response, HTTP/1.1", errno: syscall.ECONNRESET},
		{name: "reset before any response, HTTP/2", errno: syscall.ECONNRESET, h2: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			network := &aiLossyNet{errno: tc.errno}
			served, http2 := new(atomic.Int32), new(atomic.Int32)
			release := make(chan struct{})
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.ProtoMajor == 2 {
					http2.Add(1)
				}
				if _, err := io.ReadAll(r.Body); err != nil {
					return
				}
				served.Add(1) // the whole request has arrived
				if tc.inBody {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"answer":"synthetic`))
					w.(http.Flusher).Flush()
				} else {
					network.lose()
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			var tr *http.Transport
			if tc.h2 {
				srv.EnableHTTP2 = true
				srv.StartTLS()
				tr = srv.Client().Transport.(*http.Transport).Clone()
			} else {
				srv.Start()
				tr = &http.Transport{}
			}
			t.Cleanup(srv.Close)
			t.Cleanup(func() { close(release) })
			tr.DialContext = network.dial
			t.Cleanup(tr.CloseIdleConnections)
			var rt http.RoundTripper = tr
			if tc.inBody {
				rt = aiAfterHeaders{inner: tr, then: network.lose}
			}
			_, err := doAIRequest(&http.Client{Transport: rt}, aiScopeRequest(t, context.Background(), srv.URL+"/v1/synthetic"))
			if err == nil {
				t.Fatal("control: the request succeeded")
			}
			if tc.h2 && http2.Load() == 0 {
				t.Fatal("control: the request did not go over HTTP/2")
			}
			var lost *net.OpError
			if !errors.As(err, &lost) || !errors.Is(err, tc.errno) {
				t.Fatalf("control: %v is not the connection's own %v", err, tc.errno)
			}
			if got := friendlyAIError(err); got != aiConnectionLine {
				t.Errorf("the reader is told %q, not the connection line", got)
			}
			if got := served.Load(); got != 3 {
				t.Errorf("the provider was sent the request %d times, not 3 as in 1.2.24", got)
			}
		})
	}
}

// A request written whole and not answered in time is sent once and told as
// the provider taking too long: no response headers within the transport's
// wait, the request's deadline after the request was written and the
// deadline inside an answer that began, each over HTTP/1.1 and HTTP/2, and
// HTTP 504. Each server has the whole request. Over HTTP/2 the wait
// for the response headers ends in a net.Error timeout that matches no
// deadline and is no connection's.
func TestAIProviderTimeoutsAreSentOnce(t *testing.T) {
	aiNoRetryWait(t)
	held := func(t *testing.T, status int, body bool, h2 bool, deadline *aiDeadline) (string, *http.Client, *atomic.Int32, *atomic.Int32) {
		t.Helper()
		served, http2 := new(atomic.Int32), new(atomic.Int32)
		release := make(chan struct{})
		srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			served.Add(1)
			if r.ProtoMajor == 2 {
				http2.Add(1)
			}
			_, _ = io.ReadAll(r.Body) // the whole request has arrived
			if status != 0 {
				w.WriteHeader(status)
				return
			}
			if body {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"answer":"synthetic`))
				w.(http.Flusher).Flush()
			} else if deadline != nil {
				deadline.pass()
			}
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		var client *http.Client
		if h2 {
			srv.EnableHTTP2 = true
			srv.StartTLS()
			client = srv.Client()
		} else {
			srv.Start()
			client = &http.Client{Transport: &http.Transport{}}
		}
		t.Cleanup(srv.Close)
		t.Cleanup(func() { close(release) })
		t.Cleanup(client.CloseIdleConnections)
		return srv.URL + "/v1/synthetic", client, served, http2
	}
	for _, tc := range []struct {
		name     string
		status   int
		body     bool
		h2       bool
		deadline bool
		header   time.Duration
	}{
		{name: "no response headers in time", header: 100 * time.Millisecond},
		{name: "no response headers in time, HTTP/2", h2: true, header: 100 * time.Millisecond},
		{name: "the request's deadline after the write, HTTP/1.1", deadline: true},
		{name: "the request's deadline after the write, HTTP/2", h2: true, deadline: true},
		{name: "the request's deadline inside the answer, HTTP/1.1", body: true, deadline: true},
		{name: "the request's deadline inside the answer, HTTP/2", body: true, h2: true, deadline: true},
		{name: "HTTP 504", status: http.StatusGatewayTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var deadline *aiDeadline
			var ctx context.Context = context.Background()
			if tc.deadline {
				deadline = newAIDeadline()
				ctx = deadline
			}
			address, client, served, http2 := held(t, tc.status, tc.body, tc.h2, deadline)
			if tc.header > 0 {
				client.Transport.(*http.Transport).ResponseHeaderTimeout = tc.header
			}
			if tc.body {
				client.Transport = aiAfterHeaders{inner: client.Transport, then: deadline.pass}
			}
			_, err := doAIRequest(client, aiScopeRequest(t, ctx, address))
			if err == nil {
				t.Fatal("control: the request succeeded")
			}
			if tc.h2 && http2.Load() == 0 {
				t.Fatal("control: the request did not go over HTTP/2")
			}
			const line = "The AI service took too long to answer, so the request was stopped. Try again"
			if got := friendlyAIError(err); !strings.HasPrefix(got, line) {
				t.Errorf("the reader is told %q, not that the service took too long", got)
			}
			if got := served.Load(); got != 1 {
				t.Errorf("the provider was sent the request %d times, not once", got)
			}
		})
	}
}

// HTTP 408 is an ordinary HTTP failure on both transports, not evidence that
// the provider took too long. Its existing one-attempt 4xx policy is unchanged.
func TestAIHTTP408OnRealTransportsKeepsGenericGuidance(t *testing.T) {
	aiNoRetryWait(t)
	for _, h2 := range []bool{false, true} {
		name := "HTTP/1.1"
		if h2 {
			name = "HTTP/2"
		}
		t.Run(name, func(t *testing.T) {
			var served, http2 atomic.Int32
			var requestMismatch atomic.Bool
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				served.Add(1)
				if r.ProtoMajor == 2 {
					http2.Add(1)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != `{"synthetic":"question"}` {
					requestMismatch.Store(true)
				}
				w.WriteHeader(http.StatusRequestTimeout)
				_, _ = io.WriteString(w, `{"error":{"message":"Synthetic request timeout"}}`)
			}))
			var client *http.Client
			if h2 {
				srv.EnableHTTP2 = true
				srv.StartTLS()
				client = srv.Client()
			} else {
				srv.Start()
				client = &http.Client{Transport: &http.Transport{}}
			}
			t.Cleanup(srv.Close)
			t.Cleanup(client.CloseIdleConnections)
			_, err := doAIRequest(client, aiScopeRequest(t, context.Background(), srv.URL+"/v1/synthetic"))
			const want = "The AI service returned an error. Please try again shortly."
			if err == nil || friendlyAIError(err) != want {
				t.Fatalf("HTTP 408 guidance = %q, error=%v", friendlyAIError(err), err)
			}
			if served.Load() != 1 || requestMismatch.Load() {
				t.Fatalf("HTTP 408 changed request or attempt count: calls=%d, mismatch=%v", served.Load(), requestMismatch.Load())
			}
			if h2 && http2.Load() != 1 {
				t.Fatal("control: request did not use HTTP/2")
			}
		})
	}
}
