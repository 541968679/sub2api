package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	creditSnapshotInterval        = 15 * time.Minute
	creditSnapshotManualCooldown  = 30 * time.Second
	creditSnapshotCaptureTimeout  = 60 * time.Second
	creditSnapshotLogComponent    = "service.credit_snapshot"
	creditSnapshotRangeLookbackMu = 30 * time.Minute
)

const (
	creditCurveGranularityHour = "hour"
	creditCurveGranularityDay  = "day"
)

// creditBalanceFetcher 抽象"根据账号 ID 获取 UsageInfo（含 AICredits）"的行为，
// 在测试中便于替换。生产实现是 *AccountUsageService.GetUsage。
type creditBalanceFetcher interface {
	GetUsage(ctx context.Context, accountID int64) (*UsageInfo, error)
}

// CreditSnapshotService 定时采样 Antigravity AI Credits 余额并提供聚合查询。
type CreditSnapshotService struct {
	repo        CreditSnapshotRepository
	accountRepo AccountRepository
	balance     creditBalanceFetcher
	usageAgg    AntigravityUsageAggregator

	manualMu     sync.Mutex
	lastManualAt time.Time

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewCreditSnapshotService 创建服务实例。Initialize 会在后台启动定时采样。
func NewCreditSnapshotService(
	repo CreditSnapshotRepository,
	accountRepo AccountRepository,
	usageService *AccountUsageService,
	usageAgg AntigravityUsageAggregator,
) *CreditSnapshotService {
	return &CreditSnapshotService{
		repo:        repo,
		accountRepo: accountRepo,
		balance:     usageService,
		usageAgg:    usageAgg,
		stopCh:      make(chan struct{}),
	}
}

// Initialize 启动定时采样。不做首次同步采样——等 ticker 第一次触发即可，避免阻塞启动流程。
func (s *CreditSnapshotService) Initialize() error {
	if s.repo == nil || s.accountRepo == nil || s.balance == nil {
		return errors.New("credit snapshot service: missing dependency")
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(creditSnapshotInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := s.safeCapture(context.Background(), "scheduled"); err != nil {
					logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] Scheduled capture failed: %v", err)
				}
			case <-s.stopCh:
				return
			}
		}
	}()
	logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] Scheduler started (interval=%v)", creditSnapshotInterval)
	return nil
}

// Stop 停止定时任务，等待后台 goroutine 退出。
func (s *CreditSnapshotService) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.wg.Wait()
}

// safeCapture 执行一次采样，失败只记日志。
func (s *CreditSnapshotService) safeCapture(parent context.Context, reason string) (int, error) {
	ctx, cancel := context.WithTimeout(parent, creditSnapshotCaptureTimeout)
	defer cancel()
	n, err := s.captureOnce(ctx)
	if err != nil {
		logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] capture (%s) failed: %v", reason, err)
	} else {
		logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] capture (%s) wrote %d rows", reason, n)
	}
	return n, err
}

// captureOnce 拉取所有启用 antigravity 账号的 AI Credits 余额并写库。按 email 去重
// （同 Google 账号共享 credits）。单账号失败只记 warn 不影响整体。
func (s *CreditSnapshotService) captureOnce(ctx context.Context) (int, error) {
	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformAntigravity)
	if err != nil {
		return 0, err
	}
	seen := make(map[string]struct{}, len(accounts))
	written := 0
	now := time.Now()
	for i := range accounts {
		acct := accounts[i]
		if acct.Status != StatusActive {
			continue
		}
		email := antigravityAccountEmail(&acct)
		if email == "" {
			continue
		}
		if _, dup := seen[email]; dup {
			continue
		}
		seen[email] = struct{}{}

		info, err := s.balance.GetUsage(ctx, acct.ID)
		if err != nil {
			logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] GetUsage failed for account %d (%s): %v", acct.ID, email, err)
			continue
		}
		if info == nil || len(info.AICredits) == 0 {
			continue
		}
		for _, c := range info.AICredits {
			creditType := strings.TrimSpace(c.CreditType)
			if creditType == "" {
				creditType = "UNKNOWN"
			}
			snap := &CreditSnapshot{
				Email:      email,
				CreditType: creditType,
				Amount:     c.Amount,
				CapturedAt: now,
			}
			if err := s.repo.Insert(ctx, snap); err != nil {
				logger.LegacyPrintf(creditSnapshotLogComponent, "[CreditSnapshot] Insert failed for %s/%s: %v", email, creditType, err)
				continue
			}
			written++
		}
	}
	return written, nil
}

// TriggerManualCapture 手动触发一次采样，带 30 秒进程内冷却锁。冷却期内返回 throttled=true
// 不采样。调用方可据此向用户提示"近期刚采样过"。
func (s *CreditSnapshotService) TriggerManualCapture(ctx context.Context) (captured int, throttled bool, err error) {
	s.manualMu.Lock()
	if !s.lastManualAt.IsZero() && time.Since(s.lastManualAt) < creditSnapshotManualCooldown {
		s.manualMu.Unlock()
		return 0, true, nil
	}
	s.lastManualAt = time.Now()
	s.manualMu.Unlock()

	n, err := s.safeCapture(ctx, "manual")
	return n, false, err
}

// GetAntigravityUsageRatio 聚合时间窗内的 credits 消耗、额度使用和调用次数，派生比率。
// 消耗算法：对每个 email 在 [start-lookback, end] 内的快照按时间升序遍历相邻对，
// 累加正向 delta（上一条 amount - 当前 amount，>0 才加）。负向 delta（充值/重置）跳过。
// lookback 用于把窗口起点之前的最近一条快照也纳入，避免丢掉"窗口开头那一段消耗"。
func (s *CreditSnapshotService) GetAntigravityUsageRatio(ctx context.Context, start, end time.Time) (*AntigravityUsageRatio, error) {
	if !end.After(start) {
		return nil, errors.New("end must be after start")
	}

	snapStart := start.Add(-creditSnapshotRangeLookbackMu)
	grouped, err := s.repo.ListInRange(ctx, snapStart, end)
	if err != nil {
		return nil, err
	}

	creditsByType := make(map[string]float64)
	snapshotCount := 0
	emailsSampled := 0
	for _, snaps := range grouped {
		if len(snaps) == 0 {
			continue
		}
		emailsSampled++
		snapshotCount += len(snaps)
		for i := 1; i < len(snaps); i++ {
			prev := snaps[i-1]
			curr := snaps[i]
			if curr.CreditType != prev.CreditType {
				continue
			}
			delta := prev.Amount - curr.Amount
			if delta > 0 {
				creditsByType[curr.CreditType] += delta
			}
		}
	}

	var totalCredits float64
	for _, v := range creditsByType {
		totalCredits += v
	}

	callCount, totalCost, err := s.usageAgg.AggregateUsage(ctx, start, end)
	if err != nil {
		return nil, err
	}

	result := &AntigravityUsageRatio{
		Start:           start,
		End:             end,
		CreditsConsumed: totalCredits,
		CreditsByType:   creditsByType,
		QuotaUsedUSD:    totalCost,
		CallCount:       callCount,
		SnapshotCount:   snapshotCount,
		EmailsSampled:   emailsSampled,
	}
	if totalCredits > 0 {
		qpc := totalCost / totalCredits
		cpc := float64(callCount) / totalCredits
		result.QuotaPerCredit = &qpc
		result.CallsPerCredit = &cpc
	}
	return result, nil
}

// antigravityAccountEmail 从 Account.Credentials 取出 email。
// 同一 Google 账号授权多个 Antigravity 账号时会共享 credits 余额，按 email 去重。
func (s *CreditSnapshotService) GetAntigravityCreditCurve(ctx context.Context, start, end time.Time, granularity string) (*AntigravityCreditCurve, error) {
	if !end.After(start) {
		return nil, errors.New("end must be after start")
	}
	if granularity != creditCurveGranularityDay {
		granularity = creditCurveGranularityHour
	}

	points := buildCreditCurveBuckets(start, end, granularity)
	if len(points) == 0 {
		return &AntigravityCreditCurve{Start: start, End: end, Granularity: granularity, Points: nil}, nil
	}
	pointByStart := make(map[int64]*AntigravityCreditCurvePoint, len(points))
	for i := range points {
		points[i].CreditsByType = make(map[string]float64)
		pointByStart[creditCurveBucketKey(points[i].Start, granularity)] = &points[i]
	}

	usageWindows, err := s.usageAgg.AggregateUsageWindows(ctx, start, end, granularity)
	if err != nil {
		return nil, err
	}
	for _, usage := range usageWindows {
		point := pointByStart[creditCurveBucketKey(usage.Start, granularity)]
		if point == nil {
			continue
		}
		point.CallCount = usage.CallCount
		point.TotalTokens = usage.TotalTokens
		point.QuotaUsedUSD = usage.QuotaUsed
		point.ActualCostUSD = usage.ActualCost
	}

	grouped, err := s.repo.ListInRange(ctx, start.Add(-creditSnapshotRangeLookbackMu), end)
	if err != nil {
		return nil, err
	}
	for _, snaps := range grouped {
		for i := 1; i < len(snaps); i++ {
			prev := snaps[i-1]
			curr := snaps[i]
			if curr.CreditType != prev.CreditType || curr.CapturedAt.Before(start) || !curr.CapturedAt.Before(end) {
				continue
			}
			point := pointByStart[creditCurveBucketKey(curr.CapturedAt, granularity)]
			if point != nil {
				point.SnapshotCount++
			}
			delta := prev.Amount - curr.Amount
			if delta <= 0 {
				continue
			}
			distributeCreditDelta(points, start, end, granularity, prev, curr, delta)
		}
	}

	for i := range points {
		fillCreditCurveRatios(&points[i])
	}
	scoreCreditCurveAnomalies(points)

	return &AntigravityCreditCurve{
		Start:       start,
		End:         end,
		Granularity: granularity,
		Points:      points,
	}, nil
}

func buildCreditCurveBuckets(start, end time.Time, granularity string) []AntigravityCreditCurvePoint {
	cursor := creditCurveBucketStart(start, granularity)
	points := make([]AntigravityCreditCurvePoint, 0)
	for cursor.Before(end) {
		next := cursor.Add(time.Hour)
		if granularity == creditCurveGranularityDay {
			next = cursor.AddDate(0, 0, 1)
		}
		points = append(points, AntigravityCreditCurvePoint{Start: cursor, End: next})
		cursor = next
	}
	return points
}

func creditCurveBucketStart(t time.Time, granularity string) time.Time {
	if granularity == creditCurveGranularityDay {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
	return t.Truncate(time.Hour)
}

func creditCurveBucketKey(t time.Time, granularity string) int64 {
	return creditCurveBucketStart(t, granularity).Unix()
}

func distributeCreditDelta(points []AntigravityCreditCurvePoint, start, end time.Time, granularity string, prev, curr CreditSnapshot, delta float64) {
	if delta <= 0 || curr.CreditType == "" {
		return
	}
	intervalStart := prev.CapturedAt
	if intervalStart.Before(start) {
		intervalStart = start
	}
	intervalEnd := curr.CapturedAt
	if intervalEnd.After(end) {
		intervalEnd = end
	}
	if !intervalEnd.After(intervalStart) {
		return
	}

	type weightedPoint struct {
		index  int
		weight float64
	}
	weighted := make([]weightedPoint, 0, 2)
	totalWeight := 0.0
	for i := range points {
		if !points[i].End.After(intervalStart) || !points[i].Start.Before(intervalEnd) {
			continue
		}
		weight := creditCurveUsageWeight(&points[i])
		if weight <= 0 {
			continue
		}
		weighted = append(weighted, weightedPoint{index: i, weight: weight})
		totalWeight += weight
	}
	if totalWeight > 0 {
		for _, item := range weighted {
			share := delta * item.weight / totalWeight
			addCreditDeltaToPoint(&points[item.index], curr.CreditType, share)
		}
		return
	}

	fallbackKey := creditCurveBucketKey(curr.CapturedAt, granularity)
	for i := range points {
		if creditCurveBucketKey(points[i].Start, granularity) == fallbackKey {
			addCreditDeltaToPoint(&points[i], curr.CreditType, delta)
			return
		}
	}
}

func creditCurveUsageWeight(point *AntigravityCreditCurvePoint) float64 {
	if point == nil {
		return 0
	}
	if point.QuotaUsedUSD > 0 {
		return point.QuotaUsedUSD
	}
	if point.ActualCostUSD > 0 {
		return point.ActualCostUSD
	}
	if point.TotalTokens > 0 {
		return float64(point.TotalTokens)
	}
	if point.CallCount > 0 {
		return float64(point.CallCount)
	}
	return 0
}

func addCreditDeltaToPoint(point *AntigravityCreditCurvePoint, creditType string, delta float64) {
	if point == nil || delta <= 0 {
		return
	}
	if point.CreditsByType == nil {
		point.CreditsByType = make(map[string]float64)
	}
	point.CreditsConsumed += delta
	point.CreditsByType[creditType] += delta
}

func fillCreditCurveRatios(point *AntigravityCreditCurvePoint) {
	if point == nil {
		return
	}
	if point.CallCount > 0 {
		v := point.CreditsConsumed / float64(point.CallCount)
		point.CreditsPerCall = &v
	}
	if point.CreditsConsumed > 0 {
		qpc := point.QuotaUsedUSD / point.CreditsConsumed
		tpc := float64(point.TotalTokens) / point.CreditsConsumed
		point.QuotaPerCredit = &qpc
		point.TokensPerCredit = &tpc
	}
}

func scoreCreditCurveAnomalies(points []AntigravityCreditCurvePoint) {
	ratios := make([]float64, 0, len(points))
	for _, point := range points {
		if point.CreditsPerCall != nil && point.CallCount >= 5 && point.CreditsConsumed > 0 {
			ratios = append(ratios, *point.CreditsPerCall)
		}
	}
	if len(ratios) < 3 {
		return
	}
	sort.Float64s(ratios)
	median := ratios[len(ratios)/2]
	if len(ratios)%2 == 0 {
		median = (ratios[len(ratios)/2-1] + ratios[len(ratios)/2]) / 2
	}
	if median <= 0 {
		return
	}
	for i := range points {
		point := &points[i]
		if point.CreditsPerCall == nil || point.CallCount < 5 {
			continue
		}
		score := *point.CreditsPerCall / median
		point.AnomalyScore = score
		if score >= 3 && point.CreditsConsumed >= 100 {
			point.AnomalyDescription = fmt.Sprintf("credits/call %.1fx median", score)
		}
	}
}

func antigravityAccountEmail(a *Account) string {
	if a == nil || a.Credentials == nil {
		return ""
	}
	if v, ok := a.Credentials["email"]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(strings.ToLower(s))
		}
	}
	return ""
}
