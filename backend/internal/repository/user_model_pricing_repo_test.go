//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserModelPricingBatchUpsertSQLUpdates1hFieldsOnConflict(t *testing.T) {
	require.Contains(t, userModelPricingBatchUpsertSQL, "cache_write_1h_price = EXCLUDED.cache_write_1h_price")
	require.Contains(t, userModelPricingBatchUpsertSQL, "display_cache_creation_1h_price = EXCLUDED.display_cache_creation_1h_price")
}

func TestUserModelPricingRepositoryBatchUpsertExecutes1hConflictUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewUserModelPricingRepository(db)
	oneH := 1.5
	mock.ExpectExec("INSERT INTO user_model_pricing_overrides").
		WithArgs(
			int64(42), "claude-opus-4-8",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			true, "", sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.BatchUpsert(context.Background(), 42, []service.UserModelPricingOverride{{
		Model:                       "claude-opus-4-8",
		CacheWrite1hPrice:           &oneH,
		DisplayCacheCreation1hPrice: &oneH,
		Enabled:                     true,
	}})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.True(t, strings.Contains(userModelPricingBatchUpsertSQL, "ON CONFLICT (user_id, model)"))
}
