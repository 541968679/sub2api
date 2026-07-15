package handler

import (
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CodexModels serves the versioned Codex manifest used by CLI and desktop
// model pickers while leaving the ordinary OpenAI models list unchanged.
//
// When the OpenAI group has bound Grok accounts with OpenAI-group access
// enabled, Grok text models are injected into the manifest so Codex /model
// picker can see them. Non-Codex clients without client_version still use
// Gateway.Models (OpenAI list shape).
//
// If ChatGPT's upstream catalog is unreachable (common on local Windows when
// chatgpt.com times out), we still return a Codex-shaped body with injected
// Grok models so Desktop does not fall back to a GPT-only local catalog.
func (h *OpenAIGatewayHandler) CodexModels(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "API key group is required")
		return
	}
	if apiKey.Group.Platform != service.PlatformOpenAI {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Codex models manifest is only available for OpenAI groups")
		return
	}

	// CLI uses ?client_version=; Desktop often only sends Version header.
	clientVersion := strings.TrimSpace(c.Query("client_version"))
	if clientVersion == "" {
		clientVersion = strings.TrimSpace(c.GetHeader("Version"))
	}

	var (
		body []byte
		etag string
	)

	account, err := h.gatewayService.SelectAccountForCodexModels(c.Request.Context(), apiKey.GroupID)
	if err != nil {
		// No OAuth account to proxy ChatGPT catalog — still serve Grok inject.
		reqLog := requestLogger(c, "handler.openai_gateway.codex_models")
		reqLog.Warn("openai.codex_models.fallback_no_oauth_account", zap.Error(err))
		body = service.EmptyCodexModelsManifestBody(clientVersion)
	} else {
		manifest, fetchErr := h.gatewayService.FetchCodexModelsManifest(
			c.Request.Context(), account, clientVersion, c.GetHeader("If-None-Match"),
		)
		if fetchErr != nil {
			// chatgpt.com timeout/errors previously 502'd Desktop and left the
			// picker without injected Grok. Fall back to a local empty catalog
			// shell and inject Grok so the model remains selectable.
			reqLog := requestLogger(c, "handler.openai_gateway.codex_models")
			reqLog.Warn("openai.codex_models.fallback_upstream_failed",
				zap.Error(fetchErr),
				zap.Int("upstream_code", infraerrors.Code(fetchErr)),
				zap.String("upstream_message", infraerrors.Message(fetchErr)),
			)
			body = service.EmptyCodexModelsManifestBody(clientVersion)
		} else {
			// 304 responses have no body to inject into; clients revalidate via ETag.
			// When we inject Grok models we must not pass through the upstream ETag,
			// otherwise Codex may cache a pre-injection body forever.
			if manifest.NotModified {
				if manifest.ETag != "" {
					c.Header("ETag", manifest.ETag)
				}
				c.Status(http.StatusNotModified)
				return
			}
			body = manifest.Body
			etag = manifest.ETag
		}
	}

	// Always inject canonical grok-4.5 for OpenAI groups; also inject any extra
	// Grok text models from bound opt-in accounts.
	grokIDs := service.EnsureOpenAICanonicalGrokModels(
		h.gatewayService.ListGrokOpenAIGroupAccessModelIDs(c.Request.Context(), apiKey.GroupID),
	)
	if len(grokIDs) > 0 {
		if injected, injectErr := service.InjectGrokModelsIntoCodexManifest(body, grokIDs); injectErr == nil {
			body = injected
			// Body changed relative to upstream; drop ETag so clients do not
			// treat the injected document as identical to OpenAI's original.
			etag = ""
		}
	}

	if etag != "" {
		c.Header("ETag", etag)
	}
	c.Data(http.StatusOK, "application/json", body)
}
