package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// NO TEST IN THIS PACKAGE REACHES API.BIBLE, A FEED, OR A REAL KEY.
//
// The generator's two network paths are variables (loadPublished,
// fetchLicensedEdition), and here they are replaced for the whole binary with
// ones that fail the run loudly: a test that wants either stands in its own
// fixture and restores it. The credentials a development shell may hold —
// BIBLE_API_KEY, the site's key, the operator's licence and provider-id
// overrides, the QA unlock — are taken out of the environment before any test
// runs, as the root package's TestMain does, so a run from that shell is the
// run CI makes. A test that needs one sets it with t.Setenv.
func TestMain(m *testing.M) {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case name == "BIBLE_API_KEY", name == siteKeyEnv, name == "BIBLETEXT_ENABLE_TESTING",
			strings.HasPrefix(name, "BIBLETEXT_LICENSE_"), strings.HasPrefix(name, "BIBLETEXT_PROVIDER_ID_"):
			os.Unsetenv(name)
		}
	}
	fetchLicensedEdition = func(id, _ string) (bibletext.LicensedEdition, error) {
		panic(fmt.Sprintf("a test reached the real API.Bible fetch for %q; stand in fetchLicensedEdition", id))
	}
	loadPublished = func(string, bool) ([]loadedVersion, error) {
		panic("a test reached the real feed download; stand in loadPublished")
	}
	os.Exit(m.Run())
}
