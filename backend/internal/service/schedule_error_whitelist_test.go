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
		Message:   "invalid json from hop",
		Whitelist: &allOn,
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

func TestClassify_CustomMessageContainsExcludes(t *testing.T) {
	t.Parallel()
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled: true, MessageContains: "no available key",
	}}
	in := OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "upstream", Type: "upstream_error",
		Message:           "mapped downstream",
		ProviderErrorCode: "channel:no_available_key",
		Whitelist:         &wl,
	}
	got := ClassifyOpsErrorRateCalibers(in)
	require.False(t, got.CountedInAccountScheduleRate)

	off := DefaultScheduleErrorWhitelist()
	off.Custom = []ScheduleErrorCustomRule{{
		Enabled: false, MessageContains: "no available key",
	}}
	in.Whitelist = &off
	require.True(t, ClassifyOpsErrorRateCalibers(in).CountedInAccountScheduleRate)
}

func TestClassify_CustomStructuredAND(t *testing.T) {
	t.Parallel()
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled: true, ErrorType: "upstream_error", Phase: "upstream", StatusCode: 400,
		ProviderErrorCode: "channel:no_available_key",
	}}
	hit := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "upstream", Type: "upstream_error",
		ProviderErrorCode: "channel:no_available_key",
		Whitelist:         &wl,
	})
	require.False(t, hit.CountedInAccountScheduleRate)

	missPhase := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 400, Phase: "request", Type: "upstream_error",
		ProviderErrorCode: "channel:no_available_key",
		Whitelist:         &wl,
	})
	require.True(t, missPhase.CountedInAccountScheduleRate)
}

func TestClassify_CustomCannotExclude502URF(t *testing.T) {
	t.Parallel()
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled: true, ErrorType: "upstream_error", Phase: "upstream", StatusCode: 502,
		MessageContains: "upstream request failed",
	}}
	got := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 502, Phase: "upstream", Type: "upstream_error",
		Message:   "Upstream request failed",
		Whitelist: &wl,
	})
	require.True(t, got.CountedInAccountScheduleRate)
}

func TestSQLCustomRule_EscapesLikeMetacharacters(t *testing.T) {
	t.Parallel()
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled: true, MessageContains: `100%_fail'`,
	}}
	pred := SQLScheduleQualityExcludedPredicateWith("", wl)
	require.Contains(t, pred, `ESCAPE '\'`)
	require.Contains(t, pred, `%100\%\_fail''%`)
	require.NotContains(t, pred, `%100%_fail'%`)
}

func TestValidateScheduleErrorWhitelist_RejectsEmptyCustom(t *testing.T) {
	t.Parallel()
	err := ValidateScheduleErrorWhitelist(&ScheduleErrorWhitelist{
		Families: map[string]bool{},
		Custom:   []ScheduleErrorCustomRule{{Enabled: true}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "at least one match field")
}

func TestSetScheduleErrorWhitelist_PersistsCustom(t *testing.T) {
	t.Parallel()
	repo := newRuntimeSettingRepoStub()
	svc := NewSettingService(repo, nil)
	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		Enabled: true, MessageContains: "quota exceeded",
	}}
	require.NoError(t, svc.SetScheduleErrorWhitelist(context.Background(), &wl))
	got, err := svc.GetScheduleErrorWhitelist(context.Background())
	require.NoError(t, err)
	require.Len(t, got.Custom, 1)
	require.True(t, got.Custom[0].Enabled)
	require.Equal(t, "quota exceeded", got.Custom[0].MessageContains)
	require.NotEmpty(t, got.Custom[0].ID)
}

func TestUpsertScheduleErrorCustomRule_DedupEnables(t *testing.T) {
	t.Parallel()
	repo := newRuntimeSettingRepoStub()
	svc := NewSettingService(repo, nil)
	first, err := svc.UpsertScheduleErrorCustomRule(context.Background(), ScheduleErrorCustomRule{
		Enabled: false, MessageContains: "quota exceeded",
	})
	require.NoError(t, err)
	require.Len(t, first.Custom, 1)
	id := first.Custom[0].ID
	first.Custom[0].Enabled = false
	require.NoError(t, svc.SetScheduleErrorWhitelist(context.Background(), first))

	second, err := svc.UpsertScheduleErrorCustomRule(context.Background(), ScheduleErrorCustomRule{
		MessageContains: "Quota Exceeded",
	})
	require.NoError(t, err)
	require.Len(t, second.Custom, 1)
	require.Equal(t, id, second.Custom[0].ID)
	require.True(t, second.Custom[0].Enabled)
}

func TestBuildScheduleErrorCustomRuleFromLog(t *testing.T) {
	t.Parallel()
	log := &OpsErrorLog{
		Type: "upstream_error", Phase: "upstream", StatusCode: 400,
		ClientStatusCode:     400,
		ProviderErrorCode:    "channel:no_available_key",
		UpstreamErrorMessage: "no available key",
		Message:              "mapped",
	}
	structured, err := BuildScheduleErrorCustomRuleFromLog(log, ScheduleErrorFromErrorStructured)
	require.NoError(t, err)
	require.Equal(t, "upstream_error", structured.ErrorType)
	require.Equal(t, "upstream", structured.Phase)
	require.Equal(t, 400, structured.StatusCode)
	require.Equal(t, "channel:no_available_key", structured.ProviderErrorCode)
	require.Empty(t, structured.MessageContains)

	msg, err := BuildScheduleErrorCustomRuleFromLog(log, ScheduleErrorFromErrorMessage)
	require.NoError(t, err)
	require.Equal(t, "channel:no_available_key no available key", msg.MessageContains)
	require.Empty(t, msg.ErrorType)

	_, err = BuildScheduleErrorCustomRuleFromLog(&OpsErrorLog{
		Type: "upstream_error", Phase: "upstream", StatusCode: 502,
		ClientStatusCode: 502, Message: "Upstream request failed",
	}, ScheduleErrorFromErrorStructured)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be whitelisted")
}
