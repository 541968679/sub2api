package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type minimalProxyRepo struct {
	proxy *Proxy
	err   error
}

func (r *minimalProxyRepo) Create(context.Context, *Proxy) error { return nil }
func (r *minimalProxyRepo) GetByID(context.Context, int64) (*Proxy, error) {
	return r.proxy, r.err
}
func (r *minimalProxyRepo) ListByIDs(context.Context, []int64) ([]Proxy, error) { return nil, nil }
func (r *minimalProxyRepo) Update(context.Context, *Proxy) error                { return nil }
func (r *minimalProxyRepo) Delete(context.Context, int64) error                 { return nil }
func (r *minimalProxyRepo) List(context.Context, pagination.PaginationParams) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *minimalProxyRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string) ([]Proxy, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *minimalProxyRepo) ListWithFiltersAndAccountCount(context.Context, pagination.PaginationParams, string, string, string) ([]ProxyWithAccountCount, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (r *minimalProxyRepo) ListActive(context.Context) ([]Proxy, error) { return nil, nil }
func (r *minimalProxyRepo) ListActiveWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	return nil, nil
}
func (r *minimalProxyRepo) ListPoolEnabledWithAccountCount(context.Context) ([]ProxyWithAccountCount, error) {
	return nil, nil
}
func (r *minimalProxyRepo) ExistsByHostPortAuth(context.Context, string, int, string, string) (bool, error) {
	return false, nil
}
func (r *minimalProxyRepo) CountAccountsByProxyID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *minimalProxyRepo) ClearProxyIDForAccounts(context.Context, int64) (int64, error) {
	return 0, nil
}
func (r *minimalProxyRepo) ListAccountSummariesByProxyID(context.Context, int64) ([]ProxyAccountSummary, error) {
	return nil, nil
}

func TestModerationHTTPClient_DirectWhenNoProxyID(t *testing.T) {
	svc := &ContentModerationService{httpClient: &http.Client{Timeout: time.Second}}
	client, err := svc.moderationHTTPClient(context.Background(), &ContentModerationConfig{}, time.Second)
	require.NoError(t, err)
	require.Same(t, svc.httpClient, client)
}

func TestModerationHTTPClient_ProxyIDWithoutRepoErrors(t *testing.T) {
	svc := &ContentModerationService{}
	id := int64(7)
	_, err := svc.moderationHTTPClient(context.Background(), &ContentModerationConfig{ProxyID: &id}, time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy repository is unavailable")
}

func TestModerationHTTPClient_InactiveProxyErrors(t *testing.T) {
	id := int64(3)
	svc := &ContentModerationService{
		proxyRepo: &minimalProxyRepo{proxy: &Proxy{ID: id, Status: "inactive", Protocol: "http", Host: "127.0.0.1", Port: 8080}},
	}
	_, err := svc.moderationHTTPClient(context.Background(), &ContentModerationConfig{ProxyID: &id}, time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not active")
}
