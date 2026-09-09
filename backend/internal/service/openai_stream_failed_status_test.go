package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIStreamFailedEventSemanticStatus_RateLimitBeatsInvalidRequest(t *testing.T) {
	// Upstream may set type=invalid_request with code=rate_limit_exceeded on HTTP 200 SSE.
	payload := []byte(`{"type":"response.failed","response":{"error":{"type":"invalid_request_error","code":"rate_limit_exceeded","message":"Rate limit reached"}}}`)
	require.Equal(t, http.StatusTooManyRequests, openAIStreamFailedEventSemanticStatus(payload, "Rate limit reached"))
	require.Equal(t, http.StatusTooManyRequests, openAIStreamFailureStatus(payload, "Rate limit reached"))
	require.True(t, openAIStreamFailedEventShouldFailover(payload, "Rate limit reached"))
}

func TestOpenAIStreamFailedEventSemanticStatus_ContextLengthNotRateLimit(t *testing.T) {
	payload := []byte(`{"type":"response.failed","response":{"error":{"type":"invalid_request_error","code":"context_length_exceeded","message":"Your input exceeds the context window of this model."}}}`)
	require.Equal(t, http.StatusBadRequest, openAIStreamFailedEventSemanticStatus(payload, "exceeds the context window"))
	require.False(t, openAIStreamFailedEventShouldFailover(payload, "exceeds the context window"))
}
