//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestTurnUsageRecordContext_DistinctPerTurn(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "conn-uuid")
	parent = context.WithValue(parent, ctxkey.RequestID, "local-id")

	turn1 := turnUsageRecordContext(parent, 1, "resp_a")
	turn2 := turnUsageRecordContext(parent, 2, "resp_b")

	cid1, _ := turn1.Value(ctxkey.ClientRequestID).(string)
	cid2, _ := turn2.Value(ctxkey.ClientRequestID).(string)
	require.Equal(t, "conn-uuid:turn:resp_a", cid1)
	require.Equal(t, "conn-uuid:turn:resp_b", cid2)
	require.NotEqual(t, cid1, cid2)

	rid1, _ := turn1.Value(ctxkey.RequestID).(string)
	rid2, _ := turn2.Value(ctxkey.RequestID).(string)
	require.Equal(t, "local-id:turn:resp_a", rid1)
	require.Equal(t, "local-id:turn:resp_b", rid2)
}

func TestTurnUsageRecordContext_FallsBackToTurnNumber(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "conn-uuid")

	turn3 := turnUsageRecordContext(parent, 3, "")
	cid, _ := turn3.Value(ctxkey.ClientRequestID).(string)
	require.Equal(t, "conn-uuid:turn:3", cid)
}

func TestTurnUsageRecordContext_NoIDsPassthrough(t *testing.T) {
	parent := context.Background()
	derived := turnUsageRecordContext(parent, 1, "resp_a")
	require.Nil(t, derived.Value(ctxkey.ClientRequestID))
	require.Nil(t, derived.Value(ctxkey.RequestID))

	require.Nil(t, turnUsageRecordContext(nil, 1, "resp_a"))
}
