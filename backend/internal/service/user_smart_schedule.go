package service

import (
	"context"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultSmartScheduleCooldownMinutes = 15
	MinSmartScheduleCooldownMinutes     = 1
	MaxSmartScheduleCooldownMinutes     = 1440
	SmartScheduleUserCacheTTL           = 10 * time.Minute
	ProbeConcurrencyModeFollowN         = "follow_n"
	ProbeConcurrencyModeCustom          = "custom"
)

// SmartScheduleAccountMember is one pool member + optional pair cap.
// CurrentConcurrency is read-only live occupancy for this user on this account.
// CooldownUntil is read-only: when the pair cooldown HASH field is still in the future.
// Paused is a durable user×account skip; the account stays in the pool. Writes ignore it.
type SmartScheduleAccountMember struct {
	AccountID          int64                         `json:"account_id"`
	Platform           string                        `json:"platform"`
	MaxConcurrency     *int                          `json:"max_concurrency,omitempty"`
	SortOrder          *int                          `json:"sort_order"`
	Priority           int                           `json:"priority"` // read-only live accounts.priority; writes ignore
	CurrentConcurrency int                           `json:"current_concurrency,omitempty"`
	CooldownUntil      *time.Time                    `json:"cooldown_until,omitempty"`
	ResumeUntil        *time.Time                    `json:"resume_until,omitempty"`
	ResumeChipUntil    *time.Time                    `json:"resume_chip_until,omitempty"`
	Paused             bool                          `json:"paused,omitempty"`
	Probing            bool                          `json:"probing"`
	Pinned             bool                          `json:"pinned"`
	ProbeCap           *int                          `json:"probe_cap,omitempty"`
	PairQuality        *SmartSchedulePairQualityView `json:"pair_quality,omitempty"`
	WillCool           bool                          `json:"will_cool"`
}

// SmartScheduleSortAssignment is one membership row's pool display order.
type SmartScheduleSortAssignment struct {
	AccountID int64 `json:"account_id"`
	SortOrder int   `json:"sort_order"`
}

// SmartSchedulePlatformPolicy is the hot-path view of one (user, platform) policy.
type SmartSchedulePlatformPolicy struct {
	Enabled                  bool
	QualityMaxP50TTFTMs      *int
	QualityMinSuccessRate    *float64
	QualityWindowSamples     *int
	QualityMinSuccessSamples *int
	QualityMinTTFTSamples    *int
	QualityCondition         *string
	CooldownMinutes          int
	ProbeConcurrencyMode     string
	ProbeConcurrency         *int
	UpdatedAt                time.Time
	AccountIDs               map[int64]struct{}
	Caps                     map[int64]int
	SortOrders               map[int64]int
	Paused                   map[int64]struct{}
}

func (p *SmartSchedulePlatformPolicy) HasAccount(accountID int64) bool {
	if p == nil || accountID <= 0 {
		return false
	}
	_, ok := p.AccountIDs[accountID]
	return ok
}

func (p *SmartSchedulePlatformPolicy) PairCap(accountID int64) int {
	if p == nil || accountID <= 0 || len(p.Caps) == 0 {
		return 0
	}
	n := p.Caps[accountID]
	if n < 1 {
		return 0
	}
	return n
}

func (p *SmartSchedulePlatformPolicy) IsPaused(accountID int64) bool {
	if p == nil || accountID <= 0 || len(p.Paused) == 0 {
		return false
	}
	_, ok := p.Paused[accountID]
	return ok
}

func (p *SmartSchedulePlatformPolicy) HasQualityMetrics() bool {
	if p == nil {
		return false
	}
	return p.QualityMaxP50TTFTMs != nil || p.QualityMinSuccessRate != nil
}

func (p *SmartSchedulePlatformPolicy) QualityGate() QualityHardCloseSettings {
	if p == nil || !p.HasQualityMetrics() {
		return QualityHardCloseSettings{}
	}
	n := p.WindowN()
	return fillUserQualityGateDefaults(QualityHardCloseSettings{
		MaxP50TTFTMs:      p.QualityMaxP50TTFTMs,
		MinSuccessRate:    p.QualityMinSuccessRate,
		MinSuccessSamples: n,
		MinTTFTSamples:    n,
		Condition:         derefString(p.QualityCondition),
	})
}

func (p *SmartSchedulePlatformPolicy) MemberCount() int {
	if p == nil {
		return 0
	}
	return len(p.AccountIDs)
}

// ProbeDesiredConcurrency is the probing in-flight target before the member cap ceiling.
// follow_n / omit / invalid stored custom → window N. custom + 1–100 → that number.
// This is not account_quality_window_n.
func (p *SmartSchedulePlatformPolicy) ProbeDesiredConcurrency() int {
	if p == nil {
		return DefaultSmartScheduleWindowN
	}
	mode, custom := EchoProbeConcurrency(p.ProbeConcurrencyMode, p.ProbeConcurrency)
	if mode == ProbeConcurrencyModeCustom && custom != nil {
		return *custom
	}
	return p.WindowN()
}

// ProbeInFlightCap is min(desired, memberCap) or desired when the member has no cap.
func (p *SmartSchedulePlatformPolicy) ProbeInFlightCap(memberCap int) int {
	return ProbeInFlightCap(p.ProbeDesiredConcurrency(), memberCap)
}

// UserSmartScheduleBundle is the cached per-user map of platform policies.
type UserSmartScheduleBundle struct {
	Policies map[string]*SmartSchedulePlatformPolicy `json:"policies"`
}

func (b *UserSmartScheduleBundle) Policy(platform string) *SmartSchedulePlatformPolicy {
	if b == nil || len(b.Policies) == 0 {
		return nil
	}
	return b.Policies[normalizeSmartSchedulePlatform(platform)]
}

func (b *UserSmartScheduleBundle) EnabledPolicy(platform string) *SmartSchedulePlatformPolicy {
	p := b.Policy(platform)
	if p == nil || !p.Enabled || p.MemberCount() == 0 {
		// Empty pool has no business meaning. Treat it as disabled so the
		// user falls back to legacy allow/deny/gate/cap instead of failing
		// closed (e.g. last member CASCADE-deleted, stale cache).
		return nil
	}
	return p
}

// SmartScheduleLookup is the hot-path reader for user smart-schedule policy + cooldown.
// Cache miss / lookup error fail open to legacy AdmitsScheduleUser.
// Redis miss / no probe mark = not probing (no backfill).
type SmartScheduleLookup interface {
	Lookup(ctx context.Context, userID int64) *UserSmartScheduleBundle
	CooldownActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool
	StartCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time)
	GetPairQuality(ctx context.Context, accountID, userID int64, platform string) *PairQualityLive
	IsProbing(ctx context.Context, accountID, userID int64, platform string) bool
	MarkProbing(ctx context.Context, accountID, userID int64, platform string)
	ClearProbing(ctx context.Context, accountID, userID int64, platform string)
	GraduateProbing(ctx context.Context, accountID, userID int64, platform string)
	IsPinned(ctx context.Context, accountID, userID int64, platform string) bool
	MarkPinned(ctx context.Context, accountID, userID int64, platform string)
	ClearPinned(ctx context.Context, accountID, userID int64, platform string)
	PairResumeActive(ctx context.Context, accountID, userID int64, platform string, now time.Time) bool
	ClearPairResume(ctx context.Context, accountID, userID int64, platform string)
}

// UserSmartScheduleCache is the admin + hot-path cache (invalidate on save).
type UserSmartScheduleCache interface {
	SmartScheduleLookup
	Invalidate(ctx context.Context, userID int64) error
	ClearCooldown(ctx context.Context, accountID, userID int64, platform string) error
	ClearCooldownAllPlatforms(ctx context.Context, accountID, userID int64) error
	SetCooldown(ctx context.Context, accountID, userID int64, platform string, minutes int, now time.Time) (time.Time, error)
	ApplyMemberPaused(ctx context.Context, userID, accountID int64, platform string, paused bool) error
	GetCooldownUntilBatch(ctx context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]time.Time
	IngestPairQuality(ctx context.Context, accountID, userID int64, platform string, n int, success bool, firstTokenMs *int) *PairQualityLive
	ZeroPairQuality(ctx context.Context, accountID, userID int64, platform string, eventType string)
	GetPairQualityBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]*PairQualityLive
	ListPairQualitySnapshots(ctx context.Context, accountID, userID int64, platform string, limit int) []PairQualitySnapshot
	ListPairQualityEvents(ctx context.Context, accountID, userID int64, platform string, limit int) []PairQualityEvent
	AppendPairQualityEvent(ctx context.Context, accountID, userID int64, platform string, event PairQualityEvent)
	IsProbingBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]bool
	IsPinnedBatch(ctx context.Context, accountIDs []int64, userID int64, platform string) map[int64]bool
	MarkPairResume(ctx context.Context, accountID, userID int64, platform string) error
	GetPairResumeUntilBatch(ctx context.Context, accountIDs []int64, userID int64, platform string, now time.Time) map[int64]PairResumeUntil
}

// PairResumeUntil is the per-pool 豁免期 overlay (not account-quality:resume).
type PairResumeUntil struct {
	ChipUntil  time.Time
	WatchUntil time.Time
}

func (p PairResumeUntil) Active(now time.Time) bool {
	return p.ChipUntil.After(now) || p.WatchUntil.After(now)
}

// UserSmartScheduleRepository persists policies and pool members.
type UserSmartScheduleRepository interface {
	ListByUser(ctx context.Context, userID int64) (*UserSmartScheduleBundle, error)
	ListByUsers(ctx context.Context, userIDs []int64) (map[int64]*UserSmartScheduleBundle, error)
	ReplacePlatform(ctx context.Context, userID int64, platform string, policy SmartSchedulePlatformWrite) error
	UpdateSortOrders(ctx context.Context, userID int64, platform string, orders []SmartScheduleSortAssignment) error
	SetMemberPaused(ctx context.Context, userID, accountID int64, platform string, paused bool) error
}

// UserSmartScheduleSummary is the compact list-column view of one user's smart schedule.
type UserSmartScheduleSummary struct {
	EnabledPlatforms []string       `json:"enabled_platforms"`
	PoolCounts       map[string]int `json:"pool_counts"`
}

// SmartSchedulePlatformWrite is the replace-all payload for one platform.
type SmartSchedulePlatformWrite struct {
	Enabled                  bool
	QualityMaxP50TTFTMs      *int
	QualityMinSuccessRate    *float64
	QualityWindowSamples     *int
	QualityWindowN           *int
	QualityMinSuccessSamples *int
	QualityMinTTFTSamples    *int
	QualityCondition         *string
	CooldownMinutes          int
	ProbeConcurrencyMode     string
	ProbeConcurrency         *int
	Accounts                 []SmartScheduleAccountMember
}

// SmartSchedulePlatformView is the admin GET/PUT response for one platform.
type SmartSchedulePlatformView struct {
	Enabled                  bool                         `json:"enabled"`
	QualityMaxP50TTFTMs      *int                         `json:"quality_max_p50_ttft_ms"`
	QualityMinSuccessRate    *float64                     `json:"quality_min_success_rate"`
	QualityWindowSamples     *int                         `json:"quality_window_samples"`
	QualityWindowN           *int                         `json:"quality_window_n"`
	QualityMinSuccessSamples *int                         `json:"quality_min_success_samples"`
	QualityMinTTFTSamples    *int                         `json:"quality_min_ttft_samples"`
	QualityCondition         *string                      `json:"quality_condition"`
	CooldownMinutes          int                          `json:"cooldown_minutes"`
	ProbeConcurrencyMode     string                       `json:"probe_concurrency_mode"`
	ProbeConcurrency         *int                         `json:"probe_concurrency"`
	UpdatedAt                time.Time                    `json:"updated_at,omitempty"`
	Accounts                 []SmartScheduleAccountMember `json:"accounts"`
}

// UserSmartScheduleView is GET /admin/users/:id/smart-schedule.
type UserSmartScheduleView struct {
	UserID          int64                                `json:"user_id"`
	DefaultPlatform string                               `json:"default_platform,omitempty"`
	Platforms       map[string]SmartSchedulePlatformView `json:"platforms"`
}

const (
	PairAdmissionPaused     = "paused"
	PairAdmissionCooling    = "cooling"
	PairAdmissionProbing    = "probing"
	PairAdmissionResumed    = "resumed"
	PairAdmissionSelectable = "selectable"
	PairAdmissionPinned     = "pinned"
)

// PairAdmissionResult is the admin response after switching a pair's live state.
type PairAdmissionResult struct {
	AccountID     int64      `json:"account_id"`
	UserID        int64      `json:"user_id"`
	State         string     `json:"state"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
	Probing       bool       `json:"probing"`
	Pinned        bool       `json:"pinned"`
	ProbeCap      *int       `json:"probe_cap,omitempty"`
}

func ClampSmartScheduleCooldownMinutes(minutes int) int {
	if minutes < MinSmartScheduleCooldownMinutes {
		return DefaultSmartScheduleCooldownMinutes
	}
	if minutes > MaxSmartScheduleCooldownMinutes {
		return MaxSmartScheduleCooldownMinutes
	}
	return minutes
}

func NormalizeProbeConcurrencyMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return ProbeConcurrencyModeFollowN, nil
	}
	switch mode {
	case ProbeConcurrencyModeFollowN, ProbeConcurrencyModeCustom:
		return mode, nil
	default:
		return "", infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "probe_concurrency_mode must be follow_n or custom")
	}
}

// NormalizeProbeConcurrencyWrite validates admin writes.
// omit/empty mode → follow_n and ignore custom. custom requires 1–100.
func NormalizeProbeConcurrencyWrite(mode string, custom *int) (string, *int, error) {
	normalized, err := NormalizeProbeConcurrencyMode(mode)
	if err != nil {
		return "", nil, err
	}
	if normalized == ProbeConcurrencyModeFollowN {
		return ProbeConcurrencyModeFollowN, nil, nil
	}
	if custom == nil {
		return "", nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "probe_concurrency is required when probe_concurrency_mode is custom")
	}
	if *custom < MinSmartScheduleWindowN || *custom > MaxSmartScheduleWindowN {
		return "", nil, infraerrors.BadRequest("SMART_SCHEDULE_INVALID_QUALITY", "probe_concurrency must be between 1 and 100")
	}
	copied := *custom
	return ProbeConcurrencyModeCustom, &copied, nil
}

// EchoProbeConcurrency is the GET/cache shape. Invalid stored custom falls back to follow_n.
func EchoProbeConcurrency(mode string, custom *int) (string, *int) {
	normalized, value, err := NormalizeProbeConcurrencyWrite(mode, custom)
	if err != nil {
		return ProbeConcurrencyModeFollowN, nil
	}
	return normalized, value
}

func normalizeSmartSchedulePlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func IsAllowedSmartSchedulePlatform(platform string) bool {
	return IsAllowedQuotaPlatform(normalizeSmartSchedulePlatform(platform))
}

func lookupEnabledSmartPolicy(ctx context.Context, lookup SmartScheduleLookup, userID int64, platform string) *SmartSchedulePlatformPolicy {
	if lookup == nil || userID <= 0 {
		return nil
	}
	return lookup.Lookup(ctx, userID).EnabledPolicy(platform)
}

func resolvePairMaxConcurrency(ctx context.Context, account *Account, lookup SmartScheduleLookup) int {
	max, _ := resolvePairSlotAcquire(ctx, account, lookup)
	return max
}
