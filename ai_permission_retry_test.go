package bibletext

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAIReaderPermissionDuringRetry(t *testing.T) {
	isolateAICache(t)
	const (
		key         = "synthetic-approved"
		original    = "gpt-5-synthetic-old"
		replacement = "gpt-5-synthetic-new"
	)
	for _, action := range []string{"Find", "Study", "Test key"} {
		for _, tc := range []struct {
			name, during, change              string
			firstStatus, wantSends, wantLists int
			cancel                            bool
		}{
			{"still allowed", "", "", 404, 2, 1, false},
			{"clear as successful reply arrives", "send", "clear", 200, 1, 0, true},
			{"clear and allow before first send", "start", "renew", 404, 0, 0, true},
			{"clear during first request", "send", "clear", 404, 1, 0, true},
			{"replace during first request", "send", "replace", 404, 1, 0, true},
			{"clear during discovery", "list", "clear", 404, 1, 1, true},
			{"replace during discovery", "list", "replace", 404, 1, 1, true},
			{"clear with environment override", "send", "clear with env", 404, 1, 0, true},
			{"withdraw during discovery", "list", "disable", 404, 1, 1, true},
			{"environment key changes", "list", "env changes", 404, 1, 1, true},
			{"clear and allow same key again", "list", "renew", 404, 1, 1, true},
			{"same key saved again", "list", "same", 404, 2, 1, false},
			{"other provider cleared", "list", "other", 404, 2, 1, false},
			{"permitted transport retry", "", "", 503, 2, 0, false},
			{"withdraw before transport retry", "send", "clear", 503, 1, 0, true},
		} {
			if action == "Test key" && tc.name != "still allowed" {
				continue // The fixed probe never carries a reader's query or passage.
			}
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				store := newTestKeyStore()
				store.setActiveProvider(providerOpenAI)
				store.setAPIKey(providerOpenAI, key)
				if tc.change == "clear with env" || tc.change == "env changes" {
					store.setAPIKey(providerOpenAI, "synthetic-saved")
					t.Setenv("OPENAI_API_KEY", key)
				}
				if action != "Test key" {
					grantAIPermission(store, providerOpenAI)
				}
				change := func(stage string) {
					if tc.during != stage {
						return
					}
					switch tc.change {
					case "clear", "clear with env":
						store.setAPIKey(providerOpenAI, "")
					case "replace":
						store.setAPIKey(providerOpenAI, "synthetic-replacement")
					case "disable":
						store.setAIEnabled(false)
					case "renew":
						store.setAPIKey(providerOpenAI, "")
						store.setAPIKey(providerOpenAI, key)
						grantAIPermission(store, providerOpenAI)
					case "same":
						store.setAPIKey(providerOpenAI, key)
					case "other":
						store.setAPIKey(providerGemini, "")
					case "env changes":
						t.Setenv("OPENAI_API_KEY", "synthetic-environment")
					}
				}
				sends, lists := 0, 0
				rt := mockTransport(func(req *http.Request) (*http.Response, error) {
					sends++
					if req.Header.Get("Authorization") != "Bearer "+key {
						t.Fatal("unexpected request credential")
					}
					status := http.StatusOK
					body := `{"choices":[{"message":{"content":"Acts 2:1"}}]}`
					if sends == 1 {
						change("send")
						status = tc.firstStatus
						if status != http.StatusOK {
							body = `{"error":{"message":"synthetic service response"}}`
						}
					}
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
				})
				newClient := func(s *keyStore, apiKey string) aiClient {
					change("start")
					return &modelResolver{
						store: s, id: providerOpenAI, apiKey: apiKey, def: original, tier: "gpt-5",
						list: func(_ context.Context, credential string) ([]discoveredModel, error) {
							if credential != key {
								t.Fatal("unexpected catalog credential")
							}
							lists++
							change("list")
							return []discoveredModel{{id: replacement, rank: 1}}, nil
						},
						build: func(model, _ string) aiClient {
							c := newOpenAIClient(apiKey, "https://synthetic.invalid", model)
							c.http = &http.Client{Transport: rt}
							return c
						},
					}
				}
				info, _ := providerByID(providerOpenAI)
				info.New = newClient
				ctx := context.Background()
				var err error
				state := &AppState{aiKeys: store}
				switch action {
				case "Find":
					prev := aiSearchGenerate
					t.Cleanup(func() { aiSearchGenerate = prev })
					aiSearchGenerate = func(ctx context.Context, _ providerInfo, store *keyStore, key, prompt string) (string, error) {
						return prev(ctx, info, store, key, prompt)
					}
					_, err = runAISearch(ctx, state, "A synthetic query "+tc.name)
				case "Study":
					prev := aiStudyGenerate
					t.Cleanup(func() { aiStudyGenerate = prev })
					aiStudyGenerate = func(ctx context.Context, _ providerInfo, store *keyStore, key, prompt string) (string, error) {
						return prev(ctx, info, store, key, prompt)
					}
					_, err = runAIActionForPassage(ctx, state, aiActionExplain, aiPassage{Book: "Acts", Chapter: 2, Text: "A synthetic passage " + tc.name}, "")
				case "Test key":
					err = aiKeyTestRun(ctx, info, store, key)
				}
				if sends != tc.wantSends || lists != tc.wantLists || (tc.cancel && !errors.Is(err, context.Canceled)) || (!tc.cancel && err != nil) {
					t.Fatalf("sends=%d lists=%d err=%v; want %d, %d, canceled=%v", sends, lists, err, tc.wantSends, tc.wantLists, tc.cancel)
				}
				if tc.cancel && store.resolvedModel(providerOpenAI) != "" {
					t.Error("revoked request saved its replacement model")
				}
			})
		}
	}
}
