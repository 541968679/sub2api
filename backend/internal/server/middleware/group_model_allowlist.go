package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupModelAllowlist is request-admission middleware for group model allowlists.
//
// Default OFF: missing/false group_model_allowlist_enforce skips the check
// entirely (does not read the body). Pack 6 does not turn enforce on.
func GroupModelAllowlist(settingService *service.SettingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if settingService == nil || !settingService.IsGroupModelAllowlistEnforceEnabled(c.Request.Context()) {
			c.Next()
			return
		}
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || !apiKey.Group.ModelAllowlistEnabled() {
			c.Next()
			return
		}
		allowlist := apiKey.Group.ModelAllowlist
		if c.Request == nil {
			c.Next()
			return
		}
		if isResponsesWebSocketRoute(c) {
			c.Next()
			return
		}

		var models []string
		if model := groupModelAllowlistModelFromParams(c); model != "" {
			models = []string{model}
		} else {
			switch c.Request.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				candidates, done := groupModelAllowlistModelsFromBody(c)
				if !done {
					return
				}
				models = candidates
			}
			if len(models) == 0 {
				if model := strings.TrimSpace(c.Query("model")); model != "" {
					models = []string{model}
				}
			}
		}

		blocked := ""
		for _, candidate := range models {
			if !allowlist.Allows(candidate) {
				blocked = candidate
				break
			}
		}
		if blocked == "" {
			c.Next()
			return
		}

		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
		c.Set("ingress_reject_reason", "model_not_allowed")
		groupModelAllowlistErrorWriter(c)(c, http.StatusNotFound, fmt.Sprintf("Model %q is not available for this group", blocked))
		c.Abort()
	}
}

func isResponsesWebSocketRoute(c *gin.Context) bool {
	if c.Request == nil || c.Request.Method != http.MethodGet {
		return false
	}
	switch c.FullPath() {
	case "/v1/responses", "/responses", "/backend-api/codex/responses":
		return true
	}
	return false
}

func groupModelAllowlistModelsFromBody(c *gin.Context) ([]string, bool) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		status := http.StatusBadRequest
		message := "Failed to read request body"
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
		c.Abort()
		return nil, false
	}
	if c.Request != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
	}
	model := strings.TrimSpace(jsonTopLevelModel(body))
	if model == "" {
		return nil, true
	}
	return []string{model}, true
}

func jsonTopLevelModel(body []byte) string {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	raw, ok := payload["model"]
	if !ok {
		return ""
	}
	var model string
	if err := json.Unmarshal(raw, &model); err != nil {
		return ""
	}
	return model
}

func groupModelAllowlistModelFromParams(c *gin.Context) string {
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return model
	}
	if modelAction := strings.TrimSpace(c.Param("modelAction")); modelAction != "" {
		modelAction = strings.TrimPrefix(modelAction, "/")
		if idx := strings.LastIndex(modelAction, ":"); idx >= 0 {
			return strings.TrimSpace(modelAction[:idx])
		}
		return modelAction
	}
	return ""
}

func groupModelAllowlistErrorWriter(c *gin.Context) GatewayErrorWriter {
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	switch {
	case strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta"):
		return GoogleErrorWriter
	case strings.Contains(path, "/messages"):
		return AnthropicErrorWriter
	default:
		return OpenAIErrorWriter
	}
}

// OpenAIErrorWriter 按 OpenAI API 规范输出模型级错误（model_not_found）。
func OpenAIErrorWriter(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "invalid_request_error",
			"code":    "model_not_found",
		},
	})
}
