//go:build unit

package service

import (
	"math"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestProportionalRebateReversal(t *testing.T) {
	t.Parallel()
	require.InDelta(t, 10, ProportionalRebateReversal(10, 100, 100, 0), 1e-9)
	require.InDelta(t, 4, ProportionalRebateReversal(10, 40, 100, 0), 1e-9)
	require.InDelta(t, 3, ProportionalRebateReversal(10, 100, 100, 7), 1e-9)
	require.Zero(t, ProportionalRebateReversal(0, 100, 100, 0))
	require.Zero(t, ProportionalRebateReversal(10, 0, 100, 0))
	require.Zero(t, ProportionalRebateReversal(10, 50, 0, 0))
	require.Zero(t, ProportionalRebateReversal(10, 100, 100, 10))
	require.Zero(t, ProportionalRebateReversal(math.NaN(), 100, 100, 0))
}

func TestTruncateRebateToCap(t *testing.T) {
	t.Parallel()
	require.InDelta(t, 12, TruncateRebateToCap(12, 100, 0), 1e-9)
	require.InDelta(t, 12, TruncateRebateToCap(12, 100, -1), 1e-9)
	require.InDelta(t, 4, TruncateRebateToCap(12, 6, 10), 1e-9)
	require.Zero(t, TruncateRebateToCap(12, 10, 10))
	require.Zero(t, TruncateRebateToCap(0, 0, 10))
}

func TestAffiliateDurationAndExplicitZeroOverrides(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := created.AddDate(0, 0, 10)

	require.False(t, AffiliateDurationExpired(created, 0, later))
	require.False(t, AffiliateDurationExpired(time.Time{}, 30, later))
	require.False(t, AffiliateDurationExpired(created, 30, created.AddDate(0, 0, 10)))
	require.True(t, AffiliateDurationExpired(created, 7, later))

	zero := 0.0
	zeroHours := 0
	require.InDelta(t, 0, ResolveNullableFloat(&zero, 20), 1e-9)
	require.InDelta(t, 20, ResolveNullableFloat(nil, 20), 1e-9)
	require.Equal(t, 0, ResolveNullableInt(&zeroHours, 24))
	require.Equal(t, 24, ResolveNullableInt(nil, 24))
	require.InDelta(t, 0, ResolveAffiliateRate(&zero, 20), 1e-9)
	require.InDelta(t, 20, ResolveAffiliateRate(nil, 20), 1e-9)

	in := AffiliateAccrualInput{
		Credited:           100,
		InviteeCreatedAt:   created,
		GlobalRate:         20,
		GlobalCap:          0,
		GlobalFreezeHours:  24,
		GlobalDurationDays: 0,
		Now:                later,
	}
	amount, freeze, ok := AccrualAmount(in, LockedAffiliatePolicy{InviteEnabled: true, Rate: &zero, FreezeHours: &zeroHours}, 0)
	require.False(t, ok)
	require.Zero(t, amount)
	require.Zero(t, freeze)

	amount, freeze, ok = AccrualAmount(in, LockedAffiliatePolicy{InviteEnabled: true, FreezeHours: &zeroHours}, 0)
	require.True(t, ok)
	require.InDelta(t, 20, amount, 1e-9)
	require.Zero(t, freeze)

	amount, freeze, ok = AccrualAmount(in, LockedAffiliatePolicy{InviteEnabled: true}, 0)
	require.True(t, ok)
	require.InDelta(t, 20, amount, 1e-9)
	require.Equal(t, 24, freeze)

	expired := in
	expired.GlobalDurationDays = 1
	_, _, ok = AccrualAmount(expired, LockedAffiliatePolicy{InviteEnabled: true}, 0)
	require.False(t, ok)

	_, _, ok = AccrualAmount(in, LockedAffiliatePolicy{InviteEnabled: false}, 0)
	require.False(t, ok)
}

func TestAffiliateRefundReverseSeq(t *testing.T) {
	t.Parallel()
	require.Equal(t, int64(150000000), AffiliateRefundReverseSeq(1.5))
	require.Equal(t, int64(1), AffiliateRefundReverseSeq(0))
	require.Equal(t, int64(1), AffiliateRefundReverseSeq(math.NaN()))
}

func TestTransferAffiliateQuotaDisabledWithoutRepo(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	transferred, balance, err := svc.TransferAffiliateQuota(t.Context(), 1)
	require.Zero(t, transferred)
	require.Zero(t, balance)
	require.ErrorIs(t, err, ErrAffiliateTransferDisabled)
	require.Equal(t, "AFFILIATE_TRANSFER_DISABLED", infraerrors.Reason(err))
	require.Nil(t, svc.repo)
}

func TestSplitRebateSumsToGross(t *testing.T) {
	t.Parallel()
	master, sales := SplitRebate(12.34567891, 30)
	require.InDelta(t, 12.34567891, master+sales, 1e-8)
	require.InDelta(t, roundTo(12.34567891*0.30, 8), master, 1e-9)

	master, sales = SplitRebate(10, 0)
	require.Zero(t, master)
	require.InDelta(t, 10, sales, 1e-9)

	master, sales = SplitRebate(10, 100)
	require.InDelta(t, 8, master, 1e-9)
	require.InDelta(t, 2, sales, 1e-9)
}

func TestPlanRebateShares(t *testing.T) {
	t.Parallel()
	stored := 40.0
	shares := PlanRebateShares(10, 2, 9, true, &stored, 50)
	require.Len(t, shares, 2)
	require.Equal(t, AffiliateAccrualRoleDirect, shares[0].Role)
	require.Equal(t, int64(2), shares[0].PayeeID)
	require.Equal(t, AffiliateAccrualRoleMasterCut, shares[1].Role)
	require.Equal(t, int64(9), shares[1].PayeeID)
	sum := 0.0
	for _, share := range shares {
		sum += share.Amount
		require.Equal(t, int64(2), share.DirectInviterID)
	}
	require.InDelta(t, 10, sum, 1e-9)
	require.InDelta(t, 4, shares[1].Amount, 1e-9)
	require.InDelta(t, 6, shares[0].Amount, 1e-9)

	capped := PlanRebateShares(10, 2, 9, true, &stored, 25)
	require.InDelta(t, 2.5, capped[1].Amount, 1e-9)

	whole := PlanRebateShares(10, 2, 2, true, &stored, 50)
	require.Len(t, whole, 1)
	require.InDelta(t, 10, whole[0].Amount, 1e-9)
	require.Nil(t, whole[0].CutPercent)

	disabled := PlanRebateShares(10, 2, 9, false, &stored, 50)
	require.Len(t, disabled, 1)
	require.InDelta(t, 10, disabled[0].Amount, 1e-9)

	zeroCut := 0.0
	noCut := PlanRebateShares(10, 2, 9, true, &zeroCut, 50)
	require.Len(t, noCut, 1)
	require.InDelta(t, 10, noCut[0].Amount, 1e-9)
}

func TestDistributeRebateTakeLastRowGetsRemainder(t *testing.T) {
	t.Parallel()
	got := DistributeRebateTake([]float64{6, 4}, 5)
	require.InDelta(t, 3, got[0], 1e-9)
	require.InDelta(t, 2, got[1], 1e-9)
	require.InDelta(t, 5, got[0]+got[1], 1e-9)

	thirds := DistributeRebateTake([]float64{1, 1, 1}, 1)
	require.InDelta(t, 1, thirds[0]+thirds[1]+thirds[2], 1e-9)
	require.LessOrEqual(t, thirds[0], 1.0)
	require.LessOrEqual(t, thirds[1], 1.0)
	require.LessOrEqual(t, thirds[2], 1.0)
}

func TestRejectMixedAffiliateCodes(t *testing.T) {
	t.Parallel()
	err := RejectMixedAffiliateCodes("ABCDE", "SALES")
	require.ErrorIs(t, err, ErrAffiliateCodeConflict)
	require.Equal(t, "AFFILIATE_CODE_CONFLICT", infraerrors.Reason(err))
	require.NoError(t, RejectMixedAffiliateCodes("ABCDE", ""))
	require.NoError(t, RejectMixedAffiliateCodes("", "SALES"))
	require.NoError(t, RejectMixedAffiliateCodes("  ", " "))
}
