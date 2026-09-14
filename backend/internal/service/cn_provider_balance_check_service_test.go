package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type fakeCNQuotaProber struct {
	mu     sync.Mutex
	probed []int64
}

func (f *fakeCNQuotaProber) QueryUsage(ctx context.Context, accountID int64) (*CNProviderQuotaProbeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.probed = append(f.probed, accountID)
	return &CNProviderQuotaProbeResult{Success: true, Persisted: true}, nil
}

type fakeCNCheckRepo struct {
	AccountRepository
	byPlatform map[string][]Account
}

func (r *fakeCNCheckRepo) ListByPlatform(ctx context.Context, platform string) ([]Account, error) {
	return r.byPlatform[platform], nil
}

func TestCNProviderBalanceCheckRunOnceProbesCodingPlanQuota(t *testing.T) {
	kimiActive := Account{ID: 1, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}}
	kimiPaused := Account{ID: 2, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false,
		Credentials: map[string]any{"account_mode": "coding"}}
	kimiInactive := Account{ID: 3, Platform: PlatformKimi, Type: AccountTypeAPIKey, Status: StatusDisabled,
		Credentials: map[string]any{"account_mode": "coding"}}
	zhipuCoding := Account{ID: 4, Platform: PlatformZhipu, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}}
	minimaxCoding := Account{ID: 5, Platform: PlatformMiniMax, Type: AccountTypeAPIKey, Status: StatusActive,
		Credentials: map[string]any{"account_mode": "coding"}}

	repo := &fakeCNCheckRepo{byPlatform: map[string][]Account{
		PlatformKimi:    {kimiActive, kimiPaused, kimiInactive},
		PlatformZhipu:   {zhipuCoding},
		PlatformMiniMax: {minimaxCoding},
	}}
	prober := &fakeCNQuotaProber{}
	svc := &CNProviderBalanceCheckService{
		accountRepo:  repo,
		quotaService: prober,
		cfg:          &config.Config{},
	}

	svc.runOnce()

	require.ElementsMatch(t, []int64{1, 2, 4, 5}, prober.probed)
}
