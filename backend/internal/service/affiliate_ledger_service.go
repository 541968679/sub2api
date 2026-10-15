package service

import (
	"context"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// InviteEnabled reports the raw admin grant. A missing profile is not enabled.
// It does not create a profile.
func (s *AffiliateService) InviteEnabled(ctx context.Context, userID int64) bool {
	if s == nil || s.repo == nil || userID <= 0 {
		return false
	}
	summary, err := s.repo.GetAffiliateByUserID(ctx, userID)
	if err != nil || summary == nil {
		return false
	}
	return summary.AffInviteEnabled
}

// CanInvite is the global switch AND the per-user grant.
func (s *AffiliateService) CanInvite(ctx context.Context, userID int64) bool {
	return s.IsEnabled(ctx) && s.InviteEnabled(ctx, userID)
}

// AccrueForCredit writes at most one rebate for a recharge fact.
// A disabled inviter, an exhausted cap, or a repeated source returns 0.
func (s *AffiliateService) AccrueForCredit(ctx context.Context, inviteeUserID int64, credited float64, sourceKind string, sourceID int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || sourceID <= 0 || strings.TrimSpace(sourceKind) == "" {
		return 0, nil
	}
	if credited <= 0 || math.IsNaN(credited) || math.IsInf(credited, 0) {
		return 0, nil
	}
	if !s.IsEnabled(ctx) {
		return 0, nil
	}
	invitee, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if invitee == nil || invitee.InviterID == nil || *invitee.InviterID <= 0 || *invitee.InviterID == inviteeUserID {
		return 0, nil
	}
	rate, cap, freeze, duration := s.accrualGlobals(ctx)
	return s.repo.ApplySourcedAccrual(ctx, AffiliateAccrualInput{
		InviterID:          *invitee.InviterID,
		InviteeUserID:      inviteeUserID,
		InviteeCreatedAt:   invitee.CreatedAt,
		Credited:           credited,
		SourceKind:         sourceKind,
		SourceID:           sourceID,
		GlobalRate:         rate,
		GlobalCap:          cap,
		GlobalFreezeHours:  freeze,
		GlobalDurationDays: duration,
		Now:                time.Now(),
		MasterCutMax:       s.masterCutMax(ctx),
	})
}

func (s *AffiliateService) accrualGlobals(ctx context.Context) (rate, cap float64, freeze, duration int) {
	if s == nil || s.settingService == nil {
		return AffiliateRebateRateDefault, AffiliateRebatePerInviteeCapDefault, 0, AffiliateRebateDurationDaysDefault
	}
	return s.settingService.GetAffiliateRebateRatePercent(ctx),
		s.settingService.GetAffiliateRebatePerInviteeCap(ctx),
		s.settingService.GetAffiliateRebateFreezeHours(ctx),
		s.settingService.GetAffiliateRebateDurationDays(ctx)
}

// ReverseForPaymentRefund revokes the rebate of one payment order.
// It still runs after the inviter's permission has been turned off.
// ErrAffiliateReverseAlreadyApplied means this reverse_seq is already stored.
func (s *AffiliateService) ReverseForPaymentRefund(ctx context.Context, in AffiliatePaymentReverseInput) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if in.InviteeUserID <= 0 || in.OrderID <= 0 || in.ReverseSeq <= 0 {
		return 0, nil
	}
	if in.RefundedCredited <= 0 || in.OriginalCredited <= 0 {
		return 0, nil
	}
	return s.repo.ReversePaymentAccrual(ctx, in)
}

// ReverseForAdminDebit revokes the oldest unreversed accruals for this invitee
// until the balance decrease is consumed or the net rebate is gone.
func (s *AffiliateService) ReverseForAdminDebit(ctx context.Context, in AffiliateAdminDebitInput) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if in.InviteeUserID <= 0 || in.ReverseSeq <= 0 || in.Debit <= 0 || math.IsNaN(in.Debit) || math.IsInf(in.Debit, 0) {
		return 0, nil
	}
	return s.repo.ReverseAdminDebit(ctx, in)
}

// AdminSettle records an offline payout against settleable quota.
func (s *AffiliateService) AdminSettle(ctx context.Context, userID int64, amount float64, note string) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return infraerrors.BadRequest("AFFILIATE_SETTLE_NOTE_REQUIRED", "settlement note is required")
	}
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return infraerrors.BadRequest("AFFILIATE_SETTLE_AMOUNT_INVALID", "settlement amount must be greater than 0")
	}
	return s.repo.SettleQuota(ctx, userID, roundTo(amount, 8), note)
}

func (s *AffiliateService) AdminSetInviteEnabled(ctx context.Context, userID int64, enabled bool) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.UpdateAffiliatePolicy(ctx, userID, AffiliatePolicyPatch{SetInvite: &enabled})
}

func (s *AffiliateService) AdminSetPerInviteeCap(ctx context.Context, userID int64, cap *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateOptionalCap(cap); err != nil {
		return err
	}
	patch := AffiliatePolicyPatch{}
	if cap == nil {
		patch.ClearCap = true
	} else {
		v := roundTo(*cap, 8)
		patch.SetCap = &v
	}
	return s.repo.UpdateAffiliatePolicy(ctx, userID, patch)
}

func (s *AffiliateService) AdminSetFreezeHours(ctx context.Context, userID int64, hours *int) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateOptionalFreeze(hours); err != nil {
		return err
	}
	patch := AffiliatePolicyPatch{}
	if hours == nil {
		patch.ClearFreeze = true
	} else {
		patch.SetFreeze = hours
	}
	return s.repo.UpdateAffiliatePolicy(ctx, userID, patch)
}

func (s *AffiliateService) AdminSetDurationDays(ctx context.Context, userID int64, days *int) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateOptionalDuration(days); err != nil {
		return err
	}
	patch := AffiliatePolicyPatch{}
	if days == nil {
		patch.ClearDuration = true
	} else {
		patch.SetDuration = days
	}
	return s.repo.UpdateAffiliatePolicy(ctx, userID, patch)
}

func (s *AffiliateService) AdminBatchSetInviteEnabled(ctx context.Context, userIDs []int64, enabled bool) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	cleaned := make([]int64, 0, len(userIDs))
	seen := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetInviteEnabled(ctx, cleaned, enabled)
}

// AdminClearUserCustomSettings removes per-user overrides and the invite grant,
// then resets the invite code so the user leaves the custom list.
func (s *AffiliateService) AdminClearUserCustomSettings(ctx context.Context, userID int64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	disabled := false
	if err := s.repo.UpdateAffiliatePolicy(ctx, userID, AffiliatePolicyPatch{
		SetInvite:     &disabled,
		ClearRate:     true,
		ClearCap:      true,
		ClearFreeze:   true,
		ClearDuration: true,
		SetMaster:     &disabled,
	}); err != nil {
		return err
	}
	_, err := s.repo.ResetUserAffCode(ctx, userID)
	return err
}

func (s *AffiliateService) AdminSetMaster(ctx context.Context, userID int64, enabled bool) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.SetMaster(ctx, userID, enabled)
}

// SetOwnMasterCut stores the master's cut for future accruals.
// The value is clamped to the current admin ceiling. Zero means no cut.
func (s *AffiliateService) SetOwnMasterCut(ctx context.Context, userID int64, percent float64) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 {
		return ErrAffiliateMasterCutInvalid
	}
	max := s.masterCutMax(ctx)
	if percent > max {
		return ErrAffiliateMasterCutInvalid
	}
	return s.repo.SetMasterCut(ctx, userID, roundTo(percent, 8))
}

func validateOptionalCap(cap *float64) error {
	if cap == nil {
		return nil
	}
	v := *cap
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return infraerrors.BadRequest("INVALID_CAP", "invalid per-invitee rebate cap")
	}
	return nil
}

func validateOptionalFreeze(hours *int) error {
	if hours == nil {
		return nil
	}
	if *hours < 0 || *hours > AffiliateRebateFreezeHoursMax {
		return infraerrors.BadRequest("INVALID_FREEZE_HOURS", "freeze hours out of range")
	}
	return nil
}

func validateOptionalDuration(days *int) error {
	if days == nil {
		return nil
	}
	if *days < 0 || *days > AffiliateRebateDurationDaysMax {
		return infraerrors.BadRequest("INVALID_DURATION_DAYS", "duration days out of range")
	}
	return nil
}
