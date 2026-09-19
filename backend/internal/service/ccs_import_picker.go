package service

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ccsImportUpstreamFetchTimeout = 12 * time.Second
	ccsImportPerAccountTimeout    = 5 * time.Second
	ccsImportFetchParallelism     = 4
)

// UpstreamModelsFetcher loads an account's live upstream GET /v1/models list.
type UpstreamModelsFetcher interface {
	FetchUpstreamSupportedModels(ctx context.Context, account *Account) ([]string, error)
}

// CollectGroupAccountUpstreamModelIDs unions live upstream /v1/models IDs from
// every non-shadow account in the group. Failed accounts are skipped.
func CollectGroupAccountUpstreamModelIDs(ctx context.Context, fetcher UpstreamModelsFetcher, accounts []Account) []string {
	if fetcher == nil || len(accounts) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, ccsImportUpstreamFetchTimeout)
	defer cancel()

	type result struct {
		ids []string
	}
	results := make(chan result, len(accounts))
	sem := make(chan struct{}, ccsImportFetchParallelism)
	var wg sync.WaitGroup

	for i := range accounts {
		acc := accounts[i]
		if acc.ParentAccountID != nil {
			continue
		}
		wg.Add(1)
		go func(account Account) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			reqCtx, reqCancel := context.WithTimeout(ctx, ccsImportPerAccountTimeout)
			defer reqCancel()
			ids, err := fetcher.FetchUpstreamSupportedModels(reqCtx, &account)
			if err != nil {
				slog.Warn("ccs_import_upstream_models_failed",
					"account_id", account.ID,
					"platform", account.Platform,
					"error", err)
				return
			}
			results <- result{ids: ids}
		}(acc)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var merged []string
	for item := range results {
		merged = append(merged, item.ids...)
	}
	return uniqueTrimmedModelIDsSorted(merged)
}

func uniqueTrimmedModelIDsSorted(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// CcsImportPickerModelIDs is the GET /v1/models source for groups with
// ccs_import_model_picker_enabled: live upstream union, default model pinned first.
func CcsImportPickerModelIDs(ctx context.Context, fetcher UpstreamModelsFetcher, accounts []Account, defaultModel string) []string {
	return mergeCcsImportPickerOptions(defaultModel, CollectGroupAccountUpstreamModelIDs(ctx, fetcher, accounts))
}

func mergeCcsImportPickerOptions(defaultModel string, ids []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(ids)+1)
	trimmedDefault := strings.TrimSpace(defaultModel)
	if trimmedDefault != "" {
		out = append(out, trimmedDefault)
		seen[strings.ToLower(trimmedDefault)] = struct{}{}
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	return out
}
