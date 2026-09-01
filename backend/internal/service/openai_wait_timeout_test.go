//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blockingHeaderHTTPUpstream struct{}

func (u *blockingHeaderHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func (u *blockingHeaderHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

type openaiWaitTimeoutSettingRepo struct {
	data map[string]string
}

func (r *openaiWaitTimeoutSettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	if r.data != nil {
		if value, ok := r.data[key]; ok {
			return &Setting{Key: key, Value: value}, nil
		}
	}
	return nil, ErrSettingNotFound
}

func (r *openaiWaitTimeoutSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.data != nil {
		if value, ok := r.data[key]; ok {
			return value, nil
		}
	}
	return "", ErrSettingNotFound
}

func (r *openaiWaitTimeoutSettingRepo) Set(_ context.Context, key, value string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	r.data[key] = value
	return nil
}

func (r *openaiWaitTimeoutSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if r.data != nil {
			if value, ok := r.data[key]; ok {
				out[key] = value
			}
		}
	}
	return out, nil
}

func (r *openaiWaitTimeoutSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	for key, value := range settings {
		r.data[key] = value
	}
	return nil
}

func (r *openaiWaitTimeoutSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.data))
	for key, value := range r.data {
		out[key] = value
	}
	return out, nil
}

func (r *openaiWaitTimeoutSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.data, key)
	return nil
}

func TestNormalizeOpenAIWaitTimeoutSettings(t *testing.T) {
	t.Parallel()
	zero := NormalizeOpenAIWaitTimeoutSettings(OpenAIWaitTimeoutSettings{})
	require.Equal(t, 0, zero.HeaderWaitSeconds)
	require.Equal(t, 0, zero.FirstUsefulFrameSeconds)

	ok := NormalizeOpenAIWaitTimeoutSettings(OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 120, FirstUsefulFrameSeconds: 15})
	require.Equal(t, 120, ok.HeaderWaitSeconds)
	require.Equal(t, 15, ok.FirstUsefulFrameSeconds)

	bad := NormalizeOpenAIWaitTimeoutSettings(OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 3, FirstUsefulFrameSeconds: 400})
	require.Equal(t, DefaultOpenAIHeaderWaitSeconds, bad.HeaderWaitSeconds)
	require.Equal(t, DefaultOpenAIFirstUsefulFrameSeconds, bad.FirstUsefulFrameSeconds)

	omitted := parseOpenAIWaitTimeoutSettingsJSON("{}")
	require.Equal(t, DefaultOpenAIHeaderWaitSeconds, omitted.HeaderWaitSeconds)
	require.Equal(t, DefaultOpenAIFirstUsefulFrameSeconds, omitted.FirstUsefulFrameSeconds)

	explicitZero := parseOpenAIWaitTimeoutSettingsJSON(`{"header_wait_seconds":0,"first_useful_frame_seconds":0}`)
	require.Equal(t, 0, explicitZero.HeaderWaitSeconds)
	require.Equal(t, 0, explicitZero.FirstUsefulFrameSeconds)
}

func TestGetSetOpenAIWaitTimeoutSettings_DefaultsAndRoundTrip(t *testing.T) {
	invalidateOpenAIWaitTimeoutSettingsCache()
	t.Cleanup(invalidateOpenAIWaitTimeoutSettingsCache)

	svc := NewSettingService(&openaiWaitTimeoutSettingRepo{}, &config.Config{})
	got, err := svc.GetOpenAIWaitTimeoutSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultOpenAIHeaderWaitSeconds, got.HeaderWaitSeconds)
	require.Equal(t, DefaultOpenAIFirstUsefulFrameSeconds, got.FirstUsefulFrameSeconds)

	require.Error(t, svc.SetOpenAIWaitTimeoutSettings(context.Background(), &OpenAIWaitTimeoutSettings{
		HeaderWaitSeconds:       3,
		FirstUsefulFrameSeconds: 30,
	}))

	require.NoError(t, svc.SetOpenAIWaitTimeoutSettings(context.Background(), &OpenAIWaitTimeoutSettings{
		HeaderWaitSeconds:       120,
		FirstUsefulFrameSeconds: 10,
	}))
	got, err = svc.GetOpenAIWaitTimeoutSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 120, got.HeaderWaitSeconds)
	require.Equal(t, 10, got.FirstUsefulFrameSeconds)
}

func TestGetOpenAIWaitTimeoutSettings_BadJSONUsesDefaults(t *testing.T) {
	invalidateOpenAIWaitTimeoutSettingsCache()
	t.Cleanup(invalidateOpenAIWaitTimeoutSettingsCache)

	svc := NewSettingService(&openaiWaitTimeoutSettingRepo{data: map[string]string{
		SettingKeyOpenAIWaitTimeoutSettings: "{not-json",
	}}, &config.Config{})
	got, err := svc.GetOpenAIWaitTimeoutSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultOpenAIHeaderWaitSeconds, got.HeaderWaitSeconds)
	require.Equal(t, DefaultOpenAIFirstUsefulFrameSeconds, got.FirstUsefulFrameSeconds)
}

func TestDoOpenAIUpstreamWithHeaderWait_Timeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	override := OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 1, FirstUsefulFrameSeconds: 0}
	openAIWaitTimeoutSettingsOverride = &override
	t.Cleanup(func() { openAIWaitTimeoutSettingsOverride = nil })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	svc := &OpenAIGatewayService{httpUpstream: &blockingHeaderHTTPUpstream{}}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.invalid", nil)
	require.NoError(t, err)

	started := time.Now()
	resp, doErr := svc.doOpenAIUpstreamWithHeaderWait(c.Request.Context(), c, &Account{ID: 9, Platform: PlatformOpenAI}, req, "", false, "gpt-5.4")
	require.Nil(t, resp)
	var failover *UpstreamFailoverError
	require.True(t, errors.As(doErr, &failover))
	require.Equal(t, http.StatusBadGateway, failover.StatusCode)
	requireClientHidesWaitTimeout(t, string(failover.ResponseBody))
	require.Contains(t, string(failover.ResponseBody), OpenAIWaitTimeoutClientMessage())
	require.Contains(t, string(failover.RawUpstreamBody), OpenAIHeaderWaitTimeoutMarker)
	require.True(t, IsOpenAIWaitTimeoutOpsError("", "", string(failover.RawUpstreamBody)))
	require.Contains(t, opsContextString(c, OpsUpstreamErrorMessageKey), OpenAIHeaderWaitTimeoutMarker)
	require.Equal(t, 0, rec.Body.Len())
	require.GreaterOrEqual(t, time.Since(started), time.Second)
	require.Less(t, time.Since(started), 4*time.Second)
}

func TestDoOpenAIUpstreamWithHeaderWait_ClientCancelNotTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	override := OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 5, FirstUsefulFrameSeconds: 0}
	openAIWaitTimeoutSettingsOverride = &override
	t.Cleanup(func() { openAIWaitTimeoutSettingsOverride = nil })

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	req, err := http.NewRequestWithContext(parent, http.MethodPost, "http://example.invalid", nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	svc := &OpenAIGatewayService{httpUpstream: &blockingHeaderHTTPUpstream{}}
	resp, doErr := svc.doOpenAIUpstreamWithHeaderWait(context.Background(), c, &Account{ID: 9, Platform: PlatformOpenAI}, req, "", false, "gpt-5.4")
	require.Nil(t, resp)
	require.ErrorIs(t, doErr, context.Canceled)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(doErr, &failover))
}

func TestHandleStreamingResponse_FirstUsefulFrameTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	override := OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 0, FirstUsefulFrameSeconds: 1}
	openAIWaitTimeoutSettingsOverride = &override
	t.Cleanup(func() { openAIWaitTimeoutSettingsOverride = nil })

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 0, StreamKeepaliveInterval: 0, MaxLineSize: defaultMaxLineSize},
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{}}
	go func() {
		_, _ = pw.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\"}}\n\n"))
	}()
	t.Cleanup(func() { _ = pw.Close() })

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 2, Platform: PlatformOpenAI}, time.Now(), "gpt-5.4", "gpt-5.4")
	var failover *UpstreamFailoverError
	require.True(t, errors.As(err, &failover), "got %v", err)
	requireClientHidesWaitTimeout(t, string(failover.ResponseBody))
	require.Contains(t, string(failover.ResponseBody), OpenAIWaitTimeoutClientMessage())
	require.Contains(t, string(failover.RawUpstreamBody), OpenAIFirstUsefulFrameTimeoutMarker)
	require.True(t, IsOpenAIWaitTimeoutOpsError("", "", string(failover.RawUpstreamBody)))
	require.NotContains(t, err.Error(), "stream data interval timeout")
	require.False(t, rec.Body.Len() > 0 && strings.Contains(rec.Body.String(), "output_item.added"))
}

func TestHandleStreamingResponse_UsefulFrameInTime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	override := OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 0, FirstUsefulFrameSeconds: 2}
	openAIWaitTimeoutSettingsOverride = &override
	t.Cleanup(func() { openAIWaitTimeoutSettingsOverride = nil })

	svc := &OpenAIGatewayService{cfg: &config.Config{
		Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 0, StreamKeepaliveInterval: 0, MaxLineSize: defaultMaxLineSize},
	}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{}}
	go func() {
		defer func() { _ = pw.Close() }()
		_, _ = pw.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\"}}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"message\"},\"output_index\":0}\n\n"))
		_, _ = pw.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\"}}\n\n"))
	}()

	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 2, Platform: PlatformOpenAI}, time.Now(), "gpt-5.4", "gpt-5.4")
	require.False(t, IsOpenAIWaitTimeoutOpsError(fmtErr(err), "", ""))
	if err != nil {
		var failover *UpstreamFailoverError
		if errors.As(err, &failover) {
			require.NotContains(t, string(failover.ResponseBody), OpenAIFirstUsefulFrameTimeoutMarker)
			require.NotContains(t, string(failover.ResponseBody), OpenAIHeaderWaitTimeoutMarker)
		}
	}
}

func fmtErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func requireClientHidesWaitTimeout(t *testing.T, clientVisible string) {
	t.Helper()
	require.NotContains(t, clientVisible, OpenAIHeaderWaitTimeoutMarker)
	require.NotContains(t, clientVisible, OpenAIFirstUsefulFrameTimeoutMarker)
	require.NotContains(t, clientVisible, "waited_ms=")
}

func TestRewriteOpenAIWaitTimeoutClientText(t *testing.T) {
	t.Parallel()
	require.Equal(t, OpenAIWaitTimeoutClientMessage(), rewriteOpenAIWaitTimeoutClientText(OpenAIHeaderWaitTimeoutMarker+" waited_ms=90001"))
	require.Equal(t, OpenAIWaitTimeoutClientMessage(), rewriteOpenAIWaitTimeoutClientText(OpenAIFirstUsefulFrameTimeoutMarker))
	require.Equal(t, "context window exceeded", rewriteOpenAIWaitTimeoutClientText("context window exceeded"))
}

func TestNewOpenAIStreamFailoverError_WaitTimeoutSplitsClientAndOps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	svc := &OpenAIGatewayService{}
	opsMessage := openAIWaitTimeoutMessage(OpenAIHeaderWaitTimeoutMarker, 90*time.Second)
	failover := svc.newOpenAIStreamFailoverError(c, &Account{ID: 9, Platform: PlatformOpenAI}, false, "", nil, opsMessage)
	require.Equal(t, http.StatusBadGateway, failover.StatusCode)
	requireClientHidesWaitTimeout(t, string(failover.ResponseBody))
	require.Contains(t, string(failover.ResponseBody), OpenAIWaitTimeoutClientMessage())
	require.Contains(t, string(failover.RawUpstreamBody), OpenAIHeaderWaitTimeoutMarker)
	require.Contains(t, string(failover.RawUpstreamBody), "waited_ms=")
	require.Contains(t, opsContextString(c, OpsUpstreamErrorMessageKey), OpenAIHeaderWaitTimeoutMarker)
}

func TestOpenAIWaitTimeoutCanSilentFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	require.True(t, openAIWaitTimeoutCanSilentFailover(c, false))
	require.False(t, openAIWaitTimeoutCanSilentFailover(c, true))
	MarkResponseCommitted(c)
	require.False(t, openAIWaitTimeoutCanSilentFailover(c, false))
}

func TestAbortOpenAIWaitTimeoutAfterCommit_NotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	svc := &OpenAIGatewayService{}
	err := svc.abortOpenAIWaitTimeoutAfterCommit(c.Request.Context(), c, &Account{ID: 3, Platform: PlatformOpenAI}, "gpt-5.4", false, OpenAIFirstUsefulFrameTimeoutMarker, 31*time.Second)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover), "committed abort must not switch accounts")
	require.Contains(t, err.Error(), "upstream response failed:")
	require.Contains(t, err.Error(), OpenAIFirstUsefulFrameTimeoutMarker)
	require.True(t, IsOpenAIWaitTimeoutOpsError(err.Error(), "", ""))
}

func TestOpenAIFirstUsefulFrameTimeoutErr_CommittedAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	svc := &OpenAIGatewayService{}

	silent, err := svc.openAIFirstUsefulFrameTimeoutErr(c.Request.Context(), c, &Account{ID: 3}, "gpt-5.4", false, time.Second, false)
	require.True(t, silent)
	var failover *UpstreamFailoverError
	require.True(t, errors.As(err, &failover))

	MarkResponseCommitted(c)
	silent, err = svc.openAIFirstUsefulFrameTimeoutErr(c.Request.Context(), c, &Account{ID: 3}, "gpt-5.4", false, time.Second, false)
	require.False(t, silent)
	require.False(t, errors.As(err, &failover))
	require.Contains(t, err.Error(), OpenAIFirstUsefulFrameTimeoutMarker)
}

func TestHandleStreamingResponse_FirstUsefulFrameTimeoutAfterFlushPreamble(t *testing.T) {
	gin.SetMode(gin.TestMode)
	override := OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 0, FirstUsefulFrameSeconds: 1}
	openAIWaitTimeoutSettingsOverride = &override
	t.Cleanup(func() { openAIWaitTimeoutSettingsOverride = nil })
	storeGatewayFlushPreambleCache(t, true)

	svc := &OpenAIGatewayService{
		cfg: &config.Config{
			Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 0, StreamKeepaliveInterval: 0, MaxLineSize: defaultMaxLineSize},
		},
		settingService: NewSettingService(&openaiWaitTimeoutSettingRepo{data: map[string]string{
			SettingKeyOpenAIResponsesFlushPreamble: "true",
		}}, &config.Config{}),
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	pr, pw := io.Pipe()
	resp := &http.Response{StatusCode: http.StatusOK, Body: pr, Header: http.Header{}}
	go func() {
		_, _ = pw.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"r1\"}}\n\n"))
	}()
	t.Cleanup(func() { _ = pw.Close() })

	started := time.Now()
	_, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, &Account{ID: 2, Platform: PlatformOpenAI}, time.Now(), "gpt-5.4", "gpt-5.4")
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover), "got failover %v", err)
	require.Contains(t, err.Error(), OpenAIFirstUsefulFrameTimeoutMarker)
	require.NotContains(t, err.Error(), "stream data interval timeout")
	body := rec.Body.String()
	require.Contains(t, body, `"type":"response.created"`)
	require.Contains(t, body, `"type":"error"`)
	requireClientHidesWaitTimeout(t, body)
	require.Contains(t, body, OpenAIWaitTimeoutClientMessage())
	require.GreaterOrEqual(t, time.Since(started), time.Second)
	require.Less(t, time.Since(started), 4*time.Second)
}

func TestClassifyOpsErrorRateCalibers_OpenAIWaitTimeoutRecovered(t *testing.T) {
	recoveredTimeout := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 200,
		Phase:        "upstream",
		Type:         "upstream_error",
		Message:      "Recovered upstream error 502: " + OpenAIHeaderWaitTimeoutMarker,
	})
	require.True(t, recoveredTimeout.IsRecovered)
	require.False(t, recoveredTimeout.CountedInUserErrorRate)
	require.True(t, recoveredTimeout.CountedInAccountCompareRate)
	require.True(t, recoveredTimeout.CountedInAccountScheduleRate)

	frameTimeout := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus:         200,
		Phase:                "upstream",
		Type:                 "upstream_error",
		Message:              "Recovered",
		UpstreamErrorMessage: OpenAIFirstUsefulFrameTimeoutMarker + " waited_ms=31000",
	})
	require.True(t, frameTimeout.CountedInAccountScheduleRate)
	require.False(t, frameTimeout.CountedInUserErrorRate)

	recovered429 := ClassifyOpsErrorRateCalibers(OpsErrorCaliberInput{
		ClientStatus: 200,
		Phase:        "upstream",
		Type:         "rate_limit_error",
		Message:      "Recovered upstream error 429: too many requests",
	})
	require.False(t, recovered429.CountedInAccountScheduleRate)

	wl := DefaultScheduleErrorWhitelist()
	wl.Custom = []ScheduleErrorCustomRule{{
		ID:              "c_wait",
		Enabled:         true,
		MessageContains: OpenAIHeaderWaitTimeoutMarker,
	}}
	excluded := IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{
		Status:  502,
		Phase:   "upstream",
		Type:    "upstream_error",
		Message: OpenAIHeaderWaitTimeoutMarker,
	}, wl)
	require.False(t, excluded)
}

func TestParseOpenAIWaitTimeoutSettingsJSON_RoundTrip(t *testing.T) {
	raw, err := json.Marshal(OpenAIWaitTimeoutSettings{HeaderWaitSeconds: 90, FirstUsefulFrameSeconds: 30})
	require.NoError(t, err)
	got := parseOpenAIWaitTimeoutSettingsJSON(string(raw))
	require.Equal(t, 90, got.HeaderWaitSeconds)
	require.Equal(t, 30, got.FirstUsefulFrameSeconds)
}
