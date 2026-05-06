package service

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	antigravityCreditSampleAccountIDsEnv = "SUB2API_ANTIGRAVITY_CREDIT_SAMPLE_ACCOUNT_IDS"
	antigravityCreditSampleTimeout       = 20 * time.Second
	antigravityCreditSampleConfidenceHi  = "high"
	antigravityCreditSampleConfidenceLo  = "low"
	antigravityCreditSampleConfidenceErr = "error"
)

type AntigravityCreditRequestSample struct {
	UsageLogID       *int64
	RequestID        string
	AccountID        int64
	APIKeyID         *int64
	UserID           *int64
	Email            string
	CreditType       string
	BeforeAmount     *float64
	AfterAmount      *float64
	DeltaAmount      *float64
	BeforeCapturedAt *time.Time
	AfterCapturedAt  *time.Time
	Confidence       string
	Error            string
	CreatedAt        time.Time
}

type AntigravityCreditSampleRepository interface {
	Insert(ctx context.Context, sample *AntigravityCreditRequestSample) error
}

type antigravityCreditBalance struct {
	email      string
	amounts    map[string]float64
	capturedAt time.Time
	err        error
}

type antigravityCreditSampleSpan struct {
	enabled bool
	before  *antigravityCreditBalance
}

type AntigravityCreditSampler struct {
	repo    AntigravityCreditSampleRepository
	fetcher *AntigravityQuotaFetcher
}

func NewAntigravityCreditSampler(repo AntigravityCreditSampleRepository, fetcher *AntigravityQuotaFetcher) *AntigravityCreditSampler {
	return &AntigravityCreditSampler{repo: repo, fetcher: fetcher}
}

func (s *AntigravityCreditSampler) EnabledForAccount(accountID int64) bool {
	ids := parseDiagnosticIDSet(os.Getenv(antigravityCreditSampleAccountIDsEnv))
	if len(ids) == 0 {
		return false
	}
	return ids[accountID]
}

func (s *AntigravityCreditSampler) Begin(ctx context.Context, account *Account) *antigravityCreditSampleSpan {
	if s == nil || s.repo == nil || s.fetcher == nil || account == nil || !s.EnabledForAccount(account.ID) {
		return &antigravityCreditSampleSpan{}
	}
	balance := s.fetch(ctx, account)
	return &antigravityCreditSampleSpan{enabled: true, before: balance}
}

func (s *AntigravityCreditSampler) Finish(ctx context.Context, span *antigravityCreditSampleSpan, account *Account, usageLog *UsageLog) {
	if s == nil || s.repo == nil || s.fetcher == nil || span == nil || !span.enabled || account == nil || usageLog == nil {
		return
	}
	after := s.fetch(ctx, account)
	s.writeSamples(ctx, account, usageLog, span.before, after)
}

func (s *AntigravityCreditSampler) fetch(ctx context.Context, account *Account) *antigravityCreditBalance {
	result := &antigravityCreditBalance{
		email:      antigravitySamplerAccountEmail(account),
		amounts:    make(map[string]float64),
		capturedAt: time.Now(),
	}
	if account == nil {
		result.err = errors.New("account is nil")
		return result
	}
	fetchCtx, cancel := context.WithTimeout(detachSamplerContext(ctx), antigravityCreditSampleTimeout)
	defer cancel()
	proxyURL := s.fetcher.GetProxyURL(fetchCtx, account)
	quota, err := s.fetcher.FetchQuota(fetchCtx, account, proxyURL)
	result.capturedAt = time.Now()
	if err != nil {
		result.err = err
		return result
	}
	if quota == nil || quota.UsageInfo == nil {
		return result
	}
	for _, credit := range quota.UsageInfo.AICredits {
		creditType := strings.TrimSpace(credit.CreditType)
		if creditType == "" {
			creditType = "UNKNOWN"
		}
		result.amounts[creditType] = credit.Amount
	}
	return result
}

func (s *AntigravityCreditSampler) writeSamples(ctx context.Context, account *Account, usageLog *UsageLog, before, after *antigravityCreditBalance) {
	sampleCtx, cancel := context.WithTimeout(detachSamplerContext(ctx), 10*time.Second)
	defer cancel()
	types := make(map[string]struct{})
	if before != nil {
		for typ := range before.amounts {
			types[typ] = struct{}{}
		}
	}
	if after != nil {
		for typ := range after.amounts {
			types[typ] = struct{}{}
		}
	}
	if len(types) == 0 {
		types["UNKNOWN"] = struct{}{}
	}
	for creditType := range types {
		beforeAmount, beforeAt := sampleAmount(before, creditType)
		afterAmount, afterAt := sampleAmount(after, creditType)
		delta, confidence := computeCreditDelta(beforeAmount, afterAmount)
		errText := joinSamplerErrors(before, after)
		if errText != "" {
			confidence = antigravityCreditSampleConfidenceErr
		}
		usageLogID := usageLog.ID
		apiKeyID := usageLog.APIKeyID
		userID := usageLog.UserID
		sample := &AntigravityCreditRequestSample{
			UsageLogID:       optionalPositiveInt64(usageLogID),
			RequestID:        usageLog.RequestID,
			AccountID:        account.ID,
			APIKeyID:         optionalPositiveInt64(apiKeyID),
			UserID:           optionalPositiveInt64(userID),
			Email:            coalesceSamplerEmail(account, after, before),
			CreditType:       creditType,
			BeforeAmount:     beforeAmount,
			AfterAmount:      afterAmount,
			DeltaAmount:      delta,
			BeforeCapturedAt: beforeAt,
			AfterCapturedAt:  afterAt,
			Confidence:       confidence,
			Error:            errText,
			CreatedAt:        time.Now(),
		}
		_ = s.repo.Insert(sampleCtx, sample)
	}
}

func parseDiagnosticIDSet(raw string) map[int64]bool {
	out := make(map[int64]bool)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err == nil && id > 0 {
			out[id] = true
		}
	}
	return out
}

func detachSamplerContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func sampleAmount(balance *antigravityCreditBalance, creditType string) (*float64, *time.Time) {
	if balance == nil {
		return nil, nil
	}
	value, ok := balance.amounts[creditType]
	if !ok {
		return nil, &balance.capturedAt
	}
	return &value, &balance.capturedAt
}

func computeCreditDelta(before, after *float64) (*float64, string) {
	if before == nil || after == nil {
		return nil, antigravityCreditSampleConfidenceLo
	}
	deltaValue := *before - *after
	return &deltaValue, antigravityCreditSampleConfidenceHi
}

func joinSamplerErrors(before, after *antigravityCreditBalance) string {
	var parts []string
	if before != nil && before.err != nil {
		parts = append(parts, "before: "+before.err.Error())
	}
	if after != nil && after.err != nil {
		parts = append(parts, "after: "+after.err.Error())
	}
	return strings.Join(parts, "; ")
}

func optionalPositiveInt64(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func antigravitySamplerAccountEmail(account *Account) string {
	if account == nil || account.Credentials == nil {
		return ""
	}
	if raw, ok := account.Credentials["email"].(string); ok {
		return strings.TrimSpace(strings.ToLower(raw))
	}
	return ""
}

func coalesceSamplerEmail(account *Account, balances ...*antigravityCreditBalance) string {
	for _, balance := range balances {
		if balance != nil && strings.TrimSpace(balance.email) != "" {
			return balance.email
		}
	}
	return antigravitySamplerAccountEmail(account)
}
