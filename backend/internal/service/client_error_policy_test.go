package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveUpstreamClientError_FactoryMatchesLegacyMap(t *testing.T) {
	cases := []struct {
		status       int
		gatewayType  string
		openAIType   string
		geminiStatus int
		message      string
	}{
		{status: 401, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream authentication failed, please contact administrator"},
		{status: 403, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream access forbidden, please contact administrator"},
		{status: 429, gatewayType: "rate_limit_error", openAIType: "rate_limit_error", geminiStatus: http.StatusTooManyRequests, message: "Upstream rate limit exceeded, please retry later"},
		{status: 529, gatewayType: "overloaded_error", openAIType: "upstream_error", geminiStatus: http.StatusServiceUnavailable, message: "Upstream service overloaded, please retry later"},
		{status: 500, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream service temporarily unavailable"},
		{status: 502, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream service temporarily unavailable"},
		{status: 503, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream service temporarily unavailable"},
		{status: 504, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream service temporarily unavailable"},
		{status: 413, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream request failed"},
		{status: 400, gatewayType: "upstream_error", openAIType: "upstream_error", geminiStatus: http.StatusBadGateway, message: "Upstream request failed"},
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			status, errType, msg := ResolveUpstreamClientError(ctx, ClientErrorProfileGateway, "", tc.status, nil)
			require.Equal(t, tc.geminiStatus, status)
			require.Equal(t, tc.gatewayType, errType)
			require.Equal(t, tc.message, msg)

			status, errType, msg = ResolveUpstreamClientError(ctx, ClientErrorProfileOpenAI, "", tc.status, nil)
			require.Equal(t, tc.geminiStatus, status)
			require.Equal(t, tc.openAIType, errType)
			require.Equal(t, tc.message, msg)

			status, _, msg = ResolveUpstreamClientError(ctx, ClientErrorProfileGemini, "", tc.status, nil)
			require.Equal(t, tc.geminiStatus, status)
			require.Equal(t, tc.message, msg)
		})
	}
}

func TestResolveUpstreamClientError_DisabledBuiltinFallsThrough(t *testing.T) {
	wl := DefaultScheduleErrorWhitelist()
	wl.Replacements = FactoryErrorReplacements()
	for i := range wl.Replacements {
		if wl.Replacements[i].ID == "status_429" {
			wl.Replacements[i].Enabled = false
		}
	}
	restore := swapWhitelist(wl)
	defer restore()

	status, errType, msg := ResolveUpstreamClientError(context.Background(), ClientErrorProfileGateway, "", http.StatusTooManyRequests, nil)
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, "upstream_error", errType)
	require.Equal(t, "Upstream request failed", msg)
}

func TestResolveUpstreamClientError_CustomAndFamilyOverride(t *testing.T) {
	exclude := false
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		ID:              "c_test",
		Enabled:         true,
		MessageContains: "quota exhausted",
		ReplaceWith:     ClientErrorTemplate429,
		ScheduleExclude: &exclude,
	}}
	wl.FamilyReplace = map[string]string{
		ScheduleErrorFamilyClientContextTooLong: ClientErrorTemplate413,
	}
	restore := swapWhitelist(wl)
	defer restore()

	status, errType, msg := ResolveUpstreamClientError(context.Background(), ClientErrorProfileOpenAI, "", 500, []byte(`{"error":{"message":"quota exhausted"}}`))
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_error", errType)
	require.Equal(t, "Upstream rate limit exceeded, please retry later", msg)

	status, errType, msg = ResolveUpstreamClientError(context.Background(), ClientErrorProfileGateway, "", 400, []byte(`{"error":{"message":"prompt is too long"}}`))
	require.Equal(t, http.StatusRequestEntityTooLarge, status)
	require.Equal(t, "invalid_request_error", errType)
	require.Contains(t, msg, "Prompt is too long")

	require.False(t, IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{
		Status:  500,
		Message: "quota exhausted",
	}, wl))
}

func TestResolveUpstreamClientError_WaitTimeoutIgnoresReplacement(t *testing.T) {
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled:         true,
		MessageContains: OpenAIHeaderWaitTimeoutMarker,
		ReplaceWith:     ClientErrorTemplate413,
	}}
	restore := swapWhitelist(wl)
	defer restore()

	body := []byte(`{"error":{"message":"` + OpenAIHeaderWaitTimeoutMarker + ` waited_ms=10"}}`)
	_, _, msg := ResolveUpstreamClientError(context.Background(), ClientErrorProfileOpenAI, "", 502, body)
	require.Equal(t, "Upstream service temporarily unavailable", msg)
	require.NotContains(t, msg, OpenAIHeaderWaitTimeoutMarker)
}

func TestScheduleExclude_ReplacementAndLegacyCustom(t *testing.T) {
	wl := DefaultScheduleErrorWhitelist()
	wl.Replacements = FactoryErrorReplacements()
	for i := range wl.Replacements {
		if wl.Replacements[i].ID == "status_5xx" {
			wl.Replacements[i].ScheduleExclude = true
		}
	}
	require.False(t, IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{
		Status:  502,
		Message: "Upstream request failed",
	}, wl))
	require.True(t, IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{
		Status:  500,
		Message: "boom",
	}, wl))

	exclude := false
	legacy := DefaultScheduleErrorWhitelist()
	legacy.Custom = []ScheduleErrorCustomRule{{
		Enabled:         true,
		MessageContains: "weird upstream",
	}}
	require.True(t, IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{Message: "weird upstream"}, legacy))
	legacy.Custom[0].ScheduleExclude = &exclude
	require.False(t, IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{Message: "weird upstream"}, legacy))

	sql := SQLScheduleQualityExcludedPredicateWith("e.", wl)
	require.Contains(t, sql, "IN (500, 502, 503, 504)")
	require.Contains(t, sql, "upstream request failed")
	plain := SQLScheduleQualityExcludedPredicateWith("e.", DefaultScheduleErrorWhitelist())
	require.NotContains(t, plain, "IN (500, 502, 503, 504)")
}

func TestNormalizeErrorReplacements_LockedCatchAllStaysOn(t *testing.T) {
	in := FactoryErrorReplacements()
	for i := range in {
		if in[i].Locked {
			in[i].Enabled = false
			in[i].ScheduleExclude = true
			in[i].Template = ClientErrorTemplate429
		}
	}
	out := NormalizeErrorReplacements(in)
	var catchAll ErrorReplacementRule
	for _, rule := range out {
		if rule.Locked {
			catchAll = rule
		}
	}
	require.True(t, catchAll.Enabled)
	require.False(t, catchAll.ScheduleExclude)
	require.Equal(t, ClientErrorTemplate429, catchAll.Template)
	require.NotContains(t, strings.Join(replacementIDs(out), ","), "status_made_up")
}

func replacementIDs(rules []ErrorReplacementRule) []string {
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	return ids
}

func swapWhitelist(wl ScheduleErrorWhitelist) func() {
	previous := scheduleErrorWLCache
	scheduleErrorWLCache = &scheduleErrorWhitelistCache{}
	RegisterScheduleErrorWhitelistSource(func(context.Context) ScheduleErrorWhitelist {
		return wl
	})
	return func() {
		scheduleErrorWLCache = previous
	}
}
