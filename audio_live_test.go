package bibletext

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestLiveAudioAssetsExist reconciles what the app ADVERTISES against what the
// audio host actually holds. The app decides a chapter has a recording by looking
// it up in a bundled timing table (recordingHasChapter), which says what was
// aligned — not what was uploaded. Those two drifted apart in practice: BSB Daniel
// 7-10 and Romans 7 were advertised for weeks while their MP3s were missing from
// the release, so every reader who opened them got a failed stream instead of
// audio, and nothing in the build could tell. Counting assets per release is
// enough to notice, but only comparing NAMES proves it, which is what this does.
//
// It asks GitHub for each release's asset list (a handful of API calls) rather
// than requesting 2,500 MP3s, and it builds the expected names by calling the real
// urlFor functions, so it tests the exact strings the player will fetch and cannot
// drift from them. Network, so it skips by default:
//
//	BIBLETEXT_CHECK_AUDIO=1 go test -run TestLiveAudioAssetsExist -v .
//
// Run it after uploading a recording and before shipping one.
func TestLiveAudioAssetsExist(t *testing.T) {
	if os.Getenv("BIBLETEXT_CHECK_AUDIO") != "1" {
		t.Skip("set BIBLETEXT_CHECK_AUDIO=1 to reconcile the audio host (network)")
	}
	// Only meaningful against a GitHub-releases host; a fork pointing product.json
	// somewhere else is not a failure of this repo.
	const marker = "/releases/download/"
	if !strings.HasPrefix(audioHostBase, "https://github.com/") || !strings.HasSuffix(audioHostBase, marker) {
		t.Skipf("audio host %q is not a GitHub releases host — nothing to reconcile", audioHostBase)
	}
	repo := strings.TrimSuffix(strings.TrimPrefix(audioHostBase, "https://github.com/"), marker)

	// Every chapter every recording claims, as <tag>/<filename>.
	loadTimings()
	want := map[string]map[string]string{} // tag -> filename -> "version/recording Book Ch"
	for _, version := range []string{"bsb", "web", "webc"} {
		for _, rec := range recordingsFor(version) {
			for book, chs := range allTimings[rec.id] {
				for chStr := range chs {
					ch, err := strconv.Atoi(chStr)
					if err != nil {
						t.Fatalf("%s: non-numeric chapter %q for %s", rec.id, chStr, book)
					}
					url, ok := rec.urlFor(book, ch)
					if !ok {
						continue // another recording on this version owns the chapter
					}
					rest := strings.TrimPrefix(url, audioHostBase)
					tag, file, found := strings.Cut(rest, "/")
					if !found {
						t.Fatalf("%s %s %d: URL %q is not <host><tag>/<file>", rec.id, book, ch, url)
					}
					if want[tag] == nil {
						want[tag] = map[string]string{}
					}
					want[tag][file] = fmt.Sprintf("%s/%s %s %d", version, rec.id, book, ch)
				}
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("no advertised chapters found — the reconciliation would prove nothing")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	total, missing := 0, 0
	for tag, files := range want {
		have, err := releaseAssetNames(client, repo, tag)
		if err != nil {
			t.Errorf("release %q: %v", tag, err)
			continue
		}
		var absent []string
		for f, where := range files {
			total++
			if !have[f] {
				absent = append(absent, fmt.Sprintf("%s (%s)", f, where))
			}
		}
		sort.Strings(absent)
		if len(absent) > 0 {
			missing += len(absent)
			t.Errorf("release %q is missing %d of the %d chapters the app advertises:\n  %s",
				tag, len(absent), len(files), strings.Join(absent, "\n  "))
		}
		t.Logf("%s: %d advertised, %d present on the host, %d assets in the release", tag, len(files), len(files)-len(absent), len(have))
	}
	if total == 0 {
		t.Fatal("no chapters were checked — the reconciliation proved nothing")
	}
	t.Logf("reconciled %d advertised chapters across %d releases; %d missing", total, len(want), missing)
}

// releaseAssetNames lists a release's asset filenames, following pagination —
// the WEB and BSB Old Testament releases hold over 900 assets each, far past the
// default page size, and a partial list would read as missing files.
func releaseAssetNames(client *http.Client, repo, tag string) (map[string]bool, error) {
	get := func(url string, into any) error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok) // lifts the 60/hour anonymous limit
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET %s: %s", url, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(into)
	}

	var rel struct {
		ID int64 `json:"id"`
	}
	if err := get(fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag), &rel); err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for page := 1; ; page++ {
		var assets []struct {
			Name string `json:"name"`
		}
		u := fmt.Sprintf("https://api.github.com/repos/%s/releases/%d/assets?per_page=100&page=%d", repo, rel.ID, page)
		if err := get(u, &assets); err != nil {
			return nil, err
		}
		if len(assets) == 0 {
			break
		}
		for _, a := range assets {
			names[a.Name] = true
		}
	}
	return names, nil
}
