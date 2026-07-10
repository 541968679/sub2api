package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestImageMonitorManualRunToResponseAlwaysOmitsReturnedImageData(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	status := &service.ImageChannelMonitorManualRunStatus{
		RunID:     "manual-run-1",
		Monitor:   &service.ImageChannelMonitor{ID: 7, Name: "4K image channel"},
		Mode:      service.ImageChannelMonitorManualGenerate,
		Running:   false,
		Stage:     "complete",
		StartedAt: now,
		UpdatedAt: now,
		Result: &service.ImageChannelMonitorResult{
			MonitorID:         7,
			Status:            service.MonitorStatusOperational,
			CheckedAt:         now,
			ReturnedImageURL:  "data:image/png;base64,url-field-payload",
			ReturnedImageData: "data:image/png;base64,large-payload",
		},
	}

	response := imageMonitorManualRunToResponse(status)
	require.NotNil(t, response.Result)
	require.Empty(t, response.Result.ReturnedImageURL, "data URLs must use the artifact endpoint too")
	require.Empty(t, response.Result.ReturnedImageData, "polling/status responses must remain metadata-only")
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "returned_image_data", "polling/status JSON must not expose inline image fields")
	require.NotContains(t, string(encoded), "data:image", "polling/status JSON must never carry inline image bytes")
}

func TestWriteImageChannelMonitorArtifactReturnsBinaryWithoutCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := []byte("\x89PNG\r\n\x1a\nindependent-manual-image")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	writeImageChannelMonitorArtifact(ctx, &service.ImageChannelMonitorArtifact{
		ContentType: "image/png",
		Size:        int64(len(payload)),
		Reader:      io.NopCloser(bytes.NewReader(payload)),
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, payload, recorder.Body.Bytes())
}

func TestImageChannelMonitorHandlerExposesManualImageEndpoint(t *testing.T) {
	var endpoint func(*ImageChannelMonitorHandler, *gin.Context) = (*ImageChannelMonitorHandler).ManualTestImage
	var lookup func(*service.ImageChannelMonitorService, string, int) (*service.ImageChannelMonitorArtifact, error) = (*service.ImageChannelMonitorService).GetManualCheckImage
	require.NotNil(t, endpoint)
	require.NotNil(t, lookup)
}

func TestImageChannelMonitorHandlerExposesClientRunCancellationEndpoint(t *testing.T) {
	var endpoint func(*ImageChannelMonitorHandler, *gin.Context) = (*ImageChannelMonitorHandler).CancelManualTestByClientRunID
	var cancel func(*service.ImageChannelMonitorService, context.Context, int64, string) (*service.ImageChannelMonitorManualRunStatus, error) = (*service.ImageChannelMonitorService).CancelManualCheckByClientRunID
	require.NotNil(t, endpoint)
	require.NotNil(t, cancel)
}

func TestImageChannelMonitorHandlerCancelsBeforeLaunchByClientRunID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewImageChannelMonitorService(nil, nil, nil, nil, nil, nil)
	handler := NewImageChannelMonitorHandler(svc)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{
		{Key: "id", Value: "21"},
		{Key: "clientRunID", Value: "cancel-before-launch-http-1"},
	}
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/image-channel-monitors/21/manual-test/client-runs/cancel-before-launch-http-1/cancel",
		nil,
	)

	handler.CancelManualTestByClientRunID(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"client_run_id":"cancel-before-launch-http-1"`)
	require.Contains(t, recorder.Body.String(), `"canceled":true`)
}

func TestImageChannelMonitorManualEditAcceptsBinaryMultipartControlRequest(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	metadata, err := json.Marshal(imageChannelMonitorManualTestRequest{
		Mode:           service.ImageChannelMonitorManualEdit,
		ClientRunID:    "binary-edit-1",
		InputImageType: "image/png",
		InputImageName: "source.png",
	})
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("metadata", string(metadata)))
	part, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	require.NoError(t, writeAllTest(part, []byte("independent-binary-image")))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/manual-test", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req

	require.True(t, strings.HasPrefix(strings.ToLower(ctx.GetHeader("Content-Type")), "multipart/form-data"))
	parsed, imageBytes, err := parseImageChannelManualMultipartRequest(ctx)
	require.NoError(t, err)
	require.Equal(t, "binary-edit-1", parsed.ClientRunID)
	require.Equal(t, "source.png", parsed.InputImageName)
	require.Equal(t, []byte("independent-binary-image"), imageBytes)
}

func writeAllTest(writer io.Writer, data []byte) error {
	_, err := writer.Write(data)
	return err
}
