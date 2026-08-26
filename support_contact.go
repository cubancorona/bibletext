package bibletext

import (
	_ "embed"
	"net/url"
	"regexp"
	"strings"
)

//go:embed config/support-email-pattern.txt
var supportEmailPatternSource string

var supportEmailPattern = regexp.MustCompile("^(?:" + mustSupportEmailPattern(supportEmailPatternSource) + ")$")

var supportEmail = mustSupportEmail(product.SupportEmail)

var supportMailtoRecipient = formatMailtoRecipient(supportEmail)

// SupportEmail returns the public mailbox used by product support surfaces.
// Its only tracked value lives in config/product.json.
func SupportEmail() string {
	return supportEmail
}

// SupportMailtoRecipient returns the configured mailbox escaped for the
// recipient component of a mailto URI.
func SupportMailtoRecipient() string {
	return supportMailtoRecipient
}

func mustSupportEmailPattern(raw string) string {
	value := strings.TrimSuffix(raw, "\n")
	value = strings.TrimSuffix(value, "\r")
	if value == "" ||
		(raw != value && raw != value+"\n" && raw != value+"\r\n") ||
		strings.ContainsAny(value, "\r\n") {
		panic("config/support-email-pattern.txt must contain exactly one non-empty pattern")
	}
	return value
}

func mustSupportEmail(raw string) string {
	value := strings.TrimSuffix(raw, "\n")
	value = strings.TrimSuffix(value, "\r")
	if value == "" ||
		(raw != value && raw != value+"\n" && raw != value+"\r\n") ||
		strings.TrimSpace(value) != value ||
		strings.ContainsAny(value, "\r\n") {
		panic("config/product.json: supportEmail must be exactly one non-empty email address")
	}
	if !supportEmailPattern.MatchString(value) {
		panic("config/product.json: supportEmail must be one conservative ASCII email address")
	}
	return value
}

func formatMailtoRecipient(mailbox string) string {
	if !supportEmailPattern.MatchString(mailbox) {
		panic("mailto recipient does not match the public support mailbox grammar")
	}
	return url.PathEscape(mailbox)
}
