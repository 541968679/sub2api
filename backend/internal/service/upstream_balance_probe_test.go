//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeUpstreamBalance_CreditGrants(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Third-party hosts try /v1/usage first; force fallthrough to credit_grants.
		if r.URL.Path == "/v1/usage" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		require.Equal(t, "/v1/dashboard/billing/credit_grants", r.URL.Path)
		require.Equal(t, "Bearer sk-test", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_available": 42.5,
			"total_granted":   100,
			"total_used":      57.5,
		})
	}))
	t.Cleanup(srv.Close)

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": srv.URL,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.Equal(t, balanceSourceCreditGrants, result.Source)
	require.InDelta(t, 42.5, result.BalanceUSD, 0.001)
}

func TestProbeUpstreamBalance_AnthropicAuthHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prefer path that reaches credit_grants after /v1/usage miss.
		if r.URL.Path == "/v1/usage" {
			http.Error(w, "no", http.StatusNotFound)
			return
		}
		require.Equal(t, "sk-ant", r.Header.Get("x-api-key"))
		require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		_ = json.NewEncoder(w).Encode(map[string]any{"total_available": 9})
	}))
	t.Cleanup(srv.Close)

	account := &Account{
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-ant",
			"base_url": srv.URL,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.InDelta(t, 9.0, result.BalanceUSD, 0.001)
}

func TestProbeUpstreamBalance_SubscriptionFallback(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			http.Error(w, "no", http.StatusNotFound)
		case "/v1/dashboard/billing/credit_grants":
			http.Error(w, "no", http.StatusForbidden)
		case "/v1/dashboard/billing/subscription":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"hard_limit_usd":     100.0,
				"has_payment_method": true,
			})
		case "/v1/dashboard/billing/usage":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_usage": 2500.0, // $25.00
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": srv.URL,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.Equal(t, balanceSourceSubscriptionUsage, result.Source)
	require.InDelta(t, 75.0, result.BalanceUSD, 0.01)
}

func TestSupportsUpstreamBalanceProbe(t *testing.T) {
	t.Parallel()
	require.True(t, SupportsUpstreamBalanceProbe(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}))
	require.True(t, SupportsUpstreamBalanceProbe(&Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}))
	require.False(t, SupportsUpstreamBalanceProbe(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.False(t, SupportsUpstreamBalanceProbe(&Account{Platform: PlatformGemini, Type: AccountTypeAPIKey}))
}

func TestProbeUpstreamBalance_Sub2APIUsage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			require.Equal(t, "Bearer sk-zc", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"mode":      "unrestricted",
				"isValid":   true,
				"balance":   98003.995,
				"remaining": 98003.995,
				"unit":      "USD",
			})
		case "/v1/dashboard/billing/credit_grants":
			http.Error(w, "404 page not found", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-zc",
			"base_url": srv.URL,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.Equal(t, balanceSourceSub2APIUsage, result.Source)
	require.InDelta(t, 98003.995, result.BalanceUSD, 0.001)
}

func TestIsOfficialOpenAIOrAnthropicHost(t *testing.T) {
	t.Parallel()
	require.True(t, isOfficialOpenAIOrAnthropicHost("https://api.openai.com"))
	require.True(t, isOfficialOpenAIOrAnthropicHost("https://api.anthropic.com/v1"))
	require.False(t, isOfficialOpenAIOrAnthropicHost("https://zerocode.kaynlab.com"))
}
