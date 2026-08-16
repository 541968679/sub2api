//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestCheckErrorPolicy — 6 table-driven cases for the pure logic function
// ---------------------------------------------------------------------------

func TestCheckErrorPolicy(t *testing.T) {
	tests := []struct {
		name       string
		account    *Account
		statusCode int
		body       []byte
		expected   ErrorPolicyResult
	}{
		{
			name: "no_policy_oauth_returns_none",
			account: &Account{
				ID:       1,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				// no custom error codes, no temp rules
			},
			statusCode: 500,
			body:       []byte(`"error"`),
			expected:   ErrorPolicyNone,
		},
		{
			name: "custom_error_codes_hit_returns_matched",
			account: &Account{
				ID:       2,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429), float64(500)},
				},
			},
			statusCode: 500,
			body:       []byte(`"error"`),
			expected:   ErrorPolicyMatched,
		},
		{
			name: "custom_error_codes_miss_returns_skipped",
			account: &Account{
				ID:       3,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429), float64(500)},
				},
			},
			statusCode: 503,
			body:       []byte(`"error"`),
			expected:   ErrorPolicySkipped,
		},
		{
			name: "temp_unschedulable_hit_returns_temp_unscheduled",
			account: &Account{
				ID:       4,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`overloaded service`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			name: "temp_unschedulable_401_first_hit_returns_temp_unscheduled",
			account: &Account{
				ID:       14,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(401),
							"keywords":         []any{"unauthorized"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			// Antigravity 401 不走升级逻辑（由 applyErrorPolicy 的 temp_unschedulable_rules 自行控制），
			// second hit 仍然返回 TempUnscheduled。
			name: "temp_unschedulable_401_second_hit_antigravity_stays_temp",
			account: &Account{
				ID:                      15,
				Type:                    AccountTypeOAuth,
				Platform:                PlatformAntigravity,
				TempUnschedulableReason: `{"status_code":401,"until_unix":1735689600}`,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(401),
							"keywords":         []any{"unauthorized"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyTempUnscheduled,
		},
		{
			name: "temp_unschedulable_body_miss_returns_none",
			account: &Account{
				ID:       5,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`random msg`),
			expected:   ErrorPolicyNone,
		},
		{
			name: "custom_error_codes_override_temp_unschedulable",
			account: &Account{
				ID:       6,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(503)},
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
							"description":      "overloaded rule",
						},
					},
				},
			},
			statusCode: 503,
			body:       []byte(`overloaded`),
			expected:   ErrorPolicyMatched, // custom codes take precedence
		},
		{
			name: "pool_mode_custom_error_codes_hit_returns_matched",
			account: &Account{
				ID:       7,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":                  true,
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(401), float64(403)},
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicyMatched,
		},
		{
			name: "pool_mode_without_custom_error_codes_returns_skipped",
			account: &Account{
				ID:       8,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode": true,
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicySkipped,
		},
		{
			name: "pool_mode_hard_eviction_r5_returns_none",
			account: &Account{
				ID:       9,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":               true,
					"pool_mode_hard_eviction": true,
				},
			},
			statusCode: 402,
			body:       []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			expected:   ErrorPolicyNone,
		},
		{
			name: "pool_mode_hard_eviction_ordinary_401_still_skipped",
			account: &Account{
				ID:       16,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":               true,
					"pool_mode_hard_eviction": true,
				},
			},
			statusCode: 401,
			body:       []byte(`unauthorized`),
			expected:   ErrorPolicySkipped,
		},
		{
			name: "custom_error_codes_win_over_hard_eviction_402",
			account: &Account{
				ID:       17,
				Type:     AccountTypeAPIKey,
				Platform: PlatformOpenAI,
				Credentials: map[string]any{
					"pool_mode":                  true,
					"pool_mode_hard_eviction":    true,
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(401)},
				},
			},
			statusCode: 402,
			body:       []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			expected:   ErrorPolicySkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)

			result := svc.CheckErrorPolicy(context.Background(), tt.account, tt.statusCode, tt.body)
			require.Equal(t, tt.expected, result, "unexpected ErrorPolicyResult")
		})
	}
}

func TestHandleUpstreamError_PoolModeCustomErrorCodesOverride(t *testing.T) {
	t.Run("pool_mode_without_custom_error_codes_still_skips", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       30,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"pool_mode": true,
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.False(t, shouldDisable)
		require.Equal(t, 0, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})

	t.Run("pool_mode_with_custom_error_codes_uses_local_error_policy", func(t *testing.T) {
		repo := &errorPolicyRepoStub{}
		svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
		account := &Account{
			ID:       31,
			Type:     AccountTypeAPIKey,
			Platform: PlatformOpenAI,
			Credentials: map[string]any{
				"pool_mode":                  true,
				"custom_error_codes_enabled": true,
				"custom_error_codes":         []any{float64(401)},
			},
		}

		shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte("unauthorized"))

		require.True(t, shouldDisable)
		require.Equal(t, 1, repo.setErrCalls)
		require.Equal(t, 0, repo.tempCalls)
	})
}

func TestIsPoolModeHardMaintenanceError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     []byte
		expected bool
	}{
		{name: "http_402_always", status: 402, body: []byte(`{}`), expected: true},
		{name: "http_400_credit_balance", status: 400, body: []byte(`{"error":{"message":"Your credit balance is too low"}}`), expected: true},
		{name: "http_400_org_disabled", status: 400, body: []byte(`{"error":{"message":"This organization has been disabled"}}`), expected: true},
		{name: "http_400_kyc", status: 400, body: []byte(`{"error":{"message":"Identity verification is required"}}`), expected: true},
		{name: "http_400_plain_bad_request", status: 400, body: []byte(`{"error":{"message":"invalid request"}}`), expected: false},
		{name: "top_level_insufficient_balance_code", status: 403, body: []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`), expected: true},
		{name: "error_code_insufficient_balance", status: 403, body: []byte(`{"error":{"code":"INSUFFICIENT_BALANCE","message":"no credits"}}`), expected: true},
		{name: "text_insufficient_account_balance", status: 403, body: []byte(`{"error":{"message":"Insufficient account balance"}}`), expected: true},
		{name: "http_429_insufficient_quota", status: 429, body: []byte(`{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`), expected: true},
		{name: "http_429_usage_limit_exceeded", status: 429, body: []byte(`{"code":"USAGE_LIMIT_EXCEEDED","message":"daily limit"}`), expected: true},
		{name: "http_429_api_key_quota_exhausted", status: 429, body: []byte(`{"code":"API_KEY_QUOTA_EXHAUSTED","message":"API key 额度已用完"}`), expected: true},
		{name: "http_429_quota_zh_text", status: 429, body: []byte(`{"message":"额度已用完"}`), expected: true},
		{name: "http_429_ordinary_rate_limit", status: 429, body: []byte(`{"error":{"code":"rate_limit_exceeded","message":"Rate limit reached"}}`), expected: false},
		{name: "api_key_expired", status: 403, body: []byte(`{"code":"API_KEY_EXPIRED","message":"API key 已过期"}`), expected: true},
		{name: "user_inactive", status: 401, body: []byte(`{"code":"USER_INACTIVE","message":"User account is not active"}`), expected: true},
		{name: "subscription_not_found", status: 403, body: []byte(`{"code":"SUBSCRIPTION_NOT_FOUND","message":"No active subscription"}`), expected: true},
		{name: "subscription_invalid", status: 403, body: []byte(`{"code":"SUBSCRIPTION_INVALID","message":"subscription invalid"}`), expected: true},
		{name: "ordinary_401", status: 401, body: []byte(`unauthorized`), expected: false},
		{name: "ordinary_403", status: 403, body: []byte(`{"error":{"message":"forbidden"}}`), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, isPoolModeHardMaintenanceError(tt.status, tt.body))
		})
	}
}

func poolModeHardEvictionAccount(id int64, hardEviction bool) *Account {
	creds := map[string]any{"pool_mode": true}
	if hardEviction {
		creds["pool_mode_hard_eviction"] = true
	}
	return &Account{
		ID:          id,
		Type:        AccountTypeAPIKey,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: creds,
	}
}

func applySetErrorToAccount(account *Account, repo *errorPolicyRepoStub) {
	account.Status = StatusError
	account.ErrorMessage = repo.lastErrorMsg
}

func TestHandleUpstreamError_PoolMode(t *testing.T) {
	type want struct {
		shouldDisable bool
		setErrCalls   int
		tempCalls     int
		schedulable   bool
	}

	tests := []struct {
		name         string
		hardEviction bool
		customCodes  []any
		status       int
		body         []byte
		want         want
	}{
		{
			name:   "ac1_legacy_pool_402",
			status: 402,
			body:   []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			want:   want{schedulable: true},
		},
		{
			name:   "ac1_legacy_pool_403_insufficient_balance",
			status: 403,
			body:   []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			want:   want{schedulable: true},
		},
		{
			name:   "ac1_legacy_pool_429_insufficient_quota",
			status: 429,
			body:   []byte(`{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`),
			want:   want{schedulable: true},
		},
		{
			name:   "ac1_legacy_pool_user_inactive",
			status: 401,
			body:   []byte(`{"code":"USER_INACTIVE","message":"User account is not active"}`),
			want:   want{schedulable: true},
		},
		{
			name:         "ac2_hard_eviction_402",
			hardEviction: true,
			status:       402,
			body:         []byte(`{"message":"Payment required"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac3_hard_eviction_400_credit_balance",
			hardEviction: true,
			status:       400,
			body:         []byte(`{"error":{"message":"Your credit balance is too low"}}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac3_hard_eviction_400_org_disabled",
			hardEviction: true,
			status:       400,
			body:         []byte(`{"error":{"message":"This organization has been disabled"}}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac3_hard_eviction_400_kyc",
			hardEviction: true,
			status:       400,
			body:         []byte(`{"error":{"message":"Identity verification is required"}}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac4_hard_eviction_403_insufficient_balance_code",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac4_hard_eviction_403_insufficient_account_balance_text",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"error":{"message":"Insufficient account balance"}}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac5_hard_eviction_429_insufficient_quota",
			hardEviction: true,
			status:       429,
			body:         []byte(`{"error":{"code":"insufficient_quota","message":"You exceeded your current quota"}}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac5_hard_eviction_429_usage_limit_exceeded",
			hardEviction: true,
			status:       429,
			body:         []byte(`{"code":"USAGE_LIMIT_EXCEEDED","message":"daily limit"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac5_hard_eviction_429_api_key_quota_exhausted",
			hardEviction: true,
			status:       429,
			body:         []byte(`{"code":"API_KEY_QUOTA_EXHAUSTED","message":"API key 额度已用完"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac5_hard_eviction_429_quota_zh",
			hardEviction: true,
			status:       429,
			body:         []byte(`{"message":"额度已用完"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac6_hard_eviction_api_key_expired",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"code":"API_KEY_EXPIRED","message":"API key 已过期"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac6_hard_eviction_user_inactive",
			hardEviction: true,
			status:       401,
			body:         []byte(`{"code":"USER_INACTIVE","message":"User account is not active"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac6_hard_eviction_subscription_not_found",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"code":"SUBSCRIPTION_NOT_FOUND","message":"No active subscription"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac6_hard_eviction_subscription_invalid",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"code":"SUBSCRIPTION_INVALID","message":"subscription invalid"}`),
			want:         want{shouldDisable: true, setErrCalls: 1},
		},
		{
			name:         "ac7_ordinary_401_not_marked",
			hardEviction: true,
			status:       401,
			body:         []byte(`unauthorized`),
			want:         want{schedulable: true},
		},
		{
			name:         "ac7_ordinary_403_not_marked",
			hardEviction: true,
			status:       403,
			body:         []byte(`{"error":{"message":"forbidden"}}`),
			want:         want{schedulable: true},
		},
		{
			name:         "ac7_ordinary_429_rate_limit_not_marked",
			hardEviction: true,
			status:       429,
			body:         []byte(`{"error":{"code":"rate_limit_exceeded","message":"Rate limit reached"}}`),
			want:         want{schedulable: true},
		},
		{
			name:         "ac8_hard_eviction_off_returns_to_legacy",
			hardEviction: false,
			status:       402,
			body:         []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			want:         want{schedulable: true},
		},
		{
			name:         "ac9_custom_codes_whitelist_excludes_402",
			hardEviction: true,
			customCodes:  []any{float64(401)},
			status:       402,
			body:         []byte(`{"code":"INSUFFICIENT_BALANCE","message":"Insufficient account balance"}`),
			want:         want{schedulable: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			account := poolModeHardEvictionAccount(40, tt.hardEviction)
			if tt.customCodes != nil {
				account.Credentials["custom_error_codes_enabled"] = true
				account.Credentials["custom_error_codes"] = tt.customCodes
			}

			shouldDisable := svc.HandleUpstreamError(context.Background(), account, tt.status, http.Header{}, tt.body)

			require.Equal(t, tt.want.shouldDisable, shouldDisable)
			require.Equal(t, tt.want.setErrCalls, repo.setErrCalls)
			require.Equal(t, tt.want.tempCalls, repo.tempCalls)
			if repo.setErrCalls > 0 {
				applySetErrorToAccount(account, repo)
				require.NotEmpty(t, repo.lastErrorMsg)
			}
			require.Equal(t, tt.want.schedulable, account.IsSchedulable())
		})
	}
}

// ---------------------------------------------------------------------------
// TestApplyErrorPolicy — 4 table-driven cases for the wrapper method
// ---------------------------------------------------------------------------

func TestApplyErrorPolicy(t *testing.T) {
	tests := []struct {
		name              string
		account           *Account
		statusCode        int
		body              []byte
		expectedHandled   bool
		expectedStatus    int  // expected outStatus
		expectedSwitchErr bool // expect *AntigravityAccountSwitchError
		handleErrorCalls  int
	}{
		{
			name: "none_not_handled",
			account: &Account{
				ID:       10,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  false,
			expectedStatus:   500, // passthrough
			handleErrorCalls: 0,
		},
		{
			name: "skipped_handled_no_handleError",
			account: &Account{
				ID:       11,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(429)},
				},
			},
			statusCode:       500, // not in custom codes
			body:             []byte(`"error"`),
			expectedHandled:  true,
			expectedStatus:   http.StatusInternalServerError, // skipped → 500
			handleErrorCalls: 0,
		},
		{
			name: "matched_handled_calls_handleError",
			account: &Account{
				ID:       12,
				Type:     AccountTypeAPIKey,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"custom_error_codes_enabled": true,
					"custom_error_codes":         []any{float64(500)},
				},
			},
			statusCode:       500,
			body:             []byte(`"error"`),
			expectedHandled:  true,
			expectedStatus:   500, // matched → original status
			handleErrorCalls: 1,
		},
		{
			name: "temp_unscheduled_returns_switch_error",
			account: &Account{
				ID:       13,
				Type:     AccountTypeOAuth,
				Platform: PlatformAntigravity,
				Credentials: map[string]any{
					"temp_unschedulable_enabled": true,
					"temp_unschedulable_rules": []any{
						map[string]any{
							"error_code":       float64(503),
							"keywords":         []any{"overloaded"},
							"duration_minutes": float64(10),
						},
					},
				},
			},
			statusCode:        503,
			body:              []byte(`overloaded`),
			expectedHandled:   true,
			expectedStatus:    503, // temp_unscheduled → original status
			expectedSwitchErr: true,
			handleErrorCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &errorPolicyRepoStub{}
			rlSvc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
			svc := &AntigravityGatewayService{
				rateLimitService: rlSvc,
			}

			var handleErrorCount int
			p := antigravityRetryLoopParams{
				ctx:     context.Background(),
				prefix:  "[test]",
				account: tt.account,
				handleError: func(ctx context.Context, prefix string, account *Account, statusCode int, headers http.Header, body []byte, requestedModel string, groupID int64, sessionHash string, isStickySession bool) *handleModelRateLimitResult {
					handleErrorCount++
					return nil
				},
				isStickySession: true,
			}

			handled, outStatus, retErr := svc.applyErrorPolicy(p, tt.statusCode, http.Header{}, tt.body)

			require.Equal(t, tt.expectedHandled, handled, "handled mismatch")
			require.Equal(t, tt.expectedStatus, outStatus, "outStatus mismatch")
			require.Equal(t, tt.handleErrorCalls, handleErrorCount, "handleError call count mismatch")

			if tt.expectedSwitchErr {
				var switchErr *AntigravityAccountSwitchError
				require.ErrorAs(t, retErr, &switchErr)
				require.Equal(t, tt.account.ID, switchErr.OriginalAccountID)
			} else {
				require.NoError(t, retErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// errorPolicyRepoStub — minimal AccountRepository stub for error policy tests
// ---------------------------------------------------------------------------

type errorPolicyRepoStub struct {
	mockAccountRepoForGemini
	tempCalls    int
	setErrCalls  int
	lastErrorMsg string
}

func (r *errorPolicyRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	return nil
}

func (r *errorPolicyRepoStub) SetError(ctx context.Context, id int64, errorMsg string) error {
	r.setErrCalls++
	r.lastErrorMsg = errorMsg
	return nil
}
