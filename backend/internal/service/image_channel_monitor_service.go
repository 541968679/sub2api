package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
	manualMu            sync.RWMutex
	manualRuns          map[string]ImageChannelMonitorManualRunStatus
	manualCancels       map[string]context.CancelFunc
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
		manualRuns:          make(map[string]ImageChannelMonitorManualRunStatus),
		manualCancels:       make(map[string]context.CancelFunc),
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

func (s *ImageChannelMonitorService) RunManualCheck(
	ctx context.Context,
	id int64,
	p ImageChannelMonitorManualTestParams,
) (*ImageChannelMonitorManualTestResult, error) {
	m, manual, mode, err := s.prepareManualCheck(ctx, id, p)
	if err != nil {
		return nil, err
	}
	result := s.runManualCheck(ctx, manual, mode, p)
	return &ImageChannelMonitorManualTestResult{
		Monitor: m,
		Mode:    mode,
		Result:  result,
	}, nil
}

func (s *ImageChannelMonitorService) StartManualCheck(
	ctx context.Context,
	id int64,
	p ImageChannelMonitorManualTestParams,
) (*ImageChannelMonitorManualRunStatus, error) {
	m, manual, mode, err := s.prepareManualCheck(ctx, id, p)
	if err != nil {
		return nil, err
	}
	runID := newImageMonitorManualRunID()
	now := time.Now()
	runCtx, cancel := context.WithCancel(context.Background())
	status := ImageChannelMonitorManualRunStatus{
		RunID:     runID,
		Monitor:   m,
		Mode:      mode,
		Running:   true,
		Stage:     "queued",
		Message:   "manual image test queued",
		StartedAt: now,
		UpdatedAt: now,
	}
	s.manualMu.Lock()
	if s.manualRuns == nil {
		s.manualRuns = make(map[string]ImageChannelMonitorManualRunStatus)
	}
	if s.manualCancels == nil {
		s.manualCancels = make(map[string]context.CancelFunc)
	}
	s.pruneManualRunsLocked(now)
	s.manualRuns[runID] = status
	s.manualCancels[runID] = cancel
	s.manualMu.Unlock()

	go s.runManualCheckDetached(runCtx, runID, manual, mode, p)

	return s.GetManualCheckStatus(ctx, runID)
}

func (s *ImageChannelMonitorService) GetManualCheckStatus(
	_ context.Context,
	runID string,
) (*ImageChannelMonitorManualRunStatus, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	s.manualMu.RLock()
	status, ok := s.manualRuns[runID]
	s.manualMu.RUnlock()
	if !ok {
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	return &status, nil
}

func (s *ImageChannelMonitorService) CancelManualCheck(
	_ context.Context,
	runID string,
) (*ImageChannelMonitorManualRunStatus, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	now := time.Now()
	s.manualMu.Lock()
	status, ok := s.manualRuns[runID]
	if !ok {
		s.manualMu.Unlock()
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	if status.Running {
		status.Running = false
		status.Canceled = true
		status.Stage = "canceled"
		status.Message = "manual image test canceled"
		status.UpdatedAt = now
		status.CompletedAt = &now
		status.Result = nil
		s.manualRuns[runID] = status
	}
	cancel := s.manualCancels[runID]
	delete(s.manualCancels, runID)
	s.manualMu.Unlock()
	if cancel != nil {
		cancel()
	}
	return &status, nil
}

func (s *ImageChannelMonitorService) prepareManualCheck(
	ctx context.Context,
	id int64,
	p ImageChannelMonitorManualTestParams,
) (*ImageChannelMonitor, *ImageChannelMonitor, string, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, nil, "", err
	}
	if m.SourceType == ImageChannelMonitorSourceCustom && m.APIKeyDecryptFailed {
		return nil, nil, "", ErrImageChannelMonitorAPIKeyDecryptFailed
	}
	mode := normalizeImageMonitorManualMode(p.Mode)
	if mode == "" {
		return nil, nil, "", ErrImageChannelMonitorInvalidManualMode
	}

	manual := *m
	if strings.TrimSpace(p.Model) != "" {
		manual.Model = strings.TrimSpace(p.Model)
	}
	if strings.TrimSpace(p.Prompt) != "" {
		manual.Prompt = strings.TrimSpace(p.Prompt)
	}
	manual.Size = strings.TrimSpace(p.Size)
	manual.Quality = defaultString(p.Quality, imageMonitorDefaultQuality)
	if p.N > 0 {
		manual.N = p.N
	}
	manual.DownloadImage = p.DownloadImage
	if p.TimeoutSeconds > 0 {
		manual.TimeoutSeconds = p.TimeoutSeconds
	}
	if err := validateImageMonitorBase(&manual); err != nil {
		return nil, nil, "", err
	}
	return m, &manual, mode, nil
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

func (s *ImageChannelMonitorService) runManualCheckDetached(
	ctx context.Context,
	runID string,
	m *ImageChannelMonitor,
	mode string,
	p ImageChannelMonitorManualTestParams,
) {
	defer func() {
		if rec := recover(); rec != nil {
			msg := fmt.Sprintf("manual image monitor check panic: %v", rec)
			s.finishManualRunStatus(runID, "error", truncateMessage(sanitizeErrorMessage(msg)), nil)
			slog.Error("image_channel_monitor: manual run panic recovered",
				"run_id", runID, "monitor_id", m.ID, "name", m.Name, "panic", rec)
		}
	}()
	result := s.runManualCheckWithStage(ctx, m, mode, p, func(stage, message string) {
		s.updateManualRunStatus(runID, stage, message)
	})
	s.finishManualRunStatus(runID, finalImageMonitorRuntimeStage(result), result.Message, result)
}

func (s *ImageChannelMonitorService) updateManualRunStatus(runID, stage, message string) {
	now := time.Now()
	s.manualMu.Lock()
	if s.manualRuns == nil {
		s.manualRuns = make(map[string]ImageChannelMonitorManualRunStatus)
	}
	status, ok := s.manualRuns[runID]
	if !ok {
		s.manualMu.Unlock()
		return
	}
	if status.Canceled {
		s.manualMu.Unlock()
		return
	}
	status.Stage = strings.TrimSpace(stage)
	status.Message = strings.TrimSpace(message)
	status.UpdatedAt = now
	s.manualRuns[runID] = status
	s.manualMu.Unlock()
}

func (s *ImageChannelMonitorService) finishManualRunStatus(
	runID string,
	stage string,
	message string,
	result *ImageChannelMonitorResult,
) {
	now := time.Now()
	s.manualMu.Lock()
	if s.manualRuns == nil {
		s.manualRuns = make(map[string]ImageChannelMonitorManualRunStatus)
	}
	status, ok := s.manualRuns[runID]
	if !ok {
		s.manualMu.Unlock()
		return
	}
	if status.Canceled {
		delete(s.manualCancels, runID)
		s.manualMu.Unlock()
		return
	}
	status.Running = false
	status.Stage = strings.TrimSpace(stage)
	status.Message = strings.TrimSpace(message)
	status.UpdatedAt = now
	status.CompletedAt = &now
	status.Result = result
	s.manualRuns[runID] = status
	delete(s.manualCancels, runID)
	s.manualMu.Unlock()
}

func (s *ImageChannelMonitorService) pruneManualRunsLocked(now time.Time) {
	if len(s.manualRuns) == 0 {
		return
	}
	cutoff := now.Add(-imageMonitorManualRunRetention)
	for runID, status := range s.manualRuns {
		if status.Running || status.CompletedAt == nil {
			continue
		}
		if status.CompletedAt.Before(cutoff) {
			delete(s.manualRuns, runID)
			delete(s.manualCancels, runID)
		}
	}
	for runID, status := range s.manualRuns {
		if len(s.manualRuns) <= imageMonitorManualRunMax {
			return
		}
		if !status.Running {
			delete(s.manualRuns, runID)
			delete(s.manualCancels, runID)
		}
	}
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
		Size:            strings.TrimSpace(p.Size),
		Quality:         defaultString(p.Quality, imageMonitorDefaultQuality),
		N:               p.N,
		DownloadImage:   p.DownloadImage,
		Enabled:         p.Enabled,
		PublicVisible:   p.PublicVisible,
		PublicName:      strings.TrimSpace(p.PublicName),
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
	if p.PublicVisible != nil {
		m.PublicVisible = *p.PublicVisible
	}
	if p.PublicName != nil {
		m.PublicName = strings.TrimSpace(*p.PublicName)
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
	m.Size = strings.TrimSpace(m.Size)
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
	stage := imageMonitorStageReporter(result, func(stage, message string) {
		s.updateRuntimeStatus(m.ID, stage, message)
	})
	timeout := time.Duration(m.TimeoutSeconds)*time.Second + imageMonitorRunOneBuffer
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stage("source", "resolving request source")
	resolved, err := s.resolveSource(runCtx, m)
	if err != nil {
		return failImageMonitorResult(result, "source", err)
	}
	return s.callImageAPI(runCtx, m, resolved, result, stage, false)
}

func (s *ImageChannelMonitorService) runManualCheck(
	ctx context.Context,
	m *ImageChannelMonitor,
	mode string,
	p ImageChannelMonitorManualTestParams,
) *ImageChannelMonitorResult {
	return s.runManualCheckWithStage(ctx, m, mode, p, nil)
}

func (s *ImageChannelMonitorService) runManualCheckWithStage(
	ctx context.Context,
	m *ImageChannelMonitor,
	mode string,
	p ImageChannelMonitorManualTestParams,
	hook imageMonitorStageFunc,
) *ImageChannelMonitorResult {
	result := &ImageChannelMonitorResult{
		MonitorID: m.ID,
		Status:    MonitorStatusError,
		CheckedAt: time.Now(),
	}
	stage := imageMonitorStageReporter(result, hook)
	timeout := time.Duration(m.TimeoutSeconds)*time.Second + imageMonitorRunOneBuffer
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stage("source", "resolving request source")
	resolved, err := s.resolveSource(runCtx, m)
	if err != nil {
		return failImageMonitorResult(result, "source", err)
	}
	if mode == ImageChannelMonitorManualEdit {
		return s.callImageEditAPI(runCtx, m, resolved, result, stage, p)
	}
	return s.callImageAPI(runCtx, m, resolved, result, stage, true)
}

func (s *ImageChannelMonitorService) callImageAPI(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
	stage imageMonitorStageFunc,
	allowB64JSON bool,
) *ImageChannelMonitorResult {
	stage("request_build", "building image generation request")
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

	return s.performImageMonitorAPIRequest(ctx, m, resolved, result, req, stage, allowB64JSON, allowB64JSON)
}

func (s *ImageChannelMonitorService) callImageEditAPI(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
	stage imageMonitorStageFunc,
	p ImageChannelMonitorManualTestParams,
) *ImageChannelMonitorResult {
	stage("request_build", "building image edit request")
	bodyBytes, contentType, err := buildImageMonitorEditPayload(m, p)
	if err != nil {
		return failImageMonitorResult(result, "request_build", err)
	}
	targetURL := buildOpenAIImagesURL(resolved.endpoint, openAIImagesEditsEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return failImageMonitorResult(result, "request_build", err)
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Header.Set("Authorization", "Bearer "+resolved.apiKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", resolved.userAgent)
	ensureOpenAIImagesAPIKeyUserAgent(req)

	return s.performImageMonitorAPIRequest(ctx, m, resolved, result, req, stage, true, true)
}

func (s *ImageChannelMonitorService) performImageMonitorAPIRequest(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
	req *http.Request,
	stage imageMonitorStageFunc,
	allowB64JSON bool,
	probeExitIP bool,
) *ImageChannelMonitorResult {
	s.captureImageMonitorRequestNetwork(ctx, req, resolved, result, probeExitIP)
	start := time.Now()
	stage("api_connect", "waiting for upstream image API headers")
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

	stage("api_headers", fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode))
	result.HTTPStatus = imageMonitorIntPtr(resp.StatusCode)
	bodyStart := time.Now()
	stage("api_body", "reading upstream image API body")
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
	return s.processImageAPIResponse(ctx, m, resolved, rawBody, result, stage, allowB64JSON)
}

func (s *ImageChannelMonitorService) captureImageMonitorRequestNetwork(
	ctx context.Context,
	req *http.Request,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
	probeExitIP bool,
) {
	if req != nil && req.URL != nil {
		result.RequestTargetURL = req.URL.String()
		result.RequestTargetHost, result.RequestTargetIPs = resolveImageMonitorURLIPs(ctx, req.URL.String())
	}
	if probeExitIP && result.ExitIP == "" {
		result.ExitIP = s.probeImageMonitorExitIP(ctx, resolved)
	}
}

func resolveImageMonitorURLIPs(ctx context.Context, rawURL string) (string, []string) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return "", nil
	}
	host := parsed.Hostname()
	if host == "" {
		return "", nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, []string{ip.String()}
	}
	lookupCtx, cancel := context.WithTimeout(ctx, imageMonitorNetworkProbeTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return host, nil
	}
	ips := make([]string, 0, len(addrs))
	seen := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		ip := addr.IP.String()
		if ip == "" {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		ips = append(ips, ip)
	}
	return host, ips
}

func (s *ImageChannelMonitorService) probeImageMonitorExitIP(
	ctx context.Context,
	resolved *imageMonitorResolvedSource,
) string {
	if s.httpUpstream == nil {
		return ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, imageMonitorNetworkProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, imageMonitorExitIPProbeURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", openAIImagesAPIKeyUserAgent)
	proxyURL := ""
	accountID := int64(0)
	concurrency := 1
	var tlsProfile *tlsfingerprint.Profile
	if resolved != nil {
		proxyURL = resolved.proxyURL
		accountID = resolved.accountID
		concurrency = resolved.concurrency
		tlsProfile = resolved.tlsProfile
		if concurrency <= 0 {
			concurrency = 1
		}
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, accountID, concurrency, tlsProfile)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ""
	}
	ip := strings.TrimSpace(string(body))
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.String()
	}
	return ""
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

func buildImageMonitorEditPayload(
	m *ImageChannelMonitor,
	p ImageChannelMonitorManualTestParams,
) ([]byte, string, error) {
	imageBytes, contentType, err := decodeImageMonitorInputImage(p.InputImageData, p.InputImageType)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fields := map[string]string{
		"model":           strings.TrimSpace(m.Model),
		"prompt":          strings.TrimSpace(m.Prompt),
		"n":               strconv.Itoa(m.N),
		"response_format": openAIImagesDefaultURLFormat,
	}
	if strings.TrimSpace(m.Size) != "" {
		fields["size"] = strings.TrimSpace(m.Size)
	}
	if strings.TrimSpace(m.Quality) != "" {
		fields["quality"] = strings.TrimSpace(m.Quality)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", fmt.Errorf("write image edit field %s: %w", key, err)
		}
	}
	fileName := sanitizeImageMonitorFileName(p.InputImageName)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename="%s"`, fileName))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", fmt.Errorf("create image edit file part: %w", err)
	}
	if _, err := part.Write(imageBytes); err != nil {
		return nil, "", fmt.Errorf("write image edit file part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("finalize image edit body: %w", err)
	}
	return buf.Bytes(), writer.FormDataContentType(), nil
}

func (s *ImageChannelMonitorService) processImageAPIResponse(
	ctx context.Context,
	m *ImageChannelMonitor,
	resolved *imageMonitorResolvedSource,
	rawBody []byte,
	result *ImageChannelMonitorResult,
	stage imageMonitorStageFunc,
	allowB64JSON bool,
) *ImageChannelMonitorResult {
	stage("json_parse", "parsing image API response")
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
	stage("image_url", "checking returned image payload")
	result.HasURL = strings.TrimSpace(first.URL) != ""
	result.HasB64JSON = strings.TrimSpace(first.B64JSON) != ""
	result.RevisedPrompt = first.RevisedPrompt
	result.ReturnedImageURL = first.URL
	if !result.HasURL {
		if result.HasB64JSON {
			result.ReturnedImageData = "data:image/png;base64," + first.B64JSON
			fillImageMonitorInlineImageInfo(result, first.B64JSON)
			if allowB64JSON {
				stage("complete", "image monitor check completed")
				result.Status = MonitorStatusOperational
				result.Message = ""
				return result
			}
			return failedImageMonitorMessage(result, "image_url", "image API returned b64_json instead of url")
		}
		return failedImageMonitorMessage(result, "image_url", "image API did not return an image url")
	}
	if host := imageURLHost(first.URL); host != "" {
		result.ImageURLHost = host
	}
	if m.DownloadImage {
		stage("image_download", "downloading returned image")
		if err := s.probeImageURL(ctx, first.URL, resolved, result); err != nil {
			result.Status = MonitorStatusDegraded
			result.ErrorStage = "image_download"
			result.Message = truncateMessage(sanitizeErrorMessage(err.Error()))
			return result
		}
	}
	stage("complete", "image monitor check completed")
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

// fillImageMonitorInlineImageInfo records byte-size/dimension metadata for
// inline b64_json payloads, which never pass through probeImageURL.
func fillImageMonitorInlineImageInfo(result *ImageChannelMonitorResult, b64Payload string) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Payload))
	if err != nil || len(decoded) == 0 {
		return
	}
	imageBytes := int64(len(decoded))
	result.ImageBytes = &imageBytes
	result.ImageContentType = http.DetectContentType(decoded)
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(decoded)); err == nil {
		result.ImageWidth = imageMonitorIntPtr(cfg.Width)
		result.ImageHeight = imageMonitorIntPtr(cfg.Height)
	}
}

func (s *ImageChannelMonitorService) probeImageURL(
	ctx context.Context,
	rawURL string,
	resolved *imageMonitorResolvedSource,
	result *ImageChannelMonitorResult,
) error {
	result.ImageDownloadURL = rawURL
	result.ImageDownloadHost, result.ImageDownloadIPs = resolveImageMonitorURLIPs(ctx, rawURL)
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
	if result.ReturnedImageData == "" && buf.Len() <= imageMonitorMaxReturnedImageData {
		contentType := result.ImageContentType
		if contentType == "" {
			contentType = http.DetectContentType(buf.Bytes())
		}
		if idx := strings.Index(contentType, ";"); idx >= 0 {
			contentType = strings.TrimSpace(contentType[:idx])
		}
		if strings.HasPrefix(contentType, "image/") {
			result.ReturnedImageData = "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		}
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

func normalizeImageMonitorManualMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ImageChannelMonitorManualGenerate:
		return ImageChannelMonitorManualGenerate
	case ImageChannelMonitorManualEdit:
		return ImageChannelMonitorManualEdit
	default:
		return ""
	}
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

func newImageMonitorManualRunID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
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

type imageMonitorStageFunc func(stage, message string)

func imageMonitorStageReporter(
	result *ImageChannelMonitorResult,
	hook imageMonitorStageFunc,
) imageMonitorStageFunc {
	return func(stage, message string) {
		stage = strings.TrimSpace(stage)
		message = strings.TrimSpace(message)
		if result != nil && stage != "" {
			result.StageEvents = append(result.StageEvents, ImageChannelMonitorStageEvent{
				Stage:   stage,
				Message: message,
				At:      time.Now(),
			})
		}
		if hook != nil {
			hook(stage, message)
		}
	}
}

func decodeImageMonitorInputImage(raw string, fallbackType string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", ErrImageChannelMonitorMissingInputImage
	}
	contentType := strings.TrimSpace(fallbackType)
	encoded := raw
	if strings.HasPrefix(raw, "data:") {
		comma := strings.Index(raw, ",")
		if comma <= len("data:") {
			return nil, "", ErrImageChannelMonitorInvalidInputImage
		}
		meta := raw[len("data:"):comma]
		encoded = raw[comma+1:]
		parts := strings.Split(meta, ";")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			contentType = strings.TrimSpace(parts[0])
		}
		if !containsImageMonitorBase64Marker(parts) {
			return nil, "", ErrImageChannelMonitorInvalidInputImage
		}
	}
	if contentType == "" {
		contentType = "image/png"
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return nil, "", ErrImageChannelMonitorInvalidInputImage
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", ErrImageChannelMonitorInvalidInputImage
	}
	if len(decoded) == 0 || len(decoded) > openAIImageMaxUploadPartSize {
		return nil, "", ErrImageChannelMonitorInvalidInputImage
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(decoded)); err != nil {
		return nil, "", ErrImageChannelMonitorInvalidInputImage
	}
	return decoded, contentType, nil
}

func containsImageMonitorBase64Marker(parts []string) bool {
	for _, part := range parts {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			return true
		}
	}
	return false
}

func sanitizeImageMonitorFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." {
		return "source.png"
	}
	var b strings.Builder
	for _, r := range name {
		if r > unicode.MaxASCII || r < 32 {
			continue
		}
		switch r {
		case '"', '\\', '/', ':', '*', '?', '<', '>', '|':
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "source.png"
	}
	return out
}
