package service

import (
	"net/http"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

func grokUpstreamErrorCorpus(responseBody []byte) (low, code string) {
	raw := strings.ToLower(strings.TrimSpace(string(responseBody)))
	code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.code").String()))
	if code == "" {
		code = strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "code").String()))
	}
	return raw, code
}

func isGrokFreeUsageExhausted(low, code string) bool {
	if strings.Contains(code, "subscription:free-usage-exhausted") ||
		strings.Contains(code, "free-usage-exhausted") ||
		strings.Contains(code, "free_usage_exhausted") ||
		strings.Contains(code, "free-usage") ||
		strings.Contains(code, "free_usage") {
		return true
	}
	return strings.Contains(low, "subscription:free-usage-exhausted") ||
		strings.Contains(low, "free-usage-exhausted") ||
		strings.Contains(low, "free_usage_exhausted")
}

func isGrokBillingQuota(statusCode int, low string) bool {
	if statusCode == http.StatusPaymentRequired {
		return true
	}
	return strings.Contains(low, "run out of credits") ||
		strings.Contains(low, "out of credits") ||
		strings.Contains(low, "need a grok subscription") ||
		strings.Contains(low, "need grok subscription") ||
		strings.Contains(low, "insufficient_quota")
}

func isGrokModelCapacityText(low string) bool {
	return strings.Contains(low, "capacity") ||
		strings.Contains(low, "overloaded") ||
		strings.Contains(low, "server_busy") ||
		strings.Contains(low, "too many concurrent") ||
		strings.Contains(low, "engine_overloaded")
}

// grokRetryableOnSameAccount marks transient 429 classes for the shared
// failover loop. Capacity is request/model pressure, not evidence that the
// credential is invalid, so a bounded retry on the same account is preferable
// before switching. Free-usage and billing exhaustion skip same-account retry.
func grokRetryableOnSameAccount(account *Account, statusCode int, responseBody []byte) bool {
	if account == nil || !account.IsGrok() {
		return false
	}
	low, code := grokUpstreamErrorCorpus(responseBody)
	if isGrokFreeUsageExhausted(low, code) || isGrokBillingQuota(statusCode, low) {
		return false
	}
	if statusCode == http.StatusTooManyRequests && isGrokModelCapacityText(low) {
		return true
	}
	return account.IsPoolMode() && account.IsPoolModeRetryableStatus(statusCode)
}

func grokSameAccountRetryMetadata(account *Account, statusCode int, responseBody []byte) (bool, time.Duration, time.Time, int) {
	if !grokRetryableOnSameAccount(account, statusCode, responseBody) {
		return false, 0, time.Time{}, 0
	}
	low, _ := grokUpstreamErrorCorpus(responseBody)
	if statusCode == http.StatusTooManyRequests && isGrokModelCapacityText(low) {
		return true, 500 * time.Millisecond, time.Now().Add(30 * time.Second), 1
	}
	return true, 0, time.Time{}, 0
}

func newGrokUpstreamFailoverError(account *Account, statusCode int, headers http.Header, body []byte) *UpstreamFailoverError {
	retryable, delay, deadline, retryMax := grokSameAccountRetryMetadata(account, statusCode, body)
	err := &UpstreamFailoverError{
		StatusCode:               statusCode,
		ResponseBody:             body,
		RetryableOnSameAccount:   retryable,
		RequestScopedTransient:   retryable && statusCode == http.StatusTooManyRequests,
		SameAccountRetryDelay:    delay,
		SameAccountRetryDeadline: deadline,
		SameAccountRetryMax:      retryMax,
	}
	if headers != nil {
		err.ResponseHeaders = headers.Clone()
	}
	return err
}

func overlayGrokSameAccountRetry(err *UpstreamFailoverError, account *Account, statusCode int, body []byte) {
	if err == nil || account == nil || !account.IsGrok() {
		return
	}
	retryable, delay, deadline, retryMax := grokSameAccountRetryMetadata(account, statusCode, body)
	err.RetryableOnSameAccount = retryable
	if retryable && statusCode == http.StatusTooManyRequests {
		err.RequestScopedTransient = true
	}
	err.SameAccountRetryDelay = delay
	err.SameAccountRetryDeadline = deadline
	err.SameAccountRetryMax = retryMax
}
