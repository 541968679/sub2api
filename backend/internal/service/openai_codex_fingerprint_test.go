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
