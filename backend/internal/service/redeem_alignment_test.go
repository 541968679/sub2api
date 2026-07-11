//go:build unit

package service

import (
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRedeemCodeExpiryAlignment(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	require.True(t, (&RedeemCode{Status: StatusUnused, ExpiresAt: &past}).IsExpiredAt(now))
	require.False(t, (&RedeemCode{Status: StatusUnused, ExpiresAt: &past}).CanUse())
	require.True(t, (&RedeemCode{Status: StatusUnused, ExpiresAt: &future}).CanUse())
	require.False(t, (&RedeemCode{Status: StatusUsed, ExpiresAt: &past}).IsExpiredAt(now))
}

func TestRedeemBatchUpdateRejectsCoreFields(t *testing.T) {
	codeType := RedeemTypeBalance
	input := &RedeemCodeBatchUpdateInput{
		IDs: []int64{1},
		Fields: RedeemCodeBatchUpdateFields{
			Type: &codeType,
		},
	}

	_, err := (&RedeemService{}).BatchUpdate(t.Context(), input)
	require.Error(t, err)
	require.Equal(t, "REDEEM_CODE_CORE_FIELDS_IMMUTABLE", infraerrors.Reason(err))
}

func TestInvitationRedeemErrorIsBadRequest(t *testing.T) {
	err := unsupportedRedeemTypeError(RedeemTypeInvitation)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "REDEEM_CODE_UNSUPPORTED_TYPE", infraerrors.Reason(err))
}

func TestBatchLimitFieldsRemainAvailable(t *testing.T) {
	batchID := "batch-1"
	code := RedeemCode{BatchID: &batchID, BatchRedeemLimitPerUser: true}
	require.Equal(t, batchID, *code.BatchID)
	require.True(t, code.BatchRedeemLimitPerUser)
}
