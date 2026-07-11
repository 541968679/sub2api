//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupOpsStreamErrorTestQueue(t *testing.T, size int) {
	t.Helper()
	resetOpsErrorLoggerStateForTest(t)
	opsErrorLogOnce.Do(func() {})
	opsErrorLogMu.Lock()
	opsErrorLogQueue = make(chan opsErrorLogJob, size)
	opsErrorLogMu.Unlock()
}

func TestOpsStreamErrorSync_RecordsHTTP200InBandErrorExactlyOnce(t *testing.T) {
	setupOpsStreamErrorTestQueue(t, 4)
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set(opsModelKey, "test-model")
	service.MarkOpsStreamError(c, "rate_limit_error", "Concurrency limit exceeded", http.StatusTooManyRequests)
	service.MarkOpsStreamError(c, "upstream_error", "generic fallback", http.StatusBadGateway)

	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	logOpsStreamError(c, ops, http.StatusOK)

	require.Equal(t, int64(1), OpsErrorLogEnqueuedTotal())
	job := <-opsErrorLogQueue
	require.Equal(t, "rate_limit_error", job.entry.ErrorType)
	require.Equal(t, "Concurrency limit exceeded", job.entry.ErrorMessage)
	require.Equal(t, http.StatusOK, job.entry.StatusCode)
	require.Equal(t, "P1", job.entry.Severity)
	require.True(t, job.entry.Stream)
}

func TestOpsStreamErrorSync_DoesNotDuplicateUpstreamOrSkippedErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name  string
		setup func(*gin.Context)
	}{
		{
			name: "upstream context owns logging",
			setup: func(c *gin.Context) {
				service.MarkOpsStreamError(c, "upstream_error", "upstream failed", http.StatusBadGateway)
				service.SetOpsUpstreamError(c, http.StatusBadGateway, "upstream failed", "")
			},
		},
		{
			name: "skip monitoring",
			setup: func(c *gin.Context) {
				service.MarkOpsStreamError(c, "upstream_error", "expected failure", http.StatusBadGateway)
				c.Set(service.OpsSkipPassthroughKey, true)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupOpsStreamErrorTestQueue(t, 4)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			tt.setup(c)

			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			if _, hasUpstream := c.Get(service.OpsUpstreamStatusCodeKey); !hasUpstream {
				logOpsStreamError(c, ops, http.StatusOK)
			}

			require.Equal(t, int64(0), OpsErrorLogEnqueuedTotal())
		})
	}
}
