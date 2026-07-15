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
	require.Equal(t, "conn-uuid:t:1-resp_a", cid1)
	require.Equal(t, "conn-uuid:t:2-resp_b", cid2)
	require.NotEqual(t, cid1, cid2)

	rid1, _ := turn1.Value(ctxkey.RequestID).(string)
	rid2, _ := turn2.Value(ctxkey.RequestID).(string)
	require.Equal(t, "local-id:t:1-resp_a", rid1)
	require.Equal(t, "local-id:t:2-resp_b", rid2)
}

func TestTurnUsageRecordContext_FallsBackToTurnNumber(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "conn-uuid")

	turn3 := turnUsageRecordContext(parent, 3, "")
	cid, _ := turn3.Value(ctxkey.ClientRequestID).(string)
	require.Equal(t, "conn-uuid:t:3", cid)
}

func TestTurnUsageRecordContext_NoIDsPassthrough(t *testing.T) {
	parent := context.Background()
	derived := turnUsageRecordContext(parent, 1, "resp_a")
	require.Nil(t, derived.Value(ctxkey.ClientRequestID))
	require.Nil(t, derived.Value(ctxkey.RequestID))

	require.Nil(t, turnUsageRecordContext(nil, 1, "resp_a"))
}

func TestTurnUsageRecordContext_KeepsUsageLogRequestIDWithinVarchar64(t *testing.T) {
	// Mirrors resolveUsageBillingRequestID: "local:" + requestID
	parentUUID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" // 36
	parent := context.WithValue(context.Background(), ctxkey.RequestID, parentUUID)
	upstreamUUID := "71cf60d9-784d-9cae-ab2f-e47a15afaa3f" // 36

	derived := turnUsageRecordContext(parent, 1, upstreamUUID)
	rid, _ := derived.Value(ctxkey.RequestID).(string)
	stored := "local:" + rid
	require.LessOrEqual(t, len(stored), 64, "stored=%q len=%d", stored, len(stored))
	require.Contains(t, rid, ":t:1-")
	require.True(t, len(rid) < len(parentUUID+":turn:"+upstreamUUID))
}
