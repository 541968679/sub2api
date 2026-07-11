//go:build unit

package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsOpsRealtimeRequestCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func(ctx context.Context) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/realtime", nil).WithContext(ctx)
		return c
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  *gin.Context
		err  error
		want bool
	}{
		{name: "standard cancellation", ctx: newContext(context.Background()), err: context.Canceled, want: true},
		{name: "request context cancellation", ctx: newContext(canceledCtx), err: errors.New("query interrupted"), want: true},
		{name: "postgres cancellation", ctx: newContext(context.Background()), err: errors.New("pq: canceling statement due to user request"), want: true},
		{name: "ordinary failure", ctx: newContext(context.Background()), err: errors.New("redis unavailable"), want: false},
		{name: "nil error", ctx: newContext(context.Background()), err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpsRealtimeRequestCanceled(tt.ctx, tt.err))
		})
	}
}
