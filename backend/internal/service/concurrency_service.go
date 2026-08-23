package service

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// ConcurrencyCache 定义并发控制的缓存接口
// 使用有序集合存储槽位，按时间戳清理过期条目
type ConcurrencyCache interface {
	// 账号槽位管理
	// 键格式: concurrency:account:{accountID}（有序集合，成员为 requestID）
	AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseAccountSlot(ctx context.Context, accountID int64, requestID string) error
	GetAccountConcurrency(ctx context.Context, accountID int64) (int, error)
	GetAccountConcurrencyBatch(ctx context.Context, accountIDs []int64) (map[int64]int, error)

	// 账号等待队列（账号级）
	IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error)
	DecrementAccountWaitCount(ctx context.Context, accountID int64) error
	GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error)

	// 用户槽位管理
	// 键格式: concurrency:user:{userID}（有序集合，成员为 requestID）
	AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseUserSlot(ctx context.Context, userID int64, requestID string) error
	GetUserConcurrency(ctx context.Context, userID int64) (int, error)

	// 账号-用户对级槽位
	// 键格式: concurrency:account_user:{accountID}:{userID}:{platform}
	AcquireAccountUserSlot(ctx context.Context, accountID, userID int64, maxConcurrency int, requestID string) (bool, error)
	ReleaseAccountUserSlot(ctx context.Context, accountID, userID int64, requestID string) error
	GetAccountUserConcurrencyBatch(ctx context.Context, accountIDs []int64, userID int64) (map[int64]int, error)

	// 等待队列计数（只在首次创建时设置 TTL）
	IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error)
	DecrementWaitCount(ctx context.Context, userID int64) error

	// 批量负载查询（只读）
	GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error)
	GetUsersLoadBatch(ctx context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error)

	// 清理过期槽位（后台任务）
	CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error
	CleanupExpiredAccountSlotKeys(ctx context.Context) error

	// 启动时清理旧进程遗留槽位与等待计数
	CleanupStaleProcessSlots(ctx context.Context, activeRequestPrefix string) error

	// ClearAccountSlots 删除账号全部并发槽与账号级等待计数（运维清理卡住请求）
	ClearAccountSlots(ctx context.Context, accountID int64) error
}

// APIKeyConcurrencyCache is an optional stats-only capability. API-key slots
// never participate in admission control; older cache implementations remain valid.
type APIKeyConcurrencyCache interface {
	TrackAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error
	ReleaseAPIKeySlot(ctx context.Context, apiKeyID int64, requestID string) error
	GetAPIKeyConcurrencyBatch(ctx context.Context, apiKeyIDs []int64) (map[int64]int, error)
}

var (
	requestIDPrefix  = initRequestIDPrefix()
	requestIDCounter atomic.Uint64
)

func initRequestIDPrefix() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return "r" + strconv.FormatUint(binary.BigEndian.Uint64(b), 36)
	}
	fallback := uint64(time.Now().UnixNano()) ^ (uint64(os.Getpid()) << 16)
	return "r" + strconv.FormatUint(fallback, 36)
}

func RequestIDPrefix() string {
	return requestIDPrefix
}

func generateRequestID() string {
	seq := requestIDCounter.Add(1)
	return requestIDPrefix + "-" + strconv.FormatUint(seq, 36)
}

func withSlotOwnerPrefix(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxkey.SlotOwnerPrefix, RequestIDPrefix())
}

func pairSlotMemberID(ctx context.Context) string {
	if ctx != nil {
		if rid, _ := ctx.Value(ctxkey.RequestID).(string); strings.TrimSpace(rid) != "" {
			return RequestIDPrefix() + "-" + strings.TrimSpace(rid)
		}
	}
	return generateRequestID()
}

func pairHoldPlatform(ctx context.Context) string {
	if ctx != nil {
		if plat, ok := ctx.Value(ctxkey.ScheduleLookupPlatform).(string); ok {
			return SmartScheduleRedisPlatform(plat)
		}
	}
	return SmartScheduleRedisPlatform("")
}

func (s *ConcurrencyService) addPairHold(key pairHoldKey) {
	if s == nil {
		return
	}
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	if s.pairHolds == nil {
		s.pairHolds = map[pairHoldKey]int{}
	}
	s.pairHolds[key]++
}

func (s *ConcurrencyService) releasePairHold(key pairHoldKey) bool {
	if s == nil {
		return true
	}
	s.pairMu.Lock()
	defer s.pairMu.Unlock()
	n := s.pairHolds[key] - 1
	if n <= 0 {
		delete(s.pairHolds, key)
		return true
	}
	s.pairHolds[key] = n
	return false
}

func detachPairRedisCtx(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	ctx = withSlotOwnerPrefix(ctx)
	if parent != nil {
		if plat, ok := parent.Value(ctxkey.ScheduleLookupPlatform).(string); ok && strings.TrimSpace(plat) != "" {
			ctx = context.WithValue(ctx, ctxkey.ScheduleLookupPlatform, plat)
		}
	}
	return ctx, cancel
}

const pairSlotTouchInterval = 30 * time.Second

func startPairSlotTouch(ctx context.Context, cache ConcurrencyCache, accountID, userID int64, maxConcurrency int, requestID string) func() {
	if cache == nil || ctx == nil || ctx.Done() == nil {
		return func() {}
	}
	var mu sync.Mutex
	var timer *time.Timer
	stopped := false
	var schedule func()
	stop := func() {
		mu.Lock()
		defer mu.Unlock()
		stopped = true
		if timer != nil {
			timer.Stop()
		}
	}
	schedule = func() {
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		timer = time.AfterFunc(pairSlotTouchInterval, func() {
			if ctx.Err() != nil {
				return
			}
			touchCtx, cancel := context.WithTimeout(withSlotOwnerPrefix(context.Background()), 2*time.Second)
			if plat, ok := ctx.Value(ctxkey.ScheduleLookupPlatform).(string); ok && strings.TrimSpace(plat) != "" {
				touchCtx = context.WithValue(touchCtx, ctxkey.ScheduleLookupPlatform, plat)
			}
			_, _ = cache.AcquireAccountUserSlot(touchCtx, accountID, userID, maxConcurrency, requestID)
			cancel()
			schedule()
		})
		mu.Unlock()
	}
	schedule()
	return stop
}

func (s *ConcurrencyService) CleanupStaleProcessSlots(ctx context.Context) error {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.CleanupStaleProcessSlots(ctx, RequestIDPrefix())
}

// ClearAccountSlots drops all concurrency slots and wait counters for an account.
func (s *ConcurrencyService) ClearAccountSlots(ctx context.Context, accountID int64) error {
	if s == nil || s.cache == nil || accountID <= 0 {
		return nil
	}
	return s.cache.ClearAccountSlots(ctx, accountID)
}

const (
	// Default extra wait slots beyond concurrency limit
	defaultExtraWaitSlots         = 20
	apiKeyConcurrencyFetchTimeout = 3 * time.Second
	apiKeySlotTrackTimeout        = 2 * time.Second
)

type pairHoldKey struct {
	accountID int64
	userID    int64
	platform  string
	requestID string
}

// ConcurrencyService manages concurrent request limiting for accounts and users
type ConcurrencyService struct {
	cache     ConcurrencyCache
	pairMu    sync.Mutex
	pairHolds map[pairHoldKey]int
}

// NewConcurrencyService creates a new ConcurrencyService
func NewConcurrencyService(cache ConcurrencyCache) *ConcurrencyService {
	return &ConcurrencyService{
		cache:     cache,
		pairHolds: map[pairHoldKey]int{},
	}
}

// AcquireResult represents the result of acquiring a concurrency slot
type AcquireResult struct {
	Acquired    bool
	ReleaseFunc func() // Must be called when done (typically via defer)
}

type AccountWithConcurrency struct {
	ID             int64
	MaxConcurrency int
}

type UserWithConcurrency struct {
	ID             int64
	MaxConcurrency int
}

type AccountLoadInfo struct {
	AccountID          int64
	CurrentConcurrency int
	WaitingCount       int
	LoadRate           int // 0-100+ (percent)
}

type UserLoadInfo struct {
	UserID             int64
	CurrentConcurrency int
	WaitingCount       int
	LoadRate           int // 0-100+ (percent)
}

// AcquireAccountSlot attempts to acquire a concurrency slot for an account.
// If the account is at max concurrency, it waits until a slot is available or timeout.
// Returns a release function that MUST be called when the request completes.
func (s *ConcurrencyService) AcquireAccountSlot(ctx context.Context, accountID int64, maxConcurrency int) (*AcquireResult, error) {
	// If maxConcurrency is 0 or negative, no limit
	if maxConcurrency <= 0 {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {}, // no-op
		}, nil
	}

	// Generate unique request ID for this slot
	requestID := generateRequestID()

	acquired, err := s.cache.AcquireAccountSlot(ctx, accountID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}

	if acquired {
		return &AcquireResult{
			Acquired: true,
			ReleaseFunc: func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := s.cache.ReleaseAccountSlot(bgCtx, accountID, requestID); err != nil {
					logger.LegacyPrintf("service.concurrency", "Warning: failed to release account slot for %d (req=%s): %v", accountID, requestID, err)
				}
			},
		}, nil
	}

	return &AcquireResult{
		Acquired:    false,
		ReleaseFunc: nil,
	}, nil
}

// AcquireUserSlot attempts to acquire a concurrency slot for a user.
// If the user is at max concurrency, it waits until a slot is available or timeout.
// Returns a release function that MUST be called when the request completes.
func (s *ConcurrencyService) AcquireUserSlot(ctx context.Context, userID int64, maxConcurrency int) (*AcquireResult, error) {
	// If maxConcurrency is 0 or negative, no limit
	if maxConcurrency <= 0 {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {}, // no-op
		}, nil
	}

	// Generate unique request ID for this slot
	requestID := generateRequestID()

	acquired, err := s.cache.AcquireUserSlot(ctx, userID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}

	if acquired {
		return &AcquireResult{
			Acquired: true,
			ReleaseFunc: func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := s.cache.ReleaseUserSlot(bgCtx, userID, requestID); err != nil {
					logger.LegacyPrintf("service.concurrency", "Warning: failed to release user slot for %d (req=%s): %v", userID, requestID, err)
				}
			},
		}, nil
	}

	return &AcquireResult{
		Acquired:    false,
		ReleaseFunc: nil,
	}, nil
}

// TrackAPIKeySlot records a best-effort stats-only slot and always fails open.
func (s *ConcurrencyService) TrackAPIKeySlot(ctx context.Context, apiKeyID int64) func() {
	if s == nil || s.cache == nil || apiKeyID <= 0 {
		return func() {}
	}
	cache, ok := s.cache.(APIKeyConcurrencyCache)
	if !ok {
		return func() {}
	}

	requestID := generateRequestID()
	baseCtx := context.Background()
	if ctx != nil {
		baseCtx = context.WithoutCancel(ctx)
	}
	trackCtx, cancel := context.WithTimeout(baseCtx, apiKeySlotTrackTimeout)
	err := cache.TrackAPIKeySlot(trackCtx, apiKeyID, requestID)
	cancel()
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: failed to track api key slot for %d (req=%s): %v", apiKeyID, requestID, err)
		return func() {}
	}

	return func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := cache.ReleaseAPIKeySlot(bgCtx, apiKeyID, requestID); err != nil {
			logger.LegacyPrintf("service.concurrency", "Warning: failed to release api key slot for %d (req=%s): %v", apiKeyID, requestID, err)
		}
	}
}

// GetAPIKeyConcurrencyBatch returns best-effort counts; Redis failures are
// represented as zeroes because this metric must not break key management.
func (s *ConcurrencyService) GetAPIKeyConcurrencyBatch(_ context.Context, apiKeyIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int, len(apiKeyIDs))
	for _, apiKeyID := range apiKeyIDs {
		result[apiKeyID] = 0
	}
	if len(apiKeyIDs) == 0 || s == nil || s.cache == nil {
		return result, nil
	}
	cache, ok := s.cache.(APIKeyConcurrencyCache)
	if !ok {
		return result, nil
	}

	redisCtx, cancel := context.WithTimeout(context.Background(), apiKeyConcurrencyFetchTimeout)
	defer cancel()
	counts, err := cache.GetAPIKeyConcurrencyBatch(redisCtx, apiKeyIDs)
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: get api key concurrency batch failed: %v", err)
		return result, nil
	}
	for _, apiKeyID := range apiKeyIDs {
		result[apiKeyID] = counts[apiKeyID]
	}
	return result, nil
}

// ============================================
// Wait Queue Count Methods
// ============================================

// IncrementWaitCount attempts to increment the wait queue counter for a user.
// Returns true if successful, false if the wait queue is full.
// maxWait should be user.Concurrency + defaultExtraWaitSlots
func (s *ConcurrencyService) IncrementWaitCount(ctx context.Context, userID int64, maxWait int) (bool, error) {
	if s.cache == nil {
		// Redis not available, allow request
		return true, nil
	}

	result, err := s.cache.IncrementWaitCount(ctx, userID, maxWait)
	if err != nil {
		// On error, allow the request to proceed (fail open)
		logger.LegacyPrintf("service.concurrency", "Warning: increment wait count failed for user %d: %v", userID, err)
		return true, nil
	}
	return result, nil
}

// DecrementWaitCount decrements the wait queue counter for a user.
// Should be called when a request completes or exits the wait queue.
func (s *ConcurrencyService) DecrementWaitCount(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}

	// Use background context to ensure decrement even if original context is cancelled
	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.cache.DecrementWaitCount(bgCtx, userID); err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: decrement wait count failed for user %d: %v", userID, err)
	}
}

// IncrementAccountWaitCount increments the wait queue counter for an account.
func (s *ConcurrencyService) IncrementAccountWaitCount(ctx context.Context, accountID int64, maxWait int) (bool, error) {
	if s.cache == nil {
		return true, nil
	}

	result, err := s.cache.IncrementAccountWaitCount(ctx, accountID, maxWait)
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: increment wait count failed for account %d: %v", accountID, err)
		return true, nil
	}
	return result, nil
}

// DecrementAccountWaitCount decrements the wait queue counter for an account.
func (s *ConcurrencyService) DecrementAccountWaitCount(ctx context.Context, accountID int64) {
	if s.cache == nil {
		return
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.cache.DecrementAccountWaitCount(bgCtx, accountID); err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: decrement wait count failed for account %d: %v", accountID, err)
	}
}

// GetAccountWaitingCount gets current wait queue count for an account.
func (s *ConcurrencyService) GetAccountWaitingCount(ctx context.Context, accountID int64) (int, error) {
	if s.cache == nil {
		return 0, nil
	}
	return s.cache.GetAccountWaitingCount(ctx, accountID)
}

// CalculateMaxWait calculates the maximum wait queue size for a user
// maxWait = userConcurrency + defaultExtraWaitSlots
func CalculateMaxWait(userConcurrency int) int {
	if userConcurrency <= 0 {
		userConcurrency = 1
	}
	return userConcurrency + defaultExtraWaitSlots
}

// GetAccountsLoadBatch returns load info for multiple accounts.
func (s *ConcurrencyService) GetAccountsLoadBatch(ctx context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	if s.cache == nil {
		return map[int64]*AccountLoadInfo{}, nil
	}
	return s.cache.GetAccountsLoadBatch(ctx, accounts)
}

// GetUsersLoadBatch returns load info for multiple users.
func (s *ConcurrencyService) GetUsersLoadBatch(ctx context.Context, users []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	if s.cache == nil {
		return map[int64]*UserLoadInfo{}, nil
	}
	return s.cache.GetUsersLoadBatch(ctx, users)
}

// CleanupExpiredAccountSlots removes expired slots for one account (background task).
func (s *ConcurrencyService) CleanupExpiredAccountSlots(ctx context.Context, accountID int64) error {
	if s.cache == nil {
		return nil
	}
	return s.cache.CleanupExpiredAccountSlots(ctx, accountID)
}

// StartSlotCleanupWorker starts a background cleanup worker for expired account slots.
func (s *ConcurrencyService) StartSlotCleanupWorker(_ AccountRepository, interval time.Duration) {
	if s == nil || s.cache == nil || interval <= 0 {
		return
	}

	runCleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.cache.CleanupExpiredAccountSlotKeys(cleanupCtx)
		cancel()
		if err != nil {
			logger.LegacyPrintf("service.concurrency", "Warning: cleanup expired account slots failed: %v", err)
		}
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		runCleanup()
		for range ticker.C {
			runCleanup()
		}
	}()
}

// GetAccountConcurrencyBatch gets current concurrency counts for multiple accounts.
// Uses a detached context with timeout to prevent HTTP request cancellation from
// causing the entire batch to fail (which would show all concurrency as 0).
func (s *ConcurrencyService) GetAccountConcurrencyBatch(ctx context.Context, accountIDs []int64) (map[int64]int, error) {
	if len(accountIDs) == 0 {
		return map[int64]int{}, nil
	}
	if s.cache == nil {
		result := make(map[int64]int, len(accountIDs))
		for _, accountID := range accountIDs {
			result[accountID] = 0
		}
		return result, nil
	}

	// Use a detached context so that a cancelled HTTP request doesn't cause
	// the Redis pipeline to fail and return all-zero concurrency counts.
	redisCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return s.cache.GetAccountConcurrencyBatch(redisCtx, accountIDs)
}

// AcquireAccountUserSlot acquires a pair-level slot.
// maxConcurrency<=0 is count-only: still write Redis, never reject for a cap.
func (s *ConcurrencyService) AcquireAccountUserSlot(ctx context.Context, accountID, userID int64, maxConcurrency int) (*AcquireResult, error) {
	if accountID <= 0 || userID <= 0 {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {},
		}, nil
	}
	if s == nil || s.cache == nil {
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: func() {},
		}, nil
	}

	ctx = withSlotOwnerPrefix(ctx)
	requestID := pairSlotMemberID(ctx)
	acquired, err := s.cache.AcquireAccountUserSlot(ctx, accountID, userID, maxConcurrency, requestID)
	if err != nil {
		return nil, err
	}
	if acquired {
		hold := pairHoldKey{
			accountID: accountID,
			userID:    userID,
			platform:  pairHoldPlatform(ctx),
			requestID: requestID,
		}
		s.addPairHold(hold)
		var once sync.Once
		stopTouch := startPairSlotTouch(ctx, s.cache, accountID, userID, maxConcurrency, requestID)
		release := func() {
			once.Do(func() {
				stopTouch()
				if !s.releasePairHold(hold) {
					return
				}
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if plat, ok := ctx.Value(ctxkey.ScheduleLookupPlatform).(string); ok && strings.TrimSpace(plat) != "" {
					bgCtx = context.WithValue(bgCtx, ctxkey.ScheduleLookupPlatform, plat)
				}
				if err := s.cache.ReleaseAccountUserSlot(withSlotOwnerPrefix(bgCtx), accountID, userID, requestID); err != nil {
					logger.LegacyPrintf("service.concurrency", "Warning: failed to release account-user slot for %d/%d (req=%s): %v", accountID, userID, requestID, err)
				}
			})
		}
		if ctx.Done() != nil {
			context.AfterFunc(ctx, release)
		}
		return &AcquireResult{
			Acquired:    true,
			ReleaseFunc: release,
		}, nil
	}
	return &AcquireResult{Acquired: false, ReleaseFunc: nil}, nil
}

// GetAccountUserConcurrencyBatch returns live pair counts for one user across accounts.
func (s *ConcurrencyService) GetAccountUserConcurrencyBatch(ctx context.Context, accountIDs []int64, userID int64) (map[int64]int, error) {
	result := make(map[int64]int, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = 0
	}
	if len(accountIDs) == 0 || userID <= 0 || s == nil || s.cache == nil {
		return result, nil
	}
	redisCtx, cancel := detachPairRedisCtx(ctx)
	defer cancel()
	counts, err := s.cache.GetAccountUserConcurrencyBatch(redisCtx, accountIDs, userID)
	if err != nil {
		logger.LegacyPrintf("service.concurrency", "Warning: get account-user concurrency batch failed: %v", err)
		return result, nil
	}
	for _, accountID := range accountIDs {
		result[accountID] = counts[accountID]
	}
	return result, nil
}
