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
	startCalls    int
	lastUntilUnix int64
}

func (m *memorySmartLookup) Lookup(_ context.Context, _ int64) *UserSmartScheduleBundle {
	if m == nil {
		return nil
	}
	return m.bundle
}

func (m *memorySmartLookup) CooldownActive(_ context.Context, accountID, userID int64, now time.Time) bool {
	if m == nil || len(m.cooldownUntil) == 0 {
		return false
	}
	until := m.cooldownUntil[smartPairKey(accountID, userID)]
	return until > now.Unix()
}

func (m *memorySmartLookup) StartCooldown(_ context.Context, accountID, userID int64, minutes int, now time.Time) {
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

func (m *memorySmartLookup) GetPairQuality(_ context.Context, accountID, userID int64) *PairQualityLive {
	if m == nil || len(m.pair) == 0 {
		return nil
	}
	return m.pair[smartPairKey(accountID, userID)]
}

func smartPairKey(accountID, userID int64) string {
	return strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(userID, 10)
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
		lookup.StartCooldown(ctx, 7, 16, 30, time.Now().UTC())
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

	t.Run("under-N pair window does not cooldown", func(t *testing.T) {
		t.Parallel()
		live := &PairQualityLive{N: DefaultSmartScheduleWindowN, TTFTMs: []int{4000, 4100}, OK: []bool{true, true}}
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

func (s *stubSmartRepo) SetMemberPaused(_ context.Context, _ int64, accountID int64, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bundle == nil || s.bundle.Policies == nil {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	for _, policy := range s.bundle.Policies {
		if policy == nil || !policy.HasAccount(accountID) {
			continue
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
	return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
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
func (s stubSmartCache) CooldownActive(_ context.Context, _ int64, _ int64, _ time.Time) bool {
	return false
}
func (s stubSmartCache) StartCooldown(_ context.Context, _ int64, _ int64, _ int, _ time.Time) {}
func (s stubSmartCache) Invalidate(_ context.Context, _ int64) error                           { return nil }
func (s stubSmartCache) ClearCooldown(_ context.Context, _ int64, _ int64) error               { return nil }
func (s stubSmartCache) SetCooldown(_ context.Context, _ int64, _ int64, minutes int, now time.Time) (time.Time, error) {
	return now.Add(time.Duration(ClampSmartScheduleCooldownMinutes(minutes)) * time.Minute), nil
}

func (s stubSmartCache) ApplyMemberPaused(context.Context, int64, int64, bool) error { return nil }

type admissionCacheRecorder struct {
	stubSmartCache
	bundle  *UserSmartScheduleBundle
	cleared int
	setMins int
	setErr  error
}

func (s *admissionCacheRecorder) Lookup(_ context.Context, _ int64) *UserSmartScheduleBundle {
	return s.bundle
}

func (s *admissionCacheRecorder) ClearCooldown(_ context.Context, _ int64, _ int64) error {
	s.cleared++
	return nil
}

func (s *admissionCacheRecorder) SetCooldown(_ context.Context, _ int64, _ int64, minutes int, now time.Time) (time.Time, error) {
	s.setMins = minutes
	if s.setErr != nil {
		return time.Time{}, s.setErr
	}
	return now.Add(time.Duration(ClampSmartScheduleCooldownMinutes(minutes)) * time.Minute), nil
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
	_, err = ParsePairAdmissionState("nope")
	require.Error(t, err)
	require.Contains(t, err.Error(), "SMART_SCHEDULE_ADMISSION_INVALID")
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
	require.True(t, UserQualityResumedChipActive(quality.byID[7], 16, time.Now().UTC()))

	selectable, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionSelectable)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionSelectable, selectable.State)
	require.False(t, UserQualityResumedChipActive(quality.byID[7], 16, time.Now().UTC()))
	require.False(t, UserQualityResumeActive(quality.byID[7], 16, time.Now().UTC()), "selectable must not write w: grace")

	cooling, err := svc.SetPairAdmission(ctx, 7, 16, PairAdmissionCooling)
	require.NoError(t, err)
	require.Equal(t, PairAdmissionCooling, cooling.State)
	require.NotNil(t, cooling.CooldownUntil)
	require.Equal(t, 30, cache.setMins)
	require.Nil(t, quality.byID[7].ResumeUsers)
	require.Nil(t, quality.byID[7].ResumeWatchingUsers)

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

func (s stubSmartCache) GetCooldownUntilBatch(_ context.Context, _ []int64, _ int64, _ time.Time) map[int64]time.Time {
	if s.until == nil {
		return map[int64]time.Time{}
	}
	return s.until
}

func (s stubSmartCache) GetPairQuality(context.Context, int64, int64) *PairQualityLive { return nil }
func (s stubSmartCache) IngestPairQuality(context.Context, int64, int64, int, bool, *int) *PairQualityLive {
	return nil
}
func (s stubSmartCache) ZeroPairQuality(context.Context, int64, int64, string) {}
func (s stubSmartCache) GetPairQualityBatch(context.Context, []int64, int64) map[int64]*PairQualityLive {
	return map[int64]*PairQualityLive{}
}
func (s stubSmartCache) ListPairQualitySnapshots(context.Context, int64, int64, int) []PairQualitySnapshot {
	return nil
}
func (s stubSmartCache) ListPairQualityEvents(context.Context, int64, int64, int) []PairQualityEvent {
	return nil
}
func (s stubSmartCache) AppendPairQualityEvent(context.Context, int64, int64, PairQualityEvent) {}

func breachedPairLive(p50 int) *PairQualityLive {
	n := DefaultSmartScheduleWindowN
	ttft := make([]int, n)
	ok := make([]bool, n)
	for i := 0; i < n; i++ {
		ttft[i] = p50
		ok[i] = true
	}
	live := &PairQualityLive{N: n, TTFTMs: ttft, OK: ok}
	RecomputePairQuality(live)
	return live
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
