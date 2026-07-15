package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	grokComposerImageBridgeVisionModel     = "grok-build-0.1"
	grokComposerImageBridgeMaxOutputTokens = 512
	grokUpstreamUserAgent                  = "sub2api-grok/1.0"
	grokCLIVersion                         = "0.2.93"
	grokDefaultResponsesModel              = "grok-4.5"
	grokRateLimitFallbackCooldown          = 2 * time.Minute
)

func (s *OpenAIGatewayService) forwardGrokResponses(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if account.Type != AccountTypeOAuth && account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("grok account type %s is not supported by Responses forwarding", account.Type)
	}

	upstreamModel := account.GetMappedModel(originalModel)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = grokDefaultResponsesModel
	}
	// Compaction-looking errors on turn 2 are usually incomplete
	// reasoning.encrypted_content items (missing summary), not real compact.
	// Still avoid heavy rewrites when a true compact item is present.
	preserveCompaction := hasGrokCompactionContext(body) || isOpenAIResponsesCompactPath(c)
	cacheIdentity := ""
	if !preserveCompaction {
		cacheIdentity = resolveGrokCacheIdentity(c, body, "", upstreamModel)
	}
	patchedBody, err := patchGrokResponsesBody(body, upstreamModel)
	if err != nil {
		return nil, err
	}
	if !preserveCompaction {
		patchedBody, err = applyGrokResponsesCacheIdentity(patchedBody, body, cacheIdentity, account.IsGrokOAuth())
		if err != nil {
			return nil, fmt.Errorf("apply grok prompt cache identity: %w", err)
		}
		// Cache-identity injection may leave tool_choice without tools (or skip
		// re-injection when the original payload had unsupported tools). Reconcile.
		patchedBody, err = sanitizeGrokResponsesTools(patchedBody)
		if err != nil {
			return nil, err
		}
	}
	// Codex multi-turn often echoes reasoning.encrypted_content without summary.
	// xAI then returns: "Could not decode the compaction blob..." even when no
	// compact ever ran. Ensure summary is present (empty list is accepted).
	patchedBody, err = ensureGrokReasoningEncryptedSummary(patchedBody)
	if err != nil {
		return nil, err
	}

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	// One automatic recovery: if xAI still rejects encrypted reasoning payload
	// (compaction-blob mislabel or decrypt failure), drop encrypted_content and
	// retry once — same strategy as OpenAI invalid_encrypted_content recovery.
	encryptedRetryTried := false
	for {
		upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
		upstreamReq, buildErr := buildGrokResponsesRequest(upstreamCtx, c, account, patchedBody, token, cacheIdentity)
		releaseUpstreamCtx()
		if buildErr != nil {
			return nil, buildErr
		}

		upstreamStart := time.Now()
		resp, doErr := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
		SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
		if doErr != nil {
			return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, doErr, false)
		}

		if resp.StatusCode >= 400 {
			respBody := s.readUpstreamErrorBody(resp)
			_ = resp.Body.Close()
			// Rewrite xAI string-error bodies into OpenAI {error:{message}} so
			// extractUpstreamErrorMessage / Desktop clients can render them.
			respBody = normalizeGrokUpstreamErrorBody(respBody)
			upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
			if upstreamMsg == "" {
				upstreamMsg = fmt.Sprintf("xAI upstream returned status %d", resp.StatusCode)
			}

			if !encryptedRetryTried &&
				resp.StatusCode == http.StatusBadRequest &&
				isGrokEncryptedReasoningUpstreamError(upstreamMsg) {
				if dropped, dropErr := dropGrokEncryptedReasoningFromBody(patchedBody); dropErr == nil && dropped != nil {
					patchedBody = dropped
					encryptedRetryTried = true
					slog.Info("grok encrypted reasoning recovery retry",
						"account_id", account.ID,
						"message", truncateString(upstreamMsg, 200),
					)
					continue
				}
			}

			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
				Platform:           account.Platform,
				AccountID:          account.ID,
				AccountName:        account.Name,
				UpstreamStatusCode: resp.StatusCode,
				UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
				Kind:               "failover",
				Message:            upstreamMsg,
			})
			s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
			if s.shouldFailoverUpstreamError(resp.StatusCode) {
				return nil, &UpstreamFailoverError{
					StatusCode:             resp.StatusCode,
					ResponseBody:           respBody,
					RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
				}
			}
			// Stream clients expect SSE error events, not a bare JSON body.
			if reqStream && c != nil && c.Writer != nil && !c.Writer.Written() {
				writeGrokResponsesStreamError(c, resp.StatusCode, upstreamMsg)
				return nil, fmt.Errorf("upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
			}
			return s.handleErrorResponse(ctx, resp, c, account, patchedBody)
		}

		// success path continues below with resp
		defer func() { _ = resp.Body.Close() }()

		s.updateGrokUsageSnapshot(ctx, account, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))

		var usage *OpenAIUsage
		var firstTokenMs *int
		responseID := ""
		if reqStream {
			streamResult, streamErr := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
			if streamErr != nil {
				return nil, streamErr
			}
			usage = streamResult.usage
			firstTokenMs = streamResult.firstTokenMs
			responseID = strings.TrimSpace(streamResult.responseID)
		} else {
			nonStreamResult, nonStreamErr := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
			if nonStreamErr != nil {
				return nil, nonStreamErr
			}
			usage = nonStreamResult.usage
			responseID = strings.TrimSpace(nonStreamResult.responseID)
		}

		if usage == nil {
			usage = &OpenAIUsage{}
		}
		reasoningEffort := extractOpenAIReasoningEffortFromBody(patchedBody, originalModel)
		return &OpenAIForwardResult{
			RequestID:       firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
			ResponseID:      responseID,
			Usage:           *usage,
			Model:           originalModel,
			UpstreamModel:   upstreamModel,
			ReasoningEffort: reasoningEffort,
			Stream:          reqStream,
			OpenAIWSMode:    false,
			ResponseHeaders: resp.Header.Clone(),
			Duration:        time.Since(startTime),
			FirstTokenMs:    firstTokenMs,
		}, nil
	}
}

// ensureGrokReasoningEncryptedSummary guarantees every reasoning item that
// carries encrypted_content also has a non-null summary array.
//
// Repro: second-turn Codex requests often echo encrypted_content without
// summary; xAI returns the misleading error:
// "Could not decode the compaction blob. Ensure it is unmodified from the compact response."
// even though no compact ran. An empty summary list is accepted.
func ensureGrokReasoningEncryptedSummary(body []byte) ([]byte, error) {
	input := gjson.GetBytes(body, "input")
	if !input.IsArray() {
		return body, nil
	}
	out := body
	var err error
	for i, item := range input.Array() {
		if strings.TrimSpace(item.Get("type").String()) != "reasoning" {
			continue
		}
		enc := item.Get("encrypted_content")
		// Only real (non-null) encrypted payloads need a summary companion.
		if !enc.Exists() || enc.Type == gjson.Null || strings.TrimSpace(enc.String()) == "" {
			continue
		}
		summary := item.Get("summary")
		// Missing or JSON null → set []
		if !summary.Exists() || summary.Type == gjson.Null {
			path := fmt.Sprintf("input.%d.summary", i)
			out, err = sjson.SetBytes(out, path, []any{})
			if err != nil {
				return nil, fmt.Errorf("ensure grok reasoning summary: %w", err)
			}
		}
	}
	return out, nil
}

func isGrokEncryptedReasoningUpstreamError(message string) bool {
	msg := strings.ToLower(strings.TrimSpace(message))
	if msg == "" {
		return false
	}
	if strings.Contains(msg, "compaction blob") {
		return true
	}
	if strings.Contains(msg, "encrypted_content") || strings.Contains(msg, "encrypted content") {
		return true
	}
	if strings.Contains(msg, "invalid_encrypted_content") {
		return true
	}
	return false
}

// dropGrokEncryptedReasoningFromBody removes encrypted_content from reasoning
// input items. Returns nil when nothing changed.
func dropGrokEncryptedReasoningFromBody(body []byte) ([]byte, error) {
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return nil, err
	}
	if !trimOpenAIEncryptedReasoningItems(reqBody) {
		return nil, nil
	}
	return json.Marshal(reqBody)
}

// normalizeGrokUpstreamErrorBody rewrites xAI's string-form error payload into
// the OpenAI-compatible shape clients (Codex Desktop) understand.
//
// xAI:    {"code":"invalid-argument","error":"Could not decode the compaction blob..."}
// OpenAI: {"error":{"type":"invalid_request_error","message":"...","code":"invalid-argument"}}
func normalizeGrokUpstreamErrorBody(body []byte) []byte {
	if len(body) == 0 || !json.Valid(body) {
		return body
	}
	// Already OpenAI-shaped.
	if msg := strings.TrimSpace(gjson.GetBytes(body, "error.message").String()); msg != "" {
		return body
	}
	errField := gjson.GetBytes(body, "error")
	if !errField.Exists() || errField.Type != gjson.String {
		return body
	}
	msg := strings.TrimSpace(errField.String())
	if msg == "" {
		return body
	}
	code := strings.TrimSpace(gjson.GetBytes(body, "code").String())
	errType := "invalid_request_error"
	if code == "" {
		code = "upstream_error"
		errType = "upstream_error"
	}
	payload := map[string]any{
		"error": map[string]any{
			"type":    errType,
			"message": msg,
			"code":    code,
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

// writeGrokResponsesStreamError emits a single Responses SSE error event so
// Codex Desktop can render the full message instead of a bare "{".
func writeGrokResponsesStreamError(c *gin.Context, statusCode int, message string) {
	if c == nil || c.Writer == nil {
		return
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = http.StatusText(statusCode)
	}
	if message == "" {
		message = "Upstream request failed"
	}
	errType := "invalid_request_error"
	if statusCode >= 500 {
		errType = "upstream_error"
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(statusCode)
	payload, err := json.Marshal(map[string]any{
		"type":            "error",
		"sequence_number": 0,
		"error": map[string]any{
			"type":    errType,
			"message": message,
			"code":    errType,
		},
	})
	if err != nil {
		payload = []byte(`{"type":"error","error":{"type":"upstream_error","message":"Upstream request failed"}}`)
	}
	_, _ = c.Writer.Write([]byte("data: " + string(payload) + "\n\n"))
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func patchGrokResponsesBody(body []byte, upstreamModel string) ([]byte, error) {
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid json request body")
	}
	// Compaction-safe path: avoid full-document remashal / tool rewrite that can
	// damage opaque blobs, but still drop Codex private input carriers that xAI
	// cannot deserialize (ModelInput).
	if hasGrokCompactionContext(body) {
		return patchGrokResponsesBodyPreserveCompaction(body, upstreamModel)
	}
	out, err := sjson.SetBytes(body, "model", upstreamModel)
	if err != nil {
		return nil, err
	}
	out, err = sanitizeGrokResponsesModelCapabilities(out, upstreamModel)
	if err != nil {
		return nil, err
	}
	for _, unsupportedField := range []string{"prompt_cache_retention", "safety_identifier"} {
		if gjson.GetBytes(out, unsupportedField).Exists() {
			out, err = sjson.DeleteBytes(out, unsupportedField)
			if err != nil {
				return nil, err
			}
		}
	}
	if strings.EqualFold(upstreamModel, "grok-4.5") {
		for _, unsupportedField := range []string{"presence_penalty", "presencePenalty", "frequency_penalty", "frequencyPenalty", "stop"} {
			if gjson.GetBytes(out, unsupportedField).Exists() {
				out, err = sjson.DeleteBytes(out, unsupportedField)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	out, err = sanitizeGrokResponsesUnsupportedFields(out)
	if err != nil {
		return nil, err
	}
	out, err = sanitizeGrokResponsesInput(out)
	if err != nil {
		return nil, err
	}
	// Upstream ff639ba7 / PR #4242: Codex multi-turn can leave content:null on
	// reasoning items; xAI rejects with 422 ModelInput deserialize failure.
	out, err = sanitizeGrokReasoningNullContent(out)
	if err != nil {
		return nil, err
	}
	out, err = sanitizeGrokResponsesTools(out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// patchGrokResponsesBodyPreserveCompaction keeps opaque items mostly intact.
// Still strips Codex-only additional_tools and reasoning content:null — those
// are not part of the compact blob and cause independent xAI ModelInput 422s.
func patchGrokResponsesBodyPreserveCompaction(body []byte, upstreamModel string) ([]byte, error) {
	out := body
	var err error
	if strings.TrimSpace(gjson.GetBytes(out, "model").String()) != strings.TrimSpace(upstreamModel) {
		out, err = sjson.SetBytes(out, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
	}
	for _, unsupportedField := range []string{
		"prompt_cache_retention",
		"safety_identifier",
		"previous_response_id",
	} {
		if gjson.GetBytes(out, unsupportedField).Exists() {
			out, err = sjson.DeleteBytes(out, unsupportedField)
			if err != nil {
				return nil, err
			}
		}
	}
	out, err = sanitizeGrokResponsesInput(out)
	if err != nil {
		return nil, err
	}
	out, err = sanitizeGrokReasoningNullContent(out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func sanitizeGrokResponsesModelCapabilities(body []byte, upstreamModel string) ([]byte, error) {
	// Composer rejects reasoning controls entirely.
	if grokModelRejectsReasoningEffort(upstreamModel) {
		out := body
		for _, field := range []string{"reasoning", "reasoning_effort", "reasoningEffort"} {
			if !gjson.GetBytes(out, field).Exists() {
				continue
			}
			var err error
			out, err = sjson.DeleteBytes(out, field)
			if err != nil {
				return nil, fmt.Errorf("remove unsupported Grok Composer %s: %w", field, err)
			}
		}
		return out, nil
	}

	// Main Grok models accept low/medium/high only. Codex catalogs may still
	// offer xhigh (OpenAI-style); clamp to high so xAI does not 400.
	return clampGrokReasoningEffortFields(body)
}

func clampGrokReasoningEffortFields(body []byte) ([]byte, error) {
	out := body
	paths := []string{"reasoning.effort", "reasoning_effort", "reasoningEffort"}
	for _, path := range paths {
		raw := strings.TrimSpace(gjson.GetBytes(out, path).String())
		if raw == "" {
			continue
		}
		clamped := clampGrokReasoningEffortValue(raw)
		if clamped == "" || clamped == raw {
			continue
		}
		var err error
		out, err = sjson.SetBytes(out, path, clamped)
		if err != nil {
			return nil, fmt.Errorf("clamp Grok %s: %w", path, err)
		}
	}
	return out, nil
}

// clampGrokReasoningEffortValue maps Codex/OpenAI-style effort labels onto the
// set xAI Grok Responses accepts: low | medium | high.
func clampGrokReasoningEffortValue(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
	switch value {
	case "low", "minimal", "min":
		return "low"
	case "medium", "med", "default":
		return "medium"
	case "high":
		return "high"
	case "xhigh", "extrahigh", "extra", "max", "ultra", "ultracode":
		return "high"
	default:
		// Unknown labels: prefer high over passthrough to avoid upstream 400s.
		return "high"
	}
}

func grokModelRejectsReasoningEffort(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = strings.TrimSpace(model[slash+1:])
	}
	switch model {
	case "grok-composer", "grok-composer-2.5-fast", "composer-2.5":
		return true
	default:
		return false
	}
}

var grokResponsesUnsupportedRecursiveFields = map[string]struct{}{
	"external_web_access": {},
}

func sanitizeGrokResponsesUnsupportedFields(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte(`"external_web_access"`)) {
		return body, nil
	}

	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !deleteJSONFields(payload, grokResponsesUnsupportedRecursiveFields) {
		return body, nil
	}
	return json.Marshal(payload)
}

func deleteJSONFields(value any, fields map[string]struct{}) bool {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		for field := range fields {
			if _, ok := typed[field]; ok {
				delete(typed, field)
				changed = true
			}
		}
		for _, child := range typed {
			if deleteJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	case []any:
		changed := false
		for _, child := range typed {
			if deleteJSONFields(child, fields) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

// additional_tools is a Codex/Responses Lite private input carrier. xAI's
// Responses schema accepts ordinary message/function-call input items but
// rejects this carrier before inference with a ModelInput deserialization
// error. Top-level supported tools remain available through the separate
// sanitizeGrokResponsesTools path.
//
// Upstream: PR #3982 fix(grok): drop Codex additional_tools from Responses input.
func sanitizeGrokResponsesInput(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte(`"additional_tools"`)) {
		return body, nil
	}
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body, nil
	}

	rawItems := input.Array()
	filtered := make([]json.RawMessage, 0, len(rawItems))
	for _, item := range rawItems {
		if strings.TrimSpace(item.Get("type").String()) == "additional_tools" {
			continue
		}
		filtered = append(filtered, json.RawMessage(item.Raw))
	}
	if len(filtered) == len(rawItems) {
		return body, nil
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	return sjson.SetRawBytes(body, "input", encoded)
}

// sanitizeGrokReasoningNullContent removes explicit null fields from reasoning
// history items. Codex multi-turn often emits content:null; xAI rejects it with
// 422 ModelInput deserialize failure, while the same item is accepted when the
// null field is omitted.
//
// Upstream: ff639ba7 / PR #4242 fix(grok): sanitize null reasoning content.
// Also drop encrypted_content:null (Desktop can echo that after stream assembly).
func sanitizeGrokReasoningNullContent(body []byte) ([]byte, error) {
	input := gjson.GetBytes(body, "input")
	if !input.Exists() || !input.IsArray() {
		return body, nil
	}

	items := input.Array()
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if strings.TrimSpace(item.Get("type").String()) != "reasoning" {
			continue
		}
		for _, field := range []string{"content", "encrypted_content"} {
			fieldResult := item.Get(field)
			if fieldResult.Exists() && fieldResult.Type == gjson.Null {
				var err error
				body, err = sjson.DeleteBytes(body, fmt.Sprintf("input.%d.%s", i, field))
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return body, nil
}

var grokResponsesSupportedToolTypes = map[string]struct{}{
	"code_execution":     {},
	"code_interpreter":   {},
	"collections_search": {},
	"file_search":        {},
	"function":           {},
	"mcp":                {},
	"shell":              {},
	"web_search":         {},
	"x_search":           {},
}

func sanitizeGrokResponsesTools(body []byte) ([]byte, error) {
	tools := gjson.GetBytes(body, "tools")
	filteredTools := make([]json.RawMessage, 0)
	var err error

	switch {
	case !tools.Exists():
		// no tools field
	case !tools.IsArray():
		// null/object/string tools are invalid for xAI; drop them
		body, err = sjson.DeleteBytes(body, "tools")
		if err != nil {
			return nil, err
		}
	default:
		rawTools := tools.Array()
		filteredTools = make([]json.RawMessage, 0, len(rawTools))
		for _, tool := range rawTools {
			toolType := strings.TrimSpace(tool.Get("type").String())
			if _, ok := grokResponsesSupportedToolTypes[toolType]; ok {
				filteredTools = append(filteredTools, json.RawMessage(tool.Raw))
			}
		}
		if len(filteredTools) != len(rawTools) {
			if len(filteredTools) == 0 {
				body, err = sjson.DeleteBytes(body, "tools")
			} else {
				var encoded []byte
				encoded, err = json.Marshal(filteredTools)
				if err != nil {
					return nil, err
				}
				body, err = sjson.SetRawBytes(body, "tools", encoded)
			}
			if err != nil {
				return nil, err
			}
		} else if len(filteredTools) == 0 {
			// tools: [] is treated as "no tools" by xAI; drop empty array + tool_choice
			body, err = sjson.DeleteBytes(body, "tools")
			if err != nil {
				return nil, err
			}
		}
	}

	// xAI 400: "A tool_choice was set on the request but no tools were specified."
	// Always reconcile tool_choice against remaining tools (including missing tools field).
	// Final absolute guard: if no tools remain in the body, tool_choice must not exist.
	if !grokResponsesBodyHasTools(body) {
		if gjson.GetBytes(body, "tool_choice").Exists() {
			body, err = sjson.DeleteBytes(body, "tool_choice")
			if err != nil {
				return nil, err
			}
		}
		return body, nil
	}
	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return body, nil
	}
	if shouldDropGrokToolChoice(toolChoice, filteredTools) {
		body, err = sjson.DeleteBytes(body, "tool_choice")
		if err != nil {
			return nil, err
		}
	}
	return body, nil
}

func grokResponsesBodyHasTools(body []byte) bool {
	tools := gjson.GetBytes(body, "tools")
	return tools.Exists() && tools.IsArray() && len(tools.Array()) > 0
}

func shouldDropGrokToolChoice(toolChoice gjson.Result, tools []json.RawMessage) bool {
	if len(tools) == 0 {
		return true
	}
	if !toolChoice.IsObject() {
		return false
	}
	choiceType := strings.TrimSpace(toolChoice.Get("type").String())
	if choiceType == "" {
		return false
	}
	if _, ok := grokResponsesSupportedToolTypes[choiceType]; !ok {
		return true
	}
	if choiceType == "function" {
		choiceName := strings.TrimSpace(toolChoice.Get("name").String())
		if choiceName == "" {
			choiceName = strings.TrimSpace(toolChoice.Get("function.name").String())
		}
		if choiceName == "" {
			return false
		}
		for _, tool := range tools {
			var item struct {
				Type     string `json:"type"`
				Name     string `json:"name"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			if err := json.Unmarshal(tool, &item); err != nil {
				continue
			}
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = strings.TrimSpace(item.Function.Name)
			}
			if strings.TrimSpace(item.Type) == "function" && name == choiceName {
				return false
			}
		}
		return true
	}
	return false
}

func (s *OpenAIGatewayService) bridgeGrokComposerImageInputs(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) ([]byte, OpenAIUsage, bool, error) {
	if !shouldBridgeGrokComposerImageInputs(body) {
		return body, OpenAIUsage{}, false, nil
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, OpenAIUsage{}, false, fmt.Errorf("parse grok composer image bridge request: %w", err)
	}

	imageURLs := collectGrokComposerImageURLs(reqBody)
	if len(imageURLs) == 0 {
		return body, OpenAIUsage{}, false, nil
	}

	descriptions := make([]string, 0, len(imageURLs))
	var bridgeUsage OpenAIUsage
	for index, imageURL := range imageURLs {
		description, usage, err := s.describeGrokComposerImage(ctx, c, account, token, imageURL, index+1)
		if err != nil {
			return body, bridgeUsage, false, err
		}
		descriptions = append(descriptions, description)
		addGrokOpenAIUsage(&bridgeUsage, usage)
	}

	if !rewriteGrokComposerImagesAsText(reqBody, descriptions) {
		return body, bridgeUsage, false, nil
	}
	bridgedBody, err := marshalOpenAIUpstreamJSON(reqBody)
	if err != nil {
		return body, bridgeUsage, false, fmt.Errorf("serialize grok composer image bridge request: %w", err)
	}
	return bridgedBody, bridgeUsage, true, nil
}

func shouldBridgeGrokComposerImageInputs(body []byte) bool {
	if len(body) == 0 || !isGrokComposerModel(gjson.GetBytes(body, "model").String()) {
		return false
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() {
		return false
	}
	return openAIJSONValueMayContainImageInput(messages)
}

func isGrokComposerModel(model string) bool {
	model = strings.TrimSpace(strings.ToLower(model))
	if model == "" {
		return false
	}
	if strings.Contains(model, "/") {
		parts := strings.Split(model, "/")
		model = strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.Contains(model, "composer")
}

func collectGrokComposerImageURLs(reqBody map[string]any) []string {
	messages, ok := reqBody["messages"].([]any)
	if !ok {
		return nil
	}

	var imageURLs []string
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}
		for _, part := range parts {
			if imageURL := grokComposerImageURLFromPart(part); imageURL != "" {
				imageURLs = append(imageURLs, imageURL)
			}
		}
	}
	return imageURLs
}

func grokComposerImageURLFromPart(part any) string {
	partMap, ok := part.(map[string]any)
	if !ok {
		return ""
	}
	if strings.TrimSpace(strings.ToLower(fmt.Sprint(partMap["type"]))) != "image_url" {
		return ""
	}
	switch imageURL := partMap["image_url"].(type) {
	case string:
		return normalizeGrokComposerImageURL(imageURL)
	case map[string]any:
		raw, _ := imageURL["url"].(string)
		return normalizeGrokComposerImageURL(raw)
	default:
		return ""
	}
}

func normalizeGrokComposerImageURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || isEmptyBase64DataURI(trimmed) {
		return ""
	}
	return trimmed
}

func (s *OpenAIGatewayService) describeGrokComposerImage(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	token string,
	imageURL string,
	index int,
) (string, OpenAIUsage, error) {
	body, err := buildGrokComposerImageDescriptionBody(imageURL, index)
	if err != nil {
		return "", OpenAIUsage{}, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	upstreamReq, err := buildGrokResponsesRequest(upstreamCtx, c, account, body, token, "")
	releaseUpstreamCtx()
	if err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("build grok composer image bridge request: %w", err)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return "", OpenAIUsage{}, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI image bridge upstream returned status %d", resp.StatusCode)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
			Kind:               "failover",
			Message:            upstreamMsg,
		})
		s.handleGrokAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			return "", OpenAIUsage{}, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return "", OpenAIUsage{}, fmt.Errorf("grok composer image bridge upstream error: %s", upstreamMsg)
	}

	s.updateGrokUsageSnapshot(ctx, account, xai.ParseQuotaHeaders(resp.Header, resp.StatusCode))
	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, nil)
	if err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("read grok composer image bridge response: %w", err)
	}

	var parsed apicompat.ResponsesResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", OpenAIUsage{}, fmt.Errorf("parse grok composer image bridge response: %w", err)
	}
	description := strings.TrimSpace(grokResponsesOutputText(&parsed))
	if description == "" {
		return "", copyOpenAIUsageFromResponsesUsage(parsed.Usage), fmt.Errorf("grok composer image bridge returned empty description")
	}
	return description, copyOpenAIUsageFromResponsesUsage(parsed.Usage), nil
}

func buildGrokComposerImageDescriptionBody(imageURL string, index int) ([]byte, error) {
	prompt := fmt.Sprintf("Describe image %d in concise, factual text for a downstream coding/composer model. Include visible text, UI elements, diagrams, errors, and spatial relationships. Do not mention that you are an image analysis bridge.", index)
	req := map[string]any{
		"model":             grokComposerImageBridgeVisionModel,
		"stream":            false,
		"store":             false,
		"max_output_tokens": grokComposerImageBridgeMaxOutputTokens,
		"input": []any{
			map[string]any{
				"type": "message",
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": prompt},
					map[string]any{"type": "input_image", "image_url": imageURL},
				},
			},
		},
	}
	return marshalOpenAIUpstreamJSON(req)
}

func grokResponsesOutputText(resp *apicompat.ResponsesResponse) string {
	if resp == nil {
		return ""
	}
	var parts []string
	for _, output := range resp.Output {
		for _, content := range output.Content {
			if content.Type == "output_text" || content.Type == "text" || content.Type == "input_text" {
				if text := strings.TrimSpace(content.Text); text != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

func rewriteGrokComposerImagesAsText(reqBody map[string]any, descriptions []string) bool {
	messages, ok := reqBody["messages"].([]any)
	if !ok {
		return false
	}

	imageIndex := 0
	changed := false
	for _, msg := range messages {
		msgMap, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := msgMap["content"].([]any)
		if !ok {
			continue
		}
		var textParts []string
		messageChanged := false
		for _, part := range parts {
			if imageURL := grokComposerImageURLFromPart(part); imageURL != "" {
				if imageIndex < len(descriptions) {
					textParts = append(textParts, fmt.Sprintf("Image %d description: %s", imageIndex+1, strings.TrimSpace(descriptions[imageIndex])))
				}
				imageIndex++
				messageChanged = true
				continue
			}
			if text := grokComposerTextFromPart(part); text != "" {
				textParts = append(textParts, text)
			}
		}
		if messageChanged {
			msgMap["content"] = strings.Join(textParts, "\n\n")
			changed = true
		}
	}
	return changed
}

func grokComposerTextFromPart(part any) string {
	partMap, ok := part.(map[string]any)
	if !ok {
		return ""
	}
	partType := strings.TrimSpace(strings.ToLower(fmt.Sprint(partMap["type"])))
	switch partType {
	case "text", "input_text":
		text, _ := partMap["text"].(string)
		return strings.TrimSpace(text)
	default:
		return ""
	}
}

func openAIJSONValueMayContainImageInput(value gjson.Result) bool {
	if !value.Exists() {
		return false
	}
	if value.IsArray() {
		found := false
		value.ForEach(func(_, item gjson.Result) bool {
			if openAIJSONValueMayContainImageInput(item) {
				found = true
				return false
			}
			return true
		})
		return found
	}
	if value.IsObject() {
		if strings.TrimSpace(value.Get("type").String()) == "input_image" || value.Get("image_url").Exists() {
			return true
		}
		return openAIJSONValueMayContainImageInput(value.Get("content"))
	}
	return false
}

func addGrokOpenAIUsage(dst *OpenAIUsage, usage OpenAIUsage) {
	if dst == nil {
		return
	}
	dst.InputTokens += usage.InputTokens
	dst.ImageInputTokens += usage.ImageInputTokens
	dst.OutputTokens += usage.OutputTokens
	dst.CacheCreationInputTokens += usage.CacheCreationInputTokens
	dst.CacheReadInputTokens += usage.CacheReadInputTokens
	dst.ImageOutputTokens += usage.ImageOutputTokens
}

func buildGrokResponsesRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token, cacheIdentity string) (*http.Request, error) {
	targetURL, err := xai.BuildResponsesURL(account.GetGrokBaseURL())
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	applyGrokCLIHeaders(req.Header)
	applyGrokCacheHeaders(req.Header, cacheIdentity)
	if c != nil {
		if v := c.GetHeader("OpenAI-Beta"); strings.TrimSpace(v) != "" {
			req.Header.Set("OpenAI-Beta", v)
		}
	}
	return req, nil
}

// applyGrokCLIHeaders identifies subscription traffic as a supported Grok CLI
// version. The CLI gateway rejects otherwise valid OAuth requests without it.
func applyGrokCLIHeaders(headers http.Header) {
	if headers == nil {
		return
	}
	headers.Set("User-Agent", grokUpstreamUserAgent)
	headers.Set("X-Grok-Client-Version", grokCLIVersion)
}

func (s *OpenAIGatewayService) updateGrokUsageSnapshot(ctx context.Context, account *Account, snapshot *xai.QuotaSnapshot) {
	if s == nil || account == nil || account.ID <= 0 || snapshot == nil {
		return
	}
	accountID := account.ID
	now := time.Now()
	resetAt, hasActiveLimit := grokRateLimitResetAt(snapshot, now)
	if hasActiveLimit {
		normalizeGrokExhaustedWindowResets(snapshot, resetAt, now)
	}
	critical := snapshot.StatusCode == http.StatusTooManyRequests || hasActiveLimit
	if s.codexSnapshotThrottle != nil {
		allowed := s.codexSnapshotThrottle.Allow(accountID, now)
		if !critical && !allowed {
			return
		}
	}

	stateCtx := ctx
	if hasActiveLimit {
		var cancel context.CancelFunc
		stateCtx, cancel = grokAccountStateContext(ctx)
		defer cancel()
	}
	if s.accountRepo != nil {
		_ = s.accountRepo.UpdateExtra(stateCtx, accountID, map[string]any{
			grokQuotaSnapshotExtraKey: snapshot,
		})
	}
	if hasActiveLimit {
		s.rateLimitGrok(stateCtx, account, resetAt)
	}
}

func parseGrokQuotaSnapshot(headers http.Header, statusCode int, now time.Time) *xai.QuotaSnapshot {
	snapshot := xai.ParseQuotaHeaders(headers, statusCode)
	if snapshot == nil && statusCode == http.StatusTooManyRequests {
		return &xai.QuotaSnapshot{
			StatusCode: statusCode,
			UpdatedAt:  now.UTC().Format(time.RFC3339),
		}
	}
	return snapshot
}

func normalizeGrokExhaustedWindowResets(snapshot *xai.QuotaSnapshot, resetAt, now time.Time) {
	if snapshot == nil || !resetAt.After(now) {
		return
	}
	for _, window := range []*xai.QuotaWindow{snapshot.Requests, snapshot.Tokens} {
		if window == nil || window.Remaining == nil || *window.Remaining > 0 {
			continue
		}
		candidate := time.Time{}
		if window.ResetUnix != nil && *window.ResetUnix > 0 {
			candidate = time.Unix(*window.ResetUnix, 0)
		} else if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(window.ResetAt)); err == nil {
			candidate = parsed
		}
		if !candidate.After(now) {
			candidate = resetAt
		}
		resetUnix := candidate.Unix()
		window.ResetUnix = &resetUnix
		window.ResetAt = candidate.UTC().Format(time.RFC3339)
	}
}

func grokRateLimitResetAt(snapshot *xai.QuotaSnapshot, now time.Time) (time.Time, bool) {
	if snapshot == nil {
		return time.Time{}, false
	}

	retryAfterExpired := false
	var resetAt time.Time
	if snapshot.RetryAfterSeconds != nil && *snapshot.RetryAfterSeconds > 0 {
		observedAt := now
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(snapshot.UpdatedAt)); err == nil {
			observedAt = parsed
		}
		retryAfterResetAt := observedAt.Add(time.Duration(*snapshot.RetryAfterSeconds) * time.Second)
		if retryAfterResetAt.After(now) {
			resetAt = retryAfterResetAt
		} else {
			retryAfterExpired = true
		}
	}

	exhausted := false
	for _, window := range []*xai.QuotaWindow{snapshot.Requests, snapshot.Tokens} {
		if window == nil || window.Remaining == nil || *window.Remaining > 0 {
			continue
		}
		exhausted = true
		candidate := time.Time{}
		if window.ResetUnix != nil && *window.ResetUnix > 0 {
			candidate = time.Unix(*window.ResetUnix, 0)
		} else if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(window.ResetAt)); err == nil {
			candidate = parsed
		}
		if candidate.After(now) && candidate.After(resetAt) {
			resetAt = candidate
		}
	}
	if !resetAt.IsZero() {
		return resetAt, true
	}
	if retryAfterExpired {
		return time.Time{}, false
	}
	if exhausted || snapshot.StatusCode == http.StatusTooManyRequests {
		return now.Add(grokRateLimitFallbackCooldown), true
	}
	return time.Time{}, false
}

func normalizeGrokRateLimitResetAt(account *Account, resetAt, now time.Time) time.Time {
	if !resetAt.After(now) {
		resetAt = now.Add(grokRateLimitFallbackCooldown)
	}
	if account != nil && account.RateLimitResetAt != nil && account.RateLimitResetAt.After(resetAt) {
		resetAt = *account.RateLimitResetAt
	}
	return resetAt
}

func grokAccountStateContext(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithTimeout(base, openAITransportErrorStateUpdateTimeout)
}

type grokRateLimitExtendingRepository interface {
	SetRateLimitedIfLater(ctx context.Context, id int64, resetAt time.Time) error
}

func persistGrokRateLimit(ctx context.Context, repo AccountRepository, account *Account, resetAt time.Time) {
	if repo == nil || account == nil || account.ID <= 0 {
		return
	}
	resetAt = normalizeGrokRateLimitResetAt(account, resetAt, time.Now())
	stateCtx, cancel := grokAccountStateContext(ctx)
	defer cancel()
	var err error
	if extendingRepo, ok := repo.(grokRateLimitExtendingRepository); ok {
		err = extendingRepo.SetRateLimitedIfLater(stateCtx, account.ID, resetAt)
	} else {
		err = repo.SetRateLimited(stateCtx, account.ID, resetAt)
	}
	if err != nil {
		slog.Warn("persist_grok_rate_limit_failed", "account_id", account.ID, "reset_at", resetAt.UTC(), "error", err)
	}
}

func (s *OpenAIGatewayService) rateLimitGrok(ctx context.Context, account *Account, resetAt time.Time) {
	if s == nil || account == nil {
		return
	}
	resetAt = normalizeGrokRateLimitResetAt(account, resetAt, time.Now())
	runtimeUntil := resetAt
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(runtimeUntil) {
		runtimeUntil = *account.TempUnschedulableUntil
	}
	s.BlockAccountScheduling(account, runtimeUntil)
	persistGrokRateLimit(ctx, s.accountRepo, account, resetAt)
}

func (s *OpenAIGatewayService) handleGrokAccountUpstreamError(ctx context.Context, account *Account, statusCode int, headers http.Header, responseBody []byte) {
	if s == nil || account == nil {
		return
	}
	now := time.Now()
	s.updateGrokUsageSnapshot(ctx, account, parseGrokQuotaSnapshot(headers, statusCode, now))
	switch statusCode {
	case http.StatusUnauthorized:
		s.tempUnscheduleGrok(ctx, account, 10*time.Minute, "grok credentials unauthorized")
	case http.StatusForbidden:
		s.tempUnscheduleGrok(ctx, account, 30*time.Minute, "grok access or entitlement denied")
	case http.StatusTooManyRequests:
		// updateGrokUsageSnapshot installs both runtime and durable rate-limit state.
	default:
		if statusCode >= 500 {
			s.tempUnscheduleGrok(ctx, account, 2*time.Minute, "grok upstream temporary error")
		}
	}
	_ = responseBody
}

func (s *OpenAIGatewayService) tempUnscheduleGrok(ctx context.Context, account *Account, cooldown time.Duration, reason string) {
	if s == nil || account == nil {
		return
	}
	until := time.Now().Add(cooldown)
	if account.TempUnschedulableUntil != nil && account.TempUnschedulableUntil.After(until) {
		until = *account.TempUnschedulableUntil
	}
	account.TempUnschedulableUntil = &until
	account.TempUnschedulableReason = reason
	s.BlockAccountScheduling(account, until)
	if s.accountRepo != nil {
		stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAITransportErrorStateUpdateTimeout)
		defer cancel()
		_ = s.accountRepo.SetTempUnschedulable(stateCtx, account.ID, until, reason)
	}
}

func (s *OpenAIGatewayService) BlockAccountScheduling(account *Account, until time.Time) {
	if s == nil || account == nil || (account.Platform != PlatformOpenAI && account.Platform != PlatformGrok) {
		return
	}
	if !until.After(time.Now()) {
		until = time.Now().Add(2 * time.Minute)
	}
	for {
		current, loaded := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
		if !loaded {
			if _, stored := s.openaiAccountRuntimeBlockUntil.LoadOrStore(account.ID, until); !stored {
				return
			}
			continue
		}
		currentUntil, ok := current.(time.Time)
		if ok && currentUntil.After(until) {
			return
		}
		if s.openaiAccountRuntimeBlockUntil.CompareAndSwap(account.ID, current, until) {
			return
		}
	}
}

func (s *OpenAIGatewayService) isOpenAIAccountRuntimeBlocked(account *Account) bool {
	if s == nil || account == nil {
		return false
	}
	value, ok := s.openaiAccountRuntimeBlockUntil.Load(account.ID)
	if !ok {
		return false
	}
	until, ok := value.(time.Time)
	if !ok || !time.Now().Before(until) {
		s.openaiAccountRuntimeBlockUntil.Delete(account.ID)
		return false
	}
	return true
}
