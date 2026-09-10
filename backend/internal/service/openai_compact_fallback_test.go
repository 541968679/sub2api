//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newOpenAICompactFallbackTestContext(t *testing.T, path string) *gin.Context {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, nil)
	return c
}

func TestPrepareOpenAICompactFallbackRetryRequiresExplicitCompact(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.4"}}}
	c := newOpenAICompactFallbackTestContext(t, "/v1/responses")
	body := []byte(`{"model":"gpt-5.5","input":[{"type":"message","role":"user","content":"hello"}]}`)
	errorBody := []byte(`{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`)

	retryBody, fallbackModel, retry := svc.prepareOpenAICompactFallbackRetry(
		c, nil, "gpt-5.5", body, http.StatusBadRequest, "maximum context length exceeded", errorBody, false,
	)

	require.False(t, retry)
	require.Empty(t, fallbackModel)
	require.Equal(t, body, retryBody)
}

func TestPrepareOpenAICompactFallbackRetryLegacyPathAndSingleAttemptGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.4"}}}
	c := newOpenAICompactFallbackTestContext(t, "/v1/responses/compact")
	body := []byte(`{"model":"gpt-5.5","input":[]}`)
	errorBody := []byte(`{"response":{"status":"failed","error":null}}`)

	retryBody, fallbackModel, retry := svc.prepareOpenAICompactFallbackRetry(
		c, nil, "gpt-5.5", body, http.StatusBadRequest, "", errorBody, false,
	)
	require.True(t, retry)
	require.Equal(t, "gpt-5.4", fallbackModel)
	require.Equal(t, "/compact", openAIResponsesRequestPathSuffix(c))

	secondBody, secondModel, secondRetry := svc.prepareOpenAICompactFallbackRetry(
		c, nil, "gpt-5.5", retryBody, http.StatusBadRequest, "", errorBody, true,
	)
	require.False(t, secondRetry)
	require.Empty(t, secondModel)
	require.Equal(t, retryBody, secondBody)
}

func TestPrepareOpenAICompactFallbackRetryDoesNotHideSpecificBusinessFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.4"}}}
	c := newOpenAICompactFallbackTestContext(t, "/v1/responses/compact")
	body := []byte(`{"model":"gpt-5.5","input":[]}`)
	errorBody := []byte(`{"response":{"status":"failed","error":{"type":"permission_error","message":"workspace denied"}}}`)

	retryBody, fallbackModel, retry := svc.prepareOpenAICompactFallbackRetry(
		c, nil, "gpt-5.5", body, http.StatusBadRequest, "workspace denied", errorBody, false,
	)

	require.False(t, retry)
	require.Empty(t, fallbackModel)
	require.Equal(t, body, retryBody)
}

func TestIsOpenAICompactModelFailureRequiresExplicitModelAvailabilityMessage(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{name: "explicit unsupported model", message: "The requested model is not supported", want: true},
		{name: "named missing model", message: "The model `gpt-5.5` does not exist", want: true},
		{name: "unsupported model code-like message", message: "unsupported model: gpt-5.5", want: true},
		{name: "unsupported model feature", message: "This model output format is not supported", want: false},
		{name: "unsupported parameter for model", message: "Parameter tools is not supported for this model", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpenAICompactModelFailure(
				http.StatusBadRequest,
				tt.message,
				[]byte(`{"error":{"message":`+strconv.Quote(tt.message)+`}}`),
			))
		})
	}
}

func TestPrepareOpenAICompactFallbackRetrySkipsSameModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "gpt-5.5"}}}
	c := newOpenAICompactFallbackTestContext(t, "/v1/responses/compact")
	body := []byte(`{"model":"gpt-5.5","input":[]}`)
	errorBody := []byte(`{"error":{"code":"model_not_found","message":"model not found"}}`)

	_, _, retry := svc.prepareOpenAICompactFallbackRetry(
		c, nil, "gpt-5.5", body, http.StatusNotFound, "model not found", errorBody, false,
	)
	require.False(t, retry)
}

func TestResolveOpenAICompactFallbackModelPrefersAccountMapping(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{OpenAICompactModel: "global-compact"}}}
	account := &Account{Credentials: map[string]any{
		"compact_model_mapping": map[string]any{"gpt-5.5": "account-compact"},
	}}

	require.Equal(t, "account-compact", svc.resolveOpenAICompactFallbackModel(account, "gpt-5.5"))
	require.Equal(t, "global-compact", svc.resolveOpenAICompactFallbackModel(account, "unmapped-model"))
}
