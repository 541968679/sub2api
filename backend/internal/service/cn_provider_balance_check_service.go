package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type cnQuotaProber interface {
	QueryUsage(ctx context.Context, accountID int64) (*CNProviderQuotaProbeResult, error)
}

const cnQuotaProbeConcurrency = 4

type CNProviderBalanceCheckService struct {
	accountRepo  AccountRepository
	quotaService cnQuotaProber
	cfg          *config.Config
}

func (s *CNProviderBalanceCheckService) platforms() []string {
	return []string{PlatformKimi, PlatformDeepseek}
}

func IsOllamaCloudUsageAccount(account *Account) bool {
	if account == nil || !account.IsCNProvider() {
		return false
	}
	baseURL := strings.ToLower(strings.TrimSpace(account.GetCredential("base_url")))
	return strings.Contains(baseURL, "ollama.com")
}

func (s *CNProviderBalanceCheckService) runOnce() {
	if s == nil || s.accountRepo == nil {
		return
	}
	type quotaTarget struct {
		id       int64
		platform string
	}
	var quotaTargets []quotaTarget
	collect := func(accounts []Account) {
		for i := range accounts {
			account := &accounts[i]
			if !account.IsActive() {
				continue
			}
			if IsOllamaCloudUsageAccount(account) {
				continue
			}
			if account.IsCodingPlan() {
				quotaTargets = append(quotaTargets, quotaTarget{id: account.ID, platform: account.Platform})
			}
		}
	}
	for _, platform := range s.platforms() {
		accounts, err := s.accountRepo.ListByPlatform(context.Background(), platform)
		if err != nil {
			log.Printf("[CNBalance] list %s accounts failed: %v", platform, err)
			continue
		}
		collect(accounts)
	}
	if s.quotaService != nil {
		for _, platform := range []string{PlatformZhipu, PlatformMiniMax} {
			accounts, err := s.accountRepo.ListByPlatform(context.Background(), platform)
			if err != nil {
				log.Printf("[CNBalance] list %s accounts failed: %v", platform, err)
				continue
			}
			collect(accounts)
		}
	}
	if len(quotaTargets) == 0 || s.quotaService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sem := make(chan struct{}, cnQuotaProbeConcurrency)
	var wg sync.WaitGroup
	for _, target := range quotaTargets {
		wg.Add(1)
		go func(t quotaTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if _, err := s.quotaService.QueryUsage(ctx, t.id); err != nil {
				log.Printf("[CNBalance] quota probe account %d (%s) failed: %v", t.id, t.platform, err)
			}
		}(target)
	}
	wg.Wait()
}
