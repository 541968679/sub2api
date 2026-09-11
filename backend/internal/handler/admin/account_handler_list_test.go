package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/stretchr/testify/require"
)

func TestAccountListLiteKeepsQualityAndSmartScheduleColumns(t *testing.T) {
	items := []AccountWithConcurrency{{
		Account: &dto.Account{
			ID:               7,
			Name:             "oauth-1",
			UserScheduleMode: "allowlist",
			ScheduleUsers: []dto.ScheduleUser{{
				ID:                    11,
				Email:                 "user@example.com",
				QualityMinSuccessRate: floatPtr(0.95),
			}},
			AccountGroups: []dto.AccountGroup{{AccountID: 7, GroupID: 3}},
			Groups:        []*dto.Group{{ID: 3, Name: "g"}},
			Credentials: map[string]any{
				"email":     "hidden@example.com",
				"plan_type": "plus",
				"api_key":   "sk-secret",
			},
		},
		CurrentConcurrency: 1,
	}}
	applyAccountListLiteProjection(items)

	require.Equal(t, "allowlist", items[0].Account.UserScheduleMode)
	require.Len(t, items[0].Account.ScheduleUsers, 1)
	require.Equal(t, 0.95, *items[0].Account.ScheduleUsers[0].QualityMinSuccessRate)
	require.Nil(t, items[0].Account.AccountGroups)
	require.Nil(t, items[0].Account.Groups)
	require.Equal(t, "plus", items[0].Account.Credentials["plan_type"])
	_, hasKey := items[0].Account.Credentials["api_key"]
	require.False(t, hasKey)

	payload, err := json.Marshal(items[0])
	require.NoError(t, err)
	require.Contains(t, string(payload), `"user_schedule_mode":"allowlist"`)
	require.Contains(t, string(payload), `"quality_min_success_rate":0.95`)
	require.NotContains(t, string(payload), `"account_groups"`)
	require.NotContains(t, string(payload), `"groups"`)
}

func floatPtr(v float64) *float64 { return &v }
