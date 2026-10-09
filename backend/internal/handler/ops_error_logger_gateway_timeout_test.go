package handler

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApplyGatewayLocalTimeoutClass_OwnLimitIsNotUpstream(t *testing.T) {
	msg := "openai_header_wait_timeout waited_ms=90001"
	entry := &service.OpsInsertErrorLogInput{
		ErrorPhase:           "upstream",
		ErrorOwner:           "provider",
		ErrorSource:          "upstream_http",
		ErrorType:            "upstream_error",
		StatusCode:           502,
		ErrorMessage:         "Upstream service temporarily unavailable",
		UpstreamErrorMessage: &msg,
	}
	applyGatewayLocalTimeoutClass(entry)
	require.Equal(t, "internal", entry.ErrorPhase)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Equal(t, "gateway", entry.ErrorSource)

	idle := "upstream stream idle for 1m0s"
	interval := &service.OpsInsertErrorLogInput{
		ErrorPhase:   "upstream",
		ErrorOwner:   "provider",
		ErrorSource:  "upstream_http",
		StatusCode:   502,
		ErrorMessage: idle,
	}
	applyGatewayLocalTimeoutClass(interval)
	require.Equal(t, "internal", interval.ErrorPhase)
	require.Equal(t, "platform", interval.ErrorOwner)
	require.Equal(t, "gateway", interval.ErrorSource)

	frame := "stream data interval timeout"
	image := "image stream data interval timeout"
	require.True(t, isGatewayLocalTimeoutText(frame))
	require.True(t, isGatewayLocalTimeoutText(image))
}

func TestApplyGatewayLocalTimeoutClass_RecoveredKeepsPhase(t *testing.T) {
	msg := "openai_first_useful_frame_timeout waited_ms=31000"
	entry := &service.OpsInsertErrorLogInput{
		ErrorPhase:           "upstream",
		ErrorOwner:           "provider",
		ErrorSource:          "upstream_http",
		StatusCode:           200,
		ErrorMessage:         "Recovered upstream error 502: " + msg,
		UpstreamErrorMessage: &msg,
	}
	applyGatewayLocalTimeoutClass(entry)
	require.Equal(t, "upstream", entry.ErrorPhase)
	require.Equal(t, "platform", entry.ErrorOwner)
	require.Equal(t, "gateway", entry.ErrorSource)
}

func TestApplyGatewayLocalTimeoutClass_ProviderHTTPStaysUpstream(t *testing.T) {
	msg := "provider returned 503 overloaded"
	entry := &service.OpsInsertErrorLogInput{
		ErrorPhase:           "upstream",
		ErrorOwner:           "provider",
		ErrorSource:          "upstream_http",
		StatusCode:           502,
		ErrorMessage:         "Upstream service temporarily unavailable",
		UpstreamErrorMessage: &msg,
	}
	applyGatewayLocalTimeoutClass(entry)
	require.Equal(t, "upstream", entry.ErrorPhase)
	require.Equal(t, "provider", entry.ErrorOwner)
	require.Equal(t, "upstream_http", entry.ErrorSource)
}

func TestNoteGatewayLocalTimeoutOps_RecordsOwnLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	noteGatewayLocalTimeoutOps(c, errors.New("stream data interval timeout"))
	require.Equal(t, "stream data interval timeout", c.GetString(service.OpsUpstreamErrorMessageKey))

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	noteGatewayLocalTimeoutOps(c2, errors.New("Upstream request failed"))
	require.Empty(t, c2.GetString(service.OpsUpstreamErrorMessageKey))
}
