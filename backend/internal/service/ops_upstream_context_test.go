package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetOpsUpstreamModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	SetOpsUpstreamModel(c, "  gpt-5.6-luna  ")
	v, ok := c.Get(OpsUpstreamModelKey)
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-luna", v)

	SetOpsUpstreamModel(c, "   ")
	v2, ok2 := c.Get(OpsUpstreamModelKey)
	require.True(t, ok2)
	require.Equal(t, "gpt-5.6-luna", v2, "empty should not clear previous value")
}

func TestSafeUpstreamURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"strips query", "https://api.anthropic.com/v1/messages?beta=true", "https://api.anthropic.com/v1/messages"},
		{"strips fragment", "https://api.openai.com/v1/responses#frag", "https://api.openai.com/v1/responses"},
		{"strips both", "https://host/path?token=secret#x", "https://host/path"},
		{"no query or fragment", "https://host/path", "https://host/path"},
		{"empty string", "", ""},
		{"whitespace only", "  ", ""},
		{"query before fragment", "https://h/p?a=1#f", "https://h/p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, safeUpstreamURL(tt.input))
		})
	}
}

func TestNormalizeOpsUpstreamEndpoint(t *testing.T) {
	require.Equal(t, "/v1/responses", normalizeOpsUpstreamEndpoint(" /v1/responses?token=secret#fragment "))
	require.Empty(t, normalizeOpsUpstreamEndpoint("https://api.example/v1/responses"))
}

func TestAppendOpsUpstreamError_UsesRequestBodyBytesFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	setOpsUpstreamRequestBody(c, []byte(`{"model":"gpt-5"}`))
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Kind:    "http_error",
		Message: "upstream failed",
	})

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, `{"model":"gpt-5"}`, events[0].UpstreamRequestBody)
}

func TestAppendOpsUpstreamError_UsesRequestBodyStringFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	c.Set(OpsUpstreamRequestBodyKey, `{"model":"gpt-4"}`)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Kind:    "request_error",
		Message: "dial timeout",
	})

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, `{"model":"gpt-4"}`, events[0].UpstreamRequestBody)
}

func TestGenericOpsUpstreamMessages_AlignsWithFrontendSet(t *testing.T) {
	locked := []string{
		"upstream request failed",
		"upstream request failed after retries",
		"upstream gateway error",
		"upstream service temporarily unavailable",
	}
	require.Len(t, genericOpsUpstreamMessages, len(locked))
	for _, msg := range locked {
		require.True(t, isGenericOpsUpstreamMessage(msg), msg)
		require.True(t, isGenericOpsUpstreamMessage(strings.ToUpper(msg)), msg)
	}
	require.False(t, isGenericOpsUpstreamMessage("no enabled keys"))
	require.False(t, isGenericOpsUpstreamMessage("Upstream authentication failed, please contact administrator"))
}

func TestSetOpsUpstreamError_MergeDoesNotWipeSpecific(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw := `{"error":{"type":"new_api_error","code":"channel:no_available_key","message":"no enabled keys"}}`
	setOpsUpstreamError(c, 503, "no enabled keys", raw)
	setOpsUpstreamError(c, 502, "Upstream service temporarily unavailable", "")

	require.Equal(t, "no enabled keys", opsContextString(c, OpsUpstreamErrorMessageKey))
	require.Equal(t, raw, opsContextString(c, OpsUpstreamErrorDetailKey))
	require.Equal(t, "channel:no_available_key", opsContextString(c, OpsProviderErrorCodeKey))
	status, ok := c.Get(OpsUpstreamStatusCodeKey)
	require.True(t, ok)
	require.Equal(t, 502, status)
}

func TestSetOpsUpstreamError_GenericWrapperDetailDoesNotWipeRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw := `{"error":{"type":"new_api_error","code":"channel:no_available_key","message":"no enabled keys"}}`
	wrapper := `{"error":{"type":"upstream_error","message":"Upstream request failed"}}`
	setOpsUpstreamError(c, 503, "no enabled keys", raw)
	setOpsUpstreamError(c, 502, "Upstream request failed", wrapper)

	require.Equal(t, "no enabled keys", opsContextString(c, OpsUpstreamErrorMessageKey))
	require.Equal(t, raw, opsContextString(c, OpsUpstreamErrorDetailKey))
	require.Equal(t, "channel:no_available_key", opsContextString(c, OpsProviderErrorCodeKey))
}

func TestSetOpsUpstreamError_GenericDoesNotOverwriteSpecificMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	setOpsUpstreamError(c, 503, "no enabled keys", "")
	setOpsUpstreamError(c, 502, "Upstream request failed", "")
	require.Equal(t, "no enabled keys", opsContextString(c, OpsUpstreamErrorMessageKey))

	setOpsUpstreamError(c, 503, "no enabled keys (retry)", "")
	require.Equal(t, "no enabled keys (retry)", opsContextString(c, OpsUpstreamErrorMessageKey))
}

func TestRecordOpsUpstreamAttempt_StoresNewAPIOriginalNotWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw := []byte(`{"error":{"type":"new_api_error","code":"channel:no_available_key","message":"no enabled keys (any suffix) for model gpt-5.4"}}`)
	recordOpsUpstreamAttempt(c, OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: 503,
		Kind:               "failover",
		Message:            "Upstream request failed",
	}, raw)

	require.Equal(t, "no enabled keys (any suffix) for model gpt-5.4", opsContextString(c, OpsUpstreamErrorMessageKey))
	require.Contains(t, opsContextString(c, OpsUpstreamErrorDetailKey), "channel:no_available_key")
	require.NotContains(t, strings.ToLower(opsContextString(c, OpsUpstreamErrorDetailKey)), "upstream request failed")
	require.Equal(t, "channel:no_available_key", opsContextString(c, OpsProviderErrorCodeKey))

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "no enabled keys (any suffix) for model gpt-5.4", events[0].Message)
	require.Contains(t, events[0].Detail, `"code":"channel:no_available_key"`)
}

func TestRecordOpsUpstreamAttempt_IgnoresGenericWrapperBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	raw := []byte(`{"error":{"type":"new_api_error","code":"channel:no_available_key","message":"no enabled keys"}}`)
	recordOpsUpstreamAttempt(c, OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: 503,
		Kind:               "failover",
	}, raw)

	wrapper := []byte(`{"error":{"type":"upstream_error","message":"Upstream service temporarily unavailable"}}`)
	recordOpsUpstreamAttempt(c, OpsUpstreamErrorEvent{
		Platform:           PlatformOpenAI,
		UpstreamStatusCode: 502,
		Kind:               "failover",
		Message:            "Upstream service temporarily unavailable",
	}, wrapper)

	require.Equal(t, "no enabled keys", opsContextString(c, OpsUpstreamErrorMessageKey))
	require.Contains(t, opsContextString(c, OpsUpstreamErrorDetailKey), "channel:no_available_key")
	require.NotContains(t, strings.ToLower(opsContextString(c, OpsUpstreamErrorDetailKey)), "upstream service temporarily unavailable")
	require.Equal(t, "channel:no_available_key", opsContextString(c, OpsProviderErrorCodeKey))

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events := v.([]*OpsUpstreamErrorEvent)
	require.Len(t, events, 2)
	require.NotContains(t, events[1].Detail, `"type":"upstream_error"`)
	require.Empty(t, events[1].Message)
}

func TestFailoverOpsRawBody_PrefersUnreadRewrittenOriginal(t *testing.T) {
	wrapper := []byte(`{"error":{"type":"upstream_error","message":"Upstream service temporarily unavailable"}}`)
	raw := []byte(`{"error":{"code":"channel:no_available_key","message":"no enabled keys"}}`)
	require.Equal(t, raw, FailoverOpsRawBody(&UpstreamFailoverError{
		ResponseBody:    wrapper,
		RawUpstreamBody: raw,
	}))
	require.Equal(t, wrapper, FailoverOpsRawBody(&UpstreamFailoverError{ResponseBody: wrapper}))
	require.Nil(t, FailoverOpsRawBody(nil))
}

func TestNewOpenAIStreamFailoverError_RecordsRawPayloadNotClientWrapper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 1730, Name: "loveapi", Platform: PlatformOpenAI}
	payload := []byte(`{"error":{"type":"new_api_error","code":"channel:no_available_key","message":"no enabled keys"}}`)

	err := svc.newOpenAIStreamFailoverError(c, account, false, "rid-1", payload, "Upstream stream disconnected before completion")
	require.NotNil(t, err)
	require.Equal(t, payload, err.RawUpstreamBody)
	require.Contains(t, string(err.ResponseBody), `"type":"upstream_error"`)
	require.Contains(t, string(err.ResponseBody), "Upstream stream disconnected before completion")
	require.NotContains(t, string(err.ResponseBody), "channel:no_available_key")

	require.Equal(t, "no enabled keys", opsContextString(c, OpsUpstreamErrorMessageKey))
	require.Contains(t, opsContextString(c, OpsUpstreamErrorDetailKey), "channel:no_available_key")
	require.Equal(t, "channel:no_available_key", opsContextString(c, OpsProviderErrorCodeKey))

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events := v.([]*OpsUpstreamErrorEvent)
	require.Len(t, events, 1)
	require.Contains(t, events[0].Detail, "channel:no_available_key")
	require.NotContains(t, events[0].Detail, `"type":"upstream_error"`)
}

func TestAppendOpsUpstreamError_CapturesActualOpenAIUpstreamEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	SetActualOpenAIUpstreamEndpoint(c, "/v1/chat/completions")

	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform: PlatformOpenAI,
		Kind:     "http_error",
		Message:  "Unsupported content type",
	})

	v, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := v.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Equal(t, "/v1/chat/completions", events[0].UpstreamEndpoint)
}
