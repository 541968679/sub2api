//go:build unit

package service

import (
	"context"
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
		ID:           7,
		Platform:     PlatformAnthropic,
		DenyUserIDs:  []int64{16},
		AllowUserIDs: []int64{99},
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
		lookup := &memorySmartLookup{bundle: smartBundle(PlatformAnthropic, enabledSmartPolicy(7, 0, &p50))}
		breached := &liveQualityCacheStub{byID: map[int64]*AccountQualityStats{7: liveQualityStats(4000, 12, 20, 0, 1)}}
		require.False(t, admitsScheduleUser(ctx, denied, breached, lookup))
		require.Equal(t, 1, lookup.startCalls)
		firstUntil := lookup.lastUntilUnix
		require.Greater(t, firstUntil, time.Now().UTC().Unix())

		good := &liveQualityCacheStub{byID: map[int64]*AccountQualityStats{7: liveQualityStats(200, 12, 20, 0, 1)}}
		require.False(t, admitsScheduleUser(ctx, denied, good, lookup))
		lookup.StartCooldown(ctx, 7, 16, 30, time.Now().UTC())
		require.Equal(t, firstUntil, lookup.lastUntilUnix)
		require.Equal(t, firstUntil, lookup.cooldownUntil[smartPairKey(7, 16)])
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
		QualityMinSuccessSamples: policy.QualityMinSuccessSamples,
		QualityMinTTFTSamples:    policy.QualityMinTTFTSamples,
		QualityCondition:         policy.QualityCondition,
		CooldownMinutes:          policy.CooldownMinutes,
		AccountIDs:               map[int64]struct{}{},
		Caps:                     map[int64]int{},
	}
	for _, member := range policy.Accounts {
		next.AccountIDs[member.AccountID] = struct{}{}
		if member.MaxConcurrency != nil && *member.MaxConcurrency >= 1 {
			next.Caps[member.AccountID] = *member.MaxConcurrency
		}
	}
	s.bundle.Policies[platform] = next
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
	counts map[int64]int
}

func (s stubPairConcurrency) GetAccountUserConcurrencyBatch(_ context.Context, accountIDs []int64, _ int64) (map[int64]int, error) {
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
	svc := NewUserSmartScheduleService(repo, nil, accounts, nil, stubPairConcurrency{counts: map[int64]int{11: 2}})
	view, err := svc.Get(ctx, 16)
	require.NoError(t, err)
	require.Len(t, view.Platforms[PlatformAnthropic].Accounts, 1)
	require.Equal(t, 2, view.Platforms[PlatformAnthropic].Accounts[0].CurrentConcurrency)
	require.Equal(t, 3, *view.Platforms[PlatformAnthropic].Accounts[0].MaxConcurrency)
}

