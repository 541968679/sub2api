package admin

import (
	"strconv"
	"strings"
	"time"
)

var accountTodayStatsBatchCache = newSnapshotCache(30 * time.Second)
var accountQualityStatsBatchCache = newSnapshotCache(30 * time.Second)
var userQualityStatsBatchCache = newSnapshotCache(30 * time.Second)

func buildAccountTodayStatsBatchCacheKey(accountIDs []int64) string {
	if len(accountIDs) == 0 {
		return "accounts_today_stats_empty"
	}
	var b strings.Builder
	b.Grow(len(accountIDs) * 6)
	_, _ = b.WriteString("accounts_today_stats:")
	for i, id := range accountIDs {
		if i > 0 {
			_ = b.WriteByte(',')
		}
		_, _ = b.WriteString(strconv.FormatInt(id, 10))
	}
	return b.String()
}

func buildAccountQualityStatsBatchCacheKey(accountIDs []int64, useFailover bool) string {
	if len(accountIDs) == 0 {
		return "accounts_quality_stats_empty"
	}
	var b strings.Builder
	b.Grow(len(accountIDs)*6 + 40)
	_, _ = b.WriteString("accounts_quality_stats:15m:")
	if useFailover {
		_, _ = b.WriteString("failover:")
	}
	for i, id := range accountIDs {
		if i > 0 {
			_ = b.WriteByte(',')
		}
		_, _ = b.WriteString(strconv.FormatInt(id, 10))
	}
	return b.String()
}

func buildUserQualityStatsBatchCacheKey(userIDs []int64, useFailover bool) string {
	if len(userIDs) == 0 {
		return "users_quality_stats_empty"
	}
	var b strings.Builder
	b.Grow(len(userIDs)*6 + 40)
	_, _ = b.WriteString("users_quality_stats:last-n:")
	if useFailover {
		_, _ = b.WriteString("failover:")
	}
	for i, id := range userIDs {
		if i > 0 {
			_ = b.WriteByte(',')
		}
		_, _ = b.WriteString(strconv.FormatInt(id, 10))
	}
	return b.String()
}
