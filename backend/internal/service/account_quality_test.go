package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountQualityStats_EmptyWindow(t *testing.T) {
	stats := BuildAccountQualityStats(0, 0, TTFTAggregate{})
	require.Equal(t, AccountQualityWindowSeconds, stats.WindowSeconds)
	require.Equal(t, int64(0), stats.SuccessCount)
	require.Equal(t, int64(0), stats.ErrorCount)
	require.Nil(t, stats.SuccessRate)
	require.Nil(t, stats.ErrorRate)
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
	require.Nil(t, stats.P95TTFTMs)
	require.Nil(t, stats.MaxTTFTMs)
	require.Equal(t, int64(0), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_SuccessAndError(t *testing.T) {
	avg := 420.4
	p50 := 300.0
	p95 := 900.2
	max := 5000.0
	stats := BuildAccountQualityStats(95, 5, TTFTAggregate{
		Samples: 90,
		Avg:     &avg,
		P50:     &p50,
		P95:     &p95,
		Max:     &max,
	})
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.95, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.ErrorRate)
	require.InDelta(t, 0.05, *stats.ErrorRate, 1e-9)
	require.NotNil(t, stats.AvgTTFTMs)
	require.Equal(t, 420, *stats.AvgTTFTMs)
	require.NotNil(t, stats.P50TTFTMs)
	require.Equal(t, 300, *stats.P50TTFTMs)
	require.NotNil(t, stats.P95TTFTMs)
	require.Equal(t, 900, *stats.P95TTFTMs)
	require.NotNil(t, stats.MaxTTFTMs)
	require.Equal(t, 5000, *stats.MaxTTFTMs)
	require.Equal(t, int64(90), stats.TTFTSamples)
}

func TestBuildAccountQualityStats_ErrorsOnly(t *testing.T) {
	stats := BuildAccountQualityStats(0, 3, TTFTAggregate{})
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.0, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.ErrorRate)
	require.InDelta(t, 1.0, *stats.ErrorRate, 1e-9)
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
}

func TestQualityResumeHelpers(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	fresh := &AccountQualityStats{SuccessCount: 2}
	old := &AccountQualityStats{}
	SetUserQualityResume(old, 16, now.Add(AccountQualityWindow))
	SetAccountQualityResume(old, now.Add(5*time.Minute))

	MergeQualityResume(fresh, old, now)
	require.True(t, UserQualityResumeActive(fresh, 16, now))
	require.True(t, AccountQualityResumeActive(fresh, now))
	require.True(t, HasActiveQualityResume(fresh, now))

	MergeQualityResume(fresh, old, now.Add(20*time.Minute))
	require.False(t, UserQualityResumeActive(fresh, 16, now.Add(20*time.Minute)))
	require.False(t, AccountQualityResumeActive(fresh, now.Add(20*time.Minute)))
	require.False(t, HasActiveQualityResume(fresh, now.Add(20*time.Minute)))
}

func TestApplyUserQualityResumeTwoPhase(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	stats := &AccountQualityStats{}
	ApplyUserQualityResume(stats, 16, now)
	require.True(t, UserQualityResumedChipActive(stats, 16, now))
	require.True(t, UserQualityResumeActive(stats, 16, now))

	afterResume := now.Add(AccountQualityWindow + time.Minute)
	require.False(t, UserQualityResumedChipActive(stats, 16, afterResume))
	require.True(t, UserQualityResumeActive(stats, 16, afterResume))

	clickAt := now.Add(2 * time.Minute)
	ApplyUserQualityWindowStart(stats, 16, clickAt)
	require.False(t, UserQualityResumedChipActive(stats, 16, clickAt))
	require.True(t, UserQualityResumeActive(stats, 16, clickAt))
	require.False(t, UserQualityResumeActive(stats, 16, clickAt.Add(AccountQualityWindow+time.Minute)))
}

func TestClearUserQualityResume(t *testing.T) {
	stats := &AccountQualityStats{}
	ApplyUserQualityResume(stats, 16, time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	ClearUserQualityResume(stats, 16)
	require.Nil(t, stats.ResumeUsers)
	require.Nil(t, stats.ResumeWatchingUsers)
}

func TestHasAccountQualitySamples(t *testing.T) {
	require.False(t, HasAccountQualitySamples(nil))
	require.False(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 0, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(1, 0, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 1, TTFTAggregate{})))
	require.True(t, HasAccountQualitySamples(BuildAccountQualityStats(0, 0, TTFTAggregate{Samples: 1})))
	bridgeOnly := BuildAccountQualityStats(0, 0, TTFTAggregate{})
	AttachBridgeQualityCounts(bridgeOnly, 0, 2)
	require.True(t, HasAccountQualitySamples(bridgeOnly))
}

func TestAttachBridgeQualityCounts_SeparateFromScheduleRate(t *testing.T) {
	stats := BuildAccountQualityStats(9, 1, TTFTAggregate{})
	AttachBridgeQualityCounts(stats, 4, 6)
	require.Equal(t, int64(9), stats.SuccessCount)
	require.Equal(t, int64(1), stats.ErrorCount)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.9, *stats.SuccessRate, 1e-9)
	require.Equal(t, int64(4), stats.BridgeSuccessCount)
	require.Equal(t, int64(6), stats.BridgeErrorCount)
	require.NotNil(t, stats.BridgeErrorRate)
	require.InDelta(t, 0.6, *stats.BridgeErrorRate, 1e-9)
}

func TestSQLClaudeGPTBridgePredicates(t *testing.T) {
	errPred := SQLClaudeGPTBridgeErrorPredicate("platform", "upstream_model")
	require.Contains(t, errPred, "IN ('antigravity','anthropic')")
	require.Contains(t, errPred, "LIKE 'gpt-%'")
	require.Equal(t, "NOT "+errPred, SQLExcludeClaudeGPTBridgeError("platform", "upstream_model"))
	usagePred := SQLClaudeGPTBridgeUsagePredicate("requested_model", "model", "upstream_model")
	require.Contains(t, usagePred, "LIKE 'claude-%'")
	require.Contains(t, usagePred, "LIKE 'gpt-%'")
}

func TestApplyAccountQualityScheduleCaliber_DefaultKeepsTerminal(t *testing.T) {
	stats := BuildAccountQualityStats(90, 0, TTFTAggregate{})
	AttachAccountQualityErrorCalibers(stats, 5, 20)
	require.Equal(t, int64(5), stats.ErrorCount)
	require.Equal(t, int64(5), stats.TerminalErrorCount)
	require.Equal(t, int64(20), stats.FailoverErrorCount)
	require.False(t, stats.ScheduleUseFailoverErrorRate)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 90.0/95.0, *stats.SuccessRate, 1e-9)

	ApplyAccountQualityScheduleCaliber(stats, false)
	require.Equal(t, int64(5), stats.ErrorCount)
	require.False(t, stats.ScheduleUseFailoverErrorRate)
	require.InDelta(t, 90.0/95.0, *stats.SuccessRate, 1e-9)
}

func TestAttachAccountQualityErrorCalibers_ZeroFailoverKeepsUserRates(t *testing.T) {
	p50 := 280.0
	stats := BuildAccountQualityStats(8, 2, TTFTAggregate{Samples: 7, P50: &p50})
	AttachAccountQualityErrorCalibers(stats, 2, 0)
	require.Equal(t, int64(2), stats.ErrorCount)
	require.Equal(t, int64(2), stats.TerminalErrorCount)
	require.Equal(t, int64(0), stats.FailoverErrorCount)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 0.8, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.P50TTFTMs)
	require.Equal(t, 280, *stats.P50TTFTMs)
}

func TestApplyAccountQualityScheduleCaliber_OnUsesFailover(t *testing.T) {
	stats := BuildAccountQualityStats(90, 0, TTFTAggregate{})
	AttachAccountQualityErrorCalibers(stats, 5, 20)
	ApplyAccountQualityScheduleCaliber(stats, true)
	require.True(t, stats.ScheduleUseFailoverErrorRate)
	require.Equal(t, int64(20), stats.ErrorCount)
	require.Equal(t, int64(5), stats.TerminalErrorCount)
	require.Equal(t, int64(20), stats.FailoverErrorCount)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 90.0/110.0, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.ErrorRate)
	require.InDelta(t, 20.0/110.0, *stats.ErrorRate, 1e-9)
}

func TestClassifyOpsErrorRateCalibers(t *testing.T) {
	recovered := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 200,
		Phase:        "upstream",
		Type:         "rate_limit_error",
		Message:      "Recovered upstream error 429: too many requests",
	})
	require.True(t, recovered.IsRecovered)
	require.False(t, recovered.CountedInUserErrorRate)
	require.True(t, recovered.CountedInAccountCompareRate)
	require.False(t, recovered.CountedInAccountScheduleRate)
	require.False(t, recovered.NeedsOpsAttention)

	recoveredOn := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 200,
		Phase:        "upstream",
		Type:         "rate_limit_error",
		Message:      "Recovered upstream error 429: too many requests",
		UseFailover:  true,
	})
	require.True(t, recoveredOn.CountedInAccountScheduleRate)
	require.False(t, recoveredOn.CountedInUserErrorRate)
	require.False(t, recoveredOn.NeedsOpsAttention)

	modelMiss := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 404,
		Phase:        "internal",
		Type:         "api_error",
		Message:      "model_not_found: claude-bad",
	})
	require.False(t, modelMiss.IsRecovered)
	require.True(t, modelMiss.CountedInUserErrorRate)
	require.False(t, modelMiss.CountedInAccountCompareRate)
	require.False(t, modelMiss.CountedInAccountScheduleRate)
	require.True(t, modelMiss.NeedsOpsAttention)

	terminal := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 502,
		Phase:        "upstream",
		Type:         "upstream_error",
		Message:      "bad gateway",
	})
	require.True(t, terminal.CountedInUserErrorRate)
	require.True(t, terminal.CountedInAccountCompareRate)
	require.True(t, terminal.CountedInAccountScheduleRate)
	require.False(t, terminal.NeedsOpsAttention)

	bridge := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus:  502,
		Phase:         "upstream",
		Type:          "upstream_error",
		Message:       "bad gateway",
		Platform:      PlatformAnthropic,
		UpstreamModel: "gpt-4.1",
	})
	require.True(t, bridge.CountedInUserErrorRate)
	require.False(t, bridge.CountedInAccountCompareRate)
	require.False(t, bridge.CountedInAccountScheduleRate)
	require.False(t, bridge.NeedsOpsAttention)
}

func TestClassifyOpsErrorRateCalibers_ScheduleExcludeAndAttention(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		in        OpsErrorCaliberInput
		schedule  bool
		attention bool
	}{
		{
			name: "invalid_request_400",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "request", Type: "invalid_request_error",
				Message: "missing required parameter",
			},
			schedule: true,
		},
		{
			name: "hop_invalid_request_upstream_400",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "upstream", Type: "invalid_request_error",
				Message: "invalid json from hop",
			},
			schedule: true,
		},
		{
			name: "upstream_request_failed_400",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "internal", Type: "api_error",
				Message: "Upstream request failed",
			},
			schedule: true,
		},
		{
			name: "prompt_too_long",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "request", Type: "invalid_request_error",
				Message: "prompt is too long",
			},
			schedule: true,
		},
		{
			name: "pair_concurrency_429",
			in: OpsErrorCaliberInput{
				ClientStatus: 429, Phase: "request", Type: "rate_limit_error",
				Message: "Concurrency limit exceeded for account",
			},
			schedule: true,
		},
		{
			name: "group_no_account_404",
			in: OpsErrorCaliberInput{
				ClientStatus: 404, Phase: "internal", Type: "api_error",
				Message: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
			},
			attention: true,
		},
		{
			name: "group_no_account_502",
			in: OpsErrorCaliberInput{
				ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
				Message: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
			},
			schedule:  true,
			attention: true,
		},
		{
			name: "routing_503",
			in: OpsErrorCaliberInput{
				ClientStatus: 503, Phase: "routing", Type: "api_error",
				Message: "Service temporarily unavailable",
			},
			schedule:  true,
			attention: true,
		},
		{
			name: "chat_completions_endpoint",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "upstream", Type: "invalid_request_error",
				Message: "not supported on the Chat Completions endpoint",
			},
			schedule:  true,
			attention: true,
		},
		{
			name: "unsupported_content_type",
			in: OpsErrorCaliberInput{
				ClientStatus: 400, Phase: "upstream", Type: "invalid_request_error",
				Message: "Unsupported content type",
			},
			schedule:  true,
			attention: true,
		},
		{
			name: "invalid_url",
			in: OpsErrorCaliberInput{
				ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
				Message: "Invalid URL",
			},
			schedule:  true,
			attention: true,
		},
		{
			name: "upstream_request_failed_502",
			in: OpsErrorCaliberInput{
				ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
				Message: "Upstream request failed",
			},
			schedule: true,
		},
		{
			name: "true_upstream_429",
			in: OpsErrorCaliberInput{
				ClientStatus: 429, Phase: "upstream", Type: "rate_limit_error",
				Message: "Rate limit reached for gpt-5.2",
			},
			schedule: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyOpsErrorRateCalibers(tc.in)
			require.Equal(t, tc.schedule, got.CountedInAccountScheduleRate)
			require.Equal(t, tc.attention, got.NeedsOpsAttention)
			on := tc.in
			on.UseFailover = true
			gotOn := ClassifyOpsErrorRateCalibers(on)
			require.Equal(t, tc.schedule, gotOn.CountedInAccountScheduleRate)
			require.Equal(t, tc.attention, gotOn.NeedsOpsAttention)
		})
	}
}

func TestSQLScheduleQualityExcludedPredicate_Covers502GroupGap(t *testing.T) {
	pred := SQLScheduleQualityExcludedPredicate("")
	require.Contains(t, pred, "%not supported by any configured account%")
	require.Contains(t, pred, "IN (400, 403, 404, 503)")
	require.Contains(t, pred, "<> 'upstream'")
	groupPred := SQLGroupNoAccountForModelPredicate("")
	require.NotContains(t, groupPred, "<> 'upstream'")
	require.NotContains(t, groupPred, "IN (400, 403, 404, 503)")
	require.Contains(t, SQLOpsAttentionPredicate("e."), "e.error_message")
	require.Contains(t, SQLExcludeAccountQualityScheduleNoise(""), "NOT (")
}

func TestSQLAccountQualityRoutingModelMissPredicate(t *testing.T) {
	pred := SQLAccountQualityRoutingModelMissPredicate()
	require.Contains(t, pred, "COALESCE(status_code, 0) IN (400, 403, 404, 503)")
	require.Contains(t, pred, "COALESCE(error_phase, '') <> 'upstream'")
	require.Contains(t, pred, "'upstream_error','overloaded_error','rate_limit_error'")
	require.Contains(t, pred, "LOWER(COALESCE(error_type, '')) = 'model_not_found'")
	require.Contains(t, pred, "%model_not_found%")
	require.Contains(t, pred, "%unknown model%")
	require.Contains(t, pred, "%model not found%")
	require.Contains(t, pred, "%unsupported model%")
	require.Contains(t, pred, "%does not exist%")
	require.Contains(t, pred, "%not supported by any configured account%")
	require.Contains(t, pred, "%supporting model:%")
	require.Contains(t, pred, "%no account supports%")
	require.Contains(t, pred, "%not in whitelist%")
	require.NotContains(t, pred, "429")
	require.NotContains(t, pred, "502")
	require.NotContains(t, pred, "upstream_status_code")
	require.NotContains(t, pred, "COALESCE(upstream_status_code")
	require.Equal(t, "NOT ("+pred+")", SQLExcludeAccountQualityRoutingModelMiss())
}

func TestTruncateToAccountQualitySnapshotTime(t *testing.T) {
	got := TruncateToAccountQualitySnapshotTime(time.Date(2026, 8, 14, 12, 7, 30, 0, time.UTC))
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), got)

	cst := time.FixedZone("CST", 8*3600)
	got = TruncateToAccountQualitySnapshotTime(time.Date(2026, 8, 14, 20, 7, 30, 0, cst))
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), got)
}

func TestSnapshotFromAccountQualityStats_MatchesLiveNullsAndTruncates(t *testing.T) {
	captured := time.Date(2026, 8, 14, 12, 7, 30, 0, time.UTC)
	errorsOnly := BuildAccountQualityStats(0, 3, TTFTAggregate{})
	row := SnapshotFromAccountQualityStats(7, captured, errorsOnly)

	require.Equal(t, int64(7), row.AccountID)
	require.Equal(t, time.Date(2026, 8, 14, 12, 5, 0, 0, time.UTC), row.CapturedAt)
	require.Equal(t, AccountQualityWindowSeconds, row.WindowSeconds)
	require.Equal(t, int64(0), row.SuccessCount)
	require.Equal(t, int64(3), row.ErrorCount)
	require.NotNil(t, row.SuccessRate)
	require.InDelta(t, 0.0, *row.SuccessRate, 1e-9)
	require.Nil(t, row.AvgTTFTMs)
	require.Nil(t, row.P50TTFTMs)
	require.Nil(t, row.P95TTFTMs)
	require.Nil(t, row.MaxTTFTMs)
	require.Equal(t, int64(0), row.TTFTSamples)

	item := row.ToHistoryItem()
	require.Equal(t, row.CapturedAt, item.CapturedAt)
	require.Equal(t, row.WindowSeconds, item.WindowSeconds)
	require.Equal(t, row.SuccessCount, item.SuccessCount)
	require.Equal(t, row.ErrorCount, item.ErrorCount)
	require.Equal(t, row.SuccessRate, item.SuccessRate)
	require.NotNil(t, item.ErrorRate)
	require.InDelta(t, 1.0, *item.ErrorRate, 1e-9)
	require.Nil(t, item.P50TTFTMs)
	require.Equal(t, row.TTFTSamples, item.TTFTSamples)
}

func TestBuildAccountQualityStats_TTFTWithoutSamplesIgnored(t *testing.T) {
	avg := 100.0
	stats := BuildAccountQualityStats(10, 0, TTFTAggregate{Samples: 0, Avg: &avg})
	require.Nil(t, stats.AvgTTFTMs)
	require.Nil(t, stats.P50TTFTMs)
	require.NotNil(t, stats.SuccessRate)
	require.InDelta(t, 1.0, *stats.SuccessRate, 1e-9)
	require.NotNil(t, stats.ErrorRate)
	require.InDelta(t, 0.0, *stats.ErrorRate, 1e-9)
}

func TestAccountQualityStats_EmptyWindowJSONDoesNotEmitZeroRates(t *testing.T) {
	raw, err := json.Marshal(BuildAccountQualityStats(0, 0, TTFTAggregate{}))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Nil(t, payload["success_rate"])
	require.Nil(t, payload["error_rate"])
	require.NotContains(t, string(raw), `"success_rate":0`)
	require.NotContains(t, string(raw), `"error_rate":0`)
}

func TestNormalizeAccountQualityRates_ClearsBogusZeroOnEmptyWindow(t *testing.T) {
	zero := 0.0
	stats := &AccountQualityStats{SuccessRate: &zero, ErrorRate: &zero}
	NormalizeAccountQualityRates(stats)
	require.Nil(t, stats.SuccessRate)
	require.Nil(t, stats.ErrorRate)
}
