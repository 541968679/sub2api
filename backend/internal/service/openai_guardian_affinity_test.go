//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func guardianAffinityTestContext(t *testing.T, model, subagent, parentHeader, metadata string) context.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", nil)
	c.Request.Header.Set(openAISubagentHeader, subagent)
	if parentHeader != "" {
		c.Request.Header.Set(codexParentThreadIDHeader, parentHeader)
	}
	if metadata != "" {
		c.Request.Header.Set(codexTurnMetadataHeader, metadata)
	}
	return WithOpenAIGuardianParentAffinity(context.Background(), c, nil, model)
}

func TestWithOpenAIGuardianParentAffinity_RequiresUnambiguousReviewLineage(t *testing.T) {
	parentID := "11111111-1111-4111-8111-111111111111"
	wantHash := DeriveSessionHashFromSeed(parentID)

	for _, subagent := range []string{"guardian", "review", "GUARDIAN"} {
		t.Run(subagent, func(t *testing.T) {
			ctx := guardianAffinityTestContext(t, codexAutoReviewModel, subagent, parentID, `{"parent_thread_id":"`+parentID+`"}`)
			affinity, ok := openAIGuardianParentAffinityFromContext(ctx)
			require.True(t, ok)
			require.Equal(t, wantHash, affinity.currentSessionHash)
		})
	}

	t.Run("metadata only", func(t *testing.T) {
		ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", "", `{"parent_thread_id":"`+parentID+`"}`)
		_, ok := openAIGuardianParentAffinityFromContext(ctx)
		require.True(t, ok)
	})

	t.Run("websocket envelope metadata", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/openai/v1/responses", nil)
		body := []byte(`{"type":"response.create","response":{"model":"codex-auto-review","client_metadata":{"x-codex-turn-metadata":"{\"parent_thread_id\":\"` + parentID + `\",\"subagent_kind\":\"guardian\"}"}}}`)
		ctx := WithOpenAIGuardianParentAffinity(context.Background(), c, body, codexAutoReviewModel)
		affinity, ok := openAIGuardianParentAffinityFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, wantHash, affinity.currentSessionHash)
	})

	for name, ctx := range map[string]context.Context{
		"ordinary model":       guardianAffinityTestContext(t, "gpt-5.6-sol", "guardian", parentID, ""),
		"ordinary subagent":    guardianAffinityTestContext(t, codexAutoReviewModel, "collab_spawn", parentID, ""),
		"missing parent":       guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", "", ""),
		"conflicting lineage":  guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, `{"parent_thread_id":"different-parent"}`),
		"conflicting subagent": guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, `{"parent_thread_id":"`+parentID+`","subagent_kind":"collab_spawn"}`),
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := openAIGuardianParentAffinityFromContext(ctx)
			require.False(t, ok)
		})
	}
}

func TestOpenAIGatewayService_GuardianParentAffinitySelectsParentAccountAcrossSchedulers(t *testing.T) {
	parentID := "22222222-2222-4222-8222-222222222222"
	parentHash := DeriveSessionHashFromSeed(parentID)
	groupID := int64(102001)

	for _, mode := range []struct {
		name           string
		advanced       string
		stickyWeighted string
	}{
		{name: "legacy", advanced: "false"},
		{name: "advanced", advanced: "true"},
		{name: "advanced sticky weighted", advanced: "true", stickyWeighted: "true"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			accounts := []Account{
				{
					ID: 39001, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
					Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
					GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"},
				},
				{
					ID: 39002, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
					Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 0,
					GroupIDs: []int64{groupID}, Credentials: map[string]any{"plan_type": "team"},
				},
			}
			cfg := &config.Config{}
			cfg.Gateway.OpenAIWS.LBTopK = 2
			cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:" + parentHash: 39001}}
			svc := &OpenAIGatewayService{
				accountRepo:        schedulerGroupAwareOpenAIAccountRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}},
				cache:              cache,
				cfg:                cfg,
				rateLimitService:   newOpenAIAdvancedSchedulerRateLimitService(mode.advanced, mode.stickyWeighted),
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{acquireResults: map[int64]bool{39001: true, 39002: true}}),
			}

			ctx := guardianAffinityTestContext(t, codexAutoReviewModel, "guardian", parentID, "")
			selection, decision, err := svc.SelectAccountWithScheduler(
				ctx, &groupID, "", "guardian-child-session", codexAutoReviewModel,
				nil, OpenAIUpstreamTransportAny, false,
			)
			require.NoError(t, err)
			require.NotNil(t, selection)
			require.Equal(t, int64(39001), selection.Account.ID)
			require.Equal(t, openAIAccountScheduleLayerGuardianParent, decision.Layer)
			require.Zero(t, cache.deletedSessions["openai:"+parentHash])
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
		})
	}
}
