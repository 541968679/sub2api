package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type distributionRepository struct {
	client *dbent.Client
}

func NewDistributionRepository(client *dbent.Client, _ *sql.DB) service.DistributionRepository {
	return &distributionRepository{client: client}
}

func (r *distributionRepository) EnsureAgent(ctx context.Context, userID int64) (*service.DistributionAgentApplication, error) {
	client := clientFromContext(ctx, r.client)
	return queryDistributionAgent(ctx, client, userID)
}

func (r *distributionRepository) CreateAgentApplication(ctx context.Context, userID int64, contact, reason string) (*service.DistributionAgentApplication, error) {
	contact = strings.TrimSpace(contact)
	reason = strings.TrimSpace(reason)

	var out *service.DistributionAgentApplication
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		existing, err := queryDistributionAgent(txCtx, txClient, userID)
		if err == nil && existing != nil {
			if existing.Status == service.DistributionAgentStatusPending || existing.Status == service.DistributionAgentStatusApproved {
				return service.ErrDistributionAlreadyApplied
			}
		} else if err != nil && !errors.Is(err, service.ErrDistributionAgentNotFound) {
			return err
		}

		rows, err := txClient.QueryContext(txCtx, `
INSERT INTO distribution_agents (user_id, status, contact, reason, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET
	status = CASE
		WHEN distribution_agents.status = 'approved' THEN distribution_agents.status
		ELSE EXCLUDED.status
	END,
	contact = EXCLUDED.contact,
	reason = EXCLUDED.reason,
	updated_at = NOW()
RETURNING user_id, status, contact, reason, admin_note, rmb_per_usd_override::double precision, subscription_discount_override::double precision, reviewed_by, reviewed_at, created_at, updated_at`,
			userID, service.DistributionAgentStatusPending, contact, reason)
		if err != nil {
			if isUniqueConstraintViolation(err) {
				return service.ErrDistributionAlreadyApplied
			}
			return fmt.Errorf("create distribution application: %w", err)
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return fmt.Errorf("create distribution application: no row returned")
		}
		var app service.DistributionAgentApplication
		var reviewedBy sql.NullInt64
		var reviewedAt sql.NullTime
		var rmbOverride sql.NullFloat64
		var subscriptionOverride sql.NullFloat64
		if err := rows.Scan(
			&app.UserID,
			&app.Status,
			&app.Contact,
			&app.Reason,
			&app.AdminNote,
			&rmbOverride,
			&subscriptionOverride,
			&reviewedBy,
			&reviewedAt,
			&app.CreatedAt,
			&app.UpdatedAt,
		); err != nil {
			return err
		}
		if reviewedBy.Valid {
			app.ReviewedBy = &reviewedBy.Int64
		}
		if rmbOverride.Valid {
			app.RMBPerUSDOverride = &rmbOverride.Float64
		}
		if subscriptionOverride.Valid {
			app.SubscriptionDiscountOverride = &subscriptionOverride.Float64
		}
		if reviewedAt.Valid {
			app.ReviewedAt = &reviewedAt.Time
		}
		out = &app
		return rows.Err()
	})
	return out, err
}

func (r *distributionRepository) GetAgentApplication(ctx context.Context, userID int64) (*service.DistributionAgentApplication, error) {
	client := clientFromContext(ctx, r.client)
	return queryDistributionAgent(ctx, client, userID)
}

func (r *distributionRepository) ListAgentApplications(ctx context.Context, page, pageSize int, search string) ([]service.DistributionAgentApplication, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	like := "%" + strings.TrimSpace(search) + "%"
	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, `
SELECT COUNT(*)
FROM distribution_agents da
JOIN users u ON u.id = da.user_id
WHERE ($1 = '%%' OR u.email ILIKE $1 OR u.username ILIKE $1)`, like)
	if err != nil {
		return nil, 0, err
	}

	rows, err := client.QueryContext(ctx, `
SELECT da.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), da.status, da.contact, da.reason, da.admin_note,
       da.rmb_per_usd_override::double precision, da.subscription_discount_override::double precision,
       da.reviewed_by, da.reviewed_at, da.created_at, da.updated_at
FROM distribution_agents da
JOIN users u ON u.id = da.user_id
WHERE ($1 = '%%' OR u.email ILIKE $1 OR u.username ILIKE $1)
ORDER BY da.updated_at DESC
LIMIT $2 OFFSET $3`, like, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.DistributionAgentApplication, 0)
	for rows.Next() {
		var item service.DistributionAgentApplication
		var reviewedBy sql.NullInt64
		var reviewedAt sql.NullTime
		var rmbOverride sql.NullFloat64
		var subscriptionOverride sql.NullFloat64
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.Username, &item.Status, &item.Contact, &item.Reason, &item.AdminNote, &rmbOverride, &subscriptionOverride, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if rmbOverride.Valid {
			item.RMBPerUSDOverride = &rmbOverride.Float64
		}
		if subscriptionOverride.Valid {
			item.SubscriptionDiscountOverride = &subscriptionOverride.Float64
		}
		if reviewedBy.Valid {
			item.ReviewedBy = &reviewedBy.Int64
		}
		if reviewedAt.Valid {
			item.ReviewedAt = &reviewedAt.Time
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *distributionRepository) ReviewAgentApplication(ctx context.Context, userID int64, approved bool, adminNote string, reviewedBy int64) (*service.DistributionAgentApplication, error) {
	adminNote = strings.TrimSpace(adminNote)
	status := service.DistributionAgentStatusRejected
	if approved {
		status = service.DistributionAgentStatusApproved
	}

	var out *service.DistributionAgentApplication
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		rows, err := txClient.QueryContext(txCtx, `
UPDATE distribution_agents
SET status = $1,
    admin_note = $2,
    reviewed_by = $3,
    reviewed_at = NOW(),
    updated_at = NOW()
WHERE user_id = $4
RETURNING user_id, status, contact, reason, admin_note, rmb_per_usd_override::double precision, subscription_discount_override::double precision, reviewed_by, reviewed_at, created_at, updated_at`,
			status, adminNote, reviewedBy, userID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return service.ErrDistributionAgentNotFound
		}
		var item service.DistributionAgentApplication
		var reviewedByID sql.NullInt64
		var reviewedAt sql.NullTime
		var rmbOverride sql.NullFloat64
		var subscriptionOverride sql.NullFloat64
		if err := rows.Scan(&item.UserID, &item.Status, &item.Contact, &item.Reason, &item.AdminNote, &rmbOverride, &subscriptionOverride, &reviewedByID, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
		}
		if rmbOverride.Valid {
			item.RMBPerUSDOverride = &rmbOverride.Float64
		}
		if subscriptionOverride.Valid {
			item.SubscriptionDiscountOverride = &subscriptionOverride.Float64
		}
		if reviewedByID.Valid {
			item.ReviewedBy = &reviewedByID.Int64
		}
		if reviewedAt.Valid {
			item.ReviewedAt = &reviewedAt.Time
		}
		out = &item
		return rows.Err()
	})
	return out, err
}

func (r *distributionRepository) EnsureWallet(ctx context.Context, userID int64) (*service.DistributionWallet, error) {
	client := clientFromContext(ctx, r.client)
	return ensureDistributionWalletWithClient(ctx, client, userID)
}

func (r *distributionRepository) GetWalletByUserID(ctx context.Context, userID int64) (*service.DistributionWallet, error) {
	client := clientFromContext(ctx, r.client)
	return queryDistributionWallet(ctx, client, userID)
}

func (r *distributionRepository) ListWalletLedger(ctx context.Context, userID int64, page, pageSize int) ([]service.DistributionWalletLedgerEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, `SELECT COUNT(*) FROM distribution_wallet_ledger WHERE user_id = $1`, userID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := client.QueryContext(ctx, `
SELECT id, wallet_id, user_id, action, amount::double precision, balance_after::double precision, reference_type, reference_id, note, created_at
FROM distribution_wallet_ledger
WHERE user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.DistributionWalletLedgerEntry, 0)
	for rows.Next() {
		var item service.DistributionWalletLedgerEntry
		if err := rows.Scan(&item.ID, &item.WalletID, &item.UserID, &item.Action, &item.Amount, &item.BalanceAfter, &item.ReferenceType, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *distributionRepository) ListWallets(ctx context.Context, page, pageSize int, search string) ([]service.DistributionWallet, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	like := "%" + strings.TrimSpace(search) + "%"
	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, `
SELECT COUNT(*)
FROM distribution_wallets dw
JOIN users u ON u.id = dw.user_id
WHERE ($1 = '%%' OR u.email ILIKE $1 OR u.username ILIKE $1 OR dw.user_id::text = trim(both '%' from $1))`, like)
	if err != nil {
		return nil, 0, err
	}

	rows, err := client.QueryContext(ctx, `
SELECT dw.id, dw.user_id, dw.agent_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
       dw.balance::double precision, dw.total_recharged::double precision,
       dw.total_spent::double precision, dw.total_rebate::double precision,
       dw.status, dw.created_at, dw.updated_at
FROM distribution_wallets dw
JOIN users u ON u.id = dw.user_id
WHERE ($1 = '%%' OR u.email ILIKE $1 OR u.username ILIKE $1 OR dw.user_id::text = trim(both '%' from $1))
ORDER BY dw.updated_at DESC, dw.id DESC
LIMIT $2 OFFSET $3`, like, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.DistributionWallet, 0)
	for rows.Next() {
		var item service.DistributionWallet
		if err := rows.Scan(&item.ID, &item.UserID, &item.AgentID, &item.UserEmail, &item.Username, &item.Balance, &item.TotalRecharged, &item.TotalSpent, &item.TotalRebate, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *distributionRepository) UpdateAgentRates(ctx context.Context, userID int64, rates service.DistributionAgentRateSettings) (*service.DistributionAgentApplication, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
UPDATE distribution_agents
SET rmb_per_usd_override = $1,
    subscription_discount_override = $2,
    updated_at = NOW()
WHERE user_id = $3
RETURNING user_id, status, contact, reason, admin_note, rmb_per_usd_override::double precision, subscription_discount_override::double precision, reviewed_by, reviewed_at, created_at, updated_at`,
		nullableFloat64(rates.RMBPerUSDOverride),
		nullableFloat64(rates.SubscriptionDiscountOverride),
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, service.ErrDistributionAgentNotFound
	}
	item, err := scanDistributionAgent(rows)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func (r *distributionRepository) ListAllWalletLedger(ctx context.Context, page, pageSize int, userID int64) ([]service.DistributionWalletLedgerEntry, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	client := clientFromContext(ctx, r.client)

	total, err := scanInt64(ctx, client, `
SELECT COUNT(*) FROM distribution_wallet_ledger
WHERE ($1::bigint = 0 OR user_id = $1)`, userID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := client.QueryContext(ctx, `
SELECT id, wallet_id, user_id, action, amount::double precision, balance_after::double precision, reference_type, reference_id, note, created_at
FROM distribution_wallet_ledger
WHERE ($1::bigint = 0 OR user_id = $1)
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3`, userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.DistributionWalletLedgerEntry, 0)
	for rows.Next() {
		var item service.DistributionWalletLedgerEntry
		if err := rows.Scan(&item.ID, &item.WalletID, &item.UserID, &item.Action, &item.Amount, &item.BalanceAfter, &item.ReferenceType, &item.ReferenceID, &item.Note, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *distributionRepository) CreateAsset(ctx context.Context, input service.DistributionCreateAssetInput) (*service.DistributionAsset, error) {
	client := clientFromContext(ctx, r.client)
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = service.DistributionAssetStatusActive
	}
	rows, err := client.QueryContext(ctx, `
INSERT INTO distribution_assets (
    user_id, wallet_id, asset_type, reference_type, reference_id, display_value, package_url,
    face_value, cost_rmb, group_id, validity_days, quota_usd, status, customer_user_id,
    used_at, expires_at, note, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, NOW(), NOW())
ON CONFLICT (reference_type, reference_id) DO UPDATE SET
    display_value = EXCLUDED.display_value,
    package_url = EXCLUDED.package_url,
    face_value = EXCLUDED.face_value,
    cost_rmb = EXCLUDED.cost_rmb,
    group_id = EXCLUDED.group_id,
    validity_days = EXCLUDED.validity_days,
    quota_usd = EXCLUDED.quota_usd,
    status = EXCLUDED.status,
    customer_user_id = EXCLUDED.customer_user_id,
    used_at = EXCLUDED.used_at,
    expires_at = EXCLUDED.expires_at,
    note = EXCLUDED.note,
    updated_at = NOW()
RETURNING id, user_id, wallet_id, asset_type, reference_type, reference_id, display_value, package_url,
          face_value::double precision, cost_rmb::double precision, group_id, validity_days,
          quota_usd::double precision, status, customer_user_id, used_at, expires_at,
          refunded_at, refunded_rmb::double precision, refunded_by, note, created_at, updated_at`,
		input.UserID,
		input.WalletID,
		input.AssetType,
		input.ReferenceType,
		input.ReferenceID,
		input.DisplayValue,
		input.PackageURL,
		input.FaceValue,
		input.CostRMB,
		nullableInt64(input.GroupID),
		input.ValidityDays,
		input.QuotaUSD,
		status,
		nullableInt64(input.CustomerUserID),
		nullableTime(input.UsedAt),
		nullableTime(input.ExpiresAt),
		strings.TrimSpace(input.Note),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, fmt.Errorf("create distribution asset: no row returned")
	}
	out, err := scanDistributionAsset(rows)
	if err != nil {
		return nil, err
	}
	return out, rows.Err()
}

func (r *distributionRepository) ListAssets(ctx context.Context, page, pageSize int, userID int64, assetType, status, search string) ([]service.DistributionAsset, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 500 {
		pageSize = 50
	}
	offset := (page - 1) * pageSize
	assetType = strings.TrimSpace(assetType)
	status = strings.TrimSpace(status)
	like := "%" + strings.TrimSpace(search) + "%"
	client := clientFromContext(ctx, r.client)

	where := `
WHERE ($1::bigint = 0 OR da.user_id = $1)
  AND ($2 = '' OR da.asset_type = $2)
  AND ($3 = '' OR CASE
         WHEN rc.status = 'used' THEN 'used'
         WHEN rc.status = 'expired' THEN 'expired'
         WHEN ak.deleted_at IS NOT NULL THEN 'disabled'
         WHEN ak.expires_at IS NOT NULL AND ak.expires_at <= NOW() THEN 'expired'
         WHEN ak.status IS NOT NULL AND ak.status <> 'active' THEN ak.status
         ELSE da.status
       END = $3)
  AND ($4 = '%%' OR da.display_value ILIKE $4 OR da.reference_id ILIKE $4 OR u.email ILIKE $4 OR u.username ILIKE $4)`
	total, err := scanInt64(ctx, client, `
SELECT COUNT(*)
FROM distribution_assets da
JOIN users u ON u.id = da.user_id
LEFT JOIN users cu ON cu.id = da.customer_user_id
LEFT JOIN groups g ON g.id = da.group_id
LEFT JOIN redeem_codes rc ON da.reference_type = 'redeem_code' AND rc.code = da.reference_id
LEFT JOIN api_keys ak ON da.reference_type = 'api_key' AND ak.id::text = da.reference_id
`+where, userID, assetType, status, like)
	if err != nil {
		return nil, 0, err
	}

	rows, err := client.QueryContext(ctx, `
SELECT da.id, da.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
       da.wallet_id, da.asset_type, da.reference_type, da.reference_id,
       da.display_value, da.package_url, da.face_value::double precision,
       da.cost_rmb::double precision, da.group_id, COALESCE(g.name, ''),
       da.validity_days, da.quota_usd::double precision,
       CASE
         WHEN rc.status = 'used' THEN 'used'
         WHEN rc.status = 'expired' THEN 'expired'
         WHEN ak.deleted_at IS NOT NULL THEN 'disabled'
         WHEN ak.expires_at IS NOT NULL AND ak.expires_at <= NOW() THEN 'expired'
         WHEN ak.status IS NOT NULL AND ak.status <> 'active' THEN ak.status
         ELSE da.status
       END AS effective_status,
       COALESCE(da.customer_user_id, rc.used_by) AS effective_customer_user_id,
       COALESCE(cu.email, rcu.email, '') AS effective_customer_email,
       COALESCE(da.used_at, rc.used_at) AS effective_used_at,
       COALESCE(da.expires_at, ak.expires_at) AS effective_expires_at,
       da.refunded_at, da.refunded_rmb::double precision, da.refunded_by,
       da.note, da.created_at, da.updated_at
FROM distribution_assets da
JOIN users u ON u.id = da.user_id
LEFT JOIN redeem_codes rc ON da.reference_type = 'redeem_code' AND rc.code = da.reference_id
LEFT JOIN api_keys ak ON da.reference_type = 'api_key' AND ak.id::text = da.reference_id
LEFT JOIN users cu ON cu.id = da.customer_user_id
LEFT JOIN users rcu ON rcu.id = rc.used_by
LEFT JOIN groups g ON g.id = da.group_id
`+where+`
ORDER BY da.created_at DESC, da.id DESC
LIMIT $5 OFFSET $6`, userID, assetType, status, like, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]service.DistributionAsset, 0)
	for rows.Next() {
		item, err := scanDistributionAssetWithJoins(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *distributionRepository) GetAssetByID(ctx context.Context, id int64) (*service.DistributionAsset, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
SELECT da.id, da.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
       da.wallet_id, da.asset_type, da.reference_type, da.reference_id,
       da.display_value, da.package_url, da.face_value::double precision,
       da.cost_rmb::double precision, da.group_id, COALESCE(g.name, ''),
       da.validity_days, da.quota_usd::double precision,
       CASE
         WHEN rc.status = 'used' THEN 'used'
         WHEN rc.status = 'expired' THEN 'expired'
         WHEN ak.deleted_at IS NOT NULL THEN 'disabled'
         WHEN ak.expires_at IS NOT NULL AND ak.expires_at <= NOW() THEN 'expired'
         WHEN ak.status IS NOT NULL AND ak.status <> 'active' THEN ak.status
         ELSE da.status
       END AS effective_status,
       COALESCE(da.customer_user_id, rc.used_by) AS effective_customer_user_id,
       COALESCE(cu.email, rcu.email, '') AS effective_customer_email,
       COALESCE(da.used_at, rc.used_at) AS effective_used_at,
       COALESCE(da.expires_at, ak.expires_at) AS effective_expires_at,
       da.refunded_at, da.refunded_rmb::double precision, da.refunded_by,
       da.note, da.created_at, da.updated_at
FROM distribution_assets da
JOIN users u ON u.id = da.user_id
LEFT JOIN redeem_codes rc ON da.reference_type = 'redeem_code' AND rc.code = da.reference_id
LEFT JOIN api_keys ak ON da.reference_type = 'api_key' AND ak.id::text = da.reference_id
LEFT JOIN users cu ON cu.id = da.customer_user_id
LEFT JOIN users rcu ON rcu.id = rc.used_by
LEFT JOIN groups g ON g.id = da.group_id
WHERE da.id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, infraerrors.NotFound("DISTRIBUTION_ASSET_NOT_FOUND", "distribution asset not found")
	}
	item, err := scanDistributionAssetWithJoins(rows)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func (r *distributionRepository) MarkAssetRefunded(ctx context.Context, assetID int64, status string, refundedBy int64) (*service.DistributionAsset, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
UPDATE distribution_assets
SET status = $1,
    refunded_at = NOW(),
    refunded_rmb = cost_rmb,
    refunded_by = $2,
    updated_at = NOW()
WHERE id = $3 AND refunded_at IS NULL AND refunded_rmb = 0
RETURNING id, user_id, wallet_id, asset_type, reference_type, reference_id, display_value, package_url,
          face_value::double precision, cost_rmb::double precision, group_id, validity_days,
          quota_usd::double precision, status, customer_user_id, used_at, expires_at,
          refunded_at, refunded_rmb::double precision, refunded_by, note, created_at, updated_at`,
		status,
		nullableInt64Value(refundedBy),
		assetID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, infraerrors.Conflict("DISTRIBUTION_ASSET_REFUNDED", "distribution asset has already been refunded")
	}
	item, err := scanDistributionAsset(rows)
	if err != nil {
		return nil, err
	}
	return item, rows.Err()
}

func (r *distributionRepository) UpdateWalletStatus(ctx context.Context, userID int64, status string) (*service.DistributionWallet, error) {
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
UPDATE distribution_wallets
SET status = $1, updated_at = NOW()
WHERE user_id = $2
RETURNING id, user_id, agent_id, balance::double precision, total_recharged::double precision, total_spent::double precision, total_rebate::double precision, status, created_at, updated_at`,
		status, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, service.ErrDistributionWalletNotFound
	}
	var wallet service.DistributionWallet
	if err := rows.Scan(&wallet.ID, &wallet.UserID, &wallet.AgentID, &wallet.Balance, &wallet.TotalRecharged, &wallet.TotalSpent, &wallet.TotalRebate, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return nil, err
	}
	return &wallet, rows.Err()
}

func (r *distributionRepository) AdjustWalletBalance(ctx context.Context, userID int64, amount float64, action, referenceType, referenceID, note string, createdBy int64) (*service.DistributionWallet, error) {
	var out *service.DistributionWallet
	err := r.withTx(ctx, func(txCtx context.Context, txClient *dbent.Client) error {
		wallet, err := queryDistributionWallet(txCtx, txClient, userID)
		if err != nil {
			return err
		}
		if wallet.Status != service.DistributionWalletStatusActive && action != service.DistributionLedgerActionAdminAdjust {
			return service.ErrDistributionWalletInactive
		}
		if wallet.Balance+amount < -0.00000001 {
			return service.ErrDistributionInsufficient
		}
		rows, err := txClient.QueryContext(txCtx, `
UPDATE distribution_wallets
SET balance = balance + $1,
    total_recharged = total_recharged + CASE WHEN $1 > 0 THEN $1 ELSE 0 END,
    total_spent = total_spent + CASE WHEN $1 < 0 THEN -$1 ELSE 0 END,
    updated_at = NOW()
WHERE user_id = $2
RETURNING id, user_id, agent_id, balance::double precision, total_recharged::double precision, total_spent::double precision, total_rebate::double precision, status, created_at, updated_at`,
			amount, userID)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		if !rows.Next() {
			return service.ErrDistributionWalletNotFound
		}
		var updated service.DistributionWallet
		if err := rows.Scan(&updated.ID, &updated.UserID, &updated.AgentID, &updated.Balance, &updated.TotalRecharged, &updated.TotalSpent, &updated.TotalRebate, &updated.Status, &updated.CreatedAt, &updated.UpdatedAt); err != nil {
			return err
		}
		if err := rows.Err(); err != nil {
			return err
		}
		_, err = txClient.ExecContext(txCtx, `
INSERT INTO distribution_wallet_ledger (wallet_id, user_id, action, amount, balance_after, reference_type, reference_id, note, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
			updated.ID, updated.UserID, action, amount, updated.Balance, referenceType, referenceID, note, nullablePositiveInt64(createdBy))
		if err != nil {
			return err
		}
		out = &updated
		return nil
	})
	return out, err
}

func (r *distributionRepository) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.withTx(ctx, func(txCtx context.Context, _ *dbent.Client) error {
		return fn(txCtx)
	})
}

func (r *distributionRepository) withTx(ctx context.Context, fn func(txCtx context.Context, txClient *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin distribution transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit distribution transaction: %w", err)
	}
	return nil
}

func queryDistributionAgent(ctx context.Context, client *dbent.Client, userID int64) (*service.DistributionAgentApplication, error) {
	if client == nil {
		return nil, service.ErrDistributionAgentNotFound
	}
	rows, err := client.QueryContext(ctx, `
SELECT user_id, status, contact, reason, admin_note, rmb_per_usd_override::double precision, subscription_discount_override::double precision, reviewed_by, reviewed_at, created_at, updated_at
FROM distribution_agents
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrDistributionAgentNotFound
	}
	item, err := scanDistributionAgent(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return item, nil
}

func ensureDistributionWalletWithClient(ctx context.Context, client *dbent.Client, userID int64) (*service.DistributionWallet, error) {
	if client == nil {
		return nil, service.ErrDistributionWalletNotFound
	}
	app, err := queryDistributionAgent(ctx, client, userID)
	if err != nil {
		return nil, err
	}
	if app.Status == service.DistributionAgentStatusPending {
		return nil, service.ErrDistributionAgentPending
	}
	if app.Status == service.DistributionAgentStatusRejected {
		return nil, service.ErrDistributionAgentRejected
	}
	if app.Status == service.DistributionAgentStatusFrozen {
		return nil, service.ErrDistributionAgentFrozen
	}

	rows, err := client.QueryContext(ctx, `
INSERT INTO distribution_wallets (user_id, agent_id, balance, total_recharged, total_spent, total_rebate, status, created_at, updated_at)
VALUES ($1, (SELECT id FROM distribution_agents WHERE user_id = $1), 0, 0, 0, 0, $2, NOW(), NOW())
ON CONFLICT (user_id) DO UPDATE SET updated_at = NOW()
RETURNING id, user_id, agent_id, balance::double precision, total_recharged::double precision, total_spent::double precision, total_rebate::double precision, status, created_at, updated_at`,
		userID, service.DistributionWalletStatusActive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, service.ErrDistributionWalletNotFound
	}
	var wallet service.DistributionWallet
	if err := rows.Scan(&wallet.ID, &wallet.UserID, &wallet.AgentID, &wallet.Balance, &wallet.TotalRecharged, &wallet.TotalSpent, &wallet.TotalRebate, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &wallet, nil
}

func queryDistributionWallet(ctx context.Context, client *dbent.Client, userID int64) (*service.DistributionWallet, error) {
	if client == nil {
		return nil, service.ErrDistributionWalletNotFound
	}
	rows, err := client.QueryContext(ctx, `
SELECT id, user_id, agent_id, balance::double precision, total_recharged::double precision, total_spent::double precision, total_rebate::double precision, status, created_at, updated_at
FROM distribution_wallets
WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, service.ErrDistributionWalletNotFound
	}
	var wallet service.DistributionWallet
	if err := rows.Scan(&wallet.ID, &wallet.UserID, &wallet.AgentID, &wallet.Balance, &wallet.TotalRecharged, &wallet.TotalSpent, &wallet.TotalRebate, &wallet.Status, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &wallet, nil
}

type distributionAssetScanner interface {
	Scan(dest ...any) error
}

func scanDistributionAgent(rows distributionAssetScanner) (*service.DistributionAgentApplication, error) {
	var item service.DistributionAgentApplication
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	var rmbOverride sql.NullFloat64
	var subscriptionOverride sql.NullFloat64
	if err := rows.Scan(
		&item.UserID,
		&item.Status,
		&item.Contact,
		&item.Reason,
		&item.AdminNote,
		&rmbOverride,
		&subscriptionOverride,
		&reviewedBy,
		&reviewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if rmbOverride.Valid {
		item.RMBPerUSDOverride = &rmbOverride.Float64
	}
	if subscriptionOverride.Valid {
		item.SubscriptionDiscountOverride = &subscriptionOverride.Float64
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	return &item, nil
}

func scanDistributionAsset(rows distributionAssetScanner) (*service.DistributionAsset, error) {
	var item service.DistributionAsset
	var groupID sql.NullInt64
	var customerUserID sql.NullInt64
	var usedAt sql.NullTime
	var expiresAt sql.NullTime
	var refundedAt sql.NullTime
	var refundedBy sql.NullInt64
	if err := rows.Scan(
		&item.ID,
		&item.UserID,
		&item.WalletID,
		&item.AssetType,
		&item.ReferenceType,
		&item.ReferenceID,
		&item.DisplayValue,
		&item.PackageURL,
		&item.FaceValue,
		&item.CostRMB,
		&groupID,
		&item.ValidityDays,
		&item.QuotaUSD,
		&item.Status,
		&customerUserID,
		&usedAt,
		&expiresAt,
		&refundedAt,
		&item.RefundedRMB,
		&refundedBy,
		&item.Note,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		item.GroupID = &groupID.Int64
	}
	if customerUserID.Valid {
		item.CustomerUserID = &customerUserID.Int64
	}
	if usedAt.Valid {
		item.UsedAt = &usedAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if refundedAt.Valid {
		item.RefundedAt = &refundedAt.Time
	}
	if refundedBy.Valid {
		item.RefundedBy = &refundedBy.Int64
	}
	return &item, nil
}

func scanDistributionAssetWithJoins(rows distributionAssetScanner) (*service.DistributionAsset, error) {
	var item service.DistributionAsset
	var groupID sql.NullInt64
	var customerUserID sql.NullInt64
	var usedAt sql.NullTime
	var expiresAt sql.NullTime
	var refundedAt sql.NullTime
	var refundedBy sql.NullInt64
	if err := rows.Scan(
		&item.ID,
		&item.UserID,
		&item.UserEmail,
		&item.Username,
		&item.WalletID,
		&item.AssetType,
		&item.ReferenceType,
		&item.ReferenceID,
		&item.DisplayValue,
		&item.PackageURL,
		&item.FaceValue,
		&item.CostRMB,
		&groupID,
		&item.GroupName,
		&item.ValidityDays,
		&item.QuotaUSD,
		&item.Status,
		&customerUserID,
		&item.CustomerEmail,
		&usedAt,
		&expiresAt,
		&refundedAt,
		&item.RefundedRMB,
		&refundedBy,
		&item.Note,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if groupID.Valid {
		item.GroupID = &groupID.Int64
	}
	if customerUserID.Valid {
		item.CustomerUserID = &customerUserID.Int64
	}
	if usedAt.Valid {
		item.UsedAt = &usedAt.Time
	}
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	if refundedAt.Valid {
		item.RefundedAt = &refundedAt.Time
	}
	if refundedBy.Valid {
		item.RefundedBy = &refundedBy.Int64
	}
	return &item, nil
}

func nullableInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullableInt64Value(v int64) sql.NullInt64 {
	return sql.NullInt64{Int64: v, Valid: v > 0}
}

func nullablePositiveInt64(v int64) any {
	if v <= 0 {
		return nil
	}
	return v
}

func nullableFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func nullableTime(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}
