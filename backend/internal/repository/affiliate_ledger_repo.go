package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

var errAffiliateAccrualConflict = errors.New("affiliate accrual conflict")

func (r *affiliateRepository) GetAffiliateByUserID(ctx context.Context, userID int64) (*service.AffiliateSummary, error) {
	if userID <= 0 {
		return nil, service.ErrAffiliateProfileNotFound
	}
	client := clientFromContext(ctx, r.client)
	return queryAffiliateByUserID(ctx, client, userID)
}

func (r *affiliateRepository) ApplySourcedAccrual(ctx context.Context, in service.AffiliateAccrualInput) (float64, error) {
	if in.InviterID <= 0 || in.InviteeUserID <= 0 || in.SourceID <= 0 || strings.TrimSpace(in.SourceKind) == "" || in.Credited <= 0 {
		return 0, nil
	}
	var applied float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		parentID, err := peekSalesParentID(txCtx, txClient, in.InviterID)
		if err != nil {
			if errors.Is(err, service.ErrAffiliateProfileNotFound) {
				return nil
			}
			return err
		}
		locked, err := lockAffiliatePayees(txCtx, txClient, in.InviterID, parentID)
		if err != nil {
			if errors.Is(err, service.ErrAffiliateProfileNotFound) {
				return nil
			}
			return err
		}
		inviter, ok := locked[in.InviterID]
		if !ok {
			return nil
		}
		exists, err := affiliateAccrualSourceExists(txCtx, txClient, in.SourceKind, in.SourceID)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		net, err := affiliateInviteeNetRebate(txCtx, txClient, in.InviterID, in.InviteeUserID)
		if err != nil {
			return err
		}
		amount, freezeHours, ok := service.AccrualAmount(in, inviter.policy(), net)
		if !ok || amount <= 0 {
			return nil
		}
		masterID := int64(0)
		if inviter.SalesParentID != nil {
			masterID = *inviter.SalesParentID
		}
		var storedCut *float64
		masterActive := false
		if master, found := locked[masterID]; found && masterID != in.InviterID {
			masterActive = master.IsMaster
			storedCut = master.MasterCut
		}
		shares := service.PlanRebateShares(amount, in.InviterID, masterID, masterActive, storedCut, in.MasterCutMax)
		if len(shares) == 0 {
			return nil
		}
		for _, share := range shares {
			if err := addAffiliateAccrualBalances(txCtx, txClient, share.PayeeID, share.Amount, freezeHours); err != nil {
				return err
			}
		}
		for _, share := range shares {
			if err := insertAffiliateAccrual(txCtx, txClient, affiliateAccrualWrite{
				UserID:          share.PayeeID,
				InviteeUserID:   in.InviteeUserID,
				Amount:          share.Amount,
				SourceKind:      in.SourceKind,
				SourceID:        in.SourceID,
				FreezeHours:     freezeHours,
				Role:            share.Role,
				CutPercent:      share.CutPercent,
				DirectInviterID: share.DirectInviterID,
			}); err != nil {
				if isAffiliateUniqueViolation(err) {
					return errAffiliateAccrualConflict
				}
				return err
			}
		}
		applied = amount
		return nil
	})
	if errors.Is(err, errAffiliateAccrualConflict) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return applied, nil
}

func (r *affiliateRepository) ReversePaymentAccrual(ctx context.Context, in service.AffiliatePaymentReverseInput) (float64, error) {
	if in.InviteeUserID <= 0 || in.OrderID <= 0 || in.ReverseSeq <= 0 {
		return 0, nil
	}
	var reversed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		payees, err := listPaymentAccrualPayees(txCtx, txClient, in.InviteeUserID, in.OrderID)
		if err != nil {
			return err
		}
		if len(payees) == 0 {
			return nil
		}
		locked, err := lockAffiliatePayees(txCtx, txClient, payees...)
		if err != nil {
			return err
		}
		accruals, err := lockPaymentAccruals(txCtx, txClient, in.InviteeUserID, in.OrderID)
		if err != nil {
			return err
		}
		if len(accruals) == 0 {
			return nil
		}
		running := make(map[int64]affiliateLockedRow, len(locked))
		for id, row := range locked {
			running[id] = row
		}
		now := time.Now()
		for _, accrual := range accruals {
			row, ok := running[accrual.InviterID]
			if !ok {
				return service.ErrAffiliateProfileNotFound
			}
			already, err := sumSourceReverses(txCtx, txClient, accrual.InviterID, service.AffiliateSourcePaymentOrder, in.OrderID)
			if err != nil {
				return err
			}
			amount := service.ProportionalRebateReversal(accrual.Amount, in.RefundedCredited, in.OriginalCredited, already)
			if amount <= 0 {
				continue
			}
			quota, frozen := service.SplitAffiliateDebit(row.Quota, row.Frozen, amount, accrual.stillFrozen(now))
			row.Quota, row.Frozen = quota, frozen
			running[accrual.InviterID] = row
			if err := insertAffiliateReverse(txCtx, txClient, affiliateReverseRow{
				InviterID:       accrual.InviterID,
				InviteeUserID:   in.InviteeUserID,
				Amount:          amount,
				SourceKind:      service.AffiliateSourcePaymentOrder,
				SourceID:        in.OrderID,
				ReverseSeq:      in.ReverseSeq,
				OrderID:         in.OrderID,
				QuotaAfter:      quota,
				FrozenAfter:     frozen,
				HistoryAfter:    row.History,
				Role:            accrual.reverseRole(),
				DirectInviterID: accrual.directInviter(),
			}); err != nil {
				return err
			}
			reversed += amount
		}
		for id, row := range running {
			original := locked[id]
			if row.Quota == original.Quota && row.Frozen == original.Frozen {
				continue
			}
			if err := setAffiliateBalances(txCtx, txClient, id, row.Quota, row.Frozen); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reversed, nil
}

func (r *affiliateRepository) ReverseAdminDebit(ctx context.Context, in service.AffiliateAdminDebitInput) (float64, error) {
	if in.InviteeUserID <= 0 || in.ReverseSeq <= 0 || in.Debit <= 0 {
		return 0, nil
	}
	var reversed float64
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		inviterID, found, err := affiliateInviterID(txCtx, txClient, in.InviteeUserID)
		if err != nil || !found || inviterID <= 0 {
			return err
		}
		payees, err := listInviteeAccrualPayees(txCtx, txClient, inviterID, in.InviteeUserID)
		if err != nil {
			return err
		}
		if len(payees) == 0 {
			payees = []int64{inviterID}
		}
		locked, err := lockAffiliatePayees(txCtx, txClient, payees...)
		if err != nil {
			if errors.Is(err, service.ErrAffiliateProfileNotFound) {
				return nil
			}
			return err
		}
		if _, ok := locked[inviterID]; !ok {
			return nil
		}
		accruals, err := lockInviteeAccruals(txCtx, txClient, inviterID, in.InviteeUserID)
		if err != nil {
			return err
		}
		groups := groupAccrualsBySource(accruals)
		running := make(map[int64]affiliateLockedRow, len(locked))
		for id, row := range locked {
			running[id] = row
		}
		left := in.Debit
		now := time.Now()
		type pendingReverse struct {
			accrual sourcedAccrual
			amount  float64
		}
		pending := make([]pendingReverse, 0)
		for _, group := range groups {
			if left <= 0 {
				break
			}
			remainings := make([]float64, len(group.rows))
			grossRemaining := 0.0
			for i, accrual := range group.rows {
				already, err := sumSourceReverses(txCtx, txClient, accrual.InviterID, accrual.SourceKind, accrual.SourceID)
				if err != nil {
					return err
				}
				remaining := accrual.Amount - already
				if remaining < 0 {
					remaining = 0
				}
				remainings[i] = remaining
				grossRemaining += remaining
			}
			if grossRemaining <= 0 {
				continue
			}
			take := grossRemaining
			if take > left {
				take = left
			}
			shares := service.DistributeRebateTake(remainings, take)
			consumed := 0.0
			for i, share := range shares {
				if share <= 0 {
					continue
				}
				pending = append(pending, pendingReverse{accrual: group.rows[i], amount: share})
				consumed += share
			}
			left -= consumed
			reversed += consumed
		}
		if len(pending) == 0 {
			reversed = 0
			return nil
		}
		for _, item := range pending {
			row, ok := running[item.accrual.InviterID]
			if !ok {
				return service.ErrAffiliateProfileNotFound
			}
			quota, frozen := service.SplitAffiliateDebit(row.Quota, row.Frozen, item.amount, item.accrual.stillFrozen(now))
			row.Quota, row.Frozen = quota, frozen
			running[item.accrual.InviterID] = row
			orderID := int64(0)
			if item.accrual.SourceKind == service.AffiliateSourcePaymentOrder {
				orderID = item.accrual.SourceID
			}
			if err := insertAffiliateReverse(txCtx, txClient, affiliateReverseRow{
				InviterID:       item.accrual.InviterID,
				InviteeUserID:   in.InviteeUserID,
				Amount:          item.amount,
				SourceKind:      item.accrual.SourceKind,
				SourceID:        item.accrual.SourceID,
				ReverseSeq:      in.ReverseSeq,
				OrderID:         orderID,
				QuotaAfter:      quota,
				FrozenAfter:     frozen,
				HistoryAfter:    row.History,
				Role:            item.accrual.reverseRole(),
				DirectInviterID: item.accrual.directInviter(),
			}); err != nil {
				return err
			}
		}
		for id, row := range running {
			original := locked[id]
			if row.Quota == original.Quota && row.Frozen == original.Frozen {
				continue
			}
			if err := setAffiliateBalances(txCtx, txClient, id, row.Quota, row.Frozen); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return reversed, nil
}

func (r *affiliateRepository) SettleQuota(ctx context.Context, userID int64, amount float64, note string) error {
	note = strings.TrimSpace(note)
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	if amount <= 0 || note == "" {
		return service.ErrAffiliateSettleRejected
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := thawFrozenQuotaTx(txCtx, txClient, userID); err != nil {
			return err
		}
		row, err := lockAffiliateRow(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		if row.Quota <= 0 || amount-row.Quota > 1e-8 {
			return service.ErrAffiliateSettleRejected
		}
		quota := row.Quota - amount
		if err := setAffiliateBalances(txCtx, txClient, userID, quota, row.Frozen); err != nil {
			return err
		}
		if _, err := txClient.ExecContext(txCtx, `
INSERT INTO user_affiliate_ledger (
    user_id, action, amount, note,
    aff_quota_after, aff_frozen_quota_after, aff_history_quota_after,
    created_at, updated_at
) VALUES ($1, 'settle', $2, $3, $4, $5, $6, NOW(), NOW())`,
			userID, amount, note, quota, row.Frozen, row.History); err != nil {
			return fmt.Errorf("insert affiliate settle ledger: %w", err)
		}
		return nil
	})
}

func (r *affiliateRepository) UpdateAffiliatePolicy(ctx context.Context, userID int64, patch service.AffiliatePolicyPatch) error {
	if userID <= 0 {
		return service.ErrUserNotFound
	}
	if patch.Empty() {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
			return err
		}
		sets := make([]string, 0, 5)
		args := make([]any, 0, 6)
		add := func(column string, value any) {
			args = append(args, value)
			sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
		}
		if patch.SetInvite != nil {
			add("aff_invite_enabled", *patch.SetInvite)
		}
		if patch.ClearRate {
			add("aff_rebate_rate_percent", nil)
		} else if patch.SetRate != nil {
			add("aff_rebate_rate_percent", *patch.SetRate)
		}
		if patch.ClearCap {
			add("aff_rebate_per_invitee_cap", nil)
		} else if patch.SetCap != nil {
			add("aff_rebate_per_invitee_cap", *patch.SetCap)
		}
		if patch.ClearFreeze {
			add("aff_rebate_freeze_hours", nil)
		} else if patch.SetFreeze != nil {
			add("aff_rebate_freeze_hours", *patch.SetFreeze)
		}
		if patch.ClearDuration {
			add("aff_rebate_duration_days", nil)
		} else if patch.SetDuration != nil {
			add("aff_rebate_duration_days", *patch.SetDuration)
		}
		if patch.SetMaster != nil {
			add("is_master", *patch.SetMaster)
		}
		if len(sets) == 0 {
			return nil
		}
		args = append(args, userID)
		query := "UPDATE user_affiliates SET " + strings.Join(sets, ", ") + fmt.Sprintf(", updated_at = NOW() WHERE user_id = $%d", len(args))
		res, err := txClient.ExecContext(txCtx, query, args...)
		if err != nil {
			return fmt.Errorf("update affiliate policy: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return service.ErrUserNotFound
		}
		return nil
	})
}

func (r *affiliateRepository) BatchSetInviteEnabled(ctx context.Context, userIDs []int64, enabled bool) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		for _, userID := range userIDs {
			if userID <= 0 {
				continue
			}
			if _, err := ensureUserAffiliateWithClient(txCtx, txClient, userID); err != nil {
				return err
			}
		}
		_, err := txClient.ExecContext(txCtx, `
UPDATE user_affiliates
SET aff_invite_enabled = $1, updated_at = NOW()
WHERE user_id = ANY($2)`, enabled, pq.Array(userIDs))
		if err != nil {
			return fmt.Errorf("batch set affiliate invite: %w", err)
		}
		return nil
	})
}

type affiliateLockedRow struct {
	InviteEnabled bool
	Rate          *float64
	Cap           *float64
	FreezeHours   *int
	DurationDays  *int
	Quota         float64
	Frozen        float64
	History       float64
	InviterID     *int64
	SalesParentID *int64
	SalesCode     string
	IsMaster      bool
	MasterCut     *float64
}

func (row affiliateLockedRow) policy() service.LockedAffiliatePolicy {
	return service.LockedAffiliatePolicy{
		InviteEnabled: row.InviteEnabled,
		Rate:          row.Rate,
		Cap:           row.Cap,
		FreezeHours:   row.FreezeHours,
		DurationDays:  row.DurationDays,
	}
}

func lockAffiliateRow(ctx context.Context, client affiliateQueryExecer, userID int64) (affiliateLockedRow, error) {
	rows, err := client.QueryContext(ctx, `
SELECT aff_invite_enabled,
       aff_rebate_rate_percent,
       aff_rebate_per_invitee_cap,
       aff_rebate_freeze_hours,
       aff_rebate_duration_days,
       aff_quota::double precision,
       aff_frozen_quota::double precision,
       aff_history_quota::double precision,
       inviter_id,
       sales_parent_id,
       COALESCE(sales_code, ''),
       is_master,
       master_cut_percent
FROM user_affiliates
WHERE user_id = $1
FOR UPDATE`, userID)
	if err != nil {
		return affiliateLockedRow{}, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return affiliateLockedRow{}, err
		}
		return affiliateLockedRow{}, service.ErrAffiliateProfileNotFound
	}
	var row affiliateLockedRow
	var rate, cap, masterCut sql.NullFloat64
	var freeze, duration sql.NullInt64
	var inviterID, salesParentID sql.NullInt64
	if err := rows.Scan(
		&row.InviteEnabled, &rate, &cap, &freeze, &duration,
		&row.Quota, &row.Frozen, &row.History,
		&inviterID, &salesParentID, &row.SalesCode, &row.IsMaster, &masterCut,
	); err != nil {
		return affiliateLockedRow{}, err
	}
	row.Rate = nullableFloat64Ptr(rate)
	row.Cap = nullableFloat64Ptr(cap)
	row.FreezeHours = nullInt64ToPtr(freeze)
	row.DurationDays = nullInt64ToPtr(duration)
	if inviterID.Valid && inviterID.Int64 > 0 {
		row.InviterID = &inviterID.Int64
	}
	if salesParentID.Valid && salesParentID.Int64 > 0 {
		row.SalesParentID = &salesParentID.Int64
	}
	row.MasterCut = nullableFloat64Ptr(masterCut)
	return row, rows.Err()
}

func peekSalesParentID(ctx context.Context, client affiliateQueryExecer, userID int64) (int64, error) {
	rows, err := client.QueryContext(ctx, `SELECT sales_parent_id FROM user_affiliates WHERE user_id = $1`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, service.ErrAffiliateProfileNotFound
	}
	var parent sql.NullInt64
	if err := rows.Scan(&parent); err != nil {
		return 0, err
	}
	if !parent.Valid || parent.Int64 <= 0 {
		return 0, rows.Err()
	}
	return parent.Int64, rows.Err()
}

// lockAffiliatePayees locks affiliate rows in ascending user id.
// The direct inviter must exist. A missing optional parent is skipped.
func lockAffiliatePayees(ctx context.Context, client affiliateQueryExecer, userIDs ...int64) (map[int64]affiliateLockedRow, error) {
	ids := uniquePositiveIDs(userIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make(map[int64]affiliateLockedRow, len(ids))
	for _, id := range ids {
		row, err := lockAffiliateRow(ctx, client, id)
		if err != nil {
			if errors.Is(err, service.ErrAffiliateProfileNotFound) {
				continue
			}
			return nil, err
		}
		out[id] = row
	}
	if len(ids) > 0 {
		if _, ok := out[ids[0]]; !ok && len(out) == 0 {
			return nil, service.ErrAffiliateProfileNotFound
		}
	}
	return out, nil
}

func nullInt64ToPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func affiliateAccrualSourceExists(ctx context.Context, client affiliateQueryExecer, sourceKind string, sourceID int64) (bool, error) {
	rows, err := client.QueryContext(ctx, `
SELECT 1
FROM user_affiliate_ledger
WHERE action = 'accrue'
  AND (
    (source_kind = $1 AND source_id = $2)
    OR ($1 = 'payment_order' AND source_order_id = $2)
  )
LIMIT 1`, sourceKind, sourceID)
	if err != nil {
		return false, fmt.Errorf("query affiliate accrual source: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return false, rows.Err()
	}
	return true, rows.Close()
}

func affiliateInviteeNetRebate(ctx context.Context, client affiliateQueryExecer, inviterID, inviteeUserID int64) (float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(CASE WHEN action = 'accrue' THEN amount WHEN action = 'reverse' THEN -amount ELSE 0 END), 0)::double precision
FROM user_affiliate_ledger
WHERE source_user_id = $2
  AND action IN ('accrue', 'reverse')
  AND (user_id = $1 OR direct_inviter_id = $1)`, inviterID, inviteeUserID)
	if err != nil {
		return 0, fmt.Errorf("query invitee net rebate: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var net float64
	if err := rows.Scan(&net); err != nil {
		return 0, err
	}
	return net, rows.Err()
}

func addAffiliateAccrualBalances(ctx context.Context, client affiliateQueryExecer, inviterID int64, amount float64, freezeHours int) error {
	var query string
	if freezeHours > 0 {
		query = `UPDATE user_affiliates SET aff_frozen_quota = aff_frozen_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2`
	} else {
		query = `UPDATE user_affiliates SET aff_quota = aff_quota + $1, aff_history_quota = aff_history_quota + $1, updated_at = NOW() WHERE user_id = $2`
	}
	res, err := client.ExecContext(ctx, query, amount, inviterID)
	if err != nil {
		return fmt.Errorf("add affiliate accrual balance: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return service.ErrAffiliateProfileNotFound
	}
	return nil
}

type affiliateAccrualWrite struct {
	UserID          int64
	InviteeUserID   int64
	Amount          float64
	SourceKind      string
	SourceID        int64
	FreezeHours     int
	Role            string
	CutPercent      *float64
	DirectInviterID int64
}

func insertAffiliateAccrual(ctx context.Context, client affiliateQueryExecer, in affiliateAccrualWrite) error {
	var sourceOrder any
	if in.SourceKind == service.AffiliateSourcePaymentOrder {
		sourceOrder = in.SourceID
	}
	var cut any
	if in.CutPercent != nil {
		cut = *in.CutPercent
	}
	var direct any
	if in.DirectInviterID > 0 {
		direct = in.DirectInviterID
	}
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = service.AffiliateAccrualRoleDirect
	}
	var err error
	if in.FreezeHours > 0 {
		_, err = client.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger (
    user_id, action, amount, source_user_id, source_order_id, source_kind, source_id,
    accrual_role, cut_percent_snapshot, direct_inviter_id,
    frozen_until, created_at, updated_at
) VALUES ($1, 'accrue', $2, $3, $4, $5, $6, $7, $8, $9, NOW() + make_interval(hours => $10), NOW(), NOW())`,
			in.UserID, in.Amount, in.InviteeUserID, sourceOrder, in.SourceKind, in.SourceID,
			role, cut, direct, in.FreezeHours)
	} else {
		_, err = client.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger (
    user_id, action, amount, source_user_id, source_order_id, source_kind, source_id,
    accrual_role, cut_percent_snapshot, direct_inviter_id,
    created_at, updated_at
) VALUES ($1, 'accrue', $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())`,
			in.UserID, in.Amount, in.InviteeUserID, sourceOrder, in.SourceKind, in.SourceID,
			role, cut, direct)
	}
	if err != nil {
		return fmt.Errorf("insert affiliate accrue ledger: %w", err)
	}
	return nil
}

type sourcedAccrual struct {
	InviterID       int64
	Amount          float64
	FrozenUtil      sql.NullTime
	SourceKind      string
	SourceID        int64
	Role            string
	DirectInviterID int64
	CreatedAt       time.Time
}

func (a sourcedAccrual) reverseRole() string {
	if strings.TrimSpace(a.Role) == "" {
		return service.AffiliateAccrualRoleDirect
	}
	return a.Role
}

func (a sourcedAccrual) directInviter() int64 {
	if a.DirectInviterID > 0 {
		return a.DirectInviterID
	}
	return a.InviterID
}

func (a sourcedAccrual) stillFrozen(now time.Time) bool {
	return a.FrozenUtil.Valid && a.FrozenUtil.Time.After(now)
}

func listPaymentAccrualPayees(ctx context.Context, client affiliateQueryExecer, inviteeUserID, orderID int64) ([]int64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT DISTINCT user_id
FROM user_affiliate_ledger
WHERE action = 'accrue'
  AND source_user_id = $1
  AND (
    (source_kind = 'payment_order' AND source_id = $2)
    OR (source_kind IS NULL AND source_order_id = $2)
  )`, inviteeUserID, orderID)
	if err != nil {
		return nil, fmt.Errorf("list payment accrual payees: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func lockPaymentAccruals(ctx context.Context, client affiliateQueryExecer, inviteeUserID, orderID int64) ([]sourcedAccrual, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       amount::double precision,
       frozen_until,
       COALESCE(source_kind, ''),
       COALESCE(source_id, 0),
       COALESCE(accrual_role, ''),
       COALESCE(direct_inviter_id, 0),
       created_at
FROM user_affiliate_ledger
WHERE action = 'accrue'
  AND source_user_id = $1
  AND (
    (source_kind = 'payment_order' AND source_id = $2)
    OR (source_kind IS NULL AND source_order_id = $2)
  )
ORDER BY id
FOR UPDATE`, inviteeUserID, orderID)
	if err != nil {
		return nil, fmt.Errorf("lock payment accruals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]sourcedAccrual, 0, 2)
	for rows.Next() {
		var accrual sourcedAccrual
		if err := rows.Scan(
			&accrual.InviterID, &accrual.Amount, &accrual.FrozenUtil,
			&accrual.SourceKind, &accrual.SourceID, &accrual.Role, &accrual.DirectInviterID, &accrual.CreatedAt,
		); err != nil {
			return nil, err
		}
		if accrual.SourceKind == "" {
			accrual.SourceKind = service.AffiliateSourcePaymentOrder
			accrual.SourceID = orderID
		}
		out = append(out, accrual)
	}
	return out, rows.Err()
}

func listInviteeAccrualPayees(ctx context.Context, client affiliateQueryExecer, inviterID, inviteeUserID int64) ([]int64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT DISTINCT user_id
FROM user_affiliate_ledger
WHERE source_user_id = $2
  AND action = 'accrue'
  AND source_kind IS NOT NULL
  AND source_id IS NOT NULL
  AND (user_id = $1 OR direct_inviter_id = $1)`, inviterID, inviteeUserID)
	if err != nil {
		return nil, fmt.Errorf("list invitee accrual payees: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]int64, 0, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func lockInviteeAccruals(ctx context.Context, client affiliateQueryExecer, inviterID, inviteeUserID int64) ([]sourcedAccrual, error) {
	rows, err := client.QueryContext(ctx, `
SELECT user_id,
       amount::double precision,
       frozen_until,
       source_kind,
       source_id,
       COALESCE(accrual_role, ''),
       COALESCE(direct_inviter_id, 0),
       created_at
FROM user_affiliate_ledger
WHERE source_user_id = $2
  AND action = 'accrue'
  AND source_kind IS NOT NULL
  AND source_id IS NOT NULL
  AND (user_id = $1 OR direct_inviter_id = $1)
ORDER BY created_at, id
FOR UPDATE`, inviterID, inviteeUserID)
	if err != nil {
		return nil, fmt.Errorf("lock invitee accruals: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]sourcedAccrual, 0)
	for rows.Next() {
		var accrual sourcedAccrual
		if err := rows.Scan(
			&accrual.InviterID, &accrual.Amount, &accrual.FrozenUtil,
			&accrual.SourceKind, &accrual.SourceID, &accrual.Role, &accrual.DirectInviterID, &accrual.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, accrual)
	}
	return out, rows.Err()
}

type accrualGroup struct {
	created time.Time
	rows    []sourcedAccrual
}

func groupAccrualsBySource(accruals []sourcedAccrual) []accrualGroup {
	index := make(map[string]int)
	groups := make([]accrualGroup, 0)
	for _, accrual := range accruals {
		key := accrual.SourceKind + ":" + fmt.Sprint(accrual.SourceID)
		pos, ok := index[key]
		if !ok {
			pos = len(groups)
			index[key] = pos
			groups = append(groups, accrualGroup{created: accrual.CreatedAt})
		}
		group := &groups[pos]
		if accrual.reverseRole() == service.AffiliateAccrualRoleDirect && (group.created.IsZero() || accrual.CreatedAt.Before(group.created)) {
			group.created = accrual.CreatedAt
		}
		group.rows = append(group.rows, accrual)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].created.Equal(groups[j].created) {
			return false
		}
		return groups[i].created.Before(groups[j].created)
	})
	for i := range groups {
		sort.SliceStable(groups[i].rows, func(a, b int) bool {
			ra := groups[i].rows[a].reverseRole()
			rb := groups[i].rows[b].reverseRole()
			if ra == rb {
				return groups[i].rows[a].InviterID < groups[i].rows[b].InviterID
			}
			return ra == service.AffiliateAccrualRoleDirect
		})
	}
	return groups
}

func sumSourceReverses(ctx context.Context, client affiliateQueryExecer, inviterID int64, sourceKind string, sourceID int64) (float64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COALESCE(SUM(amount), 0)::double precision
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'reverse'
  AND source_kind = $2
  AND source_id = $3`, inviterID, sourceKind, sourceID)
	if err != nil {
		return 0, fmt.Errorf("sum affiliate reverses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var total float64
	if err := rows.Scan(&total); err != nil {
		return 0, err
	}
	return total, rows.Err()
}

func affiliateInviterID(ctx context.Context, client affiliateQueryExecer, inviteeUserID int64) (int64, bool, error) {
	rows, err := client.QueryContext(ctx, `SELECT inviter_id FROM user_affiliates WHERE user_id = $1`, inviteeUserID)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return 0, false, rows.Err()
	}
	var inviter sql.NullInt64
	if err := rows.Scan(&inviter); err != nil {
		return 0, false, err
	}
	if !inviter.Valid || inviter.Int64 <= 0 {
		return 0, false, nil
	}
	return inviter.Int64, true, rows.Err()
}

func setAffiliateBalances(ctx context.Context, client affiliateQueryExecer, userID int64, quota, frozen float64) error {
	res, err := client.ExecContext(ctx, `
UPDATE user_affiliates
SET aff_quota = $1, aff_frozen_quota = $2, updated_at = NOW()
WHERE user_id = $3`, quota, frozen, userID)
	if err != nil {
		return fmt.Errorf("set affiliate balances: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return service.ErrAffiliateProfileNotFound
	}
	return nil
}

type affiliateReverseRow struct {
	InviterID       int64
	InviteeUserID   int64
	Amount          float64
	SourceKind      string
	SourceID        int64
	ReverseSeq      int64
	OrderID         int64
	QuotaAfter      float64
	FrozenAfter     float64
	HistoryAfter    float64
	Role            string
	DirectInviterID int64
}

func insertAffiliateReverse(ctx context.Context, client affiliateQueryExecer, row affiliateReverseRow) error {
	var sourceOrder any
	if row.OrderID > 0 {
		sourceOrder = row.OrderID
	}
	role := strings.TrimSpace(row.Role)
	if role == "" {
		role = service.AffiliateAccrualRoleDirect
	}
	var direct any
	if row.DirectInviterID > 0 {
		direct = row.DirectInviterID
	}
	_, err := client.ExecContext(ctx, `
INSERT INTO user_affiliate_ledger (
    user_id, action, amount, source_user_id, source_order_id,
    source_kind, source_id, reverse_seq, accrual_role, direct_inviter_id,
    aff_quota_after, aff_frozen_quota_after, aff_history_quota_after,
    created_at, updated_at
) VALUES ($1, 'reverse', $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())`,
		row.InviterID, row.Amount, row.InviteeUserID, sourceOrder,
		row.SourceKind, row.SourceID, row.ReverseSeq, role, direct,
		row.QuotaAfter, row.FrozenAfter, row.HistoryAfter)
	if err != nil {
		if isAffiliateUniqueViolation(err) {
			return service.ErrAffiliateReverseAlreadyApplied
		}
		return fmt.Errorf("insert affiliate reverse ledger: %w", err)
	}
	return nil
}
