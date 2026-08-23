//go:build unit

package service

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveAccountTestModel_AnthropicAPIKeyIdentityAndPassthrough(t *testing.T) {
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001",
			},
		},
	}

	dated := resolveAccountTestModel(account, "claude-haiku-4-5-20251001")
	require.Equal(t, "claude-haiku-4-5-20251001", dated.Selected)
	require.Equal(t, "claude-haiku-4-5-20251001", dated.Mapped)
	require.Equal(t, AccountTestMappingSourceAccount, dated.Source)

	short := resolveAccountTestModel(account, "claude-haiku-4-5")
	require.Equal(t, "claude-haiku-4-5", short.Selected)
	require.Equal(t, "claude-haiku-4-5", short.Mapped)
	require.Equal(t, AccountTestMappingSourceNone, short.Source)
}

func TestResolveAccountTestModel_AnthropicOAuthUsesPrefix(t *testing.T) {
	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
	}

	got := resolveAccountTestModel(account, "claude-haiku-4-5")
	require.Equal(t, "claude-haiku-4-5", got.Selected)
	require.Equal(t, "claude-haiku-4-5-20251001", got.Mapped)
	require.Equal(t, AccountTestMappingSourcePrefix, got.Source)
}

func TestAccountTestService_ClaudeAPIKey403DoesNotSetError(t *testing.T) {
	ctx, recorder := newTestContext()
	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusForbidden, `{"error":"resolve groups failed: model unsupported by selected groups: claude-haiku-4-5"}`),
	}}
	svc := &AccountTestService{
		accountRepo:         repo,
		httpUpstream:        upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}

	for _, poolMode := range []bool{true, false} {
		t.Run(map[bool]string{true: "pool_on", false: "pool_off"}[poolMode], func(t *testing.T) {
			repo.setErrorID = 0
			repo.setErrorMsg = ""
			upstream.responses = []*http.Response{
				newJSONResponse(http.StatusForbidden, `{"error":"model unsupported"}`),
			}
			account := &Account{
				ID:          1650,
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":   "sk-test",
					"base_url":  "https://compatible.example.com",
					"pool_mode": poolMode,
					"model_mapping": map[string]any{
						"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001",
					},
				},
			}

			err := svc.testClaudeAccountConnection(ctx, account, "claude-haiku-4-5")
			require.Error(t, err)
			require.Zero(t, repo.setErrorID)
			require.Len(t, upstream.requests, 1)
			body, readErr := io.ReadAll(upstream.requests[0].Body)
			require.NoError(t, readErr)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(body, &payload))
			require.Equal(t, "claude-haiku-4-5", payload["model"])
			require.Contains(t, recorder.Body.String(), `"selected_model":"claude-haiku-4-5"`)
			require.Contains(t, recorder.Body.String(), `"mapped_model":"claude-haiku-4-5"`)
			require.Contains(t, recorder.Body.String(), `"mapping_source":"none"`)
			upstream.requests = nil
		})
	}
}

func TestAccountTestService_ClaudeAPIKeyIdentityUsesAccountSource(t *testing.T) {
	ctx, recorder := newTestContext()
	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		newJSONResponse(http.StatusForbidden, `{"error":"no"}`),
	}}
	svc := &AccountTestService{
		accountRepo:         repo,
		httpUpstream:        upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{Enabled: false},
			},
		},
	}
	account := &Account{
		ID:          1650,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://compatible.example.com",
			"model_mapping": map[string]any{
				"claude-haiku-4-5-20251001": "claude-haiku-4-5-20251001",
			},
		},
	}

	err := svc.testClaudeAccountConnection(ctx, account, "claude-haiku-4-5-20251001")
	require.Error(t, err)
	require.Zero(t, repo.setErrorID)
	require.Len(t, upstream.requests, 1)
	body, readErr := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, readErr)
	require.Contains(t, string(body), `"model":"claude-haiku-4-5-20251001"`)
	require.Contains(t, recorder.Body.String(), `"mapping_source":"account"`)
}

func TestAccountTestService_ClaudeOAuthShortNameUsesPrefix(t *testing.T) {
	ctx, recorder := newTestContext()
	repo := &openAIAccountTestRepo{}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{
		{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"ok\"}}\n\n",
			)),
		},
	}}
	svc := &AccountTestService{
		accountRepo:         repo,
		httpUpstream:        upstream,
		tlsFPProfileService: &TLSFingerprintProfileService{},
	}
	account := &Account{
		ID:          7,
		Platform:    PlatformAnthropic,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "oauth-token"},
	}

	err := svc.testClaudeAccountConnection(ctx, account, "claude-haiku-4-5")
	require.NoError(t, err)
	require.Zero(t, repo.setErrorID)
	require.Len(t, upstream.requests, 1)
	body, readErr := io.ReadAll(upstream.requests[0].Body)
	require.NoError(t, readErr)
	require.Contains(t, string(body), `"model":"claude-haiku-4-5-20251001"`)
	require.Contains(t, recorder.Body.String(), `"selected_model":"claude-haiku-4-5"`)
	require.Contains(t, recorder.Body.String(), `"mapped_model":"claude-haiku-4-5-20251001"`)
	require.Contains(t, recorder.Body.String(), `"mapping_source":"prefix"`)
}
