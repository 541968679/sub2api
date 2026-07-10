package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type imageManualAPIKeyReaderStub struct {
	key *APIKey
	err error
}

func (s *imageManualAPIKeyReaderStub) GetByID(context.Context, int64) (*APIKey, error) {
	return s.key, s.err
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
		ExecutionMode:    ImageChannelMonitorExecutionGatewayAccount,
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
	retry, err := svc.StartManualCheck(context.Background(), 21, params)
	require.NoError(t, err)
	require.Equal(t, started.RunID, retry.RunID)

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

func TestImageChannelManualGatewayAccountRequiresSingleExpectedAccount(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{StatusCode: http.StatusOK, Body: []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`)}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, []int64{42, 43}, nil)

	status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		ExecutionMode:    ImageChannelMonitorExecutionGatewayAccount,
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
		ExecutionMode: ImageChannelMonitorExecutionGatewayGroup,
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

func TestImageChannelManualGatewayEditRunsCarryIndependentMultipartBodies(t *testing.T) {
	gateway := &imageManualGatewayStub{response: &ImageManualGatewayResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"data":[{"b64_json":"iVBORw0KGgo="}]}`),
	}}
	svc := newConfiguredImageManualCoreTestService(t, gateway, nil, nil)
	start := func(clientRunID, payload string) string {
		status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
			ExecutionMode: ImageChannelMonitorExecutionGatewayGroup,
			APIKeyID:       7,
			ClientRunID:    clientRunID,
			Mode:           ImageChannelMonitorManualEdit,
			Prompt:         "edit an independently uploaded image",
			N:              1,
			InputImageData: "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(payload)),
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
	require.Contains(t, string(first.Body), "unique-image-one")
	require.Contains(t, string(second.Body), "unique-image-two")
	require.NotEqual(t, first.Body, second.Body)
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
