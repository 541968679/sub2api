package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCodexFingerprintContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{"codex_cli_only": true}}
	detector := NewOpenAICodexClientRestrictionDetector(nil)

	contextWith := func(userAgent string, headers map[string]string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		c.Request.Header.Set("User-Agent", userAgent)
		for key, value := range headers {
			c.Request.Header.Set(key, value)
		}
		return c
	}

	t.Run("official client fails closed without required engine signal", func(t *testing.T) {
		result := detector.DetectWithPolicy(contextWith("codex_cli_rs/0.41.0", nil), account, CodexRestrictionPolicy{
			EngineFingerprintSignals: openai.DefaultEngineFingerprintSignals,
		}, nil)
		require.False(t, result.Matched)
		require.Equal(t, CodexClientRestrictionReasonMissingEngineFingerprint, result.Reason)
	})

	t.Run("official client with fingerprint preserves default compatibility", func(t *testing.T) {
		result := detector.DetectWithPolicy(contextWith("codex_cli_rs/0.41.0", map[string]string{"x-codex-window-id": "window"}), account, CodexRestrictionPolicy{
			EngineFingerprintSignals: openai.DefaultEngineFingerprintSignals,
		}, nil)
		require.True(t, result.Matched)
	})

	t.Run("empty policy preserves legacy official originator behavior", func(t *testing.T) {
		result := detector.DetectWithPolicy(contextWith("curl/8", map[string]string{"originator": "codex_cli_rs"}), account, CodexRestrictionPolicy{}, nil)
		require.True(t, result.Matched)
		require.Equal(t, CodexClientRestrictionReasonMatchedOriginator, result.Reason)
	})

	t.Run("empty policy does not require an engine fingerprint", func(t *testing.T) {
		result := detector.DetectWithPolicy(contextWith("codex_cli_rs/no-semver", nil), account, CodexRestrictionPolicy{}, nil)
		require.True(t, result.Matched)
	})

	t.Run("version rejection carries actionable bounds", func(t *testing.T) {
		result := detector.DetectWithPolicy(contextWith("codex_cli_rs/0.39.0", nil), account, CodexRestrictionPolicy{MinCodexVersion: "0.42.0"}, nil)
		require.False(t, result.Matched)
		require.Equal(t, "0.39.0", result.DetectedVersion)
		require.Equal(t, "0.42.0", result.MinCodexVersion)
		require.Contains(t, CodexClientRestrictionMessage(result), "0.39.0")
		require.Contains(t, CodexClientRestrictionMessage(result), "0.42.0")
	})

	t.Run("restriction remains isolated to OpenAI OAuth account", func(t *testing.T) {
		for _, candidate := range []*Account{
			{Platform: PlatformGrok, Type: AccountTypeOAuth, Extra: map[string]any{"codex_cli_only": true}},
			{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"codex_cli_only": true}},
		} {
			result := detector.DetectWithPolicy(contextWith("curl/8", nil), candidate, CodexRestrictionPolicy{}, nil)
			require.False(t, result.Enabled)
		}
	})
}
