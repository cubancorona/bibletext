package bibletext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Gemini's 429 RESOURCE_EXHAUSTED bodies open with the sentence OpenAI uses
// for an exhausted account, then name the quota and the delay before a retry
// in QuotaFailure and RetryInfo details. They are rate limits.
const geminiFreeTierMinuteBody = `{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. For more information on this error, head to: https://ai.google.dev/gemini-api/docs/rate-limits.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaMetric":"generativelanguage.googleapis.com/generate_content_free_tier_input_token_count","quotaId":"GenerateContentInputTokensPerModelPerMinute-FreeTier","quotaDimensions":{"location":"global","model":"gemini-2.5-pro"},"quotaValue":"250000"}]},{"@type":"type.googleapis.com/google.rpc.Help","links":[{"description":"Learn more about Gemini API quotas","url":"https://ai.google.dev/gemini-api/docs/rate-limits"}]},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"39s"}]}}`

const geminiFreeTierDayBody = `{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details. For more information on this error, head to: https://ai.google.dev/gemini-api/docs/rate-limits.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaMetric":"generativelanguage.googleapis.com/generate_content_free_tier_requests","quotaId":"GenerateRequestsPerDayPerProjectPerModel-FreeTier","quotaDimensions":{"location":"global","model":"gemini-2.5-flash"},"quotaValue":"250"}]},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"22s"}]}}`

// The same limit with the quota and the retry delay in the message itself,
// on further lines after the clause.
const geminiQuotaLinesBody = `{"error":{"code":429,"message":"You exceeded your current quota, please check your plan and billing details.\nQuota exceeded for metric: generativelanguage.googleapis.com/generate_content_free_tier_input_token_count, limit: 250000, model: gemini-2.5-flash\nPlease retry in 39.844676573s.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.Help","links":[{"description":"Learn more about Gemini API quotas","url":"https://ai.google.dev/gemini-api/docs/rate-limits"}]},{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaMetric":"generativelanguage.googleapis.com/generate_content_free_tier_input_token_count","quotaId":"GenerateContentInputTokensPerModelPerMinute-FreeTier","quotaDimensions":{"location":"global","model":"gemini-2.5-flash"},"quotaValue":"250000"}]},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"39s"}]}}`

func TestAIAccountFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		kind       aiFailureKind
	}{
		{"openai-credit", `{"error":{"code":"credit_balance_exhausted","type":"insufficient_quota"}}`, 429, aiFailureAccountLimit},
		{"openai-org-cap", `{"error":{"code":"organization_spend_limit_exceeded"}}`, 429, aiFailureAccountLimit},
		{"openai-project-cap", `{"error":{"code":"project_spend_limit_exceeded"}}`, 429, aiFailureAccountLimit},
		{"openai-approved-usage-cap", `{"error":{"code":"organization_usage_limit_exceeded"}}`, 429, aiFailureAccountLimit},
		{"openai-legacy-quota", `{"error":{"type":"insufficient_quota","code":null}}`, 429, aiFailureAccountLimit},
		{"openai-specific-rate-wins", `{"error":{"type":"insufficient_quota","code":"rate_limit_exceeded"}}`, 429, 0},
		{"openai-specific-rate-beats-clause", `{"error":{"type":"insufficient_quota","code":"rate_limit_exceeded","message":"You exceeded your current quota, please check your plan and billing details."}}`, 429, 0},
		{"conflicting-rate-code", `{"error":{"code":"rate_limit_exceeded","details":{"error_code":"enforced_spend_limit_reached"}}}`, 429, 0},
		{"openai-rate", `{"error":{"code":"slow_down","type":"rate_limit_error"}}`, 429, 0},
		{"claude-credit", `{"error":{"type":"invalid_request_error","message":"Your credit balance is too low to access the Anthropic API."}}`, 400, aiFailureAccountLimit},
		{"claude-org-cap", `{"error":{"type":"invalid_request_error","message":"You have reached your specified API usage limits. Access resumes later."}}`, 400, aiFailureAccountLimit},
		{"claude-workspace-cap", `{"error":{"type":"invalid_request_error","message":"You have reached your specified workspace API usage limits."}}`, 400, aiFailureAccountLimit},
		{"claude-tier-cap", `{"error":{"type":"rate_limit_error","details":{"error_code":"enforced_spend_limit_reached"}}}`, 429, aiFailureAccountLimit},
		{"claude-billing", `{"error":{"type":"billing_error","message":"Payment information needs attention."}}`, 402, aiFailureBilling},
		{"claude-rate", `{"error":{"type":"rate_limit_error","message":"Too many requests."}}`, 429, 0},
		{"gemini-prepay", `{"error":{"code":402,"status":"RESOURCE_EXHAUSTED"}}`, 402, aiFailureAccountLimit},
		{"gemini-spend-reason", `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SPEND_LIMIT_EXCEEDED"}]}}`, 429, aiFailureAccountLimit},
		{"gemini-ordinary-quota", `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaMetric":"generate_content_free_tier_requests"}]}]}}`, 429, 0},
		{"gemini-enable-billing", `{"error":{"status":"FAILED_PRECONDITION","message":"Please enable billing on your project."}}`, 400, 0},
		{"gemini-free-tier-minute", geminiFreeTierMinuteBody, 429, 0},
		{"gemini-free-tier-day", geminiFreeTierDayBody, 429, 0},
		{"gemini-quota-lines", geminiQuotaLinesBody, 429, 0},
		{"gemini-spend-reason-with-retry", `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"You exceeded your current quota, please check your plan and billing details.","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"SPEND_LIMIT_EXCEEDED"},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"60s"}]}}`, 429, aiFailureAccountLimit},
		{"gemini-prepay-with-quota-detail", `{"error":{"code":402,"status":"RESOURCE_EXHAUSTED","message":"Your Prepay credit balance is depleted.","details":[{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[]}]}}`, 402, aiFailureAccountLimit},
		{"grok-flat-credit", `{"code":"insufficient_credits","error":"Account credits depleted."}`, 403, aiFailureAccountLimit},
		{"grok-flat-spend", `{"code":"spending_limit_reached","error":"Limit reached."}`, 429, aiFailureAccountLimit},
		{"grok-credit-clause", `{"error":"Your newly created team doesn't have any credits yet. Purchase credits."}`, 403, aiFailureAccountLimit},
		{"grok-permission", `{"error":{"type":"permission_error","message":"This team cannot access the model."}}`, 403, 0},
		{"grok-rate", `{"code":"rate_limit_exceeded","error":"Rate limit exceeded."}`, 429, 0},
		{"openai-permission", `{"error":{"type":"invalid_request_error","code":"insufficient_permissions","message":"You have insufficient permissions for this operation."}}`, 403, 0},
		{"claude-permission", `{"error":{"type":"permission_error","message":"Your API key does not have permission to use the specified resource."}}`, 403, 0},
		{"gemini-permission", `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"Permission denied on resource project synthetic-project."}}`, 403, 0},
		{"not-auth-rejection", `{"error":{"code":"insufficient_credits"}}`, 401, 0},
		{"not-model-discovery", `{"error":{"code":"insufficient_credits"}}`, 404, 0},
		{"not-server-overload", `{"error":{"code":"insufficient_credits"}}`, 503, 0},
		{"not-success", `{"error":{"code":"insufficient_credits"}}`, 200, 0},
		{"not-unstructured-html", `<html>insufficient_quota</html>`, 429, 0},
		{"not-text-in-another-field", `{"answer":"Your credit balance is too low","error":{"message":"Invalid request."}}`, 400, 0},
		{"not-quoted-user-content", `{"error":{"message":"Invalid field: Your credit balance is too low"}}`, 400, 0},
		{"gateway-timeout", `<html>upstream timeout</html>`, 504, aiFailureTimeout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := aiResponseFailure(tc.status, []byte(tc.body))
			if tc.kind == 0 {
				if err != nil {
					t.Fatalf("ordinary failure became an account limit or timeout: %v", err)
				}
				return
			}
			var failure *aiResponseError
			var status *apiHTTPError
			if !errors.As(err, &failure) || failure.kind != tc.kind || !errors.As(err, &status) || status.StatusCode != tc.status {
				t.Fatalf("failure kind/status lost: %v", err)
			}
			if status.Details != "" {
				t.Fatal("account/timeout error must not trigger completion-cap retry or model discovery")
			}
		})
	}
}

func TestAIAccountFailureNeverRetainsUntrustedDetails(t *testing.T) {
	info, _ := providerByID(providerAnthropic)
	for _, suffix := range []string{
		". synthetic-secret-token", ". <script>synthetic-html</script>",
		". Question: synthetic-private-question\nAccount: synthetic-account",
		". https://synthetic.invalid/?token=synthetic-secret", ". " + strings.Repeat("synthetic-private-text", 1000),
	} {
		body := fmt.Sprintf(`{"error":{"code":"insufficient_credits","message":%q}}`, "Your credit balance is too low"+suffix)
		err := withAIErrorProvider(aiResponseFailure(400, []byte(body)), info)
		p, ok := aiSpecialError(err)
		if !ok || p.detail != "Anthropic said: “Your credit balance is too low”" {
			t.Fatalf("safe provider clause missing: %#v", p)
		}
		all := fmt.Sprintf("%v %+v %s %s", err, err, p.message, p.detail)
		if strings.Contains(all, "synthetic-") || len(p.detail) > 180 || strings.ContainsAny(p.detail, "\r\n<>") {
			t.Fatal("untrusted response text escaped the account-error boundary")
		}
	}
	// The clause matches regardless of letter case, and the quotation is the
	// reply's own prefix, not the list's spelling of it.
	for _, message := range []string{"YOUR CREDIT BALANCE IS TOO LOW. synthetic-private-tail", "your credit balance is too low, synthetic-private-tail"} {
		body := fmt.Sprintf(`{"error":{"code":"insufficient_credits","message":%q}}`, message)
		p, ok := aiSpecialError(withAIErrorProvider(aiResponseFailure(400, []byte(body)), info))
		if want := "Anthropic said: \u201c" + message[:len("Your credit balance is too low")] + "\u201d"; !ok || p.detail != want {
			t.Fatalf("a case-folded clause match showed %q, want %q", p.detail, want)
		}
	}
	for _, message := range []string{"synthetic-private-question", "<b>Your credit balance is too low</b>", "Your credit balance is too lowly", "Your credit balance\u202e is too low"} {
		body := fmt.Sprintf(`{"error":{"code":"insufficient_credits","message":%q}}`, message)
		p, ok := aiSpecialError(withAIErrorProvider(aiResponseFailure(429, []byte(body)), info))
		if !ok || p.detail != "" {
			t.Fatal("an unknown or malformed provider reason must be omitted")
		}
	}
}

// The billing link opens a fixed page on the vendor's own host, with no
// query, fragment or credentials, and an unknown provider gets no link.
func TestAIBillingDestinationsAreTheVendorsOwnPages(t *testing.T) {
	hosts := map[string]bool{"platform.openai.com": true, "platform.claude.com": true, "aistudio.google.com": true, "console.x.ai": true}
	for _, info := range aiProviders() {
		vendor, address := aiBillingDestination(info)
		u, err := url.Parse(address)
		if err != nil || u.Scheme != "https" || !hosts[u.Host] || u.User != nil || u.RawQuery != "" || u.Fragment != "" || vendor == "" {
			t.Errorf("%s billing destination %q (%s) is not a plain https page on a vendor host", info.ID, address, vendor)
		}
	}
	if _, address := aiBillingDestination(providerInfo{}); address != "" {
		t.Errorf("an unknown provider got a billing destination: %q", address)
	}
}

type aiSyntheticTimeout struct{}

func (aiSyntheticTimeout) Error() string   { return "synthetic timeout" }
func (aiSyntheticTimeout) Timeout() bool   { return true }
func (aiSyntheticTimeout) Temporary() bool { return false }

// syntheticAITimeout is a request written whole and not answered before its
// deadline, as doAIRequest reports one.
func syntheticAITimeout() error {
	return aiProviderTimeout(&url.Error{Op: "Post", URL: "https://synthetic.invalid/private", Err: context.DeadlineExceeded})
}

// A timeout reads as the provider's only with the evidence doAIRequest has:
// its mark for the request's deadline, or the wait for the response headers,
// met after the request was written whole, or a 504 from the far
// side. A deadline or timeout without it, a bare deadline, a transport
// timeout, a dial that timed out, keeps the connection line. The code before
// read every one of those as the provider taking too long.
func TestAIErrorNamesProviderAndDistinguishesTimeoutFromCancel(t *testing.T) {
	for _, info := range aiProviders() {
		for _, cause := range []error{
			syntheticAITimeout(), aiProviderTimeout(aiSyntheticTimeout{}),
			&apiHTTPError{StatusCode: 504}, aiResponseFailure(504, nil),
		} {
			err := withAIErrorProvider(cause, info)
			// With no screen there is no faster-model action to name.
			want := info.ShortName + " took too long to answer, so the request was stopped. Try again."
			p, _ := aiSpecialError(err)
			withFaster := info.ShortName + " took too long to answer, so the request was stopped. Try again, or switch to a faster model."
			if got := friendlyAIError(err); !isAITimeout(err) || got != want || p.shown(true) != withFaster {
				t.Errorf("timeout message = %q, want %q, and %q beside the faster model", got, want, p.shown(true))
			}
		}
		for _, cause := range []error{
			context.DeadlineExceeded, aiSyntheticTimeout{},
			&url.Error{Op: "Post", URL: "https://synthetic.invalid/private", Err: context.DeadlineExceeded},
			&url.Error{Op: "Post", URL: "https://synthetic.invalid/private", Err: &net.OpError{Op: "dial", Net: "tcp", Err: aiSyntheticTimeout{}}},
		} {
			err := withAIErrorProvider(cause, info)
			if _, special := aiSpecialError(err); special || isAITimeout(err) || friendlyAIError(err) != aiConnectionLine {
				t.Errorf("%v with no sign the request was written reads %q, not the connection line", cause, friendlyAIError(err))
			}
		}
		for _, cause := range []error{context.Canceled, errors.Join(context.Canceled, aiSyntheticTimeout{})} {
			err := withAIErrorProvider(cause, info)
			if isAITimeout(err) || strings.Contains(friendlyAIError(err), "too long") || !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation was changed into a timeout")
			}
		}
		billing := withAIErrorProvider(aiResponseFailure(429, []byte(`{"error":{"code":"credit_balance_exhausted"}}`)), info)
		vendor, _ := aiBillingDestination(info)
		want := info.ShortName + " didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with " + vendor + "; nothing in BibleText needs changing."
		if friendlyAIError(billing) != want {
			t.Fatal("account failure did not use the provider-specific explanation")
		}
	}
	if p, ok := aiSpecialError(errBudgetExhausted); ok || p.billingURL != "" {
		t.Fatal("reasoning/output allowance must not become an account billing error")
	}
}

// HTTP 408 retains ordinary 4xx handling: one attempt and the generic line,
// including when its body resembles an account or model-recovery response.
func TestAIHTTP408KeepsGenericErrorAndExistingAttemptCount(t *testing.T) {
	aiNoRetryWait(t)
	const want = "The AI service returned an error. Please try again shortly."
	for _, body := range []string{
		`{}`,
		`{"error":{"message":"Request timed out."}}`,
		`{"error":{"code":"credit_balance_exhausted","message":"Your credit balance is too low."}}`,
		`{"error":{"message":"The model is unavailable."}}`,
	} {
		client := &mockHTTP{statusCode: http.StatusRequestTimeout, body: body}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = doAIRequest(client, req)
		var status *apiHTTPError
		if !errors.As(err, &status) || status.StatusCode != http.StatusRequestTimeout || status.Details != "" || client.calls != 1 {
			t.Fatalf("HTTP 408 changed status or attempt count: calls=%d, err=%v", client.calls, err)
		}
		for _, info := range aiProviders() {
			for _, failure := range []error{err, &apiHTTPError{StatusCode: http.StatusRequestTimeout}} {
				wrapped := withAIErrorProvider(failure, info)
				if p, special := aiSpecialError(wrapped); special || p.timeout || isAITimeout(wrapped) || friendlyAIError(wrapped) != want {
					t.Errorf("%s HTTP 408 became special recovery: %#v, %q", info.ID, p, friendlyAIError(wrapped))
				}
			}
		}
	}
}

// The bodies exercise the actual detail recognizers, but HTTP 408 must not
// retain those categories or enter model discovery or completion-cap recovery.
func TestAIHTTP408DoesNotUseBodyDrivenRecovery(t *testing.T) {
	aiNoRetryWait(t)
	for _, tc := range []struct{ name, body, category string }{
		{"model-not-found", `{"error":{"message":"The model 'gpt-synthetic' does not exist or you do not have access to it.","type":"invalid_request_error","code":"model_not_found"}}`, "model unavailable"},
		{"completion-cap", `{"error":{"message":"max_tokens is too large: 32768. This model supports at most 16384 completion tokens, whereas you provided 32768.","type":"invalid_request_error","param":"max_tokens","code":null}}`, completionCapDetail + "16384"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeAIErrorDetail([]byte(tc.body)); got != tc.category {
				t.Fatalf("control: body category=%q, want %q", got, tc.category)
			}
			terminal := aiResponseFailure(http.StatusRequestTimeout, []byte(tc.body))
			plain, ok := terminal.(*apiHTTPError)
			if !ok || plain.StatusCode != http.StatusRequestTimeout || plain.Details != "" {
				t.Fatalf("HTTP 408 did not retain a plain body-free terminal error: %v", terminal)
			}
			store := newTestKeyStore()
			beforeModel := store.resolvedModel(providerOpenAI)
			client := &mockHTTP{statusCode: http.StatusRequestTimeout, body: tc.body}
			var built []string
			listed := false
			resolver := &modelResolver{store: store, id: providerOpenAI, def: "gpt-synthetic", tier: "gpt", apiKey: "synthetic-key",
				list: func(context.Context, string) ([]discoveredModel, error) {
					listed = true
					return []discoveredModel{{id: "gpt-synthetic-next", rank: 1}}, nil
				},
				build: func(model string) aiClient {
					built = append(built, model)
					return &openAIClient{apiKey: "synthetic-key", baseURL: "https://synthetic.invalid/v1", model: model, http: client}
				},
			}
			_, err := resolver.generate(context.Background(), "A synthetic question.")
			var status *apiHTTPError
			const want = "The AI service returned an error. Please try again shortly."
			if !errors.As(err, &status) || status.StatusCode != http.StatusRequestTimeout || status.Details != "" || friendlyAIError(err) != want {
				t.Fatalf("HTTP 408 recovery changed the generic error: %v, %q", err, friendlyAIError(err))
			}
			if listed || len(built) != 1 || built[0] != "gpt-synthetic" || client.calls != 1 || store.resolvedModel(providerOpenAI) != beforeModel {
				t.Fatalf("HTTP 408 entered model/budget recovery: listed=%v, models=%v, requests=%d", listed, built, client.calls)
			}
			if isModelNotFound(err) {
				t.Fatal("HTTP 408 read as a missing model")
			}
			if _, cap := completionCapOf(err); cap {
				t.Fatal("HTTP 408 read as a completion-cap refusal")
			}
			if client.lastReq == nil || client.lastReq.Body == nil {
				t.Fatal("control: no original HTTP request was sent")
			}
			var request openAIRequest
			if err := json.NewDecoder(client.lastReq.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			_ = client.lastReq.Body.Close()
			if request.Model != "gpt-synthetic" || request.MaxCompletionTokens != aiMaxOutputTokens {
				t.Fatalf("HTTP 408 changed model/output budget: %#v", request)
			}
		})
	}
}

// mockWrittenHTTP writes each request, reporting the connection and the
// write through the request's trace as the transport does, and then fails
// it with err.
type mockWrittenHTTP struct {
	err   error
	calls int
}

func (m *mockWrittenHTTP) Do(req *http.Request) (*http.Response, error) {
	m.calls++
	if trace := httptrace.ContextClientTrace(req.Context()); trace != nil {
		if trace.GetConn != nil {
			trace.GetConn(req.URL.Host)
		}
		if trace.GotConn != nil {
			trace.GotConn(httptrace.GotConnInfo{})
		}
		if trace.WroteRequest != nil {
			trace.WroteRequest(httptrace.WroteRequestInfo{})
		}
	}
	return nil, m.err
}

// aiCutAnswerHTTP answers each request with the start of a 200 whose body
// then fails with err, cut running first.
type aiCutAnswerHTTP struct {
	err   error
	cut   func()
	calls int
}

func (m *aiCutAnswerHTTP) Do(*http.Request) (*http.Response, error) {
	m.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(io.MultiReader(strings.NewReader(`{"answer":"synthetic`), aiCutReader{err: m.err, cut: m.cut}))}, nil
}

type aiCutReader struct {
	err error
	cut func()
}

func (r aiCutReader) Read([]byte) (int, error) {
	if r.cut != nil {
		r.cut()
	}
	return 0, r.err
}

// Inside the answer only the request's deadline is the provider's. An answer
// cut off as the deadline passes reads as the provider taking too long and
// is not sent again, whatever the read reports: the connection may close at
// that moment with an unexpected EOF. An answer cut off with the deadline
// still ahead was lost with its connection, and is tried as 1.2.24 tries it,
// with its line, even when the read reports a timeout of its own; the code
// before took a timeout that was no connection's for the provider's there,
// and sent the request once.
func TestAIAnswerCutOffIsTheProvidersOnlyAtItsDeadline(t *testing.T) {
	aiNoRetryWait(t)
	deadline := newAIDeadline()
	atDeadline := &aiCutAnswerHTTP{err: io.ErrUnexpectedEOF, cut: deadline.pass}
	req, _ := http.NewRequestWithContext(deadline, http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
	if _, err := doAIRequest(atDeadline, req); !isAITimeout(err) || atDeadline.calls != 1 {
		t.Errorf("an answer cut off as the deadline passed: calls=%d, %q", atDeadline.calls, friendlyAIError(err))
	}
	for _, cause := range []error{
		aiSyntheticTimeout{},
		&net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ETIMEDOUT)},
		io.ErrUnexpectedEOF,
	} {
		ahead := &aiCutAnswerHTTP{err: cause}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
		if _, err := doAIRequest(ahead, req); isAITimeout(err) || friendlyAIError(err) != aiConnectionLine || ahead.calls != 3 {
			t.Errorf("an answer cut off by %v with the deadline ahead: calls=%d, %q", cause, ahead.calls, friendlyAIError(err))
		}
	}
}

func TestAITimeoutAndAccountRejectionAreNotResent(t *testing.T) {
	previous := aiRetrySleep
	t.Cleanup(func() { aiRetrySleep = previous })
	aiRetrySleep = func(time.Duration) {}
	for _, tc := range []struct {
		status int
		body   string
	}{
		{429, `{"error":{"code":"project_spend_limit_exceeded"}}`},
		{400, `{"error":{"message":"Your credit balance is too low."}}`},
		{402, `{"error":{"type":"billing_error"}}`},
		{504, `upstream timeout`},
	} {
		client := &mockHTTP{statusCode: tc.status, body: tc.body}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
		if _, err := doAIRequest(client, req); err == nil || client.calls != 1 {
			t.Errorf("failure was resent or hidden: calls=%d, err=%v", client.calls, err)
		}
	}
	// One of the request's own timers running out after the write, its
	// deadline or a transport's wait that is no connection's, is the
	// provider's, sent once. A timeout the connection itself reports after
	// the write is that connection lost, and a timeout with nothing written a
	// connection never made: each is tried three times as in 1.2.24. The code
	// before told the connection's own timeout after the write as the
	// provider taking too long, after one attempt.
	for _, timer := range []error{context.DeadlineExceeded, aiSyntheticTimeout{}} {
		written := &mockWrittenHTTP{err: timer}
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
		if _, err := doAIRequest(written, req); !isAITimeout(err) || written.calls != 1 {
			t.Errorf("%v after the write was resent or not told as a timeout: calls=%d, err=%v", timer, written.calls, err)
		}
	}
	lost := &mockWrittenHTTP{err: &net.OpError{Op: "read", Net: "tcp", Err: os.NewSyscallError("read", syscall.ETIMEDOUT)}}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
	if _, err := doAIRequest(lost, req); isAITimeout(err) || friendlyAIError(err) != aiConnectionLine || lost.calls != 3 {
		t.Errorf("a connection that timed out after the write: calls=%d, %q", lost.calls, friendlyAIError(err))
	}
	unwritten := &mockHTTP{err: aiSyntheticTimeout{}}
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
	if _, err := doAIRequest(unwritten, req); isAITimeout(err) || friendlyAIError(err) != aiConnectionLine || unwritten.calls != 3 {
		t.Errorf("a timeout with nothing written: calls=%d, %q", unwritten.calls, friendlyAIError(err))
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	aiRetrySleep = func(time.Duration) { cancel() }
	client := &mockHTTP{err: io.ErrUnexpectedEOF}
	req, _ = http.NewRequestWithContext(ctx, http.MethodPost, "https://synthetic.invalid", strings.NewReader("{}"))
	if _, err := doAIRequest(client, req); !errors.Is(err, context.Canceled) || client.calls != 1 {
		t.Fatalf("cancellation during backoff resent a request: calls=%d, err=%v", client.calls, err)
	}
}
