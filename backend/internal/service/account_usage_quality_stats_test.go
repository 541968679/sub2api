package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type qualityStatsBatchRepoStub struct {
	UsageLogRepository
	lastIDs   []int64
	lastStart time.Time
	result    map[int64]*AccountQualityStats
	err       error
}

func (s *qualityStatsBatchRepoStub) GetAccountQualityStatsBatch(ctx context.Context, accountIDs []int64, startTime time.Time) (map[int64]*AccountQualityStats, error) {
	s.lastIDs = append([]int64(nil), accountIDs...)
	s.lastStart = startTime
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func (s *qualityStatsBatchRepoStub) GetUserQualityStatsBatch(ctx context.Context, userIDs []int64, startTime time.Time) (map[int64]*AccountQualityStats, error) {
	return s.GetAccountQualityStatsBatch(ctx, userIDs, startTime)
}

func TestAccountUsageService_GetQualityStatsBatch_DedupCapAndFill(t *testing.T) {
	rate := 1.0
	ttft := 100
	repo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			1: {
				WindowSeconds: AccountQualityWindowSeconds,
				SuccessCount:  3,
				ErrorCount:    0,
				SuccessRate:   &rate,
				AvgTTFTMs:     &ttft,
				TTFTSamples:   3,
			},
		},
	}
	svc := &AccountUsageService{usageLogRepo: repo}

	// include duplicate, non-positive, and an ID with no row from repo
	ids := []int64{1, 1, 0, -2, 2}
	got, err := svc.GetQualityStatsBatch(context.Background(), ids)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, []int64{1, 2}, repo.lastIDs)
	require.WithinDuration(t, time.Now().Add(-AccountQualityWindow), repo.lastStart, 2*time.Second)

	require.NotNil(t, got[1])
	require.Equal(t, int64(3), got[1].SuccessCount)
	require.NotNil(t, got[1].AvgTTFTMs)
	require.Equal(t, 100, *got[1].AvgTTFTMs)

	require.NotNil(t, got[2])
	require.Nil(t, got[2].SuccessRate)
	require.Nil(t, got[2].AvgTTFTMs)
}

func TestAccountUsageService_GetQualityStatsBatch_MaxBatchSize(t *testing.T) {
	repo := &qualityStatsBatchRepoStub{result: map[int64]*AccountQualityStats{}}
	svc := &AccountUsageService{usageLogRepo: repo}

	ids := make([]int64, 0, AccountQualityMaxBatchSize+50)
	for i := int64(1); i <= int64(AccountQualityMaxBatchSize+50); i++ {
		ids = append(ids, i)
	}
	got, err := svc.GetQualityStatsBatch(context.Background(), ids)
	require.NoError(t, err)
	require.Len(t, repo.lastIDs, AccountQualityMaxBatchSize)
	require.Len(t, got, AccountQualityMaxBatchSize)
}

func TestAccountUsageService_GetUserQualityStatsBatch_DedupCapAndFill(t *testing.T) {
	rate := 1.0
	ttft := 100
	repo := &qualityStatsBatchRepoStub{
		result: map[int64]*AccountQualityStats{
			1: {
				WindowSeconds: AccountQualityWindowSeconds,
				SuccessCount:  3,
				ErrorCount:    0,
				SuccessRate:   &rate,
				AvgTTFTMs:     &ttft,
				TTFTSamples:   3,
			},
		},
	}
	svc := &AccountUsageService{usageLogRepo: repo}

	ids := []int64{1, 1, 0, -2, 2}
	got, err := svc.GetUserQualityStatsBatch(context.Background(), ids)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, []int64{1, 2}, repo.lastIDs)
	require.WithinDuration(t, time.Now().Add(-AccountQualityWindow), repo.lastStart, 2*time.Second)

	require.NotNil(t, got[1])
	require.Equal(t, int64(3), got[1].SuccessCount)
	require.NotNil(t, got[1].AvgTTFTMs)
	require.Equal(t, 100, *got[1].AvgTTFTMs)

	require.NotNil(t, got[2])
	require.Nil(t, got[2].SuccessRate)
	require.Nil(t, got[2].AvgTTFTMs)
}

func TestAccountUsageService_GetUserQualityStatsBatch_MaxBatchSize(t *testing.T) {
	repo := &qualityStatsBatchRepoStub{result: map[int64]*AccountQualityStats{}}
	svc := &AccountUsageService{usageLogRepo: repo}

	ids := make([]int64, 0, AccountQualityMaxBatchSize+50)
	for i := int64(1); i <= int64(AccountQualityMaxBatchSize+50); i++ {
		ids = append(ids, i)
	}
	got, err := svc.GetUserQualityStatsBatch(context.Background(), ids)
	require.NoError(t, err)
	require.Len(t, repo.lastIDs, AccountQualityMaxBatchSize)
	require.Len(t, got, AccountQualityMaxBatchSize)
}
