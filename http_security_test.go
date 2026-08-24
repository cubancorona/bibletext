package bibletext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHTTPClientRefusesCrossOriginCredentialRedirects(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
	}))
	defer origin.Close()

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "API.Bible", header: "api-key", value: "fixture-bible-credential"},
		{name: "Anthropic", header: "x-api-key", value: "fixture-anthropic-credential"},
		{name: "Gemini", header: "x-goog-api-key", value: "fixture-gemini-credential"},
		{name: "OpenAI-compatible", header: "Authorization", value: "Bearer fixture-ai-credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, origin.URL+"/start", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set(tc.header, tc.value)
			resp, err := newHTTPClient().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound {
				t.Fatalf("redirect response = %d, want %d", resp.StatusCode, http.StatusFound)
			}
		})
	}
	if got := targetHits.Load(); got != 0 {
		t.Fatalf("cross-origin redirect target received %d authenticated requests", got)
	}
}

func TestHTTPClientAllowsSameOriginRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/finish", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/finish", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "fixture-credential" {
			t.Errorf("same-origin redirect lost credential header; got %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-api-key", "fixture-credential")
	resp, err := newHTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("same-origin redirect response = %d", resp.StatusCode)
	}
}

type credentialEchoClient struct {
	body string
}

func (c credentialEchoClient) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(c.body)),
		Header:     make(http.Header),
	}, nil
}

func TestAIErrorOmitsAuthenticatedResponseBody(t *testing.T) {
	const credential = "fixture-private-request-credential"
	for _, tc := range []struct {
		header string
		value  string
	}{
		{header: "api-key", value: credential},
		{header: "x-api-key", value: credential},
		{header: "x-goog-api-key", value: credential},
		{header: "Authorization", value: "Bearer " + credential},
	} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://provider.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(tc.header, tc.value)
		body := fmt.Sprintf(`{"error":{"message":"diagnostic echoed %s"}}`, credential)
		_, err = doAIRequest(credentialEchoClient{body: body}, req)
		if err == nil {
			t.Fatal("expected authenticated error")
		}
		if strings.Contains(err.Error(), credential) {
			t.Fatalf("%s credential survived authenticated-body omission", tc.header)
		}
		if strings.Contains(err.Error(), "diagnostic echoed") {
			t.Fatalf("%s authenticated response body reached the error: %v", tc.header, err)
		}
	}
}

func TestAIErrorRetainsOnlySafeModelCategory(t *testing.T) {
	const credential = "fixture-private-request-credential"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://provider.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("x-api-key", credential)
	body := fmt.Sprintf(`{"error":{"message":"model retired; diagnostic echoed %s"}}`, credential)
	_, err = doAIRequest(credentialEchoClient{body: body}, req)
	var statusErr *apiHTTPError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected status error, got %v", err)
	}
	if statusErr.Details != "model unavailable" {
		t.Fatalf("safe detail = %q", statusErr.Details)
	}
	if strings.Contains(err.Error(), credential) || strings.Contains(err.Error(), "diagnostic") {
		t.Fatalf("authenticated response content reached the error: %v", err)
	}
}
