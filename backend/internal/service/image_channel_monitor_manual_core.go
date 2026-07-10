package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	imageChannelManualArtifactFilePrefix        = "sub2api-manual-image-"
	imageChannelManualArtifactSweepEvery        = time.Minute
	imageChannelManualDownloadMaxAttempts       = 4
	imageChannelManualDownloadRetryBaseDelay    = 200 * time.Millisecond
	imageChannelManualDownloadRetryBodyMaxBytes = 32 << 10
)

var errImageChannelManualArtifactSourceRead = errors.New("manual image artifact source read failed")

type imageChannelManualRunOutcome struct {
	result         *ImageChannelMonitorResult
	gatewayStatus  string
	deliveryStatus string
	artifacts      []imageChannelMonitorStoredArtifact
}

type imageChannelManualGatewayMetadata struct {
	Data []imageChannelManualGatewayMetadataEntry `json:"data"`
}

type imageChannelManualGatewayMetadataEntry struct {
	URL           *string `json:"url"`
	B64JSON       *string `json:"b64_json"`
	RevisedPrompt string  `json:"revised_prompt"`
}

type imageChannelManualDownloadMetrics struct {
	downloadMs  int
	firstByteMs *int
	url         string
	host        string
}

func (s *ImageChannelMonitorService) normalizeManualExecutionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ImageChannelMonitorExecutionGatewayGroup:
		return ImageChannelMonitorExecutionGatewayGroup
	case ImageChannelMonitorExecutionGatewayAccount:
		return ImageChannelMonitorExecutionGatewayAccount
	case ImageChannelMonitorExecutionDirectProbe:
		return ImageChannelMonitorExecutionDirectProbe
	case "":
		s.manualMu.RLock()
		gatewayConfigured := s.manualGateway != nil && s.manualAPIKeyReader != nil
		s.manualMu.RUnlock()
		if gatewayConfigured {
			return ImageChannelMonitorExecutionGatewayAccount
		}
		return ImageChannelMonitorExecutionDirectProbe
	default:
		return ""
	}
}

func isImageChannelManualGatewayMode(mode string) bool {
	return mode == ImageChannelMonitorExecutionGatewayGroup || mode == ImageChannelMonitorExecutionGatewayAccount
}

func imageChannelManualPayloadHash(
	monitorID int64,
	mode string,
	p ImageChannelMonitorManualTestParams,
) (string, error) {
	payload := struct {
		MonitorID         int64  `json:"monitor_id"`
		ExecutionMode     string `json:"execution_mode"`
		APIKeyID          int64  `json:"api_key_id"`
		ExpectedAccountID int64  `json:"expected_account_id"`
		Mode              string `json:"mode"`
		Model             string `json:"model"`
		Prompt            string `json:"prompt"`
		Size              string `json:"size"`
		Quality           string `json:"quality"`
		N                 int    `json:"n"`
		DownloadImage     bool   `json:"download_image"`
		ResponseFormat    string `json:"response_format"`
		TimeoutSeconds    int    `json:"timeout_seconds"`
		InputImageHash    string `json:"input_image_hash"`
		InputImageType    string `json:"input_image_type"`
		InputImageName    string `json:"input_image_name"`
	}{
		MonitorID:         monitorID,
		ExecutionMode:     p.ExecutionMode,
		APIKeyID:          p.APIKeyID,
		ExpectedAccountID: p.ExpectedAccountID,
		Mode:              mode,
		Model:             strings.TrimSpace(p.Model),
		Prompt:            strings.TrimSpace(p.Prompt),
		Size:              strings.TrimSpace(p.Size),
		Quality:           strings.TrimSpace(p.Quality),
		N:                 p.N,
		DownloadImage:     p.DownloadImage,
		ResponseFormat:    strings.TrimSpace(p.ResponseFormat),
		TimeoutSeconds:    p.TimeoutSeconds,
		InputImageHash:    imageChannelManualInputHash(p),
		InputImageType:    strings.TrimSpace(p.InputImageType),
		InputImageName:    sanitizeImageMonitorFileName(p.InputImageName),
	}
	hasher := sha256.New()
	if err := json.NewEncoder(hasher).Encode(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func imageChannelManualInputHash(p ImageChannelMonitorManualTestParams) string {
	if len(p.InputImageBytes) > 0 {
		digest := sha256.Sum256(p.InputImageBytes)
		return hex.EncodeToString(digest[:])
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(p.InputImageData)))
	return hex.EncodeToString(digest[:])
}

func (s *ImageChannelMonitorService) lookupManualClientRun(
	monitorID int64,
	clientRunID string,
	payloadHash string,
	idempotent bool,
	now time.Time,
) (*ImageChannelMonitorManualRunStatus, error, bool) {
	s.manualMu.Lock()
	defer s.manualMu.Unlock()
	s.pruneManualRunsLocked(now)
	if status, err, ok := s.manualCanceledClientRunLocked(monitorID, clientRunID, now); ok {
		return status, err, true
	}
	if !idempotent {
		return nil, nil, false
	}
	return s.manualIdempotentRunLocked(clientRunID, payloadHash)
}

func (s *ImageChannelMonitorService) manualCanceledClientRunLocked(
	monitorID int64,
	clientRunID string,
	now time.Time,
) (*ImageChannelMonitorManualRunStatus, error, bool) {
	intent, ok := s.manualCancelIntents[clientRunID]
	if !ok {
		return nil, nil, false
	}
	if intent.monitorID != monitorID {
		return nil, ErrImageChannelMonitorManualRunConflict, true
	}
	return canceledImageChannelManualClientRunStatus(monitorID, clientRunID, now), nil, true
}

func canceledImageChannelManualClientRunStatus(
	monitorID int64,
	clientRunID string,
	now time.Time,
) *ImageChannelMonitorManualRunStatus {
	completedAt := now
	return &ImageChannelMonitorManualRunStatus{
		Monitor:           &ImageChannelMonitor{ID: monitorID},
		ClientRunID:       clientRunID,
		Canceled:          true,
		Stage:             "canceled",
		Message:           "manual image test canceled before start",
		StartedAt:         now,
		UpdatedAt:         now,
		CompletedAt:       &completedAt,
		GatewayStatus:     ImageChannelMonitorGatewayCanceled,
		DeliveryStatus:    ImageChannelMonitorDeliveryCanceled,
		ObservationStatus: ImageChannelMonitorObservationObservable,
		Artifacts:         []ImageChannelMonitorArtifactSummary{},
	}
}

func (s *ImageChannelMonitorService) manualIdempotentRunLocked(
	clientRunID string,
	payloadHash string,
) (*ImageChannelMonitorManualRunStatus, error, bool) {
	entry, ok := s.manualIdempotency[clientRunID]
	if !ok {
		return nil, nil, false
	}
	if entry.payloadHash != payloadHash {
		return nil, ErrImageChannelMonitorManualRunConflict, true
	}
	if status, exists := s.manualRuns[entry.runID]; exists {
		return cloneImageChannelManualRunStatus(status), nil, true
	}
	return nil, ErrImageChannelMonitorManualRunExpired, true
}

func (s *ImageChannelMonitorService) prepareManualGatewayRequest(
	ctx context.Context,
	monitor *ImageChannelMonitor,
	executionMode string,
	p ImageChannelMonitorManualTestParams,
) (string, error) {
	s.manualMu.RLock()
	apiKeyReader := s.manualAPIKeyReader
	groupReader := s.manualGroupReader
	gateway := s.manualGateway
	s.manualMu.RUnlock()
	if apiKeyReader == nil || gateway == nil {
		return "", ErrImageChannelMonitorManualGatewayNotConfigured
	}
	apiKey, err := apiKeyReader.GetByID(ctx, p.APIKeyID)
	if err != nil {
		return "", fmt.Errorf("get manual image API key: %w", err)
	}
	if apiKey == nil || strings.TrimSpace(apiKey.Key) == "" {
		return "", errors.New("manual image API key secret is empty")
	}
	if len(apiKey.IPWhitelist) > 0 || len(apiKey.IPBlacklist) > 0 {
		return "", ErrImageChannelMonitorManualAPIKeyIPRestricted
	}
	if executionMode != ImageChannelMonitorExecutionGatewayAccount {
		return strings.TrimSpace(apiKey.Key), nil
	}
	if p.ExpectedAccountID <= 0 || monitor == nil || monitor.SourceType != ImageChannelMonitorSourceAccount || monitor.AccountID == nil || *monitor.AccountID != p.ExpectedAccountID {
		return "", ErrImageChannelMonitorManualAccountIsolation
	}
	if apiKey.GroupID == nil || groupReader == nil {
		return "", ErrImageChannelMonitorManualAccountIsolation
	}
	accountIDs, err := groupReader.GetAccountIDsByGroupIDs(ctx, []int64{*apiKey.GroupID})
	if err != nil {
		return "", fmt.Errorf("resolve manual API key group accounts: %w", err)
	}
	unique := make(map[int64]struct{}, len(accountIDs))
	for _, accountID := range accountIDs {
		unique[accountID] = struct{}{}
	}
	if len(unique) != 1 {
		return "", ErrImageChannelMonitorManualAccountIsolation
	}
	if _, ok := unique[p.ExpectedAccountID]; !ok {
		return "", ErrImageChannelMonitorManualAccountIsolation
	}
	return strings.TrimSpace(apiKey.Key), nil
}

func cloneImageChannelManualRunStatus(status ImageChannelMonitorManualRunStatus) *ImageChannelMonitorManualRunStatus {
	cloned := status
	if status.Monitor != nil {
		monitor := *status.Monitor
		monitor.APIKey = ""
		monitor.APIKeyDecryptFailed = false
		cloned.Monitor = &monitor
	}
	cloned.Artifacts = append([]ImageChannelMonitorArtifactSummary(nil), status.Artifacts...)
	if status.Result != nil {
		result := *status.Result
		result.ReturnedImageData = ""
		result.RequestTargetIPs = append([]string(nil), status.Result.RequestTargetIPs...)
		result.ImageDownloadIPs = append([]string(nil), status.Result.ImageDownloadIPs...)
		result.GatewayRequestIDs = append([]string(nil), status.Result.GatewayRequestIDs...)
		result.StageEvents = append([]ImageChannelMonitorStageEvent(nil), status.Result.StageEvents...)
		cloned.Result = &result
	}
	return &cloned
}

func imageChannelManualStatusMonitor(monitor *ImageChannelMonitor) *ImageChannelMonitor {
	if monitor == nil {
		return nil
	}
	snapshot := *monitor
	snapshot.APIKey = ""
	snapshot.APIKeyDecryptFailed = false
	return &snapshot
}

func sortImageChannelManualRunsOldestFirst(statuses []ImageChannelMonitorManualRunStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		left, right := statuses[i], statuses[j]
		if left.CompletedAt != nil && right.CompletedAt != nil && !left.CompletedAt.Equal(*right.CompletedAt) {
			return left.CompletedAt.Before(*right.CompletedAt)
		}
		if !left.StartedAt.Equal(right.StartedAt) {
			return left.StartedAt.Before(right.StartedAt)
		}
		return left.RunID < right.RunID
	})
}

func (s *ImageChannelMonitorService) expireManualRunLocked(runID string, now time.Time) {
	if _, ok := s.manualRuns[runID]; !ok {
		return
	}
	artifacts := append([]imageChannelMonitorStoredArtifact(nil), s.manualArtifacts[runID]...)
	delete(s.manualRuns, runID)
	delete(s.manualCancels, runID)
	delete(s.manualArtifacts, runID)
	if s.manualExpired == nil {
		s.manualExpired = make(map[string]time.Time)
	}
	s.manualExpired[runID] = now
	removeImageChannelManualArtifactFiles(artifacts)
}

func (s *ImageChannelMonitorService) deleteManualIdempotencyForRunLocked(runID string) {
	for clientRunID, entry := range s.manualIdempotency {
		if entry.runID == runID {
			delete(s.manualIdempotency, clientRunID)
		}
	}
}

func removeImageChannelManualArtifactFiles(artifacts []imageChannelMonitorStoredArtifact) {
	for _, artifact := range artifacts {
		path := strings.TrimSpace(artifact.path)
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			continue
		}
	}
}

func (s *ImageChannelMonitorService) sweepImageChannelManualOrphansLocked(now time.Time) {
	if !s.manualLastSweep.IsZero() && now.Sub(s.manualLastSweep) < imageChannelManualArtifactSweepEvery {
		return
	}
	s.manualLastSweep = now
	directory := strings.TrimSpace(s.manualArtifactDir)
	if directory == "" {
		return
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	cutoff := now.Add(-imageMonitorManualRunRetention)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), imageChannelManualArtifactFilePrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(directory, entry.Name()))
	}
}

func (s *ImageChannelMonitorService) GetManualCheckImage(
	runID string,
	index int,
) (*ImageChannelMonitorArtifact, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	s.manualMu.Lock()
	defer s.manualMu.Unlock()
	s.pruneManualRunsLocked(time.Now())
	status, ok := s.manualRuns[runID]
	if !ok {
		if _, expired := s.manualExpired[runID]; expired {
			return nil, ErrImageChannelMonitorManualRunExpired
		}
		return nil, ErrImageChannelMonitorManualRunNotFound
	}
	if status.Running {
		return nil, ErrImageChannelMonitorManualImageRunning
	}
	for _, artifact := range s.manualArtifacts[runID] {
		if artifact.Index != index {
			continue
		}
		file, err := os.Open(artifact.path)
		if err != nil {
			return nil, ErrImageChannelMonitorManualImageNotFound
		}
		return &ImageChannelMonitorArtifact{
			ContentType: artifact.ContentType,
			Size:        artifact.Size,
			Reader:      file,
		}, nil
	}
	return nil, ErrImageChannelMonitorManualImageNotFound
}

func (s *ImageChannelMonitorService) runManualGatewayCheck(
	ctx context.Context,
	runID string,
	monitor *ImageChannelMonitor,
	mode string,
	apiKey string,
	p ImageChannelMonitorManualTestParams,
	hook imageMonitorStageFunc,
) (outcome imageChannelManualRunOutcome) {
	result := &ImageChannelMonitorResult{
		MonitorID:      monitor.ID,
		Status:         MonitorStatusError,
		CheckedAt:      time.Now(),
		ResponseFormat: monitor.ResponseFormat,
	}
	outcome = imageChannelManualRunOutcome{
		result:         result,
		gatewayStatus:  ImageChannelMonitorGatewayFailed,
		deliveryStatus: ImageChannelMonitorDeliveryNotRequested,
	}
	defer func() {
		if rec := recover(); rec != nil {
			removeImageChannelManualArtifactFiles(outcome.artifacts)
			panic(rec)
		}
	}()
	stage := imageMonitorStageReporter(result, hook)
	deliveryTimeout := time.Duration(monitor.TimeoutSeconds)*time.Second + imageMonitorRunOneBuffer

	stage("request_build", "building real gateway image request")
	requestPath := openAIImagesGenerationsEndpoint
	contentType := "application/json"
	var body []byte
	var err error
	if mode == ImageChannelMonitorManualEdit {
		requestPath = openAIImagesEditsEndpoint
		body, contentType, err = buildImageMonitorEditPayload(monitor, p)
	} else {
		body, err = json.Marshal(buildImageMonitorPayload(monitor))
	}
	if err != nil {
		outcome.deliveryStatus = ImageChannelMonitorDeliveryFailed
		outcome.result = failImageMonitorResult(result, "request_build", err)
		return outcome
	}

	s.manualMu.RLock()
	gateway := s.manualGateway
	s.manualMu.RUnlock()
	if gateway == nil {
		outcome.result = failImageMonitorResult(result, "gateway", ErrImageChannelMonitorManualGatewayNotConfigured)
		return outcome
	}
	stage("gateway", "waiting for local gateway response")
	response, requestErr := gateway.Do(ctx, ImageManualGatewayRequest{
		Path:        requestPath,
		APIKey:      apiKey,
		ContentType: contentType,
		Body:        body,
		RequestID:   runID,
	})
	if response != nil {
		defer func() { _ = response.Close() }()
		applyImageManualGatewayResponseMetrics(result, response)
	}
	if requestErr != nil {
		if response != nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			outcome.gatewayStatus = ImageChannelMonitorGatewaySucceeded
			outcome.deliveryStatus = ImageChannelMonitorDeliveryFailed
			result.Status = MonitorStatusDegraded
			result.ErrorStage = "gateway_body"
			result.Message = cleanImageManualError(requestErr)
			return outcome
		}
		outcome.result = failImageMonitorResult(result, "gateway_transport", requestErr)
		return outcome
	}
	if response == nil {
		outcome.result = failImageMonitorMessage(result, "gateway_transport", "local gateway returned no response")
		return outcome
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		stage("gateway_response", fmt.Sprintf("local gateway returned HTTP %d", response.StatusCode))
		result.Status = MonitorStatusFailed
		result.ErrorStage = defaultString(response.ErrorStage, "gateway_response")
		result.Message = imageManualGatewayFailureMessage(response)
		return outcome
	}

	outcome.gatewayStatus = ImageChannelMonitorGatewaySucceeded
	metadata := response.MetadataBytes()
	reader, err := response.Reader()
	if err != nil {
		outcome.deliveryStatus = ImageChannelMonitorDeliveryFailed
		result.Status = MonitorStatusDegraded
		result.ErrorStage = "gateway_body"
		result.Message = cleanImageManualError(err)
		return outcome
	}
	deliveryCtx, cancelDelivery := context.WithTimeout(ctx, deliveryTimeout)
	defer cancelDelivery()
	stage("delivery", "processing gateway image response")
	artifacts, deliveryStatus, errorStage, deliveryErr := s.consumeImageManualGatewayResponse(
		deliveryCtx, runID, monitor, metadata, reader, result, stage,
	)
	outcome.artifacts = artifacts
	outcome.deliveryStatus = deliveryStatus
	if deliveryErr != nil {
		result.Status = MonitorStatusDegraded
		result.ErrorStage = errorStage
		result.Message = cleanImageManualError(deliveryErr)
		return outcome
	}
	stage("complete", "manual gateway image test completed")
	result.Status = MonitorStatusOperational
	result.ErrorStage = ""
	result.Message = ""
	return outcome
}

func applyImageManualGatewayResponseMetrics(result *ImageChannelMonitorResult, response *ImageManualGatewayResponse) {
	if result == nil || response == nil {
		return
	}
	result.HTTPStatus = imageMonitorIntPtr(response.StatusCode)
	headerMs := int(response.HeaderDuration / time.Millisecond)
	totalMs := int(response.TotalDuration / time.Millisecond)
	bodyMs := totalMs - headerMs
	if bodyMs < 0 {
		bodyMs = 0
	}
	result.APIHeaderMs = imageMonitorIntPtr(headerMs)
	result.APIBodyMs = imageMonitorIntPtr(bodyMs)
	result.APITotalMs = imageMonitorIntPtr(totalMs)
	if responseSize := response.Size(); responseSize <= int64(^uint(0)>>1) {
		jsonBytes := int(responseSize)
		result.JSONBytes = &jsonBytes
	}
	result.GatewayClientRequestID = strings.TrimSpace(response.ClientRequestID)
	result.GatewayRequestIDs = append([]string(nil), response.RequestIDs...)
}

func imageManualGatewayFailureMessage(response *ImageManualGatewayResponse) string {
	if response == nil {
		return "local gateway returned no response"
	}
	message := strings.TrimSpace(response.ErrorMessage)
	if message == "" && len(response.Body) > 0 {
		message = truncateForErrorBody(string(response.Body))
	}
	if message == "" {
		message = fmt.Sprintf("local gateway returned HTTP %d", response.StatusCode)
	}
	return cleanImageManualError(errors.New(message))
}

func cleanImageManualError(err error) string {
	if err == nil {
		return ""
	}
	return truncateMessage(sanitizeErrorMessage(err.Error()))
}

func (s *ImageChannelMonitorService) consumeImageManualGatewayResponse(
	ctx context.Context,
	runID string,
	monitor *ImageChannelMonitor,
	metadataBytes []byte,
	reader io.Reader,
	result *ImageChannelMonitorResult,
	stage imageMonitorStageFunc,
) (artifacts []imageChannelMonitorStoredArtifact, deliveryStatus string, errorStage string, resultErr error) {
	defer func() {
		if rec := recover(); rec != nil {
			removeImageChannelManualArtifactFiles(artifacts)
			panic(rec)
		}
	}()
	metadata, err := parseImageChannelManualGatewayMetadata(metadataBytes)
	if err != nil {
		return artifacts, ImageChannelMonitorDeliveryFailed, "json_parse", err
	}
	dataCount := len(metadata.Data)
	if dataCount == 0 {
		return artifacts, ImageChannelMonitorDeliveryFailed, "image_delivery", errors.New("gateway image response contains no image data")
	}
	metadataB64Count := 0
	metadataURLCount := 0
	entryURLs := make([]string, dataCount)
	skippedURLCount := 0
	var firstErr error
	firstErrorStage := ""
	for entryIndex := range metadata.Data {
		entry := &metadata.Data[entryIndex]
		entryURL := ""
		if entry.URL != nil {
			entryURL = strings.TrimSpace(*entry.URL)
			entryURLs[entryIndex] = entryURL
		}
		hasB64 := entry.B64JSON != nil && strings.TrimSpace(*entry.B64JSON) != ""
		if entryIndex == 0 {
			result.RevisedPrompt = entry.RevisedPrompt
			result.ReturnedImageURL = entryURL
		}
		result.HasURL = result.HasURL || entryURL != ""
		if entryURL != "" && result.ImageURLHost == "" {
			result.ImageURLHost = imageURLHost(entryURL)
		}
		if entry.B64JSON != nil {
			result.HasB64JSON = result.HasB64JSON || hasB64
			metadataB64Count++
		}
		if entry.URL != nil {
			metadataURLCount++
		}
	}

	var artifactStreamErr error
	inlineURLIndices := make(map[int]struct{})
	streamCounts, streamErr := streamImageManualImageValues(reader, func(key string, dataIndex int, value io.Reader) error {
		if dataIndex < 0 || dataIndex >= len(metadata.Data) {
			return errors.New("gateway image response data index is outside metadata")
		}
		entry := metadata.Data[dataIndex]
		hasB64 := entry.B64JSON != nil && strings.TrimSpace(*entry.B64JSON) != ""
		switch key {
		case imageManualB64JSONKey:
			if entry.B64JSON == nil {
				return fmt.Errorf("gateway image response b64_json metadata mismatch at data index %d", dataIndex)
			}
			if !hasB64 {
				return nil
			}
			artifact, err := s.persistImageManualArtifact(
				runID,
				dataIndex,
				ImageChannelMonitorArtifactSourceB64JSON,
				"",
				value,
			)
			if err != nil {
				artifactStreamErr = err
				return err
			}
			artifacts = append(artifacts, artifact)
			return nil
		case imageManualURLKey:
			if entry.URL == nil {
				return fmt.Errorf("gateway image response url metadata mismatch at data index %d", dataIndex)
			}
			if strings.TrimSpace(*entry.URL) == "" || hasB64 {
				return nil
			}
			artifact, handled, err := s.persistImageManualDataURLArtifact(runID, dataIndex, value)
			if err != nil {
				artifactStreamErr = err
				return err
			}
			if handled {
				inlineURLIndices[dataIndex] = struct{}{}
				artifacts = append(artifacts, artifact)
			}
		}
		return nil
	})
	if streamErr != nil {
		if artifactStreamErr != nil {
			return artifacts, ImageChannelMonitorDeliveryFailed, "image_delivery", artifactStreamErr
		}
		return artifacts, ImageChannelMonitorDeliveryFailed, "json_parse", streamErr
	}
	if streamCounts.b64JSON != metadataB64Count {
		return artifacts, ImageChannelMonitorDeliveryFailed, "json_parse", fmt.Errorf(
			"gateway image response b64_json metadata mismatch: metadata=%d response=%d",
			metadataB64Count,
			streamCounts.b64JSON,
		)
	}
	if streamCounts.url != metadataURLCount {
		return artifacts, ImageChannelMonitorDeliveryFailed, "json_parse", fmt.Errorf(
			"gateway image response url metadata mismatch: metadata=%d response=%d",
			metadataURLCount,
			streamCounts.url,
		)
	}

	type downloadOutcome struct {
		index    int
		artifact imageChannelMonitorStoredArtifact
		metrics  imageChannelManualDownloadMetrics
		err      error
	}
	downloadCount := 0
	for entryIndex := range metadata.Data {
		entry := metadata.Data[entryIndex]
		hasB64 := entry.B64JSON != nil && strings.TrimSpace(*entry.B64JSON) != ""
		_, hasInlineURL := inlineURLIndices[entryIndex]
		if hasB64 || hasInlineURL {
			continue
		}
		switch {
		case entryURLs[entryIndex] != "" && monitor.DownloadImage:
			downloadCount++
		case entryURLs[entryIndex] != "":
			skippedURLCount++
		default:
			if firstErr == nil {
				firstErr = errors.New("gateway image data entry contains neither url nor b64_json")
				firstErrorStage = "image_delivery"
			}
		}
	}
	if downloadCount > 0 {
		outcomes := make(chan downloadOutcome, downloadCount)
		for entryIndex := range metadata.Data {
			entry := metadata.Data[entryIndex]
			hasB64 := entry.B64JSON != nil && strings.TrimSpace(*entry.B64JSON) != ""
			_, hasInlineURL := inlineURLIndices[entryIndex]
			if hasB64 || hasInlineURL || entryURLs[entryIndex] == "" || !monitor.DownloadImage {
				continue
			}
			go func(index int, rawURL string) {
				artifact, metrics, downloadErr := s.downloadImageManualArtifact(ctx, runID, index, rawURL)
				outcomes <- downloadOutcome{index: index, artifact: artifact, metrics: metrics, err: downloadErr}
			}(entryIndex, entryURLs[entryIndex])
		}
		downloadOutcomes := make([]downloadOutcome, 0, downloadCount)
		for range downloadCount {
			downloadOutcomes = append(downloadOutcomes, <-outcomes)
		}
		sort.SliceStable(downloadOutcomes, func(i, j int) bool {
			return downloadOutcomes[i].index < downloadOutcomes[j].index
		})
		var firstMetrics *imageChannelManualDownloadMetrics
		for _, outcome := range downloadOutcomes {
			if outcome.err != nil {
				if firstErr == nil {
					firstErr = outcome.err
					firstErrorStage = "image_download"
				}
				continue
			}
			artifacts = append(artifacts, outcome.artifact)
			if firstMetrics == nil {
				metrics := outcome.metrics
				firstMetrics = &metrics
			}
		}
		if firstMetrics != nil {
			result.ImageDownloadMs = imageMonitorIntPtr(firstMetrics.downloadMs)
			result.ImageFirstByteMs = firstMetrics.firstByteMs
			result.ImageDownloadURL = firstMetrics.url
			result.ImageDownloadHost = firstMetrics.host
		}
	}
	sort.SliceStable(artifacts, func(i, j int) bool { return artifacts[i].Index < artifacts[j].Index })
	if len(artifacts) > 0 {
		fillImageManualResultFromArtifact(result, artifacts[0])
	}
	if firstErr != nil {
		if len(artifacts) > 0 {
			return artifacts, ImageChannelMonitorDeliverySucceeded, firstErrorStage, firstErr
		}
		return artifacts, ImageChannelMonitorDeliveryFailed, firstErrorStage, firstErr
	}
	if len(artifacts) > 0 {
		return artifacts, ImageChannelMonitorDeliverySucceeded, "", nil
	}
	if skippedURLCount == dataCount {
		return artifacts, ImageChannelMonitorDeliveryNotRequested, "", nil
	}
	stage("delivery", "gateway response did not yield an image artifact")
	return artifacts, ImageChannelMonitorDeliveryFailed, "image_delivery", errors.New("gateway response did not yield an image artifact")
}

func (s *ImageChannelMonitorService) persistImageManualDataURLArtifact(
	runID string,
	index int,
	reader io.Reader,
) (imageChannelMonitorStoredArtifact, bool, error) {
	var artifact imageChannelMonitorStoredArtifact
	prefix := make([]byte, len("data:"))
	n, err := io.ReadFull(reader, prefix)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return artifact, false, nil
		}
		return artifact, false, err
	}
	if n != len(prefix) || !strings.EqualFold(string(prefix), "data:") {
		return artifact, false, nil
	}

	const maxMetadataBytes = 4 << 10
	metadata := make([]byte, 0, 64)
	buffer := make([]byte, 1)
	for len(metadata) <= maxMetadataBytes {
		if _, err := io.ReadFull(reader, buffer); err != nil {
			return artifact, true, errors.New("gateway image data URL is incomplete")
		}
		if buffer[0] == ',' {
			contentType, err := parseImageManualDataURLMetadata(string(metadata))
			if err != nil {
				return artifact, true, err
			}
			artifact, err = s.persistImageManualArtifact(
				runID,
				index,
				ImageChannelMonitorArtifactSourceURL,
				contentType,
				base64.NewDecoder(base64.StdEncoding, reader),
			)
			return artifact, true, err
		}
		metadata = append(metadata, buffer[0])
	}
	return artifact, true, errors.New("gateway image data URL metadata is too large")
}

func parseImageManualDataURLMetadata(metadata string) (string, error) {
	parts := strings.Split(metadata, ";")
	if len(parts) == 0 || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(parts[0])), "image/") || !containsImageMonitorBase64Marker(parts) {
		return "", errors.New("gateway image data URL is invalid")
	}
	return strings.TrimSpace(parts[0]), nil
}

func parseImageChannelManualGatewayMetadata(raw []byte) (imageChannelManualGatewayMetadata, error) {
	var metadata imageChannelManualGatewayMetadata
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return metadata, errors.New("gateway image response metadata is unavailable")
	}
	if trimmed[0] != '{' {
		return metadata, errors.New("gateway image response must be a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	if err := decoder.Decode(&metadata); err != nil {
		return metadata, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return metadata, errors.New("gateway image response contains multiple JSON values")
		}
		return metadata, err
	}
	return metadata, nil
}

func (s *ImageChannelMonitorService) persistImageManualArtifact(
	runID string,
	index int,
	source string,
	declaredContentType string,
	reader io.Reader,
) (artifact imageChannelMonitorStoredArtifact, resultErr error) {
	if reader == nil {
		return artifact, errors.New("image artifact reader is nil")
	}
	s.manualMu.RLock()
	artifactDir := strings.TrimSpace(s.manualArtifactDir)
	s.manualMu.RUnlock()
	if artifactDir == "" {
		artifactDir = os.TempDir()
	}
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		return artifact, fmt.Errorf("create manual image artifact directory: %w", err)
	}
	file, err := os.CreateTemp(artifactDir, imageChannelManualArtifactFilePrefix+runID+"-*.artifact")
	if err != nil {
		return artifact, fmt.Errorf("create manual image artifact: %w", err)
	}
	path := file.Name()
	keep := false
	defer func() {
		if closeErr := file.Close(); resultErr == nil && closeErr != nil {
			resultErr = closeErr
		}
		if !keep || resultErr != nil {
			_ = os.Remove(path)
		}
	}()

	prefix := make([]byte, 512)
	prefixSize, readErr := io.ReadFull(reader, prefix)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return artifact, fmt.Errorf("%w: %v", errImageChannelManualArtifactSourceRead, readErr)
	}
	if prefixSize == 0 {
		return artifact, errors.New("image artifact is empty")
	}
	if _, err := file.Write(prefix[:prefixSize]); err != nil {
		return artifact, fmt.Errorf("write image artifact prefix: %w", err)
	}
	remainingLimit := imageMonitorMaxDownloadBytes - int64(prefixSize) + 1
	written, err := copyImageChannelManualArtifact(file, io.LimitReader(reader, remainingLimit))
	if err != nil {
		return artifact, fmt.Errorf("write image artifact: %w", err)
	}
	size := int64(prefixSize) + written
	if size > imageMonitorMaxDownloadBytes {
		return artifact, errors.New("image artifact exceeded monitor limit")
	}
	contentType, err := resolveImageManualContentType(declaredContentType, prefix[:prefixSize])
	if err != nil {
		return artifact, err
	}
	keep = true
	return imageChannelMonitorStoredArtifact{
		ImageChannelMonitorArtifactSummary: ImageChannelMonitorArtifactSummary{
			Index:       index,
			ContentType: contentType,
			Size:        size,
			Source:      source,
		},
		path: path,
	}, nil
}

func copyImageChannelManualArtifact(writer io.Writer, reader io.Reader) (int64, error) {
	buffer := make([]byte, 32<<10)
	var written int64
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			writeSize, writeErr := writer.Write(buffer[:n])
			written += int64(writeSize)
			if writeErr != nil {
				return written, writeErr
			}
			if writeSize != n {
				return written, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return written, nil
		}
		if readErr != nil {
			return written, fmt.Errorf("%w: %v", errImageChannelManualArtifactSourceRead, readErr)
		}
	}
}

func resolveImageManualContentType(declared string, prefix []byte) (string, error) {
	declared = normalizeImageManualContentType(declared)
	detected := normalizeImageManualContentType(http.DetectContentType(prefix))
	if strings.HasPrefix(detected, "image/") {
		return detected, nil
	}
	if strings.HasPrefix(declared, "image/") && detected == "application/octet-stream" {
		return declared, nil
	}
	return "", fmt.Errorf("manual image artifact has non-image content type %q", detected)
}

func normalizeImageManualContentType(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(mediaType))
	}
	if separator := strings.Index(raw, ";"); separator >= 0 {
		raw = raw[:separator]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func (s *ImageChannelMonitorService) downloadImageManualArtifact(
	ctx context.Context,
	runID string,
	index int,
	rawURL string,
) (imageChannelMonitorStoredArtifact, imageChannelManualDownloadMetrics, error) {
	var metrics imageChannelManualDownloadMetrics
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return imageChannelMonitorStoredArtifact{}, metrics, errors.New("gateway returned an invalid image URL")
	}
	s.manualMu.RLock()
	consumer := s.manualConsumer
	s.manualMu.RUnlock()
	if consumer == nil {
		return imageChannelMonitorStoredArtifact{}, metrics, errors.New("manual image consumer is not configured")
	}
	startedAt := time.Now()
	var response *http.Response
	for attempt := 1; attempt <= imageChannelManualDownloadMaxAttempts; attempt++ {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if requestErr != nil {
			return imageChannelMonitorStoredArtifact{}, metrics, requestErr
		}
		request.Header.Set("Accept", "image/*")
		request.Header.Set("User-Agent", imageManualGatewayUserAgent)
		response, err = consumer.Do(request)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return imageChannelMonitorStoredArtifact{}, metrics, fmt.Errorf("download gateway image: %w", ctxErr)
			}
			if attempt == imageChannelManualDownloadMaxAttempts {
				return imageChannelMonitorStoredArtifact{}, metrics, fmt.Errorf(
					"download gateway image failed after %d attempts: %w",
					attempt,
					err,
				)
			}
			if waitErr := waitImageChannelManualDownloadRetry(ctx, imageChannelManualDownloadRetryDelay(nil, attempt)); waitErr != nil {
				return imageChannelMonitorStoredArtifact{}, metrics, waitErr
			}
			continue
		}
		if response == nil {
			return imageChannelMonitorStoredArtifact{}, metrics, errors.New("image download returned no response")
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			timedReader := &imageManualFirstByteReader{reader: response.Body, startedAt: startedAt}
			artifact, persistErr := s.persistImageManualArtifact(
				runID,
				index,
				ImageChannelMonitorArtifactSourceURL,
				response.Header.Get("Content-Type"),
				timedReader,
			)
			_ = response.Body.Close()
			response = nil
			if persistErr != nil {
				if !isImageChannelManualDownloadReadError(persistErr) || attempt == imageChannelManualDownloadMaxAttempts {
					return imageChannelMonitorStoredArtifact{}, metrics, persistErr
				}
				if waitErr := waitImageChannelManualDownloadRetry(ctx, imageChannelManualDownloadRetryDelay(nil, attempt)); waitErr != nil {
					return imageChannelMonitorStoredArtifact{}, metrics, waitErr
				}
				continue
			}
			metrics.downloadMs = int(time.Since(startedAt) / time.Millisecond)
			if timedReader.firstByteAt > 0 {
				firstByteMs := int(timedReader.firstByteAt / time.Millisecond)
				metrics.firstByteMs = imageMonitorIntPtr(firstByteMs)
			}
			metrics.url = parsed.String()
			metrics.host = parsed.Hostname()
			return artifact, metrics, nil
		}
		statusCode := response.StatusCode
		retryable := isImageChannelManualDownloadRetryableStatus(statusCode)
		retryDelay := imageChannelManualDownloadRetryDelay(response, attempt)
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, imageChannelManualDownloadRetryBodyMaxBytes))
		_ = response.Body.Close()
		response = nil
		if !retryable || attempt == imageChannelManualDownloadMaxAttempts {
			return imageChannelMonitorStoredArtifact{}, metrics, fmt.Errorf("image download HTTP %d after %d attempt(s)", statusCode, attempt)
		}
		if waitErr := waitImageChannelManualDownloadRetry(ctx, retryDelay); waitErr != nil {
			return imageChannelMonitorStoredArtifact{}, metrics, waitErr
		}
	}
	return imageChannelMonitorStoredArtifact{}, metrics, errors.New("image download exhausted retries without a response")
}

func isImageChannelManualDownloadReadError(err error) bool {
	return errors.Is(err, errImageChannelManualArtifactSourceRead)
}

func isImageChannelManualDownloadRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooEarly ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func imageChannelManualDownloadRetryDelay(response *http.Response, attempt int) time.Duration {
	if response != nil {
		retryAfter := strings.TrimSpace(response.Header.Get("Retry-After"))
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
				return time.Duration(seconds) * time.Second
			}
			if retryAt, err := http.ParseTime(retryAfter); err == nil {
				if delay := time.Until(retryAt); delay > 0 {
					return delay
				}
				return 0
			}
		}
	}
	if attempt < 1 {
		attempt = 1
	}
	return imageChannelManualDownloadRetryBaseDelay * time.Duration(1<<(attempt-1))
}

func waitImageChannelManualDownloadRetry(ctx context.Context, delay time.Duration) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait to retry gateway image download: %w", ctx.Err())
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait to retry gateway image download: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

type imageManualFirstByteReader struct {
	reader      io.Reader
	startedAt   time.Time
	firstByteAt time.Duration
}

func (r *imageManualFirstByteReader) Read(buffer []byte) (int, error) {
	n, err := r.reader.Read(buffer)
	if n > 0 && r.firstByteAt == 0 {
		r.firstByteAt = time.Since(r.startedAt)
	}
	return n, err
}

func fillImageManualResultFromArtifact(result *ImageChannelMonitorResult, artifact imageChannelMonitorStoredArtifact) {
	if result == nil {
		return
	}
	result.ImageBytes = imageChannelManualInt64Ptr(artifact.Size)
	result.ImageContentType = artifact.ContentType
	file, err := os.Open(artifact.path)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	if config, _, err := image.DecodeConfig(file); err == nil {
		result.ImageWidth = imageMonitorIntPtr(config.Width)
		result.ImageHeight = imageMonitorIntPtr(config.Height)
	}
}

func imageChannelManualInt64Ptr(value int64) *int64 { return &value }

func (s *ImageChannelMonitorService) materializeDirectManualResult(
	runID string,
	monitor *ImageChannelMonitor,
	result *ImageChannelMonitorResult,
) imageChannelManualRunOutcome {
	outcome := imageChannelManualRunOutcome{
		result:         result,
		deliveryStatus: ImageChannelMonitorDeliveryFailed,
	}
	if result == nil {
		return outcome
	}
	if result.Status == MonitorStatusOperational {
		outcome.deliveryStatus = ImageChannelMonitorDeliverySucceeded
		if result.HasURL && monitor != nil && !monitor.DownloadImage {
			outcome.deliveryStatus = ImageChannelMonitorDeliveryNotRequested
		}
	}
	inlineData := strings.TrimSpace(result.ReturnedImageData)
	result.ReturnedImageData = ""
	if inlineData == "" {
		return outcome
	}
	contentType, encoded, err := splitImageManualDataURL(inlineData)
	if err != nil {
		result.Status = MonitorStatusDegraded
		result.ErrorStage = "artifact_store"
		result.Message = cleanImageManualError(err)
		outcome.deliveryStatus = ImageChannelMonitorDeliveryFailed
		return outcome
	}
	source := ImageChannelMonitorArtifactSourceB64JSON
	if result.HasURL {
		source = ImageChannelMonitorArtifactSourceURL
	}
	artifact, err := s.persistImageManualArtifact(
		runID,
		0,
		source,
		contentType,
		base64.NewDecoder(base64.StdEncoding, strings.NewReader(encoded)),
	)
	if err != nil {
		result.Status = MonitorStatusDegraded
		result.ErrorStage = "artifact_store"
		result.Message = cleanImageManualError(err)
		outcome.deliveryStatus = ImageChannelMonitorDeliveryFailed
		return outcome
	}
	outcome.artifacts = []imageChannelMonitorStoredArtifact{artifact}
	fillImageManualResultFromArtifact(result, artifact)
	return outcome
}

func splitImageManualDataURL(raw string) (string, string, error) {
	if len(raw) < len("data:") || !strings.EqualFold(raw[:len("data:")], "data:") {
		return "", "", errors.New("manual image data is not a data URL")
	}
	metadata, encoded, ok := strings.Cut(raw[len("data:"):], ",")
	if !ok || strings.TrimSpace(encoded) == "" {
		return "", "", errors.New("manual image data URL is incomplete")
	}
	contentType, err := parseImageManualDataURLMetadata(metadata)
	if err != nil {
		return "", "", errors.New("manual image data URL is invalid")
	}
	return contentType, strings.TrimSpace(encoded), nil
}

func (s *ImageChannelMonitorService) finishManualRunOutcome(runID string, outcome imageChannelManualRunOutcome) {
	now := time.Now()
	s.manualMu.Lock()
	status, ok := s.manualRuns[runID]
	if !ok || status.Canceled {
		delete(s.manualCancels, runID)
		s.manualMu.Unlock()
		removeImageChannelManualArtifactFiles(outcome.artifacts)
		return
	}
	if outcome.result != nil {
		outcome.result.ReturnedImageData = ""
	}
	status.Running = false
	status.Stage = finalImageMonitorRuntimeStage(outcome.result)
	status.Message = ""
	if outcome.result != nil {
		status.Message = strings.TrimSpace(outcome.result.Message)
	}
	status.UpdatedAt = now
	status.CompletedAt = &now
	status.Result = outcome.result
	status.GatewayStatus = outcome.gatewayStatus
	status.DeliveryStatus = outcome.deliveryStatus
	status.ObservationStatus = ImageChannelMonitorObservationObservable
	status.Artifacts = make([]ImageChannelMonitorArtifactSummary, 0, len(outcome.artifacts))
	for _, artifact := range outcome.artifacts {
		status.Artifacts = append(status.Artifacts, artifact.ImageChannelMonitorArtifactSummary)
	}
	s.manualRuns[runID] = status
	if len(outcome.artifacts) > 0 {
		s.manualArtifacts[runID] = append([]imageChannelMonitorStoredArtifact(nil), outcome.artifacts...)
	} else {
		delete(s.manualArtifacts, runID)
	}
	delete(s.manualCancels, runID)
	s.pruneManualRunsLocked(now)
	s.manualMu.Unlock()
}
