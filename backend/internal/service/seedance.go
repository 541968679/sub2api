package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	SeedanceEndpointCreate GrokMediaEndpoint = "seedance_create"
	SeedanceEndpointStatus GrokMediaEndpoint = "seedance_status"
	SeedanceEndpointDelete GrokMediaEndpoint = "seedance_delete"
	seedanceTaskTTL                          = 24 * time.Hour
)

func (e GrokMediaEndpoint) IsSeedance() bool {
	return e == SeedanceEndpointCreate || e == SeedanceEndpointStatus || e == SeedanceEndpointDelete
}

// SeedanceTaskKey isolates ownership and billing keys from Grok video tasks.
func SeedanceTaskKey(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "seedance:")
	if id == "" {
		return ""
	}
	return "seedance:" + id
}

func seedanceSessionHash(userID, apiKeyID int64, taskID string) string {
	taskID = strings.TrimPrefix(strings.TrimSpace(taskID), "seedance:")
	return fmt.Sprintf("seedance:%d:%d:%s", userID, apiKeyID, taskID)
}

func ParseSeedanceRequest(body []byte) (GrokMediaRequestInfo, error) {
	var info GrokMediaRequestInfo
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return info, fmt.Errorf("request body must be a JSON object")
	}
	model := gjson.GetBytes(body, "model")
	if model.Type != gjson.String || strings.TrimSpace(model.String()) == "" {
		return info, fmt.Errorf("model is required")
	}
	content := gjson.GetBytes(body, "content")
	if !content.IsArray() || len(content.Array()) == 0 {
		return info, fmt.Errorf("content must be a non-empty array")
	}
	info.Model = strings.TrimSpace(model.String())
	var texts []string
	for _, item := range content.Array() {
		switch item.Get("type").String() {
		case "text":
			texts = append(texts, item.Get("text").String())
		case "image_url":
			info.InputImageURLs = append(info.InputImageURLs, item.Get("image_url.url").String())
		}
	}
	info.Prompt = strings.Join(texts, "\n")
	return info, nil
}

func buildSeedanceURL(base string, endpoint GrokMediaEndpoint, taskID string) (string, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return "", fmt.Errorf("invalid base_url")
	}
	if !strings.HasSuffix(base, "/api/v3") && !strings.HasSuffix(base, "/v3") {
		base += "/api/v3"
	}
	base += "/contents/generations/tasks"
	if endpoint != SeedanceEndpointCreate {
		taskID = strings.TrimPrefix(strings.TrimSpace(taskID), "seedance:")
		if err := validateSeedanceTaskID(taskID); err != nil {
			return "", err
		}
		base += "/" + taskID
	}
	return base, nil
}

func validateSeedanceTaskID(taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || taskID == "." || taskID == ".." || len(taskID) > 200 {
		return fmt.Errorf("invalid Seedance task ID")
	}
	if strings.ContainsAny(taskID, "/\\?#\r\n") {
		return fmt.Errorf("invalid Seedance task ID")
	}
	return nil
}

// ForwardSeedance preserves the Ark body. Only model is rewritten from the account mapping.
// A non-2xx create is returned as-is and must not be retried.
func (s *OpenAIGatewayService) ForwardSeedance(ctx context.Context, c *gin.Context, account *Account, endpoint GrokMediaEndpoint, taskID string, body []byte) (*OpenAIForwardResult, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, fmt.Errorf("seedance upstream client is not configured")
	}
	if account == nil || !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilitySeedance) || !endpoint.IsSeedance() {
		return nil, fmt.Errorf("seedance requires an OpenAI API key account with a custom base URL")
	}
	base, err := s.validateUpstreamBaseURL(account.GetCredential("base_url"))
	if err != nil {
		return nil, err
	}
	target, err := buildSeedanceURL(base, endpoint, taskID)
	if err != nil {
		return nil, err
	}
	model, upstreamModel := "", ""
	method := http.MethodGet
	switch endpoint {
	case SeedanceEndpointCreate:
		info, parseErr := ParseSeedanceRequest(body)
		if parseErr != nil {
			return nil, parseErr
		}
		model = info.Model
		upstreamModel = account.GetMappedModel(model)
		body, err = sjson.SetBytes(body, "model", upstreamModel)
		if err != nil {
			return nil, err
		}
		method = http.MethodPost
	case SeedanceEndpointDelete:
		method = http.MethodDelete
	}
	token := strings.TrimSpace(account.GetCredential("api_key"))
	if token == "" {
		return nil, fmt.Errorf("seedance account missing api_key")
	}
	var bodyReader io.Reader
	if endpoint == SeedanceEndpointCreate {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if endpoint == SeedanceEndpointCreate {
		req.Header.Set("Content-Type", "application/json")
	}
	account.ApplyHeaderOverrides(req.Header)
	proxy := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxy = account.Proxy.URL()
	}
	started := time.Now()
	resp, err := s.httpUpstream.Do(req, proxy, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(started).Milliseconds())
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	SetActualOpenAIUpstreamEndpoint(c, "/api/v3/contents/generations/tasks")
	if resp.StatusCode >= 300 {
		writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
		return nil, fmt.Errorf("seedance upstream status %d", resp.StatusCode)
	}
	result := &OpenAIForwardResult{
		Model:           model,
		BillingModel:    model,
		UpstreamModel:   upstreamModel,
		Duration:        time.Since(started),
		ResponseHeaders: resp.Header.Clone(),
	}
	if endpoint == SeedanceEndpointCreate {
		id := strings.TrimSpace(gjson.GetBytes(responseBody, "id").String())
		if id == "" {
			return nil, fmt.Errorf("seedance create response missing task ID")
		}
		result.ResponseID = SeedanceTaskKey(id)
		result.RequestID = result.ResponseID
	}
	if endpoint == SeedanceEndpointStatus {
		result.ResponseID = SeedanceTaskKey(taskID)
		result.RequestID = result.ResponseID
		result.UpstreamModel = strings.TrimSpace(gjson.GetBytes(responseBody, "model").String())
		if gjson.GetBytes(responseBody, "status").String() == "succeeded" {
			tokens := int(gjson.GetBytes(responseBody, "usage.completion_tokens").Int())
			if tokens < 0 {
				tokens = 0
			}
			result.Usage.OutputTokens = tokens
		}
	}
	writeGrokMediaResponse(c, resp, responseBody, s.responseHeaderFilter)
	return result, nil
}

func (s *OpenAIGatewayService) BindSeedanceTask(ctx context.Context, groupID *int64, userID, apiKeyID int64, taskID, requestedModel string, accountID int64) error {
	if err := validateSeedanceTaskID(strings.TrimPrefix(taskID, "seedance:")); err != nil {
		return err
	}
	return s.setStickySessionBinding(ctx, groupID, seedanceSessionHash(userID, apiKeyID, taskID), StickySessionBinding{
		AccountID: accountID,
		Model:     strings.TrimSpace(requestedModel),
	}, seedanceTaskTTL)
}

func (s *OpenAIGatewayService) LookupSeedanceTask(ctx context.Context, groupID *int64, userID, apiKeyID int64, taskID string) (StickySessionBinding, error) {
	if s == nil {
		return StickySessionBinding{}, fmt.Errorf("seedance task binding unavailable")
	}
	if err := validateSeedanceTaskID(strings.TrimPrefix(taskID, "seedance:")); err != nil {
		return StickySessionBinding{}, err
	}
	binding := s.readSessionBinding(ctx, groupID, seedanceSessionHash(userID, apiKeyID, taskID))
	if binding.AccountID <= 0 {
		return StickySessionBinding{}, fmt.Errorf("seedance task not found")
	}
	return binding, nil
}

func (s *OpenAIGatewayService) LoadSeedanceAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 {
		return nil, fmt.Errorf("seedance account unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("seedance account unavailable")
	}
	if strings.TrimSpace(account.GetCredential("base_url")) == "" || strings.TrimSpace(account.GetCredential("api_key")) == "" {
		return nil, fmt.Errorf("seedance account unavailable")
	}
	return account, nil
}

// RejectUnpricedModel stops a Seedance create before the Ark request when the
// public model has no positive price. It uses the same gate as the other OpenAI paths.
func (s *OpenAIGatewayService) RejectUnpricedModel(ctx context.Context, c *gin.Context, model string) error {
	if s == nil {
		return nil
	}
	return s.rejectUnpricedModel(ctx, c, model)
}
