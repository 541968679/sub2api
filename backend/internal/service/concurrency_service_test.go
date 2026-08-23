//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// stubConcurrencyCacheForTest 用于并发服务单元测试的缓存桩
type stubConcurrencyCacheForTest struct {
	acquireResult     bool
	acquireErr        error
	releaseErr        error
	concurrency       int
	concurrencyErr    error
	waitAllowed       bool
	waitErr           error
	waitCount         int
	waitCountErr      error
	loadBatch         map[int64]*AccountLoadInfo
	loadBatchErr      error
	usersLoadBatch    map[int64]*UserLoadInfo
	usersLoadErr      error
	cleanupErr        error
	apiKeyConcurrency map[int64]int

	// 记录调用
	releasedAccountIDs []int64
	releasedRequestIDs []string
}

var _ ConcurrencyCache = (*stubConcurrencyCacheForTest)(nil)

func (c *stubConcurrencyCacheForTest) AcquireAccountSlot(_ context.Context, _ int64, _ int, _ string) (bool, error) {
	return c.acquireResult, c.acquireErr
}
func (c *stubConcurrencyCacheForTest) ReleaseAccountSlot(_ context.Context, accountID int64, requestID string) error {
	c.releasedAccountIDs = append(c.releasedAccountIDs, accountID)
	c.releasedRequestIDs = append(c.releasedRequestIDs, requestID)
	return c.releaseErr
}
func (c *stubConcurrencyCacheForTest) GetAccountConcurrency(_ context.Context, _ int64) (int, error) {
	return c.concurrency, c.concurrencyErr
}
func (c *stubConcurrencyCacheForTest) GetAccountConcurrencyBatch(_ context.Context, accountIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		if c.concurrencyErr != nil {
			return nil, c.concurrencyErr
		}
		result[accountID] = c.concurrency
	}
	return result, nil
}
func (c *stubConcurrencyCacheForTest) IncrementAccountWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	return c.waitAllowed, c.waitErr
}
func (c *stubConcurrencyCacheForTest) DecrementAccountWaitCount(_ context.Context, _ int64) error {
	return nil
}
func (c *stubConcurrencyCacheForTest) GetAccountWaitingCount(_ context.Context, _ int64) (int, error) {
	return c.waitCount, c.waitCountErr
}
func (c *stubConcurrencyCacheForTest) AcquireUserSlot(_ context.Context, _ int64, _ int, _ string) (bool, error) {
	return c.acquireResult, c.acquireErr
}
func (c *stubConcurrencyCacheForTest) ReleaseUserSlot(_ context.Context, _ int64, _ string) error {
	return c.releaseErr
}
func (c *stubConcurrencyCacheForTest) GetUserConcurrency(_ context.Context, _ int64) (int, error) {
	return c.concurrency, c.concurrencyErr
}
func (c *stubConcurrencyCacheForTest) AcquireAccountUserSlot(_ context.Context, _ int64, _ int64, _ int, _ string) (bool, error) {
	return c.acquireResult, c.acquireErr
}
func (c *stubConcurrencyCacheForTest) ReleaseAccountUserSlot(_ context.Context, _ int64, _ int64, _ string) error {
	return c.releaseErr
}
func (c *stubConcurrencyCacheForTest) GetAccountUserConcurrencyBatch(_ context.Context, accountIDs []int64, _ int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = c.concurrency
	}
	return result, c.concurrencyErr
}
func (c *stubConcurrencyCacheForTest) IncrementWaitCount(_ context.Context, _ int64, _ int) (bool, error) {
	return c.waitAllowed, c.waitErr
}
func (c *stubConcurrencyCacheForTest) DecrementWaitCount(_ context.Context, _ int64) error {
	return nil
}
func (c *stubConcurrencyCacheForTest) GetAccountsLoadBatch(_ context.Context, _ []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	return c.loadBatch, c.loadBatchErr
}
func (c *stubConcurrencyCacheForTest) GetUsersLoadBatch(_ context.Context, _ []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	return c.usersLoadBatch, c.usersLoadErr
}
func (c *stubConcurrencyCacheForTest) CleanupExpiredAccountSlots(_ context.Context, _ int64) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) ClearAccountSlots(_ context.Context, _ int64) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) CleanupExpiredAccountSlotKeys(_ context.Context) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) CleanupStaleProcessSlots(_ context.Context, _ string) error {
	return c.cleanupErr
}

func (c *stubConcurrencyCacheForTest) TrackAPIKeySlot(_ context.Context, _ int64, _ string) error {
	return nil
}

func (c *stubConcurrencyCacheForTest) ReleaseAPIKeySlot(_ context.Context, _ int64, _ string) error {
	return nil
}

func (c *stubConcurrencyCacheForTest) GetAPIKeyConcurrencyBatch(_ context.Context, ids []int64) (map[int64]int, error) {
	out := make(map[int64]int, len(ids))
	for _, id := range ids {
		out[id] = c.apiKeyConcurrency[id]
	}
	return out, nil
}

type trackingConcurrencyCache struct {
	stubConcurrencyCacheForTest
	cleanupPrefix string
}

func (c *trackingConcurrencyCache) CleanupStaleProcessSlots(_ context.Context, prefix string) error {
	c.cleanupPrefix = prefix
	return c.cleanupErr
}

func TestCleanupStaleProcessSlots_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}
	require.NoError(t, svc.CleanupStaleProcessSlots(context.Background()))
}

func TestCleanupStaleProcessSlots_DelegatesPrefix(t *testing.T) {
	cache := &trackingConcurrencyCache{}
	svc := NewConcurrencyService(cache)
	require.NoError(t, svc.CleanupStaleProcessSlots(context.Background()))
	require.Equal(t, RequestIDPrefix(), cache.cleanupPrefix)
}

func TestAcquireAccountSlot_Success(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
}

func TestAcquireAccountSlot_Failure(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: false}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.NoError(t, err)
	require.False(t, result.Acquired)
	require.Nil(t, result.ReleaseFunc)
}

func TestAcquireAccountSlot_UnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(&stubConcurrencyCacheForTest{})

	for _, maxConcurrency := range []int{0, -1} {
		result, err := svc.AcquireAccountSlot(context.Background(), 1, maxConcurrency)
		require.NoError(t, err)
		require.True(t, result.Acquired, "maxConcurrency=%d 应无限制通过", maxConcurrency)
		require.NotNil(t, result.ReleaseFunc, "ReleaseFunc 应为 no-op 函数")
	}
}

func TestAcquireAccountSlot_CacheError(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireErr: errors.New("redis down")}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 1, 5)
	require.Error(t, err)
	require.Nil(t, result)
}

func TestAcquireAccountSlot_ReleaseDecrements(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountSlot(context.Background(), 42, 5)
	require.NoError(t, err)
	require.True(t, result.Acquired)

	// 调用 ReleaseFunc 应释放槽位
	result.ReleaseFunc()

	require.Len(t, cache.releasedAccountIDs, 1)
	require.Equal(t, int64(42), cache.releasedAccountIDs[0])
	require.Len(t, cache.releasedRequestIDs, 1)
	require.NotEmpty(t, cache.releasedRequestIDs[0], "requestID 不应为空")
}

func TestAcquireUserSlot_IndependentFromAccount(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{acquireResult: true}
	svc := NewConcurrencyService(cache)

	// 用户槽位获取应独立于账户槽位
	result, err := svc.AcquireUserSlot(context.Background(), 100, 3)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.NotNil(t, result.ReleaseFunc)
}

func TestAcquireUserSlot_UnlimitedConcurrency(t *testing.T) {
	svc := NewConcurrencyService(&stubConcurrencyCacheForTest{})

	result, err := svc.AcquireUserSlot(context.Background(), 1, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
}

func TestAcquireAccountUserSlot_CountOnlyWritesAndReleases(t *testing.T) {
	cache := &pairOccupancyCache{stubConcurrencyCacheForTest: stubConcurrencyCacheForTest{acquireResult: true}}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountUserSlot(context.Background(), 11, 16, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, []int{0}, cache.maxes())
	require.Equal(t, 1, cache.count(11))
	require.NotEqual(t, 999, cache.maxes()[0])

	result.ReleaseFunc()
	require.Equal(t, 0, cache.count(11))
}

func TestAcquireAccountUserSlot_InvalidIDsSkipRedis(t *testing.T) {
	cache := &pairOccupancyCache{}
	svc := NewConcurrencyService(cache)

	result, err := svc.AcquireAccountUserSlot(context.Background(), 0, 16, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Empty(t, cache.maxes())
}

func TestAcquireAccountUserSlot_ReusesInboundRequestID(t *testing.T) {
	cache := &pairOccupancyCache{stubConcurrencyCacheForTest: stubConcurrencyCacheForTest{acquireResult: true}}
	svc := NewConcurrencyService(cache)
	ctx := context.WithValue(context.Background(), ctxkey.RequestID, "http-req-16")

	first, err := svc.AcquireAccountUserSlot(ctx, 11, 16, 0)
	require.NoError(t, err)
	require.True(t, first.Acquired)
	second, err := svc.AcquireAccountUserSlot(ctx, 11, 16, 0)
	require.NoError(t, err)
	require.True(t, second.Acquired)
	require.Equal(t, 1, cache.count(11), "retry/failover must refresh the same pair member, not leak a second ID")

	first.ReleaseFunc()
	require.Equal(t, 1, cache.count(11), "shared pair member stays until the last holder releases")
	second.ReleaseFunc()
	require.Equal(t, 0, cache.count(11))
}

func TestAcquireAccountUserSlot_ContextDoneReleasesWithoutCaller(t *testing.T) {
	cache := &pairOccupancyCache{stubConcurrencyCacheForTest: stubConcurrencyCacheForTest{acquireResult: true}}
	svc := NewConcurrencyService(cache)
	ctx, cancel := context.WithCancel(context.Background())

	result, err := svc.AcquireAccountUserSlot(ctx, 11, 16, 0)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.Equal(t, 1, cache.count(11))

	cancel()
	require.Eventually(t, func() bool {
		return cache.count(11) == 0
	}, time.Second, 10*time.Millisecond, "request ctx cancel must ZREM even when ReleaseFunc is skipped")
}

type platformAwarePairCache struct {
	stubConcurrencyCacheForTest
	lastPlatform string
}

func (c *platformAwarePairCache) GetAccountUserConcurrencyBatch(ctx context.Context, accountIDs []int64, _ int64) (map[int64]int, error) {
	if plat, ok := ctx.Value(ctxkey.ScheduleLookupPlatform).(string); ok {
		c.lastPlatform = plat
	}
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = 3
	}
	return out, nil
}

func TestGetAccountUserConcurrencyBatch_PreservesLookupPlatform(t *testing.T) {
	cache := &platformAwarePairCache{}
	svc := NewConcurrencyService(cache)
	ctx := WithScheduleLookupPlatform(context.Background(), PlatformOpenAI)

	counts, err := svc.GetAccountUserConcurrencyBatch(ctx, []int64{1730}, 16)
	require.NoError(t, err)
	require.Equal(t, 3, counts[1730])
	require.Equal(t, PlatformOpenAI, cache.lastPlatform, "detached Redis ctx must keep openai vs _ pairing shard")
}

func TestGenerateRequestID_UsesStablePrefixAndMonotonicCounter(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()
	require.NotEmpty(t, id1)
	require.NotEmpty(t, id2)

	p1 := strings.Split(id1, "-")
	p2 := strings.Split(id2, "-")
	require.Len(t, p1, 2)
	require.Len(t, p2, 2)
	require.Equal(t, p1[0], p2[0], "同一进程前缀应保持一致")

	n1, err := strconv.ParseUint(p1[1], 36, 64)
	require.NoError(t, err)
	n2, err := strconv.ParseUint(p2[1], 36, 64)
	require.NoError(t, err)
	require.Equal(t, n1+1, n2, "计数器应单调递增")
}

func TestGetAccountsLoadBatch_ReturnsCorrectData(t *testing.T) {
	expected := map[int64]*AccountLoadInfo{
		1: {AccountID: 1, CurrentConcurrency: 3, WaitingCount: 0, LoadRate: 60},
		2: {AccountID: 2, CurrentConcurrency: 5, WaitingCount: 2, LoadRate: 100},
	}
	cache := &stubConcurrencyCacheForTest{loadBatch: expected}
	svc := NewConcurrencyService(cache)

	accounts := []AccountWithConcurrency{
		{ID: 1, MaxConcurrency: 5},
		{ID: 2, MaxConcurrency: 5},
	}
	result, err := svc.GetAccountsLoadBatch(context.Background(), accounts)
	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestGetAccountsLoadBatch_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	result, err := svc.GetAccountsLoadBatch(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestIncrementWaitCount_Success(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitAllowed: true}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestIncrementWaitCount_QueueFull(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitAllowed: false}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestIncrementWaitCount_FailOpen(t *testing.T) {
	// Redis 错误时应 fail-open（允许请求通过）
	cache := &stubConcurrencyCacheForTest{waitErr: errors.New("redis timeout")}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err, "Redis 错误不应传播")
	require.True(t, allowed, "Redis 错误时应 fail-open")
}

func TestIncrementWaitCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	allowed, err := svc.IncrementWaitCount(context.Background(), 1, 25)
	require.NoError(t, err)
	require.True(t, allowed, "nil cache 应 fail-open")
}

func TestCalculateMaxWait(t *testing.T) {
	tests := []struct {
		concurrency int
		expected    int
	}{
		{5, 25},  // 5 + 20
		{1, 21},  // 1 + 20
		{0, 21},  // min(1) + 20
		{-1, 21}, // min(1) + 20
		{10, 30}, // 10 + 20
	}
	for _, tt := range tests {
		result := CalculateMaxWait(tt.concurrency)
		require.Equal(t, tt.expected, result, "CalculateMaxWait(%d)", tt.concurrency)
	}
}

func TestGetAccountWaitingCount(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitCount: 5}
	svc := NewConcurrencyService(cache)

	count, err := svc.GetAccountWaitingCount(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 5, count)
}

func TestGetAccountWaitingCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	count, err := svc.GetAccountWaitingCount(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestGetAccountConcurrencyBatch(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{concurrency: 3}
	svc := NewConcurrencyService(cache)

	result, err := svc.GetAccountConcurrencyBatch(context.Background(), []int64{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, result, 3)
	for _, id := range []int64{1, 2, 3} {
		require.Equal(t, 3, result[id])
	}
}

func TestIncrementAccountWaitCount_FailOpen(t *testing.T) {
	cache := &stubConcurrencyCacheForTest{waitErr: errors.New("redis error")}
	svc := NewConcurrencyService(cache)

	allowed, err := svc.IncrementAccountWaitCount(context.Background(), 1, 10)
	require.NoError(t, err, "Redis 错误不应传播")
	require.True(t, allowed, "Redis 错误时应 fail-open")
}

func TestIncrementAccountWaitCount_NilCache(t *testing.T) {
	svc := &ConcurrencyService{cache: nil}

	allowed, err := svc.IncrementAccountWaitCount(context.Background(), 1, 10)
	require.NoError(t, err)
	require.True(t, allowed)
}
