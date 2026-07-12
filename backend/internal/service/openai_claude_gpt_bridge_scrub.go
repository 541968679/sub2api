package service

import (
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// 桥接模式出口消毒器：Claude-GPT bridge 的下游客户端只应看到 Claude/Anthropic
// 语义。上游 OpenAI 的错误文本、URL、组织 id 和 gpt-* 模型名会经多条透传通道
// 到达客户端，构成提供商身份泄漏。以下函数把这些品牌指纹从客户端可见字符串中
// 清除，仅在 bridge 模式生效，OpenAI 原生分组不受影响。

var (
	// 品牌词（含所有格）整体移除，例如 "Your OpenAI account" -> "Your account"。
	bridgeBrandWordRegex = regexp.MustCompile(`(?i)\b(openai|chatgpt|chat gpt|codex)('s)?\b`)
	// gpt-* / gpt_* 模型名替换为请求的 Claude 模型或中性占位。
	bridgeGPTModelRegex = regexp.MustCompile(`(?i)\bgpt[-_][a-z0-9.\-]+\b`)
	// 指向 openai.com / chatgpt.com 的 URL 整体掩蔽。
	bridgeProviderURLRegex = regexp.MustCompile(`(?i)https?://\S*(?:openai|chatgpt)\.com\S*`)
	// OpenAI 组织 id（org-xxxxxxxx）掩蔽。
	bridgeOrgIDRegex = regexp.MustCompile(`\borg-[A-Za-z0-9]{6,}\b`)
	// 清理移除品牌词后遗留的多余空白。
	bridgeMultiSpaceRegex = regexp.MustCompile(`[ \t]{2,}`)
	// 清理遗留的孤立标点前空格，如 " ." / " ,"。
	bridgeSpaceBeforePunctRegex = regexp.MustCompile(`\s+([.,:;!?])`)
)

const bridgeScrubModelContextKey = "openai_claude_gpt_bridge_scrub_model"

// SetBridgeScrubModel records the requested Claude model so provider-identity
// scrubbing can replace leaked gpt-* upstream model names with the model the
// client actually asked for. Safe to call with an empty model.
func SetBridgeScrubModel(c *gin.Context, claudeModel string) {
	if c == nil {
		return
	}
	claudeModel = strings.TrimSpace(claudeModel)
	if claudeModel == "" {
		return
	}
	c.Set(bridgeScrubModelContextKey, claudeModel)
}

func bridgeScrubModelReplacement(c *gin.Context) string {
	if c != nil {
		if v, ok := c.Get(bridgeScrubModelContextKey); ok {
			if model, ok := v.(string); ok && strings.TrimSpace(model) != "" {
				return strings.TrimSpace(model)
			}
		}
	}
	return "the model"
}

// ScrubProviderIdentityText removes upstream provider brand identity (OpenAI,
// ChatGPT, Codex), gpt-* model names, provider URLs, and org ids from a
// client-visible string. modelReplacement substitutes for gpt-* tokens (pass
// the requested Claude model, or a neutral noun). It is provider-agnostic and
// pure, exported so the handler layer can scrub replayed upstream error bodies.
func ScrubProviderIdentityText(s, modelReplacement string) string {
	if s == "" {
		return s
	}
	if strings.TrimSpace(modelReplacement) == "" {
		modelReplacement = "the model"
	}
	out := bridgeProviderURLRegex.ReplaceAllString(s, "the upstream endpoint")
	out = bridgeOrgIDRegex.ReplaceAllString(out, "[redacted]")
	out = bridgeGPTModelRegex.ReplaceAllString(out, modelReplacement)
	out = bridgeBrandWordRegex.ReplaceAllString(out, "")
	out = bridgeMultiSpaceRegex.ReplaceAllString(out, " ")
	out = bridgeSpaceBeforePunctRegex.ReplaceAllString(out, "$1")
	// 移除品牌词后可能遗留 " the account" 前的双空格已处理；再修剪首尾。
	return strings.TrimSpace(out)
}

// scrubBridgeClientText scrubs a client-visible message only in Claude-GPT
// bridge mode; OpenAI-platform requests keep their original provider wording.
func scrubBridgeClientText(c *gin.Context, msg string) string {
	if msg == "" || !isOpenAIClaudeGPTBridgeForward(c) {
		return msg
	}
	return ScrubProviderIdentityText(msg, bridgeScrubModelReplacement(c))
}

// ScrubBridgeClientText is the exported entry for the handler layer's failover
// replay path, where raw upstream error type/message extracted from an
// UpstreamFailoverError body must be neutralized before reaching a bridge
// client. It is a no-op outside bridge mode.
func ScrubBridgeClientText(c *gin.Context, msg string) string {
	return scrubBridgeClientText(c, msg)
}
