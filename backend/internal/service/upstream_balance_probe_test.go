//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProbeUpstreamBalance_CreditGrants(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Third-party hosts try /v1/usage and new-api first; force fallthrough to credit_grants.
		switch r.URL.Path {
		case "/v1/usage", "/api/usage/token", "/api/usage/token/", "/api/status":
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
		// Prefer path that reaches credit_grants after earlier probes miss.
		switch r.URL.Path {
		case "/v1/usage", "/api/usage/token", "/api/usage/token/", "/api/status":
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
		case "/v1/usage", "/api/usage/token", "/api/usage/token/", "/api/status":
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

func TestProbeUpstreamBalance_NewAPITokenUsage(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			http.Error(w, "404 page not found", http.StatusNotFound)
		case "/api/usage/token", "/api/usage/token/":
			require.Equal(t, "Bearer sk-tb", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    true,
				"message": "ok",
				"data": map[string]any{
					"object":          "token_usage",
					"name":            "plus",
					"total_granted":   1_000_000.0,
					"total_used":      250_000.0,
					"total_available": 750_000.0,
					"unlimited_quota": false,
					"expires_at":      0,
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	// base_url ends with /v1 like token-bits production accounts.
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-tb",
			"base_url": srv.URL + "/v1",
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceNewAPITokenUsage, result.Source)
	require.False(t, result.Unlimited)
	// 750000 / 500000 = $1.50
	require.InDelta(t, 1.5, result.BalanceUSD, 0.001)
}

func TestProbeUpstreamBalance_NewAPIUnlimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/usage":
			http.Error(w, "no", http.StatusNotFound)
		case "/api/usage/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": true,
				"data": map[string]any{
					"total_available": -122.0,
					"unlimited_quota": true,
					"object":          "token_usage",
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":  "sk-tb",
			"base_url": srv.URL + "/v1",
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.True(t, result.Unlimited)
	require.Equal(t, 0.0, result.BalanceUSD)
}

func TestOriginFromBaseURL(t *testing.T) {
	t.Parallel()
	require.Equal(t, "https://api.token-bits.com", originFromBaseURL("https://api.token-bits.com/v1"))
	require.Equal(t, "https://api.token-bits.com", originFromBaseURL("https://api.token-bits.com/v1/"))
	require.Equal(t, "https://api.token-bits.com", originFromBaseURL("https://api.token-bits.com"))
}

func TestNewAPIUserWalletCreds(t *testing.T) {
	t.Parallel()
	token, userID, ok := newAPIUserWalletCreds(nil)
	require.False(t, ok)
	require.Empty(t, token)
	require.Empty(t, userID)

	_, _, ok = newAPIUserWalletCreds(&Account{Credentials: map[string]any{
		credentialKeyNewAPIAccessToken: "tok",
		credentialKeyNewAPIUserID:      0,
	}})
	require.False(t, ok)

	token, userID, ok = newAPIUserWalletCreds(&Account{Credentials: map[string]any{
		credentialKeyNewAPIAccessToken: "  wallet-tok  ",
		credentialKeyNewAPIUserID:      "952",
	}})
	require.True(t, ok)
	require.Equal(t, "wallet-tok", token)
	require.Equal(t, "952", userID)
}

func TestProbeUpstreamBalance_NewAPIUserSelfPreferred(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			require.Equal(t, "Bearer "+walletTok, r.Header.Get("Authorization"))
			require.Equal(t, "952", r.Header.Get("New-Api-User"))
			require.NotEqual(t, "Bearer sk-tb", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"id":         952,
					"quota":      31_288_577.0,
					"used_quota": 124_894_510.0,
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
			})
		case "/v1/usage":
			_ = json.NewEncoder(w).Encode(map[string]any{"balance": 999999.0, "mode": "unrestricted"})
		case "/api/usage/token", "/api/usage/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": true,
				"data": map[string]any{"total_available": 0, "unlimited_quota": true},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      952,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceNewAPIUserSelf, result.Source)
	require.False(t, result.Unlimited)
	require.True(t, result.HasUsed)
	require.InDelta(t, 62.577154, result.BalanceUSD, 0.001)
	require.InDelta(t, 249.78902, result.UsedUSD, 0.001)
	require.True(t, result.HasWallet)
	require.False(t, result.HasSubscription)
	require.InDelta(t, 62.577154, result.WalletUSD, 0.001)
	require.NotContains(t, result.Error, walletTok)
}

func TestProbeUpstreamBalance_NewAPIWalletPlusSubscription(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	now := time.Now().UTC().Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			require.Equal(t, "Bearer "+walletTok, r.Header.Get("Authorization"))
			require.Equal(t, "201", r.Header.Get("New-Api-User"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"id":         201,
					"quota":      -611148.0,
					"used_quota": 2_450_321_817.0,
				},
			})
		case "/api/subscription/self":
			require.Equal(t, "Bearer "+walletTok, r.Header.Get("Authorization"))
			require.Equal(t, "201", r.Header.Get("New-Api-User"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"subscriptions": []map[string]any{
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 0,
							"status": "active", "end_time": now + 86400,
						}},
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 53_559_180,
							"status": "active", "end_time": now + 86400,
						}},
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 150_000,
							"status": "cancelled", "end_time": now + 86400,
						}},
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 0,
							"status": "active", "end_time": now - 10,
						}},
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 111_000_000,
							"status": "active", "end_time": now + 86400,
						}},
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 0,
							"status": "active", "end_time": 0,
						}},
					},
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      201,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceNewAPIWalletSubscription, result.Source)
	require.True(t, result.HasWallet)
	require.True(t, result.HasSubscription)
	require.InDelta(t, 0, result.WalletUSD, 0.001)
	require.InDelta(t, 336.88164, result.SubscriptionUSD, 0.001)
	require.InDelta(t, 336.88164, result.BalanceUSD, 0.001)
	require.False(t, result.Unlimited)
	require.NotContains(t, result.Error, walletTok)
	usage := &UsageInfo{}
	applyBalanceResultToUsage(usage, result)
	raw, err := json.Marshal(usage)
	require.NoError(t, err)
	require.NotContains(t, string(raw), walletTok)
	require.NotContains(t, string(raw), credentialKeyNewAPIAccessToken)
}

func TestProbeUpstreamBalance_NewAPISubscriptionEmptyKeepsWallet(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]any{"id": 952, "quota": 31_288_577.0, "used_quota": 0.0},
			})
		case "/api/subscription/self":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]any{"subscriptions": []any{}},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      952,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceNewAPIWalletSubscription, result.Source)
	require.True(t, result.HasWallet)
	require.True(t, result.HasSubscription)
	require.InDelta(t, 62.577154, result.BalanceUSD, 0.001)
	require.Equal(t, 0.0, result.SubscriptionUSD)
}

func TestApplyBalanceProbeToExtra_ClearsStaleSubscription(t *testing.T) {
	t.Parallel()
	account := &Account{Extra: map[string]any{
		extraKeyUpstreamBalanceSubscriptionUSD: 336.61,
		extraKeyUpstreamBalanceWalletUSD:       10.0,
	}}
	now := time.Now().UTC()
	updates := applyBalanceProbeToExtra(account, UpstreamBalanceResult{
		BalanceUSD: 62.57,
		HasWallet:  true,
		WalletUSD:  62.57,
		Source:     balanceSourceNewAPIUserSelf,
	}, now)
	require.Nil(t, updates[extraKeyUpstreamBalanceSubscriptionUSD])
	require.InDelta(t, 62.57, updates[extraKeyUpstreamBalanceWalletUSD], 0.001)
	require.Nil(t, account.Extra[extraKeyUpstreamBalanceSubscriptionUSD])
	payload, err := json.Marshal(updates)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "wallet-fixture-token")
	require.NotContains(t, string(payload), credentialKeyNewAPIAccessToken)
}

func TestProbeUpstreamBalance_NewAPISubscriptionWhenWalletFails(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	now := time.Now().UTC().Unix()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "message": "Unauthorized"})
		case "/api/subscription/self":
			require.Equal(t, "Bearer "+walletTok, r.Header.Get("Authorization"))
			require.Equal(t, "201", r.Header.Get("New-Api-User"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data": map[string]any{
					"subscriptions": []map[string]any{
						{"subscription": map[string]any{
							"amount_total": 111_000_000, "amount_used": 0,
							"status": "active", "end_time": now + 86400,
						}},
					},
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      201,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceNewAPIWalletSubscription, result.Source)
	require.False(t, result.HasWallet)
	require.True(t, result.HasSubscription)
	require.InDelta(t, 222.0, result.SubscriptionUSD, 0.001)
	require.InDelta(t, 222.0, result.BalanceUSD, 0.001)
	require.False(t, result.HasUsed)
	require.NotContains(t, result.Error, walletTok)
}

func TestProbeUpstreamBalance_OpenAIBilling1e8IsNotNewAPIWallet(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self", "/api/subscription/self", "/v1/usage", "/api/usage/token", "/api/usage/token/", "/v1/dashboard/billing/credit_grants":
			http.Error(w, "no", http.StatusNotFound)
		case "/v1/dashboard/billing/subscription":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"hard_limit_usd":     100_000_000.0,
				"has_payment_method": true,
			})
		case "/v1/dashboard/billing/usage":
			_ = json.NewEncoder(w).Encode(map[string]any{"total_usage": 0})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      201,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error, result.Error)
	require.Equal(t, balanceSourceSubscriptionUsage, result.Source)
	require.False(t, result.HasWallet)
	require.False(t, result.HasSubscription)
	require.Equal(t, 0.0, result.WalletUSD)
	require.Equal(t, 0.0, result.SubscriptionUSD)
	updates := applyBalanceProbeToExtra(account, result, time.Now().UTC())
	require.Nil(t, updates[extraKeyUpstreamBalanceWalletUSD])
	require.Nil(t, updates[extraKeyUpstreamBalanceSubscriptionUSD])
	require.NotContains(t, result.Error, walletTok)
}

func TestProbeUpstreamBalance_NewAPIUserSelfIDMismatchFallsBack(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]any{"id": 1, "quota": 500000.0},
			})
		case "/v1/usage":
			http.Error(w, "no", http.StatusNotFound)
		case "/api/usage/token", "/api/usage/token/":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": true,
				"data": map[string]any{
					"total_available": 750_000.0,
					"total_used":      250_000.0,
					"unlimited_quota": false,
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      952,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Equal(t, balanceSourceNewAPITokenUsage, result.Source)
	require.InDelta(t, 1.5, result.BalanceUSD, 0.001)
	require.NotContains(t, result.Error, walletTok)
	_, _, _, ok, errMsg := fetchNewAPIUserSelfBalance(
		context.Background(), srv.Client(), account, walletTok, "952", srv.URL+"/v1",
	)
	require.False(t, ok)
	require.Contains(t, errMsg, "id mismatch")
	require.NotContains(t, errMsg, walletTok)
}

func TestProbeUpstreamBalance_NewAPIUserSelfUnauthorizedFallsBack(t *testing.T) {
	t.Parallel()
	const walletTok = "wallet-fixture-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self":
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"message": "Unauthorized, invalid access token",
			})
		case "/v1/usage":
			http.Error(w, "no", http.StatusNotFound)
		case "/api/usage/token", "/api/usage/token/":
			require.Equal(t, "Bearer sk-tb", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": true,
				"data": map[string]any{
					"total_available": 0,
					"unlimited_quota": true,
					"total_used":      500000.0,
				},
			})
		case "/api/status":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"quota_per_unit": 500000.0},
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
			"api_key":                      "sk-tb",
			"base_url":                     srv.URL + "/v1",
			credentialKeyNewAPIAccessToken: walletTok,
			credentialKeyNewAPIUserID:      952,
		},
	}
	result := ProbeUpstreamBalance(context.Background(), account)
	require.Empty(t, result.Error)
	require.Equal(t, balanceSourceNewAPITokenUsage, result.Source)
	require.True(t, result.Unlimited)
	require.Equal(t, 0.0, result.BalanceUSD)
	require.NotContains(t, result.Error, walletTok)
}
