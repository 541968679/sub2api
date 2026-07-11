package domain

import "strings"

// Status constants
const (
	StatusActive          = "active"
	StatusDisabled        = "disabled"
	StatusPendingApproval = "pending_approval"
	StatusError           = "error"
	StatusUnused          = "unused"
	StatusUsed            = "used"
	StatusExpired         = "expired"
)

// Role constants
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Platform constants
const (
	PlatformAnthropic   = "anthropic"
	PlatformOpenAI      = "openai"
	PlatformGemini      = "gemini"
	PlatformAntigravity = "antigravity"
	PlatformGrok        = "grok"
)

const (
	MappingBillingObjectRequested = "requested"
	MappingBillingObjectMapped    = "mapped"
)

// Account type constants
const (
	AccountTypeOAuth          = "oauth"           // OAuth类型账号（full scope: profile + inference）
	AccountTypeSetupToken     = "setup-token"     // Setup Token类型账号（inference only scope）
	AccountTypeAPIKey         = "apikey"          // API Key类型账号
	AccountTypeUpstream       = "upstream"        // 上游透传类型账号（通过 Base URL + API Key 连接上游）
	AccountTypeBedrock        = "bedrock"         // AWS Bedrock 类型账号（通过 SigV4 签名或 API Key 连接 Bedrock，由 credentials.auth_mode 区分）
	AccountTypeServiceAccount = "service_account" // Google Service Account 类型账号（用于 Vertex AI）
)

// Redeem type constants
const (
	RedeemTypeBalance      = "balance"
	RedeemTypeConcurrency  = "concurrency"
	RedeemTypeSubscription = "subscription"
	RedeemTypeInvitation   = "invitation"
)

// PromoCode status constants
const (
	PromoCodeStatusActive   = "active"
	PromoCodeStatusDisabled = "disabled"
)

// Admin adjustment type constants
const (
	AdjustmentTypeAdminBalance     = "admin_balance"     // 管理员调整余额
	AdjustmentTypeAdminConcurrency = "admin_concurrency" // 管理员调整并发数
)

// Group subscription type constants
const (
	SubscriptionTypeStandard     = "standard"     // 标准计费模式（按余额扣费）
	SubscriptionTypeSubscription = "subscription" // 订阅模式（按限额控制）
)

// Subscription status constants
const (
	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusSuspended = "suspended"
)

// GetAntigravityDefaultMappingOverride 可在应用启动时设置，用于从 settings 表读取管理员自定义的默认映射。
// 返回 nil 表示使用内置 DefaultAntigravityModelMapping。
var GetAntigravityDefaultMappingOverride func() map[string]string

// GetPlatformDefaultMappingOverride 可在应用启动时设置，用于从 settings 表读取平台级默认映射。
// 返回 nil 表示该平台没有已保存的默认映射覆盖。
var GetPlatformDefaultMappingOverride func(platform string) map[string]string

// GetPlatformDefaultMappingBillingObjectOverride returns per-mapping billing
// object overrides keyed by mapping source model/pattern.
var GetPlatformDefaultMappingBillingObjectOverride func(platform string) map[string]string

// GetModelPricingHiddenModelsOverride 可在应用启动时设置，用于从 settings 表读取
// 管理员在模型配置页删除（隐藏）的模型集合，键为小写模型名。
// 仅影响模型配置列表展示，不影响计费与请求转发。
var GetModelPricingHiddenModelsOverride func() map[string]bool

// ResolveModelPricingHiddenModels 返回模型配置页隐藏的模型集合（可能为 nil）。
func ResolveModelPricingHiddenModels() map[string]bool {
	if GetModelPricingHiddenModelsOverride == nil {
		return nil
	}
	return GetModelPricingHiddenModelsOverride()
}

func NormalizeMappingBillingObject(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MappingBillingObjectRequested:
		return MappingBillingObjectRequested
	case MappingBillingObjectMapped:
		return MappingBillingObjectMapped
	default:
		return ""
	}
}

// ResolvePlatformDefaultModelMapping 返回平台级默认映射。
// Antigravity 有内置严格白名单；Anthropic 有 LiteLLM/官方命名兼容别名；其他平台默认没有映射，只有管理员配置后才返回。
func ResolvePlatformDefaultModelMapping(platform string) map[string]string {
	if GetPlatformDefaultMappingOverride != nil {
		if m := GetPlatformDefaultMappingOverride(platform); m != nil {
			return m
		}
	}
	switch platform {
	case PlatformAntigravity:
		return DefaultAntigravityModelMapping
	case PlatformAnthropic:
		return DefaultAnthropicModelMapping
	}
	return nil
}

func ResolvePlatformDefaultMappingBillingObjects(platform string) map[string]string {
	if GetPlatformDefaultMappingBillingObjectOverride == nil {
		return nil
	}
	return GetPlatformDefaultMappingBillingObjectOverride(platform)
}

func ResolvePlatformDefaultMappingBillingObject(platform, mappingKey string) string {
	mappingKey = strings.TrimSpace(mappingKey)
	if mappingKey != "" {
		objects := ResolvePlatformDefaultMappingBillingObjects(platform)
		if len(objects) > 0 {
			if object := NormalizeMappingBillingObject(objects[mappingKey]); object != "" {
				return object
			}
		}
	}
	return MappingBillingObjectRequested
}

// ResolveAntigravityDefaultMapping 返回管理员自定义映射（如有），否则返回内置默认映射。
func ResolveAntigravityDefaultMapping() map[string]string {
	if GetAntigravityDefaultMappingOverride != nil {
		if m := GetAntigravityDefaultMappingOverride(); m != nil {
			return m
		}
	}
	return ResolvePlatformDefaultModelMapping(PlatformAntigravity)
}

// DefaultAntigravityModelMapping 是 Antigravity 平台的默认模型映射
// 当账号未配置 model_mapping 时使用此默认值
// 与前端 useModelWhitelist.ts 中的 antigravityDefaultMappings 保持一致
var DefaultAntigravityModelMapping = map[string]string{
	// Claude 白名单
	"claude-fable-5":             "claude-fable-5",           // 官方模型
	"claude-opus-4-8":            "claude-opus-4-8",          // 官方模型
	"claude-opus-4-7":            "claude-opus-4-7",          // 官方模型
	"claude-opus-4-6-thinking":   "claude-opus-4-6-thinking", // 官方模型
	"claude-opus-4-6":            "claude-opus-4-6-thinking", // 简称映射
	"claude-opus-4-5-thinking":   "claude-opus-4-6-thinking", // 迁移旧模型
	"claude-sonnet-4-6":          "claude-sonnet-4-6",
	"claude-sonnet-4-5":          "claude-sonnet-4-5",
	"claude-sonnet-4-5-thinking": "claude-sonnet-4-5-thinking",
	// Claude 详细版本 ID 映射
	"claude-opus-4-5-20251101":   "claude-opus-4-6-thinking", // 迁移旧模型
	"claude-sonnet-4-5-20250929": "claude-sonnet-4-5",
	// Claude Haiku → Sonnet（无 Haiku 支持）
	"claude-haiku-4-5":          "claude-sonnet-4-6",
	"claude-haiku-4-5-20251001": "claude-sonnet-4-6",
	// Gemini 2.5 白名单
	"gemini-2.5-flash":               "gemini-2.5-flash",
	"gemini-2.5-flash-image":         "gemini-2.5-flash-image",
	"gemini-2.5-flash-image-preview": "gemini-2.5-flash-image",
	"gemini-2.5-flash-lite":          "gemini-2.5-flash-lite",
	"gemini-2.5-flash-thinking":      "gemini-2.5-flash-thinking",
	"gemini-2.5-pro":                 "gemini-2.5-pro",
	// Gemini 3 白名单
	"gemini-3-flash":    "gemini-3-flash",
	"gemini-3-pro-high": "gemini-3-pro-high",
	"gemini-3-pro-low":  "gemini-3-pro-low",
	// Gemini 3 preview 映射
	"gemini-3-flash-preview": "gemini-3-flash",
	"gemini-3-pro-preview":   "gemini-3-pro-high",
	// Gemini 3.1 白名单
	"gemini-3.1-pro-high": "gemini-3.1-pro-high",
	"gemini-3.1-pro-low":  "gemini-3.1-pro-low",
	// Gemini 3.1 preview 映射
	"gemini-3.1-pro-preview": "gemini-3.1-pro-high",
	// Gemini 3.1 image 白名单
	"gemini-3.1-flash-image": "gemini-3.1-flash-image",
	// Gemini 3.1 image preview 映射
	"gemini-3.1-flash-image-preview": "gemini-3.1-flash-image",
	// Gemini 3 image 兼容映射（向 3.1 image 迁移）
	"gemini-3-pro-image":         "gemini-3.1-flash-image",
	"gemini-3-pro-image-preview": "gemini-3.1-flash-image",
	// 其他官方模型
	"gpt-oss-120b-medium":    "gpt-oss-120b-medium",
	"tab_flash_lite_preview": "tab_flash_lite_preview",
}

// DefaultAnthropicModelMapping normalizes common LiteLLM Anthropic aliases to
// the official Anthropic request model names used elsewhere in Sub2API.
var DefaultAnthropicModelMapping = map[string]string{
	"claude-4-opus-20250514":   "claude-opus-4-20250514",
	"claude-4-sonnet-20250514": "claude-sonnet-4-20250514",
}

// DefaultBedrockModelMapping 是 AWS Bedrock 平台的默认模型映射
// 将 Anthropic 标准模型名映射到 Bedrock 模型 ID
// 注意：此处的 "us." 前缀仅为默认值，ResolveBedrockModelID 会根据账号配置的
// aws_region 自动调整为匹配的区域前缀（如 eu.、apac.、jp. 等）
var DefaultBedrockModelMapping = map[string]string{
	// Claude Fable
	"claude-fable-5": "anthropic.claude-fable-5",
	// Claude Opus
	"claude-opus-4-8":          "us.anthropic.claude-opus-4-8-v1",
	"claude-opus-4-7":          "us.anthropic.claude-opus-4-7-v1",
	"claude-opus-4-6-thinking": "us.anthropic.claude-opus-4-6-v1",
	"claude-opus-4-6":          "us.anthropic.claude-opus-4-6-v1",
	"claude-opus-4-5-thinking": "us.anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-5-20251101": "us.anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-1":          "us.anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-opus-4-20250514":   "us.anthropic.claude-opus-4-20250514-v1:0",
	// Claude Sonnet
	"claude-sonnet-5":            "us.anthropic.claude-sonnet-5-v1",
	"claude-sonnet-4-6-thinking": "us.anthropic.claude-sonnet-4-6",
	"claude-sonnet-4-6":          "us.anthropic.claude-sonnet-4-6",
	"claude-sonnet-4-5":          "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-5-thinking": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-5-20250929": "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-20250514":   "us.anthropic.claude-sonnet-4-20250514-v1:0",
	// Claude Haiku
	"claude-haiku-4-5":          "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-haiku-4-5-20251001": "us.anthropic.claude-haiku-4-5-20251001-v1:0",
}
