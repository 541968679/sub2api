//go:build unit

package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	entsql "entgo.io/ent/dialect/sql"
)

func TestIsSubscriptionUsageMetricSort(t *testing.T) {
	require.True(t, isSubscriptionUsageMetricSort("total_consumed"))
	require.True(t, isSubscriptionUsageMetricSort("avg_daily"))
	require.True(t, isSubscriptionUsageMetricSort("usage_rate"))
	require.True(t, isSubscriptionUsageMetricSort("TOTAL_CONSUMED_USD"))
	require.False(t, isSubscriptionUsageMetricSort("expires_at"))
	require.False(t, isSubscriptionUsageMetricSort(""))
}

func TestSubscriptionUsageMetricOrder_BuildsExprs(t *testing.T) {
	cases := []struct {
		sortBy   string
		asc      bool
		wantFrag string
	}{
		{sortBy: "total_consumed", asc: false, wantFrag: "created_at >="},
		{sortBy: "avg_daily", asc: true, wantFrag: "GREATEST(1, FLOOR(EXTRACT(EPOCH"},
		{sortBy: "usage_rate", asc: false, wantFrag: "LEAST(1.0"},
	}
	for _, tc := range cases {
		t.Run(tc.sortBy, func(t *testing.T) {
			opts := subscriptionUsageMetricOrder(tc.sortBy, tc.asc)
			require.Len(t, opts, 1)

			// Build a minimal selector with the user_subscriptions table columns.
			t1 := entsql.Table("user_subscriptions")
			sel := entsql.Select(t1.C("id"), t1.C("starts_at"), t1.C("expires_at"), t1.C("group_id")).From(t1)
			// OrderOption expects field names matching ent schema; C() uses bare names when From is set.
			// The order func uses s.C(field) which qualifies against the main table.
			opts[0](sel)
			query, args := sel.Query()
			_ = args
			require.NotEmpty(t, query)
			require.Contains(t, query, tc.wantFrag)
			if tc.asc {
				require.True(t, strings.Contains(strings.ToUpper(query), " ASC") || strings.Contains(query, " ASC "))
			} else {
				require.Contains(t, strings.ToUpper(query), "DESC")
			}
		})
	}
}
