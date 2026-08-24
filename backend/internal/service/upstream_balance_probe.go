package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
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
	// New API / one-api style: GET /api/usage/token with API key (Bearer sk-...).
	balanceSourceNewAPITokenUsage = "newapi_usage_token"
	// New API user wallet: GET /api/user/self with system access token + New-Api-User.
	balanceSourceNewAPIUserSelf = "newapi_user_self"

	credentialKeyNewAPIAccessToken = "newapi_access_token"
	credentialKeyNewAPIUserID      = "newapi_user_id"

	// Default New API quota unit: 500000 internal units == $1 USD (from /api/status.quota_per_unit).
	defaultNewAPIQuotaPerUnit = 500000.0
)

// UpstreamBalanceResult is a successful or failed balance probe outcome.
type UpstreamBalanceResult struct {
	// BalanceUSD is remaining/available prepaid balance when known (may be 0 when unlimited).
	BalanceUSD float64
	// UsedUSD is spent amount when the upstream reports it (e.g. New API total_used).
	UsedUSD float64
	// HasUsed is true when UsedUSD was provided by the upstream probe.
	HasUsed bool
	// Unlimited is true when the upstream token has unlimited quota (New API).
	Unlimited bool
	Source    string
	Error     string
	FetchedAt time.Time
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

// originFromBaseURL returns scheme://host from a base URL that may include /v1 path.
func originFromBaseURL(baseURL string) string {
	base := strings.TrimSpace(baseURL)
	base = strings.TrimRight(base, "/")
	// Strip trailing /v1 so /api/usage/token is not under /v1.
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = strings.TrimSuffix(base, "/v1")
		base = strings.TrimSuffix(base, "/V1")
	}
	return strings.TrimRight(base, "/")
}

// ProbeUpstreamBalance fetches prepaid-style balance via compatible billing APIs.
//
// Probe order:
//  0. GET {origin}/api/user/self when credentials.newapi_access_token + newapi_user_id are set
//  1. GET {base}/v1/usage  → balance / remaining (Sub2API / ZeroCode)
//  2. GET {origin}/api/usage/token → New API token_usage (token-bits / one-api)
//  3. GET {base}/v1/dashboard/billing/credit_grants
//  4. subscription + usage (OpenAI-shape hard_limit - spent)
//
// Official OpenAI/Anthropic hosts skip steps 1–2 first unless step 0 applies.
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

	if accessToken, userID, ok := newAPIUserWalletCreds(account); ok {
		bal, used, hasUsed, probeOK, probeErr := fetchNewAPIUserSelfBalance(ctx, client, account, accessToken, userID, baseURL)
		if probeOK {
			result.BalanceUSD = bal
			result.UsedUSD = used
			result.HasUsed = hasUsed
			result.Unlimited = false
			result.Source = balanceSourceNewAPIUserSelf
			result.Error = ""
			return result
		}
		appendErr(probeErr)
	}

	// 1–2) Third-party first: Sub2API then New API (credit_grants often 404 there).
	if !isOfficialOpenAIOrAnthropicHost(baseURL) {
		if bal, ok, probeErr := fetchSub2APIUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.Source = balanceSourceSub2APIUsage
			result.Error = ""
			return result
		} else {
			appendErr(probeErr)
		}
		if bal, used, hasUsed, unlimited, ok, probeErr := fetchNewAPITokenUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.UsedUSD = used
			result.HasUsed = hasUsed
			result.Unlimited = unlimited
			result.Source = balanceSourceNewAPITokenUsage
			result.Error = ""
			return result
		} else {
			appendErr(probeErr)
		}
	}

	// 3) OpenAI-shape credit_grants
	if bal, used, hasUsed, ok, probeErr := fetchCreditGrantsBalance(ctx, client, account, apiKey, baseURL); ok {
		result.BalanceUSD = bal
		result.UsedUSD = used
		result.HasUsed = hasUsed
		result.Source = balanceSourceCreditGrants
		result.Error = ""
		return result
	} else {
		appendErr(probeErr)
	}

	// 4) subscription + usage
	if bal, used, hasUsed, ok, probeErr := fetchSubscriptionUsageBalance(ctx, client, account, apiKey, baseURL); ok {
		result.BalanceUSD = bal
		result.UsedUSD = used
		result.HasUsed = hasUsed
		result.Source = balanceSourceSubscriptionUsage
		result.Error = ""
		return result
	} else {
		appendErr(probeErr)
	}

	// 5) Last chance on official-like hosts: Sub2API then New API.
	if isOfficialOpenAIOrAnthropicHost(baseURL) {
		if bal, ok, probeErr := fetchSub2APIUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.Source = balanceSourceSub2APIUsage
			result.Error = ""
			return result
		} else {
			appendErr(probeErr)
		}
		if bal, used, hasUsed, unlimited, ok, probeErr := fetchNewAPITokenUsageBalance(ctx, client, account, apiKey, baseURL); ok {
			result.BalanceUSD = bal
			result.UsedUSD = used
			result.HasUsed = hasUsed
			result.Unlimited = unlimited
			result.Source = balanceSourceNewAPITokenUsage
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

func newAPIUserWalletCreds(account *Account) (token, userID string, ok bool) {
	if account == nil {
		return "", "", false
	}
	token = strings.TrimSpace(account.GetCredential(credentialKeyNewAPIAccessToken))
	id := account.GetCredentialAsInt64(credentialKeyNewAPIUserID)
	if token == "" || id < 1 {
		return "", "", false
	}
	return token, strconv.FormatInt(id, 10), true
}

type newAPIUserSelfResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    *struct {
		ID        int64   `json:"id"`
		Quota     float64 `json:"quota"`
		UsedQuota float64 `json:"used_quota"`
	} `json:"data"`
}

func fetchNewAPIUserSelfBalance(ctx context.Context, client *http.Client, account *Account, accessToken, userID, baseURL string) (balance, used float64, hasUsed, ok bool, errMsg string) {
	origin := originFromBaseURL(baseURL)
	if origin == "" {
		return 0, 0, false, false, "newapi user/self: empty origin"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/api/user/self", nil)
	if err != nil {
		return 0, 0, false, false, "newapi user/self: bad request"
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("New-Api-User", userID)
	req.Header.Set("Accept", "application/json")
	body, status, err := doBalanceRequest(client, req)
	if err != nil {
		return 0, 0, false, false, "newapi user/self: " + err.Error()
	}
	if status != http.StatusOK {
		return 0, 0, false, false, fmt.Sprintf("newapi user/self status %d: %s", status, truncateForErr(body, 200))
	}
	if looksLikeHTML(body) {
		return 0, 0, false, false, "newapi user/self returned HTML"
	}
	var resp newAPIUserSelfResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, false, false, "newapi user/self decode failed"
	}
	if !resp.Success || resp.Data == nil {
		msg := strings.TrimSpace(resp.Message)
		if msg == "" {
			msg = "not ok"
		}
		return 0, 0, false, false, "newapi user/self: " + msg
	}
	if resp.Data.ID != 0 {
		want, convErr := strconv.ParseInt(userID, 10, 64)
		if convErr != nil || resp.Data.ID != want {
			return 0, 0, false, false, "newapi user/self id mismatch"
		}
	}
	unit := resolveNewAPIQuotaPerUnit(ctx, client, account, strings.TrimSpace(account.GetCredential("api_key")), origin)
	if unit <= 0 {
		unit = defaultNewAPIQuotaPerUnit
	}
	balanceUSD := resp.Data.Quota / unit
	if balanceUSD < 0 {
		balanceUSD = 0
	}
	usedUSD := resp.Data.UsedQuota / unit
	if usedUSD < 0 {
		usedUSD = 0
	}
	return balanceUSD, usedUSD, true, true, ""
}

// newAPITokenUsageResponse matches New API GET /api/usage/token JSON.
type newAPITokenUsageResponse struct {
	Code    any    `json:"code"` // true or 1
	Message string `json:"message"`
	Data    *struct {
		Object          string  `json:"object"`
		Name            string  `json:"name"`
		TotalGranted    float64 `json:"total_granted"`
		TotalUsed       float64 `json:"total_used"`
		TotalAvailable  float64 `json:"total_available"`
		UnlimitedQuota  bool    `json:"unlimited_quota"`
		ExpiresAt       int64   `json:"expires_at"`
	} `json:"data"`
}

func fetchNewAPITokenUsageBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (balance, used float64, hasUsed, unlimited, ok bool, errMsg string) {
	origin := originFromBaseURL(baseURL)
	if origin == "" {
		return 0, 0, false, false, false, "newapi: empty origin"
	}
	// Prefer no trailing slash first (token-bits returns 200); then with slash.
	urls := []string{
		origin + "/api/usage/token",
		origin + "/api/usage/token/",
	}
	var lastErr string
	for _, url := range urls {
		// New API always expects Authorization: Bearer <sk-...> regardless of Anthropic scheme.
		body, status, err := doBalanceGETBearer(ctx, client, apiKey, url)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		if status == http.StatusMovedPermanently || status == http.StatusFound || status == http.StatusTemporaryRedirect {
			lastErr = fmt.Sprintf("newapi usage/token redirect %d", status)
			continue
		}
		if status != http.StatusOK {
			lastErr = fmt.Sprintf("newapi usage/token status %d: %s", status, truncateForErr(body, 200))
			continue
		}
		if looksLikeHTML(body) {
			lastErr = "newapi usage/token returned HTML"
			continue
		}
		var resp newAPITokenUsageResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			lastErr = fmt.Sprintf("newapi usage/token decode: %v", err)
			continue
		}
		// Success envelope: code true/1 and data present.
		if !newAPICodeOK(resp.Code) || resp.Data == nil {
			lastErr = fmt.Sprintf("newapi usage/token not ok: %s", resp.Message)
			continue
		}
		// Prefer status-reported unit; fall back to defaultNewAPIQuotaPerUnit.
		unit := resolveNewAPIQuotaPerUnit(ctx, client, account, apiKey, origin)
		if unit <= 0 {
			unit = defaultNewAPIQuotaPerUnit
		}
		available := resp.Data.TotalAvailable
		unlimited = resp.Data.UnlimitedQuota
		// Convert internal quota units → USD. Clamp negative remaining to 0 for display.
		balanceUSD := available / unit
		if balanceUSD < 0 {
			balanceUSD = 0
		}
		usedUSD := resp.Data.TotalUsed / unit
		if usedUSD < 0 {
			usedUSD = 0
		}
		return balanceUSD, usedUSD, true, unlimited, true, ""
	}
	return 0, 0, false, false, false, lastErr
}

func newAPICodeOK(code any) bool {
	switch v := code.(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case json.Number:
		f, err := v.Float64()
		return err == nil && f != 0
	case string:
		return v == "true" || v == "1" || v == "ok"
	default:
		return false
	}
}

// resolveNewAPIQuotaPerUnit reads /api/status.data.quota_per_unit when available.
func resolveNewAPIQuotaPerUnit(ctx context.Context, client *http.Client, account *Account, apiKey, origin string) float64 {
	_ = account
	body, status, err := doBalanceGETBearer(ctx, client, apiKey, origin+"/api/status")
	if err != nil || status != http.StatusOK || looksLikeHTML(body) {
		return defaultNewAPIQuotaPerUnit
	}
	var resp struct {
		Data struct {
			QuotaPerUnit float64 `json:"quota_per_unit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return defaultNewAPIQuotaPerUnit
	}
	if resp.Data.QuotaPerUnit > 0 {
		return resp.Data.QuotaPerUnit
	}
	return defaultNewAPIQuotaPerUnit
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

func fetchCreditGrantsBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (balance, used float64, hasUsed, ok bool, errMsg string) {
	url := JoinOpenAIBillingURL(baseURL, "/v1/dashboard/billing/credit_grants")
	body, status, err := doBalanceGET(ctx, client, account, apiKey, url)
	if err != nil {
		return 0, 0, false, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, 0, false, false, fmt.Sprintf("credit_grants status %d: %s", status, truncateForErr(body, 200))
	}
	if looksLikeHTML(body) {
		return 0, 0, false, false, "credit_grants returned HTML (not JSON)"
	}
	var resp openAICreditGrantsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, 0, false, false, fmt.Sprintf("credit_grants decode: %v", err)
	}
	usedUSD := resp.TotalUsed
	hasUsedUSD := resp.TotalUsed > 0 || resp.TotalGranted > 0 || resp.TotalAvailable > 0
	// Prefer total_available; some gateways use total_remaining.
	if resp.TotalAvailable > 0 || resp.TotalGranted > 0 || resp.TotalUsed > 0 {
		return resp.TotalAvailable, usedUSD, hasUsedUSD, true, ""
	}
	if resp.TotalRemaining > 0 {
		return resp.TotalRemaining, usedUSD, hasUsedUSD, true, ""
	}
	// Zero available is still a valid reading when object decoded.
	return resp.TotalAvailable, usedUSD, hasUsedUSD, true, ""
}

func fetchSubscriptionUsageBalance(ctx context.Context, client *http.Client, account *Account, apiKey, baseURL string) (balance, used float64, hasUsed, ok bool, errMsg string) {
	subURL := JoinOpenAIBillingURL(baseURL, "/v1/dashboard/billing/subscription")
	body, status, err := doBalanceGET(ctx, client, account, apiKey, subURL)
	if err != nil {
		return 0, 0, false, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, 0, false, false, fmt.Sprintf("subscription status %d: %s", status, truncateForErr(body, 200))
	}
	var sub openAISubscriptionResponse
	if err := json.Unmarshal(body, &sub); err != nil {
		return 0, 0, false, false, fmt.Sprintf("subscription decode: %v", err)
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
		return 0, 0, false, false, err.Error()
	}
	if status != http.StatusOK {
		return 0, 0, false, false, fmt.Sprintf("usage status %d: %s", status, truncateForErr(body, 200))
	}
	var usage openAIUsageResponse
	if err := json.Unmarshal(body, &usage); err != nil {
		return 0, 0, false, false, fmt.Sprintf("usage decode: %v", err)
	}
	// TotalUsage is in cents (0.01 USD).
	used = usage.TotalUsage / 100
	if used < 0 {
		used = 0
	}
	balance = hardLimit - used
	return balance, used, true, true, ""
}

func doBalanceGET(ctx context.Context, client *http.Client, account *Account, apiKey, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	applyUpstreamBalanceAuth(req, account, apiKey)
	return doBalanceRequest(client, req)
}

func doBalanceGETBearer(ctx context.Context, client *http.Client, apiKey, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	return doBalanceRequest(client, req)
}

func doBalanceRequest(client *http.Client, req *http.Request) ([]byte, int, error) {
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
