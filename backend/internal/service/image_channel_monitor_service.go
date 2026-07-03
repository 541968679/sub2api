package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type ImageChannelMonitorService struct {
	repo                ImageChannelMonitorRepository
	accountReader       imageChannelMonitorAccountReader
	proxyReader         imageChannelMonitorProxyReader
	encryptor           SecretEncryptor
	httpUpstream        HTTPUpstream
	tlsFPProfileService *TLSFingerprintProfileService
	scheduler           ImageMonitorScheduler
	runtimeMu           sync.RWMutex
	runtimeStatus       map[int64]ImageChannelMonitorRuntimeStatus
}

func NewImageChannelMonitorService(
	repo ImageChannelMonitorRepository,
	accountReader imageChannelMonitorAccountReader,
	proxyReader imageChannelMonitorProxyReader,
	encryptor SecretEncryptor,
	httpUpstream HTTPUpstream,
	tlsFPProfileService *TLSFingerprintProfileService,
) *ImageChannelMonitorService {
	return &ImageChannelMonitorService{
		repo:                repo,
		accountReader:       accountReader,
		proxyReader:         proxyReader,
		encryptor:           encryptor,
		httpUpstream:        httpUpstream,
		tlsFPProfileService: tlsFPProfileService,
		runtimeStatus:       make(map[int64]ImageChannelMonitorRuntimeStatus),
	}
}

func (s *ImageChannelMonitorService) SetScheduler(scheduler ImageMonitorScheduler) {
	s.scheduler = scheduler
}

func (s *ImageChannelMonitorService) List(
	ctx context.Context,
	params ImageChannelMonitorListParams,
) ([]*ImageChannelMonitor, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 200 {
		params.PageSize = 20
	}
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list image channel monitors: %w", err)
	}
	for _, it := range items {
		s.decryptInPlace(it)
	}
	return items, total, nil
}

func (s *ImageChannelMonitorService) Get(ctx context.Context, id int64) (*ImageChannelMonitor, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptInPlace(m)
	return m, nil
}

func (s *ImageChannelMonitorService) Create(
	ctx context.Context,
	p ImageChannelMonitorCreateParams,
) (*ImageChannelMonitor, error) {
	m, plainAPIKey, err := s.buildCreateMonitor(ctx, p)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create image channel monitor: %w", err)
	}
	m.APIKey = plainAPIKey
	if s.scheduler != nil {
		s.scheduler.Schedule(m)
	}
	return m, nil
}

func (s *ImageChannelMonitorService) Update(
	ctx context.Context,
	id int64,
	p ImageChannelMonitorUpdateParams,
) (*ImageChannelMonitor, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	plainAPIKey, err := s.applyUpdate(ctx, existing, p)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update image channel monitor: %w", err)
	}
	if plainAPIKey != nil {
		existing.APIKey = *plainAPIKey
	} else {
		s.decryptInPlace(existing)
	}
	if s.scheduler != nil {
		s.scheduler.Schedule(existing)
	}
	return existing, nil
}

func (s *ImageChannelMonitorService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete image channel monitor: %w", err)
	}
	s.runtimeMu.Lock()
	delete(s.runtimeStatus, id)
	s.runtimeMu.Unlock()
	if s.scheduler != nil {
		s.scheduler.Unschedule(id)
	}
	return nil
}

func (s *ImageChannelMonitorService) ListHistory(
	ctx context.Context,
	id int64,
	limit int,
) ([]*ImageChannelMonitorHistoryEntry, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = MonitorHistoryDefaultLimit
	}
	if limit > MonitorHistoryMaxLimit {
		limit = MonitorHistoryMaxLimit
	}
	return s.repo.ListHistory(ctx, id, limit)
}

func (s *ImageChannelMonitorService) GetRuntimeStatus(
	ctx context.Context,
	id int64,
) (*ImageChannelMonitorRuntimeStatus, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	status := ImageChannelMonitorRuntimeStatus{
		MonitorID: id,
		Running:   false,
		Stage:     "idle",
		Message:   "",
	}

	s.runtimeMu.RLock()
	if current, ok := s.runtimeStatus[id]; ok {
		status = current
	}
	s.runtimeMu.RUnlock()

	now := time.Now()
	if m.Enabled && m.IntervalSeconds > 0 {
		var next time.Time
		if m.LastCheckedAt != nil {
			next = m.LastCheckedAt.Add(time.Duration(m.IntervalSeconds) * time.Second)
			if next.Before(now) {
				next = now
			}
		} else {
			next = now
		}
		seconds := int(time.Until(next).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		status.NextCheckAt = &next
		status.SecondsUntilNextCheck = &seconds
	}

	return &status, nil
}

func (s *ImageChannelMonitorService) ListEnabledMonitors(ctx context.Context) ([]*ImageChannelMonitor, error) {
	items, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		s.decryptInPlace(it)
	}
	return items, nil
}

func (s *ImageChannelMonitorService) RunCheck(ctx context.Context, id int64) (*ImageChannelMonitorResult, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !s.tryBeginRuntimeStatus(m.ID, "source", "resolving monitor source") {
		return nil, ErrImageChannelMonitorAlreadyRunning
	}
	if m.SourceType == ImageChannelMonitorSourceCustom && m.APIKeyDecryptFailed {
		s.finishRuntimeStatus(m.ID, "source", "api key decrypt failed")
		return nil, ErrImageChannelMonitorAPIKeyDecryptFailed
	}
	result := s.runCheck(ctx, m)
	s.finishRuntimeStatus(m.ID, finalImageMonitorRuntimeStage(result), result.Message)
	s.persistResult(ctx, m, result)
	return result, nil
}

func (s *ImageChannelMonitorService) RunCheckAsync(
	ctx context.Context,
	id int64,
) (*ImageChannelMonitorRuntimeStatus, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.SourceType == ImageChannelMonitorSourceCustom && m.APIKeyDecryptFailed {
		return nil, ErrImageChannelMonitorAPIKeyDecryptFailed
	}
	if !s.tryBeginRuntimeStatus(m.ID, "queued", "image monitor check queued") {
		return s.GetRuntimeStatus(ctx, id)
	}

	go s.runCheckDetached(m)

	return s.GetRuntimeStatus(ctx, id)
}

func (s *ImageChannelMonitorService) runCheckDetached(m *ImageChannelMonitor) {
	defer func() {
		if rec := recover(); rec != nil {
			msg := fmt.Sprintf("image monitor check panic: %v", rec)
			s.finishRuntimeStatus(m.ID, "error", truncateMessage(sanitizeErrorMessage(msg)))
			slog.Error("image_channel_monitor: async run panic recovered",
				"monitor_id", m.ID, "name", m.Name, "panic", rec)
		}
	}()
	result := s.runCheck(context.Background(), m)
	s.finishRuntimeStatus(m.ID, finalImageMonitorRuntimeStage(result), result.Message)
	s.persistResult(context.Background(), m, result)
}

func (s *ImageChannelMonitorService) tryBeginRuntimeStatus(id int64, stage, message string) bool {
	if id <= 0 {
		return false
	}
	now := time.Now()
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if s.runtimeStatus == nil {
		s.runtimeStatus = make(map[int64]ImageChannelMonitorRuntimeStatus)
	}
	if current, ok := s.runtimeStatus[id]; ok && current.Running {
		return false
	}
	s.runtimeStatus[id] = ImageChannelMonitorRuntimeStatus{
		MonitorID: id,
		Running:   true,
		Stage:     stage,
		Message:   message,
		StartedAt: &now,
		UpdatedAt: &now,
	}
	return true
}

func (s *ImageChannelMonitorService) updateRuntimeStatus(id int64, stage, message string) {
	if id <= 0 {
		return
	}
	now := time.Now()
	s.runtimeMu.Lock()
	if s.runtimeStatus == nil {
		s.runtimeStatus = make(map[int64]ImageChannelMonitorRuntimeStatus)
	}
	status := s.runtimeStatus[id]
	if status.MonitorID == 0 {
		status.MonitorID = id
		status.Running = true
		status.StartedAt = &now
	}
	status.Stage = stage
	status.Message = message
	status.UpdatedAt = &now
	s.runtimeStatus[id] = status
	s.runtimeMu.Unlock()
}

func (s *ImageChannelMonitorService) finishRuntimeStatus(id int64, stage, message string) {
	if id <= 0 {
		return
	}
	now := time.Now()
	s.runtimeMu.Lock()
	if s.runtimeStatus == nil {
		s.runtimeStatus = make(map[int64]ImageChannelMonitorRuntimeStatus)
	}
	status := s.runtimeStatus[id]
	if status.MonitorID == 0 {
		status.MonitorID = id
		status.StartedAt = &now
	}
	status.Running = false
	status.Stage = stage
	status.Message = message
	status.UpdatedAt = &now
	status.CompletedAt = &now
	s.runtimeStatus[id] = status
	s.runtimeMu.Unlock()
}

func (s *ImageChannelMonitorService) buildCreateMonitor(
	ctx context.Context,
	p ImageChannelMonitorCreateParams,
) (*ImageChannelMonitor, string, error) {
	if p.N == 0 {
		p.N = 1
	}
	if p.IntervalSeconds == 0 {
		p.IntervalSeconds = imageMonitorDefaultIntervalSeconds
	}
	if p.TimeoutSeconds == 0 {
		p.TimeoutSeconds = imageMonitorDefaultTimeoutSeconds
	}
	m := &ImageChannelMonitor{
		Name:            strings.TrimSpace(p.Name),
		SourceType:      defaultImageMonitorSource(p.SourceType),
		Endpoint:        strings.TrimSpace(p.Endpoint),
		AccountID:       p.AccountID,
		ProxyID:         p.ProxyID,
		Model:           defaultString(p.Model, imageMonitorDefaultModel),
		Prompt:          defaultString(p.Prompt, imageMonitorDefaultPrompt),
		Size:            defaultString(p.Size, imageMonitorDefaultSize),
		Quality:         defaultString(p.Quality, imageMonitorDefaultQuality),
		N:               p.N,
		DownloadImage:   p.DownloadImage,
		Enabled:         p.Enabled,
		IntervalSeconds: p.IntervalSeconds,
		TimeoutSeconds:  p.TimeoutSeconds,
		CreatedBy:       p.CreatedBy,
	}
	plainAPIKey, err := s.normalizeAndSecure(ctx, m, strings.TrimSpace(p.APIKey), true)
	if err != nil {
		return nil, "", err
	}
	return m, plainAPIKey, nil
}

func (s *ImageChannelMonitorService) applyUpdate(
	ctx context.Context,
	m *ImageChannelMonitor,
	p ImageChannelMonitorUpdateParams,
) (*string, error) {
	if p.Name != nil {
		m.Name = strings.TrimSpace(*p.Name)
	}
	if p.SourceType != nil {
		m.SourceType = defaultImageMonitorSource(*p.SourceType)
	}
	if p.Endpoint != nil {
		m.Endpoint = strings.TrimSpace(*p.Endpoint)
	}
	if p.AccountID != nil {
		m.AccountID = p.AccountID
	}
	if p.ProxyID != nil {
		m.ProxyID = p.ProxyID
	}
	if p.Model != nil {
		m.Model = strings.TrimSpace(*p.Model)
	}
	if p.Prompt != nil {
		m.Prompt = strings.TrimSpace(*p.Prompt)
	}
	if p.Size != nil {
		m.Size = strings.TrimSpace(*p.Size)
	}
	if p.Quality != nil {
		m.Quality = strings.TrimSpace(*p.Quality)
	}
	if p.N != nil {
		m.N = *p.N
	}
	if p.DownloadImage != nil {
		m.DownloadImage = *p.DownloadImage
	}
	if p.Enabled != nil {
		m.Enabled = *p.Enabled
	}
	if p.IntervalSeconds != nil {
		m.IntervalSeconds = *p.IntervalSeconds
	}
	if p.TimeoutSeconds != nil {
		m.TimeoutSeconds = *p.TimeoutSeconds
	}
	apiKey := ""
	apiKeyProvided := false
	if p.APIKey != nil && strings.TrimSpace(*p.APIKey) != "" {
		apiKey = strings.TrimSpace(*p.APIKey)
		apiKeyProvided = true
	}
	plain, err := s.normalizeAndSecure(ctx, m, apiKey, apiKeyProvided)
	if err != nil {
		return nil, err
	}
	if apiKeyProvided {
		return &plain, nil
	}
	return nil, nil
}

func (s *ImageChannelMonitorService) normalizeAndSecure(
	ctx context.Context,
	m *ImageChannelMonitor,
	apiKey string,
	apiKeyProvided bool,
) (string, error) {
	if err := validateImageMonitorBase(m); err != nil {
		return "", err
	}
	switch m.SourceType {
	case ImageChannelMonitorSourceCustom:
		if err := validateEndpoint(m.Endpoint); err != nil {
			return "", err
		}
		m.AccountID = nil
		m.AccountName = ""
		if m.ProxyID != nil && *m.ProxyID <= 0 {
			m.ProxyID = nil
			m.ProxyName = ""
		}
		if m.ProxyID != nil {
			proxy, err := s.resolveMonitorProxy(ctx, *m.ProxyID)
			if err != nil {
				return "", err
			}
			m.ProxyName = proxy.Name
		} else {
			m.ProxyName = ""
		}
		if apiKeyProvided {
			encrypted, err := s.encryptor.Encrypt(apiKey)
			if err != nil {
				return "", fmt.Errorf("encrypt image monitor api key: %w", err)
			}
			m.APIKey = encrypted
			return apiKey, nil
		}
		if strings.TrimSpace(m.APIKey) == "" {
			return "", ErrImageChannelMonitorMissingAPIKey
		}
		return "", nil
	case ImageChannelMonitorSourceAccount:
		if m.AccountID == nil || *m.AccountID <= 0 {
			return "", ErrImageChannelMonitorMissingAccount
		}
		account, err := s.accountReader.GetByID(ctx, *m.AccountID)
		if err != nil {
			return "", err
		}
		if !isSupportedImageMonitorAccount(account) {
			return "", ErrImageChannelMonitorUnsupportedAccount
		}
		m.Endpoint = ""
		m.APIKey = ""
		m.AccountName = account.Name
		m.ProxyID = nil
		m.ProxyName = ""
		return "", nil
	default:
		return "", ErrImageChannelMonitorInvalidSource
	}
}

func (s *ImageChannelMonitorService) resolveMonitorProxy(ctx context.Context, id int64) (*Proxy, error) {
	if id <= 0 {
		return nil, ErrProxyNotFound
	}
	if s.proxyReader == nil {
		return nil, ErrProxyNotFound
	}
	return s.proxyReader.GetByID(ctx, id)
}

func validateImageMonitorBase(m *ImageChannelMonitor) error {
	if m == nil {
		return ErrImageChannelMonitorNotFound
	}
	if m.SourceType != ImageChannelMonitorSourceCustom && m.SourceType != ImageChannelMonitorSourceAccount {
		return ErrImageChannelMonitorInvalidSource
	}
	if strings.TrimSpace(m.Model) == "" {
		return ErrImageChannelMonitorMissingModel
	}
	if strings.TrimSpace(m.Prompt) == "" {
		return ErrImageChannelMonitorMissingPrompt
	}
	if m.N < 1 || m.N > 10 {
		return ErrImageChannelMonitorInvalidN
	}
	if m.IntervalSeconds < monitorMinIntervalSeconds || m.IntervalSeconds > monitorMaxIntervalSeconds {
		return ErrImageChannelMonitorInvalidInterval
	}
	if m.TimeoutSeconds < 30 || m.TimeoutSeconds > 600 {
		return ErrImageChannelMonitorInvalidTimeout
	}
	m.Endpoint = normalizeEndpoint(m.Endpoint)
	m.Model = strings.TrimSpace(m.Model)
	m.Prompt = strings.TrimSpace(m.Prompt)
	m.Size = defaultString(m.Size, imageMonitorDefaultSize)
	m.Quality = defaultString(m.Quality, imageMonitorDefaultQuality)
	return nil
}

type imageMonitorResolvedSource struct {
	endpoint    string
	apiKey      string
	proxyURL    string
	accountID   int64
	concurrency int
	tlsProfile  *tlsfingerprint.Profile
	userAgent   string
}

func (s *ImageChannelMonitorService) resolveSource(
	ctx context.Context,
	m *ImageChannelMonitor,
) (*imageMonitorResolvedSource, error) {
	switch m.SourceType {
	case ImageChannelMonitorSourceCustom:
		proxyURL := ""
		if m.ProxyID != nil {
			proxy, err := s.resolveMonitorProxy(ctx, *m.ProxyID)
			if err != nil {
				return nil, err
			}
			proxyURL = proxy.URL()
		}
		return &imageMonitorResolvedSource{
			endpoint:    m.Endpoint,
			apiKey:      m.APIKey,
			proxyURL:    proxyURL,
			concurrency: 1,
			userAgent:   openAIImagesAPIKeyUserAgent,
		}, nil
	case ImageChannelMonitorSourceAccount:
		if m.AccountID == nil {
			return nil, ErrImageChannelMonitorMissingAccount
		}
		account, err := s.accountReader.GetByID(ctx, *m.AccountID)
		if err != nil {
			return nil, err
		}
		if !isSupportedImageMonitorAccount(account) {
			return nil, ErrImageChannelMonitorUnsupportedAccount
		}
		proxyURL := ""
		if account.ProxyID != nil && account.Proxy != nil {
			proxyURL = account.Proxy.URL()
		}
		userAgent := account.GetOpenAIUserAgent()
		if userAgent == "" {
			userAgent = openAIImagesAPIKeyUserAgent
		}
		var profile *tlsfingerprint.Profile
		if s.tlsFPProfileService != nil {
			profile = s.tlsFPProfileService.ResolveTLSProfile(account)
		}
		return &imageMonitorResolvedSource{
			endpoint:    account.GetOpenAIBaseURL(),
			apiKey:      account.GetOpenAIApiKey(),
			proxyURL:    proxyURL,
			accountID:   account.ID,
			concurrency: account.Concurrency,
			tlsProfile:  profile,
			userAgent:   userAgent,
		}, nil
	default:
		return nil, ErrImageChannelMonitorInvalidSource
	}
}

func (s *ImageChannelMonitorService) runCheck(
	ctx context.Context,
	m *ImageChannelMonitor,
) *ImageChannelMonitorResult {
	result := &ImageChannelMonitorResult{
		MonitorID: m.ID,
		Status:    MonitorStatusError,
		CheckedAt: time.Now(),
	}
	timeout := time.Duration(m.TimeoutSeconds)*time.Second + imageMonitorRunOneBuffer
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s.updateRuntimeStatus(m.ID, "source", "resolving request source")
	resolved, err := s.resolveSource(runCtx, m)
	if err != nil {
		return failImageMonitorResult(result, "source", err)
	}
	return s.callImageAPI(runCtx, m, resolved, result)
}

func (s *ImageChannelMonitorService) callImageAPI(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
) *ImageChannelMonitorResult {
	s.updateRuntimeStatus(m.ID, "request_build", "building image generation request")
	bodyBytes, err := json.Marshal(buildImageMonitorPayload(m))
	if err != nil {
		return failImageMonitorResult(result, "request_build", err)
	}
	targetURL := buildOpenAIImagesURL(resolved.endpoint, openAIImagesGenerationsEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return failImageMonitorResult(result, "request_build", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Authorization", "Bearer "+resolved.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", resolved.userAgent)
	ensureOpenAIImagesAPIKeyUserAgent(req)

	start := time.Now()
	s.updateRuntimeStatus(m.ID, "api_connect", "waiting for upstream image API headers")
	resp, err := s.httpUpstream.DoWithTLS(
		req,
		resolved.proxyURL,
		resolved.accountID,
		resolved.concurrency,
		resolved.tlsProfile,
	)
	headerMs := int(time.Since(start) / time.Millisecond)
	result.APIHeaderMs = &headerMs
	if err != nil {
		return failImageMonitorResult(result, "api_connect", err)
	}
	defer func() { _ = resp.Body.Close() }()

	s.updateRuntimeStatus(m.ID, "api_headers", fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode))
	result.HTTPStatus = imageMonitorIntPtr(resp.StatusCode)
	bodyStart := time.Now()
	s.updateRuntimeStatus(m.ID, "api_body", "reading upstream image API body")
	rawBody, readErr := io.ReadAll(io.LimitReader(resp.Body, imageMonitorMaxResponseBytes+1))
	bodyMs := int(time.Since(bodyStart) / time.Millisecond)
	totalMs := int(time.Since(start) / time.Millisecond)
	jsonBytes := len(rawBody)
	result.APIBodyMs = &bodyMs
	result.APITotalMs = &totalMs
	result.JSONBytes = &jsonBytes
	if readErr != nil {
		return failImageMonitorResult(result, "api_body", readErr)
	}
	if len(rawBody) > imageMonitorMaxResponseBytes {
		return failImageMonitorMessage(result, "api_body", "image API response exceeded monitor limit")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("upstream HTTP %d: %s", resp.StatusCode, truncateForErrorBody(string(rawBody)))
		return failImageMonitorMessage(result, "api_response", msg)
	}
	return s.processImageAPIResponse(ctx, m, resolved, rawBody, result)
}

func buildImageMonitorPayload(m *ImageChannelMonitor) map[string]any {
	payload := map[string]any{
		"model":           strings.TrimSpace(m.Model),
		"prompt":          strings.TrimSpace(m.Prompt),
		"n":               m.N,
		"response_format": openAIImagesDefaultURLFormat,
	}
	if strings.TrimSpace(m.Size) != "" {
		payload["size"] = strings.TrimSpace(m.Size)
	}
	if strings.TrimSpace(m.Quality) != "" {
		payload["quality"] = strings.TrimSpace(m.Quality)
	}
	return payload
}

func (s *ImageChannelMonitorService) processImageAPIResponse(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	rawBody []byte,
	result *ImageChannelMonitorResult,
) *ImageChannelMonitorResult {
	s.updateRuntimeStatus(m.ID, "json_parse", "parsing image API response")
	var parsed struct {
		Data []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return failImageMonitorResult(result, "json_parse", err)
	}
	if len(parsed.Data) == 0 {
		return failedImageMonitorMessage(result, "json_parse", "image API returned empty data")
	}
	first := parsed.Data[0]
	s.updateRuntimeStatus(m.ID, "image_url", "checking returned image payload")
	result.HasURL = strings.TrimSpace(first.URL) != ""
	result.HasB64JSON = strings.TrimSpace(first.B64JSON) != ""
	result.RevisedPrompt = first.RevisedPrompt
	result.ReturnedImageURL = first.URL
	if !result.HasURL {
		if result.HasB64JSON {
			result.ReturnedImageData = "data:image/png;base64," + first.B64JSON
			return failedImageMonitorMessage(result, "image_url", "image API returned b64_json instead of url")
		}
		return failedImageMonitorMessage(result, "image_url", "image API did not return an image url")
	}
	if host := imageURLHost(first.URL); host != "" {
		result.ImageURLHost = host
	}
	if m.DownloadImage {
		s.updateRuntimeStatus(m.ID, "image_download", "downloading returned image")
		if err := s.probeImageURL(ctx, first.URL, resolved, result); err != nil {
			result.Status = MonitorStatusDegraded
			result.ErrorStage = "image_download"
			result.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
			return result
		}
	}
	s.updateRuntimeStatus(m.ID, "complete", "image monitor check completed")
	result.Status = MonitorStatusOperational
	result.Message = ""
	return result
}

func (s *ImageChannelMonitorService) persistResult(
	ctx context.Context,
	m *ImageChannelMonitor,
	result *ImageChannelMonitorResult,
) {
	row := &ImageChannelMonitorHistoryRow{
		MonitorID:        m.ID,
		Status:           result.Status,
		HTTPStatus:       result.HTTPStatus,
		APIHeaderMs:      result.APIHeaderMs,
		APIBodyMs:        result.APIBodyMs,
		APITotalMs:       result.APITotalMs,
		JSONBytes:        result.JSONBytes,
		HasURL:           result.HasURL,
		HasB64JSON:       result.HasB64JSON,
		ImageURLHost:     result.ImageURLHost,
		ImageFirstByteMs: result.ImageFirstByteMs,
		ImageDownloadMs:  result.ImageDownloadMs,
		ImageBytes:       result.ImageBytes,
		ImageContentType: result.ImageContentType,
		ImageWidth:       result.ImageWidth,
		ImageHeight:      result.ImageHeight,
		ErrorStage:       result.ErrorStage,
		Message:          result.Message,
		CheckedAt:        result.CheckedAt,
	}
	if err := s.repo.InsertHistory(ctx, row); err != nil {
		slog.Error("image_channel_monitor: insert history failed", "monitor_id", m.ID, "error", err)
	}
	if err := s.repo.MarkChecked(ctx, m.ID, time.Now()); err != nil {
		slog.Error("image_channel_monitor: mark checked failed", "monitor_id", m.ID, "error", err)
	}
}

func (s *ImageChannelMonitorService) decryptInPlace(m *ImageChannelMonitor) {
	if m == nil || m.SourceType != ImageChannelMonitorSourceCustom || strings.TrimSpace(m.APIKey) == "" {
		return
	}
	plain, err := s.encryptor.Decrypt(m.APIKey)
	if err != nil {
		m.APIKey = ""
		m.APIKeyDecryptFailed = true
		return
	}
	m.APIKey = plain
	m.APIKeyDecryptFailed = false
}

func (s *ImageChannelMonitorService) probeImageURL(
	ctx context.Context,
	rawURL string,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if resolved != nil && strings.TrimSpace(resolved.userAgent) != "" {
		req.Header.Set("User-Agent", resolved.userAgent)
	}
	start := time.Now()
	if s.httpUpstream == nil {
		return fmt.Errorf("http upstream is not configured")
	}
	proxyURL := ""
	accountID := int64(0)
	concurrency := 1
	if resolved != nil {
		proxyURL = resolved.proxyURL
		accountID = resolved.accountID
		concurrency = resolved.concurrency
		if concurrency <= 0 {
			concurrency = 1
		}
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, accountID, concurrency)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("image download HTTP %d", resp.StatusCode)
	}
	result.ImageContentType = strings.TrimSpace(resp.Header.Get("Content-Type"))
	limited := io.LimitReader(resp.Body, imageMonitorMaxDownloadBytes+1)
	var buf bytes.Buffer
	tmp := make([]byte, 32*1024)
	firstByteRecorded := false
	for {
		n, readErr := limited.Read(tmp)
		if n > 0 {
			if !firstByteRecorded {
				firstMs := int(time.Since(start) / time.Millisecond)
				result.ImageFirstByteMs = &firstMs
				firstByteRecorded = true
			}
			if _, err := buf.Write(tmp[:n]); err != nil {
				return err
			}
			if buf.Len() > imageMonitorMaxDownloadBytes {
				return fmt.Errorf("image download exceeded monitor limit")
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	downloadMs := int(time.Since(start) / time.Millisecond)
	imageBytes := int64(buf.Len())
	result.ImageDownloadMs = &downloadMs
	result.ImageBytes = &imageBytes
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(buf.Bytes())); err == nil {
		result.ImageWidth = imageMonitorIntPtr(cfg.Width)
		result.ImageHeight = imageMonitorIntPtr(cfg.Height)
	}
	return nil
}

func failImageMonitorResult(
	result *ImageChannelMonitorResult,
	stage string,
	err error,
) *ImageChannelMonitorResult {
	if err == nil {
		return failImageMonitorMessage(result, stage, "")
	}
	return failImageMonitorMessage(result, stage, err.Error())
}

func failImageMonitorMessage(
	result *ImageChannelMonitorResult,
	stage string,
	message string,
) *ImageChannelMonitorResult {
	result.Status = MonitorStatusError
	result.ErrorStage = stage
	result.Message = truncateMessage(sanitizeErrorMessage(message))
	return result
}

func failedImageMonitorMessage(
	result *ImageChannelMonitorResult,
	stage string,
	message string,
) *ImageChannelMonitorResult {
	result.Status = MonitorStatusFailed
	result.ErrorStage = stage
	result.Message = truncateMessage(sanitizeErrorMessage(message))
	return result
}

func defaultImageMonitorSource(sourceType string) string {
	sourceType = strings.TrimSpace(sourceType)
	if sourceType == "" {
		return ImageChannelMonitorSourceCustom
	}
	return sourceType
}

func defaultString(v string, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func isSupportedImageMonitorAccount(account *Account) bool {
	return account != nil && account.IsOpenAIApiKey() && strings.TrimSpace(account.GetOpenAIApiKey()) != ""
}

func imageURLHost(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func imageMonitorIntPtr(v int) *int {
	return &v
}

func finalImageMonitorRuntimeStage(result *ImageChannelMonitorResult) string {
	if result == nil {
		return "error"
	}
	if result.Status == MonitorStatusOperational {
		return "complete"
	}
	if strings.TrimSpace(result.ErrorStage) != "" {
		return result.ErrorStage
	}
	if strings.TrimSpace(result.Status) != "" {
		return result.Status
	}
	return "error"
}
