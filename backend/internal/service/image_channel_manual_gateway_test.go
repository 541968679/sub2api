package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestImageManualGatewayClientForwardsIndependentJSONRequestsToFixedLoopback(t *testing.T) {
	t.Parallel()

	type capturedRequest struct {
		path          string
		authorization string
		contentType   string
		requestID     string
		body          []byte
	}
	captured := make(chan capturedRequest, 2)
	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read loopback request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		captured <- capturedRequest{
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
			requestID:     r.Header.Get("X-Request-ID"),
			body:          body,
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Client-Request-ID", "gateway-client-request-1")
		w.Header().Add("X-Request-ID", "manual-run-1")
		w.Header().Add("X-Request-ID", "upstream-request-1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":1,"data":[{"url":"https://images.example/1.png"}]}`))
	}))

	client := newImageManualGatewayClient(imageManualLoopbackConfig(t, server))
	bodyOne := []byte(`{"model":"gpt-image-1","prompt":"first image"}`)
	bodyTwo := []byte(`{"model":"gpt-image-1","prompt":"second image"}`)

	responseOne, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/generations",
		APIKey:      "sk-manual-secret",
		ContentType: "application/json",
		Body:        bodyOne,
		Timeout:     30 * time.Second,
		RequestID:   "manual-run-1",
	})
	require.NoError(t, err)
	responseTwo, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/generations",
		APIKey:      "sk-manual-secret",
		ContentType: "application/json",
		Body:        bodyTwo,
		Timeout:     30 * time.Second,
		RequestID:   "manual-run-2",
	})
	require.NoError(t, err)

	first := <-captured
	second := <-captured
	require.Equal(t, "/v1/images/generations", first.path)
	require.Equal(t, "Bearer sk-manual-secret", first.authorization)
	require.Equal(t, "application/json", first.contentType)
	require.Equal(t, "manual-run-1", first.requestID)
	require.Equal(t, bodyOne, first.body)
	require.Equal(t, "manual-run-2", second.requestID)
	require.Equal(t, bodyTwo, second.body)
	require.NotEqual(t, first.body, second.body, "each manual run must carry its own complete request body")
	require.Equal(t, http.StatusCreated, responseOne.StatusCode)
	require.Equal(t, "gateway-client-request-1", responseOne.ClientRequestID)
	require.Equal(t, []string{"manual-run-1", "upstream-request-1"}, responseOne.RequestIDs,
		"X-Request-ID can contain both local and upstream values and must not be mislabeled")
	require.Equal(t, http.StatusCreated, responseTwo.StatusCode)
	require.NotContains(t, fmt.Sprintf("%+v", responseOne), "sk-manual-secret")
	require.NotContains(t, fmt.Sprintf("%+v", responseTwo), "sk-manual-secret")
}

func TestImageManualGatewayClientForwardsMultipartEditBodyWithoutRebuildingIt(t *testing.T) {
	t.Parallel()

	var receivedBody []byte
	var receivedContentType string
	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			t.Errorf("request path = %q, want /v1/images/edits", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		receivedContentType = r.Header.Get("Content-Type")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read multipart loopback request body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))

	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	require.NoError(t, writer.SetBoundary("manual-run-boundary"))
	require.NoError(t, writer.WriteField("model", "gpt-image-1"))
	require.NoError(t, writer.WriteField("prompt", "edit this unique image"))
	imagePart, err := writer.CreateFormFile("image", "input.png")
	require.NoError(t, err)
	_, err = imagePart.Write([]byte("\x89PNG\r\n\x1a\nunique-image-bytes"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	expectedBody := append([]byte(nil), multipartBody.Bytes()...)
	contentType := writer.FormDataContentType()

	client := newImageManualGatewayClient(imageManualLoopbackConfig(t, server))
	response, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/edits",
		APIKey:      "sk-edit-secret",
		ContentType: contentType,
		Body:        expectedBody,
		Timeout:     30 * time.Second,
		RequestID:   "manual-edit-1",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, contentType, receivedContentType)
	require.Equal(t, expectedBody, receivedBody, "the runner must forward this run's multipart bytes verbatim")
}

func TestImageManualGatewayClientKeepsGatewayHTTPErrorDiagnostics(t *testing.T) {
	t.Parallel()

	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Client-Request-ID", "manual-error-1")
		w.Header().Add("X-Request-ID", "manual-error-1")
		w.Header().Add("X-Request-ID", "upstream-error-request-1")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"no schedulable image account","type":"no_available_account","code":"NO_AVAILABLE_ACCOUNT","stage":"scheduler"}}`))
	}))

	client := newImageManualGatewayClient(imageManualLoopbackConfig(t, server))
	response, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/generations",
		APIKey:      "sk-diagnostic-secret",
		ContentType: "application/json",
		Body:        []byte(`{"model":"gpt-image-1","prompt":"diagnose"}`),
		Timeout:     30 * time.Second,
		RequestID:   "manual-error-1",
	})
	require.NoError(t, err, "an HTTP error is a completed gateway response, not a network transport failure")
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	require.Equal(t, "manual-error-1", response.ClientRequestID)
	require.Equal(t, []string{"manual-error-1", "upstream-error-request-1"}, response.RequestIDs)
	require.Equal(t, "no_available_account", response.ErrorType)
	require.Equal(t, "NO_AVAILABLE_ACCOUNT", response.ErrorCode)
	require.Equal(t, "scheduler", response.ErrorStage)
	require.Contains(t, string(response.Body), "no schedulable image account")
	require.NotContains(t, fmt.Sprintf("%+v", response), "sk-diagnostic-secret")
}

func TestImageManualGatewayClientRejectsRedirectWithoutLeakingAuthorization(t *testing.T) {
	t.Parallel()

	var redirectTargetHits atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetHits.Add(1)
		t.Errorf("loopback gateway client followed redirect to %s with Authorization=%q", r.URL, r.Header.Get("Authorization"))
	}))
	t.Cleanup(redirectTarget.Close)

	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL+"/must-not-be-called", http.StatusTemporaryRedirect)
	}))
	client := newImageManualGatewayClient(imageManualLoopbackConfig(t, server))

	response, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/generations",
		APIKey:      "sk-never-forward",
		ContentType: "application/json",
		Body:        []byte(`{"model":"gpt-image-1","prompt":"redirect"}`),
		Timeout:     30 * time.Second,
		RequestID:   "manual-redirect-1",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusTemporaryRedirect, response.StatusCode)
	require.Zero(t, redirectTargetHits.Load())
	require.NotContains(t, fmt.Sprintf("%+v", response), "sk-never-forward")
}

func TestImageManualGatewayClientRejectsNonGatewayTarget(t *testing.T) {
	t.Parallel()

	var loopbackHits atomic.Int32
	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loopbackHits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	client := newImageManualGatewayClient(imageManualLoopbackConfig(t, server))

	response, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "https://attacker.example/v1/images/generations",
		APIKey:      "sk-fixed-target-only",
		ContentType: "application/json",
		Body:        []byte(`{"model":"gpt-image-1","prompt":"must stay local"}`),
		Timeout:     30 * time.Second,
		RequestID:   "manual-invalid-target-1",
	})
	require.Error(t, err)
	require.Nil(t, response)
	require.Zero(t, loopbackHits.Load())
	require.NotContains(t, err.Error(), "sk-fixed-target-only")
}

func TestImageManualGatewayClientBoundsGatewayResponseBody(t *testing.T) {
	t.Parallel()

	server := newImageManualLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.CopyN(w, zeroReader{}, int64(imageMonitorMaxResponseBytes)+1)
	}))
	cfg := imageManualLoopbackConfig(t, server)
	cfg.Gateway.UpstreamResponseReadMaxBytes = 1024
	client := newImageManualGatewayClient(cfg)

	response, err := client.Do(context.Background(), ImageManualGatewayRequest{
		Path:        "/v1/images/generations",
		APIKey:      "sk-large-response",
		ContentType: "application/json",
		Body:        []byte(`{"model":"gpt-image-1","prompt":"large response"}`),
		Timeout:     30 * time.Second,
		RequestID:   "manual-large-1",
	})
	require.Error(t, err)
	if response != nil {
		require.LessOrEqual(t, len(response.Body), 1024)
		require.NotContains(t, fmt.Sprintf("%+v", response), "sk-large-response")
	}
	require.NotContains(t, err.Error(), "sk-large-response")
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func newImageManualLoopbackTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.Start()
	t.Cleanup(server.Close)
	host, _, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", host)
	return server
}

func imageManualLoopbackConfig(t *testing.T, server *httptest.Server) *config.Config {
	t.Helper()
	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(server.URL, "http://127.0.0.1:"))
	return &config.Config{Server: config.ServerConfig{Port: port}}
}
