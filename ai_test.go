package bibletext

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockHTTP implements httpClient, capturing the request and returning canned data.
type mockHTTP struct {
	lastReq    *http.Request
	statusCode int
	body       string
	err        error
	calls      int
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

// fakePrefs is an in-memory prefStore for keyStore tests.
type fakePrefs struct{ m map[string]string }

func newFakePrefs() *fakePrefs                { return &fakePrefs{m: map[string]string{}} }
func (f *fakePrefs) String(key string) string { return f.m[key] }
func (f *fakePrefs) StringWithFallback(key, fb string) string {
	if v, ok := f.m[key]; ok {
		return v
	}
	return fb
}
func (f *fakePrefs) SetString(key, value string) { f.m[key] = value }

type fakeSecrets struct {
	m         map[string]string
	failWrite bool
	failRead  bool // simulate a transient credential-store error
}

func (f *fakeSecrets) Name() string { return "test store" }

func (f *fakeSecrets) Read(account string) (string, bool, bool) {
	if f.failRead {
		return "", false, false
	}
	v, ok := f.m[account]
	return v, ok, true
}

func (f *fakeSecrets) Write(account, value string) bool {
	if f.failWrite {
		return false
	}
	if value == "" {
		delete(f.m, account)
	} else {
		f.m[account] = value
	}
	return true
}

// --- Gemini -----------------------------------------------------------------

func geminiTestClient(m *mockHTTP) *geminiClient {
	return &geminiClient{apiKey: "test-key", model: "gemini-test", baseURL: "https://example.test/v1", http: m}
}

func TestGeminiGenerateSendsKeyURLAndParses(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"candidates":[{"content":{"parts":[{"text":"Hello "},{"text":"world."}]}}]}`}
	out, err := geminiTestClient(m).generate(context.Background(), "prompt")
	if err != nil || out != "Hello world." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("x-goog-api-key") != "test-key" {
		t.Errorf("missing api key header")
	}
	if m.lastReq.URL.String() != "https://example.test/v1/models/gemini-test:generateContent" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestGeminiRateLimited(t *testing.T) {
	m := &mockHTTP{statusCode: http.StatusTooManyRequests, body: `{"error":{"message":"quota"}}`}
	_, err := geminiTestClient(m).generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 {
		t.Fatalf("want 429 apiHTTPError, got %v", err)
	}
	if !strings.Contains(friendlyAIError(err), "busy") {
		t.Errorf("friendly = %q", friendlyAIError(err))
	}
}

func TestDoAIRequestRetriesOn5xx(t *testing.T) {
	old := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { aiRetrySleep = old })

	m := &mockHTTP{statusCode: http.StatusServiceUnavailable, body: `{"error":{"message":"overloaded"}}`}
	c := &geminiClient{apiKey: "k", model: "m", baseURL: "https://x.test/v1", http: m}
	_, err := c.generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("want 503 apiHTTPError, got %v", err)
	}
	if m.calls != 3 {
		t.Errorf("expected 3 attempts on 5xx, got %d", m.calls)
	}
}

func TestDoAIRequestNoRetryOn4xx(t *testing.T) {
	old := aiRetrySleep
	aiRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { aiRetrySleep = old })

	m := &mockHTTP{statusCode: http.StatusBadRequest, body: `{"error":{"message":"bad"}}`}
	c := &geminiClient{apiKey: "k", model: "m", baseURL: "https://x.test/v1", http: m}
	_, err := c.generate(context.Background(), "p")
	var apiErr *apiHTTPError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("want 400 apiHTTPError, got %v", err)
	}
	if m.calls != 1 {
		t.Errorf("expected 1 attempt on 4xx, got %d", m.calls)
	}
}

func TestGenerateEmptyKey(t *testing.T) {
	c := geminiTestClient(&mockHTTP{})
	c.apiKey = ""
	if _, err := c.generate(context.Background(), "p"); !errors.Is(err, errNoAPIKey) {
		t.Fatalf("want errNoAPIKey, got %v", err)
	}
}

func TestParseGeminiText(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{"ok", `{"candidates":[{"content":{"parts":[{"text":"Answer."}]}}]}`, "Answer.", false},
		{"no candidates", `{"candidates":[]}`, "", true},
		{"blocked", `{"promptFeedback":{"blockReason":"SAFETY"}}`, "", true},
		{"garbage", `nope`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGeminiText([]byte(tc.body))
			if tc.wantErr != (err != nil) || (!tc.wantErr && got != tc.want) {
				t.Fatalf("got (%q, %v)", got, err)
			}
		})
	}
}

// --- OpenAI / Anthropic adapters --------------------------------------------

func TestOpenAIGenerate(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"choices":[{"message":{"content":"Hi."}}]}`}
	c := &openAIClient{apiKey: "k", model: "gpt", baseURL: "https://x.test/v1", http: m}
	out, err := c.generate(context.Background(), "p")
	if err != nil || out != "Hi." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("Authorization") != "Bearer k" {
		t.Errorf("auth header = %q", m.lastReq.Header.Get("Authorization"))
	}
	if m.lastReq.URL.String() != "https://x.test/v1/chat/completions" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestAnthropicGenerate(t *testing.T) {
	m := &mockHTTP{statusCode: 200, body: `{"content":[{"type":"text","text":"Hello."}]}`}
	c := &anthropicClient{apiKey: "k", model: "claude", baseURL: "https://a.test", http: m}
	out, err := c.generate(context.Background(), "p")
	if err != nil || out != "Hello." {
		t.Fatalf("got (%q, %v)", out, err)
	}
	if m.lastReq.Header.Get("x-api-key") != "k" {
		t.Errorf("x-api-key = %q", m.lastReq.Header.Get("x-api-key"))
	}
	if m.lastReq.Header.Get("anthropic-version") == "" {
		t.Errorf("missing anthropic-version header")
	}
	if m.lastReq.URL.String() != "https://a.test/v1/messages" {
		t.Errorf("url = %q", m.lastReq.URL.String())
	}
}

func TestParseAdapterText(t *testing.T) {
	if v, _ := parseOpenAIText([]byte(`{"choices":[{"message":{"content":"A"}}]}`)); v != "A" {
		t.Errorf("openai parse = %q", v)
	}
	if _, err := parseOpenAIText([]byte(`{"choices":[]}`)); err == nil {
		t.Errorf("openai empty should error")
	}
	if v, _ := parseAnthropicText([]byte(`{"content":[{"type":"text","text":"B"}]}`)); v != "B" {
		t.Errorf("anthropic parse = %q", v)
	}
	if _, err := parseAnthropicText([]byte(`{"content":[]}`)); err == nil {
		t.Errorf("anthropic empty should error")
	}
}

func TestProviderRegistry(t *testing.T) {
	for _, id := range []string{providerGemini, providerOpenAI, providerAnthropic, providerGrok} {
		p, ok := providerByID(id)
		if !ok || p.New == nil || p.Model == "" || p.KeyURL == "" || p.Name == "" {
			t.Errorf("provider %q incomplete: %+v ok=%v", id, p, ok)
		}
	}
	if _, ok := providerByID("nope"); ok {
		t.Errorf("unknown provider should not resolve")
	}
}

// --- Key store + resolution -------------------------------------------------

func TestKeyStore(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	if ks.activeProvider() != defaultProviderID {
		t.Errorf("default active = %q", ks.activeProvider())
	}
	ks.setActiveProvider(providerAnthropic)
	if ks.activeProvider() != providerAnthropic {
		t.Errorf("active = %q", ks.activeProvider())
	}
	ks.setActiveProvider("bogus") // ignored
	if ks.activeProvider() != providerAnthropic {
		t.Errorf("bogus provider should be ignored, got %q", ks.activeProvider())
	}
	ks.setAPIKey(providerOpenAI, "  sk-123  ")
	if ks.apiKey(providerOpenAI) != "sk-123" {
		t.Errorf("key not trimmed/stored: %q", ks.apiKey(providerOpenAI))
	}
}

func TestKeyStoreMigratesLegacyKeyToSecureStore(t *testing.T) {
	prefs := newFakePrefs()
	prefs.SetString(prefKeyPrefix+providerOpenAI, "legacy-key")
	secrets := &fakeSecrets{m: map[string]string{}}
	ks := &keyStore{prefs: prefs, secrets: secrets}

	if got := ks.apiKey(providerOpenAI); got != "legacy-key" {
		t.Fatalf("migrated key = %q, want legacy-key", got)
	}
	if got := secrets.m[providerOpenAI]; got != "legacy-key" {
		t.Fatalf("secure copy = %q, want legacy-key", got)
	}
	if got := prefs.String(prefKeyPrefix + providerOpenAI); got != "" {
		t.Fatalf("legacy preference was not erased: %q", got)
	}
}

func TestKeyStoreWritesAndClearsSecurely(t *testing.T) {
	prefs := newFakePrefs()
	secrets := &fakeSecrets{m: map[string]string{}}
	ks := &keyStore{prefs: prefs, secrets: secrets}

	if !ks.setAPIKey(providerGemini, "  secure-key  ") {
		t.Fatal("secure write failed")
	}
	if got := ks.apiKey(providerGemini); got != "secure-key" {
		t.Fatalf("secure key = %q, want secure-key", got)
	}
	if !ks.setAPIKey(providerGemini, "") {
		t.Fatal("secure delete failed")
	}
	if got := ks.apiKey(providerGemini); got != "" {
		t.Fatalf("key survived clear: %q", got)
	}
}

func TestKeyStoreKeepsLegacyKeyWhenSecureMigrationFails(t *testing.T) {
	prefs := newFakePrefs()
	prefs.SetString(prefKeyPrefix+providerAnthropic, "legacy-key")
	secrets := &fakeSecrets{m: map[string]string{}, failWrite: true}
	ks := &keyStore{prefs: prefs, secrets: secrets}

	if got := ks.apiKey(providerAnthropic); got != "legacy-key" {
		t.Fatalf("legacy key after failed migration = %q, want legacy-key", got)
	}
	if got := prefs.String(prefKeyPrefix + providerAnthropic); got != "legacy-key" {
		t.Fatalf("failed migration erased legacy key: %q", got)
	}
}

func TestProviderAPIKeyPrefersEnv(t *testing.T) {
	ks := newKeyStoreWith(newFakePrefs())
	ks.setAPIKey(providerGemini, "stored")
	t.Setenv("GEMINI_API_KEY", "env")
	if providerAPIKey(ks, providerGemini) != "env" {
		t.Errorf("env should win")
	}
	t.Setenv("GEMINI_API_KEY", "")
	if providerAPIKey(ks, providerGemini) != "stored" {
		t.Errorf("stored should be used without env")
	}
}

// --- Shared prompts / orchestration -----------------------------------------

// john3Passage is the aiPassage the prompt tests build: John 3:16 in a
// non-default version, which proves the translation prompt names the ACTIVE
// version rather than a hardcoded "World English Bible".
func john3Passage() aiPassage {
	return aiPassage{Book: "John", Chapter: 3, Lo: 16, Hi: 16, Ref: "John 3:16",
		Text: "For God so loved the world", Version: "Berean Standard Bible"}
}

func TestBuildAIPromptVariesByAction(t *testing.T) {
	psg := john3Passage()
	sel, version := psg.Text, psg.Version
	explain := buildAIPrompt(aiActionExplain, psg)
	ctx := buildAIPrompt(aiActionContext, psg)
	trans := buildAIPrompt(aiActionTranslation, psg)

	for _, p := range []string{explain, ctx, trans} {
		if !strings.Contains(p, "John 3") || !strings.Contains(p, sel) {
			t.Errorf("prompt missing ref/selection: %q", p)
		}
	}
	if !strings.Contains(strings.ToLower(ctx), "context") {
		t.Errorf("context prompt should mention context")
	}
	if !strings.Contains(trans, version) {
		t.Errorf("translation prompt should name the active version (%q), got: %q", version, trans)
	}
	if strings.Contains(trans, "World English Bible") {
		t.Errorf("translation prompt must not hardcode WEB when another version is active")
	}
	if explain == ctx || ctx == trans {
		t.Errorf("prompts should differ by action")
	}
}

func TestBuildAskPrompt(t *testing.T) {
	psg := john3Passage()
	sel, version := psg.Text, psg.Version
	const question = "Who is the world here?"
	p := buildAskPrompt(psg, question)

	if !strings.Contains(p, question) {
		t.Errorf("ask prompt should carry the reader's question, got: %q", p)
	}
	for _, want := range []string{"John 3", sel, version} {
		if !strings.Contains(p, want) {
			t.Errorf("ask prompt missing %q: %q", want, p)
		}
	}
	// It must instruct a direct answer (not the fixed Explain task) and stay honest when
	// the passage doesn't address the question.
	low := strings.ToLower(p)
	if !strings.Contains(low, "question") || !strings.Contains(low, "does not address") {
		t.Errorf("ask prompt should answer the question and admit when unaddressed: %q", p)
	}
	if p == buildAIPrompt(aiActionExplain, psg) {
		t.Errorf("ask prompt should differ from the Explain prompt")
	}
}

// --- The passage every study prompt names -----------------------------------

// The span resolves to the verses the prompt cites. Controls: a state with no
// Bible, and a span past the chapter's last verse, both cite the chapter
// alone with Lo and Hi zero.
func TestAIPassageRefNamesVerses(t *testing.T) {
	st := sampleState() // John 1:1-3 loaded; the reader is on John 1
	verses := st.Bible.GetChapter("John", 1)
	v1, v2 := verses[0].Text, verses[1].Text
	for _, c := range []struct {
		name, text string
		span       selSpan
		ref        string
		lo, hi     int
	}{
		{"two verses", v1 + " " + v2, selSpanFromNative(1, 2), "John 1:1–2", 1, 2},
		{"one verse", v2, selSpanFromNative(2, 2), "John 1:2", 2, 2},
		{"a span past the last verse", "words this chapter never says", selSpanFromNative(7, 9), "John 1", 0, 0},
	} {
		p := aiPassageFor(st, c.text, c.span)
		if p.Ref != c.ref || p.Lo != c.lo || p.Hi != c.hi {
			t.Errorf("%s: Ref %q Lo %d Hi %d, want %q %d %d", c.name, p.Ref, p.Lo, p.Hi, c.ref, c.lo, c.hi)
		}
		if p.Book != "John" || p.Chapter != 1 || p.Text != c.text || p.Version != st.currentVersion().Name {
			t.Errorf("%s: passage carries %q %d %q %q", c.name, p.Book, p.Chapter, p.Text, p.Version)
		}
	}
	bare := &AppState{CurrentBook: "John", CurrentChapter: 1}
	if p := aiPassageFor(bare, v2, selSpanFromNative(2, 2)); p.Ref != "John 1" || p.Lo != 0 || p.Hi != 0 {
		t.Errorf("without a Bible: Ref %q Lo %d Hi %d, want the chapter alone", p.Ref, p.Lo, p.Hi)
	}
	if p := aiPassageFor(nil, "x", selSpan{}); p.Ref != "" || p.Text != "x" {
		t.Errorf("nil state: %+v", p)
	}
}

// The prompt's citation is the share card's citation for the same drag: the
// repeated refrain at verse 2, at verse 3, across both, a selection under the
// matching probe's floor, and the zero span's matching fallback.
func TestAIPassageRefAgreesWithShareCitation(t *testing.T) {
	st := refrainChapterState()
	for _, c := range []struct {
		text string
		span selSpan
	}{
		{refrain, selSpanFromNative(2, 2)},
		{refrain, selSpanFromNative(3, 3)},
		{"gods; " + refrain + " 3 Give thanks to the Lord of lords;", selSpanFromNative(2, 3)},
		{"gods;", selSpanFromNative(2, 2)},
		{"gods;", selSpan{}},
		{refrain, selSpan{}},
	} {
		got := aiPassageFor(st, c.text, c.span).Ref
		if want := citationForSelection(st, c.text, c.span); got != want {
			t.Errorf("%q %+v: prompt cites %q, the share card %q", c.text, c.span, got, want)
		}
		if c.span.valid() {
			if _, cite, _, _ := prepareShareQuote(st, c.text, c.span); cite != got {
				t.Errorf("%q %+v: prompt cites %q, the share pipeline %q", c.text, c.span, got, cite)
			}
		}
	}
	if got := aiPassageFor(st, refrain, selSpanFromNative(2, 2)).Ref; got != "Psalms 136:2" {
		t.Fatalf("control: the refrain at verse 2 cites %q, want Psalms 136:2", got)
	}
}

// Every study prompt ends on the passage line with the verses the selection
// resolved to, and nothing after the quotation. Control: a passage no verse
// resolves cites the chapter alone, as before.
func TestBuildAIPromptNamesVerseRange(t *testing.T) {
	psg := aiPassage{Book: "John", Chapter: 3, Lo: 16, Hi: 18, Ref: "John 3:16–18",
		Text: "For God so loved the world", Version: "World English Bible"}
	chapterOnly := psg
	chapterOnly.Lo, chapterOnly.Hi, chapterOnly.Ref = 0, 0, "John 3"
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionAsk} {
		build := func(p aiPassage) string {
			if action == aiActionAsk {
				return buildAskPrompt(p, "Who is the world here?")
			}
			return buildAIPrompt(action, p)
		}
		tail := "Passage (John 3:16–18):\n“For God so loved the world”"
		if got := build(psg); !strings.HasSuffix(got, tail) {
			t.Errorf("%s: the prompt does not end on %q:\n%s", action, tail, got)
		}
		if got := build(chapterOnly); !strings.HasSuffix(got, "Passage (John 3):\n“For God so loved the world”") {
			t.Errorf("%s: an unresolved passage must cite the chapter alone:\n%s", action, got)
		}
	}
}

// An authored line break in the selection reaches the model as a line break,
// not as the two characters a %q quoting wrote.
func TestBuildAIPromptKeepsLineBreaks(t *testing.T) {
	psg := aiPassage{Book: "Psalms", Chapter: 23, Lo: 1, Hi: 2, Ref: "Psalms 23:1–2",
		Text:    "Yahweh is my shepherd:\nI shall lack nothing.\nHe makes me lie down in green pastures.",
		Version: "World English Bible"}
	for _, action := range []string{aiActionExplain, aiActionContext, aiActionTranslation, aiActionAsk} {
		got := buildAIPrompt(action, psg)
		if action == aiActionAsk {
			got = buildAskPrompt(psg, "Who is the shepherd?")
		}
		if !strings.Contains(got, "“"+psg.Text+"”") {
			t.Errorf("%s: the selection is not quoted as written:\n%s", action, got)
		}
		if strings.Contains(got, `\n`) {
			t.Errorf("%s: a line break reached the prompt as a backslash escape:\n%s", action, got)
		}
	}
}

// An answer is filed under the verses the prompt named: the same words at
// another verse ask again, and an answer written for the chapter alone is
// never served for a verse-numbered request. Control: a selection no verse
// resolves keeps the chapter-only key it always had.
func TestRunAIActionCacheKeyCarriesVerseSpan(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "") // no key: a request not answered from the cache ends in noKeyError
	st := refrainChapterState()    // no aiKeys -> inert store, active=gemini
	base := providerGemini + "|" + geminiModel + "|" + aiActionExplain
	seed := func(scope, text, answer string) {
		key := aiCacheKey(scope, "Psalms", 136, text)
		aiCacheSet(key, answer)
		t.Cleanup(func() {
			aiCacheMu.Lock()
			delete(aiCache, key)
			aiCacheMu.Unlock()
		})
	}
	ask := func(text string, span selSpan) (string, error) {
		return runAIAction(context.Background(), st, aiActionExplain, text, "", span)
	}

	seed(base+"|2-2", refrain, "about verse two")
	if got, err := ask(refrain, selSpanFromNative(2, 2)); err != nil || got != "about verse two" {
		t.Fatalf("verse 2: got (%q, %v), want the answer filed under 2-2", got, err)
	}
	if got, err := ask(refrain, selSpanFromNative(3, 3)); !isNoKeyError(err) {
		t.Errorf("verse 3 was served verse 2's answer: (%q, %v)", got, err)
	}
	if got, err := ask(refrain, selSpanFromNative(2, 2)); err != nil || got != "about verse two" {
		t.Errorf("verse 2 again: got (%q, %v), want the same cached answer", got, err)
	}

	seed(base, refrain, "about the chapter")
	if got, err := ask(refrain, selSpanFromNative(3, 3)); !isNoKeyError(err) {
		t.Errorf("a chapter-only entry served a verse-numbered request: (%q, %v)", got, err)
	}
	// A selection under the matching floor resolves to no verse without a
	// span, so its key has no range and the chapter-only entry answers it.
	seed(base, "gods;", "about the chapter")
	if got, err := ask("gods;", selSpan{}); err != nil || got != "about the chapter" {
		t.Errorf("unresolved selection: got (%q, %v), want the chapter-only answer", got, err)
	}
	if got, err := ask("gods;", selSpanFromNative(2, 2)); !isNoKeyError(err) {
		t.Errorf("the same selection with a span was served the chapter-only answer: (%q, %v)", got, err)
	}

	// Ask folds the question in before the range.
	seed(providerGemini+"|"+geminiModel+"|ask|Why?|2-2", refrain, "asked at verse two")
	if got, err := runAIAction(context.Background(), st, aiActionAsk, refrain, "Why?", selSpanFromNative(2, 2)); err != nil || got != "asked at verse two" {
		t.Errorf("ask at verse 2: got (%q, %v)", got, err)
	}
}

func TestAIActionTitle(t *testing.T) {
	for action, want := range map[string]string{
		aiActionExplain: "Explanation", aiActionContext: "Context",
		aiActionTranslation: "Translation", aiActionAsk: "Answer", "unknown": "Explanation",
	} {
		if got := aiActionTitle(action); got != want {
			t.Errorf("aiActionTitle(%q) = %q, want %q", action, got, want)
		}
	}
}

func TestRunAIActionUsesCache(t *testing.T) {
	state := &AppState{CurrentBook: "Mark", CurrentChapter: 1} // no aiKeys -> inert store, active=gemini
	// The scope carries the RESOLVED MODEL (here the default: no override, no
	// self-heal cache) so two models' answers can never share a key — the
	// faster-model switch re-asks and both requests may settle and cache.
	key := aiCacheKey(providerGemini+"|"+geminiModel+"|"+aiActionExplain, "Mark", 1, "the beginning")
	aiCacheSet(key, "cached answer")
	t.Cleanup(func() {
		aiCacheMu.Lock()
		delete(aiCache, key)
		aiCacheMu.Unlock()
	})

	got, err := runAIAction(context.Background(), state, aiActionExplain, "the beginning", "", selSpan{})
	if err != nil || got != "cached answer" {
		t.Fatalf("cache hit: got (%q, %v)", got, err)
	}
}

func TestRunAIActionNoKeyError(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "") // ensure no ambient dev key
	state := &AppState{CurrentBook: "Acts", CurrentChapter: 2}
	_, err := runAIAction(context.Background(), state, aiActionExplain, "unique selection no cache", "", selSpan{})
	if !isNoKeyError(err) {
		t.Fatalf("want noKeyError, got %v", err)
	}
	if !strings.Contains(friendlyAIError(err), "settings") {
		t.Errorf("friendly should point to settings: %q", friendlyAIError(err))
	}
}

// A transient credential-store READ error must be indistinguishable from
// neither "absent" nor success: the legacy Preferences copy keeps serving,
// and the migration erase is SKIPPED. Collapsing error to absent could strand
// a migrated key invisible or erase the only copy.
func TestKeyStoreReadErrorKeepsLegacyCopy(t *testing.T) {
	prefs := &fakePrefs{m: map[string]string{prefKeyPrefix + "gemini": "legacy-key"}}
	secrets := &fakeSecrets{m: map[string]string{}, failRead: true}
	ks := &keyStore{prefs: prefs, secrets: secrets}

	if got := ks.apiKey("gemini"); got != "legacy-key" {
		t.Fatalf("store error must fall back to the legacy copy, got %q", got)
	}
	if prefs.m[prefKeyPrefix+"gemini"] != "legacy-key" {
		t.Fatal("store error must NOT trigger the migration erase")
	}
	if len(secrets.m) != 0 {
		t.Fatal("store error must not attempt migration writes")
	}

	// Store recovers → migration proceeds and the legacy copy is erased.
	secrets.failRead = false
	if got := ks.apiKey("gemini"); got != "legacy-key" {
		t.Fatalf("recovered store must still serve the key, got %q", got)
	}
	if secrets.m["gemini"] != "legacy-key" {
		t.Fatal("recovered store must migrate the key")
	}
	if prefs.m[prefKeyPrefix+"gemini"] != "" {
		t.Fatal("migration must erase the legacy copy after a successful write")
	}
}

// The saved-key status must describe where the key ACTUALLY is: a platform
// with a credential store whose WRITE failed (an ad-hoc-signed build has no
// keychain entitlement — reproduced on the iOS simulator) keeps the key in
// Preferences, and the UI must not claim it is in the keychain.
func TestKeyInSecureStoreReflectsRealLocation(t *testing.T) {
	secrets := &fakeSecrets{m: map[string]string{}}
	k := &keyStore{prefs: newFakePrefs(), secrets: secrets}

	if k.keyInSecureStore(providerGemini) {
		t.Error("no key stored anywhere → not in the secure store")
	}

	k.setAPIKey(providerGemini, "abc123")
	if !k.keyInSecureStore(providerGemini) {
		t.Error("a successfully written key IS in the secure store")
	}

	// Write failures (an ad-hoc-signed build has no keychain entitlement):
	// the key lives in Preferences instead, and the label must say so.
	failing := &fakeSecrets{m: map[string]string{}, failWrite: true}
	k2 := &keyStore{prefs: newFakePrefs(), secrets: failing}
	k2.setAPIKey(providerOpenAI, "xyz789")
	if k2.keyInSecureStore(providerOpenAI) {
		t.Error("a FAILED keychain write must not report the key as secured")
	}
	if got := k2.apiKey(providerOpenAI); got != "xyz789" {
		t.Errorf("the key must still be readable from Preferences, got %q", got)
	}

	// A store that errors on read is not a definitive "found".
	secrets.failRead = true
	if k.keyInSecureStore(providerGemini) {
		t.Error("a store READ error must not report the key as secured")
	}
}

// A failed keychain DELETE must still report failure — telling a reader their
// key is gone when it is still stored is a false assurance (the Settings sheet
// shows "Couldn't remove the stored key").
func TestKeyStoreFailedDeleteReportsFailure(t *testing.T) {
	secrets := &fakeSecrets{m: map[string]string{"gemini": "stored"}}
	k := &keyStore{prefs: newFakePrefs(), secrets: secrets}
	secrets.failWrite = true
	if k.setAPIKey(providerGemini, "") {
		t.Error("a failed delete must report failure, not success")
	}
}

// TestQuotedOneLineNeverDoublesTheOpeningMark: the AI sheets wrap the reader's
// selection in curly double marks for display. A verse that OPENS a quotation
// carries its own mark — Psalm 46:10 begins “Be still, and know that I am God.
// — and wrapping that verbatim printed a doubled ““ in the Explain panel
// (seen on an iPad screenshot). Marks INSIDE the line must survive: the line
// previews what was highlighted, it is not the Bluebook share pipeline.
func TestQuotedOneLineNeverDoublesTheOpeningMark(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"verse that opens a quotation",
			"“Be still, and know that I am God.",
			"“Be still, and know that I am God.”"},
		{"plain prose is unchanged",
			"For God so loved the world",
			"“For God so loved the world”"},
		{"selection carrying both of its own marks",
			"“Be still, and know that I am God.”",
			"“Be still, and know that I am God.”"},
		{"an inner quotation is preserved",
			"He said, “I am the way,” and they believed",
			"“He said, “I am the way,” and they believed”"},
		{"a trailing possessive apostrophe is not eaten",
			"the disciples’",
			"“the disciples’”"},
		{"surrounding whitespace is collapsed away",
			"  \n Be still  ",
			"“Be still”"},
	} {
		if got := quotedOneLine(tc.in, 300); got != tc.want {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
	// Truncation still applies, and the closing mark still lands last.
	long := strings.Repeat("word ", 80)
	got := quotedOneLine("“"+long, 40)
	if !strings.HasPrefix(got, "“word") || !strings.HasSuffix(got, "…”") {
		t.Errorf("truncated preview malformed: %q", got)
	}
	if strings.HasPrefix(got, "““") {
		t.Errorf("truncated preview doubled the opening mark: %q", got)
	}
}
