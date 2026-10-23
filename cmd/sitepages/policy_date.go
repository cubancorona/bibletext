package main

// The privacy policy's effective date, held to its words. The policy says that
// a change is posted with a new effective date, and the date a changed policy
// carries is the day it is published (docs/RELEASING.md, stage 11).
// scripts/publish-site.sh hands this command the live copy and the policy
// rendered by renderSitePages as the site's pages are, before the reader is
// generated. A policy whose words or links differ from the live copy's must
// carry the day of the publish, whatever the live copy's date, so a second
// change on the day the first went live carries that day too. A policy whose
// words and links are the live copy's carries the live copy's date, or the
// day of the publish where it is restated, and no other: a date moved with
// nothing changed would say the policy took effect on a day it did not.
//
// The comparison is of the words a reader reads and where its links lead: the
// body's text, with comments, styles and scripts dropped, tags taken out,
// character references decoded and white space collapsed, and the date set
// apart; and the address of each of the body's links, in order, its
// character references decoded. So rewrapped source lines, a change of markup
// or style, and the template comment the renderer leaves are not a change of
// the policy, and any change of its words, or of where one of its links
// leads, is, however small.

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// policyDateLayout is how the policy writes its effective date, and
	// policyTodayLayout how scripts/publish-site.sh passes the day of the
	// publish.
	policyDateLayout  = "2 January 2006"
	policyTodayLayout = "2006-01-02"
)

var (
	policyBody      = regexp.MustCompile(`(?is)<body\b[^>]*>(.*)</body>`)
	policyHidden    = regexp.MustCompile(`(?is)<!--.*?-->|<script\b.*?</script\s*>|<style\b.*?</style\s*>`)
	policyTag       = regexp.MustCompile(`<[^>]*>`)
	policyLink      = regexp.MustCompile(`(?is)<a\b[^>]*?\shref\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	policyEffective = regexp.MustCompile(`Effective date: ([0-9]{1,2} [A-Z][a-z]+ [0-9]{4})`)
)

// policyShown is page's body with its comments, styles and scripts dropped:
// what a reader is shown, still marked up.
func policyShown(page []byte) string {
	body := string(page)
	if m := policyBody.FindStringSubmatch(body); m != nil {
		body = m[1]
	}
	return policyHidden.ReplaceAllString(body, " ")
}

// policyWords is the text of page's body as a reader reads it, white space
// collapsed to single spaces.
func policyWords(page []byte) string {
	body := policyTag.ReplaceAllString(policyShown(page), "")
	return strings.Join(strings.Fields(html.UnescapeString(body)), " ")
}

// policyLinks is where each link in page's body leads, in order, as the
// browser reads the address: character references decoded, the white space
// around it dropped.
func policyLinks(page []byte) []string {
	var links []string
	for _, m := range policyLink.FindAllStringSubmatch(policyShown(page), -1) {
		links = append(links, strings.TrimSpace(html.UnescapeString(m[1]+m[2]+m[3])))
	}
	return links
}

// policyChange says how built differs from live, by their words with the
// date set apart and by their links, or is empty where neither differs.
func policyChange(builtWords, liveWords string, built, live []byte) string {
	words := builtWords != liveWords
	links := !slices.Equal(policyLinks(built), policyLinks(live))
	switch {
	case words && links:
		return "the privacy policy's words and links differ from the live copy's"
	case words:
		return "the privacy policy's words differ from the live copy's"
	case links:
		return "the privacy policy's links differ from the live copy's"
	}
	return ""
}

// policyText splits a rendered policy into its effective date, as written,
// and its words with the date taken out and its label left in. date is empty
// when the page states none in the policy's form.
func policyText(page []byte) (date, words string) {
	words = policyWords(page)
	m := policyEffective.FindStringSubmatchIndex(words)
	if m == nil {
		return "", words
	}
	return words[m[2]:m[3]], words[:m[2]] + words[m[3]:]
}

// checkPolicyDate holds the policy about to be published (built) to the live
// copy (live, nil where there is none) on the day today, in
// policyTodayLayout. It says what it found, or why the publish must stop.
func checkPolicyDate(live, built []byte, today string) (string, error) {
	day, err := time.Parse(policyTodayLayout, today)
	if err != nil {
		return "", fmt.Errorf("the day of the publish is %q, not a date in the form %s", today, policyTodayLayout)
	}
	date, words := policyText(built)
	if date == "" {
		return "", errors.New("the privacy policy to publish states no effective date in the form \"Effective date: 2 January 2006\"")
	}
	effective, err := time.Parse(policyDateLayout, date)
	if err != nil {
		return "", fmt.Errorf("the privacy policy's effective date %q is not a date: %v", date, err)
	}
	if live == nil {
		return "no live privacy policy to compare with", nil
	}
	liveDate, liveWords := policyText(live)
	liveEffective, err := time.Parse(policyDateLayout, liveDate)
	isLiveDate := err == nil && liveEffective.Equal(effective)
	isToday := effective.Equal(day)
	publishDay := day.Format(policyDateLayout)
	changed := policyChange(words, liveWords, built, live)
	switch {
	case changed == "" && isLiveDate:
		return "the privacy policy's words and links are the live copy's, effective " + date, nil
	case changed == "" && isToday:
		return "the privacy policy's words and links are the live copy's, restated as effective " + date +
			", the day of this publish", nil
	case changed == "":
		return "", fmt.Errorf("the privacy policy's words and links are the live copy's, but its effective date is moved from %s to %s, "+
			"which is neither the live copy's date nor the day of this publish, %s; put back the live copy's date, %s "+
			"(docs/RELEASING.md, stage 11)", liveDate, date, publishDay, liveDate)
	case isToday:
		// Whatever the live copy's date: on the day a change went live, a
		// second change carries that day too.
		return "the privacy policy changed since the live copy and is effective " + date + ", the day of this publish", nil
	case isLiveDate:
		return "", fmt.Errorf("%s, and its effective date is still the live copy's, %s; "+
			"set it to the day of this publish, %s (docs/RELEASING.md, stage 11)", changed, date, publishDay)
	default:
		return "", fmt.Errorf("%s, and its effective date is %s, not the day of this publish, %s "+
			"(docs/RELEASING.md, stage 11)", changed, date, publishDay)
	}
}

// checkPolicyDateInTrees reads privacy.html from the live tree and the tree
// about to be published and holds the second to the first (checkPolicyDate).
// A live tree with no policy is a site that has never carried one.
func checkPolicyDateInTrees(liveDir, builtDir, today string) (string, error) {
	if info, err := os.Stat(liveDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("the live tree %q is not a directory", liveDir)
	}
	built, err := os.ReadFile(filepath.Join(builtDir, "privacy.html"))
	if err != nil {
		return "", fmt.Errorf("read the privacy policy to publish: %w", err)
	}
	live, err := os.ReadFile(filepath.Join(liveDir, "privacy.html"))
	if errors.Is(err, os.ErrNotExist) {
		live = nil
	} else if err != nil {
		return "", fmt.Errorf("read the live privacy policy: %w", err)
	}
	return checkPolicyDate(live, built, today)
}
