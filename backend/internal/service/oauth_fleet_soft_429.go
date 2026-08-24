package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

const oauthFleetSoft429RedisKeyPrefix = "oauth-soft-429:"

var oauthFleetSoft429DefaultPlatforms = []string{
	PlatformAnthropic,
	PlatformOpenAI,
	PlatformGemini,
	PlatformAntigravity,
	PlatformGrok,
}

var oauthFleetSoft429AllowedPlatforms = map[string]struct{}{
	PlatformAnthropic:   {},
	PlatformOpenAI:      {},
	PlatformGemini:      {},
	PlatformAntigravity: {},
	PlatformGrok:        {},
}

var oauthFleetSoft429DefaultHardBodyCodes = []string{
	"insufficient_quota",
	"usage_limit_exceeded",
	"usage_limit_reached",
	"api_key_quota_exhausted",
}

// Built-in quota-death body codes cannot be softened by the admin code table.
var oauthFleetSoft429BuiltinHardBodyCodes = map[string]struct{}{
	"insufficient_quota":      {},
	"usage_limit_exceeded":    {},
	"usage_limit_reached":     {},
	"api_key_quota_exhausted": {},
}

type oauth429Class int

const (
	oauth429NotApplicable oauth429Class = iota
	oauth429Soft
	oauth429Hard
)

func (c oauth429Class) String() string {
	switch c {
	case oauth429Soft:
		return "soft"
	case oauth429Hard:
		return "hard"
	default:
		return "not_applicable"
	}
}

// OAuthFleetSoft429Cache is the layer-2 short Redis exclude (scheduler filter only).
type OAuthFleetSoft429Cache interface {
	SetSoftExclude(ctx context.Context, accountID int64, ttl time.Duration) error
	ListSoftExcluded(ctx context.Context) ([]int64, error)
}

func OAuthFleetSoft429RedisKey(accountID int64) string {
	return oauthFleetSoft429RedisKeyPrefix + strconv.FormatInt(accountID, 10)
}

func (s *RateLimitService) SetOAuthFleetSoft429Cache(cache OAuthFleetSoft429Cache) {
	if s == nil {
		return
	}
	s.oauthFleetSoft429Cache = cache
}

func (s *RateLimitService) loadOAuthFleetSoft429Settings(ctx context.Context) *OAuthFleetSoft429Settings {
	if s == nil || s.settingService == nil {
		return DefaultOAuthFleetSoft429Settings()
	}
	settings, err := s.settingService.GetOAuthFleetSoft429Settings(ctx)
	if err != nil || settings == nil {
		return DefaultOAuthFleetSoft429Settings()
	}
	return settings
}

// oauthFleetSoft429Applies follows design §5.6:
// account extra false → off; extra true → canary on; else global enabled + scope.
func oauthFleetSoft429Applies(account *Account, settings *OAuthFleetSoft429Settings) bool {
	if account == nil || !account.IsOAuth() {
		return false
	}
	if settings == nil {
		settings = DefaultOAuthFleetSoft429Settings()
	}
	if override := boolOverrideFromMap(account.Extra, AccountExtraOAuthFleetSoft429); override != nil {
		return *override
	}
	if !settings.Enabled {
		return false
	}
	if account.Type == AccountTypeSetupToken && !settings.IncludeSetupToken {
		return false
	}
	if settings.Scope == OAuthFleetSoft429ScopeOptIn && !oauthFleetSoft429PlatformAllowed(account.Platform, settings.Platforms) {
		return false
	}
	return true
}

func oauthFleetSoft429PlatformAllowed(platform string, platforms []string) bool {
	platform = strings.ToLower(strings.TrimSpace(platform))
	for _, p := range platforms {
		if strings.ToLower(strings.TrimSpace(p)) == platform {
			return true
		}
	}
	return false
}

func classifyOAuth429(account *Account, settings *OAuthFleetSoft429Settings, status int, headers http.Header, body []byte) (oauth429Class, string) {
	if status != http.StatusTooManyRequests {
		return oauth429NotApplicable, "not_429"
	}
	if settings == nil {
		settings = DefaultOAuthFleetSoft429Settings()
	}

	if account != nil && account.Platform == PlatformAnthropic && selectAnthropicExhaustedWindow(headers, time.Now()) != nil {
		return oauth429Hard, "anthropic_window_exhausted"
	}
	if isOAuth429CodexWindowExhausted(headers) {
		return oauth429Hard, "codex_window_100"
	}
	if isOAuth429BuiltinQuotaDeath(body) {
		return oauth429Hard, "quota_death"
	}

	codes := extractOAuth429BodyCodes(body)
	if hitsOAuth429CodeSet(codes, settings.HardBodyCodes) {
		return oauth429Hard, "hard_body_code"
	}

	dur, hasReset := parseOAuth429ResetDuration(account, headers, body)
	switch strings.ToLower(strings.TrimSpace(settings.LongResetPolicy)) {
	case OAuthFleetSoft429LongResetHard:
		if hasReset {
			return oauth429Hard, "long_reset_policy_hard"
		}
		return oauth429Soft, "no_reliable_reset"
	case OAuthFleetSoft429LongResetThreshold:
		threshold := settings.LongResetThresholdSeconds
		if threshold < oauthFleetSoft429MinThresholdSeconds || threshold > oauthFleetSoft429MaxThresholdSeconds {
			threshold = oauthFleetSoft429DefaultThresholdSeconds
		}
		if hasReset && dur >= time.Duration(threshold)*time.Second {
			return oauth429Hard, "long_reset_threshold"
		}
		return oauth429Soft, "below_reset_threshold"
	default:
		return oauth429Soft, "long_reset_policy_soft"
	}
}

func isOAuth429CodexWindowExhausted(headers http.Header) bool {
	snapshot := ParseCodexRateLimitHeaders(headers)
	if snapshot == nil {
		return false
	}
	normalized := snapshot.Normalize()
	if normalized == nil {
		return false
	}
	is7d := normalized.Used7dPercent != nil && *normalized.Used7dPercent >= 100
	is5h := normalized.Used5hPercent != nil && *normalized.Used5hPercent >= 100
	if is7d && normalized.Reset7dSeconds != nil {
		return true
	}
	if is5h && normalized.Reset5hSeconds != nil {
		return true
	}
	return false
}

func isOAuth429BuiltinQuotaDeath(body []byte) bool {
	for _, code := range extractOAuth429BodyCodes(body) {
		if _, ok := oauthFleetSoft429BuiltinHardBodyCodes[code]; ok {
			return true
		}
	}
	msg := strings.ToLower(extractUpstreamErrorMessage(body))
	if strings.Contains(msg, "额度已用完") {
		return true
	}
	return false
}

func extractOAuth429BodyCodes(body []byte) []string {
	seen := map[string]struct{}{}
	add := func(raw string) {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code == "" {
			return
		}
		seen[code] = struct{}{}
	}
	add(strings.TrimSpace(gjson.GetBytes(body, "error.type").String()))
	add(strings.TrimSpace(gjson.GetBytes(body, "error.code").String()))
	add(strings.TrimSpace(gjson.GetBytes(body, "code").String()))
	add(extractUpstreamErrorCode(body))
	out := make([]string, 0, len(seen))
	for code := range seen {
		out = append(out, code)
	}
	return out
}

func hitsOAuth429CodeSet(codes, table []string) bool {
	if len(codes) == 0 || len(table) == 0 {
		return false
	}
	allowed := map[string]struct{}{}
	for _, raw := range table {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code != "" {
			allowed[code] = struct{}{}
		}
	}
	for _, code := range codes {
		if _, ok := allowed[code]; ok {
			return true
		}
	}
	return false
}

func parseOAuth429ResetDuration(account *Account, headers http.Header, body []byte) (time.Duration, bool) {
	now := time.Now()
	if account != nil && account.Platform == PlatformOpenAI {
		if resetAt := calculateOpenAI429ResetTime(headers); resetAt != nil && resetAt.After(now) {
			return resetAt.Sub(now), true
		}
	}
	if result := calculateAnthropic429ResetTime(headers); result != nil && result.resetAt.After(now) {
		return result.resetAt.Sub(now), true
	}
	if dur, ok := parseRetryAfterDuration(headers); ok {
		return dur, true
	}
	if resetAt := parseOpenAIRateLimitResetTime(body); resetAt != nil {
		t := time.Unix(*resetAt, 0)
		if t.After(now) {
			return t.Sub(now), true
		}
	}
	if resetAt := ParseGeminiRateLimitResetTime(body); resetAt != nil {
		t := time.Unix(*resetAt, 0)
		if t.After(now) {
			return t.Sub(now), true
		}
	}
	return 0, false
}

func parseRetryAfterDuration(headers http.Header) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	raw := strings.TrimSpace(headers.Get("Retry-After"))
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(raw); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
	}
	return 0, false
}

func (s *RateLimitService) resolveOAuthFleetAccount(ctx context.Context, account *Account) *Account {
	if account == nil || !account.IsShadow() || s == nil || s.accountRepo == nil {
		return account
	}
	parent, err := resolveCredentialAccount(ctx, s.accountRepo, account)
	if err != nil || parent == nil {
		return account
	}
	return parent
}

// TryHandleOAuthFleetSoft429 classifies an OAuth 429 and, when soft, writes Redis
// and returns true so callers skip SetRateLimited / SetTempUnschedulable.
func (s *RateLimitService) TryHandleOAuthFleetSoft429(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) bool {
	if s == nil || statusCode != http.StatusTooManyRequests || account == nil {
		return false
	}
	target := s.resolveOAuthFleetAccount(ctx, account)
	settings := s.loadOAuthFleetSoft429Settings(ctx)
	if !oauthFleetSoft429Applies(target, settings) {
		return false
	}
	class, reason := classifyOAuth429(target, settings, statusCode, headers, responseBody)
	if class != oauth429Soft {
		return false
	}
	s.persistOAuthFleetSoft429(ctx, target, settings, reason, account)
	return true
}

func (s *RateLimitService) persistOAuthFleetSoft429(ctx context.Context, account *Account, settings *OAuthFleetSoft429Settings, reason string, original *Account) {
	if settings == nil {
		settings = DefaultOAuthFleetSoft429Settings()
	}
	ttl := time.Duration(settings.TTLSeconds) * time.Second
	if settings.TTLSeconds < oauthFleetSoft429MinTTLSeconds || settings.TTLSeconds > oauthFleetSoft429MaxTTLSeconds {
		ttl = time.Duration(oauthFleetSoft429DefaultTTLSeconds) * time.Second
	}
	override := "unset"
	if account != nil {
		if v := boolOverrideFromMap(account.Extra, AccountExtraOAuthFleetSoft429); v != nil {
			if *v {
				override = "true"
			} else {
				override = "false"
			}
		}
	}
	if s.oauthFleetSoft429Cache != nil && account != nil {
		if err := s.oauthFleetSoft429Cache.SetSoftExclude(ctx, account.ID, ttl); err != nil {
			slog.Warn("oauth_fleet_soft_429_redis_set_failed",
				"account_id", account.ID,
				"error", err)
		}
	}
	logID := int64(0)
	platform := ""
	if account != nil {
		logID = account.ID
		platform = account.Platform
	}
	origID := logID
	if original != nil {
		origID = original.ID
	}
	slog.Info("oauth_fleet_soft_429",
		"account_id", logID,
		"original_account_id", origID,
		"platform", platform,
		"reason", "soft",
		"hard_reason", reason,
		"applies", true,
		"override", override,
		"ttl_seconds", int(ttl/time.Second))
}

func (s *RateLimitService) logOAuthFleetHard429(account *Account, reason, override string) {
	if account == nil {
		return
	}
	slog.Info("oauth_fleet_soft_429",
		"account_id", account.ID,
		"platform", account.Platform,
		"reason", "hard",
		"hard_reason", reason,
		"applies", true,
		"override", override)
}

// MergeOAuthFleetSoft429Exclusions unions live Redis soft-exclude IDs into excluded.
// Hard affinity (existing sticky binding / previous_response) must pass skip=true.
// A generated sessionHash alone is not affinity.
func (s *RateLimitService) MergeOAuthFleetSoft429Exclusions(ctx context.Context, excluded map[int64]struct{}, skipHardAffinity bool) map[int64]struct{} {
	if s == nil || skipHardAffinity || s.oauthFleetSoft429Cache == nil {
		return excluded
	}
	ids, err := s.oauthFleetSoft429Cache.ListSoftExcluded(ctx)
	if err != nil || len(ids) == 0 {
		return excluded
	}
	out := excluded
	if out == nil {
		out = make(map[int64]struct{}, len(ids))
	} else {
		cloned := make(map[int64]struct{}, len(out)+len(ids))
		for id := range out {
			cloned[id] = struct{}{}
		}
		out = cloned
	}
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func mergeOAuthFleetSoft429ExcludedIDs(ctx context.Context, rateLimit *RateLimitService, excluded map[int64]struct{}, hasHardAffinity bool) map[int64]struct{} {
	if rateLimit == nil {
		return excluded
	}
	return rateLimit.MergeOAuthFleetSoft429Exclusions(ctx, excluded, hasHardAffinity)
}

// oauthFleetSoft429HasHardAffinity reports whether layer-2 Redis exclude must be
// skipped. A non-empty sessionHash is NOT affinity: almost every request
// generates one from the body. Skip only for an existing sticky binding or
// previous_response_id.
func oauthFleetSoft429HasHardAffinity(previousResponseID string, stickyAccountID int64) bool {
	return strings.TrimSpace(previousResponseID) != "" || stickyAccountID > 0
}

func oauthFleetSoft429StickyAccountID(ctx context.Context, cache GatewayCache, groupID *int64, sessionHash string) int64 {
	if cache == nil || strings.TrimSpace(sessionHash) == "" {
		return 0
	}
	id, err := cache.GetSessionAccountID(ctx, derefGroupID(groupID), sessionHash)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func normalizeOAuthFleetSoft429SettingsForGet(in *OAuthFleetSoft429Settings) *OAuthFleetSoft429Settings {
	if in == nil {
		return DefaultOAuthFleetSoft429Settings()
	}
	out := *in
	if out.TTLSeconds < oauthFleetSoft429MinTTLSeconds || out.TTLSeconds > oauthFleetSoft429MaxTTLSeconds {
		if !out.Enabled {
			out.TTLSeconds = oauthFleetSoft429DefaultTTLSeconds
		} else if out.TTLSeconds < oauthFleetSoft429MinTTLSeconds {
			out.TTLSeconds = oauthFleetSoft429MinTTLSeconds
		} else {
			out.TTLSeconds = oauthFleetSoft429MaxTTLSeconds
		}
	}
	switch strings.ToLower(strings.TrimSpace(out.LongResetPolicy)) {
	case OAuthFleetSoft429LongResetHard, OAuthFleetSoft429LongResetThreshold:
		out.LongResetPolicy = strings.ToLower(strings.TrimSpace(out.LongResetPolicy))
	default:
		out.LongResetPolicy = OAuthFleetSoft429LongResetSoft
	}
	if out.LongResetThresholdSeconds < oauthFleetSoft429MinThresholdSeconds || out.LongResetThresholdSeconds > oauthFleetSoft429MaxThresholdSeconds {
		out.LongResetThresholdSeconds = oauthFleetSoft429DefaultThresholdSeconds
	}
	if strings.ToLower(strings.TrimSpace(out.Scope)) == OAuthFleetSoft429ScopeOptIn {
		out.Scope = OAuthFleetSoft429ScopeOptIn
	} else {
		out.Scope = OAuthFleetSoft429ScopeAllOAuth
	}
	out.Platforms = sanitizeOAuthFleetSoft429Platforms(out.Platforms, false)
	if len(out.Platforms) == 0 {
		out.Platforms = append([]string(nil), oauthFleetSoft429DefaultPlatforms...)
	}
	out.SoftStatusCodes = sanitizeOAuthFleetSoft429StatusCodes(out.SoftStatusCodes)
	out.SoftBodyCodes = sanitizeOAuthFleetSoft429BodyCodes(out.SoftBodyCodes)
	out.HardBodyCodes = sanitizeOAuthFleetSoft429BodyCodes(out.HardBodyCodes)
	return &out
}

func normalizeOAuthFleetSoft429SettingsForSet(in *OAuthFleetSoft429Settings) (*OAuthFleetSoft429Settings, error) {
	if in == nil {
		return nil, fmt.Errorf("settings cannot be nil")
	}
	out := *in
	if out.TTLSeconds < oauthFleetSoft429MinTTLSeconds || out.TTLSeconds > oauthFleetSoft429MaxTTLSeconds {
		if out.Enabled {
			return nil, fmt.Errorf("ttl_seconds must be between 5-300")
		}
		out.TTLSeconds = oauthFleetSoft429DefaultTTLSeconds
	}
	policy := strings.ToLower(strings.TrimSpace(out.LongResetPolicy))
	switch policy {
	case OAuthFleetSoft429LongResetSoft, OAuthFleetSoft429LongResetHard, OAuthFleetSoft429LongResetThreshold:
		out.LongResetPolicy = policy
	default:
		return nil, fmt.Errorf("long_reset_policy must be soft, hard, or threshold")
	}
	if policy == OAuthFleetSoft429LongResetThreshold {
		if out.LongResetThresholdSeconds < oauthFleetSoft429MinThresholdSeconds || out.LongResetThresholdSeconds > oauthFleetSoft429MaxThresholdSeconds {
			if out.Enabled {
				return nil, fmt.Errorf("long_reset_threshold_seconds must be between 5-86400")
			}
			out.LongResetThresholdSeconds = oauthFleetSoft429DefaultThresholdSeconds
		}
	} else if out.LongResetThresholdSeconds <= 0 {
		out.LongResetThresholdSeconds = oauthFleetSoft429DefaultThresholdSeconds
	}
	scope := strings.ToLower(strings.TrimSpace(out.Scope))
	switch scope {
	case OAuthFleetSoft429ScopeAllOAuth, OAuthFleetSoft429ScopeOptIn:
		out.Scope = scope
	default:
		return nil, fmt.Errorf("scope must be all_oauth or opt_in")
	}
	platforms, err := validateOAuthFleetSoft429Platforms(out.Platforms, scope == OAuthFleetSoft429ScopeOptIn)
	if err != nil {
		return nil, err
	}
	out.Platforms = platforms
	statusCodes, err := validateOAuthFleetSoft429StatusCodes(out.SoftStatusCodes)
	if err != nil {
		return nil, err
	}
	out.SoftStatusCodes = statusCodes
	softCodes, err := validateOAuthFleetSoft429BodyCodes(out.SoftBodyCodes, "soft_body_codes")
	if err != nil {
		return nil, err
	}
	hardCodes, err := validateOAuthFleetSoft429BodyCodes(out.HardBodyCodes, "hard_body_codes")
	if err != nil {
		return nil, err
	}
	out.SoftBodyCodes = softCodes
	out.HardBodyCodes = hardCodes
	return &out, nil
}

func sanitizeOAuthFleetSoft429Platforms(in []string, _ bool) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		p := strings.ToLower(strings.TrimSpace(raw))
		if _, ok := oauthFleetSoft429AllowedPlatforms[p]; !ok {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func validateOAuthFleetSoft429Platforms(in []string, requireOne bool) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		p := strings.ToLower(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if _, ok := oauthFleetSoft429AllowedPlatforms[p]; !ok {
			return nil, fmt.Errorf("platforms contains unknown platform %q", raw)
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if requireOne && len(out) == 0 {
		return nil, fmt.Errorf("platforms must contain at least 1 platform when scope is opt_in")
	}
	if len(out) == 0 {
		out = append([]string(nil), oauthFleetSoft429DefaultPlatforms...)
	}
	return out, nil
}

func sanitizeOAuthFleetSoft429StatusCodes(in []int) []int {
	for _, code := range in {
		if code == http.StatusTooManyRequests {
			return []int{http.StatusTooManyRequests}
		}
	}
	return []int{http.StatusTooManyRequests}
}

func validateOAuthFleetSoft429StatusCodes(in []int) ([]int, error) {
	if len(in) == 0 {
		return []int{http.StatusTooManyRequests}, nil
	}
	out := make([]int, 0, 1)
	seen := map[int]struct{}{}
	for _, code := range in {
		if code != http.StatusTooManyRequests {
			return nil, fmt.Errorf("soft_status_codes may only contain 429")
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	if len(out) == 0 {
		out = []int{http.StatusTooManyRequests}
	}
	return out, nil
}

func sanitizeOAuthFleetSoft429BodyCodes(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		if len(out) >= oauthFleetSoft429MaxBodyCodes {
			break
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

func validateOAuthFleetSoft429BodyCodes(in []string, field string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		code := strings.ToLower(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		if len(out) >= oauthFleetSoft429MaxBodyCodes {
			return nil, fmt.Errorf("%s may contain at most %d entries", field, oauthFleetSoft429MaxBodyCodes)
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out, nil
}
