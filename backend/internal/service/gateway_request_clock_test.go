//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStampGatewayRequestStartAt_RequestClockIncludesPriorWait(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	requestStart := time.Now().Add(-80 * time.Millisecond)
	StampGatewayRequestStartAt(c, requestStart)
	hopStart := time.Now().Add(-10 * time.Millisecond)

	reqMs := requestElapsedMs(c, hopStart)
	hopMs := elapsedMsSince(hopStart)
	require.GreaterOrEqual(t, reqMs, hopMs)
	require.GreaterOrEqual(t, reqMs, 50)

	var first, hopFirst, trueMs *int
	stampRequestFirstTokenMs(&first, c, hopStart)
	stampHopFirstTokenMs(&hopFirst, hopStart)
	stampHopFirstTokenMs(&trueMs, hopStart)
	require.NotNil(t, first)
	require.NotNil(t, hopFirst)
	require.GreaterOrEqual(t, *first, *hopFirst)

	dur := durationMsForUsage(c.Request.Context(), 10*time.Millisecond)
	require.GreaterOrEqual(t, dur, *first)
	require.Equal(t, *first, ensureDurationCoversFirstToken(dur-1000, first))
}

func TestDurationMsForUsage_MissingStampUsesHop(t *testing.T) {
	t.Parallel()
	require.Equal(t, 12, durationMsForUsage(nil, 12*time.Millisecond))
}
