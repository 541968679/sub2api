package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *adminServiceImpl) CreateShadow(ctx context.Context, parentID int64, opts ShadowOptions) (*Account, error) {
	parent, err := s.accountRepo.GetByID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("get parent account: %w", err)
	}
	if !parent.IsOpenAIOAuth() || parent.IsShadow() {
		return nil, infraerrors.New(http.StatusBadRequest, "SPARK_SHADOW_INVALID_PARENT", "spark shadow requires a real OpenAI OAuth parent account")
	}
	if shadows, err := listAccountShadows(ctx, s.accountRepo, parentID); err != nil {
		return nil, fmt.Errorf("check existing spark shadows: %w", err)
	} else if len(shadows) > 0 {
		return nil, infraerrors.New(http.StatusConflict, "SPARK_SHADOW_ALREADY_EXISTS", "parent account already has a spark shadow account")
	}
	groupIDs := append([]int64(nil), opts.GroupIDs...)
	if len(groupIDs) > 0 && s.groupRepo != nil {
		if err := s.validateGroupIDsExist(ctx, groupIDs); err != nil {
			return nil, err
		}
	} else if len(groupIDs) == 0 && len(parent.GroupIDs) > 0 {
		groupIDs = append(groupIDs, parent.GroupIDs...)
	} else if len(groupIDs) == 0 && s.groupRepo != nil {
		groups, err := s.groupRepo.ListActiveByPlatform(ctx, PlatformOpenAI)
		if err == nil {
			for _, group := range groups {
				if group.Name == PlatformOpenAI+"-default" {
					groupIDs = []int64{group.ID}
					break
				}
			}
		}
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = parent.Name + " (Spark)"
	}
	if runes := []rune(name); len(runes) > 100 {
		name = string(runes[:100])
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = parent.Concurrency
	}
	priority := opts.Priority
	if priority <= 0 {
		priority = parent.Priority
	}
	shadow := &Account{Name: name, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive,
		Credentials: map[string]any{"model_mapping": defaultSparkShadowModelMapping()}, ParentAccountID: &parentID,
		QuotaDimension: QuotaDimensionSpark, ProxyID: parent.ProxyID, Priority: priority, Concurrency: concurrency, Schedulable: true}
	if err := s.accountRepo.Create(ctx, shadow); err != nil {
		if existing, qerr := listAccountShadows(ctx, s.accountRepo, parentID); qerr == nil && len(existing) > 0 {
			return nil, infraerrors.New(http.StatusConflict, "SPARK_SHADOW_ALREADY_EXISTS", "parent account already has a spark shadow account")
		}
		return nil, fmt.Errorf("create spark shadow: %w", err)
	}
	if len(groupIDs) > 0 {
		if err := s.accountRepo.BindGroups(ctx, shadow.ID, groupIDs); err != nil {
			if delErr := s.accountRepo.Delete(context.WithoutCancel(ctx), shadow.ID); delErr != nil {
				slog.Error("spark_shadow_bind_groups_rollback_failed", "shadow_id", shadow.ID, "error", delErr)
			}
			return nil, fmt.Errorf("bind groups for spark shadow: %w", err)
		}
		shadow.GroupIDs = groupIDs
	}
	return shadow, nil
}

func propagateAccountProxyToShadows(ctx context.Context, repo AccountRepository, parentID int64, proxyID *int64) error {
	shadows, err := listAccountShadows(ctx, repo, parentID)
	if err != nil {
		return err
	}
	for _, shadow := range shadows {
		shadow.ProxyID = proxyID
		if err := repo.Update(ctx, shadow); err != nil {
			return err
		}
	}
	return nil
}

func validateSparkShadowAccountUpdate(ctx context.Context, repo AccountRepository, account *Account, input *UpdateAccountInput) error {
	if account == nil || input == nil {
		return nil
	}
	if account.IsShadow() {
		if input.Type != "" && input.Type != AccountTypeOAuth {
			return infraerrors.BadRequest("SPARK_SHADOW_TYPE_IMMUTABLE", "spark shadow type must remain oauth")
		}
		if !isAllowedSparkShadowCredentialsUpdate(input.Credentials) {
			return infraerrors.BadRequest("SPARK_SHADOW_CREDENTIALS_FORBIDDEN", "spark shadow cannot store authentication credentials")
		}
		return nil
	}
	shadows, err := listAccountShadows(ctx, repo, account.ID)
	if err != nil {
		return err
	}
	if len(shadows) > 0 && input.Type != "" && input.Type != AccountTypeOAuth {
		return infraerrors.BadRequest("SPARK_SHADOW_PARENT_TYPE_IMMUTABLE", "spark shadow parent must remain OpenAI OAuth")
	}
	return nil
}

func deleteAccountWithShadows(ctx context.Context, repo AccountRepository, id int64) error {
	shadows, err := listAccountShadows(ctx, repo, id)
	if err != nil {
		return err
	}
	for _, shadow := range shadows {
		if err := repo.Delete(ctx, shadow.ID); err != nil {
			return err
		}
	}
	return repo.Delete(ctx, id)
}
