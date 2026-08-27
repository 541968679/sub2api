package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func isNonChatProbeModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "audio") || strings.Contains(lower, "realtime")
}

func mappingProbeValues(account *Account) []string {
	if account == nil {
		return nil
	}
	mapping := account.GetModelMapping()
	values := make([]string, 0, len(mapping))
	for _, upstream := range mapping {
		upstream = strings.TrimSpace(upstream)
		if upstream == "" || strings.Contains(upstream, "*") {
			continue
		}
		values = append(values, upstream)
	}
	return values
}

// firstSortedMappingProbeModel is the old selector: mapping values, skip empty
// and "*", sort.Strings, take [0], else DefaultTestModel. Used only for
// reprobe eligibility so the new chat-aware selector cannot hide audio-first accounts.
func firstSortedMappingProbeModel(account *Account) string {
	candidates := mappingProbeValues(account)
	if len(candidates) == 0 {
		return openai.DefaultTestModel
	}
	sort.Strings(candidates)
	return candidates[0]
}

func selectResponsesProbeModel(account *Account) string {
	candidates := mappingProbeValues(account)
	chat := make([]string, 0, len(candidates))
	for _, model := range candidates {
		if isNonChatProbeModel(model) {
			continue
		}
		chat = append(chat, model)
	}
	for _, model := range chat {
		if model == openai.DefaultTestModel {
			return openai.DefaultTestModel
		}
	}
	if len(chat) == 0 {
		return openai.DefaultTestModel
	}
	sort.Strings(chat)
	return chat[0]
}

func extraBoolFalse(extra map[string]any, key string) bool {
	if extra == nil {
		return false
	}
	v, ok := extra[key]
	if !ok {
		return false
	}
	flag, ok := v.(bool)
	return ok && !flag
}

func extraBoolPtr(extra map[string]any, key string) *bool {
	if extra == nil {
		return nil
	}
	v, ok := extra[key]
	if !ok {
		return nil
	}
	flag, ok := v.(bool)
	if !ok {
		return nil
	}
	return &flag
}

// NeedsOpenAICapabilityReprobe is true for a live OpenAI API Key whose old
// sort-first probe model is audio/realtime and whose Responses or Chat
// Completions support flag is an explicit bool false. Missing or non-bool
// extra keys are unknown and do not reprobe. Soft-deleted rows never reach
// this predicate: ListAllWithFilters already excludes them, and Account has
// no DeletedAt field.
func NeedsOpenAICapabilityReprobe(account *Account) bool {
	if account == nil {
		return false
	}
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	if !isNonChatProbeModel(firstSortedMappingProbeModel(account)) {
		return false
	}
	return extraBoolFalse(account.Extra, openai_compat.ExtraKeyResponsesSupported) ||
		extraBoolFalse(account.Extra, openai_compat.ExtraKeyChatCompletionsSupported)
}

// OpenAICapabilityReprobeItem is one listed account. It never includes credentials.
type OpenAICapabilityReprobeItem struct {
	AccountID                    int64  `json:"account_id"`
	Name                         string `json:"name"`
	OldProbeModel                string `json:"old_probe_model"`
	NewProbeModel                string `json:"new_probe_model"`
	ResponsesMode                string `json:"openai_responses_mode,omitempty"`
	ResponsesSupported           *bool  `json:"openai_responses_supported"`
	ChatCompletionsSupported     *bool  `json:"openai_chat_completions_supported"`
	NeedsOpenAICapabilityReprobe bool   `json:"needs_openai_capability_reprobe"`
}

// OpenAICapabilityReprobeResult is the dry-run / execute listing.
type OpenAICapabilityReprobeResult struct {
	DryRun     bool                          `json:"dry_run"`
	AllAPIKeys bool                          `json:"all_apikeys"`
	Count      int                           `json:"count"`
	Accounts   []OpenAICapabilityReprobeItem `json:"accounts"`
}

func openAICapabilityReprobeItem(account *Account) OpenAICapabilityReprobeItem {
	mode := ""
	if account != nil && account.Extra != nil {
		if raw, ok := account.Extra[openai_compat.ExtraKeyResponsesMode].(string); ok {
			mode = raw
		}
	}
	return OpenAICapabilityReprobeItem{
		AccountID:                    account.ID,
		Name:                         account.Name,
		OldProbeModel:                firstSortedMappingProbeModel(account),
		NewProbeModel:                selectResponsesProbeModel(account),
		ResponsesMode:                mode,
		ResponsesSupported:           extraBoolPtr(account.Extra, openai_compat.ExtraKeyResponsesSupported),
		ChatCompletionsSupported:     extraBoolPtr(account.Extra, openai_compat.ExtraKeyChatCompletionsSupported),
		NeedsOpenAICapabilityReprobe: NeedsOpenAICapabilityReprobe(account),
	}
}

func (s *AccountTestService) ListOpenAIAPIKeysNeedingCapabilityReprobe(ctx context.Context) ([]Account, error) {
	return s.ListOpenAIAPIKeysForCapabilityReprobe(ctx, false)
}

func (s *AccountTestService) ListOpenAIAPIKeysForCapabilityReprobe(ctx context.Context, allAPIKeys bool) ([]Account, error) {
	if s == nil || s.accountRepo == nil {
		return nil, errors.New("account repository is not available")
	}
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, PlatformOpenAI, AccountTypeAPIKey, "", "", 0, "")
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0)
	for i := range accounts {
		acc := &accounts[i]
		if acc.Platform != PlatformOpenAI || acc.Type != AccountTypeAPIKey {
			continue
		}
		if !allAPIKeys && !NeedsOpenAICapabilityReprobe(acc) {
			continue
		}
		out = append(out, accounts[i])
	}
	return out, nil
}

func (s *AccountTestService) ReprobeOpenAIAPIKeysNeedingCapabilityReprobe(ctx context.Context, dryRun, allAPIKeys bool) (*OpenAICapabilityReprobeResult, error) {
	accounts, err := s.ListOpenAIAPIKeysForCapabilityReprobe(ctx, allAPIKeys)
	if err != nil {
		return nil, err
	}
	result := &OpenAICapabilityReprobeResult{
		DryRun:     dryRun,
		AllAPIKeys: allAPIKeys,
		Count:      len(accounts),
		Accounts:   make([]OpenAICapabilityReprobeItem, 0, len(accounts)),
	}
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		item := openAICapabilityReprobeItem(&accounts[i])
		ids = append(ids, item.AccountID)
		if dryRun {
			result.Accounts = append(result.Accounts, item)
			continue
		}
		s.ProbeOpenAIAPIKeyResponsesSupport(ctx, accounts[i].ID)
		if reloaded, getErr := s.accountRepo.GetByID(ctx, accounts[i].ID); getErr == nil && reloaded != nil {
			item = openAICapabilityReprobeItem(reloaded)
		}
		result.Accounts = append(result.Accounts, item)
	}
	logger.LegacyPrintf("service.openai_probe",
		"capability_reprobe: dry_run=%v all_apikeys=%v count=%d ids=%v",
		dryRun, allAPIKeys, result.Count, ids,
	)
	return result, nil
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
