package bibletext

import (
	"strings"
	"testing"
)

// The tracked identity file must parse, and the derived values every surface
// consumes must agree with it. These assertions look tautological until a
// refactor points one consumer at a literal again — then the pair drifts and
// the mismatch fails here rather than on a fork's first deploy.
func TestProductIdentityDrivesEverySurface(t *testing.T) {
	if SiteBase() != "https://"+SiteHost() {
		t.Errorf("SiteBase %q does not decompose to host %q", SiteBase(), SiteHost())
	}
	if shareLinkBase != SiteBase() {
		t.Errorf("share links build on %q, product says %q", shareLinkBase, SiteBase())
	}
	if shareLinkHost != SiteHost() {
		t.Errorf("share links parse against %q, product says %q", shareLinkHost, SiteHost())
	}
	if audioHostBase != product.AudioBase {
		t.Errorf("audio downloads from %q, product says %q", audioHostBase, product.AudioBase)
	}
	if SupportEmail() != product.SupportEmail {
		t.Errorf("support surfaces show %q, product says %q", SupportEmail(), product.SupportEmail)
	}
	// The impersonation guard must refuse the product's own name however it is
	// cased or spaced, or a sender can dress a note in the app's voice.
	for _, name := range []string{
		ProductName(),
		strings.ToUpper(ProductName()),
		ProductName() + " Support",
		" " + strings.ToLower(ProductName()) + " ",
	} {
		if !senderNameRefused(name) {
			t.Errorf("byline guard accepts %q, which impersonates the product", name)
		}
	}
}

// Every malformed identity file must refuse to start. The one-field-at-a-time
// table proves each validation can actually fire; a validator that cannot fail
// is decoration.
func TestProductIdentityRejectsMalformedFiles(t *testing.T) {
	good := string(productConfigSource)
	cases := map[string]string{
		"not json":            "{",
		"unknown field":       strings.Replace(good, `"productName"`, `"produtcName"`, 1),
		"empty name":          strings.Replace(good, `"BibleText"`, `""`, 1),
		"padded name":         strings.Replace(good, `"BibleText"`, `" BibleText"`, 1),
		"http site":           strings.Replace(good, `"https://bibletext.co.uk"`, `"http://bibletext.co.uk"`, 1),
		"site with path":      strings.Replace(good, `"https://bibletext.co.uk"`, `"https://bibletext.co.uk/site"`, 1),
		"uppercase app id":    strings.Replace(good, `"uk.co.bibletext"`, `"UK.co.bibletext"`, 1),
		"bare app id":         strings.Replace(good, `"uk.co.bibletext"`, `"bibletext"`, 1),
		"missing email":       strings.Replace(good, `"`+product.SupportEmail+`"`, `""`, 1),
		"audio not https":     strings.Replace(good, `"https://github.com/cubancorona/bibletext-audio/releases/download/"`, `"ftp://x/"`, 1),
		"audio bare scheme":   strings.Replace(good, `"https://github.com/cubancorona/bibletext-audio/releases/download/"`, `"https://"`, 1),
		"audio empty host":    strings.Replace(good, `"https://github.com/cubancorona/bibletext-audio/releases/download/"`, `"https:///x/"`, 1),
		"trailing object":     good + `{"productName":"Other"}`,
		"audio without slash": strings.Replace(good, `bibletext-audio/releases/download/"`, `bibletext-audio/releases/download"`, 1),
		"repo with fragment":  strings.Replace(good, `"https://github.com/cubancorona/bibletext"`, `"https://github.com/cubancorona/bibletext#main"`, 1),
		"lowercase secret":    strings.Replace(good, `"BIBLETEXT_BUNDLED_KEY_ENC"`, `"bibletext_bundled_key_enc"`, 1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if raw == good {
				t.Fatal("the mutation did not change the file; the case tests nothing")
			}
			defer func() {
				if recover() == nil {
					t.Error("malformed identity accepted without panic")
				}
			}()
			mustProductIdentity([]byte(raw))
		})
	}
}
