// Package openai_compat 提供 OpenAI 协议族在不同上游间的能力差异判定工具。
//
// 背景：sub2api 的 OpenAI APIKey 账号通过 base_url 接入多种第三方 OpenAI 兼容上游
// （DeepSeek、Kimi、GLM、Qwen 等）。这些上游普遍只支持 /v1/chat/completions，
// 不存在 /v1/responses 端点。但网关历史代码无差别走 CC→Responses 转换并打到
// /v1/responses，导致兼容上游 404。
//
// 本包提供基于"账号探测标记"的能力判定，配合
// internal/service/openai_apikey_responses_probe.go 在创建/修改账号时一次性
// 探测并落标。
//
// 设计取舍：
//   - 不维护静态 host 白名单——避免新增厂商时必须改代码（讨论沉淀于
//     pensieve/short-term/knowledge/upstream-capability-detection-design-tradeoffs）
//   - 标记缺失时默认走 Responses，保持与重构前老代码完全一致的存量
//     账号行为（"现状即证据"原则；详见
//     pensieve/short-term/maxims/preserve-existing-runtime-behavior-when-replacing-logic-in-stateful-systems）
//   - auto 在两路都可用时仍把入站 CC 转到 Responses；原样映射必须显式
//     openai_responses_mode=passthrough，禁止生产默认突变
package openai_compat

// AccountResponsesSupport 描述账号上游对某一 OpenAI HTTP 端点的探测状态。
//
// 仅用于 platform=openai + type=apikey 的账号；其他账号类型不应调用本包判定。
type AccountResponsesSupport int

const (
	// ResponsesSupportUnknown 表示账号尚未完成能力探测（extra 字段缺失）。
	// 上游路由层应按"现状即证据"原则默认走 Responses，保持与重构前一致。
	ResponsesSupportUnknown AccountResponsesSupport = iota

	// ResponsesSupportYes 探测确认上游支持该端点。
	ResponsesSupportYes

	// ResponsesSupportNo 探测确认上游不支持该端点。
	ResponsesSupportNo
)

// ResponsesSupportMode 描述账号级 Responses / Chat Completions 路由覆盖模式。
type ResponsesSupportMode string

const (
	// ResponsesSupportModeAuto 表示跟随自动探测结果（入站 CC 在 Responses
	// 可用/未知时仍转 Responses，与历史行为一致）。
	ResponsesSupportModeAuto ResponsesSupportMode = "auto"

	// ResponsesSupportModeForceResponses 强制使用 /v1/responses。
	ResponsesSupportModeForceResponses ResponsesSupportMode = "force_responses"

	// ResponsesSupportModeForceChatCompletions 强制使用 /v1/chat/completions。
	ResponsesSupportModeForceChatCompletions ResponsesSupportMode = "force_chat_completions"

	// ResponsesSupportModePassthrough 入站端点原样映射到上游（CC→CC 且
	// Responses→Responses）。显式覆盖探测，失败不自动改桥。
	ResponsesSupportModePassthrough ResponsesSupportMode = "passthrough"
)

// InboundEndpoint 是网关看到的入站协议面。两条入站路必须分开判定。
type InboundEndpoint int

const (
	// InboundChatCompletions 入站 /v1/chat/completions。
	InboundChatCompletions InboundEndpoint = iota

	// InboundResponses 入站 /v1/responses。
	InboundResponses
)

// UpstreamEndpoint 是实际上游 HTTP 端点。
type UpstreamEndpoint int

const (
	// UpstreamResponses 上游 /v1/responses。
	UpstreamResponses UpstreamEndpoint = iota

	// UpstreamChatCompletions 上游 /v1/chat/completions。
	UpstreamChatCompletions
)

// ExtraKeyResponsesMode 是 accounts.extra JSON 中存储手动覆盖模式的键名。
// 值类型为 string：auto=跟随探测，force_responses=强制 Responses，
// force_chat_completions=强制 Chat Completions，passthrough=入站=上游。
const ExtraKeyResponsesMode = "openai_responses_mode"

// ExtraKeyResponsesSupported 是 accounts.extra JSON 中存储 Responses 探测结果的键名。
// 值类型为 bool：true=支持、false=不支持、键缺失=未探测。
const ExtraKeyResponsesSupported = "openai_responses_supported"

// ExtraKeyChatCompletionsSupported 是 accounts.extra JSON 中存储 Chat Completions
// 探测结果的键名。仅展示/落标，不参与 auto 改路。
const ExtraKeyChatCompletionsSupported = "openai_chat_completions_supported"

// NormalizeResponsesSupportMode 归一化账号级路由覆盖模式。
// 缺失或非法值按 auto 处理，以保持存量行为。禁止把未知字符串映射成 passthrough。
func NormalizeResponsesSupportMode(mode string) ResponsesSupportMode {
	switch ResponsesSupportMode(mode) {
	case ResponsesSupportModeForceResponses:
		return ResponsesSupportModeForceResponses
	case ResponsesSupportModeForceChatCompletions:
		return ResponsesSupportModeForceChatCompletions
	case ResponsesSupportModePassthrough:
		return ResponsesSupportModePassthrough
	default:
		return ResponsesSupportModeAuto
	}
}

// ResponsesSupportModeFromExtra 读取 extra 中的路由模式（缺省/非法为 auto）。
func ResponsesSupportModeFromExtra(extra map[string]any) ResponsesSupportMode {
	if extra == nil {
		return ResponsesSupportModeAuto
	}
	mode, _ := extra[ExtraKeyResponsesMode].(string)
	return NormalizeResponsesSupportMode(mode)
}

func probeFlagFromExtra(extra map[string]any, key string) AccountResponsesSupport {
	if extra == nil {
		return ResponsesSupportUnknown
	}
	v, ok := extra[key]
	if !ok {
		return ResponsesSupportUnknown
	}
	supported, ok := v.(bool)
	if !ok {
		return ResponsesSupportUnknown
	}
	if supported {
		return ResponsesSupportYes
	}
	return ResponsesSupportNo
}

// ResolveResponsesProbeSupport 只读 openai_responses_supported，不折叠 force_*。
func ResolveResponsesProbeSupport(extra map[string]any) AccountResponsesSupport {
	return probeFlagFromExtra(extra, ExtraKeyResponsesSupported)
}

// ResolveChatCompletionsProbeSupport 只读 openai_chat_completions_supported。
// 结果不进入 auto 路由。
func ResolveChatCompletionsProbeSupport(extra map[string]any) AccountResponsesSupport {
	return probeFlagFromExtra(extra, ExtraKeyChatCompletionsSupported)
}

// ResolveResponsesSupport 从账号的 extra map 中读取手动覆盖模式与探测标记。
//
// force_* 仍折叠进三态，供旧调用方 / 旧测试使用。passthrough 不折叠，回落到探测标记。
// 两条入站路的分流请用 ResolveUpstreamAPI，不要再用本函数绑死。
func ResolveResponsesSupport(extra map[string]any) AccountResponsesSupport {
	if extra == nil {
		return ResponsesSupportUnknown
	}
	if mode, ok := extra[ExtraKeyResponsesMode].(string); ok {
		switch NormalizeResponsesSupportMode(mode) {
		case ResponsesSupportModeForceResponses:
			return ResponsesSupportYes
		case ResponsesSupportModeForceChatCompletions:
			return ResponsesSupportNo
		}
	}
	return ResolveResponsesProbeSupport(extra)
}

// ResolveUpstreamAPI 按 (inbound, extra) 决定上游端点。
//
// CCsupp 探测结果不进入 auto 分支。缺 extra / 非法 mode / 未探测 = 今天的 auto+unknown。
func ResolveUpstreamAPI(inbound InboundEndpoint, extra map[string]any) UpstreamEndpoint {
	switch ResponsesSupportModeFromExtra(extra) {
	case ResponsesSupportModeForceResponses:
		return UpstreamResponses
	case ResponsesSupportModeForceChatCompletions:
		return UpstreamChatCompletions
	case ResponsesSupportModePassthrough:
		if inbound == InboundChatCompletions {
			return UpstreamChatCompletions
		}
		return UpstreamResponses
	default:
		if ResolveResponsesProbeSupport(extra) == ResponsesSupportNo {
			return UpstreamChatCompletions
		}
		return UpstreamResponses
	}
}

// ShouldUseResponsesAPI 判断 OpenAI APIKey 账号的入站 /v1/chat/completions 请求
// 是否应走"CC→Responses 转换 + 上游 /v1/responses"路径。
//
// 语义收窄为 ResolveUpstreamAPI(InboundChatCompletions)==UpstreamResponses。
// 入站 /v1/responses 禁止再调用本函数，应改用 ResolveUpstreamAPI(InboundResponses)。
func ShouldUseResponsesAPI(extra map[string]any) bool {
	return ResolveUpstreamAPI(InboundChatCompletions, extra) == UpstreamResponses
}
