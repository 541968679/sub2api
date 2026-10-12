package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	AccountExtraPublicScheduleQuality = "public_schedule_quality"

	PublicScheduleStateSelectable = "selectable"
	PublicScheduleStateCooling    = "cooling"
	PublicScheduleStateProbing    = "probing"
	PublicScheduleStatePaused     = "paused"
	PublicScheduleStatePinned     = "pinned"
	PublicScheduleStateResumed    = "resumed"

	DefaultPublicScheduleWindowN         = DefaultSmartScheduleWindowN
	DefaultPublicScheduleCooldownMinutes = DefaultSmartScheduleCooldownMinutes
	DefaultPublicScheduleMaxP50TTFTMs    = DefaultQualityHardCloseMaxP50TTFTMs
	DefaultPublicScheduleMinSuccessRate  = DefaultQualityHardCloseMinSuccessRate
	DefaultPublicScheduleSchedK          = 3
	DefaultPublicScheduleSchedC          = 2

	PublicScheduleStoreWindowN     = MaxSmartScheduleWindowN
	PublicScheduleResumeChip       = AccountQualityWindow
	PublicScheduleResumeWatch      = 2 * AccountQualityWindow
	PublicScheduleWindowTTL        = 7 * 24 * time.Hour
	PublicSchedulePolicyCacheTTL   = 10 * time.Minute
	PublicScheduleSettingsCacheTTL = 30 * time.Second
)

// PublicScheduleQualitySettings is the site-wide public-schedule quality plane.
// Enabled is the master switch for this slice; per-account overlay is ignored
// until that UI is wired.
type PublicScheduleQualitySettings struct {
	Enabled                        bool     `json:"enabled"`
	TTFTWindowN                    int      `json:"ttft_window_n"`
	SuccessWindowN                 int      `json:"success_window_n"`
	QualityMaxP50TTFTMs            *int     `json:"quality_max_p50_ttft_ms"`
	QualityMinSuccessRate          *float64 `json:"quality_min_success_rate"`
	QualityMaxP50DurationMs        *int     `json:"quality_max_p50_duration_ms"`
	QualityMaxSlowInWindow         *int     `json:"quality_max_slow_in_window"`
	QualityMaxConsecutiveSlow      *int     `json:"quality_max_consecutive_slow"`
	QualitySchedWindowN            *int     `json:"quality_sched_window_n"`
	QualitySchedMaxSlowInWindow    *int     `json:"quality_sched_max_slow_in_window"`
	QualitySchedMaxConsecutiveSlow *int     `json:"quality_sched_max_consecutive_slow"`
	CooldownMinutes                int      `json:"cooldown_minutes"`
	SoftCooldown                   bool     `json:"soft_cooldown"`
}

// PublicScheduleQualityOverlay is stored in extra.public_schedule_quality.
// Nil pointer fields inherit the site template.
type PublicScheduleQualityOverlay struct {
	Enabled                        bool     `json:"enabled"`
	TTFTWindowN                    *int     `json:"ttft_window_n,omitempty"`
	SuccessWindowN                 *int     `json:"success_window_n,omitempty"`
	QualityMaxP50TTFTMs            *int     `json:"quality_max_p50_ttft_ms,omitempty"`
	QualityMinSuccessRate          *float64 `json:"quality_min_success_rate,omitempty"`
	QualityMaxP50DurationMs        *int     `json:"quality_max_p50_duration_ms,omitempty"`
	QualityMaxSlowInWindow         *int     `json:"quality_max_slow_in_window,omitempty"`
	QualityMaxConsecutiveSlow      *int     `json:"quality_max_consecutive_slow,omitempty"`
	QualitySchedWindowN            *int     `json:"quality_sched_window_n,omitempty"`
	QualitySchedMaxSlowInWindow    *int     `json:"quality_sched_max_slow_in_window,omitempty"`
	QualitySchedMaxConsecutiveSlow *int     `json:"quality_sched_max_consecutive_slow,omitempty"`
	CooldownMinutes                *int     `json:"cooldown_minutes,omitempty"`
	SoftCooldown                   *bool    `json:"soft_cooldown,omitempty"`
}

// PublicScheduleQualityResolved is overlay over site defaults.
type PublicScheduleQualityResolved struct {
	Enabled                        bool     `json:"enabled"`
	TTFTWindowN                    int      `json:"ttft_window_n"`
	SuccessWindowN                 int      `json:"success_window_n"`
	QualityMaxP50TTFTMs            *int     `json:"quality_max_p50_ttft_ms"`
	QualityMinSuccessRate          *float64 `json:"quality_min_success_rate"`
	QualityMaxP50DurationMs        *int     `json:"quality_max_p50_duration_ms"`
	QualityMaxSlowInWindow         *int     `json:"quality_max_slow_in_window"`
	QualityMaxConsecutiveSlow      *int     `json:"quality_max_consecutive_slow"`
	QualitySchedWindowN            *int     `json:"quality_sched_window_n"`
	QualitySchedMaxSlowInWindow    *int     `json:"quality_sched_max_slow_in_window"`
	QualitySchedMaxConsecutiveSlow *int     `json:"quality_sched_max_consecutive_slow"`
	CooldownMinutes                int      `json:"cooldown_minutes"`
	SoftCooldown                   bool     `json:"soft_cooldown"`
}

// PublicScheduleRuntimeState is the account-level six-state record. Redis miss
// is fail-open selectable.
type PublicScheduleRuntimeState struct {
	State     string    `json:"state"`
	Until     time.Time `json:"until,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Soft      bool      `json:"soft,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// PublicScheduleQualityView is the admin GET/PUT account payload.
type PublicScheduleQualityView struct {
	Overlay  PublicScheduleQualityOverlay  `json:"overlay"`
	Resolved PublicScheduleQualityResolved `json:"resolved"`
	State    string                        `json:"state"`
	Until    *time.Time                    `json:"until,omitempty"`
	Reason   string                        `json:"reason,omitempty"`
	WillCool bool                          `json:"will_cool"`
	Quality  *SmartSchedulePairQualityView `json:"quality,omitempty"`
}

func DefaultPublicScheduleQualitySettings() *PublicScheduleQualitySettings {
	p50 := DefaultPublicScheduleMaxP50TTFTMs
	rate := DefaultPublicScheduleMinSuccessRate
	k := DefaultPublicScheduleSchedK
	c := DefaultPublicScheduleSchedC
	return &PublicScheduleQualitySettings{
		TTFTWindowN:                    DefaultPublicScheduleWindowN,
		SuccessWindowN:                 DefaultPublicScheduleWindowN,
		QualityMaxP50TTFTMs:            &p50,
		QualityMinSuccessRate:          &rate,
		QualitySchedMaxSlowInWindow:    &k,
		QualitySchedMaxConsecutiveSlow: &c,
		CooldownMinutes:                DefaultPublicScheduleCooldownMinutes,
		SoftCooldown:                   false,
	}
}

func DefaultPublicScheduleQualityOverlay() PublicScheduleQualityOverlay {
	return PublicScheduleQualityOverlay{Enabled: false}
}

func NormalizePublicScheduleQualitySettings(settings *PublicScheduleQualitySettings) {
	if settings == nil {
		return
	}
	settings.TTFTWindowN = ClampSmartScheduleWindowN(settings.TTFTWindowN)
	settings.SuccessWindowN = ClampSmartScheduleWindowN(settings.SuccessWindowN)
	settings.CooldownMinutes = ClampSmartScheduleCooldownMinutes(settings.CooldownMinutes)
	settings.QualityMaxP50TTFTMs = normalizeOptionalPositiveInt(settings.QualityMaxP50TTFTMs)
	settings.QualityMaxP50DurationMs = normalizeOptionalPositiveInt(settings.QualityMaxP50DurationMs)
	settings.QualityMaxSlowInWindow = normalizeOptionalPositiveInt(settings.QualityMaxSlowInWindow)
	settings.QualityMaxConsecutiveSlow = normalizeOptionalPositiveInt(settings.QualityMaxConsecutiveSlow)
	settings.QualitySchedWindowN = normalizeOptionalWindowN(settings.QualitySchedWindowN)
	settings.QualitySchedMaxSlowInWindow = normalizeOptionalPositiveInt(settings.QualitySchedMaxSlowInWindow)
	settings.QualitySchedMaxConsecutiveSlow = normalizeOptionalPositiveInt(settings.QualitySchedMaxConsecutiveSlow)
	if settings.QualitySchedMaxSlowInWindow == nil {
		k := DefaultPublicScheduleSchedK
		settings.QualitySchedMaxSlowInWindow = &k
	}
	if settings.QualitySchedMaxConsecutiveSlow == nil {
		c := DefaultPublicScheduleSchedC
		settings.QualitySchedMaxConsecutiveSlow = &c
	}
	if settings.QualityMinSuccessRate != nil {
		rate := *settings.QualityMinSuccessRate
		if rate < 0 {
			rate = 0
		}
		if rate > 1 {
			rate = 1
		}
		settings.QualityMinSuccessRate = &rate
	}
}

func ValidatePublicScheduleQualitySettings(settings *PublicScheduleQualitySettings) error {
	if settings == nil {
		return fmt.Errorf("public schedule quality settings required")
	}
	NormalizePublicScheduleQualitySettings(settings)
	return nil
}

func ValidatePublicScheduleQualityOverlay(overlay *PublicScheduleQualityOverlay) error {
	if overlay == nil {
		return fmt.Errorf("public schedule quality overlay required")
	}
	if overlay.TTFTWindowN != nil {
		n := ClampSmartScheduleWindowN(*overlay.TTFTWindowN)
		overlay.TTFTWindowN = &n
	}
	if overlay.SuccessWindowN != nil {
		n := ClampSmartScheduleWindowN(*overlay.SuccessWindowN)
		overlay.SuccessWindowN = &n
	}
	if overlay.CooldownMinutes != nil {
		n := ClampSmartScheduleCooldownMinutes(*overlay.CooldownMinutes)
		overlay.CooldownMinutes = &n
	}
	if overlay.QualityMinSuccessRate != nil {
		rate := *overlay.QualityMinSuccessRate
		if rate < 0 {
			rate = 0
		}
		if rate > 1 {
			rate = 1
		}
		overlay.QualityMinSuccessRate = &rate
	}
	return nil
}

func ParseAccountPublicScheduleOverlay(extra map[string]any) PublicScheduleQualityOverlay {
	def := DefaultPublicScheduleQualityOverlay()
	if extra == nil {
		return def
	}
	raw, ok := extra[AccountExtraPublicScheduleQuality]
	if !ok || raw == nil {
		return def
	}
	payload, err := json.Marshal(raw)
	if err != nil {
		return def
	}
	var overlay PublicScheduleQualityOverlay
	if err := json.Unmarshal(payload, &overlay); err != nil {
		return def
	}
	_ = ValidatePublicScheduleQualityOverlay(&overlay)
	return overlay
}

func AccountPublicScheduleEnabled(account *Account) bool {
	if account == nil {
		return false
	}
	return ParseAccountPublicScheduleOverlay(account.Extra).Enabled
}

func ResolvePublicScheduleQuality(site PublicScheduleQualitySettings, overlay PublicScheduleQualityOverlay) PublicScheduleQualityResolved {
	NormalizePublicScheduleQualitySettings(&site)
	_ = ValidatePublicScheduleQualityOverlay(&overlay)
	// This slice tests the global plane only. Overlay knobs stay in the type
	// for a later per-account UI and must not change runtime.
	return PublicScheduleQualityResolved{
		Enabled:                        site.Enabled,
		TTFTWindowN:                    site.TTFTWindowN,
		SuccessWindowN:                 site.SuccessWindowN,
		QualityMaxP50TTFTMs:            site.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:          site.QualityMinSuccessRate,
		QualityMaxP50DurationMs:        site.QualityMaxP50DurationMs,
		QualityMaxSlowInWindow:         site.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      site.QualityMaxConsecutiveSlow,
		QualitySchedWindowN:            site.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    site.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: site.QualitySchedMaxConsecutiveSlow,
		CooldownMinutes:                site.CooldownMinutes,
		SoftCooldown:                   site.SoftCooldown,
	}
}

func (r PublicScheduleQualityResolved) asPolicy() *SmartSchedulePlatformPolicy {
	return &SmartSchedulePlatformPolicy{
		Enabled:                        r.Enabled,
		QualityMaxP50TTFTMs:            r.QualityMaxP50TTFTMs,
		QualityMinSuccessRate:          r.QualityMinSuccessRate,
		QualityMinTTFTSamples:          intPtr(r.TTFTWindowN),
		QualityMinSuccessSamples:       intPtr(r.SuccessWindowN),
		QualityMaxP50DurationMs:        r.QualityMaxP50DurationMs,
		QualityMaxSlowInWindow:         r.QualityMaxSlowInWindow,
		QualityMaxConsecutiveSlow:      r.QualityMaxConsecutiveSlow,
		QualitySchedWindowN:            r.QualitySchedWindowN,
		QualitySchedMaxSlowInWindow:    r.QualitySchedMaxSlowInWindow,
		QualitySchedMaxConsecutiveSlow: r.QualitySchedMaxConsecutiveSlow,
		CooldownMinutes:                r.CooldownMinutes,
		SoftCooldown:                   r.SoftCooldown,
	}
}

func (r PublicScheduleQualityResolved) SchedKnobs() QualityEvalKnobs {
	return SchedQualityKnobs(r.asPolicy())
}

func (r PublicScheduleQualityResolved) ProbeKnobs() QualityEvalKnobs {
	return ProbeQualityKnobs(r.asPolicy())
}

func (st *PublicScheduleRuntimeState) Normalized(now time.Time) string {
	if st == nil || strings.TrimSpace(st.State) == "" {
		return PublicScheduleStateSelectable
	}
	state := strings.ToLower(strings.TrimSpace(st.State))
	switch state {
	case PublicScheduleStateCooling:
		if !st.Until.IsZero() && !st.Until.After(now) {
			return PublicScheduleStateProbing
		}
		return PublicScheduleStateCooling
	case PublicScheduleStateResumed:
		if !st.Until.IsZero() && !st.Until.After(now) {
			return PublicScheduleStateSelectable
		}
		return PublicScheduleStateResumed
	case PublicScheduleStateProbing, PublicScheduleStatePaused, PublicScheduleStatePinned, PublicScheduleStateSelectable:
		return state
	default:
		return PublicScheduleStateSelectable
	}
}

func (st *PublicScheduleRuntimeState) IsDemoted(now time.Time) bool {
	switch st.Normalized(now) {
	case PublicScheduleStateCooling, PublicScheduleStateProbing, PublicScheduleStatePaused:
		return true
	default:
		return false
	}
}

func (st *PublicScheduleRuntimeState) UntilPtr() *time.Time {
	if st == nil || st.Until.IsZero() {
		return nil
	}
	until := st.Until.UTC()
	return &until
}

func projectPublicScheduleLive(live *PairQualityLive, nTTFT, nOK int) *PairQualityLive {
	nTTFT = ClampSmartScheduleWindowN(nTTFT)
	nOK = ClampSmartScheduleWindowN(nOK)
	if live == nil {
		return ZeroPairQualityLiveWindows(nTTFT, nOK)
	}
	out := *live
	out.TTFTMs = trimFIFOInt(live.TTFTMs, nTTFT)
	out.DurationMs = trimFIFOInt(live.DurationMs, nTTFT)
	out.OK = trimFIFOBool(live.OK, nOK)
	out.NTTFT = nTTFT
	out.NOK = nOK
	out.NDuration = nTTFT
	out.N = maxSmartScheduleWindowN(nTTFT, nOK)
	RecomputePairQuality(&out)
	return &out
}

func ingestPublicScheduleSample(live *PairQualityLive, success bool, firstTokenMs, durationMs *int) *PairQualityLive {
	if live == nil {
		live = &PairQualityLive{}
	}
	n := PublicScheduleStoreWindowN
	live.NTTFT = n
	live.NOK = n
	live.NDuration = n
	live.N = n
	if success && firstTokenMs != nil && *firstTokenMs >= 0 {
		live.TTFTMs = appendFIFOInt(live.TTFTMs, *firstTokenMs, n)
	}
	if success && durationMs != nil && *durationMs >= 0 {
		live.DurationMs = appendFIFOInt(live.DurationMs, *durationMs, n)
	}
	live.OK = appendFIFOBool(live.OK, success, n)
	RecomputePairQuality(live)
	return live
}

func formatPublicScheduleReasons(reasons []SmartScheduleCooldownReason) string {
	if len(reasons) == 0 {
		return ""
	}
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		detail := strings.TrimSpace(reason.Detail)
		if detail == "" {
			detail = strings.TrimSpace(reason.Code)
		}
		if detail != "" {
			parts = append(parts, detail)
		}
	}
	return strings.Join(parts, "; ")
}

func preferPublicScheduleAccounts(
	ctx context.Context,
	runtime *PublicScheduleQualityService,
	lookup SmartScheduleLookup,
	userID int64,
	platform string,
	accounts []*Account,
) []*Account {
	if runtime == nil || len(accounts) == 0 {
		return accounts
	}
	if !isUnpooledScheduleUser(ctx, lookup, userID, platform) {
		return accounts
	}
	healthy, demoted := runtime.partitionAccounts(ctx, accounts)
	if len(healthy) > 0 {
		return healthy
	}
	if len(demoted) > 0 {
		return demoted
	}
	return accounts
}

func preferPublicScheduleOpenAICandidates(
	ctx context.Context,
	runtime *PublicScheduleQualityService,
	lookup SmartScheduleLookup,
	userID int64,
	platform string,
	candidates []openAIAccountCandidateScore,
) []openAIAccountCandidateScore {
	if runtime == nil || len(candidates) == 0 {
		return candidates
	}
	accounts := make([]*Account, 0, len(candidates))
	for i := range candidates {
		accounts = append(accounts, candidates[i].account)
	}
	preferred := preferPublicScheduleAccounts(ctx, runtime, lookup, userID, platform, accounts)
	if len(preferred) == 0 || len(preferred) == len(candidates) {
		return candidates
	}
	keep := make(map[int64]struct{}, len(preferred))
	for _, acc := range preferred {
		if acc != nil {
			keep[acc.ID] = struct{}{}
		}
	}
	out := make([]openAIAccountCandidateScore, 0, len(preferred))
	for _, candidate := range candidates {
		if candidate.account == nil {
			continue
		}
		if _, ok := keep[candidate.account.ID]; ok {
			out = append(out, candidate)
		}
	}
	if len(out) == 0 {
		return candidates
	}
	return out
}

const (
	// publicScheduleSwitchLoadGap is how many load-rate points a peer must be
	// under a demoted sticky account before the session leaves it.
	publicScheduleSwitchLoadGap = 20
	// publicScheduleSwitchTTFTGapMs is the smallest absolute TTFT improvement
	// that counts as a clearer peer. The peer must also be at most two thirds
	// of the sticky TTFT, so a few hundred milliseconds do not move a session.
	publicScheduleSwitchTTFTGapMs = 1000
)

func scheduleSwitchLoadRate(loadByID map[int64]*AccountLoadInfo, accountID int64) int {
	if loadByID == nil || accountID <= 0 {
		return 0
	}
	info := loadByID[accountID]
	if info == nil {
		return 0
	}
	return info.LoadRate
}

func publicScheduleSwitchTTFT(ctx context.Context, runtime *PublicScheduleQualityService, accountID int64, override map[int64]int) *int {
	if override != nil {
		if ms, ok := override[accountID]; ok && ms >= 0 {
			copied := ms
			return &copied
		}
	}
	if runtime == nil || accountID <= 0 {
		return nil
	}
	live := runtime.cacheWindow(ctx, accountID)
	if live == nil || live.P50TTFTMs == nil || *live.P50TTFTMs < 0 {
		return nil
	}
	copied := *live.P50TTFTMs
	return &copied
}

// peerClearlyHealthier reports whether a peer is far enough ahead on load or
// TTFT to take a session off a demoted sticky account.
func peerClearlyHealthier(stickyLoad, peerLoad int, stickyTTFT, peerTTFT *int) bool {
	if peerLoad < 100 && stickyLoad-peerLoad >= publicScheduleSwitchLoadGap {
		return true
	}
	if stickyTTFT == nil || peerTTFT == nil || *stickyTTFT <= 0 || *peerTTFT < 0 {
		return false
	}
	gap := *stickyTTFT - *peerTTFT
	if gap < publicScheduleSwitchTTFTGapMs {
		return false
	}
	return *peerTTFT*3 <= *stickyTTFT*2
}

// stickyNeedsPublicHealthEscape reports whether a public-pool pin is demoted.
// Callers use it to skip load and TTFT reads on a healthy sticky hit.
func stickyNeedsPublicHealthEscape(
	ctx context.Context,
	runtime *PublicScheduleQualityService,
	lookup SmartScheduleLookup,
	userID int64,
	platform string,
	sticky *Account,
) bool {
	if runtime == nil || sticky == nil {
		return false
	}
	if !isUnpooledScheduleUser(ctx, lookup, userID, platform) {
		return false
	}
	return runtime.IsDemoted(ctx, sticky)
}

// shouldEscapeDemotedStickyForHealthierPeer clears a public-schedule demoted
// session when a peer is clearly healthier, including when that peer is also
// demoted. A selectable peer is handled by shouldEscapeSessionStickyForPublicQuality.
func shouldEscapeDemotedStickyForHealthierPeer(
	ctx context.Context,
	runtime *PublicScheduleQualityService,
	lookup SmartScheduleLookup,
	userID int64,
	platform string,
	sticky *Account,
	candidates []*Account,
	loadByID map[int64]*AccountLoadInfo,
	ttftMsByID map[int64]int,
) bool {
	if runtime == nil || sticky == nil {
		return false
	}
	if !isUnpooledScheduleUser(ctx, lookup, userID, platform) {
		return false
	}
	if !runtime.IsDemoted(ctx, sticky) {
		return false
	}
	stickyLoad := scheduleSwitchLoadRate(loadByID, sticky.ID)
	stickyTTFT := publicScheduleSwitchTTFT(ctx, runtime, sticky.ID, ttftMsByID)
	for _, acc := range candidates {
		if acc == nil || acc.ID == sticky.ID {
			continue
		}
		peerLoad := scheduleSwitchLoadRate(loadByID, acc.ID)
		peerTTFT := publicScheduleSwitchTTFT(ctx, runtime, acc.ID, ttftMsByID)
		if peerClearlyHealthier(stickyLoad, peerLoad, stickyTTFT, peerTTFT) {
			return true
		}
	}
	return false
}

func shouldEscapeSessionStickyForPublicQuality(
	ctx context.Context,
	runtime *PublicScheduleQualityService,
	lookup SmartScheduleLookup,
	userID int64,
	platform string,
	sticky *Account,
	candidates []*Account,
) bool {
	if runtime == nil || sticky == nil {
		return false
	}
	if !isUnpooledScheduleUser(ctx, lookup, userID, platform) {
		return false
	}
	if !runtime.IsDemoted(ctx, sticky) {
		return false
	}
	for _, acc := range candidates {
		if acc == nil || acc.ID == sticky.ID {
			continue
		}
		if !runtime.IsDemoted(ctx, acc) {
			return true
		}
	}
	return false
}

func normalizeOptionalPositiveInt(v *int) *int {
	if v == nil || *v < 1 {
		return nil
	}
	copied := *v
	return &copied
}

func normalizeOptionalWindowN(v *int) *int {
	if v == nil || *v < 1 {
		return nil
	}
	n := ClampSmartScheduleWindowN(*v)
	return &n
}
