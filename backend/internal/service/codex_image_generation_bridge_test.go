package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAccount_CodexImageGenerationBridgeOverride(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    *bool
	}{
		{
			name: "new top-level enabled",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"codex_image_generation_bridge": true,
				},
			},
			want: boolPtrForTest(true),
		},
		{
			name: "new top-level disabled",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"codex_image_generation_bridge": false,
				},
			},
			want: boolPtrForTest(false),
		},
		{
			name: "legacy top-level enabled",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"codex_image_generation_bridge_enabled": true,
				},
			},
			want: boolPtrForTest(true),
		},
		{
			name: "new key has priority over legacy key",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"codex_image_generation_bridge":         false,
					"codex_image_generation_bridge_enabled": true,
				},
			},
			want: boolPtrForTest(false),
		},
		{
			name: "nested openai new key",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"openai": map[string]any{
						"codex_image_generation_bridge": true,
					},
				},
			},
			want: boolPtrForTest(true),
		},
		{
			name: "nested openai legacy key",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra: map[string]any{
					"openai": map[string]any{
						"codex_image_generation_bridge_enabled": false,
					},
				},
			},
			want: boolPtrForTest(false),
		},
		{
			name: "non-openai ignored",
			account: &Account{
				Platform: PlatformAnthropic,
				Extra: map[string]any{
					"codex_image_generation_bridge": true,
				},
			},
			want: nil,
		},
		{
			name: "missing override follows global",
			account: &Account{
				Platform: PlatformOpenAI,
				Extra:    map[string]any{},
			},
			want: nil,
		},
		{
			name:    "nil account",
			account: nil,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.account.CodexImageGenerationBridgeOverride()
			if tt.want == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, *tt.want, *got)
		})
	}
}

func TestOpenAIGatewayService_IsCodexImageGenerationBridgeEnabled(t *testing.T) {
	t.Run("account override enables when global disabled", func(t *testing.T) {
		svc := &OpenAIGatewayService{cfg: &config.Config{}}
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				"codex_image_generation_bridge": true,
			},
		}
		require.True(t, svc.isCodexImageGenerationBridgeEnabled(account))
	})

	t.Run("account override disables when global enabled", func(t *testing.T) {
		svc := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{CodexImageGenerationBridgeEnabled: true},
			},
		}
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				"codex_image_generation_bridge": false,
			},
		}
		require.False(t, svc.isCodexImageGenerationBridgeEnabled(account))
	})

	t.Run("falls back to global config", func(t *testing.T) {
		enabled := &OpenAIGatewayService{
			cfg: &config.Config{
				Gateway: config.GatewayConfig{CodexImageGenerationBridgeEnabled: true},
			},
		}
		disabled := &OpenAIGatewayService{cfg: &config.Config{}}
		account := &Account{Platform: PlatformOpenAI, Extra: map[string]any{}}

		require.True(t, enabled.isCodexImageGenerationBridgeEnabled(account))
		require.False(t, disabled.isCodexImageGenerationBridgeEnabled(account))
	})

	t.Run("nil service and nil account are disabled", func(t *testing.T) {
		var svc *OpenAIGatewayService
		require.False(t, svc.isCodexImageGenerationBridgeEnabled(nil))
	})
}

func boolPtrForTest(v bool) *bool {
	return &v
}
