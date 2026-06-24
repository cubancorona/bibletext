package bibletext

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFetchWithRetryRetriesAndSucceeds(t *testing.T) {
	originalRetrySleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalRetrySleep }()

	attempts := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return jsonResponse(http.StatusTooManyRequests, `{"error":"rate limited"}`), nil
		}
		return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"ok"}]}`), nil
	})

	body, err := fetchWithRetry(client, "https://bible-api.com/Genesis+1", 3)
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if !strings.Contains(string(body), `"verses"`) {
		t.Fatalf("expected verses payload, got: %s", string(body))
	}
}

func TestFetchWithRetryHandles429WithRetryAfter(t *testing.T) {
	originalRetrySleep := retrySleep
	var delays []time.Duration
	retrySleep = func(d time.Duration) {
		delays = append(delays, d)
	}
	defer func() { retrySleep = originalRetrySleep }()

	attempts := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return jsonResponseWithHeaders(http.StatusTooManyRequests, `{"error":"rate limited"}`, map[string]string{
				"Retry-After": "2",
			}), nil
		}
		return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"ok"}]}`), nil
	})

	_, err := fetchWithRetry(client, "https://bible-api.com/Jude+1", 6)
	if err != nil {
		t.Fatalf("expected success after 429 retries, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if len(delays) < 2 {
		t.Fatalf("expected retry delays to be captured, got %d", len(delays))
	}
	if delays[0] != 2*time.Second || delays[1] != 2*time.Second {
		t.Fatalf("expected Retry-After delay of 2s, got %v", delays)
	}
}

func TestFetchWithRetryReturnsNotFoundError(t *testing.T) {
	originalRetrySleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalRetrySleep }()

	attempts := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		attempts++
		return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
	})

	_, err := fetchWithRetry(client, "https://bible-api.com/NoBook+1", 3)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !errors.Is(err, errChapterNotFound) {
		t.Fatalf("expected errChapterNotFound, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for 404, got %d", attempts)
	}
}

func TestParseRetryAfter(t *testing.T) {
	d, ok := parseRetryAfter("3")
	if !ok {
		t.Fatal("expected Retry-After seconds to parse")
	}
	if d != 3*time.Second {
		t.Fatalf("expected 3s delay, got %s", d)
	}
}

func TestDecodeChapterResponseInvalidJSON(t *testing.T) {
	_, err := decodeChapterResponse("Genesis", 1, []byte("{bad json"))
	if err == nil {
		t.Fatal("expected JSON decode error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("expected invalid JSON error message, got: %v", err)
	}
}

func TestFetchBibleFromAPIWithClientReportsIncompleteBook(t *testing.T) {
	originalRetrySleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalRetrySleep }()

	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/Genesis+1":
			return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"In the beginning."}]}`), nil
		case "/Genesis+2":
			return jsonResponse(http.StatusInternalServerError, `{"error":"upstream failed"}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"error":"missing"}`), nil
		}
	})

	bd, err := fetchBibleFromAPIWithClient([]string{"Genesis"}, client, func(time.Duration) {})
	if err == nil {
		t.Fatal("expected partial-load error, got nil")
	}
	if bd == nil {
		t.Fatal("expected partial bible data, got nil")
	}

	genesis1 := bd.GetChapter("Genesis", 1)
	if len(genesis1) == 0 {
		t.Fatal("expected Genesis 1 verses to be loaded before failure")
	}
	if !strings.Contains(err.Error(), "failed or incomplete books: Genesis") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestFetchBibleFromAPIWithClientStopsOnNotFoundChapter(t *testing.T) {
	originalRetrySleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalRetrySleep }()

	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/Jude+1":
			return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"Jude 1:1"}]}`), nil
		case "/Jude+2":
			return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"error":"missing"}`), nil
		}
	})

	bd, err := fetchBibleFromAPIWithClient([]string{"Jude"}, client, func(time.Duration) {})
	if err != nil {
		t.Fatalf("expected clean load for single-chapter book, got: %v", err)
	}

	jude1 := bd.GetChapter("Jude", 1)
	if len(jude1) != 1 {
		t.Fatalf("expected 1 verse in Jude 1, got %d", len(jude1))
	}
}

func TestFetchBibleFromAPIWithClientRecoversFromRateLimiting(t *testing.T) {
	originalRetrySleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalRetrySleep }()

	attempts := 0
	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/Jude+1" {
			return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		}

		attempts++
		// Force one full fetchWithRetry exhaustion (6 attempts), then allow success.
		if attempts <= 6 {
			return jsonResponseWithHeaders(http.StatusTooManyRequests, `{"error":"rate limited"}`, map[string]string{
				"Retry-After": "1",
			}), nil
		}
		return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"Jude 1:1"}]}`), nil
	})

	bd, err := fetchBibleFromAPIWithClient([]string{"Jude"}, client, func(time.Duration) {})
	if err != nil {
		t.Fatalf("expected rate-limit recovery success, got: %v", err)
	}

	jude1 := bd.GetChapter("Jude", 1)
	if len(jude1) != 1 {
		t.Fatalf("expected Jude chapter to load after rate-limit recovery, got %d verses", len(jude1))
	}
}

// TestFetchReportsProgressPerBook verifies the first-run fetch reports per-book
// progress to loadProgressFn, which the loading screen renders (see app.go).
func TestFetchReportsProgressPerBook(t *testing.T) {
	var books []string
	var lastTotal int
	loadProgressFn = func(loaded, total int, book string) {
		books = append(books, book)
		lastTotal = total
	}
	defer func() { loadProgressFn = nil }()

	client := newMockClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/Jude+1" {
			return jsonResponse(http.StatusOK, `{"verses":[{"verse":1,"text":"Jude 1:1"}]}`), nil
		}
		return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
	})

	if _, err := fetchBibleFromAPIWithClient([]string{"Jude"}, client, func(time.Duration) {}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(books) != 1 || books[0] != "Jude" {
		t.Fatalf("expected one progress report for Jude, got %v", books)
	}
	if lastTotal != 1 {
		t.Errorf("expected total books = 1, got %d", lastTotal)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return jsonResponseWithHeaders(status, body, nil)
}

func jsonResponseWithHeaders(status int, body string, headers map[string]string) *http.Response {
	header := make(http.Header)
	for k, v := range headers {
		header.Set(k, v)
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     header,
	}
}

func newMockClient(rt func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{
		Timeout:   2 * time.Second,
		Transport: mockTransport(rt),
	}
}

type mockTransport func(*http.Request) (*http.Response, error)

func (m mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}
