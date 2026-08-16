package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/usersmartscheduleaccount"
	"github.com/Wei-Shaw/sub2api/ent/usersmartschedulepolicy"
	"github.com/Wei-Shaw/sub2api/internal/service"
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
	return assembleSmartScheduleBundle(policies, members), nil
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
		if _, err := client.UserSmartScheduleAccount.Delete().
			Where(
				usersmartscheduleaccount.UserIDEQ(userID),
				usersmartscheduleaccount.PlatformEQ(platform),
			).
			Exec(txCtx); err != nil {
			return fmt.Errorf("clear smart schedule accounts: %w", err)
		}
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
		out.Policies[row.Platform] = &service.SmartSchedulePlatformPolicy{
			Enabled:                  row.Enabled,
			QualityMaxP50TTFTMs:      row.QualityMaxP50TtftMs,
			QualityMinSuccessRate:    row.QualityMinSuccessRate,
			QualityMinSuccessSamples: row.QualityMinSuccessSamples,
			QualityMinTTFTSamples:    row.QualityMinTtftSamples,
			QualityCondition:         row.QualityCondition,
			CooldownMinutes:          row.CooldownMinutes,
			UpdatedAt:                row.UpdatedAt,
			AccountIDs:               map[int64]struct{}{},
			Caps:                     map[int64]int{},
			SortOrders:               map[int64]int{},
		}
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
