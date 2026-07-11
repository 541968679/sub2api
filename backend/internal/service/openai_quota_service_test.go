package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type openAIQuotaAccountRepoStub struct {
	AccountRepository
	account *Account
}

func (r *openAIQuotaAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, ErrAccountNotFound
}

func TestOpenAIQuotaRejectsGrokBeforeUpstreamAccess(t *testing.T) {
	repo := &openAIQuotaAccountRepoStub{account: &Account{
		ID:       42,
		Platform: PlatformGrok,
		Type:     AccountTypeOAuth,
	}}
	upstreamCalled := false
	svc := NewOpenAIQuotaService(repo, nil, &OpenAITokenProvider{}, func(string) (*req.Client, error) {
		upstreamCalled = true
		return req.C(), nil
	})

	_, err := svc.QueryUsage(context.Background(), 42)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "OPENAI_QUOTA_INVALID_PLATFORM", infraerrors.Reason(err))
	require.False(t, upstreamCalled)
}

func TestOpenAIQuotaRejectsAPIKeyAccount(t *testing.T) {
	repo := &openAIQuotaAccountRepoStub{account: &Account{
		ID:       43,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
	}}
	svc := NewOpenAIQuotaService(repo, nil, &OpenAITokenProvider{}, func(string) (*req.Client, error) {
		t.Fatal("API-key account must be rejected before an upstream client is built")
		return nil, nil
	})

	_, err := svc.QueryUsage(context.Background(), 43)

	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "OPENAI_QUOTA_INVALID_TYPE", infraerrors.Reason(err))
}

func TestParseOpenAIRateLimitResetCreditDetailsSanitizesPayload(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "credits", body: `{"credits":[{"id":"secret-id","expires_at":"2026-07-03T04:05:06Z"}]}`, want: []string{"2026-07-03T04:05:06Z"}},
		{name: "camel case", body: `{"rate_limit_reset_credits":[{"token":"secret-token","expiresAt":"2026-07-04T04:05:06Z"}]}`, want: []string{"2026-07-04T04:05:06Z"}},
		{name: "array", body: `[{"expires_at":"2026-07-05T04:05:06Z"},{"id":"omitted-without-expiry"}]`, want: []string{"2026-07-05T04:05:06Z"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOpenAIRateLimitResetCreditDetails([]byte(tt.body))
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))
			for i := range tt.want {
				require.Equal(t, tt.want[i], got[i].ExpiresAt)
			}
			encoded, err := json.Marshal(got)
			require.NoError(t, err)
			require.NotContains(t, string(encoded), "secret")
		})
	}
}

func TestBuildCodexQuotaHeaders(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"user_agent": "Codex Desktop/1.2.3",
		},
	}
	headers := buildCodexCommonHeaders("access-token", "chatgpt-account", account)

	require.Equal(t, "Bearer access-token", headers["authorization"])
	require.Equal(t, "chatgpt-account", headers["chatgpt-account-id"])
	require.Equal(t, "codex-1", headers["openai-beta"])
	require.Equal(t, "Codex Desktop", headers["originator"])
	require.Equal(t, "Codex Desktop/1.2.3", headers["user-agent"])
}

func TestBuildCodexQuotaHeadersFallsBackToPairedProjectIdentity(t *testing.T) {
	headers := buildCodexCommonHeaders("access-token", "chatgpt-account", &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	})

	want := make(http.Header)
	want.Set("originator", "Codex Desktop")
	want.Set("user-agent", codexCLIUserAgent)
	enforceCodexIdentityHeaders(want)
	require.Equal(t, want.Get("originator"), headers["originator"])
	require.Equal(t, want.Get("user-agent"), headers["user-agent"])
}

func TestOpenAIQuotaUpstreamStatusMapping(t *testing.T) {
	require.Equal(t, http.StatusUnauthorized, mapUpstreamStatus(http.StatusUnauthorized))
	require.Equal(t, http.StatusForbidden, mapUpstreamStatus(http.StatusForbidden))
	require.Equal(t, http.StatusTooManyRequests, mapUpstreamStatus(http.StatusTooManyRequests))
	require.Equal(t, http.StatusBadGateway, mapUpstreamStatus(http.StatusBadRequest))
	require.Equal(t, http.StatusBadGateway, mapUpstreamStatus(http.StatusServiceUnavailable))
}

func TestOpenAIQuotaPrepareUpstreamCallSupportsPersonalAccessToken(t *testing.T) {
	account := &Account{
		ID:       904,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":          OpenAIAuthModePersonalAccessToken,
			"access_token":       "pat-access-token",
			"chatgpt_account_id": "pat-account-id",
		},
	}
	repo := &openAIQuotaAccountRepoStub{account: account}
	service := NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, nil, nil), func(string) (*req.Client, error) {
		return req.C(), nil
	})

	accessToken, accountID, _, preparedAccount, err := service.prepareUpstreamCall(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, "pat-access-token", accessToken)
	require.Equal(t, "pat-account-id", accountID)
	require.Same(t, account, preparedAccount)
}

func TestOpenAIQuotaResetPassesStableRedeemRequestIDUpstream(t *testing.T) {
	const redeemID = "123e4567-e89b-42d3-a456-426614174000"
	account := &Account{
		ID:       905,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":          OpenAIAuthModePersonalAccessToken,
			"access_token":       "pat-access-token",
			"chatgpt_account_id": "pat-account-id",
		},
	}
	repo := &openAIQuotaAccountRepoStub{account: account}
	var upstreamRedeemID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var payload map[string]string
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		upstreamRedeemID = payload["redeem_request_id"]
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"ok","windows_reset":1}`))
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := func(string) (*req.Client, error) {
		client := req.C().WrapRoundTripFunc(func(roundTripper req.RoundTripper) req.RoundTripFunc {
			return func(request *req.Request) (*req.Response, error) {
				request.URL.Scheme = target.Scheme
				request.URL.Host = target.Host
				return roundTripper.RoundTrip(request)
			}
		})
		return client, nil
	}
	service := NewOpenAIQuotaService(repo, nil, NewOpenAITokenProvider(repo, nil, nil), factory)

	_, err = service.ResetCredit(context.Background(), account.ID, redeemID)

	require.NoError(t, err)
	require.Equal(t, redeemID, upstreamRedeemID)
}
