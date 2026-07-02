//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type apiKeyConcurrencyCacheStub struct {
	*stubConcurrencyCacheForTest
	trackErr error
	counts   map[int64]int
	tracked  []int64
	released []int64
}

func (s *apiKeyConcurrencyCacheStub) TrackAPIKeySlot(_ context.Context, apiKeyID int64, _ string) error {
	s.tracked = append(s.tracked, apiKeyID)
	return s.trackErr
}

func (s *apiKeyConcurrencyCacheStub) ReleaseAPIKeySlot(_ context.Context, apiKeyID int64, _ string) error {
	s.released = append(s.released, apiKeyID)
	return nil
}

func (s *apiKeyConcurrencyCacheStub) GetAPIKeyConcurrencyBatch(_ context.Context, _ []int64) (map[int64]int, error) {
	return s.counts, nil
}

func TestAPIKeyConcurrencyStatsTracksAndReleases(t *testing.T) {
	cache := &apiKeyConcurrencyCacheStub{stubConcurrencyCacheForTest: &stubConcurrencyCacheForTest{}, counts: map[int64]int{7: 3}}
	svc := NewConcurrencyService(cache)

	release := svc.TrackAPIKeySlot(context.Background(), 7)
	require.Equal(t, []int64{7}, cache.tracked)
	release()
	require.Equal(t, []int64{7}, cache.released)

	counts, err := svc.GetAPIKeyConcurrencyBatch(context.Background(), []int64{7, 8})
	require.NoError(t, err)
	require.Equal(t, map[int64]int{7: 3, 8: 0}, counts)
}

func TestAPIKeyConcurrencyStatsFailOpen(t *testing.T) {
	cache := &apiKeyConcurrencyCacheStub{stubConcurrencyCacheForTest: &stubConcurrencyCacheForTest{}, trackErr: errors.New("redis down")}
	svc := NewConcurrencyService(cache)

	require.NotPanics(t, svc.TrackAPIKeySlot(context.Background(), 7))
	require.Empty(t, cache.released)
}
