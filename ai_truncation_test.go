package bibletext

import (
	"errors"
	"strings"
	"testing"
)

// A provider that stops a model mid-answer returns real text that simply ends,
// often mid-word. Every parser used to hand that back as a finished answer:
// Anthropic had no stop_reason field at all, and the other two consulted their
// reason only when the text was EMPTY, so a PARTIAL answer walked straight
// past the guard. The reader then had no way to tell an explanation that
// finished from one that was cut off.
func TestParsersMarkAnAnswerTheModelWasCutOffIn(t *testing.T) {
	const partial = "This verse comes near the end of Jeremiah 23, and the key to it is a J"

	for _, tc := range []struct {
		name  string
		body  string
		parse func([]byte) (string, error)
	}{
		{
			"anthropic max_tokens",
			`{"content":[{"type":"text","text":"` + partial + `"}],"stop_reason":"max_tokens"}`,
			parseAnthropicText,
		},
		{
			"anthropic refusal",
			`{"content":[{"type":"text","text":"` + partial + `"}],"stop_reason":"refusal"}`,
			parseAnthropicText,
		},
		{
			"gemini MAX_TOKENS",
			`{"candidates":[{"content":{"parts":[{"text":"` + partial + `"}]},"finishReason":"MAX_TOKENS"}]}`,
			parseGeminiText,
		},
		{
			"openai length",
			`{"choices":[{"message":{"content":"` + partial + `"},"finish_reason":"length"}]}`,
			parseOpenAIText,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.parse([]byte(tc.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(got, "Cut short") {
				t.Fatalf("a truncated answer was returned unmarked, so the reader sees a "+
					"sentence stopping mid-word as though the model had finished:\n%q", got)
			}
			if !strings.Contains(got, partial) {
				t.Fatalf("the partial answer was discarded; it was paid for and is still "+
					"useful, so it must be kept and merely marked:\n%q", got)
			}
		})
	}
}

// The control for the test above: a COMPLETE answer must never be marked, or
// every answer in the app grows a warning that means nothing.
func TestParsersLeaveAFinishedAnswerAlone(t *testing.T) {
	const whole = "This verse comes near the end of Jeremiah 23, and the point is plain."

	for _, tc := range []struct {
		name  string
		body  string
		parse func([]byte) (string, error)
	}{
		{"anthropic end_turn",
			`{"content":[{"type":"text","text":"` + whole + `"}],"stop_reason":"end_turn"}`,
			parseAnthropicText},
		{"anthropic stop_sequence",
			`{"content":[{"type":"text","text":"` + whole + `"}],"stop_reason":"stop_sequence"}`,
			parseAnthropicText},
		{"gemini STOP",
			`{"candidates":[{"content":{"parts":[{"text":"` + whole + `"}]},"finishReason":"STOP"}]}`,
			parseGeminiText},
		{"openai stop",
			`{"choices":[{"message":{"content":"` + whole + `"},"finish_reason":"stop"}]}`,
			parseOpenAIText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.parse([]byte(tc.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != whole {
				t.Fatalf("a finished answer was altered; it must be returned verbatim:\n%q", got)
			}
		})
	}
}

// An early stop that leaves NOTHING behind stays an error — a notice with no
// answer above it would be worse than saying plainly that nothing came back.
func TestAnEmptyTruncatedAnswerIsStillAnError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		parse func([]byte) (string, error)
	}{
		{"anthropic", `{"content":[{"type":"text","text":""}],"stop_reason":"max_tokens"}`, parseAnthropicText},
		{"gemini", `{"candidates":[{"content":{"parts":[{"text":""}]},"finishReason":"MAX_TOKENS"}]}`, parseGeminiText},
		{"openai", `{"choices":[{"message":{"content":""},"finish_reason":"length"}]}`, parseOpenAIText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.parse([]byte(tc.body))
			if !errors.Is(err, errBudgetExhausted) {
				t.Fatalf("want errBudgetExhausted for an empty truncated answer, got %v", err)
			}
		})
	}
}

// An empty REFUSAL is the model declining, not an empty reply. Observed live:
// one model refused Jeremiah 23 part-way through, then answered it in full on
// the next attempt.
func TestAnEmptyRefusalSaysTheModelDeclined(t *testing.T) {
	_, err := parseAnthropicText([]byte(`{"content":[],"stop_reason":"refusal"}`))
	if err == nil {
		t.Fatal("a refusal with no text must be an error")
	}
	if !strings.Contains(err.Error(), "declined") {
		t.Fatalf("a refusal must say the model declined, not blame an empty answer: %v", err)
	}
}
