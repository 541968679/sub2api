package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

const (
	UserScheduleModeUnrestricted = "unrestricted"
	UserScheduleModeAllow        = "allow"
	UserScheduleModeDeny         = "deny"
)

// ScheduleUserRef is the admin-facing hydration of a schedule-list user.
type ScheduleUserRef struct {
	ID                       int64    `json:"id"`
	Email                    string   `json:"email"`
	Deleted                  bool     `json:"deleted"`
	Allow                    bool     `json:"allow"`
	Deny                     bool     `json:"deny"`
	MaxConcurrency           *int     `json:"max_concurrency,omitempty"`
	QualityMaxP50TTFTMs      *int     `json:"quality_max_p50_ttft_ms,omitempty"`
	QualityMinSuccessRate    *float64 `json:"quality_min_success_rate,omitempty"`
	QualityMinSuccessSamples *int     `json:"quality_min_success_samples,omitempty"`
	QualityMinTTFTSamples    *int     `json:"quality_min_ttft_samples,omitempty"`
	QualityCondition         *string  `json:"quality_condition,omitempty"`
	QualityBlocked           bool     `json:"quality_blocked,omitempty"`
	QualityResumedUntil      *int64   `json:"quality_resumed_until,omitempty"`
	QualityWindowUntil       *int64   `json:"quality_window_until,omitempty"`
}

// UserConcurrencyEntry is one replace-all pair-cap row on account update.
type UserConcurrencyEntry struct {
	UserID         int64 `json:"user_id"`
	MaxConcurrency int   `json:"max_concurrency"`
}

// UserConcurrencyPatch merges one user's pair cap without rewriting lists.
// MaxConcurrency nil or 0 deletes that user's cap.
type UserConcurrencyPatch struct {
	UserID         int64 `json:"user_id"`
	MaxConcurrency *int  `json:"max_concurrency"`
}

// UserQualityGateEntry is one replace-all quality-gate row on account update.
type UserQualityGateEntry struct {
	UserID            int64    `json:"user_id"`
	MaxP50TTFTMs      *int     `json:"quality_max_p50_ttft_ms"`
	MinSuccessRate    *float64 `json:"quality_min_success_rate"`
	MinSuccessSamples *int     `json:"quality_min_success_samples"`
	MinTTFTSamples    *int     `json:"quality_min_ttft_samples"`
	Condition         *string  `json:"quality_condition"`
}

// UserQualityGatePatch merges one user's quality gate without rewriting lists.
// All quality fields null/omitted deletes that user's gate.
type UserQualityGatePatch struct {
	UserID            int64    `json:"user_id"`
	MaxP50TTFTMs      *int     `json:"quality_max_p50_ttft_ms"`
	MinSuccessRate    *float64 `json:"quality_min_success_rate"`
	MinSuccessSamples *int     `json:"quality_min_success_samples"`
	MinTTFTSamples    *int     `json:"quality_min_ttft_samples"`
	Condition         *string  `json:"quality_condition"`
}

// AccountUserScheduleWrite is the full join-table replacement for one account.
type AccountUserScheduleWrite struct {
	AllowUserIDs     []int64
	DenyUserIDs      []int64
	UserConcurrency  map[int64]int
	UserQualityGates map[int64]QualityHardCloseSettings
}

func NormalizeUserScheduleMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case UserScheduleModeAllow:
		return UserScheduleModeAllow
	case UserScheduleModeDeny:
		return UserScheduleModeDeny
	case UserScheduleModeUnrestricted, "":
		return UserScheduleModeUnrestricted
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func IsValidUserScheduleMode(mode string) bool {
	switch NormalizeUserScheduleMode(mode) {
	case UserScheduleModeUnrestricted, UserScheduleModeAllow, UserScheduleModeDeny:
		return true
	default:
		return false
	}
}

func (a *Account) normalizedUserScheduleMode() string {
	if a == nil {
		return UserScheduleModeUnrestricted
	}
	return NormalizeUserScheduleMode(a.UserScheduleMode)
}

func (a *Account) hasUserScheduleRules() bool {
	if a == nil {
		return false
	}
	return len(a.AllowUserIDs) > 0 || len(a.DenyUserIDs) > 0 || len(a.UserConcurrency) > 0 || len(a.UserQualityGates) > 0
}

// DeriveLegacyUserSchedule fills leftover exclusive-mode fields from the
// independent lists so old readers keep a best-effort view. Both lists
// nonempty cannot be expressed as a single mode; those stay unrestricted
// and leftover readers must not use mode as admission truth.
func (a *Account) DeriveLegacyUserSchedule() {
	if a == nil {
		return
	}
	allow := normalizeScheduleUserIDs(a.AllowUserIDs)
	deny := normalizeScheduleUserIDs(a.DenyUserIDs)
	a.AllowUserIDs = allow
	a.DenyUserIDs = deny
	a.UserConcurrency = normalizeUserConcurrencyMap(a.UserConcurrency)
	a.UserQualityGates = copyUserQualityGates(a.UserQualityGates)
	switch {
	case len(allow) > 0 && len(deny) == 0:
		a.UserScheduleMode = UserScheduleModeAllow
		a.ScheduleUserIDs = append([]int64(nil), allow...)
	case len(deny) > 0 && len(allow) == 0:
		a.UserScheduleMode = UserScheduleModeDeny
		a.ScheduleUserIDs = append([]int64(nil), deny...)
	default:
		a.UserScheduleMode = UserScheduleModeUnrestricted
		a.ScheduleUserIDs = unionScheduleUserIDs(allow, deny, concurrencyUserIDs(a.UserConcurrency), qualityGateUserIDs(a.UserQualityGates))
	}
}

// AllowsScheduleUser reports whether this account may be scheduled for userID.
//
// Priority:
//  1. userID<=0 and any allow/deny/pair-cap/quality-gate rule exists → false (fail closed)
//  2. deny list hit → false (cap ignored)
//  3. allow list nonempty and miss → false (cap ignored)
//  4. else true
func (a *Account) AllowsScheduleUser(userID int64) bool {
	if a == nil {
		return false
	}
	if userID <= 0 {
		return !a.hasUserScheduleRules()
	}
	if containsScheduleUserID(a.DenyUserIDs, userID) {
		return false
	}
	if len(a.AllowUserIDs) > 0 && !containsScheduleUserID(a.AllowUserIDs, userID) {
		return false
	}
	return true
}

// PairMaxConcurrency returns the explicit pair cap N>=1, or 0 if unset.
// Callers must still check AllowsScheduleUser first; a denied user may still
// have a stored number that must not take effect.
func (a *Account) PairMaxConcurrency(userID int64) int {
	if a == nil || userID <= 0 || len(a.UserConcurrency) == 0 {
		return 0
	}
	n := a.UserConcurrency[userID]
	if n < 1 {
		return 0
	}
	return n
}

// QualityGateBlocksUser reports whether this account's live 15-minute quality
// breaches the optional per-user gate. No gate, nil stats, or zero judged
// metrics do not block (fail open). Result is pair exclude, not temp-unschedulable.
func (a *Account) QualityGateBlocksUser(userID int64, stats *AccountQualityStats) bool {
	return a.qualityGateBlocksUserAt(userID, stats, time.Now().UTC())
}

func (a *Account) qualityGateBlocksUserAt(userID int64, stats *AccountQualityStats, now time.Time) bool {
	if a == nil || userID <= 0 || len(a.UserQualityGates) == 0 {
		return false
	}
	gate, ok := a.UserQualityGates[userID]
	if !ok || !qualityGateHasMetric(gate) {
		return false
	}
	if UserQualityResumeActive(stats, userID, now) {
		return false
	}
	gate = fillUserQualityGateDefaults(gate)
	blocked, _ := EvaluateAccountQualityHardClose(stats, gate, false)
	return blocked
}

// AdmitsScheduleUser is identity allow plus the optional quality gate.
func (a *Account) AdmitsScheduleUser(userID int64, stats *AccountQualityStats) bool {
	return a.AllowsScheduleUser(userID) && !a.QualityGateBlocksUser(userID, stats)
}

// AccountQualityLiveCache is the Redis live 15-minute quality window used on
// the hot path. Cache miss / nil stats must not block admission.
type AccountQualityLiveCache interface {
	Get(ctx context.Context, accountID int64) (*AccountQualityStats, error)
	Replace(ctx context.Context, stats map[int64]*AccountQualityStats) error
	MarkUserResume(ctx context.Context, accountID, userID int64) error
	MarkUserQualityWindow(ctx context.Context, accountID, userID int64) error
	ClearUserResume(ctx context.Context, accountID, userID int64) error
	MarkAccountResume(ctx context.Context, accountID int64) error
}

func loadLiveQualityForAdmission(ctx context.Context, cache AccountQualityLiveCache, account *Account, force bool) *AccountQualityStats {
	if account == nil || cache == nil {
		return nil
	}
	if !force && len(account.UserQualityGates) == 0 {
		return nil
	}
	stats, err := cache.Get(ctx, account.ID)
	if err != nil {
		return nil
	}
	return stats
}

func admitsScheduleUser(ctx context.Context, account *Account, cache AccountQualityLiveCache, lookup SmartScheduleLookup) bool {
	if account == nil {
		return false
	}
	userID := scheduleUserIDFromContext(ctx, 0)
	lookupPlatform := smartScheduleLookupPlatformFromCtx(ctx, account, lookup)
	policy := lookupEnabledSmartPolicy(ctx, lookup, userID, lookupPlatform)
	if policy == nil {
		return account.AdmitsScheduleUser(userID, loadLiveQualityForAdmission(ctx, cache, account, false))
	}
	if !policy.HasAccount(account.ID) {
		return false
	}
	if policy.IsPaused(account.ID) {
		return false
	}
	now := time.Now().UTC()
	if lookup != nil && lookup.IsPinned(ctx, account.ID, userID, lookupPlatform) {
		return true
	}
	if lookup != nil && lookup.CooldownActive(ctx, account.ID, userID, lookupPlatform, now) {
		return false
	}
	// Pair 豁免期 lives on smart-schedule:resume:{platform}:{account}.
	// account-quality:resume is Track A only and must not skip evaluate here.
	probing := lookup != nil && lookup.IsProbing(ctx, account.ID, userID, lookupPlatform)
	if pairQualityResumeBlocksEvaluate(ctx, lookup, probing, account.ID, userID, lookupPlatform, now) {
		return true
	}
	clearLeftoverPairResumeIfProbing(ctx, lookup, probing, account.ID, userID, lookupPlatform, now)
	if !policy.HasQualityMetrics() && !probing {
		return true
	}
	var pair *PairQualityLive
	if lookup != nil {
		pair = lookup.GetPairQuality(ctx, account.ID, userID, lookupPlatform)
	}
	return evaluateSmartSchedulePairQuality(ctx, lookup, account.ID, userID, lookupPlatform, policy, pair, now)
}

func containsScheduleUserID(ids []int64, userID int64) bool {
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

func normalizeUserConcurrencyMap(in map[int64]int) map[int64]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int64]int, len(in))
	for userID, n := range in {
		if userID <= 0 || n < 1 {
			continue
		}
		out[userID] = n
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func concurrencyUserIDs(in map[int64]int) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, 0, len(in))
	for userID, n := range in {
		if userID <= 0 || n < 1 {
			continue
		}
		out = append(out, userID)
	}
	return out
}

func unionScheduleUserIDs(groups ...[]int64) []int64 {
	var merged []int64
	for _, group := range groups {
		merged = append(merged, group...)
	}
	return normalizeScheduleUserIDs(merged)
}

func copyUserConcurrencyMap(in map[int64]int) map[int64]int {
	normalized := normalizeUserConcurrencyMap(in)
	if len(normalized) == 0 {
		return nil
	}
	out := make(map[int64]int, len(normalized))
	for k, v := range normalized {
		out[k] = v
	}
	return out
}

func buildAccountUserScheduleWrite(allow, deny []int64, caps map[int64]int, gates map[int64]QualityHardCloseSettings) AccountUserScheduleWrite {
	return AccountUserScheduleWrite{
		AllowUserIDs:     normalizeScheduleUserIDs(allow),
		DenyUserIDs:      normalizeScheduleUserIDs(deny),
		UserConcurrency:  copyUserConcurrencyMap(caps),
		UserQualityGates: copyUserQualityGates(gates),
	}
}

func qualityGateHasMetric(gate QualityHardCloseSettings) bool {
	return gate.MaxP50TTFTMs != nil || gate.MinSuccessRate != nil
}

func qualityGateHasConfiguredColumn(p50 *int, rate *float64, _ *int, _ *int, _ *string) bool {
	// A gate is enabled only when at least one judged metric is set.
	// Samples/condition are modifiers; condition-only or samples-only must not
	// create a map entry (that would fail-close userID<=0 without ever blocking).
	return p50 != nil || rate != nil
}

func fillUserQualityGateDefaults(gate QualityHardCloseSettings) QualityHardCloseSettings {
	if gate.MinSuccessSamples < 1 {
		gate.MinSuccessSamples = DefaultQualityHardCloseMinSuccessSamples
	}
	if gate.MinTTFTSamples < 1 {
		gate.MinTTFTSamples = DefaultQualityHardCloseMinTTFTSamples
	}
	cond := strings.ToLower(strings.TrimSpace(gate.Condition))
	if cond != QualityHardCloseConditionAnd {
		gate.Condition = QualityHardCloseConditionOr
	} else {
		gate.Condition = cond
	}
	gate.Enabled = true
	return gate
}

func userQualityGateFromFields(p50 *int, rate *float64, minSuccess, minTTFT *int, condition *string) (QualityHardCloseSettings, bool) {
	if !qualityGateHasConfiguredColumn(p50, rate, minSuccess, minTTFT, condition) {
		return QualityHardCloseSettings{}, false
	}
	return fillUserQualityGateDefaults(QualityHardCloseSettings{
		MaxP50TTFTMs:      p50,
		MinSuccessRate:    rate,
		MinSuccessSamples: derefPositiveOrZero(minSuccess),
		MinTTFTSamples:    derefPositiveOrZero(minTTFT),
		Condition:         derefString(condition),
	}), true
}

func derefPositiveOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func qualityGateClears(p50 *int, rate *float64, minSuccess, minTTFT *int, condition *string) bool {
	return !qualityGateHasConfiguredColumn(p50, rate, minSuccess, minTTFT, condition)
}

func qualityGateUserIDs(in map[int64]QualityHardCloseSettings) []int64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]int64, 0, len(in))
	for userID := range in {
		if userID <= 0 {
			continue
		}
		out = append(out, userID)
	}
	return out
}

func copyUserQualityGates(in map[int64]QualityHardCloseSettings) map[int64]QualityHardCloseSettings {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int64]QualityHardCloseSettings, len(in))
	for userID, gate := range in {
		if userID <= 0 || !qualityGateHasMetric(gate) {
			continue
		}
		out[userID] = fillUserQualityGateDefaults(gate)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func stampScheduleUserQuality(ref *ScheduleUserRef, gate QualityHardCloseSettings, ok bool) {
	if ref == nil {
		return
	}
	if !ok {
		ref.QualityMaxP50TTFTMs = nil
		ref.QualityMinSuccessRate = nil
		ref.QualityMinSuccessSamples = nil
		ref.QualityMinTTFTSamples = nil
		ref.QualityCondition = nil
		return
	}
	gate = fillUserQualityGateDefaults(gate)
	ref.QualityMaxP50TTFTMs = gate.MaxP50TTFTMs
	ref.QualityMinSuccessRate = gate.MinSuccessRate
	minSuccess := gate.MinSuccessSamples
	minTTFT := gate.MinTTFTSamples
	cond := gate.Condition
	ref.QualityMinSuccessSamples = &minSuccess
	ref.QualityMinTTFTSamples = &minTTFT
	ref.QualityCondition = &cond
}

// stampScheduleUsersQualityRuntime marks list/edit chips from the same live
// cache the hot path reads. Cache miss / nil stats fail open (no 已停).
func stampScheduleUsersQualityRuntime(account *Account, users []ScheduleUserRef, stats *AccountQualityStats, now time.Time) {
	if account == nil || len(users) == 0 {
		return
	}
	for i := range users {
		if users[i].QualityMaxP50TTFTMs == nil && users[i].QualityMinSuccessRate == nil {
			continue
		}
		if until, ok := userResumeUntilUnix(stats, users[i].ID); ok && until > now.Unix() {
			copied := until
			users[i].QualityResumedUntil = &copied
		}
		if until, ok := userWatchingUntilUnix(stats, users[i].ID); ok && until > now.Unix() {
			copied := until
			users[i].QualityWindowUntil = &copied
		}
		users[i].QualityBlocked = account.qualityGateBlocksUserAt(users[i].ID, stats, now)
	}
}

// scheduleUserIDFromContext prefers an explicit positive user ID, then ctxkey.UserID.
func scheduleUserIDFromContext(ctx context.Context, explicit int64) int64 {
	if explicit > 0 {
		return explicit
	}
	if ctx == nil {
		return 0
	}
	if v, ok := ctx.Value(ctxkey.UserID).(int64); ok && v > 0 {
		return v
	}
	return 0
}

func withScheduleUserID(ctx context.Context, explicit int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	uid := scheduleUserIDFromContext(ctx, explicit)
	if uid <= 0 {
		return ctx
	}
	if existing, ok := ctx.Value(ctxkey.UserID).(int64); ok && existing == uid {
		return ctx
	}
	return context.WithValue(ctx, ctxkey.UserID, uid)
}
