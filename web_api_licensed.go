package bibletext

import (
	"fmt"
	"sort"
	"strings"
)

// The seams cmd/websitegen needs to publish a LICENSED translation's text —
// today the NKJV, behind the one switch in cmd/websitegen/nkjv_text.go.
//
// They sit beside web_api.go's for the same reason: the site renders from the
// app's own fetch, decoder and versification, so the page cannot disagree with
// the app about what the text is. What is different is where the text comes
// from. The public-domain editions are one key-less download each; a licensed
// one is a whole-canon walk of API.Bible with a key, and the site does that
// walk afresh on every build rather than keeping a copy.

// LicensedEdition is a licensed translation fetched whole for the web reader,
// with the accounting the generator logs and checks.
type LicensedEdition struct {
	Bible *BibleData
	// Requests is how many API.Bible requests the fetch made: the quota it
	// spent, which the Starter plan bills per call.
	Requests int64
	// CopyrightLines is how many DISTINCT copyright lines the responses
	// carried. The site prints the registry's one notice on every page of the
	// text, which is honest only while the provider sends one line throughout.
	CopyrightLines int
}

// FetchLicensedEdition downloads a licensed translation whole from API.Bible
// through the same walk, decoder and validation the app uses (fetchAPIBible),
// and through nothing else.
//
// It differs from the app's licensedAPISource.fetch in three ways, each on
// purpose:
//
//   - The key is the argument and only the argument. BIBLE_API_KEY, the
//     reader's stored key and the compiled release fallback are never
//     consulted, so the site cannot fetch with a credential its publisher did
//     not hand it, and an empty key fails before any request is sent.
//   - The provider id is the registry's own (defaultProviderBibleID).
//     BIBLETEXT_PROVIDER_ID_<ID> and BIBLETEXT_LICENSE_<ID> are ignored: no
//     environment variable can point the site at a different bible.
//   - Nothing is written to disk. The app keeps a licensed copy under
//     API.Bible's recency terms; the site fetches afresh on every build, so a
//     copy it kept would only be one more thing to expire.
//
// It is not safe to run beside another API.Bible fetch in the same process,
// because the request and copyright counts are package-wide. The generator
// runs exactly one.
func FetchLicensedEdition(versionID, apiKey string) (LicensedEdition, error) {
	v, ok := versionByID(versionID)
	if !ok {
		return LicensedEdition{}, fmt.Errorf("%q is not a registered translation", versionID)
	}
	src, ok := v.source.(*licensedAPISource)
	if !ok || src.defaultProviderBibleID == "" {
		return LicensedEdition{}, fmt.Errorf("%q is not a licensed API.Bible translation with a known provider id", versionID)
	}
	if apiKey == "" {
		return LicensedEdition{}, fmt.Errorf("%s: no API.Bible key was supplied", strings.ToUpper(versionID))
	}

	apiBibleCopyrightMu.Lock()
	apiBibleCopyrightSeen = map[string]int{}
	apiBibleCopyrightMu.Unlock()
	before := apiBibleCallCount.Load()

	bd, err := fetchAPIBible(strings.ToUpper(versionID), src.defaultProviderBibleID, apiKey)
	out := LicensedEdition{Requests: apiBibleCallCount.Load() - before}
	if err != nil {
		return out, err
	}
	apiBibleCopyrightMu.Lock()
	out.CopyrightLines = len(apiBibleCopyrightSeen)
	apiBibleCopyrightMu.Unlock()
	out.Bible = bd
	return out, nil
}

// VersionLicenseNotice is the attribution the rights holder requires with a
// translation's text (BibleVersion.LicenseNotice), or "" for a public-domain
// or unknown id. The web reader prints it on every page of licensed text, so
// the site and the app credit the edition in the same words.
func VersionLicenseNotice(id string) string {
	v, ok := versionByID(id)
	if !ok {
		return ""
	}
	return v.LicenseNotice
}

// WebSmallCapitalRunes are the characters the small capitals are drawn with
// (smallCapitals, small_caps_draw.go), sorted. The web reader's supplementary
// face for a licensed edition must carry every one of them, and its test asks
// this rather than keeping a copy of the table.
func WebSmallCapitalRunes() []rune {
	out := make([]rune, 0, len(smallCapitals))
	for _, sc := range smallCapitals {
		out = append(out, sc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
