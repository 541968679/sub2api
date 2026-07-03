package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type imageMonitorAccountReaderStub struct {
	account *Account
	err     error
}

func (s *imageMonitorAccountReaderStub) GetByID(context.Context, int64) (*Account, error) {
	return s.account, s.err
}

type imageMonitorProxyReaderStub struct {
	proxy *Proxy
	err   error
}

func (s *imageMonitorProxyReaderStub) GetByID(context.Context, int64) (*Proxy, error) {
	return s.proxy, s.err
}

type imageMonitorHTTPUpstreamRecorder struct {
	statusCode  int
	body        string
	req         *http.Request
	requestBody []byte
	proxyURL    string
	accountID   int64
	concurrency int
}

func (r *imageMonitorHTTPUpstreamRecorder) Do(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
) (*http.Response, error) {
	return r.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (r *imageMonitorHTTPUpstreamRecorder) DoWithTLS(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
	_ *tlsfingerprint.Profile,
) (*http.Response, error) {
	r.req = req
	r.proxyURL = proxyURL
	r.accountID = accountID
	r.concurrency = accountConcurrency
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	r.requestBody = body
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

func TestImageChannelMonitorRunCheckUsesOpenAIAPIKeyAccountSource(t *testing.T) {
	accountID := int64(7)
	account := &Account{
		ID:          accountID,
		Name:        "openai-image",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 3,
		Credentials: map[string]any{
			"api_key":    "acct-key",
			"base_url":   "https://upstream.example/custom/v1",
			"user_agent": "custom-ua",
		},
	}
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"url":"https://cdn.example/generated.png","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(
		nil,
		&imageMonitorAccountReaderStub{account: account},
		nil,
		nil,
		upstream,
		nil,
	)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             12,
		SourceType:     ImageChannelMonitorSourceAccount,
		AccountID:      &accountID,
		Model:          "gpt-image-1",
		Prompt:         "draw a square",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "https://upstream.example/custom/v1/images/generations", upstream.req.URL.String())
	require.Equal(t, "Bearer acct-key", upstream.req.Header.Get("Authorization"))
	require.Equal(t, "custom-ua", upstream.req.Header.Get("User-Agent"))
	require.Equal(t, accountID, upstream.accountID)
	require.Equal(t, 3, upstream.concurrency)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.requestBody, &payload))
	require.Equal(t, "gpt-image-1", payload["model"])
	require.Equal(t, "draw a square", payload["prompt"])
	require.Equal(t, "url", payload["response_format"])
}

func TestImageChannelMonitorRunCheckMarksB64JSONAsFailedForURLMonitor(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"b64_json":"aGVhbHRoLWNoZWNr","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             13,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusFailed, result.Status)
	require.False(t, result.HasURL)
	require.True(t, result.HasB64JSON)
	require.Equal(t, "image_url", result.ErrorStage)
	require.Contains(t, result.Message, "b64_json")
	require.Equal(t, "Bearer custom-key", upstream.req.Header.Get("Authorization"))
}

func TestImageChannelMonitorRunCheckUsesCustomProxy(t *testing.T) {
	proxyID := int64(9)
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"url":"https://cdn.example/generated.png","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(
		nil,
		nil,
		&imageMonitorProxyReaderStub{proxy: &Proxy{
			ID:       proxyID,
			Name:     "image-proxy",
			Protocol: "http",
			Host:     "proxy.example",
			Port:     8080,
		}},
		nil,
		upstream,
		nil,
	)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             14,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		ProxyID:        &proxyID,
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "http://proxy.example:8080", upstream.proxyURL)
}
