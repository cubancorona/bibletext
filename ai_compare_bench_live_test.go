package bibletext

// The live recorder for the Compare perspectives fairness bench: one
// recording round against the real providers, scored as it goes, with every
// answer written out for a person to read before any of it becomes a fixture.
//
// Doubly gated so it can never spend money by accident: it needs both the
// provider keys in the environment and BIBLETEXT_COMPAREBENCH=1, and it
// refuses to start without BIBLETEXT_COMPAREBENCH_DIR, the directory the
// answers are written to (never under testdata; AGENTS.md). From the repo
// root:
//
//	set -a; source ./.env.local; set +a
//	BIBLETEXT_COMPAREBENCH=1 BIBLETEXT_COMPAREBENCH_DIR=/tmp/compare-round-1 \
//	  go test -run '^TestCompareBenchLive$' -v -timeout 3600s .
//
// One round is ten Compare calls on each provider with a key, in parallel
// across providers and one passage at a time within one, plus ten Explain
// calls on the default provider: about fifty calls on the capable default
// models, of the order of $5 to $10 and fifteen to thirty minutes. Each call
// has a budget of two minutes; a call that runs out is logged as a timeout,
// counted as neither a refusal nor a pass, and fails the row on the default
// provider. A provider's refusal is logged, not failed.
//
// The answers land at <dir>/<provider>/<passage>.md, in the fixture format
// the offline test reads (front matter, then the answer), and the Explain
// answers at <dir>/<provider>/explain/<passage>.md. After a person has read
// a provider's set, copying <dir>/<provider>/ to testdata/compare/<provider>/
// is what turns TestCompareBenchRecordedAnswersAreFair green.
//
// The round fails when the default provider's answer fails the scorer, is
// declined or times out on any passage, when a provider with a complete set
// leaves a roster reading out of every answer, or when the Explain control
// shows the shared prompt sentence has lengthened Explain past a few short
// paragraphs (more than two of the ten over the length below).

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	// compareBenchCallBudget is the ceiling on one call.
	compareBenchCallBudget = 120 * time.Second
	// An Explain answer is a few short paragraphs when it is at most this
	// many words and paragraphs, with no heading or bullet line.
	compareBenchExplainWords      = 350
	compareBenchExplainParagraphs = 5
	compareBenchExplainTolerance  = 2
)

// compareBenchSwitchOn skips the test unless the bench switch is set: a key
// in the shell is as often there for the app as for a paid round.
func compareBenchSwitchOn(t *testing.T) {
	t.Helper()
	if os.Getenv(compareBenchSwitch) != "1" {
		t.Skipf("set %s=1 (and source .env.local) to run the paid recording round", compareBenchSwitch)
	}
}

// compareBenchRun is what a round needs: where to write, and the providers
// that have a key, the default provider first.
type compareBenchRun struct {
	dir       string
	providers []providerInfo
}

// compareBenchSettings refuses a round with nowhere to write or without the
// default provider's key, since the round is judged on that provider; the
// other providers are included as their keys allow.
func compareBenchSettings(dir string, keyFor func(providerInfo) string) (compareBenchRun, error) {
	run := compareBenchRun{dir: strings.TrimSpace(dir)}
	if run.dir == "" {
		return run, fmt.Errorf("%s is unset: it names the directory the answers are written to", compareBenchOutDir)
	}
	for _, p := range aiProviders() {
		if keyFor(p) != "" {
			run.providers = append(run.providers, p)
		}
	}
	sort.SliceStable(run.providers, func(i, j int) bool {
		return run.providers[i].ID == defaultProviderID && run.providers[j].ID != defaultProviderID
	})
	if len(run.providers) == 0 || run.providers[0].ID != defaultProviderID {
		return run, fmt.Errorf("no key for the default provider in %s: the round is judged on it", envVarFor(defaultProviderID))
	}
	return run, nil
}

// benchRow is one call's record.
type benchRow struct {
	provider, action, id, model string
	secs                        float64
	words                       int
	readings                    []string
	outcome                     string // ok, declined, timeout, error
	detail                      string
	problems                    []problem
	path                        string
}

// benchCall makes one call under the budget and classifies its end.
func benchCall(client aiClient, prompt string) (answer string, secs float64, outcome, detail string) {
	ctx, cancel := context.WithTimeout(context.Background(), compareBenchCallBudget)
	defer cancel()
	start := time.Now()
	out, err := client.generate(ctx, prompt)
	secs = time.Since(start).Seconds()
	var declined *aiDeclinedError
	switch {
	case err == nil:
		return out, secs, "ok", ""
	case errors.As(err, &declined):
		return "", secs, "declined", err.Error()
	case errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil:
		return "", secs, "timeout", err.Error()
	}
	return "", secs, "error", err.Error()
}

// writeBenchAnswer records one answer at <dir>/<provider>/<passage>.md, or
// under explain/ for the Explain control, in the fixture format.
func writeBenchAnswer(dir, provider, action string, c benchCase, model, prompt, body string, at time.Time) (string, error) {
	sub := filepath.Join(dir, provider)
	if action != aiActionCompare {
		sub = filepath.Join(sub, action)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return "", err
	}
	sum := promptFingerprint(prompt)
	path := filepath.Join(sub, c.ID+".md")
	fields := map[string]string{"passage": c.ID, "ref": c.Ref, "version": compareBenchVersion, "action": action,
		"provider": provider, "model": model, "recorded": at.UTC().Format(time.RFC3339), "prompt_sha1": sum}
	return path, os.WriteFile(path, []byte(formatBenchFixture(fields, body)), 0o644)
}

// explainShortProse reports why an Explain answer is not a few short
// paragraphs of prose, or "" when it is.
func explainShortProse(md string) string {
	if n := words(md); n > compareBenchExplainWords {
		return fmt.Sprintf("%d words, want at most %d", n, compareBenchExplainWords)
	}
	paragraphs := 0
	for _, chunk := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(chunk) != "" {
			paragraphs++
		}
	}
	if paragraphs > compareBenchExplainParagraphs {
		return fmt.Sprintf("%d paragraphs, want at most %d", paragraphs, compareBenchExplainParagraphs)
	}
	for _, line := range strings.Split(md, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "* ") || strings.HasPrefix(s, "• ") {
			return fmt.Sprintf("a heading or bullet line: %q", s)
		}
	}
	return ""
}

func TestCompareBenchLive(t *testing.T) {
	compareBenchSwitchOn(t)
	for _, p := range aiProviders() {
		useLiveEnv(t, envVarFor(p.ID)) // the key the suite found; TestMain withholds it
	}
	run, err := compareBenchSettings(os.Getenv(compareBenchOutDir), func(p providerInfo) string {
		return providerAPIKey(newKeyStoreWith(newFakePrefs()), p.ID)
	})
	if err != nil {
		t.Fatal(err)
	}
	tb := loadBenchTable(t)
	bd := benchBible(t)
	if err := os.MkdirAll(run.dir, 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var rows []benchRow
	record := func(r benchRow) {
		mu.Lock()
		rows = append(rows, r)
		mu.Unlock()
	}
	var wg sync.WaitGroup
	for _, p := range run.providers {
		p := p
		wg.Add(1)
		go func() { // providers in parallel, passages within a provider one at a time
			defer wg.Done()
			// A store of this provider's own: the resolver caches the model it
			// settles on in the store, and the stores are not shared between
			// goroutines.
			store := newKeyStoreWith(newFakePrefs())
			client := p.New(store, providerAPIKey(store, p.ID))
			actions := []string{aiActionCompare}
			if p.ID == defaultProviderID {
				actions = append(actions, aiActionExplain)
			}
			for _, action := range actions {
				for _, c := range tb.Passages {
					prompt, err := aiStudyPrompt(action, c.passage(), "")
					if err != nil {
						t.Error(err)
						return
					}
					r := benchRow{provider: p.ID, action: action, id: c.ID}
					var answer string
					answer, r.secs, r.outcome, r.detail = benchCall(client, prompt)
					r.model = activeModelFor(store, p.ID)
					if r.outcome == "ok" {
						r.words = answerWords(answer)
						if action == aiActionCompare {
							r.readings = readingTitles(answer)
							r.problems = scoreCompareAnswer(bd, c, answer)
						} else if why := explainShortProse(answer); why != "" {
							r.problems = []problem{{check: 0, detail: why}}
						}
						path, err := writeBenchAnswer(run.dir, p.ID, action, c, r.model, prompt, answer, time.Now())
						if err != nil {
							t.Errorf("%s/%s: writing the answer: %v", p.ID, c.ID, err)
						}
						r.path = path
					}
					record(r)
				}
			}
		}()
	}
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].provider != rows[j].provider {
			return rows[i].provider < rows[j].provider
		}
		if rows[i].action != rows[j].action {
			return rows[i].action < rows[j].action
		}
		return rows[i].id < rows[j].id
	})
	t.Logf("%-10s %-8s %-18s %6s %5s  %s", "provider", "action", "passage", "secs", "words", "readings / outcome / problems")
	for _, r := range rows {
		line := r.outcome
		if r.outcome == "ok" {
			line = strings.Join(r.readings, "; ")
		} else {
			line += ": " + r.detail
		}
		for _, p := range r.problems {
			line += " | " + p.String()
		}
		t.Logf("%-10s %-8s %-18s %6.1f %5d  %s", r.provider, r.action, r.id, r.secs, r.words, line)
	}

	answers := map[string]map[string]string{}
	for _, p := range run.providers {
		answers[p.ID] = map[string]string{}
	}
	longExplains := 0
	var secsByProvider = map[string][]float64{}
	for _, r := range rows {
		if r.action == aiActionExplain {
			if len(r.problems) > 0 || r.outcome != "ok" {
				longExplains++
			}
			continue
		}
		secsByProvider[r.provider] = append(secsByProvider[r.provider], r.secs)
		if r.outcome == "ok" {
			b, err := os.ReadFile(r.path)
			if err == nil {
				if f, err := parseBenchFixture(string(b)); err == nil {
					answers[r.provider][r.id] = f.body
				}
			}
		}
		if r.provider != defaultProviderID {
			continue
		}
		if r.outcome != "ok" {
			t.Errorf("%s/%s: %s (%s)", r.provider, r.id, r.outcome, r.detail)
		}
		for _, p := range r.problems {
			t.Errorf("%s/%s: %s", r.provider, r.id, p)
		}
	}
	for _, p := range compareOmissions(tb, answers) {
		if strings.HasPrefix(p.detail, defaultProviderID+" ") {
			t.Error(p)
		} else {
			t.Log(p)
		}
	}
	if longExplains > compareBenchExplainTolerance {
		t.Errorf("%d of %d Explain answers on %s are not a few short paragraphs; the shared prompt sentence has lengthened Explain", longExplains, len(tb.Passages), defaultProviderID)
	}
	for _, p := range run.providers {
		secs := secsByProvider[p.ID]
		sort.Float64s(secs)
		if len(secs) > 0 {
			t.Logf("%s: median %.1f s over %d calls; answers under %s", p.ID, secs[len(secs)/2], len(secs), filepath.Join(run.dir, p.ID))
		}
	}
}

// --- Controls, run on every plain run -------------------------------------------

// A key alone does not run the round: without the switch the live test
// skips before it reaches a key. Control: with the switch set it goes on.
func TestCompareBenchLiveWantsItsSwitch(t *testing.T) {
	reached := func(switched string) bool {
		called := false
		t.Setenv(compareBenchSwitch, switched)
		t.Run(compareBenchSwitch+"="+switched, func(t *testing.T) { compareBenchSwitchOn(t); called = true })
		return called
	}
	if reached("") {
		t.Error("without the switch the paid round would have run")
	}
	if !reached("1") {
		t.Error("control: with the switch set the round still skipped, so the skip above proves nothing")
	}
}

// A round needs somewhere to write and the default provider's key, and is
// refused without either; with them it lists every provider that has a key,
// the default first, and leaves out those without.
func TestCompareBenchSettings(t *testing.T) {
	all := func(providerInfo) string { return "dummy-not-a-key" }
	if _, err := compareBenchSettings("", all); err == nil || !strings.Contains(err.Error(), compareBenchOutDir) {
		t.Errorf("no directory: got %v", err)
	}
	noDefault := func(p providerInfo) string {
		if p.ID == defaultProviderID {
			return ""
		}
		return "dummy-not-a-key"
	}
	if _, err := compareBenchSettings(t.TempDir(), noDefault); err == nil || !strings.Contains(err.Error(), envVarFor(defaultProviderID)) {
		t.Errorf("no default key: got %v", err)
	}
	run, err := compareBenchSettings(t.TempDir(), all)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.providers) != len(aiProviders()) || run.providers[0].ID != defaultProviderID {
		t.Errorf("with every key the round lists %d providers, first %q", len(run.providers), run.providers[0].ID)
	}
	onlyDefault := func(p providerInfo) string {
		if p.ID == defaultProviderID {
			return "dummy-not-a-key"
		}
		return ""
	}
	if run, err := compareBenchSettings(t.TempDir(), onlyDefault); err != nil || len(run.providers) != 1 {
		t.Errorf("with the default key alone: %d providers, %v", len(run.providers), err)
	}
}

// What the recorder writes is what the offline loader reads, in the layout
// testdata/compare expects, so a reviewed directory is copied in as it is:
// the comparison at the top level with its fingerprint, the Explain control
// under explain/ where the loader does not look.
func TestCompareBenchRecorderWritesWhereTheLoaderReads(t *testing.T) {
	tb := loadBenchTable(t)
	c := tb.byID()["matthew-16-18"]
	dir := t.TempDir()
	body := readCompareControl(t, "good.md")
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	prompt := buildComparePrompt(c.passage())
	path, err := writeBenchAnswer(dir, defaultProviderID, aiActionCompare, c, "a-model", prompt, body, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, defaultProviderID, c.ID+".md"); path != want {
		t.Errorf("the comparison landed at %s, want %s", path, want)
	}
	if _, err := writeBenchAnswer(dir, defaultProviderID, aiActionExplain, c, "a-model", "an explain prompt", "An explanation.", at); err != nil {
		t.Fatal(err)
	}
	got := loadBenchFixturesFrom(t, dir)
	f, ok := got[defaultProviderID][c.ID]
	if !ok || len(got[defaultProviderID]) != 1 {
		t.Fatalf("the loader found %v, want the one comparison", got)
	}
	if f.body != body || f.fields["prompt_sha1"] != promptSHA1(c) || f.fields["action"] != aiActionCompare ||
		f.fields["model"] != "a-model" || f.fields["recorded"] != "2026-10-07T12:00:00Z" || f.fields["ref"] != c.Ref {
		t.Errorf("the fixture read back as %v", f.fields)
	}
	if ps := scoreCompareAnswer(NewBibleData(), c, f.body); len(ps) != 0 {
		t.Errorf("the recorded control does not pass the scorer: %s", problemLines(ps))
	}
}

// The Explain control's rule: a few short paragraphs of prose pass; more
// words, more paragraphs, a heading or a bullet do not.
func TestExplainShortProse(t *testing.T) {
	short := "The passage opens the chapter.\n\nIt turns on one word.\n\nThe parts connect through it."
	if why := explainShortProse(short); why != "" {
		t.Errorf("three short paragraphs: %s", why)
	}
	long := strings.Repeat("word ", compareBenchExplainWords+1)
	if why := explainShortProse(long); !strings.Contains(why, "words") {
		t.Errorf("control: %d words passed (%q)", compareBenchExplainWords+1, why)
	}
	many := strings.Repeat("A paragraph.\n\n", compareBenchExplainParagraphs+1)
	if why := explainShortProse(many); !strings.Contains(why, "paragraphs") {
		t.Errorf("control: %d paragraphs passed (%q)", compareBenchExplainParagraphs+1, why)
	}
	for _, marked := range []string{"## Meaning\n\nProse.", "Prose.\n\n- a point\n- another"} {
		if why := explainShortProse(marked); !strings.Contains(why, "heading or bullet") {
			t.Errorf("control: %q passed (%q)", marked, why)
		}
	}
}
