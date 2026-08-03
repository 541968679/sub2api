package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	httpclient "github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

const (
	upstreamBalanceProbeTimeout = 15 * time.Second
	upstreamBalanceEnvDisable   = "SUB2API_UPSTREAM_BALANCE_PROBE"

	balanceSourceCreditGrants      = "credit_grants"
	balanceSourceSubscriptionUsage = "subscription_usage"
	// Sub2API-compatible gateways (e.g. ZeroCode): GET /v1/usage with API key.
	balanceSourceSub2APIUsage = "sub2api_v1_usage"
)

// UpstreamBalanceResult is a successful or failed balance probe outcome.
type UpstreamBalanceResult struct {
	BalanceUSD float64
	Source     string
	Error      string
	FetchedAt  time.Time
}

type openAICreditGrantsResponse struct {
	TotalAvailable float64 `json:"total_available"`
	TotalRemaining float64 `json:"total_remaining"`
	TotalGranted   float64 `json:"total_granted"`
	TotalUsed      float64 `json:"total_used"`
}

type openAISubscriptionResponse struct {
	HardLimitUSD       float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD float64 `json:"system_hard_limit_usd"`
	SoftLimitUSD       float64 `json:"soft_limit_usd"`
	HasPaymentMethod   bool    `json:"has_payment_method"`
}

type openAIUsageResponse struct {
	TotalUsage float64 `json:"total_usage"` // unit: 0.01 USD
}

// IsUpstreamBalanceProbeEnabled returns false when the kill-switch is set.
func IsUpstreamBalanceProbeEnabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(upstreamBalanceEnvDisable)))
	return v != "0" && v != "false" && v != "off" && v != "no"
}

// SupportsUpstreamBalanceProbe reports whether this account is in MVP scope.
func SupportsUpstreamBalanceProbe(account *Account) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	switch account.Platform {
	case PlatformOpenAI, PlatformAnthropic:
		return true
	default:
		return false
	}
}

// ResolveUpstreamBalanceBaseURL picks the probe base URL for the account.
func ResolveUpstreamBalanceBaseURL(account *Account) string {
	if account == nil {
		return ""
	}
	if account.IsOpenAIApiKey() {
		return strings.TrimSpace(account.GetOpenAIBaseURL())
	}
	if account.Platform == PlatformAnthropic && account.Type == AccountTypeAPIKey {
		base := strings.TrimSpace(account.GetBaseURL())
		if base == "" {
			return "https://api.anthropic.com"
		}
		return base
	}
	return strings.TrimSpace(account.GetCredential("base_url"))
}

// JoinOpenAIBillingURL joins base URL with a /v1/dashboard/billing/... path.
// Handles bases that already end with /v1.
func JoinOpenAIBillingURL(baseURL, billingPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path := billingPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// billingPath expected like /v1/dashboard/billing/credit_grants
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

// isOfficialOpenAIOrAnthropicHost reports first-party API hosts that do not
// expose Sub2API-style GET /v1/usage balance.
func isOfficialOpenAIOrAnthropicHost(baseURL string) bool {
	u := strings.ToLower(strings.TrimSpace(baseURL))
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	host := u
	if i := strings.Index(u, "/"); i >= 0 {
		host = u[:i]
	}
	switch host {
	case "api.openai.com", "api.anthropic.com":
		return true
	default:
		return false
	}
}

// ProbeUpstreamBalance fetches prepaid-style balance via compatible billing APIs.
//
// Probe order for third-party / self-hosted (e.g. ZeroCode = Sub2API):
//  1. GET {base}/v1/usage  → balance / remaining (Sub2API public usage summary)
//  2. GET {base}/v1/dashboard/billing/credit_grants
//  3. subscription + usage (OpenAI-shape hard_limit - spent)
//
// Official OpenAI/Anthropic hosts skip step 1 (they do not speak Sub2API /v1/usage).
func ProbeUpstreamBalance(ctx context.Context, account *Account) UpstreamBalanceResult {
	now := time.Now().UTC()
	result := UpstreamBalanceResult{FetchedAt: now}
	if !IsUpstreamBalanceProbeEnabled() {
		result.Error = "balance probe disabled"
		return result
	}
	if !SupportsUpstreamBalanceProbe(account) {
		result.Error = "account type does not support balance probe"
		return result
	}

	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		result.Error = "missing api_key"
		return result
	}
	baseURL := ResolveUpstreamBalanceBaseURL(account)
	if baseURL == "" {
		result.Error = "missing base_url"
		return result
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:           proxyURL,
		Timeout:            upstreamBalanceProbeTimeout,
		ValidateResolvedIP: true,
		AllowPrivateHosts:  true,
	})
	if err != nil {
		result.Error = fmt.Sprintf("http client: %v", err)
		return result
	}

	appendErr := func(probeErr string) {
		if probeErr == "" {
			return
		}
		if result.Error != "" {
			result.Error = result.Error + "; " + probeErr
		} else {
			result.Error = probeErr
		}
	}

	// 1) Sub2API-compatible /v1/usage (ZeroCode and same-stack gateways).
	// Prefer first on third-party hosts: credit_grants is often 404 there.
	if !isOfficialOpenAIOrAnthropicHost(baseURL) {
		if bal, ok, probeErr := fetchSub2APIUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.Source = balanceSourceSub2APIUsage
			result.Error = ""
			return result
		} else {
			appendErr(probeErr)
		}
	}

	// 2) OpenAI-shape credit_grants
	if bal, ok, probeErr := fetchCreditGrantsBalance(ctx, client, account, apiKey, baseURL); ok {
		result.BalanceUSD = bal
		result.Source = balanceSourceCreditGrants
		result.Error = ""
		return result
	} else {
		appendErr(probeErr)
	}

	// 3) subscription + usage
	if bal, ok, probeErr := fetchSubscriptionUsageBalance(ctx, client, account, apiKey, baseURL); ok {
		result.BalanceUSD = bal
		result.Source = balanceSourceSubscriptionUsage
		result.Error = ""
		return result
	} else {
		appendErr(probeErr)
	}

	// 4) Last chance: Sub2API /v1/usage even on unknown official-like hosts.
	if isOfficialOpenAIOrAnthropicHost(baseURL) {
		if bal, ok, probeErr := fetchSub2APIUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.Source = balanceSourceSub2APIUsage
			result.Error = ""
			return result
		} else {
			appendErr(probeErr)
		}
	}

	if result.Error == "" {
		result.Error = "balance probe failed"
	}
	return result
}

// sub2apiUsageBalanceResponse matches GatewayHandler.Usage JSON for balance mode.
type sub2apiUsageBalanceResponse struct {
	Mode      string   `json:"mode"`
	Balance   *float64 `json:"balance"`
	Remaining *float64 `json:"remaining"`
	Unit      string   `json:"unit"`
	IsValid   *bool    `json:"isValid"`
	Quota     *struct {
		Remaining float64 `json:"remaining"`
		Limit     float64 `json:"limit"`
		Used      float64 `json:"used"`
	} `json:"quota"`
}

func fetchSub2APIUsageBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (float64, bool, string) {
	url := JoinOpenAIBillingURL(baseURL, "/v1/usage")
	body, status, err := doBalanceGET(ctx, client, account, apiKey, url)
	if err != nil {
		return 0, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, false, fmt.Sprintf("sub2api /v1/usage status %d: %s", status, truncateForErr(body, 200))
	}
	if looksLikeHTML(body) {
		return 0, false, "sub2api /v1/usage returned HTML (not JSON)"
	}
	var resp sub2apiUsageBalanceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Sprintf("sub2api /v1/usage decode: %v", err)
	}
	// Prefer explicit wallet balance, then top-level remaining, then key quota remaining.
	if resp.Balance != nil {
		return *resp.Balance, true, ""
	}
	if resp.Remaining != nil {
		return *resp.Remaining, true, ""
	}
	if resp.Quota != nil {
		return resp.Quota.Remaining, true, ""
	}
	// Require recognizable Sub2API fields so we do not treat unrelated /v1/usage as balance.
	if resp.Mode == "" && resp.IsValid == nil {
		return 0, false, "sub2api /v1/usage: no balance/remaining fields"
	}
	return 0, false, "sub2api /v1/usage: balance not present"
}

func looksLikeHTML(body []byte) bool {
	s := strings.TrimSpace(strings.ToLower(string(body)))
	return strings.HasPrefix(s, "<!doctype") || strings.HasPrefix(s, "<html")
}

func fetchCreditGrantsBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (float64, bool, string) {
	url := JoinOpenAIBillingURL(baseURL, "/v1/dashboard/billing/credit_grants")
	body, status, err := doBalanceGET(ctx, client, account, apiKey, url)
	if err != nil {
		return 0, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, false, fmt.Sprintf("credit_grants status %d: %s", status, truncateForErr(body, 200))
	}
	if looksLikeHTML(body) {
		return 0, false, "credit_grants returned HTML (not JSON)"
	}
	var resp openAICreditGrantsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, false, fmt.Sprintf("credit_grants decode: %v", err)
	}
	// Prefer total_available; some gateways use total_remaining.
	if resp.TotalAvailable > 0 || resp.TotalGranted > 0 || resp.TotalUsed > 0 {
		return resp.TotalAvailable, true, ""
	}
	if resp.TotalRemaining > 0 {
		return resp.TotalRemaining, true, ""
	}
	// Zero available is still a valid reading when object decoded.
	return resp.TotalAvailable, true, ""
}

func fetchSubscriptionUsageBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (float64, bool, string) {
	subURL := JoinOpenAIBillingURL(baseURL, "/v1/dashboard/billing/subscription")
	body, status, err := doBalanceGET(ctx, client, account, apiKey, subURL)
	if err != nil {
		return 0, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, false, fmt.Sprintf("subscription status %d: %s", status, truncateForErr(body, 200))
	}
	var sub openAISubscriptionResponse
	if err := json.Unmarshal(body, &sub); err != nil {
		return 0, false, fmt.Sprintf("subscription decode: %v", err)
	}
	hardLimit := sub.HardLimitUSD
	if hardLimit <= 0 {
		hardLimit = sub.SystemHardLimitUSD
	}
	if hardLimit <= 0 {
		hardLimit = sub.SoftLimitUSD
	}

	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !sub.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}
	usagePath := fmt.Sprintf("/v1/dashboard/billing/usage?start_date=%s&end_date=%s", startDate, endDate)
	usageURL := JoinOpenAIBillingURL(baseURL, usagePath)
	// JoinOpenAIBillingURL may mishandle query if path has ? — build carefully.
	if strings.Contains(usagePath, "?") {
		basePath, query, _ := strings.Cut(usagePath, "?")
		usageURL = JoinOpenAIBillingURL(baseURL, basePath) + "?" + query
	}

	body, status, err = doBalanceGET(ctx, client, account, apiKey, usageURL)
	if err != nil {
		return 0, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, false, fmt.Sprintf("usage status %d: %s", status, truncateForErr(body, 200))
	}
	var usage openAIUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return 0, false, fmt.Sprintf("usage decode: %v", err)
	}
	// TotalUsage is in cents (0.01 USD).
	balance := hardLimit - usage.TotalUsage/100
	return balance, true, ""
}

func doBalanceGET(ctx context.Context, client *http.Client, account *Account, apiKey, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	applyUpstreamBalanceAuth(req, account, apiKey)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func applyUpstreamBalanceAuth(req *http.Request, account *Account, apiKey string) {
	if account != nil && account.Platform == PlatformAnthropic {
		setAnthropicAPIKeyAuthHeader(req.Header, account, apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func truncateForErr(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
