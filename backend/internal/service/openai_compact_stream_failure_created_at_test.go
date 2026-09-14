//go:build unit

package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteOpenAICompactSSEFailureMessage_CarriesCreatedAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	writeOpenAICompactSSEFailureMessage(c, http.StatusBadGateway, "upstream_error", "boom")

	body := rec.Body.String()
	require.Contains(t, body, "event: response.failed")

	_, payload, found := strings.Cut(body, "data: ")
	require.True(t, found, "SSE frame must include a data line: %q", body)

	var event struct {
		Type     string `json:"type"`
		Response struct {
			ID        string `json:"id"`
			Object    string `json:"object"`
			CreatedAt int64  `json:"created_at"`
			Status    string `json:"status"`
		} `json:"response"`
	}
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(payload)), &event))

	require.Equal(t, "response.failed", event.Type)
	require.Equal(t, "response", event.Response.Object)
	require.Equal(t, "failed", event.Response.Status)
	require.Greater(t, event.Response.CreatedAt, int64(0),
		"response.failed must carry a valid created_at")
}
