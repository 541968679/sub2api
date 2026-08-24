//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type oauthFleetSoft429CacheStub struct {
	sets    []oauthFleetSoft429Set
	listed  []int64
	listErr error
	setErr  error
}

type oauthFleetSoft429Set struct {
	id  int64
	ttl time.Duration
}

func (c *oauthFleetSoft429CacheStub) SetSoftExclude(_ context.Context, accountID int64, ttl time.Duration) error {
	if c.setErr != nil {
		return c.setErr
	}
	c.sets = append(c.sets, oauthFleetSoft429Set{id: accountID, ttl: ttl})
	return nil
}

func (c *oauthFleetSoft429CacheStub) ListSoftExcluded(_ context.Context) ([]int64, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return append([]int64(nil), c.listed...), nil
}

type oauthFleetSoft429RepoStub struct {
	mockAccountRepoForGemini
	rateLimitCalls   int
	tempUnschedCalls int
	lastRateLimitAt  time.Time
}

func (r *oauthFleetSoft429RepoStub) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	r.rateLimitCalls++
	r.lastRateLimitAt = resetAt
	return nil
}

func (r *oauthFleetSoft429RepoStub) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.tempUnschedCalls++
	return nil
}

func oauthFleetEnabledSettings() *OAuthFleetSoft429Settings {
	s := DefaultOAuthFleetSoft429Settings()
	s.Enabled = true
	return s
}

func oauthFleetAccount(id int64, platform, typ string, extra map[string]any) *Account {
	return &Account{
		ID:          id,
		Platform:    platform,
		Type:        typ,
		Status:      StatusActive,
		Schedulable: true,
		Extra:       extra,
	}
}

func persistOAuthFleetSettings(t *testing.T, settings *OAuthFleetSoft429Settings) *SettingService {
	t.Helper()
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})
	require.NoError(t, svc.SetOAuthFleetSoft429Settings(context.Background(), settings))
	return svc
}

func newOAuthFleetRateLimit(t *testing.T, settings *OAuthFleetSoft429Settings, cache OAuthFleetSoft429Cache) (*RateLimitService, *oauthFleetSoft429RepoStub) {
	t.Helper()
	repo := &oauthFleetSoft429RepoStub{}
	cfg := &config.Config{}
	cfg.RateLimit.OAuth401CooldownMinutes = 10
	svc := NewRateLimitService(repo, nil, cfg, nil, nil)
	if settings != nil {
		svc.SetSettingService(persistOAuthFleetSettings(t, settings))
	}
	svc.SetOAuthFleetSoft429Cache(cache)
	return svc, repo
}

func TestDefaultOAuthFleetSoft429Settings_EnabledFalseUnlike529(t *testing.T) {
	d := DefaultOAuthFleetSoft429Settings()
	require.False(t, d.Enabled)
	require.Equal(t, 20, d.TTLSeconds)
	require.Equal(t, OAuthFleetSoft429LongResetSoft, d.LongResetPolicy)
	require.Equal(t, OAuthFleetSoft429ScopeAllOAuth, d.Scope)
	require.True(t, d.IncludeSetupToken)
	require.Equal(t, []int{429}, d.SoftStatusCodes)
	require.Contains(t, d.SoftBodyCodes, "rate_limit_exceeded")
	require.NotEqual(t, DefaultOverloadCooldownSettings().Enabled, d.Enabled)
}

func TestGetOAuthFleetSoft429Settings_EmptyAndBadJSON_DefaultOff(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetOAuthFleetSoft429Settings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)

	repo.data[SettingKeyOAuthFleetSoft429Settings] = ""
	settings, err = svc.GetOAuthFleetSoft429Settings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)

	repo.data[SettingKeyOAuthFleetSoft429Settings] = "not-json"
	settings, err = svc.GetOAuthFleetSoft429Settings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 20, settings.TTLSeconds)
}

func TestGetOAuthFleetSoft429Settings_ReadsFromDB(t *testing.T) {
	repo := newMockSettingRepo()
	in := DefaultOAuthFleetSoft429Settings()
	in.Enabled = true
	in.TTLSeconds = 45
	data, err := json.Marshal(in)
	require.NoError(t, err)
	repo.data[SettingKeyOAuthFleetSoft429Settings] = string(data)
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetOAuthFleetSoft429Settings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, 45, settings.TTLSeconds)
}

func TestSetOAuthFleetSoft429Settings_Validation(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	enabled := DefaultOAuthFleetSoft429Settings()
	enabled.Enabled = true
	enabled.TTLSeconds = 4
	require.Error(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled))
	require.Contains(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled).Error(), "ttl_seconds")

	enabled.TTLSeconds = 301
	require.Error(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled))

	enabled.TTLSeconds = 20
	enabled.SoftStatusCodes = []int{529}
	require.Error(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled))
	require.Contains(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled).Error(), "429")

	enabled.SoftStatusCodes = []int{429}
	enabled.Platforms = []string{"not-a-platform"}
	require.Error(t, svc.SetOAuthFleetSoft429Settings(context.Background(), enabled))

	disabled := DefaultOAuthFleetSoft429Settings()
	disabled.Enabled = false
	disabled.TTLSeconds = 0
	require.NoError(t, svc.SetOAuthFleetSoft429Settings(context.Background(), disabled))
	got, err := svc.GetOAuthFleetSoft429Settings(context.Background())
	require.NoError(t, err)
	require.False(t, got.Enabled)
	require.Equal(t, 20, got.TTLSeconds)
}

func TestOAuthFleetSoft429Settings_NotExposedOnPublicSettings(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyOAuthFleetSoft429Settings: `{"enabled":true,"ttl_seconds":20}`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	raw, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "oauth_fleet")
	require.NotContains(t, string(raw), SettingKeyOAuthFleetSoft429Settings)
}

func TestOAuthFleetSoft429Applies_ResolutionOrder(t *testing.T) {
	off := DefaultOAuthFleetSoft429Settings()
	on := oauthFleetEnabledSettings()
	on.Scope = OAuthFleetSoft429ScopeOptIn
	on.Platforms = []string{PlatformAnthropic}

	oauth := oauthFleetAccount(1, PlatformOpenAI, AccountTypeOAuth, nil)
	require.False(t, oauthFleetSoft429Applies(nil, on))
	require.False(t, oauthFleetSoft429Applies(&Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}, on))
	require.False(t, oauthFleetSoft429Applies(oauth, off))

	oauth.Extra = map[string]any{AccountExtraOAuthFleetSoft429: false}
	require.False(t, oauthFleetSoft429Applies(oauth, on))

	oauth.Extra = map[string]any{AccountExtraOAuthFleetSoft429: true}
	require.True(t, oauthFleetSoft429Applies(oauth, off), "canary extra=true must apply even when global is off")
	require.True(t, oauthFleetSoft429Applies(oauth, on), "canary ignores opt_in platform miss")

	oauth.Extra = nil
	require.False(t, oauthFleetSoft429Applies(oauth, on), "opt_in + openai not listed")
	oauth.Platform = PlatformAnthropic
	require.True(t, oauthFleetSoft429Applies(oauth, on))

	setup := oauthFleetAccount(2, PlatformAnthropic, AccountTypeSetupToken, nil)
	on.IncludeSetupToken = false
	require.False(t, oauthFleetSoft429Applies(setup, on))
	setup.Extra = map[string]any{AccountExtraOAuthFleetSoft429: true}
	require.True(t, oauthFleetSoft429Applies(setup, on))
}

func TestClassifyOAuth429_SoftHardAndLongReset(t *testing.T) {
	settings := oauthFleetEnabledSettings()
	account := oauthFleetAccount(1, PlatformOpenAI, AccountTypeOAuth, nil)

	class, _ := classifyOAuth429(account, settings, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, oauth429Soft, class)

	class, _ = classifyOAuth429(account, settings, 429, http.Header{}, nil)
	require.Equal(t, oauth429Soft, class)

	class, _ = classifyOAuth429(account, settings, 429, http.Header{}, []byte(`{"error":{"type":"usage_limit_reached"}}`))
	require.Equal(t, oauth429Hard, class)

	softTable := oauthFleetEnabledSettings()
	softTable.SoftBodyCodes = []string{"usage_limit_reached", "rate_limit_exceeded"}
	class, _ = classifyOAuth429(account, softTable, 429, http.Header{}, []byte(`{"error":{"type":"usage_limit_reached"}}`))
	require.Equal(t, oauth429Hard, class, "builtin quota death cannot be softened")

	longHeaders := http.Header{}
	longHeaders.Set("x-codex-primary-used-percent", "40")
	longHeaders.Set("x-codex-primary-reset-after-seconds", "400000")
	longHeaders.Set("x-codex-primary-window-minutes", "10080")
	longHeaders.Set("x-codex-secondary-used-percent", "10")
	longHeaders.Set("x-codex-secondary-reset-after-seconds", "10000")
	longHeaders.Set("x-codex-secondary-window-minutes", "300")

	class, _ = classifyOAuth429(account, settings, 429, longHeaders, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, oauth429Soft, class)

	hardPolicy := oauthFleetEnabledSettings()
	hardPolicy.LongResetPolicy = OAuthFleetSoft429LongResetHard
	class, _ = classifyOAuth429(account, hardPolicy, 429, longHeaders, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, oauth429Hard, class)

	threshold := oauthFleetEnabledSettings()
	threshold.LongResetPolicy = OAuthFleetSoft429LongResetThreshold
	threshold.LongResetThresholdSeconds = 60
	class, _ = classifyOAuth429(account, threshold, 429, longHeaders, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, oauth429Hard, class)

	exhausted := http.Header{}
	exhausted.Set("x-codex-primary-used-percent", "100")
	exhausted.Set("x-codex-primary-reset-after-seconds", "384607")
	exhausted.Set("x-codex-primary-window-minutes", "10080")
	exhausted.Set("x-codex-secondary-used-percent", "3")
	exhausted.Set("x-codex-secondary-reset-after-seconds", "17369")
	exhausted.Set("x-codex-secondary-window-minutes", "300")
	softTable.SoftBodyCodes = []string{"rate_limit_exceeded"}
	class, _ = classifyOAuth429(account, softTable, 429, exhausted, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, oauth429Hard, class)
}

func TestHandleUpstreamError_OAuthFleetSoft429_NoDBAndRedis(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	settings := oauthFleetEnabledSettings()
	settings.TTLSeconds = 25
	svc, repo := newOAuthFleetRateLimit(t, settings, cache)
	account := oauthFleetAccount(9, PlatformOpenAI, AccountTypeOAuth, nil)
	beforeSched := account.IsSchedulable()

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached"}}`))
	require.False(t, shouldDisable)
	require.Zero(t, repo.rateLimitCalls)
	require.Zero(t, repo.tempUnschedCalls)
	require.Len(t, cache.sets, 1)
	require.Equal(t, int64(9), cache.sets[0].id)
	require.Equal(t, 25*time.Second, cache.sets[0].ttl)
	require.Equal(t, beforeSched, account.IsSchedulable())
	require.True(t, account.IsSchedulable())
}

func TestHandleUpstreamError_OAuthFleetSoft429_CanaryWhenGlobalOff(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, DefaultOAuthFleetSoft429Settings(), cache)
	account := oauthFleetAccount(3, PlatformOpenAI, AccountTypeOAuth, map[string]any{AccountExtraOAuthFleetSoft429: true})

	svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Zero(t, repo.rateLimitCalls)
	require.Len(t, cache.sets, 1)
}

func TestHandleUpstreamError_OAuthFleetSoft429_EmptyKVUsesHandle429(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, nil, cache)
	account := oauthFleetAccount(4, PlatformOpenAI, AccountTypeOAuth, nil)

	svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Empty(t, cache.sets)
}

func TestHandleUpstreamError_OAuthFleetSoft429_ExtraFalseUsesHandle429(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), cache)
	account := oauthFleetAccount(5, PlatformOpenAI, AccountTypeOAuth, map[string]any{AccountExtraOAuthFleetSoft429: false})

	svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded"}}`))
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Empty(t, cache.sets)
}

func TestHandleUpstreamError_OAuthFleetSoft429_TempRuleDoesNotSteal(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), cache)
	account := oauthFleetAccount(6, PlatformOpenAI, AccountTypeOAuth, nil)
	account.Credentials = map[string]any{
		"temp_unschedulable_enabled": true,
		"temp_unschedulable_rules": []any{
			map[string]any{
				"error_code":       float64(429),
				"keywords":         []any{"rate limit"},
				"duration_minutes": float64(10),
			},
		},
	}

	svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached"}}`))
	require.Zero(t, repo.tempUnschedCalls)
	require.Zero(t, repo.rateLimitCalls)
	require.Len(t, cache.sets, 1)
}

func TestHandleUpstreamError_OAuthFleetHardWindowStillPersists(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), cache)
	account := oauthFleetAccount(7, PlatformAnthropic, AccountTypeOAuth, nil)
	resetAt := time.Now().Add(3 * time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-5h-utilization", "1.02")
	headers.Set("anthropic-ratelimit-unified-5h-reset", strconv.FormatInt(resetAt.Unix(), 10))

	svc.HandleUpstreamError(context.Background(), account, 429, headers, []byte(`{"error":{"type":"rate_limit_exceeded","message":"rate limit"}}`))
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Equal(t, resetAt, repo.lastRateLimitAt)
	require.Empty(t, cache.sets)
}

func TestHandleUpstreamError_OAuthFleetUsageLimitReachedStillHard(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	settings := oauthFleetEnabledSettings()
	settings.SoftBodyCodes = []string{"usage_limit_reached", "rate_limit_exceeded"}
	svc, repo := newOAuthFleetRateLimit(t, settings, cache)
	account := oauthFleetAccount(8, PlatformOpenAI, AccountTypeOAuth, nil)
	resetUnix := time.Now().Add(2 * time.Hour).Unix()

	svc.HandleUpstreamError(context.Background(), account, 429, http.Header{}, []byte(`{"error":{"type":"usage_limit_reached","resets_at":`+strconv.FormatInt(resetUnix, 10)+`}}`))
	require.Equal(t, 1, repo.rateLimitCalls)
	require.Empty(t, cache.sets)
}

func TestHandleUpstreamError_OAuthFleetLongResetPolicy(t *testing.T) {
	longHeaders := http.Header{}
	longHeaders.Set("x-codex-primary-used-percent", "40")
	longHeaders.Set("x-codex-primary-reset-after-seconds", "400000")
	longHeaders.Set("x-codex-primary-window-minutes", "10080")
	longHeaders.Set("x-codex-secondary-used-percent", "10")
	longHeaders.Set("x-codex-secondary-reset-after-seconds", "10000")
	longHeaders.Set("x-codex-secondary-window-minutes", "300")
	body := []byte(`{"error":{"type":"rate_limit_exceeded"}}`)

	t.Run("soft", func(t *testing.T) {
		cache := &oauthFleetSoft429CacheStub{}
		svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), cache)
		account := oauthFleetAccount(10, PlatformOpenAI, AccountTypeOAuth, nil)
		svc.HandleUpstreamError(context.Background(), account, 429, longHeaders, body)
		require.Zero(t, repo.rateLimitCalls)
		require.Len(t, cache.sets, 1)
	})
	t.Run("hard", func(t *testing.T) {
		cache := &oauthFleetSoft429CacheStub{}
		settings := oauthFleetEnabledSettings()
		settings.LongResetPolicy = OAuthFleetSoft429LongResetHard
		svc, repo := newOAuthFleetRateLimit(t, settings, cache)
		account := oauthFleetAccount(11, PlatformOpenAI, AccountTypeOAuth, nil)
		svc.HandleUpstreamError(context.Background(), account, 429, longHeaders, body)
		require.Equal(t, 1, repo.rateLimitCalls)
		require.Empty(t, cache.sets)
	})
	t.Run("threshold", func(t *testing.T) {
		cache := &oauthFleetSoft429CacheStub{}
		settings := oauthFleetEnabledSettings()
		settings.LongResetPolicy = OAuthFleetSoft429LongResetThreshold
		settings.LongResetThresholdSeconds = 60
		svc, repo := newOAuthFleetRateLimit(t, settings, cache)
		account := oauthFleetAccount(12, PlatformOpenAI, AccountTypeOAuth, nil)
		svc.HandleUpstreamError(context.Background(), account, 429, longHeaders, body)
		require.Equal(t, 1, repo.rateLimitCalls)
		require.Empty(t, cache.sets)
	})
}

func TestHandleUpstreamError_OAuth401Unchanged(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{}
	svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), cache)
	account := oauthFleetAccount(13, PlatformOpenAI, AccountTypeOAuth, nil)

	shouldDisable := svc.HandleUpstreamError(context.Background(), account, 401, http.Header{}, []byte(`unauthorized`))
	require.True(t, shouldDisable)
	require.Equal(t, 1, repo.tempUnschedCalls)
	require.Empty(t, cache.sets)
}

func TestMergeOAuthFleetSoft429Exclusions(t *testing.T) {
	cache := &oauthFleetSoft429CacheStub{listed: []int64{1, 2}}
	svc := NewRateLimitService(&oauthFleetSoft429RepoStub{}, nil, &config.Config{}, nil, nil)
	svc.SetOAuthFleetSoft429Cache(cache)

	merged := svc.MergeOAuthFleetSoft429Exclusions(context.Background(), map[int64]struct{}{3: {}}, false)
	require.Contains(t, merged, int64(1))
	require.Contains(t, merged, int64(2))
	require.Contains(t, merged, int64(3))

	sticky := svc.MergeOAuthFleetSoft429Exclusions(context.Background(), map[int64]struct{}{3: {}}, true)
	require.NotContains(t, sticky, int64(1))
	require.Contains(t, sticky, int64(3))

	cache.listErr = context.DeadlineExceeded
	failOpen := svc.MergeOAuthFleetSoft429Exclusions(context.Background(), map[int64]struct{}{3: {}}, false)
	require.Equal(t, map[int64]struct{}{3: {}}, failOpen)
}

func TestOAuthFleetSoft429HasHardAffinity(t *testing.T) {
	require.False(t, oauthFleetSoft429HasHardAffinity("", 0))
	require.False(t, oauthFleetSoft429HasHardAffinity("   ", 0))
	require.True(t, oauthFleetSoft429HasHardAffinity("resp_abc", 0))
	require.True(t, oauthFleetSoft429HasHardAffinity("", 9))
}

func TestGatewaySelectAccount_OAuthFleetSoft429Layer2_SessionHashIsNotAffinity(t *testing.T) {
	ctx := context.Background()
	repo := &mockAccountRepoForPlatform{
		accounts: []Account{
			{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 1, Status: StatusActive, Schedulable: true, Concurrency: 5},
			{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 2, Status: StatusActive, Schedulable: true, Concurrency: 5},
		},
		accountsByID: map[int64]*Account{},
	}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	gwCache := &mockGatewayCacheForPlatform{}
	rl := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	rl.SetOAuthFleetSoft429Cache(&oauthFleetSoft429CacheStub{listed: []int64{1}})
	svc := &GatewayService{
		accountRepo:      repo,
		cache:            gwCache,
		cfg:              testConfig(),
		rateLimitService: rl,
	}

	picked, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "generated-session-hash", "claude-3-5-sonnet-20241022", nil)
	require.NoError(t, err)
	require.NotNil(t, picked)
	require.Equal(t, int64(2), picked.ID, "new request with sessionHash but no sticky binding must honor layer-2")

	gwCache.sessionBindings = map[string]int64{"generated-session-hash": 1}
	pinned, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "generated-session-hash", "claude-3-5-sonnet-20241022", nil)
	require.NoError(t, err)
	require.NotNil(t, pinned)
	require.Equal(t, int64(1), pinned.ID, "existing sticky binding must skip layer-2")
}

func TestOAuthFleetSoft429Override_SurvivesCredentialReplace(t *testing.T) {
	account := oauthFleetAccount(14, PlatformOpenAI, AccountTypeOAuth, map[string]any{AccountExtraOAuthFleetSoft429: true})
	account.Credentials = map[string]any{"access_token": "old", "refresh_token": "r1"}
	account.Credentials = map[string]any{"access_token": "new", "refresh_token": "r2"}
	require.Equal(t, true, account.Extra[AccountExtraOAuthFleetSoft429])
	require.True(t, oauthFleetSoft429Applies(account, DefaultOAuthFleetSoft429Settings()))
}

func TestCheckErrorPolicy_OAuthFleetSoft429SkipsTempUnsched(t *testing.T) {
	svc, repo := newOAuthFleetRateLimit(t, oauthFleetEnabledSettings(), &oauthFleetSoft429CacheStub{})
	account := oauthFleetAccount(15, PlatformOpenAI, AccountTypeOAuth, nil)
	account.Credentials = map[string]any{
		"temp_unschedulable_enabled": true,
		"temp_unschedulable_rules": []any{
			map[string]any{
				"error_code":       float64(429),
				"keywords":         []any{"rate limit"},
				"duration_minutes": float64(10),
			},
		},
	}
	result := svc.CheckErrorPolicy(context.Background(), account, 429, []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached"}}`))
	require.Equal(t, ErrorPolicyNone, result)
	require.Zero(t, repo.tempUnschedCalls)
}
