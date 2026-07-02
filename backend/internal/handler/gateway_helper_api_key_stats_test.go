//go:build unit

package handler

import (
	"context"
	"testing"
	"time"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type helperAPIKeyStatsCache struct {
	*helperConcurrencyCacheStub
	tracked  []int64
	released []int64
}

func (s *helperAPIKeyStatsCache) TrackAPIKeySlot(_ context.Context, apiKeyID int64, _ string) error {
	s.tracked = append(s.tracked, apiKeyID)
	return nil
}

func (s *helperAPIKeyStatsCache) ReleaseAPIKeySlot(_ context.Context, apiKeyID int64, _ string) error {
	s.released = append(s.released, apiKeyID)
	return nil
}

func (s *helperAPIKeyStatsCache) GetAPIKeyConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	return make(map[int64]int, len(ids)), nil
}

func TestAcquireUserSlotWithWaitTracksAPIKeyStats(t *testing.T) {
	cache := &helperAPIKeyStatsCache{helperConcurrencyCacheStub: &helperConcurrencyCacheStub{userSeq: []bool{true}}}
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 5*time.Millisecond)
	c, _ := newHelperTestContext("POST", "/v1/messages")
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 77})
	streamStarted := false

	release, err := helper.AcquireUserSlotWithWait(c, 2, 3, false, &streamStarted)
	require.NoError(t, err)
	require.Equal(t, []int64{77}, cache.tracked)
	release()
	require.Equal(t, []int64{77}, cache.released)
	require.Equal(t, 1, cache.userReleaseCalls)
}
