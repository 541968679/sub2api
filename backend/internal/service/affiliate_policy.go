package service

import (
	"math"
	"time"
)

const (
	AffiliateSourcePaymentOrder = "payment_order"
	AffiliateSourceRedeemCode   = "redeem_code"
	AffiliateSourceAdminBalance = "admin_balance"
)

// AffiliateAccrualInput is the recharge fact plus the global rules in force
// when the accrual starts. Per-inviter overrides are read under the row lock.
type AffiliateAccrualInput struct {
	InviterID          int64
	InviteeUserID      int64
	InviteeCreatedAt   time.Time
	Credited           float64
	SourceKind         string
	SourceID           int64
	GlobalRate         float64
	GlobalCap          float64
	GlobalFreezeHours  int
	GlobalDurationDays int
	Now                time.Time
	// MasterCutMax is the admin ceiling in force for this accrual.
	// The stored master rate is clamped to it inside the row lock.
	MasterCutMax float64
}

// LockedAffiliatePolicy is the inviter row as read under FOR UPDATE.
type LockedAffiliatePolicy struct {
	InviteEnabled bool
	Rate          *float64
	Cap           *float64
	FreezeHours   *int
	DurationDays  *int
}

// AffiliatePaymentReverseInput reverses the rebate of one balance order.
// RefundedCredited and OriginalCredited are balance units, not payment currency.
type AffiliatePaymentReverseInput struct {
	InviteeUserID    int64
	OrderID          int64
	RefundedCredited float64
	OriginalCredited float64
	ReverseSeq       int64
}

// AffiliateAdminDebitInput reverses rebate FIFO for one admin balance decrease.
// ReverseSeq is the adjustment redeem record id.
type AffiliateAdminDebitInput struct {
	InviteeUserID int64
	Debit         float64
	ReverseSeq    int64
}

// AffiliatePolicyPatch updates only the fields the admin sent.
// A clear flag writes NULL. A nil pointer without the clear flag leaves the column alone.
type AffiliatePolicyPatch struct {
	SetInvite     *bool
	SetRate       *float64
	ClearRate     bool
	SetCap        *float64
	ClearCap      bool
	SetFreeze     *int
	ClearFreeze   bool
	SetDuration   *int
	ClearDuration bool
	SetMaster     *bool
}

func (p AffiliatePolicyPatch) Empty() bool {
	return p.SetInvite == nil && p.SetRate == nil && !p.ClearRate &&
		p.SetCap == nil && !p.ClearCap &&
		p.SetFreeze == nil && !p.ClearFreeze &&
		p.SetDuration == nil && !p.ClearDuration &&
		p.SetMaster == nil
}

// ProportionalRebateReversal returns the rebate to revoke for one refund.
// The result is capped by the original accrual minus reverses already stored.
func ProportionalRebateReversal(originalRebate, refundedCredited, originalCredited, alreadyReversed float64) float64 {
	if originalRebate <= 0 || refundedCredited <= 0 || originalCredited <= 0 {
		return 0
	}
	if math.IsNaN(originalRebate) || math.IsNaN(refundedCredited) || math.IsNaN(originalCredited) || math.IsNaN(alreadyReversed) {
		return 0
	}
	if math.IsInf(originalRebate, 0) || math.IsInf(refundedCredited, 0) || math.IsInf(originalCredited, 0) {
		return 0
	}
	if alreadyReversed < 0 || math.IsNaN(alreadyReversed) {
		alreadyReversed = 0
	}
	remaining := roundTo(originalRebate-alreadyReversed, 8)
	if remaining <= 0 {
		return 0
	}
	requested := roundTo(originalRebate*(refundedCredited/originalCredited), 8)
	if requested > remaining {
		requested = remaining
	}
	if requested <= 0 {
		return 0
	}
	return requested
}

// TruncateRebateToCap limits one accrual to the invitee's remaining cap.
// cap <= 0 means unlimited, including an explicit per-inviter 0.
func TruncateRebateToCap(rebate, existingNet, cap float64) float64 {
	if rebate <= 0 || math.IsNaN(rebate) || math.IsInf(rebate, 0) {
		return 0
	}
	if cap <= 0 || math.IsNaN(cap) || math.IsInf(cap, 0) {
		return rebate
	}
	if existingNet >= cap {
		return 0
	}
	remaining := cap - existingNet
	if rebate > remaining {
		return roundTo(remaining, 8)
	}
	return rebate
}

// ResolveNullableFloat returns the override when it is set, otherwise global.
func ResolveNullableFloat(override *float64, global float64) float64 {
	if override != nil {
		return *override
	}
	return global
}

// ResolveNullableInt returns the override when it is set, otherwise global.
func ResolveNullableInt(override *int, global int) int {
	if override != nil {
		return *override
	}
	return global
}

// ResolveAffiliateRate applies the per-inviter rate when set.
// A non-finite override falls back to the global rate. Both are clamped.
func ResolveAffiliateRate(override *float64, global float64) float64 {
	if override != nil {
		v := *override
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return clampAffiliateRebateRate(v)
		}
	}
	return clampAffiliateRebateRate(global)
}

// AffiliateDurationExpired reports whether rebate for this invitee has ended.
// durationDays <= 0 means the relationship never expires.
func AffiliateDurationExpired(createdAt time.Time, durationDays int, now time.Time) bool {
	if durationDays <= 0 || createdAt.IsZero() {
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	return now.After(createdAt.AddDate(0, 0, durationDays))
}

// AccrualAmount decides the rebate to write while the inviter row is locked.
// ok is false when nothing should be inserted.
func AccrualAmount(in AffiliateAccrualInput, locked LockedAffiliatePolicy, existingNet float64) (amount float64, freezeHours int, ok bool) {
	if !locked.InviteEnabled || in.Credited <= 0 {
		return 0, 0, false
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	duration := ResolveNullableInt(locked.DurationDays, in.GlobalDurationDays)
	if AffiliateDurationExpired(in.InviteeCreatedAt, duration, now) {
		return 0, 0, false
	}
	rate := ResolveAffiliateRate(locked.Rate, in.GlobalRate)
	rebate := roundTo(in.Credited*(rate/100), 8)
	if rebate <= 0 {
		return 0, 0, false
	}
	cap := ResolveNullableFloat(locked.Cap, in.GlobalCap)
	rebate = TruncateRebateToCap(rebate, existingNet, cap)
	if rebate <= 0 {
		return 0, 0, false
	}
	freezeHours = ResolveNullableInt(locked.FreezeHours, in.GlobalFreezeHours)
	if freezeHours < 0 {
		freezeHours = 0
	}
	return rebate, freezeHours, true
}

// SplitAffiliateDebit takes amount from frozen quota first when preferFrozen
// is set, then from settleable quota. Settleable quota may become negative.
func SplitAffiliateDebit(quota, frozen, amount float64, preferFrozen bool) (float64, float64) {
	if amount <= 0 {
		return quota, frozen
	}
	fromFrozen := 0.0
	if preferFrozen && frozen > 0 {
		fromFrozen = amount
		if fromFrozen > frozen {
			fromFrozen = frozen
		}
	}
	return quota - (amount - fromFrozen), frozen - fromFrozen
}

// AffiliateRefundReverseSeq is stable for the same credited refund amount.
func AffiliateRefundReverseSeq(refundedCredited float64) int64 {
	if refundedCredited <= 0 || math.IsNaN(refundedCredited) || math.IsInf(refundedCredited, 0) {
		return 1
	}
	seq := int64(math.Round(refundedCredited * 1e8))
	if seq < 1 {
		return 1
	}
	return seq
}

// RebateShare is one payee of a single recharge rebate.
// The shares of one event sum to the gross rebate.
type RebateShare struct {
	PayeeID         int64
	Role            string
	Amount          float64
	CutPercent      *float64
	DirectInviterID int64
}

// ClampAffiliateMasterCutMax bounds the admin ceiling to [0, 80].
func ClampAffiliateMasterCutMax(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return AffiliateMasterCutMaxDefault
	}
	if v > AffiliateMasterCutAbsoluteMax {
		return AffiliateMasterCutAbsoluteMax
	}
	return v
}

// EffectiveMasterCut is the cut applied to future accruals.
// A nil or non-positive stored rate means the master has not taken a cut.
// The result never exceeds the current admin ceiling or the absolute max.
func EffectiveMasterCut(stored *float64, ceiling float64) float64 {
	if stored == nil {
		return 0
	}
	v := *stored
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	max := ClampAffiliateMasterCutMax(ceiling)
	if v > max {
		v = max
	}
	return roundTo(v, 8)
}

// SplitRebate cuts gross into the master share and the salesperson share.
// The master share is rounded first. The salesperson receives the remainder,
// so the two amounts sum to gross.
func SplitRebate(gross, cutPercent float64) (master, sales float64) {
	if gross <= 0 || math.IsNaN(gross) || math.IsInf(gross, 0) {
		return 0, 0
	}
	gross = roundTo(gross, 8)
	if cutPercent <= 0 || math.IsNaN(cutPercent) || math.IsInf(cutPercent, 0) {
		return 0, gross
	}
	if cutPercent > AffiliateMasterCutAbsoluteMax {
		cutPercent = AffiliateMasterCutAbsoluteMax
	}
	master = roundTo(gross*cutPercent/100, 8)
	if master < 0 {
		master = 0
	}
	if master > gross {
		master = gross
	}
	sales = roundTo(gross-master, 8)
	if sales < 0 {
		sales = 0
	}
	return master, sales
}

// PlanRebateShares decides who is paid for one already-capped gross rebate.
// A master who is the direct inviter is not cut. A disabled master, a zero
// cut, or a missing parent leaves the whole amount on the direct row.
// A zero share is omitted. The caller still records the source once any
// positive share is inserted.
func PlanRebateShares(gross float64, inviterID, masterID int64, masterActive bool, storedCut *float64, cutMax float64) []RebateShare {
	if gross <= 0 || inviterID <= 0 || math.IsNaN(gross) || math.IsInf(gross, 0) {
		return nil
	}
	whole := RebateShare{
		PayeeID:         inviterID,
		Role:            AffiliateAccrualRoleDirect,
		Amount:          roundTo(gross, 8),
		DirectInviterID: inviterID,
	}
	if masterID <= 0 || masterID == inviterID || !masterActive {
		return []RebateShare{whole}
	}
	cut := EffectiveMasterCut(storedCut, cutMax)
	if cut <= 0 {
		return []RebateShare{whole}
	}
	masterAmt, salesAmt := SplitRebate(gross, cut)
	cutCopy := cut
	out := make([]RebateShare, 0, 2)
	if salesAmt > 0 {
		out = append(out, RebateShare{
			PayeeID:         inviterID,
			Role:            AffiliateAccrualRoleDirect,
			Amount:          salesAmt,
			CutPercent:      &cutCopy,
			DirectInviterID: inviterID,
		})
	}
	if masterAmt > 0 {
		out = append(out, RebateShare{
			PayeeID:         masterID,
			Role:            AffiliateAccrualRoleMasterCut,
			Amount:          masterAmt,
			CutPercent:      &cutCopy,
			DirectInviterID: inviterID,
		})
	}
	return out
}

// DistributeRebateTake splits one debit across the remaining amounts of one
// recharge. Earlier rows take a rounded proportion. The last positive row
// receives the remainder, and any leftover that does not fit is pushed back
// onto earlier rows that still have room.
func DistributeRebateTake(remainings []float64, take float64) []float64 {
	out := make([]float64, len(remainings))
	if take <= 0 || len(remainings) == 0 || math.IsNaN(take) || math.IsInf(take, 0) {
		return out
	}
	total := 0.0
	lastPositive := -1
	for i, v := range remainings {
		if v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) {
			total += v
			lastPositive = i
		}
	}
	total = roundTo(total, 8)
	if total <= 0 || lastPositive < 0 {
		return out
	}
	if take > total {
		take = total
	}
	take = roundTo(take, 8)
	left := take
	for i, rem := range remainings {
		if rem <= 0 || left <= 0 || math.IsNaN(rem) || math.IsInf(rem, 0) {
			continue
		}
		share := left
		if i != lastPositive {
			share = roundTo(take*rem/total, 8)
		}
		if share > rem {
			share = rem
		}
		if share > left {
			share = left
		}
		share = roundTo(share, 8)
		if share < 0 {
			share = 0
		}
		out[i] = share
		left = roundTo(left-share, 8)
	}
	if left > 0 {
		for i := range out {
			spare := roundTo(remainings[i]-out[i], 8)
			if spare <= 0 || left <= 0 {
				continue
			}
			add := spare
			if add > left {
				add = left
			}
			out[i] = roundTo(out[i]+add, 8)
			left = roundTo(left-add, 8)
		}
	}
	return out
}
