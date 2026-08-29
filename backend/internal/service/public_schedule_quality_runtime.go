package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// PublicScheduleQualityService is the account-quality plane for unpooled
// selection. Writers are all completions on the account; readers are unpooled
// users only. Disabled accounts never change selection.
type PublicScheduleQualityService struct {
	cache       PublicScheduleQualityCache
	settings    *SettingService
	accountRepo AccountRepository

	mu            sync.Mutex
	site          *PublicScheduleQualitySettings
	siteAt        time.Time
	overlayByID   map[int64]publicScheduleOverlayCache
}

type publicScheduleOverlayCache struct {
	overlay   PublicScheduleQualityOverlay
	expiresAt time.Time
}

func NewPublicScheduleQualityService(
	cache PublicScheduleQualityCache,
	settings *SettingService,
	accountRepo AccountRepository,
) *PublicScheduleQualityService {
	return &PublicScheduleQualityService{
		cache:       cache,
		settings:    settings,
		accountRepo: accountRepo,
		overlayByID: map[int64]publicScheduleOverlayCache{},
	}
}

func (s *PublicScheduleQualityService) ObserveCompletion(ctx context.Context, obs AccountQualityObservation) {
	if s == nil || obs.AccountID <= 0 {
		return
	}
	resolved, _ := s.resolveAccount(ctx, obs.AccountID, nil)
	state := s.effectiveState(ctx, obs.AccountID)
	name := state.Normalized(time.Now().UTC())

	ingestMain := true
	ingestSoft := false
	evaluate := resolved.Enabled
	switch name {
	case PublicScheduleStateCooling:
		ingestMain = false
		ingestSoft = resolved.SoftCooldown
	case PublicScheduleStatePaused:
		ingestMain = false
	case PublicScheduleStatePinned, PublicScheduleStateResumed:
		evaluate = false
	}

	var window, soft *PairQualityLive
	if ingestMain {
		window = ingestPublicScheduleSample(s.cacheWindow(ctx, obs.AccountID), obs.Success, obs.FirstTokenMs, obs.DurationMs)
		s.storeWindow(ctx, obs.AccountID, window)
	} else {
		window = s.cacheWindow(ctx, obs.AccountID)
	}
	if ingestSoft {
		soft = ingestPublicScheduleSample(s.cacheSoft(ctx, obs.AccountID), obs.Success, obs.FirstTokenMs, obs.DurationMs)
		s.storeSoft(ctx, obs.AccountID, soft)
	}

	evalLive := projectPublicScheduleLive(window, resolved.TTFTWindowN, resolved.SuccessWindowN)
	sched := EvalQuality(evalLive, resolved.SchedKnobs())
	willCool := resolved.Enabled && name == PublicScheduleStateSelectable && sched.State == LatencyEvalFail
	slog.Info("public_quality_shadow",
		"account_id", obs.AccountID,
		"enabled", resolved.Enabled,
		"state", name,
		"will_cool", willCool,
		"eval", sched.State,
		"reason", formatPublicScheduleReasons(sched.Reasons),
	)
	if !evaluate {
		return
	}

	now := time.Now().UTC()
	switch name {
	case PublicScheduleStateSelectable:
		if sched.State == LatencyEvalFail {
			s.startCooldown(ctx, obs.AccountID, resolved, formatPublicScheduleReasons(sched.Reasons), now)
		}
	case PublicScheduleStateCooling:
		if !resolved.SoftCooldown || soft == nil {
			return
		}
		softLive := projectPublicScheduleLive(soft, resolved.TTFTWindowN, resolved.SuccessWindowN)
		if EvalQuality(softLive, resolved.SchedKnobs()).State == LatencyEvalPass {
			s.becomeSelectable(ctx, obs.AccountID)
		}
	case PublicScheduleStateProbing:
		probe := EvalQuality(evalLive, resolved.ProbeKnobs())
		switch probe.State {
		case LatencyEvalPass:
			s.becomeSelectable(ctx, obs.AccountID)
		case LatencyEvalFail:
			s.startCooldown(ctx, obs.AccountID, resolved, formatPublicScheduleReasons(probe.Reasons), now)
		}
	}
}

func (s *PublicScheduleQualityService) partitionAccounts(ctx context.Context, accounts []*Account) (healthy, demoted []*Account) {
	if len(accounts) == 0 {
		return nil, nil
	}
	if !s.siteEnabled(ctx) {
		return accounts, nil
	}
	ids := make([]int64, 0, len(accounts))
	for _, acc := range accounts {
		if acc != nil && acc.ID > 0 {
			ids = append(ids, acc.ID)
		}
	}
	states := s.stateBatch(ctx, ids)
	now := time.Now().UTC()
	for _, acc := range accounts {
		if acc == nil {
			continue
		}
		if states[acc.ID].IsDemoted(now) {
			demoted = append(demoted, acc)
			continue
		}
		healthy = append(healthy, acc)
	}
	return healthy, demoted
}

func (s *PublicScheduleQualityService) IsDemoted(ctx context.Context, account *Account) bool {
	if s == nil || account == nil || !s.siteEnabled(ctx) {
		return false
	}
	return s.effectiveState(ctx, account.ID).IsDemoted(time.Now().UTC())
}

func (s *PublicScheduleQualityService) GetView(ctx context.Context, account *Account) (*PublicScheduleQualityView, error) {
	if account == nil {
		return nil, fmt.Errorf("account required")
	}
	site, err := s.SiteSettings(ctx)
	if err != nil {
		return nil, err
	}
	overlay := ParseAccountPublicScheduleOverlay(account.Extra)
	resolved := ResolvePublicScheduleQuality(*site, overlay)
	return s.buildView(ctx, account.ID, overlay, resolved, s.effectiveState(ctx, account.ID), time.Now().UTC()), nil
}

func (s *PublicScheduleQualityService) GetViewBatch(ctx context.Context, accountIDs []int64) map[int64]*PublicScheduleQualityView {
	out := map[int64]*PublicScheduleQualityView{}
	if s == nil {
		return out
	}
	site, err := s.SiteSettings(ctx)
	if err != nil || site == nil {
		site = DefaultPublicScheduleQualitySettings()
	}
	overlay := DefaultPublicScheduleQualityOverlay()
	resolved := ResolvePublicScheduleQuality(*site, overlay)
	now := time.Now().UTC()
	states := s.stateBatch(ctx, accountIDs)
	for _, id := range accountIDs {
		if id <= 0 {
			continue
		}
		out[id] = s.buildView(ctx, id, overlay, resolved, states[id], now)
	}
	return out
}

func (s *PublicScheduleQualityService) buildView(
	ctx context.Context,
	accountID int64,
	overlay PublicScheduleQualityOverlay,
	resolved PublicScheduleQualityResolved,
	state *PublicScheduleRuntimeState,
	now time.Time,
) *PublicScheduleQualityView {
	name := state.Normalized(now)
	window := projectPublicScheduleLive(s.cacheWindow(ctx, accountID), resolved.TTFTWindowN, resolved.SuccessWindowN)
	eval := EvalQuality(window, resolved.SchedKnobs())
	qv := window.View()
	applyPhaseMetricsAlias(&qv, qualityPhaseMetrics(window, resolved.SchedKnobs(), resolved.QualityMaxP50DurationMs != nil))
	view := &PublicScheduleQualityView{
		Overlay:  overlay,
		Resolved: resolved,
		State:    name,
		Until:    state.UntilPtr(),
		Reason:   state.Reason,
		WillCool: resolved.Enabled && name == PublicScheduleStateSelectable && eval.State == LatencyEvalFail,
		Quality:  &qv,
	}
	if eval.State == LatencyEvalFail && view.Reason == "" {
		view.Reason = formatPublicScheduleReasons(eval.Reasons)
	}
	return view
}

func (s *PublicScheduleQualityService) UpdateOverlay(ctx context.Context, accountID int64, overlay PublicScheduleQualityOverlay) error {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return fmt.Errorf("public schedule quality unavailable")
	}
	if err := ValidatePublicScheduleQualityOverlay(&overlay); err != nil {
		return err
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
		AccountExtraPublicScheduleQuality: overlay,
	}); err != nil {
		return err
	}
	s.rememberOverlay(accountID, overlay)
	return nil
}

func (s *PublicScheduleQualityService) SetManualState(ctx context.Context, accountID int64, state string) error {
	if s == nil || accountID <= 0 {
		return fmt.Errorf("public schedule quality unavailable")
	}
	now := time.Now().UTC()
	name := strings.ToLower(strings.TrimSpace(state))
	switch name {
	case PublicScheduleStateSelectable:
		s.becomeSelectable(ctx, accountID)
	case PublicScheduleStateCooling:
		site, err := s.SiteSettings(ctx)
		if err != nil {
			return err
		}
		_, overlay := s.resolveAccount(ctx, accountID, nil)
		resolved := ResolvePublicScheduleQuality(*site, overlay)
		s.forceState(ctx, accountID, &PublicScheduleRuntimeState{
			State:     PublicScheduleStateCooling,
			Until:     now.Add(time.Duration(resolved.CooldownMinutes) * time.Minute),
			Reason:    "manual",
			Soft:      resolved.SoftCooldown,
			UpdatedAt: now,
		})
	case PublicScheduleStateProbing:
		s.forceState(ctx, accountID, &PublicScheduleRuntimeState{State: PublicScheduleStateProbing, UpdatedAt: now})
	case PublicScheduleStatePaused:
		s.forceState(ctx, accountID, &PublicScheduleRuntimeState{State: PublicScheduleStatePaused, UpdatedAt: now})
	case PublicScheduleStatePinned:
		s.forceState(ctx, accountID, &PublicScheduleRuntimeState{State: PublicScheduleStatePinned, UpdatedAt: now})
	case PublicScheduleStateResumed:
		s.forceState(ctx, accountID, &PublicScheduleRuntimeState{
			State:     PublicScheduleStateResumed,
			Until:     now.Add(PublicScheduleResumeWatch),
			UpdatedAt: now,
		})
	default:
		return fmt.Errorf("unknown public schedule state %q", state)
	}
	return nil
}

func (s *PublicScheduleQualityService) SiteSettings(ctx context.Context) (*PublicScheduleQualitySettings, error) {
	if s == nil {
		return DefaultPublicScheduleQualitySettings(), nil
	}
	s.mu.Lock()
	if s.site != nil && time.Since(s.siteAt) < PublicScheduleSettingsCacheTTL {
		out := *s.site
		s.mu.Unlock()
		return &out, nil
	}
	s.mu.Unlock()
	if s.settings == nil {
		return DefaultPublicScheduleQualitySettings(), nil
	}
	settings, err := s.settings.GetPublicScheduleQualitySettings(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.site = settings
	s.siteAt = time.Now()
	s.mu.Unlock()
	return settings, nil
}

func (s *PublicScheduleQualityService) InvalidateSiteSettings() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.site = nil
	s.siteAt = time.Time{}
	s.mu.Unlock()
}

func (s *PublicScheduleQualityService) siteEnabled(ctx context.Context) bool {
	site, err := s.SiteSettings(ctx)
	return err == nil && site != nil && site.Enabled
}

func (s *PublicScheduleQualityService) rememberOverlay(accountID int64, overlay PublicScheduleQualityOverlay) {
	if s == nil || accountID <= 0 {
		return
	}
	s.mu.Lock()
	s.overlayByID[accountID] = publicScheduleOverlayCache{
		overlay:   overlay,
		expiresAt: time.Now().Add(PublicSchedulePolicyCacheTTL),
	}
	s.mu.Unlock()
}

func (s *PublicScheduleQualityService) resolveAccount(ctx context.Context, accountID int64, account *Account) (PublicScheduleQualityResolved, PublicScheduleQualityOverlay) {
	site, _ := s.SiteSettings(ctx)
	if site == nil {
		site = DefaultPublicScheduleQualitySettings()
	}
	overlay := DefaultPublicScheduleQualityOverlay()
	if account != nil {
		overlay = ParseAccountPublicScheduleOverlay(account.Extra)
		s.rememberOverlay(accountID, overlay)
	} else if cached, ok := s.cachedOverlay(accountID); ok {
		overlay = cached
	} else if s.accountRepo != nil {
		if acc, err := s.accountRepo.GetByID(ctx, accountID); err == nil && acc != nil {
			overlay = ParseAccountPublicScheduleOverlay(acc.Extra)
			s.rememberOverlay(accountID, overlay)
		}
	}
	return ResolvePublicScheduleQuality(*site, overlay), overlay
}

func (s *PublicScheduleQualityService) cachedOverlay(accountID int64) (PublicScheduleQualityOverlay, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.overlayByID[accountID]
	if !ok || time.Now().After(entry.expiresAt) {
		return PublicScheduleQualityOverlay{}, false
	}
	return entry.overlay, true
}

func (s *PublicScheduleQualityService) effectiveState(ctx context.Context, accountID int64) *PublicScheduleRuntimeState {
	if s == nil || s.cache == nil || accountID <= 0 {
		return &PublicScheduleRuntimeState{State: PublicScheduleStateSelectable}
	}
	now := time.Now().UTC()
	st := s.cache.GetState(ctx, accountID)
	if st == nil {
		return &PublicScheduleRuntimeState{State: PublicScheduleStateSelectable}
	}
	name := st.Normalized(now)
	if name != st.State {
		if name == PublicScheduleStateSelectable {
			s.cache.ClearState(ctx, accountID)
			s.cache.ClearSoft(ctx, accountID)
			return &PublicScheduleRuntimeState{State: PublicScheduleStateSelectable}
		}
		st.State = name
		if name == PublicScheduleStateProbing {
			st.Until = time.Time{}
			st.Soft = false
			s.cache.ClearSoft(ctx, accountID)
		}
		st.UpdatedAt = now
		s.cache.SetState(ctx, accountID, st)
	}
	return st
}

func (s *PublicScheduleQualityService) stateBatch(ctx context.Context, accountIDs []int64) map[int64]*PublicScheduleRuntimeState {
	out := map[int64]*PublicScheduleRuntimeState{}
	if s == nil || s.cache == nil {
		return out
	}
	raw := s.cache.GetStateBatch(ctx, accountIDs)
	now := time.Now().UTC()
	for _, id := range accountIDs {
		st := raw[id]
		if st == nil {
			out[id] = &PublicScheduleRuntimeState{State: PublicScheduleStateSelectable}
			continue
		}
		name := st.Normalized(now)
		if name != st.State {
			st = s.effectiveState(ctx, id)
		}
		out[id] = st
	}
	return out
}

func (s *PublicScheduleQualityService) startCooldown(ctx context.Context, accountID int64, resolved PublicScheduleQualityResolved, reason string, now time.Time) {
	if s == nil || s.cache == nil {
		return
	}
	until := now.Add(time.Duration(resolved.CooldownMinutes) * time.Minute)
	if s.cache.TryStartCooldown(ctx, accountID, until, reason, resolved.SoftCooldown) {
		s.cache.ClearSoft(ctx, accountID)
		slog.Info("public_quality_cooldown_start",
			"account_id", accountID,
			"until", until,
			"reason", reason,
			"soft", resolved.SoftCooldown,
		)
	}
}

func (s *PublicScheduleQualityService) becomeSelectable(ctx context.Context, accountID int64) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.ClearState(ctx, accountID)
	s.cache.ClearSoft(ctx, accountID)
}

func (s *PublicScheduleQualityService) forceState(ctx context.Context, accountID int64, state *PublicScheduleRuntimeState) {
	if s == nil || s.cache == nil || state == nil {
		return
	}
	if state.State == PublicScheduleStateSelectable {
		s.becomeSelectable(ctx, accountID)
		return
	}
	if state.State != PublicScheduleStateCooling {
		s.cache.ClearSoft(ctx, accountID)
	}
	s.cache.SetState(ctx, accountID, state)
}

func (s *PublicScheduleQualityService) cacheWindow(ctx context.Context, accountID int64) *PairQualityLive {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.GetWindow(ctx, accountID)
}

func (s *PublicScheduleQualityService) storeWindow(ctx context.Context, accountID int64, live *PairQualityLive) {
	if s == nil || s.cache == nil || live == nil {
		return
	}
	s.cache.StoreWindow(ctx, accountID, live)
}

func (s *PublicScheduleQualityService) cacheSoft(ctx context.Context, accountID int64) *PairQualityLive {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.GetSoft(ctx, accountID)
}

func (s *PublicScheduleQualityService) storeSoft(ctx context.Context, accountID int64, live *PairQualityLive) {
	if s == nil || s.cache == nil || live == nil {
		return
	}
	s.cache.StoreSoft(ctx, accountID, live)
}

func (s *SettingService) GetPublicScheduleQualitySettings(ctx context.Context) (*PublicScheduleQualitySettings, error) {
	if s == nil || s.settingRepo == nil {
		return DefaultPublicScheduleQualitySettings(), nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPublicScheduleQualitySettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultPublicScheduleQualitySettings(), nil
		}
		return nil, fmt.Errorf("get public schedule quality settings: %w", err)
	}
	if value == "" {
		return DefaultPublicScheduleQualitySettings(), nil
	}
	var settings PublicScheduleQualitySettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultPublicScheduleQualitySettings(), nil
	}
	NormalizePublicScheduleQualitySettings(&settings)
	return &settings, nil
}

func (s *SettingService) SetPublicScheduleQualitySettings(ctx context.Context, settings *PublicScheduleQualitySettings) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("settings service not ready")
	}
	if err := ValidatePublicScheduleQualitySettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal public schedule quality settings: %w", err)
	}
	return s.settingRepo.Set(ctx, SettingKeyPublicScheduleQualitySettings, string(data))
}
