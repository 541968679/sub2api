//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func responseFailedContextLengthSSE() string {
	failed := `{"type":"response.failed","response":{"id":"resp_err","object":"response","status":"failed","error":{"code":"context_length_exceeded","type":"invalid_request_error","message":"Your input exceeds the context window of this model. Please adjust your input and try again."},"output":[],"usage":{"input_tokens":100000,"output_tokens":0,"total_tokens":100000}}}`
	return fmt.Sprintf("data: %s\n\n", failed)
}

func responseFailedOverloadedSSE() string {
	failed := `{"type":"response.failed","response":{"id":"resp_busy","status":"failed","error":{"code":"server_is_overloaded","type":"server_error","message":"Selected model is at capacity. Please try a different model."},"output":[]}}`
	return fmt.Sprintf("data: %s\n\n", failed)
}

func bindResponseFailedRule(c *gin.Context, platform string, upstreamStatus, responseStatus int) {
	rule := &model.ErrorPassthroughRule{
		ID:              1,
		Name:            "response-failed-context-window",
		Enabled:         true,
		Priority:        1,
		Platforms:       []string{platform},
		ErrorCodes:      []int{upstreamStatus},
		Keywords:        []string{"context_length_exceeded"},
		MatchMode:       model.MatchModeAll,
		ResponseCode:    &responseStatus,
		PassthroughBody: true,
	}
	svc := &ErrorPassthroughService{}
	svc.setLocalCache([]*model.ErrorPassthroughRule{rule})
	BindErrorPassthroughService(c, svc)
}

func responseFailedRecorder(t *testing.T, path string, body []byte) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return rec, c
}

func responseFailedUpstream() *httpUpstreamRecorder {
	return &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(responseFailedContextLengthSSE())),
	}}
}

func TestResponseFailedSync_NativeResponsesAppliesSemanticStatusRuleAndRecordsOpsOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec, c := responseFailedRecorder(t, "/v1/responses", nil)
	bindResponseFailedRule(c, PlatformOpenAI, http.StatusBadRequest, http.StatusBadRequest)
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	resp := responseFailedUpstream().resp
	account := rawChatCompletionsTestAccount()

	_, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "a configured client error must not fail over")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
	events, _ := c.Get(OpsUpstreamErrorsKey)
	require.Len(t, events, 1, "the terminal failure must create one upstream Ops marker")
}

func TestResponseFailedSync_NativePassthroughAppliesRuleBeforeClientOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec, c := responseFailedRecorder(t, "/v1/responses", nil)
	bindResponseFailedRule(c, PlatformOpenAI, http.StatusBadRequest, http.StatusBadRequest)
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	account := rawChatCompletionsTestAccount()

	_, err := svc.handleStreamingResponsePassthrough(context.Background(), responseFailedUpstream().resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "upstream_error", gjson.Get(rec.Body.String(), "error.type").String())
	require.NotContains(t, rec.Body.String(), "event: response.failed")
}

func TestResponseFailedSync_ChatAndMessagesReturnNoBillableForwardResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		path      string
		body      []byte
		streaming bool
		call      func(*OpenAIGatewayService, *gin.Context, *Account, []byte) (*OpenAIForwardResult, error)
	}{
		{
			name: "chat",
			path: "/v1/chat/completions",
			body: []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`),
			call: func(s *OpenAIGatewayService, c *gin.Context, a *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, a, body, "", "")
			},
		},
		{
			name:      "chat stream",
			path:      "/v1/chat/completions",
			body:      []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":true}`),
			streaming: true,
			call: func(s *OpenAIGatewayService, c *gin.Context, a *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsChatCompletions(context.Background(), c, a, body, "", "")
			},
		},
		{
			name: "messages",
			path: "/v1/messages",
			body: []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":false}`),
			call: func(s *OpenAIGatewayService, c *gin.Context, a *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, a, body, "", "")
			},
		},
		{
			name:      "messages stream",
			path:      "/v1/messages",
			body:      []byte(`{"model":"gpt-5.4","max_tokens":32,"messages":[{"role":"user","content":"hello"}],"stream":true}`),
			streaming: true,
			call: func(s *OpenAIGatewayService, c *gin.Context, a *Account, body []byte) (*OpenAIForwardResult, error) {
				return s.ForwardAsAnthropic(context.Background(), c, a, body, "", "")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, c := responseFailedRecorder(t, tt.path, tt.body)
			bindResponseFailedRule(c, PlatformOpenAI, http.StatusBadRequest, http.StatusBadRequest)
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: responseFailedUpstream()}

			result, err := tt.call(svc, c, rawChatCompletionsTestAccount(), tt.body)

			require.Error(t, err)
			if !tt.streaming {
				require.Nil(t, result, "buffered failures must not produce a successful forward result")
			}
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Contains(t, gjson.Get(rec.Body.String(), "error.message").String(), "context window")
			events, _ := c.Get(OpsUpstreamErrorsKey)
			require.Len(t, events, 1, "failed terminal must create exactly one upstream Ops marker")
		})
	}
}

func TestResponseFailedSync_TransientFailureStillFailsOverWithoutWritingOrBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec, c := responseFailedRecorder(t, "/v1/responses", nil)
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	account := rawChatCompletionsTestAccount()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(responseFailedOverloadedSSE())),
	}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.4", "gpt-5.4")

	require.Error(t, err)
	require.NotNil(t, result, "usage container may be returned for diagnostics")
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.False(t, c.Writer.Written(), "pre-output failover must not commit a downstream response")
	require.Empty(t, rec.Body.String())
}

func TestResponseFailedSync_GrokRulesStayPlatformScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := []byte(strings.TrimSpace(strings.TrimPrefix(responseFailedContextLengthSSE(), "data: ")))
	tests := []struct {
		name         string
		rulePlatform string
		wantMatched  bool
	}{
		{name: "openai rule does not leak", rulePlatform: PlatformOpenAI, wantMatched: false},
		{name: "grok rule matches", rulePlatform: PlatformGrok, wantMatched: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, c := responseFailedRecorder(t, "/v1/chat/completions", nil)
			bindResponseFailedRule(c, tt.rulePlatform, http.StatusBadRequest, http.StatusBadRequest)

			status, _, _, matched := applyOpenAIStreamFailedErrorPassthroughRule(c, PlatformGrok, payload, "context window exceeded")

			require.Equal(t, tt.wantMatched, matched)
			if matched {
				require.Equal(t, http.StatusBadRequest, status)
			}
		})
	}
}
