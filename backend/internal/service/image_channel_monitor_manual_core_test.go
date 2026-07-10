package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type imageManualAPIKeyReaderStub struct {
	key   *APIKey
	err   error
	calls atomic.Int32
}

func (s *imageManualAPIKeyReaderStub) GetByID(context.Context, int64) (*APIKey, error) {
	s.calls.Add(1)
	return s.key, s.err
}

type imageManualBlockingAPIKeyReader struct {
	key     *APIKey
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (s *imageManualBlockingAPIKeyReader) GetByID(ctx context.Context, _ int64) (*APIKey, error) {
	s.calls.Add(1)
	select {
	case s.started <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return s.key, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type imageManualGroupReaderStub struct {
	accountIDs []int64
	err        error
}

func (s *imageManualGroupReaderStub) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return append([]int64(nil), s.accountIDs...), s.err
}

type imageManualGatewayStub struct {
	mu       sync.Mutex
	requests []ImageManualGatewayRequest
	response *ImageManualGatewayResponse
	err      error
	block    <-chan struct{}
}

type imageManualGatewayFunc func(context.Context, ImageManualGatewayRequest) (*ImageManualGatewayResponse, error)

func (f imageManualGatewayFunc) Do(ctx context.Context, request ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
	return f(ctx, request)
}

func (s *imageManualGatewayStub) Do(_ context.Context, request ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
	if s.block != nil {
		<-s.block
	}
	s.mu.Lock()
	request.Body = append([]byte(nil), request.Body...)
	s.requests = append(s.requests, request)
	s.mu.Unlock()
	return s.response, s.err
}

func (s *imageManualGatewayStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *imageManualGatewayStub) requestAt(index int) ImageManualGatewayRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[index]
}

type imageManualRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f imageManualRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type imageManualArtifactGateReader struct {
	reader      *bytes.Reader
	artifactDir string
	gateAt      int
	read        int
}

func (r *imageManualArtifactGateReader) Read(buffer []byte) (int, error) {
	if r.read >= r.gateAt && !imageManualArtifactExists(r.artifactDir) {
		return 0, errors.New("gateway response was consumed before artifact streaming began")
	}
	if !imageManualArtifactExists(r.artifactDir) && r.read+len(buffer) > r.gateAt {
		buffer = buffer[:r.gateAt-r.read]
	}
	n, err := r.reader.Read(buffer)
	r.read += n
	return n, err
}

func imageManualArtifactExists(directory string) bool {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), imageChannelManualArtifactFilePrefix) {
			return true
		}
	}
	return false
}

func TestImageChannelManualGatewayRunIsIdempotentAndStoresEveryImageAsArtifact(t *testing.T) {
	pngOne := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	pngTwo := append([]byte(nil), pngOne...)
	pngTwo[len(pngTwo)-1] ^= 1
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode:      http.StatusOK,
		Body:            []byte(fmt.Sprintf(`{"created":1,"data":[{"b64_json":%q},{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(pngOne), base64.StdEncoding.EncodeToString(pngTwo))),
		ClientRequestID: "client-request-1",
		RequestIDs:      []string{"manual-request-1", "upstream-request-1"},
		HeaderDuration:  12 * time.Millisecond,
		TotalDuration:   20 * time.Millisecond,
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42}, nil)
	params := ImageChannelMonitorManualTestParams{
		ExecutionMode:     ImageChannelMonitorExecutionGatewayAccount,
		APIKeyID:          7,
		ExpectedAccountID: 42,
		ClientRunID:       "browser-run-1",
		Mode:              ImageChannelMonitorManualGenerate,
		Model:             "gpt-image-1",
		Prompt:            "independent request one",
		N:                 2,
		ResponseFormat:    ImageMonitorResponseFormatB64,
		TimeoutSeconds:    30,
	}

	started, err := svc.StartManualCheck(context.Background(), 21, params)
	require.NoError(t, err)
	apiKeyReader := svc.manualAPIKeyReader.(*imageManualAPIKeyReaderStub)
	apiKeyReader.err = errors.New("API key database is temporarily unavailable")
	svc.repo.(*imageMonitorRepoStub).getErr = errors.New("monitor database is temporarily unavailable")
	retry, err := svc.StartManualCheck(context.Background(), 21, params)
	require.NoError(t, err)
	require.Equal(t, started.RunID, retry.RunID)
	require.Equal(t, int32(1), apiKeyReader.calls.Load(), "idempotent retry must return before API key lookup")

	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, ImageChannelMonitorObservationObservable, status.ObservationStatus)
	require.Len(t, status.Artifacts, 2)
	require.Equal(t, "b64_json", status.Artifacts[0].Source)
	require.Equal(t, "image/png", status.Artifacts[0].ContentType)
	require.Empty(t, status.Result.ReturnedImageData)
	require.Equal(t, "client-request-1", status.Result.GatewayClientRequestID)
	require.Equal(t, []string{"manual-request-1", "upstream-request-1"}, status.Result.GatewayRequestIDs)
	require.Equal(t, 1, gateway.callCount(), "an idempotent retry must not issue or bill another gateway request")

	artifactOne, err := svc.GetManualCheckImage(status.RunID, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = artifactOne.Reader.Close() })
	gotOne, err := io.ReadAll(artifactOne.Reader)
	require.NoError(t, err)
	require.Equal(t, pngOne, gotOne)

	artifactTwo, err := svc.GetManualCheckImage(status.RunID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = artifactTwo.Reader.Close() })
	gotTwo, err := io.ReadAll(artifactTwo.Reader)
	require.NoError(t, err)
	require.Equal(t, pngTwo, gotTwo)

	params.Prompt = "different billable request"
	conflict, err := svc.StartManualCheck(context.Background(), 21, params)
	require.Nil(t, conflict)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualRunConflict)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, 1, gateway.callCount())
}

func TestImageChannelManualGatewayConcurrentClientRunIDStartsOnlyOneGatewayRequest(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[]}`),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42}, nil)
	reader := &imageManualBlockingAPIKeyReader{
		key:     &APIKey{ID: 7, Key: "sk-concurrent-idempotency", GroupID: imageManualInt64Ptr(9), Status: StatusActive},
		started: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	svc.manualAPIKeyReader = reader
	params := ImageChannelMonitorManualTestParams{
		ExecutionMode:     ImageChannelMonitorExecutionGatewayAccount,
		APIKeyID:          7,
		ExpectedAccountID: 42,
		ClientRunID:       "concurrent-browser-run-1",
		Mode:              ImageChannelMonitorManualGenerate,
		Model:             "gpt-image-1",
		Prompt:            "one billable request despite concurrent launch retries",
		N:                 1,
		TimeoutSeconds:    30,
	}

	type startResult struct {
		status *ImageChannelMonitorManualRunStatus
		err    error
	}
	results := make(chan startResult, 2)
	for range 2 {
		go func() {
			status, err := svc.StartManualCheck(context.Background(), 21, params)
			results <- startResult{status: status, err: err}
		}()
	}
	for range 2 {
		select {
		case <-reader.started:
		case <-time.After(time.Second):
			t.Fatal("concurrent launch did not reach credential lookup")
		}
	}
	close(reader.release)

	first := <-results
	second := <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotNil(t, first.status)
	require.NotNil(t, second.status)
	require.Equal(t, first.status.RunID, second.status.RunID)
	waitForImageManualRun(t, svc, first.status.RunID)
	require.Equal(t, 1, gateway.callCount(), "concurrent retries must not generate or bill twice")
}

func TestImageChannelManualGatewayUsesRealGatewayDeadlineAndSeparateDeliveryBudget(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := imageManualGatewayFunc(func(ctx context.Context, request ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
		require.Zero(t, request.Timeout, "manual monitor timeout must not shorten the real gateway deadline")
		_, hasDeadline := ctx.Deadline()
		require.False(t, hasDeadline, "gateway mode should inherit only the run cancel context")
		return &ImageManualGatewayResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(png))),
		}, nil
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "real-gateway-deadline-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "let the real image route own its timeout",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Len(t, status.Artifacts, 1)
}

func TestImageChannelManualGatewayCancelStillStopsRealGatewayRequest(t *testing.T) {
	gatewayStarted := make(chan struct{}, 1)
	gatewayCanceled := make(chan struct{}, 1)
	gateway := imageManualGatewayFunc(func(ctx context.Context, _ ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
		gatewayStarted <- struct{}{}
		<-ctx.Done()
		gatewayCanceled <- struct{}{}
		return nil, ctx.Err()
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "cancel-real-gateway-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "cancel the independent real request",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	select {
	case <-gatewayStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway request did not start")
	}
	status, err := svc.CancelManualCheck(context.Background(), started.RunID)
	require.NoError(t, err)
	require.True(t, status.Canceled)
	select {
	case <-gatewayCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not propagate to the real gateway request")
	}
}

func TestImageChannelManualCancelByClientRunIDBeforeStartPreventsGatewayRequest(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	apiKeyReader := svc.manualAPIKeyReader.(*imageManualAPIKeyReaderStub)

	canceled, err := svc.CancelManualCheckByClientRunID(context.Background(), 21, "cancel-before-start-1")
	require.NoError(t, err)
	require.True(t, canceled.Canceled)
	require.False(t, canceled.Running)
	require.Empty(t, canceled.RunID, "a cancel intent must not create a synthetic backend run")

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "cancel-before-start-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "this request must never be generated or billed",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.True(t, started.Canceled)
	require.False(t, started.Running)
	require.Empty(t, started.RunID)
	require.Zero(t, apiKeyReader.calls.Load(), "a prior cancel intent should win before credential lookup")
	require.Zero(t, gateway.callCount(), "a prior cancel intent must prevent generation and billing")
}

func TestImageChannelManualCancelByClientRunIDWinsWhileStartIsPreparing(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	originalReader := svc.manualAPIKeyReader.(*imageManualAPIKeyReaderStub)
	blockingReader := &imageManualBlockingAPIKeyReader{
		key:     originalReader.key,
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	svc.manualAPIKeyReader = blockingReader

	type startResult struct {
		status *ImageChannelMonitorManualRunStatus
		err    error
	}
	resultCh := make(chan startResult, 1)
	go func() {
		status, startErr := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
			ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
			APIKeyID:       7,
			ClientRunID:    "cancel-during-prepare-1",
			Mode:           ImageChannelMonitorManualGenerate,
			Prompt:         "cancel must atomically beat registration",
			N:              1,
			TimeoutSeconds: 30,
		})
		resultCh <- startResult{status: status, err: startErr}
	}()

	select {
	case <-blockingReader.started:
	case <-time.After(2 * time.Second):
		t.Fatal("start did not reach gateway credential preparation")
	}
	canceled, err := svc.CancelManualCheckByClientRunID(context.Background(), 21, "cancel-during-prepare-1")
	require.NoError(t, err)
	require.True(t, canceled.Canceled)
	close(blockingReader.release)

	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.NotNil(t, result.status)
		require.True(t, result.status.Canceled)
		require.False(t, result.status.Running)
		require.Empty(t, result.status.RunID)
	case <-time.After(2 * time.Second):
		t.Fatal("start did not finish after credential preparation was released")
	}
	require.Zero(t, gateway.callCount(), "the atomic cancel tombstone must prevent the detached gateway request")
}

func TestImageChannelManualCancelByClientRunIDCancelsRegisteredGatewayRun(t *testing.T) {
	gatewayStarted := make(chan struct{}, 1)
	gatewayCanceled := make(chan struct{}, 1)
	gateway := imageManualGatewayFunc(func(ctx context.Context, _ ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
		gatewayStarted <- struct{}{}
		<-ctx.Done()
		gatewayCanceled <- struct{}{}
		return nil, ctx.Err()
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "cancel-registered-by-client-id-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "cancel the registered independent request",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	select {
	case <-gatewayStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("gateway request did not start")
	}

	canceled, err := svc.CancelManualCheckByClientRunID(context.Background(), 21, "cancel-registered-by-client-id-1")
	require.NoError(t, err)
	require.Equal(t, started.RunID, canceled.RunID)
	require.True(t, canceled.Canceled)
	select {
	case <-gatewayCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("client_run_id cancellation did not propagate to the registered gateway request")
	}
}

func TestImageChannelManualCancelByClientRunIDAlsoCoversDirectProbe(t *testing.T) {
	release := make(chan struct{})
	upstream := &imageMonitorHTTPUpstreamRecorder{
		block: release,
		body:  `{"data":[{"b64_json":"aW1hZ2U="}]}`,
	}
	svc := newConfiguredImageManualCoreTestService(t, &imageManualGatewayStub{}, nil, nil)
	svc.httpUpstream = upstream

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionDirectProbe,
		ClientRunID:    "cancel-registered-direct-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "cancel the direct request after a lost launch response",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.NotEmpty(t, started.RunID)

	canceled, err := svc.CancelManualCheckByClientRunID(context.Background(), 21, "cancel-registered-direct-1")
	require.NoError(t, err)
	require.Equal(t, started.RunID, canceled.RunID)
	require.True(t, canceled.Canceled)
	close(release)
}

func TestImageChannelManualCancelByClientRunIDBeforeDirectStartPreventsRequest(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"b64_json":"aW1hZ2U="}]}`,
	}
	svc := newConfiguredImageManualCoreTestService(t, &imageManualGatewayStub{}, nil, nil)
	svc.httpUpstream = upstream

	canceled, err := svc.CancelManualCheckByClientRunID(context.Background(), 21, "cancel-before-direct-start-1")
	require.NoError(t, err)
	require.True(t, canceled.Canceled)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionDirectProbe,
		ClientRunID:    "cancel-before-direct-start-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "the direct request must not start after cancellation",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.True(t, started.Canceled)
	require.Nil(t, upstream.req, "a prior cancel intent must prevent the direct upstream request")
}

func TestImageChannelManualGatewayStreamsLargeB64ArtifactBeforeResponseIsExhausted(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	png = append(png, bytes.Repeat([]byte{0xab}, 2<<20)...)
	body := []byte(fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(png)))
	artifactDir := t.TempDir()
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	svc.manualArtifactDir = artifactDir
	reader := &imageManualArtifactGateReader{
		reader:      bytes.NewReader(body),
		artifactDir: artifactDir,
		gateAt:      128 << 10,
	}
	result := &ImageChannelMonitorResult{}

	artifacts, deliveryStatus, errorStage, err := svc.consumeImageManualGatewayResponse(
		context.Background(),
		"stream-large-b64",
		&ImageChannelMonitor{},
		(&ImageManualGatewayResponse{Body: body}).MetadataBytes(),
		reader,
		result,
		func(string, string) {},
	)

	require.NoError(t, err)
	require.Empty(t, errorStage)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, deliveryStatus)
	require.Len(t, artifacts, 1)
	require.Equal(t, int64(len(png)), artifacts[0].Size)
	removeImageChannelManualArtifactFiles(artifacts)
}

func TestImageChannelManualGatewayStreamsLargeDataURLArtifactWithoutHTTPDownload(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	png = append(png, bytes.Repeat([]byte{0xcd}, openAIImagesResponseSpoolMetadataStringMaxBytes)...)
	body := []byte(fmt.Sprintf(`{"data":[{"url":%q}]}`, "data:image/png;base64,"+base64.StdEncoding.EncodeToString(png)))
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       body,
	}}
	var downloadCalls atomic.Int32
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		downloadCalls.Add(1)
		return nil, errors.New("data URL must not use the HTTP image downloader")
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "large-data-url-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "preserve a large inline URL response",
		N:              1,
		DownloadImage:  true,
		ResponseFormat: ImageMonitorResponseFormatURL,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, MonitorStatusOperational, status.Result.Status)
	require.True(t, status.Result.HasURL)
	require.Len(t, status.Artifacts, 1)
	require.Equal(t, 0, status.Artifacts[0].Index)
	require.Equal(t, ImageChannelMonitorArtifactSourceURL, status.Artifacts[0].Source)
	require.Equal(t, int64(len(png)), status.Artifacts[0].Size)
	require.Zero(t, downloadCalls.Load())

	artifact, err := svc.GetManualCheckImage(status.RunID, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = artifact.Reader.Close() })
	actual, err := io.ReadAll(artifact.Reader)
	require.NoError(t, err)
	require.Equal(t, png, actual)
}

func TestImageChannelManualGatewayAccountRequiresSingleExpectedAccount(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{StatusCode: http.StatusOK, Body: []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`)}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42, 43}, nil)

	status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:     ImageChannelMonitorExecutionGatewayAccount,
		APIKeyID:          7,
		ExpectedAccountID: 42,
		ClientRunID:       "isolated-run-1",
		Mode:              ImageChannelMonitorManualGenerate,
		Prompt:            "must only reach account 42",
		N:                 1,
		TimeoutSeconds:    30,
	})
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualAccountIsolation)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Zero(t, gateway.callCount())
}

func TestImageChannelManualGatewayRejectsAPIKeyWithIPRestrictions(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{StatusCode: http.StatusOK}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42}, nil)
	apiKeyReader := svc.manualAPIKeyReader.(*imageManualAPIKeyReaderStub)
	apiKeyReader.key.IPWhitelist = []string{"203.0.113.10"}

	status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "ip-restricted-key-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "do not mistake loopback ACL rejection for an image failure",
		N:              1,
		TimeoutSeconds: 30,
	})

	require.Nil(t, status)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualAPIKeyIPRestricted)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Zero(t, gateway.callCount())
}

func TestImageChannelManualGatewayAccountRejectsNonAccountMonitor(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{StatusCode: http.StatusOK, Body: []byte(`{"data":[]}`)}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42}, nil)
	monitor := svc.repo.(*imageMonitorRepoStub).monitor
	monitor.SourceType = ImageChannelMonitorSourceCustom
	monitor.Endpoint = "https://api.example.com"
	monitor.APIKey = "custom-source-secret"

	status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:     ImageChannelMonitorExecutionGatewayAccount,
		APIKeyID:          7,
		ExpectedAccountID: 42,
		ClientRunID:       "non-account-monitor-1",
		Mode:              ImageChannelMonitorManualGenerate,
		Prompt:            "must not claim account isolation",
		N:                 1,
		TimeoutSeconds:    30,
	})
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualAccountIsolation)
	require.Zero(t, gateway.callCount())
}

func TestImageChannelManualGatewayStatusNeverRetainsCustomMonitorSecret(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(png))),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	monitor := svc.repo.(*imageMonitorRepoStub).monitor
	monitor.SourceType = ImageChannelMonitorSourceCustom
	monitor.Endpoint = "https://api.example.com"
	monitor.APIKey = "custom-monitor-secret-must-not-persist"

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "sanitized-monitor-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "do not retain monitor credentials",
		N:              1,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, started.Monitor)
	require.Empty(t, started.Monitor.APIKey)
	require.NotContains(t, fmt.Sprintf("%+v", started), "custom-monitor-secret-must-not-persist")

	completed := waitForImageManualRun(t, svc, started.RunID)
	require.Empty(t, completed.Monitor.APIKey)
	require.NotContains(t, fmt.Sprintf("%+v", completed), "custom-monitor-secret-must-not-persist")
}

func TestImageChannelManualGatewayURLDeliveryFailureDoesNotOverwriteGatewaySuccess(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/result.png"}]}`),
	}}
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "delivery-failure-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "gateway succeeds but CDN fails",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliveryFailed, status.DeliveryStatus)
	require.Equal(t, MonitorStatusDegraded, status.Result.Status)
	require.Equal(t, "image_download", status.Result.ErrorStage)
	require.Empty(t, status.Artifacts)
}

func TestImageChannelManualGatewayURLDeliveryFollowsSafeCDNRedirect(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/start.png"}]}`),
	}}
	var calls atomic.Int32
	consumer := newImageChannelManualConsumerClient()
	consumer.Transport = imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		switch request.URL.Hostname() {
		case "images.example":
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": []string{"https://cdn.example/final.png"}},
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Request:    request,
			}, nil
		case "cdn.example":
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"image/png"}},
				Body:       io.NopCloser(bytes.NewReader(png)),
				Request:    request,
			}, nil
		default:
			return nil, fmt.Errorf("unexpected redirect host %s", request.URL.Hostname())
		}
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "redirect-delivery-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "follow a normal CDN redirect",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Len(t, status.Artifacts, 1)
	require.Equal(t, int32(2), calls.Load())
}

func TestImageChannelManualGatewayURLDeliveryRetriesTransientHTTPFailure(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/flaky.png"}]}`),
	}}
	var calls atomic.Int32
	consumer := newImageChannelManualConsumerClient()
	consumer.Transport = imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempt := calls.Add(1)
		if attempt == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader("temporarily unavailable")),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "transient-delivery-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "retry an idempotent image download",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, MonitorStatusOperational, status.Result.Status)
	require.Equal(t, int32(2), calls.Load())
}

func TestImageChannelManualGatewayURLDeliveryRetriesInterruptedResponseBody(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/interrupted.png"}]}`),
	}}
	var calls atomic.Int32
	consumer := newImageChannelManualConsumerClient()
	consumer.Transport = imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		attempt := calls.Add(1)
		body := io.Reader(bytes.NewReader(png))
		if attempt == 1 {
			body = io.MultiReader(
				bytes.NewReader(png[:16]),
				iotest.ErrReader(errors.New("CDN response interrupted")),
			)
		}
		return &http.Response{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": []string{"image/png"}},
			Body:          io.NopCloser(body),
			ContentLength: int64(len(png)),
			Request:       request,
		}, nil
	})
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "interrupted-delivery-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "retry an interrupted CDN body",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, MonitorStatusOperational, status.Result.Status)
	require.Equal(t, int32(2), calls.Load())
	require.Len(t, status.Artifacts, 1)
}

func TestImageChannelManualGatewaySlowURLDeliveryDoesNotSerializeConcurrentRuns(t *testing.T) {
	const concurrency = 8
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/slow.png"}]}`),
	}}
	release := make(chan struct{})
	var startedDownloads atomic.Int32
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		startedDownloads.Add(1)
		<-release
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)
	runIDs := make([]string, 0, concurrency)
	for i := 0; i < concurrency; i++ {
		started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
			ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
			APIKeyID:       7,
			ClientRunID:    fmt.Sprintf("slow-delivery-%d", i),
			Mode:           ImageChannelMonitorManualGenerate,
			Prompt:         fmt.Sprintf("independent slow delivery %d", i),
			N:              1,
			DownloadImage:  true,
			TimeoutSeconds: 30,
		})
		require.NoError(t, err)
		runIDs = append(runIDs, started.RunID)
	}
	require.Eventually(t, func() bool {
		return startedDownloads.Load() == concurrency
	}, 2*time.Second, 5*time.Millisecond, "all URL deliveries should begin independently")
	close(release)
	for _, runID := range runIDs {
		status := waitForImageManualRun(t, svc, runID)
		require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	}
}

func TestImageChannelManualGatewayMultipleURLArtifactsDownloadConcurrently(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/one.png"},{"url":"https://images.example/two.png"}]}`),
	}}
	release := make(chan struct{})
	var startedDownloads atomic.Int32
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		startedDownloads.Add(1)
		<-release
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "parallel-multi-url-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "download every generated image independently",
		N:              2,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return startedDownloads.Load() == 2
	}, 2*time.Second, 5*time.Millisecond)
	close(release)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Len(t, status.Artifacts, 2)
}

func TestImageChannelManualGatewayPartialURLDeliveryKeepsSuccessfulArtifact(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/missing.png"},{"url":"https://images.example/good.png"}]}`),
	}}
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "missing") {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "partial-multi-url-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "keep every image that can be delivered",
		N:              2,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, MonitorStatusDegraded, status.Result.Status)
	require.Equal(t, "image_download", status.Result.ErrorStage)
	require.Len(t, status.Artifacts, 1)
	require.Equal(t, 1, status.Artifacts[0].Index)
	artifact, err := svc.GetManualCheckImage(status.RunID, 1)
	require.NoError(t, err)
	t.Cleanup(func() { _ = artifact.Reader.Close() })
}

func TestImageChannelManualGatewayB64ArtifactSucceedsWhenURLFormatWasRequested(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, base64.StdEncoding.EncodeToString(png))),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)

	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "url-request-b64-response-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "accept any usable delivered image",
		N:              1,
		ResponseFormat: ImageMonitorResponseFormatURL,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	status := waitForImageManualRun(t, svc, started.RunID)
	require.Equal(t, ImageChannelMonitorGatewaySucceeded, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliverySucceeded, status.DeliveryStatus)
	require.Equal(t, MonitorStatusOperational, status.Result.Status)
	require.True(t, status.Result.HasB64JSON)
	require.Len(t, status.Artifacts, 1)
}

func TestImageChannelManualGatewayEditRunsCarryIndependentMultipartBodies(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	start := func(clientRunID, payload string) string {
		inputImage := append(append([]byte(nil), png...), []byte(payload)...)
		status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
			ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
			APIKeyID:       7,
			ClientRunID:    clientRunID,
			Mode:           ImageChannelMonitorManualEdit,
			Prompt:         "edit an independently uploaded image",
			N:              1,
			InputImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString(inputImage),
			InputImageType: "image/png",
			InputImageName: clientRunID + ".png",
			TimeoutSeconds: 30,
		})
		require.NoError(t, err)
		return status.RunID
	}

	firstRunID := start("edit-run-1", "unique-image-one")
	secondRunID := start("edit-run-2", "unique-image-two")
	_ = waitForImageManualRun(t, svc, firstRunID)
	_ = waitForImageManualRun(t, svc, secondRunID)
	require.Equal(t, 2, gateway.callCount())
	first := gateway.requestAt(0)
	second := gateway.requestAt(1)
	require.Equal(t, openAIImagesEditsEndpoint, first.Path)
	require.Equal(t, openAIImagesEditsEndpoint, second.Path)
	require.NotEqual(t, first.Body, second.Body)
	firstBody := string(first.Body)
	secondBody := string(second.Body)
	require.True(t,
		(strings.Contains(firstBody, "unique-image-one") && strings.Contains(secondBody, "unique-image-two")) ||
			(strings.Contains(firstBody, "unique-image-two") && strings.Contains(secondBody, "unique-image-one")),
		"concurrent runs must each carry their own input image bytes",
	)
}

func TestImageChannelManualArtifactLookupDistinguishesRunningAndMissing(t *testing.T) {
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	now := time.Now()
	svc.manualRuns["running-run"] = ImageChannelMonitorManualRunStatus{
		RunID:             "running-run",
		Running:           true,
		StartedAt:         now,
		UpdatedAt:         now,
		ObservationStatus: ImageChannelMonitorObservationObservable,
	}

	artifact, err := svc.GetManualCheckImage("running-run", 0)
	require.Nil(t, artifact)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualImageRunning)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))

	completedAt := time.Now()
	status := svc.manualRuns["running-run"]
	status.Running = false
	status.CompletedAt = &completedAt
	svc.manualRuns["running-run"] = status

	artifact, err = svc.GetManualCheckImage("running-run", 0)
	require.Nil(t, artifact)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualImageNotFound)
	require.Equal(t, http.StatusNotFound, infraerrors.Code(err))

	artifact, err = svc.GetManualCheckImage("unknown-run", 0)
	require.Nil(t, artifact)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualRunNotFound)
	require.Equal(t, http.StatusNotFound, infraerrors.Code(err))
}

func TestImageChannelManualArtifactExpiresWithGoneTombstoneAndDeletesFile(t *testing.T) {
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	artifactPath := filepath.Join(t.TempDir(), "expired.png")
	require.NoError(t, os.WriteFile(artifactPath, []byte("expired-image"), 0o600))
	completedAt := time.Now().Add(-imageMonitorManualRunRetention - time.Second)
	svc.manualRuns["expired-run"] = ImageChannelMonitorManualRunStatus{
		RunID:             "expired-run",
		StartedAt:         completedAt.Add(-time.Second),
		UpdatedAt:         completedAt,
		CompletedAt:       &completedAt,
		ObservationStatus: ImageChannelMonitorObservationObservable,
	}
	svc.manualArtifacts["expired-run"] = []imageChannelMonitorStoredArtifact{{
		ImageChannelMonitorArtifactSummary: ImageChannelMonitorArtifactSummary{
			Index:       0,
			ContentType: "image/png",
			Size:        13,
			Source:      ImageChannelMonitorArtifactSourceB64JSON,
		},
		path: artifactPath,
	}}

	artifact, err := svc.GetManualCheckImage("expired-run", 0)
	require.Nil(t, artifact)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualRunExpired)
	require.Equal(t, http.StatusGone, infraerrors.Code(err))
	require.NoFileExists(t, artifactPath)

	status, err := svc.GetManualCheckStatus(context.Background(), "expired-run")
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualRunExpired)
}

func TestImageChannelManualConfigurationSweepsOnlyExpiredArtifactOrphans(t *testing.T) {
	directory := t.TempDir()
	orphanPath := filepath.Join(directory, imageChannelManualArtifactFilePrefix+"orphan.artifact")
	unrelatedPath := filepath.Join(directory, "unrelated.tmp")
	require.NoError(t, os.WriteFile(orphanPath, []byte("stale image"), 0o600))
	require.NoError(t, os.WriteFile(unrelatedPath, []byte("keep me"), 0o600))
	staleAt := time.Now().Add(-imageMonitorManualRunRetention - time.Minute)
	require.NoError(t, os.Chtimes(orphanPath, staleAt, staleAt))
	require.NoError(t, os.Chtimes(unrelatedPath, staleAt, staleAt))

	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	svc.configureManualGatewayForTest(nil, nil, nil, nil, directory)

	require.NoFileExists(t, orphanPath)
	require.FileExists(t, unrelatedPath)
}

func TestImageChannelManualRunLimitEvictsOldestCompletedRunDeterministically(t *testing.T) {
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	tempDir := t.TempDir()
	base := time.Now().Add(-10 * time.Minute)
	for i := 0; i <= imageMonitorManualRunMax; i++ {
		runID := fmt.Sprintf("run-%03d", i)
		completedAt := base.Add(time.Duration(i) * time.Second)
		artifactPath := filepath.Join(tempDir, runID+".png")
		require.NoError(t, os.WriteFile(artifactPath, []byte(runID), 0o600))
		svc.manualRuns[runID] = ImageChannelMonitorManualRunStatus{
			RunID:             runID,
			StartedAt:         completedAt.Add(-time.Second),
			UpdatedAt:         completedAt,
			CompletedAt:       &completedAt,
			ObservationStatus: ImageChannelMonitorObservationObservable,
		}
		svc.manualArtifacts[runID] = []imageChannelMonitorStoredArtifact{{
			ImageChannelMonitorArtifactSummary: ImageChannelMonitorArtifactSummary{Index: 0, ContentType: "image/png", Size: int64(len(runID))},
			path:                               artifactPath,
		}}
	}

	status, err := svc.GetManualCheckStatus(context.Background(), "run-200")
	require.NoError(t, err)
	require.Equal(t, "run-200", status.RunID)
	require.Len(t, svc.manualRuns, imageMonitorManualRunMax)
	require.NoFileExists(t, filepath.Join(tempDir, "run-000.png"))
	require.FileExists(t, filepath.Join(tempDir, "run-001.png"))

	status, err = svc.GetManualCheckStatus(context.Background(), "run-000")
	require.Nil(t, status)
	require.ErrorIs(t, err, ErrImageChannelMonitorManualRunExpired)
}

func TestImageChannelManualCancelDeletesCommittedArtifacts(t *testing.T) {
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	artifactPath := filepath.Join(t.TempDir(), "canceled.png")
	require.NoError(t, os.WriteFile(artifactPath, []byte("cancel-me"), 0o600))
	now := time.Now()
	svc.manualRuns["cancel-run"] = ImageChannelMonitorManualRunStatus{
		RunID:             "cancel-run",
		Running:           true,
		StartedAt:         now,
		UpdatedAt:         now,
		GatewayStatus:     ImageChannelMonitorGatewayPending,
		DeliveryStatus:    ImageChannelMonitorDeliveryPending,
		ObservationStatus: ImageChannelMonitorObservationObservable,
		Artifacts: []ImageChannelMonitorArtifactSummary{{
			Index: 0, ContentType: "image/png", Size: 9, Source: ImageChannelMonitorArtifactSourceB64JSON,
		}},
	}
	svc.manualArtifacts["cancel-run"] = []imageChannelMonitorStoredArtifact{{
		ImageChannelMonitorArtifactSummary: ImageChannelMonitorArtifactSummary{Index: 0, ContentType: "image/png", Size: 9, Source: ImageChannelMonitorArtifactSourceB64JSON},
		path:                               artifactPath,
	}}

	status, err := svc.CancelManualCheck(context.Background(), "cancel-run")
	require.NoError(t, err)
	require.Equal(t, ImageChannelMonitorGatewayCanceled, status.GatewayStatus)
	require.Equal(t, ImageChannelMonitorDeliveryCanceled, status.DeliveryStatus)
	require.Equal(t, ImageChannelMonitorObservationObservable, status.ObservationStatus)
	require.Empty(t, status.Artifacts)
	require.NoFileExists(t, artifactPath)
}

func TestImageChannelManualCancelWinsAgainstInFlightArtifactCommit(t *testing.T) {
	png := decodeManualTestBase64(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2l5sAAAAASUVORK5CYII=")
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"url":"https://images.example/cancel-race.png"}]}`),
	}}
	consumerStarted := make(chan struct{}, 1)
	releaseConsumer := make(chan struct{})
	consumer := &http.Client{Transport: imageManualRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		consumerStarted <- struct{}{}
		<-releaseConsumer
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(png)),
			Request:    request,
		}, nil
	})}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, consumer)
	started, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:  ImageChannelMonitorExecutionGatewayGroup,
		APIKeyID:       7,
		ClientRunID:    "cancel-artifact-race-1",
		Mode:           ImageChannelMonitorManualGenerate,
		Prompt:         "cancel while CDN delivery is in flight",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 30,
	})
	require.NoError(t, err)
	select {
	case <-consumerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("URL consumer did not start")
	}
	canceled, err := svc.CancelManualCheck(context.Background(), started.RunID)
	require.NoError(t, err)
	require.True(t, canceled.Canceled)
	close(releaseConsumer)

	require.Eventually(t, func() bool {
		entries, readErr := os.ReadDir(svc.manualArtifactDir)
		if readErr != nil {
			return false
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), imageChannelManualArtifactFilePrefix) {
				return false
			}
		}
		status, statusErr := svc.GetManualCheckStatus(context.Background(), started.RunID)
		return statusErr == nil && status.Canceled && status.Result == nil && len(status.Artifacts) == 0
	}, 2*time.Second, 5*time.Millisecond)
}

func newConfiguredImageManualCoreTestService(
	t *testing.T,
	gateway imageManualGatewayDoer,
	accountIDs []int64,
	consumer *http.Client,
) *ImageChannelMonitorService {
	t.Helper()
	accountID := int64(42)
	svc := NewImageChannelMonitorService(
		&imageMonitorRepoStub{monitor: &ImageChannelMonitor{
			ID:              21,
			Name:            "real gateway image target",
			SourceType:      ImageChannelMonitorSourceAccount,
			AccountID:       &accountID,
			Model:           "gpt-image-1",
			Prompt:          "draw",
			Quality:         "auto",
			N:               1,
			IntervalSeconds: 300,
			TimeoutSeconds:  300,
		}},
		&imageMonitorAccountReaderStub{account: &Account{
			ID:          42,
			Name:        "selected account",
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Credentials: map[string]any{"api_key": "unused-direct-secret"},
		}},
		nil,
		imageMonitorPlainEncryptor{},
		nil,
		nil,
	)
	tempDir := t.TempDir()
	svc.configureManualGatewayForTest(
		&imageManualAPIKeyReaderStub{key: &APIKey{ID: 7, Key: "sk-real-gateway", GroupID: imageManualInt64Ptr(9), Status: StatusActive}},
		&imageManualGroupReaderStub{accountIDs: accountIDs},
		gateway,
		consumer,
		tempDir,
	)
	return svc
}

func waitForImageManualRun(t *testing.T, svc *ImageChannelMonitorService, runID string) *ImageChannelMonitorManualRunStatus {
	t.Helper()
	var status *ImageChannelMonitorManualRunStatus
	require.Eventually(t, func() bool {
		var err error
		status, err = svc.GetManualCheckStatus(context.Background(), runID)
		return err == nil && status != nil && !status.Running
	}, 2*time.Second, 5*time.Millisecond)
	return status
}

func decodeManualTestBase64(t *testing.T, raw string) []byte {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(raw)
	require.NoError(t, err)
	return decoded
}

func imageManualInt64Ptr(value int64) *int64 { return &value }
