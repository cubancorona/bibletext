package main

// THE NKJV'S TEXT ON THE SITE, BEHIND ONE SWITCH.
//
// The app emits /nkjv/<book>/<chapter>/ share links, and the site answers every
// one of them in one of two ways, chosen by the constant below:
//
//	off  notice pages (notice.go): the passage is named, the text is not, and
//	     the reader is offered the app and the same passage in a public-domain
//	     edition. No request is made to API.Bible and no key is read.
//	on   the text itself, rendered exactly as the public-domain editions are,
//	     with the rights holder's notice and a line saying when and where this
//	     build retrieved it at the foot of every /nkjv/ page.
//
// ON FETCHES AFRESH, EVERY TIME. Each build downloads the whole NKJV from
// API.Bible (about two hundred requests) into memory, renders from it and
// drops it. Nothing is cached on disk, so there is no stale copy to expire and
// no licensed file for an interrupted build to leave behind; the price is that
// a dry run spends the same quota a publish does.
//
// WHY A CONSTANT. It is how this repository gates a capability that ships
// dormant (senderNamesEnabled in notes_byline.go, readingJustifyProse in
// reading_page.go): both states stay compiled, so `go test ./...` exercises
// both, which a build tag could not do in one run. Everything below takes the
// state as an argument; main() reads the constant once and hands it down.

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // the retrieval date is London's, on any build host
	"unicode"

	bibletext "github.com/cubancorona/bibletext"
)

// nkjvSiteText is the ONE switch for the NKJV's text on bibletext.co.uk.
//
//	true:  every build fetches the whole NKJV fresh from API.Bible (in memory,
//	       never cached), writes /nkjv/ as scripture pages carrying the
//	       copyright notice and the retrieval line, and adds the NKJV to the
//	       version switcher, the "Go to" picker and the 404.
//	false: the site as it was before the switch existed — /nkjv/ notice pages,
//	       no API call, no key.
//
// Flip it here and nowhere else. scripts/publish-site.sh asks the built
// generator which state it is publishing (-print-nkjv-text) and its guards
// follow the answer. There is deliberately no environment variable or flag
// that overrides it: the committed value is the state the site is in.
const nkjvSiteText = false

// nkjvTextState is the switch as scripts/publish-site.sh reads it.
func nkjvTextState(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// siteLicensedVersions are the licensed translations the app emits share links
// for. The site serves every one of them: as text while the switch is on, as
// notice pages while it is off. Today that is the NKJV alone.
//
// An entry here carries an id and a display name and nothing else. Its text
// can reach the site only through loadLicensed, which only the switch reaches;
// publishedVersions, which carries a decoder and a download URL, stays the
// public-domain three (licensed_exclusion_test.go).
func siteLicensedVersions() []noticeVersion {
	return []noticeVersion{{ID: "nkjv", Name: "New King James Version"}}
}

// noticedVersionsFor is what the site writes notice trees for: every licensed
// translation while the switch is off, none while it is on.
func noticedVersionsFor(on bool) []noticeVersion {
	if on {
		return nil
	}
	return siteLicensedVersions()
}

// licensedVersionsFor is what the site fetches and publishes the text of:
// none while the switch is off, every licensed translation while it is on.
func licensedVersionsFor(on bool) []noticeVersion {
	if on {
		return siteLicensedVersions()
	}
	return nil
}

// siteKeyEnv carries the API.Bible key into one run of the generator while the
// switch is on. scripts/release-bible-key.sh's run_with_site_bible_key reads
// it from the login Keychain and sets it on this process alone; takeSiteKey
// removes it from the environment before anything else runs.
//
// A variable of its own rather than BIBLE_API_KEY, which a development shell
// may hold for the app: the site fetches with the key it is handed for the
// purpose, and with no other.
const siteKeyEnv = "BIBLETEXT_SITE_NKJV_KEY"

// fetchLicensedEdition is the network. A variable so the tests can stand in for
// API.Bible; nothing else assigns it.
var fetchLicensedEdition = bibletext.FetchLicensedEdition

// takeSiteKey reads the key and takes it out of the environment straight away,
// so no later child process could inherit it. Its errors describe the value and
// never contain it.
func takeSiteKey() (string, error) {
	key := os.Getenv(siteKeyEnv)
	os.Unsetenv(siteKeyEnv)
	switch {
	case key == "":
		return "", fmt.Errorf("the NKJV's text is switched on (cmd/websitegen/nkjv_text.go) and %s is not set; "+
			"scripts/publish-site.sh supplies it from the login Keychain", siteKeyEnv)
	case len(key) < 16 || len(key) > 512:
		return "", fmt.Errorf("%s has an invalid length", siteKeyEnv)
	case strings.ContainsFunc(key, unicode.IsSpace):
		return "", fmt.Errorf("%s contains whitespace", siteKeyEnv)
	}
	return key, nil
}

// webLicence is what a page of licensed text must say about it: the rights
// holder's notice, and the London civil date this build retrieved the text.
type webLicence struct {
	Notice    string
	Retrieved time.Time
}

// london is the zone the retrieval date is stated in. time/tzdata is imported
// above, so a host without a zone database still has it.
var london = func() *time.Location {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(fmt.Sprintf("no Europe/London zone: %v", err))
	}
	return loc
}()

// londonDate is the civil date in London at t, as midnight there.
func londonDate(t time.Time) time.Time {
	y, m, d := t.In(london).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, london)
}

// retrievedLineFormat is the provenance line at the foot of every NKJV page:
// when this build fetched the text, and from where. Held in this one constant
// so the wording is a one-line change. %s is the date, written as
// <time datetime="2026-10-02">2 October 2026</time>; the link carries no
// tracking parameters.
const retrievedLineFormat = `Text retrieved from <a href="https://api.bible">API.Bible</a> on %s.`

// licenceFoot is the copyright notice and the retrieval line, set in the page
// footer (pageHead.foot) in the footer's own small, muted type
// (nkjv_assets.go). Empty for a page with no licence, which is what keeps every
// other page's footer byte-identical.
func licenceFoot(l *webLicence) string {
	if l == nil {
		return ""
	}
	date := fmt.Sprintf(`<time datetime="%s">%s</time>`,
		l.Retrieved.Format("2006-01-02"), l.Retrieved.Format("2 January 2006"))
	return `<p class="lic">` + template.HTMLEscapeString(l.Notice) + `</p>` +
		`<p class="retrieved">` + fmt.Sprintf(retrievedLineFormat, date) + `</p>`
}

// loadLicensed fetches and checks the text of every licensed translation the
// switch publishes. Off, it returns nothing and touches nothing: no key, no
// request. On, any failure is the build's failure — there is no fallback to
// the notice pages, because a site that quietly published notices while
// switched on would be a different site from the one its switch says.
//
// ref is the reference translation's text (the WEB), which states what a whole
// edition contains (bibletext.ExpectedVerseNumbers). now is read once, when the
// fetch has succeeded, and becomes the date every page states.
func loadLicensed(on, offline bool, key string, ref *bibletext.BibleData, now func() time.Time) ([]loadedVersion, error) {
	if !on {
		return nil, nil
	}
	if offline {
		return nil, errors.New("the NKJV's text is fetched fresh on every build; -offline cannot build it")
	}
	if key == "" {
		return nil, errors.New("no API.Bible key for the NKJV's text")
	}
	var out []loadedVersion
	for _, lv := range licensedVersionsFor(on) {
		start := time.Now()
		ed, err := fetchLicensedEdition(lv.ID, key)
		if err != nil {
			return nil, fmt.Errorf("%s: fetch from API.Bible: %w", lv.ID, err)
		}
		bd := ed.Bible
		if bd == nil {
			return nil, fmt.Errorf("%s: the fetch returned no text", lv.ID)
		}
		if ed.CopyrightLines != 1 {
			return nil, fmt.Errorf("%s: the responses carried %d different copyright lines; every page prints one notice, "+
				"so the registry's LicenseNotice needs checking against the provider before this can publish", lv.ID, ed.CopyrightLines)
		}
		if headingCount(bd) == 0 {
			return nil, fmt.Errorf("%s: decoded no publisher headings; the feed or the decoder has changed", lv.ID)
		}
		if err := checkLicensedComplete(lv.ID, bd, ref); err != nil {
			return nil, err
		}
		notice := bibletext.VersionLicenseNotice(lv.ID)
		if notice == "" {
			return nil, fmt.Errorf("%s: the registry has no licence notice to print with the text", lv.ID)
		}
		retrieved := londonDate(now())
		chapters, verses := 0, 0
		for _, book := range bd.Books {
			chapters += len(bd.Verses[book])
			for _, vs := range bd.Verses[book] {
				verses += len(vs)
			}
		}
		log.Printf("%-5s fetched fresh from API.Bible: %d books, %d chapters, %d verses, %d headings; "+
			"%d requests in %s; retrieved %s", lv.ID, len(bd.Books), chapters, verses, headingCount(bd),
			ed.Requests, time.Since(start).Round(time.Millisecond), retrieved.Format("2 January 2006"))
		out = append(out, loadedVersion{
			webVersion: webVersion{ID: lv.ID, Name: lv.Name, headings: true},
			bible:      bd,
			licence:    &webLicence{Notice: notice, Retrieved: retrieved},
		})
	}
	return out, nil
}

// checkLicensedComplete refuses an edition that is not whole: its books must be
// the reference's canon and every chapter's verse numbers exactly what the
// reference and the measured delta say they are. A short fetch would otherwise
// publish as a chapter that stops early, or a 404 where the app links.
//
// The error names references and numbers, never text, and at most five of
// each, so a fetch that came back wholly wrong cannot flood the log.
func checkLicensedComplete(id string, got, ref *bibletext.BibleData) error {
	want, err := bibletext.ExpectedVerseNumbers(id, ref)
	if err != nil {
		return fmt.Errorf("%s: %w", id, err)
	}
	var problems []string
	note := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	gotBooks := map[string]bool{}
	for _, book := range got.Books {
		gotBooks[book] = true
		if _, ok := want[book]; !ok {
			note("%s is not in the reference canon", book)
		}
	}
	for _, book := range ref.Books {
		if len(ref.Verses[book]) > 0 && (!gotBooks[book] || len(got.Verses[book]) == 0) {
			note("%s is missing", book)
		}
	}
	books := make([]string, 0, len(want))
	for book := range want {
		books = append(books, book)
	}
	sort.Strings(books)
	for _, book := range books {
		if !gotBooks[book] {
			continue
		}
		for ch := range got.Verses[book] {
			if _, ok := want[book][ch]; !ok {
				note("%s %d is not in the reference canon", book, ch)
			}
		}
		chapters := make([]int, 0, len(want[book]))
		for ch := range want[book] {
			chapters = append(chapters, ch)
		}
		sort.Ints(chapters)
		for _, ch := range chapters {
			vs, ok := got.Verses[book][ch]
			if !ok {
				note("%s %d is missing", book, ch)
				continue
			}
			have := map[int]bool{}
			for _, v := range vs {
				have[v.Verse] = true
			}
			expected := map[int]bool{}
			var missing, extra []int
			for _, n := range want[book][ch] {
				expected[n] = true
				if !have[n] {
					missing = append(missing, n)
				}
			}
			for n := range have {
				if !expected[n] {
					extra = append(extra, n)
				}
			}
			sort.Ints(extra)
			if len(missing) > 0 {
				note("%s %d lacks verse %s", book, ch, firstFew(missing))
			}
			if len(extra) > 0 {
				note("%s %d has unexpected verse %s", book, ch, firstFew(extra))
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	more := ""
	if len(problems) > 5 {
		more = fmt.Sprintf(", and %d more", len(problems)-5)
		problems = problems[:5]
	}
	return fmt.Errorf("%s is not the whole edition, so it will not be published: %s%s",
		id, strings.Join(problems, "; "), more)
}

// firstFew lists up to five verse numbers and counts the rest.
func firstFew(nums []int) string {
	shown := make([]string, 0, 5)
	for i, n := range nums {
		if i == 5 {
			return strings.Join(shown, ", ") + fmt.Sprintf(" and %d more", len(nums)-5)
		}
		shown = append(shown, strconv.Itoa(n))
	}
	return strings.Join(shown, ", ")
}
