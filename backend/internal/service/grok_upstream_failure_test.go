//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrokRetryableOnSameAccount_CapacityAndRateLimit(t *testing.T) {
	account := &Account{ID: 9105, Platform: PlatformGrok, Type: AccountTypeOAuth}
	require.True(t, grokRetryableOnSameAccount(account, http.StatusTooManyRequests,
		[]byte(`{"error":{"message":"The model is currently at capacity due to high demand"}}`)))
	require.False(t, grokRetryableOnSameAccount(account, http.StatusTooManyRequests,
		[]byte(`{"error":{"message":"rate limit exceeded"}}`)))
	require.False(t, grokRetryableOnSameAccount(account, http.StatusPaymentRequired,
		[]byte(`{"error":{"message":"You have run out of credits or need a Grok subscription"}}`)))
	poolAccount := &Account{ID: 9108, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Credentials: map[string]any{"pool_mode": true}}
	require.False(t, grokRetryableOnSameAccount(poolAccount, http.StatusTooManyRequests,
		[]byte(`{"error":{"code":"subscription:free-usage-exhausted"}}`)),
		"pool free-usage must fail over instead of retrying the exhausted account")
	require.False(t, grokRetryableOnSameAccount(account, http.StatusBadRequest,
		[]byte(`{"error":{"message":"capacity field is invalid"}}`)))
	nonGrok := &Account{ID: 9106, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	require.False(t, grokRetryableOnSameAccount(nonGrok, http.StatusTooManyRequests,
		[]byte(`{"error":{"message":"model at capacity"}}`)))
}
