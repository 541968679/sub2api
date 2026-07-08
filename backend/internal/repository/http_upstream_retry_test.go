package repository

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type staticGatewayRetrySettings struct {
	max int
}

func (s staticGatewayRetrySettings) GetGatewayNetworkRetryMax(context.Context) int {
	return s.max
}

func TestHTTPUpstreamNetworkRetryRetriesTransportError(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&attempts, 1) <= 2 {
			closeRetryTestConnection(t, w)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(upstream.Close)

	svc := NewHTTPUpstream(retryTestConfig(), staticGatewayRetrySettings{max: 2})
	req, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader("payload"))
	require.NoError(t, err)

	resp, err := svc.Do(req, "", 1, 1)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "ok", string(body))
	require.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestHTTPUpstreamNetworkRetryStopsAtConfiguredMax(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		closeRetryTestConnection(t, w)
	}))
	t.Cleanup(upstream.Close)

	svc := NewHTTPUpstream(retryTestConfig(), staticGatewayRetrySettings{max: 1})
	req, err := http.NewRequest(http.MethodPost, upstream.URL, strings.NewReader("payload"))
	require.NoError(t, err)

	resp, err := svc.Do(req, "", 1, 1)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(&attempts))
}

func TestHTTPUpstreamNetworkRetryDoesNotRetryHTTPResponse(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream.Close)

	svc := NewHTTPUpstream(retryTestConfig(), staticGatewayRetrySettings{max: 2})
	req, err := http.NewRequest(http.MethodGet, upstream.URL, nil)
	require.NoError(t, err)

	resp, err := svc.Do(req, "", 1, 1)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestHTTPUpstreamNetworkRetryRequiresReplayableBody(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		closeRetryTestConnection(t, w)
	}))
	t.Cleanup(upstream.Close)

	svc := NewHTTPUpstream(retryTestConfig(), staticGatewayRetrySettings{max: 2})
	req, err := http.NewRequest(http.MethodPost, upstream.URL, io.NopCloser(strings.NewReader("payload")))
	require.NoError(t, err)
	require.Nil(t, req.GetBody)

	resp, err := svc.Do(req, "", 1, 1)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestHTTPUpstreamNetworkRetryCanBeDisabledByContext(t *testing.T) {
	var attempts int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		closeRetryTestConnection(t, w)
	}))
	t.Cleanup(upstream.Close)

	svc := NewHTTPUpstream(retryTestConfig(), staticGatewayRetrySettings{max: 2})
	ctx := service.WithHTTPUpstreamNetworkRetryDisabled(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader("payload"))
	require.NoError(t, err)

	resp, err := svc.Do(req, "", 1, 1)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestRetriableUpstreamNetworkErrorClassifier(t *testing.T) {
	require.True(t, isRetriableUpstreamNetworkError(io.ErrUnexpectedEOF))
	require.True(t, isRetriableUpstreamNetworkError(errors.New("Network error. Please check your connection.")))
	require.False(t, isRetriableUpstreamNetworkError(context.Canceled))
	require.False(t, isRetriableUpstreamNetworkError(errors.New("upstream returned status 400")))
}

func retryTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				AllowPrivateHosts: true,
			},
		},
	}
}

func closeRetryTestConnection(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		t.Errorf("expected httptest response writer to support hijacking")
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		t.Errorf("hijack response writer: %v", err)
		return
	}
	_ = conn.Close()
}
