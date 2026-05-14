package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
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
RETURNING user_id, status, contact, reason, admin_note, reviewed_by, reviewed_at, created_at, updated_at`,
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
		if err := rows.Scan(
			&app.UserID,
			&app.Status,
			&app.Contact,
			&app.Reason,
			&app.AdminNote,
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
SELECT da.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), da.status, da.contact, da.reason, da.admin_note, da.reviewed_by, da.reviewed_at, da.created_at, da.updated_at
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
		if err := rows.Scan(&item.UserID, &item.UserEmail, &item.Username, &item.Status, &item.Contact, &item.Reason, &item.AdminNote, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
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
RETURNING user_id, status, contact, reason, admin_note, reviewed_by, reviewed_at, created_at, updated_at`,
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
		if err := rows.Scan(&item.UserID, &item.Status, &item.Contact, &item.Reason, &item.AdminNote, &reviewedByID, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return err
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
		creator := sql.NullInt64{Int64: createdBy, Valid: createdBy > 0}
		_, err = txClient.ExecContext(txCtx, `
INSERT INTO distribution_wallet_ledger (wallet_id, user_id, action, amount, balance_after, reference_type, reference_id, note, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())`,
			updated.ID, updated.UserID, action, amount, updated.Balance, referenceType, referenceID, note, creator)
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
SELECT user_id, status, contact, reason, admin_note, reviewed_by, reviewed_at, created_at, updated_at
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
	var item service.DistributionAgentApplication
	var reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	if err := rows.Scan(&item.UserID, &item.Status, &item.Contact, &item.Reason, &item.AdminNote, &reviewedBy, &reviewedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	if reviewedBy.Valid {
		item.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		item.ReviewedAt = &reviewedAt.Time
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &item, nil
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
