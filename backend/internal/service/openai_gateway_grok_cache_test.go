//go:build unit

package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newGrokCacheTestContext(apiKeyID int64, platform string) *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	if apiKeyID > 0 {
		c.Set("api_key", &APIKey{ID: apiKeyID, Group: &Group{Platform: platform}})
	}
	return c
}

func TestResolveGrokCacheIdentityStableAndTenantIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	round1 := []byte(`{"model":"grok","instructions":"be concise","input":[{"role":"user","content":"first"}]}`)
	round2 := []byte(`{"model":"grok","instructions":"be concise","input":[{"role":"user","content":"first"},{"role":"assistant","content":"answer"},{"role":"user","content":"second"}]}`)

	first := resolveGrokCacheIdentity(newGrokCacheTestContext(101, PlatformGrok), round1, "", "grok-4.5")
	second := resolveGrokCacheIdentity(newGrokCacheTestContext(101, PlatformGrok), round2, "", "grok-4.5")
	otherTenant := resolveGrokCacheIdentity(newGrokCacheTestContext(102, PlatformGrok), round1, "", "grok-4.5")
	otherModel := resolveGrokCacheIdentity(newGrokCacheTestContext(101, PlatformGrok), round1, "", "grok-4.3")

	require.NotEmpty(t, first)
	require.Len(t, first, 36)
	require.Equal(t, first, second)
	require.NotEqual(t, first, otherTenant)
	require.NotEqual(t, first, otherModel)
}

func TestGrokConversationHeaderIsScopedToGrokRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"grok","prompt_cache_key":"body-session","input":"hi"}`)

	grokContext := newGrokCacheTestContext(201, PlatformGrok)
	grokContext.Request.Header.Set(grokConversationIDHeader, "native-grok-session")
	require.Equal(t, "native-grok-session", (&OpenAIGatewayService{}).ExtractSessionID(grokContext, body))

	openAIContext := newGrokCacheTestContext(201, PlatformOpenAI)
	openAIContext.Request.Header.Set(grokConversationIDHeader, "must-be-ignored")
	require.Equal(t, "body-session", (&OpenAIGatewayService{}).ExtractSessionID(openAIContext, body))
}

func TestApplyGrokResponsesCacheIdentityPreservesClientToolIntent(t *testing.T) {
	plain := []byte(`{"model":"grok-4.5","input":"hello"}`)
	withIdentity, err := applyGrokResponsesCacheIdentity(plain, plain, "isolated-id", true)
	require.NoError(t, err)
	require.Equal(t, "isolated-id", gjson.GetBytes(withIdentity, "prompt_cache_key").String())
	require.Equal(t, "web_search", gjson.GetBytes(withIdentity, "tools.0.type").String())
	require.Equal(t, "x_search", gjson.GetBytes(withIdentity, "tools.1.type").String())
	require.Equal(t, "none", gjson.GetBytes(withIdentity, "tool_choice").String())

	explicitTools := []byte(`{"model":"grok-4.5","input":"hello","tools":[],"tool_choice":"none"}`)
	preserved, err := applyGrokResponsesCacheIdentity(explicitTools, explicitTools, "isolated-id", true)
	require.NoError(t, err)
	require.Equal(t, "[]", gjson.GetBytes(preserved, "tools").Raw)
	require.Equal(t, "none", gjson.GetBytes(preserved, "tool_choice").String())

	unscoped, err := applyGrokResponsesCacheIdentity(
		[]byte(`{"model":"grok-4.5","prompt_cache_key":"raw-client-key"}`),
		plain,
		"",
		true,
	)
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(unscoped, "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(unscoped, "tools").Exists())
}

func TestGrokCompactRequestSkipsCacheIdentityAndNativeTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newGrokCacheTestContext(301, PlatformGrok)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)
	body := []byte(`{"model":"grok","input":"compact this","prompt_cache_key":"raw-client-key"}`)

	identity := resolveGrokCacheIdentity(c, body, "", "grok-4.5")
	patched, err := applyGrokResponsesCacheIdentity(body, body, identity, true)

	require.NoError(t, err)
	require.Empty(t, identity)
	require.False(t, gjson.GetBytes(patched, "prompt_cache_key").Exists())
	require.False(t, gjson.GetBytes(patched, "tools").Exists())
	require.False(t, gjson.GetBytes(patched, "tool_choice").Exists())
}
