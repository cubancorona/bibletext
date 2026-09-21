package bibletext

// The app-association files decide whether a shared link opens the APP.
//
// The one way this feature can genuinely break something is by claiming too
// much of the domain: privacy.html and support.html are the URLs registered
// with App Store Connect, which Apple's own reviewer opens, and the landing
// page is where every download link lives. If the app claimed those, tapping
// them on a device with BibleText installed would bounce into the app instead
// of the browser.
//
// So the scope is an ALLOW-LIST, and this test is the tripwire against a
// well-meaning future edit widening it to "/*" — a change that would look
// harmless in review and only misbehave on a device with the app installed.

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"strings"
	"testing"
)

type aasaFile struct {
	Applinks struct {
		Details []struct {
			AppIDs     []string `json:"appIDs"`
			Components []struct {
				Path    string `json:"/"`
				Exclude bool   `json:"exclude"`
			} `json:"components"`
		} `json:"details"`
	} `json:"applinks"`
}

func TestAppleAppSiteAssociationScope(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-app-site-association")
	if err != nil {
		t.Fatalf("the Apple association file must exist and be published: %v", err)
	}
	var f aasaFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("not valid JSON — Apple would reject it silently: %v", err)
	}
	if len(f.Applinks.Details) != 1 {
		t.Fatalf("want exactly one details entry, got %d", len(f.Applinks.Details))
	}
	d := f.Applinks.Details[0]
	if len(d.AppIDs) != 1 || d.AppIDs[0] != "R8PC7239T2.uk.co.bibletext" {
		t.Errorf("appIDs = %v, want [R8PC7239T2.uk.co.bibletext]", d.AppIDs)
	}

	// Components are evaluated top to bottom, first match wins. Walk them the
	// way iOS does and assert what each URL resolves to.
	resolves := func(url string) (claimed bool) {
		for _, c := range d.Components {
			if matchAASAPath(c.Path, url) {
				return !c.Exclude
			}
		}
		return false // matching nothing means "not a universal link"
	}

	for _, mustNotClaim := range []string{
		"/privacy.html", "/support.html", "/", "/index.html", "/404.html",
		"/assets/reader.css", "/assets/reader.js",
	} {
		if resolves(mustNotClaim) {
			t.Errorf("%s is claimed by the app — it must always open in the browser "+
				"(privacy and support are the URLs App Store Connect points at)", mustNotClaim)
		}
	}
	for _, mustClaim := range []string{
		"/web/john/3/", "/bsb/psalms/23/", "/webc/1-maccabees/2/",
		// /nkjv/ matters MORE than the other three, not less: the site publishes
		// no pages there, so an unclaimed NKJV link does not degrade to a web
		// page — it degrades to a 404.
		"/nkjv/john/3/",
	} {
		if !resolves(mustClaim) {
			t.Errorf("%s is NOT claimed — shared links to it would not open the app", mustClaim)
		}
	}

	// Every id a link path may name must be claimed, or the app emits links it
	// cannot receive. Derived from the grammar itself so a fifth id cannot be
	// added to share_link.go and forgotten here.
	for id := range linkPathVersionIDs {
		if !resolves("/" + id + "/john/3/") {
			t.Errorf("/%s/ is a link path id but is not claimed in the association file", id)
		}
	}
}

// TestAndroidManifestClaimsEveryLinkPath is the same tripwire for Android. The
// two files are edited by hand, separately, and an id added to one and not the
// other produces a link that opens the app on one platform and 404s on the
// other — with nothing failing at build time to say so.
// PARSED, NOT GREPPED, and the difference is the whole point.
//
// This test used to run strings.Contains over the raw manifest. Deleting a
// pathPrefix failed it correctly -- and COMMENTING OUT THE ENTIRE
// intent-filter did not, because every string it looks for survives inside an
// XML comment. The App Links claim could have been switched off in full with
// nothing anywhere noticing, which is not hypothetical: commit 3587ae3ee
// removed that filter deliberately on 9 August 2026 and it was restored the
// same day.
//
// encoding/xml discards comments, so a commented-out filter simply is not
// there to be found. The claim now has to be live in the document to pass.
type androidManifest struct {
	Application struct {
		Activities []struct {
			IntentFilters []struct {
				AutoVerify string `xml:"http://schemas.android.com/apk/res/android autoVerify,attr"`
				Data       []struct {
					Scheme     string `xml:"http://schemas.android.com/apk/res/android scheme,attr"`
					Host       string `xml:"http://schemas.android.com/apk/res/android host,attr"`
					PathPrefix string `xml:"http://schemas.android.com/apk/res/android pathPrefix,attr"`
					PathPattern string `xml:"http://schemas.android.com/apk/res/android pathPattern,attr"`
				} `xml:"data"`
			} `xml:"intent-filter"`
		} `xml:"activity"`
	} `xml:"application"`
}

func TestAndroidManifestClaimsEveryLinkPath(t *testing.T) {
	raw, err := os.ReadFile("cmd/mobile/AndroidManifest.xml")
	if err != nil {
		t.Fatalf("the custom manifest must exist — fyne's generated one claims no links: %v", err)
	}
	var m androidManifest
	if err := xml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("cmd/mobile/AndroidManifest.xml does not parse as XML: %v", err)
	}
	// The claim must be LIVE: an autoVerify filter carrying https data for the
	// site. Collect what it actually declares.
	claimed := map[string]bool{}
	filters := 0
	for _, a := range m.Application.Activities {
		for _, f := range a.IntentFilters {
			if f.AutoVerify != "true" {
				continue
			}
			filters++
			for _, d := range f.Data {
				if d.Scheme == "https" && d.Host == SiteHost() {
					if d.PathPrefix != "" {
						claimed[strings.Trim(d.PathPrefix, "/")] = true
					}
					// A bare-host or wildcard claim would swallow the privacy
					// and support URLs the stores point at.
					if d.PathPrefix == "/" || d.PathPrefix == "" || d.PathPattern != "" {
						t.Errorf("the manifest claims the whole host (pathPrefix=%q pathPattern=%q) — "+
							"privacy and support must stay in the browser", d.PathPrefix, d.PathPattern)
					}
				}
			}
		}
	}
	if filters == 0 {
		t.Fatal("no LIVE autoVerify intent-filter for " + SiteHost() + " — the Android build claims no " +
			"App Links at all, so every shared link opens the browser. A filter that is present but " +
			"commented out counts as absent, which is the case this parse exists to catch.")
	}
	for id := range linkPathVersionIDs {
		if !claimed[id] {
			t.Errorf("/%s/ is not claimed by a live intent-filter — an emitted /%s/ link "+
				"would open the browser, not the app", id, id)
		}
	}
}

// matchAASAPath implements the subset of Apple's pattern syntax this file uses:
// a literal path, optionally ending in "*".
func matchAASAPath(pattern, url string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(url, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == url
}

// TestAssetLinksMatchesTheSignedApp: Android verifies the app's signing
// certificate against this file. A wrong fingerprint means verification fails
// silently and links quietly stop opening the app.
func TestAssetLinksMatchesTheSignedApp(t *testing.T) {
	raw, err := os.ReadFile("docs/assetlinks.json")
	if err != nil {
		t.Fatalf("the Android association file must exist and be published: %v", err)
	}
	var links []struct {
		Relation []string `json:"relation"`
		Target   struct {
			Namespace    string   `json:"namespace"`
			PackageName  string   `json:"package_name"`
			Fingerprints []string `json:"sha256_cert_fingerprints"`
		} `json:"target"`
	}
	if err := json.Unmarshal(raw, &links); err != nil {
		t.Fatalf("not valid JSON — Android would fail verification: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("want one statement, got %d", len(links))
	}
	l := links[0]
	if l.Target.PackageName != "uk.co.bibletext" {
		t.Errorf("package_name = %q, want uk.co.bibletext", l.Target.PackageName)
	}
	if l.Target.Namespace != "android_app" {
		t.Errorf("namespace = %q, want android_app", l.Target.Namespace)
	}
	if len(l.Target.Fingerprints) == 0 {
		t.Fatal("no signing fingerprint — verification cannot succeed")
	}
	// Read from the APK that is actually distributed:
	//   apksigner verify --print-certs BibleText-Android.apk
	const shipped = "41:B3:DF:F3:D9:9D:5C:7E:F7:79:A7:DA:8F:85:F5:C3:84:18:C2:3C:19:FE:9D:86:D9:95:30:0B:90:59:07:55"
	found := false
	for _, fp := range l.Target.Fingerprints {
		if strings.EqualFold(fp, shipped) {
			found = true
		}
	}
	if !found {
		t.Errorf("the release signing fingerprint is missing from assetlinks.json;\n got %v\nwant %s",
			l.Target.Fingerprints, shipped)
	}
}
