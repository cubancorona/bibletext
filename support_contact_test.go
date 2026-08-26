package bibletext

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSupportEmailConfiguration(t *testing.T) {
	if got := SupportEmail(); got == "" {
		t.Fatal("SupportEmail returned an empty address")
	}
	if got := mustSupportEmail(product.SupportEmail); got != SupportEmail() {
		t.Fatalf("embedded support configuration resolved inconsistently")
	}
	if got := mustSupportEmail("support@example.invalid\r\n"); got != "support@example.invalid" {
		t.Fatalf("CRLF configuration resolved to %q", got)
	}
	if got := mustSupportEmail("support+site@example.invalid\n"); got != "support+site@example.invalid" {
		t.Fatalf("plus-tag configuration resolved to %q", got)
	}
}

func TestSupportEmailConfigurationRejectsInvalidInput(t *testing.T) {
	for _, raw := range []string{
		"",
		"not-an-email\n",
		"Support <support@example.invalid>\n",
		"support@example.invalid\nsecond@example.invalid\n",
		"support@example.invalid\r",
		"support@example.invalid\n\n",
		" support@example.invalid\n",
		".support@example.invalid\n",
		"support.@example.invalid\n",
		"support..site@example.invalid\n",
		"support@.example.invalid\n",
		"support@example..invalid\n",
		"support@example.invalid.\n",
		"support@-example.invalid\n",
		"support@example-.invalid\n",
		"support?tag@example.invalid\n",
		"support#tag@example.invalid\n",
		"support&tag@example.invalid\n",
		"support/tag@example.invalid\n",
		"support%tag@example.invalid\n",
		"support=tag@example.invalid\n",
		"support:tag@example.invalid\n",
		"support;tag@example.invalid\n",
		"support,tag@example.invalid\n",
	} {
		raw := raw
		t.Run(strings.ReplaceAll(raw, "\n", "_"), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid support configuration did not panic")
				}
			}()
			mustSupportEmail(raw)
		})
	}
}

func TestSupportMailtoRecipientRoundTrip(t *testing.T) {
	const mailbox = "support+site@example.invalid"
	recipient := formatMailtoRecipient(mailbox)
	message := (&url.URL{
		Scheme:   "mailto",
		Opaque:   recipient,
		RawQuery: url.Values{"subject": {"Synthetic subject"}, "body": {"Synthetic body"}}.Encode(),
	}).String()
	parsed, err := url.Parse(message)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := url.PathUnescape(parsed.Opaque)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != mailbox {
		t.Fatalf("mailto recipient round trip resolved to %q", decoded)
	}
	if got := parsed.Query().Get("subject"); got != "Synthetic subject" {
		t.Fatalf("mailto subject round trip resolved to %q", got)
	}
	if got := parsed.Query().Get("body"); got != "Synthetic body" {
		t.Fatalf("mailto body round trip resolved to %q", got)
	}
	if got := SupportMailtoRecipient(); got != formatMailtoRecipient(SupportEmail()) {
		t.Fatal("configured mailto recipient was not derived from the support mailbox")
	}
}

func TestMailtoRecipientRejectsURIDelimiters(t *testing.T) {
	for _, mailbox := range []string{
		"support?tag@example.invalid",
		"support#tag@example.invalid",
		"support&tag@example.invalid",
		"support/tag@example.invalid",
		"support%tag@example.invalid",
		"support=tag@example.invalid",
		"support:tag@example.invalid",
		"support;tag@example.invalid",
		"support,tag@example.invalid",
	} {
		mailbox := mailbox
		t.Run(mailbox, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("URI delimiter was accepted in a mailto recipient")
				}
			}()
			formatMailtoRecipient(mailbox)
		})
	}
}

func TestSupportEmailHasOneTrackedSource(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list repository files: %v", err)
	}
	want := []byte(SupportEmail())
	var copies []string
	for _, rawPath := range bytes.Split(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if bytes.Contains(data, want) {
			copies = append(copies, path)
		}
	}
	if len(copies) != 1 || copies[0] != "config/product.json" {
		t.Fatalf("the public support address must occur only in config/product.json; found it in %v", copies)
	}
}
