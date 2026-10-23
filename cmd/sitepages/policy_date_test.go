package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	bibletext "github.com/cubancorona/bibletext"
)

// policyPage is a synthetic policy in the real page's shape: a style block,
// the renderer's template comment, the date line and two paragraphs.
func policyPage(date, words string) []byte {
	return []byte(`<!DOCTYPE html>
<html lang="en">
<head>
<title>Example policy</title>
<style>
  body { color: #111; }
</style>
</head>
<body>
  <h1>Example — Privacy Policy</h1>
  <p class="eff">Effective date: ` + date + `</p>

  <h2>Section</h2>
  <!-- Rendered from a template. -->
  <p>` + words + `</p>
  <p>Questions go to <a href="mailto:support@example.invalid">support@example.invalid</a>.</p>
</body>
</html>
`)
}

const policyToday = "2026-10-29" // 29 October 2026

func TestThePolicyDateIsHeldToItsWords(t *testing.T) {
	live := policyPage("11 October 2026", "The app keeps &amp; sends nothing.")
	// A changed policy published on the day of this run, and its mail link
	// pointed elsewhere or written with a query.
	liveToday := policyPage("29 October 2026", "The app keeps &amp; sends nothing else.")
	relink := func(page []byte, from, to string) []byte {
		return []byte(strings.Replace(string(page), from, to, 1))
	}
	const mailLink = `<a href="mailto:support@example.invalid">`
	withQuery := relink(live, mailLink, `<a href="mailto:support@example.invalid?subject=Q&amp;A">`)
	for _, c := range []struct {
		name  string
		live  []byte // the default live copy when nil
		built []byte
		ok    bool
	}{
		{"the same words and date", nil, live, true},
		{"the words rewrapped and the markup changed", nil,
			[]byte(strings.Replace(strings.Replace(string(live), "<p>The app keeps &amp; sends nothing.</p>",
				"<p>The app\n    keeps <strong>&amp;</strong>\tsends   nothing.</p>", 1),
				"color: #111", "color: #222", 1)), true},
		{"the template comment gone", nil, []byte(strings.Replace(string(live), "<!-- Rendered from a template. -->", "", 1)), true},
		{"one word changed, the live copy's date", nil, policyPage("11 October 2026", "The app keeps &amp; sends nothing else."), false},
		{"one word changed, a date that is not the publish day", nil, policyPage("28 October 2026", "The app keeps &amp; sends nothing else."), false},
		{"one word changed, the publish day", nil, policyPage("29 October 2026", "The app keeps &amp; sends nothing else."), true},
		{"a sentence added, the publish day", nil, policyPage("29 October 2026", "The app keeps &amp; sends nothing. It asks first."), true},
		{"a sentence added, the live copy's date", nil, policyPage("11 October 2026", "The app keeps &amp; sends nothing. It asks first."), false},
		{"the date line taken out", nil, []byte(strings.Replace(string(live), `<p class="eff">Effective date: 11 October 2026</p>`, "", 1)), false},
		{"a date that is not one", nil, policyPage("31 February 2026", "The app keeps &amp; sends nothing else."), false},
		// The live copy's date is the day of this run: a second change that
		// day carries that day too.
		{"a second change on the day the first went live, dated that day", liveToday,
			policyPage("29 October 2026", "The app keeps &amp; sends nothing else at all."), true},
		{"a second change on the day the first went live, dated the day before", liveToday,
			policyPage("28 October 2026", "The app keeps &amp; sends nothing else at all."), false},
		{"the policy published again on the day it went live", liveToday, liveToday, true},
		// The words kept: the date is the live copy's, or the day of this run
		// for a policy restated, and no other day.
		{"the words kept, the date moved to the publish day", nil, policyPage("29 October 2026", "The app keeps &amp; sends nothing."), true},
		{"the words kept, the date moved back to a day long past", nil, policyPage("1 January 2020", "The app keeps &amp; sends nothing."), false},
		{"the words kept, the date moved to a day that is neither", nil, policyPage("28 October 2026", "The app keeps &amp; sends nothing."), false},
		{"the words kept, the date moved past the publish day", nil, policyPage("30 October 2026", "The app keeps &amp; sends nothing."), false},
		// Where a link leads is part of the policy; how its tag is written is
		// not.
		{"a link's address changed, the live copy's date", nil,
			relink(live, mailLink, `<a href="mailto:help@example.invalid">`), false},
		{"a link's address changed, the publish day", nil,
			relink(policyPage("29 October 2026", "The app keeps &amp; sends nothing."), mailLink, `<a href="mailto:help@example.invalid">`), true},
		{"a link's tag written otherwise, the address kept", nil,
			relink(live, mailLink, "<a class=\"mail\"\n   href='mailto:support@example.invalid'>"), true},
		{"a link's address written with a character reference decoded", withQuery,
			relink(live, mailLink, `<a href="mailto:support@example.invalid?subject=Q&A">`), true},
		{"a link's query changed, the live copy's date", withQuery,
			relink(live, mailLink, `<a href="mailto:support@example.invalid?subject=Q">`), false},
		{"a link inside a comment changed", nil,
			relink(live, "<!-- Rendered from a template. -->", `<!-- <a href="https://example.invalid/old">old</a> -->`), true},
	} {
		from := live
		if c.live != nil {
			from = c.live
		}
		said, err := checkPolicyDate(from, c.built, policyToday)
		if c.ok && err != nil {
			t.Errorf("%s: refused: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: accepted (%s)", c.name, said)
		}
	}
}

// A refusal names the date the policy carries, the day of the publish, and
// the release step that says what to do.
func TestThePolicyDateRefusalSaysWhatToSet(t *testing.T) {
	live := policyPage("11 October 2026", "Words.")
	_, err := checkPolicyDate(live, policyPage("11 October 2026", "Other words."), policyToday)
	if err == nil {
		t.Fatal("a changed policy with the live copy's date was accepted")
	}
	for _, want := range []string{"11 October 2026", "29 October 2026", "docs/RELEASING.md, stage 11"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
	// The same day written another way is the same date.
	if _, err := checkPolicyDate(policyPage("1 October 2026", "Words."),
		policyPage("01 October 2026", "Other words."), policyToday); err == nil ||
		!strings.Contains(err.Error(), "still the live copy's") {
		t.Errorf("the live copy's date written with a leading zero was not taken for the live copy's: %v", err)
	}
	// A date moved with the words kept names the live copy's date to put back.
	_, err = checkPolicyDate(live, policyPage("1 January 2020", "Words."), policyToday)
	if err == nil {
		t.Fatal("the live copy's words with their date moved back were accepted")
	}
	for _, want := range []string{"1 January 2020", "11 October 2026", "29 October 2026", "docs/RELEASING.md, stage 11"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal of a moved date does not name %q: %v", want, err)
		}
	}
	// A link that leads elsewhere is said to be one.
	_, err = checkPolicyDate(live, []byte(strings.Replace(string(live), "mailto:support@example.invalid\"",
		"mailto:help@example.invalid\"", 1)), policyToday)
	if err == nil || !strings.Contains(err.Error(), "links differ from the live copy's") {
		t.Errorf("a link that leads elsewhere was not refused as one: %v", err)
	}
}

// With no live copy there is nothing to compare, but the policy must still
// state a date; and the day of the publish must be a date.
func TestThePolicyDateWithNoLiveCopyAndAMalformedDay(t *testing.T) {
	if _, err := checkPolicyDate(nil, policyPage("29 October 2026", "Words."), policyToday); err != nil {
		t.Errorf("a first publish was refused: %v", err)
	}
	if _, err := checkPolicyDate(nil, []byte("<body><p>No date.</p></body>"), policyToday); err == nil {
		t.Error("a policy with no effective date was accepted")
	}
	for _, day := range []string{"", "29 October 2026", "2026-10-29T00:00"} {
		if _, err := checkPolicyDate(nil, policyPage("29 October 2026", "Words."), day); err == nil {
			t.Errorf("the day %q was accepted", day)
		}
	}
}

// The trees publish-site.sh hands over: a live tree with no policy is a first
// publish, a live tree that is not there refuses.
func TestThePolicyDateReadsBothTrees(t *testing.T) {
	live, built := t.TempDir(), t.TempDir()
	write := func(dir string, page []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "privacy.html"), page, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(built, policyPage("11 October 2026", "New words."))
	if _, err := checkPolicyDateInTrees(live, built, policyToday); err != nil {
		t.Errorf("a live tree with no policy refused: %v", err)
	}
	write(live, policyPage("11 October 2026", "Old words."))
	if _, err := checkPolicyDateInTrees(live, built, policyToday); err == nil {
		t.Error("a changed policy with the live copy's date was accepted from the trees")
	}
	write(built, policyPage("29 October 2026", "New words."))
	if _, err := checkPolicyDateInTrees(live, built, policyToday); err != nil {
		t.Errorf("a changed policy dated the publish day refused: %v", err)
	}
	if _, err := checkPolicyDateInTrees(filepath.Join(live, "missing"), built, policyToday); err == nil {
		t.Error("a live tree that is not there was accepted")
	}
}

// The real policy, rendered as the publish renders it, states its date in the
// form the check reads, so the check is never blind to it; and its words with
// the date changed are its words.
func TestTheRealPolicyStatesADateTheCheckReads(t *testing.T) {
	out := t.TempDir()
	if err := renderSitePages(filepath.Join("..", "..", "docs"), out,
		bibletext.SupportEmail(), bibletext.SupportMailtoRecipient(), "off"); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(out, "privacy.html"))
	if err != nil {
		t.Fatal(err)
	}
	date, words := policyText(page)
	if date == "" {
		t.Fatal("docs/privacy.html states no effective date the check can read")
	}
	if !strings.Contains(words, "Effective date:") || strings.Contains(words, date) {
		t.Errorf("the date was not set apart from the words: %q", date)
	}
	moved := []byte(strings.Replace(string(page), "Effective date: "+date, "Effective date: 29 October 2026", 1))
	if _, w := policyText(moved); w != words {
		t.Error("changing the date alone changed the policy's words")
	}
	if _, err := checkPolicyDate(page, moved, policyToday); err != nil {
		t.Errorf("the real policy with only its date moved to the publish day was refused: %v", err)
	}
	if _, err := checkPolicyDate(page, []byte(strings.Replace(string(page), "does not track", "does not ever track", 1)), policyToday); err == nil {
		t.Error("the real policy with a word changed and its date kept was accepted")
	}
}

// scripts/publish-site.sh holds the policy it renders to the live copy, with
// London's date, before the reader is generated, so a publish it refuses
// spends no API.Bible quota: a publish stops on a refusal, and when the live
// copy cannot be fetched; a dry run goes on and ends refused. Mutation: the
// call dropped or moved after the generation, a refusal let through a publish
// or a dry run, or a publish let go on without the live copy.
func TestThePublishHoldsThePolicyDateToTheLiveCopy(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scripts", "publish-site.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(b)
	at := func(what, pattern string) int {
		t.Helper()
		loc := regexp.MustCompile(pattern).FindStringIndex(script)
		if loc == nil {
			t.Fatalf("scripts/publish-site.sh does not %s", what)
		}
		return loc[0]
	}
	state := at("ask the generator which NKJV state it builds", `NKJV_TEXT=\$\(nkjv_text_state build/websitegen\)`)
	live := at("copy the live policy", `git show origin/gh-pages:privacy\.html > "\$LIVE_TREE/live/privacy\.html"`)
	render := at("render the policy for the check as the site's pages are rendered",
		`go run \./cmd/sitepages -source docs -out "\$LIVE_TREE/built" -nkjv-text "\$NKJV_TEXT"`)
	check := at("hold the policy's date to the live copy with London's date, a publish stopping on a refusal",
		`if ! go run \./cmd/sitepages -check-policy-date -live "\$LIVE_TREE/live" -out "\$LIVE_TREE/built" \\\n\s+`+
			`-today "\$\(TZ=Europe/London date \+%F\)"; then\n\s+\$DRY_RUN \|\|\n\s+fail "[^"]+"\n\s+POLICY_REFUSED=yes\n`)
	generate := at("generate the reader", `run_with_site_bible_key build/websitegen -out "\$OUT"`)
	dryRunEnd := at("end a refused dry run refused",
		`if \$DRY_RUN; then\n(?:\s*#.*\n)*\s*\[\[ -z "\$POLICY_REFUSED" \]\] \|\|\n\s+fail "[^"]+"\n\s+echo "==> --dry-run: built and verified`)
	push := at("push", `git -C "\$WORKTREE" push`)
	if !(state < live && live < render && render < check && check < generate && generate < dryRunEnd && dryRunEnd < push) {
		t.Error("the policy-date check does not come after the NKJV state and before the reader's generation, " +
			"with the refused dry run ending before the push")
	}
	if !strings.Contains(script, `$DRY_RUN || fail "origin/gh-pages could not be fetched, so the privacy policy cannot be held to the live copy"`) {
		t.Error("a publish goes ahead when the live copy could not be fetched, with the policy's date unchecked")
	}
}
