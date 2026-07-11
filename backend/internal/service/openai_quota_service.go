package service

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/imroc/req/v3"
)

var ErrSparkShadowResetNotSupported = infraerrors.New(http.StatusConflict, "SPARK_SHADOW_RESET_NOT_SUPPORTED", "spark shadow account does not support credit reset; reset the parent account")

var openAIQuotaRedeemRequestIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func IsValidOpenAIQuotaRedeemRequestID(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) == 36 && openAIQuotaRedeemRequestIDPattern.MatchString(value)
}

// Endpoints used by the OpenAI/ChatGPT/Codex quota query and reset feature.
const (
	chatGPTUsageURL             = "https://chatgpt.com/backend-api/wham/usage"
	chatGPTRateLimitCreditsURL  = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"
	chatGPTRateLimitResetURL    = "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume"
	openaiQuotaUpstreamTimeout  = 20 * time.Second
	openaiQuotaCodexBeta        = "codex-1"
	openaiQuotaCodexLanguageTag = "zh-CN"
	openaiQuotaSecFetchSite     = "none"
	openaiQuotaSecFetchMode     = "no-cors"
	openaiQuotaSecFetchDest     = "empty"
)

// OpenAIRateLimitWindow describes a single rate-limit window returned by
// /wham/usage. The upstream returns an explicit `null` window when the slot
// is unused, so consumers should treat a nil pointer as "no data".
type OpenAIRateLimitWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  int64   `json:"reset_after_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

// OpenAIRateLimit is a rate-limit envelope (primary + optional secondary window).
type OpenAIRateLimit struct {
	Allowed         bool                   `json:"allowed"`
	LimitReached    bool                   `json:"limit_reached"`
	PrimaryWindow   *OpenAIRateLimitWindow `json:"primary_window,omitempty"`
	SecondaryWindow *OpenAIRateLimitWindow `json:"secondary_window,omitempty"`
}

// OpenAIAdditionalRateLimit describes a per-feature rate limit (e.g. Codex Spark).
type OpenAIAdditionalRateLimit struct {
	LimitName      string           `json:"limit_name"`
	MeteredFeature string           `json:"metered_feature"`
	RateLimit      *OpenAIRateLimit `json:"rate_limit,omitempty"`
}

// OpenAIRateLimitResetCreditDetail exposes only non-sensitive credit metadata.
type OpenAIRateLimitResetCreditDetail struct {
	ExpiresAt string `json:"expires_at,omitempty"`
}

// OpenAIRateLimitResetCredits captures the "available_count" surfaced for the
// rate_limit_reset_credit grant type, which the reset action consumes.
type OpenAIRateLimitResetCredits struct {
	AvailableCount int                                `json:"available_count"`
	Credits        []OpenAIRateLimitResetCreditDetail `json:"credits,omitempty"`
}

// OpenAIQuotaUsage is the typed projection of /wham/usage we expose to the UI.
// Fields not relevant to the quota card are intentionally omitted to keep the
// surface narrow; full upstream payload preservation is unnecessary.
type OpenAIQuotaUsage struct {
	UserID                string                       `json:"user_id,omitempty"`
	AccountID             string                       `json:"account_id,omitempty"`
	Email                 string                       `json:"email,omitempty"`
	PlanType              string                       `json:"plan_type,omitempty"`
	RateLimit             *OpenAIRateLimit             `json:"rate_limit,omitempty"`
	AdditionalRateLimits  []OpenAIAdditionalRateLimit  `json:"additional_rate_limits,omitempty"`
	RateLimitResetCredits *OpenAIRateLimitResetCredits `json:"rate_limit_reset_credits,omitempty"`
	FetchedAt             int64                        `json:"fetched_at"`
}

// OpenAIQuotaResetCredit captures the redeemed credit metadata returned by the
// reset endpoint.
type OpenAIQuotaResetCredit struct {
	ID              string `json:"id,omitempty"`
	ResetType       string `json:"reset_type,omitempty"`
	Status          string `json:"status,omitempty"`
	GrantedAt       string `json:"granted_at,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	RedeemStartedAt string `json:"redeem_started_at,omitempty"`
	RedeemedAt      string `json:"redeemed_at,omitempty"`
}

// OpenAIQuotaResetResult is the typed projection of /wham/rate-limit-reset-credits/consume.
// The inner Credit also carries `redeemed_at` (RFC3339 string); we deliberately do
// NOT add a top-level redeemed_at to avoid ambiguity with the nested field.
type OpenAIQuotaResetResult struct {
	Code         string                  `json:"code"`
	Credit       *OpenAIQuotaResetCredit `json:"credit,omitempty"`
	WindowsReset int                     `json:"windows_reset"`
}

// OpenAIQuotaService queries and consumes ChatGPT/Codex rate-limit reset credits
// for OpenAI OAuth accounts. It reuses the privacy client factory so all calls
// flow through the impersonated HTTP client (Cloudflare-friendly TLS fingerprint).
type OpenAIQuotaService struct {
	accountRepo          AccountRepository
	proxyRepo            ProxyRepository
	tokenProvider        *OpenAITokenProvider
	privacyClientFactory PrivacyClientFactory
}

// NewOpenAIQuotaService constructs a quota service. token provider is required —
// it ensures we always invoke upstream with a valid (refreshed-if-needed)
// access_token, sharing the same refresh/locking machinery used by the gateway.
func NewOpenAIQuotaService(
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	tokenProvider *OpenAITokenProvider,
	privacyClientFactory PrivacyClientFactory,
) *OpenAIQuotaService {
	return &OpenAIQuotaService{
		accountRepo:          accountRepo,
		proxyRepo:            proxyRepo,
		tokenProvider:        tokenProvider,
		privacyClientFactory: privacyClientFactory,
	}
}

// QueryUsage fetches the latest rate-limit/usage snapshot for the given OpenAI
// OAuth account. Returns infraerrors so the handler layer can map them to
// stable error codes / HTTP statuses.
func (s *OpenAIQuotaService) QueryUsage(ctx context.Context, accountID int64) (*OpenAIQuotaUsage, error) {
	accessToken, chatGPTAccountID, proxyURL, account, err := s.prepareUpstreamCall(ctx, accountID)
	if err != nil {
		return nil, err
	}

	client, err := s.privacyClientFactory(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_CLIENT_ERROR", "failed to build upstream client: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, openaiQuotaUpstreamTimeout)
	defer cancel()

	var payload OpenAIQuotaUsage
	resp, err := client.R().
		SetContext(callCtx).
		SetHeaders(buildCodexCommonHeaders(accessToken, chatGPTAccountID, account)).
		SetSuccessResult(&payload).
		Get(chatGPTUsageURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_REQUEST_FAILED", "upstream request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		status := resp.StatusCode
		body := truncate(resp.String(), 240)
		slog.Warn("openai_quota_query_failed", "account_id", accountID, "status", status, "body", body)
		return nil, infraerrors.Newf(mapUpstreamStatus(status), "OPENAI_QUOTA_UPSTREAM_ERROR", "upstream returned %d: %s", status, body)
	}

	payload.FetchedAt = time.Now().Unix()
	if payload.RateLimitResetCredits != nil && payload.RateLimitResetCredits.AvailableCount > 0 {
		payload.RateLimitResetCredits.Credits = s.queryResetCreditDetails(callCtx, client, accessToken, chatGPTAccountID, account, accountID)
	}
	return &payload, nil
}

func (s *OpenAIQuotaService) queryResetCreditDetails(ctx context.Context, client *req.Client, accessToken, chatGPTAccountID string, account *Account, accountID int64) []OpenAIRateLimitResetCreditDetail {
	resp, err := client.R().
		SetContext(ctx).
		SetHeaders(buildCodexCommonHeaders(accessToken, chatGPTAccountID, account)).
		Get(chatGPTRateLimitCreditsURL)
	if err != nil {
		slog.Warn("openai_quota_reset_credit_details_failed", "account_id", accountID, "error", err)
		return nil
	}
	if !resp.IsSuccessState() {
		slog.Warn("openai_quota_reset_credit_details_failed", "account_id", accountID, "status", resp.StatusCode)
		return nil
	}

	credits, err := parseOpenAIRateLimitResetCreditDetails(resp.Bytes())
	if err != nil {
		slog.Warn("openai_quota_reset_credit_details_parse_failed", "account_id", accountID, "error", err)
		return nil
	}
	return credits
}

// ResetCredit consumes one rate_limit_reset_credit for the given OpenAI account.
// The redeem_request_id is auto-generated (uuid-like) — upstream uses it for
// idempotency. Returns the consumed credit metadata so the UI can refresh.
func (s *OpenAIQuotaService) ResetCredit(ctx context.Context, accountID int64, redeemRequestID string) (*OpenAIQuotaResetResult, error) {
	if s != nil && s.accountRepo != nil {
		account, err := s.accountRepo.GetByID(ctx, accountID)
		if err != nil {
			return nil, infraerrors.Newf(http.StatusNotFound, "OPENAI_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
		}
		if account != nil && account.IsShadow() {
			return nil, ErrSparkShadowResetNotSupported
		}
	}
	if !IsValidOpenAIQuotaRedeemRequestID(redeemRequestID) {
		return nil, infraerrors.New(http.StatusBadRequest, "OPENAI_QUOTA_INVALID_REDEEM_REQUEST_ID", "a valid redeem_request_id is required")
	}
	accessToken, chatGPTAccountID, proxyURL, account, err := s.prepareUpstreamCall(ctx, accountID)
	if err != nil {
		return nil, err
	}

	client, err := s.privacyClientFactory(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_CLIENT_ERROR", "failed to build upstream client: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, openaiQuotaUpstreamTimeout)
	defer cancel()

	headers := buildCodexCommonHeaders(accessToken, chatGPTAccountID, account)
	headers["content-type"] = "application/json"

	var payload OpenAIQuotaResetResult
	resp, err := client.R().
		SetContext(callCtx).
		SetHeaders(headers).
		SetBody(map[string]string{"redeem_request_id": strings.TrimSpace(redeemRequestID)}).
		SetSuccessResult(&payload).
		Post(chatGPTRateLimitResetURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_RESET_REQUEST_FAILED", "upstream request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		status := resp.StatusCode
		body := truncate(resp.String(), 240)
		slog.Warn("openai_quota_reset_failed", "account_id", accountID, "status", status, "body", body)
		return nil, infraerrors.Newf(mapUpstreamStatus(status), "OPENAI_QUOTA_RESET_UPSTREAM_ERROR", "upstream returned %d: %s", status, body)
	}

	slog.Info("openai_quota_reset_success",
		"account_id", accountID,
		"code", payload.Code,
		"windows_reset", payload.WindowsReset,
	)
	return &payload, nil
}

// prepareUpstreamCall loads the account, validates it, obtains a fresh access
// token via the shared TokenProvider, and resolves the chatgpt-account-id and
// proxy URL. Centralized so QueryUsage / ResetCredit share validation.
func (s *OpenAIQuotaService) prepareUpstreamCall(ctx context.Context, accountID int64) (accessToken, chatGPTAccountID, proxyURL string, account *Account, err error) {
	if s == nil || s.accountRepo == nil || s.tokenProvider == nil || s.privacyClientFactory == nil {
		return "", "", "", nil, infraerrors.New(http.StatusInternalServerError, "OPENAI_QUOTA_NOT_CONFIGURED", "openai quota service is not configured")
	}

	requested, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return "", "", "", nil, infraerrors.Newf(http.StatusNotFound, "OPENAI_QUOTA_ACCOUNT_NOT_FOUND", "account not found: %v", err)
	}
	if requested == nil {
		return "", "", "", nil, infraerrors.New(http.StatusNotFound, "OPENAI_QUOTA_ACCOUNT_NOT_FOUND", "account not found")
	}
	if requested.Platform != PlatformOpenAI {
		return "", "", "", nil, infraerrors.New(http.StatusBadRequest, "OPENAI_QUOTA_INVALID_PLATFORM", "account is not an OpenAI account")
	}
	if requested.Type != AccountTypeOAuth {
		return "", "", "", nil, infraerrors.New(http.StatusBadRequest, "OPENAI_QUOTA_INVALID_TYPE", "account is not an OAuth account")
	}

	account, err = resolveCredentialAccount(ctx, s.accountRepo, requested)
	if err != nil {
		return "", "", "", nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_PARENT_UNAVAILABLE", "%v", err)
	}
	chatGPTAccountID = strings.TrimSpace(account.GetCredential("chatgpt_account_id"))
	if chatGPTAccountID == "" {
		// Fall back to organization_id — some legacy accounts only persisted poid.
		chatGPTAccountID = strings.TrimSpace(account.GetCredential("organization_id"))
	}
	if chatGPTAccountID == "" {
		return "", "", "", nil, infraerrors.New(http.StatusBadRequest, "OPENAI_QUOTA_MISSING_ACCOUNT_ID", "chatgpt_account_id is missing; please re-authorize this account")
	}

	accessToken, err = s.tokenProvider.GetAccessToken(ctx, account)
	if err != nil {
		return "", "", "", nil, infraerrors.Newf(http.StatusBadGateway, "OPENAI_QUOTA_TOKEN_UNAVAILABLE", "failed to acquire access token: %v", err)
	}
	if strings.TrimSpace(accessToken) == "" {
		return "", "", "", nil, infraerrors.New(http.StatusBadGateway, "OPENAI_QUOTA_TOKEN_UNAVAILABLE", "access token is empty")
	}

	// account.Proxy is eager-loaded by accountRepo.GetByID (see
	// repository.accountsToService), so we can read the proxy URL directly
	// instead of round-tripping the DB again. Fall back to proxyRepo only
	// when Proxy isn't pre-populated (defensive — e.g. callers that built
	// the Account by hand).
	if account.ProxyID != nil {
		switch {
		case account.Proxy != nil:
			proxyURL = account.Proxy.URL()
		case s.proxyRepo != nil:
			if proxy, perr := s.proxyRepo.GetByID(ctx, *account.ProxyID); perr == nil && proxy != nil {
				proxyURL = proxy.URL()
			}
		}
	}

	return accessToken, chatGPTAccountID, proxyURL, account, nil
}

// buildCodexCommonHeaders sets the request headers expected by the chatgpt.com
// backend so calls succeed past Cloudflare/WASM checks.
func buildCodexCommonHeaders(accessToken, chatGPTAccountID string, account *Account) map[string]string {
	userAgent := codexCLIUserAgent
	if account != nil {
		if custom := strings.TrimSpace(account.GetOpenAIUserAgent()); custom != "" {
			userAgent = custom
		}
	}
	headers := make(http.Header)
	headers.Set("authorization", "Bearer "+accessToken)
	headers.Set("chatgpt-account-id", chatGPTAccountID)
	headers.Set("openai-beta", openaiQuotaCodexBeta)
	headers.Set("oai-language", openaiQuotaCodexLanguageTag)
	headers.Set("originator", "codex_cli_rs")
	headers.Set("user-agent", userAgent)
	headers.Set("accept", "application/json")
	headers.Set("sec-fetch-site", openaiQuotaSecFetchSite)
	headers.Set("sec-fetch-mode", openaiQuotaSecFetchMode)
	headers.Set("sec-fetch-dest", openaiQuotaSecFetchDest)
	headers.Set("priority", "u=4, i")
	enforceCodexIdentityHeaders(headers)
	result := make(map[string]string, len(headers))
	for key, values := range headers {
		if len(values) > 0 {
			result[strings.ToLower(key)] = values[0]
		}
	}
	return result
}

type openAIRateLimitResetCreditDetailPayload struct {
	ExpiresAt      string `json:"expires_at,omitempty"`
	ExpiresAtCamel string `json:"expiresAt,omitempty"`
}

type openAIRateLimitResetCreditDetailsPayload struct {
	Credits               []openAIRateLimitResetCreditDetailPayload `json:"credits,omitempty"`
	RateLimitResetCredits []openAIRateLimitResetCreditDetailPayload `json:"rate_limit_reset_credits,omitempty"`
	Items                 []openAIRateLimitResetCreditDetailPayload `json:"items,omitempty"`
	Data                  []openAIRateLimitResetCreditDetailPayload `json:"data,omitempty"`
}

func parseOpenAIRateLimitResetCreditDetails(body []byte) ([]OpenAIRateLimitResetCreditDetail, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var rawCredits []openAIRateLimitResetCreditDetailPayload
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rawCredits); err != nil {
			return nil, err
		}
	} else {
		var payload openAIRateLimitResetCreditDetailsPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return nil, err
		}
		rawCredits = firstNonEmptyResetCreditPayload(
			payload.Credits,
			payload.RateLimitResetCredits,
			payload.Items,
			payload.Data,
		)
	}

	credits := make([]OpenAIRateLimitResetCreditDetail, 0, len(rawCredits))
	for _, raw := range rawCredits {
		expiresAt := strings.TrimSpace(raw.ExpiresAt)
		if expiresAt == "" {
			expiresAt = strings.TrimSpace(raw.ExpiresAtCamel)
		}
		if expiresAt != "" {
			credits = append(credits, OpenAIRateLimitResetCreditDetail{ExpiresAt: expiresAt})
		}
	}
	return credits, nil
}

func firstNonEmptyResetCreditPayload(lists ...[]openAIRateLimitResetCreditDetailPayload) []openAIRateLimitResetCreditDetailPayload {
	for _, list := range lists {
		if len(list) > 0 {
			return list
		}
	}
	return nil
}

// mapUpstreamStatus collapses upstream HTTP statuses into a stable set we
// surface from the admin handler. 4xx upstream errors are surfaced as 502
// (BadGateway) so callers can distinguish "your input is bad" (400) from
// "upstream said no" (502); 401/403 are bubbled directly to hint at re-auth.
func mapUpstreamStatus(status int) int {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return status
	case status == http.StatusTooManyRequests:
		return http.StatusTooManyRequests
	case status >= 400 && status < 500:
		return http.StatusBadGateway
	case status >= 500:
		return http.StatusBadGateway
	default:
		return http.StatusBadGateway
	}
}
