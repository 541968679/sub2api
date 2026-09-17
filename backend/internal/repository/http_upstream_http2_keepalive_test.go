package repository

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func http2KeepAliveTestPoolSettings() poolSettings {
	return poolSettings{
		maxIdleConns:          10,
		maxIdleConnsPerHost:   5,
		maxConnsPerHost:       10,
		idleConnTimeout:       90 * time.Second,
		responseHeaderTimeout: time.Minute,
	}
}

func requireHTTP2Configured(t *testing.T, tr *http.Transport, msg string) {
	t.Helper()
	require.NotNil(t, tr, msg)
	if tr.Protocols != nil {
		require.True(t, tr.Protocols.HTTP2(), msg)
	}
}

func TestEnableHTTP2KeepAlive_EnablesPingHealthCheck(t *testing.T) {
	tr := &http.Transport{}

	h2, err := enableHTTP2KeepAlive(tr)
	require.NoError(t, err)
	require.NotNil(t, h2, "must return a configured *http2.Transport")

	require.Positive(t, h2.ReadIdleTimeout, "idle PING must be enabled")
	require.Equal(t, longStreamHTTP2ReadIdleTimeout, h2.ReadIdleTimeout)
	require.Equal(t, longStreamHTTP2PingTimeout, h2.PingTimeout, "PING timeout must be set")
	requireHTTP2Configured(t, tr, "http2 must be attached to the http.Transport")
}

func TestBuildUpstreamTransport_LongStreamH2_EnablesPingHealthCheck(t *testing.T) {
	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeLongStreamH2)
	require.NoError(t, err)
	require.True(t, tr.ForceAttemptHTTP2, "long_stream_h2 must enable HTTP/2")
	requireHTTP2Configured(t, tr, "long_stream_h2 must configure http2 keepalive")
}

func TestBuildUpstreamTransport_NonLongStreamH2_NotEagerlyConfigured(t *testing.T) {
	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeDefault)
	require.NoError(t, err)
	require.Nil(t, tr.Protocols, "default mode must not eagerly configure http2 keepalive")
	require.Nil(t, tr.TLSNextProto["h2"], "default mode must not eagerly configure http2 keepalive")
}

func TestBuildUpstreamTransport_LongStreamH2_NegotiatesHTTP2(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), nil, upstreamProtocolModeLongStreamH2)
	require.NoError(t, err)
	defer tr.CloseIdleConnections()
	require.NotNil(t, tr.TLSClientConfig)
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	tr.TLSClientConfig.RootCAs = roots

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 2, resp.ProtoMajor, "long_stream_h2 must negotiate HTTP/2")
}

func TestBuildUpstreamTransport_LongStreamH2_WithHTTPProxy_EnablesKeepAlive(t *testing.T) {
	proxyURL, err := url.Parse("http://127.0.0.1:18080")
	require.NoError(t, err)

	tr, err := buildUpstreamTransport(http2KeepAliveTestPoolSettings(), proxyURL, upstreamProtocolModeLongStreamH2)
	require.NoError(t, err)
	require.True(t, tr.ForceAttemptHTTP2)
	requireHTTP2Configured(t, tr, "proxied long_stream_h2 must enable http2 keepalive")
	require.NotNil(t, tr.Proxy, "HTTP proxy must still be set on Transport.Proxy")
}
