package bibletext

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// The project's identity lives in ONE tracked file, config/product.json, and
// every surface that names the product derives from it: the share-link URL
// contract, the audio host, the support mailbox, the byline impersonation
// guard, and the site generator's canonical URLs. A fork changes that file and
// nothing else in Go.
//
// Two kinds of identity deliberately stay OUTSIDE it. Values read by external
// build tools that cannot parse JSON stay in the formats those tools dictate —
// the two FyneApp.toml files, the Universal Links and assetlinks files, the
// Pages CNAME — and scripts/check-product-identity.py holds them equal to this
// file so they cannot drift. Per-publisher platform records (the Apple team
// id, the Android signing certificate fingerprint) are not product identity at
// all and belong to whoever ships the build.
//
// Every field is validated at init and an invalid file refuses to start, for
// the same reason config/support-email.txt did before it was folded in here: a
// half-edited fork must fail its first build loudly, not ship a product that
// quietly claims someone else's name.

//go:embed config/product.json
var productConfigSource []byte

type productIdentity struct {
	ProductName          string `json:"productName"`
	SiteBase             string `json:"siteBase"`
	AppID                string `json:"appID"`
	DesktopAppID         string `json:"desktopAppID"`
	SupportEmail         string `json:"supportEmail"`
	AudioBase            string `json:"audioBase"`
	SourceRepo           string `json:"sourceRepo"`
	BundledKeySecretName string `json:"bundledKeySecretName"`
}

var product = mustProductIdentity(productConfigSource)

// productSiteHost is the bare host of SiteBase ("bibletext.co.uk"), the form
// the share-link parser matches against.
var productSiteHost = mustProductSiteHost(product.SiteBase)

// productNameFolded is the case-folded product name, the form the byline
// impersonation guard compares sender names against.
var productNameFolded = strings.ToLower(product.ProductName)

var productAppIDPattern = regexp.MustCompile(`^[a-z0-9]+(\.[a-z0-9-]+)+$`)

func mustProductIdentity(raw []byte) productIdentity {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var p productIdentity
	if err := dec.Decode(&p); err != nil {
		panic("config/product.json does not parse: " + err.Error())
	}
	if strings.TrimSpace(p.ProductName) != p.ProductName || p.ProductName == "" {
		panic("config/product.json: productName must be non-empty with no surrounding whitespace")
	}
	mustHTTPSNoTrailingSlash("siteBase", p.SiteBase)
	if !productAppIDPattern.MatchString(p.AppID) {
		panic("config/product.json: appID must be a lowercase reverse-DNS identifier")
	}
	if !productAppIDPattern.MatchString(p.DesktopAppID) {
		panic("config/product.json: desktopAppID must be a lowercase reverse-DNS identifier")
	}
	if p.SupportEmail == "" {
		panic("config/product.json: supportEmail must be set")
	}
	if !strings.HasPrefix(p.AudioBase, "https://") || !strings.HasSuffix(p.AudioBase, "/") {
		panic("config/product.json: audioBase must be an https:// URL ending in /")
	}
	// sourceRepo is a repository URL, so unlike siteBase a path is expected;
	// only the scheme, a host, and the absence of decoration are enforced.
	if u, err := url.Parse(p.SourceRepo); err != nil || u.Scheme != "https" ||
		u.Host == "" || u.RawQuery != "" || u.Fragment != "" ||
		strings.HasSuffix(p.SourceRepo, "/") {
		panic("config/product.json: sourceRepo must be an https:// repository URL without trailing slash")
	}
	if strings.ToUpper(p.BundledKeySecretName) != p.BundledKeySecretName || p.BundledKeySecretName == "" {
		panic("config/product.json: bundledKeySecretName must be a non-empty UPPER_CASE name")
	}
	return p
}

func mustHTTPSNoTrailingSlash(field, value string) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" ||
		u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(value, "/") {
		panic("config/product.json: " + field + " must be a bare https:// origin with no path")
	}
}

func mustProductSiteHost(base string) string {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		panic("config/product.json: siteBase has no host")
	}
	return u.Host
}

// SiteBase returns the origin the published site and every shared link live on
// ("https://bibletext.co.uk"). Exported for the site generators, which build
// canonical URLs from it.
func SiteBase() string { return product.SiteBase }

// SiteHost returns SiteBase's bare host, the form deep-link matching uses.
func SiteHost() string { return productSiteHost }

// ProductName returns the product's display name.
func ProductName() string { return product.ProductName }

// AppID returns the mobile bundle/application identifier.
func AppID() string { return product.AppID }
