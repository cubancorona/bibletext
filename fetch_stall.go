package bibletext

// The whole-corpus downloads (helloao complete.json bodies) grew past 45MB
// when the feeds gained Strong's data, and a flat http.Client wall-clock
// deadline starves any connection slower than size/deadline — the download
// dies at the timeout however steadily it was progressing, the retry loop
// fails the same way every attempt, and the reader silently keeps the
// previous edition. The right deadline for a large transfer is INACTIVITY:
// any progress keeps it alive; only a genuine stall aborts. This client
// applies one inactivity window to every phase — connect, headers, and each
// body read — so a trickling download completes in however long it needs,
// while a dead connection still fails fast enough for the retry loop's
// backoff to matter.

import (
	"context"
	"io"
	"net/http"
	"time"
)

// corpusStallTimeout is the inactivity window: abort only after this long
// with no bytes. Generous, because the cost of a false abort (a whole
// restarted 45MB download) dwarfs the cost of waiting out a hiccup.
const corpusStallTimeout = 30 * time.Second

// newCorpusClient is the client the three corpus fetchers share.
func newCorpusClient() httpClient {
	return stallGuardClient{inner: &http.Client{}, stall: corpusStallTimeout}
}

type stallGuardClient struct {
	inner *http.Client
	stall time.Duration
}

func (c stallGuardClient) Do(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	// One timer covers connect+headers now and every body read afterwards:
	// armed before Do, reset by each successful read, firing cancels the
	// request's context so the transport aborts the pending operation.
	timer := time.AfterFunc(c.stall, cancel)
	resp, err := c.inner.Do(req.WithContext(ctx))
	if err != nil {
		timer.Stop()
		cancel()
		return nil, err
	}
	timer.Reset(c.stall)
	resp.Body = &stallGuardBody{inner: resp.Body, timer: timer, stall: c.stall, cancel: cancel}
	return resp, nil
}

// stallGuardBody resets the inactivity timer on every read and releases the
// context on Close — the caller (fetchWithRetry) closes the body on every
// path, so the context cannot leak.
type stallGuardBody struct {
	inner  io.ReadCloser
	timer  *time.Timer
	stall  time.Duration
	cancel context.CancelFunc
}

func (b *stallGuardBody) Read(p []byte) (int, error) {
	n, err := b.inner.Read(p)
	if n > 0 {
		b.timer.Reset(b.stall)
	}
	return n, err
}

func (b *stallGuardBody) Close() error {
	b.timer.Stop()
	b.cancel()
	return b.inner.Close()
}
