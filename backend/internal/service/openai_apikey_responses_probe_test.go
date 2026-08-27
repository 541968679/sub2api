//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesProbePayloadRequiresFunctionCall(t *testing.T) {
	body := openaiResponsesProbePayload("glm-4.6")

	require.Equal(t, "glm-4.6", gjson.GetBytes(body, "model").String())
	require.Equal(t, "required", gjson.GetBytes(body, "tool_choice").String())
	require.Equal(t, "function", gjson.GetBytes(body, "tools.0.type").String())
	require.Equal(t, "probe_ping", gjson.GetBytes(body, "tools.0.name").String())
	require.False(t, gjson.GetBytes(body, "instructions").Exists())
	require.False(t, gjson.GetBytes(body, "stream").Bool())
}

func TestSelectResponsesProbeModel(t *testing.T) {
	t.Run("sort first among chat models", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"z":     "z-model",
					"a":     "a-model",
					"wild":  "glm-*",
					"empty": "",
				},
			},
		}
		require.Equal(t, "a-model", selectResponsesProbeModel(account))
	})

	t.Run("audio plus default test model prefers gpt-5.4", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"audio":   "gpt-4o-audio-preview",
					"default": openai.DefaultTestModel,
				},
			},
		}
		require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(account))
		require.Equal(t, "gpt-5.4", selectResponsesProbeModel(account))
	})

	t.Run("only audio and realtime falls back to default", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"audio":    "gpt-4o-audio-preview",
					"realtime": "gpt-4o-realtime-preview",
				},
			},
		}
		require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(account))
	})

	t.Run("no mapping or only wildcards falls back to default", func(t *testing.T) {
		require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(&Account{}))
		wildcardOnly := &Account{
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"wild":  "glm-*",
					"empty": "",
				},
			},
		}
		require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(wildcardOnly))
	})

	t.Run("plain gpt-4o is kept as a chat candidate", func(t *testing.T) {
		account := &Account{
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"audio": "gpt-4o-audio-preview",
					"chat":  "gpt-4o",
				},
			},
		}
		require.Equal(t, "gpt-4o", selectResponsesProbeModel(account))
	})
}

func zhimaShapedProbeAccount(id int64, name string) *Account {
	return &Account{
		ID:       id,
		Name:     name,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://upstream.example",
			"model_mapping": map[string]any{
				"audio":   "gpt-4o-audio-preview",
				"default": openai.DefaultTestModel,
			},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported:       false,
			openai_compat.ExtraKeyChatCompletionsSupported: false,
		},
	}
}

func TestNeedsOpenAICapabilityReprobe(t *testing.T) {
	t.Run("zhima-shaped audio first and rsupp false", func(t *testing.T) {
		require.True(t, NeedsOpenAICapabilityReprobe(zhimaShapedProbeAccount(1732, "zhima")))
		require.Equal(t, "gpt-4o-audio-preview", firstSortedMappingProbeModel(zhimaShapedProbeAccount(1732, "zhima")))
	})

	t.Run("tokenbits-shaped no audio-first", func(t *testing.T) {
		account := &Account{
			ID:       12,
			Name:     "tokenbits",
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"model_mapping": map[string]any{
					"a": "gpt-5.4",
					"b": "gpt-5-mini",
				},
			},
			Extra: map[string]any{
				openai_compat.ExtraKeyResponsesSupported:       false,
				openai_compat.ExtraKeyChatCompletionsSupported: false,
			},
		}
		require.False(t, isNonChatProbeModel(firstSortedMappingProbeModel(account)))
		require.False(t, NeedsOpenAICapabilityReprobe(account))
	})

	t.Run("tokenbits-shaped both flags true", func(t *testing.T) {
		account := zhimaShapedProbeAccount(15, "jizhiapi")
		account.Extra[openai_compat.ExtraKeyResponsesSupported] = true
		account.Extra[openai_compat.ExtraKeyChatCompletionsSupported] = true
		require.False(t, NeedsOpenAICapabilityReprobe(account))
	})

	t.Run("missing extra keys", func(t *testing.T) {
		account := zhimaShapedProbeAccount(1, "unknown")
		account.Extra = nil
		require.False(t, NeedsOpenAICapabilityReprobe(account))
		account.Extra = map[string]any{
			openai_compat.ExtraKeyResponsesSupported: "false",
		}
		require.False(t, NeedsOpenAICapabilityReprobe(account))
	})

	t.Run("oauth and non-openai are skipped", func(t *testing.T) {
		oauth := zhimaShapedProbeAccount(2, "oauth")
		oauth.Type = AccountTypeOAuth
		require.False(t, NeedsOpenAICapabilityReprobe(oauth))
		claude := zhimaShapedProbeAccount(3, "claude")
		claude.Platform = PlatformAnthropic
		require.False(t, NeedsOpenAICapabilityReprobe(claude))
		require.False(t, NeedsOpenAICapabilityReprobe(nil))
	})

	t.Run("audio present but not sort-first is not eligible", func(t *testing.T) {
		account := zhimaShapedProbeAccount(4, "chat-first")
		account.Credentials["model_mapping"] = map[string]any{
			"chat":  "gpt-4o",
			"audio": "gpt-4o-audio-preview",
		}
		require.Equal(t, "gpt-4o", firstSortedMappingProbeModel(account))
		require.False(t, NeedsOpenAICapabilityReprobe(account))
	})

	t.Run("ccsupp-only false still eligible", func(t *testing.T) {
		account := zhimaShapedProbeAccount(5, "cc-false")
		account.Extra[openai_compat.ExtraKeyResponsesSupported] = true
		account.Extra[openai_compat.ExtraKeyChatCompletionsSupported] = false
		require.True(t, NeedsOpenAICapabilityReprobe(account))
	})

	t.Run("passthrough mode does not block eligibility", func(t *testing.T) {
		account := zhimaShapedProbeAccount(6, "zhima-passthrough")
		account.Extra[openai_compat.ExtraKeyResponsesMode] = string(openai_compat.ResponsesSupportModePassthrough)
		require.True(t, NeedsOpenAICapabilityReprobe(account))
	})
}

type openaiCapabilityReprobeRepo struct {
	openAIAccountTestRepo
	listed []Account
}

func (r *openaiCapabilityReprobeRepo) ListAllWithFilters(context.Context, string, string, string, string, int64, string) ([]Account, error) {
	return r.listed, nil
}

func (r *openaiCapabilityReprobeRepo) UpdateExtra(ctx context.Context, id int64, updates map[string]any) error {
	if err := r.openAIAccountTestRepo.UpdateExtra(ctx, id, updates); err != nil {
		return err
	}
	if acc := r.accountsByID[id]; acc != nil {
		if acc.Extra == nil {
			acc.Extra = map[string]any{}
		}
		for k, v := range updates {
			acc.Extra[k] = v
		}
	}
	return nil
}

func TestAccountTestService_ListAndReprobeOpenAIAPIKeysNeedingCapabilityReprobe(t *testing.T) {
	zhima := zhimaShapedProbeAccount(1732, "zhima")
	tokenbits := &Account{
		ID:       12,
		Name:     "tokenbits",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "sk-test",
			"model_mapping": map[string]any{
				"a": "gpt-5.4",
			},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported:       true,
			openai_compat.ExtraKeyChatCompletionsSupported: true,
		},
	}
	missing := zhimaShapedProbeAccount(99, "missing-keys")
	missing.Extra = map[string]any{}
	oauth := zhimaShapedProbeAccount(88, "oauth")
	oauth.Type = AccountTypeOAuth

	repo := &openaiCapabilityReprobeRepo{
		openAIAccountTestRepo: openAIAccountTestRepo{
			mockAccountRepoForGemini: mockAccountRepoForGemini{
				accountsByID: map[int64]*Account{
					zhima.ID: zhima,
				},
			},
		},
		listed: []Account{*zhima, *tokenbits, *missing, *oauth},
	}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusOK, `{"output":[{"type":"function_call","name":"probe_ping"}]}`),
		newJSONResponse(http.StatusOK, `{"choices":[{"index":0,"message":{"content":"pong"}}]}`),
	}}
	svc := &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
	}

	listed, err := svc.ListOpenAIAPIKeysNeedingCapabilityReprobe(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, int64(1732), listed[0].ID)

	dry, err := svc.ReprobeOpenAIAPIKeysNeedingCapabilityReprobe(context.Background(), true, false)
	require.NoError(t, err)
	require.True(t, dry.DryRun)
	require.False(t, dry.AllAPIKeys)
	require.Equal(t, 1, dry.Count)
	require.Equal(t, int64(1732), dry.Accounts[0].AccountID)
	require.Equal(t, "gpt-4o-audio-preview", dry.Accounts[0].OldProbeModel)
	require.Equal(t, openai.DefaultTestModel, dry.Accounts[0].NewProbeModel)
	require.True(t, dry.Accounts[0].NeedsOpenAICapabilityReprobe)
	require.Empty(t, upstream.requests)

	allDry, err := svc.ReprobeOpenAIAPIKeysNeedingCapabilityReprobe(context.Background(), true, true)
	require.NoError(t, err)
	require.True(t, allDry.AllAPIKeys)
	require.Equal(t, 3, allDry.Count)
	require.Empty(t, upstream.requests)

	exec, err := svc.ReprobeOpenAIAPIKeysNeedingCapabilityReprobe(context.Background(), false, false)
	require.NoError(t, err)
	require.False(t, exec.DryRun)
	require.Equal(t, 1, exec.Count)
	require.NotNil(t, exec.Accounts[0].ResponsesSupported)
	require.True(t, *exec.Accounts[0].ResponsesSupported)
	require.NotNil(t, exec.Accounts[0].ChatCompletionsSupported)
	require.True(t, *exec.Accounts[0].ChatCompletionsSupported)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "http://upstream.example/v1/responses", upstream.requests[0].URL.String())
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.requests[1].URL.String())
	for _, req := range upstream.requests {
		body, readErr := io.ReadAll(req.Body)
		require.NoError(t, readErr)
		require.Equal(t, openai.DefaultTestModel, gjson.GetBytes(body, "model").String())
	}
	require.Equal(t, map[string]any{
		openai_compat.ExtraKeyResponsesSupported:       true,
		openai_compat.ExtraKeyChatCompletionsSupported: true,
	}, repo.updatedExtra)
	_, hasMode := repo.updatedExtra[openai_compat.ExtraKeyResponsesMode]
	require.False(t, hasMode)
}

func TestDecideResponsesProbeSupport(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "404 unsupported", status: http.StatusNotFound, body: `{}`, want: false},
		{name: "405 unsupported", status: http.StatusMethodNotAllowed, body: `{}`, want: false},
		{name: "400 endpoint exists", status: http.StatusBadRequest, body: `{}`, want: true},
		{name: "500 endpoint exists", status: http.StatusInternalServerError, body: `{}`, want: true},
		{
			name:   "2xx with function call",
			status: http.StatusOK,
			body:   `{"output":[{"type":"reasoning"},{"type":"function_call","name":"probe_ping"}]}`,
			want:   true,
		},
		{
			name:   "2xx reasoning only",
			status: http.StatusOK,
			body:   `{"output":[{"type":"reasoning"}]}`,
			want:   false,
		},
		{name: "2xx invalid body", status: http.StatusOK, body: `{}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, decideResponsesProbeSupport(tt.status, []byte(tt.body)))
		})
	}
}

func TestDecideChatCompletionsProbeSupport(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "404 unsupported", status: http.StatusNotFound, body: `{}`, want: false},
		{name: "405 unsupported", status: http.StatusMethodNotAllowed, body: `{}`, want: false},
		{name: "400 endpoint exists", status: http.StatusBadRequest, body: `{}`, want: true},
		{name: "500 endpoint exists", status: http.StatusInternalServerError, body: `{}`, want: true},
		{
			name:   "2xx with choices",
			status: http.StatusOK,
			body:   `{"choices":[{"index":0,"message":{"content":"pong"}}]}`,
			want:   true,
		},
		{
			name:   "2xx chat.completion object",
			status: http.StatusOK,
			body:   `{"object":"chat.completion","id":"cmpl"}`,
			want:   true,
		},
		{name: "2xx unlike chat completions", status: http.StatusOK, body: `{"output":[]}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, decideChatCompletionsProbeSupport(tt.status, []byte(tt.body)))
		})
	}
}
