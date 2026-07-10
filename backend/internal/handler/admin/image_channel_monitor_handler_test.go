package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
			ReturnedImageURL:  "https://images.example/result.png",
			ReturnedImageData: "data:image/png;base64,large-payload",
		},
	}

	response := imageMonitorManualRunToResponse(status)
	require.NotNil(t, response.Result)
	require.Equal(t, "https://images.example/result.png", response.Result.ReturnedImageURL)
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
