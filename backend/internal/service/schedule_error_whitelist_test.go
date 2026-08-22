package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func whitelistOff(id string) ScheduleErrorWhitelist {
	wl := DefaultScheduleErrorWhitelist()
	wl.Families[id] = false
	return wl
}

func whitelistAll(on bool) ScheduleErrorWhitelist {
	wl := DefaultScheduleErrorWhitelist()
	for _, id := range ScheduleErrorFamilyIDs {
		wl.Families[id] = on
	}
	return wl
}

func TestClassify_HopInvalidRequestCounts_RequestPhaseFollowsWhitelist(t *testing.T) {
	t.Parallel()
	hop := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "upstream", Type: "invalid_request_error",
		Message: "invalid json from hop",
	})
	require.True(t, hop.CountedInAccountScheduleRate)
	require.False(t, hop.NeedsOpsAttention)

	clientDefault := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "request", Type: "invalid_request_error",
		Message: "invalid json from hop",
	})
	require.True(t, clientDefault.CountedInAccountScheduleRate)
	require.False(t, clientDefault.NeedsOpsAttention)

	allOn := whitelistAll(true)
	clientOn := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "request", Type: "invalid_request_error",
		Message: "invalid json from hop",
		Whitelist:    &allOn,
	})
	require.False(t, clientOn.CountedInAccountScheduleRate)
	require.False(t, clientOn.NeedsOpsAttention)
}

func TestClassify_GroupNoAccountWhitelistToggle(t *testing.T) {
	t.Parallel()
	in := OpsErrorCaliberInput{
		ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
		Message: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
	}

	gotDefault := ClassifyOpsErrorRateCalibers(in)
	require.True(t, gotDefault.CountedInAccountScheduleRate)
	require.True(t, gotDefault.NeedsOpsAttention)

	on := in
	wlOn := whitelistAll(true)
	on.Whitelist = &wlOn
	gotOn := ClassifyOpsErrorRateCalibers(on)
	require.False(t, gotOn.CountedInAccountScheduleRate)
	require.True(t, gotOn.NeedsOpsAttention)

	off := in
	wl := whitelistOff(ScheduleErrorFamilyGroupNoAccount)
	off.Whitelist = &wl
	gotOff := ClassifyOpsErrorRateCalibers(off)
	require.True(t, gotOff.CountedInAccountScheduleRate)
	require.True(t, gotOff.NeedsOpsAttention)
}

func TestClassify_RoutingModelMissAlwaysExcluded(t *testing.T) {
	t.Parallel()
	in := OpsErrorCaliberInput{
		ClientStatus: 404, Phase: "internal", Type: "model_not_found",
		Message: "model_not_found: claude-bad",
	}
	got := ClassifyOpsErrorRateCalibers(in)
	require.False(t, got.CountedInAccountScheduleRate)
	require.True(t, got.NeedsOpsAttention)

	allOff := whitelistAll(false)
	in.Whitelist = &allOff
	gotOff := ClassifyOpsErrorRateCalibers(in)
	require.False(t, gotOff.CountedInAccountScheduleRate)
	require.True(t, gotOff.NeedsOpsAttention)
}

func TestClassify_502UpstreamRequestFailedAlwaysCounted(t *testing.T) {
	t.Parallel()
	in := OpsErrorCaliberInput{
		ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
		Message: "Upstream request failed",
	}
	for _, name := range []string{"default", "all_on", "all_off"} {
		t.Run(name, func(t *testing.T) {
			tc := in
			switch name {
			case "all_on":
				wl := whitelistAll(true)
				tc.Whitelist = &wl
			case "all_off":
				wl := whitelistAll(false)
				tc.Whitelist = &wl
			}
			got := ClassifyOpsErrorRateCalibers(tc)
			require.True(t, got.CountedInAccountScheduleRate)
			require.False(t, got.NeedsOpsAttention)
		})
	}
}

func TestClassify_400UpstreamRequestFailedDefaultCounted(t *testing.T) {
	t.Parallel()
	got := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "internal", Type: "api_error",
		Message: "Upstream request failed",
	})
	require.True(t, got.CountedInAccountScheduleRate)
	require.False(t, got.NeedsOpsAttention)

	allOn := whitelistAll(true)
	gotOn := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "internal", Type: "api_error",
		Message:   "Upstream request failed",
		Whitelist: &allOn,
	})
	require.False(t, gotOn.CountedInAccountScheduleRate)
	require.False(t, gotOn.NeedsOpsAttention)
}

func TestApplyOpsErrorRateCalibers_BodyOnlyGroupNoAccount(t *testing.T) {
	t.Parallel()
	item := &OpsErrorLog{
		Phase:     "upstream",
		Type:      "upstream_error",
		Message:   "",
		ErrorBody: `Model "gpt-5.6-terra" is not supported by any configured account in this group`,
		Platform:  "openai",
	}
	ApplyOpsErrorRateCalibers(item, 502, item.ErrorBody, false)
	require.True(t, item.NeedsOpsAttention)
	require.True(t, item.CountedInAccountScheduleRate)
}

func TestApplyErrorLogCalibers_KeepsErrorBody(t *testing.T) {
	t.Parallel()
	svc := &OpsService{}
	result := &OpsErrorLogList{Errors: []*OpsErrorLog{{
		Phase:            "upstream",
		Type:             "upstream_error",
		StatusCode:       502,
		ClientStatusCode: 502,
		ErrorBody:        `supporting model: gpt-5.6-terra`,
	}}}
	svc.applyErrorLogCalibers(context.Background(), result)
	require.True(t, result.Errors[0].NeedsOpsAttention)
}

func TestValidateScheduleErrorWhitelist_AcceptsLegacyRoutingModelMiss(t *testing.T) {
	t.Parallel()
	err := ValidateScheduleErrorWhitelist(&ScheduleErrorWhitelist{
		Families: map[string]bool{ScheduleErrorFamilyRoutingModelMiss: true},
	})
	require.NoError(t, err)
	normalized := NormalizeScheduleErrorWhitelist(ScheduleErrorWhitelist{
		Families: map[string]bool{ScheduleErrorFamilyRoutingModelMiss: false},
	})
	_, present := normalized.Families[ScheduleErrorFamilyRoutingModelMiss]
	require.False(t, present)
}

func TestValidateScheduleErrorWhitelist_RejectsUnknownFamily(t *testing.T) {
	t.Parallel()
	err := ValidateScheduleErrorWhitelist(&ScheduleErrorWhitelist{
		Families: map[string]bool{"drop_all_upstream_failed": true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown schedule error whitelist family")
}

func TestSetScheduleErrorWhitelist_RejectsUnknownFamily(t *testing.T) {
	t.Parallel()
	repo := newRuntimeSettingRepoStub()
	svc := NewSettingService(repo, nil)
	err := svc.SetScheduleErrorWhitelist(context.Background(), &ScheduleErrorWhitelist{
		Families: map[string]bool{"custom_like": true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
}

func TestGetScheduleErrorWhitelist_MissingAndPartial(t *testing.T) {
	t.Parallel()
	repo := newRuntimeSettingRepoStub()
	svc := NewSettingService(repo, nil)
	got, err := svc.GetScheduleErrorWhitelist(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultScheduleErrorWhitelist(), *got)

	partial := ScheduleErrorWhitelist{Families: map[string]bool{
		ScheduleErrorFamilyGroupNoAccount: false,
	}}
	require.NoError(t, svc.SetScheduleErrorWhitelist(context.Background(), &partial))
	got, err = svc.GetScheduleErrorWhitelist(context.Background())
	require.NoError(t, err)
	require.False(t, got.FamilyEnabled(ScheduleErrorFamilyGroupNoAccount))
	require.False(t, got.FamilyEnabled(ScheduleErrorFamilyClientInvalidRequest))

	var stored ScheduleErrorWhitelist
	require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyScheduleErrorWhitelist]), &stored))
	require.Len(t, stored.Families, len(ScheduleErrorFamilyIDs))
	_, hasLegacyMiss := stored.Families[ScheduleErrorFamilyRoutingModelMiss]
	require.False(t, hasLegacyMiss)
}

func TestSQLScheduleQualityExcludedPredicate_RespectsWhitelist(t *testing.T) {
	t.Parallel()
	def := SQLScheduleQualityExcludedPredicate("")
	require.Contains(t, def, "%not supported by any configured account%")
	require.Contains(t, def, "IN (400, 403, 404, 503)")
	require.Contains(t, def, "<> 'upstream'")
	require.Contains(t, def, "502")
	require.Contains(t, def, "%upstream request failed%")
	require.NotContains(t, def, "invalid_request_error")

	allOff := SQLScheduleQualityExcludedPredicateWith("", whitelistAll(false))
	require.NotEqual(t, "FALSE", allOff)
	require.Contains(t, allOff, "IN (400, 403, 404, 503)")
	require.NotContains(t, allOff, "invalid_request_error")

	onlyGroup := whitelistAll(false)
	onlyGroup.Families[ScheduleErrorFamilyGroupNoAccount] = true
	groupPred := SQLScheduleQualityExcludedPredicateWith("", onlyGroup)
	require.Contains(t, groupPred, "%not supported by any configured account%")
	require.NotContains(t, groupPred, "invalid_request_error")
}

func TestParseScheduleErrorWhitelistJSON_EmptyIsDefault(t *testing.T) {
	t.Parallel()
	require.Equal(t, DefaultScheduleErrorWhitelist(), ParseScheduleErrorWhitelistJSON(""))
	require.Equal(t, DefaultScheduleErrorWhitelist(), ParseScheduleErrorWhitelistJSON("not-json"))
	got := ParseScheduleErrorWhitelistJSON(`{"families":{}}`)
	require.False(t, got.FamilyEnabled(ScheduleErrorFamilyClientInvalidRequest))
	require.False(t, got.FamilyEnabled(ScheduleErrorFamilyGroupNoAccount))
	require.Equal(t, DefaultScheduleErrorWhitelist(), got)
}
