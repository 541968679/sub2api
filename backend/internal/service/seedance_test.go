package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"net/http/httptest"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseSeedanceRequestRequiresModelAndContent(t *testing.T) {
	_, err := ParseSeedanceRequest([]byte(`{"content":[{"type":"text","text":"sea"}]}`))
	require.Error(t, err)
	_, err = ParseSeedanceRequest([]byte(`{"model":"seedance-video","content":[]}`))
	require.Error(t, err)

	info, err := ParseSeedanceRequest([]byte(`{"model":" seedance-video ","content":[{"type":"text","text":"waves"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}`))
	require.NoError(t, err)
	require.Equal(t, "seedance-video", info.Model)
	require.Equal(t, "waves", info.Prompt)
	require.Equal(t, []string{"https://example.com/a.png"}, info.InputImageURLs)
}

func TestBuildSeedanceURL(t *testing.T) {
	got, err := buildSeedanceURL("https://ark.example.com/api/v3", SeedanceEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://ark.example.com/api/v3/contents/generations/tasks", got)

	got, err = buildSeedanceURL("https://ark.example.com", SeedanceEndpointStatus, "task/1")
	require.Error(t, err)

	got, err = buildSeedanceURL("https://relay.example.com/v3/", SeedanceEndpointStatus, "task_1")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/v3/contents/generations/tasks/task_1", got)
}

func TestAccountSupportsSeedanceCapability(t *testing.T) {
	ready := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":            "https://ark.example.com/api/v3",
			"api_key":             "ark-key",
			"openai_capabilities": []string{"chat_completions", "embeddings", "seedance"},
		},
	}
	require.True(t, ready.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	require.True(t, ready.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))

	unset := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://ark.example.com"}}
	require.False(t, unset.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	require.True(t, unset.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
	require.True(t, unset.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings))

	empty := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url":            "https://ark.example.com",
		"openai_capabilities": map[string]any{},
	}}
	require.False(t, empty.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	require.True(t, empty.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))

	explicitOff := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"openai_capabilities": map[string]any{"chat_completions": false},
	}}
	require.False(t, explicitOff.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))

	grok := &Account{Platform: PlatformGrok, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"base_url":            "https://ark.example.com",
		"openai_capabilities": []string{"seedance"},
	}}
	require.False(t, grok.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance))
	require.True(t, grok.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions))
}

func TestStickySessionValueKeepsOrdinaryPinsAndSeedanceModel(t *testing.T) {
	plain, err := ParseStickySessionValue("1718")
	require.NoError(t, err)
	require.Equal(t, "1718", EncodeStickySessionValue(plain))

	overflow, err := ParseStickySessionValue("1718|o")
	require.NoError(t, err)
	require.Equal(t, "1718|o", EncodeStickySessionValue(overflow))

	withModel := StickySessionBinding{AccountID: 42, Model: "seedance video"}
	encoded := EncodeStickySessionValue(withModel)
	parsed, err := ParseStickySessionValue(encoded)
	require.NoError(t, err)
	require.Equal(t, int64(42), parsed.AccountID)
	require.False(t, parsed.Overflow)
	require.Equal(t, "seedance video", parsed.Model)
}

func TestSeedanceTaskBindingIsScopedToAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := &stubGatewayCache{}
	svc := &OpenAIGatewayService{cache: cache, cfg: &config.Config{}}
	groupID := int64(7)
	require.NoError(t, svc.BindSeedanceTask(context.Background(), &groupID, 3, 9, "task_1", "seedance-video", 42))

	own, err := svc.LookupSeedanceTask(context.Background(), &groupID, 3, 9, "task_1")
	require.NoError(t, err)
	require.Equal(t, int64(42), own.AccountID)
	require.Equal(t, "seedance-video", own.Model)

	_, err = svc.LookupSeedanceTask(context.Background(), &groupID, 3, 10, "task_1")
	require.Error(t, err)
}

type seedanceUpstream struct {
	calls int
	last  *http.Request
	body  []byte
	code  int
}

func (s *seedanceUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	s.last = req
	if req.Body != nil {
		s.body, _ = io.ReadAll(req.Body)
	}
	code := s.code
	if code == 0 {
		code = http.StatusOK
	}
	payload := `{"id":"task_1","status":"succeeded","model":"doubao-seedance","usage":{"completion_tokens":120}}`
	if req.Method == http.MethodPost {
		payload = `{"id":"task_1","status":"queued"}`
	}
	if code >= 300 {
		payload = `{"error":"nope"}`
	}
	return &http.Response{
		StatusCode: code,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(payload)),
	}, nil
}

func (s *seedanceUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestForwardSeedanceRewritesModelAndDoesNotRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &seedanceUpstream{code: http.StatusBadGateway}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":             "ark-key",
			"base_url":            "https://ark.example.com/api/v3",
			"openai_capabilities": []string{"seedance"},
			"model_mapping":       map[string]any{"seedance-video": "doubao-seedance-1"},
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	payload := []byte(`{"model":"seedance-video","content":[{"type":"text","text":"waves"}]}`)
	c.Request = httptestRequest(http.MethodPost, string(payload))

	_, err := svc.ForwardSeedance(context.Background(), c, account, SeedanceEndpointCreate, "", payload)
	require.Error(t, err)
	require.Equal(t, 1, upstream.calls)
	require.Contains(t, string(upstream.body), `"model":"doubao-seedance-1"`)
	require.NotContains(t, string(upstream.body), "seedance-video")
}

func TestForwardSeedanceStatusBillsOutputTokensNotVideoCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &seedanceUpstream{}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":             "ark-key",
			"base_url":            "https://ark.example.com/api/v3",
			"openai_capabilities": []string{"seedance"},
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptestRequest(http.MethodGet, "")
	result, err := svc.ForwardSeedance(context.Background(), c, account, SeedanceEndpointStatus, "task_1", nil)
	require.NoError(t, err)
	require.Equal(t, 120, result.Usage.OutputTokens)
	require.Equal(t, 0, result.VideoCount)
	require.Equal(t, 0, result.ImageCount)
	require.Equal(t, "seedance:task_1", result.RequestID)
	require.Equal(t, "doubao-seedance", result.UpstreamModel)
}

func httptestRequest(method, body string) *http.Request {
	req, _ := http.NewRequest(method, "http://gateway.local/api/v3/contents/generations/tasks", strings.NewReader(body))
	if body != "" {
		req.Header.Set("X-Body", body)
	}
	return req
}
