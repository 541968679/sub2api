package service

// Anthropic /v1/messages/count_tokens 的 OpenAI 计数桥。以官方 upstream/main
// (e316ebf5, PR #3497 + #3635 语义) 的 openai_gateway_count_tokens.go 为基线
// 手工移植：请求转换复用 apicompat.AnthropicToResponses，上游计数调用
// POST /v1/responses/input_tokens，OAuth 缺 scope / 端点不支持时回退本地
// tiktoken 估算，且不得因此把健康账号临时下线或标错。
//
// 本 fork 的适配差异：
//   - 保留 fork 的 account.ApplyHeaderOverrides 账号级请求头覆写。
//   - 新增 Claude-GPT bridge 宽松模式：bridge count 属于辅助能力，任何
//     上游失败（含 429/5xx/传输错误）都回退本地估算返回 200，只保留
//     HandleUpstreamError 的账号状态记账；绝不把 bridge 容量问题变成
//     count_tokens 的 4xx/5xx，也绝不落入 native Antigravity 池。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
	"go.uber.org/zap"
)

const (
	openaiPlatformAPIInputTokensURL = "https://api.openai.com/v1/responses/input_tokens"

	openAIResponsesInputItemTokenOverhead = 3
	openAIResponsesContentPartOverhead    = 1
	openAIInputTokensFallbackMinimum      = 1

	// openAIInputTokensEstimateMaxBytes 限制送入本地 tiktoken 的内容规模。
	// 网关 body 上限默认 256MB，直接对超大请求跑 BPE/回溯正则会构成本地
	// 计算型 DoS；超过该规模的请求退化为字节数近似（约 4 字节/词元）。
	openAIInputTokensEstimateMaxBytes = 8 << 20
)

type openAIInputTokensCountRequest struct {
	Model        string                    `json:"model"`
	Instructions string                    `json:"instructions,omitempty"`
	Input        json.RawMessage           `json:"input,omitempty"`
	Tools        []apicompat.ResponsesTool `json:"tools,omitempty"`
	ToolChoice   json.RawMessage           `json:"tool_choice,omitempty"`
}

type openAIInputTokensCountPrepared struct {
	Request         openAIInputTokensCountRequest
	OriginalModel   string
	NormalizedModel string
	BillingModel    string
	UpstreamModel   string
}

// ForwardCountTokensAsAnthropic bridges Anthropic /v1/messages/count_tokens to
// OpenAI POST /v1/responses/input_tokens and returns Anthropic-compatible output.
func (s *OpenAIGatewayService) ForwardCountTokensAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) error {
	return s.forwardCountTokensAsAnthropic(ctx, c, account, body, defaultMappedModel, false)
}

// ForwardCountTokensAsAnthropicClaudeGPTBridge is the Claude-GPT bridge count
// path: identical conversion and upstream call, but every upstream failure
// falls back to a local estimate instead of surfacing an error status.
func (s *OpenAIGatewayService) ForwardCountTokensAsAnthropicClaudeGPTBridge(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
) error {
	return s.forwardCountTokensAsAnthropic(ctx, c, account, body, defaultMappedModel, true)
}

func (s *OpenAIGatewayService) forwardCountTokensAsAnthropic(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	defaultMappedModel string,
	bridgeLenient bool,
) error {
	if account == nil {
		writeAnthropicCountTokensError(c, http.StatusServiceUnavailable, "api_error", "No available OpenAI accounts")
		return fmt.Errorf("count_tokens: missing account")
	}

	prepared, err := prepareOpenAIInputTokensCountRequest(body, account, defaultMappedModel)
	if err != nil {
		writeAnthropicCountTokensError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return err
	}

	upstreamBody, err := marshalOpenAIUpstreamJSON(prepared.Request)
	if err != nil {
		writeAnthropicCountTokensError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return fmt.Errorf("marshal openai input_tokens body: %w", err)
	}

	logger.L().Debug("openai count_tokens: model mapping applied",
		zap.Int64("account_id", account.ID),
		zap.String("original_model", prepared.OriginalModel),
		zap.String("normalized_model", prepared.NormalizedModel),
		zap.String("billing_model", prepared.BillingModel),
		zap.String("upstream_model", prepared.UpstreamModel),
	)

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		if bridgeLenient {
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, 0, "bridge_access_token_failed")
			return nil
		}
		writeAnthropicCountTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to get access token")
		return fmt.Errorf("get access token: %w", err)
	}

	upstreamReq, err := s.buildInputTokensUpstreamRequest(ctx, c, account, upstreamBody, token)
	if err != nil {
		writeAnthropicCountTokensError(c, http.StatusInternalServerError, "api_error", "Failed to build request")
		return fmt.Errorf("build input_tokens request: %w", err)
	}

	proxyURL := ""
	if account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		safeErr := sanitizeUpstreamErrorMessage(err.Error())
		setOpsUpstreamError(c, 0, safeErr, "")
		if bridgeLenient {
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, 0, "bridge_upstream_transport_failed")
			return nil
		}
		writeAnthropicCountTokensError(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
		return fmt.Errorf("openai input_tokens upstream request failed: %s", safeErr)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if bridgeLenient {
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, resp.StatusCode, "bridge_upstream_read_failed")
			return nil
		}
		writeAnthropicCountTokensError(c, http.StatusBadGateway, "upstream_error", "Failed to read response")
		return fmt.Errorf("read input_tokens response: %w", err)
	}

	if resp.StatusCode >= 400 {
		upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if account.Type == AccountTypeOAuth && isOpenAIOAuthInputTokensUnsupported(resp.StatusCode, respBody) {
			// OAuth 缺 scope / 端点不存在不是账号故障：直接本地估算，
			// 不写限流、不临时下线、不标错。
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, resp.StatusCode, "oauth_input_tokens_unsupported")
			return nil
		}

		if s.rateLimitService != nil {
			s.rateLimitService.HandleUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody)
		}

		if bridgeLenient {
			// bridge count 是辅助路径：保留上面的账号状态记账，但对
			// 客户端始终回退本地估算，避免把容量问题放大成 count 5xx。
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, resp.StatusCode, "bridge_upstream_error")
			return nil
		}

		if isOpenAIInputTokensUnsupported(resp.StatusCode, respBody) {
			writeAnthropicCountTokensError(c, http.StatusNotFound, "not_found_error", "Token counting is not supported by upstream")
			return nil
		}

		upstreamDetail := ""
		if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
			maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
			if maxBytes <= 0 {
				maxBytes = 2048
			}
			upstreamDetail = truncateString(string(respBody), maxBytes)
		}
		setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)

		errMsg := "Upstream request failed"
		switch resp.StatusCode {
		case 429:
			errMsg = "Rate limit exceeded"
		case 500, 502, 503, 504, 529:
			errMsg = "Upstream service temporarily unavailable"
		}
		writeAnthropicCountTokensError(c, resp.StatusCode, "upstream_error", errMsg)
		if upstreamMsg == "" {
			return fmt.Errorf("input_tokens upstream error: %d", resp.StatusCode)
		}
		return fmt.Errorf("input_tokens upstream error: %d message=%s", resp.StatusCode, upstreamMsg)
	}

	inputTokens := gjson.GetBytes(respBody, "input_tokens")
	if !inputTokens.Exists() {
		if bridgeLenient {
			writeOpenAIInputTokensLocalEstimate(c, account, prepared, resp.StatusCode, "bridge_upstream_missing_input_tokens")
			return nil
		}
		writeAnthropicCountTokensError(c, http.StatusBadGateway, "upstream_error", "Upstream response missing input_tokens")
		return fmt.Errorf("input_tokens response missing input_tokens field")
	}

	c.JSON(http.StatusOK, gin.H{
		"input_tokens": int(inputTokens.Int()),
	})
	return nil
}

// EstimateCountTokensClaudeGPTBridge serves the bridge count path when no
// bridge account is currently schedulable (rate limited, overloaded, paused,
// or probe failure): it converts the Anthropic request the same way and
// returns a local tiktoken estimate. It never sends upstream requests and
// never touches the native Antigravity pool.
func (s *OpenAIGatewayService) EstimateCountTokensClaudeGPTBridge(
	c *gin.Context,
	body []byte,
	upstreamModel string,
) error {
	prepared, err := prepareOpenAIInputTokensCountRequestForModel(body, upstreamModel)
	if err != nil {
		writeAnthropicCountTokensError(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return err
	}
	writeOpenAIInputTokensLocalEstimate(c, nil, prepared, 0, "bridge_no_schedulable_account")
	return nil
}

// openAIInputTokensEstimateSizeBytes approximates the serialized size of the
// content that would be fed to the tokenizer.
func openAIInputTokensEstimateSizeBytes(req openAIInputTokensCountRequest) int {
	size := len(req.Instructions) + len(req.Input) + len(req.ToolChoice)
	for _, tool := range req.Tools {
		size += len(tool.Name) + len(tool.Description) + len(tool.Parameters)
	}
	return size
}

func prepareOpenAIInputTokensCountRequest(
	body []byte,
	account *Account,
	defaultMappedModel string,
) (*openAIInputTokensCountPrepared, error) {
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic count_tokens request: %w", err)
	}

	originalModel := anthropicReq.Model
	applyOpenAICompatModelNormalization(&anthropicReq)
	normalizedModel := anthropicReq.Model
	billingModel := resolveOpenAIForwardModel(account, normalizedModel, strings.TrimSpace(defaultMappedModel))
	upstreamModel := normalizeOpenAIModelForUpstream(account, billingModel)

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic request to responses: %w", err)
	}

	return &openAIInputTokensCountPrepared{
		Request: openAIInputTokensCountRequest{
			Model:        upstreamModel,
			Instructions: responsesReq.Instructions,
			Input:        responsesReq.Input,
			Tools:        responsesReq.Tools,
			ToolChoice:   responsesReq.ToolChoice,
		},
		OriginalModel:   originalModel,
		NormalizedModel: normalizedModel,
		BillingModel:    billingModel,
		UpstreamModel:   upstreamModel,
	}, nil
}

// prepareOpenAIInputTokensCountRequestForModel converts without an account,
// pinning the upstream model explicitly (bridge local-estimate path).
func prepareOpenAIInputTokensCountRequestForModel(body []byte, upstreamModel string) (*openAIInputTokensCountPrepared, error) {
	var anthropicReq apicompat.AnthropicRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic count_tokens request: %w", err)
	}

	originalModel := anthropicReq.Model
	applyOpenAICompatModelNormalization(&anthropicReq)
	normalizedModel := anthropicReq.Model
	upstreamModel = strings.TrimSpace(upstreamModel)
	if upstreamModel == "" {
		upstreamModel = normalizedModel
	}

	responsesReq, err := apicompat.AnthropicToResponses(&anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("convert anthropic request to responses: %w", err)
	}

	return &openAIInputTokensCountPrepared{
		Request: openAIInputTokensCountRequest{
			Model:        upstreamModel,
			Instructions: responsesReq.Instructions,
			Input:        responsesReq.Input,
			Tools:        responsesReq.Tools,
			ToolChoice:   responsesReq.ToolChoice,
		},
		OriginalModel:   originalModel,
		NormalizedModel: normalizedModel,
		BillingModel:    upstreamModel,
		UpstreamModel:   upstreamModel,
	}, nil
}

func (s *OpenAIGatewayService) buildInputTokensUpstreamRequest(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	token string,
) (*http.Request, error) {
	targetURL := openaiPlatformAPIInputTokensURL
	if account.Type == AccountTypeAPIKey {
		if baseURL := account.GetOpenAIBaseURL(); strings.TrimSpace(baseURL) != "" {
			validatedURL, err := s.validateUpstreamBaseURL(baseURL)
			if err != nil {
				return nil, err
			}
			targetURL = buildOpenAIResponsesInputTokensURL(validatedURL)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("authorization", "Bearer "+token)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")

	if c != nil && c.Request != nil {
		for key, values := range c.Request.Header {
			lower := strings.ToLower(strings.TrimSpace(key))
			if lower != "user-agent" && lower != "accept-language" {
				continue
			}
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}
	}

	// 账号级请求头覆写（仅 openai api_key 账号启用时生效；OAuth 路径 no-op）
	account.ApplyHeaderOverrides(req.Header)

	return req, nil
}

func writeAnthropicCountTokensError(c *gin.Context, status int, errType, message string) {
	c.JSON(status, gin.H{
		"type": "error",
		"error": gin.H{
			"type":    errType,
			"message": message,
		},
	})
}

func isOpenAIInputTokensUnsupported(statusCode int, body []byte) bool {
	if statusCode != http.StatusNotFound {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	return strings.Contains(msg, "input_tokens") && strings.Contains(msg, "not found")
}

// writeOpenAIInputTokensLocalEstimate 输出本地 tiktoken 估算结果。
// estimateReason 仅用于日志观测（count_tokens_estimated），不进入响应体。
func writeOpenAIInputTokensLocalEstimate(c *gin.Context, account *Account, prepared *openAIInputTokensCountPrepared, statusCode int, estimateReason string) {
	estimated := openAIInputTokensFallbackMinimum
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	if got, err := estimateOpenAIInputTokens(prepared.Request); err == nil {
		if got > 0 {
			estimated = got
		}
		logger.L().Info("openai count_tokens: local tiktoken estimate",
			zap.Int64("account_id", accountID),
			zap.Int("upstream_status", statusCode),
			zap.Bool("count_tokens_estimated", true),
			zap.String("estimate_reason", estimateReason),
			zap.Int("estimated_input_tokens", estimated),
			zap.String("upstream_model", prepared.UpstreamModel),
		)
	} else {
		logger.L().Warn("openai count_tokens: local tiktoken estimate failed, using minimum",
			zap.Int64("account_id", accountID),
			zap.Int("upstream_status", statusCode),
			zap.Bool("count_tokens_estimated", true),
			zap.String("estimate_reason", estimateReason),
			zap.Int("estimated_input_tokens", estimated),
			zap.String("upstream_model", prepared.UpstreamModel),
			zap.Error(err),
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"input_tokens": estimated,
	})
}

func isOpenAIOAuthInputTokensUnsupported(statusCode int, body []byte) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
	default:
		return false
	}

	bodyLower := strings.ToLower(string(body))
	msg := strings.ToLower(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body)))

	if code == "missing_scope" ||
		strings.Contains(bodyLower, "api.responses.write") ||
		strings.Contains(bodyLower, "missing scopes") ||
		strings.Contains(bodyLower, "insufficient_scope") {
		return true
	}

	if statusCode == http.StatusNotFound && isOpenAIInputTokensUnsupported(statusCode, body) {
		return true
	}

	return strings.Contains(msg, "input_tokens") &&
		(strings.Contains(msg, "not found") ||
			strings.Contains(msg, "not supported") ||
			strings.Contains(msg, "unsupported"))
}

func estimateOpenAIInputTokens(req openAIInputTokensCountRequest) (int, error) {
	if size := openAIInputTokensEstimateSizeBytes(req); size > openAIInputTokensEstimateMaxBytes {
		approx := size / 4
		if approx < openAIInputTokensFallbackMinimum {
			approx = openAIInputTokensFallbackMinimum
		}
		return approx, nil
	}

	codec, err := openAIInputTokensCodecForModel(req.Model)
	if err != nil {
		return 0, err
	}

	total := 0
	addCount := func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		n, err := codec.Count(text)
		if err != nil {
			return err
		}
		total += n
		return nil
	}

	if err := addCount(req.Instructions); err != nil {
		return 0, err
	}
	inputTokens, err := estimateOpenAIInputTokensForInput(codec, req.Input)
	if err != nil {
		return 0, err
	}
	total += inputTokens

	for _, tool := range req.Tools {
		raw, err := marshalOpenAIUpstreamJSON(tool)
		if err != nil {
			return 0, err
		}
		if err := addCount(string(raw)); err != nil {
			return 0, err
		}
	}
	if len(req.ToolChoice) > 0 {
		compacted, err := compactOpenAIInputTokensJSON(req.ToolChoice)
		if err != nil {
			return 0, err
		}
		if err := addCount(compacted); err != nil {
			return 0, err
		}
	}

	if total < 0 {
		return 0, nil
	}
	return total, nil
}

func estimateOpenAIInputTokensForInput(codec tokenizer.Codec, raw json.RawMessage) (int, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return 0, nil
	}

	var plainText string
	if err := json.Unmarshal(raw, &plainText); err == nil {
		return codec.Count(plainText)
	}

	var items []apicompat.ResponsesInputItem
	if err := json.Unmarshal(raw, &items); err == nil {
		return estimateOpenAIInputTokensForInputItems(codec, items)
	}

	compacted, err := compactOpenAIInputTokensJSON(raw)
	if err != nil {
		return 0, err
	}
	return codec.Count(compacted)
}

func estimateOpenAIInputTokensForInputItems(codec tokenizer.Codec, items []apicompat.ResponsesInputItem) (int, error) {
	total := 0
	countText := func(text string) error {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		n, err := codec.Count(text)
		if err != nil {
			return err
		}
		total += n
		return nil
	}

	for _, item := range items {
		total += openAIResponsesInputItemTokenOverhead
		if err := countText(item.Role); err != nil {
			return 0, err
		}
		if item.Type != "" && item.Type != "message" {
			if err := countText(item.Type); err != nil {
				return 0, err
			}
		}
		if err := countText(item.Name); err != nil {
			return 0, err
		}
		if err := countText(item.Arguments); err != nil {
			return 0, err
		}
		if err := countText(item.Output); err != nil {
			return 0, err
		}
		if err := countText(item.CallID); err != nil {
			return 0, err
		}
		if err := countText(item.ID); err != nil {
			return 0, err
		}

		if len(bytes.TrimSpace(item.Content)) == 0 {
			continue
		}

		var contentText string
		if err := json.Unmarshal(item.Content, &contentText); err == nil {
			if err := countText(contentText); err != nil {
				return 0, err
			}
			continue
		}

		var parts []apicompat.ResponsesContentPart
		if err := json.Unmarshal(item.Content, &parts); err == nil {
			for _, part := range parts {
				total += openAIResponsesContentPartOverhead
				switch part.Type {
				case "input_text", "output_text", "text":
					if err := countText(part.Text); err != nil {
						return 0, err
					}
				case "input_image":
					if err := countText(estimateOpenAIInputImageText(part.ImageURL)); err != nil {
						return 0, err
					}
				default:
					if err := countText(part.Type); err != nil {
						return 0, err
					}
				}
			}
			continue
		}

		compacted, err := compactOpenAIInputTokensJSON(item.Content)
		if err != nil {
			return 0, err
		}
		if err := countText(compacted); err != nil {
			return 0, err
		}
	}

	return total, nil
}

func estimateOpenAIInputImageText(imageURL string) string {
	trimmed := strings.TrimSpace(imageURL)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "data:") {
		if comma := strings.Index(trimmed, ","); comma > 0 {
			return trimmed[:comma]
		}
	}
	return trimmed
}

func compactOpenAIInputTokensJSON(raw json.RawMessage) (string, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func openAIInputTokensCodecForModel(model string) (tokenizer.Codec, error) {
	switch openAIInputTokensEncodingForModel(model) {
	case tokenizer.Cl100kBase:
		return tokenizer.Get(tokenizer.Cl100kBase)
	default:
		return tokenizer.Get(tokenizer.O200kBase)
	}
}

func openAIInputTokensEncodingForModel(model string) tokenizer.Encoding {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(normalized, "gpt-3.5"),
		(strings.HasPrefix(normalized, "gpt-4") &&
			!strings.HasPrefix(normalized, "gpt-4o") &&
			!strings.HasPrefix(normalized, "gpt-4.1")),
		strings.HasPrefix(normalized, "text-embedding-"):
		return tokenizer.Cl100kBase
	default:
		return tokenizer.O200kBase
	}
}
