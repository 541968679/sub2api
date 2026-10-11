//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_UpdateAccount_PreservesOAuthSecretsWhenLiteCredentialsPosted(t *testing.T) {
	t.Parallel()

	repo := &accountRepoStubForBulkUpdate{
		getByIDAccounts: map[int64]*Account{
			4: {
				ID:       4,
				Name:     "ces",
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Status:   StatusActive,
				Credentials: map[string]any{
					"access_token":          "keep-at",
					"chatgpt_session_token": "keep-st",
					"email":                 "user@example.com",
					"plan_type":             "free",
					"expires_at":            "2026-09-12T08:11:35Z",
				},
			},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	updated, err := svc.UpdateAccount(context.Background(), 4, &UpdateAccountInput{
		Credentials: map[string]any{
			"email":                     "user@example.com",
			"plan_type":                 "free",
			"intercept_warmup_requests": true,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "keep-at", updated.Credentials["access_token"])
	require.Equal(t, "keep-st", updated.Credentials["chatgpt_session_token"])
	require.Equal(t, "2026-09-12T08:11:35Z", updated.Credentials["expires_at"])
	require.Equal(t, true, updated.Credentials["intercept_warmup_requests"])
}

func TestPreserveOAuthSecretCredentialsKeepsIncomingTokens(t *testing.T) {
	t.Parallel()

	out := preserveOAuthSecretCredentials(
		map[string]any{
			"access_token":          "old-at",
			"chatgpt_session_token": "old-st",
			"refresh_token":         "old-rt",
		},
		map[string]any{
			"access_token":          "new-at",
			"chatgpt_session_token": "new-st",
			"plan_type":             "plus",
		},
	)
	require.Equal(t, "new-at", out["access_token"])
	require.Equal(t, "new-st", out["chatgpt_session_token"])
	require.Equal(t, "old-rt", out["refresh_token"])
	require.Equal(t, "plus", out["plan_type"])
}

func TestPreserveOAuthSecretCredentialsKeepsTokenRhythmSession(t *testing.T) {
	t.Parallel()

	out := preserveOAuthSecretCredentials(
		map[string]any{
			"api_key":                    "sk_tr_old",
			"tokenrhythm_session_token":  "sess_old",
			"tokenrhythm_refresh_token":  "rf_old",
		},
		map[string]any{
			"api_key": "sk_tr_new",
			"base_url": "https://tokenrhythm.studio",
		},
	)
	require.Equal(t, "sk_tr_new", out["api_key"])
	require.Equal(t, "sess_old", out["tokenrhythm_session_token"])
	require.Equal(t, "rf_old", out["tokenrhythm_refresh_token"])
}
