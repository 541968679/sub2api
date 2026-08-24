package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/usersmartscheduleaccount"
	"github.com/Wei-Shaw/sub2api/ent/usersmartschedulepolicy"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type userSmartScheduleRepository struct {
	client *dbent.Client
}

// NewUserSmartScheduleRepository persists user × platform smart-schedule policies and pool members.
func NewUserSmartScheduleRepository(client *dbent.Client) service.UserSmartScheduleRepository {
	return &userSmartScheduleRepository{client: client}
}

func (r *userSmartScheduleRepository) ListByUser(ctx context.Context, userID int64) (*service.UserSmartScheduleBundle, error) {
	if r == nil || r.client == nil || userID <= 0 {
		return &service.UserSmartScheduleBundle{Policies: map[string]*service.SmartSchedulePlatformPolicy{}}, nil
	}
	client := clientFromContext(ctx, r.client)
	policies, err := client.UserSmartSchedulePolicy.Query().
		Where(usersmartschedulepolicy.UserIDEQ(userID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list smart schedule policies: %w", err)
	}
	members, err := client.UserSmartScheduleAccount.Query().
		Where(usersmartscheduleaccount.UserIDEQ(userID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list smart schedule accounts: %w", err)
	}
	bundle := assembleSmartScheduleBundle(policies, members)
	if err := overlaySmartSchedulePaused(ctx, client, []int64{userID}, map[int64]*service.UserSmartScheduleBundle{
		userID: bundle,
	}); err != nil {
		return nil, err
	}
	if err := overlaySmartScheduleProbeConcurrency(ctx, client, []int64{userID}, map[int64]*service.UserSmartScheduleBundle{
		userID: bundle,
	}); err != nil {
		return nil, err
	}
	if err := overlaySmartScheduleLatencyGate(ctx, client, []int64{userID}, map[int64]*service.UserSmartScheduleBundle{
		userID: bundle,
	}); err != nil {
		return nil, err
	}
	return bundle, nil
}

func (r *userSmartScheduleRepository) ListByUsers(ctx context.Context, userIDs []int64) (map[int64]*service.UserSmartScheduleBundle, error) {
	out := make(map[int64]*service.UserSmartScheduleBundle, len(userIDs))
	if r == nil || r.client == nil || len(userIDs) == 0 {
		return out, nil
	}
	client := clientFromContext(ctx, r.client)
	policies, err := client.UserSmartSchedulePolicy.Query().
		Where(usersmartschedulepolicy.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list smart schedule policies: %w", err)
	}
	members, err := client.UserSmartScheduleAccount.Query().
		Where(usersmartscheduleaccount.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list smart schedule accounts: %w", err)
	}
	policiesByUser := make(map[int64][]*dbent.UserSmartSchedulePolicy, len(userIDs))
	for _, row := range policies {
		if row == nil {
			continue
		}
		policiesByUser[row.UserID] = append(policiesByUser[row.UserID], row)
	}
	membersByUser := make(map[int64][]*dbent.UserSmartScheduleAccount, len(userIDs))
	for _, row := range members {
		if row == nil {
			continue
		}
		membersByUser[row.UserID] = append(membersByUser[row.UserID], row)
	}
	for _, userID := range userIDs {
		out[userID] = assembleSmartScheduleBundle(policiesByUser[userID], membersByUser[userID])
	}
	if err := overlaySmartSchedulePaused(ctx, client, userIDs, out); err != nil {
		return nil, err
	}
	if err := overlaySmartScheduleProbeConcurrency(ctx, client, userIDs, out); err != nil {
		return nil, err
	}
	if err := overlaySmartScheduleLatencyGate(ctx, client, userIDs, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *userSmartScheduleRepository) ReplacePlatform(ctx context.Context, userID int64, platform string, policy service.SmartSchedulePlatformWrite) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("smart schedule repository unavailable")
	}
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		now := time.Now().UTC()
		existing, err := client.UserSmartSchedulePolicy.Query().
			Where(
				usersmartschedulepolicy.UserIDEQ(userID),
				usersmartschedulepolicy.PlatformEQ(platform),
			).
			Only(txCtx)
		if err != nil && !dbent.IsNotFound(err) {
			return fmt.Errorf("get smart schedule policy: %w", err)
		}
		if existing == nil {
			create := client.UserSmartSchedulePolicy.Create().
				SetUserID(userID).
				SetPlatform(platform).
				SetEnabled(policy.Enabled).
				SetCooldownMinutes(policy.CooldownMinutes).
				SetCreatedAt(now).
				SetUpdatedAt(now).
				SetNillableQualityMaxP50TtftMs(policy.QualityMaxP50TTFTMs).
				SetNillableQualityMinSuccessRate(policy.QualityMinSuccessRate).
				SetNillableQualityMinSuccessSamples(policy.QualityMinSuccessSamples).
				SetNillableQualityMinTtftSamples(policy.QualityMinTTFTSamples)
			if policy.QualityCondition != nil && *policy.QualityCondition != "" {
				create.SetQualityCondition(*policy.QualityCondition)
			}
			if err := create.Exec(txCtx); err != nil {
				return fmt.Errorf("create smart schedule policy: %w", err)
			}
		} else {
			update := client.UserSmartSchedulePolicy.UpdateOne(existing).
				SetEnabled(policy.Enabled).
				SetCooldownMinutes(policy.CooldownMinutes).
				SetUpdatedAt(now)
			applySmartSchedulePolicyQualityUpdate(update, policy)
			if err := update.Exec(txCtx); err != nil {
				return fmt.Errorf("update smart schedule policy: %w", err)
			}
		}
		pausedIDs, err := listPausedSmartScheduleAccountIDs(txCtx, client, userID, platform)
		if err != nil {
			return err
		}
		if _, err := client.UserSmartScheduleAccount.Delete().
			Where(
				usersmartscheduleaccount.UserIDEQ(userID),
				usersmartscheduleaccount.PlatformEQ(platform),
			).
			Exec(txCtx); err != nil {
			return fmt.Errorf("clear smart schedule accounts: %w", err)
		}
		keepPaused := remainingPausedSmartScheduleIDs(pausedIDs, policy.Accounts)
		for _, member := range policy.Accounts {
			create := client.UserSmartScheduleAccount.Create().
				SetUserID(userID).
				SetAccountID(member.AccountID).
				SetPlatform(platform).
				SetCreatedAt(now)
			if member.MaxConcurrency != nil && *member.MaxConcurrency >= 1 {
				create.SetMaxConcurrency(*member.MaxConcurrency)
			}
			if member.SortOrder != nil {
				create.SetSortOrder(*member.SortOrder)
			}
			if err := create.Exec(txCtx); err != nil {
				return fmt.Errorf("insert smart schedule account %d: %w", member.AccountID, err)
			}
		}
		if err := restoreSmartSchedulePaused(txCtx, client, userID, platform, keepPaused); err != nil {
			return err
		}
		if err := writeSmartScheduleProbeConcurrency(txCtx, client, userID, platform, policy); err != nil {
			return err
		}
		if err := writeSmartScheduleLatencyGate(txCtx, client, userID, platform, policy); err != nil {
			return err
		}
		return nil
	})
}

func (r *userSmartScheduleRepository) UpdateSortOrders(ctx context.Context, userID int64, platform string, orders []service.SmartScheduleSortAssignment) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("smart schedule repository unavailable")
	}
	if len(orders) == 0 {
		return nil
	}
	return r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		for _, order := range orders {
			n, err := client.UserSmartScheduleAccount.Update().
				Where(
					usersmartscheduleaccount.UserIDEQ(userID),
					usersmartscheduleaccount.AccountIDEQ(order.AccountID),
					usersmartscheduleaccount.PlatformEQ(platform),
				).
				SetSortOrder(order.SortOrder).
				Save(txCtx)
			if err != nil {
				return fmt.Errorf("update smart schedule sort_order %d: %w", order.AccountID, err)
			}
			if n == 0 {
				return fmt.Errorf("smart schedule account %d not found", order.AccountID)
			}
		}
		return nil
	})
}

func (r *userSmartScheduleRepository) SetMemberPaused(ctx context.Context, userID, accountID int64, platform string, paused bool) error {
	if r == nil || r.client == nil {
		return fmt.Errorf("smart schedule repository unavailable")
	}
	platform = strings.TrimSpace(strings.ToLower(platform))
	if userID <= 0 || accountID <= 0 || platform == "" {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	client := clientFromContext(ctx, r.client)
	res, err := client.ExecContext(ctx, `
		UPDATE user_smart_schedule_accounts
		SET paused = $4
		WHERE user_id = $1 AND account_id = $2 AND platform = $3
	`, userID, accountID, platform, paused)
	if err != nil {
		return fmt.Errorf("set smart schedule paused: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set smart schedule paused rows: %w", err)
	}
	if n == 0 {
		return infraerrors.BadRequest("SMART_SCHEDULE_UNKNOWN_ACCOUNT", "account is not in this platform pool")
	}
	return nil
}

func (r *userSmartScheduleRepository) withTx(ctx context.Context, fn func(txCtx context.Context, client *dbent.Client) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(ctx, tx.Client())
	}
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin smart schedule transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx, tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit smart schedule transaction: %w", err)
	}
	return nil
}

func assembleSmartScheduleBundle(policies []*dbent.UserSmartSchedulePolicy, members []*dbent.UserSmartScheduleAccount) *service.UserSmartScheduleBundle {
	out := &service.UserSmartScheduleBundle{
		Policies: make(map[string]*service.SmartSchedulePlatformPolicy, len(service.AllowedQuotaPlatforms)),
	}
	for _, row := range policies {
		if row == nil {
			continue
		}
		policy := &service.SmartSchedulePlatformPolicy{
			Enabled:               row.Enabled,
			QualityMaxP50TTFTMs:   row.QualityMaxP50TtftMs,
			QualityMinSuccessRate: row.QualityMinSuccessRate,
			QualityCondition:      row.QualityCondition,
			CooldownMinutes:       row.CooldownMinutes,
			UpdatedAt:             row.UpdatedAt,
			AccountIDs:            map[int64]struct{}{},
			Caps:                  map[int64]int{},
			SortOrders:            map[int64]int{},
		}
		if row.QualityMaxP50TtftMs != nil || row.QualityMinSuccessRate != nil || row.QualityMinSuccessSamples != nil || row.QualityMinTtftSamples != nil {
			policy.QualityMinSuccessSamples = row.QualityMinSuccessSamples
			policy.QualityMinTTFTSamples = row.QualityMinTtftSamples
		}
		out.Policies[row.Platform] = policy
	}
	for _, row := range members {
		if row == nil {
			continue
		}
		policy := out.Policies[row.Platform]
		if policy == nil {
			policy = &service.SmartSchedulePlatformPolicy{
				CooldownMinutes: service.DefaultSmartScheduleCooldownMinutes,
				AccountIDs:      map[int64]struct{}{},
				Caps:            map[int64]int{},
				SortOrders:      map[int64]int{},
			}
			out.Policies[row.Platform] = policy
		}
		if policy.AccountIDs == nil {
			policy.AccountIDs = map[int64]struct{}{}
		}
		policy.AccountIDs[row.AccountID] = struct{}{}
		if row.MaxConcurrency != nil && *row.MaxConcurrency >= 1 {
			if policy.Caps == nil {
				policy.Caps = map[int64]int{}
			}
			policy.Caps[row.AccountID] = *row.MaxConcurrency
		}
		if row.SortOrder != nil {
			if policy.SortOrders == nil {
				policy.SortOrders = map[int64]int{}
			}
			policy.SortOrders[row.AccountID] = *row.SortOrder
		}
	}
	return out
}

func applySmartSchedulePolicyQualityUpdate(update *dbent.UserSmartSchedulePolicyUpdateOne, policy service.SmartSchedulePlatformWrite) {
	if policy.QualityMaxP50TTFTMs != nil {
		update.SetQualityMaxP50TtftMs(*policy.QualityMaxP50TTFTMs)
	} else {
		update.ClearQualityMaxP50TtftMs()
	}
	if policy.QualityMinSuccessRate != nil {
		update.SetQualityMinSuccessRate(*policy.QualityMinSuccessRate)
	} else {
		update.ClearQualityMinSuccessRate()
	}
	if policy.QualityMinSuccessSamples != nil {
		update.SetQualityMinSuccessSamples(*policy.QualityMinSuccessSamples)
	} else {
		update.ClearQualityMinSuccessSamples()
	}
	if policy.QualityMinTTFTSamples != nil {
		update.SetQualityMinTtftSamples(*policy.QualityMinTTFTSamples)
	} else {
		update.ClearQualityMinTtftSamples()
	}
	if policy.QualityCondition != nil && *policy.QualityCondition != "" {
		update.SetQualityCondition(*policy.QualityCondition)
	} else {
		update.ClearQualityCondition()
	}
}

func remainingPausedSmartScheduleIDs(pausedIDs []int64, members []service.SmartScheduleAccountMember) []int64 {
	if len(pausedIDs) == 0 {
		return nil
	}
	keepSet := make(map[int64]struct{}, len(members))
	for _, member := range members {
		if member.AccountID > 0 {
			keepSet[member.AccountID] = struct{}{}
		}
	}
	out := make([]int64, 0, len(pausedIDs))
	for _, accountID := range pausedIDs {
		if _, ok := keepSet[accountID]; ok {
			out = append(out, accountID)
		}
	}
	return out
}

func listPausedSmartScheduleAccountIDs(ctx context.Context, client *dbent.Client, userID int64, platform string) ([]int64, error) {
	if client == nil || userID <= 0 || platform == "" {
		return nil, nil
	}
	rows, err := client.QueryContext(ctx, `
		SELECT account_id
		FROM user_smart_schedule_accounts
		WHERE user_id = $1 AND platform = $2 AND paused = true
	`, userID, platform)
	if err != nil {
		return nil, fmt.Errorf("list paused smart schedule accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, fmt.Errorf("scan paused smart schedule account: %w", err)
		}
		if accountID > 0 {
			out = append(out, accountID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list paused smart schedule accounts: %w", err)
	}
	return out, nil
}

func restoreSmartSchedulePaused(ctx context.Context, client *dbent.Client, userID int64, platform string, accountIDs []int64) error {
	if client == nil || userID <= 0 || platform == "" || len(accountIDs) == 0 {
		return nil
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE user_smart_schedule_accounts
		SET paused = true
		WHERE user_id = $1 AND platform = $2 AND account_id = ANY($3)
	`, userID, platform, pq.Array(accountIDs)); err != nil {
		return fmt.Errorf("restore smart schedule paused: %w", err)
	}
	return nil
}

func overlaySmartSchedulePaused(
	ctx context.Context,
	client *dbent.Client,
	userIDs []int64,
	bundles map[int64]*service.UserSmartScheduleBundle,
) error {
	if client == nil || len(userIDs) == 0 || len(bundles) == 0 {
		return nil
	}
	rows, err := client.QueryContext(ctx, `
		SELECT user_id, account_id, platform
		FROM user_smart_schedule_accounts
		WHERE user_id = ANY($1) AND paused = true
	`, pq.Array(userIDs))
	if err != nil {
		return fmt.Errorf("overlay smart schedule paused: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID, accountID int64
		var platform string
		if err := rows.Scan(&userID, &accountID, &platform); err != nil {
			return fmt.Errorf("scan smart schedule paused: %w", err)
		}
		bundle := bundles[userID]
		if bundle == nil || bundle.Policies == nil || accountID <= 0 {
			continue
		}
		policy := bundle.Policies[platform]
		if policy == nil || !policy.HasAccount(accountID) {
			continue
		}
		if policy.Paused == nil {
			policy.Paused = map[int64]struct{}{}
		}
		policy.Paused[accountID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("overlay smart schedule paused: %w", err)
	}
	return nil
}

func writeSmartScheduleProbeConcurrency(ctx context.Context, client *dbent.Client, userID int64, platform string, policy service.SmartSchedulePlatformWrite) error {
	if client == nil || userID <= 0 || platform == "" {
		return nil
	}
	mode, custom, err := service.NormalizeProbeConcurrencyWrite(policy.ProbeConcurrencyMode, policy.ProbeConcurrency)
	if err != nil {
		return err
	}
	var storedCustom any
	if custom != nil {
		storedCustom = *custom
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE user_smart_schedule_policies
		SET probe_concurrency_mode = $3, probe_concurrency = $4
		WHERE user_id = $1 AND platform = $2
	`, userID, platform, mode, storedCustom); err != nil {
		return fmt.Errorf("write smart schedule probe concurrency: %w", err)
	}
	return nil
}

func overlaySmartScheduleProbeConcurrency(
	ctx context.Context,
	client *dbent.Client,
	userIDs []int64,
	bundles map[int64]*service.UserSmartScheduleBundle,
) error {
	if client == nil || len(userIDs) == 0 || len(bundles) == 0 {
		return nil
	}
	rows, err := client.QueryContext(ctx, `
		SELECT user_id, platform, probe_concurrency_mode, probe_concurrency
		FROM user_smart_schedule_policies
		WHERE user_id = ANY($1)
	`, pq.Array(userIDs))
	if err != nil {
		return fmt.Errorf("overlay smart schedule probe concurrency: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID int64
		var platform string
		var mode sql.NullString
		var custom sql.NullInt64
		if err := rows.Scan(&userID, &platform, &mode, &custom); err != nil {
			return fmt.Errorf("scan smart schedule probe concurrency: %w", err)
		}
		bundle := bundles[userID]
		if bundle == nil || bundle.Policies == nil {
			continue
		}
		policy := bundle.Policies[platform]
		if policy == nil {
			continue
		}
		var customPtr *int
		if custom.Valid {
			n := int(custom.Int64)
			customPtr = &n
		}
		policy.ProbeConcurrencyMode, policy.ProbeConcurrency = service.EchoProbeConcurrency(mode.String, customPtr)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("overlay smart schedule probe concurrency: %w", err)
	}
	return nil
}

func writeSmartScheduleLatencyGate(ctx context.Context, client *dbent.Client, userID int64, platform string, policy service.SmartSchedulePlatformWrite) error {
	if client == nil || userID <= 0 || platform == "" {
		return nil
	}
	var slow, consec, duration, schedN, schedSlow, schedConsec any
	if policy.QualityMaxSlowInWindow != nil {
		slow = *policy.QualityMaxSlowInWindow
	}
	if policy.QualityMaxConsecutiveSlow != nil {
		consec = *policy.QualityMaxConsecutiveSlow
	}
	if policy.QualityMaxP50DurationMs != nil {
		duration = *policy.QualityMaxP50DurationMs
	}
	if policy.QualitySchedWindowN != nil {
		schedN = *policy.QualitySchedWindowN
	}
	if policy.QualitySchedMaxSlowInWindow != nil {
		schedSlow = *policy.QualitySchedMaxSlowInWindow
	}
	if policy.QualitySchedMaxConsecutiveSlow != nil {
		schedConsec = *policy.QualitySchedMaxConsecutiveSlow
	}
	if _, err := client.ExecContext(ctx, `
		UPDATE user_smart_schedule_policies
		SET quality_max_slow_in_window = $3,
		    quality_max_consecutive_slow = $4,
		    quality_max_p50_duration_ms = $5,
		    quality_sched_window_n = $6,
		    quality_sched_max_slow_in_window = $7,
		    quality_sched_max_consecutive_slow = $8
		WHERE user_id = $1 AND platform = $2
	`, userID, platform, slow, consec, duration, schedN, schedSlow, schedConsec); err != nil {
		return fmt.Errorf("write smart schedule latency gate: %w", err)
	}
	return nil
}

func overlaySmartScheduleLatencyGate(
	ctx context.Context,
	client *dbent.Client,
	userIDs []int64,
	bundles map[int64]*service.UserSmartScheduleBundle,
) error {
	if client == nil || len(userIDs) == 0 || len(bundles) == 0 {
		return nil
	}
	rows, err := client.QueryContext(ctx, `
		SELECT user_id, platform, quality_max_slow_in_window, quality_max_consecutive_slow, quality_max_p50_duration_ms,
		       quality_sched_window_n, quality_sched_max_slow_in_window, quality_sched_max_consecutive_slow
		FROM user_smart_schedule_policies
		WHERE user_id = ANY($1)
	`, pq.Array(userIDs))
	if err != nil {
		return fmt.Errorf("overlay smart schedule latency gate: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var userID int64
		var platform string
		var slow, consec, duration, schedN, schedSlow, schedConsec sql.NullInt64
		if err := rows.Scan(&userID, &platform, &slow, &consec, &duration, &schedN, &schedSlow, &schedConsec); err != nil {
			return fmt.Errorf("scan smart schedule latency gate: %w", err)
		}
		bundle := bundles[userID]
		if bundle == nil || bundle.Policies == nil {
			continue
		}
		policy := bundle.Policies[platform]
		if policy == nil {
			continue
		}
		if slow.Valid {
			n := int(slow.Int64)
			policy.QualityMaxSlowInWindow = &n
		}
		if consec.Valid {
			n := int(consec.Int64)
			policy.QualityMaxConsecutiveSlow = &n
		}
		if duration.Valid {
			n := int(duration.Int64)
			policy.QualityMaxP50DurationMs = &n
		}
		if schedN.Valid {
			n := int(schedN.Int64)
			policy.QualitySchedWindowN = &n
		}
		if schedSlow.Valid {
			n := int(schedSlow.Int64)
			policy.QualitySchedMaxSlowInWindow = &n
		}
		if schedConsec.Valid {
			n := int(schedConsec.Int64)
			policy.QualitySchedMaxConsecutiveSlow = &n
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("overlay smart schedule latency gate: %w", err)
	}
	return nil
}
