//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type opsProjectionAccountRepo struct {
	AccountRepository
	platform string
	groupID  *int64
	accounts []Account
}

func (r *opsProjectionAccountRepo) ListOpsAccountsForStats(_ context.Context, platform string, groupID *int64) ([]Account, error) {
	r.platform = platform
	r.groupID = groupID
	return append([]Account(nil), r.accounts...), nil
}

func TestListAllAccountsForOps_UsesFilteredLightweightProjection(t *testing.T) {
	groupID := int64(42)
	repo := &opsProjectionAccountRepo{accounts: []Account{{ID: 7, Platform: PlatformOpenAI}}}
	svc := &OpsService{accountRepo: repo}

	accounts, err := svc.listAllAccountsForOps(context.Background(), PlatformOpenAI, &groupID)

	require.NoError(t, err)
	require.Equal(t, PlatformOpenAI, repo.platform)
	require.NotNil(t, repo.groupID)
	require.Equal(t, groupID, *repo.groupID)
	require.Equal(t, []Account{{ID: 7, Platform: PlatformOpenAI}}, accounts)
}
