package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/tidwall/gjson"
)

// Keep the probe short enough for background account updates, with enough room
// for reasoning models to produce the required function call.
const openaiResponsesProbeTimeout = 15 * time.Second

const responsesProbeMaxBodyBytes = 256 * 1024

// openaiResponsesProbePayload builds a Responses request that requires a tool
// call. Endpoints that only return reasoning text but cannot produce
// function_call output should not be routed through /v1/responses.
func openaiResponsesProbePayload(modelID string) []byte {
	if strings.TrimSpace(modelID) == "" {
		modelID = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": modelID,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "Call the probe_ping function with ok=true to acknowledge readiness. You must use the tool."},
				},
			},
		},
		"tools": []map[string]any{
			{
				"type":        "function",
				"name":        "probe_ping",
				"description": "Capability probe. Call to acknowledge.",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"ok": map[string]any{"type": "boolean"},
					},
					"required": []string{"ok"},
				},
			},
		},
		"tool_choice":       "required",
		"max_output_tokens": 512,
		"stream":            false,
	})
	return body
}

// openaiChatCompletionsProbePayload is a cheap existence probe. Do not require
// tool_calls: many compatible midstreams can chat but cannot emit tools.
func openaiChatCompletionsProbePayload(modelID string) []byte {
	if strings.TrimSpace(modelID) == "" {
		modelID = openai.DefaultTestModel
	}
	body, _ := json.Marshal(map[string]any{
		"model": modelID,
		"messages": []map[string]any{
			{"role": "user", "content": "Reply with the single word pong."},
		},
		"max_tokens": 16,
		"stream":     false,
	})
	return body
}

func selectResponsesProbeModel(account *Account) string {
	mapping := account.GetModelMapping()
	candidates := make([]string, 0, len(mapping))
	for _, upstream := range mapping {
		upstream = strings.TrimSpace(upstream)
		if upstream == "" || strings.Contains(upstream, "*") {
			continue
		}
		candidates = append(candidates, upstream)
	}
	if len(candidates) == 0 {
		return openai.DefaultTestModel
	}
	sort.Strings(candidates)
	return candidates[0]
}

type openaiProbeHTTPResult struct {
	status int
	body   []byte
}

// ProbeOpenAIAPIKeyResponsesSupport probes /v1/responses then /v1/chat/completions
// and persists both flags in one UpdateExtra. Transport failure for an endpoint
// leaves that key unwritten.
func (s *AccountTestService) ProbeOpenAIAPIKeyResponsesSupport(ctx context.Context, accountID int64) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.openai_probe", "probe_load_account_failed: account_id=%d err=%v", accountID, err)
		return
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return
	}

	apiKey := account.GetOpenAIApiKey()
	if apiKey == "" {
		logger.LegacyPrintf("service.openai_probe", "probe_skip_no_apikey: account_id=%d", accountID)
		return
	}
	baseURL := account.GetOpenAIBaseURL()
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	normalizedBaseURL, err := s.validateUpstreamBaseURL(baseURL)
	if err != nil {
		logger.LegacyPrintf("service.openai_probe", "probe_invalid_baseurl: account_id=%d base_url=%q err=%v", accountID, baseURL, err)
		return
	}

	probeModel := selectResponsesProbeModel(account)
	extraUpdate := make(map[string]any, 2)

	respURL := buildOpenAIResponsesURL(normalizedBaseURL)
	if result, probeErr := s.doOpenAIAPIKeyProbe(ctx, account, apiKey, respURL, openaiResponsesProbePayload(probeModel)); probeErr != nil {
		logger.LegacyPrintf("service.openai_probe", "probe_request_failed: account_id=%d url=%s err=%v", accountID, respURL, probeErr)
	} else {
		supported := decideResponsesProbeSupport(result.status, result.body)
		extraUpdate[openai_compat.ExtraKeyResponsesSupported] = supported
		logger.LegacyPrintf("service.openai_probe",
			"probe_responses_done: account_id=%d base_url=%s probe_model=%s status=%d supported=%v",
			accountID, normalizedBaseURL, probeModel, result.status, supported,
		)
	}

	ccURL := buildOpenAIChatCompletionsURL(normalizedBaseURL)
	if result, probeErr := s.doOpenAIAPIKeyProbe(ctx, account, apiKey, ccURL, openaiChatCompletionsProbePayload(probeModel)); probeErr != nil {
		logger.LegacyPrintf("service.openai_probe", "probe_request_failed: account_id=%d url=%s err=%v", accountID, ccURL, probeErr)
	} else {
		supported := decideChatCompletionsProbeSupport(result.status, result.body)
		extraUpdate[openai_compat.ExtraKeyChatCompletionsSupported] = supported
		logger.LegacyPrintf("service.openai_probe",
			"probe_chat_completions_done: account_id=%d base_url=%s probe_model=%s status=%d supported=%v",
			accountID, normalizedBaseURL, probeModel, result.status, supported,
		)
	}

	if len(extraUpdate) == 0 {
		return
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, extraUpdate); err != nil {
		logger.LegacyPrintf("service.openai_probe", "probe_persist_failed: account_id=%d extra=%v err=%v", accountID, extraUpdate, err)
		return
	}
	logger.LegacyPrintf("service.openai_probe", "probe_persist_done: account_id=%d extra=%v", accountID, extraUpdate)
}

func (s *AccountTestService) doOpenAIAPIKeyProbe(
	ctx context.Context,
	account *Account,
	apiKey string,
	probeURL string,
	payload []byte,
) (*openaiProbeHTTPResult, error) {
	probeCtx, cancel := context.WithTimeout(ctx, openaiResponsesProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, probeURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	account.ApplyHeaderOverrides(req.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, s.tlsFPProfileService.ResolveTLSProfile(account))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, responsesProbeMaxBodyBytes))
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responsesProbeMaxBodyBytes))
	if readErr != nil {
		return nil, readErr
	}
	return &openaiProbeHTTPResult{status: resp.StatusCode, body: bodyBytes}, nil
}

func decideResponsesProbeSupport(status int, body []byte) bool {
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return false
	}
	if status < 200 || status >= 300 {
		return true
	}
	return responsesProbeBodyHasFunctionCall(body)
}

func decideChatCompletionsProbeSupport(status int, body []byte) bool {
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return false
	}
	if status < 200 || status >= 300 {
		return true
	}
	return chatCompletionsProbeBodyLooksSupported(body)
}

func responsesProbeBodyHasFunctionCall(body []byte) bool {
	output := gjson.GetBytes(body, "output")
	if !output.IsArray() {
		return false
	}
	for _, item := range output.Array() {
		if strings.TrimSpace(item.Get("type").String()) == "function_call" {
			return true
		}
	}
	return false
}

func chatCompletionsProbeBodyLooksSupported(body []byte) bool {
	choices := gjson.GetBytes(body, "choices")
	if choices.IsArray() {
		return true
	}
	object := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "object").String()))
	return object == "chat.completion" || object == "chat.completion.chunk"
}
