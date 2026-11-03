package bibletext

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
)

type aiFailureKind uint8

const (
	aiFailureAccountLimit aiFailureKind = iota + 1
	aiFailureBilling
	aiFailureTimeout
)

// Only fixed categories and recognized clauses cross the transport boundary.
// Arbitrary provider messages can contain echoed credentials or reader text.
type aiResponseError struct {
	kind   aiFailureKind
	reason string
	cause  error
}

func (e *aiResponseError) Error() string { return "AI request rejected" }
func (e *aiResponseError) Unwrap() error { return e.cause }

// The provider belongs to the request, even if Settings changes before it lands.
type aiProviderError struct {
	provider providerInfo
	cause    error
}

func (e *aiProviderError) Error() string { return "AI request failed" }
func (e *aiProviderError) Unwrap() error { return e.cause }

func withAIErrorProvider(err error, info providerInfo) error {
	if err == nil {
		return nil
	}
	var existing *aiProviderError
	if errors.As(err, &existing) {
		return err
	}
	return &aiProviderError{provider: info, cause: err}
}

func activeAIProvider(state *AppState) providerInfo {
	if state != nil {
		if info, ok := providerByID(state.keys().activeProvider()); ok {
			return info
		}
	}
	info, _ := providerByID(defaultProviderID)
	return info
}

func aiBillingDestination(info providerInfo) (vendor, address string) {
	switch info.ID {
	case providerOpenAI:
		return "OpenAI", "https://platform.openai.com/settings/organization/billing"
	case providerAnthropic:
		return "Anthropic", "https://platform.claude.com/settings/billing"
	case providerGemini:
		return "Google", "https://aistudio.google.com/spend"
	case providerGrok:
		return "SpaceXAI", "https://console.x.ai"
	default:
		return "the provider", ""
	}
}

type aiErrorPresentation struct {
	message, detail, billingURL, vendor string
	providerID, provider                string
	timeout                             bool
}

func aiSpecialError(err error) (aiErrorPresentation, bool) {
	if err == nil || errors.Is(err, context.Canceled) {
		return aiErrorPresentation{}, false
	}
	var provider *aiProviderError
	var info providerInfo
	if errors.As(err, &provider) {
		info = provider.provider
	}
	name := info.ShortName
	if name == "" {
		name = info.Name
	}
	if name == "" {
		name = "The AI service"
	}
	vendor, address := aiBillingDestination(info)
	var response *aiResponseError
	if errors.As(err, &response) && response.kind != aiFailureTimeout {
		message := name + " didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with " + vendor + "; nothing in " + ProductName() + " needs changing."
		if response.kind == aiFailureBilling {
			message = name + " didn't accept the request because of a billing issue with the account behind your key. You can check that with " + vendor + "; nothing in " + ProductName() + " needs changing."
		}
		detail := ""
		if response.reason != "" {
			detail = vendor + " said: “" + response.reason + "”"
		}
		return aiErrorPresentation{message: message, detail: detail, billingURL: address, vendor: vendor,
			providerID: info.ID, provider: name}, true
	}
	if isAITimeout(err) {
		return aiErrorPresentation{message: aiTimeoutMessage(name, false),
			providerID: info.ID, provider: name, timeout: true}, true
	}
	return aiErrorPresentation{}, false
}

// aiTimeoutMessage is the timeout line. It names the faster model only where
// the screen showing it offers that model (aiErrorFasterModel): a reader
// already on it, or whose provider has changed since, is not pointed at an
// action that is not there.
func aiTimeoutMessage(provider string, fasterOffered bool) string {
	if fasterOffered {
		return provider + " took too long to answer, so the request was stopped. Try again, or switch to a faster model."
	}
	return provider + " took too long to answer, so the request was stopped. Try again."
}

// shown is p's message as a screen shows it with its actions: a timeout
// names the faster model only when the screen offers it.
func (p aiErrorPresentation) shown(fasterOffered bool) string {
	if p.timeout {
		return aiTimeoutMessage(p.provider, fasterOffered)
	}
	return p.message
}

// isAITimeout reports a request that got no answer in time: doAIRequest's
// mark for the request's deadline, or the transport's wait for the response
// headers, met after the request was written whole (aiProviderTimeout), or
// an HTTP 504. HTTP 408 keeps ordinary 4xx guidance. A deadline or timeout
// met before then, while dialling, resolving the host or completing the TLS
// handshake, is a connection never made, and a timeout the connection reports
// is that connection lost: both
// keep the connection line and its retries, since "took too long to answer"
// and a faster model would misdirect a reader whose network is what failed.
// A dial timeout satisfies errors.Is(err, context.DeadlineExceeded) as well,
// so neither that nor net.Error's Timeout is evidence here.
func isAITimeout(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var response *aiResponseError
	if errors.As(err, &response) && response.kind == aiFailureTimeout {
		return true
	}
	var apiErr *apiHTTPError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusGatewayTimeout
}

// aiProviderTimeout marks cause, the request's deadline or the wait for the
// response headers met after the request was written whole, as the
// provider's: no answer came in time. The write is what the transport
// reports, not delivery, so a request whose network went silent after it
// reads the same way when its deadline passes.
func aiProviderTimeout(cause error) error {
	return &aiResponseError{kind: aiFailureTimeout, cause: cause}
}

// aiTransportTimeout reports one of the request's own timers running out,
// whichever phase of the request it met: the request's deadline, or the
// transport's wait for the response headers (ResponseHeaderTimeout), which
// over HTTP/2 is a net.Error timeout that matches no deadline. A timeout
// the connection itself reports, a net.OpError, is that connection failing,
// as a reset is: TCP gives up on a peer that stopped answering, its
// keep-alive probes or retransmissions unanswered, with ETIMEDOUT. A
// cancellation is neither.
func aiTransportTimeout(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var connection *net.OpError
	if errors.As(err, &connection) {
		return false
	}
	var timeout net.Error
	return errors.As(err, &timeout) && timeout.Timeout()
}

// These clauses carry no identifiers, amounts, dates, URLs or request text. A
// match keeps the clause-length prefix of the reply, in the reply's own letter
// case, and never the rest of it, so the quotation is what the provider sent.
// Unknown messages are omitted rather than relying on incomplete redaction.
var aiAccountLimitClauses = []string{
	"You exceeded your current quota, please check your plan and billing details",
	"Your credit balance is too low",
	"Your Prepay credit balance is depleted",
	"Your organization has no prepaid credits remaining",
	"You have reached your specified workspace API usage limits",
	"You have reached your specified API usage limits",
	"You have reached your API usage limits: your organization has crossed its monthly API usage threshold",
	"Your newly created team doesn't have any credits yet",
	"Your team has exhausted its credits",
	"Your team has reached its spending limit",
	"You have reached your monthly spending limit",
}

func aiAccountLimitReason(message string) string {
	message = strings.TrimSpace(message)
	for _, clause := range aiAccountLimitClauses {
		if len(message) >= len(clause) && strings.EqualFold(message[:len(clause)], clause) {
			// Do not recognize a longer word that happens to share the prefix.
			if len(message) == len(clause) || strings.ContainsRune(".!,;: \t\r\n", rune(message[len(clause)])) {
				// Own the safe prefix so it cannot keep the rest of the reply alive.
				return strings.Clone(message[:len(clause)])
			}
		}
	}
	return ""
}

func aiAccountLimitCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "insufficient_quota", "insufficient_credits", "credit_balance_exhausted",
		"billing_hard_limit_reached", "billing_limit_exceeded", "spend_limit_exceeded",
		"spending_limit_reached", "organization_spend_limit_exceeded", "project_spend_limit_exceeded",
		"organization_usage_limit_exceeded", "enforced_spend_limit_reached":
		return true
	}
	return false
}

// aiResponseFailure recognizes structured account failures before generic model
// discovery or retries. RESOURCE_EXHAUSTED alone at 429 is a rate/quota error;
// Google's 402 version specifically means depleted prepaid credit.
func aiResponseFailure(status int, body []byte) error {
	cause := &apiHTTPError{StatusCode: status}
	if status == http.StatusRequestTimeout {
		// Keep generic 4xx guidance without admitting body-derived model or
		// completion-cap categories that this terminal response never carried.
		return cause
	}
	if status == http.StatusGatewayTimeout {
		return &aiResponseError{kind: aiFailureTimeout, cause: cause}
	}
	if status != 400 && status != 402 && status != 403 && status != 429 {
		return nil
	}
	var envelope struct {
		Error   json.RawMessage `json:"error"`
		Code    string          `json:"code"`
		Type    string          `json:"type"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return nil
	}
	var detail struct {
		Code    json.RawMessage `json:"code"`
		Type    string          `json:"type"`
		Status  string          `json:"status"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	}
	if json.Unmarshal(envelope.Error, &detail) != nil {
		// Some compatible APIs use a top-level code and a string error.
		_ = json.Unmarshal(envelope.Error, &detail.Message)
	}
	code := envelope.Code
	_ = json.Unmarshal(detail.Code, &code) // numeric Google status is not a code
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "rate_limit_exceeded", "rate_limit_error", "slow_down", "too_many_requests":
		return nil // an explicit transient code outranks an ambiguous quota message
	}
	if detail.Type == "" {
		detail.Type = envelope.Type
	}
	if detail.Message == "" {
		detail.Message = envelope.Message
	}
	var nested struct {
		Code string `json:"error_code"`
	}
	_ = json.Unmarshal(detail.Details, &nested)
	var google []struct {
		Type   string `json:"@type"`
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(detail.Details, &google)
	account := aiAccountLimitCode(code) || aiAccountLimitCode(nested.Code)
	quota := false
	for _, item := range google {
		switch item.Type {
		case "type.googleapis.com/google.rpc.ErrorInfo":
			if aiAccountLimitCode(item.Reason) {
				account = true
			}
		case "type.googleapis.com/google.rpc.QuotaFailure", "type.googleapis.com/google.rpc.RetryInfo":
			quota = true
		}
	}
	// A specific rate-limit code outranks an ambiguous broad quota type.
	if code == "" && detail.Type == "insufficient_quota" {
		account = true
	}
	prepay := status == 402 && detail.Status == "RESOURCE_EXHAUSTED"
	// Gemini opens every 429 RESOURCE_EXHAUSTED with the sentence OpenAI uses
	// for an exhausted account, then names the quota and the delay before a
	// retry in QuotaFailure and RetryInfo details. Those details mark a rate
	// limit, so without a structured account code they outrank the clause.
	if quota && !account && !prepay {
		return nil
	}
	reason := aiAccountLimitReason(detail.Message)
	if reason != "" || prepay {
		account = true
	}
	if account {
		return &aiResponseError{kind: aiFailureAccountLimit, reason: reason, cause: cause}
	}
	if status == 402 && detail.Type == "billing_error" {
		return &aiResponseError{kind: aiFailureBilling, cause: cause}
	}
	return nil
}
