package service

import "github.com/gin-gonic/gin"

const featureKeyCodexImageGenerationBridge = "codex_image_generation_bridge"
const featureKeyOpenAIImagesEndpointEnabled = "openai_images_endpoint_enabled"
const openAICodexImageGenerationBridgeResponseEnabledKey = "openai_codex_image_generation_bridge_response_enabled"

func boolOverridePtr(v bool) *bool {
	return &v
}

func boolOverrideFromMap(values map[string]any, keys ...string) *bool {
	if values == nil {
		return nil
	}
	for _, key := range keys {
		if v, ok := values[key].(bool); ok {
			return boolOverridePtr(v)
		}
	}
	return nil
}

func (a *Account) CodexImageGenerationBridgeOverride() *bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Extra == nil {
		return nil
	}
	if override := boolOverrideFromMap(a.Extra, featureKeyCodexImageGenerationBridge, "codex_image_generation_bridge_enabled"); override != nil {
		return override
	}
	openaiConfig, _ := a.Extra[PlatformOpenAI].(map[string]any)
	return boolOverrideFromMap(openaiConfig, featureKeyCodexImageGenerationBridge, "codex_image_generation_bridge_enabled")
}

func (a *Account) OpenAIImagesEndpointEnabled() bool {
	if a == nil || a.Platform != PlatformOpenAI {
		return false
	}
	if override := boolOverrideFromMap(a.Extra, featureKeyOpenAIImagesEndpointEnabled); override != nil {
		return *override
	}
	openaiConfig, _ := a.Extra[PlatformOpenAI].(map[string]any)
	if override := boolOverrideFromMap(openaiConfig, featureKeyOpenAIImagesEndpointEnabled); override != nil {
		return *override
	}
	return true
}

func (s *OpenAIGatewayService) isCodexImageGenerationBridgeEnabled(account *Account) bool {
	if override := account.CodexImageGenerationBridgeOverride(); override != nil {
		return *override
	}
	return s != nil && s.cfg != nil && s.cfg.Gateway.CodexImageGenerationBridgeEnabled
}

func setOpenAICodexImageGenerationBridgeResponseEnabled(c *gin.Context, enabled bool) {
	if c != nil {
		c.Set(openAICodexImageGenerationBridgeResponseEnabledKey, enabled)
	}
}

func isOpenAICodexImageGenerationBridgeResponseEnabled(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, exists := c.Get(openAICodexImageGenerationBridgeResponseEnabledKey)
	if !exists {
		return false
	}
	enabled, _ := value.(bool)
	return enabled
}
