package bibletext

// The AI error screens' wording, word for word. Each line was decided as
// written (docs/AI_ERRORS.md), so each is compared with its exact bytes,
// the straight apostrophes and the curly quotation marks of the quoted
// reason included: a change of one character to any of them fails here.

import (
	"context"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// aiErrorActionRows is the text of each row aiErrorActions lays out, top to
// bottom.
func aiErrorActionRows(t *testing.T, actions *fyne.Container) []string {
	t.Helper()
	var rows []string
	for _, row := range actions.Objects {
		texts := treeTexts(row)
		if len(texts) != 1 {
			t.Fatalf("an action row reads %v, not one action", texts)
		}
		rows = append(rows, texts[0])
	}
	return rows
}

func TestTheAIErrorWordingWordForWord(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	type pin struct{ name, got, want string }
	var pins []pin
	add := func(name, got, want string) { pins = append(pins, pin{name, got, want}) }
	billingBody := []byte(`{"error":{"type":"billing_error","message":"Payment information needs attention. synthetic-private-tail"}}`)
	for _, tc := range []struct {
		id, vendor, account, billing, said, link, timeout, timeoutAlone string
	}{
		{providerOpenAI, "OpenAI",
			"ChatGPT didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with OpenAI; nothing in BibleText needs changing.",
			"ChatGPT didn't accept the request because of a billing issue with the account behind your key. You can check that with OpenAI; nothing in BibleText needs changing.",
			"OpenAI said: “You exceeded your current quota, please check your plan and billing details”",
			"ChatGPT billing",
			"ChatGPT took too long to answer, so the request was stopped. Try again, or switch to a faster model.",
			"ChatGPT took too long to answer, so the request was stopped. Try again."},
		{providerAnthropic, "Anthropic",
			"Claude didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with Anthropic; nothing in BibleText needs changing.",
			"Claude didn't accept the request because of a billing issue with the account behind your key. You can check that with Anthropic; nothing in BibleText needs changing.",
			"Anthropic said: “Your credit balance is too low”",
			"Claude billing",
			"Claude took too long to answer, so the request was stopped. Try again, or switch to a faster model.",
			"Claude took too long to answer, so the request was stopped. Try again."},
		{providerGemini, "Google",
			"Gemini didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with Google; nothing in BibleText needs changing.",
			"Gemini didn't accept the request because of a billing issue with the account behind your key. You can check that with Google; nothing in BibleText needs changing.",
			"Google said: “Your Prepay credit balance is depleted”",
			"Gemini billing",
			"Gemini took too long to answer, so the request was stopped. Try again, or switch to a faster model.",
			"Gemini took too long to answer, so the request was stopped. Try again."},
		{providerGrok, "SpaceXAI",
			"Grok didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with SpaceXAI; nothing in BibleText needs changing.",
			"Grok didn't accept the request because of a billing issue with the account behind your key. You can check that with SpaceXAI; nothing in BibleText needs changing.",
			"SpaceXAI said: “Your team has exhausted its credits”",
			"Grok billing",
			"Grok took too long to answer, so the request was stopped. Try again, or switch to a faster model.",
			"Grok took too long to answer, so the request was stopped. Try again."},
	} {
		info, ok := providerByID(tc.id)
		if !ok {
			t.Fatalf("control: no provider %q", tc.id)
		}
		vendor, _ := aiBillingDestination(info)
		add(tc.id+" vendor", vendor, tc.vendor)

		account := withAIErrorProvider(syntheticAIAccountLimit(info), info)
		add(tc.id+" account limit", friendlyAIError(account), tc.account)
		p, _ := aiSpecialError(account)
		add(tc.id+" quoted reason", p.detail, tc.said)
		add(tc.id+" billing", friendlyAIError(withAIErrorProvider(aiResponseFailure(402, billingBody), info)), tc.billing)

		timeout, _ := aiSpecialError(withAIErrorProvider(syntheticAITimeout(), info))
		add(tc.id+" timeout beside the faster model", timeout.shown(true), tc.timeout)
		add(tc.id+" timeout without it", timeout.shown(false), tc.timeoutAlone)

		// The rows as the screens lay them out: Try again above the billing
		// link, and above the faster model where it is offered.
		state := sampleState()
		state.aiKeys = newKeyStoreWith(newFakePrefs())
		state.keys().setActiveProvider(info.ID)
		state.keys().setAPIKey(info.ID, "synthetic-key")
		rows := aiErrorActionRows(t, aiErrorActions(state, p, func() {}))
		if len(rows) != 2 {
			t.Fatalf("%s: the account limit offers %v", tc.id, rows)
		}
		add(tc.id+" account limit, first row", rows[0], "Try again")
		add(tc.id+" account limit, second row", rows[1], tc.link)
		rows = aiErrorActionRows(t, aiErrorActions(state, timeout, func() {}))
		if len(rows) != 2 {
			t.Fatalf("%s: the timeout offers %v", tc.id, rows)
		}
		add(tc.id+" timeout, first row", rows[0], "Try again")
		add(tc.id+" timeout, second row", rows[1], "Switch to a faster model")

		// The quoted reason as the screen draws it.
		view := aiSearchErrorView(state, account, func() {})
		drawn := ""
		if said := aiErrorRichText(view, p.detail); said != nil {
			drawn = segmentText(said.Segments)
		}
		add(tc.id+" quoted reason drawn", drawn, tc.said)
	}

	// With no provider captured: "The AI service", "the provider", and no
	// billing link.
	fallback, _ := aiSpecialError(syntheticAIAccountError())
	add("no provider, account limit", friendlyAIError(syntheticAIAccountError()),
		"The AI service didn't accept the request: the account behind your key has reached its spending limit or run out of credit. You can change that with the provider; nothing in BibleText needs changing.")
	add("no provider, quoted reason", fallback.detail, "the provider said: “Your credit balance is too low”")
	add("no provider, billing", friendlyAIError(aiResponseFailure(402, billingBody)),
		"The AI service didn't accept the request because of a billing issue with the account behind your key. You can check that with the provider; nothing in BibleText needs changing.")
	add("no provider, timeout", friendlyAIError(syntheticAITimeout()),
		"The AI service took too long to answer, so the request was stopped. Try again.")
	vendor, address := aiBillingDestination(providerInfo{})
	add("no provider, vendor", vendor, "the provider")
	if address != "" || fallback.billingURL != "" || aiBillingLink(fallback) != nil {
		t.Error("with no provider captured the error offered a billing link")
	}
	rows := aiErrorActionRows(t, aiErrorActions(nil, fallback, func() {}))
	if len(rows) != 1 {
		t.Errorf("with no provider captured the error offers %v, not Try again alone", rows)
	}

	// A request the reader cancelled.
	gemini, _ := providerByID(providerGemini)
	add("cancelled", friendlyAIError(context.Canceled), "The request was cancelled.")
	add("cancelled, with its provider", friendlyAIError(withAIErrorProvider(context.Canceled, gemini)), "The request was cancelled.")

	for _, p := range pins {
		t.Run(p.name, func(t *testing.T) {
			if p.got != p.want {
				t.Errorf("reads %q\n         want %q", p.got, p.want)
			}
		})
	}
}
