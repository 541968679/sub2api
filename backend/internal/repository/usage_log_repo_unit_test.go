//go:build unit

package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSafeDateFormat(t *testing.T) {
	tests := []struct {
		name        string
		granularity string
		expected    string
	}{
		// 合法值
		{"hour", "hour", "YYYY-MM-DD HH24:00"},
		{"day", "day", "YYYY-MM-DD"},
		{"week", "week", "IYYY-IW"},
		{"month", "month", "YYYY-MM"},

		// 非法值回退到默认
		{"空字符串", "", "YYYY-MM-DD"},
		{"未知粒度 year", "year", "YYYY-MM-DD"},
		{"未知粒度 minute", "minute", "YYYY-MM-DD"},

		// 恶意字符串
		{"SQL 注入尝试", "'; DROP TABLE users; --", "YYYY-MM-DD"},
		{"带引号", "day'", "YYYY-MM-DD"},
		{"带括号", "day)", "YYYY-MM-DD"},
		{"Unicode", "日", "YYYY-MM-DD"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := safeDateFormat(tc.granularity)
			require.Equal(t, tc.expected, got, "safeDateFormat(%q)", tc.granularity)
		})
	}
}

func TestBuildUsageLogBatchInsertQuery_UsesConflictDoNothing(t *testing.T) {
	log := &service.UsageLog{
		UserID:       1,
		APIKeyID:     2,
		AccountID:    3,
		RequestID:    "req-batch-no-update",
		Model:        "gpt-5",
		InputTokens:  10,
		OutputTokens: 5,
		TotalCost:    1.2,
		ActualCost:   1.2,
		CreatedAt:    time.Now().UTC(),
	}
	prepared := prepareUsageLogInsert(log)

	query, _ := buildUsageLogBatchInsertQuery([]string{usageLogBatchKey(log.RequestID, log.APIKeyID)}, map[string]usageLogInsertPrepared{
		usageLogBatchKey(log.RequestID, log.APIKeyID): prepared,
	})

	require.Contains(t, query, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	require.NotContains(t, strings.ToUpper(query), "DO UPDATE")
}

func TestUsageLogRepositoryCreateBestEffort_QueueFullWaitsForDrain(t *testing.T) {
	repo := &usageLogRepository{
		db:                &sql.DB{},
		bestEffortBatchCh: make(chan usageLogBestEffortRequest, 1),
	}
	repo.bestEffortBatchCh <- usageLogBestEffortRequest{}

	go func() {
		time.Sleep(50 * time.Millisecond)
		<-repo.bestEffortBatchCh
		req := <-repo.bestEffortBatchCh
		req.resultCh <- nil
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := repo.CreateBestEffort(ctx, &service.UsageLog{
		UserID: 1, APIKeyID: 2, AccountID: 3, Model: "gpt-5", CreatedAt: time.Now().UTC(),
	})

	require.NoError(t, err)
}

func TestUsageLogRepositoryCreate_QueueFullWaitsForDrain(t *testing.T) {
	repo := &usageLogRepository{
		db:            &sql.DB{},
		createBatchCh: make(chan usageLogCreateRequest, 1),
	}
	repo.createBatchCh <- usageLogCreateRequest{}

	go func() {
		time.Sleep(50 * time.Millisecond)
		<-repo.createBatchCh
		req := <-repo.createBatchCh
		completeUsageLogCreateRequest(req, usageLogCreateResult{inserted: true})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	inserted, err := repo.Create(ctx, &service.UsageLog{
		UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: "req-queue-drain", Model: "gpt-5", CreatedAt: time.Now().UTC(),
	})

	require.NoError(t, err)
	require.True(t, inserted)
}
