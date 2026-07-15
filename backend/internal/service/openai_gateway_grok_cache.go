package service

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	grokConversationIDHeader        = "X-Grok-Conv-Id"
	grokFreeCacheNativeToolsJSON    = `[{"type":"web_search"},{"type":"x_search"}]`
	grokFreeCacheDisabledToolChoice = "none"
)

func resolveGrokCacheIdentity(c *gin.Context, body []byte, explicitKey, upstreamModel string) string {
	apiKeyID := getAPIKeyIDFromContext(c)
	if apiKeyID <= 0 || isOpenAIResponsesCompactPath(c) {
		return ""
	}
	model := strings.ToLower(strings.TrimSpace(upstreamModel))
	if model == "" {
		return ""
	}
	seed := explicitGrokCacheSeed(c, body, explicitKey)
	if seed == "" {
		seed = deriveOpenAIContentSessionSeed(body)
	}
	if seed == "" {
		return ""
	}
	isolatedSeed := fmt.Sprintf("grok-prompt-cache:v1:%d:%s:%s", apiKeyID, model, seed)
	return generateSessionUUID(isolatedSeed)
}

func explicitGrokCacheSeed(c *gin.Context, body []byte, explicitKey string) string {
	seed := ""
	if c != nil {
		seed = strings.TrimSpace(c.GetHeader("session_id"))
		if seed == "" {
			seed = strings.TrimSpace(c.GetHeader("conversation_id"))
		}
		if seed == "" {
			seed = strings.TrimSpace(c.GetHeader(grokConversationIDHeader))
		}
	}
	if seed == "" && len(body) > 0 {
		seed = strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())
	}
	if seed == "" {
		seed = strings.TrimSpace(explicitKey)
	}
	return seed
}

func isGrokRequestContext(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, exists := c.Get("api_key")
	if !exists {
		return false
	}
	apiKey, ok := v.(*APIKey)
	return ok && apiKey != nil && apiKey.Group != nil && apiKey.Group.Platform == PlatformGrok
}

func applyGrokResponsesCacheIdentity(body, intentSourceBody []byte, identity string, injectFreeTierTools bool) ([]byte, error) {
	// Compaction blobs from /responses/compact (or remote_compaction_v2) are
	// cryptographically bound to the original request shape. Rewriting
	// prompt_cache_key or injecting free-tier tools makes xAI return
	// "Could not decode the compaction blob. Ensure it is unmodified...".
	if hasGrokCompactionContext(body) || hasGrokCompactionContext(intentSourceBody) {
		return body, nil
	}
	identity = strings.TrimSpace(identity)
	if identity == "" {
		if gjson.GetBytes(body, "prompt_cache_key").Exists() {
			return sjson.DeleteBytes(body, "prompt_cache_key")
		}
		return body, nil
	}
	out, err := sjson.SetBytes(body, "prompt_cache_key", identity)
	if err != nil {
		return nil, err
	}
	if !injectFreeTierTools {
		return out, nil
	}
	// Decide from the *patched* body, not the original Codex payload. Codex often
	// sends unsupported tools that we strip; the original still "has tools" and
	// used to skip free-tier injection while leaving a bare tool_choice.
	// An explicit tools field (including tools:[]) is client intent — do not
	// overwrite with free-tier defaults.
	if gjson.GetBytes(out, "tools").Exists() {
		return out, nil
	}
	out, err = sjson.SetRawBytes(out, "tools", []byte(grokFreeCacheNativeToolsJSON))
	if err != nil {
		return nil, err
	}
	return sjson.SetBytes(out, "tool_choice", grokFreeCacheDisabledToolChoice)
}

// hasGrokCompactionContext detects remote-compaction request shapes whose
// opaque blobs must pass through to xAI without gateway rewrites.
func hasGrokCompactionContext(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	if HasCompactionTriggerInInput(body) {
		return true
	}
	// Cheap gate: avoid scanning every request. Match both compact and
	// compaction tokens (OpenAI remote_compaction_v2 uses type=compaction).
	if !bytes.Contains(body, []byte("compact")) {
		return false
	}
	// Also match common opaque field names that carry the compact blob.
	if bytes.Contains(body, []byte(`"type":"compaction"`)) ||
		bytes.Contains(body, []byte(`"type": "compaction"`)) ||
		bytes.Contains(body, []byte(`"type":"compaction_trigger"`)) ||
		bytes.Contains(body, []byte(`"type": "compaction_trigger"`)) {
		return true
	}
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		// Non-array input may still be a compact-only payload on /compact.
		return bytes.Contains(body, []byte("compaction"))
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		typ := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		if strings.Contains(typ, "compaction") || strings.HasPrefix(typ, "compact_") || typ == "compact" {
			found = true
			return false
		}
		if item.Get("compaction").Exists() || item.Get("compact_result").Exists() {
			found = true
			return false
		}
		return true
	})
	return found
}

func applyGrokCacheHeaders(headers http.Header, identity string) {
	if headers == nil {
		return
	}
	identity = strings.TrimSpace(identity)
	if identity == "" {
		headers.Del(grokConversationIDHeader)
		return
	}
	headers.Set(grokConversationIDHeader, identity)
}

func stripGrokChatPromptCacheKey(body []byte) ([]byte, error) {
	if !gjson.GetBytes(body, "prompt_cache_key").Exists() {
		return body, nil
	}
	return sjson.DeleteBytes(body, "prompt_cache_key")
}
