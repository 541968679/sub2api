//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCodexFingerprintSeed = "11111111-1111-4111-8111-111111111111"

func newTestOAuthAccount(id int64, extra map[string]any) *Account {
	if codexFingerprintModeRequiresSeed(codexFingerprintModeFromExtra(extra)) {
		if extra == nil {
			extra = make(map[string]any)
		}
		if _, exists := extra[codexFingerprintSeedExtraKey]; !exists {
			extra[codexFingerprintSeedExtraKey] = testCodexFingerprintSeed
		}
	}
	return &Account{
		ID:       id,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra:    extra,
	}
}

func TestGetCodexFingerprintMode(t *testing.T) {
	tests := []struct {
		name     string
		account  *Account
		expected codexFingerprintMode
	}{
		{"nil 账号", nil, codexFingerprintOff},
		{"非 OAuth 账号", &Account{Platform: PlatformOpenAI, Type: "api_key"}, codexFingerprintOff},
		{"OpenAI setup token", &Account{Platform: PlatformOpenAI, Type: AccountTypeSetupToken, Extra: map[string]any{codexFingerprintModeExtraKey: "session"}}, codexFingerprintSession},
		{"Anthropic setup token", &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Extra: map[string]any{codexFingerprintModeExtraKey: "session"}}, codexFingerprintOff},
		{"无 extra 默认 off", newTestOAuthAccount(1, nil), codexFingerprintOff},
		{"空值默认 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: ""}), codexFingerprintOff},
		{"非法值默认 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "invalid"}), codexFingerprintOff},
		{"显式 off", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"}), codexFingerprintOff},
		{"device", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "device"}), codexFingerprintDevice},
		{"session", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "session"}), codexFingerprintSession},
		{"full", newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "full"}), codexFingerprintFull},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.account.GetCodexFingerprintMode())
		})
	}
}

func TestResolveCodexFingerprintIDsFromRequest_ExplicitOff(t *testing.T) {
	account := newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: "off"})
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	assert.Nil(t, ids, "显式 off 模式应返回 nil")
}

func TestResolveCodexFingerprintIDsFromRequest_DefaultIsOff(t *testing.T) {
	account := newTestOAuthAccount(1, nil)
	assert.Nil(t, resolveCodexFingerprintIDsFromRequest(account, nil), "无 extra 应视为 off")
}

func TestResolveCodexFingerprintIDsFromRequest_ExplicitOptInHonored(t *testing.T) {
	for _, mode := range []string{"device", "session", "full"} {
		t.Run(mode, func(t *testing.T) {
			account := newTestOAuthAccount(1, map[string]any{codexFingerprintModeExtraKey: mode})
			ids := resolveCodexFingerprintIDsFromRequest(account, nil)
			require.NotNil(t, ids, "显式配置必须生效")
			assert.Equal(t, codexFingerprintMode(mode), ids.mode)
			assert.NotEmpty(t, ids.installationID)
			_, err := uuid.Parse(ids.installationID)
			require.NoError(t, err)
		})
	}
}

func TestApplyCodexFingerprintHeaders_OffMode(t *testing.T) {
	h := http.Header{}
	h.Set("session_id", "client-session")
	applyCodexFingerprintHeaders(h, nil)
	assert.Equal(t, "client-session", h.Get("session_id"))
	assert.Empty(t, h.Get("x-codex-installation-id"))
}

func TestApplyCodexFingerprintHeaders_DeviceMode(t *testing.T) {
	account := newTestOAuthAccount(7, map[string]any{codexFingerprintModeExtraKey: "device"})
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	require.NotNil(t, ids)

	h := http.Header{}
	h.Set("session_id", "client-session")
	h.Set("x-codex-turn-metadata", `{"sandbox":"none","installation_id":"old"}`)
	applyCodexFingerprintHeaders(h, ids)
	assert.Equal(t, ids.installationID, h.Get("x-codex-installation-id"))
	assert.Equal(t, "client-session", h.Get("session_id"), "device mode must not rewrite session_id")
	assert.Contains(t, h.Get("x-codex-turn-metadata"), ids.installationID)
	assert.Contains(t, h.Get("x-codex-turn-metadata"), `"sandbox":"none"`)
}

func TestApplyCodexFingerprintHeaders_SessionMode(t *testing.T) {
	account := newTestOAuthAccount(8, map[string]any{codexFingerprintModeExtraKey: "session"})
	client := http.Header{}
	client.Set("session-id", "client-session-a")
	ids := resolveCodexFingerprintIDsFromRequest(account, client)
	require.NotNil(t, ids)
	require.NotEmpty(t, ids.sessionID)

	h := http.Header{}
	h.Set("session_id", "client-session-a")
	applyCodexFingerprintHeaders(h, ids)
	assert.Equal(t, ids.sessionID, h.Get("session_id"))
	assert.Equal(t, ids.sessionID, h.Get("session-id"))
	assert.Equal(t, ids.threadID, h.Get("thread-id"))
	assert.Equal(t, ids.installationID, h.Get("x-codex-installation-id"))
}

func TestApplyCodexFingerprintHeaders_FullMode(t *testing.T) {
	account := newTestOAuthAccount(9, map[string]any{codexFingerprintModeExtraKey: "full"})
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	require.NotNil(t, ids)

	h := http.Header{}
	applyCodexFingerprintHeaders(h, ids)
	assert.Equal(t, ids.sessionID, h.Get("session_id"))
	assert.Equal(t, ids.threadID, h.Get("thread-id"))
	assert.Equal(t, ids.windowID, h.Get("x-codex-window-id"))
}

func TestApplyCodexFingerprintClientMetadata_OffMode(t *testing.T) {
	body := map[string]any{"client_metadata": map[string]any{"session_id": "keep-me"}}
	assert.False(t, applyCodexFingerprintClientMetadata(body, nil))
	meta := body["client_metadata"].(map[string]any)
	assert.Equal(t, "keep-me", meta["session_id"])
}

func TestApplyCodexFingerprintClientMetadata_SessionMode(t *testing.T) {
	account := newTestOAuthAccount(10, map[string]any{codexFingerprintModeExtraKey: "session"})
	ids := resolveCodexFingerprintIDsFromRequest(account, http.Header{"session-id": []string{"client-session-a"}})
	require.NotNil(t, ids)

	body := map[string]any{"client_metadata": map[string]any{"session_id": "client-session-a", "other": "keep"}}
	require.True(t, applyCodexFingerprintClientMetadata(body, ids))
	meta := body["client_metadata"].(map[string]any)
	assert.Equal(t, ids.sessionID, meta["session_id"])
	assert.Equal(t, ids.turnID, meta["turn_id"])
	assert.Equal(t, "keep", meta["other"])
}

func TestFingerprintIDs_HeaderAndBody_TurnID_Consistent(t *testing.T) {
	account := newTestOAuthAccount(11, map[string]any{codexFingerprintModeExtraKey: "full"})
	ids := resolveCodexFingerprintIDsFromRequest(account, nil)
	require.NotNil(t, ids)

	h := http.Header{}
	h.Set("x-codex-turn-metadata", `{"turn_id":"old"}`)
	applyCodexFingerprintHeaders(h, ids)
	body := map[string]any{"client_metadata": map[string]any{}}
	require.True(t, applyCodexFingerprintClientMetadata(body, ids))
	meta := body["client_metadata"].(map[string]any)
	assert.Equal(t, ids.turnID, meta["turn_id"])
	assert.Contains(t, h.Get("x-codex-turn-metadata"), ids.turnID)
}

func TestExtractClientSessionID(t *testing.T) {
	hyphen := http.Header{}
	hyphen.Set("session-id", "hyphen-form")
	hyphen.Set("session_id", "underscore-form")
	assert.Equal(t, "hyphen-form", extractClientSessionID(hyphen))

	underscore := http.Header{}
	underscore.Set("session_id", "underscore-form")
	assert.Equal(t, "underscore-form", extractClientSessionID(underscore))
	assert.Equal(t, "", extractClientSessionID(http.Header{}))
}
