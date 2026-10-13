package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/sync/singleflight"
)

const forceRequestedResponseModelContextKey = "force_requested_response_model"

// clientFacingModelFieldPaths are the structured response fields that name the
// model for a client. Free-text error messages are not rewritten.
var clientFacingModelFieldPaths = []string{
	"model",
	"response.model",
	"message.model",
	"modelVersion",
	"response.modelVersion",
	"response.response.modelVersion",
}

type cachedForceRequestedResponseModel struct {
	enabled   bool
	expiresAt int64
}

var forceRequestedResponseModelCache atomic.Value // *cachedForceRequestedResponseModel
var forceRequestedResponseModelSF singleflight.Group

const forceRequestedResponseModelCacheTTL = 60 * time.Second
const forceRequestedResponseModelErrorTTL = 5 * time.Second
const forceRequestedResponseModelDBTimeout = 5 * time.Second

func parseForceRequestedResponseModelEnabled(raw string) bool {
	return strings.TrimSpace(raw) == "true"
}

func storeForceRequestedResponseModelCache(enabled bool, ttl time.Duration) {
	forceRequestedResponseModelCache.Store(&cachedForceRequestedResponseModel{
		enabled:   enabled,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

func refreshForceRequestedResponseModelCache(enabled bool) {
	forceRequestedResponseModelSF.Forget("force_requested_response_model")
	storeForceRequestedResponseModelCache(enabled, forceRequestedResponseModelCacheTTL)
}

// IsForceRequestedResponseModelEnabled reports whether client-facing response
// model names are overwritten with the model the client requested.
// Missing, invalid, unread, and nil service values are off. Only the stored
// string "true" enables it.
func (s *SettingService) IsForceRequestedResponseModelEnabled(ctx context.Context) bool {
	if s == nil {
		return false
	}
	if cached, ok := forceRequestedResponseModelCache.Load().(*cachedForceRequestedResponseModel); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled
		}
	}
	result, _, _ := forceRequestedResponseModelSF.Do("force_requested_response_model", func() (any, error) {
		if cached, ok := forceRequestedResponseModelCache.Load().(*cachedForceRequestedResponseModel); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.enabled, nil
			}
		}
		if s.settingRepo == nil {
			storeForceRequestedResponseModelCache(false, forceRequestedResponseModelCacheTTL)
			return false, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), forceRequestedResponseModelDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyForceRequestedResponseModel)
		if err != nil {
			if !errors.Is(err, ErrSettingNotFound) {
				slog.Warn("failed to get force_requested_response_model setting", "error", err)
				storeForceRequestedResponseModelCache(false, forceRequestedResponseModelErrorTTL)
				return false, nil
			}
			storeForceRequestedResponseModelCache(false, forceRequestedResponseModelCacheTTL)
			return false, nil
		}
		enabled := parseForceRequestedResponseModelEnabled(value)
		storeForceRequestedResponseModelCache(enabled, forceRequestedResponseModelCacheTTL)
		return enabled, nil
	})
	if val, ok := result.(bool); ok {
		return val
	}
	return false
}

// ArmForceRequestedResponseModel records the client-requested model for this
// request when the admin switch is on. The first non-empty name wins, so a
// later call with the mapped upstream name does not replace it. Observation
// of the real upstream name must still happen before the response rewrite.
func ArmForceRequestedResponseModel(ctx context.Context, c *gin.Context, settings *SettingService, requestedModel string) {
	if c == nil || settings == nil || !settings.IsForceRequestedResponseModelEnabled(ctx) {
		return
	}
	if existing, ok := c.Get(forceRequestedResponseModelContextKey); ok {
		if text, _ := existing.(string); strings.TrimSpace(text) != "" {
			return
		}
	}
	c.Set(forceRequestedResponseModelContextKey, strings.TrimSpace(requestedModel))
}

func (s *GatewayService) armForceRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	if s == nil {
		return
	}
	ArmForceRequestedResponseModel(ctx, c, s.settingService, requestedModel)
}

// ArmClientRequestedResponseModel records the model name from the inbound
// request. Call it before channel mapping rewrites the body. A later arm with
// the mapped name does not replace a name already stored.
func (s *GatewayService) ArmClientRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	s.armForceRequestedResponseModel(ctx, c, requestedModel)
}

func (s *OpenAIGatewayService) armForceRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	if s == nil {
		return
	}
	ArmForceRequestedResponseModel(ctx, c, s.settingService, requestedModel)
}

// ArmClientRequestedResponseModel records the model name from the inbound
// request. Call it before channel mapping rewrites the body.
func (s *OpenAIGatewayService) ArmClientRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	s.armForceRequestedResponseModel(ctx, c, requestedModel)
}

func (s *AntigravityGatewayService) armForceRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	if s == nil {
		return
	}
	ArmForceRequestedResponseModel(ctx, c, s.settingService, requestedModel)
}

// ArmClientRequestedResponseModel records the model name from the inbound
// request. Call it before channel mapping rewrites the body.
func (s *AntigravityGatewayService) ArmClientRequestedResponseModel(ctx context.Context, c *gin.Context, requestedModel string) {
	s.armForceRequestedResponseModel(ctx, c, requestedModel)
}

func forcedRequestedResponseModel(c *gin.Context) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.Get(forceRequestedResponseModelContextKey)
	if !ok {
		return "", false
	}
	model, _ := value.(string)
	model = strings.TrimSpace(model)
	return model, model != ""
}

func forceRequestedResponseModelArmed(c *gin.Context) bool {
	_, ok := forcedRequestedResponseModel(c)
	return ok
}

func applyForcedRequestedResponseModelBody(c *gin.Context, body []byte) []byte {
	requested, ok := forcedRequestedResponseModel(c)
	if !ok || len(body) == 0 {
		return body
	}
	next, changed := forceRewriteClientModelFields(body, requested)
	if !changed {
		return body
	}
	return next
}

func applyForcedRequestedResponseModelLine(c *gin.Context, line string) string {
	requested, ok := forcedRequestedResponseModel(c)
	if !ok || line == "" {
		return line
	}
	if payload, prefix, found := splitClientModelDataLine(line); found {
		trimmed := strings.TrimSpace(payload)
		if trimmed == "" || trimmed == "[DONE]" {
			return line
		}
		rewritten, changed := forceRewriteClientModelFields([]byte(payload), requested)
		if !changed {
			return line
		}
		return prefix + string(rewritten)
	}
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return line
	}
	rewritten, changed := forceRewriteClientModelFields([]byte(trimmed), requested)
	if !changed {
		return line
	}
	return strings.Replace(line, trimmed, string(rewritten), 1)
}

func applyForcedRequestedResponseModelSSEText(c *gin.Context, body string) string {
	if !forceRequestedResponseModelArmed(c) || body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	changed := false
	for i, line := range lines {
		next := applyForcedRequestedResponseModelLine(c, line)
		if next != line {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return body
	}
	return strings.Join(lines, "\n")
}

func splitClientModelDataLine(line string) (payload, prefix string, ok bool) {
	trimmedLeft := strings.TrimLeft(line, " \t")
	pad := line[:len(line)-len(trimmedLeft)]
	const marker = "data:"
	if !strings.HasPrefix(trimmedLeft, marker) {
		return "", "", false
	}
	rest := trimmedLeft[len(marker):]
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	return rest[i:], pad + trimmedLeft[:len(marker)+i], true
}

func forceRewriteClientModelFields(payload []byte, requestedModel string) ([]byte, bool) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" || len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload, false
	}
	root := gjson.ParseBytes(payload)
	if !root.IsObject() {
		return payload, false
	}
	updated := payload
	changed := false
	for _, path := range clientFacingModelFieldPaths {
		value := gjson.GetBytes(updated, path)
		if !value.Exists() || value.Type != gjson.String {
			continue
		}
		current := strings.TrimSpace(value.Str)
		if current == "" || current == requestedModel {
			continue
		}
		next, err := sjson.SetBytes(updated, path, requestedModel)
		if err != nil {
			continue
		}
		updated = next
		changed = true
	}
	if !changed {
		return payload, false
	}
	return updated, true
}
