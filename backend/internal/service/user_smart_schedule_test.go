//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type memorySmartLookup struct {
	bundle        *UserSmartScheduleBundle
	cooldownUntil map[string]int64
	pair          map[string]*PairQualityLive
	probing       map[string]bool
	pinned        map[string]bool
	resumeUntil   map[string]int64
	startCalls    int
	lastUntilUnix int64
	graduated     int
}

func (m *memorySmartLookup) Lookup(_ context.Context, _ int64) *UserSmartScheduleBundle {
	if m == nil {
		return nil
	}
	return m.bundle
}

func (m *memorySmartLookup) CooldownActive(_ context.Context, accountID, userID int64, _ string, now time.Time) bool {
	if m == nil || len(m.cooldownUntil) == 0 {
		return false
	}
	until := m.cooldownUntil[smartPairKey(accountID, userID)]
	return until > now.Unix()
}

func (m *memorySmartLookup) StartCooldown(_ context.Context, accountID, userID int64, _ string, minutes int, now time.Time) {
	if m == nil {
		return
	}
	if m.cooldownUntil == nil {
		m.cooldownUntil = map[string]int64{}
	}
	m.startCalls++
	key := smartPairKey(accountID, userID)
	until := now.Add(time.Duration(minutes) * time.Minute).Unix()
	if existing, ok := m.cooldownUntil[key]; ok && existing > now.Unix() {
		return
	}
	m.cooldownUntil[key] = until
	m.lastUntilUnix = until
}

func (m *memorySmartLookup) StartCooldownWithReason(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time, _ string) {
	m.StartCooldown(ctx, accountID, userID, platform, minutes, now)
}

func (m *memorySmartLookup) GetPairQuality(_ context.Context, accountID, userID int64, _ string) *PairQualityLive {
	if m == nil || len(m.pair) == 0 {
		return nil
	}
	return m.pair[smartPairKey(accountID, userID)]
}

func (m *memorySmartLookup) IsProbing(_ context.Context, accountID, userID int64, _ string) bool {
	if m == nil || len(m.probing) == 0 {
		return false
	}
	return m.probing[smartPairKey(accountID, userID)]
}

func (m *memorySmartLookup) MarkProbing(_ context.Context, accountID, userID int64, _ string) {
	if m == nil {
		return
	}
	if m.probing == nil {
		m.probing = map[string]bool{}
	}
	m.probing[smartPairKey(accountID, userID)] = true
}

func (m *memorySmartLookup) ClearProbing(_ context.Context, accountID, userID int64, _ string) {
	if m == nil || len(m.probing) == 0 {
		return
	}
	delete(m.probing, smartPairKey(accountID, userID))
}

func (m *memorySmartLookup) GraduateProbing(ctx context.Context, accountID, userID int64, platform string) {
	if m == nil {
		return
	}
	if m.IsProbing(ctx, accountID, userID, platform) {
		m.graduated++
	}
	m.ClearProbing(ctx, accountID, userID, platform)
}

func (m *memorySmartLookup) IsPinned(_ context.Context, accountID, userID int64, _ string) bool {
	if m == nil || len(m.pinned) == 0 {
		return false
	}
	return m.pinned[smartPairKey(accountID, userID)]
}

func (m *memorySmartLookup) MarkPinned(_ context.Context, accountID, userID int64, _ string) {
	if m == nil {
		return
	}
	if m.pinned == nil {
		m.pinned = map[string]bool{}
	}
	m.pinned[smartPairKey(accountID, userID)] = true
}

func (m *memorySmartLookup) ClearPinned(_ context.Context, accountID, userID int64, _ string) {
	if m == nil || len(m.pinned) == 0 {
		return
	}
	delete(m.pinned, smartPairKey(accountID, userID))
}

func (m *memorySmartLookup) PairResumeActive(_ context.Context, accountID, userID int64, platform string, now time.Time) bool {
	if m == nil || len(m.resumeUntil) == 0 {
		return false
	}
	return m.resumeUntil[smartPairPlatformKey(accountID, userID, platform)] > now.Unix()
}

func (m *memorySmartLookup) ClearPairResume(_ context.Context, accountID, userID int64, platform string) {
	if m == nil || len(m.resumeUntil) == 0 {
		return
	}
	delete(m.resumeUntil, smartPairPlatformKey(accountID, userID, platform))
}

func smartPairKey(accountID, userID int64) string {
	return strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func smartPairPlatformKey(accountID, userID int64, platform string) string {
	return SmartScheduleRedisPlatform(platform) + ":" + smartPairKey(accountID, userID)
}

func enabledSmartPolicy(accountID int64, capN int, p50 *int) *SmartSchedulePlatformPolicy {
	policy := &SmartSchedulePlatformPolicy{
		Enabled:         true,
		CooldownMinutes: 15,
		AccountIDs:      map[int64]struct{}{accountID: {}},
		Caps:            map[int64]int{},
	}
	if capN >= 1 {
		policy.Caps[accountID] = capN
	}
	policy.QualityMaxP50TTFTMs = p50
	return policy
}

func smartBundle(platform string, policy *SmartSchedulePlatformPolicy) *UserSmartScheduleBundle {
	return &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{platform: policy}}
}

func TestAdmitsScheduleUser_SmartScheduleSynthesis(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	p50 := 1000
	denied := &Account{
		ID:              7,
		Platform:        PlatformAnthropic,
		DenyUserIDs:     []int64{16},
		AllowUserIDs:    []int64{99},
		UserConcurrency: map[int64]int{16: 2},
		UserQualityGates: map[int64]QualityHardCloseSettings{
			16: fillUserQualityGateDefaults(QualityHardCloseSettings{MaxP50TTFTMs: &p50}),
		},
	}

	t.Run("disabled policy keeps legacy deny", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:    false,
			AccountIDs: map[int64]struct{}{7: {}},
		})}
		require.False(t, admitsScheduleUser(ctx, denied, nil, lookup))
	})

	t.Run("missing platform keeps legacy deny", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil))}
		require.False(t, admitsScheduleUser(ctx, denied, nil, lookup))
	})

	t.Run("enabled pool miss rejects even unrestricted account", func(t *testing.T) {
		t.Parallel()
		open := &Account{ID: 8, Platform: PlatformAnthropic}
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
		require.False(t, admitsScheduleUser(ctx, open, nil, lookup))
	})

	t.Run("enabled in-pool ignores old deny gate and cap", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 9, nil))}
		require.True(t, admitsScheduleUser(ctx, denied, &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{7: liveQualityStats(4000, 12, 20, 0, 1)},
		}, lookup))
		require.Equal(t, 9, resolvePairMaxConcurrency(ctx, denied, lookup))
		require.Equal(t, 2, denied.PairMaxConcurrency(16))
	})

	t.Run("quality breach starts cooldown and later quality recovery stays blocked", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{
			bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, &p50)),
			pair:   map[string]*PairQualityLive{smartPairKey(7, 16): breachedPairLive(4000)},
		}
		accountLive := &liveQualityCacheStub{byID: map[int64]*AccountQualityStats{7: liveQualityStats(4000, 12, 20, 0, 1)}}
		require.False(t, admitsScheduleUser(ctx, denied, accountLive, lookup))
		require.Equal(t, 1, lookup.startCalls)
		firstUntil := lookup.lastUntilUnix
		require.Greater(t, firstUntil, time.Now().UTC().Unix())

		lookup.pair[smartPairKey(7, 16)] = breachedPairLive(200)
		require.False(t, admitsScheduleUser(ctx, denied, accountLive, lookup))
		lookup.StartCooldown(ctx, 7, 16, PlatformAnthropic, 30, time.Now().UTC())
		require.Equal(t, firstUntil, lookup.lastUntilUnix)
		require.Equal(t, firstUntil, lookup.cooldownUntil[smartPairKey(7, 16)])
	})

	t.Run("account 15m live breach does not start smart-schedule cooldown", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, &p50))}
		require.True(t, admitsScheduleUser(ctx, denied, &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{7: liveQualityStats(4000, 12, 20, 0, 1)},
		}, lookup))
		require.Equal(t, 0, lookup.startCalls)
	})

	t.Run("under-N pair window does not cooldown without C/K trip", func(t *testing.T) {
		t.Parallel()
		live := &PairQualityLive{N: DefaultSmartScheduleWindowN, TTFTMs: []int{4000}, OK: []bool{true}}
		RecomputePairQuality(live)
		lookup := &memorySmartLookup{
			bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, &p50)),
			pair:   map[string]*PairQualityLive{smartPairKey(7, 16): live},
		}
		require.True(t, admitsScheduleUser(ctx, denied, nil, lookup))
		require.Equal(t, 0, lookup.startCalls)
	})

	t.Run("live miss with quality metrics fails open", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, &p50))}
		require.True(t, admitsScheduleUser(ctx, denied, &liveQualityCacheStub{}, lookup))
		require.Equal(t, 0, lookup.startCalls)
	})

	t.Run("userID zero does not apply smart schedule", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
		require.False(t, admitsScheduleUser(context.Background(), denied, nil, lookup))
	})

	t.Run("disabled pair cap falls back to account user concurrency", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled: false,
			Caps:    map[int64]int{7: 9},
		})}
		require.Equal(t, 2, resolvePairMaxConcurrency(ctx, denied, lookup))
	})

	t.Run("paused in-pool member is skipped without starting cooldown", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{7: {}},
			Paused:          map[int64]struct{}{7: {}},
		})}
		require.False(t, admitsScheduleUser(ctx, denied, &liveQualityCacheStub{
			byID: map[int64]*AccountQualityStats{7: liveQualityStats(200, 12, 20, 0, 1)},
		}, lookup))
		require.Equal(t, 0, lookup.startCalls)
	})

	t.Run("AG policy disabled keeps openai closed pool for bridge", func(t *testing.T) {
		t.Parallel()
		agCtx := agGroupScheduleCtx(16)
		deniedOAI := &Account{ID: 7, Platform: PlatformOpenAI, DenyUserIDs: []int64{16}}
		outside := &Account{ID: 8, Platform: PlatformOpenAI}
		lookup := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: {Enabled: false, AccountIDs: map[int64]struct{}{7: {}}},
		}}}
		require.True(t, admitsScheduleUser(agCtx, deniedOAI, nil, lookup), "in-pool OAI must still admit via openai")
		require.False(t, admitsScheduleUser(agCtx, outside, nil, lookup), "out-of-openai-pool must reject")
	})

	t.Run("AG empty enabled pool keeps openai closed pool for bridge", func(t *testing.T) {
		t.Parallel()
		agCtx := agGroupScheduleCtx(16)
		deniedOAI := &Account{ID: 7, Platform: PlatformOpenAI, DenyUserIDs: []int64{16}}
		outside := &Account{ID: 8, Platform: PlatformOpenAI}
		lookup := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: {Enabled: true, AccountIDs: map[int64]struct{}{}},
		}}}
		require.True(t, admitsScheduleUser(agCtx, deniedOAI, nil, lookup))
		require.False(t, admitsScheduleUser(agCtx, outside, nil, lookup))
	})

	t.Run("AG missing policy keeps openai closed pool for bridge", func(t *testing.T) {
		t.Parallel()
		agCtx := agGroupScheduleCtx(16)
		deniedOAI := &Account{ID: 7, Platform: PlatformOpenAI, DenyUserIDs: []int64{16}}
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil))}
		require.True(t, admitsScheduleUser(agCtx, deniedOAI, nil, lookup))
	})

	t.Run("AG on with members admits only AG pool", func(t *testing.T) {
		t.Parallel()
		agCtx := agGroupScheduleCtx(16)
		nativeCtx := nativeOpenAIGroupScheduleCtx(16)
		deniedOAI := &Account{ID: 7, Platform: PlatformOpenAI, DenyUserIDs: []int64{16}}
		openaiOnly := &Account{ID: 8, Platform: PlatformOpenAI}
		lookup := &memorySmartLookup{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(8, 0, nil),
			PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
		}}}
		require.True(t, admitsScheduleUser(agCtx, deniedOAI, nil, lookup), "AG-pool OAI admits even with account deny")
		require.False(t, admitsScheduleUser(agCtx, openaiOnly, nil, lookup), "openai-only member must not fail-open")
		require.True(t, admitsScheduleUser(nativeCtx, openaiOnly, nil, lookup), "native GPT stays openai-only")
		require.False(t, admitsScheduleUser(nativeCtx, deniedOAI, nil, lookup), "native GPT must not use AG pool")
	})

	t.Run("enabled empty pool falls back to legacy", func(t *testing.T) {
		t.Parallel()
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:    true,
			AccountIDs: map[int64]struct{}{},
		})}
		require.False(t, admitsScheduleUser(ctx, denied, nil, lookup), "legacy deny must still apply")
		open := &Account{ID: 8, Platform: PlatformAnthropic}
		require.True(t, admitsScheduleUser(ctx, open, nil, lookup), "empty enabled pool must not fail-close unrestricted accounts")
		require.Equal(t, 2, resolvePairMaxConcurrency(ctx, denied, lookup))
	})
}

type stubSmartRepo struct {
	mu     sync.Mutex
	bundle *UserSmartScheduleBundle
}

func (s *stubSmartRepo) ListByUser(_ context.Context, _ int64) (*UserSmartScheduleBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneSmartBundle(s.bundle), nil
}

func (s *stubSmartRepo) ListByUsers(_ context.Context, userIDs []int64) (map[int64]*UserSmartScheduleBundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int64]*UserSmartScheduleBundle, len(userIDs))
	for _, userID := range userIDs {
		out[userID] = cloneSmartBundle(s.bundle)
	}
	return out, nil
}

func (s *stubSmartRepo) ReplacePlatform(_ context.Context, _ int64, platform string, policy SmartSchedulePlatformWrite) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bundle == nil {
		s.bundle = &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{}}
	}
	if s.bundle.Policies == nil {
		s.bundle.Policies = map[string]*SmartSchedulePlatformPolicy{}
	}
	next := &SmartSchedulePlatformPolicy{
		Enabled:                  policy.Enabled,
		QualityMaxP50TTFTMs:      policy.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:    policy.QualityMinSuccessRate,
		QualityWindowSamples:     policy.QualityWindowSamples,
		QualityMinSuccessSamples: policy.QualityMinSuccessSamples,
		QualityMinTTFTSamples:    policy.QualityMinTTFTSamples,
		QualityCondition:         policy.QualityCondition,
		CooldownMinutes:          policy.CooldownMinutes,
		SoftCooldown:             policy.SoftCooldown,
		ProbeLatencyV2:           policy.ProbeLatencyV2,
		ProbeConcurrencyMode:     policy.ProbeConcurrencyMode,
		ProbeConcurrency:         policy.ProbeConcurrency,
		AccountIDs:               map[int64]struct{}{},
		Caps:                     map[int64]int{},
		SortOrders:               map[int64]int{},
	}
	for _, member := range policy.Accounts {
		next.AccountIDs[member.AccountID] = struct{}{}
		if member.MaxConcurrency != nil && *member.MaxConcurrency >= 1 {
			next.Caps[member.AccountID] = *member.MaxConcurrency
		}
		if member.SortOrder != nil {
			next.SortOrders[member.AccountID] = *member.SortOrder
		}
	}
	prev := s.bundle.Policies[platform]
	if prev != nil {
		next.Paused = map[int64]struct{}{}
		for accountID := range next.AccountIDs {
			if prev.IsPaused(accountID) {
				next.Paused[accountID] = struct{}{}
			}
		}
	}
	s.bundle.Policies[platform] = next
	return nil
}

func (s *stubSmartRepo) SetMemberPaused(_ context.Context, _ int64, accountID int64, platform string, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bundle == nil || s.bundle.Policies == nil {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	policy := s.bundle.Policies[normalizeSmartSchedulePlatform(platform)]
	if policy == nil || !policy.HasAccount(accountID) {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	if policy.Paused == nil {
		policy.Paused = map[int64]struct{}{}
	}
	if paused {
		policy.Paused[accountID] = struct{}{}
	} else {
		delete(policy.Paused, accountID)
	}
	return nil
}

func (s *stubSmartRepo) UpdateSortOrders(_ context.Context, _ int64, platform string, orders []SmartScheduleSortAssignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bundle == nil || s.bundle.Policies == nil {
		return nil
	}
	policy := s.bundle.Policies[platform]
	if policy == nil {
		return nil
	}
	if policy.SortOrders == nil {
		policy.SortOrders = map[int64]int{}
	}
	for _, order := range orders {
		if _, ok := policy.AccountIDs[order.AccountID]; !ok {
			continue
		}
		policy.SortOrders[order.AccountID] = order.SortOrder
	}
	return nil
}

func cloneSmartBundle(in *UserSmartScheduleBundle) *UserSmartScheduleBundle {
	out := &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{}}
	if in == nil {
		return out
	}
	for platform, policy := range in.Policies {
		if policy == nil {
			continue
		}
		copied := *policy
		copied.AccountIDs = map[int64]struct{}{}
		for id := range policy.AccountIDs {
			copied.AccountIDs[id] = struct{}{}
		}
		copied.Caps = map[int64]int{}
		for id, n := range policy.Caps {
			copied.Caps[id] = n
		}
		copied.SortOrders = map[int64]int{}
		for id, n := range policy.SortOrders {
			copied.SortOrders[id] = n
		}
		copied.Paused = map[int64]struct{}{}
		for id := range policy.Paused {
			copied.Paused[id] = struct{}{}
		}
		out.Policies[platform] = &copied
	}
	return out
}

type stubSmartAccountRepo struct {
	accounts []*Account
}

func (s *stubSmartAccountRepo) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	byID := map[int64]*Account{}
	for _, acc := range s.accounts {
		if acc != nil {
			byID[acc.ID] = acc
		}
	}
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		if acc := byID[id]; acc != nil {
			out = append(out, acc)
		}
	}
	return out, nil
}

func TestUserSmartScheduleService_EmptyPoolAndCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic},
		{ID: 12, Platform: PlatformOpenAI},
	}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)

	t.Run("cannot enable empty pool", func(t *testing.T) {
		t.Parallel()
		_, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
		})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_EMPTY_POOL", infraerrors.Reason(err))
	})

	t.Run("deleting last member auto-disables", func(t *testing.T) {
		t.Parallel()
		local := NewUserSmartScheduleService(&stubSmartRepo{}, nil, accounts, nil, nil)
		_, err := local.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts:        []SmartScheduleAccountMember{{AccountID: 11, Platform: PlatformAnthropic}},
		})
		require.NoError(t, err)
		view, err := local.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         false,
			CooldownMinutes: 15,
			Accounts:        nil,
		})
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].Enabled)
		require.Empty(t, view.Platforms[PlatformAnthropic].Accounts)
	})

	t.Run("copy copies thresholds not members", func(t *testing.T) {
		t.Parallel()
		localRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 3, intPtr(800)))}
		localRepo.bundle.Policies[PlatformAnthropic].QualityMinSuccessRate = float64Ptr(0.9)
		localRepo.bundle.Policies[PlatformOpenAI] = &SmartSchedulePlatformPolicy{
			Enabled:         false,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{12: {}},
			Caps:            map[int64]int{12: 4},
		}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		view, err := local.CopyPlatform(ctx, 16, PlatformOpenAI, PlatformAnthropic)
		require.NoError(t, err)
		dest := view.Platforms[PlatformOpenAI]
		require.True(t, dest.Enabled)
		require.Equal(t, 800, *dest.QualityMaxP50TTFTMs)
		require.InDelta(t, 0.9, *dest.QualityMinSuccessRate, 0.0001)
		require.Len(t, dest.Accounts, 1)
		require.Equal(t, int64(12), dest.Accounts[0].AccountID)
		require.Equal(t, 4, *dest.Accounts[0].MaxConcurrency)
		require.Equal(t, ProbeConcurrencyModeFollowN, dest.ProbeConcurrencyMode)
		require.Nil(t, dest.ProbeConcurrency)
		require.False(t, dest.ProbeLatencyV2)
	})

	t.Run("copy copies probe settings as their own fields", func(t *testing.T) {
		t.Parallel()
		from := enabledSmartPolicy(11, 3, intPtr(800))
		from.QualityWindowSamples = intPtr(14)
		from.ProbeConcurrencyMode = ProbeConcurrencyModeCustom
		from.ProbeConcurrency = intPtr(2)
		from.ProbeLatencyV2 = true
		localRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, from)}
		localRepo.bundle.Policies[PlatformOpenAI] = &SmartSchedulePlatformPolicy{
			Enabled:         false,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{12: {}},
			Caps:            map[int64]int{12: 4},
		}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		view, err := local.CopyPlatform(ctx, 16, PlatformOpenAI, PlatformAnthropic)
		require.NoError(t, err)
		dest := view.Platforms[PlatformOpenAI]
		require.Equal(t, ProbeConcurrencyModeCustom, dest.ProbeConcurrencyMode)
		require.NotNil(t, dest.ProbeConcurrency)
		require.Equal(t, 2, *dest.ProbeConcurrency)
		require.Equal(t, 14, *dest.QualityWindowSamples)
		require.NotEqual(t, *dest.QualityWindowSamples, *dest.ProbeConcurrency)
		require.True(t, dest.ProbeLatencyV2)
	})

	t.Run("copy onto empty dest forces enabled off", func(t *testing.T) {
		t.Parallel()
		localRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 0, nil))}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		view, err := local.CopyPlatform(ctx, 16, PlatformOpenAI, PlatformAnthropic)
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformOpenAI].Enabled)
		require.Empty(t, view.Platforms[PlatformOpenAI].Accounts)
	})

	t.Run("get reports enabled empty pool as disabled", func(t *testing.T) {
		t.Parallel()
		localRepo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, &SmartSchedulePlatformPolicy{
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{},
		})}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		view, err := local.Get(ctx, 16)
		require.NoError(t, err)
		require.False(t, view.Platforms[PlatformAnthropic].Enabled)
		require.Empty(t, view.Platforms[PlatformAnthropic].Accounts)
	})

	t.Run("list summaries only include enabled non-empty platforms", func(t *testing.T) {
		t.Parallel()
		localRepo := &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI: {
				Enabled:         true,
				CooldownMinutes: 15,
				AccountIDs:      map[int64]struct{}{1: {}, 2: {}},
			},
			PlatformAnthropic: {Enabled: true, CooldownMinutes: 15, AccountIDs: map[int64]struct{}{}},
			PlatformGemini:    {Enabled: false, CooldownMinutes: 15, AccountIDs: map[int64]struct{}{9: {}}},
		}}}
		local := NewUserSmartScheduleService(localRepo, nil, accounts, nil, nil)
		got, err := local.ListSummaries(ctx, []int64{16})
		require.NoError(t, err)
		summary := got["16"]
		require.Equal(t, []string{PlatformOpenAI}, summary.EnabledPlatforms)
		require.Equal(t, 2, summary.PoolCounts[PlatformOpenAI])
		require.NotContains(t, summary.PoolCounts, PlatformAnthropic)
		require.NotContains(t, summary.PoolCounts, PlatformGemini)
	})
}

type stubPairConcurrency struct {
	counts    map[int64]int
	requested []int64
}

func (s *stubPairConcurrency) GetAccountUserConcurrencyBatch(_ context.Context, accountIDs []int64, _ int64) (map[int64]int, error) {
	s.requested = append([]int64(nil), accountIDs...)
	out := make(map[int64]int, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = s.counts[id]
	}
	return out, nil
}

func TestUserSmartScheduleService_HydratesPairCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 3, nil))}
	accounts := &stubSmartAccountRepo{accounts: []*Account{{ID: 11, Platform: PlatformAnthropic}}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, &stubPairConcurrency{counts: map[int64]int{11: 2}})
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.Len(t, view.Platforms[PlatformAnthropic].Accounts, 1)
	require.Equal(t, 2, view.Platforms[PlatformAnthropic].Accounts[0].CurrentConcurrency)
	require.Equal(t, 3, *view.Platforms[PlatformAnthropic].Accounts[0].MaxConcurrency)
}

func TestUserSmartScheduleService_HydratesCooldownAndUncappedCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	until := time.Now().UTC().Add(12 * time.Minute)
	repo := &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI: {
			Enabled:         true,
			CooldownMinutes: 15,
			UpdatedAt:       time.Now().UTC(),
			AccountIDs:      map[int64]struct{}{21: {}, 22: {}},
			Caps:            map[int64]int{21: 4},
		},
	}}}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 21, Platform: PlatformOpenAI},
		{ID: 22, Platform: PlatformOpenAI},
	}}
	pair := &stubPairConcurrency{counts: map[int64]int{21: 3, 22: 9}}
	svc := NewUserSmartScheduleService(repo, stubSmartCache{until: map[int64]time.Time{21: until}}, accounts, nil, pair)
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, view.DefaultPlatform)
	byID := map[int64]SmartScheduleAccountMember{}
	for _, member := range view.Platforms[PlatformOpenAI].Accounts {
		byID[member.AccountID] = member
	}
	require.Equal(t, 3, byID[21].CurrentConcurrency)
	require.NotNil(t, byID[21].CooldownUntil)
	require.Equal(t, until.Unix(), byID[21].CooldownUntil.Unix())
	require.Equal(t, 9, byID[22].CurrentConcurrency, "uncapped pair still hydrates this user×account occupancy")
	require.Nil(t, byID[22].MaxConcurrency)
	require.ElementsMatch(t, []int64{21, 22}, pair.requested)
}

func TestUserSmartScheduleService_HydratesUncappedPairCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformAnthropic: {
			Enabled:    true,
			UpdatedAt:  time.Now().UTC(),
			AccountIDs: map[int64]struct{}{11: {}},
		},
	}}}
	accounts := &stubSmartAccountRepo{accounts: []*Account{{ID: 11, Platform: PlatformAnthropic, Concurrency: 99}}}
	pair := &stubPairConcurrency{counts: map[int64]int{11: 3}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, pair)
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.Len(t, view.Platforms[PlatformAnthropic].Accounts, 1)
	member := view.Platforms[PlatformAnthropic].Accounts[0]
	require.Nil(t, member.MaxConcurrency)
	require.Equal(t, 3, member.CurrentConcurrency)
	require.NotEqual(t, 99, member.CurrentConcurrency, "pair occupancy must not use account.concurrency")
	require.Equal(t, []int64{11}, pair.requested)
}

func TestPickDefaultSmartSchedulePlatform(t *testing.T) {
	t.Parallel()
	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)

	t.Run("single enabled platform wins", func(t *testing.T) {
		t.Parallel()
		view := bundleToView(16, &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI: {
				Enabled:    true,
				AccountIDs: map[int64]struct{}{1: {}},
				UpdatedAt:  older,
			},
			PlatformAnthropic: {
				Enabled:    false,
				AccountIDs: map[int64]struct{}{2: {}, 3: {}},
				UpdatedAt:  newer,
			},
		}})
		require.Equal(t, PlatformOpenAI, pickDefaultSmartSchedulePlatform(view))
	})

	t.Run("several enabled pick most recently updated", func(t *testing.T) {
		t.Parallel()
		view := bundleToView(16, &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformAnthropic: {
				Enabled:    true,
				AccountIDs: map[int64]struct{}{1: {}, 2: {}, 3: {}},
				UpdatedAt:  older,
			},
			PlatformOpenAI: {
				Enabled:    true,
				AccountIDs: map[int64]struct{}{4: {}},
				UpdatedAt:  newer,
			},
		}})
		require.Equal(t, PlatformOpenAI, pickDefaultSmartSchedulePlatform(view))
	})

	t.Run("none enabled pick most recently updated pool", func(t *testing.T) {
		t.Parallel()
		view := bundleToView(16, &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformAnthropic: {
				Enabled:    false,
				AccountIDs: map[int64]struct{}{1: {}},
				UpdatedAt:  older,
			},
			PlatformGemini: {
				Enabled:    false,
				AccountIDs: map[int64]struct{}{2: {}},
				UpdatedAt:  newer,
			},
		}})
		require.Equal(t, PlatformGemini, pickDefaultSmartSchedulePlatform(view))
	})

	t.Run("empty view falls back to anthropic", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, PlatformAnthropic, pickDefaultSmartSchedulePlatform(emptySmartScheduleView(16)))
	})
}

type stubSmartCache struct {
	until map[int64]time.Time
}

func (s stubSmartCache) Lookup(_ context.Context, _ int64) *UserSmartScheduleBundle { return nil }
func (s stubSmartCache) CooldownActive(_ context.Context, _ int64, _ int64, _ string, _ time.Time) bool {
	return false
}
func (s stubSmartCache) StartCooldown(_ context.Context, _ int64, _ int64, _ string, _ int, _ time.Time) {
}
func (s stubSmartCache) StartCooldownWithReason(_ context.Context, _ int64, _ int64, _ string, _ int, _ time.Time, _ string) {
}
func (s stubSmartCache) Invalidate(_ context.Context, _ int64) error { return nil }
func (s stubSmartCache) ClearCooldown(_ context.Context, _ int64, _ int64, _ string) error {
	return nil
}
func (s stubSmartCache) ClearCooldownAllPlatforms(_ context.Context, _ int64, _ int64) error {
	return nil
}
func (s stubSmartCache) SetCooldown(_ context.Context, _ int64, _ int64, _ string, minutes int, now time.Time) (time.Time, error) {
	return now.Add(time.Duration(ClampSmartScheduleCooldownMinutes(minutes)) * time.Minute), nil
}
func (s stubSmartCache) SetCooldownWithReason(_ context.Context, _ int64, _ int64, _ string, minutes int, now time.Time, _ string) (time.Time, error) {
	return now.Add(time.Duration(ClampSmartScheduleCooldownMinutes(minutes)) * time.Minute), nil
}
func (s stubSmartCache) GetCooldownReason(context.Context, int64, int64, string) string { return "" }

func (s stubSmartCache) ApplyMemberPaused(context.Context, int64, int64, string, bool) error {
	return nil
}

type admissionCacheRecorder struct {
	stubSmartCache
	bundle       *UserSmartScheduleBundle
	cleared      int
	setMins      int
	setErr       error
	probing      map[string]bool
	pinned       map[string]bool
	zeros        []string
	markedProbe  int
	clearedProbe int
	markedPin    int
	clearedPin   int
	graduated    int
	resumeUntil  map[string]int64
}

func (s *admissionCacheRecorder) Lookup(_ context.Context, _ int64) *UserSmartScheduleBundle {
	return s.bundle
}

func (s *admissionCacheRecorder) ClearCooldown(_ context.Context, _ int64, _ int64, _ string) error {
	s.cleared++
	return nil
}

func (s *admissionCacheRecorder) SetCooldown(_ context.Context, _ int64, _ int64, _ string, minutes int, now time.Time) (time.Time, error) {
	return s.SetCooldownWithReason(context.Background(), 0, 0, "", minutes, now, "")
}

func (s *admissionCacheRecorder) SetCooldownWithReason(_ context.Context, _ int64, _ int64, _ string, minutes int, now time.Time, _ string) (time.Time, error) {
	s.setMins = minutes
	if s.setErr != nil {
		return time.Time{}, s.setErr
	}
	return now.Add(time.Duration(ClampSmartScheduleCooldownMinutes(minutes)) * time.Minute), nil
}

func (s *admissionCacheRecorder) ZeroPairQuality(_ context.Context, _ int64, _ int64, _ string, eventType string) {
	s.zeros = append(s.zeros, eventType)
}

func (s *admissionCacheRecorder) IsProbing(_ context.Context, accountID, userID int64, _ string) bool {
	return s.probing[smartPairKey(accountID, userID)]
}

func (s *admissionCacheRecorder) EnterProbe(ctx context.Context, accountID, userID int64, platform string) ProbeAdmissionOutcome {
	s.ZeroPairQuality(ctx, accountID, userID, platform, PairQualityEventExpiryZero)
	s.MarkProbing(ctx, accountID, userID, platform)
	return ProbeAdmissionProbing
}

func (s *admissionCacheRecorder) MarkProbing(_ context.Context, accountID, userID int64, _ string) {
	s.markedProbe++
	if s.probing == nil {
		s.probing = map[string]bool{}
	}
	s.probing[smartPairKey(accountID, userID)] = true
}

func (s *admissionCacheRecorder) ClearProbing(_ context.Context, accountID, userID int64, _ string) {
	s.clearedProbe++
	delete(s.probing, smartPairKey(accountID, userID))
}

func (s *admissionCacheRecorder) GraduateProbing(ctx context.Context, accountID, userID int64, platform string) {
	if s.IsProbing(ctx, accountID, userID, platform) {
		s.graduated++
	}
	s.ClearProbing(ctx, accountID, userID, platform)
}

func (s *admissionCacheRecorder) IsProbingBatch(_ context.Context, accountIDs []int64, userID int64, _ string) map[int64]bool {
	out := map[int64]bool{}
	for _, accountID := range accountIDs {
		if s.IsProbing(context.Background(), accountID, userID, "") {
			out[accountID] = true
		}
	}
	return out
}

func (s *admissionCacheRecorder) IsPinned(_ context.Context, accountID, userID int64, _ string) bool {
	return s.pinned[smartPairKey(accountID, userID)]
}

func (s *admissionCacheRecorder) MarkPinned(_ context.Context, accountID, userID int64, _ string) {
	s.markedPin++
	if s.pinned == nil {
		s.pinned = map[string]bool{}
	}
	s.pinned[smartPairKey(accountID, userID)] = true
}

func (s *admissionCacheRecorder) ClearPinned(_ context.Context, accountID, userID int64, _ string) {
	s.clearedPin++
	delete(s.pinned, smartPairKey(accountID, userID))
}

func (s *admissionCacheRecorder) IsPinnedBatch(_ context.Context, accountIDs []int64, userID int64, _ string) map[int64]bool {
	out := map[int64]bool{}
	for _, accountID := range accountIDs {
		if s.IsPinned(context.Background(), accountID, userID, "") {
			out[accountID] = true
		}
	}
	return out
}

func (s *admissionCacheRecorder) PairResumeActive(_ context.Context, accountID, userID int64, platform string, now time.Time) bool {
	return s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)] > now.Unix()
}

func (s *admissionCacheRecorder) ClearPairResume(_ context.Context, accountID, userID int64, platform string) {
	delete(s.resumeUntil, smartPairPlatformKey(accountID, userID, platform))
}

func (s *admissionCacheRecorder) MarkPairResume(_ context.Context, accountID, userID int64, platform string) error {
	if s.resumeUntil == nil {
		s.resumeUntil = map[string]int64{}
	}
	s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)] = time.Now().UTC().Add(2 * AccountQualityWindow).Unix()
	return nil
}

func (s *admissionCacheRecorder) GetPairResumeUntilBatch(_ context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]PairResumeUntil {
	out := map[int64]PairResumeUntil{}
	for _, accountID := range accountIDs {
		until := s.resumeUntil[smartPairPlatformKey(accountID, userID, platform)]
		if until > now.Unix() {
			out[accountID] = PairResumeUntil{WatchUntil: time.Unix(until, 0).UTC()}
		}
	}
	return out
}

func TestParsePairAdmissionState(t *testing.T) {
	t.Parallel()
	got, err := ParsePairAdmissionState("")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, got)
	got, err = ParsePairAdmissionState(" selectable ")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, got)
	got, err = ParsePairAdmissionState("paused")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPaused, got)
	got, err = ParsePairAdmissionState("probing")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionProbing, got)
	got, err = ParsePairAdmissionState("cooling")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionCooling, got)
	got, err = ParsePairAdmissionState("resumed")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, got)
	got, err = ParsePairAdmissionState("pinned")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPinned, got)
	_, err = ParsePairAdmissionState("nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_ADMISSION_INVALID")
	_, err = ParsePairAdmissionState("unpause")
	require.Error(t, err)
	_, err = ParsePairAdmissionState("pin")
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_ADMISSION_INVALID")
	_, err = ParsePairAdmissionState("long_exempt")
	require.Error(t, err)
}

func TestUserSmartScheduleService_SetPairAdmission(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	quality := &liveQualityCacheStub{}
	cache := &admissionCacheRecorder{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformAnthropic: {CooldownMinutes: 30},
		}},
	}
	svc := NewUserSmartScheduleService(nil, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, quality, nil)

	resumed, err := svc.SetPairAdmission(ctx, 7, 16, "")
	require.NoError(t, err)
	require.Equal(t, PairAdmissionResumed, resumed.State)
	require.Equal(t, 1, cache.cleared)
	require.True(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
	require.Nil(t, quality.byID[7], "pair 豁免期 must not write Track A resume")

	selectable, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, selectable.State)
	require.False(t, cache.PairResumeActive(ctx, 7, 16, PlatformAnthropic, time.Now().UTC()))
	require.Nil(t, quality.byID[7], "selectable must not write Track A grace")

	cooling, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionCooling)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionCooling, cooling.State)
	require.NotNil(t, cooling.CooldownUntil)
	require.Equal(t, 30, cache.setMins)
	require.Nil(t, quality.byID[7], "cooling must not write Track A resume")

	_, err = svc.SetPairAdmission(ctx, 7, 16, "bogus")
	require.Error(t, err)
}

func TestUserSmartScheduleService_SetPairAdmissionPaused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	quality := &liveQualityCacheStub{}
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, quality, nil)

	paused, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPaused)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionPaused, paused.State)
	require.Nil(t, paused.CooldownUntil)
	require.True(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))
	require.Equal(t, 1, cache.cleared)

	selectable, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, selectable.State)
	require.False(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))
}

func TestUserSmartScheduleService_SetPairAdmissionCoolingFailsKeepsPaused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache := &admissionCacheRecorder{
		bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil)),
		setErr: errors.New("cooldown write failed"),
	}
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformAnthropic},
	}}, &liveQualityCacheStub{}, nil)
	_, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionPaused)
	require.NoError(t, err)
	require.True(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7))

	_, err = svc.SetPairAdmission(ctx, 7, 16, PairAdmissionCooling)
	require.Error(t, err)
	require.True(t, repo.bundle.Policies[PlatformAnthropic].IsPaused(7), "failed cooling write must not unpause")
}

func (s stubSmartCache) GetCooldownUntilBatch(_ context.Context, _ []int64, _ int64, _ string, _ time.Time) map[int64]time.Time {
	if s.until == nil {
		return map[int64]time.Time{}
	}
	return s.until
}

func (s stubSmartCache) GetPairQuality(context.Context, int64, int64, string) *PairQualityLive {
	return nil
}
func (s stubSmartCache) IngestPairQuality(context.Context, int64, int64, string, int, int, bool, *int, *int) *PairQualityLive {
	return nil
}
func (s stubSmartCache) ZeroPairQuality(context.Context, int64, int64, string, string) {}
func (s stubSmartCache) IngestSoftCooldown(context.Context, int64, int64, string, int, int, bool, *int, *int, int) *PairQualityLive {
	return nil
}
func (s stubSmartCache) GetSoftCooldown(context.Context, int64, int64, string) *PairQualityLive {
	return nil
}
func (s stubSmartCache) ZeroSoftCooldown(context.Context, int64, int64, string) {}
func (s stubSmartCache) GetSoftCooldownBatch(context.Context, []int64, int64, string) map[int64]*PairQualityLive {
	return map[int64]*PairQualityLive{}
}
func (s stubSmartCache) SoftEndCooldown(context.Context, int64, int64, string, string) {}
func (s stubSmartCache) EnterProbe(context.Context, int64, int64, string) ProbeAdmissionOutcome {
	return ProbeAdmissionProbing
}
func (s stubSmartCache) IsCooldownHard(context.Context, int64, int64, string) bool { return false }
func (s stubSmartCache) GetPairQualityBatch(context.Context, []int64, int64, string) map[int64]*PairQualityLive {
	return map[int64]*PairQualityLive{}
}
func (s stubSmartCache) ListPairQualitySnapshots(context.Context, int64, int64, string, int) []PairQualitySnapshot {
	return nil
}
func (s stubSmartCache) ListPairQualityEvents(context.Context, int64, int64, string, int) []PairQualityEvent {
	return nil
}
func (s stubSmartCache) AppendPairQualityEvent(context.Context, int64, int64, string, PairQualityEvent) {
}
func (s stubSmartCache) IsProbing(context.Context, int64, int64, string) bool  { return false }
func (s stubSmartCache) MarkProbing(context.Context, int64, int64, string)     {}
func (s stubSmartCache) ClearProbing(context.Context, int64, int64, string)    {}
func (s stubSmartCache) GraduateProbing(context.Context, int64, int64, string) {}
func (s stubSmartCache) IsProbingBatch(context.Context, []int64, int64, string) map[int64]bool {
	return map[int64]bool{}
}
func (s stubSmartCache) IsPinned(context.Context, int64, int64, string) bool { return false }
func (s stubSmartCache) MarkPinned(context.Context, int64, int64, string)    {}
func (s stubSmartCache) ClearPinned(context.Context, int64, int64, string)   {}
func (s stubSmartCache) IsPinnedBatch(context.Context, []int64, int64, string) map[int64]bool {
	return map[int64]bool{}
}
func (s stubSmartCache) PairResumeActive(context.Context, int64, int64, string, time.Time) bool {
	return false
}
func (s stubSmartCache) ClearPairResume(context.Context, int64, int64, string) {}
func (s stubSmartCache) MarkPairResume(context.Context, int64, int64, string) error {
	return nil
}
func (s stubSmartCache) GetPairResumeUntilBatch(context.Context, []int64, int64, string, time.Time) map[int64]PairResumeUntil {
	return map[int64]PairResumeUntil{}
}

func breachedPairLive(p50 int) *PairQualityLive {
	n := DefaultSmartScheduleSchedN
	ttft := make([]int, n)
	ok := make([]bool, n)
	for i := 0; i < n; i++ {
		ttft[i] = p50
		ok[i] = true
	}
	live := &PairQualityLive{N: n, NTTFT: n, NOK: n, TTFTMs: ttft, OK: ok}
	RecomputePairQuality(live)
	return live
}

func TestSanitizePoolMembers_OpenAIAllowedOnAntigravityStillRejectedElsewhere(t *testing.T) {
	t.Parallel()
	svc := NewUserSmartScheduleService(nil, nil, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformOpenAI},
		{ID: 8, Platform: PlatformAntigravity},
	}}, nil, nil)

	got, err := svc.sanitizePoolMembers(context.Background(), 16, PlatformAntigravity, []SmartScheduleAccountMember{
		{AccountID: 7},
		{AccountID: 8},
	})
	require.NoError(t, err)
	require.Equal(t, []int64{7, 8}, []int64{got[0].AccountID, got[1].AccountID})

	_, err = svc.sanitizePoolMembers(context.Background(), 16, PlatformAnthropic, []SmartScheduleAccountMember{
		{AccountID: 7},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_PLATFORM_MISMATCH")
}

func TestPutPlatform_DualMembershipDoesNotRemoveOtherPool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &stubSmartRepo{bundle: smartBundle(PlatformOpenAI, enabledSmartPolicy(7, 0, nil))}
	svc := NewUserSmartScheduleService(repo, nil, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformOpenAI},
	}}, nil, nil)
	_, err := svc.PutPlatform(ctx, 16, PlatformAntigravity, SmartSchedulePlatformWrite{
		Enabled:         true,
		CooldownMinutes: 15,
		Accounts:        []SmartScheduleAccountMember{{AccountID: 7}},
	})
	require.NoError(t, err)
	require.True(t, repo.bundle.Policies[PlatformOpenAI].HasAccount(7))
	require.True(t, repo.bundle.Policies[PlatformAntigravity].HasAccount(7))
}

func TestObservePairCompletion_DualMembershipWithoutPlatformSkips(t *testing.T) {
	t.Parallel()
	cache := &observeCacheStub{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
		}},
	}
	svc := NewUserSmartScheduleService(nil, cache, nil, nil, nil)
	svc.ObservePairCompletion(context.Background(), PairQualityObservation{AccountID: 7, UserID: 16, Success: true})
	require.Empty(t, cache.ingested)
}

func TestUserSmartScheduleService_SortOrderPersistsOnMembership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic, Priority: 80},
		{ID: 12, Platform: PlatformAnthropic, Priority: 3},
	}}

	t.Run("put and get keep sort_order and id fallback", func(t *testing.T) {
		t.Parallel()
		repo := &stubSmartRepo{}
		svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)
		second := 2
		_, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts: []SmartScheduleAccountMember{
				{AccountID: 12, Platform: PlatformAnthropic, SortOrder: &second},
				{AccountID: 11, Platform: PlatformAnthropic},
			},
		})
		require.NoError(t, err)
		view, err := svc.Get(ctx, 16)
		require.NoError(t, err)
		members := view.Platforms[PlatformAnthropic].Accounts
		require.Len(t, members, 2)
		require.Equal(t, int64(12), members[0].AccountID)
		require.Equal(t, 2, *members[0].SortOrder)
		require.Equal(t, 3, members[0].Priority)
		require.Equal(t, int64(11), members[1].AccountID)
		require.Nil(t, members[1].SortOrder)
		require.Equal(t, 80, members[1].Priority)
	})

	t.Run("patch sort_order does not change pair caps", func(t *testing.T) {
		t.Parallel()
		capN := 4
		first := 1
		repo := &stubSmartRepo{}
		svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)
		_, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts: []SmartScheduleAccountMember{
				{AccountID: 11, Platform: PlatformAnthropic, MaxConcurrency: &capN},
				{AccountID: 12, Platform: PlatformAnthropic, SortOrder: &first},
			},
		})
		require.NoError(t, err)
		view, err := svc.PatchSortOrders(ctx, 16, PlatformAnthropic, []SmartScheduleSortAssignment{
			{AccountID: 12, SortOrder: 1},
			{AccountID: 11, SortOrder: 2},
		})
		require.NoError(t, err)
		byID := map[int64]SmartScheduleAccountMember{}
		for _, member := range view.Platforms[PlatformAnthropic].Accounts {
			byID[member.AccountID] = member
		}
		require.Equal(t, 1, *byID[12].SortOrder)
		require.Equal(t, 2, *byID[11].SortOrder)
		require.Equal(t, 4, *byID[11].MaxConcurrency)
		require.Nil(t, byID[12].MaxConcurrency)
		require.Equal(t, int64(12), view.Platforms[PlatformAnthropic].Accounts[0].AccountID)
	})

	t.Run("patch rejects accounts outside the pool", func(t *testing.T) {
		t.Parallel()
		repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(11, 0, nil))}
		svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)
		_, err := svc.PatchSortOrders(ctx, 16, PlatformAnthropic, []SmartScheduleSortAssignment{
			{AccountID: 12, SortOrder: 1},
		})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_UNKNOWN_ACCOUNT", infraerrors.Reason(err))
	})
}

func TestUserSmartScheduleService_HydratesLiveAccountPriorityNotSortOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	first := 1
	second := 2
	repo := &stubSmartRepo{}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic, Priority: 80},
		{ID: 12, Platform: PlatformAnthropic, Priority: 3},
	}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)
	_, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
		Enabled:         true,
		CooldownMinutes: 15,
		Accounts: []SmartScheduleAccountMember{
			{AccountID: 11, Platform: PlatformAnthropic, SortOrder: &first, Priority: 1},
			{AccountID: 12, Platform: PlatformAnthropic, SortOrder: &second, Priority: 2},
		},
	})
	require.NoError(t, err)
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	byID := map[int64]SmartScheduleAccountMember{}
	for _, member := range view.Platforms[PlatformAnthropic].Accounts {
		byID[member.AccountID] = member
	}
	require.Equal(t, 1, *byID[11].SortOrder)
	require.Equal(t, 80, byID[11].Priority, "GET must hydrate live accounts.priority, not sort_order or write payload")
	require.Equal(t, 2, *byID[12].SortOrder)
	require.Equal(t, 3, byID[12].Priority)
}

func TestCompareSmartScheduleMemberIDs(t *testing.T) {
	t.Parallel()
	orders := map[int64]int{12: 1, 11: 2}
	require.True(t, compareSmartScheduleMemberIDs(12, 11, orders))
	require.False(t, compareSmartScheduleMemberIDs(11, 12, orders))
	require.True(t, compareSmartScheduleMemberIDs(8, 9, nil), "null sort_order keeps id order")
	require.True(t, compareSmartScheduleMemberIDs(12, 99, orders), "assigned before unset")
	require.False(t, compareSmartScheduleMemberIDs(99, 12, orders))
}

func TestUserSmartScheduleService_DropsDeletedPoolMembers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ghost := int64(1706)
	repo := &stubSmartRepo{bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
		PlatformOpenAI: {
			Enabled:         true,
			CooldownMinutes: 15,
			AccountIDs:      map[int64]struct{}{11: {}, ghost: {}},
			SortOrders:      map[int64]int{11: 1, ghost: 2},
		},
	}}}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformOpenAI, Priority: 7},
	}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)

	t.Run("get omits soft-deleted members", func(t *testing.T) {
		t.Parallel()
		view, err := svc.Get(ctx, 220)
		require.NoError(t, err)
		members := view.Platforms[PlatformOpenAI].Accounts
		require.Len(t, members, 1)
		require.Equal(t, int64(11), members[0].AccountID)
		require.Equal(t, 7, members[0].Priority)
		require.True(t, view.Platforms[PlatformOpenAI].Enabled)
	})

	t.Run("put strips in-pool ghosts and keeps new live accounts", func(t *testing.T) {
		t.Parallel()
		localRepo := &stubSmartRepo{bundle: cloneSmartBundle(repo.bundle)}
		localAccounts := &stubSmartAccountRepo{accounts: []*Account{
			{ID: 11, Platform: PlatformOpenAI, Priority: 7},
			{ID: 1718, Platform: PlatformOpenAI, Priority: 4},
		}}
		local := NewUserSmartScheduleService(localRepo, nil, localAccounts, nil, nil)
		view, err := local.PutPlatform(ctx, 220, PlatformOpenAI, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts: []SmartScheduleAccountMember{
				{AccountID: 11, Platform: PlatformOpenAI},
				{AccountID: ghost, Platform: PlatformOpenAI},
				{AccountID: 1718, Platform: PlatformOpenAI},
			},
		})
		require.NoError(t, err)
		ids := make([]int64, 0, len(view.Platforms[PlatformOpenAI].Accounts))
		for _, member := range view.Platforms[PlatformOpenAI].Accounts {
			ids = append(ids, member.AccountID)
		}
		require.Equal(t, []int64{11, 1718}, ids)
		require.NotContains(t, localRepo.bundle.Policies[PlatformOpenAI].AccountIDs, ghost)
	})

	t.Run("put still rejects unknown ids that were never in the pool", func(t *testing.T) {
		t.Parallel()
		_, err := svc.PutPlatform(ctx, 220, PlatformOpenAI, SmartSchedulePlatformWrite{
			Enabled:         true,
			CooldownMinutes: 15,
			Accounts: []SmartScheduleAccountMember{
				{AccountID: 11, Platform: PlatformOpenAI},
				{AccountID: 99999, Platform: PlatformOpenAI},
			},
		})
		require.Error(t, err)
		require.Equal(t, "SMART_SCHEDULE_UNKNOWN_ACCOUNT", infraerrors.Reason(err))
	})
}

func TestAdmitsScheduleUser_AGOffResumeUsesOpenAIShard(t *testing.T) {
	t.Parallel()
	p50 := 50
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	lookup := &memorySmartLookup{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, &p50),
			PlatformAntigravity: {Enabled: false, AccountIDs: map[int64]struct{}{7: {}}},
		}},
		pair:        map[string]*PairQualityLive{smartPairKey(7, 16): breachedPairLive(400)},
		resumeUntil: map[string]int64{smartPairPlatformKey(7, 16, PlatformAntigravity): time.Now().UTC().Add(20 * time.Minute).Unix()},
	}
	require.False(t, admitsScheduleUser(agGroupScheduleCtx(16), oai, nil, lookup), "AG-off 豁免期 on AG shard must not admit; openai shard still evaluates")
	require.Equal(t, 1, lookup.startCalls)
	delete(lookup.cooldownUntil, smartPairKey(7, 16))
	lookup.resumeUntil[smartPairPlatformKey(7, 16, PlatformOpenAI)] = time.Now().UTC().Add(20 * time.Minute).Unix()
	require.True(t, admitsScheduleUser(agGroupScheduleCtx(16), oai, nil, lookup), "AG off must honor openai 豁免期")
}

func TestAdmitsScheduleUser_PairResumeDoesNotLeakAcrossPools(t *testing.T) {
	t.Parallel()
	p50 := 50
	oai := &Account{ID: 7, Platform: PlatformOpenAI}
	lookup := &memorySmartLookup{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, &p50),
			PlatformAntigravity: enabledSmartPolicy(7, 0, &p50),
		}},
		pair:        map[string]*PairQualityLive{smartPairKey(7, 16): breachedPairLive(400)},
		resumeUntil: map[string]int64{smartPairPlatformKey(7, 16, PlatformAntigravity): time.Now().UTC().Add(20 * time.Minute).Unix()},
	}
	agCtx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	agCtx = context.WithValue(agCtx, ctxkey.Group, &Group{ID: 15, Platform: PlatformAntigravity, Status: StatusActive, Hydrated: true})
	oaiCtx := context.WithValue(context.Background(), ctxkey.UserID, int64(16))
	oaiCtx = context.WithValue(oaiCtx, ctxkey.Group, &Group{ID: 19, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true})

	require.True(t, admitsScheduleUser(agCtx, oai, nil, lookup), "AG 豁免期 must fail-open AG pool")
	require.Equal(t, 0, lookup.startCalls)
	require.False(t, admitsScheduleUser(oaiCtx, oai, nil, lookup), "openai pool must still evaluate and cool")
	require.Equal(t, 1, lookup.startCalls)
}

func TestSetPairAdmission_OmittedPlatformOnlyTouchesAccountPlatform(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache := &admissionCacheRecorder{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
		}},
		resumeUntil: map[string]int64{
			smartPairPlatformKey(7, 16, PlatformAntigravity): time.Now().UTC().Add(20 * time.Minute).Unix(),
		},
	}
	svc := NewUserSmartScheduleService(nil, cache, &stubSmartAccountRepo{accounts: []*Account{
		{ID: 7, Platform: PlatformOpenAI},
	}}, &liveQualityCacheStub{}, nil)
	_, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionResumed)
	require.NoError(t, err)
	require.True(t, cache.PairResumeActive(ctx, 7, 16, PlatformOpenAI, time.Now().UTC()))
	require.True(t, cache.PairResumeActive(ctx, 7, 16, PlatformAntigravity, time.Now().UTC()), "omitted platform must not clear the other pool")
}

func TestGetPairQualityBatch_DualMembershipRequiresPlatform(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 80
	cache := &observeCacheStub{
		bundle: &UserSmartScheduleBundle{Policies: map[string]*SmartSchedulePlatformPolicy{
			PlatformOpenAI:      enabledSmartPolicy(7, 0, nil),
			PlatformAntigravity: enabledSmartPolicy(7, 0, nil),
		}},
		live: map[string]*PairQualityLive{smartPairKey(7, 16): ApplyPairQualityIngest(nil, 3, true, &p50)},
	}
	svc := NewUserSmartScheduleService(&stubSmartRepo{bundle: cache.bundle}, cache, nil, nil, nil)

	skipped, err := svc.GetPairQualityBatch(ctx, 16, []int64{7}, "")
	require.NoError(t, err)
	_, leaked := skipped.Pairs["7"]
	require.False(t, leaked, "dual membership without platform must not collapse by account id")

	oaiBatch, err := svc.GetPairQualityBatch(ctx, 16, []int64{7}, PlatformOpenAI)
	require.NoError(t, err)
	require.Contains(t, oaiBatch.Pairs, "7")

	_, err = svc.GetPairQualityDetailForAccount(ctx, 16, 7)
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_PLATFORM_REQUIRED")
}

func TestPutPlatform_ChangingTTFTNDoesNotRewriteSuccessN(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p50 := 200
	policy := enabledSmartPolicy(11, 2, &p50)
	policy.QualityMinTTFTSamples = intPtr(10)
	policy.QualityMinSuccessSamples = intPtr(20)
	repo := &stubSmartRepo{bundle: smartBundle(PlatformAnthropic, policy)}
	accounts := &stubSmartAccountRepo{accounts: []*Account{
		{ID: 11, Platform: PlatformAnthropic},
	}}
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, nil)
	view, err := svc.PutPlatform(ctx, 16, PlatformAnthropic, SmartSchedulePlatformWrite{
		Enabled:               true,
		CooldownMinutes:       15,
		QualityMaxP50TTFTMs:   &p50,
		QualityWindowN:        intPtr(20),
		QualityMinTTFTSamples: intPtr(4),
		Accounts:              []SmartScheduleAccountMember{{AccountID: 11, Platform: PlatformAnthropic}},
	})
	require.NoError(t, err)
	got := view.Platforms[PlatformAnthropic]
	require.NotNil(t, got.QualityMinTTFTSamples)
	require.NotNil(t, got.QualityMinSuccessSamples)
	require.Equal(t, 4, *got.QualityMinTTFTSamples)
	require.Equal(t, 20, *got.QualityMinSuccessSamples)
}
